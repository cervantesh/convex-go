package syncprotocol

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/cervantesh/convex-go/internal/core"
)

type Value = core.Value

// QueryID identifies a query within a Convex sync query set.
type QueryID uint32

// QuerySetVersion is the monotonically increasing version for query set edits.
type QuerySetVersion uint32

// IdentityVersion is the monotonically increasing version for auth identity.
type IdentityVersion uint32

// RequestID identifies a mutation or action request on the sync connection.
type RequestID uint32

// SessionRequestSeqNumber identifies a request within a sync session.
type SessionRequestSeqNumber uint32

// SyncTimestamp is Convex's sync-protocol timestamp. It is encoded on the wire
// as base64 over an 8-byte little-endian uint64, unlike HTTP timestamp tokens.
type SyncTimestamp uint64

func (t SyncTimestamp) MarshalJSON() ([]byte, error) {
	if uint64(t) > uint64(math.MaxInt64) {
		return nil, fmt.Errorf("convex: sync timestamp %d exceeds maximum %d", uint64(t), int64(math.MaxInt64))
	}
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(t))
	return json.Marshal(base64.StdEncoding.EncodeToString(buf[:]))
}

func (t *SyncTimestamp) UnmarshalJSON(data []byte) error {
	var encoded string
	if err := json.Unmarshal(data, &encoded); err != nil {
		return fmt.Errorf("convex: sync timestamp must be a base64 string: %w", err)
	}
	decoded, err := decodeSyncBase64Compat(encoded)
	if err != nil {
		return fmt.Errorf("convex: malformed sync timestamp: %w", err)
	}
	if len(decoded) != 8 {
		return fmt.Errorf("convex: received %d bytes, expected 8 for sync timestamp", len(decoded))
	}
	value := binary.LittleEndian.Uint64(decoded)
	if value > uint64(math.MaxInt64) {
		return fmt.Errorf("convex: sync timestamp %d exceeds maximum %d", value, int64(math.MaxInt64))
	}
	*t = SyncTimestamp(value)
	return nil
}

func decodeSyncBase64Compat(encoded string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err == nil {
		return decoded, nil
	}
	if decoded, urlErr := base64.URLEncoding.DecodeString(encoded); urlErr == nil {
		return decoded, nil
	}
	return nil, err
}

// OptionalString tracks whether a nullable string field was missing, present as
// null, or present as a concrete string.
type OptionalString struct {
	Present bool
	Value   *string
}

// OptionalValue tracks whether a nullable Convex Value field was missing,
// present as null, or present as a concrete Convex value.
type OptionalValue struct {
	Present bool
	Value   Value
}

// ClientMessage is a Convex sync message sent by a client.
type ClientMessage interface {
	clientMessage()
}

// ServerMessage is a Convex sync message sent by the server.
type ServerMessage interface {
	serverMessage()
}

// QuerySetModification is a modification within ModifyQuerySet.
type QuerySetModification interface {
	querySetModification()
}

// StateModification is a query result state modification from a Transition.
type StateModification interface {
	stateModification()
}

// DecodeClientMessage decodes a discriminated Convex sync client message.
func DecodeClientMessage(data []byte) (ClientMessage, error) {
	messageType, err := discriminator(data)
	if err != nil {
		return nil, err
	}
	switch messageType {
	case "Connect":
		var msg ConnectMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "ModifyQuerySet":
		var msg ModifyQuerySetMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "Mutation":
		var msg MutationMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "Action":
		var msg ActionMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "Authenticate":
		var msg AuthenticateMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "Event":
		var msg EventMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	default:
		return nil, fmt.Errorf("convex: unknown client sync message type %q", messageType)
	}
}

// EncodeClientMessage encodes a Convex sync client message.
func EncodeClientMessage(msg ClientMessage) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("convex: nil client sync message")
	}
	return json.Marshal(msg)
}

