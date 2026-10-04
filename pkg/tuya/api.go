// Package tuya provides a Go client library for interacting with Tuya smart devices via the Tuya Device Sharing SDK.
package tuya

import (
	"context"
	"net/http"
	"sync"
)

// This file contains the client interfaces for using the Tuya API client.

// Auth defines the interface for Tuya authentication operations.
type Auth interface {
	// Tuya supports a QR code based login flow. This is used to generate a QR code that can be scanned by the user to login to the Tuya cloud.
	GenerateQrCodeForLogin(ctx context.Context, req LoginRequest) (LoginResponse, error)
	// Validate the login code and get the access token that is generated using the GenerateQrCodeForLogin method.
	ValidateLoginCode(ctx context.Context, req ValidateLoginCodeRequest) (ValidateLoginCodeResponse, error)
}

// Homes defines the interface for Tuya home management operations.
type Homes interface {
	QueryHomes(ctx context.Context, req QueryHomesRequest) (QueryHomesResponse, error)
}

// Devices defines the interface for Tuya device operations.
type Devices interface { //nolint:interfacebloat // This public aggregate preserves the established device client contract.
	SendCommands(ctx context.Context, req SendCommandsRequest) (SendCommandsResponse, error)
	GetDeviceStreamAllocate(ctx context.Context, req GetDeviceStreamAllocateRequest) (GetDeviceStreamAllocateResponse, error)
	QueryDevicesByHome(ctx context.Context, req QueryDevicesByHomeRequest) (QueryDevicesByHomeResponse, error)
	QueryDevicesByIDs(ctx context.Context, req QueryDevicesByIDsRequest) (QueryDevicesByIDsResponse, error)
	QueryDeviceStatus(ctx context.Context, req QueryDeviceStatusRequest) (QueryDeviceStatusResponse, error)
	QueryDeviceSpecification(ctx context.Context, req QueryDeviceSpecificationRequest) (QueryDeviceSpecificationResponse, error)
	GetDeviceDetails(ctx context.Context, req GetDeviceDetailsRequest) (GetDeviceDetailsResponse, error)
	QueryDevicesByUser(ctx context.Context, req QueryDevicesByUserRequest) (QueryDevicesByUserResponse, error)
	QueryDevices(ctx context.Context, req QueryDevicesRequest) (QueryDevicesResponse, error)
	UpdateDeviceFunctionName(ctx context.Context, req UpdateDeviceFunctionNameRequest) (UpdateDeviceFunctionNameResponse, error)
	QueryDeviceLogs(ctx context.Context, req QueryDeviceLogsRequest) (QueryDeviceLogsResponse, error)
	ResetDeviceFactoryDefaults(ctx context.Context, req ResetDeviceFactoryDefaultsRequest) (ResetDeviceFactoryDefaultsResponse, error)
	DeleteDevice(ctx context.Context, req DeleteDeviceRequest) (DeleteDeviceResponse, error)
	QuerySubDevices(ctx context.Context, req QuerySubDevicesRequest) (QuerySubDevicesResponse, error)
	QueryDeviceFactoryInfos(ctx context.Context, req QueryDeviceFactoryInfosRequest) (QueryDeviceFactoryInfosResponse, error)
	UpdateDeviceName(ctx context.Context, req UpdateDeviceNameRequest) (UpdateDeviceNameResponse, error)
	AddDeviceUser(ctx context.Context, req AddDeviceUserRequest) (AddDeviceUserResponse, error)
	UpdateDeviceUser(ctx context.Context, req UpdateDeviceUserRequest) (UpdateDeviceUserResponse, error)
	DeleteDeviceUser(ctx context.Context, req DeleteDeviceUserRequest) (DeleteDeviceUserResponse, error)
	GetDeviceUser(ctx context.Context, req GetDeviceUserRequest) (GetDeviceUserResponse, error)
	ListDeviceUsers(ctx context.Context, req ListDeviceUsersRequest) (ListDeviceUsersResponse, error)
	UpdateMultiOutletName(ctx context.Context, req UpdateMultiOutletNameRequest) (UpdateMultiOutletNameResponse, error)
	ListMultiOutletNames(ctx context.Context, req ListMultiOutletNamesRequest) (ListMultiOutletNamesResponse, error)
}

// MessageQueue defines the interface for Tuya message queue operations.
type MessageQueue interface {
	Start(ctx context.Context, req MessageQueueStartRequest) (MessageQueueStartResponse, error)
	Stop(ctx context.Context, req MessageQueueStopRequest) (MessageQueueStopResponse, error)

	GetMessageQueueConfig(ctx context.Context) (MessageQueueConfig, error)
	RefreshMQ(ctx context.Context, req RefreshMQRequest) (RefreshMQResponse, error)
	AddMessageListener(ctx context.Context, req AddMessageListenerRequest) (AddMessageListenerResponse, error)
	RemoveMessageListener(ctx context.Context, req RemoveMessageListenerRequest) (RemoveMessageListenerResponse, error)
}

