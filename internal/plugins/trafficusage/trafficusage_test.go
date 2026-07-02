// VALIDATES: package identity (plugin Name / config root) and logger holder defaults.
// PREVENTS: drift between the registered plugin name, the YANG config root, and
// the metric/namespace prefix; a nil package logger panicking before configure.

package trafficusage

import (
	"log/slog"
	"testing"
)

func TestNameAndConfigRoot(t *testing.T) {
	if Name != "traffic-usage" {
		t.Errorf("Name = %q, want traffic-usage", Name)
	}
	if configRoot != "traffic/usage" {
		t.Errorf("configRoot = %q, want traffic/usage", configRoot)
	}
}

func TestLoggerDefaultNotNil(t *testing.T) {
	if logger() == nil {
		t.Fatal("logger() = nil; must be primed to a discard logger in init")
	}
	// setLogger(nil) must not clobber the primed logger.
	setLogger(nil)
	if logger() == nil {
		t.Fatal("logger() = nil after setLogger(nil); must keep previous logger")
	}
	l := slog.New(slog.DiscardHandler)
	setLogger(l)
	if logger() != l {
		t.Error("setLogger did not install the provided logger")
	}
}
