package tuya

import (
	"context"
	"errors"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Repeated values stay test-local so synthetic fixtures remain independent of production constants.
const (
	messagesFixtureCloudDeviceDevice1InChannelSta = "cloud/device/device-1/in/channel/sta"
	messagesFixtureDevice1                        = "device-1"
	messagesFixtureGeneral                        = "general"
)

// Synthetic MQTT messages exercise session-owned listener state without a
// provider account or a live broker.
type syntheticMQTTMessage struct {
	topic   string
	payload []byte
}

func TestSyntheticMessageQueueStartReportsConnectionFailure(t *testing.T) {
	t.Parallel()

	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}

	queue := client.NewSession(Tokens{}).MessageQueue
	want := errTestSyntheticBroker
	queue.State.connect = func(context.Context, *SharingMessageQueueImpl) error { return want }

	result, err := queue.Start(context.Background(), MessageQueueStartRequest{})
	if !errors.Is(err, want) || result.Success {
		t.Fatalf("Start = %+v, %v; want connection failure", result, err)
	}

	status := queue.Status()
	if status.Running || status.Connected || !errors.Is(status.LastError, want) {
		t.Fatalf("Status = %+v", status)
	}
}

//nolint:cyclop,funlen // The test asserts every synchronization boundary in the stop/reconnect race.
func TestSyntheticMessageQueueStopWaitsForReconnect(t *testing.T) {
	t.Parallel()

	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}

	queue := client.NewSession(Tokens{}).MessageQueue
	entered := make(chan struct{})
	release := make(chan struct{})
	attempts := 0

	queue.State.connect = func(context.Context, *SharingMessageQueueImpl) error {
		attempts++
		if attempts == 1 {
			return nil
		}

		close(entered)
		<-release

		return errTestSyntheticReconnect
	}
	if result, err := queue.Start(context.Background(), MessageQueueStartRequest{}); err != nil || !result.Success {
		t.Fatalf("Start = %+v, %v", result, err)
	}

	if status := queue.Status(); !status.Running || !status.Connected || status.LastError != nil {
		t.Fatalf("initial Status = %+v", status)
	}

	if _, err := queue.RefreshMQ(context.Background(), RefreshMQRequest{}); err != nil {
		t.Fatal(err)
	}

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("reconnect did not start")
	}

	stopped := make(chan error, 1)

	go func() {
		_, err := queue.Stop(context.Background(), MessageQueueStopRequest{})
		stopped <- err
	}()

	select {
	case <-stopped:
		t.Fatal("Stop returned before reconnect exited")
	case <-time.After(20 * time.Millisecond):
	}

	close(release)

	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not wait for loop exit")
	}

	if status := queue.Status(); status.Running || status.Connected || status.LastError == nil {
		t.Fatalf("final Status = %+v", status)
	}
}

