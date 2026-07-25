// Design: plan/learned/1005-cp-survival-2-copp-port179.md -- config parsing

package copp

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/firewall"
)

const defaultPort = 179

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

	rateStr, ok := bgpTree["rate"].(string)
	if !ok || rateStr == "" {
		return coppPolicy{}, false, fmt.Errorf("copp: bgp rate is required")
	}

	lim, err := firewall.ParseRateSpec(rateStr)
	if err != nil {
		return coppPolicy{}, false, fmt.Errorf("copp: rate: %w", err)
	}

	policy := coppPolicy{
		Rate:       lim.Rate,
		RateUnit:   lim.Unit,
		Dimension:  lim.Dimension,
		OverPolicy: overPolicyAccept,
	}

	if burstStr, ok := bgpTree["burst"].(string); ok {
		b, err := strconv.ParseUint(burstStr, 10, 32)
		if err != nil {
			return coppPolicy{}, false, fmt.Errorf("copp: burst: invalid value %q: %w", burstStr, err)
		}
		policy.Burst = uint32(b)
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

	if op, ok := bgpTree["over-limit-policy"].(string); ok {
		switch op {
		case overPolicyDrop, overPolicyAccept:
			policy.OverPolicy = op
		default:
			return coppPolicy{}, false, fmt.Errorf("copp: over-limit-policy: invalid value %q (want drop|accept)", op)
		}
	}

	return policy, true, nil
}

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
