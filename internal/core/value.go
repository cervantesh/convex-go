package core

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

const maxIdentifierLen = 1024

const (
	wireIntegerKey          = "$integer"
	wireFloatKey            = "$float"
	wireBytesKey            = "$bytes"
	invalidJSONNumberFormat = "convex: invalid JSON number %q: %w"
)

// Int64 forces a Go integer to be encoded as a Convex Int64.
type Int64 int64

// Number forces a Go numeric value to be encoded as a Convex Float64, which is
// the value expected by JavaScript validators such as v.number().
type Number float64

// Bytes forces a byte slice to be encoded as Convex Bytes.
type Bytes []byte

// ValueKind identifies a Convex value variant.
type ValueKind string

const (
	NullKind    ValueKind = "null"
	Int64Kind   ValueKind = "int64"
	Float64Kind ValueKind = "float64"
	BooleanKind ValueKind = "boolean"
	StringKind  ValueKind = "string"
	BytesKind   ValueKind = "bytes"
	ArrayKind   ValueKind = "array"
	ObjectKind  ValueKind = "object"
)

// Value is the Go equivalent of convex-rs' Value enum. It represents exactly
// the values Convex can accept as function arguments or return from functions.
type Value struct {
	kind  ValueKind
	i64   int64
	f64   float64
	b     bool
	s     string
	bytes []byte
	array []Value
	obj   map[string]Value
}

// NullValue returns a Convex null.
func NullValue() Value {
	return Value{kind: NullKind}
}

// Int64Value returns a Convex Int64.
func Int64Value(n int64) Value {
	return Value{kind: Int64Kind, i64: n}
}

// Float64Value returns a Convex Float64.
func Float64Value(f float64) Value {
	return Value{kind: Float64Kind, f64: f}
}

// BooleanValue returns a Convex boolean.
func BooleanValue(b bool) Value {
	return Value{kind: BooleanKind, b: b}
}

// StringValue returns a Convex string.
func StringValue(s string) Value {
	return Value{kind: StringKind, s: s}
}

// BytesValue returns Convex Bytes.
func BytesValue(b []byte) Value {
	cp := append([]byte(nil), b...)
	return Value{kind: BytesKind, bytes: cp}
}

// ArrayValue returns a Convex array.
func ArrayValue(values []Value) Value {
	cp := append([]Value(nil), values...)
	return Value{kind: ArrayKind, array: cp}
}

// ObjectValue returns a Convex object after validating field names.
func ObjectValue(fields map[string]Value) (Value, error) {
	cp := make(map[string]Value, len(fields))
	for k, v := range fields {
		if err := validateObjectField(k); err != nil {
			return Value{}, err
		}
		cp[k] = v
	}
	return Value{kind: ObjectKind, obj: cp}, nil
}

// MustObjectValue is a convenience for package-level examples and tests.
func MustObjectValue(fields map[string]Value) Value {
	v, err := ObjectValue(fields)
	if err != nil {
		panic(err)
	}
	return v
}

// ValueFromGo converts ordinary Go values into a typed Convex Value.
func ValueFromGo(value any) (Value, error) {
	if v, ok := value.(Value); ok {
		return v, nil
	}
	wire, err := EncodeValue(value)
	if err != nil {
		return Value{}, err
	}
	return valueFromWire(wire)
}

// ParseValueJSON parses Convex encoded JSON into a typed Convex Value.
func ParseValueJSON(data []byte) (Value, error) {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		return Value{}, err
	}
	return valueFromWire(raw)
}

// Kind returns the value variant. The zero Value is treated as null.
func (v Value) Kind() ValueKind {
	if v.kind == "" {
		return NullKind
	}
	return v.kind
}

// ConvexJSON returns the Convex encoded JSON wire representation.
func (v Value) ConvexJSON() (any, error) {
	switch v.Kind() {
	case NullKind:
		return nil, nil
	case Int64Kind:
		return encodedInt64(v.i64), nil
	case Float64Kind:
		return encodedFloat64(v.f64), nil
	case BooleanKind:
		return v.b, nil
	case StringKind:
		return v.s, nil
	case BytesKind:
		return encodedBytes(v.bytes), nil
	case ArrayKind:
		return convexJSONArray(v.array)
	case ObjectKind:
		return convexJSONObject(v.obj)
	default:
		return nil, fmt.Errorf("convex: unknown value kind %q", v.kind)
	}
}

