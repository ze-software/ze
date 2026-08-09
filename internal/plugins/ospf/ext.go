// Design: docs/architecture/ospf/ospf-ext-4-extended-link-prefix.md -- shared state and metrics for the
// RFC 7684 Extended Prefix (Opaque Type 7) and Extended Link (Opaque Type 8) consumers.
// RFC: rfc/short/rfc7684.md (Extended Prefix/Link Opaque LSAs), rfc/short/rfc5250.md sec 5
// (Type-11 originator reachability, consumed from ext-1).
//
// The two consumers register with the ext-1 opaque carrier (registerExtConsumers), read the
// router's advertised prefixes/links from the LSDB self-LSAs to associate the correct Route
// Type / Link Type-ID-Data, and own the ze_ospf_ext_* metric series. This file holds the
// pieces both consumers share: the metric surface, the origination withdraw-tracking state
// (extOriginator), the received-attribute resolver (extReceiver), and the prefix/mask helpers.

package ospf

import (
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// extMetrics is the RFC 7684 ze_ospf_ext_* series (spec-ospf-ext-4), owned by the Extended
// Prefix/Link consumers (distinct from the ext-1 ze_ospf_opaque_* carrier series).
type extMetrics struct {
	prefixLSAs   metrics.GaugeVec   // labels: scope (area/as)
	linkLSAs     metrics.Gauge      // always area scope
	originations metrics.CounterVec // labels: opaque_type (7/8)
	malformed    metrics.CounterVec // labels: opaque_type (7/8)
	subtlvErrors metrics.CounterVec // labels: registry (prefix/link)
}

func nopExtMetrics() extMetrics {
	nop := metrics.NopRegistry{}
	return extMetrics{
		prefixLSAs:   nop.GaugeVec("", "", nil),
		linkLSAs:     nop.Gauge("", ""),
		originations: nop.CounterVec("", "", nil),
		malformed:    nop.CounterVec("", "", nil),
		subtlvErrors: nop.CounterVec("", "", nil),
	}
}

// setExtMetrics registers the five ze_ospf_ext_* series on the engine's metric registry.
func (e *engine) setExtMetrics(reg metrics.Registry) {
	e.ext = extMetrics{
		prefixLSAs:   reg.GaugeVec("ze_ospf_ext_prefix_lsas", "Current OSPF Extended Prefix Opaque LSAs (RFC 7684 Opaque Type 7), by flooding scope.", []string{"scope"}),
		linkLSAs:     reg.Gauge("ze_ospf_ext_link_lsas", "Current OSPF Extended Link Opaque LSAs (RFC 7684 Opaque Type 8); always area-scoped."),
		originations: reg.CounterVec("ze_ospf_ext_originations_total", "Total OSPF Extended Prefix/Link Opaque LSAs originated, by opaque type.", []string{"opaque_type"}),
		malformed:    reg.CounterVec("ze_ospf_ext_malformed_total", "Total malformed OSPF Extended Prefix/Link Opaque LSA bodies rejected on receipt (RFC 7684 sec 5), by opaque type.", []string{"opaque_type"}),
		subtlvErrors: reg.CounterVec("ze_ospf_ext_subtlv_errors_total", "Total registered Extended Prefix/Link sub-TLV codec panics recovered, by registry.", []string{"registry"}),
	}
	// Fresh tracker for the newly bound gauge so a drained scope label is zeroed on the real
	// series rather than inheriting a stale zeroing from the nop registry.
	e.extPrefixLSAsGauge = newGaugeVecTracker()
}

// sub-TLV error metric registry labels (ze_ospf_ext_subtlv_errors_total{registry=...}).
const (
	extRegistryPrefix = "prefix"
	extRegistryLink   = "link"
)

// registerExtConsumers registers the Extended Prefix (Opaque Type 7) and Extended Link (Opaque
// Type 8) opaque consumers bound to this engine (spec-ospf-ext-4). Production calls it once for
// the IPv4 engine; tests call it after resetOpaqueConsumers. Type 7 registers with the area
// scope as its default; a per-origination Type 11 (AS) override selects AS scope for an
// AS-external prefix (RFC 7684 sec 2). Type 8 is area-scope only (RFC 7684 sec 3).
func registerExtConsumers(e *engine) error {
	// spec-ospf-ext-14: wire the Extended Prefix/Link body decoders into the debug detail
	// registry so `show ospf database opaque-area detail` renders them typed.
	registerOpaqueDetailDecoder(packet.ExtPrefixOpaqueType, "extended-prefix", func(b []byte) (any, error) {
		v, err := packet.DecodeExtPrefixLSA(b)
		return v, err
	})
	registerOpaqueDetailDecoder(packet.ExtLinkOpaqueType, "extended-link", func(b []byte) (any, error) {
		v, err := packet.DecodeExtLinkLSA(b)
		return v, err
	})
	if err := registerOpaqueConsumer(packet.ExtPrefixOpaqueType, OpaqueScopeArea, e.extPrefixOnOriginate, e.extPrefixOnReceive); err != nil {
		return err
	}
	return registerOpaqueConsumer(packet.ExtLinkOpaqueType, OpaqueScopeArea, e.extLinkOnOriginate, e.extLinkOnReceive)
}

// refreshExtMetrics recomputes the ze_ospf_ext_prefix_lsas (by scope) and ze_ospf_ext_link_lsas
// population gauges from the current LSDB. Cheap; called on each origination pass and receive.
func (e *engine) refreshExtMetrics() {
	if e.lsdb == nil {
		return
	}
	counts := map[OpaqueScope]int{}
	for _, v := range e.lsdb.OpaqueLSAsByType(packet.ExtPrefixOpaqueType) {
		counts[OpaqueScope(v.Scope)]++
	}
	samples := make([]gaugeSample, 0, len(counts))
	for scope, n := range counts {
		samples = append(samples, gaugeSample{labels: []string{scope.String()}, value: float64(n)})
	}
	e.extPrefixLSAsGauge.apply(e.ext.prefixLSAs, samples)
	e.ext.linkLSAs.Set(float64(len(e.lsdb.OpaqueLSAsByType(packet.ExtLinkOpaqueType))))
}

// extPrefixScope returns the flooding scope for an Extended Prefix TLV of the given Route Type
// (RFC 7684 sec 2.1: the LSA scope MUST satisfy every prefix). Intra/inter-area prefixes are
// area-scoped (LS Type 10); AS-external / NSSA-external prefixes are AS-scoped (LS Type 11).
func extPrefixScope(routeType uint8) OpaqueScope {
	switch routeType {
	case packet.ExtRouteTypeASExternal, packet.ExtRouteTypeNSSAExternal:
		return OpaqueScopeAS
	default:
		return OpaqueScopeArea
	}
}

// prefixKeyBytes packs an IPv4 prefix into a 5-byte key (4 address octets + prefix length) for
// stable Opaque-ID assignment and dedup.
func prefixKeyBytes(p netip.Prefix) [5]byte {
	addr := p.Addr().As4()
	return [5]byte{addr[0], addr[1], addr[2], addr[3], byte(p.Bits())}
}

// resolvedPrefixString renders a 5-byte prefix key (4 address octets + prefix length) as a
// CIDR string, for the resolved-attribute display.
func resolvedPrefixString(k [5]byte) string {
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{k[0], k[1], k[2], k[3]}), int(k[4])).String()
}

