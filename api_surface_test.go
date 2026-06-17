package convex

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var (
	_ func(string, ...Option) (*Client, error)     = NewClient
	_ func(string, ...Option) (*HTTPClient, error) = NewHTTPClient
)

func TestPublicAPISurfaceCanonicalMethods(t *testing.T) {
	ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
	anyType := reflect.TypeOf((*any)(nil)).Elem()

	httpType := reflect.TypeOf((*HTTPClient)(nil))
	assertMethod(t, httpType, "Query", []reflect.Type{ctxType, stringType(), anyType}, []reflect.Type{anyType, errorType()})
	assertMethod(t, httpType, "Mutation", []reflect.Type{ctxType, stringType(), anyType, reflect.TypeOf([]MutationOption{})}, []reflect.Type{anyType, errorType()})
	if method, _ := httpType.MethodByName("Mutation"); !method.Type.IsVariadic() {
		t.Fatal("HTTPClient.Mutation must keep the variadic MutationOption escape hatch")
	}
	assertMethod(t, httpType, "Action", []reflect.Type{ctxType, stringType(), anyType}, []reflect.Type{anyType, errorType()})

	clientType := reflect.TypeOf((*Client)(nil))
	assertMethod(t, clientType, "Query", []reflect.Type{ctxType, stringType(), anyType}, []reflect.Type{anyType, errorType()})
	assertMethod(t, clientType, "Mutation", []reflect.Type{ctxType, stringType(), anyType, reflect.TypeOf([]MutationOption{})}, []reflect.Type{anyType, errorType()})
	if method, _ := clientType.MethodByName("Mutation"); !method.Type.IsVariadic() {
		t.Fatal("Client.Mutation must keep the variadic MutationOption escape hatch")
	}
	assertMethod(t, clientType, "Action", []reflect.Type{ctxType, stringType(), anyType}, []reflect.Type{anyType, errorType()})
	assertMethod(t, clientType, "Subscribe", []reflect.Type{ctxType, stringType(), anyType}, []reflect.Type{reflect.TypeOf((*QuerySubscription)(nil)), errorType()})
	assertMethod(t, clientType, "SetAuthCallback", []reflect.Type{reflect.TypeOf((UserTokenFetcher)(nil))}, []reflect.Type{errorType()})
	assertMethod(t, clientType, "SetAuthCallbackContext", []reflect.Type{ctxType, reflect.TypeOf((UserTokenFetcher)(nil))}, []reflect.Type{errorType()})
	assertMethod(t, clientType, "ConnectionState", nil, []reflect.Type{reflect.TypeOf(ConnectionState{})})
	assertMethod(t, clientType, "SubscribeToConnectionState", []reflect.Type{reflect.TypeOf((func(ConnectionState))(nil))}, []reflect.Type{reflect.TypeOf((func())(nil))})
	assertMethod(t, clientType, "Close", nil, []reflect.Type{errorType()})
	if _, ok := clientType.MethodByName("Watch"); ok {
		t.Fatal("Watch must not become the primary realtime verb; use Subscribe and keep WatchAll as an advanced helper")
	}

	wsType := reflect.TypeOf((*WebSocketClient)(nil))
	assertMethod(t, wsType, "Subscribe", []reflect.Type{ctxType, stringType(), anyType}, []reflect.Type{reflect.TypeOf((*QuerySubscription)(nil)), errorType()})
	assertMethod(t, wsType, "Query", []reflect.Type{ctxType, stringType(), anyType}, []reflect.Type{anyType, errorType()})
	assertMethod(t, wsType, "Mutation", []reflect.Type{ctxType, stringType(), anyType, reflect.TypeOf([]SyncMutationOption{})}, []reflect.Type{errorType()})
	if method, _ := wsType.MethodByName("Mutation"); !method.Type.IsVariadic() {
		t.Fatal("WebSocketClient.Mutation must keep the variadic SyncMutationOption escape hatch")
	}
	assertMethod(t, wsType, "SetAuthCallback", []reflect.Type{reflect.TypeOf((UserTokenFetcher)(nil))}, []reflect.Type{errorType()})
	assertMethod(t, wsType, "SetAuthCallbackContext", []reflect.Type{ctxType, reflect.TypeOf((UserTokenFetcher)(nil))}, []reflect.Type{errorType()})
	assertMethod(t, wsType, "ConnectionState", nil, []reflect.Type{reflect.TypeOf(ConnectionState{})})
	assertMethod(t, wsType, "SubscribeToConnectionState", []reflect.Type{reflect.TypeOf((func(ConnectionState))(nil))}, []reflect.Type{reflect.TypeOf((func())(nil))})
	assertMethod(t, wsType, "WatchAll", []reflect.Type{ctxType}, []reflect.Type{reflect.TypeOf((*QuerySetSubscription)(nil)), errorType()})
	if _, ok := wsType.MethodByName("Watch"); ok {
		t.Fatal("Watch must not become the primary realtime verb; use Subscribe and keep WatchAll as an advanced helper")
	}
}

