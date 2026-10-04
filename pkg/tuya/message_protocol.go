package tuya

import (
	"encoding/json"
	"fmt"

	wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)

// This describes the overall message protocol that is sent over via MQTT to the client.
//https://developer.tuya.com/en/docs/iot/message-type?id=Kavqerli65a1u#title-6-Report%20device%20data%20(protocol%201000)

// Protocol constants.
const (
	// ProtocolDeviceReport represents device state change reports (protocol 4).
	ProtocolDeviceReport = int(wire.N4)
	// ProtocolOther represents device management events (protocol 20).
	ProtocolOther = int(wire.N20)
)

// Business code constants for device management events.
const (
	BizcodeOnline     = string(wire.TuyaBizCodeOnline)
	BizcodeOffline    = string(wire.TuyaBizCodeOffline)
	BizcodeNameUpdate = string(wire.TuyaBizCodeNameUpdate)
	BizcodeDelete     = string(wire.TuyaBizCodeDelete)
	BizcodeBindUser   = string(wire.TuyaBizCodeBindUser)
	// BizcodeDpNameUpdate represents a device point name update, such as led_dimmer_1 to led_dimmer_2.
	BizcodeDpNameUpdate = string(wire.TuyaBizCodeDpNameUpdate)
)

// Event represents the base interface for all event types.
type Event interface {
	GetDeviceID() string
	GetEventType() string
}

// GetDeviceID returns the device ID for the state change event.
func (e *DeviceStateChangeEvent) GetDeviceID() string {
	return e.DeviceID
}

// GetEventType returns the event type.
func (e *DeviceStateChangeEvent) GetEventType() string {
	return SDKEventTypeDeviceStateChange
}

// HasStatusCode checks if the event contains a specific status code change.
func (e *DeviceStateChangeEvent) HasStatusCode(code string) bool {
	for _, status := range e.Status {
		if status.Code == code {
			return true
		}
	}

	return false
}

// GetStatusValue returns the value for a specific status code, or nil if not found.
func (e *DeviceStateChangeEvent) GetStatusValue(code string) any {
	for _, status := range e.Status {
		if status.Code == code {
			return status.Value
		}
	}

	return nil
}

// GetBooleanStatusValue returns the boolean value for a specific status code
// Returns false if the code is not found or cannot be converted to boolean.
func (e *DeviceStateChangeEvent) GetBooleanStatusValue(code string) bool {
	value := e.GetStatusValue(code)
	if value == nil {
		return false
	}

	switch typedValue := value.(type) {
	case bool:
		return typedValue
	case string:
		return typedValue == "true" || typedValue == "1"
	case float64:
		return typedValue != 0
	case int:
		return typedValue != 0
	default:
		return false
	}
}

// GetDeviceID returns the device ID for the management event.
func (e *DeviceManagementEvent) GetDeviceID() string {
	return e.DeviceID
}

// GetEventType returns the event type.
func (e *DeviceManagementEvent) GetEventType() string {
	return SDKEventTypeDeviceManagementPrefix + e.BizCode
}

// GetDeviceID returns the device ID.
func (e *DeviceOnlineEvent) GetDeviceID() string {
	return e.DeviceID
}

// GetEventType returns the event type.
func (e *DeviceOnlineEvent) GetEventType() string {
	return SDKEventTypeDeviceOnline
}

// GetDeviceID returns the device ID.
func (e *DeviceOfflineEvent) GetDeviceID() string {
	return e.DeviceID
}

// GetEventType returns the event type.
func (e *DeviceOfflineEvent) GetEventType() string {
	return SDKEventTypeDeviceOffline
}

// GetDeviceID returns the device ID.
func (e *DeviceNameUpdateEvent) GetDeviceID() string {
	return e.DeviceID
}

// GetEventType returns the event type.
func (e *DeviceNameUpdateEvent) GetEventType() string {
	return SDKEventTypeDeviceNameUpdate
}

// GetDeviceID returns the device ID.
func (e *DeviceDeleteEvent) GetDeviceID() string {
	return e.DeviceID
}

// GetEventType returns the event type.
func (e *DeviceDeleteEvent) GetEventType() string {
	return SDKEventTypeDeviceDelete
}

// ParseEvent parses a raw MQTT message into the appropriate event type.
func ParseEvent(rawMessage map[string]any) (Event, error) { //nolint:ireturn // Protocol variants share the public Event interface.
	// Extract protocol number
	protocolFloat, protocolOK := rawMessage[wire.PropertyRawSharingMessageProtocol].(float64)
	if !protocolOK {
		return nil, errProtocolFieldInvalid
	}

	protocol := int(protocolFloat)

	// Preserve the existing public validation errors before converting to the
	// schema-generated transport and protocol data models.
	_, protocolOK = rawMessage[wire.PropertyRawSharingMessageData].(map[string]any)
	if !protocolOK {
		return nil, errDataFieldInvalid
	}

	message, err := convertWireValue[wire.RawSharingMessage](rawMessage)
	if err != nil {
		return nil, fmt.Errorf("invalid MQTT wire message: %w", err)
	}

	message.Protocol = wire.RawSharingMessageProtocol(protocol)

	return parseRawSharingMessage(message)
}

