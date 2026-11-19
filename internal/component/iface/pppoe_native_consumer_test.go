// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- actual native consumer

package iface

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/core/rtproto"
)

type nativeConsumerCall struct {
	operation   string
	local, peer netip.Prefix
	gateway     netip.Addr
	metric      int
	proto       rtproto.Proto
}
type nativeConsumerWriter struct {
	logicalNativeWriter
	calls      []nativeConsumerCall
	routeError error
}

func (w *nativeConsumerWriter) SetMTU(ctx context.Context, _ int) error {
	if _, ok := ctx.Deadline(); !ok {
		panic("native consumer write lacks bounded context")
	}
	w.calls = append(w.calls, nativeConsumerCall{operation: "mtu"})
	return nil
}
func (w *nativeConsumerWriter) SetAdminUp(context.Context) error {
	w.calls = append(w.calls, nativeConsumerCall{operation: "up"})
	return nil
}
func (w *nativeConsumerWriter) AddAddressP2P(_ context.Context, local, peer netip.Prefix) error {
	w.calls = append(w.calls, nativeConsumerCall{operation: "add-address", local: local, peer: peer})
	return nil
}
func (w *nativeConsumerWriter) RemoveAddressP2P(_ context.Context, local, peer netip.Prefix) error {
	w.calls = append(w.calls, nativeConsumerCall{operation: "remove-address", local: local, peer: peer})
	return nil
}
func (w *nativeConsumerWriter) AddRoute(_ context.Context, local netip.Prefix, gateway netip.Addr, metric int, proto rtproto.Proto) error {
	w.calls = append(w.calls, nativeConsumerCall{operation: "add-route", local: local, gateway: gateway, metric: metric, proto: proto})
	return w.routeError
}
func (w *nativeConsumerWriter) RemoveRoute(_ context.Context, local netip.Prefix, gateway netip.Addr, metric int, proto rtproto.Proto) error {
	w.calls = append(w.calls, nativeConsumerCall{operation: "remove-route", local: local, gateway: gateway, metric: metric, proto: proto})
	return nil
}

type nativeConsumerDialer struct {
	session PPPoESession
	calls   int
}

func (d *nativeConsumerDialer) Dial(PPPoEClientConfig, <-chan struct{}, *slog.Logger) (PPPoESession, error) { //nolint:gocritic // hugeParam: public dial boundary.
	d.calls++
	return d.session, nil
}

// TestPPPoENativeConsumerIndependentFamilyWrites drives the actual consumer
// with exact recorded prefixes and route intent, then withdraws IPv4 while
// installing IPv6 and checks the same complete result is acknowledged once.
func TestPPPoENativeConsumerIndependentFamilyWrites(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		writer := &nativeConsumerWriter{}
		changes := make(chan PPPoEResult, 1)
		applied := make(chan uint64, 1)
		local, peer := netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("192.0.2.1")
		dialer := &nativeConsumerDialer{session: PPPoESession{
			NativeWriter: writer, LocalIP: local, PeerIP: peer, NegMTU: 1492,
			Changes: changes, Done: make(chan struct{}), Applied: func(seq uint64) { applied <- seq }, Cleanup: func() error { return nil },
		}}
		client := NewPPPoEClient(PPPoEClientConfig{Name: "replacement-name", RoutePriority: 73, Timing: testPPPoETiming}, dialer, slog.New(slog.NewTextHandler(io.Discard, nil)))
		client.Start()
		t.Cleanup(func() {
			if err := client.Stop(); err != nil {
				t.Error(err)
			}
		})
		synctest.Wait()
		if len(writer.calls) != 4 {
			t.Fatalf("initial native calls: %+v", writer.calls)
		}
		if writer.calls[2].local != netip.PrefixFrom(local, 32) || writer.calls[2].peer != netip.PrefixFrom(peer, 32) {
			t.Fatalf("full original IPv4 pair: %+v", writer.calls[2])
		}
		if writer.calls[3].metric != 73 || writer.calls[3].proto != rtproto.Iface {
			t.Fatalf("route intent: %+v", writer.calls[3])
		}
		writer.calls = nil
		linkLocal := netip.MustParseAddr("fe80::77")
		changes <- PPPoEResult{Seq: 9, NegMTU: 1492, LinkLocal: linkLocal}
		synctest.Wait()
		if len(writer.calls) != 3 || writer.calls[0].operation != "remove-route" || writer.calls[1].operation != "remove-address" || writer.calls[2].operation != "add-address" {
			t.Fatalf("IPv4 withdrawal / IPv6 install: %+v", writer.calls)
		}
		prefix := netip.PrefixFrom(linkLocal, 64)
		if writer.calls[2].local != prefix || writer.calls[2].peer != prefix {
			t.Fatalf("ordinary IPv6 exact /64 pair: %+v", writer.calls[2])
		}
		if len(applied) != 1 || <-applied != 9 {
			t.Fatal("complete result not acknowledged exactly once")
		}
	})
}

