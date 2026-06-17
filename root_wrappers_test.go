package convex

import (
	"context"
	"testing"
)

func TestRootValueAndPathWrappers(t *testing.T) {
	if NullValue().Kind() != NullKind {
		t.Fatal("NullValue wrapper returned non-null value")
	}
	if value, ok := Float64Value(1.5).Float64(); !ok || value != 1.5 {
		t.Fatalf("unexpected Float64Value wrapper result: %v ok=%v", value, ok)
	}
	if value, ok := BooleanValue(true).Bool(); !ok || !value {
		t.Fatalf("unexpected BooleanValue wrapper result: %v ok=%v", value, ok)
	}
	if value, ok := BytesValue([]byte("abc")).Bytes(); !ok || string(value) != "abc" {
		t.Fatalf("unexpected BytesValue wrapper result: %q ok=%v", value, ok)
	}
	array := ArrayValue([]Value{StringValue("x")})
	if values, ok := array.Array(); !ok || len(values) != 1 || values[0].GoValue() != "x" {
		t.Fatalf("unexpected ArrayValue wrapper result: %#v ok=%v", values, ok)
	}
	object, err := ObjectValue(map[string]Value{"ok": BooleanValue(true)})
	if err != nil {
		t.Fatal(err)
	}
	if fields, ok := object.Object(); !ok || fields["ok"].GoValue() != true {
		t.Fatalf("unexpected ObjectValue wrapper result: %#v ok=%v", fields, ok)
	}
	fromGo, err := ValueFromGo(map[string]any{"count": Int64(3)})
	if err != nil {
		t.Fatal(err)
	}
	if fromGo.Kind() != ObjectKind {
		t.Fatalf("unexpected ValueFromGo wrapper kind: %s", fromGo.Kind())
	}
	if encoded, err := EncodeValue(map[string]any{"count": Int64(3)}); err != nil || encoded == nil {
		t.Fatalf("unexpected EncodeValue wrapper result: %#v err=%v", encoded, err)
	}
	decoded, err := DecodeJSON([]byte(`{"count":{"$integer":"AwAAAAAAAAA="}}`))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.(map[string]any)["count"] != Int64(3) {
		t.Fatalf("unexpected DecodeJSON wrapper result: %#v", decoded)
	}
	value, err := DecodeValue(map[string]any{"$integer": "BAAAAAAAAAA="})
	if err != nil {
		t.Fatal(err)
	}
	if value != Int64(4) {
		t.Fatalf("unexpected DecodeValue wrapper result: %#v", value)
	}

	if _, err := ParseFunctionName("list"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseModulePath("messages"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseUDFPath("messages:list"); err != nil {
		t.Fatal(err)
	}
	if got, err := ParseFunctionPath("api.messages.list"); err != nil || got != "messages:list" {
		t.Fatalf("unexpected ParseFunctionPath wrapper result: %q err=%v", got, err)
	}
	if err := ValidateIdentifier("valid_name"); err != nil {
		t.Fatal(err)
	}
}

func TestRootSyncAuthTokenWrappers(t *testing.T) {
	user := UserAuthToken("user-token")
	if user.Value != "user-token" {
		t.Fatalf("unexpected user auth token: %#v", user)
	}
	admin := AdminAuthToken("admin-token", SyncUserIdentityAttributes{Issuer: "issuer", Subject: "subject"})
	if admin.Value != "admin-token" || admin.ActingAs == nil || admin.ActingAs.TokenIdentifier != "issuer|subject" {
		t.Fatalf("unexpected admin auth token: %#v", admin)
	}
	if none := NoAuthToken(); none.Value != "" {
		t.Fatalf("unexpected no-auth token: %#v", none)
	}
}

func TestRootWebSocketClientIDOption(t *testing.T) {
	if _, err := NewWebSocketClient(context.Background(), "https://happy-animal-123.convex.cloud", WithWebSocketClientID(" ")); err == nil {
		t.Fatal("expected invalid websocket client ID option to fail")
	}
}
