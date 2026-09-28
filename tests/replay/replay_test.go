package replay_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/portpowered/go-tuya/pkg/tuya"
)

type replayKey struct {
	method string
	path   string
}

func (key replayKey) String() string {
	return key.method + " " + key.path
}

type replayCase struct {
	fixture  string
	status   int
	validate func(*http.Request) error
}

type fixtureTransport struct {
	cases map[replayKey]replayCase
	calls []replayKey
}

func (transport *fixtureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	key := replayKey{method: request.Method, path: request.URL.Path}
	replay, ok := transport.cases[key]
	if !ok {
		return nil, fmt.Errorf("unexpected replay request: %s", key)
	}
	if replay.validate != nil {
		if err := replay.validate(request); err != nil {
			return nil, fmt.Errorf("validate replay request %s: %w", key, err)
		}
	}

	body, err := os.ReadFile(filepath.Join("fixtures", replay.fixture))
	if err != nil {
		return nil, fmt.Errorf("read replay fixture %q: %w", replay.fixture, err)
	}
	transport.calls = append(transport.calls, key)
	status := replay.status
	if status == 0 {
		status = http.StatusOK
	}

	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    request,
	}, nil
}

func replayClient(transport *fixtureTransport) *tuya.Session {
	client, err := tuya.NewClient(
		tuya.WithHTTPTransport(transport),
		tuya.WithClientID("synthetic-client-id"),
		tuya.WithAuthenticationURL("https://login.example.invalid"),
		tuya.WithCloudAPIURL("https://api.example.invalid"),
	)
	if err != nil {
		panic(err)
	}
	return client.NewSession(tuya.Tokens{})
}

func authenticatedReplayClient(transport *fixtureTransport) *tuya.Session {
	client, err := tuya.NewClient(
		tuya.WithHTTPTransport(transport),
		tuya.WithClientID("synthetic-client-id"),
		tuya.WithCloudAPIURL("https://api.example.invalid"),
	)
	if err != nil {
		panic(err)
	}
	return client.NewSession(tuya.Tokens{
		AccessToken:  "synthetic-access-token",
		RefreshToken: "synthetic-refresh-token",
		ExpireTime:   time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
	})
}

func assertReplayCalls(t *testing.T, transport *fixtureTransport, want ...replayKey) {
	t.Helper()
	if !reflect.DeepEqual(transport.calls, want) {
		got := make([]string, len(transport.calls))
		for i, key := range transport.calls {
			got[i] = key.String()
		}
		wantStrings := make([]string, len(want))
		for i, key := range want {
			wantStrings[i] = key.String()
		}
		t.Fatalf("replay requests = %v, want %v", got, wantStrings)
	}
}

func requireAuthRequest(request *http.Request, method, path string, query url.Values) error {
	if request.Method != method {
		return fmt.Errorf("method = %q, want %q", request.Method, method)
	}
	if request.URL.Path != path {
		return fmt.Errorf("path = %q, want %q", request.URL.Path, path)
	}
	for name, want := range query {
		if got := request.URL.Query()[name]; !reflect.DeepEqual(got, want) {
			return fmt.Errorf("query %s = %q, want %q", name, got, want)
		}
	}
	if got := request.Header.Get("Content-Type"); got != "application/json" {
		return fmt.Errorf("Content-Type = %q, want application/json", got)
	}
	return nil
}

func decryptEncryptedQuery(request *http.Request, refreshToken string) (map[string]any, error) {
	encoded := request.URL.Query().Get("encdata")
	cipherData, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode encdata: %w", err)
	}
	if len(cipherData) < 12 {
		return nil, fmt.Errorf("encdata has %d bytes, want at least 12", len(cipherData))
	}

	requestID := request.Header.Get("X-requestId")
	hash := md5.Sum([]byte(requestID + refreshToken))
	hashKey := hex.EncodeToString(hash[:])
	mac := hmac.New(sha256.New, []byte(requestID))
	if _, err := mac.Write([]byte(hashKey)); err != nil {
		return nil, fmt.Errorf("derive request key: %w", err)
	}
	key := hex.EncodeToString(mac.Sum(nil))[:16]
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("create request cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create request authenticator: %w", err)
	}
	plain, err := gcm.Open(nil, cipherData[:12], cipherData[12:], nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt encdata: %w", err)
	}

	var parameters map[string]any
	if err := json.Unmarshal(plain, &parameters); err != nil {
		return nil, fmt.Errorf("decode request parameters: %w", err)
	}
	return parameters, nil
}