// ParseDeviceStateChangeEvent parses a device state change event.
func ParseDeviceStateChangeEvent(data map[string]any) (*DeviceStateChangeEvent, error) {
	wireData, err := convertWireValue[wire.RawDeviceReportData](data)
	if err != nil {
		return nil, fmt.Errorf("invalid device report payload: %w", err)
	}

	return parseRawDeviceReport(wireData)
}

// ParseDeviceManagementEvent parses a device management event.
func ParseDeviceManagementEvent(data map[string]any) (Event, error) { //nolint:ireturn // Management codes produce several public Event implementations.
	wireData, err := convertWireValue[wire.RawDeviceManagementData](data)
	if err != nil {
		return nil, fmt.Errorf("invalid management event payload: %w", err)
	}

	return parseRawDeviceManagementEvent(wireData)
}

func parseRawSharingMessage(message wire.RawSharingMessage) (Event, error) { //nolint:ireturn // One return type covers protocol variants.
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
		return nil, fmt.Errorf("%w: %d", errUnsupportedProtocol, message.Protocol)
	}
}

func parseRawDeviceReport(data wire.RawDeviceReportData) (*DeviceStateChangeEvent, error) {
	event := &DeviceStateChangeEvent{
		DataID:     dereference(data.DataId),
		DeviceID:   dereference(data.DevId),
		ProductKey: dereference(data.ProductKey),
		Status:     nil,
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
		return nil, errStateChangeDeviceIDMissing
	}

	return event, nil
}

func parseRawDeviceManagementEvent(data wire.RawDeviceManagementData) (Event, error) { //nolint:ireturn // Management codes map to different event types.
	productKey := dereference(data.ProductKey)

	bizCode := dereference(data.BizCode)
	if bizCode == "" {
		return nil, errManagementBizCodeMissing
	}

	devID := dereference(data.DevId)
	if devID == "" && data.BizData != nil {
		devID = dereference(data.BizData.DevId)
	}

	if devID == "" {
		return nil, errManagementDeviceIDMissing
	}

	var (
		bizData    map[string]any
		eventTime  int64
		deviceName string
		deviceUser string
	)

	if data.BizData != nil {
		converted, err := convertWireValue[map[string]any](*data.BizData)
		if err != nil {
			return nil, fmt.Errorf("invalid management business data: %w", err)
		}

		bizData = converted
		eventTime = dereference(data.BizData.Time)
		deviceName = dereference(data.BizData.Name)
		deviceUser = dereference(data.BizData.Uid)
	} else {
		bizData = make(map[string]any)
	}

	return managementEvent(bizCode, devID, productKey, eventTime, deviceName, deviceUser, bizData), nil
}

//nolint:ireturn // Management codes map to different event types.
func managementEvent(
	bizCode, deviceID, productKey string,
	eventTime int64,
	name, userID string,
	bizData map[string]any,
) Event {
	switch bizCode {
	case BizcodeOnline:
		return &DeviceOnlineEvent{DeviceID: deviceID, ProductKey: productKey, Time: eventTime}
	case BizcodeOffline:
		return &DeviceOfflineEvent{DeviceID: deviceID, ProductKey: productKey, Time: eventTime}
	case BizcodeNameUpdate:
		return &DeviceNameUpdateEvent{DeviceID: deviceID, ProductKey: productKey, NewName: name}
	case BizcodeDelete:
		return &DeviceDeleteEvent{DeviceID: deviceID, ProductKey: productKey, UID: userID}
	default:
		return &DeviceManagementEvent{DeviceID: deviceID, ProductKey: productKey, BizCode: bizCode, BizData: bizData}
	}
}

// EventListener represents a callback function for processed events.
type EventListener func(event Event)

// ProcessMQTTMessage processes a raw MQTT message and calls the appropriate listeners.
func ProcessMQTTMessage(rawMessage string, listeners []EventListener) error {
	var message wire.RawSharingMessage

	err := json.Unmarshal([]byte(rawMessage), &message)
	if err != nil {
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

// TypedEventListener provides type-safe event handling.
type TypedEventListener struct {
	OnStateChange func(*DeviceStateChangeEvent)
	OnOnline      func(*DeviceOnlineEvent)
	OnOffline     func(*DeviceOfflineEvent)
	OnNameUpdate  func(*DeviceNameUpdateEvent)
	OnDelete      func(*DeviceDeleteEvent)
	OnManagement  func(*DeviceManagementEvent)
}

// HandleEvent handles an event with type-specific callbacks.
func (l *TypedEventListener) HandleEvent(event Event) {
	switch typedEvent := event.(type) {
	case *DeviceStateChangeEvent:
		callTypedEventListener(l.OnStateChange, typedEvent)
	case *DeviceOnlineEvent:
		callTypedEventListener(l.OnOnline, typedEvent)
	case *DeviceOfflineEvent:
		callTypedEventListener(l.OnOffline, typedEvent)
	case *DeviceNameUpdateEvent:
		callTypedEventListener(l.OnNameUpdate, typedEvent)
	case *DeviceDeleteEvent:
		callTypedEventListener(l.OnDelete, typedEvent)
	case *DeviceManagementEvent:
		callTypedEventListener(l.OnManagement, typedEvent)
	}
}

func callTypedEventListener[T any](listener func(T), event T) {
	if listener != nil {
		listener(event)
	}
}

// CreateTypedEventListener creates an EventListener from TypedEventListener.
func CreateTypedEventListener(typedListener *TypedEventListener) EventListener {
	return func(event Event) {
		typedListener.HandleEvent(event)
	}
}
