//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- actual setup/close refusals
// Related: client_native_owner.go -- createOriginalPair, bindOriginalPair and cleanup

package pppoeclient

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/pppcpefault"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

type setupFaultCounterCreator interface {
	CreateLinkCounterReader(*nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error)
}
type setupFaultBackend struct {
	iface.Backend
	native      iface.NativePPPBackend
	counters    setupFaultCounterCreator
	denyRestore bool
	writer      iface.NativePPPWriter
	bindResult  error
	closeCalls  atomic.Uint32
}

func (b *setupFaultBackend) CreateLinkCounterReader(request *nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error) {
	return b.counters.CreateLinkCounterReader(request)
}
func (b *setupFaultBackend) BindNativePPPUnit(owner iface.NativePPPUnitOwner, index int, unit int32) (iface.NativePPPWriter, error) {
	writer, err := b.native.BindNativePPPUnit(owner, index, unit)
	b.writer, b.bindResult = writer, err
	if err != nil {
		return writer, err
	}
	if b.denyRestore {
		// This is the actual locked joined namespace worker after real bind.
		// Its next real restore setns fails in the kernel; no result is forged.
		if err := pppcpefault.DenyWorkerRestore(); err != nil {
			return writer, err
		}
	}
	return writer, nil
}
func (b *setupFaultBackend) Close() error { b.closeCalls.Add(1); return b.Backend.Close() }

type setupFaultObservation struct {
	bindErr, restoreErr error
	sameCaller          bool
}
type setupFaultResult struct {
	session iface.PPPoESession
	err     error
}
type setupFaultDialer struct {
	owner          *clientNativeOwner
	denyRestore    bool
	denyChannel    bool
	calls          atomic.Uint32
	first          chan setupFaultResult
	observed       chan setupFaultObservation
	cleanupEntered chan struct{}
	cleanupRelease chan struct{}
	cleanupOnce    sync.Once
	cleanupCalls   atomic.Uint32
	firstCleanup   chan error
	secondCleanup  chan error
	workerStarts   atomic.Uint32
}

func (d *setupFaultDialer) Dial(cfg iface.PPPoEClientConfig, stop <-chan struct{}, log *slog.Logger) (iface.PPPoESession, error) { //nolint:gocritic // hugeParam: actual CPE dial boundary.
	if d.calls.Add(1) != 1 {
		return iface.PPPoESession{}, errors.New("setup fault wrongly admitted another native Dial")
	}
	session, err := (&Dialer{}).dialPrepared(cfg, stop, log, func() (*clientDialPreparation, error) {
		return &clientDialPreparation{setup: d.setup, close: d.owner.close,
			nativeWriter: func() iface.NativePPPWriter { return d.owner.writer }}, nil
	})
	if session.Cleanup != nil {
		original := session.Cleanup
		session.Cleanup = func() error {
			attempt := d.cleanupCalls.Add(1)
			d.cleanupOnce.Do(func() { close(d.cleanupEntered) })
			<-d.cleanupRelease
			err := original()
			if attempt == 1 {
				d.firstCleanup <- err
			}
			if attempt == 2 {
				d.secondCleanup <- err
			}
			return err
		}
	}
	d.first <- setupFaultResult{session: session, err: err}
	return session, err
}

func (d *setupFaultDialer) setup(transport *os.File, name string) (client clientProtocol, result error) {
	if err := d.owner.createOriginalPair(transport, name); err != nil {
		return nil, err
	}
	if !d.denyRestore {
		client, err := d.owner.bindOriginalPair()
		if err != nil {
			return nil, err
		}
		if !d.denyChannel {
			return client, nil
		}
		// Only this genuine original channel close syscall is denied. Unit
		// Close remains able to advance, so no artificial dependency cycle is
		// inserted between original retirement and writer settlement.
		err = d.owner.channel.WithDescriptor(pppcpefault.DenyOriginalClose)
		return client, err
	}
	// Install the all-thread close fault before the disposable namespace worker
	// acquires its private restore filter. Synchronizing filters after a failed
	// restore would race that worker's real locked-thread retirement.
	if err := d.owner.channel.WithDescriptor(pppcpefault.DenyOriginalClose); err != nil {
		return nil, err
	}
	if err := pppkernel.SetNativePPPWorkerStartForTest(d.owner.unit, func() error {
		d.workerStarts.Add(1)
		foreign, err := netns.New()
		if err != nil {
			return err
		}
		return foreign.Close()
	}); err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	original, err := netns.Get()
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, original.Close()) }()
	// The SAME actual worker starts in its genuine new namespace before capture,
	// enters the unit's original namespace, binds, then attempts the denied restore.
	// This independently locked caller stays in its own original namespace.
	client, bindErr := d.owner.bindOriginalPair()
	var restoreErr error
	var sameCaller bool
	if restoreErr == nil {
		current, currentErr := netns.Get()
		if currentErr == nil {
			var before, after unix.Stat_t
			beforeErr := unix.Fstat(int(original), &before)
			afterErr := unix.Fstat(int(current), &after)
			if beforeErr == nil {
				if afterErr == nil {
					sameCaller = before.Dev == after.Dev
					if sameCaller {
						sameCaller = before.Ino == after.Ino
					}
				}
			}
			restoreErr = errors.Join(restoreErr, beforeErr, afterErr, current.Close())
		} else {
			restoreErr = currentErr
		}
	}
	d.observed <- setupFaultObservation{bindErr: bindErr, restoreErr: restoreErr, sameCaller: sameCaller}
	if restoreErr != nil {
		return client, errors.Join(bindErr, restoreErr)
	}
	if bindErr == nil {
		return client, errors.New("actual namespace restore unexpectedly succeeded")
	}
	// The already-installed genuine channel Close failure keeps this real
	// partial facet reachable when bind/restore error propagates out of Dial.
	return client, bindErr
}

