package tuya

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5" // #nosec G501 - MD5 is required by Tuya API protocol
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// EncryptedClient provides access to Tuya Customer API operations with encryption of the payload and response.
// The tuya API requires that payloads are encrypted with AES-GCM and the response is encrypted with AES-GCM.
type EncryptedClient struct {
	Client *ClientImpl
}

// Get performs an encrypted GET request
func (c *EncryptedClient) Get(ctx context.Context, path string, params map[string]interface{}, operationRequest OperationRequest) (*EncryptedAPIResponse, error) {
	return c.makeRequest(ctx, "GET", path, params, nil, operationRequest)
}

// Post performs an encrypted POST request
func (c *EncryptedClient) Post(ctx context.Context, path string, params, body map[string]interface{}, operationRequest OperationRequest) (*EncryptedAPIResponse, error) {
	return c.makeRequest(ctx, "POST", path, params, body, operationRequest)
}

// Put performs an encrypted PUT request
func (c *EncryptedClient) Put(ctx context.Context, path string, body map[string]interface{}, operationRequest OperationRequest) (*EncryptedAPIResponse, error) {
	return c.makeRequest(ctx, "PUT", path, nil, body, operationRequest)
}

// Delete performs an encrypted DELETE request
func (c *EncryptedClient) Delete(ctx context.Context, path string, params map[string]interface{}, operationRequest OperationRequest) (*EncryptedAPIResponse, error) {
	return c.makeRequest(ctx, "DELETE", path, params, nil, operationRequest)
}

// makeRequest performs the actual encrypted HTTP request
// https://developer.tuya.com/en/docs/iot/new-singnature
// The actual request is wrapped around roughly like:
// curl -X GET tuyawebsite.com/api/blah/blah?encdata={encrypted query params} --body {encdata: encrypted request body} --header "X-appKey: 123" --header "X-requestId: 123"
// --header "X-sid: 123" --header "X-time: 123" --header "X-token: 123" --header "X-sign: 123"
// Then the response looks like
// { result: "encrypted response data", success: true, code: 200, msg: "success", t: 123, tid: "123" }
func (c *EncryptedClient) makeRequest(ctx context.Context, method, path string, params, body map[string]interface{}, operationRequest OperationRequest) (*EncryptedAPIResponse, error) {
	// Generate request ID and secret
	rid := GenerateRID()
	sid := ""

	var token string
	var err error
	var authContext *AuthorizationContext

	// Extract authorization context from request if provided
	if operationRequest != nil {
		req := operationRequest.GetRequest()
		authContext = req.AuthorizationContext
	}

	// Use authorization context if provided, otherwise fall back to TokenProvider
	if authContext != nil && authContext.RefreshToken != "" {
		token = authContext.RefreshToken
	} else {
		token, err = c.Client.TokenProvider.GetRefreshToken(ctx, GetRefreshTokenRequest{
			Request: operationRequest.GetRequest(),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get refresh token: %w", err)
		}
	}

	// Create hash key: MD5(rid + refresh_token)
	// #nosec G401 - MD5 is required by Tuya API protocol
	h := md5.New()
	h.Write([]byte(rid + token))
	hashKey := hex.EncodeToString(h.Sum(nil))

	// Generate secret
	secret := secretGenerating(rid, sid, hashKey)

	// Encrypt params and body
	var queryEncdata, bodyEncdata string
	var finalParams map[string]interface{}
	var finalBody map[string]interface{}

	if len(params) > 0 {
		queryJSON := formToJSON(params)
		encrypted, err := aesGCMEncrypt(queryJSON, secret)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt params: %w", err)
		}
		queryEncdata = string(encrypted)
		finalParams = map[string]interface{}{
			"encdata": queryEncdata,
		}
	}

	if len(body) > 0 {
		bodyJSON := formToJSON(body)
		encrypted, err := aesGCMEncrypt(bodyJSON, secret)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt body: %w", err)
		}
		bodyEncdata = string(encrypted)
		finalBody = map[string]interface{}{
			"encdata": bodyEncdata,
		}
	}

	// Create headers
	t := time.Now().UnixNano() / int64(time.Millisecond)
	headers := map[string]string{
		"X-appKey":    c.Client.ClientID,
		"X-requestId": rid,
		"X-sid":       sid,
		"X-time":      strconv.FormatInt(t, 10),
	}

	var accessToken string
	// Use authorization context if provided, otherwise fall back to TokenProvider
	if authContext != nil && authContext.AccessToken != "" {
		accessToken = authContext.AccessToken
	} else {
		accessToken, err = c.Client.TokenProvider.GetAccessToken(ctx, GetAccessTokenRequest{
			Request: operationRequest.GetRequest(),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get access token: %w", err)
		}
	}

	if accessToken != "" {
		headers["X-token"] = accessToken
	}

	// Generate signature
	sign := restfulSign(hashKey, queryEncdata, bodyEncdata, headers)
	headers["X-sign"] = sign

	// Build URL
	url := c.Client.CloudAPIURL + path

	// Create HTTP request
	var reqBody io.Reader
	if finalBody != nil {
		bodyBytes, err := json.Marshal(finalBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		reqBody = bytes.NewBuffer(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	q := req.URL.Query()
	if finalParams != nil {
		// Convert params to URL query string
		q.Add("encdata", queryEncdata)
	}

	req.URL.RawQuery = q.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Make request
	resp, err := c.Client.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close response body: %v", err)
			// Log error but don't override return error
		}
	}()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response error: code=%d, content=%s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var responseData map[string]interface{}
	if err := json.Unmarshal(respBody, &responseData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check success
	success, _ := responseData["success"].(bool)
	if !success {
		code, _ := responseData["code"].(string)
		msg, _ := responseData["msg"].(string)
		// map[string]interface {} ["code": "1010", "msg": "token is expired", "t": 1753502581085, "tid": "1231", "success": false, ]
		return nil, fmt.Errorf("network error: (%s) %s", code, msg)
	}

	// Decrypt result
	if encResult, ok := responseData["result"].(string); ok {
		decrypted, err := aesGCMDecrypt(encResult, secret)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt response: %w", err)
		}

		// Try to parse as JSON
		var parsedResult interface{}
		if err := json.Unmarshal([]byte(decrypted), &parsedResult); err != nil {
			// If not JSON, use as string
			responseData["result"] = decrypted
		} else {
			responseData["result"] = parsedResult
		}
	}

	// Build response object
	response := &EncryptedAPIResponse{
		StatusCode: resp.StatusCode,
		Body:       responseData,
		Success:    success,
	}

	if code, ok := responseData["code"].(float64); ok {
		response.Code = int(code)
	}
	if msg, ok := responseData["msg"].(string); ok {
		response.Message = msg
	}
	if t, ok := responseData["t"].(float64); ok {
		response.Time = int64(t)
	}

	return response, nil
}

// GenerateRID generates a UUID-based request ID
func GenerateRID() string {
	return uuid.New().String()
}

// randomNonce generates a random nonce string
func randomNonce(length int) string {
	const charset = "ABCDEFGHJKMNPQRSTWXYZabcdefhijkmnprstwxyz2345678"
	result := make([]byte, length)
	for i := range result {
		randBytes := make([]byte, 1)
		if _, err := rand.Read(randBytes); err != nil {
			// Fallback to less secure option if crypto/rand fails
			result[i] = charset[len(charset)/2] // Use middle character as fallback
		} else {
			result[i] = charset[int(randBytes[0])%len(charset)]
		}
	}
	return string(result)
}

// formToJSON converts map to JSON string
func formToJSON(content map[string]interface{}) string {
	if content == nil {
		return ""
	}
	jsonBytes, _ := json.Marshal(content)
	return string(jsonBytes)
}

// aesGCMEncrypt encrypts data using AES-GCM with base64 encoding
func aesGCMEncrypt(rawData, secret string) ([]byte, error) {
	nonce := randomNonce(12)
	// Convert to bytes
	data := []byte(rawData)
	key := []byte(secret)
	nonceBytes := []byte(nonce)

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, nonceBytes, data, nil)

	// Encode nonce and ciphertext separately then concatenate
	nonceB64 := base64.StdEncoding.EncodeToString(nonceBytes)
	ciphertextB64 := base64.StdEncoding.EncodeToString(ciphertext)

	return []byte(nonceB64 + ciphertextB64), nil
}

