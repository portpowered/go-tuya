package tuya

import (
	"fmt"
	"strconv"
	"time"
)

// Capability type constants.
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

	dpCodeSwitchLED   = "switch_led"
	dpCodeBrightness  = "bright_value"
	dpCodeTemperature = "temp_value"
	dpCodeColor       = "colour_data"
	dpCodeSwitch      = "switch_1"
)

const (
	hueCycleDegrees               = 360
	minimumPercentage             = 0
	percentageScale               = 100
	tuyaDataPointScale            = 1000
	tgqBrightnessOffset           = 10
	tgqBrightnessRange            = 990
	colorHexComponentWidth        = 4
	minimumColorTemperatureMireds = 153
	maximumColorTemperatureMireds = 500
	colorTemperatureMiredsRange   = maximumColorTemperatureMireds - minimumColorTemperatureMireds
	tuyaTemperatureTenthsScale    = 10
)

// Capability represents a standardized device capability interface.
type Capability interface {
	GetCapabilityType() string
	GetValue() any
	SetValue(value any) error
}

// PowerState represents the power state of a device.
type PowerState string

// Power state constants.
const (
	// PowerStateOn indicates the device is powered on.
	PowerStateOn PowerState = "on"
	// PowerStateOff indicates the device is powered off.
	PowerStateOff PowerState = "off"
)

// PowerCapability represents power on/off capability.
type PowerCapability struct {
	State PowerState `json:"state"` // "on" or "off"
}

// GetCapabilityType returns the capability type for power.
func (p *PowerCapability) GetCapabilityType() string {
	return CapabilityTypePower
}

// GetValue returns the current power state.
func (p *PowerCapability) GetValue() any {
	return p.State
}

// SetValue sets the power state from various input types.
func (p *PowerCapability) SetValue(value any) error {
	switch typedValue := value.(type) {
	case string:
		if typedValue == string(PowerStateOn) || typedValue == string(PowerStateOff) {
			p.State = PowerState(typedValue)

			return nil
		}
	case bool:
		if typedValue {
			p.State = PowerStateOn
		} else {
			p.State = PowerStateOff
		}

		return nil
	}

	return fmt.Errorf("%w: %v", errInvalidPowerStateValue, value)
}

// BrightnessCapability represents brightness adjustment capability.
type BrightnessCapability struct {
	Level int `json:"level"` // 0-100 percentage
}

// GetCapabilityType returns the capability type for brightness.
func (b *BrightnessCapability) GetCapabilityType() string {
	return CapabilityTypeBrightness
}

// GetValue returns the current brightness level.
func (b *BrightnessCapability) GetValue() any {
	return b.Level
}

// SetValue sets the brightness level from various input types.
func (b *BrightnessCapability) SetValue(value any) error {
	level, ok := parseBoundedInt(value, minimumPercentage, percentageScale)
	if ok {
		b.Level = level

		return nil
	}

	return fmt.Errorf("%w: %v (must be 0-100)", errInvalidBrightnessLevel, value)
}

// ColorCapability represents color control capability (HSV).
type ColorCapability struct {
	Hue        int `json:"hue"`        // 0-360 degrees
	Saturation int `json:"saturation"` // 0-100 percentage
	Value      int `json:"value"`      // 0-100 percentage (brightness in HSV)
}

// GetCapabilityType returns the capability type for color.
func (c *ColorCapability) GetCapabilityType() string {
	return CapabilityTypeColor
}

// GetValue returns the current color values as an HSV map.
func (c *ColorCapability) GetValue() any {
	return map[string]int{
		"hue":        c.Hue,
		"saturation": c.Saturation,
		"value":      c.Value,
	}
}

// SetValue sets the color values from various input types.
func (c *ColorCapability) SetValue(value any) error {
	switch typedValue := value.(type) {
	case map[string]any:
		c.setFloatValues(typedValue)

		return nil
	case map[string]int:
		c.setIntValues(typedValue)

		return nil
	}

	return fmt.Errorf("%w: %v", errInvalidColorValue, value)
}

