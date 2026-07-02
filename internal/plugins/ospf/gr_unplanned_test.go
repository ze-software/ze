// VALIDATES: the RFC 3623 sec 5 unplanned-outage path -- disabled by default so a cold boot
// originates no Grace-LSA (AC-22), and when enabled it enters in-restart with the restart
// reason restricted to 0 (unknown) or 3 (switch to redundant control processor) (AC-23).
// PREVENTS: an uncoordinated router forwarding on stale entries after a crash, or sending an
// out-of-spec unplanned restart reason.
package ospf

import (
	"testing"
	"time"
)

// TestUnplannedDisabledByDefault (AC-22, R-11): the default config never enters an unplanned
// restart on a cold boot.
func TestUnplannedDisabledByDefault(t *testing.T) {
	e := grEnableEngine(t, false, time.Unix(1_000_000, 0)) // planned support, unplanned OFF
	if e.gr.cfg.unplannedEnabled() {
		t.Fatalf("planned support must not enable unplanned")
	}
	e.gr.maybeUnplannedRestart()
	if e.gr.inRestart() {
		t.Fatalf("unplanned disabled: must not enter restart on cold boot")
	}

	// Also true for a wholly default (GR-off) engine.
	def := newEngine(nil)
	def.gr.maybeUnplannedRestart()
	if def.gr.inRestart() {
		t.Fatalf("GR off: must not enter unplanned restart")
	}
}

// TestUnplannedGraceBeforeHello (AC-23): with planned-and-unplanned support the cold boot
// enters in-restart and the reason is a valid unplanned reason (0 or 3).
func TestUnplannedGraceBeforeHello(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	e := grEnableEngine(t, false, now)
	e.gr.configure(gracefulRestartConfig{
		present:           true,
		RestartInterval:   120,
		RestarterSupport:  grSupportPlannedAndUnplanned,
		HelperEnabled:     true,
		StrictLSAChecking: true,
	})
	e.gr.maybeUnplannedRestart()
	if !e.gr.inRestart() {
		t.Fatalf("unplanned enabled: cold boot must enter in-restart")
	}
	if r := grUnplannedReason(); r != grReasonUnknown && r != grReasonRedundantCP {
		t.Fatalf("unplanned reason %d must be 0 or 3 (RFC 3623 sec 5)", r)
	}
	if e.gr.reason != grReasonUnknown && e.gr.reason != grReasonRedundantCP {
		t.Fatalf("in-restart reason %d must be a valid unplanned reason", e.gr.reason)
	}
}
