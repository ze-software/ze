// Design: docs/architecture/ospf/ospf-ext-3-router-information.md -- the RFC 7770 Router Information LSA.
// RFC: rfc/short/rfc7770.md -- sec 2.1 (OSPFv2 RI = Opaque type 4), sec 2.3 (TLV stream),
// sec 2.4 (Informational Capabilities TLV first), sec 2.5 (capability bits), sec 2.6
// (Functional Capabilities TLV), sec 2.7 (per-scope flooding), sec 3 (multi-instance).
// RFC: rfc/short/rfc5250.md sec 3/5 -- the OSPFv2 opaque carrier RI rides on (ext-1).
//
// This is the address-family-neutral RI consumer. It derives the Informational Capability
// bits from live engine state (RFC 7770 sec 2.4 MUST: accurate in the advertised scope),
// builds the SINGLE shared RI TLV body (identical bytes across OSPFv2 and OSPFv3, AC-11),
// and carries it two ways: OSPFv2 as an ext-1 opaque consumer (Opaque type 4, this file),
// OSPFv3 as a native function-code-12 self-LSA (origination_v6_ri.go). The carrier owns the
// LS-ID split, scope flooding, the O-bit and the RFC 5250 sec 5 reachability gate; this file
// supplies only the body and the capability derivation, and owns the ze_ospf_ri_* metrics.

package ospf

import (
	"bytes"
	"sort"
	"sync"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// riMaxInstanceBodyLen is the largest RI LSA body (the bytes after the 20-octet LSA header)
// one instance can carry: the 16-bit LSA Length field (RFC 7770 figures 1/2) caps the whole
// LSA, so the body max is 0xFFFF - 20. Registered TLVs that overflow one instance spill into
// the next (RFC 7770 sec 3); Instance 0 always keeps the type-1 TLV first.
const riMaxInstanceBodyLen = 0xFFFF - types.LSAHeaderLen

// riCapabilityBitLabels name the ze_ospf_ri_capability_bits gauge series (RFC 7770 sec 2.5
// bits 0-3, the ones this implementation derives).
const (
	riBitLabelGRCapable  = "gr-capable"
	riBitLabelGRHelper   = "gr-helper"
	riBitLabelStubRouter = "stub-router"
	riBitLabelTE         = "te"
)

// riMetrics is the ze_ospf_ri_* series (spec-ospf-ext-3), address-family-neutral and owned
// by the RI consumer. The handles are shared between the OSPFv2 and OSPFv3 engines (the af
// label distinguishes them); the gauge trackers are per-engine.
type riMetrics struct {
	lsas          metrics.GaugeVec   // labels: af, scope
	originations  metrics.CounterVec // labels: af, scope
	received      metrics.CounterVec // labels: af
	builderErrors metrics.Counter    // no labels
	capabilityBit metrics.GaugeVec   // labels: bit
}

func nopRIMetrics() riMetrics {
	nop := metrics.NopRegistry{}
	return riMetrics{
		lsas:          nop.GaugeVec("", "", nil),
		originations:  nop.CounterVec("", "", nil),
		received:      nop.CounterVec("", "", nil),
		builderErrors: nop.Counter("", ""),
		capabilityBit: nop.GaugeVec("", "", nil),
	}
}

// setRIMetrics registers the five ze_ospf_ri_* series on reg (called for the IPv4 engine).
func (e *engine) setRIMetrics(reg metrics.Registry) {
	e.ri = riMetrics{
		lsas:          reg.GaugeVec("ze_ospf_ri_lsas", "Current OSPF Router Information LSAs, by address family and flooding scope.", []string{"af", "scope"}),
		originations:  reg.CounterVec("ze_ospf_ri_originations_total", "Total OSPF Router Information LSAs originated, by address family and flooding scope.", []string{"af", "scope"}),
		received:      reg.CounterVec("ze_ospf_ri_received_total", "Total OSPF Router Information LSAs received from peers, by address family.", []string{"af"}),
		builderErrors: reg.Counter("ze_ospf_ri_tlv_builder_errors_total", "Total registered RI-TLV builder panics recovered during RI LSA origination."),
		capabilityBit: reg.GaugeVec("ze_ospf_ri_capability_bits", "Current OSPF Router Informational Capability bits advertised by this router, by bit name.", []string{"bit"}),
	}
	e.riLSAsGauge = newGaugeVecTracker()
	e.riCapBitsGauge = newGaugeVecTracker()
}

// afLabel is the ze_ospf_ri_* address-family label: v2 for OSPFv2, v3 for OSPFv3.
func (e *engine) afLabel() string {
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		return "v3"
	}
	return "v2"
}