type setupFaultFixture struct {
	owner       *clientNativeOwner
	backend     *setupFaultBackend
	dialer      *setupFaultDialer
	client      *iface.PPPoEClient
	ctx         context.Context
	peerReady   chan *pppoeac.Session
	peerDone    chan error
	releaseOnce sync.Once
}

func newSetupFaultFixture(t *testing.T, restore bool) *setupFaultFixture {
	t.Helper()
	return newSetupFaultFixtureWith(t, restore, true)
}
func newSetupFaultFixtureWith(t *testing.T, restore, denyChannel bool) *setupFaultFixture {
	t.Helper()
	raw, err := iface.OpenBackend("netlink")
	if err != nil {
		t.Fatal(err)
	}
	native, ok := raw.(iface.NativePPPBackend)
	if !ok {
		t.Fatal(errors.Join(errors.New("actual backend has no native CPE binding"), raw.Close()))
	}
	counters, ok := raw.(setupFaultCounterCreator)
	if !ok {
		t.Fatal(errors.Join(errors.New("actual backend has no original counter creator"), raw.Close()))
	}
	backend := &setupFaultBackend{Backend: raw, native: native, counters: counters, denyRestore: restore}
	owner := finalCounterNativeOwner(t, backend)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	h := &setupFaultFixture{owner: owner, backend: backend, ctx: ctx, peerReady: make(chan *pppoeac.Session, 1), peerDone: make(chan error, 1)}
	h.dialer = &setupFaultDialer{owner: owner, denyRestore: restore, denyChannel: denyChannel, first: make(chan setupFaultResult, 1), observed: make(chan setupFaultObservation, 1),
		cleanupEntered: make(chan struct{}), cleanupRelease: make(chan struct{}), firstCleanup: make(chan error, 1), secondCleanup: make(chan error, 1)}
	peer := pppoeac.OpenVeth(t, "zsf0", "zsa0", 92)
	go func() {
		session, err := peer.Accept(ctx)
		if err != nil {
			h.peerDone <- err
			return
		}
		if !restore {
			options := []byte{3, 5, 0xc2, 0x23, 5, 5, 6, 0x10, 0x20, 0x30, 0x40}
			if err := session.LCP(ctx, 70, options, false); err != nil {
				h.peerDone <- err
				return
			}
			if err := session.CHAP(ctx, 80, []byte{1, 3, 5, 7}, "subscriber", "secret"); err != nil {
				h.peerDone <- err
				return
			}
			if err := session.IPCP(ctx, 90, netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")); err != nil {
				h.peerDone <- err
				return
			}
		}
		h.peerReady <- session
		<-ctx.Done()
		h.peerDone <- nil
	}()
	h.client = iface.NewPPPoEClient(iface.PPPoEClientConfig{Name: "setupwan", SourceInterface: "zsf0", MTU: 1400, RoutePriority: 73,
		Timing: testDialTiming, Username: "subscriber", AuthSecret: "secret"}, h.dialer, slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() {
		h.releaseCleanup()
		err := h.client.Stop()
		if denyChannel || restore {
			if !errors.Is(err, iface.ErrNativePPPPending) {
				t.Errorf("actual channel failure lost custody: %v", err)
			}
			if !errors.Is(err, unix.EPERM) {
				t.Errorf("actual syscall refusal lost: %v", err)
			}
		} else if err != nil {
			t.Error(err)
		}
		cancel()
		if err := <-h.peerDone; err != nil {
			t.Error(err)
		}
		// No raw numeric retry is safe. The isolated child exits after its
		// Pending assertions and the kernel releases its intentionally held fd.
	})
	h.client.Start()
	return h
}
func (h *setupFaultFixture) releaseCleanup() {
	h.releaseOnce.Do(func() { close(h.dialer.cleanupRelease) })
}
func setupFaultCleanupResult(t *testing.T, h *setupFaultFixture, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-h.ctx.Done():
		t.Fatal("actual setup cleanup result missing")
		return h.ctx.Err()
	}
}

