package convex

import (
	"context"
	"sync"
)

// QuerySubscription streams results for one realtime Convex query.
type QuerySubscription struct {
	client *WebSocketClient
	id     SubscriberID

	updates chan FunctionResult
	closed  chan struct{}

	mu                sync.Mutex
	closedOK          bool
	unsubscribeQueued bool
	closeErr          error
}

// ID returns the opaque subscription identifier assigned by the sync layer.
func (s *QuerySubscription) ID() SubscriberID {
	return s.id
}

// Next waits for the next query result for this subscription.
func (s *QuerySubscription) Next(ctx context.Context) (FunctionResult, error) {
	ctx = nonNilContext(ctx)
	if err := ctxErr(ctx); err != nil {
		return FunctionResult{}, err
	}
	select {
	case <-s.closed:
		return FunctionResult{}, ErrSubscriptionClosed
	default:
	}
	select {
	case <-s.client.done:
		return FunctionResult{}, s.client.doneErr()
	default:
	}
	select {
	case result := <-s.updates:
		return result, nil
	case <-s.closed:
		return FunctionResult{}, ErrSubscriptionClosed
	case <-s.client.done:
		return FunctionResult{}, s.client.doneErr()
	case <-ctx.Done():
		return FunctionResult{}, ctx.Err()
	}
}

// Unsubscribe removes the underlying realtime subscription.
func (s *QuerySubscription) Unsubscribe(ctx context.Context) error {
	ctx = nonNilContext(ctx)
	if err := ctxErr(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closedOK {
		return s.closeErr
	}
	if !s.unsubscribeQueued {
		if err := s.client.unsubscribe(ctx, s); err != nil {
			return err
		}
		s.unsubscribeQueued = true
	}
	if err := s.client.flush(ctx); err != nil {
		if s.client.ctx.Err() != nil {
			s.client.finishUnsubscribe(s)
			close(s.closed)
			s.closedOK = true
			s.closeErr = err
		}
		return err
	}
	s.client.finishUnsubscribe(s)
	close(s.closed)
	s.closedOK = true
	s.closeErr = nil
	return nil
}

// Close unsubscribes with a background context.
func (s *QuerySubscription) Close() error {
	return s.Unsubscribe(context.Background())
}

// QuerySetSubscription streams coalesced snapshots of all active realtime queries.
type QuerySetSubscription struct {
	client *WebSocketClient

	updates chan QueryResults
	closed  chan struct{}

	closeOnce sync.Once
	closeErr  error
}

// Next waits for the next coalesced query-set snapshot.
func (s *QuerySetSubscription) Next(ctx context.Context) (QueryResults, error) {
	ctx = nonNilContext(ctx)
	if err := ctxErr(ctx); err != nil {
		return QueryResults{}, err
	}
	select {
	case <-s.closed:
		return QueryResults{}, ErrSubscriptionClosed
	default:
	}
	select {
	case <-s.client.done:
		return QueryResults{}, s.client.doneErr()
	default:
	}
	select {
	case results := <-s.updates:
		return results, nil
	case <-s.closed:
		return QueryResults{}, ErrSubscriptionClosed
	case <-s.client.done:
		return QueryResults{}, s.client.doneErr()
	case <-ctx.Done():
		return QueryResults{}, ctx.Err()
	}
}

// Close stops the coalesced snapshot stream.
func (s *QuerySetSubscription) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = s.client.unwatch(s)
	})
	return s.closeErr
}

func sendLatest[T any](ch chan T, value T) {
	select {
	case ch <- value:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- value:
	default:
	}
}

func functionResultError(kind FunctionKind, path string, result FunctionResult) error {
	if convexErr, ok := result.ConvexError(); ok {
		return &FunctionError{
			Kind:      kind,
			Path:      path,
			Message:   convexErr.Message,
			Data:      convexErr.Data.GoValue(),
			DataValue: convexErr.Data,
			Convex:    convexErr,
			HasData:   true,
		}
	}
	if message, ok := result.ErrorMessage(); ok {
		return &FunctionError{Kind: kind, Path: path, Message: message}
	}
	return result.Err()
}

func ctxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
