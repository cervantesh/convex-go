package baseclient

import (
	"fmt"
	"sort"
)

func (c *Client) ReceiveMessage(msg ServerMessage) (*QueryResults, error) {
	switch m := msg.(type) {
	case TransitionMessage:
		return c.receiveTransition(m)
	case MutationResponseMessage:
		return c.receiveMutationResponse(m)
	case ActionResponseMessage:
		if err := c.updateRequestFromActionResponse(m); err != nil {
			return nil, err
		}
		return nil, nil
	case AuthErrorMessage:
		return nil, syncAuthErrorFromMessage(m)
	case FatalErrorMessage:
		return nil, fmt.Errorf("convex: fatal error: %s", m.Error)
	case PingMessage:
		return nil, nil
	case TransitionChunkMessage:
		return nil, fmt.Errorf("convex: unexpected transition chunk")
	default:
		return nil, fmt.Errorf("convex: unsupported server message %T", msg)
	}
}

func (c *Client) receiveTransition(message TransitionMessage) (*QueryResults, error) {
	if message.StartVersion != c.remoteVersion {
		return nil, fmt.Errorf("convex: transition start version mismatch: got %#v want %#v", message.StartVersion, c.remoteVersion)
	}
	for _, modification := range message.Modifications {
		if err := c.applyStateModification(modification); err != nil {
			return nil, err
		}
	}
	c.remoteVersion = message.EndVersion
	c.observeTimestamp(message.EndVersion.TS)
	c.completeMutationsThrough(message.EndVersion.TS)
	if err := c.recomputeOptimisticResults(); err != nil {
		return nil, err
	}
	results := c.queryResultsSnapshot()
	return &results, nil
}

func (c *Client) applyStateModification(modification StateModification) error {
	switch mod := modification.(type) {
	case QueryUpdated:
		if c.isActiveQuery(mod.QueryID) {
			c.serverResults[mod.QueryID] = ValueResult(mod.Value)
		}
	case QueryFailed:
		if c.isActiveQuery(mod.QueryID) {
			c.serverResults[mod.QueryID] = queryFailedResult(mod)
		}
	case QueryRemoved:
		delete(c.serverResults, mod.QueryID)
		delete(c.results, mod.QueryID)
	default:
		return fmt.Errorf("convex: unsupported state modification %T", modification)
	}
	return nil
}

func queryFailedResult(mod QueryFailed) FunctionResult {
	if mod.ErrorData.Present {
		return ConvexErrorResult(ConvexError{
			Message: mod.ErrorMessage,
			Data:    mod.ErrorData.Value,
		})
	}
	return ErrorMessageResult(mod.ErrorMessage)
}

func (c *Client) receiveMutationResponse(message MutationResponseMessage) (*QueryResults, error) {
	hadOptimistic := false
	if request, ok := c.requests[message.RequestID]; ok {
		hadOptimistic = request.optimisticUpdate != nil
	}
	if err := c.updateRequestFromMutationResponse(message); err != nil {
		return nil, err
	}
	if message.TS != nil {
		c.observeTimestamp(*message.TS)
	}
	if !message.Success && hadOptimistic {
		results := c.queryResultsSnapshot()
		return &results, nil
	}
	return nil, nil
}

func syncAuthErrorFromMessage(message AuthErrorMessage) *SyncAuthError {
	return &SyncAuthError{
		Message:             message.Error,
		BaseVersion:         message.BaseVersion,
		AuthUpdateAttempted: message.AuthUpdateAttempted,
	}
}

func (c *Client) queryResultsSnapshot() QueryResults {
	subscribers := make(map[SubscriberID]struct{}, len(c.subscribers))
	for id := range c.subscribers {
		subscribers[id] = struct{}{}
	}
	results := make(map[QueryID]FunctionResult, len(c.results))
	for id, result := range c.results {
		results[id] = result
	}
	return QueryResults{subscribers: subscribers, results: results}
}

func (c *Client) recomputeOptimisticResults() error {
	results, err := c.optimisticResultsWith(nil)
	if err != nil {
		return err
	}
	c.results = results
	return nil
}

func (c *Client) optimisticResultsWith(candidate *trackedSyncRequest) (map[QueryID]FunctionResult, error) {
	results := copyFunctionResults(c.serverResults)
	requests := c.optimisticRequests(candidate)
	if len(requests) == 0 {
		return results, nil
	}
	store := &OptimisticLocalStore{
		results: results,
		queries: c.queries,
	}
	for _, request := range requests {
		if err := request.optimisticUpdate(store); err != nil {
			return nil, fmt.Errorf("convex: optimistic update for mutation request %d: %w", request.id, err)
		}
	}
	return results, nil
}

func (c *Client) optimisticRequests(candidate *trackedSyncRequest) []*trackedSyncRequest {
	requests := make([]*trackedSyncRequest, 0, len(c.requests)+1)
	for _, request := range c.requests {
		if request.kind == syncRequestMutation && request.optimisticUpdate != nil {
			requests = append(requests, request)
		}
	}
	if candidate != nil && candidate.kind == syncRequestMutation && candidate.optimisticUpdate != nil {
		requests = append(requests, candidate)
	}
	sort.Slice(requests, func(i, j int) bool { return requests[i].id < requests[j].id })
	return requests
}

func copyFunctionResults(results map[QueryID]FunctionResult) map[QueryID]FunctionResult {
	copied := make(map[QueryID]FunctionResult, len(results))
	for id, result := range results {
		copied[id] = result
	}
	return copied
}

func (c *Client) isActiveQuery(id QueryID) bool {
	_, ok := c.queryIDToToken[id]
	return ok
}