// TestNativeCPEOriginalNamespaceRestoreFailureRetainsPartialFacet denies the
// actual restore syscall on the joined native binding worker AFTER its real
// original socket/probe succeeds. A separately denied actual channel close
// keeps the genuine partial facet/cleanup live while the unit still advances.
// The caller thread is restored independently and no namespace observation is
// ever read as a release grant.
func TestNativeCPEOriginalNamespaceRestoreFailureRetainsPartialFacet(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := newSetupFaultFixture(t, true)
	var observed setupFaultObservation
	select {
	case observed = <-h.dialer.observed:
	case <-h.ctx.Done():
		t.Fatal("actual joined bind/restore never returned")
	}
	if h.backend.bindResult != nil {
		t.Fatalf("real native bind itself failed: %v", h.backend.bindResult)
	}
	if h.backend.writer == nil {
		t.Fatal("real successful bind returned no facet")
	}
	if !errors.Is(observed.bindErr, unix.EPERM) {
		t.Fatalf("actual worker restore refusal lost: %v", observed.bindErr)
	}
	if observed.restoreErr != nil {
		t.Fatalf("caller thread failed restoration: %v", observed.restoreErr)
	}
	if !observed.sameCaller {
		t.Fatal("joined worker changed caller namespace")
	}
	if h.dialer.workerStarts.Load() != 1 {
		t.Fatalf("genuine joined worker starts = %d", h.dialer.workerStarts.Load())
	}
	var first setupFaultResult
	select {
	case first = <-h.dialer.first:
	case <-h.ctx.Done():
		t.Fatal("partial Dial did not return")
	}
	if first.session.Cleanup == nil {
		t.Fatal("actual namespace error forgot pending partial cleanup")
	}
	if first.session.NativeWriter != h.backend.writer {
		t.Fatal("namespace error replaced the same partial native facet")
	}
	if !errors.Is(first.err, iface.ErrNativePPPPending) {
		t.Fatalf("failed original channel close became terminal: %v", first.err)
	}
	if !h.owner.unit.NativeOperationsPending() {
		t.Fatal("shared original discarded actual partial binding")
	}
	h.releaseCleanup()
	for _, result := range []<-chan error{h.dialer.firstCleanup, h.dialer.secondCleanup} {
		err := setupFaultCleanupResult(t, h, result)
		if !errors.Is(err, unix.EPERM) {
			t.Fatalf("actual restore/close causes lost: %v", err)
		}
		if !errors.Is(err, iface.ErrNativePPPPending) {
			t.Fatalf("real partial custody lost: %v", err)
		}
	}
	if err := h.owner.unit.WaitNativePPPUnitRetired(h.ctx); err != nil {
		t.Fatalf("unit close failed to advance: %v", err)
	}
	if err := h.owner.channel.WaitNativePPPChannelRetired(h.ctx); !errors.Is(err, unix.EPERM) {
		t.Fatalf("actual failed channel Close granted release: %v", err)
	}
	if h.backend.closeCalls.Load() != 0 {
		t.Fatal("namespace/physical uncertainty released original backend")
	}
	if h.dialer.calls.Load() != 1 {
		t.Fatal("partial namespace/close failure admitted another Dial")
	}
}

