package core

import (
	"math"
	"strings"
	"testing"
)

func TestValueConformanceSpecialFloatFixturesFromConvexJS(t *testing.T) {
	// Source: convex-js/src/values/value.ts isSpecial, jsonToConvex, and
	// convexToJsonInternal. Special floats use $float on the wire; ordinary
	// numbers must remain JSON numbers.
	t.Run("negative zero keeps special float encoding and sign bit", func(t *testing.T) {
		encoded, err := EncodeJSON(math.Copysign(0, -1))
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) != `{"$float":"AAAAAAAAAIA="}` {
			t.Fatalf("encoded mismatch:\ngot:  %s\nwant: %s", encoded, `{"$float":"AAAAAAAAAIA="}`)
		}

		decoded, err := DecodeJSON(encoded)
		if err != nil {
			t.Fatal(err)
		}
		value := decoded.(float64)
		if value != 0 || !math.Signbit(value) {
			t.Fatalf("expected negative zero round trip, got %#v", value)
		}
	})

	t.Run("positive infinity keeps special float encoding", func(t *testing.T) {
		decoded, err := DecodeJSON([]byte(`{"$float":"AAAAAAAA8H8="}`))
		if err != nil {
			t.Fatal(err)
		}
		if value := decoded.(float64); !math.IsInf(value, 1) {
			t.Fatalf("expected positive infinity, got %#v", value)
		}
	})

	t.Run("ordinary floats reject special float encoding", func(t *testing.T) {
		_, err := DecodeJSON([]byte(`{"$float":"AAAAAAAA+D8="}`))
		if err == nil {
			t.Fatal("expected ordinary float in $float wrapper to be rejected")
		}
		if !strings.Contains(err.Error(), "should be encoded as a number") {
			t.Fatalf("unexpected error for ordinary float wrapper: %v", err)
		}
	})
}
