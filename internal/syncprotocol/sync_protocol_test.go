package syncprotocol

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/cervantesh/convex-go/internal/core"
)

func TestSyncTimestampEncoding(t *testing.T) {
	got, err := json.Marshal(SyncTimestamp(42))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `"KgAAAAAAAAA="` {
		t.Fatalf("encoded timestamp mismatch: %s", got)
	}

	var decoded SyncTimestamp
	if err := json.Unmarshal([]byte(`"AAAAAAAAABA="`), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != SyncTimestamp(1152921504606846976) {
		t.Fatalf("decoded timestamp mismatch: %d", decoded)
	}
}

func TestClientConnectRoundTrip(t *testing.T) {
	raw := []byte(`{"type":"Connect","sessionId":"00000000-0000-0000-0000-000000000000","connectionCount":2,"lastCloseReason":"reconnect","maxObservedTimestamp":"AQAAAAAAAAA=","clientTs":123456}`)

	msg, err := DecodeClientMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	connect, ok := msg.(ConnectMessage)
	if !ok {
		t.Fatalf("expected ConnectMessage, got %T", msg)
	}
	if connect.LastCloseReason != "reconnect" {
		t.Fatalf("unexpected last close reason: %q", connect.LastCloseReason)
	}
	if connect.MaxObservedTimestamp == nil || *connect.MaxObservedTimestamp != SyncTimestamp(1) {
		t.Fatalf("unexpected max observed timestamp: %#v", connect.MaxObservedTimestamp)
	}

	encoded, err := EncodeClientMessage(connect)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, raw, encoded)
}

func TestClientConnectPreservesNullLastCloseReason(t *testing.T) {
	raw := []byte(`{"type":"Connect","sessionId":"00000000-0000-0000-0000-000000000000","connectionCount":1,"lastCloseReason":null,"clientTs":123456}`)

	msg, err := DecodeClientMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	connect := msg.(ConnectMessage)
	if connect.LastCloseReason != "unknown" {
		t.Fatalf("expected user-facing default, got %q", connect.LastCloseReason)
	}

	encoded, err := EncodeClientMessage(connect)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, raw, encoded)
}

func TestModifyQuerySetAddRemove(t *testing.T) {
	raw := []byte(`{"type":"ModifyQuerySet","baseVersion":0,"newVersion":1,"modifications":[{"type":"Add","queryId":7,"udfPath":"messages:list","args":[{"name":"Ada"}],"journal":null,"componentPath":"comp"},{"type":"Remove","queryId":7}]}`)

	msg, err := DecodeClientMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	modify, ok := msg.(ModifyQuerySetMessage)
	if !ok {
		t.Fatalf("expected ModifyQuerySetMessage, got %T", msg)
	}
	if modify.BaseVersion != 0 || modify.NewVersion != 1 {
		t.Fatalf("unexpected versions: %#v", modify)
	}
	if len(modify.Modifications) != 2 {
		t.Fatalf("expected two modifications, got %d", len(modify.Modifications))
	}
	add, ok := modify.Modifications[0].(QuerySetAdd)
	if !ok {
		t.Fatalf("expected QuerySetAdd, got %T", modify.Modifications[0])
	}
	if !add.Journal.Present || add.Journal.Value != nil {
		t.Fatalf("expected explicit null journal, got %#v", add.Journal)
	}
	if len(add.Args) != 1 {
		t.Fatalf("expected one arg, got %d", len(add.Args))
	}
	arg, ok := add.Args[0].Object()
	if !ok {
		t.Fatalf("expected object arg, got %#v", add.Args[0])
	}
	name, ok := arg["name"].String()
	if !ok || name != "Ada" {
		t.Fatalf("unexpected arg name: %#v", arg["name"])
	}
	if _, ok := modify.Modifications[1].(QuerySetRemove); !ok {
		t.Fatalf("expected QuerySetRemove, got %T", modify.Modifications[1])
	}

	encoded, err := EncodeClientMessage(modify)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, raw, encoded)
}

func TestServerTransitionWithNullErrorData(t *testing.T) {
	raw := []byte(`{"type":"Transition","startVersion":{"querySet":0,"identity":0,"ts":"AAAAAAAAAAA="},"endVersion":{"querySet":1,"identity":0,"ts":"AQAAAAAAAAA="},"modifications":[{"type":"QueryFailed","queryId":1,"errorMessage":"boom","logLines":[],"journal":null,"errorData":null}],"clientClockSkew":null,"serverTs":null}`)

	msg, err := DecodeServerMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	transition, ok := msg.(TransitionMessage)
	if !ok {
		t.Fatalf("expected TransitionMessage, got %T", msg)
	}
	if len(transition.Modifications) != 1 {
		t.Fatalf("expected one modification, got %d", len(transition.Modifications))
	}
	failed, ok := transition.Modifications[0].(QueryFailed)
	if !ok {
		t.Fatalf("expected QueryFailed, got %T", transition.Modifications[0])
	}
	if !failed.ErrorData.Present || failed.ErrorData.Value.Kind() != core.NullKind {
		t.Fatalf("expected explicit null errorData, got %#v", failed.ErrorData)
	}

	encoded, err := EncodeServerMessage(transition)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, raw, encoded)
}

func TestMutationResponseSuccessRoundTrips(t *testing.T) {
	raw := []byte(`{"type":"MutationResponse","requestId":1,"success":true,"result":{"ok":true},"ts":"AQAAAAAAAAA=","logLines":[]}`)
	mutation := assertServerMessageRoundTrip[MutationResponseMessage](t, raw)
	if !mutation.Success || mutation.TS == nil || *mutation.TS != SyncTimestamp(1) {
		t.Fatalf("unexpected mutation response: %#v", mutation)
	}
}

func TestActionResponseFailureWithErrorDataRoundTrips(t *testing.T) {
	raw := []byte(`{"type":"ActionResponse","requestId":2,"success":false,"result":"boom","logLines":[],"errorData":{"reason":"bad"}}`)
	action := assertServerMessageRoundTrip[ActionResponseMessage](t, raw)
	if action.Success || action.ErrorMessage != "boom" || !action.ErrorData.Present {
		t.Fatalf("unexpected action response: %#v", action)
	}
}

