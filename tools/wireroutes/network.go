package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var errNetworkBoundary = errors.New("untracked outbound network edge")

func productionGoFiles() ([]string, error) {
	var files []string

	for _, root := range productionSourceRoots() {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			if entry.IsDir() {
				if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor") {
					return filepath.SkipDir
				}

				return nil
			}

			if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			files = append(files, filepath.Clean(path))

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", root, err)
		}
	}

	sort.Strings(files)

	return files, nil
}

func productionSourceRoots() []string {
	return []string{"pkg", "cmd", "examples"}
}

func importAlias(file *ast.File, importPath string) string {
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)

		if err != nil || path != importPath {
			continue
		}

		if spec.Name != nil {
			return spec.Name.Name
		}

		return filepath.Base(importPath)
	}

	return ""
}

func functionAliasShadowed(function *ast.FuncDecl, alias string) bool {
	if alias == "" || function.Body == nil {
		return false
	}

	if fieldsContainName(function.Recv, alias) || fieldsContainName(function.Type.Params, alias) || fieldsContainName(function.Type.Results, alias) {
		return true
	}

	return functionBodyBindsName(function, alias)
}

func functionBodyBindsName(function *ast.FuncDecl, name string) bool {
	bound := false

	ast.Inspect(function.Body, func(node ast.Node) bool {
		if bound {
			return false
		}

		bound = nodeBindsName(node, name)

		return true
	})

	return bound
}

func nodeBindsName(node ast.Node, name string) bool {
	switch value := node.(type) {
	case *ast.AssignStmt:
		if value.Tok == token.DEFINE && expressionsContainName(value.Lhs, name) {
			return true
		}
	case *ast.ValueSpec:
		return valueSpecBindsName(value, name)
	case *ast.RangeStmt:
		return rangeBindsName(value, name)
	case *ast.FuncLit:
		if fieldsContainName(value.Type.Params, name) || fieldsContainName(value.Type.Results, name) {
			return true
		}
	}

	return false
}

func valueSpecBindsName(spec *ast.ValueSpec, name string) bool {
	for _, identifier := range spec.Names {
		if identifier.Name == name {
			return true
		}
	}

	return false
}

func rangeBindsName(statement *ast.RangeStmt, name string) bool {
	return statement.Tok == token.DEFINE &&
		(expressionContainsName(statement.Key, name) || expressionContainsName(statement.Value, name))
}

func fieldsContainName(fields *ast.FieldList, name string) bool {
	if fields == nil {
		return false
	}

	for _, field := range fields.List {
		for _, identifier := range field.Names {
			if identifier.Name == name {
				return true
			}
		}
	}

	return false
}

func expressionsContainName(expressions []ast.Expr, name string) bool {
	for _, expression := range expressions {
		if expressionContainsName(expression, name) {
			return true
		}
	}

	return false
}

func expressionContainsName(expression ast.Expr, name string) bool {
	identifier, matchedType := expression.(*ast.Ident)

	return matchedType && identifier.Name == name
}

func validateSourceNetworkBoundary(filename string, source []byte) error {
	file, err := parser.ParseFile(token.NewFileSet(), filename, source, 0)
	if err != nil {
		return fmt.Errorf("parse source: %w", err)
	}

	imports, err := networkImports(file, filename)
	if err != nil {
		return err
	}

	if filepath.Clean(filename) == filepath.Clean(httpBoundary) {
		return validateHTTPBoundary(file, imports)
	}

	if filepath.Clean(filename) == filepath.Clean(mqttBoundary) {
		return validateMQTTBoundary(file, imports)
	}

	findings := directHTTPSelectors(file, imports)

	if len(findings) > 0 {
		sort.Strings(findings)

		return fmt.Errorf("direct HTTP calls %s must use %s: %w", strings.Join(findings, ", "), httpBoundary, errNetworkBoundary)
	}

	return validateDirectSocketSelectors(file, imports)
}

func networkImports(file *ast.File, filename string) (map[string]string, error) {
	imports := make(map[string]string, len(file.Imports))

	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("read import path: %w", err)
		}

		name := filepath.Base(path)

		if spec.Name != nil {
			name = spec.Name.Name
		}

		imports[name] = path

		if path == pahoImport && filepath.Clean(filename) != filepath.Clean(mqttBoundary) {
			return nil, fmt.Errorf("paho import belongs only in %s: %w", mqttBoundary, errNetworkBoundary)
		}

		if isExternalNetworkPackage(path) && filepath.Clean(filename) != filepath.Clean(mqttBoundary) {
			return nil, fmt.Errorf("network dependency %q is not behind its inventoried boundary: %w", path, errNetworkBoundary)
		}
	}

	return imports, nil
}

func directHTTPSelectors(file *ast.File, imports map[string]string) []string {
	var findings []string

	for alias, path := range imports {
		if path != "net/http" {
			continue
		}

		for _, declaration := range file.Decls {
			ast.Inspect(declaration, func(node ast.Node) bool {
				selector, matchedType := node.(*ast.SelectorExpr)

				if matchedType && directHTTPSelector(file, declaration, selector, alias, imports) {
					findings = append(findings, selector.Sel.Name)
				}

				return true
			})
		}
	}

	return findings
}

func directHTTPSelector(file *ast.File, declaration ast.Decl, selector *ast.SelectorExpr, alias string, imports map[string]string) bool {
	name := selector.Sel.Name

	if name == "Do" || name == "RoundTrip" {
		if base, identifier := selector.X.(*ast.Ident); identifier {
			if importedPath, importedPackage := imports[base.Name]; importedPackage {
				return importedPath == "net/http"
			}
		}

		return true
	}

	if !isHTTPPackageCall(name) {
		return false
	}

	if isIdentifier(selector.X, alias) {
		return true
	}

	return isHTTPClientMethod(name) && isHTTPClientReceiver(file, declaration, selector.X, alias)
}

func validateDirectSocketSelectors(file *ast.File, imports map[string]string) error {
	for alias, path := range imports {
		kind, methods := socketNetworkMethods(path)

		selectors := networkSelectorUses(file, alias, methods)

		if len(selectors) > 0 {
			return fmt.Errorf("direct %s calls %s need an injectable dependency boundary: %w", kind, strings.Join(selectors, ", "), errNetworkBoundary)
		}
	}

	selectors := networkDialSelectorUses(file)
	if len(selectors) > 0 {
		return fmt.Errorf("direct or captured socket methods %s need an injectable dependency boundary: %w", strings.Join(selectors, ", "), errNetworkBoundary)
	}

	return nil
}

func socketNetworkMethods(path string) (string, map[string]bool) {
	switch path {
	case "net":
		return "socket", map[string]bool{networkDialMethod: true, "DialTimeout": true, networkDialContextMethod: true, "Listen": true, "ListenPacket": true}
	case "crypto/tls":
		return "TLS socket", map[string]bool{networkDialMethod: true, "DialWithDialer": true}
	case "golang.org/x/net/websocket", "github.com/gorilla/websocket", "nhooyr.io/websocket", "github.com/coder/websocket", "github.com/gobwas/ws":
		return "WebSocket", map[string]bool{networkDialMethod: true, networkDialContextMethod: true, "NewClient": true}
	default:
		return "", nil
	}
}

