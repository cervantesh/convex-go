package core

import (
	"fmt"
	"path"
	"strings"
)

const maxIdentifierLength = 64

// ValidateIdentifier validates a Convex identifier.
func ValidateIdentifier(s string) error {
	if isValidIdentifier(s) {
		return nil
	}
	if s == "" {
		return fmt.Errorf("convex: identifier cannot be empty")
	}
	if len(s) > maxIdentifierLength {
		return fmt.Errorf("convex: identifier %q is too long (%d > %d)", s, len(s), maxIdentifierLength)
	}
	first := s[0]
	if !isASCIIAlpha(first) && first != '_' {
		return fmt.Errorf("convex: invalid first character %q in identifier %q", first, s)
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !isASCIIAlpha(c) && !isASCIIDigit(c) && c != '_' {
			return fmt.Errorf("convex: invalid character %q in identifier %q", c, s)
		}
	}
	if strings.Trim(s, "_") == "" {
		return fmt.Errorf("convex: identifier %q cannot contain only underscores", s)
	}
	return nil
}

func isValidIdentifier(s string) bool {
	if s == "" || len(s) > maxIdentifierLength {
		return false
	}
	if !isASCIIAlpha(s[0]) && s[0] != '_' {
		return false
	}
	hasNonUnderscore := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isASCIIAlpha(c) || isASCIIDigit(c) {
			hasNonUnderscore = true
			continue
		}
		if c != '_' {
			return false
		}
	}
	return hasNonUnderscore
}

// FunctionName is an exported function name within a Convex module.
type FunctionName string

// ParseFunctionName validates a function name.
func ParseFunctionName(s string) (FunctionName, error) {
	if err := ValidateIdentifier(s); err != nil {
		return "", err
	}
	return FunctionName(s), nil
}

// ModulePath is a user-specified Convex module path.
type ModulePath struct {
	raw string
}

// ParseModulePath validates a module path. Paths may omit the .js extension.
func ParseModulePath(s string) (ModulePath, error) {
	if s == "" {
		return ModulePath{}, fmt.Errorf("convex: module path cannot be empty")
	}
	if strings.HasPrefix(s, "/") || path.IsAbs(s) {
		return ModulePath{}, fmt.Errorf("convex: module paths must be relative")
	}
	cleaned := path.Clean(strings.ReplaceAll(s, "\\", "/"))
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." || strings.Contains(cleaned, "/../") {
		return ModulePath{}, fmt.Errorf("convex: invalid module path %q", s)
	}
	parts := strings.Split(cleaned, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return ModulePath{}, fmt.Errorf("convex: invalid path component %q in %q", part, s)
		}
		if err := validatePathComponent(part); err != nil {
			return ModulePath{}, err
		}
	}
	if ext := path.Ext(cleaned); ext != "" && ext != ".js" {
		return ModulePath{}, fmt.Errorf("convex: module path %q has extension %q; expected .js", s, ext)
	}
	return ModulePath{raw: cleaned}, nil
}

func (p ModulePath) String() string {
	return p.raw
}

// IsSystem reports whether the module lives under _system.
func (p ModulePath) IsSystem() bool {
	return strings.HasPrefix(p.raw, "_system/") || p.raw == "_system" || p.raw == "_system.js"
}

// IsDeps reports whether the module lives under _deps.
func (p ModulePath) IsDeps() bool {
	return strings.HasPrefix(p.raw, "_deps/") || p.raw == "_deps" || strings.HasPrefix(p.raw, "actions/_deps/")
}

// IsHTTP reports whether this is the deployment HTTP router module.
func (p ModulePath) IsHTTP() bool {
	return p.Canonicalize().raw == "http.js"
}

// IsCron reports whether this is the deployment crons module.
func (p ModulePath) IsCron() bool {
	return p.Canonicalize().raw == "crons.js"
}

