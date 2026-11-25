// Design: docs/architecture/api/architecture.md -- unit tests for typed auto-load reasons, implied bus participants, multi-namespace origin-annotated delivery edges, and lifecycle receptions.
// Related: autoload_reasons.go -- EvaluateImpliedEndpoints, BuildImpliedParticipants, CompileOfflineBusFromTree.
// Related: delivery_graph.go -- DeliveryGraph.InspectEdges, DeliveryEdge.
// Related: startup_autoload.go -- AutoLoadReason, AutoLoadedEndpoint.

package server

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const impliedExtraNS = "implied-telemetry-test"

func init() {
	_ = events.RegisterNamespace(impliedExtraNS, "metric")
}

// TestImpliedReceptionPrintsAsWrittenBlock verifies that a registered
// reception is projected with origin "registered" and source "registration of
// <name>", renders as a round-trippable bus block, and becomes written-only
// with zero remaining implied count once pasted back into configuration.
//
// VALIDATES: AC-23, AC-38, S30 -- implied reception round-trips cleanly.
// PREVENTS: implied blocks failing to parse back or remaining duplicated as
// implied after being written explicitly.
func TestImpliedReceptionPrintsAsWrittenBlock(t *testing.T) {
	msgUpdate := graphMsg(graphUpdate)
	senderPath := "/" + graphNS + "/entry/s1"
	collectorPath := plugin.EndpointPath("collector")

	compiled, err := CompileBus(NewTagInterner(), []BusParticipantInput{
		{
			Path:  senderPath,
			Sends: []rpc.MessageSendDecl{{Message: msgUpdate}},
		},
		{
			Path:         collectorPath,
			EndpointName: "collector",
			Receives:     []rpc.MessageReceiveDecl{{Message: msgUpdate}},
		},
	})
	require.NoError(t, err)

	participants := BuildImpliedParticipants(compiled)
	var collector *config.ImpliedParticipant
	for i := range participants {
		if participants[i].Path == collectorPath {
			collector = &participants[i]
			break
		}
	}
	require.NotNil(t, collector, "collector must appear in BuildImpliedParticipants")
	require.Len(t, collector.Receive, 1)
	assert.Equal(t, msgUpdate, collector.Receive[0].Message)
	assert.True(t, collector.Receive[0].Allowed)
	assert.Equal(t, config.ImpliedOriginRegistered, collector.Receive[0].Origin)
	assert.Equal(t, "registration of collector", collector.Receive[0].Source)
	assert.Equal(t, []string{senderPath}, collector.Receive[0].Targets)

	rendered := config.RenderImpliedText("", config.ImpliedReport{
		Participants: []config.ImpliedParticipant{*collector},
	})
	assert.Contains(t, rendered, "# implied by: registration of collector")
	assert.Contains(t, rendered, "participant "+config.QuoteIfNeeded(collectorPath))
	assert.Contains(t, rendered, "message "+config.QuoteIfNeeded(msgUpdate))
	assert.Equal(t, config.FormatHierarchical, config.DetectFormat(rendered),
		"rendered implied block must stay hierarchical and never trigger FormatSetMeta")

	schema, err := config.YANGSchema()
	require.NoError(t, err)
	parsedTree, err := config.NewParser(schema).Parse(rendered)
	require.NoError(t, err, "rendered implied bus block must parse cleanly")
	busTree := parsedTree.GetContainer("bus")
	require.NotNil(t, busTree)

	busCfg, err := plugin.ParseBusConfigFromMap(busTree.ToMap())
	require.NoError(t, err)
	ov, ok := busCfg.Participants[collectorPath]
	require.True(t, ok, "parsed bus config must carry participant override for collector")

	recompiled, err := CompileBus(NewTagInterner(), []BusParticipantInput{
		{
			Path:  senderPath,
			Sends: []rpc.MessageSendDecl{{Message: msgUpdate}},
		},
		{
			Path:          collectorPath,
			EndpointName:  "collector",
			Receives:      []rpc.MessageReceiveDecl{{Message: msgUpdate}},
			BusSideBlock:  ov.Block,
			BusSideSource: ov.Source,
		},
	})
	require.NoError(t, err)

	reParticipants := BuildImpliedParticipants(recompiled)
	var writtenCollector *config.ImpliedParticipant
	for i := range reParticipants {
		if reParticipants[i].Path == collectorPath {
			writtenCollector = &reParticipants[i]
			break
		}
	}
	require.NotNil(t, writtenCollector)
	require.Len(t, writtenCollector.Receive, 1)
	assert.Equal(t, config.ImpliedOriginWritten, writtenCollector.Receive[0].Origin)
	assert.Zero(t, config.ImpliedReport{
		Participants: []config.ImpliedParticipant{*writtenCollector},
	}.Count(), "explicitly written copy must leave zero implied count")
}

