// Design: docs/architecture/api/architecture.md -- unified BGP feature-requirement checks over compiled bus filters
// Related: register.go -- registers validateBGPBusRequirements into config.RegisterBusRequirementValidator
// Related: peers.go -- peersAndDynamicGroups invokes the same compiled-bus checks
// Related: flowspec_binding.go -- hasFlowSpecFamily
// Related: filter_state_binding.go -- filterOwnerProcess

package bgpconfig

import (
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	bgpredist "github.com/ze-software/ze/internal/component/bgp/redistribute"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	redistributeConfigRoot = "redistribute"
	claimPeerUpReplay      = "bgp-peer-up-replay"
	msgBGPUpdate           = bgpevents.Namespace + ":" + bgpevents.EventUpdate
	msgBGPState            = bgpevents.Namespace + ":" + bgpevents.EventState
	msgBGPRefresh          = bgpevents.Namespace + ":" + bgpevents.EventRefresh
	msgBGPRaw              = bgpevents.Namespace + ":" + bgpevents.SendRaw
)

// bgpCompiledBusContext bundles the compiled DeliveryGraph, CompiledBus,
// endpoints, and peer path mappings derived from one configuration tree.
type bgpCompiledBusContext struct {
	graph       *pluginserver.DeliveryGraph
	compiled    *pluginserver.CompiledBus
	endpoints   []plugin.PluginConfig
	delivery    []pluginserver.DeliveryPeer
	peerToAddr  map[*reactor.PeerSettings]string
	peerToPath  map[*reactor.PeerSettings]string
	resolvedMap map[string]map[string]any // peer name -> resolved peer map
}

// validateBGPBusRequirements is the unified BusRequirementValidator registered
// with config.RegisterBusRequirementValidator (commit refusal 7). It evaluates
// all five BGP feature requirements (Redistribution, FlowSpec, Explicit AIGP,
// Filter-owner bgp:state, and Dissemination roles) plus capability-send checks
// against one compiled bus snapshot without mutating tree.
func validateBGPBusRequirements(tree *config.Tree, ctx *config.BusValidationContext) []error {
	if tree == nil {
		return nil
	}
	if tree.GetContainer("bgp") == nil {
		return nil
	}
	cloned := tree.Clone()
	peers, groups, resolvedBGPTree, err := buildPeerPopulationsWithResolved(cloned)
	// refuse-by-default: guards validateBGPBusRequirementsForPopulations -- structural peer parse errors are reported by the dedicated peer validator.
	if err != nil {
		return nil
	}
	if err := validateBGPBusRequirementsForPopulations(tree, peers, groups, resolvedBGPTree, ctx); err != nil {
		return []error{err}
	}
	return nil
}

// validateBGPBusRequirementsForPopulations builds the compiled bus once for
// (tree, peers, groups) and evaluates all BGP feature requirements, capability
// send requirements, and dissemination ownership.
func validateBGPBusRequirementsForPopulations(
	tree *config.Tree,
	peers []*reactor.PeerSettings,
	groups []*reactor.DynamicGroupConfig,
	resolvedBGPTree map[string]any,
	ctx *config.BusValidationContext,
) error {
	settings := populationSettings(peers, groups)
	if len(settings) == 0 {
		return nil
	}
	busCtx, err := compileBGPBusContext(tree, peers, groups, resolvedBGPTree, ctx)
	if err != nil {
		return err
	}

	var refusals []error
	if err := checkRedistributionRequirements(tree, settings, busCtx); err != nil {
		refusals = append(refusals, err)
	}
	if err := checkFlowSpecRequirements(settings, busCtx); err != nil {
		refusals = append(refusals, err)
	}
	if err := checkExplicitAIGPRequirements(settings, busCtx); err != nil {
		refusals = append(refusals, err)
	}
	if err := checkFilterOwnerStateRequirements(settings, busCtx); err != nil {
		refusals = append(refusals, err)
	}
	if err := validateDisseminationOnGraph(settings, busCtx); err != nil {
		refusals = append(refusals, err)
	}
	if err := validatePeerProcessCapsOnGraph(settings, busCtx); err != nil {
		refusals = append(refusals, err)
	}
	return errors.Join(refusals...)
}

// compileBGPBusContext builds DeliveryPeers, endpoint BusParticipantInputs,
// and the single CompiledBus / DeliveryGraph from tree.
func compileBGPBusContext(
	tree *config.Tree,
	peers []*reactor.PeerSettings,
	groups []*reactor.DynamicGroupConfig,
	resolvedBGPTree map[string]any,
	ctx *config.BusValidationContext,
) (*bgpCompiledBusContext, error) {
	var endpoints []plugin.PluginConfig
	if ctx != nil && len(ctx.Endpoints) > 0 {
		endpoints = slices.Clone(ctx.Endpoints)
	} else {
		extracted, err := config.ExtractPluginsFromTree(tree)
		if err != nil {
			return nil, err
		}
		endpoints = extracted
	}

	busCfg, err := plugin.ExtractBusConfigFromMap(tree.ToMap())
	if err != nil {
		return nil, err
	}

	delivery, peerToAddr, peerToPath, groupInputs, err := buildBGPDeliveryPeersFromTree(tree, peers, groups)
	if err != nil {
		return nil, err
	}

	endpointInputs, err := buildBGPEndpointInputs(tree, endpoints, busCfg, ctx)
	if err != nil {
		return nil, err
	}
	extraInputs := make([]pluginserver.BusParticipantInput, 0, len(groupInputs)+len(endpointInputs))
	extraInputs = append(extraInputs, groupInputs...)
	extraInputs = append(extraInputs, endpointInputs...)

	graph, err := pluginserver.BuildDeliveryGraph(bgpevents.Namespace, delivery, busCfg, extraInputs)
	if err != nil {
		return nil, err
	}

	return &bgpCompiledBusContext{
		graph:       graph,
		compiled:    graph.Compiled(),
		endpoints:   endpoints,
		delivery:    delivery,
		peerToAddr:  peerToAddr,
		peerToPath:  peerToPath,
		resolvedMap: indexResolvedPeerMaps(resolvedBGPTree),
	}, nil
}

