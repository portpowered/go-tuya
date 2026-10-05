package main

import (
	"go/ast"
	"path/filepath"
	"strconv"
)

func generatedWireEnumFieldType(
	receiver ast.Expr, fieldName string, aliases map[string]bool, path string,
	models map[string]generatedModel, assignments wireSourceAssignments,
) string {
	modelName := generatedWireModelForValue(receiver, aliases, path, models, assignments, make(map[wireSourceVariable]bool))

	model, exists := generatedWireResolvedModel(modelName, models, make(map[string]bool))
	if !exists {
		return ""
	}

	enumName := model.Fields[fieldName]

	enum, exists := models[enumName]
	if !exists || len(enum.EnumMembers) == 0 {
		return ""
	}

	return enumName
}

func generatedWireResolvedModel(name string, models map[string]generatedModel, visiting map[string]bool) (generatedModel, bool) {
	model, exists := models[name]
	if !exists || visiting[name] {
		return generatedModel{Name: "", File: "", Alias: "", Fields: nil, EnumMembers: nil}, false
	}

	if len(model.Fields) != 0 || model.Alias == "" {
		return model, true
	}

	visiting[name] = true

	return generatedWireResolvedModel(model.Alias, models, visiting)
}

//nolint:cyclop,gocognit // Generated model provenance crosses declarations and local helper returns.
func generatedWireModelForValue(
	expression ast.Expr, aliases map[string]bool, path string, models map[string]generatedModel,
	assignments wireSourceAssignments, visiting map[wireSourceVariable]bool,
) string {
	if expression == nil {
		return ""
	}

	switch expression := expression.(type) {
	case *ast.CompositeLit:
		return generatedWireModelForType(expression.Type, aliases, path, models, assignments)
	case *ast.ParenExpr:
		return generatedWireModelForValue(expression.X, aliases, path, models, assignments, visiting)
	case *ast.StarExpr, *ast.UnaryExpr, *ast.TypeAssertExpr:
		return generatedWireModelForValue(wireExpressionOperand(expression), aliases, path, models, assignments, visiting)
	case *ast.Ident:
		return generatedWireModelForIdentifier(expression, aliases, path, models, assignments, visiting)
	case *ast.SelectorExpr:
		for _, value := range wireAggregateFieldExpressions(expression, assignments) {
			if name := generatedWireModelForValue(value, aliases, path, models, assignments, visiting); name != "" {
				return name
			}
		}
	case *ast.CallExpr:
		if name := generatedWireModelForType(expression.Fun, aliases, path, models, assignments); name != "" {
			return name
		}

		for _, function := range wireLocalHelpers(expression.Fun, assignments, make(map[wireSourceVariable]bool)) {
			if function == nil {
				continue
			}

			fileAliases, filePath := wireAliasesForNode(function, aliases, path, assignments)

			if function.Results != nil {
				for _, result := range function.Results.List {
					if name := generatedWireModelForType(result.Type, fileAliases, filePath, models, assignments); name != "" {
						return name
					}
				}
			}

			if body := assignments.functionBodies[function]; body != nil {
				for _, result := range generatedWireHelperReturns(function, body) {
					if name := generatedWireModelForValue(result, aliases, path, models, assignments, visiting); name != "" {
						return name
					}
				}
			}
		}
	}

	return ""
}

//nolint:cyclop // Identifier analysis checks declared types before following each source alias.
func generatedWireModelForIdentifier(
	identifier *ast.Ident, aliases map[string]bool, path string, models map[string]generatedModel,
	assignments wireSourceAssignments, visiting map[wireSourceVariable]bool,
) string {
	if identifier.Obj == nil {
		if filepath.ToSlash(filepath.Dir(wireSourcePathForNode(identifier, path, assignments))) == generatedModelDirectory && models[identifier.Name].Name != "" {
			return identifier.Name
		}

		return ""
	}

	declaration, isNode := identifier.Obj.Decl.(ast.Node)

	key := wireSourceVariable{declaration: declaration, name: identifier.Name}
	if !isNode || visiting[key] {
		return ""
	}

	visiting[key] = true

	switch declaration := declaration.(type) {
	case *ast.Field:
		if name := generatedWireModelForType(declaration.Type, aliases, path, models, assignments); name != "" {
			return name
		}
	case *ast.ValueSpec:
		if name := generatedWireModelForType(declaration.Type, aliases, path, models, assignments); name != "" {
			return name
		}
	case *ast.TypeSpec:
		fileAliases, filePath := wireAliasesForNode(declaration, aliases, path, assignments)

		return generatedWireModelForType(declaration.Type, fileAliases, filePath, models, assignments)
	}

	if value := wireAliasValue(identifier.Name, declaration); value != nil {
		if name := generatedWireModelForValue(value, aliases, path, models, assignments, visiting); name != "" {
			return name
		}
	}

	for _, value := range wireAliasExpressions(identifier, assignments) {
		if name := generatedWireModelForValue(value, aliases, path, models, assignments, visiting); name != "" {
			return name
		}
	}

	return ""
}

