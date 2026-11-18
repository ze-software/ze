// Design: docs/architecture/l2tp/subscriber-session-model.md -- captured original retirement
// Related: session_retirement.go -- cleanupCapturedOriginal

package l2tp

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// A concurrent first Close is a waitable original retirement, not a failed
// clear. Hold the real admitted owner's Close until cleanup joins its worker.
func TestCapturedOriginalCleanupWaitsForInflightPPPClose(t *testing.T) {
	rig := newOriginalRetirementWait(t, nil, nil)
	rig.blockOriginalClose(t)
	wait, result := rig.startCleanup(t)
	awaitOriginalRetirementWait(t, wait, result)
	rig.owner.release()
	if err := originalRetirementResult(t, result); err != nil {
		t.Fatalf("confirmed original cleanup failed: %v", err)
	}
	rig.requireReleased(t)
	if got := rig.owner.closes.Load(); got != 1 {
		t.Fatalf("successful original Close calls=%d, want one", got)
	}
}

// Reaching worker retirement cannot turn an actual original Close failure
// into success. The retained row must continue to expose that same cause.
func TestCapturedOriginalCleanupRetainsInflightPPPCloseFailure(t *testing.T) {
	failure := errors.New("original physical owner close failed")
	rig := newOriginalRetirementWait(t, nil, failure)
	rig.blockOriginalClose(t)
	wait, result := rig.startCleanup(t)
	awaitOriginalRetirementWait(t, wait, result)
	rig.owner.release()
	if err := originalRetirementResult(t, result); !errors.Is(err, failure) {
		t.Fatalf("original physical failure lost: %v", err)
	}
	status, err := rig.control.CleanupStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Retired {
		t.Fatal("failed Close did not finish original worker retirement")
	}
	if status.Released {
		t.Fatal("failed original Close was certified released")
	}
	if !errors.Is(status.Err, failure) {
		t.Fatalf("original status lost physical failure: %v", status.Err)
	}
	if err := retiredSessionCleanup(rig.session); !errors.Is(err, failure) {
		t.Fatalf("retained original row lost physical failure: %v", err)
	}
}

// Cancel while the real original owner remains inside Close. The command
// must return cancellation without claiming release or joining physical I/O.
func TestCapturedOriginalCleanupCancellationInterruptsRetirementWait(t *testing.T) {
	rig := newOriginalRetirementWait(t, nil, nil)
	rig.blockOriginalClose(t)
	wait, result := rig.startCleanup(t)
	awaitOriginalRetirementWait(t, wait, result)
	wait.cancel()
	if err := originalRetirementResult(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled original retirement wait=%v", err)
	}
	status, err := rig.control.CleanupStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Released {
		t.Fatal("cancellation certified a still-blocked physical Close")
	}
	if status.Err != nil {
		t.Fatalf("cancellation replaced the unfinished physical result: %v", status.Err)
	}
	rig.owner.release()
	rig.requireReleased(t)
}

// Reuse the actual Driver key after original retirement, replace the NAS SID
// row, then retry the old pointer. Its completed owner must stay closed once
// while the genuinely admitted same-ID successor remains live and untouched.
func TestCapturedOriginalCleanupRetryPreservesSameIDSuccessor(t *testing.T) {
	original := newOriginalRetirementWait(t, nil, nil)
	original.blockOriginalClose(t)
	wait, result := original.startCleanup(t)
	awaitOriginalRetirementWait(t, wait, result)
	original.owner.release()
	if err := originalRetirementResult(t, result); err != nil {
		t.Fatal(err)
	}
	original.requireReleased(t)
	select {
	case event := <-original.driver.EventsOut():
		down, valid := event.(ppp.EventSessionDown)
		if !valid {
			t.Fatalf("original retirement event=%T, want session-down", event)
		}
		if down.Control != original.control {
			t.Fatal("original session-down changed admitted worker")
		}
	case <-time.After(time.Second):
		t.Fatal("original retirement omitted session-down")
	}
	successor := newOriginalRetirementWait(t, original.driver, nil)
	if successor.session.pppAttempt == original.session.pppAttempt {
		t.Fatal("same-ID successor reused the original admission")
	}
	original.reactor.tunnelsMu.Lock()
	original.tunnel.sessions[original.session.localSID] = successor.session
	original.reactor.tunnelsMu.Unlock()
	if err := original.reactor.cleanupCapturedOriginal(t.Context(), original.tunnel, original.session); err != nil {
		t.Fatalf("old original retry failed after replacement: %v", err)
	}
	if !successor.control.Active() {
		t.Fatal("old original cleanup canceled the same-ID successor")
	}
	if got := successor.owner.closes.Load(); got != 0 {
		t.Fatalf("old original cleanup closed successor %d times", got)
	}
	if got := original.owner.closes.Load(); got != 1 {
		t.Fatalf("old original retry repeated successful physical Close: %d", got)
	}
	if original.tunnel.sessions[original.session.localSID] != successor.session {
		t.Fatal("old original retry removed the replacement NAS row")
	}
}

type originalRetirementOwner struct {
	*currentCoreHandle
	entered     chan struct{}
	resume      chan struct{}
	enterOnce   sync.Once
	releaseOnce sync.Once
	closes      atomic.Int32
	fail        atomic.Bool
	failure     error
}

func (owner *originalRetirementOwner) Close() error {
	owner.closes.Add(1)
	err := owner.currentCoreHandle.Close()
	owner.enterOnce.Do(func() { close(owner.entered) })
	<-owner.resume
	if owner.fail.Load() {
		return errors.Join(err, owner.failure)
	}
	return err
}

func (owner *originalRetirementOwner) release() {
	owner.releaseOnce.Do(func() { close(owner.resume) })
}

