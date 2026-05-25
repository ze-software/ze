// Design: docs/architecture/core-design.md -- remove-private-as filter config parsing
// Related: private_as.go -- policy behavior helpers
// Related: filter_remove_private_as.go -- SDK entry point and filter handler

package filter_remove_private_as

import "fmt"

const maxNameLen = 256

type removePrivateASDef struct {
	name string
	mode removeMode
}

func parseRemovePrivateASDefs(bgpCfg map[string]any) (map[string]*removePrivateASDef, error) {
	result := make(map[string]*removePrivateASDef)
	policyBlock, ok := bgpCfg["policy"].(map[string]any)
	if !ok {
		return result, nil
	}
	block, ok := policyBlock["remove-private-as"].(map[string]any)
	if !ok {
		return result, nil
	}
	for name, raw := range block {
		if len(name) > maxNameLen {
			return nil, fmt.Errorf("remove-private-as name %q exceeds maximum length %d", name, maxNameLen)
		}
		defMap, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("remove-private-as %q: not a map", name)
		}
		mode := removeModeStrip
		if rawMode, ok := defMap["replace-with"]; ok {
			s, ok := rawMode.(string)
			if !ok {
				return nil, fmt.Errorf("remove-private-as %q: replace-with is not a string", name)
			}
			switch s {
			case "", removeModeStripText:
				mode = removeModeStrip
			case removeModePeerASText:
				mode = removeModePeerAS
			default:
				return nil, fmt.Errorf("remove-private-as %q: invalid replace-with %q", name, s)
			}
		}
		result[name] = &removePrivateASDef{name: name, mode: mode}
	}
	return result, nil
}
