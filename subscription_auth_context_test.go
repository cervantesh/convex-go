package convex

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWebSocketClientSetAuthContextReturnsContextErrorWhenFlushBlocked(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)

	blocked, _ := attempt.conn.blockNextWrite()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- client.SetAuthContext(ctx, "user-token")
	}()

	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocked auth flush")
	}

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline exceeded auth flush, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for auth flush error")
	}
}

func TestClientSetAuthContextReturnsContextErrorWhenRealtimeFlushBlocked(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, err := NewClient(
		"https://happy-animal-123.convex.cloud",
		withWebSocketDialer(dialer),
		WithWebSocketReconnectBackoff(0),
		WithWebSocketInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	subscription, err := client.Subscribe(context.Background(), "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	_ = decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))
	_ = subscription

	blocked, _ := attempt.conn.blockNextWrite()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- client.SetAuthContext(ctx, "user-token")
	}()

	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocked root auth flush")
	}

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline exceeded root auth flush, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for root auth flush error")
	}
}

func TestWebSocketClientSetAdminAuthContextUsesUserIdentityAttributes(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)

	err := client.SetAdminAuthContext(context.Background(), "admin-token", UserIdentityAttributes{
		"issuer":  "issuer",
		"subject": "subject",
	})
	if err != nil {
		t.Fatal(err)
	}

	auth := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if auth.TokenType != "Admin" || auth.Value != "admin-token" {
		t.Fatalf("unexpected admin auth message: %#v", auth)
	}
	if auth.ActingAs == nil || auth.ActingAs.TokenIdentifier != "issuer|subject" {
		t.Fatalf("unexpected acting-as identity: %#v", auth.ActingAs)
	}
}

func TestWebSocketClientSetAuthUsesBoundedContext(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)

	if err := client.SetAuth("user-token"); err != nil {
		t.Fatal(err)
	}

	auth := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if auth.TokenType != "User" || auth.Value != "user-token" {
		t.Fatalf("unexpected user auth message: %#v", auth)
	}
}

func TestWebSocketClientSetAdminAuthUsesBoundedContext(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)

	err := client.SetAdminAuth("admin-token", UserIdentityAttributes{
		"issuer":  "issuer",
		"subject": "subject",
	})
	if err != nil {
		t.Fatal(err)
	}

	auth := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if auth.TokenType != "Admin" || auth.Value != "admin-token" {
		t.Fatalf("unexpected admin auth message: %#v", auth)
	}
	if auth.ActingAs == nil || auth.ActingAs.TokenIdentifier != "issuer|subject" {
		t.Fatalf("unexpected acting-as identity: %#v", auth.ActingAs)
	}
}

func TestWebSocketClientClearAuthUsesBoundedContext(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)

	if err := client.ClearAuth(); err != nil {
		t.Fatal(err)
	}

	auth := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if auth.TokenType != "None" || auth.Value != "" {
		t.Fatalf("unexpected clear auth message: %#v", auth)
	}
}

func TestBoundedTimeoutUsesMinimumForNonPositiveDurations(t *testing.T) {
	if got := boundedTimeout(0); got != time.Millisecond {
		t.Fatalf("boundedTimeout(0) = %s, want %s", got, time.Millisecond)
	}
	if got := boundedTimeout(-time.Second); got != time.Millisecond {
		t.Fatalf("boundedTimeout(-1s) = %s, want %s", got, time.Millisecond)
	}
	if got := boundedTimeout(2 * time.Second); got != 2*time.Second {
		t.Fatalf("boundedTimeout(2s) = %s, want %s", got, 2*time.Second)
	}
}
