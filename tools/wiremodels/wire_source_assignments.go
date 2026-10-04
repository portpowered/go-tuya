package main

import "go/ast"

type wireSourceVariable struct {
	declaration ast.Node
	name        string
}

type wireSourceAssignments struct {
	values              map[wireSourceVariable][]ast.Expr
	generatedParameters map[wireSourceVariable]bool
	globalVariables     map[wireSourceVariable]bool
	publicInterfaces    map[string]bool
	urlValuesVariables  map[wireSourceVariable]bool
	urlValuesFunctions  map[*ast.FuncType]bool
	callableFields      map[string][]ast.Expr
	functionBodies      map[*ast.FuncType]*ast.BlockStmt
	methods             map[string][]*ast.FuncDecl
}

func indexWireSourceAssignments(file *ast.File) wireSourceAssignments {
	assignments := newWireSourceAssignments()
	indexWirePackageScope(file, assignments)
	indexWireMethods(file, assignments)
	ast.Inspect(file, func(node ast.Node) bool {
		indexWireSourceNode(file, node, assignments)

		return true
	})

	return assignments
}

func newWireSourceAssignments() wireSourceAssignments {
	return wireSourceAssignments{
		values:              make(map[wireSourceVariable][]ast.Expr),
		generatedParameters: make(map[wireSourceVariable]bool),
		globalVariables:     make(map[wireSourceVariable]bool),
		publicInterfaces:    make(map[string]bool),
		urlValuesVariables:  make(map[wireSourceVariable]bool),
		urlValuesFunctions:  make(map[*ast.FuncType]bool),
		callableFields:      make(map[string][]ast.Expr),
		functionBodies:      make(map[*ast.FuncType]*ast.BlockStmt),
		methods:             make(map[string][]*ast.FuncDecl),
	}
}

func indexWirePackageScope(file *ast.File, assignments wireSourceAssignments) {
	for name, object := range file.Scope.Objects {
		if declaration, variable := object.Decl.(*ast.ValueSpec); variable && object.Kind == ast.Var {
			assignments.globalVariables[wireSourceVariable{declaration: declaration, name: name}] = true
		}

		if declaration, typeName := object.Decl.(*ast.TypeSpec); typeName {
			if _, interfaceType := declaration.Type.(*ast.InterfaceType); interfaceType {
				assignments.publicInterfaces[name] = true
			}
		}
	}
}

func indexWireMethods(file *ast.File, assignments wireSourceAssignments) {
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok && function.Recv != nil {
			assignments.methods[function.Name.Name] = append(assignments.methods[function.Name.Name], function)
		}
	}
}

func indexWireSourceNode(file *ast.File, node ast.Node, assignments wireSourceAssignments) {
	indexWireSourceDeclaration(file, node, assignments)
	indexWireCallableField(node, assignments)
	indexWireAssignmentValues(node, assignments)
}

func indexWireSourceDeclaration(file *ast.File, node ast.Node, assignments wireSourceAssignments) {
	switch function := node.(type) {
	case *ast.FuncDecl:
		assignments.functionBodies[function.Type] = function.Body
		indexURLValuesSignature(file, function.Type, function.Recv, assignments)
	case *ast.FuncLit:
		assignments.functionBodies[function.Type] = function.Body
		indexURLValuesSignature(file, function.Type, nil, assignments)
	case *ast.ValueSpec:
		if wireURLValuesDeclaredType(function.Type, file, make(map[wireSourceVariable]bool)) {
			for _, name := range function.Names {
				assignments.urlValuesVariables[wireSourceVariable{declaration: function, name: name.Name}] = true
			}
		}
	}
}

func indexWireCallableField(node ast.Node, assignments wireSourceAssignments) {
	entry, isField := node.(*ast.KeyValueExpr)
	if !isField {
		return
	}

	name, isName := entry.Key.(*ast.Ident)
	if !isName {
		return
	}

	assignments.callableFields[name.Name] = append(assignments.callableFields[name.Name], entry.Value)
}

func indexWireAssignmentValues(node ast.Node, assignments wireSourceAssignments) {
	assignment, isAssignment := node.(*ast.AssignStmt)
	if !isAssignment {
		return
	}

	for index, left := range assignment.Lhs {
		identifier, isIdentifier := left.(*ast.Ident)
		if !isIdentifier || identifier.Obj == nil || index >= len(assignment.Rhs) {
			continue
		}

		declaration, isNode := identifier.Obj.Decl.(ast.Node)
		if !isNode {
			continue
		}

		key := wireSourceVariable{declaration: declaration, name: identifier.Name}
		assignments.values[key] = append(assignments.values[key], assignment.Rhs[index])
	}
}

func indexURLValuesSignature(file *ast.File, function *ast.FuncType, receiver *ast.FieldList, assignments wireSourceAssignments) {
	if hasURLValuesResult(file, function) {
		assignments.urlValuesFunctions[function] = true
	}

	for _, fields := range []*ast.FieldList{receiver, function.Params, function.Results} {
		if fields == nil {
			continue
		}

		for _, field := range fields.List {
			if !wireURLValuesDeclaredType(field.Type, file, make(map[wireSourceVariable]bool)) {
				continue
			}

			for _, name := range field.Names {
				assignments.urlValuesVariables[wireSourceVariable{declaration: field, name: name.Name}] = true
			}
		}
	}
}

func hasURLValuesResult(file *ast.File, function *ast.FuncType) bool {
	if function.Results == nil || len(function.Results.List) == 0 {
		return false
	}

	resultType := function.Results.List[0].Type

	return wireURLValuesDeclaredType(resultType, file, make(map[wireSourceVariable]bool))
}

func isURLValuesType(expression ast.Expr, aliases map[string]bool) bool {
	selector, isSelector := expression.(*ast.SelectorExpr)
	if !isSelector || selector.Sel.Name != "Values" {
		return false
	}

	identifier, isIdentifier := selector.X.(*ast.Ident)

	return isIdentifier && identifier.Obj == nil && aliases[identifier.Name]
}

func wireAliasExpressions(identifier *ast.Ident, assignments wireSourceAssignments) []ast.Expr {
	if identifier.Obj == nil {
		return nil
	}

	declaration, isNode := identifier.Obj.Decl.(ast.Node)
	if !isNode {
		return nil
	}

	var values []ast.Expr

	if value := wireAliasValue(identifier.Name, declaration); value != nil {
		values = append(values, value)
	}

	key := wireSourceVariable{declaration: declaration, name: identifier.Name}
	for _, value := range assignments.values[key] {
		_, parameter := declaration.(*ast.Field)
		if parameter || assignments.globalVariables[key] || value.Pos() < identifier.Pos() {
			values = append(values, value)
		}
	}

	return values
}
