package tuya

import (
	"fmt"
	"testing"
)

func TestPowerCapability(t *testing.T) {
	power := &PowerCapability{}

	// Test boolean input
	err := power.SetValue(true)
	if err != nil {
		t.Fatalf("Failed to set boolean value: %v", err)
	}
	if power.State != PowerStateOn {
		t.Errorf("Expected state 'on', got %s", power.State)
	}

	// Test string input
	err = power.SetValue("off")
	if err != nil {
		t.Fatalf("Failed to set string value: %v", err)
	}
	if power.State != PowerStateOff {
		t.Errorf("Expected state 'off', got %s", power.State)
	}

	// Test invalid input
	err = power.SetValue("invalid")
	if err == nil {
		t.Error("Expected error for invalid power state")
	}

	// Test interface methods
	if power.GetCapabilityType() != CapabilityTypePower {
		t.Errorf("Expected capability type 'power', got %s", power.GetCapabilityType())
	}
	if power.GetValue() != PowerStateOff {
		t.Errorf("Expected value 'off', got %v", power.GetValue())
	}
}

func TestBrightnessCapability(t *testing.T) {
	brightness := &BrightnessCapability{}

	// Test int input
	err := brightness.SetValue(75)
	if err != nil {
		t.Fatalf("Failed to set int value: %v", err)
	}
	if brightness.Level != 75 {
		t.Errorf("Expected level 75, got %d", brightness.Level)
	}

	// Test float64 input
	err = brightness.SetValue(float64(50))
	if err != nil {
		t.Fatalf("Failed to set float64 value: %v", err)
	}
	if brightness.Level != 50 {
		t.Errorf("Expected level 50, got %d", brightness.Level)
	}

	// Test string input
	err = brightness.SetValue("25")
	if err != nil {
		t.Fatalf("Failed to set string value: %v", err)
	}
	if brightness.Level != 25 {
		t.Errorf("Expected level 25, got %d", brightness.Level)
	}

	// Test invalid range
	err = brightness.SetValue(150)
	if err == nil {
		t.Error("Expected error for brightness level > 100")
	}

	err = brightness.SetValue(-10)
	if err == nil {
		t.Error("Expected error for brightness level < 0")
	}

	// Test interface methods
	if brightness.GetCapabilityType() != CapabilityTypeBrightness {
		t.Errorf("Expected capability type 'brightness', got %s", brightness.GetCapabilityType())
	}
	if brightness.GetValue() != 25 {
		t.Errorf("Expected value 25, got %v", brightness.GetValue())
	}
}

func TestColorCapability(t *testing.T) {
	color := &ColorCapability{}

	// Test map[string]interface{} input
	colorMap := map[string]interface{}{
		"hue":        float64(180),
		"saturation": float64(75),
		"value":      float64(50),
	}
	err := color.SetValue(colorMap)
	if err != nil {
		t.Fatalf("Failed to set color map value: %v", err)
	}
	if color.Hue != 180 || color.Saturation != 75 || color.Value != 50 {
		t.Errorf("Expected HSV(180,75,50), got HSV(%d,%d,%d)", color.Hue, color.Saturation, color.Value)
	}

	// Test map[string]int input
	colorIntMap := map[string]int{
		"hue":        270,
		"saturation": 80,
		"value":      60,
	}
	err = color.SetValue(colorIntMap)
	if err != nil {
		t.Fatalf("Failed to set color int map value: %v", err)
	}
	if color.Hue != 270 || color.Saturation != 80 || color.Value != 60 {
		t.Errorf("Expected HSV(270,80,60), got HSV(%d,%d,%d)", color.Hue, color.Saturation, color.Value)
	}

	// Test hue wrapping
	colorMap["hue"] = float64(390) // Should wrap to 30
	color.SetValue(colorMap)
	if color.Hue != 30 {
		t.Errorf("Expected hue 30 (wrapped from 390), got %d", color.Hue)
	}

	// Test interface methods
	if color.GetCapabilityType() != CapabilityTypeColor {
		t.Errorf("Expected capability type 'color', got %s", color.GetCapabilityType())
	}

	value := color.GetValue().(map[string]int)
	if value["hue"] != 30 || value["saturation"] != 75 || value["value"] != 50 {
		t.Errorf("Expected HSV map(30,75,50), got %v", value)
	}
}

func TestColorTemperatureCapability(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		wantErr   bool
		wantValue int
	}{
		{name: "valid int warm", input: 400, wantValue: 400},
		{name: "valid int cool", input: 153, wantValue: 153},
		{name: "valid int max", input: 500, wantValue: 500},
		{name: "valid float64", input: float64(250), wantValue: 250},
		{name: "valid string", input: "300", wantValue: 300},
		{name: "too low", input: 100, wantErr: true},
		{name: "too high", input: 600, wantErr: true},
		{name: "invalid string", input: "abc", wantErr: true},
		{name: "invalid type", input: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := &ColorTemperatureCapability{}
			err := ct.SetValue(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if ct.Mireds != tt.wantValue {
				t.Errorf("Expected %d mireds, got %d", tt.wantValue, ct.Mireds)
			}
		})
	}

	// Test interface methods
	ct := &ColorTemperatureCapability{Mireds: 300}
	if ct.GetCapabilityType() != CapabilityTypeColorTemperature {
		t.Errorf("Expected capability type %q, got %q", CapabilityTypeColorTemperature, ct.GetCapabilityType())
	}
	if ct.GetValue() != 300 {
		t.Errorf("Expected value 300, got %v", ct.GetValue())
	}
}

func TestTemperatureSensorCapability(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		wantErr   bool
		wantValue float64
	}{
		{name: "valid float64", input: float64(22.5), wantValue: 22.5},
		{name: "valid int", input: 25, wantValue: 25.0},
		{name: "valid string", input: "18.3", wantValue: 18.3},
		{name: "negative temp", input: float64(-10.0), wantValue: -10.0},
		{name: "zero", input: float64(0), wantValue: 0},
		{name: "invalid string", input: "abc", wantErr: true},
		{name: "invalid type", input: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &TemperatureSensorCapability{}
			err := ts.SetValue(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if ts.Temperature != tt.wantValue {
				t.Errorf("Expected %f, got %f", tt.wantValue, ts.Temperature)
			}
		})
	}

	// Test interface methods
	ts := &TemperatureSensorCapability{Temperature: 22.5}
	if ts.GetCapabilityType() != CapabilityTypeTemperatureSensor {
		t.Errorf("Expected capability type %q, got %q", CapabilityTypeTemperatureSensor, ts.GetCapabilityType())
	}
	if ts.GetValue() != 22.5 {
		t.Errorf("Expected value 22.5, got %v", ts.GetValue())
	}
}

func TestColorTemperatureMapping_Bidirectional(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	// Test Tuya→Capability: temp_value 0 → 153 mireds (warmest / 6500K)
	status := []DeviceStatusChange{
		{Code: "temp_value", Value: float64(0)},
	}
	capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("dj", status)
	if err != nil {
		t.Fatalf("Failed to convert color temp status: %v", err)
	}
	if len(capabilities) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(capabilities))
	}
	ctCap, ok := capabilities[0].(*ColorTemperatureCapability)
	if !ok {
		t.Fatal("Expected ColorTemperatureCapability")
	}
	if ctCap.Mireds != 153 {
		t.Errorf("Expected 153 mireds for Tuya 0, got %d", ctCap.Mireds)
	}

	// Test Tuya→Capability: temp_value 1000 → 500 mireds (coolest / 2000K)
	status = []DeviceStatusChange{
		{Code: "temp_value", Value: float64(1000)},
	}
	capabilities, err = dcm.GetCapabilitiesFromTuyaStatus("dj", status)
	if err != nil {
		t.Fatalf("Failed to convert color temp status: %v", err)
	}
	ctCap = capabilities[0].(*ColorTemperatureCapability)
	if ctCap.Mireds != 500 {
		t.Errorf("Expected 500 mireds for Tuya 1000, got %d", ctCap.Mireds)
	}

	// Test Capability→Tuya round trip: 153 mireds → Tuya 0
	code, value, err := dcm.GetTuyaCommandFromCapability("dj", &ColorTemperatureCapability{Mireds: 153})
	if err != nil {
		t.Fatalf("Failed to convert color temp capability: %v", err)
	}
	if code != "temp_value" {
		t.Errorf("Expected code 'temp_value', got %s", code)
	}
	if value.(int) != 0 {
		t.Errorf("Expected Tuya value 0 for 153 mireds, got %v", value)
	}

	// Test Capability→Tuya round trip: 500 mireds → Tuya 1000
	_, value, err = dcm.GetTuyaCommandFromCapability("dj", &ColorTemperatureCapability{Mireds: 500})
	if err != nil {
		t.Fatalf("Failed to convert color temp capability: %v", err)
	}
	if value.(int) != 1000 {
		t.Errorf("Expected Tuya value 1000 for 500 mireds, got %v", value)
	}
}

