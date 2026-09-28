package tuya

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/portpowered/go-tuya/pkg/tuya/internal/wire"
)

// DevicesService provides methods for managing smart devices
type DevicesService service

// QueryDevicesByHome fetches all devices for a given home.
// TODO: there is actually a mechanism inside of here to basically fetch across like each device and get their capabilities.
// This is done individually for each device, along with their states, which is a very expensive operation generally.
func (c *DevicesService) QueryDevicesByHome(ctx context.Context, req QueryDevicesByHomeRequest) (QueryDevicesByHomeResponse, error) {
	resp, err := c.client.EncryptedClient.Get(ctx, wire.RouteQueryHomeDevices, map[string]interface{}{
		"homeId": req.HomeID,
	}, &req)
	if err != nil {
		return QueryDevicesByHomeResponse{}, err
	}

	tuyaResponse, err := serialize[[]DeviceResponseResultElement](resp)
	if err != nil {
		return QueryDevicesByHomeResponse{}, err
	}

	return QueryDevicesByHomeResponse{
		Results: mapResults(tuyaResponse),
	}, nil
}

// QueryDevicesByHomeAssistantDevices fetches all devices for a given home using the Home Assistant devices endpoint.
func (c *DevicesService) QueryDevicesByHomeAssistantDevices(ctx context.Context, req QueryDevicesByHomeRequest) (QueryDevicesByHomeResponse, error) {
	resp, err := c.client.EncryptedClient.Get(ctx, wire.RouteQueryHomeDevices, map[string]interface{}{
		"homeId": req.HomeID,
	}, &req)
	if err != nil {
		return QueryDevicesByHomeResponse{}, err
	}

	tuyaResponse, err := serialize[[]DeviceResponseResultElement](resp)
	if err != nil {
		return QueryDevicesByHomeResponse{}, err
	}

	return QueryDevicesByHomeResponse{
		Results: mapResults(tuyaResponse),
	}, nil
}

func mapResults(results []DeviceResponseResultElement) []Device {
	devices := make([]Device, len(results))
	for i, result := range results {
		devices[i] = Device{
			ID:          result.ID,
			LocalKey:    result.LocalKey,
			Name:        result.Name,
			Category:    result.Category,
			ProductID:   result.ProductID,
			ProductName: result.ProductName,
			SubCategory: result.SubCategory,
			Icon:        result.Icon,
			IP:          result.IP,
			Lat:         result.Lat,
			Lon:         result.Lon,
			TimeZone:    result.TimeZone,
			ActiveTime:  result.ActiveTime,
			CreateTime:  result.CreateTime,
			UpdateTime:  result.UpdateTime,
			Online:      result.Online,
			Status:      result.Status,
		}
	}
	return devices
}

func mapGeneratedDeviceRecords(records []wire.DeviceRecord) []Device {
	devices := make([]Device, len(records))
	for i, record := range records {
		devices[i] = Device{
			ID:           dereference(record.Id),
			LocalKey:     dereference(record.LocalKey),
			Name:         dereference(record.Name),
			Category:     dereference(record.Category),
			ProductID:    dereference(record.ProductId),
			ProductName:  dereference(record.ProductName),
			SubCategory:  additionalString(record.AdditionalProperties, "subCategory"),
			Icon:         dereference(record.Icon),
			IP:           dereference(record.Ip),
			Lat:          additionalString(record.AdditionalProperties, "lat"),
			Lon:          additionalString(record.AdditionalProperties, "lon"),
			Model:        dereference(record.Model),
			TimeZone:     dereference(record.TimeZone),
			ActiveTime:   dereference(record.ActiveTime),
			CreateTime:   dereference(record.CreateTime),
			UpdateTime:   dereference(record.UpdateTime),
			Online:       dereference(record.Online),
			Status:       mapGeneratedDeviceStatus(record.Status),
			Capabilities: nil,
		}
	}
	return devices
}

func mapGeneratedDeviceStatus(statuses *[]wire.DeviceStatus) []Status {
	if statuses == nil {
		return nil
	}

	result := make([]Status, len(*statuses))
	for i, status := range *statuses {
		result[i] = Status{Code: dereference(status.Code), Value: status.Value}
	}
	return result
}

func additionalString(properties map[string]interface{}, name string) string {
	value, _ := properties[name].(string)
	return value
}

