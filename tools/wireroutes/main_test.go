package main

import (
	"strings"
	"testing"
)

func TestParseRoutesAndGenerate(t *testing.T) {
	t.Parallel()

	source := `openapi: 3.1.0
paths:
  /v1.0/devices/{device_id}:
    get:
      operationId: getDevice
    delete:
      operationId: deleteDevice
components:
  schemas: {}
`

	routes, err := parseRoutes(source)
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) != 2 {
		t.Fatalf("got %d routes, want 2", len(routes))
	}

	generated, err := generate(routes)
	if err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{
		`RouteGetDevice`,
		`"/v1.0/devices/%s"`,
		`MethodDeleteDevice`,
		`"DELETE"`,
	} {
		if !strings.Contains(string(generated), expected) {
			t.Fatalf("generated routes missing %q", expected)
		}
	}
}

func TestParseChannelsAndGenerate(t *testing.T) {
	t.Parallel()

	source := `asyncapi: 3.0.0
channels:
  deviceStatus:
    address: '{deviceTopic}/sta'
operations:
  receiveStatus:
    action: receive
`

	channels, err := parseChannels(source)
	if err != nil {
		t.Fatal(err)
	}

	generated, err := generateMQTT(channels)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(generated), `MQTTChannelDeviceStatus`) ||
		!strings.Contains(string(generated), `"{deviceTopic}/sta"`) {
		t.Fatalf("generated channels missing deviceStatus: %s", generated)
	}
}

func TestGateRejectsUnknownPath(t *testing.T) {
	t.Parallel()

	missing, err := missingPathLiterals("synthetic.go", []byte(`package tuya
func bad() { _ = "/v1.0/ghosts" }
`), map[string]bool{"/v1.0/devices": true})
	if err != nil {
		t.Fatal(err)
	}

	if len(missing) != 1 || !strings.Contains(missing[0], "/v1.0/ghosts") {
		t.Fatalf("unknown path escaped gate: %v", missing)
	}
}

func TestOperationGateResolvesGeneratedImportAndShadowing(t *testing.T) {
	t.Parallel()

	operations := map[string]bool{"OperationGetDevice": true}
	channels := map[string]bool{}

	valid := []byte(`package tuya
import model "github.com/portpowered/go-tuya/pkg/dependencymodels"
func call(c *Client) { c.requestOperation(ctx, model.OperationGetDevice(), nil, nil) }
`)

	err := validateCallSites("pkg/tuya/valid.go", valid, operations, channels)
	if err != nil {
		t.Fatalf("valid aliased generated package was rejected: %v", err)
	}

	invalid := [][]byte{
		[]byte(`package tuya
import wire "example.com/unverified/dependencymodels"
func call(c *Client) { c.requestOperation(ctx, wire.OperationGetDevice(), nil, nil) }
`),
		[]byte(`package tuya
import wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
func call(c *Client) { wire := fake; c.requestOperation(ctx, wire.OperationGetDevice(), nil, nil) }
`),
	}
	for index, source := range invalid {
		err := validateCallSites("pkg/tuya/invalid.go", source, operations, channels)
		if err == nil {
			t.Fatalf("invalid generated import case %d escaped the gate", index)
		}
	}
}

func TestNetworkBoundaryRejectsConvenienceCallsMethodValuesAndPahoImports(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		source string
	}{
		{
			name: "HTTP convenience",
			source: `package tuya
import nethttp "net/http"
func bad() { _, _ = nethttp.Get("https://example.invalid") }
`,
		},
		{
			name: "HTTP method value",
			source: `package tuya
import "net/http"
func bad(client *http.Client) { send := client.Do; _ = send }
`,
		},
		{
			name: "HTTP client convenience method value",
			source: `package tuya
import nethttp "net/http"
func bad(client *nethttp.Client) { get := client.Get; _ = get }
`,
		},
		{
			name: "HTTP default client convenience method",
			source: `package tuya
import "net/http"
func bad() { get := http.DefaultClient.Get; _ = get }
`,
		},
		{
			name: "Paho outside wrapper",
			source: `package tuya
import mqtt "github.com/eclipse/paho.mqtt.golang"
func bad() { _ = mqtt.NewClientOptions() }
`,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := validateSourceNetworkBoundary("pkg/tuya/synthetic.go", []byte(testCase.source))
			if err == nil {
				t.Fatal("unregistered network edge escaped the gate")
			}
		})
	}
}

func TestHTTPBoundaryRequiresGeneratedValidationAndInjectedSend(t *testing.T) {
	t.Parallel()

	bad := []byte(`package httptransport
import "net/http"
func Do(client *http.Client, method, target string) (*http.Response, error) {
 request, err := http.NewRequestWithContext(ctx, method, target, nil)
 if err != nil { return nil, err }
 return client.Do(request)
}
`)

	err := validateSourceNetworkBoundary(httpBoundary, bad)
	if err == nil {
		t.Fatal("HTTP boundary without generated route validation escaped the gate")
	}
}