func TestTemperatureSensorMapping_Bidirectional(t *testing.T) {
	// wsdcg category is now in initializeStandardMappings()
	dcm := NewDeviceCapabilityMap()

	// Test Tuya→Capability: 225 (tenths of degree) → 22.5°C
	status := []DeviceStatusChange{
		{Code: "va_temperature", Value: float64(225)},
	}
	capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("wsdcg", status)
	if err != nil {
		t.Fatalf("Failed to convert temperature status: %v", err)
	}
	if len(capabilities) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(capabilities))
	}
	tsCap, ok := capabilities[0].(*TemperatureSensorCapability)
	if !ok {
		t.Fatal("Expected TemperatureSensorCapability")
	}
	if tsCap.Temperature != 22.5 {
		t.Errorf("Expected 22.5°C, got %f", tsCap.Temperature)
	}

	// Test Capability→Tuya round trip: 22.5°C → 225
	code, value, err := dcm.GetTuyaCommandFromCapability("wsdcg", &TemperatureSensorCapability{Temperature: 22.5})
	if err != nil {
		t.Fatalf("Failed to convert temperature capability: %v", err)
	}
	if code != "va_temperature" {
		t.Errorf("Expected code 'va_temperature', got %s", code)
	}
	if value.(int) != 225 {
		t.Errorf("Expected Tuya value 225, got %v", value)
	}

	// Test negative temperature: -50 → -5.0°C
	status = []DeviceStatusChange{
		{Code: "va_temperature", Value: float64(-50)},
	}
	capabilities, err = dcm.GetCapabilitiesFromTuyaStatus("wsdcg", status)
	if err != nil {
		t.Fatalf("Failed to convert negative temperature: %v", err)
	}
	tsCap = capabilities[0].(*TemperatureSensorCapability)
	if tsCap.Temperature != -5.0 {
		t.Errorf("Expected -5.0°C, got %f", tsCap.Temperature)
	}
}

func TestHumiditySensorCapability(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		wantErr   bool
		wantValue int
	}{
		{name: "valid int 50%", input: 50, wantValue: 50},
		{name: "valid int 0%", input: 0, wantValue: 0},
		{name: "valid int 100%", input: 100, wantValue: 100},
		{name: "valid float64", input: float64(65), wantValue: 65},
		{name: "valid string", input: "42", wantValue: 42},
		{name: "too low", input: -1, wantErr: true},
		{name: "too high", input: 101, wantErr: true},
		{name: "invalid string", input: "abc", wantErr: true},
		{name: "invalid type", input: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hs := &HumiditySensorCapability{}
			err := hs.SetValue(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if hs.Humidity != tt.wantValue {
				t.Errorf("Expected %d%%, got %d%%", tt.wantValue, hs.Humidity)
			}
		})
	}

	// Test interface methods
	hs := &HumiditySensorCapability{Humidity: 65}
	if hs.GetCapabilityType() != CapabilityTypeHumiditySensor {
		t.Errorf("Expected capability type %q, got %q", CapabilityTypeHumiditySensor, hs.GetCapabilityType())
	}
	if hs.GetValue() != 65 {
		t.Errorf("Expected value 65, got %v", hs.GetValue())
	}
}

func TestFanSpeedCapability(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		wantErr   bool
		wantValue int
	}{
		{name: "valid int 75%", input: 75, wantValue: 75},
		{name: "valid int 0%", input: 0, wantValue: 0},
		{name: "valid int 100%", input: 100, wantValue: 100},
		{name: "valid float64", input: float64(50), wantValue: 50},
		{name: "valid string", input: "25", wantValue: 25},
		{name: "too low", input: -1, wantErr: true},
		{name: "too high", input: 101, wantErr: true},
		{name: "invalid string", input: "abc", wantErr: true},
		{name: "invalid type", input: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &FanSpeedCapability{}
			err := fs.SetValue(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if fs.Speed != tt.wantValue {
				t.Errorf("Expected %d%%, got %d%%", tt.wantValue, fs.Speed)
			}
		})
	}

	// Test interface methods
	fs := &FanSpeedCapability{Speed: 50}
	if fs.GetCapabilityType() != CapabilityTypeFanSpeed {
		t.Errorf("Expected capability type %q, got %q", CapabilityTypeFanSpeed, fs.GetCapabilityType())
	}
	if fs.GetValue() != 50 {
		t.Errorf("Expected value 50, got %v", fs.GetValue())
	}
}

func TestHumiditySensorMapping_Bidirectional(t *testing.T) {
	// wsdcg category is now in initializeStandardMappings()
	dcm := NewDeviceCapabilityMap()

	// Test Tuya→Capability: 65 → 65%
	status := []DeviceStatusChange{
		{Code: "va_humidity", Value: float64(65)},
	}
	capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("wsdcg", status)
	if err != nil {
		t.Fatalf("Failed to convert humidity status: %v", err)
	}
	if len(capabilities) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(capabilities))
	}
	hsCap, ok := capabilities[0].(*HumiditySensorCapability)
	if !ok {
		t.Fatal("Expected HumiditySensorCapability")
	}
	if hsCap.Humidity != 65 {
		t.Errorf("Expected 65%%, got %d%%", hsCap.Humidity)
	}

	// Test Capability→Tuya round trip: 65% → 65
	code, value, err := dcm.GetTuyaCommandFromCapability("wsdcg", &HumiditySensorCapability{Humidity: 65})
	if err != nil {
		t.Fatalf("Failed to convert humidity capability: %v", err)
	}
	if code != "va_humidity" {
		t.Errorf("Expected code 'va_humidity', got %s", code)
	}
	if value.(int) != 65 {
		t.Errorf("Expected Tuya value 65, got %v", value)
	}

	// Test boundary: 0%
	status = []DeviceStatusChange{
		{Code: "va_humidity", Value: float64(0)},
	}
	capabilities, err = dcm.GetCapabilitiesFromTuyaStatus("wsdcg", status)
	if err != nil {
		t.Fatalf("Failed to convert 0%% humidity: %v", err)
	}
	hsCap = capabilities[0].(*HumiditySensorCapability)
	if hsCap.Humidity != 0 {
		t.Errorf("Expected 0%%, got %d%%", hsCap.Humidity)
	}

	// Test boundary: 100%
	status = []DeviceStatusChange{
		{Code: "va_humidity", Value: float64(100)},
	}
	capabilities, err = dcm.GetCapabilitiesFromTuyaStatus("wsdcg", status)
	if err != nil {
		t.Fatalf("Failed to convert 100%% humidity: %v", err)
	}
	hsCap = capabilities[0].(*HumiditySensorCapability)
	if hsCap.Humidity != 100 {
		t.Errorf("Expected 100%%, got %d%%", hsCap.Humidity)
	}
}

func TestFanSpeedMapping_Bidirectional(t *testing.T) {
	dcm := NewDeviceCapabilityMap()
	// Fan with 4 discrete speed levels
	dcm.AddMapping("fs", "fan_speed_percent", createFanSpeedMapping("fan_speed_percent", 4))

	// Test Tuya→Capability: level 2 of 4 → 50%
	status := []DeviceStatusChange{
		{Code: "fan_speed_percent", Value: float64(2)},
	}
	capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("fs", status)
	if err != nil {
		t.Fatalf("Failed to convert fan speed status: %v", err)
	}
	if len(capabilities) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(capabilities))
	}
	fsCap, ok := capabilities[0].(*FanSpeedCapability)
	if !ok {
		t.Fatal("Expected FanSpeedCapability")
	}
	if fsCap.Speed != 50 {
		t.Errorf("Expected 50%%, got %d%%", fsCap.Speed)
	}

	// Test Capability→Tuya round trip: 50% → level 2
	code, value, err := dcm.GetTuyaCommandFromCapability("fs", &FanSpeedCapability{Speed: 50})
	if err != nil {
		t.Fatalf("Failed to convert fan speed capability: %v", err)
	}
	if code != "fan_speed_percent" {
		t.Errorf("Expected code 'fan_speed_percent', got %s", code)
	}
	if value.(int) != 2 {
		t.Errorf("Expected Tuya value 2, got %v", value)
	}

	// Test Tuya→Capability: level 4 of 4 → 100%
	status = []DeviceStatusChange{
		{Code: "fan_speed_percent", Value: float64(4)},
	}
	capabilities, err = dcm.GetCapabilitiesFromTuyaStatus("fs", status)
	if err != nil {
		t.Fatalf("Failed to convert max fan speed: %v", err)
	}
	fsCap = capabilities[0].(*FanSpeedCapability)
	if fsCap.Speed != 100 {
		t.Errorf("Expected 100%%, got %d%%", fsCap.Speed)
	}

	// Test Capability→Tuya: 100% → level 4
	_, value, err = dcm.GetTuyaCommandFromCapability("fs", &FanSpeedCapability{Speed: 100})
	if err != nil {
		t.Fatalf("Failed to convert 100%% fan speed: %v", err)
	}
	if value.(int) != 4 {
		t.Errorf("Expected Tuya value 4, got %v", value)
	}

	// Test Tuya→Capability: level 0 → 0%
	status = []DeviceStatusChange{
		{Code: "fan_speed_percent", Value: float64(0)},
	}
	capabilities, err = dcm.GetCapabilitiesFromTuyaStatus("fs", status)
	if err != nil {
		t.Fatalf("Failed to convert 0 fan speed: %v", err)
	}
	fsCap = capabilities[0].(*FanSpeedCapability)
	if fsCap.Speed != 0 {
		t.Errorf("Expected 0%%, got %d%%", fsCap.Speed)
	}
}

