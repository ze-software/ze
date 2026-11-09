// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- Bounded route recovery.
// Related: pppoe_changes_test.go -- Real session consumer and recording backend.

package iface

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// routeAddFailure adds an independent failure to the existing removal recorder.
type routeAddFailure struct{ *changeBackend }

func (b *routeAddFailure) AddRoute(device, destination, gateway string, metric int, proto rtproto.Proto) error {
	if err := b.changeBackend.AddRoute(device, destination, gateway, metric, proto); err != nil {
		return err
	}
	return errors.New("injected independent route-add failure")
}

// TestPPPoEClientSimultaneousRouteFailures requires both attempts and warnings,
// then retries only failed route operations on the unchanged complete result.
func TestPPPoEClientSimultaneousRouteFailures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newChangeSession(t, &PPPoEClientConfig{})
		h.backend.fail = "route-remove"
		h.changes <- changedResult("192.0.2.2", "192.0.2.9", 1492)
		synctest.Wait()
		h.client.backend = &routeAddFailure{h.backend}
		var logs bytes.Buffer
		h.client.logger = slog.New(slog.NewTextHandler(&logs, nil))
		h.backend.calls = nil
		latest := changedResult("192.0.2.2", "192.0.2.10", 1492)
		h.changes <- latest
		synctest.Wait()
		wantRoutes := []changeCall{
			{op: "route-add", device: "wan", local: "0.0.0.0/0", peer: "192.0.2.10", metric: 73, proto: rtproto.Iface},
			{op: "route-remove", device: "wan", local: "0.0.0.0/0", peer: "192.0.2.1", metric: 73, proto: rtproto.Iface},
		}
		if len(h.backend.calls) != 4 {
			t.Fatalf("failed change calls = %+v", h.backend.calls)
		}
		for i, want := range wantRoutes {
			if h.backend.calls[i+1] != want {
				t.Errorf("route call %d = %+v, want %+v", i, h.backend.calls[i+1], want)
			}
		}
		for _, gateway := range []string{"192.0.2.10", "192.0.2.1"} {
			matched := false
			for line := range strings.SplitSeq(logs.String(), "\n") {
				if !strings.Contains(line, "level=WARN") {
					continue
				}
				if !strings.Contains(line, "gateway="+gateway) {
					continue
				}
				if strings.Contains(line, "device=wan") {
					matched = true
				}
			}
			if !matched {
				t.Errorf("no distinct warning for %s: %s", gateway, logs.String())
			}
		}
		if len(h.ended) != 0 {
			t.Fatal("simultaneous route failures ended PPP")
		}
		h.client.backend = h.backend
		h.backend.fail = ""
		h.backend.calls = nil
		h.changes <- latest
		synctest.Wait()
		if len(h.backend.calls) != len(wantRoutes) {
			t.Fatalf("unchanged retry calls = %+v", h.backend.calls)
		}
		for i, want := range wantRoutes {
			if h.backend.calls[i] != want {
				t.Errorf("retry call %d = %+v, want %+v", i, h.backend.calls[i], want)
			}
		}
		h.backend.calls = nil
		h.changes <- latest
		synctest.Wait()
		if len(h.backend.calls) != 0 {
			t.Fatalf("successful retry repeated calls: %+v", h.backend.calls)
		}
	})
}

// TestPPPoEClientRouteReturnsToPendingGateway replaces several gateways while
// cleanup fails, then returns to the retained gateway without deleting it.
func TestPPPoEClientRouteReturnsToPendingGateway(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newChangeSession(t, &PPPoEClientConfig{})
		h.backend.fail = "route-remove"
		for _, gateway := range []string{"192.0.2.9", "192.0.2.10", "192.0.2.11"} {
			h.backend.calls = nil
			h.changes <- changedResult("192.0.2.2", gateway, 1492)
			synctest.Wait()
			want := []changeCall{
				{op: "address-add", device: "wan", local: "192.0.2.2/32", peer: gateway + "/32"},
				{op: "route-add", device: "wan", local: "0.0.0.0/0", peer: gateway, metric: 73, proto: rtproto.Iface},
				{op: "route-remove", device: "wan", local: "0.0.0.0/0", peer: "192.0.2.1", metric: 73, proto: rtproto.Iface},
			}
			if len(h.backend.calls) != 4 {
				t.Fatalf("replacement calls = %+v", h.backend.calls)
			}
			for i, call := range want {
				if h.backend.calls[i] != call {
					t.Errorf("replacement call %d = %+v, want %+v", i, h.backend.calls[i], call)
				}
			}
		}
		h.backend.calls = nil
		original := changedResult("192.0.2.2", "192.0.2.1", 1492)
		h.changes <- original
		synctest.Wait()
		want := []changeCall{
			{op: "address-add", device: "wan", local: "192.0.2.2/32", peer: "192.0.2.1/32"},
			{op: "route-add", device: "wan", local: "0.0.0.0/0", peer: "192.0.2.1", metric: 73, proto: rtproto.Iface},
			{op: "address-remove", device: "wan", local: "192.0.2.2/32", peer: "192.0.2.11/32"},
		}
		if len(h.backend.calls) != len(want) {
			t.Fatalf("return calls = %+v", h.backend.calls)
		}
		for i, call := range want {
			if h.backend.calls[i] != call {
				t.Errorf("return call %d = %+v, want %+v", i, h.backend.calls[i], call)
			}
		}
		h.backend.calls = nil
		h.backend.fail = ""
		h.changes <- original
		synctest.Wait()
		if len(h.backend.calls) != 0 {
			t.Fatalf("returned gateway retained cleanup debt: %+v", h.backend.calls)
		}
		if len(h.ended) != 0 {
			t.Fatal("route replacements ended PPP")
		}
	})
}
