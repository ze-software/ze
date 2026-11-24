// Design: docs/architecture/api/architecture.md -- compiled-bus dissemination ownership.
// Related: dissemination.go -- roles are resolved from declarations and compiled bus filters.
package server

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const disseminationTestNamespace = "dissemination-test"
const disseminationTestRole = "selected-output"

func init() {
	_ = events.RegisterNamespace(disseminationTestNamespace, "update", "state", "refresh", "raw")
}

func disseminationMsg(name string) string {
	return disseminationTestNamespace + ":" + name
}

func disseminationTestDeclaration() rpc.DisseminationDecl {
	return rpc.DisseminationDecl{
		Namespace:          disseminationTestNamespace,
		Role:               disseminationTestRole,
		SourceReceive:      []string{"update-received", "state"},
		DestinationReceive: []string{"state", "refresh", "update-sent"},
		DestinationSend:    []string{"update"},
	}
}

func disseminationTestSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: disseminationMsg("update")},
		{Message: disseminationMsg("raw")},
	}
}

func disseminationTestReceives() []rpc.MessageReceiveDecl {
	return []rpc.MessageReceiveDecl{
		{Message: disseminationMsg("update"), Tags: []rpc.MessageTagDecl{{Name: events.TagDirection, Values: []string{events.DirectionReceived, events.DirectionSent}}}},
		{Message: disseminationMsg("state")},
		{Message: disseminationMsg("refresh")},
	}
}

func disseminationTestProcess(name string, explicit bool) DisseminationProcess {
	process := DisseminationProcess{Name: name, Declarations: []rpc.DisseminationDecl{disseminationTestDeclaration()}}
	if explicit {
		process.Claims = []string{disseminationTestRole}
	}
	return process
}

func disseminationEndpointInput(name string, block plugin.BusBlock) BusParticipantInput {
	return BusParticipantInput{
		Path:         plugin.EndpointPath(name),
		EndpointName: name,
		Sends:        disseminationTestSends(),
		Receives:     disseminationTestReceives(),
		OwnBlock:     block,
	}
}

// TestDisseminationOwnerUsesSameInstance proves source and destination grants
// complete one configured endpoint, never two aliases of the same implementation.
//
// VALIDATES: ResolveDisseminationOwner requires both source and destination on
// the same configured instance.
// PREVENTS: splitting source and destination grants across distinct aliases.
func TestDisseminationOwnerUsesSameInstance(t *testing.T) {
	p1 := "/" + disseminationTestNamespace + "/entry/p1"
	p2 := "/" + disseminationTestNamespace + "/entry/p2"
	peers := []DeliveryPeer{
		{Addr: "192.0.2.1", Name: "p1", Path: p1},
		{Addr: "192.0.2.2", Name: "p2", Path: p2},
	}
	processes := []DisseminationProcess{disseminationTestProcess("one", false), disseminationTestProcess("two", false)}

	// Endpoint "one" only receives from p1; endpoint "two" only interacts with p2.
	epOne := disseminationEndpointInput("one", plugin.BusBlock{
		Receive: plugin.BusDirectionBlock{TargetsPresent: true, Targets: []string{p1}},
		Send:    plugin.BusDirectionBlock{TargetsPresent: true, TargetsNone: true},
	})
	epTwo := disseminationEndpointInput("two", plugin.BusBlock{
		Receive: plugin.BusDirectionBlock{TargetsPresent: true, Targets: []string{p2}},
		Send:    plugin.BusDirectionBlock{TargetsPresent: true, Targets: []string{p2}},
	})
	// Narrow p2's update to sent only so p2 alone is not a complete source (no update-received).
	epTwo.OwnBlock.Receive.Messages = map[string]*plugin.BusMessageRule{
		disseminationMsg("update"): {Tags: map[string][]string{events.TagDirection: {events.DirectionSent}}},
	}

	graph, err := BuildDeliveryGraph(disseminationTestNamespace, peers, plugin.BusConfig{}, []BusParticipantInput{epOne, epTwo})
	require.NoError(t, err)
	owner, err := graph.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole, processes)
	require.NoError(t, err)
	require.False(t, owner.IsSet(), "different aliases must not complete each other's grants")

	// Now let endpoint "two" receive from both p1 and p2 (complete source on p1, destination on p2).
	epTwo.OwnBlock.Receive.Targets = []string{p1, p2}
	epTwo.OwnBlock.Receive.Messages = nil
	graph, err = BuildDeliveryGraph(disseminationTestNamespace, peers, plugin.BusConfig{}, []BusParticipantInput{epOne, epTwo})
	require.NoError(t, err)
	owner, err = graph.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole, processes)
	require.NoError(t, err)
	require.True(t, owner.IsSet())
	require.Equal(t, "two", owner.Name())
	require.False(t, owner.Explicit(), "ordinary output has not claimed the role")
}

