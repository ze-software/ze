// Design: docs/research/l2tpv2-ze-integration.md -- owned kernel PPP session resources

package kernel

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/rtproto"
)

type sessionTestStream struct {
	closed     chan struct{}
	once       sync.Once
	closes     int
	closeErr   error
	closePanic any
}

func (s *sessionTestStream) Read([]byte) (int, error)    { <-s.closed; return 0, io.ErrClosedPipe }
func (s *sessionTestStream) Write(b []byte) (int, error) { return len(b), nil }
func (s *sessionTestStream) InterruptRead() error        { s.once.Do(func() { close(s.closed) }); return nil }
func (s *sessionTestStream) Close() error {
	s.closes++
	err := s.InterruptRead()
	if s.closePanic != nil {
		panic(s.closePanic)
	}
	return errors.Join(err, s.closeErr)
}

type sessionTestBackend struct {
	calls    []string
	name     string
	mtu      int
	local    string
	peer     string
	route    string
	gateway  string
	metric   int
	protocol rtproto.Proto
}

func (b *sessionTestBackend) SetMTU(name string, mtu int) error {
	b.calls = append(b.calls, "mtu")
	b.name = name
	b.mtu = mtu
	return nil
}
func (b *sessionTestBackend) SetAdminUp(name string) error {
	b.calls = append(b.calls, "up")
	b.name = name
	return nil
}
func (b *sessionTestBackend) AddAddressP2P(name, local, peer string) error {
	b.calls = append(b.calls, "address-add")
	b.name = name
	b.local = local
	b.peer = peer
	return nil
}
func (b *sessionTestBackend) RemoveAddress(name, local string) error {
	b.calls = append(b.calls, "address-remove")
	b.name = name
	b.local = local
	return nil
}
func (b *sessionTestBackend) AddRoute(name, destination, gateway string, metric int, protocol rtproto.Proto) error {
	b.calls = append(b.calls, "route-add")
	b.name = name
	b.route = destination
	b.gateway = gateway
	b.metric = metric
	b.protocol = protocol
	return nil
}
func (b *sessionTestBackend) RemoveRoute(name, destination, gateway string, metric int, protocol rtproto.Proto) error {
	b.calls = append(b.calls, "route-remove")
	b.name = name
	b.route = destination
	b.gateway = gateway
	b.metric = metric
	b.protocol = protocol
	return nil
}

func sessionTestConfig() SessionConfig {
	backend := &sessionTestBackend{}
	unit := newSessionTestUnit(32, 7, ppp.SessionIdentity{InterfaceName: "ppp7", NativeIndex: 23})
	unit.info.backend = backend
	return SessionConfig{ChanFD: 31, UnitFD: 32, UnitNum: 7, Channel: &sessionTestChannelSource{fd: 31}, Unit: unit, Backend: backend}
}

func sessionTestOps(streams map[int]*sessionTestStream, released *[]int) sessionOps {
	closeFD := func(fd int) error { *released = append(*released, fd); return nil }
	return sessionOps{
		wrap:        func(fd int, _ string) (io.ReadWriteCloser, error) { return streams[fd], nil },
		wrapUnit:    func(lease sessionUnitLease) (io.ReadWriteCloser, error) { return streams[lease.descriptor()], nil },
		closeFD:     closeFD,
		closeUnit:   sessionTestUnitCloser(closeFD),
		connect:     func(int, int) error { return nil },
		disconnect:  func(int) error { return nil },
		setMRU:      func(int, uint16) error { return nil },
		setIPv4Mode: func(int, NPMode) error { return nil },
		setIPv6Mode: func(int, NPMode) error { return nil },
	}
}

func newSessionTest(t *testing.T) (ppp.FrameChannel, ppp.SessionHandle, *sessionTestBackend, map[int]*sessionTestStream) {
	t.Helper()
	streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
	var released []int
	cfg := sessionTestConfig()
	frames, handle, err := sessionTestNewSession(cfg, sessionTestOps(streams, &released))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	})
	backend, ok := cfg.Backend.(*sessionTestBackend)
	if !ok {
		t.Fatalf("kernel backend has type %T, want *sessionTestBackend", cfg.Backend)
	}
	return frames, handle, backend, streams
}

