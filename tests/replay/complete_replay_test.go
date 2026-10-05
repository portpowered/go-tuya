package replay_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5" // #nosec G501 -- replay tests reproduce Tuya's protocol-mandated request-key derivation.
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

// Repeated values stay test-local so synthetic fixtures remain independent of production constants.
const (
	completeReplayFixtureDevice1               = "device-1"
	completeReplayFixtureGetDeviceDetails      = "getDeviceDetails"
	completeReplayFixtureSyntheticRefreshToken = "synthetic-refresh-token"
	completeReplayFixtureUser2                 = "user-2"
	completeReplayFixtureUpdateDeviceName      = "updateDeviceName"
	completeReplayFixtureDeskLampName          = "Desk lamp"
)

type operationPair struct {
	fixtureExchange

	OperationID       string `json:"operation_id"`
	ResponseTransform string `json:"response_transform,omitempty"`
}

type orderedHTTPReplay struct {
	pairs []operationPair
	next  int
}

func (r *orderedHTTPReplay) RoundTrip(request *http.Request) (*http.Response, error) {
	if r.next >= len(r.pairs) {
		return nil, testMismatchf("unexpected exchange after transcript exhaustion")
	}

	pair := r.pairs[r.next]

	err := matchFixtureRequest(request, pair.fixtureExchange)
	if err != nil {
		return nil, fmt.Errorf("exchange %d %s: %w", r.next, pair.OperationID, err)
	}

	body := pair.Response.Body

	switch pair.ResponseTransform {
	case "":
	case "encrypted-string-result":
		var envelope map[string]any
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, fmt.Errorf("decode paired response envelope: %w", err)
		}

		plain, ok := envelope["result"].(string)
		if !ok {
			return nil, errReplayTestAddDeviceUserResult
		}

		cipherResult, err := encryptSyntheticResponse(request.Header.Get("X-Requestid"), plain)
		if err != nil {
			return nil, fmt.Errorf("encrypt paired response result: %w", err)
		}

		envelope["result"] = cipherResult

		body, err = json.Marshal(envelope)
		if err != nil {
			return nil, fmt.Errorf("encode paired response envelope: %w", err)
		}
	default:
		return nil, testMismatchf("unknown response transform %q", pair.ResponseTransform)
	}

	r.next++

	return &http.Response{
		StatusCode: pair.Response.Status,
		Header:     http.Header(pair.Response.Headers),
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    request,
	}, nil
}

// The provider encrypts string results with a key derived from the request ID.
// This fixed synthetic nonce makes the stored plaintext result replayable while
// retaining the wire encryption step and a stored paired response template.
func encryptSyntheticResponse(requestID, plain string) (string, error) {
	hash := md5.Sum([]byte(requestID + completeReplayFixtureSyntheticRefreshToken)) // #nosec G401 -- replay encryption mirrors Tuya's protocol-defined test key.
	mac := hmac.New(sha256.New, []byte(requestID))
	_, _ = mac.Write([]byte(hex.EncodeToString(hash[:])))
	key := hex.EncodeToString(mac.Sum(nil))[:16]

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("create synthetic response cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create synthetic response authenticator: %w", err)
	}

	quoted, err := json.Marshal(plain)
	if err != nil {
		return "", fmt.Errorf("encode synthetic response plaintext: %w", err)
	}

	nonce := []byte("synthetic123")

	return base64.StdEncoding.EncodeToString(append(nonce, gcm.Seal(nil, nonce, quoted, nil)...)), nil
}

func runReplayDeviceNameUpdate(session *tuya.Session) error {
	_, err := session.DevicesService.UpdateDeviceName(
		context.Background(),
		tuya.UpdateDeviceNameRequest{
			DeviceID: completeReplayFixtureDevice1,
			Name:     completeReplayFixtureDeskLampName,
		},
	)

	return wrapReplayOperationError(err)
}

func wrapReplayOperationError(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("paired replay operation: %w", err)
}