func (c *Client) observeTimestamp(ts SyncTimestamp) {
	if c.maxObservedTimestamp == nil || ts > *c.maxObservedTimestamp {
		c.maxObservedTimestamp = &ts
	}
}

func (c *Client) queueAuth(token AuthToken) error {
	if token.TokenType == "" {
		token.TokenType = AuthTokenNone
	}
	if token.TokenType == AuthTokenAdmin && token.ActingAs != nil && token.ActingAs.TokenIdentifier == "" && token.ActingAs.Issuer != "" && token.ActingAs.Subject != "" {
		identity := *token.ActingAs
		identity.TokenIdentifier = identity.Issuer + "|" + identity.Subject
		token.ActingAs = &identity
	}
	baseVersion := c.identityVersion
	c.identityVersion++
	c.outgoing = append(c.outgoing, AuthenticateMessage{
		BaseVersion: baseVersion,
		TokenType:   token.TokenType,
		Value:       token.Value,
		ActingAs:    token.ActingAs,
	})
	return nil
}

func (c *Client) updateRequestFromMutationResponse(message MutationResponseMessage) error {
	request, ok := c.requests[message.RequestID]
	if !ok {
		if _, completed := c.completedMutations[message.RequestID]; completed {
			return nil
		}
		return fmt.Errorf("convex: invalid mutation request id %d", message.RequestID)
	}
	if request.kind != syncRequestMutation {
		return fmt.Errorf("convex: mismatched mutation response for request id %d", message.RequestID)
	}
	result := functionResultFromResponse(message.Success, message.Value, message.ErrorMessage, message.ErrorData)
	request.result = result
	request.hasResult = true
	request.ts = message.TS
	if !message.Success {
		hadOptimistic := c.completeMutationRequest(message.RequestID)
		if hadOptimistic {
			if err := c.recomputeOptimisticResults(); err != nil {
				return err
			}
		}
		return nil
	}
	if message.TS == nil {
		return fmt.Errorf("convex: successful mutation response missing timestamp")
	}
	return nil
}

func (c *Client) updateRequestFromActionResponse(message ActionResponseMessage) error {
	request, ok := c.requests[message.RequestID]
	if !ok {
		return fmt.Errorf("convex: invalid action request id %d", message.RequestID)
	}
	if request.kind != syncRequestAction {
		return fmt.Errorf("convex: mismatched action response for request id %d", message.RequestID)
	}
	c.rememberActionResult(message.RequestID, functionResultFromResponse(message.Success, message.Value, message.ErrorMessage, message.ErrorData))
	delete(c.requests, message.RequestID)
	return nil
}

func (c *Client) completeMutationsThrough(ts SyncTimestamp) {
	ids := make([]RequestID, 0)
	for id, request := range c.requests {
		if request.kind != syncRequestMutation || !request.hasResult || request.ts == nil {
			continue
		}
		if *request.ts <= ts {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		c.completeMutationRequest(id)
	}
}

func (c *Client) completeMutationRequest(id RequestID) bool {
	request, ok := c.requests[id]
	if !ok || !request.hasResult {
		return false
	}
	c.rememberMutationResult(id, request.result)
	delete(c.requests, id)
	return request.optimisticUpdate != nil
}

func (c *Client) rememberMutationResult(id RequestID, result FunctionResult) {
	c.mutationResults[id] = result
	c.completedMutations[id] = struct{}{}
	c.completedMutationOrder = append(c.completedMutationOrder, id)
	limit := normalizedCompletedRequestRetentionLimit()
	for len(c.completedMutationOrder) > limit {
		evict := c.completedMutationOrder[0]
		c.completedMutationOrder = dropOldestRequestID(c.completedMutationOrder)
		delete(c.mutationResults, evict)
		delete(c.completedMutations, evict)
	}
}

func (c *Client) rememberActionResult(id RequestID, result FunctionResult) {
	c.actionResults[id] = result
	c.actionResultOrder = append(c.actionResultOrder, id)
	limit := normalizedCompletedRequestRetentionLimit()
	for len(c.actionResultOrder) > limit {
		evict := c.actionResultOrder[0]
		c.actionResultOrder = dropOldestRequestID(c.actionResultOrder)
		delete(c.actionResults, evict)
	}
}

func normalizedCompletedRequestRetentionLimit() int {
	if completedRequestRetentionLimit < 1 {
		return 1
	}
	return completedRequestRetentionLimit
}

func dropOldestRequestID(ids []RequestID) []RequestID {
	copy(ids, ids[1:])
	return ids[:len(ids)-1]
}

func (c *Client) replayQueryAdds() []QuerySetModification {
	queries := make([]*localBaseQuery, 0, len(c.queries))
	for _, query := range c.queries {
		queries = append(queries, query)
	}
	sort.Slice(queries, func(i, j int) bool {
		return queries[i].id < queries[j].id
	})
	adds := make([]QuerySetModification, len(queries))
	for i, query := range queries {
		adds[i] = QuerySetAdd{
			QueryID: query.id,
			UDFPath: query.canonicalPath,
			Args:    append([]Value(nil), query.args...),
		}
	}
	return adds
}

func (c *Client) cancelPendingActionsForReconnect() {
	ids := make([]RequestID, 0)
	for id, request := range c.requests {
		if request.kind == syncRequestAction {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		c.rememberActionResult(id, ErrorMessageResult(actionReconnectErrorMessage))
		delete(c.requests, id)
	}
}

func functionResultFromResponse(success bool, value Value, message string, data OptionalValue) FunctionResult {
	if success {
		return ValueResult(value)
	}
	if data.Present {
		return ConvexErrorResult(ConvexError{Message: message, Data: data.Value})
	}
	return ErrorMessageResult(message)
}
