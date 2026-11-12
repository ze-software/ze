// Design: docs/architecture/web-interface.md -- the web editors refuse a value
// at set and frame every fix for the operator's position
// Related: cli.go -- applyWebSet, commandBehindTop, handleCLICommit
// Related: cli_terminal.go -- executeTerminalConfigMode, the terminal's `top`
// Related: editor.go -- EditorManager.judgeValue

package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/plugins/ospf/yang"
)

// The refused route-refresh line every case below types, and its pieces.
const (
	routeRefreshPath    = "bgp peer peer1 session capability route-refresh"
	routeRefreshProblem = routeRefreshPath + `: "requre" is not an accepted value`
	routeRefreshWhy     = "route-refresh accepts only these values:"
	routeRefreshRootFix = "fix: set " + routeRefreshPath + " require"
	routeRefreshTopFix  = "fix: top set " + routeRefreshPath + " require"
	ospfSourceChoices   = "<connected|static|kernel|bgp|isis>"
)

// webTerminalLine posts one terminal line typed at path (slash-joined, empty
// for the root) through the real terminal handler and decodes its answer.
func webTerminalLine(t *testing.T, handler http.HandlerFunc, path, line string) terminalResponse {
	t.Helper()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, authedRequest(t, http.MethodPost, "/cli/terminal",
		url.Values{"command": {line}, "path": {path}}))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp terminalResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// webBarLine posts one CLI bar line typed at path through the real bar
// handler and answers the response body.
func webBarLine(t *testing.T, handler http.HandlerFunc, path, line string) string {
	t.Helper()
	w := httptest.NewRecorder()
	r := authedRequest(t, http.MethodPost, "/cli", url.Values{"command": {line}, "path": {path}})
	r.Header.Set("HX-Request", "true")
	handler.ServeHTTP(w, r)
	return w.Body.String()
}

// lastLine answers the last non-empty line of text.
func lastLine(text string) string {
	text = strings.TrimRight(text, "\n")
	return text[strings.LastIndexByte(text, '\n')+1:]
}

// newYANGTerminal answers the terminal handler over the real YANG schema.
func newYANGTerminal(t *testing.T) (*EditorManager, http.HandlerFunc) {
	t.Helper()
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	return mgr, HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, tree, nil, nil, nil)
}

// TestWebTerminalSetRefusesEnum drives the terminal `set` of a value outside
// an enumeration, at the root.
//
// VALIDATES: AC-7 -- refused at set with the problem, the accepted values and
// the root fix on the last line; a following `show` holds no `requre`; the
// accepted value is set (the control that proves the path reaches the leaf).
// PREVENTS: the web storing a value only the commit refuses (text 6).
func TestWebTerminalSetRefusesEnum(t *testing.T) {
	_, terminal := newYANGTerminal(t)

	refused := webTerminalLine(t, terminal, "", "set "+routeRefreshPath+" requre")
	assert.True(t, strings.HasPrefix(refused.Output, "error: "+routeRefreshProblem), "output %q", refused.Output)
	assert.Contains(t, refused.Output, "\n"+routeRefreshWhy+"\n  enable")
	assert.Equal(t, routeRefreshRootFix, lastLine(refused.Output))
	assert.NotContains(t, refused.Output, "invalid enum")
	assert.NotContains(t, webTerminalLine(t, terminal, "", "show").Output, "requre", "a refused value was stored")

	accepted := webTerminalLine(t, terminal, "", "set "+routeRefreshPath+" require")
	require.False(t, strings.HasPrefix(accepted.Output, "error"), "the accepted value was refused: %q", accepted.Output)
	assert.Contains(t, webTerminalLine(t, terminal, "", "show").Output, "route-refresh require")
}

