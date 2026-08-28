package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/peer"
)

// peerContractCI builds a .ci file with the given peer block body and ze-peer
// exec line, writes it plus a minimal config, and returns the .ci path.
func peerContractCI(t *testing.T, exec, block string) string {
	t.Helper()
	tmpDir := t.TempDir()
	ciFile := filepath.Join(tmpDir, "contract.ci")
	confFile := filepath.Join(tmpDir, "test.conf")

	ciContent := "option=file:path=test.conf\n" +
		"stdin=peer:terminator=EOF_PEER\n" +
		block +
		"EOF_PEER\n" +
		"cmd=background:seq=1:exec=" + exec + ":stdin=peer\n"

	require.NoError(t, os.WriteFile(ciFile, []byte(ciContent), 0o600))
	require.NoError(t, os.WriteFile(confFile, []byte(minimalConfig), 0o600))
	return ciFile
}

// TestParseAndAdd_CheckPeerWithOnlyJSONExpectRejected reproduces the exact shape
// of test/plugin/bgp-redistribute-announce.ci: a check-mode ze-peer whose peer
// block's only expectation is expect=json. expect=json is consumed by the test
// runner, never forwarded to ze-peer, so ze-peer's Config.Expect is empty and it
// exits 1 before binding. ze then dials a dead port for the whole test.
//
// VALIDATES: F2 — the parser refuses a peer block that cannot test anything.
// PREVENTS: The vacuous-green class that hid a non-running BGP session behind an
// exit-code assertion (spec-fixit-redistribute-establishment-stall, D1).
func TestParseAndAdd_CheckPeerWithOnlyJSONExpectRejected(t *testing.T) {
	ResetNickCounter()

	ciFile := peerContractCI(t, "ze-peer --port $PORT",
		"option=timeout:value=5s\n"+
			`expect=json:conn=1:seq=1:json={ "type": "update" }`+"\n")

	et := NewEncodingTests(filepath.Dir(ciFile))
	_, err := et.parseAndAdd(ciFile)
	require.Error(t, err, "a check-mode peer with only expect=json must be rejected at parse time")

	msg := err.Error()
	assert.Contains(t, msg, "no test data available to test against",
		"error should quote the ze-peer exit that this guard mirrors")
	assert.Contains(t, msg, "expect=json",
		"error should name expect=json as the directive that does not reach ze-peer")
	assert.Contains(t, msg, "expect=bgp",
		"error should name the remedy directive")
}

// TestParseAndAdd_CheckPeerWithNoExpectAtAllRejected covers the second vacuous
// shape found in the blast radius: a peer block carrying only options, with no
// expectation of any kind.
//
// VALIDATES: F2 — rejection keys off "no peer-consumed directive", not off the
// presence of expect=json specifically.
func TestParseAndAdd_CheckPeerWithNoExpectAtAllRejected(t *testing.T) {
	ResetNickCounter()

	ciFile := peerContractCI(t, "ze-peer --port $PORT",
		"option=timeout:value=5s\noption=asn:value=65533\n")

	et := NewEncodingTests(filepath.Dir(ciFile))
	_, err := et.parseAndAdd(ciFile)
	require.Error(t, err, "a check-mode peer with no expectation at all must be rejected")
	assert.Contains(t, err.Error(), "no test data available to test against")
}

// TestParseAndAdd_CheckPeerWithBGPExpectAccepted is the control: a peer block
// with a real ze-peer-consumed expectation must still parse.
//
// VALIDATES: F2 — the guard is not over-broad.
// PREVENTS: Rejecting the ~470 .ci files that already assert correctly.
func TestParseAndAdd_CheckPeerWithBGPExpectAccepted(t *testing.T) {
	ResetNickCounter()

	ciFile := peerContractCI(t, "ze-peer --port $PORT",
		"option=timeout:value=5s\n"+
			"expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304\n"+
			`expect=json:conn=1:seq=1:json={ "type": "keepalive" }`+"\n")

	et := NewEncodingTests(filepath.Dir(ciFile))
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err, "a peer block with expect=bgp must parse")
}

// TestParseAndAdd_CheckPeerWithActionSendAccepted verifies the guard accepts
// every ze-peer-consumed directive, not just expect=bgp. A peer block that only
// sends raw bytes still gives ze-peer work to do, so it binds and listens.
//
// VALIDATES: F2 — the accepted set matches peer.ConsumesLine exactly.
func TestParseAndAdd_CheckPeerWithActionSendAccepted(t *testing.T) {
	ResetNickCounter()

	ciFile := peerContractCI(t, "ze-peer --port $PORT",
		"action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304\n")

	et := NewEncodingTests(filepath.Dir(ciFile))
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err, "a peer block with action=send must parse")
}

