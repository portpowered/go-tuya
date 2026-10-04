package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	pathpkg "path"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const generatedModelImport = "github.com/portpowered/go-tuya/pkg/dependencymodels"
const publicValuesGeneratedPath = "pkg/tuya/public_values.gen.go"
const propertiesGeneratedPath = "pkg/dependencymodels/properties.gen.go"

var (
	errAnonymousJSONObject    = errors.New("anonymous JSON objects require named schema components")
	errMissingGeneratedType   = errors.New("schema component has no generated Go type")
	errMissingEnumDefinition  = errors.New("schema enum owner has no definition")
	errGeneratedEnumMembers   = errors.New("generated enum members differ from schema values")
	errGeneratedEnumNames     = errors.New("generated enum variable names differ from schema values")
	errInvalidNumericSchema   = errors.New("public schema has invalid numeric constant values")
	errNumericConstantDrift   = errors.New("public numeric constant differs from schema")
	errUnusedNumericConstant  = errors.New("public numeric constant has no production use")
	errUnownedNumericConstant = errors.New("generated numeric public constant is not owned by schema")
)

const emptyMarkdownCell = "—"

type modelUse struct {
	path string
	line int
	name string
}

type sourceModel struct {
	name        string
	path        string
	packageName string
	line        int
	properties  []string
	uses        []modelUse
}

type schemaEnumRow struct {
	owner     string
	typeName  string
	valueType string
	members   map[string]string
	bindings  []string
}

type generatedConstant struct {
	name     string
	typeName string
	value    string
	file     string
	numeric  bool
}

type sourceFile struct {
	path string
	file *ast.File
	set  *token.FileSet
}

type sourceInventory struct {
	wireUses       map[string][]modelUse
	operationUses  map[string][]modelUse
	publicUses     map[string][]modelUse
	models         []sourceModel
	jsonBoundaries []modelUse
	codecs         []modelUse
	anonymous      []string
}

type wireInventoryInputs struct {
	inventory      sourceInventory
	schemas        map[string]any
	generatedFiles map[string]string
	enumMembers    map[string][]string
	enumValues     map[string]map[string]string
	keyValues      map[string]string
}

func prepareWireInventoryInputs(sources modelSources) (wireInventoryInputs, error) {
	var inputs wireInventoryInputs

	var err error

	inputs.inventory, err = collectProductionInventory()
	if err != nil {
		return inputs, err
	}

	inputs.schemas, err = componentSchemas(sources.bundle)
	if err != nil {
		return inputs, err
	}

	inputs.generatedFiles, inputs.enumMembers, inputs.enumValues, err = inspectGeneratedModelFiles(sources.manifest)
	if err != nil {
		return inputs, err
	}

	validation := []func() error{
		func() error { return checkUnregisteredGeneratedModelFiles(inputs.generatedFiles) },
		func() error { return validateSchemaGeneratedTypes(inputs.schemas, inputs.generatedFiles) },
		func() error { return validateGeneratedEnumValues(inputs.schemas, inputs.enumValues) },
		func() error { return checkGeneratedWireConstructions(inputs.generatedFiles) },
	}
	for _, check := range validation {
		err = check()
		if err != nil {
			return inputs, err
		}
	}

	inputs.keyValues, err = inspectGeneratedWireKeyConstants()
	if err != nil {
		return inputs, err
	}

	return inputs, nil
}

func generateWireModelInventory(sources modelSources) ([]byte, error) {
	inputs, err := prepareWireInventoryInputs(sources)
	if err != nil {
		return nil, err
	}

	operations := inventoryOperations(sources.bundle)
	componentParents, componentRoots := schemaInventoryGraph(sources.bundle, inputs.schemas)

	var output strings.Builder

	writeInventoryHeader(&output)

	writeHTTPInventoryRows(&output, operations, inputs.inventory.operationUses)

	err = writeMQTTInventory(&output, inputs.inventory.wireUses)
	if err != nil {
		return nil, err
	}

	writeWireComponentInventory(&output, sources, inputs.schemas, inputs.generatedFiles, inputs.enumMembers,
		inputs.inventory.wireUses, componentParents, componentRoots)
	writePrimitiveBindings(&output, inputs.schemas, inputs.enumValues, inputs.keyValues, inputs.inventory.wireUses)

	err = writePublicProjectionInventory(&output, inputs.inventory.publicUses)
	if err != nil {
		return nil, err
	}

	writeHandwrittenPopulation(&output, inputs.inventory.models)
	writeJSONBoundaries(&output, inputs.inventory.jsonBoundaries, inputs.inventory.codecs)

	return []byte(output.String()), nil
}

func writeHTTPInventoryRows(output *strings.Builder, operations []inventoryOperation, uses map[string][]modelUse) {
	for _, operation := range operations {
		fmt.Fprintf(output, "| `%s` | %s | `%s %s` | %s | `wire.Operation%s()` | %s |\n",
			operation.id, operation.evidence, operation.method, operation.path,
			markdownCell(strings.Join(operation.parameters, ", ")),
			upperFirst(operation.id), markdownCell(joinUseLocations(uses[operation.id])))
	}
}

func collectProductionInventory() (sourceInventory, error) {
	files, err := readProductionSourceFiles()
	if err != nil {
		return sourceInventory{}, err
	}

	inventory := inspectInventorySources(files)

	err = rejectAnonymousJSONObjects(inventory.anonymous)
	if err != nil {
		return sourceInventory{}, err
	}

	return inventory, nil
}

func validateSchemaGeneratedTypes(schemas map[string]any, generatedFiles map[string]string) error {
	for name := range schemas {
		if generatedFiles[name] == "" {
			return fmt.Errorf("schema component %q has no generated Go type: %w", name, errMissingGeneratedType)
		}
	}

	return nil
}

func schemaInventoryGraph(bundle, schemas map[string]any) (map[string][]string, map[string][]string) {
	parents := schemaComponentParents(schemas)
	roots := schemaOperationRoots(bundle)

	return parents, roots
}

func schemaComponentParents(schemas map[string]any) map[string][]string {
	parents := make(map[string][]string)

	for parent, definition := range schemas {
		for _, reference := range schemaReferences(definition) {
			parents[reference] = append(parents[reference], parent)
		}
	}

	return parents
}

func schemaOperationRoots(bundle map[string]any) map[string][]string {
	roots := make(map[string][]string)

	paths, _ := bundle["paths"].(map[string]any)

	for _, rawPath := range paths {
		pathOperations, _ := rawPath.(map[string]any)
		for _, rawOperation := range pathOperations {
			operation, _ := rawOperation.(map[string]any)
			operationID, _ := operation["operationId"].(string)

			for _, reference := range schemaReferences(operation) {
				roots[reference] = append(roots[reference], operationID)
			}
		}
	}

	return roots
}

