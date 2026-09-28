package tuya

import (
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-tuya/pkg/tuya/internal/wire"
)

// This describes the overall message protocol that is sent over via MQTT to the client.
//https://developer.tuya.com/en/docs/iot/message-type?id=Kavqerli65a1u#title-6-Report%20device%20data%20(protocol%201000)

// Protocol constants
const (
	// ProtocolDeviceReport represents device state change reports (protocol 4)
	ProtocolDeviceReport = 4
	// ProtocolOther represents device management events (protocol 20)
	ProtocolOther = 20
)

// Business code constants for device management events
const (
	BizcodeOnline     = "online"
	BizcodeOffline    = "offline"
	BizcodeNameUpdate = "nameUpdate"
	BizcodeDelete     = "delete"
	BizcodeBindUser   = "bindUser"
	// This is when a state change name is updated. i.e. led_dimmer_1 -> led_dimmer_2.
	BizcodeDpNameUpdate = "dpNameUpdate"
)

// Event represents the base interface for all event types
type Event interface {
	GetDeviceID() string
	GetEventType() string
}

// DeviceStateChangeEvent represents a device state change event (protocol 4).
type DeviceStateChangeEvent struct {
	DataID     string               `json:"dataId"`
	DeviceID   string               `json:"devId"`
	ProductKey string               `json:"productKey"`
	Status     []DeviceStatusChange `json:"status"`
}

// DeviceStatusChange represents a single status change within a device state change event
type DeviceStatusChange struct {
	Code      string      `json:"code"`
	Value     interface{} `json:"value"`
	Timestamp int64       `json:"t"`
}

// GetDeviceID returns the device ID for the state change event
func (e *DeviceStateChangeEvent) GetDeviceID() string {
	return e.DeviceID
}

// GetEventType returns the event type
func (e *DeviceStateChangeEvent) GetEventType() string {
	return "device_state_change"
}

// HasStatusCode checks if the event contains a specific status code change
func (e *DeviceStateChangeEvent) HasStatusCode(code string) bool {
	for _, status := range e.Status {
		if status.Code == code {
			return true
		}
	}
	return false
}

// GetStatusValue returns the value for a specific status code, or nil if not found
func (e *DeviceStateChangeEvent) GetStatusValue(code string) interface{} {
	for _, status := range e.Status {
		if status.Code == code {
			return status.Value
		}
	}
	return nil
}

// GetBooleanStatusValue returns the boolean value for a specific status code
// Returns false if the code is not found or cannot be converted to boolean
func (e *DeviceStateChangeEvent) GetBooleanStatusValue(code string) bool {
	value := e.GetStatusValue(code)
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	case float64:
		return v != 0
	case int:
		return v != 0
	default:
		return false
	}
}

// DeviceManagementEvent represents a device management event (protocol 20)
type DeviceManagementEvent struct {
	DeviceID   string                 `json:"devId"`
	ProductKey string                 `json:"productKey"`
	BizCode    string                 `json:"bizCode"`
	BizData    map[string]interface{} `json:"bizData"`
}

// GetDeviceID returns the device ID for the management event
func (e *DeviceManagementEvent) GetDeviceID() string {
	return e.DeviceID
}

// GetEventType returns the event type
func (e *DeviceManagementEvent) GetEventType() string {
	return fmt.Sprintf("device_management_%s", e.BizCode)
}

// DeviceOnlineEvent represents a device coming online
type DeviceOnlineEvent struct {
	DeviceID   string `json:"devId"`
	ProductKey string `json:"productKey"`
	Time       int64  `json:"time"`
}

// GetDeviceID returns the device ID
func (e *DeviceOnlineEvent) GetDeviceID() string {
	return e.DeviceID
}

// GetEventType returns the event type
func (e *DeviceOnlineEvent) GetEventType() string {
	return "device_online"
}

// DeviceOfflineEvent represents a device going offline
type DeviceOfflineEvent struct {
	DeviceID   string `json:"devId"`
	ProductKey string `json:"productKey"`
	Time       int64  `json:"time"`
}

// GetDeviceID returns the device ID
func (e *DeviceOfflineEvent) GetDeviceID() string {
	return e.DeviceID
}