func TestPublicAPISurfaceDoesNotExportReconnectOrTimeoutErrors(t *testing.T) {
	for _, name := range exportedDecls(t) {
		switch name {
		case "ReconnectError", "TimeoutError":
			t.Fatalf("%s must not be exported; use internal reconnect errors and context.DeadlineExceeded", name)
		}
	}
}

func TestPublicAPISurfaceKeepsSubscriberIDOpaque(t *testing.T) {
	for _, subscriberType := range []reflect.Type{
		reflect.TypeOf(SubscriberID{}),
		reflect.TypeOf((*SubscriberID)(nil)),
	} {
		if _, ok := subscriberType.MethodByName("QueryID"); ok {
			t.Fatalf("%s must not expose QueryID; raw protocol query ids belong in baseclient/internal layers", subscriberType)
		}
	}
}

func TestPublicAPISurfaceDoesNotExportBaseClient(t *testing.T) {
	for _, name := range exportedDecls(t) {
		switch name {
		case "BaseConvexClient", "NewBaseConvexClient", "WithWebSocketBaseClient":
			t.Fatalf("%s belongs in package baseclient, not the root SDK package", name)
		}
	}
}

func TestPublicAPISurfaceDoesNotExportRawSyncProtocol(t *testing.T) {
	for _, name := range exportedDecls(t) {
		switch name {
		case "ActionMessage",
			"ActionResponseMessage",
			"AuthErrorMessage",
			"AuthenticateMessage",
			"ClientMessage",
			"ConnectMessage",
			"DecodeClientMessage",
			"DecodeServerMessage",
			"EncodeClientMessage",
			"EncodeServerMessage",
			"EventMessage",
			"FatalErrorMessage",
			"IdentityVersion",
			"ModifyQuerySetMessage",
			"MutationMessage",
			"MutationResponseMessage",
			"OptionalString",
			"OptionalValue",
			"PingMessage",
			"QueryFailed",
			"QueryID",
			"QueryRemoved",
			"QuerySetAdd",
			"QuerySetModification",
			"QuerySetRemove",
			"QuerySetVersion",
			"QueryUpdated",
			"ServerMessage",
			"SessionRequestSeqNumber",
			"StateModification",
			"StateVersion",
			"SyncTimestamp",
			"TransitionChunkMessage",
			"TransitionMessage",
			"WebSocketConn",
			"WebSocketDialer",
			"WebSocketManager",
			"WebSocketManagerOption":
				t.Fatalf("%s is raw sync protocol or transport API; expose it from baseclient/internal layers, not root", name)
		}
	}
}