// DecodeServerMessage decodes a discriminated Convex sync server message.
func DecodeServerMessage(data []byte) (ServerMessage, error) {
	messageType, err := discriminator(data)
	if err != nil {
		return nil, err
	}
	switch messageType {
	case "Transition":
		var msg TransitionMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "TransitionChunk":
		var msg TransitionChunkMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "MutationResponse":
		var msg MutationResponseMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "ActionResponse":
		var msg ActionResponseMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "AuthError":
		var msg AuthErrorMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "FatalError":
		var msg FatalErrorMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "Ping":
		return PingMessage{}, nil
	default:
		return nil, fmt.Errorf("convex: unknown server sync message type %q", messageType)
	}
}

// EncodeServerMessage encodes a Convex sync server message.
func EncodeServerMessage(msg ServerMessage) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("convex: nil server sync message")
	}
	return json.Marshal(msg)
}

// ConnectMessage begins or resumes a sync session.
type ConnectMessage struct {
	SessionID            string
	ConnectionCount      uint32
	LastCloseReason      string
	MaxObservedTimestamp *SyncTimestamp
	ClientTS             *int64

	lastCloseReasonPresent bool
	lastCloseReasonNull    bool
}

func (ConnectMessage) clientMessage() {
	// Marker method restricts client message implementations to this package.
}

func (m ConnectMessage) MarshalJSON() ([]byte, error) {
	out := map[string]any{
		"type":            "Connect",
		"sessionId":       m.SessionID,
		"connectionCount": m.ConnectionCount,
	}
	if m.LastCloseReason != "" {
		out["lastCloseReason"] = m.LastCloseReason
	}
	if m.lastCloseReasonPresent && m.lastCloseReasonNull {
		out["lastCloseReason"] = nil
	}
	if m.MaxObservedTimestamp != nil {
		out["maxObservedTimestamp"] = m.MaxObservedTimestamp
	}
	if m.ClientTS != nil {
		out["clientTs"] = *m.ClientTS
	}
	return json.Marshal(out)
}

func (m *ConnectMessage) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var sessionID string
	var connectionCount uint32
	var maxObservedTimestamp *SyncTimestamp
	var clientTS *int64
	if err := unmarshalExactField(raw, "sessionId", &sessionID); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "connectionCount", &connectionCount); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "maxObservedTimestamp", &maxObservedTimestamp); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "clientTs", &clientTS); err != nil {
		return err
	}
	var lastCloseReason string
	lastCloseReasonRaw, lastCloseReasonPresent := raw["lastCloseReason"]
	lastCloseReasonNull := false
	if lastCloseReasonPresent {
		if rawIsNull(lastCloseReasonRaw) {
			lastCloseReasonNull = true
		} else if err := json.Unmarshal(lastCloseReasonRaw, &lastCloseReason); err != nil {
			return fmt.Errorf("convex: lastCloseReason must be string or null: %w", err)
		}
	}
	if lastCloseReason == "" {
		lastCloseReason = "unknown"
	}
	m.SessionID = sessionID
	m.ConnectionCount = connectionCount
	m.LastCloseReason = lastCloseReason
	m.MaxObservedTimestamp = maxObservedTimestamp
	m.ClientTS = clientTS
	m.lastCloseReasonPresent = lastCloseReasonPresent
	m.lastCloseReasonNull = lastCloseReasonNull
	return nil
}

// ModifyQuerySetMessage applies Add and Remove edits to the active query set.
type ModifyQuerySetMessage struct {
	BaseVersion   QuerySetVersion
	NewVersion    QuerySetVersion
	Modifications []QuerySetModification
}

func (ModifyQuerySetMessage) clientMessage() {
	// Marker method restricts client message implementations to this package.
}

func (m ModifyQuerySetMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":          "ModifyQuerySet",
		"baseVersion":   m.BaseVersion,
		"newVersion":    m.NewVersion,
		"modifications": m.Modifications,
	})
}

func (m *ModifyQuerySetMessage) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	modificationsRaw, ok := raw["modifications"]
	if !ok || rawIsNull(modificationsRaw) {
		return fmt.Errorf("convex: ModifyQuerySet missing modifications")
	}
	var baseVersion QuerySetVersion
	var newVersion QuerySetVersion
	var modifications []json.RawMessage
	if err := unmarshalExactField(raw, "baseVersion", &baseVersion); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "newVersion", &newVersion); err != nil {
		return err
	}
	if err := json.Unmarshal(modificationsRaw, &modifications); err != nil {
		return err
	}
	mods := make([]QuerySetModification, len(modifications))
	for i, raw := range modifications {
		mod, err := decodeQuerySetModification(raw)
		if err != nil {
			return fmt.Errorf("convex: query set modification %d: %w", i, err)
		}
		mods[i] = mod
	}
	m.BaseVersion = baseVersion
	m.NewVersion = newVersion
	m.Modifications = mods
	return nil
}