func TestDeviceCapabilityMap_TGQDimmer(t *testing.T) {
	// Test the specific example: switch_led_1 with tgq category
	dcm := NewDeviceCapabilityMap()

	// Test Tuya to Capability conversion (Event processing)
	status := []DeviceStatusChange{
		{Code: "switch_led_1", Value: true},
		{Code: "bright_value_v2", Value: float64(500)}, // Mid-range brightness
	}

	capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("tgq", status)
	if err != nil {
		t.Fatalf("Failed to convert Tuya status to capabilities: %v", err)
	}

	if len(capabilities) != 2 {
		t.Fatalf("Expected 2 capabilities, got %d", len(capabilities))
	}

	// Check power capability
	var powerCap *PowerCapability
	var brightnessCap *BrightnessCapability

	for _, cap := range capabilities {
		switch c := cap.(type) {
		case *PowerCapability:
			powerCap = c
		case *BrightnessCapability:
			brightnessCap = c
		}
	}

	if powerCap == nil {
		t.Fatal("Expected power capability")
	}
	if powerCap.State != PowerStateOn {
		t.Errorf("Expected power state 'on', got %s", powerCap.State)
	}

	if brightnessCap == nil {
		t.Fatal("Expected brightness capability")
	}
	// Tuya brightness 500 in range 10-1000 should convert to ~49% (approximately)
	if brightnessCap.Level < 45 || brightnessCap.Level > 55 {
		t.Errorf("Expected brightness around 49%%, got %d%%", brightnessCap.Level)
	}

	// Test Capability to Tuya conversion (Command processing)
	newPowerCap := &PowerCapability{State: PowerStateOff}
	code, value, err := dcm.GetTuyaCommandFromCapability("tgq", newPowerCap)
	if err != nil {
		t.Fatalf("Failed to convert power capability to Tuya command: %v", err)
	}
	if code != "switch_led_1" {
		t.Errorf("Expected code 'switch_led_1', got %s", code)
	}
	if value != false {
		t.Errorf("Expected value false, got %v", value)
	}

	newBrightnessCap := &BrightnessCapability{Level: 80}
	code, value, err = dcm.GetTuyaCommandFromCapability("tgq", newBrightnessCap)
	if err != nil {
		t.Fatalf("Failed to convert brightness capability to Tuya command: %v", err)
	}
	if code != "bright_value_v2" {
		t.Errorf("Expected code 'bright_value_v2', got %s", code)
	}
	// 80% should convert to around 802 in Tuya's 10-1000 range
	tuyaValue := value.(int)
	if tuyaValue < 800 || tuyaValue > 810 {
		t.Errorf("Expected Tuya brightness around 802, got %d", tuyaValue)
	}
}

func TestDeviceCapabilityMap_DJLight(t *testing.T) {
	// Test Light (dj) category mappings
	dcm := NewDeviceCapabilityMap()

	status := []DeviceStatusChange{
		{Code: "switch_led", Value: false},
		{Code: "bright_value", Value: float64(300)},
		{Code: "colour_data_v2", Value: "00b403e803e8"}, // Example HSV hex
	}

	capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("dj", status)
	if err != nil {
		t.Fatalf("Failed to convert DJ light status: %v", err)
	}

	if len(capabilities) != 3 {
		t.Fatalf("Expected 3 capabilities, got %d", len(capabilities))
	}

	// Find capabilities by type
	var powerCap *PowerCapability
	var brightnessCap *BrightnessCapability
	var colorCap *ColorCapability

	for _, cap := range capabilities {
		switch c := cap.(type) {
		case *PowerCapability:
			powerCap = c
		case *BrightnessCapability:
			brightnessCap = c
		case *ColorCapability:
			colorCap = c
		}
	}

	// Verify power capability
	if powerCap == nil || powerCap.State != PowerStateOff {
		t.Error("Expected power capability with state 'off'")
	}

	// Verify brightness capability
	if brightnessCap == nil || brightnessCap.Level != 30 {
		t.Errorf("Expected brightness capability with level 30, got %d", brightnessCap.Level)
	}

	// Verify color capability
	if colorCap == nil {
		t.Fatal("Expected color capability")
	}
	// 0x00b4 = 180 degrees, 0x03e8 = 1000 -> 100%, 0x03e8 = 1000 -> 100%
	if colorCap.Hue != 180 {
		t.Errorf("Expected hue 180, got %d", colorCap.Hue)
	}
	if colorCap.Saturation != 100 {
		t.Errorf("Expected saturation 100, got %d", colorCap.Saturation)
	}
	if colorCap.Value != 100 {
		t.Errorf("Expected value 100, got %d", colorCap.Value)
	}
}

func TestDeviceCapabilityMap_EventConversion(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	// Create a mock device state change event
	event := &DeviceStateChangeEvent{
		DeviceID:   "test-dimmer-123",
		ProductKey: "test-product-key",
		Status: []DeviceStatusChange{
			{Code: "switch_led_1", Value: true},
			{Code: "bright_value_v2", Value: float64(750)},
		},
	}

	standardizedEvent, err := dcm.ConvertToStandardizedEvent(event, "tgq")
	if err != nil {
		t.Fatalf("Failed to convert to standardized event: %v", err)
	}

	if standardizedEvent.DeviceID != "test-dimmer-123" {
		t.Errorf("Expected device ID 'test-dimmer-123', got %s", standardizedEvent.DeviceID)
	}
	if standardizedEvent.Category != "tgq" {
		t.Errorf("Expected category 'tgq', got %s", standardizedEvent.Category)
	}
	if len(standardizedEvent.Capabilities) != 2 {
		t.Fatalf("Expected 2 capabilities, got %d", len(standardizedEvent.Capabilities))
	}

	// Check that we have power and brightness capabilities
	hasPower := false
	hasBrightness := false
	for _, cap := range standardizedEvent.Capabilities {
		switch cap.GetCapabilityType() {
		case CapabilityTypePower:
			hasPower = true
			if cap.GetValue() != PowerStateOn {
				t.Errorf("Expected power state 'on', got %v", cap.GetValue())
			}
		case CapabilityTypeBrightness:
			hasBrightness = true
			// 750 in range 10-1000 should be ~75%
			level := cap.GetValue().(int)
			if level < 70 || level > 80 {
				t.Errorf("Expected brightness around 75%%, got %d%%", level)
			}
		}
	}

	if !hasPower {
		t.Error("Expected power capability")
	}
	if !hasBrightness {
		t.Error("Expected brightness capability")
	}
}

