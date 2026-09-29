package tuya

import (
	"context"
	"fmt"
	"strings"

	"github.com/portpowered/go-tuya/pkg/tuya/internal/wire"
)

// DevicesService provides methods for managing smart devices.
type DevicesService service

// QueryDevicesByHome fetches all devices for a given home.
// Fetching every device's capabilities would require an additional request per device.
// This is done individually for each device, along with their states, which is a very expensive operation generally.
func (c *DevicesService) QueryDevicesByHome(ctx context.Context, req QueryDevicesByHomeRequest) (QueryDevicesByHomeResponse, error) {
	params, err := wireRequestMap(wire.QueryHomeDevicesParams{HomeId: &req.HomeID, DeviceIds: nil})
	if err != nil {
		return QueryDevicesByHomeResponse{}, err
	}

	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationQueryHomeDevices(), nil, params, nil, &req)
	if err != nil {
		return QueryDevicesByHomeResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.HomeDevicesEnvelope](resp)
	if err != nil {
		return QueryDevicesByHomeResponse{}, err
	}

	return QueryDevicesByHomeResponse{
		Results: mapWireDevices(dereference(wireResponse.Result)),
		Success: false,
		Message: "",
	}, nil
}

// QueryDevicesByHomeAssistantDevices fetches all devices for a given home using the Home Assistant devices endpoint.
func (c *DevicesService) QueryDevicesByHomeAssistantDevices(ctx context.Context, req QueryDevicesByHomeRequest) (QueryDevicesByHomeResponse, error) {
	params, err := wireRequestMap(wire.QueryHomeDevicesParams{HomeId: &req.HomeID, DeviceIds: nil})
	if err != nil {
		return QueryDevicesByHomeResponse{}, err
	}

	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationQueryHomeDevices(), nil, params, nil, &req)
	if err != nil {
		return QueryDevicesByHomeResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.HomeDevicesEnvelope](resp)
	if err != nil {
		return QueryDevicesByHomeResponse{}, err
	}

	return QueryDevicesByHomeResponse{
		Results: mapWireDevices(dereference(wireResponse.Result)),
		Success: false,
		Message: "",
	}, nil
}

func mapWireDevices(records []wire.DeviceRecord) []Device {
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

func additionalString(properties map[string]any, name string) string {
	value, _ := properties[name].(string)

	return value
}

func dereference[T any](value *T) T { //nolint:ireturn // This generic helper returns the pointed-to concrete wire field type.
	if value == nil {
		var zero T

		return zero
	}

	return *value
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

func optionalCommaList(values []string) *string {
	if len(values) == 0 {
		return nil
	}

	joined := strings.Join(values, ",")

	return &joined
}

func checkedInt64AsInt(value int64) (int, error) {
	converted := int(value)
	if int64(converted) != value {
		return 0, fmt.Errorf("%w %d is outside the generated model range", errWireQueryIntegerOutOfRange, value)
	}

	return converted, nil
}

func optionalInt64AsInt(value *int64) (*int, error) {
	if value == nil {
		//nolint:nilnil // An absent optional parameter remains nil and is not an error.
		return nil, nil
	}

	converted, err := checkedInt64AsInt(*value)
	if err != nil {
		return nil, err
	}

	return &converted, nil
}

func decodeBooleanResult(response *EncryptedAPIResponse) (bool, error) {
	wireResponse, err := decodeWireResponse[wire.BooleanResultEnvelope](response)
	if err != nil {
		return false, err
	}

	return dereference(wireResponse.Result), nil
}

// QueryDevicesByIDs fetches devices by their IDs.
func (c *DevicesService) QueryDevicesByIDs(ctx context.Context, req QueryDevicesByIDsRequest) (QueryDevicesByIDsResponse, error) {
	deviceIDs := strings.Join(req.DeviceIDs, ",")

	params, err := wireRequestMap(wire.QueryHomeDevicesParams{HomeId: nil, DeviceIds: &deviceIDs})
	if err != nil {
		return QueryDevicesByIDsResponse{}, err
	}

	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationQueryHomeDevices(), nil, params, nil, &req)
	if err != nil {
		return QueryDevicesByIDsResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.HomeDevicesEnvelope](resp)
	if err != nil {
		return QueryDevicesByIDsResponse{}, err
	}

	return QueryDevicesByIDsResponse{
		Results: mapWireDevices(dereference(wireResponse.Result)),
	}, nil
}

// SendCommands sends control commands to a device
// https://developer.tuya.com/en/docs/cloud/device-control?id=K95zu01ksols7
func (c *DevicesService) SendCommands(ctx context.Context, req SendCommandsRequest) (SendCommandsResponse, error) {
	commands := make([]wire.SendCommandsBody_Commands_Item, len(req.Commands))
	for i, command := range req.Commands {
		commands[i] = wire.SendCommandsBody_Commands_Item{Code: command.Code, Value: command.Value, AdditionalProperties: nil}
	}

	body, err := wireRequestMap(wire.SendCommandsBody{Commands: commands, AdditionalProperties: nil})
	if err != nil {
		return SendCommandsResponse{}, err
	}

	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationSendDeviceCommands(), []any{req.DeviceID}, map[string]any{}, body, &req)
	if err != nil {
		return SendCommandsResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.BooleanResultEnvelope](resp)
	if err != nil {
		return SendCommandsResponse{}, fmt.Errorf("error unmarshalling response: %w", err)
	}

	return SendCommandsResponse{
		Result:  dereference(wireResponse.Result),
		Message: "",
	}, nil
}

