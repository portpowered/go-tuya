package main

import (
	"go/ast"
	"go/token"
)

const (
	urlValuesSetArgumentCount = 2
	wireAddMethod             = "Add"
)

//nolint:cyclop // Query receiver provenance follows aliases, aggregate fields, and constructors.
func wireURLValuesReceiver(
	expression ast.Expr, file *ast.File, assignments wireSourceAssignments, visiting map[wireSourceVariable]bool,
) bool {
	switch expression := expression.(type) {
	case *ast.ParenExpr:
		return wireURLValuesReceiver(expression.X, file, assignments, visiting)
	case *ast.SelectorExpr:
		owner := wireReceiverDeclaration(expression.X, assignments, make(map[wireSourceVariable]bool))
		if owner != nil && assignments.urlValuesFields[wireSourceField{declaration: owner, name: expression.Sel.Name}] {
			return true
		}

		for _, value := range wireAggregateFieldExpressions(expression, assignments) {
			if wireURLValuesReceiver(value, file, assignments, visiting) {
				return true
			}
		}

		return false
	case *ast.StarExpr:
		return wireURLValuesReceiver(expression.X, file, assignments, visiting)
	case *ast.TypeAssertExpr:
		return wireURLValuesReceiver(expression.X, file, assignments, visiting)
	case *ast.CompositeLit:
		return wireURLValuesDeclaredType(expression.Type, file, make(map[wireSourceVariable]bool))
	case *ast.CallExpr:
		return wireURLValuesCallReceiver(expression, file, assignments)
	case *ast.Ident:
		return wireURLValuesIdentifierReceiver(expression, file, assignments, visiting)
	}

	return false
}

func wireURLValuesCallReceiver(call *ast.CallExpr, file *ast.File, assignments wireSourceAssignments) bool {
	aliases := primitiveImportAliases(file, primitivePackage{ImportPath: "net/url", Name: "url", Directory: ""})
	if isURLValuesType(call.Fun, aliases) {
		return true
	}

	if builtin, isBuiltin := call.Fun.(*ast.Ident); isBuiltin && builtin.Obj == nil && builtin.Name == "make" &&
		len(call.Args) != 0 && wireURLValuesDeclaredType(call.Args[0], file, make(map[wireSourceVariable]bool)) {
		return true
	}

	for _, function := range wireLocalHelpers(call.Fun, assignments, make(map[wireSourceVariable]bool)) {
		if function != nil && assignments.urlValuesFunctions[function] {
			return true
		}
	}

	return false
}

//nolint:cyclop // URL mutations are accepted only when a direct schema guard dominates the call in its range.
func verifiedWireURLValuesMutation(file *ast.File, call *ast.CallExpr, assignments wireSourceAssignments) bool {
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector || (selector.Sel.Name != "Set" && selector.Sel.Name != wireAddMethod) ||
		len(call.Args) != urlValuesSetArgumentCount ||
		!wireURLValuesReceiver(selector.X, file, assignments, make(map[wireSourceVariable]bool)) {
		return false
	}

	key, isIdentifier := call.Args[0].(*ast.Ident)
	if !isIdentifier || key.Obj == nil {
		return false
	}

	aliases := primitiveImportAliases(file, primitivePackage{ImportPath: wireImportPath, Name: generatedModelPackageName, Directory: generatedModelDirectory})
	verified := false

	ast.Inspect(file, func(node ast.Node) bool {
		if verified {
			return false
		}

		rangeStatement, isRange := node.(*ast.RangeStmt)
		if !isRange || call.Pos() < rangeStatement.Body.Pos() || call.End() > rangeStatement.Body.End() ||
			!wireRangeDeclaresIdentifier(rangeStatement, key) {
			return true
		}

		for _, statement := range rangeStatement.Body.List {
			guard, isGuard := statement.(*ast.IfStmt)
			if !isGuard || guard.End() >= call.Pos() || !wireRejectsUnknownQueryKey(guard, key, aliases) {
				continue
			}

			verified = true

			return false
		}

		return true
	})

	return verified
}

func wireRangeDeclaresIdentifier(statement *ast.RangeStmt, identifier *ast.Ident) bool {
	for _, expression := range []ast.Expr{statement.Key, statement.Value} {
		candidate, isIdentifier := expression.(*ast.Ident)
		if isIdentifier && candidate.Obj == identifier.Obj {
			return true
		}
	}

	return false
}

