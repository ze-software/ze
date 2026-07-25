// VALIDATES: spec-ospf-ext-14 AC-14/AC-15/AC-17/AC-18/AC-24, A-9 -- `debug ipv6 ospf inject
// lsa ...` originates a crafted v3 LSA through the base OriginateSelf seam only when debug is
// enabled; the scope is derived from the LS Type S2/S1 bits (reserved=11 rejected); an
// over-length body / bad hex is rejected; withdraw MaxAge-flushes; MinLSInterval paces.
// PREVENTS: v6 injection with debug off, a reserved-scope type, an unbounded body, a leaked
// withdraw, or a panic on a bad body.
package ospf

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestV3InjectRequiresDebugEnabled(t *testing.T) {
	setDebugInjectEnabled(false)
	e := newV6DecodeEngine(t, types.RouterID{10, 0, 0, 1})
	_, err := e.debugInjectV3([]string{"scope", "area", "type", "0x2001", "id", "1", "hex", "00000000"})
	if err == nil || !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("v6 inject with debug off must be rejected, got %v", err)
	}
}

func TestDebugInjectV3LSAFloods(t *testing.T) {
	injectEnabled(t)
	rec := withDebugMetrics(t)
	router := types.RouterID{10, 0, 0, 1}
	e := newV6DecodeEngine(t, router)

	res, err := e.debugInjectV3([]string{"scope", "area", "type", "0x2001", "id", "1", "hex", "00000000"})
	if err != nil {
		t.Fatalf("v6 inject: %v", err)
	}
	if !res.Installed || res.Action != "originate" {
		t.Fatalf("v6 inject result = %+v", res)
	}
	rows := v3DetailRowsOf(t, e, "")
	found := false
	for _, r := range rows {
		if r.LSType == "router" && r.LocalOriginated {
			found = true
		}
	}
	if !found {
		t.Fatalf("injected v6 router LSA not visible/local: %+v", rows)
	}
	if rec.get("ze_ospfv3_debug_injections_total") == 0 || rec.get("ze_ospfv3_debug_injected_lsas") == 0 {
		t.Fatalf("v6 inject metrics not updated: %+v", rec.counts)
	}
}

func TestV3InjectWithdrawFlushes(t *testing.T) {
	injectEnabled(t)
	rec := withDebugMetrics(t)
	router := types.RouterID{10, 0, 0, 1}
	e := newV6DecodeEngine(t, router)
	if _, err := e.debugInjectV3([]string{"scope", "area", "type", "0x2001", "id", "2", "hex", "00000000"}); err != nil {
		t.Fatalf("inject: %v", err)
	}
	res, err := e.debugInjectV3([]string{"scope", "area", "type", "0x2001", "id", "2", "withdraw"})
	if err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if res.Action != "withdraw" {
		t.Fatalf("action = %q", res.Action)
	}
	if rec.get("ze_ospfv3_debug_injected_lsas") != 0 {
		t.Fatalf("v6 injected gauge should be 0 after withdraw, got %d", rec.get("ze_ospfv3_debug_injected_lsas"))
	}
}

func TestV3InjectReservedScopeRejected(t *testing.T) {
	injectEnabled(t)
	e := newV6DecodeEngine(t, types.RouterID{10, 0, 0, 1})
	// LS Type 0x6001 has S2/S1 = 11 (reserved).
	if _, err := e.debugInjectV3([]string{"scope", "as", "type", "0x6001", "id", "1", "hex", "00"}); err == nil {
		t.Fatalf("reserved scope (S2/S1=11) must be rejected")
	}
}

func TestV3InjectBodyOverflowRejected(t *testing.T) {
	injectEnabled(t)
	e := newV6DecodeEngine(t, types.RouterID{10, 0, 0, 1})
	big := strings.Repeat("00", 65516) // 65516 bytes > 65515 max body
	if _, err := e.debugInjectV3([]string{"scope", "area", "type", "0x2001", "id", "1", "hex", big}); err == nil {
		t.Fatalf("over-length v6 body must be rejected")
	}
}

func TestV3InjectMalformedBodyRejected(t *testing.T) {
	injectEnabled(t)
	e := newV6DecodeEngine(t, types.RouterID{10, 0, 0, 1})
	if _, err := e.debugInjectV3([]string{"scope", "area", "type", "0x2001", "id", "1", "hex", "abc"}); err == nil {
		t.Fatalf("odd-length hex must be rejected")
	}
}

func TestV3InjectRespectsMinLSInterval(t *testing.T) {
	injectEnabled(t)
	router := types.RouterID{10, 0, 0, 1}
	e := newV6DecodeEngine(t, router)
	if _, err := e.debugInjectV3([]string{"scope", "area", "type", "0x2001", "id", "3", "hex", "00000000"}); err != nil {
		t.Fatalf("first inject: %v", err)
	}
	res, err := e.debugInjectV3([]string{"scope", "area", "type", "0x2001", "id", "3", "hex", "02000000"})
	if err != nil {
		t.Fatalf("second inject: %v", err)
	}
	if res.Installed {
		t.Fatalf("v6 re-origination within MinLSInterval should be paced")
	}
}
