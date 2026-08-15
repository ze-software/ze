// Design: docs/architecture/config/syntax.md — peer configuration extraction and route expansion
// RFC: rfc/short/rfc2545.md — Section 3, the leaves that feed the global next-hop slot
// RFC: rfc/short/rfc4486.md — per-family prefix limits and the teardown they cause
// Related: loader_prefix.go — prefix expansion for route splitting
// Related: loader_routes.go — BGP route type conversion

package bgpconfig

import (
	"fmt"
	"net/netip"
	"sort"
	"strconv"

	coreenv "github.com/ze-software/ze/internal/core/env"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/redistribute"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/config"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Env var registrations for BGP config overrides are centralized in config/environment.go.

// PeersFromConfigTree builds PeerSettings from a config tree.
// MUST: The input tree is modified in place (inactive nodes are pruned).
// Callers that need the original tree must clone it first.
//
// This replaces the TreeToConfig → configToPeer pipeline by:
//  1. Resolving templates at the map level (ResolveBGPTree)
//  2. Parsing basic peer settings via reactor.PeersFromTree
//  3. Extracting routes from all template layers (globs → templates → peer)
//  4. Applying environment overrides (port)
//
// Routes stay in the config package because they depend on config-internal
// types (StaticRouteConfig, ParseRouteAttributes, etc.) that reactor cannot import.
func PeersFromConfigTree(tree *config.Tree) ([]*reactor.PeerSettings, error) {
	peers, _, err := peersAndDynamicGroups(tree)
	return peers, err
}

// peersAndDynamicGroups builds both peer populations a BGP config declares: the
// statically configured peers, and the template every member of a dynamic group
// is built from (Reactor.SetDynamicGroups).
//
// ONE walk builds both, and that is the point. Every layer a statically
// configured peer takes -- routes, filter chains, loop-detection defaults, the
// cluster-id sync, the port override -- is applied to a dynamic group's template
// by the same line of code. A second walk that read the layers somebody
// remembered is what left an IXP route server's dynamic members with no policy
// at all: until 2026-08-13 no `ImportFilters` assignment was reachable from the
// dynamic path, so no prefix, AS-path, community or IRR filter ever ran for them.
func peersAndDynamicGroups(tree *config.Tree) ([]*reactor.PeerSettings, []*reactor.DynamicGroupConfig, error) {
	// Register BGP redistribute sources (bgp, ibgp, ebgp) for config validation.
	redistribute.RegisterBGPSources()

	// Step 0: Prune inactive containers and list entries.
	// Inactive nodes are treated as if they were not in the config.
	schema, err := config.YANGSchema()
	if err != nil {
		return nil, nil, fmt.Errorf("load schema for inactive pruning: %w", err)
	}
	config.PruneInactive(tree, schema)

	// Step 1: Resolve templates at the map level.
	bgpTree, err := ResolveBGPTree(tree)
	if err != nil {
		return nil, nil, err
	}

	// Step 1a: Validate required fields after inheritance resolution.
	if err := CheckRequiredFields(schema, bgpTree); err != nil {
		return nil, nil, err
	}

	// Step 1b: Apply YANG schema defaults to each peer map and to each dynamic
	// group template. This makes YANG the single source of truth for RFC defaults
	// (hold-time, connect-retry, port, etc.) instead of Go constants.
	applyPeerSchemaDefaults(bgpTree)

	// Step 2: Parse basic peer settings from the resolved map. A dynamic group's
	// template is parsed by the same parser (reactor.ParseDynamicGroupTemplate)
	// and joins `settings` below, so every layer after this point reaches both
	// populations.
	peers, err := reactor.PeersFromTree(bgpTree)
	if err != nil {
		return nil, nil, err
	}
	groups, err := dynamicGroupsFromTree(bgpTree)
	if err != nil {
		return nil, nil, err
	}

	settings := make([]*reactor.PeerSettings, 0, len(peers)+len(groups))
	settings = append(settings, peers...)
	dynByGroup := make(map[string]*reactor.PeerSettings, len(groups))
	for _, dg := range groups {
		settings = append(settings, dg.Settings)
		dynByGroup[dg.GroupName] = dg.Settings
	}

	if len(settings) == 0 {
		return peers, groups, nil
	}

	// Step 3: Extract routes from group and peer layers.
	// Routes accumulate from 2 layers:
	//   Layer 1: Group-level routes (shared by all peers in the group)
	//   Layer 2: Peer's own routes
	bgpContainer := tree.GetContainer("bgp")
	if bgpContainer == nil {
		return peers, groups, nil
	}

	// Build name -> PeerSettings index for matching.
	// The peer list key is now the peer name (not the IP address).
	peerIndex := make(map[string]*reactor.PeerSettings, len(peers))
	for _, ps := range peers {
		peerIndex[ps.Name] = ps
	}

	// Layer 0: BGP-level routes (global defaults for all peers).
	for _, ps := range settings {
		if err := patchRoutes(ps, ps.Name, bgpContainer); err != nil {
			return nil, nil, err
		}
	}

	// Grouped peers: routes from group + peer layers. A dynamic group has no
	// peer layer -- its members are created from the group alone -- so it takes
	// the group's routes and stops there.
	for _, groupEntry := range bgpContainer.GetListOrdered("group") {
		groupTree := groupEntry.Value

		if ds, ok := dynByGroup[groupEntry.Key]; ok {
			if err := patchRoutes(ds, groupEntry.Key, groupTree); err != nil {
				return nil, nil, err
			}
		}

		for _, peerEntry := range groupTree.GetListOrdered("peer") {
			addr := peerEntry.Key
			peerTree := peerEntry.Value

			ps, ok := peerIndex[addr]
			if !ok {
				continue
			}

			// Layer 1: Routes from group defaults.
			if err := patchRoutes(ps, addr, groupTree); err != nil {
				return nil, nil, err
			}

			// Layer 2: Routes from peer's own tree.
			if err := patchRoutes(ps, addr, peerTree); err != nil {
				return nil, nil, err
			}
		}
	}

	// Standalone peers: routes from peer's own tree only.
	for _, peerEntry := range bgpContainer.GetListOrdered("peer") {
		addr := peerEntry.Key
		peerTree := peerEntry.Value

		ps, ok := peerIndex[addr]
		if !ok {
			continue
		}

		if err := patchRoutes(ps, addr, peerTree); err != nil {
			return nil, nil, err
		}
	}

	// Step 3b: Extract filter chains from all layers (cumulative).
	// Like routes, filters accumulate: bgp + group + peer.
	bgpImport, bgpExport := extractFilterChain(bgpContainer)

	for _, groupEntry := range bgpContainer.GetListOrdered("group") {
		groupTree := groupEntry.Value
		groupImport, groupExport := extractFilterChain(groupTree)

		// The dynamic group's own chain is bgp + group, which is the whole
		// import and export policy an IXP route server's members are subject to:
		// they have no peer layer to add to it.
		if ds, ok := dynByGroup[groupEntry.Key]; ok {
			ds.ImportFilters = concatFilters(bgpImport, groupImport)
			ds.ExportFilters = concatFilters(bgpExport, groupExport)
		}

		for _, peerEntry := range groupTree.GetListOrdered("peer") {
			ps, ok := peerIndex[peerEntry.Key]
			if !ok {
				continue
			}
			peerImport, peerExport := extractFilterChain(peerEntry.Value)
			ps.ImportFilters = concatFilters(bgpImport, groupImport, peerImport)
			ps.ExportFilters = concatFilters(bgpExport, groupExport, peerExport)
		}
	}

	for _, peerEntry := range bgpContainer.GetListOrdered("peer") {
		ps, ok := peerIndex[peerEntry.Key]
		if !ok {
			continue
		}
		peerImport, peerExport := extractFilterChain(peerEntry.Value)
		ps.ImportFilters = concatFilters(bgpImport, peerImport)
		ps.ExportFilters = concatFilters(bgpExport, peerExport)
	}

	// Step 3b2: Validate policy filter names in peer filter chains.
	// Names without ":" must exist in the policy registry. Names with ":" are
	// external plugin filters validated at runtime (plugins register at stage 1).
	policyTree := bgpContainer.GetContainer("policy")
	var policySchema *config.ContainerNode
	if node, err := schema.Lookup("bgp/policy"); err == nil {
		if cn, ok := node.(*config.ContainerNode); ok {
			policySchema = cn
		}
	}
	filterReg, err := BuildFilterRegistry(policyTree, policySchema)
	if err != nil {
		return nil, nil, err
	}
	var tb textbuf.Buffer
	for _, ps := range settings {
		tb.Reset()
		if err := filterReg.ValidateFilterNames(ps.ImportFilters, tb.Str("peer ").Str(ps.Name).Str(" import").String()); err != nil {
			return nil, nil, err
		}
		tb.Reset()
		if err := filterReg.ValidateFilterNames(ps.ExportFilters, tb.Str("peer ").Str(ps.Name).Str(" export").String()); err != nil {
			return nil, nil, err
		}
	}

	// Step 3b2a: Canonicalize chain refs to the full `<plugin-process>:<filter>`
	// form. Users can write any of:
	//   - `filter import [ bgp-filter-prefix:CUSTOMERS ]`  (explicit plugin)
	//   - `filter import [ prefix-list:CUSTOMERS ]`        (filter-type prefix)
	//   - `filter import [ CUSTOMERS ]`                    (plain name)
	// All three resolve to the same dispatch target at runtime; this step
	// rewrites them into the canonical form before peer settings are frozen.
	for _, ps := range settings {
		ps.ImportFilters = canonicalizeFilterRefs(ps.ImportFilters, filterReg)
		ps.ExportFilters = canonicalizeFilterRefs(ps.ExportFilters, filterReg)
	}

	// Step 3b3: Prepend default filter names to each peer's import chain.
	// Default filters (loop-detection) auto-populate unless already referenced.
	prependDefaultFilters(bgpContainer, settings)

	// Step 3c: Extract loop-detection policy settings into PeerSettings.
	// For each peer, check if any import filter references a loop-detection entry
	// in the policy section. If so, apply allow-own-as and cluster-id to the peer.
	applyLoopDetectionConfig(bgpContainer, settings)

	// Step 3d: Sync session/cluster-id with loop-detection/cluster-id.
	// RFC 4456: the same cluster-id must be used for both egress (CLUSTER_LIST prepend)
	// and ingress (CLUSTER_LIST loop check). If only one is configured, propagate it.
	for _, ps := range settings {
		if ps.ClusterID != 0 && ps.LoopClusterID == 0 {
			ps.LoopClusterID = ps.ClusterID
		} else if ps.LoopClusterID != 0 && ps.ClusterID == 0 {
			ps.ClusterID = ps.LoopClusterID
		}
	}

	// Step 4: Apply port override from ze.test.bgp.port env var (test infrastructure).
	applyPortOverride(settings)

	// Step 5: Validate connection mode.
	for _, ps := range settings {
		if !ps.Connection.Connect && !ps.Connection.Accept {
			return nil, nil, fmt.Errorf("peer %s: connect and accept cannot both be false", ps.Name)
		}
	}

	// Step 5: Validate capability-process constraints.
	if err := validatePeerProcessCaps(settings); err != nil {
		return nil, nil, err
	}

	return peers, groups, nil
}

// dynamicGroupsFromTree extracts dynamic group configs from the resolved BGP tree.
// Returns nil if no dynamic groups are configured.
//
// The template's PeerSettings comes from reactor.ParseDynamicGroupTemplate, the
// parser that reads a statically configured peer. The group's config tree has the
// same shape as a peer's (resolveDynamicGroup, resolve.go), so the two share one
// parser and a leaf added to it reaches both.
func dynamicGroupsFromTree(bgpTree map[string]any) ([]*reactor.DynamicGroupConfig, error) {
	raw, ok := bgpTree["dynamic-groups"]
	if !ok {
		return nil, nil
	}
	templates, ok := raw.([]DynamicGroupTemplate)
	if !ok {
		return nil, nil
	}

	var localAS uint32
	if sessionMap, ok := bgpTree["session"].(map[string]any); ok {
		if asnMap, ok := sessionMap["asn"].(map[string]any); ok {
			if tv, ok := asnMap["local"].(string); ok {
				if n, err := strconv.ParseUint(tv, 10, 32); err == nil {
					localAS = uint32(n)
				}
			}
		}
	}

	var routerID uint32
	if v, ok := bgpTree["router-id"].(string); ok {
		if addr, err := netip.ParseAddr(v); err == nil {
			routerID = ipToUint32(addr)
		}
	}

	groups := make([]*reactor.DynamicGroupConfig, 0, len(templates))
	for _, tmpl := range templates {
		ps, err := reactor.ParseDynamicGroupTemplate(tmpl.GroupName, tmpl.Template, localAS, routerID)
		if err != nil {
			return nil, fmt.Errorf("bgp/group %s: %w", tmpl.GroupName, err)
		}

		groups = append(groups, &reactor.DynamicGroupConfig{
			GroupName: tmpl.GroupName,
			Ranges:    tmpl.Ranges,
			MaxPeers:  tmpl.MaxPeers,
			Settings:  ps,
			Template:  tmpl.Template,
		})
	}
	return groups, nil
}

// applyPeerSchemaDefaults applies YANG defaults to each peer entry in the resolved
// BGP tree, and to each dynamic group's template.
// This makes YANG the single source of truth for defaults (RFC hold-time, port, etc.)
// instead of duplicating them as Go constants in NewPeerSettings.
//
// The dynamic group takes the PEER schema's defaults, because its template
// becomes a peer: every member of the group is built from it. A group left out
// of this walk takes whatever NewPeerSettings happens to state instead, so a
// YANG default corrected in one place would reach the static peer alone.
func applyPeerSchemaDefaults(bgpTree map[string]any) {
	schema, err := config.YANGSchema()
	if err != nil {
		return
	}
	// Navigate to the peer ListNode in the schema (bgp > peer).
	peerSchema, err := schema.Lookup("bgp/peer")
	if err != nil {
		return
	}

	if peerMap, ok := bgpTree["peer"].(map[string]any); ok {
		for _, v := range peerMap {
			if entry, ok := v.(map[string]any); ok {
				config.ApplyDefaults(entry, peerSchema)
			}
		}
	}

	if templates, ok := bgpTree["dynamic-groups"].([]DynamicGroupTemplate); ok {
		for _, tmpl := range templates {
			config.ApplyDefaults(tmpl.Template, peerSchema)
		}
	}
}

// patchRoutes extracts routes from a peer's *Tree and patches them into PeerSettings.
func patchRoutes(ps *reactor.PeerSettings, addr string, peerTree *config.Tree) error {
	// Extract routes from peer's own tree.
	routes, err := extractRoutesFromTree(peerTree)
	if err != nil {
		return fmt.Errorf("peer %s routes: %w", addr, err)
	}

	// Convert and patch static routes.
	if err := patchStaticRoutes(ps, routes.StaticRoutes, addr); err != nil {
		return err
	}

	// Convert and patch generic plugin routes (native update{} nlri form).
	for i := range routes.PluginRoutes {
		route, err := convertPluginRoute(routes.PluginRoutes[i])
		if err != nil {
			return fmt.Errorf("peer %s plugin route %s: %w", addr, routes.PluginRoutes[i].Family, err)
		}
		ps.PluginRoutes = append(ps.PluginRoutes, route)
	}

	// Legacy ExaBGP flow{} syntax: route through the flowspec plugin's parser.
	for _, fr := range extractFlowSpecRoutes(peerTree) {
		prc, err := flowSpecConfigToPlugin(fr)
		if err != nil {
			return fmt.Errorf("peer %s flowspec route: %w", addr, err)
		}
		route, err := convertPluginRoute(prc)
		if err != nil {
			return fmt.Errorf("peer %s flowspec route: %w", addr, err)
		}
		ps.PluginRoutes = append(ps.PluginRoutes, route)
	}

	return nil
}

// patchStaticRoutes converts StaticRouteConfig to reactor.StaticRoute and adds them to PeerSettings.
func patchStaticRoutes(ps *reactor.PeerSettings, routes []StaticRouteConfig, addr string) error {
	for i := range routes {
		sr := &routes[i]
		attrs, err := ParseRouteAttributes(sr)
		if err != nil {
			return fmt.Errorf("peer %s static route %s: %w", addr, sr.Prefix, err)
		}

		// Create RouteNextHop from config.
		var nextHop bgptypes.RouteNextHop
		if sr.NextHopSelf {
			nextHop = bgptypes.NewNextHopSelf()
		} else if attrs.NextHop.IsValid() {
			nextHop = bgptypes.NewNextHopExplicit(attrs.NextHop)
		}

		// Convert raw attributes.
		var rawAttrs []reactor.RawAttribute
		for _, ra := range attrs.RawAttributes {
			rawAttrs = append(rawAttrs, reactor.RawAttribute{
				Code:  ra.Code,
				Flags: ra.Flags,
				Value: ra.Value,
			})
		}

		// Handle split: expand prefix into more-specific prefixes.
		prefixes := []netip.Prefix{attrs.Prefix}
		if splitLen := parseSplitLen(sr.Split); splitLen > 0 {
			prefixes = expandPrefix(attrs.Prefix, splitLen)
		}

		// Create a route for each prefix (usually just one, unless split).
		for _, prefix := range prefixes {
			labels := make([]uint32, len(attrs.Labels))
			for i, l := range attrs.Labels {
				labels[i] = uint32(l)
			}

			route := reactor.StaticRoute{
				Prefix:            prefix,
				NextHop:           nextHop,
				Origin:            uint8(attrs.Origin),
				LocalPreference:   attrs.LocalPreference,
				MED:               attrs.MED,
				Communities:       attrs.Community.Values,
				LargeCommunities:  attrs.LargeCommunity.Values,
				ExtCommunity:      attrs.ExtendedCommunity.Raw,
				ExtCommunityBytes: sortExtCommunities(attrs.ExtendedCommunity.Bytes),
				PathID:            uint32(attrs.PathID),
				Labels:            labels,
				RD:                attrs.RD.Raw,
				RDBytes:           attrs.RD.Bytes,
				ASPath:            attrs.ASPath.Values,
				AggregatorASN:     attrs.Aggregator.ASN,
				AggregatorIP:      attrs.Aggregator.IP,
				HasAggregator:     attrs.Aggregator.Valid,
				AtomicAggregate:   attrs.AtomicAggregate,
				OriginatorID:      attrs.OriginatorID,
				ClusterList:       attrs.ClusterList,
				AIGPMetric:        attrs.AIGPMetric,
				PrefixSIDBytes:    attrs.PrefixSID.Bytes,
				RawAttributes:     rawAttrs,
			}

			// RFC 4364: VPN routes require at least one label.
			if route.IsVPN() && len(route.Labels) == 0 {
				return fmt.Errorf("peer %s VPN route %s requires at least one label", addr, prefix)
			}

			ps.StaticRoutes = append(ps.StaticRoutes, route)
		}
	}

	return nil
}

// validatePeerProcessCaps checks that peers with route-refresh or graceful-restart
// capabilities attach at least one process permitted to send updates.
// These capabilities require a process to resend routes on demand.
func validatePeerProcessCaps(peers []*reactor.PeerSettings) error {
	for _, ps := range peers {
		needsProcess := false
		capName := ""
		for _, cap := range ps.Capabilities {
			switch cap.Code() { //nolint:exhaustive // only route-refresh and GR require process bindings
			case capability.CodeRouteRefresh:
				needsProcess = true
				if capName == "" {
					capName = "route-refresh"
				}
			case capability.CodeGracefulRestart:
				needsProcess = true
				capName = "graceful-restart"
			}
		}
		// Graceful-restart is stored in RawCapabilityConfig (built by GR plugin at runtime),
		// not as a capability.Capability in the slice.
		if _, ok := ps.RawCapabilityConfig["graceful-restart"]; ok {
			needsProcess = true
			capName = "graceful-restart"
		}
		if !needsProcess {
			continue
		}

		hasValidProcess := false
		for _, b := range ps.ProcessBindings {
			if b.MaySend(bgpevents.SendUpdate) {
				hasValidProcess = true
				break
			}
		}
		if hasValidProcess {
			continue
		}

		if len(ps.ProcessBindings) == 0 {
			return fmt.Errorf("peer %s: %s requires an attached process with send [ update ]\n  the peer attaches no process",
				ps.Address, capName)
		}
		var names []string
		for _, b := range ps.ProcessBindings {
			names = append(names, "attach process "+b.PluginName)
		}
		return fmt.Errorf("peer %s: %s requires an attached process with send [ update ]\n  configured: %s - none have send [ update ]",
			ps.Address, capName, textbuf.Join(names, ", "))
	}
	return nil
}

// applyLoopDetectionConfig extracts loop-detection policy settings and applies them
// to peers whose import filter chains reference the filter instance by name.
// Each loop-detection entry in bgp > policy > loop-detection has allow-own-as and cluster-id
// leaves. When a peer's import filter chain contains the entry's name, its PeerSettings
// receives the corresponding values.
func applyLoopDetectionConfig(bgpContainer *config.Tree, peers []*reactor.PeerSettings) {
	policyTree := bgpContainer.GetContainer("policy")
	if policyTree == nil {
		return
	}

	ldEntries := policyTree.GetList("loop-detection")
	if len(ldEntries) == 0 {
		return
	}

	for _, ps := range peers {
		for _, ref := range ps.ImportFilters {
			entry, ok := ldEntries[ref.Name]
			if !ok {
				continue
			}

			// If the loop-detection filter is deactivated, suppress the
			// in-process LoopIngress for this peer.
			if ref.Inactive {
				ps.LoopDisabled = true
				break
			}

			// Extract allow-own-as (uint8, default 0).
			if v, ok := entry.Get("allow-own-as"); ok {
				n, err := strconv.ParseUint(v, 10, 8)
				if err == nil {
					ps.LoopAllowOwnAS = uint8(n)
				}
			}

			// Extract cluster-id (IPv4 address -> uint32).
			if v, ok := entry.Get("cluster-id"); ok {
				ip, err := netip.ParseAddr(v)
				if err == nil {
					ps.LoopClusterID = ipToUint32(ip)
				}
			}

			// First matching loop-detection entry wins for this peer.
			break
		}
	}
}

// prependDefaultFilters adds default filter names to each peer's import chain
// if not already present (explicitly or as inactive:). Default filters come from
// loop-detection entries in the policy section. Each entry's name is prepended
// to ImportFilters so loop detection runs first in the chain.
func prependDefaultFilters(bgpContainer *config.Tree, peers []*reactor.PeerSettings) {
	policyTree := bgpContainer.GetContainer("policy")
	if policyTree == nil {
		return
	}

	ldEntries := policyTree.GetList("loop-detection")
	if len(ldEntries) == 0 {
		return
	}

	// Collect default filter names (all loop-detection entries), sorted for deterministic order.
	defaults := make([]string, 0, len(ldEntries))
	for name := range ldEntries {
		defaults = append(defaults, name)
	}
	sort.Strings(defaults)

	for _, ps := range peers {
		for _, dflt := range defaults {
			if filterChainContains(ps.ImportFilters, dflt) {
				continue
			}
			ps.ImportFilters = append([]filterapi.FilterRef{{Name: dflt}}, ps.ImportFilters...)
		}
	}
}

// filterChainContains checks if a filter chain contains a name, regardless of
// the ref's deactivation state.
func filterChainContains(chain []filterapi.FilterRef, name string) bool {
	for _, entry := range chain {
		if entry.Name == name {
			return true
		}
	}
	return false
}

// portOverrideFromEnv returns the runtime-only BGP port override used by test
// infrastructure.
func portOverrideFromEnv() (uint16, bool) {
	p := coreenv.Get(envKeyTCPPort)
	if p == "" {
		return 0, false
	}
	v, err := strconv.ParseUint(p, 10, 16)
	if err != nil {
		return 0, false
	}
	return uint16(v), true //nolint:gosec // Validated above
}

// applyPortOverride overrides peer remote port from ze.test.bgp.port env var.
// This is a runtime-only mechanism for the test infrastructure (not YANG config).
func applyPortOverride(peers []*reactor.PeerSettings) {
	port, ok := portOverrideFromEnv()
	if !ok {
		return
	}
	for _, ps := range peers {
		ps.Port = port
	}
}
