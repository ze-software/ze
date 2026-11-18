// Design: docs/guide/operations.md -- local full restore reaches its actual store owner.
package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/configtext"
)

// TestRestoreFullRoutesBusyStoreAndChecksReceipt proves that a live tree's
// owner receives the selected absolute source and only a complete matching
// acceptance receipt reports success, never that recovery already completed.
func TestRestoreFullRoutesBusyStoreAndChecksReceipt(t *testing.T) {
	folder := t.TempDir()
	live, err := storage.Create(folder)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, live.Close()) })
	source := dataArtifact(t, "router.conf", "replacement")
	storePath := filepath.Join(folder, storage.TreeName)
	for _, test := range []struct {
		name     string
		mutate   func(map[string]any)
		accepted bool
	}{
		{name: "matching", accepted: true},
		{name: "wrong source", mutate: func(body map[string]any) { body["path"] = "/different.zefs" }},
		{name: "missing digest", mutate: func(body map[string]any) { delete(body, "digest") }},
		{name: "invalid digest", mutate: func(body map[string]any) { body["digest"] = strings.Repeat("z", 64) }},
		{name: "missing bytes", mutate: func(body map[string]any) { delete(body, "bytes") }},
		{name: "not accepted", mutate: func(body map[string]any) { body["state"] = "completed" }},
		{name: "wrong restart", mutate: func(body map[string]any) { body["restart-exit-status"] = 0 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := map[string]any{"path": source, "state": "accepted", "bytes": 123,
				"digest": strings.Repeat("a", 64), "restart-exit-status": 75}
			if test.mutate != nil {
				test.mutate(body)
			}
			encoded, err := json.Marshal(body)
			require.NoError(t, err)
			calls := 0
			ask := func(gotFolder, line string) ([]byte, error) {
				calls++
				require.Equal(t, folder, gotFolder)
				require.Equal(t, "request data restore path "+configtext.QuoteCommandWord(source)+" full", line)
				return encoded, nil
			}
			code, output, diagnostic := captureCatStdio(t, func() int {
				return cmdRestoreFullThrough(storePath, restoreArgs{file: source, mode: restoreModeFull}, ask)
			})
			require.Equal(t, 1, calls, diagnostic)
			if test.accepted {
				require.Zero(t, code, diagnostic)
				require.Contains(t, output, "accepted")
				require.NotContains(t, output, "keys now form")
				return
			}
			require.NotZero(t, code)
			require.Empty(t, output)
		})
	}
}
