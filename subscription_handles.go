package convex

import (
	"context"
	"sync"
)

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

func (s *QuerySubscription) ID() SubscriberID {
	return s.id
}

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

func (s *QuerySubscription) Close() error {
	return s.Unsubscribe(context.Background())
}

type QuerySetSubscription struct {
	client *WebSocketClient

	updates chan QueryResults
	closed  chan struct{}

	closeOnce sync.Once
	closeErr  error
}

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