func dereference[T any](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}

// DeviceResponseResult represents the response structure for device queries
type DeviceResponseResult struct {
	Result []DeviceResponseResultElement `json:"result"`
}

// DeviceResponseResultElement represents a single device in the response
type DeviceResponseResultElement struct {
	ID          string   `json:"id"`
	UUID        string   `json:"uuid"`
	Name        string   `json:"name"`
	OwnerID     string   `json:"owner_id"`
	UID         string   `json:"uid"`
	ProductID   string   `json:"product_id"`
	ProductName string   `json:"product_name"`
	SubCategory string   `json:"subCategory,omitempty"`
	Category    string   `json:"category"`
	Icon        string   `json:"icon"`
	IP          string   `json:"ip"`
	LocalKey    string   `json:"local_key"`
	Online      bool     `json:"online"`
	Sub         bool     `json:"sub"`
	BizType     int      `json:"biz_type"`
	ActiveTime  int64    `json:"active_time"`
	CreateTime  int64    `json:"create_time"`
	UpdateTime  int64    `json:"update_time"`
	TimeZone    string   `json:"time_zone"`
	Lat         string   `json:"lat"`
	Lon         string   `json:"lon"`
	Status      []Status `json:"status"`
}

// QueryDevicesByIDs fetches devices by their IDs
func (c *DevicesService) QueryDevicesByIDs(ctx context.Context, req QueryDevicesByIDsRequest) (QueryDevicesByIDsResponse, error) {
	resp, err := c.client.EncryptedClient.Get(ctx, wire.RouteQueryHomeDevices, map[string]interface{}{
		"deviceIds": strings.Join(req.DeviceIDs, ","),
	}, &req)
	if err != nil {
		return QueryDevicesByIDsResponse{}, err
	}

	tuyaResponse, err := serialize[[]DeviceResponseResultElement](resp)
	if err != nil {
		return QueryDevicesByIDsResponse{}, err
	}

	// return QueryDevicesByHomeResponse{
	// 	Devices: devices,
	// }, nil
	// TODO: implement device querying by home
	return QueryDevicesByIDsResponse{
		Results: mapResults(tuyaResponse),
	}, nil
}

// SendCommands sends control commands to a device
// https://developer.tuya.com/en/docs/cloud/device-control?id=K95zu01ksols7
func (c *DevicesService) SendCommands(ctx context.Context, req SendCommandsRequest) (SendCommandsResponse, error) {
	resp, err := c.client.EncryptedClient.Post(ctx, fmt.Sprintf(wire.RouteSendDeviceCommands, req.DeviceID), map[string]interface{}{}, map[string]interface{}{
		"commands": req.Commands,
	}, &req)
	if err != nil {
		return SendCommandsResponse{}, err
	}

	tuyaResponse, err := serialize[bool](resp)
	if err != nil {
		return SendCommandsResponse{}, fmt.Errorf("error unmarshalling response: %w", err)
	}

	return SendCommandsResponse{
		Result: tuyaResponse,
	}, nil
}

// GetDeviceStreamAllocate allocates a stream for a device
func (c *DevicesService) GetDeviceStreamAllocate(_ context.Context, _ GetDeviceStreamAllocateRequest) (GetDeviceStreamAllocateResponse, error) {
	// TODO: implement device stream allocation
	return GetDeviceStreamAllocateResponse{
		Success: false,
		Message: "GetDeviceStreamAllocate not yet implemented",
	}, nil
}

// QueryDeviceStatus retrieves the current status of a device
func (c *DevicesService) QueryDeviceStatus(ctx context.Context,
	req QueryDeviceStatusRequest) (QueryDeviceStatusResponse, error) {
	resp, err := c.client.EncryptedClient.Get(ctx, fmt.Sprintf(wire.RouteQueryDeviceStatus, req.DeviceID), nil, &req)
	if err != nil {
		return QueryDeviceStatusResponse{}, err
	}

	// Marshal the response body to JSON bytes
	tuyaResponse, err := serialize[tuyaCloudDeviceStatusResponse](resp)
	if err != nil {
		return QueryDeviceStatusResponse{}, fmt.Errorf("error unmarshalling response: %w", err)
	}

	return QueryDeviceStatusResponse{
		Status: tuyaResponse.DPStatusRelationDTOS,
	}, nil
}