// TestDisseminationOwnerRequiresCompleteGrants proves incomplete source or
// destination bus filters cannot request ordinary dissemination.
//
// VALIDATES: removing any required receive or send message disables ownership.
// PREVENTS: partial compiled-bus filters silently activating dissemination.
func TestDisseminationOwnerRequiresCompleteGrants(t *testing.T) {
	p1 := "/" + disseminationTestNamespace + "/entry/p1"
	p2 := "/" + disseminationTestNamespace + "/entry/p2"
	for _, defect := range []string{"none", "missing source update", "missing refresh", "missing sent updates", "raw only"} {
		t.Run(defect, func(t *testing.T) {
			ep := disseminationEndpointInput("ordinary", plugin.BusBlock{})
			switch defect {
			case "none":
			case "missing source update":
				ep.OwnBlock.Receive.Messages = map[string]*plugin.BusMessageRule{
					disseminationMsg("update"): {Tags: map[string][]string{events.TagDirection: {events.DirectionSent}}},
				}
			case "missing refresh":
				ep.OwnBlock.Receive.AllowPresent = true
				ep.OwnBlock.Receive.Allow = []string{disseminationMsg("update"), disseminationMsg("state")}
			case "missing sent updates":
				ep.OwnBlock.Receive.Messages = map[string]*plugin.BusMessageRule{
					disseminationMsg("update"): {Tags: map[string][]string{events.TagDirection: {events.DirectionReceived}}},
				}
			case "raw only":
				ep.OwnBlock.Send.AllowPresent = true
				ep.OwnBlock.Send.Allow = []string{disseminationMsg("raw")}
			}
			graph, err := BuildDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{
				{Addr: "192.0.2.1", Name: "p1", Path: p1},
				{Addr: "192.0.2.2", Name: "p2", Path: p2},
			}, plugin.BusConfig{}, []BusParticipantInput{ep})
			require.NoError(t, err)
			owner, err := graph.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole, []DisseminationProcess{disseminationTestProcess("ordinary", false)})
			require.NoError(t, err)
			require.Equal(t, defect == "none", owner.IsSet())
		})
	}
}

// TestDisseminationOwnerScopesExplicitClaim proves an eligible explicit role
// displaces ordinary output only at the destination covered by its bus filter.
//
// VALIDATES: explicit claim precedence scoped by send > to / receive > from.
// PREVENTS: an explicit claimant displacing ordinary output on uncovered peers.
func TestDisseminationOwnerScopesExplicitClaim(t *testing.T) {
	p1 := "/" + disseminationTestNamespace + "/entry/p1"
	p2 := "/" + disseminationTestNamespace + "/entry/p2"
	p3 := "/" + disseminationTestNamespace + "/entry/p3"
	graph, err := BuildDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{
		{Addr: "192.0.2.1", Name: "p1", Path: p1},
		{Addr: "192.0.2.2", Name: "p2", Path: p2},
		{Addr: "192.0.2.3", Name: "p3", Path: p3},
	}, plugin.BusConfig{}, []BusParticipantInput{
		disseminationEndpointInput("ordinary", plugin.BusBlock{}),
		disseminationEndpointInput("role-alias", plugin.BusBlock{
			Send: plugin.BusDirectionBlock{TargetsPresent: true, Targets: []string{p2}},
		}),
	})
	require.NoError(t, err)
	processes := []DisseminationProcess{disseminationTestProcess("ordinary", false), disseminationTestProcess("role-alias", true)}
	role, err := graph.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole, processes)
	require.NoError(t, err)
	require.Equal(t, "role-alias", role.Name())
	require.True(t, role.Explicit())
	ordinary, err := graph.ResolveDisseminationOwner("192.0.2.3", disseminationTestRole, processes)
	require.NoError(t, err)
	require.Equal(t, "ordinary", ordinary.Name())
	require.False(t, ordinary.Explicit())
}

