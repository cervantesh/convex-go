package codegen

import (
	"regexp"
	"strings"
	"unicode"
)

var handlerPattern = regexp.MustCompile(`(?s)handler\s*:\s*(?:async\s*)?.*?=>`)

func inferResultType(original string, stripped string, callOpen int) string {
	callClose := findMatchingDelimiter(stripped, callOpen, '(', ')')
	if callClose < 0 || callClose <= callOpen {
		return ""
	}
	callOriginal := original[callOpen+1 : callClose]
	callStripped := stripped[callOpen+1 : callClose]
	handler := handlerPattern.FindStringIndex(callStripped)
	if handler == nil {
		return ""
	}
	return inferHandlerResultType(callOriginal[handler[1]:], callStripped[handler[1]:])
}

func inferHandlerResultType(original string, stripped string) string {
	original, stripped = trimLeadingSpace(original, stripped)
	if stripped == "" {
		if startsWithStringLiteral(original) {
			return "string"
		}
		return ""
	}
	if stripped[0] == '{' {
		return inferBlockReturnType(original, stripped)
	}
	return inferExpressionType(original, stripped)
}

func inferBlockReturnType(original string, stripped string) string {
	blockClose := findMatchingDelimiter(stripped, 0, '{', '}')
	if blockClose < 0 {
		return ""
	}
	bodyOriginal := original[1:blockClose]
	bodyStripped := stripped[1:blockClose]
	returnIndex := strings.Index(bodyStripped, "return")
	if returnIndex < 0 {
		return ""
	}
	return inferExpressionType(bodyOriginal[returnIndex+len("return"):], bodyStripped[returnIndex+len("return"):])
}

func inferExpressionType(original string, stripped string) string {
	original, stripped = trimLeadingSpace(original, stripped)
	if stripped == "" {
		if startsWithStringLiteral(original) {
			return "string"
		}
		return ""
	}
	switch {
	case strings.HasPrefix(stripped, "true"), strings.HasPrefix(stripped, "false"):
		return "bool"
	case strings.HasPrefix(stripped, "null"), strings.HasPrefix(stripped, "undefined"):
		return ""
	case strings.HasPrefix(stripped, "["):
		if findMatchingDelimiter(stripped, 0, '[', ']') >= 0 {
			return "[]any"
		}
	case strings.HasPrefix(stripped, "{"):
		if findMatchingDelimiter(stripped, 0, '{', '}') >= 0 {
			return "map[string]any"
		}
	case strings.HasPrefix(stripped, "("):
		closeIndex := findMatchingDelimiter(stripped, 0, '(', ')')
		if closeIndex < 0 {
			return ""
		}
		return inferExpressionType(original[1:closeIndex], stripped[1:closeIndex])
	case startsWithStringLiteral(original):
		return "string"
	case startsWithNumberLiteral(stripped):
		return "float64"
	}
	return ""
}

func trimLeadingSpace(original string, stripped string) (string, string) {
	return strings.TrimLeftFunc(original, unicode.IsSpace), strings.TrimLeftFunc(stripped, unicode.IsSpace)
}

func startsWithStringLiteral(original string) bool {
	if original == "" {
		return false
	}
	switch original[0] {
	case '"', '\'', '`':
		return true
	default:
		return false
	}
}

func startsWithNumberLiteral(stripped string) bool {
	if stripped == "" {
		return false
	}
	if stripped[0] >= '0' && stripped[0] <= '9' {
		return true
	}
	if (stripped[0] == '+' || stripped[0] == '-') && len(stripped) > 1 {
		next := stripped[1]
		return (next >= '0' && next <= '9') || next == '.'
	}
	return stripped[0] == '.' && len(stripped) > 1 && stripped[1] >= '0' && stripped[1] <= '9'
}

func findMatchingDelimiter(source string, openIndex int, open byte, close byte) int {
	if openIndex < 0 || openIndex >= len(source) || source[openIndex] != open {
		return -1
	}
	depth := 0
	for i := openIndex; i < len(source); i++ {
		switch source[i] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
