//go:build ze_web

// Design: docs/architecture/cli/command-completion.md -- root completion parity.

package hub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	configcli "github.com/ze-software/ze/internal/component/config/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zeweb "github.com/ze-software/ze/internal/component/web"
)

// TestWebTopRunCompletesLiveArguments compares ordinary and root-selected input
// through authenticated HTTP, the real command tree and live argument RPC.
func TestWebTopRunCompletesLiveArguments(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	calls := 0
	var expectedArgs []string
	require.NoError(t, server.Dispatcher().RegisterWithOptions("show parity sample", func(*pluginserver.CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		t.Fatal("completion executed the target")
		return nil, nil
	}, "Read a sample", pluginserver.RegisterOptions{Complete: func(_ *pluginserver.Server, args []string) []pluginserver.Completion {
		calls++
		require.Equal(t, expectedArgs, args)
		return []pluginserver.Completion{{Value: "private-value", ShortHelp: "Live sample"}}
	}}))
	tree := &command.Node{Children: map[string]*command.Node{"show": {Children: map[string]*command.Node{"parity": {Children: map[string]*command.Node{"sample": {}}}}}}}
	source := webCompletionSource(tree, server.Dispatcher().Registry().VisibleCommandEntries, server.WithArgCompletion, serverDispatcher(server, "web"))
	completer, mgr, schema := newWebTopCompletionEditor(t)
	handler := zeweb.HandleCLICompleteWithSource(completer, source, serverOperationalCommand(server), mgr, schema)
	store := zeweb.NewSessionStore(func() ([]authz.UserConfig, error) { return nil, nil })
	authenticated := zeweb.AuthMiddlewareWithAudit(store, completionAuthenticator{&completionPolicy{t: t}}, func(http.ResponseWriter, *http.Request) {}, handler, nil)
	for _, tc := range []struct {
		input string
		args  []string
		mode  string
	}{
		{input: "run show parity sample pr", mode: "config"},
		{input: "top run show parity sample pr", mode: "config"},
		{input: "top top run show parity sample ", mode: "config"},
		{input: `top run show parity sample "two words|one" pr`, args: []string{"two words|one"}, mode: "config"},
		{input: `top top run show parity sample "" pr`, args: []string{""}, mode: "config"},
		{input: `top run show parity sample "pr`, mode: "config"},
		{input: `top run show parity sample ""`, mode: "config"},
		{input: "top run show parity sample pr", mode: "operational"},
	} {
		expectedArgs = tc.args
		query := url.Values{"input": {tc.input}, "mode": {tc.mode}, "path": {"system"}}
		r := httptest.NewRequest(http.MethodGet, "/cli/complete?"+query.Encode(), http.NoBody)
		r.RemoteAddr = "203.0.113.9:4567"
		r.SetBasicAuth("operator", "synthetic")
		w := httptest.NewRecorder()
		authenticated.ServeHTTP(w, r)
		require.Equal(t, http.StatusOK, w.Code, tc.input)
		require.JSONEq(t, `[{"text":"private-value","description":"Live sample","type":"value"}]`, w.Body.String(), tc.input)
	}
	require.Equal(t, 8, calls)
}

