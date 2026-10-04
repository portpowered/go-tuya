package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/portpowered/go-tuya/pkg/tuya"
)

type deviceView struct {
	HomeID string        `json:"home_id"`
	Device deviceSummary `json:"device"`
}

type homeSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type deviceSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	ProductName string `json:"product_name"`
	Online      bool   `json:"online"`
}

type commandSentResult struct {
	DeviceID string `json:"device_id"`
	Code     string `json:"code"`
	Sent     bool   `json:"sent"`
}

type deviceStatusResult struct {
	DeviceID string        `json:"device_id"`
	Online   bool          `json:"online"`
	Status   []tuya.Status `json:"status"`
}

type deviceCommandInput struct {
	DeviceID string
	Code     string
	Value    any
}

func emptyDeviceCommandInput() deviceCommandInput {
	var command deviceCommandInput

	return command
}

func homesCommand(ctx context.Context, args []string, config settings, out io.Writer, deps Dependencies) error {
	if len(args) != 1 || args[0] != "list" {
		return usageError("usage: go-tuya [global flags] homes list")
	}

	document, err := loadTokenDocument(config)
	if err != nil {
		return err
	}

	requestCtx, cancel := context.WithTimeout(ctx, config.requestTimeout)
	defer cancel()

	return withSession(requestCtx, config, document, deps, func(session *tuya.Session) error {
		response, err := session.HomeService.QueryHomes(requestCtx, tuya.QueryHomesRequest{Request: newRequest()})
		if err != nil {
			return safeSDKError("list homes", err)
		}

		if config.jsonOutput {
			homes := make([]homeSummary, 0, len(response.Results))
			for _, home := range response.Results {
				homes = append(homes, homeSummary{ID: home.ID, Name: home.Name})
			}

			return writeJSON(out, homes)
		}

		for _, home := range response.Results {
			_, err = fmt.Fprintf(out, "%s\t%s\n", home.ID, home.Name)
			if err != nil {
				return newCommandError("could not write command output", "output")
			}
		}

		return nil
	})
}

func devicesCommand(ctx context.Context, args []string, config settings, out io.Writer, deps Dependencies) error {
	if len(args) == 0 {
		return usageError("usage: go-tuya [global flags] devices list|status|spec")
	}

	switch args[0] {
	case commandStatus, "spec":
		return readDeviceCommand(ctx, args, config, out, deps)
	case "list":
		return listDevicesCommand(ctx, args[1:], config, out, deps)
	default:
		return usageError("usage: go-tuya [global flags] devices list|status|spec")
	}
}

func readDeviceCommand(ctx context.Context, args []string, config settings, out io.Writer, deps Dependencies) error {
	if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
		return usageError("device status and spec require a device ID")
	}

	document, err := loadTokenDocument(config)
	if err != nil {
		return err
	}

	requestCtx, cancel := context.WithTimeout(ctx, config.requestTimeout)
	defer cancel()

	return withSession(requestCtx, config, document, deps, func(session *tuya.Session) error {
		if args[0] == commandStatus {
			return deviceStatusCommand(requestCtx, session, args[1], config, out)
		}

		return deviceSpecCommand(requestCtx, session, args[1], config, out)
	})
}

func listDevicesCommand(ctx context.Context, args []string, config settings, out io.Writer, deps Dependencies) error {
	flags := flag.NewFlagSet("devices list", flag.ContinueOnError)
	homeID := flags.String("home", "", "list devices in one home")

	rest, help, err := parseCommandFlags(flags, args, out, "Usage: go-tuya [global flags] devices list [--home id]")
	if err != nil || help {
		return err
	}

	if len(rest) != 0 {
		return usageError("devices list does not accept positional arguments")
	}

	document, err := loadTokenDocument(config)
	if err != nil {
		return err
	}

	requestCtx, cancel := context.WithTimeout(ctx, config.requestTimeout)
	defer cancel()

	return withSession(requestCtx, config, document, deps, func(session *tuya.Session) error {
		devices, err := queryDevicesForHomes(requestCtx, session, *homeID)
		if err != nil {
			return err
		}

		return writeDeviceViews(out, config.jsonOutput, devices)
	})
}

