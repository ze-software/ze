package fixture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/core/env"
)

// TestDocumentTooWideOperatorLogsInWithNoStore proves the document-too-wide
// fixture needs no credential store, so it owes no `ze init`.
//
// VALIDATES: with only fixture10OperatorEnvironment applied and its store
//
//	directory empty, the CLI's own credential resolution
//	(sshclient.LoadCredentialsWithFlags, the call `ze cli --user admin`
//	makes) answers the fixture daemon's host and SSH port, the user
//	admin and the password the daemon's config hashes.
//
// PREVENTS: a fixture that pays for `ze init` (netlink interface discovery and
//
//	a bcrypt hash) inside its 15-second process bound for a store
//	whose every value the environment already supplies; that bound
//	was overrun on a loaded host on 2026-10-27.
//
// MUTATION: drop envSSHPassword from fixture10OperatorEnvironment and the
// resolution fails with "no password source for user"; drop envSSHPort and it
// answers port 2222, not the daemon's.
func TestDocumentTooWideOperatorLogsInWithNoStore(t *testing.T) {
	const sshPort = 40179
	storeDir := t.TempDir()

	// Registered first so it runs last: the environment is restored before
	// the cache is rebuilt from it.
	t.Cleanup(env.ResetCache)
	for name, value := range fixture10OperatorEnvironment(storeDir, sshPort) {
		t.Setenv(name, value)
	}
	env.ResetCache()

	credentials, err := sshclient.LoadCredentialsWithFlags("admin")
	if err != nil {
		t.Fatalf("the operator environment alone must log in: %v", err)
	}
	if credentials.Host != addrLoopback {
		t.Errorf("host = %q, want %q", credentials.Host, addrLoopback)
	}
	if credentials.Port != strconv.Itoa(sshPort) {
		t.Errorf("port = %q, want %d", credentials.Port, sshPort)
	}
	if credentials.Username != "admin" {
		t.Errorf("user = %q, want admin", credentials.Username)
	}
	if credentials.Auth != valueTestPassword {
		t.Errorf("password = %q, want the configured %q", credentials.Auth, valueTestPassword)
	}
}

// TestDocumentTooWideRunsNoInit proves the document-too-wide driver starts no
// `ze init`, the wait that overran fixture10ProcessTimeout on 2026-10-27
// ("initialize isolated CLI state: exit=124").
//
// VALIDATES: the body of fixture10DocumentTooWide passes no "init" string
//
//	literal to any call, so no child it starts is `ze init`.
//
// PREVENTS: the init step returning, and with it interface discovery whose
//
//	duration follows the host's link count and link churn rather
//	than anything the fixture tests.
//
// MUTATION: restore `fixture10Run(ctx, ..., initInput, "ze", "init")` in
// fixture10DocumentTooWide and this test fails naming the call's position.
func TestDocumentTooWideRunsNoInit(t *testing.T) {
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, "plugin_fixture_10_process.go", nil, 0)
	if err != nil {
		t.Fatalf("parse the fixture source: %v", err)
	}
	var body *ast.BlockStmt
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == "fixture10DocumentTooWide" {
			body = function.Body
		}
	}
	if body == nil {
		t.Fatal("fixture10DocumentTooWide is not declared in plugin_fixture_10_process.go")
	}
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		for _, argument := range call.Args {
			literal, isLiteral := argument.(*ast.BasicLit)
			if !isLiteral || literal.Kind != token.STRING {
				continue
			}
			if value, unquoteErr := strconv.Unquote(literal.Value); unquoteErr == nil && value == argInit {
				t.Errorf("%s starts `ze init`; the fixture's CLI needs no store", files.Position(call.Pos()))
			}
		}
		return true
	})
}
