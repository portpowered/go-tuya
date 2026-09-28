package tuya

import (
	"fmt"
	"strconv"
	"time"
)

// Capability type constants
const (
	CapabilityTypePower             = "power"
	CapabilityTypeBrightness        = "brightness"
	CapabilityTypeColor             = "color"
	CapabilityTypeColorTemperature  = "color-temperature"
	CapabilityTypeTemperatureSensor = "temperature-sensor"
	CapabilityTypeHumiditySensor    = "humidity-sensor"
	CapabilityTypeFanSpeed          = "fan-speed"
	CapabilityTypeLock              = "lock"
	CapabilityTypeContactSensor     = "contact-sensor"
	CapabilityTypeWindowCovering    = "window-covering"
	CapabilityTypeCamera            = "camera"
	CapabilityTypeRTCSession        = "rtc-session"
)

// Capability represents a standardized device capability interface
type Capability interface {
	GetCapabilityType() string
	GetValue() interface{}
	SetValue(value interface{}) error
}

// PowerState represents the power state of a device
type PowerState string

// Power state constants
const (
	// PowerStateOn indicates the device is powered on
	PowerStateOn PowerState = "on"
	// PowerStateOff indicates the device is powered off
	PowerStateOff PowerState = "off"
)

// PowerCapability represents power on/off capability
type PowerCapability struct {
	State PowerState `json:"state"` // "on" or "off"
}

// GetCapabilityType returns the capability type for power
func (p *PowerCapability) GetCapabilityType() string {
	return CapabilityTypePower
}

// GetValue returns the current power state
func (p *PowerCapability) GetValue() interface{} {
	return p.State
}

// SetValue sets the power state from various input types
func (p *PowerCapability) SetValue(value interface{}) error {
	switch v := value.(type) {
	case string:
		if v == string(PowerStateOn) || v == string(PowerStateOff) {
			p.State = PowerState(v)
			return nil
		}
	case bool:
		if v {
			p.State = PowerStateOn
		} else {
			p.State = PowerStateOff
		}
		return nil
	}
	return fmt.Errorf("invalid power state value: %v", value)
}

// BrightnessCapability represents brightness adjustment capability
type BrightnessCapability struct {
	Level int `json:"level"` // 0-100 percentage
}

// GetCapabilityType returns the capability type for brightness
func (b *BrightnessCapability) GetCapabilityType() string {
	return CapabilityTypeBrightness
}

// GetValue returns the current brightness level
func (b *BrightnessCapability) GetValue() interface{} {
	return b.Level
}

// SetValue sets the brightness level from various input types
func (b *BrightnessCapability) SetValue(value interface{}) error {
	switch v := value.(type) {
	case int:
		if v >= 0 && v <= 100 {
			b.Level = v
			return nil
		}
	case float64:
		level := int(v)
		if level >= 0 && level <= 100 {
			b.Level = level
			return nil
		}
	case string:
		if level, err := strconv.Atoi(v); err == nil && level >= 0 && level <= 100 {
			b.Level = level
			return nil
		}
	}
	return fmt.Errorf("invalid brightness level: %v (must be 0-100)", value)
}

// ColorCapability represents color control capability (HSV)
type ColorCapability struct {
	Hue        int `json:"hue"`        // 0-360 degrees
	Saturation int `json:"saturation"` // 0-100 percentage
	Value      int `json:"value"`      // 0-100 percentage (brightness in HSV)
}

// GetCapabilityType returns the capability type for color
func (c *ColorCapability) GetCapabilityType() string {
	return CapabilityTypeColor
}

// GetValue returns the current color values as an HSV map
func (c *ColorCapability) GetValue() interface{} {
	return map[string]int{
		"hue":        c.Hue,
		"saturation": c.Saturation,
		"value":      c.Value,
	}
}

// SetValue sets the color values from various input types
func (c *ColorCapability) SetValue(value interface{}) error {
	switch v := value.(type) {
	case map[string]interface{}:
		if hue, ok := v["hue"].(float64); ok {
			c.Hue = int(hue) % 360
		}
		if sat, ok := v["saturation"].(float64); ok && sat >= 0 && sat <= 100 {
			c.Saturation = int(sat)
		}
		if val, ok := v["value"].(float64); ok && val >= 0 && val <= 100 {
			c.Value = int(val)
		}
		return nil
	case map[string]int:
		if hue, ok := v["hue"]; ok {
			c.Hue = hue % 360
		}
		if sat, ok := v["saturation"]; ok && sat >= 0 && sat <= 100 {
			c.Saturation = sat
		}
		if val, ok := v["value"]; ok && val >= 0 && val <= 100 {
			c.Value = val
		}
		return nil
	}
	return fmt.Errorf("invalid color value: %v", value)
}

