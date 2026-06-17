package baseclient

import (
	"errors"
	"testing"
)

func TestAuthCallbackIdentityVersioningStaticAndClear(t *testing.T) {
	client := New()

	if err := client.SetAuth("jwt-token"); err != nil {
		t.Fatal(err)
	}
	user := popAuthenticateMessage(t, client)
	if user.BaseVersion != 0 || user.TokenType != AuthTokenUser || user.Value != "jwt-token" {
		t.Fatalf("unexpected user auth message: %#v", user)
	}

	if err := client.ClearAuth(); err != nil {
		t.Fatal(err)
	}
	none := popAuthenticateMessage(t, client)
	if none.BaseVersion != 1 || none.TokenType != AuthTokenNone || none.Value != "" {
		t.Fatalf("unexpected clear auth message: %#v", none)
	}

	if err := client.SetAuth("next-token"); err != nil {
		t.Fatal(err)
	}
	next := popAuthenticateMessage(t, client)
	if next.BaseVersion != 2 || next.TokenType != AuthTokenUser || next.Value != "next-token" {
		t.Fatalf("unexpected next auth message: %#v", next)
	}
}

func TestAuthCallbackIdentityVersioningStaticAuthRefreshesOnReconnect(t *testing.T) {
	client := New()

	if err := client.SetAuth("jwt-token"); err != nil {
		t.Fatal(err)
	}
	_ = popAuthenticateMessage(t, client)
	if err := client.RefreshAuthForReconnect(); err != nil {
		t.Fatal(err)
	}
	refreshed := popAuthenticateMessage(t, client)
	if refreshed.BaseVersion != 1 || refreshed.TokenType != AuthTokenUser || refreshed.Value != "jwt-token" {
		t.Fatalf("unexpected static reconnect auth: %#v", refreshed)
	}
}

func TestAuthCallbackIdentityVersioningCallbackAndReconnectRefresh(t *testing.T) {
	client := New()
	var calls []bool

	if err := client.SetAuthCallback(func(forceRefresh bool) (AuthToken, error) {
		calls = append(calls, forceRefresh)
		if forceRefresh {
			return UserAuthToken("fresh-token"), nil
		}
		return UserAuthToken("initial-token"), nil
	}); err != nil {
		t.Fatal(err)
	}
	initial := popAuthenticateMessage(t, client)
	if initial.BaseVersion != 0 || initial.TokenType != AuthTokenUser || initial.Value != "initial-token" {
		t.Fatalf("unexpected initial auth message: %#v", initial)
	}

	if err := client.RefreshAuthForReconnect(); err != nil {
		t.Fatal(err)
	}
	refreshed := popAuthenticateMessage(t, client)
	if refreshed.BaseVersion != 1 || refreshed.TokenType != AuthTokenUser || refreshed.Value != "fresh-token" {
		t.Fatalf("unexpected reconnect auth message: %#v", refreshed)
	}
	if len(calls) != 2 || calls[0] != false || calls[1] != true {
		t.Fatalf("unexpected fetcher calls: %#v", calls)
	}
}

func TestAuthCallbackIdentityVersioningCallbackErrorDoesNotQueue(t *testing.T) {
	client := New()
	wantErr := errors.New("no token")

	err := client.SetAuthCallback(func(forceRefresh bool) (AuthToken, error) {
		if forceRefresh {
			t.Fatal("unexpected force refresh")
		}
		return AuthToken{}, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected callback error, got %v", err)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected no auth message after callback error, got %#v", msg)
	}

	if err := client.RefreshAuthForReconnect(); err == nil {
		t.Fatal("expected no stored callback after failed SetAuthCallback")
	}
}

func TestAuthCallbackIdentityVersioningCallbackErrorPreservesPreviousProvider(t *testing.T) {
	client := New()
	wantErr := errors.New("new provider failed")

	if err := client.SetAuth("old-token"); err != nil {
		t.Fatal(err)
	}
	_ = popAuthenticateMessage(t, client)
	err := client.SetAuthCallback(func(forceRefresh bool) (AuthToken, error) {
		return AuthToken{}, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected callback error, got %v", err)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected no message from failed callback, got %#v", msg)
	}

	if err := client.RefreshAuthForReconnect(); err != nil {
		t.Fatal(err)
	}
	refreshed := popAuthenticateMessage(t, client)
	if refreshed.BaseVersion != 1 || refreshed.TokenType != AuthTokenUser || refreshed.Value != "old-token" {
		t.Fatalf("expected previous provider to remain active, got %#v", refreshed)
	}
}

func TestAuthCallbackIdentityVersioningCallbackCanReturnAdminAndNone(t *testing.T) {
	client := New()
	call := 0

	if err := client.SetAuthCallback(func(forceRefresh bool) (AuthToken, error) {
		call++
		if !forceRefresh {
			return AdminAuthToken("admin-token", SyncUserIdentityAttributes{Issuer: "issuer", Subject: "subject"}), nil
		}
		return NoAuthToken(), nil
	}); err != nil {
		t.Fatal(err)
	}
	admin := popAuthenticateMessage(t, client)
	if admin.BaseVersion != 0 || admin.TokenType != AuthTokenAdmin || admin.Value != "admin-token" {
		t.Fatalf("unexpected admin callback auth: %#v", admin)
	}
	if admin.ActingAs == nil || admin.ActingAs.TokenIdentifier != "issuer|subject" {
		t.Fatalf("unexpected actingAs: %#v", admin.ActingAs)
	}

	if err := client.RefreshAuthForReconnect(); err != nil {
		t.Fatal(err)
	}
	none := popAuthenticateMessage(t, client)
	if none.BaseVersion != 1 || none.TokenType != AuthTokenNone {
		t.Fatalf("unexpected none callback auth: %#v", none)
	}
	if call != 2 {
		t.Fatalf("unexpected callback count: %d", call)
	}
}