func TestDeviceCapabilityMap_CapabilityQueries(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	// Test capability detection
	if !dcm.HasPowerCapability("tgq") {
		t.Error("Expected tgq category to have power capability")
	}
	if !dcm.HasBrightnessCapability("tgq") {
		t.Error("Expected tgq category to have brightness capability")
	}
	if dcm.HasColorCapability("tgq") {
		t.Error("Expected tgq category to NOT have color capability")
	}

	if !dcm.HasColorCapability("dj") {
		t.Error("Expected dj category to have color capability")
	}

	// Test new switch/sensor capability queries
	if !dcm.HasPowerCapability("kg") {
		t.Error("Expected kg category to have power capability")
	}
	if !dcm.HasPowerCapability("cz") {
		t.Error("Expected cz category to have power capability")
	}
	if !dcm.HasPowerCapability("pc") {
		t.Error("Expected pc category to have power capability")
	}
	// Test tgkg (wall switch dimmer) capabilities
	if !dcm.HasPowerCapability("tgkg") {
		t.Error("Expected tgkg category to have power capability")
	}
	if !dcm.HasBrightnessCapability("tgkg") {
		t.Error("Expected tgkg category to have brightness capability")
	}

	if !dcm.HasTemperatureSensorCapability("wsdcg") {
		t.Error("Expected wsdcg category to have temperature sensor capability")
	}
	if !dcm.HasHumiditySensorCapability("wsdcg") {
		t.Error("Expected wsdcg category to have humidity sensor capability")
	}
	if !dcm.HasContactSensorCapability("mcs") {
		t.Error("Expected mcs category to have contact sensor capability")
	}

	// Test new light/fan/cover/lock capability queries
	if !dcm.HasPowerCapability("dc") {
		t.Error("Expected dc category to have power capability")
	}
	if !dcm.HasBrightnessCapability("dc") {
		t.Error("Expected dc category to have brightness capability")
	}
	if !dcm.HasColorCapability("dc") {
		t.Error("Expected dc category to have color capability")
	}
	if !dcm.HasColorTemperatureCapability("dc") {
		t.Error("Expected dc category to have color temperature capability")
	}
	if !dcm.HasPowerCapability("fwd") {
		t.Error("Expected fwd category to have power capability")
	}
	if !dcm.HasPowerCapability("fs") {
		t.Error("Expected fs category to have power capability")
	}
	if !dcm.HasFanSpeedCapability("fs") {
		t.Error("Expected fs category to have fan speed capability")
	}
	if !dcm.HasPowerCapability("fsd") {
		t.Error("Expected fsd category to have power capability")
	}
	if !dcm.HasFanSpeedCapability("fsd") {
		t.Error("Expected fsd category to have fan speed capability")
	}
	if !dcm.HasPowerCapability("cl") {
		t.Error("Expected cl category to have power capability")
	}
	if !dcm.HasWindowCoveringCapability("cl") {
		t.Error("Expected cl category to have window covering capability")
	}
	if !dcm.HasPowerCapability("clkg") {
		t.Error("Expected clkg category to have power capability")
	}
	if !dcm.HasWindowCoveringCapability("clkg") {
		t.Error("Expected clkg category to have window covering capability")
	}
	if !dcm.HasLockCapability("ms") {
		t.Error("Expected ms category to have lock capability")
	}
	if !dcm.HasLockCapability("jtmspro") {
		t.Error("Expected jtmspro category to have lock capability")
	}

	// Test supported categories
	categories := dcm.GetSupportedCategories()
	expectedCategories := []string{"tgkg", "tgq", "dj", "dd", "kg", "cz", "pc", "wsdcg", "mcs", "dc", "fwd", "fs", "fsd", "cl", "clkg", "ms", "jtmspro", "sp", "ipc"}
	for _, expected := range expectedCategories {
		found := false
		for _, category := range categories {
			if category == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected category %s to be supported", expected)
		}
	}

	// Test supported codes for category
	tgqCodes := dcm.GetSupportedCodesForCategory("tgq")
	expectedCodes := []string{"switch_led_1", "bright_value_v2"}
	for _, expected := range expectedCodes {
		found := false
		for _, code := range tgqCodes {
			if code == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected code %s to be supported in tgq category", expected)
		}
	}
}

func TestDeviceCapabilityMap_CustomMapping(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	// Add a custom mapping for a new device category
	customMapping := CapabilityMapping{
		CapabilityType: CapabilityTypePower,
		TuyaToCapability: func(value interface{}) (Capability, error) {
			capability := &PowerCapability{}
			err := capability.SetValue(value)
			return capability, err
		},
		CapabilityToTuya: func(capability Capability) (string, interface{}, error) {
			if power, ok := capability.(*PowerCapability); ok {
				return "custom_power", power.State == PowerStateOn, nil
			}
			return "", nil, fmt.Errorf("invalid capability type")
		},
	}

	dcm.AddMapping("custom_category", "custom_power_code", customMapping)

	// Test the custom mapping
	status := []DeviceStatusChange{
		{Code: "custom_power_code", Value: true},
	}

	capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("custom_category", status)
	if err != nil {
		t.Fatalf("Failed to use custom mapping: %v", err)
	}

	if len(capabilities) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(capabilities))
	}

	powerCap := capabilities[0].(*PowerCapability)
	if powerCap.State != PowerStateOn {
		t.Errorf("Expected power state 'on', got %s", powerCap.State)
	}

	// Test reverse mapping
	code, value, err := dcm.GetTuyaCommandFromCapability("custom_category", powerCap)
	if err != nil {
		t.Fatalf("Failed to convert custom capability: %v", err)
	}
	if code != "custom_power" {
		t.Errorf("Expected code 'custom_power', got %s", code)
	}
	if value != true {
		t.Errorf("Expected value true, got %v", value)
	}
}

func TestDeviceCapabilityMap_UnsupportedCategory(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	// Test unsupported category
	status := []DeviceStatusChange{
		{Code: "some_code", Value: true},
	}

	_, err := dcm.GetCapabilitiesFromTuyaStatus("unsupported_category", status)
	if err == nil {
		t.Error("Expected error for unsupported category")
	}

	powerCap := &PowerCapability{State: PowerStateOn}
	_, _, err = dcm.GetTuyaCommandFromCapability("unsupported_category", powerCap)
	if err == nil {
		t.Error("Expected error for unsupported category in command conversion")
	}
}

func TestDeviceCapabilityMap_StripLight(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	// Test Strip Light (dd) category
	status := []DeviceStatusChange{
		{Code: "switch_led", Value: true},
		{Code: "bright_value", Value: float64(600)},
		{Code: "colour_data", Value: "01680320012c"}, // Different color format
	}

	capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("dd", status)
	if err != nil {
		t.Fatalf("Failed to convert strip light status: %v", err)
	}

	if len(capabilities) != 3 {
		t.Fatalf("Expected 3 capabilities, got %d", len(capabilities))
	}

	// Check that all three capability types are present
	hasPower := false
	hasBrightness := false
	hasColor := false

	for _, cap := range capabilities {
		switch cap.GetCapabilityType() {
		case CapabilityTypePower:
			hasPower = true
		case CapabilityTypeBrightness:
			hasBrightness = true
		case CapabilityTypeColor:
			hasColor = true
		}
	}

	if !hasPower || !hasBrightness || !hasColor {
		t.Error("Expected power, brightness, and color capabilities for strip light")
	}
}

// Example usage test demonstrating the complete workflow
func ExampleDeviceCapabilityMap() {
	// Initialize the capability mapping system
	dcm := NewDeviceCapabilityMap()

	// Simulate receiving a Tuya device state change event
	event := &DeviceStateChangeEvent{
		DeviceID:   "my-smart-dimmer",
		ProductKey: "dimmer-product-key",
		Status: []DeviceStatusChange{
			{Code: "switch_led_1", Value: true},
			{Code: "bright_value_v2", Value: float64(800)},
		},
	}

	// Convert to standardized capabilities
	standardizedEvent, _ := dcm.ConvertToStandardizedEvent(event, "tgq")

	// Process standardized capabilities
	for _, capability := range standardizedEvent.Capabilities {
		switch cap := capability.(type) {
		case *PowerCapability:
			if cap.State == PowerStateOn {
				// Device is powered on
			}
		case *BrightnessCapability:
			// Use cap.Level (0-100 percentage)
			_ = cap.Level
		}
	}

	// Create commands using standardized capabilities
	powerOff := &PowerCapability{State: PowerStateOff}
	code, value, _ := dcm.GetTuyaCommandFromCapability("tgq", powerOff)

	// Send command: code="switch_led_1", value=false
	_ = code
	_ = value
}

func TestLockCapability(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		wantErr   bool
		wantValue LockState
	}{
		{name: "valid string locked", input: "locked", wantValue: LockStateLocked},
		{name: "valid string unlocked", input: "unlocked", wantValue: LockStateUnlocked},
		{name: "valid bool true (locked)", input: true, wantValue: LockStateLocked},
		{name: "valid bool false (unlocked)", input: false, wantValue: LockStateUnlocked},
		{name: "invalid string", input: "open", wantErr: true},
		{name: "invalid type int", input: 1, wantErr: true},
		{name: "invalid type float64", input: float64(1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &LockCapability{}
			err := l.SetValue(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if l.State != tt.wantValue {
				t.Errorf("Expected %q, got %q", tt.wantValue, l.State)
			}
		})
	}

	// Test interface methods
	l := &LockCapability{State: LockStateLocked}
	if l.GetCapabilityType() != CapabilityTypeLock {
		t.Errorf("Expected capability type %q, got %q", CapabilityTypeLock, l.GetCapabilityType())
	}
	if l.GetValue() != LockStateLocked {
		t.Errorf("Expected value %q, got %v", LockStateLocked, l.GetValue())
	}
}

