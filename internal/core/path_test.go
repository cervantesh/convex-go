package core

import (
	"strings"
	"testing"
)

func TestParseUDFPathCanonicalize(t *testing.T) {
	p, err := ParseUDFPath("messages:list")
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Canonicalize().String(); got != "messages.js:list" {
		t.Fatalf("unexpected canonical path: %q", got)
	}

	p, err = ParseUDFPath("messages")
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Canonicalize().String(); got != "messages.js:default" {
		t.Fatalf("unexpected canonical path: %q", got)
	}
}

func TestParseFunctionPath(t *testing.T) {
	tests := map[string]string{
		"api.messages.list":        "messages:list",
		"api.messages.thread.list": "messages/thread:list",
		"internal.admin.runThing":  "admin:runThing",
		"foo/bar.ts:baz":           "foo/bar:baz",
		"foo/bar":                  "foo/bar:default",
	}
	for input, want := range tests {
		got, err := ParseFunctionPath(input)
		if err != nil {
			t.Fatalf("ParseFunctionPath(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("ParseFunctionPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseFunctionPathStripsSupportedExtensions(t *testing.T) {
	for _, ext := range []string{".ts", ".tsx", ".jsx", ".mts", ".mjs", ".cts", ".cjs"} {
		input := "dir/messages" + ext + ":list"
		got, err := ParseFunctionPath(input)
		if err != nil {
			t.Fatalf("ParseFunctionPath(%q): %v", input, err)
		}
		if got != "dir/messages:list" {
			t.Fatalf("ParseFunctionPath(%q) = %q, want dir/messages:list", input, got)
		}
	}
}

func TestValidateIdentifier(t *testing.T) {
	if err := ValidateIdentifier("list_messages2"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "1bad", "bad-name", "__"} {
		if err := ValidateIdentifier(bad); err == nil {
			t.Fatalf("expected %q to be invalid", bad)
		}
	}
}

func TestValidateIdentifierBoundaryCharacters(t *testing.T) {
	valid := []string{
		"a",
		"A",
		"_a",
		"a_",
		"z9",
		strings.Repeat("a", maxIdentifierLength),
	}
	for _, input := range valid {
		if err := ValidateIdentifier(input); err != nil {
			t.Fatalf("expected identifier %q to be valid: %v", input, err)
		}
	}

	invalid := []string{
		"_",
		"__",
		"9bad",
		"bad.name",
		"bad-name",
		strings.Repeat("a", maxIdentifierLength+1),
	}
	for _, input := range invalid {
		if err := ValidateIdentifier(input); err == nil {
			t.Fatalf("expected identifier %q to be invalid", input)
		}
	}
}

func TestModulePathClassifiersAndStrings(t *testing.T) {
	module, err := ParseModulePath("actions/_deps/lib")
	if err != nil {
		t.Fatal(err)
	}
	if module.String() != "actions/_deps/lib" || !module.IsDeps() {
		t.Fatalf("unexpected deps module: %#v", module)
	}
	if system, _ := ParseModulePath("_system/foo"); !system.IsSystem() {
		t.Fatalf("expected system module: %#v", system)
	}
	if httpModule, _ := ParseModulePath("http"); !httpModule.IsHTTP() {
		t.Fatalf("expected http module: %#v", httpModule)
	}
	if cronModule, _ := ParseModulePath("crons.js"); !cronModule.IsCron() {
		t.Fatalf("expected cron module: %#v", cronModule)
	}

	udf, err := ParseUDFPath("messages:list")
	if err != nil {
		t.Fatal(err)
	}
	if udf.String() != "messages:list" {
		t.Fatalf("unexpected UDF string: %q", udf.String())
	}
	if udf.Canonicalize().String() != "messages.js:list" {
		t.Fatalf("unexpected canonical UDF string: %q", udf.Canonicalize().String())
	}
}

func TestModulePathSystemAndDepsClassifiers(t *testing.T) {
	system := map[string]bool{
		"_system":         true,
		"_system.js":      true,
		"_system/foo":     true,
		"_systematic/foo": false,
		"api/_system":     false,
	}
	for input, want := range system {
		module, err := ParseModulePath(input)
		if err != nil {
			t.Fatalf("ParseModulePath(%q): %v", input, err)
		}
		if got := module.IsSystem(); got != want {
			t.Fatalf("ModulePath(%q).IsSystem() = %v, want %v", input, got, want)
		}
	}

	deps := map[string]bool{
		"_deps":             true,
		"_deps/lib":         true,
		"actions/_deps/lib": true,
		"actions/deps/lib":  false,
		"api/_deps":         false,
	}
	for input, want := range deps {
		module, err := ParseModulePath(input)
		if err != nil {
			t.Fatalf("ParseModulePath(%q): %v", input, err)
		}
		if got := module.IsDeps(); got != want {
			t.Fatalf("ModulePath(%q).IsDeps() = %v, want %v", input, got, want)
		}
	}
}

func TestParsePathsRejectInvalidForms(t *testing.T) {
	tooLong := strings.Repeat("a", maxIdentifierLength+1)
	for _, input := range []string{"/absolute", "../parent", "foo/bar.ts", tooLong, "foo/..", "foo/__", "foo/bad-name", "foo/bad name"} {
		if _, err := ParseModulePath(input); err == nil {
			t.Fatalf("expected ParseModulePath(%q) to fail", input)
		}
	}
	for _, input := range []string{"api.tooFew", "internal.tooFew", "foo/bar:bad-name", "foo/..:default", "foo/bar:__"} {
		if _, err := ParseFunctionPath(input); err == nil {
			t.Fatalf("expected ParseFunctionPath(%q) to fail", input)
		}
	}
}
