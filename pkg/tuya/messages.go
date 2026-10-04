package tuya

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"

	wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)

const (
	mqttDisconnectQuiesceMillis    = 250
	mqttConfigRefreshInterval      = 2 * time.Hour
	mqttExpiryRefreshBufferSeconds = 60
	minimumMQTTExpirySeconds       = 1
	reconnectBackoffMultiplier     = 2
	maximumReconnectBackoffSeconds = 60
)

// GetMessageQueueConfig retrieves MQTT configuration from Tuya API
// https://developer.tuya.com/en/docs/cloud/c2c2630d7c?id=Kb68mozbi3foh
func (s *SharingMessageQueueImpl) GetMessageQueueConfig(ctx context.Context) (MessageQueueConfig, error) {
	linkID := uuid.New().String()

	body, err := wireRequestMap(wire.MessageQueueConfigBody{LinkId: linkID, AdditionalProperties: nil})
	if err != nil {
		return MessageQueueConfig{}, err
	}

	resp, err := s.Client.EncryptedClient.requestOperation(ctx, wire.OperationGetMessageQueueConfig(), nil,
		map[string]any{},
		body,
		&MessageQueueStartRequest{Request: Request{AuthorizationContext: nil, DoNotRefreshToken: false}})
	if err != nil {
		return MessageQueueConfig{}, err
	}

	wireResponse, err := decodeWireResponse[wire.MessageQueueConfigEnvelope](resp)
	if err != nil {
		return MessageQueueConfig{}, fmt.Errorf("failed to parse MQTT config response: %w", err)
	}

	var wireConfig wire.MessageQueueConfiguration
	if wireResponse.Result != nil {
		wireConfig = *wireResponse.Result
	}

	config := MessageQueueConfig{
		URL:         dereference(wireConfig.Url),
		ClientID:    dereference(wireConfig.ClientId),
		Username:    dereference(wireConfig.Username),
		Password:    dereference(wireConfig.Password),
		ExpireTime:  dereference(wireConfig.ExpireTime),
		OwnerTopic:  "",
		DeviceTopic: "",
	}

	if wireConfig.Topic != nil {
		if wireConfig.Topic.OwnerId != nil {
			config.OwnerTopic = dereference(wireConfig.Topic.OwnerId.Sub)
		}

		if wireConfig.Topic.DevId != nil {
			config.DeviceTopic = dereference(wireConfig.Topic.DevId.Sub)
		}
	}

	return config, nil
}

// MessageQueueConfig represents the MQTT configuration from Tuya API.
type MessageQueueConfig struct {
	URL string `json:"url"`
	//nolint:tagliatelle // OpenAPI defines these provider response keys in camelCase.
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
	//nolint:tagliatelle // OpenAPI defines these provider response keys in camelCase.
	ExpireTime int64 `json:"expireTime"`
	//nolint:tagliatelle // AsyncAPI defines these provider topic keys in camelCase.
	OwnerTopic string `json:"ownerTopic"`
	//nolint:tagliatelle // AsyncAPI defines these provider topic keys in camelCase.
	DeviceTopic string `json:"deviceTopic"`
}

func (config MessageQueueConfig) isEmpty() bool {
	return config.URL == "" &&
		config.ClientID == "" &&
		config.Username == "" &&
		config.Password == "" &&
		config.ExpireTime == 0 &&
		config.OwnerTopic == "" &&
		config.DeviceTopic == ""
}

// MQTTClientFactory creates the MQTT edge client used by a session's message
// queue. It can be replaced with a test double or custom Paho client.
type MQTTClientFactory func(*mqtt.ClientOptions) mqtt.Client

func newMQTTClient(options *mqtt.ClientOptions) mqtt.Client { //nolint:ireturn // Paho constructs and exposes clients through this interface.
	return mqtt.NewClient(options)
}

// MessageListener represents a callback function for MQTT messages.
type MessageListener func(topic string, message any)

// DeviceListener represents a callback function used to activate event listening for device events.
type DeviceListener func(deviceID string, status Event)

