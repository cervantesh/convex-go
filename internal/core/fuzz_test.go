package core

import (
	"bytes"
	"encoding/json"
	"testing"
)

func FuzzParseFunctionPath(f *testing.F) {
	for _, seed := range []string{
		"messages:list",
		"messages",
		"dir/messages.ts:list",
		"components/search:run",
		"",
		":" ,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		normalized, err := ParseFunctionPath(input)
		if err != nil {
			return
		}
		if normalized == "" {
			t.Fatal("ParseFunctionPath returned an empty normalized path")
		}
		if _, err := ParseUDFPath(normalized); err != nil {
			t.Fatalf("ParseFunctionPath(%q) returned invalid normalized path %q: %v", input, normalized, err)
		}
	})
}

func FuzzValueJSONRoundTrip(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`null`),
		[]byte(`{"$integer":"AQAAAAAAAAA="}`),
		[]byte(`{"$float":"AAAAAAAA8D8="}`),
		[]byte(`{"payload":{"$bytes":"YWJj"},"nested":[true,false,null]}`),
		[]byte(`{"name":"Ada","active":true,"count":{"$integer":"AgAAAAAAAAA="}}`),
		[]byte(`{"bad":{"$float":1}}`),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		value, err := ParseValueJSON(raw)
		if err != nil {
			return
		}

		encoded, err := value.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON failed after ParseValueJSON(%q): %v", raw, err)
		}
		if _, err := DecodeJSON(encoded); err != nil {
			t.Fatalf("DecodeJSON failed after MarshalJSON round-trip: %v", err)
		}

		var reparsed Value
		if err := reparsed.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("UnmarshalJSON failed after MarshalJSON round-trip: %v", err)
		}
		reencoded, err := reparsed.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON failed after UnmarshalJSON round-trip: %v", err)
		}

		if !bytes.Equal(compactJSONCore(t, encoded), compactJSONCore(t, reencoded)) {
			t.Fatalf("value round-trip changed encoding\nencoded=%s\nreencoded=%s", encoded, reencoded)
		}
	})
}

func compactJSONCore(t *testing.T, raw []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		t.Fatalf("json.Compact(%q): %v", raw, err)
	}
	return append([]byte(nil), buf.Bytes()...)
}