func TestContactSensorCapability(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		wantErr   bool
		wantValue bool
	}{
		{name: "bool true (open)", input: true, wantValue: true},
		{name: "bool false (closed)", input: false, wantValue: false},
		{name: "string true", input: "true", wantValue: true},
		{name: "string false", input: "false", wantValue: false},
		{name: "string open", input: "open", wantValue: true},
		{name: "string closed", input: "closed", wantValue: false},
		{name: "invalid string", input: "maybe", wantErr: true},
		{name: "invalid type int", input: 1, wantErr: true},
		{name: "invalid type float64", input: float64(0), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := &ContactSensorCapability{}
			err := cs.SetValue(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if cs.IsOpen != tt.wantValue {
				t.Errorf("Expected IsOpen=%v, got %v", tt.wantValue, cs.IsOpen)
			}
		})
	}

	// Test interface methods
	cs := &ContactSensorCapability{IsOpen: true}
	if cs.GetCapabilityType() != CapabilityTypeContactSensor {
		t.Errorf("Expected capability type %q, got %q", CapabilityTypeContactSensor, cs.GetCapabilityType())
	}
	if cs.GetValue() != true {
		t.Errorf("Expected value true, got %v", cs.GetValue())
	}
}

func TestWindowCoveringCapability(t *testing.T) {
	tests := []struct {
		name      string
		input     interface{}
		wantErr   bool
		wantValue int
	}{
		{name: "valid int 50%", input: 50, wantValue: 50},
		{name: "valid int 0%", input: 0, wantValue: 0},
		{name: "valid int 100%", input: 100, wantValue: 100},
		{name: "valid float64", input: float64(75), wantValue: 75},
		{name: "valid string", input: "30", wantValue: 30},
		{name: "too low", input: -1, wantErr: true},
		{name: "too high", input: 101, wantErr: true},
		{name: "invalid string", input: "abc", wantErr: true},
		{name: "invalid type", input: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wc := &WindowCoveringCapability{}
			err := wc.SetValue(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if wc.Position != tt.wantValue {
				t.Errorf("Expected %d%%, got %d%%", tt.wantValue, wc.Position)
			}
		})
	}

	// Test interface methods
	wc := &WindowCoveringCapability{Position: 60}
	if wc.GetCapabilityType() != CapabilityTypeWindowCovering {
		t.Errorf("Expected capability type %q, got %q", CapabilityTypeWindowCovering, wc.GetCapabilityType())
	}
	if wc.GetValue() != 60 {
		t.Errorf("Expected value 60, got %v", wc.GetValue())
	}
}

func TestLockMapping_Bidirectional(t *testing.T) {
	dcm := NewDeviceCapabilityMap()
	dcm.AddMapping("ms", "closed_opened", createLockMapping("closed_opened"))

	// Test Tuya→Capability: true (locked)
	status := []DeviceStatusChange{
		{Code: "closed_opened", Value: true},
	}
	capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("ms", status)
	if err != nil {
		t.Fatalf("Failed to convert lock status: %v", err)
	}
	if len(capabilities) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(capabilities))
	}
	lockCap, ok := capabilities[0].(*LockCapability)
	if !ok {
		t.Fatal("Expected LockCapability")
	}
	if lockCap.State != LockStateLocked {
		t.Errorf("Expected locked, got %q", lockCap.State)
	}

	// Test Capability→Tuya round trip: locked → true
	code, value, err := dcm.GetTuyaCommandFromCapability("ms", &LockCapability{State: LockStateLocked})
	if err != nil {
		t.Fatalf("Failed to convert lock capability: %v", err)
	}
	if code != "closed_opened" {
		t.Errorf("Expected code 'closed_opened', got %s", code)
	}
	if value != true {
		t.Errorf("Expected Tuya value true, got %v", value)
	}

	// Test Tuya→Capability: false (unlocked)
	status = []DeviceStatusChange{
		{Code: "closed_opened", Value: false},
	}
	capabilities, err = dcm.GetCapabilitiesFromTuyaStatus("ms", status)
	if err != nil {
		t.Fatalf("Failed to convert unlocked status: %v", err)
	}
	lockCap = capabilities[0].(*LockCapability)
	if lockCap.State != LockStateUnlocked {
		t.Errorf("Expected unlocked, got %q", lockCap.State)
	}

	// Test Capability→Tuya: unlocked → false
	_, value, err = dcm.GetTuyaCommandFromCapability("ms", &LockCapability{State: LockStateUnlocked})
	if err != nil {
		t.Fatalf("Failed to convert unlocked capability: %v", err)
	}
	if value != false {
		t.Errorf("Expected Tuya value false, got %v", value)
	}
}

func TestContactSensorMapping_Bidirectional(t *testing.T) {
	// mcs category is now in initializeStandardMappings()
	dcm := NewDeviceCapabilityMap()

	// Test Tuya→Capability: true (open)
	status := []DeviceStatusChange{
		{Code: "doorcontact_state", Value: true},
	}
	capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("mcs", status)
	if err != nil {
		t.Fatalf("Failed to convert contact sensor status: %v", err)
	}
	if len(capabilities) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(capabilities))
	}
	csCap, ok := capabilities[0].(*ContactSensorCapability)
	if !ok {
		t.Fatal("Expected ContactSensorCapability")
	}
	if !csCap.IsOpen {
		t.Error("Expected IsOpen=true")
	}

	// Test Capability→Tuya round trip: open → true
	code, value, err := dcm.GetTuyaCommandFromCapability("mcs", &ContactSensorCapability{IsOpen: true})
	if err != nil {
		t.Fatalf("Failed to convert contact sensor capability: %v", err)
	}
	if code != "doorcontact_state" {
		t.Errorf("Expected code 'doorcontact_state', got %s", code)
	}
	if value != true {
		t.Errorf("Expected Tuya value true, got %v", value)
	}

	// Test Tuya→Capability: false (closed)
	status = []DeviceStatusChange{
		{Code: "doorcontact_state", Value: false},
	}
	capabilities, err = dcm.GetCapabilitiesFromTuyaStatus("mcs", status)
	if err != nil {
		t.Fatalf("Failed to convert closed contact: %v", err)
	}
	csCap = capabilities[0].(*ContactSensorCapability)
	if csCap.IsOpen {
		t.Error("Expected IsOpen=false")
	}

	// Test Capability→Tuya: closed → false
	_, value, err = dcm.GetTuyaCommandFromCapability("mcs", &ContactSensorCapability{IsOpen: false})
	if err != nil {
		t.Fatalf("Failed to convert closed contact capability: %v", err)
	}
	if value != false {
		t.Errorf("Expected Tuya value false, got %v", value)
	}
}

func TestWindowCoveringMapping_Bidirectional(t *testing.T) {
	dcm := NewDeviceCapabilityMap()
	dcm.AddMapping("cl", "percent_control", createWindowCoveringMapping("percent_control"))

	// Test Tuya→Capability: 75 → 75%
	status := []DeviceStatusChange{
		{Code: "percent_control", Value: float64(75)},
	}
	capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("cl", status)
	if err != nil {
		t.Fatalf("Failed to convert window covering status: %v", err)
	}
	if len(capabilities) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(capabilities))
	}
	wcCap, ok := capabilities[0].(*WindowCoveringCapability)
	if !ok {
		t.Fatal("Expected WindowCoveringCapability")
	}
	if wcCap.Position != 75 {
		t.Errorf("Expected 75%%, got %d%%", wcCap.Position)
	}

	// Test Capability→Tuya round trip: 75% → 75
	code, value, err := dcm.GetTuyaCommandFromCapability("cl", &WindowCoveringCapability{Position: 75})
	if err != nil {
		t.Fatalf("Failed to convert window covering capability: %v", err)
	}
	if code != "percent_control" {
		t.Errorf("Expected code 'percent_control', got %s", code)
	}
	if value.(int) != 75 {
		t.Errorf("Expected Tuya value 75, got %v", value)
	}

	// Test boundary: 0% (fully closed)
	status = []DeviceStatusChange{
		{Code: "percent_control", Value: float64(0)},
	}
	capabilities, err = dcm.GetCapabilitiesFromTuyaStatus("cl", status)
	if err != nil {
		t.Fatalf("Failed to convert 0%% position: %v", err)
	}
	wcCap = capabilities[0].(*WindowCoveringCapability)
	if wcCap.Position != 0 {
		t.Errorf("Expected 0%%, got %d%%", wcCap.Position)
	}

	// Test boundary: 100% (fully open)
	status = []DeviceStatusChange{
		{Code: "percent_control", Value: float64(100)},
	}
	capabilities, err = dcm.GetCapabilitiesFromTuyaStatus("cl", status)
	if err != nil {
		t.Fatalf("Failed to convert 100%% position: %v", err)
	}
	wcCap = capabilities[0].(*WindowCoveringCapability)
	if wcCap.Position != 100 {
		t.Errorf("Expected 100%%, got %d%%", wcCap.Position)
	}
}

