package convex

import "context"

var (
	_ func(*Client, context.Context, string) error                                     = (*Client).SetAuthContext
	_ func(*Client, context.Context) error                                             = (*Client).ClearAuthContext
	_ func(*Client, context.Context, string, ...UserIdentityAttributes) error          = (*Client).SetAdminAuthContext
	_ func(*WebSocketClient, context.Context, string) error                            = (*WebSocketClient).SetAuthContext
	_ func(*WebSocketClient, context.Context) error                                    = (*WebSocketClient).ClearAuthContext
	_ func(*WebSocketClient, context.Context, string, ...UserIdentityAttributes) error = (*WebSocketClient).SetAdminAuthContext
)
