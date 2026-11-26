// Design: docs/architecture/api/architecture.md -- Stage 2 and live rebuild participant filter wire delivery
// Related: delivery_graph.go -- Stage2FilterFor and UpdateDeliveryGraphWithEndpoints
// Related: startup.go -- deliverConfigRPC

package server

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestStage2FilterForPopulatesScopeFlagsAndPeerAddresses verifies that
// Stage2FilterFor projects unscoped, explicit, and none target scopes together
// with concrete remote IP addresses for static and dynamic peers.
//
// VALIDATES: Stage2FilterFor sets SendToAll, SendToNone, ReceiveFromAll,
// ReceiveFromNone, per-rule TargetAll and TargetNone, and populates
// BusFilterTarget.Address with each static and dynamic peer's concrete IP.
// PREVENTS: plugins being unable to tell unscoped all-peers (including future
// dynamic peers) from explicit peer sets or explicit none, or losing peer IP
// addresses on resolved targets.
func TestStage2FilterForPopulatesScopeFlagsAndPeerAddresses(t *testing.T) {
	t.Parallel()

	staticPath := "/" + graphNS + "/entry/static1"
	dynPath := "/" + graphNS + "/ip/198.51.100.42"
	groupPath := "/" + graphNS + "/group/ixp"

	peers := []DeliveryPeer{
		{
			Addr:           "192.0.2.10",
			Name:           "static1",
			Path:           staticPath,
			SecondaryPaths: []string{"/" + graphNS + "/ip/192.0.2.10"},
		},
		{
			Addr:       "198.51.100.42",
			Path:       dynPath,
			ParentPath: groupPath,
			Dynamic:    true,
		},
	}

	endpoints := []BusParticipantInput{
		{
			Path:         plugin.EndpointPath("ep-unscoped"),
			EndpointName: "ep-unscoped",
			Sends:        []rpc.MessageSendDecl{{Message: graphMsg(graphUpdate)}},
			Receives:     []rpc.MessageReceiveDecl{{Message: graphMsg(graphUpdate)}},
		},
		{
			Path:         plugin.EndpointPath("ep-scoped"),
			EndpointName: "ep-scoped",
			Sends:        []rpc.MessageSendDecl{{Message: graphMsg(graphUpdate)}},
			Receives:     []rpc.MessageReceiveDecl{{Message: graphMsg(graphUpdate)}},
			OwnBlock: plugin.BusBlock{
				Send:    plugin.BusDirectionBlock{TargetsPresent: true, Targets: []string{staticPath}},
				Receive: plugin.BusDirectionBlock{TargetsPresent: true, Targets: []string{groupPath}},
			},
		},
		{
			Path:         plugin.EndpointPath("ep-none"),
			EndpointName: "ep-none",
			Sends:        []rpc.MessageSendDecl{{Message: graphMsg(graphUpdate)}},
			Receives:     []rpc.MessageReceiveDecl{{Message: graphMsg(graphUpdate)}},
			OwnBlock: plugin.BusBlock{
				Send:    plugin.BusDirectionBlock{TargetsPresent: true, TargetsNone: true},
				Receive: plugin.BusDirectionBlock{TargetsPresent: true, TargetsNone: true},
			},
		},
	}

	g, err := BuildDeliveryGraph(graphNS, peers, plugin.BusConfig{}, endpoints)
	require.NoError(t, err)

	// 1. Unscoped endpoint: SendToAll/ReceiveFromAll true, both peers with IPs.
	unscoped := g.Stage2FilterFor("ep-unscoped")
	require.NotNil(t, unscoped)
	assert.True(t, unscoped.SendToAll)
	assert.False(t, unscoped.SendToNone)
	assert.True(t, unscoped.ReceiveFromAll)
	assert.False(t, unscoped.ReceiveFromNone)
	require.Len(t, unscoped.SendRules, 1)
	assert.True(t, unscoped.SendRules[0].TargetAll)
	assert.False(t, unscoped.SendRules[0].TargetNone)
	addrByPath := make(map[string]string, len(unscoped.SendRules[0].Targets))
	for _, tgt := range unscoped.SendRules[0].Targets {
		addrByPath[tgt.Path] = tgt.Address
	}
	assert.Equal(t, "192.0.2.10", addrByPath[staticPath])
	assert.Equal(t, "198.51.100.42", addrByPath[dynPath])
	require.Len(t, unscoped.ReceiveRules, 1)
	assert.True(t, unscoped.ReceiveRules[0].TargetAll)
	assert.False(t, unscoped.ReceiveRules[0].TargetNone)

	// 2. Scoped endpoint: neither All nor None, concrete addresses resolved.
	scoped := g.Stage2FilterFor("ep-scoped")
	require.NotNil(t, scoped)
	assert.False(t, scoped.SendToAll)
	assert.False(t, scoped.SendToNone)
	assert.False(t, scoped.ReceiveFromAll)
	assert.False(t, scoped.ReceiveFromNone)
	require.Len(t, scoped.SendRules, 1)
	assert.False(t, scoped.SendRules[0].TargetAll)
	assert.False(t, scoped.SendRules[0].TargetNone)
	require.Len(t, scoped.SendRules[0].Targets, 1)
	assert.Equal(t, staticPath, scoped.SendRules[0].Targets[0].Path)
	assert.Equal(t, "192.0.2.10", scoped.SendRules[0].Targets[0].Address)
	require.Len(t, scoped.ReceiveRules, 1)
	assert.False(t, scoped.ReceiveRules[0].TargetAll)
	assert.False(t, scoped.ReceiveRules[0].TargetNone)
	require.Len(t, scoped.ReceiveRules[0].Targets, 1)
	assert.Equal(t, dynPath, scoped.ReceiveRules[0].Targets[0].Path)
	assert.Equal(t, "198.51.100.42", scoped.ReceiveRules[0].Targets[0].Address)

	// 3. Explicit none endpoint: SendToNone/ReceiveFromNone true, no targets.
	noneFilter := g.Stage2FilterFor("ep-none")
	require.NotNil(t, noneFilter)
	assert.False(t, noneFilter.SendToAll)
	assert.True(t, noneFilter.SendToNone)
	assert.False(t, noneFilter.ReceiveFromAll)
	assert.True(t, noneFilter.ReceiveFromNone)
	require.Len(t, noneFilter.SendRules, 1)
	assert.False(t, noneFilter.SendRules[0].TargetAll)
	assert.True(t, noneFilter.SendRules[0].TargetNone)
	assert.Empty(t, noneFilter.SendRules[0].Targets)
	require.Len(t, noneFilter.ReceiveRules, 1)
	assert.False(t, noneFilter.ReceiveRules[0].TargetAll)
	assert.True(t, noneFilter.ReceiveRules[0].TargetNone)
	assert.Empty(t, noneFilter.ReceiveRules[0].Targets)
}

