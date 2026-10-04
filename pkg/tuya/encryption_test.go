package tuya

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5" // #nosec G501 -- the test reproduces Tuya's protocol-mandated request-key derivation.
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)

// Repeated values stay test-local so synthetic fixtures remain independent of production constants.
const (
	encryptionFixtureRetained          = "retained"
	encryptionFixtureSyntheticAccess   = "synthetic-access"
	encryptionFixtureSyntheticClientID = "synthetic-client-id"
	encryptionFixtureMarker            = "synthetic"
	encryptionFixtureValue             = "value"
)

//nolint:cyclop,funlen,gocognit // One signed encrypted request fixture verifies all protocol headers, payload content, and response retention.
func TestEncryptedClient_MakeRequestPayload(t *testing.T) {
	t.Parallel()

	const (
		path         = "/v1.1/m/thing/test-device/commands"
		appKey       = "test-client-id"
		accessToken  = "test-access-token"
		refreshToken = "test-refresh-token"
	)

	body := map[string]any{
		"scene": "toggle",
		"payload": map[string]any{
			"device_id":            "device-123",
			encryptionFixtureValue: "on",
		},
	}

	expectedBodyJSON, err := formToJSON(body)
	if err != nil {
		t.Fatalf("failed to marshal expected body: %v", err)
	}

	var expectedBody map[string]any

	if err := json.Unmarshal([]byte(expectedBodyJSON), &expectedBody); err != nil {
		t.Fatalf("failed to unmarshal expected body: %v", err)
	}

	requestMade := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		defer close(requestMade)
		defer func() {
			if err := request.Body.Close(); err != nil {
				t.Errorf("close request body: %v", err)
			}
		}()

		if request.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", request.Method)
		}

		if request.URL.Path != path {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}

		if got := request.Header.Get("X-Appkey"); got != appKey {
			t.Fatalf("missing or incorrect X-appKey header, got: %s", got)
		}

		rid := request.Header.Get("X-Requestid")
		if rid == "" {
			t.Fatalf("missing X-requestId header")
		}

		if _, ok := request.Header["X-Sid"]; !ok {
			t.Fatalf("missing X-sid header entry")
		}

		if request.Header.Get("X-Token") != accessToken {
			t.Fatalf("missing or incorrect X-token header")
		}

		if request.Header.Get("X-Time") == "" {
			t.Fatalf("missing X-time header")
		}

		bodyBytes, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var encryptedBody map[string]string
		if err := json.Unmarshal(bodyBytes, &encryptedBody); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		encBody := encryptedBody["encdata"]
		if encBody == "" {
			t.Fatalf("missing encrypted body payload")
		}

		hash := md5.Sum([]byte(rid + refreshToken)) // #nosec G401 -- Tuya's wire protocol derives this test key with MD5.
		hashKey := hex.EncodeToString(hash[:])
		secret := secretGenerating(rid, "", hashKey)

		decrypted := decryptRequestPayload(t, encBody, secret)

		var gotBody map[string]any
		if err := json.Unmarshal([]byte(decrypted), &gotBody); err != nil {
			t.Fatalf("failed to unmarshal decrypted payload: %v", err)
		}

		if !reflect.DeepEqual(gotBody, expectedBody) {
			t.Fatalf("decrypted body does not match original payload\nexpected: %#v\ngot: %#v", expectedBody, gotBody)
		}

		headersForSign := map[string]string{
			"X-appKey":    request.Header.Get("X-Appkey"),
			"X-requestId": request.Header.Get("X-Requestid"),
			"X-sid":       request.Header.Get("X-Sid"),
			"X-time":      request.Header.Get("X-Time"),
			"X-token":     request.Header.Get("X-Token"),
		}

		expectedSign := restfulSign(hashKey, request.URL.Query().Get("encdata"), encBody, headersForSign)
		if gotSign := request.Header.Get("X-Sign"); gotSign == "" || gotSign != expectedSign {
			t.Fatalf("unexpected X-sign header, expected %s got %s", expectedSign, gotSign)
		}

		responseBody, err := json.Marshal(map[string]any{
			"success":       true,
			"code":          200,
			"msg":           "ok",
			"result":        map[string]any{"marker": encryptionFixtureMarker},
			"providerField": encryptionFixtureRetained,
		})
		if err != nil {
			responseWriter.WriteHeader(http.StatusInternalServerError)

			return
		}

		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write(responseBody)
	}))
	defer server.Close()

	client := &Session{
		HTTPClient:  server.Client(),
		CloudAPIURL: server.URL,
		ClientID:    appKey,
	}
	client.SetTokens(Tokens{AccessToken: accessToken, RefreshToken: refreshToken})
	client.EncryptedClient = &EncryptedClient{Client: client}

	ctx := context.Background()
	opReq := testOperationRequest{}

	response, err := client.EncryptedClient.Post(ctx, path, nil, body, opReq)
	if err != nil {
		t.Fatalf("makeRequest returned error: %v", err)
	}

	result, ok := response.Body["result"].(map[string]any)
	if !ok || result["marker"] != encryptionFixtureMarker || response.Body["providerField"] != encryptionFixtureRetained {
		t.Fatalf("open transport response fields were not retained: %#v", response.Body)
	}

	select {
	case <-requestMade:
	case <-time.After(2 * time.Second):
		t.Fatalf("server handler was not invoked")
	}
}

