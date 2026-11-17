// Design: docs/architecture/l2tp/subscriber-session-model.md -- retained original IPv6 child cleanup
package ppp

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/rtproto"
)

var errIPv6ChildCarrier = errors.New("original IPv6 carrier release remains unconfirmed")
var errIPv6ChildAddress = errors.New("original link-local withdrawal failed")
var errIPv6ChildAcquisition = errors.New("IPv6 carrier partially acquired")

// Actual service readers/RA run through the existing native-carrier seam. This
// provider caches ordinary Close, but owns its separate exact recovery method.
type retainedIPv6ChildCarrier struct {
	IPv6Carrier
	once          sync.Once
	firstErr      error
	fail          atomic.Bool
	closeAttempts atomic.Int32
	retries       atomic.Int32
	released      atomic.Bool
	finalRA       atomic.Int32
}

func (c *retainedIPv6ChildCarrier) Close() error {
	c.once.Do(func() {
		c.closeAttempts.Add(1)
		if c.fail.Load() {
			c.firstErr = errIPv6ChildCarrier
			return
		}
		c.firstErr = c.IPv6Carrier.Close()
		if c.firstErr == nil {
			c.released.Store(true)
		}
	})
	return c.firstErr
}
func (c *retainedIPv6ChildCarrier) RetryIPv6CarrierCleanup(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.retries.Add(1)
	if c.fail.Load() {
		return errIPv6ChildCarrier
	}
	if c.released.Load() {
		return nil
	}
	if err := c.IPv6Carrier.Close(); err != nil {
		return err
	}
	c.released.Store(true)
	return nil
}
func (c *retainedIPv6ChildCarrier) WriteRA(payload []byte, hop int) error {
	if len(payload) >= 8 && payload[6] == 0 && payload[7] == 0 {
		c.finalRA.Add(1)
	}
	return c.IPv6Carrier.WriteRA(payload, hop)
}

// Only native I/O/report identity is substituted. The worker/Attempt/requests,
// service startup, cancellation, physical Close and retirement are genuine.
type ipv6ChildPhysicalOwner struct {
	*genericCleanupLease
	birth             string
	failRemove        atomic.Bool
	failRoute         atomic.Bool
	postCloseRemoves  atomic.Int32
	physicallyClosed  atomic.Bool
	identityForbidden atomic.Bool
}

func (o *ipv6ChildPhysicalOwner) Identity() SessionIdentity {
	if o.identityForbidden.Load() {
		panic("BUG: child retry reread reused owner identity")
	}
	return SessionIdentity{InterfaceName: "ppp-reused-child", NativeIndex: 707}
}
func (o *ipv6ChildPhysicalOwner) Close() error {
	err := o.genericCleanupLease.Close()
	if err == nil {
		o.physicallyClosed.Store(true)
	}
	return err
}
func (o *ipv6ChildPhysicalOwner) RemoveAddress(prefix netip.Prefix) error {
	if o.physicallyClosed.Load() {
		o.postCloseRemoves.Add(1)
		return io.ErrClosedPipe
	}
	if o.failRemove.Load() {
		return errIPv6ChildAddress
	}
	return o.SessionHandle.RemoveAddress(prefix)
}
func (o *ipv6ChildPhysicalOwner) RemoveRoute(prefix netip.Prefix, via netip.Addr, table int, protocol rtproto.Proto) error {
	if o.physicallyClosed.Load() {
		o.postCloseRemoves.Add(1)
		return io.ErrClosedPipe
	}
	if o.failRoute.Load() {
		return errIPv6ChildAddress
	}
	return o.SessionHandle.RemoveRoute(prefix, via, table, protocol)
}
func (o *ipv6ChildPhysicalOwner) BindNativeReleaseSeal(NativeReleaseSeal) error { return nil }
func (o *ipv6ChildPhysicalOwner) NativeReleaseBound() bool                      { return true }
func (o *ipv6ChildPhysicalOwner) NativeReleasePending() bool                    { return false }
func (o *ipv6ChildPhysicalOwner) NativeReleaseBirthTag() string                 { return o.birth }

