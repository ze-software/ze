package filter_irr

import (
	"os"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestMain registers the address families, which a test binary of this package
// does not link: rewritable looks an mp block's family up by name to learn
// whether the engine can rewrite it, and fails closed on a name it cannot find.
func TestMain(m *testing.M) {
	family.RegisterTestFamilies()
	os.Exit(m.Run())
}

// splitIRRConfig splits a test config written as one bgp map into the two
// roots the filter reads: its "policy" entry is the policy root, and the rest
// is the bgp root.
func splitIRRConfig(cfg map[string]any) irrDelivery {
	policyCfg, _ := cfg["policy"].(map[string]any)
	bgpCfg := make(map[string]any, len(cfg))
	for k, v := range cfg {
		if k == "policy" {
			continue
		}
		bgpCfg[k] = v
	}
	return irrDelivery{policy: policyCfg, bgp: bgpCfg}
}

// parseIRRConfigOf parses a test config written as one bgp map.
func parseIRRConfigOf(cfg map[string]any) *irrConfig {
	split := splitIRRConfig(cfg)
	return parseIRRConfig(split.policy, split.bgp)
}

// configureOf installs a test config written as one bgp map.
func (plug *irrPlugin) configureOf(cfg map[string]any) {
	plug.handleConfigure(splitIRRConfig(cfg))
}
