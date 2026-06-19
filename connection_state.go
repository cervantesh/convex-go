package convex

import (
	"time"

	"github.com/cervantesh/convex-go/internal/syncclient"
)

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

type connectionStateObserver struct {
	callback    func(ConnectionState)
	unsubscribe func()
	last        ConnectionState
	hasLast     bool
	attaching   bool
}

func disconnectedConnectionState() ConnectionState {
	return ConnectionState{Phase: ConnectionPhaseDisconnected}
}

func connectionStateFromSync(state syncclient.ConnectionState) ConnectionState {
	return ConnectionState{
		Phase:             ConnectionPhase(state.Phase),
		HasEverConnected:  state.HasEverConnected,
		ConnectionCount:   state.ConnectionCount,
		ConnectionRetries: state.ConnectionRetries,
		LastChange:        state.LastChange,
	}
}