// TestDisseminationOwnerRefusesAmbiguousClaim proves claim order cannot pick
// between two explicitly eligible instances.
//
// VALIDATES: ambiguous explicit claimants are refused naming peer and both endpoints.
// PREVENTS: order-dependent silent winner selection.
func TestDisseminationOwnerRefusesAmbiguousClaim(t *testing.T) {
	graph, err := BuildDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{
		{Addr: "192.0.2.1", Name: "source", Path: "/" + disseminationTestNamespace + "/entry/source"},
		{Addr: "192.0.2.2", Name: "destination", Path: "/" + disseminationTestNamespace + "/entry/destination"},
	}, plugin.BusConfig{}, []BusParticipantInput{
		disseminationEndpointInput("left", plugin.BusBlock{}),
		disseminationEndpointInput("right", plugin.BusBlock{}),
	})
	require.NoError(t, err)
	owner, err := graph.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole, []DisseminationProcess{disseminationTestProcess("right", true), disseminationTestProcess("left", true)})
	require.Error(t, err)
	require.False(t, owner.IsSet())
	require.ErrorContains(t, err, "destination")
	require.ErrorContains(t, err, "left")
	require.ErrorContains(t, err, "right")
}

// TestDisseminationOwnerRefusesAmbiguousOrdinary proves two independent
// ordinary instances cannot select the same destination without one owner.
//
// VALIDATES: ambiguous ordinary owners are refused naming both endpoints.
// PREVENTS: order-dependent silent winner selection between ordinary owners.
func TestDisseminationOwnerRefusesAmbiguousOrdinary(t *testing.T) {
	graph, err := BuildDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{
		{Addr: "192.0.2.1", Name: "source", Path: "/" + disseminationTestNamespace + "/entry/source"},
		{Addr: "192.0.2.2", Name: "destination", Path: "/" + disseminationTestNamespace + "/entry/destination"},
	}, plugin.BusConfig{}, []BusParticipantInput{
		disseminationEndpointInput("left", plugin.BusBlock{}),
		disseminationEndpointInput("right", plugin.BusBlock{}),
	})
	require.NoError(t, err)
	owner, err := graph.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole, []DisseminationProcess{disseminationTestProcess("left", false), disseminationTestProcess("right", false)})
	require.Error(t, err)
	require.False(t, owner.IsSet())
	require.ErrorContains(t, err, "left")
	require.ErrorContains(t, err, "right")
}

// TestDisseminationOwnerDeclarationTransports proves alias resolution and a
// genuine external declaration preserve the same configured owner.
//
// VALIDATES: compiled alias and runtime Stage 1 declaration resolve identically.
// PREVENTS: falling back to a compiled claim after an empty runtime claim.
func TestDisseminationOwnerDeclarationTransports(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, registry.Register(registry.Registration{
		Name: "dissemination-implementation", RunEngine: func(net.Conn) int { return 0 }, CLIHandler: func([]string) int { return 0 },
		Claims: []string{disseminationTestRole}, Dissemination: []rpc.DisseminationDecl{disseminationTestDeclaration()},
		Sends: disseminationTestSends(), Receives: disseminationTestReceives(),
	}))
	for _, external := range []bool{false, true} {
		name := "configured-alias"
		configuration := plugin.PluginConfig{Name: name, CompiledPlugin: "dissemination-implementation", Internal: true}
		server := &Server{config: &ServerConfig{Plugins: []plugin.PluginConfig{configuration}}}
		if external {
			name = "external-not-in-registry"
			configuration = plugin.PluginConfig{Name: name, Run: "external-program"}
			server.config.Plugins = []plugin.PluginConfig{configuration}
			declaration := rpc.DeclareRegistrationInput{
				Claims:        []string{disseminationTestRole},
				Dissemination: []rpc.DisseminationDecl{disseminationTestDeclaration()},
				Sends:         disseminationTestSends(),
				Receives:      disseminationTestReceives(),
			}
			encoded, err := json.Marshal(declaration)
			require.NoError(t, err)
			var decoded rpc.DeclareRegistrationInput
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			owner := process.NewProcess(configuration)
			owner.SetRegistration(registrationFromRPC(&decoded))
			manager := process.NewProcessManager(nil)
			require.NoError(t, manager.AddProcess(name, owner))
			server.procManager.Store(manager)
		}
		require.NoError(t, server.SetDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{
			{Addr: "192.0.2.1", Name: "p1", Path: "/" + disseminationTestNamespace + "/entry/p1"},
			{Addr: "192.0.2.2", Name: "p2", Path: "/" + disseminationTestNamespace + "/entry/p2"},
		}))
		owner, err := server.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole)
		require.NoError(t, err)
		require.Equal(t, name, owner.Name())
		require.True(t, owner.Explicit())
		if external {
			server.procManager.Load().GetProcess(name).SetRegistration(&plugin.PluginRegistration{
				Dissemination: []rpc.DisseminationDecl{disseminationTestDeclaration()},
				Sends:         disseminationTestSends(),
				Receives:      disseminationTestReceives(),
			})
			owner, err = server.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole)
			require.NoError(t, err)
			require.False(t, owner.Explicit(), "an authenticated empty runtime claim is authoritative")
		}
	}
}
