// Design: docs/architecture/core-design.md — community filter plugin
// RFC: rfc/short/rfc8195.md — Section 3.2, the relation-to-origin tag filter
// RFC: rfc/short/rfc7454.md — Section 11, the own-Global-Administrator scrub
// RFC: rfc/short/rfc7999.md — Section 3.2, the blackhole propagation guard
// Detail: config.go — config parsing for community definitions and filter rules
// Detail: filter.go — ingress filter (direct payload mutation)
// Detail: egress.go — egress filter (ModAccumulator ops)
// Detail: handler.go — AttrModHandlers for progressive build
// Detail: relation.go — RFC 9234 role to RFC 8195 parameter mapping
// Detail: scrub.go — Section 11 keep-list match
// Detail: blackhole.go — RFC 7999 propagation guard

// Package filter_community implements the bgp-filter-community plugin. It
// allows operators to tag and strip BGP communities on ingress and egress
// using named community definitions and cumulative filter rules.
package filter_community

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errFilterCommunityInvalidBgpConfigJson = errors.New("filter-community: invalid bgp config JSON")

var logger = slogutil.LazyLogger("bgp.filter.community")

// state holds the plugin's runtime state, populated via OnConfigure
// callback. Protected by mu for concurrent access from filter closures.
var (
	mu          sync.RWMutex
	definitions communityDefs
	peerConfigs map[string]filterConfig // keyed by peer name
)

// runFilterCommunity runs the community filter plugin using the SDK RPC
// protocol. This is the in-process entry point called via
// InternalPluginRunner.
func runFilterCommunity(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-community", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			bgpCfg, ok := configjson.ParseBGPSubtree(section.Data)
			if !ok {
				return errFilterCommunityInvalidBgpConfigJson
			}
			if err := configureCommunityFilter(bgpCfg); err != nil {
				return fmt.Errorf("filter-community: %w", err)
			}
		}
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"bgp"},
	}); err != nil {
		logger().Error("filter-community plugin failed", "error", err)
		return 1
	}

	return 0
}

// configureCommunityFilter parses community definitions and per-peer filter
// configs from the raw BGP config subtree, accumulating filter tag/strip
// lists across bgp-level, group-level. Peer-level (same inheritance as
// ResolveBGPTree).
func configureCommunityFilter(bgpCfg map[string]any) error {
	defs, err := parseCommunityDefinitions(bgpCfg)
	if err != nil {
		return fmt.Errorf("community definitions: %w", err)
	}

	// BGP-level filter config (applies to all peers as base).
	bgpFilter := parseFilterConfig(bgpCfg)

	// Parse per-peer filter configs, accumulating bgp + group + peer levels.
	configs := make(map[string]filterConfig)
	configjson.ForEachPeer(bgpCfg, func(peerName string, peerMap, groupMap map[string]any, origin configjson.PeerOrigin) {
		// Layer 1: BGP-level defaults.
		fc := bgpFilter

		// Layer 2: Group-level (if peer is in a group).
		if groupMap != nil {
			groupFilter := parseFilterConfig(groupMap)
			fc = mergeFilterConfigs(fc, groupFilter)
		}

		// Layer 3: Peer-level (highest precedence).
		if peerMap != nil {
			peerFilter := parseFilterConfig(peerMap)
			fc = mergeFilterConfigs(fc, peerFilter)
		}

		if !fc.hasAnyRule() {
			return
		}

		// A dynamic group's template is keyed by configjson.CapabilitySelector
		// rather than by its bare name. This map is keyed by peer NAME and read
		// by src.Name / dest.Name below, and a group name and a peer name share no
		// uniqueness check: config.ResolveBGPTree refuses a duplicate PEER name and
		// never compares a group's against it, so `bgp { peer ix {...} group ix
		// {...} }` loads. Under the bare name the group's template would overwrite
		// that peer's entry -- groups are visited after standalone peers -- and the
		// peer would silently get another object's community policy. The "group:"
		// prefix cannot collide: naming.ValidateNodeName accepts no ":".
		//
		// The entry is carried, not yet consumed: the three readers below hold a
		// peer's name and no group identity. Reaching it is the remaining half.
		configs[configjson.CapabilitySelector(peerName, origin)] = fc
	})

	// Validate all referenced community names exist.
	for subject, fc := range configs {
		for _, refs := range [][]string{fc.ingressTag, fc.ingressStrip, fc.egressTag, fc.egressStrip} {
			if err := validateCommunityRefs(defs, refs); err != nil {
				return fmt.Errorf("%s: %w", configLabel(subject), err)
			}
		}
		if err := validateScrubKeepList(fc); err != nil {
			return fmt.Errorf("%s: %w", configLabel(subject), err)
		}
	}

	mu.Lock()
	definitions = defs
	peerConfigs = configs
	mu.Unlock()

	logger().Debug("configured",
		"definitions", len(defs),
		"peers-with-filters", len(configs),
	)

	return nil
}

