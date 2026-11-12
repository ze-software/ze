// Design: docs/guide/command-reference.md -- request plugin restart, the command the fixture is restarted by
// Related: plugin_fixture_restart_generation.go -- the drivers under test

package fixture

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRestartGenerationMode checks the plugin's one argument: none serves as
// before, ignore-first-stop arms the ignored stop, anything else is refused
// rather than read as either.
func TestRestartGenerationMode(t *testing.T) {
	ignore, err := restartGenerationMode(nil)
	require.NoError(t, err)
	assert.False(t, ignore)

	ignore, err = restartGenerationMode([]string{restartGenerationIgnoreStop})
	require.NoError(t, err)
	assert.True(t, ignore)

	for _, args := range [][]string{{"ignore-stop"}, {restartGenerationIgnoreStop, "extra"}} {
		_, err = restartGenerationMode(args)
		assert.Error(t, err, "arguments %q", args)
	}
}

// restartGenerationGoneDir makes a fresh directory current, as the runner does
// for a .ci step, and writes the pid marker, and the held marker when held is
// set, the way the ignoring generation leaves them.
func restartGenerationGoneDir(t *testing.T, pid int, held bool) {
	t.Helper()
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile(restartGenerationPIDMarker, []byte(strconv.Itoa(pid)+"\n"), 0o600))
	if held {
		require.NoError(t, os.WriteFile(restartGenerationHeldMarker, []byte("held\n"), 0o600))
	}
}

// exitedPID answers the pid of a child that has exited and been reaped.
func exitedPID(t *testing.T) int {
	t.Helper()
	child := exec.CommandContext(t.Context(), "true")
	require.NoError(t, child.Run())
	return child.Process.Pid
}

// TestRestartGenerationGoneAnswersForAnExitedProcess checks the passing
// verdict: the ignoring generation ran on past its close (the held marker) and
// its pid names no process.
func TestRestartGenerationGoneAnswersForAnExitedProcess(t *testing.T) {
	restartGenerationGoneDir(t, exitedPID(t), true)
	require.NoError(t, restartGenerationGone(t.Context(), nil))
}

// TestRestartGenerationGoneRefusesWithoutTheHold checks the vacuity guard: a
// process that is gone but never outlived its closed connection proves no
// kill, so the step refuses even though the pid names no process.
func TestRestartGenerationGoneRefusesWithoutTheHold(t *testing.T) {
	restartGenerationGoneDir(t, exitedPID(t), false)
	err := restartGenerationGone(t.Context(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no kill is proven")
}

// TestRestartGenerationGoneRefusesALiveProcess checks the defect verdict: a
// pid that still runs (this test's own) is refused once the wait ends. A short
// context ends Poll early, so the test does not wait out the whole bound.
func TestRestartGenerationGoneRefusesALiveProcess(t *testing.T) {
	restartGenerationGoneDir(t, os.Getpid(), true)
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	err := restartGenerationGone(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is still running")
}

// TestRestartGenerationGoneRefusesArguments checks that the step takes none.
func TestRestartGenerationGoneRefusesArguments(t *testing.T) {
	assert.Error(t, restartGenerationGone(t.Context(), []string{"extra"}))
}
