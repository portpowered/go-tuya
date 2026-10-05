package main

import (
	"go/ast"
	"maps"
)

// Resolve the lexical type declaration rather than trusting a method spelling.
//
//nolint:cyclop,funlen,gocognit // Receiver types may be nested through fields, pointers, aliases, or literals.
func wireReceiverDeclaration(expression ast.Expr, assignments wireSourceAssignments, visiting map[wireSourceVariable]bool) *ast.TypeSpec {
	if expression == nil {
		return nil
	}

	key := wireSourceVariable{declaration: expression, name: "receiver expression"}
	if visiting[key] {
		return nil
	}

	visiting[key] = true
	defer delete(visiting, key)

	switch expression := expression.(type) {
	case *ast.ParenExpr:
		return wireReceiverDeclaration(expression.X, assignments, visiting)
	case *ast.StarExpr:
		return wireReceiverDeclaration(expression.X, assignments, visiting)
	case *ast.UnaryExpr:
		return wireReceiverDeclaration(expression.X, assignments, visiting)
	case *ast.IndexExpr:
		return wireReceiverDeclaration(expression.X, assignments, visiting)
	case *ast.IndexListExpr:
		return wireReceiverDeclaration(expression.X, assignments, visiting)
	case *ast.CompositeLit:
		return wireReceiverDeclaration(expression.Type, assignments, visiting)
	case *ast.CallExpr:
		var owner *ast.TypeSpec

		for _, results := range wireCallResultLists(expression.Fun, assignments) {
			if results == nil || len(results.List) == 0 {
				continue
			}

			candidate := wireReceiverDeclaration(results.List[0].Type, assignments, maps.Clone(visiting))
			if candidate == nil || (owner != nil && candidate != owner) {
				return nil
			}

			owner = candidate
		}

		return owner
	case *ast.SelectorExpr:
		owner := wireReceiverDeclaration(expression.X, assignments, visiting)
		if owner == nil {
			return nil
		}

		structure := wireReceiverStructure(owner, make(map[*ast.TypeSpec]bool))
		if structure == nil || structure.Fields == nil {
			return nil
		}

		for _, field := range structure.Fields.List {
			for _, name := range field.Names {
				if name.Name == expression.Sel.Name {
					return wireReceiverDeclaration(field.Type, assignments, visiting)
				}
			}
		}

		return nil
	case *ast.MapType:
		return wireReceiverDeclaration(expression.Value, assignments, visiting)
	case *ast.TypeAssertExpr:
		return wireReceiverDeclaration(expression.Type, assignments, visiting)
	case *ast.Ident:
		return wireReceiverIdentifier(expression, assignments, visiting)
	}

	return nil
}

//nolint:cyclop // Calls can be local functions, generic functions, literals, or resolved receiver methods.
func wireCallResultLists(expression ast.Expr, assignments wireSourceAssignments) []*ast.FieldList {
	switch expression := expression.(type) {
	case *ast.ParenExpr:
		return wireCallResultLists(expression.X, assignments)
	case *ast.IndexExpr:
		return wireCallResultLists(expression.X, assignments)
	case *ast.IndexListExpr:
		return wireCallResultLists(expression.X, assignments)
	case *ast.FuncLit:
		return []*ast.FieldList{expression.Type.Results}
	case *ast.Ident:
		if expression.Obj == nil {
			return nil
		}

		if function, isFunction := expression.Obj.Decl.(*ast.FuncDecl); isFunction {
			return []*ast.FieldList{function.Type.Results}
		}

		if declaration, isVariable := expression.Obj.Decl.(*ast.ValueSpec); isVariable && declaration.Type != nil {
			if function, isFunction := declaration.Type.(*ast.FuncType); isFunction {
				return []*ast.FieldList{function.Results}
			}
		}
	case *ast.SelectorExpr:
		owner := wireReceiverDeclaration(expression.X, assignments, make(map[wireSourceVariable]bool))
		if owner == nil {
			return nil
		}

		results := make([]*ast.FieldList, 0)

		for _, declaration := range assignments.methods[expression.Sel.Name] {
			if declaration.Recv != nil && len(declaration.Recv.List) != 0 &&
				wireReceiverDeclaration(declaration.Recv.List[0].Type, assignments, make(map[wireSourceVariable]bool)) == owner {
				results = append(results, declaration.Type.Results)
			}
		}

		return results
	}

	return nil
}

