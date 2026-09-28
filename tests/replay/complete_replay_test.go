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
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/portpowered/go-tuya/pkg/tuya"
)

type operationPair struct {
	OperationID       string `json:"operation_id"`
	ResponseTransform string `json:"response_transform,omitempty"`
	fixtureExchange
}

type orderedHTTPReplay struct {
	pairs []operationPair
	next  int
}

func (r *orderedHTTPReplay) RoundTrip(request *http.Request) (*http.Response, error) {
	if r.next >= len(r.pairs) {
		return nil, fmt.Errorf("unexpected exchange after transcript exhaustion: %s %s", request.Method, request.URL)
	}
	pair := r.pairs[r.next]
	if err := matchFixtureRequest(request, pair.fixtureExchange); err != nil {
		return nil, fmt.Errorf("exchange %d %s: %w", r.next, pair.OperationID, err)
	}
	body := pair.Response.Body
	switch pair.ResponseTransform {
	case "":
	case "encrypted-string-result":
		var envelope map[string]any
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, err
		}
		plain, ok := envelope["result"].(string)
		if !ok {
			return nil, fmt.Errorf("addDeviceUser fixture result is not a string")
		}
		cipherResult, err := encryptSyntheticResponse(request.Header.Get("X-requestId"), plain)
		if err != nil {
			return nil, err
		}
		envelope["result"] = cipherResult
		body, err = json.Marshal(envelope)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown response transform %q", pair.ResponseTransform)
	}
	r.next++
	return &http.Response{StatusCode: pair.Response.Status, Header: http.Header(pair.Response.Headers), Body: io.NopCloser(bytes.NewReader(body)), Request: request}, nil
}

// The provider encrypts string results with a key derived from the request ID.
// This fixed synthetic nonce makes the stored plaintext result replayable while
// retaining the wire encryption step and a stored paired response template.
func encryptSyntheticResponse(requestID, plain string) (string, error) {
	hash := md5.Sum([]byte(requestID + "synthetic-refresh-token"))
	mac := hmac.New(sha256.New, []byte(requestID))
	_, _ = mac.Write([]byte(hex.EncodeToString(hash[:])))
	key := hex.EncodeToString(mac.Sum(nil))[:16]
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	quoted, err := json.Marshal(plain)
	if err != nil {
		return "", err
	}
	nonce := []byte("synthetic123")
	return base64.StdEncoding.EncodeToString(append(nonce, gcm.Seal(nil, nonce, quoted, nil)...)), nil
}

func (r *orderedHTTPReplay) verifyConsumed() error {
	if r.next != len(r.pairs) {
		return fmt.Errorf("consumed %d of %d HTTP exchanges", r.next, len(r.pairs))
	}
	return nil
}

func loadOperationPairs(t *testing.T) []operationPair {
	t.Helper()
	data, err := os.ReadFile("fixtures/device-sharing/synthetic/remaining-operations.synthetic.json")
	if err != nil {
		t.Fatal(err)
	}
	var pairs []operationPair
	if err := json.Unmarshal(data, &pairs); err != nil {
		t.Fatal(err)
	}
	return pairs
}

