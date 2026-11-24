// Design: docs/architecture/web-components.md -- graphical actions retain command word boundaries.

package web

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGraphicalActionsKeepLiteralPipeKeys follows generated copy and rename
// commands through HTTP and checks the exact source and target list keys.
func TestGraphicalActionsKeepLiteralPipeKeys(t *testing.T) {
	for _, action := range []string{"copy", "rename"} {
		t.Run(action, func(t *testing.T) {
			mgr, schema, _, _ := setupCLITerminalYANGTest(t)
			source, target := "|", "copied"
			if action == "rename" {
				source, target = "source", "|"
			}
			require.NoError(t, mgr.SetValue("testuser", []string{"bgp", "group", source}, "description", "original"))
			handler := HandleGraphicalAction(mgr, schema, nil, nil, nil, nil)
			path := encodeConfigPath([]string{"bgp", "group", source})
			rec := graphicalActionRequest(t, handler, action, path, url.Values{"target": {target}})
			requireStatus(t, http.StatusOK, rec)
			groups := mgr.snapshotTree("testuser").GetContainer("bgp").GetList("group")
			require.NotNil(t, groups[target], rec.Body.String())
			value, ok := groups[target].Get("description")
			assert.True(t, ok)
			assert.Equal(t, "original", value)
			if action == "copy" {
				assert.NotNil(t, groups[source])
			} else {
				assert.Nil(t, groups[source])
			}
		})
	}
}
