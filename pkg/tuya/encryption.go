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
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/portpowered/go-tuya/pkg/tuya/internal/wire"
)

const (
	aesGCMNonceSize          = 12
	derivedSecretKeySize     = 16
	signatureHeaderSeparator = "||"
)

// EncryptedClient provides access to Tuya Customer API operations with encryption of the payload and response.
// The tuya API requires that payloads are encrypted with AES-GCM and the response is encrypted with AES-GCM.
type EncryptedClient struct {
	Client *Session
}

// Get performs an encrypted GET request.
func (c *EncryptedClient) Get(ctx context.Context, path string, params map[string]any, operationRequest OperationRequest) (*EncryptedAPIResponse, error) {
	return c.makeRequest(ctx, "GET", path, params, nil, operationRequest)
}

// Post performs an encrypted POST request.
func (c *EncryptedClient) Post(
	ctx context.Context,
	path string,
	params, body map[string]any,
	operationRequest OperationRequest,
) (*EncryptedAPIResponse, error) {
	return c.makeRequest(ctx, "POST", path, params, body, operationRequest)
}

// Put performs an encrypted PUT request.
func (c *EncryptedClient) Put(ctx context.Context, path string, body map[string]any, operationRequest OperationRequest) (*EncryptedAPIResponse, error) {
	return c.makeRequest(ctx, "PUT", path, nil, body, operationRequest)
}

// Delete performs an encrypted DELETE request.
func (c *EncryptedClient) Delete(ctx context.Context, path string, params map[string]any, operationRequest OperationRequest) (*EncryptedAPIResponse, error) {
	return c.makeRequest(ctx, "DELETE", path, params, nil, operationRequest)
}

// requestOperation dispatches an inventoried operation using its generated
// method and path as one unit.
func (c *EncryptedClient) requestOperation(
	ctx context.Context,
	operation wire.Operation,
	pathArgs []any,
	params, body map[string]any,
	request OperationRequest,
) (*EncryptedAPIResponse, error) {
	path := operation.Path
	if len(pathArgs) > 0 {
		path = fmt.Sprintf(path, pathArgs...)
	}

	return c.makeRequest(ctx, operation.Method, path, params, body, request)
}

// makeRequest performs the actual encrypted HTTP request
// https://developer.tuya.com/en/docs/iot/new-singnature
// Requests carry encrypted query parameters or body data in encdata, with
// X-appKey, X-requestId, X-sid, X-time, X-token, and X-sign headers.
// Responses contain a result, success, code, message, timestamp, and request ID.
func (c *EncryptedClient) makeRequest(
	ctx context.Context,
	method, path string,
	params, body map[string]any,
	operationRequest OperationRequest,
) (*EncryptedAPIResponse, error) {
	if !wire.IsKnownOperation(method, path) {
		return nil, clientError(ErrorInvalidOperation, fmt.Errorf("operation %s %s %w", method, path, errUnschematizedOperation))
	}

	rid := GenerateRID()
	sid := ""
	authContext := requestAuthorizationContext(operationRequest)
	tokens := c.Client.Tokens()

	refreshToken, err := requiredRefreshToken(authContext, tokens)
	if err != nil {
		return nil, err
	}

	// #nosec G401 - MD5 is required by Tuya API protocol
	h := md5.New()
	h.Write([]byte(rid + refreshToken))
	hashKey := hex.EncodeToString(h.Sum(nil))
	secret := secretGenerating(rid, sid, hashKey)

	payload, err := encryptRequestPayload(params, body, secret)
	if err != nil {
		return nil, err
	}

	accessToken, err := requiredAccessToken(authContext, tokens)
	if err != nil {
		return nil, err
	}

	headers, err := signedRequestHeaders(c.Client.ClientID, rid, sid, accessToken, hashKey, payload)
	if err != nil {
		return nil, err
	}

	request, err := c.newEncryptedRequest(ctx, method, path, payload, headers)
	if err != nil {
		return nil, err
	}

	response, err := c.Client.HTTPClient.Do(request)
	if err != nil {
		return nil, clientError(ErrorTransport, fmt.Errorf("failed to make request: %w", err))
	}

	defer func() {
		closeErr := response.Body.Close()
		if closeErr != nil {
			log.Printf("failed to close response body: %v", closeErr) // Do not override the request result.
		}
	}()

	return decodeEncryptedResponse(response, secret)
}