// Every remaining OpenAPI operation is exercised against a stored, synthetic
// request/response pair. The ordered transport fails before returning a body
// when any request differs from its paired fixture.
func TestRemainingOperationsPairedReplay(t *testing.T) {
	pairs := loadOperationPairs(t)
	r := &orderedHTTPReplay{pairs: pairs}
	s := authenticatedReplayClient(r)
	ctx := context.Background()
	for _, step := range []struct {
		id  string
		run func() error
	}{
		{"getDeviceDetails", func() error {
			v, e := s.DevicesService.GetDeviceDetails(ctx, tuya.GetDeviceDetailsRequest{DeviceID: "device-1"})
			if e == nil && v.Device.ID != "device-1" {
				return fmt.Errorf("device ID = %q", v.Device.ID)
			}
			return e
		}},
		{"getDevicesByUser", func() error {
			v, e := s.DevicesService.QueryDevicesByUser(ctx, tuya.QueryDevicesByUserRequest{UID: "user-1"})
			if e == nil && (len(v.Devices) != 1 || v.Devices[0].ID != "device-1") {
				return fmt.Errorf("devices = %+v", v.Devices)
			}
			return e
		}},
		{"updateDeviceFunctionName", func() error {
			v, e := s.DevicesService.UpdateDeviceFunctionName(ctx, tuya.UpdateDeviceFunctionNameRequest{DeviceID: "device-1", FunctionCode: "switch", Name: "Power"})
			if e == nil && !v.Result {
				return fmt.Errorf("rename failed")
			}
			return e
		}},
		{"getDeviceLogs", func() error {
			_, e := s.DevicesService.QueryDeviceLogs(ctx, tuya.QueryDeviceLogsRequest{DeviceID: "device-1", Types: []string{"report"}})
			return e
		}},
		{"resetDeviceFactory", func() error {
			v, e := s.DevicesService.ResetDeviceFactoryDefaults(ctx, tuya.ResetDeviceFactoryDefaultsRequest{DeviceID: "device-1"})
			if e == nil && !v.Result {
				return fmt.Errorf("reset failed")
			}
			return e
		}},
		{"getSubDevices", func() error {
			_, e := s.DevicesService.QuerySubDevices(ctx, tuya.QuerySubDevicesRequest{DeviceID: "device-1"})
			return e
		}},
		{"getFactoryInfos", func() error {
			_, e := s.DevicesService.QueryDeviceFactoryInfos(ctx, tuya.QueryDeviceFactoryInfosRequest{DeviceIDs: []string{"device-1"}})
			return e
		}},
		{"addDeviceUser", func() error {
			v, e := s.DevicesService.AddDeviceUser(ctx, tuya.AddDeviceUserRequest{DeviceID: "device-1", NickName: "Guest"})
			if e == nil && v.UserID != "user-2" {
				return fmt.Errorf("user ID = %q", v.UserID)
			}
			return e
		}},
		{"updateDeviceUser", func() error {
			v, e := s.DevicesService.UpdateDeviceUser(ctx, tuya.UpdateDeviceUserRequest{DeviceID: "device-1", UserID: "user-2", NickName: "Guest"})
			if e == nil && !v.Result {
				return fmt.Errorf("update user failed")
			}
			return e
		}},
		{"getDeviceUser", func() error {
			_, e := s.DevicesService.GetDeviceUser(ctx, tuya.GetDeviceUserRequest{DeviceID: "device-1", UserID: "user-2"})
			return e
		}},
		{"listDeviceUsers", func() error {
			_, e := s.DevicesService.ListDeviceUsers(ctx, tuya.ListDeviceUsersRequest{DeviceID: "device-1"})
			return e
		}},
		{"deleteDeviceUser", func() error {
			v, e := s.DevicesService.DeleteDeviceUser(ctx, tuya.DeleteDeviceUserRequest{DeviceID: "device-1", UserID: "user-2"})
			if e == nil && !v.Result {
				return fmt.Errorf("delete user failed")
			}
			return e
		}},
		{"updateMultiOutletName", func() error {
			v, e := s.DevicesService.UpdateMultiOutletName(ctx, tuya.UpdateMultiOutletNameRequest{DeviceID: "device-1", Identifier: "outlet-1", Name: "Desk"})
			if e == nil && !v.Result {
				return fmt.Errorf("rename outlet failed")
			}
			return e
		}},
		{"listMultiOutletNames", func() error {
			_, e := s.DevicesService.ListMultiOutletNames(ctx, tuya.ListMultiOutletNamesRequest{DeviceID: "device-1"})
			return e
		}},
		{"updateDeviceName", func() error {
			v, e := s.DevicesService.UpdateDeviceName(ctx, tuya.UpdateDeviceNameRequest{DeviceID: "device-1", Name: "Desk lamp"})
			if e == nil && !v.Result {
				return fmt.Errorf("rename failed")
			}
			return e
		}},
		{"sendDeviceCommands", func() error {
			v, e := s.DevicesService.SendCommands(ctx, tuya.SendCommandsRequest{DeviceID: "device-1", Commands: []tuya.Command{{Code: "switch", Value: true}}})
			if e == nil && !v.Result {
				return fmt.Errorf("command failed")
			}
			return e
		}},
		{"queryDeviceSpecification", func() error {
			_, e := s.DevicesService.QueryDeviceSpecification(ctx, tuya.QueryDeviceSpecificationRequest{DeviceID: "device-1"})
			return e
		}},
		{"refreshAccessToken", func() error {
			v, e := s.AuthService.RefreshToken(ctx, tuya.RefreshTokenRequest{RefreshToken: "synthetic-refresh-token"})
			if e == nil && v.AccessToken != "synthetic-rotated-access" {
				return fmt.Errorf("access token = %q", v.AccessToken)
			}
			return e
		}},
		{"getMessageQueueConfig", func() error {
			v, e := s.MessageQueue.GetMessageQueueConfig(ctx)
			if e == nil && v.ClientID != "synthetic-mqtt-client" {
				return fmt.Errorf("MQTT client ID = %q", v.ClientID)
			}
			return e
		}},
		{"startRTCSession", func() error {
			v, e := s.StartRTCStream(ctx, tuya.StartRTCStreamRequest{DeviceID: "device-1", SDPOffer: "v=0 synthetic offer"})
			if e == nil && (v.GetSessionID() != "session-1" || v.GetSDPAnswer() != "v=0 synthetic answer") {
				return fmt.Errorf("RTC stream = %+v", v)
			}
			return e
		}},
		{"stopRTCSession", func() error {
			return s.StopRTCStream(ctx, tuya.StopRTCStreamRequest{DeviceID: "device-1", SessionID: "session-1"})
		}},
		{"deleteDevice", func() error {
			v, e := s.DevicesService.DeleteDevice(ctx, tuya.DeleteDeviceRequest{DeviceID: "device-1"})
			if e == nil && !v.Result {
				return fmt.Errorf("delete failed")
			}
			return e
		}},
	} {
		if r.next >= len(pairs) || pairs[r.next].OperationID != step.id {
			t.Fatalf("fixture order at %d: expected %s", r.next, step.id)
		}
		if err := step.run(); err != nil {
			t.Fatalf("%s: %v", step.id, err)
		}
	}
	if err := r.verifyConsumed(); err != nil {
		t.Fatal(err)
	}
	if response, err := r.RoundTrip(&http.Request{Method: "GET"}); err == nil || response != nil {
		t.Fatalf("exhausted transcript returned %v, %v", response, err)
	}
}

