package syncclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"reflect"
	"time"

	coderwebsocket "github.com/coder/websocket"

	"github.com/cervantesh/convex-go/baseclient"
	"github.com/cervantesh/convex-go/internal/syncprotocol"
)

type websocketRunState struct {
	sessionID       string
	connectionCount uint32
	lastCloseReason string
	needsRestart    bool
}

func newWebSocketRunState() (websocketRunState, error) {
	sessionID, err := newSyncSessionID()
	if err != nil {
		return websocketRunState{}, err
	}
	return websocketRunState{
		sessionID:       sessionID,
		lastCloseReason: "InitialConnect",
	}, nil
}

func (m *Manager) prepareNextConnection(ctx context.Context, state *websocketRunState) error {
	if !state.needsRestart {
		return nil
	}
	return m.prepareReconnect(ctx)
}

func (m *Manager) handleDialFailure(ctx context.Context, state *websocketRunState, err error) error {
	m.markReconnecting()
	if sleepErr := sleepContext(ctx, m.reconnectBackoff); sleepErr != nil {
		return sleepErr
	}
	state.connectionCount++
	state.lastCloseReason = err.Error()
	return nil
}

func (m *Manager) writeLoop(ctx context.Context, conn Conn, requests <-chan flushRequest, errCh chan<- writeResult) {
	for {
		select {
		case request := <-requests:
			err := m.flushQueued(ctx, conn)
			sendFlushAck(request.ack, err)
			if err != nil {
				ackPendingFlushRequests(requests, err)
				select {
				case errCh <- writeResult{err: err, replay: request.replay}:
				default:
				}
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func ackPendingFlushRequests(requests <-chan flushRequest, err error) {
	for {
		select {
		case request := <-requests:
			sendFlushAck(request.ack, err)
		default:
			return
		}
	}
}

func sendFlushAck(ack chan error, err error) {
	select {
	case ack <- err:
	default:
	}
}

func (m *Manager) receiveServerMessage(ctx context.Context, data []byte) error {
	msg, err := syncprotocol.DecodeServerMessage(data)
	if err != nil {
		return err
	}
	if _, ok := msg.(syncprotocol.PingMessage); ok {
		return nil
	}
	m.mu.Lock()
	results, err := m.client.ReceiveMessage(msg)
	m.mu.Unlock()
	if err != nil {
		return err
	}
	if results == nil {
		return nil
	}
	select {
	case m.results <- *results:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) publishLatest(results baseclient.QueryResults) {
	sendLatest(m.results, results)
}

func queryResultsEqual(left, right baseclient.QueryResults) bool {
	return reflect.DeepEqual(left.Iter(), right.Iter())
}

func (m *Manager) flushQueued(ctx context.Context, conn Conn) error {
	for {
		m.mu.Lock()
		msg := m.client.PopNextMessage()
		m.mu.Unlock()
		if msg == nil {
			return nil
		}
		if err := m.writeClientMessage(ctx, conn, msg); err != nil {
			return err
		}
	}
}

func (m *Manager) writeClientMessage(ctx context.Context, conn Conn, msg syncprotocol.ClientMessage) error {
	data, err := syncprotocol.EncodeClientMessage(msg)
	if err != nil {
		return err
	}
	return conn.Write(ctx, data)
}

func (m *Manager) maxObservedTimestamp() *syncprotocol.SyncTimestamp {
	m.mu.Lock()
	defer m.mu.Unlock()
	ts, ok := m.client.MaxObservedTimestamp()
	if !ok {
		return nil
	}
	return &ts
}

type readResult struct {
	data []byte
	err  error
}

type flushRequest struct {
	ack    chan error
	replay bool
}

type writeResult struct {
	err    error
	replay bool
}

func readLoop(ctx context.Context, conn Conn, out chan<- readResult) {
	for {
		data, err := conn.Read(ctx)
		select {
		case out <- readResult{data: data, err: err}:
		case <-ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

func sendLatest[T any](ch chan T, value T) {
	select {
	case ch <- value:
	default:
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- value:
		default:
		}
	}
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

type reconnectableWebSocketError struct {
	err    error
	replay bool
}

func (e reconnectableWebSocketError) Error() string {
	if e.err == nil {
		return "convex: websocket reconnect"
	}
	return e.err.Error()
}

func (e reconnectableWebSocketError) Unwrap() error {
	return e.err
}

func resetTimer(timer *time.Timer, timeout time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(timeout)
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	if duration == 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func newSyncSessionID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return formatSyncSessionID(bytes), nil
}

func formatSyncSessionID(bytes [16]byte) string {
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

type coderSyncDialer struct{}

func (coderSyncDialer) Dial(ctx context.Context, url string, header http.Header) (Conn, error) {
	conn, _, err := coderwebsocket.Dial(ctx, url, &coderwebsocket.DialOptions{HTTPHeader: header})
	if err != nil {
		return nil, err
	}
	return coderSyncConn{conn: conn}, nil
}

type coderSyncConn struct {
	conn *coderwebsocket.Conn
}

func (c coderSyncConn) Read(ctx context.Context) ([]byte, error) {
	_, data, err := c.conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c coderSyncConn) Write(ctx context.Context, data []byte) error {
	return c.conn.Write(ctx, coderwebsocket.MessageText, data)
}

func (c coderSyncConn) Close(error) error {
	return c.conn.Close(coderwebsocket.StatusNormalClosure, "")
}
