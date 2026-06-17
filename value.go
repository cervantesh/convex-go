package convex

import (
	"fmt"

	"github.com/cervantesh/convex-go/internal/core"
)

// Int64 forces a Go integer to be encoded as a Convex Int64.
type Int64 = core.Int64

// Number forces a Go numeric value to be encoded as a Convex Float64, which is
// the value expected by JavaScript validators such as v.number().
type Number = core.Number

// Bytes forces a byte slice to be encoded as Convex Bytes.
type Bytes = core.Bytes

// ValueKind identifies a Convex value variant.
type ValueKind = core.ValueKind

const (
	NullKind    = core.NullKind
	Int64Kind   = core.Int64Kind
	Float64Kind = core.Float64Kind
	BooleanKind = core.BooleanKind
	StringKind  = core.StringKind
	BytesKind   = core.BytesKind
	ArrayKind   = core.ArrayKind
	ObjectKind  = core.ObjectKind
)

// Value is the Go equivalent of convex-rs' Value enum.
type Value = core.Value

// NullValue returns a Convex null.
func NullValue() Value {
	return core.NullValue()
}

// Int64Value returns a Convex Int64.
func Int64Value(n int64) Value {
	return core.Int64Value(n)
}

// Float64Value returns a Convex Float64.
func Float64Value(f float64) Value {
	return core.Float64Value(f)
}

// BooleanValue returns a Convex boolean.
func BooleanValue(b bool) Value {
	return core.BooleanValue(b)
}

// StringValue returns a Convex string.
func StringValue(s string) Value {
	return core.StringValue(s)
}

// BytesValue returns Convex Bytes.
func BytesValue(b []byte) Value {
	return core.BytesValue(b)
}

// ArrayValue returns a Convex array.
func ArrayValue(values []Value) Value {
	return core.ArrayValue(values)
}

// ObjectValue returns a Convex object after validating field names.
func ObjectValue(fields map[string]Value) (Value, error) {
	return core.ObjectValue(fields)
}

// MustObjectValue is a convenience for package-level examples and tests.
func MustObjectValue(fields map[string]Value) Value {
	return core.MustObjectValue(fields)
}

// ValueFromGo converts ordinary Go values into a typed Convex Value.
func ValueFromGo(value any) (Value, error) {
	return core.ValueFromGo(value)
}

// ParseValueJSON parses Convex encoded JSON into a typed Convex Value.
func ParseValueJSON(data []byte) (Value, error) {
	return core.ParseValueJSON(data)
}

// EncodeValue converts a Go value to Convex's encoded JSON wire shape.
func EncodeValue(value any) (any, error) {
	return core.EncodeValue(value)
}

// EncodeJSON encodes a Go value to Convex's encoded JSON.
func EncodeJSON(value any) ([]byte, error) {
	return core.EncodeJSON(value)
}

// DecodeJSON decodes Convex encoded JSON into a Go value.
func DecodeJSON(data []byte) (any, error) {
	return core.DecodeJSON(data)
}

// DecodeValue decodes a Convex encoded JSON wire shape into a Go value.
func DecodeValue(value any) (any, error) {
	return core.DecodeValue(value)
}

func encodeArgs(args any) (map[string]any, error) {
	if args == nil {
		return map[string]any{}, nil
	}
	encoded, err := core.EncodeValue(args)
	if err != nil {
		return nil, err
	}
	obj, ok := encoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("convex: function arguments must encode to an object, got %T", encoded)
	}
	return obj, nil
}
