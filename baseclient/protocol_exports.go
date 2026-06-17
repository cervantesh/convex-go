package baseclient

import (
	"sort"

	"github.com/cervantesh/convex-go/internal/core"
	"github.com/cervantesh/convex-go/internal/syncprotocol"
)

type (
	ActionMessage              = syncprotocol.ActionMessage
	ActionResponseMessage      = syncprotocol.ActionResponseMessage
	AuthErrorMessage           = syncprotocol.AuthErrorMessage
	AuthTokenType              = syncprotocol.AuthTokenType
	AuthenticateMessage        = syncprotocol.AuthenticateMessage
	CanonicalizedUDFPath       = core.CanonicalizedUDFPath
	ClientMessage              = syncprotocol.ClientMessage
	ConvexError                = core.ConvexError
	FatalErrorMessage          = syncprotocol.FatalErrorMessage
	FunctionResult             = core.FunctionResult
	IdentityVersion            = syncprotocol.IdentityVersion
	Int64                      = core.Int64
	ModifyQuerySetMessage      = syncprotocol.ModifyQuerySetMessage
	ModulePath                 = core.ModulePath
	MutationMessage            = syncprotocol.MutationMessage
	MutationResponseMessage    = syncprotocol.MutationResponseMessage
	Number                     = core.Number
	OptionalString             = syncprotocol.OptionalString
	OptionalValue              = syncprotocol.OptionalValue
	PingMessage                = syncprotocol.PingMessage
	QueryFailed                = syncprotocol.QueryFailed
	QueryID                    = syncprotocol.QueryID
	QueryRemoved               = syncprotocol.QueryRemoved
	QuerySetAdd                = syncprotocol.QuerySetAdd
	QuerySetModification       = syncprotocol.QuerySetModification
	QuerySetRemove             = syncprotocol.QuerySetRemove
	QuerySetVersion            = syncprotocol.QuerySetVersion
	QueryUpdated               = syncprotocol.QueryUpdated
	RequestID                  = syncprotocol.RequestID
	ServerMessage              = syncprotocol.ServerMessage
	StateModification          = syncprotocol.StateModification
	StateVersion               = syncprotocol.StateVersion
	SyncTimestamp              = syncprotocol.SyncTimestamp
	SyncUserIdentityAttributes = syncprotocol.SyncUserIdentityAttributes
	TransitionChunkMessage     = syncprotocol.TransitionChunkMessage
	TransitionMessage          = syncprotocol.TransitionMessage
	UDFPath                    = core.UDFPath
	Value                      = core.Value
)

const (
	AuthTokenAdmin = syncprotocol.AuthTokenAdmin
	AuthTokenNone  = syncprotocol.AuthTokenNone
	AuthTokenUser  = syncprotocol.AuthTokenUser
)

func ValueResult(value Value) FunctionResult {
	return core.ValueResult(value)
}

func ErrorMessageResult(message string) FunctionResult {
	return core.ErrorMessageResult(message)
}

func ConvexErrorResult(err ConvexError) FunctionResult {
	return core.ConvexErrorResult(err)
}

func NullValue() Value {
	return core.NullValue()
}

func StringValue(value string) Value {
	return core.StringValue(value)
}

func Int64Value(value int64) Value {
	return core.Int64Value(value)
}

func ValueFromGo(value any) (Value, error) {
	return core.ValueFromGo(value)
}

func ParseFunctionPath(path string) (string, error) {
	return core.ParseFunctionPath(path)
}

func ParseUDFPath(path string) (UDFPath, error) {
	return core.ParseUDFPath(path)
}

func DecodeClientMessage(data []byte) (ClientMessage, error) {
	return syncprotocol.DecodeClientMessage(data)
}

func EncodeClientMessage(message ClientMessage) ([]byte, error) {
	return syncprotocol.EncodeClientMessage(message)
}

func DecodeServerMessage(data []byte) (ServerMessage, error) {
	return syncprotocol.DecodeServerMessage(data)
}

func EncodeServerMessage(message ServerMessage) ([]byte, error) {
	return syncprotocol.EncodeServerMessage(message)
}

type SubscriberID struct {
	queryID QueryID
	index   uint64
}

type QueryResultEntry struct {
	SubscriberID SubscriberID
	Result       FunctionResult
	HasResult    bool
}

type AuthToken struct {
	TokenType AuthTokenType
	Value     string
	ActingAs  *SyncUserIdentityAttributes
}

type AuthTokenFetcher func(forceRefresh bool) (AuthToken, error)

func UserAuthToken(token string) AuthToken {
	return AuthToken{TokenType: AuthTokenUser, Value: token}
}

func AdminAuthToken(token string, actingAs ...SyncUserIdentityAttributes) AuthToken {
	auth := AuthToken{TokenType: AuthTokenAdmin, Value: token}
	if len(actingAs) > 0 {
		identity := actingAs[0]
		if identity.TokenIdentifier == "" && identity.Issuer != "" && identity.Subject != "" {
			identity.TokenIdentifier = identity.Issuer + "|" + identity.Subject
		}
		auth.ActingAs = &identity
	}
	return auth
}

func NoAuthToken() AuthToken {
	return AuthToken{TokenType: AuthTokenNone}
}

type SyncAuthError struct {
	Message             string
	BaseVersion         *IdentityVersion
	AuthUpdateAttempted *bool
}

func (e *SyncAuthError) Error() string {
	return "convex: auth error: " + e.Message
}

type QueryResults struct {
	subscribers map[SubscriberID]struct{}
	results     map[QueryID]FunctionResult
}

func (r QueryResults) Get(id SubscriberID) (FunctionResult, bool) {
	if _, ok := r.subscribers[id]; !ok {
		return FunctionResult{}, false
	}
	result, ok := r.results[id.queryID]
	return result, ok
}

func (r QueryResults) Len() int {
	return len(r.subscribers)
}

func (r QueryResults) IsEmpty() bool {
	return len(r.subscribers) == 0
}

func (r QueryResults) Iter() []QueryResultEntry {
	ids := make([]SubscriberID, 0, len(r.subscribers))
	for id := range r.subscribers {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if ids[i].queryID != ids[j].queryID {
			return ids[i].queryID < ids[j].queryID
		}
		return ids[i].index < ids[j].index
	})
	entries := make([]QueryResultEntry, len(ids))
	for i, id := range ids {
		result, ok := r.results[id.queryID]
		entries[i] = QueryResultEntry{
			SubscriberID: id,
			Result:       result,
			HasResult:    ok,
		}
	}
	return entries
}
