package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/portpowered/go-tuya/pkg/tuya"
)

const (
	routineBoolean                = "boolean"
	routineInteger                = "integer"
	routineEnum                   = "enum"
	duplicateRoutineNameThreshold = 2
)

type routineControl struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Actions []string `json:"actions,omitempty"`
	Minimum *int64   `json:"minimum,omitempty"`
	Maximum *int64   `json:"maximum,omitempty"`
	Step    *int64   `json:"step,omitempty"`
	Options []string `json:"options,omitempty"`
	code    string
	label   string
}

type deviceRoutinesResult struct {
	DeviceID string           `json:"device_id"`
	Routines []routineControl `json:"routines"`
}

type routineSentResult struct {
	DeviceID string `json:"device_id"`
	Routine  string `json:"routine"`
	Sent     bool   `json:"sent"`
}

func deviceRoutinesCommand(ctx context.Context, args []string, config settings, out io.Writer, deps Dependencies) error {
	if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
		return usageError("usage: go-tuya [global flags] devices routines <device-id>")
	}

	deviceID := strings.TrimSpace(args[1])

	document, err := loadTokenDocument(config)
	if err != nil {
		return err
	}

	requestCtx, cancel := context.WithTimeout(ctx, config.requestTimeout)
	defer cancel()

	return withSession(requestCtx, config, document, deps, func(session *tuya.Session) error {
		controls, queryErr := queryDeviceRoutines(requestCtx, session, deviceID)
		if queryErr != nil {
			return queryErr
		}

		return writeDeviceRoutines(out, config.jsonOutput, deviceID, controls)
	})
}

func deviceRoutineCommand(ctx context.Context, args []string, config settings, out io.Writer, deps Dependencies) error {
	if len(args) != 3 || strings.TrimSpace(args[0]) == "" || strings.TrimSpace(args[1]) == "" || strings.TrimSpace(args[2]) == "" {
		return usageError("usage: go-tuya [global flags] device routine <device-id> <routine> <value>")
	}

	deviceID := strings.TrimSpace(args[0])
	selector := strings.TrimSpace(args[1])
	rawValue := strings.TrimSpace(args[2])

	document, err := loadTokenDocument(config)
	if err != nil {
		return err
	}

	requestCtx, cancel := context.WithTimeout(ctx, config.requestTimeout)
	defer cancel()

	return withSession(requestCtx, config, document, deps, func(session *tuya.Session) error {
		controls, queryErr := queryDeviceRoutines(requestCtx, session, deviceID)
		if queryErr != nil {
			return queryErr
		}

		control, findErr := findRoutineControl(controls, selector)
		if findErr != nil {
			return findErr
		}

		value, valueErr := parseRoutineValue(control, rawValue)
		if valueErr != nil {
			return valueErr
		}

		_, sendErr := session.DevicesService.SendCommands(requestCtx, tuya.SendCommandsRequest{
			Request:  newRequest(),
			DeviceID: deviceID,
			Commands: []tuya.Command{{Code: control.code, Value: value}},
		})
		if sendErr != nil {
			return routineRequestError(requestCtx, "send device routine", sendErr)
		}

		result := routineSentResult{DeviceID: deviceID, Routine: control.Name, Sent: true}

		return emit(out, config.jsonOutput, result, fmt.Sprintf("Routine %s sent to device %s.", control.Name, deviceID))
	})
}

func queryDeviceRoutines(ctx context.Context, session *tuya.Session, deviceID string) ([]routineControl, error) {
	response, err := session.DevicesService.QueryDeviceSpecification(ctx, tuya.QueryDeviceSpecificationRequest{
		Request:  newRequest(),
		DeviceID: deviceID,
	})
	if err != nil {
		return nil, routineRequestError(ctx, "read device routines", err)
	}

	return discoverRoutineControls(response.Specification), nil
}

func routineRequestError(ctx context.Context, action string, err error) error {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return newCommandError("device routine interrupted", "interrupted", exitCodeInterrupted)
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return newCommandError(action+" timed out", "timeout")
	}

	return safeSDKError(action, err)
}

func discoverRoutineControls(specification tuya.Specification) []routineControl {
	controls := make([]routineControl, 0, len(specification.Functions))

	for _, function := range specification.Functions {
		control, ok := routineControlFromFunction(function)
		if ok {
			controls = append(controls, control)
		}
	}

	assignRoutineNames(controls)
	sort.Slice(controls, func(left, right int) bool { return controls[left].Name < controls[right].Name })

	return controls
}

