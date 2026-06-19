package convex

import "github.com/cervantesh/convex-go/internal/syncclient"

// ConnectionState reports the current realtime transport snapshot.
func (c *WebSocketClient) ConnectionState() ConnectionState {
	return connectionStateFromSync(c.manager.ConnectionState())
}

// SubscribeToConnectionState invokes cb immediately with the current realtime
// snapshot and again when the state changes. It returns an unsubscribe
// function that is safe to call more than once.
func (c *WebSocketClient) SubscribeToConnectionState(cb func(ConnectionState)) func() {
	if cb == nil {
		return func() {}
	}
	return c.manager.SubscribeToConnectionState(func(state syncclient.ConnectionState) {
		cb(connectionStateFromSync(state))
	})
}
