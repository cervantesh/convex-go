package convex

import "github.com/cervantesh/convex-go/internal/core"

// FunctionResult mirrors convex-rs' FunctionResult for users building higher
// level clients, subscription managers, or language bindings.
type FunctionResult = core.FunctionResult

// ValueResult creates a successful function result.
func ValueResult(value Value) FunctionResult {
	return core.ValueResult(value)
}

// ErrorMessageResult creates a non-application error result.
func ErrorMessageResult(message string) FunctionResult {
	return core.ErrorMessageResult(message)
}

// ConvexErrorResult creates an application-level ConvexError result.
func ConvexErrorResult(err ConvexError) FunctionResult {
	return core.ConvexErrorResult(err)
}
