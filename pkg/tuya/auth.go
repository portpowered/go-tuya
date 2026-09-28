package tuya

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/portpowered/go-tuya/pkg/tuya/internal/wire"
)

// AuthService is the service used to generate the QR code, and then validate the code to get the access token.
type AuthService service

// RefreshToken refreshes the access token using the provided refresh token.
// Example output:
// "{\
// "t\":123123,\
// "sign\":\"1231\",
// \"tid\":\"1231\"
// ,\"success\":true,
// \"result\":\"1"}"
func (c *AuthService) RefreshToken(ctx context.Context, req RefreshTokenRequest) (RefreshTokenResponse, error) {
	// This is an explicit caller-requested exchange. Sign it with the supplied
	// refresh token and the session's current access token; do not refresh first.
	req.AuthorizationContext = &AuthorizationContext{
		AccessToken:  c.client.Tokens().AccessToken,
		RefreshToken: req.RefreshToken,
	}

	response, err := c.client.EncryptedClient.Get(ctx, fmt.Sprintf(wire.RouteRefreshAccessToken, req.RefreshToken), nil, &req)
	if err != nil {
		return RefreshTokenResponse{}, fmt.Errorf("failed to refresh token: %w", err)
	}

	if success, ok := response.Body["success"].(bool); ok && success {
		if result, ok := response.Body["result"].(map[string]interface{}); ok {
			tokenInfo := map[string]interface{}{
				"t":             response.Body["t"],
				"expire_time":   result["expireTime"],
				"uid":           result["uid"],
				"access_token":  result["accessToken"],
				"refresh_token": result["refreshToken"],
			}
			info := NewCustomerTokenInfo(tokenInfo)
			return RefreshTokenResponse{
				AccessToken:  info.AccessToken,
				RefreshToken: info.RefreshToken,
				ExpireTime:   info.ExpireTime,
				UID:          info.UID,
				T:            info.T,
			}, nil
		}
	}
	return RefreshTokenResponse{}, fmt.Errorf("failed to refresh token: %+v", response)
}

// RefreshTokenRequest represents a request to refresh an access token
type RefreshTokenRequest struct {
	Request
	RefreshToken string `json:"refresh_token"`
}

// GetRequest returns the underlying Request for the RefreshTokenRequest
func (r *RefreshTokenRequest) GetRequest() Request {
	return r.Request
}

// RefreshTokenResponse represents the response from a token refresh request
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	// Time of expiry in time since epoch format.
	ExpireTime int64  `json:"expire_time"`
	UID        string `json:"uid"`
	T          int64  `json:"t"`
}