func TestAuthTokenConstructorsDoNotCreateEmptyActingAs(t *testing.T) {
	auth := AdminAuthToken("admin-token")
	if auth.TokenType != AuthTokenAdmin || auth.Value != "admin-token" {
		t.Fatalf("unexpected admin token: %#v", auth)
	}
	if auth.ActingAs != nil {
		t.Fatalf("expected no acting-as identity without arguments, got %#v", auth.ActingAs)
	}
}

func TestAuthTokenConstructorsPreserveExplicitTokenIdentifier(t *testing.T) {
	auth := AdminAuthToken("admin-token", SyncUserIdentityAttributes{
		TokenIdentifier: "custom-id",
		Issuer:          "issuer",
		Subject:         "subject",
	})
	if auth.ActingAs == nil {
		t.Fatal("expected acting-as identity")
	}
	if auth.ActingAs.TokenIdentifier != "custom-id" {
		t.Fatalf("expected explicit token identifier to be preserved, got %#v", auth.ActingAs)
	}
}

func TestAuthTokenConstructorsRequireIssuerAndSubjectForDerivedTokenIdentifier(t *testing.T) {
	tests := []SyncUserIdentityAttributes{
		{Issuer: "issuer"},
		{Subject: "subject"},
	}
	for _, identity := range tests {
		auth := AdminAuthToken("admin-token", identity)
		if auth.ActingAs == nil {
			t.Fatal("expected acting-as identity")
		}
		if auth.ActingAs.TokenIdentifier != "" {
			t.Fatalf("expected no derived token identifier for partial identity, got %#v", auth.ActingAs)
		}
	}
}

func TestAuthCallbackIdentityVersioningNilCallbackClearsAuth(t *testing.T) {
	client := New()

	if err := client.SetAuth("jwt-token"); err != nil {
		t.Fatal(err)
	}
	_ = popAuthenticateMessage(t, client)
	if err := client.SetAuthCallback(nil); err != nil {
		t.Fatal(err)
	}
	none := popAuthenticateMessage(t, client)
	if none.BaseVersion != 1 || none.TokenType != AuthTokenNone {
		t.Fatalf("unexpected clear via nil callback: %#v", none)
	}
	if err := client.RefreshAuthForReconnect(); err == nil {
		t.Fatal("expected no callback after nil callback clear")
	}
}

func TestAuthCallbackIdentityVersioningAdminActingAs(t *testing.T) {
	client := New()

	if err := client.SetAdminAuth("admin-token", SyncUserIdentityAttributes{
		Issuer:  "issuer",
		Subject: "subject",
	}); err != nil {
		t.Fatal(err)
	}
	admin := popAuthenticateMessage(t, client)
	if admin.BaseVersion != 0 || admin.TokenType != AuthTokenAdmin || admin.Value != "admin-token" {
		t.Fatalf("unexpected admin auth message: %#v", admin)
	}
	if admin.ActingAs == nil || admin.ActingAs.TokenIdentifier != "issuer|subject" {
		t.Fatalf("unexpected actingAs identity: %#v", admin.ActingAs)
	}
}

func TestAuthCallbackIdentityVersioningAdminActingAsPreservesExplicitTokenIdentifier(t *testing.T) {
	client := New()

	if err := client.SetAdminAuth("admin-token", SyncUserIdentityAttributes{
		TokenIdentifier: "custom-id",
		Issuer:          "issuer",
		Subject:         "subject",
	}); err != nil {
		t.Fatal(err)
	}
	admin := popAuthenticateMessage(t, client)
	if admin.ActingAs == nil || admin.ActingAs.TokenIdentifier != "custom-id" {
		t.Fatalf("expected explicit token identifier to be preserved, got %#v", admin.ActingAs)
	}
}

func TestAuthCallbackIdentityVersioningAuthErrorIsTyped(t *testing.T) {
	client := New()
	baseVersion := IdentityVersion(7)
	attempted := true

	_, err := client.ReceiveMessage(AuthErrorMessage{
		Error:               "bad auth",
		BaseVersion:         &baseVersion,
		AuthUpdateAttempted: &attempted,
	})
	var authErr *SyncAuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected SyncAuthError, got %T %v", err, err)
	}
	if authErr.Message != "bad auth" || authErr.BaseVersion == nil || *authErr.BaseVersion != baseVersion {
		t.Fatalf("unexpected auth error metadata: %#v", authErr)
	}
	if authErr.AuthUpdateAttempted == nil || *authErr.AuthUpdateAttempted != true {
		t.Fatalf("unexpected auth attempted metadata: %#v", authErr)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("auth error should not queue auth automatically, got %#v", msg)
	}
}

func popAuthenticateMessage(t *testing.T, client *Client) AuthenticateMessage {
	t.Helper()
	msg := client.PopNextMessage()
	auth, ok := msg.(AuthenticateMessage)
	if !ok {
		t.Fatalf("expected AuthenticateMessage, got %T %#v", msg, msg)
	}
	return auth
}