func TestAdminAuthDecodesActingAsAliasAndEncodesImpersonating(t *testing.T) {
	raw := []byte(`{"type":"Authenticate","baseVersion":0,"tokenType":"Admin","value":"admin-token","actingAs":{"issuer":"issuer","subject":"subject"}}`)
	auth := assertClientMessageDecode[AuthenticateMessage](t, raw)
	if auth.ActingAs == nil || auth.ActingAs.TokenIdentifier != "issuer|subject" {
		t.Fatalf("unexpected actingAs identity: %#v", auth.ActingAs)
	}
	encoded, err := EncodeClientMessage(auth)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONHas(t, encoded, "impersonating")
	assertJSONMissing(t, encoded, "actingAs")
}

func TestSyncTimestampRejectsMalformedValues(t *testing.T) {
	for _, raw := range []string{`"not-base64!"`, `"AQ=="`, `"AAAAAAAAAIA="`} {
		t.Run(raw, func(t *testing.T) {
			var timestamp SyncTimestamp
			if err := json.Unmarshal([]byte(raw), &timestamp); err == nil {
				t.Fatalf("expected %s to be rejected", raw)
			}
		})
	}
}

func TestSyncTimestampRejectsOutOfRangeMarshal(t *testing.T) {
	_, err := json.Marshal(SyncTimestamp(uint64(math.MaxInt64) + 1))
	if err == nil {
		t.Fatal("expected out-of-range timestamp to be rejected")
	}
}

func TestSyncTimestampAcceptsMaxInt64(t *testing.T) {
	raw := []byte(`"/////////38="`)

	var timestamp SyncTimestamp
	if err := json.Unmarshal(raw, &timestamp); err != nil {
		t.Fatal(err)
	}
	if timestamp != SyncTimestamp(math.MaxInt64) {
		t.Fatalf("decoded timestamp mismatch: got %d want %d", timestamp, core.Int64(math.MaxInt64))
	}

	encoded, err := json.Marshal(SyncTimestamp(math.MaxInt64))
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != string(raw) {
		t.Fatalf("encoded max timestamp mismatch: got %s want %s", encoded, raw)
	}
}

func TestSyncTimestampConformanceAcceptsURLSafeBase64(t *testing.T) {
	var timestamp SyncTimestamp
	if err := json.Unmarshal([]byte(`"_________38="`), &timestamp); err != nil {
		t.Fatal(err)
	}
	if timestamp != SyncTimestamp(math.MaxInt64) {
		t.Fatalf("decoded timestamp mismatch: got %d want %d", timestamp, core.Int64(math.MaxInt64))
	}
}

func TestOptionalStringAndValueHelpers(t *testing.T) {
	raw := map[string]json.RawMessage{
		"stringNull": json.RawMessage(`null`),
		"string":     json.RawMessage(`"cursor"`),
		"stringBad":  json.RawMessage(`1`),
		"valueNull":  json.RawMessage(`null`),
		"value":      json.RawMessage(`{"$integer":"AQAAAAAAAAA="}`),
		"valueBad":   json.RawMessage(`{"$float":1}`),
	}
	if value, err := optionalStringFromRaw(raw, "missing"); err != nil || value.Present {
		t.Fatalf("unexpected missing string: %#v %v", value, err)
	}
	if value, err := optionalStringFromRaw(raw, "stringNull"); err != nil || !value.Present || value.Value != nil {
		t.Fatalf("unexpected null string: %#v %v", value, err)
	}
	if value, err := optionalStringFromRaw(raw, "string"); err != nil || value.Value == nil || *value.Value != "cursor" {
		t.Fatalf("unexpected string: %#v %v", value, err)
	}
	if _, err := optionalStringFromRaw(raw, "stringBad"); err == nil {
		t.Fatal("expected bad optional string to fail")
	}
	assertOptionalValue(t, raw)
}

func assertOptionalValue(t *testing.T, raw map[string]json.RawMessage) {
	t.Helper()
	if value, err := optionalValueFromRaw(raw, "missing"); err != nil || value.Present {
		t.Fatalf("unexpected missing value: %#v %v", value, err)
	}
	if value, err := optionalValueFromRaw(raw, "valueNull"); err != nil || !value.Present || value.Value.Kind() != core.NullKind {
		t.Fatalf("unexpected null value: %#v %v", value, err)
	}
	if value, err := optionalValueFromRaw(raw, "value"); err != nil || value.Value.GoValue() != core.Int64(1) {
		t.Fatalf("unexpected optional value: %#v %v", value, err)
	}
	if _, err := optionalValueFromRaw(raw, "valueBad"); err == nil {
		t.Fatal("expected bad optional value to fail")
	}
}

func TestOptionalNumberAndTimestampHelpers(t *testing.T) {
	raw := map[string]json.RawMessage{
		"tsNull":       json.RawMessage(`null`),
		"ts":           json.RawMessage(`"AQAAAAAAAAA="`),
		"tsBad":        json.RawMessage(`"bad"`),
		"floatNull":    json.RawMessage(`null`),
		"float":        json.RawMessage(`1.5`),
		"floatBad":     json.RawMessage(`"bad"`),
		"intNull":      json.RawMessage(`null`),
		"int":          json.RawMessage(`7`),
		"intBad":       json.RawMessage(`"bad"`),
		"identityNull": json.RawMessage(`null`),
		"identity":     json.RawMessage(`3`),
		"identityBad":  json.RawMessage(`"bad"`),
	}
	assertOptionalTimestamp(t, raw)
	assertOptionalFloat64(t, raw)
	assertOptionalInt64(t, raw)
	assertOptionalIdentityVersion(t, raw)
}

func assertOptionalTimestamp(t *testing.T, raw map[string]json.RawMessage) {
	t.Helper()
	if value, present, err := optionalTimestampFromRaw(raw, "missing"); err != nil || present || value != nil {
		t.Fatalf("unexpected missing timestamp: %#v %v %v", value, present, err)
	}
	if value, present, err := optionalTimestampFromRaw(raw, "tsNull"); err != nil || !present || value != nil {
		t.Fatalf("unexpected null timestamp: %#v %v %v", value, present, err)
	}
	if value, present, err := optionalTimestampFromRaw(raw, "ts"); err != nil || !present || *value != 1 {
		t.Fatalf("unexpected timestamp: %#v %v %v", value, present, err)
	}
	if _, _, err := optionalTimestampFromRaw(raw, "tsBad"); err == nil {
		t.Fatal("expected bad timestamp to fail")
	} else if value, present, err := optionalTimestampFromRaw(raw, "tsBad"); err == nil || value != nil || present {
		t.Fatalf("bad timestamp should not report a present value: %#v %v %v", value, present, err)
	}
}