func isHTTPClientMethod(name string) bool {
	switch name {
	case "Get", "Post", "PostForm", "Head":
		return true
	default:
		return false
	}
}

func isHTTPClientReceiver(file *ast.File, declaration ast.Decl, expression ast.Expr, alias string) bool {
	known := make(map[string]bool)

	for _, topLevel := range file.Decls {
		collectGlobalHTTPClients(topLevel, alias, known)
	}

	function, matchedType := declaration.(*ast.FuncDecl)

	if matchedType {
		collectHTTPClientParameters(function.Type.Params, alias, known)

		if function.Body != nil {
			ast.Inspect(function.Body, func(node ast.Node) bool {
				collectLocalHTTPClients(node, alias, known)

				return true
			})
		}
	}

	return isHTTPClientExpression(expression, alias, known)
}

func collectGlobalHTTPClients(declaration ast.Decl, alias string, known map[string]bool) {
	value, matchedType := declaration.(*ast.GenDecl)

	if !matchedType || value.Tok != token.VAR {
		return
	}

	for _, spec := range value.Specs {
		variable, matchedType := spec.(*ast.ValueSpec)

		if matchedType {
			collectHTTPClientValues(variable, alias, known)
		}
	}
}

func collectHTTPClientValues(variable *ast.ValueSpec, alias string, known map[string]bool) {
	for index, name := range variable.Names {
		if isHTTPClientType(variable.Type, alias) ||
			(index < len(variable.Values) && isHTTPClientExpression(variable.Values[index], alias, known)) {
			known[name.Name] = true
		}
	}
}

func collectHTTPClientParameters(parameters *ast.FieldList, alias string, known map[string]bool) {
	if parameters == nil {
		return
	}

	for _, field := range parameters.List {
		if !isHTTPClientType(field.Type, alias) {
			continue
		}

		for _, name := range field.Names {
			known[name.Name] = true
		}
	}
}

func collectLocalHTTPClients(node ast.Node, alias string, known map[string]bool) {
	switch value := node.(type) {
	case *ast.ValueSpec:
		collectHTTPClientValues(value, alias, known)
	case *ast.AssignStmt:
		for index, left := range value.Lhs {
			name, isName := left.(*ast.Ident)

			if isName && index < len(value.Rhs) && isHTTPClientExpression(value.Rhs[index], alias, known) {
				known[name.Name] = true
			}
		}
	}
}

func isHTTPClientType(expression ast.Expr, alias string) bool {
	switch value := expression.(type) {
	case *ast.StarExpr:
		return isHTTPClientType(value.X, alias)
	case *ast.ParenExpr:
		return isHTTPClientType(value.X, alias)
	case *ast.SelectorExpr:
		return value.Sel.Name == "Client" && isIdentifier(value.X, alias)
	default:
		return false
	}
}

func isHTTPClientExpression(expression ast.Expr, alias string, known map[string]bool) bool {
	switch value := expression.(type) {
	case *ast.Ident:
		return known[value.Name]
	case *ast.SelectorExpr:
		return isHTTPClientSelector(value, alias)
	case *ast.StarExpr:
		return isHTTPClientType(value, alias)
	case *ast.ParenExpr:
		return isHTTPClientExpression(value.X, alias, known)
	case *ast.UnaryExpr:
		return value.Op == token.AND && isHTTPClientExpression(value.X, alias, known)
	case *ast.CompositeLit:
		return isHTTPClientType(value.Type, alias)
	default:
		return false
	}
}

func isHTTPClientSelector(selector *ast.SelectorExpr, alias string) bool {
	if isIdentifier(selector.X, alias) && selector.Sel.Name == "DefaultClient" {
		return true
	}

	if isHTTPClientType(selector, alias) {
		return true
	}

	return selector.Sel.Name == "HTTPClient" || selector.Sel.Name == "httpClient"
}

func validateHTTPBoundary(file *ast.File, imports map[string]string) error {
	httpAlias := aliasForPath(imports, "net/http")

	wireAlias := aliasForPath(imports, wireImport)

	requestFunction := uniqueFunction(file, "Do")

	err := validateHTTPDoDefinition(requestFunction, httpAlias, wireAlias)
	if err != nil {
		return err
	}

	if !validHTTPBodyFactory(file, imports) {
		return fmt.Errorf("HTTP request body factory must return a stable reader over the supplied bytes: %w", errNetworkBoundary)
	}

	pathCalls := assignedCalls(requestFunction, "formatOperationPath", "")

	keyCalls := assignedCalls(requestFunction, "validateRequestKeys", "")

	requestCalls := assignedCalls(requestFunction, "NewRequestWithContext", httpAlias)

	sendCalls := assignedCalls(requestFunction, "Do", "client")

	operationGuards := generatedOperationGuards(requestFunction, wireAlias)

	urlCalls := assignedCalls(requestFunction, "requestURL", "")

	if !uniqueHTTPCallCounts(len(pathCalls), len(operationGuards), len(keyCalls), len(urlCalls), len(requestCalls), len(sendCalls)) {
		return fmt.Errorf("HTTP Do must bind route, operation, keys, URL, request, and send in one function: %w", errNetworkBoundary)
	}

	pathCall := pathCalls[0]

	operationGuard := operationGuards[0]

	keyCall := keyCalls[0]

	urlCall := urlCalls[0]

	requestCall := requestCalls[0]

	sendCall := sendCalls[0]

	if !orderedHTTPCallIndices(pathCall.index, operationGuard.index, keyCall.index, urlCall.index, requestCall.index, sendCall.index) {
		return fmt.Errorf("HTTP validation and request construction must execute in route-to-send order: %w", errNetworkBoundary)
	}

	err = validateHTTPRequestBindings(
		requestFunction, pathCall, operationGuard, keyCall, urlCall, requestCall, sendCall, imports, httpAlias,
	)
	if err != nil {
		return err
	}

	return validateHTTPBoundarySend(file, imports, requestFunction, requestCall, sendCall, httpAlias, wireAlias)
}

func validateHTTPRequestBindings(
	function *ast.FuncDecl, pathCall indexedCall, operationGuard indexedGuard, keyCall, urlCall, requestCall, sendCall indexedCall,
	imports map[string]string, httpAlias string,
) error {
	if !validHTTPRouteBindings(pathCall, operationGuard, keyCall) {
		return fmt.Errorf("HTTP operation validation must bind the schema method, final route, and request keys: %w", errNetworkBoundary)
	}

	if !validatedHTTPInputsAreUnmodified(function, pathCall, operationGuard, urlCall, requestCall, imports) {
		return fmt.Errorf("HTTP schema-validated method, route, and URL authority must remain unchanged through request construction: %w", errNetworkBoundary)
	}

	if !validHTTPURLBindings(function, keyCall, urlCall) {
		return fmt.Errorf("HTTP key validation and URL construction must bind the validated request inputs: %w", errNetworkBoundary)
	}

	if !validHTTPRequestBindings(function, requestCall, sendCall, httpAlias) {
		return fmt.Errorf("HTTP request must use the generated method and validated URL before send: %w", errNetworkBoundary)
	}

	if !httpRequestIsUnmodified(function, requestCall, sendCall) {
		return fmt.Errorf("HTTP request and backing body bytes must remain unchanged before injected send: %w", errNetworkBoundary)
	}

	return nil
}