// tuyaCloudDeviceStatusResponse represents the response from querying device status
type tuyaCloudDeviceStatusResponse struct {
	Category             string                `json:"category"`
	DPStatusRelationDTOS []DeviceStatusMapping `json:"dpStatusRelationDTOS"`
}

func serialize[K any](resp *EncryptedAPIResponse) (K, error) {
	var finalResponse K
	// Marshal the response body to JSON bytes
	jsonData, err := json.Marshal(resp.Body)
	if err != nil {
		return finalResponse, fmt.Errorf("error marshalling response: %w", err)
	}

	// Define a struct to match the Tuya API response format
	var tuyaResponse struct {
		Result  K     `json:"result"`
		Success bool  `json:"success"`
		T       int64 `json:"t"`
	}

	err = json.Unmarshal(jsonData, &tuyaResponse)
	if err != nil {
		return finalResponse, fmt.Errorf("error unmarshalling response: %w", err)
	}

	return tuyaResponse.Result, nil
}

func buildDeviceUserBody(nickName string, sex int, birthday *int64, height, weight *int, contact string) map[string]interface{} {
	body := map[string]interface{}{
		"nick_name": nickName,
		"sex":       sex,
	}

	if birthday != nil {
		body["birthday"] = *birthday
	}
	if height != nil {
		body["height"] = *height
	}
	if weight != nil {
		body["weight"] = *weight
	}
	if contact != "" {
		body["contact"] = contact
	}

	return body
}

// QueryDeviceSpecification retrieves the specification details for a device
func (c *DevicesService) QueryDeviceSpecification(ctx context.Context,
	req QueryDeviceSpecificationRequest) (QueryDeviceSpecificationResponse, error) {

	resp, err := c.client.EncryptedClient.Get(ctx, fmt.Sprintf(wire.RouteQueryDeviceSpecification, req.DeviceID), nil, &req)
	if err != nil {
		return QueryDeviceSpecificationResponse{}, err
	}
	tuyaResponse, err := serialize[Specification](resp)
	if err != nil {
		return QueryDeviceSpecificationResponse{}, fmt.Errorf("error unmarshalling response: %w", err)
	}

	return QueryDeviceSpecificationResponse{
		Specification: tuyaResponse,
	}, nil
}

// GetDeviceDetails retrieves detailed information about a single device.
func (c *DevicesService) GetDeviceDetails(ctx context.Context, req GetDeviceDetailsRequest) (GetDeviceDetailsResponse, error) {
	resp, err := c.client.EncryptedClient.Get(ctx, fmt.Sprintf(wire.RouteGetDeviceDetails, req.DeviceID), nil, &req)
	if err != nil {
		return GetDeviceDetailsResponse{}, err
	}

	tuyaResponse, err := serialize[DeviceResponseResultElement](resp)
	if err != nil {
		return GetDeviceDetailsResponse{}, err
	}

	mapped := mapResults([]DeviceResponseResultElement{tuyaResponse})
	var device Device
	if len(mapped) > 0 {
		device = mapped[0]
	}

	return GetDeviceDetailsResponse{
		Device: device,
	}, nil
}

// QueryDevicesByUser retrieves devices associated with a specific UID.
func (c *DevicesService) QueryDevicesByUser(ctx context.Context, req QueryDevicesByUserRequest) (QueryDevicesByUserResponse, error) {
	params := map[string]interface{}{}
	if req.From != "" {
		params["from"] = req.From
	}
	if req.PageNo != nil {
		params["page_no"] = *req.PageNo
	}
	if req.PageSize != nil {
		params["page_size"] = *req.PageSize
	}

	resp, err := c.client.EncryptedClient.Get(ctx, fmt.Sprintf(wire.RouteGetDevicesByUser, req.UID), params, &req)
	if err != nil {
		return QueryDevicesByUserResponse{}, err
	}

	tuyaResponse, err := serialize[[]DeviceResponseResultElement](resp)
	if err != nil {
		return QueryDevicesByUserResponse{}, err
	}

	return QueryDevicesByUserResponse{
		Devices: mapResults(tuyaResponse),
	}, nil
}