func requireEncryptedReadRequest(request *http.Request, path string, hasEncodedQuery bool) error {
	if request.Method != http.MethodGet {
		return fmt.Errorf("method = %q, want GET", request.Method)
	}
	if request.URL.Path != path {
		return fmt.Errorf("path = %q, want %q", request.URL.Path, path)
	}
	if hasEncodedQuery && request.URL.Query().Get("encdata") == "" {
		return fmt.Errorf("encdata query parameter is missing")
	}
	if got := request.Header.Get("X-appKey"); got != "synthetic-client-id" {
		return fmt.Errorf("X-appKey = %q, want synthetic client id", got)
	}
	if request.Header.Get("X-requestId") == "" {
		return fmt.Errorf("X-requestId is missing")
	}
	if request.Header.Get("X-sign") == "" {
		return fmt.Errorf("X-sign is missing")
	}
	if got := request.Header.Get("X-token"); got != "synthetic-access-token" {
		return fmt.Errorf("X-token = %q, want synthetic access token", got)
	}
	if _, err := strconv.ParseInt(request.Header.Get("X-time"), 10, 64); err != nil {
		return fmt.Errorf("X-time is not an integer: %w", err)
	}
	return nil
}

func TestLoginReplay_QRAndValidation(t *testing.T) {
	transport := &fixtureTransport{
		cases: map[replayKey]replayCase{
			{method: http.MethodPost, path: "/v1.0/m/life/home-assistant/qrcode/tokens"}: {
				fixture: "auth/synthetic/qr-created.synthetic.json",
				validate: func(request *http.Request) error {
					return requireAuthRequest(request, http.MethodPost, "/v1.0/m/life/home-assistant/qrcode/tokens", url.Values{
						"clientid": {"synthetic-client-id"},
						"usercode": {"synthetic-user-code"},
						"schema":   {"haauthorize"},
					})
				},
			},
			{method: http.MethodGet, path: "/v1.0/m/life/home-assistant/qrcode/tokens/synthetic-qr-code-001"}: {
				fixture: "auth/synthetic/qr-validated.synthetic.json",
				validate: func(request *http.Request) error {
					return requireAuthRequest(request, http.MethodGet, "/v1.0/m/life/home-assistant/qrcode/tokens/synthetic-qr-code-001", url.Values{
						"clientid": {"synthetic-client-id"},
						"usercode": {"synthetic-user-code"},
					})
				},
			},
		},
	}
	client := replayClient(transport)

	qr, err := client.AuthService.GenerateQrCodeForLogin(context.Background(), tuya.LoginRequest{
		AccessCode: "synthetic-user-code",
	})
	if err != nil {
		t.Fatalf("GenerateQrCodeForLogin() error = %v", err)
	}
	if qr.Code != "synthetic-qr-code-001" {
		t.Fatalf("QR code = %q, want synthetic QR code", qr.Code)
	}
	if qr.QrFormattedCode != "tuyaSmart--qrLogin?token=synthetic-qr-code-001" {
		t.Fatalf("formatted QR code = %q", qr.QrFormattedCode)
	}

	validated, err := client.AuthService.ValidateLoginCode(context.Background(), tuya.ValidateLoginCodeRequest{
		LoginCode: qr.Code,
		UserCode:  "synthetic-user-code",
	})
	if err != nil {
		t.Fatalf("ValidateLoginCode() error = %v", err)
	}
	if !validated.Success || validated.AccessToken != "synthetic-access-token" || validated.RefreshToken != "synthetic-refresh-token" {
		t.Fatalf("validated token response = %+v", validated)
	}
	if validated.ExpireTime != 3600 || validated.TerminalID != "synthetic-terminal-01" || validated.UID != "synthetic-user-01" {
		t.Fatalf("validated identity response = %+v", validated)
	}
	if validated.Endpoint != "https://api.example.invalid" {
		t.Fatalf("validated endpoint = %q", validated.Endpoint)
	}

	assertReplayCalls(t, transport,
		replayKey{method: http.MethodPost, path: "/v1.0/m/life/home-assistant/qrcode/tokens"},
		replayKey{method: http.MethodGet, path: "/v1.0/m/life/home-assistant/qrcode/tokens/synthetic-qr-code-001"},
	)
}

