// Command wireroutes generates Go route formats from the checked-in OpenAPI paths.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var parameter = regexp.MustCompile(`\{[^{}]+\}`)

func main() {
	check := flag.Bool("check", false, "fail when generated routes differ")
	flag.Parse()
	const source = "api/openapi.yaml"
	const target = "pkg/tuya/internal/wire/routes.gen.go"
	const mqttSource = "api/mqtt.asyncapi.yaml"
	const mqttTarget = "pkg/tuya/internal/wire/mqtt.gen.go"
	data, err := os.ReadFile(source)
	if err != nil {
		fatal(err)
	}
	routes, err := parseRoutes(string(data))
	if err != nil {
		fatal(err)
	}
	generated, err := generate(routes)
	if err != nil {
		fatal(err)
	}
	mqttData, err := os.ReadFile(mqttSource)
	if err != nil {
		fatal(err)
	}
	channels, err := parseChannels(string(mqttData))
	if err != nil {
		fatal(err)
	}
	generatedMQTT, err := generateMQTT(channels)
	if err != nil {
		fatal(err)
	}
	if *check {
		if err := checkSourceRoutes(routes); err != nil {
			fatal(err)
		}
		if err := checkSourceCallSites(routes, channels); err != nil {
			fatal(err)
		}
		current, err := os.ReadFile(target)
		if err != nil {
			fatal(err)
		}
		if !bytes.Equal(current, generated) {
			fatal(fmt.Errorf("%s is stale; run go run ./tools/wireroutes", target))
		}
		currentMQTT, err := os.ReadFile(mqttTarget)
		if err != nil {
			fatal(err)
		}
		if !bytes.Equal(currentMQTT, generatedMQTT) {
			fatal(fmt.Errorf("%s is stale; run go run ./tools/wireroutes", mqttTarget))
		}
		return
	}
	if err := os.WriteFile(target, generated, 0644); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(mqttTarget, generatedMQTT, 0644); err != nil {
		fatal(err)
	}
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
	files, err := filepath.Glob("pkg/tuya/*.go")
	if err != nil {
		return err
	}
	for _, filename := range files {
		if strings.HasSuffix(filename, "_test.go") {
			continue
		}
		content, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		if err := validateCallSites(filename, content, operations, knownChannels); err != nil {
			return err
		}
	}
	return nil
}

func validateCallSites(filename string, source []byte, operations, channels map[string]bool) error {
	file, err := parser.ParseFile(token.NewFileSet(), filename, source, 0)
	if err != nil {
		return err
	}
	var findings []string
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		functionMethods := make(map[string]bool)
		functionRoutes := make(map[string]bool)
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok {
				if name, ok := selector.X.(*ast.Ident); ok && name.Name == "wire" {
					if strings.HasPrefix(selector.Sel.Name, "Method") {
						functionMethods[strings.TrimPrefix(selector.Sel.Name, "Method")] = true
					}
					if strings.HasPrefix(selector.Sel.Name, "Route") {
						functionRoutes[strings.TrimPrefix(selector.Sel.Name, "Route")] = true
					}
				}
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				if identifier, ok := call.Fun.(*ast.Ident); ok && (identifier.Name == "subscribeChannel" || identifier.Name == "unsubscribeChannel") {
					if len(call.Args) < 2 || !isWireSelector(call.Args[1], channels) {
						findings = append(findings, "MQTT subscription lifecycle must use a schema channel")
					}
				}
				return true
			}
			switch selector.Sel.Name {
			case "requestOperation":
				if len(call.Args) < 2 || !isWireOperation(call.Args[1], operations) {
					findings = append(findings, "encrypted operation must use a generated operation descriptor")
				}
			case "Get", "Post", "Put", "Delete":
				if strings.Contains(exprString(selector.X), "EncryptedClient") || strings.HasSuffix(exprString(selector.X), ".client") {
					findings = append(findings, "internal encrypted calls must use generated operation descriptors")
				}
			case "Subscribe":
				if function.Name.Name != "subscribeChannel" || len(call.Args) == 0 || exprString(call.Args[0]) != "channelAddress(channel, runtimeTopic)" {
					findings = append(findings, "MQTT Subscribe must use subscribeChannel and a generated channel")
				}
			case "Unsubscribe":
				if function.Name.Name != "unsubscribeChannel" || len(call.Args) == 0 || exprString(call.Args[0]) != "channelAddress(channel, runtimeTopic)" {
					findings = append(findings, "MQTT Unsubscribe must use unsubscribeChannel and a generated channel")
				}
			case "NewRequestWithContext":
				if function.Name.Name != "makeRequest" && (len(call.Args) < 2 || !strings.HasPrefix(exprString(call.Args[1]), "wire.Method")) {
					findings = append(findings, "direct HTTP request must use a generated operation method")
				}
			}
			return true
		})
		for operation := range functionMethods {
			if !functionRoutes[operation] {
				findings = append(findings, "generated HTTP method has no matching route: "+operation)
			}
		}
	}
	if len(findings) > 0 {
		return fmt.Errorf("%s: %s", filename, strings.Join(findings, "; "))
	}
	return nil
}

func isWireSelector(expression ast.Expr, known map[string]bool) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	return ok && identifier.Name == "wire" && known[selector.Sel.Name]
}

