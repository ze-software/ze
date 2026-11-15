package filter_irr

import (
	"os"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// TestMain registers the address families, which a test binary of this package
// does not link: judgedFamily looks a field's family up by name to learn
// whether the list judges its NLRI, and judges nothing of a name it cannot find.
func TestMain(m *testing.M) {
	family.RegisterTestFamilies()
	os.Exit(m.Run())
}

// splitIRRConfig makes a test config written as one bgp map the delivery the
// filter reads: the bgp root is the only root it reads.
func splitIRRConfig(cfg map[string]any) irrDelivery {
	return irrDelivery{bgp: cfg}
}

// parseIRRConfigOf parses a test config written as one bgp map.
func parseIRRConfigOf(cfg map[string]any) *irrConfig {
	return parseIRRConfig(splitIRRConfig(cfg).bgp)
}

// configureOf installs a test config written as one bgp map.
func (plug *irrPlugin) configureOf(cfg map[string]any) {
	plug.handleConfigure(splitIRRConfig(cfg))
}
