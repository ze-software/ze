// Design: docs/architecture/web-interface.md -- admin command discovery.
package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/command"
)

// TestAdminCommandDiscoveryOmitsHiddenNodes asks the real admin JSON handler
// for its navigation, including a visible grouping whose only child is hidden.
func TestAdminCommandDiscoveryOmitsHiddenNodes(t *testing.T) {
	tree := &command.Node{Children: map[string]*command.Node{
		"public": {Name: "public", WireMethod: "fixture:public"},
		"secret": {Name: "secret", Hidden: true, WireMethod: "fixture:secret"},
		"group": {Name: "group", Children: map[string]*command.Node{
			"concealed": {Name: "concealed", Hidden: true},
		}},
	}}
	handler := HandleAdminView(nil, StaticCommandTree(tree))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/?format=json", http.NoBody)
	handler.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	var answer struct {
		Children []string `json:"children"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &answer))
	require.Equal(t, []string{"group", "public"}, answer.Children)
	require.False(t, adminHasChildren(tree.Children["group"]))
	require.Empty(t, adminChildNames(tree.Children["secret"]))
	// Hidden is a discovery property, not an execution-path deletion.
	require.NotNil(t, command.FindNode(tree, []string{"secret"}))
}
