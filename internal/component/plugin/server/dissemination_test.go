// Design: docs/architecture/api/architecture.md -- explicit peer dissemination ownership.
// Related: dissemination.go -- roles are resolved from declarations and actual grants.
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
	_ = events.RegisterNamespace(disseminationTestNamespace, "update", "state", "refresh")
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

func disseminationTestProcess(name string, explicit bool) DisseminationProcess {
	process := DisseminationProcess{Name: name, Declarations: []rpc.DisseminationDecl{disseminationTestDeclaration()}}
	if explicit {
		process.Claims = []string{disseminationTestRole}
	}
	return process
}

func disseminationTestSource(process string) plugin.PeerProcessBinding {
	return plugin.PeerProcessBinding{
		PluginName: process,
		Explicit:   true,
		Receive:    map[string]events.Direction{"update": events.DirReceived, "state": events.DirBoth},
	}
}

func disseminationTestDestination(process string) plugin.PeerProcessBinding {
	return plugin.PeerProcessBinding{
		PluginName: process,
		Explicit:   true,
		Receive: map[string]events.Direction{
			"state": events.DirBoth, "refresh": events.DirBoth, "update": events.DirSent,
		},
		Send: map[string]bool{"update": true},
	}
}

// TestDisseminationOwnerUsesSameInstance proves source and destination grants
// complete one configured process, never two aliases of the same implementation.
// Method: split grants across instances, then complete only the destination's instance.
func TestDisseminationOwnerUsesSameInstance(t *testing.T) {
	peers := []DeliveryPeer{
		{Addr: "192.0.2.1", Bindings: []plugin.PeerProcessBinding{disseminationTestSource("one")}},
		{Addr: "192.0.2.2", Bindings: []plugin.PeerProcessBinding{disseminationTestDestination("two")}},
	}
	processes := []DisseminationProcess{disseminationTestProcess("one", false), disseminationTestProcess("two", false)}
	graph := newDeliveryGraph(disseminationTestNamespace, peers)
	owner, err := graph.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole, processes)
	require.NoError(t, err)
	require.False(t, owner.IsSet(), "different aliases must not complete each other's grants")
	peers[0].Bindings = append(peers[0].Bindings, disseminationTestSource("two"))
	graph = newDeliveryGraph(disseminationTestNamespace, peers)
	owner, err = graph.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole, processes)
	require.NoError(t, err)
	require.True(t, owner.IsSet())
	require.Equal(t, "two", owner.Name())
	require.False(t, owner.Explicit(), "ordinary output has not claimed the role")
}

// TestDisseminationOwnerRequiresExplicitCompleteGrants proves derived feeds,
// implicit state and incomplete destination grants cannot request ordinary transit.
// Method: remove one authority fact at a time while retaining the useful complete case.
func TestDisseminationOwnerRequiresExplicitCompleteGrants(t *testing.T) {
	for _, defect := range []string{"none", "derived source", "derived destination", "implicit source state", "missing refresh", "missing sent updates", "raw only"} {
		t.Run(defect, func(t *testing.T) {
			source := disseminationTestSource("ordinary")
			target := disseminationTestDestination("ordinary")
			switch defect {
			case "none":
			case "derived source":
				source.Explicit = false
			case "derived destination":
				target.Explicit = false
			case "implicit source state":
				source.Implicit = map[string]bool{"state": true}
			case "missing refresh":
				delete(target.Receive, "refresh")
			case "missing sent updates":
				delete(target.Receive, "update")
			case "raw only":
				target.Send = map[string]bool{"raw": true}
			}
			graph := newDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{
				{Addr: "192.0.2.1", Bindings: []plugin.PeerProcessBinding{source}},
				{Addr: "192.0.2.2", Bindings: []plugin.PeerProcessBinding{target}},
			})
			owner, err := graph.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole, []DisseminationProcess{disseminationTestProcess("ordinary", false)})
			require.NoError(t, err)
			require.Equal(t, defect == "none", owner.IsSet())
		})
	}
}

