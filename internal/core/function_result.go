package core

import "fmt"

// FunctionKind names a Convex function class.
type FunctionKind string

const (
	QueryKind    FunctionKind = "query"
	MutationKind FunctionKind = "mutation"
	ActionKind   FunctionKind = "action"
)

// ConvexError is an application-level error thrown by a Convex function with
// ConvexError on the server.
type ConvexError struct {
	Message string
	Data    Value
}

func (e *ConvexError) Error() string {
	return e.Message
}

// FunctionError describes an error returned by a Convex query, mutation, or
// action.
type FunctionError struct {
	Kind        FunctionKind
	Path        string
	Message     string
	Data        any
	DataValue   Value
	Convex      *ConvexError
	HasData     bool
	LogLines    []string
	StatusCode  int
	RawResponse string
}

func (e *FunctionError) Error() string {
	if e.Path == "" {
		return "convex: function failed: " + e.Message
	}
	return fmt.Sprintf("convex: %s %q failed: %s", e.Kind, e.Path, e.Message)
}

// Unwrap exposes application-level ConvexError values to errors.As.
func (e *FunctionError) Unwrap() error {
	if e.Convex != nil {
		return e.Convex
	}
	return nil
}

// FunctionResult mirrors convex-rs' FunctionResult for users building higher
// level clients, subscription managers, or language bindings.
type FunctionResult struct {
	value        Value
	errorMessage string
	convexError  *ConvexError
}

// ValueResult creates a successful function result.
func ValueResult(value Value) FunctionResult {
	return FunctionResult{value: value}
}

// ErrorMessageResult creates a non-application error result.
func ErrorMessageResult(message string) FunctionResult {
	return FunctionResult{errorMessage: message}
}

// ConvexErrorResult creates an application-level ConvexError result.
func ConvexErrorResult(err ConvexError) FunctionResult {
	return FunctionResult{convexError: &err}
}

// IsValue reports whether the result is a successful value.
func (r FunctionResult) IsValue() bool {
	return r.errorMessage == "" && r.convexError == nil
}

// Value returns the successful value and whether it is present.
func (r FunctionResult) Value() (Value, bool) {
	if !r.IsValue() {
		return Value{}, false
	}
	return r.value, true
}

// ErrorMessage returns the plain error message and whether it is present.
func (r FunctionResult) ErrorMessage() (string, bool) {
	if r.errorMessage == "" {
		return "", false
	}
	return r.errorMessage, true
}

// ConvexError returns the application error and whether it is present.
func (r FunctionResult) ConvexError() (*ConvexError, bool) {
	if r.convexError == nil {
		return nil, false
	}
	return r.convexError, true
}

// Err converts non-value results into Go errors.
func (r FunctionResult) Err() error {
	if r.IsValue() {
		return nil
	}
	if r.convexError != nil {
		return r.convexError
	}
	return &FunctionError{Message: r.errorMessage}
}
