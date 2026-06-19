package convex

import "github.com/cervantesh/convex-go/baseclient"

// SubscriberID identifies one local subscriber to a query.
type SubscriberID = baseclient.SubscriberID

// QueryResultEntry is one subscriber row in a QueryResults snapshot.
type QueryResultEntry = baseclient.QueryResultEntry

// SyncAuthError is returned when the server rejects an auth update.
type SyncAuthError = baseclient.SyncAuthError

// QueryResults is a snapshot of the latest known results for active
// subscribers.
type QueryResults = baseclient.QueryResults
