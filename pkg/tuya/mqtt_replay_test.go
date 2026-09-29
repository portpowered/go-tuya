package tuya

import (
	"bytes"
	"context"
	"crypto/md5" // #nosec G501 -- paired transcript tests reproduce Tuya's protocol-mandated request-key derivation.
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Repeated values stay test-local so synthetic fixtures remain independent of production constants.
const (
	mqttFixtureClient       = "client"
	mqttFixtureConnect      = "connect"
	mqttFixtureDevice1      = "device-1"
	mqttFixtureGeneral      = "general"
	syntheticRefreshFixture = "synthetic-refresh"
)

func testMismatchf(format string, args ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), errTestReplayMismatch)
}

var (
	errTestConfigBodyRule        = errors.New("unexpected config fixture body rule")
	errTestConfigSignature       = errors.New("config request signature does not authenticate headers and encrypted body")
	errTestDuplicateMQTTExchange = errors.New("duplicate MQTT config HTTP exchange")
	errTestInvalidCapabilityType = errors.New("invalid capability type")
	errTestInvalidRequestID      = errors.New("invalid request ID")
	errTestInvalidSignature      = errors.New("invalid signature")
	errTestInvalidTimestamp      = errors.New("invalid timestamp")
	errTestMissingConfigBody     = errors.New("encrypted config request body is missing")
	errTestMQTTConfigRequest     = errors.New("MQTT config request does not match paired method/origin/path/query")
	errTestMQTTFactoryOptions    = errors.New("MQTT factory options do not match paired HTTP config")
	errTestReplayMismatch        = errors.New("synthetic replay mismatch")
	errTestRequestRequiresTokens = errors.New("request should not be made without explicit tokens")
	errTestSyntheticBroker       = errors.New("synthetic broker unavailable")
	errTestSyntheticNetwork      = errors.New("synthetic network failure")
	errTestSyntheticOption       = errors.New("synthetic option failure")
	errTestSyntheticReconnect    = errors.New("synthetic reconnect failure")
	errTestUnexpectedRequest     = errors.New("unexpected request")
)

type mqttReplayFrame struct {
	Direction string `json:"direction"`
	Action    string `json:"action"`
	Topic     string `json:"topic,omitempty"`
	Payload   string `json:"payload,omitempty"`
}

type mqttReplayTranscript struct {
	Provenance string            `json:"provenance"`
	Frames     []mqttReplayFrame `json:"frames"`
	next       int
	err        error
}

func (r *mqttReplayTranscript) accept(frame mqttReplayFrame) {
	if r.err != nil {
		return
	}

	if r.next >= len(r.Frames) {
		r.err = testMismatchf("unexpected MQTT frame after transcript exhaustion: %+v", frame)

		return
	}

	if !reflect.DeepEqual(frame, r.Frames[r.next]) {
		r.err = testMismatchf("MQTT frame %d = %+v, want %+v", r.next, frame, r.Frames[r.next])

		return
	}

	r.next++
}

func (r *mqttReplayTranscript) verifyConsumed() error {
	if r.err != nil {
		return r.err
	}

	if r.next != len(r.Frames) {
		return testMismatchf("consumed %d of %d MQTT frames", r.next, len(r.Frames))
	}

	return nil
}

type mqttReplayClient struct {
	mqtt.Client

	transcript *mqttReplayTranscript
	options    *mqtt.ClientOptions
}

func (*mqttReplayClient) IsConnected() bool { return true }
func (c *mqttReplayClient) Connect() mqtt.Token { //nolint:ireturn // Paho requires the token interface on mqtt.Client implementations.
	c.transcript.accept(mqttReplayFrame{Direction: mqttFixtureClient, Action: mqttFixtureConnect, Topic: c.options.Servers[0].String()})

	if c.transcript.err == nil {
		c.options.OnConnect(c)
	}

	return completedMQTTToken{}
}

type completedMQTTToken struct{}

