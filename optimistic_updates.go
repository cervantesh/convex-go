package convex

import "github.com/cervantesh/convex-go/baseclient"

// OptimisticUpdate is a temporary local query update applied while a sync
// mutation is pending. The callback may be replayed when new server results
// arrive before the mutation completes. Keep callbacks synchronous and avoid
// calling client methods from inside the callback.
type OptimisticUpdate = baseclient.OptimisticUpdate

// SyncMutationOption configures WebSocket-backed sync mutations.
type SyncMutationOption = baseclient.SyncMutationOption

// OptimisticQueryResult is one active query visible to an optimistic update.
type OptimisticQueryResult = baseclient.OptimisticQueryResult

// OptimisticLocalStore exposes active local query results to an optimistic
// update.
type OptimisticLocalStore = baseclient.OptimisticLocalStore

// WithOptimisticUpdate applies update to active local query results while the
// sync mutation is pending.
func WithOptimisticUpdate(update OptimisticUpdate) SyncMutationOption {
	return baseclient.WithOptimisticUpdate(update)
}
