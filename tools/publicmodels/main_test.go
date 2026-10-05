package main

import (
	"errors"
	"strings"
	"testing"
)

const (
	projectionComponentsKey     = "components"
	projectionSchemasKey        = "schemas"
	capabilityTypeName          = "CapabilityType"
	projectionStringType        = "string"
	generateConstantsKey        = "x-go-tuya-generate-untyped-constants"
	projectionEnumKey           = "enum"
	capabilityPowerName         = "CapabilityTypePower"
	projectionTypeKey           = "type"
	projectionEnumNamesKey      = "x-enum-varnames"
	capabilityPowerValue        = "power"
	projectionObjectType        = "object"
	capabilityNormalizationName = "CapabilityNormalization"
)

func TestGenerateUntypedConstantsUsesSchemaNamesAndValues(t *testing.T) {
	t.Parallel()

	document := map[string]any{
		projectionComponentsKey: map[string]any{
			projectionSchemasKey: map[string]any{
				capabilityTypeName: map[string]any{
					projectionTypeKey:      projectionStringType,
					generateConstantsKey:   true,
					projectionEnumKey:      []any{capabilityPowerValue, "color-temperature"},
					projectionEnumNamesKey: []any{capabilityPowerName, "CapabilityTypeColorTemperature"},
				},
				"ColorCapability": map[string]any{
					projectionTypeKey: projectionObjectType,
					"properties": map[string]any{
						"hue": map[string]any{projectionTypeKey: "integer"},
					},
				},
				capabilityNormalizationName: map[string]any{
					projectionTypeKey: projectionObjectType,
					"x-go-tuya-numeric-constants": map[string]any{
						"CapabilityScalePercentMaximum":    100,
						"CapabilityScaleTemperatureTenths": 10,
					},
				},
			},
		},
	}

	generated, err := generateUntypedConstants(document)
	if err != nil {
		t.Fatal(err)
	}

	const generatedPackageHeader = publicValuesGeneratedHeader + "\n\npackage tuya\n\n"
	if !strings.HasPrefix(string(generated), generatedPackageHeader) {
		t.Fatalf("generated header attaches the marker to the package doc:\n%s", generated)
	}

	if strings.HasSuffix(string(generated), "\n\n") {
		t.Fatal("generated public values have extra blank lines at EOF")
	}

	for _, expected := range []string{
		capabilityPowerName + ` = "power"`,
		`CapabilityTypeColorTemperature = "color-temperature"`,
		"CapabilityScalePercentMaximum = 100",
		"CapabilityScaleTemperatureTenths = 10",
		`ProjectionPropertyColorCapabilityHue = "hue"`,
	} {
		if !strings.Contains(strings.Join(strings.Fields(string(generated)), " "), expected) {
			t.Fatalf("generated constants omit %q:\n%s", expected, generated)
		}
	}
}

func TestGenerateUntypedConstantsRejectsUnpairedEnums(t *testing.T) {
	t.Parallel()

	document := map[string]any{
		projectionComponentsKey: map[string]any{
			projectionSchemasKey: map[string]any{
				capabilityTypeName: map[string]any{
					projectionTypeKey:      projectionStringType,
					generateConstantsKey:   true,
					projectionEnumKey:      []any{capabilityPowerValue, "color"},
					projectionEnumNamesKey: []any{"CapabilityTypePower"},
				},
			},
		},
	}

	_, err := generateUntypedConstants(document)
	if !errors.Is(err, errInvalidProjectionSchema) {
		t.Fatalf("unpaired enum values accepted: %v", err)
	}
}

func TestGenerateUntypedConstantsRejectsDuplicateValues(t *testing.T) {
	t.Parallel()

	document := map[string]any{
		projectionComponentsKey: map[string]any{
			projectionSchemasKey: map[string]any{
				capabilityTypeName: map[string]any{
					projectionTypeKey:      projectionStringType,
					generateConstantsKey:   true,
					projectionEnumKey:      []any{capabilityPowerValue, capabilityPowerValue},
					projectionEnumNamesKey: []any{capabilityPowerName, "CapabilityTypeAlias"},
				},
			},
		},
	}

	_, err := generateUntypedConstants(document)
	if !errors.Is(err, errInvalidProjectionSchema) {
		t.Fatalf("duplicate enum values accepted: %v", err)
	}
}

func TestGenerateNumericProjectionConstantsRejectsInvalidValuesAndNames(t *testing.T) {
	t.Parallel()

	for name, constants := range map[string]map[string]any{
		"non-integer value": {"CapabilityScalePercentMaximum": 100.5},
		"unexported name":   {"percentageMaximum": 100},
		"keyword name":      {"type": 100},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			document := map[string]any{
				projectionComponentsKey: map[string]any{
					projectionSchemasKey: map[string]any{
						capabilityNormalizationName: map[string]any{
							projectionTypeKey:             projectionObjectType,
							"x-go-tuya-numeric-constants": constants,
						},
					},
				},
			}

			_, err := generateUntypedConstants(document)
			if !errors.Is(err, errInvalidProjectionSchema) {
				t.Fatalf("invalid numeric constants accepted: %v", err)
			}
		})
	}
}

func TestValidateConfigRequiresNativeImportsTemplate(t *testing.T) {
	t.Parallel()

	config := validProjectionConfig()

	err := validateConfig(config)
	if err != nil {
		t.Fatalf("valid native imports template config rejected: %v", err)
	}

	options := projectionOutputOptions(t, config)
	delete(options, "user-templates")

	err = validateConfig(config)
	if !errors.Is(err, errInvalidProjectionSchema) {
		t.Fatalf("config without native imports template accepted: %v", err)
	}

	config = validProjectionConfig()
	options = projectionOutputOptions(t, config)
	options["user-templates"] = map[string]any{"imports.tmpl": "untracked/imports.tmpl"}

	err = validateConfig(config)
	if !errors.Is(err, errInvalidProjectionSchema) {
		t.Fatalf("config with untracked imports template accepted: %v", err)
	}
}

func projectionOutputOptions(t *testing.T, config map[string]any) map[string]any {
	t.Helper()

	options, ok := config["output-options"].(map[string]any)
	if !ok {
		t.Fatal("public projection output options are missing")
	}

	return options
}

func validProjectionConfig() map[string]any {
	return map[string]any{
		"package":  "tuya",
		"generate": map[string]any{"models": true},
		"output":   modelOutputPath,
		"output-options": map[string]any{
			"exclude-schemas":              []any{capabilityNormalizationName, "ContactSensorTextValue", "RTCSessionCapability", "SDKEventType"},
			"skip-prune":                   true,
			"prefer-skip-optional-pointer": true,
			"prefer-skip-optional-pointer-on-container-types": true,
			"user-templates": map[string]any{"imports.tmpl": importsTemplatePath},
		},
	}
}
