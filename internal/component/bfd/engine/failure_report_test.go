package engine

import (
	"bytes"
	"errors"
	"io/fs"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/statestore"
)

// failureHook is a MetricsHook that counts the two failure events this file
// proves: authentication failures and refused sends, the latter by packet kind.
type failureHook struct {
	mu           sync.Mutex
	authFailures int
	sendFailures map[string]int
}

func newFailureHook() *failureHook { return &failureHook{sendFailures: map[string]int{}} }

func (*failureHook) OnStateChange(_, _ packet.State, _ packet.Diag, _, _ string) {}
func (*failureHook) OnTxPacket(string)                                           {}
func (*failureHook) OnRxPacket(string)                                           {}
func (*failureHook) OnEchoTx(string)                                             {}
func (*failureHook) OnEchoRx(string)                                             {}
func (*failureHook) OnEchoRTT(string, time.Duration)                             {}

func (h *failureHook) OnAuthFailure(string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.authFailures++
}

func (h *failureHook) OnSendFailure(_, packetKind string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sendFailures[packetKind]++
}

// raisedIssue is one raise a fakeReporter recorded.
type raisedIssue struct {
	code, subject, message, fix string
}

// fakeReporter is an IssueReporter that records every raise and clear, in
// the order the engine made them, and queues nothing.
type fakeReporter struct {
	mu       sync.Mutex
	warnings []raisedIssue
	events   []raisedIssue
	clears   []raisedIssue
}

func (f *fakeReporter) RaiseWarning(code, subject, message, fix string, _ func(error)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.warnings = append(f.warnings, raisedIssue{code, subject, message, fix})
	return nil
}

func (f *fakeReporter) RaiseError(code, subject, message, fix string, _ func(error)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, raisedIssue{code, subject, message, fix})
	return nil
}

func (f *fakeReporter) ClearWarning(code, subject string, _ func(error)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clears = append(f.clears, raisedIssue{code: code, subject: subject})
	return nil
}

// counts answers how many warnings, error events and clears f recorded.
func (f *fakeReporter) counts() (warned, raised, cleared int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.warnings), len(f.events), len(f.clears)
}

// capturedLog is the buffer captureEngineLog writes to. It is safe for
// concurrent use, because a sequence persister's writer goroutine logs too.
type capturedLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *capturedLog) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

func (c *capturedLog) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

func (c *capturedLog) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buf.Reset()
}