// riCapabilities is this router's RFC 7770 sec 2.5 Informational Capability state (bits 0-3),
// derived from live config; bits 4-5 (P2P-over-LAN, Experimental TE) are left clear.
type riCapabilities struct {
	GRCapable  bool // bit 0, RFC 3623
	GRHelper   bool // bit 1, RFC 3623
	StubRouter bool // bit 2, RFC 6987
	TE         bool // bit 3, RFC 3630
}

// infoField packs the capability booleans into the 32-bit Informational Capabilities word
// (RFC 7770 sec 2.4: bits numbered left to right, MSB = bit 0).
func (c riCapabilities) infoField() uint32 {
	var f uint32
	if c.GRCapable {
		f |= packet.RIInfoBitMask(packet.RIInfoBitGracefulRestart)
	}
	if c.GRHelper {
		f |= packet.RIInfoBitMask(packet.RIInfoBitGracefulRestartHelper)
	}
	if c.StubRouter {
		f |= packet.RIInfoBitMask(packet.RIInfoBitStubRouter)
	}
	if c.TE {
		f |= packet.RIInfoBitMask(packet.RIInfoBitTrafficEngineering)
	}
	return f
}

// deriveRICapabilities reads live engine state into the RFC 7770 sec 2.5 informational bits.
// RFC 7770 sec 2.4 MUST: the bits accurately reflect the router's capabilities. Derived from
// real config each origination, so the advertisement cannot lie and stays correct as
// features toggle: stub-router from the RFC 6987 max-metric config, TE from the RFC 3630 TE
// config, graceful-restart from the GR seam (ext-9 sets it; nil = not GR-capable).
func (e *engine) deriveRICapabilities() riCapabilities {
	e.mu.Lock()
	cfg := e.cfg
	gr := e.riGRState
	e.mu.Unlock()
	caps := riCapabilities{}
	mm := cfg.MaxMetric
	caps.StubRouter = mm.RouterLSAAlways || mm.OnStartupSec > 0 || mm.OnShutdownSec > 0
	caps.TE = cfg.HasTERouterAddress || anyInterfaceHasTE(cfg)
	if gr != nil {
		caps.GRCapable, caps.GRHelper = gr()
	}
	return caps
}

// anyInterfaceHasTE reports whether any interface carries a traffic-engineering block (the
// RFC 3630 TE capability, informational bit 3).
func anyInterfaceHasTE(cfg ospfConfig) bool {
	for i := range cfg.Interfaces {
		if cfg.Interfaces[i].TE != nil {
			return true
		}
	}
	return false
}

// buildRIInstances builds the shared RI LSA body for one flooding scope as a slice of
// per-instance bodies (RFC 7770 sec 3 multi-instance). Instance 0 always carries the type-1
// Informational Capabilities TLV FIRST (sec 2.4 MUST) and then the empty type-2 Functional
// Capabilities carrier (sec 2.6); registered consumer TLVs for this scope follow in ascending
// TLV-type order (sec 2.4) and overflow into Instance 1+. The body is identical across
// address families for the same scope (AC-11) because both carriages call this one builder.
func (e *engine) buildRIInstances(scope OpaqueScope, router types.RouterID) [][]byte {
	caps := e.deriveRICapabilities()
	lead := []packet.RITLV{
		{Type: packet.RITLVInformationalCapabilities, Value: packet.RICapabilitiesValue(caps.infoField())},
		{Type: packet.RITLVFunctionalCapabilities, Value: packet.RICapabilitiesValue(0)},
	}
	builders := riTLVBuildersForScope(scope)
	extra := make([]packet.RITLV, 0, len(builders))
	for _, entry := range builders {
		extra = append(extra, invokeRITLVBuilder(entry, router, e.ri.builderErrors.Inc)...)
	}
	return packRIInstances(lead, extra)
}