func assertOptionalFloat64(t *testing.T, raw map[string]json.RawMessage) {
	t.Helper()
	if value, present, err := optionalFloat64FromRaw(raw, "missing"); err != nil || present || value != nil {
		t.Fatalf("unexpected missing float: %#v %v %v", value, present, err)
	}
	if value, present, err := optionalFloat64FromRaw(raw, "floatNull"); err != nil || !present || value != nil {
		t.Fatalf("unexpected null float: %#v %v %v", value, present, err)
	}
	if value, present, err := optionalFloat64FromRaw(raw, "float"); err != nil || !present || *value != 1.5 {
		t.Fatalf("unexpected float: %#v %v %v", value, present, err)
	}
	if _, _, err := optionalFloat64FromRaw(raw, "floatBad"); err == nil {
		t.Fatal("expected bad float to fail")
	} else if value, present, err := optionalFloat64FromRaw(raw, "floatBad"); err == nil || value != nil || present {
		t.Fatalf("bad float should not report a present value: %#v %v %v", value, present, err)
	}
}

func assertOptionalInt64(t *testing.T, raw map[string]json.RawMessage) {
	t.Helper()
	if value, present, err := optionalInt64FromRaw(raw, "missing"); err != nil || present || value != nil {
		t.Fatalf("unexpected missing int: %#v %v %v", value, present, err)
	}
	if value, present, err := optionalInt64FromRaw(raw, "intNull"); err != nil || !present || value != nil {
		t.Fatalf("unexpected null int: %#v %v %v", value, present, err)
	}
	if value, present, err := optionalInt64FromRaw(raw, "int"); err != nil || !present || *value != 7 {
		t.Fatalf("unexpected int: %#v %v %v", value, present, err)
	}
	if _, _, err := optionalInt64FromRaw(raw, "intBad"); err == nil {
		t.Fatal("expected bad int to fail")
	} else if value, present, err := optionalInt64FromRaw(raw, "intBad"); err == nil || value != nil || present {
		t.Fatalf("bad int should not report a present value: %#v %v %v", value, present, err)
	}
}

func assertOptionalIdentityVersion(t *testing.T, raw map[string]json.RawMessage) {
	t.Helper()
	if value, present, err := optionalIdentityVersionFromRaw(raw, "missing"); err != nil || present || value != nil {
		t.Fatalf("unexpected missing identity: %#v %v %v", value, present, err)
	}
	if value, present, err := optionalIdentityVersionFromRaw(raw, "identityNull"); err != nil || !present || value != nil {
		t.Fatalf("unexpected null identity: %#v %v %v", value, present, err)
	}
	if value, present, err := optionalIdentityVersionFromRaw(raw, "identity"); err != nil || !present || *value != 3 {
		t.Fatalf("unexpected identity: %#v %v %v", value, present, err)
	}
	if _, _, err := optionalIdentityVersionFromRaw(raw, "identityBad"); err == nil {
		t.Fatal("expected bad identity to fail")
	} else if value, present, err := optionalIdentityVersionFromRaw(raw, "identityBad"); err == nil || value != nil || present {
		t.Fatalf("bad identity should not report a present value: %#v %v %v", value, present, err)
	}
}

func TestClientMutationActionEventAndAuthVariants(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want any
	}{
		{
			name: "mutation",
			raw:  []byte(`{"type":"Mutation","requestId":3,"udfPath":"messages:send","args":[{"body":"hello"}],"componentPath":"comp"}`),
			want: MutationMessage{},
		},
		{
			name: "action",
			raw:  []byte(`{"type":"Action","requestId":4,"udfPath":"jobs:run","args":[true],"componentPath":"comp"}`),
			want: ActionMessage{},
		},
		{
			name: "event",
			raw:  []byte(`{"type":"Event","eventType":"Trace","event":{"span":"abc"}}`),
			want: EventMessage{},
		},
		{
			name: "authenticate user",
			raw:  []byte(`{"type":"Authenticate","baseVersion":2,"tokenType":"User","value":"jwt-token"}`),
			want: AuthenticateMessage{},
		},
		{
			name: "authenticate none",
			raw:  []byte(`{"type":"Authenticate","baseVersion":3,"tokenType":"None"}`),
			want: AuthenticateMessage{},
		},
		{
			name: "authenticate admin impersonating",
			raw:  []byte(`{"type":"Authenticate","baseVersion":4,"tokenType":"Admin","value":"admin-token","impersonating":{"issuer":"issuer","subject":"subject","tokenIdentifier":"issuer|subject"}}`),
			want: AuthenticateMessage{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := assertClientMessageRoundTrip(t, tt.raw, tt.want)
			assertAdminActingAsWhenPresent(t, msg)
		})
	}
}

func TestModifyQuerySetJournalPresenceVariants(t *testing.T) {
	raw := []byte(`{"type":"ModifyQuerySet","baseVersion":1,"newVersion":2,"modifications":[{"type":"Add","queryId":8,"udfPath":"messages:list","args":[]},{"type":"Add","queryId":9,"udfPath":"messages:list","args":[],"journal":"cursor"}]}`)

	msg, err := DecodeClientMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	modify := msg.(ModifyQuerySetMessage)
	withoutJournal := modify.Modifications[0].(QuerySetAdd)
	if withoutJournal.Journal.Present {
		t.Fatalf("expected omitted journal to stay absent, got %#v", withoutJournal.Journal)
	}
	withJournal := modify.Modifications[1].(QuerySetAdd)
	if !withJournal.Journal.Present || withJournal.Journal.Value == nil || *withJournal.Journal.Value != "cursor" {
		t.Fatalf("expected string journal, got %#v", withJournal.Journal)
	}

	encoded, err := EncodeClientMessage(modify)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, raw, encoded)
}

func TestModifyQuerySetRequiresModifications(t *testing.T) {
	_, err := DecodeClientMessage([]byte(`{"type":"ModifyQuerySet","baseVersion":1,"newVersion":2}`))
	if err == nil {
		t.Fatal("expected missing modifications to be rejected")
	}
	if err.Error() != "convex: ModifyQuerySet missing modifications" {
		t.Fatalf("unexpected missing modifications error: %v", err)
	}

	_, err = DecodeClientMessage([]byte(`{"type":"ModifyQuerySet","baseVersion":1,"newVersion":2,"modifications":null}`))
	if err == nil {
		t.Fatal("expected null modifications to be rejected")
	}
	if err.Error() != "convex: ModifyQuerySet missing modifications" {
		t.Fatalf("unexpected null modifications error: %v", err)
	}
}