// captureEngineLog swaps the package logger for one writing every level to a
// buffer, restoring it when the test ends. Tests using it are not parallel.
func captureEngineLog(t *testing.T) *capturedLog {
	t.Helper()
	log := &capturedLog{}
	saved := engineLog
	engineLog = func() *slog.Logger {
		return slog.New(slog.NewTextHandler(log, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	t.Cleanup(func() { engineLog = saved })
	return log
}

// eventsFor answers a copy of the error events f recorded under subject.
func (f *fakeReporter) eventsFor(subject string) []raisedIssue {
	f.mu.Lock()
	defer f.mu.Unlock()
	var events []raisedIssue
	for _, event := range f.events {
		if event.subject == subject {
			events = append(events, event)
		}
	}
	return events
}

// stubTransport answers Start and Stop with the errors it holds.
type stubTransport struct {
	startErr error
	stopErr  error
}

func (s stubTransport) Start() error                { return s.startErr }
func (s stubTransport) Stop() error                 { return s.stopErr }
func (stubTransport) Send(transport.Outbound) error { return nil }
func (stubTransport) RX() <-chan transport.Inbound  { return nil }

var (
	errEchoBind     = errors.New("echo port already in use")
	errControlClose = errors.New("control socket close failed")
)

// VALIDATES: when the echo transport fails to start, Loop.Start rolls the
// control transport back and returns BOTH failures, so its caller
// (runtimeState.loopFor, which wraps the error with %w) observes a control
// socket that failed to close.
// PREVENTS: the control stop failure reaching only a Debug line, which the
// default WARN level hides.
// Method: a Loop over a control transport whose Stop fails and an echo
// transport whose Start fails; errors.Is on Start's answer for each.
func TestLoopStartReturnsTheControlStopFailureOfItsRollback(t *testing.T) {
	l := NewLoopWithEcho(stubTransport{stopErr: errControlClose}, stubTransport{startErr: errEchoBind}, clock.RealClock{})
	err := l.Start()
	if !errors.Is(err, errEchoBind) {
		t.Fatalf("Start = %v, want the echo bind failure", err)
	}
	if !errors.Is(err, errControlClose) {
		t.Fatalf("Start = %v, want the control stop failure joined to it", err)
	}
}

// VALIDATES: an echo reflection the transport refuses is counted on every
// packet (OnSendFailure, packet echo-reflect) and logged at Warn once per
// sendWarnInterval for the session, naming the peer and the packet kind.
// PREVENTS: the peer's echo function failing, and its session going Down,
// with our only trace a Debug line, and a Warn per echo packet flooding the log.
// Method: a Loop whose echo transport refuses every Send; three echoes from
// the session's peer under a discriminator that is not ours, so each one is
// reflected.
func TestEchoReflectFailureIsCountedAndWarnedOnce(t *testing.T) {
	log := captureEngineLog(t)
	l := NewLoopWithEcho(&captureTransport{}, refusingTransport{}, clock.RealClock{})
	req := echoReqFor(addrB, addrA)
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	key := req.Key()
	hook := newFailureHook()
	l.SetMetricsHook(hook)

	for range 3 {
		buf := make([]byte, packet.EchoLen)
		packet.WriteEcho(buf, 0, packet.Echo{LocalDiscriminator: 0x99999999, Sequence: 1})
		l.handleEchoInbound(transport.Inbound{
			From: key.Peer, Local: key.Local, Interface: key.Interface,
			Mode: api.SingleHop, TTL: 255, Bytes: buf,
		})
	}

	if got := hook.sendFailures[sendKinds[sendKindEchoReflect].label]; got != 3 {
		t.Fatalf("OnSendFailure(echo-reflect) fired %d times, want 3", got)
	}
	if got := strings.Count(log.String(), "level=WARN"); got != 1 {
		t.Fatalf("%d Warn lines for three refused reflections, want 1:\n%s", got, log.String())
	}
	for _, want := range []string{"packet=" + sendKinds[sendKindEchoReflect].label, "peer=" + addrB, errSendRefused.Error()} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("Warn line does not name %q:\n%s", want, log.String())
		}
	}
}

// VALIDATES: a received Control packet that fails authentication is counted
// on every packet (OnAuthFailure), logged at Warn once per sendWarnInterval
// with the doctor code, the peer, the reason and the fix, and raised through
// the loop's IssueReporter; a packet that verifies afterwards clears the
// raise. The secret appears in neither.
// PREVENTS: a key mismatch that keeps the session from coming Up visible only
// at Debug, a Warn per packet a forger controls, a warning that outlives the
// mismatch, and a raise on the process-local bus a forked plugin's daemon
// never reads.
// Method: an authenticated session with a fake reporter; three packets with
// a tampered digest, then one signed correctly.
func TestAuthFailureIsCountedWarnedOnceAndClearedOnRecovery(t *testing.T) {
	log := captureEngineLog(t)
	l, _, key, signer := rfc5880AuthLoop(t, packet.AuthTypeKeyedSHA1)
	m := machineFor(t, l, key)
	hook := newFailureHook()
	l.SetMetricsHook(hook)
	reporter := &fakeReporter{}
	l.SetIssueReporter(reporter)

	for range 3 {
		tampered := rfc5880SignedInbound(key, signer, m.LocalDiscriminator(), 1000)
		tampered.Bytes[len(tampered.Bytes)-1] ^= 0xFF
		l.handleInbound(tampered)
	}

	if hook.authFailures != 3 {
		t.Fatalf("OnAuthFailure fired %d times, want 3", hook.authFailures)
	}
	if got := strings.Count(log.String(), "level=WARN"); got != 1 {
		t.Fatalf("%d Warn lines for three failed packets, want 1:\n%s", got, log.String())
	}
	for _, want := range []string{"code=" + AuthFailedCode, "peer=" + addrB, "reason=", "fix=", "key-id and secret", "report=queued"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("Warn line does not name %q:\n%s", want, log.String())
		}
	}
	if strings.Contains(log.String(), string(rfc5880Secret)) {
		t.Errorf("the log carries the authentication secret:\n%s", log.String())
	}
	if warnings, _, clears := reporter.counts(); warnings != 1 || clears != 0 {
		t.Fatalf("%d raises and %d clears after three failed packets, want 1 and 0: %+v", warnings, clears, reporter.warnings)
	}
	raised := reporter.warnings[0]
	if raised.code != AuthFailedCode {
		t.Errorf("raised code %q, want %q", raised.code, AuthFailedCode)
	}
	if strings.Contains(raised.message, string(rfc5880Secret)) || strings.Contains(raised.fix, string(rfc5880Secret)) {
		t.Errorf("the warning carries the authentication secret: %+v", raised)
	}

	l.handleInbound(rfc5880SignedInbound(key, signer, m.LocalDiscriminator(), 1001))
	if _, _, clears := reporter.counts(); clears != 1 {
		t.Fatalf("%d clears after a packet verified, want 1", clears)
	}
	if cleared := reporter.clears[0]; cleared.code != AuthFailedCode || cleared.subject != raised.subject {
		t.Errorf("cleared %+v, want the raise's code and subject %q", cleared, raised.subject)
	}
}

