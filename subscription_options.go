package convex

import (
	"errors"
	"time"

	"github.com/cervantesh/convex-go/internal/syncclient"
)

var ErrSubscriptionClosed = errors.New("convex: subscription closed")

var defaultRealtimeControlTimeout = time.Second
var defaultQueryCleanupTimeout = time.Second

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

func WithWebSocketClientID(clientID string) WebSocketOption {
	return webSocketOptionFunc(func(options *webSocketOptions) error {
		options.managerOptions = append(options.managerOptions, syncclient.WithClientID(clientID))
		return nil
	})
}

func WithWebSocketReconnectBackoff(backoff time.Duration) WebSocketOption {
	return webSocketOptionFunc(func(options *webSocketOptions) error {
		options.managerOptions = append(options.managerOptions, syncclient.WithReconnectBackoff(backoff))
		return nil
	})
}

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