func writeInventoryHeader(output *strings.Builder) {
	output.WriteString("# Wire and model inventory\n\n")
	output.WriteString("Generated by `tools/wiremodels` from the responsibility schemas, generated Go models, and production source references. ")
	output.WriteString("Provider-documented HTTP operations and implementation-derived private/event contracts remain labeled at their schema source. ")
	output.WriteString("This file is a population and call-site index, not evidence that a contract was verified against a live account.\n\n")
	output.WriteString("## HTTP operations\n\n")
	output.WriteString("| Operation | Evidence | Method and path | Query/header/path names | Generated route | Production call sites |\n")
	output.WriteString("| --- | --- | --- | --- | --- | --- |\n")
}

func rejectAnonymousJSONObjects(locations []string) error {
	if len(locations) == 0 {
		return nil
	}

	return fmt.Errorf("anonymous JSON object in production source %s: %w", strings.Join(locations, ", "), errAnonymousJSONObject)
}

type inventoryOperation struct {
	id         string
	method     string
	path       string
	evidence   string
	parameters []string
}

func inventoryOperations(bundle map[string]any) []inventoryOperation {
	paths, _ := bundle["paths"].(map[string]any)

	var operations []inventoryOperation

	for path, rawPath := range paths {
		pathOperations, _ := rawPath.(map[string]any)
		for method, rawOperation := range pathOperations {
			operation, _ := rawOperation.(map[string]any)

			operationID, _ := operation["operationId"].(string)
			if operationID == "" {
				continue
			}

			evidence, _ := operation["x-go-tuya-evidence"].(string)
			parameters, _ := operation["parameters"].([]any)

			var names []string

			for _, rawParameter := range parameters {
				parameter, _ := rawParameter.(map[string]any)
				name, _ := parameter["name"].(string)

				location, _ := parameter["in"].(string)
				if name != "" && location != "" {
					names = append(names, location+":"+name)
				}
			}

			sort.Strings(names)
			operations = append(operations, inventoryOperation{id: operationID, method: strings.ToUpper(method), path: path, evidence: evidence, parameters: names})
		}
	}

	sort.Slice(operations, func(i, j int) bool { return operations[i].id < operations[j].id })

	return operations
}

func writeMQTTInventory(output *strings.Builder, wireUses map[string][]modelUse) error {
	output.WriteString("\n## MQTT channels and external network edge\n\n")
	output.WriteString("Channel templates and payload references are implementation-derived in `api/mqtt.asyncapi.yaml`. ")
	output.WriteString("The Paho v1.5.1 packet framing and connection source are inventoried separately in `api/external/paho-mqtt-v1.5.1.yaml`; ")
	output.WriteString("synthetic success and denied-CONNACK frames use the `SetCustomOpenConnectionFn` seam and `net.Pipe`.\n\n")
	output.WriteString("| Channel | Generated constant | Template | Production call sites |\n")
	output.WriteString("| --- | --- | --- | --- |\n")

	data, err := os.ReadFile("api/mqtt.asyncapi.yaml")
	if err != nil {
		return fmt.Errorf("read MQTT schema for inventory: %w", err)
	}

	var document map[string]any

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		return fmt.Errorf("decode MQTT schema for inventory: %w", err)
	}

	channels, _ := document["channels"].(map[string]any)

	names := make([]string, 0, len(channels))
	for name := range channels {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		channel, _ := channels[name].(map[string]any)
		address, _ := channel["address"].(string)
		constant := "MQTTChannel" + upperFirst(name)
		fmt.Fprintf(output, "| `%s` | `wire.%s` | `%s` | %s |\n", name, constant, address, markdownCell(joinUseLocations(wireUses[constant])))
	}

	return nil
}

func writeWireComponentInventory(
	output *strings.Builder,
	sources modelSources,
	schemas map[string]any,
	generatedFiles map[string]string,
	enumMembers map[string][]string,
	wireUses map[string][]modelUse,
	parents, operationRoots map[string][]string,
) {
	output.WriteString("\n## Generated provider wire components\n\n")
	output.WriteString("Each component is owned by one API responsibility in `api/models/<group>.yaml`. ")
	output.WriteString("Go output is produced by `go run ./tools/wiremodels -generate`, which invokes pinned `oapi-codegen/v2@v2.8.0`; ")
	output.WriteString("group files are under `pkg/dependencymodels/`. The `use` column lists exact-import source references, ")
	output.WriteString("operation call sites, and incoming schema references. Components without direct source or operation references ")
	output.WriteString("remain visible as unused schema entries for review.\n\n")
	output.WriteString("| Responsibility | Schema component / generated Go type | Source and generated file | ")
	output.WriteString("Wire properties or enum values | Incoming schema refs / HTTP operations | Production uses |\n")
	output.WriteString("| --- | --- | --- | --- | --- | --- |\n")

	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		group := sources.owners[name]
		definition, _ := schemas[name].(map[string]any)

		file := generatedFiles[name]
		if file == "" {
			file = "not found in generated output"
		}

		properties := schemaProperties(definition)
		if values := enumMembers[name]; len(values) > 0 {
			properties = append(properties, "enum: "+strings.Join(values, ", "))
		}

		incoming := append([]string(nil), parents[name]...)
		incoming = append(incoming, operationRoots[name]...)
		sort.Strings(incoming)
		incoming = compactStrings(incoming)

		uses := append([]modelUse(nil), wireUses[name]...)
		for _, member := range enumMembers[name] {
			uses = append(uses, wireUses[member]...)
		}

		fmt.Fprintf(output, "| `%s` | `%s` | `api/models/%s.yaml` → `%s` | %s | %s | %s |\n",
			group, name, group, file, markdownCell(strings.Join(properties, ", ")),
			markdownCell(strings.Join(incoming, ", ")), markdownCell(joinUseLocations(uses)))
	}

	writeGeneratedOperationParameterTypes(output, generatedFiles, wireUses)
}

func writeGeneratedOperationParameterTypes(output *strings.Builder, generatedFiles map[string]string, wireUses map[string][]modelUse) {
	output.WriteString("\nGenerated operation parameter structs are also listed because they are produced from OpenAPI `parameters`, not component schemas:\n\n")
	output.WriteString("| Go type | Generated file | Production uses |\n| --- | --- | --- |\n")

	names := make([]string, 0)

	for name, file := range generatedFiles {
		if strings.HasSuffix(name, "Params") {
			names = append(names, name)
			_ = file
		}
	}

	sort.Strings(names)

	for _, name := range names {
		fmt.Fprintf(output, "| `%s` | `%s` | %s |\n", name, generatedFiles[name], markdownCell(joinUseLocations(wireUses[name])))
	}
}