func routineControlFromFunction(function tuya.SpecificationFunction) (routineControl, bool) {
	code := strings.TrimSpace(function.Code)
	if code == "" {
		var empty routineControl

		return empty, false
	}

	kind := supportedRoutineType(function.Type)
	if kind == "" {
		var empty routineControl

		return empty, false
	}

	metadata, metadataOK := decodeRoutineMetadata(function.Values)
	if !metadataOK {
		metadata = map[string]any{}
	}

	control := routineControl{
		Name:    "",
		Type:    kind,
		Actions: nil,
		Minimum: nil,
		Maximum: nil,
		Step:    nil,
		Options: nil,
		code:    code,
		label:   strings.TrimSpace(function.Name),
	}

	control.Name = commonRoutineName(code, kind)

	if control.Name == "" {
		control.Name = routineSlug(control.label)
	}

	if control.Name == "" {
		var empty routineControl

		return empty, false
	}

	switch kind {
	case routineBoolean:
		control.Actions = []string{"on", "off"}
	case routineInteger:
		control.Minimum, control.Maximum, control.Step = integerRoutineBounds(metadata)
	case routineEnum:
		control.Options = enumRoutineOptions(metadata["range"])
		if len(control.Options) == 0 {
			var empty routineControl

			return empty, false
		}
	}

	return control, true
}

func supportedRoutineType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "boolean", "bool":
		return routineBoolean
	case "integer":
		return routineInteger
	case "enum":
		return routineEnum
	default:
		return ""
	}
}

func commonRoutineName(code, kind string) string {
	code = strings.ToLower(strings.TrimSpace(code))

	if kind == routineBoolean && oneOf(code, "switch", "switch_1", "switch_2", "switch_led", "switch_led_1", "switch_led_2") {
		return "switch"
	}

	if kind == routineInteger && oneOf(code, "bright_value", "bright_value_1", "bright_value_v2", "brightness") {
		return "brightness"
	}

	if oneOf(code, "temp", "temp_value", "temp_set", "temp_set_c", "temp_set_f") && (kind == routineInteger || kind == routineEnum) {
		return "temperature"
	}

	return ""
}

func oneOf(value string, options ...string) bool {
	return slices.Contains(options, value)
}

func routineSlug(value string) string {
	var (
		builder           strings.Builder
		previousSeparator = true
	)

	for _, character := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			builder.WriteRune(character)

			previousSeparator = false

			continue
		}

		if !previousSeparator {
			builder.WriteByte('-')

			previousSeparator = true
		}
	}

	return strings.Trim(builder.String(), "-")
}

func assignRoutineNames(controls []routineControl) {
	counts := make(map[string]int, len(controls))
	for _, control := range controls {
		counts[control.Name]++
	}

	for index := range controls {
		if counts[controls[index].Name] < duplicateRoutineNameThreshold {
			continue
		}

		name := routineSlug(controls[index].label)
		if name == "" || name == controls[index].Name {
			name = routineSlug(controls[index].code)
		}

		if name != "" {
			controls[index].Name = name
		}
	}

	seen := make(map[string]int, len(controls))
	for index := range controls {
		seen[controls[index].Name]++
		if seen[controls[index].Name] > 1 {
			controls[index].Name = fmt.Sprintf("%s-%d", controls[index].Name, seen[controls[index].Name])
		}
	}
}

func decodeRoutineMetadata(value any) (map[string]any, bool) {
	if value == nil {
		return map[string]any{}, true
	}

	var raw []byte

	if text, ok := value.(string); ok {
		text = strings.TrimSpace(text)
		if text == "" || text == "{}" {
			return map[string]any{}, true
		}

		raw = []byte(text)
	} else {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, false
		}

		raw = encoded
	}

	var metadata map[string]any

	err := json.Unmarshal(raw, &metadata)
	if err != nil || metadata == nil {
		return nil, false
	}

	return metadata, true
}

func integerRoutineBounds(metadata map[string]any) (*int64, *int64, *int64) {
	rangeMetadata := metadata
	if nested, ok := metadata["range"].(map[string]any); ok {
		rangeMetadata = nested
	}

	minimum := metadataInteger(rangeMetadata["min"])
	maximum := metadataInteger(rangeMetadata["max"])
	step := metadataInteger(rangeMetadata["step"])

	if minimum != nil && maximum != nil && *minimum > *maximum {
		return nil, nil, nil
	}

	if step != nil && *step <= 0 {
		step = nil
	}

	return minimum, maximum, step
}