// prefixFromNetMask reconstructs an IPv4 prefix from a network address and a contiguous
// dotted-quad mask (the RFC 2328 Router-LSA stub-link / Summary-LSA encoding). ok is false for
// a non-contiguous mask.
func prefixFromNetMask(network, mask [4]byte) (netip.Prefix, bool) {
	bits, ok := extMaskPrefixLen(mask)
	if !ok {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(netip.AddrFrom4(network), bits).Masked(), true
}

// extMaskPrefixLen counts the leading one-bits of a contiguous IPv4 mask (mirrors spf.maskPrefixLen,
// which is unexported in that package). A non-contiguous mask returns ok=false.
func extMaskPrefixLen(mask [4]byte) (int, bool) {
	bits := 0
	seenZero := false
	for _, b := range mask {
		for bit := 7; bit >= 0; bit-- {
			if b&(1<<uint(bit)) != 0 {
				if seenZero {
					return 0, false
				}
				bits++
				continue
			}
			seenZero = true
		}
	}
	return bits, true
}

// extPrefixKey identifies one originated Extended Prefix Opaque LSA for stable Opaque-ID
// assignment and withdraw diffing: its flooding scope, target area, Route Type, and prefix.
type extPrefixKey struct {
	scope     OpaqueScope
	area      types.AreaID
	routeType uint8
	prefix    [5]byte
}

// extLinkKey identifies one originated Extended Link Opaque LSA: its area and the Router-LSA
// link identity it mirrors.
type extLinkKey struct {
	area     types.AreaID
	linkType uint8
	linkID   [4]byte
	linkData [4]byte
}

// extOrig records an originated instance's Opaque ID + target for a later withdraw.
type extOrig struct {
	id    uint32
	area  types.AreaID
	scope OpaqueScope
}

// extOriginator holds the Extended Prefix/Link origination state: a stable prefix/link ->
// Opaque-ID mapping (so an advertisement keeps its Opaque ID across passes, RFC 7684 sec 2/3:
// the Opaque ID has no semantics) and the previously-originated sets for withdraw diffing
// (AC-13). Opaque Type 7 and Type 8 have independent Opaque-ID namespaces.
type extOriginator struct {
	mu           sync.Mutex
	prefixIDs    map[extPrefixKey]uint32
	linkIDs      map[extLinkKey]uint32
	nextPrefixID uint32
	nextLinkID   uint32
	prevPrefix   map[extPrefixKey]extOrig
	prevLink     map[extLinkKey]extOrig
}

func newExtOriginator() *extOriginator {
	return &extOriginator{
		prefixIDs:  map[extPrefixKey]uint32{},
		linkIDs:    map[extLinkKey]uint32{},
		prevPrefix: map[extPrefixKey]extOrig{},
		prevLink:   map[extLinkKey]extOrig{},
	}
}

// prefixIDFor returns the stable Opaque ID for a prefix advertisement, allocating a fresh
// ascending one on first use (RFC 7684 sec 2.1 RECOMMENDED ascending Opaque IDs).
func (o *extOriginator) prefixIDFor(key extPrefixKey) uint32 {
	if id, ok := o.prefixIDs[key]; ok {
		return id
	}
	o.nextPrefixID++
	o.prefixIDs[key] = o.nextPrefixID
	return o.nextPrefixID
}

// linkIDFor returns the stable Opaque ID for a link advertisement.
func (o *extOriginator) linkIDFor(key extLinkKey) uint32 {
	if id, ok := o.linkIDs[key]; ok {
		return id
	}
	o.nextLinkID++
	o.linkIDs[key] = o.nextLinkID
	return o.nextLinkID
}

// extRecvKey keys a received Extended Prefix attribute set by advertising router and prefix,
// for the RFC 7684 sec 2 lowest-Opaque-ID cross-LSA dedup.
type extRecvKey struct {
	adv    types.RouterID
	prefix [5]byte
}

// extRecvEntry is the resolved attribute set for one (router, prefix): the winning Opaque ID
// and the normalized flags, plus RFC 5250 sec 5 usability (a Type-11 LSA from an unreachable
// originator is present-but-unusable).
type extRecvEntry struct {
	opaqueID  uint32
	routeType uint8
	flags     uint8
	scope     OpaqueScope
	usable    bool
}

// extReceiver resolves received Extended Prefix attributes across LSAs from the same router
// (RFC 7684 sec 2: the lowest Opaque ID wins) and records Type-11 reachability (RFC 5250 sec
// 5). This spec applies no attribute (empty containers), so the resolved set is exposed for
// `show` and for a downstream consumer to read; it never feeds SPF or the route table.
type extReceiver struct {
	mu       sync.Mutex
	prefixes map[extRecvKey]extRecvEntry
}

func newExtReceiver() *extReceiver { return &extReceiver{prefixes: map[extRecvKey]extRecvEntry{}} }

// applyPrefix records the attributes for one received Extended Prefix TLV, keeping the entry
// from the lowest Opaque ID when the same (router, prefix) appears across LSAs (RFC 7684 sec 2).
func (r *extReceiver) applyPrefix(adv types.RouterID, opaqueID uint32, routeType, flags uint8, prefix [5]byte, scope OpaqueScope, usable bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := extRecvKey{adv: adv, prefix: prefix}
	if cur, ok := r.prefixes[key]; ok && cur.opaqueID < opaqueID {
		return // an existing strictly-lower Opaque ID wins (RFC 7684 sec 2)
	}
	// A strictly-lower ID returns above; an equal ID (a refresh of the SAME LSA at a higher
	// sequence, delivered only on a newer install) falls through and overwrites so updated
	// flags / route type / usability are reflected; a lower incoming ID also overwrites.
	r.prefixes[key] = extRecvEntry{opaqueID: opaqueID, routeType: routeType, flags: flags, scope: scope, usable: usable}
}

// withdrawPrefixes removes every resolved entry contributed by (adv, opaqueID) when its LSA is
// MaxAge-purged (RFC 2328 sec 14).
func (r *extReceiver) withdrawPrefixes(adv types.RouterID, opaqueID uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, e := range r.prefixes {
		if k.adv == adv && e.opaqueID == opaqueID {
			delete(r.prefixes, k)
		}
	}
}

// lookupPrefix returns the resolved entry for (adv, prefix), for tests and `show`.
func (r *extReceiver) lookupPrefix(adv types.RouterID, prefix [5]byte) (extRecvEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.prefixes[extRecvKey{adv: adv, prefix: prefix}]
	return e, ok
}
