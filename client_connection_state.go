package convex

// ConnectionState reports the current realtime state for the client's lazy
// WebSocket component. If realtime has not started yet, it returns Disconnected.
func (c *Client) ConnectionState() ConnectionState {
	c.mu.Lock()
	realtime := c.webSocketClient
	c.mu.Unlock()
	if realtime == nil {
		return disconnectedConnectionState()
	}
	return realtime.ConnectionState()
}

// SubscribeToConnectionState invokes cb immediately with the current snapshot
// and again when the realtime state changes. It returns an unsubscribe
// function that is safe to call more than once.
func (c *Client) SubscribeToConnectionState(cb func(ConnectionState)) func() {
	if cb == nil {
		return func() {}
	}
	observer := &connectionStateObserver{callback: cb}

	c.mu.Lock()
	if c.connectionStateObservers == nil {
		c.connectionStateObservers = map[uint64]*connectionStateObserver{}
	}
	id := c.nextConnectionStateObserverID
	c.nextConnectionStateObserverID++
	c.connectionStateObservers[id] = observer
	realtime := c.webSocketClient
	c.mu.Unlock()

	c.forwardConnectionState(id, c.ConnectionState())
	if realtime != nil {
		c.attachConnectionStateObserver(id, realtime)
	}

	return func() {
		c.mu.Lock()
		observer, ok := c.connectionStateObservers[id]
		if !ok {
			c.mu.Unlock()
			return
		}
		unsubscribe := observer.unsubscribe
		delete(c.connectionStateObservers, id)
		c.mu.Unlock()
		if unsubscribe != nil {
			unsubscribe()
		}
	}
}

func (c *Client) attachConnectionStateObservers(realtime *WebSocketClient) {
	c.mu.Lock()
	ids := make([]uint64, 0, len(c.connectionStateObservers))
	for id, observer := range c.connectionStateObservers {
		if observer == nil || observer.attaching || observer.unsubscribe != nil {
			continue
		}
		ids = append(ids, id)
	}
	c.mu.Unlock()
	for _, id := range ids {
		c.attachConnectionStateObserver(id, realtime)
	}
}

func (c *Client) attachConnectionStateObserver(id uint64, realtime *WebSocketClient) {
	c.mu.Lock()
	observer, ok := c.connectionStateObservers[id]
	if !ok || observer == nil || observer.attaching || observer.unsubscribe != nil {
		c.mu.Unlock()
		return
	}
	observer.attaching = true
	c.mu.Unlock()

	unsubscribe := realtime.SubscribeToConnectionState(func(state ConnectionState) {
		c.forwardConnectionState(id, state)
	})

	c.mu.Lock()
	observer, ok = c.connectionStateObservers[id]
	if !ok || observer == nil {
		c.mu.Unlock()
		unsubscribe()
		return
	}
	observer.attaching = false
	if observer.unsubscribe != nil {
		existing := observer.unsubscribe
		c.mu.Unlock()
		existing()
		unsubscribe()
		return
	}
	observer.unsubscribe = unsubscribe
	c.mu.Unlock()
}

func (c *Client) clearConnectionStateObserverAttachments() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, observer := range c.connectionStateObservers {
		if observer == nil {
			continue
		}
		observer.unsubscribe = nil
		observer.attaching = false
	}
}

func (c *Client) forwardConnectionState(id uint64, state ConnectionState) {
	c.mu.Lock()
	observer, ok := c.connectionStateObservers[id]
	if !ok || observer == nil {
		c.mu.Unlock()
		return
	}
	if observer.hasLast && observer.last == state {
		c.mu.Unlock()
		return
	}
	observer.last = state
	observer.hasLast = true
	cb := observer.callback
	c.mu.Unlock()
	cb(state)
}
