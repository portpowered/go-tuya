package tuya

import (
	"testing"
)

// Repeated values stay test-local so synthetic fixtures remain independent of production constants.
const (
	messageFixture002dj00118fe34d9                               = "002dj00118fe34d9****"
	messageFixture6c95a93fd9xxx                                  = "6c95a93fd9xxx***"
	messageFixtureBizCode                                        = "bizCode"
	messageFixtureBizData                                        = "bizData"
	messageFixtureCode                                           = "code"
	messageFixtureData                                           = "data"
	messageFixtureDevID                                          = "devId"
	messageFixtureFalse                                          = "false"
	messageFixtureName                                           = "name"
	messageFixtureProductKey                                     = "productKey"
	messageFixtureProtocol                                       = "protocol"
	messageFixtureRetained                                       = "retained"
	messageFixtureStatus                                         = "status"
	messageFixtureSwitch                                         = "switch"
	messageFixtureSyntheticDeviceID                              = "synthetic-device-id"
	messageFixtureSyntheticProductKey                            = "synthetic-product-key"
	messageFixtureTestDevice                                     = "test-device"
	messageFixtureTheProductKeyDefinedOnTheTuyaDeveloperPlatform = "The product key defined on the Tuya Developer Platform"
	messageFixtureTime                                           = "time"
	messageFixtureValue                                          = "value"
)

//nolint:cyclop,funlen // The test validates every typed field in one synthetic device-state event payload.
func TestParseEvent_DeviceStateChange(t *testing.T) {
	t.Parallel()

	// Test data based on the provided example
	rawMessage := map[string]any{
		messageFixtureProtocol: float64(4), // Changed from 1000 to 4 (ProtocolDeviceReport)
		"t":                    float64(1628229842692),
		messageFixtureData: map[string]any{
			"dataId":                 "AAXI3c1i6xxx***",
			messageFixtureDevID:      messageFixture6c95a93fd9xxx,
			messageFixtureProductKey: "awgmk9pixxx***",
			messageFixtureStatus: []any{
				map[string]any{
					messageFixtureCode:  "switch_1",
					messageFixtureValue: false,
					"t":                 float64(1628229842692),
					"1":                 messageFixtureFalse,
				},
			},
		},
	}

	event, err := ParseEvent(rawMessage)
	if err != nil {
		t.Fatalf("Failed to parse event: %v", err)
	}

	stateEvent, ok := event.(*DeviceStateChangeEvent)
	if !ok {
		t.Fatalf("Expected DeviceStateChangeEvent, got %T", event)
	}

	// Validate basic fields
	if stateEvent.DataID != "AAXI3c1i6xxx***" {
		t.Errorf("Expected DataID=AAXI3c1i6xxx***, got %s", stateEvent.DataID)
	}

	if stateEvent.DeviceID != messageFixture6c95a93fd9xxx {
		t.Errorf("Expected DeviceID=6c95a93fd9xxx***, got %s", stateEvent.DeviceID)
	}

	if stateEvent.ProductKey != "awgmk9pixxx***" {
		t.Errorf("Expected ProductKey=awgmk9pixxx***, got %s", stateEvent.ProductKey)
	}

	// Validate status array
	if len(stateEvent.Status) != 1 {
		t.Fatalf("Expected 1 status item, got %d", len(stateEvent.Status))
	}

	status := stateEvent.Status[0]
	if status.Code != "switch_1" {
		t.Errorf("Expected Code=switch_1, got %s", status.Code)
	}

	if status.Value != false {
		t.Errorf("Expected Value=false, got %v", status.Value)
	}

	if status.Timestamp != 1628229842692 {
		t.Errorf("Expected Timestamp=1628229842692, got %d", status.Timestamp)
	}

	// Test helper methods
	if !stateEvent.HasStatusCode("switch_1") {
		t.Error("Expected HasStatusCode(switch_1) to return true")
	}

	if stateEvent.HasStatusCode("nonexistent") {
		t.Error("Expected HasStatusCode(nonexistent) to return false")
	}

	if stateEvent.GetStatusValue("switch_1") != false {
		t.Error("Expected GetStatusValue(switch_1) to return false")
	}

	if stateEvent.GetStatusValue("nonexistent") != nil {
		t.Error("Expected GetStatusValue(nonexistent) to return nil")
	}

	if stateEvent.GetBooleanStatusValue("switch_1") != false {
		t.Error("Expected GetBooleanStatusValue(switch_1) to return false")
	}

	// Test Event interface
	if stateEvent.GetDeviceID() != messageFixture6c95a93fd9xxx {
		t.Errorf("Expected GetDeviceID()=6c95a93fd9xxx***, got %s", stateEvent.GetDeviceID())
	}

	if stateEvent.GetEventType() != "device_state_change" {
		t.Errorf("Expected GetEventType()=device_state_change, got %s", stateEvent.GetEventType())
	}
}