func writePrimitiveBindings(
	output *strings.Builder,
	schemas map[string]any,
	enumValues map[string]map[string]string,
	keyValues map[string]string,
	wireUses map[string][]modelUse,
) {
	output.WriteString("\n## Primitive values and known-value bindings\n\n")
	output.WriteString("Open string properties stay open for future values; `x-go-tuya-known-values-schema` ties currently recognized values ")
	output.WriteString("to a generated enum without closing the wire field. Generated constants are listed with their resolved production references.\n\n")
	output.WriteString("| Schema owner | Generated primitive type and members | Known-value field bindings | Production uses |\n")
	output.WriteString("| --- | --- | --- | --- |\n")

	for _, row := range schemaEnumRows(schemas, enumValues) {
		members := make([]string, 0, len(row.members))

		memberUses := append([]modelUse(nil), wireUses[row.typeName]...)

		for member, value := range row.members {
			formatted := strconv.Quote(value)
			if row.valueType != "string" {
				formatted = value
			}

			members = append(members, fmt.Sprintf("`%s` = %s", member, formatted))
			memberUses = append(memberUses, wireUses[member]...)
		}

		sort.Strings(members)

		bindings := row.bindings
		fmt.Fprintf(output, "| `%s` | %s | %s | %s |\n", row.owner, markdownCell("`"+row.typeName+"` — "+strings.Join(members, ", ")),
			markdownCell(strings.Join(bindings, ", ")), markdownCell(joinUseLocations(memberUses)))
	}

	output.WriteString("\nGenerated query and header keys are schema-derived from operation parameters and explicitly marked header/query components:\n\n")
	output.WriteString("| Key family | Schema value | Generated declaration | Production use |\n| --- | --- | --- | --- |\n")

	for _, name := range []string{"QueryParam", "Header", "Property"} {
		var constants []string

		for key := range keyValues {
			if strings.HasPrefix(key, name) {
				constants = append(constants, key)
			}
		}

		sort.Strings(constants)

		for _, key := range constants {
			fmt.Fprintf(output, "| %s | `%s` | `wire.%s` | %s |\n", name, keyValues[key], key, markdownCell(joinUseLocations(wireUses[key])))
		}
	}
}

func writePublicProjectionInventory(output *strings.Builder, uses map[string][]modelUse) error {
	output.WriteString("\n## Public semantic projections\n\n")
	output.WriteString("Public event and capability models are owned by `api/public-projections.yaml`, separate from provider response contracts. ")
	output.WriteString("`go run ./tools/publicmodels -generate` produces `pkg/tuya/public_projections.gen.go`; marked SDK constants ")
	output.WriteString("and the empty RTC capability marker are emitted in `pkg/tuya/public_values.gen.go`.\n\n")
	output.WriteString("| Public schema component | Generated declaration and constants | Schema properties or values | Production uses |\n")
	output.WriteString("| --- | --- | --- | --- |\n")

	schemas, constants, err := publicProjectionInventorySource()
	if err != nil {
		return err
	}

	return writePublicProjectionRows(output, schemas, constants, uses)
}

func publicProjectionInventorySource() (map[string]any, []generatedConstant, error) {
	data, err := os.ReadFile("api/public-projections.yaml")
	if err != nil {
		return nil, nil, fmt.Errorf("read public projection schema for inventory: %w", err)
	}

	var document map[string]any

	err = yaml.Unmarshal(data, &document)
	if err != nil {
		return nil, nil, fmt.Errorf("decode public projection schema for inventory: %w", err)
	}

	constants, err := inspectGeneratedPublicConstants()
	if err != nil {
		return nil, nil, err
	}

	components, _ := document["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)

	return schemas, constants, nil
}

func writePublicProjectionRows(output *strings.Builder, schemas map[string]any, constants []generatedConstant, uses map[string][]modelUse) error {
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		schema, _ := schemas[name].(map[string]any)

		err := validatePublicNumericConstants(name, schema, constants, uses)
		if err != nil {
			return err
		}

		generated, properties, componentUses := publicProjectionRow(name, schema, constants, uses)
		fmt.Fprintf(output, "| `%s` | %s | %s | %s |\n", name,
			markdownCell(strings.Join(generated, ", ")), markdownCell(strings.Join(properties, ", ")),
			markdownCell(joinUseLocations(componentUses)))
	}

	return nil
}

func publicProjectionRow(name string, schema map[string]any, constants []generatedConstant, uses map[string][]modelUse) ([]string, []string, []modelUse) {
	declarationFile := publicProjectionDeclarationFile(name, schema)
	properties := schemaProperties(schema)

	if values := schemaEnumValues(schema); len(values) > 0 {
		properties = append(properties, "enum: "+strings.Join(values, ", "))
	}

	if numeric := schemaNumericConstants(schema); len(numeric) > 0 {
		properties = append(properties, "numeric constants: "+strings.Join(schemaNumericConstantDisplay(numeric), ", "))
	}

	generated := publicProjectionGeneratedValues(name, schema, declarationFile, constants)
	componentUses := publicProjectionUses(name, schema, constants, uses)

	sort.Strings(generated)
	sortModelUses(componentUses)

	return generated, properties, componentUses
}

func publicProjectionDeclarationFile(name string, schema map[string]any) string {
	if name == "RTCSessionCapability" || schema["x-go-tuya-generate-untyped-constants"] == true || schema["x-go-tuya-numeric-constants"] != nil {
		return publicValuesGeneratedPath
	}

	return "pkg/tuya/public_projections.gen.go"
}

func publicProjectionGeneratedValues(name string, schema map[string]any, declarationFile string, constants []generatedConstant) []string {
	var generated []string
	if schema["x-go-tuya-generate-untyped-constants"] != true {
		generated = append(generated, fmt.Sprintf("`%s`: type %s", declarationFile, name))
	}

	for _, constant := range constants {
		if publicProjectionConstantMatches(name, schema, constant) {
			value := strconv.Quote(constant.value)
			if constant.numeric {
				value = constant.value
			}

			generated = append(generated, fmt.Sprintf("`%s`: %s=%s", constant.file, constant.name, value))
		}
	}

	return generated
}

func publicProjectionConstantMatches(name string, schema map[string]any, constant generatedConstant) bool {
	if strings.HasPrefix(constant.name, "ProjectionProperty"+name) || constant.typeName == name {
		return true
	}

	if numeric := schemaNumericConstants(schema); numeric != nil {
		_, exists := numeric[constant.name]

		return exists
	}

	if schema["x-go-tuya-generate-untyped-constants"] != true {
		return false
	}

	return slices.Contains(schemaEnumVariableNames(schema), constant.name)
}

