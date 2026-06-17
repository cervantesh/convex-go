package syncclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	coderwebsocket "github.com/coder/websocket"

	"github.com/cervantesh/convex-go/baseclient"
	"github.com/cervantesh/convex-go/internal/syncprotocol"
)

const syncWebSocketPath = "/api/sync"
const defaultClientID = "go-0.1.0"

var errInactiveServer = errors.New("convex: inactive server")

// Dialer opens sync WebSocket connections. It is primarily exposed so
// tests can run against a fake transport.
type Dialer interface {
	Dial(ctx context.Context, url string, header http.Header) (Conn, error)
}

// Conn is the small connection interface Manager needs.
type Conn interface {
	Read(ctx context.Context) ([]byte, error)
	Write(ctx context.Context, data []byte) error
	Close(error) error
}

// Manager connects the internal sync state machine to Convex's sync
// WebSocket transport. It owns network reconnects for advanced protocol
// integrations. Most applications should use WebSocketClient for high-level
// subscriptions.
type Manager struct {
	url               string
	client            *baseclient.Client
	clientID          string
	dialer            Dialer
	reconnectBackoff  time.Duration
	inactivityTimeout time.Duration
	results           chan baseclient.QueryResults
	flushCh           chan chan error
	mu                sync.Mutex
	done              chan struct{}
	runMu             sync.Mutex
	runStarted        bool
	runErr            error
}

// Option configures a Manager.
type Option func(*Manager) error

// New creates a low-level sync WebSocket manager.
func New(deploymentURL string, opts ...Option) (*Manager, error) {
	syncURL, err := WebSocketURL(deploymentURL)
	if err != nil {
		return nil, err
	}
	manager := &Manager{
		url:               syncURL,
		client:            baseclient.New(),
		clientID:          defaultClientID,
		dialer:            coderSyncDialer{},
		reconnectBackoff:  100 * time.Millisecond,
		inactivityTimeout: 30 * time.Second,
		results:           make(chan baseclient.QueryResults, 16),
		flushCh:           make(chan chan error, 16),
		done:              make(chan struct{}),
	}
	for _, opt := range opts {
		if err := opt(manager); err != nil {
			return nil, err
		}
	}
	return manager, nil
}

// WithBaseClient uses an existing deterministic sync state machine.
func WithBaseClient(client *baseclient.Client) Option {
	return func(manager *Manager) error {
		if client == nil {
			return fmt.Errorf("convex: nil base client")
		}
		manager.client = client
		return nil
	}
}

// WithClientID sets the Convex-Client header value.
func WithClientID(clientID string) Option {
	return func(manager *Manager) error {
		if strings.TrimSpace(clientID) == "" {
			return fmt.Errorf("convex: client ID cannot be empty")
		}
		manager.clientID = clientID
		return nil
	}
}

// WithDialer uses a custom WebSocket dialer.
// It is primarily intended for tests and framework integrations.
func WithDialer(dialer Dialer) Option {
	return func(manager *Manager) error {
		if dialer == nil {
			return fmt.Errorf("convex: nil websocket dialer")
		}
		manager.dialer = dialer
		return nil
	}
}

// WithReconnectBackoff configures the delay between reconnect
// attempts after a transport failure.
func WithReconnectBackoff(backoff time.Duration) Option {
	return func(manager *Manager) error {
		if backoff < 0 {
			return fmt.Errorf("convex: reconnect backoff cannot be negative")
		}
		manager.reconnectBackoff = backoff
		return nil
	}
}

// WithInactivityTimeout configures how long the manager waits without
// inbound server activity before reconnecting.
func WithInactivityTimeout(timeout time.Duration) Option {
	return func(manager *Manager) error {
		if timeout <= 0 {
			return fmt.Errorf("convex: inactivity timeout must be positive")
		}
		manager.inactivityTimeout = timeout
		return nil
	}
}

// WebSocketURL maps a Convex deployment URL to the sync WebSocket endpoint.
func WebSocketURL(deploymentURL string) (string, error) {
	if strings.TrimSpace(deploymentURL) == "" {
		return "", fmt.Errorf("convex: deployment URL cannot be empty")
	}
	parsed, err := url.Parse(deploymentURL)
	if err != nil {
		return "", fmt.Errorf("convex: invalid deployment URL %q: %w", deploymentURL, err)
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("convex: unsupported websocket deployment URL scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("convex: deployment URL must include a host")
	}
	parsed.Path = syncWebSocketPath
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

// Results returns query result snapshots produced by inbound transition
// messages.
func (m *Manager) Results() <-chan baseclient.QueryResults {
	return m.results
}

// LatestResults returns the current local query result snapshot.
func (m *Manager) LatestResults() baseclient.QueryResults {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client.LatestResults()
}

// Subscribe queues a sync query subscription on the managed sync state.
func (m *Manager) Subscribe(path string, args any) (baseclient.SubscriberID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client.Subscribe(path, args)
}

// Unsubscribe removes a sync query subscription.
func (m *Manager) Unsubscribe(id baseclient.SubscriberID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client.Unsubscribe(id)
}

// Mutation queues a sync mutation request.
func (m *Manager) Mutation(path string, args any, opts ...baseclient.SyncMutationOption) (baseclient.RequestID, error) {
	m.mu.Lock()
	before := m.client.LatestResults()
	id, err := m.client.Mutation(path, args, opts...)
	after := m.client.LatestResults()
	m.mu.Unlock()
	if err == nil && !queryResultsEqual(before, after) {
		m.publishLatest(after)
	}
	return id, err
}

// Action queues a sync action request.
func (m *Manager) Action(path string, args any) (baseclient.RequestID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client.Action(path, args)
}

// SetAuth sets a static user auth token on the managed sync state.
func (m *Manager) SetAuth(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client.SetAuth(token)
}

// SetAdminAuth sets a static admin auth token on the managed sync state.
func (m *Manager) SetAdminAuth(token string, actingAs ...baseclient.SyncUserIdentityAttributes) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client.SetAdminAuth(token, actingAs...)
}

// ClearAuth clears sync auth on the managed sync state.
func (m *Manager) ClearAuth() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client.ClearAuth()
}