type ipv6ChildFixture struct {
	driver  *Driver
	owner   *ipv6ChildPhysicalOwner
	carrier *retainedIPv6ChildCarrier
	control SessionControl
	service *IPv6Service
	release func()
}

// The real admitted auth request is held only to keep the worker parked while
// the existing IPv6-service input seam installs its actual native resources.
func newIPv6ChildFixture(t *testing.T, number uint16, birth string, carrierFailure, partial bool) *ipv6ChildFixture {
	t.Helper()
	driver := NewProductionDriver(discardLogger())
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Stop)
	start, peer, original := frameLeaseStart(t, number)
	inner, ok := original.SessionHandle.(*testSessionHandle)
	if !ok {
		t.Fatalf("actual native fixture owner=%T", original.SessionHandle)
	}
	carrier := &retainedIPv6ChildCarrier{IPv6Carrier: newServiceCarrier()}
	carrier.fail.Store(carrierFailure)
	inner.openIPv6Carrier = func(context.Context) (IPv6Carrier, error) {
		if partial {
			return carrier, errors.Join(errIPv6ChildAcquisition, carrier.Close())
		}
		return carrier, nil
	}
	owner := &ipv6ChildPhysicalOwner{genericCleanupLease: &genericCleanupLease{SessionHandle: original.SessionHandle, peer: peer}, birth: birth}
	start.Dataplane = owner
	start.SubscriberID = "opaque/IPv6-child/" + birth
	var err error
	start.Attempt, err = NewSessionAttempt(start.SubscriberID, owner)
	if err != nil {
		t.Fatal(err)
	}
	frameLeaseProxy(&start)
	driver.SessionsIn() <- start
	opened, ok := waitForEvent(t, driver.EventsOut(), time.Second).(EventLCPUp)
	if !ok {
		t.Fatal("real child fixture omitted LCP-up")
	}
	var request EventAuthRequest
	select {
	case raw := <-driver.AuthEventsOut():
		request, ok = raw.(EventAuthRequest)
		if !ok {
			t.Fatal("real child fixture omitted auth request")
		}
	case <-time.After(time.Second):
		t.Fatal("real child fixture omitted auth wait")
	}
	if request.Control != opened.Control {
		t.Fatal("real child request changed worker")
	}
	request.pending.mu.Lock()
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { request.pending.mu.Unlock() }) }
	fixture := &ipv6ChildFixture{driver: driver, owner: owner, carrier: carrier, control: request.Control, release: release}
	t.Cleanup(func() {
		release()
		owner.identityForbidden.Store(false)
		owner.failRemove.Store(false)
		owner.failRoute.Store(false)
		carrier.fail.Store(false)
		driver.Stop()
		if err := driver.RetryPendingNativeRelease(context.Background()); err != nil {
			t.Error(err)
		}
	})
	request.Control.worker.mu.Lock()
	worker := request.Control.worker
	worker.localInterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 1}
	worker.peerInterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 2}
	worker.peerInterfaceIDNegotiated = true
	worker.afterLCPOpenIPv6Service()
	fixture.service = worker.ipv6Svc
	request.Control.worker.mu.Unlock()
	if fixture.service == nil {
		t.Fatal("actual startup lost its live/partial child cleanup owner")
	}
	return fixture
}
func (f *ipv6ChildFixture) retire(t *testing.T) SessionCleanupStatus {
	t.Helper()
	f.control.Cancel()
	f.release()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	status, err := f.control.WaitRetired(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return status
}
func (f *ipv6ChildFixture) retained(t *testing.T) bool {
	t.Helper()
	f.driver.mu.Lock()
	defer f.driver.mu.Unlock()
	return f.driver.owners[f.owner] == f.control.worker
}

func TestRetiredOwnerRetainsFailedIPv6CarrierUntilExactRecovery(t *testing.T) {
	fixture := newIPv6ChildFixture(t, 111, "birth/child-live", true, false)
	status := fixture.retire(t)
	if fixture.owner.closes.Load() == 0 {
		t.Fatal("failed child Stop suppressed necessary physical Close")
	}
	if !status.Retired || !status.Released || !errors.Is(status.Err, errIPv6ChildCarrier) {
		t.Fatalf("actual retained child outcome=%+v", status)
	}
	if !fixture.retained(t) {
		t.Fatal("retirement discarded original failed carrier cleanup authority")
	}
	if fixture.carrier.released.Load() {
		t.Fatal("physical unit release fabricated separate carrier cleanup")
	}
	if err := fixture.control.CloseCanceledOwner(t.Context()); err == nil {
		t.Fatal("cached physical success concealed pending child")
	}
	finalRA := fixture.carrier.finalRA.Load()
	if finalRA != 1 {
		t.Fatalf("actual service final RA count=%d, want1", finalRA)
	}
	fixture.carrier.fail.Store(false)
	fixture.owner.identityForbidden.Store(true)
	if err := fixture.driver.RetryPendingNativeRelease(t.Context()); err != nil {
		t.Fatal(err)
	}
	status, err := fixture.control.CleanupStatus()
	if err != nil || !status.Released || status.Err != nil {
		t.Fatalf("exact child recovery=%+v,%v", status, err)
	}
	if fixture.retained(t) {
		t.Fatal("confirmed child recovery retained original index")
	}
	if !fixture.carrier.released.Load() {
		t.Fatal("retry did not release original carrier")
	}
	if fixture.owner.closes.Load() != 1 || fixture.carrier.closeAttempts.Load() != 1 {
		t.Fatal("child retry repeated native/ordinary carrier Close")
	}
	if fixture.carrier.finalRA.Load() != finalRA {
		t.Fatal("child retry repeated worker stop/final RA")
	}
	retries := fixture.carrier.retries.Load()
	if err := fixture.control.RetryNativeRelease(); err != nil {
		t.Fatal(err)
	}
	if fixture.carrier.retries.Load() != retries {
		t.Fatal("completed old retry touched a released carrier again")
	}
}

func TestRetiredOwnerRetainsFailedPartialIPv6Carrier(t *testing.T) {
	fixture := newIPv6ChildFixture(t, 112, "birth/child-partial", true, true)
	status := fixture.retire(t)
	if fixture.owner.closes.Load() == 0 {
		t.Fatal("partial child failure prevented physical Close")
	}
	if !status.Retired || !status.Released || !errors.Is(status.Err, errIPv6ChildCarrier) {
		t.Fatalf("partial child cleanup=%+v", status)
	}
	if !fixture.retained(t) {
		t.Fatal("partial startup lost its exact failed carrier after retirement")
	}
	if fixture.carrier.finalRA.Load() != 0 {
		t.Fatal("partial acquisition started service/borrowed missing lanes")
	}
	fixture.carrier.fail.Store(false)
	if err := fixture.driver.RetryPendingNativeRelease(t.Context()); err != nil {
		t.Fatal(err)
	}
	status, err := fixture.control.CleanupStatus()
	if err != nil || status.Err != nil || !fixture.carrier.released.Load() {
		t.Fatalf("partial carrier recovery=%+v,%v", status, err)
	}
	if fixture.retained(t) {
		t.Fatal("confirmed partial recovery retained cleanup index")
	}
	if fixture.carrier.closeAttempts.Load() != 1 {
		t.Fatal("partial rollback repeated ordinary cached Close")
	}
}

func TestPhysicalReleaseRetiresOnlyOriginalIPv6UnitWitnesses(t *testing.T) {
	fixture := newIPv6ChildFixture(t, 113, "birth/child-unit", false, false)
	prefix := netip.MustParsePrefix("2001:db8:707::/64")
	releases := 0
	var releasedPrefix netip.Prefix
	fixture.service.mu.Lock()
	fixture.service.cfg.ReleasePrefix = func(actual netip.Prefix) { releasedPrefix = actual; releases++ }
	fixture.service.mu.Unlock()
	if err := fixture.service.installRoute(prefix); err != nil {
		t.Fatal(err)
	}
	fixture.owner.failRemove.Store(true)
	fixture.owner.failRoute.Store(true)
	entered := make(chan struct{})
	resume := make(chan struct{})
	fixture.owner.entered, fixture.owner.resume = entered, resume
	var openOnce sync.Once
	open := func() { openOnce.Do(func() { close(resume) }) }
	t.Cleanup(open)
	fixture.control.Cancel()
	fixture.release()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("physical Close was suppressed by failed link-local cleanup")
	}
	status, err := fixture.control.CleanupStatus()
	if err != nil || !errors.Is(status.Err, errIPv6ChildAddress) {
		t.Fatalf("child error lost before physical confirmation: %+v,%v", status, err)
	}
	if status.Released {
		t.Fatal("blocked physical Close invented confirmation")
	}
	fixture.service.mu.Lock()
	retainedRoute, retainedPrefix := fixture.service.routeInstalled, fixture.service.delegatedPrefix
	fixture.service.mu.Unlock()
	if !retainedRoute || retainedPrefix != prefix || releases != 0 {
		t.Fatal("unconfirmed physical deletion discarded old route/prefix witness")
	}
	open()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	status, err = fixture.control.WaitRetired(ctx)
	if err != nil || status.Err != nil || !status.Released {
		t.Fatalf("confirmed old unit witness retirement=%+v,%v", status, err)
	}
	fixture.service.mu.Lock()
	address, route, pendingPrefix := fixture.service.linkLocalInstalled, fixture.service.routeInstalled, fixture.service.delegatedPrefix
	fixture.service.mu.Unlock()
	if address || route || pendingPrefix.IsValid() {
		t.Fatal("physical unit deletion left old unit-scoped witnesses owned")
	}
	if releases != 1 || releasedPrefix != prefix {
		t.Fatal("captured delegated prefix callback was not released exactly once for its original prefix")
	}
	if fixture.owner.postCloseRemoves.Load() != 0 {
		t.Fatal("retirement deleted by already-closed/reusable unit identity")
	}
	if fixture.owner.closes.Load() != 1 {
		t.Fatal("witness retirement repeated confirmed physical Close")
	}
}

