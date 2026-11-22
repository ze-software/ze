//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- actual CPE fault boundaries
// Related: native_cpe_fault_integration_linux_test.go -- useful native controls

package ifacenetlink

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/component/l2tp/pppoeclient"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/test/pppcpefault"
	"github.com/ze-software/ze/internal/test/pppoeac"
)

type cpeNativeFault uint8

const (
	cpeFaultUnspecified cpeNativeFault = iota
	cpePartialBindTerminal
	cpePartialBindSocketClose
	cpeUncertainACK
	cpeUnitClose
	cpeSocketClose
	cpeHealthy
	cpeProbeSocketClose
	cpeChangedACK
)

type cpeNativeFaultBackend struct {
	*netlinkBackend
	t             *testing.T
	fault         cpeNativeFault
	bindCause     error
	unit          *pppkernel.CreatedUnit
	writer        *nativePPPWriter
	facet         *cpeNativeFaultWriter
	bound         chan struct{}
	ackEntered    chan struct{}
	ackRelease    chan struct{}
	ackOnce       sync.Once
	writeResult   chan error
	sends         atomic.Uint32
	mutations     atomic.Uint32
	writerCloses  atomic.Uint32
	backendCloses atomic.Uint32
	backendDone   chan struct{}
	doneOnce      sync.Once
	probeFaultErr error
	socketLabel   int // read-only leak observation; never native mutation or Close authority
}

func (b *cpeNativeFaultBackend) BindNativePPPUnit(owner iface.NativePPPUnitOwner, index int, number int32) (iface.NativePPPWriter, error) {
	unit, ok := owner.(*pppkernel.CreatedUnit)
	if !ok {
		return nil, errors.New("CPE fault did not receive the genuine CreatedUnit")
	}
	b.unit = unit
	if b.fault == cpeProbeSocketClose {
		// Root owns this exact instance-local observer: after real Socket/File
		// mint, before Bind/GetSockname/query51. It supplies no I/O or result.
		b.nativePPPs.afterSocket = func(file *os.File) {
			connection, err := file.SyscallConn()
			if err != nil {
				b.probeFaultErr = err
				return
			}
			var installErr error
			controlErr := connection.Control(func(fd uintptr) {
				b.socketLabel = int(fd)
				installErr = pppcpefault.DenyOriginalClose(int(fd))
			})
			b.probeFaultErr = errors.Join(controlErr, installErr)
			if b.probeFaultErr == nil {
				b.probeFaultErr = pppcpefault.DenyWorkerOwnerQuery()
			}
		}
	}
	bound, err := b.netlinkBackend.BindNativePPPUnit(owner, index, number)
	if b.fault == cpeProbeSocketClose {
		if bound != nil {
			b.facet = &cpeNativeFaultWriter{NativePPPWriter: bound, backend: b}
		}
		close(b.bound)
		// This is refused partial cleanup custody, not a native write grant.
		if b.probeFaultErr != nil {
			return b.facet, errors.Join(err, b.probeFaultErr)
		}
		if b.facet == nil {
			return nil, err
		}
		return b.facet, err
	}
	if err != nil {
		return bound, err
	}
	writer, ok := bound.(*nativePPPWriter)
	if !ok {
		return bound, errors.New("CPE fault did not bind the actual native producer")
	}
	b.unit, b.writer = unit, writer
	b.facet = &cpeNativeFaultWriter{NativePPPWriter: bound, backend: b}
	writer.beforeSend = func() { b.sends.Add(1) }
	var faultErr error
	switch b.fault {
	case cpePartialBindTerminal, cpeUncertainACK, cpeChangedACK, cpeHealthy:
	case cpePartialBindSocketClose, cpeSocketClose:
		connection, err := writer.file.SyscallConn()
		if err != nil {
			return b.facet, err
		}
		var installErr error
		faultErr = connection.Control(func(fd uintptr) { installErr = pppcpefault.DenyOriginalClose(int(fd)) })
		faultErr = errors.Join(faultErr, installErr)
	case cpeUnitClose:
		faultErr = unit.WithDescriptor(pppcpefault.DenyOriginalClose)
	case cpeFaultUnspecified:
		return b.facet, errors.New("CPE native fault is unspecified")
	case cpeProbeSocketClose:
		return b.facet, errors.New("CPE probe fault crossed its separate refused-facet lane")
	default:
		return b.facet, errors.New("CPE native fault is unknown")
	}
	close(b.bound)
	if faultErr != nil {
		return b.facet, faultErr
	}
	switch b.fault {
	case cpePartialBindTerminal, cpePartialBindSocketClose:
		// The original socket/probe really succeeded. This private producer
		// refusal returns that SAME partial facet; it grants no mutation.
		return b.facet, b.bindCause
	case cpeUncertainACK, cpeChangedACK, cpeUnitClose, cpeSocketClose, cpeHealthy:
		return b.facet, nil
	case cpeFaultUnspecified:
		return b.facet, errors.New("CPE native fault is unspecified")
	case cpeProbeSocketClose:
		return b.facet, errors.New("CPE probe fault crossed its separate refused-facet lane")
	default:
		return b.facet, errors.New("CPE native fault is unknown")
	}
}