func TestParseEvent_DeviceStateChange_LEDSwitch(t *testing.T) {
	t.Parallel()

	// Test with led_switch_1 as mentioned in requirements
	rawMessage := map[string]any{
		messageFixtureProtocol: float64(4), // Changed from 1000 to 4 (ProtocolDeviceReport)
		messageFixtureData: map[string]any{
			messageFixtureDevID: "test-device-123",
			messageFixtureStatus: []any{
				map[string]any{
					messageFixtureCode:  "led_switch_1",
					messageFixtureValue: true,
					"t":                 float64(1628229842692),
				},
			},
		},
	}

	event, err := ParseEvent(rawMessage)
	if err != nil {
		t.Fatalf("Failed to parse event: %v", err)
	}

	stateEvent := mustAssert[*DeviceStateChangeEvent](t, event)

	// Test led_switch_1 specifically
	if !stateEvent.HasStatusCode("led_switch_1") {
		t.Error("Expected HasStatusCode(led_switch_1) to return true")
	}

	if stateEvent.GetBooleanStatusValue("led_switch_1") != true {
		t.Error("Expected GetBooleanStatusValue(led_switch_1) to return true")
	}
}

func TestParseEvent_DeviceOnline(t *testing.T) {
	t.Parallel()

	rawMessage := map[string]any{
		messageFixtureProtocol: float64(20),
		messageFixtureData: map[string]any{
			messageFixtureDevID:      messageFixture002dj00118fe34d9,
			messageFixtureProductKey: messageFixtureTheProductKeyDefinedOnTheTuyaDeveloperPlatform,
			messageFixtureBizCode:    "online",
			messageFixtureBizData: map[string]any{
				messageFixtureTime: float64(146052438362),
			},
		},
	}

	event, err := ParseEvent(rawMessage)
	if err != nil {
		t.Fatalf("Failed to parse event: %v", err)
	}

	onlineEvent, ok := event.(*DeviceOnlineEvent)
	if !ok {
		t.Fatalf("Expected DeviceOnlineEvent, got %T", event)
	}

	if onlineEvent.DeviceID != messageFixture002dj00118fe34d9 {
		t.Errorf("Expected DeviceID=002dj00118fe34d9****, got %s", onlineEvent.DeviceID)
	}

	if onlineEvent.ProductKey != messageFixtureTheProductKeyDefinedOnTheTuyaDeveloperPlatform {
		t.Errorf("Expected ProductKey=The product key defined on the Tuya Developer Platform, got %s", onlineEvent.ProductKey)
	}

	if onlineEvent.Time != 146052438362 {
		t.Errorf("Expected Time=146052438362, got %d", onlineEvent.Time)
	}

	// Test Event interface
	if onlineEvent.GetEventType() != "device_online" {
		t.Errorf("Expected GetEventType()=device_online, got %s", onlineEvent.GetEventType())
	}
}

func TestParseEvent_DeviceOffline(t *testing.T) {
	t.Parallel()

	rawMessage := map[string]any{
		messageFixtureProtocol: float64(20),
		messageFixtureData: map[string]any{
			messageFixtureDevID:      messageFixture002dj00118fe34d9,
			messageFixtureProductKey: messageFixtureTheProductKeyDefinedOnTheTuyaDeveloperPlatform,
			messageFixtureBizCode:    "offline",
			messageFixtureBizData: map[string]any{
				messageFixtureTime: float64(146052438362),
			},
		},
	}

	event, err := ParseEvent(rawMessage)
	if err != nil {
		t.Fatalf("Failed to parse event: %v", err)
	}

	offlineEvent, ok := event.(*DeviceOfflineEvent)
	if !ok {
		t.Fatalf("Expected DeviceOfflineEvent, got %T", event)
	}

	if offlineEvent.GetEventType() != "device_offline" {
		t.Errorf("Expected GetEventType()=device_offline, got %s", offlineEvent.GetEventType())
	}
}