func schemaNumericConstants(schema map[string]any) map[string]string {
	rawConstants, hasConstants := schema["x-go-tuya-numeric-constants"]
	if !hasConstants {
		return nil
	}

	constants, validConstants := rawConstants.(map[string]any)
	if !validConstants {
		return nil
	}

	values := make(map[string]string, len(constants))

	for name, rawValue := range constants {
		switch value := rawValue.(type) {
		case int:
			values[name] = strconv.Itoa(value)
		case int64:
			values[name] = strconv.FormatInt(value, 10)
		case uint64:
			values[name] = strconv.FormatUint(value, 10)
		default:
			return nil
		}
	}

	return values
}

func schemaNumericConstantDisplay(values map[string]string) []string {
	names := sortedNumericConstantNames(values)

	display := make([]string, 0, len(names))
	for _, name := range names {
		display = append(display, name+"="+values[name])
	}

	return display
}

func sortedNumericConstantNames(values map[string]string) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

//nolint:cyclop // Each branch checks one ownership, value, or production-use invariant.
func validatePublicNumericConstants(schemaName string, schema map[string]any, constants []generatedConstant, uses map[string][]modelUse) error {
	expected := schemaNumericConstants(schema)
	if schema["x-go-tuya-numeric-constants"] != nil && expected == nil {
		return fmt.Errorf("%w: %s", errInvalidNumericSchema, schemaName)
	}

	if expected == nil {
		return nil
	}

	actual := make(map[string]string)

	for _, constant := range constants {
		if constant.numeric && constant.file == publicValuesGeneratedPath {
			actual[constant.name] = constant.value
		}
	}

	for _, name := range sortedNumericConstantNames(expected) {
		value := expected[name]
		if actual[name] != value {
			return fmt.Errorf("%w: schema %s, constant %s=%s, generated %s", errNumericConstantDrift, schemaName, name, value, actual[name])
		}

		if len(uses[name]) == 0 {
			return fmt.Errorf("%w: schema %s, constant %s", errUnusedNumericConstant, schemaName, name)
		}
	}

	for name := range actual {
		if _, exists := expected[name]; !exists {
			return fmt.Errorf("%w: %s, schema %s", errUnownedNumericConstant, name, schemaName)
		}
	}

	return nil
}

func publicProjectionUses(name string, schema map[string]any, constants []generatedConstant, uses map[string][]modelUse) []modelUse {
	componentUses := append([]modelUse(nil), uses[name]...)

	for _, constant := range constants {
		if publicProjectionConstantMatches(name, schema, constant) {
			componentUses = append(componentUses, uses[constant.name]...)
		}
	}

	return componentUses
}

func writeHandwrittenPopulation(output *strings.Builder, models []sourceModel) {
	output.WriteString("\n## Handwritten JSON-tagged SDK and CLI population\n\n")
	output.WriteString("These source structs are inventoried separately from generated provider wire models. ")
	output.WriteString("SDK request and response types are caller-facing inputs or semantic outputs; ")
	output.WriteString("the wire call sites construct or decode the generated provider types listed above. ")
	output.WriteString("CLI structs are local JSON output or credential-file formats, ")
	output.WriteString("not provider payloads. Any inline anonymous JSON object is rejected by generation and must become a named schema component.\n\n")
	output.WriteString("| Type and source | JSON properties | Production identifier references | Classification |\n| --- | --- | --- | --- |\n")

	for _, model := range models {
		classification := "SDK-facing JSON model; not passed directly to a provider JSON encoder/decoder"
		if strings.HasPrefix(model.path, "cmd/") {
			classification = "CLI output or local credential-file shape; not a provider payload"
		}

		uses := joinUseLocations(model.uses)
		if uses == emptyMarkdownCell {
			uses = "no production references beyond declaration; retained public compatibility surface"
		}

		fmt.Fprintf(output, "| `%s` — `%s:%d` | %s | %s | %s |\n", model.name, model.path, model.line,
			markdownCell(strings.Join(model.properties, ", ")), markdownCell(uses), classification)
	}
}

func writeJSONBoundaries(output *strings.Builder, boundaries, codecs []modelUse) {
	output.WriteString("\n## JSON boundary and custom-codec inventory\n\n")
	output.WriteString("`pkg/tuya/wire_decode.go` converts values through generated schema models; endpoint decoders in `auth.go`, `encryption.go`, ")
	output.WriteString("`message_protocol.go`, and `messages.go` use generated wire types. ")
	output.WriteString("The list below is collected from production JSON ")
	output.WriteString("encoder/decoder call sites.\n\n")
	output.WriteString("| JSON boundary | Source |\n| --- | --- |\n")

	for _, boundary := range boundaries {
		fmt.Fprintf(output, "| `%s` | `%s:%d` |\n", boundary.name, boundary.path, boundary.line)
	}

	output.WriteString("\nHandwritten JSON codec methods:\n\n")

	if len(codecs) == 0 {
		output.WriteString("- None in handwritten SDK source. `oapi-codegen` emits `AdditionalProperties` codecs ")
		output.WriteString("in generated model files from the responsibility schemas.\n")

		return
	}

	for _, codec := range codecs {
		fmt.Fprintf(output, "- `%s` at `%s:%d`\n", codec.name, codec.path, codec.line)
	}
}

func inspectInventorySources(files []sourceFile) sourceInventory {
	inventory := sourceInventory{
		wireUses:       make(map[string][]modelUse),
		operationUses:  make(map[string][]modelUse),
		publicUses:     make(map[string][]modelUse),
		models:         nil,
		jsonBoundaries: nil,
		codecs:         nil,
		anonymous:      nil,
	}

	inventory.models = collectDeclaredModels(files)
	for _, source := range files {
		collectSourceUses(source, &inventory)
	}

	collectModelUseLocations(inventory.models, files)
	inventory.anonymous = collectAnonymousJSONObjects(files)
	sortSourceInventory(&inventory)

	return inventory
}

func collectDeclaredModels(files []sourceFile) []sourceModel {
	var models []sourceModel

	for _, source := range files {
		for _, declaration := range source.file.Decls {
			typeDeclaration, isGenDecl := declaration.(*ast.GenDecl)
			if !isGenDecl || typeDeclaration.Tok != token.TYPE {
				continue
			}

			for _, spec := range typeDeclaration.Specs {
				typeSpec, isTypeSpec := spec.(*ast.TypeSpec)
				if !isTypeSpec {
					continue
				}

				structure, isStruct := typeSpec.Type.(*ast.StructType)
				if !isStruct {
					continue
				}

				properties := jsonProperties(structure)
				if len(properties) == 0 {
					continue
				}

				models = append(models, sourceModel{
					name:        typeSpec.Name.Name,
					path:        source.path,
					packageName: source.file.Name.Name,
					line:        source.set.Position(typeSpec.Name.Pos()).Line,
					properties:  properties,
					uses:        nil,
				})
			}
		}
	}

	return models
}

