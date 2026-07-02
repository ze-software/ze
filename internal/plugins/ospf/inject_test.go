// VALIDATES: spec-ospf-ext-14 AC-13/AC-15/AC-17/AC-24, A-8/A-9 -- `debug ip ospf inject
// opaque ...` originates a crafted Private-Use opaque LSA through the ext-1 seam only when
// the debug enablement is on; withdraw MaxAge-flushes it; the injected LSA is local; a bad
// hex body / out-of-range selector is rejected without a panic; MinLSInterval paces
// re-origination.
// PREVENTS: injection reachable with debug off, a non-Private-Use opaque type, an
// unbounded opaque id, a withdraw that leaks, or a panic on a bad body.
package ospf

import (
	"strings"
	"testing"
)

func injectEnabled(t *testing.T) {
	t.Helper()
	setDebugInjectEnabled(true)
	t.Cleanup(func() { setDebugInjectEnabled(false) })
}

func TestInjectRequiresDebugEnabled(t *testing.T) {
	setDebugInjectEnabled(false)
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)
	_, err := eng.debugInjectOpaque([]string{"scope", "area", "id", "1", "hex", "01020304"})
	if err == nil || !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("inject with debug off must be rejected, got err=%v", err)
	}
	if len(eng.opaqueDetailRows(OpaqueScopeArea)) != 0 {
		t.Fatalf("no LSA should be originated when debug is disabled")
	}
}

func TestDebugInjectOpaqueFloods(t *testing.T) {
	injectEnabled(t)
	rec := withDebugMetrics(t)
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)

	res, err := eng.debugInjectOpaque([]string{"scope", "area", "id", "5", "hex", "0001000401020304"})
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	if !res.Installed || res.Action != "originate" || res.OpaqueID != 5 {
		t.Fatalf("inject result = %+v", res)
	}
	rows := eng.opaqueDetailRows(OpaqueScopeArea)
	if len(rows) != 1 || !rows[0].LocalOriginated || rows[0].OpaqueID != 5 {
		t.Fatalf("injected LSA not visible/local: %+v", rows)
	}
	if rec.get("ze_ospf_debug_injections_total") == 0 || rec.get("ze_ospf_debug_injected_lsas") == 0 {
		t.Fatalf("inject metrics not updated: %+v", rec.counts)
	}
}

func TestInjectUsesPrivateOpaqueType(t *testing.T) {
	injectEnabled(t)
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)
	res, err := eng.debugInjectOpaque([]string{"scope", "area", "id", "1", "hex", "00"})
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	if res.OpaqueType < 128 {
		t.Fatalf("default opaque type %d is not Private-Use (128-255)", res.OpaqueType)
	}
}

func TestInjectWithdrawFlushes(t *testing.T) {
	injectEnabled(t)
	rec := withDebugMetrics(t)
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)
	if _, err := eng.debugInjectOpaque([]string{"scope", "area", "id", "7", "hex", "00010000"}); err != nil {
		t.Fatalf("inject: %v", err)
	}
	res, err := eng.debugInjectOpaque([]string{"scope", "area", "id", "7", "withdraw"})
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if res.Action != "withdraw" {
		t.Fatalf("withdraw action = %q", res.Action)
	}
	if rec.get("ze_ospf_debug_injected_lsas") != 0 {
		t.Fatalf("injected gauge should return to 0 after withdraw, got %d", rec.get("ze_ospf_debug_injected_lsas"))
	}
}

func TestInjectBoundaryRejects(t *testing.T) {
	injectEnabled(t)
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)
	cases := map[string][]string{
		"opaque-id over 24 bits": {"scope", "area", "id", "16777216", "hex", "00"},
		"opaque-type below 128":  {"scope", "area", "id", "1", "type", "127", "hex", "00"},
		"bad hex body":           {"scope", "area", "id", "1", "hex", "zz"},
		"unknown scope":          {"scope", "bogus", "id", "1", "hex", "00"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := eng.debugInjectOpaque(args); err == nil {
				t.Fatalf("%s: expected rejection", name)
			}
		})
	}
}

func TestInjectRespectsMinLSInterval(t *testing.T) {
	injectEnabled(t)
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)
	if _, err := eng.debugInjectOpaque([]string{"scope", "area", "id", "9", "hex", "00010000"}); err != nil {
		t.Fatalf("first inject: %v", err)
	}
	// A different body for the same key within MinLSInterval is rate-limited (not installed).
	res, err := eng.debugInjectOpaque([]string{"scope", "area", "id", "9", "hex", "0001000401020304"})
	if err != nil {
		t.Fatalf("second inject: %v", err)
	}
	if res.Installed {
		t.Fatalf("re-origination within MinLSInterval should be paced (not installed)")
	}
}

func TestInjectMalformedBodyRecovered(t *testing.T) {
	injectEnabled(t)
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)
	// Odd-length hex is invalid; the engine must reject cleanly, never panic.
	if _, err := eng.debugInjectOpaque([]string{"scope", "area", "id", "1", "hex", "abc"}); err == nil {
		t.Fatalf("odd-length hex must be rejected")
	}
}