// GetDeviceStreamAllocate allocates a stream for a device.
func (c *DevicesService) GetDeviceStreamAllocate(_ context.Context, _ GetDeviceStreamAllocateRequest) (GetDeviceStreamAllocateResponse, error) {
	// Device stream allocation is not implemented yet.
	return GetDeviceStreamAllocateResponse{
		StreamURL: "",
		Success:   false,
		Message:   "GetDeviceStreamAllocate not yet implemented",
	}, nil
}

// QueryDeviceStatus retrieves the current status of a device.
func (c *DevicesService) QueryDeviceStatus(ctx context.Context,
	req QueryDeviceStatusRequest) (QueryDeviceStatusResponse, error) {
	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationQueryDeviceStatus(), []any{req.DeviceID}, nil, nil, &req)
	if err != nil {
		return QueryDeviceStatusResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.DeviceStatusEnvelope](resp)
	if err != nil {
		return QueryDeviceStatusResponse{}, fmt.Errorf("error unmarshalling response: %w", err)
	}

	var wireStatus []wire.DeviceStatusMapping
	if wireResponse.Result != nil {
		wireStatus = dereference(wireResponse.Result.DpStatusRelationDTOS)
	}

	publicStatus, err := convertWireValue[[]DeviceStatusMapping](wireStatus)
	if err != nil {
		return QueryDeviceStatusResponse{}, fmt.Errorf("error converting device status: %w", err)
	}

	return QueryDeviceStatusResponse{
		Status:  publicStatus,
		Message: "",
	}, nil
}

func buildDeviceUserBody(nickName string, sex int, birthday *int64, height, weight *int, contact string) (map[string]any, error) {
	var contactValue *string
	if contact != "" {
		contactValue = &contact
	}

	return wireRequestMap(wire.DeviceUserBody{
		NickName:             nickName,
		Sex:                  sex,
		Birthday:             birthday,
		Height:               height,
		Weight:               weight,
		Contact:              contactValue,
		AdditionalProperties: nil,
	})
}

// QueryDeviceSpecification retrieves the specification details for a device.
func (c *DevicesService) QueryDeviceSpecification(ctx context.Context,
	req QueryDeviceSpecificationRequest) (QueryDeviceSpecificationResponse, error) {
	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationQueryDeviceSpecification(), []any{req.DeviceID}, nil, nil, &req)
	if err != nil {
		return QueryDeviceSpecificationResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.DeviceSpecificationEnvelope](resp)
	if err != nil {
		return QueryDeviceSpecificationResponse{}, fmt.Errorf("error unmarshalling response: %w", err)
	}

	var wireSpecification wire.DeviceSpecification
	if wireResponse.Result != nil {
		wireSpecification = *wireResponse.Result
	}

	specification, err := convertWireValue[Specification](wireSpecification)
	if err != nil {
		return QueryDeviceSpecificationResponse{}, fmt.Errorf("error converting device specification: %w", err)
	}

	return QueryDeviceSpecificationResponse{
		Specification: specification,
	}, nil
}

