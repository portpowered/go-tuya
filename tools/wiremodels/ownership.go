package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var errModelOwnership = errors.New("schema ownership is incomplete or ambiguous")

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
			switch method {
			case "get", "post", "put", "patch", "delete", "head", "options", "trace":
			default:
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

	return validateComponentReferences(bundle, bundle)
}

func validateComponentReferences(value any, bundle map[string]any) error {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			if key == "$ref" {
				if reference, ok := child.(string); ok {
					err := validateSchemaReference(reference, bundle)
					if err != nil {
						return err
					}
				}
			}

			err := validateComponentReferences(child, bundle)
			if err != nil {
				return err
			}
		}
	case []any:
		for _, child := range node {
			err := validateComponentReferences(child, bundle)
			if err != nil {
				return err
			}
		}
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