// buildBGPDeliveryPeersFromTree projects static peers and dynamic groups into
// DeliveryPeer entries carrying their canonical YANG paths, parent group paths,
// secondary /bgp/ip/<addr> paths, local BusBlocks, and session facts, plus
// pure-container group BusParticipantInputs for group-to-peer inheritance.
func buildBGPDeliveryPeersFromTree(
	tree *config.Tree,
	peers []*reactor.PeerSettings,
	groups []*reactor.DynamicGroupConfig,
) (
	delivery []pluginserver.DeliveryPeer,
	peerToAddr map[*reactor.PeerSettings]string,
	peerToPath map[*reactor.PeerSettings]string,
	groupInputs []pluginserver.BusParticipantInput,
	err error,
) {
	peerToAddr = make(map[*reactor.PeerSettings]string, len(peers)+len(groups))
	peerToPath = make(map[*reactor.PeerSettings]string, len(peers)+len(groups))
	bgpTree := tree.GetContainer("bgp")

	groupBusBlocks := make(map[string]plugin.BusBlock)
	if bgpTree != nil {
		for _, gEntry := range bgpTree.GetListOrdered("group") {
			gPath := "/bgp/group/" + gEntry.Key
			if busSub := gEntry.Value.GetContainer("bus"); busSub != nil {
				blk, parseErr := plugin.ParseBusBlockFromMap(busSub.ToMap(), gPath+" > bus")
				if parseErr != nil {
					return nil, nil, nil, nil, parseErr
				}
				groupBusBlocks[gEntry.Key] = blk
			}
		}
	}

	dynamicGroupSet := make(map[string]bool, len(groups))
	for _, dg := range groups {
		dynamicGroupSet[dg.GroupName] = true
	}

	// Add non-dynamic group container inputs so group > bus blocks inherit
	// item-by-item into grouped peers inside CompileBus.
	if bgpTree != nil {
		for _, gEntry := range bgpTree.GetListOrdered("group") {
			if dynamicGroupSet[gEntry.Key] {
				continue
			}
			gPath := "/bgp/group/" + gEntry.Key
			groupInputs = append(groupInputs, pluginserver.BusParticipantInput{
				Path:          gPath,
				OwnBlock:      groupBusBlocks[gEntry.Key].Clone(),
				OwnOrigin:     pluginserver.BusOriginWritten,
				OwnSource:     gPath + " > bus",
				PureContainer: true,
			})
		}
	}

	var tb textbuf.Buffer
	for i, ps := range peers {
		pPath := peerCanonicalPath(ps)
		parentPath := ""
		if ps.GroupName != "" {
			parentPath = "/bgp/group/" + ps.GroupName
		}
		addr := ps.Address.String()
		if !ps.Address.IsValid() {
			addr = tb.Reset().Str("peer-").Int(int64(i)).String()
		}
		var secondary []string
		if ps.Address.IsValid() {
			secondary = []string{bgpSecondaryIPPrefix + ps.Address.String()}
		}

		ownBlock, parseErr := peerOwnBusBlock(bgpTree, ps, pPath)
		if parseErr != nil {
			return nil, nil, nil, nil, parseErr
		}
		dp := pluginserver.DeliveryPeer{
			Addr:           addr,
			Name:           ps.Name,
			Path:           pPath,
			ParentPath:     parentPath,
			SecondaryPaths: secondary,
			Bus:            ownBlock,
			BusOrigin:      pluginserver.BusOriginWritten,
			BusSource:      pPath + " > bus",
			SessionFacts:   peerSessionFacts(ps),
		}
		delivery = append(delivery, dp)
		peerToAddr[ps] = addr
		peerToPath[ps] = pPath
	}

	for _, dg := range groups {
		gPath := "/bgp/group/" + dg.GroupName
		addr := "dynamic-group:" + dg.GroupName
		dp := pluginserver.DeliveryPeer{
			Addr:         addr,
			Name:         dg.GroupName,
			Path:         gPath,
			Bus:          groupBusBlocks[dg.GroupName].Clone(),
			BusOrigin:    pluginserver.BusOriginWritten,
			BusSource:    gPath + " > bus",
			SessionFacts: peerSessionFacts(dg.Settings),
		}
		delivery = append(delivery, dp)
		peerToAddr[dg.Settings] = addr
		peerToPath[dg.Settings] = gPath
	}

	return delivery, peerToAddr, peerToPath, groupInputs, nil
}