// TestParseAndAdd_SinkPeerWithoutExpectAccepted verifies the guard is scoped to
// check mode. A sink peer absorbs anything and legitimately declares no
// expectation; cmd_peer.go's own "no test data" bail is check-mode only, so this
// guard must be too.
//
// VALIDATES: F2 — mode scoping mirrors cmd_peer.go.
// PREVENTS: Breaking the multi-peer sink pattern documented in ci-format.md.
func TestParseAndAdd_SinkPeerWithoutExpectAccepted(t *testing.T) {
	ResetNickCounter()

	ciFile := peerContractCI(t, "ze-peer --bind 127.0.0.2 --mode sink --port $PORT",
		"option=tcp_connections:value=1\n")

	et := NewEncodingTests(filepath.Dir(ciFile))
	_, err := et.parseAndAdd(ciFile)
	require.NoError(t, err, "a sink-mode peer needs no expectation")
}

// TestZePeerExecMode covers mode extraction from the exec string, including the
// default. Getting this wrong in either direction is costly: default-to-sink
// would disable the F2 guard everywhere, and misreading --mode sink as check
// would reject valid tests.
func TestZePeerExecMode(t *testing.T) {
	tests := []struct {
		name string
		exec string
		want peer.Mode
	}{
		{"default is check", "ze-peer --port $PORT", peer.ModeCheck},
		{"explicit check", "ze-peer --mode check --port $PORT", peer.ModeCheck},
		{"sink", "ze-peer --bind 127.0.0.2 --mode sink --port $PORT", peer.ModeSink},
		{"equals form", "ze-peer --mode=sink --port $PORT", peer.ModeSink},
		{"echo", "ze-peer --mode echo --port $PORT", peer.ModeEcho},
		{"inject", "ze-peer --mode inject --port $PORT --inject-count 5", peer.ModeInject},
		{"trailing --mode with no value defaults to check", "ze-peer --port $PORT --mode", peer.ModeCheck},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, zePeerExecMode(tc.exec))
		})
	}
}

// TestIsZePeerExec verifies peer detection matches on the command word, so a
// helper script whose arguments mention ze-peer is not mistaken for one.
func TestIsZePeerExec(t *testing.T) {
	assert.True(t, isZePeerExec("ze-peer --port 1790"))
	assert.False(t, isZePeerExec("ze bgp server -"))
	assert.False(t, isZePeerExec("fixture-driver --wait-for ze-peer"))
	assert.False(t, isZePeerExec(""))
}

// TestPeerBindFailure_NamesNoTestDataCause verifies F3: when ze-peer exits
// because it had nothing to check, the runner's failure says so in terms the
// author can act on, instead of burying the cause in a stderr dump.
//
// VALIDATES: F3 — "peer never bound" is a first-class, self-explaining failure.
func TestPeerBindFailure_NamesNoTestDataCause(t *testing.T) {
	err := peerBindFailure(5*time.Second, "no test data available to test against\n", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exited without binding")
	assert.Contains(t, err.Error(), "expect=json")
	assert.Contains(t, err.Error(), "does NOT reach ze-peer")
}

// TestPeerBindFailure_GenericWhenCauseUnknown verifies the fallback still
// reports both streams for an unrecognized bind failure (e.g. port in use), and
// that it names the deadline the caller actually enforced.
//
// VALIDATES: the reported budget is the caller's, not the authored 5s base.
// PREVENTS: a parallel run (which widens the bind budget by
// ParallelTimeoutHeadroom) reporting "within 5s" for a limit it never applied,
// sending the reader hunting for a 5s timeout that was not in force.
func TestPeerBindFailure_GenericWhenCauseUnknown(t *testing.T) {
	err := peerBindFailure(15*time.Second, "listen: address already in use", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not start listening within 15s")
	assert.NotContains(t, err.Error(), "within 5s")
	assert.Contains(t, err.Error(), "address already in use")
}

// TestHasCheckPeer verifies which peers can govern a test's result. Only a
// check-mode peer validates messages and reports "successful"; sink/echo/inject
// peers loop until killed and never report completion, so requiring "successful"
// from them would assert nothing and fail every such test.
//
// VALIDATES: F1 scoping — the peer-governs rule keys off check-mode peers.
// PREVENTS: Failing scaffolding-peer tests (e.g. event-predicate-wait, whose
// echo peer reflects updates back and never completes).
func TestHasCheckPeer(t *testing.T) {
	tests := []struct {
		name string
		cmds []RunCommand
		want bool
	}{
		{"no peer", []RunCommand{{Exec: "ze bgp server -"}}, false},
		{"check peer", []RunCommand{{Exec: "ze-peer --port $PORT"}}, true},
		{"echo peer only", []RunCommand{{Exec: "ze-peer --port $PORT --mode echo"}}, false},
		{"sink peer only", []RunCommand{{Exec: "ze-peer --mode sink --port $PORT"}}, false},
		{"inject peer only", []RunCommand{{Exec: "ze-peer --mode inject --port $PORT"}}, false},
		{
			name: "sink plus check peer",
			cmds: []RunCommand{
				{Exec: "ze-peer --bind 127.0.0.2 --mode sink --port $PORT"},
				{Exec: "ze-peer --port $PORT"},
			},
			want: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, hasCheckPeer(tc.cmds))
		})
	}
}