type originalRetirementWait struct {
	reactor *l2tpReactor
	tunnel  *L2TPTunnel
	session *L2TPSession
	driver  *ppp.Driver
	owner   *originalRetirementOwner
	control ppp.SessionControl
}

// Use the concrete pipe provider, the public admission queue and actual
// lifecycle/auth requests; no private PPP state or fabricated Control is used.
func newOriginalRetirementWait(t *testing.T, driver *ppp.Driver, failure error) *originalRetirementWait {
	t.Helper()
	native, peer := net.Pipe()
	owner := &originalRetirementOwner{
		currentCoreHandle: &currentCoreHandle{Conn: native, identity: ppp.SessionIdentity{InterfaceName: "ppp91", NativeIndex: 91}},
		entered:           make(chan struct{}), resume: make(chan struct{}), failure: failure,
	}
	owner.fail.Store(failure != nil)
	if driver == nil {
		driver = ppp.NewProductionDriver(slog.New(slog.DiscardHandler))
		if err := driver.Start(); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		owner.fail.Store(false)
		owner.release()
		if err := peer.Close(); err != nil {
			t.Error(err)
		}
		driver.Stop()
		if err := driver.RetryPendingNativeRelease(context.Background()); err != nil {
			t.Error(err)
		}
	})
	attempt, err := ppp.NewSessionAttempt("opaque/original-retirement", owner)
	if err != nil {
		t.Fatal(err)
	}
	lcp, err := ppp.NewLCPConfig(3*time.Second, 2, 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	options := []byte{1, 4, 5, 220, 5, 6, 0xca, 0xfe, 0xba, 0xbe}
	driver.SessionsIn() <- ppp.StartSession{
		LCP: lcp, TunnelID: 7, SessionID: 91, SubscriberID: attempt.SubscriberID(), Attempt: attempt,
		Frames: owner, Dataplane: owner, LNSMode: true, DisableIPCP: true, DisableIPv6CP: true,
		AuthTimeout: time.Hour, EchoInterval: time.Hour,
		ProxyLCPInitialRecv: options, ProxyLCPLastSent: options, ProxyLCPLastRecv: options,
	}
	var opened ppp.EventLCPUp
	select {
	case event := <-driver.EventsOut():
		var valid bool
		opened, valid = event.(ppp.EventLCPUp)
		if !valid {
			t.Fatalf("actual admission event=%T, want LCP-up", event)
		}
	case <-time.After(time.Second):
		t.Fatal("actual original admission did not reach LCP-up")
	}
	select {
	case event := <-driver.AuthEventsOut():
		request, valid := event.(ppp.EventAuthRequest)
		if !valid {
			t.Fatalf("actual auth wait event=%T", event)
		}
		if request.Control != opened.Control {
			t.Fatal("actual auth request changed admitted worker")
		}
	case <-time.After(time.Second):
		t.Fatal("actual original admission did not reach auth wait")
	}
	if opened.Control.Attempt() != attempt {
		t.Fatal("actual admission did not retain original attempt")
	}
	session := &L2TPSession{localSID: 91, pppAttempt: attempt}
	tunnel := &L2TPTunnel{localTID: 7, sessions: map[uint16]*L2TPSession{91: session}}
	reactor := &l2tpReactor{tunnelsByLocalID: map[uint16]*L2TPTunnel{7: tunnel},
		retiredSessions: map[*L2TPSession]*L2TPTunnel{session: tunnel}}
	return &originalRetirementWait{reactor: reactor, tunnel: tunnel, session: session,
		driver: driver, owner: owner, control: opened.Control}
}

func (rig *originalRetirementWait) blockOriginalClose(t *testing.T) {
	t.Helper()
	if !rig.session.pppAttempt.Cancel() {
		t.Fatal("original attempt could not be canceled")
	}
	select {
	case <-rig.owner.entered:
	case <-time.After(time.Second):
		t.Fatal("original worker did not enter actual owner Close")
	}
}

// Done records the real retirement wait's cancellation selection. Delegating
// every context method preserves ordinary cancellation/deadline behavior.
type originalRetirementContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
	cancel  context.CancelFunc
}

func (ctx *originalRetirementContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.entered) })
	return ctx.Context.Done()
}

// Own and join this one finite command call, including after a failed oracle.
func (rig *originalRetirementWait) startCleanup(t *testing.T) (*originalRetirementContext, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	wait := &originalRetirementContext{Context: ctx, entered: make(chan struct{}), cancel: cancel}
	result := make(chan error, 1)
	finished := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		rig.owner.release()
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Error("captured original cleanup did not end during cleanup")
		}
	})
	go func() {
		defer close(finished)
		result <- rig.reactor.cleanupCapturedOriginal(wait, rig.tunnel, rig.session)
	}()
	return wait, result
}

func awaitOriginalRetirementWait(t *testing.T, wait *originalRetirementContext, result <-chan error) {
	t.Helper()
	select {
	case <-wait.entered:
	case err := <-result:
		t.Fatalf("captured original cleanup returned before its in-flight Close retired: %v", err)
	case <-time.After(time.Second):
		t.Fatal("captured original cleanup did not join its exact retirement wait")
	}
	select {
	case err := <-result:
		t.Fatalf("captured original cleanup returned while its physical Close was blocked: %v", err)
	default:
	}
}

func originalRetirementResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(time.Second):
		t.Fatal("captured original cleanup did not finish after its decisive event")
		return nil
	}
}

func (rig *originalRetirementWait) requireReleased(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	status, err := rig.control.WaitRetired(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Retired {
		t.Fatal("original worker did not retire")
	}
	if !status.Released {
		t.Fatal("successful original Close was not confirmed released")
	}
	if status.Err != nil {
		t.Fatalf("successful original Close retained error: %v", status.Err)
	}
}
