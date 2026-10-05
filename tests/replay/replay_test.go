package replay_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5" // #nosec G501 -- replay tests reproduce Tuya's protocol-mandated request-key derivation.
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-tuya/pkg/tuya"
)

// Repeated values stay test-local so synthetic fixtures remain independent of production constants.
const (
	replayFixtureSyntheticAccessToken             = "synthetic-access-token"
	replayFixtureSyntheticClientID                = "synthetic-client-id"
	replayFixtureSyntheticAuthenticationSchema    = "synthetic-app-schema"
	replayFixtureSyntheticProduct01               = "synthetic-product-01"
	replayFixtureSyntheticRefreshToken            = "synthetic-refresh-token"
	replayFixtureSyntheticUserCode                = "synthetic-user-code"
	replayFixtureV10MLifeHomeAssistantQrcodeRoute = "/v1.0/m/life/home-assistant/qrcode/tokens"
	replayFixtureV10MLifeUsersHomes               = "/v1.0/m/life/users/homes"
	replayFixtureQRCreated                        = "auth/synthetic/qr-created.synthetic.json"
)

type replayKey struct {
	method string
	path   string
}

func (key replayKey) String() string {
	return key.method + " " + key.path
}

type replayCase struct {
	fixture  string
	validate func(*http.Request) error
}

type fixtureRequest struct {
	Method     string              `json:"method"`
	Origin     string              `json:"origin"`
	Path       string              `json:"path"`
	Query      map[string][]string `json:"query"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body"`
	PlainQuery map[string]any      `json:"plain_query,omitempty"`
	PlainBody  map[string]any      `json:"plain_body,omitempty"`
}

type fixtureExchange struct {
	Request  fixtureRequest `json:"request"`
	Response struct {
		Status  int                 `json:"status"`
		Headers map[string][]string `json:"headers"`
		Body    json.RawMessage     `json:"body"`
	} `json:"response"`
}

type fixtureTransport struct {
	cases map[replayKey]replayCase
	calls []replayKey
}

type trackedFixtureBody struct {
	io.Reader

	closed *bool
}

func (body trackedFixtureBody) Close() error {
	*body.closed = true

	return nil
}

func (transport *fixtureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil || request.URL == nil {
		return nil, testMismatchf("request URL is missing")
	}

	key := replayKey{method: request.Method, path: request.URL.Path}

	replay, ok := transport.cases[key]

	if !ok {
		return nil, testMismatchf("unexpected replay request does not match an expected route")
	}

	if slices.Contains(transport.calls, key) {
		return nil, testMismatchf("duplicate replay request for an expected route")
	}

	data, err := os.ReadFile(filepath.Join("fixtures", replay.fixture))
	if err != nil {
		return nil, fmt.Errorf("read replay fixture %q: %w", replay.fixture, err)
	}

	var exchange fixtureExchange
	if err := json.Unmarshal(data, &exchange); err != nil {
		return nil, fmt.Errorf("decode replay fixture %q: %w", replay.fixture, err)
	}

	if err := matchFixtureRequest(request, exchange); err != nil {
		return nil, fmt.Errorf("replay request does not match fixture: %w", err)
	}

	if replay.validate != nil {
		err := replay.validate(request)
		if err != nil {
			return nil, fmt.Errorf("replay request validation failed: %w", err)
		}
	}

	transport.calls = append(transport.calls, key)

	return &http.Response{
		StatusCode: exchange.Response.Status,
		Header:     http.Header(exchange.Response.Headers),
		Body:       io.NopCloser(bytes.NewReader(exchange.Response.Body)),
		Request:    request,
	}, nil
}

func (transport *fixtureTransport) verifyConsumed() error {
	if len(transport.calls) != len(transport.cases) {
		return testMismatchf("consumed %d of %d replay exchanges", len(transport.calls), len(transport.cases))
	}

	return nil
}

func matchFixtureRequest(request *http.Request, exchange fixtureExchange) error {
	if err := validateFixtureRequestURL(request); err != nil {
		return err
	}

	fixtureOrigin, err := parseFixtureOrigin(exchange.Request.Origin)
	if err != nil {
		return err
	}

	if err := matchFixtureRequestIdentity(request, exchange.Request, fixtureOrigin); err != nil {
		return err
	}

	if err := matchFixtureRequestFields(request, exchange.Request); err != nil {
		return err
	}

	return matchFixtureRequestPayload(request, exchange.Request)
}

func matchFixtureRequestFields(request *http.Request, want fixtureRequest) error {
	actualQuery, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		return testMismatchf("request query string is malformed")
	}

	if err := matchFixtureQueryValues(actualQuery, want.Query); err != nil {
		return err
	}

	return matchFixtureHeaders(request.Header, want.Headers)
}

