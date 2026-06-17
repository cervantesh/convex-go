package core

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestEncodeValue(t *testing.T) {
	got, err := EncodeValue(map[string]any{
		"n":     Int64(-1),
		"plain": int64(42),
		"bytes": []byte("abc"),
		"arr":   []any{true, "x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"n":     map[string]any{"$integer": "//////////8="},
		"plain": map[string]any{"$integer": "KgAAAAAAAAA="},
		"bytes": map[string]any{"$bytes": "YWJj"},
		"arr":   []any{true, "x"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EncodeValue mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestEncodeStructJSONTags(t *testing.T) {
	type args struct {
		Name  string `json:"name"`
		Skip  string `json:"-"`
		Empty string `json:"empty,omitempty"`
		Count int    `json:"count"`
	}
	got, err := EncodeValue(args{Name: "Ada", Skip: "no", Count: 3})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"name":  "Ada",
		"count": map[string]any{"$integer": "AwAAAAAAAAA="},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EncodeValue mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestEncodeSpecialFloat(t *testing.T) {
	got, err := EncodeValue(math.Inf(1))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]any{"$float": "AAAAAAAA8H8="}) {
		t.Fatalf("unexpected encoded inf: %#v", got)
	}
}

func TestDecodeValue(t *testing.T) {
	raw := []byte(`{"n":{"$integer":"//////////8="},"bytes":{"$bytes":"YWJj"},"f":{"$float":"AAAAAAAA8H8="},"ok":true}`)
	got, err := DecodeJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	obj := got.(map[string]any)
	if obj["n"] != Int64(-1) {
		t.Fatalf("unexpected int: %#v", obj["n"])
	}
	if string(obj["bytes"].(Bytes)) != "abc" {
		t.Fatalf("unexpected bytes: %#v", obj["bytes"])
	}
	if !math.IsInf(obj["f"].(float64), 1) {
		t.Fatalf("unexpected float: %#v", obj["f"])
	}
	if obj["ok"] != true {
		t.Fatalf("unexpected bool: %#v", obj["ok"])
	}
}

func TestDecodeInto(t *testing.T) {
	var out struct {
		Count int    `json:"count"`
		Name  string `json:"name"`
	}
	err := decodeInto(map[string]any{"count": Int64(3), "name": "Ada"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.Count != 3 || out.Name != "Ada" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func decodeInto(result any, out any) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func TestEncodeRejectsReservedField(t *testing.T) {
	_, err := EncodeValue(map[string]any{"$bad": true})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEncodeJSONProducesObject(t *testing.T) {
	got, err := EncodeJSON(map[string]any{"n": Int64(1)})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["n"].(map[string]any); !ok {
		t.Fatalf("expected encoded object, got %s", string(got))
	}
}

func TestValueWireParity(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			name:  "int64 little endian",
			value: Int64(-1),
			want:  `{"$integer":"//////////8="}`,
		},
		{
			name:  "bytes base64",
			value: Bytes("abc"),
			want:  `{"$bytes":"YWJj"}`,
		},
		{
			name:  "negative zero special float",
			value: math.Copysign(0, -1),
			want:  `{"$float":"AAAAAAAAAIA="}`,
		},
		{
			name:  "positive infinity special float",
			value: math.Inf(1),
			want:  `{"$float":"AAAAAAAA8H8="}`,
		},
		{
			name:  "negative infinity special float",
			value: math.Inf(-1),
			want:  `{"$float":"AAAAAAAA8P8="}`,
		},
		{
			name:  "regular float as JSON number",
			value: 1.5,
			want:  `1.5`,
		},
		{
			name:  "array",
			value: []any{Int64(1), "x", true},
			want:  `[{"$integer":"AQAAAAAAAAA="},"x",true]`,
		},
		{
			name: "object deterministic keys",
			value: map[string]any{
				"b": true,
				"a": Int64(1),
			},
			want: `{"a":{"$integer":"AQAAAAAAAAA="},"b":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeJSON(tt.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Fatalf("encoded mismatch:\ngot:  %s\nwant: %s", got, tt.want)
			}
		})
	}

	nanJSON, err := EncodeJSON(math.NaN())
	if err != nil {
		t.Fatal(err)
	}
	decodedNaN, err := DecodeJSON(nanJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !math.IsNaN(decodedNaN.(float64)) {
		t.Fatalf("expected NaN roundtrip, got %#v", decodedNaN)
	}
}

func TestValueWireParityRejectsUnsupportedSingletons(t *testing.T) {
	for _, raw := range []string{`{"$set":[]}`, `{"$map":[]}`} {
		t.Run(raw, func(t *testing.T) {
			if _, err := DecodeJSON([]byte(raw)); err == nil {
				t.Fatalf("expected %s to be rejected", raw)
			}
		})
	}
}

func TestValueWireParityRejectsRegularFloatInSpecialEncoding(t *testing.T) {
	_, err := DecodeJSON([]byte(`{"$float":"AAAAAAAA8D8="}`))
	if err == nil {
		t.Fatal("expected regular float encoded as $float to be rejected")
	}
}

func TestValueWireParityObjectFieldValidation(t *testing.T) {
	tooLong := strings.Repeat("a", maxIdentifierLen+1)
	tests := []map[string]any{
		{"$reserved": true},
		{"bad\nfield": true},
		{"caf\u00e9": true},
		{tooLong: true},
	}
	for _, value := range tests {
		if _, err := EncodeJSON(value); err == nil {
			t.Fatalf("expected invalid field to be rejected: %#v", value)
		}
	}
}

func TestGoIntegerMapping(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			name:  "int maps to Convex Int64 for convex-rs parity",
			value: int(10),
			want:  `{"$integer":"CgAAAAAAAAA="}`,
		},
		{
			name:  "int64 maps to Convex Int64",
			value: int64(10),
			want:  `{"$integer":"CgAAAAAAAAA="}`,
		},
		{
			name:  "uint64 maps to Convex Int64 when in range",
			value: uint64(10),
			want:  `{"$integer":"CgAAAAAAAAA="}`,
		},
		{
			name:  "float64 maps to Convex Float64 JSON number",
			value: float64(10),
			want:  `10`,
		},
		{
			name:  "Number maps to Convex Float64 JSON number for JS v.number",
			value: Number(10),
			want:  `10`,
		},
		{
			name:  "Int64 explicitly maps to Convex Int64 for JS v.int64",
			value: Int64(10),
			want:  `{"$integer":"CgAAAAAAAAA="}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeJSON(tt.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Fatalf("encoded mismatch:\ngot:  %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestGoIntegerMappingRejectsOutOfRangeUint64(t *testing.T) {
	_, err := EncodeJSON(uint64(1) << 63)
	if err == nil {
		t.Fatal("expected out-of-range uint64 to be rejected")
	}
	if !strings.Contains(err.Error(), "<root>") {
		t.Fatalf("expected root path in out-of-range uint64 error, got %v", err)
	}
}

func TestValueConformanceFixturesFromOfficialClients(t *testing.T) {
	// Sources:
	// - convex-js/src/values/value.ts: bigint, float, bytes, and field validation.
	// - convex-js/src/values/base64.ts: URL-safe base64 decode compatibility.
	// - convex-rs/src/value/json/mod.rs: encoded JSON value round trips.
	t.Run("encodes nested values with sorted object keys", func(t *testing.T) {
		got, err := EncodeJSON(map[string]any{
			"z": []any{Int64(-2), Bytes([]byte{0, 16, 131}), math.Inf(-1)},
			"a": map[string]any{"ok": true, "n": Number(12.5)},
		})
		if err != nil {
			t.Fatal(err)
		}
		want := `{"a":{"n":12.5,"ok":true},"z":[{"$integer":"/v////////8="},{"$bytes":"ABCD"},{"$float":"AAAAAAAA8P8="}]}`
		if string(got) != want {
			t.Fatalf("encoded mismatch:\ngot:  %s\nwant: %s", got, want)
		}
	})

	t.Run("decodes url safe base64 values accepted by convex js", func(t *testing.T) {
		decoded, err := DecodeJSON([]byte(`{"bytes":{"$bytes":"-_8="},"max":{"$integer":"_________38="}}`))
		if err != nil {
			t.Fatal(err)
		}
		obj := decoded.(map[string]any)
		if got := []byte(obj["bytes"].(Bytes)); !reflect.DeepEqual(got, []byte{0xfb, 0xff}) {
			t.Fatalf("unexpected URL-safe bytes decode: %#v", got)
		}
		if got := obj["max"].(Int64); got != Int64(math.MaxInt64) {
			t.Fatalf("unexpected URL-safe integer decode: %#v", got)
		}
	})

	t.Run("rejects official unsupported singleton sentinels", func(t *testing.T) {
		for _, raw := range []string{`{"$set":[]}`, `{"$map":[]}`} {
			if _, err := DecodeJSON([]byte(raw)); err == nil {
				t.Fatalf("expected %s to be rejected", raw)
			}
		}
	})
}

func TestValueConformanceAdditionalOfficialFixtures(t *testing.T) {
	// Sources:
	// - convex-js/src/values/value.ts: MIN_INT64, MAX_IDENTIFIER_LEN, and sorted object keys.
	// - convex-rs/src/value/json/mod.rs: Int64 encoded JSON round trips.
	t.Run("encodes minimum int64 little endian", func(t *testing.T) {
		got, err := EncodeJSON(Int64(math.MinInt64))
		if err != nil {
			t.Fatal(err)
		}
		want := `{"$integer":"AAAAAAAAAIA="}`
		if string(got) != want {
			t.Fatalf("encoded mismatch:\ngot:  %s\nwant: %s", got, want)
		}
		decoded, err := DecodeJSON(got)
		if err != nil {
			t.Fatal(err)
		}
		if decoded != Int64(math.MinInt64) {
			t.Fatalf("decoded mismatch: %#v", decoded)
		}
	})

	t.Run("accepts maximum official object field length", func(t *testing.T) {
		key := strings.Repeat("a", maxIdentifierLen)
		got, err := EncodeJSON(map[string]any{key: true})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), key) {
			t.Fatalf("encoded object omitted max-length key: %s", got)
		}
	})
}

func TestValueConvexPyDocumentedMapping(t *testing.T) {
	// Source: get-convex/convex-py README "Convex types" mapping. Go integers
	// intentionally follow convex-rs Int64 defaults, so JS number-style Python
	// ints are represented explicitly with Number here.
	got, err := EncodeJSON(map[string]any{
		"none":        nil,
		"boolean":     true,
		"string":      "abc",
		"numberInt":   Number(1),
		"numberFloat": float64(3.2),
		"int64":       Int64(1234),
		"bytes":       Bytes([]byte("abc")),
		"list":        []any{Number(1), Number(3.2), "abc"},
		"dict":        map[string]any{"a": "abc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"boolean":true,"bytes":{"$bytes":"YWJj"},"dict":{"a":"abc"},"int64":{"$integer":"0gQAAAAAAAA="},"list":[1,3.2,"abc"],"none":null,"numberFloat":3.2,"numberInt":1,"string":"abc"}`
	if string(got) != want {
		t.Fatalf("encoded mismatch:\ngot:  %s\nwant: %s", got, want)
	}

	decoded, err := DecodeJSON(got)
	if err != nil {
		t.Fatal(err)
	}
	obj := decoded.(map[string]any)
	if obj["none"] != nil || obj["boolean"] != true || obj["string"] != "abc" {
		t.Fatalf("unexpected scalar decode: %#v", obj)
	}
	if obj["numberInt"] != float64(1) || obj["numberFloat"] != float64(3.2) {
		t.Fatalf("unexpected number decode: %#v", obj)
	}
	if obj["int64"] != Int64(1234) {
		t.Fatalf("unexpected int64 decode: %#v", obj["int64"])
	}
	if string(obj["bytes"].(Bytes)) != "abc" {
		t.Fatalf("unexpected bytes decode: %#v", obj["bytes"])
	}
	list := obj["list"].([]any)
	if !reflect.DeepEqual(list, []any{float64(1), float64(3.2), "abc"}) {
		t.Fatalf("unexpected list decode: %#v", list)
	}
	if !reflect.DeepEqual(obj["dict"], map[string]any{"a": "abc"}) {
		t.Fatalf("unexpected dict decode: %#v", obj["dict"])
	}
}

func TestValueConstructorsAccessorsAndCopies(t *testing.T) {
	bytesValue := BytesValue([]byte("abc"))
	bytesCopy, ok := bytesValue.Bytes()
	if !ok {
		t.Fatal("expected bytes accessor to succeed")
	}
	bytesCopy[0] = 'z'
	originalBytes, _ := bytesValue.Bytes()
	if string(originalBytes) != "abc" {
		t.Fatalf("bytes accessor leaked mutable storage: %q", originalBytes)
	}

	arrayValue := ArrayValue([]Value{StringValue("one")})
	arrayCopy, ok := arrayValue.Array()
	if !ok {
		t.Fatal("expected array accessor to succeed")
	}
	arrayCopy[0] = StringValue("two")
	originalArray, _ := arrayValue.Array()
	if text, _ := originalArray[0].String(); text != "one" {
		t.Fatalf("array accessor leaked mutable storage: %#v", originalArray)
	}

	objectValue := MustObjectValue(map[string]Value{"name": StringValue("Ada")})
	objectCopy, ok := objectValue.Object()
	if !ok {
		t.Fatal("expected object accessor to succeed")
	}
	objectCopy["name"] = StringValue("Grace")
	originalObject, _ := objectValue.Object()
	if text, _ := originalObject["name"].String(); text != "Ada" {
		t.Fatalf("object accessor leaked mutable storage: %#v", originalObject)
	}

	if value, ok := Float64Value(1.25).Float64(); !ok || value != 1.25 {
		t.Fatalf("unexpected float accessor: %v %v", value, ok)
	}
	if value, ok := BooleanValue(true).Bool(); !ok || !value {
		t.Fatalf("unexpected bool accessor: %v %v", value, ok)
	}
	if value, ok := Int64Value(7).Int64(); !ok || value != 7 {
		t.Fatalf("unexpected int accessor: %v %v", value, ok)
	}
	if _, ok := StringValue("x").Float64(); ok {
		t.Fatal("wrong-kind float accessor succeeded")
	}
	if NullValue().Kind() != NullKind || (Value{}).Kind() != NullKind {
		t.Fatal("zero Value should behave as null")
	}
}

func TestValueKindNamesAndWrongKindAccessors(t *testing.T) {
	tests := []struct {
		name string
		got  ValueKind
		want ValueKind
	}{
		{name: "null", got: NullValue().Kind(), want: NullKind},
		{name: "int64", got: Int64Value(1).Kind(), want: Int64Kind},
		{name: "float64", got: Float64Value(1).Kind(), want: Float64Kind},
		{name: "boolean", got: BooleanValue(true).Kind(), want: BooleanKind},
		{name: "string", got: StringValue("x").Kind(), want: StringKind},
		{name: "bytes", got: BytesValue([]byte("x")).Kind(), want: BytesKind},
		{name: "array", got: ArrayValue([]Value{NullValue()}).Kind(), want: ArrayKind},
		{name: "object", got: MustObjectValue(map[string]Value{"x": NullValue()}).Kind(), want: ObjectKind},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Fatalf("%s kind = %q, want %q", tt.name, tt.got, tt.want)
		}
	}

	value := StringValue("not the requested kind")
	if got, ok := value.Int64(); ok || got != 0 {
		t.Fatalf("wrong-kind Int64 accessor succeeded: %v %v", got, ok)
	}
	if got, ok := value.Bool(); ok || got {
		t.Fatalf("wrong-kind Bool accessor succeeded: %v %v", got, ok)
	}
	if got, ok := BooleanValue(true).String(); ok || got != "" {
		t.Fatalf("wrong-kind String accessor succeeded: %q %v", got, ok)
	}
	if got, ok := StringValue("not a float").Float64(); ok || got != 0 {
		t.Fatalf("wrong-kind Float64 accessor succeeded: %v %v", got, ok)
	}
	if got, ok := value.Bytes(); ok || got != nil {
		t.Fatalf("wrong-kind Bytes accessor succeeded: %#v %v", got, ok)
	}
	if got, ok := value.Array(); ok || got != nil {
		t.Fatalf("wrong-kind Array accessor succeeded: %#v %v", got, ok)
	}
	if got, ok := value.Object(); ok || got != nil {
		t.Fatalf("wrong-kind Object accessor succeeded: %#v %v", got, ok)
	}
}

func TestValueConvexJSONVariants(t *testing.T) {
	value := MustObjectValue(map[string]Value{
		"nil":   NullValue(),
		"bool":  BooleanValue(true),
		"text":  StringValue("hello"),
		"int":   Int64Value(-1),
		"float": Float64Value(1.5),
		"nan":   Float64Value(math.NaN()),
		"bytes": BytesValue([]byte("abc")),
		"array": ArrayValue([]Value{StringValue("x")}),
	})
	got, err := value.ConvexJSON()
	if err != nil {
		t.Fatal(err)
	}
	obj := got.(map[string]any)
	if obj["nil"] != nil || obj["bool"] != true || obj["text"] != "hello" || obj["float"] != 1.5 {
		t.Fatalf("unexpected scalar JSON: %#v", obj)
	}
	if !reflect.DeepEqual(obj["int"], map[string]any{"$integer": "//////////8="}) {
		t.Fatalf("unexpected int JSON: %#v", obj["int"])
	}
	if !reflect.DeepEqual(obj["bytes"], map[string]any{"$bytes": "YWJj"}) {
		t.Fatalf("unexpected bytes JSON: %#v", obj["bytes"])
	}
	if _, ok := obj["nan"].(map[string]any); !ok {
		t.Fatalf("expected special float object, got %#v", obj["nan"])
	}
	if _, err := (Value{kind: ValueKind("bad")}).ConvexJSON(); err == nil {
		t.Fatal("expected unknown kind error")
	}
}

func TestEncodeValuePreservesTypedValueVariants(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  any
	}{
		{name: "typed string", value: StringValue("typed"), want: "typed"},
		{name: "typed int64", value: Int64Value(-1), want: map[string]any{"$integer": "//////////8="}},
		{name: "number wrapper", value: Number(1.25), want: 1.25},
		{name: "bytes wrapper", value: Bytes("abc"), want: map[string]any{"$bytes": "YWJj"}},
	}
	for _, tt := range tests {
		got, err := EncodeValue(tt.value)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%s encoded mismatch:\ngot:  %#v\nwant: %#v", tt.name, got, tt.want)
		}
	}
}

func TestValueConvexJSONArrayPropagatesChildErrors(t *testing.T) {
	value := ArrayValue([]Value{
		StringValue("ok"),
		{kind: ObjectKind, obj: map[string]Value{"$bad": NullValue()}},
	})
	if _, err := value.ConvexJSON(); err == nil {
		t.Fatal("expected invalid child value to fail array ConvexJSON")
	}
}

func TestValueFromGoAndDecodeValuePublicAPI(t *testing.T) {
	passthrough, err := ValueFromGo(StringValue("already-typed"))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := passthrough.String(); !ok || got != "already-typed" {
		t.Fatalf("unexpected passthrough ValueFromGo result: %#v", passthrough)
	}

	fromGo, err := ValueFromGo(map[string]any{
		"count": json.Number("5"),
		"ratio": json.Number("1.25"),
		"bytes": Bytes("abc"),
	})
	if err != nil {
		t.Fatal(err)
	}
	obj, ok := fromGo.Object()
	if !ok {
		t.Fatalf("expected object value, got %#v", fromGo)
	}
	if n, _ := obj["count"].Int64(); n != 5 {
		t.Fatalf("unexpected json.Number int mapping: %#v", obj["count"])
	}
	if f, _ := obj["ratio"].Float64(); f != 1.25 {
		t.Fatalf("unexpected json.Number float mapping: %#v", obj["ratio"])
	}

	decoded, err := DecodeValue(map[string]any{
		"items": []any{
			json.Number("2.5"),
			map[string]any{"$integer": "AQAAAAAAAAA="},
			map[string]any{"$bytes": "YWJj"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	items := decoded.(map[string]any)["items"].([]any)
	if items[0] != 2.5 || items[1] != Int64(1) || string(items[2].(Bytes)) != "abc" {
		t.Fatalf("unexpected decoded values: %#v", items)
	}

	if _, err := ValueFromGo(make(chan int)); err == nil {
		t.Fatal("expected unsupported ValueFromGo input to fail")
	}
}

func TestValueRejectsMalformedWireValues(t *testing.T) {
	tests := []any{
		map[string]any{"$bytes": 1},
		map[string]any{"$bytes": "***"},
		map[string]any{"$integer": 1},
		map[string]any{"$integer": "bad"},
		map[string]any{"$integer": "AQ=="},
		map[string]any{"$float": 1},
		map[string]any{"$float": "bad"},
		map[string]any{"$float": "AQ=="},
		map[string]any{"$set": []any{}},
		map[string]any{"$map": []any{}},
		map[string]any{"$bad": true},
		make(chan int),
		json.Number("bad"),
	}
	for _, raw := range tests {
		if _, err := DecodeValue(raw); err == nil {
			t.Fatalf("expected DecodeValue(%#v) to fail", raw)
		}
	}
}

func TestDecodeValueReportsUnsupportedSingletons(t *testing.T) {
	tests := map[string]string{
		`{"$set":[]}`: "Set",
		`{"$map":[]}`: "Map",
	}
	for raw, want := range tests {
		_, err := DecodeJSON([]byte(raw))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("DecodeJSON(%s) error = %v, want mention %q", raw, err, want)
		}
	}
}

func TestEncodeValueAdditionalTypesAndErrors(t *testing.T) {
	type embedded struct {
		Embedded string `json:"embedded"`
	}
	type payload struct {
		embedded
		Name  string         `json:"name"`
		Zero  int            `json:"zero,omitempty"`
		Items []string       `json:"items,omitempty"`
		When  json.Number    `json:"when"`
		Any   map[string]any `json:"any"`
	}
	got, err := EncodeValue(&payload{
		embedded: embedded{Embedded: "yes"},
		Name:     "Ada",
		When:     json.Number("10"),
		Any:      map[string]any{"float": json.Number("1e2")},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := got.(map[string]any)
	if _, ok := obj["zero"]; ok {
		t.Fatalf("omitempty zero field was encoded: %#v", obj)
	}
	if obj["embedded"] != "yes" || obj["name"] != "Ada" {
		t.Fatalf("unexpected struct encoding: %#v", obj)
	}

	var nilPointer *payload
	if encoded, err := EncodeValue(nilPointer); err != nil || encoded != nil {
		t.Fatalf("nil pointer encoded as %#v with err %v", encoded, err)
	}
	if _, err := EncodeValue(map[int]string{1: "bad"}); err == nil {
		t.Fatal("expected non-string map key error")
	}
	if _, err := EncodeValue(uint64(1) << 63); err == nil {
		t.Fatal("expected out-of-range uint64 error")
	}
	if _, err := EncodeValue(make(chan int)); err == nil {
		t.Fatal("expected unsupported type error")
	}
	if _, err := EncodeValue(json.Number("not-a-number")); err == nil {
		t.Fatal("expected invalid json.Number error")
	}
	if _, err := EncodeValue(struct {
		Bad chan int `json:"bad"`
	}{Bad: make(chan int)}); err == nil {
		t.Fatal("expected nested unsupported type error")
	}
}

func TestEncodeValueErrorPaths(t *testing.T) {
	_, err := EncodeValue(map[string]any{"nested": map[string]any{"bad": make(chan int)}})
	if err == nil || !strings.Contains(err.Error(), ".nested.bad") {
		t.Fatalf("expected nested map error path, got %v", err)
	}
	_, err = EncodeValue([]any{"ok", make(chan int)})
	if err == nil || !strings.Contains(err.Error(), "[1]") {
		t.Fatalf("expected array index error path, got %v", err)
	}
}

func TestEncodeStructOmitsZeroKinds(t *testing.T) {
	type EmbeddedString string
	type payload struct {
		EmbeddedString
		Bool   bool           `json:"bool,omitempty"`
		Uint   uint           `json:"uint,omitempty"`
		Float  float64        `json:"float,omitempty"`
		Ptr    *int           `json:"ptr,omitempty"`
		Array  [0]int         `json:"array,omitempty"`
		Map    map[string]any `json:"map,omitempty"`
		String string         `json:"string,omitempty"`
		Keep   string         `json:"keep"`
	}
	got, err := EncodeValue(payload{EmbeddedString: "embedded", Keep: "yes"})
	if err != nil {
		t.Fatal(err)
	}
	obj := got.(map[string]any)
	if !reflect.DeepEqual(obj, map[string]any{"EmbeddedString": "embedded", "keep": "yes"}) {
		t.Fatalf("unexpected omitted-zero struct: %#v", obj)
	}
	if _, err := EncodeValue(struct {
		Bad string `json:"$bad"`
	}{Bad: "x"}); err == nil {
		t.Fatal("expected invalid struct field name error")
	}
}

func TestEncodeStructOmitZeroTagAndEmbeddedErrors(t *testing.T) {
	type badEmbedded struct {
		Bad chan int `json:"bad"`
	}
	type payload struct {
		ZeroString string `json:"zeroString,omitzero"`
		KeepString string `json:"keepString,omitzero"`
		badEmbedded
	}
	_, err := EncodeValue(payload{
		KeepString:  "keep",
		badEmbedded: badEmbedded{Bad: make(chan int)},
	})
	if err == nil || !strings.Contains(err.Error(), ".bad") {
		t.Fatalf("expected embedded field error path, got %v", err)
	}

	got, err := EncodeValue(struct {
		ZeroString string `json:"zeroString,omitzero"`
		KeepString string `json:"keepString,omitzero"`
	}{KeepString: "keep"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]any{"keepString": "keep"}) {
		t.Fatalf("unexpected omitzero encoding: %#v", got)
	}
}

func TestEncodeJSONNumberOverflowFallsBackToFloat(t *testing.T) {
	got, err := EncodeValue(json.Number("9223372036854775808"))
	if err != nil {
		t.Fatal(err)
	}
	if got != float64(9223372036854775808) {
		t.Fatalf("unexpected overflow json.Number encoding: %#v", got)
	}
}

func TestEncodeJSONNumberClassifiesIntegerDecimalAndExponent(t *testing.T) {
	tests := []struct {
		name  string
		input json.Number
		want  any
	}{
		{name: "integer", input: json.Number("10"), want: map[string]any{"$integer": "CgAAAAAAAAA="}},
		{name: "negative integer", input: json.Number("-1"), want: map[string]any{"$integer": "//////////8="}},
		{name: "decimal", input: json.Number("1.25"), want: 1.25},
		{name: "exponent", input: json.Number("1e3"), want: float64(1000)},
	}
	for _, tt := range tests {
		got, err := EncodeValue(tt.input)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%s encoded mismatch:\ngot:  %#v\nwant: %#v", tt.name, got, tt.want)
		}
	}
}

func TestValueJSONMethodsAndMustObjectValue(t *testing.T) {
	value := MustObjectValue(map[string]Value{"ok": BooleanValue(true)})
	data, err := value.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var decoded Value
	if err := decoded.UnmarshalJSON(data); err != nil {
		t.Fatal(err)
	}
	obj, ok := decoded.Object()
	if !ok {
		t.Fatalf("expected object after unmarshal, got %#v", decoded)
	}
	if okValue, _ := obj["ok"].Bool(); !okValue {
		t.Fatalf("unexpected decoded object: %#v", obj)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("expected MustObjectValue to panic on invalid fields")
		}
	}()
	_ = MustObjectValue(map[string]Value{"$reserved": BooleanValue(true)})
}

func TestObjectFieldValidationBoundaries(t *testing.T) {
	maxKey := strings.Repeat("a", maxIdentifierLen)
	if _, err := ObjectValue(map[string]Value{maxKey: NullValue(), "~": StringValue("tilde")}); err != nil {
		t.Fatalf("expected max-length ASCII object fields to be valid: %v", err)
	}
	for _, key := range []string{
		strings.Repeat("a", maxIdentifierLen+1),
		"$reserved",
		"bad\nfield",
		string([]byte{127}),
	} {
		if _, err := ObjectValue(map[string]Value{key: NullValue()}); err == nil {
			t.Fatalf("expected object field %q to be invalid", key)
		}
	}
}
