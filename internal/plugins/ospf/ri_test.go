// VALIDATES: spec-ospf-ext-3 -- the RFC 7770 Router Information consumer: capability bits
// derived from live state (AC-6), one shared body identical across address families (AC-11),
// the type-1 TLV first (AC-7), idempotent re-origination (AC-10), withdraw on disable (AC-9),
// instance overflow (AC-16), smallest-Instance-ID selection (AC-15), the RFC 7770 sec 2.7
// NSSA SHOULD (R-9), and the default area+AS scope (AC-5).
// PREVENTS: a lying capability advertisement, divergent v2/v3 bodies, a mis-ordered TLV
// stream, LSDB churn on unchanged bodies, a lingering RI LSA after disable, silent truncation
// on overflow, or the wrong instance winning on receive.
package ospf

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

// riCfg builds an OSPFv2 config with opaque enabled, one backbone area with a point-to-point
// interface, and a router-information container (enabled + the given scopes).
func riCfg(enabled bool, scopes ...string) string {
	scopeJSON := ""
	if len(scopes) > 0 {
		parts := make([]string, len(scopes))
		for i, s := range scopes {
			parts[i] = strconv.Quote(s)
		}
		scopeJSON = `,"scope":[` + strings.Join(parts, ",") + `]`
	}
	return `{"ospf":{"router-id":"1.1.1.1","opaque":true,` +
		`"router-information":{"enabled":` + strconv.FormatBool(enabled) + scopeJSON + `},` +
		`"areas":{"area":{"0":{"area-id":"0"}}},` +
		`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","network-type":"point-to-point"}}}}}`
}

func TestRICapabilityBitsFromState(t *testing.T) {
	// max-metric configured -> stub-router; no TE -> TE clear; GR seam -> GR-helper set.
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"1.1.1.1","opaque":true,`+
		`"max-metric":{"router-lsa":{"always":"true"}},`+
		`"router-information":{"enabled":true}}}`)
	eng.riGRState = func() (bool, bool) { return false, true }

	caps := eng.deriveRICapabilities()
	if !caps.StubRouter {
		t.Errorf("stub-router bit not set from max-metric config")
	}
	if caps.TE {
		t.Errorf("TE bit set with no TE configured")
	}
	if !caps.GRHelper || caps.GRCapable {
		t.Errorf("GR bits = capable %v helper %v, want capable=false helper=true", caps.GRCapable, caps.GRHelper)
	}
	// The encoded word sets bit 1 (GR-helper) and bit 2 (stub-router), clears bit 3 (TE).
	field := caps.infoField()
	// RFC requirement: RFC7770-2.4-2 negative -- an unconfigured capability (no TE anywhere in
	// this config) leaves its Informational Capabilities bit clear in the encoded word, so the
	// advertisement cannot over-claim a capability the router lacks (§2.4).
	if field&packet.RIInfoBitMask(packet.RIInfoBitTrafficEngineering) != 0 {
		t.Errorf("TE bit set in encoded field %#08x", field)
	}
	// RFC requirement: RFC7770-2.4-2 positive -- a configured capability (stub-router derived
	// from the max-metric config) sets its Informational Capabilities bit in the encoded word, so
	// the TLV accurately reflects the router's actual capabilities (§2.4).
	if field&packet.RIInfoBitMask(packet.RIInfoBitStubRouter) == 0 {
		t.Errorf("stub-router bit clear in encoded field %#08x", field)
	}
	if field&packet.RIInfoBitMask(packet.RIInfoBitGracefulRestartHelper) == 0 {
		t.Errorf("GR-helper bit clear in encoded field %#08x", field)
	}
}

func TestRICapabilityTEBitFromConfig(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"1.1.1.1","opaque":true,"router-address":"9.9.9.9",`+
		`"router-information":{"enabled":true},`+
		`"areas":{"area":{"0":{"area-id":"0"}}},`+
		`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","traffic-engineering":{"enable":true}}}}}}`)
	// RFC requirement: RFC7770-2.4-2 positive -- a configured capability (an interface
	// traffic-engineering block) is accurately reflected as the RFC 3630 TE Informational
	// Capability, deriving from live config rather than a static flag (§2.4).
	if !eng.deriveRICapabilities().TE {
		t.Fatalf("TE bit not set with an interface traffic-engineering block")
	}
}