// QueryDevices retrieves devices using the global device-list API.
func (c *DevicesService) QueryDevices(ctx context.Context, req QueryDevicesRequest) (QueryDevicesResponse, error) {
	params := map[string]interface{}{
		"page_no":   req.PageNo,
		"page_size": req.PageSize,
	}

	if req.Schema != "" {
		params["schema"] = req.Schema
	}
	if req.ProductID != "" {
		params["product_id"] = req.ProductID
	}
	if len(req.DeviceIDs) > 0 {
		params["device_ids"] = strings.Join(req.DeviceIDs, ",")
	}
	if req.StartTime != "" {
		params["start_time"] = req.StartTime
	}
	if req.EndTime != "" {
		params["end_time"] = req.EndTime
	}
	if req.LastID != "" {
		params["last_id"] = req.LastID
	}

	resp, err := c.client.EncryptedClient.Get(ctx, wire.RouteGetDeviceList, params, &req)
	if err != nil {
		return QueryDevicesResponse{}, err
	}

	tuyaResponse, err := serialize[wire.DeviceListResult](resp)
	if err != nil {
		return QueryDevicesResponse{}, err
	}

	var responseDevices []wire.DeviceRecord
	if tuyaResponse.Devices != nil {
		responseDevices = *tuyaResponse.Devices
	}

	var total int64
	if tuyaResponse.Total != nil {
		total = *tuyaResponse.Total
	}
	var lastID string
	if tuyaResponse.LastId != nil {
		lastID = *tuyaResponse.LastId
	}

	return QueryDevicesResponse{
		Devices: mapGeneratedDeviceRecords(responseDevices),
		Total:   total,
		LastID:  lastID,
	}, nil
}

// UpdateDeviceFunctionName updates the display name of a device function.
func (c *DevicesService) UpdateDeviceFunctionName(ctx context.Context, req UpdateDeviceFunctionNameRequest) (UpdateDeviceFunctionNameResponse, error) {
	body := map[string]interface{}{
		"name": req.Name,
	}
	resp, err := c.client.EncryptedClient.Put(ctx, fmt.Sprintf(wire.RouteUpdateDeviceFunctionName, req.DeviceID, req.FunctionCode), body, &req)
	if err != nil {
		return UpdateDeviceFunctionNameResponse{}, err
	}

	tuyaResponse, err := serialize[bool](resp)
	if err != nil {
		return UpdateDeviceFunctionNameResponse{}, err
	}

	return UpdateDeviceFunctionNameResponse{
		Result: tuyaResponse,
	}, nil
}

// QueryDeviceLogs fetches operation logs for a device.
func (c *DevicesService) QueryDeviceLogs(ctx context.Context, req QueryDeviceLogsRequest) (QueryDeviceLogsResponse, error) {
	params := map[string]interface{}{
		"type":       strings.Join(req.Types, ","),
		"start_time": req.StartTime,
		"end_time":   req.EndTime,
	}

	if len(req.Codes) > 0 {
		params["codes"] = strings.Join(req.Codes, ",")
	}
	if req.StartRowKey != "" {
		params["start_row_key"] = req.StartRowKey
	}
	if req.LastRowKey != "" {
		params["last_row_key"] = req.LastRowKey
	}
	if req.LastEventTime != nil {
		params["last_event_time"] = *req.LastEventTime
	}
	if req.Size != nil {
		params["size"] = *req.Size
	}
	if req.QueryType != nil {
		params["query_type"] = *req.QueryType
	}

	resp, err := c.client.EncryptedClient.Get(ctx, fmt.Sprintf(wire.RouteGetDeviceLogs, req.DeviceID), params, &req)
	if err != nil {
		return QueryDeviceLogsResponse{}, err
	}

	tuyaResponse, err := serialize[QueryDeviceLogsResponse](resp)
	if err != nil {
		return QueryDeviceLogsResponse{}, err
	}

	return tuyaResponse, nil
}

// ResetDeviceFactoryDefaults restores a device to factory defaults.
func (c *DevicesService) ResetDeviceFactoryDefaults(ctx context.Context, req ResetDeviceFactoryDefaultsRequest) (ResetDeviceFactoryDefaultsResponse, error) {
	resp, err := c.client.EncryptedClient.Put(ctx, fmt.Sprintf(wire.RouteResetDeviceFactory, req.DeviceID), nil, &req)
	if err != nil {
		return ResetDeviceFactoryDefaultsResponse{}, err
	}

	tuyaResponse, err := serialize[bool](resp)
	if err != nil {
		return ResetDeviceFactoryDefaultsResponse{}, err
	}

	return ResetDeviceFactoryDefaultsResponse{
		Result: tuyaResponse,
	}, nil
}