// SetAuthCallback stores a refreshable auth callback.
func (m *Manager) SetAuthCallback(fetcher baseclient.AuthTokenFetcher) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client.SetAuthCallback(fetcher)
}

// Flush sends any messages currently queued by the managed sync state.
func (m *Manager) Flush(ctx context.Context) error {
	ctx = nonNilContext(ctx)
	ack := make(chan error, 1)
	select {
	case m.flushCh <- ack:
	case <-m.done:
		return m.doneErr()
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-ack:
		return err
	case <-m.done:
		return m.doneErr()
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Run connects and reconnects until ctx is canceled.
func (m *Manager) Run(ctx context.Context) (err error) {
	ctx = nonNilContext(ctx)
	if err := m.startRun(); err != nil {
		return err
	}
	defer func() {
		m.finishRun(err)
	}()
	state, err := newWebSocketRunState()
	if err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := m.prepareNextConnection(ctx, &state); err != nil {
			return err
		}
		conn, err := m.dial(ctx)
		if err != nil {
			if err := m.handleDialFailure(ctx, &state, err); err != nil {
				return err
			}
			continue
		}
		if err := m.runConnectedConnection(ctx, conn, &state); err != nil {
			return err
		}
	}
}

func (m *Manager) startRun() error {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	if m.runStarted {
		return fmt.Errorf("convex: websocket manager is already running")
	}
	m.runStarted = true
	return nil
}

func (m *Manager) finishRun(err error) {
	m.runMu.Lock()
	m.runErr = err
	close(m.done)
	m.runMu.Unlock()
}

func (m *Manager) doneErr() error {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	if m.runErr != nil {
		return m.runErr
	}
	return context.Canceled
}

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
	if sleepErr := sleepContext(ctx, m.reconnectBackoff); sleepErr != nil {
		return sleepErr
	}
	state.connectionCount++
	state.lastCloseReason = err.Error()
	return nil
}

func (m *Manager) runConnectedConnection(ctx context.Context, conn Conn, state *websocketRunState) error {
	maxObserved := m.maxObservedTimestamp()
	err := m.runConnection(ctx, conn, state.sessionID, state.connectionCount, state.lastCloseReason, maxObserved)
	_ = conn.Close(err)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	var reconnect reconnectableWebSocketError
	if !errors.As(err, &reconnect) {
		return err
	}
	if reconnect.err == nil {
		return reconnect
	}
	state.connectionCount++
	state.lastCloseReason = err.Error()
	state.needsRestart = reconnect.replay
	return sleepContext(ctx, m.reconnectBackoff)
}

func (m *Manager) dial(ctx context.Context) (Conn, error) {
	header := http.Header{}
	header.Set("Convex-Client", m.clientID)
	return m.dialer.Dial(ctx, m.url, header)
}

func (m *Manager) prepareReconnect(ctx context.Context) error {
	for {
		m.mu.Lock()
		if err := m.client.RestartForReconnect(); err == nil {
			m.mu.Unlock()
			return nil
		}
		m.mu.Unlock()
		if err := sleepContext(ctx, m.reconnectBackoff); err != nil {
			return err
		}
	}
}

func (m *Manager) runConnection(ctx context.Context, conn Conn, sessionID string, connectionCount uint32, lastCloseReason string, maxObserved *syncprotocol.SyncTimestamp) error {
	clientTS := time.Now().UnixMilli()
	if err := m.writeClientMessage(ctx, conn, syncprotocol.ConnectMessage{
		SessionID:            sessionID,
		ConnectionCount:      connectionCount,
		LastCloseReason:      lastCloseReason,
		MaxObservedTimestamp: maxObserved,
		ClientTS:             &clientTS,
	}); err != nil {
		return reconnectableWebSocketError{err: err}
	}
	readCh := make(chan readResult, 1)
	readCtx, cancelRead := context.WithCancel(ctx)
	defer cancelRead()
	go readLoop(readCtx, conn, readCh)

	flushRequests := make(chan flushRequest, 16)
	writeErrCh := make(chan writeResult, 1)
	writeCtx, cancelWrite := context.WithCancel(ctx)
	defer cancelWrite()
	go m.writeLoop(writeCtx, conn, flushRequests, writeErrCh)

	select {
	case flushRequests <- flushRequest{replay: true}:
	case <-ctx.Done():
		return ctx.Err()
	}

	timer := time.NewTimer(m.inactivityTimeout)
	defer timer.Stop()
	for {
		select {
		case result := <-readCh:
			if result.err != nil {
				return reconnectableWebSocketError{err: result.err, replay: true}
			}
			resetTimer(timer, m.inactivityTimeout)
			if err := m.receiveServerMessage(ctx, result.data); err != nil {
				return err
			}
		case ack := <-m.flushCh:
			select {
			case flushRequests <- flushRequest{ack: ack, replay: true}:
			case <-ctx.Done():
				ack <- ctx.Err()
				return ctx.Err()
			}
		case result := <-writeErrCh:
			ackPendingFlushRequests(flushRequests, result.err)
			return reconnectableWebSocketError(result)
		case <-timer.C:
			return reconnectableWebSocketError{err: errInactiveServer, replay: true}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
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
