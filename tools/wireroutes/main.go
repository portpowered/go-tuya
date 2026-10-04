// Command wireroutes generates Go route formats from the checked-in OpenAPI paths.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const generatedSourcePermissions = 0o644

const (
	openAPISource = "api/openapi.yaml"
	routeTarget   = "pkg/dependencymodels/routes.gen.go"
	mqttSource    = "api/mqtt.asyncapi.yaml"
	mqttTarget    = "pkg/dependencymodels/mqtt.gen.go"
	wireImport    = "github.com/portpowered/go-tuya/pkg/dependencymodels"
	pahoImport    = "github.com/eclipse/paho.mqtt.golang"
	httpBoundary  = "pkg/dependencies/httptransport/http.go"
	mqttBoundary  = "pkg/dependencies/mqtttransport/paho.go"
)

var parameter = regexp.MustCompile(`\{[^{}]+\}`)

func main() {
	check := flag.Bool("check", false, "fail when generated routes differ")

	flag.Parse()

	files, err := generateFiles()
	if err != nil {
		fatal(err)
	}

	if *check {
		err = checkGeneratedFiles(files.routes, files.channels, routeTarget, mqttTarget, files.routesData, files.mqttData)
		if err != nil {
			fatal(err)
		}

		return
	}

	err = writeGeneratedFiles(files)
	if err != nil {
		fatal(err)
	}
}

type generatedFiles struct {
	routes     []route
	channels   []channel
	routesData []byte
	mqttData   []byte
}

func generateFiles() (generatedFiles, error) {
	// #nosec G304 -- openAPISource is the fixed checked-in OpenAPI schema path.
	data, err := os.ReadFile(openAPISource)
	if err != nil {
		return generatedFiles{}, fmt.Errorf("read OpenAPI schema %q: %w", openAPISource, err)
	}

	routes, err := parseRoutes(string(data))
	if err != nil {
		return generatedFiles{}, err
	}

	routesData, err := generate(routes)
	if err != nil {
		return generatedFiles{}, err
	}

	// #nosec G304 -- mqttSource is the fixed checked-in AsyncAPI schema path.
	mqttData, err := os.ReadFile(mqttSource)
	if err != nil {
		return generatedFiles{}, fmt.Errorf("read AsyncAPI schema %q: %w", mqttSource, err)
	}

	channels, err := parseChannels(string(mqttData))
	if err != nil {
		return generatedFiles{}, err
	}

	generatedMQTT, err := generateMQTT(channels)
	if err != nil {
		return generatedFiles{}, err
	}

	return generatedFiles{routes: routes, channels: channels, routesData: routesData, mqttData: generatedMQTT}, nil
}

func writeGeneratedFiles(files generatedFiles) error {
	// #nosec G306 -- generated Go source is intentionally readable by repository users.
	err := os.WriteFile(routeTarget, files.routesData, generatedSourcePermissions)
	if err != nil {
		return fmt.Errorf("write generated routes %q: %w", routeTarget, err)
	}

	// #nosec G306 -- generated Go source is intentionally readable by repository users.
	err = os.WriteFile(mqttTarget, files.mqttData, generatedSourcePermissions)
	if err != nil {
		return fmt.Errorf("write generated MQTT channels %q: %w", mqttTarget, err)
	}

	return nil
}

func checkGeneratedFiles(routes []route, channels []channel, target, mqttTarget string, generated, generatedMQTT []byte) error {
	err := checkSourceRoutes(routes)
	if err != nil {
		return err
	}

	err = checkSourceCallSites(routes, channels)
	if err != nil {
		return err
	}

	err = validateExternalMQTTContract()
	if err != nil {
		return err
	}

	// #nosec G304 -- target is the fixed generated route file path.
	current, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("read generated routes: %w", err)
	}

	if !bytes.Equal(current, generated) {
		return fmt.Errorf("%s %w; run go run ./tools/wireroutes", target, errGeneratedFileStale)
	}

	// #nosec G304 -- mqttTarget is the fixed generated MQTT file path.
	currentMQTT, err := os.ReadFile(mqttTarget)
	if err != nil {
		return fmt.Errorf("read generated MQTT channels: %w", err)
	}

	if !bytes.Equal(currentMQTT, generatedMQTT) {
		return fmt.Errorf("%s %w; run go run ./tools/wireroutes", mqttTarget, errGeneratedFileStale)
	}

	return nil
}