func collectSourceUses(source sourceFile, inventory *sourceInventory) {
	imports := sourceImports(source.file)
	wireAlias := importNameForPath(source.file, generatedModelImport)

	ast.Inspect(source.file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.SelectorExpr:
			collectSelectorUses(source, imports, wireAlias, value, inventory)
		case *ast.Ident:
			collectLocalPublicUse(source, value, inventory.publicUses)
		case *ast.CallExpr:
			collectJSONBoundaryUse(source, value, &inventory.jsonBoundaries)
		case *ast.FuncDecl:
			collectJSONCodecUse(source, value, &inventory.codecs)
		}

		return true
	})
}

func collectSelectorUses(source sourceFile, imports map[string]string, wireAlias string, selector *ast.SelectorExpr, inventory *sourceInventory) {
	base, isIdentifier := selector.X.(*ast.Ident)
	if !isIdentifier {
		return
	}

	if wireAlias != "" && base.Name == wireAlias && imports[base.Name] == generatedModelImport {
		line := source.set.Position(selector.Pos()).Line
		use := modelUse{path: source.path, line: line, name: selector.Sel.Name}

		inventory.wireUses[selector.Sel.Name] = append(inventory.wireUses[selector.Sel.Name], use)

		if operationID, isOperation := operationIDFromAccessor(selector.Sel.Name); isOperation {
			inventory.operationUses[operationID] = append(inventory.operationUses[operationID], use)
		}
	}

	if imports[base.Name] == "github.com/portpowered/go-tuya/pkg/tuya" {
		line := source.set.Position(selector.Sel.Pos()).Line
		use := modelUse{path: source.path, line: line, name: selector.Sel.Name}
		inventory.publicUses[selector.Sel.Name] = append(inventory.publicUses[selector.Sel.Name], use)
	}
}

func collectLocalPublicUse(source sourceFile, identifier *ast.Ident, uses map[string][]modelUse) {
	if source.file.Name.Name != "tuya" || pathpkg.Dir(source.path) != "pkg/tuya" {
		return
	}

	line := source.set.Position(identifier.Pos()).Line
	uses[identifier.Name] = append(uses[identifier.Name], modelUse{path: source.path, line: line, name: identifier.Name})
}

func collectJSONBoundaryUse(source sourceFile, call *ast.CallExpr, boundaries *[]modelUse) {
	name := calledName(call.Fun)
	if !strings.HasPrefix(name, "json.Marshal") && !strings.HasPrefix(name, "json.Unmarshal") &&
		!strings.HasPrefix(name, "json.NewDecoder") && name != "Decode" {
		return
	}

	*boundaries = append(*boundaries, modelUse{path: source.path, line: source.set.Position(call.Pos()).Line, name: name})
}

func collectJSONCodecUse(source sourceFile, function *ast.FuncDecl, codecs *[]modelUse) {
	if function.Name == nil || (function.Name.Name != "MarshalJSON" && function.Name.Name != "UnmarshalJSON") {
		return
	}

	*codecs = append(*codecs, modelUse{path: source.path, line: source.set.Position(function.Pos()).Line, name: function.Name.Name})
}

func collectModelUseLocations(models []sourceModel, files []sourceFile) {
	for index := range models {
		for _, source := range files {
			models[index].uses = append(models[index].uses, modelUsesInSource(models[index], source)...)
		}
	}
}

func modelUsesInSource(model sourceModel, source sourceFile) []modelUse {
	imports := sourceImports(source.file)
	publicSDKAlias := importNameForPath(source.file, "github.com/portpowered/go-tuya/pkg/tuya")

	var uses []modelUse

	ast.Inspect(source.file, func(node ast.Node) bool {
		if source.file.Name.Name == model.packageName && pathpkg.Dir(source.path) == pathpkg.Dir(model.path) {
			uses = append(uses, localModelUse(model, source, node)...)

			return true
		}

		if publicSDKAlias == "" || imports[publicSDKAlias] != "github.com/portpowered/go-tuya/pkg/tuya" {
			return true
		}

		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		qualifier, isIdentifier := selector.X.(*ast.Ident)
		if !isIdentifier || qualifier.Name != publicSDKAlias || selector.Sel.Name != model.name {
			return true
		}

		uses = append(uses, modelUse{path: source.path, line: source.set.Position(selector.Sel.Pos()).Line, name: selector.Sel.Name})

		return true
	})

	return uses
}

func localModelUse(model sourceModel, source sourceFile, node ast.Node) []modelUse {
	identifier, ok := node.(*ast.Ident)
	if !ok || identifier.Name != model.name {
		return nil
	}

	position := source.set.Position(identifier.Pos())
	if source.path == model.path && position.Line == model.line {
		return nil
	}

	return []modelUse{{path: source.path, line: position.Line, name: identifier.Name}}
}

func collectAnonymousJSONObjects(files []sourceFile) []string {
	var anonymous []string

	for _, source := range files {
		namedPositions := namedStructPositions(source.file)
		ast.Inspect(source.file, func(node ast.Node) bool {
			structure, isStruct := node.(*ast.StructType)
			if isStruct && len(jsonProperties(structure)) > 0 && !namedPositions[structure.Pos()] {
				anonymous = append(anonymous, fmt.Sprintf("%s:%d", source.path, source.set.Position(structure.Pos()).Line))
			}

			return true
		})
	}

	return compactStrings(anonymous)
}

func namedStructPositions(file *ast.File) map[token.Pos]bool {
	positions := make(map[token.Pos]bool)

	ast.Inspect(file, func(node ast.Node) bool {
		typeSpec, isTypeSpec := node.(*ast.TypeSpec)
		if !isTypeSpec {
			return true
		}

		structure, isStruct := typeSpec.Type.(*ast.StructType)
		if isStruct {
			positions[structure.Pos()] = true
		}

		return true
	})

	return positions
}

func sortSourceInventory(inventory *sourceInventory) {
	for _, uses := range inventory.wireUses {
		sortModelUses(uses)
	}

	for _, uses := range inventory.operationUses {
		sortModelUses(uses)
	}

	for _, uses := range inventory.publicUses {
		sortModelUses(uses)
	}

	for index := range inventory.models {
		slices.Sort(inventory.models[index].properties)
		sortModelUses(inventory.models[index].uses)
	}

	sortModelUses(inventory.jsonBoundaries)
	sortModelUses(inventory.codecs)
	sort.Strings(inventory.anonymous)
}

