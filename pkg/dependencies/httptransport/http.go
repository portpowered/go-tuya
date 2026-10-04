// Package httptransport owns the SDK's outbound HTTP request boundary.
package httptransport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)

var (
	// ErrInvalidOperation identifies a request that does not match the generated schema.
	ErrInvalidOperation = errors.New("invalid HTTP operation")
	// ErrTransport identifies a failure from the injected HTTP client.
	ErrTransport = errors.New("HTTP transport failed")

	errPathPlaceholders = errors.New("route placeholders and path arguments do not match")
	errPathArgument     = errors.New("path arguments must be non-empty strings without dot-segment values")
)

// Do builds and sends one schema-bound HTTP request through the supplied client.
// The operation descriptor owns the method and route; path arguments fill only
// its schema-declared path placeholders.
// The caller must close the successful response's body.
func Do(
	ctx context.Context,
	client *http.Client,
	origin string,
	operation wire.Operation,
	pathArguments []any,
	query url.Values,
	headers map[string]string,
	body []byte,
) (*http.Response, error) {
	if client == nil {
		return nil, fmt.Errorf("%w: HTTP client is nil", ErrInvalidOperation)
	}

	err := validateOrigin(origin)
	if err != nil {
		return nil, err
	}

	path, err := formatOperationPath(operation.Path, pathArguments)
	if err != nil {
		return nil, fmt.Errorf("%w: format schema route: %w", ErrInvalidOperation, err)
	}

	if !wire.IsKnownOperation(operation.Method, path) {
		return nil, fmt.Errorf("%w: %s %s is not in the API schema", ErrInvalidOperation, operation.Method, path)
	}

	err = validateRequestKeys(query, headers)
	if err != nil {
		return nil, err
	}

	target, err := requestURL(origin, path, query)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, operation.Method, target.String(), requestBody(body))
	if err != nil {
		return nil, fmt.Errorf("%w: create request: %w", ErrInvalidOperation, err)
	}

	for name, value := range headers {
		request.Header.Set(name, value)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTransport, err)
	}

	return response, nil
}

func formatOperationPath(template string, arguments []any) (string, error) {
	placeholderCount := strings.Count(template, "%s")
	if placeholderCount != len(arguments) || strings.Contains(strings.ReplaceAll(template, "%s", ""), "%") {
		return "", errPathPlaceholders
	}

	segments := strings.Split(template, "%s")

	var path strings.Builder

	path.Grow(len(template))

	for index, segment := range segments {
		path.WriteString(segment)

		if index == len(arguments) {
			break
		}

		argument, ok := arguments[index].(string)
		if !ok || argument == "" || argument == "." || argument == ".." {
			return "", errPathArgument
		}

		path.WriteString(url.PathEscape(argument))
	}

	return path.String(), nil
}

func validateRequestKeys(query url.Values, headers map[string]string) error {
	for name := range query {
		if !wire.IsKnownQueryParam(name) {
			return fmt.Errorf("%w: query parameter %q is not in the API schema", ErrInvalidOperation, name)
		}
	}

	for name := range headers {
		if !wire.IsKnownHeader(name) {
			return fmt.Errorf("%w: header %q is not in the API schema", ErrInvalidOperation, name)
		}
	}

	return nil
}

func requestURL(origin, path string, query url.Values) (*url.URL, error) {
	target, err := url.Parse(strings.TrimRight(origin, "/") + path)
	if err != nil {
		return nil, fmt.Errorf("%w: build request URL: %w", ErrInvalidOperation, err)
	}

	if target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("%w: build request URL without an absolute origin", ErrInvalidOperation)
	}

	if query != nil {
		target.RawQuery = query.Encode()
	}

	return target, nil
}

func requestBody(body []byte) io.Reader {
	if body == nil {
		return nil
	}

	return bytes.NewReader(body)
}

func validateOrigin(origin string) error {
	parsed, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("%w: parse origin: %w", ErrInvalidOperation, err)
	}

	if (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" ||
		parsed.User != nil ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return fmt.Errorf("%w: origin must be an absolute HTTP URL without credentials, query, or fragment", ErrInvalidOperation)
	}

	return nil
}
