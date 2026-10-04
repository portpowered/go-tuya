package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/portpowered/go-tuya/pkg/tuya"
)

const (
	privateDirectoryMode = 0o700
	privateFileMode      = 0o600
	closeTimeout         = 5 * time.Second
	schemeTCP            = "tcp"
	schemeWS             = "ws"
)

type tokenDocument struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpireTime   int64  `json:"expire_time"`
	CloudAPIURL  string `json:"cloud_api_url,omitempty"`
}

type pendingLogin struct {
	LoginCode string    `json:"login_code"`
	UserCode  string    `json:"user_code"`
	CreatedAt time.Time `json:"created_at"`
}

type tokenStatus struct {
	Authenticated   bool       `json:"authenticated"`
	HasRefreshToken bool       `json:"has_refresh_token"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	TokenFile       string     `json:"token_file,omitempty"`
}

func loadTokenDocument(config settings) (tokenDocument, error) {
	data, err := os.ReadFile(config.tokenFile)
	if err == nil {
		return decodeTokenDocument(data)
	}

	if !errors.Is(err, os.ErrNotExist) {
		return emptyTokenDocument(), newCommandError("could not read token file", "filesystem")
	}

	return loadEnvironmentTokens()
}

func decodeTokenDocument(data []byte) (tokenDocument, error) {
	var document tokenDocument
	if json.Unmarshal(data, &document) != nil || document.AccessToken == "" || document.RefreshToken == "" {
		return emptyTokenDocument(), newCommandError("token file is invalid or incomplete", "credentials")
	}

	return document, nil
}

func loadEnvironmentTokens() (tokenDocument, error) {
	accessToken := os.Getenv("TUYA_AUTH_TOKEN")
	refreshToken := os.Getenv("TUYA_REFRESH_TOKEN")

	if accessToken == "" && refreshToken == "" {
		return emptyTokenDocument(), newCommandError("no saved tokens; run auth qr or import tokens", "credentials")
	}

	if accessToken == "" || refreshToken == "" {
		return emptyTokenDocument(), newCommandError("both TUYA_AUTH_TOKEN and TUYA_REFRESH_TOKEN are required", "credentials")
	}

	document := tokenDocument{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpireTime:   0,
		CloudAPIURL:  "",
	}

	if rawExpiry := os.Getenv("TUYA_AUTH_TOKEN_EXPIRED"); rawExpiry != "" {
		expiry, parseErr := strconv.ParseInt(rawExpiry, 10, 64)
		if parseErr != nil || expiry < 0 {
			return emptyTokenDocument(), newCommandError("TUYA_AUTH_TOKEN_EXPIRED must be Unix milliseconds", "credentials")
		}

		document.ExpireTime = expiry
	}

	return document, nil
}

func emptyTokenDocument() tokenDocument {
	var document tokenDocument

	return document
}

func optionalTokenDocument(config settings) (tokenDocument, bool, error) {
	_, err := os.Stat(config.tokenFile)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		document, loadErr := loadTokenDocument(config)

		return document, loadErr == nil, loadErr
	}

	if os.Getenv("TUYA_AUTH_TOKEN") == "" && os.Getenv("TUYA_REFRESH_TOKEN") == "" {
		var emptyDocument tokenDocument

		return emptyDocument, false, nil
	}

	document, loadErr := loadTokenDocument(config)

	return document, loadErr == nil, loadErr
}

func saveTokenDocument(path string, document tokenDocument) error {
	if document.AccessToken == "" || document.RefreshToken == "" {
		return newCommandError("both access and refresh tokens are required", "credentials")
	}

	return writeProtectedJSON(path, document)
}

func writeProtectedJSON(path string, value any) error {
	directory := filepath.Dir(path)

	err := os.MkdirAll(directory, privateDirectoryMode)
	if err != nil {
		return newCommandError("could not create credential directory", "filesystem")
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return newCommandError("could not encode credential file", "internal")
	}

	file, err := os.CreateTemp(directory, ".go-tuya-secret-*")
	if err != nil {
		return newCommandError("could not create credential file", "filesystem")
	}

	temporaryPath := file.Name()

	defer func() { _ = os.Remove(temporaryPath) }()

	err = file.Chmod(privateFileMode)
	if err != nil {
		_ = file.Close()

		return newCommandError("could not restrict credential file permissions", "filesystem")
	}

	err = restrictSecretFile(temporaryPath)
	if err != nil {
		_ = file.Close()

		return newCommandError("could not restrict credential file permissions", "filesystem")
	}

	_, err = file.Write(append(data, '\n'))
	if err != nil {
		_ = file.Close()

		return newCommandError("could not write credential file", "filesystem")
	}

	err = file.Sync()
	if err != nil {
		_ = file.Close()

		return newCommandError("could not sync credential file", "filesystem")
	}

	err = file.Close()
	if err != nil {
		return newCommandError("could not close credential file", "filesystem")
	}

	err = os.Rename(temporaryPath, path)
	if err != nil {
		return newCommandError("could not save credential file", "filesystem")
	}

	return nil
}

func readJSONInput(input io.Reader) (tokenDocument, error) {
	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()

	var document tokenDocument

	err := decoder.Decode(&document)
	if err != nil {
		return emptyTokenDocument(), newCommandError("could not read token JSON", "credentials")
	}

	if document.AccessToken == "" || document.RefreshToken == "" {
		return emptyTokenDocument(), newCommandError("token JSON must include access_token and refresh_token", "credentials")
	}

	return document, nil
}

func tokensFromDocument(document tokenDocument) tuya.Tokens {
	return tuya.Tokens{
		AccessToken:  document.AccessToken,
		RefreshToken: document.RefreshToken,
		ExpireTime:   document.ExpireTime,
	}
}

func resolveCloudURL(config settings, document tokenDocument) string {
	if config.cloudAPIURL != "" {
		return config.cloudAPIURL
	}

	return document.CloudAPIURL
}

func newRequest() tuya.Request {
	var request tuya.Request

	return request
}

func withSession(ctx context.Context, config settings, document tokenDocument, deps Dependencies, action func(*tuya.Session) error) (runErr error) {
	clientSettings := config
	clientSettings.cloudAPIURL = resolveCloudURL(config, document)

	client, err := newSDKClient(clientSettings, deps)
	if err != nil {
		return err
	}

	session := client.NewSession(tokensFromDocument(document))

	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
		defer cancel()

		err := session.Close(closeCtx)

		if err != nil && runErr == nil {
			runErr = safeSDKError("close Tuya session", err)
		}

		session.HTTPClient.CloseIdleConnections()
	}()

	return action(session)
}

func expiredAt(expireTime int64) *time.Time {
	if expireTime <= 0 {
		return nil
	}

	expiry := time.UnixMilli(expireTime).UTC()

	return &expiry
}
