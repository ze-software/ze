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
	// RFC requirement: RFC3623-5-1 negative -- with unplanned recovery turned OFF the operator
	// has disabled the option (RFC 3623 sec 5): unplannedEnabled() is false, so
	// maybeUnplannedRestart() enters no restart and originates no Grace-LSA on cold boot
	// (gr_restarter.go:100 gate; config.go unplannedEnabled defaults off).
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
	// RFC requirement: RFC3623-5-1 positive -- when the operator opts INTO unplanned recovery
	// (grSupportPlannedAndUnplanned) the option is active (RFC 3623 sec 5): unplannedEnabled()
	// is true, so maybeUnplannedRestart() enters in-restart and originates Grace-LSAs on cold
	// boot (gr_restarter.go:95-112), proving the recovery this requirement lets the operator
	// disable actually exists.
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
	// RFC requirement: RFC3623-5-4 positive -- on an unplanned restart the restart reason set in
	// the Grace-LSAs MUST be 0 (unknown) or 3 (switch to redundant control processor): the reason
	// grUnplannedReason feeds origination, and the reason recorded in-restart, are both in {0,3}
	// (RFC 3623 sec 5; gr_restarter.go:88 -> grReasonRedundantCP=3, gr.go:36).
	if r := grUnplannedReason(); r != grReasonUnknown && r != grReasonRedundantCP {
		t.Fatalf("unplanned reason %d must be 0 or 3 (RFC 3623 sec 5)", r)
	}
	if e.gr.reason != grReasonUnknown && e.gr.reason != grReasonRedundantCP {
		t.Fatalf("in-restart reason %d must be a valid unplanned reason", e.gr.reason)
	}
}
