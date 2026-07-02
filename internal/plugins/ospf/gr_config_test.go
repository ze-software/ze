// VALIDATES: the family-neutral `graceful-restart` config resolves the restarter support
// enum + interval and the helper support + strict-lsa-checking, inherits into the OSPFv3
// sub-config, applies its RFC 3623 App B defaults, and rejects an out-of-range interval
// (1..1800, AC-24).
// PREVENTS: a mis-parsed GR policy, a grace period above LSRefreshTime, or the OSPFv3 family
// silently running without the router-wide GR policy.
package ospf

import (
	"errors"
	"testing"
)

func TestGracefulRestartConfigResolves(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","graceful-restart":{"restarter":{"support":"planned-and-unplanned","restart-interval":300},"helper":{"support":false,"strict-lsa-checking":false}},"areas":{"area":{"0":{"area-id":"0"}}},"address-family":{"ipv6":{"areas":{"area":{"0":{"area-id":"0"}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	gr := cfg.GracefulRestart
	if !gr.present || gr.RestarterSupport != grSupportPlannedAndUnplanned || gr.RestartInterval != 300 {
		t.Fatalf("restarter config wrong: %+v", gr)
	}
	if gr.HelperEnabled || gr.StrictLSAChecking {
		t.Fatalf("helper config wrong: %+v", gr)
	}
	if !gr.unplannedEnabled() || !gr.restarterEnabled() {
		t.Fatalf("support predicates wrong: %+v", gr)
	}
	// RFC 3623 / RFC 5187: the OSPFv3 family inherits the router-wide GR policy.
	if cfg.V6 == nil || cfg.V6.GracefulRestart.RestarterSupport != grSupportPlannedAndUnplanned {
		t.Fatalf("OSPFv3 family must inherit the graceful-restart policy: %+v", cfg.V6)
	}
}

func TestGracefulRestartConfigDefaults(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","graceful-restart":{},"areas":{"area":{"0":{"area-id":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	gr := cfg.GracefulRestart
	// RFC 3623 App B defaults: restarter disabled, interval 120, helper on, strict on.
	if gr.RestarterSupport != grSupportDisabled || gr.RestartInterval != DefaultRestartInterval || !gr.HelperEnabled || !gr.StrictLSAChecking {
		t.Fatalf("defaults wrong: %+v", gr)
	}
	if gr.restarterEnabled() {
		t.Fatalf("default must not enable the restarter")
	}
}

// TestGracePeriodRangeRejectsAbove1800 (AC-24, R-12): the restart interval must be 1..1800 s.
func TestGracePeriodRangeRejectsAbove1800(t *testing.T) {
	cfg := defaultOSPFConfig()
	cfg.present = true
	cfg.RouterID = mustRouterID(t, "10.0.0.1")
	cfg.GracefulRestart = gracefulRestartConfig{present: true, RestartInterval: 1801, RestarterSupport: grSupportPlanned, HelperEnabled: true}
	if err := validateConfig(cfg); !errors.Is(err, ErrGraceIntervalRange) {
		t.Fatalf("validateConfig(interval 1801) = %v, want ErrGraceIntervalRange", err)
	}
	// 1800 is the last valid value.
	cfg.GracefulRestart.RestartInterval = 1800
	if err := validateConfig(cfg); errors.Is(err, ErrGraceIntervalRange) {
		t.Fatalf("validateConfig(interval 1800) must accept the boundary")
	}
	// 0 is below the range.
	cfg.GracefulRestart.RestartInterval = 0
	if err := validateConfig(cfg); !errors.Is(err, ErrGraceIntervalRange) {
		t.Fatalf("validateConfig(interval 0) = %v, want ErrGraceIntervalRange", err)
	}
}