// TestImpliedShowsReplacedDefaults verifies that overriding a participant's
// registered default send destinations with a written to list surfaces the
// replaced defaults in BuildImpliedParticipants and in RenderImpliedText.
//
// VALIDATES: AC-23 -- written to shows replaced registered defaults.
// PREVENTS: silently hiding which default destinations an explicit to list
// removed.
func TestImpliedShowsReplacedDefaults(t *testing.T) {
	msgUpdate := graphMsg(graphUpdate)
	c1 := "/" + graphNS + "/entry/c1"
	c2 := "/" + graphNS + "/entry/c2"
	svcPath := "/" + graphNS + "/service/dispatcher"

	compiled, err := CompileBus(NewTagInterner(), []BusParticipantInput{
		{
			Path:     c1,
			Receives: []rpc.MessageReceiveDecl{{Message: msgUpdate}},
		},
		{
			Path:     c2,
			Receives: []rpc.MessageReceiveDecl{{Message: msgUpdate}},
		},
		{
			Path:         svcPath,
			EndpointName: "dispatcher",
			Sends: []rpc.MessageSendDecl{{
				Message:             msgUpdate,
				DefaultDestinations: []string{c1, c2},
			}},
			OwnBlock: plugin.BusBlock{
				Send: plugin.BusDirectionBlock{
					TargetsPresent: true,
					Targets:        []string{c1},
				},
			},
			OwnOrigin: BusOriginWritten,
			OwnSource: "bus > endpoint dispatcher",
		},
	})
	require.NoError(t, err)

	participants := BuildImpliedParticipants(compiled)
	var dispatcher *config.ImpliedParticipant
	for i := range participants {
		if participants[i].Path == svcPath {
			dispatcher = &participants[i]
			break
		}
	}
	require.NotNil(t, dispatcher)
	require.Len(t, dispatcher.Send, 1)
	assert.Equal(t, config.ImpliedOriginWritten, dispatcher.Send[0].Origin)
	assert.Equal(t, []string{c1}, dispatcher.Send[0].Targets)
	assert.Equal(t, []string{c1, c2}, dispatcher.Send[0].ReplacedDefaults)
	assert.Contains(t, dispatcher.Send[0].Source, "bus > endpoint dispatcher")

	rendered := config.RenderImpliedText("", config.ImpliedReport{
		Participants: []config.ImpliedParticipant{*dispatcher},
	})
	assert.Contains(t, rendered, "replaced registered default(s)")
	assert.Contains(t, rendered, c2)
}