func TestServerTransitionUpdateRemoveAndErrorDataPresence(t *testing.T) {
	raw := []byte(`{"type":"Transition","startVersion":{"querySet":1,"identity":0,"ts":"AQAAAAAAAAA="},"endVersion":{"querySet":2,"identity":0,"ts":"AgAAAAAAAAA="},"modifications":[{"type":"QueryUpdated","queryId":1,"value":{"ok":true},"logLines":["updated"],"journal":"cursor"},{"type":"QueryFailed","queryId":2,"errorMessage":"boom","logLines":[],"journal":null},{"type":"QueryRemoved","queryId":3}],"clientClockSkew":12.5,"serverTs":123456}`)

	msg, err := DecodeServerMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	transition := msg.(TransitionMessage)
	updated := transition.Modifications[0].(QueryUpdated)
	if !updated.Journal.Present || updated.Journal.Value == nil || *updated.Journal.Value != "cursor" {
		t.Fatalf("unexpected updated journal: %#v", updated.Journal)
	}
	failed := transition.Modifications[1].(QueryFailed)
	if failed.ErrorData.Present {
		t.Fatalf("expected omitted errorData to stay absent, got %#v", failed.ErrorData)
	}
	if !failed.Journal.Present || failed.Journal.Value != nil {
		t.Fatalf("expected null failed journal, got %#v", failed.Journal)
	}
	if _, ok := transition.Modifications[2].(QueryRemoved); !ok {
		t.Fatalf("expected QueryRemoved, got %T", transition.Modifications[2])
	}
	if transition.ClientClockSkew == nil || *transition.ClientClockSkew != 12.5 {
		t.Fatalf("unexpected client clock skew: %#v", transition.ClientClockSkew)
	}
	if transition.ServerTS == nil || *transition.ServerTS != 123456 {
		t.Fatalf("unexpected server ts: %#v", transition.ServerTS)
	}

	encoded, err := EncodeServerMessage(transition)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, raw, encoded)
}

func TestServerTransitionRequiresQueryJournals(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{
			name:    "updated",
			raw:     `{"type":"Transition","startVersion":{"querySet":1,"identity":0,"ts":"AQAAAAAAAAA="},"endVersion":{"querySet":2,"identity":0,"ts":"AgAAAAAAAAA="},"modifications":[{"type":"QueryUpdated","queryId":1,"value":{"ok":true},"logLines":[]}],"clientClockSkew":null,"serverTs":null}`,
			wantErr: "convex: state modification 0: convex: QueryUpdated missing journal",
		},
		{
			name:    "failed",
			raw:     `{"type":"Transition","startVersion":{"querySet":1,"identity":0,"ts":"AQAAAAAAAAA="},"endVersion":{"querySet":2,"identity":0,"ts":"AgAAAAAAAAA="},"modifications":[{"type":"QueryFailed","queryId":1,"errorMessage":"boom","logLines":[]}],"clientClockSkew":null,"serverTs":null}`,
			wantErr: "convex: state modification 0: convex: QueryFailed missing journal",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeServerMessage([]byte(tt.raw))
			if err == nil {
				t.Fatal("expected missing journal to be rejected")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("unexpected missing journal error: %v", err)
			}
		})
	}
}

func TestResponseCodecMatrix(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want any
	}{
		{
			name: "mutation response failure",
			raw:  []byte(`{"type":"MutationResponse","requestId":5,"success":false,"result":"boom","ts":null,"logLines":["line"],"errorData":{"reason":"bad"}}`),
			want: MutationResponseMessage{},
		},
		{
			name: "action response success",
			raw:  []byte(`{"type":"ActionResponse","requestId":6,"success":true,"result":{"ok":true},"logLines":["line"]}`),
			want: ActionResponseMessage{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := DecodeServerMessage(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			if reflect.TypeOf(msg) != reflect.TypeOf(tt.want) {
				t.Fatalf("decoded type mismatch: got %T want %T", msg, tt.want)
			}
			encoded, err := EncodeServerMessage(msg)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, tt.raw, encoded)
		})
	}
}

func TestOtherServerMessagesRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want any
	}{
		{
			name: "transition chunk",
			raw:  []byte(`{"type":"TransitionChunk","chunk":"abc","partNumber":0,"totalParts":2,"transitionId":"transition-1"}`),
			want: TransitionChunkMessage{},
		},
		{
			name: "auth error",
			raw:  []byte(`{"type":"AuthError","error":"bad auth","baseVersion":null,"authUpdateAttempted":true}`),
			want: AuthErrorMessage{},
		},
		{
			name: "fatal error",
			raw:  []byte(`{"type":"FatalError","error":"fatal"}`),
			want: FatalErrorMessage{},
		},
		{
			name: "ping",
			raw:  []byte(`{"type":"Ping"}`),
			want: PingMessage{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := DecodeServerMessage(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			if reflect.TypeOf(msg) != reflect.TypeOf(tt.want) {
				t.Fatalf("decoded type mismatch: got %T want %T", msg, tt.want)
			}
			encoded, err := EncodeServerMessage(msg)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, tt.raw, encoded)
		})
	}
}

