// Design: docs/architecture/doctor-and-health-checks.md -- IKE engine dropped-error Warn, doctor-code, and issuequeue tests

package engine

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

type droppedErrLogCapture struct {
	mu      sync.Mutex
	records []slog.Record
}

func (c *droppedErrLogCapture) Enabled(_ context.Context, _ slog.Level) bool { return true }

//nolint:gocritic // slog.Handler interface requires slog.Record by value
func (c *droppedErrLogCapture) Handle(_ context.Context, r slog.Record) error {
	c.mu.Lock()
	c.records = append(c.records, r.Clone())
	c.mu.Unlock()
	return nil
}

func (c *droppedErrLogCapture) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *droppedErrLogCapture) WithGroup(string) slog.Handler      { return c }

func (c *droppedErrLogCapture) warns() []slog.Record {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []slog.Record
	for i := range c.records {
		if c.records[i].Level == slog.LevelWarn {
			out = append(out, c.records[i])
		}
	}
	return out
}

func (c *droppedErrLogCapture) reset() {
	c.mu.Lock()
	c.records = nil
	c.mu.Unlock()
}

func droppedErrAttr(r *slog.Record, key string) (string, bool) {
	var val string
	var found bool
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			val = a.Value.String()
			found = true
			return false
		}
		return true
	})
	return val, found
}

type mockIssuePublisher struct {
	mu       sync.Mutex
	issues   []sdk.Issue
	raiseErr error
}

func (p *mockIssuePublisher) RaiseIssue(_ context.Context, issue sdk.Issue) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.raiseErr != nil {
		return p.raiseErr
	}
	p.issues = append(p.issues, issue)
	return nil
}

func (p *mockIssuePublisher) ClearIssue(_ context.Context, _, _ string) error { return nil }

func (p *mockIssuePublisher) setError(err error) {
	p.mu.Lock()
	p.raiseErr = err
	p.mu.Unlock()
}

func (p *mockIssuePublisher) snapshot() []sdk.Issue {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]sdk.Issue, len(p.issues))
	copy(out, p.issues)
	return out
}

func (p *mockIssuePublisher) reset() {
	p.mu.Lock()
	p.issues = nil
	p.mu.Unlock()
}

type failingEventBus struct {
	err error
}

func (b *failingEventBus) Emit(_, _ string, _ any) (int, error) { return 0, b.err }
func (b *failingEventBus) Subscribe(_, _ string, _ func(any)) func() {
	return func() {}
}

type erroringDP struct {
	mockDP
	removeSAErr     error
	removePolicyErr error
}

func (d *erroringDP) RemoveSA(_ uint32, _ net.IP, _ uint8) error { return d.removeSAErr }
func (d *erroringDP) RemovePolicyParams(_ dataplane.SPParams) error {
	return d.removePolicyErr
}

func assertWarnWithDoctorCode(t *testing.T, r *slog.Record, wantCode, wantFix, wantReport string) {
	t.Helper()
	if code, ok := droppedErrAttr(r, "code"); !ok || code != wantCode {
		t.Errorf("warn code = %q (present=%v), want %q", code, ok, wantCode)
	}
	if fix, ok := droppedErrAttr(r, "fix"); !ok || fix != wantFix {
		t.Errorf("warn fix = %q (present=%v), want %q", fix, ok, wantFix)
	}
	if wantReport != "" {
		if rep, ok := droppedErrAttr(r, "report"); !ok || rep != wantReport {
			t.Errorf("warn report = %q (present=%v), want %q", rep, ok, wantReport)
		}
	}
}

func startTestIKEIssueQueue(t *testing.T) (*issuequeue.Queue, *mockIssuePublisher) {
	t.Helper()
	resetIKEIssueTrackersForTest()
	pub := &mockIssuePublisher{}
	q := issuequeue.Start(pub)
	prev := ikeIssueQueue.Swap(q)
	t.Cleanup(func() {
		ikeIssueQueue.Store(prev)
		q.Stop()
		resetIKEIssueTrackersForTest()
	})
	return q, pub
}

