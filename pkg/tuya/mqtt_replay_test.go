package tuya

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

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
}

func (*mqttReplayClient) IsConnected() bool { return true }
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
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	queue := client.NewSession(Tokens{}).MessageQueue
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
	broker := &mqttReplayClient{transcript: &replay}
	queue.State.connect = func(_ context.Context, _ *SharingMessageQueueImpl) error {
		replay.accept(mqttReplayFrame{Direction: "client", Action: "connect", Topic: "ssl://mqtt.example.invalid:8883"})
		queue.State.mqConfig = MessageQueueConfig{URL: "ssl://mqtt.example.invalid:8883", ClientID: "synthetic-mqtt-client", DeviceTopic: "cloud/device/{devId}/in/channel", ExpireTime: 7200}
		queue.State.mqttClient = broker
		queue.State.onConnect(broker)
		return replay.err
	}
	if result, err := queue.Start(ctx, MessageQueueStartRequest{}); err != nil || !result.Success {
		t.Fatalf("Start = %+v, %v", result, err)
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
		queue.State.onMessage(broker, syntheticMQTTMessage{topic: frame.Topic, payload: []byte(frame.Payload)})
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
