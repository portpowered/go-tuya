package tuya

import (
	"errors"
	"fmt"
)

var (
	errRefreshTokenFailure                = errors.New("failed to refresh token")
	errLoginFailure                       = errors.New("login failed")
	errLoginCodeValidationFailure         = errors.New("validate login code failed")
	errInvalidPowerStateValue             = errors.New("invalid power state value")
	errInvalidBrightnessLevel             = errors.New("invalid brightness level")
	errInvalidColorValue                  = errors.New("invalid color value")
	errInvalidColorTemperature            = errors.New("invalid color temperature")
	errInvalidTemperatureValue            = errors.New("invalid temperature value")
	errInvalidHumidityValue               = errors.New("invalid humidity value")
	errInvalidFanSpeedValue               = errors.New("invalid fan speed value")
	errInvalidLockStateValue              = errors.New("invalid lock state value")
	errInvalidContactSensorValue          = errors.New("invalid contact sensor value")
	errInvalidWindowCoveringPosition      = errors.New("invalid window covering position")
	errInvalidStreamingState              = errors.New("invalid streaming state")
	errInvalidBrightnessValue             = errors.New("invalid brightness value")
	errInvalidColorTemperatureValue       = errors.New("invalid color temperature value")
	errInvalidTemperatureSensorValue      = errors.New("invalid temperature sensor value")
	errInvalidHumiditySensorValue         = errors.New("invalid humidity sensor value")
	errInvalidWindowCoveringValue         = errors.New("invalid window covering value")
	errInvalidPowerCapability             = errors.New("invalid capability type for power")
	errInvalidBrightnessCapability        = errors.New("invalid capability type for brightness")
	errInvalidColorCapability             = errors.New("invalid capability type for color")
	errInvalidColorTemperatureCapability  = errors.New("invalid capability type for color temperature")
	errInvalidTemperatureSensorCapability = errors.New("invalid capability type for temperature sensor")
	errInvalidHumiditySensorCapability    = errors.New("invalid capability type for humidity sensor")
	errInvalidFanSpeedCapability          = errors.New("invalid capability type for fan speed")
	errInvalidLockCapability              = errors.New("invalid capability type for lock")
	errInvalidContactSensorCapability     = errors.New("invalid capability type for contact sensor")
	errInvalidWindowCoveringCapability    = errors.New("invalid capability type for window covering")
	errInvalidCameraCapability            = errors.New("invalid capability type for camera")
	errUnsupportedDeviceCategory          = errors.New("unsupported device category")
	errUnmappedCapability                 = errors.New("no mapping found for capability type")
	errNilClientOption                    = errors.New("client option")
	errHTTPClientRequired                 = errors.New("HTTP client is required")
	errClientIDRequired                   = errors.New("client ID must not be empty")
	errAuthenticationSchemaRequired       = errors.New("authentication schema must not be empty")
	errMQTTFactoryRequired                = errors.New("MQTT client factory is required")
	errHTTPClientNil                      = errors.New("HTTP client must not be nil")
	errHTTPOptionsConflict                = errors.New("HTTP client and transport options are mutually exclusive")
	errHTTPTransportNil                   = errors.New("HTTP transport must not be nil")
	errMQTTFactoryNil                     = errors.New("MQTT client factory must not be nil")
	errRTCSignalingNil                    = errors.New("RTC signaling client must not be nil")
	errAbsoluteHTTPURL                    = errors.New("must be an absolute HTTP or HTTPS URL")
	errDeviceIDRequired                   = errors.New("device ID is required")
	errSDPOfferRequired                   = errors.New("SDP offer is required")
	errRTCSignalingUnconfigured           = errors.New("RTC signaling client is not configured")
	errSDPAnswerEmpty                     = errors.New("empty SDP answer received from Tuya API")
	errUnsupportedRegion                  = errors.New("unsupported region")
	errWireQueryIntegerOutOfRange         = errors.New("wire query integer")
	errUnschematizedOperation             = errors.New("is not in api/openapi.yaml")
	errRefreshTokenRequired               = errors.New("refresh token is required; set it on the session or request")
	errAccessTokenRequired                = errors.New("access token is required; set it on the session or request")
	errHTTPResponse                       = errors.New("response error")
	errNetworkError                       = errors.New("network error")
	errCipherDataTooShort                 = errors.New("cipher data too short")
	errProtocolFieldInvalid               = errors.New("missing or invalid protocol field")
	errDataFieldInvalid                   = errors.New("missing or invalid data field")
	errUnsupportedProtocol                = errors.New("unsupported protocol")
	errStateChangeDeviceIDMissing         = errors.New("missing device ID in state change event")
	errManagementBizCodeMissing           = errors.New("missing bizCode in management event")
	errManagementDeviceIDMissing          = errors.New("missing devId in management event")
	errMQTTFactoryReturnedNil             = errors.New("MQTT client factory returned nil")
	errResponseIsNil                      = errors.New("response is nil")
	errWireFieldNotJSONEncoded            = errors.New("wire field")
)

// ErrorKind identifies a stable class of client failure.
type ErrorKind string

const (
	// ErrorInvalidOperation indicates that an operation is not part of the supported API.
	ErrorInvalidOperation ErrorKind = "invalid_operation"
	// ErrorUnauthorized indicates that the request lacks valid authorization.
	ErrorUnauthorized ErrorKind = "unauthorized"
	// ErrorNotFound indicates that the requested provider resource does not exist.
	ErrorNotFound ErrorKind = "not_found"
	// ErrorTransport indicates that the request failed at the transport layer.
	ErrorTransport ErrorKind = "transport"
	// ErrorProvider indicates that the provider rejected or failed the operation.
	ErrorProvider ErrorKind = "provider"
	// ErrorProtocol indicates that the provider response violated the expected protocol.
	ErrorProtocol ErrorKind = "protocol"
)

// ClientError classifies a failed client operation and preserves its cause.
// Use errors.As to inspect Kind without parsing error text.
type ClientError struct {
	Kind  ErrorKind
	Cause error
}

func (e *ClientError) Error() string {
	return fmt.Sprintf("tuya %s: %v", e.Kind, e.Cause)
}

func (e *ClientError) Unwrap() error { return e.Cause }

func clientError(kind ErrorKind, cause error) error {
	if cause == nil {
		return nil
	}

	return &ClientError{Kind: kind, Cause: cause}
}