// The handle is the sole descriptor owner; borrowed lanes cannot close it.
func TestSessionFrameChannelClosedOnceByOwner(t *testing.T) {
	frames, handle, _, streams := newSessionTest(t)
	if _, ok := frames.Control().(io.Closer); ok {
		t.Fatal("control lane exposes Close")
	}
	if _, ok := frames.Opened().(io.Closer); ok {
		t.Fatal("opened lane exposes Close")
	}
	if frames.Control() == nil {
		t.Fatal("missing control lane")
	}
	if frames.Opened() == nil {
		t.Fatal("missing kernel unit lane")
	}
	for range 2 {
		if err := handle.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for fd, stream := range streams {
		if stream.closes != 1 {
			t.Errorf("fd %d closed %d times", fd, stream.closes)
		}
	}
	if _, err := frames.Control().Write([]byte{1}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write after close: %v", err)
	}
}

// A kernel handle preserves the exact activation and route programming calls.
func TestKernelDataplaneParity(t *testing.T) {
	streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
	var released []int
	cfg := sessionTestConfig()
	backend, ok := cfg.Backend.(*sessionTestBackend)
	if !ok {
		t.Fatalf("kernel backend has type %T, want *sessionTestBackend", cfg.Backend)
	}
	ops := sessionTestOps(streams, &released)
	ops.connect = func(fd, unit int) error {
		if fd != 31 || unit != 7 {
			t.Errorf("connect %d/%d", fd, unit)
		}
		backend.calls = append(backend.calls, "connect")
		return nil
	}
	ops.setMRU = func(fd int, mru uint16) error {
		if fd != 32 || mru != 1500 {
			t.Errorf("receive MRU %d/%d", fd, mru)
		}
		backend.calls = append(backend.calls, "receive-mru")
		return nil
	}
	ops.disconnect = func(fd int) error {
		if fd != 31 {
			t.Errorf("disconnect %d", fd)
		}
		backend.calls = append(backend.calls, "disconnect")
		return nil
	}
	_, handle, err := sessionTestNewSession(cfg, ops)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	})
	if got := handle.Identity(); got != (ppp.SessionIdentity{InterfaceName: "ppp7", NativeIndex: 23}) {
		t.Fatalf("identity %+v", got)
	}
	local := netip.MustParsePrefix("192.0.2.1/32")
	peer := netip.MustParsePrefix("192.0.2.2/32")
	route := netip.MustParsePrefix("2001:db8:7::/64")
	via := netip.MustParseAddr("fe80::2")
	steps := []func() error{func() error { return handle.SetMTU(1492) }, handle.Connect, func() error { return handle.SetReceiveMRU(1500) }, handle.SetAdminUp, func() error { return handle.AddAddressP2P(local, peer) }, func() error { return handle.AddRoute(route, via, 3, rtproto.Iface) }, func() error { return handle.RemoveRoute(route, via, 3, rtproto.Iface) }, func() error { return handle.RemoveAddress(local) }, handle.Disconnect}
	for _, step := range steps {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"mtu", "connect", "receive-mru", "up", "address-add", "route-add", "route-remove", "address-remove", "disconnect"}
	if !reflect.DeepEqual(backend.calls, want) {
		t.Fatalf("calls %v, want %v", backend.calls, want)
	}
	if backend.name != "ppp7" {
		t.Errorf("backend interface %q", backend.name)
	}
	if backend.mtu != 1492 {
		t.Errorf("MTU %d", backend.mtu)
	}
	if backend.local != "192.0.2.1/32" {
		t.Errorf("local %q", backend.local)
	}
	if backend.peer != "192.0.2.2/32" {
		t.Errorf("peer %q", backend.peer)
	}
	if backend.route != "2001:db8:7::/64" {
		t.Errorf("route %q", backend.route)
	}
	if backend.gateway != "fe80::2" {
		t.Errorf("gateway %q", backend.gateway)
	}
	if backend.metric != 3 {
		t.Errorf("metric %d", backend.metric)
	}
	if backend.protocol != rtproto.Iface {
		t.Errorf("protocol %d", backend.protocol)
	}
	got, err := handle.Counters()
	if err != nil {
		t.Fatal(err)
	}
	if got != (ppp.SessionCounters{RxBytes: 11, TxBytes: 13, RxPackets: 17, TxPackets: 19}) {
		t.Fatalf("counters %+v", got)
	}
}