// VALIDATES: failed packets interleaved with verified ones, within one
// sendWarnInterval, give one Warn line and one raise, every failure counted,
// and the one clear the verified packet owes for that raise.
// PREVENTS: a sender that injects bad packets between the peer's good ones
// (or Meticulous reordering failing the sequence window) earning a Warn, a
// raise and a clear per packet, under l.mu on the express loop: the window
// is the rate limit and a verified packet must not restart it.
// Method: fail, verify, fail, fail on one authenticated session.
func TestInterleavedAuthFailuresWarnAndRaiseOncePerWindow(t *testing.T) {
	log := captureEngineLog(t)
	l, _, key, signer := rfc5880AuthLoop(t, packet.AuthTypeKeyedSHA1)
	m := machineFor(t, l, key)
	hook := newFailureHook()
	l.SetMetricsHook(hook)
	reporter := &fakeReporter{}
	l.SetIssueReporter(reporter)
	fail := func(seq uint32) {
		tampered := rfc5880SignedInbound(key, signer, m.LocalDiscriminator(), seq)
		tampered.Bytes[len(tampered.Bytes)-1] ^= 0xFF
		l.handleInbound(tampered)
	}

	fail(1000)
	l.handleInbound(rfc5880SignedInbound(key, signer, m.LocalDiscriminator(), 1001))
	fail(1002)
	fail(1003)

	if hook.authFailures != 3 {
		t.Fatalf("OnAuthFailure fired %d times, want 3", hook.authFailures)
	}
	if got := strings.Count(log.String(), "level=WARN"); got != 1 {
		t.Fatalf("%d Warn lines for three failed packets in one window, want 1:\n%s", got, log.String())
	}
	if warnings, _, clears := reporter.counts(); warnings != 1 || clears != 1 {
		t.Fatalf("%d raises and %d clears, want 1 and 1: raises %+v clears %+v",
			warnings, clears, reporter.warnings, reporter.clears)
	}
}

// VALIDATES: an unsaved Meticulous sequence number is logged at Warn with its
// doctor code, the session, the effect and the fix, and raised as an error
// event; with no reporter installed the Warn line says the raise reached no
// bus.
// PREVENTS: a failed persist of the TX sequence reaching nobody, and a raise
// that failed passing for one that was made.
// Method: reportSeqUnsaved over a sink holding a fake reporter, then over a
// nil sink.
func TestReportSeqUnsavedWarnsAndRaisesAnErrorEvent(t *testing.T) {
	log := captureEngineLog(t)
	reporter := &fakeReporter{}
	var sink issueSink
	var installed IssueReporter = reporter
	sink.reporter.Store(&installed)
	key := api.Key{Peer: netipAddr(t, addrB), VRF: "default", Mode: api.SingleHop}
	errStore := errors.New("store write refused")

	reportSeqUnsaved(&sink, key, seqCloseFailedProblem, errStore)

	for _, want := range []string{"level=WARN", "code=" + SeqUnsavedCode, "peer=" + addrB, errStore.Error(), "fix=\"" + seqUnsavedFix + "\"", "report=queued"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("Warn line does not name %q:\n%s", want, log.String())
		}
	}
	if _, raised, _ := reporter.counts(); raised != 1 {
		t.Fatalf("%d error events raised, want 1", raised)
	}
	if got := reporter.events[0]; got.code != SeqUnsavedCode || got.fix != seqUnsavedFix {
		t.Errorf("raised %+v, want code %q and fix %q", got, SeqUnsavedCode, seqUnsavedFix)
	}

	log.Reset()
	reportSeqUnsaved(nil, key, seqWriteFailedProblem, errStore)
	if !strings.Contains(log.String(), "report-error=") {
		t.Errorf("Warn line with no reporter does not say the raise failed:\n%s", log.String())
	}
}

