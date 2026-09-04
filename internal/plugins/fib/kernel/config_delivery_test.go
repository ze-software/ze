// Detail: register.go -- parseFIBConfig

package fibkernel

import (
	"testing"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestParseFIBConfigReadsTheDeliveredStrings checks the config shape a plugin
// actually receives. Tree.values is a map[string]string and toMap copies it
// through unchanged, so every leaf arrives as the text the operator wrote. The
// assertions this replaces read .(bool) and .(float64), which the delivered
// map never satisfies, so both settings were discarded in silence and the
// defaults stood whatever the config said.
func TestParseFIBConfigReadsTheDeliveredStrings(t *testing.T) {
	cfg, err := parseFIBConfig([]sdk.ConfigSection{{
		Root: configRoot,
		Data: `{"flush-on-stop":"true","sweep-delay":"30"}`,
	}})
	if err != nil {
		t.Fatalf("parseFIBConfig: %v", err)
	}
	if !cfg.FlushOnStop {
		t.Error("flush-on-stop = false, want true: the operator's setting was discarded")
	}
	if cfg.SweepDelay != 30*time.Second {
		t.Errorf("sweep-delay = %v, want 30s: the operator's setting was discarded", cfg.SweepDelay)
	}
}

// TestParseFIBConfigKeepsItsDefaultsWhenTheLeavesAreAbsent checks the other
// half: an absent leaf must leave the default standing rather than write a
// zero over it.
func TestParseFIBConfigKeepsItsDefaultsWhenTheLeavesAreAbsent(t *testing.T) {
	cfg, err := parseFIBConfig([]sdk.ConfigSection{{Root: configRoot, Data: `{}`}})
	if err != nil {
		t.Fatalf("parseFIBConfig: %v", err)
	}
	if cfg.FlushOnStop {
		t.Error("flush-on-stop = true, want the default false")
	}
	if cfg.SweepDelay != sweepDelay {
		t.Errorf("sweep-delay = %v, want the default %v", cfg.SweepDelay, sweepDelay)
	}
}
