package tuya

// NewCustomerTokenInfo converts a token response map to the public token
// representation. The provider's expire_time value is a duration in seconds.
func NewCustomerTokenInfo(tokenData map[string]interface{}) *CustomerTokenInfo {
	info := &CustomerTokenInfo{}
	if value, ok := tokenData["t"].(float64); ok {
		info.T = int64(value)
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
