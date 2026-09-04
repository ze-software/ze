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
//	bgp { policy { modify NAME { del { med; } } } }
//
// Static set operations produce pre-built delta strings.
// Increment/decrement operations are computed at runtime from current values.
// Community add/remove operations emit text directives for the engine.
package filter_modify

import (
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/filtertext"
	// The schema lookup in parseAttributeDefaults needs the bgp module in the
	// loader, and the module puts itself there from this package's init
	// (configyang.RegisterModule, bgp/yang/register.go). The daemon links it for
	// other reasons; this package's own test binary does not.
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	maxNameLen = 256
	// attributeDefaultsPath is the schema container that DECLARES the value the
	// arithmetic starts from for an attribute the route does not carry. The
	// numbers live there and nowhere else (ai/rules/principles.md), and the RFC
	// reasoning for each one is the ze:help on its leaf.
	attributeDefaultsPath = "bgp/defaults/attribute"
)

var setBlockAllowedKeys = map[string]bool{
	localPreferenceAttr:              true,
	medAttr:                          true,
	originAttr:                       true,
	nextHopAttr:                      true,
	asPathPrependAttr:                true,
	communityAddDirective:            true,
	communityRemoveDirective:         true,
	largeCommunityAddDirective:       true,
	largeCommunityRemoveDirective:    true,
	extendedCommunityAddDirective:    true,
	extendedCommunityRemoveDirective: true,
}

var incDecAttrs = map[string]bool{
	localPreferenceAttr: true,
	medAttr:             true,
	aigpAttr:            true,
}

// attributeDefaultsContainer resolves the schema container ONCE for the life of
// the process.
//
// The YANG modules are compiled into the binary, so the answer cannot change
// while the daemon runs, and config.YANGSchema puts every module through the
// loader and the resolver. Paying that on each config delivery would put a
// whole-schema resolve on the reload path for two numbers, and a plugin that
// reloads is a plugin an operator is waiting on.
var attributeDefaultsContainer = sync.OnceValues(func() (*config.ContainerNode, error) {
	schema, err := config.YANGSchema()
	if err != nil {
		return nil, fmt.Errorf("attribute defaults: load schema: %w", err)
	}
	node, err := schema.Lookup(attributeDefaultsPath)
	if err != nil {
		return nil, fmt.Errorf("attribute defaults: resolve %s: %w", attributeDefaultsPath, err)
	}
	container, ok := node.(*config.ContainerNode)
	if !ok {
		return nil, fmt.Errorf("attribute defaults: %s is not a container", attributeDefaultsPath)
	}
	return container, nil
})

// parseAttributeDefaults reads bgp { defaults { attribute { } } }, the value an
// increment or a decrement starts from for an attribute the route does not
// carry. One entry per leaf the container declares, so an attribute the
// container does not name gets no arithmetic at all (currentForArithmetic).
//
// A leaf the operator left out arrives here from the schema itself:
// ApplyDefaults writes the YANG default into the map, which is how a default
// reaches a section that is not a peer entry (config/schema_defaults.go, and
// sysrib.parseAdminDistanceConfig does the same for rib { distance { } }).
//
// An unreadable schema is an ERROR rather than an empty map. An empty map
// refuses every arithmetic on an absent attribute and looks exactly like a
// route that carried its own value, so the operator would read a metric that
// silently stopped being adjusted.
func parseAttributeDefaults(bgpCfg map[string]any) (map[string]uint32, error) {
	declared := map[string]any{}
	if defaultsBlock, ok := bgpCfg["defaults"].(map[string]any); ok {
		if attrBlock, ok := defaultsBlock["attribute"].(map[string]any); ok {
			// Copied rather than used in place: ApplyDefaults writes into the map
			// it is given, and the config tree this section arrived in belongs to
			// the caller.
			maps.Copy(declared, attrBlock)
		}
	}

	container, err := attributeDefaultsContainer()
	if err != nil {
		return nil, err
	}

	// A key the container does not declare is REFUSED rather than kept. YANG
	// validation already refuses one at config load, and this is the second
	// check the pair rule asks for: a hand-written or migrated tree reaches the
	// plugin without passing that validation, and an `aigp` key that survived
	// would give the arithmetic a base RFC 7311 Section 4.1 says no number can
	// stand in for. The allowed set is the container's own children, so it
	// cannot disagree with the schema.
	for key := range declared {
		if !container.Has(key) {
			return nil, fmt.Errorf("attribute defaults: unknown key %q (declared: %s)",
				key, strings.Join(container.Children(), ", "))
		}
	}

	config.ApplyDefaults(declared, container)

	base := make(map[string]uint32, len(declared))
	for attr, raw := range declared {
		value, ok := readOptionalUint32(raw)
		if !ok {
			return nil, fmt.Errorf("attribute defaults: %s: %v is not an unsigned 32-bit value", attr, raw)
		}
		base[attr] = value
	}
	return base, nil
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
			case "match", "set", "del", "increment", "decrement":
			default:
				return nil, fmt.Errorf("modify %q: unknown key %q", name, key)
			}
		}

		if matchBlock, ok := defMap["match"].(map[string]any); ok {
			cond, err := parseMatchBlock(matchBlock)
			if err != nil {
				return nil, fmt.Errorf("modify %q: %w", name, err)
			}
			def.match = cond
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

		if delBlock, ok := defMap["del"].(map[string]any); ok {
			for key := range delBlock {
				if key != medAttr {
					return nil, fmt.Errorf("modify %q: del: unknown key %q", name, key)
				}
			}
			def.medRemove = readBool(delBlock[medAttr])
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

		if def.delta == "" && len(def.increments) == 0 && len(def.decrements) == 0 &&
			len(def.commOps) == 0 && !def.medRemove {
			return nil, fmt.Errorf("modify %q: no operations defined", name)
		}

		result[name] = def
	}
	return result, nil
}