// A partial construction owns every supplied descriptor, including failures.
func TestKernelSessionConstructionFailureClosesDescriptors(t *testing.T) {
	wantErr := errors.New("injected setup failure")
	for _, point := range []string{"index", "control", "unit"} {
		t.Run(point, func(t *testing.T) {
			streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
			var released []int
			ops := sessionTestOps(streams, &released)
			cfg := sessionTestConfig()
			if point == "index" {
				sessionTestUnit(cfg).describeErr = wantErr
			}
			if point != "index" {
				ops.wrap = func(fd int, _ string) (io.ReadWriteCloser, error) {
					if point == "control" && fd == 31 {
						return nil, wantErr
					}
					if point == "unit" && fd == 32 {
						return nil, wantErr
					}
					return streams[fd], nil
				}
			}
			_, _, err := sessionTestNewSession(cfg, ops)
			if !errors.Is(err, wantErr) {
				t.Fatalf("error %v, want %v", err, wantErr)
			}
			for _, fd := range []int{31, 32} {
				count := streams[fd].closes
				for _, raw := range released {
					if raw == fd {
						count++
					}
				}
				if count != 1 {
					t.Errorf("fd %d released %d times", fd, count)
				}
			}
		})
	}
}

// A failed counter read remains a failure and cannot manufacture idle bytes.
func TestKernelSessionCountersPreserveFailure(t *testing.T) {
	want := errors.New("counter read failed")
	cfg := sessionTestConfig()
	sessionTestUnit(cfg).readCounters = func() (ppp.SessionCounters, error) { return ppp.SessionCounters{}, want }
	streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
	var released []int
	_, handle, err := sessionTestNewSession(cfg, sessionTestOps(streams, &released))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := handle.Counters(); !errors.Is(err, want) {
		t.Fatalf("counters error %v", err)
	}
}

// Closing a session gates every operation before another descriptor can reuse it.
func TestKernelSessionRefusesOperationsAfterClose(t *testing.T) {
	_, handle, backend, _ := newSessionTest(t)
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	for _, step := range []func() error{func() error { return handle.SetMTU(1492) }, handle.Connect, handle.Disconnect, func() error { return handle.SetReceiveMRU(1500) }, handle.SetAdminUp, func() error { return handle.RemoveAddress(netip.MustParsePrefix("192.0.2.1/32")) }, func() error { _, err := handle.Counters(); return err }, func() error { _, err := handle.OpenIPv6Carrier(context.Background()); return err }} {
		if err := step(); !errors.Is(err, io.ErrClosedPipe) {
			t.Errorf("closed operation: %v", err)
		}
	}
	if len(backend.calls) != 0 {
		t.Fatalf("closed handle reached backend: %v", backend.calls)
	}
}

