// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- In-session interface changes.
// Related: pppoe_client.go -- runSession owns all backend writes.

package iface

import (
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"reflect"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// changeBackend records the real session owner's calls. Unimplemented methods
// deliberately retain the nil Backend so an unexpected call fails the test.
type changeBackend struct {
	Backend
	calls []changeCall
	fail  string
}

type changeCall struct {
	op, device, local, peer string
	mtu, metric             int
	proto                   rtproto.Proto
}

func (b *changeBackend) record(call changeCall) error {
	b.calls = append(b.calls, call)
	if b.fail == call.op {
		return errors.New("injected backend failure")
	}
	return nil
}

func (b *changeBackend) SetMTU(device string, mtu int) error {
	return b.record(changeCall{op: "mtu", device: device, mtu: mtu})
}

func (b *changeBackend) SetAdminUp(device string) error {
	return b.record(changeCall{op: "up", device: device})
}

func (b *changeBackend) AddAddressP2P(device, local, peer string) error {
	return b.record(changeCall{op: "address-add", device: device, local: local, peer: peer})
}

func (b *changeBackend) RemoveAddressP2P(device, local, peer string) error {
	return b.record(changeCall{op: "address-remove", device: device, local: local, peer: peer})
}

func (b *changeBackend) RemoveAddress(device, local string) error {
	return b.record(changeCall{op: "ambiguous-address-remove", device: device, local: local})
}

func (b *changeBackend) AddRoute(device, destination, gateway string, metric int, proto rtproto.Proto) error {
	return b.record(changeCall{op: "route-add", device: device, local: destination, peer: gateway, metric: metric, proto: proto})
}

func (b *changeBackend) RemoveRoute(device, destination, gateway string, metric int, proto rtproto.Proto) error {
	return b.record(changeCall{op: "route-remove", device: device, local: destination, peer: gateway, metric: metric, proto: proto})
}

type changeSession struct {
	client   *PPPoEClient
	backend  *changeBackend
	changes  chan PPPoEResult
	ended    chan error
	cleanups int
}

func newChangeSession(t *testing.T, cfg PPPoEClientConfig) *changeSession {
	t.Helper()
	h := &changeSession{
		backend: &changeBackend{},
		changes: make(chan PPPoEResult, 1),
		ended:   make(chan error, 1),
	}
	cfg.Name = "wan"
	cfg.RoutePriority = 73
	dialer := &fakeDialer{sess: PPPoESession{
		SessionID: 42, UnitNum: 7, NegMTU: 1492,
		LocalIP: netip.MustParseAddr("192.0.2.2"),
		PeerIP:  netip.MustParseAddr("192.0.2.1"),
		Changes: h.changes, Done: make(chan struct{}),
		Cleanup: func() { h.cleanups++ },
	}}
	h.client = NewPPPoEClient(cfg, dialer, h.backend, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go h.run()
	synctest.Wait()
	if len(h.backend.calls) < 3 {
		t.Fatalf("initial setup did not run: %v", h.backend.calls)
	}
	h.backend.calls = nil
	t.Cleanup(func() {
		close(h.client.stopCh)
		synctest.Wait()
		if h.cleanups != 1 {
			t.Errorf("Cleanup calls = %d, want 1", h.cleanups)
		}
	})
	return h
}

func (h *changeSession) run() { h.ended <- h.client.runSession() }

func changedResult(local, peer string, mtu uint16) PPPoEResult {
	return PPPoEResult{LocalIP: netip.MustParseAddr(local), PeerIP: netip.MustParseAddr(peer), NegMTU: mtu}
}

// TestPPPoEClientSessionChanges drives runSession with complete records and
// checks exact calls, ordering, device identity and operator-visible addresses.
func TestPPPoEClientSessionChanges(t *testing.T) {
	address := func(op, local, peer string) changeCall {
		return changeCall{op: op, device: "wan", local: local + "/32", peer: peer + "/32"}
	}
	route := func(op, peer string) changeCall {
		return changeCall{op: op, device: "wan", local: "0.0.0.0/0", peer: peer, metric: 73, proto: rtproto.Iface}
	}
	for _, tc := range []struct {
		name   string
		cfg    PPPoEClientConfig
		result PPPoEResult
		want   []changeCall
	}{
		{"local", PPPoEClientConfig{}, changedResult("192.0.2.3", "192.0.2.1", 1492), []changeCall{
			address("address-add", "192.0.2.3", "192.0.2.1"), address("address-remove", "192.0.2.2", "192.0.2.1"),
		}},
		{"peer", PPPoEClientConfig{}, changedResult("192.0.2.2", "192.0.2.9", 1492), []changeCall{
			address("address-add", "192.0.2.2", "192.0.2.9"), route("route-add", "192.0.2.9"),
			route("route-remove", "192.0.2.1"), address("address-remove", "192.0.2.2", "192.0.2.1"),
		}},
		{"both-addresses", PPPoEClientConfig{}, changedResult("192.0.2.3", "192.0.2.9", 1492), []changeCall{
			address("address-add", "192.0.2.3", "192.0.2.9"), route("route-add", "192.0.2.9"),
			route("route-remove", "192.0.2.1"), address("address-remove", "192.0.2.2", "192.0.2.1"),
		}},
		{"mtu", PPPoEClientConfig{}, changedResult("192.0.2.2", "192.0.2.1", 1400), []changeCall{
			{op: "mtu", device: "wan", mtu: 1400},
		}},
		{"mtu-capped", PPPoEClientConfig{MTU: 1300}, changedResult("192.0.2.2", "192.0.2.1", 1400), nil},
		{"unchanged", PPPoEClientConfig{}, changedResult("192.0.2.2", "192.0.2.1", 1492), nil},
		{"no-default-route", PPPoEClientConfig{NoDefaultRoute: true}, changedResult("192.0.2.2", "192.0.2.9", 1492), []changeCall{
			address("address-add", "192.0.2.2", "192.0.2.9"), address("address-remove", "192.0.2.2", "192.0.2.1"),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newChangeSession(t, tc.cfg)
				h.changes <- tc.result
				synctest.Wait()
				if len(h.changes) != 0 {
					t.Error("runSession did not consume the complete result")
				}
				if !reflect.DeepEqual(h.backend.calls, tc.want) {
					t.Errorf("calls = %+v, want %+v", h.backend.calls, tc.want)
				}
				status := h.client.status()
				if status.LocalIP != tc.result.LocalIP {
					t.Errorf("status local = %s, want %s", status.LocalIP, tc.result.LocalIP)
				}
				if status.PeerIP != tc.result.PeerIP {
					t.Errorf("status peer = %s, want %s", status.PeerIP, tc.result.PeerIP)
				}
				if status.PPPInterface != "wan" {
					t.Errorf("session device changed: %+v", status)
				}
				if status.SessionID != 42 {
					t.Errorf("session ID changed: %+v", status)
				}
				if status.State != "up" {
					t.Errorf("session no longer up: %+v", status)
				}
			})
		})
	}
}

// TestPPPoEClientChangeApplyFailureCleansUp injects each fatal backend failure
// after initial setup and requires runSession to return through Cleanup.
func TestPPPoEClientChangeApplyFailureCleansUp(t *testing.T) {
	for _, operation := range []string{"mtu", "address-add", "address-remove"} {
		t.Run(operation, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				h := newChangeSession(t, PPPoEClientConfig{})
				h.backend.fail = operation
				h.changes <- changedResult("192.0.2.3", "192.0.2.1", 1400)
				synctest.Wait()
				if len(h.ended) != 1 {
					t.Fatal("apply failure did not end runSession")
				}
				if err := <-h.ended; err == nil {
					t.Error("apply failure returned nil")
				}
				if h.cleanups != 1 {
					t.Errorf("Cleanup calls = %d, want 1", h.cleanups)
				}
			})
		})
	}
}