func (c *ColorCapability) setFloatValues(values map[string]any) {
	if hue, ok := values["hue"].(float64); ok {
		c.Hue = int(hue) % hueCycleDegrees
	}

	if saturation, ok := values["saturation"].(float64); ok && saturation >= 0 && saturation <= 100 {
		c.Saturation = int(saturation)
	}

	if value, ok := values["value"].(float64); ok && value >= 0 && value <= 100 {
		c.Value = int(value)
	}
}

func (c *ColorCapability) setIntValues(values map[string]int) {
	if hue, ok := values["hue"]; ok {
		c.Hue = hue % hueCycleDegrees
	}

	if saturation, ok := values["saturation"]; ok && saturation >= 0 && saturation <= 100 {
		c.Saturation = saturation
	}

	if value, ok := values["value"]; ok && value >= 0 && value <= 100 {
		c.Value = value
	}
}

// ColorTemperatureCapability represents color temperature control capability.
type ColorTemperatureCapability struct {
	Mireds int `json:"mireds"` // Color temperature in mireds (153-500, i.e. 2000-6500K)
}

// GetCapabilityType returns the capability type for color temperature.
func (ct *ColorTemperatureCapability) GetCapabilityType() string {
	return CapabilityTypeColorTemperature
}

// GetValue returns the current color temperature in mireds.
func (ct *ColorTemperatureCapability) GetValue() any {
	return ct.Mireds
}

// SetValue sets the color temperature from various input types.
func (ct *ColorTemperatureCapability) SetValue(value any) error {
	mireds, ok := parseBoundedInt(value, minimumColorTemperatureMireds, maximumColorTemperatureMireds)
	if ok {
		ct.Mireds = mireds

		return nil
	}

	return fmt.Errorf("%w: %v (must be 153-500 mireds)", errInvalidColorTemperature, value)
}

// TemperatureSensorCapability represents a temperature sensor reading.
type TemperatureSensorCapability struct {
	Temperature float64 `json:"temperature"` // Temperature in Celsius
}

// GetCapabilityType returns the capability type for temperature sensor.
func (ts *TemperatureSensorCapability) GetCapabilityType() string {
	return CapabilityTypeTemperatureSensor
}

// GetValue returns the current temperature in Celsius.
func (ts *TemperatureSensorCapability) GetValue() any {
	return ts.Temperature
}

// SetValue sets the temperature from various input types.
func (ts *TemperatureSensorCapability) SetValue(value any) error {
	switch typedValue := value.(type) {
	case float64:
		ts.Temperature = typedValue

		return nil
	case int:
		ts.Temperature = float64(typedValue)

		return nil
	case string:
		temp, err := strconv.ParseFloat(typedValue, 64)
		if err == nil {
			ts.Temperature = temp

			return nil
		}
	}

	return fmt.Errorf("%w: %v", errInvalidTemperatureValue, value)
}

// HumiditySensorCapability represents a humidity sensor reading.
type HumiditySensorCapability struct {
	Humidity int `json:"humidity"` // Humidity percentage (0-100)
}

// GetCapabilityType returns the capability type for humidity sensor.
func (hs *HumiditySensorCapability) GetCapabilityType() string {
	return CapabilityTypeHumiditySensor
}

// GetValue returns the current humidity percentage.
func (hs *HumiditySensorCapability) GetValue() any {
	return hs.Humidity
}

// SetValue sets the humidity from various input types.
func (hs *HumiditySensorCapability) SetValue(value any) error {
	humidity, ok := parseBoundedInt(value, minimumPercentage, percentageScale)
	if ok {
		hs.Humidity = humidity

		return nil
	}

	return fmt.Errorf("%w: %v (must be 0-100)", errInvalidHumidityValue, value)
}

// FanSpeedCapability represents fan speed control capability.
type FanSpeedCapability struct {
	Speed int `json:"speed"` // Fan speed percentage (0-100)
}

// GetCapabilityType returns the capability type for fan speed.
func (fs *FanSpeedCapability) GetCapabilityType() string {
	return CapabilityTypeFanSpeed
}

// GetValue returns the current fan speed percentage.
func (fs *FanSpeedCapability) GetValue() any {
	return fs.Speed
}