func TestParseEvent_DeviceNameUpdate(t *testing.T) {
	t.Parallel()

	rawMessage := map[string]any{
		messageFixtureProtocol: float64(20),
		messageFixtureData: map[string]any{
			messageFixtureDevID:      messageFixture002dj00118fe34d9,
			messageFixtureProductKey: messageFixtureTheProductKeyDefinedOnTheTuyaDeveloperPlatform,
			messageFixtureBizCode:    "nameUpdate",
			messageFixtureBizData: map[string]any{
				messageFixtureDevID: messageFixture002dj00118fe34d9,
				messageFixtureName:  "new name",
			},
		},
	}

	event, err := ParseEvent(rawMessage)
	if err != nil {
		t.Fatalf("Failed to parse event: %v", err)
	}

	nameEvent, ok := event.(*DeviceNameUpdateEvent)
	if !ok {
		t.Fatalf("Expected DeviceNameUpdateEvent, got %T", event)
	}

	if nameEvent.NewName != "new name" {
		t.Errorf("Expected NewName=new name, got %s", nameEvent.NewName)
	}

	if nameEvent.GetEventType() != "device_name_update" {
		t.Errorf("Expected GetEventType()=device_name_update, got %s", nameEvent.GetEventType())
	}
}

func TestParseEvent_DeviceDelete(t *testing.T) {
	t.Parallel()

	rawMessage := map[string]any{
		messageFixtureProtocol: float64(20),
		messageFixtureData: map[string]any{
			messageFixtureDevID:      messageFixture002dj00118fe34d9,
			messageFixtureProductKey: messageFixtureTheProductKeyDefinedOnTheTuyaDeveloperPlatform,
			messageFixtureBizCode:    "delete",
			messageFixtureBizData: map[string]any{
				messageFixtureDevID: messageFixture002dj00118fe34d9,
				"uid":               "ay1529485403390S****",
			},
		},
	}

	event, err := ParseEvent(rawMessage)
	if err != nil {
		t.Fatalf("Failed to parse event: %v", err)
	}

	deleteEvent, ok := event.(*DeviceDeleteEvent)
	if !ok {
		t.Fatalf("Expected DeviceDeleteEvent, got %T", event)
	}

	if deleteEvent.UID != "ay1529485403390S****" {
		t.Errorf("Expected UID=ay1529485403390S****, got %s", deleteEvent.UID)
	}

	if deleteEvent.GetEventType() != "device_delete" {
		t.Errorf("Expected GetEventType()=device_delete, got %s", deleteEvent.GetEventType())
	}
}

func TestProcessMQTTMessage(t *testing.T) {
	t.Parallel()

	// Test with JSON string input
	messageJSON := `{
		"protocol": 4,
		"t": 1628229842692,
		"data": {
			"devId": "test-device",
			"status": [{
				"code": "led_switch_1",
				"value": true
			}]
		}
	}`

	var receivedEvents []Event

	listeners := []EventListener{
		func(event Event) {
			receivedEvents = append(receivedEvents, event)
		},
	}

	err := ProcessMQTTMessage(messageJSON, listeners)
	if err != nil {
		t.Fatalf("Failed to process MQTT message: %v", err)
	}

	if len(receivedEvents) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(receivedEvents))
	}

	stateEvent, ok := receivedEvents[0].(*DeviceStateChangeEvent)
	if !ok {
		t.Fatalf("Expected DeviceStateChangeEvent, got %T", receivedEvents[0])
	}

	if stateEvent.DeviceID != messageFixtureTestDevice {
		t.Errorf("Expected DeviceID=test-device, got %s", stateEvent.DeviceID)
	}
}

