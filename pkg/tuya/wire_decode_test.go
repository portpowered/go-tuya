package tuya

import (
	"errors"
	"testing"

	wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)

const syntheticValue = "synthetic"

//nolint:tagliatelle // These deliberately unregistered wire keys prove unknown keys fail closed.
type unregisteredQuery struct {
	Value string `json:"brandNewQueryKey"`
}

//nolint:tagliatelle // This deliberately unregistered wire key proves unknown keys fail closed.
type unregisteredHeader struct {
	Value string `json:"X-BrandNew"`
}

func TestGeneratedQueryAndHeaderKeyValidation(t *testing.T) {
	t.Parallel()

	query, err := wireQueryValues(wire.EncryptedRequestQuery{Encdata: "synthetic-encdata"})
	if err != nil {
		t.Fatal(err)
	}

	if got := query.Get(wire.QueryParamEncdata); got != "synthetic-encdata" {
		t.Fatalf("generated query value = %q", got)
	}

	_, err = wireQueryValues(unregisteredQuery{Value: syntheticValue})

	var clientErr *ClientError

	if !errors.As(err, &clientErr) || clientErr.Kind != ErrorInvalidOperation {
		t.Fatalf("unregistered query key error = %v, want invalid operation", err)
	}

	headers, err := wireStringMap(wire.EncryptedRequestHeaders{
		XAppKey:    "synthetic-app",
		XRequestId: syntheticValue,
		XSid:       syntheticValue,
		XSign:      syntheticValue,
		XTime:      syntheticValue,
		XToken:     syntheticValue,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := headers[wire.HeaderXAppKey]; got != "synthetic-app" {
		t.Fatalf("generated header value = %q", got)
	}

	_, err = wireStringMap(unregisteredHeader{Value: syntheticValue})

	clientErr = nil

	if !errors.As(err, &clientErr) || clientErr.Kind != ErrorInvalidOperation {
		t.Fatalf("unregistered header key error = %v, want invalid operation", err)
	}
}