func (r *orderedHTTPReplay) verifyConsumed() error {
	if r.next != len(r.pairs) {
		return testMismatchf("consumed %d of %d HTTP exchanges", r.next, len(r.pairs))
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
//
//nolint:cyclop,funlen,gocognit,gocyclo,maintidx // The fixture matrix keeps each operation beside its ordered request/response pair.
func TestRemainingOperationsPairedReplay(t *testing.T) {
	t.Parallel()

	pairs := loadOperationPairs(t)
	replay := &orderedHTTPReplay{pairs: pairs}
	session := authenticatedReplayClient(replay)
	ctx := context.Background()

	for _, step := range []struct {
		id  string
		run func() error
	}{
		{completeReplayFixtureGetDeviceDetails, func() error {
			v, e := session.DevicesService.GetDeviceDetails(ctx, tuya.GetDeviceDetailsRequest{DeviceID: completeReplayFixtureDevice1})
			if e == nil && v.Device.ID != completeReplayFixtureDevice1 {
				return testMismatchf("device ID = %q", v.Device.ID)
			}

			return wrapReplayOperationError(e)
		}},
		{"getDevicesByUser", func() error {
			v, e := session.DevicesService.QueryDevicesByUser(ctx, tuya.QueryDevicesByUserRequest{UID: "user-1"})
			if e == nil && (len(v.Devices) != 1 || v.Devices[0].ID != completeReplayFixtureDevice1) {
				return testMismatchf("devices = %+v", v.Devices)
			}

			return wrapReplayOperationError(e)
		}},
		{"updateDeviceFunctionName", func() error {
			response, requestErr := session.DevicesService.UpdateDeviceFunctionName(
				ctx,
				tuya.UpdateDeviceFunctionNameRequest{
					DeviceID:     completeReplayFixtureDevice1,
					FunctionCode: "switch",
					Name:         "Power",
				},
			)
			if requestErr == nil && !response.Result {
				return errReplayTestRenameFailed
			}

			return wrapReplayOperationError(requestErr)
		}},
		{"getDeviceLogs", func() error {
			_, e := session.DevicesService.QueryDeviceLogs(ctx, tuya.QueryDeviceLogsRequest{DeviceID: completeReplayFixtureDevice1, Types: []string{"report"}})

			return wrapReplayOperationError(e)
		}},
		{"resetDeviceFactory", func() error {
			v, e := session.DevicesService.ResetDeviceFactoryDefaults(ctx, tuya.ResetDeviceFactoryDefaultsRequest{DeviceID: completeReplayFixtureDevice1})
			if e == nil && !v.Result {
				return errReplayTestResetFailed
			}

			return wrapReplayOperationError(e)
		}},
		{"getSubDevices", func() error {
			_, e := session.DevicesService.QuerySubDevices(ctx, tuya.QuerySubDevicesRequest{DeviceID: completeReplayFixtureDevice1})

			return wrapReplayOperationError(e)
		}},
		{"getFactoryInfos", func() error {
			_, e := session.DevicesService.QueryDeviceFactoryInfos(ctx, tuya.QueryDeviceFactoryInfosRequest{DeviceIDs: []string{completeReplayFixtureDevice1}})

			return wrapReplayOperationError(e)
		}},
		{"addDeviceUser", func() error {
			v, e := session.DevicesService.AddDeviceUser(ctx, tuya.AddDeviceUserRequest{DeviceID: completeReplayFixtureDevice1, NickName: "Guest"})
			if e == nil && v.UserID != completeReplayFixtureUser2 {
				return testMismatchf("user ID = %q", v.UserID)
			}

			return wrapReplayOperationError(e)
		}},
		{"updateDeviceUser", func() error {
			response, requestErr := session.DevicesService.UpdateDeviceUser(
				ctx,
				tuya.UpdateDeviceUserRequest{
					DeviceID: completeReplayFixtureDevice1,
					UserID:   completeReplayFixtureUser2,
					NickName: "Guest",
				},
			)
			if requestErr == nil && !response.Result {
				return errReplayTestUpdateUserFailed
			}

			return wrapReplayOperationError(requestErr)
		}},
		{"getDeviceUser", func() error {
			_, e := session.DevicesService.GetDeviceUser(ctx, tuya.GetDeviceUserRequest{DeviceID: completeReplayFixtureDevice1, UserID: completeReplayFixtureUser2})

			return wrapReplayOperationError(e)
		}},
		{"listDeviceUsers", func() error {
			_, e := session.DevicesService.ListDeviceUsers(ctx, tuya.ListDeviceUsersRequest{DeviceID: completeReplayFixtureDevice1})

			return wrapReplayOperationError(e)
		}},
		{"deleteDeviceUser", func() error {
			response, requestErr := session.DevicesService.DeleteDeviceUser(ctx, tuya.DeleteDeviceUserRequest{
				DeviceID: completeReplayFixtureDevice1,
				UserID:   completeReplayFixtureUser2,
			})
			if requestErr == nil && !response.Result {
				return errReplayTestDeleteUserFailed
			}

			return wrapReplayOperationError(requestErr)
		}},
		{"updateMultiOutletName", func() error {
			response, requestErr := session.DevicesService.UpdateMultiOutletName(
				ctx,
				tuya.UpdateMultiOutletNameRequest{
					DeviceID:   completeReplayFixtureDevice1,
					Identifier: "outlet-1",
					Name:       "Desk",
				},
			)
			if requestErr == nil && !response.Result {
				return errReplayTestRenameOutletFailed
			}

			return wrapReplayOperationError(requestErr)
		}},
		{"listMultiOutletNames", func() error {
			_, e := session.DevicesService.ListMultiOutletNames(ctx, tuya.ListMultiOutletNamesRequest{DeviceID: completeReplayFixtureDevice1})

			return wrapReplayOperationError(e)
		}},
		{completeReplayFixtureUpdateDeviceName, func() error {
			deviceNameResponse, updateErr := session.DevicesService.UpdateDeviceName(
				ctx,
				tuya.UpdateDeviceNameRequest{
					DeviceID: completeReplayFixtureDevice1,
					Name:     completeReplayFixtureDeskLampName,
				},
			)
			if updateErr == nil && !deviceNameResponse.Result {
				return errReplayTestRenameFailed
			}

			return wrapReplayOperationError(updateErr)
		}},
		{"sendDeviceCommands", func() error {
			v, requestErr := session.DevicesService.SendCommands(ctx, tuya.SendCommandsRequest{
				DeviceID: completeReplayFixtureDevice1,
				Commands: []tuya.Command{{Code: "switch", Value: true}},
			})
			if requestErr == nil && !v.Result {
				return errReplayTestCommandFailed
			}

			return wrapReplayOperationError(requestErr)
		}},
		{"queryDeviceSpecification", func() error {
			_, e := session.DevicesService.QueryDeviceSpecification(ctx, tuya.QueryDeviceSpecificationRequest{DeviceID: completeReplayFixtureDevice1})

			return wrapReplayOperationError(e)
		}},
		{"refreshAccessToken", func() error {
			v, e := session.AuthService.RefreshToken(ctx, tuya.RefreshTokenRequest{RefreshToken: completeReplayFixtureSyntheticRefreshToken})
			if e == nil && v.AccessToken != "synthetic-rotated-access" {
				return testMismatchf("access token = %q", v.AccessToken)
			}

			return wrapReplayOperationError(e)
		}},
		{"getMessageQueueConfig", func() error {
			v, e := session.MessageQueue.GetMessageQueueConfig(ctx)
			if e == nil && v.ClientID != "synthetic-mqtt-client" {
				return testMismatchf("MQTT client ID = %q", v.ClientID)
			}

			return wrapReplayOperationError(e)
		}},
		{"startRTCSession", func() error {
			v, e := session.StartRTCStream(ctx, tuya.StartRTCStreamRequest{DeviceID: completeReplayFixtureDevice1, SDPOffer: "v=0 synthetic offer"})
			if e == nil && (v.GetSessionID() != "session-1" || v.GetSDPAnswer() != "v=0 synthetic answer") {
				return testMismatchf("RTC stream = %+v", v)
			}

			return wrapReplayOperationError(e)
		}},
		{"stopRTCSession", func() error {
			return session.StopRTCStream(ctx, tuya.StopRTCStreamRequest{DeviceID: completeReplayFixtureDevice1, SessionID: "session-1"})
		}},
		{"deleteDevice", func() error {
			v, e := session.DevicesService.DeleteDevice(ctx, tuya.DeleteDeviceRequest{DeviceID: completeReplayFixtureDevice1})
			if e == nil && !v.Result {
				return errReplayTestDeleteFailed
			}

			return wrapReplayOperationError(e)
		}},
	} {
		if replay.next >= len(pairs) || pairs[replay.next].OperationID != step.id {
			t.Fatalf("fixture order at %d: expected %s", replay.next, step.id)
		}

		err := step.run()
		if err != nil {
			t.Fatalf("%s: %v", step.id, err)
		}
	}

	err := replay.verifyConsumed()
	if err != nil {
		t.Fatal(err)
	}

	if response, err := replay.RoundTrip(&http.Request{Method: http.MethodGet}); err == nil || response != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}

		t.Fatalf("exhausted transcript returned %v, %v", response, err)
	}
}

