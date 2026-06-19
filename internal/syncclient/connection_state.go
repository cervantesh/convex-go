package syncclient

import "time"

type ConnectionPhase string

const (
	ConnectionPhaseDisconnected ConnectionPhase = "Disconnected"
	ConnectionPhaseConnecting   ConnectionPhase = "Connecting"
	ConnectionPhaseConnected    ConnectionPhase = "Connected"
	ConnectionPhaseReconnecting ConnectionPhase = "Reconnecting"
)

type ConnectionState struct {
	Phase             ConnectionPhase
	HasEverConnected  bool
	ConnectionCount   uint64
	ConnectionRetries uint64
	LastChange        time.Time
}

func (m *Manager) ConnectionState() ConnectionState {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return m.connectionState
}

func (m *Manager) SubscribeToConnectionState(cb func(ConnectionState)) func() {
	if cb == nil {
		return func() {}
	}
	m.stateMu.Lock()
	id := m.nextConnectionStateSubscriberID
	m.nextConnectionStateSubscriberID++
	m.connectionStateSubscribers[id] = cb
	state := m.connectionState
	m.stateMu.Unlock()
	cb(state)
	return func() {
		m.stateMu.Lock()
		delete(m.connectionStateSubscribers, id)
		m.stateMu.Unlock()
	}
}

func (m *Manager) markConnecting() {
	m.updateConnectionState(func(state ConnectionState) ConnectionState {
		state.Phase = ConnectionPhaseConnecting
		return state
	})
}

func (m *Manager) markConnected() {
	m.updateConnectionState(func(state ConnectionState) ConnectionState {
		state.Phase = ConnectionPhaseConnected
		state.HasEverConnected = true
		state.ConnectionCount++
		return state
	})
}

func (m *Manager) markReconnecting() {
	m.updateConnectionState(func(state ConnectionState) ConnectionState {
		state.Phase = ConnectionPhaseReconnecting
		state.ConnectionRetries++
		return state
	})
}

func (m *Manager) markDisconnected() {
	m.updateConnectionState(func(state ConnectionState) ConnectionState {
		state.Phase = ConnectionPhaseDisconnected
		return state
	})
}

func (m *Manager) updateConnectionState(update func(ConnectionState) ConnectionState) {
	m.stateMu.Lock()
	next := update(m.connectionState)
	if next.Phase == "" {
		next.Phase = ConnectionPhaseDisconnected
	}
	if connectionStateEqual(m.connectionState, next) {
		m.stateMu.Unlock()
		return
	}
	next.LastChange = time.Now()
	m.connectionState = next
	callbacks := make([]func(ConnectionState), 0, len(m.connectionStateSubscribers))
	for _, cb := range m.connectionStateSubscribers {
		callbacks = append(callbacks, cb)
	}
	m.stateMu.Unlock()
	for _, cb := range callbacks {
		cb(next)
	}
}

func connectionStateEqual(left ConnectionState, right ConnectionState) bool {
	return left.Phase == right.Phase &&
		left.HasEverConnected == right.HasEverConnected &&
		left.ConnectionCount == right.ConnectionCount &&
		left.ConnectionRetries == right.ConnectionRetries
}
