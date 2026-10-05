package main

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// Schema fixture keys stay explicit and independent of generated provider keys.
const (
	propertyFixtureComponents = "components"
	propertyFixtureSchemas    = "schemas"
	propertyFixtureProperties = "properties"
	propertyFixtureType       = "type"
	propertyFixturePaths      = "paths"
	propertyFixtureGet        = "get"
	propertyFixtureParameters = "parameters"
	propertyFixtureName       = "name"
	propertyFixtureHeader     = "header"
)

func TestGeneratePropertyConstantsQualifiesKeysBySchema(t *testing.T) {
	t.Parallel()

	bundle := map[string]any{
		propertyFixtureComponents: map[string]any{
			propertyFixtureSchemas: map[string]any{
				"RawSharingMessage": map[string]any{
					propertyFixtureProperties: map[string]any{
						"protocol": map[string]any{propertyFixtureType: "integer"},
						"data":     map[string]any{propertyFixtureType: schemaObjectType},
					},
				},
			},
		},
	}

	generated, err := generatePropertyConstants(bundle)
	if err != nil {
		t.Fatal(err)
	}

	if strings.HasSuffix(string(generated), "\n\n") {
		t.Fatal("generated property constants have extra blank lines at EOF")
	}

	for _, expected := range []string{
		"PropertyRawSharingMessageData = " + strconv.Quote("data"),
		"PropertyRawSharingMessageProtocol = " + strconv.Quote("protocol"),
	} {
		if !strings.Contains(strings.Join(strings.Fields(string(generated)), " "), expected) {
			t.Fatalf("property constants omit %q:\n%s", expected, generated)
		}
	}
}

func TestGeneratePropertyConstantsIncludesQueryAndHeaderKeys(t *testing.T) {
	t.Parallel()

	bundle := map[string]any{
		propertyFixturePaths: map[string]any{
			"/v1.0/login": map[string]any{
				propertyFixtureGet: map[string]any{
					propertyFixtureParameters: []any{
						map[string]any{propertyFixtureName: "clientid", "in": "query"},
						map[string]any{propertyFixtureName: "X-requestId", "in": propertyFixtureHeader},
					},
				},
			},
		},
		propertyFixtureComponents: map[string]any{
			propertyFixtureSchemas: map[string]any{
				"EncryptedRequestQuery": map[string]any{
					"x-go-tuya-query-keys":    true,
					propertyFixtureProperties: map[string]any{"encdata": map[string]any{propertyFixtureType: stringSchemaType}},
				},
				"EncryptedRequestHeaders": map[string]any{
					"x-go-tuya-header-keys":   true,
					propertyFixtureProperties: map[string]any{"X-appKey": map[string]any{propertyFixtureType: stringSchemaType}},
				},
			},
		},
	}

	generated, err := generatePropertyConstants(bundle)
	if err != nil {
		t.Fatal(err)
	}

	if strings.HasSuffix(string(generated), "\n\n") {
		t.Fatal("generated property constants have extra blank lines at EOF")
	}

	for _, expected := range []string{
		`QueryParamClientid = "clientid"`,
		`QueryParamEncdata = "encdata"`,
		`HeaderXRequestId = "X-requestId"`,
		`HeaderXAppKey = "X-appKey"`,
	} {
		if !strings.Contains(strings.Join(strings.Fields(string(generated)), " "), expected) {
			t.Fatalf("generated key constants omit %q:\n%s", expected, generated)
		}
	}
}

func TestGeneratePropertyConstantsRejectsNameCollisions(t *testing.T) {
	t.Parallel()

	bundle := map[string]any{
		propertyFixtureComponents: map[string]any{
			propertyFixtureSchemas: map[string]any{
				"Payload": map[string]any{
					propertyFixtureProperties: map[string]any{
						"some-key": map[string]any{propertyFixtureType: stringSchemaType},
						"some_key": map[string]any{propertyFixtureType: stringSchemaType},
					},
				},
			},
		},
	}

	_, err := generatePropertyConstants(bundle)
	if !errors.Is(err, errModelOwnership) {
		t.Fatalf("colliding key constants accepted: %v", err)
	}
}