func TestOrderedReplayExhaustionDiagnosticHidesRequestValues(t *testing.T) {
	t.Parallel()

	const requestSecret = "synthetic-diagnostic-query-secret"

	requestURL := "https://api.example.invalid/v1.0/devices?access_token=" + requestSecret
	replay := &orderedHTTPReplay{}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, requestURL, nil)
	if err != nil {
		t.Fatal(err)
	}

	response, err := replay.RoundTrip(request)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}

	if err == nil || response != nil {
		t.Fatalf("exhausted replay returned response %v, error %v", response, err)
	}

	if strings.Contains(err.Error(), requestSecret) {
		t.Fatalf("exhaustion diagnostic exposed a request value: %v", err)
	}

	if replay.next != 0 {
		t.Fatal("unexpected request consumed a replay pair")
	}
}

func TestHTTPReplaySchemaInventoryAndExhaustion(t *testing.T) {
	t.Parallel()

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

//nolint:funlen // Each invalid volatile value stays beside its fixture mutation and expected request rejection.
func TestPairedReplayRejectsInvalidVolatileValues(t *testing.T) {
	t.Parallel()

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
		{"signature", completeReplayFixtureGetDeviceDetails, func(r *http.Request) { r.Header.Set("X-Sign", strings.Repeat("a", 64)) }, func(s *tuya.Session) error {
			_, err := s.DevicesService.GetDeviceDetails(context.Background(), tuya.GetDeviceDetailsRequest{DeviceID: completeReplayFixtureDevice1})

			return wrapReplayOperationError(err)
		}},
		{"request ID", completeReplayFixtureGetDeviceDetails, func(r *http.Request) { r.Header.Set("X-Requestid", "invalid") }, func(s *tuya.Session) error {
			_, err := s.DevicesService.GetDeviceDetails(context.Background(), tuya.GetDeviceDetailsRequest{DeviceID: completeReplayFixtureDevice1})

			return wrapReplayOperationError(err)
		}},
		{"timestamp", completeReplayFixtureGetDeviceDetails, func(r *http.Request) { r.Header.Set("X-Time", "-1") }, func(s *tuya.Session) error {
			_, err := s.DevicesService.GetDeviceDetails(context.Background(), tuya.GetDeviceDetailsRequest{DeviceID: completeReplayFixtureDevice1})

			return wrapReplayOperationError(err)
		}},
		{"encrypted query", "getDeviceLogs", func(r *http.Request) {
			q := r.URL.Query()
			q.Set("encdata", "c3ludGhldGlj")
			r.URL.RawQuery = q.Encode()
		}, func(s *tuya.Session) error {
			_, err := s.DevicesService.QueryDeviceLogs(
				context.Background(),
				tuya.QueryDeviceLogsRequest{
					DeviceID: completeReplayFixtureDevice1,
					Types:    []string{"report"},
				},
			)

			return wrapReplayOperationError(err)
		}},
		{"encrypted body", completeReplayFixtureUpdateDeviceName, func(r *http.Request) {
			body := []byte(`{"encdata":"c3ludGhldGlj"}`)
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.ContentLength = int64(len(body))
			r.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
		}, runReplayDeviceNameUpdate},
		{"content length", completeReplayFixtureUpdateDeviceName, func(r *http.Request) { r.ContentLength++ }, runReplayDeviceNameUpdate},
		{"GetBody", completeReplayFixtureUpdateDeviceName, func(r *http.Request) {
			r.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("synthetic-mutated-replay-body")), nil
			}
		}, runReplayDeviceNameUpdate},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			replay := &orderedHTTPReplay{pairs: []operationPair{byID[test.id]}}

			s := authenticatedReplayClient(tamperingTransport{inner: replay, change: test.change})

			err := test.run(s)
			if err == nil {
				t.Fatal("tampered request returned a response")
			}

			if replay.next != 0 {
				t.Fatal("mismatched request consumed fixture")
			}
		})
	}
}
