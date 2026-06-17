package baseclient

import (
	"fmt"
	"sort"
)

// OptimisticUpdate is a temporary local query update applied while a sync
// mutation is pending. The callback may be replayed when new server results
// arrive before the mutation completes. Keep callbacks synchronous and avoid
// calling client methods from inside the callback.
type OptimisticUpdate func(*OptimisticLocalStore) error

// SyncMutationOption configures WebSocket-backed sync mutations.
type SyncMutationOption func(*syncMutationOptions)

type syncMutationOptions struct {
	optimisticUpdate OptimisticUpdate
}

// WithOptimisticUpdate applies update to active local query results while the
// sync mutation is pending.
func WithOptimisticUpdate(update OptimisticUpdate) SyncMutationOption {
	return func(opts *syncMutationOptions) {
		opts.optimisticUpdate = update
	}
}

// OptimisticQueryResult is one active query visible to an optimistic update.
type OptimisticQueryResult struct {
	Path     string
	Args     Value
	Value    Value
	HasValue bool
}

// OptimisticLocalStore exposes active local query results to an optimistic
// update. Values returned from the store should be treated as immutable.
type OptimisticLocalStore struct {
	results map[QueryID]FunctionResult
	queries map[string]*localBaseQuery
}

// GetQuery returns the current value for an active query. Error query results
// and loading results are returned as ok=false, matching Convex JS semantics.
func (s *OptimisticLocalStore) GetQuery(path string, args any) (Value, bool, error) {
	_, _, token, err := canonicalQuery(path, args)
	if err != nil {
		return Value{}, false, err
	}
	query, ok := s.queries[token]
	if !ok {
		return Value{}, false, nil
	}
	result, ok := s.results[query.id]
	if !ok {
		return Value{}, false, nil
	}
	value, ok := optimisticResultValue(result)
	return value, ok, nil
}

// GetAllQueries returns all active queries with the given canonical path.
func (s *OptimisticLocalStore) GetAllQueries(path string) ([]OptimisticQueryResult, error) {
	canonicalPath, err := canonicalQueryPath(path)
	if err != nil {
		return nil, err
	}
	queries := sortedBaseQueries(s.queries)
	results := make([]OptimisticQueryResult, 0)
	for _, query := range queries {
		if query.canonicalPath != canonicalPath {
			continue
		}
		entry := OptimisticQueryResult{
			Path: query.canonicalPath,
		}
		if len(query.args) > 0 {
			entry.Args = query.args[0]
		}
		if result, hasResult := s.results[query.id]; hasResult {
			if value, ok := optimisticResultValue(result); ok {
				entry.Value = value
				entry.HasValue = true
			}
		}
		results = append(results, entry)
	}
	return results, nil
}

// SetQuery sets the visible local value for an active query. Passing nil stores
// a Convex null value; use SetQueryLoading to hide a query result while it is
// recomputed by the server.
func (s *OptimisticLocalStore) SetQuery(path string, args any, value any) error {
	_, _, token, err := canonicalQuery(path, args)
	if err != nil {
		return err
	}
	query, ok := s.queries[token]
	if !ok {
		return nil
	}
	convexValue, err := ValueFromGo(value)
	if err != nil {
		return fmt.Errorf("convex: encode optimistic query value: %w", err)
	}
	s.results[query.id] = ValueResult(convexValue)
	return nil
}

// SetQueryLoading hides the visible local result for an active query.
func (s *OptimisticLocalStore) SetQueryLoading(path string, args any) error {
	_, _, token, err := canonicalQuery(path, args)
	if err != nil {
		return err
	}
	query, ok := s.queries[token]
	if !ok {
		return nil
	}
	delete(s.results, query.id)
	return nil
}

func optimisticResultValue(result FunctionResult) (Value, bool) {
	if !result.IsValue() {
		return Value{}, false
	}
	return result.Value()
}

func syncMutationOptionsFrom(opts []SyncMutationOption) syncMutationOptions {
	var out syncMutationOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&out)
		}
	}
	return out
}

func sortedBaseQueries(queries map[string]*localBaseQuery) []*localBaseQuery {
	out := make([]*localBaseQuery, 0, len(queries))
	for _, query := range queries {
		out = append(out, query)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].id < out[j].id
	})
	return out
}