//nolint:cyclop // Type analysis resolves generated imports and local aliases without type-checking the package.
func generatedWireModelForType(
	expression ast.Expr, aliases map[string]bool, path string, models map[string]generatedModel, assignments wireSourceAssignments,
) string {
	switch expression := expression.(type) {
	case *ast.StarExpr, *ast.ParenExpr:
		return generatedWireModelForType(wireExpressionOperand(expression), aliases, path, models, assignments)
	case *ast.SelectorExpr:
		identifier, isPackage := expression.X.(*ast.Ident)

		fileAliases, _ := wireAliasesForNode(expression, aliases, path, assignments)
		if isPackage && identifier.Obj == nil && fileAliases[identifier.Name] && models[expression.Sel.Name].Name != "" {
			return expression.Sel.Name
		}
	case *ast.Ident:
		if expression.Obj == nil {
			if filepath.ToSlash(filepath.Dir(wireSourcePathForNode(expression, path, assignments))) == generatedModelDirectory && models[expression.Name].Name != "" {
				return expression.Name
			}

			return ""
		}

		declaration, isNode := expression.Obj.Decl.(ast.Node)
		if !isNode {
			return ""
		}

		if typeSpec, isType := declaration.(*ast.TypeSpec); isType {
			declarationAliases, declarationPath := wireAliasesForNode(typeSpec, aliases, path, assignments)
			if filepath.ToSlash(filepath.Dir(declarationPath)) == generatedModelDirectory && models[typeSpec.Name.Name].Name != "" {
				return typeSpec.Name.Name
			}

			return generatedWireModelForType(typeSpec.Type, declarationAliases, declarationPath, models, assignments)
		}
	}

	return ""
}

func wireExpressionOperand(expression ast.Expr) ast.Expr {
	switch expression := expression.(type) {
	case *ast.StarExpr:
		return expression.X
	case *ast.UnaryExpr:
		return expression.X
	case *ast.TypeAssertExpr:
		return expression.X
	case *ast.ParenExpr:
		return expression.X
	default:
		return nil
	}
}

func wireAliasesForNode(node ast.Node, aliases map[string]bool, path string, assignments wireSourceAssignments) (map[string]bool, string) {
	file := assignments.sourceFiles[node]
	if file == nil {
		return aliases, path
	}

	return primitiveImportAliases(file, primitivePackage{ImportPath: wireImportPath, Name: generatedModelPackageName, Directory: generatedModelDirectory}),
		wireSourcePathForNode(node, path, assignments)
}

func wireSourcePathForNode(node ast.Node, fallback string, assignments wireSourceAssignments) string {
	if path := assignments.sourcePaths[node]; path != "" {
		return path
	}

	return fallback
}

func generatedWireHelperReturns(function *ast.FuncType, body *ast.BlockStmt) []ast.Expr {
	var expressions []ast.Expr

	ast.Inspect(body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}

		if returned, isReturn := node.(*ast.ReturnStmt); isReturn {
			expressions = append(expressions, wireReturnExpressions(function, returned)...)
		}

		return true
	})

	return expressions
}

func generatedWireMapResult(
	call *ast.CallExpr, aliases map[string]bool, path string, models map[string]generatedModel, assignments wireSourceAssignments,
	visiting map[wireSourceVariable]bool,
) bool {
	generatedArgument := false

	for _, argument := range call.Args {
		if generatedWireReceiver(argument, aliases, path, models, visiting, assignments) {
			generatedArgument = true

			break
		}
	}

	if !generatedArgument {
		return false
	}

	for _, function := range wireLocalHelpers(call.Fun, assignments, make(map[wireSourceVariable]bool)) {
		if function == nil || function.Results == nil || len(function.Results.List) == 0 {
			continue
		}

		if wireMapResultType(function.Results.List[0].Type) {
			return true
		}
	}

	return false
}

