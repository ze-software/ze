// Design: docs/research/l2tpv2-ze-integration.md -- shared schema defaults at the transport boundary
// Related: lcp.go -- loadLCPDefaults reads the module set DefaultLoader answers

package pppconfig

import (
	"errors"
	"testing"
	"time"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
)

// lcpProbeModule is a module the test declares for an external plugin and then
// registers as a compiled-in module too, under the same name.
const lcpProbeModule = `module ze-lcp-probe-conf {
    namespace "urn:ze:test:lcp-probe";
    prefix lcpprobe;
    container lcp-probe {
        leaf interval { type uint8; }
    }
}`

// TestLCPDefaultsAreTheSharedGroupingDefaults: the defaults an absent lcp block
// takes are the four the ze-ppp-lcp grouping declares, read from the process's
// module set. Method: read them with the set untouched, then parse an absent
// block through the transport parser.
func TestLCPDefaultsAreTheSharedGroupingDefaults(t *testing.T) {
	defaults, err := loadLCPDefaults()
	if err != nil {
		t.Fatalf("loadLCPDefaults: %v", err)
	}
	if want := [4]int{3000, 2, 10, 5}; defaults != want {
		t.Fatalf("defaults = %v, want %v", defaults, want)
	}
	c, err := ParseLCP("l2tp", nil)
	if err != nil {
		t.Fatalf("ParseLCP of an absent block: %v", err)
	}
	if c.RestartTimer() != 3000*time.Millisecond {
		t.Errorf("restart timer = %s, want 3s", c.RestartTimer())
	}
	if c.MaxTerminate() != 2 {
		t.Errorf("max-terminate = %d, want 2", c.MaxTerminate())
	}
	if c.MaxConfigure() != 10 {
		t.Errorf("max-configure = %d, want 10", c.MaxConfigure())
	}
	if c.MaxFailure() != 5 {
		t.Errorf("max-failure = %d, want 5", c.MaxFailure())
	}
}

// TestLCPDefaultsReadTheProcessModuleSet: the defaults come from the set
// yang.DefaultLoader already resolved, never from a loader of the function's
// own, and a refusal of that set reaches the operator as an error rather than
// as zero defaults.
//
// Method: make the process's set and a fresh resolve of the registry disagree.
// The test publishes an external module, then registers a compiled-in module of
// the same name. The registry alone still resolves, while DefaultLoader answers
// the join's refusal of the name clash. A reader of the process's set answers
// that refusal; a loader of its own answers the four defaults.
func TestLCPDefaultsReadTheProcessModuleSet(t *testing.T) {
	t.Cleanup(configyang.ResetPublishedModulesForTest())
	probe, err := configyang.ExternalModuleFromText("lcp-probe", lcpProbeModule)
	if err != nil {
		t.Fatalf("probe module: %v", err)
	}
	if err := configyang.PublishExternalModules([]configyang.ExternalModule{probe}, configyang.NewValidatorRegistry()); err != nil {
		t.Fatalf("publish the probe module: %v", err)
	}
	t.Cleanup(configyang.RegisterModuleForTest("ze-lcp-probe-conf.yang", lcpProbeModule))
	_, refusal := configyang.DefaultLoader()
	if refusal == nil {
		t.Fatal("precondition: DefaultLoader answered a set holding a compiled-in and an external module of one name")
	}

	defaults, err := loadLCPDefaults()
	if err == nil {
		t.Fatalf("loadLCPDefaults answered %v from a module set of its own; the process's set refuses: %v", defaults, refusal)
	}
	if !errors.Is(err, refusal) {
		t.Fatalf("loadLCPDefaults refused with %v, want the process set's refusal %v", err, refusal)
	}

	_, err = ParseLCP("pppoe", nil)
	if _, ok := errors.AsType[*opererr.Error](err); !ok {
		t.Fatalf("ParseLCP over a refused module set = %v, want opererr.Error", err)
	}
	if !errors.Is(err, refusal) {
		t.Fatalf("ParseLCP refusal %v does not carry the process set's refusal %v", err, refusal)
	}
}
