package tuya

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Repeated values stay test-local so synthetic fixtures remain independent of production constants.
const (
	rtcFixtureCamera001        = "camera-001"
	rtcFixtureCode             = "code"
	rtcFixtureResult           = "result"
	rtcFixtureSdp              = "sdp"
	rtcFixtureSessionID        = "session_id"
	rtcFixtureSuccess          = "success"
	rtcFixtureTestAccessToken  = "test-access-token"
	rtcFixtureTestClientID     = "test-client-id"
	rtcFixtureTestRefreshToken = "test-refresh-token"
	rtcFixtureV0               = "v=0\r\n"
	rtcFixtureV0Answer         = "v=0\r\nanswer\r\n"
	rtcFixtureV0Offer          = "v=0\r\noffer\r\n"
)

// setupRTCTestClient creates a Session with a mock HTTPS server and valid token
// provider for testing RTC operations through the EncryptedClient.
// The mock server receives encrypted requests but returns plain JSON responses;
// the EncryptedClient skips decryption when the "result" field is not a string.
func setupRTCTestClient(handler http.HandlerFunc) (*Session, *httptest.Server) {
	server := httptest.NewTLSServer(handler)
	cloudURL := server.URL
	clientID := rtcFixtureTestClientID

	base, err := newSyntheticClient(
		WithHTTPClient(server.Client()),
		WithCloudAPIURL(cloudURL),
		WithClientID(clientID),
	)
	if err != nil {
		panic(err)
	}

	client := base.NewSession(Tokens{
		AccessToken:  rtcFixtureTestAccessToken,
		RefreshToken: rtcFixtureTestRefreshToken,
		ExpireTime:   time.Now().Add(time.Hour).UnixMilli(),
	})

	return client, server
}

// tuyaSuccessResponse builds a Tuya-style success response with the given result.
func tuyaSuccessResponse(result any) map[string]any {
	return map[string]any{
		rtcFixtureSuccess: true,
		rtcFixtureResult:  result,
		"t":               time.Now().UnixMilli(),
	}
}

// tuyaErrorResponse builds a Tuya-style error response.
func tuyaErrorResponse(code, msg string) map[string]any {
	return map[string]any{
		rtcFixtureSuccess: false,
		rtcFixtureCode:    code,
		"msg":             msg,
		"t":               time.Now().UnixMilli(),
	}
}

func TestStartRTCStream_Success(t *testing.T) {
	t.Parallel()

	expectedSessionID := "test-session-abc123"
	expectedSDPAnswer := "v=0\r\no=- 0 0 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\na=group:BUNDLE 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\n"

	handler := func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", request.Method)
		}

		if !strings.Contains(request.URL.Path, "/v1.0/m/life/ipc/") && !strings.Contains(request.URL.Path, "/webrtc/session") {
			t.Errorf("unexpected path: %s", request.URL.Path)
		}

		responseWriter.Header().Set("Content-Type", "application/json")
		writeSyntheticJSONResponse(t, responseWriter, tuyaSuccessResponse(map[string]any{
			rtcFixtureSessionID: expectedSessionID,
			rtcFixtureSdp:       expectedSDPAnswer,
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
	t.Parallel()

	client, server := setupRTCTestClient(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	})
	defer server.Close()

	ctx := context.Background()

	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: "",
		SDPOffer: rtcFixtureV0,
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
	t.Parallel()

	client, server := setupRTCTestClient(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	})
	defer server.Close()

	ctx := context.Background()

	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: rtcFixtureCamera001,
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
	t.Parallel()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeSyntheticJSONResponse(t, w, tuyaErrorResponse("DEVICE_OFFLINE", "device is not online"))
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()

	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: "offline-camera",
		SDPOffer: rtcFixtureV0,
	})
	if err == nil {
		t.Fatal("expected error for API failure")
	}

	if stream != nil {
		t.Error("expected nil stream")
	}
}

func TestStartRTCStream_EmptySDPAnswer(t *testing.T) {
	t.Parallel()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeSyntheticJSONResponse(t, w, tuyaSuccessResponse(map[string]any{
			rtcFixtureSessionID: "session-123",
			rtcFixtureSdp:       "",
		}))
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()

	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: rtcFixtureCamera001,
		SDPOffer: rtcFixtureV0,
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
	t.Parallel()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeSyntheticResponseBody(t, w, "internal server error")
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()

	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: rtcFixtureCamera001,
		SDPOffer: rtcFixtureV0,
	})
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}

	if stream != nil {
		t.Error("expected nil stream")
	}
}

func TestRTCStream_Stop(t *testing.T) {
	t.Parallel()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeSyntheticJSONResponse(t, w, tuyaSuccessResponse(map[string]any{
			rtcFixtureSessionID: "session-to-stop",
			rtcFixtureSdp:       rtcFixtureV0Answer,
		}))
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()

	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: rtcFixtureCamera001,
		SDPOffer: rtcFixtureV0Offer,
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
	t.Parallel()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeSyntheticJSONResponse(t, w, tuyaSuccessResponse(map[string]any{
			rtcFixtureSessionID: "ctx-session",
			rtcFixtureSdp:       rtcFixtureV0Answer,
		}))
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()

	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		DeviceID: rtcFixtureCamera001,
		SDPOffer: rtcFixtureV0Offer,
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Stop cancels the context
	err = stream.Stop()
	if err != nil {
		t.Fatalf("stop stream to cancel context: %v", err)
	}

	select {
	case <-stream.done:

		// expected
	default:
		t.Error("expected stream context to be cancelled after stop")
	}
}

func TestStartRTCStream_WithAuthorizationContext(t *testing.T) {
	t.Parallel()

	handler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeSyntheticJSONResponse(t, w, tuyaSuccessResponse(map[string]any{
			rtcFixtureSessionID: "auth-ctx-session",
			rtcFixtureSdp:       "v=0\r\nauth-answer\r\n",
		}))
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()

	stream, err := client.StartRTCStream(ctx, StartRTCStreamRequest{
		Request: Request{
			AuthorizationContext: &AuthorizationContext{
				AccessToken:  rtcFixtureTestAccessToken,
				RefreshToken: rtcFixtureTestRefreshToken,
				ExpireTime:   time.Now().Add(time.Hour).UnixMilli(),
			},
		},
		DeviceID: rtcFixtureCamera001,
		SDPOffer: rtcFixtureV0Offer,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if stream.GetSessionID() != "auth-ctx-session" {
		t.Errorf("expected session ID %q, got %q", "auth-ctx-session", stream.GetSessionID())
	}
}

func TestStopRTCStream_Success(t *testing.T) {
	t.Parallel()

	callCount := 0
	handler := func(responseWriter http.ResponseWriter, r *http.Request) {
		callCount++

		responseWriter.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodDelete || callCount > 1 {
			writeSyntheticJSONResponse(t, responseWriter, tuyaSuccessResponse(true))

			return
		}

		// First call is StartRTCStream
		writeSyntheticJSONResponse(t, responseWriter, tuyaSuccessResponse(map[string]any{
			rtcFixtureSessionID: "stop-session",
			rtcFixtureSdp:       rtcFixtureV0Answer,
		}))
	}

	client, server := setupRTCTestClient(handler)
	defer server.Close()

	ctx := context.Background()

	err := client.StopRTCStream(ctx, StopRTCStreamRequest{
		DeviceID:  rtcFixtureCamera001,
		SessionID: "stop-session",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
