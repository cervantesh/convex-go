package repohealth

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestKeyPublicExportsKeepDocComments(t *testing.T) {
	cases := []struct {
		file     string
		receiver string
		name     string
	}{
		{file: "connection_state.go", name: "ConnectionPhase"},
		{file: "connection_state.go", name: "ConnectionState"},
		{file: "client_connection_state.go", receiver: "Client", name: "ConnectionState"},
		{file: "client_connection_state.go", receiver: "Client", name: "SubscribeToConnectionState"},
		{file: "subscription_client.go", name: "WebSocketClient"},
		{file: "subscription_client.go", name: "NewWebSocketClient"},
		{file: "subscription_client.go", receiver: "WebSocketClient", name: "Subscribe"},
		{file: "subscription_client.go", receiver: "WebSocketClient", name: "QueryValue"},
		{file: "subscription_client.go", receiver: "WebSocketClient", name: "Query"},
		{file: "subscription_client.go", receiver: "WebSocketClient", name: "Mutation"},
		{file: "subscription_client.go", receiver: "WebSocketClient", name: "WatchAll"},
		{file: "subscription_client.go", receiver: "WebSocketClient", name: "Close"},
		{file: "subscription_client.go", receiver: "WebSocketClient", name: "SetAuth"},
		{file: "subscription_client.go", receiver: "WebSocketClient", name: "SetAuthContext"},
		{file: "subscription_client.go", receiver: "WebSocketClient", name: "SetAuthCallback"},
		{file: "subscription_client.go", receiver: "WebSocketClient", name: "SetAuthCallbackContext"},
		{file: "subscription_client.go", receiver: "WebSocketClient", name: "SetAdminAuth"},
		{file: "subscription_client.go", receiver: "WebSocketClient", name: "SetAdminAuthContext"},
		{file: "subscription_client.go", receiver: "WebSocketClient", name: "ClearAuth"},
		{file: "subscription_client.go", receiver: "WebSocketClient", name: "ClearAuthContext"},
		{file: "subscription_connection_state.go", receiver: "WebSocketClient", name: "ConnectionState"},
		{file: "subscription_connection_state.go", receiver: "WebSocketClient", name: "SubscribeToConnectionState"},
		{file: "subscription_options.go", name: "ErrSubscriptionClosed"},
		{file: "subscription_options.go", name: "WebSocketOption"},
		{file: "subscription_options.go", name: "WithWebSocketClientID"},
		{file: "subscription_options.go", name: "WithWebSocketReconnectBackoff"},
		{file: "subscription_options.go", name: "WithWebSocketInactivityTimeout"},
		{file: "subscription_handles.go", name: "QuerySubscription"},
		{file: "subscription_handles.go", receiver: "QuerySubscription", name: "ID"},
		{file: "subscription_handles.go", receiver: "QuerySubscription", name: "Next"},
		{file: "subscription_handles.go", receiver: "QuerySubscription", name: "Unsubscribe"},
		{file: "subscription_handles.go", receiver: "QuerySubscription", name: "Close"},
		{file: "subscription_handles.go", name: "QuerySetSubscription"},
		{file: "subscription_handles.go", receiver: "QuerySetSubscription", name: "Next"},
		{file: "subscription_handles.go", receiver: "QuerySetSubscription", name: "Close"},
	}
	for _, tc := range cases {
		assertExportHasDocComment(t, tc.file, tc.receiver, tc.name)
	}
}

func assertExportHasDocComment(t *testing.T, relPath, receiver, name string) {
	t.Helper()
	path := filepath.Join(repoRoot(t), filepath.FromSlash(relPath))
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", relPath, err)
	}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.Name != name {
				continue
			}
			if exportReceiverName(d.Recv) != receiver {
				continue
			}
			if d.Doc == nil || len(d.Doc.List) == 0 {
				t.Fatalf("%s %s must keep a doc comment", relPath, qualifiedExportName(receiver, name))
			}
			return
		case *ast.GenDecl:
			if receiver != "" {
				continue
			}
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.Name == name {
						if !hasSpecDoc(d, s.Doc) {
							t.Fatalf("%s %s must keep a doc comment", relPath, name)
						}
						return
					}
				case *ast.ValueSpec:
					for _, ident := range s.Names {
						if ident.Name == name {
							if !hasSpecDoc(d, s.Doc) {
								t.Fatalf("%s %s must keep a doc comment", relPath, name)
							}
							return
						}
					}
				}
			}
		}
	}
	t.Fatalf("did not find %s in %s", qualifiedExportName(receiver, name), relPath)
}

func exportReceiverName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	switch expr := recv.List[0].Type.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		if ident, ok := expr.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

func hasSpecDoc(decl *ast.GenDecl, doc *ast.CommentGroup) bool {
	return doc != nil && len(doc.List) > 0 || decl.Doc != nil && len(decl.Doc.List) > 0
}

func qualifiedExportName(receiver, name string) string {
	if receiver == "" {
		return name
	}
	return receiver + "." + name
}