// TestDeliveryEdgeOrigins verifies multi-namespace delivery inspection across
// all five origins (written, inherited, registered, required, dormant), S29
// single-entry deduplication for a multi-client registered sender, and S31
// dynamic container attribution beside a dynamic participant's address while
// static participants carry no group.
//
// VALIDATES: AC-25, S29, S31 -- InspectEdges reports all origins, collapses
// registered multi-destination sends to one line, and distinguishes dynamic vs
// static participants across multiple namespaces.
// PREVENTS: missing origins, per-client duplicate rows for registered wiring,
// missing dynamic container paths, or dropping non-primary namespaces.
func TestInspectEdgesDeliveryOriginsAndDynamicGroups(t *testing.T) {
	msgUpdate := graphMsg(graphUpdate)
	msgState := graphMsg(graphState)
	msgMetric := impliedExtraNS + ":metric"
	msgMissing := graphNS + ":absent-signal"

	c1 := "/" + graphNS + "/entry/c1"
	c2 := "/" + graphNS + "/entry/c2"
	c3 := "/" + graphNS + "/entry/c3"
	dynPath := "/" + graphNS + "/ip/198.51.100.5"
	dynGroup := "/" + graphNS + "/container/dyn1"
	staticPath := "/" + graphNS + "/entry/static1"
	inhParent := "/" + graphNS + "/container/transit"
	inhChild := "/" + graphNS + "/container/transit/entry/m1"

	peers := []DeliveryPeer{
		{Addr: "192.0.2.1", Name: "c1", Path: c1},
		{Addr: "192.0.2.2", Name: "c2", Path: c2},
		{Addr: "192.0.2.3", Name: "c3", Path: c3},
		{Addr: "198.51.100.5", Path: dynPath, ParentPath: dynGroup, Dynamic: true},
		{Addr: "192.0.2.9", Name: "static1", Path: staticPath},
		{Addr: "192.0.2.20", Name: "m1", Path: inhChild, ParentPath: inhParent},
	}

	extraEndpoints := []BusParticipantInput{
		// S29: registered multi-destination sender covering c1, c2, c3.
		{
			Path:         "/" + graphNS + "/hub-distributor",
			EndpointName: "hub-distributor",
			Sends: []rpc.MessageSendDecl{{
				Message:             msgUpdate,
				DefaultDestinations: []string{c1, c2, c3},
			}},
		},
		// Written reception from dynPath and staticPath.
		{
			Path:         plugin.EndpointPath("written-sink"),
			EndpointName: "written-sink",
			Receives:     []rpc.MessageReceiveDecl{{Message: msgState}},
			OwnBlock: plugin.BusBlock{
				Receive: plugin.BusDirectionBlock{
					TargetsPresent: true,
					Targets:        []string{dynPath, staticPath},
				},
			},
			OwnOrigin: BusOriginWritten,
			OwnSource: "bus > endpoint written-sink",
		},
		// Inherited target via transit container override.
		{
			Path:         plugin.EndpointPath("inherited-sink"),
			EndpointName: "inherited-sink",
			Receives:     []rpc.MessageReceiveDecl{{Message: msgState}},
		},
		// Required feature endpoint.
		{
			Path:         plugin.EndpointPath("required-sink"),
			EndpointName: "required-sink",
			Receives:     []rpc.MessageReceiveDecl{{Message: msgState}},
			Required:     true,
		},
		// Dormant reception with no active sender.
		{
			Path:         plugin.EndpointPath("dormant-sink"),
			EndpointName: "dormant-sink",
			Receives:     []rpc.MessageReceiveDecl{{Message: msgMissing}},
		},
		// Second namespace producer & consumer (multi-namespace proof).
		{
			Path:         plugin.EndpointPath("metric-src"),
			EndpointName: "metric-src",
			Sends:        []rpc.MessageSendDecl{{Message: msgMetric}},
		},
		{
			Path:         plugin.EndpointPath("metric-dst"),
			EndpointName: "metric-dst",
			Receives:     []rpc.MessageReceiveDecl{{Message: msgMetric}},
		},
	}

	busCfg := plugin.BusConfig{
		Participants: map[string]plugin.BusParticipantOverride{
			inhParent: {
				Path:   inhParent,
				Source: "bus > participant " + inhParent,
				Block: plugin.BusBlock{
					Send: plugin.BusDirectionBlock{
						TargetsPresent: true,
						Targets:        []string{plugin.EndpointPath("inherited-sink")},
					},
				},
			},
		},
	}

	g, err := BuildDeliveryGraph(graphNS, peers, busCfg, extraEndpoints)
	require.NoError(t, err)

	edges := g.InspectEdges()
	require.NotEmpty(t, edges)

	seenOrigins := make(map[string]bool)
	var distributorEdges []DeliveryEdge
	var dynEdge, staticEdge, metricEdge *DeliveryEdge

	for i := range edges {
		e := &edges[i]
		seenOrigins[e.Origin] = true

		if e.SenderPath == "/"+graphNS+"/hub-distributor" && e.Message == msgUpdate {
			distributorEdges = append(distributorEdges, *e)
		}
		if e.SenderPath == dynPath && e.ReceiverEndpoint == "written-sink" && e.Message == msgState {
			dynEdge = e
		}
		if e.SenderPath == staticPath && e.ReceiverEndpoint == "written-sink" && e.Message == msgState {
			staticEdge = e
		}
		if e.Message == msgMetric && e.SenderEndpoint == "metric-src" && e.ReceiverEndpoint == "metric-dst" {
			metricEdge = e
		}
	}

	// All 5 origins must be present.
	assert.True(t, seenOrigins[config.ImpliedOriginWritten], "expected written edge")
	assert.True(t, seenOrigins[config.ImpliedOriginInherited], "expected inherited edge")
	assert.True(t, seenOrigins[config.ImpliedOriginRegistered], "expected registered edge")
	assert.True(t, seenOrigins[config.ImpliedOriginRequired], "expected required edge")
	assert.True(t, seenOrigins[config.ImpliedOriginDormant], "expected dormant edge")

	// S29: single-entry deduplication for hub-distributor covering c1, c2, c3.
	require.Len(t, distributorEdges, 1, "registered sender covering 3 destinations must collapse to one row")
	assert.Equal(t, config.ImpliedOriginRegistered, distributorEdges[0].Origin)
	assert.Equal(t, []string{c1, c2, c3}, distributorEdges[0].Destinations)

	// S31: dynamic participant carries admitting container path; static carries none.
	require.NotNil(t, dynEdge)
	assert.Equal(t, dynGroup, dynEdge.SenderGroup)
	require.NotNil(t, staticEdge)
	assert.Empty(t, staticEdge.SenderGroup)

	// Multi-namespace edge present alongside primary namespace.
	require.NotNil(t, metricEdge)
	assert.Equal(t, config.ImpliedOriginRegistered, metricEdge.Origin)
}