func TestPublicAPISurfaceExportedDeclarationsAreReviewed(t *testing.T) {
	want := []string{
		"ActionKind",
		"ActionReference",
		"AdminAuthToken",
		"ArrayKind",
		"ArrayValue",
		"AuthToken",
		"AuthTokenFetcher",
		"BooleanKind",
		"BooleanValue",
		"Bytes",
		"BytesKind",
		"BytesValue",
		"CanonicalizedModulePath",
		"CanonicalizedUDFPath",
		"Client",
		"ConnectionPhase",
		"ConnectionPhaseConnected",
		"ConnectionPhaseConnecting",
		"ConnectionPhaseDisconnected",
		"ConnectionPhaseReconnecting",
		"ConnectionState",
		"ConvexError",
		"ConvexErrorResult",
		"DecodeJSON",
		"DecodeValue",
		"EncodeJSON",
		"EncodeValue",
		"ErrSubscriptionClosed",
		"ErrorMessageResult",
		"Float64Kind",
		"Float64Value",
		"FunctionError",
		"FunctionKind",
		"FunctionName",
		"FunctionResult",
		"HTTPClient",
		"HTTPError",
		"Int64",
		"Int64Kind",
		"Int64Value",
		"ModulePath",
		"MustObjectValue",
		"MutationKind",
		"MutationOption",
		"MutationReference",
		"NewActionReference",
		"NewClient",
		"NewHTTPClient",
		"NewMutationReference",
		"NewQueryReference",
		"NewWebSocketClient",
		"NoAuthToken",
		"NullKind",
		"NullValue",
		"Number",
		"ObjectKind",
		"ObjectValue",
		"OptimisticLocalStore",
		"OptimisticQueryResult",
		"OptimisticUpdate",
		"Option",
		"ParseFunctionName",
		"ParseFunctionPath",
		"ParseModulePath",
		"ParseUDFPath",
		"ParseValueJSON",
		"QueryKind",
		"QueryReference",
		"QueryResultEntry",
		"QueryResults",
		"QuerySetSubscription",
		"QuerySubscription",
		"StringKind",
		"StringValue",
		"SubscriberID",
		"SyncAuthError",
		"SyncMutationOption",
		"SyncUserIdentityAttributes",
		"TypedQuerySubscription",
		"UDFPath",
		"UserAuthToken",
		"UserIdentityAttributes",
		"UserTokenFetcher",
		"ValidateIdentifier",
		"Value",
		"ValueFromGo",
		"ValueKind",
		"ValueResult",
		"Version",
		"WebSocketClient",
		"WebSocketOption",
		"WithAdminAuth",
		"WithAuth",
		"WithClientID",
		"WithHTTPClient",
		"WithOptimisticUpdate",
		"WithSkipDeploymentURLCheck",
		"WithSkipMutationQueue",
		"WithWebSocketClientID",
		"WithWebSocketInactivityTimeout",
		"WithWebSocketReconnectBackoff",
	}
	got := exportedDecls(t)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("exported declarations changed; review public API before updating this guard\ngot:  %#v\nwant: %#v", got, want)
	}
}

func assertMethod(t *testing.T, receiver reflect.Type, name string, inputs, outputs []reflect.Type) {
	t.Helper()
	method, ok := receiver.MethodByName(name)
	if !ok {
		t.Fatalf("%s missing method %s", receiver, name)
	}
	methodType := method.Type
	if methodType.NumIn() != len(inputs)+1 {
		t.Fatalf("%s.%s input count mismatch: got %d want %d", receiver, name, methodType.NumIn()-1, len(inputs))
	}
	for i, want := range inputs {
		if got := methodType.In(i + 1); got != want {
			t.Fatalf("%s.%s input %d mismatch: got %s want %s", receiver, name, i, got, want)
		}
	}
	if methodType.NumOut() != len(outputs) {
		t.Fatalf("%s.%s output count mismatch: got %d want %d", receiver, name, methodType.NumOut(), len(outputs))
	}
	for i, want := range outputs {
		if got := methodType.Out(i); got != want {
			t.Fatalf("%s.%s output %d mismatch: got %s want %s", receiver, name, i, got, want)
		}
	}
}

func exportedDecls(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var names []string
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				if fn.Recv == nil && ast.IsExported(fn.Name.Name) {
					names = append(names, fn.Name.Name)
				}
				continue
			}
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if ast.IsExported(spec.Name.Name) {
						names = append(names, spec.Name.Name)
					}
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if ast.IsExported(name.Name) {
							names = append(names, name.Name)
						}
					}
				}
			}
		}
	}
	sort.Strings(names)
	return names
}

func stringType() reflect.Type {
	return reflect.TypeOf("")
}

func errorType() reflect.Type {
	return reflect.TypeOf((*error)(nil)).Elem()
}
