// Design: docs/architecture/config/syntax.md — peer configuration extraction and route expansion
// RFC: rfc/short/rfc2545.md — Section 3, the leaves that feed the global next-hop slot
// RFC: rfc/short/rfc4486.md — per-family prefix limits and the teardown they cause
// Related: loader_prefix.go — prefix expansion for route splitting
// Related: loader_routes.go — BGP route type conversion

package bgpconfig

import (
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/reactor/loopdetect"
	"github.com/ze-software/ze/internal/component/bgp/redistribute"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/policy"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
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
// is built from. Boot installs the groups with Reactor.SetDynamicGroups; a reload
// hands them to the reactor's journaled dynamic step instead.
//
// ONE walk builds both, and that is the point. Every layer a statically
// configured peer takes -- routes, filter chains, loop-detection defaults, the
// cluster-id sync, the port override -- is applied to a dynamic group's template
// by the same line of code. A second walk that read the layers somebody
// remembered is what left an IXP route server's dynamic members with no policy
// at all: until 2026-08-13 no import-policy assignment was reachable from the
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
	if err := validateFilterPolicies(tree); err != nil {
		return nil, nil, err
	}

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

	// Resolve named ownership while each layer still has its written location.
	policyTree := tree.GetContainer(policy.RootName)
	filterReg, err := policyRegistryOf(tree, schema)
	if err != nil {
		return nil, nil, err
	}

	// Step 3b: Extract filter chains from all layers (cumulative).
	// Like routes, filters accumulate: bgp + group + peer.
	bgpImport, bgpExport, err := extractFilterPolicies(bgpContainer, "bgp")
	if err != nil {
		return nil, nil, err
	}
	if err := validateExportPolicyReferences(bgpExport, filterReg, "bgp"); err != nil {
		return nil, nil, err
	}

	for _, groupEntry := range bgpContainer.GetListOrdered("group") {
		groupTree := groupEntry.Value
		groupPeers := groupTree.GetListOrdered("peer")
		if len(groupPeers) == 0 {
			if _, dynamic := dynByGroup[groupEntry.Key]; !dynamic {
				continue
			}
		}
		groupImport, groupExport, err := extractFilterPolicies(groupTree, "bgp/group/"+groupEntry.Key)
		if err != nil {
			return nil, nil, err
		}
		if err := validateExportPolicyReferences(groupExport, filterReg, "bgp", "group", groupEntry.Key); err != nil {
			return nil, nil, err
		}

		// The dynamic group's own chain is bgp + group, which is the whole
		// import and export policy an IXP route server's members are subject to:
		// they have no peer layer to add to it.
		if ds, ok := dynByGroup[groupEntry.Key]; ok {
			ds.ImportPolicy = filterapi.CompilePolicy(policy.ConcatChains(bgpImport, groupImport))
			ds.ExportPolicy = filterapi.CompilePolicy(policy.ConcatChains(bgpExport, groupExport))
		}

		for _, peerEntry := range groupPeers {
			ps, ok := peerIndex[peerEntry.Key]
			if !ok {
				continue
			}
			peerImport, peerExport, err := extractFilterPolicies(peerEntry.Value, "bgp/group/"+groupEntry.Key+"/peer/"+peerEntry.Key)
			if err != nil {
				return nil, nil, err
			}
			if err := validateExportPolicyReferences(peerExport, filterReg, "bgp", "group", groupEntry.Key, "peer", peerEntry.Key); err != nil {
				return nil, nil, err
			}
			ps.ImportPolicy = filterapi.CompilePolicy(policy.ConcatChains(bgpImport, groupImport, peerImport))
			ps.ExportPolicy = filterapi.CompilePolicy(policy.ConcatChains(bgpExport, groupExport, peerExport))
		}
	}

	for _, peerEntry := range bgpContainer.GetListOrdered("peer") {
		ps, ok := peerIndex[peerEntry.Key]
		if !ok {
			continue
		}
		peerImport, peerExport, err := extractFilterPolicies(peerEntry.Value, "bgp/peer/"+peerEntry.Key)
		if err != nil {
			return nil, nil, err
		}
		if err := validateExportPolicyReferences(peerExport, filterReg, "bgp", "peer", peerEntry.Key); err != nil {
			return nil, nil, err
		}
		ps.ImportPolicy = filterapi.CompilePolicy(policy.ConcatChains(bgpImport, peerImport))
		ps.ExportPolicy = filterapi.CompilePolicy(policy.ConcatChains(bgpExport, peerExport))
	}

	// Step 3b2: Validate policy filter names in peer filter chains.
	// Names without ":" must exist in the policy registry. Names with ":" are
	// external plugin filters validated at runtime (plugins register at stage 1).
	// A dynamic group's name holding a variable is judged per peer once the
	// peer resolves it (checkTemplatedMember, templatedChain).
	var tb textbuf.Buffer
	for _, ps := range settings {
		tb.Reset()
		if err := validateChainNames(filterReg, ps.ImportPolicy.Filters(), ps.IsDynamic, "bgp import", tb.Str("peer ").Str(ps.Name).Str(" import filters").String()); err != nil {
			return nil, nil, err
		}
		tb.Reset()
		if err := validateChainNames(filterReg, ps.ExportPolicy.Filters(), ps.IsDynamic, attachmentExport, tb.Str("peer ").Str(ps.Name).Str(" export filters").String()); err != nil {
			return nil, nil, err
		}
	}

	// Step 3b2a: Canonicalize chain refs to the full `<plugin-process>:<filter>`
	// form. Users can write any of:
	//   - `filters [ bgp-filter-prefix:CUSTOMERS ]`  (explicit plugin)
	//   - `filters [ prefix-list:CUSTOMERS ]`        (filter-type prefix)
	//   - `filters [ CUSTOMERS ]`                    (plain name)
	// All three resolve to the same dispatch target at runtime; this step
	// rewrites them into the canonical form before peer settings are frozen.
	for _, ps := range settings {
		ps.ImportPolicy = ps.ImportPolicy.WithBoundFilters(canonicalizeFilterRefs(ps.ImportPolicy.Filters(), filterReg))
		ps.ExportPolicy = ps.ExportPolicy.WithBoundFilters(canonicalizeFilterRefs(ps.ExportPolicy.Filters(), filterReg))
	}

	// Step 3b3: Insert the members plugins declare for the peers whose config
	// needs them (filter_insertion.go), then prepend the operator's default
	// filters (loop-detection) unless already referenced, so loop detection
	// stays at the head of every import chain.
	conditional, err := insertDeclaredFilters(bgpTree, peers, groups, filterReg)
	if err != nil {
		return nil, nil, err
	}
	prependDefaultFilters(policyTree, settings, filterReg)
	// The plugin whose member a peer's chain carries is fed that peer's
	// state, so its removal reaches the plugin (filter_state_binding.go).
	if err := grantFilterOwnersState(tree, settings); err != nil {
		return nil, nil, err
	}

	// Step 3b4: Compile the chains of the members that evaluate in process
	// over the route view, after every ref is canonical and bound.
	bases, err := parseAttributeBases(tree, schema)
	if err != nil {
		return nil, nil, err
	}
	if err := compileLibraryChains(settings, conditional, filterReg, bases); err != nil {
		return nil, nil, err
	}

	// Step 3c: Extract loop-detection policy settings into PeerSettings.
	// For each peer, check if any import filter references a loop-detection entry
	// in the policy section. If so, apply allow-own-as and cluster-id to the peer.
	applyLoopDetectionConfig(policyTree, settings)

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

	// Step 0a: Refuse a `bgp update-delay` container the reactor could not honor.
	// The VALUE is read in CreateReactorFromTree; this call is here for the
	// ERROR alone, because this walk is the door `ze config validate` and `ze
	// doctor` reach (register.go, validatePeersFromTree) and BGP is excluded
	// from the generic custom-validator walk. Without it an operator's verify
	// accepts a config the daemon refuses at startup.
	if _, err := ParseUpdateDelay(tree); err != nil {
		return nil, nil, err
	}

	warnStrictPeersWithoutEngine(peers, tree)

	return peers, groups, nil
}