// TestLifecycleDeliveriesAreRegisteredReceptions verifies that an endpoint
// declaring WantsValidateOpen receives the namespace state lifecycle reception
// through the compiled bus filter with origin "registered", and is blocked
// when a written receive allow-list omits it.
//
// VALIDATES: AC-39, S32 -- WantsValidateOpen lifecycle deliveries appear in
// InspectEdges with origin "registered" and obey receive allow-lists.
// PREVENTS: OPEN-validator removal observers bypassing CompiledBus filtering.
func TestLifecycleDeliveriesAreRegisteredReceptions(t *testing.T) {
	ns, _, stateET := graphIDs(t)
	msgState := graphMsg(graphState)
	msgUpdate := graphMsg(graphUpdate)

	s := &Server{}
	pm := process.NewProcessManager(nil)
	validatorProc := process.NewProcess(plugin.PluginConfig{Name: "open-validator"})
	validatorProc.SetRegistration(&plugin.PluginRegistration{
		WantsValidateOpen: true,
		Receives:          []rpc.MessageReceiveDecl{{Message: msgUpdate}},
		Done:              true,
	})
	require.NoError(t, pm.AddProcess("open-validator", validatorProc))
	s.procManager.Store(pm)

	peers := []DeliveryPeer{{
		Addr: "192.0.2.44",
		Name: "p44",
		Path: "/" + graphNS + "/entry/p44",
	}}

	// Default registration: WantsValidateOpen synthesizes registered state reception.
	require.NoError(t, s.SetDeliveryGraph(graphNS, peers))

	assert.Contains(t, s.DeliveryGraph().Receivers(ns, stateET, events.DirUnspecified, "192.0.2.44"), "open-validator")
	foundStateEdge := false
	for _, e := range s.DeliveryGraph().InspectEdges() {
		if e.ReceiverEndpoint == "open-validator" && e.Message == msgState {
			foundStateEdge = true
			assert.Equal(t, config.ImpliedOriginRegistered, e.Origin)
		}
	}
	assert.True(t, foundStateEdge, "WantsValidateOpen endpoint must show registered state reception in InspectEdges")

	// Written receive allow-list omitting state MUST block lifecycle delivery.
	s.SetBusConfig(plugin.BusConfig{
		Endpoints: map[string]plugin.BusBlock{
			"open-validator": {
				Receive: plugin.BusDirectionBlock{
					AllowPresent: true,
					Allow:        []string{msgUpdate},
				},
			},
		},
	})
	require.NoError(t, s.SetDeliveryGraph(graphNS, peers))

	assert.NotContains(t, s.DeliveryGraph().Receivers(ns, stateET, events.DirUnspecified, "192.0.2.44"), "open-validator")
	for _, e := range s.DeliveryGraph().InspectEdges() {
		assert.False(t, e.ReceiverEndpoint == "open-validator" && e.Message == msgState,
			"receive allow-list omitting state must remove the lifecycle reception edge")
	}
}