func peerCanonicalPath(ps *reactor.PeerSettings) string {
	var tb textbuf.Buffer
	if ps.GroupName != "" {
		return tb.Str("/bgp/group/").Str(ps.GroupName).Str("/peer/").Str(ps.Name).String()
	}
	return tb.Str("/bgp/peer/").Str(ps.Name).String()
}

func peerOwnBusBlock(bgpTree *config.Tree, ps *reactor.PeerSettings, pPath string) (plugin.BusBlock, error) {
	if bgpTree == nil {
		return plugin.BusBlock{}, nil
	}
	var peerTree *config.Tree
	if ps.GroupName != "" {
		if gTree := bgpTree.GetList("group")[ps.GroupName]; gTree != nil {
			peerTree = gTree.GetList("peer")[ps.Name]
		}
	} else {
		peerTree = bgpTree.GetList("peer")[ps.Name]
	}
	// refuse-by-default: guards buildBGPDeliveryPeersFromTree -- peer absent from raw bgp tree has no written own bus block.
	if peerTree == nil {
		return plugin.BusBlock{}, nil
	}
	busSub := peerTree.GetContainer("bus")
	if busSub == nil {
		return plugin.BusBlock{}, nil
	}
	return plugin.ParseBusBlockFromMap(busSub.ToMap(), pPath+" > bus")
}

func peerSessionFacts(ps *reactor.PeerSettings) map[string]string {
	if ps == nil {
		return nil
	}
	rsClient := "false"
	if ps.RSClient {
		rsClient = "true"
	}
	sessType := "ebgp"
	if ps.IsIBGP() {
		sessType = "ibgp"
	}
	rrClient := "false"
	if ps.RouteReflectorClient {
		rrClient = "true"
	}
	return map[string]string{
		"rs-client":              rsClient,
		"session-type":           sessType,
		"route-reflector-client": rrClient,
	}
}

