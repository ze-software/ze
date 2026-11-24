// Design: docs/architecture/web-interface.md -- actual terminal proof helpers.
package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"testing"

	"github.com/stretchr/testify/require"
)

// terminalAsUser follows the authenticated HTTP entry point and its JSON answer.
func terminalAsUser(t *testing.T, mgr *EditorManager, user string, path []string, line string) terminalResponse {
	t.Helper()
	handler := HandleCLITerminalWithDispatchAuthorizerAndAudit(mgr, mgr.schema, mgr.snapshotTree(user), nil, nil, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, postConfigRequest(t, "/cli/terminal", url.Values{
		"command": {line}, "path": {encodeConfigPath(path)}, "mode": {terminalModeConfig},
	}, user))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var answer terminalResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &answer))
	return answer
}