// Cold admission rejects invalid descriptors, typed nil dependencies and absent
// raw-counter providers before wrapping either descriptor.
func TestKernelSessionRejectsInvalidConstruction(t *testing.T) {
	for _, name := range []string{"channel", "unit", "alias", "number", "backend", "typed-nil", "counters", "index"} {
		t.Run(name, func(t *testing.T) {
			cfg := sessionTestConfig()
			var released []int
			ops := sessionTestOps(nil, &released)
			ops.wrap = func(int, string) (io.ReadWriteCloser, error) {
				t.Fatal("invalid admission reached stream wrapping")
				return nil, errors.New("unexpected stream wrapping")
			}
			switch name {
			case "channel":
				cfg.ChanFD = -1
				cfg.Channel = nil
			case "unit":
				cfg.UnitFD = -1
				cfg.Unit = nil
			case "alias":
				cfg.UnitFD = cfg.ChanFD
				sessionTestUnit(cfg).info.fd = cfg.UnitFD
			case "number":
				cfg.UnitNum = -1
			case "backend":
				cfg.Backend = nil
			case "typed-nil":
				cfg.Backend = (*sessionTestBackend)(nil)
			case "counters":
				sessionTestUnit(cfg).readCounters = nil
			case "index":
				sessionTestUnit(cfg).info.identity.NativeIndex = 0
			}
			if _, _, err := sessionTestNewSession(cfg, ops); err == nil {
				t.Fatal("invalid construction accepted")
			}
			want := []int{}
			if cfg.ChanFD >= 0 {
				want = append(want, cfg.ChanFD)
			}
			if cfg.UnitFD >= 0 {
				if cfg.UnitFD != cfg.ChanFD {
					want = append(want, cfg.UnitFD)
				}
			}
			if !reflect.DeepEqual(released, want) {
				t.Fatalf("released %v, want %v", released, want)
			}
		})
	}
}

// A channel wrapper must supply an owned stream, not nil or a typed nil.
func TestKernelSessionRejectsMissingWrappedStream(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "nil", true: "typed-nil"}[typed], func(t *testing.T) {
			var released []int
			ops := sessionTestOps(nil, &released)
			ops.wrap = func(int, string) (io.ReadWriteCloser, error) {
				if typed {
					return (*sessionTestStream)(nil), nil
				}
				return nil, nil //nolint:nilnil // Deliberately malformed wrapper result exercises admission refusal.
			}
			if _, _, err := sessionTestNewSession(sessionTestConfig(), ops); err == nil {
				t.Fatal("missing wrapped stream accepted")
			}
			if !reflect.DeepEqual(released, []int{31, 32}) {
				t.Fatalf("released %v", released)
			}
		})
	}
}

// Relay interruption wakes only the control reader; writes and the distinct
// opened lane remain available until their single owner closes them.
func TestKernelSessionInterruptPreservesWrites(t *testing.T) {
	frames, _, _, streams := newSessionTest(t)
	if err := frames.InterruptRead(); err != nil {
		t.Fatal(err)
	}
	if _, err := frames.Control().Read(make([]byte, 2)); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("interrupted read: %v", err)
	}
	if n, err := frames.Control().Write([]byte{1, 2}); err != nil || n != 2 {
		t.Fatalf("write %d/%v", n, err)
	}
	select {
	case <-streams[32].closed:
		t.Fatal("control interruption closed unit")
	default:
	}
	if streams[31].closes != 0 {
		t.Fatal("control interruption released descriptor")
	}
}

// A failed close remains visible, both resources are attempted, and a repeated
// close cannot issue another operation against a reused descriptor.
func TestKernelSessionClosePreservesFailure(t *testing.T) {
	streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
	var released []int
	_, handle, err := sessionTestNewSession(sessionTestConfig(), sessionTestOps(streams, &released))
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("close failed")
	streams[31].closeErr = want
	t.Cleanup(func() {
		if err := handle.Close(); !errors.Is(err, want) {
			t.Errorf("close: %v", err)
		}
	})
	for range 2 {
		if err := handle.Close(); !errors.Is(err, want) {
			t.Fatalf("close: %v", err)
		}
	}
	for fd, stream := range streams {
		if stream.closes != 1 {
			t.Errorf("fd %d closes %d", fd, stream.closes)
		}
	}
}