// TestPPPoENativePendingCleanupBlocksAdmission injects an uncertain route and
// retained cleanup result. No later Dial can run until that same cleanup owner
// reports terminal history, after which its callback is no longer repeated.
func TestPPPoENativePendingCleanupBlocksAdmission(t *testing.T) {
	cause := errors.New("lost original native acknowledgement")
	writer := &nativeConsumerWriter{routeError: errors.Join(ErrNativePPPPending, cause)}
	pending := true
	cleanups := 0
	dialer := &nativeConsumerDialer{session: PPPoESession{NativeWriter: writer, NegMTU: 1492,
		LocalIP: netip.MustParseAddr("192.0.2.2"), PeerIP: netip.MustParseAddr("192.0.2.1"),
		Cleanup: func() error {
			cleanups++
			if pending {
				return errors.Join(ErrNativePPPPending, cause)
			}
			return cause
		},
	}}
	client := NewPPPoEClient(PPPoEClientConfig{Name: "wan", Timing: testPPPoETiming}, dialer, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := client.runSession(); !errors.Is(err, ErrNativePPPPending) || !errors.Is(err, cause) {
		t.Fatalf("original causes: %v", err)
	}
	if len(writer.calls) != 4 {
		t.Fatalf("write followed pending outcome: %+v", writer.calls)
	}
	if err := client.runSession(); !errors.Is(err, ErrNativePPPPending) {
		t.Fatalf("pending cleanup admitted a dial: %v", err)
	}
	if dialer.calls != 1 || cleanups != 2 {
		t.Fatalf("dial=%d cleanup=%d", dialer.calls, cleanups)
	}
	pending = false
	if err := client.finishCleanup(); !errors.Is(err, cause) || errors.Is(err, ErrNativePPPPending) {
		t.Fatalf("terminal historical outcome: %v", err)
	}
	if err := client.finishCleanup(); err != nil {
		t.Fatalf("terminal callback repeated: %v", err)
	}
	if cleanups != 3 {
		t.Fatalf("cleanup calls=%d", cleanups)
	}
}

// TestPPPoENativeStopBeforeStartCannotDial repeats Start and Stop after an
// initial terminal Stop and observes the actual dialer's call count.
func TestPPPoENativeStopBeforeStartCannotDial(t *testing.T) {
	dialer := &nativeConsumerDialer{}
	client := NewPPPoEClient(PPPoEClientConfig{Name: "wan"}, dialer, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := client.Stop(); err != nil {
		t.Fatal(err)
	}
	client.Start()
	if err := client.Stop(); err != nil {
		t.Fatal(err)
	}
	if dialer.calls != 0 {
		t.Fatalf("stopped publication resurrected %d dials", dialer.calls)
	}
}

// TestPPPoENativeMissingWriterRefusesMutation returns a cleanup owner without
// mutation authority and requires explicit refusal plus exactly one cleanup.
func TestPPPoENativeMissingWriterRefusesMutation(t *testing.T) {
	cleanups := 0
	dialer := &nativeConsumerDialer{session: PPPoESession{Cleanup: func() error { cleanups++; return nil }}}
	client := NewPPPoEClient(PPPoEClientConfig{Name: "wan"}, dialer, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := client.runSession(); !errors.Is(err, ErrNativePPPUnavailable) {
		t.Fatalf("missing original writer: %v", err)
	}
	if cleanups != 1 {
		t.Fatalf("refusal forgot owner cleanup: %d", cleanups)
	}
}

// TestPPPoENativeRegistryRetainsPendingOwner reconciles around the same pending
// original, observes the resource-free desired client without native admission,
// then completes cleanup and proves useful admission through that same registry.
func TestPPPoENativeRegistryRetainsPendingOwner(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var activeMu sync.Mutex
		pending := true
		old := NewPPPoEClient(PPPoEClientConfig{Name: "old"}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
		old.pendingCleanup = func() error {
			if !activeMu.TryLock() {
				t.Fatal("registry lock held across original cleanup")
			}
			activeMu.Unlock()
			if pending {
				return ErrNativePPPPending
			}
			return nil
		}
		old.cleanupErr = ErrNativePPPPending
		active := map[string]*PPPoEClient{"old": old}
		old.registry = &pppoeClientSet{mu: &activeMu, active: active}
		dialer := &nativeConsumerDialer{session: PPPoESession{NativeWriter: &nativeConsumerWriter{}, NegMTU: 1492,
			Done: make(chan struct{}), Cleanup: func() error { return nil }}}
		saved := pppoeDialerVar
		pppoeDialerVar = dialer
		defer func() { pppoeDialerVar = saved }()
		cfg := &ifaceConfig{PPPoEClient: []pppoeClientEntry{{Name: "new", Timing: testPPPoETiming}}}
		reconcilePPPoEClients(cfg, active, &activeMu, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
		synctest.Wait()
		if active["old"] != old {
			t.Fatal("pending original was forgotten")
		}
		if active["new"] == nil {
			t.Fatal("desired resource-free client was dropped")
		}
		if dialer.calls != 0 {
			t.Fatal("native replacement admitted during Pending")
		}
		pending = false
		reconcilePPPoEClients(cfg, active, &activeMu, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
		// sleep(test-clock): release any first admission-backoff wait of the already-published desired worker.
		time.Sleep(testPPPoETiming.ReconnectDelayMin)
		synctest.Wait()
		if active["old"] != nil || active["new"] == nil || dialer.calls != 1 {
			t.Fatalf("terminal cleanup did not release admission: %+v dials=%d", active, dialer.calls)
		}
		if err := active["new"].Stop(); err != nil {
			t.Fatal(err)
		}
	})
}