// TestWebTerminalSetFixKeepsTheTypedContext types the same refusal inside a
// context.
//
// VALIDATES: AC-6 on the web terminal -- the fix is the `set` the operator
// typed there, corrected, in its relative form.
// PREVENTS: a fix naming a path the context-relative `set` cannot take.
func TestWebTerminalSetFixKeepsTheTypedContext(t *testing.T) {
	_, terminal := newYANGTerminal(t)

	refused := webTerminalLine(t, terminal, "bgp/peer/peer1", "set session capability route-refresh requre")
	assert.Equal(t, "fix: set session capability route-refresh require", lastLine(refused.Output))
	assert.Equal(t, "bgp/peer/peer1", refused.Path)
}

// TestWebTerminalKeyRefusalNamesTheWholeLine creates an entry under a key the
// key leaf refuses, on the way to a leaf.
//
// VALIDATES: AC-27 -- refused where the entry would be created, with the
// accepted keys and the whole line as the fix, the key replaced; nothing
// stored.
// PREVENTS: a key refusal whose fix drops the rest of the line typed.
func TestWebTerminalKeyRefusalNamesTheWholeLine(t *testing.T) {
	_, terminal := newYANGTerminal(t)

	refused := webTerminalLine(t, terminal, "", "set ospf redistribute bogus nssa-propagate true")
	assert.True(t, strings.HasPrefix(refused.Output, `error: ospf redistribute bogus: "bogus" is not an accepted value`), "output %q", refused.Output)
	assert.Equal(t, "fix: set ospf redistribute "+ospfSourceChoices+" nssa-propagate true", lastLine(refused.Output))
	assert.NotContains(t, webTerminalLine(t, terminal, "", "show").Output, "bogus", "an entry was created under the refused key")
}

// TestWebTerminalKeyLeafFormStoresKeyedEntry types `set <list> <key-leaf>
// <key>`, the form that names an entry by its key leaf.
//
// VALIDATES: AC-33 -- `bogus` refused with the `set` typed as its fix and
// nothing stored; `connected` creates the keyed entry `redistribute connected`
// and no anonymous entry.
// PREVENTS: the mis-store of journal guard-added-to-one-half-of-a-pair: the
// web stored the key as a leaf of an anonymous `default` entry.
func TestWebTerminalKeyLeafFormStoresKeyedEntry(t *testing.T) {
	mgr, terminal := newYANGTerminal(t)

	refused := webTerminalLine(t, terminal, "", "set ospf redistribute source bogus")
	assert.Equal(t, "fix: set ospf redistribute source "+ospfSourceChoices, lastLine(refused.Output))

	created := webTerminalLine(t, terminal, "", "set ospf redistribute source connected")
	assert.Equal(t, "created redistribute connected", created.Output)
	// The tree, not its printed form, is judged: Serialize writes a keyed list
	// whose entries hold nothing as `redistribute { connected }`.
	ospf := mgr.Tree("testuser").GetContainer("ospf")
	require.NotNil(t, ospf, "the set created no ospf container")
	assert.Equal(t, []string{"connected"}, ospf.ListKeys("redistribute"),
		"the entry is stored under its key, and no anonymous entry beside it")
}

// TestWebTerminalInEntryKeyLeafRefused sets an entry's own key leaf from
// inside the path that names the entry.
//
// VALIDATES: AC-33 -- the in-entry key-leaf refusal; nothing stored.
// PREVENTS: the key written as a second value beside the entry's key.
func TestWebTerminalInEntryKeyLeafRefused(t *testing.T) {
	mgr, terminal := newYANGTerminal(t)
	require.False(t, strings.HasPrefix(webTerminalLine(t, terminal, "", "set ospf redistribute source connected").Output, "error"))
	before := config.Serialize(mgr.Tree("testuser"), mgr.schema)

	refused := webTerminalLine(t, terminal, "", "set ospf redistribute connected source static")
	assert.Contains(t, refused.Output, "source is the key of list redistribute and cannot be set inside an entry")
	assert.Equal(t, "fix: rename ospf redistribute connected to <new-key>", lastLine(refused.Output))
	assert.Equal(t, before, config.Serialize(mgr.Tree("testuser"), mgr.schema), "the refused line changed the tree")
}