func checkSourceCallSites(routes []route, channels []channel) error {
	operations := make(map[string]bool, len(routes))
	for _, operation := range routes {
		operations["Operation"+strings.ToUpper(operation.id[:1])+operation.id[1:]] = true
	}

	knownChannels := make(map[string]bool, len(channels))
	for _, channel := range channels {
		knownChannels["MQTTChannel"+strings.ToUpper(channel.id[:1])+channel.id[1:]] = true
	}

	files, err := productionGoFiles()
	if err != nil {
		return fmt.Errorf("find production Go files: %w", err)
	}

	for _, filename := range files {
		// #nosec G304 -- filename is discovered beneath fixed production roots.
		content, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("read Go file %q: %w", filename, err)
		}

		validationErr := validateCallSites(filename, content, operations, knownChannels)
		if validationErr != nil {
			return fmt.Errorf("check wire call sites in %q: %w", filename, validationErr)
		}

		validationErr = validateSourceNetworkBoundary(filename, content)
		if validationErr != nil {
			return fmt.Errorf("check network boundary in %q: %w", filename, validationErr)
		}
	}

	return nil
}

func validateCallSites(filename string, source []byte, operations, channels map[string]bool) error {
	file, err := parser.ParseFile(token.NewFileSet(), filename, source, 0)
	if err != nil {
		return fmt.Errorf("parse Go file %q: %w", filename, err)
	}

	var findings []string

	modelAlias := importAlias(file, wireImport)

	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}

		functionAliasShadowed := modelAlias != "" && functionAliasShadowed(function, modelAlias)
		callSiteOptions := functionCallSiteOptions{
			operations:         operations,
			channels:           channels,
			modelAlias:         modelAlias,
			modelAliasShadowed: functionAliasShadowed,
		}
		functionMethods, functionRoutes, callFindings := inspectFunctionCallSites(function, callSiteOptions)
		findings = append(findings, callFindings...)

		for operation := range functionMethods {
			if !functionRoutes[operation] {
				findings = append(findings, "generated HTTP method has no matching route: "+operation)
			}
		}
	}

	if len(findings) > 0 {
		return fmt.Errorf("%w: %s: %s", errSourceCallSiteValidation, filename, strings.Join(findings, "; "))
	}

	return nil
}

type functionCallSiteOptions struct {
	operations         map[string]bool
	channels           map[string]bool
	modelAlias         string
	modelAliasShadowed bool
}

func inspectFunctionCallSites(function *ast.FuncDecl, options functionCallSiteOptions) (map[string]bool, map[string]bool, []string) {
	methods := make(map[string]bool)
	routes := make(map[string]bool)

	var findings []string

	ast.Inspect(function.Body, func(node ast.Node) bool {
		recordWireReference(node, methods, routes, options.modelAlias, options.modelAliasShadowed)

		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}

		finding := inspectCall(function.Name.Name, call, options.operations, options.channels, options.modelAlias, options.modelAliasShadowed)
		if finding != "" {
			findings = append(findings, finding)
		}

		return true
	})

	return methods, routes, findings
}

func recordWireReference(node ast.Node, methods, routes map[string]bool, modelAlias string, modelAliasShadowed bool) {
	selector, isSelector := node.(*ast.SelectorExpr)
	if !isSelector {
		return
	}

	name, isIdentifier := selector.X.(*ast.Ident)
	if !isIdentifier || modelAlias == "" || name.Name != modelAlias || modelAliasShadowed {
		return
	}

	if operation, ok := strings.CutPrefix(selector.Sel.Name, "Method"); ok {
		methods[operation] = true
	}

	if operation, ok := strings.CutPrefix(selector.Sel.Name, "Route"); ok {
		routes[operation] = true
	}
}

func inspectCall(functionName string, call *ast.CallExpr, operations, channels map[string]bool, modelAlias string, modelAliasShadowed bool) string {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return inspectChannelHelperCall(call, channels, modelAlias)
	}

	switch selector.Sel.Name {
	case "requestOperation":
		return inspectOperationCall(call, operations, modelAlias, modelAliasShadowed)
	case "Subscribe", "Unsubscribe":
		return inspectSubscriptionCall(functionName, selector.Sel.Name, call)
	case "newEncryptedRequest":
		if functionName != "makeRequest" {
			return "encrypted request builder must only be called by makeRequest"
		}
	}

	return ""
}