// GetDeviceDetails retrieves detailed information about a single device.
func (c *DevicesService) GetDeviceDetails(ctx context.Context, req GetDeviceDetailsRequest) (GetDeviceDetailsResponse, error) {
	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationGetDeviceDetails(), []any{req.DeviceID}, nil, nil, &req)
	if err != nil {
		return GetDeviceDetailsResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.DeviceDetailsEnvelope](resp)
	if err != nil {
		return GetDeviceDetailsResponse{}, err
	}

	var device Device

	if wireResponse.Result != nil {
		mapped := mapWireDevices([]wire.DeviceRecord{*wireResponse.Result})
		if len(mapped) > 0 {
			device = mapped[0]
		}
	}

	return GetDeviceDetailsResponse{
		Device: device,
	}, nil
}

// QueryDevicesByUser retrieves devices associated with a specific UID.
func (c *DevicesService) QueryDevicesByUser(ctx context.Context, req QueryDevicesByUserRequest) (QueryDevicesByUserResponse, error) {
	var from *string
	if req.From != "" {
		from = &req.From
	}

	params, err := wireRequestMap(wire.GetDevicesByUserParams{
		From:     from,
		PageNo:   req.PageNo,
		PageSize: req.PageSize,
	})
	if err != nil {
		return QueryDevicesByUserResponse{}, err
	}

	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationGetDevicesByUser(), []any{req.UID}, params, nil, &req)
	if err != nil {
		return QueryDevicesByUserResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.DeviceListByUserEnvelope](resp)
	if err != nil {
		return QueryDevicesByUserResponse{}, err
	}

	return QueryDevicesByUserResponse{
		Devices: mapWireDevices(dereference(wireResponse.Result)),
	}, nil
}

// QueryDevices retrieves devices using the global device-list API.
func (c *DevicesService) QueryDevices(ctx context.Context, req QueryDevicesRequest) (QueryDevicesResponse, error) {
	params, err := wireRequestMap(wire.GetDeviceListParams{
		DeviceIds: optionalCommaList(req.DeviceIDs),
		ProductId: optionalString(req.ProductID),
		Schema:    optionalString(req.Schema),
		LastId:    optionalString(req.LastID),
		PageSize:  req.PageSize,
		PageNo:    req.PageNo,
		StartTime: optionalString(req.StartTime),
		EndTime:   optionalString(req.EndTime),
	})
	if err != nil {
		return QueryDevicesResponse{}, err
	}

	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationGetDeviceList(), nil, params, nil, &req)
	if err != nil {
		return QueryDevicesResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.DeviceListEnvelope](resp)
	if err != nil {
		return QueryDevicesResponse{}, err
	}

	var responseDevices []wire.DeviceRecord
	if wireResponse.Result != nil && wireResponse.Result.Devices != nil {
		responseDevices = *wireResponse.Result.Devices
	}

	var total int64
	if wireResponse.Result != nil && wireResponse.Result.Total != nil {
		total = *wireResponse.Result.Total
	}

	var lastID string
	if wireResponse.Result != nil && wireResponse.Result.LastId != nil {
		lastID = *wireResponse.Result.LastId
	}

	return QueryDevicesResponse{
		Devices: mapWireDevices(responseDevices),
		Total:   total,
		LastID:  lastID,
	}, nil
}