// switchTransport refuses every Send with errSendRefused while refuse holds,
// and accepts it otherwise. Tests drive it from one goroutine.
type switchTransport struct{ refuse bool }

func (*switchTransport) Start() error { return nil }
func (*switchTransport) Stop() error  { return nil }
func (s *switchTransport) Send(transport.Outbound) error {
	if s.refuse {
		return errSendRefused
	}
	return nil
}
func (*switchTransport) RX() <-chan transport.Inbound { return nil }

// VALIDATES: a Control packet the transport refuses is logged at Warn with
// the doctor code SendFailedCode, the packet kind and a fix naming the
// session's interface, and raised once through the loop's IssueReporter for
// two refusals inside one window; the next Control packet that is sent clears
// that raise, and a later one clears nothing more.
// PREVENTS: a refused send reported only by a Warn line with no code, no fix
// and no raise, so show warnings never lists why the session went Down, and a
// warning that outlives the failure.
// Method: a loop over a transport that refuses while told to, a stepped
// clock and a fake reporter; two refused sends 10 s apart, then two that land.
func TestControlSendFailureIsRaisedWithItsFixAndClearedWhenASendLands(t *testing.T) {
	log := captureEngineLog(t)
	clk := &steppedClock{now: time.Unix(1_000_000, 0)}
	tr := &switchTransport{refuse: true}
	l := NewLoop(tr, clk)
	reporter := &fakeReporter{}
	l.SetIssueReporter(reporter)
	req := reqFor(addrB, addrA)
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	send := func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		entry := l.sessions[req.Key()]
		l.sendLocked(entry, entry.machine.Build())
	}

	send()
	clk.now = clk.now.Add(10 * time.Second)
	send()

	wantFix := "ze show interface name " + req.Interface + " detail"
	if got := strings.Count(log.String(), "level=WARN"); got != 1 {
		t.Fatalf("%d Warn lines for two refused sends in one window, want 1:\n%s", got, log.String())
	}
	for _, want := range []string{"code=" + SendFailedCode, "packet=control", "peer=" + addrB,
		"Control packet", errSendRefused.Error(), "fix=\"" + wantFix + "\"", "report=queued"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("Warn line does not name %q:\n%s", want, log.String())
		}
	}
	if warnings, _, clears := reporter.counts(); warnings != 1 || clears != 0 {
		t.Fatalf("%d raises and %d clears after two refused sends, want 1 and 0: %+v", warnings, clears, reporter.warnings)
	}
	raised := reporter.warnings[0]
	if raised.code != SendFailedCode || raised.fix != wantFix {
		t.Errorf("raised %+v, want code %q and fix %q", raised, SendFailedCode, wantFix)
	}
	for _, want := range []string{"Control packet", errSendRefused.Error()} {
		if !strings.Contains(raised.message, want) {
			t.Errorf("raised message %q does not name %q", raised.message, want)
		}
	}
	if !strings.Contains(raised.subject, "packet control") {
		t.Errorf("raised subject %q does not name the packet kind", raised.subject)
	}

	tr.refuse = false
	send()
	send()
	if _, _, clears := reporter.counts(); clears != 1 {
		t.Fatalf("%d clears after two sends landed, want 1", clears)
	}
	if cleared := reporter.clears[0]; cleared.code != SendFailedCode || cleared.subject != raised.subject {
		t.Errorf("cleared %+v, want the raise's code and subject %q", cleared, raised.subject)
	}
}