// parseMatchBlock reads the condition a route meets before the operations
// apply. An unknown key is refused rather than ignored: a condition nobody
// evaluates reads as in force in the running config and is not.
func parseMatchBlock(block map[string]any) (matchCond, error) {
	var cond matchCond

	for key := range block {
		switch key {
		case "community", "large-community", "extended-community":
		default:
			return matchCond{}, fmt.Errorf("match: unknown key %q (allowed: community, large-community, extended-community)", key)
		}
	}

	for _, s := range readStringList(block["community"]) {
		// Normalized through the attribute's own text form, because the match is
		// a comparison against the text the formatter emits and that text
		// substitutes a name for a well-known value. An operator who writes
		// 65535:666 means the value the formatter renders as "blackhole", and
		// without this the condition could never fire.
		v, err := attribute.ParseCommunity(s)
		if err != nil {
			return matchCond{}, fmt.Errorf("match: community %q: %w", s, err)
		}
		cond.communities = append(cond.communities, matchCommunity{
			kind:  filtertext.CommunityStandard,
			value: attribute.Community(v).String(),
		})
	}

	// Large and extended values are validated and kept as written. Neither
	// attribute substitutes a name for a value, so the written form is the
	// emitted form.
	for _, s := range readStringList(block["large-community"]) {
		if err := validateLargeCommunity(s); err != nil {
			return matchCond{}, fmt.Errorf("match: large-community %q: %w", s, err)
		}
		cond.communities = append(cond.communities, matchCommunity{
			kind:  filtertext.CommunityLarge,
			value: s,
		})
	}

	for _, s := range readStringList(block["extended-community"]) {
		if err := validateExtCommunity(s); err != nil {
			return matchCond{}, fmt.Errorf("match: extended-community %q: %w", s, err)
		}
		cond.communities = append(cond.communities, matchCommunity{
			kind:  filtertext.CommunityExtended,
			value: s,
		})
	}

	return cond, nil
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
		{communityAddDirective, communityAddDirective, validateStdCommunity},
		{communityRemoveDirective, communityRemoveDirective, validateStdCommunity},
		{largeCommunityAddDirective, largeCommunityAddDirective, validateLargeCommunity},
		{largeCommunityRemoveDirective, largeCommunityRemoveDirective, validateLargeCommunity},
		{extendedCommunityAddDirective, extendedCommunityAddDirective, validateExtCommunity},
		{extendedCommunityRemoveDirective, extendedCommunityRemoveDirective, validateExtCommunity},
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

// validateNoConflict checks that set, del, and increment/decrement don't target
// the same attribute.
func validateNoConflict(def *modifyDef) error {
	// Deleting MULTI_EXIT_DISC and writing one are opposite instructions about
	// the same attribute. Refuse the pair rather than let their processing order
	// decide what reaches the wire.
	if def.medRemove {
		if containsAttrName(def.delta, medAttr) {
			return fmt.Errorf("del med conflicts with set med (mutually exclusive)")
		}
		for _, ops := range [][]incdec{def.increments, def.decrements} {
			for _, op := range ops {
				if op.attr == medAttr {
					return fmt.Errorf("del med conflicts with increment/decrement med (mutually exclusive)")
				}
			}
		}
	}

	setAttrs := map[string]bool{}
	if def.delta != "" {
		for _, attr := range []string{localPreferenceAttr, medAttr, aigpAttr} {
			// containsAttrName ANCHORS on a token boundary, so it answers for
			// a set of any value including 0. The value read that used to sit
			// beside it added nothing this cannot see and matched unanchored.
			if containsAttrName(def.delta, attr) {
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