// UpdateDeviceFunctionName updates the display name of a device function.
func (c *DevicesService) UpdateDeviceFunctionName(ctx context.Context, req UpdateDeviceFunctionNameRequest) (UpdateDeviceFunctionNameResponse, error) {
	body, err := wireRequestMap(wire.UpdateNameBody{Name: req.Name, AdditionalProperties: nil})
	if err != nil {
		return UpdateDeviceFunctionNameResponse{}, err
	}

	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationUpdateDeviceFunctionName(), []any{req.DeviceID, req.FunctionCode}, nil, body, &req)
	if err != nil {
		return UpdateDeviceFunctionNameResponse{}, err
	}

	result, err := decodeBooleanResult(resp)
	if err != nil {
		return UpdateDeviceFunctionNameResponse{}, err
	}

	return UpdateDeviceFunctionNameResponse{
		Result: result,
	}, nil
}

// QueryDeviceLogs fetches operation logs for a device.
func (c *DevicesService) QueryDeviceLogs(ctx context.Context, req QueryDeviceLogsRequest) (QueryDeviceLogsResponse, error) {
	lastEventTime, err := optionalInt64AsInt(req.LastEventTime)
	if err != nil {
		return QueryDeviceLogsResponse{}, err
	}

	startTime, err := checkedInt64AsInt(req.StartTime)
	if err != nil {
		return QueryDeviceLogsResponse{}, err
	}

	endTime, err := checkedInt64AsInt(req.EndTime)
	if err != nil {
		return QueryDeviceLogsResponse{}, err
	}

	params, err := wireRequestMap(wire.GetDeviceLogsParams{
		Type:          strings.Join(req.Types, ","),
		StartTime:     startTime,
		EndTime:       endTime,
		Codes:         optionalCommaList(req.Codes),
		StartRowKey:   optionalString(req.StartRowKey),
		LastRowKey:    optionalString(req.LastRowKey),
		LastEventTime: lastEventTime,
		Size:          req.Size,
		QueryType:     req.QueryType,
	})
	if err != nil {
		return QueryDeviceLogsResponse{}, err
	}

	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationGetDeviceLogs(), []any{req.DeviceID}, params, nil, &req)
	if err != nil {
		return QueryDeviceLogsResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.DeviceLogsEnvelope](resp)
	if err != nil {
		return QueryDeviceLogsResponse{}, err
	}

	if wireResponse.Result == nil {
		return QueryDeviceLogsResponse{
			Logs:          nil,
			HasNext:       false,
			DeviceID:      "",
			CurrentRowKey: "",
			NextRowKey:    "",
			Count:         0,
		}, nil
	}

	result, err := convertWireValue[QueryDeviceLogsResponse](*wireResponse.Result)
	if err != nil {
		return QueryDeviceLogsResponse{}, err
	}

	return result, nil
}

// ResetDeviceFactoryDefaults restores a device to factory defaults.
func (c *DevicesService) ResetDeviceFactoryDefaults(ctx context.Context, req ResetDeviceFactoryDefaultsRequest) (ResetDeviceFactoryDefaultsResponse, error) {
	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationResetDeviceFactory(), []any{req.DeviceID}, nil, nil, &req)
	if err != nil {
		return ResetDeviceFactoryDefaultsResponse{}, err
	}

	result, err := decodeBooleanResult(resp)
	if err != nil {
		return ResetDeviceFactoryDefaultsResponse{}, err
	}

	return ResetDeviceFactoryDefaultsResponse{
		Result: result,
	}, nil
}

// DeleteDevice removes a device by ID.
func (c *DevicesService) DeleteDevice(ctx context.Context, req DeleteDeviceRequest) (DeleteDeviceResponse, error) {
	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationDeleteDevice(), []any{req.DeviceID}, nil, nil, &req)
	if err != nil {
		return DeleteDeviceResponse{}, err
	}

	result, err := decodeBooleanResult(resp)
	if err != nil {
		return DeleteDeviceResponse{}, err
	}

	return DeleteDeviceResponse{
		Result: result,
	}, nil
}

