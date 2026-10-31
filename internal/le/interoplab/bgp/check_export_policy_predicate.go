// Design: docs/architecture/testing/interop.md -- the IGP export-policy scenarios.
// Related: check_export_policy.go -- the checker bodies that query FRR and call these.
// Related: check_rib_candidates_predicate.go -- frrISISRouteVerdict, the import-side reading of the same JSON.
package bgp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
)

// frrISISExportWant is one prefix FRR must install through IS-IS: its next hop
// is Ze and its metric is FRR's link metric to Ze plus the metric Ze advertised.
type frrISISExportWant struct {
	prefix  netip.Prefix
	nextHop netip.Addr
	metric  uint32
}

// frrNextHop is one next hop of an FRR route in `show ip route ... json`.
type frrNextHop struct {
	IP string `json:"ip"`
}

// frrMetricRouteEntry is one route of FRR's `show ip route ... json` with the
// metric zebra holds for it, which for IS-IS is the SPF cost.
type frrMetricRouteEntry struct {
	Protocol string       `json:"protocol"`
	Selected bool         `json:"selected"`
	Metric   *uint32      `json:"metric"`
	NextHops []frrNextHop `json:"nexthops"`
}

// isisExportPolicyVerdict decides whether output, FRR's `show ip route isis
// json`, holds every wanted prefix as a selected IS-IS route through its next
// hop at its metric, and holds no route for refused. The absence is read in
// the same answer as the presences, so a dead adjacency or an empty table
// cannot pass it.
func isisExportPolicyVerdict(output string, wants []frrISISExportWant, refused netip.Prefix) error {
	var routes map[string][]frrMetricRouteEntry
	if err := json.Unmarshal([]byte(output), &routes); err != nil {
		return fmt.Errorf("decode FRR's show ip route isis answer: %w", err)
	}
	for index := range wants {
		if err := frrISISExportRouteVerdict(routes[wants[index].prefix.String()], &wants[index]); err != nil {
			return err
		}
	}
	if _, held := routes[refused.String()]; held {
		return fmt.Errorf("FRR holds an IS-IS route for %s, which Ze's export chain rejects: %s", refused, output)
	}
	return nil
}

// frrISISExportRouteVerdict decides whether entries, FRR's routes for one
// prefix, hold a selected IS-IS route through want's next hop at want's metric.
func frrISISExportRouteVerdict(entries []frrMetricRouteEntry, want *frrISISExportWant) error {
	if len(entries) == 0 {
		return fmt.Errorf("FRR holds no IS-IS route for %s", want.prefix)
	}
	for i := range entries {
		if entries[i].Protocol != sourceISIS {
			continue
		}
		if !entries[i].Selected {
			continue
		}
		if !frrNextHopsHold(entries[i].NextHops, want.nextHop) {
			continue
		}
		if entries[i].Metric == nil {
			return fmt.Errorf("FRR's IS-IS route for %s carries no metric", want.prefix)
		}
		if *entries[i].Metric != want.metric {
			return fmt.Errorf("FRR's IS-IS route for %s has metric %d, want %d", want.prefix, *entries[i].Metric, want.metric)
		}
		return nil
	}
	return fmt.Errorf("FRR holds no selected IS-IS route for %s through %s", want.prefix, want.nextHop)
}

// frrNextHopsHold reports whether hops names address.
func frrNextHopsHold(hops []frrNextHop, address netip.Addr) bool {
	for _, hop := range hops {
		if hop.IP == address.String() {
			return true
		}
	}
	return false
}

// ospfExternalMetricType is the metric type FRR prints for an AS-External-LSA.
type ospfExternalMetricType string

const (
	// ospfExternalE1 is metric type 1, the E bit clear (RFC 2328 Appendix A.4.5).
	ospfExternalE1 ospfExternalMetricType = "E1"
	// ospfExternalE2 is metric type 2, the E bit set.
	ospfExternalE2 ospfExternalMetricType = "E2"
)

// ospfExternalWant is one AS-External-LSA FRR must hold from Ze.
type ospfExternalWant struct {
	prefix     netip.Prefix
	metric     int64
	metricType ospfExternalMetricType
	tag        int64
}

// ospfExternalLSA is the part of one AS-External-LSA in FRR's `show ip ospf
// database external json` that the export scenario judges. FRR's
// show_as_external_lsa_detail (ospfd/ospf_vty.c) writes these keys beside the
// LSA header's linkStateId and advertisingRouter.
type ospfExternalLSA struct {
	ID         string `json:"linkStateId"`
	Router     string `json:"advertisingRouter"`
	MaskLength *int   `json:"networkMask"`
	MetricType string `json:"metricType"`
	Metric     *int64 `json:"metric"`
	Tag        *int64 `json:"externalRouteTag"`
}

// ospfExternalNodesMax bounds the walk over FRR's answer. The lab holds a
// handful of LSAs; an answer past this is not the database the scenario built.
const ospfExternalNodesMax = 4096

