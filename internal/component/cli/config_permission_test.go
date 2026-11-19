// Design: docs/architecture/config/yang-config-design.md -- config editor
// Related: config_permission.go -- the verb table and the gate under test

package cli

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// gateTestLine is a config-mode edit the test editor accepts
// (testValidBGPConfig, TestCommitGrammar).
const (
	gateTestLine  = "set bgp router-id 5.6.7.8"
	gateTestValue = "5.6.7.8"
)

// TestEveryMutatingEditorVerbHasAPermission proves AC-5 and guards R-4: every
// verb dispatchCommandWords routes is either one of the verbs that change
// nothing, or carries the permission string the web terminal authorized it
// with before the table moved. A verb added to the dispatch without either
// fails here, so a new mutating verb cannot arrive ungated.
//
// Method: read the case labels of the verb switch in dispatchCommandWords from
// the source (resolving the cmd* constants, through the command verb registry
// where they point there), then judge each with ConfigPermission.
func TestEveryMutatingEditorVerbHasAPermission(t *testing.T) {
	mutating := map[string]string{
		"set":        "config set",
		"activate":   "config set",
		"delete":     "config delete",
		"deactivate": "config delete",
		"commit":     "config commit",
		"discard":    "config discard",
		"disconnect": "config discard",
		"rollback":   "config rollback",
		"rename":     "config rename",
		"copy":       "config add",
		"insert":     "config add",
		"save":       "config save",
		"load":       "config save",
	}
	// Each of these changes neither the candidate, the store nor the running
	// configuration: navigation, display, help, and leaving the editor.
	unchanging := map[string]bool{
		"exit": true, "quit": true, "help": true, "?": true, "top": true,
		"up": true, "edit": true, "show": true, "option": true, "who": true,
	}

	dispatched := dispatchedEditorVerbs(t)
	require.NotEmpty(t, dispatched, "no case label found in dispatchCommandWords")
	for _, verb := range dispatched {
		permission, gated := ConfigPermission(verb)
		if unchanging[verb] {
			assert.False(t, gated, "verb %q changes nothing and must stay ungated", verb)
			continue
		}
		want, known := mutating[verb]
		if !known {
			t.Errorf("dispatchCommandWords routes %q, which is neither a mutating verb with a "+
				"permission nor a verb that changes nothing: give it an entry in configPermissions "+
				"(config_permission.go) and in this test, or list it here as changing nothing", verb)
			continue
		}
		assert.True(t, gated, "mutating verb %q is ungated", verb)
		assert.Equal(t, want, permission, "verb %q", verb)
	}

	routed := make(map[string]bool, len(dispatched))
	for _, verb := range dispatched {
		routed[verb] = true
	}
	for verb := range mutating {
		assert.True(t, routed[verb], "mutating verb %q is no longer dispatched", verb)
	}
	for verb := range configPermissions {
		assert.True(t, routed[verb], "configPermissions names %q, which dispatchCommandWords does not route", verb)
	}
}

// TestEditorGateRefusesBeforeHandler proves AC-1, AC-2, AC-3 and AC-4 at the
// model: with an authorizer that denies everything, `set`, `top set`,
// `commit now` and the operational-mode `set` shortcut are refused, the
// candidate and the store keep their bytes, and the refusal reads as the
// dispatcher's access-control refusal, names the permission, ends with a fix
// and classifies as pluginserver.ErrUnauthorized.
//
// Method: drive dispatchCommand, and handleEnter for the shortcut, over the
// commit-grammar model with a recording, denying authorizer.
func TestEditorGateRefusesBeforeHandler(t *testing.T) {
	cases := []struct {
		name       string
		line       string
		permission string
	}{
		{"set", gateTestLine, "config set"},
		{"top set", "top " + gateTestLine, "config set"},
		{"top top set", "top top " + gateTestLine, "config set"},
		{"commit", "commit now", "config commit"},
		{"delete", "delete bgp router-id", "config delete"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model, store, configPath := newCommitGrammarModel(t, "")
			var asked []string
			model.SetConfigAuthorizer(func(permission string) bool {
				asked = append(asked, permission)
				return false
			}, "")
			working := model.editor.WorkingContent()
			stored, err := store.ReadFile(configPath)
			require.NoError(t, err)

			_, err = model.dispatchCommand(tc.line)
			requireConfigRefusal(t, err, tc.permission)
			assert.Equal(t, []string{tc.permission}, asked)
			assert.Equal(t, working, model.editor.WorkingContent(), "a refused line changed the candidate")
			after, err := store.ReadFile(configPath)
			require.NoError(t, err)
			assert.Equal(t, stored, after, "a refused line changed the store")
		})
	}

	t.Run("operational shortcut", func(t *testing.T) {
		model, _, _ := newCommitGrammarModel(t, "")
		var asked []string
		model.SetConfigAuthorizer(func(permission string) bool {
			asked = append(asked, permission)
			return false
		}, "")
		working := model.editor.WorkingContent()
		model.SetMode(ModeOperational)
		model.SetInput(gateTestLine)
		next, execute := model.handleEnter()
		updated, ok := next.(Model)
		require.True(t, ok)
		assert.Equal(t, ModeConfig, updated.Mode(), "the shortcut moves the session to config mode")
		require.NotNil(t, execute)
		answer, ok := execute().(commandResultMsg)
		require.True(t, ok)
		requireConfigRefusal(t, answer.err, "config set")
		assert.Equal(t, []string{"config set"}, asked)
		assert.Equal(t, working, updated.editor.WorkingContent(), "a refused shortcut changed the candidate")
	})
}

