// Design: docs/architecture/config/transaction-protocol.md -- one verifier per plugin

package geodns

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestOneVerifierPerPlugin proves that the static verifier the registry holds
// and the reload's config verify give one verdict, so a validate and a reload
// cannot disagree. Method: run the registry's InProcessConfigVerifier and the
// plugin's sdk.ConfigTx Verify over the same candidates; both accept, or both
// refuse with the same text.
func TestOneVerifierPerPlugin(t *testing.T) {
	reg := registry.Lookup("geodns")
	if reg == nil {
		t.Fatal("geodns is not registered")
	}
	if reg.InProcessConfigVerifier == nil {
		t.Fatal("geodns registers no InProcessConfigVerifier")
	}
	tx := newTestGeoDNSConfigTx(t)
	cases := []struct {
		name     string
		sections []sdk.ConfigSection
		refused  bool
	}{
		{"accepted", geodnsSections("a.example.", "x"), false},
		{"malformed", []sdk.ConfigSection{{Root: configRootService, Data: `{`}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			static := reg.InProcessConfigVerifier(tc.sections)
			reload := tx.Verify(tc.sections)
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