// Error represents the standard error interface for Tuya API errors.
type Error interface {
	//https://developer.tuya.com/en/docs/iot/error-code
	// Standard error codes for the tuya API
	GetCode() int
	GetMessage() string
}

// Data models section

// Home represents a smart home containing devices and other attributes.
type Home struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	GeoName    string `json:"geo_name"`
	TimeZone   string `json:"time_zone"`
	CreateTime int64  `json:"create_time"`
	UpdateTime int64  `json:"update_time"`
}

// Device represents a smart device returned by the Tuya API.
type Device struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	LocalKey    string `json:"local_key"`
	Category    string `json:"category"`
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	SubCategory string `json:"sub_category"`
	Icon        string `json:"icon"`
	IP          string `json:"ip"`
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	Model       string `json:"model"`
	TimeZone    string `json:"time_zone"`
	ActiveTime  int64  `json:"active_time"`
	CreateTime  int64  `json:"create_time"`
	UpdateTime  int64  `json:"update_time"`
	Online      bool   `json:"online"`
	// OnlinePresent distinguishes an explicit offline state from an omitted observation.
	OnlinePresent bool     `json:"-"`
	Status        []Status `json:"status"`
	Capabilities  []string `json:"capabilities"`
}

// Status represents the current status of a device property.
type Status struct {
	Code  string `json:"code"`
	Value any    `json:"value"`
}

// Scene represents a Tuya smart scene.
type Scene struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	HomeID  string        `json:"home_id"`
	Enabled bool          `json:"enabled"`
	Actions []SceneAction `json:"actions"`
}

// SceneAction represents an action within a scene.
type SceneAction struct {
	DeviceID string         `json:"device_id"`
	Commands map[string]any `json:"commands"`
}

// Session is an account-scoped API and connection lifecycle.
type Session struct {
	HTTPClient *http.Client
	tokenMu    sync.RWMutex
	tokens     Tokens

	// Generic Client configurations.
	// Cloud API configurations.
	CloudAPIURL string
	// Unique client identifier for the client using the library
	ClientID string
	// URL for the authentication endpoints. This is the same for all regions.
	AuthenticationURL string

	// Service implementation details.
	EncryptedClient *EncryptedClient
	AuthService     *AuthService
	DevicesService  *DevicesService
	HomeService     *HomeService
	UserService     *UserService
	SceneService    *SceneService
	// Used for message queue operations
	MessageQueue *SharingMessageQueueImpl

	mqttClientFactory MQTTClientFactory
	rtcSignaling      RTCSignaling
}

// SharingMessageQueueImpl implements the MessageQueue interface.
type SharingMessageQueueImpl struct {
	Client *Session
	State  *mqttState
}

// CustomerTokenInfo contains customer token information.
type CustomerTokenInfo struct {
	// Token that can be used to access the service underlying the API.
	AccessToken string `json:"access_token"`
	// Token that can be used to generate a new access token.
	RefreshToken string `json:"refresh_token"`
	// Time of expiry in milliseconds since epoch format.
	ExpireTime int64 `json:"expire_time"`
	// Unique identifier for the token.
	UID string `json:"uid"`
	// Time of initialization in milliseconds since epoch format.
	T int64 `json:"t"`
}

// Manager API Request/Response types

// LoginRequest represents a request to generate a QR code for login.
type LoginRequest struct {
	// (required - for tuya HA auth schema) Access code that is unique to the user.
	// https://www.home-assistant.io/integrations/tuya/ instructions for the token are available here.
	AccessCode string

	// (Required) Authorization schema for accessing the users data
	Schema string
}

// OperationRequest represents any request that can return its underlying Request.
type OperationRequest interface {
	GetRequest() Request
}

// Request represents the embedded request context that is used to perform the request.
type Request struct {
	AuthorizationContext *AuthorizationContext
	// DoNotRefreshToken is kept for source compatibility. Token refresh is never
	// performed implicitly by this package.
	//
	// Deprecated: call AuthService.RefreshToken explicitly.
	DoNotRefreshToken bool
}

// AuthorizationContext supplies request-specific credentials that override
// the credentials assigned to the containing Session.
type AuthorizationContext struct {
	AccessToken  string
	RefreshToken string
	ExpireTime   int64
}