func (b *cpeNativeFaultBackend) Close() error {
	b.backendCloses.Add(1)
	if b.unit == nil {
		b.t.Error("backend closed before genuine native bind")
		return b.netlinkBackend.Close()
	}
	if b.facet == nil {
		b.t.Error("backend closed without genuine native facet")
		return b.netlinkBackend.Close()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := b.unit.WaitNativePPPUnitRetired(ctx); err != nil {
		b.t.Errorf("backend release preceded actual original retirement: %v", err)
	}
	if b.facet.ClosePending() {
		b.t.Error("backend release preceded actual native socket close")
	}
	err := b.netlinkBackend.Close()
	select {
	case <-b.closed.channel():
		b.doneOnce.Do(func() { close(b.backendDone) })
	default:
	}
	return err
}

// The facet never invents an outcome or pending flag. Every result/status comes
// from the SAME actual writer; only its existing native observer port cancels
// a real child context AFTER actual SEND. All consumer mutation attempts count,
// including ones a real pending writer would subsequently refuse.
type cpeNativeFaultWriter struct {
	iface.NativePPPWriter
	backend *cpeNativeFaultBackend
}

func (w *cpeNativeFaultWriter) SetMTU(ctx context.Context, mtu int) error {
	w.backend.mutations.Add(1)
	if w.backend.fault != cpeUncertainACK {
		return w.NativePPPWriter.SetMTU(ctx, mtu)
	}
	return w.uncertain(ctx, func(ctx context.Context) error { return w.NativePPPWriter.SetMTU(ctx, mtu) })
}
func (w *cpeNativeFaultWriter) uncertain(ctx context.Context, operation func(context.Context) error) error {
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	w.backend.writer.beforeRead = func() {
		w.backend.writer.beforeRead = nil
		if w.backend.writer.pending == nil {
			w.backend.t.Error("ACK boundary had no real submitted transaction")
		}
		cancel()
		w.backend.ackOnce.Do(func() { close(w.backend.ackEntered) })
		<-w.backend.ackRelease
	}
	err := operation(child)
	w.backend.writeResult <- err
	return err
}
func (w *cpeNativeFaultWriter) SetAdminUp(ctx context.Context) error {
	w.backend.mutations.Add(1)
	return w.NativePPPWriter.SetAdminUp(ctx)
}
func (w *cpeNativeFaultWriter) SetAdminDown(ctx context.Context) error {
	w.backend.mutations.Add(1)
	return w.NativePPPWriter.SetAdminDown(ctx)
}
func (w *cpeNativeFaultWriter) SetAlias(ctx context.Context, alias string) error {
	w.backend.mutations.Add(1)
	return w.NativePPPWriter.SetAlias(ctx, alias)
}
func (w *cpeNativeFaultWriter) AddAddressP2P(ctx context.Context, local, peer netip.Prefix) error {
	w.backend.mutations.Add(1)
	if w.backend.fault == cpeChangedACK {
		if local == netip.MustParsePrefix("198.51.100.2/32") {
			return w.uncertain(ctx, func(ctx context.Context) error { return w.NativePPPWriter.AddAddressP2P(ctx, local, peer) })
		}
	}
	return w.NativePPPWriter.AddAddressP2P(ctx, local, peer)
}
func (w *cpeNativeFaultWriter) RemoveAddressP2P(ctx context.Context, local, peer netip.Prefix) error {
	w.backend.mutations.Add(1)
	return w.NativePPPWriter.RemoveAddressP2P(ctx, local, peer)
}
func (w *cpeNativeFaultWriter) AddRoute(ctx context.Context, destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) error {
	w.backend.mutations.Add(1)
	return w.NativePPPWriter.AddRoute(ctx, destination, gateway, metric, protocol)
}
func (w *cpeNativeFaultWriter) RemoveRoute(ctx context.Context, destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) error {
	w.backend.mutations.Add(1)
	return w.NativePPPWriter.RemoveRoute(ctx, destination, gateway, metric, protocol)
}
func (w *cpeNativeFaultWriter) Settle(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		w.backend.t.Errorf("cleanup reused canceled context: %v", err)
	}
	if _, bounded := ctx.Deadline(); !bounded {
		w.backend.t.Error("cleanup settlement has no bound")
	}
	return w.NativePPPWriter.Settle(ctx)
}
func (w *cpeNativeFaultWriter) Close() error {
	w.backend.writerCloses.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := w.backend.unit.WaitNativePPPUnitRetired(ctx); err != nil {
		w.backend.t.Errorf("writer Close preceded actual unit retirement: %v", err)
	}
	return w.NativePPPWriter.Close()
}

