package convex

import "github.com/cervantesh/convex-go/internal/syncclient"

func (c *WebSocketClient) ConnectionState() ConnectionState {
	return connectionStateFromSync(c.manager.ConnectionState())
}

func (c *WebSocketClient) SubscribeToConnectionState(cb func(ConnectionState)) func() {
	if cb == nil {
		return func() {}
	}
	return c.manager.SubscribeToConnectionState(func(state syncclient.ConnectionState) {
		cb(connectionStateFromSync(state))
	})
}
