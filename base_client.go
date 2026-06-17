package convex

import "github.com/cervantesh/convex-go/baseclient"

// SubscriberID identifies one local subscriber to a query.
type SubscriberID = baseclient.SubscriberID

// QueryResultEntry is one subscriber row in a QueryResults snapshot.
type QueryResultEntry = baseclient.QueryResultEntry

// Deprecated: advanced sync auth token modeling lives in package baseclient.
// Most applications should use WithAuth, WithAdminAuth, SetAuth, or
// SetAdminAuth instead of handling Authenticate-message tokens directly.
type AuthToken = baseclient.AuthToken

// Deprecated: refreshable auth callbacks belong to package baseclient. The
// root package keeps this alias only as an advanced pre-v1 compatibility shim.
type AuthTokenFetcher = baseclient.AuthTokenFetcher

// SyncAuthError is returned when the server rejects an auth update.
type SyncAuthError = baseclient.SyncAuthError

// QueryResults is a snapshot of the latest known results for active
// subscribers.
type QueryResults = baseclient.QueryResults

// Deprecated: advanced sync identity modeling belongs to package baseclient.
// Root realtime APIs use UserIdentityAttributes instead.
type SyncUserIdentityAttributes = baseclient.SyncUserIdentityAttributes

// Deprecated: use package baseclient for direct sync auth token construction.
func UserAuthToken(token string) AuthToken {
	return baseclient.UserAuthToken(token)
}

// Deprecated: use package baseclient for direct sync auth token construction.
func AdminAuthToken(token string, actingAs ...SyncUserIdentityAttributes) AuthToken {
	return baseclient.AdminAuthToken(token, actingAs...)
}

// Deprecated: use package baseclient for direct sync auth token construction.
func NoAuthToken() AuthToken {
	return baseclient.NoAuthToken()
}