func matchFixtureRequestPayload(request *http.Request, want fixtureRequest) error {
	body, err := readFixtureRequestBody(request)
	if err != nil {
		return err
	}

	if err := matchFixtureRequestFraming(request, body, want); err != nil {
		return err
	}

	if err := matchFixtureBody(request, body, want); err != nil {
		return err
	}

	if err := matchFixtureEncryptedQuery(request, want); err != nil {
		return err
	}

	return matchFixtureSignature(request, body, want)
}

func validateFixtureRequestURL(request *http.Request) error {
	if request == nil || request.URL == nil {
		return testMismatchf("request URL is missing")
	}

	if request.RequestURI != "" {
		return testMismatchf("request RequestURI is not allowed for outbound replay")
	}

	if request.URL.User != nil {
		return testMismatchf("request URL user information is not allowed")
	}

	if request.URL.Opaque != "" {
		return testMismatchf("opaque request URLs are not allowed")
	}

	if request.URL.Fragment != "" || request.URL.RawFragment != "" {
		return testMismatchf("request URL fragments are not allowed")
	}

	return nil
}

func parseFixtureOrigin(origin string) (*url.URL, error) {
	fixtureOrigin, err := url.Parse(origin)
	if err != nil {
		return nil, fmt.Errorf("paired fixture origin is invalid: %w", errReplayTestInvalidFixture)
	}

	if err := validateFixtureOrigin(fixtureOrigin); err != nil {
		return nil, err
	}

	return fixtureOrigin, nil
}

func validateFixtureOrigin(origin *url.URL) error {
	if err := validateFixtureOriginAuthority(origin); err != nil {
		return err
	}

	if err := validateFixtureOriginPath(origin); err != nil {
		return err
	}

	if !strings.EqualFold(origin.Scheme, "http") && !strings.EqualFold(origin.Scheme, "https") {
		return fmt.Errorf("paired fixture origin is invalid: %w", errReplayTestInvalidFixture)
	}

	return nil
}

func validateFixtureOriginAuthority(origin *url.URL) error {
	if origin.Host == "" {
		return fmt.Errorf("paired fixture origin is invalid: %w", errReplayTestInvalidFixture)
	}

	if origin.User != nil {
		return fmt.Errorf("paired fixture origin is invalid: %w", errReplayTestInvalidFixture)
	}

	if origin.Opaque != "" {
		return fmt.Errorf("paired fixture origin is invalid: %w", errReplayTestInvalidFixture)
	}

	return nil
}

func validateFixtureOriginPath(origin *url.URL) error {
	if origin.Path != "" || origin.RawPath != "" {
		return fmt.Errorf("paired fixture origin is invalid: %w", errReplayTestInvalidFixture)
	}

	if origin.RawQuery != "" || origin.ForceQuery {
		return fmt.Errorf("paired fixture origin is invalid: %w", errReplayTestInvalidFixture)
	}

	if origin.Fragment != "" || origin.RawFragment != "" {
		return fmt.Errorf("paired fixture origin is invalid: %w", errReplayTestInvalidFixture)
	}

	return nil
}

func matchFixtureRequestIdentity(request *http.Request, want fixtureRequest, fixtureOrigin *url.URL) error {
	if request.Method != want.Method {
		return testMismatchf("request method does not match fixture")
	}

	if !strings.EqualFold(request.URL.Scheme, fixtureOrigin.Scheme) {
		return testMismatchf("request scheme does not match fixture origin")
	}

	if !strings.EqualFold(request.URL.Host, fixtureOrigin.Host) {
		return testMismatchf("request URL authority does not match fixture origin")
	}

	if request.URL.EscapedPath() != want.Path {
		return testMismatchf("request escaped path does not match fixture")
	}

	if request.Host != "" && !strings.EqualFold(request.Host, fixtureOrigin.Host) {
		return testMismatchf("request Host authority does not match fixture origin")
	}

	return nil
}

func matchFixtureQueryValues(actual, expected map[string][]string) error {
	if len(actual) != len(expected) {
		return testMismatchf("request query parameters do not match fixture")
	}

	for name, values := range expected {
		got := actual[name]
		if len(got) != len(values) {
			return testMismatchf("request query parameters do not match fixture")
		}

		for i, value := range values {
			if !matchFixtureValue(got[i], value) {
				return testMismatchf("request query parameters do not match fixture")
			}
		}
	}

	return nil
}

func matchFixtureHeaders(actual http.Header, expected map[string][]string) error {
	for name, values := range expected {
		got := actual.Values(name)
		if len(got) != len(values) {
			return testMismatchf("request headers do not match fixture")
		}

		for i, value := range values {
			if !matchFixtureValue(got[i], value) {
				return testMismatchf("request headers do not match fixture")
			}
		}
	}

	return nil
}

func TestMatchFixtureHeadersUsesCanonicalCaseMatching(t *testing.T) {
	t.Parallel()

	err := matchFixtureHeaders(http.Header{"X-Appkey": {"synthetic-client-id"}}, map[string][]string{
		"X-appKey": {"synthetic-client-id"},
	})
	if err != nil {
		t.Fatalf("matchFixtureHeaders() error = %v", err)
	}
}