// GetEventType returns the event type
func (e *DeviceOfflineEvent) GetEventType() string {
	return "device_offline"
}

// DeviceNameUpdateEvent represents a device name change
type DeviceNameUpdateEvent struct {
	DeviceID   string `json:"devId"`
	ProductKey string `json:"productKey"`
	NewName    string `json:"name"`
}

// GetDeviceID returns the device ID
func (e *DeviceNameUpdateEvent) GetDeviceID() string {
	return e.DeviceID
}

// GetEventType returns the event type
func (e *DeviceNameUpdateEvent) GetEventType() string {
	return "device_name_update"
}

// DeviceDeleteEvent represents a device being deleted
type DeviceDeleteEvent struct {
	DeviceID   string `json:"devId"`
	ProductKey string `json:"productKey"`
	UID        string `json:"uid"`
}

// GetDeviceID returns the device ID
func (e *DeviceDeleteEvent) GetDeviceID() string {
	return e.DeviceID
}

// GetEventType returns the event type
func (e *DeviceDeleteEvent) GetEventType() string {
	return "device_delete"
}

// ParseEvent parses a raw MQTT message into the appropriate event type
func ParseEvent(rawMessage map[string]interface{}) (Event, error) {
	// Extract protocol number
	protocolFloat, ok := rawMessage["protocol"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid protocol field")
	}
	protocol := int(protocolFloat)

	// Preserve the existing public validation errors before converting to the
	// schema-generated transport and protocol data models.
	_, ok = rawMessage["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid data field")
	}
	message, err := convertWireValue[wire.RawSharingMessage](rawMessage)
	if err != nil {
		return nil, fmt.Errorf("invalid MQTT wire message: %w", err)
	}
	message.Protocol = wire.RawSharingMessageProtocol(protocol)
	return parseRawSharingMessage(message)
}

// ParseDeviceStateChangeEvent parses a device state change event
func ParseDeviceStateChangeEvent(data map[string]interface{}) (*DeviceStateChangeEvent, error) {
	wireData, err := convertWireValue[wire.RawDeviceReportData](data)
	if err != nil {
		return nil, fmt.Errorf("invalid device report payload: %w", err)
	}
	return parseRawDeviceReport(wireData)
}

// ParseDeviceManagementEvent parses a device management event
func ParseDeviceManagementEvent(data map[string]interface{}) (Event, error) {
	wireData, err := convertWireValue[wire.RawDeviceManagementData](data)
	if err != nil {
		return nil, fmt.Errorf("invalid management event payload: %w", err)
	}
	return parseRawDeviceManagementEvent(wireData)
}

func parseRawSharingMessage(message wire.RawSharingMessage) (Event, error) {
	switch message.Protocol {
	case wire.RawSharingMessageProtocol(ProtocolDeviceReport):
		data, err := convertWireValue[wire.RawDeviceReportData](message.Data)
		if err != nil {
			return nil, fmt.Errorf("invalid device report payload: %w", err)
		}
		return parseRawDeviceReport(data)
	case wire.RawSharingMessageProtocol(ProtocolOther):
		data, err := convertWireValue[wire.RawDeviceManagementData](message.Data)
		if err != nil {
			return nil, fmt.Errorf("invalid management event payload: %w", err)
		}
		return parseRawDeviceManagementEvent(data)
	default:
		return nil, fmt.Errorf("unsupported protocol: %d", message.Protocol)
	}
}

func parseRawDeviceReport(data wire.RawDeviceReportData) (*DeviceStateChangeEvent, error) {
	event := &DeviceStateChangeEvent{
		DataID:     dereference(data.DataId),
		DeviceID:   dereference(data.DevId),
		ProductKey: dereference(data.ProductKey),
	}
	if data.Status != nil {
		event.Status = make([]DeviceStatusChange, len(*data.Status))
		for i, status := range *data.Status {
			event.Status[i] = DeviceStatusChange{
				Code:      dereference(status.Code),
				Value:     status.Value,
				Timestamp: dereference(status.T),
			}
		}
	}
	if event.DeviceID == "" {
		return nil, fmt.Errorf("missing device ID in state change event")
	}
	return event, nil
}

