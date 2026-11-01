// Design: docs/architecture/core-design.md -- route modify filter config parsing
// Related: modify.go -- delta building and attribute encoding
// Related: filter_modify.go -- SDK entry point and handleFilterUpdate
//
// Config parsing for the bgp-filter-modify plugin.
//
// Reads named modify definitions out of the policy root:
//
//	policy { modify NAME { set { local-preference 200; } } }
//	policy { modify NAME { increment { local-preference 50; } } }
//	policy { modify NAME { set { community-add [ 65000:200 ]; } } }
//	policy { modify NAME { del { med; } } }
//
// Each definition's operations become the filter sets it answers (modify.go),
// built here once. The absent-attribute base an increment starts from is the
// engine's, read from bgp { defaults { attribute { } } } by the chain compile.
package filter_modify

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/filtertext"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const maxNameLen = 256

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

// parseModifyDefs reads policy/modify definitions out of policyCfg.
func parseModifyDefs(policyCfg map[string]any) (map[string]*modifyDef, error) {
	result := make(map[string]*modifyDef)

	modBlock, ok := policyCfg["modify"].(map[string]any)
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
		def, err := parseModifyDef(name, defMap)
		if err != nil {
			return nil, fmt.Errorf("modify %q: %w", name, err)
		}
		result[name] = def
	}
	return result, nil
}

// modifyOps is one definition's operations as config states them, the input
// to the conflict check and to the sets.
type modifyOps struct {
	setBlock   map[string]any
	increments []incdec
	decrements []incdec
	medRemove  bool
}

type incdec struct {
	attr  string // "local-preference", "med", "aigp"
	value uint32
}

// parseModifyDef reads one definition and builds the sets it answers.
func parseModifyDef(name string, defMap map[string]any) (*modifyDef, error) {
	def := &modifyDef{name: name}
	var ops modifyOps

	for key := range defMap {
		switch key {
		case "match", "set", "del", "increment", "decrement":
		default:
			return nil, fmt.Errorf("unknown key %q", key)
		}
	}

	if matchBlock, ok := defMap["match"].(map[string]any); ok {
		cond, err := parseMatchBlock(matchBlock)
		if err != nil {
			return nil, err
		}
		def.match = cond
	}

	if setBlock, ok := defMap["set"].(map[string]any); ok {
		for key := range setBlock {
			// A Loc-RIB set (distance, tag) is judged by the route
			// evaluator; the field check refuses it on a BGP chain.
			if ribSetKeys[key] {
				continue
			}
			if !setBlockAllowedKeys[key] {
				return nil, fmt.Errorf("set: unknown key %q", key)
			}
		}
		ops.setBlock = setBlock
	}

	if delBlock, ok := defMap["del"].(map[string]any); ok {
		for key := range delBlock {
			if key != medAttr {
				return nil, fmt.Errorf("del: unknown key %q", key)
			}
		}
		ops.medRemove = readBool(delBlock[medAttr])
	}

	if incBlock, ok := defMap["increment"].(map[string]any); ok {
		parsed, err := parseIncDecBlock(incBlock, "increment")
		if err != nil {
			return nil, err
		}
		ops.increments = parsed
	}

	if decBlock, ok := defMap["decrement"].(map[string]any); ok {
		parsed, err := parseIncDecBlock(decBlock, "decrement")
		if err != nil {
			return nil, err
		}
		ops.decrements = parsed
	}

	if err := validateNoConflict(&ops); err != nil {
		return nil, err
	}
	if err := buildSets(def, &ops); err != nil {
		return nil, err
	}

	if len(def.sets) == 0 && !ops.medRemove && !hasRIBSet(defMap) {
		return nil, fmt.Errorf("no operations defined")
	}
	return def, nil
}