func readFixtureRequestBody(request *http.Request) ([]byte, error) {
	if request.Body == nil {
		return nil, nil
	}

	originalBody := request.Body
	body, readErr := io.ReadAll(originalBody)

	closeErr := originalBody.Close()

	request.Body = io.NopCloser(bytes.NewReader(body))

	if readErr != nil || closeErr != nil {
		return nil, testMismatchf("request body could not be read")
	}

	return body, nil
}

func TestReadFixtureRequestBodyClosesAndRestoresBody(t *testing.T) {
	t.Parallel()

	closed := false
	request := &http.Request{
		Body: trackedFixtureBody{
			Reader: strings.NewReader("synthetic-request-body"),
			closed: &closed,
		},
	}

	body, err := readFixtureRequestBody(request)
	if err != nil {
		t.Fatal(err)
	}

	if !closed {
		t.Fatal("original request body was not closed")
	}

	if string(body) != "synthetic-request-body" {
		t.Fatal("read request body did not match synthetic input")
	}

	restored, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}

	if err := request.Body.Close(); err != nil {
		t.Fatal(err)
	}

	if string(restored) != "synthetic-request-body" {
		t.Fatal("request body was not restored after matching")
	}
}

func matchFixtureRequestFraming(request *http.Request, body []byte, want fixtureRequest) error {
	wantBody := want.Body != ""
	if err := validateFixtureRequestFraming(request, body, wantBody); err != nil {
		return err
	}

	if !wantBody {
		if request.GetBody != nil {
			return testMismatchf("unexpected request body factory")
		}

		return nil
	}

	return matchFixtureBodyFactory(request, body)
}

func validateFixtureRequestFraming(request *http.Request, body []byte, wantBody bool) error {
	if (request.Body != nil) != wantBody {
		return testMismatchf("request body framing does not match fixture")
	}

	if request.ContentLength != int64(len(body)) {
		return testMismatchf("request body framing does not match fixture")
	}

	if len(request.TransferEncoding) != 0 {
		return testMismatchf("request body framing does not match fixture")
	}

	if request.Close {
		return testMismatchf("request body framing does not match fixture")
	}

	return nil
}

func matchFixtureBodyFactory(request *http.Request, body []byte) error {
	if request.GetBody == nil {
		return testMismatchf("request body factory is missing")
	}

	replayBody, err := request.GetBody()
	if err != nil || replayBody == nil {
		return testMismatchf("request body factory failed")
	}

	replayed, readErr := io.ReadAll(replayBody)

	closeErr := replayBody.Close()
	if readErr != nil || closeErr != nil || !bytes.Equal(replayed, body) {
		return testMismatchf("request body factory does not reproduce the request body")
	}

	return nil
}

func matchFixtureBody(request *http.Request, body []byte, want fixtureRequest) error {
	if string(body) == want.Body {
		return nil
	}

	if want.Body != "<encrypted-json>" {
		return testMismatchf("request body does not match fixture")
	}

	var envelope struct {
		Encdata string `json:"encdata"`
	}

	if err := json.Unmarshal(body, &envelope); err != nil {
		return testMismatchf("request body is not an encrypted JSON envelope")
	}

	if envelope.Encdata == "" {
		return testMismatchf("request body is not an encrypted JSON envelope")
	}

	plain, err := decryptEncdata(envelope.Encdata, request.Header.Get("X-Requestid"), replayFixtureSyntheticRefreshToken)
	if err != nil {
		return fmt.Errorf("cannot validate encrypted request body: %w", err)
	}

	if !matchJSONMeaning(plain, want.PlainBody) {
		return testMismatchf("decrypted request body does not match fixture")
	}

	return nil
}

func matchFixtureEncryptedQuery(request *http.Request, want fixtureRequest) error {
	expected := want.Query["encdata"]
	if len(expected) != 1 || expected[0] != "<encrypted>" {
		return nil
	}

	plain, err := decryptEncryptedQuery(request, replayFixtureSyntheticRefreshToken)
	if err != nil {
		return fmt.Errorf("cannot validate encrypted request query: %w", err)
	}

	if want.PlainQuery != nil && !matchJSONMeaning(plain, want.PlainQuery) {
		return testMismatchf("decrypted request query does not match fixture")
	}

	return nil
}

func matchFixtureSignature(request *http.Request, body []byte, want fixtureRequest) error {
	expected := want.Headers["X-sign"]
	if len(expected) != 1 || expected[0] != "<tuya-signature>" {
		return nil
	}

	if err := verifyTuyaSignature(request, body, replayFixtureSyntheticRefreshToken); err != nil {
		return fmt.Errorf("paired replay validation: %w", err)
	}

	return nil
}

