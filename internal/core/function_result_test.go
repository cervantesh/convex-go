package core

import (
	"errors"
	"strings"
	"testing"
)

func TestFunctionResultVariants(t *testing.T) {
	valueResult := ValueResult(Int64Value(42))
	if !valueResult.IsValue() {
		t.Fatal("value result should report IsValue")
	}
	if value, ok := valueResult.Value(); !ok || value.GoValue() != Int64(42) {
		t.Fatalf("unexpected value result: %#v ok=%v", value, ok)
	}
	if _, ok := valueResult.ErrorMessage(); ok {
		t.Fatal("value result should not expose an error message")
	}
	if _, ok := valueResult.ConvexError(); ok {
		t.Fatal("value result should not expose a ConvexError")
	}
	if err := valueResult.Err(); err != nil {
		t.Fatalf("value result should not convert to error: %v", err)
	}

	messageResult := ErrorMessageResult("failed")
	if messageResult.IsValue() {
		t.Fatal("error message result should not report IsValue")
	}
	if message, ok := messageResult.ErrorMessage(); !ok || message != "failed" {
		t.Fatalf("unexpected error message result: %q ok=%v", message, ok)
	}
	if value, ok := messageResult.Value(); ok || value.Kind() != NullKind {
		t.Fatalf("error message result should not expose a value: %#v ok=%v", value, ok)
	}
	if _, ok := messageResult.ConvexError(); ok {
		t.Fatal("error message result should not expose a ConvexError")
	}
	var functionErr *FunctionError
	if err := messageResult.Err(); !errors.As(err, &functionErr) || functionErr.Message != "failed" {
		t.Fatalf("expected FunctionError, got %T: %v", err, err)
	}

	convexResult := ConvexErrorResult(ConvexError{Message: "bad", Data: Int64Value(1)})
	if value, ok := convexResult.Value(); ok || value.Kind() != NullKind {
		t.Fatalf("ConvexError result should not expose a value: %#v ok=%v", value, ok)
	}
	if _, ok := convexResult.ErrorMessage(); ok {
		t.Fatal("ConvexError result should not expose a plain error message")
	}
	convexErr, ok := convexResult.ConvexError()
	if !ok || convexErr.Message != "bad" {
		t.Fatalf("unexpected ConvexError result: %#v ok=%v", convexErr, ok)
	}
	var asConvexErr *ConvexError
	if err := convexResult.Err(); !errors.As(err, &asConvexErr) || asConvexErr.Message != "bad" {
		t.Fatalf("expected ConvexError, got %T: %v", err, err)
	}
}

func TestFunctionKindConstantsMatchConvexHTTPKinds(t *testing.T) {
	tests := map[FunctionKind]string{
		QueryKind:    "query",
		MutationKind: "mutation",
		ActionKind:   "action",
	}
	for kind, want := range tests {
		if string(kind) != want {
			t.Fatalf("unexpected function kind: got %q want %q", kind, want)
		}
	}
}

func TestFunctionAndConvexErrorStrings(t *testing.T) {
	if got := (&ConvexError{Message: "bad"}).Error(); got != "bad" {
		t.Fatalf("unexpected ConvexError string: %q", got)
	}
	if got := (&FunctionError{Message: "failed"}).Error(); got != "convex: function failed: failed" {
		t.Fatalf("unexpected FunctionError without path: %q", got)
	}
	if got := (&FunctionError{Kind: QueryKind, Path: "messages:list", Message: "failed"}).Error(); !strings.Contains(got, `query "messages:list" failed`) {
		t.Fatalf("unexpected FunctionError with path: %q", got)
	}
	if unwrap := (&FunctionError{}).Unwrap(); unwrap != nil {
		t.Fatalf("unexpected unwrap without ConvexError: %#v", unwrap)
	}
	convexErr := &ConvexError{Message: "bad"}
	if unwrap := (&FunctionError{Convex: convexErr}).Unwrap(); unwrap != convexErr {
		t.Fatalf("unexpected unwrap: %#v", unwrap)
	}
}
