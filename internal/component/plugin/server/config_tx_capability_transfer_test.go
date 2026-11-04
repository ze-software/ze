package server

import (
	"context"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestCapabilityTransactionTransfer checks the whole candidate through real reload
// RPCs: either verify order can transfer ownership, and rollback restores the
// old owner in both the OPEN reader and the retained process declarations.
func TestCapabilityTransactionTransfer(t *testing.T) {
	for _, removalFirst := range []bool{false, true} {
		name := "new-owner-verifies-first"
		if removalFirst {
			name = "old-owner-verifies-first"
		}
		for _, rollback := range []bool{false, true} {
			outcome := "commit"
			if rollback {
				outcome = "rollback"
			}
			t.Run(name+"/"+outcome, func(t *testing.T) {
				plugins := []pluginDef{
					{name: "newowner", roots: []string{"bgp"}, verifyResp: &rpc.ConfigVerifyOutput{
						Status: rpc.StatusOK, Capabilities: roleDeclaration("03"),
					}},
					{name: "oldowner", roots: []string{"bgp"}, verifyResp: &rpc.ConfigVerifyOutput{
						Status: rpc.StatusOK, Capabilities: &rpc.DeclareCapabilitiesInput{},
					}},
				}
				if removalFirst {
					plugins[0], plugins[1] = plugins[1], plugins[0]
				}
				if rollback {
					plugins = append(plugins, pluginDef{name: "failing", roots: []string{"bgp"},
						applyResp: &rpc.ConfigApplyOutput{Status: rpc.StatusError, Error: "transfer apply refused"}})
				}
				s := newTestReloadServer(t, &mockReloadReactor{tree: map[string]any{
					"bgp": map[string]any{"router-id": "1.2.3.4"},
				}}, plugins)
				s.capInjector = plugin.NewCapabilityInjector()
				seedCapabilities(t, s, "oldowner", roleDeclaration("00"))

				err := s.ReloadConfig(context.Background(), map[string]any{
					"bgp": map[string]any{"router-id": "5.6.7.8"},
				})
				wantOwner, wantPayload := "newowner", "03"
				if rollback {
					if err == nil || !strings.Contains(err.Error(), "transfer apply refused") {
						t.Fatalf("rollback did not reach the failing apply: %v", err)
					}
					wantOwner, wantPayload = "oldowner", "00"
				} else if err != nil {
					t.Fatalf("candidate ownership transfer refused: %v", err)
				}
				got := s.GetPluginCapabilitiesForSelectors(capabilityReloadPeer)
				if len(got) != 1 || got[0].Plugin != wantOwner || len(got[0].Value) != 1 {
					t.Fatalf("OPEN capability snapshot = %+v, want one capability from %s", got, wantOwner)
				}
				wantValue := byte(3)
				if rollback {
					wantValue = 0
				}
				if got[0].Value[0] != wantValue {
					t.Fatalf("OPEN payload = %x, want %02x", got[0].Value, wantValue)
				}
				for _, owner := range []string{"oldowner", "newowner"} {
					recorded := s.procManager.Load().GetProcess(owner).Capabilities()
					if owner == wantOwner {
						if recorded == nil || len(recorded.Capabilities) != 1 || recorded.Capabilities[0].Payload != wantPayload {
							t.Fatalf("%s process declaration = %+v, want payload %s", owner, recorded, wantPayload)
						}
						continue
					}
					if recorded != nil && len(recorded.Capabilities) != 0 {
						t.Fatalf("departing owner %s retained %+v", owner, recorded)
					}
				}
			})
		}
	}
}

// TestCapabilityTransactionConflictBeforeApply checks duplicate staged owners
// and an unchanged owner at the verify barrier, with no candidate publication
// or participant apply allowed when the complete candidate conflicts.
func TestCapabilityTransactionConflictBeforeApply(t *testing.T) {
	for _, stagedHolder := range []bool{false, true} {
		name := "unchanged-owner"
		if stagedHolder {
			name = "duplicate-candidate"
		}
		t.Run(name, func(t *testing.T) {
			plugins := []pluginDef{
				{name: "newowner", roots: []string{"bgp"}, verifyResp: &rpc.ConfigVerifyOutput{
					Status: rpc.StatusOK, Capabilities: roleDeclaration("03"),
				}},
				{name: "holder", roots: []string{"bgp"}},
				{name: "observer", roots: []string{"bgp"}},
				{name: "operation-only", roots: []string{"unrelated"}, configOps: []rpc.ConfigOperationDecl{{
					Root: "unrelated", Decompose: true, Operations: []rpc.ConfigOperationType{testOpSetProperty},
				}}},
			}
			if stagedHolder {
				plugins[1].verifyResp = &rpc.ConfigVerifyOutput{Status: rpc.StatusOK, Capabilities: roleDeclaration("01")}
			}
			s := newTestReloadServer(t, &mockReloadReactor{tree: map[string]any{
				"bgp": map[string]any{"router-id": "1.2.3.4"},
			}}, plugins)
			s.capInjector = plugin.NewCapabilityInjector()
			seedCapabilities(t, s, "holder", roleDeclaration("00"))
			err := s.ReloadConfig(context.Background(), map[string]any{
				"bgp": map[string]any{"router-id": "5.6.7.8"},
			})
			if err == nil || !strings.Contains(err.Error(), "verify failed") || !strings.Contains(err.Error(), "capability conflict") {
				t.Fatalf("conflicting candidate was not refused during verify: %v", err)
			}
			for _, pd := range plugins {
				if calls := pd.responder.getApplyCalls(); calls != 0 {
					t.Fatalf("%s applied %d times after conflicting verify", pd.name, calls)
				}
			}
			got := s.GetPluginCapabilitiesForSelectors(capabilityReloadPeer)
			if len(got) != 1 || got[0].Plugin != "holder" || len(got[0].Value) != 1 || got[0].Value[0] != 0 {
				t.Fatalf("failed transaction changed complete running snapshot: %+v", got)
			}
		})
	}
}