func TestSyncProtocolRejectsMalformedEnvelopes(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`{}`),
		[]byte(`{"type":"UnknownClient"}`),
		[]byte(`not-json`),
		[]byte(`{"type":"ModifyQuerySet","baseVersion":1,"newVersion":2,"modifications":[{"type":"Patch"}]}`),
	} {
		if _, err := DecodeClientMessage(raw); err == nil {
			t.Fatalf("expected client decode to fail for %s", raw)
		}
	}
	for _, raw := range [][]byte{
		[]byte(`{}`),
		[]byte(`{"type":"UnknownServer"}`),
		[]byte(`not-json`),
		[]byte(`{"type":"Transition","startVersion":{"querySet":1,"identity":0,"ts":"AQAAAAAAAAA="},"endVersion":{"querySet":2,"identity":0,"ts":"AgAAAAAAAAA="},"modifications":[{"type":"Patch"}]}`),
		[]byte(`{"type":"Transition","startVersion":{"querySet":"bad","identity":0,"ts":"AQAAAAAAAAA="},"endVersion":{"querySet":2,"identity":0,"ts":"AgAAAAAAAAA="},"modifications":[]}`),
		[]byte(`{"type":"Transition","startVersion":{"querySet":1,"identity":0,"ts":"bad"},"endVersion":{"querySet":2,"identity":0,"ts":"AgAAAAAAAAA="},"modifications":[]}`),
		[]byte(`{"type":"Transition","startVersion":{"querySet":1,"identity":0,"ts":"AQAAAAAAAAA="},"endVersion":{"querySet":2,"identity":0,"ts":"AgAAAAAAAAA="},"modifications":[],"clientClockSkew":"bad"}`),
		[]byte(`{"type":"Transition","startVersion":{"querySet":1,"identity":0,"ts":"AQAAAAAAAAA="},"endVersion":{"querySet":2,"identity":0,"ts":"AgAAAAAAAAA="},"modifications":[],"serverTs":"bad"}`),
		[]byte(`{"type":"AuthError","error":"bad","baseVersion":"bad"}`),
	} {
		if _, err := DecodeServerMessage(raw); err == nil {
			t.Fatalf("expected server decode to fail for %s", raw)
		}
	}
	if _, err := EncodeClientMessage(nil); err == nil {
		t.Fatal("expected nil client message encode to fail")
	} else if err.Error() != "convex: nil client sync message" {
		t.Fatalf("unexpected nil client message error: %v", err)
	}
	if _, err := EncodeServerMessage(nil); err == nil {
		t.Fatal("expected nil server message encode to fail")
	} else if err.Error() != "convex: nil server sync message" {
		t.Fatalf("unexpected nil server message error: %v", err)
	}

	_, err := DecodeServerMessage([]byte(`{"Type":"Ping"}`))
	if err == nil {
		t.Fatal("expected missing exact type to fail")
	}
	if err.Error() != "convex: sync message missing type" {
		t.Fatalf("unexpected missing type error: %v", err)
	}
}

func TestSyncProtocolRejectsMalformedFields(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`{"type":"Connect","sessionId":"s","connectionCount":0,"lastCloseReason":1}`),
		[]byte(`{"type":"ModifyQuerySet","baseVersion":1,"newVersion":2,"modifications":"bad"}`),
		[]byte(`{"type":"ModifyQuerySet","baseVersion":1,"newVersion":2,"modifications":[1]}`),
		[]byte(`{"type":"ModifyQuerySet","baseVersion":1,"newVersion":2,"modifications":[{"type":"Add","queryId":1,"udfPath":"x","args":[],"journal":1}]}`),
		[]byte(`{"type":"Authenticate","baseVersion":0,"tokenType":"Admin","value":"admin","actingAs":"bad"}`),
	} {
		if _, err := DecodeClientMessage(raw); err == nil {
			t.Fatalf("expected malformed client field to fail for %s", raw)
		}
	}
	for _, raw := range [][]byte{
		[]byte(`{"type":"Transition","startVersion":{"querySet":1,"identity":0,"ts":"AQAAAAAAAAA="},"endVersion":{"querySet":2,"identity":0,"ts":"AgAAAAAAAAA="},"modifications":[1]}`),
		[]byte(`{"type":"Transition","startVersion":"bad","endVersion":{"querySet":2,"identity":0,"ts":"AgAAAAAAAAA="},"modifications":[]}`),
		[]byte(`{"type":"Transition","startVersion":{"querySet":1,"identity":0,"ts":"AQAAAAAAAAA="},"endVersion":"bad","modifications":[]}`),
		[]byte(`{"type":"Transition","startVersion":{"querySet":1,"identity":0,"ts":"AQAAAAAAAAA="},"endVersion":{"querySet":2,"identity":0,"ts":"AgAAAAAAAAA="},"modifications":[{"type":"QueryUpdated","queryId":1,"value":true,"logLines":[],"journal":1}]}`),
		[]byte(`{"type":"Transition","startVersion":{"querySet":1,"identity":0,"ts":"AQAAAAAAAAA="},"endVersion":{"querySet":2,"identity":0,"ts":"AgAAAAAAAAA="},"modifications":[{"type":"QueryFailed","queryId":1,"errorMessage":"bad","logLines":[],"journal":null,"errorData":{"$float":1}}]}`),
		[]byte(`{"type":"MutationResponse","requestId":1,"success":false,"result":"bad","ts":"bad","logLines":[]}`),
		[]byte(`{"type":"MutationResponse","requestId":1,"success":false,"result":"bad","ts":null,"logLines":[],"errorData":{"$float":1}}`),
		[]byte(`{"type":"ActionResponse","requestId":1,"success":false,"result":"bad","logLines":[],"errorData":{"$float":1}}`),
	} {
		if _, err := DecodeServerMessage(raw); err == nil {
			t.Fatalf("expected malformed server field to fail for %s", raw)
		}
	}
}

func TestSyncProtocolMarkerMethods(t *testing.T) {
	ConnectMessage{}.clientMessage()
	ModifyQuerySetMessage{}.clientMessage()
	MutationMessage{}.clientMessage()
	ActionMessage{}.clientMessage()
	AuthenticateMessage{}.clientMessage()
	EventMessage{}.clientMessage()
	QuerySetAdd{}.querySetModification()
	QuerySetRemove{}.querySetModification()
	TransitionMessage{}.serverMessage()
	MutationResponseMessage{}.serverMessage()
	ActionResponseMessage{}.serverMessage()
	AuthErrorMessage{}.serverMessage()
	FatalErrorMessage{}.serverMessage()
	PingMessage{}.serverMessage()
	TransitionChunkMessage{}.serverMessage()
	QueryUpdated{}.stateModification()
	QueryFailed{}.stateModification()
	QueryRemoved{}.stateModification()
}

func TestAuthErrorRoundTripWithConcreteBaseVersion(t *testing.T) {
	raw := []byte(`{"type":"AuthError","error":"bad auth","baseVersion":2,"authUpdateAttempted":false}`)
	msg := assertServerMessageRoundTrip[AuthErrorMessage](t, raw)
	if msg.BaseVersion == nil || *msg.BaseVersion != 2 {
		t.Fatalf("unexpected auth error base version: %#v", msg.BaseVersion)
	}
	if msg.AuthUpdateAttempted == nil || *msg.AuthUpdateAttempted {
		t.Fatalf("unexpected auth update attempted: %#v", msg.AuthUpdateAttempted)
	}
}