// GenerateQrCodeForLogin generates a QR code for login authentication
func (c *AuthService) GenerateQrCodeForLogin(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	schema := req.Schema
	if schema == "" {
		schema = AuthenticationSchema
	}
	url := fmt.Sprintf("%s%s?clientid=%s&usercode=%s&schema=%s",
		c.client.AuthenticationURL, wire.RouteGenerateLoginQRCode, c.client.ClientID, req.AccessCode, schema)
	httpRequest, err := http.NewRequestWithContext(ctx, wire.MethodGenerateLoginQRCode, url, nil)
	if err != nil {
		return LoginResponse{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := c.client.HTTPClient.Do(httpRequest)
	if err != nil {
		return LoginResponse{}, err
	}

	defer func() {
		_ = response.Body.Close()
	}()
	var loginResponse tuyaCloudLoginResponse
	err = json.NewDecoder(response.Body).Decode(&loginResponse)
	if err != nil {
		return LoginResponse{}, err
	}

	if !loginResponse.Success {
		return LoginResponse{}, fmt.Errorf("login failed: %+v", loginResponse)
	}

	// Possible response:
	// Failed due to wrong URI: "{\"code\":\"-9999999\",\"msg\":\"app param is invalid\",\"t\":123,\"tid\":\"123\",\"success\":false}"
	// Success: {"success":true,"tid":"123","t":123,"result":{"qrcode":"123"}}
	// "https://openapi.tuyaus.com/v1.0/m/life/home-assistant/qrcode/tokens?clientid=123&usercode=12312&schema=haauthorize"
	// The QRlogin has to be prefixed with: f"tuyaSmart--qrLogin?token=
	// See: https://github.com/home-assistant/core/blob/dev/homeassistant/components/tuya/config_flow.py#L48
	return LoginResponse{
		Code:            loginResponse.Result.Qrcode,
		QrFormattedCode: fmt.Sprintf("tuyaSmart--qrLogin?token=%s", loginResponse.Result.Qrcode),
	}, nil
}

// ValidateLoginCode validates the login code and returns the access token.
// Input example:
// curl -X GET https://apigw.iotbing.com/v1.0/m/life/home-assistant/qrcode/tokens/code?clientid=12312312&usercode=12312
// Output example:
// {"success":true,"tid":"12312","t":123123,
// "result":{"access_token":"1231","refresh_token":"123123","expire_time":7200,
// "terminal_id":"1231231","uid":"1231","username":"123123","endpoint":"https://apigw.tuyaus.com"}}
func (c *AuthService) ValidateLoginCode(ctx context.Context, req ValidateLoginCodeRequest) (ValidateLoginCodeResponse, error) {
	url := fmt.Sprintf("%s%s?clientid=%s&usercode=%s",
		c.client.AuthenticationURL, fmt.Sprintf(wire.RouteValidateLoginCode, req.LoginCode), c.client.ClientID, req.UserCode)

	httpRequest, err := http.NewRequestWithContext(ctx, wire.MethodValidateLoginCode, url, nil)
	if err != nil {
		return ValidateLoginCodeResponse{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := c.client.HTTPClient.Do(httpRequest)
	if err != nil {
		return ValidateLoginCodeResponse{}, err
	}

	defer func() {
		_ = response.Body.Close()
	}()
	var validateLoginCodeResponse tuyaCloudValidateLoginCodeResponse
	err = json.NewDecoder(response.Body).Decode(&validateLoginCodeResponse)
	if err != nil {
		return ValidateLoginCodeResponse{}, err
	}

	if !validateLoginCodeResponse.Success {
		return ValidateLoginCodeResponse{}, fmt.Errorf("validate login code failed: %+v", validateLoginCodeResponse)
	}

	return ValidateLoginCodeResponse{
		Success:      true,
		AccessToken:  validateLoginCodeResponse.Result.AccessToken,
		RefreshToken: validateLoginCodeResponse.Result.RefreshToken,
		ExpireTime:   validateLoginCodeResponse.Result.ExpireTime,
		TerminalID:   validateLoginCodeResponse.Result.TerminalID,
		UID:          validateLoginCodeResponse.Result.UID,
		Username:     validateLoginCodeResponse.Result.Username,
		Endpoint:     validateLoginCodeResponse.Result.Endpoint,
	}, nil
}

// tuyaCloudValidateLoginCodeResponse represents the response from validating a login code
type tuyaCloudValidateLoginCodeResponse struct {
	Success bool   `json:"success"`
	Tid     string `json:"tid"`
	T       int64  `json:"t"`
	Result  struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpireTime   int64  `json:"expire_time"`
		TerminalID   string `json:"terminal_id"`
		UID          string `json:"uid"`
		Username     string `json:"username"`
		Endpoint     string `json:"endpoint"`
	}
}

// tuyaCloudLoginResponse represents the response from generating a login QR code to be scanned by the Tuya Smart app.
type tuyaCloudLoginResponse struct {
	Success bool   `json:"success"`
	Tid     string `json:"tid"`
	T       int64  `json:"t,omitempty"`
	Result  struct {
		Qrcode string `json:"qrcode"`
	} `json:"result"`
}
