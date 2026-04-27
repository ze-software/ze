package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/schema" // Register BGP YANG for editor tests.
	"codeberg.org/thomas-mangin/ze/internal/component/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	_ "codeberg.org/thomas-mangin/ze/internal/component/hub/schema" // Required by ze-bgp-conf.yang.
)

// VALIDATES: AC-3 (edit command updates breadcrumb + content), AC-15 (POST /cli dispatches command).
// PREVENTS: CLI bar command parsing fails on simple input.
func TestCLIBarCommandParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantVerb string
		wantArgs []string
	}{
		{
			name:     "simple verb",
			input:    "show",
			wantVerb: "show",
			wantArgs: nil,
		},
		{
			name:     "verb with args",
			input:    "edit bgp peer",
			wantVerb: "edit",
			wantArgs: []string{"bgp", "peer"},
		},
		{
			name:     "set with value",
			input:    "set remote-as 65002",
			wantVerb: "set",
			wantArgs: []string{"remote-as", "65002"},
		},
		{
			name:     "quoted argument",
			input:    `set description "my peer"`,
			wantVerb: "set",
			wantArgs: []string{"description", "my peer"},
		},
		{
			name:     "leading spaces",
			input:    "   show bgp",
			wantVerb: "show",
			wantArgs: []string{"bgp"},
		},
		{
			name:     "trailing spaces",
			input:    "top   ",
			wantVerb: "top",
			wantArgs: nil,
		},
		{
			name:     "empty input",
			input:    "",
			wantVerb: "",
			wantArgs: nil,
		},
		{
			name:     "whitespace only",
			input:    "   ",
			wantVerb: "",
			wantArgs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := parseCLICommand(tt.input)
			assert.Equal(t, tt.wantVerb, cmd.Verb)
			if tt.wantArgs == nil {
				assert.Empty(t, cmd.Args)
			} else {
				assert.Equal(t, tt.wantArgs, cmd.Args)
			}
		})
	}
}

// VALIDATES: AC-2 (CLI bar prompt matches breadcrumb path).
// PREVENTS: Prompt format diverges from spec.
func TestCLIBarPromptSync(t *testing.T) {
	tests := []struct {
		name       string
		path       []string
		wantPrompt string
	}{
		{
			name:       "root",
			path:       nil,
			wantPrompt: "ze# ",
		},
		{
			name:       "single segment",
			path:       []string{"bgp"},
			wantPrompt: "ze[bgp]# ",
		},
		{
			name:       "nested path",
			path:       []string{"bgp", "peer", "192.168.1.1"},
			wantPrompt: "ze[bgp peer 192.168.1.1]# ",
		},
		{
			name:       "empty slice",
			path:       []string{},
			wantPrompt: "ze# ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCLIPrompt(tt.path)
			assert.Equal(t, tt.wantPrompt, got)
		})
	}
}