func validateHTTPBoundarySend(
	file *ast.File, imports map[string]string, requestFunction *ast.FuncDecl,
	requestCall, sendCall indexedCall, httpAlias, wireAlias string,
) error {
	if !validHTTPSendBindings(file, imports, requestFunction, requestCall, sendCall, wireAlias) {
		return fmt.Errorf("HTTP send must use the injected client after generated query/header-key guards: %w", errNetworkBoundary)
	}

	if !onlyExpectedHTTPCalls(file, httpAlias, requestCall.call, sendCall.call) {
		return fmt.Errorf("http transport contains an unregistered HTTP network call: %w", errNetworkBoundary)
	}

	return nil
}

func validateHTTPDoDefinition(requestFunction *ast.FuncDecl, httpAlias, wireAlias string) error {
	if httpAlias == "" || wireAlias == "" {
		return fmt.Errorf("HTTP boundary must import net/http and generated models: %w", errNetworkBoundary)
	}

	if requestFunction == nil || functionAliasShadowed(requestFunction, httpAlias) || functionAliasShadowed(requestFunction, wireAlias) ||
		functionBodyBindsName(requestFunction, "client") {
		return fmt.Errorf("HTTP boundary must use unshadowed imports and injected client in Do: %w", errNetworkBoundary)
	}

	if !hasNamedParameter(requestFunction, "client", isHTTPClientTypeAlias(httpAlias)) ||
		!hasNamedParameter(requestFunction, "operation", selectorTypeMatcher(wireAlias, "Operation")) {
		return fmt.Errorf("HTTP Do must receive an injected client and generated operation: %w", errNetworkBoundary)
	}

	return nil
}

func uniqueHTTPCallCounts(counts ...int) bool {
	for _, count := range counts {
		if count != 1 {
			return false
		}
	}

	return true
}

func orderedHTTPCallIndices(indices ...int) bool {
	for index := 1; index < len(indices); index++ {
		if indices[index-1] >= indices[index] {
			return false
		}
	}

	return true
}

func validHTTPRouteBindings(pathCall indexedCall, operationGuard indexedGuard, keyCall indexedCall) bool {
	return assignedTo(pathCall, "path") && pathFormattingUsesSchemaRoute(pathCall.call) &&
		directReturnOnly(operationGuard.statement.Body) && operationGuardUsesFinalRoute(operationGuard.call) &&
		assignedTo(keyCall, "err") && keyValidationUsesRequestInputs(keyCall.call)
}

func validatedHTTPInputsAreUnmodified(
	function *ast.FuncDecl, pathCall indexedCall, operationGuard indexedGuard, urlCall, requestCall indexedCall,
	imports map[string]string,
) bool {
	if function == nil {
		return false
	}

	guardPathUses, guardOperationUses := safeHTTPGuardDiagnosticUses(operationGuard.statement.Body, aliasForPath(imports, "fmt"))

	return validatedHTTPPathIsUnmodified(function, pathCall, operationGuard, urlCall, guardPathUses) &&
		validatedHTTPMethodIsUnmodified(function, pathCall, operationGuard, requestCall, guardOperationUses) &&
		validatedHTTPURLIsUnmodified(function, urlCall, requestCall)
}

func validatedHTTPPathIsUnmodified(
	function *ast.FuncDecl, pathCall indexedCall, operationGuard indexedGuard, urlCall indexedCall, guardUses map[token.Pos]bool,
) bool {
	if functionSignatureBindsName(function, "path") || !validatedHTTPPathCallsAreBound(operationGuard, urlCall) {
		return false
	}

	assignment, matchedAssignment := pathCall.statement.(*ast.AssignStmt)
	if !matchedAssignment || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 2 ||
		!isIdentifier(assignment.Lhs[0], "path") || !isIdentifier(assignment.Lhs[1], "err") {
		return false
	}

	allowedPathUses := map[token.Pos]bool{
		assignment.Lhs[0].Pos():           true,
		operationGuard.call.Args[1].Pos(): true,
		urlCall.call.Args[1].Pos():        true,
	}

	for position := range guardUses {
		allowedPathUses[position] = true
	}

	return namedValueUsesUnchanged(function.Body, "path", function.Body.Pos(), urlCall.call.End(), allowedPathUses)
}

func validatedHTTPPathCallsAreBound(operationGuard indexedGuard, urlCall indexedCall) bool {
	return len(operationGuard.call.Args) == 2 && len(urlCall.call.Args) == 3 &&
		isIdentifier(operationGuard.call.Args[1], "path") && isIdentifier(urlCall.call.Args[1], "path") &&
		isSelectorOn(operationGuard.call.Args[0], "operation", "Method")
}

func validatedHTTPMethodIsUnmodified(
	function *ast.FuncDecl, pathCall indexedCall, operationGuard indexedGuard, requestCall indexedCall,
	guardUses map[token.Pos]bool,
) bool {
	if len(pathCall.call.Args) != 2 || len(operationGuard.call.Args) != 2 || len(requestCall.call.Args) < 2 ||
		!isSelectorOn(requestCall.call.Args[1], "operation", "Method") {
		return false
	}

	pathOperation, isSelector := pathCall.call.Args[0].(*ast.SelectorExpr)
	if !isSelector {
		return false
	}

	guardMethod, isSelector := operationGuard.call.Args[0].(*ast.SelectorExpr)
	if !isSelector {
		return false
	}

	requestMethod, isSelector := requestCall.call.Args[1].(*ast.SelectorExpr)
	if !isSelector {
		return false
	}

	allowedOperationUses := map[token.Pos]bool{
		pathOperation.X.Pos(): true,
		guardMethod.X.Pos():   true,
		requestMethod.X.Pos(): true,
	}

	for position := range guardUses {
		allowedOperationUses[position] = true
	}

	return namedValueUsesUnchanged(function.Body, "operation", function.Body.Pos(), function.Body.End(), allowedOperationUses)
}

func validatedHTTPURLIsUnmodified(function *ast.FuncDecl, urlCall, requestCall indexedCall) bool {
	if function == nil || urlCall.call == nil || requestCall.call == nil ||
		functionSignatureBindsName(function, "target") {
		return false
	}

	target := requestURLStringReceiver(requestCall.call)
	if target == nil {
		return false
	}

	return namedValueUsesUnchanged(
		function.Body,
		"target",
		urlCall.call.End(),
		function.Body.End(),
		map[token.Pos]bool{target.Pos(): true},
	)
}