// policyRegistryOf builds the namespace of filter instances from the
// top-level policy root, which every protocol with a filter attachment
// shares. A config with no policy root still gets a registry: it holds no
// instance, so every plain chain name is refused by name.
func policyRegistryOf(tree *config.Tree, schema *config.Schema) (*policy.Registry, error) {
	node, err := schema.Lookup(policy.RootName)
	if err != nil {
		return nil, fmt.Errorf("policy registry: %w", err)
	}
	policySchema, ok := node.(*config.ContainerNode)
	if !ok {
		return nil, fmt.Errorf("policy registry: the %s schema node is not a container", policy.RootName)
	}
	return policy.BuildRegistry(tree.GetContainer(policy.RootName), policySchema)
}

// validateChainNames refuses a chain naming no filter instance under the
// policy root, an instance whose filter type no BGP filter serves, and an
// instance that reads or sets a route field the BGP attachment does not carry
// or allow (policy.CheckChain at attachment, "bgp import" or "bgp export").
// A reference to an external plugin's filter gets the same judgement of the
// fields that plugin declared at Stage 1 (checkExternalChainRef).
// A deactivated reference is checked like an active one, because deactivation
// is a flag on policy.Ref and not part of its name. A reference naming its
// filter through a config variable is judged by checkTemplatedMember, which
// refuses it unless dynamic (the chain is a dynamic group's template).
func validateChainNames(reg *policy.Registry, refs []policy.Ref, dynamic bool, attachment, context string) error {
	typesMap := registry.FilterTypesMap()
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		if isTemplatedMember(ref.Name) {
			if err := checkTemplatedMember(reg, ref.Name, dynamic, attachment, context); err != nil {
				return err
			}
			continue
		}
		if err := reg.ValidateName(ref.Name, context); err != nil {
			return err
		}
		entry, ok := policyChainInstance(reg, ref, typesMap)
		if !ok {
			// An external plugin's filter: judged against what the plugin
			// declared at Stage 1, now or when it declares (D-10).
			if err := checkExternalChainRef(ref.Name, attachment, context); err != nil {
				return err
			}
			continue
		}
		if err := bgpServesFilterType(ref, entry, typesMap, attachment, context); err != nil {
			return err
		}
		names = append(names, entry.Name)
	}
	if err := policy.CheckChain(attachment, reg, names); err != nil {
		return fmt.Errorf("%s: %w", context, err)
	}
	return nil
}