// DeleteDevice removes a device by ID.
func (c *DevicesService) DeleteDevice(ctx context.Context, req DeleteDeviceRequest) (DeleteDeviceResponse, error) {
	resp, err := c.client.EncryptedClient.Delete(ctx, fmt.Sprintf(wire.RouteDeleteDevice, req.DeviceID), nil, &req)
	if err != nil {
		return DeleteDeviceResponse{}, err
	}

	tuyaResponse, err := serialize[bool](resp)
	if err != nil {
		return DeleteDeviceResponse{}, err
	}

	return DeleteDeviceResponse{
		Result: tuyaResponse,
	}, nil
}

// QuerySubDevices returns sub-devices for a gateway.
func (c *DevicesService) QuerySubDevices(ctx context.Context, req QuerySubDevicesRequest) (QuerySubDevicesResponse, error) {
	resp, err := c.client.EncryptedClient.Get(ctx, fmt.Sprintf(wire.RouteGetSubDevices, req.DeviceID), nil, &req)
	if err != nil {
		return QuerySubDevicesResponse{}, err
	}

	tuyaResponse, err := serialize[[]SubDevice](resp)
	if err != nil {
		return QuerySubDevicesResponse{}, err
	}

	return QuerySubDevicesResponse{
		Devices: tuyaResponse,
	}, nil
}

// QueryDeviceFactoryInfos fetches factory metadata for devices.
func (c *DevicesService) QueryDeviceFactoryInfos(ctx context.Context, req QueryDeviceFactoryInfosRequest) (QueryDeviceFactoryInfosResponse, error) {
	params := map[string]interface{}{
		"device_ids": strings.Join(req.DeviceIDs, ","),
	}

	resp, err := c.client.EncryptedClient.Get(ctx, wire.RouteGetFactoryInfos, params, &req)
	if err != nil {
		return QueryDeviceFactoryInfosResponse{}, err
	}

	tuyaResponse, err := serialize[[]DeviceFactoryInfo](resp)
	if err != nil {
		return QueryDeviceFactoryInfosResponse{}, err
	}

	return QueryDeviceFactoryInfosResponse{
		Devices: tuyaResponse,
	}, nil
}

// UpdateDeviceName sets the human-readable name of a device.
func (c *DevicesService) UpdateDeviceName(ctx context.Context, req UpdateDeviceNameRequest) (UpdateDeviceNameResponse, error) {
	body := map[string]interface{}{
		"name": req.Name,
	}
	resp, err := c.client.EncryptedClient.Put(ctx, fmt.Sprintf(wire.RouteUpdateDeviceName, req.DeviceID), body, &req)
	if err != nil {
		return UpdateDeviceNameResponse{}, err
	}

	tuyaResponse, err := serialize[bool](resp)
	if err != nil {
		return UpdateDeviceNameResponse{}, err
	}

	return UpdateDeviceNameResponse{
		Result: tuyaResponse,
	}, nil
}

// AddDeviceUser creates a user profile for a device.
func (c *DevicesService) AddDeviceUser(ctx context.Context, req AddDeviceUserRequest) (AddDeviceUserResponse, error) {
	body := buildDeviceUserBody(req.NickName, req.Sex, req.Birthday, req.Height, req.Weight, req.Contact)

	resp, err := c.client.EncryptedClient.Post(ctx, fmt.Sprintf(wire.RouteAddDeviceUser, req.DeviceID), nil, body, &req)
	if err != nil {
		return AddDeviceUserResponse{}, err
	}

	tuyaResponse, err := serialize[string](resp)
	if err != nil {
		return AddDeviceUserResponse{}, err
	}

	return AddDeviceUserResponse{
		UserID: tuyaResponse,
	}, nil
}