// SetValue sets the fan speed from various input types.
func (fs *FanSpeedCapability) SetValue(value any) error {
	speed, ok := parseBoundedInt(value, minimumPercentage, percentageScale)
	if ok {
		fs.Speed = speed

		return nil
	}

	return fmt.Errorf("%w: %v (must be 0-100)", errInvalidFanSpeedValue, value)
}

// LockState represents the lock state of a device.
type LockState string

// Lock state constants.
const (
	LockStateLocked   LockState = "locked"
	LockStateUnlocked LockState = "unlocked"
)

// LockCapability represents lock/unlock capability.
type LockCapability struct {
	State LockState `json:"state"` // "locked" or "unlocked"
}

// GetCapabilityType returns the capability type for lock.
func (l *LockCapability) GetCapabilityType() string {
	return CapabilityTypeLock
}

// GetValue returns the current lock state.
func (l *LockCapability) GetValue() any {
	return l.State
}

// SetValue sets the lock state from various input types.
func (l *LockCapability) SetValue(value any) error {
	switch typedValue := value.(type) {
	case string:
		if typedValue == string(LockStateLocked) || typedValue == string(LockStateUnlocked) {
			l.State = LockState(typedValue)

			return nil
		}
	case bool:
		// true = locked (closed), false = unlocked (opened)
		if typedValue {
			l.State = LockStateLocked
		} else {
			l.State = LockStateUnlocked
		}

		return nil
	}

	return fmt.Errorf("%w: %v", errInvalidLockStateValue, value)
}

// ContactSensorCapability represents a contact (door/window) sensor reading.
type ContactSensorCapability struct {
	//nolint:tagliatelle // Preserve the public capability model's established JSON key.
	IsOpen bool `json:"isOpen"` // true if the contact is open (door/window open)
}

// GetCapabilityType returns the capability type for contact sensor.
func (cs *ContactSensorCapability) GetCapabilityType() string {
	return CapabilityTypeContactSensor
}

// GetValue returns whether the contact is open.
func (cs *ContactSensorCapability) GetValue() any {
	return cs.IsOpen
}

// SetValue sets the contact sensor state from various input types.
func (cs *ContactSensorCapability) SetValue(value any) error {
	switch typedValue := value.(type) {
	case bool:
		cs.IsOpen = typedValue

		return nil
	case string:
		switch typedValue {
		case "true", "open":
			cs.IsOpen = true

			return nil
		case "false", "closed":
			cs.IsOpen = false

			return nil
		}
	}

	return fmt.Errorf("%w: %v", errInvalidContactSensorValue, value)
}

// WindowCoveringCapability represents window covering (curtain/blind) position.
type WindowCoveringCapability struct {
	Position int `json:"position"` // Position percentage (0-100, where 0=closed, 100=fully open)
}

// GetCapabilityType returns the capability type for window covering.
func (wc *WindowCoveringCapability) GetCapabilityType() string {
	return CapabilityTypeWindowCovering
}

// GetValue returns the current position percentage.
func (wc *WindowCoveringCapability) GetValue() any {
	return wc.Position
}

// SetValue sets the window covering position from various input types.
func (wc *WindowCoveringCapability) SetValue(value any) error {
	position, ok := parseBoundedInt(value, minimumPercentage, percentageScale)
	if ok {
		wc.Position = position

		return nil
	}

	return fmt.Errorf("%w: %v (must be 0-100)", errInvalidWindowCoveringPosition, value)
}

func parseBoundedInt(value any, minimum, maximum int) (int, bool) {
	switch typedValue := value.(type) {
	case int:
		return withinIntRange(typedValue, minimum, maximum)
	case float64:
		return withinIntRange(int(typedValue), minimum, maximum)
	case string:
		converted, err := strconv.Atoi(typedValue)
		if err != nil {
			return 0, false
		}

		return withinIntRange(converted, minimum, maximum)
	}

	return 0, false
}

func withinIntRange(value, minimum, maximum int) (int, bool) {
	return value, value >= minimum && value <= maximum
}

// StreamingState represents the streaming state of a camera.
type StreamingState string