func TestRIBodyIdenticalAcrossAF(t *testing.T) {
	resetRITLVs()
	t.Cleanup(resetRITLVs)
	router := types.RouterID{1, 1, 1, 1}
	// Two engines, different codecs, identical RI-relevant config -> byte-identical bodies.
	v4, _ := newRedistEngine(t, riCfg(true, "area"))
	cfg6, err := parseOSPFConfig(ospfSec(riCfg(true, "area")), nil)
	if err != nil {
		t.Fatalf("parse v6 cfg: %v", err)
	}
	v6 := newV6RIEngine(t)
	v6.setConfig(cfg6)

	b4 := v4.buildRIInstances(OpaqueScopeArea, router)
	b6 := v6.buildRIInstances(OpaqueScopeArea, router)
	if len(b4) != 1 || len(b6) != 1 {
		t.Fatalf("instance counts v4=%d v6=%d, want 1 each", len(b4), len(b6))
	}
	if !bytes.Equal(b4[0], b6[0]) {
		t.Fatalf("RI body differs across address families:\n v2=%x\n v3=%x", b4[0], b6[0])
	}
}

func TestRITLVType1First(t *testing.T) {
	resetRITLVs()
	t.Cleanup(resetRITLVs)
	// Even with a registered builder, the type-1 Informational Capabilities TLV is first (sec 2.4).
	if err := registerRITLV(8, OpaqueScopeArea, func(types.RouterID) []packet.RITLV {
		return []packet.RITLV{{Type: 8, Value: []byte{1, 2, 3, 4}}}
	}); err != nil {
		t.Fatalf("registerRITLV: %v", err)
	}
	eng, router := newRedistEngine(t, riCfg(true, "area"))
	decoded, err := packet.DecodeRITLVStream(eng.buildRIInstances(OpaqueScopeArea, router)[0])
	if err != nil {
		t.Fatalf("decode RI body: %v", err)
	}
	// RFC requirement: RFC7770-2.4-1 positive -- the type-1 Informational Capabilities TLV is the
	// FIRST TLV in Instance 0 of the RI LSA, even with a registered downstream TLV present, so a
	// receiver always finds the capability word at the canonical position (§2.4).
	if len(decoded) == 0 || decoded[0].Type != packet.RITLVInformationalCapabilities {
		t.Fatalf("first TLV type = %v, want 1 (Informational Capabilities)", decoded)
	}
}

func TestRIFunctionalCapabilitiesEmittedZero(t *testing.T) {
	resetRITLVs()
	t.Cleanup(resetRITLVs)
	// Ze supports no functional capabilities, so the Router Functional Capabilities TLV (type 2)
	// it originates carries the constant all-zero value ("no functional capability supported").
	eng, router := newRedistEngine(t, riCfg(true, "area"))
	decoded, err := packet.DecodeRITLVStream(eng.buildRIInstances(OpaqueScopeArea, router)[0])
	if err != nil {
		t.Fatalf("decode RI body: %v", err)
	}
	var fn *packet.RITLV
	for i := range decoded {
		if decoded[i].Type == packet.RITLVFunctionalCapabilities {
			fn = &decoded[i]
		}
	}
	if fn == nil {
		t.Fatalf("no Functional Capabilities (type-2) TLV emitted in Instance 0")
	}
	if len(fn.Value) != packet.RICapabilitiesMinLen {
		t.Fatalf("functional capabilities value len = %d, want %d", len(fn.Value), packet.RICapabilitiesMinLen)
	}
	// RFC requirement: RFC7770-2.6-2 positive -- the originated Functional Capabilities TLV value
	// is the constant all-zero 4-octet word, so it reflects the router's actual (empty) functional
	// capability set rather than advertising an unsupported capability (§2.6).
	for i, b := range fn.Value {
		if b != 0 {
			t.Fatalf("functional capabilities value byte %d = %#x, want 0 (no functional capability supported)", i, b)
		}
	}
	if got := packet.RIReadCapabilities(fn.Value); got != 0 {
		t.Fatalf("functional capabilities word = %#08x, want 0", got)
	}
}