func inspectOperationCall(call *ast.CallExpr, operations map[string]bool, modelAlias string, modelAliasShadowed bool) string {
	if modelAliasShadowed || len(call.Args) < 2 || !isWireOperation(call.Args[1], operations, modelAlias) {
		return "encrypted operation must use a generated operation descriptor"
	}

	return ""
}

func inspectSubscriptionCall(functionName, methodName string, call *ast.CallExpr) string {
	helperName := strings.ToLower(methodName) + "Channel"
	if functionName != helperName || !hasGeneratedChannelArgument(call) {
		return "MQTT " + methodName + " must use " + helperName + " and a generated channel"
	}

	return ""
}

func inspectChannelHelperCall(call *ast.CallExpr, channels map[string]bool, modelAlias string) string {
	identifier, ok := call.Fun.(*ast.Ident)
	if !ok || (identifier.Name != "subscribeChannel" && identifier.Name != "unsubscribeChannel") {
		return ""
	}

	if len(call.Args) < 2 || !isWireSelector(call.Args[1], channels, modelAlias) {
		return "MQTT subscription lifecycle must use a schema channel"
	}

	return ""
}

func hasGeneratedChannelArgument(call *ast.CallExpr) bool {
	return len(call.Args) > 0 && exprString(call.Args[0]) == "channelAddress(channel, runtimeTopic)"
}

func isWireSelector(expression ast.Expr, known map[string]bool, modelAlias string) bool {
	selector, isSelector := expression.(*ast.SelectorExpr)
	if !isSelector {
		return false
	}

	identifier, isIdentifier := selector.X.(*ast.Ident)

	return isIdentifier && identifier.Name == modelAlias && modelAlias != "" && known[selector.Sel.Name]
}

func isWireOperation(expression ast.Expr, known map[string]bool, modelAlias string) bool {
	call, ok := expression.(*ast.CallExpr)

	return ok && len(call.Args) == 0 && isWireSelector(call.Fun, known, modelAlias)
}

func exprString(expression ast.Expr) string {
	var output bytes.Buffer

	_ = format.Node(&output, token.NewFileSet(), expression)

	return output.String()
}

type channel struct {
	id      string
	address string
}

var (
	errGeneratedFileStale       = errors.New("is stale")
	errSourceCallSiteValidation = errors.New("source call-site validation failed")
	errAsyncAPIChannelsMissing  = errors.New("no AsyncAPI channels found")
	errMQTTChannelInvalid       = errors.New("invalid or duplicate MQTT channel")
	errOpenAPIRoutesMissing     = errors.New("routes absent from api/openapi.yaml")
	errRoutePathOrIDMissing     = errors.New("operation without path or ID")
	errOpenAPIOperationsMissing = errors.New("no OpenAPI operations found")
	errDuplicateOperationID     = errors.New("duplicate operation ID")
)

func parseChannels(content string) ([]channel, error) {
	var parser channelParser

	for line := range strings.SplitSeq(content, "\n") {
		if parser.addLine(line) {
			break
		}
	}

	if len(parser.channels) == 0 {
		return nil, errAsyncAPIChannelsMissing
	}

	return parser.channels, nil
}

type channelParser struct {
	insideChannels bool
	channelID      string
	channels       []channel
}

func (parser *channelParser) addLine(line string) bool {
	if line == "channels:" {
		parser.insideChannels = true

		return false
	}

	if !parser.insideChannels {
		return false
	}

	if line != "" && line[0] != ' ' && line[0] != '#' {
		return true
	}

	if id, ok := parseChannelID(line); ok {
		parser.channelID = id

		return false
	}

	if address, ok := parseChannelAddress(line); parser.channelID != "" && ok {
		parser.channels = append(parser.channels, channel{id: parser.channelID, address: address})
		parser.channelID = ""
	}

	return false
}

func parseChannelID(line string) (string, bool) {
	if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") || !strings.HasSuffix(strings.TrimSpace(line), ":") {
		return "", false
	}

	return strings.TrimSuffix(strings.TrimSpace(line), ":"), true
}

func parseChannelAddress(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)

	if !strings.HasPrefix(line, "    address:") {
		return "", false
	}

	return strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "address:")), "\"'"), true
}