// Streaming state constants for camera capability.
const (
	// StreamingStateIdle indicates the camera is not actively streaming.
	StreamingStateIdle StreamingState = "idle"
	// StreamingStateStreaming indicates the camera is actively streaming.
	StreamingStateStreaming StreamingState = "streaming"
)

// CameraCapability represents camera functionality.
type CameraCapability struct {
	//nolint:tagliatelle // Preserve the public capability model's established JSON key.
	StreamingState StreamingState `json:"streamingState"`
}

// GetCapabilityType returns the capability type for camera.
func (c *CameraCapability) GetCapabilityType() string {
	return CapabilityTypeCamera
}

// GetValue returns the current streaming state.
func (c *CameraCapability) GetValue() any {
	return c.StreamingState
}

// SetValue sets the camera streaming state.
func (c *CameraCapability) SetValue(value any) error {
	switch typedValue := value.(type) {
	case string:
		switch StreamingState(typedValue) {
		case StreamingStateIdle, StreamingStateStreaming:
			c.StreamingState = StreamingState(typedValue)

			return nil
		}
	case StreamingState:
		c.StreamingState = typedValue

		return nil
	}

	return fmt.Errorf("%w: %v (must be 'idle' or 'streaming')", errInvalidStreamingState, value)
}

// RTCSessionCapability represents WebRTC session capability.
type RTCSessionCapability struct{}

// GetCapabilityType returns the capability type for RTC session.
func (r *RTCSessionCapability) GetCapabilityType() string {
	return CapabilityTypeRTCSession
}

// GetValue returns nil as RTC session has no persistent value.
func (r *RTCSessionCapability) GetValue() any {
	return nil
}

// SetValue is a no-op for RTC session capability.
func (r *RTCSessionCapability) SetValue(_ any) error {
	return nil
}

// CapabilityMapping defines how to map Tuya codes to capabilities.
type CapabilityMapping struct {
	CapabilityType   string
	TuyaToCapability func(any) (Capability, error)
	CapabilityToTuya func(Capability) (string, any, error)
}

// DeviceCapabilityMap maps device categories and codes to capabilities.
type DeviceCapabilityMap struct {
	mappings map[string]map[string]CapabilityMapping
}

// NewDeviceCapabilityMap creates a new capability mapping system.
func NewDeviceCapabilityMap() *DeviceCapabilityMap {
	dcm := &DeviceCapabilityMap{
		mappings: make(map[string]map[string]CapabilityMapping),
	}
	dcm.initializeStandardMappings()

	return dcm
}

// Helper functions for common mappings to reduce duplication.
func createPowerMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypePower,
		TuyaToCapability: func(value any) (Capability, error) {
			capability := &PowerCapability{State: ""}
			err := capability.SetValue(value)

			return capability, err
		},
		CapabilityToTuya: func(capability Capability) (string, any, error) {
			if power, ok := capability.(*PowerCapability); ok {
				return tuyaCode, power.State == PowerStateOn, nil
			}

			return "", nil, errInvalidPowerCapability
		},
	}
}

func createBrightnessMapping(tuyaCode string, maxValue float64) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeBrightness,
		TuyaToCapability: func(value any) (Capability, error) {
			capability := &BrightnessCapability{Level: 0}

			if val, ok := value.(float64); ok {
				var percentage int
				if maxValue == tuyaDataPointScale {
					percentage = int(val / tuyaDataPointScale * percentageScale)
				} else {
					// For tgq category: range 10-1000
					percentage = min(max(int((val-tgqBrightnessOffset)/tgqBrightnessRange*percentageScale), 0), percentageScale)
				}

				err := capability.SetValue(percentage)

				return capability, err
			}

			return capability, fmt.Errorf("%w: %v", errInvalidBrightnessValue, value)
		},
		CapabilityToTuya: func(capability Capability) (string, any, error) {
			if brightness, ok := capability.(*BrightnessCapability); ok {
				var tuyaValue int
				if maxValue == tuyaDataPointScale {
					tuyaValue = int(float64(brightness.Level) / percentageScale * tuyaDataPointScale)
				} else {
					// For tgq category: range 10-1000
					tuyaValue = int(float64(brightness.Level)/percentageScale*tgqBrightnessRange + tgqBrightnessOffset)
				}

				return tuyaCode, tuyaValue, nil
			}

			return "", nil, errInvalidBrightnessCapability
		},
	}
}

func createColorMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeColor,
		TuyaToCapability: func(value any) (Capability, error) {
			capability := &ColorCapability{Hue: 0, Saturation: 0, Value: 0}

			colorString, ok := value.(string)
			if !ok || len(colorString) < 3*colorHexComponentWidth {
				return capability, nil
			}

			hueHex := colorString[0:colorHexComponentWidth]
			saturationHex := colorString[colorHexComponentWidth : 2*colorHexComponentWidth]
			valueHex := colorString[2*colorHexComponentWidth : 3*colorHexComponentWidth]

			hue, err := strconv.ParseInt(hueHex, 16, 32)
			if err == nil {
				capability.Hue = int(hue)
			}

			saturation, err := strconv.ParseInt(saturationHex, 16, 32)
			if err == nil {
				capability.Saturation = int(saturation / tuyaDataPointScale * percentageScale)
			}

			valueNumber, err := strconv.ParseInt(valueHex, 16, 32)
			if err == nil {
				capability.Value = int(valueNumber / tuyaDataPointScale * percentageScale)
			}

			return capability, nil
		},
		CapabilityToTuya: func(capability Capability) (string, any, error) {
			if color, ok := capability.(*ColorCapability); ok {
				hue := fmt.Sprintf("%04x", color.Hue)
				sat := fmt.Sprintf("%04x", int(float64(color.Saturation)/percentageScale*tuyaDataPointScale))
				val := fmt.Sprintf("%04x", int(float64(color.Value)/percentageScale*tuyaDataPointScale))
				colorData := hue + sat + val

				return tuyaCode, colorData, nil
			}

			return "", nil, errInvalidColorCapability
		},
	}
}

func createColorTemperatureMapping(tuyaCode string, tuyaMin, tuyaMax int) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeColorTemperature,
		TuyaToCapability: func(value any) (Capability, error) {
			capability := &ColorTemperatureCapability{Mireds: 0}

			if val, ok := value.(float64); ok {
				// Convert Tuya range (tuyaMin-tuyaMax) to mireds.
				normalized := (val - float64(tuyaMin)) / float64(tuyaMax-tuyaMin)

				mireds := min(
					max(

						int(minimumColorTemperatureMireds+normalized*float64(colorTemperatureMiredsRange)), minimumColorTemperatureMireds), maximumColorTemperatureMireds)

				err := capability.SetValue(mireds)

				return capability, err
			}

			return capability, fmt.Errorf("%w: %v", errInvalidColorTemperatureValue, value)
		},
		CapabilityToTuya: func(capability Capability) (string, any, error) {
			if ct, ok := capability.(*ColorTemperatureCapability); ok {
				// Convert mireds to the Tuya range (tuyaMin-tuyaMax).
				normalized := float64(ct.Mireds-minimumColorTemperatureMireds) / float64(colorTemperatureMiredsRange)
				tuyaValue := int(float64(tuyaMin) + normalized*float64(tuyaMax-tuyaMin))

				return tuyaCode, tuyaValue, nil
			}

			return "", nil, errInvalidColorTemperatureCapability
		},
	}
}

func createTemperatureSensorMapping(tuyaCode string, scaleDivisor float64) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeTemperatureSensor,
		TuyaToCapability: func(value any) (Capability, error) {
			capability := &TemperatureSensorCapability{Temperature: 0}

			if val, ok := value.(float64); ok {
				temp := val / scaleDivisor
				err := capability.SetValue(temp)

				return capability, err
			}

			return capability, fmt.Errorf("%w: %v", errInvalidTemperatureSensorValue, value)
		},
		CapabilityToTuya: func(capability Capability) (string, any, error) {
			if ts, ok := capability.(*TemperatureSensorCapability); ok {
				tuyaValue := int(ts.Temperature * scaleDivisor)

				return tuyaCode, tuyaValue, nil
			}

			return "", nil, errInvalidTemperatureSensorCapability
		},
	}
}

func createHumiditySensorMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeHumiditySensor,
		TuyaToCapability: func(value any) (Capability, error) {
			capability := &HumiditySensorCapability{Humidity: 0}
			if val, ok := value.(float64); ok {
				err := capability.SetValue(int(val))

				return capability, err
			}

			return capability, fmt.Errorf("%w: %v", errInvalidHumiditySensorValue, value)
		},
		CapabilityToTuya: func(capability Capability) (string, any, error) {
			if hs, ok := capability.(*HumiditySensorCapability); ok {
				return tuyaCode, hs.Humidity, nil
			}

			return "", nil, errInvalidHumiditySensorCapability
		},
	}
}

func createFanSpeedMapping(tuyaCode string, maxSpeedLevels int) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeFanSpeed,
		TuyaToCapability: func(value any) (Capability, error) {
			capability := &FanSpeedCapability{Speed: 0}

			if val, ok := value.(float64); ok {
				// Convert discrete speed levels (1-maxSpeedLevels) to percentage (0-100)
				percentage := min(max(int(val/float64(maxSpeedLevels)*percentageScale), 0), percentageScale)

				err := capability.SetValue(percentage)

				return capability, err
			}

			return capability, fmt.Errorf("%w: %v", errInvalidFanSpeedValue, value)
		},
		CapabilityToTuya: func(capability Capability) (string, any, error) {
			if fs, ok := capability.(*FanSpeedCapability); ok {
				// Convert percentage (0-100) to discrete speed levels (0-maxSpeedLevels)
				tuyaValue := min(int(float64(fs.Speed)/percentageScale*float64(maxSpeedLevels)), maxSpeedLevels)

				return tuyaCode, tuyaValue, nil
			}

			return "", nil, errInvalidFanSpeedCapability
		},
	}
}

func createLockMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeLock,
		TuyaToCapability: func(value any) (Capability, error) {
			capability := &LockCapability{State: ""}
			// Tuya locks use boolean: true = locked (closed), false = unlocked (opened)
			err := capability.SetValue(value)

			return capability, err
		},
		CapabilityToTuya: func(capability Capability) (string, any, error) {
			if lock, ok := capability.(*LockCapability); ok {
				return tuyaCode, lock.State == LockStateLocked, nil
			}

			return "", nil, errInvalidLockCapability
		},
	}
}

func createContactSensorMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeContactSensor,
		TuyaToCapability: func(value any) (Capability, error) {
			capability := &ContactSensorCapability{IsOpen: false}
			// Tuya contact sensors: true = open, false = closed
			err := capability.SetValue(value)

			return capability, err
		},
		CapabilityToTuya: func(capability Capability) (string, any, error) {
			if cs, ok := capability.(*ContactSensorCapability); ok {
				return tuyaCode, cs.IsOpen, nil
			}

			return "", nil, errInvalidContactSensorCapability
		},
	}
}

func createWindowCoveringMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeWindowCovering,
		TuyaToCapability: func(value any) (Capability, error) {
			capability := &WindowCoveringCapability{Position: 0}
			if val, ok := value.(float64); ok {
				err := capability.SetValue(int(val))

				return capability, err
			}

			return capability, fmt.Errorf("%w: %v", errInvalidWindowCoveringValue, value)
		},
		CapabilityToTuya: func(capability Capability) (string, any, error) {
			if wc, ok := capability.(*WindowCoveringCapability); ok {
				return tuyaCode, wc.Position, nil
			}

			return "", nil, errInvalidWindowCoveringCapability
		},
	}
}

// createCameraMapping creates a mapping for camera capability (marker mapping for device discovery).
func createCameraMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeCamera,
		TuyaToCapability: func(_ any) (Capability, error) {
			capability := &CameraCapability{StreamingState: StreamingStateIdle}

			return capability, nil
		},
		CapabilityToTuya: func(capability Capability) (string, any, error) {
			if cam, ok := capability.(*CameraCapability); ok {
				return tuyaCode, string(cam.StreamingState), nil
			}

			return "", nil, errInvalidCameraCapability
		},
	}
}