// TestDeliverConfigRPCRefreshesGraphForExternalStage1Endpoint verifies that
// deliverConfigRPC refreshes the delivery graph when a Phase 2 external
// endpoint completed Stage 1 after the last graph build, so Stage 2 configure
// carries its compiled ParticipantFilter.
//
// VALIDATES: deliverConfigRPC calls RefreshDeliveryGraph when Stage2FilterFor
// returns nil (external endpoint just finished Stage 1) and puts the compiled
// ParticipantFilter onto the ze-plugin-callback:configure RPC.
// PREVENTS: Phase 2 external endpoints receiving a nil ParticipantFilter at
// Stage 2 because the delivery graph had not yet incorporated their Stage 1
// registration.
func TestDeliverConfigRPCRefreshesGraphForExternalStage1Endpoint(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	engineConn, pluginMux := newDriverPipe(t)

	s := &Server{
		config:        &ServerConfig{},
		registry:      plugin.NewPluginRegistry(),
		ctx:           ctx,
		loadedPlugins: make(map[string]bool),
	}
	t.Cleanup(s.Stop)

	peerPath := "/" + graphNS + "/entry/p1"
	require.NoError(t, s.UpdateDeliveryGraphWithEndpoints(graphNS, []DeliveryPeer{
		{Addr: "192.0.2.1", Name: "p1", Path: peerPath},
	}, nil))
	require.Nil(t, s.DeliveryGraph().Stage2FilterFor("ext-late"),
		"external endpoint must not yet be in the graph before Stage 1")

	proc := process.NewProcess(plugin.PluginConfig{Name: "ext-late"})
	proc.SetRegistration(&plugin.PluginRegistration{
		Done:     true,
		Sends:    []rpc.MessageSendDecl{{Message: graphMsg(graphUpdate)}},
		Receives: []rpc.MessageReceiveDecl{{Message: graphMsg(graphUpdate)}},
	})
	proc.SetConn(engineConn)

	pm := process.NewProcessManager(nil)
	require.NoError(t, pm.AddProcess("ext-late", proc))
	s.procManager.Store(pm)

	seen := make(chan *rpc.BusParticipantFilter, 1)
	go func() {
		select {
		case req := <-pluginMux.Requests():
			var input rpc.ConfigureInput
			if err := json.Unmarshal(req.Params, &input); err == nil {
				seen <- input.ParticipantFilter
			}
			_ = pluginMux.SendOK(ctx, req.ID)
		case <-ctx.Done():
		}
	}()

	require.NoError(t, s.deliverConfigRPC(ctx, proc))

	select {
	case got := <-seen:
		require.NotNil(t, got, "Stage 2 configure must carry the refreshed ParticipantFilter")
		assert.Equal(t, plugin.EndpointPath("ext-late"), got.ParticipantPath)
		assert.True(t, got.SendToAll)
		require.Len(t, got.SendRules, 1)
		require.Len(t, got.SendRules[0].Targets, 1)
		assert.Equal(t, "192.0.2.1", got.SendRules[0].Targets[0].Address)
	case <-time.After(2 * time.Second):
		t.Fatal("plugin did not receive configure RPC")
	}
}