func readProductionSourceFiles() ([]sourceFile, error) {
	paths, err := productionSourcePaths()
	if err != nil {
		return nil, err
	}

	return parseProductionSourceFiles(paths)
}

func productionSourcePaths() ([]string, error) {
	var paths []string

	for _, root := range []string{"pkg", "cmd", "examples"} {
		rootPaths, err := productionSourcePathsUnder(root)
		if err != nil {
			return nil, fmt.Errorf("walk source root %s: %w", root, err)
		}

		paths = append(paths, rootPaths...)
	}

	sort.Strings(paths)

	return paths, nil
}

func productionSourcePathsUnder(root string) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if shouldSkipProductionDirectory(path, root, entry) {
			return filepath.SkipDir
		}

		if entry.IsDir() || !isProductionSourceFile(path) {
			return nil
		}

		paths = append(paths, filepath.Clean(path))

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk production source root %s: %w", root, err)
	}

	return paths, nil
}

func shouldSkipProductionDirectory(path, root string, entry os.DirEntry) bool {
	return entry.IsDir() && path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor")
}

func isProductionSourceFile(path string) bool {
	return filepath.Ext(path) == ".go" && !strings.HasSuffix(path, "_test.go") && !strings.HasSuffix(path, ".gen.go")
}

func parseProductionSourceFiles(paths []string) ([]sourceFile, error) {
	files := make([]sourceFile, 0, len(paths))

	for _, path := range paths {
		// #nosec G304 -- source path is discovered below fixed production roots.
		set := token.NewFileSet()

		file, err := parser.ParseFile(set, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse production source %s: %w", path, err)
		}

		files = append(files, sourceFile{path: filepath.ToSlash(path), file: file, set: set})
	}

	return files, nil
}

func sourceImports(file *ast.File) map[string]string {
	imports := make(map[string]string, len(file.Imports))

	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}

		alias := filepath.Base(path)
		if spec.Name != nil {
			alias = spec.Name.Name
		}

		imports[alias] = path
	}

	return imports
}

func importNameForPath(file *ast.File, importPath string) string {
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

func inspectGeneratedModelFiles(manifest modelManifest) (map[string]string, map[string][]string, map[string]map[string]string, error) {
	generatedFiles := make(map[string]string)
	enumMembers := make(map[string][]string)
	enumValues := make(map[string]map[string]string)

	for _, group := range manifest.Groups {
		path := "pkg/dependencymodels/" + group.Name + ".gen.go"
		// #nosec G304 -- generated paths are derived from the checked-in model manifest.
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("parse generated model file %s: %w", path, err)
		}

		indexGeneratedModelFile(file, path, generatedFiles, enumMembers, enumValues)
	}

	for name := range enumMembers {
		sort.Strings(enumMembers[name])
		enumMembers[name] = compactStrings(enumMembers[name])
	}

	return generatedFiles, enumMembers, enumValues, nil
}

func indexGeneratedModelFile(
	file *ast.File,
	path string,
	generatedFiles map[string]string,
	enumMembers map[string][]string,
	enumValues map[string]map[string]string,
) {
	for _, declaration := range file.Decls {
		group, isGroup := declaration.(*ast.GenDecl)
		if !isGroup {
			continue
		}

		for _, spec := range group.Specs {
			indexGeneratedModelSpec(spec, path, generatedFiles, enumMembers, enumValues)
		}
	}
}

func indexGeneratedModelSpec(
	spec ast.Spec,
	path string,
	generatedFiles map[string]string,
	enumMembers map[string][]string,
	enumValues map[string]map[string]string,
) {
	switch item := spec.(type) {
	case *ast.TypeSpec:
		generatedFiles[item.Name.Name] = path
	case *ast.ValueSpec:
		indexGeneratedEnumValues(item, enumMembers, enumValues)
	}
}

func indexGeneratedEnumValues(valueSpec *ast.ValueSpec, enumMembers map[string][]string, enumValues map[string]map[string]string) {
	identifier, isIdentifier := valueSpec.Type.(*ast.Ident)
	if !isIdentifier {
		return
	}

	for index, name := range valueSpec.Names {
		enumMembers[identifier.Name] = append(enumMembers[identifier.Name], name.Name)

		if index >= len(valueSpec.Values) {
			continue
		}

		value := literalValue(valueSpec.Values[index])
		if value == "" {
			continue
		}

		if enumValues[identifier.Name] == nil {
			enumValues[identifier.Name] = make(map[string]string)
		}

		enumValues[identifier.Name][name.Name] = value
	}
}

func literalValue(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.BasicLit:
		if value.Kind == token.STRING {
			unquoted, err := strconv.Unquote(value.Value)
			if err != nil {
				return ""
			}

			return unquoted
		}

		return value.Value
	case *ast.UnaryExpr:
		if value.Op == token.SUB {
			return "-" + literalValue(value.X)
		}
	}

	return ""
}

func schemaReferences(value any) []string {
	var (
		references []string
		visit      func(any)
	)

	visit = func(node any) {
		switch current := node.(type) {
		case map[string]any:
			if reference, ok := current["$ref"].(string); ok {
				if name, local := strings.CutPrefix(reference, "#/components/schemas/"); local {
					references = append(references, name)
				}
			}

			for _, child := range current {
				visit(child)
			}
		case []any:
			for _, child := range current {
				visit(child)
			}
		}
	}
	visit(value)
	sort.Strings(references)

	return compactStrings(references)
}

func componentSchemas(bundle map[string]any) (map[string]any, error) {
	components, hasComponents := bundle["components"].(map[string]any)
	if !hasComponents {
		return nil, errMissingComponents
	}

	schemas, hasSchemas := components["schemas"].(map[string]any)
	if !hasSchemas {
		return nil, errMissingComponents
	}

	return schemas, nil
}

func schemaProperties(schema map[string]any) []string {
	properties, _ := schema["properties"].(map[string]any)

	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func schemaEnumValues(schema map[string]any) []string {
	values, _ := schema["enum"].([]any)

	result := make([]string, 0, len(values))

	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		} else {
			result = append(result, fmt.Sprint(value))
		}
	}

	sort.Strings(result)

	return result
}

func schemaEnumRows(schemas map[string]any, enumValues map[string]map[string]string) []schemaEnumRow {
	bindings := knownValueBindingsByTarget(schemas)

	var rows []schemaEnumRow

	components := make([]string, 0, len(schemas))
	for name := range schemas {
		components = append(components, name)
	}

	sort.Strings(components)

	for _, component := range components {
		root, _ := schemas[component].(map[string]any)
		appendSchemaEnumRows(&rows, component, root, nil, component, enumValues, bindings)
	}

	return rows
}

