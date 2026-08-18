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
// test passed while no BGP ever ran. Per ai/rules/evidence.md the
// harness must refuse a test it cannot run rather than run it vacuously.
// See plan/spec-fixit-redistribute-establishment-stall.md (D1/D2).

package runner

import (
	"fmt"
	"sort"
	"strconv"
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

// peerRejectionMarker is what a check-mode ze-peer prints when a reject= pattern
// fires. It exists because the success token is not always the peer's last word:
// under option=linger the peer announces success BEFORE the linger loop, since
// teardown is a kill and a post-Run print can be lost. A verdict that only asks
// whether the success token appeared therefore cannot see a rejection the linger
// loop found, which made every negative assertion held open by linger vacuous.
// The retraction is checked after the success token and overrides it.
const peerRejectionMarker = peer.RejectionMarker

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
// where ai/rules/evidence.md requires a guard to deny or speak.
func failedCheckPeers(declared int, peers []peerOutput) []string {
	var failed []string
	var tb textbuf.Buffer
	captured := 0
	for i := range peers {
		if !peers[i].checkMode {
			continue
		}
		captured++
		out := peers[i].combined()
		// The retraction is read first and overrides the success token, because
		// under option=linger the peer prints success before the linger loop and
		// only then discovers the rejection.
		if strings.Contains(out, peerSuccessToken) && !strings.Contains(out, peerRejectionMarker) {
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

// peerBlockNames returns the stdin blocks a .ci file hands to a ze-peer.
//
// Every peer block is named on a `cmd=...:exec=ze-peer ...:stdin=<name>` line.
// The block named "peer" is included unconditionally, because this loop is what
// puts a block's expect= lines on Record.Expects, and the non-orchestrated path
// (a .ci with no cmd= lines: 43 tracked files, 42 of them under
// test/exabgp-compat/encoding) then feeds them to ze-peer through
// writeExpectFile (runner_output.go). Validating it is not the
// same as it being READ verbatim: see blockPeerMode.
//
// KNOWN HOLE, not closed here. A .ci that HAS cmd= lines and a "peer" block that
// no ze-peer command names starts no peer at all, so its option= lines never
// reach Record.Options and its expect= lines are checked by nobody. Seven
// committed files are in that state, all test/plugin/redistribution-*.ci
// (redistribute-import-reject.ci among them, whose only cmd= is seq=2, the
// ze-peer at seq=1 having been deleted). None carries a reject=, and a reject=
// there is refused by validatePeerBlockRejects, so this spec's own directive is
// covered. Refusing the rest is correct and costs seven tests their authoring,
// which is why it is journalled (plan/journal/silent-fall-through.md) rather than
// folded into this closure. Their option=timeout does reach Record.Extra through
// the adopt-when-empty branch below, and each file's cmd=foreground timeout
// supersedes it (resolveOrchestratedTimeout, runner_exec_util.go).
// The list is sorted: a file with two bad blocks must name the same one on
// every run, or its error text is not something a fixture or a reader can rely
// on.
func peerBlockNames(r *Record) []string {
	seen := map[string]bool{"peer": true}
	for _, cmd := range r.RunCommands {
		if isZePeerExec(cmd.Exec) && cmd.Stdin != "" {
			seen[cmd.Stdin] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// blockPeerMode is the --mode of the ze-peer a block is handed to, and whether
// any ze-peer READS the block at all.
//
// read is true ONLY when a `cmd=...:exec=ze-peer ...:stdin=<name>` line names
// the block. That is the only path on which ze-peer receives the block's text.
//
// A file with NO cmd= lines takes the non-orchestrated path, and read is false
// there too. ze-peer's input on that path is built by writeExpectFile
// (runner_output.go) from Record.Options and Record.Expects, which this guard's
// ClaimPeer arm fills from `expect=` and `action=` lines only. A `reject=bgp` is
// ClaimPeer and matches neither prefix, so on that path it would reach nothing:
// the same silent drop this file exists to close. No committed .ci is affected:
// of the 43 tracked files with no cmd= line, the only one carrying any stdin
// block at all is test/parse/config-secret-roundtrip.ci, whose block is named
// `config`.
func blockPeerMode(r *Record, name string) (mode peer.Mode, read bool) {
	for _, cmd := range r.RunCommands {
		if isZePeerExec(cmd.Exec) && cmd.Stdin == name {
			return zePeerExecMode(cmd.Exec), true
		}
	}
	return peer.ModeCheck, false
}

// validatePeerBlockDirectives is the guard against a directive that reads as an
// assertion and asserts nothing.
//
// A stdin block is handed to ze-peer verbatim, so every line in it is read first
// by ze-peer's parser. peer.ClaimLine reports what that parser did with the line,
// and this function makes each answer an obligation:
//
//	ClaimPeer      ze-peer acts on it. The runner parses expect=/action= too, for
//	               its progress and failure reporting only, so a parse error there
//	               stays a debug log.
//	ClaimNarration ze-peer records it as documentation (cmd=). Nothing acts on it
//	               and nothing is lost.
//	otherwise      ze-peer read nothing, so the RUNNER must be able to parse the
//	               line where it stands. A parse error is the file's error.
//
// The last arm is the whole point. Before it existed the loop forwarded only
// expect= and action= lines and dropped the rest in silence: a reject= inside a
// peer block reached no parser at all, and a test carrying one read as if it
// checked a negative it never checked. Deriving the accepted set from the two
// parsers rather than from a third list is what stops the next directive being
// dropped the same way (ai/rules/evidence.md).
func (et *EncodingTests) validatePeerBlockDirectives(r *Record, ciFile string) error {
	for _, name := range peerBlockNames(r) {
		block, ok := r.StdinBlocks[name]
		if !ok {
			continue
		}
		mode, read := blockPeerMode(r, name)
		if err := et.validateOnePeerBlock(r, ciFile, name, string(block), mode, read); err != nil {
			return err
		}
	}
	return nil
}

func (et *EncodingTests) validateOnePeerBlock(r *Record, ciFile, name, block string, mode peer.Mode, read bool) error {
	blockLine := 0
	for line := range strings.SplitSeq(block, "\n") {
		blockLine++
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// option=env is stricter than the claim rule below, and deliberately so.
		// It seeds the environment of every process the test starts, ze
		// included, so a reader who takes the block for a scope reads the whole
		// test wrong. It is refused rather than applied where it stands, which
		// is the decision this repository already made and tested
		// (record_parse_test.go, TestParseAndAdd_EnvVarInsidePeerBlockRejected).
		if strings.HasPrefix(trimmed, "option=env:") {
			return fmt.Errorf("stdin=%s block line %d: %q sets the environment of every process "+
				"the test starts, not of this peer, so it must not be written inside a peer block. "+
				"Move it outside (above) the stdin=%s:terminator=... header",
				name, blockLine, trimmed, name)
		}
		claim, err := peer.ClaimLine(trimmed)
		if err != nil {
			return fmt.Errorf("stdin=%s block line %d: %q is rejected by ze-peer: %w",
				name, blockLine, trimmed, err)
		}
		switch claim {
		case peer.ClaimPeer:
			// ze-peer owns it. The runner parses expect= and action= as well, so
			// its progress output names the message a peer is waiting for; that
			// copy is reporting only and its errors stay a debug log.
			if strings.HasPrefix(trimmed, "expect=") || strings.HasPrefix(trimmed, "action=") {
				if err := et.parseLine(r, ciFile, trimmed); err != nil {
					recordLogger().Debug("parsing peer block line", "line", trimmed, "error", err)
				}
			}
		case peer.ClaimNarration:
			// Documentation of the command that produced the expected bytes.
		case peer.ClaimRunner:
			// The runner owns it, so it takes effect where it stands. That is
			// what makes a reject=stderr inside a peer block a live assertion
			// (test/plugin/logging-level-filter.ci).
			//
			// option=timeout is the exception. Its scope is the whole test, not
			// this peer, so a block MUST NOT overrule a file-level one: 450
			// committed blocks carry a timeout, and two of them disagree with
			// their own file-level value (test/plugin/authz-rpc-identity.ci
			// 20s vs 15s, test/plugin/bgp-capture-replay.ci 60s vs 55s).
			// It is parsed into a scratch record and adopted only when the file
			// declares none, which is the state of most of those 450. Parsing it
			// either way proves it is not a typo, which is what this guard is for.
			target := r
			timeoutLine := strings.HasPrefix(trimmed, "option=timeout:")
			if timeoutLine {
				target = &Record{Extra: map[string]string{}}
			}
			if err := et.parseLine(target, ciFile, trimmed); err != nil {
				return fmt.Errorf("stdin=%s block line %d: %q is a test-runner directive the runner "+
					"cannot parse: %w", name, blockLine, trimmed, err)
			}
			if timeoutLine && r.Extra["timeout"] == "" {
				r.Extra["timeout"] = target.Extra["timeout"]
			}
		default:
			if err := et.parseLine(r, ciFile, trimmed); err != nil {
				return fmt.Errorf("stdin=%s block line %d: %q is consumed by neither ze-peer nor the "+
					"test runner, so placing it in a stdin=%s block silently drops it: %w",
					name, blockLine, trimmed, name, err)
			}
		}
	}
	return validatePeerBlockRejects(name, block, mode, read)
}

// validatePeerBlockRejects refuses a wire rejection with no delivery to make it
// fire.
//
// A check peer stops reading a connection when that connection's expectations
// are complete, so a `reject=bgp:conn=N` on a connection that never carries an
// `expect=bgp:conn=N` is a negative assertion nothing can reach: it passes
// whether the daemon suppressed the route or never got as far as sending it.
//
// A delivery is NECESSARY and it is not sufficient. It bounds how long the
// rejection is checked. It cannot say whether the forbidden bytes would have
// arrived inside that window.
//
// The author owes the second half, and the contract is written down. Send the
// delivery LAST on that connection, so a leak of the governed route arrives
// ahead of it. Or hold the session open with `option=linger:value=true`.
// See docs/architecture/testing/ci-format.md, "reject=bgp", and
// ai/rules/interop-and-goal-validation.md, "Prove the test discriminates".
//
// Both halves of the line are read with peer.ParseRejectRule, the parser ze-peer
// itself uses. A malformed rule has already failed in ClaimLine, which
// validateOnePeerBlock runs on every line first, so the error branch below is
// the belt to that brace. The reason to call the shared parser HERE is the
// conn= value: the one this guard measures against is the one ze-peer enforces.
func validatePeerBlockRejects(name, block string, mode peer.Mode, read bool) error {
	delivered := make(map[int]bool)
	type rejectSite struct {
		conn int
		line int
	}
	var rejects []rejectSite
	blockLine := 0
	for line := range strings.SplitSeq(block, "\n") {
		blockLine++
		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, "expect=bgp:"); ok {
			delivered[connOfExpect(after)] = true
			continue
		}
		conn, _, isReject, err := peer.ParseRejectRule(trimmed)
		if err != nil {
			return fmt.Errorf("stdin=%s block line %d: %q: %w", name, blockLine, trimmed, err)
		}
		if isReject {
			rejects = append(rejects, rejectSite{conn: conn, line: blockLine})
		}
	}
	if len(rejects) > 0 && !read {
		return fmt.Errorf("stdin=%s block line %d: reject=bgp is read by ze-peer, and no "+
			"cmd= line in this file launches a ze-peer against stdin=%s, so the block reaches "+
			"nothing. Add cmd=background:seq=N:exec=ze-peer ...:stdin=%s, or move the rejection "+
			"to the block of a peer that runs", name, rejects[0].line, name, name)
	}
	if len(rejects) > 0 && mode != peer.ModeCheck {
		return fmt.Errorf("stdin=%s block line %d: reject=bgp is enforced only by a check-mode "+
			"ze-peer, and this block is handed to a %s peer. A non-check peer reads its "+
			"connections concurrently against one checker, so a rejection could not be "+
			"attributed to the connection its conn= names", name, rejects[0].line, mode)
	}
	for _, site := range rejects {
		if delivered[site.conn] {
			continue
		}
		return fmt.Errorf("stdin=%s block line %d: reject=bgp:conn=%d declares bytes that must never "+
			"arrive, and the block declares no expect=bgp:conn=%d to deliver anything on that "+
			"connection. The peer would finish before the forbidden bytes could arrive and the "+
			"rejection would pass without discriminating. Add the delivery that the suppressed "+
			"route is measured against, and send it LAST on that connection",
			name, site.line, site.conn, site.conn)
	}
	return nil
}

// connOfExpect reads the conn= number out of an `expect=bgp:` tail, answering 0
// when the directive names none. Expectations default to connection 1 nowhere in
// this file: an `expect=bgp:` with no conn= delivers on no numbered connection,
// so it satisfies no rejection.
func connOfExpect(tail string) int {
	for part := range strings.SplitSeq(tail, ":") {
		if after, ok := strings.CutPrefix(part, "conn="); ok {
			n, err := strconv.Atoi(after)
			if err != nil {
				return 0
			}
			return n
		}
	}
	return 0
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
