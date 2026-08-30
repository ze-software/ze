// Design: docs/architecture/ospf/ospf-ext-2-traffic-engineering.md -- the TE opaque consumer.
// RFC: rfc/short/rfc3630.md (TE LSA), rfc/short/rfc5392.md (inter-AS TE), rfc/short/rfc5250.md
// sec 5 (Type-11 reachability).
//
// This is the ext-2 consumer of the ext-1 opaque carrier. It registers Opaque type 1
// (RFC 3630 TE) and type 6 (RFC 5392 inter-AS TE), parses received TE LSA bodies into the
// TED (te_ted.go), builds originations from config (te_originate.go), and owns the
// ze_ospf_te_* metric series. The carrier owns flooding, sequencing, the LS-ID split, and
// the reachability determination; this file interprets only the TE body. Removing this
// file's registration removes all TE behavior, leaving the carrier and base OSPF intact.

package ospf

import (
	"errors"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
)

// TE kind labels for the ze_ospf_te_* metric series.
const (
	teKindRouterAddress = "router-address"
	teKindLink          = "link"
	teKindInterAS       = "inter-as"
)

// errTELinkSpecViolation marks a received Link TLV that breaks a mandatory-sub-TLV or
// prohibited-sub-TLV rule (RFC 3630 sec 2.4.2 / RFC 5392 sec 3.2.1). Such an LSA is still
// stored and reflooded verbatim by the carrier, but no TED entry is created (AC-18).
var errTELinkSpecViolation = errors.New("ospf: TE Link TLV violates a mandatory/prohibited sub-TLV rule")

// teMetrics is the ze_ospf_te_* series, owned by this consumer (distinct from the ext-1
// ze_ospf_opaque_* carrier series). They count the parsed TE topology, not raw opaque LSAs.
type teMetrics struct {
	lsas         metrics.GaugeVec   // labels: scope, kind
	databaseLnks metrics.GaugeVec   // labels: area
	originations metrics.CounterVec // labels: kind
	received     metrics.CounterVec // labels: kind, usable
	parseErrors  metrics.CounterVec // labels: opaque_type
	unreachable  metrics.Gauge
}

func nopTEMetrics() teMetrics {
	nop := metrics.NopRegistry{}
	return teMetrics{
		lsas:         nop.GaugeVec("", "", nil),
		databaseLnks: nop.GaugeVec("", "", nil),
		originations: nop.CounterVec("", "", nil),
		received:     nop.CounterVec("", "", nil),
		parseErrors:  nop.CounterVec("", "", nil),
		unreachable:  nop.Gauge("", ""),
	}
}

// setTEMetrics registers the six ze_ospf_te_* series on the engine's metric registry.
func (e *engine) setTEMetrics(reg metrics.Registry) {
	e.te = teMetrics{
		lsas:         reg.GaugeVec("ze_ospf_te_lsas", "Current OSPF Traffic Engineering LSAs in the TED, by flooding scope and kind.", []string{labelScope, labelKind}),
		databaseLnks: reg.GaugeVec("ze_ospf_te_database_links", "Current OSPF Traffic Engineering Database link entries, by area.", []string{labelArea}),
		originations: reg.CounterVec("ze_ospf_te_originations_total", "Total OSPF Traffic Engineering LSAs originated, by kind.", []string{labelKind}),
		received:     reg.CounterVec("ze_ospf_te_received_total", "Total OSPF Traffic Engineering LSAs parsed into the TED, by kind and whether usable.", []string{labelKind, "usable"}),
		parseErrors:  reg.CounterVec("ze_ospf_te_parse_errors_total", "Total malformed OSPF Traffic Engineering LSA bodies skipped, by opaque type.", []string{labelOpaqueType}),
		unreachable:  reg.Gauge("ze_ospf_te_unreachable_originators", "Current OSPF Type-11 inter-AS TE entries held unusable because their originator is unreachable (RFC 5250 sec 5)."),
	}
	// Fresh trackers for the newly bound gauges so a drained scope/kind or area label set is
	// zeroed on the real series rather than inheriting a stale zeroing from the nop registry.
	e.teLSAsGauge = newGaugeVecTracker()
	e.teDBLinksGauge = newGaugeVecTracker()
}

// registerTEConsumer registers the TE opaque consumers (Opaque type 1 and type 6) bound to
// this engine. Production calls it once for the IPv4 engine; tests call it after
// resetOpaqueConsumers. Type 6 registers with the area scope as its default; a per-link
// Type 10 vs Type 11 choice overrides it per origination (RFC 5392 sec 3.1.1).
func registerTEConsumer(e *engine) error {
	// spec-ospf-ext-14: wire the TE opaque body decoder into the debug detail registry so
	// `show ospf database opaque-area detail` renders TE LSAs as typed sub-TLVs. Registered
	// here (the TE consumer's own file) so removing TE removes its detail view too.
	registerOpaqueDetailDecoder(packet.TEOpaqueType, "traffic-engineering", func(b []byte) (any, error) {
		v, err := packet.DecodeTELSA(b)
		return v, err
	})
	registerOpaqueDetailDecoder(packet.InterAsTEOpaqueType, "traffic-engineering-inter-as", func(b []byte) (any, error) {
		v, err := packet.DecodeTELSA(b)
		return v, err
	})
	if err := registerOpaqueConsumer(packet.TEOpaqueType, OpaqueScopeArea, e.teOriginateType1, e.teOnReceive); err != nil {
		return err
	}
	return registerOpaqueConsumer(packet.InterAsTEOpaqueType, OpaqueScopeArea, e.teOriginateType6, e.teOnReceive)
}