func appendSchemaEnumRows(
	rows *[]schemaEnumRow,
	component string,
	schema map[string]any,
	path []string,
	owner string,
	enumValues map[string]map[string]string,
	bindings map[string][]string,
) {
	if schema == nil {
		return
	}

	appendSchemaEnumRow(rows, component, schema, path, owner, enumValues, bindings)
	appendSchemaPropertyEnumRows(rows, component, schema, path, enumValues, bindings)
	appendSchemaItemEnumRows(rows, component, schema, path, enumValues, bindings)
}

func appendSchemaEnumRow(
	rows *[]schemaEnumRow,
	component string,
	schema map[string]any,
	path []string,
	owner string,
	enumValues map[string]map[string]string,
	bindings map[string][]string,
) {
	if _, hasEnum := schema["enum"]; !hasEnum {
		return
	}

	typeName := component + typePathSuffix(path)
	if len(path) == 0 {
		typeName = component
	}

	*rows = append(*rows, schemaEnumRow{
		owner: owner, typeName: typeName,
		valueType: schemaType(schema), members: enumValues[typeName],
		bindings: bindings[typeName],
	})
}

func appendSchemaPropertyEnumRows(
	rows *[]schemaEnumRow,
	component string,
	schema map[string]any,
	path []string,
	enumValues map[string]map[string]string,
	bindings map[string][]string,
) {
	properties, _ := schema["properties"].(map[string]any)
	propertyNames := make([]string, 0, len(properties))

	for name := range properties {
		propertyNames = append(propertyNames, name)
	}

	sort.Strings(propertyNames)

	for _, name := range propertyNames {
		child, _ := properties[name].(map[string]any)
		childPath := append(append([]string(nil), path...), name)
		childOwner := component + "." + strings.Join(childPath, ".")
		appendSchemaEnumRows(rows, component, child, childPath, childOwner, enumValues, bindings)
	}
}

func appendSchemaItemEnumRows(
	rows *[]schemaEnumRow,
	component string,
	schema map[string]any,
	path []string,
	enumValues map[string]map[string]string,
	bindings map[string][]string,
) {
	item, hasItem := schema["items"].(map[string]any)
	if !hasItem {
		return
	}

	itemPath := append(append([]string(nil), path...), "item")
	ownerPath := append(append([]string(nil), path...), "items")
	owner := component + "." + strings.Join(ownerPath, ".")
	appendSchemaEnumRows(rows, component, item, itemPath, owner, enumValues, bindings)
}

func typePathSuffix(path []string) string {
	var suffix strings.Builder
	for _, part := range path {
		suffix.WriteString(propertyIdentifier(part))
	}

	return suffix.String()
}

func schemaType(schema map[string]any) string {
	value, _ := schema["type"].(string)

	return value
}

func knownValueBindingsByTarget(schemas map[string]any) map[string][]string {
	bindings := make(map[string][]string)

	componentNames := make([]string, 0, len(schemas))
	for name := range schemas {
		componentNames = append(componentNames, name)
	}

	sort.Strings(componentNames)

	for _, component := range componentNames {
		collectKnownValueBindings(schemas[component], component, nil, bindings)
	}

	for target := range bindings {
		sort.Strings(bindings[target])
	}

	return bindings
}

func collectKnownValueBindings(value any, component string, path []string, bindings map[string][]string) {
	switch node := value.(type) {
	case map[string]any:
		collectKnownValueBinding(node, component, path, bindings)
		collectKnownPropertyBindings(node, component, path, bindings)

		if items, hasItems := node["items"]; hasItems {
			collectKnownValueBindings(items, component, append(append([]string(nil), path...), "items"), bindings)
		}
	case []any:
		collectKnownArrayBindings(node, component, path, bindings)
	}
}

func collectKnownValueBinding(node map[string]any, component string, path []string, bindings map[string][]string) {
	target, hasTarget := node["x-go-tuya-known-values-schema"].(string)
	if !hasTarget {
		return
	}

	location := component + "." + strings.Join(path, ".") + " (open string)"
	bindings[target] = append(bindings[target], location)
}

func collectKnownPropertyBindings(node map[string]any, component string, path []string, bindings map[string][]string) {
	properties, _ := node["properties"].(map[string]any)
	propertyNames := make([]string, 0, len(properties))

	for name := range properties {
		propertyNames = append(propertyNames, name)
	}

	sort.Strings(propertyNames)

	for _, name := range propertyNames {
		childPath := append(append([]string(nil), path...), name)
		collectKnownValueBindings(properties[name], component, childPath, bindings)
	}
}

func collectKnownArrayBindings(nodes []any, component string, path []string, bindings map[string][]string) {
	for index, child := range nodes {
		childPath := append(append([]string(nil), path...), strconv.Itoa(index))
		collectKnownValueBindings(child, component, childPath, bindings)
	}
}

func validateGeneratedEnumValues(schemas map[string]any, generated map[string]map[string]string) error {
	for _, row := range schemaEnumRows(schemas, generated) {
		err := validateGeneratedEnumRow(row, schemas)
		if err != nil {
			return err
		}
	}

	return nil
}

func validateGeneratedEnumRow(row schemaEnumRow, schemas map[string]any) error {
	definition := schemaDefinitionForOwner(schemas, row.owner)
	if definition == nil {
		return fmt.Errorf("schema enum owner %s: %w", row.owner, errMissingEnumDefinition)
	}

	err := validateGeneratedEnumMembers(row, definition)
	if err != nil {
		return err
	}

	return validateGeneratedEnumNames(row, definition)
}

func validateGeneratedEnumMembers(row schemaEnumRow, definition map[string]any) error {
	wantValues := schemaEnumValues(definition)
	want := make(map[string]bool, len(wantValues))

	for _, value := range wantValues {
		want[value] = true
	}

	got := make(map[string]bool, len(row.members))

	for _, value := range row.members {
		got[value] = true
	}

	if len(want) == 0 || len(got) != len(want) {
		return fmt.Errorf("schema enum %s maps to generated type %s: %w", row.owner, row.typeName, errGeneratedEnumMembers)
	}

	return compareEnumMemberValues(row, want, got)
}

func compareEnumMemberValues(row schemaEnumRow, want, got map[string]bool) error {
	for value := range want {
		if !got[value] {
			return fmt.Errorf("schema enum %s value %q is missing from generated type %s: %w", row.owner, value, row.typeName, errGeneratedEnumMembers)
		}
	}

	for member, value := range row.members {
		if !want[value] {
			return fmt.Errorf("generated enum member %s.%s value %q is absent from schema %s: %w", row.typeName, member, value, row.owner, errGeneratedEnumMembers)
		}
	}

	return nil
}

