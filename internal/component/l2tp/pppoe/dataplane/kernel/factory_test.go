// Design: docs/architecture/l2tp/bng-5-pppoe.md -- kernel factory resource ownership
// Related: factory.go -- ordered construction and exact refusal

package kernel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// TestFactoryOrdersOwners proves the native descriptor is attached before the
// PPP owner is constructed, and each successful owner has a separate lifetime.
func TestFactoryOrdersOwners(t *testing.T) {
	f := newFactoryFixture()
	opened, err := openWithOps(context.Background(), factoryRequest(), f.ops)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.calls, []string{"backend", "socket", "setup", "session"}) {
		t.Fatalf("construction order: %v", f.calls)
	}
	if f.cfg.ChanFD != 42 || f.cfg.UnitFD != 43 || f.cfg.UnitNum != 7 {
		t.Fatalf("descriptor transfer: %+v", f.cfg)
	}
	if f.cfg.InterfaceName != "" || !f.cfg.CaptureProxyLCP || f.cfg.Backend != f.backend {
		t.Fatalf("kernel options: %+v", f.cfg)
	}
	if opened.Dataplane() != f.handle {
		t.Fatal("factory replaced the kernel owner")
	}
	if err := opened.CloseTransport(); err != nil {
		t.Fatal(err)
	}
	if err := opened.CloseTransport(); err != nil {
		t.Fatal(err)
	}
	if f.transportCloses != 1 || f.handle.closes != 0 {
		t.Fatalf("transport close leaked into PPP: socket=%d PPP=%d", f.transportCloses, f.handle.closes)
	}
	if err := opened.Dataplane().Close(); err != nil {
		t.Fatal(err)
	}
	if f.handle.closes != 1 {
		t.Fatalf("PPP owner closes=%d", f.handle.closes)
	}
}

// TestFactoryFailureCleanup pins ownership at every construction boundary. The
// injected kernel constructor consumes channel/unit even when it refuses them.
func TestFactoryFailureCleanup(t *testing.T) {
	want := errors.New("injected failure")
	for _, stage := range []string{"backend", "socket", "setup", "session", "frames"} {
		t.Run(stage, func(t *testing.T) {
			f := newFactoryFixture()
			switch stage {
			case "backend":
				f.ops.backend = func() (sessionBackend, error) { return nil, want }
			case "socket":
				f.ops.openTransport = func(context.Context, dataplane.Request) (*pppoxTransport, error) {
					return nil, want
				}
			case "setup":
				f.ops.setup = func(int, string) (int, int, int, error) { return -1, -1, -1, want }
			case "session":
				f.ops.newSession = func(pppkernel.SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error) {
					return nil, nil, want
				}
			case "frames":
				f.ops.newSession = func(pppkernel.SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error) {
					return nil, f.handle, nil
				}
			}
			opened, err := openWithOps(context.Background(), factoryRequest(), f.ops)
			if err == nil || opened != nil {
				t.Fatalf("refusal returned session=%v error=%v", opened, err)
			}
			if stage != "frames" && !errors.Is(err, want) {
				t.Fatalf("lost construction failure: %v", err)
			}
			closes := 1
			if stage == "backend" || stage == "socket" {
				closes = 0
			}
			if f.transportCloses != closes {
				t.Fatalf("socket closes=%d want=%d", f.transportCloses, closes)
			}
			if stage == "frames" && f.handle.closes != 1 {
				t.Fatalf("invalid result did not consume PPP owner: %d", f.handle.closes)
			}
		})
	}
}