// teOnReceive is the RFC 5250 sec 3 reception hook for both TE opaque types. It parses the
// body into the TED (upsert), removes the entry on a withdraw, applies the RFC 5250 sec 5
// reachability flag, and never triggers SPF. A malformed body or a spec-violating Link TLV
// is counted and skipped without panicking (AC-18); the carrier still stores/refloods it.
func (e *engine) teOnReceive(r opaqueReceived) {
	if e.ted == nil {
		return
	}
	if r.OpaqueType != packet.TEOpaqueType && r.OpaqueType != packet.InterAsTEOpaqueType {
		return
	}
	if r.Withdrawn {
		e.ted.withdraw(r.AdvertisingRouter, r.OpaqueType, r.OpaqueID)
		e.refreshTEMetrics()
		return
	}
	lsa, err := packet.DecodeTELSA(r.Body)
	if err != nil {
		e.te.parseErrors.With(opaqueTypeLabel(r.OpaqueType)).Inc()
		return
	}
	// RFC 5392 has no Router Address TLV for Opaque type 6; only the Link TLV is meaningful.
	if r.OpaqueType == packet.InterAsTEOpaqueType && lsa.IsRouterAddress {
		return
	}
	if lsa.IsLink {
		if err := validateReceivedTELink(r.OpaqueType, lsa.Link); err != nil {
			e.te.parseErrors.With(opaqueTypeLabel(r.OpaqueType)).Inc()
			return
		}
	}
	e.ted.applyLSA(r.AdvertisingRouter, r.Area, r.Scope, r.OpaqueType, r.OpaqueID, lsa, r.Reachable)
	e.te.received.With(teReceiveKind(r.OpaqueType, lsa), teUsableLabel(r.Scope, r.Reachable)).Inc()
	e.refreshTEMetrics()
}

// validateReceivedTELink enforces the mandatory/prohibited sub-TLV rules for a received
// Link TLV. RFC 3630 sec 2.4.2: Link Type and Link ID are mandatory in a type-1 TE LSA.
// RFC 5392 sec 3.2.1/3.3.1: a type-6 inter-AS Link TLV MUST carry the Remote AS Number and
// MUST NOT carry the Link ID sub-TLV.
func validateReceivedTELink(opaqueType uint8, l packet.TELink) error {
	if !l.HasLinkType {
		return errTELinkSpecViolation
	}
	switch opaqueType {
	case packet.TEOpaqueType:
		if !l.HasLinkID {
			return errTELinkSpecViolation
		}
	case packet.InterAsTEOpaqueType:
		if l.HasLinkID || !l.HasRemoteAS {
			return errTELinkSpecViolation
		}
	default:
		return errTELinkSpecViolation
	}
	return nil
}

// teReceiveKind classifies a received TE LSA for the metric label.
func teReceiveKind(opaqueType uint8, lsa packet.TELSA) string {
	switch {
	case lsa.IsRouterAddress:
		return teKindRouterAddress
	case opaqueType == packet.InterAsTEOpaqueType:
		return teKindInterAS
	default:
		return teKindLink
	}
}

// teUsableLabel reports the usable label for a received entry (RFC 5250 sec 5: area-scope
// always usable; AS-scope usable only when the originator is reachable). It reuses the
// carrier's bool->label helper so the "true"/"false" literals live in one place.
func teUsableLabel(scope OpaqueScope, reachable bool) string {
	return registeredLabel(scope != OpaqueScopeAS || reachable)
}

// refreshTEMetrics recomputes the gauge series from the current TED. Cheap and called on
// each reception and each origination pass.
func (e *engine) refreshTEMetrics() {
	if e.ted == nil {
		return
	}
	linkCounts := e.ted.linkCountByArea()
	dbSamples := make([]gaugeSample, 0, len(linkCounts))
	for area, n := range linkCounts {
		dbSamples = append(dbSamples, gaugeSample{labels: []string{area.String()}, value: float64(n)})
	}
	e.teDBLinksGauge.apply(e.te.databaseLnks, dbSamples)
	e.te.unreachable.Set(float64(e.ted.unreachableCount()))
	snap := e.ted.Snapshot()
	// Count the parsed TE topology by scope + kind for the ze_ospf_te_lsas gauge.
	counts := make(map[[2]string]int, len(snap.Links)+1)
	if len(snap.RouterAddresses) > 0 {
		counts[[2]string{OpaqueScopeArea.String(), teKindRouterAddress}] += len(snap.RouterAddresses)
	}
	for i := range snap.Links {
		l := &snap.Links[i]
		kind := teKindLink
		if l.Link.IsInterAS() {
			kind = teKindInterAS
		}
		counts[[2]string{l.Scope.String(), kind}]++
	}
	lsaSamples := make([]gaugeSample, 0, len(counts))
	for k, n := range counts {
		lsaSamples = append(lsaSamples, gaugeSample{labels: []string{k[0], k[1]}, value: float64(n)})
	}
	e.teLSAsGauge.apply(e.te.lsas, lsaSamples)
}
