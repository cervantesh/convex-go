package convex

import (
	"context"
)

// QueryReference is a typed reference to a Convex query function. Generated Go
// API packages can expose values of this type while the string-based APIs stay
// available.
type QueryReference[Args, Result any] struct {
	path string
}

// NewQueryReference creates a typed query reference for path.
func NewQueryReference[Args, Result any](path string) QueryReference[Args, Result] {
	return QueryReference[Args, Result]{path: path}
}

// Kind returns QueryKind.
func (r QueryReference[Args, Result]) Kind() FunctionKind {
	return QueryKind
}

// Path returns the Convex function path.
func (r QueryReference[Args, Result]) Path() string {
	return r.path
}

// Query runs this typed query through any client that supports QueryValue.
func (r QueryReference[Args, Result]) Query(
	ctx context.Context,
	client interface {
		QueryValue(context.Context, string, any) (Value, error)
	},
	args Args,
) (Result, error) {
	value, err := client.QueryValue(ctx, r.path, args)
	if err != nil {
		var zero Result
		return zero, err
	}
	return decodeTypedValue[Result](value)
}

// Subscribe starts a typed WebSocket query subscription.
func (r QueryReference[Args, Result]) Subscribe(ctx context.Context, client *WebSocketClient, args Args) (*TypedQuerySubscription[Result], error) {
	subscription, err := client.Subscribe(ctx, r.path, args)
	if err != nil {
		return nil, err
	}
	return &TypedQuerySubscription[Result]{raw: subscription}, nil
}

// MutationReference is a typed reference to a Convex mutation function.
type MutationReference[Args, Result any] struct {
	path string
}

// NewMutationReference creates a typed mutation reference for path.
func NewMutationReference[Args, Result any](path string) MutationReference[Args, Result] {
	return MutationReference[Args, Result]{path: path}
}

// Kind returns MutationKind.
func (r MutationReference[Args, Result]) Kind() FunctionKind {
	return MutationKind
}

// Path returns the Convex function path.
func (r MutationReference[Args, Result]) Path() string {
	return r.path
}

// Mutation runs this typed mutation through the HTTP client.
func (r MutationReference[Args, Result]) Mutation(ctx context.Context, client *HTTPClient, args Args, opts ...MutationOption) (Result, error) {
	value, err := client.MutationValue(ctx, r.path, args, opts...)
	if err != nil {
		var zero Result
		return zero, err
	}
	return decodeTypedValue[Result](value)
}

// ActionReference is a typed reference to a Convex action function.
type ActionReference[Args, Result any] struct {
	path string
}

// NewActionReference creates a typed action reference for path.
func NewActionReference[Args, Result any](path string) ActionReference[Args, Result] {
	return ActionReference[Args, Result]{path: path}
}

// Kind returns ActionKind.
func (r ActionReference[Args, Result]) Kind() FunctionKind {
	return ActionKind
}

// Path returns the Convex function path.
func (r ActionReference[Args, Result]) Path() string {
	return r.path
}

// Action runs this typed action through the HTTP client.
func (r ActionReference[Args, Result]) Action(ctx context.Context, client *HTTPClient, args Args) (Result, error) {
	value, err := client.ActionValue(ctx, r.path, args)
	if err != nil {
		var zero Result
		return zero, err
	}
	return decodeTypedValue[Result](value)
}

// TypedQuerySubscription decodes query subscription updates into Result.
type TypedQuerySubscription[Result any] struct {
	raw *QuerySubscription
}

// ID exposes the underlying subscription id.
func (s *TypedQuerySubscription[Result]) ID() SubscriberID {
	return s.raw.ID()
}

// Next blocks until the next update and decodes it into Result.
func (s *TypedQuerySubscription[Result]) Next(ctx context.Context) (Result, error) {
	result, err := s.raw.Next(ctx)
	if err != nil {
		var zero Result
		return zero, err
	}
	if err := result.Err(); err != nil {
		var zero Result
		return zero, err
	}
	value, _ := result.Value()
	return decodeTypedValue[Result](value)
}

// Unsubscribe removes the subscription from the remote query set.
func (s *TypedQuerySubscription[Result]) Unsubscribe(ctx context.Context) error {
	return s.raw.Unsubscribe(ctx)
}

// Close closes the local subscription handle.
func (s *TypedQuerySubscription[Result]) Close() error {
	return s.raw.Close()
}

func decodeTypedValue[Result any](value Value) (Result, error) {
	var out Result
	if target, ok := any(&out).(*Value); ok {
		*target = value
		return out, nil
	}
	goValue := value.GoValue()
	if target, ok := any(&out).(*any); ok {
		*target = goValue
		return out, nil
	}
	if err := decodeInto(goValue, &out); err != nil {
		var zero Result
		return zero, err
	}
	return out, nil
}
