package tuya

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
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
		r.err = fmt.Errorf("unexpected MQTT frame after transcript exhaustion: %+v", frame)
		return
	}
	if !reflect.DeepEqual(frame, r.Frames[r.next]) {
		r.err = fmt.Errorf("MQTT frame %d = %+v, want %+v", r.next, frame, r.Frames[r.next])
		return
	}
	r.next++
}

func (r *mqttReplayTranscript) verifyConsumed() error {
	if r.err != nil {
		return r.err
	}
	if r.next != len(r.Frames) {
		return fmt.Errorf("consumed %d of %d MQTT frames", r.next, len(r.Frames))
	}
	return nil
}

type mqttReplayClient struct {
	mqtt.Client
	transcript *mqttReplayTranscript
	options    *mqtt.ClientOptions
}

func (*mqttReplayClient) IsConnected() bool { return true }
func (c *mqttReplayClient) Connect() mqtt.Token {
	c.transcript.accept(mqttReplayFrame{Direction: "client", Action: "connect", Topic: c.options.Servers[0].String()})
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

func (r *mqttHTTPReplay) RoundTrip(request *http.Request) (*http.Response, error) {
	if r.consumed {
		return nil, fmt.Errorf("duplicate MQTT config HTTP exchange")
	}
	if request.Method != r.pair.Request.Method || request.URL.Scheme+"://"+request.URL.Host != r.pair.Request.Origin || request.URL.EscapedPath() != r.pair.Request.Path || len(request.URL.Query()) != len(r.pair.Request.Query) {
		return nil, fmt.Errorf("MQTT config request does not match paired method/origin/path/query")
	}
	for name, values := range r.pair.Request.Headers {
		if len(values) != 1 {
			return nil, fmt.Errorf("unsupported fixture header %q", name)
		}
		value := request.Header.Get(name)
		switch values[0] {
		case "<uuid>":
			if !replayUUIDPattern.MatchString(value) {
				return nil, fmt.Errorf("invalid request ID")
			}
		case "<tuya-signature>":
			if !replaySignaturePattern.MatchString(value) {
				return nil, fmt.Errorf("invalid signature")
			}
		case "<unix-millis>":
			stamp, err := strconv.ParseInt(value, 10, 64)
			if err != nil || time.Since(time.UnixMilli(stamp)) > 5*time.Minute || time.Until(time.UnixMilli(stamp)) > 5*time.Minute {
				return nil, fmt.Errorf("invalid timestamp")
			}
		default:
			if value != values[0] {
				return nil, fmt.Errorf("header %s = %q, want %q", name, value, values[0])
			}
		}
	}
	if r.pair.Request.Body != "<encrypted-json>" {
		return nil, fmt.Errorf("unexpected config fixture body rule")
	}
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	var encrypted struct {
		Encdata string `json:"encdata"`
	}
	if err := json.Unmarshal(body, &encrypted); err != nil || encrypted.Encdata == "" {
		return nil, fmt.Errorf("missing encrypted config request body: %v", err)
	}
	requestID := request.Header.Get("X-requestId")
	hash := md5.Sum([]byte(requestID + "synthetic-refresh-token"))
	plain, err := aesGCMDecrypt(encrypted.Encdata, secretGenerating(requestID, "", hex.EncodeToString(hash[:])))
	if err != nil {
		return nil, fmt.Errorf("decrypt config request: %w", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(plain), &decoded); err != nil {
		return nil, err
	}
	linkID, ok := decoded["linkId"].(string)
	if !ok || len(decoded) != 1 || !replayUUIDPattern.MatchString(linkID) || r.pair.Request.PlainBody["linkId"] != "<uuid>" {
		return nil, fmt.Errorf("config request body = %#v", decoded)
	}
	signedHeaders := map[string]string{}
	for _, name := range []string{"X-appKey", "X-requestId", "X-sid", "X-time", "X-token"} {
		if value := request.Header.Get(name); value != "" {
			signedHeaders[name] = value
		}
	}
	if request.Header.Get("X-sign") != restfulSign(hex.EncodeToString(hash[:]), "", encrypted.Encdata, signedHeaders) {
		return nil, fmt.Errorf("config request signature does not authenticate headers and encrypted body")
	}
	r.transcript.accept(mqttReplayFrame{Direction: "client", Action: "http-request", Topic: r.pair.Request.Path, Payload: r.pair.OperationID})
	if r.transcript.err != nil {
		return nil, r.transcript.err
	}
	r.transcript.accept(mqttReplayFrame{Direction: "server", Action: "http-response", Topic: r.pair.Request.Path, Payload: fmt.Sprint(r.pair.Response.Status)})
	if r.transcript.err != nil {
		return nil, r.transcript.err
	}
	r.consumed = true
	return &http.Response{StatusCode: r.pair.Response.Status, Header: http.Header(r.pair.Response.Headers), Body: io.NopCloser(bytes.NewReader(r.pair.Response.Body)), Request: request}, nil
}
func (c *mqttReplayClient) Subscribe(topic string, _ byte, _ mqtt.MessageHandler) mqtt.Token {
	c.transcript.accept(mqttReplayFrame{Direction: "client", Action: "subscribe", Topic: topic})
	return nil
}
func (c *mqttReplayClient) Unsubscribe(topics ...string) mqtt.Token {
	for _, topic := range topics {
		c.transcript.accept(mqttReplayFrame{Direction: "client", Action: "unsubscribe", Topic: topic})
	}
	return nil
}
func (c *mqttReplayClient) Disconnect(uint) {
	c.transcript.accept(mqttReplayFrame{Direction: "client", Action: "disconnect"})
}

func TestSyntheticMQTTPairedTranscript(t *testing.T) {
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
			if len(options.Servers) != 1 || options.Servers[0].String() != "ssl://mqtt.example.invalid:8883" || options.ClientID != "synthetic-mqtt-client" || options.Username != "synthetic-user" || options.Password != "synthetic-password" || options.OnConnect == nil || options.DefaultPublishHandler == nil {
				replay.err = fmt.Errorf("MQTT factory options do not match paired HTTP config")
				return nil
			}
			replay.accept(mqttReplayFrame{Direction: "client", Action: "factory", Topic: options.Servers[0].String(), Payload: options.ClientID})
			broker = &mqttReplayClient{transcript: &replay, options: options}
			return broker
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	queue := client.NewSession(Tokens{AccessToken: "synthetic-access-token", RefreshToken: "synthetic-refresh-token", ExpireTime: time.Now().Add(time.Hour).UnixMilli()}).MessageQueue
	ctx := context.Background()
	if _, err := queue.AddMessageListener(ctx, AddMessageListenerRequest{Topic: "general", Callback: func(topic string, event interface{}) {
		parsed, ok := event.(Event)
		if !ok {
			replay.err = fmt.Errorf("owner callback type %T", event)
			return
		}
		replay.accept(mqttReplayFrame{Direction: "callback", Action: "owner-event", Topic: topic, Payload: parsed.GetDeviceID()})
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := queue.AddDeviceListener(ctx, AddDeviceListenerRequest{DeviceID: "device-1", Callback: func(deviceID string, event Event) {
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
	for i := 0; i < 2; i++ {
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
	if _, err := queue.RemoveDeviceListener(ctx, RemoveDeviceListenerRequest{DeviceID: "device-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := queue.Stop(ctx, MessageQueueStopRequest{}); err != nil {
		t.Fatal(err)
	}
	if err := replay.verifyConsumed(); err != nil {
		t.Fatal(err)
	}
	if response, err := httpReplay.RoundTrip(&http.Request{}); err == nil || response != nil {
		t.Fatalf("duplicate HTTP config request returned %v, %v", response, err)
	}
	// Negative checks prove that the transcript has no response/frame fallback.
	replay.accept(mqttReplayFrame{Direction: "client", Action: "disconnect"})
	if replay.err == nil {
		t.Fatal("duplicate frame accepted")
	}
	missing := mqttReplayTranscript{Frames: []mqttReplayFrame{{Direction: "client", Action: "connect"}}}
	if err := missing.verifyConsumed(); err == nil {
		t.Fatal("unconsumed MQTT frame accepted")
	}
	wrong := mqttReplayTranscript{Frames: []mqttReplayFrame{{Direction: "client", Action: "connect"}}}
	wrong.accept(mqttReplayFrame{Direction: "client", Action: "subscribe"})
	if wrong.err == nil {
		t.Fatal("unexpected MQTT frame accepted")
	}
}