// ColorTemperatureCapability represents color temperature control capability
type ColorTemperatureCapability struct {
	Mireds int `json:"mireds"` // Color temperature in mireds (153-500, i.e. 2000-6500K)
}

// GetCapabilityType returns the capability type for color temperature
func (ct *ColorTemperatureCapability) GetCapabilityType() string {
	return CapabilityTypeColorTemperature
}

// GetValue returns the current color temperature in mireds
func (ct *ColorTemperatureCapability) GetValue() interface{} {
	return ct.Mireds
}

// SetValue sets the color temperature from various input types
func (ct *ColorTemperatureCapability) SetValue(value interface{}) error {
	switch v := value.(type) {
	case int:
		if v >= 153 && v <= 500 {
			ct.Mireds = v
			return nil
		}
	case float64:
		mireds := int(v)
		if mireds >= 153 && mireds <= 500 {
			ct.Mireds = mireds
			return nil
		}
	case string:
		if mireds, err := strconv.Atoi(v); err == nil && mireds >= 153 && mireds <= 500 {
			ct.Mireds = mireds
			return nil
		}
	}
	return fmt.Errorf("invalid color temperature: %v (must be 153-500 mireds)", value)
}

// TemperatureSensorCapability represents a temperature sensor reading
type TemperatureSensorCapability struct {
	Temperature float64 `json:"temperature"` // Temperature in Celsius
}

// GetCapabilityType returns the capability type for temperature sensor
func (ts *TemperatureSensorCapability) GetCapabilityType() string {
	return CapabilityTypeTemperatureSensor
}

// GetValue returns the current temperature in Celsius
func (ts *TemperatureSensorCapability) GetValue() interface{} {
	return ts.Temperature
}

// SetValue sets the temperature from various input types
func (ts *TemperatureSensorCapability) SetValue(value interface{}) error {
	switch v := value.(type) {
	case float64:
		ts.Temperature = v
		return nil
	case int:
		ts.Temperature = float64(v)
		return nil
	case string:
		if temp, err := strconv.ParseFloat(v, 64); err == nil {
			ts.Temperature = temp
			return nil
		}
	}
	return fmt.Errorf("invalid temperature value: %v", value)
}

// HumiditySensorCapability represents a humidity sensor reading
type HumiditySensorCapability struct {
	Humidity int `json:"humidity"` // Humidity percentage (0-100)
}

// GetCapabilityType returns the capability type for humidity sensor
func (hs *HumiditySensorCapability) GetCapabilityType() string {
	return CapabilityTypeHumiditySensor
}

// GetValue returns the current humidity percentage
func (hs *HumiditySensorCapability) GetValue() interface{} {
	return hs.Humidity
}

// SetValue sets the humidity from various input types
func (hs *HumiditySensorCapability) SetValue(value interface{}) error {
	switch v := value.(type) {
	case int:
		if v >= 0 && v <= 100 {
			hs.Humidity = v
			return nil
		}
	case float64:
		level := int(v)
		if level >= 0 && level <= 100 {
			hs.Humidity = level
			return nil
		}
	case string:
		if level, err := strconv.Atoi(v); err == nil && level >= 0 && level <= 100 {
			hs.Humidity = level
			return nil
		}
	}
	return fmt.Errorf("invalid humidity value: %v (must be 0-100)", value)
}

// FanSpeedCapability represents fan speed control capability
type FanSpeedCapability struct {
	Speed int `json:"speed"` // Fan speed percentage (0-100)
}

// GetCapabilityType returns the capability type for fan speed
func (fs *FanSpeedCapability) GetCapabilityType() string {
	return CapabilityTypeFanSpeed
}

// GetValue returns the current fan speed percentage
func (fs *FanSpeedCapability) GetValue() interface{} {
	return fs.Speed
}

// SetValue sets the fan speed from various input types
func (fs *FanSpeedCapability) SetValue(value interface{}) error {
	switch v := value.(type) {
	case int:
		if v >= 0 && v <= 100 {
			fs.Speed = v
			return nil
		}
	case float64:
		speed := int(v)
		if speed >= 0 && speed <= 100 {
			fs.Speed = speed
			return nil
		}
	case string:
		if speed, err := strconv.Atoi(v); err == nil && speed >= 0 && speed <= 100 {
			fs.Speed = speed
			return nil
		}
	}
	return fmt.Errorf("invalid fan speed value: %v (must be 0-100)", value)
}

// LockState represents the lock state of a device
type LockState string

// Lock state constants
const (
	LockStateLocked   LockState = "locked"
	LockStateUnlocked LockState = "unlocked"
)

