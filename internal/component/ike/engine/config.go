// Design: plan/spec-ipsec-7-ikev2-engine.md -- config parsing for IKE engine
package engine

import (
	"encoding/json"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/ipsec"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// parseIPsecSections finds the "vpn" config section and parses the IPsec config.
func parseIPsecSections(sections []sdk.ConfigSection) (*ipsec.IPsecConfig, error) {
	for _, s := range sections {
		if s.Root != "vpn" {
			continue
		}
		return parseIPsecFromJSON(s.Data)
	}
	return &ipsec.IPsecConfig{
		ESPGroups: make(map[string]ipsec.ESPGroup),
		IKEGroups: make(map[string]ipsec.IKEGroup),
		Peers:     make(map[string]ipsec.SiteToSitePeer),
	}, nil
}

// parseIPsecFromJSON parses the JSON config section data into IPsecConfig.
func parseIPsecFromJSON(data string) (*ipsec.IPsecConfig, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, err
	}
	tree := treeFromMap(raw)
	return ipsec.ParseIPsecConfig(tree)
}

// treeFromMap recursively converts a map[string]any (from JSON) to a config.Tree.
// Every nested map is stored as a container (for GetContainer). If all children
// of a nested map are themselves maps, each child is also stored as a list entry
// (for GetListOrdered). This dual storage handles the ambiguity between YANG
// containers and keyed lists in untyped JSON.
func treeFromMap(m map[string]any) *config.Tree {
	t := config.NewTree()
	for k, v := range m {
		switch val := v.(type) {
		case string:
			t.Set(k, val)
		case float64:
			t.Set(k, strconv.FormatFloat(val, 'f', -1, 64))
		case bool:
			if val {
				t.Set(k, "true")
			} else {
				t.Set(k, "false")
			}
		case map[string]any:
			t.SetContainer(k, treeFromMap(val))
			if allMaps(val) {
				for entryKey, entryVal := range val {
					if em, ok := entryVal.(map[string]any); ok {
						t.AddListEntry(k, entryKey, treeFromMap(em))
					}
				}
			}
		case []any:
			for _, item := range val {
				if s, ok := item.(string); ok {
					t.AppendValue(k, s)
				}
			}
		}
	}
	return t
}

// allMaps returns true if all values in the map are maps (keyed list pattern).
func allMaps(m map[string]any) bool {
	if len(m) == 0 {
		return false
	}
	for _, v := range m {
		if _, ok := v.(map[string]any); !ok {
			return false
		}
	}
	return true
}