func metadataInteger(value any) *int64 {
	switch number := value.(type) {
	case int:
		return integerPointer(int64(number))
	case int64:
		return integerPointer(number)
	case json.Number:
		return jsonInteger(number)
	case float64:
		return floatInteger(number)
	case string:
		return stringInteger(number)
	default:
		return nil
	}
}

func integerPointer(value int64) *int64 {
	return &value
}

func jsonInteger(value json.Number) *int64 {
	parsed, err := value.Int64()
	if err != nil {
		return nil
	}

	return integerPointer(parsed)
}

func floatInteger(value float64) *int64 {
	if math.Trunc(value) != value || value >= float64(math.MaxInt64) || value < float64(math.MinInt64) {
		return nil
	}

	return integerPointer(int64(value))
}

func stringInteger(value string) *int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return nil
	}

	return integerPointer(parsed)
}

func enumRoutineOptions(value any) []string {
	options, ok := value.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(options))

	for _, option := range options {
		text, isString := option.(string)
		if !isString || strings.TrimSpace(text) == "" || slices.Contains(result, text) {
			continue
		}

		result = append(result, text)
	}

	return result
}

func writeDeviceRoutines(out io.Writer, jsonOutput bool, deviceID string, controls []routineControl) error {
	if jsonOutput {
		return writeJSON(out, deviceRoutinesResult{DeviceID: deviceID, Routines: controls})
	}

	if len(controls) == 0 {
		_, err := fmt.Fprintf(out, "No supported boolean, integer, or enum routines found for device %s.\n", deviceID)
		if err != nil {
			return newCommandError("could not write command output", "output")
		}

		return nil
	}

	for _, control := range controls {
		_, err := fmt.Fprintf(out, "%s\t%s\t%s\n", control.Name, control.Type, routineDetails(control))
		if err != nil {
			return newCommandError("could not write command output", "output")
		}
	}

	return nil
}

func routineDetails(control routineControl) string {
	switch control.Type {
	case routineBoolean:
		return strings.Join(control.Actions, "|")
	case routineInteger:
		parts := []string{}
		if control.Minimum != nil {
			parts = append(parts, fmt.Sprintf("min %d", *control.Minimum))
		}

		if control.Maximum != nil {
			parts = append(parts, fmt.Sprintf("max %d", *control.Maximum))
		}

		if control.Step != nil {
			parts = append(parts, fmt.Sprintf("step %d", *control.Step))
		}

		if len(parts) == 0 {
			return "integer"
		}

		return strings.Join(parts, ", ")
	case routineEnum:
		return strings.Join(control.Options, "|")
	default:
		return ""
	}
}

func findRoutineControl(controls []routineControl, selector string) (routineControl, error) {
	for _, control := range controls {
		if strings.EqualFold(control.Name, strings.TrimSpace(selector)) {
			return control, nil
		}
	}

	return routineControl{}, newCommandError("routine is not supported by this device; run devices routines first", "not_found")
}

func parseRoutineValue(control routineControl, value string) (any, error) {
	switch control.Type {
	case routineBoolean:
		return parseBooleanRoutineValue(value)
	case routineInteger:
		return parseIntegerRoutineValue(control, value)
	case routineEnum:
		return parseEnumRoutineValue(control, value)
	default:
		return nil, usageError("routine type is not supported")
	}
}

func parseBooleanRoutineValue(value string) (any, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on", "true":
		return true, nil
	case "off", "false":
		return false, nil
	default:
		return nil, usageError("boolean routines accept on or off")
	}
}

func parseIntegerRoutineValue(control routineControl, value string) (any, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return nil, usageError("integer routines require a whole-number value")
	}

	if control.Minimum != nil && parsed < *control.Minimum || control.Maximum != nil && parsed > *control.Maximum {
		return nil, usageError("integer routine value is outside the discovered range")
	}

	if control.Minimum != nil && control.Step != nil && !integerMatchesStep(parsed, *control.Minimum, *control.Step) {
		return nil, usageError("integer routine value does not match the discovered step")
	}

	return parsed, nil
}

func integerMatchesStep(value, minimum, step int64) bool {
	valueRemainder := value % step
	if valueRemainder < 0 {
		valueRemainder += step
	}

	minimumRemainder := minimum % step
	if minimumRemainder < 0 {
		minimumRemainder += step
	}

	return valueRemainder == minimumRemainder
}

func parseEnumRoutineValue(control routineControl, value string) (any, error) {
	if slices.Contains(control.Options, value) {
		return value, nil
	}

	return nil, usageError("enum value must match an option from the device specification")
}
