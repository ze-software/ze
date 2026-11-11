// Design: docs/architecture/testing/ci-format.md -- editor command barriers.
// Related: headless.go -- processMsg, settleWait.

package testing

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

// TestHeadlessDeclaredTimerIsNotWaitedInline proves a declared validation
// timer runs beside the headless driver, not inside the keypress that
// scheduled it. Method: type one editor key, which schedules the validation
// debounce, then read the pending timers before and after settleWait.
//
// VALIDATES: processMsg records the validation TimerMsg in hm.pending and
// returns; settleWait then waits for it and removes it.
// PREVENTS: processMsg blocking on a declared timer's Wait inline, which
// stalls every keystroke for the debounce and hides the pending state the
// wait actions read.
func TestHeadlessDeclaredTimerIsNotWaitedInline(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(testConfig), 0o600))

	hm, err := newHeadlessModel(headlessTestStore(t, configPath), configPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, hm.close()) })

	require.NoError(t, hm.sendMsg(tea.KeyPressMsg{Code: 'e', Text: "e"}))
	require.True(t, hasPendingValidation(hm), "the keypress must leave its validation timer pending, not waited inline")

	require.NoError(t, hm.settleWait())
	require.False(t, hasPendingValidation(hm), "settleWait must consume the pending validation timer")
}

// hasPendingValidation reports whether hm holds a validation timer not yet applied.
func hasPendingValidation(hm *headlessModel) bool {
	for _, tick := range hm.pending {
		if tick.request.IsValidation() {
			return true
		}
	}
	return false
}