// ValidateLoginCodeRequest is used to validate the login code and get the access token.
// The Token is generated using the GenerateQrCodeForLogin method.
// The token is validated by the user scanning the QR code.
type ValidateLoginCodeRequest struct {
	Request

	// The login code is the code that is generated by the GenerateQrCodeForLogin method.
	LoginCode string `json:"token"`
	// The user code is the access code that is unique to the user that the customer grabs from the smart life app.
	UserCode string `json:"user_code"`
}

// GetRequest returns the underlying Request for the ValidateLoginCodeRequest.
func (r *ValidateLoginCodeRequest) GetRequest() Request {
	return r.Request
}

// ValidateLoginCodeResponse represents the response from validating a login code.
type ValidateLoginCodeResponse struct {
	Success      bool   `json:"success"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpireTime   int64  `json:"expire_time"`
	TerminalID   string `json:"terminal_id"`
	UID          string `json:"uid"`
	Username     string `json:"username"`
	Endpoint     string `json:"endpoint"`
}

// LoginResponse represents the response from generating a QR code for login.
type LoginResponse struct {
	// The raw code data that should be used to call into the Tuya cloud to validate the results of the auth request.
	Code string `json:"code"`
	// The QR code formatted code is the code that should be written to the QR code.
	QrFormattedCode string `json:"qr_formatted_code"`
}

// UpdateDeviceCacheRequest represents a request to update the device cache.
type UpdateDeviceCacheRequest struct {
	DeviceIDs []string `json:"device_ids,omitempty"`
}

// UpdateDeviceCacheResponse represents the response from updating device cache.
type UpdateDeviceCacheResponse struct {
	Message string `json:"message,omitempty"`
}

// RefreshMQRequest represents a request to refresh the message queue.
type RefreshMQRequest struct {
}

// RefreshMQResponse represents the response from refreshing the message queue.
type RefreshMQResponse struct {
	Message string `json:"message,omitempty"`
}

// SendCommandsRequest represents a request to send commands to a device.
type SendCommandsRequest struct {
	Request

	DeviceID string    `json:"device_id"`
	Commands []Command `json:"commands"`
}

// Command represents a single command to send to a device.
type Command struct {
	Code  string `json:"code"`
	Value any    `json:"value"`
}

// GetRequest returns the underlying Request for the SendCommandsRequest.
func (r *SendCommandsRequest) GetRequest() Request {
	return r.Request
}

// SendCommandsResponse represents the response from sending commands to a device.
type SendCommandsResponse struct {
	Result  bool   `json:"result,omitempty"`
	Message string `json:"message,omitempty"`
}

// GetDeviceStreamAllocateRequest represents a request to allocate a device stream.
type GetDeviceStreamAllocateRequest struct {
	Request

	DeviceID string `json:"device_id"`
}

// GetRequest returns the underlying Request for the GetDeviceStreamAllocateRequest.
func (r *GetDeviceStreamAllocateRequest) GetRequest() Request {
	return r.Request
}

// GetDeviceStreamAllocateResponse represents the response from allocating a device stream.
type GetDeviceStreamAllocateResponse struct {
	StreamURL string `json:"stream_url,omitempty"`
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
}

// StartRTCStreamRequest contains parameters for starting a WebRTC stream with a Tuya camera.
type StartRTCStreamRequest struct {
	Request

	DeviceID string `json:"device_id"`
	SDPOffer string `json:"sdp_offer"`
}

// GetRequest returns the underlying Request for the StartRTCStreamRequest.
func (r *StartRTCStreamRequest) GetRequest() Request {
	return r.Request
}

// StopRTCStreamRequest contains parameters for stopping a WebRTC stream.
type StopRTCStreamRequest struct {
	Request

	SessionID string `json:"session_id"`
	DeviceID  string `json:"device_id"`
}

// GetRequest returns the underlying Request for the StopRTCStreamRequest.
func (r *StopRTCStreamRequest) GetRequest() Request {
	return r.Request
}

// QueryScenesRequest represents a request to query scenes for a home.
type QueryScenesRequest struct {
	Request

	HomeID string `json:"home_id,omitempty"`
}

// GetRequest returns the underlying Request for the QueryScenesRequest.
func (r *QueryScenesRequest) GetRequest() Request {
	return r.Request
}

// QueryScenesResponse represents the response from querying scenes.
type QueryScenesResponse struct {
	Scenes  []Scene `json:"scenes"`
	Message string  `json:"message,omitempty"`
}

// TriggerSceneRequest represents a request to trigger a scene.
type TriggerSceneRequest struct {
	Request

	SceneID string `json:"scene_id"`
}

// GetRequest returns the underlying Request for the TriggerSceneRequest.
func (r *TriggerSceneRequest) GetRequest() Request {
	return r.Request
}

// TriggerSceneResponse represents the response from triggering a scene.
type TriggerSceneResponse struct {
	Message string `json:"message,omitempty"`
}

// AddDeviceListenerRequest represents a request to add a device listener.
type AddDeviceListenerRequest struct {
	DeviceID string `json:"device_id"`
	Callback func(deviceID string, status Event)
}

// AddDeviceListenerResponse represents the response from adding a device listener.
type AddDeviceListenerResponse struct {
	Message string `json:"message,omitempty"`
}

// RemoveDeviceListenerRequest represents a request to remove a device listener.
type RemoveDeviceListenerRequest struct {
	DeviceID string `json:"device_id"`
}

// RemoveDeviceListenerResponse represents the response from removing a device listener.
type RemoveDeviceListenerResponse struct {
	Message string `json:"message,omitempty"`
}

// DeviceRepository API Request/Response types

// QueryDevicesByHomeRequest represents a request to query devices by their home ID.
type QueryDevicesByHomeRequest struct {
	Request

	HomeID string `json:"home_id"`
}

// GetRequest returns the underlying Request for the QueryDevicesByHomeRequest.
func (r *QueryDevicesByHomeRequest) GetRequest() Request {
	return r.Request
}

// QueryDevicesByHomeResponse represents the response from querying devices by their home ID.
type QueryDevicesByHomeResponse struct {
	Results []Device `json:"devices"`
	Success bool     `json:"success"`
	Message string   `json:"message,omitempty"`
}

// QueryDevicesByIDsRequest represents a request to query devices by their IDs.
type QueryDevicesByIDsRequest struct {
	Request

	DeviceIDs []string `json:"device_ids"`
}

// GetRequest returns the underlying Request for the QueryDevicesByIDsRequest.
func (r *QueryDevicesByIDsRequest) GetRequest() Request {
	return r.Request
}

// QueryDevicesByIDsResponse represents the response from querying devices by their IDs.
type QueryDevicesByIDsResponse struct {
	Results []Device `json:"devices"`
}

// QueryDeviceStatusRequest represents a request to query the status of a device.
type QueryDeviceStatusRequest struct {
	Request

	DeviceID string `json:"device_id"`
}

// GetRequest returns the underlying Request for the QueryDeviceStatusRequest.
func (r *QueryDeviceStatusRequest) GetRequest() Request {
	return r.Request
}

// QueryDeviceStatusResponse represents the response from querying the status of a device.
type QueryDeviceStatusResponse struct {
	Status  []DeviceStatusMapping `json:"status"`
	Message string                `json:"message,omitempty"`
}

// DeviceStatusMapping represents a status mapping for a device.
type DeviceStatusMapping struct {
	// this is the tuya code for the status. It matches to the code that you can use to send commands to the device.
	// https://developer.tuya.com/en/docs/cloud/device-control#title-35-Send%20instructions%20to%20the%20device
	//nolint:tagliatelle // Tuya defines these provider wire keys in camelCase.
	DPCode string `json:"dpCode"`
	//nolint:tagliatelle // Tuya defines these provider wire keys in camelCase.
	DPId int `json:"dpId"`
	//	EnumMappingMap map[string]interface{} `json:"enumMappingMap"`
	//nolint:tagliatelle // Tuya defines this provider wire key in camelCase.
	StatusCode string `json:"statusCode"`
	// StatusFormat   map[string]interface{} `json:"statusFormat"`
	//nolint:tagliatelle // Tuya defines these provider wire keys in camelCase.
	SupportLocal bool `json:"supportLocal"`
	//nolint:tagliatelle // Tuya defines these provider wire keys in camelCase.
	ValueConvert string `json:"valueConvert"`
	// Value descriptions, status mappings and enum mappings are strings of jsons....., need to do double step parsing to map them.
	//	ValueDesc      map[string]interface{} `json:"valueDesc"`
	//nolint:tagliatelle // Tuya defines this provider wire key in camelCase.
	ValueType string `json:"valueType"`
}

// QueryDeviceSpecificationRequest represents a request to query the specification for a device.
type QueryDeviceSpecificationRequest struct {
	Request

	DeviceID string `json:"device_id"`
}

// GetRequest returns the underlying Request for the QueryDeviceSpecificationRequest
// https://developer.tuya.com/en/docs/cloud/68c2e82f73?id=Kag2ybtxwlb9w
func (r *QueryDeviceSpecificationRequest) GetRequest() Request {
	return r.Request
}

// QueryDeviceSpecificationResponse represents the response from querying the specification for a device.
// https://developer.tuya.com/en/docs/cloud/68c2e82f73?id=Kag2ybtxwlb9w
// Return example:
//
//	{
//	    "result": {
//	        "functions": [
//	            {
//	                "code": "switch",
//	                "values": "{}",
//	                "type": "Boolean",
//	                "name": "Switch",
//	                "desc": "{}"
//	            }
//	        ],
//	        "category": "dj",
//	        "status": [
//	            {
//	                "code": "switch_led",
//	                "values": "{}",
//	                "type": "Boolean",
//	                "name": "switch"
//	            }
//	        ]
//	    },
//	    "t": 1591872112140,
//	    "success": true
//	}
type QueryDeviceSpecificationResponse struct {
	Specification Specification `json:"specification"`
}

// SpecificationFunction represents a function within a specification.
type SpecificationFunction struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Desc   string `json:"desc"`
	Type   string `json:"type"`
	Values any    `json:"values"`
}

// SpecificationStatus represents a status within a specification.
type SpecificationStatus struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Values any    `json:"values"`
}

// Specification represents a specification for a device.
type Specification struct {
	Functions []SpecificationFunction `json:"functions"`
	Status    []SpecificationStatus   `json:"status"`
	// The tuya category of the device, this is a code that
	// defines a specific instruction set that the endpoint can support
	// https://developer.tuya.com/en/docs/iot/categorytgkg?id=Kaiuz0ktx7m0o
	Category string `json:"category"`
}

// Device management API request/response types

// GetDeviceDetailsRequest queries details for a specific device.
type GetDeviceDetailsRequest struct {
	Request

	DeviceID string `json:"device_id"`
}

// GetRequest returns the underlying Request for the GetDeviceDetailsRequest.
func (r *GetDeviceDetailsRequest) GetRequest() Request {
	return r.Request
}

// GetDeviceDetailsResponse contains the device details payload.
type GetDeviceDetailsResponse struct {
	Device Device `json:"device"`
}

// QueryDevicesByUserRequest queries devices assigned to a Tuya UID.
type QueryDevicesByUserRequest struct {
	Request

	UID      string `json:"uid"`
	From     string `json:"from,omitempty"`
	PageNo   *int   `json:"page_no,omitempty"`
	PageSize *int   `json:"page_size,omitempty"`
}

// GetRequest returns the underlying Request for the QueryDevicesByUserRequest.
func (r *QueryDevicesByUserRequest) GetRequest() Request {
	return r.Request
}

// QueryDevicesByUserResponse contains the devices for a UID.
type QueryDevicesByUserResponse struct {
	Devices []Device `json:"devices"`
}

// QueryDevicesRequest queries devices by application/product filters.
type QueryDevicesRequest struct {
	Request

	PageNo    int      `json:"page_no"`
	PageSize  int      `json:"page_size"`
	Schema    string   `json:"schema,omitempty"`
	ProductID string   `json:"product_id,omitempty"`
	DeviceIDs []string `json:"device_ids,omitempty"`
	StartTime string   `json:"start_time,omitempty"`
	EndTime   string   `json:"end_time,omitempty"`
	LastID    string   `json:"last_id,omitempty"`
}

// GetRequest returns the underlying Request for the QueryDevicesRequest.
func (r *QueryDevicesRequest) GetRequest() Request {
	return r.Request
}

// QueryDevicesResponse represents the paginated device list response.
type QueryDevicesResponse struct {
	Devices []Device `json:"devices"`
	Total   int64    `json:"total"`
	LastID  string   `json:"last_id,omitempty"`
}

// UpdateDeviceFunctionNameRequest updates a DP/function display name.
type UpdateDeviceFunctionNameRequest struct {
	Request

	DeviceID     string `json:"device_id"`
	FunctionCode string `json:"function_code"`
	Name         string `json:"name"`
}

// GetRequest returns the underlying Request for the UpdateDeviceFunctionNameRequest.
func (r *UpdateDeviceFunctionNameRequest) GetRequest() Request {
	return r.Request
}

// UpdateDeviceFunctionNameResponse indicates success of the rename operation.
type UpdateDeviceFunctionNameResponse struct {
	Result bool `json:"result"`
}

// QueryDeviceLogsRequest fetches device logs.
type QueryDeviceLogsRequest struct {
	Request

	DeviceID      string   `json:"device_id"`
	Types         []string `json:"types"`
	StartTime     int64    `json:"start_time"`
	EndTime       int64    `json:"end_time"`
	Codes         []string `json:"codes,omitempty"`
	StartRowKey   string   `json:"start_row_key,omitempty"`
	LastRowKey    string   `json:"last_row_key,omitempty"`
	LastEventTime *int64   `json:"last_event_time,omitempty"`
	Size          *int     `json:"size,omitempty"`
	QueryType     *int     `json:"query_type,omitempty"`
}

// GetRequest returns the underlying Request for the QueryDeviceLogsRequest.
func (r *QueryDeviceLogsRequest) GetRequest() Request {
	return r.Request
}

// QueryDeviceLogsResponse contains log entries and cursor metadata.
type QueryDeviceLogsResponse struct {
	Logs          []DeviceLogEntry `json:"logs"`
	HasNext       bool             `json:"has_next"`
	DeviceID      string           `json:"device_id"`
	CurrentRowKey string           `json:"current_row_key,omitempty"`
	NextRowKey    string           `json:"next_row_key,omitempty"`
	Count         int64            `json:"count,omitempty"`
}

// DeviceLogEntry represents a single device log entry.
type DeviceLogEntry struct {
	Code      string `json:"code"`
	Value     any    `json:"value"`
	EventTime int64  `json:"event_time"`
	EventFrom string `json:"event_from"`
	EventID   int    `json:"event_id"`
	Status    string `json:"status,omitempty"`
	Row       string `json:"row,omitempty"`
}

// ResetDeviceFactoryDefaultsRequest resets a device to factory default.
type ResetDeviceFactoryDefaultsRequest struct {
	Request

	DeviceID string `json:"device_id"`
}

// GetRequest returns the underlying Request for the ResetDeviceFactoryDefaultsRequest.
func (r *ResetDeviceFactoryDefaultsRequest) GetRequest() Request {
	return r.Request
}

// ResetDeviceFactoryDefaultsResponse indicates reset success.
type ResetDeviceFactoryDefaultsResponse struct {
	Result bool `json:"result"`
}

// DeleteDeviceRequest deletes a device.
type DeleteDeviceRequest struct {
	Request

	DeviceID string `json:"device_id"`
}

// GetRequest returns the underlying Request for the DeleteDeviceRequest.
func (r *DeleteDeviceRequest) GetRequest() Request {
	return r.Request
}

// DeleteDeviceResponse indicates delete success.
type DeleteDeviceResponse struct {
	Result bool `json:"result"`
}

// QuerySubDevicesRequest queries sub devices under a gateway.
type QuerySubDevicesRequest struct {
	Request

	DeviceID string `json:"device_id"`
}

// GetRequest returns the underlying Request for the QuerySubDevicesRequest.
func (r *QuerySubDevicesRequest) GetRequest() Request {
	return r.Request
}

// QuerySubDevicesResponse contains sub device information.
type QuerySubDevicesResponse struct {
	Devices []SubDevice `json:"devices"`
}

// SubDevice captures the minimal sub-device information.
type SubDevice struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Online     bool   `json:"online"`
	OwnerID    string `json:"owner_id"`
	Category   string `json:"category"`
	ProductID  string `json:"product_id"`
	ActiveTime int64  `json:"active_time"`
	UpdateTime int64  `json:"update_time"`
}

// QueryDeviceFactoryInfosRequest retrieves device factory info.
type QueryDeviceFactoryInfosRequest struct {
	Request

	DeviceIDs []string `json:"device_ids"`
}

// GetRequest returns the underlying Request for the QueryDeviceFactoryInfosRequest.
func (r *QueryDeviceFactoryInfosRequest) GetRequest() Request {
	return r.Request
}

// QueryDeviceFactoryInfosResponse contains device factory metadata.
type QueryDeviceFactoryInfosResponse struct {
	Devices []DeviceFactoryInfo `json:"devices"`
}

// DeviceFactoryInfo represents factory metadata for a device.
type DeviceFactoryInfo struct {
	ID   string `json:"id"`
	UUID string `json:"uuid"`
	SN   string `json:"sn"`
	MAC  string `json:"mac"`
}

// UpdateDeviceNameRequest updates top level device name.
type UpdateDeviceNameRequest struct {
	Request

	DeviceID string `json:"device_id"`
	Name     string `json:"name"`
}

// GetRequest returns the underlying Request for the UpdateDeviceNameRequest.
func (r *UpdateDeviceNameRequest) GetRequest() Request {
	return r.Request
}

// UpdateDeviceNameResponse indicates rename success.
type UpdateDeviceNameResponse struct {
	Result bool `json:"result"`
}

// AddDeviceUserRequest creates a user profile on a device.
type AddDeviceUserRequest struct {
	Request

	DeviceID string `json:"device_id"`
	NickName string `json:"nick_name"`
	Sex      int    `json:"sex"`
	Birthday *int64 `json:"birthday,omitempty"`
	Height   *int   `json:"height,omitempty"`
	Weight   *int   `json:"weight,omitempty"`
	Contact  string `json:"contact,omitempty"`
}

// GetRequest returns the underlying Request for the AddDeviceUserRequest.
func (r *AddDeviceUserRequest) GetRequest() Request {
	return r.Request
}

// AddDeviceUserResponse holds the created user ID.
type AddDeviceUserResponse struct {
	UserID string `json:"user_id"`
}

// UpdateDeviceUserRequest updates device user metadata.
type UpdateDeviceUserRequest struct {
	Request

	DeviceID string `json:"device_id"`
	UserID   string `json:"user_id"`
	NickName string `json:"nick_name"`
	Sex      int    `json:"sex"`
	Birthday *int64 `json:"birthday,omitempty"`
	Height   *int   `json:"height,omitempty"`
	Weight   *int   `json:"weight,omitempty"`
	Contact  string `json:"contact,omitempty"`
}

// GetRequest returns the underlying Request for the UpdateDeviceUserRequest.
func (r *UpdateDeviceUserRequest) GetRequest() Request {
	return r.Request
}

// UpdateDeviceUserResponse indicates update success.
type UpdateDeviceUserResponse struct {
	Result bool `json:"result"`
}

// DeleteDeviceUserRequest removes a user from a device.
type DeleteDeviceUserRequest struct {
	Request

	DeviceID string `json:"device_id"`
	UserID   string `json:"user_id"`
}

// GetRequest returns the underlying Request for the DeleteDeviceUserRequest.
func (r *DeleteDeviceUserRequest) GetRequest() Request {
	return r.Request
}

// DeleteDeviceUserResponse indicates delete success.
type DeleteDeviceUserResponse struct {
	Result bool `json:"result"`
}

// GetDeviceUserRequest fetches a user by ID.
type GetDeviceUserRequest struct {
	Request

	DeviceID string `json:"device_id"`
	UserID   string `json:"user_id"`
}

// GetRequest returns the underlying Request for the GetDeviceUserRequest.
func (r *GetDeviceUserRequest) GetRequest() Request {
	return r.Request
}

// GetDeviceUserResponse contains a specific user.
type GetDeviceUserResponse struct {
	User DeviceUser `json:"user"`
}

// ListDeviceUsersRequest lists users associated with a device.
type ListDeviceUsersRequest struct {
	Request

	DeviceID string `json:"device_id"`
}

// GetRequest returns the underlying Request for the ListDeviceUsersRequest.
func (r *ListDeviceUsersRequest) GetRequest() Request {
	return r.Request
}

// ListDeviceUsersResponse contains device user list.
type ListDeviceUsersResponse struct {
	Users []DeviceUser `json:"users"`
}

// DeviceUser defines the device user payload.
type DeviceUser struct {
	DeviceID string `json:"device_id"`
	NickName string `json:"nick_name"`
	Sex      int    `json:"sex"`
	Birthday int64  `json:"birthday"`
	Height   int    `json:"height"`
	Weight   int    `json:"weight"`
	Contact  string `json:"contact"`
	UserID   string `json:"user_id,omitempty"`
}

// UpdateMultiOutletNameRequest updates the name for a single outlet.
type UpdateMultiOutletNameRequest struct {
	Request

	DeviceID   string `json:"device_id"`
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
}

// GetRequest returns the underlying Request for the UpdateMultiOutletNameRequest.
func (r *UpdateMultiOutletNameRequest) GetRequest() Request {
	return r.Request
}

// UpdateMultiOutletNameResponse indicates update success.
type UpdateMultiOutletNameResponse struct {
	Result bool `json:"result"`
}

// ListMultiOutletNamesRequest lists outlet identifiers/names.
type ListMultiOutletNamesRequest struct {
	Request

	DeviceID string `json:"device_id"`
}

// GetRequest returns the underlying Request for the ListMultiOutletNamesRequest.
func (r *ListMultiOutletNamesRequest) GetRequest() Request {
	return r.Request
}

// ListMultiOutletNamesResponse contains outlet metadata.
type ListMultiOutletNamesResponse struct {
	Names []MultiOutletName `json:"names"`
}

// MultiOutletName describes a single outlet mapping.
type MultiOutletName struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
}

// HomeRepository API Request/Response types

// QueryHomesRequest represents a request to query homes.
type QueryHomesRequest struct {
	Request
}

// GetRequest returns the underlying Request for the QueryHomesRequest.
func (r *QueryHomesRequest) GetRequest() Request {
	return r.Request
}

// QueryHomesResponse represents the response from querying homes.
type QueryHomesResponse struct {
	Results []Home `json:"homes"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// CustomerApi Request/Response types

// CustomerAPIRequest represents a request to the customer API.
type CustomerAPIRequest struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
}