func TestSyncProtocolPublicConstantsAndIdentityClaims(t *testing.T) {
	if AuthTokenNone != "None" || AuthTokenUser != "User" || AuthTokenAdmin != "Admin" {
		t.Fatalf("unexpected auth token constants: none=%q user=%q admin=%q", AuthTokenNone, AuthTokenUser, AuthTokenAdmin)
	}

	var identity SyncUserIdentityAttributes
	if err := json.Unmarshal([]byte(`{"tokenIdentifier":"explicit","issuer":"issuer","subject":"subject","numeric":1}`), &identity); err != nil {
		t.Fatal(err)
	}
	if identity.TokenIdentifier != "explicit" || identity.Issuer != "issuer" || identity.Subject != "subject" {
		t.Fatalf("unexpected identity claims: %#v", identity)
	}
	if value, ok := claimString(identity.Claims, "missing"); ok || value != "" {
		t.Fatalf("missing string claim should report absent, got %q %v", value, ok)
	}
	if value, ok := claimString(identity.Claims, "numeric"); ok || value != "" {
		t.Fatalf("non-string claim should report absent, got %q %v", value, ok)
	}

	for _, tt := range []struct {
		name     string
		identity SyncUserIdentityAttributes
	}{
		{name: "issuer only", identity: SyncUserIdentityAttributes{Issuer: "issuer"}},
		{name: "subject only", identity: SyncUserIdentityAttributes{Subject: "subject"}},
	} {
		t.Run("marshal "+tt.name, func(t *testing.T) {
			encoded, err := json.Marshal(tt.identity)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONMissing(t, encoded, "tokenIdentifier")
		})
	}
}