func TestHTTPReplaySchemaInventoryAndExhaustion(t *testing.T) {
	pairs := loadOperationPairs(t)
	seen := map[string]bool{}
	for _, pair := range pairs {
		if seen[pair.OperationID] {
			t.Fatalf("duplicate %s", pair.OperationID)
		}
		seen[pair.OperationID] = true
	}
	for _, id := range []string{"generateLoginQRCode", "validateLoginCode", "queryHomes", "queryHomeDevices", "queryDeviceStatus", "getDeviceList"} {
		seen[id] = true
	}
	data, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ids := regexp.MustCompile(`(?m)^\s+operationId: (\w+)\s*$`).FindAllSubmatch(data, -1)
	if len(ids) != len(seen) {
		t.Fatalf("schema has %d operations, paired fixtures cover %d", len(ids), len(seen))
	}
	for _, id := range ids {
		if !seen[string(id[1])] {
			t.Errorf("unpaired schema operation %s", id[1])
		}
	}
	r := &orderedHTTPReplay{pairs: pairs}
	if err := r.verifyConsumed(); err == nil {
		t.Fatal("unconsumed fixtures accepted")
	}
}

type tamperingTransport struct {
	inner  *orderedHTTPReplay
	change func(*http.Request)
}

func (r tamperingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	r.change(request)
	return r.inner.RoundTrip(request)
}

func TestPairedReplayRejectsInvalidVolatileValues(t *testing.T) {
	pairs := loadOperationPairs(t)
	byID := map[string]operationPair{}
	for _, pair := range pairs {
		byID[pair.OperationID] = pair
	}
	for _, test := range []struct {
		name, id string
		change   func(*http.Request)
		run      func(*tuya.Session) error
	}{
		{"signature", "getDeviceDetails", func(r *http.Request) { r.Header.Set("X-sign", strings.Repeat("a", 64)) }, func(s *tuya.Session) error {
			_, err := s.DevicesService.GetDeviceDetails(context.Background(), tuya.GetDeviceDetailsRequest{DeviceID: "device-1"})
			return err
		}},
		{"request ID", "getDeviceDetails", func(r *http.Request) { r.Header.Set("X-requestId", "invalid") }, func(s *tuya.Session) error {
			_, err := s.DevicesService.GetDeviceDetails(context.Background(), tuya.GetDeviceDetailsRequest{DeviceID: "device-1"})
			return err
		}},
		{"timestamp", "getDeviceDetails", func(r *http.Request) { r.Header.Set("X-time", "-1") }, func(s *tuya.Session) error {
			_, err := s.DevicesService.GetDeviceDetails(context.Background(), tuya.GetDeviceDetailsRequest{DeviceID: "device-1"})
			return err
		}},
		{"encrypted query", "getDeviceLogs", func(r *http.Request) {
			q := r.URL.Query()
			q.Set("encdata", "c3ludGhldGlj")
			r.URL.RawQuery = q.Encode()
		}, func(s *tuya.Session) error {
			_, err := s.DevicesService.QueryDeviceLogs(context.Background(), tuya.QueryDeviceLogsRequest{DeviceID: "device-1", Types: []string{"report"}})
			return err
		}},
		{"encrypted body", "updateDeviceName", func(r *http.Request) { r.Body = io.NopCloser(strings.NewReader(`{"encdata":"c3ludGhldGlj"}`)) }, func(s *tuya.Session) error {
			_, err := s.DevicesService.UpdateDeviceName(context.Background(), tuya.UpdateDeviceNameRequest{DeviceID: "device-1", Name: "Desk lamp"})
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := &orderedHTTPReplay{pairs: []operationPair{byID[test.id]}}
			s := authenticatedReplayClient(tamperingTransport{inner: r, change: test.change})
			if err := test.run(s); err == nil {
				t.Fatal("tampered request returned a response")
			}
			if r.next != 0 {
				t.Fatal("mismatched request consumed fixture")
			}
		})
	}
}