// buildSets writes the sets and the delta text of def from ops: the set
// leaves first, then the increments, the decrements and the community
// operations, and last the med removal of the import variant.
func buildSets(def *modifyDef, ops *modifyOps) error {
	delta := textbuf.Get()
	defer delta.Release()

	setBlock := ops.setBlock
	for _, attr := range [...]string{localPreferenceAttr, medAttr} {
		if v, ok := readOptionalUint32(setBlock[attr]); ok {
			def.sets = append(def.sets, setNumber(attr, rpc.FilterSetActionSet, v))
			deltaText(delta, attr, 0, string(strconv.AppendUint(nil, uint64(v), 10)))
		}
	}
	if text, ok := setBlock[nextHopAttr].(string); ok && text != "" {
		if err := validateNextHop(text); err != nil {
			return err
		}
	}
	for _, attr := range [...]string{originAttr, nextHopAttr} {
		if text, ok := setBlock[attr].(string); ok && text != "" {
			def.sets = append(def.sets, setText(attr, rpc.FilterSetActionSet, text))
			deltaText(delta, attr, 0, text)
		}
	}
	if v, ok := readOptionalUint32(setBlock[asPathPrependAttr]); ok {
		// The YANG range is 1 to 32; a hand-written tree reaches here without
		// it, and a count outside it is refused rather than dropped.
		if v < 1 || v > 32 {
			return fmt.Errorf("set: %s %d is outside 1 to 32", asPathPrependAttr, v)
		}
		def.sets = append(def.sets, setNumber(policy.FieldASPath.String(), rpc.FilterSetActionPrepend, v))
		deltaText(delta, asPathPrependAttr, 0, string(strconv.AppendUint(nil, uint64(v), 10)))
	}
	for _, op := range ops.increments {
		def.sets = append(def.sets, setNumber(op.attr, rpc.FilterSetActionIncrement, op.value))
		deltaText(delta, op.attr, '+', string(strconv.AppendUint(nil, uint64(op.value), 10)))
	}
	for _, op := range ops.decrements {
		def.sets = append(def.sets, setNumber(op.attr, rpc.FilterSetActionDecrement, op.value))
		deltaText(delta, op.attr, '-', string(strconv.AppendUint(nil, uint64(op.value), 10)))
	}
	if err := buildCommunitySets(def, delta, setBlock); err != nil {
		return err
	}
	def.delta = delta.String()

	def.medRemove = ops.medRemove
	if def.medRemove {
		def.importSets = append(append(make([]rpc.FilterSet, 0, len(def.sets)+1), def.sets...), medRemoveSet())
		deltaText(delta, medRemoveText, 0, "")
		def.importDelta = strings.TrimSuffix(delta.String(), " ")
	}
	return nil
}

// communitySetLeaf is one community leaf of the set block: the field and the
// action of the sets it builds, and the parser that refuses a value the
// engine could not apply.
type communitySetLeaf struct {
	leaf     string
	field    string
	action   rpc.FilterSetAction
	validate func(string) error
}

// communitySetLeaves are the community leaves in the order their sets are
// built.
var communitySetLeaves = [...]communitySetLeaf{
	{communityAddDirective, "community", rpc.FilterSetActionAdd, validateStdCommunity},
	{communityRemoveDirective, "community", rpc.FilterSetActionRemove, validateStdCommunity},
	{largeCommunityAddDirective, "large-community", rpc.FilterSetActionAdd, validateLargeCommunity},
	{largeCommunityRemoveDirective, "large-community", rpc.FilterSetActionRemove, validateLargeCommunity},
	{extendedCommunityAddDirective, "extended-community", rpc.FilterSetActionAdd, validateExtCommunity},
	{extendedCommunityRemoveDirective, "extended-community", rpc.FilterSetActionRemove, validateExtCommunity},
}

