// Design: docs/architecture/plugin/rib-storage-design.md -- per-scope SRv6 Service SID resolvability policy
// RFC: rfc/short/rfc9252.md -- Section 5 local-policy bypass for SRv6 Service SID resolvability
// Related: yang/ze-rib.yang -- srv6-sid-resolvability-fields grouping and four BGP augmentations
// Related: config_tx.go -- ConfigTx verify/apply/rollback and reload reselection
// Related: rib_next_hop.go -- candidateSRv6SIDResolved, which queries srv6SIDResolvabilityIgnoredLocked

package rib

import (
	"encoding/json"
	"maps"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	configKeySRv6Resolvability = "srv6-sid-resolvability"
	configKeySRv6Ignore        = "ignore"
	configKeySRv6Family        = "family"
	configKeySRv6FamilyName    = "name"
	configBoolTrue             = "true"
	configBoolFalse            = "false"
	configPrefixBGPGroup       = "bgp group "
	configSetPrefix            = "set "
	configValueSep             = " value "
	configSetIgnoreFixSuffix   = " ignore true (or false)"
)

// srv6ResolvabilityScope holds the explicit srv6-sid-resolvability settings
// stated at one config level (bgp, group, or peer). Zero value means nothing
// was stated at this level, allowing inheritance from the enclosing scope.
type srv6ResolvabilityScope struct {
	hasIgnore bool
	ignore    bool
	families  map[family.Family]bool
}

// srv6ResolvabilityPeerRule holds the explicit group and peer scopes for one
// configured peer, or one dynamic group template (where peer is empty).
type srv6ResolvabilityPeerRule struct {
	group srv6ResolvabilityScope
	peer  srv6ResolvabilityScope
}

// srv6ResolvabilityConfig holds the parsed SRv6 Service SID resolvability
// local-policy bypass rules across global bgp, group, and peer scopes.
// Strict resolvability enforcement (ignore = false) is the default everywhere.
type srv6ResolvabilityConfig struct {
	global srv6ResolvabilityScope
	peers  map[configjson.PeerConfigKey]srv6ResolvabilityPeerRule
}

func (s srv6ResolvabilityScope) isEmpty() bool {
	if s.hasIgnore {
		return false
	}
	return len(s.families) == 0
}

func (s srv6ResolvabilityScope) equal(other srv6ResolvabilityScope) bool {
	if s.hasIgnore != other.hasIgnore {
		return false
	}
	if s.hasIgnore {
		if s.ignore != other.ignore {
			return false
		}
	}
	return maps.Equal(s.families, other.families)
}

func (s srv6ResolvabilityScope) lookup(fam family.Family) (ignore, ok bool) {
	if v, found := s.families[fam]; found {
		return v, true
	}
	if s.hasIgnore {
		return s.ignore, true
	}
	return false, false
}

func (r srv6ResolvabilityPeerRule) isEmpty() bool {
	if !r.group.isEmpty() {
		return false
	}
	return r.peer.isEmpty()
}

func (r srv6ResolvabilityPeerRule) equal(other srv6ResolvabilityPeerRule) bool {
	if !r.group.equal(other.group) {
		return false
	}
	return r.peer.equal(other.peer)
}

func (c srv6ResolvabilityConfig) isEmpty() bool {
	if !c.global.isEmpty() {
		return false
	}
	return len(c.peers) == 0
}

func (c srv6ResolvabilityConfig) equal(other srv6ResolvabilityConfig) bool {
	if !c.global.equal(other.global) {
		return false
	}
	if len(c.peers) != len(other.peers) {
		return false
	}
	for k, rule := range c.peers {
		otherRule, ok := other.peers[k]
		if !ok {
			return false
		}
		if !rule.equal(otherRule) {
			return false
		}
	}
	return true
}

// ignored answers whether SRv6 Service SID reachability checking is bypassed
// for the given peer address, dynamic/enclosing group name, and address family.
// Precedence is strictly most-specific wins:
// peer family > peer > group family > group > bgp family > bgp > false.
func (c *srv6ResolvabilityConfig) ignored(peerAddr, groupName string, fam family.Family) bool {
	if c == nil {
		return false
	}
	if rule, ok := configjson.LookupPeerConfig(c.peers, peerAddr, "", groupName); ok {
		if v, matched := rule.peer.lookup(fam); matched {
			return v
		}
		if v, matched := rule.group.lookup(fam); matched {
			return v
		}
	}
	if v, matched := c.global.lookup(fam); matched {
		return v
	}
	return false
}

