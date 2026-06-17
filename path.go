package convex

import "github.com/cervantesh/convex-go/internal/core"

// FunctionKind names a Convex function class.
type FunctionKind = core.FunctionKind

const (
	QueryKind    = core.QueryKind
	MutationKind = core.MutationKind
	ActionKind   = core.ActionKind
)

// FunctionName is an exported function name within a Convex module.
type FunctionName = core.FunctionName

// ParseFunctionName validates a function name.
func ParseFunctionName(s string) (FunctionName, error) {
	return core.ParseFunctionName(s)
}

// ModulePath is a user-specified Convex module path.
type ModulePath = core.ModulePath

// ParseModulePath validates a module path. Paths may omit the .js extension.
func ParseModulePath(s string) (ModulePath, error) {
	return core.ParseModulePath(s)
}

// CanonicalizedModulePath is a module path with an explicit .js extension.
type CanonicalizedModulePath = core.CanonicalizedModulePath

// UDFPath is a user-specified Convex function path, e.g. "messages:list".
type UDFPath = core.UDFPath

// ParseUDFPath parses a Convex UDF path.
func ParseUDFPath(s string) (UDFPath, error) {
	return core.ParseUDFPath(s)
}

// CanonicalizedUDFPath is a UDF path with explicit module extension and
// function name.
type CanonicalizedUDFPath = core.CanonicalizedUDFPath

// ParseFunctionPath normalizes common Convex function name forms.
func ParseFunctionPath(s string) (string, error) {
	return core.ParseFunctionPath(s)
}

// ValidateIdentifier validates a Convex identifier.
func ValidateIdentifier(s string) error {
	return core.ValidateIdentifier(s)
}
