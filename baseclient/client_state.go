package baseclient

import (
	"fmt"
	"sort"
)

type localBaseQuery struct {
	id                QueryID
	canonicalPath     string
	args              []Value
	numSubscribers    int
	nextSubscriberIdx uint64
}

type syncRequestKind uint8

const (
	syncRequestMutation syncRequestKind = iota + 1
	syncRequestAction
)

const actionReconnectErrorMessage = "Connection lost while action was in flight"

var completedRequestRetentionLimit = 1024

type trackedSyncRequest struct {
	id               RequestID
	kind             syncRequestKind
	message          ClientMessage
	optimisticUpdate OptimisticUpdate
	result           FunctionResult
	hasResult        bool
	ts               *SyncTimestamp
}

type Client struct {
	nextQueryID            QueryID
	querySetVersion        QuerySetVersion
	queries                map[string]*localBaseQuery
	queryIDToToken         map[QueryID]string
	subscribers            map[SubscriberID]struct{}
	serverResults          map[QueryID]FunctionResult
	results                map[QueryID]FunctionResult
	remoteVersion          StateVersion
	identityVersion        IdentityVersion
	authFetcher            AuthTokenFetcher
	outgoing               []ClientMessage
	maxObservedTimestamp   *SyncTimestamp
	nextRequestID          RequestID
	requests               map[RequestID]*trackedSyncRequest
	completedMutations     map[RequestID]struct{}
	completedMutationOrder []RequestID
	mutationResults        map[RequestID]FunctionResult
	actionResults          map[RequestID]FunctionResult
	actionResultOrder      []RequestID
}

func New() *Client {
	return &Client{
		queries:            map[string]*localBaseQuery{},
		queryIDToToken:     map[QueryID]string{},
		subscribers:        map[SubscriberID]struct{}{},
		serverResults:      map[QueryID]FunctionResult{},
		results:            map[QueryID]FunctionResult{},
		requests:           map[RequestID]*trackedSyncRequest{},
		completedMutations: map[RequestID]struct{}{},
		mutationResults:    map[RequestID]FunctionResult{},
		actionResults:      map[RequestID]FunctionResult{},
	}
}

func (c *Client) Subscribe(path string, args any) (SubscriberID, error) {
	canonicalPath, encodedArgs, token, err := canonicalQuery(path, args)
	if err != nil {
		return SubscriberID{}, err
	}
	if query, ok := c.queries[token]; ok {
		query.numSubscribers++
		query.nextSubscriberIdx++
		id := SubscriberID{queryID: query.id, index: query.nextSubscriberIdx}
		c.subscribers[id] = struct{}{}
		return id, nil
	}

	queryID := c.nextQueryID
	c.nextQueryID++
	baseVersion := c.querySetVersion
	c.querySetVersion++

	query := &localBaseQuery{
		id:                queryID,
		canonicalPath:     canonicalPath,
		args:              encodedArgs,
		numSubscribers:    1,
		nextSubscriberIdx: 1,
	}
	c.queries[token] = query
	c.queryIDToToken[queryID] = token
	id := SubscriberID{queryID: queryID, index: query.nextSubscriberIdx}
	c.subscribers[id] = struct{}{}
	c.outgoing = append(c.outgoing, ModifyQuerySetMessage{
		BaseVersion: baseVersion,
		NewVersion:  c.querySetVersion,
		Modifications: []QuerySetModification{
			QuerySetAdd{
				QueryID: queryID,
				UDFPath: canonicalPath,
				Args:    encodedArgs,
			},
		},
	})
	return id, nil
}

func (c *Client) PopNextMessage() ClientMessage {
	if len(c.outgoing) == 0 {
		return nil
	}
	msg := c.outgoing[0]
	copy(c.outgoing, c.outgoing[1:])
	c.outgoing = c.outgoing[:len(c.outgoing)-1]
	return msg
}

func (c *Client) Unsubscribe(id SubscriberID) error {
	if _, ok := c.subscribers[id]; !ok {
		return fmt.Errorf("convex: unknown subscriber %v", id)
	}
	delete(c.subscribers, id)
	token, ok := c.queryIDToToken[id.queryID]
	if !ok {
		return fmt.Errorf("convex: unknown query id %d", id.queryID)
	}
	query, ok := c.queries[token]
	if !ok {
		return fmt.Errorf("convex: query token missing for query id %d", id.queryID)
	}
	if query.numSubscribers > 1 {
		query.numSubscribers--
		return nil
	}

	delete(c.queries, token)
	delete(c.queryIDToToken, id.queryID)
	delete(c.serverResults, id.queryID)
	delete(c.results, id.queryID)
	baseVersion := c.querySetVersion
	c.querySetVersion++
	c.outgoing = append(c.outgoing, ModifyQuerySetMessage{
		BaseVersion: baseVersion,
		NewVersion:  c.querySetVersion,
		Modifications: []QuerySetModification{
			QuerySetRemove{QueryID: id.queryID},
		},
	})
	return nil
}

func (c *Client) LatestResults() QueryResults {
	return c.queryResultsSnapshot()
}

func (c *Client) MaxObservedTimestamp() (SyncTimestamp, bool) {
	if c.maxObservedTimestamp == nil {
		return 0, false
	}
	return *c.maxObservedTimestamp, true
}

