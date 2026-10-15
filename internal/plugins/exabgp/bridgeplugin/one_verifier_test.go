// Design: docs/architecture/config/transaction-protocol.md -- one verifier per plugin

package bridgeplugin

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestOneVerifierPerPlugin proves that the static verifier the registry holds
// and the reload's config verify give one verdict on a candidate the running
// bridge could take, so a validate and a reload cannot disagree on the parse.
// The reload verify also refuses a running change, which needs runtime state
// the static verifier has not (TestBridgeVerifyRefusesRunningChange). Method:
// run the registry's InProcessConfigVerifier and verifyReload of a bridge
// running the accepted candidate over the same candidates; both accept, or
// both refuse with the same text.
func TestOneVerifierPerPlugin(t *testing.T) {
	reg := registry.Lookup("exabgp-bridge")
	if reg == nil {
		t.Fatal("exabgp-bridge is not registered")
	}
	if reg.InProcessConfigVerifier == nil {
		t.Fatal("exabgp-bridge registers no InProcessConfigVerifier")
	}
	running := bridgeSections("main=./a.py", `"ipv4/unicast"`, "true")
	r := runningBridge(t, running)
	cases := []struct {
		name     string
		sections []sdk.ConfigSection
		refused  bool
	}{
		{"accepted", running, false},
		{"malformed", []sdk.ConfigSection{{Root: configRoot, Data: `{`}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			static := reg.InProcessConfigVerifier(tc.sections)
			reload := r.verifyReload(tc.sections)
			if (static != nil) != tc.refused {
				t.Fatalf("static verifier answered %v, want refused=%v", static, tc.refused)
			}
			if (reload != nil) != tc.refused {
				t.Fatalf("reload verify answered %v, want refused=%v", reload, tc.refused)
			}
			if tc.refused && static.Error() != reload.Error() {
				t.Fatalf("verdicts differ: static %q, reload %q", static, reload)
			}
		})
	}
}