// srv6SIDResolvabilityIgnoredLocked answers whether SRv6 Service SID
// reachability checking is bypassed for owner/peerAddr in fam.
// Caller MUST hold peerMu.RLock (which peerGroupName reads under).
func (r *RIBManager) srv6SIDResolvabilityIgnoredLocked(owner uint64, peerAddr string, fam family.Family) bool {
	cfg := r.srv6ResolvabilityCfg.Load()
	if cfg == nil {
		return false
	}
	return cfg.ignored(peerAddr, r.peerGroupName(owner), fam)
}

// srv6SIDResolvabilityIgnoredWithGroup answers whether SRv6 Service SID
// reachability checking is bypassed for peerAddr/groupName in fam without
// acquiring peerMu. Safe for concurrent use from show pipelines.
func (r *RIBManager) srv6SIDResolvabilityIgnoredWithGroup(peerAddr, groupName string, fam family.Family) bool {
	cfg := r.srv6ResolvabilityCfg.Load()
	if cfg == nil {
		return false
	}
	return cfg.ignored(peerAddr, groupName, fam)
}

// parseSRv6ResolvabilityConfig parses srv6-sid-resolvability across the bgp,
// group, peer, and group/peer scopes without mutating live RIB state.
//
// RFC 9252 Section 5 requires strict SRv6 Service SID resolvability by default
// while permitting local policy to ignore the resolvability verdict when an
// alternate steering mechanism reaches the egress PE. Invalid booleans, unknown
// keys, or unregistered family names are refused with an actionable opererr.
func parseSRv6ResolvabilityConfig(bgpCfg map[string]any) (srv6ResolvabilityConfig, error) {
	globalScope, err := parseSRv6ResolvabilityScope(bgpCfg, "bgp")
	if err != nil {
		return srv6ResolvabilityConfig{}, err
	}

	// Validate every declared group upfront so even an empty group with an
	// invalid srv6-sid-resolvability block is refused at verify/validate.
	groupScopes := map[string]srv6ResolvabilityScope{}
	if groupsMap, ok := bgpCfg["group"].(map[string]any); ok {
		groupNames := make([]string, 0, len(groupsMap))
		for name := range groupsMap {
			groupNames = append(groupNames, name)
		}
		slices.Sort(groupNames)
		for _, groupName := range groupNames {
			groupMap, ok := groupsMap[groupName].(map[string]any)
			if !ok {
				continue
			}
			scope, groupErr := parseSRv6ResolvabilityScope(groupMap, configPrefixBGPGroup+groupName)
			if groupErr != nil {
				return srv6ResolvabilityConfig{}, groupErr
			}
			groupScopes[groupName] = scope
		}
	}

	cfg := srv6ResolvabilityConfig{
		global: globalScope,
	}
	configjson.ForEachPeer(bgpCfg, func(name string, peerMap, groupMap map[string]any, origin configjson.PeerOrigin) {
		if err != nil {
			return
		}
		var groupScope srv6ResolvabilityScope
		if groupMap != nil {
			var found bool
			groupScope, found = groupScopes[origin.Group]
			if !found {
				groupScope, err = parseSRv6ResolvabilityScope(groupMap, configPrefixBGPGroup+origin.Group)
				if err != nil {
					return
				}
			}
		}
		var peerScope srv6ResolvabilityScope
		if peerMap != nil {
			peerWhere := "bgp peer " + name
			if origin.Group != "" {
				peerWhere = configPrefixBGPGroup + origin.Group + " peer " + name
			}
			peerScope, err = parseSRv6ResolvabilityScope(peerMap, peerWhere)
			if err != nil {
				return
			}
		}
		rule := srv6ResolvabilityPeerRule{
			group: groupScope,
			peer:  peerScope,
		}
		if rule.isEmpty() {
			return
		}
		key, ok := configjson.KeyFor(name, peerMap, groupMap, origin)
		if !ok {
			return
		}
		key = canonicalPeerConfigKey(key)
		if cfg.peers == nil {
			cfg.peers = make(map[configjson.PeerConfigKey]srv6ResolvabilityPeerRule)
		}
		cfg.peers[key] = rule
	})
	if err != nil {
		return srv6ResolvabilityConfig{}, err
	}
	return cfg, nil
}