// QuerySetAdd subscribes to a query in the active query set.
type QuerySetAdd struct {
	QueryID       QueryID
	UDFPath       string
	Args          []Value
	Journal       OptionalString
	ComponentPath string
}

func (QuerySetAdd) querySetModification() {
	// Marker method restricts query set modifications to this package.
}

func (m QuerySetAdd) MarshalJSON() ([]byte, error) {
	out := map[string]any{
		"type":    "Add",
		"queryId": m.QueryID,
		"udfPath": m.UDFPath,
		"args":    m.Args,
	}
	if m.Journal.Present {
		out["journal"] = optionalStringWire(m.Journal)
	}
	if m.ComponentPath != "" {
		out["componentPath"] = m.ComponentPath
	}
	return json.Marshal(out)
}

func (m *QuerySetAdd) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var queryID QueryID
	var udfPath string
	var args []Value
	var componentPath string
	if err := unmarshalExactField(raw, "queryId", &queryID); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "udfPath", &udfPath); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "args", &args); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "componentPath", &componentPath); err != nil {
		return err
	}
	journal, err := optionalStringFromRaw(raw, "journal")
	if err != nil {
		return err
	}
	m.QueryID = queryID
	m.UDFPath = udfPath
	m.Args = args
	m.Journal = journal
	m.ComponentPath = componentPath
	return nil
}

// QuerySetRemove unsubscribes from a query in the active query set.
type QuerySetRemove struct {
	QueryID QueryID
}

func (QuerySetRemove) querySetModification() {
	// Marker method restricts query set modifications to this package.
}

func (m QuerySetRemove) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":    "Remove",
		"queryId": m.QueryID,
	})
}

func (m *QuerySetRemove) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var queryID QueryID
	if err := unmarshalExactField(raw, "queryId", &queryID); err != nil {
		return err
	}
	m.QueryID = queryID
	return nil
}

// MutationMessage requests a Convex mutation over sync.
type MutationMessage struct {
	RequestID     RequestID
	UDFPath       string
	Args          []Value
	ComponentPath string
}

func (MutationMessage) clientMessage() {
	// Marker method restricts client message implementations to this package.
}

func (m MutationMessage) MarshalJSON() ([]byte, error) {
	out := map[string]any{
		"type":      "Mutation",
		"requestId": m.RequestID,
		"udfPath":   m.UDFPath,
		"args":      m.Args,
	}
	if m.ComponentPath != "" {
		out["componentPath"] = m.ComponentPath
	}
	return json.Marshal(out)
}

func (m *MutationMessage) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var requestID RequestID
	var udfPath string
	var args []Value
	var componentPath string
	if err := unmarshalExactField(raw, "requestId", &requestID); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "udfPath", &udfPath); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "args", &args); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "componentPath", &componentPath); err != nil {
		return err
	}
	*m = MutationMessage{
		RequestID:     requestID,
		UDFPath:       udfPath,
		Args:          args,
		ComponentPath: componentPath,
	}
	return nil
}

// ActionMessage requests a Convex action over sync.
type ActionMessage struct {
	RequestID     RequestID
	UDFPath       string
	Args          []Value
	ComponentPath string
}

func (ActionMessage) clientMessage() {
	// Marker method restricts client message implementations to this package.
}

func (m ActionMessage) MarshalJSON() ([]byte, error) {
	out := map[string]any{
		"type":      "Action",
		"requestId": m.RequestID,
		"udfPath":   m.UDFPath,
		"args":      m.Args,
	}
	if m.ComponentPath != "" {
		out["componentPath"] = m.ComponentPath
	}
	return json.Marshal(out)
}

