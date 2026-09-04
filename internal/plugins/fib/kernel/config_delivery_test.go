// Detail: register.go -- parseFIBConfig

package fibkernel

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// deliverSection builds the config section this plugin actually receives, by
// driving the real producer chain rather than typing the JSON:
//
//	LoadConfig            (internal/component/config)
//	  -> (*Tree).ToMap
//	  -> ExtractConfigSubtree (internal/component/config/plugin_verify.go)
//	  -> json.Marshal         (plugin server reload.go)
//	  -> parseFIBConfig       (register.go)
//
// The point is the SHAPE. Two defects lived here at once, and a hand-typed
// fixture agreed with the reader on both: every leaf arrives as a STRING, and
// the section arrives WRAPPED in its full root path. A fixture the author
// writes states what the author believes; this one states what the daemon
// sends.
//
// The parse error is RETURNED rather than fataled here, so each test asserts it
// itself. A helper that swallows the error takes that assertion out of every
// caller, and a reader of one test can no longer see that it checks one.
func deliverSection(t *testing.T, text string) (fibConfig, error) {
	t.Helper()
	result, err := zeconfig.LoadConfig(text, "test.conf", nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	subtree := zeconfig.ExtractConfigSubtree(result.Tree.ToMap(), configRoot)
	if subtree == nil {
		t.Fatalf("ExtractConfigSubtree(%q) returned nil, so the plugin would be handed {}", configRoot)
	}
	data, err := json.Marshal(subtree)
	if err != nil {
		t.Fatalf("marshal subtree: %v", err)
	}
	t.Logf("delivered section for %s: %s", configRoot, data)
	return parseFIBConfig([]sdk.ConfigSection{{Root: configRoot, Data: string(data)}})
}

const fibBothSet = `fib {
	kernel {
		flush-on-stop true
		sweep-delay 45
	}
}
`

const fibSweepZero = `fib {
	kernel {
		sweep-delay 0
	}
}
`

const fibNeitherSet = `fib {
	kernel {
	}
}
`

// TestParseFIBConfigReadsTheDeliveredStrings is the whole point of this file.
// Both settings were unreachable for two independent reasons at once, and both
// unit tests that covered them passed. The assertions here are the operator's
// values, read back through the producer chain.
func TestParseFIBConfigReadsTheDeliveredStrings(t *testing.T) {
	cfg, err := deliverSection(t, fibBothSet)
	if err != nil {
		t.Fatalf("parseFIBConfig: %v", err)
	}

	if !cfg.FlushOnStop {
		t.Error("flush-on-stop = false, want true: the operator's setting was discarded")
	}
	if cfg.SweepDelay != 45*time.Second {
		t.Errorf("sweep-delay = %v, want 45s: the operator's setting was discarded", cfg.SweepDelay)
	}
}

// TestParseFIBConfigReadsAConfiguredZero checks the value a "> 0" guard used to
// discard. ze-fib-conf.yang declares sweep-delay as a uint16 with no range, so
// `sweep-delay 0` commits and asks for the sweep to run at once.
func TestParseFIBConfigReadsAConfiguredZero(t *testing.T) {
	cfg, err := deliverSection(t, fibSweepZero)
	if err != nil {
		t.Fatalf("parseFIBConfig: %v", err)
	}

	if cfg.SweepDelay != 0 {
		t.Errorf("sweep-delay = %v, want 0: a configured zero is a value, not an absence", cfg.SweepDelay)
	}
}

// TestParseFIBConfigKeepsItsDefaultsWhenTheLeavesAreAbsent checks the other
// half: an absent leaf leaves the default standing rather than writing a zero
// over it.
func TestParseFIBConfigKeepsItsDefaultsWhenTheLeavesAreAbsent(t *testing.T) {
	cfg, err := deliverSection(t, fibNeitherSet)
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

// TestParseFIBConfigAcceptsTheRemovalPayload checks the shape a DELETED root
// takes. Every producer spells it `{}`, and a guard that read that as a
// malformed section refused the commit, so an operator could not remove a
// `fib { kernel { } }` block they had already committed.
func TestParseFIBConfigAcceptsTheRemovalPayload(t *testing.T) {
	cfg, err := parseFIBConfig([]sdk.ConfigSection{{Root: configRoot, Data: `{}`}})
	if err != nil {
		t.Fatalf("parseFIBConfig refused the removal payload: %v", err)
	}
	if cfg.FlushOnStop {
		t.Error("flush-on-stop = true, want the default false after removal")
	}
	if cfg.SweepDelay != sweepDelay {
		t.Errorf("sweep-delay = %v, want the default %v after removal", cfg.SweepDelay, sweepDelay)
	}
}

// TestParseFIBConfigRefusesAValueItCannotRead checks the case an absent leaf
// and a malformed one used to share. configvalue answers false for both, so a
// reader that only asked "did I get a value" kept its default for a value the
// operator wrote and never said so.
//
// The payload is typed here rather than delivered, because the loader refuses
// these values before they reach the plugin: the YANG types are boolean and
// uint16. This pins the plugin's own refusal for a section that reaches it by
// another route.
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