// CustomerAPIResponse represents the response from the customer API request.
type CustomerAPIResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       any               `json:"body,omitempty"`
	Success    bool              `json:"success"`
	Message    string            `json:"message,omitempty"`
}

// SharingMQ Request/Response types

// MessageQueueStartRequest represents a request to start the message queue.
type MessageQueueStartRequest struct {
	Request
}

// GetRequest returns the underlying Request for the MessageQueueStartRequest.
func (r *MessageQueueStartRequest) GetRequest() Request {
	return r.Request
}

// MessageQueueStartResponse represents the response from starting the message queue.
type MessageQueueStartResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// MessageQueueStopRequest represents a request to stop the message queue.
type MessageQueueStopRequest struct {
	Request
}

// GetRequest returns the underlying Request for the MessageQueueStopRequest.
func (r *MessageQueueStopRequest) GetRequest() Request {
	return r.Request
}

// MessageQueueStopResponse represents the response from stopping the message queue.
type MessageQueueStopResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// AddMessageListenerRequest represents a request to add a message listener.
type AddMessageListenerRequest struct {
	Request

	Topic    string `json:"topic"`
	Callback func(topic string, message any)
}

// GetRequest returns the underlying Request for the AddMessageListenerRequest.
func (r *AddMessageListenerRequest) GetRequest() Request {
	return r.Request
}

