package tuya

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEncryptedClient_MakeRequestPayload(t *testing.T) {
	t.Parallel()

	const (
		path         = "/v1.1/m/thing/test-device/commands"
		appKey       = "test-client-id"
		accessToken  = "test-access-token"
		refreshToken = "test-refresh-token"
	)

	body := map[string]interface{}{
		"scene": "toggle",
		"payload": map[string]interface{}{
			"device_id": "device-123",
			"value":     "on",
		},
	}

	expectedBodyJSON := formToJSON(body)
	var expectedBody map[string]interface{}
	if err := json.Unmarshal([]byte(expectedBodyJSON), &expectedBody); err != nil {
		t.Fatalf("failed to unmarshal expected body: %v", err)
	}

	requestMade := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(requestMade)
		defer r.Body.Close()

		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != path {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if got := r.Header.Get("X-appKey"); got != appKey {
			t.Fatalf("missing or incorrect X-appKey header, got: %s", got)
		}

		rid := r.Header.Get("X-requestId")
		if rid == "" {
			t.Fatalf("missing X-requestId header")
		}

		if _, ok := r.Header["X-Sid"]; !ok {
			t.Fatalf("missing X-sid header entry")
		}

		if r.Header.Get("X-token") != accessToken {
			t.Fatalf("missing or incorrect X-token header")
		}

		if r.Header.Get("X-time") == "" {
			t.Fatalf("missing X-time header")
		}

		bodyBytes, err := io.ReadAll(r.Body)
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

		hash := md5.Sum([]byte(rid + refreshToken))
		hashKey := hex.EncodeToString(hash[:])
		secret := secretGenerating(rid, "", hashKey)

		decrypted := decryptRequestPayload(t, encBody, secret)

		var gotBody map[string]interface{}
		if err := json.Unmarshal([]byte(decrypted), &gotBody); err != nil {
			t.Fatalf("failed to unmarshal decrypted payload: %v", err)
		}

		if !reflect.DeepEqual(gotBody, expectedBody) {
			t.Fatalf("decrypted body does not match original payload\nexpected: %#v\ngot: %#v", expectedBody, gotBody)
		}

		headersForSign := map[string]string{
			"X-appKey":    r.Header.Get("X-appKey"),
			"X-requestId": r.Header.Get("X-requestId"),
			"X-sid":       r.Header.Get("X-sid"),
			"X-time":      r.Header.Get("X-time"),
			"X-token":     r.Header.Get("X-token"),
		}

		expectedSign := restfulSign(hashKey, r.URL.Query().Get("encdata"), encBody, headersForSign)
		if gotSign := r.Header.Get("X-sign"); gotSign == "" || gotSign != expectedSign {
			t.Fatalf("unexpected X-sign header, expected %s got %s", expectedSign, gotSign)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"code":200,"msg":"ok","result":{"marker":"synthetic"},"providerField":"retained"}`))
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
	result, ok := response.Body["result"].(map[string]interface{})
	if !ok || result["marker"] != "synthetic" || response.Body["providerField"] != "retained" {
		t.Fatalf("open transport response fields were not retained: %#v", response.Body)
	}

	select {
	case <-requestMade:
	case <-time.After(2 * time.Second):
		t.Fatalf("server handler was not invoked")
	}
}

func TestEncryptedClientRejectsInvalidRequestURL(t *testing.T) {
	session := &Session{
		HTTPClient:  &http.Client{},
		CloudAPIURL: "http://[invalid-ipv6",
		ClientID:    "synthetic-client-id",
	}
	session.SetTokens(Tokens{AccessToken: "synthetic-access", RefreshToken: "synthetic-refresh"})
	session.EncryptedClient = &EncryptedClient{Client: session}

	_, err := session.EncryptedClient.Get(context.Background(), "/v1.0/devices", nil, testOperationRequest{})
	if err == nil || !strings.Contains(err.Error(), "failed to create request") {
		t.Fatalf("request error = %v, want URL construction error", err)
	}
}

func TestEncryptedClientRejectsUnschematizedOperation(t *testing.T) {
	session := &Session{HTTPClient: &http.Client{}, CloudAPIURL: "https://example.invalid", ClientID: "synthetic-client-id"}
	session.SetTokens(Tokens{AccessToken: "synthetic-access", RefreshToken: "synthetic-refresh"})
	session.EncryptedClient = &EncryptedClient{Client: session}
	for _, candidate := range []struct{ method, path string }{
		{http.MethodGet, "/v1.0/unknown"},
		{http.MethodPost, "/v1.0/devices"},
		{http.MethodGet, "/v1.0/devices?unexpected=1"},
	} {
		_, err := session.EncryptedClient.makeRequest(context.Background(), candidate.method, candidate.path, nil, nil, testOperationRequest{})
		if err == nil || !strings.Contains(err.Error(), "not in api/openapi.yaml") {
			t.Fatalf("%s %s: got %v, want schema rejection", candidate.method, candidate.path, err)
		}
	}
}

func TestWireStringMapRejectsNonStringFields(t *testing.T) {
	_, err := wireStringMap(struct {
		Count int `json:"X-count"`
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
