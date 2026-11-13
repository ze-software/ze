// Design: docs/guide/ddos-mitigation.md -- what each end of the plugin does to a live FlowSpec rule
// Related: register.go (stopResponder), responder.go (announce, adoptAnnouncement, withdrawEarlierRecord)

package flowspec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// exitLog is the plugin logger's output, captured as JSON lines. Safe for
// concurrent use: runEngine's goroutines log while the test waits.
type exitLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *exitLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

// reset drops every line logged so far.
func (l *exitLog) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.Reset()
}

// exitLogLine is one decoded log line: the attributes the exit lines carry.
type exitLogLine struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
	Fix   string `json:"fix"`
	Code  string `json:"code"`
}

// lines decodes every line logged so far.
func (l *exitLog) lines(t *testing.T) []exitLogLine {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []exitLogLine
	for raw := range strings.SplitSeq(strings.TrimSpace(l.buf.String()), "\n") {
		if raw == "" {
			continue
		}
		var line exitLogLine
		if err := json.Unmarshal([]byte(raw), &line); err != nil {
			t.Fatalf("log line %q is not JSON: %v", raw, err)
		}
		out = append(out, line)
	}
	return out
}

// atLevel returns the lines logged at level ("INFO", "WARN", "ERROR").
func (l *exitLog) atLevel(t *testing.T, level string) []exitLogLine {
	t.Helper()
	var out []exitLogLine
	for _, line := range l.lines(t) {
		if line.Level == level {
			out = append(out, line)
		}
	}
	return out
}

// captureExitLog points the plugin logger at a JSON buffer for the test.
func captureExitLog(t *testing.T) *exitLog {
	t.Helper()
	logs := &exitLog{}
	previous := loggerPtr.Load()
	setLogger(slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { loggerPtr.Store(previous) })
	return logs
}

// orphanTestCode is the doctor code every exit WARN names.
const orphanTestCode = "doctor-ddos-flowspec-orphan-announcement"

// unrecordedFact is the part of the exit line for a live rule with no record.
const unrecordedFact = "and it is not recorded"

// recordRefusingDispatcher is fakeDispatcher with every record write refused,
// as a state put the daemon does not acknowledge. The record fakeDispatcher
// already holds stays as it is.
type recordRefusingDispatcher struct {
	*fakeDispatcher
}

func (d recordRefusingDispatcher) Record(flowspecMatch) error {
	return errors.New("state put: the daemon did not acknowledge the write")
}

// WithContext keeps the refusal: the fake honors no context.
func (d recordRefusingDispatcher) WithContext(context.Context) routeDispatcher { return d }

// TestStopResponderOnAClosedTransportWithAnUnrecordedRuleWarns proves that a
// stop which cannot withdraw a live rule whose record write failed logs one
// WARN that says the rule is not recorded and carries the withdrawal by hand.
//
// VALIDATES: the exit line claims a record only when one exists. announce sets
// the rule live before it writes the record, and a failed write does not undo
// the announcement, so no start reads a record and no doctor check reports the
// rule.
// METHOD: a responder announces through a dispatcher that refuses the record
// write, the dispatcher then answers the closed-connection error, and
// stopResponder runs for a stop the engine caused (exitStopped).
// PREVENTS: an INFO line saying "the rule stays recorded" and pointing at a
// restart that withdraws nothing, while the rule stays at the peers.
//
// MUTATION: with the !r.recorded arm in stopResponder removed, the line is the
// INFO closed-transport line and the WARN count is 0.
func TestStopResponderOnAClosedTransportWithAnUnrecordedRuleWarns(t *testing.T) {
	logs := captureExitLog(t)
	disp := &fakeDispatcher{}
	r := newResponder(enforcingConfig(), recordRefusingDispatcher{disp})
	announceOnce(t, r, exitTestVictim)
	disp.err = fmt.Errorf("update-route: %w", rpc.ErrMuxConnClosed)
	logs.reset()

	stopResponder(r, exitStopped)

	warns := logs.atLevel(t, "WARN")
	if len(warns) != 1 {
		t.Fatalf("WARN lines for an unrecorded rule left at the peers = %d, want 1 (log: %+v)", len(warns), logs.lines(t))
	}
	line := warns[0]
	if !strings.Contains(line.Msg, unrecordedFact) {
		t.Errorf("WARN %q does not say %q", line.Msg, unrecordedFact)
	}
	if strings.Contains(line.Msg, "stays recorded") {
		t.Errorf("WARN %q claims a record the plugin never wrote", line.Msg)
	}
	if line.Fix != orphanTestFix {
		t.Errorf("WARN fix = %q, want the withdrawal by hand %q", line.Fix, orphanTestFix)
	}
	if line.Code != orphanTestCode {
		t.Errorf("WARN code = %q, want %q", line.Code, orphanTestCode)
	}
	for _, info := range logs.atLevel(t, "INFO") {
		if strings.Contains(info.Msg, closedTransportFact) {
			t.Errorf("INFO %q claims a record the plugin never wrote", info.Msg)
		}
	}
}

