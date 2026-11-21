// Design: docs/architecture/l2tp/subscriber-session-model.md -- provisional accounting custody.
package ppp

import (
	"context"
	"errors"
	"testing"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// Actual auth consumption retires answer authority but must retain the
// provisional owner while the real IPCP address provider is still pending.
func TestAuthAdmissionConsumedAcceptAwaitingNCPKeepsOriginalReservation(t *testing.T) {
	session, request := startAuthAdmissionDriver(t)
	receipt := request.AccountingAdmission()
	if !receipt.IsValid() {
		t.Fatal("actual initial L2TP auth omitted its admission receipt")
	}
	forged := request
	forged.SubscriberID = "another opaque identity"
	if forged.AccountingAdmission().IsValid() {
		t.Fatal("changed original request captured accounting custody")
	}
	acceptAuthAdmissionRequest(t, session, request)
	if request.Active() {
		t.Fatal("consumed auth request remains answerable")
	}
	if _, prepared := receipt.PreparedAssignment(); prepared {
		t.Fatal("pending actual IPCP invented an assignment")
	}
	requireAuthAdmissionPending(t, receipt)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := receipt.Assignment(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled wait = %v", err)
	}
	requireAuthAdmissionPending(t, receipt)
	request.Control.Cancel()
	ctx, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if _, err := receipt.Assignment(ctx); !errors.Is(err, ErrAuthAdmissionAborted) {
		t.Fatalf("original canceled pre-NCP auth = %v", err)
	}
}

// The publication callback can identify and queue its exact reserved owner
// before the waiter is scheduled. Only the actual transport ACK authorizes
// Start. The positive path completes genuine IPCP and useful LCP traffic.
func TestAuthAdmissionPreparedPublicationTransfersAfterOriginalAck(t *testing.T) {
	session, request := startAuthAdmissionDriver(t)
	receipt := request.AccountingAdmission()
	acceptAuthAdmissionRequest(t, session, request)
	assigned := prepareAuthAdmissionIPCP(t, session)
	prepared, ok := receipt.PreparedAssignment()
	if !ok {
		t.Fatal("actual assignment publication has no original prepared binding")
	}
	if prepared.Birth != assigned.Birth || prepared.Birth != session.birth {
		t.Fatal("prepared assignment substituted the actual birth")
	}
	if prepared.NetworkLifetime != assigned.NetworkLifetime {
		t.Fatal("prepared assignment substituted the actual network lifetime")
	}
	if !prepared.CounterReceipt.IsValid() {
		t.Fatal("prepared assignment has no original counter receipt")
	}
	base, err := prepared.BaselineCounters.Counters()
	if err != nil {
		t.Fatalf("original baseline = %v", err)
	}
	if base != (l2tpevents.TrafficTotals{RxBytes: 13, TxBytes: 27, RxPackets: 3, TxPackets: 5}) {
		t.Fatalf("prepared original baseline = %+v", base)
	}
	requireAuthAdmissionPending(t, receipt)
	assigned.Acknowledge()
	session.requireUp(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	actual, err := receipt.Assignment(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Birth != prepared.Birth || actual.NetworkLifetime != prepared.NetworkLifetime || actual.CounterReceipt != prepared.CounterReceipt {
		t.Fatal("ACK transferred another original network")
	}
	frameLeaseEcho(t, session.peer)
	request.Control.Cancel()
	retained, err := receipt.Assignment(ctx)
	if err != nil {
		t.Fatalf("later terminal cancellation erased acknowledged transfer: %v", err)
	}
	if retained.CounterReceipt != actual.CounterReceipt {
		t.Fatal("terminal cancellation redirected original counter custody")
	}
}

// Cancellation while the original publication awaits ACK cannot manufacture
// an assignment, even when the same ACK becomes ready immediately afterwards.
func TestAuthAdmissionCanceledPreparedPublicationNeverTransfers(t *testing.T) {
	session, request := startAuthAdmissionDriver(t)
	receipt := request.AccountingAdmission()
	acceptAuthAdmissionRequest(t, session, request)
	assigned := prepareAuthAdmissionIPCP(t, session)
	if _, ok := receipt.PreparedAssignment(); !ok {
		t.Fatal("test never reached actual prepared publication")
	}
	requireAuthAdmissionPending(t, receipt)
	assigned.Cancel()
	assigned.Acknowledge()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := receipt.Assignment(ctx); !errors.Is(err, ErrAuthAdmissionAborted) {
		t.Fatalf("canceled original assignment = %v", err)
	}
	if receipt.binding.finish(true) {
		t.Fatal("late original ACK overwrote abort")
	}
}

// Actual StartSession.PPPoE provenance and the producer's existing network
// phase decide eligibility. Neither numeric labels nor SubscriberID syntax
// participate. A useful initial L2TP round remains admitted.
func TestAuthAdmissionTransportAndReauthEligibility(t *testing.T) {
	birth, err := subscriberid.NewL2TPBirth()
	if err != nil {
		t.Fatal(err)
	}
	session := &pppSession{l2tpBirth: birth, pppoe: true}
	if binding, err := session.beginAuthAdmission(); err != nil || binding != nil {
		t.Fatalf("PPPoE admission = %v/%v", binding, err)
	}
	session.pppoe = false
	session.ncpStarted = true
	if binding, err := session.beginAuthAdmission(); err != nil || binding != nil {
		t.Fatalf("existing-network reauth admission = %v/%v", binding, err)
	}
	session.ncpStarted = false
	binding, err := session.beginAuthAdmission()
	if err != nil || binding == nil {
		t.Fatalf("useful initial L2TP admission = %v/%v", binding, err)
	}
	session.closeAuthAdmission()
	if _, err := session.beginAuthAdmission(); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("late original auth after Close = %v", err)
	}
}

// The same admitted worker can begin another original round. A retained old
// request/ACK can neither resolve nor abort the successor's provisional owner.
func TestAuthAdmissionOriginalAbortCannotAffectSuccessor(t *testing.T) {
	birth, err := subscriberid.NewL2TPBirth()
	if err != nil {
		t.Fatal(err)
	}
	session := &pppSession{l2tpBirth: birth}
	old, err := session.beginAuthAdmission()
	if err != nil {
		t.Fatal(err)
	}
	if !old.accept() {
		t.Fatal("original accept was refused")
	}
	session.abortAuthAdmission()
	next, err := session.beginAuthAdmission()
	if err != nil {
		t.Fatal(err)
	}
	if next == old {
		t.Fatal("successor reused the original auth round")
	}
	if old.accept() {
		t.Fatal("late old Accept revived an aborted round")
	}
	if old.finish(true) {
		t.Fatal("late old ACK revived an aborted round")
	}
	requireAuthAdmissionPending(t, AuthAdmissionReceipt{binding: next})
	if !next.accept() {
		t.Fatal("late old answers consumed useful successor authority")
	}
	lifetime, err := (l2tpevents.NetworkLifetime{}).Next()
	if err != nil {
		t.Fatal(err)
	}
	counters := NetworkCounterReceipt{network: &networkCounterLifetime{lifetime: lifetime, terminalDone: make(chan struct{})}}
	baseline := l2tpevents.MeasuredTrafficCounters(l2tpevents.TrafficTotals{RxBytes: 9})
	if !next.prepare(lifetime, baseline, counters) || !next.finish(true) {
		t.Fatal("useful successor could not transfer its own prepared original")
	}
	got, err := (AuthAdmissionReceipt{binding: next}).Assignment(context.Background())
	if err != nil || got.Birth != birth || got.NetworkLifetime != lifetime || got.CounterReceipt != counters {
		t.Fatalf("successor transfer = %+v/%v", got, err)
	}
	if _, err := (AuthAdmissionReceipt{binding: old}).Assignment(context.Background()); !errors.Is(err, ErrAuthAdmissionAborted) {
		t.Fatalf("successor changed old abort = %v", err)
	}
}

func requireAuthAdmissionPending(t *testing.T, receipt AuthAdmissionReceipt) {
	t.Helper()
	if !receipt.IsValid() {
		t.Fatal("pending check has no original admission")
	}
	select {
	case <-receipt.binding.done:
		t.Fatal("original admission resolved before assignment ACK or abort")
	default:
	}
}

// All driver workers/readers are test-owned and joined by Stop. The peer is
// closed first to release any in-flight real wire exchange on a failed test.
func startAuthAdmissionDriver(t *testing.T, configure ...func(*StartSession)) (*l2tpBirthProducerSession, EventAuthRequest) {
	t.Helper()
	birth, err := subscriberid.NewL2TPBirth()
	if err != nil {
		t.Fatal(err)
	}
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	start, peer, owner := frameLeaseStart(t, 11)
	t.Cleanup(func() { closeConn(peer); driver.Stop() })
	start.L2TPBirth = birth
	start.SubscriberID = "opaque-original-auth-owner"
	start.Attempt, err = NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	start.DisableIPCP = false
	handle, ok := owner.SessionHandle.(*testSessionHandle)
	if !ok {
		t.Fatalf("session handle = %T, want *testSessionHandle", owner.SessionHandle)
	}
	handle.readCounters = func() (SessionCounters, error) {
		return SessionCounters{RxBytes: 13, TxBytes: 27, RxPackets: 3, TxPackets: 5}, nil
	}
	frameLeaseProxy(&start)
	for _, option := range configure {
		option(&start)
	}
	driver.SessionsIn() <- start
	opened, ok := waitForEvent(t, driver.EventsOut(), time.Second).(EventLCPUp)
	if !ok {
		t.Fatal("actual admission did not reach LCP-up")
	}
	request := readProviderLifetimeAuthRequest(t, driver.AuthEventsOut())
	if request.Control != opened.Control {
		t.Fatal("actual auth changed original worker")
	}
	return &l2tpBirthProducerSession{driver: driver, peer: peer, birth: birth, control: opened.Control}, request
}

func acceptAuthAdmissionRequest(t *testing.T, session *l2tpBirthProducerSession, request EventAuthRequest) {
	t.Helper()
	if err := session.driver.AuthResponse(request, true, "", nil); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-session.driver.AuthEventsOut():
		success, ok := event.(eventAuthSuccess)
		if !ok || success.Control != session.control {
			t.Fatalf("actual consumed auth = %T/%+v", event, event)
		}
	case <-time.After(time.Second):
		t.Fatal("actual auth decision was not consumed")
	}
}

func prepareAuthAdmissionIPCP(t *testing.T, session *l2tpBirthProducerSession) EventSessionIPAssigned {
	t.Helper()
	select {
	case event := <-session.driver.IPEventsOut():
		request, ok := event.(EventIPRequest)
		if !ok || request.Control != session.control {
			t.Fatalf("actual IP provider = %T/%+v", event, event)
		}
		if err := session.driver.IPResponse(request, IPResponseArgs{Accept: true, Family: AddressFamilyIPv4, Local: ipcpTestLocal, Peer: ipcpTestPeer, DNSPrimary: ipcpTestDNS1, DNSSecondary: ipcpTestDNS2}); err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("actual IPCP did not request an address")
	}
	exchange := &ncpTestDriver{driver: session.driver, peer: session.peer}
	exchange.completeIPCP(t)
	assigned, ok := waitForEvent(t, session.driver.EventsOut(), time.Second).(EventSessionIPAssigned)
	if !ok {
		t.Fatal("actual IPCP did not publish an assignment")
	}
	return assigned
}