func validateGeneratedEnumNames(row schemaEnumRow, definition map[string]any) error {
	rawNames, hasNames := definition["x-enum-varnames"].([]any)
	if !hasNames {
		return nil
	}

	rawValues, _ := definition["enum"].([]any)
	if len(rawNames) != len(rawValues) {
		return fmt.Errorf("schema enum %s: %w", row.owner, errGeneratedEnumNames)
	}

	for index, rawName := range rawNames {
		member, isMember := rawName.(string)
		if !isMember || row.members[member] != fmt.Sprint(rawValues[index]) {
			return fmt.Errorf("generated enum type %s does not preserve schema binding at member %v: %w", row.typeName, rawName, errGeneratedEnumNames)
		}
	}

	return nil
}

func schemaDefinitionForOwner(schemas map[string]any, owner string) map[string]any {
	parts := strings.Split(owner, ".")

	current, _ := schemas[parts[0]].(map[string]any)

	for _, part := range parts[1:] {
		if part == "items" {
			current, _ = current["items"].(map[string]any)

			continue
		}

		properties, _ := current["properties"].(map[string]any)
		current, _ = properties[part].(map[string]any)
	}

	return current
}

func inspectGeneratedWireKeyConstants() (map[string]string, error) {
	path := propertiesGeneratedPath
	// #nosec G304 -- generated output path is fixed and repository-owned.
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse generated property constants: %w", err)
	}

	values := make(map[string]string)

	for _, declaration := range file.Decls {
		group, isGroup := declaration.(*ast.GenDecl)
		if !isGroup {
			continue
		}

		for _, spec := range group.Specs {
			valueSpec, isValueSpec := spec.(*ast.ValueSpec)
			if !isValueSpec {
				continue
			}

			for index, name := range valueSpec.Names {
				if index >= len(valueSpec.Values) {
					continue
				}

				value := literalValue(valueSpec.Values[index])
				if value != "" {
					values[name.Name] = value
				}
			}
		}
	}

	return values, nil
}

func inspectGeneratedPublicConstants() ([]generatedConstant, error) {
	var constants []generatedConstant

	for _, path := range []string{"pkg/tuya/public_projections.gen.go", "pkg/tuya/public_values.gen.go"} {
		fileConstants, err := publicConstantsFromFile(path)
		if err != nil {
			return nil, err
		}

		constants = append(constants, fileConstants...)
	}

	sort.Slice(constants, func(i, j int) bool {
		if constants[i].file != constants[j].file {
			return constants[i].file < constants[j].file
		}

		return constants[i].name < constants[j].name
	})

	return constants, nil
}

func publicConstantsFromFile(path string) ([]generatedConstant, error) {
	// #nosec G304 -- generated output paths are fixed and repository-owned.
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse generated public projection %s: %w", path, err)
	}

	constants := make([]generatedConstant, 0, len(file.Decls))

	for _, declaration := range file.Decls {
		group, isGroup := declaration.(*ast.GenDecl)
		if !isGroup || group.Tok != token.CONST {
			continue
		}

		for _, spec := range group.Specs {
			valueSpec, isValueSpec := spec.(*ast.ValueSpec)
			if isValueSpec {
				constants = append(constants, constantsFromPublicValueSpec(valueSpec, path)...)
			}
		}
	}

	return constants, nil
}

func constantsFromPublicValueSpec(valueSpec *ast.ValueSpec, path string) []generatedConstant {
	typeName, _ := valueSpec.Type.(*ast.Ident)
	constants := make([]generatedConstant, 0, len(valueSpec.Names))

	for index, name := range valueSpec.Names {
		if index >= len(valueSpec.Values) {
			continue
		}

		value := literalValue(valueSpec.Values[index])
		if value == "" {
			continue
		}

		generated := generatedConstant{name: name.Name, typeName: "", value: value, file: filepath.ToSlash(path), numeric: false}
		if literal, isLiteral := valueSpec.Values[index].(*ast.BasicLit); isLiteral && literal.Kind != token.STRING {
			generated.numeric = true
		}

		if typeName != nil {
			generated.typeName = typeName.Name
		}

		constants = append(constants, generated)
	}

	return constants
}

func schemaEnumVariableNames(schema map[string]any) []string {
	rawNames, _ := schema["x-enum-varnames"].([]any)

	names := make([]string, 0, len(rawNames))

	for _, rawName := range rawNames {
		if name, ok := rawName.(string); ok {
			names = append(names, name)
		}
	}

	return names
}

func jsonProperties(structure *ast.StructType) []string {
	if structure == nil || structure.Fields == nil {
		return nil
	}

	var properties []string

	for _, field := range structure.Fields.List {
		if field.Tag == nil {
			continue
		}

		tag, err := strconv.Unquote(field.Tag.Value)
		if err != nil {
			continue
		}

		name, _, _ := strings.Cut(reflect.StructTag(tag).Get("json"), ",")
		if name != "" && name != "-" {
			properties = append(properties, name)
		}
	}

	return properties
}

func calledName(expression ast.Expr) string {
	switch function := expression.(type) {
	case *ast.SelectorExpr:
		return calledName(function.X) + "." + function.Sel.Name
	case *ast.Ident:
		return function.Name
	case *ast.IndexExpr:
		return calledName(function.X)
	case *ast.IndexListExpr:
		return calledName(function.X)
	default:
		return ""
	}
}

func operationIDFromAccessor(name string) (string, bool) {
	name, ok := strings.CutPrefix(name, "Operation")
	if !ok || name == "" {
		return "", false
	}

	return strings.ToLower(name[:1]) + name[1:], true
}

func upperFirst(value string) string {
	if value == "" {
		return ""
	}

	return strings.ToUpper(value[:1]) + value[1:]
}

func markdownCell(value string) string {
	if value == "" {
		return emptyMarkdownCell
	}

	return strings.NewReplacer("|", "\\|", "\n", " ").Replace(value)
}

func joinUseLocations(uses []modelUse) string {
	if len(uses) == 0 {
		return emptyMarkdownCell
	}

	locations := make([]string, 0, len(uses))
	for _, use := range uses {
		locations = append(locations, fmt.Sprintf("`%s:%d` (`%s`)", use.path, use.line, use.name))
	}

	sort.Strings(locations)

	return strings.Join(compactStrings(locations), ", ")
}

func sortModelUses(uses []modelUse) {
	sort.Slice(uses, func(leftIndex, rightIndex int) bool {
		if uses[leftIndex].path != uses[rightIndex].path {
			return uses[leftIndex].path < uses[rightIndex].path
		}

		if uses[leftIndex].line != uses[rightIndex].line {
			return uses[leftIndex].line < uses[rightIndex].line
		}

		return uses[leftIndex].name < uses[rightIndex].name
	})
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}

	sort.Strings(values)

	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}

	return result
}