func TestRIOriginateIdempotent(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	eng, router := newRedistEngine(t, riCfg(true, "area"))
	var origs int
	eng.ri.originations = countingCounterVec{n: &origs}

	first := eng.riOriginate(router)
	countAfterFirst := origs
	if countAfterFirst == 0 {
		t.Fatalf("first origination pass counted no RI originations")
	}
	second := eng.riOriginate(router)
	if origs != countAfterFirst {
		t.Fatalf("second (unchanged) pass counted %d new originations, want 0", origs-countAfterFirst)
	}
	// No withdraws on the idempotent second pass, and the same instances desired.
	if len(withdrawsOf(second)) != 0 {
		t.Fatalf("idempotent pass emitted withdraws: %+v", withdrawsOf(second))
	}
	if len(first) != len(second) {
		t.Fatalf("origination set changed on unchanged config: %d then %d", len(first), len(second))
	}
}

func TestRIWithdrawFlushes(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	eng, router := newRedistEngine(t, riCfg(true, "area", "as"))
	if got := eng.riOriginate(router); len(withdrawsOf(got)) != 0 {
		t.Fatalf("initial pass emitted withdraws: %+v", withdrawsOf(got))
	}
	// Disable RI: the previously originated instances are withdrawn (MaxAge flush via ext-1).
	offCfg, err := parseOSPFConfig(ospfSec(riCfg(false)), nil)
	if err != nil {
		t.Fatalf("parse off cfg: %v", err)
	}
	eng.setConfig(offCfg)
	out := eng.riOriginate(router)
	wd := withdrawsOf(out)
	if len(wd) == 0 {
		t.Fatalf("disabling RI emitted no withdraws")
	}
	for _, o := range out {
		if !o.Withdraw {
			t.Fatalf("origination emitted alongside withdraws after disable: %+v", o)
		}
	}
}

func TestRIInstanceOverflow(t *testing.T) {
	resetRITLVs()
	t.Cleanup(resetRITLVs)
	// A registered builder returns two ~40KB TLVs; they cannot share Instance 0 (RFC 7770 sec 3).
	big := make([]byte, 40000)
	if err := registerRITLV(8, OpaqueScopeArea, func(types.RouterID) []packet.RITLV {
		return []packet.RITLV{{Type: 8, Value: big}, {Type: 9, Value: big}}
	}); err != nil {
		t.Fatalf("registerRITLV: %v", err)
	}
	eng, router := newRedistEngine(t, riCfg(true, "area"))
	bodies := eng.buildRIInstances(OpaqueScopeArea, router)
	if len(bodies) < 2 {
		t.Fatalf("instances = %d, want >= 2 (overflow)", len(bodies))
	}
	// Instance 0 keeps the type-1 TLV first (sec 2.4).
	if types0 := riTLVTypes(t, bodies[0]); len(types0) == 0 || types0[0] != packet.RITLVInformationalCapabilities {
		t.Fatalf("Instance 0 first TLV = %v, want type-1", types0)
	}
	for i, b := range bodies {
		if len(b) > riMaxInstanceBodyLen {
			t.Fatalf("instance %d body %d bytes exceeds max %d", i, len(b), riMaxInstanceBodyLen)
		}
	}
}

