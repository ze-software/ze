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
	"strings"

	coreenv "github.com/ze-software/ze/internal/core/env"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/redistribute"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/core/bgp/capability"
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

	// Step 2a: derive the process bindings the config's redistribution rules
	// depend on. It runs over `settings`, so a dynamic group's template takes
	// them with the statically configured peers. It runs before the route and
	// filter layers because it reads the config alone.
	if err := wireRedistributeDelivery(tree, settings); err != nil {
		return nil, nil, err
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

	// Step 6: A declared RFC 9234 role carries a config obligation. A peer whose
	// role implies the transit-leak check (RFC 7454 Section 9) and whose matching
	// chain names no filter that can perform it REFUSES the config, so the file
	// states the decision either way. A peer that declares no role is bound by
	// nothing, and Ze says so in one aggregated line rather than passing silently.
	roles := peerRoles(bgpTree, peers, groups)
	if err := validateLeakFilterObligations(roles, filterReg,
		registry.FilterTypesDischarging(filterapi.ObligationTransitLeak)); err != nil {
		return nil, nil, err
	}
	warnPeersWithoutRole(rolelessPeers(roles))

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
// capabilities attach at least one process permitted to put a route on the wire.
// These capabilities require a process to resend routes on demand.
//
// Reads ProcessBinding.MayPushRoutes rather than MaySend(SendUpdate), because
// either rail satisfies the demand: ze builds the UPDATE from the process's
// route operation (`send [ update ]`), or the process hands over a whole message
// it built itself (`send [ raw ]`). The owner added the `raw` word on 2026-08-30
// and ruled that what it carries belongs to this speaker's routing update, so a
// raw-only process answers a route refresh and refills a peer after a restart.
// Naming one rail here would refuse a config that can serve both capabilities.
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
			if b.MayPushRoutes() {
				hasValidProcess = true
				break
			}
		}
		if hasValidProcess {
			continue
		}

		if len(ps.ProcessBindings) == 0 {
			return fmt.Errorf("peer %s: %s requires an attached process with send [ update ] or send [ raw ]\n  the peer attaches no process",
				ps.Address, capName)
		}
		var names []string
		for _, b := range ps.ProcessBindings {
			names = append(names, "attach process "+b.PluginName)
		}
		return fmt.Errorf("peer %s: %s requires an attached process with send [ update ] or send [ raw ]\n  configured: %s - none carry either word",
			ps.Address, capName, textbuf.Join(names, ", "))
	}
	return nil
}

// roleChainObligation says which of a peer's two filter chains must name a
// filter that can catch a transit leak, for one RFC 9234 role.
type roleChainObligation struct {
	importChain bool
	exportChain bool
}

// leakFilterByRole is the obligation matrix RFC 7454 Section 9 implies for each
// RFC 9234 role.
//
// RFC 9234 names the LOCAL speaker's position, so `customer` means the remote
// is our transit provider and `provider` means the remote is our customer. Read
// the other way round, every cell inverts.
//
//   - peer, rs, rs-client: both chains. A settlement-free peer must not hand us
//     transit and must not be handed any, and a route server relays between
//     members that owe each other the same.
//   - provider: import only. The remote is our customer, so a path of theirs
//     running through their other upstream is the leak RFC 7454 Section 9 names.
//     We sell them transit, so everything goes out.
//   - customer: export only. The remote is our upstream and legitimately sends
//     us the whole table. We must not offer it transit for a network we do not
//     carry.
//
// The five keys are the whole role/import enumeration
// (internal/component/bgp/plugins/role/yang/ze-role.yang). A role absent from
// this table obliges nothing, which is the answer a peer that declares no role
// gets: no relationship is stated, so none is implied.
var leakFilterByRole = map[string]roleChainObligation{
	"peer":      {importChain: true, exportChain: true},
	"provider":  {importChain: true},
	"customer":  {exportChain: true},
	"rs":        {importChain: true, exportChain: true},
	"rs-client": {importChain: true, exportChain: true},
}

// peerRole pairs a peer's settings with an RFC 9234 role its config declares.
// The role is NOT on PeerSettings: the role plugin owns the leaf, and the peer
// pipeline reads it off the resolved tree.
type peerRole struct {
	settings *reactor.PeerSettings
	role     string // empty when the peer declares none
}