// buildCommunitySets appends one set per configured community value.
func buildCommunitySets(def *modifyDef, delta *textbuf.Buffer, setBlock map[string]any) error {
	for i := range communitySetLeaves {
		leaf := &communitySetLeaves[i]
		values := readStringList(setBlock[leaf.leaf])
		for _, v := range values {
			if err := leaf.validate(v); err != nil {
				return fmt.Errorf("%s: %w", leaf.leaf, err)
			}
			def.sets = append(def.sets, setText(leaf.field, leaf.action, v))
		}
		if len(values) > 0 {
			deltaText(delta, leaf.leaf, 0, textbuf.Join(values, " "))
		}
	}
	return nil
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
			// A source condition (protocol, tag, level, route-type) is
			// judged by the Loc-RIB route evaluator; BGP routes carry
			// none of those fields, so the field check refuses such an
			// instance on a BGP chain.
			if policy.IsSourceConditionKey(key) {
				continue
			}
			return matchCond{}, fmt.Errorf("match: unknown key %q (allowed: community, large-community, extended-community, protocol, tag, level, route-type)", key)
		}
	}

	for _, kind := range [...]filtertext.CommunityKind{
		filtertext.CommunityStandard, filtertext.CommunityLarge, filtertext.CommunityExtended,
	} {
		field := kind.FieldName()
		for _, text := range readStringList(block[field]) {
			canonical, err := matchValueText(text, kind)
			if err != nil {
				return matchCond{}, fmt.Errorf("match: %s %q: %w", field, text, err)
			}
			cond.communities = append(cond.communities, newMatchCommunity(kind, canonical))
		}
	}

	return cond, nil
}

// matchValueText answers text, one configured community, in the form the
// route's fields carry it: the community formatter's text. That is the
// canonical text for a standard or a large community, and the decoded form
// (AppendDecoded, "target:65000:100") for an extended one, whose canonical
// text is hex.
func matchValueText(text string, kind filtertext.CommunityKind) (string, error) {
	if kind != filtertext.CommunityExtended {
		return filtertext.CanonicalCommunity(text, kind)
	}
	wire, _, err := filtertext.CommunityWire(text, kind)
	if err != nil {
		return "", err
	}
	var community attribute.ExtendedCommunity
	copy(community[:], wire[:len(community)])
	return string(community.AppendDecoded(nil)), nil
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

// validateNextHop refuses a next-hop the engine's next-hop set cannot write:
// that set writes the NEXT_HOP attribute (policychain Sets.NextHop), whose
// value is one IPv4 address, so an IPv6 address is refused when the config is
// read rather than failing every route it meets. An IPv6 next hop travels in
// MP_REACH_NLRI, which a modify does not set. The text path before the sets
// refused it too, logging "next-hop must be IPv4" and dropping the operation.
//
// RFC 4271 Section 4.3: "This is a well-known mandatory attribute that defines
// the (unicast) IP address of the router that SHOULD be used as the next hop
// to the destinations listed in the Network Layer Reachability Information
// field of the UPDATE message."
func validateNextHop(s string) error {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return fmt.Errorf("set: next-hop %q is not an IP address", s)
	}
	if !addr.Is4() {
		return fmt.Errorf("set: next-hop %s is not an IPv4 address: the NEXT_HOP attribute (RFC 4271 Section 4.3) carries IPv4 only, and an IPv6 next hop travels in MP_REACH_NLRI, which modify does not set; write an IPv4 address", s)
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

// validateExtCommunity refuses a value the engine's extended community set
// cannot apply (policychain applyAttributeSet parses one value the same way).
func validateExtCommunity(s string) error {
	_, err := attribute.ParseSingleExtCommunity(s)
	return err
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

// validateNoConflict checks that set, del, and increment/decrement don't target
// the same attribute.
func validateNoConflict(ops *modifyOps) error {
	_, setsMED := readOptionalUint32(ops.setBlock[medAttr])
	// Deleting MULTI_EXIT_DISC and writing one are opposite instructions about
	// the same attribute. Refuse the pair rather than let their processing order
	// decide what reaches the wire.
	if ops.medRemove {
		if setsMED {
			return fmt.Errorf("del med conflicts with set med (mutually exclusive)")
		}
		for _, list := range [][]incdec{ops.increments, ops.decrements} {
			for _, op := range list {
				if op.attr == medAttr {
					return fmt.Errorf("del med conflicts with increment/decrement med (mutually exclusive)")
				}
			}
		}
	}

	for _, op := range ops.increments {
		if _, set := readOptionalUint32(ops.setBlock[op.attr]); set {
			return fmt.Errorf("increment %s conflicts with set %s (mutually exclusive)", op.attr, op.attr)
		}
	}
	for _, op := range ops.decrements {
		if _, set := readOptionalUint32(ops.setBlock[op.attr]); set {
			return fmt.Errorf("decrement %s conflicts with set %s (mutually exclusive)", op.attr, op.attr)
		}
	}
	return nil
}