type encryptedRequestPayload struct {
	queryEncdata string
	bodyEncdata  string
	finalBody    []byte
}

func requestAuthorizationContext(operationRequest OperationRequest) *AuthorizationContext {
	if operationRequest == nil {
		return nil
	}

	return operationRequest.GetRequest().AuthorizationContext
}

func requiredRefreshToken(authContext *AuthorizationContext, tokens Tokens) (string, error) {
	if authContext != nil && authContext.RefreshToken != "" {
		return authContext.RefreshToken, nil
	}

	if tokens.RefreshToken == "" {
		return "", clientError(ErrorUnauthorized, errRefreshTokenRequired)
	}

	return tokens.RefreshToken, nil
}

func requiredAccessToken(authContext *AuthorizationContext, tokens Tokens) (string, error) {
	if authContext != nil && authContext.AccessToken != "" {
		return authContext.AccessToken, nil
	}

	if tokens.AccessToken == "" {
		return "", clientError(ErrorUnauthorized, errAccessTokenRequired)
	}

	return tokens.AccessToken, nil
}

func encryptRequestPayload(params, body map[string]any, secret string) (encryptedRequestPayload, error) {
	var payload encryptedRequestPayload

	if len(params) > 0 {
		queryJSON, err := formToJSON(params)
		if err != nil {
			return encryptedRequestPayload{}, clientError(ErrorProtocol, fmt.Errorf("failed to marshal params: %w", err))
		}

		encrypted, err := aesGCMEncrypt(queryJSON, secret)
		if err != nil {
			return encryptedRequestPayload{}, clientError(ErrorProtocol, fmt.Errorf("failed to encrypt params: %w", err))
		}

		payload.queryEncdata = string(encrypted)
	}

	if len(body) > 0 {
		bodyJSON, err := formToJSON(body)
		if err != nil {
			return encryptedRequestPayload{}, clientError(ErrorProtocol, fmt.Errorf("failed to marshal body: %w", err))
		}

		encrypted, err := aesGCMEncrypt(bodyJSON, secret)
		if err != nil {
			return encryptedRequestPayload{}, clientError(ErrorProtocol, fmt.Errorf("failed to encrypt body: %w", err))
		}

		payload.bodyEncdata = string(encrypted)

		payload.finalBody, err = json.Marshal(wire.EncryptedDataEnvelope{
			Encdata: payload.bodyEncdata,
		})
		if err != nil {
			return encryptedRequestPayload{}, clientError(ErrorProtocol, fmt.Errorf("failed to marshal encrypted body envelope: %w", err))
		}
	}

	return payload, nil
}

func signedRequestHeaders(clientID, rid, sid, accessToken, hashKey string, payload encryptedRequestPayload) (map[string]string, error) {
	requestHeaders := wire.EncryptedRequestHeaders{
		XAppKey:    clientID,
		XRequestId: rid,
		XSid:       sid,
		XSign:      "",
		XTime:      strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10),
		XToken:     accessToken,
	}

	headers, err := wireStringMap(requestHeaders)
	if err != nil {
		return nil, clientError(ErrorProtocol, fmt.Errorf("failed to encode request headers: %w", err))
	}

	requestHeaders.XSign = restfulSign(hashKey, payload.queryEncdata, payload.bodyEncdata, headers)

	headers, err = wireStringMap(requestHeaders)
	if err != nil {
		return nil, clientError(ErrorProtocol, fmt.Errorf("failed to encode request headers: %w", err))
	}

	return headers, nil
}