// LockCapability represents lock/unlock capability
type LockCapability struct {
	State LockState `json:"state"` // "locked" or "unlocked"
}

// GetCapabilityType returns the capability type for lock
func (l *LockCapability) GetCapabilityType() string {
	return CapabilityTypeLock
}

// GetValue returns the current lock state
func (l *LockCapability) GetValue() interface{} {
	return l.State
}

// SetValue sets the lock state from various input types
func (l *LockCapability) SetValue(value interface{}) error {
	switch v := value.(type) {
	case string:
		if v == string(LockStateLocked) || v == string(LockStateUnlocked) {
			l.State = LockState(v)
			return nil
		}
	case bool:
		// true = locked (closed), false = unlocked (opened)
		if v {
			l.State = LockStateLocked
		} else {
			l.State = LockStateUnlocked
		}
		return nil
	}
	return fmt.Errorf("invalid lock state value: %v", value)
}

// ContactSensorCapability represents a contact (door/window) sensor reading
type ContactSensorCapability struct {
	IsOpen bool `json:"isOpen"` // true if the contact is open (door/window open)
}

// GetCapabilityType returns the capability type for contact sensor
func (cs *ContactSensorCapability) GetCapabilityType() string {
	return CapabilityTypeContactSensor
}

// GetValue returns whether the contact is open
func (cs *ContactSensorCapability) GetValue() interface{} {
	return cs.IsOpen
}

// SetValue sets the contact sensor state from various input types
func (cs *ContactSensorCapability) SetValue(value interface{}) error {
	switch v := value.(type) {
	case bool:
		cs.IsOpen = v
		return nil
	case string:
		switch v {
		case "true", "open":
			cs.IsOpen = true
			return nil
		case "false", "closed":
			cs.IsOpen = false
			return nil
		}
	}
	return fmt.Errorf("invalid contact sensor value: %v", value)
}

// WindowCoveringCapability represents window covering (curtain/blind) position
type WindowCoveringCapability struct {
	Position int `json:"position"` // Position percentage (0-100, where 0=closed, 100=fully open)
}

// GetCapabilityType returns the capability type for window covering
func (wc *WindowCoveringCapability) GetCapabilityType() string {
	return CapabilityTypeWindowCovering
}

// GetValue returns the current position percentage
func (wc *WindowCoveringCapability) GetValue() interface{} {
	return wc.Position
}

// SetValue sets the window covering position from various input types
func (wc *WindowCoveringCapability) SetValue(value interface{}) error {
	switch v := value.(type) {
	case int:
		if v >= 0 && v <= 100 {
			wc.Position = v
			return nil
		}
	case float64:
		pos := int(v)
		if pos >= 0 && pos <= 100 {
			wc.Position = pos
			return nil
		}
	case string:
		if pos, err := strconv.Atoi(v); err == nil && pos >= 0 && pos <= 100 {
			wc.Position = pos
			return nil
		}
	}
	return fmt.Errorf("invalid window covering position: %v (must be 0-100)", value)
}

// StreamingState represents the streaming state of a camera
type StreamingState string

// Streaming state constants for camera capability.
const (
	// StreamingStateIdle indicates the camera is not actively streaming.
	StreamingStateIdle StreamingState = "idle"
	// StreamingStateStreaming indicates the camera is actively streaming.
	StreamingStateStreaming StreamingState = "streaming"
)

// CameraCapability represents camera functionality
type CameraCapability struct {
	StreamingState StreamingState `json:"streamingState"`
}

// GetCapabilityType returns the capability type for camera
func (c *CameraCapability) GetCapabilityType() string {
	return CapabilityTypeCamera
}

// GetValue returns the current streaming state
func (c *CameraCapability) GetValue() interface{} {
	return c.StreamingState
}

// SetValue sets the camera streaming state
func (c *CameraCapability) SetValue(value interface{}) error {
	switch v := value.(type) {
	case string:
		switch StreamingState(v) {
		case StreamingStateIdle, StreamingStateStreaming:
			c.StreamingState = StreamingState(v)
			return nil
		}
	case StreamingState:
		c.StreamingState = v
		return nil
	}
	return fmt.Errorf("invalid streaming state: %v (must be 'idle' or 'streaming')", value)
}

// RTCSessionCapability represents WebRTC session capability
type RTCSessionCapability struct{}

// GetCapabilityType returns the capability type for RTC session
func (r *RTCSessionCapability) GetCapabilityType() string {
	return CapabilityTypeRTCSession
}

// GetValue returns nil as RTC session has no persistent value
func (r *RTCSessionCapability) GetValue() interface{} {
	return nil
}

// SetValue is a no-op for RTC session capability
func (r *RTCSessionCapability) SetValue(_ interface{}) error {
	return nil
}