// TestStopResponderKeepsTheRecordOfAnAdoptedRule proves that the knowledge
// that the live rule is recorded crosses a config apply with the rule.
//
// VALIDATES: adoptAnnouncement carries the recorded flag with active, target
// and match, so the responder that owns the rule after a commit reports it the
// way the one that announced it would.
// METHOD: a responder announces with its record written, replaceResponder
// hands the rule to a new responder, the dispatcher then answers the
// closed-connection error, and stopResponder runs on the new one.
// PREVENTS: a WARN telling the operator to withdraw by hand a rule the next
// start withdraws from its record, after any unrelated commit.
//
// MUTATION: with r.recorded = prev.recorded removed from adoptAnnouncement,
// the line is the unrecorded WARN and the INFO count is 0.
func TestStopResponderKeepsTheRecordOfAnAdoptedRule(t *testing.T) {
	logs := captureExitLog(t)
	disp := &fakeDispatcher{}
	prev := newResponder(enforcingConfig(), disp)
	announceOnce(t, prev, exitTestVictim)
	next := replaceResponder(enforcingConfig(), disp, prev, true)
	t.Cleanup(func() { activeResponder.Store(nil) })
	if active, _, _ := next.status(); !active {
		t.Fatalf("setup: the new responder did not adopt the live rule")
	}
	disp.err = fmt.Errorf("update-route: %w", rpc.ErrMuxConnClosed)
	logs.reset()

	stopResponder(next, exitStopped)

	if warns := logs.atLevel(t, "WARN"); len(warns) != 0 {
		t.Errorf("WARN lines for an adopted recorded rule = %+v, want none", warns)
	}
	var closed int
	for _, info := range logs.atLevel(t, "INFO") {
		if strings.Contains(info.Msg, closedTransportFact) {
			closed++
		}
	}
	if closed != 1 {
		t.Errorf("INFO lines saying %q = %d, want 1 (log: %+v)", closedTransportFact, closed, logs.lines(t))
	}
}

// TestFailedWithdrawOfAnUnrecordedRuleNamesTheManualWithdrawal proves that a
// running withdrawal (the probe or the cap) that fails for a rule with no
// record says so, and carries the withdrawal by hand.
//
// VALIDATES: the same fact as the exit lines, on the path that runs while the
// plugin is up: the runtime doctor check reads the record, so a rule with none
// is not reported, and the line is the only pointer to the fix.
// METHOD: a responder announces through a dispatcher that refuses the record
// write, the dispatcher then refuses the withdrawal, and withdraw runs under
// the responder's lock, as probeTick and enforceMaxDuration call it.
// PREVENTS: "the rule stays recorded and ze doctor reports it" for a rule that
// nothing records and nothing reports.
//
// MUTATION: with the !recorded arm in withdraw removed, the ERROR line claims
// the record and carries no fix.
func TestFailedWithdrawOfAnUnrecordedRuleNamesTheManualWithdrawal(t *testing.T) {
	logs := captureExitLog(t)
	disp := &fakeDispatcher{}
	r := newResponder(enforcingConfig(), recordRefusingDispatcher{disp})
	announceOnce(t, r, exitTestVictim)
	disp.err = errors.New("send refused: no peer this selector names attaches this process with the required send permission")
	logs.reset()

	r.mu.Lock()
	r.withdraw()
	r.mu.Unlock()

	errs := logs.atLevel(t, "ERROR")
	if len(errs) != 1 {
		t.Fatalf("ERROR lines for a failed withdrawal = %d, want 1 (log: %+v)", len(errs), logs.lines(t))
	}
	line := errs[0]
	if strings.Contains(line.Msg, "stays recorded") {
		t.Errorf("ERROR %q claims a record the plugin never wrote", line.Msg)
	}
	if line.Fix != orphanTestFix {
		t.Errorf("ERROR fix = %q, want the withdrawal by hand %q", line.Fix, orphanTestFix)
	}
	if line.Code != orphanTestCode {
		t.Errorf("ERROR code = %q, want %q", line.Code, orphanTestCode)
	}
}

// TestStopResponderKeepsARecordThatAlreadyNamesTheRule proves that a rule is
// known recorded when the record left by an earlier failed withdrawal already
// names it, even when the new record write fails.
//
// VALIDATES: withdrawEarlierRecord leaves a record of the same rule alone, so
// the stored record names the live rule whatever the write after the
// announcement answers.
// METHOD: the dispatcher holds a record of the victim's rule and refuses every
// write; the responder announces that same rule, then the dispatcher answers
// the closed-connection error and stopResponder runs.
// PREVENTS: a WARN telling the operator nothing records a rule whose record
// the next start reads and withdraws.
//
// MUTATION: with the recordsNext result of withdrawEarlierRecord ignored in
// announce, the line is the unrecorded WARN and the INFO count is 0.
func TestStopResponderKeepsARecordThatAlreadyNamesTheRule(t *testing.T) {
	logs := captureExitLog(t)
	earlier := buildMatch(ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix(exitTestVictim), Proto: 17})
	disp := &fakeDispatcher{recorded: &earlier}
	r := newResponder(enforcingConfig(), recordRefusingDispatcher{disp})
	announceOnce(t, r, exitTestVictim)
	disp.err = fmt.Errorf("update-route: %w", rpc.ErrMuxConnClosed)
	logs.reset()

	stopResponder(r, exitStopped)

	if warns := logs.atLevel(t, "WARN"); len(warns) != 0 {
		t.Errorf("WARN lines for a rule its record already names = %+v, want none", warns)
	}
	var closed int
	for _, info := range logs.atLevel(t, "INFO") {
		if strings.Contains(info.Msg, closedTransportFact) {
			closed++
		}
	}
	if closed != 1 {
		t.Errorf("INFO lines saying %q = %d, want 1 (log: %+v)", closedTransportFact, closed, logs.lines(t))
	}
}