//nolint:cyclop // This recursive JSON comparator must distinguish each supported fixture value type.
func matchJSONMeaning(got, want any) bool {
	switch expected := want.(type) {
	case string:
		if actual, ok := got.(string); ok {
			return matchFixtureValue(actual, expected)
		}

		return false
	case map[string]any:
		actual, ok := got.(map[string]any)
		if !ok || len(actual) != len(expected) {
			return false
		}

		for key, value := range expected {
			if !matchJSONMeaning(actual[key], value) {
				return false
			}
		}

		return true
	case []any:
		actual, ok := got.([]any)
		if !ok || len(actual) != len(expected) {
			return false
		}

		for index, value := range expected {
			if !matchJSONMeaning(actual[index], value) {
				return false
			}
		}

		return true
	default:
		return reflect.DeepEqual(got, want)
	}
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var hexSignaturePattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func matchFixtureValue(got, want string) bool {
	switch want {
	case "<uuid>":
		return uuidPattern.MatchString(got)
	case "<tuya-signature>":
		return hexSignaturePattern.MatchString(got)
	case "<unix-millis>":
		stamp, err := strconv.ParseInt(got, 10, 64)

		return err == nil && stamp > 0 && time.Since(time.UnixMilli(stamp)) < 5*time.Minute && time.Until(time.UnixMilli(stamp)) < 5*time.Minute
	case "<encrypted>":
		data, err := base64.StdEncoding.DecodeString(got)

		return err == nil && len(data) > 12
	default:
		return got == want
	}
}

func verifyTuyaSignature(request *http.Request, body []byte, refreshToken string) error {
	requestID := request.Header.Get("X-Requestid")
	hash := md5.Sum([]byte(requestID + refreshToken)) // #nosec G401 -- signature verification mirrors Tuya's protocol-defined test key.
	hashKey := hex.EncodeToString(hash[:])

	var signedHeaders []string

	for _, name := range []string{"X-appKey", "X-requestId", "X-sid", "X-time", "X-token"} {
		if value := request.Header.Get(name); value != "" {
			signedHeaders = append(signedHeaders, name+"="+value)
		}
	}

	message := strings.Join(signedHeaders, "||") + request.URL.Query().Get("encdata")

	if len(body) > 0 {
		var envelope struct {
			Encdata string `json:"encdata"`
		}

		err := json.Unmarshal(body, &envelope)
		if err != nil {
			return fmt.Errorf("paired replay validation: %w", err)
		}

		message += envelope.Encdata
	}

	mac := hmac.New(sha256.New, []byte(hashKey))

	_, _ = mac.Write([]byte(message))

	if got, want := request.Header.Get("X-Sign"), hex.EncodeToString(mac.Sum(nil)); got != want {
		return errReplayTestSignature
	}

	return nil
}

//nolint:cyclop,funlen,gocognit // One mutation table checks each transport mismatch class and verifies duplicate/exhaustion behavior.
func TestFixtureTransportRejectsRequestMismatchAndDuplicate(t *testing.T) {
	t.Parallel()

	key := replayKey{method: http.MethodPost, path: replayFixtureV10MLifeHomeAssistantQrcodeRoute}
	fixtureCases := map[replayKey]replayCase{
		key: {fixture: replayFixtureQRCreated},
	}
	request := func(query string) *http.Request {
		t.Helper()

		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://login.example.invalid"+key.path+"?"+query, nil)
		if err != nil {
			t.Fatal(err)
		}

		req.Header.Set("Content-Type", "application/json")

		return req
	}

	goodQuery := "clientid=synthetic-client-id&schema=" + replayFixtureSyntheticAuthenticationSchema + "&usercode=synthetic-user-code"
	diagnosticSecret := "diagnostic-secret-value"

	for _, mutation := range []struct {
		name         string
		edit         func(*http.Request)
		secretToHide string
	}{
		{"origin", func(r *http.Request) { r.URL.Host = "other.example.invalid" }, ""},
		{"Host authority override", func(r *http.Request) { r.Host = "attacker.invalid" }, ""},
		{"URL user information", func(r *http.Request) {
			r.URL.User = url.UserPassword(diagnosticSecret, "diagnostic-password")
		}, diagnosticSecret},
		{"opaque URL", func(r *http.Request) { r.URL.Opaque = "//login.example.invalid" + key.path }, ""},
		{"malformed query", func(r *http.Request) { r.URL.RawQuery = goodQuery + "&bad=%ZZ" }, ""},
		{"query secret", func(r *http.Request) {
			r.URL.RawQuery = strings.Replace(goodQuery, "synthetic-user-code", diagnosticSecret, 1)
		}, diagnosticSecret},
		{"escaped path", func(r *http.Request) { r.URL.RawPath = "/v1.0/m/life/home-assistant/qrcode/%74okens" }, ""},
		{"header secret", func(r *http.Request) { r.Header.Set("Content-Type", diagnosticSecret) }, diagnosticSecret},
		{"body secret", func(r *http.Request) {
			body := []byte(diagnosticSecret)
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.ContentLength = int64(len(body))
		}, diagnosticSecret},
		{"fragment", func(r *http.Request) { r.URL.Fragment = diagnosticSecret }, diagnosticSecret},
		{"RequestURI", func(r *http.Request) { r.RequestURI = "/?token=" + diagnosticSecret }, diagnosticSecret},
		{"unexpected call", func(r *http.Request) { r.URL.Path = "/v1.0/unknown" }, ""},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			t.Parallel()

			candidate := request(goodQuery)
			mutation.edit(candidate)

			mutationTransport := &fixtureTransport{cases: fixtureCases}

			response, err := mutationTransport.RoundTrip(candidate)
			if response != nil && response.Body != nil {
				closeErr := response.Body.Close()
				if closeErr != nil {
					t.Fatalf("close mismatch response body: %v", closeErr)
				}
			}

			if err == nil || response != nil {
				t.Fatalf("mismatched request returned response %v, error %v", response, err)
			}

			if mutation.secretToHide != "" && strings.Contains(err.Error(), mutation.secretToHide) {
				t.Fatalf("mismatch diagnostic exposed a request value: %v", err)
			}

			if len(mutationTransport.calls) != 0 {
				t.Fatalf("mismatched request consumed an exchange: %v", mutationTransport.calls)
			}

			response, err = mutationTransport.RoundTrip(request(goodQuery))
			if err != nil || response == nil {
				t.Fatalf("valid request after %s rejection returned response %v, error %v", mutation.name, response, err)
			}

			if response.Body != nil {
				if closeErr := response.Body.Close(); closeErr != nil {
					t.Fatalf("close valid response after %s rejection: %v", mutation.name, closeErr)
				}
			}

			if err := mutationTransport.verifyConsumed(); err != nil {
				t.Fatalf("valid request after %s rejection did not consume the exchange: %v", mutation.name, err)
			}
		})
	}

	transport := &fixtureTransport{cases: fixtureCases}

	response, err := transport.RoundTrip(request(goodQuery))
	if err != nil || response == nil {
		t.Fatal(err)
	}

	if response.Body != nil {
		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Fatalf("close successful response body: %v", closeErr)
		}
	}

	if err := transport.verifyConsumed(); err != nil {
		t.Fatal(err)
	}

	response, err = transport.RoundTrip(request(goodQuery))
	if response != nil && response.Body != nil {
		closeErr := response.Body.Close()
		if closeErr != nil {
			t.Fatalf("close duplicate response body: %v", closeErr)
		}
	}

	if err == nil || response != nil {
		t.Fatalf("duplicate request returned response %v, error %v", response, err)
	}
}

