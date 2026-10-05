package main

import (
	"slices"
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

func TestSchemaParsersAcceptCRLF(t *testing.T) {
	t.Parallel()

	openAPI := "openapi: 3.1.0\npaths:\n  /v1.0/devices:\n    get:\n      operationId: getDevices\ncomponents:\n  schemas: {}\n"

	routes, err := parseRoutes(strings.ReplaceAll(openAPI, "\n", "\r\n"))
	if err != nil || len(routes) != 1 {
		t.Fatalf("CRLF OpenAPI schema failed to parse: routes=%v err=%v", routes, err)
	}

	asyncAPI := "asyncapi: 3.0.0\nchannels:\n  ownerEvents:\n    address: '{ownerTopic}'\n"

	channels, err := parseChannels(strings.ReplaceAll(asyncAPI, "\n", "\r\n"))
	if err != nil || len(channels) != 1 || channels[0].id != "ownerEvents" {
		t.Fatalf("CRLF AsyncAPI schema failed to parse: channels=%v err=%v", channels, err)
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

func TestNetworkBoundaryRejectsUnlistedSocketEdges(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		source string
	}{
		{
			name: "unlisted x/net WebSocket dial",
			source: `package tuya
import xwebsocket "golang.org/x/net/websocket"
func bad() {
 connection, err := xwebsocket.Dial("wss://mqtt.example.invalid/socket", "mqtt", "https://client.example.invalid")
 _ = connection
 _ = err
}
`,
		},
		{
			name: "captured direct dial method value",
			source: `package tuya
import "net"
func bad() {
 dial := (&net.Dialer{}).DialContext
 _ = dial
}
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

func TestNetworkBoundaryChecksEveryShippedSourceRootAndKeepsOfflinePipePositive(t *testing.T) {
	t.Parallel()

	roots := productionSourceRoots()
	for _, root := range []string{"pkg", "cmd", "examples"} {
		if !slices.Contains(roots, root) {
			t.Fatalf("production source scan omits shipped module root %q", root)
		}
	}

	cliSource := []byte(`package cli
import xwebsocket "golang.org/x/net/websocket"
func bad() {
 connection, err := xwebsocket.Dial("wss://mqtt.example.invalid/socket", "mqtt", "https://client.example.invalid")
 _ = connection
 _ = err
}
`)

	err := validateSourceNetworkBoundary("cmd/go-tuya/internal/cli/unlisted_socket.go", cliSource)
	if err == nil {
		t.Fatal("CLI network edge escaped the gate")
	}

	webSocketDial := []byte(`package tuya
import xwebsocket "golang.org/x/net/websocket"
func bad() {
 connection, err := xwebsocket.Dial("wss://mqtt.example.invalid/socket", "mqtt", "https://client.example.invalid")
 _ = connection
_ = err
}
`)

	for _, filename := range []string{
		"pkg/tuya/unlisted_socket.go",
		"cmd/go-tuya/internal/cli/unlisted_socket.go",
		"examples/unlisted_socket.go",
	} {
		err := validateSourceNetworkBoundary(filename, webSocketDial)
		if err == nil {
			t.Errorf("unlisted WebSocket network edge in %s escaped the gate", filename)
		}
	}

	offlinePipe := []byte(`package tuya
import "net"
func paired() (net.Conn, net.Conn) {
 client, server := net.Pipe()
 return client, server
}
`)

	err = validateSourceNetworkBoundary("pkg/tuya/offline_pair.go", offlinePipe)
	if err != nil {
		t.Fatalf("offline net.Pipe positive control was rejected: %v", err)
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
 "bytes"
 "io"
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
 request, err := http.NewRequestWithContext(ctx, operation.Method, target.String(), requestBody(body))
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
}
func requestBody(body []byte) io.Reader {
 if body == nil { return nil }
 return bytes.NewReader(body)
}`

type httpBoundaryMutation struct {
	name     string
	needle   string
	replace  string
	needle2  string
	replace2 string
}

const httpBoundarySendCall = "response, err := client.Do(request)"

func httpBoundaryMutations() []httpBoundaryMutation {
	mutations := httpBindingMutations()
	mutations = append(mutations, httpValidatedRouteMutations()...)
	mutations = append(mutations, httpRequestFieldMutations()...)
	mutations = append(mutations, httpRequestEscapeMutations()...)
	mutations = append(mutations, httpRequestBodyFactoryMutations()...)

	return mutations
}

func httpValidatedRouteMutations() []httpBoundaryMutation {
	const (
		operationGuard = "if !wire.IsKnownOperation(operation.Method, path) { return nil, errBoundary }"
		pathAssignment = "path, err := formatOperationPath(operation.Path, pathArguments)"
	)

	return []httpBoundaryMutation{
		httpMutation("schema route appended after guard", operationGuard, operationGuard+"\n path += \"/v1.0/unlisted\""),
		httpMutation("schema route escaped through pointer after guard", operationGuard, operationGuard+"\n mutateRoute(&path)"),
		httpMutation("HTTP method changed after guard", operationGuard, operationGuard+"\n operation.Method = \"DELETE\""),
		httpTwoPartMutation(
			"route pointer captured before guard",
			pathAssignment,
			"var path string\n pathPointer := &path\n "+pathAssignment,
			operationGuard,
			operationGuard+"\n *pathPointer += \"/v1.0/unlisted\"",
		),
		httpTwoPartMutation(
			"operation pointer captured before guard",
			pathAssignment,
			"operationPointer := &operation\n "+pathAssignment,
			operationGuard,
			operationGuard+"\n operationPointer.Method = \"DELETE\"",
		),
	}
}

func httpBindingMutations() []httpBoundaryMutation {
	const (
		operationGuard = "if !wire.IsKnownOperation(operation.Method, path) { return nil, errBoundary }"
		pathAssignment = "path, err := formatOperationPath(operation.Path, pathArguments)"
	)

	return []httpBoundaryMutation{
		httpMutation("wire import shadow", pathAssignment, "wire := fake; path, err := formatOperationPath(operation.Path, pathArguments)"),
		httpMutation("http import shadow", pathAssignment, "http := fake; path, err := formatOperationPath(operation.Path, pathArguments)"),
		httpMutation("client parameter shadow", pathAssignment, "client := fake; path, err := formatOperationPath(operation.Path, pathArguments)"),
		httpMutation("unrelated helper validation", operationGuard, "if err != nil { return nil, err }"),
		httpMutation("dead branch validation", operationGuard, "if false { "+operationGuard+" }"),
		httpMutation("key guard omitted from request path", "err = validateRequestKeys(query, headers)", "err = nil"),
	}
}

func httpRequestFieldMutations() []httpBoundaryMutation {
	return []httpBoundaryMutation{
		httpRequestMutation("request URL path mutated before send", "request.URL.Path = \"/v1.0/ghost\""),
		httpRequestMutation("request method mutated before send", "request.Method = \"DELETE\""),
		httpRequestMutation("request body mutated before send", "request.Body = nil"),
		httpRequestMutation("request URL userinfo mutated before send", "request.URL.User = nil"),
		httpRequestMutation("request body factory mutated before send", "request.GetBody = nil"),
		httpRequestMutation("request content length framing mutated before send", "request.ContentLength = -1"),
		httpRequestMutation("request transfer encoding framing mutated before send", "request.TransferEncoding = []string{\"chunked\"}"),
		httpRequestMutation("unvalidated header mutation before send", "request.Header.Add(\"X-Tuya-Forged\", \"value\")"),
	}
}

func httpRequestEscapeMutations() []httpBoundaryMutation {
	return []httpBoundaryMutation{
		httpRequestMutation("request alias escapes before send", "requestAlias := request\n requestAlias.URL.Path = \"/v1.0/ghost\""),
		httpRequestMutation("request passed to unknown mutator", "rewriteRequest(request)"),
		httpRequestMutation("request backing body bytes mutated before send", "body[0] = 'x'"),
		httpTwoPartMutation(
			"body backing bytes alias mutated before send",
			"request, err := http.NewRequestWithContext(ctx, operation.Method, target.String(), requestBody(body))",
			"bodyAlias := body\n request, err := http.NewRequestWithContext(ctx, operation.Method, target.String(), requestBody(body))",
			httpBoundarySendCall,
			"bodyAlias[0] = 'x'\n "+httpBoundarySendCall,
		),
		httpRequestMutation("request body mutation captured by callback", "mutateBody := func() { body[0] = 'x' }\n mutateBody()"),
		httpTwoPartMutation(
			"request passed to cross-package mutator",
			"package httptransport\nimport (",
			"package httptransport\nimport (\n rewriter \"example.com/requestrewriter\"",
			httpBoundarySendCall,
			"rewriter.Modify(request)\n "+httpBoundarySendCall,
		),
	}
}

func httpRequestBodyFactoryMutations() []httpBoundaryMutation {
	const requestConstruction = "request, err := http.NewRequestWithContext(ctx, operation.Method, target.String(), requestBody(body))"

	const bodyFactory = `func requestBody(body []byte) io.Reader {
 if body == nil { return nil }
 return bytes.NewReader(body)
}`

	return []httpBoundaryMutation{
		httpMutation(
			"request body helper mutates backing bytes through a callback",
			bodyFactory,
			`func requestBody(body []byte) io.Reader {
 if body == nil { return nil }
 reader := bytes.NewReader(body)
 go func() { body[0] = 'x' }()
 return reader
}`,
		),
		httpMutation(
			"local request body callback shadows checked factory",
			requestConstruction,
			`requestBody := func(payload []byte) *bytes.Reader {
 reader := bytes.NewReader(payload)
 go func() { payload[0] = 'x' }()
 return reader
}
`+requestConstruction,
		),
	}
}

func httpMutation(name, needle, replace string) httpBoundaryMutation {
	return httpBoundaryMutation{name: name, needle: needle, replace: replace, needle2: "", replace2: ""}
}

func httpRequestMutation(name, statements string) httpBoundaryMutation {
	return httpMutation(name, httpBoundarySendCall, statements+"\n "+httpBoundarySendCall)
}

func httpTwoPartMutation(name, needle, replace, needle2, replace2 string) httpBoundaryMutation {
	return httpBoundaryMutation{name: name, needle: needle, replace: replace, needle2: needle2, replace2: replace2}
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
			if testCase.needle2 != "" {
				mutated = strings.Replace(mutated, testCase.needle2, testCase.replace2, 1)
			}

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
