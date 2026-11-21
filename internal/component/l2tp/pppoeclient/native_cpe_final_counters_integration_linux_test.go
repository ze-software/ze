//go:build integration && linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- final CPE original snapshot
// Related: native_consumer_integration_linux_test.go -- actual consumer and survivor

package pppoeclient

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/pppoeac"
	"github.com/ze-software/ze/internal/test/userns"
)

type finalCounterNativeDialer struct {
	owner    *clientNativeOwner
	sessions chan iface.PPPoESession
	called   bool
}

func (d *finalCounterNativeDialer) Dial(cfg iface.PPPoEClientConfig, stop <-chan struct{}, log *slog.Logger) (iface.PPPoESession, error) { //nolint:gocritic // hugeParam: actual dial boundary owns the snapshot.
	if d.called {
		return iface.PPPoESession{}, errors.New("native final-counter control unexpectedly redialed")
	}
	d.called = true
	session, err := (&Dialer{}).dialPrepared(cfg, stop, log, func() (*clientDialPreparation, error) {
		return &clientDialPreparation{setup: d.owner.setup, close: d.owner.close,
			nativeWriter: func() iface.NativePPPWriter { return d.owner.writer }}, nil
	})
	if err == nil {
		d.sessions <- session
	}
	return session, err
}

func finalCounterNativeOwner(t *testing.T, backend iface.Backend) *clientNativeOwner {
	t.Helper()
	if err := pppkernel.CheckOriginalPPPSupport(); err != nil {
		t.Fatal(err)
	}
	creator, err := pppkernel.NewUnitCreator(backend)
	if err != nil {
		t.Fatal(err)
	}
	return &clientNativeOwner{backend: backend, creator: creator}
}

// TestNativeCPEFinalCounterOutcomeUsesOriginal drives real discovery, PPP setup,
// negotiation, iface writes and a real transmitted IP datagram. Healthy cleanup
// must retain the actual final counter reading before unit retirement. Closing
// that SAME original externally is the opposite control: cleanup must preserve
// ErrClosed and an explicit refused reading, never publish zero measurements.
func TestNativeCPEFinalCounterOutcomeUsesOriginal(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	for _, closeOriginal := range []bool{false, true} {
		name := "live original sampled"
		if closeOriginal {
			name = "closed original refused"
		}
		t.Run(name, func(t *testing.T) {
			backend, err := iface.OpenBackend("netlink")
			if err != nil {
				t.Fatal(err)
			}
			owner := finalCounterNativeOwner(t, backend)
			t.Cleanup(func() {
				if err := owner.close(); err != nil {
					if closeOriginal {
						if !errors.Is(err, os.ErrClosed) {
							t.Error(err)
						}
						return
					}
					t.Error(err)
				}
			})
			peer := pppoeac.OpenVeth(t, "zec0", "zea0", 44)
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			peerDone := make(chan error, 1)
			go scriptedACOneEpoch(ctx, peer, peerDone)
			joined := false
			t.Cleanup(func() {
				cancel()
				if !joined {
					if err := <-peerDone; err != nil {
						t.Error(err)
					}
				}
			})
			dialer := &finalCounterNativeDialer{owner: owner, sessions: make(chan iface.PPPoESession, 1)}
			client := iface.NewPPPoEClient(iface.PPPoEClientConfig{
				Name: "finalwan", SourceInterface: "zec0", MTU: 1400, RoutePriority: 73,
				Timing: testDialTiming, Username: "subscriber", AuthSecret: "secret",
			}, dialer, slog.New(slog.NewTextHandler(io.Discard, nil)))
			client.Start()
			t.Cleanup(func() {
				err := client.Stop()
				if closeOriginal {
					if !errors.Is(err, os.ErrClosed) {
						t.Errorf("closed original cause lost: %v", err)
					}
					return
				}
				if err != nil {
					t.Error(err)
				}
			})
			var session iface.PPPoESession
			select {
			case session = <-dialer.sessions:
			case <-ctx.Done():
				t.Fatal("actual CPE did not negotiate")
			}
			index := int(owner.unit.Identity().NativeIndex)
			waitNativeCPEAddress(t, ctx, index, netip.PrefixFrom(session.LocalIP, 32))
			waitFinalCPEInitialRoute(t, ctx, index, session.PeerIP.String())
			select {
			case err := <-peerDone:
				joined = true
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("actual wire epoch did not complete")
			}
			connection, err := net.DialUDP("udp4", &net.UDPAddr{IP: session.LocalIP.AsSlice()},
				&net.UDPAddr{IP: session.PeerIP.AsSlice(), Port: 39991})
			if err != nil {
				t.Fatal(err)
			}
			payload := []byte("actual CPE final counter control")
			written, writeErr := connection.Write(payload)
			closeErr := connection.Close()
			if err := errors.Join(writeErr, closeErr); err != nil {
				t.Fatal(err)
			}
			if written != len(payload) {
				t.Fatalf("datagram bytes = %d", written)
			}
			before := waitFinalCPETraffic(t, ctx, owner)
			if closeOriginal {
				if err := owner.unit.Close(); err != nil {
					t.Fatal(err)
				}
			}
			err = client.Stop()
			if closeOriginal {
				if !errors.Is(err, os.ErrClosed) {
					t.Fatalf("final original error lost: %v", err)
				}
				refused, ok := owner.finalCounters.(clientNativeCountersRefused)
				if !ok {
					t.Fatalf("closed original published a measurement: %T", owner.finalCounters)
				}
				if !errors.Is(refused.cause, os.ErrClosed) {
					t.Fatalf("counter refusal cause: %v", refused.cause)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				sampled, ok := owner.finalCounters.(clientNativeCountersSampled)
				if !ok {
					t.Fatalf("live original final snapshot unavailable: %T", owner.finalCounters)
				}
				if sampled.counters.TxPackets < before.TxPackets {
					t.Fatal("final sample lost original transmitted packets")
				}
				if sampled.counters.TxBytes < before.TxBytes {
					t.Fatal("final sample lost original transmitted bytes")
				}
			}
			if !owner.closed {
				t.Fatal("cleanup did not finish captured backend ownership")
			}
			if owner.unit.NativeOperationsPending() {
				t.Fatal("terminal cleanup retained native facet")
			}
			if err := owner.unit.WaitNativePPPUnitRetired(ctx); err != nil {
				t.Fatalf("not genuinely retired: %v", err)
			}
			if _, err := owner.unit.Counters(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("old unit still sampled: %v", err)
			}
		})
	}
}

func waitFinalCPETraffic(t *testing.T, ctx context.Context, owner *clientNativeOwner) ppp.SessionCounters {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		counters, err := owner.unit.Counters()
		if err != nil {
			t.Fatal(err)
		}
		if counters.TxPackets > 0 {
			if counters.TxBytes > 0 {
				return counters
			}
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("actual datagram produced no original PPP counters")
		}
	}
}

func waitFinalCPEInitialRoute(t *testing.T, ctx context.Context, index int, peer string) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		link, err := netlink.LinkByIndex(index)
		if err != nil {
			t.Fatal(err)
		}
		routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
		if err != nil {
			t.Fatal(err)
		}
		for i := range routes {
			route := &routes[i]
			if route.Dst == nil {
				if route.Gw.String() == peer {
					if route.Priority == 73 {
						return
					}
				}
			}
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("actual initial default route never completed")
		}
	}
}