func canonicalPeerConfigKey(key configjson.PeerConfigKey) configjson.PeerConfigKey {
	if key.Template {
		return key
	}
	addr, err := netip.ParseAddr(key.ID)
	if err != nil {
		return key
	}
	return configjson.PeerConfigKey{ID: addr.Unmap().String()}
}

func parseSRv6ResolvabilityScope(level map[string]any, where string) (srv6ResolvabilityScope, error) {
	var scope srv6ResolvabilityScope
	raw, exists := level[configKeySRv6Resolvability]
	if exists && raw != nil {
		container, ok := raw.(map[string]any)
		if !ok {
			return srv6ResolvabilityScope{}, invalidSRv6ResolvabilityContainerError(where, raw)
		}

		for key := range container {
			switch key {
			case configKeySRv6Ignore, configKeySRv6Family:
			default:
				return srv6ResolvabilityScope{}, unknownSRv6ResolvabilityKeyError(where, key)
			}
		}

		if ignoreRaw, hasIgnore := container[configKeySRv6Ignore]; hasIgnore {
			if ignoreRaw != nil {
				ignoreVal, valid := parseStrictConfigBool(ignoreRaw)
				if !valid {
					return srv6ResolvabilityScope{}, invalidSRv6ResolvabilityBoolError(where, "", ignoreRaw)
				}
				scope.hasIgnore = true
				scope.ignore = ignoreVal
			}
		}

		if rawFamily, hasFamily := container[configKeySRv6Family]; hasFamily && rawFamily != nil {
			families, err := parseSRv6ResolvabilityFamilies(rawFamily, where)
			if err != nil {
				return srv6ResolvabilityScope{}, err
			}
			scope.families = families
		}
	}
	return scope, nil
}

func parseSRv6ResolvabilityFamilies(rawFamily any, where string) (map[family.Family]bool, error) {
	familyMap, ok := rawFamily.(map[string]any)
	if !ok {
		return nil, invalidSRv6ResolvabilityFamilyListError(where, rawFamily)
	}
	entries := configvalue.ListEntries(familyMap)
	if len(entries) != len(familyMap) {
		for famName, body := range familyMap {
			if _, isMap := body.(map[string]any); !isMap {
				return nil, invalidSRv6ResolvabilityFamilyEntryError(where, famName, body)
			}
		}
	}
	families := make(map[family.Family]bool, len(entries))
	if len(entries) == 0 {
		return families, nil
	}
	for _, entry := range entries {
		fam, ok := parseFamily(entry.Key)
		if !ok {
			return nil, unknownSRv6ResolvabilityFamilyError(where, entry.Key)
		}
		for fieldKey := range entry.Fields {
			switch fieldKey {
			case configKeySRv6Ignore, configKeySRv6FamilyName:
			default:
				return nil, unknownSRv6ResolvabilityFamilyKeyError(where, entry.Key, fieldKey)
			}
		}
		ignoreRaw, hasIgnore := entry.Fields[configKeySRv6Ignore]
		if !hasIgnore {
			return nil, missingSRv6ResolvabilityFamilyIgnoreError(where, entry.Key)
		}
		if ignoreRaw == nil {
			return nil, missingSRv6ResolvabilityFamilyIgnoreError(where, entry.Key)
		}
		ignoreVal, valid := parseStrictConfigBool(ignoreRaw)
		if !valid {
			return nil, invalidSRv6ResolvabilityBoolError(where, entry.Key, ignoreRaw)
		}
		families[fam] = ignoreVal
	}
	return families, nil
}

// parseStrictConfigBool coerces a delivered YANG boolean leaf, accepting only
// JSON bool and exact string "true" or "false" values.
func parseStrictConfigBool(v any) (bool, bool) {
	switch val := v.(type) {
	case bool:
		return val, true
	case string:
		if val == configBoolTrue {
			return true, true
		}
		if val == configBoolFalse {
			return false, true
		}
		return false, false
	// refuse-by-default: guards parseSRv6ResolvabilityScope -- non-boolean config values are rejected at verify rather than defaulted.
	default:
		return false, false
	}
}

func formatRawConfigValue(raw any) string {
	b, err := json.Marshal(raw)
	if err != nil {
		return "<unprintable>"
	}
	return string(b)
}

func srv6ResolvabilityPath(where, famName string) string {
	var tb textbuf.Buffer
	tb.Str(where).Byte(' ').Str(configKeySRv6Resolvability)
	if famName != "" {
		tb.Byte(' ').Str(configKeySRv6Family).Byte(' ').Str(famName)
	}
	return tb.String()
}

