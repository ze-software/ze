// VALIDATES: spec-ospf-ext-14 AC-14/AC-15/AC-17/AC-18/AC-24, A-9 -- `debug ipv6 ospf inject
// lsa ...` originates a crafted v3 LSA through the base OriginateSelf seam only when debug is
// enabled; the scope is derived from the LS Type S2/S1 bits (reserved=11 rejected); an
// over-length body / bad hex is rejected; withdraw MaxAge-flushes; MinLSInterval paces.
// PREVENTS: v6 injection with debug off, a reserved-scope type, an unbounded body, a leaked
// withdraw, or a panic on a bad body.
package ospf

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// v3InjectRouterLSA is an area-scope Router-LSA injection, with no address-family keyword.
var v3InjectRouterLSA = []string{"type", "0x2001", "id", "1", "hex", "00000000"}

// TestDebugInjectLSATargetsTheNamedAddressFamily proves `debug ipv6 ospf inject lsa` puts the
// LSA into the LSDB of the address family its address-family keyword names, and answers the
// enablement before it reads the family. Disabled, the enablement refusal answers for a
// running family, an unknown word and a family that is not running alike, so the gate never
// depends on the family. Enabled, an injection naming ipv4-unicast (after the other
// keywords) lands in that engine's LSDB and not in ipv6-unicast's, and one naming no family
// lands in ipv6-unicast's. Method: twoAFIdleSet, the debugInjectIntoAF entry the
// cmdDebugInjectLSA arm calls, each LSDB read through v3DetailRowsOf.
func TestDebugInjectLSATargetsTheNamedAddressFamily(t *testing.T) {
	set := twoAFIdleSet(t)
	e6, _ := set.engineFor(afIPv6Unicast)
	e4, _ := set.engineFor(afIPv4Unicast)

	setDebugInjectEnabled(false)
	for _, family := range []string{afNameIPv4Unicast, "bogus", afNameIPv4Multicast} {
		args := append([]string{keywordAddressFamily, family}, v3InjectRouterLSA...)
		if _, err := set.debugInjectIntoAF(args); !errors.Is(err, errDebugInjectDisabled) {
			t.Errorf("inject naming %s with the enablement off = %v, want the enablement refusal", family, err)
		}
	}
	for _, e := range []*engine{e6, e4} {
		if hasInjectedRouterLSA(t, e) {
			t.Fatalf("an injection refused by the enablement reached the %s LSDB", e.af)
		}
	}

	injectEnabled(t)
	args := append(slices.Clone(v3InjectRouterLSA), keywordAddressFamily, afNameIPv4Unicast)
	if res, err := set.debugInjectIntoAF(args); err != nil || !res.Installed {
		t.Fatalf("inject into ipv4-unicast = %+v, %v; want installed", res, err)
	}
	if !hasInjectedRouterLSA(t, e4) {
		t.Fatalf("the LSA injected into ipv4-unicast is not in that engine's LSDB: %+v", v3DetailRowsOf(t, e4, ""))
	}
	if hasInjectedRouterLSA(t, e6) {
		t.Fatalf("the LSA injected into ipv4-unicast reached the ipv6-unicast LSDB: %+v", v3DetailRowsOf(t, e6, ""))
	}
	if res, err := set.debugInjectIntoAF(v3InjectRouterLSA); err != nil || !res.Installed {
		t.Fatalf("inject naming no family = %+v, %v; want installed", res, err)
	}
	if !hasInjectedRouterLSA(t, e6) {
		t.Fatalf("an injection naming no family is not in the ipv6-unicast LSDB: %+v", v3DetailRowsOf(t, e6, ""))
	}
}

// TestDebugInjectLSARefusesAnAddressFamilyItCannotUse proves an armed injection refuses an
// address-family keyword it cannot act on, and injects nothing: an unknown word (naming the
// four families), a family that is not running (naming the running ones and the command
// that lists them), a repeated keyword (whose fix is the operator's own command with the
// keyword once) and a keyword with no value (whose fix lists the running families).
func TestDebugInjectLSARefusesAnAddressFamilyItCannotUse(t *testing.T) {
	injectEnabled(t)
	set := twoAFIdleSet(t)
	cases := []struct {
		name  string
		args  []string
		names []string
	}{
		{"unknown", append([]string{keywordAddressFamily, "bogus"}, v3InjectRouterLSA...),
			[]string{afNameIPv6Unicast, afNameIPv6Multicast, afNameIPv4Unicast, afNameIPv4Multicast}},
		// The config's bare "ipv6" is not one of the four names the keyword takes; accepted,
		// it would inject into the running ipv6-unicast engine, which the LSDB check catches.
		{"bare ipv6", append([]string{keywordAddressFamily, afNameIPv6}, v3InjectRouterLSA...),
			[]string{strconv.Quote(afNameIPv6), afNameIPv6Unicast, afNameIPv6Multicast, afNameIPv4Unicast, afNameIPv4Multicast}},
		{"not running", append([]string{keywordAddressFamily, afNameIPv4Multicast}, v3InjectRouterLSA...),
			[]string{afNameIPv6Unicast, afNameIPv4Unicast, cmdShowIPv6Instance}},
		// The fix is the operator's own command with the keyword once, so it runs as typed.
		{"repeated", append([]string{keywordAddressFamily, afNameIPv4Unicast, keywordAddressFamily, afNameIPv4Unicast}, v3InjectRouterLSA...),
			[]string{"more than once", "\nfix: " + cmdDebugInjectLSA + " " + keywordAddressFamily + " " + afNameIPv4Unicast + " " + strings.Join(v3InjectRouterLSA, " ")}},
		// A repeated keyword followed by another keyword keeps that keyword in the fix.
		{"repeated before id", []string{keywordAddressFamily, afNameIPv4Unicast, "type", "0x2001", keywordAddressFamily, "id", "1", "hex", "00000000"},
			[]string{"more than once", "\nfix: " + cmdDebugInjectLSA + " " + keywordAddressFamily + " " + afNameIPv4Unicast + " " + strings.Join(v3InjectRouterLSA, " ")}},
		// The fix lists the running families, the value the keyword is missing.
		{"no value", append(slices.Clone(v3InjectRouterLSA), keywordAddressFamily),
			[]string{"no value", "\nfix: " + cmdShowIPv6Instance}},
	}
	for _, tc := range cases {
		_, err := set.debugInjectIntoAF(tc.args)
		if err == nil {
			t.Errorf("%s: inject %q was accepted", tc.name, tc.args)
			continue
		}
		for _, name := range tc.names {
			if !strings.Contains(err.Error(), name) {
				t.Errorf("%s: refusal %q does not name %q", tc.name, err, name)
			}
		}
	}
	for _, af := range set.runningAFs() {
		e, _ := set.engineFor(af)
		if hasInjectedRouterLSA(t, e) {
			t.Errorf("a refused injection reached the %s LSDB", af)
		}
	}
}

// hasInjectedRouterLSA reports whether e's LSDB holds a self-originated Router-LSA, which in
// twoAFIdleSet only an injection puts there.
func hasInjectedRouterLSA(t *testing.T, e *engine) bool {
	t.Helper()
	return slices.ContainsFunc(v3DetailRowsOf(t, e, ""), func(r v3DetailLSA) bool {
		return r.LSType == "router" && r.LocalOriginated
	})
}

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