// CapabilityMapping defines how to map Tuya codes to capabilities
type CapabilityMapping struct {
	CapabilityType   string
	TuyaToCapability func(interface{}) (Capability, error)
	CapabilityToTuya func(Capability) (string, interface{}, error)
}

// DeviceCapabilityMap maps device categories and codes to capabilities
type DeviceCapabilityMap struct {
	mappings map[string]map[string]CapabilityMapping
}

// NewDeviceCapabilityMap creates a new capability mapping system
func NewDeviceCapabilityMap() *DeviceCapabilityMap {
	dcm := &DeviceCapabilityMap{
		mappings: make(map[string]map[string]CapabilityMapping),
	}
	dcm.initializeStandardMappings()
	return dcm
}

// Helper functions for common mappings to reduce duplication
func createPowerMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypePower,
		TuyaToCapability: func(value interface{}) (Capability, error) {
			capability := &PowerCapability{}
			err := capability.SetValue(value)
			return capability, err
		},
		CapabilityToTuya: func(capability Capability) (string, interface{}, error) {
			if power, ok := capability.(*PowerCapability); ok {
				return tuyaCode, power.State == PowerStateOn, nil
			}
			return "", nil, fmt.Errorf("invalid capability type for power")
		},
	}
}

func createBrightnessMapping(tuyaCode string, maxValue float64) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeBrightness,
		TuyaToCapability: func(value interface{}) (Capability, error) {
			capability := &BrightnessCapability{}
			if val, ok := value.(float64); ok {
				var percentage int
				if maxValue == 1000 {
					percentage = int(val / 1000 * 100)
				} else {
					// For tgq category: range 10-1000
					percentage = int((val - 10) / 990 * 100)
					if percentage < 0 {
						percentage = 0
					}
					if percentage > 100 {
						percentage = 100
					}
				}
				err := capability.SetValue(percentage)
				return capability, err
			}
			return capability, fmt.Errorf("invalid brightness value: %v", value)
		},
		CapabilityToTuya: func(capability Capability) (string, interface{}, error) {
			if brightness, ok := capability.(*BrightnessCapability); ok {
				var tuyaValue int
				if maxValue == 1000 {
					tuyaValue = int(float64(brightness.Level) / 100 * 1000)
				} else {
					// For tgq category: range 10-1000
					tuyaValue = int(float64(brightness.Level)/100*990 + 10)
				}
				return tuyaCode, tuyaValue, nil
			}
			return "", nil, fmt.Errorf("invalid capability type for brightness")
		},
	}
}

func createColorMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeColor,
		TuyaToCapability: func(value interface{}) (Capability, error) {
			capability := &ColorCapability{}
			if colorStr, ok := value.(string); ok {
				if len(colorStr) >= 12 {
					hueHex := colorStr[0:4]
					satHex := colorStr[4:8]
					valHex := colorStr[8:12]

					if hue, err := strconv.ParseInt(hueHex, 16, 32); err == nil {
						capability.Hue = int(hue)
					}
					if sat, err := strconv.ParseInt(satHex, 16, 32); err == nil {
						capability.Saturation = int(sat / 1000 * 100)
					}
					if val, err := strconv.ParseInt(valHex, 16, 32); err == nil {
						capability.Value = int(val / 1000 * 100)
					}
				}
			}
			return capability, nil
		},
		CapabilityToTuya: func(capability Capability) (string, interface{}, error) {
			if color, ok := capability.(*ColorCapability); ok {
				hue := fmt.Sprintf("%04x", color.Hue)
				sat := fmt.Sprintf("%04x", int(float64(color.Saturation)/100*1000))
				val := fmt.Sprintf("%04x", int(float64(color.Value)/100*1000))
				colorData := hue + sat + val
				return tuyaCode, colorData, nil
			}
			return "", nil, fmt.Errorf("invalid capability type for color")
		},
	}
}

func createColorTemperatureMapping(tuyaCode string, tuyaMin, tuyaMax int) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeColorTemperature,
		TuyaToCapability: func(value interface{}) (Capability, error) {
			capability := &ColorTemperatureCapability{}
			if val, ok := value.(float64); ok {
				// Convert Tuya range (tuyaMin-tuyaMax) to mireds (153-500)
				normalized := (val - float64(tuyaMin)) / float64(tuyaMax-tuyaMin)
				mireds := int(153 + normalized*347) // 347 = 500-153
				if mireds < 153 {
					mireds = 153
				}
				if mireds > 500 {
					mireds = 500
				}
				err := capability.SetValue(mireds)
				return capability, err
			}
			return capability, fmt.Errorf("invalid color temperature value: %v", value)
		},
		CapabilityToTuya: func(capability Capability) (string, interface{}, error) {
			if ct, ok := capability.(*ColorTemperatureCapability); ok {
				// Convert mireds (153-500) to Tuya range (tuyaMin-tuyaMax)
				normalized := float64(ct.Mireds-153) / 347.0
				tuyaValue := int(float64(tuyaMin) + normalized*float64(tuyaMax-tuyaMin))
				return tuyaCode, tuyaValue, nil
			}
			return "", nil, fmt.Errorf("invalid capability type for color temperature")
		},
	}
}