// Table-driven tests for US-005: switch and sensor device category mappings

func TestSwitchCategories_Bidirectional(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	tests := []struct {
		name      string
		category  string
		tuyaCode  string
		tuyaValue interface{}
		wantState PowerState
		tuyaBack  interface{} // expected Tuya value from Capability→Tuya
	}{
		{name: "kg switch on", category: "kg", tuyaCode: "switch_1", tuyaValue: true, wantState: PowerStateOn, tuyaBack: true},
		{name: "kg switch off", category: "kg", tuyaCode: "switch_1", tuyaValue: false, wantState: PowerStateOff, tuyaBack: false},
		{name: "cz socket on", category: "cz", tuyaCode: "switch_1", tuyaValue: true, wantState: PowerStateOn, tuyaBack: true},
		{name: "cz socket off", category: "cz", tuyaCode: "switch_1", tuyaValue: false, wantState: PowerStateOff, tuyaBack: false},
		{name: "pc power strip on", category: "pc", tuyaCode: "switch_1", tuyaValue: true, wantState: PowerStateOn, tuyaBack: true},
		{name: "pc power strip off", category: "pc", tuyaCode: "switch_1", tuyaValue: false, wantState: PowerStateOff, tuyaBack: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Tuya → Capability
			status := []DeviceStatusChange{
				{Code: tt.tuyaCode, Value: tt.tuyaValue},
			}
			capabilities, err := dcm.GetCapabilitiesFromTuyaStatus(tt.category, status)
			if err != nil {
				t.Fatalf("Tuya→Capability failed: %v", err)
			}
			if len(capabilities) != 1 {
				t.Fatalf("Expected 1 capability, got %d", len(capabilities))
			}
			powerCap, ok := capabilities[0].(*PowerCapability)
			if !ok {
				t.Fatal("Expected PowerCapability")
			}
			if powerCap.State != tt.wantState {
				t.Errorf("Expected state %q, got %q", tt.wantState, powerCap.State)
			}

			// Capability → Tuya (round trip)
			code, value, err := dcm.GetTuyaCommandFromCapability(tt.category, &PowerCapability{State: tt.wantState})
			if err != nil {
				t.Fatalf("Capability→Tuya failed: %v", err)
			}
			if code != tt.tuyaCode {
				t.Errorf("Expected code %q, got %q", tt.tuyaCode, code)
			}
			if value != tt.tuyaBack {
				t.Errorf("Expected Tuya value %v, got %v", tt.tuyaBack, value)
			}
		})
	}
}

func TestWsdcgSensor_Bidirectional(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	t.Run("temperature sensor", func(t *testing.T) {
		tests := []struct {
			name     string
			tuyaVal  float64
			wantTemp float64
		}{
			{name: "normal 22.5C", tuyaVal: 225, wantTemp: 22.5},
			{name: "zero", tuyaVal: 0, wantTemp: 0},
			{name: "negative -5C", tuyaVal: -50, wantTemp: -5.0},
			{name: "hot 40C", tuyaVal: 400, wantTemp: 40.0},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Tuya → Capability
				status := []DeviceStatusChange{
					{Code: "va_temperature", Value: tt.tuyaVal},
				}
				capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("wsdcg", status)
				if err != nil {
					t.Fatalf("Tuya→Capability failed: %v", err)
				}
				tsCap := capabilities[0].(*TemperatureSensorCapability)
				if tsCap.Temperature != tt.wantTemp {
					t.Errorf("Expected %.1f°C, got %.1f°C", tt.wantTemp, tsCap.Temperature)
				}

				// Capability → Tuya round trip
				code, value, err := dcm.GetTuyaCommandFromCapability("wsdcg", &TemperatureSensorCapability{Temperature: tt.wantTemp})
				if err != nil {
					t.Fatalf("Capability→Tuya failed: %v", err)
				}
				if code != "va_temperature" {
					t.Errorf("Expected code 'va_temperature', got %s", code)
				}
				if value.(int) != int(tt.tuyaVal) {
					t.Errorf("Expected Tuya value %d, got %v", int(tt.tuyaVal), value)
				}
			})
		}
	})

	t.Run("humidity sensor", func(t *testing.T) {
		tests := []struct {
			name         string
			tuyaVal      float64
			wantHumidity int
		}{
			{name: "normal 65%", tuyaVal: 65, wantHumidity: 65},
			{name: "zero", tuyaVal: 0, wantHumidity: 0},
			{name: "max 100%", tuyaVal: 100, wantHumidity: 100},
			{name: "low 15%", tuyaVal: 15, wantHumidity: 15},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Tuya → Capability
				status := []DeviceStatusChange{
					{Code: "va_humidity", Value: tt.tuyaVal},
				}
				capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("wsdcg", status)
				if err != nil {
					t.Fatalf("Tuya→Capability failed: %v", err)
				}
				hsCap := capabilities[0].(*HumiditySensorCapability)
				if hsCap.Humidity != tt.wantHumidity {
					t.Errorf("Expected %d%%, got %d%%", tt.wantHumidity, hsCap.Humidity)
				}

				// Capability → Tuya round trip
				code, value, err := dcm.GetTuyaCommandFromCapability("wsdcg", &HumiditySensorCapability{Humidity: tt.wantHumidity})
				if err != nil {
					t.Fatalf("Capability→Tuya failed: %v", err)
				}
				if code != "va_humidity" {
					t.Errorf("Expected code 'va_humidity', got %s", code)
				}
				if value.(int) != tt.wantHumidity {
					t.Errorf("Expected Tuya value %d, got %v", tt.wantHumidity, value)
				}
			})
		}
	})

	t.Run("combined temp+humidity status", func(t *testing.T) {
		// Test that both capabilities are returned from a single status update
		status := []DeviceStatusChange{
			{Code: "va_temperature", Value: float64(225)},
			{Code: "va_humidity", Value: float64(55)},
		}
		capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("wsdcg", status)
		if err != nil {
			t.Fatalf("Failed to convert combined status: %v", err)
		}
		if len(capabilities) != 2 {
			t.Fatalf("Expected 2 capabilities, got %d", len(capabilities))
		}

		hasTemp := false
		hasHumidity := false
		for _, cap := range capabilities {
			switch cap.GetCapabilityType() {
			case CapabilityTypeTemperatureSensor:
				hasTemp = true
			case CapabilityTypeHumiditySensor:
				hasHumidity = true
			}
		}
		if !hasTemp {
			t.Error("Expected temperature sensor capability")
		}
		if !hasHumidity {
			t.Error("Expected humidity sensor capability")
		}
	})
}

func TestMcsContactSensor_Bidirectional(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	tests := []struct {
		name     string
		tuyaVal  interface{}
		wantOpen bool
		tuyaBack interface{}
	}{
		{name: "open (true)", tuyaVal: true, wantOpen: true, tuyaBack: true},
		{name: "closed (false)", tuyaVal: false, wantOpen: false, tuyaBack: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Tuya → Capability
			status := []DeviceStatusChange{
				{Code: "doorcontact_state", Value: tt.tuyaVal},
			}
			capabilities, err := dcm.GetCapabilitiesFromTuyaStatus("mcs", status)
			if err != nil {
				t.Fatalf("Tuya→Capability failed: %v", err)
			}
			if len(capabilities) != 1 {
				t.Fatalf("Expected 1 capability, got %d", len(capabilities))
			}
			csCap, ok := capabilities[0].(*ContactSensorCapability)
			if !ok {
				t.Fatal("Expected ContactSensorCapability")
			}
			if csCap.IsOpen != tt.wantOpen {
				t.Errorf("Expected IsOpen=%v, got %v", tt.wantOpen, csCap.IsOpen)
			}

			// Capability → Tuya round trip
			code, value, err := dcm.GetTuyaCommandFromCapability("mcs", &ContactSensorCapability{IsOpen: tt.wantOpen})
			if err != nil {
				t.Fatalf("Capability→Tuya failed: %v", err)
			}
			if code != "doorcontact_state" {
				t.Errorf("Expected code 'doorcontact_state', got %s", code)
			}
			if value != tt.tuyaBack {
				t.Errorf("Expected Tuya value %v, got %v", tt.tuyaBack, value)
			}
		})
	}
}