//nolint:funlen // The test keeps listener selection and callback payload assertions together for each typed event.
func TestTypedEventListener(t *testing.T) {
	t.Parallel()

	var (
		receivedStateEvents   []*DeviceStateChangeEvent
		receivedOnlineEvents  []*DeviceOnlineEvent
		receivedOfflineEvents []*DeviceOfflineEvent
	)

	typedListener := &TypedEventListener{
		OnStateChange: func(event *DeviceStateChangeEvent) {
			receivedStateEvents = append(receivedStateEvents, event)
		},
		OnOnline: func(event *DeviceOnlineEvent) {
			receivedOnlineEvents = append(receivedOnlineEvents, event)
		},
		OnOffline: func(event *DeviceOfflineEvent) {
			receivedOfflineEvents = append(receivedOfflineEvents, event)
		},
	}

	listener := CreateTypedEventListener(typedListener)

	// Test state change event
	stateEvent := &DeviceStateChangeEvent{
		DeviceID: "test-device-1",
		Status: []DeviceStatusChange{
			{Code: "led_switch_1", Value: true},
		},
	}
	listener(stateEvent)

	// Test online event
	onlineEvent := &DeviceOnlineEvent{
		DeviceID: "test-device-2",
		Time:     123456789,
	}
	listener(onlineEvent)

	// Test offline event
	offlineEvent := &DeviceOfflineEvent{
		DeviceID: "test-device-3",
		Time:     123456790,
	}
	listener(offlineEvent)

	// Verify events were received
	if len(receivedStateEvents) != 1 {
		t.Errorf("Expected 1 state change event, got %d", len(receivedStateEvents))
	}

	if len(receivedOnlineEvents) != 1 {
		t.Errorf("Expected 1 online event, got %d", len(receivedOnlineEvents))
	}

	if len(receivedOfflineEvents) != 1 {
		t.Errorf("Expected 1 offline event, got %d", len(receivedOfflineEvents))
	}

	if receivedStateEvents[0].DeviceID != "test-device-1" {
		t.Errorf("Expected state event DeviceID=test-device-1, got %s", receivedStateEvents[0].DeviceID)
	}

	if receivedOnlineEvents[0].DeviceID != "test-device-2" {
		t.Errorf("Expected online event DeviceID=test-device-2, got %s", receivedOnlineEvents[0].DeviceID)
	}

	if receivedOfflineEvents[0].DeviceID != "test-device-3" {
		t.Errorf("Expected offline event DeviceID=test-device-3, got %s", receivedOfflineEvents[0].DeviceID)
	}
}

func TestGetBooleanStatusValue_VariousTypes(t *testing.T) {
	t.Parallel()

	event := &DeviceStateChangeEvent{
		Status: []DeviceStatusChange{
			{Code: "bool_true", Value: true},
			{Code: "bool_false", Value: false},
			{Code: "string_true", Value: "true"},
			{Code: "string_false", Value: messageFixtureFalse},
			{Code: "string_1", Value: "1"},
			{Code: "string_0", Value: "0"},
			{Code: "float_nonzero", Value: float64(5.5)},
			{Code: "float_zero", Value: float64(0)},
			{Code: "int_nonzero", Value: 42},
			{Code: "int_zero", Value: 0},
			{Code: "unknown_type", Value: []string{"array"}},
		},
	}

	tests := []struct {
		code     string
		expected bool
	}{
		{"bool_true", true},
		{"bool_false", false},
		{"string_true", true},
		{"string_false", false},
		{"string_1", true},
		{"string_0", false},
		{"float_nonzero", true},
		{"float_zero", false},
		{"int_nonzero", true},
		{"int_zero", false},
		{"unknown_type", false},
		{"nonexistent", false},
	}

	for _, test := range tests {
		result := event.GetBooleanStatusValue(test.code)
		if result != test.expected {
			t.Errorf("GetBooleanStatusValue(%s): expected %t, got %t", test.code, test.expected, result)
		}
	}
}

func TestParseEvent_InvalidProtocol(t *testing.T) {
	t.Parallel()

	rawMessage := map[string]any{
		messageFixtureProtocol: float64(999), // Invalid protocol
		messageFixtureData: map[string]any{
			messageFixtureDevID: messageFixtureTestDevice,
		},
	}

	_, err := ParseEvent(rawMessage)
	if err == nil {
		t.Error("Expected error for invalid protocol, got nil")
	}

	if err.Error() != "unsupported protocol: 999" {
		t.Errorf("Expected 'unsupported protocol: 999', got %s", err.Error())
	}
}

func TestParseEvent_MissingFields(t *testing.T) {
	t.Parallel()

	// Test missing protocol
	rawMessage := map[string]any{
		messageFixtureData: map[string]any{
			messageFixtureDevID: messageFixtureTestDevice,
		},
	}

	_, err := ParseEvent(rawMessage)
	if err == nil {
		t.Error("Expected error for missing protocol, got nil")
	}

	// Test missing data
	rawMessage = map[string]any{
		messageFixtureProtocol: float64(1000),
	}

	_, err = ParseEvent(rawMessage)
	if err == nil {
		t.Error("Expected error for missing data, got nil")
	}

	// Test missing device ID in state change
	rawMessage = map[string]any{
		messageFixtureProtocol: float64(1000),
		messageFixtureData: map[string]any{
			messageFixtureStatus: []any{},
		},
	}

	_, err = ParseEvent(rawMessage)
	if err == nil {
		t.Error("Expected error for missing device ID in state change, got nil")
	}
}