func createTemperatureSensorMapping(tuyaCode string, scaleDivisor float64) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeTemperatureSensor,
		TuyaToCapability: func(value interface{}) (Capability, error) {
			capability := &TemperatureSensorCapability{}
			if val, ok := value.(float64); ok {
				temp := val / scaleDivisor
				err := capability.SetValue(temp)
				return capability, err
			}
			return capability, fmt.Errorf("invalid temperature sensor value: %v", value)
		},
		CapabilityToTuya: func(capability Capability) (string, interface{}, error) {
			if ts, ok := capability.(*TemperatureSensorCapability); ok {
				tuyaValue := int(ts.Temperature * scaleDivisor)
				return tuyaCode, tuyaValue, nil
			}
			return "", nil, fmt.Errorf("invalid capability type for temperature sensor")
		},
	}
}

func createHumiditySensorMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeHumiditySensor,
		TuyaToCapability: func(value interface{}) (Capability, error) {
			capability := &HumiditySensorCapability{}
			if val, ok := value.(float64); ok {
				err := capability.SetValue(int(val))
				return capability, err
			}
			return capability, fmt.Errorf("invalid humidity sensor value: %v", value)
		},
		CapabilityToTuya: func(capability Capability) (string, interface{}, error) {
			if hs, ok := capability.(*HumiditySensorCapability); ok {
				return tuyaCode, hs.Humidity, nil
			}
			return "", nil, fmt.Errorf("invalid capability type for humidity sensor")
		},
	}
}

func createFanSpeedMapping(tuyaCode string, maxSpeedLevels int) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeFanSpeed,
		TuyaToCapability: func(value interface{}) (Capability, error) {
			capability := &FanSpeedCapability{}
			if val, ok := value.(float64); ok {
				// Convert discrete speed levels (1-maxSpeedLevels) to percentage (0-100)
				percentage := int(val / float64(maxSpeedLevels) * 100)
				if percentage < 0 {
					percentage = 0
				}
				if percentage > 100 {
					percentage = 100
				}
				err := capability.SetValue(percentage)
				return capability, err
			}
			return capability, fmt.Errorf("invalid fan speed value: %v", value)
		},
		CapabilityToTuya: func(capability Capability) (string, interface{}, error) {
			if fs, ok := capability.(*FanSpeedCapability); ok {
				// Convert percentage (0-100) to discrete speed levels (0-maxSpeedLevels)
				tuyaValue := int(float64(fs.Speed) / 100 * float64(maxSpeedLevels))
				if tuyaValue > maxSpeedLevels {
					tuyaValue = maxSpeedLevels
				}
				return tuyaCode, tuyaValue, nil
			}
			return "", nil, fmt.Errorf("invalid capability type for fan speed")
		},
	}
}

func createLockMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeLock,
		TuyaToCapability: func(value interface{}) (Capability, error) {
			capability := &LockCapability{}
			// Tuya locks use boolean: true = locked (closed), false = unlocked (opened)
			err := capability.SetValue(value)
			return capability, err
		},
		CapabilityToTuya: func(capability Capability) (string, interface{}, error) {
			if lock, ok := capability.(*LockCapability); ok {
				return tuyaCode, lock.State == LockStateLocked, nil
			}
			return "", nil, fmt.Errorf("invalid capability type for lock")
		},
	}
}

func createContactSensorMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeContactSensor,
		TuyaToCapability: func(value interface{}) (Capability, error) {
			capability := &ContactSensorCapability{}
			// Tuya contact sensors: true = open, false = closed
			err := capability.SetValue(value)
			return capability, err
		},
		CapabilityToTuya: func(capability Capability) (string, interface{}, error) {
			if cs, ok := capability.(*ContactSensorCapability); ok {
				return tuyaCode, cs.IsOpen, nil
			}
			return "", nil, fmt.Errorf("invalid capability type for contact sensor")
		},
	}
}

func createWindowCoveringMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeWindowCovering,
		TuyaToCapability: func(value interface{}) (Capability, error) {
			capability := &WindowCoveringCapability{}
			if val, ok := value.(float64); ok {
				err := capability.SetValue(int(val))
				return capability, err
			}
			return capability, fmt.Errorf("invalid window covering value: %v", value)
		},
		CapabilityToTuya: func(capability Capability) (string, interface{}, error) {
			if wc, ok := capability.(*WindowCoveringCapability); ok {
				return tuyaCode, wc.Position, nil
			}
			return "", nil, fmt.Errorf("invalid capability type for window covering")
		},
	}
}

// createCameraMapping creates a mapping for camera capability (marker mapping for device discovery)
func createCameraMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeCamera,
		TuyaToCapability: func(_ interface{}) (Capability, error) {
			capability := &CameraCapability{StreamingState: StreamingStateIdle}
			return capability, nil
		},
		CapabilityToTuya: func(capability Capability) (string, interface{}, error) {
			if cam, ok := capability.(*CameraCapability); ok {
				return tuyaCode, string(cam.StreamingState), nil
			}
			return "", nil, fmt.Errorf("invalid capability type for camera")
		},
	}
}

// createRTCSessionMapping creates a mapping for RTC session capability (marker mapping for device discovery)
func createRTCSessionMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeRTCSession,
		TuyaToCapability: func(_ interface{}) (Capability, error) {
			return &RTCSessionCapability{}, nil
		},
		CapabilityToTuya: func(_ Capability) (string, interface{}, error) {
			return tuyaCode, nil, nil
		},
	}
}

// initializeStandardMappings sets up the default Tuya to capability mappings
func (dcm *DeviceCapabilityMap) initializeStandardMappings() {
	dcm.mappings = map[string]map[string]CapabilityMapping{
		// Wall Switch Dimmer (tgkg) category mappings
		"tgkg": {
			"switch_led_1":   createPowerMapping("switch_led_1"),
			"bright_value_1": createBrightnessMapping("bright_value_1", 990), // Range 10-1000
		},
		// Dimmer (tgq) category mappings
		"tgq": {
			"switch_led_1":    createPowerMapping("switch_led_1"),
			"bright_value_v2": createBrightnessMapping("bright_value_v2", 990), // Special range 10-1000
		},
		// Light (dj) category mappings
		"dj": {
			"switch_led":     createPowerMapping("switch_led"),
			"bright_value":   createBrightnessMapping("bright_value", 1000),
			"colour_data_v2": createColorMapping("colour_data_v2"),
			"temp_value":     createColorTemperatureMapping("temp_value", 0, 1000),
		},
		// Strip Light (dd) category mappings
		"dd": {
			"switch_led":   createPowerMapping("switch_led"),
			"bright_value": createBrightnessMapping("bright_value", 1000),
			"colour_data":  createColorMapping("colour_data"),
		},
		// Switch (kg) category mappings
		"kg": {
			"switch_1": createPowerMapping("switch_1"),
		},
		// Socket (cz) category mappings
		"cz": {
			"switch_1": createPowerMapping("switch_1"),
		},
		// Power Strip (pc) category mappings
		"pc": {
			"switch_1": createPowerMapping("switch_1"),
		},
		// Temperature + Humidity Sensor (wsdcg) category mappings
		"wsdcg": {
			"va_temperature": createTemperatureSensorMapping("va_temperature", 10), // Tuya reports tenths of degree
			"va_humidity":    createHumiditySensorMapping("va_humidity"),
		},
		// Contact Sensor (mcs) — door/window sensor category mappings
		"mcs": {
			"doorcontact_state": createContactSensorMapping("doorcontact_state"),
		},
		// String Lights (dc) category mappings
		"dc": {
			"switch_led":   createPowerMapping("switch_led"),
			"bright_value": createBrightnessMapping("bright_value", 1000),
			"colour_data":  createColorMapping("colour_data"),
			"temp_value":   createColorTemperatureMapping("temp_value", 0, 1000),
		},
		// Ambient Light (fwd) category mappings
		"fwd": {
			"switch_led":   createPowerMapping("switch_led"),
			"bright_value": createBrightnessMapping("bright_value", 1000),
			"colour_data":  createColorMapping("colour_data"),
			"temp_value":   createColorTemperatureMapping("temp_value", 0, 1000),
		},
		// Fan (fs) category mappings
		"fs": {
			"switch_fan":        createPowerMapping("switch_fan"),
			"fan_speed_percent": createFanSpeedMapping("fan_speed_percent", 100),
		},
		// Ceiling Fan Light (fsd) category mappings
		"fsd": {
			"switch_fan":        createPowerMapping("switch_fan"),
			"fan_speed_percent": createFanSpeedMapping("fan_speed_percent", 100),
		},
		// Curtain (cl) category mappings
		"cl": {
			"control":         createPowerMapping("control"),
			"percent_control": createWindowCoveringMapping("percent_control"),
		},
		// Curtain Switch (clkg) category mappings
		"clkg": {
			"control":         createPowerMapping("control"),
			"percent_control": createWindowCoveringMapping("percent_control"),
		},
		// Lock (ms) category mappings
		"ms": {
			"closed_opened": createLockMapping("closed_opened"),
		},
		// Smart Lock (jtmspro) category mappings
		"jtmspro": {
			"closed_opened": createLockMapping("closed_opened"),
		},
		// Smart Camera (sp) category mappings
		"sp": {
			"basic_indicator": createPowerMapping("basic_indicator"),
			"camera":          createCameraMapping("camera"),
			"rtc_session":     createRTCSessionMapping("rtc_session"),
		},
		// IP Camera (ipc) category mappings
		"ipc": {
			"basic_indicator": createPowerMapping("basic_indicator"),
			"camera":          createCameraMapping("camera"),
			"rtc_session":     createRTCSessionMapping("rtc_session"),
		},
	}
}