func (c *EncryptedClient) newEncryptedRequest(
	ctx context.Context,
	method, path string,
	payload encryptedRequestPayload,
	headers map[string]string,
) (*http.Request, error) {
	var body io.Reader
	if payload.finalBody != nil {
		body = bytes.NewBuffer(payload.finalBody)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.Client.CloudAPIURL+path, body)
	if err != nil {
		return nil, clientError(ErrorInvalidOperation, fmt.Errorf("failed to create request: %w", err))
	}

	queryValues := request.URL.Query()

	if payload.queryEncdata != "" {
		queryEnvelope := wire.EncryptedDataEnvelope{Encdata: payload.queryEncdata}

		encodedParams, err := wireQueryValues(queryEnvelope)
		if err != nil {
			return nil, clientError(ErrorProtocol, fmt.Errorf("failed to encode encrypted query envelope: %w", err))
		}

		for key, values := range encodedParams {
			for _, value := range values {
				queryValues.Add(key, value)
			}
		}
	}

	request.URL.RawQuery = queryValues.Encode()
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	return request, nil
}

func decodeEncryptedResponse(resp *http.Response, secret string) (*EncryptedAPIResponse, error) {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, clientError(ErrorTransport, fmt.Errorf("failed to read response: %w", err))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, responseStatusError(resp.StatusCode, respBody)
	}

	return parseSuccessfulEncryptedResponse(resp.StatusCode, respBody, secret)
}

func responseStatusError(statusCode int, responseBody []byte) error {
	kind := ErrorProvider

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		kind = ErrorUnauthorized
	case http.StatusNotFound:
		kind = ErrorNotFound
	}

	return clientError(kind, fmt.Errorf("%w: code=%d, content=%s", errHTTPResponse, statusCode, string(responseBody)))
}

func parseSuccessfulEncryptedResponse(statusCode int, responseBody []byte, secret string) (*EncryptedAPIResponse, error) {
	transportResponse, err := decodeTransportResponse(responseBody)
	if err != nil {
		return nil, err
	}

	success := dereference(transportResponse.Success)
	if !success {
		return nil, unsuccessfulTransportResponseError(transportResponse)
	}

	responseData, err := convertWireValue[map[string]any](transportResponse)
	if err != nil {
		return nil, clientError(ErrorProtocol, fmt.Errorf("failed to map transport response: %w", err))
	}

	err = populateDecryptedResult(responseData, transportResponse.Result, secret)
	if err != nil {
		return nil, err
	}

	return encryptedAPIResponse(statusCode, responseData, success, transportResponse), nil
}

func decodeTransportResponse(responseBody []byte) (wire.EncryptedHTTPResponseEnvelope, error) {
	var transportResponse wire.EncryptedHTTPResponseEnvelope

	err := json.Unmarshal(responseBody, &transportResponse)
	if err != nil {
		return wire.EncryptedHTTPResponseEnvelope{}, clientError(ErrorProtocol, fmt.Errorf("failed to unmarshal response: %w", err))
	}

	return transportResponse, nil
}

func unsuccessfulTransportResponseError(transportResponse wire.EncryptedHTTPResponseEnvelope) error {
	code, _ := transportResponse.Code.(string)
	msg, _ := transportResponse.Msg.(string)
	kind := ErrorProvider

	if code == "1010" {
		kind = ErrorUnauthorized
	}

	return clientError(kind, fmt.Errorf("%w: (%s) %s", errNetworkError, code, msg))
}

func encryptedAPIResponse(
	statusCode int,
	responseData map[string]any,
	success bool,
	transportResponse wire.EncryptedHTTPResponseEnvelope,
) *EncryptedAPIResponse {
	response := &EncryptedAPIResponse{
		StatusCode: statusCode,
		Headers:    nil,
		Body:       responseData,
		Success:    success,
		Code:       0,
		Message:    "",
		Time:       0,
	}

	if code, ok := transportResponse.Code.(float64); ok {
		response.Code = int(code)
	}

	if msg, ok := transportResponse.Msg.(string); ok {
		response.Message = msg
	}

	if transportResponse.T != nil {
		response.Time = *transportResponse.T
	}

	return response
}

func populateDecryptedResult(responseData map[string]any, result any, secret string) error {
	encryptedResult, ok := result.(string)
	if !ok {
		return nil
	}

	decrypted, err := aesGCMDecrypt(encryptedResult, secret)
	if err != nil {
		return clientError(ErrorProtocol, fmt.Errorf("failed to decrypt response: %w", err))
	}

	responseData["result"] = parsedResponseResult(decrypted)

	return nil
}