// mqttState holds the internal MQTT state for a SharingMessageQueueImpl instance.
type mqttState struct {
	// MQTT client and configuration
	mqttClient  mqtt.Client
	mqConfig    MessageQueueConfig
	isRunning   atomic.Bool
	stopCh      chan struct{}
	reconnectCh chan struct{}
	doneCh      chan struct{}
	connect     func(context.Context, *SharingMessageQueueImpl) error
	cancel      context.CancelFunc
	connected   atomic.Bool
	statusMux   sync.RWMutex
	lastError   error

	// Message listeners
	messageListeners map[string][]MessageListener
	deviceListeners  map[string][]DeviceListener
	listenersMux     sync.RWMutex
	deliveryMux      sync.Mutex

	// Lifecycle management
	lifecycleMux sync.Mutex
}

// MessageQueueStatus reports whether the session queue is running and connected,
// and the most recent connection error. A successful connection clears LastError.
type MessageQueueStatus struct {
	Running   bool
	Connected bool
	LastError error
}

// Status returns a snapshot of the message queue lifecycle.
func (s *SharingMessageQueueImpl) Status() MessageQueueStatus {
	state := s.State

	state.statusMux.RLock()
	defer state.statusMux.RUnlock()

	return MessageQueueStatus{Running: state.isRunning.Load(), Connected: state.connected.Load(), LastError: state.lastError}
}

func (state *mqttState) recordConnection(err error) {
	state.connected.Store(err == nil)
	state.statusMux.Lock()
	state.lastError = err
	state.statusMux.Unlock()
}

func (state *mqttState) connectOnce(ctx context.Context, queue *SharingMessageQueueImpl) error {
	var err error
	if state.connect != nil {
		err = state.connect(ctx, queue)
	} else {
		err = state.connectMQTT(ctx, queue)
	}

	var classified *ClientError
	if err != nil && !errors.As(err, &classified) {
		return clientError(ErrorTransport, err)
	}

	return err
}

// getOrCreateState gets or creates the MQTT state for a SharingMessageQueueImpl instance

// Start initializes and starts the MQTT message queue.
func (s *SharingMessageQueueImpl) Start(ctx context.Context, _ MessageQueueStartRequest) (MessageQueueStartResponse, error) {
	state := s.State

	state.lifecycleMux.Lock()
	defer state.lifecycleMux.Unlock()

	if state.isRunning.Load() {
		return MessageQueueStartResponse{
			Success: true,
			Message: "Message queue already running",
		}, nil
	}

	// Initialize internal state
	state.stopCh = make(chan struct{})
	state.reconnectCh = make(chan struct{}, 1)
	state.doneCh = make(chan struct{})
	loopCtx, cancel := context.WithCancel(ctx)

	state.cancel = cancel

	err := state.connectOnce(loopCtx, s)
	if err != nil {
		state.recordConnection(err)
		cancel()

		state.cancel = nil
		state.stopCh = nil
		state.doneCh = nil

		return MessageQueueStartResponse{Success: false, Message: err.Error()}, err
	}

	state.recordConnection(nil)
	state.isRunning.Store(true)

	// Start MQTT management goroutine
	go state.runMQTTLoop(loopCtx, s)

	return MessageQueueStartResponse{
		Success: true,
		Message: "Message queue started successfully",
	}, nil
}

// Stop gracefully shuts down the MQTT message queue.
func (s *SharingMessageQueueImpl) Stop(_ context.Context, _ MessageQueueStopRequest) (MessageQueueStopResponse, error) {
	state := s.State

	state.lifecycleMux.Lock()
	defer state.lifecycleMux.Unlock()

	if state.stopCh == nil {
		return MessageQueueStopResponse{
			Success: true,
			Message: "Message queue already stopped",
		}, nil
	}

	// Signal stop to background goroutine
	close(state.stopCh)

	if state.cancel != nil {
		state.cancel()
		state.cancel = nil
	}

	if state.doneCh != nil {
		<-state.doneCh
		state.doneCh = nil
	}

	state.stopCh = nil
	state.isRunning.Store(false)

	// Disconnect MQTT client
	if state.mqttClient != nil && state.mqttClient.IsConnected() {
		state.mqttClient.Disconnect(mqttDisconnectQuiesceMillis)
	}

	state.connected.Store(false)

	return MessageQueueStopResponse{
		Success: true,
		Message: "Message queue stopped successfully",
	}, nil
}

