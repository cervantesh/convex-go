package baseclient_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/cervantesh/convex-go/baseclient"
)

var _ func() *baseclient.Client = baseclient.New

func requireType[T any](T) {}

func TestPublicBaseClientExportedDeclarationsAreReviewed(t *testing.T) {
	want := []string{
		"ActionMessage",
		"ActionResponseMessage",
		"AdminAuthToken",
		"AuthErrorMessage",
		"AuthToken",
		"AuthTokenAdmin",
		"AuthTokenFetcher",
		"AuthTokenNone",
		"AuthTokenType",
		"AuthTokenUser",
		"AuthenticateMessage",
		"CanonicalizedUDFPath",
		"Client",
		"ClientMessage",
		"ConvexError",
		"ConvexErrorResult",
		"DecodeClientMessage",
		"DecodeServerMessage",
		"EncodeClientMessage",
		"EncodeServerMessage",
		"ErrorMessageResult",
		"FatalErrorMessage",
		"FunctionResult",
		"IdentityVersion",
		"Int64",
		"Int64Value",
		"ModifyQuerySetMessage",
		"ModulePath",
		"MutationMessage",
		"MutationResponseMessage",
		"New",
		"NoAuthToken",
		"NullValue",
		"Number",
		"OptimisticLocalStore",
		"OptimisticQueryResult",
		"OptimisticUpdate",
		"OptionalString",
		"OptionalValue",
		"ParseFunctionPath",
		"ParseUDFPath",
		"PingMessage",
		"QueryFailed",
		"QueryID",
		"QueryRemoved",
		"QueryResultEntry",
		"QueryResults",
		"QuerySetAdd",
		"QuerySetModification",
		"QuerySetRemove",
		"QuerySetVersion",
		"QueryUpdated",
		"RequestID",
		"ServerMessage",
		"StateModification",
		"StateVersion",
		"StringValue",
		"SubscriberID",
		"SyncAuthError",
		"SyncMutationOption",
		"SyncTimestamp",
		"SyncUserIdentityAttributes",
		"TransitionChunkMessage",
		"TransitionMessage",
		"UDFPath",
		"UserAuthToken",
		"Value",
		"ValueFromGo",
		"ValueResult",
		"WithOptimisticUpdate",
	}
	got := exportedDecls(t)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("baseclient exported declarations changed; review advanced public API before updating this guard\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestPublicBaseClientSurface(t *testing.T) {
	client := baseclient.New()
	requireType[*baseclient.Client](client)

	subscriber, err := client.Subscribe("messages:list", map[string]any{"room": "general"})
	if err != nil {
		t.Fatal(err)
	}
	requireType[baseclient.SubscriberID](subscriber)

	msg := client.PopNextMessage()
	requireType[baseclient.ClientMessage](msg)
	encoded, err := baseclient.EncodeClientMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := baseclient.DecodeClientMessage(encoded)
	if err != nil {
		t.Fatal(err)
	}
	requireType[baseclient.ClientMessage](decoded)

	results, err := client.ReceiveMessage(baseclient.PingMessage{})
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Fatalf("ping should not publish query results: %#v", results)
	}

	requireType[baseclient.QueryResults](client.LatestResults())
	mutationID, err := client.Mutation("messages:send", nil)
	if err != nil {
		t.Fatal(err)
	}
	if mutationID != 0 {
		t.Fatalf("first request id should start at zero, got %d", mutationID)
	}
	actionID, err := client.Action("tasks:run", nil)
	if err != nil {
		t.Fatal(err)
	}
	if actionID != 1 {
		t.Fatalf("second request id should be one, got %d", actionID)
	}
	if err := client.Unsubscribe(subscriber); err != nil {
		t.Fatal(err)
	}

	requireType[baseclient.FunctionResult](baseclient.ValueResult(baseclient.NullValue()))
	serverEncoded, err := baseclient.EncodeServerMessage(baseclient.PingMessage{})
	if err != nil {
		t.Fatal(err)
	}
	serverDecoded, err := baseclient.DecodeServerMessage(serverEncoded)
	if err != nil {
		t.Fatal(err)
	}
	requireType[baseclient.ServerMessage](serverDecoded)
	var _ baseclient.AuthTokenFetcher = func(bool) (baseclient.AuthToken, error) {
		return baseclient.NoAuthToken(), nil
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