func TestLightCategories_Bidirectional(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	// Test power on/off for dc and fwd categories
	lightCategories := []struct {
		name     string
		category string
	}{
		{name: "dc string lights", category: "dc"},
		{name: "fwd ambient light", category: "fwd"},
	}

	for _, lc := range lightCategories {
		t.Run(lc.name+" power", func(t *testing.T) {
			tests := []struct {
				name      string
				tuyaValue interface{}
				wantState PowerState
				tuyaBack  interface{}
			}{
				{name: "on", tuyaValue: true, wantState: PowerStateOn, tuyaBack: true},
				{name: "off", tuyaValue: false, wantState: PowerStateOff, tuyaBack: false},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					status := []DeviceStatusChange{{Code: "switch_led", Value: tt.tuyaValue}}
					capabilities, err := dcm.GetCapabilitiesFromTuyaStatus(lc.category, status)
					if err != nil {
						t.Fatalf("Tuya→Capability failed: %v", err)
					}
					if len(capabilities) != 1 {
						t.Fatalf("Expected 1 capability, got %d", len(capabilities))
					}
					powerCap := capabilities[0].(*PowerCapability)
					if powerCap.State != tt.wantState {
						t.Errorf("Expected state %q, got %q", tt.wantState, powerCap.State)
					}
					code, value, err := dcm.GetTuyaCommandFromCapability(lc.category, &PowerCapability{State: tt.wantState})
					if err != nil {
						t.Fatalf("Capability→Tuya failed: %v", err)
					}
					if code != "switch_led" {
						t.Errorf("Expected code 'switch_led', got %s", code)
					}
					if value != tt.tuyaBack {
						t.Errorf("Expected Tuya value %v, got %v", tt.tuyaBack, value)
					}
				})
			}
		})

		t.Run(lc.name+" brightness", func(t *testing.T) {
			tests := []struct {
				name      string
				tuyaValue float64
				wantLevel int
			}{
				{name: "full", tuyaValue: 1000, wantLevel: 100},
				{name: "half", tuyaValue: 500, wantLevel: 50},
				{name: "off", tuyaValue: 0, wantLevel: 0},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					status := []DeviceStatusChange{{Code: "bright_value", Value: tt.tuyaValue}}
					capabilities, err := dcm.GetCapabilitiesFromTuyaStatus(lc.category, status)
					if err != nil {
						t.Fatalf("Tuya→Capability failed: %v", err)
					}
					brightCap := capabilities[0].(*BrightnessCapability)
					if brightCap.Level != tt.wantLevel {
						t.Errorf("Expected level %d, got %d", tt.wantLevel, brightCap.Level)
					}
				})
			}
		})

		t.Run(lc.name+" color temperature", func(t *testing.T) {
			tests := []struct {
				name       string
				tuyaValue  float64
				wantMireds int
			}{
				{name: "warm (0)", tuyaValue: 0, wantMireds: 153},
				{name: "cool (1000)", tuyaValue: 1000, wantMireds: 500},
				{name: "mid (500)", tuyaValue: 500, wantMireds: 326},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					status := []DeviceStatusChange{{Code: "temp_value", Value: tt.tuyaValue}}
					capabilities, err := dcm.GetCapabilitiesFromTuyaStatus(lc.category, status)
					if err != nil {
						t.Fatalf("Tuya→Capability failed: %v", err)
					}
					ctCap := capabilities[0].(*ColorTemperatureCapability)
					if ctCap.Mireds != tt.wantMireds {
						t.Errorf("Expected %d mireds, got %d", tt.wantMireds, ctCap.Mireds)
					}
				})
			}
		})
	}
}

func TestFanCategories_Bidirectional(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	fanCategories := []struct {
		name     string
		category string
	}{
		{name: "fs fan", category: "fs"},
		{name: "fsd ceiling fan", category: "fsd"},
	}

	for _, fc := range fanCategories {
		t.Run(fc.name+" power", func(t *testing.T) {
			tests := []struct {
				name      string
				tuyaValue interface{}
				wantState PowerState
				tuyaBack  interface{}
			}{
				{name: "on", tuyaValue: true, wantState: PowerStateOn, tuyaBack: true},
				{name: "off", tuyaValue: false, wantState: PowerStateOff, tuyaBack: false},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					status := []DeviceStatusChange{{Code: "switch_fan", Value: tt.tuyaValue}}
					capabilities, err := dcm.GetCapabilitiesFromTuyaStatus(fc.category, status)
					if err != nil {
						t.Fatalf("Tuya→Capability failed: %v", err)
					}
					powerCap := capabilities[0].(*PowerCapability)
					if powerCap.State != tt.wantState {
						t.Errorf("Expected state %q, got %q", tt.wantState, powerCap.State)
					}
					code, value, err := dcm.GetTuyaCommandFromCapability(fc.category, &PowerCapability{State: tt.wantState})
					if err != nil {
						t.Fatalf("Capability→Tuya failed: %v", err)
					}
					if code != "switch_fan" {
						t.Errorf("Expected code 'switch_fan', got %s", code)
					}
					if value != tt.tuyaBack {
						t.Errorf("Expected Tuya value %v, got %v", tt.tuyaBack, value)
					}
				})
			}
		})

		t.Run(fc.name+" fan speed", func(t *testing.T) {
			tests := []struct {
				name      string
				tuyaValue float64
				wantSpeed int
				tuyaBack  int
			}{
				{name: "full speed", tuyaValue: 100, wantSpeed: 100, tuyaBack: 100},
				{name: "half speed", tuyaValue: 50, wantSpeed: 50, tuyaBack: 50},
				{name: "off", tuyaValue: 0, wantSpeed: 0, tuyaBack: 0},
				{name: "quarter speed", tuyaValue: 25, wantSpeed: 25, tuyaBack: 25},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					// Tuya → Capability
					status := []DeviceStatusChange{{Code: "fan_speed_percent", Value: tt.tuyaValue}}
					capabilities, err := dcm.GetCapabilitiesFromTuyaStatus(fc.category, status)
					if err != nil {
						t.Fatalf("Tuya→Capability failed: %v", err)
					}
					fanCap := capabilities[0].(*FanSpeedCapability)
					if fanCap.Speed != tt.wantSpeed {
						t.Errorf("Expected speed %d%%, got %d%%", tt.wantSpeed, fanCap.Speed)
					}

					// Capability → Tuya round trip
					code, value, err := dcm.GetTuyaCommandFromCapability(fc.category, &FanSpeedCapability{Speed: tt.wantSpeed})
					if err != nil {
						t.Fatalf("Capability→Tuya failed: %v", err)
					}
					if code != "fan_speed_percent" {
						t.Errorf("Expected code 'fan_speed_percent', got %s", code)
					}
					if value.(int) != tt.tuyaBack {
						t.Errorf("Expected Tuya value %d, got %v", tt.tuyaBack, value)
					}
				})
			}
		})
	}
}

func TestCoverCategories_Bidirectional(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	coverCategories := []struct {
		name     string
		category string
	}{
		{name: "cl curtain", category: "cl"},
		{name: "clkg curtain switch", category: "clkg"},
	}

	for _, cc := range coverCategories {
		t.Run(cc.name+" power", func(t *testing.T) {
			tests := []struct {
				name      string
				tuyaValue interface{}
				wantState PowerState
				tuyaBack  interface{}
			}{
				{name: "on (open)", tuyaValue: true, wantState: PowerStateOn, tuyaBack: true},
				{name: "off (close)", tuyaValue: false, wantState: PowerStateOff, tuyaBack: false},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					status := []DeviceStatusChange{{Code: "control", Value: tt.tuyaValue}}
					capabilities, err := dcm.GetCapabilitiesFromTuyaStatus(cc.category, status)
					if err != nil {
						t.Fatalf("Tuya→Capability failed: %v", err)
					}
					powerCap := capabilities[0].(*PowerCapability)
					if powerCap.State != tt.wantState {
						t.Errorf("Expected state %q, got %q", tt.wantState, powerCap.State)
					}
					code, value, err := dcm.GetTuyaCommandFromCapability(cc.category, &PowerCapability{State: tt.wantState})
					if err != nil {
						t.Fatalf("Capability→Tuya failed: %v", err)
					}
					if code != "control" {
						t.Errorf("Expected code 'control', got %s", code)
					}
					if value != tt.tuyaBack {
						t.Errorf("Expected Tuya value %v, got %v", tt.tuyaBack, value)
					}
				})
			}
		})

		t.Run(cc.name+" window covering position", func(t *testing.T) {
			tests := []struct {
				name         string
				tuyaValue    float64
				wantPosition int
				tuyaBack     int
			}{
				{name: "fully open", tuyaValue: 100, wantPosition: 100, tuyaBack: 100},
				{name: "half open", tuyaValue: 50, wantPosition: 50, tuyaBack: 50},
				{name: "closed", tuyaValue: 0, wantPosition: 0, tuyaBack: 0},
				{name: "quarter open", tuyaValue: 25, wantPosition: 25, tuyaBack: 25},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					// Tuya → Capability
					status := []DeviceStatusChange{{Code: "percent_control", Value: tt.tuyaValue}}
					capabilities, err := dcm.GetCapabilitiesFromTuyaStatus(cc.category, status)
					if err != nil {
						t.Fatalf("Tuya→Capability failed: %v", err)
					}
					wcCap := capabilities[0].(*WindowCoveringCapability)
					if wcCap.Position != tt.wantPosition {
						t.Errorf("Expected position %d%%, got %d%%", tt.wantPosition, wcCap.Position)
					}

					// Capability → Tuya round trip
					code, value, err := dcm.GetTuyaCommandFromCapability(cc.category, &WindowCoveringCapability{Position: tt.wantPosition})
					if err != nil {
						t.Fatalf("Capability→Tuya failed: %v", err)
					}
					if code != "percent_control" {
						t.Errorf("Expected code 'percent_control', got %s", code)
					}
					if value.(int) != tt.tuyaBack {
						t.Errorf("Expected Tuya value %d, got %v", tt.tuyaBack, value)
					}
				})
			}
		})
	}
}