// policyChainInstance answers the policy instance a BGP chain reference names:
// a plain name, or one qualified by its filter type or by the plugin serving
// that type. A reference qualified by any other prefix is an external
// plugin's filter, so it answers false.
func policyChainInstance(reg *policy.Registry, ref policy.Ref, typesMap map[string]string) (policy.Instance, bool) {
	entry, ok := reg.Lookup(filterInstanceName(ref.Name))
	if !ok {
		return policy.Instance{}, false
	}
	prefix, _, qualified := strings.Cut(ref.Name, ":")
	if qualified && !namesFilterType(prefix, entry.Type, typesMap) {
		return policy.Instance{}, false
	}
	return entry, true
}

// namesFilterType reports whether a chain reference prefix names filterType
// itself or the plugin that registered it.
func namesFilterType(prefix, filterType string, typesMap map[string]string) bool {
	if prefix == filterType {
		return true
	}
	plugin, served := typesMap[filterType]
	return served && plugin == prefix
}

// bgpServesFilterType refuses an instance whose filter type no BGP filter
// serves: neither the engine's loop step (loopdetect.BindLoopDetection), a plugin
// that registered the type, nor an in-process evaluator over the route view
// (policychain.HasEvaluator). Canonicalization would otherwise pass the bare
// name through to a plugin that does not exist, and the filter would never run.
func bgpServesFilterType(ref policy.Ref, entry policy.Instance, typesMap map[string]string, attachment, context string) error {
	if _, inProcess := loopdetect.BindLoopDetection(ref, entry.Type); inProcess {
		return nil
	}
	if _, served := typesMap[entry.Type]; served {
		return nil
	}
	if policychain.HasEvaluator(entry.Type) {
		return nil // evaluated in process over the route view (compileLibraryChains)
	}
	return fmt.Errorf("%s: filter %q is a %s instance, and no BGP filter serves the %s type, so it cannot run at %s; "+
		"attach it to a Loc-RIB chain (rib, static, ospf, isis or connected filter import) and remove it from this chain",
		context, entry.Name, entry.Type, entry.Type, attachment)
}

