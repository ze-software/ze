// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- Route retry progress.
// Related: pppoe_changes_test.go -- The actual runSession test driver.

package iface

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"
)

// TestPPPoEClientDesiredRouteProgress requires each desired gateway to reach
// AddRoute even while removal of an older gateway keeps failing.
func TestPPPoEClientDesiredRouteProgress(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newChangeSession(t, &PPPoEClientConfig{})
		var logs bytes.Buffer
		h.client.logger = slog.New(slog.NewTextHandler(&logs, nil))
		h.backend.fail = "route-remove"
		h.changes <- changedResult("192.0.2.3", "192.0.2.9", 1400)
		synctest.Wait()
		h.backend.calls = nil
		latest := changedResult("192.0.2.4", "192.0.2.10", 1300)
		h.changes <- latest
		synctest.Wait()
		adds := 0
		for _, call := range h.backend.calls {
			if call.op == "route-add" {
				adds++
				if call.peer != "192.0.2.10" {
					t.Fatalf("desired gateway = %s", call.peer)
				}
			}
		}
		if adds != 1 {
			t.Fatalf("desired route attempts = %d, want 1: %+v", adds, h.backend.calls)
		}
		if len(h.ended) != 0 {
			t.Fatal("old cleanup failure ended PPP")
		}
		// A failed desired install must identify its own gateway while an
		// older cleanup request is still pending from the preceding result.
		h.backend.fail = "route-add"
		logs.Reset()
		h.changes <- changedResult("192.0.2.5", "192.0.2.11", 1300)
		synctest.Wait()
		for _, text := range []string{"level=WARN", "gateway=192.0.2.11", "device=wan"} {
			if !strings.Contains(logs.String(), text) {
				t.Errorf("desired-install warning lacks %q: %s", text, logs.String())
			}
		}
	})
}

// TestPPPoEClientChangedMTUHitsConfiguredCap first installs a lower negotiated
// MTU, then requires exactly one capped write for a larger completed result.
func TestPPPoEClientChangedMTUHitsConfiguredCap(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newChangeSession(t, &PPPoEClientConfig{MTU: 1300})
		h.changes <- changedResult("192.0.2.2", "192.0.2.1", 1200)
		synctest.Wait()
		if len(h.backend.calls) != 1 {
			t.Fatalf("lower-MTU calls = %+v", h.backend.calls)
		}
		if h.backend.calls[0].mtu != 1200 {
			t.Fatal("did not establish the different previous MTU")
		}
		h.backend.calls = nil
		h.changes <- changedResult("192.0.2.2", "192.0.2.1", 1400)
		synctest.Wait()
		if len(h.backend.calls) != 1 {
			t.Fatalf("capped-MTU calls = %+v, want exactly one", h.backend.calls)
		}
		if h.backend.calls[0].op != "mtu" {
			t.Fatal("MTU-only change called another backend operation")
		}
		if h.backend.calls[0].mtu != 1300 {
			t.Fatalf("applied MTU = %d, want cap 1300", h.backend.calls[0].mtu)
		}
	})
}