type cpeFaultDialResult struct {
	session iface.PPPoESession
	err     error
}
type cpeFaultDialer struct {
	actual         *pppoeclient.Dialer
	calls          atomic.Uint32
	first          chan cpeFaultDialResult
	cleanupEntered chan struct{}
	cleanupRelease chan struct{}
	cleanupOnce    sync.Once
	cleanupCalls   atomic.Uint32
	firstCleanup   chan error
	secondCleanup  chan error
	applied        atomic.Uint32
	appliedSeq     atomic.Uint64
	observeChanges bool
	changes        chan iface.PPPoEResult
}

func (d *cpeFaultDialer) Dial(cfg iface.PPPoEClientConfig, stop <-chan struct{}, log *slog.Logger) (iface.PPPoESession, error) { //nolint:gocritic // hugeParam: exact public CPE dial boundary.
	call := d.calls.Add(1)
	session, err := d.actual.Dial(cfg, stop, log)
	var stopRelay func()
	if err == nil && d.observeChanges {
		originalChanges := session.Changes
		forward := make(chan iface.PPPoEResult)
		relayStop, relayDone := make(chan struct{}), make(chan struct{})
		var relayOnce sync.Once
		stopRelay = func() { relayOnce.Do(func() { close(relayStop) }); <-relayDone }
		session.Changes = forward
		go func() {
			defer close(relayDone)
			for {
				select {
				case change, ok := <-originalChanges:
					if !ok {
						close(forward)
						return
					}
					// Forward the exact producer result and Seq unchanged. No
					// synthetic Changes record or Applied acknowledgment exists.
					select {
					case d.changes <- change:
					case <-relayStop:
						return
					}
					select {
					case forward <- change:
					case <-relayStop:
						return
					}
				case <-relayStop:
					return
				}
			}
		}()
	}
	if session.Cleanup != nil {
		original := session.Cleanup
		session.Cleanup = func() error {
			attempt := d.cleanupCalls.Add(1)
			d.cleanupOnce.Do(func() { close(d.cleanupEntered) })
			<-d.cleanupRelease
			if stopRelay != nil {
				stopRelay()
			}
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
	if session.Applied != nil {
		original := session.Applied
		session.Applied = func(sequence uint64) { d.applied.Add(1); d.appliedSeq.Store(sequence); original(sequence) }
	}
	if call == 1 {
		d.first <- cpeFaultDialResult{session: session, err: err}
	}
	return session, err
}

type cpeNativeFaultFixture struct {
	backend            *cpeNativeFaultBackend
	dialer             *cpeFaultDialer
	client             *iface.PPPoEClient
	ctx                context.Context
	cancel             context.CancelFunc
	admitted           chan struct{}
	factoryCalls       atomic.Uint32
	peerReady          chan *pppoeac.Session
	peerDone           chan error
	cleanupReleaseOnce sync.Once
	ackReleaseOnce     sync.Once
	expectPending      bool
	secondBackend      chan *cpeNativeFaultBackend
	restart            chan struct{}
}

func startCPENativeFault(t *testing.T, fault cpeNativeFault, expectedPending bool) *cpeNativeFaultFixture {
	t.Helper()
	return newCPENativeFault(t, fault, expectedPending, true, false)
}

func newCPEFaultBackend(t *testing.T, fault cpeNativeFault) *cpeNativeFaultBackend {
	return &cpeNativeFaultBackend{netlinkBackend: &netlinkBackend{}, t: t, fault: fault,
		bindCause: errors.New("injected CPE refusal after real native bind"), bound: make(chan struct{}),
		ackEntered: make(chan struct{}), ackRelease: make(chan struct{}), backendDone: make(chan struct{}), writeResult: make(chan error, 1)}
}

func newCPENativeFault(t *testing.T, fault cpeNativeFault, expectedPending, standalone, readmit bool) *cpeNativeFaultFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	h := &cpeNativeFaultFixture{ctx: ctx, cancel: cancel, admitted: make(chan struct{}), peerReady: make(chan *pppoeac.Session, 2), peerDone: make(chan error, 1), expectPending: expectedPending, secondBackend: make(chan *cpeNativeFaultBackend, 1), restart: make(chan struct{})}
	h.backend = newCPEFaultBackend(t, fault)
	actual, err := pppoeclient.NewDialer(func() (iface.Backend, error) {
		call := h.factoryCalls.Add(1)
		if call == 1 {
			return h.backend, nil
		}
		select {
		case <-h.backend.backendDone:
		default:
			t.Error("native readmission preceded original backend completion")
		}
		if call == 2 {
			close(h.admitted)
			if readmit {
				second := newCPEFaultBackend(t, cpeHealthy)
				h.secondBackend <- second
				return second, nil
			}
		}
		return nil, errors.New("native fault control stops after proving next actual preflight admission")
	})
	if err != nil {
		t.Fatal(err)
	}
	h.dialer = &cpeFaultDialer{actual: actual, first: make(chan cpeFaultDialResult, 1),
		cleanupEntered: make(chan struct{}), cleanupRelease: make(chan struct{}), firstCleanup: make(chan error, 1), secondCleanup: make(chan error, 1), observeChanges: fault == cpeChangedACK, changes: make(chan iface.PPPoEResult, 1)}
	peer := pppoeac.OpenVeth(t, "zcf0", "zca0", 91)
	epochs := 1
	if readmit {
		epochs = 2
	}
	go func() { h.peerDone <- cpeFaultPeer(ctx, peer, fault, epochs, h.restart, h.peerReady) }()
	if standalone {
		h.client = iface.NewPPPoEClient(iface.PPPoEClientConfig{Name: "faultwan", SourceInterface: "zcf0",
			Username: "subscriber", AuthSecret: "secret", MTU: 1400, RoutePriority: 73,
			Timing: iface.PPPoEClientTiming{LCPRestartTimer: 3 * time.Second, LCPMaxConfigure: 10, LCPMaxFailure: 5,
				IPv6CPRestartTimer: 3 * time.Second, KeepaliveInterval: 30 * time.Second, KeepaliveFailures: 3,
				ReconnectDelayMin: time.Second, ReconnectDelayMax: time.Second}}, h.dialer, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}
	t.Cleanup(func() {
		h.releaseACK()
		h.releaseCleanup()
		if h.client != nil {
			err := h.client.Stop()
			if h.expectPending {
				if !errors.Is(err, iface.ErrNativePPPPending) {
					t.Errorf("real failed Close lost custody: %v", err)
				}
				if !errors.Is(err, unix.EPERM) {
					t.Errorf("real failed Close lost syscall cause: %v", err)
				}
			} else if err != nil {
				t.Error(err)
			}
		}
		cancel()
		if err := <-h.peerDone; err != nil {
			t.Error(err)
		}
		// A seccomp-denied original cannot be safely retried by numeric FD.
		// This userns child exits after its Pending assertions; the kernel then
		// releases that intentionally retained process-owned file.
	})
	if h.client != nil {
		h.client.Start()
	}
	return h
}

func (h *cpeNativeFaultFixture) releaseACK() {
	h.ackReleaseOnce.Do(func() { close(h.backend.ackRelease) })
}
func (h *cpeNativeFaultFixture) releaseCleanup() {
	h.cleanupReleaseOnce.Do(func() { close(h.dialer.cleanupRelease) })
}

func cpeFaultPeer(ctx context.Context, peer *pppoeac.Peer, fault cpeNativeFault, epochs int, restart <-chan struct{}, ready chan<- *pppoeac.Session) error {
	for range epochs {
		session, err := peer.Accept(ctx)
		if err != nil {
			return err
		}
		switch fault {
		case cpePartialBindTerminal, cpePartialBindSocketClose, cpeProbeSocketClose:
			ready <- session
			<-ctx.Done()
			return nil
		case cpeUncertainACK, cpeChangedACK, cpeUnitClose, cpeSocketClose, cpeHealthy:
		case cpeFaultUnspecified:
			return errors.New("CPE peer fault is unspecified")
		default:
			return errors.New("CPE peer fault is unknown")
		}
		// Actual RFC 1661/1994/1332 exchanges; the shared peer parses every reply.
		options := []byte{3, 5, 0xc2, 0x23, 5, 5, 6, 0x10, 0x20, 0x30, 0x40}
		if err := session.LCP(ctx, 70, options, false); err != nil {
			return err
		}
		if err := session.CHAP(ctx, 80, []byte{1, 3, 5, 7}, "subscriber", "secret"); err != nil {
			return err
		}
		if err := session.IPCP(ctx, 90, netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")); err != nil {
			return err
		}
		ready <- session
		if fault == cpeChangedACK {
			select {
			case <-restart:
			case <-ctx.Done():
				return nil
			}
			changedOptions := []byte{3, 5, 0xc2, 0x23, 5, 5, 6, 0x10, 0x20, 0x30, 0x41}
			if err := session.LCP(ctx, 71, changedOptions, true); err != nil {
				return err
			}
			if err := session.CHAP(ctx, 81, []byte{1, 3, 5, 8}, "subscriber", "secret"); err != nil {
				return err
			}
			if err := session.IPCP(ctx, 91, netip.MustParseAddr("198.51.100.1"), netip.MustParseAddr("198.51.100.2")); err != nil {
				return err
			}
			ready <- session
		}
	}
	<-ctx.Done()
	return nil
}

func cpeFaultWait(t *testing.T, ctx context.Context, signal <-chan struct{}, why string) {
	t.Helper()
	select {
	case <-signal:
	case <-ctx.Done():
		t.Fatalf("%s: %v", why, ctx.Err())
	}
}

func cpeFaultWaitResult(t *testing.T, h *cpeNativeFaultFixture, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-h.ctx.Done():
		t.Fatal("actual CPE cleanup result missing")
		return h.ctx.Err()
	}
}

func cpeFaultWaitInstalled(t *testing.T, h *cpeNativeFaultFixture) *pppoeac.Session {
	t.Helper()
	cpeFaultWait(t, h.ctx, h.backend.bound, "real bind never completed")
	var peer *pppoeac.Session
	select {
	case peer = <-h.peerReady:
	case <-h.ctx.Done():
		t.Fatal("actual PPP exchange never completed")
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		link, err := netlink.LinkByIndex(int(h.backend.unit.Identity().NativeIndex))
		if err != nil {
			t.Fatal(err)
		}
		addresses, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			t.Fatal(err)
		}
		routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
		if err != nil {
			t.Fatal(err)
		}
		addressReady, routeReady := false, false
		for _, address := range addresses {
			if address.IPNet.String() == "192.0.2.2/32" {
				addressReady = true
			}
		}
		for i := range routes {
			route := &routes[i]
			if route.Dst == nil {
				if route.Gw.String() == "192.0.2.1" {
					if route.Priority == 73 {
						routeReady = true
					}
				}
			}
		}
		if addressReady {
			if routeReady {
				return peer
			}
		}
		select {
		case <-ticker.C:
		case <-h.ctx.Done():
			t.Fatal("useful real CPE address/default route missing")
		}
	}
}