func invalidSRv6ResolvabilityContainerError(where string, raw any) error {
	path := srv6ResolvabilityPath(where, "")
	var what, fix textbuf.Buffer
	what.Str(path).Str(configValueSep).Str(formatRawConfigValue(raw)).Str(" is not a container")
	fix.Str(configSetPrefix).Str(path).Str(configSetIgnoreFixSuffix)
	return opererr.New(
		what.String(),
		"The RIB expects srv6-sid-resolvability to hold an ignore leaf or family list entries.",
		fix.String(),
	)
}

func unknownSRv6ResolvabilityKeyError(where, key string) error {
	path := srv6ResolvabilityPath(where, "")
	var what, fix textbuf.Buffer
	what.Str(path).Str(" has unknown option ").Str(key)
	fix.Str("use ignore or family under ").Str(path)
	return opererr.New(
		what.String(),
		"Only ignore and family are valid under srv6-sid-resolvability.",
		fix.String(),
	)
}

func invalidSRv6ResolvabilityBoolError(where, famName string, raw any) error {
	base := srv6ResolvabilityPath(where, famName)
	var what, fix textbuf.Buffer
	what.Str(base).Byte(' ').Str(configKeySRv6Ignore).Str(configValueSep).Str(formatRawConfigValue(raw)).Str(" is not a valid boolean")
	fix.Str(configSetPrefix).Str(base).Str(configSetIgnoreFixSuffix)
	return opererr.New(
		what.String(),
		"SRv6 Service SID resolvability ignore requires an explicit boolean true or false.",
		fix.String(),
	)
}

func invalidSRv6ResolvabilityFamilyListError(where string, raw any) error {
	path := srv6ResolvabilityPath(where, "")
	var what, fix textbuf.Buffer
	what.Str(path).Byte(' ').Str(configKeySRv6Family).Str(configValueSep).Str(formatRawConfigValue(raw)).Str(" is not a family list")
	fix.Str(configSetPrefix).Str(path).Byte(' ').Str(configKeySRv6Family).Str(" ipv4/mpls-vpn ignore true")
	return opererr.New(
		what.String(),
		"The RIB expects family entries keyed by registered address family name.",
		fix.String(),
	)
}

func invalidSRv6ResolvabilityFamilyEntryError(where, famName string, raw any) error {
	path := srv6ResolvabilityPath(where, famName)
	var what, fix textbuf.Buffer
	what.Str(path).Str(configValueSep).Str(formatRawConfigValue(raw)).Str(" is not a family block")
	fix.Str(configSetPrefix).Str(path).Str(configSetIgnoreFixSuffix)
	return opererr.New(
		what.String(),
		"Each srv6-sid-resolvability family entry must specify an ignore boolean leaf.",
		fix.String(),
	)
}

func unknownSRv6ResolvabilityFamilyError(where, famName string) error {
	path := srv6ResolvabilityPath(where, famName)
	base := srv6ResolvabilityPath(where, "")
	var what, fix textbuf.Buffer
	what.Str(path).Str(" is not a registered address family")
	fix.Str(configSetPrefix).Str(base).Byte(' ').Str(configKeySRv6Family).Str(" <registered-family> ignore true (for example ipv4/unicast, ipv6/unicast, ipv4/mpls-vpn, ipv6/mpls-vpn, or l2vpn/evpn)")
	return opererr.New(
		what.String(),
		"The RIB only accepts registered BGP address families in srv6-sid-resolvability.",
		fix.String(),
	)
}

func unknownSRv6ResolvabilityFamilyKeyError(where, famName, key string) error {
	path := srv6ResolvabilityPath(where, famName)
	var what, fix textbuf.Buffer
	what.Str(path).Str(" has unknown option ").Str(key)
	fix.Str(configSetPrefix).Str(path).Str(configSetIgnoreFixSuffix)
	return opererr.New(
		what.String(),
		"Only ignore is valid inside a srv6-sid-resolvability family entry.",
		fix.String(),
	)
}

func missingSRv6ResolvabilityFamilyIgnoreError(where, famName string) error {
	path := srv6ResolvabilityPath(where, famName)
	var what, fix textbuf.Buffer
	what.Str(path).Str(" is missing mandatory leaf ignore")
	fix.Str(configSetPrefix).Str(path).Str(configSetIgnoreFixSuffix)
	return opererr.New(
		what.String(),
		"Each srv6-sid-resolvability family entry must set ignore to true or false.",
		fix.String(),
	)
}
