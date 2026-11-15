// Design: docs/architecture/testing/ci-format.md -- expect=exit beside a check-mode peer.
// Related: one_off_proof_test.go -- the stand-in peer and daemon this test reuses.

package runner

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAssertedExitBesideCheckPeer runs a whole record through runTest with a
// check-mode peer and a foreground subject whose exit the file asserts with
// expect=exit:code=1, once for each arm of runOrchestrated that supplies that
// exit: a foreground ze daemon, a quick-exit ze command and a foreground helper.
//
// VALIDATES: an asserted non-zero exit that matched is the outcome under test,
// so the check peer's exchange alone decides the peer verdict. Three outcomes,
// per arm:
//   - the peer succeeds and the subject exits 1: the test PASSES;
//   - the peer fails and the subject exits 1: the test FAILS on peer-exchange,
//     with the exit-code step passed, so a fix that skips the peer check when an
//     exit is asserted goes red here;
//   - the peer succeeds and the subject exits 0: the test FAILS on exit-code.
//
// The stand-in peer binds, waits until the subject has run (a marker file the
// subject writes), then prints the check peer's success token or a failure and
// exits. The stand-in subject writes the marker and exits with the code the
// case names. No Ze daemon and no port beyond the peer's $PORT are involved.
// PREVENTS: the check-peer branch reading the asserted exit's *exec.ExitError
// as a failed run, which recorded `peer-exchange -> unknown` for every test
// pairing a check peer with expect=exit:code=1 even when the peer printed
// "successful" (test/plugin/failure-policy-fatal-live-peer.ci).
//
// MUTATION: (runner_exec.go, `err = nil`, `_ = err`): every "peer succeeds, exit 1" subtest goes red, because the matched exit error reaches the peer branch as a failed run.
// MUTATION: (runner_exec.go, `if !isSelfValidated(rec, hasCheckPeer(cmds)) {`, `if rec.ExpectExitCode == nil && !isSelfValidated(rec, hasCheckPeer(cmds)) {`): every "peer fails, exit 1" subtest goes red, because the failed exchange is no longer judged.
func TestAssertedExitBesideCheckPeer(t *testing.T) {
	arms := []struct {
		name string
		exec string
		arm  stepArm
	}{
		{name: "daemon", exec: "ze start ze-bgp.conf", arm: stepArmDaemon},
		{name: "quick-ze", exec: "ze config validate ze-bgp.conf", arm: stepArmQuickZe},
		{name: "helper", exec: "", arm: stepArmHelper}, // the helper script's absolute path, set per subtest
	}
	outcomes := []struct {
		name      string
		peerOK    bool
		exitCode  int
		passed    bool
		failedOn  string
		failure   string
		passSteps []string
	}{
		{name: "peer succeeds, exit 1", peerOK: true, exitCode: 1, passed: true, passSteps: []string{"exit-code", "peer-exchange"}},
		{name: "peer fails, exit 1", peerOK: false, exitCode: 1, passed: false, failedOn: "peer-exchange", passSteps: []string{"exit-code"}},
		{name: "peer succeeds, exit 0", peerOK: true, exitCode: 0, passed: false, failedOn: "exit-code", failure: failTypeExitCodeMismatch},
	}
	for _, arm := range arms {
		for _, outcome := range outcomes {
			t.Run(arm.name+"/"+outcome.name, func(t *testing.T) {
				seen := t.TempDir()
				marker := filepath.Join(seen, "subject-ran")

				// The subject: the stand-in ze for the two ze arms, a script run
				// by absolute path for the helper arm. Both write the marker the
				// peer waits on, then exit with the case's code.
				subject := filepath.Join(seen, "subject")
				subjectScript := "#!/bin/sh\n: > '" + marker + "'\nexit " + strconv.Itoa(outcome.exitCode) + "\n"
				require.NoError(t, os.WriteFile(subject, []byte(subjectScript), 0o700))
				subjectExec := arm.exec
				if arm.arm == stepArmHelper {
					subjectExec = subject
				}
				parts, err := SplitCommand(subjectExec)
				require.NoError(t, err)
				require.Equal(t, arm.arm, classifyStepArm(modeForeground, parts),
					"%q must run in the %s arm, or this subtest proves another one", subjectExec, arm.name)

				r, rec := lastHelperRecord(t, "exit-beside-peer", "10s", &RunCommand{
					Mode: modeBackground, Seq: 1, Exec: "le test peer --port $PORT", Stdin: "peer",
				})
				rec.RunCommands = append(rec.RunCommands, RunCommand{Mode: modeForeground, Seq: 2, Exec: subjectExec})
				rec.StdinBlocks = map[string][]byte{"peer": []byte("expect=bgp:conn=1:seq=1:hex=00\n")}
				expected := 1
				rec.ExpectExitCode = &expected
				r.zePath = subject

				// The stand-in check peer: binds, waits for the subject, then
				// reports the exchange the case names.
				verdict := "echo '" + peerSuccessToken + "'\nexit 0\n"
				if !outcome.peerOK {
					verdict = "echo 'connection closed before completion' >&2\nexit 1\n"
				}
				r.lePath = filepath.Join(seen, "peer")
				peerScript := "#!/bin/sh\ntrap 'exit 0' TERM\n" +
					"echo '" + peerListeningPattern + " 127.0.0.1'\n" +
					"while [ ! -f '" + marker + "' ]; do sleep 0.05; done\n" +
					verdict
				require.NoError(t, os.WriteFile(r.lePath, []byte(peerScript), 0o700))

				passed := r.runTest(t.Context(), rec, &RunOptions{})
				require.Equal(t, outcome.passed, passed,
					"error=%v failure=%q failed-peers=%v steps=%+v", rec.Error, rec.FailureType, rec.FailedPeers, rec.StepTrace)

				steps := map[string]bool{}
				for _, step := range rec.StepTrace {
					if step.Assert != "" {
						steps[step.Assert] = step.Passed
					}
				}
				for _, name := range outcome.passSteps {
					stepPassed, ok := steps[name]
					require.True(t, ok, "step %q was never judged: %+v", name, rec.StepTrace)
					require.True(t, stepPassed, "step %q failed: %+v", name, rec.StepTrace)
				}
				if outcome.failedOn == "" {
					require.Empty(t, rec.FailedPeers)
					return
				}
				stepPassed, ok := steps[outcome.failedOn]
				require.True(t, ok, "the failing step %q was never judged: %+v", outcome.failedOn, rec.StepTrace)
				require.False(t, stepPassed, "step %q passed: %+v", outcome.failedOn, rec.StepTrace)
				if outcome.failure != "" {
					require.Equal(t, outcome.failure, rec.FailureType)
					require.ErrorContains(t, rec.Error, "expected exit code 1, got 0")
				}
				if outcome.failedOn == "peer-exchange" {
					require.Equal(t, []string{"stdin=peer"}, rec.FailedPeers)
					require.NotEqual(t, failTypeExitCodeMismatch, rec.FailureType)
				}
			})
		}
	}
}