// AddMessageListenerResponse represents the response from adding a message listener.
type AddMessageListenerResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// RemoveMessageListenerRequest represents a request to remove a message listener.
type RemoveMessageListenerRequest struct {
	Request

	Topic string `json:"topic"`
}

// GetRequest returns the underlying Request for the RemoveMessageListenerRequest.
func (r *RemoveMessageListenerRequest) GetRequest() Request {
	return r.Request
}

// RemoveMessageListenerResponse represents the response from removing a message listener.
type RemoveMessageListenerResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// Compile-time assertions ensure service interfaces are implemented correctly.
var _ Auth = (*AuthService)(nil)
var _ Homes = (*HomeService)(nil)
var _ Devices = (*DevicesService)(nil)
var _ MessageQueue = (*SharingMessageQueueImpl)(nil)

// Declare that the Request struct implements the OperationRequest interface.
var _ OperationRequest = (*ValidateLoginCodeRequest)(nil)
var _ OperationRequest = (*QueryDevicesByHomeRequest)(nil)
var _ OperationRequest = (*QueryDevicesByIDsRequest)(nil)
var _ OperationRequest = (*QueryHomesRequest)(nil)
var _ OperationRequest = (*QueryScenesRequest)(nil)
var _ OperationRequest = (*TriggerSceneRequest)(nil)
var _ OperationRequest = (*SendCommandsRequest)(nil)
var _ OperationRequest = (*GetDeviceStreamAllocateRequest)(nil)
var _ OperationRequest = (*QueryDeviceStatusRequest)(nil)
var _ OperationRequest = (*QueryDeviceSpecificationRequest)(nil)
var _ OperationRequest = (*MessageQueueStartRequest)(nil)
var _ OperationRequest = (*MessageQueueStopRequest)(nil)
var _ OperationRequest = (*AddMessageListenerRequest)(nil)
var _ OperationRequest = (*RemoveMessageListenerRequest)(nil)
var _ OperationRequest = (*GetDeviceDetailsRequest)(nil)
var _ OperationRequest = (*QueryDevicesByUserRequest)(nil)
var _ OperationRequest = (*QueryDevicesRequest)(nil)
var _ OperationRequest = (*UpdateDeviceFunctionNameRequest)(nil)
var _ OperationRequest = (*QueryDeviceLogsRequest)(nil)
var _ OperationRequest = (*ResetDeviceFactoryDefaultsRequest)(nil)
var _ OperationRequest = (*DeleteDeviceRequest)(nil)
var _ OperationRequest = (*QuerySubDevicesRequest)(nil)
var _ OperationRequest = (*QueryDeviceFactoryInfosRequest)(nil)
var _ OperationRequest = (*UpdateDeviceNameRequest)(nil)
var _ OperationRequest = (*AddDeviceUserRequest)(nil)
var _ OperationRequest = (*UpdateDeviceUserRequest)(nil)
var _ OperationRequest = (*DeleteDeviceUserRequest)(nil)
var _ OperationRequest = (*GetDeviceUserRequest)(nil)
var _ OperationRequest = (*ListDeviceUsersRequest)(nil)
var _ OperationRequest = (*UpdateMultiOutletNameRequest)(nil)
var _ OperationRequest = (*ListMultiOutletNamesRequest)(nil)
var _ OperationRequest = (*StartRTCStreamRequest)(nil)
var _ OperationRequest = (*StopRTCStreamRequest)(nil)
