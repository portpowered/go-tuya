package tuya

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"

	"github.com/portpowered/go-tuya/pkg/tuya/internal/wire"
)

// GetMessageQueueConfig retrieves MQTT configuration from Tuya API
// https://developer.tuya.com/en/docs/cloud/c2c2630d7c?id=Kb68mozbi3foh
func (s *SharingMessageQueueImpl) GetMessageQueueConfig(ctx context.Context) (MessageQueueConfig, error) {
	linkID := uuid.New().String()

	resp, err := s.Client.EncryptedClient.Post(ctx, wire.RouteGetMessageQueueConfig,
		map[string]interface{}{},
		map[string]interface{}{
			"linkId": linkID,
		},
		&MessageQueueStartRequest{})
	if err != nil {
		return MessageQueueConfig{}, err
	}

	// Parse response using serialize function like devices operations
	respData, err := serialize[MessageQueueConfigResponse](resp)
	if err != nil {
		return MessageQueueConfig{}, fmt.Errorf("failed to parse MQTT config response: %w", err)
	}

	config := respData.ToMessageQueueConfig()
	return config, nil
}

// MessageQueueConfig represents the MQTT configuration from Tuya API
type MessageQueueConfig struct {
	URL         string `json:"url"`
	ClientID    string `json:"clientId"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	ExpireTime  int64  `json:"expireTime"`
	OwnerTopic  string `json:"ownerTopic"`
	DeviceTopic string `json:"deviceTopic"`
}

// MessageQueueConfigResponse represents the API response structure for MQTT topic.
type MessageQueueConfigResponse struct {
	URL        string                      `json:"url"`
	ClientID   string                      `json:"clientId"`
	Username   string                      `json:"username"`
	Password   string                      `json:"password"`
	ExpireTime int64                       `json:"expireTime"`
	Topic      MessageQueueConfigTopicInfo `json:"topic"`
}

// MessageQueueConfigTopicInfo represents the topic configuration structure
type MessageQueueConfigTopicInfo struct {
	OwnerID MessageQueueTopicSubscription `json:"ownerId"`
	DevID   MessageQueueTopicSubscription `json:"devId"`
}

// MessageQueueTopicSubscription represents a topic subscription configuration
type MessageQueueTopicSubscription struct {
	Sub string `json:"sub"`
}

// MQTTClientFactory creates the MQTT edge client used by a session's message
// queue. It can be replaced with a test double or custom Paho client.
type MQTTClientFactory func(*mqtt.ClientOptions) mqtt.Client

func newMQTTClient(options *mqtt.ClientOptions) mqtt.Client {
	return mqtt.NewClient(options)
}

// ToMessageQueueConfig converts the API response to MessageQueueConfig
func (r *MessageQueueConfigResponse) ToMessageQueueConfig() MessageQueueConfig {
	return MessageQueueConfig{
		URL:         r.URL,
		ClientID:    r.ClientID,
		Username:    r.Username,
		Password:    r.Password,
		ExpireTime:  r.ExpireTime,
		OwnerTopic:  r.Topic.OwnerID.Sub,
		DeviceTopic: r.Topic.DevID.Sub,
	}
}

// MessageListener represents a callback function for MQTT messages
type MessageListener func(topic string, message interface{})

// DeviceListener represents a callback function used to activate event listening for device events.
type DeviceListener func(deviceID string, status Event)

// mqttState holds the internal MQTT state for a SharingMessageQueueImpl instance
type mqttState struct {
	// MQTT client and configuration
	mqttClient  mqtt.Client
	mqConfig    MessageQueueConfig
	isRunning   atomic.Bool
	stopCh      chan struct{}
	reconnectCh chan struct{}

	// Message listeners
	messageListeners map[string][]MessageListener
	deviceListeners  map[string][]DeviceListener
	listenersMux     sync.RWMutex

	// Lifecycle management
	lifecycleMux sync.Mutex
}

// getOrCreateState gets or creates the MQTT state for a SharingMessageQueueImpl instance

// Start initializes and starts the MQTT message queue
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
	state.isRunning.Store(true)

	// Start MQTT management goroutine
	go state.runMQTTLoop(ctx, s)

	return MessageQueueStartResponse{
		Success: true,
		Message: "Message queue started successfully",
	}, nil
}

// Stop gracefully shuts down the MQTT message queue
func (s *SharingMessageQueueImpl) Stop(_ context.Context, _ MessageQueueStopRequest) (MessageQueueStopResponse, error) {
	state := s.State
	state.lifecycleMux.Lock()
	defer state.lifecycleMux.Unlock()

	if !state.isRunning.Load() {
		return MessageQueueStopResponse{
			Success: true,
			Message: "Message queue already stopped",
		}, nil
	}

	state.isRunning.Store(false)

	// Signal stop to background goroutine
	close(state.stopCh)

	// Disconnect MQTT client
	if state.mqttClient != nil && state.mqttClient.IsConnected() {
		state.mqttClient.Disconnect(250)
	}

	return MessageQueueStopResponse{
		Success: true,
		Message: "Message queue stopped successfully",
	}, nil
}

// AddMessageListener adds a general message listener for a specific topic
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
		Message: fmt.Sprintf("Message listener added for topic: %s", req.Topic),
	}, nil
}

// RemoveMessageListener removes a message listener for a specific topic
func (s *SharingMessageQueueImpl) RemoveMessageListener(_ context.Context, req RemoveMessageListenerRequest) (RemoveMessageListenerResponse, error) {
	state := s.State
	state.listenersMux.Lock()
	defer state.listenersMux.Unlock()

	delete(state.messageListeners, req.Topic)

	return RemoveMessageListenerResponse{
		Success: true,
		Message: fmt.Sprintf("Message listeners removed for topic: %s", req.Topic),
	}, nil
}

// RefreshMQ forces a refresh of the MQTT connection
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
	default:
		// Channel already has a pending signal
	}

	return RefreshMQResponse{
		Message: "MQTT refresh initiated",
	}, nil
}

// AddDeviceListener adds a device-specific listener
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
	if state.mqttClient != nil && state.mqttClient.IsConnected() && state.mqConfig != (MessageQueueConfig{}) {
		deviceTopic := state.getDeviceTopic(req.DeviceID, false) // Assume support_local = true
		state.mqttClient.Subscribe(deviceTopic, 0, nil)
	}

	return AddDeviceListenerResponse{
		Message: fmt.Sprintf("Device listener added for device: %s", req.DeviceID),
	}, nil
}

// RemoveDeviceListener removes a device-specific listener
func (s *SharingMessageQueueImpl) RemoveDeviceListener(_ context.Context, req RemoveDeviceListenerRequest) (RemoveDeviceListenerResponse, error) {
	state := s.State
	state.listenersMux.Lock()
	defer state.listenersMux.Unlock()

	delete(state.deviceListeners, req.DeviceID)

	// Unsubscribe from device topic if MQTT is connected
	if state.mqttClient != nil && state.mqttClient.IsConnected() && state.mqConfig != (MessageQueueConfig{}) {
		deviceTopic := state.getDeviceTopic(req.DeviceID, false) // Assume support_local = true
		state.mqttClient.Unsubscribe(deviceTopic)
	}

	return RemoveDeviceListenerResponse{
		Message: fmt.Sprintf("Device listener removed for device: %s", req.DeviceID),
	}, nil
}

// runMQTTLoop manages the MQTT connection lifecycle
func (state *mqttState) runMQTTLoop(ctx context.Context, s *SharingMessageQueueImpl) {
	backoffSeconds := 1
	ticker := time.NewTicker(2 * time.Hour) // Refresh every 2 hours
	defer ticker.Stop()

	for {
		select {
		case <-state.stopCh:
			return
		case <-state.reconnectCh:
			// Force reconnection
			if err := state.connectMQTT(ctx, s); err != nil {
				// TODO: terminate and message to the user that the message queue failed.
				_ = err // Log error but continue
			}
			backoffSeconds = 1
		case <-ticker.C:
			// Regular refresh
			if err := state.connectMQTT(ctx, s); err != nil {
				// TODO: terminate and message to the user that the message queue failed.
				_ = err // Log error but continue
			}
			backoffSeconds = 1
		default:
			// Initial connection or retry after error
			err := state.connectMQTT(ctx, s)
			if err != nil {
				log.Printf("Failed to connect to MQTT: %v, retrying in %d seconds", err, backoffSeconds)
				time.Sleep(time.Duration(backoffSeconds) * time.Second)
				backoffSeconds = min(backoffSeconds*2, 60) // Max 60 seconds
			} else {
				backoffSeconds = 1
				// Wait for next event or timer
				select {
				case <-state.stopCh:
					return
				case <-state.reconnectCh:
					continue
				case <-ticker.C:
					continue
				case <-time.After(time.Duration(state.mqConfig.ExpireTime-60) * time.Second):
					continue
				}
			}
		}
	}
}

// connectMQTT establishes MQTT connection with current configuration
func (state *mqttState) connectMQTT(ctx context.Context, s *SharingMessageQueueImpl) error {
	// Get MQTT configuration from API
	config, err := s.GetMessageQueueConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get MQTT config: %w", err)
	}

	state.mqConfig = config

	// Disconnect existing client if any
	if state.mqttClient != nil && state.mqttClient.IsConnected() {
		state.mqttClient.Disconnect(250)
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

	state.mqttClient = s.Client.mqttClientFactory(opts)
	if state.mqttClient == nil {
		return fmt.Errorf("MQTT client factory returned nil")
	}

	// Connect
	token := state.mqttClient.Connect()
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT connection failed: %w", token.Error())
	}

	log.Printf("Connected to MQTT broker: %s", config.URL)
	return nil
}

// TODO: implement a new function to subscribe to new topics as we add new event listeners,
// This should only be called when the MQTT server is connected.
// onConnect handles MQTT connection established
func (state *mqttState) onConnect(client mqtt.Client) {
	log.Println("MQTT client connected")

	// Subscribe to topics with existing listeners
	state.listenersMux.RLock()
	for topic := range state.messageListeners {
		client.Subscribe(topic, 0, nil)
		log.Printf("Subscribed to listener topic: %s", topic)
	}
	for deviceID := range state.deviceListeners {
		topic := state.getDeviceTopic(deviceID, false)
		client.Subscribe(topic, 0, nil)
		log.Printf("Subscribed to device listener topic: %s", topic)
	}
	state.listenersMux.RUnlock()
}

// onConnectionLost handles MQTT connection lost
func (state *mqttState) onConnectionLost(_ mqtt.Client, err error) {
	log.Printf("MQTT connection lost: %v", err)

	// Signal reconnection if still running
	if state.isRunning.Load() {
		select {
		case state.reconnectCh <- struct{}{}:
		default:
		}
	}
}

// onMessage handles incoming MQTT messages
func (state *mqttState) onMessage(_ mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	// Parse message
	var sharingMessage wire.RawSharingMessage
	if err := json.Unmarshal(payload, &sharingMessage); err != nil {
		log.Printf("Failed to parse message JSON: %v", err)
		return
	}
	messageData := make(map[string]interface{}, len(sharingMessage.AdditionalProperties)+3)
	for key, value := range sharingMessage.AdditionalProperties {
		messageData[key] = value
	}
	messageData["protocol"] = float64(sharingMessage.Protocol)
	messageData["data"] = sharingMessage.Data
	if sharingMessage.T != nil {
		messageData["t"] = *sharingMessage.T
	}

	// Handle general message listeners
	state.listenersMux.RLock()
	defer state.listenersMux.RUnlock()
	if listeners, exists := state.messageListeners[topic]; exists {
		evt, err := ParseEvent(messageData)
		if err != nil {
			log.Printf("Failed to parse device state change event JSON: %v", err)
			return
		}
		for _, listener := range listeners {
			go listener(topic, evt)
		}
	}

	// Handle device-specific listeners
	deviceID := state.extractDeviceIDFromTopic(topic)
	if deviceID != "" {
		if listeners, exists := state.deviceListeners[deviceID]; exists {
			evt, err := ParseEvent(messageData)
			if err != nil {
				log.Printf("Failed to parse device state change event JSON: %v", err)
				return
			}
			for _, listener := range listeners {
				go listener(deviceID, evt)
			}
		}
	}

}

// getDeviceTopic constructs device topic from device ID and support_local flag
func (state *mqttState) getDeviceTopic(deviceID string, supportLocal bool) string {
	topic := strings.ReplaceAll(state.mqConfig.DeviceTopic, "{devId}", deviceID)
	if supportLocal {
		// When a device supports local, we do the mapping between the data point id and the more comprehensible name.
		// i.e. dp1 -> led_dimmer_1.
		// To do this, each device needs to maintain the corresponding specification strategy with it for local transformations.
		topic += "/pen"
	} else {
		topic += "/sta"
	}
	return topic
}

// extractDeviceIDFromTopic extracts device ID from topic string
func (state *mqttState) extractDeviceIDFromTopic(topic string) string {
	// the default topic format is "cloud/device/{devId}/in/24d2afb98121974b6505c40a3a980362"
	parts := strings.Split(topic, "/")
	if len(parts) < 4 || parts[0] != "cloud" || parts[1] != "device" || parts[2] == "" || parts[3] != "in" {
		return ""
	}
	return parts[2]
}
