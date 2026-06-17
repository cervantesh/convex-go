package convex

import "context"

// DeploymentURL returns the normalized deployment URL.
func (c *HTTPClient) DeploymentURL() string {
	return c.address
}

// DeploymentURL returns the normalized deployment URL.
func (c *Client) DeploymentURL() string {
	return c.httpClient.DeploymentURL()
}

// Query executes a Convex query function over HTTP.
func (c *Client) Query(ctx context.Context, path string, args any) (any, error) {
	return c.httpClient.Query(ctx, path, args)
}

// Mutation executes a Convex mutation function over HTTP.
func (c *Client) Mutation(ctx context.Context, path string, args any, opts ...MutationOption) (any, error) {
	return c.httpClient.Mutation(ctx, path, args, opts...)
}

// Action executes a Convex action function over HTTP.
func (c *Client) Action(ctx context.Context, path string, args any) (any, error) {
	return c.httpClient.Action(ctx, path, args)
}

// QueryValue executes a query and returns a typed Convex Value.
func (c *Client) QueryValue(ctx context.Context, path string, args any) (Value, error) {
	return c.httpClient.QueryValue(ctx, path, args)
}

// MutationValue executes a mutation and returns a typed Convex Value.
func (c *Client) MutationValue(ctx context.Context, path string, args any, opts ...MutationOption) (Value, error) {
	return c.httpClient.MutationValue(ctx, path, args, opts...)
}

// ActionValue executes an action and returns a typed Convex Value.
func (c *Client) ActionValue(ctx context.Context, path string, args any) (Value, error) {
	return c.httpClient.ActionValue(ctx, path, args)
}

// QueryInto executes a query and decodes the result into out using encoding/json.
func (c *Client) QueryInto(ctx context.Context, path string, args any, out any) error {
	return c.httpClient.QueryInto(ctx, path, args, out)
}

// MutationInto executes a mutation and decodes the result into out using encoding/json.
func (c *Client) MutationInto(ctx context.Context, path string, args any, out any, opts ...MutationOption) error {
	return c.httpClient.MutationInto(ctx, path, args, out, opts...)
}

// ActionInto executes an action and decodes the result into out using encoding/json.
func (c *Client) ActionInto(ctx context.Context, path string, args any, out any) error {
	return c.httpClient.ActionInto(ctx, path, args, out)
}

// Function executes a Convex function of unknown type through /api/function.
func (c *Client) Function(ctx context.Context, path string, args any, componentPath string) (any, error) {
	return c.httpClient.Function(ctx, path, args, componentPath)
}

// FunctionValue executes a Convex function of unknown type through /api/function.
func (c *Client) FunctionValue(ctx context.Context, path string, args any, componentPath string) (Value, error) {
	return c.httpClient.FunctionValue(ctx, path, args, componentPath)
}

// GetTimestamp returns a timestamp token for consistent reads.
func (c *Client) GetTimestamp(ctx context.Context) (string, error) {
	return c.httpClient.GetTimestamp(ctx)
}

// QueryAtTimestamp executes a query at a timestamp returned by GetTimestamp.
func (c *Client) QueryAtTimestamp(ctx context.Context, path string, args any, timestamp string) (Value, error) {
	return c.httpClient.QueryAtTimestamp(ctx, path, args, timestamp)
}

// ConsistentQuery obtains a timestamp and executes the query at that timestamp.
func (c *Client) ConsistentQuery(ctx context.Context, path string, args any) (Value, error) {
	return c.httpClient.ConsistentQuery(ctx, path, args)
}

// Subscribe starts a realtime query subscription, initializing WebSocket
// resources lazily on first use.
func (c *Client) Subscribe(ctx context.Context, path string, args any) (*QuerySubscription, error) {
	realtime, err := c.realtime(ctx)
	if err != nil {
		return nil, err
	}
	return realtime.Subscribe(ctx, path, args)
}

// WatchAll subscribes to coalesced snapshots of all active realtime query
// results, initializing WebSocket resources lazily on first use.
func (c *Client) WatchAll(ctx context.Context) (*QuerySetSubscription, error) {
	realtime, err := c.realtime(ctx)
	if err != nil {
		return nil, err
	}
	return realtime.WatchAll(ctx)
}

// Close closes realtime resources if they have been initialized. It is safe to
// call multiple times.
func (c *Client) Close() error {
	c.mu.Lock()
	realtime := c.webSocketClient
	c.webSocketClient = nil
	c.mu.Unlock()
	if realtime == nil {
		return nil
	}
	err := realtime.Close()
	c.clearConnectionStateObserverAttachments()
	return err
}

func (c *Client) realtime(ctx context.Context) (*WebSocketClient, error) {
	if c.webSocketClient != nil {
		c.mu.Lock()
		realtime := c.webSocketClient
		c.mu.Unlock()
		return realtime, nil
	}
	c.mu.Lock()
	deploymentURL := c.deploymentURL
	webSocketOptions := append([]WebSocketOption(nil), c.webSocketOptions...)
	authCallback := c.authCallback
	adminAuthToken := c.adminAuthToken
	adminActingAs := cloneUserIdentityAttributes(c.adminActingAs)
	authToken := c.authToken
	c.mu.Unlock()
	realtime, err := NewWebSocketClient(ctx, deploymentURL, webSocketOptions...)
	if err != nil {
		return nil, err
	}
	if authCallback != nil {
		if err := realtime.setAuthCallback(ctx, authCallback, false); err != nil {
			_ = realtime.Close()
			return nil, err
		}
	} else if adminAuthToken != "" {
		if err := realtime.setAdminAuth(ctx, adminAuthToken, false, adminActingAs...); err != nil {
			_ = realtime.Close()
			return nil, err
		}
	} else if authToken != "" {
		if err := realtime.setAuth(ctx, authToken, false); err != nil {
			_ = realtime.Close()
			return nil, err
		}
	}
	c.mu.Lock()
	if c.webSocketClient != nil {
		existing := c.webSocketClient
		c.mu.Unlock()
		_ = realtime.Close()
		return existing, nil
	}
	c.webSocketClient = realtime
	c.mu.Unlock()
	c.attachConnectionStateObservers(realtime)
	return realtime, nil
}

