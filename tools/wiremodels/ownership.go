package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var errModelOwnership = errors.New("schema ownership is incomplete or ambiguous")

const (
	jsonMediaTypeSchema = "JSONMediaType"
	jsonMediaTypeRef    = "#/components/schemas/JSONMediaType"
	jsonMediaTypeValue  = "application/json"
	stringSchemaType    = "string"
)

var groupName = regexp.MustCompile(`^[a-z][a-z_]*$`)

func validateModelOwnership(manifest modelManifest, bundle map[string]any) error {
	groups := make(map[string]bool)
	owners := make(map[string]string)

	for _, group := range manifest.Groups {
		if !groupName.MatchString(group.Name) || groups[group.Name] {
			return fmt.Errorf("model group %q: %w", group.Name, errModelOwnership)
		}

		groups[group.Name] = true

		if group.Source != "api/models/"+group.Name+".yaml" {
			return fmt.Errorf("model source %q: %w", group.Source, errModelOwnership)
		}

		for _, operation := range group.Operations {
			if owners[operation] != "" {
				return fmt.Errorf("operation %q has multiple owners: %w", operation, errModelOwnership)
			}

			owners[operation] = group.Name
		}
	}

	return validateOperationOwners(bundle, owners)
}

func validateOperationOwners(bundle map[string]any, owners map[string]string) error {
	paths, ok := bundle["paths"].(map[string]any)
	if !ok || len(paths) == 0 {
		return fmt.Errorf("HTTP paths are absent: %w", errModelOwnership)
	}

	seen := make(map[string]bool)

	for path, value := range paths {
		operations, valid := value.(map[string]any)
		if !valid {
			return fmt.Errorf("path %q is invalid: %w", path, errModelOwnership)
		}

		for method, value := range operations {
			if !isHTTPOperation(method) {
				continue
			}

			err := validateOwnedOperation(method, path, value, owners, seen)
			if err != nil {
				return err
			}
		}
	}

	err := validateRemainingOwners(owners, seen)
	if err != nil {
		return err
	}

	err = validateHTTPContentTypeBindings(bundle)
	if err != nil {
		return err
	}

	return validateBundleReferences(bundle)
}

func isHTTPOperation(method string) bool {
	switch method {
	case "get", "post", "put", "patch", "delete", "head", "options", "trace":
		return true
	default:
		return false
	}
}

func validateBundleReferences(bundle map[string]any) error {
	return validateComponentReferences(bundle, bundle)
}

func validateHTTPContentTypeBindings(bundle map[string]any) error {
	schemas, err := componentSchemas(bundle)
	if err != nil {
		return err
	}

	err = validateJSONMediaTypeSchema(schemas)
	if err != nil {
		return err
	}

	paths, validPaths := bundle["paths"].(map[string]any)
	if !validPaths {
		return fmt.Errorf("HTTP paths are absent: %w", errModelOwnership)
	}

	bindings := 0

	for path, rawPath := range paths {
		count, err := contentTypeBindingsAtPath(path, rawPath)
		if err != nil {
			return err
		}

		bindings += count
	}

	if bindings == 0 {
		return fmt.Errorf("no HTTP Content-Type parameter is bound to JSONMediaType: %w", errModelOwnership)
	}

	return nil
}

func validateJSONMediaTypeSchema(schemas map[string]any) error {
	mediaType, validMediaType := schemas[jsonMediaTypeSchema].(map[string]any)
	if !validMediaType {
		return fmt.Errorf("%s schema is absent: %w", jsonMediaTypeSchema, errModelOwnership)
	}

	values, validValues := mediaType["enum"].([]any)
	if mediaType["type"] != stringSchemaType || !validValues || len(values) != 1 || values[0] != jsonMediaTypeValue {
		return fmt.Errorf("%s must define the generated %s value: %w", jsonMediaTypeSchema, jsonMediaTypeValue, errModelOwnership)
	}

	return nil
}

func contentTypeBindingsAtPath(path string, rawPath any) (int, error) {
	operations, validOperations := rawPath.(map[string]any)
	if !validOperations {
		return 0, nil
	}

	bindings := 0

	for _, rawOperation := range operations {
		operation, validOperation := rawOperation.(map[string]any)
		if !validOperation {
			continue
		}

		count, err := contentTypeBindingsAtOperation(path, operation)
		if err != nil {
			return 0, err
		}

		bindings += count
	}

	return bindings, nil
}

