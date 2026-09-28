package tuya

import (
	"context"
	"fmt"
	"time"
)

// TokenProviderImpl provides token management functionality
type TokenProviderImpl struct {
	Client    *ClientImpl
	TokenInfo *CustomerTokenInfo
}

// GetAccessToken retrieves the current access token, refreshing if necessary
func (t *TokenProviderImpl) GetAccessToken(ctx context.Context, req GetAccessTokenRequest) (string, error) {
	if !req.DoNotRefreshToken {
		if err := t.refreshAccessTokenIfNeeded(ctx); err != nil {
			return "", err
		}
	}
	return t.TokenInfo.AccessToken, nil
}

// GetRefreshToken retrieves the current refresh token, refreshing if necessary
func (t *TokenProviderImpl) GetRefreshToken(ctx context.Context, req GetRefreshTokenRequest) (string, error) {
	if !req.DoNotRefreshToken {
		if err := t.refreshAccessTokenIfNeeded(ctx); err != nil {
			return "", err
		}
	}
	return t.TokenInfo.RefreshToken, nil
}

// SetToken sets the access token, refresh token, and expiry time
func (t *TokenProviderImpl) SetToken(accessToken string, refreshToken string, expireTime int64) {
	t.TokenInfo = &CustomerTokenInfo{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpireTime:   expireTime,
	}
}

// refreshAccessTokenIfNeeded refreshes the access token if needed
func (t *TokenProviderImpl) refreshAccessTokenIfNeeded(ctx context.Context) error {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	expiredTime := t.TokenInfo.ExpireTime

	// Refresh if token expires in less than 1 minute
	if expiredTime-60*1000 > now {
		return nil
	}

	// We don't want the refresh function to attempt to refresh the token here.
	response, err := t.Client.AuthService.RefreshToken(ctx, RefreshTokenRequest{
		RefreshToken: t.TokenInfo.RefreshToken,
		Request: Request{
			DoNotRefreshToken: true,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	t.TokenInfo = &CustomerTokenInfo{
		AccessToken:  response.AccessToken,
		RefreshToken: response.RefreshToken,
		ExpireTime:   response.ExpireTime,
		UID:          response.UID,
		T:            response.T,
	}

	return nil
}

// NewCustomerTokenInfo creates a new CustomerTokenInfo from response data
func NewCustomerTokenInfo(tokenData map[string]interface{}) *CustomerTokenInfo {
	info := &CustomerTokenInfo{}

	if t, ok := tokenData["t"].(float64); ok {
		info.T = int64(t)
	}
	if expireTime, ok := tokenData["expire_time"].(float64); ok {
		info.ExpireTime = info.T + int64(expireTime)*1000
	}
	if uid, ok := tokenData["uid"].(string); ok {
		info.UID = uid
	}
	if accessToken, ok := tokenData["access_token"].(string); ok {
		info.AccessToken = accessToken
	}
	if refreshToken, ok := tokenData["refresh_token"].(string); ok {
		info.RefreshToken = refreshToken
	}

	return info
}