// TestUpdateDeliveryGraphWithEndpointsPushesFilterToRunningPlugins verifies
// that delivery-graph rebuilds push updated participant filters asynchronously
// to running plugins and silently ignore minimal plugins that answer
// not-understood.
//
// VALIDATES: UpdateDeliveryGraphWithEndpoints pushes rpc.MethodParticipantFilter
// to StageRunning processes without blocking the caller, delivers updated
// resolved targets (including newly admitted peers), and does not fail or
// stall when a minimal plugin answers "not-understood".
// PREVENTS: running plugins holding stale target lists after config reload or
// dynamic peer admission, or minimal external plugins breaking graph rebuilds.
func TestUpdateDeliveryGraphWithEndpointsPushesFilterToRunningPlugins(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	liveConn, livePluginMux := newDriverPipe(t)
	minimalConn, minimalPluginMux := newDriverPipe(t)

	s := &Server{
		config:        &ServerConfig{},
		registry:      plugin.NewPluginRegistry(),
		ctx:           ctx,
		loadedPlugins: make(map[string]bool),
	}
	t.Cleanup(s.Stop)

	liveProc := process.NewProcess(plugin.PluginConfig{Name: "ep-live"})
	liveProc.SetRunning(true)
	liveProc.SetStage(plugin.StageRunning)
	liveProc.SetConn(liveConn)

	minimalProc := process.NewProcess(plugin.PluginConfig{Name: "ep-minimal"})
	minimalProc.SetRunning(true)
	minimalProc.SetStage(plugin.StageRunning)
	minimalProc.SetConn(minimalConn)

	pm := process.NewProcessManager(nil)
	require.NoError(t, pm.AddProcess("ep-live", liveProc))
	require.NoError(t, pm.AddProcess("ep-minimal", minimalProc))
	s.procManager.Store(pm)

	received := make(chan *rpc.BusParticipantFilter, 4)
	go func() {
		for {
			select {
			case req, ok := <-livePluginMux.Requests():
				if !ok {
					return
				}
				if req.Method == rpc.MethodParticipantFilter {
					var input rpc.ParticipantFilterInput
					if err := json.Unmarshal(req.Params, &input); err == nil {
						received <- input.Filter
					}
				}
				_ = livePluginMux.SendOK(ctx, req.ID)
			case <-ctx.Done():
				return
			}
		}
	}()

	minimalSeen := make(chan struct{}, 2)
	go func() {
		for {
			select {
			case req, ok := <-minimalPluginMux.Requests():
				if !ok {
					return
				}
				select {
				case minimalSeen <- struct{}{}:
				default:
				}
				_ = minimalPluginMux.SendError(ctx, req.ID, "not-understood")
			case <-ctx.Done():
				return
			}
		}
	}()

	endpoints := []BusParticipantInput{
		{
			Path:         plugin.EndpointPath("ep-live"),
			EndpointName: "ep-live",
			Sends:        []rpc.MessageSendDecl{{Message: graphMsg(graphUpdate)}},
		},
		{
			Path:         plugin.EndpointPath("ep-minimal"),
			EndpointName: "ep-minimal",
			Sends:        []rpc.MessageSendDecl{{Message: graphMsg(graphUpdate)}},
		},
	}

	require.NoError(t, s.UpdateDeliveryGraphWithEndpoints(graphNS, []DeliveryPeer{
		{Addr: "192.0.2.1", Name: "p1", Path: "/" + graphNS + "/entry/p1"},
		{Addr: "198.51.100.7", Path: "/" + graphNS + "/ip/198.51.100.7", Dynamic: true},
	}, endpoints))

	select {
	case got := <-received:
		require.NotNil(t, got)
		assert.Equal(t, plugin.EndpointPath("ep-live"), got.ParticipantPath)
		assert.True(t, got.SendToAll)
		require.Len(t, got.SendRules, 1)
		assert.True(t, got.SendRules[0].TargetAll)
		require.Len(t, got.SendRules[0].Targets, 2)
		addrs := []string{got.SendRules[0].Targets[0].Address, got.SendRules[0].Targets[1].Address}
		assert.ElementsMatch(t, []string{"192.0.2.1", "198.51.100.7"}, addrs)
	case <-time.After(2 * time.Second):
		t.Fatal("running plugin did not receive live participant-filter push")
	}

	select {
	case <-minimalSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("minimal plugin did not receive push attempt")
	}
}