// QuerySubDevices returns sub-devices for a gateway.
func (c *DevicesService) QuerySubDevices(ctx context.Context, req QuerySubDevicesRequest) (QuerySubDevicesResponse, error) {
	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationGetSubDevices(), []any{req.DeviceID}, nil, nil, &req)
	if err != nil {
		return QuerySubDevicesResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.SubDevicesEnvelope](resp)
	if err != nil {
		return QuerySubDevicesResponse{}, err
	}

	var wireDevices []wire.SubDeviceRecord
	if wireResponse.Result != nil {
		wireDevices = *wireResponse.Result
	}

	devices, err := convertWireValue[[]SubDevice](wireDevices)
	if err != nil {
		return QuerySubDevicesResponse{}, err
	}

	return QuerySubDevicesResponse{
		Devices: devices,
	}, nil
}

// QueryDeviceFactoryInfos fetches factory metadata for devices.
func (c *DevicesService) QueryDeviceFactoryInfos(ctx context.Context, req QueryDeviceFactoryInfosRequest) (QueryDeviceFactoryInfosResponse, error) {
	params, err := wireRequestMap(wire.GetFactoryInfosParams{
		DeviceIds: strings.Join(req.DeviceIDs, ","),
	})
	if err != nil {
		return QueryDeviceFactoryInfosResponse{}, err
	}

	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationGetFactoryInfos(), nil, params, nil, &req)
	if err != nil {
		return QueryDeviceFactoryInfosResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.FactoryInfosEnvelope](resp)
	if err != nil {
		return QueryDeviceFactoryInfosResponse{}, err
	}

	var wireDevices []wire.FactoryInfoRecord
	if wireResponse.Result != nil {
		wireDevices = *wireResponse.Result
	}

	devices, err := convertWireValue[[]DeviceFactoryInfo](wireDevices)
	if err != nil {
		return QueryDeviceFactoryInfosResponse{}, err
	}

	return QueryDeviceFactoryInfosResponse{
		Devices: devices,
	}, nil
}

// UpdateDeviceName sets the human-readable name of a device.
func (c *DevicesService) UpdateDeviceName(ctx context.Context, req UpdateDeviceNameRequest) (UpdateDeviceNameResponse, error) {
	body, err := wireRequestMap(wire.UpdateNameBody{Name: req.Name, AdditionalProperties: nil})
	if err != nil {
		return UpdateDeviceNameResponse{}, err
	}

	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationUpdateDeviceName(), []any{req.DeviceID}, nil, body, &req)
	if err != nil {
		return UpdateDeviceNameResponse{}, err
	}

	result, err := decodeBooleanResult(resp)
	if err != nil {
		return UpdateDeviceNameResponse{}, err
	}

	return UpdateDeviceNameResponse{
		Result: result,
	}, nil
}

// AddDeviceUser creates a user profile for a device.
func (c *DevicesService) AddDeviceUser(ctx context.Context, req AddDeviceUserRequest) (AddDeviceUserResponse, error) {
	body, err := buildDeviceUserBody(req.NickName, req.Sex, req.Birthday, req.Height, req.Weight, req.Contact)
	if err != nil {
		return AddDeviceUserResponse{}, err
	}

	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationAddDeviceUser(), []any{req.DeviceID}, nil, body, &req)
	if err != nil {
		return AddDeviceUserResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.StringResultEnvelope](resp)
	if err != nil {
		return AddDeviceUserResponse{}, err
	}

	return AddDeviceUserResponse{
		UserID: dereference(wireResponse.Result),
	}, nil
}

// UpdateDeviceUser modifies an existing device user.
func (c *DevicesService) UpdateDeviceUser(ctx context.Context, req UpdateDeviceUserRequest) (UpdateDeviceUserResponse, error) {
	body, err := buildDeviceUserBody(req.NickName, req.Sex, req.Birthday, req.Height, req.Weight, req.Contact)
	if err != nil {
		return UpdateDeviceUserResponse{}, err
	}

	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationUpdateDeviceUser(), []any{req.DeviceID, req.UserID}, nil, body, &req)
	if err != nil {
		return UpdateDeviceUserResponse{}, err
	}

	result, err := decodeBooleanResult(resp)
	if err != nil {
		return UpdateDeviceUserResponse{}, err
	}

	return UpdateDeviceUserResponse{
		Result: result,
	}, nil
}

