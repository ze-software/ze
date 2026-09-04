// Detail: register.go -- parseFIBConfig

package fibkernel

import (
	"strings"
	"testing"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestParseFIBConfigReadsTheDeliveredStrings checks the config shape a plugin
// actually receives, wrapper included. ExtractConfigSubtree wraps a section in
// its full root path, so a hand-typed unwrapped fixture tests a payload the
// daemon never sends -- which is how both settings stayed unreachable while a
// unit test called them covered. Tree.values is a map[string]string and toMap copies it
// through unchanged, so every leaf arrives as the text the operator wrote. The
// assertions this replaces read .(bool) and .(float64), which the delivered
// map never satisfies, so both settings were discarded in silence and the
// defaults stood whatever the config said.
func TestParseFIBConfigReadsTheDeliveredStrings(t *testing.T) {
	cfg, err := parseFIBConfig([]sdk.ConfigSection{{
		Root: configRoot,
		Data: `{"fib":{"kernel":{"flush-on-stop":"true","sweep-delay":"30"}}}`,
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
	cfg, err := parseFIBConfig([]sdk.ConfigSection{{Root: configRoot, Data: `{"fib":{"kernel":{}}}`}})
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

// TestParseFIBConfigReadsAConfiguredZero checks the value that a "> 0" guard
// discards. ze-fib-conf.yang declares sweep-delay as a uint16 with no range, so
// `sweep-delay 0` commits, and it asks for the sweep to run at once. A guard
// that treated 0 as an absence would keep the 30-second default over it and
// log nothing, which is the operator's setting discarded in silence.
func TestParseFIBConfigReadsAConfiguredZero(t *testing.T) {
	cfg, err := parseFIBConfig([]sdk.ConfigSection{{
		Root: configRoot,
		Data: `{"fib":{"kernel":{"flush-on-stop":"false","sweep-delay":"0"}}}`,
	}})
	if err != nil {
		t.Fatalf("parseFIBConfig: %v", err)
	}
	if cfg.SweepDelay != 0 {
		t.Errorf("sweep-delay = %v, want 0: a configured zero is a value, not an absence", cfg.SweepDelay)
	}
}

// TestParseFIBConfigRefusesAValueItCannotRead checks the case an absent leaf
// and a malformed one used to share. configvalue answers false for both, so a
// reader that only asked "did I get a value" kept its default for a value the
// operator wrote and never said so. The map lookup separates them, and a
// malformed value is refused by name.
func TestParseFIBConfigRefusesAValueItCannotRead(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
		want string
	}{
		{name: "a boolean that is not one", data: `{"fib":{"kernel":{"flush-on-stop":"yes please"}}}`, want: "flush-on-stop"},
		{name: "a delay that is not a number", data: `{"fib":{"kernel":{"sweep-delay":"soon"}}}`, want: "sweep-delay"},
		{name: "a fractional delay", data: `{"fib":{"kernel":{"sweep-delay":"1.5"}}}`, want: "sweep-delay"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseFIBConfig([]sdk.ConfigSection{{Root: configRoot, Data: tc.data}})
			if err == nil {
				t.Fatal("parseFIBConfig accepted a value it cannot read")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to name %q", err, tc.want)
			}
		})
	}
}
