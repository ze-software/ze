// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing config resolve.
// The `segment-routing` container resolves into an sr.SRConfig for one address
// family; the IPv4 block sits at the ospf top level and the IPv6 block under
// `address-family ipv6`. SR config is NOT stored in ospfConfig (whose by-value
// copy budget is fixed): it is parsed from the raw config sections and applied to
// the process-global srWire keyed by Router ID, which the RI/Extended TLV builders
// consult. Validation runs in the config verify/configure hooks.
// RFC: rfc/short/rfc8665.md (§3 SRGB/SRLB/SRMS); rfc/short/rfc8666.md (§4 shared caps)

package ospf

import (
	"encoding/json"
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/ospf/sr"
)

// parseSegmentRouting resolves a `segment-routing` container into sr.SRConfig. It
// returns nil when nothing is configured so an empty block is inert (AC-20).
func parseSegmentRouting(m map[string]any) *sr.SRConfig {
	if len(m) == 0 {
		return nil
	}
	cfg := &sr.SRConfig{Enabled: configBool(m["enable"], false)}
	if g, ok := m["srgb"].(map[string]any); ok {
		if r, ok := parseLabelRange(g); ok {
			cfg.SRGB = append(cfg.SRGB, r)
		}
	}
	if b, ok := m["srlb"].(map[string]any); ok {
		if r, ok := parseLabelRange(b); ok {
			cfg.SRLB = append(cfg.SRLB, r)
		}
	}
	for _, entry := range keyedList(m["prefix-sid"], false) {
		if p, ok := parsePrefixSID(entry); ok {
			cfg.Prefixes = append(cfg.Prefixes, p)
		}
	}
	if v, ok := configUint8(m["srms-preference"]); ok {
		cfg.HasSRMS = true
		cfg.SRMSPreference = v
	}
	if !cfg.Enabled && len(cfg.SRGB) == 0 && len(cfg.SRLB) == 0 && len(cfg.Prefixes) == 0 {
		return nil
	}
	return cfg
}

// parseLabelRange resolves a `{ lower-bound; upper-bound }` container into a
// LabelRange (inclusive bounds; the FRR-style operator spelling).
func parseLabelRange(m map[string]any) (sr.LabelRange, bool) {
	lower, lok := configNumber(m["lower-bound"])
	upper, uok := configNumber(m["upper-bound"])
	if !lok || !uok || upper < lower {
		return sr.LabelRange{}, false
	}
	return sr.LabelRange{Base: uint32(lower), Size: uint32(upper-lower) + 1}, true
}

// parsePrefixSID resolves one `prefix-sid` list entry (keyed by prefix).
func parsePrefixSID(entry listEntry) (sr.PrefixSIDConfig, bool) {
	text := entry.key
	if s := configString(entry.data["prefix"]); s != "" {
		text = s
	}
	prefix, err := netip.ParsePrefix(text)
	if err != nil {
		return sr.PrefixSIDConfig{}, false
	}
	p := sr.PrefixSIDConfig{Prefix: prefix, NodeSID: configBool(entry.data["node-sid"], false)}
	if v, ok := configUint32(entry.data["index"]); ok {
		p.Index = v
	}
	p.NoPHP = configBool(entry.data["no-php"], false)
	p.ExplicitNull = configBool(entry.data["explicit-null"], false)
	return p, true
}

// extractSRConfigs pulls the IPv4 (top-level) and IPv6 (address-family) SR blocks
// from the raw config sections.
func extractSRConfigs(sections []configSection) (v4, v6 *sr.SRConfig) {
	for _, s := range sections {
		if s.Root != "ospf" || s.Data == "" {
			continue
		}
		var wrapper map[string]any
		if err := json.Unmarshal([]byte(s.Data), &wrapper); err != nil {
			continue
		}
		tree, _ := wrapper["ospf"].(map[string]any)
		if tree == nil {
			continue
		}
		if m, ok := tree["segment-routing"].(map[string]any); ok {
			if r := parseSegmentRouting(m); r != nil {
				v4 = r
			}
		}
		if af, ok := tree["address-family"].(map[string]any); ok {
			for _, name := range []string{"ipv6", "ipv6-unicast"} {
				sub, ok := af[name].(map[string]any)
				if !ok {
					continue
				}
				if m, ok := sub["segment-routing"].(map[string]any); ok {
					if r := parseSegmentRouting(m); r != nil {
						v6 = r
					}
				}
			}
		}
	}
	return v4, v6
}

// applySRConfig parses the SR blocks and populates srWire keyed by the router ID.
// The RI capability TLVs are shared across both address families (RFC 8666 §4), so
// the IPv4 block drives the effective/shared config, falling back to the IPv6 block
// when only that family enables SR. When BOTH families configure SR the IPv6 block is
// ALSO stored as a per-AF override, so the IPv6 origination advertises its own IPv6
// node Prefix-SIDs rather than the (IPv4-only) shared block's. Removing SR from the
// config clears the store entry (AC-20).
func applySRConfig(sections []configSection, cfg ospfConfig) {
	v4, v6 := extractSRConfigs(sections)
	eff := v4
	if eff == nil {
		eff = v6
	}
	if eff == nil {
		srWire.set(cfg.RouterID, sr.SRConfig{})
		srMetrics.Load().updateFromConfig("ipv4", sr.SRConfig{}, 0)
		srMetrics.Load().updateFromConfig("ipv6", sr.SRConfig{}, 0)
		return
	}
	srWire.set(cfg.RouterID, *eff)
	// When the shared/effective config is the IPv4 block (both families configured SR),
	// keep the IPv6 block as its own override so its IPv6 Prefix-SIDs still originate.
	// Otherwise the IPv6 family reads the shared block, so CLEAR any override left over
	// from a prior dual-stack config: removing the IPv6 SR block while IPv4 SR stays
	// enabled must not keep originating the withdrawn IPv6 node Prefix-SID (the enabled
	// path of srWireStore.set does not touch v6caps).
	if v4 != nil && v6 != nil {
		srWire.setV6(cfg.RouterID, *v6)
	} else {
		srWire.setV6(cfg.RouterID, sr.SRConfig{})
	}
	adjInUse := len(srWire.adjList(cfg.RouterID))
	if v4 != nil {
		srMetrics.Load().updateFromConfig("ipv4", *v4, adjInUse)
	}
	if v6 != nil {
		srMetrics.Load().updateFromConfig("ipv6", *v6, adjInUse)
	} else if v4 != nil {
		// The RI capabilities are shared, so the IPv6 family reflects the same enable
		// state when only IPv4 configured SR (RFC 8666 §4).
		srMetrics.Load().updateFromConfig("ipv6", *v4, adjInUse)
	}
}

// validateSRConfig validates the SR blocks in the config sections (SRGB/SRLB
// Range Size > 0, non-overlap, label bounds, prefix-SID index within the SRGB).
func validateSRConfig(sections []configSection) error {
	v4, v6 := extractSRConfigs(sections)
	for _, c := range []*sr.SRConfig{v4, v6} {
		if c == nil {
			continue
		}
		if err := c.Validate(nil); err != nil {
			return err
		}
	}
	return nil
}
