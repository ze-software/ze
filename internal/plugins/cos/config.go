// Design: plan/learned/884-cos-plugin.md -- CoS profile config parsing

package cos

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	coreCos "github.com/ze-software/ze/internal/core/cos"
)

// parseAndRegisterProfiles parses class-of-service ieee-802.1p profiles
// from the YANG JSON config section and registers them in the shared
// cos registry. Called during InProcessConfigVerifier so that profiles
// are available when the iface verifier runs (alphabetical ordering:
// "cos" < "interface").
func parseAndRegisterProfiles(data string) error {
	if data == "" {
		return nil
	}

	var tree map[string]any
	if err := json.Unmarshal([]byte(data), &tree); err != nil {
		return fmt.Errorf("cos: invalid config JSON: %w", err)
	}

	cosBlock, ok := tree["class-of-service"].(map[string]any)
	if !ok {
		return nil
	}
	profileMap, ok := cosBlock["ieee-802.1p"].(map[string]any)
	if !ok {
		return nil
	}

	for name, v := range profileMap {
		pm, _ := v.(map[string]any)
		p, err := parseProfile(name, pm)
		if err != nil {
			return err
		}
		coreCos.Register(name, p)
	}
	return nil
}

func parseProfile(name string, m map[string]any) (coreCos.Profile, error) {
	var p coreCos.Profile

	if ingress, ok := m["ingress"].(map[string]any); ok {
		pcpMap, ok := ingress["pcp"].(map[string]any)
		if ok {
			im, err := parsePCPToPriority(name, "ingress", pcpMap)
			if err != nil {
				return p, err
			}
			p.IngressMap = im
		}
	}

	if egress, ok := m["egress"].(map[string]any); ok {
		prioMap, ok := egress["priority"].(map[string]any)
		if ok {
			em, err := parsePriorityToPCP(name, "egress", prioMap)
			if err != nil {
				return p, err
			}
			p.EgressMap = em
		}
	}

	return p, nil
}

func parsePCPToPriority(profileName, direction string, entries map[string]any) (map[uint32]uint32, error) {
	m := make(map[uint32]uint32, len(entries))
	for keyStr, v := range entries {
		key, err := parsePCPValue(keyStr)
		if err != nil {
			return nil, fmt.Errorf("profile %q %s pcp %q: %w", profileName, direction, keyStr, err)
		}
		entry, _ := v.(map[string]any)
		valStr, ok := entry["priority"].(string)
		if !ok {
			return nil, fmt.Errorf("profile %q %s pcp %q: missing priority", profileName, direction, keyStr)
		}
		val, err := parsePCPValue(valStr)
		if err != nil {
			return nil, fmt.Errorf("profile %q %s pcp %q priority %q: %w", profileName, direction, keyStr, valStr, err)
		}
		m[key] = val
	}
	return m, nil
}

func parsePriorityToPCP(profileName, direction string, entries map[string]any) (map[uint32]uint32, error) {
	m := make(map[uint32]uint32, len(entries))
	for keyStr, v := range entries {
		key, err := parsePCPValue(keyStr)
		if err != nil {
			return nil, fmt.Errorf("profile %q %s priority %q: %w", profileName, direction, keyStr, err)
		}
		entry, _ := v.(map[string]any)
		valStr, ok := entry["pcp"].(string)
		if !ok {
			return nil, fmt.Errorf("profile %q %s priority %q: missing pcp", profileName, direction, keyStr)
		}
		val, err := parsePCPValue(valStr)
		if err != nil {
			return nil, fmt.Errorf("profile %q %s priority %q pcp %q: %w", profileName, direction, keyStr, valStr, err)
		}
		m[key] = val
	}
	return m, nil
}

// resolveCoSForUnit implements the class-of-service resolution logic:
// inheritance from parent, per-unit override, "none" opt-out, mutual
// exclusion with inline maps, and profile lookup.
func resolveCoSForUnit(parentCoS, unitCoS string, hasInlineMaps bool) (map[uint32]uint32, map[uint32]uint32, error) {
	name := unitCoS
	if name == "" {
		name = parentCoS
	}
	if name == "none" || name == "" {
		return nil, nil, nil
	}
	if hasInlineMaps {
		return nil, nil, fmt.Errorf("class-of-service and inline qos maps are mutually exclusive")
	}
	p, ok := coreCos.Lookup(name)
	if !ok {
		return nil, nil, fmt.Errorf("class-of-service profile %q not found", name)
	}
	return p.IngressMap, p.EgressMap, nil
}

func parsePCPValue(s string) (uint32, error) {
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, errors.New("not a number")
	}
	if n > 7 {
		return 0, errors.New("out of range (0-7)")
	}
	return uint32(n), nil
}