func requestURLStringReceiver(call *ast.CallExpr) *ast.Ident {
	if call == nil || len(call.Args) < 3 {
		return nil
	}

	stringCall, isCall := call.Args[2].(*ast.CallExpr)
	if !isCall || len(stringCall.Args) != 0 {
		return nil
	}

	stringMethod, isSelector := stringCall.Fun.(*ast.SelectorExpr)
	if !isSelector || stringMethod.Sel.Name != "String" || !isIdentifier(stringMethod.X, "target") {
		return nil
	}

	target, _ := stringMethod.X.(*ast.Ident)

	return target
}

func directReturnOnly(block *ast.BlockStmt) bool {
	if block == nil || len(block.List) != 1 {
		return false
	}

	_, isReturn := block.List[0].(*ast.ReturnStmt)

	return isReturn
}

func safeHTTPGuardDiagnosticUses(block *ast.BlockStmt, fmtAlias string) (map[token.Pos]bool, map[token.Pos]bool) {
	pathUses := make(map[token.Pos]bool)
	operationUses := make(map[token.Pos]bool)

	statement := httpGuardReturn(block)
	if statement == nil {
		return pathUses, operationUses
	}

	diagnostic := httpGuardErrorf(statement, fmtAlias)
	if diagnostic == nil {
		return pathUses, operationUses
	}

	for _, argument := range diagnostic.Args {
		switch value := argument.(type) {
		case *ast.Ident:
			if value.Name == "path" {
				pathUses[value.Pos()] = true
			}
		case *ast.SelectorExpr:
			if value.Sel.Name == "Method" && isIdentifier(value.X, "operation") {
				operationUses[value.X.Pos()] = true
			}
		}
	}

	return pathUses, operationUses
}

func httpGuardReturn(block *ast.BlockStmt) *ast.ReturnStmt {
	if !directReturnOnly(block) {
		return nil
	}

	statement, isReturn := block.List[0].(*ast.ReturnStmt)
	if !isReturn || len(statement.Results) != httpGuardReturnValues {
		return nil
	}

	return statement
}

func httpGuardErrorf(statement *ast.ReturnStmt, fmtAlias string) *ast.CallExpr {
	if statement == nil || fmtAlias == "" || len(statement.Results) != httpGuardReturnValues {
		return nil
	}

	diagnostic, isCall := statement.Results[1].(*ast.CallExpr)
	if !isCall || !isPackageCall(diagnostic, fmtAlias, "Errorf") {
		return nil
	}

	return diagnostic
}

func namedValueUsesUnchanged(block *ast.BlockStmt, name string, start, end token.Pos, allowed map[token.Pos]bool) bool {
	if block == nil {
		return false
	}

	valid := true

	ast.Inspect(block, func(node ast.Node) bool {
		identifier, isIdentifier := node.(*ast.Ident)
		if !isIdentifier || identifier.Name != name || identifier.Pos() <= start || identifier.Pos() >= end {
			return true
		}

		if !allowed[identifier.Pos()] {
			valid = false

			return false
		}

		return true
	})

	return valid
}

func validHTTPURLBindings(function *ast.FuncDecl, keyCall, urlCall indexedCall) bool {
	return hasErrorReturnGuard(function, keyCall.index, urlCall.index) && assignedTo(urlCall, "target") && requestURLUsesInputs(urlCall.call)
}

func validHTTPRequestBindings(function *ast.FuncDecl, requestCall, sendCall indexedCall, httpAlias string) bool {
	return assignedTo(requestCall, "request") && requestConstructionUsesInputs(requestCall.call, httpAlias) &&
		hasErrorReturnGuard(function, requestCall.index, sendCall.index)
}

func httpRequestIsUnmodified(function *ast.FuncDecl, requestCall, sendCall indexedCall) bool {
	allowed := allowedHTTPBoundaryRequestUses(function, requestCall, sendCall)

	return httpBoundaryRequestHasNoOtherUses(function, requestCall, sendCall, allowed) &&
		httpRequestBodyBytesAreUnmodified(function, requestCall)
}

func httpRequestBodyBytesAreUnmodified(function *ast.FuncDecl, requestCall indexedCall) bool {
	if function == nil || functionBodyBindsName(function, "requestBody") || !hasNamedParameter(function, "body", byteSliceType) {
		return false
	}

	body := requestBodyInput(requestCall.call)
	if body == nil {
		return false
	}

	return namedValueUsesUnchanged(function.Body, "body", function.Body.Pos(), function.Body.End(), map[token.Pos]bool{body.Pos(): true})
}

func requestBodyInput(call *ast.CallExpr) *ast.Ident {
	if call == nil || len(call.Args) != 4 {
		return nil
	}

	if body, isBody := call.Args[3].(*ast.Ident); isBody && body.Name == "body" {
		return body
	}

	bodyCall, isCall := call.Args[3].(*ast.CallExpr)
	if !isCall || !isIdentifier(bodyCall.Fun, "requestBody") || len(bodyCall.Args) != 1 {
		return nil
	}

	body, isBody := bodyCall.Args[0].(*ast.Ident)
	if !isBody || body.Name != "body" {
		return nil
	}

	return body
}

func byteSliceType(expression ast.Expr) bool {
	array, isArray := expression.(*ast.ArrayType)

	return isArray && array.Len == nil && isIdentifier(array.Elt, "byte")
}

func validHTTPBodyFactory(file *ast.File, imports map[string]string) bool {
	factory := uniqueFunction(file, "requestBody")
	if factory == nil || factory.Recv != nil {
		return false
	}

	return bodyFactorySignatureIsKnown(factory, imports) && bodyFactoryBodyIsSafe(factory, imports)
}

func bodyFactorySignatureIsKnown(factory *ast.FuncDecl, imports map[string]string) bool {
	bytesAlias := aliasForPath(imports, "bytes")

	ioAlias := aliasForPath(imports, "io")

	if bytesAlias == "" || ioAlias == "" {
		return false
	}

	if functionAliasShadowed(factory, bytesAlias) || functionAliasShadowed(factory, ioAlias) {
		return false
	}

	return hasNamedParameter(factory, "body", byteSliceType) &&
		hasSingleNamedResult(factory, selectorTypeMatcher(ioAlias, "Reader")) && len(factory.Body.List) == 2
}

func bodyFactoryBodyIsSafe(factory *ast.FuncDecl, imports map[string]string) bool {
	bytesAlias := aliasForPath(imports, "bytes")

	return bodyFactoryHandlesNil(factory.Body.List[0]) && bodyFactoryReturnsReader(factory.Body.List[1], bytesAlias)
}

func hasSingleNamedResult(function *ast.FuncDecl, match func(ast.Expr) bool) bool {
	if function == nil || function.Type.Results == nil || len(function.Type.Results.List) != 1 {
		return false
	}

	result := function.Type.Results.List[0]

	return len(result.Names) == 0 && match(result.Type)
}

