package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
)

//nolint:cyclop // This source gate combines construction, escape, and mutation findings for each file.
func rejectRawGeneratedWireConstructionsWithAssignments(
	file *ast.File, set *token.FileSet, path string, models map[string]generatedModel, assignments wireSourceAssignments,
) error {
	owner := primitivePackage{ImportPath: wireImportPath, Name: generatedModelPackageName, Directory: generatedModelDirectory}
	aliases := primitiveImportAliases(file, owner)
	syntaxLiterals := wireSyntaxLiteralPositions(file)

	var problems []string

	reportFixedWireLiteral := func(raw *ast.BasicLit, description string) {
		if syntaxLiterals[raw.Pos()] {
			return
		}

		decoded, err := strconv.Unquote(raw.Value)
		if err == nil && decoded != "" {
			problems = append(problems, fmt.Sprintf("%s: fixed %s %q must be generated from schema", set.Position(raw.Pos()), description, decoded))
		}
	}
	reportWireValue := func(raw *ast.BasicLit) { reportFixedWireLiteral(raw, "wire value") }
	reportWireAssignment := func(raw *ast.BasicLit) { reportFixedWireLiteral(raw, "wire assignment") }

	ast.Inspect(file, func(node ast.Node) bool {
		if call, isCall := node.(*ast.CallExpr); isCall {
			if unverifiedWireCallable(file, call, aliases, path, models, assignments) {
				problems = append(problems, fmt.Sprintf("%s: generated wire value escapes to an unverified callable", set.Position(call.Pos())))
			}

			inspectWireURLValuesMutation(file, call, assignments, reportWireValue)
			inspectWireCopyCall(file, call, aliases, path, models, assignments, reportWireValue)
		}

		if assignment, isAssignment := node.(*ast.AssignStmt); isAssignment {
			inspectWireMutationAssignment(assignment, file, aliases, path, models, assignments, reportWireAssignment)
		}

		literal, isComposite := node.(*ast.CompositeLit)
		if !isComposite {
			return true
		}

		if wireURLValuesDeclaredType(literal.Type, file, make(map[wireSourceVariable]bool)) {
			inspectWireURLValuesLiteral(literal, file, assignments, reportWireValue)

			return false
		}

		if !generatedWireDeclaredType(literal.Type, aliases, path, models) {
			return true
		}

		inspectWireValueLiterals(literal, make(map[wireSourceVariable]bool), reportWireValue, assignments)

		return false
	})

	if len(problems) != 0 {
		return fmt.Errorf("%w:\n%s", errWireSourceProvenance, strings.Join(problems, "\n"))
	}

	return nil
}

// Delimiters passed to the exact standard-library strings.Join function are
// formatting syntax. They do not close the caller-provided string field to a
// fixed SDK value. Resolve the import and selector so a local function or a
// shadowed package name cannot receive this exemption.
//
//nolint:cyclop // The exemption requires the exact imported strings.Join call and an unshadowed package name.
func wireSyntaxLiteralPositions(file *ast.File) map[token.Pos]bool {
	aliases := make(map[string]bool)

	for _, imported := range file.Imports {
		importPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil || importPath != stringsImportPath {
			continue
		}

		alias := stringsImportPath
		if imported.Name != nil {
			alias = imported.Name.Name
		}

		aliases[alias] = true
	}

	literals := make(map[token.Pos]bool)

	ast.Inspect(file, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall || len(call.Args) != 2 {
			return true
		}

		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector || selector.Sel.Name != "Join" {
			return true
		}

		packageName, isIdentifier := selector.X.(*ast.Ident)
		if !isIdentifier || packageName.Obj != nil || !aliases[packageName.Name] || wireCallHasLocalShadow(file, call, packageName.Name) {
			return true
		}

		separator, isString := call.Args[1].(*ast.BasicLit)
		if isString && separator.Kind == token.STRING {
			literals[separator.Pos()] = true
		}

		return true
	})

	return literals
}