// peerRoles pairs every configured peer with the role it declares.
//
// A statically configured peer takes its role from its entry in the resolved
// peer map, which ResolveBGPTree deep-merged over the bgp, group and peer
// layers, so a role set on a group binds each of its peers. A dynamic group
// takes its role from the template every member is built from, which binds the
// members the same way. Pairing by the settings pointer rather than by name
// keeps a group and a peer that share a name from taking each other's role.
func peerRoles(bgpTree map[string]any, peers []*reactor.PeerSettings, groups []*reactor.DynamicGroupConfig) []peerRole {
	pairs := make([]peerRole, 0, len(peers)+len(groups))

	peerMap, _ := bgpTree["peer"].(map[string]any)
	for _, ps := range peers {
		resolved, _ := peerMap[ps.Name].(map[string]any)
		pairs = append(pairs, peerRole{settings: ps, role: declaredRole(resolved)})
	}
	for _, dg := range groups {
		pairs = append(pairs, peerRole{settings: dg.Settings, role: declaredRole(dg.Template)})
	}
	return pairs
}

// declaredRole returns an RFC 9234 role a resolved peer map declares, or the
// empty string when it declares none. The leaf is role/import, and that
// `import` is the role sent in the OPEN, never a filter direction.
func declaredRole(resolved map[string]any) string {
	roleMap, ok := resolved["role"].(map[string]any)
	if !ok {
		return ""
	}
	role, _ := roleMap["import"].(string)
	return role
}

// validateLeakFilterObligations refuses a peer whose declared RFC 9234 role
// implies the transit-leak check (RFC 7454 Section 9) and whose matching filter
// chain names no filter that can perform it. Declaring a relationship is what
// binds the peer: the operator states the decision either way, and a reader of
// the config file sees which was made.
//
// declaring holds the filter types that discharge the obligation, read off the
// plugin registry by the caller. This function never names a filter type, so
// the type lives only in the plugin that implements it.
func validateLeakFilterObligations(pairs []peerRole, reg *FilterRegistry, declaring []string) error {
	// GUARD: no filter type in this binary can discharge the obligation, so
	// enforcing it would refuse every config that declares a role. A build with
	// the implementing plugin's feature tag off must still load every config it
	// loaded before. The empty set disables the rule ON PURPOSE; it is not an
	// incidental consequence of an empty loop (ai/rules/principles.md).
	if len(declaring) == 0 {
		return nil
	}

	declaringTypes := make(map[string]bool, len(declaring))
	for _, filterType := range declaring {
		declaringTypes[filterType] = true
	}

	for _, pair := range pairs {
		obligation, bound := leakFilterByRole[pair.role]
		if !bound {
			continue
		}
		missingImport := obligation.importChain && !chainNamesFilterType(pair.settings.ImportFilters, reg, declaringTypes)
		missingExport := obligation.exportChain && !chainNamesFilterType(pair.settings.ExportFilters, reg, declaringTypes)
		if !missingImport && !missingExport {
			continue
		}
		return leakFilterRefusal(pair, missingImport, missingExport, declaring)
	}
	return nil
}

// leakFilterRefusal builds the refusal an operator reads. It names the peer,
// the role that bound it, the chain that is missing the filter, and BOTH ways
// to satisfy the obligation, because a message that reports only what is wrong
// leaves the operator to guess what to write.
func leakFilterRefusal(pair peerRole, missingImport, missingExport bool, declaring []string) error {
	chains := "import and export filter chains"
	switch {
	case !missingExport:
		chains = "import filter chain"
	case !missingImport:
		chains = "export filter chain"
	}
	return fmt.Errorf(
		"peer %s: role %s requires a filter against a transit leak in the %s\n"+
			"  name a filter of type %s in it, or name one prefixed with `inactive:` to record that this session runs without the check",
		pair.settings.Name, pair.role, chains, textbuf.Join(declaring, " or "))
}

// chainNamesFilterType reports whether a filter chain names a filter of one of
// the given types.
//
// A DEACTIVATED ref counts. `inactive:` records a decision the operator made
// about this session, which is what the obligation asks for, and it is the
// reading filterChainContains already takes for a default filter.
func chainNamesFilterType(chain []filterapi.FilterRef, reg *FilterRegistry, types map[string]bool) bool {
	for _, ref := range chain {
		entry, ok := reg.Lookup(filterInstanceName(ref.Name))
		if !ok {
			continue
		}
		if types[entry.Type] {
			return true
		}
	}
	return false
}

