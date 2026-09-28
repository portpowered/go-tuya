package tuya

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// setupRTCTestClient creates a ClientImpl with a mock HTTPS server and valid token
// provider for testing RTC operations through the EncryptedClient.
// The mock server receives encrypted requests but returns plain JSON responses;
// the EncryptedClient skips decryption when the "result" field is not a string.
func setupRTCTestClient(handler http.HandlerFunc) (*ClientImpl, *httptest.Server) {
	server := httptest.NewTLSServer(handler)
	cloudURL := server.URL
	clientID := "test-client-id"
	client := NewClient(&ClientConfig{
		HTTPClient:  server.Client(),
		CloudAPIURL: &cloudURL,
		ClientID:    &clientID,
		AuthInformation: &AuthInformation{
			AccessToken:  "test-access-token",
			RefreshToken: "test-refresh-token",
			ExpireTime:   time.Now().Add(time.Hour).UnixMilli(),
		},
	})
	return client, server
}

// tuyaSuccessResponse builds a Tuya-style success response with the given result.
func tuyaSuccessResponse(result any) map[string]any {
	return map[string]any{
		"success": true,
		"result":  result,
		"t":       time.Now().UnixMilli(),
	}
}

// tuyaErrorResponse builds a Tuya-style error response.
func tuyaErrorResponse(code, msg string) map[string]any {
	return map[string]any{
		"success": false,
		"code":    code,
		"msg":     msg,
		"t":       time.Now().UnixMilli(),
	}
}

func TestStartRTCStream_Success(t *testing.T) {
	expectedSessionID := "test-session-abc123"
	expectedSDPAnswer := "v=0\r\no=- 0 0 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\na=group:BUNDLE 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\n"

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/v1.0/m/life/ipc/") && !strings.Contains(r.URL.Path, "/webrtc/session") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tuyaSuccessResponse(map[string]any{
			"session_id": expectedSessionID,
			"sdp":        expectedSDPAnswer,
		}))
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()
	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: "camera-device-001",
		SDPOffer: "v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if stream == nil {
		t.Fatal("expected stream, got nil")
	}
	if stream.GetSessionID() != expectedSessionID {
		t.Errorf("expected session ID %q, got %q", expectedSessionID, stream.GetSessionID())
	}
	if stream.GetSDPAnswer() != expectedSDPAnswer {
		t.Errorf("expected SDP answer %q, got %q", expectedSDPAnswer, stream.GetSDPAnswer())
	}
	if stream.GetDeviceID() != "camera-device-001" {
		t.Errorf("expected device ID %q, got %q", "camera-device-001", stream.GetDeviceID())
	}
	if !stream.IsAlive() {
		t.Error("expected stream to be alive")
	}
}

func TestStartRTCStream_EmptyDeviceID(t *testing.T) {
	client, server := setupRTCTestClient(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})
	defer server.Close()

	ctx := context.Background()
	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: "",
		SDPOffer: "v=0\r\n",
	})

	if err == nil {
		t.Fatal("expected error for empty device ID")
	}
	if stream != nil {
		t.Error("expected nil stream")
	}
	if !strings.Contains(err.Error(), "device ID is required") {
		t.Errorf("expected device ID error, got: %v", err)
	}
}

func TestStartRTCStream_EmptySDPOffer(t *testing.T) {
	client, server := setupRTCTestClient(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})
	defer server.Close()

	ctx := context.Background()
	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: "camera-001",
		SDPOffer: "",
	})

	if err == nil {
		t.Fatal("expected error for empty SDP offer")
	}
	if stream != nil {
		t.Error("expected nil stream")
	}
	if !strings.Contains(err.Error(), "SDP offer is required") {
		t.Errorf("expected SDP offer error, got: %v", err)
	}
}

func TestStartRTCStream_APIError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tuyaErrorResponse("DEVICE_OFFLINE", "device is not online"))
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()
	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: "offline-camera",
		SDPOffer: "v=0\r\n",
	})

	if err == nil {
		t.Fatal("expected error for API failure")
	}
	if stream != nil {
		t.Error("expected nil stream")
	}
}

func TestStartRTCStream_EmptySDPAnswer(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tuyaSuccessResponse(map[string]any{
			"session_id": "session-123",
			"sdp":        "",
		}))
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()
	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: "camera-001",
		SDPOffer: "v=0\r\n",
	})

	if err == nil {
		t.Fatal("expected error for empty SDP answer")
	}
	if stream != nil {
		t.Error("expected nil stream")
	}
	if !strings.Contains(err.Error(), "empty SDP answer") {
		t.Errorf("expected empty SDP answer error, got: %v", err)
	}
}

func TestStartRTCStream_HTTPError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()
	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: "camera-001",
		SDPOffer: "v=0\r\n",
	})

	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if stream != nil {
		t.Error("expected nil stream")
	}
}

func TestRTCStream_Stop(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tuyaSuccessResponse(map[string]any{
			"session_id": "session-to-stop",
			"sdp":        "v=0\r\nanswer\r\n",
		}))
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()
	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: "camera-001",
		SDPOffer: "v=0\r\noffer\r\n",
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if !stream.IsAlive() {
		t.Error("expected stream to be alive before stop")
	}

	err = stream.Stop()
	if err != nil {
		t.Fatalf("expected no error on stop, got: %v", err)
	}

	if stream.IsAlive() {
		t.Error("expected stream to not be alive after stop")
	}

	// Stop should be idempotent
	err = stream.Stop()
	if err != nil {
		t.Fatalf("expected no error on second stop, got: %v", err)
	}
}

func TestRTCStream_ContextCancellation(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tuyaSuccessResponse(map[string]any{
			"session_id": "ctx-session",
			"sdp":        "v=0\r\nanswer\r\n",
		}))
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()
	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: "camera-001",
		SDPOffer: "v=0\r\noffer\r\n",
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Stop cancels the context
	stream.Stop()

	select {
	case <-stream.ctx.Done():
		// expected
	default:
		t.Error("expected stream context to be cancelled after stop")
	}
}

func TestStartRTCStream_WithAuthorizationContext(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tuyaSuccessResponse(map[string]any{
			"session_id": "auth-ctx-session",
			"sdp":        "v=0\r\nauth-answer\r\n",
		}))
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()
	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		Request: Request{
			AuthorizationContext: &AuthorizationContext{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
				ExpireTime:   time.Now().Add(time.Hour).UnixMilli(),
			},
		},
		DeviceID: "camera-001",
		SDPOffer: "v=0\r\noffer\r\n",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if stream.GetSessionID() != "auth-ctx-session" {
		t.Errorf("expected session ID %q, got %q", "auth-ctx-session", stream.GetSessionID())
	}
}

func TestStopRTCStream_Success(t *testing.T) {
	callCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "DELETE" || callCount > 1 {
			json.NewEncoder(w).Encode(tuyaSuccessResponse(true))
			return
		}
		// First call is StartRTCStream
		json.NewEncoder(w).Encode(tuyaSuccessResponse(map[string]any{
			"session_id": "stop-session",
			"sdp":        "v=0\r\nanswer\r\n",
		}))
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()
	err := client.StopRTCStream(ctx, StopRTCStreamRequest{
		DeviceID:  "camera-001",
		SessionID: "stop-session",
	})

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
