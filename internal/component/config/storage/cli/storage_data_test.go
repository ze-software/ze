package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// checkStore writes keys into a new live tree store, closes it, and answers
// the tree path a check walks.
func checkStore(t *testing.T, keys map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Create(dir)
	require.NoError(t, err)
	for key, value := range keys {
		require.NoError(t, store.WriteKey(key, []byte(value)))
	}
	require.NoError(t, store.Close())
	return filepath.Join(dir, storage.TreeName)
}

// checkFindings answers the finding rows of a `show data check` payload.
func checkFindings(t *testing.T, answer command.LocalAnswer) []map[string]any {
	t.Helper()
	payload, ok := answer.Payload.(map[string]any)
	require.True(t, ok, "payload %T is not the verdict document", answer.Payload)
	rows, ok := payload["findings"].([]map[string]any)
	require.True(t, ok, "findings %T are not rows", payload["findings"])
	return rows
}

// TestDataCheckAnswersRows verifies AC-13: `show data check` answers the
// store's integrity verdict as rows. Method: a clean store answers its entry
// count, no row and exit 0; a store with one frame whose bytes no longer
// decode answers one frame row naming that key, exit 1 and the closing line
// `ze data check` prints. The handler's producer is called directly for the
// local route, as ze data check calls it.
// VALIDATES: AC-13 (rows, status).
// PREVENTS: a verdict that answers success over a corrupt store, or text no pipe can render.
func TestDataCheckAnswersRows(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		tree := checkStore(t, map[string]string{"meta/test/one": "one", "meta/test/two": "two"})
		verdict, err := integrityVerdict(tree)
		require.NoError(t, err)
		answer := verdict.answer()
		assert.Zero(t, answer.Code)
		assert.Empty(t, answer.Diagnostic)
		assert.Empty(t, checkFindings(t, answer))
		payload, ok := answer.Payload.(map[string]any)
		require.True(t, ok)
		entries, ok := payload["entries"].(int)
		require.True(t, ok)
		assert.GreaterOrEqual(t, entries, 2, "both written keys are counted")
		assert.Equal(t, 0, payload["corrupt-entries"])
		assert.Equal(t, true, payload["history-checked"])
	})

	t.Run("corrupt", func(t *testing.T) {
		const damaged = "meta/test/damaged"
		tree := checkStore(t, map[string]string{damaged: "bytes that will not decode", "meta/test/fine": "fine"})
		frame := filepath.Join(tree, filepath.FromSlash(damaged))
		raw, err := os.ReadFile(frame) //nolint:gosec // the test's own temporary file
		require.NoError(t, err)
		index := bytes.Index(raw, []byte("bytes that will not decode"))
		require.GreaterOrEqual(t, index, 0)
		raw[index] ^= 1
		require.NoError(t, os.WriteFile(frame, raw, 0o600))

		verdict, err := integrityVerdict(tree)
		require.NoError(t, err)
		answer := verdict.answer()
		assert.Equal(t, 1, answer.Code)
		assert.True(t, strings.HasPrefix(answer.Diagnostic, "error: 1/"), answer.Diagnostic)
		assert.Contains(t, answer.Diagnostic, " entries corrupt; history not checked, run ze data repair --output <path>")
		rows := checkFindings(t, answer)
		require.Len(t, rows, 1)
		assert.Equal(t, damaged, rows[0]["key"])
		assert.Equal(t, severityError, rows[0]["severity"])
		assert.Equal(t, checkFrame, rows[0]["check"])
		payload, ok := answer.Payload.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, false, payload["history-checked"])
		assert.Equal(t, 1, cmdCheck(tree, nil), "ze data check grades the same rows")
	})
}

// TestDataCheckOnTheDaemonChecksItsOwnTree verifies AC-13 on the daemon
// route: with the daemon's store published, `show data check` walks the tree
// that store commits to, with the writers held (storage.HoldWrites), and
// answers its entries. Method: a key written through the live handle is
// counted by a remote-route answer, which names the live tree.
// VALIDATES: AC-13 (daemon route).
// PREVENTS: the daemon checking a store other than the one it owns.
func TestDataCheckOnTheDaemonChecksItsOwnTree(t *testing.T) {
	store, _ := dataRPCStore(t, func(context.Context) error { return nil })
	require.NoError(t, store.WriteKey("meta/test/live", []byte("live")))

	answer := command.AnswerRemote(dataCheck, commandtest.Args())
	assert.Zero(t, answer.Code, answer.Diagnostic)
	assert.Empty(t, checkFindings(t, answer))
	payload, ok := answer.Payload.(map[string]any)
	require.True(t, ok)
	path, ok := payload["path"].(string)
	require.True(t, ok)
	assert.Equal(t, storage.TreeName, filepath.Base(path))
	entries, ok := payload["entries"].(int)
	require.True(t, ok)
	assert.Positive(t, entries)
}
