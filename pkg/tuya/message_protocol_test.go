package tuya

import (
	"testing"
)

func TestParseEvent_DeviceStateChange(t *testing.T) {
	// Test data based on the provided example
	rawMessage := map[string]interface{}{
		"protocol": float64(4), // Changed from 1000 to 4 (ProtocolDeviceReport)
		"t":        float64(1628229842692),
		"data": map[string]interface{}{
			"dataId":     "AAXI3c1i6xxx***",
			"devId":      "6c95a93fd9xxx***",
			"productKey": "awgmk9pixxx***",
			"status": []interface{}{
				map[string]interface{}{
					"code":  "switch_1",
					"value": false,
					"t":     float64(1628229842692),
					"1":     "false",
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
	if stateEvent.DeviceID != "6c95a93fd9xxx***" {
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
	if stateEvent.GetDeviceID() != "6c95a93fd9xxx***" {
		t.Errorf("Expected GetDeviceID()=6c95a93fd9xxx***, got %s", stateEvent.GetDeviceID())
	}
	if stateEvent.GetEventType() != "device_state_change" {
		t.Errorf("Expected GetEventType()=device_state_change, got %s", stateEvent.GetEventType())
	}
}

func TestParseEvent_DeviceStateChange_LEDSwitch(t *testing.T) {
	// Test with led_switch_1 as mentioned in requirements
	rawMessage := map[string]interface{}{
		"protocol": float64(4), // Changed from 1000 to 4 (ProtocolDeviceReport)
		"data": map[string]interface{}{
			"devId": "test-device-123",
			"status": []interface{}{
				map[string]interface{}{
					"code":  "led_switch_1",
					"value": true,
					"t":     float64(1628229842692),
				},
			},
		},
	}

	event, err := ParseEvent(rawMessage)
	if err != nil {
		t.Fatalf("Failed to parse event: %v", err)
	}

	stateEvent := event.(*DeviceStateChangeEvent)

	// Test led_switch_1 specifically
	if !stateEvent.HasStatusCode("led_switch_1") {
		t.Error("Expected HasStatusCode(led_switch_1) to return true")
	}
	if stateEvent.GetBooleanStatusValue("led_switch_1") != true {
		t.Error("Expected GetBooleanStatusValue(led_switch_1) to return true")
	}
}

func TestParseEvent_DeviceOnline(t *testing.T) {
	rawMessage := map[string]interface{}{
		"protocol": float64(20),
		"data": map[string]interface{}{
			"devId":      "002dj00118fe34d9****",
			"productKey": "The product key defined on the Tuya Developer Platform",
			"bizCode":    "online",
			"bizData": map[string]interface{}{
				"time": float64(146052438362),
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

	if onlineEvent.DeviceID != "002dj00118fe34d9****" {
		t.Errorf("Expected DeviceID=002dj00118fe34d9****, got %s", onlineEvent.DeviceID)
	}
	if onlineEvent.ProductKey != "The product key defined on the Tuya Developer Platform" {
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
	rawMessage := map[string]interface{}{
		"protocol": float64(20),
		"data": map[string]interface{}{
			"devId":      "002dj00118fe34d9****",
			"productKey": "The product key defined on the Tuya Developer Platform",
			"bizCode":    "offline",
			"bizData": map[string]interface{}{
				"time": float64(146052438362),
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
	rawMessage := map[string]interface{}{
		"protocol": float64(20),
		"data": map[string]interface{}{
			"devId":      "002dj00118fe34d9****",
			"productKey": "The product key defined on the Tuya Developer Platform",
			"bizCode":    "nameUpdate",
			"bizData": map[string]interface{}{
				"devId": "002dj00118fe34d9****",
				"name":  "new name",
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
	rawMessage := map[string]interface{}{
		"protocol": float64(20),
		"data": map[string]interface{}{
			"devId":      "002dj00118fe34d9****",
			"productKey": "The product key defined on the Tuya Developer Platform",
			"bizCode":    "delete",
			"bizData": map[string]interface{}{
				"devId": "002dj00118fe34d9****",
				"uid":   "ay1529485403390S****",
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

	if stateEvent.DeviceID != "test-device" {
		t.Errorf("Expected DeviceID=test-device, got %s", stateEvent.DeviceID)
	}
}

func TestTypedEventListener(t *testing.T) {
	var receivedStateEvents []*DeviceStateChangeEvent
	var receivedOnlineEvents []*DeviceOnlineEvent
	var receivedOfflineEvents []*DeviceOfflineEvent

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
	event := &DeviceStateChangeEvent{
		Status: []DeviceStatusChange{
			{Code: "bool_true", Value: true},
			{Code: "bool_false", Value: false},
			{Code: "string_true", Value: "true"},
			{Code: "string_false", Value: "false"},
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
	rawMessage := map[string]interface{}{
		"protocol": float64(999), // Invalid protocol
		"data": map[string]interface{}{
			"devId": "test-device",
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
	// Test missing protocol
	rawMessage := map[string]interface{}{
		"data": map[string]interface{}{
			"devId": "test-device",
		},
	}

	_, err := ParseEvent(rawMessage)
	if err == nil {
		t.Error("Expected error for missing protocol, got nil")
	}

	// Test missing data
	rawMessage = map[string]interface{}{
		"protocol": float64(1000),
	}

	_, err = ParseEvent(rawMessage)
	if err == nil {
		t.Error("Expected error for missing data, got nil")
	}

	// Test missing device ID in state change
	rawMessage = map[string]interface{}{
		"protocol": float64(1000),
		"data": map[string]interface{}{
			"status": []interface{}{},
		},
	}

	_, err = ParseEvent(rawMessage)
	if err == nil {
		t.Error("Expected error for missing device ID in state change, got nil")
	}
}

func TestParseDeviceStateChangeEventPublicMapAdapter(t *testing.T) {
	event, err := ParseDeviceStateChangeEvent(map[string]interface{}{
		"dataId":     "synthetic-data-id",
		"devId":      "synthetic-device-id",
		"productKey": "synthetic-product-key",
		"status": []interface{}{
			map[string]interface{}{"code": "switch", "value": true, "t": float64(1234)},
		},
	})
	if err != nil {
		t.Fatalf("ParseDeviceStateChangeEvent() error = %v", err)
	}
	if event.DeviceID != "synthetic-device-id" || event.DataID != "synthetic-data-id" {
		t.Fatalf("parsed event identity = %+v", event)
	}
	if len(event.Status) != 1 || event.Status[0].Code != "switch" || event.Status[0].Timestamp != 1234 || event.Status[0].Value != true {
		t.Fatalf("parsed event status = %+v", event.Status)
	}
	if _, err := ParseDeviceStateChangeEvent(map[string]interface{}{"status": "not-an-array"}); err == nil {
		t.Fatal("ParseDeviceStateChangeEvent() accepted a status value with the wrong shape")
	}
}

func TestParseDeviceManagementEventPublicMapAdapter(t *testing.T) {
	event, err := ParseDeviceManagementEvent(map[string]interface{}{
		"productKey": "synthetic-product-key",
		"bizCode":    BizcodeOnline,
		"bizData": map[string]interface{}{
			"devId": "synthetic-device-id",
			"time":  float64(5678),
		},
	})
	if err != nil {
		t.Fatalf("ParseDeviceManagementEvent() error = %v", err)
	}
	online, ok := event.(*DeviceOnlineEvent)
	if !ok {
		t.Fatalf("event type = %T, want *DeviceOnlineEvent", event)
	}
	if online.DeviceID != "synthetic-device-id" || online.ProductKey != "synthetic-product-key" || online.Time != 5678 {
		t.Fatalf("parsed online event = %+v", online)
	}
	genericEvent, err := ParseDeviceManagementEvent(map[string]interface{}{
		"devId":      "synthetic-device-id",
		"productKey": "synthetic-product-key",
		"bizCode":    "futureEvent",
		"bizData": map[string]interface{}{
			"newProviderField": "retained",
		},
	})
	if err != nil {
		t.Fatalf("ParseDeviceManagementEvent(unknown code) error = %v", err)
	}
	management, ok := genericEvent.(*DeviceManagementEvent)
	if !ok || management.BizData["newProviderField"] != "retained" {
		t.Fatalf("generic management event = %#v", genericEvent)
	}
	if _, err := ParseDeviceManagementEvent(map[string]interface{}{"bizData": "not-an-object"}); err == nil {
		t.Fatal("ParseDeviceManagementEvent() accepted a business payload with the wrong shape")
	}
}

func TestProcessMQTTMessage_InvalidJSON(t *testing.T) {
	invalidJSON := `{"protocol": 1000, "data": {invalid json`

	err := ProcessMQTTMessage(invalidJSON, []EventListener{})
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

// Example usage demonstration
func ExampleProcessMQTTMessage() {
	// Example MQTT message for LED switch state change
	messageJSON := `{
		"protocol": 1000,
		"data": {
			"devId": "my-smart-switch",
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
		OnOnline: func(event *DeviceOnlineEvent) {
			// Handle device coming online
		},
		OnOffline: func(event *DeviceOfflineEvent) {
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
}