// ospfExportPolicyVerdict decides whether output, FRR's `show ip ospf database
// external json`, holds exactly one AS-External-LSA from router for each want,
// with its mask, metric, metric type and tag, and none for refused. The absence
// is read in the same answer as the presences.
func ospfExportPolicyVerdict(output string, router netip.Addr, wants []ospfExternalWant, refused netip.Prefix) error {
	lsas, err := ospfExternalLSAs(output)
	if err != nil {
		return err
	}
	for index := range wants {
		if err := ospfExternalLSAVerdict(lsas, router, &wants[index]); err != nil {
			return err
		}
	}
	for index := range lsas {
		if lsas[index].ID == refused.Addr().String() {
			return fmt.Errorf("FRR holds an AS-External-LSA for %s, which Ze's export chain rejects: %s", refused, output)
		}
	}
	return nil
}

// ospfExternalLSAVerdict decides whether lsas hold exactly one LSA for want's
// prefix from router, carrying want's mask, metric, metric type and tag.
func ospfExternalLSAVerdict(lsas []ospfExternalLSA, router netip.Addr, want *ospfExternalWant) error {
	var found *ospfExternalLSA
	for index := range lsas {
		if lsas[index].ID != want.prefix.Addr().String() {
			continue
		}
		if lsas[index].Router != router.String() {
			continue
		}
		if found != nil {
			return fmt.Errorf("FRR holds two AS-External-LSAs for %s from %s", want.prefix, router)
		}
		found = &lsas[index]
	}
	if found == nil {
		return fmt.Errorf("FRR holds no AS-External-LSA for %s from %s", want.prefix, router)
	}
	if found.MaskLength == nil {
		return fmt.Errorf("FRR's AS-External-LSA for %s carries no networkMask", want.prefix)
	}
	if *found.MaskLength != want.prefix.Bits() {
		return fmt.Errorf("FRR's AS-External-LSA for %s has mask /%d", want.prefix, *found.MaskLength)
	}
	if found.Metric == nil {
		return fmt.Errorf("FRR's AS-External-LSA for %s carries no metric", want.prefix)
	}
	if *found.Metric != want.metric {
		return fmt.Errorf("FRR's AS-External-LSA for %s has metric %d, want %d", want.prefix, *found.Metric, want.metric)
	}
	if ospfExternalMetricType(found.MetricType) != want.metricType {
		return fmt.Errorf("FRR's AS-External-LSA for %s has metric type %q, want %s", want.prefix, found.MetricType, want.metricType)
	}
	if found.Tag == nil {
		return fmt.Errorf("FRR's AS-External-LSA for %s carries no externalRouteTag", want.prefix)
	}
	if *found.Tag != want.tag {
		return fmt.Errorf("FRR's AS-External-LSA for %s has tag %d, want %d", want.prefix, *found.Tag, want.tag)
	}
	return nil
}

// ospfExternalLSAs collects every object in output that carries a
// linkStateId, wherever FRR nests it: by area, by LSA type, or keyed by the
// LSA's own ID. The walk is iterative and bounded by ospfExternalNodesMax, so
// the shape of the answer cannot make it recurse or run long.
func ospfExternalLSAs(output string) ([]ospfExternalLSA, error) {
	var root any
	if err := json.Unmarshal([]byte(output), &root); err != nil {
		return nil, fmt.Errorf("decode FRR's show ip ospf database external answer: %w", err)
	}
	var lsas []ospfExternalLSA
	stack := []any{root}
	for visited := 0; len(stack) > 0; visited++ {
		if visited >= ospfExternalNodesMax {
			return nil, fmt.Errorf("FRR's show ip ospf database external answer holds more than %d JSON values", ospfExternalNodesMax)
		}
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch value := node.(type) {
		case map[string]any:
			if _, isLSA := value["linkStateId"]; !isLSA {
				for _, child := range value {
					stack = append(stack, child)
				}
				continue
			}
			lsa, err := decodeOSPFExternalLSA(value)
			if err != nil {
				return nil, err
			}
			lsas = append(lsas, lsa)
		case []any:
			stack = append(stack, value...)
		}
	}
	if len(lsas) == 0 {
		return nil, errors.New("FRR's show ip ospf database external answer holds no LSA: " + output)
	}
	return lsas, nil
}

// decodeOSPFExternalLSA re-decodes one LSA object into its typed form.
func decodeOSPFExternalLSA(object map[string]any) (ospfExternalLSA, error) {
	raw, err := json.Marshal(object)
	if err != nil {
		return ospfExternalLSA{}, fmt.Errorf("re-encode an FRR LSA object: %w", err)
	}
	var lsa ospfExternalLSA
	if err := json.Unmarshal(raw, &lsa); err != nil {
		return ospfExternalLSA{}, fmt.Errorf("decode an FRR AS-External-LSA: %w", err)
	}
	return lsa, nil
}