// DeleteDeviceUser removes a user from a device.
func (c *DevicesService) DeleteDeviceUser(ctx context.Context, req DeleteDeviceUserRequest) (DeleteDeviceUserResponse, error) {
	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationDeleteDeviceUser(), []any{req.DeviceID, req.UserID}, nil, nil, &req)
	if err != nil {
		return DeleteDeviceUserResponse{}, err
	}

	result, err := decodeBooleanResult(resp)
	if err != nil {
		return DeleteDeviceUserResponse{}, err
	}

	return DeleteDeviceUserResponse{
		Result: result,
	}, nil
}

// GetDeviceUser retrieves a single device user.
func (c *DevicesService) GetDeviceUser(ctx context.Context, req GetDeviceUserRequest) (GetDeviceUserResponse, error) {
	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationGetDeviceUser(), []any{req.DeviceID, req.UserID}, nil, nil, &req)
	if err != nil {
		return GetDeviceUserResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.DeviceUserEnvelope](resp)
	if err != nil {
		return GetDeviceUserResponse{}, err
	}

	var user DeviceUser
	if wireResponse.Result != nil {
		user, err = convertWireValue[DeviceUser](*wireResponse.Result)
		if err != nil {
			return GetDeviceUserResponse{}, err
		}
	}

	return GetDeviceUserResponse{
		User: user,
	}, nil
}

// ListDeviceUsers lists device users for a given device.
func (c *DevicesService) ListDeviceUsers(ctx context.Context, req ListDeviceUsersRequest) (ListDeviceUsersResponse, error) {
	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationListDeviceUsers(), []any{req.DeviceID}, nil, nil, &req)
	if err != nil {
		return ListDeviceUsersResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.DeviceUsersEnvelope](resp)
	if err != nil {
		return ListDeviceUsersResponse{}, err
	}

	var wireUsers []wire.DeviceUserRecord
	if wireResponse.Result != nil {
		wireUsers = *wireResponse.Result
	}

	users, err := convertWireValue[[]DeviceUser](wireUsers)
	if err != nil {
		return ListDeviceUsersResponse{}, err
	}

	return ListDeviceUsersResponse{
		Users: users,
	}, nil
}

// UpdateMultiOutletName updates the name of a single outlet on a multi-outlet device.
func (c *DevicesService) UpdateMultiOutletName(ctx context.Context, req UpdateMultiOutletNameRequest) (UpdateMultiOutletNameResponse, error) {
	body, err := wireRequestMap(wire.UpdateMultiOutletNameBody{
		Identifier:           req.Identifier,
		Name:                 req.Name,
		AdditionalProperties: nil,
	})
	if err != nil {
		return UpdateMultiOutletNameResponse{}, err
	}

	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationUpdateMultiOutletName(), []any{req.DeviceID}, nil, body, &req)
	if err != nil {
		return UpdateMultiOutletNameResponse{}, err
	}

	result, err := decodeBooleanResult(resp)
	if err != nil {
		return UpdateMultiOutletNameResponse{}, err
	}

	return UpdateMultiOutletNameResponse{
		Result: result,
	}, nil
}

// ListMultiOutletNames lists identifiers/names for a multi-outlet device.
func (c *DevicesService) ListMultiOutletNames(ctx context.Context, req ListMultiOutletNamesRequest) (ListMultiOutletNamesResponse, error) {
	resp, err := c.client.EncryptedClient.requestOperation(ctx, wire.OperationListMultiOutletNames(), []any{req.DeviceID}, nil, nil, &req)
	if err != nil {
		return ListMultiOutletNamesResponse{}, err
	}

	wireResponse, err := decodeWireResponse[wire.MultiOutletNamesEnvelope](resp)
	if err != nil {
		return ListMultiOutletNamesResponse{}, err
	}

	var wireNames []wire.MultiOutletNameRecord
	if wireResponse.Result != nil {
		wireNames = *wireResponse.Result
	}

	names, err := convertWireValue[[]MultiOutletName](wireNames)
	if err != nil {
		return ListMultiOutletNamesResponse{}, err
	}

	return ListMultiOutletNamesResponse{
		Names: names,
	}, nil
}