// setupCLITest creates an EditorManager with a test config file for CLI handler tests.
// Returns the manager, schema, renderer, and a cleanup function.
func setupCLITest(t *testing.T) (*EditorManager, *Renderer) {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	configContent := `bgp {
    router-id 1.2.3.4;
    as 65001;
    peer 192.168.1.1 {
        remote-as 65002;
    }
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0o644))

	schema, _ := buildTestSchemaAndTree()
	store := storage.NewFilesystem()
	mgr := NewEditorManager(store, configPath, schema, testEditorFactory(), testEditSessionFactory())

	renderer, err := NewRenderer()
	require.NoError(t, err)

	return mgr, renderer
}

// authedRequest creates a request with a username in context for handler tests.
func authedRequest(method, target string, body url.Values) *http.Request {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, target, strings.NewReader(body.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		r = httptest.NewRequest(method, target, http.NoBody)
	}

	r = r.WithContext(context.WithValue(r.Context(), ctxKeyUsername, "testuser"))

	return r
}

// VALIDATES: AC-3 (edit command updates breadcrumb + content).
// PREVENTS: Edit command returns error for valid schema path.
func TestCLIBarEditUpdatesContext(t *testing.T) {
	mgr, renderer := setupCLITest(t)
	schema, _ := buildTestSchemaAndTree()

	handler := HandleCLICommand(mgr, schema, renderer)

	body := url.Values{
		"command": {"edit bgp"},
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/cli", body)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	respBody := w.Body.String()
	assert.Contains(t, respBody, `id="content-area"`)
	assert.Contains(t, respBody, `id="breadcrumb-bar"`)
	assert.Contains(t, respBody, `id="cli-prompt"`)
	assert.Contains(t, respBody, "ze[bgp]# ")
	assert.Contains(t, respBody, "bgp")
}

// VALIDATES: AC-4 (set command updates value and returns notification).
// PREVENTS: Set command fails silently.
func TestCLIBarSetUpdatesNotification(t *testing.T) {
	mgr, renderer := setupCLITest(t)
	schema, _ := buildTestSchemaAndTree()

	handler := HandleCLICommand(mgr, schema, renderer)

	body := url.Values{
		"command": {"set router-id 5.6.7.8"},
		"path":    {"bgp"},
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/cli", body)
	r.Header.Set("HX-Request", "true")
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

// VALIDATES: AC-5 (show command renders config output).
// PREVENTS: Show command returns empty content.
func TestCLIBarShowRendersOutput(t *testing.T) {
	mgr, renderer := setupCLITest(t)
	schema, _ := buildTestSchemaAndTree()

	// Create a session first so ContentAtPath works.
	_, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)

	handler := HandleCLICommand(mgr, schema, renderer)

	body := url.Values{
		"command": {"show"},
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/cli", body)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `class="config-output"`)
}

// VALIDATES: AC-6 (top command clears context to root).
// PREVENTS: Top command returns non-root breadcrumb.
func TestCLIBarTopClearsContext(t *testing.T) {
	mgr, renderer := setupCLITest(t)
	schema, _ := buildTestSchemaAndTree()

	handler := HandleCLICommand(mgr, schema, renderer)

	body := url.Values{
		"command": {"top"},
		"path":    {"bgp/peer"},
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/cli", body)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ze# ")
	assert.Contains(t, w.Body.String(), `id="content-area"`)
}

// VALIDATES: AC-7 (up command navigates to parent).
// PREVENTS: Up command does not change path.
func TestCLIBarUpNavigates(t *testing.T) {
	mgr, renderer := setupCLITest(t)
	schema, _ := buildTestSchemaAndTree()

	handler := HandleCLICommand(mgr, schema, renderer)

	body := url.Values{
		"command": {"up"},
		"path":    {"bgp/peer"},
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/cli", body)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	// Path was bgp/peer, after "up" should be bgp.
	assert.Contains(t, w.Body.String(), "ze[bgp]# ")
}

// VALIDATES: AC-8 (autocomplete returns candidates).
// PREVENTS: Autocomplete endpoint returns empty or errors.
func TestCLIBarAutocomplete(t *testing.T) {
	completer := cli.NewCompleter()

	handler := HandleCLIComplete(completer, nil, nil)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/cli/complete?input=ed", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var items []struct {
		Text        string `json:"text"`
		Description string `json:"description"`
		Type        string `json:"type"`
	}
	err := json.NewDecoder(w.Body).Decode(&items)
	require.NoError(t, err)
	assert.NotEmpty(t, items, "autocomplete should return candidates for 'ed'")

	// "edit" should be among the completions.
	found := false
	for _, item := range items {
		if item.Text == "edit" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'edit' in completions")
}

// VALIDATES: AC-10 (mode toggle switches to terminal mode).
// PREVENTS: Toggle returns error or wrong content.
func TestTerminalModeToggle(t *testing.T) {
	mgr, renderer := setupCLITest(t)
	schema, _ := buildTestSchemaAndTree()

	handler := HandleCLIModeToggle(mgr, schema, renderer)

	body := url.Values{
		"mode": {"terminal"},
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/cli/mode", body)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `class="terminal-container"`)
	assert.Contains(t, w.Body.String(), `id="terminal-scrollback"`)
	assert.Contains(t, w.Body.String(), `id="terminal-input"`)
}

// VALIDATES: AC-12 (terminal endpoint returns JSON with output and feedback).
// PREVENTS: Terminal command returns wrong format.
func TestTerminalModeCommand(t *testing.T) {
	mgr, _ := setupCLITest(t)

	_, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)

	schema, tree := buildTestSchemaAndTree()
	handler := HandleCLITerminal(mgr, schema, tree)

	body := url.Values{
		"command": {"help"},
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/cli/terminal", body)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var resp terminalResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Output, "commands:")
}

// PREVENTS: Pipe operators broken or missing in the web CLI terminal.
// Covers every pipe filter type that ApplyPipeFilter and the web CLI handle.
func TestTerminalPipes(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantErr     string
		wantContain string
		wantAbsent  string
	}{
		{
			name:        "format tree",
			command:     "show bgp | format tree",
			wantContain: "router-id",
		},
		{
			name:    "format config",
			command: "show bgp | format config",
		},
		{
			name:    "format unknown",
			command: "show | format bogus",
			wantErr: "unknown format: bogus",
		},
		{
			name:        "match filters lines",
			command:     "show bgp | match peer",
			wantContain: "peer",
			wantAbsent:  "router-id",
		},
		{
			name:        "head limits output",
			command:     "show bgp | head 2",
			wantContain: "router-id",
		},
		{
			name:    "head default (no arg)",
			command: "show bgp | head",
		},
		{
			name:    "tail limits output",
			command: "show bgp | tail 2",
		},
		{
			name:    "tail default (no arg)",
			command: "show bgp | tail",
		},
		{
			name:       "compare uses real diff",
			command:    "show bgp | compare",
			wantAbsent: "+ router-id",
		},
		{
			name:        "format tree then match",
			command:     "show bgp | format tree | match peer",
			wantContain: "peer",
		},
		{
			name:        "match then head",
			command:     "show bgp | match peer | head 1",
			wantContain: "peer",
		},
		{
			name:    "unknown pipe operator",
			command: "show bgp | nosuch",
			wantErr: "unknown pipe filter: nosuch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, _ := setupCLITest(t)

			_, err := mgr.GetOrCreate("testuser")
			require.NoError(t, err)

			schema, tree := buildTestSchemaAndTree()
			handler := HandleCLITerminal(mgr, schema, tree)

			body := url.Values{"command": {tt.command}}
			w := httptest.NewRecorder()
			r := authedRequest(http.MethodPost, "/cli/terminal", body)
			handler.ServeHTTP(w, r)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp terminalResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

			if tt.wantErr != "" {
				assert.Contains(t, resp.Output, tt.wantErr)
				return
			}
			assert.NotContains(t, resp.Output, "pipe error", "pipe must not error")
			if tt.wantContain != "" {
				assert.Contains(t, resp.Output, tt.wantContain)
			}
			if tt.wantAbsent != "" {
				assert.NotContains(t, resp.Output, tt.wantAbsent)
			}
		})
	}
}

// VALIDATES: AC-13 (toggle back from terminal restores integrated mode).
// PREVENTS: Toggle back returns terminal content.
func TestIntegratedModeRestore(t *testing.T) {
	mgr, renderer := setupCLITest(t)
	schema, _ := buildTestSchemaAndTree()

	handler := HandleCLIModeToggle(mgr, schema, renderer)

	body := url.Values{
		"mode": {"integrated"},
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/cli/mode", body)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `id="content-area"`)
	// Should NOT contain terminal-specific content.
	assert.NotContains(t, w.Body.String(), `class="terminal-container"`)
}

// VALIDATES: AC-15 (unauthenticated requests rejected).
// PREVENTS: CLI endpoints accessible without auth.
func TestCLIBarRequiresAuth(t *testing.T) {
	mgr, renderer := setupCLITest(t)
	schema, _ := buildTestSchemaAndTree()

	handler := HandleCLICommand(mgr, schema, renderer)

	body := url.Values{
		"command": {"show"},
	}

	w := httptest.NewRecorder()
	// No auth context.
	r := httptest.NewRequest(http.MethodPost, "/cli", strings.NewReader(body.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// VALIDATES: Input validation (command too long is rejected).
// PREVENTS: Oversized commands cause resource exhaustion.
func TestCLIBarCommandTooLong(t *testing.T) {
	mgr, renderer := setupCLITest(t)
	schema, _ := buildTestSchemaAndTree()

	handler := HandleCLICommand(mgr, schema, renderer)

	longCommand := strings.Repeat("a", maxCommandLength+1)
	body := url.Values{
		"command": {longCommand},
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/cli", body)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// VALIDATES: Unknown command returns error notification.
// PREVENTS: Unknown commands silently ignored.
func TestCLIBarUnknownCommand(t *testing.T) {
	mgr, renderer := setupCLITest(t)
	schema, _ := buildTestSchemaAndTree()

	handler := HandleCLICommand(mgr, schema, renderer)

	body := url.Values{
		"command": {"foobar"},
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/cli", body)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "unknown command: foobar")
}

// VALIDATES: Method not allowed on wrong HTTP method.
// PREVENTS: GET requests processed as POST.
func TestCLIBarMethodNotAllowed(t *testing.T) {
	mgr, renderer := setupCLITest(t)
	schema, _ := buildTestSchemaAndTree()

	handler := HandleCLICommand(mgr, schema, renderer)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodGet, "/cli", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

// VALIDATES: AC-14 (delete command removes value and redirects).
// PREVENTS: Delete command fails silently or does not call DeleteValue.
func TestCLIBarDelete(t *testing.T) {
	mgr, renderer := setupCLITest(t)
	schema, _ := buildTestSchemaAndTree()

	handler := HandleCLICommand(mgr, schema, renderer)

	// First set a value so there is something to delete.
	err := mgr.SetValue("testuser", []string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err, "precondition: set value before delete")

	body := url.Values{
		"command": {"delete router-id"},
		"path":    {"bgp"},
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/cli", body)
	handler.ServeHTTP(w, r)

	// Non-HTMX delete redirects via htmxRedirect (303 See Other).
	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/config/edit/",
		"delete must redirect to config edit path")
}

// VALIDATES: AC-9 (commit command applies changes).
// PREVENTS: Commit command returns error for valid session with changes.
func TestCLIBarCommit(t *testing.T) {
	mgr, renderer := setupCLITest(t)
	schema, _ := buildTestSchemaAndTree()

	handler := HandleCLICommand(mgr, schema, renderer)

	// Set a value so there are pending changes to commit.
	err := mgr.SetValue("testuser", []string{"bgp"}, "router-id", "5.6.7.8")
	require.NoError(t, err, "precondition: set value before commit")

	body := url.Values{
		"command": {"commit"},
		"path":    {"bgp"},
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/cli", body)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

// VALIDATES: AC-9 (discard command clears pending changes and redirects).
// PREVENTS: Discard command fails or does not redirect.
func TestCLIBarDiscard(t *testing.T) {
	mgr, renderer := setupCLITest(t)
	schema, _ := buildTestSchemaAndTree()

	handler := HandleCLICommand(mgr, schema, renderer)

	// Set a value so the user has a draft to discard.
	err := mgr.SetValue("testuser", []string{"bgp"}, "router-id", "9.9.9.9")
	require.NoError(t, err, "precondition: set value before discard")

	body := url.Values{
		"command": {"discard"},
	}

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/cli", body)
	handler.ServeHTTP(w, r)

	// Non-HTMX discard redirects via htmxRedirect (303 See Other).
	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/config/edit/", w.Header().Get("Location"),
		"discard must redirect to config edit root")
}

// VALIDATES: buildConfigEditURL constructs correct URLs.
// PREVENTS: Malformed redirect URLs.
func TestBuildConfigEditURL(t *testing.T) {
	tests := []struct {
		name string
		path []string
		want string
	}{
		{name: "root", path: nil, want: "/config/edit/"},
		{name: "single", path: []string{"bgp"}, want: "/config/edit/bgp/"},
		{name: "nested", path: []string{"bgp", "peer"}, want: "/config/edit/bgp/peer/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buildConfigEditURL(tt.path))
		})
	}
}