func convexJSONArray(values []Value) ([]any, error) {
	out := make([]any, len(values))
	for i, item := range values {
		encoded, err := item.ConvexJSON()
		if err != nil {
			return nil, err
		}
		out[i] = encoded
	}
	return out, nil
}

func convexJSONObject(values map[string]Value) (map[string]any, error) {
	out := make(map[string]any, len(values))
	for key, item := range values {
		if err := validateObjectField(key); err != nil {
			return nil, err
		}
		encoded, err := item.ConvexJSON()
		if err != nil {
			return nil, err
		}
		out[key] = encoded
	}
	return out, nil
}

func encodedInt64(n int64) map[string]any {
	return map[string]any{wireIntegerKey: encodeInt64(n)}
}

func encodedFloat64(f float64) any {
	if isSpecialFloat(f) {
		return map[string]any{wireFloatKey: encodeFloat64(f)}
	}
	return f
}

func encodedBytes(data []byte) map[string]any {
	return map[string]any{wireBytesKey: base64.StdEncoding.EncodeToString(data)}
}

// GoValue returns an idiomatic Go representation of the Convex value.
func (v Value) GoValue() any {
	switch v.Kind() {
	case NullKind:
		return nil
	case Int64Kind:
		return Int64(v.i64)
	case Float64Kind:
		return v.f64
	case BooleanKind:
		return v.b
	case StringKind:
		return v.s
	case BytesKind:
		return Bytes(append([]byte(nil), v.bytes...))
	case ArrayKind:
		out := make([]any, len(v.array))
		for i, item := range v.array {
			out[i] = item.GoValue()
		}
		return out
	case ObjectKind:
		out := make(map[string]any, len(v.obj))
		for k, item := range v.obj {
			out[k] = item.GoValue()
		}
		return out
	default:
		return nil
	}
}

// Int64 returns the Int64 value and whether this Value has Int64 kind.
func (v Value) Int64() (int64, bool) {
	if v.Kind() != Int64Kind {
		return 0, false
	}
	return v.i64, true
}

// Float64 returns the Float64 value and whether this Value has Float64 kind.
func (v Value) Float64() (float64, bool) {
	if v.Kind() != Float64Kind {
		return 0, false
	}
	return v.f64, true
}

// Bool returns the boolean value and whether this Value has Boolean kind.
func (v Value) Bool() (bool, bool) {
	if v.Kind() != BooleanKind {
		return false, false
	}
	return v.b, true
}

// String returns the string value and whether this Value has String kind.
func (v Value) String() (string, bool) {
	if v.Kind() != StringKind {
		return "", false
	}
	return v.s, true
}

// Bytes returns a copy of the byte slice and whether this Value has Bytes kind.
func (v Value) Bytes() ([]byte, bool) {
	if v.Kind() != BytesKind {
		return nil, false
	}
	return append([]byte(nil), v.bytes...), true
}

// Array returns a copy of the array and whether this Value has Array kind.
func (v Value) Array() ([]Value, bool) {
	if v.Kind() != ArrayKind {
		return nil, false
	}
	return append([]Value(nil), v.array...), true
}

// Object returns a copy of the object and whether this Value has Object kind.
func (v Value) Object() (map[string]Value, bool) {
	if v.Kind() != ObjectKind {
		return nil, false
	}
	out := make(map[string]Value, len(v.obj))
	for k, value := range v.obj {
		out[k] = value
	}
	return out, true
}

// MarshalJSON marshals the Convex encoded JSON wire representation.
func (v Value) MarshalJSON() ([]byte, error) {
	wire, err := v.ConvexJSON()
	if err != nil {
		return nil, err
	}
	return json.Marshal(wire)
}

// UnmarshalJSON unmarshals Convex encoded JSON into Value.
func (v *Value) UnmarshalJSON(data []byte) error {
	parsed, err := ParseValueJSON(data)
	if err != nil {
		return err
	}
	*v = parsed
	return nil
}

// EncodeValue converts a Go value to Convex's JSON wire representation.
func EncodeValue(value any) (any, error) {
	return encodeValue(reflect.ValueOf(value), "")
}

