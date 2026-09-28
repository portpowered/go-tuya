package tuya

import (
	"encoding/json"
	"fmt"
	"net/url"
)

func decodeWireResponse[T any](response *EncryptedAPIResponse) (T, error) {
	var decoded T
	if response == nil {
		return decoded, clientError(ErrorProtocol, fmt.Errorf("response is nil"))
	}
	data, err := json.Marshal(response.Body)
	if err != nil {
		return decoded, clientError(ErrorProtocol, fmt.Errorf("error marshalling response: %w", err))
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return decoded, clientError(ErrorProtocol, fmt.Errorf("error unmarshalling response: %w", err))
	}
	return decoded, nil
}

func convertWireValue[T any](value any) (T, error) {
	var converted T
	data, err := json.Marshal(value)
	if err != nil {
		return converted, clientError(ErrorProtocol, fmt.Errorf("error marshalling wire value: %w", err))
	}
	if err := json.Unmarshal(data, &converted); err != nil {
		return converted, clientError(ErrorProtocol, fmt.Errorf("error converting wire value: %w", err))
	}
	return converted, nil
}

func wireRequestMap(value any) (map[string]interface{}, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, clientError(ErrorProtocol, fmt.Errorf("error marshalling wire request: %w", err))
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, clientError(ErrorProtocol, fmt.Errorf("error reading wire request fields: %w", err))
	}
	result := make(map[string]interface{}, len(fields))
	for name, raw := range fields {
		result[name] = raw
	}
	return result, nil
}

func wireQueryValues(value any) (url.Values, error) {
	fields, err := wireRequestMap(value)
	if err != nil {
		return nil, err
	}
	query := make(url.Values, len(fields))
	for name, field := range fields {
		raw, ok := field.(json.RawMessage)
		if !ok || string(raw) == "null" {
			continue
		}
		var text string
		if len(raw) > 0 && raw[0] == '"' {
			if err := json.Unmarshal(raw, &text); err != nil {
				return nil, clientError(ErrorProtocol, fmt.Errorf("error reading wire query parameter %q: %w", name, err))
			}
		} else {
			text = string(raw)
		}
		query.Set(name, text)
	}
	return query, nil
}

func wireStringMap(value any) (map[string]string, error) {
	fields, err := wireRequestMap(value)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(fields))
	for name, field := range fields {
		raw, ok := field.(json.RawMessage)
		if !ok {
			return nil, clientError(ErrorProtocol, fmt.Errorf("wire field %q is not JSON encoded", name))
		}
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, clientError(ErrorProtocol, fmt.Errorf("wire field %q is not a string: %w", name, err))
		}
		result[name] = text
	}
	return result, nil
}