func TestEncryptRequestPayloadReportsUnsupportedBodyValue(t *testing.T) {
	t.Parallel()

	_, err := encryptRequestPayload(nil, map[string]any{"unsupported": make(chan int)}, "synthetic-secret")
	if err == nil {
		t.Fatal("expected unsupported body value to fail JSON marshaling")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) || clientErr.Kind != ErrorProtocol {
		t.Fatalf("expected protocol client error, got %T: %v", err, err)
	}

	var marshalErr *json.UnsupportedTypeError
	if !errors.As(err, &marshalErr) {
		t.Fatalf("expected wrapped JSON unsupported type error, got %T: %v", err, err)
	}
}

func TestEncryptedClientRejectsInvalidRequestURL(t *testing.T) {
	t.Parallel()

	session := &Session{
		HTTPClient:  &http.Client{},
		CloudAPIURL: "http://[invalid-ipv6",
		ClientID:    encryptionFixtureSyntheticClientID,
	}
	session.SetTokens(Tokens{AccessToken: encryptionFixtureSyntheticAccess, RefreshToken: syntheticRefreshFixture})
	session.EncryptedClient = &EncryptedClient{Client: session}

	_, err := session.EncryptedClient.Get(context.Background(), "/v1.0/devices", nil, testOperationRequest{})
	if err == nil || !strings.Contains(err.Error(), "origin") {
		t.Fatalf("request error = %v, want URL construction error", err)
	}
}

func TestEncryptedClientRejectsUnschematizedOperation(t *testing.T) {
	t.Parallel()

	session := &Session{HTTPClient: &http.Client{}, CloudAPIURL: "https://example.invalid", ClientID: encryptionFixtureSyntheticClientID}
	session.SetTokens(Tokens{AccessToken: encryptionFixtureSyntheticAccess, RefreshToken: syntheticRefreshFixture})

	session.EncryptedClient = &EncryptedClient{Client: session}
	for _, candidate := range []struct{ method, path string }{
		{http.MethodGet, "/v1.0/unknown"},
		{http.MethodPost, "/v1.0/devices"},
		{http.MethodGet, "/v1.0/devices?unexpected=1"},
	} {
		_, err := session.EncryptedClient.makeRequest(
			context.Background(),
			wire.Operation{Method: candidate.method, Path: candidate.path},
			nil,
			nil,
			nil,
			testOperationRequest{},
		)
		if err == nil || !strings.Contains(err.Error(), "not in api/openapi.yaml") {
			t.Fatalf("%s %s: got %v, want schema rejection", candidate.method, candidate.path, err)
		}
	}
}

func TestClientErrorClassifiesAndPreservesTransportCause(t *testing.T) {
	t.Parallel()

	cause := errTestSyntheticNetwork
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, cause
	})}
	session := &Session{HTTPClient: client, CloudAPIURL: "https://example.invalid", ClientID: encryptionFixtureSyntheticClientID}
	session.SetTokens(Tokens{AccessToken: encryptionFixtureSyntheticAccess, RefreshToken: syntheticRefreshFixture})
	session.EncryptedClient = &EncryptedClient{Client: session}
	_, err := session.EncryptedClient.Get(context.Background(), "/v1.0/devices", nil, testOperationRequest{})

	var classified *ClientError

	if !errors.As(err, &classified) || classified.Kind != ErrorTransport || !errors.Is(err, cause) {
		t.Fatalf("transport error = %v, want typed transport error retaining cause", err)
	}
}

func TestClientErrorClassifiesProviderStatus(t *testing.T) {
	t.Parallel()

	for _, candidate := range []struct {
		status int
		kind   ErrorKind
	}{
		{http.StatusUnauthorized, ErrorUnauthorized},
		{http.StatusNotFound, ErrorNotFound},
		{http.StatusServiceUnavailable, ErrorProvider},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(candidate.status)
		}))
		session := &Session{HTTPClient: server.Client(), CloudAPIURL: server.URL, ClientID: encryptionFixtureSyntheticClientID}
		session.SetTokens(Tokens{AccessToken: encryptionFixtureSyntheticAccess, RefreshToken: syntheticRefreshFixture})
		session.EncryptedClient = &EncryptedClient{Client: session}
		_, err := session.EncryptedClient.Get(context.Background(), "/v1.0/devices", nil, testOperationRequest{})

		var classified *ClientError

		if !errors.As(err, &classified) || classified.Kind != candidate.kind {
			t.Errorf("status %d: error = %v, want kind %s", candidate.status, err, candidate.kind)
		}

		server.Close()
	}
}

func TestWireStringMapRejectsNonStringFields(t *testing.T) {
	t.Parallel()

	_, err := wireStringMap(struct {
		Count int `json:"x_count"`
	}{Count: 1})
	if err == nil {
		t.Fatal("wireStringMap() accepted a non-string header value")
	}
}

func decryptRequestPayload(t *testing.T, encdata, secret string) string {
	t.Helper()

	const nonceLength = 16
	if len(encdata) <= nonceLength {
		t.Fatalf("encrypted data too short")
	}

	noncePart := encdata[:nonceLength]
	payloadPart := encdata[nonceLength:]

	nonce, err := base64.StdEncoding.DecodeString(noncePart)
	if err != nil {
		t.Fatalf("failed to decode nonce: %v", err)
	}

	payload, err := base64.StdEncoding.DecodeString(payloadPart)
	if err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("failed to create gcm: %v", err)
	}

	plaintext, err := gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		t.Fatalf("failed to decrypt payload: %v", err)
	}

	return string(plaintext)
}

type testOperationRequest struct {
	Request
}

func (r testOperationRequest) GetRequest() Request {
	return r.Request
}