func (m *ActionMessage) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var requestID RequestID
	var udfPath string
	var args []Value
	var componentPath string
	if err := unmarshalExactField(raw, "requestId", &requestID); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "udfPath", &udfPath); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "args", &args); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "componentPath", &componentPath); err != nil {
		return err
	}
	*m = ActionMessage{
		RequestID:     requestID,
		UDFPath:       udfPath,
		Args:          args,
		ComponentPath: componentPath,
	}
	return nil
}

// AuthTokenType identifies a Convex sync authentication token variant.
type AuthTokenType string

const (
	AuthTokenNone  AuthTokenType = "None"
	AuthTokenUser  AuthTokenType = "User"
	AuthTokenAdmin AuthTokenType = "Admin"
)

// AuthenticateMessage updates sync authentication state.
type AuthenticateMessage struct {
	BaseVersion IdentityVersion
	TokenType   AuthTokenType
	Value       string
	ActingAs    *SyncUserIdentityAttributes
}

func (AuthenticateMessage) clientMessage() {
	// Marker method restricts client message implementations to this package.
}

func (m AuthenticateMessage) MarshalJSON() ([]byte, error) {
	out := map[string]any{
		"type":        "Authenticate",
		"baseVersion": m.BaseVersion,
		"tokenType":   m.TokenType,
	}
	if m.Value != "" {
		out["value"] = m.Value
	}
	if m.ActingAs != nil {
		out["impersonating"] = m.ActingAs
	}
	return json.Marshal(out)
}

func (m *AuthenticateMessage) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var baseVersion IdentityVersion
	var tokenType AuthTokenType
	var value string
	if err := unmarshalExactField(raw, "baseVersion", &baseVersion); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "tokenType", &tokenType); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "value", &value); err != nil {
		return err
	}
	var actingAs *SyncUserIdentityAttributes
	identityRaw := raw["actingAs"]
	if len(identityRaw) == 0 {
		identityRaw = raw["impersonating"]
	}
	if len(identityRaw) != 0 && !rawIsNull(identityRaw) {
		var identity SyncUserIdentityAttributes
		if err := json.Unmarshal(identityRaw, &identity); err != nil {
			return fmt.Errorf("convex: invalid actingAs identity: %w", err)
		}
		actingAs = &identity
	}
	m.BaseVersion = baseVersion
	m.TokenType = tokenType
	m.Value = value
	m.ActingAs = actingAs
	return nil
}

// EventMessage carries a client-side sync event payload.
type EventMessage struct {
	EventType string
	Event     json.RawMessage
}

func (EventMessage) clientMessage() {
	// Marker method restricts client message implementations to this package.
}

func (m EventMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":      "Event",
		"eventType": m.EventType,
		"event":     m.Event,
	})
}

func (m *EventMessage) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var eventType string
	if err := unmarshalExactField(raw, "eventType", &eventType); err != nil {
		return err
	}
	m.EventType = eventType
	m.Event = append(json.RawMessage(nil), raw["event"]...)
	return nil
}

// StateVersion identifies a sync result state.
type StateVersion struct {
	QuerySet QuerySetVersion
	Identity IdentityVersion
	TS       SyncTimestamp
}

func (v StateVersion) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"querySet": v.QuerySet,
		"identity": v.Identity,
		"ts":       v.TS,
	})
}

func (v *StateVersion) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "querySet", &v.QuerySet); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "identity", &v.Identity); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "ts", &v.TS); err != nil {
		return err
	}
	return nil
}

// TransitionMessage carries query state changes.
type TransitionMessage struct {
	StartVersion    StateVersion
	EndVersion      StateVersion
	Modifications   []StateModification
	ClientClockSkew *float64
	ServerTS        *int64

	clientClockSkewPresent bool
	serverTSPresent        bool
}

func (TransitionMessage) serverMessage() {
	// Marker method restricts server message implementations to this package.
}

func (m TransitionMessage) MarshalJSON() ([]byte, error) {
	out := map[string]any{
		"type":          "Transition",
		"startVersion":  m.StartVersion,
		"endVersion":    m.EndVersion,
		"modifications": m.Modifications,
	}
	if m.clientClockSkewPresent {
		out["clientClockSkew"] = m.ClientClockSkew
	}
	if m.serverTSPresent {
		out["serverTs"] = m.ServerTS
	}
	return json.Marshal(out)
}