func TestFixtureTransportAcceptsMatchingHostOverride(t *testing.T) {
	t.Parallel()

	key := replayKey{method: http.MethodPost, path: replayFixtureV10MLifeHomeAssistantQrcodeRoute}
	transport := &fixtureTransport{cases: map[replayKey]replayCase{
		key: {fixture: replayFixtureQRCreated},
	}}
	requestURL := "https://login.example.invalid" + key.path + "?" +
		"clientid=synthetic-client-id&schema=" + replayFixtureSyntheticAuthenticationSchema + "&usercode=synthetic-user-code"

	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, requestURL, nil)
	if err != nil {
		t.Fatal(err)
	}

	request.Host = "LOGIN.EXAMPLE.INVALID"
	request.Header.Set("Content-Type", "application/json")

	response, err := transport.RoundTrip(request)
	if err != nil || response == nil {
		t.Fatalf("matching Host override returned response %v, error %v", response, err)
	}

	if response.Body != nil {
		if err := response.Body.Close(); err != nil {
			t.Fatalf("close successful response body: %v", err)
		}
	}

	if err := transport.verifyConsumed(); err != nil {
		t.Fatal(err)
	}
}

func replayClient(transport *fixtureTransport) *tuya.Session {
	client, err := tuya.NewClient(
		tuya.WithHTTPTransport(transport),
		tuya.WithClientID(replayFixtureSyntheticClientID),
		tuya.WithAuthenticationURL("https://login.example.invalid"),
		tuya.WithCloudAPIURL("https://api.example.invalid"),
	)
	if err != nil {
		panic(err)
	}

	return client.NewSession(tuya.Tokens{})
}

func authenticatedReplayClient(transport http.RoundTripper) *tuya.Session {
	client, err := tuya.NewClient(
		tuya.WithHTTPTransport(transport),
		tuya.WithClientID(replayFixtureSyntheticClientID),
		tuya.WithCloudAPIURL("https://api.example.invalid"),
	)
	if err != nil {
		panic(err)
	}

	return client.NewSession(tuya.Tokens{
		AccessToken:  replayFixtureSyntheticAccessToken,
		RefreshToken: replayFixtureSyntheticRefreshToken,
		ExpireTime:   time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
	})
}

