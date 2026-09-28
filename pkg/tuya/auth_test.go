package tuya

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// setupAuthServiceWithMockServer creates an AuthService with a mocked HTTPS server
func setupAuthServiceWithMockServer(handler http.HandlerFunc) (*AuthService, *httptest.Server) {
	server := httptest.NewTLSServer(handler)

	client := &Session{
		HTTPClient:        server.Client(),
		ClientID:          "test-client-id",
		AuthenticationURL: server.URL,
		CloudAPIURL:       server.URL,
	}
	client.EncryptedClient = &EncryptedClient{Client: client}
	client.SetTokens(Tokens{AccessToken: "synthetic-access-token", RefreshToken: "synthetic-refresh-token"})

	authService := &AuthService{client: client}
	return authService, server
}

func TestAuthService_GenerateQrCodeForLogin_Success(t *testing.T) {
	// Mock successful response
	mockHandler := func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		expectedPath := "/v1.0/m/life/home-assistant/qrcode/tokens"
		if !strings.Contains(r.URL.Path, expectedPath) {
			t.Errorf("Expected path to contain %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify query parameters
		query := r.URL.Query()
		if query.Get("clientid") != "test-client-id" {
			t.Errorf("Expected clientid=test-client-id, got %s", query.Get("clientid"))
		}
		if query.Get("usercode") != "test-access-code" {
			t.Errorf("Expected usercode=test-access-code, got %s", query.Get("usercode"))
		}
		if query.Get("schema") != "haauthorize" {
			t.Errorf("Expected schema=haauthorize, got %s", query.Get("schema"))
		}

		// Return successful response
		response := tuyaCloudLoginResponse{
			Success: true,
			Tid:     "test-tid",
			T:       1234567890,
			Result: struct {
				Qrcode string `json:"qrcode"`
			}{
				Qrcode: "test-qr-code-token",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	// Test the method
	req := LoginRequest{
		AccessCode: "test-access-code",
		Schema:     "haauthorize",
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
	// Test that default schema is used when not provided
	mockHandler := func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("schema") != AuthenticationSchema {
			t.Errorf("Expected schema=%s, got %s", AuthenticationSchema, query.Get("schema"))
		}

		response := tuyaCloudLoginResponse{
			Success: true,
			Tid:     "test-tid",
			T:       1234567890,
			Result: struct {
				Qrcode string `json:"qrcode"`
			}{
				Qrcode: "test-qr-code-token",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	// Test with empty schema - should use default
	req := LoginRequest{
		AccessCode: "test-access-code",
		Schema:     "", // Empty schema should use default
	}

	ctx := context.Background()
	_, err := authService.GenerateQrCodeForLogin(ctx, req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestAuthService_GenerateQrCodeForLogin_Failure(t *testing.T) {
	// Mock failure response
	mockHandler := func(w http.ResponseWriter, r *http.Request) {
		response := tuyaCloudLoginResponse{
			Success: false,
			Tid:     "test-tid",
			T:       1234567890,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	// Test the method
	req := LoginRequest{
		AccessCode: "test-access-code",
		Schema:     "haauthorize",
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
	// Mock malformed JSON response
	mockHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{invalid json"))
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	req := LoginRequest{
		AccessCode: "test-access-code",
		Schema:     "haauthorize",
	}

	ctx := context.Background()
	_, err := authService.GenerateQrCodeForLogin(ctx, req)

	// Should return JSON decode error
	if err == nil {
		t.Fatal("Expected error for malformed JSON, got nil")
	}
}

func TestAuthService_ValidateLoginCode_Success(t *testing.T) {
	// Mock successful validation response
	mockHandler := func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		expectedPath := "/v1.0/m/life/home-assistant/qrcode/tokens/test-login-code"
		if !strings.Contains(r.URL.Path, expectedPath) {
			t.Errorf("Expected path to contain %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify query parameters
		query := r.URL.Query()
		if query.Get("clientid") != "test-client-id" {
			t.Errorf("Expected clientid=test-client-id, got %s", query.Get("clientid"))
		}
		if query.Get("usercode") != "test-user-code" {
			t.Errorf("Expected usercode=test-user-code, got %s", query.Get("usercode"))
		}

		// Return successful response
		response := tuyaCloudValidateLoginCodeResponse{
			Success: true,
			Tid:     "test-tid",
			T:       1234567890,
			Result: struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
				ExpireTime   int64  `json:"expire_time"`
				TerminalID   string `json:"terminal_id"`
				UID          string `json:"uid"`
				Username     string `json:"username"`
				Endpoint     string `json:"endpoint"`
			}{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
				ExpireTime:   7200,
				TerminalID:   "test-terminal-id",
				UID:          "test-uid",
				Username:     "test-username",
				Endpoint:     "https://openapi.tuyaus.com",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	// Test the method
	req := ValidateLoginCodeRequest{
		LoginCode: "test-login-code",
		UserCode:  "test-user-code",
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

	if resp.AccessToken != "test-access-token" {
		t.Errorf("Expected AccessToken=test-access-token, got %s", resp.AccessToken)
	}

	if resp.RefreshToken != "test-refresh-token" {
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
	// Mock failure response
	mockHandler := func(w http.ResponseWriter, r *http.Request) {
		response := tuyaCloudValidateLoginCodeResponse{
			Success: false,
			Tid:     "test-tid",
			T:       1234567890,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	// Test the method
	req := ValidateLoginCodeRequest{
		LoginCode: "invalid-login-code",
		UserCode:  "test-user-code",
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
	// Mock malformed JSON response
	mockHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{invalid json"))
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	req := ValidateLoginCodeRequest{
		LoginCode: "test-login-code",
		UserCode:  "test-user-code",
	}

	ctx := context.Background()
	_, err := authService.ValidateLoginCode(ctx, req)

	// Should return JSON decode error
	if err == nil {
		t.Fatal("Expected error for malformed JSON, got nil")
	}
}

func TestAuthService_ValidateLoginCode_HTTPError(t *testing.T) {
	// Mock HTTP error (server returns 500)
	mockHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	req := ValidateLoginCodeRequest{
		LoginCode: "test-login-code",
		UserCode:  "test-user-code",
	}

	ctx := context.Background()
	_, err := authService.ValidateLoginCode(ctx, req)

	// Should handle HTTP error gracefully
	if err == nil {
		t.Fatal("Expected error for HTTP 500, got nil")
	}
}

func TestAuthService_RefreshTokenReturnsRotatedTokensWithoutChangingSession(t *testing.T) {
	var requestPath string
	var requestAccessToken string
	handler := func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		requestAccessToken = r.Header.Get("X-token")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"t":       1700000000000,
			"result": map[string]interface{}{
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

	got, err := service.RefreshToken(context.Background(), RefreshTokenRequest{RefreshToken: "synthetic-explicit-refresh"})
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
	// Mock HTTP error (server returns 500)
	mockHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}

	authService, server := setupAuthServiceWithMockServer(mockHandler)
	defer server.Close()

	req := LoginRequest{
		AccessCode: "test-access-code",
		Schema:     "haauthorize",
	}

	ctx := context.Background()
	_, err := authService.GenerateQrCodeForLogin(ctx, req)

	// Should handle HTTP error gracefully
	if err == nil {
		t.Fatal("Expected error for HTTP 500, got nil")
	}
}