func (m *TransitionMessage) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var startVersion StateVersion
	var endVersion StateVersion
	var modificationRaw []json.RawMessage
	if err := unmarshalExactField(raw, "startVersion", &startVersion); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "endVersion", &endVersion); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "modifications", &modificationRaw); err != nil {
		return err
	}
	mods := make([]StateModification, len(modificationRaw))
	for i, raw := range modificationRaw {
		mod, err := decodeStateModification(raw)
		if err != nil {
			return fmt.Errorf("convex: state modification %d: %w", i, err)
		}
		mods[i] = mod
	}
	clientClockSkew, clientClockSkewPresent, err := optionalFloat64FromRaw(raw, "clientClockSkew")
	if err != nil {
		return err
	}
	serverTS, serverTSPresent, err := optionalInt64FromRaw(raw, "serverTs")
	if err != nil {
		return err
	}
	m.StartVersion = startVersion
	m.EndVersion = endVersion
	m.Modifications = mods
	m.ClientClockSkew = clientClockSkew
	m.ServerTS = serverTS
	m.clientClockSkewPresent = clientClockSkewPresent
	m.serverTSPresent = serverTSPresent
	return nil
}

// QueryUpdated reports a successful query result update.
type QueryUpdated struct {
	QueryID  QueryID
	Value    Value
	LogLines []string
	Journal  OptionalString
}

func (QueryUpdated) stateModification() {
	// Marker method restricts state modifications to this package.
}

func (m QueryUpdated) MarshalJSON() ([]byte, error) {
	out := map[string]any{
		"type":     "QueryUpdated",
		"queryId":  m.QueryID,
		"value":    m.Value,
		"logLines": m.LogLines,
		"journal":  optionalStringWire(m.Journal),
	}
	return json.Marshal(out)
}

func (m *QueryUpdated) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var queryID QueryID
	var value Value
	var logLines []string
	if err := unmarshalExactField(raw, "queryId", &queryID); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "value", &value); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "logLines", &logLines); err != nil {
		return err
	}
	journal, err := optionalStringFromRaw(raw, "journal")
	if err != nil {
		return err
	}
	if !journal.Present {
		return fmt.Errorf("convex: QueryUpdated missing journal")
	}
	m.QueryID = queryID
	m.Value = value
	m.LogLines = logLines
	m.Journal = journal
	return nil
}

// QueryFailed reports a failed query result update.
type QueryFailed struct {
	QueryID      QueryID
	ErrorMessage string
	LogLines     []string
	Journal      OptionalString
	ErrorData    OptionalValue
}

func (QueryFailed) stateModification() {
	// Marker method restricts state modifications to this package.
}

func (m QueryFailed) MarshalJSON() ([]byte, error) {
	out := map[string]any{
		"type":         "QueryFailed",
		"queryId":      m.QueryID,
		"errorMessage": m.ErrorMessage,
		"logLines":     m.LogLines,
		"journal":      optionalStringWire(m.Journal),
	}
	if m.ErrorData.Present {
		out["errorData"] = m.ErrorData.Value
	}
	return json.Marshal(out)
}

func (m *QueryFailed) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var queryID QueryID
	var errorMessage string
	var logLines []string
	if err := unmarshalExactField(raw, "queryId", &queryID); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "errorMessage", &errorMessage); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "logLines", &logLines); err != nil {
		return err
	}
	journal, err := optionalStringFromRaw(raw, "journal")
	if err != nil {
		return err
	}
	if !journal.Present {
		return fmt.Errorf("convex: QueryFailed missing journal")
	}
	errorData, err := optionalValueFromRaw(raw, "errorData")
	if err != nil {
		return err
	}
	m.QueryID = queryID
	m.ErrorMessage = errorMessage
	m.LogLines = logLines
	m.Journal = journal
	m.ErrorData = errorData
	return nil
}

// QueryRemoved reports a query result removal.
type QueryRemoved struct {
	QueryID QueryID
}

func (QueryRemoved) stateModification() {
	// Marker method restricts state modifications to this package.
}