// EncodeJSON converts a Go value to Convex's JSON wire representation and
// marshals the result to JSON.
func EncodeJSON(value any) ([]byte, error) {
	encoded, err := EncodeValue(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(encoded)
}

// DecodeJSON converts Convex's JSON wire representation into Go values.
func DecodeJSON(data []byte) (any, error) {
	value, err := ParseValueJSON(data)
	if err != nil {
		return nil, err
	}
	return value.GoValue(), nil
}

// DecodeValue converts an already-unmarshaled Convex JSON wire value into Go
// values. Convex Int64 values become Int64 and Convex Bytes become Bytes.
func DecodeValue(value any) (any, error) {
	return decodeValue(value)
}

func encodeValue(v reflect.Value, path string) (any, error) {
	v = indirectValue(v)
	if !v.IsValid() {
		return nil, nil
	}
	if encoded, ok, err := encodeSpecialValue(v); ok || err != nil {
		return encoded, err
	}
	return encodeReflectValue(v, path)
}

func indirectValue(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

func encodeSpecialValue(v reflect.Value) (any, bool, error) {
	if !v.CanInterface() {
		return nil, false, nil
	}
	switch x := v.Interface().(type) {
	case Value:
		encoded, err := x.ConvexJSON()
		return encoded, true, err
	case Number:
		return encodedFloat64(float64(x)), true, nil
	case Int64:
		return encodedInt64(int64(x)), true, nil
	case Bytes:
		return encodedBytes([]byte(x)), true, nil
	case []byte:
		return encodedBytes(x), true, nil
	case json.Number:
		encoded, err := encodeJSONNumber(x)
		return encoded, true, err
	default:
		return nil, false, nil
	}
}

func encodeReflectValue(v reflect.Value, path string) (any, error) {
	switch v.Kind() {
	case reflect.Bool:
		return v.Bool(), nil
	case reflect.String:
		return v.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return encodedInt64(v.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return encodeUint(v.Uint(), path)
	case reflect.Float32, reflect.Float64:
		return encodedFloat64(v.Convert(reflect.TypeOf(float64(0))).Float()), nil
	case reflect.Slice, reflect.Array:
		return encodeSequence(v, path)
	case reflect.Map:
		return encodeMap(v, path)
	case reflect.Struct:
		return encodeStruct(v, path)
	default:
		return nil, unsupportedValueError(v, path)
	}
}

func encodeUint(value uint64, path string) (any, error) {
	if value > math.MaxInt64 {
		return nil, fmt.Errorf("convex: unsigned integer at %s does not fit into a Convex Int64", printablePath(path))
	}
	return encodedInt64(int64(value)), nil
}

func encodeSequence(v reflect.Value, path string) ([]any, error) {
	out := make([]any, v.Len())
	for i := 0; i < v.Len(); i++ {
		encoded, err := encodeValue(v.Index(i), fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return nil, err
		}
		out[i] = encoded
	}
	return out, nil
}

func unsupportedValueError(v reflect.Value, path string) error {
	if path == "" {
		return fmt.Errorf("convex: unsupported value type %s", v.Type())
	}
	return fmt.Errorf("convex: unsupported value type %s at %s", v.Type(), path)
}

func valueFromWire(value any) (Value, error) {
	switch x := value.(type) {
	case nil:
		return NullValue(), nil
	case bool:
		return BooleanValue(x), nil
	case string:
		return StringValue(x), nil
	case json.Number:
		return valueFromJSONNumber(x)
	case float64:
		return Float64Value(x), nil
	case []any:
		return valueArrayFromWire(x)
	case map[string]any:
		return valueObjectFromWire(x)
	default:
		return Value{}, fmt.Errorf("convex: unexpected JSON value type %T", value)
	}
}

func valueFromJSONNumber(n json.Number) (Value, error) {
	f, err := n.Float64()
	if err != nil {
		return Value{}, invalidJSONNumberError(n, err)
	}
	return Float64Value(f), nil
}

func valueArrayFromWire(values []any) (Value, error) {
	out := make([]Value, len(values))
	for i, item := range values {
		decoded, err := valueFromWire(item)
		if err != nil {
			return Value{}, err
		}
		out[i] = decoded
	}
	return ArrayValue(out), nil
}

func valueObjectFromWire(values map[string]any) (Value, error) {
	if len(values) == 1 {
		for key, raw := range values {
			decoded, ok, err := valueFromWireSingleton(key, raw)
			if ok || err != nil {
				return decoded, err
			}
		}
	}
	out := make(map[string]Value, len(values))
	for key, item := range values {
		if err := validateObjectField(key); err != nil {
			return Value{}, err
		}
		decoded, err := valueFromWire(item)
		if err != nil {
			return Value{}, err
		}
		out[key] = decoded
	}
	return ObjectValue(out)
}

func valueFromWireSingleton(key string, raw any) (Value, bool, error) {
	switch key {
	case wireBytesKey:
		data, err := decodeWireBytes(raw)
		return BytesValue(data), true, err
	case wireIntegerKey:
		n, err := decodeWireInt64(raw)
		return Int64Value(n), true, err
	case wireFloatKey:
		f, err := decodeWireFloat64(raw)
		if err != nil {
			return Value{}, true, err
		}
		if !isSpecialFloat(f) {
			return Value{}, true, fmt.Errorf("convex: float %v should be encoded as a JSON number", f)
		}
		return Float64Value(f), true, nil
	case "$set":
		return Value{}, true, errors.New("convex: received a Set, which is no longer supported")
	case "$map":
		return Value{}, true, errors.New("convex: received a Map, which is no longer supported")
	default:
		return Value{}, false, nil
	}
}

func invalidJSONNumberError(n json.Number, err error) error {
	return fmt.Errorf(invalidJSONNumberFormat, n.String(), err)
}

func encodeJSONNumber(n json.Number) (any, error) {
	if strings.ContainsAny(n.String(), ".eE") {
		f, err := strconv.ParseFloat(n.String(), 64)
		if err != nil {
			return nil, invalidJSONNumberError(n, err)
		}
		return encodedFloat64(f), nil
	}
	i, err := strconv.ParseInt(n.String(), 10, 64)
	if err == nil {
		return encodedInt64(i), nil
	}
	f, ferr := strconv.ParseFloat(n.String(), 64)
	if ferr != nil {
		return nil, invalidJSONNumberError(n, err)
	}
	return f, nil
}

func encodeMap(v reflect.Value, path string) (map[string]any, error) {
	if v.IsNil() {
		return nil, nil
	}
	if v.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("convex: object at %s must have string keys", printablePath(path))
	}
	out := make(map[string]any, v.Len())
	keys := v.MapKeys()
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	for _, key := range keys {
		k := key.String()
		if err := validateObjectField(k); err != nil {
			return nil, err
		}
		childPath := "." + k
		if path != "" {
			childPath = path + childPath
		}
		encoded, err := encodeValue(v.MapIndex(key), childPath)
		if err != nil {
			return nil, err
		}
		out[k] = encoded
	}
	return out, nil
}

func encodeStruct(v reflect.Value, path string) (map[string]any, error) {
	out := map[string]any{}
	for i := 0; i < v.NumField(); i++ {
		if err := encodeStructField(out, v, path, i); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func encodeStructField(out map[string]any, v reflect.Value, path string, index int) error {
	field := v.Type().Field(index)
	if field.PkgPath != "" && !field.Anonymous {
		return nil
	}
	name, omitEmpty, skip := jsonField(field)
	if skip {
		return nil
	}
	fieldValue := v.Field(index)
	if omitEmpty && isEmptyValue(fieldValue) {
		return nil
	}
	if field.Anonymous && name == "" {
		merged, err := mergeEmbeddedStructField(out, fieldValue, path)
		if err != nil || merged {
			return err
		}
	}
	if name == "" {
		name = field.Name
	}
	return encodeNamedStructField(out, name, fieldValue, childPath(path, name))
}

func mergeEmbeddedStructField(out map[string]any, fieldValue reflect.Value, path string) (bool, error) {
	embedded, err := encodeValue(fieldValue, path)
	if err != nil {
		return false, err
	}
	embeddedMap, ok := embedded.(map[string]any)
	if !ok {
		return false, nil
	}
	for key, value := range embeddedMap {
		out[key] = value
	}
	return true, nil
}

func encodeNamedStructField(out map[string]any, name string, fieldValue reflect.Value, path string) error {
	if err := validateObjectField(name); err != nil {
		return err
	}
	encoded, err := encodeValue(fieldValue, path)
	if err != nil {
		return err
	}
	out[name] = encoded
	return nil
}

func childPath(path, name string) string {
	child := "." + name
	if path != "" {
		return path + child
	}
	return child
}

func jsonField(field reflect.StructField) (name string, omitEmpty bool, skip bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	parts := strings.Split(tag, ",")
	if parts[0] != "" {
		name = parts[0]
	}
	for _, opt := range parts[1:] {
		if opt == "omitempty" || opt == "omitzero" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, false
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	default:
		return false
	}
}

func decodeValue(value any) (any, error) {
	switch x := value.(type) {
	case nil, bool, string:
		return x, nil
	case json.Number:
		return decodeJSONNumber(x)
	case float64:
		return x, nil
	case []any:
		return decodeArrayValue(x)
	case map[string]any:
		return decodeObjectValue(x)
	default:
		return nil, fmt.Errorf("convex: unexpected JSON value type %T", value)
	}
}

func decodeJSONNumber(n json.Number) (float64, error) {
	f, err := n.Float64()
	if err != nil {
		return 0, invalidJSONNumberError(n, err)
	}
	return f, nil
}

func decodeArrayValue(values []any) ([]any, error) {
	out := make([]any, len(values))
	for i, item := range values {
		decoded, err := decodeValue(item)
		if err != nil {
			return nil, err
		}
		out[i] = decoded
	}
	return out, nil
}

func decodeObjectValue(values map[string]any) (any, error) {
	if len(values) == 1 {
		for key, raw := range values {
			decoded, ok, err := decodeWireSingleton(key, raw)
			if ok || err != nil {
				return decoded, err
			}
		}
	}
	out := make(map[string]any, len(values))
	for key, item := range values {
		if err := validateObjectField(key); err != nil {
			return nil, err
		}
		decoded, err := decodeValue(item)
		if err != nil {
			return nil, err
		}
		out[key] = decoded
	}
	return out, nil
}

func decodeWireSingleton(key string, raw any) (any, bool, error) {
	switch key {
	case wireBytesKey:
		data, err := decodeWireBytes(raw)
		return Bytes(data), true, err
	case wireIntegerKey:
		n, err := decodeWireInt64(raw)
		return Int64(n), true, err
	case wireFloatKey:
		f, err := decodeWireFloat64(raw)
		return f, true, err
	case "$set":
		return nil, true, errors.New("convex: received a Set, which is no longer supported")
	case "$map":
		return nil, true, errors.New("convex: received a Map, which is no longer supported")
	default:
		return nil, false, nil
	}
}

func decodeWireBytes(raw any) ([]byte, error) {
	s, err := wireString(raw, wireBytesKey)
	if err != nil {
		return nil, err
	}
	data, err := decodeBase64Compat(s)
	if err != nil {
		return nil, fmt.Errorf("convex: malformed %s value: %w", wireBytesKey, err)
	}
	return data, nil
}

func decodeWireInt64(raw any) (int64, error) {
	s, err := wireString(raw, wireIntegerKey)
	if err != nil {
		return 0, err
	}
	return decodeInt64(s)
}

func decodeWireFloat64(raw any) (float64, error) {
	s, err := wireString(raw, wireFloatKey)
	if err != nil {
		return 0, err
	}
	return decodeFloat64(s)
}

func wireString(raw any, key string) (string, error) {
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("convex: malformed %s field", key)
	}
	return s, nil
}

func validateObjectField(k string) error {
	if len(k) > maxIdentifierLen {
		return fmt.Errorf("convex: field name %q exceeds maximum field name length %d", k, maxIdentifierLen)
	}
	if strings.HasPrefix(k, "$") {
		return fmt.Errorf("convex: field name %q starts with a '$', which is reserved", k)
	}
	for i := 0; i < len(k); i++ {
		if k[i] < 32 || k[i] >= 127 {
			return fmt.Errorf("convex: field name %q has invalid character %q; field names must be non-control ASCII", k, k[i])
		}
	}
	return nil
}

func encodeInt64(n int64) string {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(n))
	return base64.StdEncoding.EncodeToString(buf[:])
}

func decodeInt64(encoded string) (int64, error) {
	data, err := decodeBase64Compat(encoded)
	if err != nil {
		return 0, fmt.Errorf("convex: malformed $integer value: %w", err)
	}
	if len(data) != 8 {
		return 0, fmt.Errorf("convex: received %d bytes, expected 8 for $integer", len(data))
	}
	return int64(binary.LittleEndian.Uint64(data)), nil
}

func encodeFloat64(f float64) string {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], math.Float64bits(f))
	return base64.StdEncoding.EncodeToString(buf[:])
}

func decodeFloat64(encoded string) (float64, error) {
	data, err := decodeBase64Compat(encoded)
	if err != nil {
		return 0, fmt.Errorf("convex: malformed $float value: %w", err)
	}
	if len(data) != 8 {
		return 0, fmt.Errorf("convex: received %d bytes, expected 8 for $float", len(data))
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(data)), nil
}

func decodeBase64Compat(encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err == nil {
		return data, nil
	}
	if data, urlErr := base64.URLEncoding.DecodeString(encoded); urlErr == nil {
		return data, nil
	}
	return nil, err
}

func isSpecialFloat(f float64) bool {
	return math.IsNaN(f) || math.IsInf(f, 0) || (f == 0 && math.Signbit(f))
}

func printablePath(path string) string {
	if path == "" {
		return "<root>"
	}
	return path
}