// createRTCSessionMapping creates a mapping for RTC session capability (marker mapping for device discovery).
func createRTCSessionMapping(tuyaCode string) CapabilityMapping {
	return CapabilityMapping{
		CapabilityType: CapabilityTypeRTCSession,
		TuyaToCapability: func(_ any) (Capability, error) {
			return &RTCSessionCapability{}, nil
		},
		CapabilityToTuya: func(_ Capability) (string, any, error) {
			return tuyaCode, nil, nil
		},
	}
}

// initializeStandardMappings sets up the default Tuya to capability mappings.
//
//nolint:funcorder,funlen // Keep the category map adjacent to its private constructors; splitting its data obscures the mapping.
func (dcm *DeviceCapabilityMap) initializeStandardMappings() {
	dcm.mappings = map[string]map[string]CapabilityMapping{
		// Wall Switch Dimmer (tgkg) category mappings
		"tgkg": {
			"switch_led_1":   createPowerMapping("switch_led_1"),
			"bright_value_1": createBrightnessMapping("bright_value_1", tgqBrightnessRange), // Range 10-1000
		},
		// Dimmer (tgq) category mappings
		"tgq": {
			"switch_led_1":    createPowerMapping("switch_led_1"),
			"bright_value_v2": createBrightnessMapping("bright_value_v2", tgqBrightnessRange), // Special range 10-1000
		},
		// Light (dj) category mappings
		"dj": {
			dpCodeSwitchLED:   createPowerMapping(dpCodeSwitchLED),
			dpCodeBrightness:  createBrightnessMapping(dpCodeBrightness, tuyaDataPointScale),
			"colour_data_v2":  createColorMapping("colour_data_v2"),
			dpCodeTemperature: createColorTemperatureMapping(dpCodeTemperature, 0, tuyaDataPointScale),
		},
		// Strip Light (dd) category mappings
		"dd": {
			dpCodeSwitchLED:  createPowerMapping(dpCodeSwitchLED),
			dpCodeBrightness: createBrightnessMapping(dpCodeBrightness, tuyaDataPointScale),
			dpCodeColor:      createColorMapping(dpCodeColor),
		},
		// Switch (kg) category mappings
		"kg": {
			dpCodeSwitch: createPowerMapping(dpCodeSwitch),
		},
		// Socket (cz) category mappings
		"cz": {
			dpCodeSwitch: createPowerMapping(dpCodeSwitch),
		},
		// Power Strip (pc) category mappings
		"pc": {
			dpCodeSwitch: createPowerMapping(dpCodeSwitch),
		},
		// Temperature + Humidity Sensor (wsdcg) category mappings
		"wsdcg": {
			"va_temperature": createTemperatureSensorMapping("va_temperature", tuyaTemperatureTenthsScale), // Tuya reports tenths of degree
			"va_humidity":    createHumiditySensorMapping("va_humidity"),
		},
		// Contact Sensor (mcs) — door/window sensor category mappings
		"mcs": {
			"doorcontact_state": createContactSensorMapping("doorcontact_state"),
		},
		// String Lights (dc) category mappings
		"dc": {
			dpCodeSwitchLED:   createPowerMapping(dpCodeSwitchLED),
			dpCodeBrightness:  createBrightnessMapping(dpCodeBrightness, tuyaDataPointScale),
			dpCodeColor:       createColorMapping(dpCodeColor),
			dpCodeTemperature: createColorTemperatureMapping(dpCodeTemperature, 0, tuyaDataPointScale),
		},
		// Ambient Light (fwd) category mappings
		"fwd": {
			dpCodeSwitchLED:   createPowerMapping(dpCodeSwitchLED),
			dpCodeBrightness:  createBrightnessMapping(dpCodeBrightness, tuyaDataPointScale),
			dpCodeColor:       createColorMapping(dpCodeColor),
			dpCodeTemperature: createColorTemperatureMapping(dpCodeTemperature, 0, tuyaDataPointScale),
		},
		// Fan (fs) category mappings
		"fs": {
			"switch_fan":        createPowerMapping("switch_fan"),
			"fan_speed_percent": createFanSpeedMapping("fan_speed_percent", percentageScale),
		},
		// Ceiling Fan Light (fsd) category mappings
		"fsd": {
			"switch_fan":        createPowerMapping("switch_fan"),
			"fan_speed_percent": createFanSpeedMapping("fan_speed_percent", percentageScale),
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

// GetCapabilitiesFromTuyaStatus converts Tuya device status to standardized capabilities.
func (dcm *DeviceCapabilityMap) GetCapabilitiesFromTuyaStatus(category string, status []DeviceStatusChange) ([]Capability, error) {
	var capabilities []Capability

	categoryMappings, exists := dcm.mappings[category]
	if !exists {
		return nil, fmt.Errorf("%w: %s", errUnsupportedDeviceCategory, category)
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

// GetTuyaCommandFromCapability converts a standardized capability to Tuya command.
func (dcm *DeviceCapabilityMap) GetTuyaCommandFromCapability(category string, capability Capability) (string, any, error) {
	categoryMappings, exists := dcm.mappings[category]
	if !exists {
		return "", nil, fmt.Errorf("%w: %s", errUnsupportedDeviceCategory, category)
	}

	// Find the appropriate mapping for this capability type
	for _, mapping := range categoryMappings {
		if mapping.CapabilityType == capability.GetCapabilityType() {
			return mapping.CapabilityToTuya(capability)
		}
	}

	return "", nil, fmt.Errorf("%w: %s in category: %s", errUnmappedCapability, capability.GetCapabilityType(), category)
}

// GetCapabilityFromTuyaEvent extracts capabilities from a device state change event.
func (dcm *DeviceCapabilityMap) GetCapabilityFromTuyaEvent(event *DeviceStateChangeEvent, category string) ([]Capability, error) {
	return dcm.GetCapabilitiesFromTuyaStatus(category, event.Status)
}

// StandardizedDeviceStateEvent represents a device state change with standardized capabilities.
type StandardizedDeviceStateEvent struct {
	//nolint:tagliatelle // This public event model preserves its established JSON key.
	DeviceID string `json:"deviceId"`
	//nolint:tagliatelle // This public event model preserves its established JSON key.
	ProductKey   string       `json:"productKey"`
	Category     string       `json:"category"`
	Capabilities []Capability `json:"capabilities"`
	Timestamp    int64        `json:"timestamp"`
}

// ConvertToStandardizedEvent converts a Tuya device state change event to standardized format.
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

// AddMapping allows adding custom device category and code mappings.
func (dcm *DeviceCapabilityMap) AddMapping(category, code string, mapping CapabilityMapping) {
	if dcm.mappings[category] == nil {
		dcm.mappings[category] = make(map[string]CapabilityMapping)
	}

	dcm.mappings[category][code] = mapping
}

// GetSupportedCategories returns all supported device categories.
func (dcm *DeviceCapabilityMap) GetSupportedCategories() []string {
	categories := make([]string, 0, len(dcm.mappings))
	for category := range dcm.mappings {
		categories = append(categories, category)
	}

	return categories
}

// GetSupportedCodesForCategory returns all supported codes for a given category.
func (dcm *DeviceCapabilityMap) GetSupportedCodesForCategory(category string) []string {
	var codes []string

	if categoryMappings, exists := dcm.mappings[category]; exists {
		for code := range categoryMappings {
			codes = append(codes, code)
		}
	}

	return codes
}

// HasPowerCapability checks if a device supports power control.
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

// HasBrightnessCapability checks if a device supports brightness control.
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

// HasColorCapability checks if a device supports color control.
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

// HasColorTemperatureCapability checks if a device supports color temperature control.
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

// HasTemperatureSensorCapability checks if a device supports temperature sensing.
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

// HasHumiditySensorCapability checks if a device supports humidity sensing.
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

// HasFanSpeedCapability checks if a device supports fan speed control.
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

// HasLockCapability checks if a device supports lock control.
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

// HasContactSensorCapability checks if a device supports contact sensing.
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

// HasWindowCoveringCapability checks if a device supports window covering control.
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

// HasCameraCapability checks if a device supports camera functionality.
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

// HasRTCSessionCapability checks if a device supports RTC session.
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