func (m QueryRemoved) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":    "QueryRemoved",
		"queryId": m.QueryID,
	})
}

func (m *QueryRemoved) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var queryID QueryID
	if err := unmarshalExactField(raw, "queryId", &queryID); err != nil {
		return err
	}
	m.QueryID = queryID
	return nil
}

// MutationResponseMessage is a server response to a sync mutation request.
type MutationResponseMessage struct {
	RequestID    RequestID
	Success      bool
	Value        Value
	ErrorMessage string
	TS           *SyncTimestamp
	LogLines     []string
	ErrorData    OptionalValue

	tsPresent bool
}

func (MutationResponseMessage) serverMessage() {
	// Marker method restricts server message implementations to this package.
}

func (m MutationResponseMessage) MarshalJSON() ([]byte, error) {
	out := map[string]any{
		"type":      "MutationResponse",
		"requestId": m.RequestID,
		"success":   m.Success,
		"logLines":  m.LogLines,
	}
	if m.Success {
		out["result"] = m.Value
	} else {
		out["result"] = m.ErrorMessage
		if m.ErrorData.Present {
			out["errorData"] = m.ErrorData.Value
		}
	}
	if m.tsPresent || m.TS != nil {
		out["ts"] = m.TS
	}
	return json.Marshal(out)
}

func (m *MutationResponseMessage) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var requestID RequestID
	var success bool
	var logLines []string
	if err := unmarshalExactField(raw, "requestId", &requestID); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "success", &success); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "logLines", &logLines); err != nil {
		return err
	}
	result := raw["result"]
	ts, tsPresent, err := optionalTimestampFromRaw(raw, "ts")
	if err != nil {
		return err
	}
	errorData, err := optionalValueFromRaw(raw, "errorData")
	if err != nil {
		return err
	}
	m.RequestID = requestID
	m.Success = success
	m.TS = ts
	m.tsPresent = tsPresent
	m.LogLines = logLines
	m.ErrorData = errorData
	if success {
		return json.Unmarshal(result, &m.Value)
	}
	return json.Unmarshal(result, &m.ErrorMessage)
}

// ActionResponseMessage is a server response to a sync action request.
type ActionResponseMessage struct {
	RequestID    RequestID
	Success      bool
	Value        Value
	ErrorMessage string
	LogLines     []string
	ErrorData    OptionalValue
}

func (ActionResponseMessage) serverMessage() {
	// Marker method restricts server message implementations to this package.
}

func (m ActionResponseMessage) MarshalJSON() ([]byte, error) {
	out := map[string]any{
		"type":      "ActionResponse",
		"requestId": m.RequestID,
		"success":   m.Success,
		"logLines":  m.LogLines,
	}
	if m.Success {
		out["result"] = m.Value
	} else {
		out["result"] = m.ErrorMessage
		if m.ErrorData.Present {
			out["errorData"] = m.ErrorData.Value
		}
	}
	return json.Marshal(out)
}

func (m *ActionResponseMessage) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var requestID RequestID
	var success bool
	var logLines []string
	if err := unmarshalExactField(raw, "requestId", &requestID); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "success", &success); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "logLines", &logLines); err != nil {
		return err
	}
	result := raw["result"]
	errorData, err := optionalValueFromRaw(raw, "errorData")
	if err != nil {
		return err
	}
	m.RequestID = requestID
	m.Success = success
	m.LogLines = logLines
	m.ErrorData = errorData
	if success {
		return json.Unmarshal(result, &m.Value)
	}
	return json.Unmarshal(result, &m.ErrorMessage)
}

// AuthErrorMessage reports an authentication update failure.
type AuthErrorMessage struct {
	Error               string
	BaseVersion         *IdentityVersion
	AuthUpdateAttempted *bool

	baseVersionPresent bool
}

func (AuthErrorMessage) serverMessage() {
	// Marker method restricts server message implementations to this package.
}

func (m AuthErrorMessage) MarshalJSON() ([]byte, error) {
	out := map[string]any{
		"type":  "AuthError",
		"error": m.Error,
	}
	if m.baseVersionPresent || m.BaseVersion != nil {
		out["baseVersion"] = m.BaseVersion
	}
	if m.AuthUpdateAttempted != nil {
		out["authUpdateAttempted"] = *m.AuthUpdateAttempted
	}
	return json.Marshal(out)
}