func generateMQTT(channels []channel) ([]byte, error) {
	sort.Slice(channels, func(i, j int) bool { return channels[i].id < channels[j].id })

	var out strings.Builder

	out.WriteString("// Code generated by tools/wireroutes from api/mqtt.asyncapi.yaml; DO NOT EDIT.\n")
	out.WriteString("package tuyamodels\n\n// MQTTChannel is a schema-defined subscription address template.\ntype MQTTChannel string\n\nconst (\n")

	seen := make(map[string]bool)

	for _, channel := range channels {
		name := "MQTTChannel" + strings.ToUpper(channel.id[:1]) + channel.id[1:]
		if seen[name] || channel.address == "" {
			return nil, fmt.Errorf("%w: %s", errMQTTChannelInvalid, channel.id)
		}

		seen[name] = true

		fmt.Fprintf(&out, "\t%s MQTTChannel = %q\n", name, channel.address)
	}

	out.WriteString(")\n")

	formatted, err := format.Source([]byte(out.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated route constants: %w", err)
	}

	return formatted, nil
}

func checkSourceRoutes(routes []route) error {
	known := make(map[string]bool)
	missing := make(map[string]bool)

	for _, route := range routes {
		known[normalizeRoute(route.path)] = true
	}

	files, err := productionGoFiles()
	if err != nil {
		return fmt.Errorf("find production Go files: %w", err)
	}

	for _, filename := range files {
		// #nosec G304 -- filename is discovered beneath fixed production roots.
		content, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("read Go file %q: %w", filename, err)
		}

		paths, err := missingPathLiterals(filename, content, known)
		if err != nil {
			return fmt.Errorf("scan Go file %q for route literals: %w", filename, err)
		}

		for _, path := range paths {
			missing[path] = true
		}
	}

	if len(missing) > 0 {
		var entries []string
		for entry := range missing {
			entries = append(entries, entry)
		}

		sort.Strings(entries)

		return fmt.Errorf("%w:\n  %s", errOpenAPIRoutesMissing, strings.Join(entries, "\n  "))
	}

	return nil
}

func missingPathLiterals(filename string, source []byte, known map[string]bool) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filename, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse Go file %q: %w", filename, err)
	}

	var missing []string

	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}

		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}

		start := regexp.MustCompile(`/v[0-9]+\.[0-9]+/`).FindStringIndex(value)
		if start == nil {
			return true
		}

		path, _, _ := strings.Cut(value[start[0]:], "?")
		if !known[normalizeRoute(path)] {
			missing = append(missing, fmt.Sprintf("%s: %s", filename, path))
		}

		return true
	})

	return missing, nil
}

func normalizeRoute(path string) string {
	path = parameter.ReplaceAllString(path, "{}")
	path = strings.ReplaceAll(path, "%s", "{}")
	path = strings.ReplaceAll(path, "%v", "{}")

	return path
}

type route struct {
	id     string
	method string
	path   string
}

func parseRoutes(content string) ([]route, error) {
	var parser routeParser

	for line := range strings.SplitSeq(content, "\n") {
		done, err := parser.addLine(line)
		if err != nil {
			return nil, err
		}

		if done {
			break
		}
	}

	if len(parser.routes) == 0 {
		return nil, errOpenAPIOperationsMissing
	}

	return parser.routes, nil
}

type routeParser struct {
	inPaths bool
	path    string
	method  string
	routes  []route
}

func (parser *routeParser) addLine(line string) (bool, error) {
	if line == "paths:" {
		parser.inPaths = true

		return false, nil
	}

	if !parser.inPaths {
		return false, nil
	}

	if line != "" && line[0] != ' ' && line[0] != '#' {
		return true, nil
	}

	return false, parser.addOperationLine(line)
}

func (parser *routeParser) addOperationLine(line string) error {
	if routePath, ok := parseRoutePath(line); ok {
		parser.path = routePath
		parser.method = ""

		return nil
	}

	if candidate, ok := parseRouteMethod(line); ok {
		parser.method = candidate

		return nil
	}

	if isRouteMethodIndent(line) {
		parser.method = ""

		return nil
	}

	return parser.addOperationID(line)
}