// TestAutoLoadReasonsTypedAndTokenLoadersDeleted verifies typed AutoLoadReason
// attribution across all five reason kinds (config-path, family, dependency,
// config-need, data-plane) on both live Server loaders and offline
// EvaluateImpliedEndpoints, and proves that an absent sender leaves a
// reception dormant rather than auto-loading by token (Q-11(c), M-4).
//
// VALIDATES: AC-23, Q-11(c), M-4 -- typed auto-load reasons and dormant
// reception guidance when no sender is loaded.
// PREVENTS: untyped auto-load reasons, resurrecting token auto-loaders, or
// duplicating an explicitly written endpoint as implied.
func TestAutoLoadReasonsTypedAndTokenLoadersDeleted(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	registerStub := func(reg registry.Registration) {
		t.Helper()
		reg.Description = "auto-load reason test plugin"
		reg.RunEngine = func(net.Conn) int { return 0 }
		reg.CLIHandler = func([]string) int { return 0 }
		require.NoError(t, registry.Register(reg))
	}

	const (
		rootPlugin      = "test-root-owner"
		hardDepPlugin   = "test-hard-dep"
		needProvider    = "test-need-provider"
		familyPlugin    = "test-family-codec"
		dpWriterPlugin  = "test-dp-writer"
		absentSender    = "test-absent-sender"
		absentSenderMsg = "delivery-graph-test:custom-decorated"
	)

	registerStub(registry.Registration{
		Name:                 rootPlugin,
		ConfigRoots:          []string{"test-service"},
		Dependencies:         []string{hardDepPlugin},
		OptionalDependencies: []string{needProvider},
		NeedsDataPlane:       true,
		ConfigNeeds: []registry.ConfigNeed{{
			Path:      []string{"test-service", "enrich"},
			Value:     "true",
			Providers: []string{needProvider},
		}},
	})
	registerStub(registry.Registration{Name: hardDepPlugin})
	registerStub(registry.Registration{Name: needProvider})
	registerStub(registry.Registration{
		Name:     familyPlugin,
		Families: []string{"test-afi/test-safi"},
	})
	registerStub(registry.Registration{
		Name:        dpWriterPlugin,
		DataPlane:   "test-dp",
		ProgramsFIB: true,
	})
	registerStub(registry.Registration{
		Name:  absentSender,
		Sends: []rpc.MessageSendDecl{{Message: absentSenderMsg}},
	})

	// 1. Live Server loaders record typed reasons for config-path, dependency,
	// config-need, data-plane, and family, and never auto-load absentSender
	// (token auto-loading was removed under Q-11(c)).
	treeMap := map[string]any{
		"test-service": map[string]any{
			"enrich": "true",
		},
	}
	s := &Server{
		config: &ServerConfig{
			ConfiguredPaths:    []string{"test-service"},
			ConfiguredFamilies: []string{"test-afi/test-safi"},
			DataPlane:          "test-dp",
			ConfigNeedHolds:    config.ConfigNeedHolds(treeMap),
		},
		loadedPlugins: make(map[string]bool),
	}

	pathPlugins, err := s.getConfigPathPlugins()
	require.NoError(t, err)
	require.Len(t, pathPlugins, 4)

	famPlugins, err := s.getUnclaimedFamilyPlugins()
	require.NoError(t, err)
	require.Len(t, famPlugins, 1)
	assert.Equal(t, familyPlugin, famPlugins[0].Name)

	byName := make(map[string]config.ImpliedEndpoint)
	for _, ep := range s.AutoLoadedEndpoints() {
		byName[ep.Name] = ep
	}

	assert.Equal(t, AutoLoadReasonConfigPath, byName[rootPlugin].ReasonKind)
	assert.Equal(t, "test-service", byName[rootPlugin].ReasonSource)
	assert.Equal(t, AutoLoadReasonDependency, byName[hardDepPlugin].ReasonKind)
	assert.Equal(t, rootPlugin, byName[hardDepPlugin].ReasonSource)
	assert.Equal(t, AutoLoadReasonConfigNeed, byName[needProvider].ReasonKind)
	assert.Equal(t, AutoLoadReasonDataPlane, byName[dpWriterPlugin].ReasonKind)
	assert.Equal(t, "test-dp", byName[dpWriterPlugin].ReasonSource)
	assert.Equal(t, AutoLoadReasonFamily, byName[familyPlugin].ReasonKind)
	assert.Equal(t, "test-afi/test-safi", byName[familyPlugin].ReasonSource)
	assert.NotContains(t, byName, absentSender, "token auto-loading must remain deleted (Q-11(c))")

	// Clearing on removal removes the tracked auto-load reason.
	s.unmarkPluginLoaded(familyPlugin)
	_, stillTracked := s.AutoLoadReasonFor(familyPlugin)
	assert.False(t, stillTracked)

	// 2. Offline EvaluateImpliedEndpoints mirrors config-path, dependency,
	// config-need, and family rules and excludes explicitly written endpoints.
	tree := config.NewTree()
	svc := tree.GetOrCreateContainer("test-service")
	svc.Set("enrich", "true")
	svc.GetOrCreateContainer("family").GetOrCreateContainer("test-afi").GetOrCreateContainer("test-safi")
	// Explicitly write hardDepPlugin under bus > endpoint so it is NOT implied.
	bus := tree.GetOrCreateContainer("bus")
	epTree := config.NewTree()
	epTree.Set("internal", hardDepPlugin)
	bus.AddListEntry("endpoint", hardDepPlugin, epTree)

	offlineImplied := EvaluateImpliedEndpoints(tree)
	offlineByName := make(map[string]config.ImpliedEndpoint, len(offlineImplied))
	for _, ep := range offlineImplied {
		offlineByName[ep.Name] = ep
	}
	assert.Equal(t, AutoLoadReasonConfigPath, offlineByName[rootPlugin].ReasonKind)
	assert.Equal(t, AutoLoadReasonConfigNeed, offlineByName[needProvider].ReasonKind)
	assert.Equal(t, AutoLoadReasonFamily, offlineByName[familyPlugin].ReasonKind)
	assert.NotContains(t, offlineByName, hardDepPlugin, "explicitly written endpoint must not appear as implied")
	assert.NotContains(t, offlineByName, absentSender, "absent sender must not be auto-loaded")

	// 3. Reception whose sender is absent becomes dormant and names the sender
	// plugin plus the exact bus { endpoint ... } line to load it (M-4 / Q-11(c)).
	compiled, err := CompileBus(NewTagInterner(), []BusParticipantInput{
		{
			Path:         plugin.EndpointPath("consumer"),
			EndpointName: "consumer",
			Receives:     []rpc.MessageReceiveDecl{{Message: absentSenderMsg}},
		},
	})
	require.NoError(t, err)

	impliedParticipants := BuildImpliedParticipants(compiled)
	require.Len(t, impliedParticipants, 1)
	require.Len(t, impliedParticipants[0].Receive, 1)
	recvRule := impliedParticipants[0].Receive[0]
	assert.True(t, recvRule.Dormant)
	assert.Equal(t, config.ImpliedOriginDormant, recvRule.Origin)
	assert.Contains(t, recvRule.DormantReason, absentSender)
	assert.Contains(t, recvRule.DormantReason, "bus { endpoint "+absentSender+" { internal "+absentSender+"; } }")
}

