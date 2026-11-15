// Design: docs/architecture/ldp/mpls-ldp.md -- the problems LDP reports to the operator
// Related: register_test.go -- TestLDPHelloSendFailureIsWarned uses these fakes
package ldp

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// recordingPublisher keeps every raise it takes and every clear LDP sends to
// the report bus, and answers the errors of raiseErrs in order to the first
// raises, which it counts as refused and does not keep. counts flushes the
// report queue first, so it sees every report queued before the call; read
// raised only after counts. Safe for concurrent use.
type recordingPublisher struct {
	mu        sync.Mutex
	queue     *issuequeue.Queue
	raised    []sdk.Issue
	refused   int
	raiseErrs []error
	cleared   []string
}

func (p *recordingPublisher) RaiseIssue(_ context.Context, issue sdk.Issue) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.raiseErrs) > 0 {
		err := p.raiseErrs[0]
		p.raiseErrs = p.raiseErrs[1:]
		p.refused++
		return err
	}
	p.raised = append(p.raised, issue)
	return nil
}

func (p *recordingPublisher) ClearIssue(_ context.Context, code, subject string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cleared = append(p.cleared, code+" "+subject)
	return nil
}

func (p *recordingPublisher) counts() (raised, cleared int) {
	p.queue.Flush()
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.raised), len(p.cleared)
}

func (p *recordingPublisher) refusals() int {
	p.queue.Flush()
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.refused
}

// countingVec is a CounterVec that keeps each label's count. Safe for
// concurrent use.
type countingVec struct {
	mu     sync.Mutex
	counts map[string]float64
}

type countingVecCounter struct {
	vec   *countingVec
	label string
}

func (c countingVecCounter) Inc() { c.Add(1) }
func (c countingVecCounter) Add(delta float64) {
	c.vec.mu.Lock()
	defer c.vec.mu.Unlock()
	if c.vec.counts == nil {
		c.vec.counts = make(map[string]float64)
	}
	c.vec.counts[c.label] += delta
}

func (v *countingVec) With(labelValues ...string) metrics.Counter {
	return countingVecCounter{vec: v, label: strings.Join(labelValues, ",")}
}

func (v *countingVec) Delete(labelValues ...string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	label := strings.Join(labelValues, ",")
	_, held := v.counts[label]
	delete(v.counts, label)
	return held
}

func (v *countingVec) value(label string) float64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.counts[label]
}

// useProblemFakes starts a report queue over a recording publisher, in place
// of the one runLDPEngine starts, installs counting metrics for one test, and
// restores the previous ones when it ends. Its tests MUST NOT run in parallel,
// because both live in package variables.
func useProblemFakes(t *testing.T) (publisher *recordingPublisher, sendErrors, discards *countingVec) {
	t.Helper()
	previousQueue := issueQueue.Load()
	previousMetrics := ldpMetricsPtr.Load()
	publisher = &recordingPublisher{}
	publisher.queue = issuequeue.Start(publisher)
	issueQueue.Store(publisher.queue)
	t.Cleanup(func() {
		issueQueue.Store(previousQueue)
		publisher.queue.Stop()
		ldpMetricsPtr.Store(previousMetrics)
	})
	sendErrors, discards = &countingVec{}, &countingVec{}
	ldpMetricsPtr.Store(&ldpMetrics{helloSendErrors: sendErrors, helloDiscards: discards})
	return publisher, sendErrors, discards
}

// levelRecords answers the records logged at level.
func (h *recordingHandler) levelRecords(level slog.Level) []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	var found []slog.Record
	for i := range h.records {
		if h.records[i].Level == level {
			found = append(found, h.records[i])
		}
	}
	return found
}

// recordAttr answers the text of the record's attribute key, "" when absent.
func recordAttr(r slog.Record, key string) string { //nolint:gocritic // slog.Record is passed by value throughout slog
	var value string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			value = a.Value.String()
			return false
		}
		return true
	})
	return value
}