// packRIInstances greedily packs the lead TLVs (Instance 0) and the registered TLVs into
// 4-byte-aligned instance bodies bounded by riMaxInstanceBodyLen. Instance 0 keeps the lead
// TLVs (type-1 first); a TLV that would overflow the current instance starts a new one
// (RFC 7770 sec 3). It always returns at least Instance 0.
func packRIInstances(lead, extra []packet.RITLV) [][]byte {
	cur := append([]packet.RITLV(nil), lead...)
	curLen := packet.RITLVsEncodedLen(cur)
	var instances [][]byte
	flush := func() { instances = append(instances, packet.EncodeRITLVs(cur)) }
	for _, t := range extra {
		tlen := packet.RITLVsEncodedLen([]packet.RITLV{t})
		if curLen+tlen > riMaxInstanceBodyLen && len(cur) > 0 {
			flush()
			cur = nil
			curLen = 0
		}
		cur = append(cur, t)
		curLen += tlen
	}
	flush()
	return instances
}

// riOrigKey identifies one OSPFv2 RI opaque origination for withdraw diffing: its flooding
// scope, target area (area scope), target interface (link scope), and Instance ID.
type riOrigKey struct {
	scope OpaqueScope
	area  types.AreaID
	iface string
	inst  uint32
}

// riOriginator holds the OSPFv2 RI opaque-origination state: the bodies emitted last pass,
// so an instance whose body is unchanged does not re-count an origination (idempotent,
// AC-10) and an instance no longer desired is withdrawn (AC-9). OSPFv3 RI needs no such
// tracking; its self-LSA seam (OriginateSelf + FlushStaleSelfLSAs) is idempotent and
// self-withdrawing.
type riOriginator struct {
	mu   sync.Mutex
	prev map[riOrigKey][]byte
}

func newRIOriginator() *riOriginator { return &riOriginator{prev: map[riOrigKey][]byte{}} }

// registerRIConsumer registers the OSPFv2 RI opaque consumer (Opaque type 4) bound to this
// engine (RFC 7770 sec 2.1). Production calls it once for the IPv4 engine; the OSPFv3 RI is
// native (origination_v6_ri.go). The registered default scope is area; a per-origination
// Scope override selects area/AS/link (RFC 7770 sec 2.7).
func registerRIConsumer(e *engine) error {
	// spec-ospf-ext-14: wire the RI opaque body decoder into the debug detail registry so
	// `show ospf database opaque-area detail` renders RI LSAs as their TLV stream.
	registerOpaqueDetailDecoder(packet.RIOpaqueType, "router-information", func(b []byte) (any, error) {
		v, err := packet.DecodeRITLVStream(b)
		return v, err
	})
	return registerOpaqueConsumer(packet.RIOpaqueType, OpaqueScopeArea, e.riOriginate, e.riOnReceive)
}

// riOriginate is the OSPFv2 RI OnOriginate (RFC 5250 sec 3 pull model): each self-LSA pass it
// returns the FULL desired set of RI opaque originations, plus a withdraw for any instance no
// longer desired. The carrier (ext-1) assigns sequence numbers, builds the Opaque LSA (LS
// type from scope, LS ID = Opaque type 4 << 24 | Instance ID), installs, floods, and (for
// Type 11) applies the RFC 5250 sec 5 reachability gate; an unchanged body floods nothing.
func (e *engine) riOriginate(router types.RouterID) []opaqueOrigination {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()

	o := e.riOrig
	o.mu.Lock()
	defer o.mu.Unlock()

	var out []opaqueOrigination
	desired := make(map[riOrigKey][]byte)
	ri := cfg.RouterInformation
	if ri.Enabled {
		if ri.HasScope(OpaqueScopeArea) {
			for _, area := range attachedAreaIDs(cfg) {
				e.riEmitOpaque(router, OpaqueScopeArea, area, "", &out, desired)
			}
		}
		if ri.HasScope(OpaqueScopeAS) {
			e.riEmitOpaque(router, OpaqueScopeAS, types.BackboneArea, "", &out, desired)
			// RFC 7770 sec 2.7 SHOULD: with AS scope, also advertise an area-scoped RI LSA into
			// each attached NSSA area so NSSA-internal routers still see the capabilities.
			// Only when Area scope is not already configured: if it is, the Area branch above
			// already emitted an area-scoped RI into every attached area (NSSAs included), so
			// repeating it here would double-emit the same (scope=area, area=nssa) instance.
			if !ri.HasScope(OpaqueScopeArea) {
				for _, area := range attachedNSSAAreas(cfg) {
					e.riEmitOpaque(router, OpaqueScopeArea, area, "", &out, desired)
				}
			}
		}
		if ri.HasScope(OpaqueScopeLink) {
			for _, name := range activeInterfaceNames(cfg) {
				e.riEmitOpaque(router, OpaqueScopeLink, cfg.interfaceArea(name), name, &out, desired)
			}
		}
	}

	// Withdraw instances originated last pass but no longer desired (AC-9).
	for key := range o.prev {
		if _, ok := desired[key]; !ok {
			out = append(out, opaqueOrigination{OpaqueID: key.inst, Area: key.area, Interface: key.iface, Scope: key.scope, Withdraw: true})
		}
	}
	o.prev = desired
	e.refreshRIMetrics()
	return out
}

