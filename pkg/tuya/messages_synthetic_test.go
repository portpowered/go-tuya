package tuya

import (
	"context"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Synthetic MQTT messages exercise session-owned listener state without a
// provider account or a live broker.
type syntheticMQTTMessage struct {
	topic   string
	payload []byte
}

func (m syntheticMQTTMessage) Duplicate() bool   { return false }
func (m syntheticMQTTMessage) Qos() byte         { return 0 }
func (m syntheticMQTTMessage) Retained() bool    { return false }
func (m syntheticMQTTMessage) Topic() string     { return m.topic }
func (m syntheticMQTTMessage) MessageID() uint16 { return 0 }
func (m syntheticMQTTMessage) Payload() []byte   { return m.payload }
func (m syntheticMQTTMessage) Ack()              {}

type syntheticMQTTClient struct {
	mqtt.Client
	subscriptions   []string
	unsubscriptions []string
	disconnects     int
}

func (*syntheticMQTTClient) IsConnected() bool { return true }
func (c *syntheticMQTTClient) Subscribe(topic string, _ byte, _ mqtt.MessageHandler) mqtt.Token {
	c.subscriptions = append(c.subscriptions, topic)
	return nil
}
func (c *syntheticMQTTClient) Unsubscribe(topics ...string) mqtt.Token {
	c.unsubscriptions = append(c.unsubscriptions, topics...)
	return nil
}
func (c *syntheticMQTTClient) Disconnect(uint) { c.disconnects++ }

func TestSyntheticMessageQueueListeners(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	queue := client.NewSession(Tokens{}).MessageQueue
	state := queue.State
	state.mqConfig.DeviceTopic = "cloud/device/{devId}/in/channel"
	broker := &syntheticMQTTClient{}
	state.mqttClient = broker
	ctx := context.Background()
	if got, err := queue.AddMessageListener(ctx, AddMessageListenerRequest{Topic: "general"}); err != nil || got.Success {
		t.Fatalf("nil message callback: %+v, %v", got, err)
	}
	if got, err := queue.AddDeviceListener(ctx, AddDeviceListenerRequest{DeviceID: "device-1"}); err != nil || got.Message != "Callback function cannot be nil" {
		t.Fatalf("nil device callback: %+v, %v", got, err)
	}
	general := make(chan interface{}, 1)
	device := make(chan Event, 1)
	if got, err := queue.AddMessageListener(ctx, AddMessageListenerRequest{Topic: "general", Callback: func(_ string, event interface{}) { general <- event }}); err != nil || !got.Success {
		t.Fatalf("add message callback: %+v, %v", got, err)
	}
	if got, err := queue.AddDeviceListener(ctx, AddDeviceListenerRequest{DeviceID: "device-1", Callback: func(_ string, event Event) { device <- event }}); err != nil || got.Message == "" {
		t.Fatalf("add device callback: %+v, %v", got, err)
	}
	if len(broker.subscriptions) != 1 || broker.subscriptions[0] != "cloud/device/device-1/in/channel/sta" {
		t.Fatalf("subscriptions = %v", broker.subscriptions)
	}
	state.onConnect(broker)
	if len(broker.subscriptions) != 3 {
		t.Fatalf("reconnect subscriptions = %v", broker.subscriptions)
	}
	message := []byte(`{"protocol":4,"data":{"devId":"device-1","status":[]}}`)
	state.onMessage(broker, syntheticMQTTMessage{topic: "general", payload: message})
	select {
	case got := <-general:
		if got == nil {
			t.Fatal("nil parsed event")
		}
	case <-time.After(time.Second):
		t.Fatal("general listener did not receive message")
	}
	state.onMessage(broker, syntheticMQTTMessage{topic: "cloud/device/device-1/in/channel/sta", payload: message})
	select {
	case got := <-device:
		if got.GetDeviceID() != "device-1" {
			t.Fatalf("device ID = %q", got.GetDeviceID())
		}
	case <-time.After(time.Second):
		t.Fatal("device listener did not receive message")
	}
	state.onMessage(broker, syntheticMQTTMessage{topic: "general", payload: []byte("not JSON")})
	if _, err := queue.RemoveMessageListener(ctx, RemoveMessageListenerRequest{Topic: "general"}); err != nil {
		t.Fatal(err)
	}
	if _, err := queue.RemoveDeviceListener(ctx, RemoveDeviceListenerRequest{DeviceID: "device-1"}); err != nil {
		t.Fatal(err)
	}
	if len(broker.unsubscriptions) != 1 || broker.unsubscriptions[0] != "cloud/device/device-1/in/channel/sta" {
		t.Fatalf("unsubscriptions = %v", broker.unsubscriptions)
	}
	if got := state.getDeviceTopic("device-1", true); got != "cloud/device/device-1/in/channel/pen" {
		t.Fatalf("local topic = %q", got)
	}
	if got := state.extractDeviceIDFromTopic("cloud/device/device-1/in/channel/sta"); got != "device-1" {
		t.Fatalf("extracted device = %q", got)
	}
}

func TestSyntheticMessageQueueLifecycle(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	queue := client.NewSession(Tokens{}).MessageQueue
	ctx := context.Background()
	if got, err := queue.Stop(ctx, MessageQueueStopRequest{}); err != nil || !got.Success {
		t.Fatalf("stopped queue: %+v, %v", got, err)
	}
	if got, err := queue.RefreshMQ(ctx, RefreshMQRequest{}); err != nil || got.Message != "Message queue is not running" {
		t.Fatalf("stopped refresh: %+v, %v", got, err)
	}
	state := queue.State
	state.isRunning.Store(true)
	state.stopCh = make(chan struct{})
	state.reconnectCh = make(chan struct{}, 1)
	state.mqttClient = &syntheticMQTTClient{}
	if got, err := queue.RefreshMQ(ctx, RefreshMQRequest{}); err != nil || got.Message != "MQTT refresh initiated" {
		t.Fatalf("refresh: %+v, %v", got, err)
	}
	if len(state.reconnectCh) != 1 {
		t.Fatal("refresh did not enqueue reconnect")
	}
	state.onConnectionLost(state.mqttClient, context.Canceled)
	if len(state.reconnectCh) != 1 {
		t.Fatal("duplicate reconnect queued")
	}
	if got, err := queue.Stop(ctx, MessageQueueStopRequest{}); err != nil || !got.Success {
		t.Fatalf("stop: %+v, %v", got, err)
	}
	if state.mqttClient.(*syntheticMQTTClient).disconnects != 1 {
		t.Fatal("broker was not disconnected")
	}
}