func (completedMQTTToken) Wait() bool                     { return true }
func (completedMQTTToken) WaitTimeout(time.Duration) bool { return true }
func (completedMQTTToken) Done() <-chan struct{} {
	done := make(chan struct{})
	close(done)

	return done
}
func (completedMQTTToken) Error() error { return nil }

type mqttHTTPPair struct {
	OperationID string `json:"operation_id"`
	Request     struct {
		Method    string              `json:"method"`
		Origin    string              `json:"origin"`
		Path      string              `json:"path"`
		Query     map[string][]string `json:"query"`
		Headers   map[string][]string `json:"headers"`
		Body      string              `json:"body"`
		PlainBody map[string]any      `json:"plain_body"`
	} `json:"request"`
	Response struct {
		Status  int                 `json:"status"`
		Headers map[string][]string `json:"headers"`
		Body    json.RawMessage     `json:"body"`
	} `json:"response"`
}

type mqttHTTPReplay struct {
	transcript *mqttReplayTranscript
	pair       mqttHTTPPair
	consumed   bool
}

var replayUUIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var replaySignaturePattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

//nolint:cyclop,funlen,gocognit,gocyclo // The replay transport validates one complete encrypted and signed HTTP exchange before consuming its fixture.
func (r *mqttHTTPReplay) RoundTrip(request *http.Request) (*http.Response, error) {
	if r.consumed {
		return nil, errTestDuplicateMQTTExchange
	}

	if request.Method != r.pair.Request.Method ||
		request.URL.Scheme+"://"+request.URL.Host != r.pair.Request.Origin ||
		request.URL.EscapedPath() != r.pair.Request.Path ||
		len(request.URL.Query()) != len(r.pair.Request.Query) {
		return nil, errTestMQTTConfigRequest
	}

	for name, values := range r.pair.Request.Headers {
		if len(values) != 1 {
			return nil, testMismatchf("unsupported fixture header %q", name)
		}

		value := request.Header.Get(name)

		switch values[0] {
		case "<uuid>":
			if !replayUUIDPattern.MatchString(value) {
				return nil, errTestInvalidRequestID
			}
		case "<tuya-signature>":
			if !replaySignaturePattern.MatchString(value) {
				return nil, errTestInvalidSignature
			}
		case "<unix-millis>":
			stamp, err := strconv.ParseInt(value, 10, 64)
			if err != nil || time.Since(time.UnixMilli(stamp)) > 5*time.Minute || time.Until(time.UnixMilli(stamp)) > 5*time.Minute {
				return nil, errTestInvalidTimestamp
			}
		default:
			if value != values[0] {
				return nil, testMismatchf("header %s = %q, want %q", name, value, values[0])
			}
		}
	}

	if r.pair.Request.Body != "<encrypted-json>" {
		return nil, errTestConfigBodyRule
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, fmt.Errorf("read paired MQTT config request: %w", err)
	}

	var encrypted struct {
		Encdata string `json:"encdata"`
	}

	if err := json.Unmarshal(body, &encrypted); err != nil {
		return nil, fmt.Errorf("missing encrypted config request body: %w", err)
	}

	if encrypted.Encdata == "" {
		return nil, fmt.Errorf("missing encrypted config request body: %w", errTestMissingConfigBody)
	}

	requestID := request.Header.Get("X-Requestid")
	hash := md5.Sum([]byte(requestID + "synthetic-refresh-token")) // #nosec G401 -- the paired transcript uses Tuya's protocol-defined test key.

	plain, err := aesGCMDecrypt(encrypted.Encdata, secretGenerating(requestID, "", hex.EncodeToString(hash[:])))
	if err != nil {
		return nil, fmt.Errorf("decrypt config request: %w", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(plain), &decoded); err != nil {
		return nil, fmt.Errorf("decode paired MQTT config request: %w", err)
	}

	linkID, ok := decoded["linkId"].(string)
	if !ok || len(decoded) != 1 || !replayUUIDPattern.MatchString(linkID) || r.pair.Request.PlainBody["linkId"] != "<uuid>" {
		return nil, testMismatchf("config request body = %#v", decoded)
	}

	signedHeaders := map[string]string{}

	for _, name := range []string{"X-appKey", "X-requestId", "X-sid", "X-time", "X-token"} {
		if value := request.Header.Get(name); value != "" {
			signedHeaders[name] = value
		}
	}

	if request.Header.Get("X-Sign") != restfulSign(hex.EncodeToString(hash[:]), "", encrypted.Encdata, signedHeaders) {
		return nil, errTestConfigSignature
	}

	r.transcript.accept(mqttReplayFrame{Direction: mqttFixtureClient, Action: "http-request", Topic: r.pair.Request.Path, Payload: r.pair.OperationID})

	if r.transcript.err != nil {
		return nil, r.transcript.err
	}

	r.transcript.accept(mqttReplayFrame{Direction: "server", Action: "http-response", Topic: r.pair.Request.Path, Payload: strconv.Itoa(r.pair.Response.Status)})

	if r.transcript.err != nil {
		return nil, r.transcript.err
	}

	r.consumed = true

	return &http.Response{
		StatusCode: r.pair.Response.Status,
		Header:     http.Header(r.pair.Response.Headers),
		Body:       io.NopCloser(bytes.NewReader(r.pair.Response.Body)),
		Request:    request,
	}, nil
}

//nolint:ireturn // Required by the Paho client interface.
func (c *mqttReplayClient) Subscribe(
	topic string,
	_ byte,
	_ mqtt.MessageHandler,
) mqtt.Token {
	c.transcript.accept(mqttReplayFrame{Direction: mqttFixtureClient, Action: "subscribe", Topic: topic})

	return nil
}

//nolint:ireturn // Required by the Paho client interface.
func (c *mqttReplayClient) Unsubscribe(topics ...string) mqtt.Token {
	for _, topic := range topics {
		c.transcript.accept(mqttReplayFrame{Direction: mqttFixtureClient, Action: "unsubscribe", Topic: topic})
	}

	return nil
}
func (c *mqttReplayClient) Disconnect(uint) {
	c.transcript.accept(mqttReplayFrame{Direction: mqttFixtureClient, Action: "disconnect"})
}

//nolint:cyclop,funlen,gocognit,gocyclo // This test replays the full paired MQTT and HTTP lifecycle in transcript order.
func TestSyntheticMQTTPairedTranscript(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../tests/replay/fixtures/mqtt/synthetic/owner-device-session.synthetic.json")
	if err != nil {
		t.Fatal(err)
	}

	var replay mqttReplayTranscript
	if err := json.Unmarshal(data, &replay); err != nil {
		t.Fatal(err)
	}

	if replay.Provenance == "" {
		t.Fatal("missing synthetic provenance")
	}

	configData, err := os.ReadFile("../../tests/replay/fixtures/device-sharing/synthetic/remaining-operations.synthetic.json")
	if err != nil {
		t.Fatal(err)
	}

	var allPairs []mqttHTTPPair
	if err := json.Unmarshal(configData, &allPairs); err != nil {
		t.Fatal(err)
	}

	var configPair mqttHTTPPair

	for _, pair := range allPairs {
		if pair.OperationID == "getMessageQueueConfig" {
			configPair = pair

			break
		}
	}

	if configPair.OperationID == "" {
		t.Fatal("missing paired MQTT config HTTP exchange")
	}

	httpReplay := &mqttHTTPReplay{transcript: &replay, pair: configPair}

	var broker *mqttReplayClient

	client, err := NewClient(
		WithHTTPTransport(httpReplay),
		WithClientID("synthetic-client-id"),
		WithCloudAPIURL("https://api.example.invalid"),
		WithMQTTClientFactory(func(options *mqtt.ClientOptions) mqtt.Client {
			if len(options.Servers) != 1 ||
				options.Servers[0].String() != "ssl://mqtt.example.invalid:8883" ||
				options.ClientID != "synthetic-mqtt-client" ||
				options.Username != "synthetic-user" ||
				options.Password != "synthetic-password" ||
				options.OnConnect == nil ||
				options.DefaultPublishHandler == nil {
				replay.err = errTestMQTTFactoryOptions

				return nil
			}

			replay.accept(mqttReplayFrame{Direction: mqttFixtureClient, Action: "factory", Topic: options.Servers[0].String(), Payload: options.ClientID})
			broker = &mqttReplayClient{transcript: &replay, options: options}

			return broker
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	session := client.NewSession(Tokens{
		AccessToken:  "synthetic-access-token",
		RefreshToken: "synthetic-refresh-token",
		ExpireTime:   time.Now().Add(time.Hour).UnixMilli(),
	})
	queue := session.MessageQueue

	ctx := context.Background()

	if _, err := queue.AddMessageListener(ctx, AddMessageListenerRequest{
		Topic: mqttFixtureGeneral,
		Callback: func(topic string, event any) {
			parsed, ok := event.(Event)
			if !ok {
				replay.err = testMismatchf("owner callback type %T", event)

				return
			}

			replay.accept(mqttReplayFrame{Direction: "callback", Action: "owner-event", Topic: topic, Payload: parsed.GetDeviceID()})
		},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := queue.AddDeviceListener(ctx, AddDeviceListenerRequest{DeviceID: mqttFixtureDevice1, Callback: func(deviceID string, event Event) {
		replay.accept(mqttReplayFrame{Direction: "callback", Action: "device-event", Topic: deviceID, Payload: event.GetDeviceID()})
	}}); err != nil {
		t.Fatal(err)
	}

	if result, err := queue.Start(ctx, MessageQueueStartRequest{}); err != nil || !result.Success {
		t.Fatalf("Start = %+v, %v", result, err)
	}

	if !httpReplay.consumed || broker == nil {
		t.Fatal("paired HTTP config or MQTT factory was not consumed")
	}

	for range 2 {
		if replay.next >= len(replay.Frames) {
			t.Fatal("missing inbound frame")
		}

		frame := replay.Frames[replay.next]
		if frame.Direction != "server" || frame.Action != "message" {
			t.Fatalf("next frame = %+v, want inbound message", frame)
		}

		replay.accept(frame)
		broker.options.DefaultPublishHandler(broker, syntheticMQTTMessage{topic: frame.Topic, payload: []byte(frame.Payload)})

		if replay.err != nil {
			t.Fatal(replay.err)
		}
	}

	if _, err := queue.RemoveDeviceListener(ctx, RemoveDeviceListenerRequest{DeviceID: mqttFixtureDevice1}); err != nil {
		t.Fatal(err)
	}

	if _, err := queue.Stop(ctx, MessageQueueStopRequest{}); err != nil {
		t.Fatal(err)
	}

	if err := replay.verifyConsumed(); err != nil {
		t.Fatal(err)
	}

	if response, err := httpReplay.RoundTrip(&http.Request{}); err == nil || response != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}

		t.Fatalf("duplicate HTTP config request returned %v, %v", response, err)
	}
	// Negative checks prove that the transcript has no response/frame fallback.
	replay.accept(mqttReplayFrame{Direction: mqttFixtureClient, Action: "disconnect"})

	if replay.err == nil {
		t.Fatal("duplicate frame accepted")
	}

	missing := mqttReplayTranscript{Frames: []mqttReplayFrame{{Direction: mqttFixtureClient, Action: mqttFixtureConnect}}}
	if err := missing.verifyConsumed(); err == nil {
		t.Fatal("unconsumed MQTT frame accepted")
	}

	wrong := mqttReplayTranscript{Frames: []mqttReplayFrame{{Direction: mqttFixtureClient, Action: mqttFixtureConnect}}}
	wrong.accept(mqttReplayFrame{Direction: mqttFixtureClient, Action: "subscribe"})

	if wrong.err == nil {
		t.Fatal("unexpected MQTT frame accepted")
	}
}