// VALIDATES: a refused echo reflection raises SendFailedCode under the
// echo-reflect kind, and once the peer stops sending echoes, tick withdraws
// the raise only when warningQuiet has passed since the last refusal.
// PREVENTS: a warning that stays raised for as long as the session is
// configured once the failing packets stop, since no later reflection lands
// to clear it; and a withdrawal before the failure can have ended.
// Method: a loop with a refusing echo transport and a stepped clock; one
// reflected echo, then tick one second before warningQuiet and at it.
func TestEchoReflectWarningIsWithdrawnOnceTheFailuresStop(t *testing.T) {
	captureEngineLog(t)
	clk := &steppedClock{now: time.Unix(1_000_000, 0)}
	l := NewLoopWithEcho(&captureTransport{}, refusingTransport{}, clk)
	reporter := &fakeReporter{}
	l.SetIssueReporter(reporter)
	req := echoReqFor(addrB, addrA)
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	key := req.Key()
	buf := make([]byte, packet.EchoLen)
	packet.WriteEcho(buf, 0, packet.Echo{LocalDiscriminator: 0x99999999, Sequence: 1})
	l.handleEchoInbound(transport.Inbound{
		From: key.Peer, Local: key.Local, Interface: key.Interface,
		Mode: api.SingleHop, TTL: 255, Bytes: buf,
	})
	start := clk.now

	if warnings, _, _ := reporter.counts(); warnings != 1 {
		t.Fatalf("%d raises after a refused reflection, want 1", warnings)
	}
	raised := reporter.warnings[0]
	if raised.code != SendFailedCode || !strings.Contains(raised.subject, "packet echo-reflect") {
		t.Fatalf("raised %+v, want code %q under the echo-reflect kind", raised, SendFailedCode)
	}
	if !strings.Contains(raised.message, "the peer's echo function fails") {
		t.Errorf("raised message %q does not say the peer's echo function fails", raised.message)
	}

	clk.now = start.Add(warningQuiet - time.Second)
	l.tick()
	if _, _, clears := reporter.counts(); clears != 0 {
		t.Fatalf("%d clears one second before warningQuiet, want 0", clears)
	}
	clk.now = start.Add(warningQuiet)
	l.tick()
	if _, _, clears := reporter.counts(); clears != 1 {
		t.Fatalf("%d clears at warningQuiet after the last refusal, want 1", clears)
	}
	if cleared := reporter.clears[0]; cleared.code != SendFailedCode || cleared.subject != raised.subject {
		t.Errorf("cleared %+v, want the raise's code and subject %q", cleared, raised.subject)
	}
}

// VALIDATES: the authentication warning of a peer that fails and then falls
// silent is withdrawn by tick once warningQuiet has passed since its last
// failed packet, and a failure that goes on keeps it raised: the time runs
// from the last failure, not the first.
// PREVENTS: the warning staying raised for as long as the session is
// configured once the peer stops sending, since no packet verifies to clear
// it; and a withdrawal while packets still fail.
// Method: an authenticated session on a stepped clock; failed packets at 0 s
// and 90 s, ticks at 120 s, at 90 s plus warningQuiet less one second, and at
// 90 s plus warningQuiet.
func TestAuthWarningIsWithdrawnOnceTheFailuresStop(t *testing.T) {
	captureEngineLog(t)
	l, _, key, signer := rfc5880AuthLoop(t, packet.AuthTypeKeyedSHA1)
	clk := &steppedClock{now: time.Unix(1_000_000, 0)}
	l.clk = clk
	m := machineFor(t, l, key)
	reporter := &fakeReporter{}
	l.SetIssueReporter(reporter)
	fail := func(seq uint32) {
		tampered := rfc5880SignedInbound(key, signer, m.LocalDiscriminator(), seq)
		tampered.Bytes[len(tampered.Bytes)-1] ^= 0xFF
		l.handleInbound(tampered)
	}
	clears := func() int {
		_, _, cleared := reporter.counts()
		return cleared
	}
	start := clk.now

	fail(1000)
	clk.now = start.Add(90 * time.Second)
	fail(1001)
	last := clk.now

	clk.now = start.Add(warningQuiet)
	l.tick()
	if got := clears(); got != 0 {
		t.Fatalf("%d clears at warningQuiet after the first failure while a later one is 30 s old, want 0", got)
	}
	clk.now = last.Add(warningQuiet - time.Second)
	l.tick()
	if got := clears(); got != 0 {
		t.Fatalf("%d clears one second before warningQuiet after the last failure, want 0", got)
	}
	clk.now = last.Add(warningQuiet)
	l.tick()
	if got := clears(); got != 1 {
		t.Fatalf("%d clears at warningQuiet after the last failure, want 1", got)
	}
	if cleared := reporter.clears[0]; cleared.code != AuthFailedCode {
		t.Errorf("cleared %+v, want code %q", cleared, AuthFailedCode)
	}
}