// TestRefreshDeliveryGraphDoesNotClobberConcurrentPeerPublish verifies that
// concurrent RefreshDeliveryGraph calls (as happen during Phase 1 Stage 2
// handshakes) cannot overwrite a peer set published by SetDeliveryGraph with a
// stale empty snapshot.
//
// VALIDATES: deliveryGraphMu serializes RefreshDeliveryGraph read-modify-write
// cycles against SetDeliveryGraph so resolved peer addresses are never lost.
// PREVENTS: Stage2FilterFor delivering empty Target.Address under CPU load.
func TestRefreshDeliveryGraphDoesNotClobberConcurrentPeerPublish(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s := &Server{
		config:        &ServerConfig{},
		registry:      plugin.NewPluginRegistry(),
		ctx:           ctx,
		loadedPlugins: make(map[string]bool),
	}
	t.Cleanup(s.Stop)

	peerPath := "/" + graphNS + "/entry/peer1"
	const peerAddr = "192.0.2.10"
	peers := []DeliveryPeer{{Addr: peerAddr, Name: "peer1", Path: peerPath}}

	for range 64 {
		s.deliveryGraph.Store(emptyDeliveryGraph)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				s.RefreshDeliveryGraph()
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			require.NoError(t, s.SetDeliveryGraph(graphNS, peers))
		}()
		close(start)
		wg.Wait()

		got := s.DeliveryGraph()
		require.Equal(t, graphNS, got.namespace, "concurrent RefreshDeliveryGraph clobbered namespace")
		require.Len(t, got.disseminationPeers, 1, "concurrent RefreshDeliveryGraph clobbered published peer set")
		require.Equal(t, peerAddr, got.pathToAddr[peerPath], "resolved peer address lost after concurrent refresh")
	}
}