// aesGCMDecrypt decrypts AES-GCM encrypted data
func aesGCMDecrypt(cipherData, secret string) (string, error) {
	// Decode base64
	cipherBytes, err := base64.StdEncoding.DecodeString(cipherData)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Split nonce and ciphertext (first 12 bytes are nonce)
	if len(cipherBytes) < 12 {
		return "", fmt.Errorf("cipher data too short")
	}

	nonce := cipherBytes[:12]
	ciphertext := cipherBytes[12:]

	// Create AES-GCM cipher
	key := []byte(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// secretGenerating generates the secret key for encryption
func secretGenerating(rid, sid, hashKey string) string {
	message := hashKey
	mod := 16

	if sid != "" {
		sidLength := len(sid)
		length := sidLength
		if sidLength > mod {
			length = mod
		}

		ecode := ""
		for i := 0; i < length; i++ {
			idx := int(sid[i]) % mod
			if idx < len(sid) {
				ecode += string(sid[idx])
			}
		}
		message += "_" + ecode
	}

	// Create HMAC-SHA256
	h := hmac.New(sha256.New, []byte(rid))
	h.Write([]byte(message))
	byteTemp := h.Sum(nil)
	secret := hex.EncodeToString(byteTemp)

	// Return first 16 characters
	if len(secret) > 16 {
		return secret[:16]
	}
	return secret
}

// restfulSign generates the signature for the request
func restfulSign(hashKey, queryEncdata, bodyEncdata string, headers map[string]string) string {
	headerKeys := []string{"X-appKey", "X-requestId", "X-sid", "X-time", "X-token"}
	headerSignStr := ""

	for _, key := range headerKeys {
		if val, exists := headers[key]; exists && val != "" {
			headerSignStr += key + "=" + val + "||"
		}
	}

	// Remove last "||"
	if len(headerSignStr) > 2 {
		headerSignStr = headerSignStr[:len(headerSignStr)-2]
	}

	signStr := headerSignStr
	if queryEncdata != "" {
		signStr += queryEncdata
	}
	if bodyEncdata != "" {
		signStr += bodyEncdata
	}

	// Create HMAC-SHA256 signature
	h := hmac.New(sha256.New, []byte(hashKey))
	h.Write([]byte(signStr))
	return hex.EncodeToString(h.Sum(nil))
}

// EncryptedAPIResponse represents an encrypted customer API response
type EncryptedAPIResponse struct {
	StatusCode int                    `json:"status_code"`
	Headers    map[string]string      `json:"headers,omitempty"`
	Body       map[string]interface{} `json:"body,omitempty"`
	Success    bool                   `json:"success"`
	Code       int                    `json:"code,omitempty"`
	Message    string                 `json:"msg,omitempty"`
	Time       int64                  `json:"t,omitempty"`
}
