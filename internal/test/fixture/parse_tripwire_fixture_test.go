package fixture

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: parse/write-tripwire creates a live executable tripwire that emits
// its marker message to stderr and exits 97 when invoked.
// PREVENTS: a vacuous tripwire whose reject assertions can never fire.
func TestParseWriteTripwireIsLiveExecutable(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	require.NoError(t, parseWriteTripwire(t.Context(), []string{"orphan-tripwire", "orphan-tripwire-executed"}))

	cmd := exec.CommandContext(t.Context(), filepath.Join(dir, "orphan-tripwire")) //nolint:gosec // executing the tripwire just created in t.TempDir()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	require.Error(t, err, "tripwire must exit nonzero when executed")

	exitErr, ok := errors.AsType[*exec.ExitError](err)
	require.True(t, ok, "tripwire must fail via exit status, not exec error: %v", err)
	assert.Equal(t, 97, exitErr.ExitCode())
	assert.Empty(t, stdout.String())
	assert.Equal(t, "orphan-tripwire-executed\n", stderr.String())

	// Invalid arguments are refused.
	require.Error(t, parseWriteTripwire(t.Context(), []string{"only-path"}))
	require.Error(t, parseWriteTripwire(t.Context(), []string{"../escape", "msg"}))
	require.Error(t, parseWriteTripwire(t.Context(), []string{"tripwire", "bad'quote"}))
}