func TestDeviceReadReplay_HomesDevicesAndStatus(t *testing.T) {
	transport := &fixtureTransport{
		cases: map[replayKey]replayCase{
			{method: http.MethodGet, path: "/v1.0/m/life/users/homes"}: {
				fixture: "device-sharing/synthetic/homes-list.synthetic.json",
				validate: func(request *http.Request) error {
					return requireEncryptedReadRequest(request, "/v1.0/m/life/users/homes", false)
				},
			},
			{method: http.MethodGet, path: "/v1.0/m/life/ha/home/devices"}: {
				fixture: "device-sharing/synthetic/home-devices.synthetic.json",
				validate: func(request *http.Request) error {
					if err := requireEncryptedReadRequest(request, "/v1.0/m/life/ha/home/devices", true); err != nil {
						return err
					}
					parameters, err := decryptEncryptedQuery(request, "synthetic-refresh-token")
					if err != nil {
						return err
					}
					if got := parameters["homeId"]; got != "synthetic-home-01" {
						return fmt.Errorf("encrypted homeId = %v, want synthetic-home-01", got)
					}
					return nil
				},
			},
			{method: http.MethodGet, path: "/v1.0/m/life/devices/synthetic-device-01/status"}: {
				fixture: "device-sharing/synthetic/device-status.synthetic.json",
				validate: func(request *http.Request) error {
					return requireEncryptedReadRequest(request, "/v1.0/m/life/devices/synthetic-device-01/status", false)
				},
			},
		},
	}
	client := authenticatedReplayClient(transport)
	ctx := context.Background()

	homes, err := client.HomeService.QueryHomes(ctx, tuya.QueryHomesRequest{})
	if err != nil {
		t.Fatalf("QueryHomes() error = %v", err)
	}
	if len(homes.Results) != 1 || homes.Results[0].ID != "synthetic-home-01" || homes.Results[0].Name != "Synthetic Test Home" {
		t.Fatalf("homes result = %+v", homes.Results)
	}

	devices, err := client.DevicesService.QueryDevicesByHome(ctx, tuya.QueryDevicesByHomeRequest{HomeID: homes.Results[0].ID})
	if err != nil {
		t.Fatalf("QueryDevicesByHome() error = %v", err)
	}
	if len(devices.Results) != 1 || devices.Results[0].ID != "synthetic-device-01" || devices.Results[0].Name != "Synthetic Lamp" || !devices.Results[0].Online {
		t.Fatalf("devices result = %+v", devices.Results)
	}

	status, err := client.DevicesService.QueryDeviceStatus(ctx, tuya.QueryDeviceStatusRequest{DeviceID: devices.Results[0].ID})
	if err != nil {
		t.Fatalf("QueryDeviceStatus() error = %v", err)
	}
	if len(status.Status) != 1 || status.Status[0].DPCode != "switch_1" || status.Status[0].DPId != 1 || !status.Status[0].SupportLocal {
		t.Fatalf("device status result = %+v", status.Status)
	}

	assertReplayCalls(t, transport,
		replayKey{method: http.MethodGet, path: "/v1.0/m/life/users/homes"},
		replayKey{method: http.MethodGet, path: "/v1.0/m/life/ha/home/devices"},
		replayKey{method: http.MethodGet, path: "/v1.0/m/life/devices/synthetic-device-01/status"},
	)
}

