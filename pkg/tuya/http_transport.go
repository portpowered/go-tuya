package tuya

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/portpowered/go-tuya/pkg/dependencies/httptransport"
	wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)

func doHTTP(
	ctx context.Context,
	client *http.Client,
	origin string,
	operation wire.Operation,
	pathArguments []any,
	query url.Values,
	headers map[string]string,
	body []byte,
) (*http.Response, error) {
	response, err := httptransport.Do(ctx, client, origin, operation, pathArguments, query, headers, body)
	if err != nil {
		kind := ErrorTransport
		if errors.Is(err, httptransport.ErrInvalidOperation) {
			kind = ErrorInvalidOperation
		}

		return nil, clientError(kind, err)
	}

	return response, nil
}
