// Design: docs/architecture/testing/ci-format.md — .ci peer blocks and their consumers
// Related: record_parse.go — parseAndAdd calls validatePeerBlocks at parse time
// Related: runner_exec.go — the bind barrier and isSelfValidated call into this file
//
// The ze-peer contract: what a .ci file must declare for its ze-peer to be able
// to test anything (validatePeerBlocks, at parse time), the failure surfaced when
// a peer never binds (peerBindFailure, at run time), and the decision of whether
// a test's BGP exchange governs its result at all (isSelfValidated).
//
// All three exist because of one defect class. A check-mode ze-peer with no
// expectation exits 1 BEFORE binding a listening socket (the "no test data"
// branch of ze-test peer). ze then dials a dead port, gets connection refused,
// and backs off 5->10->20->40s, which reads as a BGP establishment stall. An
// expect=exit:code=0 on the same test skipped every BGP-level assertion, so the
// test passed while no BGP ever ran. Per ai/rules/fail-closed-guards.md the
// harness must refuse a test it cannot run rather than run it vacuously.
// See plan/spec-fixit-redistribute-establishment-stall.md (D1/D2).

package runner

import (
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/peer"
)

// peerNoTestDataMessage is the stderr ze-peer emits in check mode with no
// expectations, from the "no test data" branch of ze-test peer's cmd_peer.go.
const peerNoTestDataMessage = "no test data available to test against"

// zePeerBin is the command name the runner recognizes as a BGP test peer.
const zePeerBin = "ze-peer"

// hasCheckPeer reports whether any command launches a check-mode ze-peer.
//
// Only a check-mode peer validates the messages it receives and reports
// "successful" on a clean exchange. sink/echo/inject peers are scaffolding: they
// accept whatever arrives and loop until killed, so they never report completion
// and cannot govern a test's result. Keying the peer-governs rule off ze-peer's
// mere presence would fail every scaffolding test while asserting nothing.
func hasCheckPeer(cmds []RunCommand) bool {
	for _, cmd := range cmds {
		if isCheckPeerExec(cmd.Exec) {
			return true
		}
	}
	return false
}

// isCheckPeerExec reports whether an exec value launches a check-mode ze-peer --
// the only peer kind that validates what it receives and reports peerSuccessToken.
func isCheckPeerExec(exec string) bool {
	return isZePeerExec(exec) && zePeerExecMode(exec) == peer.ModeCheck
}

// countCheckPeers is how many check-mode peers the .ci declares, which
// failedCheckPeers compares against how many actually produced a capture.
func countCheckPeers(cmds []RunCommand) int {
	n := 0
	for _, cmd := range cmds {
		if isCheckPeerExec(cmd.Exec) {
			n++
		}
	}
	return n
}

// peerSuccessToken is what a check-mode ze-peer prints on a clean exchange
// (internal/test/cli/cmd_bgp.go).
const peerSuccessToken = "successful"

// peerLabel names a peer in a failure message, preferring whatever the .ci author
// actually wrote: an explicit cmd name, else the stdin block the peer's
// expectations came from, else the command's sequence number.
func peerLabel(cmd RunCommand) string {
	if cmd.Name != "" {
		return cmd.Name
	}
	var tb textbuf.Buffer
	if cmd.Stdin != "" {
		return tb.Str("stdin=").Str(cmd.Stdin).String()
	}
	return tb.Str("cmd seq=").Int(int64(cmd.Seq)).String()
}

// failedCheckPeers names every check-mode peer that did not report a clean
// exchange, plus any check-mode peer that produced no capture at all.
//
// EVERY check-mode peer must succeed. The verdict used to be one
// strings.Contains over all peers' output concatenated into rec.PeerOutput
// (runner_exec.go), so in a multi-peer test the first peer's success masked every
// other peer's failure: a run where the destination peer logged "connection
// closed before completion" was still reported PASS because the source peer had
// printed peerSuccessToken. Two suite tests (forward-overflow-two-tier,
// role-otc-unicast-scope) read as "flaky, passes 1 in 10" for exactly this reason
// while failing deterministically. That is the vacuous-pass shape this file
// exists to close (see the header), one layer further out, so the fix belongs
// here and not at the call site.
//
// declared is the number of check-mode peers the .ci asked for. A shortfall is
// itself a failure: a check peer that never produced a peerOutput cannot have
// validated anything, and reporting "no failures" for it would fail open exactly
// where ai/rules/fail-closed-guards.md requires a guard to deny or speak.
func failedCheckPeers(declared int, peers []peerOutput) []string {
	var failed []string
	var tb textbuf.Buffer
	captured := 0
	for i := range peers {
		if !peers[i].checkMode {
			continue
		}
		captured++
		if strings.Contains(peers[i].combined(), peerSuccessToken) {
			continue
		}
		label := peers[i].label
		if label == "" {
			label = tb.Reset().Str("check peer #").Int(int64(captured)).String()
		}
		failed = append(failed, label)
	}
	if captured < declared {
		failed = append(failed, tb.Reset().
			Int(int64(declared-captured)).Str(" of ").Int(int64(declared)).
			Str(" check-mode peers never started").String())
	}
	return failed
}

