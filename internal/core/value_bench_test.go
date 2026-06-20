package core

import "testing"

var benchmarkValuePayload = map[string]any{
	"room":  "lobby",
	"body":  "hello from convex-go",
	"count": Int64(42),
	"score": Number(3.14159),
	"meta": map[string]any{
		"seen":  true,
		"bytes": Bytes([]byte("bench")),
		"tags":  []any{"a", "b", "c"},
	},
}

func BenchmarkValueJSONRoundTrip(b *testing.B) {
	encoded, err := EncodeJSON(benchmarkValuePayload)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(encoded)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := EncodeJSON(benchmarkValuePayload)
		if err != nil {
			b.Fatal(err)
		}
		decoded, err := DecodeJSON(data)
		if err != nil {
			b.Fatal(err)
		}
		if decoded == nil {
			b.Fatal("decoded benchmark payload must not be nil")
		}
	}
}