// VALIDATES: AC-3 -- malformed Hellos on an interface are each counted and
// form one condition: one WARN line naming the interface, why (no adjacency
// forms) and the fix, under doctor-ldp-hello-discarded, and one error event.
// Method: feed processDiscoveryPacket a truncated datagram and a PDU with an
// unsupported version, three discards in all, as readDiscovery does.
// PREVENTS: the discard logged at debug only, invisible at the default level,
// and a WARN per malformed packet, which a peer controls the rate of.
func TestLDPMalformedHelloIsCountedAndWarnedOnce(t *testing.T) {
	publisher, _, discards := useProblemFakes(t)
	handler := &recordingHandler{}
	log := slog.New(handler)
	table := newAdjacencyTable()

	badVersion := encodeHelloPDU([4]byte{10, 0, 0, 2}, 15, 0)
	badVersion[1] = 2
	processDiscoveryPacket([]byte{0x00, 0x01}, [4]byte{10, 0, 0, 1}, "eth0", table, nil, log)
	processDiscoveryPacket(badVersion, [4]byte{10, 0, 0, 1}, "eth0", table, nil, log)
	processDiscoveryPacket([]byte{0x00, 0x01}, [4]byte{10, 0, 0, 1}, "eth0", table, nil, log)

	if got := discards.value("eth0"); got != 3 {
		t.Errorf("ze_ldp_hello_discards_total{interface=eth0} = %v, want 3", got)
	}
	// The counter is per interface: eth0's discards count nothing against eth1.
	if got := discards.value("eth1"); got != 0 {
		t.Errorf("ze_ldp_hello_discards_total{interface=eth1} = %v, want 0", got)
	}
	warns := handler.levelRecords(slog.LevelWarn)
	if len(warns) != 1 {
		t.Fatalf("WARN lines = %d, want 1 for one condition", len(warns))
	}
	warn := warns[0]
	if !strings.Contains(warn.Message, "interface eth0") {
		t.Errorf("WARN message %q does not name interface eth0", warn.Message)
	}
	if got := recordAttr(warn, "code"); got != codeLDPHelloDiscarded {
		t.Errorf("code = %q, want %q", got, codeLDPHelloDiscarded)
	}
	if got := recordAttr(warn, "why"); !strings.Contains(got, "no adjacency") {
		t.Errorf("why = %q, want it to say no adjacency forms", got)
	}
	if got := recordAttr(warn, "fix"); got != showNeighborCommand {
		t.Errorf("fix = %q, want %q", got, showNeighborCommand)
	}
	raised, _ := publisher.counts()
	if raised != 1 {
		t.Fatalf("raises = %d, want 1", raised)
	}
	if issue := publisher.raised[0]; issue.Severity != rpc.ReportError || issue.Subject != "eth0" {
		t.Errorf("raised %+v, want an error event for subject eth0", issue)
	}
	if issue := publisher.raised[0]; issue.Message != warn.Message || issue.Fix != showNeighborCommand {
		t.Errorf("raised message %q fix %q, want the WARN line's %q and %q", issue.Message, issue.Fix, warn.Message, showNeighborCommand)
	}
}

// VALIDATES: AC-3 -- the daemon refuses the error event of a discarded Hello and of
// an ended session after the report queue took both. Each failed callback logs one
// WARN line saying show errors does not list it, under the problem's code, naming
// the interface or the peer, the daemon's error and the fix, while the problem's
// own WARN line still reads report=queued.
// Method: a recording publisher that refuses its first two raises; one malformed
// Hello and one session the peer ended; flush, then read the WARN records.
// PREVENTS: a discard or a lost session the daemon never listed vanishing with no
// line saying so.
func TestLDPRefusedReportsAreWarned(t *testing.T) {
	publisher, _, _ := useProblemFakes(t)
	refusal := errors.New("report-issue: the daemon refused the raise")
	publisher.raiseErrs = []error{refusal, refusal}
	handler := &recordingHandler{}
	log := slog.New(handler)

	noteHelloDiscard(log, newAdjacencyTable(), "eth0", errDiscoveryTruncated)
	reportSessionEnd(log, &Session{peerAddr: netip.MustParseAddr("10.0.0.2"), log: log, stopCh: make(chan struct{})},
		errKeepaliveExpiry)
	if got := publisher.refusals(); got != 2 {
		t.Fatalf("the daemon refused %d raises, want both (2)", got)
	}

	for _, tc := range []struct {
		refusal, code, subjectKey, subject string
	}{
		{"ldp: a discarded Hello could not be raised on the daemon's report bus, so show errors does not list it",
			codeLDPHelloDiscarded, "interface", "eth0"},
		{"ldp: an ended session could not be raised on the daemon's report bus, so show errors does not list it",
			codeLDPSessionEnded, "peer", "10.0.0.2"},
	} {
		var refused, problem []slog.Record
		for _, record := range handler.levelRecords(slog.LevelWarn) {
			if recordAttr(record, "code") != tc.code {
				continue
			}
			if record.Message == tc.refusal {
				refused = append(refused, record)
				continue
			}
			problem = append(problem, record)
		}
		if len(refused) != 1 || len(problem) != 1 {
			t.Fatalf("%s: refused-raise WARN lines %d and problem WARN lines %d, want 1 and 1", tc.code, len(refused), len(problem))
		}
		for key, want := range map[string]string{
			tc.subjectKey: tc.subject, "error": refusal.Error(), "fix": showNeighborCommand,
		} {
			if got := recordAttr(refused[0], key); got != want {
				t.Errorf("%s: the refused raise's %s = %q, want %q", tc.code, key, got, want)
			}
		}
		if got := recordAttr(problem[0], "report"); got != "queued" {
			t.Errorf("%s: the problem's report = %q, want queued: the queue took the raise", tc.code, got)
		}
	}
}

