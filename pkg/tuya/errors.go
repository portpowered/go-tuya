package tuya

import "fmt"

// ErrorKind identifies a stable class of client failure.
type ErrorKind string

const (
	ErrorInvalidOperation ErrorKind = "invalid_operation"
	ErrorUnauthorized     ErrorKind = "unauthorized"
	ErrorNotFound         ErrorKind = "not_found"
	ErrorTransport        ErrorKind = "transport"
	ErrorProvider         ErrorKind = "provider"
	ErrorProtocol         ErrorKind = "protocol"
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