func wireMapResultType(expression ast.Expr) bool {
	switch expression := expression.(type) {
	case *ast.MapType:
		return true
	case *ast.ParenExpr:
		return wireMapResultType(expression.X)
	case *ast.StarExpr:
		return wireMapResultType(expression.X)
	}

	return false
}

//nolint:cyclop,funlen,gocognit // Enum validation checks generated constants, caller inputs, aliases, and helper returns.
func generatedWireEnumExpressionAllowed(
	expression ast.Expr, enumName string, aliases map[string]bool, path string, models map[string]generatedModel,
	assignments wireSourceAssignments, visiting map[wireSourceVariable]bool,
) bool {
	if expression == nil {
		return false
	}

	switch expression := expression.(type) {
	case *ast.ParenExpr:
		return generatedWireEnumExpressionAllowed(expression.X, enumName, aliases, path, models, assignments, visiting)
	case *ast.SelectorExpr:
		identifier, isPackage := expression.X.(*ast.Ident)
		fileAliases, _ := wireAliasesForNode(expression, aliases, path, assignments)

		enum := models[enumName]
		if isPackage && identifier.Obj == nil && fileAliases[identifier.Name] && enum.EnumMembers[expression.Sel.Name] != "" {
			return true
		}

		if generatedWireEnumStructFieldMatches(expression, enumName, aliases, path, models, assignments) {
			return true
		}

		for _, value := range wireAggregateFieldExpressions(expression, assignments) {
			if generatedWireEnumExpressionAllowed(value, enumName, aliases, path, models, assignments, visiting) {
				return true
			}
		}

		return false
	case *ast.Ident:
		return generatedWireEnumIdentifierAllowed(expression, enumName, aliases, path, models, assignments, visiting)
	case *ast.CallExpr:
		if generatedWireModelForType(expression.Fun, aliases, path, models, assignments) == enumName && len(expression.Args) == 1 {
			return generatedWireEnumExpressionAllowed(expression.Args[0], enumName, aliases, path, models, assignments, visiting)
		}

		functions := wireLocalHelpers(expression.Fun, assignments, make(map[wireSourceVariable]bool))
		if len(functions) == 0 {
			return false
		}

		for _, function := range functions {
			if function == nil {
				return false
			}

			body := assignments.functionBodies[function]
			if body == nil {
				return false
			}

			found := false
			for _, result := range generatedWireHelperReturns(function, body) {
				found = true

				if !generatedWireEnumExpressionAllowed(result, enumName, aliases, path, models,
					wireReturnParameterValues(function, expression, assignments), visiting) {
					return false
				}
			}

			if !found {
				return false
			}
		}

		return true
	case *ast.BasicLit:
		return false
	}

	return false
}

func generatedWireEnumStructFieldMatches(
	expression *ast.SelectorExpr, enumName string, aliases map[string]bool, path string,
	models map[string]generatedModel, assignments wireSourceAssignments,
) bool {
	owner := wireReceiverDeclaration(expression.X, assignments, make(map[wireSourceVariable]bool))
	if owner == nil {
		return false
	}

	structure := wireReceiverStructure(owner, make(map[*ast.TypeSpec]bool))
	if structure == nil || structure.Fields == nil {
		return false
	}

	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			if name.Name != expression.Sel.Name {
				continue
			}

			fieldAliases, fieldPath := wireAliasesForNode(field, aliases, path, assignments)
			if generatedWireModelForType(field.Type, fieldAliases, fieldPath, models, assignments) == enumName {
				return true
			}
		}
	}

	return false
}

func generatedWireEnumIdentifierAllowed(
	identifier *ast.Ident, enumName string, aliases map[string]bool, path string, models map[string]generatedModel,
	assignments wireSourceAssignments, visiting map[wireSourceVariable]bool,
) bool {
	if identifier.Obj == nil {
		return false
	}

	declaration, isNode := identifier.Obj.Decl.(ast.Node)

	key := wireSourceVariable{declaration: declaration, name: identifier.Name}
	if !isNode || visiting[key] {
		return false
	}

	if parameter, isParameter := declaration.(*ast.Field); isParameter {
		fileAliases, filePath := wireAliasesForNode(parameter, aliases, path, assignments)

		return generatedWireModelForType(parameter.Type, fileAliases, filePath, models, assignments) == enumName
	}

	visiting[key] = true

	values := wireAliasExpressions(identifier, assignments)
	if value := wireAliasValue(identifier.Name, declaration); value != nil {
		values = append(values, value)
	}

	if len(values) == 0 {
		return false
	}

	for _, value := range values {
		if !generatedWireEnumExpressionAllowed(value, enumName, aliases, path, models, assignments, visiting) {
			return false
		}
	}

	return true
}