// MutationResult returns a recently recorded mutation response by request id.
func (c *Client) MutationResult(id RequestID) (FunctionResult, bool) {
	result, ok := c.mutationResults[id]
	return result, ok
}

// ActionResult returns a recently recorded action response by request id.
func (c *Client) ActionResult(id RequestID) (FunctionResult, bool) {
	result, ok := c.actionResults[id]
	return result, ok
}

func (c *Client) SetAuth(token string) error {
	auth := UserAuthToken(token)
	c.authFetcher = func(forceRefresh bool) (AuthToken, error) {
		return auth, nil
	}
	return c.queueAuth(auth)
}

func (c *Client) SetAdminAuth(token string, actingAs ...SyncUserIdentityAttributes) error {
	auth := AdminAuthToken(token, actingAs...)
	c.authFetcher = func(forceRefresh bool) (AuthToken, error) {
		return auth, nil
	}
	return c.queueAuth(auth)
}

func (c *Client) ClearAuth() error {
	c.authFetcher = nil
	return c.queueAuth(NoAuthToken())
}

func (c *Client) SetAuthCallback(fetcher AuthTokenFetcher) error {
	if fetcher == nil {
		c.authFetcher = nil
		return c.queueAuth(NoAuthToken())
	}
	token, err := fetcher(false)
	if err != nil {
		return err
	}
	c.authFetcher = fetcher
	return c.queueAuth(token)
}

func (c *Client) RefreshAuthForReconnect() error {
	if c.authFetcher == nil {
		return fmt.Errorf("convex: no auth callback configured")
	}
	token, err := c.authFetcher(true)
	if err != nil {
		return err
	}
	return c.queueAuth(token)
}

func (c *Client) Mutation(path string, args any, opts ...SyncMutationOption) (RequestID, error) {
	id, _, err := c.mutation(path, args, opts...)
	return id, err
}

func (c *Client) mutation(path string, args any, opts ...SyncMutationOption) (RequestID, *QueryResults, error) {
	canonicalPath, encodedArgs, err := canonicalRequest(path, args)
	if err != nil {
		return 0, nil, err
	}
	options := syncMutationOptionsFrom(opts)
	requestID := c.nextRequestID
	message := MutationMessage{
		RequestID: requestID,
		UDFPath:   canonicalPath,
		Args:      encodedArgs,
	}
	request := &trackedSyncRequest{
		id:               requestID,
		kind:             syncRequestMutation,
		message:          message,
		optimisticUpdate: options.optimisticUpdate,
	}
	var snapshot *QueryResults
	if options.optimisticUpdate != nil {
		results, err := c.optimisticResultsWith(request)
		if err != nil {
			return 0, nil, err
		}
		c.results = results
		current := c.queryResultsSnapshot()
		snapshot = &current
	}
	c.nextRequestID++
	c.requests[requestID] = request
	c.outgoing = append(c.outgoing, message)
	return requestID, snapshot, nil
}

func (c *Client) Action(path string, args any) (RequestID, error) {
	canonicalPath, encodedArgs, err := canonicalRequest(path, args)
	if err != nil {
		return 0, err
	}
	requestID := c.nextRequestID
	c.nextRequestID++
	message := ActionMessage{
		RequestID: requestID,
		UDFPath:   canonicalPath,
		Args:      encodedArgs,
	}
	c.requests[requestID] = &trackedSyncRequest{
		id:      requestID,
		kind:    syncRequestAction,
		message: message,
	}
	c.outgoing = append(c.outgoing, message)
	return requestID, nil
}

func (c *Client) ReplayOngoingRequests() []ClientMessage {
	requests := make([]*trackedSyncRequest, 0, len(c.requests))
	for _, request := range c.requests {
		if request.kind != syncRequestMutation {
			continue
		}
		requests = append(requests, request)
	}
	sort.Slice(requests, func(i, j int) bool {
		left, right := requests[i], requests[j]
		if left.ts != nil && right.ts != nil && *left.ts != *right.ts {
			return *left.ts > *right.ts
		}
		if (left.ts != nil) != (right.ts != nil) {
			return left.ts != nil
		}
		return left.id < right.id
	})
	messages := make([]ClientMessage, len(requests))
	for i, request := range requests {
		messages[i] = request.message
	}
	return messages
}

func (c *Client) RestartForReconnect() error {
	c.outgoing = nil
	c.identityVersion = 0
	c.querySetVersion = 0
	c.remoteVersion = StateVersion{}
	c.cancelPendingActionsForReconnect()

	if c.authFetcher != nil {
		if err := c.RefreshAuthForReconnect(); err != nil {
			return err
		}
	}

	adds := c.replayQueryAdds()
	if len(adds) > 0 {
		baseVersion := c.querySetVersion
		c.querySetVersion++
		c.outgoing = append(c.outgoing, ModifyQuerySetMessage{
			BaseVersion:   baseVersion,
			NewVersion:    c.querySetVersion,
			Modifications: adds,
		})
	}
	c.outgoing = append(c.outgoing, c.ReplayOngoingRequests()...)
	return nil
}

func (c *Client) GetQuery(id QueryID) (FunctionResult, bool) {
	if _, ok := c.queryIDToToken[id]; !ok {
		return FunctionResult{}, false
	}
	result, ok := c.results[id]
	return result, ok
}