func queryDevicesForHomes(ctx context.Context, session *tuya.Session, selectedHomeID string) ([]deviceView, error) {
	homeIDs := []string{}
	if selectedHomeID != "" {
		homeIDs = append(homeIDs, selectedHomeID)
	} else {
		response, err := session.HomeService.QueryHomes(ctx, tuya.QueryHomesRequest{Request: newRequest()})
		if err != nil {
			return nil, safeSDKError("list homes", err)
		}

		for _, home := range response.Results {
			homeIDs = append(homeIDs, home.ID)
		}
	}

	devices := make([]deviceView, 0)

	for _, currentHomeID := range homeIDs {
		response, err := session.DevicesService.QueryDevicesByHome(ctx, tuya.QueryDevicesByHomeRequest{
			Request: newRequest(),
			HomeID:  currentHomeID,
		})
		if err != nil {
			return nil, safeSDKError("list devices", err)
		}

		for _, device := range response.Results {
			devices = append(devices, deviceView{HomeID: currentHomeID, Device: deviceSummary{
				ID:          device.ID,
				Name:        device.Name,
				Category:    device.Category,
				ProductName: device.ProductName,
				Online:      device.Online,
			}})
		}
	}

	return devices, nil
}

func writeDeviceViews(out io.Writer, jsonOutput bool, devices []deviceView) error {
	if jsonOutput {
		return writeJSON(out, devices)
	}

	for _, item := range devices {
		_, err := fmt.Fprintf(out, "%s\t%s\t%s\n", item.HomeID, item.Device.ID, item.Device.Name)
		if err != nil {
			return newCommandError("could not write command output", "output")
		}
	}

	return nil
}

func deviceCommand(ctx context.Context, args []string, config settings, input io.Reader, out io.Writer, deps Dependencies) error {
	command, err := parseDeviceCommand(args, input, out)
	if err != nil {
		return err
	}

	document, err := loadTokenDocument(config)
	if err != nil {
		return err
	}

	requestCtx, cancel := context.WithTimeout(ctx, config.requestTimeout)
	defer cancel()

	return withSession(requestCtx, config, document, deps, func(session *tuya.Session) error {
		_, err := session.DevicesService.SendCommands(requestCtx, tuya.SendCommandsRequest{
			Request:  newRequest(),
			DeviceID: command.DeviceID,
			Commands: []tuya.Command{{Code: command.Code, Value: command.Value}},
		})
		if err != nil {
			return safeSDKError("send device command", err)
		}

		result := commandSentResult{DeviceID: command.DeviceID, Code: command.Code, Sent: true}

		return emit(out, config.jsonOutput, result, fmt.Sprintf("Command %s sent to device %s.", command.Code, command.DeviceID))
	})
}

func parseDeviceCommand(args []string, input io.Reader, out io.Writer) (deviceCommandInput, error) {
	if len(args) == 0 || args[0] != "command" {
		return emptyDeviceCommandInput(), usageError("usage: go-tuya [global flags] device command (--value-file path|--value-stdin) <device-id> <code>")
	}

	valueFile, rest, err := parseDeviceCommandFlags(args[1:], out)
	if err != nil {
		return emptyDeviceCommandInput(), err
	}

	deviceID, commandCode, err := validateDeviceCommandTarget(rest)
	if err != nil {
		return emptyDeviceCommandInput(), err
	}

	value, err := readDeviceCommandValue(valueFile, input)
	if err != nil {
		return emptyDeviceCommandInput(), err
	}

	return deviceCommandInput{DeviceID: deviceID, Code: commandCode, Value: value}, nil
}

