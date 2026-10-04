package tuya

import (
	"context"
	"encoding/json"
	"fmt"
)

// APIError exposes safe diagnostics without vendor messages, bodies, or URLs.
// It does not retain an underlying transport cause, which can contain credentials.
type APIError struct {
	category string
	status   int
	code     string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Tuya API %s failure (HTTP %d, provider code %s)", e.category, e.status, e.code)
}

// DiagnosticCategory identifies the request failure stage.
func (e *APIError) DiagnosticCategory() string { return e.category }

// HTTPStatus returns the response status, or zero if no response arrived.
func (e *APIError) HTTPStatus() int { return e.status }

// ProviderCode returns a bounded decimal provider code, or an empty string.
func (e *APIError) ProviderCode() string { return e.code }

func transportFailure(ctx context.Context) error {
	err := ctx.Err()
	if err != nil {
		return clientError(ErrorTransport, err)
	}

	return clientError(ErrorTransport, newAPIError("transport", 0, nil))
}

func newAPIError(category string, status int, body []byte) *APIError {
	var envelope struct {
		Code json.RawMessage `json:"code"`
	}

	err := json.Unmarshal(body, &envelope)
	if err != nil {
		return &APIError{category: category, status: status, code: ""}
	}

	code := string(envelope.Code)
	if len(code) > 0 && code[0] == '"' {
		if json.Unmarshal(envelope.Code, &code) != nil {
			code = ""
		}
	}

	return &APIError{category: category, status: status, code: boundedProviderCode(code)}
}

func boundedProviderCode(code string) string {
	digits := code
	if len(digits) > 0 && digits[0] == '-' {
		digits = digits[1:]
	}

	if len(digits) == 0 || len(digits) > 20 {
		return ""
	}

	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return ""
		}
	}

	return code
}