// AddMessageListener adds a general message listener for a specific topic.
func (s *SharingMessageQueueImpl) AddMessageListener(_ context.Context, req AddMessageListenerRequest) (AddMessageListenerResponse, error) {
	state := s.State

	state.listenersMux.Lock()
	defer state.listenersMux.Unlock()

	if req.Callback == nil {
		return AddMessageListenerResponse{
			Success: false,
			Message: "Callback function cannot be nil",
		}, nil
	}

	// Convert callback signature
	listener := MessageListener(req.Callback)

	// Add listener to topic
	state.messageListeners[req.Topic] = append(state.messageListeners[req.Topic], listener)

	return AddMessageListenerResponse{
		Success: true,
		Message: "Message listener added for topic: " + req.Topic,
	}, nil
}

// RemoveMessageListener removes a message listener for a specific topic.
func (s *SharingMessageQueueImpl) RemoveMessageListener(_ context.Context, req RemoveMessageListenerRequest) (RemoveMessageListenerResponse, error) {
	state := s.State

	state.listenersMux.Lock()
	defer state.listenersMux.Unlock()

	delete(state.messageListeners, req.Topic)

	return RemoveMessageListenerResponse{
		Success: true,
		Message: "Message listeners removed for topic: " + req.Topic,
	}, nil
}

// RefreshMQ forces a refresh of the MQTT connection.
func (s *SharingMessageQueueImpl) RefreshMQ(_ context.Context, _ RefreshMQRequest) (RefreshMQResponse, error) {
	state := s.State

	if !state.isRunning.Load() {
		return RefreshMQResponse{
			Message: "Message queue is not running",
		}, nil
	}

	// Signal reconnection
	select {
	case state.reconnectCh <- struct{}{}:
	default: // Channel already has a pending signal.
	}

	return RefreshMQResponse{
		Message: "MQTT refresh initiated",
	}, nil
}

// AddDeviceListener adds a device-specific listener.
func (s *SharingMessageQueueImpl) AddDeviceListener(_ context.Context, req AddDeviceListenerRequest) (AddDeviceListenerResponse, error) {
	state := s.State

	state.listenersMux.Lock()
	defer state.listenersMux.Unlock()

	if req.Callback == nil {
		return AddDeviceListenerResponse{
			Message: "Callback function cannot be nil",
		}, nil
	}

	// Convert callback signature
	listener := DeviceListener(req.Callback)

	// Add listener for device
	state.deviceListeners[req.DeviceID] = append(state.deviceListeners[req.DeviceID], listener)

	// Subscribe to device topic if MQTT is connected
	if state.mqttClient != nil && state.mqttClient.IsConnected() && !state.mqConfig.isEmpty() {
		deviceTopic := strings.ReplaceAll(state.mqConfig.DeviceTopic, "{devId}", req.DeviceID)
		subscribeChannel(state.mqttClient, wire.MQTTChannelDeviceStatus, deviceTopic)
	}

	return AddDeviceListenerResponse{
		Message: "Device listener added for device: " + req.DeviceID,
	}, nil
}

// RemoveDeviceListener removes a device-specific listener.
func (s *SharingMessageQueueImpl) RemoveDeviceListener(_ context.Context, req RemoveDeviceListenerRequest) (RemoveDeviceListenerResponse, error) {
	state := s.State

	state.listenersMux.Lock()
	defer state.listenersMux.Unlock()

	delete(state.deviceListeners, req.DeviceID)

	// Unsubscribe from device topic if MQTT is connected
	if state.mqttClient != nil && state.mqttClient.IsConnected() && !state.mqConfig.isEmpty() {
		deviceTopic := strings.ReplaceAll(state.mqConfig.DeviceTopic, "{devId}", req.DeviceID)
		unsubscribeChannel(state.mqttClient, wire.MQTTChannelDeviceStatus, deviceTopic)
	}

	return RemoveDeviceListenerResponse{
		Message: "Device listener removed for device: " + req.DeviceID,
	}, nil
}

