package fixture

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installAwaitAnswerZe puts a `ze` on PATH whose body is answer, a shell
// fragment run with $count set to how many times the fake has been called (1 on
// the first). Each call appends its arguments to the returned argv file, one
// line per call.
func installAwaitAnswerZe(t *testing.T, answer string) (argv string) {
	t.Helper()
	bin := t.TempDir()
	counter := filepath.Join(bin, "count")
	argv = filepath.Join(bin, "argv")
	script := "#!/bin/sh\n" +
		"count=$(cat '" + counter + "' 2>/dev/null || echo 0)\n" +
		"count=$((count + 1))\n" +
		"echo \"$count\" > '" + counter + "'\n" +
		"printf '%s\\n' \"$*\" >> '" + argv + "'\n" +
		answer + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "ze"), []byte(script), 0o700))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argv
}

// awaitAnswerCalls answers the argument line of every call the fake ze received.
func awaitAnswerCalls(t *testing.T, argv string) []string {
	t.Helper()
	raw, err := os.ReadFile(argv) //nolint:gosec // the test chose the path under its own temp dir
	require.NoError(t, err)
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

// TestDaemonAwaitAnswer proves the barrier a .ci places before steps that
// assert on a daemon state over SSH: it releases only when the client exits
// zero AND its stdout carries the needle, and otherwise fails naming the last
// answer. A .ci (the doctor-plugin-stopped functional test) trusts the
// release to mean both that the SSH listener accepts and that the state exists.
//
// VALIDATES: a refused connection (nonzero exit) and an answer without the
// needle are both waited past; a needle in a failed run does not release; an
// answer that never arrives is an error quoting the last run, never success.
// PREVENTS: a barrier that releases on the first answer, or on any exit zero,
// before the state the next step asserts on is visible.
// MUTATION: drop the `code != 0` guard and "needle on a failed run" releases;
// drop the strings.Contains and "refused then answers" releases on call 3.
func TestDaemonAwaitAnswer(t *testing.T) {
	t.Run("refused then answers", func(t *testing.T) {
		argv := installAwaitAnswerZe(t, `if [ "$count" -le 2 ]; then echo "connection refused" >&2; exit 1; fi
if [ "$count" -eq 3 ]; then echo '{"warnings": []}'; exit 0; fi
echo '{"warnings": [{"code": "doctor-plugin-stopped"}]}'`)
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		require.NoError(t, daemonAwaitAnswer(ctx, []string{"doctor-plugin-stopped", "show warnings | json"}))
		calls := awaitAnswerCalls(t, argv)
		assert.Len(t, calls, 4, "the barrier must keep asking until the needle appears, and stop then")
		for _, call := range calls {
			assert.Equal(t, "cli -c show warnings | json", call)
		}
	})
	t.Run("needle on a failed run", func(t *testing.T) {
		installAwaitAnswerZe(t, `echo "doctor-plugin-stopped"; exit 1`)
		ctx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
		defer cancel()
		err := daemonAwaitAnswer(ctx, []string{"doctor-plugin-stopped", "show warnings"})
		require.Error(t, err, "a run that exited nonzero released the barrier")
		assert.Contains(t, err.Error(), "the last run exited 1")
	})
	t.Run("never holds", func(t *testing.T) {
		installAwaitAnswerZe(t, `echo '{"warnings": []}'`)
		ctx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
		defer cancel()
		err := daemonAwaitAnswer(ctx, []string{"doctor-plugin-stopped", "show warnings"})
		require.Error(t, err, "an answer without the needle released the barrier")
		assert.Contains(t, err.Error(), `never answered with "doctor-plugin-stopped"`)
		assert.Contains(t, err.Error(), `{\"warnings\": []}`, "the error must quote the last answer")
	})
	t.Run("usage", func(t *testing.T) {
		require.Error(t, daemonAwaitAnswer(t.Context(), []string{"only-a-needle"}))
		require.Error(t, daemonAwaitAnswer(t.Context(), []string{"", "show warnings"}))
	})
}