// TestFactoryPrivatePanicCleanup injects panics in the actual private setup and
// constructor operations. The unpublished PPPoX owner closes once, while the
// constructor alone consumes its channel/unit and the original panic survives
// even when transport cleanup fails.
func TestFactoryPrivatePanicCleanup(t *testing.T) {
	for _, stage := range []string{"setup", "session"} {
		for _, cleanupFails := range []bool{false, true} {
			t.Run(stage+"/cleanup-fails="+strconv.FormatBool(cleanupFails), func(t *testing.T) {
				f := newFactoryFixture()
				want := errors.New("private construction panic")
				cleanupErr := errors.New("transport cleanup failure")
				channelCloses, unitCloses := 0, 0
				if cleanupFails {
					f.ops.openTransport = func(context.Context, dataplane.Request) (*pppoxTransport, error) {
						return &pppoxTransport{fd: 41, closeFD: func(int) error {
							f.transportCloses++
							return cleanupErr
						}}, nil
					}
				}
				switch stage {
				case "setup":
					f.ops.setup = func(int, string) (int, int, int, error) { panic(want) }
				case "session":
					f.ops.newSession = func(cfg pppkernel.SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error) {
						f.cfg = cfg
						// Model the constructor's consuming contract on panic as well
						// as error: the factory must never release these again.
						defer func() { channelCloses++; unitCloses++ }()
						panic(want)
					}
				}
				var opened dataplane.OpenedSession
				var recovered any
				returned := false
				func() {
					defer func() { recovered = recover() }()
					var err error
					opened, err = openWithOps(context.Background(), factoryRequest(), f.ops)
					returned = true
					t.Errorf("private panic returned error=%v", err)
				}()
				if cleanupFails {
					panicErr, ok := recovered.(error)
					if !ok || !errors.Is(panicErr, want) || !errors.Is(panicErr, cleanupErr) {
						t.Fatalf("construction or cleanup failure lost: %v", recovered)
					}
				} else if recovered != want {
					t.Fatalf("construction panic changed: %v", recovered)
				}
				if returned || opened != nil {
					t.Fatalf("panicked construction published session=%v returned=%v", opened, returned)
				}
				if f.transportCloses != 1 || f.handle.closes != 0 {
					t.Fatalf("panic closes: transport=%d PPP=%d", f.transportCloses, f.handle.closes)
				}
				wantDescriptors := 0
				if stage == "session" {
					wantDescriptors = 1
					if f.cfg.ChanFD != 42 || f.cfg.UnitFD != 43 {
						t.Fatalf("constructor descriptor transfer: %+v", f.cfg)
					}
				}
				if channelCloses != wantDescriptors || unitCloses != wantDescriptors {
					t.Fatalf("constructor closes: channel=%d unit=%d want=%d", channelCloses, unitCloses, wantDescriptors)
				}
			})
		}
	}
}

// TestFactoryCancellation consumes completed allocations before reporting the
// canceled request; no worker is left to finish an abandoned construction.
func TestFactoryCancellation(t *testing.T) {
	for _, stage := range []string{"before", "socket", "setup", "session"} {
		t.Run(stage, func(t *testing.T) {
			f := newFactoryFixture()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch stage {
			case "before":
				cancel()
			case "socket":
				original := f.ops.openTransport
				f.ops.openTransport = func(ctx context.Context, req dataplane.Request) (*pppoxTransport, error) {
					owner, err := original(ctx, req)
					cancel()
					return owner, err
				}
			case "setup":
				original := f.ops.setup
				f.ops.setup = func(fd int, name string) (int, int, int, error) {
					channel, unit, number, err := original(fd, name)
					cancel()
					return channel, unit, number, err
				}
			case "session":
				original := f.ops.newSession
				f.ops.newSession = func(cfg pppkernel.SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error) {
					frames, handle, err := original(cfg)
					cancel()
					return frames, handle, err
				}
			}
			opened, err := openWithOps(ctx, factoryRequest(), f.ops)
			if opened != nil || !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled construction: session=%v error=%v", opened, err)
			}
			closes := 1
			if stage == "before" {
				closes = 0
			}
			if f.transportCloses != closes {
				t.Fatalf("socket closes=%d want=%d", f.transportCloses, closes)
			}
			if stage == "setup" || stage == "session" {
				if f.handle.closes != 1 {
					t.Fatalf("allocated PPP owner closes=%d", f.handle.closes)
				}
			}
		})
	}
}

// TestFactoryRefusesUnsupportedRequests proves invalid and namespace-bearing
// requests reach no kernel or interface operation.
func TestFactoryRefusesUnsupportedRequests(t *testing.T) {
	for _, alter := range []func(*dataplane.Request){
		func(r *dataplane.Request) { r.AccessName = "" },
		func(r *dataplane.Request) { r.Device = "" },
		func(r *dataplane.Request) { r.Device = "sixteen-byte-dev" + "x" },
		func(r *dataplane.Request) { r.Device = "eth\x000" },
		func(r *dataplane.Request) { r.Namespace = "subscriber" },
		func(r *dataplane.Request) { r.SessionID = 0 },
		func(r *dataplane.Request) { r.SessionID = 0xffff },
		func(r *dataplane.Request) { r.PeerMAC = [6]byte{} },
		func(r *dataplane.Request) { r.PeerMAC[0] = 1 },
	} {
		f := newFactoryFixture()
		req := factoryRequest()
		alter(&req)
		if opened, err := openWithOps(context.Background(), req, f.ops); err == nil || opened != nil {
			t.Fatalf("invalid request admitted: %+v", req)
		}
		if len(f.calls) != 0 {
			t.Fatalf("invalid request allocated resources: %v", f.calls)
		}
	}
}