// configLabel names the config object an error is about, reading the key back.
// A template's key carries the "group:" prefix configjson.CapabilitySelector
// added, and "peer group:ix" would tell the operator to look at a peer that does
// not exist.
func configLabel(key string) string {
	var tb textbuf.Buffer
	if group, found := strings.CutPrefix(key, configjson.GroupKeyPrefix); found {
		return tb.Str("group ").Str(group).String()
	}
	return tb.Str("peer ").Str(key).String()
}

// ingressFilter is the registered IngressFilterFunc at
// filterapi.FilterStagePolicy. Looks up the source peer's filter config and
// applies strip, RFC 7454 Section 11 scrub, RFC 7999 propagation guard,
// then tag.
func ingressFilter(src filterapi.PeerFilterInfo, payload []byte, _ map[string]any) (bool, []byte) {
	mu.RLock()
	defs := definitions
	fc, hasCfg := peerConfigs[src.Name]
	mu.RUnlock()

	if !hasCfg {
		return true, nil
	}

	modified := applyIngressFilter(payload, defs, fc, src.LocalAS, src.PeerAS)
	if modified != nil {
		logger().Info("community ingress applied",
			"peer", src.Name,
			"tag", fc.ingressTag,
			"strip", fc.ingressStrip,
			"scrub-own-ga", fc.scrubEnabled(),
			"blackhole-propagation", fc.blackholeGuardToken(),
		)
	}
	return true, modified
}

// relationIngressFilter is the registered IngressFilterFunc at
// filterapi.FilterStageAnnotation, one priority behind the role plugin.
//
// It is a SECOND registered filter rather than a step inside ingressFilter
// above, and the reason is ordering rather than tidiness. The relation
// parameter derives from what the source peer IS to us, which only the role
// plugin can answer. That plugin publishes the answer from its own ingress
// filter at FilterStageAnnotation, and this plugin's policy-stage filter
// runs before it. A step inside ingressFilter would read a key nothing had
// written yet.
//
// Stage and priority are the declaration that orders the two (register.go).
// filterapi.LessOrder sorts by stage, then priority, then name, so priority
// 1 here sorts after bgp-role's priority 0 in the same stage.
//
// It is also the correct stage on its own terms: FilterStageAnnotation is
// documented as "protocol modifications that stamp routes", which is
// exactly what an RFC 8195 relation tag is.
func relationIngressFilter(src filterapi.PeerFilterInfo, payload []byte, meta map[string]any) (bool, []byte) {
	mu.RLock()
	fc, hasCfg := peerConfigs[src.Name]
	mu.RUnlock()

	if !hasCfg || !fc.relationTagEnabled() {
		return true, nil
	}

	// iBGP carries no customer/peer/provider relation: the source is inside
	// the local AS, so RFC 8195 Section 3.2 has nothing to state about it. A
	// tag written here would be a claim about a relationship that does not
	// exist.
	if src.PeerAS == src.LocalAS {
		return true, nil
	}

	peerRole := relationPeerRoleFromMeta(meta)
	modified := applyRelationTag(payload, fc, src.LocalAS, peerRole)
	if modified != nil {
		logger().Info("community relation tag applied",
			"peer", src.Name,
			"peer-role", peerRole,
			"function", fc.relationFunctionNumber(),
			"parameter", relationParameterFor(peerRole),
		)
	}
	return true, modified
}

// egressFilter is the registered EgressFilterFunc. Looks up the destination
// peer's filter config and accumulates ops.
func egressFilter(_, dest filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
	mu.RLock()
	defs := definitions
	fc, hasCfg := peerConfigs[dest.Name]
	mu.RUnlock()

	if !hasCfg {
		return true
	}

	applyEgressFilter(defs, fc, mods)
	return true // Community filter never suppresses routes.
}