// GetCapabilitiesFromTuyaStatus converts Tuya device status to standardized capabilities
func (dcm *DeviceCapabilityMap) GetCapabilitiesFromTuyaStatus(category string, status []DeviceStatusChange) ([]Capability, error) {
	var capabilities []Capability

	categoryMappings, exists := dcm.mappings[category]
	if !exists {
		return nil, fmt.Errorf("unsupported device category: %s", category)
	}

	for _, statusItem := range status {
		if mapping, exists := categoryMappings[statusItem.Code]; exists {
			capability, err := mapping.TuyaToCapability(statusItem.Value)
			if err != nil {
				return nil, fmt.Errorf("failed to convert %s: %w", statusItem.Code, err)
			}
			capabilities = append(capabilities, capability)
		}
	}

	return capabilities, nil
}

// GetTuyaCommandFromCapability converts a standardized capability to Tuya command
func (dcm *DeviceCapabilityMap) GetTuyaCommandFromCapability(category string, capability Capability) (string, interface{}, error) {
	categoryMappings, exists := dcm.mappings[category]
	if !exists {
		return "", nil, fmt.Errorf("unsupported device category: %s", category)
	}

	// Find the appropriate mapping for this capability type
	for _, mapping := range categoryMappings {
		if mapping.CapabilityType == capability.GetCapabilityType() {
			return mapping.CapabilityToTuya(capability)
		}
	}

	return "", nil, fmt.Errorf("no mapping found for capability type: %s in category: %s", capability.GetCapabilityType(), category)
}

// GetCapabilityFromTuyaEvent extracts capabilities from a device state change event
func (dcm *DeviceCapabilityMap) GetCapabilityFromTuyaEvent(event *DeviceStateChangeEvent, category string) ([]Capability, error) {
	return dcm.GetCapabilitiesFromTuyaStatus(category, event.Status)
}

// StandardizedDeviceStateEvent represents a device state change with standardized capabilities
type StandardizedDeviceStateEvent struct {
	DeviceID     string       `json:"deviceId"`
	ProductKey   string       `json:"productKey"`
	Category     string       `json:"category"`
	Capabilities []Capability `json:"capabilities"`
	Timestamp    int64        `json:"timestamp"`
}

// ConvertToStandardizedEvent converts a Tuya device state change event to standardized format
func (dcm *DeviceCapabilityMap) ConvertToStandardizedEvent(event *DeviceStateChangeEvent, category string) (*StandardizedDeviceStateEvent, error) {
	capabilities, err := dcm.GetCapabilityFromTuyaEvent(event, category)
	if err != nil {
		return nil, err
	}

	return &StandardizedDeviceStateEvent{
		DeviceID:     event.DeviceID,
		ProductKey:   event.ProductKey,
		Category:     category,
		Capabilities: capabilities,
		Timestamp:    time.Now().Unix(),
	}, nil
}

// AddMapping allows adding custom device category and code mappings
func (dcm *DeviceCapabilityMap) AddMapping(category, code string, mapping CapabilityMapping) {
	if dcm.mappings[category] == nil {
		dcm.mappings[category] = make(map[string]CapabilityMapping)
	}
	dcm.mappings[category][code] = mapping
}

// GetSupportedCategories returns all supported device categories
func (dcm *DeviceCapabilityMap) GetSupportedCategories() []string {
	var categories []string
	for category := range dcm.mappings {
		categories = append(categories, category)
	}
	return categories
}

// GetSupportedCodesForCategory returns all supported codes for a given category
func (dcm *DeviceCapabilityMap) GetSupportedCodesForCategory(category string) []string {
	var codes []string
	if categoryMappings, exists := dcm.mappings[category]; exists {
		for code := range categoryMappings {
			codes = append(codes, code)
		}
	}
	return codes
}