// VALIDATES: every Answer 2 site in internal/component/ike/engine emits a Warn
// carrying its registered doctor code, fix command, and issuequeue.Outcome,
// enqueues a ReportError sdk.Issue onto the plugin-owned issuequeue.Queue,
// coalesces repeated consecutive failures without flooding, resets once-per-
// condition tracking on recovery (or RunRaises retry after refusal), and keeps
// ENOENT/ESRCH teardown quiet.
// PREVENTS: operator-actionable IKE failures disappearing at Debug, missing
// doctor codes, blocking hot goroutines on synchronous daemon RPC, or flooding
// Warn logs and report-issue windows.
func TestIKEEngineDroppedErrorsWarnWithDoctorCodes(t *testing.T) {
	q, pub := startTestIKEIssueQueue(t)
	capture := &droppedErrLogCapture{}
	log := slog.New(capture)

	t.Run("certurl fetch failure warns once per backoff window and queues issue", func(t *testing.T) {
		capture.reset()
		pub.reset()
		resetIKEIssueTrackersForTest()

		payload := make([]byte, wire.CertHashURLHashLen)
		payload[0] = 0xab
		payload = append(payload, []byte("http://127.0.0.1:1/cert.der")...)

		certURLFetches.start("peer-certurl", payload, nil, log)
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if certURLFetches.inFlight() == 0 && len(capture.warns()) > 0 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		// Second attempt inside certURLFailBackoff must be suppressed.
		certURLFetches.start("peer-certurl", payload, nil, log)
		q.Flush()

		warns := capture.warns()
		if len(warns) != 1 {
			t.Fatalf("certurl warn count = %d, want 1", len(warns))
		}
		assertWarnWithDoctorCode(t, &warns[0], codeDoctorIPsecCertURLFetchFailed, fixShowIPsecSA, "queued")
		issues := pub.snapshot()
		if len(issues) != 1 || issues[0].Code != codeDoctorIPsecCertURLFetchFailed || issues[0].Severity != rpc.ReportError {
			t.Fatalf("certurl queued issues = %+v, want 1 ReportError with %s", issues, codeDoctorIPsecCertURLFetchFailed)
		}
	})

	t.Run("child removal ignores ENOENT and ESRCH but warns and coalesces other errors", func(t *testing.T) {
		capture.reset()
		pub.reset()
		resetIKEIssueTrackersForTest()

		child := &ChildSA{
			Owner:       "peer-child",
			InboundSPI:  0x11112222,
			OutboundSPI: 0x33334444,
			LocalAddr:   net.ParseIP("192.0.2.1"),
			RemoteAddr:  net.ParseIP("198.51.100.1"),
		}

		// ENOENT / ESRCH must stay silent at Warn and enqueue nothing.
		removeChildSAOutgoing(child, &erroringDP{removeSAErr: syscall.ENOENT}, log, false)
		removeChildSAIncoming(child, &erroringDP{removeSAErr: syscall.ESRCH}, log, false)
		q.Flush()
		if len(capture.warns()) != 0 || len(pub.snapshot()) != 0 {
			t.Fatalf("ENOENT/ESRCH produced warns=%d issues=%d, want 0/0", len(capture.warns()), len(pub.snapshot()))
		}

		// Real kernel refusal must Warn + enqueue once per peer/direction, coalescing repeats.
		permDP := &erroringDP{removeSAErr: errors.New("operation not permitted")}
		removeChildSAOutgoing(child, permDP, log, false)
		removeChildSAOutgoing(child, permDP, log, false)
		removeChildSAIncoming(child, permDP, log, false)
		q.Flush()

		warns := capture.warns()
		if len(warns) != 3 {
			t.Fatalf("child remove warn count = %d, want 3", len(warns))
		}
		assertWarnWithDoctorCode(t, &warns[0], codeDoctorIPsecChildRemoveFailed, fixShowIPsecDataplaneSA, "queued")
		assertWarnWithDoctorCode(t, &warns[1], codeDoctorIPsecChildRemoveFailed, fixShowIPsecDataplaneSA, "coalesced")
		assertWarnWithDoctorCode(t, &warns[2], codeDoctorIPsecChildRemoveFailed, fixShowIPsecDataplaneSA, "queued")
		issues := pub.snapshot()
		if len(issues) != 2 {
			t.Fatalf("child remove queued issues = %d, want 2 (outbound + inbound, coalesced repeat)", len(issues))
		}
		for _, is := range issues {
			if is.Code != codeDoctorIPsecChildRemoveFailed || is.Severity != rpc.ReportError {
				t.Errorf("unexpected child issue: %+v", is)
			}
		}
	})

	t.Run("delete and teardown build failures warn, release window, and queue issues", func(t *testing.T) {
		capture.reset()
		pub.reset()
		resetIKEIssueTrackersForTest()

		ps := &PeerSession{peerName: "peer-delete"}
		sa := &SA{PeerName: "peer-delete", IsInitiator: true, SKKeys: &crypto.SKKeys{}}
		sa.reserveRequestWindow()
		ps.writeDelete(sa, nil, &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}, log)
		if sa.requestOutstanding {
			t.Error("writeDelete did not release request window after build failure")
		}

		tr, err := transport.NewUDPTransport("127.0.0.1:0", slogutil.DiscardLogger())
		if err != nil {
			t.Fatalf("NewUDPTransport: %v", err)
		}
		t.Cleanup(func() { _ = tr.Close() })

		saTeardown := &SA{
			PeerName:    "peer-teardown",
			IsInitiator: true,
			SKKeys:      &crypto.SKKeys{}, // non-nil but empty keys force buildEncryptedMessageEx error
		}
		sendIKESATeardown(saTeardown, tr, wire.NotifyTSUnacceptable, log)
		if saTeardown.requestOutstanding {
			t.Error("sendIKESATeardown did not release request window after build failure")
		}
		q.Flush()

		warns := capture.warns()
		if len(warns) != 2 {
			t.Fatalf("delete warn count = %d, want 2", len(warns))
		}
		assertWarnWithDoctorCode(t, &warns[0], codeDoctorIPsecMessageBuildFailed, fixShowIPsecSA, "queued")
		assertWarnWithDoctorCode(t, &warns[1], codeDoctorIPsecMessageBuildFailed, fixShowIPsecSA, "queued")
		if len(pub.snapshot()) != 2 {
			t.Fatalf("delete queued issues = %d, want 2", len(pub.snapshot()))
		}
	})

	t.Run("sendRaw counts, warns once per condition, resets on recovery, and retries refused raises", func(t *testing.T) {
		capture.reset()
		pub.reset()
		resetIKEIssueTrackersForTest()

		srvConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
		if err != nil {
			t.Fatalf("ListenUDP: %v", err)
		}
		t.Cleanup(func() { _ = srvConn.Close() })
		srvAddr, ok := srvConn.LocalAddr().(*net.UDPAddr)
		if !ok {
			t.Fatalf("ListenUDP LocalAddr = %T, want *net.UDPAddr", srvConn.LocalAddr())
		}

		goodTr, err := transport.NewUDPTransport("127.0.0.1:0", slogutil.DiscardLogger())
		if err != nil {
			t.Fatalf("NewUDPTransport good: %v", err)
		}
		t.Cleanup(func() { _ = goodTr.Close() })

		closedTr, err := transport.NewUDPTransport("127.0.0.1:0", slogutil.DiscardLogger())
		if err != nil {
			t.Fatalf("NewUDPTransport closed: %v", err)
		}
		_ = closedTr.Close()

		sa := &SA{
			PeerName:     "peer-sendraw",
			InitiatorSPI: [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
			ResponderSPI: [8]byte{8, 7, 6, 5, 4, 3, 2, 1},
			peerEndpoint: srvAddr,
			ikeSocket:    closedTr,
		}
		msg := make([]byte, wire.HeaderLen)

		beforeCount := sendFailureCount("peer-sendraw", sendFailureKindIKE)
		sendRaw(sa, closedTr, msg, log)
		sendRaw(sa, closedTr, msg, log)
		q.Flush()

		if got := sendFailureCount("peer-sendraw", sendFailureKindIKE); got != beforeCount+2 {
			t.Fatalf("sendFailureCount = %d, want %d", got, beforeCount+2)
		}
		warns := capture.warns()
		if len(warns) != 1 {
			t.Fatalf("sendRaw warn count across 2 consecutive failures = %d, want 1", len(warns))
		}
		assertWarnWithDoctorCode(t, &warns[0], codeDoctorIPsecSendFailed, fixShowIPsecSA, "queued")
		if len(pub.snapshot()) != 1 {
			t.Fatalf("sendRaw queued issues = %d, want 1", len(pub.snapshot()))
		}

		// Recovery on good transport clears the condition.
		sa.ikeSocket = goodTr
		sendRaw(sa, goodTr, msg, log)

		// Next failure after recovery emits a fresh Warn + issue.
		sa.ikeSocket = closedTr
		sendRaw(sa, closedTr, msg, log)
		q.Flush()
		if len(capture.warns()) != 2 {
			t.Fatalf("sendRaw warn count after recovery + new failure = %d, want 2", len(capture.warns()))
		}
		if len(pub.snapshot()) != 2 {
			t.Fatalf("sendRaw queued issues after recovery + new failure = %d, want 2", len(pub.snapshot()))
		}

		// Daemon refusal triggers RunRaises retry after coalesce window passes.
		capture.reset()
		pub.reset()
		resetIKEIssueTrackersForTest()
		now := time.Now()
		ikeSendRuns.mu.Lock()
		ikeSendRuns.now = func() time.Time { return now }
		ikeSendRuns.mu.Unlock()

		pub.setError(errors.New("daemon unavailable"))
		sendRaw(sa, closedTr, msg, log)
		q.Flush()
		if len(pub.snapshot()) != 0 {
			t.Fatalf("expected 0 accepted issues while daemon refuses, got %d", len(pub.snapshot()))
		}

		// Daemon recovers; before window elapses, no retry is queued.
		pub.setError(nil)
		sendRaw(sa, closedTr, msg, log)
		q.Flush()
		if len(pub.snapshot()) != 0 {
			t.Fatalf("retry fired before window elapsed: got %d issues", len(pub.snapshot()))
		}

		// Once window elapses, RunRaises retries and succeeds without a second Warn flood.
		now = now.Add(ikeIssueCoalesceWindow + time.Second)
		sendRaw(sa, closedTr, msg, log)
		q.Flush()
		if len(pub.snapshot()) != 1 {
			t.Fatalf("RunRaises retry did not deliver issue after window elapsed: got %d", len(pub.snapshot()))
		}
	})

	t.Run("emitChildUp, emitChildDown, emitChildRekey warn and queue issues on bus error", func(t *testing.T) {
		capture.reset()
		pub.reset()
		resetIKEIssueTrackersForTest()

		bus := &failingEventBus{err: errors.New("bus closed")}
		child := &ChildSA{InboundSPI: 0x1010, OutboundSPI: 0x2020}
		emitChildUp(bus, "peer-emit", child, log)
		emitChildDown(bus, "peer-emit", child, log)
		emitChildRekey(bus, "peer-emit", child, log)
		q.Flush()

		warns := capture.warns()
		if len(warns) != 3 {
			t.Fatalf("emitChild* warn count = %d, want 3", len(warns))
		}
		for i := range warns {
			assertWarnWithDoctorCode(t, &warns[i], codeDoctorIPsecEventEmitFailed, fixShowIPsecSA, "queued")
		}
		if len(pub.snapshot()) != 3 {
			t.Fatalf("emitChild* queued issues = %d, want 3", len(pub.snapshot()))
		}
	})

	t.Run("inbound, notify_error, notify_invalid_msgid, notify_suspicious, responder build failures warn and queue", func(t *testing.T) {
		capture.reset()
		pub.reset()
		resetIKEIssueTrackersForTest()

		tr, err := transport.NewUDPTransport("127.0.0.1:0", slogutil.DiscardLogger())
		if err != nil {
			t.Fatalf("NewUDPTransport: %v", err)
		}
		t.Cleanup(func() { _ = tr.Close() })

		remote := &net.UDPAddr{IP: net.IPv4(198, 51, 100, 10), Port: 500}
		ps := &PeerSession{peerName: "peer-builds"}
		sa := &SA{
			PeerName:     "peer-builds",
			IsInitiator:  true,
			peerEndpoint: remote,
			SKKeys:       &crypto.SKKeys{}, // non-nil empty keys cause buildEncryptedMessageEx / buildErrorNotifyResponse to fail
		}

		// inbound.go:852
		ps.handleInformationalOwned(sa, &wire.Message{Header: wire.Header{MessageID: 7}}, nil, false, tr, nil, log)
		// notify_error.go:175
		beforeErrSupp := errorNotifySuppressedCount("build-failed")
		ps.respondError(sa, 7, wire.ExchangeInformational, wire.NotifyNoProposalChosen, nil, tr, log)
		if errorNotifySuppressedCount("build-failed") != beforeErrSupp+1 {
			t.Error("respondError did not increment build-failed suppression counter")
		}
		// notify_invalid_msgid.go:111
		beforeInvalidSupp := errorNotifySuppressedCount("invalid-msgid-build-failed")
		ps.sendInvalidMessageID(sa, 42, tr, log)
		if sa.requestOutstanding {
			t.Error("sendInvalidMessageID did not release request window")
		}
		if errorNotifySuppressedCount("invalid-msgid-build-failed") != beforeInvalidSupp+1 {
			t.Error("sendInvalidMessageID did not increment invalid-msgid-build-failed counter")
		}
		// notify_suspicious.go:239
		beforeSuspSupp := errorNotifySuppressedCount("suspicious-build-failed")
		ps.informSuspicious(sa, transport.Packet{RemoteAddr: remote}, tr, log)
		if sa.requestOutstanding {
			t.Error("informSuspicious did not release request window")
		}
		if errorNotifySuppressedCount("suspicious-build-failed") != beforeSuspSupp+1 {
			t.Error("informSuspicious did not increment suspicious-build-failed counter")
		}
		// responder.go:1338 and :1360
		ps.respondAuthError(sa, 1, wire.NotifyNoProposalChosen, tr, remote, log)
		ps.sendAuthFailed(sa, 1, tr, remote, log)
		q.Flush()

		warns := capture.warns()
		if len(warns) != 6 {
			t.Fatalf("build-failure warn count = %d, want 6", len(warns))
		}
		for i := range warns {
			assertWarnWithDoctorCode(t, &warns[i], codeDoctorIPsecMessageBuildFailed, fixShowIPsecSA, "queued")
		}
		if len(pub.snapshot()) != 6 {
			t.Fatalf("build-failure queued issues = %d, want 6", len(pub.snapshot()))
		}
	})

	t.Run("sendProbeDFClear counts and warns once per condition", func(t *testing.T) {
		capture.reset()
		pub.reset()
		resetIKEIssueTrackersForTest()

		closedTr, err := transport.NewUDPTransport("127.0.0.1:0", slogutil.DiscardLogger())
		if err != nil {
			t.Fatalf("NewUDPTransport: %v", err)
		}
		_ = closedTr.Close()

		ps := &PeerSession{
			peerName:     "peer-probe",
			pendingProbe: &probeState{},
		}
		sa := &SA{
			PeerName:     "peer-probe",
			InitiatorSPI: [8]byte{9, 9, 9, 9, 9, 9, 9, 9},
			peerEndpoint: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 500},
			ikeSocket:    closedTr,
			requestMsg:   make([]byte, wire.HeaderLen),
			requestMsgID: 3,
		}

		before := sendFailureCount("peer-probe", sendFailureKindProbeDFClear)
		now := time.Now()
		ps.sendProbeDFClear(sa, closedTr, now, log)
		ps.sendProbeDFClear(sa, closedTr, now.Add(time.Second), log)
		q.Flush()

		if got := sendFailureCount("peer-probe", sendFailureKindProbeDFClear); got != before+2 {
			t.Fatalf("probe send failure count = %d, want %d", got, before+2)
		}
		warns := capture.warns()
		if len(warns) != 1 {
			t.Fatalf("probe warn count = %d, want 1", len(warns))
		}
		assertWarnWithDoctorCode(t, &warns[0], codeDoctorIPsecSendFailed, fixShowIPsecSA, "queued")
		if len(pub.snapshot()) != 1 {
			t.Fatalf("probe queued issues = %d, want 1", len(pub.snapshot()))
		}
	})

	t.Run("removeSPDPolicies ignores ENOENT and warns with codeDoctorIPsecSPDUndoIncomplete on failure", func(t *testing.T) {
		capture.reset()
		pub.reset()
		resetIKEIssueTrackersForTest()

		_, localNet, _ := net.ParseCIDR("10.0.0.0/24")
		_, remoteNet, _ := net.ParseCIDR("10.1.0.0/24")
		policies := map[string]ipsec.SPDPolicy{
			"allow-mgmt": {
				Name:         "allow-mgmt",
				Order:        10,
				Action:       dataplane.SPActionBypass,
				Direction:    ipsec.SPDDirOut,
				LocalPrefix:  localNet,
				RemotePrefix: remoteNet,
			},
		}

		removeSPDPolicies(&erroringDP{removePolicyErr: syscall.ENOENT}, policies, log)
		q.Flush()
		if len(capture.warns()) != 0 || len(pub.snapshot()) != 0 {
			t.Fatalf("ENOENT SPD remove produced warns=%d issues=%d, want 0/0", len(capture.warns()), len(pub.snapshot()))
		}

		removeSPDPolicies(&erroringDP{removePolicyErr: errors.New("kernel EPERM")}, policies, log)
		q.Flush()
		warns := capture.warns()
		if len(warns) != 1 {
			t.Fatalf("SPD remove warn count = %d, want 1", len(warns))
		}
		assertWarnWithDoctorCode(t, &warns[0], codeDoctorIPsecSPDUndoIncomplete, fixShowIPsecDataplanePolicy, "queued")
		issues := pub.snapshot()
		if len(issues) != 1 || issues[0].Code != codeDoctorIPsecSPDUndoIncomplete {
			t.Fatalf("SPD remove queued issues = %+v, want 1 with %s", issues, codeDoctorIPsecSPDUndoIncomplete)
		}
	})

	t.Run("dataplane rollback sink and transport keepalive sink enqueue via issuequeue", func(t *testing.T) {
		capture.reset()
		pub.reset()
		resetIKEIssueTrackersForTest()

		rbSink := xfrmRollbackFailedSink(q, log)
		rbAttr := rbSink(dataplane.XFRMRollbackFailed{
			SPI:         0xdeadbeef,
			Src:         netip.MustParseAddr("198.51.100.7"),
			Dst:         netip.MustParseAddr("203.0.113.9"),
			InstallErr:  errors.New("watch failed"),
			RollbackErr: errors.New("del failed"),
		})
		if rbAttr.Key != "report" || rbAttr.Value.String() != "queued" {
			t.Errorf("xfrmRollbackFailedSink attr = %v, want report=queued", rbAttr)
		}

		kaSink := newNATKeepaliveSink(q, log)
		remote := &net.UDPAddr{IP: net.IPv4(198, 51, 100, 9), Port: 4500}
		kaAttr1 := kaSink(transport.NATKeepaliveEvent{Kind: transport.NATKeepaliveEventFailed, Remote: remote, Err: errors.New("network unreachable")})
		kaAttr2 := kaSink(transport.NATKeepaliveEvent{Kind: transport.NATKeepaliveEventFailed, Remote: remote, Err: errors.New("network unreachable")})
		if kaAttr1.Key != "report" || kaAttr1.Value.String() != "queued" {
			t.Errorf("first NATKeepaliveEventFailed attr = %v, want report=queued", kaAttr1)
		}
		if !kaAttr2.Equal(slog.Attr{}) {
			t.Errorf("second consecutive NATKeepaliveEventFailed attr = %v, want empty (coalesced)", kaAttr2)
		}
		kaSink(transport.NATKeepaliveEvent{Kind: transport.NATKeepaliveEventRecovered, Remote: remote})
		kaAttr3 := kaSink(transport.NATKeepaliveEvent{Kind: transport.NATKeepaliveEventFailed, Remote: remote, Err: errors.New("network unreachable")})
		if kaAttr3.Key != "report" || kaAttr3.Value.String() != "queued" {
			t.Errorf("NATKeepaliveEventFailed after recovery attr = %v, want report=queued", kaAttr3)
		}
		q.Flush()

		issues := pub.snapshot()
		if len(issues) != 3 {
			t.Fatalf("sink queued issues = %d, want 3 (1 rollback + 2 keepalive runs)", len(issues))
		}
		if issues[0].Code != dataplane.CodeDoctorIPsecXFRMRollbackFailed {
			t.Errorf("issue[0].Code = %q, want %q", issues[0].Code, dataplane.CodeDoctorIPsecXFRMRollbackFailed)
		}
		if issues[1].Code != transport.CodeDoctorIPsecNATKeepaliveFailed || issues[2].Code != transport.CodeDoctorIPsecNATKeepaliveFailed {
			t.Errorf("keepalive issue codes = %q, %q, want %q", issues[1].Code, issues[2].Code, transport.CodeDoctorIPsecNATKeepaliveFailed)
		}
	})

	t.Run("long peer name and peer cert URL are bounded for report-issue and redact credentials", func(t *testing.T) {
		capture.reset()
		pub.reset()
		resetIKEIssueTrackersForTest()

		longPeer := strings.Repeat("p", 255)
		const secret = "supersecret"
		const urlPrefix = "http://alice:" + secret + "@example.invalid/"
		longURL := urlPrefix + strings.Repeat("a", 600-len(urlPrefix))

		payload := make([]byte, wire.CertHashURLHashLen)
		payload[0] = 0xcd
		payload = append(payload, []byte(longURL)...)

		certURLFetches.start(longPeer, payload, nil, log)
		deadline := time.Now().Add(certURLTimeout + 2*time.Second)
		for time.Now().Before(deadline) {
			if certURLFetches.inFlight() == 0 && len(capture.warns()) >= 1 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}

		// Also exercise an unparseable peer URL containing credentials.
		badPayload := make([]byte, wire.CertHashURLHashLen)
		badPayload[0] = 0xce
		badPayload = append(badPayload, []byte(longURL+"%zz")...)
		certURLFetches.start(longPeer, badPayload, nil, log)
		deadline = time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if certURLFetches.inFlight() == 0 && len(capture.warns()) >= 2 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}

		// Exercise a subject that exceeds rpc.ReportSubjectMax (255 + ":outbound" = 264)
		// and a message that exceeds rpc.ReportMessageMax.
		longErr := errors.New(strings.Repeat("x", rpc.ReportMessageMax+100))
		raiseChildRemoveIssue(longPeer, "outbound", 0x99887766, longErr, log)
		q.Flush()

		warns := capture.warns()
		if len(warns) != 2 {
			t.Fatalf("certurl redaction warn count = %d, want 2", len(warns))
		}
		for i := range warns {
			assertWarnWithDoctorCode(t, &warns[i], codeDoctorIPsecCertURLFetchFailed, fixShowIPsecSA, "queued")
			if strings.Contains(warns[i].Message, secret) {
				t.Errorf("warn[%d] message leaked credential %q: %q", i, secret, warns[i].Message)
			}
			warns[i].Attrs(func(a slog.Attr) bool {
				if strings.Contains(a.Key, secret) || strings.Contains(a.Value.String(), secret) {
					t.Errorf("warn[%d] attr %q leaked credential %q: %q", i, a.Key, secret, a.Value.String())
				}
				return true
			})
		}

		issues := pub.snapshot()
		if len(issues) != 3 {
			t.Fatalf("queued issues = %d, want 3 (2 certurl + 1 child-remove)", len(issues))
		}
		for i, is := range issues {
			in := rpc.ReportIssueInput{
				Action:   rpc.ReportRaise,
				Severity: is.Severity,
				Code:     is.Code,
				Subject:  is.Subject,
				Message:  is.Message,
				Fix:      is.Fix,
			}
			if err := rpc.ValidateReportIssue(&in); err != nil {
				t.Errorf("issues[%d] (%s) failed rpc.ValidateReportIssue: %v", i, is.Code, err)
			}
			if len(is.Subject) > rpc.ReportSubjectMax {
				t.Errorf("issues[%d] subject length = %d, want <= %d", i, len(is.Subject), rpc.ReportSubjectMax)
			}
			if len(is.Message) > rpc.ReportMessageMax {
				t.Errorf("issues[%d] message length = %d, want <= %d", i, len(is.Message), rpc.ReportMessageMax)
			}
			if !utf8.ValidString(is.Subject) || !utf8.ValidString(is.Message) {
				t.Errorf("issues[%d] produced invalid UTF-8 after bounding", i)
			}
			if strings.Contains(is.Message, secret) || strings.Contains(is.Subject, secret) {
				t.Errorf("issues[%d] leaked credential %q in %+v", i, secret, is)
			}
		}

		// Verify fitReportField preserves UTF-8 validity when cutting mid-rune.
		multiByte := strings.Repeat("é", 200)
		cut := fitReportField(multiByte, 255)
		if len(cut) > 255 || !utf8.ValidString(cut) || cut != strings.Repeat("é", 127) {
			t.Errorf("fitReportField multi-byte cut = %d bytes valid=%v, want 254 valid UTF-8 bytes", len(cut), utf8.ValidString(cut))
		}
	})
}