func (m *AuthErrorMessage) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var message string
	var authUpdateAttempted *bool
	if err := unmarshalExactField(raw, "error", &message); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "authUpdateAttempted", &authUpdateAttempted); err != nil {
		return err
	}
	baseVersion, present, err := optionalIdentityVersionFromRaw(raw, "baseVersion")
	if err != nil {
		return err
	}
	m.Error = message
	m.BaseVersion = baseVersion
	m.baseVersionPresent = present
	m.AuthUpdateAttempted = authUpdateAttempted
	return nil
}

// FatalErrorMessage reports a fatal sync protocol error.
type FatalErrorMessage struct {
	Error string
}

func (FatalErrorMessage) serverMessage() {
	// Marker method restricts server message implementations to this package.
}

func (m FatalErrorMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":  "FatalError",
		"error": m.Error,
	})
}

func (m *FatalErrorMessage) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var message string
	if err := unmarshalExactField(raw, "error", &message); err != nil {
		return err
	}
	m.Error = message
	return nil
}

// PingMessage is a server heartbeat message.
type PingMessage struct{}

func (PingMessage) serverMessage() {
	// Marker method restricts server message implementations to this package.
}

func (PingMessage) MarshalJSON() ([]byte, error) {
	return []byte(`{"type":"Ping"}`), nil
}

// TransitionChunkMessage carries one chunk of a large Transition.
type TransitionChunkMessage struct {
	Chunk        string
	PartNumber   uint32
	TotalParts   uint32
	TransitionID string
}

func (TransitionChunkMessage) serverMessage() {
	// Marker method restricts server message implementations to this package.
}

func (m TransitionChunkMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"type":         "TransitionChunk",
		"chunk":        m.Chunk,
		"partNumber":   m.PartNumber,
		"totalParts":   m.TotalParts,
		"transitionId": m.TransitionID,
	})
}

func (m *TransitionChunkMessage) UnmarshalJSON(data []byte) error {
	raw, err := rawObject(data)
	if err != nil {
		return err
	}
	var chunk string
	var partNumber uint32
	var totalParts uint32
	var transitionID string
	if err := unmarshalExactField(raw, "chunk", &chunk); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "partNumber", &partNumber); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "totalParts", &totalParts); err != nil {
		return err
	}
	if err := unmarshalExactField(raw, "transitionId", &transitionID); err != nil {
		return err
	}
	*m = TransitionChunkMessage{
		Chunk:        chunk,
		PartNumber:   partNumber,
		TotalParts:   totalParts,
		TransitionID: transitionID,
	}
	return nil
}

func decodeQuerySetModification(raw json.RawMessage) (QuerySetModification, error) {
	modType, err := discriminator(raw)
	if err != nil {
		return nil, err
	}
	switch modType {
	case "Add":
		var mod QuerySetAdd
		if err := json.Unmarshal(raw, &mod); err != nil {
			return nil, err
		}
		return mod, nil
	case "Remove":
		var mod QuerySetRemove
		if err := json.Unmarshal(raw, &mod); err != nil {
			return nil, err
		}
		return mod, nil
	default:
		return nil, fmt.Errorf("convex: unknown query set modification type %q", modType)
	}
}

func decodeStateModification(raw json.RawMessage) (StateModification, error) {
	modType, err := discriminator(raw)
	if err != nil {
		return nil, err
	}
	switch modType {
	case "QueryUpdated":
		var mod QueryUpdated
		if err := json.Unmarshal(raw, &mod); err != nil {
			return nil, err
		}
		return mod, nil
	case "QueryFailed":
		var mod QueryFailed
		if err := json.Unmarshal(raw, &mod); err != nil {
			return nil, err
		}
		return mod, nil
	case "QueryRemoved":
		var mod QueryRemoved
		if err := json.Unmarshal(raw, &mod); err != nil {
			return nil, err
		}
		return mod, nil
	default:
		return nil, fmt.Errorf("convex: unknown state modification type %q", modType)
	}
}