func TestDeviceReadReplay_APIError(t *testing.T) {
	transport := &fixtureTransport{
		cases: map[replayKey]replayCase{
			{method: http.MethodGet, path: "/v1.0/m/life/users/homes"}: {
				fixture: "device-sharing/synthetic/api-error.synthetic.json",
				validate: func(request *http.Request) error {
					return requireEncryptedReadRequest(request, "/v1.0/m/life/users/homes", false)
				},
			},
		},
	}
	client := authenticatedReplayClient(transport)

	_, err := client.HomeService.QueryHomes(context.Background(), tuya.QueryHomesRequest{})
	if err == nil {
		t.Fatal("QueryHomes() error = nil, want synthetic API error")
	}
	if got, want := err.Error(), "network error: (1010) synthetic token rejected"; got != want {
		t.Fatalf("QueryHomes() error = %q, want %q", got, want)
	}
	assertReplayCalls(t, transport, replayKey{method: http.MethodGet, path: "/v1.0/m/life/users/homes"})
}

func TestQueryDevicesReplay_DocumentedList(t *testing.T) {
	transport := &fixtureTransport{
		cases: map[replayKey]replayCase{
			{method: http.MethodGet, path: "/v1.0/devices"}: {
				fixture: "device-sharing/synthetic/global-device-list.synthetic.json",
				validate: func(request *http.Request) error {
					if err := requireEncryptedReadRequest(request, "/v1.0/devices", true); err != nil {
						return err
					}
					parameters, err := decryptEncryptedQuery(request, "synthetic-refresh-token")
					if err != nil {
						return err
					}
					want := map[string]any{
						"page_no":    float64(1),
						"page_size":  float64(2),
						"schema":     "synthetic-app-schema",
						"product_id": "synthetic-product-01",
						"device_ids": "synthetic-device-a,synthetic-device-b",
						"last_id":    "synthetic-list-cursor-before",
					}
					if !reflect.DeepEqual(parameters, want) {
						return fmt.Errorf("encrypted parameters = %#v, want %#v", parameters, want)
					}
					return nil
				},
			},
		},
	}
	client := authenticatedReplayClient(transport)

	response, err := client.DevicesService.QueryDevices(context.Background(), tuya.QueryDevicesRequest{
		PageNo:    1,
		PageSize:  2,
		Schema:    "synthetic-app-schema",
		ProductID: "synthetic-product-01",
		DeviceIDs: []string{"synthetic-device-a", "synthetic-device-b"},
		LastID:    "synthetic-list-cursor-before",
	})
	if err != nil {
		t.Fatalf("QueryDevices() error = %v", err)
	}
	if response.Total != 1 || response.LastID != "synthetic-list-cursor-01" {
		t.Fatalf("QueryDevices() pagination = total %d, last ID %q", response.Total, response.LastID)
	}
	if len(response.Devices) != 1 {
		t.Fatalf("QueryDevices() returned %d devices, want 1", len(response.Devices))
	}

	wantDevice := tuya.Device{
		ID:           "synthetic-query-device-01",
		Name:         "Synthetic Device List Lamp",
		LocalKey:     "synthetic-local-key-01",
		Category:     "dj",
		ProductID:    "synthetic-product-01",
		ProductName:  "Synthetic Product",
		SubCategory:  "synthetic-light",
		Icon:         "https://example.invalid/synthetic-query-device.png",
		IP:           "192.0.2.15",
		Lat:          "37.0",
		Lon:          "-122.0",
		Model:        "Synthetic Model A",
		TimeZone:     "UTC",
		ActiveTime:   1700000000,
		CreateTime:   1690000000,
		UpdateTime:   1700000001,
		Online:       true,
		Status:       []tuya.Status{{Code: "switch_led", Value: true}, {Code: "brightness", Value: float64(42)}},
		Capabilities: nil,
	}
	if !reflect.DeepEqual(response.Devices[0], wantDevice) {
		t.Fatalf("QueryDevices() device = %+v, want %+v", response.Devices[0], wantDevice)
	}

	assertReplayCalls(t, transport, replayKey{method: http.MethodGet, path: "/v1.0/devices"})
}