func bodyFactoryHandlesNil(statement ast.Stmt) bool {
	guard, isGuard := statement.(*ast.IfStmt)
	if !isGuard || guard.Else != nil || !identifierEqualsNil(guard.Cond, "body") || len(guard.Body.List) != 1 {
		return false
	}

	returned, isReturn := guard.Body.List[0].(*ast.ReturnStmt)

	return isReturn && len(returned.Results) == 1 && isIdentifier(returned.Results[0], "nil")
}

func identifierEqualsNil(expression ast.Expr, name string) bool {
	binary, isBinary := expression.(*ast.BinaryExpr)
	if !isBinary || binary.Op != token.EQL {
		return false
	}

	return (isIdentifier(binary.X, name) && isIdentifier(binary.Y, "nil")) ||
		(isIdentifier(binary.Y, name) && isIdentifier(binary.X, "nil"))
}

func bodyFactoryReturnsReader(statement ast.Stmt, bytesAlias string) bool {
	returned, isReturn := statement.(*ast.ReturnStmt)
	if !isReturn || len(returned.Results) != 1 {
		return false
	}

	call, isCall := returned.Results[0].(*ast.CallExpr)

	return isCall && isPackageCall(call, bytesAlias, "NewReader") &&
		len(call.Args) == 1 && isIdentifier(call.Args[0], "body")
}

func allowedHTTPBoundaryRequestUses(function *ast.FuncDecl, requestCall, sendCall indexedCall) map[token.Pos]bool {
	allowed := make(map[token.Pos]bool)
	requestStart := requestCall.call.End()

	if len(sendCall.call.Args) == 1 && isIdentifier(sendCall.call.Args[0], "request") {
		allowed[sendCall.call.Args[0].Pos()] = true
	}

	ast.Inspect(function.Body, func(node ast.Node) bool {
		rangeStatement, isRange := node.(*ast.RangeStmt)
		if isRange && isValidatedHeaderLoop(rangeStatement, requestStart, sendCall.call.Pos()) {
			allowHeaderLoopRequestUse(rangeStatement, requestStart, sendCall.call.Pos(), allowed)
		}

		return true
	})

	return allowed
}

func isValidatedHeaderLoop(statement *ast.RangeStmt, requestStart, sendStart token.Pos) bool {
	return isIdentifier(statement.X, "headers") && statement.Pos() >= requestStart && statement.End() <= sendStart
}

func allowHeaderLoopRequestUse(statement *ast.RangeStmt, requestStart, sendStart token.Pos, allowed map[token.Pos]bool) {
	ast.Inspect(statement.Body, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall || !headerSetUsesLoopValues(call, statement) {
			return true
		}

		request := validatedHeaderRequestReceiver(call)
		if request != nil && request.Pos() >= requestStart && request.End() <= sendStart {
			allowed[request.Pos()] = true
		}

		return true
	})
}

func validatedHeaderRequestReceiver(call *ast.CallExpr) *ast.Ident {
	method, isMethod := call.Fun.(*ast.SelectorExpr)
	if !isMethod || method.Sel.Name != "Set" {
		return nil
	}

	header, isHeader := method.X.(*ast.SelectorExpr)
	if !isHeader || header.Sel.Name != "Header" {
		return nil
	}

	request, isRequest := header.X.(*ast.Ident)
	if !isRequest || request.Name != "request" {
		return nil
	}

	return request
}

func httpBoundaryRequestHasNoOtherUses(function *ast.FuncDecl, requestCall, sendCall indexedCall, allowed map[token.Pos]bool) bool {
	requestStart := requestCall.call.End()
	requestEnd := sendCall.call.End()
	valid := true

	ast.Inspect(function.Body, func(node ast.Node) bool {
		identifier, isIdentifier := node.(*ast.Ident)
		if !isIdentifier || identifier.Name != "request" || identifier.Pos() < requestStart || identifier.Pos() > requestEnd {
			return true
		}

		if !allowed[identifier.Pos()] {
			valid = false

			return false
		}

		return true
	})

	return valid
}

func validHTTPSendBindings(
	file *ast.File, imports map[string]string, function *ast.FuncDecl,
	requestCall, sendCall indexedCall, wireAlias string,
) bool {
	return assignedTo(sendCall, "response") && sendUsesInjectedClient(sendCall.call) &&
		hasHeaderApplication(function, requestCall.index, sendCall.index) && hasInjectedKeyGuards(file, imports, wireAlias)
}

type indexedCall struct {
	call      *ast.CallExpr
	statement ast.Stmt
	index     int
}

type indexedGuard struct {
	call      *ast.CallExpr
	statement *ast.IfStmt
	index     int
}

func uniqueFunction(file *ast.File, name string) *ast.FuncDecl {
	var found *ast.FuncDecl

	for _, declaration := range file.Decls {
		function, matchedType := declaration.(*ast.FuncDecl)

		if !matchedType || function.Name.Name != name {
			continue
		}

		if found != nil {
			return nil
		}

		found = function
	}

	return found
}

func hasNamedParameter(function *ast.FuncDecl, name string, match func(ast.Expr) bool) bool {
	if function == nil || function.Type.Params == nil {
		return false
	}

	for _, field := range function.Type.Params.List {
		if !match(field.Type) {
			continue
		}

		for _, identifier := range field.Names {
			if identifier.Name == name {
				return true
			}
		}
	}

	return false
}

func functionSignatureBindsName(function *ast.FuncDecl, name string) bool {
	if function == nil {
		return true
	}

	return fieldsContainName(function.Recv, name) ||
		fieldsContainName(function.Type.Params, name) ||
		fieldsContainName(function.Type.Results, name)
}

func isHTTPClientTypeAlias(alias string) func(ast.Expr) bool {
	return func(expression ast.Expr) bool {
		return isHTTPClientType(expression, alias)
	}
}

func hasParameterName(function *ast.FuncDecl, name string) bool {
	if function == nil || function.Type.Params == nil {
		return false
	}

	return fieldsContainName(function.Type.Params, name)
}

func selectorTypeMatcher(alias, name string) func(ast.Expr) bool {
	return func(expression ast.Expr) bool {
		return isSelectorOn(expression, alias, name)
	}
}

func assignedCalls(function *ast.FuncDecl, name, receiver string) []indexedCall {
	var calls []indexedCall

	for index, statement := range function.Body.List {
		assignment, matchedType := statement.(*ast.AssignStmt)

		if !matchedType {
			continue
		}

		for _, expression := range assignment.Rhs {
			call, matchedType := expression.(*ast.CallExpr)

			if matchedType && calledSelector(call, name, receiver) {
				calls = append(calls, indexedCall{call: call, statement: statement, index: index})
			}
		}
	}

	return calls
}

func calledSelector(call *ast.CallExpr, name, receiver string) bool {
	if receiver == "" {
		identifier, matchedType := call.Fun.(*ast.Ident)

		return matchedType && identifier.Name == name
	}

	selector, matchedType := call.Fun.(*ast.SelectorExpr)

	if !matchedType || selector.Sel.Name != name {
		return false
	}

	identifier, matchedType := selector.X.(*ast.Ident)

	return matchedType && identifier.Name == receiver
}

