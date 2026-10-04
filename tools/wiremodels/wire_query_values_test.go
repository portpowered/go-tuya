package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestWireQueryValuesRejectFixedKeysAcrossFileHelpers(t *testing.T) {
	t.Parallel()

	files := parseWireQueryValuesSources(t, map[string]string{
		"first.go": `package sample
import "net/url"
func mutateQuery(values url.Values) { values.Set("unregistered-query", "value") }
func mutateIndexedQuery(values url.Values) { alias := values; alias["unregistered-index"] = []string{"value"} }
func fixedQueryLiteral() url.Values { return url.Values{"unregistered-literal": []string{"value"}} }
`,
		"second.go": `package sample
import "net/url"
func callQueryHelpers(values url.Values) { mutateQuery(values); mutateIndexedQuery(values) }
`,
	})

	set := token.NewFileSet()

	err := checkWireSourcePackage(files, set, map[string]generatedModel{})
	if err == nil {
		t.Fatal("checkWireSourcePackage() accepted fixed query keys")
	}

	for _, key := range []string{"unregistered-query", "unregistered-index", "unregistered-literal"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("checkWireSourcePackage() error %q does not include %q", err, key)
		}
	}
}

func TestWireQueryValuesPreserveCallerOpenKeysAndValues(t *testing.T) {
	t.Parallel()

	files := parseWireQueryValuesSources(t, map[string]string{
		"open.go": `package sample
import "net/url"
type QueryValues = url.Values
func mutateCallerQuery(values QueryValues, key, value string) {
	values.Set(key, value)
	values[key] = []string{value}
}
`,
	})

	err := checkWireSourcePackage(files, token.NewFileSet(), map[string]generatedModel{})
	if err != nil {
		t.Fatalf("checkWireSourcePackage() rejected caller-owned query keys: %v", err)
	}
}

func TestWireQueryValuesRejectNamedResultAndGlobalMapKeys(t *testing.T) {
	t.Parallel()

	files := parseWireQueryValuesSources(t, map[string]string{
		"named.go": `package sample
import "net/url"
var globalQuery = url.Values{}
func buildQuery() (values url.Values) {
	values = url.Values{}
	values["unregistered-named-result"] = []string{"value"}
	return
}
func mutateGlobalQuery() { globalQuery["unregistered-global"] = []string{"value"} }
`,
	})

	err := checkWireSourcePackage(files, token.NewFileSet(), map[string]generatedModel{})
	if err == nil {
		t.Fatal("checkWireSourcePackage() accepted fixed named-result or global query keys")
	}

	for _, key := range []string{"unregistered-named-result", "unregistered-global"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("checkWireSourcePackage() error %q does not include %q", err, key)
		}
	}
}

func parseWireQueryValuesSources(t *testing.T, sources map[string]string) []wireSourceFile {
	t.Helper()

	set := token.NewFileSet()

	files := make([]wireSourceFile, 0, len(sources))

	for path, source := range sources {
		file, err := parser.ParseFile(set, path, source, 0)
		if err != nil {
			t.Fatalf("parser.ParseFile(%q): %v", path, err)
		}

		files = append(files, wireSourceFile{path: path, file: file})
	}

	return files
}
