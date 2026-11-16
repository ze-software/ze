// Design: docs/architecture/cli/command-completion.md -- shared command explanations.

package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/command"
)

// TestWebCLIExplainsCompletedOperationalInput asks the actual completion route
// for the current command, including config-mode run, without executing it.
// MUTATION: ignore explain=true; the candidate array cannot answer this request.
func TestWebCLIExplainsCompletedOperationalInput(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	const explanation = "Read the live routing state.\nNo configuration is changed."
	tree := &command.Node{Children: map[string]*command.Node{
		"show": {Children: map[string]*command.Node{
			"probe": {Description: explanation},
			"undeclared": {ShortHelp: "A summary alone"},
		}},
	}}
	handler := HandleCLICompleteWithCommandCompleter(newCompleter(t), cli.NewCommandCompleter(tree), mgr, schema)
	for _, tc := range []struct {
		mode, input string
		found       bool
	}{
		{"operational", "show probe", true},
		{"operational", "show probe ", true},
		{"config", "run show probe", true},
		{"config", "top run show probe", true},
		{"operational", "show undeclared", false},
		{"operational", "show unknown", false},
		{"config", "show probe", false},
	} {
		t.Run(tc.mode+"/"+tc.input, func(t *testing.T) {
			query := url.Values{"mode": {tc.mode}, "input": {tc.input}, "explain": {"true"}}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, authedRequest(t, http.MethodGet, "/cli/complete?"+query.Encode(), nil))
			require.Equal(t, http.StatusOK, recorder.Code)
			var answer struct {
				Explanation string `json:"explanation"`
				Found       bool   `json:"found"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &answer))
			assert.Equal(t, tc.found, answer.Found)
			if tc.found {
				assert.Equal(t, explanation, answer.Explanation)
			} else {
				assert.Empty(t, answer.Explanation)
			}
		})
	}
}

// TestWebCLIExplanationRequiresAuthentication keeps long help behind the same
// authenticated boundary as candidate completion.
func TestWebCLIExplanationRequiresAuthentication(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	handler := HandleCLICompleteWithCommandCompleter(newCompleter(t), nil, mgr, schema)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/cli/complete?explain=true&input=show", http.NoBody))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