func assignedTo(call indexedCall, name string) bool {
	assignment, matchedType := call.statement.(*ast.AssignStmt)

	if !matchedType {
		return false
	}

	return expressionsContainName(assignment.Lhs, name)
}

func generatedOperationGuards(function *ast.FuncDecl, wireAlias string) []indexedGuard {
	var guards []indexedGuard

	for index, statement := range function.Body.List {
		guard, matchedType := statement.(*ast.IfStmt)

		if !matchedType || guard.Else != nil {
			continue
		}

		call, matchedType := negatedGeneratedCall(guard.Cond, wireAlias, "IsKnownOperation")

		if matchedType {
			guards = append(guards, indexedGuard{call: call, statement: guard, index: index})
		}
	}

	return guards
}

func negatedGeneratedCall(expression ast.Expr, alias, name string) (*ast.CallExpr, bool) {
	unary, matchedType := expression.(*ast.UnaryExpr)

	if !matchedType || unary.Op != token.NOT {
		return nil, false
	}

	call, matchedType := unary.X.(*ast.CallExpr)

	if !matchedType || !calledSelector(call, name, alias) {
		return nil, false
	}

	return call, true
}

func containsDirectReturn(block *ast.BlockStmt) bool {
	for _, statement := range block.List {
		if _, matchedType := statement.(*ast.ReturnStmt); matchedType {
			return true
		}
	}

	return false
}

func pathFormattingUsesSchemaRoute(call *ast.CallExpr) bool {
	return len(call.Args) == 2 && isSelectorOn(call.Args[0], "operation", "Path") && isIdentifier(call.Args[1], "pathArguments")
}

func operationGuardUsesFinalRoute(call *ast.CallExpr) bool {
	return len(call.Args) == 2 && isSelectorOn(call.Args[0], "operation", "Method") && isIdentifier(call.Args[1], "path")
}

func keyValidationUsesRequestInputs(call *ast.CallExpr) bool {
	return len(call.Args) == 2 && isIdentifier(call.Args[0], "query") && isIdentifier(call.Args[1], "headers")
}

func requestURLUsesInputs(call *ast.CallExpr) bool {
	return len(call.Args) == 3 && isIdentifier(call.Args[0], "origin") && isIdentifier(call.Args[1], "path") && isIdentifier(call.Args[2], "query")
}

func requestConstructionUsesInputs(call *ast.CallExpr, httpAlias string) bool {
	if len(call.Args) != 4 || !isSelectorOn(call.Args[1], "operation", "Method") ||
		!isMethodCallOn(call.Args[2], "target", "String") || requestBodyInput(call) == nil {
		return false
	}

	return isPackageCall(call, httpAlias, "NewRequestWithContext")
}

func isMethodCallOn(expression ast.Expr, receiver, method string) bool {
	call, matchedType := expression.(*ast.CallExpr)

	if !matchedType {
		return false
	}

	return isSelectorOn(call.Fun, receiver, method)
}

func isPackageCall(call *ast.CallExpr, alias, name string) bool {
	selector, matchedType := call.Fun.(*ast.SelectorExpr)

	if !matchedType || selector.Sel.Name != name {
		return false
	}

	identifier, matchedType := selector.X.(*ast.Ident)

	return matchedType && identifier.Name == alias
}

func sendUsesInjectedClient(call *ast.CallExpr) bool {
	return len(call.Args) == 1 && isIdentifier(call.Args[0], "request") && calledSelector(call, "Do", "client")
}

func hasErrorReturnGuard(function *ast.FuncDecl, afterIndex, beforeIndex int) bool {
	for index, statement := range function.Body.List {
		if index <= afterIndex || index >= beforeIndex {
			continue
		}

		guard, matchedType := statement.(*ast.IfStmt)

		if matchedType && guard.Else == nil && errorIsNotNil(guard.Cond) && containsDirectReturn(guard.Body) {
			return true
		}
	}

	return false
}

func errorIsNotNil(expression ast.Expr) bool {
	binary, matchedType := expression.(*ast.BinaryExpr)

	if !matchedType || binary.Op != token.NEQ {
		return false
	}

	return isIdentifier(binary.X, "err") && isIdentifier(binary.Y, "nil")
}

func hasInjectedKeyGuards(file *ast.File, imports map[string]string, wireAlias string) bool {
	function := uniqueFunction(file, "validateRequestKeys")

	if function == nil || functionAliasShadowed(function, wireAlias) ||
		!hasParameterName(function, "query") || !hasParameterName(function, "headers") {
		return false
	}

	return imports[wireAlias] == wireImport && keyRangeGuard(function, "query", wireAlias, "IsKnownQueryParam") &&
		keyRangeGuard(function, "headers", wireAlias, "IsKnownHeader")
}

func keyRangeGuard(function *ast.FuncDecl, parameter, wireAlias, predicate string) bool {
	for _, statement := range function.Body.List {
		rangeStatement, matchedType := statement.(*ast.RangeStmt)

		if !matchedType || !isIdentifier(rangeStatement.X, parameter) {
			continue
		}

		if keyRangeBodyGuard(rangeStatement, wireAlias, predicate) {
			return true
		}
	}

	return false
}

func keyRangeBodyGuard(rangeStatement *ast.RangeStmt, wireAlias, predicate string) bool {
	for _, bodyStatement := range rangeStatement.Body.List {
		guard, matchedType := bodyStatement.(*ast.IfStmt)

		if !matchedType || guard.Else != nil || !containsDirectReturn(guard.Body) {
			continue
		}

		call, isPredicate := negatedGeneratedCall(guard.Cond, wireAlias, predicate)

		if isPredicate && len(call.Args) == 1 && isIdentifier(call.Args[0], "name") && isIdentifier(rangeStatement.Key, "name") {
			return true
		}
	}

	return false
}

func onlyExpectedHTTPCalls(file *ast.File, httpAlias string, requestCall, sendCall *ast.CallExpr) bool {
	valid := true

	for _, declaration := range file.Decls {
		ast.Inspect(declaration, func(node ast.Node) bool {
			selector, matchedType := node.(*ast.SelectorExpr)

			if !matchedType {
				return true
			}

			if isHTTPPackageCallOn(selector, httpAlias) && selector != requestCall.Fun {
				valid = false

				return false
			}

			if !isHTTPClientNetworkMethod(selector.Sel.Name) || !isHTTPClientReceiver(file, declaration, selector.X, httpAlias) {
				return true
			}

			if selector != sendCall.Fun {
				valid = false

				return false
			}

			return true
		})
	}

	return valid
}

func isHTTPPackageCallOn(selector *ast.SelectorExpr, alias string) bool {
	if !isHTTPPackageCall(selector.Sel.Name) {
		return false
	}

	return isIdentifier(selector.X, alias)
}

func isHTTPClientNetworkMethod(name string) bool {
	return isHTTPClientMethod(name) || name == "Do" || name == "RoundTrip"
}