func TestIPv6ChildRecoveryDoesNotClearReusedNativeIdentityWarning(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	old := newIPv6ChildFixture(t, 114, "birth/child-old", true, false)
	oldStatus := old.retire(t)
	if !errors.Is(oldStatus.Err, errIPv6ChildCarrier) {
		t.Fatal("old carrier failure was not retained")
	}
	successor := newIPv6ChildFixture(t, 115, "birth/child-successor", true, false)
	newStatus := successor.retire(t)
	if !errors.Is(newStatus.Err, errIPv6ChildCarrier) {
		t.Fatal("successor carrier failure was not retained")
	}
	hasBirth := func(birth string) bool {
		for _, warning := range report.Warnings() {
			if warning.Detail["native-birth"] == birth && warning.Detail["operation"] == CleanupIPv6.String() {
				return true
			}
		}
		return false
	}
	if !hasBirth(old.owner.birth) || !hasBirth(successor.owner.birth) {
		t.Fatal("same-index child warnings were not birth-scoped")
	}
	oldCloses, newCloses := old.owner.closes.Load(), successor.owner.closes.Load()
	successorRetries := successor.carrier.retries.Load()
	old.owner.identityForbidden.Store(true)
	successor.owner.identityForbidden.Store(true)
	old.carrier.fail.Store(false)
	if err := old.driver.RetryPendingNativeRelease(t.Context()); err != nil {
		t.Fatal(err)
	}
	if hasBirth(old.owner.birth) {
		t.Fatal("old recovery did not clear its own exact child warning")
	}
	if !hasBirth(successor.owner.birth) {
		t.Fatal("old cleanup cleared successor warning at reused interface identity")
	}
	if old.owner.closes.Load() != oldCloses || successor.owner.closes.Load() != newCloses {
		t.Fatal("child recovery reclosed a physical/successor owner")
	}
	if successor.carrier.retries.Load() != successorRetries {
		t.Fatal("old recovery called successor carrier")
	}
}