//nolint:cyclop // Every lexical binding form that can shadow an import must remain covered.
func wireCallHasLocalShadow(file *ast.File, call *ast.CallExpr, name string) bool {
	shadowed := false

	ast.Inspect(file, func(node ast.Node) bool {
		var (
			function *ast.FuncType
			receiver *ast.FieldList
			body     *ast.BlockStmt
		)

		switch functionNode := node.(type) {
		case *ast.FuncDecl:
			function, receiver, body = functionNode.Type, functionNode.Recv, functionNode.Body
		case *ast.FuncLit:
			function, body = functionNode.Type, functionNode.Body
		default:
			return true
		}

		if body == nil || call.Pos() < body.Pos() || call.End() > body.End() {
			return true
		}

		if fieldListHasName(receiver, name) || fieldListHasName(function.Params, name) || fieldListHasName(function.Results, name) ||
			wireFunctionBodyBindsName(body, name) {
			shadowed = true

			return false
		}

		return true
	})

	return shadowed
}

func fieldListHasName(fields *ast.FieldList, name string) bool {
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

//nolint:cyclop // The AST visitor recognizes each statement form that can introduce a local shadow.
func wireFunctionBodyBindsName(body *ast.BlockStmt, name string) bool {
	bound := false

	ast.Inspect(body, func(node ast.Node) bool {
		if _, nestedFunction := node.(*ast.FuncLit); nestedFunction {
			return false
		}

		switch declaration := node.(type) {
		case *ast.ValueSpec:
			for _, identifier := range declaration.Names {
				bound = bound || identifier.Name == name
			}
		case *ast.TypeSpec:
			bound = bound || declaration.Name.Name == name
		case *ast.AssignStmt:
			if declaration.Tok == token.DEFINE {
				for _, left := range declaration.Lhs {
					identifier, isIdentifier := left.(*ast.Ident)
					bound = bound || (isIdentifier && identifier.Name == name)
				}
			}
		case *ast.RangeStmt:
			if declaration.Tok == token.DEFINE {
				key, isKey := declaration.Key.(*ast.Ident)
				value, isValue := declaration.Value.(*ast.Ident)
				bound = bound || (isKey && key.Name == name) || (isValue && value.Name == name)
			}
		}

		return !bound
	})

	return bound
}

func isGeneratedWireConstruction(expression ast.Expr, aliases map[string]bool, path string, models map[string]generatedModel) bool {
	name := ""

	switch expression := expression.(type) {
	case *ast.SelectorExpr:
		identifier, isIdentifier := expression.X.(*ast.Ident)
		if isIdentifier && identifier.Obj == nil && aliases[identifier.Name] {
			name = expression.Sel.Name
		}
	case *ast.Ident:
		if filepath.ToSlash(filepath.Dir(path)) == generatedModelDirectory {
			name = expression.Name
		}
	}

	model, exists := models[name]

	return exists && strings.HasPrefix(model.File, generatedModelDirectory+"/")
}

// Follow source-local aliases as well as direct literals. Parameters and
// generated constants have no local value declaration and remain caller inputs.
//
//nolint:cyclop // This recursive syntax walk tracks generated values, map keys, and source aliases together.
func inspectWireValueLiterals(expression ast.Expr, visiting map[wireSourceVariable]bool, report func(*ast.BasicLit), assignments wireSourceAssignments) {
	if expression == nil {
		return
	}

	ast.Inspect(expression, func(node ast.Node) bool {
		if call, callable := node.(*ast.CallExpr); callable {
			if inspectWireReturnedLiterals(call, visiting, report, assignments) {
				return false
			}
		}

		if literal, isComposite := node.(*ast.CompositeLit); isComposite {
			if inspectWireMapLiterals(literal, visiting, report, assignments) {
				return false
			}
		}

		if field, isField := node.(*ast.KeyValueExpr); isField {
			if key, isLiteral := field.Key.(*ast.BasicLit); isLiteral && key.Kind == token.STRING {
				report(key)
			}

			inspectWireValueLiterals(field.Value, visiting, report, assignments)

			return false
		}

		if literal, isLiteral := node.(*ast.BasicLit); isLiteral && literal.Kind == token.STRING {
			report(literal)
		}

		if identifier, isIdentifier := node.(*ast.Ident); isIdentifier {
			inspectWireAlias(identifier, visiting, report, assignments)
		}

		return true
	})
}

func inspectWireMapLiterals(
	literal *ast.CompositeLit, visiting map[wireSourceVariable]bool, report func(*ast.BasicLit), assignments wireSourceAssignments,
) bool {
	if _, isMap := literal.Type.(*ast.MapType); !isMap {
		return false
	}

	for _, element := range literal.Elts {
		if entry, isEntry := element.(*ast.KeyValueExpr); isEntry {
			inspectWireValueLiterals(entry.Key, visiting, report, assignments)
			inspectWireValueLiterals(entry.Value, visiting, report, assignments)
		}
	}

	return true
}

func inspectWireAlias(identifier *ast.Ident, visiting map[wireSourceVariable]bool, report func(*ast.BasicLit), assignments wireSourceAssignments) {
	if identifier.Obj == nil {
		return
	}

	declaration, isNode := identifier.Obj.Decl.(ast.Node)

	key := wireSourceVariable{declaration: declaration, name: identifier.Name}

	if !isNode || visiting[key] {
		return
	}

	visiting[key] = true

	if value := wireAliasValue(identifier.Name, declaration); value != nil {
		inspectWireValueLiterals(value, visiting, report, assignments)
	}

	for _, value := range assignments.values[key] {
		_, parameter := declaration.(*ast.Field)
		if parameter || assignments.globalVariables[key] || value.Pos() < identifier.Pos() {
			inspectWireValueLiterals(value, visiting, report, assignments)
		}
	}
}

func wireAliasValue(name string, declaration ast.Node) ast.Expr {
	switch declaration := declaration.(type) {
	case *ast.ValueSpec:
		for index, identifier := range declaration.Names {
			if identifier.Name == name && index < len(declaration.Values) {
				return declaration.Values[index]
			}
		}
	case *ast.AssignStmt:
		for index, left := range declaration.Lhs {
			identifier, isIdentifier := left.(*ast.Ident)
			if isIdentifier && identifier.Name == name && index < len(declaration.Rhs) {
				return declaration.Rhs[index]
			}
		}
	}

	return nil
}

//nolint:cyclop // Receiver provenance must cover every Go expression form that can preserve an SDK value.
func generatedWireReceiver(
	expression ast.Expr, aliases map[string]bool, path string, models map[string]generatedModel,
	visiting map[wireSourceVariable]bool, assignments wireSourceAssignments,
) bool {
	switch expression := expression.(type) {
	case *ast.SelectorExpr:
		return generatedWireReceiver(expression.X, aliases, path, models, visiting, assignments)
	case *ast.UnaryExpr:
		return generatedWireReceiver(expression.X, aliases, path, models, visiting, assignments)
	case *ast.StarExpr:
		return generatedWireReceiver(expression.X, aliases, path, models, visiting, assignments)
	case *ast.IndexExpr:
		return generatedWireReceiver(expression.X, aliases, path, models, visiting, assignments)
	case *ast.SliceExpr:
		return generatedWireReceiver(expression.X, aliases, path, models, visiting, assignments)
	case *ast.TypeAssertExpr:
		return generatedWireReceiver(expression.X, aliases, path, models, visiting, assignments)
	case *ast.ParenExpr:
		return generatedWireReceiver(expression.X, aliases, path, models, visiting, assignments)
	case *ast.CompositeLit:
		return generatedWireAggregate(expression, aliases, path, models, visiting, assignments)
	case *ast.FuncLit:
		return generatedWireReturnedValue(expression.Type, expression.Body, aliases, path, models, visiting, assignments)
	case *ast.Ident:
		return generatedWireIdentifier(expression, aliases, path, models, visiting, assignments)
	case *ast.CallExpr:
		if len(expression.Args) == 1 && wireTypeConversion(expression.Fun) {
			return generatedWireReceiver(expression.Args[0], aliases, path, models, visiting, assignments)
		}

		return generatedWireCallResult(expression, aliases, path, models, visiting, assignments)
	}

	return false
}

//nolint:cyclop // Identifier provenance follows declarations, assignments, generated types, and parameters.
func generatedWireIdentifier(
	identifier *ast.Ident, aliases map[string]bool, path string, models map[string]generatedModel,
	visiting map[wireSourceVariable]bool, assignments wireSourceAssignments,
) bool {
	if identifier.Obj == nil {
		return false
	}

	declaration, isNode := identifier.Obj.Decl.(ast.Node)

	key := wireSourceVariable{declaration: declaration, name: identifier.Name}

	if !isNode || visiting[key] {
		return false
	}

	visiting[key] = true

	if assignments.generatedParameters[key] {
		return true
	}

	switch declaration := declaration.(type) {
	case *ast.Field:
		if generatedWireDeclaredType(declaration.Type, aliases, path, models) {
			return true
		}
	case *ast.ValueSpec:
		if generatedWireDeclaredType(declaration.Type, aliases, path, models) {
			return true
		}
	}

	if value := wireAliasValue(identifier.Name, declaration); value != nil &&
		generatedWireReceiver(value, aliases, path, models, visiting, assignments) {
		return true
	}

	for _, value := range wireAliasExpressions(identifier, assignments) {
		if generatedWireReceiver(value, aliases, path, models, visiting, assignments) {
			return true
		}
	}

	return false
}

func generatedWireDeclaredType(expression ast.Expr, aliases map[string]bool, path string, models map[string]generatedModel) bool {
	switch expression := expression.(type) {
	case *ast.StarExpr:
		return generatedWireDeclaredType(expression.X, aliases, path, models)
	case *ast.ArrayType:
		return generatedWireDeclaredType(expression.Elt, aliases, path, models)
	case *ast.MapType:
		return generatedWireDeclaredType(expression.Value, aliases, path, models)
	case *ast.ParenExpr:
		return generatedWireDeclaredType(expression.X, aliases, path, models)
	}

	return isGeneratedWireConstruction(expression, aliases, path, models)
}

func wireAssignmentParts(expression ast.Expr) (ast.Expr, ast.Expr) {
	switch expression := expression.(type) {
	case *ast.SelectorExpr:
		return expression.X, nil
	case *ast.IndexExpr:
		return expression.X, expression.Index
	}

	return nil, nil
}

func inspectWireMutationAssignment(
	assignment *ast.AssignStmt, file *ast.File, aliases map[string]bool, path string, models map[string]generatedModel,
	assignments wireSourceAssignments, report func(*ast.BasicLit),
) {
	for index, left := range assignment.Lhs {
		receiver, key := wireAssignmentParts(left)
		if receiver == nil || key == nil || index >= len(assignment.Rhs) {
			continue
		}

		if wireURLValuesReceiver(receiver, file, assignments, make(map[wireSourceVariable]bool)) {
			inspectWireValueLiterals(key, make(map[wireSourceVariable]bool), report, assignments)

			continue
		}

		if !generatedWireReceiver(receiver, aliases, path, models, make(map[wireSourceVariable]bool), assignments) {
			continue
		}

		inspectWireReceiverKeys(receiver, make(map[wireSourceVariable]bool), report, assignments)
		inspectWireValueLiterals(key, make(map[wireSourceVariable]bool), report, assignments)
		inspectWireValueLiterals(assignment.Rhs[index], make(map[wireSourceVariable]bool), report, assignments)
	}
}
