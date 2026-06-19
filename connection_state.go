package convex

import (
	"time"

	"github.com/cervantesh/convex-go/internal/syncclient"
)

// ConnectionPhase describes the public realtime connection phase.
type ConnectionPhase string

const (
	// ConnectionPhaseDisconnected reports that the realtime transport is not connected.
	ConnectionPhaseDisconnected ConnectionPhase = "Disconnected"
	// ConnectionPhaseConnecting reports that the realtime transport is dialing.
	ConnectionPhaseConnecting   ConnectionPhase = "Connecting"
	// ConnectionPhaseConnected reports that the realtime transport is connected.
	ConnectionPhaseConnected    ConnectionPhase = "Connected"
	// ConnectionPhaseReconnecting reports that the realtime transport is retrying.
	ConnectionPhaseReconnecting ConnectionPhase = "Reconnecting"
)

// ConnectionState is a stable snapshot of the public realtime connection state.
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