func assertReplayCalls(t *testing.T, transport *fixtureTransport, want ...replayKey) {
	t.Helper()

	if !reflect.DeepEqual(transport.calls, want) {
		got := make([]string, len(transport.calls))
		for i, key := range transport.calls {
			got[i] = key.String()
		}

		wantStrings := make([]string, len(want))
		for i, key := range want {
			wantStrings[i] = key.String()
		}

		t.Fatalf("replay requests = %v, want %v", got, wantStrings)
	}

	err := transport.verifyConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func requireAuthRequest(request *http.Request, method, path string, query url.Values) error {
	if request.Method != method {
		return testMismatchf("request method does not match authentication fixture")
	}

	if request.URL.Path != path {
		return testMismatchf("request path does not match authentication fixture")
	}

	for name, want := range query {
		if got := request.URL.Query()[name]; !reflect.DeepEqual(got, want) {
			return testMismatchf("authentication query parameters do not match fixture")
		}
	}

	if got := request.Header.Get("Content-Type"); got != "application/json" {
		return testMismatchf("Content-Type = %q, want application/json", got)
	}

	return nil
}

func decryptEncryptedQuery(request *http.Request, refreshToken string) (map[string]any, error) {
	return decryptEncdata(request.URL.Query().Get("encdata"), request.Header.Get("X-Requestid"), refreshToken)
}

func decryptEncdata(encoded, requestID, refreshToken string) (map[string]any, error) {
	cipherData, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode encdata: %w", err)
	}

	if len(cipherData) < 12 {
		return nil, testMismatchf("encdata has %d bytes, want at least 12", len(cipherData))
	}

	hash := md5.Sum([]byte(requestID + refreshToken)) // #nosec G401 -- replay decryption mirrors Tuya's protocol-defined test key.
	hashKey := hex.EncodeToString(hash[:])

	mac := hmac.New(sha256.New, []byte(requestID))

	if _, err := mac.Write([]byte(hashKey)); err != nil {
		return nil, fmt.Errorf("derive request key: %w", err)
	}

	key := hex.EncodeToString(mac.Sum(nil))[:16]

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("create request cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create request authenticator: %w", err)
	}

	plain, err := gcm.Open(nil, cipherData[:12], cipherData[12:], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt encdata: %w", err)
	}

	var parameters map[string]any
	if err := json.Unmarshal(plain, &parameters); err != nil {
		return nil, fmt.Errorf("decode request parameters: %w", err)
	}

	return parameters, nil
}

func requireEncryptedReadRequest(request *http.Request, path string, hasEncodedQuery bool) error {
	if request.Method != http.MethodGet {
		return testMismatchf("request method does not match encrypted read fixture")
	}

	if request.URL.Path != path {
		return testMismatchf("request path does not match encrypted read fixture")
	}

	if hasEncodedQuery && request.URL.Query().Get("encdata") == "" {
		return errReplayTestEncdataMissing
	}

	if got := request.Header.Get("X-Appkey"); got != replayFixtureSyntheticClientID {
		return testMismatchf("X-Appkey does not match encrypted read fixture")
	}

	if request.Header.Get("X-Requestid") == "" {
		return errReplayTestRequestIDMissing
	}

	if request.Header.Get("X-Sign") == "" {
		return errReplayTestSignMissing
	}

	if got := request.Header.Get("X-Token"); got != replayFixtureSyntheticAccessToken {
		return testMismatchf("X-Token does not match encrypted read fixture")
	}

	if _, err := strconv.ParseInt(request.Header.Get("X-Time"), 10, 64); err != nil {
		return testMismatchf("X-Time is not an integer")
	}

	return nil
}

