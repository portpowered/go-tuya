package httptransport_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/portpowered/go-tuya/pkg/dependencies/httptransport"
	wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)

const (
	testOrigin      = "https://example.invalid"
	testContentType = "application/json"
	testDevice      = "device-1"
)

var errUnexpectedTransport = errors.New("unexpected transport call")

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func testClient(transport roundTripFunc) *http.Client {
	return &http.Client{Transport: transport, CheckRedirect: nil, Jar: nil, Timeout: 0}
}

func testResponse(request *http.Request) *http.Response {
	return &http.Response{
		Status: "200 OK", StatusCode: http.StatusOK, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok")), ContentLength: 2,
		TransferEncoding: nil, Close: false, Uncompressed: false, Trailer: nil, Request: request, TLS: nil,
	}
}

func TestDoSendsGeneratedOperationThroughInjectedClient(t *testing.T) {
	t.Parallel()

	body := []byte("payload")
	client := testClient(func(request *http.Request) (*http.Response, error) {
		assertGeneratedRequest(t, request, body)

		return testResponse(request), nil
	})
	query := url.Values{wire.QueryParamClientid: {"first value", "second/value"}}
	headers := map[string]string{wire.HeaderContentType: testContentType}

	response, err := httptransport.Do(context.Background(), client, testOrigin+"/proxy/",
		wire.OperationGenerateLoginQRCode(), nil, query, headers, body)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
}

func assertGeneratedRequest(t *testing.T, request *http.Request, body []byte) {
	t.Helper()

	if request.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", request.Method)
	}

	expected := testOrigin + "/proxy/v1.0/m/life/home-assistant/qrcode/tokens?clientid=first+value&clientid=second%2Fvalue"
	if request.URL.String() != expected {
		t.Errorf("URL = %q, want %q", request.URL.String(), expected)
	}

	if request.Header.Get(wire.HeaderContentType) != testContentType {
		t.Errorf("header = %q, want %q", request.Header.Get(wire.HeaderContentType), testContentType)
	}

	defer func() { _ = request.Body.Close() }()

	requestBody, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(requestBody) != string(body) {
		t.Errorf("body = %q, want %q", requestBody, body)
	}
}

func TestDoEscapesPathArguments(t *testing.T) {
	t.Parallel()

	client := testClient(func(request *http.Request) (*http.Response, error) {
		expected := "/v1.0/devices/device%2Fwith%20space"
		if request.URL.EscapedPath() != expected {
			t.Errorf("escaped path = %q, want %q", request.URL.EscapedPath(), expected)
		}

		return testResponse(request), nil
	})

	response, err := httptransport.Do(context.Background(), client, testOrigin,
		wire.OperationGetDeviceDetails(), []any{"device/with space"}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_ = response.Body.Close()
}

type rejectedRequest struct {
	operation wire.Operation
	arguments []any
	query     url.Values
	headers   map[string]string
}

func assertRejectedRequest(t *testing.T, origin string, candidate rejectedRequest) {
	t.Helper()

	client := testClient(func(*http.Request) (*http.Response, error) {
		t.Error("invalid request reached transport")

		return nil, errUnexpectedTransport
	})

	response, err := httptransport.Do(context.Background(), client, origin,
		candidate.operation, candidate.arguments, candidate.query, candidate.headers, nil)
	if response != nil {
		_ = response.Body.Close()
	}

	if !errors.Is(err, httptransport.ErrInvalidOperation) {
		t.Fatalf("invalid request accepted: %v", err)
	}
}

func TestDoRejectsInvalidPathArguments(t *testing.T) {
	t.Parallel()

	for _, arguments := range [][]any{nil, {testDevice, "extra"}, {42}, {""}, {"."}, {".."}} {
		assertRejectedRequest(t, testOrigin, rejectedRequest{
			operation: wire.OperationGetDeviceDetails(), arguments: arguments, query: nil, headers: nil,
		})
	}
}

func TestDoRejectsChangedOperationAndUnknownKeys(t *testing.T) {
	t.Parallel()

	for _, candidate := range []rejectedRequest{
		{operation: wire.Operation{Method: http.MethodPost, Path: wire.RouteGetDeviceDetails},
			arguments: []any{testDevice}, query: nil, headers: nil},
		{operation: wire.Operation{Method: http.MethodGet, Path: wire.RouteGenerateLoginQRCode},
			arguments: nil, query: nil, headers: nil},
		{operation: wire.Operation{Method: http.MethodGet, Path: wire.RouteGetDeviceDetails + "/extra"},
			arguments: []any{testDevice}, query: nil, headers: nil},
		{operation: wire.Operation{Method: http.MethodGet, Path: "/v1.0/devices/%d"},
			arguments: []any{testDevice}, query: nil, headers: nil},
		{operation: wire.OperationQueryHomes(), arguments: nil,
			query: url.Values{"brandNewKey": {"x"}}, headers: nil},
		{operation: wire.OperationQueryHomes(), arguments: nil, query: nil,
			headers: map[string]string{"X-Forged": "x"}},
	} {
		assertRejectedRequest(t, testOrigin, candidate)
	}
}

func TestDoRejectsUnconfiguredOrigin(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{
		"https://user:secret@example.invalid", testOrigin + "?tenant=unknown", testOrigin + "#fragment",
		"file:///tmp/tuya", "https://%zz", "/relative",
	} {
		assertRejectedRequest(t, origin, rejectedRequest{
			operation: wire.OperationQueryHomes(), arguments: nil, query: nil, headers: nil,
		})
	}
}

func TestDoRejectsNilClient(t *testing.T) {
	t.Parallel()

	response, err := httptransport.Do(context.Background(), nil, testOrigin,
		wire.OperationQueryHomes(), nil, nil, nil, nil)
	if response != nil {
		_ = response.Body.Close()
	}

	if !errors.Is(err, httptransport.ErrInvalidOperation) {
		t.Fatalf("nil client error = %v", err)
	}
}

func TestDoPreservesTransportError(t *testing.T) {
	t.Parallel()

	client := testClient(func(*http.Request) (*http.Response, error) { return nil, errUnexpectedTransport })

	response, err := httptransport.Do(context.Background(), client, testOrigin,
		wire.OperationQueryHomes(), nil, nil, nil, nil)
	if response != nil {
		_ = response.Body.Close()
	}

	if !errors.Is(err, httptransport.ErrTransport) || !errors.Is(err, errUnexpectedTransport) {
		t.Fatalf("transport cause not preserved: %v", err)
	}
}