// VALIDATES: RFC 5036 Section 3.5 -- a Hello whose Message Length is below its
// 4-octet Message ID is a malformed discovery message: processDiscoveryPacket
// discards it, counts it and WARNs naming the cause, and forms no adjacency.
// Method: a well-formed Hello PDU with its message length rewritten to 2,
// fed to processDiscoveryPacket, which recovers nothing: a panic fails the test.
// PREVENTS: a slice whose end precedes its start, which panicked the discovery
// reader on a datagram any host on the link can send.
func TestLDPHelloShorterThanItsMessageIDIsDiscarded(t *testing.T) {
	_, _, discards := useProblemFakes(t)
	handler := &recordingHandler{}
	table := newAdjacencyTable()
	short := encodeHelloPDU([4]byte{10, 0, 0, 2}, 15, 0)
	short[ldpHeaderLen+2], short[ldpHeaderLen+3] = 0, 2

	processDiscoveryPacket(short, [4]byte{10, 0, 0, 1}, "eth0", table, nil, slog.New(handler))

	if got := discards.value("eth0"); got != 1 {
		t.Errorf("ze_ldp_hello_discards_total{interface=eth0} = %v, want 1", got)
	}
	warns := handler.levelRecords(slog.LevelWarn)
	if len(warns) != 1 || !strings.Contains(recordAttr(warns[0], "error"), errHelloShorterThanMessageID.Error()) {
		t.Fatalf("WARN lines %+v, want one naming %q", warns, errHelloShorterThanMessageID)
	}
	if got := len(table.All()); got != 0 {
		t.Errorf("adjacencies = %d after a malformed Hello, want 0", got)
	}
}

// VALIDATES: R-1 -- the discard record answers "log it" for the first discard
// on an interface, not for one inside the quiet period, again once the
// interface has been quiet for helloDiscardQuiet, and again after forget.
// PREVENTS: a once-ever latch that hides a stream of malformed Hellos that
// returns later, and a record that outlives its interface.
func TestHelloDiscardLogWarnsAgainAfterQuiet(t *testing.T) {
	var discards helloDiscardLog
	start := time.Unix(1_700_000_000, 0)
	if !discards.firstAfterQuiet("eth0", start) {
		t.Fatal("first discard on eth0 not logged")
	}
	if discards.firstAfterQuiet("eth0", start.Add(time.Second)) {
		t.Fatal("discard inside the quiet period logged")
	}
	if !discards.firstAfterQuiet("eth1", start.Add(time.Second)) {
		t.Fatal("first discard on eth1 not logged: the record is not per interface")
	}
	if !discards.firstAfterQuiet("eth0", start.Add(time.Second+helloDiscardQuiet)) {
		t.Fatal("discard after a quiet period not logged")
	}
	discards.forget("eth0")
	if !discards.firstAfterQuiet("eth0", start.Add(time.Second+helloDiscardQuiet+time.Second)) {
		t.Fatal("discard after forget not logged")
	}
}

// VALIDATES: AC-3 -- a session that ended without Ze stopping it is one WARN
// line naming the peer, why (its labels are withdrawn) and the fix, under
// doctor-ldp-session-ended, with one error event; a session Ze stopped is one
// INFO line and raises nothing.
// PREVENTS: a lost session logged at info only, and a WARN for every session
// a reload or an adjacency expiry closed on purpose.
func TestLDPSessionEndByPeerIsWarned(t *testing.T) {
	publisher, _, _ := useProblemFakes(t)
	handler := &recordingHandler{}
	log := slog.New(handler)
	peer := netip.MustParseAddr("10.0.0.2")

	ended := &Session{peerAddr: peer, log: log, stopCh: make(chan struct{})}
	reportSessionEnd(log, ended, errKeepaliveExpiry)

	warns := handler.levelRecords(slog.LevelWarn)
	if len(warns) != 1 {
		t.Fatalf("WARN lines = %d, want 1", len(warns))
	}
	warn := warns[0]
	if !strings.Contains(warn.Message, "10.0.0.2") {
		t.Errorf("WARN message %q does not name the peer", warn.Message)
	}
	if got := recordAttr(warn, "code"); got != codeLDPSessionEnded {
		t.Errorf("code = %q, want %q", got, codeLDPSessionEnded)
	}
	if got := recordAttr(warn, "why"); !strings.Contains(got, "labels") {
		t.Errorf("why = %q, want it to say the labels are withdrawn", got)
	}
	if got := recordAttr(warn, "fix"); got != showNeighborCommand {
		t.Errorf("fix = %q, want %q", got, showNeighborCommand)
	}
	if raised, _ := publisher.counts(); raised != 1 {
		t.Fatalf("raises = %d, want 1", raised)
	}
	if issue := publisher.raised[0]; issue.Severity != rpc.ReportError || issue.Subject != "10.0.0.2" {
		t.Errorf("raised %+v, want an error event for subject 10.0.0.2", issue)
	}
	if issue := publisher.raised[0]; issue.Message != warn.Message || issue.Fix != showNeighborCommand {
		t.Errorf("raised message %q fix %q, want the WARN line's %q and %q", issue.Message, issue.Fix, warn.Message, showNeighborCommand)
	}

	stopped := &Session{peerAddr: peer, log: log, stopCh: make(chan struct{})}
	close(stopped.stopCh)
	reportSessionEnd(log, stopped, errSessionClosed)
	if got := len(handler.levelRecords(slog.LevelWarn)); got != 1 {
		t.Errorf("WARN lines after a session Ze stopped = %d, want still 1", got)
	}
	if got := len(handler.levelRecords(slog.LevelInfo)); got != 1 {
		t.Errorf("INFO lines after a session Ze stopped = %d, want 1", got)
	}
	if raised, _ := publisher.counts(); raised != 1 {
		t.Errorf("raises after a session Ze stopped = %d, want still 1", raised)
	}
}

