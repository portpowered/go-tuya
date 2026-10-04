package tuya

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)

// Repeated values stay test-local so synthetic fixtures remain independent of production constants.
const (
	authFixtureHaauthorize      = "haauthorize"
	authFixtureResult           = "result"
	authFixtureSuccess          = "success"
	authFixtureTestAccessCode   = "test-access-code"
	authFixtureTestAccessToken  = "test-access-token"
	authFixtureTestClientID     = "test-client-id"
	authFixtureExplicitRefresh  = "synthetic-explicit-refresh"
	authFixtureTestLoginCode    = "test-login-code"
	authFixtureTestRefreshToken = "test-refresh-token"
	authFixtureTestUserCode     = "test-user-code"
)

func writeSyntheticJSONResponse(t *testing.T, responseWriter http.ResponseWriter, value any) {
	t.Helper()

	err := json.NewEncoder(responseWriter).Encode(value)
	if err != nil {
		t.Errorf("encode synthetic JSON response: %v", err)
	}
}

func writeSyntheticResponseBody(t *testing.T, responseWriter http.ResponseWriter, body string) {
	t.Helper()

	_, err := responseWriter.Write([]byte(body))
	if err != nil {
		t.Errorf("write synthetic response body: %v", err)
	}
}

func syntheticQRCodeEnvelope(success bool) wire.QRCodeEnvelope {
	tid := "test-tid"
	timestamp := int64(1234567890)

	response := wire.QRCodeEnvelope{Success: &success, Tid: &tid, T: &timestamp}

	if success {
		code := "test-qr-code-token"
		response.Result = &wire.QRCodeResult{Qrcode: &code}
	}

	return response
}

func syntheticLoginCodeEnvelope(success bool) wire.LoginCodeEnvelope {
	tid := "test-tid"
	timestamp := int64(1234567890)

	response := wire.LoginCodeEnvelope{Success: &success, Tid: &tid, T: &timestamp}

	if success {
		accessToken := authFixtureTestAccessToken
		refreshToken := authFixtureTestRefreshToken
		expireTime := int64(7200)
		terminalID := "test-terminal-id"
		uid := "test-uid"
		username := "test-username"
		endpoint := "https://openapi.tuyaus.com"
		response.Result = &wire.LoginCodeResult{
			AccessToken:  &accessToken,
			RefreshToken: &refreshToken,
			ExpireTime:   &expireTime,
			TerminalId:   &terminalID,
			Uid:          &uid,
			Username:     &username,
			Endpoint:     &endpoint,
		}
	}

	return response
}

// setupAuthServiceWithMockServer creates an AuthService with a mocked HTTPS server.
func setupAuthServiceWithMockServer(handler http.HandlerFunc) (*AuthService, *httptest.Server) {
	server := httptest.NewTLSServer(handler)

	client := &Session{
		HTTPClient:        server.Client(),
		ClientID:          authFixtureTestClientID,
		AuthenticationURL: server.URL,
		CloudAPIURL:       server.URL,
	}
	client.EncryptedClient = &EncryptedClient{Client: client}
	client.SetTokens(Tokens{AccessToken: mqttFixtureAccessToken, RefreshToken: mqttFixtureRefreshToken})

	authService := &AuthService{client: client}

	return authService, server
}

func TestAuthService_GenerateQrCodeForLogin_Success(t *testing.T) {
	t.Parallel()

	// Mock successful response
	mockHandler := func(responseWriter http.ResponseWriter, request *http.Request) {
		// Verify request method and path
		if request.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", request.Method)
		}

		expectedPath := "/v1.0/m/life/home-assistant/qrcode/tokens"
		if !strings.Contains(request.URL.Path, expectedPath) {
			t.Errorf("Expected path to contain %s, got %s", expectedPath, request.URL.Path)
		}

		// Verify query parameters
		query := request.URL.Query()
		if query.Get("clientid") != authFixtureTestClientID {
			t.Errorf("Expected clientid=test-client-id, got %s", query.Get("clientid"))
		}

		if query.Get("usercode") != authFixtureTestAccessCode {
			t.Errorf("Expected usercode=test-access-code, got %s", query.Get("usercode"))
		}

		if query.Get("schema") != authFixtureHaauthorize {
			t.Errorf("Expected schema=haauthorize, got %s", query.Get("schema"))
		}

		// Return successful response
		response := syntheticQRCodeEnvelope(true)

		responseWriter.Header().Set("Content-Type", "application/json")
		writeSyntheticJSONResponse(t, responseWriter, response)
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	// Test the method
	req := LoginRequest{
		AccessCode: authFixtureTestAccessCode,
		Schema:     authFixtureHaauthorize,
	}

	ctx := context.Background()
	resp, err := authService.GenerateQrCodeForLogin(ctx, req)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Code != "test-qr-code-token" {
		t.Errorf("Expected Code=test-qr-code-token, got %s", resp.Code)
	}

	expectedQrFormatted := "tuyaSmart--qrLogin?token=test-qr-code-token"
	if resp.QrFormattedCode != expectedQrFormatted {
		t.Errorf("Expected QrFormattedCode=%s, got %s", expectedQrFormatted, resp.QrFormattedCode)
	}
}