//nolint:cyclop,funlen // The paired QR workflow verifies both ordered calls and every returned login field.
func TestLoginReplay_QRAndValidation(t *testing.T) {
	t.Parallel()

	transport := &fixtureTransport{
		cases: map[replayKey]replayCase{
			{method: http.MethodPost, path: replayFixtureV10MLifeHomeAssistantQrcodeRoute}: {
				fixture: replayFixtureQRCreated,
				validate: func(request *http.Request) error {
					return requireAuthRequest(request, http.MethodPost, replayFixtureV10MLifeHomeAssistantQrcodeRoute, url.Values{
						"clientid": {replayFixtureSyntheticClientID},
						"usercode": {replayFixtureSyntheticUserCode},
						"schema":   {replayFixtureSyntheticAuthenticationSchema},
					})
				},
			},
			{method: http.MethodGet, path: "/v1.0/m/life/home-assistant/qrcode/tokens/synthetic-qr-code-001"}: {
				fixture: "auth/synthetic/qr-validated.synthetic.json",
				validate: func(request *http.Request) error {
					return requireAuthRequest(request, http.MethodGet, "/v1.0/m/life/home-assistant/qrcode/tokens/synthetic-qr-code-001", url.Values{
						"clientid": {replayFixtureSyntheticClientID},
						"usercode": {replayFixtureSyntheticUserCode},
					})
				},
			},
		},
	}
	client := replayClient(transport)

	qrCode, err := client.AuthService.GenerateQrCodeForLogin(context.Background(), tuya.LoginRequest{
		AccessCode: replayFixtureSyntheticUserCode,
		Schema:     replayFixtureSyntheticAuthenticationSchema,
	})
	if err != nil {
		t.Fatalf("GenerateQrCodeForLogin() error = %v", err)
	}

	if qrCode.Code != "synthetic-qr-code-001" {
		t.Fatalf("QR code = %q, want synthetic QR code", qrCode.Code)
	}

	if qrCode.QrFormattedCode != "tuyaSmart--qrLogin?token=synthetic-qr-code-001" {
		t.Fatalf("formatted QR code = %q", qrCode.QrFormattedCode)
	}

	validated, err := client.AuthService.ValidateLoginCode(context.Background(), tuya.ValidateLoginCodeRequest{
		LoginCode: qrCode.Code,
		UserCode:  replayFixtureSyntheticUserCode,
	})
	if err != nil {
		t.Fatalf("ValidateLoginCode() error = %v", err)
	}

	if !validated.Success || validated.AccessToken != replayFixtureSyntheticAccessToken || validated.RefreshToken != replayFixtureSyntheticRefreshToken {
		t.Fatalf("validated token response = %+v", validated)
	}

	if validated.ExpireTime != 3600 || validated.TerminalID != "synthetic-terminal-01" || validated.UID != "synthetic-user-01" {
		t.Fatalf("validated identity response = %+v", validated)
	}

	if validated.Endpoint != "https://api.example.invalid" {
		t.Fatalf("validated endpoint = %q", validated.Endpoint)
	}

	assertReplayCalls(t, transport,
		replayKey{method: http.MethodPost, path: replayFixtureV10MLifeHomeAssistantQrcodeRoute},
		replayKey{method: http.MethodGet, path: "/v1.0/m/life/home-assistant/qrcode/tokens/synthetic-qr-code-001"},
	)
}

//nolint:cyclop,funlen // The paired read workflow validates each dependent home, device, and status response in request order.
func TestDeviceReadReplay_HomesDevicesAndStatus(t *testing.T) {
	t.Parallel()

	transport := &fixtureTransport{
		cases: map[replayKey]replayCase{
			{method: http.MethodGet, path: replayFixtureV10MLifeUsersHomes}: {
				fixture: "device-sharing/synthetic/homes-list.synthetic.json",
				validate: func(request *http.Request) error {
					return requireEncryptedReadRequest(request, replayFixtureV10MLifeUsersHomes, false)
				},
			},
			{method: http.MethodGet, path: "/v1.0/m/life/ha/home/devices"}: {
				fixture: "device-sharing/synthetic/home-devices.synthetic.json",
				validate: func(request *http.Request) error {
					if err := requireEncryptedReadRequest(request, "/v1.0/m/life/ha/home/devices", true); err != nil {
						return fmt.Errorf("paired replay validation: %w", err)
					}

					parameters, err := decryptEncryptedQuery(request, replayFixtureSyntheticRefreshToken)
					if err != nil {
						return fmt.Errorf("paired replay validation: %w", err)
					}

					if got := parameters["homeId"]; got != "synthetic-home-01" {
						return testMismatchf("decrypted home ID does not match fixture")
					}

					return nil
				},
			},
			{method: http.MethodGet, path: "/v1.0/m/life/devices/synthetic-device-01/status"}: {
				fixture: "device-sharing/synthetic/device-status.synthetic.json",
				validate: func(request *http.Request) error {
					return requireEncryptedReadRequest(request, "/v1.0/m/life/devices/synthetic-device-01/status", false)
				},
			},
		},
	}
	client := authenticatedReplayClient(transport)
	ctx := context.Background()

	homes, err := client.HomeService.QueryHomes(ctx, tuya.QueryHomesRequest{})
	if err != nil {
		t.Fatalf("QueryHomes() error = %v", err)
	}

	if len(homes.Results) != 1 || homes.Results[0].ID != "synthetic-home-01" || homes.Results[0].Name != "Synthetic Test Home" {
		t.Fatalf("homes result = %+v", homes.Results)
	}

	devices, err := client.DevicesService.QueryDevicesByHome(ctx, tuya.QueryDevicesByHomeRequest{HomeID: homes.Results[0].ID})
	if err != nil {
		t.Fatalf("QueryDevicesByHome() error = %v", err)
	}

	if len(devices.Results) != 1 || devices.Results[0].ID != "synthetic-device-01" || devices.Results[0].Name != "Synthetic Lamp" || !devices.Results[0].Online {
		t.Fatalf("devices result = %+v", devices.Results)
	}

	status, err := client.DevicesService.QueryDeviceStatus(ctx, tuya.QueryDeviceStatusRequest{DeviceID: devices.Results[0].ID})
	if err != nil {
		t.Fatalf("QueryDeviceStatus() error = %v", err)
	}

	if len(status.Status) != 1 || status.Status[0].DPCode != "switch_1" || status.Status[0].DPId != 1 || !status.Status[0].SupportLocal {
		t.Fatalf("device status result = %+v", status.Status)
	}

	assertReplayCalls(t, transport,
		replayKey{method: http.MethodGet, path: replayFixtureV10MLifeUsersHomes},
		replayKey{method: http.MethodGet, path: "/v1.0/m/life/ha/home/devices"},
		replayKey{method: http.MethodGet, path: "/v1.0/m/life/devices/synthetic-device-01/status"},
	)
}