// TestLiveSingletonOwnBusAttributionAndTwoPlaces checks that Server's live
// DeliveryGraph compilation reads a singleton participant's own `bus` block
// from the running config tree (e.g. bgp > route-server > bus), merges it with
// bus > endpoint when both agree, and refuses when they disagree (S10).
//
// VALIDATES: Server.collectBusEndpointInputs + extractSingletonBusBlock + BuildImpliedParticipants.
// PREVENTS: live | implied reporting Origin "registered" when only the singleton's own bus block is written.
func TestLiveSingletonOwnBusAttributionAndTwoPlaces(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	msgUpdate := graphMsg(graphUpdate)
	rsPath := "/" + graphNS + "/route-server"
	groupTransit := "/" + graphNS + "/group/transit"
	peerT1 := groupTransit + "/peer/t1"
	peerT2 := groupTransit + "/peer/t2"
	peerEdge := "/" + graphNS + "/peer/edge1"

	require.NoError(t, registry.Register(registry.Registration{
		Name:             "test-rs",
		Description:      "singleton route-server test plugin",
		ParticipantNodes: []rpc.ParticipantNodeDecl{{SchemaPath: rsPath}},
		Sends:            []rpc.MessageSendDecl{{Message: msgUpdate}},
		Receives:         []rpc.MessageReceiveDecl{{Message: msgUpdate}},
		RunEngine:        func(net.Conn) int { return 0 },
		CLIHandler:       func([]string) int { return 0 },
	}))

	peers := []DeliveryPeer{
		{Addr: "127.0.0.2", Name: "t1", Path: peerT1, ParentPath: groupTransit},
		{Addr: "127.0.0.3", Name: "t2", Path: peerT2, ParentPath: groupTransit},
		{Addr: "127.0.0.4", Name: "edge1", Path: peerEdge},
	}

	makeServer := func(tree map[string]any, busCfg plugin.BusConfig) *Server {
		pm := process.NewProcessManager(nil)
		proc := process.NewProcess(plugin.PluginConfig{Name: "test-rs", CompiledPlugin: "test-rs"})
		proc.SetRegistration(&plugin.PluginRegistration{
			Sends:    []rpc.MessageSendDecl{{Message: msgUpdate}},
			Receives: []rpc.MessageReceiveDecl{{Message: msgUpdate}},
			Done:     true,
		})
		require.NoError(t, pm.AddProcess("test-rs", proc))
		s := &Server{reactor: &mockReloadReactor{tree: tree}}
		s.procManager.Store(pm)
		s.SetBusConfig(busCfg)
		s.SetBusConfigTree(tree)
		return s
	}

	findSendRule := func(t *testing.T, compiled *CompiledBus) config.ImpliedDirectionRule {
		t.Helper()
		for _, p := range BuildImpliedParticipants(compiled) {
			if p.Path != rsPath {
				continue
			}
			for _, r := range p.Send {
				if r.Message == msgUpdate {
					return r
				}
			}
		}
		t.Fatalf("missing %s send rule on %s", msgUpdate, rsPath)
		return config.ImpliedDirectionRule{}
	}

	busSideTransit := plugin.BusConfig{
		Endpoints: map[string]plugin.BusBlock{
			"test-rs": {
				Send: plugin.BusDirectionBlock{
					TargetsPresent: true,
					Targets:        []string{groupTransit},
				},
			},
		},
	}
	ownTreeTransit := map[string]any{
		graphNS: map[string]any{
			"route-server": map[string]any{
				"bus": map[string]any{
					"send": map[string]any{
						"to": []any{groupTransit},
					},
				},
			},
		},
	}
	ownTreeEdge := map[string]any{
		graphNS: map[string]any{
			"route-server": map[string]any{
				"bus": map[string]any{
					"send": map[string]any{
						"to": []any{peerEdge},
					},
				},
			},
		},
	}

	// (a) bus > endpoint test-rs only.
	sA := makeServer(nil, busSideTransit)
	require.NoError(t, sA.SetDeliveryGraph(graphNS, peers))
	ruleA := findSendRule(t, sA.DeliveryGraph().Compiled())
	assert.Equal(t, config.ImpliedOriginWritten, ruleA.Origin)
	assert.Contains(t, ruleA.Source, "test-rs")
	assert.Equal(t, []string{peerT1, peerT2}, ruleA.Targets)

	// (b) own section (bgp > route-server > bus) only.
	sB := makeServer(ownTreeTransit, plugin.BusConfig{})
	require.NoError(t, sB.SetDeliveryGraph(graphNS, peers))
	ruleB := findSendRule(t, sB.DeliveryGraph().Compiled())
	assert.Equal(t, config.ImpliedOriginWritten, ruleB.Origin)
	assert.Contains(t, ruleB.Source, "route-server")
	assert.Equal(t, []string{peerT1, peerT2}, ruleB.Targets)

	// (c) both places agreeing: source names both.
	sC := makeServer(ownTreeTransit, busSideTransit)
	require.NoError(t, sC.SetDeliveryGraph(graphNS, peers))
	ruleC := findSendRule(t, sC.DeliveryGraph().Compiled())
	assert.Equal(t, config.ImpliedOriginWritten, ruleC.Origin)
	assert.Contains(t, ruleC.Source, "test-rs")
	assert.Contains(t, ruleC.Source, "route-server")
	assert.Equal(t, []string{peerT1, peerT2}, ruleC.Targets)

	// (d) both places disagreeing: refused naming both.
	sD := makeServer(ownTreeEdge, busSideTransit)
	err := sD.SetDeliveryGraph(graphNS, peers)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test-rs")
	assert.Contains(t, err.Error(), "route-server")
}