//nolint:cyclop // The test checks both callback blocking and release behavior across two concurrent dispatches.
func TestSyntheticMessageDeliveryAppliesBackpressure(t *testing.T) {
	t.Parallel()

	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}

	queue := client.NewSession(Tokens{}).MessageQueue
	entered := make(chan struct{}, 2)
	release := make(chan struct{})

	_, err = queue.AddMessageListener(context.Background(), AddMessageListenerRequest{
		Topic: messagesFixtureGeneral,
		Callback: func(_ string, _ any) {
			entered <- struct{}{}

			<-release
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	message := syntheticMQTTMessage{topic: messagesFixtureGeneral, payload: []byte(`{"protocol":4,"data":{"devId":"device-1","status":[]}}`)}
	firstDone := make(chan struct{})

	go func() { queue.State.onMessage(nil, message); close(firstDone) }()

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first callback did not start")
	}

	secondDone := make(chan struct{})

	go func() { queue.State.onMessage(nil, message); close(secondDone) }()

	select {
	case <-entered:
		t.Fatal("second callback ran while the first was blocked")
	case <-secondDone:
		t.Fatal("second dispatch completed while the first callback was blocked")
	case <-time.After(20 * time.Millisecond):
	}

	close(release)

	for _, done := range []<-chan struct{}{firstDone, secondDone} {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("dispatch did not finish after callback release")
		}
	}
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

//nolint:ireturn // Required by the Paho client interface.
func (c *syntheticMQTTClient) Subscribe(
	topic string,
	_ byte,
	_ mqtt.MessageHandler,
) mqtt.Token {
	c.subscriptions = append(c.subscriptions, topic)

	return nil
}

//nolint:ireturn // Required by the Paho client interface.
func (c *syntheticMQTTClient) Unsubscribe(topics ...string) mqtt.Token {
	c.unsubscriptions = append(c.unsubscriptions, topics...)

	return nil
}
func (c *syntheticMQTTClient) Disconnect(uint) { c.disconnects++ }

//nolint:cyclop,funlen // The test covers listener registration, delivery, reconnect subscription, and removal as one lifecycle.
func TestSyntheticMessageQueueListeners(t *testing.T) {
	t.Parallel()

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
	if got, err := queue.AddMessageListener(ctx, AddMessageListenerRequest{
		Topic: messagesFixtureGeneral,
	}); err != nil || got.Success {
		t.Fatalf("nil message callback: %+v, %v", got, err)
	}

	if got, err := queue.AddDeviceListener(ctx, AddDeviceListenerRequest{
		DeviceID: messagesFixtureDevice1,
	}); err != nil || got.Message != "Callback function cannot be nil" {
		t.Fatalf("nil device callback: %+v, %v", got, err)
	}

	general := make(chan any, 1)
	device := make(chan Event, 1)

	if got, err := queue.AddMessageListener(ctx, AddMessageListenerRequest{
		Topic: messagesFixtureGeneral,
		Callback: func(_ string, event any) {
			general <- event
		},
	}); err != nil || !got.Success {
		t.Fatalf("add message callback: %+v, %v", got, err)
	}

	if got, err := queue.AddDeviceListener(ctx, AddDeviceListenerRequest{
		DeviceID: messagesFixtureDevice1,
		Callback: func(_ string, event Event) {
			device <- event
		},
	}); err != nil || got.Message == "" {
		t.Fatalf("add device callback: %+v, %v", got, err)
	}

	if len(broker.subscriptions) != 1 || broker.subscriptions[0] != messagesFixtureCloudDeviceDevice1InChannelSta {
		t.Fatalf("subscriptions = %v", broker.subscriptions)
	}

	state.onConnect(broker)

	if len(broker.subscriptions) != 3 {
		t.Fatalf("reconnect subscriptions = %v", broker.subscriptions)
	}

	message := []byte(`{"protocol":4,"data":{"devId":"device-1","status":[]}}`)
	state.onMessage(broker, syntheticMQTTMessage{topic: messagesFixtureGeneral, payload: message})

	select {
	case got := <-general:
		if got == nil {
			t.Fatal("nil parsed event")
		}
	case <-time.After(time.Second):
		t.Fatal("general listener did not receive message")
	}

	state.onMessage(broker, syntheticMQTTMessage{topic: messagesFixtureCloudDeviceDevice1InChannelSta, payload: message})

	select {
	case got := <-device:
		if got.GetDeviceID() != messagesFixtureDevice1 {
			t.Fatalf("device ID = %q", got.GetDeviceID())
		}
	case <-time.After(time.Second):
		t.Fatal("device listener did not receive message")
	}

	state.onMessage(broker, syntheticMQTTMessage{topic: messagesFixtureGeneral, payload: []byte("not JSON")})

	if _, err := queue.RemoveMessageListener(ctx, RemoveMessageListenerRequest{Topic: messagesFixtureGeneral}); err != nil {
		t.Fatal(err)
	}

	if _, err := queue.RemoveDeviceListener(ctx, RemoveDeviceListenerRequest{DeviceID: messagesFixtureDevice1}); err != nil {
		t.Fatal(err)
	}

	if len(broker.unsubscriptions) != 1 || broker.unsubscriptions[0] != messagesFixtureCloudDeviceDevice1InChannelSta {
		t.Fatalf("unsubscriptions = %v", broker.unsubscriptions)
	}

	if got := state.getDeviceTopic(messagesFixtureDevice1, true); got != "cloud/device/device-1/in/channel/pen" {
		t.Fatalf("local topic = %q", got)
	}

	if got := state.extractDeviceIDFromTopic(messagesFixtureCloudDeviceDevice1InChannelSta); got != messagesFixtureDevice1 {
		t.Fatalf("extracted device = %q", got)
	}
}

//nolint:cyclop // The test checks stopped, reconnecting, and shutdown states through the public queue methods.
func TestSyntheticMessageQueueLifecycle(t *testing.T) {
	t.Parallel()

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

	if mustAssert[*syntheticMQTTClient](t, state.mqttClient).disconnects != 1 {
		t.Fatal("broker was not disconnected")
	}
}