// TestIsSelfValidated covers F1: the decision that governs whether a test's BGP
// exchange is validated at all.
//
// Until 2026-07-16 an expect=exit:code=0 alone marked a test self-validated,
// which skipped BOTH the peer "successful" check and validateJSON for every test
// that had a ze-peer. A test could then pass on ze's exit code while no BGP
// session ever ran. The first row below is the regression that matters.
//
// VALIDATES: F1 — a ze-peer always governs, regardless of other assertions.
// PREVENTS: The masking defect (D2) that hid D1
// (spec-fixit-redistribute-establishment-stall).
func TestIsSelfValidated(t *testing.T) {
	code := 0
	tests := []struct {
		name    string
		rec     *Record
		hasPeer bool
		want    bool
	}{
		{
			name:    "peer plus exit code is NOT self-validated (D2 regression)",
			rec:     &Record{ExpectExitCode: &code},
			hasPeer: true,
			want:    false,
		},
		{
			name:    "peer plus stdout assertion is NOT self-validated",
			rec:     &Record{ExpectStdoutMatch: []string{"ok"}},
			hasPeer: true,
			want:    false,
		},
		{
			name:    "peer with no other assertion is NOT self-validated",
			rec:     &Record{},
			hasPeer: true,
			want:    false,
		},
		{
			name:    "peer-less exit-code test is self-validated",
			rec:     &Record{ExpectExitCode: &code},
			hasPeer: false,
			want:    true,
		},
		{
			name:    "peer-less stdout test is self-validated",
			rec:     &Record{ExpectStdoutMatch: []string{"ok"}},
			hasPeer: false,
			want:    true,
		},
		{
			name:    "peer-less file-check test is self-validated",
			rec:     &Record{FileChecks: []fileCheck{{Path: "x", Exists: true}}},
			hasPeer: false,
			want:    true,
		},
		{
			name:    "peer-less test with no assertion at all is not self-validated",
			rec:     &Record{},
			hasPeer: false,
			want:    false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isSelfValidated(tc.rec, tc.hasPeer))
		})
	}
}

// mkPeerOutput builds a peerOutput carrying the given captured text, as os/exec
// would have filled it. Only stdout is populated; combined() reads both.
func mkPeerOutput(t *testing.T, checkMode bool, label, captured string) peerOutput {
	t.Helper()
	po := peerOutput{stdout: newSyncWriter(), stderr: &lockedBuilder{}, checkMode: checkMode, label: label}
	if _, err := po.stdout.Write([]byte(captured)); err != nil {
		t.Fatalf("seed peer stdout: %v", err)
	}
	return po
}