// TestDeliveryGraphGroupInheritanceAttribution verifies that a grouped peer
// which writes its own `send > to` while inheriting `receive > from` from its
// parent group attributes the inherited receive rule as Origin "inherited"
// naming the group, and attributes the overridden receive rule as "written"
// once the peer writes its own receive rule (S8).
//
// VALIDATES: BuildDeliveryGraph parent container OwnBus inheritance + BuildImpliedParticipants.
// PREVENTS: flattened ResolveBGPTree bus blocks stamping inherited group rules as "written" on the peer.
func TestDeliveryGraphGroupInheritanceAttribution(t *testing.T) {
	msgUpdate := graphMsg(graphUpdate)
	groupPath := "/" + graphNS + "/group/transit"
	peerPath := groupPath + "/peer/t1"
	epRIB := plugin.EndpointPath("rib")
	epCollector := plugin.EndpointPath("collector")
	epSender := plugin.EndpointPath("sender")

	endpoints := []BusParticipantInput{
		{
			Path:         epRIB,
			EndpointName: "rib",
			Sends:        []rpc.MessageSendDecl{{Message: msgUpdate}},
			Receives:     []rpc.MessageReceiveDecl{{Message: msgUpdate}},
		},
		{
			Path:         epCollector,
			EndpointName: "collector",
			Receives:     []rpc.MessageReceiveDecl{{Message: msgUpdate}},
		},
		{
			Path:         epSender,
			EndpointName: "sender",
			Sends:        []rpc.MessageSendDecl{{Message: msgUpdate}},
		},
	}

	groupBlock := plugin.BusBlock{
		Send: plugin.BusDirectionBlock{
			TargetsPresent: true,
			Targets:        []string{epRIB},
		},
		Receive: plugin.BusDirectionBlock{
			TargetsPresent: true,
			Targets:        []string{epRIB},
		},
	}

	peerOwnSendOnly := plugin.BusBlock{
		Send: plugin.BusDirectionBlock{
			TargetsPresent: true,
			Targets:        []string{epCollector},
		},
	}

	// Run 1: peer overrides send > to, inherits receive > from from group transit.
	g1, err := BuildDeliveryGraph(graphNS, []DeliveryPeer{{
		Addr:            "127.0.0.2",
		Name:            "t1",
		Path:            peerPath,
		ParentPath:      groupPath,
		OwnBus:          peerOwnSendOnly,
		ParentBus:       groupBlock,
		ParentBusSource: groupPath + " > bus",
	}}, plugin.BusConfig{}, endpoints)
	require.NoError(t, err)

	var recvRule1, sendRule1 config.ImpliedDirectionRule
	for _, p := range BuildImpliedParticipants(g1.Compiled()) {
		if p.Path != peerPath {
			continue
		}
		for _, r := range p.Receive {
			if r.Message == msgUpdate {
				recvRule1 = r
			}
		}
		for _, r := range p.Send {
			if r.Message == msgUpdate {
				sendRule1 = r
			}
		}
	}
	assert.Equal(t, config.ImpliedOriginInherited, recvRule1.Origin)
	assert.Contains(t, recvRule1.Source, "transit")
	assert.Equal(t, []string{epRIB}, recvRule1.Targets)

	assert.Equal(t, config.ImpliedOriginWritten, sendRule1.Origin)
	assert.Contains(t, sendRule1.Source, peerPath)
	assert.Equal(t, []string{epCollector}, sendRule1.Targets)

	// Run 2: peer also overrides receive > from to sender -> Origin becomes "written".
	peerOwnBoth := peerOwnSendOnly.Clone()
	peerOwnBoth.Receive = plugin.BusDirectionBlock{
		TargetsPresent: true,
		Targets:        []string{epSender},
	}
	g2, err := BuildDeliveryGraph(graphNS, []DeliveryPeer{{
		Addr:            "127.0.0.2",
		Name:            "t1",
		Path:            peerPath,
		ParentPath:      groupPath,
		OwnBus:          peerOwnBoth,
		ParentBus:       groupBlock,
		ParentBusSource: groupPath + " > bus",
	}}, plugin.BusConfig{}, endpoints)
	require.NoError(t, err)

	var recvRule2 config.ImpliedDirectionRule
	for _, p := range BuildImpliedParticipants(g2.Compiled()) {
		if p.Path != peerPath {
			continue
		}
		for _, r := range p.Receive {
			if r.Message == msgUpdate {
				recvRule2 = r
			}
		}
	}
	assert.Equal(t, config.ImpliedOriginWritten, recvRule2.Origin)
	assert.Contains(t, recvRule2.Source, peerPath)
	assert.Equal(t, []string{epSender}, recvRule2.Targets)
}