// riEmitOpaque appends one opaque origination per instance body of the RI LSA for a
// (scope, area, iface) target, counting an origination only when the body is new or changed
// (idempotent re-origination, AC-10).
func (e *engine) riEmitOpaque(router types.RouterID, scope OpaqueScope, area types.AreaID, iface string, out *[]opaqueOrigination, desired map[riOrigKey][]byte) {
	bodies := e.buildRIInstances(scope, router)
	for i, body := range bodies {
		inst := uint32(i)
		key := riOrigKey{scope: scope, area: area, iface: iface, inst: inst}
		*out = append(*out, opaqueOrigination{OpaqueID: inst, Area: area, Interface: iface, Scope: scope, Body: body})
		desired[key] = body
		if prev, ok := e.riOrig.prev[key]; !ok || !bytes.Equal(prev, body) {
			e.ri.originations.With(e.afLabel(), scope.String()).Inc()
		}
	}
}

// riOnReceive is the RFC 5250 sec 3 reception hook for OSPFv2 RI opaque LSAs. RI drives no
// protocol behavior from received informational bits (RFC 7770 sec 2.4 informational-only);
// the LSA is stored and reflooded by the carrier and rendered by `show ospf database
// router-information`. This only counts the receive and refreshes the gauges.
func (e *engine) riOnReceive(r opaqueReceived) {
	if r.OpaqueType != packet.RIOpaqueType || r.Withdrawn {
		return
	}
	e.ri.received.With(e.afLabel()).Inc()
	e.refreshRIMetrics()
}

// riV3Scopes pairs each OSPFv3 RI wire type with its flooding scope, for counting/rendering
// the native RI LSAs (RFC 7770 sec 2.2).
var riV3Scopes = []struct {
	scope OpaqueScope
	typ   types.LSType
}{
	{OpaqueScopeLink, types.LSType(ospfv3types.LSTypeRouterInformationLink)},
	{OpaqueScopeArea, types.LSType(ospfv3types.LSTypeRouterInformationArea)},
	{OpaqueScopeAS, types.LSType(ospfv3types.LSTypeRouterInformationAS)},
}

