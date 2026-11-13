package isis

import (
	"fmt"
	"net/netip"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	isisredistribute "github.com/ze-software/ze/internal/plugins/isis/redistribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestExportChainSlotRestoresOnRollback keeps the original pointer-identity
// assertions through the real SDK owner, which now uses ConfigTx rather than a
// separate export-chain journal. Each installed chain must also make the
// expected accepting or rejecting decision for a real route.
func TestExportChainSlotRestoresOnRollback(t *testing.T) {
	const (
		cfgA = `{"isis":{"net":"49.0001.0000.0000.0001.00","filter":{"export":{"final":"accept"}}}}`
		cfgB = `{"isis":{"net":"49.0001.0000.0000.0001.00","filter":{"export":{"final":"reject"}}}}`
	)
	for _, bridged := range []bool{false, true} {
		t.Run(fmt.Sprintf("bridge=%t", bridged), func(t *testing.T) {
			_, call := startISISRuntimeRollbackPlugin(t, bridged, cfgA)
			entry := configredist.RouteEntry{}
			entry.Route.Prefix = netip.MustParsePrefix("192.0.2.1/32")
			assertChain := func(stage string, want ribpolicy.Chain, accept bool) {
				t.Helper()
				got := isisredistribute.ExportChain()
				if got == nil || got != want {
					t.Fatalf("%s: installed chain = %p, want exact chain %p", stage, got, want)
				}
				wantAction := ribpolicy.ActionReject
				if accept {
					wantAction = ribpolicy.ActionAccept
				}
				if action := got.Evaluate(entry.Route).Action; action != wantAction {
					t.Errorf("%s: chain verdict = %v, want %v", stage, action, wantAction)
				}
			}
			verifyApply := func(data string) {
				t.Helper()
				call("ze-plugin-callback:config-verify", rpc.ConfigVerifyInput{Sections: []rpc.ConfigSection{{Root: configRoot, Data: data}}})
				call("ze-plugin-callback:config-apply", rpc.ConfigApplyInput{Sections: []rpc.ConfigDiffSection{{Root: configRoot, Changed: data}}})
			}
			running := isisredistribute.ExportChain()
			assertChain("startup", running, true)
			verifyApply(cfgB)
			next := isisredistribute.ExportChain()
			if next == running {
				t.Fatal("candidate apply retained the old chain")
			}
			assertChain("candidate applied", next, false)
			call("ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "candidate"})
			assertChain("rollback", running, true)
			call("ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "duplicate"})
			assertChain("duplicate rollback", running, true)

			// Repeating the config and delivering no owned root must keep the
			// prior chain after rollback.
			verifyApply(cfgA)
			call("ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "equal"})
			assertChain("equal candidate rollback", running, true)
			call("ze-plugin-callback:config-verify", rpc.ConfigVerifyInput{})
			call("ze-plugin-callback:config-apply", rpc.ConfigApplyInput{})
			call("ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "absent"})
			assertChain("absent candidate rollback", running, true)

			verifyApply(cfgB)
			next = isisredistribute.ExportChain()
			assertChain("candidate reapplied", next, false)
			call("ze-plugin-callback:config-committed", map[string]string{"transaction-id": "accepted"})
			call("ze-plugin-callback:config-rollback", map[string]string{"transaction-id": "foreign"})
			assertChain("committed candidate rollback", next, false)
		})
	}
}