func TestCameraCapability(t *testing.T) {
	cam := &CameraCapability{}

	// Test string input
	err := cam.SetValue("idle")
	if err != nil {
		t.Fatalf("Failed to set string value: %v", err)
	}
	if cam.StreamingState != StreamingStateIdle {
		t.Errorf("Expected state 'idle', got %s", cam.StreamingState)
	}

	err = cam.SetValue("streaming")
	if err != nil {
		t.Fatalf("Failed to set string value: %v", err)
	}
	if cam.StreamingState != StreamingStateStreaming {
		t.Errorf("Expected state 'streaming', got %s", cam.StreamingState)
	}

	// Test StreamingState input
	err = cam.SetValue(StreamingStateIdle)
	if err != nil {
		t.Fatalf("Failed to set StreamingState value: %v", err)
	}
	if cam.StreamingState != StreamingStateIdle {
		t.Errorf("Expected state 'idle', got %s", cam.StreamingState)
	}

	// Test invalid input
	err = cam.SetValue("invalid")
	if err == nil {
		t.Error("Expected error for invalid streaming state")
	}

	// Test interface methods
	if cam.GetCapabilityType() != CapabilityTypeCamera {
		t.Errorf("Expected capability type 'camera', got %s", cam.GetCapabilityType())
	}
	if cam.GetValue() != StreamingStateIdle {
		t.Errorf("Expected value 'idle', got %v", cam.GetValue())
	}
}

func TestRTCSessionCapability(t *testing.T) {
	rtc := &RTCSessionCapability{}

	if rtc.GetCapabilityType() != CapabilityTypeRTCSession {
		t.Errorf("Expected capability type 'rtc-session', got %s", rtc.GetCapabilityType())
	}
	if rtc.GetValue() != nil {
		t.Errorf("Expected nil value, got %v", rtc.GetValue())
	}

	// SetValue should be a no-op
	err := rtc.SetValue("anything")
	if err != nil {
		t.Fatalf("SetValue should not return error: %v", err)
	}
}

func TestCameraCategories_CapabilityQueries(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	cameraCategories := []string{"sp", "ipc"}
	for _, category := range cameraCategories {
		t.Run(category, func(t *testing.T) {
			if !dcm.HasPowerCapability(category) {
				t.Errorf("Expected %s category to have power capability", category)
			}
			if !dcm.HasCameraCapability(category) {
				t.Errorf("Expected %s category to have camera capability", category)
			}
			if !dcm.HasRTCSessionCapability(category) {
				t.Errorf("Expected %s category to have rtc-session capability", category)
			}
			// Cameras should not have unrelated capabilities
			if dcm.HasBrightnessCapability(category) {
				t.Errorf("Expected %s category to NOT have brightness capability", category)
			}
			if dcm.HasLockCapability(category) {
				t.Errorf("Expected %s category to NOT have lock capability", category)
			}
		})
	}
}

func TestCameraCategories_Bidirectional(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	cameraCategories := []struct {
		name     string
		category string
	}{
		{name: "sp smart camera", category: "sp"},
		{name: "ipc IP camera", category: "ipc"},
	}

	for _, cc := range cameraCategories {
		t.Run(cc.name, func(t *testing.T) {
			// Test power capability (basic_indicator)
			t.Run("power", func(t *testing.T) {
				status := []DeviceStatusChange{{Code: "basic_indicator", Value: true}}
				capabilities, err := dcm.GetCapabilitiesFromTuyaStatus(cc.category, status)
				if err != nil {
					t.Fatalf("Tuya→Capability failed: %v", err)
				}
				if len(capabilities) != 1 {
					t.Fatalf("Expected 1 capability, got %d", len(capabilities))
				}
				powerCap, ok := capabilities[0].(*PowerCapability)
				if !ok {
					t.Fatal("Expected PowerCapability")
				}
				if powerCap.State != PowerStateOn {
					t.Errorf("Expected state 'on', got %s", powerCap.State)
				}

				// Round trip
				code, value, err := dcm.GetTuyaCommandFromCapability(cc.category, &PowerCapability{State: PowerStateOn})
				if err != nil {
					t.Fatalf("Capability→Tuya failed: %v", err)
				}
				if code != "basic_indicator" {
					t.Errorf("Expected code 'basic_indicator', got %s", code)
				}
				if value != true {
					t.Errorf("Expected Tuya value true, got %v", value)
				}
			})

			// Test camera capability
			t.Run("camera", func(t *testing.T) {
				status := []DeviceStatusChange{{Code: "camera", Value: "idle"}}
				capabilities, err := dcm.GetCapabilitiesFromTuyaStatus(cc.category, status)
				if err != nil {
					t.Fatalf("Tuya→Capability failed: %v", err)
				}
				if len(capabilities) != 1 {
					t.Fatalf("Expected 1 capability, got %d", len(capabilities))
				}
				camCap, ok := capabilities[0].(*CameraCapability)
				if !ok {
					t.Fatal("Expected CameraCapability")
				}
				if camCap.StreamingState != StreamingStateIdle {
					t.Errorf("Expected state 'idle', got %s", camCap.StreamingState)
				}
			})
		})
	}
}

func TestLockCategories_Bidirectional(t *testing.T) {
	dcm := NewDeviceCapabilityMap()

	lockCategories := []struct {
		name     string
		category string
	}{
		{name: "ms lock", category: "ms"},
		{name: "jtmspro smart lock", category: "jtmspro"},
	}

	for _, lc := range lockCategories {
		t.Run(lc.name, func(t *testing.T) {
			tests := []struct {
				name      string
				tuyaValue interface{}
				wantState LockState
				tuyaBack  interface{}
			}{
				{name: "locked", tuyaValue: true, wantState: LockStateLocked, tuyaBack: true},
				{name: "unlocked", tuyaValue: false, wantState: LockStateUnlocked, tuyaBack: false},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					// Tuya → Capability
					status := []DeviceStatusChange{{Code: "closed_opened", Value: tt.tuyaValue}}
					capabilities, err := dcm.GetCapabilitiesFromTuyaStatus(lc.category, status)
					if err != nil {
						t.Fatalf("Tuya→Capability failed: %v", err)
					}
					if len(capabilities) != 1 {
						t.Fatalf("Expected 1 capability, got %d", len(capabilities))
					}
					lockCap, ok := capabilities[0].(*LockCapability)
					if !ok {
						t.Fatal("Expected LockCapability")
					}
					if lockCap.State != tt.wantState {
						t.Errorf("Expected state %q, got %q", tt.wantState, lockCap.State)
					}

					// Capability → Tuya round trip
					code, value, err := dcm.GetTuyaCommandFromCapability(lc.category, &LockCapability{State: tt.wantState})
					if err != nil {
						t.Fatalf("Capability→Tuya failed: %v", err)
					}
					if code != "closed_opened" {
						t.Errorf("Expected code 'closed_opened', got %s", code)
					}
					if value != tt.tuyaBack {
						t.Errorf("Expected Tuya value %v, got %v", tt.tuyaBack, value)
					}
				})
			}
		})
	}
}