func discriminator(data []byte) (string, error) {
	raw, err := rawObject(data)
	if err != nil {
		return "", err
	}
	var messageType string
	if err := unmarshalExactField(raw, "type", &messageType); err != nil {
		return "", err
	}
	if messageType == "" {
		return "", fmt.Errorf("convex: sync message missing type")
	}
	return messageType, nil
}

func rawObject(data []byte) (map[string]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func unmarshalExactField(raw map[string]json.RawMessage, field string, out any) error {
	valueRaw, ok := raw[field]
	if !ok {
		return nil
	}
	if err := json.Unmarshal(valueRaw, out); err != nil {
		return fmt.Errorf("convex: field %s: %w", field, err)
	}
	return nil
}

func optionalStringFromRaw(raw map[string]json.RawMessage, field string) (OptionalString, error) {
	valueRaw, ok := raw[field]
	if !ok {
		return OptionalString{}, nil
	}
	if rawIsNull(valueRaw) {
		return OptionalString{Present: true}, nil
	}
	var value string
	if err := json.Unmarshal(valueRaw, &value); err != nil {
		return OptionalString{}, fmt.Errorf("convex: field %s must be string or null: %w", field, err)
	}
	return OptionalString{Present: true, Value: &value}, nil
}

func optionalStringWire(value OptionalString) any {
	if value.Value == nil {
		return nil
	}
	return *value.Value
}

func optionalValueFromRaw(raw map[string]json.RawMessage, field string) (OptionalValue, error) {
	valueRaw, ok := raw[field]
	if !ok {
		return OptionalValue{}, nil
	}
	if rawIsNull(valueRaw) {
		return OptionalValue{Present: true, Value: core.NullValue()}, nil
	}
	var value Value
	if err := json.Unmarshal(valueRaw, &value); err != nil {
		return OptionalValue{}, fmt.Errorf("convex: field %s must be Convex value or null: %w", field, err)
	}
	return OptionalValue{Present: true, Value: value}, nil
}

func optionalTimestampFromRaw(raw map[string]json.RawMessage, field string) (*SyncTimestamp, bool, error) {
	valueRaw, ok := raw[field]
	if !ok {
		return nil, false, nil
	}
	if rawIsNull(valueRaw) {
		return nil, true, nil
	}
	var value SyncTimestamp
	if err := json.Unmarshal(valueRaw, &value); err != nil {
		return nil, false, err
	}
	return &value, true, nil
}

func optionalFloat64FromRaw(raw map[string]json.RawMessage, field string) (*float64, bool, error) {
	valueRaw, ok := raw[field]
	if !ok {
		return nil, false, nil
	}
	if rawIsNull(valueRaw) {
		return nil, true, nil
	}
	var value float64
	if err := json.Unmarshal(valueRaw, &value); err != nil {
		return nil, false, fmt.Errorf("convex: field %s must be number or null: %w", field, err)
	}
	return &value, true, nil
}

func optionalInt64FromRaw(raw map[string]json.RawMessage, field string) (*int64, bool, error) {
	valueRaw, ok := raw[field]
	if !ok {
		return nil, false, nil
	}
	if rawIsNull(valueRaw) {
		return nil, true, nil
	}
	var value int64
	if err := json.Unmarshal(valueRaw, &value); err != nil {
		return nil, false, fmt.Errorf("convex: field %s must be integer or null: %w", field, err)
	}
	return &value, true, nil
}

func optionalIdentityVersionFromRaw(raw map[string]json.RawMessage, field string) (*IdentityVersion, bool, error) {
	valueRaw, ok := raw[field]
	if !ok {
		return nil, false, nil
	}
	if rawIsNull(valueRaw) {
		return nil, true, nil
	}
	var value IdentityVersion
	if err := json.Unmarshal(valueRaw, &value); err != nil {
		return nil, false, err
	}
	return &value, true, nil
}

func rawIsNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}

func decodeRawMap(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	var out map[string]any
	if err := decoder.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func claimString(claims map[string]any, field string) (string, bool) {
	value, ok := claims[field]
	if !ok {
		return "", false
	}
	s, ok := value.(string)
	return s, ok
}