// TestWebTopRunExplainsRawArguments selects the same operational declaration
// behind run and repeated top while leaving incomplete quotes unexplained.
func TestWebTopRunExplainsRawArguments(t *testing.T) {
	const explanation = "Read the selected sample."
	tree := &command.Node{Children: map[string]*command.Node{
		"show": {Children: map[string]*command.Node{
			"parity": {Children: map[string]*command.Node{
				"sample": {WireMethod: "show parity sample", Description: explanation},
			}},
		}},
	}}
	completer, mgr, schema := newWebTopCompletionEditor(t)
	handler := zeweb.HandleCLICompleteWithCommandCompleter(completer, cli.NewCommandCompleter(tree), mgr, schema)
	store := zeweb.NewSessionStore(func() ([]authz.UserConfig, error) { return nil, nil })
	authenticated := zeweb.AuthMiddlewareWithAudit(store, completionAuthenticator{&completionPolicy{t: t}}, func(http.ResponseWriter, *http.Request) {}, handler, nil)
	for _, tc := range []struct {
		input string
		found bool
	}{
		{"run show parity sample", true},
		{`top run show parity sample "two words|one"`, true},
		{`top top run show parity sample "" `, true},
		{`top run show parity sample "unfinished`, false},
		{"top set system host", false},
	} {
		query := url.Values{"input": {tc.input}, "mode": {"config"}, "path": {"system"}, "explain": {"true"}}
		r := httptest.NewRequest(http.MethodGet, "/cli/complete?"+query.Encode(), http.NoBody)
		r.RemoteAddr = "203.0.113.9:4567"
		r.SetBasicAuth("operator", "synthetic")
		w := httptest.NewRecorder()
		authenticated.ServeHTTP(w, r)
		require.Equal(t, http.StatusOK, w.Code, tc.input)
		var answer struct {
			Explanation string `json:"explanation"`
			Found       bool   `json:"found"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &answer))
		require.Equal(t, tc.found, answer.Found, tc.input)
		if tc.found {
			require.Equal(t, explanation, answer.Explanation, tc.input)
		} else {
			require.Empty(t, answer.Explanation, tc.input)
		}
	}
}

// TestWebTopSetCompletesRootFromNestedPosition keeps ordinary set relative
// while root-selected requests use the same authenticated candidate editor.
func TestWebTopSetCompletesRootFromNestedPosition(t *testing.T) {
	completer, mgr, schema := newWebTopCompletionEditor(t)
	handler := zeweb.HandleCLICompleteWithCommandCompleter(completer, nil, mgr, schema)
	store := zeweb.NewSessionStore(func() ([]authz.UserConfig, error) { return nil, nil })
	authenticated := zeweb.AuthMiddlewareWithAudit(store, completionAuthenticator{&completionPolicy{t: t}}, func(http.ResponseWriter, *http.Request) {}, handler, nil)
	for _, tc := range []struct {
		input, expected, mode string
	}{
		{"set ho", "host", "config"},
		{"top set syst", "system", "config"},
		{"top top set syst", "system", "config"},
		{"top set syst", "system", "operational"},
	} {
		query := url.Values{"input": {tc.input}, "mode": {tc.mode}, "path": {"system"}}
		r := httptest.NewRequest(http.MethodGet, "/cli/complete?"+query.Encode(), http.NoBody)
		r.RemoteAddr = "203.0.113.9:4567"
		r.SetBasicAuth("operator", "synthetic")
		w := httptest.NewRecorder()
		authenticated.ServeHTTP(w, r)
		require.Equal(t, http.StatusOK, w.Code, tc.input)
		var items []struct {
			Text string `json:"text"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
		require.Len(t, items, 1, tc.input)
		require.Equal(t, tc.expected, items[0].Text, tc.input)
	}
}

// TestWebTopRunCompletionRetainsDenial refuses both target-command and query
// policy denial before the live producer can reveal its private value.
func TestWebTopRunCompletionRetainsDenial(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	require.NoError(t, server.Dispatcher().RegisterWithOptions("show parity sample", func(*pluginserver.CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
		t.Fatal("completion executed the target")
		return nil, nil
	}, "Read a sample", pluginserver.RegisterOptions{Complete: func(*pluginserver.Server, []string) []pluginserver.Completion {
		t.Fatal("denied completion reached the live producer")
		return nil
	}}))
	tree := &command.Node{Children: map[string]*command.Node{"show": {Children: map[string]*command.Node{"parity": {Children: map[string]*command.Node{"sample": {}}}}}}}
	source := webCompletionSource(tree, server.Dispatcher().Registry().VisibleCommandEntries, server.WithArgCompletion, serverDispatcher(server, "web"))
	completer, mgr, schema := newWebTopCompletionEditor(t)
	handler := zeweb.HandleCLICompleteWithSource(completer, source, serverOperationalCommand(server), mgr, schema)
	store := zeweb.NewSessionStore(func() ([]authz.UserConfig, error) { return nil, nil })
	authenticated := zeweb.AuthMiddlewareWithAudit(store, completionAuthenticator{&completionPolicy{t: t}}, func(http.ResponseWriter, *http.Request) {}, handler, nil)
	for _, username := range []string{"reader", "rpc-denied"} {
		query := url.Values{"input": {"top top run show parity sample pr"}, "mode": {"config"}, "path": {"system"}}
		r := httptest.NewRequest(http.MethodGet, "/cli/complete?"+query.Encode(), http.NoBody)
		r.RemoteAddr = "203.0.113.9:4567"
		r.SetBasicAuth(username, "synthetic")
		w := httptest.NewRecorder()
		authenticated.ServeHTTP(w, r)
		require.Equal(t, http.StatusForbidden, w.Code, username)
		require.NotContains(t, w.Body.String(), "private-value")
	}
}

func newWebTopCompletionEditor(t *testing.T) (*cli.Completer, *zeweb.EditorManager, *config.Schema) {
	t.Helper()
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	completer, err := cli.NewCompleter()
	require.NoError(t, err)
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.WriteFile("config.conf", []byte("system { host completion-test; }\n"), 0o600))
	mgr := zeweb.NewEditorManager(store, "config.conf", schema, newEditorFactory(configcli.ValidateContent), newEditSessionFactory())
	t.Cleanup(func() { require.NoError(t, mgr.Close()) })
	require.NoError(t, mgr.SetValue("operator", []string{"system"}, "host", "completion-candidate"))
	return completer, mgr, schema
}