func contentTypeBindingsAtOperation(path string, operation map[string]any) (int, error) {
	parameters, _ := operation["parameters"].([]any)
	bindings := 0

	for _, rawParameter := range parameters {
		parameter, validParameter := rawParameter.(map[string]any)
		if !validParameter || parameter["name"] != "Content-Type" {
			continue
		}

		err := validateContentTypeParameter(path, parameter)
		if err != nil {
			return 0, err
		}

		bindings++
	}

	return bindings, nil
}

func validateContentTypeParameter(path string, parameter map[string]any) error {
	schema, validSchema := parameter["schema"].(map[string]any)
	if parameter["in"] != "header" || !validSchema || schema["$ref"] != jsonMediaTypeRef {
		return fmt.Errorf("Content-Type parameter at %s is not bound to %s: %w", path, jsonMediaTypeSchema, errModelOwnership)
	}

	return nil
}

func validateComponentReferences(value any, bundle map[string]any) error {
	switch node := value.(type) {
	case map[string]any:
		return validateMapComponentReferences(node, bundle)
	case []any:
		return validateSliceComponentReferences(node, bundle)
	}

	return nil
}

func validateMapComponentReferences(node map[string]any, bundle map[string]any) error {
	for key, child := range node {
		err := validateComponentMetadata(key, child, node, bundle)
		if err != nil {
			return err
		}

		err = validateComponentReferences(child, bundle)
		if err != nil {
			return err
		}
	}

	return nil
}

func validateSliceComponentReferences(nodes []any, bundle map[string]any) error {
	for _, child := range nodes {
		err := validateComponentReferences(child, bundle)
		if err != nil {
			return err
		}
	}

	return nil
}

func validateComponentMetadata(key string, child any, source, bundle map[string]any) error {
	switch key {
	case "$ref":
		reference, isReference := child.(string)
		if !isReference {
			return nil
		}

		return validateSchemaReference(reference, bundle)
	case "x-go-tuya-known-values-schema":
		reference, isReference := child.(string)
		if !isReference {
			return fmt.Errorf("known-value binding %v is invalid: %w", child, errModelOwnership)
		}

		return validateKnownValueBinding(reference, source, bundle)
	default:
		return nil
	}
}

func validateKnownValueBinding(name string, source map[string]any, bundle map[string]any) error {
	if source["type"] != stringSchemaType {
		return fmt.Errorf("known-value source is not an open string: %w", errModelOwnership)
	}

	if _, closed := source["enum"]; closed {
		return fmt.Errorf("known-value source must stay open to future strings: %w", errModelOwnership)
	}

	schemas, err := componentSchemas(bundle)
	if err != nil {
		return fmt.Errorf("known-value components are absent: %w", errModelOwnership)
	}

	target, validTarget := schemas[name].(map[string]any)
	if !validTarget || target["type"] != stringSchemaType {
		return fmt.Errorf("known-value enum %q is missing or not a string: %w", name, errModelOwnership)
	}

	values, validValues := target["enum"].([]any)
	if !validValues || len(values) == 0 {
		return fmt.Errorf("known-value enum %q is empty or malformed: %w", name, errModelOwnership)
	}

	return validateUniqueStringEnum(name, values)
}

func validateUniqueStringEnum(name string, values []any) error {
	seen := make(map[string]bool, len(values))

	for _, rawValue := range values {
		value, ok := rawValue.(string)
		if !ok || value == "" || seen[value] {
			return fmt.Errorf("known-value enum %q has an invalid or duplicate member: %w", name, errModelOwnership)
		}

		seen[value] = true
	}

	return nil
}

func validateSchemaReference(reference string, bundle map[string]any) error {
	name, local := strings.CutPrefix(reference, "#/components/schemas/")
	if !local {
		return nil
	}

	components, hasComponents := bundle["components"].(map[string]any)
	if !hasComponents {
		return errMissingComponents
	}

	schemas, ok := components["schemas"].(map[string]any)
	if !ok || schemas[name] == nil {
		return fmt.Errorf("unresolved component %q: %w", name, errModelOwnership)
	}

	return nil
}

func validateOwnedOperation(method, path string, value any, owners map[string]string, seen map[string]bool) error {
	operation, valid := value.(map[string]any)
	if !valid {
		return fmt.Errorf("operation %s %s is invalid: %w", method, path, errModelOwnership)
	}

	identifier, valid := operation["operationId"].(string)
	if !valid || owners[identifier] == "" || seen[identifier] {
		return fmt.Errorf("operation %s %s (%q): %w", method, path, identifier, errModelOwnership)
	}

	seen[identifier] = true

	return nil
}

func validateRemainingOwners(owners map[string]string, seen map[string]bool) error {
	for identifier := range owners {
		if !seen[identifier] {
			return fmt.Errorf("unknown owned operation %q: %w", identifier, errModelOwnership)
		}
	}

	return nil
}
