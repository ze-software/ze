package bgp

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// frrLDPNeighborsAnswer is composed from the fields FRR 10.7.1's
// show_nbr_msg_json writes (ldpd/ldp_vty_exec.c) for ze at its transport
// address and a second neighbor; it is not a capture of a lab run.
const frrLDPNeighborsAnswer = `{"neighbors":[` +
	`{"addressFamily":"ipv4","neighborId":"172.30.0.9","state":"OPERATIONAL","transportAddress":"172.30.0.9","upTime":"01:00:00"},` +
	`{"addressFamily":"ipv4","neighborId":"172.30.0.2","state":"OPERATIONAL","transportAddress":"172.30.0.250","upTime":"00:00:07"}]}`

// VALIDATES: the checker reads the neighbor at ze's transport address, with
// its state and its HH:MM:SS upTime.
// PREVENTS: another neighbor's state or upTime judged as ze's, and an
// unreadable answer read as a neighbor.
func TestFRRLDPNeighborAt(t *testing.T) {
	neighbor, err := frrLDPNeighborAt(frrLDPNeighborsAnswer, "172.30.0.250")
	if err != nil {
		t.Fatalf("frrLDPNeighborAt: %v", err)
	}
	if neighbor.state != ldpStateOperational {
		t.Fatalf("state = %q, want %q", neighbor.state, ldpStateOperational)
	}
	if neighbor.upTime != 7*time.Second {
		t.Fatalf("upTime = %s, want 7s", neighbor.upTime)
	}
	for name, answer := range map[string]string{
		"absent":     `{"neighbors":[{"state":"OPERATIONAL","transportAddress":"172.30.0.9","upTime":"00:00:07"}]}`,
		"empty":      `{"neighbors":[]}`,
		"not json":   `% Unknown command: show mpls ldp neighbor json`,
		"day form":   `{"neighbors":[{"state":"OPERATIONAL","transportAddress":"172.30.0.250","upTime":"1d02h03m"}]}`,
		"bad digits": `{"neighbors":[{"state":"OPERATIONAL","transportAddress":"172.30.0.250","upTime":"00:0x:07"}]}`,
		"negative":   `{"neighbors":[{"state":"OPERATIONAL","transportAddress":"172.30.0.250","upTime":"00:-1:07"}]}`,
	} {
		if _, err := frrLDPNeighborAt(answer, "172.30.0.250"); err == nil {
			t.Errorf("%s: frrLDPNeighborAt accepted %s", name, answer)
		}
	}
}

// VALIDATES: the session counts as settled only OPERATIONAL for 3 s, and as
// re-established only OPERATIONAL with an upTime no older than the clear.
// PREVENTS: the session that was never cleared, still OPERATIONAL, passing as
// the one ze brought back.
func TestLDPNeighborReestablishedWithin(t *testing.T) {
	cases := []struct {
		name       string
		neighbor   ldpNeighbor
		sinceClear time.Duration
		settled    bool
		back       bool
	}{
		{"fresh session after the clear", ldpNeighbor{ldpStateOperational, time.Second}, 2500 * time.Millisecond, false, true},
		{"uncleared session, rounded down", ldpNeighbor{ldpStateOperational, 5 * time.Second}, 2500 * time.Millisecond, true, false},
		{"not operational yet", ldpNeighbor{"OPENSENT", 0}, 2500 * time.Millisecond, false, false},
		{"present after the clear", ldpNeighbor{"PRESENT", 0}, time.Second, false, false},
		{"settled boundary", ldpNeighbor{ldpStateOperational, ldpClearSettled}, time.Hour, true, true},
	}
	for _, c := range cases {
		if got := c.neighbor.settled(); got != c.settled {
			t.Errorf("%s: settled = %v, want %v", c.name, got, c.settled)
		}
		if got := c.neighbor.reestablishedWithin(c.sinceClear); got != c.back {
			t.Errorf("%s: reestablishedWithin(%s) = %v, want %v", c.name, c.sinceClear, got, c.back)
		}
	}
}

// The zeLog lines are composed in the form ze's slog text handler writes; the
// messages are the literals of Session.ReadLoop, runSession and
// recordSetupFailure.
const (
	zeLogFatal  = `time=2026-11-12T00:00:01Z level=WARN msg="ldp: fatal notification received, closing the session" subsystem=ldp status=2147483658 refers-to=0` + "\n"
	zeLogEnded  = `time=2026-11-12T00:00:01Z level=INFO msg="ldp: session ended" subsystem=ldp peer=172.30.0.3 reason="ldp: peer sent a fatal notification: status 0x8000000a"` + "\n"
	zeLogFailed = `time=2026-11-12T00:00:01Z level=INFO msg="ldp: session setup failed, next attempt delayed" subsystem=ldp peer=172.30.0.3:0 delay=15s` + "\n"
)