func TestResponseMarshalOmitsAbsentOptionalFields(t *testing.T) {
	mutation, err := EncodeServerMessage(MutationResponseMessage{
		RequestID: 1,
		Success:   true,
		Value:     core.NullValue(),
		LogLines:  []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertJSONMissing(t, mutation, "ts")

	auth, err := EncodeServerMessage(AuthErrorMessage{Error: "bad auth"})
	if err != nil {
		t.Fatal(err)
	}
	assertJSONMissing(t, auth, "baseVersion")
	assertJSONMissing(t, auth, "authUpdateAttempted")
}

// Sources for the following Convex Rust conformance tests:
//   - convex-rs/sync_types/src/types/json.rs:
//     authentication_token_backwards_compatability,
//     server_message_mutation_response_with_null_error_data_roundtrips,
//     server_message_transition_with_null_error_data_roundtrips,
//     test_empty_array_roundtrips.
func TestSyncProtocolConvexRSClientFixtures(t *testing.T) {
	clientFixtures := []struct {
		name string
		raw  []byte
		want any
	}{
		{
			name: "old admin authenticate without actingAs",
			raw:  []byte(`{"type":"Authenticate","tokenType":"Admin","value":"fakefakefake","baseVersion":0}`),
			want: AuthenticateMessage{},
		},
		{
			name: "old user authenticate",
			raw:  []byte(`{"type":"Authenticate","tokenType":"User","value":"fakefakefake","baseVersion":0}`),
			want: AuthenticateMessage{},
		},
		{
			name: "mutation empty args array",
			raw:  []byte(`{"type":"Mutation","requestId":0,"udfPath":"path","args":[]}`),
			want: MutationMessage{},
		},
	}
	for _, tt := range clientFixtures {
		t.Run(tt.name, func(t *testing.T) {
			assertClientMessageRoundTrip(t, tt.raw, tt.want)
		})
	}
}

func TestSyncProtocolConvexRSServerFixtures(t *testing.T) {
	serverFixtures := []struct {
		name string
		raw  []byte
		want any
	}{
		{
			name: "mutation response with null errorData",
			raw:  []byte(`{"type":"MutationResponse","requestId":1,"success":false,"result":"","ts":null,"logLines":[],"errorData":null}`),
			want: MutationResponseMessage{},
		},
		{
			name: "transition query failed with null errorData",
			raw:  []byte(`{"type":"Transition","startVersion":{"querySet":1,"identity":1,"ts":"AQAAAAAAAAA="},"endVersion":{"querySet":1,"identity":1,"ts":"AQAAAAAAAAA="},"modifications":[{"type":"QueryFailed","queryId":1,"errorMessage":"","logLines":[],"journal":null,"errorData":null}],"clientClockSkew":1,"serverTs":1}`),
			want: TransitionMessage{},
		},
	}
	for _, tt := range serverFixtures {
		t.Run(tt.name, func(t *testing.T) {
			assertServerMessageRoundTripAny(t, tt.raw, tt.want)
		})
	}
}

func TestSyncProtocolConformanceAdminAuthUsesImpersonatingField(t *testing.T) {
	// Source: convex-js/src/browser/sync/protocol.ts AdminAuthentication.
	auth := AuthenticateMessage{
		BaseVersion: 0,
		TokenType:   AuthTokenAdmin,
		Value:       "admin-token",
		ActingAs: &SyncUserIdentityAttributes{
			Issuer:  "issuer",
			Subject: "subject",
		},
	}
	encoded, err := EncodeClientMessage(auth)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, []byte(`{"type":"Authenticate","baseVersion":0,"tokenType":"Admin","value":"admin-token","impersonating":{"issuer":"issuer","subject":"subject","tokenIdentifier":"issuer|subject"}}`), encoded)
}

func TestSyncProtocolConformanceTransitionWithEmptyModificationsRoundTrips(t *testing.T) {
	// Source: convex-rs/sync_types/src/types/json.rs test_empty_array_roundtrips.
	raw := []byte(`{"type":"Transition","startVersion":{"querySet":1,"identity":0,"ts":"AQAAAAAAAAA="},"endVersion":{"querySet":1,"identity":0,"ts":"AQAAAAAAAAA="},"modifications":[],"clientClockSkew":null,"serverTs":null}`)
	msg := assertServerMessageRoundTrip[TransitionMessage](t, raw)
	if len(msg.Modifications) != 0 {
		t.Fatalf("expected empty transition modifications, got %#v", msg.Modifications)
	}
}

func TestClientMessageDecodeUsesExactWireFieldNames(t *testing.T) {
	t.Run("connect ignores Go field aliases", func(t *testing.T) {
		raw := []byte(`{"type":"Connect","Type":"Mutation","sessionId":"wire-session","SessionID":"alias-session","connectionCount":2,"ConnectionCount":99,"maxObservedTimestamp":"AQAAAAAAAAA=","MaxObservedTimestamp":"AgAAAAAAAAA=","clientTs":3,"ClientTS":4}`)
		connect := assertClientMessageDecode[ConnectMessage](t, raw)
		if connect.SessionID != "wire-session" || connect.ConnectionCount != 2 {
			t.Fatalf("decoded alias field instead of exact wire fields: %#v", connect)
		}
		if connect.MaxObservedTimestamp == nil || *connect.MaxObservedTimestamp != SyncTimestamp(1) {
			t.Fatalf("decoded alias maxObservedTimestamp: %#v", connect.MaxObservedTimestamp)
		}
		if connect.ClientTS == nil || *connect.ClientTS != 3 {
			t.Fatalf("decoded alias clientTs: %#v", connect.ClientTS)
		}
	})

	t.Run("modify query set ignores Go field aliases", func(t *testing.T) {
		raw := []byte(`{"type":"ModifyQuerySet","baseVersion":1,"BaseVersion":90,"newVersion":2,"NewVersion":91,"modifications":[{"type":"Add","queryId":7,"QueryID":70,"udfPath":"messages:list","UDFPath":"wrong:path","args":[{"ok":true}],"Args":[false],"componentPath":"component","ComponentPath":"wrong","journal":"cursor"}],"Modifications":[{"type":"Remove","queryId":99}]}`)
		modify := assertClientMessageDecode[ModifyQuerySetMessage](t, raw)
		if modify.BaseVersion != 1 || modify.NewVersion != 2 || len(modify.Modifications) != 1 {
			t.Fatalf("decoded alias query set fields: %#v", modify)
		}
		add := modify.Modifications[0].(QuerySetAdd)
		if add.QueryID != 7 || add.UDFPath != "messages:list" || add.ComponentPath != "component" {
			t.Fatalf("decoded alias add fields: %#v", add)
		}
		if len(add.Args) != 1 || add.Args[0].Kind() != core.ObjectKind {
			t.Fatalf("decoded alias args: %#v", add.Args)
		}
	})

	t.Run("mutation action auth and event ignore Go field aliases", func(t *testing.T) {
		mutation := assertClientMessageDecode[MutationMessage](t, []byte(`{"type":"Mutation","requestId":3,"RequestID":30,"udfPath":"messages:send","UDFPath":"wrong:path","args":[{"body":"hello"}],"Args":[false],"componentPath":"component","ComponentPath":"wrong"}`))
		if mutation.RequestID != 3 || mutation.UDFPath != "messages:send" || mutation.ComponentPath != "component" || len(mutation.Args) != 1 {
			t.Fatalf("decoded alias mutation fields: %#v", mutation)
		}

		action := assertClientMessageDecode[ActionMessage](t, []byte(`{"type":"Action","requestId":4,"RequestID":40,"udfPath":"jobs:run","UDFPath":"wrong:path","args":[true],"Args":[false],"componentPath":"component","ComponentPath":"wrong"}`))
		if action.RequestID != 4 || action.UDFPath != "jobs:run" || action.ComponentPath != "component" || len(action.Args) != 1 {
			t.Fatalf("decoded alias action fields: %#v", action)
		}

		auth := assertClientMessageDecode[AuthenticateMessage](t, []byte(`{"type":"Authenticate","baseVersion":5,"BaseVersion":50,"tokenType":"User","TokenType":"Admin","value":"jwt","Value":"wrong","actingAs":{"issuer":"issuer","subject":"subject"},"ActingAs":{"issuer":"wrong","subject":"wrong"}}`))
		if auth.BaseVersion != 5 || auth.TokenType != AuthTokenUser || auth.Value != "jwt" {
			t.Fatalf("decoded alias auth fields: %#v", auth)
		}
		if auth.ActingAs == nil || auth.ActingAs.TokenIdentifier != "issuer|subject" {
			t.Fatalf("decoded alias actingAs: %#v", auth.ActingAs)
		}

		event := assertClientMessageDecode[EventMessage](t, []byte(`{"type":"Event","eventType":"Trace","EventType":"Wrong","event":{"span":"abc"},"Event":{"span":"wrong"}}`))
		if event.EventType != "Trace" || string(event.Event) != `{"span":"abc"}` {
			t.Fatalf("decoded alias event fields: %#v", event)
		}
	})
}

func TestServerMessageDecodeUsesExactWireFieldNames(t *testing.T) {
	t.Run("transition ignores Go field aliases", func(t *testing.T) {
		raw := []byte(`{"type":"Transition","Type":"Ping","startVersion":{"querySet":1,"QuerySet":90,"identity":2,"Identity":91,"ts":"AQAAAAAAAAA=","TS":"AgAAAAAAAAA="},"StartVersion":{"querySet":99,"identity":99,"ts":"AwAAAAAAAAA="},"endVersion":{"querySet":3,"QuerySet":92,"identity":4,"Identity":93,"ts":"BAAAAAAAAAA=","TS":"BQAAAAAAAAA="},"EndVersion":{"querySet":98,"identity":98,"ts":"BgAAAAAAAAA="},"modifications":[{"type":"QueryUpdated","queryId":7,"QueryID":70,"value":{"ok":true},"Value":false,"logLines":["wire"],"LogLines":["alias"],"journal":"cursor"}],"Modifications":[{"type":"QueryRemoved","queryId":99}],"clientClockSkew":1.25,"ClientClockSkew":9.5,"serverTs":123,"ServerTS":999}`)
		transition := assertServerMessageDecode[TransitionMessage](t, raw)
		if transition.StartVersion.QuerySet != 1 || transition.StartVersion.Identity != 2 || transition.StartVersion.TS != 1 {
			t.Fatalf("decoded alias startVersion: %#v", transition.StartVersion)
		}
		if transition.EndVersion.QuerySet != 3 || transition.EndVersion.Identity != 4 || transition.EndVersion.TS != 4 {
			t.Fatalf("decoded alias endVersion: %#v", transition.EndVersion)
		}
		if transition.ClientClockSkew == nil || *transition.ClientClockSkew != 1.25 || transition.ServerTS == nil || *transition.ServerTS != 123 {
			t.Fatalf("decoded alias optional transition fields: %#v", transition)
		}
		if len(transition.Modifications) != 1 {
			t.Fatalf("decoded alias modifications: %#v", transition.Modifications)
		}
		updated := transition.Modifications[0].(QueryUpdated)
		if updated.QueryID != 7 || len(updated.LogLines) != 1 || updated.LogLines[0] != "wire" {
			t.Fatalf("decoded alias query update fields: %#v", updated)
		}
	})

	t.Run("responses and errors ignore Go field aliases", func(t *testing.T) {
		mutation := assertServerMessageDecode[MutationResponseMessage](t, []byte(`{"type":"MutationResponse","requestId":8,"RequestID":80,"success":true,"Success":false,"result":{"ok":true},"Result":"wrong","ts":"AQAAAAAAAAA=","TS":"AgAAAAAAAAA=","logLines":["wire"],"LogLines":["alias"]}`))
		if mutation.RequestID != 8 || !mutation.Success || mutation.TS == nil || *mutation.TS != 1 || len(mutation.LogLines) != 1 || mutation.LogLines[0] != "wire" {
			t.Fatalf("decoded alias mutation response fields: %#v", mutation)
		}

		action := assertServerMessageDecode[ActionResponseMessage](t, []byte(`{"type":"ActionResponse","requestId":9,"RequestID":90,"success":false,"Success":true,"result":"wire-error","Result":{"ok":true},"logLines":["wire"],"LogLines":["alias"],"errorData":{"reason":"wire"},"ErrorData":{"reason":"alias"}}`))
		if action.RequestID != 9 || action.Success || action.ErrorMessage != "wire-error" || len(action.LogLines) != 1 || action.LogLines[0] != "wire" {
			t.Fatalf("decoded alias action response fields: %#v", action)
		}

		auth := assertServerMessageDecode[AuthErrorMessage](t, []byte(`{"type":"AuthError","error":"wire-auth","Error":"alias-auth","baseVersion":10,"BaseVersion":90,"authUpdateAttempted":false,"AuthUpdateAttempted":true}`))
		if auth.Error != "wire-auth" || auth.BaseVersion == nil || *auth.BaseVersion != 10 || auth.AuthUpdateAttempted == nil || *auth.AuthUpdateAttempted {
			t.Fatalf("decoded alias auth error fields: %#v", auth)
		}

		fatal := assertServerMessageDecode[FatalErrorMessage](t, []byte(`{"type":"FatalError","error":"wire-fatal","Error":"alias-fatal"}`))
		if fatal.Error != "wire-fatal" {
			t.Fatalf("decoded alias fatal error field: %#v", fatal)
		}

		chunk := assertServerMessageDecode[TransitionChunkMessage](t, []byte(`{"type":"TransitionChunk","chunk":"wire","Chunk":"alias","partNumber":1,"PartNumber":90,"totalParts":2,"TotalParts":91,"transitionId":"transition","TransitionID":"wrong"}`))
		if chunk.Chunk != "wire" || chunk.PartNumber != 1 || chunk.TotalParts != 2 || chunk.TransitionID != "transition" {
			t.Fatalf("decoded alias transition chunk fields: %#v", chunk)
		}
	})
}

func assertClientMessageDecode[T ClientMessage](t *testing.T, raw []byte) T {
	t.Helper()
	msg, err := DecodeClientMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	typed, ok := msg.(T)
	if !ok {
		t.Fatalf("decoded type mismatch: got %T want %T", msg, *new(T))
	}
	return typed
}

func assertServerMessageDecode[T ServerMessage](t *testing.T, raw []byte) T {
	t.Helper()
	msg, err := DecodeServerMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	typed, ok := msg.(T)
	if !ok {
		t.Fatalf("decoded type mismatch: got %T want %T", msg, *new(T))
	}
	return typed
}

func assertClientMessageRoundTrip(t *testing.T, raw []byte, want any) ClientMessage {
	t.Helper()
	msg, err := DecodeClientMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.TypeOf(msg) != reflect.TypeOf(want) {
		t.Fatalf("decoded type mismatch: got %T want %T", msg, want)
	}
	encoded, err := EncodeClientMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, raw, encoded)
	return msg
}