const httpBoundaryValidSource = `package httptransport
import (
 "net/http"
 wire "github.com/portpowered/go-tuya/pkg/dependencymodels"
)
func Do(
 ctx any, client *http.Client, origin string, operation wire.Operation,
 pathArguments []any, query map[string][]string, headers map[string]string, body []byte,
) (*http.Response, error) {
 path, err := formatOperationPath(operation.Path, pathArguments)
 if err != nil { return nil, err }
 if !wire.IsKnownOperation(operation.Method, path) { return nil, errBoundary }
 err = validateRequestKeys(query, headers)
 if err != nil { return nil, err }
 target, err := requestURL(origin, path, query)
 if err != nil { return nil, err }
 request, err := http.NewRequestWithContext(ctx, operation.Method, target.String(), body)
 if err != nil { return nil, err }
 for name, value := range headers { request.Header.Set(name, value) }
 response, err := client.Do(request)
 if err != nil { return nil, err }
 return response, nil
}
func validateRequestKeys(query map[string][]string, headers map[string]string) error {
 for name := range query { if !wire.IsKnownQueryParam(name) { return errBoundary } }
 for name := range headers { if !wire.IsKnownHeader(name) { return errBoundary } }
 return nil
}`

type httpBoundaryMutation struct {
	name    string
	needle  string
	replace string
}

func httpBoundaryMutations() []httpBoundaryMutation {
	const (
		operationGuard = "if !wire.IsKnownOperation(operation.Method, path) { return nil, errBoundary }"
		pathAssignment = "path, err := formatOperationPath(operation.Path, pathArguments)"
	)

	return []httpBoundaryMutation{
		{
			name:    "wire import shadow",
			needle:  pathAssignment,
			replace: "wire := fake; path, err := formatOperationPath(operation.Path, pathArguments)",
		},
		{
			name:    "http import shadow",
			needle:  pathAssignment,
			replace: "http := fake; path, err := formatOperationPath(operation.Path, pathArguments)",
		},
		{
			name:    "client parameter shadow",
			needle:  pathAssignment,
			replace: "client := fake; path, err := formatOperationPath(operation.Path, pathArguments)",
		},
		{
			name:    "unrelated helper validation",
			needle:  operationGuard,
			replace: "if err != nil { return nil, err }",
		},
		{
			name:    "dead branch validation",
			needle:  operationGuard,
			replace: "if false { " + operationGuard + " }",
		},
		{
			name:    "key guard omitted from request path",
			needle:  "err = validateRequestKeys(query, headers)",
			replace: "err = nil",
		},
	}
}

func TestHTTPBoundaryRejectsShadowedAndUnrelatedValidation(t *testing.T) {
	t.Parallel()

	err := validateSourceNetworkBoundary(httpBoundary, []byte(httpBoundaryValidSource))
	if err != nil {
		t.Fatalf("valid schema-bound HTTP boundary rejected: %v", err)
	}

	for _, testCase := range httpBoundaryMutations() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			mutated := strings.Replace(httpBoundaryValidSource, testCase.needle, testCase.replace, 1)
			if mutated == httpBoundaryValidSource {
				t.Fatal("test mutation did not match the source fixture")
			}

			err := validateSourceNetworkBoundary(httpBoundary, []byte(mutated))
			if err == nil {
				t.Fatal("unregistered or unbound HTTP network edge escaped the gate")
			}
		})
	}
}

func TestGateRejectsChangedMethodAndUnknownChannel(t *testing.T) {
	t.Parallel()

	operations := map[string]bool{"OperationGetDevice": true}
	channels := map[string]bool{"MQTTChannelDeviceStatus": true}

	cases := []struct {
		name   string
		source string
	}{
		{"handwritten method", "package tuya\nfunc bad() { c.EncryptedClient.Post(ctx, \"/v1.0/devices\", nil, req)"},
		{"unknown operation", "package tuya\nfunc bad() { c.EncryptedClient.requestOperation(ctx, wire.OperationGhost(), nil, req)"},
		{"unknown channel", `package tuya
func bad() { subscribeChannel(client, wire.MQTTChannelGhost, "topic") }
`},
		{"direct subscription", `package tuya
func bad() { client.Subscribe("topic", 0, nil) }
`},
		{"direct unsubscription", `package tuya
func bad() { client.Unsubscribe("topic") }
`},
		{"request builder outside makeRequest", `package tuya
func bad() { c.newEncryptedRequest(ctx, method, path, payload, headers) }
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateCallSites("synthetic.go", []byte(tc.source), operations, channels)
			if err == nil {
				t.Fatal("invalid callsite escaped gate")
			}
		})
	}

	err := validateSourceNetworkBoundary("pkg/tuya/synthetic.go", []byte(`package tuya
import http "net/http"
func bad(ctx context.Context, url string) { http.NewRequestWithContext(ctx, http.MethodDelete, url, nil) }
`))
	if err == nil {
		t.Fatal("direct HTTP request construction escaped transport boundary")
	}
}