// Descriptor zero is an owned Unix descriptor, not the old StartSession's
// absent-field sentinel. Fake operations ensure this test never closes stdin.
func TestKernelSessionOwnsDescriptorZero(t *testing.T) {
	for _, lane := range []string{"channel", "unit"} {
		t.Run(lane, func(t *testing.T) {
			cfg := sessionTestConfig()
			if lane == "channel" {
				cfg.ChanFD = 0
			} else {
				cfg.UnitFD = 0
			}
			sessionTestChannel(cfg).fd = cfg.ChanFD
			sessionTestUnit(cfg).info.fd = cfg.UnitFD
			streams := map[int]*sessionTestStream{
				cfg.ChanFD: {closed: make(chan struct{})},
				cfg.UnitFD: {closed: make(chan struct{})},
			}
			var released []int
			ops := sessionTestOps(streams, &released)
			ops.connect = func(fd, unit int) error {
				if fd != cfg.ChanFD {
					t.Errorf("connect fd %d, want %d", fd, cfg.ChanFD)
				}
				if unit != cfg.UnitNum {
					t.Errorf("connect unit %d, want %d", unit, cfg.UnitNum)
				}
				return nil
			}
			ops.setMRU = func(fd int, mru uint16) error {
				if fd != cfg.UnitFD {
					t.Errorf("receive MRU fd %d, want %d", fd, cfg.UnitFD)
				}
				if mru != 1500 {
					t.Errorf("receive MRU %d, want 1500", mru)
				}
				return nil
			}
			frames, handle, err := sessionTestNewSession(cfg, ops)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := handle.Close(); err != nil {
					t.Error(err)
				}
			})
			if frames.Control() == nil {
				t.Fatal("descriptor zero lost the control lane")
			}
			if frames.Opened() == nil {
				t.Fatal("descriptor zero lost the opened lane")
			}
			if err := handle.Connect(); err != nil {
				t.Fatal(err)
			}
			if err := handle.SetReceiveMRU(1500); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := handle.Close(); err != nil {
					t.Fatal(err)
				}
			}
			for fd, stream := range streams {
				if stream.closes != 1 {
					t.Errorf("fd %d closes %d, want 1", fd, stream.closes)
				}
			}
			if len(released) != 0 {
				t.Fatalf("wrapped descriptors were closed raw: %v", released)
			}
		})
	}
}

// Rejected ownership still releases descriptor zero, deduplicating an aliased
// pair. Fake close operations record ownership without touching process fds.
func TestKernelSessionRefusalReleasesDescriptorZero(t *testing.T) {
	for _, lane := range []string{"channel", "unit", "alias"} {
		t.Run(lane, func(t *testing.T) {
			cfg := sessionTestConfig()
			cfg.Backend = nil
			want := []int{31, 0}
			if lane == "channel" {
				cfg.ChanFD = 0
				want = []int{0, 32}
			}
			if lane == "unit" {
				cfg.UnitFD = 0
			}
			if lane == "alias" {
				cfg.ChanFD = 0
				cfg.UnitFD = 0
				want = []int{0}
			}
			sessionTestChannel(cfg).fd = cfg.ChanFD
			sessionTestUnit(cfg).info.fd = cfg.UnitFD
			var released []int
			ops := sessionTestOps(nil, &released)
			ops.wrap = func(int, string) (io.ReadWriteCloser, error) {
				t.Fatal("invalid admission wrapped a descriptor")
				return nil, errors.New("unexpected descriptor wrapping")
			}
			if _, _, err := sessionTestNewSession(cfg, ops); err == nil {
				t.Fatal("invalid construction accepted")
			}
			if !reflect.DeepEqual(released, want) {
				t.Fatalf("released %v, want %v", released, want)
			}
		})
	}
}

// A panic before publication cannot bypass rollback just because no error was
// returned. Every resource is attempted once and the original panic survives.
func TestKernelSessionConstructionPanicReleasesDescriptors(t *testing.T) {
	for _, point := range []string{"index", "control", "unit"} {
		t.Run(point, func(t *testing.T) {
			want := errors.New("injected constructor panic")
			streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
			var released []int
			ops := sessionTestOps(streams, &released)
			cfg := sessionTestConfig()
			if point == "index" {
				sessionTestUnit(cfg).describePanic = want
			}
			if point != "index" {
				ops.wrap = func(fd int, _ string) (io.ReadWriteCloser, error) {
					if point == "control" {
						if fd == 31 {
							panic(want)
						}
					}
					if point == "unit" {
						if fd == 32 {
							panic(want)
						}
					}
					return streams[fd], nil
				}
			}
			failure := sessionConstructionPanic(t, cfg, ops)
			if failure != want { //nolint:errorlint // Exact panic identity is the rollback contract; wrapping would violate it.
				t.Fatalf("panic %v, want original %v", failure, want)
			}
			for _, fd := range []int{31, 32} {
				count := streams[fd].closes
				for _, raw := range released {
					if raw == fd {
						count++
					}
				}
				if count != 1 {
					t.Errorf("fd %d released %d times, want 1", fd, count)
				}
			}
		})
	}
}