func isWireOperation(expression ast.Expr, known map[string]bool) bool {
	call, ok := expression.(*ast.CallExpr)
	return ok && len(call.Args) == 0 && isWireSelector(call.Fun, known)
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

func parseChannels(content string) ([]channel, error) {
	var channels []channel
	inChannels := false
	id := ""
	for _, line := range strings.Split(content, "\n") {
		if line == "channels:" {
			inChannels = true
			continue
		}
		if !inChannels {
			continue
		}
		if line != "" && line[0] != ' ' && line[0] != '#' {
			break
		}
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			id = strings.TrimSuffix(strings.TrimSpace(line), ":")
			continue
		}
		if id != "" && strings.HasPrefix(line, "    address:") {
			address := strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "address:")), "\"'")
			channels = append(channels, channel{id: id, address: address})
			id = ""
		}
	}
	if len(channels) == 0 {
		return nil, fmt.Errorf("no AsyncAPI channels found")
	}
	return channels, nil
}

func generateMQTT(channels []channel) ([]byte, error) {
	sort.Slice(channels, func(i, j int) bool { return channels[i].id < channels[j].id })
	var out strings.Builder
	out.WriteString("// Code generated by tools/wireroutes from api/mqtt.asyncapi.yaml; DO NOT EDIT.\n")
	out.WriteString("package wire\n\n// MQTTChannel is a schema-defined subscription address template.\ntype MQTTChannel string\n\nconst (\n")
	seen := make(map[string]bool)
	for _, channel := range channels {
		name := "MQTTChannel" + strings.ToUpper(channel.id[:1]) + channel.id[1:]
		if seen[name] || channel.address == "" {
			return nil, fmt.Errorf("invalid or duplicate MQTT channel: %s", channel.id)
		}
		seen[name] = true
		fmt.Fprintf(&out, "\t%s MQTTChannel = %q\n", name, channel.address)
	}
	out.WriteString(")\n")
	return format.Source([]byte(out.String()))
}

func checkSourceRoutes(routes []route) error {
	known := make(map[string]bool)
	missing := make(map[string]bool)
	for _, route := range routes {
		known[normalizeRoute(route.path)] = true
	}
	files, err := filepath.Glob("pkg/tuya/*.go")
	if err != nil {
		return err
	}
	for _, filename := range files {
		if strings.HasSuffix(filename, "_test.go") {
			continue
		}
		content, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		paths, err := missingPathLiterals(filename, content, known)
		if err != nil {
			return err
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
		return fmt.Errorf("routes absent from api/openapi.yaml:\n  %s", strings.Join(entries, "\n  "))
	}
	return nil
}

func missingPathLiterals(filename string, source []byte, known map[string]bool) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filename, source, 0)
	if err != nil {
		return nil, err
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
		path := strings.SplitN(value[start[0]:], "?", 2)[0]
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
	var routes []route
	var path, method string
	inPaths := false
	for _, line := range strings.Split(content, "\n") {
		if line == "paths:" {
			inPaths = true
			continue
		}
		if !inPaths {
			continue
		}
		if line != "" && line[0] != ' ' && line[0] != '#' {
			break
		}
		if strings.HasPrefix(line, "  /") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			path = strings.TrimSuffix(strings.TrimSpace(line), ":")
			method = ""
			continue
		}
		if strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "     ") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			candidate := strings.TrimSuffix(strings.TrimSpace(line), ":")
			switch candidate {
			case "get", "post", "put", "patch", "delete", "head", "options":
				method = strings.ToUpper(candidate)
			default:
				method = ""
			}
			continue
		}
		if method != "" && strings.HasPrefix(line, "      operationId:") {
			id := strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "operationId:")), "\"'")
			if id == "" || path == "" {
				return nil, fmt.Errorf("operation without path or ID")
			}
			routes = append(routes, route{id: id, method: method, path: path})
			method = ""
		}
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("no OpenAPI operations found")
	}
	return routes, nil
}

func generate(routes []route) ([]byte, error) {
	sort.Slice(routes, func(i, j int) bool { return routes[i].id < routes[j].id })
	var out strings.Builder
	out.WriteString("// Code generated by tools/wireroutes from api/openapi.yaml; DO NOT EDIT.\n")
	out.WriteString("package wire\n\nimport \"strings\"\n\n// Operation binds an HTTP method to its schema path.\ntype Operation struct {\n\tMethod string\n\tPath string\n}\n\nconst (\n")
	seen := make(map[string]bool)
	for _, route := range routes {
		name := "Route" + strings.ToUpper(route.id[:1]) + route.id[1:]
		if seen[name] {
			return nil, fmt.Errorf("duplicate operation ID: %s", route.id)
		}
		seen[name] = true
		format := parameter.ReplaceAllString(route.path, "%s")
		fmt.Fprintf(&out, "\t%s = %q\n", name, format)
		fmt.Fprintf(&out, "\tMethod%s = %q\n", strings.TrimPrefix(name, "Route"), route.method)
	}
	out.WriteString(")\n\n")
	for _, route := range routes {
		name := strings.ToUpper(route.id[:1]) + route.id[1:]
		fmt.Fprintf(&out, "func Operation%s() Operation { return Operation{Method: Method%s, Path: Route%s} }\n", name, name, name)
	}
	out.WriteString("\nvar knownOperations = []Operation{\n")
	for _, route := range routes {
		name := strings.ToUpper(route.id[:1]) + route.id[1:]
		fmt.Fprintf(&out, "\tOperation%s(),\n", name)
	}
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
	return format.Source([]byte(out.String()))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