func TestParseDeviceStateChangeEventPublicMapAdapter(t *testing.T) {
	t.Parallel()

	event, err := ParseDeviceStateChangeEvent(map[string]any{
		"dataId":                 "synthetic-data-id",
		messageFixtureDevID:      messageFixtureSyntheticDeviceID,
		messageFixtureProductKey: messageFixtureSyntheticProductKey,
		messageFixtureStatus: []any{
			map[string]any{messageFixtureCode: messageFixtureSwitch, messageFixtureValue: true, "t": float64(1234)},
		},
	})
	if err != nil {
		t.Fatalf("ParseDeviceStateChangeEvent() error = %v", err)
	}

	if event.DeviceID != messageFixtureSyntheticDeviceID || event.DataID != "synthetic-data-id" {
		t.Fatalf("parsed event identity = %+v", event)
	}

	if len(event.Status) != 1 || event.Status[0].Code != messageFixtureSwitch || event.Status[0].Timestamp != 1234 || event.Status[0].Value != true {
		t.Fatalf("parsed event status = %+v", event.Status)
	}

	if _, err := ParseDeviceStateChangeEvent(map[string]any{messageFixtureStatus: "not-an-array"}); err == nil {
		t.Fatal("ParseDeviceStateChangeEvent() accepted a status value with the wrong shape")
	}
}

func TestParseDeviceManagementEventPublicMapAdapter(t *testing.T) {
	t.Parallel()

	event, err := ParseDeviceManagementEvent(map[string]any{
		messageFixtureProductKey: messageFixtureSyntheticProductKey,
		messageFixtureBizCode:    BizcodeOnline,
		messageFixtureBizData: map[string]any{
			messageFixtureDevID: messageFixtureSyntheticDeviceID,
			messageFixtureTime:  float64(5678),
		},
	})
	if err != nil {
		t.Fatalf("ParseDeviceManagementEvent() error = %v", err)
	}

	online, valid := event.(*DeviceOnlineEvent)
	if !valid {
		t.Fatalf("event type = %T, want *DeviceOnlineEvent", event)
	}

	if online.DeviceID != messageFixtureSyntheticDeviceID || online.ProductKey != messageFixtureSyntheticProductKey || online.Time != 5678 {
		t.Fatalf("parsed online event = %+v", online)
	}

	genericEvent, err := ParseDeviceManagementEvent(map[string]any{
		messageFixtureDevID:      messageFixtureSyntheticDeviceID,
		messageFixtureProductKey: messageFixtureSyntheticProductKey,
		messageFixtureBizCode:    "futureEvent",
		messageFixtureBizData: map[string]any{
			"newProviderField": messageFixtureRetained,
		},
	})
	if err != nil {
		t.Fatalf("ParseDeviceManagementEvent(unknown code) error = %v", err)
	}

	management, valid := genericEvent.(*DeviceManagementEvent)
	if !valid || management.BizData["newProviderField"] != messageFixtureRetained {
		t.Fatalf("generic management event = %#v", genericEvent)
	}

	if _, err := ParseDeviceManagementEvent(map[string]any{messageFixtureBizData: "not-an-object"}); err == nil {
		t.Fatal("ParseDeviceManagementEvent() accepted a business payload with the wrong shape")
	}
}

func TestProcessMQTTMessage_InvalidJSON(t *testing.T) {
	t.Parallel()

	invalidJSON := `{"protocol": 1000, "data": {invalid json`

	err := ProcessMQTTMessage(invalidJSON, []EventListener{})
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

// Example usage demonstration.
func ExampleProcessMQTTMessage() {
	// Example MQTT message for LED switch state change
	messageJSON := `{
		"protocol": 1000,
		"data": {
			messageFixtureDevID: "my-smart-switch",
			"status": [{
				"code": "led_switch_1",
				"value": true
			}]
		}
	}`

	// Create typed event listener
	typedListener := &TypedEventListener{
		OnStateChange: func(event *DeviceStateChangeEvent) {
			if event.HasStatusCode("led_switch_1") {
				isOn := event.GetBooleanStatusValue("led_switch_1")

				// Handle LED switch state change
				_ = isOn // Use the state as needed
			}
		},
		OnOnline: func(_ *DeviceOnlineEvent) {

			// Handle device coming online
		},
		OnOffline: func(_ *DeviceOfflineEvent) {

			// Handle device going offline
		},
	}

	listeners := []EventListener{CreateTypedEventListener(typedListener)}

	// Process the message
	err := ProcessMQTTMessage(messageJSON, listeners)
	if err != nil {
		// Handle error
		return
	}

	// Output:
}
