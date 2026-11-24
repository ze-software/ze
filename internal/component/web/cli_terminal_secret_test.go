// Design: docs/architecture/web-interface.md -- the web CLI terminal never
// echoes a secret into the response body

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
)

// The values these cases type at the web terminal. Each carries a distinctive
// TAIL, asserted separately from the whole: a message can escape the value and
// then contain no copy of it while publishing every character.
const (
	typedTerminalSecret     = "hunter2-a4471bc-tail"
	typedTerminalSecretTail = "a4471bc-tail"
)

// TestWebTerminalSetNeverEchoesASecret drives the verb dispatch the terminal
// runs for the line the operator types.
//
// VALIDATES: the acknowledgement the web CLI terminal answers for a
// ze:sensitive leaf carries config.SecretDataPlaceholder and no part of the
// value.
// PREVENTS: a credential traveling to the browser in the response body, where
// view-source, the disk cache and any proxy reading the document all hold it.
// executeTerminalSet answered `set <leaf> <value>` with the raw value.
func TestWebTerminalSetNeverEchoesASecret(t *testing.T) {
	mgr := newTestEditorManager(t)
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	contextPath := []string{"system", "authentication", "user", "alice"}

	output := executeTerminalNav(schema, nil, mgr, "alice", contextPath,
		cliCommand{Verb: verbSet, Args: []string{"plaintext-password", typedTerminalSecret}})

	require.Contains(t, output, "plaintext-password",
		"the acknowledgement must name the leaf, or this case proves nothing")
	assert.NotContains(t, output, typedTerminalSecret, "the acknowledgement published the value")
	assert.NotContains(t, output, typedTerminalSecretTail, "the acknowledgement published the tail of the value")
	assert.Contains(t, output, config.SecretDataPlaceholder, "the acknowledgement wrote no placeholder")
}

// TestWebTerminalSetStillEchoesAValueTheSchemaDoesNotMark is the other polarity.
//
// VALIDATES: an unmarked leaf still echoes what the operator typed.
// PREVENTS: a vacuous pass above. config.DisplayValueAtPath fails closed on a
// path it cannot resolve, so a resolver that cannot walk an ordinary token path
// would mask every acknowledgement the terminal answers.
func TestWebTerminalSetStillEchoesAValueTheSchemaDoesNotMark(t *testing.T) {
	mgr := newTestEditorManager(t)
	schema, err := config.YANGSchema()
	require.NoError(t, err)

	output := executeTerminalNav(schema, nil, mgr, "alice", []string{"bgp"},
		cliCommand{Verb: verbSet, Args: []string{"router-id", "10.9.8.7"}})

	assert.Contains(t, output, "10.9.8.7",
		"the terminal masked a leaf the schema does not mark, so its mask reads something other than the schema")
	assert.NotContains(t, output, config.SecretDataPlaceholder)
}

// TestWebTerminalHTTPSetMasksSecretInOutputAndFeedback drives the real HTTP
// POST /cli/terminal handler and checks both JSON `output` and JSON `feedback`.
//
// VALIDATES: AC-18 -- setting a ze:sensitive leaf (`plaintext-password`) masks
// the value in both JSON `output` and JSON `feedback`, while a non-secret leaf
// preserves its clear value in both fields.
// PREVENTS: terminalFeedback echoing raw `set` arguments after executeTerminalSet
// masked `output`.
func TestWebTerminalHTTPSetMasksSecretInOutputAndFeedback(t *testing.T) {
	mgr := newTestEditorManager(t)
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, schema, nil, nil, nil, nil)

	postTerminal := func(path, command string) terminalResponse {
		t.Helper()
		form := url.Values{
			"command": {command},
			"path":    {path},
			"mode":    {terminalModeConfig},
		}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/cli/terminal", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withUsername(req.Context(), "alice"))
		rec := httptest.NewRecorder()
		handler(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var resp terminalResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		return resp
	}

	secretResp := postTerminal("system/authentication/user/alice", "set plaintext-password "+typedTerminalSecret)
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "output", value: secretResp.Output},
		{name: "feedback", value: secretResp.Feedback},
	} {
		require.Contains(t, field.value, "plaintext-password", "JSON %s must name the leaf", field.name)
		assert.Contains(t, field.value, config.SecretDataPlaceholder, "JSON %s must show secret placeholder", field.name)
		assert.NotContains(t, field.value, typedTerminalSecret, "JSON %s leaked the secret", field.name)
		assert.NotContains(t, field.value, typedTerminalSecretTail, "JSON %s leaked the secret tail", field.name)
	}

	// Non-secret control: clear value must be preserved in both output and feedback.
	plainResp := postTerminal("bgp", "set router-id 10.9.8.7")
	assert.Contains(t, plainResp.Output, "10.9.8.7")
	assert.NotContains(t, plainResp.Output, config.SecretDataPlaceholder)
	assert.Contains(t, plainResp.Feedback, "10.9.8.7")
	assert.NotContains(t, plainResp.Feedback, config.SecretDataPlaceholder)
}