func parseDeviceCommandFlags(args []string, out io.Writer) (string, []string, error) {
	flags := flag.NewFlagSet("device command", flag.ContinueOnError)
	valueFile := flags.String("value-file", "", "read command value JSON from a file")
	valueStdin := flags.Bool("value-stdin", false, "read command value JSON from stdin")

	rest, help, err := parseCommandFlags(flags, args, out, "Usage: go-tuya [global flags] device command (--value-file path|--value-stdin) <device-id> <code>")
	if err != nil || help {
		return "", nil, err
	}

	if len(rest) != 2 || (*valueFile != "") == *valueStdin {
		return "", nil, usageError("device command requires a device ID, command code, and exactly one of --value-file or --value-stdin")
	}

	return *valueFile, rest, nil
}

func validateDeviceCommandTarget(rest []string) (string, string, error) {
	deviceID := strings.TrimSpace(rest[0])

	commandCode := strings.TrimSpace(rest[1])

	if deviceID == "" || commandCode == "" {
		return "", "", usageError("device ID and command code are required")
	}

	return deviceID, commandCode, nil
}

func readDeviceCommandValue(valueFile string, input io.Reader) (any, error) {
	valueReader := input

	if valueFile != "" {
		file, openErr := os.Open(valueFile) //nolint:gosec // The caller explicitly selected a local command-value file.
		if openErr != nil {
			return nil, newCommandError("could not open command value file", "filesystem")
		}

		defer func() { _ = file.Close() }()

		valueReader = file
	}

	value, err := decodeCommandValue(valueReader)
	if err != nil {
		return nil, err
	}

	return value, nil
}

func decodeCommandValue(reader io.Reader) (any, error) {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()

	var value any

	err := decoder.Decode(&value)
	if err != nil {
		return nil, usageError("command value input must contain valid JSON")
	}

	err = decoder.Decode(new(any))
	if err != io.EOF {
		return nil, usageError("command value input must contain exactly one JSON value")
	}

	return value, nil
}

func deviceStatusCommand(requestCtx context.Context, session *tuya.Session, deviceID string, config settings, out io.Writer) error {
	response, err := session.DevicesService.QueryDevicesByIDs(requestCtx, tuya.QueryDevicesByIDsRequest{
		Request:   newRequest(),
		DeviceIDs: []string{deviceID},
	})
	if err != nil {
		return safeSDKError("read device status", err)
	}

	var result *deviceStatusResult

	for _, device := range response.Results {
		if device.ID == deviceID {
			result = &deviceStatusResult{DeviceID: device.ID, Online: device.Online, Status: device.Status}

			break
		}
	}

	if result == nil {
		return newCommandError("device was not returned by Tuya", "not_found")
	}

	if config.jsonOutput {
		return writeJSON(out, result)
	}

	_, err = fmt.Fprintf(out, "Device: %s\nOnline: %t\n", result.DeviceID, result.Online)
	if err != nil {
		return newCommandError("could not write command output", "output")
	}

	for _, status := range result.Status {
		value, err := json.Marshal(status.Value)
		if err != nil {
			return newCommandError("could not encode device status", "protocol")
		}

		_, err = fmt.Fprintf(out, "%s\t%s\n", status.Code, value)
		if err != nil {
			return newCommandError("could not write command output", "output")
		}
	}

	return nil
}

func deviceSpecCommand(requestCtx context.Context, session *tuya.Session, deviceID string, config settings, out io.Writer) error {
	response, err := session.DevicesService.QueryDeviceSpecification(requestCtx, tuya.QueryDeviceSpecificationRequest{Request: newRequest(), DeviceID: deviceID})
	if err != nil {
		return safeSDKError("read device specification", err)
	}

	if config.jsonOutput {
		return writeJSON(out, response)
	}

	return emit(out, false, response, fmt.Sprintf("Specification read for device %s.", deviceID))
}
