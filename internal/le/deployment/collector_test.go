package deployment

// The collector and the two waits built on it.
//
// Goal: Pin down the evidence from which a proof reads its verdict. Every
// deployment proof decides from one daemon's output and a handful of lines.
// These tests specify which line counts, which line ends the wait early, and
// what a caller can read afterward. These rules determine the verdicts.
// Method: Feed a collector through the same stream a process would write. No
// process runs.

import (
	"io"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// feed writes lines into a collector through the same reader a daemon's pipe
// would be, and waits for the stream to end.
func feed(t *testing.T, seen *collector, lines ...string) {
	t.Helper()

	seen.stream("test> ", strings.NewReader(strings.Join(lines, "\n")+"\n"), io.Discard)
	seen.wait()
}

// idleProcess answers a process that has already exited. Every wait below
// receives this process. Each case has already fed all its lines, so no more
// lines will be written. A wait that does not answer at once waits on nothing.
func idleProcess(t *testing.T) *running {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "true")
	proc, err := startWatched(cmd, "true> ", newCollector(), io.Discard)
	if err != nil {
		t.Fatalf("start the idle process: %v", err)
	}
	t.Cleanup(proc.stop)
	select {
	case <-proc.done:
		return proc
	case <-t.Context().Done():
		t.Fatal("wait for the idle process:", t.Context().Err())
		return nil
	}
}

// VALIDATES: A needle the collector was built to watch keeps the lines that
// carried it. A needle it was not built to watch keeps no lines.
// PREVENTS: This keeps a proof from reading a field from a line nobody kept. The
// interface a session came up on and the address it was given are both read this
// way. A collector that returned an empty list would send the run down its
// fallback path with no evidence.
func TestACollectorKeepsTheLinesThatCarriedEachNeedle(t *testing.T) {
	seen := newCollector("session IP assigned", "PPP session up")
	feed(t,
		seen,
		"l2tp: listener bound",
		"l2tp: session IP assigned username=alice address=10.100.0.2",
		"l2tp: session IP assigned username=bob address=10.100.0.3",
		"l2tp: PPP session up interface=ppp0",
	)

	assigned := seen.carrying("session IP assigned")
	if len(assigned) != 2 {
		t.Fatalf("the collector kept %d assignment lines, want 2: %v", len(assigned), assigned)
	}
	if !strings.Contains(assigned[0], "alice") || !strings.Contains(assigned[1], "bob") {
		t.Errorf("the assignment lines are not in the order they arrived: %v", assigned)
	}
	if got := seen.carrying("listener bound"); len(got) != 0 {
		t.Errorf("the collector kept lines for a needle it was never given: %v", got)
	}
	if !seen.saw("PPP session up") {
		t.Error("a needle that arrived reads as unseen")
	}
	if seen.saw("listener bound") {
		t.Error("a needle the collector was never given reads as seen")
	}
}

// VALIDATES: the lines kept for one needle are bounded.
// PREVENTS: a daemon that repeats one line making this tool hold a whole
// session's log. The proofs run their daemon with debug logging on, so a needle
// that matches a per-packet line matches thousands of them.
func TestACollectorBoundsWhatItKeepsForOneNeedle(t *testing.T) {
	seen := newCollector("repeated")
	lines := make([]string, 0, needleHits*3)
	for i := range cap(lines) {
		lines = append(lines, "repeated line "+strconv.Itoa(i))
	}
	feed(t, seen, lines...)

	if kept := len(seen.carrying("repeated")); kept != needleHits {
		t.Errorf("the collector kept %d lines for one needle, want the bound of %d", kept, needleHits)
	}
}

// VALIDATES: sawAll answers only once every needle has arrived.
// PREVENTS: a proof reporting LCP, IPCP and route injection complete when two
// of the three arrived. The three are waited for together, so a partial answer
// here is a verdict over a path that was never walked.
func TestSawAllAnswersOnlyWhenEveryNeedleArrived(t *testing.T) {
	wanted := []string{"one", "two", "three"}
	seen := newCollector(wanted...)

	feed(t, seen, "line one", "line two")
	if seen.sawAll(wanted) {
		t.Error("two needles of three read as all three")
	}
	feed(t, seen, "line three")
	if !seen.sawAll(wanted) {
		t.Error("three needles of three read as incomplete")
	}
}

// VALIDATES: A fatal line ends the wait with the needle in the error. It wins
// over a success line that arrived in the same burst.
// PREVENTS: This ordering prevents a false proven-path verdict. The line that
// ends a session and the line that reports one arrive within milliseconds of
// each other. A wait that read the success set first would report a proven path
// after the daemon had already said the kernel refused it.
func TestAFatalDaemonLineEndsTheWaitAndNamesItself(t *testing.T) {
	proc := idleProcess(t)
	fatal := []string{"genl family resolve failed", "ncp: timeout"}
	seen := newCollector(append([]string{"session up"}, fatal...)...)

	feed(t, seen, "l2tp: session up interface=ppp0", "l2tp: ncp: timeout after 15s")

	arrived, err := awaitAll(seen, []string{"session up"}, fatal, proc, time.Second)
	if err == nil {
		t.Fatalf("a wait over a fatal line answered arrived=%v with no error", arrived)
	}
	if arrived {
		t.Error("a wait that failed also reported its needles arrived")
	}
	if !strings.Contains(err.Error(), "ncp: timeout") {
		t.Errorf("the error does not name the line that ended the wait: %v", err)
	}
}

// VALIDATES: a wait with no fatal line answers when its needles arrive, and
// answers false rather than blocking when they do not.
// PREVENTS: a bound that is not a bound. Every step of the L2TP proof is waited
// for this way, so a wait that never returned would hang the gate rather than
// failing it.
func TestAWaitAnswersItsNeedlesAndItsBound(t *testing.T) {
	proc := idleProcess(t)
	seen := newCollector("listener bound", "never arrives")

	feed(t, seen, "l2tp: listener bound addr=172.30.0.1:1701")

	arrived, err := awaitAll(seen, []string{"listener bound"}, nil, proc, time.Second)
	if err != nil || !arrived {
		t.Errorf("a needle that arrived answered arrived=%v err=%v", arrived, err)
	}

	arrived, err = awaitAll(seen, []string{"never arrives"}, nil, proc, 100*time.Millisecond)
	if err != nil {
		t.Errorf("a needle that never arrived answered an error rather than a miss: %v", err)
	}
	if arrived {
		t.Error("a needle that never arrived reads as arrived")
	}
}
