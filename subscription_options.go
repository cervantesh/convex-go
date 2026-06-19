package convex

import (
	"errors"
	"time"

	"github.com/cervantesh/convex-go/internal/syncclient"
)

// ErrSubscriptionClosed reports that a subscription or watcher has been closed.
var ErrSubscriptionClosed = errors.New("convex: subscription closed")

var defaultRealtimeControlTimeout = time.Second
var defaultQueryCleanupTimeout = time.Second

// WebSocketOption configures a WebSocketClient or the realtime portion of Client.
type WebSocketOption interface {
	Option
	applyWebSocketOption(*webSocketOptions) error
}

type webSocketOptions struct {
	managerOptions []syncclient.Option
}

type webSocketOptionFunc func(*webSocketOptions) error

func (f webSocketOptionFunc) applyWebSocketOption(options *webSocketOptions) error {
	return f(options)
}

func (f webSocketOptionFunc) applyHTTPOption(*HTTPClient) error {
	return nil
}

func (f webSocketOptionFunc) applyClientOption(client *Client) error {
	client.webSocketOptions = append(client.webSocketOptions, f)
	return nil
}

// WithWebSocketClientID overrides the client id used by the realtime transport.
func WithWebSocketClientID(clientID string) WebSocketOption {
	return webSocketOptionFunc(func(options *webSocketOptions) error {
		options.managerOptions = append(options.managerOptions, syncclient.WithClientID(clientID))
		return nil
	})
}

// WithWebSocketReconnectBackoff overrides the fixed reconnect delay used by
// the realtime transport.
func WithWebSocketReconnectBackoff(backoff time.Duration) WebSocketOption {
	return webSocketOptionFunc(func(options *webSocketOptions) error {
		options.managerOptions = append(options.managerOptions, syncclient.WithReconnectBackoff(backoff))
		return nil
	})
}

// WithWebSocketInactivityTimeout overrides the inactivity timeout used by the
// realtime transport.
func WithWebSocketInactivityTimeout(timeout time.Duration) WebSocketOption {
	return webSocketOptionFunc(func(options *webSocketOptions) error {
		options.managerOptions = append(options.managerOptions, syncclient.WithInactivityTimeout(timeout))
		return nil
	})
}

func withWebSocketDialer(dialer syncclient.Dialer) WebSocketOption {
	return webSocketOptionFunc(func(options *webSocketOptions) error {
		options.managerOptions = append(options.managerOptions, syncclient.WithDialer(dialer))
		return nil
	})
}