func TestAuthService_GenerateQrCodeForLogin_DefaultSchema(t *testing.T) {
	t.Parallel()

	// Test that default schema is used when not provided
	mockHandler := func(responseWriter http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if query.Get("schema") != AuthenticationSchema {
			t.Errorf("Expected schema=%s, got %s", AuthenticationSchema, query.Get("schema"))
		}

		response := syntheticQRCodeEnvelope(true)

		responseWriter.Header().Set("Content-Type", "application/json")
		writeSyntheticJSONResponse(t, responseWriter, response)
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	// Test with empty schema - should use default
	req := LoginRequest{
		AccessCode: authFixtureTestAccessCode,
		Schema:     "", // Empty schema should use default
	}

	ctx := context.Background()

	_, err := authService.GenerateQrCodeForLogin(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestAuthService_GenerateQrCodeForLogin_Failure(t *testing.T) {
	t.Parallel()

	// Mock failure response
	mockHandler := func(w http.ResponseWriter, _ *http.Request) {
		response := syntheticQRCodeEnvelope(false)

		w.Header().Set("Content-Type", "application/json")
		writeSyntheticJSONResponse(t, w, response)
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	// Test the method
	req := LoginRequest{
		AccessCode: authFixtureTestAccessCode,
		Schema:     authFixtureHaauthorize,
	}

	ctx := context.Background()
	_, err := authService.GenerateQrCodeForLogin(ctx, req)

	// Should return an error for non-success response
	if err == nil {
		t.Fatal("Expected error for non-success response, got nil")
	}

	if !strings.Contains(err.Error(), "login failed") {
		t.Errorf("Expected error message to contain 'login failed', got %s", err.Error())
	}
}

func TestAuthService_GenerateQrCodeForLogin_MalformedJSON(t *testing.T) {
	t.Parallel()

	// Mock malformed JSON response
	mockHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeSyntheticResponseBody(t, w, "{invalid json")
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	req := LoginRequest{
		AccessCode: authFixtureTestAccessCode,
		Schema:     authFixtureHaauthorize,
	}

	ctx := context.Background()
	_, err := authService.GenerateQrCodeForLogin(ctx, req)

	// Should return JSON decode error
	if err == nil {
		t.Fatal("Expected error for malformed JSON, got nil")
	}
}

//nolint:cyclop,funlen // This success test checks the complete request/response contract, including optional returned account details.
func TestAuthService_ValidateLoginCode_Success(t *testing.T) {
	t.Parallel()

	// Mock successful validation response
	mockHandler := func(responseWriter http.ResponseWriter, request *http.Request) {
		// Verify request method and path
		if request.Method != http.MethodGet {
			t.Errorf("Expected GET request, got %s", request.Method)
		}

		expectedPath := "/v1.0/m/life/home-assistant/qrcode/tokens/test-login-code"
		if !strings.Contains(request.URL.Path, expectedPath) {
			t.Errorf("Expected path to contain %s, got %s", expectedPath, request.URL.Path)
		}

		// Verify query parameters
		query := request.URL.Query()
		if query.Get("clientid") != authFixtureTestClientID {
			t.Errorf("Expected clientid=test-client-id, got %s", query.Get("clientid"))
		}

		if query.Get("usercode") != authFixtureTestUserCode {
			t.Errorf("Expected usercode=test-user-code, got %s", query.Get("usercode"))
		}

		// Return successful response
		response := syntheticLoginCodeEnvelope(true)

		responseWriter.Header().Set("Content-Type", "application/json")
		writeSyntheticJSONResponse(t, responseWriter, response)
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	// Test the method
	req := ValidateLoginCodeRequest{
		LoginCode: authFixtureTestLoginCode,
		UserCode:  authFixtureTestUserCode,
	}

	ctx := context.Background()
	resp, err := authService.ValidateLoginCode(ctx, req)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !resp.Success {
		t.Error("Expected Success=true")
	}

	if resp.AccessToken != authFixtureTestAccessToken {
		t.Errorf("Expected AccessToken=test-access-token, got %s", resp.AccessToken)
	}

	if resp.RefreshToken != authFixtureTestRefreshToken {
		t.Errorf("Expected RefreshToken=test-refresh-token, got %s", resp.RefreshToken)
	}

	if resp.ExpireTime != 7200 {
		t.Errorf("Expected ExpireTime=7200, got %d", resp.ExpireTime)
	}

	if resp.TerminalID != "test-terminal-id" {
		t.Errorf("Expected TerminalID=test-terminal-id, got %s", resp.TerminalID)
	}

	if resp.UID != "test-uid" {
		t.Errorf("Expected UID=test-uid, got %s", resp.UID)
	}

	if resp.Username != "test-username" {
		t.Errorf("Expected Username=test-username, got %s", resp.Username)
	}

	if resp.Endpoint != "https://openapi.tuyaus.com" {
		t.Errorf("Expected Endpoint=https://openapi.tuyaus.com, got %s", resp.Endpoint)
	}
}

func TestAuthService_ValidateLoginCode_Failure(t *testing.T) {
	t.Parallel()

	// Mock failure response
	mockHandler := func(w http.ResponseWriter, _ *http.Request) {
		response := syntheticLoginCodeEnvelope(false)

		w.Header().Set("Content-Type", "application/json")
		writeSyntheticJSONResponse(t, w, response)
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	// Test the method
	req := ValidateLoginCodeRequest{
		LoginCode: "invalid-login-code",
		UserCode:  authFixtureTestUserCode,
	}

	ctx := context.Background()
	_, err := authService.ValidateLoginCode(ctx, req)

	// Should return an error for non-success response
	if err == nil {
		t.Fatal("Expected error for non-success response, got nil")
	}

	if !strings.Contains(err.Error(), "validate login code failed") {
		t.Errorf("Expected error message to contain 'validate login code failed', got %s", err.Error())
	}
}

func TestAuthService_ValidateLoginCode_MalformedJSON(t *testing.T) {
	t.Parallel()

	// Mock malformed JSON response
	mockHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeSyntheticResponseBody(t, w, "{invalid json")
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	req := ValidateLoginCodeRequest{
		LoginCode: authFixtureTestLoginCode,
		UserCode:  authFixtureTestUserCode,
	}

	ctx := context.Background()
	_, err := authService.ValidateLoginCode(ctx, req)

	// Should return JSON decode error
	if err == nil {
		t.Fatal("Expected error for malformed JSON, got nil")
	}
}

func TestAuthService_ValidateLoginCode_HTTPError(t *testing.T) {
	t.Parallel()

	// Mock HTTP error (server returns 500)
	mockHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeSyntheticResponseBody(t, w, "Internal Server Error")
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	req := ValidateLoginCodeRequest{
		LoginCode: authFixtureTestLoginCode,
		UserCode:  authFixtureTestUserCode,
	}

	ctx := context.Background()
	_, err := authService.ValidateLoginCode(ctx, req)

	// Should handle HTTP error gracefully
	if err == nil {
		t.Fatal("Expected error for HTTP 500, got nil")
	}
}

func TestAuthService_RefreshTokenReturnsRotatedTokensWithoutChangingSession(t *testing.T) {
	t.Parallel()

	var (
		requestPath        string
		requestAccessToken string
	)

	handler := func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		requestAccessToken = r.Header.Get("X-Token")
		w.Header().Set("Content-Type", "application/json")
		writeSyntheticJSONResponse(t, w, map[string]any{
			authFixtureSuccess: true,
			"t":                1700000000000,
			authFixtureResult: map[string]any{
				"expireTime":   7200,
				"uid":          "synthetic-user-id",
				"accessToken":  "synthetic-rotated-access",
				"refreshToken": "synthetic-rotated-refresh",
			},
		})
	}

	service, server := setupAuthServiceWithMockServer(handler)
	defer server.Close()

	oldTokens := Tokens{
		AccessToken:  "synthetic-current-access",
		RefreshToken: "synthetic-current-refresh",
		ExpireTime:   1700000000000,
	}
	service.client.SetTokens(oldTokens)

	got, err := service.RefreshToken(context.Background(), RefreshTokenRequest{RefreshToken: authFixtureExplicitRefresh})
	if err != nil {
		t.Fatalf("RefreshToken() error = %v", err)
	}

	if requestPath != "/v1.0/m/token/synthetic-explicit-refresh" {
		t.Errorf("refresh path = %q", requestPath)
	}

	if requestAccessToken != oldTokens.AccessToken {
		t.Errorf("request access token = %q, want explicit current session token", requestAccessToken)
	}

	want := RefreshTokenResponse{
		AccessToken:  "synthetic-rotated-access",
		RefreshToken: "synthetic-rotated-refresh",
		ExpireTime:   1700000000000 + 7200*1000,
		UID:          "synthetic-user-id",
		T:            1700000000000,
	}
	if got != want {
		t.Errorf("RefreshToken() = %+v, want %+v", got, want)
	}

	if current := service.client.Tokens(); current != oldTokens {
		t.Errorf("RefreshToken() changed session tokens to %+v; callers must apply replacements explicitly", current)
	}
}

func TestAuthService_GenerateQrCodeForLogin_HTTPError(t *testing.T) {
	t.Parallel()

	// Mock HTTP error (server returns 500)
	mockHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeSyntheticResponseBody(t, w, "Internal Server Error")
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	req := LoginRequest{
		AccessCode: authFixtureTestAccessCode,
		Schema:     authFixtureHaauthorize,
	}

	ctx := context.Background()
	_, err := authService.GenerateQrCodeForLogin(ctx, req)

	// Should handle HTTP error gracefully
	if err == nil {
		t.Fatal("Expected error for HTTP 500, got nil")
	}
}