// UpdateDeviceUser modifies an existing device user.
func (c *DevicesService) UpdateDeviceUser(ctx context.Context, req UpdateDeviceUserRequest) (UpdateDeviceUserResponse, error) {
	body := buildDeviceUserBody(req.NickName, req.Sex, req.Birthday, req.Height, req.Weight, req.Contact)

	resp, err := c.client.EncryptedClient.Put(ctx, fmt.Sprintf(wire.RouteUpdateDeviceUser, req.DeviceID, req.UserID), body, &req)
	if err != nil {
		return UpdateDeviceUserResponse{}, err
	}

	tuyaResponse, err := serialize[bool](resp)
	if err != nil {
		return UpdateDeviceUserResponse{}, err
	}

	return UpdateDeviceUserResponse{
		Result: tuyaResponse,
	}, nil
}

// DeleteDeviceUser removes a user from a device.
func (c *DevicesService) DeleteDeviceUser(ctx context.Context, req DeleteDeviceUserRequest) (DeleteDeviceUserResponse, error) {
	resp, err := c.client.EncryptedClient.Delete(ctx, fmt.Sprintf(wire.RouteDeleteDeviceUser, req.DeviceID, req.UserID), nil, &req)
	if err != nil {
		return DeleteDeviceUserResponse{}, err
	}

	tuyaResponse, err := serialize[bool](resp)
	if err != nil {
		return DeleteDeviceUserResponse{}, err
	}

	return DeleteDeviceUserResponse{
		Result: tuyaResponse,
	}, nil
}

// GetDeviceUser retrieves a single device user.
func (c *DevicesService) GetDeviceUser(ctx context.Context, req GetDeviceUserRequest) (GetDeviceUserResponse, error) {
	resp, err := c.client.EncryptedClient.Get(ctx, fmt.Sprintf(wire.RouteGetDeviceUser, req.DeviceID, req.UserID), nil, &req)
	if err != nil {
		return GetDeviceUserResponse{}, err
	}

	tuyaResponse, err := serialize[DeviceUser](resp)
	if err != nil {
		return GetDeviceUserResponse{}, err
	}

	return GetDeviceUserResponse{
		User: tuyaResponse,
	}, nil
}

// ListDeviceUsers lists device users for a given device.
func (c *DevicesService) ListDeviceUsers(ctx context.Context, req ListDeviceUsersRequest) (ListDeviceUsersResponse, error) {
	resp, err := c.client.EncryptedClient.Get(ctx, fmt.Sprintf(wire.RouteListDeviceUsers, req.DeviceID), nil, &req)
	if err != nil {
		return ListDeviceUsersResponse{}, err
	}

	tuyaResponse, err := serialize[[]DeviceUser](resp)
	if err != nil {
		return ListDeviceUsersResponse{}, err
	}

	return ListDeviceUsersResponse{
		Users: tuyaResponse,
	}, nil
}

// UpdateMultiOutletName updates the name of a single outlet on a multi-outlet device.
func (c *DevicesService) UpdateMultiOutletName(ctx context.Context, req UpdateMultiOutletNameRequest) (UpdateMultiOutletNameResponse, error) {
	body := map[string]interface{}{
		"identifier": req.Identifier,
		"name":       req.Name,
	}

	resp, err := c.client.EncryptedClient.Put(ctx, fmt.Sprintf(wire.RouteUpdateMultiOutletName, req.DeviceID), body, &req)
	if err != nil {
		return UpdateMultiOutletNameResponse{}, err
	}

	tuyaResponse, err := serialize[bool](resp)
	if err != nil {
		return UpdateMultiOutletNameResponse{}, err
	}

	return UpdateMultiOutletNameResponse{
		Result: tuyaResponse,
	}, nil
}

// ListMultiOutletNames lists identifiers/names for a multi-outlet device.
func (c *DevicesService) ListMultiOutletNames(ctx context.Context, req ListMultiOutletNamesRequest) (ListMultiOutletNamesResponse, error) {
	resp, err := c.client.EncryptedClient.Get(ctx, fmt.Sprintf(wire.RouteListMultiOutletNames, req.DeviceID), nil, &req)
	if err != nil {
		return ListMultiOutletNamesResponse{}, err
	}

	tuyaResponse, err := serialize[[]MultiOutletName](resp)
	if err != nil {
		return ListMultiOutletNamesResponse{}, err
	}

	return ListMultiOutletNamesResponse{
		Names: tuyaResponse,
	}, nil
}
