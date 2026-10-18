package tuya

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const maxTraceIDLength = 64

// APIError exposes safe diagnostics without vendor messages, bodies, or URLs.
// It does not retain an underlying transport cause, which can contain credentials.
type APIError struct {
	category  string
	status    int
	code      string
	traceID   string
	requestID string
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

// ProviderTraceID returns the bounded provider trace identifier, when supplied.
func (e *APIError) ProviderTraceID() string { return e.traceID }

// ProviderRequestID returns the generated outgoing request identifier.
func (e *APIError) ProviderRequestID() string { return e.requestID }

func transportFailure(ctx context.Context) error {
	err := ctx.Err()
	if err != nil {
		return clientError(ErrorTransport, err)
	}

	return clientError(ErrorTransport, newAPIError("transport", 0, nil))
}

func newAPIError(category string, status int, body []byte) *APIError {
	var envelope struct {
		Code    json.RawMessage `json:"code"`
		TraceID json.RawMessage `json:"tid"`
	}

	err := json.Unmarshal(body, &envelope)
	if err != nil {
		return &APIError{category: category, status: status, code: "", traceID: "", requestID: ""}
	}

	code := string(envelope.Code)
	if len(code) > 0 && code[0] == '"' {
		if json.Unmarshal(envelope.Code, &code) != nil {
			code = ""
		}
	}

	var traceID string

	_ = json.Unmarshal(envelope.TraceID, &traceID)

	return &APIError{category: category, status: status, code: boundedProviderCode(code), traceID: boundedTraceID(traceID), requestID: ""}
}

// Only opaque identifier characters are retained; vendor messages and URLs
// never become diagnostics. This value is deliberately absent from Error().
func boundedTraceID(value string) string {
	if len(value) == 0 || len(value) > maxTraceIDLength {
		return ""
	}

	for _, character := range value {
		if !traceIDCharacter(character) {
			return ""
		}
	}

	return value
}

func traceIDCharacter(character rune) bool {
	return (character >= '0' && character <= '9') || (character >= 'a' && character <= 'z') ||
		(character >= 'A' && character <= 'Z') || character == '-' || character == '_'
}

func correlateFailure(err error, requestID string) error {
	var diagnostic *APIError
	if errors.As(err, &diagnostic) {
		diagnostic.requestID = requestID
	}

	return err
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
