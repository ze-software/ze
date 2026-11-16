// Design: docs/architecture/web-interface.md -- shared browser command behavior.

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

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/configtext"
)

// applyWebSet lets the typed-refusal tests inspect the real shared producer's
// error before the HTTP adapter converts it to a response body.
func applyWebSet(mgr *EditorManager, _ *config.Schema, username string, path, operator, args []string) (string, error) {
	parts := []string{verbSet}
	for _, argument := range args {
		parts = append(parts, configtext.QuoteCommandWord(argument))
	}
	answer, err := mgr.ConfigCommand(username, contract.ConfigCommandRequest{Line: strings.Join(parts, " "), Path: path, Operator: operator})
	return answer.Feedback, err
}

// TestWebTerminalHTTPMasksSecretFeedback checks the complete transport envelope,
// rather than only the set producer's acknowledgement.
func TestWebTerminalHTTPMasksSecretFeedback(t *testing.T) {
	_, terminal := newYANGTerminal(t)
	response := webTerminalLine(t, terminal, "", `set system authentication user alice plaintext-password "browser-secret-sentinel"`)
	encoded, err := json.Marshal(response)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "browser-secret-sentinel")
	assert.Contains(t, response.Output, config.SecretDataPlaceholder)
	control := webTerminalLine(t, terminal, "", "set bgp router-id 192.0.2.99")
	assert.Contains(t, control.Feedback, "192.0.2.99")
}

// TestWebTerminalShowPipesReachEditor compares metadata/error views with ordinary
// configuration, including a control proving the candidate contains changes.
func TestWebTerminalShowPipesReachEditor(t *testing.T) {
	_, terminal := newYANGTerminal(t)
	webTerminalLine(t, terminal, "", "set bgp router-id 192.0.2.99")
	plain := webTerminalLine(t, terminal, "", "show bgp")
	assert.Contains(t, plain.Output, "192.0.2.99")
	errors := webTerminalLine(t, terminal, "", "show | errors")
	assert.NotContains(t, errors.Output, "router-id 192.0.2.99;")
	assert.NotEmpty(t, errors.Output)
	assert.Equal(t, errors.Output, webTerminalLine(t, terminal, "", "errors").Output)
	history := webTerminalLine(t, terminal, "", "show | history")
	assert.NotEqual(t, plain.Output, history.Output)
	assert.NotContains(t, history.Output, "router-id 192.0.2.99;")
}

// TestWebTerminalLoadChangesCandidateOnly pastes a relative body, checks the
// independent context, and proves invalid input leaves the candidate unchanged.
func TestWebTerminalLoadChangesCandidateOnly(t *testing.T) {
	mgr, terminal := newYANGTerminal(t)
	response := runTerminalCommandForm(t, terminal, url.Values{
		"command": {"load terminal relative merge"}, "path": {"bgp"},
		"content": {"router-id 192.0.2.88;"},
	})
	assert.NotContains(t, response.Output, "unknown command")
	assert.Contains(t, webTerminalLine(t, terminal, "", "show bgp").Output, "192.0.2.88")
	before := config.Serialize(mgr.Tree("testuser"), mgr.schema)
	refused := runTerminalCommandForm(t, terminal, url.Values{
		"command": {"load terminal relative replace"}, "path": {"bgp"},
		"content": {"unknown-setting 123;"},
	})
	assert.Contains(t, refused.Output, "error:")
	assert.Equal(t, before, config.Serialize(mgr.Tree("testuser"), mgr.schema))
	assert.Positive(t, mgr.ChangeCount("testuser"))
}

// TestWebCLIBarRunsStructuralCommands uses the same HTTP surface as the
// integrated bar and verifies the renamed candidate, not a success message.
func TestWebCLIBarRunsStructuralCommands(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	bar := HandleCLICommandWithAuthorizerAndAudit(mgr, schema, renderer, nil, nil)
	webBarLine(t, bar, "", "set bgp group original description marker")
	response := webBarLine(t, bar, "", "rename bgp group original to renamed")
	assert.NotEmpty(t, strings.TrimSpace(response))
	shown := mgr.ContentAtPath("testuser", []string{"bgp"})
	assert.Contains(t, shown, "renamed")
	assert.NotContains(t, shown, "group original")
}

// TestWebTerminalOptionRunsSharedProducer proves an option query is answered
// and an invalid option is refused instead of silently ignored.
func TestWebTerminalOptionRunsSharedProducer(t *testing.T) {
	_, terminal := newYANGTerminal(t)
	good := webTerminalLine(t, terminal, "", "option author")
	assert.Contains(t, good.Feedback+good.Output, "author:")
	bad := webTerminalLine(t, terminal, "", "option invented")
	assert.Contains(t, bad.Output, "unknown option:")
}