// TestPeerVerdictRequiresAllCheckPeers is the regression guard for the
// multi-peer masking defect.
//
// VALIDATES: AC-1 -- every check-mode peer must report peerSuccessToken; one
// peer's success does not stand in for another's failure.
// PREVENTS: the verdict reverting to a single strings.Contains over the joined
// rec.PeerOutput (runner_exec.go), under which a run whose destination peer
// logged "connection closed before completion" was reported PASS because the
// source peer had printed "successful". Tests 224 and 398 read as "flaky, passes
// 1 in 10" for years on that basis while failing deterministically.
func TestPeerVerdictRequiresAllCheckPeers(t *testing.T) {
	cases := []struct {
		name     string
		declared int
		peers    []peerOutput
		want     []string
	}{
		{
			name:     "all check peers succeed",
			declared: 2,
			peers: []peerOutput{
				mkPeerOutput(t, true, "stdin=peer1", "exchange successful\n"),
				mkPeerOutput(t, true, "stdin=peer2", "exchange successful\n"),
			},
			want: nil,
		},
		{
			name:     "second peer fails while first succeeds",
			declared: 2,
			peers: []peerOutput{
				mkPeerOutput(t, true, "stdin=peer1", "exchange successful\n"),
				mkPeerOutput(t, true, "stdin=peer2", "failed: connection closed before completion\n"),
			},
			want: []string{"stdin=peer2"},
		},
		{
			name:     "first peer fails while second succeeds",
			declared: 2,
			peers: []peerOutput{
				mkPeerOutput(t, true, "stdin=peer1", "failed: message mismatch\n"),
				mkPeerOutput(t, true, "stdin=peer2", "exchange successful\n"),
			},
			want: []string{"stdin=peer1"},
		},
		{
			name:     "both fail",
			declared: 2,
			peers: []peerOutput{
				mkPeerOutput(t, true, "stdin=peer1", "failed: message mismatch\n"),
				mkPeerOutput(t, true, "stdin=peer2", "failed: message mismatch\n"),
			},
			want: []string{"stdin=peer1", "stdin=peer2"},
		},
		{
			name:     "scaffolding peers are never required to report success",
			declared: 1,
			peers: []peerOutput{
				mkPeerOutput(t, true, "stdin=peer1", "exchange successful\n"),
				mkPeerOutput(t, false, "sink", "listening on 0.0.0.0:1179\n"),
			},
			want: nil,
		},
		{
			name:     "unlabelled peer still named",
			declared: 1,
			peers: []peerOutput{
				mkPeerOutput(t, true, "", "failed: message mismatch\n"),
			},
			want: []string{"check peer #1"},
		},
		{
			name:     "declared check peer that never started is a failure",
			declared: 2,
			peers: []peerOutput{
				mkPeerOutput(t, true, "stdin=peer1", "exchange successful\n"),
			},
			want: []string{"1 of 2 check-mode peers never started"},
		},
		{
			name:     "no peers captured at all",
			declared: 1,
			peers:    nil,
			want:     []string{"1 of 1 check-mode peers never started"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := failedCheckPeers(c.declared, c.peers)
			assert.Equal(t, c.want, got)
		})
	}
}

// TestCountCheckPeers verifies the declared-count side of the shortfall check:
// only check-mode ze-peers count, so a sink peer cannot inflate the expectation
// and turn a healthy test red.
func TestCountCheckPeers(t *testing.T) {
	cmds := []RunCommand{
		{Exec: "ze-peer --port $PORT"},
		{Exec: "ze-peer --mode sink --port $PORT"},
		{Exec: "ze-peer --mode=check --port $PORT"},
		{Exec: "ze --web 8080 x.conf"},
	}
	assert.Equal(t, 2, countCheckPeers(cmds))
}

// TestPeerLabelPrefersAuthoredNames checks the failure message names the peer
// the way the .ci author wrote it, falling back only when nothing was declared.
func TestPeerLabelPrefersAuthoredNames(t *testing.T) {
	assert.Equal(t, "collector", peerLabel(RunCommand{Name: "collector", Stdin: "peer1", Seq: 3}))
	assert.Equal(t, "stdin=peer1", peerLabel(RunCommand{Stdin: "peer1", Seq: 3}))
	assert.Equal(t, "cmd seq=3", peerLabel(RunCommand{Seq: 3}))
}

// TestPeerVerdictFailsOnRetractionAfterSuccess is the regression guard for the
// linger vacuity defect.
//
// VALIDATES: a rejection the linger loop finds fails the peer, even though the
// peer already printed peerSuccessToken before entering that loop.
// PREVENTS: the verdict reverting to a bare peerSuccessToken substring test.
// (*Peer).completed prints the success token BEFORE the linger loop, because
// teardown is a kill and the post-Run print can be lost. Under a bare substring
// test every negative assertion held open by option=linger is therefore vacuous:
// the rejection is detected, returned, and then discarded by the verdict.
func TestPeerVerdictFailsOnRetractionAfterSuccess(t *testing.T) {
	lingered := "exchange successful\n" +
		"lingering: holding the session open until teardown (option=linger)\n" +
		peerRejectionMarker + ": received bytes that reject=bgp:pattern=01180A0100 forbids\n"

	got := failedCheckPeers(1, []peerOutput{mkPeerOutput(t, true, "stdin=peer1", lingered)})
	if len(got) != 1 || got[0] != "stdin=peer1" {
		t.Errorf("failedCheckPeers = %v, want [stdin=peer1]: a rejection found after the "+
			"success token was printed must still fail the peer", got)
	}

	clean := "exchange successful\n" +
		"lingering: holding the session open until teardown (option=linger)\n"
	if got := failedCheckPeers(1, []peerOutput{mkPeerOutput(t, true, "stdin=peer1", clean)}); got != nil {
		t.Errorf("failedCheckPeers = %v, want nil: a lingering peer that saw no rejection passes", got)
	}
}