func hasHeaderApplication(function *ast.FuncDecl, afterIndex, beforeIndex int) bool {
	for index, statement := range function.Body.List {
		if index <= afterIndex || index >= beforeIndex {
			continue
		}

		rangeStatement, matchedType := statement.(*ast.RangeStmt)

		if !matchedType || !isIdentifier(rangeStatement.X, "headers") {
			continue
		}

		for _, bodyStatement := range rangeStatement.Body.List {
			expression, matchedType := bodyStatement.(*ast.ExprStmt)

			if !matchedType {
				continue
			}

			call, matchedType := expression.X.(*ast.CallExpr)

			if matchedType && headerSetUsesLoopValues(call, rangeStatement) {
				return true
			}
		}
	}

	return false
}

func headerSetUsesLoopValues(call *ast.CallExpr, rangeStatement *ast.RangeStmt) bool {
	if len(call.Args) != 2 || !isIdentifier(call.Args[0], "name") || !isIdentifier(call.Args[1], "value") ||
		!isIdentifier(rangeStatement.Key, "name") || !isIdentifier(rangeStatement.Value, "value") {
		return false
	}

	selector, matchedType := call.Fun.(*ast.SelectorExpr)

	if !matchedType || selector.Sel.Name != "Set" {
		return false
	}

	return isSelectorOn(selector.X, "request", "Header")
}

func validateMQTTBoundary(file *ast.File, imports map[string]string) error {
	if aliasForPath(imports, pahoImport) == "" {
		return fmt.Errorf("MQTT boundary must wrap pinned Paho: %w", errNetworkBoundary)
	}

	var newClient, newOptions int

	for _, declaration := range file.Decls {
		ast.Inspect(declaration, func(node ast.Node) bool {
			name := pahoConstructorName(node, imports)
			switch name {
			case "NewClient":
				newClient++
			case "NewClientOptions":
				newOptions++
			}

			return true
		})
	}

	if newClient != 1 || newOptions != 1 {
		return fmt.Errorf("MQTT boundary must expose exactly one Paho client and options constructor: %w", errNetworkBoundary)
	}

	return nil
}

func pahoConstructorName(node ast.Node, imports map[string]string) string {
	call, matchedType := node.(*ast.CallExpr)

	if !matchedType {
		return ""
	}

	selector, matchedType := call.Fun.(*ast.SelectorExpr)

	if !matchedType {
		return ""
	}

	base, matchedType := selector.X.(*ast.Ident)

	if !matchedType || imports[base.Name] != pahoImport {
		return ""
	}

	return selector.Sel.Name
}

func aliasForPath(imports map[string]string, path string) string {
	for alias, importedPath := range imports {
		if importedPath == path {
			return alias
		}
	}

	return ""
}

func isExternalNetworkPackage(path string) bool {
	if path == "github.com/portpowered/go-tuya" || strings.HasPrefix(path, "github.com/portpowered/go-tuya/") {
		return false
	}

	switch path {
	case pahoImport, "github.com/gobwas/ws":
		return true
	}

	path = strings.ToLower(path)
	for _, protocol := range []string{"websocket", "web-socket", "mqtt", "amqp", "grpc", "socket"} {
		if strings.Contains(path, protocol) {
			return true
		}
	}

	return false
}

func networkDialSelectorUses(file *ast.File) []string {
	methods := map[string]bool{
		networkDialMethod: true, networkDialContextMethod: true, "DialTimeout": true, "DialWithDialer": true,
		"DialTLS": true, "DialTLSContext": true, "Listen": true, "ListenPacket": true,
		"ListenTCP": true, "ListenUDP": true, "ListenMulticastUDP": true, "ListenUnix": true,
	}

	var found []string

	for _, declaration := range file.Decls {
		ast.Inspect(declaration, func(node ast.Node) bool {
			selector, matchedType := node.(*ast.SelectorExpr)
			if matchedType && methods[selector.Sel.Name] {
				found = append(found, selector.Sel.Name)
			}

			return true
		})
	}

	sort.Strings(found)

	return found
}

func isHTTPPackageCall(name string) bool {
	switch name {
	case "Get", "Post", "PostForm", "Head", "NewRequest", "NewRequestWithContext":
		return true
	default:
		return false
	}
}

func networkSelectorUses(file *ast.File, alias string, names map[string]bool) []string {
	var found []string

	for _, declaration := range file.Decls {
		ast.Inspect(declaration, func(node ast.Node) bool {
			selector, matchedType := node.(*ast.SelectorExpr)

			if !matchedType || !names[selector.Sel.Name] {
				return true
			}

			if base, matchedType := selector.X.(*ast.Ident); matchedType && base.Name == alias {
				found = append(found, selector.Sel.Name)
			}

			return true
		})
	}

	sort.Strings(found)

	return found
}

func isSelectorOn(expression ast.Expr, base, member string) bool {
	selector, matchedType := expression.(*ast.SelectorExpr)

	if !matchedType || selector.Sel.Name != member {
		return false
	}

	identifier, matchedType := selector.X.(*ast.Ident)

	return matchedType && identifier.Name == base
}

func isIdentifier(expression ast.Expr, name string) bool {
	identifier, matchedType := expression.(*ast.Ident)

	return matchedType && identifier.Name == name
}

type externalMQTTContract struct {
	Contract   string `yaml:"contract"`
	Evidence   string `yaml:"evidence"`
	Dependency struct {
		Module  string `yaml:"module"`
		Version string `yaml:"version"`
	} `yaml:"dependency"`
	Source struct {
		TransportWrapper    string `yaml:"transportWrapper"`
		SessionAdapter      string `yaml:"sessionAdapter"`
		ConnectionInjection string `yaml:"connectionInjection"`
		ReplayTest          string `yaml:"replayTest"`
	} `yaml:"source"`
	Protocol struct {
		Name    string `yaml:"name"`
		Level   int    `yaml:"level"`
		Version string `yaml:"version"`
	} `yaml:"protocol"`
	Replay struct {
		Classification       string `yaml:"classification"`
		SuccessFixture       string `yaml:"successFixture"`
		DeniedConnectFixture string `yaml:"deniedConnectFixture"`
		SuccessTest          string `yaml:"successTest"`
		FailureTest          string `yaml:"failureTest"`
		Transport            string `yaml:"transport"`
	} `yaml:"replay"`
}

type externalMQTTFixture struct {
	Provenance string              `json:"provenance"`
	Frames     []externalMQTTFrame `json:"frames"`
}

const (
	networkDialMethod = "Dial"

	networkDialContextMethod = "DialContext"

	httpGuardReturnValues = 2

	mqttClientDirection = "client"

	mqttServerDirection = "server"

	mqttConnectPacket = 1

	mqttConnackPacket = 2

	mqttPublishPacket = 3

	mqttSubscribePacket = 8

	mqttSubackPacket = 9

	mqttUnsubscribePacket = 10

	mqttUnsubackPacket = 11

	mqttDisconnectPacket = 14
)

type externalMQTTFrame struct {
	Direction string `json:"direction"`
	Hex       string `json:"hex"`
	PacketID  bool   `json:"packet_id"`
}

