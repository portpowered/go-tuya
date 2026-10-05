package tuya

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
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
// \"result\":\"1"}".
func (c *AuthService) RefreshToken(ctx context.Context, req RefreshTokenRequest) (RefreshTokenResponse, error) {
	// This is an explicit caller-requested exchange. Sign it with the supplied
	// refresh token and the session's current access token; do not refresh first.
	req.AuthorizationContext = &AuthorizationContext{
		AccessToken:  c.client.Tokens().AccessToken,
		RefreshToken: req.RefreshToken,
		ExpireTime:   0,
	}

	response, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationRefreshAccessToken(), []any{req.RefreshToken}, nil, nil, &req)
	if err != nil {
		return RefreshTokenResponse{}, fmt.Errorf("failed to refresh token: %w", err)
	}

	wireResponse, err := decodeWireResponse[wire.RefreshTokenEnvelope](response)
	if err != nil {
		return RefreshTokenResponse{}, clientError(ErrorProtocol, fmt.Errorf("failed to decode refresh response: %w", err))
	}

	if dereference(wireResponse.Success) && wireResponse.Result != nil {
		timestamp := dereference(wireResponse.T)
		expirySeconds := dereference(wireResponse.Result.ExpireTime)

		return RefreshTokenResponse{
			AccessToken:  dereference(wireResponse.Result.AccessToken),
			RefreshToken: dereference(wireResponse.Result.RefreshToken),
			// The refresh endpoint returns expireTime as a duration in seconds.
			// The public RefreshTokenResponse has historically exposed its
			// expiry as an epoch timestamp in milliseconds.
			ExpireTime: timestamp + expirySeconds*1000,
			UID:        dereference(wireResponse.Result.Uid),
			T:          timestamp,
		}, nil
	}

	return RefreshTokenResponse{}, clientError(ErrorProvider, fmt.Errorf("%w: %+v", errRefreshTokenFailure, response))
}

// RefreshTokenRequest represents a request to refresh an access token.
type RefreshTokenRequest struct {
	Request

	RefreshToken string `json:"refresh_token"`
}

// GetRequest returns the underlying Request for the RefreshTokenRequest.
func (r *RefreshTokenRequest) GetRequest() Request {
	return r.Request
}

// RefreshTokenResponse represents the response from a token refresh request.
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	// Time of expiry in time since epoch format.
	ExpireTime int64  `json:"expire_time"`
	UID        string `json:"uid"`
	T          int64  `json:"t"`
}

// GenerateQrCodeForLogin generates a QR code for login authentication.
func (c *AuthService) GenerateQrCodeForLogin(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	schema := strings.TrimSpace(req.Schema)
	if schema == "" {
		return LoginResponse{}, clientError(ErrorInvalidOperation, errAuthenticationSchemaRequired)
	}

	query, err := wireQueryValues(wire.GenerateLoginQRCodeParams{
		Clientid:    c.client.ClientID,
		Usercode:    req.AccessCode,
		Schema:      schema,
		ContentType: wire.JSONMediaTypeApplicationJSON,
	})
	if err != nil {
		return LoginResponse{}, clientError(ErrorProtocol, err)
	}

	response, err := doHTTP(
		ctx,
		c.client.HTTPClient,
		c.client.AuthenticationURL,
		wire.OperationGenerateLoginQRCode(),
		nil,
		query,
		map[string]string{wire.HeaderContentType: string(wire.JSONMediaTypeApplicationJSON)},
		nil,
	)
	if err != nil {
		return LoginResponse{}, err
	}

	defer func() {
		_ = response.Body.Close()
	}()

	var loginResponse wire.QRCodeEnvelope

	err = json.NewDecoder(response.Body).Decode(&loginResponse)
	if err != nil {
		return LoginResponse{}, clientError(ErrorProtocol, err)
	}

	if !dereference(loginResponse.Success) {
		return LoginResponse{}, clientError(ErrorProvider, fmt.Errorf("%w: %+v", errLoginFailure, loginResponse))
	}

	var qrCode string
	if loginResponse.Result != nil {
		qrCode = dereference(loginResponse.Result.Qrcode)
	}

	return LoginResponse{
		Code:            qrCode,
		QrFormattedCode: string(wire.QRCodeTokenPrefixSmartLife) + qrCode,
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
	query, err := wireQueryValues(wire.ValidateLoginCodeParams{
		Clientid:    c.client.ClientID,
		Usercode:    req.UserCode,
		ContentType: wire.JSONMediaTypeApplicationJSON,
	})
	if err != nil {
		return ValidateLoginCodeResponse{}, clientError(ErrorProtocol, err)
	}

	response, err := doHTTP(
		ctx,
		c.client.HTTPClient,
		c.client.AuthenticationURL,
		wire.OperationValidateLoginCode(),
		[]any{req.LoginCode},
		query,
		map[string]string{wire.HeaderContentType: string(wire.JSONMediaTypeApplicationJSON)},
		nil,
	)
	if err != nil {
		return ValidateLoginCodeResponse{}, err
	}

	defer func() {
		_ = response.Body.Close()
	}()

	var validateLoginCodeResponse wire.LoginCodeEnvelope

	err = json.NewDecoder(response.Body).Decode(&validateLoginCodeResponse)
	if err != nil {
		return ValidateLoginCodeResponse{}, clientError(ErrorProtocol, err)
	}

	if !dereference(validateLoginCodeResponse.Success) || validateLoginCodeResponse.Result == nil {
		return ValidateLoginCodeResponse{}, clientError(ErrorProvider, fmt.Errorf("%w: %+v", errLoginCodeValidationFailure, validateLoginCodeResponse))
	}

	return ValidateLoginCodeResponse{
		Success:      dereference(validateLoginCodeResponse.Success),
		AccessToken:  dereference(validateLoginCodeResponse.Result.AccessToken),
		RefreshToken: dereference(validateLoginCodeResponse.Result.RefreshToken),
		ExpireTime:   dereference(validateLoginCodeResponse.Result.ExpireTime),
		TerminalID:   dereference(validateLoginCodeResponse.Result.TerminalId),
		UID:          dereference(validateLoginCodeResponse.Result.Uid),
		Username:     dereference(validateLoginCodeResponse.Result.Username),
		Endpoint:     dereference(validateLoginCodeResponse.Result.Endpoint),
	}, nil
}
