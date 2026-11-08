// Design: docs/architecture/testing/runner-architecture.md -- peer stdin routing and the then=stop linger fence.
// Related: process_lifecycle_test.go -- the owned-process proofs over the same runTest path.

package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// lingerPeerBlock is a check peer's expectation block that declares linger.
// It carries no $PORT, so the bytes the peer receives equal it exactly.
const lingerPeerBlock = "option=linger:value=true\nexpect=bgp:conn=1:seq=1:hex=00\n"

// lingerFenceNeedle is the line the stand-in daemon prints on stderr, which
// the record's await=stderr fence waits for.
const lingerFenceNeedle = "linger-fence-needle"

// TestPeerLingerSurvivesStdinRouting runs a whole record through runTest, the
// route `le test <suite>` takes, once per stdin route a ze-peer line can take.
//
// VALIDATES: the runner reads a check peer's option=linger from the .ci block
// whether that block reaches the peer as the file the runner appends to argv
// (`le test peer --port $PORT`) or on standard input (`le test peer ... -`).
// The stand-in peer binds and then never prints its success token, so only a
// peer the runner knows to linger holds the then=stop fence: the run fails with
// the fence's "lingering check peer(s)" refusal. A peer read as non-lingering
// is not waited, the daemon is stopped at once, and the run fails later for
// another reason, which this test refuses. The stand-in also records what it
// received, so each subtest proves its route was the one it names.
// PREVENTS: inferring the peer's linger from what is piped. A file-routed peer
// pipes nothing, so it would lose linger, and a reload test would stop its
// daemon while the peer was still owed routes.
//
// MUTATION: (runner_exec.go, `peerLingers(stdinContent),`, `stdinPiped && peerLingers(stdinContent),`): the file subtest goes red, because the error is no longer the linger refusal: a file-routed peer pipes nothing.
// MUTATION: (runner_exec.go, `args = append(args, tmpFile.Name())`, `args, stdinContent = append(args, tmpFile.Name()), nil`): the file subtest goes red, because the block is dropped once written to the file, so linger is read from nothing.
func TestPeerLingerSurvivesStdinRouting(t *testing.T) {
	for _, tc := range []struct {
		name  string
		exec  string
		piped bool
	}{
		{name: "file", exec: "le test peer --port $PORT", piped: false},
		{name: "pipe", exec: "le test peer --port $PORT -", piped: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seen := t.TempDir()
			argvFile := filepath.Join(seen, "argv")
			stdinFile := filepath.Join(seen, "stdin")
			blockFile := filepath.Join(seen, "block")

			r, rec := lastHelperRecord(t, "linger-"+tc.name, "20s", &RunCommand{
				Mode: modeBackground, Seq: 1, Exec: tc.exec, Stdin: "peer",
			})
			rec.RunCommands = append(rec.RunCommands, RunCommand{
				Mode: modeForeground, Seq: 2, Exec: "ze -", Stdin: "ze-bgp",
			})
			rec.StdinBlocks = map[string][]byte{
				"peer":   []byte(lingerPeerBlock),
				"ze-bgp": []byte("bgp {\n}\n"),
			}
			rec.AwaitStderr = []string{lingerFenceNeedle}
			rec.AwaitStderrTimeout = "300ms"
			rec.AwaitThenStop = true

			// The stand-in peer records its argv, its standard input and the
			// file its last argument names (the runner deletes that file at
			// cleanup), binds, and then holds without ever completing.
			r.lePath = filepath.Join(seen, "peer")
			peerScript := "#!/bin/sh\ntrap 'exit 0' TERM\n" +
				"printf '%s\\n' \"$@\" > '" + argvFile + "'\n" +
				"cat > '" + stdinFile + "'\n" +
				"for arg; do last=$arg; done\n" +
				"if [ -f \"$last\" ]; then cat \"$last\" > '" + blockFile + "'; fi\n" +
				"echo '" + peerListeningPattern + " 127.0.0.1'\n" +
				"while :; do sleep 0.05; done\n"
			require.NoError(t, os.WriteFile(r.lePath, []byte(peerScript), 0o700))

			// The stand-in daemon prints the fence needle and runs until SIGTERM.
			r.zePath = filepath.Join(seen, "daemon")
			daemonScript := "#!/bin/sh\ntrap 'exit 0' TERM\necho '" + lingerFenceNeedle + "' >&2\nwhile :; do sleep 0.05; done\n"
			require.NoError(t, os.WriteFile(r.zePath, []byte(daemonScript), 0o700))

			passed := r.runTest(t.Context(), rec, &RunOptions{})
			require.False(t, passed, "a lingering peer that never completes must fail the then=stop fence")
			require.Error(t, rec.Error)
			require.Contains(t, rec.Error.Error(), "lingering check peer(s)",
				"the runner did not read option=linger from the %s-routed block", tc.name)

			argv, err := os.ReadFile(argvFile) //nolint:gosec // path under t.TempDir
			require.NoError(t, err)
			words := strings.Fields(string(argv))
			require.Equal(t, tc.piped, strings.Contains("\n"+string(argv), "\n-\n"),
				"argv %q: a `-` is what selects the pipe route", words)

			stdin, err := os.ReadFile(stdinFile) //nolint:gosec // path under t.TempDir
			require.NoError(t, err)
			block, blockErr := os.ReadFile(blockFile) //nolint:gosec // path under t.TempDir
			if tc.piped {
				require.Equal(t, lingerPeerBlock, string(stdin), "the pipe route writes the block to standard input")
				require.ErrorIs(t, blockErr, os.ErrNotExist, "the pipe route appends no file to argv")
				return
			}
			require.Empty(t, stdin, "the file route pipes nothing")
			require.NoError(t, blockErr, "the file route appends the block's file to argv")
			require.Equal(t, lingerPeerBlock, string(block))
		})
	}
}
