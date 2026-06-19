package convex

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/cervantesh/convex-go/internal/syncclient"
)

type WebSocketClient struct {
	manager *syncclient.Manager
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}

	closeOnce   sync.Once
	releaseOnce sync.Once
	errMu       sync.Mutex
	runErr      error

	mu            sync.Mutex
	subscriptions map[SubscriberID]chan FunctionResult
	watchers      map[*QuerySetSubscription]chan QueryResults
	latest        QueryResults
	hasLatest     bool
}

func NewWebSocketClient(ctx context.Context, deploymentURL string, opts ...WebSocketOption) (*WebSocketClient, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	options := webSocketOptions{
		managerOptions: []syncclient.Option{syncclient.WithClientID("go-" + Version)},
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt.applyWebSocketOption(&options); err != nil {
			return nil, err
		}
	}
	manager, err := syncclient.New(deploymentURL, options.managerOptions...)
	if err != nil {
		return nil, err
	}
	clientCtx, cancel := context.WithCancel(ctx)
	client := &WebSocketClient{
		manager:       manager,
		ctx:           clientCtx,
		cancel:        cancel,
		done:          make(chan struct{}),
		subscriptions: map[SubscriberID]chan FunctionResult{},
		watchers:      map[*QuerySetSubscription]chan QueryResults{},
		latest:        manager.LatestResults(),
		hasLatest:     true,
	}
	go client.run()
	go client.pumpResults()
	return client, nil
}

func (c *WebSocketClient) Subscribe(ctx context.Context, path string, args any) (*QuerySubscription, error) {
	ctx = nonNilContext(ctx)
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	if err := c.doneErr(); err != nil {
		return nil, err
	}
	id, err := c.manager.Subscribe(path, args)
	if err != nil {
		return nil, err
	}
	subscription := &QuerySubscription{
		client:  c,
		id:      id,
		updates: make(chan FunctionResult, 1),
		closed:  make(chan struct{}),
	}
	c.mu.Lock()
	c.subscriptions[id] = subscription.updates
	if c.hasLatest {
		if result, ok := c.latest.Get(id); ok {
			sendLatest(subscription.updates, result)
		}
	}
	c.mu.Unlock()
	c.publishLatest(c.manager.LatestResults())
	if err := c.flush(ctx); err != nil {
		c.mu.Lock()
		delete(c.subscriptions, id)
		c.mu.Unlock()
		_ = c.manager.Unsubscribe(id)
		c.publishLatest(c.manager.LatestResults())
		return nil, err
	}
	return subscription, nil
}

func (c *WebSocketClient) QueryValue(ctx context.Context, path string, args any) (Value, error) {
	ctx = nonNilContext(ctx)
	subscription, err := c.Subscribe(ctx, path, args)
	if err != nil {
		return Value{}, err
	}
	defer func() {
		cleanupCtx, cancel := boundedQueryCleanupContext()
		defer cancel()
		_ = subscription.Unsubscribe(cleanupCtx)
	}()
	result, err := subscription.Next(ctx)
	if err != nil {
		return Value{}, err
	}
	if err := result.Err(); err != nil {
		return Value{}, functionResultError(QueryKind, path, result)
	}
	value, _ := result.Value()
	return value, nil
}

func (c *WebSocketClient) Query(ctx context.Context, path string, args any) (any, error) {
	ctx = nonNilContext(ctx)
	value, err := c.QueryValue(ctx, path, args)
	if err != nil {
		return nil, err
	}
	return value.GoValue(), nil
}

func (c *WebSocketClient) Mutation(ctx context.Context, path string, args any, opts ...SyncMutationOption) error {
	ctx = nonNilContext(ctx)
	if err := ctxErr(ctx); err != nil {
		return err
	}
	if err := c.doneErr(); err != nil {
		return err
	}
	_, err := c.manager.Mutation(path, args, opts...)
	if err != nil {
		return err
	}
	c.publishLatest(c.manager.LatestResults())
	if err := c.flush(ctx); err != nil {
		return err
	}
	return nil
}

func (c *WebSocketClient) WatchAll(ctx context.Context) (*QuerySetSubscription, error) {
	ctx = nonNilContext(ctx)
	if err := ctxErr(ctx); err != nil {
		return nil, err
	}
	if err := c.doneErr(); err != nil {
		return nil, err
	}
	watcher := &QuerySetSubscription{
		client:  c,
		updates: make(chan QueryResults, 1),
		closed:  make(chan struct{}),
	}
	c.mu.Lock()
	c.watchers[watcher] = watcher.updates
	if c.hasLatest {
		sendLatest(watcher.updates, c.latest)
	}
	c.mu.Unlock()
	return watcher, nil
}