type expectedMQTTFrame struct {
	direction  string
	packetType byte
	packetID   bool
}

func validateExternalMQTTContract() error {
	const contractPath = "api/external/paho-mqtt-v1.5.1.yaml"
	// #nosec G304 -- the schema path is fixed in the repository.
	contractData, err := os.ReadFile(contractPath)
	if err != nil {
		return fmt.Errorf("read external MQTT contract: %w", err)
	}

	var contract externalMQTTContract

	err = yaml.Unmarshal(contractData, &contract)
	if err != nil {
		return fmt.Errorf("decode external MQTT contract: %w", err)
	}

	if mismatchedMQTTDependency(contract) || mismatchedMQTTSource(contract) || mismatchedMQTTProtocolReplay(contract) {
		return fmt.Errorf("external MQTT contract is incomplete or mismatched: %w", errNetworkBoundary)
	}

	err = validateMQTTModuleVersions()
	if err != nil {
		return err
	}

	err = validateMQTTFixture(contract.Replay.SuccessFixture, []expectedMQTTFrame{
		{direction: mqttClientDirection, packetType: mqttConnectPacket, packetID: false},
		{direction: mqttServerDirection, packetType: mqttConnackPacket, packetID: false},
		{direction: mqttClientDirection, packetType: mqttSubscribePacket, packetID: true},
		{direction: mqttServerDirection, packetType: mqttSubackPacket, packetID: true},
		{direction: mqttClientDirection, packetType: mqttSubscribePacket, packetID: true},
		{direction: mqttServerDirection, packetType: mqttSubackPacket, packetID: true},
		{direction: mqttServerDirection, packetType: mqttPublishPacket, packetID: false},
		{direction: mqttServerDirection, packetType: mqttPublishPacket, packetID: false},
		{direction: mqttClientDirection, packetType: mqttUnsubscribePacket, packetID: true},
		{direction: mqttServerDirection, packetType: mqttUnsubackPacket, packetID: true},
		{direction: mqttClientDirection, packetType: mqttDisconnectPacket, packetID: false},
	}, false)
	if err != nil {
		return err
	}

	err = validateMQTTFixture(contract.Replay.DeniedConnectFixture, []expectedMQTTFrame{
		{direction: mqttClientDirection, packetType: mqttConnectPacket, packetID: false},
		{direction: mqttServerDirection, packetType: mqttConnackPacket, packetID: false},
	}, true)
	if err != nil {
		return err
	}

	return validateMQTTReplaySource(contract)
}

func validateMQTTReplaySource(contract externalMQTTContract) error {
	replaySource, err := os.ReadFile(contract.Source.ReplayTest)
	if err != nil {
		return fmt.Errorf("read external MQTT paired replay test: %w", err)
	}

	if !strings.Contains(string(replaySource), "SetCustomOpenConnectionFn") ||
		!strings.Contains(string(replaySource), contract.Replay.SuccessTest) ||
		!strings.Contains(string(replaySource), contract.Replay.FailureTest) {
		return fmt.Errorf("external MQTT contract has no matching connection-injection replay tests: %w", errNetworkBoundary)
	}

	return nil
}

func mismatchedMQTTDependency(contract externalMQTTContract) bool {
	return contract.Contract != "mqtt" ||
		contract.Evidence != "implementation-derived" ||
		contract.Dependency.Module != pahoImport ||
		contract.Dependency.Version != "v1.5.1"
}

func mismatchedMQTTSource(contract externalMQTTContract) bool {
	return contract.Source.TransportWrapper != mqttBoundary ||
		contract.Source.SessionAdapter != "pkg/tuya/messages.go" ||
		contract.Source.ReplayTest != "pkg/tuya/mqtt_framed_replay_test.go"
}

func mismatchedMQTTProtocolReplay(contract externalMQTTContract) bool {
	return contract.Protocol.Name != "MQTT" ||
		contract.Protocol.Level != 4 ||
		contract.Protocol.Version != "3.1.1" ||
		contract.Replay.Classification != "synthetic" ||
		contract.Replay.Transport != "net.Pipe returned by SetCustomOpenConnectionFn"
}

func validateMQTTModuleVersions() error {
	moduleData, err := os.ReadFile("go.mod")
	if err != nil {
		return fmt.Errorf("read root module metadata: %w", err)
	}

	if !moduleRequiresVersion(string(moduleData), pahoImport, "v1.5.1") {
		return fmt.Errorf("root module does not pin Paho v1.5.1: %w", errNetworkBoundary)
	}

	cliModuleData, err := os.ReadFile("cmd/go-tuya/go.mod")
	if err != nil {
		return fmt.Errorf("read CLI module metadata: %w", err)
	}

	if moduleRequiresVersion(string(cliModuleData), pahoImport, "v1.5.1") {
		return fmt.Errorf("CLI must consume the SDK MQTT wrapper, not add Paho as a direct network edge: %w", errNetworkBoundary)
	}

	return nil
}

func moduleRequiresVersion(moduleText, modulePath, version string) bool {
	for line := range strings.SplitSeq(moduleText, "\n") {
		fields := strings.Fields(line)

		if len(fields) == 2 && fields[0] == modulePath && fields[1] == version {
			return true
		}
	}

	return false
}

func validateMQTTFixture(path string, expected []expectedMQTTFrame, deniedConnect bool) error {
	// #nosec G304 -- fixture paths are read from a checked-in source-matched contract.
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read MQTT fixture %q: %w", path, err)
	}

	var fixture externalMQTTFixture

	err = json.Unmarshal(data, &fixture)
	if err != nil {
		return fmt.Errorf("decode MQTT fixture %q: %w", path, err)
	}

	if !strings.Contains(strings.ToLower(fixture.Provenance), "synthetic") || len(fixture.Frames) != len(expected) {
		return fmt.Errorf("MQTT fixture %q is not the expected synthetic transcript: %w", path, errNetworkBoundary)
	}

	for index, frame := range fixture.Frames {
		err := validateMQTTFixtureFrame(path, index, frame, expected[index])
		if err != nil {
			return err
		}
	}

	if deniedConnect {
		packet, _ := hex.DecodeString(fixture.Frames[1].Hex)

		if len(packet) != 4 || !strings.EqualFold(fixture.Frames[1].Hex, "20020005") {
			return fmt.Errorf("denied CONNACK must carry the synthetic authorization rejection code: %w", errNetworkBoundary)
		}
	}

	return nil
}

func validateMQTTFixtureFrame(path string, index int, frame externalMQTTFrame, want expectedMQTTFrame) error {
	packet, decodeErr := hex.DecodeString(frame.Hex)

	if decodeErr != nil || len(packet) < 2 {
		return fmt.Errorf("MQTT fixture %q frame %d is invalid: %w", path, index, errNetworkBoundary)
	}

	if frame.Direction != want.direction || packet[0]>>4 != want.packetType || frame.PacketID != want.packetID {
		return fmt.Errorf("MQTT fixture %q frame %d does not match contract order: %w", path, index, errNetworkBoundary)
	}

	return nil
}