// runMQTTLoop manages the MQTT connection lifecycle.
func (state *mqttState) runMQTTLoop(ctx context.Context, queue *SharingMessageQueueImpl) {
	defer close(state.doneCh)
	defer state.isRunning.Store(false)

	backoffSeconds := 1

	ticker := time.NewTicker(mqttConfigRefreshInterval) // Refresh every 2 hours
	defer ticker.Stop()

	for {
		if !state.waitForMQTTTrigger(ctx, ticker) {
			return
		}

		err := state.connectOnce(ctx, queue)
		if err != nil {
			state.recordConnection(err)
			log.Printf("MQTT connection failed; retrying in %d seconds", backoffSeconds)

			if !state.waitBeforeReconnect(ctx, backoffSeconds) {
				return
			}

			backoffSeconds = min(backoffSeconds*reconnectBackoffMultiplier, maximumReconnectBackoffSeconds)

			state.queueReconnect()

			continue
		}

		state.recordConnection(nil)

		backoffSeconds = 1
	}
}

func (state *mqttState) waitForMQTTTrigger(ctx context.Context, ticker *time.Ticker) bool {
	expiry := time.Duration(max(state.mqConfig.ExpireTime-mqttExpiryRefreshBufferSeconds, minimumMQTTExpirySeconds)) * time.Second

	expiryTimer := time.NewTimer(expiry)
	defer expiryTimer.Stop()

	select {
	case <-state.stopCh:
		return false
	case <-ctx.Done():
		state.recordConnection(ctx.Err())

		return false
	case <-state.reconnectCh:
	case <-ticker.C:
	case <-expiryTimer.C:
	}

	return true
}

func (state *mqttState) waitBeforeReconnect(ctx context.Context, backoffSeconds int) bool {
	select {
	case <-state.stopCh:
		return false
	case <-ctx.Done():
		return false
	case <-time.After(time.Duration(backoffSeconds) * time.Second):
		return true
	}
}

func (state *mqttState) queueReconnect() {
	select {
	case state.reconnectCh <- struct{}{}:
	default:
	}
}

// connectMQTT establishes MQTT connection with current configuration.
func (state *mqttState) connectMQTT(ctx context.Context, queue *SharingMessageQueueImpl) error {
	// Get MQTT configuration from API
	config, err := queue.GetMessageQueueConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get MQTT config: %w", err)
	}

	state.mqConfig = config

	// Disconnect existing client if any
	if state.mqttClient != nil && state.mqttClient.IsConnected() {
		state.mqttClient.Disconnect(mqttDisconnectQuiesceMillis)
	}

	// Create new MQTT client
	opts := mqtt.NewClientOptions()
	opts.AddBroker(config.URL)
	opts.SetClientID(config.ClientID)
	opts.SetUsername(config.Username)
	opts.SetPassword(config.Password)
	opts.SetOnConnectHandler(func(client mqtt.Client) { state.onConnect(client) })
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) { state.onConnectionLost(client, err) })
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) { state.onMessage(client, msg) })

	// Handle SSL/TLS
	if strings.HasPrefix(config.URL, "ssl://") || strings.HasPrefix(config.URL, "tls://") {
		opts.SetTLSConfig(nil) // Use default TLS config
	}

	state.mqttClient = queue.Client.mqttClientFactory(opts)
	if state.mqttClient == nil {
		return errMQTTFactoryReturnedNil
	}

	// Connect
	token := state.mqttClient.Connect()
	select {
	case <-ctx.Done():
		return fmt.Errorf("MQTT connection canceled: %w", ctx.Err())
	case <-token.Done():
		err := token.Error()
		if err != nil {
			return fmt.Errorf("MQTT connection failed: %w", err)
		}
	}

	log.Print("Connected to MQTT broker")

	return nil
}

