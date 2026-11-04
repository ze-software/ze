package server

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestCapabilityTransactionVerifierPopulation drives the real coordinator with
// an explicit participant order (the process manager's snapshots are unordered).
// Both transfer orders must wait for the last nil declaration, without waiting
// for a queued operation-only participant that receives no section verify.
func TestCapabilityTransactionVerifierPopulation(t *testing.T) {
	for _, removalFirst := range []bool{false, true} {
		name := "new-owner-first"
		if removalFirst {
			name = "old-owner-first"
		}
		t.Run(name, func(t *testing.T) {
			plugins := []pluginDef{
				{name: "newowner", roots: []string{"bgp"}, verifyResp: &rpc.ConfigVerifyOutput{
					Status: rpc.StatusOK, Capabilities: roleDeclaration("03"),
				}},
				{name: "oldowner", roots: []string{"bgp"}, verifyResp: &rpc.ConfigVerifyOutput{
					Status: rpc.StatusOK, Capabilities: &rpc.DeclareCapabilitiesInput{},
				}},
				{name: "observer", roots: []string{"bgp"}},
				{name: "operation-only", roots: []string{"unrelated"}, configOps: []rpc.ConfigOperationDecl{{
					Root: "unrelated", Decompose: true, Operations: []rpc.ConfigOperationType{testOpSetProperty},
				}}},
			}
			if removalFirst {
				plugins[0], plugins[1] = plugins[1], plugins[0]
			}
			oldTree := map[string]any{"bgp": map[string]any{"router-id": "1.2.3.4"}}
			newTree := map[string]any{"bgp": map[string]any{"router-id": "5.6.7.8"}}
			s := newTestReloadServer(t, &mockReloadReactor{tree: oldTree}, plugins)
			s.capInjector = plugin.NewCapabilityInjector()
			seedCapabilities(t, s, "oldowner", roleDeclaration("00"))
			diff := config.DiffMaps(oldTree, newTree)
			affected := make([]affectedPlugin, 0, len(plugins))
			for _, pd := range plugins {
				proc := s.procManager.Load().GetProcess(pd.name)
				sections, err := reloadConfigSections(newTree, diff, proc.Registration())
				if err != nil {
					t.Fatal(err)
				}
				affected = append(affected, affectedPlugin{proc: proc, sections: sections})
			}
			if err := s.runTxCoordinator(context.Background(), affected, diff, oldTree, newTree); err != nil {
				t.Fatalf("complete transfer with inactive participant refused: %v", err)
			}
			got := s.GetPluginCapabilitiesForSelectors(capabilityReloadPeer)
			if len(got) != 1 || got[0].Plugin != "newowner" || len(got[0].Value) != 1 || got[0].Value[0] != 3 {
				t.Fatalf("complete verified candidate not installed: %+v", got)
			}
			for _, pd := range plugins {
				want := 1
				if pd.name == "operation-only" {
					want = 0
				}
				if calls := pd.responder.getVerifyCalls(); calls != want {
					t.Fatalf("%s verify calls = %d, want %d", pd.name, calls, want)
				}
				if calls := pd.responder.getApplyCalls(); calls != want {
					t.Fatalf("%s apply calls = %d, want %d", pd.name, calls, want)
				}
			}
		})
	}
}
