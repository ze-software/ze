// Design: docs/architecture/testing/runner-architecture.md -- peer stdin routing, the then=stop linger fence, and tmpfs materialization.
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

// tmpfsProofData is the non-executable control file's body. The tab, the two
// trailing spaces and the blank line are the bytes a trimming or re-joining
// copy would change, so an exact comparison sees them.
const tmpfsProofData = "first line\n\tindented with a tab\ntrailing spaces  \n\nlast line\n"

// tmpfsProofCI is the .ci body both routes read: an extensionless shim
// declared mode=755 and a control with no mode=. The shim copies itself and the
// control out of the work directory and reports the control's mode, because
// runTest removes that directory before it returns. out is an absolute
// directory outside the work directory.
func tmpfsProofCI(out, tail string) (shim, ci string) {
	shim = "#!/bin/sh\n" +
		"cat ./data.txt > '" + out + "/data.txt'\n" +
		"cat ./wg > '" + out + "/wg'\n" +
		"if [ -x ./data.txt ]; then echo executable; else echo plain; fi > '" + out + "/data.mode'\n"
	ci = "tmpfs=wg:mode=755:terminator=EOF_WG\n" + shim + "EOF_WG\n\n" +
		"tmpfs=data.txt:terminator=EOF_DATA\n" + tmpfsProofData + "EOF_DATA\n\n" +
		tail
	return shim, ci
}

// TestTmpfsContentAndModeReachBothRoutes materializes one pair of tmpfs blocks
// through each route a .ci takes, and checks exact bytes and the executable
// bit on what reached disk.
//
// VALIDATES: the parse suite (ParsingRunner.setupWorkDir) and the ordinary
// route every other suite takes (EncodingTests parse, then runTest, which
// rebuilds the tmpfs through tmpfsForRecord and executes cmd= lines) both keep
// a tmpfs block's content byte for byte and its declared mode. The shim is
// named `wg`, with no extension, so only the declared mode=755 can make it
// executable: a route that derives the mode from the path makes it 0644. The
// ordinary route proves it by executing the shim; the shim's own copies prove
// the bytes. The control has no mode= and must stay non-executable, so the
// mode assertion cannot pass by making everything executable.
// PREVENTS: a route that copies content alone, or rebuilds the file with a
// derived or fixed mode, which made `exec=./wg` fail with "permission denied"
// on one route while the other honored the block.
//
// MUTATION: (runner_exec.go, `v.AddFileMode(path, []byte(s), file.Mode)`, `v.AddFile(path, []byte(s))`): the ordinary subtest goes red, because `wg` is rebuilt 0644 and runTest fails to execute it.
// MUTATION: (runner_exec.go, `v.AddFileMode(path, []byte(s), file.Mode)`, `v.AddFileMode(path, []byte(s[:len(s)-1]), file.Mode)`): the ordinary subtest goes red, because every rebuilt file loses its last newline, so the bytes the shim copies out differ.
// MUTATION: (tmpfs.go, `if err := os.WriteFile(fullPath, f.Content, f.Mode); err != nil {`, `if err := os.WriteFile(fullPath, f.Content, 0o644); err != nil {`): the parse subtest goes red, because `wg` reaches disk without its owner execute bit.
// MUTATION: (tmpfs.go, `content.WriteByte('\n')`, `content.WriteByte(' ')`): the parse subtest goes red, because the bytes on disk differ from the blocks (the shim is compared first).
func TestTmpfsContentAndModeReachBothRoutes(t *testing.T) {
	t.Run("parse", func(t *testing.T) {
		dir := t.TempDir()
		shim, ci := tmpfsProofCI(t.TempDir(), "cmd=foreground:seq=1:exec=./wg\nexpect=exit:code=0\n")
		ciFile := filepath.Join(dir, "tmpfs-proof.ci")
		require.NoError(t, os.WriteFile(ciFile, []byte(ci), 0o600))

		pt := NewParsingTests(dir)
		test, err := pt.parseCIFile(ciFile)
		require.NoError(t, err)

		r := NewParsingRunner(pt, dir, "ze")
		workDir, err := r.setupWorkDir(test)
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(workDir) })

		shimInfo, err := os.Stat(filepath.Join(workDir, "wg"))
		require.NoError(t, err)
		require.NotZero(t, shimInfo.Mode().Perm()&0o100, "the parse route must write the mode=755 shim executable")
		shimBytes, err := os.ReadFile(filepath.Join(workDir, "wg")) //nolint:gosec // path under the test's work directory
		require.NoError(t, err)
		require.Equal(t, shim, string(shimBytes), "the parse route must write the shim's bytes unchanged")

		dataInfo, err := os.Stat(filepath.Join(workDir, "data.txt"))
		require.NoError(t, err)
		require.Zero(t, dataInfo.Mode().Perm()&0o111, "a block with no mode= and no executable extension stays non-executable")
		dataBytes, err := os.ReadFile(filepath.Join(workDir, "data.txt")) //nolint:gosec // path under the test's work directory
		require.NoError(t, err)
		require.Equal(t, tmpfsProofData, string(dataBytes), "the parse route must write the control's bytes unchanged")
	})

	t.Run("ordinary", func(t *testing.T) {
		dir := t.TempDir()
		out := t.TempDir()
		shim, ci := tmpfsProofCI(out, "option=timeout:value=10s\ncmd=foreground:seq=1:exec=./wg\nexpect=exit:code=0\n")
		ciFile := filepath.Join(dir, "tmpfs-proof.ci")
		require.NoError(t, os.WriteFile(ciFile, []byte(ci), 0o600))

		tests := NewEncodingTests(dir)
		rec, err := tests.parseAndAdd(ciFile)
		require.NoError(t, err)
		r, err := NewRunner(tests, dir)
		require.NoError(t, err)
		t.Cleanup(r.Cleanup)

		require.True(t, r.runTest(t.Context(), rec, &RunOptions{}),
			"the ordinary route must execute the mode=755 shim: %v", rec.Error)

		shimBytes, err := os.ReadFile(filepath.Join(out, "wg")) //nolint:gosec // path under t.TempDir
		require.NoError(t, err, "the shim ran but did not copy itself out")
		require.Equal(t, shim, string(shimBytes), "the ordinary route must write the shim's bytes unchanged")
		dataBytes, err := os.ReadFile(filepath.Join(out, "data.txt")) //nolint:gosec // path under t.TempDir
		require.NoError(t, err)
		require.Equal(t, tmpfsProofData, string(dataBytes), "the ordinary route must write the control's bytes unchanged")
		dataMode, err := os.ReadFile(filepath.Join(out, "data.mode")) //nolint:gosec // path under t.TempDir
		require.NoError(t, err)
		require.Equal(t, "plain\n", string(dataMode), "a block with no mode= and no executable extension stays non-executable")
	})
}
