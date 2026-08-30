// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing metrics. The seven
// ze_ospf_sr_* series carry an `af` label (ipv4/ipv6) so the one OSPF engine reports
// both address families under one namespace (spec-ospf-ext-5). Owned by SR: removing
// the SR code removes these series.
// RFC: rfc/short/rfc8665.md; rfc/short/rfc8666.md

package ospf

import (
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
)

// srMetricsSet holds the SR metric series. They are process-global (af-labeled),
// registered once regardless of how many engine instances run.
type srMetricsSet struct {
	enabled         metrics.GaugeVec   // af
	prefixSIDs      metrics.GaugeVec   // af, direction
	adjSIDs         metrics.GaugeVec   // af
	labelsInstalled metrics.GaugeVec   // af, op
	computeErrors   metrics.CounterVec // af, reason
	srlbInUse       metrics.GaugeVec   // af
	malformedTLVs   metrics.CounterVec // af, tlv
}

func newSRMetrics(reg metrics.Registry) *srMetricsSet {
	if reg == nil {
		reg = metrics.NopRegistry{}
	}
	return &srMetricsSet{
		enabled: reg.GaugeVec("ze_ospf_sr_enabled",
			"Whether OSPF Segment Routing is enabled, by address family (RFC 8665/8666).",
			[]string{"af"}),
		prefixSIDs: reg.GaugeVec("ze_ospf_sr_prefix_sids",
			"OSPF SR Prefix-SIDs, by address family and direction (originated/received).",
			[]string{"af", labelDirection}),
		adjSIDs: reg.GaugeVec("ze_ospf_sr_adj_sids",
			"OSPF SR Adjacency-SIDs advertised by this node, by address family.",
			[]string{"af"}),
		labelsInstalled: reg.GaugeVec("ze_ospf_sr_labels_installed",
			"OSPF SR MPLS forwarding entries installed, by address family and operation (push/swap/pop).",
			[]string{"af", "op"}),
		computeErrors: reg.CounterVec("ze_ospf_sr_label_compute_errors_total",
			"OSPF SR label-computation errors, by address family and reason (index-out-of-range/unknown-algorithm/duplicate/bad-vl).",
			[]string{"af", labelReason}),
		srlbInUse: reg.GaugeVec("ze_ospf_sr_srlb_labels_in_use",
			"OSPF SR local-block (SRLB) labels currently allocated, by address family.",
			[]string{"af"}),
		malformedTLVs: reg.CounterVec("ze_ospf_sr_malformed_tlvs_total",
			"OSPF SR malformed TLV/sub-TLV receptions, by address family and TLV kind.",
			[]string{"af", "tlv"}),
	}
}

// updateFromConfig sets the config-derived gauges for one address family: enabled,
// the count of originated Prefix-SIDs, and the SRLB labels in use.
func (m *srMetricsSet) updateFromConfig(af string, cfg sr.SRConfig, adjInUse int) {
	if cfg.Enabled {
		m.enabled.With(af).Set(1)
	} else {
		m.enabled.With(af).Set(0)
	}
	m.prefixSIDs.With(af, "originated").Set(float64(len(cfg.Prefixes)))
	m.adjSIDs.With(af).Set(float64(adjInUse))
	m.srlbInUse.With(af).Set(float64(adjInUse))
}

// observeMalformed counts a malformed SR TLV reception. af varies by address
// family; the OSPFv3 (ipv6) reception path passes "ipv6" once the v3 Extended-LSA
// carriage is wired.
//
//nolint:unparam // af is "ipv4" until the v3 reception path calls with "ipv6".
func (m *srMetricsSet) observeMalformed(af, tlv string)       { m.malformedTLVs.With(af, tlv).Inc() }
func (m *srMetricsSet) observeComputeError(af, reason string) { m.computeErrors.With(af, reason).Inc() }
func (m *srMetricsSet) observeLabelsInstalled(af, op string, n int) {
	m.labelsInstalled.With(af, op).Set(float64(n))
}

var (
	srMetricsOnce sync.Once
	// srMetrics is an atomic pointer so a leaked SPF Computer goroutine reading it
	// races cleanly with a test that swaps it via Store (go test -race clean). It is
	// seeded with a NopRegistry set so reads before setSRMetrics never deref nil.
	srMetrics = func() *atomic.Pointer[srMetricsSet] {
		p := &atomic.Pointer[srMetricsSet]{}
		p.Store(newSRMetrics(metrics.NopRegistry{}))
		return p
	}()
)

// setSRMetrics binds the SR metric series to the real registry once (the series
// are process-global and bound a single time).
func setSRMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	srMetricsOnce.Do(func() { srMetrics.Store(newSRMetrics(reg)) })
}
