package syncprotocol

import (
	"bytes"
	"encoding/json"
	"testing"
)

func FuzzDecodeClientMessage(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte("{\"type\":\"Event\",\"eventType\":\"Trace\",\"event\":{\"span\":\"abc\"}}"),
		[]byte("{\"type\":\"Mutation\",\"requestId\":3,\"udfPath\":\"messages:send\",\"args\":[{\"body\":\"hello\"}]}"),
		[]byte("{\"type\":\"Authenticate\",\"baseVersion\":5,\"tokenType\":\"User\",\"value\":\"jwt\"}"),
		[]byte("{\"type\":\"Unknown\"}"),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		msg, err := DecodeClientMessage(raw)
		if err != nil {
			return
		}

		encoded, err := EncodeClientMessage(msg)
		if err != nil {
			t.Fatalf("EncodeClientMessage failed after DecodeClientMessage(%q): %v", raw, err)
		}
		reparsed, err := DecodeClientMessage(encoded)
		if err != nil {
			t.Fatalf("DecodeClientMessage failed after encode round-trip: %v", err)
		}
		reencoded, err := EncodeClientMessage(reparsed)
		if err != nil {
			t.Fatalf("EncodeClientMessage failed after decode round-trip: %v", err)
		}

		if !bytes.Equal(compactJSONSync(t, encoded), compactJSONSync(t, reencoded)) {
			t.Fatalf("client message round-trip changed encoding\nencoded=%s\nreencoded=%s", encoded, reencoded)
		}
	})
}

func FuzzDecodeServerMessage(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte("{\"type\":\"Ping\"}"),
		[]byte("{\"type\":\"AuthError\",\"error\":\"bad auth\",\"baseVersion\":10,\"authUpdateAttempted\":false}"),
		[]byte("{\"type\":\"MutationResponse\",\"requestId\":8,\"success\":true,\"result\":{\"ok\":true},\"ts\":\"AQAAAAAAAAA=\",\"logLines\":[\"wire\"]}"),
		[]byte("{\"type\":\"Unknown\"}"),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		msg, err := DecodeServerMessage(raw)
		if err != nil {
			return
		}

		encoded, err := EncodeServerMessage(msg)
		if err != nil {
			t.Fatalf("EncodeServerMessage failed after DecodeServerMessage(%q): %v", raw, err)
		}
		reparsed, err := DecodeServerMessage(encoded)
		if err != nil {
			t.Fatalf("DecodeServerMessage failed after encode round-trip: %v", err)
		}
		reencoded, err := EncodeServerMessage(reparsed)
		if err != nil {
			t.Fatalf("EncodeServerMessage failed after decode round-trip: %v", err)
		}

		if !bytes.Equal(compactJSONSync(t, encoded), compactJSONSync(t, reencoded)) {
			t.Fatalf("server message round-trip changed encoding\nencoded=%s\nreencoded=%s", encoded, reencoded)
		}
	})
}

func FuzzSyncTimestampJSON(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte("\"AQAAAAAAAAA=\""),
		[]byte("\"_________38=\""),
		[]byte("\"bad\""),
		[]byte("1"),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		var ts SyncTimestamp
		if err := json.Unmarshal(raw, &ts); err != nil {
			return
		}

		encoded, err := json.Marshal(ts)
		if err != nil {
			t.Fatalf("json.Marshal failed after SyncTimestamp unmarshal: %v", err)
		}
		var reparsed SyncTimestamp
		if err := json.Unmarshal(encoded, &reparsed); err != nil {
			t.Fatalf("json.Unmarshal failed after SyncTimestamp marshal round-trip: %v", err)
		}
		if ts != reparsed {
			t.Fatalf("SyncTimestamp round-trip changed value: got %v want %v", reparsed, ts)
		}
	})
}

func compactJSONSync(t *testing.T, raw []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		t.Fatalf("json.Compact(%q): %v", string(raw), err)
	}
	return append([]byte(nil), buf.Bytes()...)
}