// TestNativeCPEFailedOriginalChannelCloseStillAdvancesUnit starts from actual
// successful PPP negotiation and useful iface writes. Peer PADT drives exact
// disconnect and the denied channel syscall. Cleanup MUST still close the unit,
// but retain the channel/facet/backend and same callback until physical custody
// is resolved; completed channel admission flags are not physical evidence.
func TestNativeCPEFailedOriginalChannelCloseStillAdvancesUnit(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := newSetupFaultFixture(t, false)
	var first setupFaultResult
	select {
	case first = <-h.dialer.first:
	case <-h.ctx.Done():
		t.Fatal("actual successful Dial missing")
	}
	if first.err != nil {
		t.Fatal(first.err)
	}
	index := int(h.owner.unit.Identity().NativeIndex)
	waitNativeCPEAddress(t, h.ctx, index, netip.MustParsePrefix("192.0.2.2/32"))
	waitFinalCPEInitialRoute(t, h.ctx, index, "192.0.2.1")
	var peer *pppoeac.Session
	select {
	case peer = <-h.peerReady:
	case <-h.ctx.Done():
		t.Fatal("actual wire epoch did not finish")
	}
	if err := peer.SendPADT(); err != nil {
		t.Fatal(err)
	}
	h.releaseCleanup()
	for _, result := range []<-chan error{h.dialer.firstCleanup, h.dialer.secondCleanup} {
		err := setupFaultCleanupResult(t, h, result)
		if !errors.Is(err, unix.EPERM) {
			t.Fatalf("actual channel syscall failure lost: %v", err)
		}
		if !errors.Is(err, iface.ErrNativePPPPending) {
			t.Fatalf("live channel file became terminal history: %v", err)
		}
	}
	if err := h.owner.unit.WaitNativePPPUnitRetired(h.ctx); err != nil {
		t.Fatalf("unit was blocked behind failed channel: %v", err)
	}
	if err := h.owner.channel.WaitNativePPPChannelRetired(h.ctx); !errors.Is(err, unix.EPERM) {
		t.Fatalf("channel first failed close became release: %v", err)
	}
	if !h.owner.unit.NativeOperationsPending() {
		t.Fatal("failed channel close discarded actual native facet")
	}
	if !h.backend.writer.ClosePending() {
		t.Fatal("writer closed while actual original channel remained owned")
	}
	if h.backend.closeCalls.Load() != 0 {
		t.Fatal("failed actual channel Close released backend")
	}
	if h.dialer.calls.Load() != 1 {
		t.Fatal("failed actual channel Close admitted another Dial")
	}
}

// TestNativeCPEChannelBorrowBlocksRetirementUntilActualClose holds a genuine
// RawConn borrow of the same live channel through consumer-triggered shutdown.
// The passive witness refuses before actual Close completes, then succeeds
// after that borrow and Close finish. No metadata or context cancellation grants
// early retirement; successful physical closure permits writer/backend release.
func TestNativeCPEChannelBorrowBlocksRetirementUntilActualClose(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	h := newSetupFaultFixtureWith(t, false, false)
	var first setupFaultResult
	select {
	case first = <-h.dialer.first:
	case <-h.ctx.Done():
		t.Fatal("actual healthy Dial missing")
	}
	if first.err != nil {
		t.Fatal(first.err)
	}
	waitNativeCPEAddress(t, h.ctx, int(h.owner.unit.Identity().NativeIndex), netip.MustParsePrefix("192.0.2.2/32"))
	entered, release := make(chan struct{}), make(chan struct{})
	borrowDone := make(chan error, 1)
	borrowJoined := false
	var releaseOnce sync.Once
	releaseBorrow := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(func() {
		releaseBorrow()
		if !borrowJoined {
			join, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
			defer cancel()
			select {
			case err := <-borrowDone:
				if err != nil {
					t.Error(err)
				}
			case <-join.Done():
				t.Error("actual channel borrower did not join")
			}
		}
	})
	go func() {
		borrowDone <- h.owner.channel.WithDescriptor(func(int) error { close(entered); <-release; return nil })
	}()
	select {
	case <-entered:
	case <-h.ctx.Done():
		t.Fatal("actual original channel borrow missing")
	}
	probe, cancel := context.WithCancel(context.Background())
	cancel()
	if err := h.owner.channel.WaitNativePPPChannelRetired(probe); !errors.Is(err, context.Canceled) {
		t.Fatalf("borrowed live channel fabricated retirement: %v", err)
	}
	var peer *pppoeac.Session
	select {
	case peer = <-h.peerReady:
	case <-h.ctx.Done():
		t.Fatal("actual peer negotiation missing")
	}
	if err := peer.SendPADT(); err != nil {
		t.Fatal(err)
	}
	if err := h.owner.channel.WaitNativePPPChannelRetired(probe); !errors.Is(err, context.Canceled) {
		t.Fatalf("channel borrow became physical release: %v", err)
	}
	releaseBorrow()
	select {
	case err := <-borrowDone:
		borrowJoined = true
		if err != nil {
			t.Fatal(err)
		}
	case <-h.ctx.Done():
		t.Fatal("actual borrower did not finish after release")
	}
	h.releaseCleanup()
	if err := setupFaultCleanupResult(t, h, h.dialer.firstCleanup); err != nil {
		t.Fatalf("genuine successful cleanup: %v", err)
	}
	if err := h.owner.channel.WaitNativePPPChannelRetired(probe); err != nil {
		t.Fatalf("completed actual channel Close lost retirement: %v", err)
	}
	if err := h.owner.unit.WaitNativePPPUnitRetired(probe); err != nil {
		t.Fatalf("completed actual unit Close lost retirement: %v", err)
	}
	if h.backend.writer.ClosePending() {
		t.Fatal("genuine successful socket Close retained custody")
	}
	if h.backend.closeCalls.Load() != 1 {
		t.Fatalf("successful physical originals did not release backend once: %d", h.backend.closeCalls.Load())
	}
}