// A failed or panicking rollback cannot hide the original constructor panic or
// stop the second descriptor's release.
func TestKernelSessionConstructionPanicRetainsCleanupFailures(t *testing.T) {
	for _, kind := range []string{"error", "panic"} {
		t.Run(kind, func(t *testing.T) {
			original := errors.New("constructor failed")
			cleanup := errors.New("channel cleanup failed")
			streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
			if kind == "error" {
				streams[31].closeErr = cleanup
			} else {
				streams[31].closePanic = cleanup
			}
			var released []int
			ops := sessionTestOps(streams, &released)
			ops.wrap = func(fd int, _ string) (io.ReadWriteCloser, error) {
				if fd == 32 {
					panic(original)
				}
				return streams[fd], nil
			}
			failure := sessionConstructionPanic(t, sessionTestConfig(), ops)
			err, ok := failure.(error)
			if !ok {
				t.Fatalf("panic %v does not retain error causes", failure)
			}
			if !errors.Is(err, original) {
				t.Errorf("original panic lost: %v", err)
			}
			if !errors.Is(err, cleanup) {
				t.Errorf("cleanup failure lost: %v", err)
			}
			if streams[31].closes != 1 {
				t.Errorf("channel closed %d times", streams[31].closes)
			}
			if !reflect.DeepEqual(released, []int{32}) {
				t.Fatalf("unit release %v, want [32]", released)
			}
		})
	}
}

// A non-error panic remains the same payload when rollback succeeds; if
// cleanup also fails, both the panic context and cleanup cause remain visible.
func TestKernelSessionConstructionStringPanicPreserved(t *testing.T) {
	for _, cleanupFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "clean", true: "cleanup-error"}[cleanupFails], func(t *testing.T) {
			const original = "constructor string panic"
			cleanup := errors.New("descriptor cleanup failed")
			var released []int
			ops := sessionTestOps(nil, &released)
			cfg := sessionTestConfig()
			sessionTestUnit(cfg).describePanic = original
			ops.closeFD = func(fd int) error {
				released = append(released, fd)
				if cleanupFails {
					if fd == 31 {
						return cleanup
					}
				}
				return nil
			}
			failure := sessionConstructionPanic(t, cfg, ops)
			if !reflect.DeepEqual(released, []int{31, 32}) {
				t.Fatalf("release order %v", released)
			}
			if cleanupFails {
				err, ok := failure.(error)
				if !ok {
					t.Fatalf("panic %v does not retain cleanup error", failure)
				}
				if !strings.Contains(err.Error(), original) {
					t.Errorf("panic context lost: %v", err)
				}
				if !errors.Is(err, cleanup) {
					t.Errorf("cleanup error lost: %v", err)
				}
				return
			}
			if failure != original {
				t.Errorf("panic %v, want original payload %q", failure, original)
			}
		})
	}
}