func parsedResponseResult(decrypted string) any {
	var parsedResult any

	err := json.Unmarshal([]byte(decrypted), &parsedResult)
	if err != nil {
		return decrypted
	}

	return parsedResult
}

// GenerateRID generates a UUID-based request ID.
func GenerateRID() string {
	return uuid.New().String()
}

// randomNonce generates a random nonce string.
func randomNonce(length int) string {
	const charset = "ABCDEFGHJKMNPQRSTWXYZabcdefhijkmnprstwxyz2345678"

	result := make([]byte, length)
	for index := range result {
		randBytes := make([]byte, 1)

		_, err := rand.Read(randBytes)
		if err != nil {
			// Fallback to less secure option if crypto/rand fails
			result[index] = charset[len(charset)/2] // Use middle character as fallback
		} else {
			result[index] = charset[int(randBytes[0])%len(charset)]
		}
	}

	return string(result)
}

// formToJSON converts map to JSON string.
func formToJSON(content map[string]any) (string, error) {
	if content == nil {
		return "", nil
	}

	jsonBytes, err := json.Marshal(content)
	if err != nil {
		return "", fmt.Errorf("marshal form as JSON: %w", err)
	}

	return string(jsonBytes), nil
}

// aesGCMEncrypt encrypts data using AES-GCM with base64 encoding.
func aesGCMEncrypt(rawData, secret string) ([]byte, error) {
	nonce := randomNonce(aesGCMNonceSize)
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

// aesGCMDecrypt decrypts AES-GCM encrypted data.
func aesGCMDecrypt(cipherData, secret string) (string, error) {
	// Decode base64
	cipherBytes, err := base64.StdEncoding.DecodeString(cipherData)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Split nonce and ciphertext using Tuya's 12-byte AES-GCM nonce.
	if len(cipherBytes) < aesGCMNonceSize {
		return "", errCipherDataTooShort
	}

	nonce := cipherBytes[:aesGCMNonceSize]
	ciphertext := cipherBytes[aesGCMNonceSize:]

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

// secretGenerating generates the secret key for encryption.
func secretGenerating(rid, sid, hashKey string) string {
	message := hashKey

	if sid != "" {
		sidLength := len(sid)

		length := min(sidLength, derivedSecretKeySize)

		ecode := ""

		var ecodeSb435 strings.Builder

		for i := range length {
			idx := int(sid[i]) % derivedSecretKeySize
			if idx < len(sid) {
				ecodeSb435.WriteString(string(sid[idx]))
			}
		}

		ecode += ecodeSb435.String()

		message += "_" + ecode
	}

	// Create HMAC-SHA256
	h := hmac.New(sha256.New, []byte(rid))
	h.Write([]byte(message))
	byteTemp := h.Sum(nil)
	secret := hex.EncodeToString(byteTemp)

	// Return first 16 characters
	if len(secret) > derivedSecretKeySize {
		return secret[:derivedSecretKeySize]
	}

	return secret
}

// restfulSign generates the signature for the request.
func restfulSign(hashKey, queryEncdata, bodyEncdata string, headers map[string]string) string {
	headerKeys := []string{"X-appKey", "X-requestId", "X-sid", "X-time", "X-token"}
	headerSignStr := ""

	var headerSignStrSb464 strings.Builder

	for _, key := range headerKeys {
		if val, exists := headers[key]; exists && val != "" {
			headerSignStrSb464.WriteString(key + "=" + val + signatureHeaderSeparator)
		}
	}

	headerSignStr += headerSignStrSb464.String()

	// Remove last "||"
	if len(headerSignStr) > len(signatureHeaderSeparator) {
		headerSignStr = headerSignStr[:len(headerSignStr)-len(signatureHeaderSeparator)]
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

// EncryptedAPIResponse represents an encrypted customer API response.
type EncryptedAPIResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       map[string]any    `json:"body,omitempty"`
	Success    bool              `json:"success"`
	Code       int               `json:"code,omitempty"`
	Message    string            `json:"msg,omitempty"`
	Time       int64             `json:"t,omitempty"`
}
