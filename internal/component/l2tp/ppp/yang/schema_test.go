// Design: docs/research/l2tpv2-ze-integration.md -- one shared default authority

package yang_test

import (
	"testing"
	"time"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/l2tp/pppconfig"
	_ "github.com/ze-software/ze/internal/component/l2tp/pppoe/yang"
	_ "github.com/ze-software/ze/internal/component/l2tp/yang"
)

// TestPPPLCPGroupingDefaultsResolveInBothModules resolves the actual imports
// and compares every runtime setting with the default of each consumer leaf.
// There are no Go bootstrap constants: runtime defaults come from the grouping.
func TestPPPLCPGroupingDefaultsResolveInBothModules(t *testing.T) {
	loader := configyang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadRegistered(); err != nil {
		t.Fatal(err)
	}
	resolved, err := loader.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"restart-timer": "3000", "max-terminate": "2", "max-configure": "10", "max-failure": "5"}
	for _, transport := range []string{"l2tp", "pppoe"} {
		t.Run(transport, func(t *testing.T) {
			module := resolved.GetEntry("ze-" + transport + "-conf")
			if module == nil {
				t.Fatal("missing transport module")
			}
			root := module.Dir[transport]
			if root == nil {
				t.Fatal("missing transport container")
			}
			lcp := root.Dir["lcp"]
			if lcp == nil {
				t.Fatal("shared lcp grouping did not resolve")
			}
			for name, value := range want {
				leaf := lcp.Dir[name]
				if leaf == nil {
					t.Fatalf("missing leaf %s", name)
				}
				if len(leaf.Default) != 1 {
					t.Fatalf("%s default = %v", name, leaf.Default)
				}
				if leaf.Default[0] != value {
					t.Fatalf("%s default = %s, want %s", name, leaf.Default[0], value)
				}
			}
			c, err := pppconfig.ParseLCP(transport, nil)
			if err != nil {
				t.Fatal(err)
			}
			if c.RestartTimer() != 3*time.Second {
				t.Fatalf("runtime timer = %s", c.RestartTimer())
			}
			if c.MaxTerminate() != 2 {
				t.Fatalf("runtime max-terminate = %d", c.MaxTerminate())
			}
			if c.MaxConfigure() != 10 {
				t.Fatalf("runtime max-configure = %d", c.MaxConfigure())
			}
			if c.MaxFailure() != 5 {
				t.Fatalf("runtime max-failure = %d", c.MaxFailure())
			}
		})
	}
}

// TestParseLCPRefusesMalformedValues covers direct provider-map callers as well
// as values the YANG parser would refuse, including overflow before conversion.
func TestParseLCPRefusesMalformedValues(t *testing.T) {
	for _, raw := range []any{
		"500", map[string]any{"restart-timer": "50"}, map[string]any{"restart-timer": "60001"},
		map[string]any{"restart-timer": "9223372036854775807"}, map[string]any{"max-configure": "0"},
		map[string]any{"max-terminate": "256"}, map[string]any{"max-failure": "-1"},
		map[string]any{"max-failure": 5}, map[string]any{"restart-timer": "invalid"},
	} {
		if _, err := pppconfig.ParseLCP("l2tp", raw); err == nil {
			t.Fatalf("accepted malformed value %#v", raw)
		}
	}
}