// SetAuth sets a bearer auth token for subsequent calls.
func (c *HTTPClient) SetAuth(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.auth = token
	if token != "" {
		c.adminAuth = ""
	}
	c.authCallback = nil
}

// SetAuth sets a bearer auth token for subsequent calls. When realtime is
// already running it uses a bounded background context for the sync flush; use
// SetAuthContext when you need to observe or control that flush.
func (c *Client) SetAuth(token string) {
	ctx, cancel := boundedRealtimeControlContext()
	defer cancel()
	_ = c.SetAuthContext(ctx, token)
}

// SetAuthContext sets a bearer auth token and uses ctx for any realtime flush.
// The local client state is updated before any realtime write, so an error from
// this method means the running transport did not flush within ctx.
func (c *Client) SetAuthContext(ctx context.Context, token string) error {
	c.httpClient.SetAuth(token)
	c.mu.Lock()
	c.authToken = token
	c.adminAuthToken = ""
	c.adminActingAs = nil
	c.authCallback = nil
	realtime := c.webSocketClient
	c.mu.Unlock()
	if realtime != nil {
		return realtime.SetAuthContext(ctx, token)
	}
	return nil
}

// ClearAuth removes bearer auth.
func (c *HTTPClient) ClearAuth() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.auth = ""
	c.adminAuth = ""
	c.authCallback = nil
}

// ClearAuth removes auth from subsequent calls. When realtime is already
// running it uses a bounded background context for the sync flush; use
// ClearAuthContext when you need to observe or control that flush.
func (c *Client) ClearAuth() {
	ctx, cancel := boundedRealtimeControlContext()
	defer cancel()
	_ = c.ClearAuthContext(ctx)
}

// ClearAuthContext removes auth and uses ctx for any realtime flush. The local
// client state is updated before any realtime write, so an error from this
// method means the running transport did not flush within ctx.
func (c *Client) ClearAuthContext(ctx context.Context) error {
	c.httpClient.ClearAuth()
	c.mu.Lock()
	c.authToken = ""
	c.adminAuthToken = ""
	c.adminActingAs = nil
	c.authCallback = nil
	realtime := c.webSocketClient
	c.mu.Unlock()
	if realtime != nil {
		return realtime.ClearAuthContext(ctx)
	}
	return nil
}

// SetAdminAuth sets a Convex admin auth token for subsequent calls. When
// actingAs is provided, the admin key acts as that identity.
func (c *HTTPClient) SetAdminAuth(token string, actingAs ...UserIdentityAttributes) error {
	adminAuth, err := encodeAdminAuth(token, actingAs...)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.adminAuth = adminAuth
	if adminAuth != "" {
		c.auth = ""
	}
	c.authCallback = nil
	return nil
}

// SetAdminAuth sets a Convex admin auth token for subsequent calls. When
// actingAs is provided, the admin key acts as that identity. When realtime is
// already running it uses a bounded background context for the sync flush; use
// SetAdminAuthContext when you need to observe or control that flush.
func (c *Client) SetAdminAuth(token string, actingAs ...UserIdentityAttributes) error {
	ctx, cancel := boundedRealtimeControlContext()
	defer cancel()
	return c.SetAdminAuthContext(ctx, token, actingAs...)
}

// SetAdminAuthContext sets admin auth and uses ctx for any realtime flush. The
// local client state is updated before any realtime write, so an error from
// this method means the running transport did not flush within ctx.
func (c *Client) SetAdminAuthContext(ctx context.Context, token string, actingAs ...UserIdentityAttributes) error {
	if err := c.httpClient.SetAdminAuth(token, actingAs...); err != nil {
		return err
	}
	c.mu.Lock()
	c.adminAuthToken = token
	c.adminActingAs = cloneUserIdentityAttributes(actingAs)
	c.authToken = ""
	c.authCallback = nil
	realtime := c.webSocketClient
	c.mu.Unlock()
	if realtime != nil {
		if err := realtime.SetAdminAuthContext(ctx, token, actingAs...); err != nil {
			return err
		}
	}
	return nil
}

// SetAuthCallback installs a user JWT callback for HTTP requests and realtime
// auth refresh. Passing nil clears any current user/admin auth.
func (c *Client) SetAuthCallback(fetcher UserTokenFetcher) error {
	ctx, cancel := boundedRealtimeControlContext()
	defer cancel()
	return c.SetAuthCallbackContext(ctx, fetcher)
}

// SetAuthCallbackContext installs a user JWT callback and uses ctx for any
// realtime auth flush.
func (c *Client) SetAuthCallbackContext(ctx context.Context, fetcher UserTokenFetcher) error {
	c.mu.Lock()
	realtime := c.webSocketClient
	c.mu.Unlock()
	if realtime != nil {
		if err := realtime.SetAuthCallbackContext(ctx, fetcher); err != nil {
			return err
		}
	} else if fetcher != nil {
		if _, err := fetcher(false); err != nil {
			return err
		}
	}
	c.httpClient.setAuthCallback(fetcher)
	c.mu.Lock()
	c.authCallback = fetcher
	c.authToken = ""
	c.adminAuthToken = ""
	c.adminActingAs = nil
	c.mu.Unlock()
	return nil
}
