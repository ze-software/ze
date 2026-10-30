// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: register.go -- RootName

package policy

import (
	"encoding/json"
	"fmt"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// SectionParser builds the sdk.ConfigParser of a plugin whose config is what
// parse reads from the policy root: a filter type plugin reads its own list
// there. A delivery with no policy section carries nothing for the plugin. A
// policy section that is not a JSON object is refused with errInvalid, and a
// parse error is wrapped with the plugin's name, so the operator reads which
// plugin refused the commit.
func SectionParser[T any](plugin string, errInvalid error, parse func(policyCfg map[string]any) (T, error)) sdk.ConfigParser[T] {
	return func(sections []sdk.ConfigSection) (T, bool, error) {
		var none T
		for _, section := range sections {
			if section.Root != RootName {
				continue
			}
			policyCfg, ok := parseSubtree(section.Data, RootName)
			if !ok {
				return none, false, errInvalid
			}
			cfg, err := parse(policyCfg)
			if err != nil {
				return none, false, fmt.Errorf("%s: %w", plugin, err)
			}
			return cfg, true, nil
		}
		return none, false, nil
	}
}

// SectionsParser builds the sdk.ConfigParser of a filter type plugin that
// reads its own list under the policy root and also a root it does not own:
// a family-filter checks the export chains of the bgp peers that reference
// it, and a modify filter reads bgp's attribute defaults. The plugin MUST
// declare readRoot in registry.Registration.ConfigReads, because a reload
// then delivers both roots whole whenever either changes
// (plugin/server reloadConfigSections). A root absent from that delivery holds
// no config, so parse receives it as a nil map, which reads as empty. A
// delivery with neither root carries nothing for the plugin.
func SectionsParser[T any](plugin string, errInvalid error, readRoot string, parse func(policyCfg, readCfg map[string]any) (T, error)) sdk.ConfigParser[T] {
	return func(sections []sdk.ConfigSection) (T, bool, error) {
		var none T
		var policyCfg, readCfg map[string]any
		present := false
		for _, section := range sections {
			if section.Root != RootName && section.Root != readRoot {
				continue
			}
			subtree, ok := parseSubtree(section.Data, section.Root)
			if !ok {
				return none, false, errInvalid
			}
			if section.Root == RootName {
				policyCfg = subtree
			} else {
				readCfg = subtree
			}
			present = true
		}
		if !present {
			return none, false, nil
		}
		cfg, err := parse(policyCfg, readCfg)
		if err != nil {
			return none, false, fmt.Errorf("%s: %w", plugin, err)
		}
		return cfg, true, nil
	}
}

// parseSubtree unwraps the root's object from a section's JSON, which arrives
// either wrapped as {"<root>": {...}} or as the bare object.
func parseSubtree(data, root string) (map[string]any, bool) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(data), &tree); err != nil {
		return nil, false
	}
	if inner, ok := tree[root].(map[string]any); ok {
		return inner, true
	}
	return tree, true
}