// filterInstanceName strips the prefix a chain ref can carry, leaving the
// instance name that keys the policy registry. The plain form carries no
// prefix; the filter-type form, the plugin form and the canonical form each
// carry the instance name after the colon.
func filterInstanceName(ref string) string {
	if _, after, found := strings.Cut(ref, ":"); found {
		return after
	}
	return ref
}

// rolelessPeers names the peers and the dynamic groups that declare no RFC 9234
// role, in config order. They are accepted: the obligation binds only a peer
// that declares a relationship. This is the ONE declaration of that set, read
// by the warning logged at config load and by the `ze doctor` check, so the two
// never enumerate different peers.
func rolelessPeers(pairs []peerRole) []string {
	var names []string
	for _, pair := range pairs {
		if pair.role != "" {
			continue
		}
		if !reportsRoleless(pair.settings) {
			continue
		}
		names = append(names, pair.settings.Name)
	}
	return names
}

// reportsRoleless reports whether a peer that declares no role is named. ONE
// config leaves it out, and the reason is what the report says: an RFC 9234
// role describes an eBGP relationship, so a session Ze KNOWS is iBGP owes no
// role, and naming one would be noise on every route-reflector deployment.
func reportsRoleless(ps *reactor.PeerSettings) bool {
	// A dynamic group states no remote AS. It arrives in the member's OPEN
	// (RFC 4271 Section 4.2), so PeerAS stays 0 on the template
	// (reactor.ParseDynamicGroupTemplate) and the comparison below has nothing
	// to compare: 0 is UNKNOWN here, never iBGP. The session can be eBGP, and a
	// listen range that declares no role is the IXP route-server shape this
	// report exists for.
	if ps.IsDynamic {
		return true
	}
	// A static peer states its remote AS or it is refused: parsePeerFromTree
	// returns ErrIncompleteConfig without it and PeersFromTree drops the peer
	// (reactor/config.go), so 0 on a static peer is a Ze defect rather than an
	// operator's config. Name it instead of dropping it. An unknown AS is not
	// evidence of iBGP, and a peer in the warning is how such a defect reaches
	// a reader at all.
	if ps.PeerAS == 0 {
		return true
	}
	return ps.PeerAS != ps.LocalAS
}

// warnPeersWithoutRole logs ONE line for the whole config, naming how many
// peers and dynamic groups declare no role and the first few by name. One line
// per peer would bury the fact it exists to report on a router with a thousand
// sessions.
func warnPeersWithoutRole(names []string) {
	if len(names) == 0 {
		return
	}
	shown := names
	if len(shown) > rolelessPeersNamed {
		shown = shown[:rolelessPeersNamed]
	}
	configLogger().Warn("peers and dynamic groups declare no RFC 9234 role, so no transit-leak filter is required of them",
		"peers", len(names),
		"first", textbuf.Join(shown, ", "))
}

// rolelessPeersNamed bounds how many peer names the aggregated warning prints.
// The count is always exact; the names are a sample that keeps the line short.
const rolelessPeersNamed = 5

// rolelessPeersFromTree names the peers and the dynamic groups a config
// declares with no RFC 9234 role. It fills the infra seam `ze doctor` reads
// (register.go), so the doctor report and the config-load warning enumerate one
// set.
//
// MUST be called on a CLONE: it prunes inactive nodes from the tree in place,
// exactly as the peer pipeline does.
//
// An unreadable config yields no names rather than an error: a config the
// engine refuses is already reported by doctor's own peer-validation check, and
// naming the same failure twice tells the operator nothing new.
func rolelessPeersFromTree(tree *config.Tree) []string {
	schema, err := config.YANGSchema()
	if err != nil {
		return nil
	}
	config.PruneInactive(tree, schema)

	bgpTree, err := ResolveBGPTree(tree)
	if err != nil {
		return nil
	}
	peers, err := reactor.PeersFromTree(bgpTree)
	if err != nil {
		return nil
	}
	groups, err := dynamicGroupsFromTree(bgpTree)
	if err != nil {
		return nil
	}
	return rolelessPeers(peerRoles(bgpTree, peers, groups))
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