// TestWebTopCommandKeepsPosition types `top <command>` at position
// `bgp peer peer1` on the web terminal.
//
// VALIDATES: AC-50 -- each line answers as on the SSH editor: the position
// stays `bgp peer peer1` except after `top edit ospf` (ospf) and `top` alone
// (root); `top show` answers the root; a failing line answers the inner
// refusal.
// PREVENTS: the terminal's `top` keeping the emptied position, or ignoring
// the rest of the line.
func TestWebTopCommandKeepsPosition(t *testing.T) {
	mgr, terminal := newYANGTerminal(t)
	const at = "bgp/peer/peer1"

	set := webTerminalLine(t, terminal, at, "top set ospf router-id 192.0.2.9")
	assert.Equal(t, at, set.Path)
	assert.Equal(t, "set ospf router-id 192.0.2.9", set.Output)
	assert.Contains(t, config.Serialize(mgr.Tree("testuser"), mgr.schema), "192.0.2.9", "the set did not run at the root")

	shown := webTerminalLine(t, terminal, at, "top show")
	assert.Equal(t, at, shown.Path)
	assert.Equal(t, webTerminalLine(t, terminal, "", "show").Output, shown.Output, "top show did not answer the root")

	assert.Equal(t, "ospf", webTerminalLine(t, terminal, at, "top edit ospf").Path)
	assert.Empty(t, webTerminalLine(t, terminal, at, "top").Path)

	failing := webTerminalLine(t, terminal, at, "top set nosuchleaf 1")
	assert.Equal(t, at, failing.Path)
	assert.True(t, strings.HasPrefix(failing.Output, "error"), "output %q", failing.Output)
}

// TestWebTopKeepsTheOperatorForTheFix types a refused `set` behind `top`
// inside a context.
//
// VALIDATES: AC-53 on the web terminal -- the fix is framed for the
// operator's position, not the root `top` runs the command at.
// PREVENTS: the `top` recursion resetting cliCommand.Operator, which prints a
// root-form `set` the operator cannot type where they stand.
func TestWebTopKeepsTheOperatorForTheFix(t *testing.T) {
	_, terminal := newYANGTerminal(t)

	refused := webTerminalLine(t, terminal, "ospf", "top set "+routeRefreshPath+" requre")
	assert.Equal(t, routeRefreshTopFix, lastLine(refused.Output))
	assert.Equal(t, "ospf", refused.Path)
}

// TestTopIsAuthorizedAsItsCommand reads the RBAC command each web editor asks
// for a `top` line.
//
// VALIDATES: `top <command>` is authorized as the command it runs.
// PREVENTS: `top set ...` carrying a mutation past the profile gate, which
// sees only the verb `top` and gates nothing.
func TestTopIsAuthorizedAsItsCommand(t *testing.T) {
	assert.Equal(t, webCommandConfigSet, terminalAuthCommand(parseCLICommand("top set ospf router-id 192.0.2.9")))
	assert.Equal(t, webCommandConfigCommit, terminalAuthCommand(parseCLICommand("top top commit now")))
	assert.Empty(t, terminalAuthCommand(parseCLICommand("top")))
}

// TestCLIBarTopRunsTheRestOfTheLine types `top set` on the CLI bar inside a
// context.
//
// VALIDATES: AC-50 on the CLI bar -- the rest of the line runs at the root.
// PREVENTS: text 14: handleCLITop ignored its arguments and dropped the line.
func TestCLIBarTopRunsTheRestOfTheLine(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	bar := HandleCLICommandWithAuthorizerAndAudit(mgr, schema, renderer, nil, nil)

	webBarLine(t, bar, "bgp", "top set ospf router-id 192.0.2.9")
	assert.Contains(t, config.Serialize(mgr.Tree("testuser"), mgr.schema), "192.0.2.9", "the bar dropped the rest of the top line")
}