func wireReceiverStructure(declaration *ast.TypeSpec, visiting map[*ast.TypeSpec]bool) *ast.StructType {
	if declaration == nil || visiting[declaration] {
		return nil
	}

	visiting[declaration] = true

	return wireReceiverTypeStructure(declaration.Type, visiting)
}

func wireReceiverTypeStructure(expression ast.Expr, visiting map[*ast.TypeSpec]bool) *ast.StructType {
	switch expression := expression.(type) {
	case *ast.StructType:
		return expression
	case *ast.Ident:
		if expression.Obj == nil {
			return nil
		}

		nested, isType := expression.Obj.Decl.(*ast.TypeSpec)
		if !isType {
			return nil
		}

		return wireReceiverStructure(nested, visiting)
	case *ast.StarExpr:
		return wireReceiverTypeStructure(expression.X, visiting)
	case *ast.ParenExpr:
		return wireReceiverTypeStructure(expression.X, visiting)
	}

	return nil
}

func wireReceiverFieldDeclared(selector *ast.SelectorExpr, assignments wireSourceAssignments) bool {
	owner := wireReceiverDeclaration(selector.X, assignments, make(map[wireSourceVariable]bool))

	structure := wireReceiverStructure(owner, make(map[*ast.TypeSpec]bool))
	if structure == nil || structure.Fields == nil {
		return false
	}

	for _, field := range structure.Fields.List {
		if len(field.Names) == 0 {
			if wireEmbeddedFieldName(field.Type) == selector.Sel.Name {
				return true
			}

			continue
		}

		for _, name := range field.Names {
			if name.Name == selector.Sel.Name {
				return true
			}
		}
	}

	return false
}

func wireEmbeddedFieldName(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.SelectorExpr:
		return expression.Sel.Name
	case *ast.StarExpr:
		return wireEmbeddedFieldName(expression.X)
	case *ast.IndexExpr:
		return wireEmbeddedFieldName(expression.X)
	case *ast.IndexListExpr:
		return wireEmbeddedFieldName(expression.X)
	case *ast.ParenExpr:
		return wireEmbeddedFieldName(expression.X)
	}

	return ""
}

//nolint:cyclop // Receiver identity follows local type aliases and helper provenance with cycle guards.
func wireReceiverIdentifier(identifier *ast.Ident, assignments wireSourceAssignments, visiting map[wireSourceVariable]bool) *ast.TypeSpec {
	if identifier.Obj == nil {
		return nil
	}

	if declaration, named := identifier.Obj.Decl.(*ast.TypeSpec); named {
		return declaration
	}

	declaration, valid := identifier.Obj.Decl.(ast.Node)

	key := wireSourceVariable{declaration: declaration, name: identifier.Name}

	if !valid || visiting[key] {
		return nil
	}

	visiting[key] = true

	switch declaration := declaration.(type) {
	case *ast.Field:
		return wireReceiverDeclaration(declaration.Type, assignments, visiting)
	case *ast.ValueSpec:
		if declaration.Type != nil {
			return wireReceiverDeclaration(declaration.Type, assignments, visiting)
		}
	}

	var owner *ast.TypeSpec

	for _, value := range wireAliasExpressions(identifier, assignments) {
		candidate := wireReceiverDeclaration(value, assignments, maps.Clone(visiting))
		if candidate == nil || (owner != nil && candidate != owner) {
			return nil
		}

		owner = candidate
	}

	return owner
}

func wireCallableFieldDeclared(selector *ast.SelectorExpr, assignments wireSourceAssignments) bool {
	owner := wireReceiverDeclaration(selector.X, assignments, make(map[wireSourceVariable]bool))
	if owner == nil {
		return false
	}

	object := wireReceiverStructure(owner, make(map[*ast.TypeSpec]bool))
	if object == nil {
		return false
	}

	for _, field := range object.Fields.List {
		if _, callable := field.Type.(*ast.FuncType); !callable {
			continue
		}

		for _, name := range field.Names {
			if name.Name == selector.Sel.Name {
				return true
			}
		}
	}

	return false
}
