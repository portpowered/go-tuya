package tuya

import (
	"encoding/json"
	"fmt"
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

// RawMQTTMessage represents the raw message structure received from MQTT
type RawMQTTMessage struct {
	Protocol int                    `json:"protocol"`
	Data     map[string]interface{} `json:"data"`
	T        int64                  `json:"t,omitempty"`
}

// Event represents the base interface for all event types
type Event interface {
	GetDeviceID() string
	GetEventType() string
}

// DeviceStateChangeEvent represents a device state change event (protocol 1000)
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

	// Extract data field
	data, ok := rawMessage["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid data field")
	}

	switch protocol {
	case ProtocolDeviceReport:
		return ParseDeviceStateChangeEvent(data)
	case ProtocolOther:
		return ParseDeviceManagementEvent(data)
	default:
		return nil, fmt.Errorf("unsupported protocol: %d", protocol)
	}
}

// ParseDeviceStateChangeEvent parses a device state change event
func ParseDeviceStateChangeEvent(data map[string]interface{}) (*DeviceStateChangeEvent, error) {
	event := &DeviceStateChangeEvent{}

	// Extract basic fields
	if dataID, ok := data["dataId"].(string); ok {
		event.DataID = dataID
	}
	if devID, ok := data["devId"].(string); ok {
		event.DeviceID = devID
	}
	if productKey, ok := data["productKey"].(string); ok {
		event.ProductKey = productKey
	}

	// Parse status array
	if statusArray, ok := data["status"].([]interface{}); ok {
		event.Status = make([]DeviceStatusChange, len(statusArray))
		for i, statusItem := range statusArray {
			if statusMap, ok := statusItem.(map[string]interface{}); ok {
				status := DeviceStatusChange{}
				if code, ok := statusMap["code"].(string); ok {
					status.Code = code
				}
				if value, ok := statusMap["value"]; ok {
					status.Value = value
				}
				if t, ok := statusMap["t"].(float64); ok {
					status.Timestamp = int64(t)
				}
				event.Status[i] = status
			}
		}
	}

	if event.DeviceID == "" {
		return nil, fmt.Errorf("missing device ID in state change event")
	}

	return event, nil
}

// ParseDeviceManagementEvent parses a device management event
func ParseDeviceManagementEvent(data map[string]interface{}) (Event, error) {
	// map[string]interface {} ["protocol": 20,
	// 	"data": map[string]interface {}
	// 			[
	// 				"bizCode": *(*interface {})(0xc0001da258),
	// 				"bizData": *(*interface {})(0xc0001da278),
	// 				"ts": *(*interface {})(0xc0001da298),
	// 			]
	// 		]
	// 	"t": 1753691950565,
	// ]
	//

	// Extract basic fields

	productKey, _ := data["productKey"].(string)
	bizCode, ok := data["bizCode"].(string)
	if !ok {
		return nil, fmt.Errorf("missing bizCode in management event")
	}

	bizData, ok := data["bizData"].(map[string]interface{})
	if !ok {
		bizData = make(map[string]interface{})
	}

	// Get devID from main data object first, then fall back to bizData
	devID, ok := data["devId"].(string)
	if !ok {
		// Fall back to bizData if not in main data
		if devIDFromBiz, ok := bizData["devId"].(string); ok {
			devID = devIDFromBiz
		} else {
			return nil, fmt.Errorf("missing devId in management event")
		}
	}

	// Create specific event types based on bizCode
	switch bizCode {
	case BizcodeOnline:
		event := &DeviceOnlineEvent{
			DeviceID:   devID,
			ProductKey: productKey,
		}
		if time, ok := bizData["time"].(float64); ok {
			event.Time = int64(time)
		}
		return event, nil

	case BizcodeOffline:
		event := &DeviceOfflineEvent{
			DeviceID:   devID,
			ProductKey: productKey,
		}
		if time, ok := bizData["time"].(float64); ok {
			event.Time = int64(time)
		}
		return event, nil

	case BizcodeNameUpdate:
		event := &DeviceNameUpdateEvent{
			DeviceID:   devID,
			ProductKey: productKey,
		}
		if name, ok := bizData["name"].(string); ok {
			event.NewName = name
		}
		return event, nil

	case BizcodeDelete:
		event := &DeviceDeleteEvent{
			DeviceID:   devID,
			ProductKey: productKey,
		}
		if uid, ok := bizData["uid"].(string); ok {
			event.UID = uid
		}
		return event, nil

	default:
		// Return generic management event for unknown bizCodes
		return &DeviceManagementEvent{
			DeviceID:   devID,
			ProductKey: productKey,
			BizCode:    bizCode,
			BizData:    bizData,
		}, nil
	}
}

// EventListener represents a callback function for processed events
type EventListener func(event Event)

// ProcessMQTTMessage processes a raw MQTT message and calls the appropriate listeners
func ProcessMQTTMessage(rawMessage string, listeners []EventListener) error {
	// Parse JSON
	var msgData map[string]interface{}
	if err := json.Unmarshal([]byte(rawMessage), &msgData); err != nil {
		return fmt.Errorf("failed to parse MQTT message JSON: %w", err)
	}

	// Parse into event
	event, err := ParseEvent(msgData)
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
