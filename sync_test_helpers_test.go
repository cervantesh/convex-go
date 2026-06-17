package convex

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/cervantesh/convex-go/internal/syncclient"
	"github.com/cervantesh/convex-go/internal/syncprotocol"
)

type (
	ClientMessage         = syncprotocol.ClientMessage
	ServerMessage         = syncprotocol.ServerMessage
	AuthenticateMessage   = syncprotocol.AuthenticateMessage
	ConnectMessage        = syncprotocol.ConnectMessage
	ModifyQuerySetMessage = syncprotocol.ModifyQuerySetMessage
	MutationMessage       = syncprotocol.MutationMessage
	QuerySetAdd           = syncprotocol.QuerySetAdd
	QuerySetRemove        = syncprotocol.QuerySetRemove
	TransitionMessage     = syncprotocol.TransitionMessage
	QueryUpdated          = syncprotocol.QueryUpdated
	QueryFailed           = syncprotocol.QueryFailed
	StateModification     = syncprotocol.StateModification
	StateVersion          = syncprotocol.StateVersion
	OptionalString        = syncprotocol.OptionalString
)

type fakeSyncDialer struct {
	attempts chan fakeDialAttempt
	failures chan error
	configs  chan func(*fakeSyncConn)
}

type fakeDialAttempt struct {
	url    string
	header http.Header
	conn   *fakeSyncConn
}

func newFakeSyncDialer() *fakeSyncDialer {
	return &fakeSyncDialer{
		attempts: make(chan fakeDialAttempt, 16),
		failures: make(chan error, 16),
		configs:  make(chan func(*fakeSyncConn), 16),
	}
}

func (d *fakeSyncDialer) Dial(ctx context.Context, url string, header http.Header) (syncclient.Conn, error) {
	select {
	case err := <-d.failures:
		return nil, err
	default:
	}
	conn := newFakeSyncConn()
	select {
	case configure := <-d.configs:
		configure(conn)
	default:
	}
	copied := header.Clone()
	select {
	case d.attempts <- fakeDialAttempt{url: url, header: copied, conn: conn}:
		return conn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (d *fakeSyncDialer) waitAttempt(t *testing.T) fakeDialAttempt {
	t.Helper()
	select {
	case attempt := <-d.attempts:
		return attempt
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dial attempt")
		return fakeDialAttempt{}
	}
}

type fakeSyncConn struct {
	incoming    chan []byte
	sent        chan []byte
	closed      chan error
	mu          sync.Mutex
	writeCount  int
	writeBlocks map[int]writeBlock
}

type writeBlock struct {
	blocked chan struct{}
	unblock chan struct{}
}

func newFakeSyncConn() *fakeSyncConn {
	return &fakeSyncConn{
		incoming:    make(chan []byte, 16),
		sent:        make(chan []byte, 16),
		closed:      make(chan error, 1),
		writeBlocks: make(map[int]writeBlock),
	}
}

func (c *fakeSyncConn) Read(ctx context.Context) ([]byte, error) {
	select {
	case data := <-c.incoming:
		return data, nil
	case err := <-c.closed:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *fakeSyncConn) Write(ctx context.Context, data []byte) error {
	c.mu.Lock()
	c.writeCount++
	block, shouldBlock := c.writeBlocks[c.writeCount]
	if shouldBlock {
		delete(c.writeBlocks, c.writeCount)
	}
	c.mu.Unlock()
	if shouldBlock {
		close(block.blocked)
		select {
		case <-block.unblock:
		case err := <-c.closed:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	copied := append([]byte(nil), data...)
	select {
	case c.sent <- copied:
		return nil
	case err := <-c.closed:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *fakeSyncConn) Close(error) error {
	select {
	case c.closed <- context.Canceled:
	default:
	}
	return nil
}

func (c *fakeSyncConn) waitSent(t *testing.T) []byte {
	t.Helper()
	select {
	case data := <-c.sent:
		return data
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sent message")
		return nil
	}
}

func (c *fakeSyncConn) blockNextWrite() (<-chan struct{}, chan<- struct{}) {
	_, blocked, unblock := c.blockNextWriteNumbered()
	return blocked, unblock
}

func (c *fakeSyncConn) blockNextWriteNumbered() (int, <-chan struct{}, chan<- struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	writeNumber := c.writeCount + 1
	blocked := make(chan struct{})
	unblock := make(chan struct{})
	c.writeBlocks[writeNumber] = writeBlock{blocked: blocked, unblock: unblock}
	return writeNumber, blocked, unblock
}

func (c *fakeSyncConn) removeWriteBlock(writeNumber int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.writeBlocks, writeNumber)
}

func (c *fakeSyncConn) receive(t *testing.T, msg ServerMessage) {
	t.Helper()
	data, err := syncprotocol.EncodeServerMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case c.incoming <- data:
	case <-time.After(time.Second):
		t.Fatal("timed out queueing server message")
	}
}

func decodeSentClientMessage[T ClientMessage](t *testing.T, data []byte) T {
	t.Helper()
	msg, err := syncprotocol.DecodeClientMessage(data)
	if err != nil {
		t.Fatal(err)
	}
	typed, ok := msg.(T)
	if !ok {
		t.Fatalf("unexpected client message type %T: %s", msg, data)
	}
	return typed
}

func onlyQuerySetAdd(t *testing.T, msg ModifyQuerySetMessage) QuerySetAdd {
	t.Helper()
	if len(msg.Modifications) != 1 {
		t.Fatalf("expected one query set modification, got %#v", msg.Modifications)
	}
	add, ok := msg.Modifications[0].(QuerySetAdd)
	if !ok {
		t.Fatalf("expected QuerySetAdd, got %T %#v", msg.Modifications[0], msg.Modifications[0])
	}
	return add
}

func onlyQuerySetRemove(t *testing.T, msg ModifyQuerySetMessage) QuerySetRemove {
	t.Helper()
	if len(msg.Modifications) != 1 {
		t.Fatalf("expected one query set modification, got %#v", msg.Modifications)
	}
	remove, ok := msg.Modifications[0].(QuerySetRemove)
	if !ok {
		t.Fatalf("expected QuerySetRemove, got %T %#v", msg.Modifications[0], msg.Modifications[0])
	}
	return remove
}