func assertServerMessageRoundTrip[T ServerMessage](t *testing.T, raw []byte) T {
	t.Helper()
	msg := assertServerMessageRoundTripAny(t, raw, *new(T))
	typed, ok := msg.(T)
	if !ok {
		t.Fatalf("decoded type mismatch: got %T want %T", msg, *new(T))
	}
	return typed
}

func assertServerMessageRoundTripAny(t *testing.T, raw []byte, want any) ServerMessage {
	t.Helper()
	msg, err := DecodeServerMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.TypeOf(msg) != reflect.TypeOf(want) {
		t.Fatalf("decoded type mismatch: got %T want %T", msg, want)
	}
	encoded, err := EncodeServerMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, raw, encoded)
	return msg
}

func assertAdminActingAsWhenPresent(t *testing.T, msg ClientMessage) {
	t.Helper()
	auth, ok := msg.(AuthenticateMessage)
	if !ok || auth.TokenType != AuthTokenAdmin {
		return
	}
	if auth.ActingAs == nil || auth.ActingAs.TokenIdentifier != "issuer|subject" {
		t.Fatalf("unexpected actingAs identity: %#v", auth.ActingAs)
	}
}

func assertJSONEqual(t *testing.T, want, got []byte) {
	t.Helper()
	var wantJSON any
	if err := json.Unmarshal(want, &wantJSON); err != nil {
		t.Fatalf("bad want JSON: %v", err)
	}
	var gotJSON any
	if err := json.Unmarshal(got, &gotJSON); err != nil {
		t.Fatalf("bad got JSON %s: %v", got, err)
	}
	if !reflect.DeepEqual(gotJSON, wantJSON) {
		t.Fatalf("JSON mismatch\ngot:  %s\nwant: %s", got, want)
	}
}

func assertJSONHas(t *testing.T, raw []byte, field string) {
	t.Helper()
	fields := decodeJSONObject(t, raw)
	if _, ok := fields[field]; !ok {
		t.Fatalf("expected JSON field %q in %s", field, raw)
	}
}

func assertJSONMissing(t *testing.T, raw []byte, field string) {
	t.Helper()
	fields := decodeJSONObject(t, raw)
	if _, ok := fields[field]; ok {
		t.Fatalf("expected JSON field %q to be absent in %s", field, raw)
	}
}

func decodeJSONObject(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("bad JSON object %s: %v", raw, err)
	}
	return fields
}