// VALIDATES: ze's log across the clear passes only with FRR's Shutdown
// received, the session end logged at info, and no setup backoff recorded.
// PREVENTS: a run where the clear never reached ze, or where info lines are not
// logged, passing on the absence of a setup failure; and the unfixed session
// end, which records a backoff, passing.
func TestRequireLDPBackoffReleased(t *testing.T) {
	startup := zeLogFailed // a failed attempt before the first session is not judged
	before := countZeLDPLog(startup)
	cases := []struct {
		name     string
		after    string
		refusal  string
		accepted bool
	}{
		{"backoff released", startup + zeLogFatal + zeLogEnded, "", true},
		{"backoff recorded", startup + zeLogFatal + zeLogEnded + zeLogFailed, "recorded a setup backoff", false},
		{"clear never reached ze", startup + zeLogEnded, "never reached ze", false},
		{"info not logged", startup + zeLogFatal, "proves nothing", false},
	}
	for _, c := range cases {
		err := requireLDPBackoffReleased(before, countZeLDPLog(c.after))
		if c.accepted {
			if err != nil {
				t.Errorf("%s: refused: %v", c.name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: accepted", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.refusal) {
			t.Errorf("%s: refusal %q does not say %q", c.name, err, c.refusal)
		}
	}
}

// VALIDATES: the redial bound accepts the Hello-cadence redial and refuses
// the one a 15 s setup backoff produces.
// PREVENTS: a redial delayed by the backoff passing the wall-time check.
func TestRequireLDPRedialWithin(t *testing.T) {
	if err := requireLDPRedialWithin(2 * time.Second); err != nil {
		t.Fatalf("a 2 s redial was refused: %v", err)
	}
	if err := requireLDPRedialWithin(ldpClearRedialBound); err != nil {
		t.Fatalf("a redial at the bound was refused: %v", err)
	}
	// 15 s is setupRetryDelayMin in internal/plugins/ldp/register.go, the first
	// delay recordSetupFailure imposes.
	if err := requireLDPRedialWithin(15 * time.Second); err == nil {
		t.Fatal("a redial after the 15 s setup backoff was accepted")
	}
}

// ldpBackoffClearRecordingDirectory holds the committed recording of a passing
// ldp-backoff-clear-frr lab run.
const ldpBackoffClearRecordingDirectory = "testdata/ldp-backoff-clear-frr"

// TestLDPBackoffClearReplaysTheLabRecording replays the committed recording of
// a passing lab run through the scenario's registered checker
// (interoplab.ReplayScenario), so the checker is judged on the answers a real
// FRR run gave it, with no Docker.
// VALIDATES: the checker passes on the recorded run exactly as the lab did:
// FRR OPERATIONAL with upTime 3 s, the clear, OPERATIONAL again with upTime 0,
// and ze's log with the fatal Notification and the session end and no setup
// backoff. The refusals of a changed log stay with TestRequireLDPBackoffReleased:
// a replayed failure reaches checkerFailure, whose log reads the passing run
// never made, so the replay refuses rather than answering a red verdict.
// PREVENTS: a checker change judged only by a lab rerun.
func TestLDPBackoffClearReplaysTheLabRecording(t *testing.T) {
	checkout, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	recordings, err := filepath.Glob(filepath.Join(ldpBackoffClearRecordingDirectory, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(recordings) != 1 {
		t.Fatalf("%s holds %d recordings, want exactly one. Fix: run `INTEROP_SCENARIO=%s ./le test integration interop`, "+
			"copy the recording it names from <session scratch>/%s/bgp/ into %s, review it like any fixture, and commit it",
			ldpBackoffClearRecordingDirectory, len(recordings), ldpBackoffClearScenario, interoplab.RecordingDirectory, ldpBackoffClearRecordingDirectory)
	}
	result, err := interoplab.ReplayScenario(t.Context(), checkout, recordings[0])
	if err != nil {
		t.Fatalf("replay of %s: %v", recordings[0], err)
	}
	if !result.Lab.Passed {
		t.Fatalf("the committed recording is of a failed lab run (%s); commit one of a passing run", result.Lab.Error)
	}
	if result.Replay != result.Lab {
		t.Errorf("the checker replayed verdict %#v on the recording of a lab run whose verdict was %#v", result.Replay, result.Lab)
	}
}

// VALIDATES: the trail records one entry for each state change, an unreadable
// answer as its own state, and an OPERATIONAL upTime that went backwards as a
// restart.
// PREVENTS: a failed wait that cannot say whether the session came up late or
// flapped, and a flap hidden because FRR named OPERATIONAL on both probes.
func TestLDPStateTrail(t *testing.T) {
	trail := ldpStateTrail{origin: "into the wait"}
	unread := errors.New("FRR lists no LDP neighbor at transport address 172.30.0.250")
	trail.observe(ldpNeighbor{}, unread, 0)
	trail.observe(ldpNeighbor{}, unread, time.Second)
	trail.observe(ldpNeighbor{"OPENSENT", 0}, nil, 2*time.Second)
	trail.observe(ldpNeighbor{ldpStateOperational, 0}, nil, 3*time.Second)
	trail.observe(ldpNeighbor{ldpStateOperational, time.Second}, nil, 4*time.Second)
	trail.observe(ldpNeighbor{ldpStateOperational, 0}, nil, 5*time.Second)
	want := []ldpStateChange{
		{ldpStateUnread, 0},
		{"OPENSENT", 2 * time.Second},
		{ldpStateOperational, 3 * time.Second},
		{ldpStateOperational + ldpStateRestarted, 5 * time.Second},
	}
	if len(trail.changes) != len(want) {
		t.Fatalf("changes = %v, want %v", trail.changes, want)
	}
	for index, change := range want {
		if trail.changes[index] != change {
			t.Errorf("change %d = %v, want %v", index, trail.changes[index], change)
		}
	}
	if trail.dropped != 0 {
		t.Errorf("dropped = %d, want 0", trail.dropped)
	}
}

// VALIDATES: a trail keeps ldpStateChangesMax changes and counts the rest.
// PREVENTS: a session flapping for the whole wait flooding the failure.
func TestLDPStateTrailBound(t *testing.T) {
	trail := ldpStateTrail{origin: "after the clear"}
	for index := range ldpStateChangesMax + 3 {
		state := "PRESENT"
		if index%2 == 1 {
			state = ldpStateOperational
		}
		trail.observe(ldpNeighbor{state, 0}, nil, time.Duration(index)*time.Second)
	}
	if len(trail.changes) != ldpStateChangesMax {
		t.Fatalf("kept %d changes, want %d", len(trail.changes), ldpStateChangesMax)
	}
	if trail.dropped != 3 {
		t.Fatalf("dropped = %d, want 3", trail.dropped)
	}
	summary := ldpWaitSummary(interoplab.WaitReport{Attempts: ldpStateChangesMax + 3}, &trail)
	if !strings.Contains(summary, "and 3 later changes not shown") {
		t.Fatalf("summary does not count the dropped changes: %s", summary)
	}
}

// VALIDATES: the summary names the probe count, the elapsed time, the failed
// probes and each recorded state with its offset; with no state read it says
// FRR's view is unknown.
// PREVENTS: a timeout whose red says only that the deadline passed.
func TestLDPWaitSummary(t *testing.T) {
	trail := ldpStateTrail{origin: "into the wait"}
	trail.observe(ldpNeighbor{"PRESENT", 0}, nil, 0)
	trail.observe(ldpNeighbor{ldpStateOperational, 0}, nil, 117512*time.Millisecond)
	report := interoplab.WaitReport{Attempts: 121, TransientFailures: 2, Elapsed: 120400 * time.Millisecond}
	summary := ldpWaitSummary(report, &trail)
	for _, want := range []string{
		"121 probes over 2m0.4s",
		"2 of them could not read FRR's neighbors",
		"PRESENT 0s into the wait, then OPERATIONAL 1m57.512s into the wait.",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary %q does not say %q", summary, want)
		}
	}
	empty := ldpWaitSummary(interoplab.WaitReport{Attempts: 3, TransientFailures: 3}, &ldpStateTrail{origin: "into the wait"})
	if !strings.Contains(empty, "FRR's view of the session is unknown") {
		t.Errorf("summary with no reading does not say FRR's view is unknown: %s", empty)
	}
}

// VALIDATES: a failed wait's error keeps its cause and carries the summary,
// ze's `show ldp neighbor` answer and FRR's log tail; with neither peer
// answering it still names why each piece is missing.
// PREVENTS: an LDP wait red that shows no evidence of why it timed out.
func TestLDPWaitFailureCarriesEvidence(t *testing.T) {
	cause := errors.New("the LDP session with ze never stayed OPERATIONAL for 3s")
	trail := ldpStateTrail{origin: "into the wait"}
	trail.observe(ldpNeighbor{ldpStateOperational, 0}, nil, time.Second)
	report := interoplab.WaitReport{Attempts: 2, Elapsed: 2 * time.Second}

	err := ldpWaitFailure(t.Context(), &recordingLab{output: "peer-answer"}, cause, report, &trail)
	if !errors.Is(err, cause) {
		t.Fatalf("the evidence lost its cause: %v", err)
	}
	for _, want := range []string{
		"The wait made 2 probes over 2s",
		"OPERATIONAL 1s into the wait",
		"--- ze `show ldp neighbor` ---\npeer-answer",
		"--- FRR log tail ---\npeer-answer",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not carry %q:\n%v", want, err)
		}
	}

	unreachable := errors.New("container gone")
	err = ldpWaitFailure(t.Context(), &recordingLab{failure: unreachable}, cause, report, &trail)
	if !errors.Is(err, cause) {
		t.Fatalf("the evidence lost its cause: %v", err)
	}
	for _, want := range []string{"(ze did not answer: container gone)", "FRR log unreadable: container gone"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not say %q:\n%v", want, err)
		}
	}
}