// TestWebTerminalOptionsPersistWithinUser checks a second request against the
// same editor, and a separate user's default, without a timer or shared model.
func TestWebTerminalOptionsPersistWithinUser(t *testing.T) {
	_, terminal := newYANGTerminal(t)
	assert.Contains(t, webTerminalLine(t, terminal, "", "option errors").Output, "enabled")
	webTerminalLine(t, terminal, "", "option errors hints")
	assert.Contains(t, webTerminalLine(t, terminal, "", "option errors").Output, "disabled")
	r := authedRequest(t, http.MethodPost, "/cli/terminal", url.Values{"command": {"option errors"}})
	r = r.WithContext(withUsername(r.Context(), "other-user"))
	w := httptest.NewRecorder()
	terminal.ServeHTTP(w, r)
	assert.Contains(t, w.Body.String(), "enabled")
}

// TestWebTerminalRefusesMutationPipe checks that a parseable pipe cannot stage
// a change before its unsupported operator is refused.
func TestWebTerminalRefusesMutationPipe(t *testing.T) {
	mgr, terminal := newYANGTerminal(t)
	webTerminalLine(t, terminal, "", "show")
	before := config.Serialize(mgr.Tree("testuser"), mgr.schema)
	answer := webTerminalLine(t, terminal, "", "set bgp router-id 192.0.2.77 | head 1")
	assert.Contains(t, answer.Output, "does not support piping")
	assert.Equal(t, before, config.Serialize(mgr.Tree("testuser"), mgr.schema))
}

// TestWebTerminalInsertUsesCLIOrdering proves first/before/after affect the
// stored sequence and preserve the policy beside it.
func TestWebTerminalInsertUsesCLIOrdering(t *testing.T) {
	mgr, terminal := newYANGTerminal(t)
	webTerminalLine(t, terminal, "", "set bgp filter import final accept")
	for _, line := range []string{
		"insert bgp filter import filters alpha first",
		"insert bgp filter import filters bravo last",
		"insert bgp filter import filters charlie after alpha",
	} {
		answer := webTerminalLine(t, terminal, "", line)
		require.NotContains(t, answer.Output, "error:", line)
	}
	policy := mgr.Tree("testuser").GetContainer("bgp").GetContainer("filter").GetContainer("import")
	assert.Equal(t, []string{"alpha", "charlie", "bravo"}, policy.GetMultiValues("filters"))
	final, exists := policy.Get("final")
	assert.True(t, exists)
	assert.Equal(t, "accept", final)
}

// TestWebTerminalUpLeavesListEntry runs each request after the prior answer,
// using the server's returned context exactly as the browser queue does.
func TestWebTerminalUpLeavesListEntry(t *testing.T) {
	_, terminal := newYANGTerminal(t)
	webTerminalLine(t, terminal, "", "set bgp group navigation description marker")
	entered := webTerminalLine(t, terminal, "", "edit bgp group navigation")
	require.Equal(t, "bgp/group/navigation", entered.Path)
	parent := webTerminalLine(t, terminal, entered.Path, "up")
	assert.Equal(t, "bgp", parent.Path)
	set := webTerminalLine(t, terminal, parent.Path, "set router-id 192.0.2.72")
	assert.NotContains(t, set.Output, "error:")
	assert.Contains(t, webTerminalLine(t, terminal, "", "show bgp").Output, "192.0.2.72")
}

// TestWebTerminalDiscardUsesCurrentContext retains an unrelated pending edit
// while discarding one relative setting, as the SSH editor does.
func TestWebTerminalDiscardUsesCurrentContext(t *testing.T) {
	mgr, terminal := newYANGTerminal(t)
	webTerminalLine(t, terminal, "", "set bgp router-id 192.0.2.91")
	webTerminalLine(t, terminal, "", "set bgp group retained description retained")
	answer := webTerminalLine(t, terminal, "bgp", "discard router-id")
	assert.NotContains(t, answer.Output, "error:")
	shown := config.Serialize(mgr.Tree("testuser"), mgr.schema)
	assert.NotContains(t, shown, "192.0.2.91")
	assert.Contains(t, shown, "retained")
}

// TestWebTerminalLoadCannotReadDaemonFiles refuses a file command before a
// browser-selected file payload exists; a path never grants filesystem access.
func TestWebTerminalLoadCannotReadDaemonFiles(t *testing.T) {
	_, terminal := newYANGTerminal(t)
	w := httptest.NewRecorder()
	terminal.ServeHTTP(w, authedRequest(t, http.MethodPost, "/cli/terminal", url.Values{
		"command": {"load file absolute replace /etc/passwd"},
	}))
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "root:")
	assert.Contains(t, w.Body.String(), "file")
}
