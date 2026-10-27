// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- config parsing

package copp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/firewall"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const defaultPort = 179

// parseCoppSections parses the control-plane-protection body coppData and
// attaches the BGP neighbor sources read from the `bgp` section of the same
// delivery. The engine delivers the bgp root because copp lists it under
// ConfigReads: at start, and whole beside control-plane-protection on every
// reload that changes either root (reloadConfigSections,
// internal/component/plugin/server/reload_compensation.go).
//
// A delivery with no bgp section names no neighbor: the configuration holds
// no bgp root. A bgp section that does not parse refuses the policy, because
// a set built from part of it would drop a configured neighbor's SYN.
//
// RFC 7454 Section 4: the ACL discards traffic to port 179 "sourced from an
// address not known or permitted to become a BGP neighbor".
func parseCoppSections(sections []sdk.ConfigSection, coppData string) (coppPolicy, bool, error) {
	policy, found, err := parseCoppConfig(coppData)
	if err != nil {
		return coppPolicy{}, false, err
	}
	if !found {
		return coppPolicy{}, false, nil
	}
	for _, section := range sections {
		if section.Root != bgpConfigRoot {
			continue
		}
		neighbors, err := parseNeighborSources(section.Data)
		if err != nil {
			return coppPolicy{}, false, err
		}
		policy.Neighbors = neighbors
	}
	return policy, true, nil
}

// parseNeighborSources reads the neighbor sources from one bgp section body,
// in the wrapped shape the SDK delivers ({"bgp": {...}}). A body without the
// bgp key is the root removed, delivered as {}, and names no neighbor.
func parseNeighborSources(bgpData string) ([]netip.Prefix, error) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(bgpData), &tree); err != nil {
		return nil, fmt.Errorf("copp: unmarshal bgp config: %w", err)
	}
	body, ok := tree[bgpConfigRoot].(map[string]any)
	// refuse-by-default: guards parseCoppSections -- a removed bgp root arrives as {} and configures no neighbor
	if !ok {
		return nil, nil
	}
	neighbors, err := config.BGPNeighborSources(body)
	if err != nil {
		return nil, fmt.Errorf("copp: bgp neighbors: %w", err)
	}
	return neighbors, nil
}

func parseCoppConfig(jsonData string) (coppPolicy, bool, error) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(jsonData), &tree); err != nil {
		return coppPolicy{}, false, fmt.Errorf("unmarshal copp config: %w", err)
	}

	cppTree, ok := tree["control-plane-protection"].(map[string]any)
	if !ok {
		return coppPolicy{}, false, nil
	}

	bgpTree, ok := cppTree["bgp"].(map[string]any)
	if !ok {
		return coppPolicy{}, false, nil
	}

	var policy coppPolicy
	rateLimit, limited, err := parseRateLimit(bgpTree)
	if err != nil {
		return coppPolicy{}, false, err
	}
	if limited {
		policy.RateLimit = &rateLimit
	}

	ports, err := parsePorts(bgpTree)
	if err != nil {
		return coppPolicy{}, false, err
	}
	policy.ProtectedPorts = ports
	if len(policy.ProtectedPorts) == 0 {
		policy.ProtectedPorts = []uint16{defaultPort}
	}

	trusted, err := parseTrustedSources(bgpTree)
	if err != nil {
		return coppPolicy{}, false, err
	}
	policy.TrustedSources = trusted

	return policy, true, nil
}

// parseRateLimit reads the `rate`, `burst` and `over-limit-policy` leaves. A
// container with no `rate` answers nil: the operator asked for the ACL alone.
//
// RFC 7454 Section 4: "In addition to strict filtering, rate-limiting MAY be
// configured for accepted BGP traffic."
//
// `over-limit-policy` defaults to drop. A limit whose over-limit packets are
// accepted stops nothing, so the setting that limits is the one an operator
// gets without writing it; `accept` stays available to observe load first.
//
// limited is false when the container holds no `rate`; the caller then
// builds the ACL with no limiter term.
func parseRateLimit(bgpTree map[string]any) (rateLimit coppRateLimit, limited bool, err error) {
	overPolicy := overPolicyDrop
	if op, ok := bgpTree["over-limit-policy"].(string); ok {
		switch op {
		case overPolicyDrop, overPolicyAccept:
			overPolicy = op
		default:
			return coppRateLimit{}, false, fmt.Errorf("copp: over-limit-policy: invalid value %q (want drop|accept)", op)
		}
	}

	burstStr, hasBurst := bgpTree["burst"].(string)
	rateStr, hasRate := bgpTree["rate"].(string)
	if !hasRate {
		if hasBurst {
			return coppRateLimit{}, false, errBurstWithoutRate
		}
		return coppRateLimit{}, false, nil
	}

	lim, err := firewall.ParseRateSpec(rateStr)
	if err != nil {
		return coppRateLimit{}, false, fmt.Errorf("copp: rate: %w", err)
	}
	rateLimit = coppRateLimit{
		Rate:       lim.Rate,
		RateUnit:   lim.Unit,
		Dimension:  lim.Dimension,
		OverPolicy: overPolicy,
	}
	if hasBurst {
		b, err := strconv.ParseUint(burstStr, 10, 32)
		if err != nil {
			return coppRateLimit{}, false, fmt.Errorf("copp: burst: invalid value %q: %w", burstStr, err)
		}
		rateLimit.Burst = uint32(b)
	}
	return rateLimit, true, nil
}

var errBurstWithoutRate = errors.New("copp: control-plane-protection bgp sets burst but no rate; " +
	"a burst sizes the rate limit, so it needs one. " +
	"Set the rate, or delete the burst: set control-plane-protection bgp rate 100/second")

func parsePorts(bgpTree map[string]any) ([]uint16, error) {
	var ports []uint16

	parseOne := func(v string) error {
		p, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return fmt.Errorf("copp: protected-port: invalid value %q: %w", v, err)
		}
		if err := firewall.ValidatePort(uint16(p)); err != nil {
			return fmt.Errorf("copp: protected-port: %w", err)
		}
		ports = append(ports, uint16(p))
		return nil
	}

	if v, ok := bgpTree["protected-port"].(string); ok {
		if err := parseOne(v); err != nil {
			return nil, err
		}
	}
	if list, ok := bgpTree["protected-port"].([]any); ok {
		for _, item := range list {
			if s, ok := item.(string); ok {
				if err := parseOne(s); err != nil {
					return nil, err
				}
			}
		}
	}

	return ports, nil
}

func parseTrustedSources(bgpTree map[string]any) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix

	parseOne := func(s string) error {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return fmt.Errorf("copp: trusted-source: invalid prefix %q: %w", s, err)
		}
		prefixes = append(prefixes, p)
		return nil
	}

	if v, ok := bgpTree["trusted-source"].(string); ok {
		if err := parseOne(v); err != nil {
			return nil, err
		}
	}
	if list, ok := bgpTree["trusted-source"].([]any); ok {
		for _, item := range list {
			if s, ok := item.(string); ok {
				if err := parseOne(s); err != nil {
					return nil, err
				}
			}
		}
	}

	return prefixes, nil
}
