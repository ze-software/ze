package fixture

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUIMonitorSaveFixtures(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	tmpSave := filepath.Join(dir, ".ze-save-1234")
	require.NoError(t, os.WriteFile(tmpSave, []byte("monitoring\n---- STATE 127.0.0.1 65000 up\n"), 0o600))

	require.NoError(t, uiAwaitSaveFile(t.Context(), []string{"monitoring"}))
	require.NoError(t, uiAwaitSaveFile(t.Context(), []string{`STATE.* up`}))

	committed := filepath.Join(dir, "events.log")
	require.NoError(t, os.WriteFile(committed, []byte("---- STATE 127.0.0.1 65000 up\n"), 0o600))

	// Temp file still present must fail uiCheckCommittedSave.
	require.Error(t, uiCheckCommittedSave(t.Context(), []string{"events.log", `STATE.* up`, " OPEN "}))

	require.NoError(t, os.Remove(tmpSave))
	require.NoError(t, uiCheckCommittedSave(t.Context(), []string{"events.log", `STATE.* up`, " OPEN "}))

	// Forbidden text must fail uiCheckCommittedSave.
	require.NoError(t, os.WriteFile(committed, []byte("recv OPEN 127.0.0.1\n---- STATE 127.0.0.1 65000 up\n"), 0o600))
	require.Error(t, uiCheckCommittedSave(t.Context(), []string{"events.log", `STATE.* up`, " OPEN "}))
}