func generatedWireRecursiveHelperCall(call *ast.CallExpr, assignments wireSourceAssignments) bool {
	for _, function := range wireLocalHelpers(call.Fun, assignments, make(map[wireSourceVariable]bool)) {
		if function == nil || assignments.functionBodies[function] == nil {
			continue
		}

		if wireHelperRecurses(function, assignments, make(map[*ast.FuncType]bool), make(map[*ast.FuncType]bool)) {
			return true
		}
	}

	return false
}

func wireHelperRecurses(
	function *ast.FuncType, assignments wireSourceAssignments, visiting map[*ast.FuncType]bool, checked map[*ast.FuncType]bool,
) bool {
	if visiting[function] {
		return true
	}

	if checked[function] {
		return false
	}

	body := assignments.functionBodies[function]
	if body == nil {
		checked[function] = true

		return false
	}

	visiting[function] = true

	recursive := false

	ast.Inspect(body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}

		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return !recursive
		}

		for _, callee := range wireLocalHelpers(call.Fun, assignments, make(map[wireSourceVariable]bool)) {
			if callee != nil && wireHelperRecurses(callee, assignments, visiting, checked) {
				recursive = true

				return false
			}
		}

		return !recursive
	})
	delete(visiting, function)

	checked[function] = true

	return recursive
}

func importedUnverifiedWireHelper(file *ast.File, call *ast.CallExpr, assignments wireSourceAssignments) bool {
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector || verifiedWireCallable(file, call.Fun, assignments) {
		return false
	}

	identifier, isPackage := selector.X.(*ast.Ident)
	if !isPackage || identifier.Obj != nil {
		return false
	}

	for _, imported := range file.Imports {
		importPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			continue
		}

		alias := filepath.Base(importPath)
		if imported.Name != nil {
			alias = imported.Name.Name
		}

		if alias == identifier.Name {
			return true
		}
	}

	return false
}

func wireUnboundHelperParameter(identifier *ast.Ident, assignments wireSourceAssignments) bool {
	return wireUnboundHelperParameterVisiting(identifier, assignments, make(map[wireSourceVariable]bool))
}

func wireUnboundHelperParameterVisiting(
	identifier *ast.Ident, assignments wireSourceAssignments, visiting map[wireSourceVariable]bool,
) bool {
	field, isParameter := wireParameterDeclaration(identifier)
	if isParameter {
		key := wireSourceVariable{declaration: field, name: identifier.Name}
		if assignments.externalParameters[key] {
			return false
		}

		if visiting[key] {
			return true
		}

		visiting[key] = true
		defer delete(visiting, key)

		values := assignments.values[key]
		if len(values) == 0 {
			return true
		}

		for _, value := range values {
			if wireExpressionHasUnboundParameter(value, assignments, visiting) {
				return true
			}
		}

		return false
	}

	for _, value := range wireAliasExpressions(identifier, assignments) {
		if wireExpressionHasUnboundParameter(value, assignments, visiting) {
			return true
		}
	}

	return false
}

func wireExpressionHasUnboundParameter(expression ast.Expr, assignments wireSourceAssignments, visiting map[wireSourceVariable]bool) bool {
	unbound := false

	ast.Inspect(expression, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}

		identifier, isIdentifier := node.(*ast.Ident)
		if isIdentifier && wireUnboundHelperParameterVisiting(identifier, assignments, visiting) {
			unbound = true

			return false
		}

		return !unbound
	})

	return unbound
}

func wireParameterDeclaration(identifier *ast.Ident) (*ast.Field, bool) {
	if identifier.Obj == nil {
		return nil, false
	}

	declaration, isField := identifier.Obj.Decl.(*ast.Field)

	return declaration, isField
}

func wireEnumAssignmentProblem(
	left ast.Expr, right ast.Expr, aliases map[string]bool, path string,
	models map[string]generatedModel, assignments wireSourceAssignments,
) string {
	selector, isSelector := left.(*ast.SelectorExpr)
	if !isSelector {
		return ""
	}

	enumName := generatedWireEnumFieldType(selector.X, selector.Sel.Name, aliases, path, models, assignments)
	if enumName == "" || generatedWireEnumExpressionAllowed(right, enumName, aliases, path, models, assignments, make(map[wireSourceVariable]bool)) {
		return ""
	}

	return enumName
}