func parseRawDeviceManagementEvent(data wire.RawDeviceManagementData) (Event, error) {
	productKey := dereference(data.ProductKey)
	bizCode := dereference(data.BizCode)
	if bizCode == "" {
		return nil, fmt.Errorf("missing bizCode in management event")
	}
	devID := dereference(data.DevId)
	if devID == "" && data.BizData != nil {
		devID = dereference(data.BizData.DevId)
	}
	if devID == "" {
		return nil, fmt.Errorf("missing devId in management event")
	}

	var bizData map[string]interface{}
	if data.BizData != nil {
		converted, err := convertWireValue[map[string]interface{}](*data.BizData)
		if err != nil {
			return nil, fmt.Errorf("invalid management business data: %w", err)
		}
		bizData = converted
	} else {
		bizData = make(map[string]interface{})
	}
	var eventTime int64
	if data.BizData != nil {
		eventTime = dereference(data.BizData.Time)
	}

	switch bizCode {
	case BizcodeOnline:
		return &DeviceOnlineEvent{DeviceID: devID, ProductKey: productKey, Time: eventTime}, nil
	case BizcodeOffline:
		return &DeviceOfflineEvent{DeviceID: devID, ProductKey: productKey, Time: eventTime}, nil
	case BizcodeNameUpdate:
		var name string
		if data.BizData != nil {
			name = dereference(data.BizData.Name)
		}
		return &DeviceNameUpdateEvent{DeviceID: devID, ProductKey: productKey, NewName: name}, nil
	case BizcodeDelete:
		var uid string
		if data.BizData != nil {
			uid = dereference(data.BizData.Uid)
		}
		return &DeviceDeleteEvent{DeviceID: devID, ProductKey: productKey, UID: uid}, nil
	default:
		return &DeviceManagementEvent{DeviceID: devID, ProductKey: productKey, BizCode: bizCode, BizData: bizData}, nil
	}
}

// EventListener represents a callback function for processed events
type EventListener func(event Event)

// ProcessMQTTMessage processes a raw MQTT message and calls the appropriate listeners
func ProcessMQTTMessage(rawMessage string, listeners []EventListener) error {
	var message wire.RawSharingMessage
	if err := json.Unmarshal([]byte(rawMessage), &message); err != nil {
		return fmt.Errorf("failed to parse MQTT message JSON: %w", err)
	}

	event, err := parseRawSharingMessage(message)
	if err != nil {
		return fmt.Errorf("failed to parse event: %w", err)
	}

	// Call all listeners
	for _, listener := range listeners {
		listener(event)
	}

	return nil
}

// TypedEventListener provides type-safe event handling
type TypedEventListener struct {
	OnStateChange func(*DeviceStateChangeEvent)
	OnOnline      func(*DeviceOnlineEvent)
	OnOffline     func(*DeviceOfflineEvent)
	OnNameUpdate  func(*DeviceNameUpdateEvent)
	OnDelete      func(*DeviceDeleteEvent)
	OnManagement  func(*DeviceManagementEvent)
}

// HandleEvent handles an event with type-specific callbacks
func (l *TypedEventListener) HandleEvent(event Event) {
	switch e := event.(type) {
	case *DeviceStateChangeEvent:
		if l.OnStateChange != nil {
			l.OnStateChange(e)
		}
	case *DeviceOnlineEvent:
		if l.OnOnline != nil {
			l.OnOnline(e)
		}
	case *DeviceOfflineEvent:
		if l.OnOffline != nil {
			l.OnOffline(e)
		}
	case *DeviceNameUpdateEvent:
		if l.OnNameUpdate != nil {
			l.OnNameUpdate(e)
		}
	case *DeviceDeleteEvent:
		if l.OnDelete != nil {
			l.OnDelete(e)
		}
	case *DeviceManagementEvent:
		if l.OnManagement != nil {
			l.OnManagement(e)
		}
	}
}

// CreateTypedEventListener creates an EventListener from TypedEventListener
func CreateTypedEventListener(typedListener *TypedEventListener) EventListener {
	return func(event Event) {
		typedListener.HandleEvent(event)
	}
}