// Future listener registrations can subscribe to their topics while MQTT is connected.
// onConnect handles MQTT connection established.
func (state *mqttState) onConnect(client mqtt.Client) {
	log.Println("MQTT client connected")

	// Subscribe to topics with existing listeners
	state.listenersMux.RLock()

	for topic := range state.messageListeners {
		subscribeChannel(client, wire.MQTTChannelOwnerEvents, topic)
		log.Print("Subscribed to account event listener")
	}

	for deviceID := range state.deviceListeners {
		deviceTopic := strings.ReplaceAll(state.mqConfig.DeviceTopic, "{devId}", deviceID)
		subscribeChannel(client, wire.MQTTChannelDeviceStatus, deviceTopic)

		log.Print("Subscribed to device event listener")
	}

	state.listenersMux.RUnlock()
}

func subscribeChannel(client mqtt.Client, channel wire.MQTTChannel, runtimeTopic string) {
	client.Subscribe(channelAddress(channel, runtimeTopic), 0, nil)
}

func unsubscribeChannel(client mqtt.Client, channel wire.MQTTChannel, runtimeTopic string) {
	client.Unsubscribe(channelAddress(channel, runtimeTopic))
}

func channelAddress(channel wire.MQTTChannel, runtimeTopic string) string {
	address := string(channel)
	address = strings.ReplaceAll(address, "{ownerTopic}", runtimeTopic)

	return strings.ReplaceAll(address, "{deviceTopic}", runtimeTopic)
}

// onConnectionLost handles MQTT connection lost.
func (state *mqttState) onConnectionLost(_ mqtt.Client, err error) {
	log.Print("MQTT connection lost")
	state.recordConnection(err)

	// Signal reconnection if still running
	if state.isRunning.Load() {
		select {
		case state.reconnectCh <- struct{}{}:
		default:
		}
	}
}

// onMessage handles incoming MQTT messages.
func (state *mqttState) onMessage(_ mqtt.Client, msg mqtt.Message) {
	state.deliveryMux.Lock()
	defer state.deliveryMux.Unlock()

	topic := msg.Topic()
	payload := msg.Payload()

	// Parse message
	var sharingMessage wire.RawSharingMessage

	err := json.Unmarshal(payload, &sharingMessage)
	if err != nil {
		log.Print("Failed to parse MQTT message JSON")

		return
	}

	state.listenersMux.RLock()
	messageListeners := append([]MessageListener(nil), state.messageListeners[topic]...)
	deviceID := state.extractDeviceIDFromTopic(topic)

	var deviceListeners []DeviceListener

	if deviceID != "" {
		deviceListeners = append([]DeviceListener(nil), state.deviceListeners[deviceID]...)
	}

	state.listenersMux.RUnlock()

	if len(messageListeners) == 0 && len(deviceListeners) == 0 {
		return
	}

	evt, err := parseRawSharingMessage(sharingMessage)
	if err != nil {
		log.Print("Failed to parse MQTT event payload")

		return
	}

	for _, listener := range messageListeners {
		listener(topic, evt)
	}

	for _, listener := range deviceListeners {
		listener(deviceID, evt)
	}
}

// getDeviceTopic constructs device topic from device ID and support_local flag.
func (state *mqttState) getDeviceTopic(deviceID string, supportLocal bool) string {
	topic := strings.ReplaceAll(state.mqConfig.DeviceTopic, "{devId}", deviceID)
	if supportLocal {
		// When a device supports local, we do the mapping between the data point id and the more comprehensible name.
		// i.e. dp1 -> led_dimmer_1.
		// To do this, each device needs to maintain the corresponding specification strategy with it for local transformations.
		return channelAddress(wire.MQTTChannelDeviceLocal, topic)
	}

	return channelAddress(wire.MQTTChannelDeviceStatus, topic)
}

// extractDeviceIDFromTopic extracts device ID from topic string.
func (state *mqttState) extractDeviceIDFromTopic(topic string) string {
	// the default topic format is "cloud/device/{devId}/in/24d2afb98121974b6505c40a3a980362"
	parts := strings.Split(topic, "/")
	if len(parts) < 4 || parts[0] != "cloud" || parts[1] != "device" || parts[2] == "" || parts[3] != "in" {
		return ""
	}

	return parts[2]
}
