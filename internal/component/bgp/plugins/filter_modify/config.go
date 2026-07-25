// Design: docs/architecture/core-design.md -- route modify filter config parsing
// Related: modify.go -- delta building and attribute encoding
// Related: filter_modify.go -- SDK entry point and handleFilterUpdate
//
// Config parsing for the bgp-filter-modify plugin.
//
// Reads named modify definitions out of the BGP config subtree:
//
//	bgp { policy { modify NAME { set { local-preference 200; } } } }
//	bgp { policy { modify NAME { increment { local-preference 50; } } } }
//	bgp { policy { modify NAME { set { community-add [ 65000:200 ]; } } } }
//
// Static set operations produce pre-built delta strings.
// Increment/decrement operations are computed at runtime from current values.
// Community add/remove operations emit text directives for the engine.
package filter_modify

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	maxNameLen = 256
)

var setBlockAllowedKeys = map[string]bool{
	"local-preference":          true,
	"med":                       true,
	"origin":                    true,
	"next-hop":                  true,
	"as-path-prepend":           true,
	"community-add":             true,
	"community-remove":          true,
	"large-community-add":       true,
	"large-community-remove":    true,
	"extended-community-add":    true,
	"extended-community-remove": true,
}

var incDecAttrs = map[string]bool{
	"local-preference": true,
	"med":              true,
	"aigp":             true,
}

func parseModifyDefs(bgpCfg map[string]any) (map[string]*modifyDef, error) {
	result := make(map[string]*modifyDef)

	policyBlock, ok := bgpCfg["policy"].(map[string]any)
	if !ok {
		return result, nil
	}
	modBlock, ok := policyBlock["modify"].(map[string]any)
	if !ok {
		return result, nil
	}

	for name, raw := range modBlock {
		if len(name) > maxNameLen {
			return nil, fmt.Errorf("modify name %q exceeds maximum length %d", name, maxNameLen)
		}
		defMap, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("modify %q: not a map", name)
		}

		def := &modifyDef{name: name}

		for key := range defMap {
			switch key {
			case "set", "increment", "decrement":
			default:
				return nil, fmt.Errorf("modify %q: unknown key %q", name, key)
			}
		}

		if setBlock, ok := defMap["set"].(map[string]any); ok {
			for key := range setBlock {
				if !setBlockAllowedKeys[key] {
					return nil, fmt.Errorf("modify %q: set: unknown key %q", name, key)
				}
			}
			def.delta = buildDelta(setBlock)
			if err := parseCommOps(setBlock, def); err != nil {
				return nil, fmt.Errorf("modify %q: %w", name, err)
			}
		}

		if incBlock, ok := defMap["increment"].(map[string]any); ok {
			ops, err := parseIncDecBlock(incBlock, "increment")
			if err != nil {
				return nil, fmt.Errorf("modify %q: %w", name, err)
			}
			def.increments = ops
		}

		if decBlock, ok := defMap["decrement"].(map[string]any); ok {
			ops, err := parseIncDecBlock(decBlock, "decrement")
			if err != nil {
				return nil, fmt.Errorf("modify %q: %w", name, err)
			}
			def.decrements = ops
		}

		if err := validateNoConflict(def); err != nil {
			return nil, fmt.Errorf("modify %q: %w", name, err)
		}

		if def.delta == "" && len(def.increments) == 0 && len(def.decrements) == 0 && len(def.commOps) == 0 {
			return nil, fmt.Errorf("modify %q: no operations defined", name)
		}

		result[name] = def
	}
	return result, nil
}

func parseIncDecBlock(block map[string]any, kind string) ([]incdec, error) {
	var ops []incdec
	for attr, raw := range block {
		if !incDecAttrs[attr] {
			return nil, fmt.Errorf("%s: unsupported attribute %q (allowed: local-preference, med, aigp)", kind, attr)
		}
		v, ok := readOptionalUint32(raw)
		if !ok || v == 0 {
			return nil, fmt.Errorf("%s %s: value must be a positive integer", kind, attr)
		}
		ops = append(ops, incdec{attr: attr, value: v})
	}
	return ops, nil
}

func parseCommOps(setBlock map[string]any, def *modifyDef) error {
	type commDef struct {
		key       string
		directive string
		validate  func(string) error
	}

	defs := []commDef{
		{"community-add", "community-add", validateStdCommunity},
		{"community-remove", "community-remove", validateStdCommunity},
		{"large-community-add", "large-community-add", validateLargeCommunity},
		{"large-community-remove", "large-community-remove", validateLargeCommunity},
		{"extended-community-add", "extended-community-add", validateExtCommunity},
		{"extended-community-remove", "extended-community-remove", validateExtCommunity},
	}

	for _, cd := range defs {
		values := readStringList(setBlock[cd.key])
		if len(values) == 0 {
			continue
		}
		if cd.validate != nil {
			for _, v := range values {
				if err := cd.validate(v); err != nil {
					return fmt.Errorf("%s: %w", cd.key, err)
				}
			}
		}
		def.commOps = append(def.commOps, commOp{
			directive: cd.directive,
			values:    joinValues(values),
		})
	}
	return nil
}

func validateStdCommunity(s string) error {
	_, err := attribute.ParseCommunity(s)
	return err
}

func validateLargeCommunity(s string) error {
	_, err := attribute.ParseLargeCommunity(s)
	return err
}

func validateExtCommunity(s string) error {
	b := attribute.NewBuilder()
	return b.ParseExtCommunity(s)
}

func readStringList(v any) []string {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []any:
		var result []string
		for _, item := range val {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	case string:
		if val != "" {
			return []string{val}
		}
	}
	return nil
}

func joinValues(values []string) string {
	return textbuf.Join(values, " ")
}

// validateNoConflict checks that set and increment/decrement don't target the same attribute.
func validateNoConflict(def *modifyDef) error {
	setAttrs := map[string]bool{}
	if def.delta != "" {
		for _, attr := range []string{"local-preference", "med", "aigp"} {
			if extractUint32Attr(def.delta, attr) > 0 || containsAttrName(def.delta, attr) {
				setAttrs[attr] = true
			}
		}
	}
	for _, op := range def.increments {
		if setAttrs[op.attr] {
			return fmt.Errorf("increment %s conflicts with set %s (mutually exclusive)", op.attr, op.attr)
		}
	}
	for _, op := range def.decrements {
		if setAttrs[op.attr] {
			return fmt.Errorf("decrement %s conflicts with set %s (mutually exclusive)", op.attr, op.attr)
		}
	}
	return nil
}

func containsAttrName(delta, attr string) bool {
	prefix := attr + " "
	return strings.HasPrefix(delta, prefix) || strings.Contains(delta, " "+prefix)
}
