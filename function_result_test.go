package convex

import (
	"errors"
	"strings"
	"testing"
)

func TestFunctionResultAccessorsAndErrors(t *testing.T) {
	valueResult := ValueResult(StringValue("ok"))
	if err := valueResult.Err(); err != nil {
		t.Fatalf("value result returned error: %v", err)
	}
	if _, ok := valueResult.ErrorMessage(); ok {
		t.Fatal("value result should not expose an error message")
	}
	if _, ok := valueResult.ConvexError(); ok {
		t.Fatal("value result should not expose a ConvexError")
	}

	messageResult := ErrorMessageResult("plain failure")
	if message, ok := messageResult.ErrorMessage(); !ok || message != "plain failure" {
		t.Fatalf("unexpected error message: %q %v", message, ok)
	}
	if _, ok := messageResult.Value(); ok {
		t.Fatal("error result should not expose a value")
	}
	if err := messageResult.Err(); err == nil || !strings.Contains(err.Error(), "plain failure") {
		t.Fatalf("unexpected plain error: %v", err)
	}

	convexResult := ConvexErrorResult(ConvexError{Message: "bad", Data: Int64Value(1)})
	err := convexResult.Err()
	var convexErr *ConvexError
	if !errors.As(err, &convexErr) {
		t.Fatalf("expected ConvexError, got %T: %v", err, err)
	}
	if convexErr.Error() != "bad" {
		t.Fatalf("unexpected ConvexError string: %q", convexErr.Error())
	}
}

func TestHTTPAndFunctionErrorStrings(t *testing.T) {
	if got := (&HTTPError{StatusCode: 401}).Error(); got != "convex: http request failed with status 401" {
		t.Fatalf("unexpected HTTPError without body: %q", got)
	}
	if got := (&HTTPError{StatusCode: 500, Body: "broken"}).Error(); !strings.Contains(got, "broken") {
		t.Fatalf("unexpected HTTPError with body: %q", got)
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
}

func TestSyncAuthErrorString(t *testing.T) {
	err := (&SyncAuthError{Message: "expired"}).Error()
	if err != "convex: auth error: expired" {
		t.Fatalf("unexpected SyncAuthError string: %q", err)
	}
}