func TestRIMultiInstanceSmallestID(t *testing.T) {
	// RFC 7770 sec 3: for an unspecified-multi-instance TLV, the smallest Instance ID wins.
	inst0 := packet.EncodeRITLVs([]packet.RITLV{{Type: packet.RITLVInformationalCapabilities, Value: packet.RICapabilitiesValue(packet.RIInfoBitMask(packet.RIInfoBitStubRouter))}})
	inst1 := packet.EncodeRITLVs([]packet.RITLV{{Type: packet.RITLVInformationalCapabilities, Value: packet.RICapabilitiesValue(packet.RIInfoBitMask(packet.RIInfoBitTrafficEngineering))}})
	view := buildRIView([]riObservation{
		{af: "v2", scope: "as", advRouter: "2.2.2.2", instance: 1, body: inst1},
		{af: "v2", scope: "as", advRouter: "2.2.2.2", instance: 0, body: inst0},
	})
	if len(view.RouterInformation) != 1 {
		t.Fatalf("router entries = %d, want 1 group", len(view.RouterInformation))
	}
	e := view.RouterInformation[0]
	if e.EffectiveInstance != 0 {
		t.Fatalf("effective instance = %d, want 0 (smallest)", e.EffectiveInstance)
	}
	if len(e.Capabilities) != 1 || e.Capabilities[0] != "stub-router" {
		t.Fatalf("effective capabilities = %v, want [stub-router] from Instance 0", e.Capabilities)
	}
	if len(e.Instances) != 2 {
		t.Fatalf("listed instances = %d, want 2", len(e.Instances))
	}
}

func TestRIASScopeAlsoAreaIntoNSSA(t *testing.T) {
	resetOpaqueConsumers()
	t.Cleanup(resetOpaqueConsumers)
	// AS scope + an attached NSSA -> also an area-scoped RI LSA into the NSSA (RFC 7770 sec 2.7).
	cfg := `{"ospf":{"router-id":"1.1.1.1","opaque":true,` +
		`"router-information":{"enabled":true,"scope":["as"]},` +
		`"areas":{"area":{"0":{"area-id":"0"},"0.0.0.7":{"area-id":"0.0.0.7","area-type":"nssa"}}},` +
		`"interfaces":{"interface":{` +
		`"eth0":{"name":"eth0","area":"0"},` +
		`"eth1":{"name":"eth1","area":"0.0.0.7"}}}}}`
	eng, router := newRedistEngine(t, cfg)
	out := eng.riOriginate(router)
	var haveAS, haveNSSAArea bool
	nssa := types.AreaID{0, 0, 0, 7}
	for _, o := range out {
		if o.Withdraw {
			continue
		}
		if o.Scope == OpaqueScopeAS {
			haveAS = true
		}
		if o.Scope == OpaqueScopeArea && o.Area == nssa {
			haveNSSAArea = true
		}
	}
	if !haveAS {
		t.Fatalf("no AS-scope RI originated")
	}
	if !haveNSSAArea {
		t.Fatalf("no area-scope RI originated into the attached NSSA (RFC 7770 sec 2.7 SHOULD)")
	}
}

func TestRIDefaultScopeAreaAndAS(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(riCfg(true)), nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ri := cfg.RouterInformation
	if !ri.Enabled {
		t.Fatalf("RI not enabled")
	}
	if !ri.HasScope(OpaqueScopeArea) || !ri.HasScope(OpaqueScopeAS) {
		t.Fatalf("default scope = %v, want area + as", ri.Scopes)
	}
	if ri.HasScope(OpaqueScopeLink) {
		t.Fatalf("default scope must not include link")
	}
}

// withdrawsOf returns the withdraw originations in a set.
func withdrawsOf(out []opaqueOrigination) []opaqueOrigination {
	var wd []opaqueOrigination
	for _, o := range out {
		if o.Withdraw {
			wd = append(wd, o)
		}
	}
	return wd
}
