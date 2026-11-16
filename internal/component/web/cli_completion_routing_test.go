// Design: docs/architecture/cli/command-completion.md -- shared convenience routing.

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestWebCompletionRoutesEditorConveniencesLikeEnter uses the same input with
// and without an actual operational registration; only the latter asks the
// operational producer, while the former offers schema nodes.
func TestWebCompletionRoutesEditorConveniencesLikeEnter(t *testing.T) {
	mgr, schema, _, _ := setupCLITerminalYANGTest(t)
	const input = "delete bgp "
	for _, registered := range []bool{false, true} {
		calls := 0
		fake := &fakeCommandCompleter{items: []contract.Completion{{Text: "daemon-only", Type: "value"}}}
		source := func(ctx context.Context, caller plugin.CallerIdentity) CommandCompleter {
			assert.NotNil(t, ctx)
			assert.Equal(t, "testuser", caller.Username)
			calls++
			return fake
		}
		handler := HandleCLICompleteWithSource(newCompleter(t), source, func(string) bool { return registered }, mgr, schema)
		query := url.Values{"input": {input}, "mode": {"operational"}}
		w := httptest.NewRecorder()
		handler(w, authedRequest(t, http.MethodGet, "/cli/complete?"+query.Encode(), nil))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 1, calls)
		if registered {
			assert.Contains(t, w.Body.String(), "daemon-only")
		} else {
			assert.NotContains(t, w.Body.String(), "daemon-only")
			assert.Contains(t, w.Body.String(), "peer")
		}
	}
}