func TestDeviceReadReplay_APIError(t *testing.T) {
	t.Parallel()

	transport := &fixtureTransport{
		cases: map[replayKey]replayCase{
			{method: http.MethodGet, path: replayFixtureV10MLifeUsersHomes}: {
				fixture: "device-sharing/synthetic/api-error.synthetic.json",
				validate: func(request *http.Request) error {
					return requireEncryptedReadRequest(request, replayFixtureV10MLifeUsersHomes, false)
				},
			},
		},
	}
	client := authenticatedReplayClient(transport)

	_, err := client.HomeService.QueryHomes(context.Background(), tuya.QueryHomesRequest{})
	if err == nil {
		t.Fatal("QueryHomes() error = nil, want synthetic API error")
	}

	var classified *tuya.ClientError
	if !errors.As(err, &classified) || classified.Kind != tuya.ErrorUnauthorized {
		t.Fatalf("QueryHomes() error = %v, want typed unauthorized failure", err)
	}

	assertReplayCalls(t, transport, replayKey{method: http.MethodGet, path: replayFixtureV10MLifeUsersHomes})
}

//nolint:funlen // The replay test validates the complete documented query and response fixture in one auditable exchange.
func TestQueryDevicesReplay_DocumentedList(t *testing.T) {
	t.Parallel()

	transport := &fixtureTransport{
		cases: map[replayKey]replayCase{
			{method: http.MethodGet, path: "/v1.0/devices"}: {
				fixture: "device-sharing/synthetic/global-device-list.synthetic.json",
				validate: func(request *http.Request) error {
					if err := requireEncryptedReadRequest(request, "/v1.0/devices", true); err != nil {
						return fmt.Errorf("paired replay validation: %w", err)
					}

					parameters, err := decryptEncryptedQuery(request, replayFixtureSyntheticRefreshToken)
					if err != nil {
						return fmt.Errorf("paired replay validation: %w", err)
					}

					want := map[string]any{
						"page_no":    float64(1),
						"page_size":  float64(2),
						"schema":     "synthetic-app-schema",
						"product_id": replayFixtureSyntheticProduct01,
						"device_ids": "synthetic-device-a,synthetic-device-b",
						"last_id":    "synthetic-list-cursor-before",
					}
					if !reflect.DeepEqual(parameters, want) {
						return testMismatchf("decrypted query parameters do not match fixture")
					}

					return nil
				},
			},
		},
	}
	client := authenticatedReplayClient(transport)

	response, err := client.DevicesService.QueryDevices(context.Background(), tuya.QueryDevicesRequest{
		PageNo:    1,
		PageSize:  2,
		Schema:    "synthetic-app-schema",
		ProductID: replayFixtureSyntheticProduct01,
		DeviceIDs: []string{"synthetic-device-a", "synthetic-device-b"},
		LastID:    "synthetic-list-cursor-before",
	})
	if err != nil {
		t.Fatalf("QueryDevices() error = %v", err)
	}

	if response.Total != 1 || response.LastID != "synthetic-list-cursor-01" {
		t.Fatalf("QueryDevices() pagination = total %d, last ID %q", response.Total, response.LastID)
	}

	if len(response.Devices) != 1 {
		t.Fatalf("QueryDevices() returned %d devices, want 1", len(response.Devices))
	}

	wantDevice := tuya.Device{
		ID:           "synthetic-query-device-01",
		Name:         "Synthetic Device List Lamp",
		LocalKey:     "synthetic-local-key-01",
		Category:     "dj",
		ProductID:    replayFixtureSyntheticProduct01,
		ProductName:  "Synthetic Product",
		SubCategory:  "synthetic-light",
		Icon:         "https://example.invalid/synthetic-query-device.png",
		IP:           "192.0.2.15",
		Lat:          "37.0",
		Lon:          "-122.0",
		Model:        "Synthetic Model A",
		TimeZone:     "UTC",
		ActiveTime:   1700000000,
		CreateTime:   1690000000,
		UpdateTime:   1700000001,
		Online:       true,
		Status:       []tuya.Status{{Code: "switch_led", Value: true}, {Code: "brightness", Value: float64(42)}},
		Capabilities: nil,
	}
	if !reflect.DeepEqual(response.Devices[0], wantDevice) {
		t.Fatalf("QueryDevices() device = %+v, want %+v", response.Devices[0], wantDevice)
	}

	assertReplayCalls(t, transport, replayKey{method: http.MethodGet, path: "/v1.0/devices"})
}