// refusingStateStore is a state store that holds no key and refuses every
// write, the shape of a store on a full or read-only disk. Only the key read
// and write a sequence persister makes are reached.
type refusingStateStore struct{ statestore.Storage }

var errStateStoreRefused = errors.New("state store is read-only")

func (refusingStateStore) ReadKey(string) ([]byte, error) { return nil, fs.ErrNotExist }
func (refusingStateStore) WriteKey(string, []byte) error  { return errStateStoreRefused }

// VALIDATES: a Meticulous session whose sequence number the state store
// refuses is reported through the real paths: buildAuthPair wires the
// persister's failure callback to reportSeqUnsaved, and closeAuth reports the
// failure Close answers, both when the session is removed (removeEntryLocked)
// and when the loop stops with the session still present (Loop.Stop). Each
// report is a Warn line with the doctor code and the fix and an error event
// on the fake reporter, naming the session; the secret is in none of them.
// PREVENTS: the persister's failure callback left unwired or wired to
// nothing, and a removal or a stop that closes the authentication and drops
// the failure Close answers.
// Method: statestore.SetStore of a refusing store; two Meticulous sessions
// with persistence; one Control packet each, so a sequence number is pending;
// removeEntryLocked of one, then Stop.
func TestSeqUnsavedIsReportedWhenASessionIsRemovedAndWhenTheLoopStops(t *testing.T) {
	log := captureEngineLog(t)
	previous := statestore.Store()
	statestore.SetStore(refusingStateStore{})
	t.Cleanup(func() { statestore.SetStore(previous) })
	l := NewLoop(&captureTransport{}, clock.RealClock{})
	reporter := &fakeReporter{}
	l.SetIssueReporter(reporter)
	secret := "k-unsaved-sequence"
	ensure := func(peer string) api.Key {
		req := reqFor(peer, addrA)
		req.Auth = &api.AuthSettings{
			Type:       packet.AuthTypeMeticulousKeyedSHA1,
			KeyID:      1,
			Secret:     []byte(secret),
			Meticulous: true,
		}
		req.PersistDir = "enabled"
		if _, err := l.EnsureSession(req); err != nil {
			t.Fatalf("EnsureSession(%s): %v", peer, err)
		}
		key := req.Key()
		l.mu.Lock()
		defer l.mu.Unlock()
		entry := l.sessions[key]
		l.sendLocked(entry, entry.machine.Build())
		return key
	}
	removed := ensure(addrB)
	stopped := ensure("203.0.113.3")
	// reported checks that key's session raised exactly its two reports: the
	// write the persister's callback reported and the failure closeAuth
	// reported. The other session's writer goroutine may report its own
	// failed write at any time, so events are selected by subject.
	reported := func(key api.Key) {
		t.Helper()
		events := reporter.eventsFor(seqSubject(key))
		if len(events) != 2 {
			t.Fatalf("%d error events raised for %s, want 2: %+v", len(events), seqSubject(key), events)
		}
		var write, closed int
		for _, event := range events {
			if event.code != SeqUnsavedCode || event.fix != seqUnsavedFix {
				t.Errorf("raised %+v, want code %q and fix %q", event, SeqUnsavedCode, seqUnsavedFix)
			}
			if !strings.Contains(event.message, errStateStoreRefused.Error()) {
				t.Errorf("raised message %q does not name the store's refusal", event.message)
			}
			if strings.HasPrefix(event.message, seqWriteFailedProblem) {
				write++
			}
			if strings.HasPrefix(event.message, seqCloseFailedProblem) {
				closed++
			}
			if strings.Contains(event.message, secret) || strings.Contains(event.fix, secret) {
				t.Errorf("the error event carries the authentication secret: %+v", event)
			}
		}
		if write != 1 {
			t.Errorf("%d write-failed reports for %s, want 1: %+v", write, seqSubject(key), events)
		}
		if closed != 1 {
			t.Errorf("%d close-failed reports for %s, want 1: %+v", closed, seqSubject(key), events)
		}
	}

	l.mu.Lock()
	l.removeEntryLocked(removed, l.sessions[removed])
	l.mu.Unlock()
	reported(removed)

	if err := l.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	reported(stopped)
	if _, raised, _ := reporter.counts(); raised != 4 {
		t.Errorf("%d error events raised in all, want 4", raised)
	}

	if got := strings.Count(log.String(), "code="+SeqUnsavedCode); got != 4 {
		t.Errorf("%d Warn lines carry %s, want 4:\n%s", got, SeqUnsavedCode, log.String())
	}
	if !strings.Contains(log.String(), "fix=\""+seqUnsavedFix+"\"") {
		t.Errorf("the Warn lines do not name the fix %q:\n%s", seqUnsavedFix, log.String())
	}
	if strings.Contains(log.String(), secret) {
		t.Errorf("the log carries the authentication secret:\n%s", log.String())
	}
}

func netipAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	a, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("ParseAddr(%q): %v", s, err)
	}
	return a
}

// errDaemonRefusedRaise is what refusingReporter hands every failed callback.
var errDaemonRefusedRaise = errors.New("report-issue: the daemon refused the raise")

// refusingReporter takes every report, as the plugin's queue does, and hands it
// at once to its failed callback, as the queue's worker does when the daemon
// refuses it. Safe for concurrent use: it keeps nothing.
type refusingReporter struct{}

func (refusingReporter) RaiseWarning(_, _, _, _ string, failed func(error)) error {
	failed(errDaemonRefusedRaise)
	return nil
}

func (refusingReporter) RaiseError(_, _, _, _ string, failed func(error)) error {
	failed(errDaemonRefusedRaise)
	return nil
}

func (refusingReporter) ClearWarning(_, _ string, failed func(error)) error {
	failed(errDaemonRefusedRaise)
	return nil
}

// VALIDATES: a report the daemon refuses after the queue took it reaches the
// operator anyway: the failed callback of each BFD report writes one Warn line
// saying the report bus is wrong about the session, under the report's code,
// naming the subject, the daemon's error and where the engine's own lines are
// read, while the problem's Warn line still says the raise was queued.
// Method: reportSeqUnsaved over a sink whose reporter refuses every raise, then
// each callback the auth and send reports pass (authReportFailed,
// sendReportFailed) handed the same refusal.
// PREVENTS: a warning or error event the daemon never listed vanishing with no
// line saying so.
func TestRefusedReportsAreWarned(t *testing.T) {
	log := captureEngineLog(t)
	var sink issueSink
	var installed IssueReporter = refusingReporter{}
	sink.reporter.Store(&installed)
	key := api.Key{Peer: netipAddr(t, addrB), VRF: "default", Mode: api.SingleHop}

	reportSeqUnsaved(&sink, key, seqWriteFailedProblem, errors.New("store write refused"))
	authReportFailed("peer " + addrB + " auth")(errDaemonRefusedRaise)
	sendReportFailed("peer " + addrB + " send")(errDaemonRefusedRaise)

	lines := strings.Split(strings.TrimSpace(log.String()), "\n")
	for _, tc := range []struct{ refusal, code, subject string }{
		{"bfd could not raise an unsaved authentication sequence number on the report bus, so show errors does not list it",
			SeqUnsavedCode, seqSubject(key)},
		{"bfd could not update the authentication warning of a session on the report bus, so show warnings may be wrong about it",
			AuthFailedCode, "peer " + addrB + " auth"},
		{"bfd could not update the send warning of a session on the report bus, so show warnings may be wrong about it",
			SendFailedCode, "peer " + addrB + " send"},
	} {
		var refusalLine string
		for _, line := range lines {
			if strings.Contains(line, tc.refusal) {
				refusalLine = line
			}
		}
		for _, want := range []string{
			"level=WARN",
			"code=" + tc.code,
			`subject="` + tc.subject + `"`,
			`error="` + errDaemonRefusedRaise.Error() + `"`,
			`fix="` + engineLogFix + `"`,
		} {
			if !strings.Contains(refusalLine, want) {
				t.Errorf("%s: the refused report's Warn line %q lacks %q (log:\n%s)", tc.code, refusalLine, want, log.String())
			}
		}
	}
	var problemLine string
	for _, line := range lines {
		if strings.Contains(line, `msg="`+seqWriteFailedProblem+`"`) {
			problemLine = line
		}
	}
	if !strings.Contains(problemLine, "report=queued") {
		t.Errorf("the unsaved sequence's Warn line %q does not say the raise was queued", problemLine)
	}
}