func buildBGPEndpointInputs(
	tree *config.Tree,
	endpoints []plugin.PluginConfig,
	busCfg plugin.BusConfig,
	ctx *config.BusValidationContext,
) ([]pluginserver.BusParticipantInput, error) {
	seenEndpoint := make(map[string]bool)
	seenCompiled := make(map[string]bool)
	var out []pluginserver.BusParticipantInput

	extDecls := make(map[string]config.ExternalBusDecl)
	for _, d := range config.ExternalBusDeclarations() {
		extDecls[d.Endpoint] = d
	}

	addEndpoint := func(epName string, reg *registry.Registration, isExternal bool) error {
		if epName == "" {
			return nil
		}
		if seenEndpoint[epName] {
			return nil
		}
		if reg != nil && reg.Name == attachmentOwnerBGP {
			return nil
		}
		seenEndpoint[epName] = true
		if reg != nil {
			seenCompiled[reg.Name] = true
		}

		epPath := plugin.EndpointPath(epName)
		canonicalPath := epPath
		var secondary []string
		var ownBlock plugin.BusBlock
		var ownSource string
		var sends []rpc.MessageSendDecl
		var receives []rpc.MessageReceiveDecl

		if reg != nil {
			sends = reg.Sends
			receives = reg.Receives
			if singletonPath, singletonTree := findSingletonNodeInTree(tree, reg.ParticipantNodes); singletonPath != "" {
				canonicalPath = singletonPath
				if singletonPath != epPath {
					secondary = []string{epPath}
				}
				if singletonTree != nil {
					if busSub := singletonTree.GetContainer("bus"); busSub != nil {
						ownSource = singletonPath + " > bus"
						parsed, err := plugin.ParseBusBlockFromMap(busSub.ToMap(), ownSource)
						if err != nil {
							return err
						}
						ownBlock = parsed
					}
				}
			}
		} else if isExternal {
			hasDecl := false
			if decl, ok := extDecls[epName]; ok {
				sends = decl.Sends
				receives = decl.Receives
				hasDecl = true
			} else if ctx != nil {
				if vp, ok := ctx.Participants[epPath]; ok && vp.HasDeclaration {
					sends = vp.Sends
					receives = vp.Receives
					hasDecl = true
				}
			}
			if !hasDecl {
				// An external endpoint without a static offline declaration
				// registers its messages at runtime Stage 1. Synthesize
				// provisional declarations from its written bus block so
				// offline compilation evaluates its written rules without
				// false Refusal 4 rejections.
				sends, receives = provisionalExternalDecls(busCfg.Endpoints[epName], busCfg.Participants[epPath].Block)
			}
		}

		out = append(out, pluginserver.BusParticipantInput{
			Path:           canonicalPath,
			EndpointName:   epName,
			SecondaryPaths: secondary,
			Sends:          sends,
			Receives:       receives,
			OwnBlock:       ownBlock,
			OwnOrigin:      pluginserver.BusOriginWritten,
			OwnSource:      ownSource,
		})
		return nil
	}

	// 1. Explicit endpoints declared under bus > endpoint.
	for _, ep := range endpoints {
		var reg *registry.Registration
		if ep.IsCompiled() {
			reg = registry.Lookup(ep.CompiledIdentity())
		}
		if err := addEndpoint(ep.Name, reg, !ep.IsCompiled()); err != nil {
			return nil, err
		}
	}

	// 2. Auto-loaded compiled-in endpoints active for tree's config paths.
	activePaths := config.CollectContainerPaths(tree)
	for _, name := range registry.PluginsForConfigPaths(activePaths) {
		if seenCompiled[name] {
			continue
		}
		reg := registry.Lookup(name)
		if reg == nil {
			continue
		}
		if err := addEndpoint(reg.Name, reg, false); err != nil {
			return nil, err
		}
	}

	// 3. Any additional endpoints present in busCfg.Endpoints (e.g. unit tests
	// specifying bus > endpoint overrides for auto-loaded compiled plugins).
	for _, epName := range slices.Sorted(maps.Keys(busCfg.Endpoints)) {
		if seenEndpoint[epName] {
			continue
		}
		reg := registry.Lookup(epName)
		if err := addEndpoint(epName, reg, reg == nil); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// provisionalExternalDecls derives offline send/receive declarations for an
// external endpoint that has not yet connected in Stage 1 from any messages
// explicitly named in its written bus blocks.
func provisionalExternalDecls(blocks ...plugin.BusBlock) ([]rpc.MessageSendDecl, []rpc.MessageReceiveDecl) {
	seenSend := make(map[string]bool)
	seenRecv := make(map[string]bool)
	var sends []rpc.MessageSendDecl
	var receives []rpc.MessageReceiveDecl

	addSend := func(msg string) {
		if msg == "" {
			return
		}
		if seenSend[msg] {
			return
		}
		seenSend[msg] = true
		sends = append(sends, rpc.MessageSendDecl{Message: msg})
	}
	addRecv := func(msg string) {
		if msg == "" {
			return
		}
		if seenRecv[msg] {
			return
		}
		seenRecv[msg] = true
		decl := rpc.MessageReceiveDecl{Message: msg}
		if msg == msgBGPUpdate {
			decl.Tags = []rpc.MessageTagDecl{{
				Name:   events.TagDirection,
				Values: []string{events.DirectionReceived, events.DirectionSent},
			}}
		}
		receives = append(receives, decl)
	}

	for i := range blocks {
		for _, msg := range blocks[i].Send.Allow {
			addSend(msg)
		}
		for _, msg := range slices.Sorted(maps.Keys(blocks[i].Send.Messages)) {
			addSend(msg)
		}
		for _, msg := range blocks[i].Receive.Allow {
			addRecv(msg)
		}
		for _, msg := range slices.Sorted(maps.Keys(blocks[i].Receive.Messages)) {
			addRecv(msg)
		}
	}
	return sends, receives
}

func findSingletonNodeInTree(tree *config.Tree, nodes []rpc.ParticipantNodeDecl) (string, *config.Tree) {
	if tree == nil {
		return "", nil
	}
	if len(nodes) != 1 {
		return "", nil
	}
	if nodes[0].SchemaPath == "" {
		return "", nil
	}
	trimmed := strings.TrimPrefix(nodes[0].SchemaPath, "/")
	if trimmed == "" {
		return "", nil
	}
	parts := strings.Split(trimmed, "/")
	segs := make([]string, 0, len(parts))
	cur := tree
	for _, part := range parts {
		if part == "" {
			continue
		}
		local := part
		if _, after, ok := strings.Cut(part, ":"); ok {
			local = after
		}
		segs = append(segs, local)
		if cur != nil {
			cur = cur.GetContainer(local)
		}
	}
	if len(segs) == 0 {
		return "", nil
	}
	return "/" + textbuf.Join(segs, "/"), cur
}

func indexResolvedPeerMaps(resolvedBGPTree map[string]any) map[string]map[string]any {
	out := make(map[string]map[string]any)
	if len(resolvedBGPTree) == 0 {
		return out
	}
	if peersMap, ok := resolvedBGPTree["peer"].(map[string]any); ok {
		for name, raw := range peersMap {
			if m, ok := raw.(map[string]any); ok {
				out[name] = m
			}
		}
	}
	if groupsMap, ok := resolvedBGPTree["group"].(map[string]any); ok {
		for gName, rawG := range groupsMap {
			gm, ok := rawG.(map[string]any)
			if !ok {
				continue
			}
			out[gName] = gm
			if gPeers, ok := gm["peer"].(map[string]any); ok {
				for pName, rawP := range gPeers {
					if pm, ok := rawP.(map[string]any); ok {
						out[pName] = pm
					}
				}
			}
		}
	}
	return out
}

// discoverSelectedRIBRegistration locates the ordinary selected-RIB owner
// dynamically from compiled registrations (Dissemination role
// "bgp-selected-output" without claiming it as an override), avoiding any
// hardcoded plugin name literal.
func discoverSelectedRIBRegistration() *registry.Registration {
	for _, reg := range registry.All() {
		if slices.Contains(reg.Claims, reactor.SelectedDisseminationRole) {
			continue
		}
		for _, decl := range reg.Dissemination {
			if decl.Namespace == bgpevents.Namespace && decl.Role == reactor.SelectedDisseminationRole {
				return reg
			}
		}
	}
	return nil
}

// discoverRedistributeRegistration locates the redistribution orchestrator
// dynamically by its declared ConfigRoots ("redistribute"), avoiding any
// hardcoded plugin name literal.
func discoverRedistributeRegistration() *registry.Registration {
	for _, reg := range registry.All() {
		if slices.Contains(reg.ConfigRoots, redistributeConfigRoot) {
			return reg
		}
	}
	return nil
}

// --- Requirement 1: Redistribution (S18, S33) ---

func checkRedistributionRequirements(tree *config.Tree, settings []*reactor.PeerSettings, busCtx *bgpCompiledBusContext) error {
	rules, err := config.ExtractRedistributeRules(tree)
	if err != nil {
		return err
	}
	hasSourceBGP, hasDestBGP := false, false
	for i := range rules {
		if bgpredist.SourceIsBGP(rules[i].Source) {
			hasSourceBGP = true
		}
		if bgpredist.DestinationIsBGP(rules[i].Destination) {
			hasDestBGP = true
		}
	}
	if !hasSourceBGP && !hasDestBGP {
		return nil
	}

	var refusals []error
	if hasSourceBGP {
		if ribReg := discoverSelectedRIBRegistration(); ribReg != nil {
			ribName := processNameFor(busCtx.endpoints, ribReg.Name)
			ribPath := endpointCanonicalPath(busCtx.compiled, ribName)
			for _, ps := range settings {
				addr := busCtx.peerToAddr[ps]
				pPath := busCtx.peerToPath[ps]
				if err := requirePeerFeedsEndpoint(
					busCtx, "redistribution (source bgp)", ps.Name, addr, pPath, ribName, ribPath,
					[]requiredEventFeed{
						{et: bgpevents.EventUpdate, dir: events.DirReceived, msg: msgBGPUpdate},
						{et: bgpevents.EventUpdate, dir: events.DirSent, msg: msgBGPUpdate},
						{et: bgpevents.EventState, dir: events.DirUnspecified, msg: msgBGPState},
						{et: bgpevents.EventRefresh, dir: events.DirUnspecified, msg: msgBGPRefresh},
					},
				); err != nil {
					refusals = append(refusals, err)
				}
			}
		}
	}

	if hasDestBGP {
		if orchReg := discoverRedistributeRegistration(); orchReg != nil {
			orchName := processNameFor(busCtx.endpoints, orchReg.Name)
			orchPath := endpointCanonicalPath(busCtx.compiled, orchName)
			for _, ps := range settings {
				addr := busCtx.peerToAddr[ps]
				pPath := busCtx.peerToPath[ps]
				if err := requirePeerFeedsEndpoint(
					busCtx, "redistribution (destination bgp)", ps.Name, addr, pPath, orchName, orchPath,
					[]requiredEventFeed{
						{et: bgpevents.EventState, dir: events.DirUnspecified, msg: msgBGPState},
					},
				); err != nil {
					refusals = append(refusals, err)
				}
				if !busCtx.graph.MaySend(addr, orchName, bgpevents.SendUpdate) {
					refusals = append(refusals, missingSendRequirementError(
						busCtx.compiled, "redistribution (destination bgp)", orchName, orchPath, pPath, msgBGPUpdate,
					))
				}
			}
		}
	}

	return errors.Join(refusals...)
}

// --- Requirement 2: FlowSpec (S34) ---

func checkFlowSpecRequirements(settings []*reactor.PeerSettings, busCtx *bgpCompiledBusContext) error {
	var flowSpecPeers []*reactor.PeerSettings
	for _, ps := range settings {
		hasFS, err := hasFlowSpecFamily(ps)
		if err != nil {
			return err
		}
		if hasFS {
			flowSpecPeers = append(flowSpecPeers, ps)
		}
	}
	if len(flowSpecPeers) == 0 {
		return nil
	}

	ribReg := discoverSelectedRIBRegistration()
	if ribReg == nil {
		return nil
	}

	var refusals []error

	// In-daemon check for the selected-RIB owner and any replay claimant serving
	// a FlowSpec peer.
	for _, declared := range busCtx.endpoints {
		if declared.RunsInDaemon() {
			continue
		}
		id := declared.CompiledIdentity()
		if id == "" {
			if names := plugin.RegistryNames(declared); len(names) > 0 {
				id = names[0]
			}
		}
		if id == ribReg.Name {
			var problem, why, fix textbuf.Buffer
			problem.Str("FlowSpec authorization requires endpoint ").Quoted(declared.Name).Str(" to run in-daemon")
			why.Str("FlowSpec authorization reads daemon-local selecting-RIB state and cannot run as a separate process")
			fix.Str("bus { endpoint ").Str(declared.Name).Str(" { internal ").Str(ribReg.Name).Str("; process in-daemon; } }")
			refusals = append(refusals, opererr.New(problem.String(), why.String(), fix.String()))
			continue
		}
		reg := registry.Lookup(id)
		if reg == nil {
			continue
		}
		if !reg.RunsInProcessOnly && !slices.Contains(reg.Claims, claimPeerUpReplay) && !reg.SignalsSessionReady {
			continue
		}
		for _, ps := range flowSpecPeers {
			addr := busCtx.peerToAddr[ps]
			if busCtx.graph.ProcessReceivesState(addr, declared.Name) && busCtx.graph.MaySend(addr, declared.Name, bgpevents.SendUpdate) {
				var problem, why, fix textbuf.Buffer
				problem.Str("peer ").Str(ps.Name).Str(": FlowSpec replay requires endpoint ").Quoted(declared.Name).Str(" to run in-daemon")
				why.Str("a separate-process replay plugin cannot query the daemon-local selecting RIB for FlowSpec validation")
				fix.Str("bus { endpoint ").Str(declared.Name).Str(" { internal ").Str(reg.Name).Str("; process in-daemon; } }")
				refusals = append(refusals, opererr.New(problem.String(), why.String(), fix.String()))
				break
			}
		}
	}

	// Selected-RIB must receive bgp:update (direction received) and bgp:state
	// from every BGP peer when FlowSpec is configured.
	ribName := processNameFor(busCtx.endpoints, ribReg.Name)
	ribPath := endpointCanonicalPath(busCtx.compiled, ribName)
	for _, ps := range settings {
		addr := busCtx.peerToAddr[ps]
		pPath := busCtx.peerToPath[ps]
		if err := requirePeerFeedsEndpoint(
			busCtx, "FlowSpec authorization", ps.Name, addr, pPath, ribName, ribPath,
			[]requiredEventFeed{
				{et: bgpevents.EventUpdate, dir: events.DirReceived, msg: msgBGPUpdate},
				{et: bgpevents.EventState, dir: events.DirUnspecified, msg: msgBGPState},
			},
		); err != nil {
			refusals = append(refusals, err)
		}
	}

	return errors.Join(refusals...)
}

// --- Requirement 3: Explicit AIGP (S-8, AC-15, S26, S35) ---

func checkExplicitAIGPRequirements(settings []*reactor.PeerSettings, busCtx *bgpCompiledBusContext) error {
	anyExplicit := false
	for _, ps := range settings {
		if peerExplicitAIGP(ps, busCtx.resolvedMap[ps.Name]) {
			anyExplicit = true
			break
		}
	}
	if !anyExplicit {
		return nil
	}
	ribReg := discoverSelectedRIBRegistration()
	if ribReg == nil {
		return nil
	}
	ribName := processNameFor(busCtx.endpoints, ribReg.Name)
	ribPath := endpointCanonicalPath(busCtx.compiled, ribName)

	var refusals []error
	for _, ps := range settings {
		addr := busCtx.peerToAddr[ps]
		pPath := busCtx.peerToPath[ps]
		if err := requirePeerFeedsEndpoint(
			busCtx, "AIGP selection", ps.Name, addr, pPath, ribName, ribPath,
			[]requiredEventFeed{
				{et: bgpevents.EventUpdate, dir: events.DirReceived, msg: msgBGPUpdate},
				{et: bgpevents.EventState, dir: events.DirUnspecified, msg: msgBGPState},
			},
		); err != nil {
			refusals = append(refusals, err)
		}
	}
	return errors.Join(refusals...)
}

// peerExplicitAIGP reports whether the operator explicitly configured
// `session { aigp { ... } }` on ps (without `enabled false`). An iBGP peer
// using the RFC 7311 default (AIGPEnabled() == true with nothing written)
// returns false (S-8, AC-15, S26).
func peerExplicitAIGP(ps *reactor.PeerSettings, _ map[string]any) bool {
	if ps == nil {
		return false
	}
	return ps.AIGPConfigured
}

// --- Requirement 4: Plugin-owned filter members needing bgp:state (S35) ---

func checkFilterOwnerStateRequirements(settings []*reactor.PeerSettings, busCtx *bgpCompiledBusContext) error {
	types := registry.FilterTypesMap()
	var refusals []error

	for _, ps := range settings {
		addr := busCtx.peerToAddr[ps]
		pPath := busCtx.peerToPath[ps]
		checkedOwner := make(map[string]bool)
		for _, ref := range slices.Concat(ps.ImportPolicy.Filters(), ps.ExportPolicy.Filters()) {
			ownerProc, owned := filterOwnerProcess(ref, busCtx.endpoints, types)
			if !owned {
				continue
			}
			if checkedOwner[ownerProc] {
				continue
			}
			checkedOwner[ownerProc] = true
			if !endpointDeclaresStateReceive(busCtx.compiled, ownerProc) {
				continue
			}
			ownerPath := endpointCanonicalPath(busCtx.compiled, ownerProc)
			if err := requirePeerFeedsEndpoint(
				busCtx, "filter owner state requirement", ps.Name, addr, pPath, ownerProc, ownerPath,
				[]requiredEventFeed{
					{et: bgpevents.EventState, dir: events.DirUnspecified, msg: msgBGPState},
				},
			); err != nil {
				refusals = append(refusals, err)
			}
		}
	}
	return errors.Join(refusals...)
}

func endpointDeclaresStateReceive(compiled *pluginserver.CompiledBus, endpointName string) bool {
	if compiled == nil {
		return false
	}
	f := lookupEndpointFilter(compiled, endpointName)
	if f == nil {
		return false
	}
	_, declared := f.ReceiveRules[msgBGPState]
	return declared
}

// --- Requirement 5: Capability send check (route-refresh / graceful-restart) ---

func validatePeerProcessCapsOnGraph(settings []*reactor.PeerSettings, busCtx *bgpCompiledBusContext) error {
	for _, ps := range settings {
		needsProcess := false
		capName := ""
		for _, c := range ps.Capabilities {
			// Only route-refresh and graceful-restart require a route-pushing endpoint.
			//exhaustive:ignore
			switch c.Code() {
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
		if _, ok := ps.RawCapabilityConfig["graceful-restart"]; ok {
			needsProcess = true
			capName = "graceful-restart"
		}
		if !needsProcess {
			continue
		}

		addr := busCtx.peerToAddr[ps]
		if len(busCtx.graph.SendersForPeer(addr, bgpevents.SendUpdate)) > 0 {
			continue
		}
		if len(busCtx.graph.SendersForPeer(addr, bgpevents.SendRaw)) > 0 {
			continue
		}

		pPath := busCtx.peerToPath[ps]
		var problem, why, fix textbuf.Buffer
		problem.Str("peer ").Str(ps.Name).Str(" (").Str(pPath).Str("): ").Str(capName).
			Str(" requires an endpoint permitted to send ").Str(msgBGPUpdate).Str(" or ").Str(msgBGPRaw)
		why.Str("no active bus endpoint is permitted by the compiled bus filters to send ").
			Str(msgBGPUpdate).Str(" or ").Str(msgBGPRaw).Str(" to ").Str(pPath)
		fix.Str("ensure an endpoint can send ").Str(msgBGPUpdate).Str(" or ").Str(msgBGPRaw).
			Str(" to ").Str(pPath).Str(" and remove any restrictive receive > from or send > allow block")
		return opererr.New(problem.String(), why.String(), fix.String())
	}
	return nil
}

// --- Shared diagnostic builders ---

type requiredEventFeed struct {
	et  string
	dir events.Direction
	msg string
}

func requirePeerFeedsEndpoint(
	busCtx *bgpCompiledBusContext,
	feature, peerName, addr, peerPath, endpointName, endpointPath string,
	feeds []requiredEventFeed,
) error {
	nsID := events.LookupNamespaceID(bgpevents.Namespace)
	var missing []string
	for _, f := range feeds {
		etID := events.LookupEventTypeID(f.et)
		if !slices.Contains(busCtx.graph.Receivers(nsID, etID, f.dir, addr), endpointName) {
			if !slices.Contains(missing, f.msg) {
				missing = append(missing, f.msg)
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return missingReceiveRequirementError(busCtx.compiled, feature, peerName, peerPath, endpointName, endpointPath, missing)
}

func missingReceiveRequirementError(
	compiled *pluginserver.CompiledBus,
	feature, peerName, peerPath, endpointName, endpointPath string,
	missingMsgs []string,
) *opererr.Error {
	narrowedBy, fix := diagnoseMissingReceive(compiled, peerPath, endpointName, endpointPath, missingMsgs)
	var problem, why textbuf.Buffer
	problem.Str("feature ").Str(feature).Str(": peer ").Quoted(peerName).Str(" (").Str(peerPath).
		Str(") does not deliver [ ").Join(missingMsgs, " ").Str(" ] to ").Str(endpointPath)
	why.Str(feature).Str(" requires ").Str(endpointPath).Str(" to receive [ ").Join(missingMsgs, " ").
		Str(" ] from ").Str(peerPath)
	if narrowedBy != "" {
		why.Str(" (narrowed by ").Str(narrowedBy).Byte(')')
	}
	return opererr.New(problem.String(), why.String(), fix)
}

func diagnoseMissingReceive(
	compiled *pluginserver.CompiledBus,
	peerPath, endpointName, endpointPath string,
	missingMsgs []string,
) (narrowedBy, fix string) {
	if compiled != nil {
		if pf := compiled.Filter(peerPath); pf != nil {
			for _, msg := range missingMsgs {
				if r := pf.SendRules[msg]; r != nil {
					if !r.Allowed {
						src := formatRuleSources(r.AllowOrigin.Sources, peerPath+" > bus > send > allow")
						var fb textbuf.Buffer
						fb.Str("add [ ").Join(missingMsgs, " ").Str(" ] to ").Str(peerPath).Str(" > bus > send > allow")
						return src, fb.String()
					}
					if r.ToNone {
						src := formatRuleSources(r.ToOrigin.Sources, peerPath+" > bus > send > to")
						var fb textbuf.Buffer
						fb.Str("add ").Str(endpointPath).Str(" to ").Str(peerPath).Str(" > bus > send > to")
						return src, fb.String()
					}
					if !r.ToAll && !slices.Contains(r.ResolvedPaths, endpointPath) {
						src := formatRuleSources(r.ToOrigin.Sources, peerPath+" > bus > send > to")
						var fb textbuf.Buffer
						fb.Str("add ").Str(endpointPath).Str(" to ").Str(peerPath).Str(" > bus > send > to")
						return src, fb.String()
					}
				}
			}
		}
		if ef := lookupEndpointFilter(compiled, endpointName); ef != nil {
			for _, msg := range missingMsgs {
				if r := ef.ReceiveRules[msg]; r != nil {
					if !r.Allowed {
						src := formatRuleSources(r.AllowOrigin.Sources, "bus > endpoint "+endpointName+" > receive > allow")
						var fb textbuf.Buffer
						fb.Str("set bus endpoint ").Str(endpointName).Str(" receive allow ").Join(missingMsgs, " ")
						return src, fb.String()
					}
					if r.FromNone {
						src := formatRuleSources(r.FromOrigin.Sources, "bus > endpoint "+endpointName+" > receive > from")
						var fb textbuf.Buffer
						fb.Str("set bus endpoint ").Str(endpointName).Str(" receive from ").Str(peerPath)
						return src, fb.String()
					}
					if !r.FromAll && !slices.Contains(r.ResolvedPaths, peerPath) {
						src := formatRuleSources(r.FromOrigin.Sources, "bus > endpoint "+endpointName+" > receive > from")
						var fb textbuf.Buffer
						fb.Str("set bus endpoint ").Str(endpointName).Str(" receive from ").Str(peerPath)
						return src, fb.String()
					}
				}
			}
		}
	}
	var fb textbuf.Buffer
	fb.Str("bus { endpoint ").Str(endpointName).Str(" { receive { allow [ ").Join(missingMsgs, " ").Str(" ]; } } }")
	return "", fb.String()
}

func missingSendRequirementError(
	compiled *pluginserver.CompiledBus,
	feature, endpointName, endpointPath, peerPath, msg string,
) *opererr.Error {
	narrowedBy, fix := diagnoseMissingSend(compiled, endpointName, endpointPath, peerPath, msg)
	var problem, why textbuf.Buffer
	problem.Str("feature ").Str(feature).Str(": endpoint ").Quoted(endpointName).Str(" (").Str(endpointPath).
		Str(") is not permitted to send ").Str(msg).Str(" to ").Str(peerPath)
	why.Str(feature).Str(" requires ").Str(endpointPath).Str(" to send ").Str(msg).Str(" to ").Str(peerPath)
	if narrowedBy != "" {
		why.Str(" (narrowed by ").Str(narrowedBy).Byte(')')
	}
	return opererr.New(problem.String(), why.String(), fix)
}

func diagnoseMissingSend(
	compiled *pluginserver.CompiledBus,
	endpointName, endpointPath, peerPath, msg string,
) (narrowedBy, fix string) {
	if compiled != nil {
		if ef := lookupEndpointFilter(compiled, endpointName); ef != nil {
			if r := ef.SendRules[msg]; r != nil {
				if !r.Allowed {
					src := formatRuleSources(r.AllowOrigin.Sources, "bus > endpoint "+endpointName+" > send > allow")
					var fb textbuf.Buffer
					fb.Str("set bus endpoint ").Str(endpointName).Str(" send allow ").Str(msg)
					return src, fb.String()
				}
				if r.ToNone {
					src := formatRuleSources(r.ToOrigin.Sources, "bus > endpoint "+endpointName+" > send > to")
					var fb textbuf.Buffer
					fb.Str("set bus endpoint ").Str(endpointName).Str(" send to ").Str(peerPath)
					return src, fb.String()
				}
				if !r.ToAll && !slices.Contains(r.ResolvedPaths, peerPath) {
					src := formatRuleSources(r.ToOrigin.Sources, "bus > endpoint "+endpointName+" > send > to")
					var fb textbuf.Buffer
					fb.Str("set bus endpoint ").Str(endpointName).Str(" send to ").Str(peerPath)
					return src, fb.String()
				}
			}
		}
		if pf := compiled.Filter(peerPath); pf != nil {
			if r := pf.ReceiveRules[msg]; r != nil {
				if !r.Allowed {
					src := formatRuleSources(r.AllowOrigin.Sources, peerPath+" > bus > receive > allow")
					var fb textbuf.Buffer
					fb.Str("add ").Str(msg).Str(" to ").Str(peerPath).Str(" > bus > receive > allow")
					return src, fb.String()
				}
				if r.FromNone {
					src := formatRuleSources(r.FromOrigin.Sources, peerPath+" > bus > receive > from")
					var fb textbuf.Buffer
					fb.Str("add ").Str(endpointPath).Str(" to ").Str(peerPath).Str(" > bus > receive > from")
					return src, fb.String()
				}
				if !r.FromAll && !slices.Contains(r.ResolvedPaths, endpointPath) {
					src := formatRuleSources(r.FromOrigin.Sources, peerPath+" > bus > receive > from")
					var fb textbuf.Buffer
					fb.Str("add ").Str(endpointPath).Str(" to ").Str(peerPath).Str(" > bus > receive > from")
					return src, fb.String()
				}
			}
		}
	}
	var fb textbuf.Buffer
	fb.Str("set bus endpoint ").Str(endpointName).Str(" send allow ").Str(msg)
	return "", fb.String()
}

func lookupEndpointFilter(compiled *pluginserver.CompiledBus, endpointName string) *pluginserver.CompiledParticipantFilter {
	if compiled == nil {
		return nil
	}
	if endpointName == "" {
		return nil
	}
	if f := compiled.Filter(plugin.EndpointPath(endpointName)); f != nil {
		return f
	}
	for _, p := range compiled.Participants() {
		if f := compiled.Filter(p); f != nil && f.EndpointName == endpointName {
			return f
		}
	}
	return nil
}

func endpointCanonicalPath(compiled *pluginserver.CompiledBus, endpointName string) string {
	if f := lookupEndpointFilter(compiled, endpointName); f != nil {
		return f.Path
	}
	return plugin.EndpointPath(endpointName)
}

func formatRuleSources(sources []string, fallback string) string {
	if len(sources) == 0 {
		return fallback
	}
	return textbuf.Join(sources, ", ")
}