func (c *WebSocketClient) Close() error {
	c.closeOnce.Do(c.cancel)
	<-c.done
	err := c.runError()
	if err == nil || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (c *WebSocketClient) SetAuth(token string) error {
	ctx, cancel := boundedRealtimeControlContext()
	defer cancel()
	return c.SetAuthContext(ctx, token)
}

func (c *WebSocketClient) SetAuthContext(ctx context.Context, token string) error {
	return c.setAuth(ctx, token, true)
}

// SetAuthCallback installs a user JWT callback for realtime auth refresh.
// Passing nil clears any current auth.
func (c *WebSocketClient) SetAuthCallback(fetcher UserTokenFetcher) error {
	ctx, cancel := boundedRealtimeControlContext()
	defer cancel()
	return c.SetAuthCallbackContext(ctx, fetcher)
}

// SetAuthCallbackContext installs a user JWT callback and uses ctx for the
// resulting auth flush.
func (c *WebSocketClient) SetAuthCallbackContext(ctx context.Context, fetcher UserTokenFetcher) error {
	return c.setAuthCallback(ctx, fetcher, true)
}

func (c *WebSocketClient) setAuthCallback(ctx context.Context, fetcher UserTokenFetcher, flush bool) error {
	if err := c.doneErr(); err != nil {
		return err
	}
	if err := c.manager.SetAuthCallback(adaptUserTokenFetcher(fetcher)); err != nil {
		return err
	}
	if flush {
		return c.flush(ctx)
	}
	return nil
}

func (c *WebSocketClient) setAuth(ctx context.Context, token string, flush bool) error {
	if err := c.doneErr(); err != nil {
		return err
	}
	if err := c.manager.SetAuth(token); err != nil {
		return err
	}
	if flush {
		return c.flush(ctx)
	}
	return nil
}

func (c *WebSocketClient) SetAdminAuth(token string, actingAs ...UserIdentityAttributes) error {
	ctx, cancel := boundedRealtimeControlContext()
	defer cancel()
	return c.SetAdminAuthContext(ctx, token, actingAs...)
}

func (c *WebSocketClient) SetAdminAuthContext(ctx context.Context, token string, actingAs ...UserIdentityAttributes) error {
	return c.setAdminAuth(ctx, token, true, actingAs...)
}

func (c *WebSocketClient) setAdminAuth(ctx context.Context, token string, flush bool, actingAs ...UserIdentityAttributes) error {
	if err := c.doneErr(); err != nil {
		return err
	}
	if err := c.manager.SetAdminAuth(token, syncIdentityAttributes(actingAs)...); err != nil {
		return err
	}
	if flush {
		return c.flush(ctx)
	}
	return nil
}

func (c *WebSocketClient) ClearAuth() error {
	ctx, cancel := boundedRealtimeControlContext()
	defer cancel()
	return c.ClearAuthContext(ctx)
}

func (c *WebSocketClient) ClearAuthContext(ctx context.Context) error {
	return c.clearAuth(ctx, true)
}

func (c *WebSocketClient) clearAuth(ctx context.Context, flush bool) error {
	if err := c.doneErr(); err != nil {
		return err
	}
	if err := c.manager.ClearAuth(); err != nil {
		return err
	}
	if flush {
		return c.flush(ctx)
	}
	return nil
}

func (c *WebSocketClient) run() {
	err := c.manager.Run(c.ctx)
	c.errMu.Lock()
	c.runErr = err
	c.errMu.Unlock()
	c.cancel()
	c.releaseRuntimeState()
	close(c.done)
}

func (c *WebSocketClient) pumpResults() {
	for {
		select {
		case results := <-c.manager.Results():
			c.publishLatest(results)
		case <-c.ctx.Done():
			return
		case <-c.done:
			return
		}
	}
}

func (c *WebSocketClient) publishLatest(results QueryResults) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.latest = results
	c.hasLatest = true
	for id, updates := range c.subscriptions {
		if result, ok := results.Get(id); ok {
			sendLatest(updates, result)
		}
	}
	for _, updates := range c.watchers {
		sendLatest(updates, results)
	}
}

func (c *WebSocketClient) unsubscribe(ctx context.Context, subscription *QuerySubscription) error {
	if err := c.manager.Unsubscribe(subscription.id); err != nil {
		return err
	}
	c.publishLatest(c.manager.LatestResults())
	return nil
}

func (c *WebSocketClient) finishUnsubscribe(subscription *QuerySubscription) {
	c.mu.Lock()
	delete(c.subscriptions, subscription.id)
	c.mu.Unlock()
}

func (c *WebSocketClient) unwatch(watcher *QuerySetSubscription) error {
	c.mu.Lock()
	_, active := c.watchers[watcher]
	if active {
		delete(c.watchers, watcher)
	}
	c.mu.Unlock()
	close(watcher.closed)
	return nil
}

func (c *WebSocketClient) flush(ctx context.Context) error {
	ctx = nonNilContext(ctx)
	flushCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(c.ctx, cancel)
	defer func() {
		stop()
		cancel()
	}()
	if err := c.manager.Flush(flushCtx); err != nil {
		if c.ctx.Err() != nil {
			return c.doneErr()
		}
		return err
	}
	return nil
}

func (c *WebSocketClient) doneErr() error {
	select {
	case <-c.done:
		if err := c.runError(); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return context.Canceled
	default:
		if err := c.ctx.Err(); err != nil {
			return err
		}
		return nil
	}
}

func (c *WebSocketClient) runError() error {
	c.errMu.Lock()
	defer c.errMu.Unlock()
	return c.runErr
}

func (c *WebSocketClient) releaseRuntimeState() {
	c.releaseOnce.Do(func() {
		c.mu.Lock()
		c.subscriptions = nil
		c.watchers = nil
		c.latest = QueryResults{}
		c.hasLatest = false
		c.mu.Unlock()
	})
}

func boundedRealtimeControlContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), boundedTimeout(defaultRealtimeControlTimeout))
}

func boundedQueryCleanupContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), boundedTimeout(defaultQueryCleanupTimeout))
}

func boundedTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return time.Millisecond
	}
	return timeout
}
