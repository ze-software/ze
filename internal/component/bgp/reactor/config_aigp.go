// Design: docs/architecture/route-selection.md
// RFC: rfc/short/rfc7311.md, Sections 3.3 and 3.4.1.
package reactor

import (
	"fmt"
	"strconv"

	"github.com/ze-software/ze/internal/core/configvalue"
)

func applyAIGPSettings(ps *PeerSettings, name string, session map[string]any) error {
	cfg, ok := mapMap(session, "aigp")
	if !ok {
		return nil
	}
	if raw, present := cfg["enabled"]; present {
		value, valid := mapBool(cfg, "enabled")
		if !valid {
			return fmt.Errorf("peer %s: invalid aigp enabled value %v", name, raw)
		}
		ps.AIGPSession = &value
	}
	if raw, present := cfg["originate"]; present {
		value, valid := mapBool(cfg, "originate")
		if !valid {
			return fmt.Errorf("peer %s: invalid aigp originate value %v", name, raw)
		}
		ps.AIGPOriginate = value
	}
	if raw, present := cfg["link-metric"]; present {
		value, err := strconv.ParseUint(fmt.Sprint(raw), 10, 64)
		if err != nil {
			return fmt.Errorf("peer %s: invalid aigp link-metric: %w", name, err)
		}
		if value == 0 {
			return fmt.Errorf("peer %s: aigp link-metric must be non-zero", name)
		}
		ps.AIGPLinkMetric = value
	}
	for _, text := range configvalue.LeafList(cfg["domain-as"]) {
		value, err := strconv.ParseUint(text, 10, 32)
		if err != nil {
			return fmt.Errorf("peer %s: invalid aigp domain-as: %w", name, err)
		}
		if value == 0 {
			return fmt.Errorf("peer %s: aigp domain-as cannot be zero", name)
		}
		ps.AIGPDomainAS = append(ps.AIGPDomainAS, uint32(value))
	}
	return nil
}

// AIGPEnabled applies the per-session override or the RFC 7311 session default.
func (s *PeerSettings) AIGPEnabled() bool {
	if s.AIGPSession != nil {
		return *s.AIGPSession
	}
	// RFC 7311 Section 3.3: "For all other External BGP (EBGP) sessions,
	// the default value of AIGP_SESSION MUST be 'disabled'."
	return s.IsIBGP()
}