// refreshRIMetrics recomputes the ze_ospf_ri_lsas population gauge (by scope) and the
// ze_ospf_ri_capability_bits gauge from current state, and (for OSPFv3, which has no opaque
// receive hook) counts newly observed peer RI LSAs into ze_ospf_ri_received_total. Cheap;
// called on each origination pass and on each OSPFv2 receive.
func (e *engine) refreshRIMetrics() {
	if e.lsdb == nil {
		return
	}
	e.mu.Lock()
	self := e.cfg.RouterID
	e.mu.Unlock()
	af := e.afLabel()

	counts := map[OpaqueScope]int{}
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		for _, sc := range riV3Scopes {
			for _, v := range e.lsdb.LSAViewsByType(sc.typ) {
				counts[sc.scope]++
				e.noteRIReceived(af, v.AdvertisingRouter, self, sc.scope, packet.OpaqueIDOf(v.LinkStateID)|uint32(v.LinkStateID[0])<<24)
			}
		}
	} else {
		for _, v := range e.lsdb.OpaqueLSAsByType(packet.RIOpaqueType) {
			counts[OpaqueScope(v.Scope)]++
		}
	}
	samples := make([]gaugeSample, 0, len(counts))
	for scope, n := range counts {
		samples = append(samples, gaugeSample{labels: []string{af, scope.String()}, value: float64(n)})
	}
	e.riLSAsGauge.apply(e.ri.lsas, samples)

	caps := e.deriveRICapabilities()
	e.riCapBitsGauge.apply(e.ri.capabilityBit, []gaugeSample{
		{labels: []string{riBitLabelGRCapable}, value: boolGauge(caps.GRCapable)},
		{labels: []string{riBitLabelGRHelper}, value: boolGauge(caps.GRHelper)},
		{labels: []string{riBitLabelStubRouter}, value: boolGauge(caps.StubRouter)},
		{labels: []string{riBitLabelTE}, value: boolGauge(caps.TE)},
	})
}

// noteRIReceived counts a peer's OSPFv3 RI LSA once, the first time this engine observes its
// (advertising-router, scope, instance) identity. OSPFv3 RI is a native LSA with no per-LSA
// receive hook, so this refresh-driven diff is how ze_ospf_ri_received_total{af=v3} advances;
// self-originated LSAs are excluded.
func (e *engine) noteRIReceived(af string, adv, self types.RouterID, scope OpaqueScope, instance uint32) {
	if adv == self {
		return
	}
	key := riOrigKey{scope: scope, area: types.AreaID(adv), inst: instance}
	e.riMu.Lock()
	_, seen := e.riSeen[key]
	if !seen {
		e.riSeen[key] = struct{}{}
	}
	e.riMu.Unlock()
	if !seen {
		e.ri.received.With(af).Inc()
	}
}

// boolGauge maps a capability boolean to a gauge value (1 set, 0 clear).
func boolGauge(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// attachedAreaIDs returns the distinct areas this router is attached to (an enrolled
// interface binds a declared area), in ascending order, for area-scoped RI origination.
func attachedAreaIDs(cfg ospfConfig) []types.AreaID {
	seen := map[types.AreaID]struct{}{}
	var out []types.AreaID
	for _, ic := range cfg.enrolledInterfaces() {
		if _, ok := seen[ic.AreaID]; ok {
			continue
		}
		seen[ic.AreaID] = struct{}{}
		out = append(out, ic.AreaID)
	}
	sort.Slice(out, func(i, j int) bool { return lessAreaID(out[i], out[j]) })
	return out
}

// attachedNSSAAreas returns the attached NSSA areas (RFC 7770 sec 2.7 SHOULD: an AS-scoped RI
// router also advertises area-scoped RI into attached NSSAs), in ascending order.
func attachedNSSAAreas(cfg ospfConfig) []types.AreaID {
	attached := map[types.AreaID]struct{}{}
	for _, ic := range cfg.enrolledInterfaces() {
		attached[ic.AreaID] = struct{}{}
	}
	var out []types.AreaID
	for _, a := range cfg.Areas {
		if a.AreaType != areaTypeNSSA {
			continue
		}
		if _, ok := attached[a.AreaID]; ok {
			out = append(out, a.AreaID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return lessAreaID(out[i], out[j]) })
	return out
}

// activeInterfaceNames returns the names of enrolled interfaces, in ascending order, for
// link-scoped RI origination (one RI LSA per interface).
func activeInterfaceNames(cfg ospfConfig) []string {
	enrolled := cfg.enrolledInterfaces()
	out := make([]string, 0, len(enrolled))
	for _, ic := range enrolled {
		out = append(out, ic.Name)
	}
	sort.Strings(out)
	return out
}

// interfaceArea returns the area an interface is bound to (for link-scoped RI, whose
// sequence bookkeeping still keys on the interface's area).
func (c ospfConfig) interfaceArea(name string) types.AreaID {
	for _, ic := range c.Interfaces {
		if ic.Name == name {
			return ic.AreaID
		}
	}
	return types.BackboneArea
}