// VALIDATES: spec-dropped-error-6 ISSUE 3 -- the daemon refuses the first raise
// of a run of failed Hello sends, so the bus does not list the warning. The
// next failure inside helloDiscardQuiet raises nothing; the first failure once
// that window passed raises again, which the daemon takes; the failures after
// it raise nothing more. The run writes one WARN line for the failure and one
// for the refused raise, and the bus ends up holding exactly one warning.
// Method: a publisher that refuses its first raise, a fake clock on the
// record, and a flush of the report queue before each count.
// PREVENTS: an interface broken for hours absent from show warnings because
// the one raise of its run was refused, and a raise on every failed Hello.
func TestLDPHelloSendRetriesARefusedRaiseOncePerWindow(t *testing.T) {
	publisher, _, _ := useProblemFakes(t)
	publisher.raiseErrs = []error{errors.New("report-issue: daemon busy")}
	handler := &recordingHandler{}
	log := slog.New(handler)
	health := newHelloSendHealth("eth0")
	start := time.Unix(1_700_000_000, 0)
	clock := start
	health.now = func() time.Time { return clock }
	down := errors.New("write udp4: network is unreachable")

	health.observe(log, down)
	if got := publisher.refusals(); got != 1 {
		t.Fatalf("the daemon refused %d raises after the first failure, want 1", got)
	}
	clock = start.Add(helloDiscardQuiet - time.Second)
	health.observe(log, down)
	if raised, _ := publisher.counts(); raised != 0 || publisher.refusals() != 1 {
		t.Fatalf("a failure inside the window: raises taken %d, refused %d, want 0 and still 1", raised, publisher.refusals())
	}
	clock = start.Add(helloDiscardQuiet)
	health.observe(log, down)
	if raised, _ := publisher.counts(); raised != 1 {
		t.Fatalf("the first failure once the window passed: raises taken %d, want 1", raised)
	}
	if issue := publisher.raised[0]; issue.Severity != rpc.ReportWarning || issue.Code != codeLDPHelloSendFailed || issue.Subject != "eth0" {
		t.Errorf("raised %+v, want the hello send warning for subject eth0", issue)
	}
	for _, later := range []time.Duration{2 * helloDiscardQuiet, 10 * helloDiscardQuiet} {
		clock = start.Add(later)
		health.observe(log, down)
	}
	if raised, _ := publisher.counts(); raised != 1 || publisher.refusals() != 1 {
		t.Fatalf("failures after a raise the daemon took: raises taken %d, refused %d, want still 1 and 1", raised, publisher.refusals())
	}

	warns := handler.levelRecords(slog.LevelWarn)
	if len(warns) != 2 {
		t.Fatalf("WARN lines = %d, want the run's failure and the refused raise (2)", len(warns))
	}
	// The refusal is logged by the queue's worker, which can run before the
	// discovery goroutine writes the run's failure line, so it is found by its text.
	refused := -1
	for index, record := range warns {
		if strings.Contains(record.Message, "could not be raised") {
			refused = index
		}
	}
	if refused < 0 {
		t.Fatalf("no WARN line says the raise could not be raised: %+v", warns)
	}
	if got := recordAttr(warns[refused], "code"); got != codeLDPHelloSendFailed {
		t.Errorf("the refused raise's WARN code = %q, want %q", got, codeLDPHelloSendFailed)
	}
}