// TestFactoryCountersUseRawBackend proves the injected reader maps raw backend
// counters, preserves errors and refuses a missing snapshot.
func TestFactoryCountersUseRawBackend(t *testing.T) {
	f := newFactoryFixture()
	opened, err := openWithOps(context.Background(), factoryRequest(), f.ops)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(opened.Dataplane().Close(), opened.CloseTransport()); err != nil {
			t.Error(err)
		}
	})
	counters, err := f.cfg.ReadCounters("ppp7")
	if err != nil || counters != (ppp.SessionCounters{
		RxBytes: 11, TxBytes: 13, RxPackets: 17, TxPackets: 19,
		RxErrors: ppp.MeasuredCounter(0), TxErrors: ppp.MeasuredCounter(0),
		RxDropped: ppp.MeasuredCounter(0), TxDropped: ppp.MeasuredCounter(0),
	}) {
		t.Fatalf("raw counters=%+v error=%v", counters, err)
	}
	if f.backend.statsName != "ppp7" {
		t.Fatalf("counter device=%q", f.backend.statsName)
	}
	want := errors.New("counter read")
	f.backend.statsErr = want
	if _, err := f.cfg.ReadCounters("ppp7"); !errors.Is(err, want) {
		t.Fatalf("counter failure lost: %v", err)
	}
	f.backend.statsErr = nil
	f.backend.stats = nil
	if _, err := f.cfg.ReadCounters("ppp7"); err == nil {
		t.Fatal("missing counter snapshot reported success")
	}
}

type factoryFixture struct {
	ops             factoryOps
	calls           []string
	cfg             pppkernel.SessionConfig
	backend         *factoryBackend
	handle          *factoryHandle
	transportCloses int
}

func newFactoryFixture() *factoryFixture {
	f := &factoryFixture{backend: &factoryBackend{stats: &iface.InterfaceStats{RxBytes: 11, TxBytes: 13, RxPackets: 17, TxPackets: 19}}, handle: &factoryHandle{}}
	f.ops = factoryOps{
		backend: func() (sessionBackend, error) { f.calls = append(f.calls, "backend"); return f.backend, nil },
		openTransport: func(_ context.Context, request dataplane.Request) (*pppoxTransport, error) {
			f.calls = append(f.calls, "socket")
			if request != factoryRequest() {
				return nil, errors.New("request changed before socket creation")
			}
			return &pppoxTransport{fd: 41, closeFD: func(int) error { f.transportCloses++; return nil }}, nil
		},
		setup: func(fd int, name string) (int, int, int, error) {
			f.calls = append(f.calls, "setup")
			if fd != 41 || name != "" {
				return -1, -1, -1, errors.New("wrong PPP setup binding")
			}
			return 42, 43, 7, nil
		},
		newSession: func(cfg pppkernel.SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error) {
			f.calls = append(f.calls, "session")
			f.cfg = cfg
			return &factoryFrames{}, f.handle, nil
		},
	}
	return f
}

func factoryRequest() dataplane.Request {
	return dataplane.Request{AccessName: "access", Device: "eth0", SessionID: 5, PeerMAC: [6]byte{2, 3, 4, 5, 6, 7}, CaptureProxyLCP: true}
}

type factoryBackend struct {
	pppkernel.InterfaceOperations
	stats     *iface.InterfaceStats
	statsErr  error
	statsName string
}

func (b *factoryBackend) GetStats(name string) (*iface.InterfaceStats, error) {
	b.statsName = name
	return b.stats, b.statsErr
}

type factoryHandle struct {
	ppp.SessionHandle
	closes int
}

func (h *factoryHandle) Close() error { h.closes++; return nil }

type factoryFrames struct{ buffer bytes.Buffer }

func (f *factoryFrames) Control() io.ReadWriter { return &f.buffer }
func (*factoryFrames) Opened() io.Reader        { return nil }
func (*factoryFrames) InterruptRead() error     { return nil }
