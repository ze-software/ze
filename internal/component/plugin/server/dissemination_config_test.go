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
// filter sets prove neither transport decoding nor an old graph decides startup.
//
// VALIDATES: Stage 1 dissemination validation compiles against candidate scope.
// PREVENTS: Stage 1 judging a new external program against an active old graph.
func TestDisseminationStage1UsesCandidateScope(t *testing.T) {
	for _, complete := range []bool{true, false} {
		t.Run(map[bool]string{true: "new complete grants", false: "new incomplete grants"}[complete], func(t *testing.T) {
			const name = "external-candidate-table"
			require.Nil(t, registry.Lookup(name))
			peers := []DeliveryPeer{
				{Addr: "192.0.2.1", Name: "p1", Path: "/" + disseminationTestNamespace + "/entry/p1"},
				{Addr: "192.0.2.2", Name: "p2", Path: "/" + disseminationTestNamespace + "/entry/p2"},
			}
			treeFor := func(valid bool) map[string]any {
				m := map[string]any{"complete": valid}
				if !valid {
					m["bus"] = map[string]any{
						"endpoint": map[string]any{
							name: map[string]any{
								"receive": map[string]any{
									"allow": []any{"none"},
								},
							},
						},
					}
				}
				return m
			}
			candidateDisseminationResolver(t, func(tree map[string]any) ([]DeliveryPeer, error) {
				_, known := tree["complete"].(bool)
				require.True(t, known, "resolver must read the selected candidate")
				return peers, nil
			})
			s, _ := newLifecycleStartupServer(t)
			configuration := plugin.PluginConfig{Name: name, Run: "external-program"}
			s.config.Plugins = []plugin.PluginConfig{configuration}
			proc := process.NewProcess(configuration)
			manager := process.NewProcessManager(nil)
			require.NoError(t, manager.AddProcess(name, proc))
			s.procManager.Store(manager)
			active, err := BuildDeliveryGraph(disseminationTestNamespace, peers, plugin.BusConfig{}, []BusParticipantInput{
				disseminationEndpointInput(name, plugin.BusBlock{}),
			})
			require.NoError(t, err)
			s.deliveryGraph.Store(active)

			tree := treeFor(complete)
			require.NoError(t, s.bindConfigureConfig([]*process.Process{proc}, configureConfig{tree: tree}))
			defer s.clearConfigureConfig([]*process.Process{proc})
			// The scope owns its cloned candidate, not a mutable map reference.
			tree["complete"] = !complete

			input := &rpc.DeclareRegistrationInput{
				ProtocolVersion: rpc.ProtocolVersion,
				Dissemination:   []rpc.DisseminationDecl{disseminationTestDeclaration()},
				Sends:           disseminationTestSends(),
				Receives:        disseminationTestReceives(),
			}
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
//
// VALIDATES: missing namespace resolver fails closed during Stage 1 validation.
// PREVENTS: silently accepting an unverified external dissemination declaration.
func TestDisseminationCandidateNeedsNamespaceResolver(t *testing.T) {
	candidateDisseminationResolver(t, nil)
	const name = "external-no-resolver"
	s, _ := newLifecycleStartupServer(t)
	configuration := plugin.PluginConfig{Name: name, Run: "external-program"}
	s.config.Plugins = []plugin.PluginConfig{configuration}
	proc := process.NewProcess(configuration)
	proc.SetRegistration(&plugin.PluginRegistration{
		Dissemination: []rpc.DisseminationDecl{disseminationTestDeclaration()},
		Sends:         disseminationTestSends(),
		Receives:      disseminationTestReceives(),
	})
	manager := process.NewProcessManager(nil)
	require.NoError(t, manager.AddProcess(name, proc))
	s.procManager.Store(manager)
	require.NoError(t, s.bindConfigureConfig([]*process.Process{proc}, configureConfig{tree: map[string]any{}}))
	defer s.clearConfigureConfig([]*process.Process{proc})
	require.ErrorContains(t, s.validateDisseminationForProcess(proc), "configuration resolver")
}