func (parser *routeParser) addOperationID(line string) error {
	if id, ok := parseRouteOperationID(line); parser.method != "" && ok {
		if id == "" || parser.path == "" {
			return errRoutePathOrIDMissing
		}

		parser.routes = append(parser.routes, route{id: id, method: parser.method, path: parser.path})
		parser.method = ""
	}

	return nil
}

func parseRoutePath(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(line, "  /") || !strings.HasSuffix(trimmed, ":") {
		return "", false
	}

	return strings.TrimSuffix(trimmed, ":"), true
}

func isRouteMethodIndent(line string) bool {
	return strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "     ") && strings.HasSuffix(strings.TrimSpace(line), ":")
}

func parseRouteMethod(line string) (string, bool) {
	if !isRouteMethodIndent(line) {
		return "", false
	}

	candidate := strings.TrimSuffix(strings.TrimSpace(line), ":")
	switch candidate {
	case "get", "post", "put", "patch", "delete", "head", "options":
		return strings.ToUpper(candidate), true
	default:
		return "", false
	}
}

func parseRouteOperationID(line string) (string, bool) {
	if !strings.HasPrefix(line, "      operationId:") {
		return "", false
	}

	trimmed := strings.TrimSpace(line)

	return strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "operationId:")), "\"'"), true
}

func generate(routes []route) ([]byte, error) {
	sort.Slice(routes, func(i, j int) bool { return routes[i].id < routes[j].id })

	var out strings.Builder

	out.WriteString("// Code generated by tools/wireroutes from api/openapi.yaml; DO NOT EDIT.\n")
	out.WriteString(
		"package tuyamodels\n\n" +
			"import \"strings\"\n\n" +
			"// Operation binds an HTTP method to its schema path.\n" +
			"type Operation struct {\n\tMethod string\n\tPath string\n}\n\n" +
			"const (\n",
	)

	err := writeRouteConstants(&out, routes)
	if err != nil {
		return nil, err
	}

	writeOperationAccessors(&out, routes)
	writeKnownOperations(&out, routes)

	writeOperationMatchers(&out)

	formatted, err := format.Source([]byte(out.String()))
	if err != nil {
		return nil, fmt.Errorf("format generated MQTT channels: %w", err)
	}

	return formatted, nil
}

func writeOperationMatchers(out *strings.Builder) {
	out.WriteString(`}

// IsKnownOperation accepts only a method and path defined in the OpenAPI inventory.
func IsKnownOperation(method, path string) bool {
	if strings.ContainsAny(path, "?#") {
		return false
	}
	for _, operation := range knownOperations {
		if method == operation.Method && matchesOperationPath(operation.Path, path) {
			return true
		}
	}
	return false
}

func matchesOperationPath(template, path string) bool {
	templateParts := strings.Split(template, "/")
	pathParts := strings.Split(path, "/")
	if len(templateParts) != len(pathParts) {
		return false
	}
	for index, part := range templateParts {
		if part == "%s" {
			if pathParts[index] == "" {
				return false
			}
			continue
		}
		if part != pathParts[index] {
			return false
		}
	}
	return true
}
`)
}

func writeRouteConstants(out *strings.Builder, routes []route) error {
	seen := make(map[string]bool)

	for _, route := range routes {
		name := "Route" + strings.ToUpper(route.id[:1]) + route.id[1:]
		if seen[name] {
			return fmt.Errorf("%w: %s", errDuplicateOperationID, route.id)
		}

		seen[name] = true
		routeFormat := parameter.ReplaceAllString(route.path, "%s")
		fmt.Fprintf(out, "\t%s = %q\n", name, routeFormat)
		fmt.Fprintf(out, "\tMethod%s = %q\n", strings.TrimPrefix(name, "Route"), route.method)
	}

	out.WriteString(")\n\n")

	return nil
}

func writeOperationAccessors(out *strings.Builder, routes []route) {
	for _, route := range routes {
		name := strings.ToUpper(route.id[:1]) + route.id[1:]
		fmt.Fprintf(out, "func Operation%s() Operation { return Operation{Method: Method%s, Path: Route%s} }\n", name, name, name)
	}
}

func writeKnownOperations(out *strings.Builder, routes []route) {
	out.WriteString("\nvar knownOperations = []Operation{\n")

	for _, route := range routes {
		name := strings.ToUpper(route.id[:1]) + route.id[1:]
		fmt.Fprintf(out, "\tOperation%s(),\n", name)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