// isSelfValidated reports whether a test's result is governed entirely by the
// exit/output/file/logging assertions evaluated in the runner, rather than by the
// BGP exchange a check-mode ze-peer validates.
//
// A test with a check-mode ze-peer is NEVER self-validated. The peer's exchange
// governs, and other assertions are additive. Making an exit-code or output
// assertion able to disable the peer check is what let a test pass with no BGP
// session at all.
func isSelfValidated(rec *Record, hasCheckPeer bool) bool {
	if hasCheckPeer {
		return false
	}
	hasOutputAssertion := len(rec.ExpectStderrMatch) > 0 ||
		len(rec.ExpectStdoutMatch) > 0 ||
		len(rec.ExpectStdoutNotMatch) > 0 ||
		len(rec.ExpectStdoutRegex) > 0 ||
		len(rec.RejectStdoutRegex) > 0 ||
		len(rec.ExpectStderr) > 0 || len(rec.RejectStderr) > 0 ||
		len(rec.ExpectSyslog) > 0 || len(rec.RejectSyslog) > 0 ||
		len(rec.FileChecks) > 0 ||
		len(rec.HTTPChecks) > 0
	return rec.ExpectExitCode != nil || hasOutputAssertion
}

// peerBindFailure builds the error for a ze-peer that never printed its
// "listening on" readiness token. It names the "no test data" case explicitly:
// that is the one cause an author can fix from the .ci file alone, and leaving it
// buried in a stderr dump is what let it hide.
// timeout is the deadline actually enforced by the caller, not the authored 5s
// base: a parallel run widens it by ParallelTimeoutHeadroom, and an error that
// names a budget the run did not use sends the reader hunting for a 5s limit
// that was never applied.
func peerBindFailure(timeout time.Duration, stderr, stdout string) error {
	if strings.Contains(stderr, peerNoTestDataMessage) {
		return fmt.Errorf("ze-peer exited without binding: %q. Its peer block declares no "+
			"ze-peer-consumed expectation (expect=bgp:, or action=send/notification/rewrite/"+
			"close/sighup/sigterm), so ze-peer had nothing to check and never listened, and "+
			"every ze dial hit connection refused. Note expect=json is validated by the test "+
			"runner and does NOT reach ze-peer",
			peerNoTestDataMessage)
	}
	return fmt.Errorf("peer did not start listening within %s (stderr=%q, stdout=%q)", timeout, stderr, stdout)
}

// validatePeerBlocks rejects a record whose check-mode ze-peer would exit before
// binding because its peer block declares nothing ze-peer consumes.
//
// This mirrors, at parse time, ze-peer's own runtime bail: it refuses to run when
// Config.Expect is empty, and Config.Expect is populated ONLY from lines
// peer.ConsumesLine accepts. A block of expect=json alone therefore yields an
// empty Expect. Rejecting here names the file and the remedy instead of leaving
// the author a connection-refused backoff to explain.
//
// Only check mode is validated: sink/echo/inject peers legitimately carry no
// expectations, and ze-peer's guard is likewise check-mode only.
func validatePeerBlocks(r *Record) error {
	for _, cmd := range r.RunCommands {
		if !isZePeerExec(cmd.Exec) || zePeerExecMode(cmd.Exec) != peer.ModeCheck {
			continue
		}
		if cmd.Stdin == "" {
			// No expect file argument: ze-peer rejects this itself, and the shape
			// does not occur in the suite.
			continue
		}
		block, ok := r.StdinBlocks[cmd.Stdin]
		if !ok {
			// Reported at run time as `stdin block %q not found`.
			continue
		}
		if peerBlockHasConsumedDirective(string(block)) {
			continue
		}
		return fmt.Errorf("stdin=%s block: check-mode ze-peer (cmd seq=%d) declares no "+
			"ze-peer-consumed expectation, so it exits with %q before binding a listening "+
			"socket and the test can only pass vacuously. Add an "+
			"expect=bgp:conn=N:seq=N:hex=... (or an action=send/notification/rewrite/close/"+
			"sighup/sigterm) to the block, or run the peer with --mode sink. Note expect=json "+
			"is consumed by the test runner, NOT by ze-peer, so it does not make the peer "+
			"listen. See plan/spec-fixit-redistribute-establishment-stall.md",
			cmd.Stdin, cmd.Seq, peerNoTestDataMessage)
	}
	return nil
}

// peerBlockHasConsumedDirective reports whether any line of a peer block is
// forwarded to ze-peer. peer.ConsumesLine is the producer's own definition of
// that set, so this cannot drift from what ze-peer actually collects.
func peerBlockHasConsumedDirective(block string) bool {
	for line := range strings.SplitSeq(block, "\n") {
		if peer.ConsumesLine(line) {
			return true
		}
	}
	return false
}

// isZePeerExec reports whether a cmd= exec value launches ze-peer. It matches the
// command word so a helper whose arguments mention ze-peer does not false-positive.
func isZePeerExec(exec string) bool {
	fields := strings.Fields(exec)
	return len(fields) > 0 && fields[0] == zePeerBin
}

// zePeerExecMode extracts the peer mode from a ze-peer exec value, defaulting to
// ModeCheck exactly as ze-peer's --mode flag does.
func zePeerExecMode(exec string) peer.Mode {
	fields := strings.Fields(exec)
	for i, f := range fields {
		if name, ok := strings.CutPrefix(f, "--mode="); ok {
			mode, _ := peer.ParseMode(name)
			return mode
		}
		if f == "--mode" && i+1 < len(fields) {
			mode, _ := peer.ParseMode(fields[i+1])
			return mode
		}
	}
	return peer.ModeCheck
}
