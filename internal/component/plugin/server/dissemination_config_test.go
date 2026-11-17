// Design: docs/architecture/api/process-protocol.md -- candidate Stage 1 grants.
package server

import (
	"encoding/json"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func candidateDisseminationResolver(t *testing.T, resolve DisseminationConfigResolver) {
	t.Helper()
	disseminationConfigResolvers.Lock()
	before := maps.Clone(disseminationConfigResolvers.byNamespace)
	disseminationConfigResolvers.byNamespace = make(map[string]DisseminationConfigResolver)
	disseminationConfigResolvers.Unlock()
	t.Cleanup(func() {
		disseminationConfigResolvers.Lock()
		disseminationConfigResolvers.byNamespace = before
		disseminationConfigResolvers.Unlock()
	})
	if resolve != nil {
		require.NoError(t, RegisterDisseminationConfigResolver(disseminationTestNamespace, resolve))
	}
}

// TestDisseminationStage1UsesCandidateScope drives the real registration sink
// for a program absent from the compiled registry. Opposite active/candidate
// grant sets prove neither transport decoding nor an old graph decides startup.
func TestDisseminationStage1UsesCandidateScope(t *testing.T) {
	for _, complete := range []bool{true, false} {
		t.Run(map[bool]string{true: "new complete grants", false: "new incomplete grants"}[complete], func(t *testing.T) {
			const name = "external-candidate-table"
			require.Nil(t, registry.Lookup(name))
			peers := func(valid bool) []DeliveryPeer {
				destination := disseminationTestDestination(name)
				if !valid {
					destination.Receive = nil
				}
				return []DeliveryPeer{
					{Addr: "192.0.2.1", Bindings: []plugin.PeerProcessBinding{disseminationTestSource(name)}},
					{Addr: "192.0.2.2", Bindings: []plugin.PeerProcessBinding{destination}},
				}
			}
			candidateDisseminationResolver(t, func(tree map[string]any) ([]DeliveryPeer, error) {
				valid, known := tree["complete"].(bool)
				require.True(t, known, "resolver must read the selected candidate")
				return peers(valid), nil
			})
			s, _ := newLifecycleStartupServer(t)
			configuration := plugin.PluginConfig{Name: name, Run: "external-program"}
			s.config.Plugins = []plugin.PluginConfig{configuration}
			proc := process.NewProcess(configuration)
			manager := process.NewProcessManager(nil)
			manager.AddProcess(name, proc)
			s.procManager.Store(manager)
			active := newDeliveryGraph(disseminationTestNamespace, peers(!complete))
			s.deliveryGraph.Store(active)
			tree := map[string]any{"complete": complete}
			require.NoError(t, s.bindConfigureConfig([]*process.Process{proc}, configureConfig{tree: tree}))
			defer s.clearConfigureConfig([]*process.Process{proc})
			// The graph owns its parsed values, not a mutable resolver result.
			tree["complete"] = !complete
			input := &rpc.DeclareRegistrationInput{ProtocolVersion: rpc.ProtocolVersion,
				Dissemination: []rpc.DisseminationDecl{disseminationTestDeclaration()}}
			wire, err := json.Marshal(input)
			require.NoError(t, err)
			var decoded rpc.DeclareRegistrationInput
			require.NoError(t, json.Unmarshal(wire, &decoded))
			err = (&engineStartupSink{s: s, proc: proc}).onRegistration(&decoded)
			if complete {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "incomplete destination grants")
			}
			require.Same(t, active, s.DeliveryGraph(), "startup validation never publishes the candidate")
		})
	}
}

// TestDisseminationCandidateNeedsNamespaceResolver proves a missing producer
// refuses instead of using a plausible active graph for a different candidate.
func TestDisseminationCandidateNeedsNamespaceResolver(t *testing.T) {
	candidateDisseminationResolver(t, nil)
	const name = "external-no-resolver"
	s, _ := newLifecycleStartupServer(t)
	configuration := plugin.PluginConfig{Name: name, Run: "external-program"}
	s.config.Plugins = []plugin.PluginConfig{configuration}
	proc := process.NewProcess(configuration)
	proc.SetRegistration(&plugin.PluginRegistration{Dissemination: []rpc.DisseminationDecl{disseminationTestDeclaration()}})
	manager := process.NewProcessManager(nil)
	manager.AddProcess(name, proc)
	s.procManager.Store(manager)
	require.NoError(t, s.bindConfigureConfig([]*process.Process{proc}, configureConfig{tree: map[string]any{}}))
	defer s.clearConfigureConfig([]*process.Process{proc})
	require.ErrorContains(t, s.validateDisseminationForProcess(proc), "configuration resolver")
}