// TestCLIBarSetRefusesEnum types the refused value on the CLI bar inside a
// context, behind `top`.
//
// VALIDATES: Q-8b -- the bar's `set` judges the value; the fix is framed for
// the bar's position.
// PREVENTS: the bar storing what the terminal refuses.
func TestCLIBarSetRefusesEnum(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	bar := HandleCLICommandWithAuthorizerAndAudit(mgr, schema, renderer, nil, nil)
	// The refusal comes before the editor session is opened, so open it here:
	// the stored-value check below then reads a real candidate, never nil.
	_, err = mgr.GetOrCreate("testuser")
	require.NoError(t, err)

	body := webBarLine(t, bar, "ospf", "top set "+routeRefreshPath+" requre")
	assert.Contains(t, body, routeRefreshWhy)
	assert.Contains(t, body, routeRefreshTopFix)
	assert.NotContains(t, config.Serialize(mgr.Tree("testuser"), mgr.schema), "requre", "a refused value was stored")
}

// TestWebFormRefusesEnumAtSet posts the refused value to the form's set
// endpoint, as a crafted request would.
//
// VALIDATES: AC-7 -- the form refuses at set; the notice holds the problem,
// the accepted values and the root fix on its last line (the form holds no
// editor position).
// PREVENTS: the form checking the type alone and storing an out-of-enum
// value until commit.
func TestWebFormRefusesEnumAtSet(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	// The set is refused before any editor session opens, so open alice's
	// first: the last assertion reads her candidate.
	_, err := mgr.GetOrCreate("alice")
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.Handle("POST /config/set/", HandleConfigSetWithAuthorizer(mgr, schema, nil, nil))

	req := postConfigRequest(t, "/config/set/bgp/peer/peer1/session/capability/", map[string][]string{
		"leaf":  {"route-refresh"},
		"value": {"requre"},
	}, "alice")
	rec := httptest.NewRecorder()
	serverHandler(mux).ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Contains(t, body, routeRefreshProblem)
	assert.Contains(t, body, routeRefreshWhy)
	assert.Equal(t, routeRefreshRootFix, lastLine(body))
	assert.NotContains(t, config.Serialize(mgr.Tree("alice"), mgr.schema), "requre", "a refused value was stored")
}

// storeRefusedRouteRefresh writes `route-refresh requre` into user's candidate
// past every set-time check, as a candidate edited before this check existed
// holds it, so the commit is what refuses it.
func storeRefusedRouteRefresh(t *testing.T, mgr *EditorManager, user string) {
	t.Helper()
	us, err := mgr.GetOrCreate(user)
	require.NoError(t, err)
	require.NoError(t, us.editor.SetValue([]string{"bgp", "peer", "peer1", "session", "capability"}, "route-refresh", "requre"))
}

// TestWebTerminalCommitShowsEnumRefusal commits a candidate holding the
// refused value on the web terminal, at the root and inside a context.
//
// VALIDATES: AC-23 on the web terminal -- the refusal names the leaf with the
// root fix at the root and the `top set` fix inside a context.
// PREVENTS: a commit fix framed for the root while the operator stands inside
// a context (runCommit received no position).
func TestWebTerminalCommitShowsEnumRefusal(t *testing.T) {
	installPeerValidator(t, func(*config.Tree) error { return nil })
	mgr, _ := newCommitTestManager(t)
	terminal := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, mgr.schema, nil, nil, nil, nil)
	storeRefusedRouteRefresh(t, mgr, "testuser")

	atRoot := webTerminalLine(t, terminal, "", "commit now")
	assert.Contains(t, atRoot.Output, routeRefreshRootFix)
	assert.NotContains(t, atRoot.Output, "top set")

	inContext := webTerminalLine(t, terminal, "ospf", "commit now")
	assert.Contains(t, inContext.Output, routeRefreshTopFix)
	assert.Equal(t, "ospf", inContext.Path)
}

