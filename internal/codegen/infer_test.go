package codegen

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateInfersSafeLiteralResultTypes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "queries.ts"), `
export const emptyList = query({
  handler: async () => [],
});

export const status = query({
  handler: async () => true,
});

export const profile = query({
  handler: async () => ({ ok: true }),
});

export const greeting = query({
  handler: async () => "hello",
});

export const count = query({
  handler: async () => 42,
});

export const blockList = query({
  handler: async () => {
    return [];
  },
});

export const fallback = query({
  handler: async () => loadFromDB(),
});
`)

	got, err := Generate(Config{
		SourceDir:   dir,
		PackageName: "convexapi",
		ImportPath:  "github.com/cervantesh/convex-go",
	})
	if err != nil {
		t.Fatal(err)
	}
	body := string(got)

	for _, want := range []string{
		`var QueriesBlockList = convex.NewQueryReference[map[string]any, []any]("queries:blockList")`,
		`var QueriesCount = convex.NewQueryReference[map[string]any, float64]("queries:count")`,
		`var QueriesEmptyList = convex.NewQueryReference[map[string]any, []any]("queries:emptyList")`,
		`var QueriesGreeting = convex.NewQueryReference[map[string]any, string]("queries:greeting")`,
		`var QueriesProfile = convex.NewQueryReference[map[string]any, map[string]any]("queries:profile")`,
		`var QueriesStatus = convex.NewQueryReference[map[string]any, bool]("queries:status")`,
		`var QueriesFallback = convex.NewQueryReference[map[string]any, any]("queries:fallback")`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("generated output missing %q:\n%s", want, body)
		}
	}
}

func TestInferResultTypeFallbacksStayGeneric(t *testing.T) {
	source := `
export const fallback = query({
  handler: async () => computeResult(),
});
`
	stripped := stripCommentsAndStrings(source)
	match := exportedFunctionPattern.FindStringSubmatchIndex(stripped)
	if match == nil {
		t.Fatal("expected export match")
	}
	if got := inferResultType(source, stripped, match[1]-1); got != "" {
		t.Fatalf("expected generic fallback, got %q", got)
	}
}

func TestInferHelpers(t *testing.T) {
	if got := inferExpressionType(`"hello"`, `"     "`); got != "string" {
		t.Fatalf("blank stripped string expression = %q", got)
	}
	if got := inferExpressionType(`"hello"`, `"hello"`); got != "string" {
		t.Fatalf("string expression = %q", got)
	}
	if got := inferExpressionType(`42`, `42`); got != "float64" {
		t.Fatalf("numeric expression = %q", got)
	}
	if got := inferExpressionType(`({ ok: true })`, `({ ok: true })`); got != "map[string]any" {
		t.Fatalf("object expression = %q", got)
	}
	if got := inferExpressionType(`[]`, `[]`); got != "[]any" {
		t.Fatalf("array expression = %q", got)
	}
	if got := inferExpressionType(`true`, `true`); got != "bool" {
		t.Fatalf("bool expression = %q", got)
	}
	if got := inferExpressionType(`null`, `null`); got != "" {
		t.Fatalf("null expression = %q", got)
	}
	if got := inferBlockReturnType("{ return []; }", "{ return []; }"); got != "[]any" {
		t.Fatalf("block return = %q", got)
	}
	if got := findMatchingDelimiter("({[]})", 0, '(', ')'); got != 5 {
		t.Fatalf("matching delimiter = %d", got)
	}
	if got := findMatchingDelimiter("(", 0, '(', ')'); got != -1 {
		t.Fatalf("unterminated delimiter = %d", got)
	}
}