// Raw-descriptor rollback has no wrapper to own cleanup. A failed raw close
// must still allow the other descriptor's release before the panic propagates.
func TestKernelSessionConstructionPanicSurvivesRawClosePanic(t *testing.T) {
	original := errors.New("constructor failed")
	cleanup := errors.New("raw close panicked")
	var released []int
	ops := sessionTestOps(nil, &released)
	cfg := sessionTestConfig()
	sessionTestUnit(cfg).describePanic = original
	ops.closeFD = func(fd int) error {
		released = append(released, fd)
		if fd == 31 {
			panic(cleanup)
		}
		return nil
	}
	failure := sessionConstructionPanic(t, cfg, ops)
	err, ok := failure.(error)
	if !ok {
		t.Fatalf("panic %v does not retain error causes", failure)
	}
	if !errors.Is(err, original) {
		t.Errorf("original panic lost: %v", err)
	}
	if !errors.Is(err, cleanup) {
		t.Errorf("cleanup panic lost: %v", err)
	}
	if !reflect.DeepEqual(released, []int{31, 32}) {
		t.Fatalf("release order %v", released)
	}
}

// A normal constructor error must retain failed cleanup without publishing a
// partly owned frame channel or handle.
func TestKernelSessionConstructionFailureRetainsCleanupFailures(t *testing.T) {
	for _, kind := range []string{"error", "panic"} {
		t.Run(kind, func(t *testing.T) {
			original := errors.New("unit wrapping failed")
			cleanup := errors.New("channel cleanup failed")
			streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
			if kind == "error" {
				streams[31].closeErr = cleanup
			} else {
				streams[31].closePanic = cleanup
			}
			var released []int
			ops := sessionTestOps(streams, &released)
			ops.wrap = func(fd int, _ string) (io.ReadWriteCloser, error) {
				if fd == 32 {
					return nil, original
				}
				return streams[fd], nil
			}
			frames, handle, err := sessionTestNewSession(sessionTestConfig(), ops)
			if !errors.Is(err, original) {
				t.Errorf("original error lost: %v", err)
			}
			if !errors.Is(err, cleanup) {
				t.Errorf("cleanup failure lost: %v", err)
			}
			if frames != nil {
				t.Error("failed construction published frames")
			}
			if handle != nil {
				t.Error("failed construction published a handle")
			}
			if streams[31].closes != 1 {
				t.Errorf("channel closes %d", streams[31].closes)
			}
			if !reflect.DeepEqual(released, []int{32}) {
				t.Fatalf("unit release %v, want [32]", released)
			}
		})
	}
}

// A dependency may return a stream together with an error. Both returned
// resources still belong to rollback, even if the first close panics.
func TestKernelSessionConstructionFailureReleasesReturnedStreams(t *testing.T) {
	original := errors.New("unit wrapping failed")
	channelCleanup := errors.New("channel close panicked")
	unitCleanup := errors.New("unit close failed")
	streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
	streams[31].closePanic = channelCleanup
	streams[32].closeErr = unitCleanup
	var released []int
	ops := sessionTestOps(streams, &released)
	ops.wrap = func(fd int, _ string) (io.ReadWriteCloser, error) {
		if fd == 32 {
			return streams[fd], original
		}
		return streams[fd], nil
	}
	frames, handle, err := sessionTestNewSession(sessionTestConfig(), ops)
	if !errors.Is(err, original) {
		t.Errorf("original error lost: %v", err)
	}
	if !errors.Is(err, channelCleanup) {
		t.Errorf("channel cleanup failure lost: %v", err)
	}
	if !errors.Is(err, unitCleanup) {
		t.Errorf("unit cleanup failure lost: %v", err)
	}
	if frames != nil {
		t.Error("failed construction published frames")
	}
	if handle != nil {
		t.Error("failed construction published a handle")
	}
	for fd, stream := range streams {
		if stream.closes != 1 {
			t.Errorf("fd %d closes %d", fd, stream.closes)
		}
	}
	if len(released) != 0 {
		t.Fatalf("returned streams also closed raw: %v", released)
	}
}

func sessionConstructionPanic(t *testing.T, cfg SessionConfig, ops sessionOps) (failure any) {
	t.Helper()
	defer func() { failure = recover() }()
	_, handle, err := sessionTestNewSession(cfg, ops)
	if handle != nil {
		if closeErr := handle.Close(); closeErr != nil {
			t.Errorf("unexpected handle close: %v", closeErr)
		}
	}
	t.Errorf("constructor returned instead of panicking: %v", err)
	return nil
}