// TestEditorGateAppendsConfigurationName proves AC-8 at the model: a session on
// a named stored configuration asks with the verb's string, a space, then the
// name, so a deny entry naming the file refuses its commit while `set` passes.
//
// Method: install an authorizer that denies `config commit client-edge.conf`
// only, then set and commit through dispatchCommand.
func TestEditorGateAppendsConfigurationName(t *testing.T) {
	model, store, configPath := newCommitGrammarModel(t, "")
	const denied = "config commit client-edge.conf"
	var asked []string
	model.SetConfigAuthorizer(func(permission string) bool {
		asked = append(asked, permission)
		return permission != denied
	}, "client-edge.conf")
	stored, err := store.ReadFile(configPath)
	require.NoError(t, err)

	_, err = model.dispatchCommand(gateTestLine)
	require.NoError(t, err)
	assert.Contains(t, model.editor.WorkingContent(), gateTestValue)

	_, err = model.dispatchCommand("commit now")
	requireConfigRefusal(t, err, denied)
	assert.Equal(t, []string{"config set client-edge.conf", denied}, asked)
	after, err := store.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, stored, after, "a refused commit changed the store")
}

// TestEditorGateLeavesNavigationUngated proves the AC-1 survivors: navigation
// and display never reach the authorizer, so a read-only user keeps browsing
// config mode.
//
// Method: install an authorizer that fails the test when called, then run each
// navigation line through dispatchCommand.
func TestEditorGateLeavesNavigationUngated(t *testing.T) {
	model, _, _ := newCommitGrammarModel(t, "alice")
	model.SetConfigAuthorizer(func(permission string) bool {
		t.Errorf("navigation asked the authorizer about %q", permission)
		return false
	}, "")
	for _, line := range []string{"show", "edit bgp", "up", "top show", "top", "who", "help"} {
		_, err := model.dispatchCommand(line)
		if err != nil {
			assert.NotErrorIs(t, err, pluginserver.ErrUnauthorized, "line %q", line)
		}
	}
}

// TestEditorWithoutAuthorizerKeepsEveryVerb proves AC-12 at the model: with no
// config authorizer installed (the attached console, `ze config edit -f` and
// `ze config edit --backup`),
// `set` and `commit now` run and the commit reaches the store.
//
// Method: set and commit through dispatchCommand with no authorizer, then read
// the stored file.
func TestEditorWithoutAuthorizerKeepsEveryVerb(t *testing.T) {
	model, store, configPath := newCommitGrammarModel(t, "")
	_, err := model.dispatchCommand(gateTestLine)
	require.NoError(t, err)
	result, err := model.dispatchCommand("commit now")
	require.NoError(t, err)
	assert.NotContains(t, result.statusMessage, "blocked")
	stored, err := store.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(stored), gateTestValue)
}

// requireConfigRefusal checks the editor's access-control refusal: the first
// line opens with the dispatcher's message and names the permission, the last
// line is the fix naming it, and errors.Is finds pluginserver.ErrUnauthorized.
func requireConfigRefusal(t *testing.T, err error, permission string) {
	t.Helper()
	require.Error(t, err)
	assert.True(t, errors.Is(err, pluginserver.ErrUnauthorized), "refusal does not wrap ErrUnauthorized: %v", err)
	lines := strings.Split(err.Error(), "\n")
	assert.Equal(t, "command restricted by access control: "+permission, lines[0])
	last := lines[len(lines)-1]
	assert.True(t, strings.HasPrefix(last, "fix: "), "last line is not a fix: %q", last)
	assert.Contains(t, last, permission)
}

// dispatchedEditorVerbs answers the verbs the switch on cmd in
// dispatchCommandWords routes, read from model_commands.go. A case label is a
// string literal or a constant of this package, which may in turn name a
// constant of internal/component/command (the verb registry).
func dispatchedEditorVerbs(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	local := packageConstants(t, fset, ".")
	registry := packageConstants(t, fset, filepath.Join("..", "command"))

	var resolve func(expr ast.Expr) string
	resolve = func(expr ast.Expr) string {
		switch e := expr.(type) {
		case *ast.BasicLit:
			value, err := strconv.Unquote(e.Value)
			require.NoError(t, err)
			return value
		case *ast.Ident:
			value, ok := local[e.Name]
			require.True(t, ok, "constant %s not found in package cli", e.Name)
			return resolve(value)
		case *ast.SelectorExpr:
			pkg, ok := e.X.(*ast.Ident)
			require.True(t, ok)
			require.Equal(t, "command", pkg.Name, "case label from an unexpected package")
			value, ok := registry[e.Sel.Name]
			require.True(t, ok, "constant command.%s not found", e.Sel.Name)
			return resolve(value)
		}
		t.Fatalf("unexpected case label %T", expr)
		return ""
	}

	file, err := parser.ParseFile(fset, "model_commands.go", nil, 0)
	require.NoError(t, err)
	var verbs []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name != "dispatchCommandWords" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			sw, ok := node.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			tag, ok := sw.Tag.(*ast.Ident)
			if !ok {
				return true
			}
			if tag.Name != "cmd" {
				return true
			}
			for _, stmt := range sw.Body.List {
				clause, ok := stmt.(*ast.CaseClause)
				require.True(t, ok)
				for _, label := range clause.List {
					verbs = append(verbs, resolve(label))
				}
			}
			return false
		})
	}
	return verbs
}

// packageConstants answers every constant declared in the non-test Go files of
// dir, by name, as its declaring expression.
func packageConstants(t *testing.T, fset *token.FileSet, dir string) map[string]ast.Expr {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	constants := map[string]ast.Expr{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		require.NoError(t, err)
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			if gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, ident := range value.Names {
					if i < len(value.Values) {
						constants[ident.Name] = value.Values[i]
					}
				}
			}
		}
	}
	return constants
}