// Canonicalize returns the module path with an explicit .js extension.
func (p ModulePath) Canonicalize() CanonicalizedModulePath {
	if path.Ext(p.raw) == "" {
		return CanonicalizedModulePath{raw: p.raw + ".js"}
	}
	return CanonicalizedModulePath(p)
}

// CanonicalizedModulePath is a module path with an explicit .js extension.
type CanonicalizedModulePath struct {
	raw string
}

func (p CanonicalizedModulePath) String() string {
	return p.raw
}

// UDFPath is a user-specified Convex function path, e.g. "messages:list".
type UDFPath struct {
	Module   ModulePath
	Function *FunctionName
}

// ParseUDFPath parses a Convex UDF path. If no function is specified, the
// default export is implied during canonicalization.
func ParseUDFPath(s string) (UDFPath, error) {
	modulePart := s
	var fn *FunctionName
	if index := strings.LastIndex(s, ":"); index >= 0 {
		modulePart = s[:index]
		after := s[index+1:]
		parsedFn, err := ParseFunctionName(after)
		if err != nil {
			return UDFPath{}, err
		}
		fn = &parsedFn
	}
	module, err := ParseModulePath(modulePart)
	if err != nil {
		return UDFPath{}, err
	}
	return UDFPath{Module: module, Function: fn}, nil
}

func (p UDFPath) String() string {
	if p.Function == nil {
		return p.Module.String()
	}
	return p.Module.String() + ":" + string(*p.Function)
}

// Canonicalize returns a one-to-one canonical function path.
func (p UDFPath) Canonicalize() CanonicalizedUDFPath {
	fn := FunctionName("default")
	if p.Function != nil {
		fn = *p.Function
	}
	return CanonicalizedUDFPath{
		Module:   p.Module.Canonicalize(),
		Function: fn,
	}
}

// CanonicalizedUDFPath is a UDF path with explicit module extension and
// function name.
type CanonicalizedUDFPath struct {
	Module   CanonicalizedModulePath
	Function FunctionName
}

func (p CanonicalizedUDFPath) String() string {
	return p.Module.String() + ":" + string(p.Function)
}

// ParseFunctionPath normalizes common Convex function name forms. It supports
// "api.foo.bar" and "internal.foo.bar" from generated JS APIs, plus
// "foo/bar:baz" and "foo/bar".
func ParseFunctionPath(s string) (string, error) {
	if strings.HasPrefix(s, "api.") || strings.HasPrefix(s, "internal.") {
		parts := strings.Split(s, ".")
		if len(parts) < 3 {
			return "", fmt.Errorf("convex: function name has too few parts: %q", s)
		}
		exportName := parts[len(parts)-1]
		moduleName := strings.Join(parts[1:len(parts)-1], "/")
		normalized := moduleName + ":" + exportName
		if _, err := ParseUDFPath(normalized); err != nil {
			return "", err
		}
		return normalized, nil
	}
	filePath, exportName, _ := strings.Cut(s, ":")
	exportName = strings.TrimSpace(exportName)
	if exportName == "" {
		exportName = "default"
	}
	for _, ext := range []string{".ts", ".tsx", ".jsx", ".mts", ".mjs", ".cts", ".cjs"} {
		filePath = strings.TrimSuffix(filePath, ext)
	}
	normalized := filePath + ":" + exportName
	if _, err := ParseUDFPath(normalized); err != nil {
		return "", err
	}
	return normalized, nil
}

func validatePathComponent(s string) error {
	if len(s) > maxIdentifierLength {
		return fmt.Errorf("convex: path component %q is too long (%d > %d)", s, len(s), maxIdentifierLength)
	}
	hasAlnum := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isASCIIAlpha(c) || isASCIIDigit(c) {
			hasAlnum = true
			continue
		}
		if c != '_' && c != '.' {
			return fmt.Errorf("convex: path component %q can only contain alphanumeric characters, underscores, or periods", s)
		}
	}
	if !hasAlnum {
		return fmt.Errorf("convex: path component %q must contain at least one alphanumeric character", s)
	}
	return nil
}

func isASCIIAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isASCIIDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