// validateExportPolicyReferences checks an original active layer before merging
// or canonicalizing loses the location and spelling the editor must deactivate.
// Only layers inherited by a static peer or dynamic template reach this check.
func validateExportPolicyReferences(chain policy.Chain, reg *policy.Registry, path ...string) error {
	for _, ref := range chain.Refs() {
		if ref.Inactive {
			continue
		}
		entry, ok := reg.Lookup(filterInstanceName(ref.Name))
		if !ok {
			continue
		}
		bound, inProcess := loopdetect.BindLoopDetection(ref, entry.Type)
		if !inProcess {
			continue
		}
		location := config.FormatEditPath(append(path, "filter", "export", "filters", ref.Name))
		return fmt.Errorf("%s: export filter %s is only supported on import; its ingress check does not run on export.\nTo keep this reference inactive, run in the configuration editor:\ntop\ndeactivate %s\ncommit now", location, bound.Name, location)
	}
	return nil
}

// warnStrictPeersWithoutEngine logs ONE line naming every peer that asks for
// BFD strict mode where the configuration declares no BFD engine.
//
// It is the config-load half of what bfd_strict_doctor.go reports, so the two
// enumerate one set, and it exists because the consequence is invisible on the
// peer's own surface: draft-ietf-idr-bgp-bfd-strict-mode Section 8.5.5 holds
// such a peer in OpenSent for the life of the process, and an operator reading
// `show bgp peer list` sees only a session that will not come up.
//
// It WARNS rather than refuses. A configuration whose BFD block arrives in a
// later commit is a real operator sequence, and refusing the file would make
// the order of two edits load-bearing.
func warnStrictPeersWithoutEngine(peers []*reactor.PeerSettings, tree *config.Tree) {
	if len(peers) == 0 || tree == nil {
		return
	}
	// The BFD engine is a TOP-LEVEL block, so the bgp subtree cannot answer for
	// it and the whole tree is read instead.
	if tree.GetContainer("bfd") != nil {
		return
	}
	names := make([]string, 0, len(peers))
	for _, ps := range peers {
		if ps.BFD == nil || !ps.BFD.Enabled || !ps.BFD.Strict {
			continue
		}
		name := ps.Name
		if name == "" {
			name = ps.Address.String()
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return
	}
	shown := names
	if len(shown) > strictPeersNamed {
		shown = shown[:strictPeersNamed]
	}
	configLogger().Warn("peers ask for BFD strict mode and no BFD engine is configured, so they will never establish",
		"peers", len(names),
		"first", textbuf.Join(shown, ", "),
		"action", "add a top-level bfd { enabled true; ... } block, or remove strict true from the peer")
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
				Prefix:             prefix,
				NextHop:            nextHop,
				Origin:             uint8(attrs.Origin),
				LocalPreference:    attrs.LocalPreference,
				HasLocalPreference: attrs.HasLocalPreference,
				MED:                attrs.MED,
				HasMED:             attrs.HasMED,
				Communities:        attrs.Community.Values,
				LargeCommunities:   attrs.LargeCommunity.Values,
				ExtCommunity:       attrs.ExtendedCommunity.Raw,
				ExtCommunityBytes:  sortExtCommunities(attrs.ExtendedCommunity.Bytes),
				PathID:             uint32(attrs.PathID),
				Labels:             labels,
				RD:                 attrs.RD.Raw,
				RDBytes:            attrs.RD.Bytes,
				ASPath:             attrs.ASPath.Values,
				AggregatorASN:      attrs.Aggregator.ASN,
				AggregatorIP:       attrs.Aggregator.IP,
				HasAggregator:      attrs.Aggregator.Valid,
				AtomicAggregate:    attrs.AtomicAggregate,
				OriginatorID:       attrs.OriginatorID,
				ClusterList:        attrs.ClusterList,
				AIGPMetric:         attrs.AIGPMetric,
				PrefixSIDBytes:     attrs.PrefixSID,
				RawAttributes:      rawAttrs,
			}

			// RFC 4364: VPN routes require at least one label.
			if route.IsVPN() && len(route.Labels) == 0 {
				return fmt.Errorf("peer %s VPN route %s requires at least one label", addr, prefix)
			}

			// RFC 9252 Section 7: no transposition where the route family has no label
			// field, and the family is known only once the route is built.
			if err := attribute.CheckSRv6TranspositionForFamily(route.PrefixSIDBytes, route.Family()); err != nil {
				return fmt.Errorf("peer %s static route %s: %w", addr, prefix, err)
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
			//exhaustive:ignore // Only route-refresh and graceful-restart require a route-pushing process binding.
			switch cap.Code() {
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

// strictPeersNamed bounds how many peer names the aggregated BFD-strict
// warning prints. The count is always exact; the names are a sample that keeps
// the line short.
const strictPeersNamed = 5

// applyLoopDetectionConfig extracts loop-detection policy settings and applies them
// to peers whose import filter chains reference the filter instance by name.
// Each loop-detection entry in policy > loop-detection has allow-own-as and cluster-id
// leaves. When a peer's import filter chain contains the entry's name, its PeerSettings
// receives the corresponding values.
func applyLoopDetectionConfig(policyTree *config.Tree, peers []*reactor.PeerSettings) {
	if policyTree == nil {
		return
	}

	ldEntries := policyTree.GetList("loop-detection")
	if len(ldEntries) == 0 {
		return
	}

	for _, ps := range peers {
		for i, ref := range ps.ImportPolicy.Filters() {
			if !ps.ImportPolicy.InProcess(i) {
				continue
			}
			entry, ok := ldEntries[filterInstanceName(ref.Name)]
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
// to ImportPolicy so loop detection runs first without changing its final.
func prependDefaultFilters(policyTree *config.Tree, peers []*reactor.PeerSettings, reg *policy.Registry) {
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
	slices.Sort(defaults)
	refs := make([]policy.Ref, len(defaults))
	for i, name := range defaults {
		refs[i] = policy.Ref{Name: name}
	}
	refs, bound := canonicalizeFilterRefs(refs, reg)

	for _, ps := range peers {
		for i, dflt := range refs {
			if filterChainContains(ps.ImportPolicy.Filters(), dflt.Name) {
				continue
			}
			// bound is nil when no reference bound in process, else parallel to refs.
			ps.ImportPolicy = ps.ImportPolicy.Prepend(dflt, i < len(bound) && bound[i])
		}
	}
}

// filterChainContains checks a canonical reference, or a plain instance name,
// regardless of its deactivation state.
func filterChainContains(chain []policy.Ref, name string) bool {
	for _, entry := range chain {
		if entry.Name == name {
			return true
		}
		if !strings.ContainsRune(name, ':') {
			if filterInstanceName(entry.Name) == name {
				return true
			}
		}
	}
	return false
}

// applyPortOverride overrides peer remote port from ze.test.bgp.port env var.
// This is a runtime-only mechanism for the test infrastructure (not YANG config).
//
// The key and its reader live in the reactor package, which the runtime create
// path also reads them from (Reactor.AddDynamicPeer). One declaration, so a
// peer built by either route dials the same port (ai/rules/principles.md).
func applyPortOverride(peers []*reactor.PeerSettings) {
	port, ok := reactor.PortOverrideFromEnv()
	if !ok {
		return
	}
	for _, ps := range peers {
		ps.Port = port
	}
}