// TestCLIBarCommitFramesTheFixWithThePosition commits the same candidate from
// the CLI bar inside a context.
//
// VALIDATES: AC-23 and AC-51 on the CLI bar -- handleCLICommit receives the
// bar's position, so the fix reads `top set <full path> ...`.
// PREVENTS: the bar's commit dropping the position it was typed at.
func TestCLIBarCommitFramesTheFixWithThePosition(t *testing.T) {
	installPeerValidator(t, func(*config.Tree) error { return nil })
	mgr, _ := newCommitTestManager(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	bar := HandleCLICommandWithAuthorizerAndAudit(mgr, mgr.schema, renderer, nil, nil)
	storeRefusedRouteRefresh(t, mgr, "testuser")

	assert.Contains(t, webBarLine(t, bar, "ospf", "commit now"), routeRefreshTopFix)
	atRoot := webBarLine(t, bar, "", "commit now")
	assert.Contains(t, atRoot, routeRefreshRootFix)
	assert.NotContains(t, atRoot, "top set")
}

// TestWebTerminalEntryVerbFixKeepsThePosition types `rename`, `copy` and
// `insert` under a refused key on the web terminal, at the root and behind
// `top` inside another context.
//
// VALIDATES: AC-26 and AC-29 on the web terminal -- each refusal's fix is the
// verb the operator typed with the key corrected: `rename|copy ospf
// redistribute connected to <...>` and `insert ospf redistribute <...>` at the
// root, the same behind `top` from `bgp`; nothing is stored.
// PREVENTS: the creator's root `set` fix reaching the operator for a rename,
// copy or insert, or a fix with no `top` inside a context.
func TestWebTerminalEntryVerbFixKeepsThePosition(t *testing.T) {
	mgr, terminal := newYANGTerminal(t)
	require.False(t, strings.HasPrefix(webTerminalLine(t, terminal, "", "set ospf redistribute source connected").Output, "error"))
	before := config.Serialize(mgr.Tree("testuser"), mgr.schema)

	for _, verb := range []string{"rename", "copy"} {
		line := verb + " ospf redistribute connected to bogus"
		atRoot := webTerminalLine(t, terminal, "", line)
		assert.Equal(t, "fix: "+line[:len(line)-len("bogus")]+ospfSourceChoices, lastLine(atRoot.Output), verb)
		behindTop := webTerminalLine(t, terminal, "bgp", "top "+line)
		assert.Equal(t, "fix: top "+line[:len(line)-len("bogus")]+ospfSourceChoices, lastLine(behindTop.Output), verb)
	}

	inserted := webTerminalLine(t, terminal, "", "insert ospf redistribute bogus")
	assert.Equal(t, "fix: insert ospf redistribute "+ospfSourceChoices, lastLine(inserted.Output))
	insertedBehindTop := webTerminalLine(t, terminal, "bgp", "top insert ospf redistribute bogus")
	assert.Equal(t, "fix: top insert ospf redistribute "+ospfSourceChoices, lastLine(insertedBehindTop.Output))

	assert.Equal(t, before, config.Serialize(mgr.Tree("testuser"), mgr.schema), "a refused line changed the tree")
}

// TestWebFormRenameRefusalFixIsTheRename posts a refused destination key to
// the form's rename endpoint.
//
// VALIDATES: AC-26 on the web form -- HTTP 400; the body holds the problem and
// its last line is `fix: rename ospf redistribute connected to <...>`, the
// root form (the form holds no position); the entry keeps its key.
// PREVENTS: the form answering the creator's `set` fix for a rename.
func TestWebFormRenameRefusalFixIsTheRename(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	require.NoError(t, mgr.CreateEntry("testuser", []string{"ospf", "redistribute", "connected"}))
	req := postConfigRequest(t, "/config/rename/ospf/redistribute/connected/", url.Values{"new-key": {"bogus"}}, "testuser")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handleConfigRename(mgr, schema).ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "bogus")
	assert.Equal(t, "fix: rename ospf redistribute connected to "+ospfSourceChoices, lastLine(rec.Body.String()))
	assert.Equal(t, []string{"connected"}, mgr.Tree("testuser").GetContainer("ospf").ListKeys("redistribute"))
}