// TestDisseminationOwnerScopesExplicitClaim proves an eligible explicit role
// displaces ordinary output only at the destination that grants that instance.
// Method: compare a role destination with a sibling ordinary destination.
func TestDisseminationOwnerScopesExplicitClaim(t *testing.T) {
	graph := newDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{
		{Addr: "192.0.2.1", Bindings: []plugin.PeerProcessBinding{disseminationTestSource("ordinary"), disseminationTestSource("role-alias")}},
		{Addr: "192.0.2.2", Bindings: []plugin.PeerProcessBinding{disseminationTestDestination("ordinary"), disseminationTestDestination("role-alias")}},
		{Addr: "192.0.2.3", Bindings: []plugin.PeerProcessBinding{disseminationTestDestination("ordinary")}},
	})
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
// between two explicitly eligible instances. Method: grant both complete sets
// and require the refusal to identify the peer and both configured names.
func TestDisseminationOwnerRefusesAmbiguousClaim(t *testing.T) {
	graph := newDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{
		{Addr: "192.0.2.1", Bindings: []plugin.PeerProcessBinding{disseminationTestSource("left"), disseminationTestSource("right")}},
		{Addr: "192.0.2.2", Name: "destination", Bindings: []plugin.PeerProcessBinding{disseminationTestDestination("left"), disseminationTestDestination("right")}},
	})
	owner, err := graph.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole, []DisseminationProcess{disseminationTestProcess("right", true), disseminationTestProcess("left", true)})
	require.Error(t, err)
	require.False(t, owner.IsSet())
	require.ErrorContains(t, err, "destination")
	require.ErrorContains(t, err, "left")
	require.ErrorContains(t, err, "right")
}

// TestDisseminationOwnerRefusesAmbiguousOrdinary proves two independent RIB
// instances cannot select the same destination without one effective owner.
// Method: reuse complete grants with neither process claiming the role.
func TestDisseminationOwnerRefusesAmbiguousOrdinary(t *testing.T) {
	graph := newDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{
		{Addr: "192.0.2.1", Bindings: []plugin.PeerProcessBinding{disseminationTestSource("left"), disseminationTestSource("right")}},
		{Addr: "192.0.2.2", Bindings: []plugin.PeerProcessBinding{disseminationTestDestination("left"), disseminationTestDestination("right")}},
	})
	owner, err := graph.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole, []DisseminationProcess{disseminationTestProcess("left", false), disseminationTestProcess("right", false)})
	require.Error(t, err)
	require.False(t, owner.IsSet())
	require.ErrorContains(t, err, "left")
	require.ErrorContains(t, err, "right")
}

// TestDisseminationOwnerDeclarationTransports proves alias resolution and a
// genuine external declaration preserve the same configured owner. Method:
// compare a compiled alias with JSON Stage 1 input absent from the registry,
// then remove its runtime claim without falling back to a compiled claim.
func TestDisseminationOwnerDeclarationTransports(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, registry.Register(registry.Registration{
		Name: "dissemination-implementation", RunEngine: func(net.Conn) int { return 0 }, CLIHandler: func([]string) int { return 0 },
		Claims: []string{disseminationTestRole}, Dissemination: []rpc.DisseminationDecl{disseminationTestDeclaration()},
	}))
	for _, external := range []bool{false, true} {
		name := "configured-alias"
		configuration := plugin.PluginConfig{Name: name, Run: "dissemination-implementation"}
		server := &Server{config: &ServerConfig{Plugins: []plugin.PluginConfig{configuration}}}
		if external {
			name = "external-not-in-registry"
			configuration = plugin.PluginConfig{Name: name, Run: "external-program"}
			server.config.Plugins = []plugin.PluginConfig{configuration}
			declaration := rpc.DeclareRegistrationInput{Claims: []string{disseminationTestRole}, Dissemination: []rpc.DisseminationDecl{disseminationTestDeclaration()}}
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
		server.deliveryGraph.Store(newDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{
			{Addr: "192.0.2.1", Bindings: []plugin.PeerProcessBinding{disseminationTestSource(name)}},
			{Addr: "192.0.2.2", Bindings: []plugin.PeerProcessBinding{disseminationTestDestination(name)}},
		}))
		owner, err := server.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole)
		require.NoError(t, err)
		require.Equal(t, name, owner.Name())
		require.True(t, owner.Explicit())
		if external {
			server.procManager.Load().GetProcess(name).SetRegistration(&plugin.PluginRegistration{Dissemination: []rpc.DisseminationDecl{disseminationTestDeclaration()}})
			owner, err = server.ResolveDisseminationOwner("192.0.2.2", disseminationTestRole)
			require.NoError(t, err)
			require.False(t, owner.Explicit(), "an authenticated empty runtime claim is authoritative")
		}
	}
}