//nolint:cyclop // This accepts only a direct known-key guard whose body exits the current iteration.
func wireRejectsUnknownQueryKey(statement *ast.IfStmt, key *ast.Ident, aliases map[string]bool) bool {
	negation, isNegation := statement.Cond.(*ast.UnaryExpr)
	if !isNegation || negation.Op != token.NOT {
		return false
	}

	call, isCall := negation.X.(*ast.CallExpr)
	if !isCall || len(call.Args) != 1 {
		return false
	}

	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector || selector.Sel.Name != "IsKnownQueryParam" {
		return false
	}

	packageName, isPackage := selector.X.(*ast.Ident)
	argument, isArgument := call.Args[0].(*ast.Ident)

	if !isPackage || packageName.Obj != nil || !aliases[packageName.Name] || !isArgument || argument.Obj != key.Obj {
		return false
	}

	for _, statement := range statement.Body.List {
		switch statement := statement.(type) {
		case *ast.ReturnStmt:
			return true
		case *ast.BranchStmt:
			if statement.Tok == token.CONTINUE {
				return true
			}
		}
	}

	return false
}

func wireURLValuesIdentifierReceiver(
	identifier *ast.Ident, file *ast.File, assignments wireSourceAssignments, visiting map[wireSourceVariable]bool,
) bool {
	if identifier.Obj == nil {
		return false
	}

	declaration, isNode := identifier.Obj.Decl.(ast.Node)

	key := wireSourceVariable{declaration: declaration, name: identifier.Name}
	if !isNode || visiting[key] {
		return false
	}

	if assignments.urlValuesVariables[key] || wireURLValuesDeclaredType(wireSourceDeclaredType(declaration), file, visiting) {
		return true
	}

	visiting[key] = true

	return wireURLValuesIdentifierAliases(identifier, declaration, file, assignments, visiting)
}

func wireURLValuesIdentifierAliases(
	identifier *ast.Ident, declaration ast.Node, file *ast.File, assignments wireSourceAssignments,
	visiting map[wireSourceVariable]bool,
) bool {
	if typeSpec, isType := declaration.(*ast.TypeSpec); isType && wireURLValuesReceiver(typeSpec.Type, file, assignments, visiting) {
		return true
	}

	if value := wireAliasValue(identifier.Name, declaration); value != nil && wireURLValuesReceiver(value, file, assignments, visiting) {
		return true
	}

	for _, value := range wireAliasExpressions(identifier, assignments) {
		if wireURLValuesReceiver(value, file, assignments, visiting) {
			return true
		}
	}

	return false
}

func inspectWireURLValuesMutation(file *ast.File, call *ast.CallExpr, assignments wireSourceAssignments, report func(*ast.BasicLit)) {
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector || !wireURLValuesReceiver(selector.X, file, assignments, make(map[wireSourceVariable]bool)) {
		return
	}

	var key ast.Expr

	switch selector.Sel.Name {
	case "Set", "Add":
		if len(call.Args) != urlValuesSetArgumentCount {
			return
		}

		key = call.Args[0]
	case "Del":
		if len(call.Args) != 1 {
			return
		}

		key = call.Args[0]
	default:
		return
	}

	inspectWireValueLiterals(key, make(map[wireSourceVariable]bool), report, assignments)
}

func inspectWireURLValuesLiteral(literal *ast.CompositeLit, file *ast.File, assignments wireSourceAssignments, report func(*ast.BasicLit)) {
	if !wireURLValuesDeclaredType(literal.Type, file, make(map[wireSourceVariable]bool)) {
		return
	}

	for _, element := range literal.Elts {
		if entry, isEntry := element.(*ast.KeyValueExpr); isEntry {
			inspectWireValueLiterals(entry.Key, make(map[wireSourceVariable]bool), report, assignments)
		}
	}
}

func wireURLValuesDeclaredType(expression ast.Expr, file *ast.File, visiting map[wireSourceVariable]bool) bool {
	if isURLValuesType(expression, primitiveImportAliases(file, primitivePackage{ImportPath: "net/url", Name: "url", Directory: ""})) {
		return true
	}

	identifier, isIdentifier := expression.(*ast.Ident)
	if !isIdentifier || identifier.Obj == nil {
		return false
	}

	declaration, isNode := identifier.Obj.Decl.(ast.Node)

	key := wireSourceVariable{declaration: declaration, name: identifier.Name}

	if !isNode || visiting[key] {
		return false
	}

	typeSpec, isType := declaration.(*ast.TypeSpec)
	if !isType {
		return false
	}

	visiting[key] = true

	return wireURLValuesDeclaredType(typeSpec.Type, file, visiting)
}

func wireSourceDeclaredType(declaration ast.Node) ast.Expr {
	switch declaration := declaration.(type) {
	case *ast.Field:
		return declaration.Type
	case *ast.ValueSpec:
		return declaration.Type
	case *ast.TypeSpec:
		return declaration.Type
	default:
		return nil
	}
}
