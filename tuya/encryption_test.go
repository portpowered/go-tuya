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
	"testing"
	"time"
)

func TestEncryptedClient_MakeRequestPayload(t *testing.T) {
	t.Parallel()

	const (
		path         = "/v1.0/devices/test-device/commands"
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
		_, _ = w.Write([]byte(`{"success":true,"code":200,"msg":"ok"}`))
	}))
	defer server.Close()

	client := &ClientImpl{
		HTTPClient:  server.Client(),
		CloudAPIURL: server.URL,
		ClientID:    appKey,
	}
	client.TokenProvider = &fakeTokenProvider{
		accessToken:  accessToken,
		refreshToken: refreshToken,
	}
	client.EncryptedClient = &EncryptedClient{Client: client}

	ctx := context.Background()
	opReq := testOperationRequest{}

	if _, err := client.EncryptedClient.Post(ctx, path, nil, body, opReq); err != nil {
		t.Fatalf("makeRequest returned error: %v", err)
	}

	select {
	case <-requestMade:
	case <-time.After(2 * time.Second):
		t.Fatalf("server handler was not invoked")
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

type fakeTokenProvider struct {
	accessToken  string
	refreshToken string
}

func (f *fakeTokenProvider) GetAccessToken(ctx context.Context, req GetAccessTokenRequest) (string, error) {
	return f.accessToken, nil
}

func (f *fakeTokenProvider) GetRefreshToken(ctx context.Context, req GetRefreshTokenRequest) (string, error) {
	return f.refreshToken, nil
}

func (f *fakeTokenProvider) SetToken(accessToken string, refreshToken string, expireTime int64) {}

func (f *fakeTokenProvider) refreshAccessTokenIfNeeded(ctx context.Context) error {
	return nil
}

type testOperationRequest struct {
	Request
}

func (r testOperationRequest) GetRequest() Request {
	return r.Request
}