// HasPowerCapability checks if a device supports power control
func (dcm *DeviceCapabilityMap) HasPowerCapability(category string) bool {
	if categoryMappings, exists := dcm.mappings[category]; exists {
		for _, mapping := range categoryMappings {
			if mapping.CapabilityType == CapabilityTypePower {
				return true
			}
		}
	}
	return false
}

// HasBrightnessCapability checks if a device supports brightness control
func (dcm *DeviceCapabilityMap) HasBrightnessCapability(category string) bool {
	if categoryMappings, exists := dcm.mappings[category]; exists {
		for _, mapping := range categoryMappings {
			if mapping.CapabilityType == CapabilityTypeBrightness {
				return true
			}
		}
	}
	return false
}

// HasColorCapability checks if a device supports color control
func (dcm *DeviceCapabilityMap) HasColorCapability(category string) bool {
	if categoryMappings, exists := dcm.mappings[category]; exists {
		for _, mapping := range categoryMappings {
			if mapping.CapabilityType == CapabilityTypeColor {
				return true
			}
		}
	}
	return false
}

// HasColorTemperatureCapability checks if a device supports color temperature control
func (dcm *DeviceCapabilityMap) HasColorTemperatureCapability(category string) bool {
	if categoryMappings, exists := dcm.mappings[category]; exists {
		for _, mapping := range categoryMappings {
			if mapping.CapabilityType == CapabilityTypeColorTemperature {
				return true
			}
		}
	}
	return false
}

// HasTemperatureSensorCapability checks if a device supports temperature sensing
func (dcm *DeviceCapabilityMap) HasTemperatureSensorCapability(category string) bool {
	if categoryMappings, exists := dcm.mappings[category]; exists {
		for _, mapping := range categoryMappings {
			if mapping.CapabilityType == CapabilityTypeTemperatureSensor {
				return true
			}
		}
	}
	return false
}

// HasHumiditySensorCapability checks if a device supports humidity sensing
func (dcm *DeviceCapabilityMap) HasHumiditySensorCapability(category string) bool {
	if categoryMappings, exists := dcm.mappings[category]; exists {
		for _, mapping := range categoryMappings {
			if mapping.CapabilityType == CapabilityTypeHumiditySensor {
				return true
			}
		}
	}
	return false
}

// HasFanSpeedCapability checks if a device supports fan speed control
func (dcm *DeviceCapabilityMap) HasFanSpeedCapability(category string) bool {
	if categoryMappings, exists := dcm.mappings[category]; exists {
		for _, mapping := range categoryMappings {
			if mapping.CapabilityType == CapabilityTypeFanSpeed {
				return true
			}
		}
	}
	return false
}

// HasLockCapability checks if a device supports lock control
func (dcm *DeviceCapabilityMap) HasLockCapability(category string) bool {
	if categoryMappings, exists := dcm.mappings[category]; exists {
		for _, mapping := range categoryMappings {
			if mapping.CapabilityType == CapabilityTypeLock {
				return true
			}
		}
	}
	return false
}

// HasContactSensorCapability checks if a device supports contact sensing
func (dcm *DeviceCapabilityMap) HasContactSensorCapability(category string) bool {
	if categoryMappings, exists := dcm.mappings[category]; exists {
		for _, mapping := range categoryMappings {
			if mapping.CapabilityType == CapabilityTypeContactSensor {
				return true
			}
		}
	}
	return false
}

// HasWindowCoveringCapability checks if a device supports window covering control
func (dcm *DeviceCapabilityMap) HasWindowCoveringCapability(category string) bool {
	if categoryMappings, exists := dcm.mappings[category]; exists {
		for _, mapping := range categoryMappings {
			if mapping.CapabilityType == CapabilityTypeWindowCovering {
				return true
			}
		}
	}
	return false
}

// HasCameraCapability checks if a device supports camera functionality
func (dcm *DeviceCapabilityMap) HasCameraCapability(category string) bool {
	if categoryMappings, exists := dcm.mappings[category]; exists {
		for _, mapping := range categoryMappings {
			if mapping.CapabilityType == CapabilityTypeCamera {
				return true
			}
		}
	}
	return false
}

// HasRTCSessionCapability checks if a device supports RTC session
func (dcm *DeviceCapabilityMap) HasRTCSessionCapability(category string) bool {
	if categoryMappings, exists := dcm.mappings[category]; exists {
		for _, mapping := range categoryMappings {
			if mapping.CapabilityType == CapabilityTypeRTCSession {
				return true
			}
		}
	}
	return false
}
