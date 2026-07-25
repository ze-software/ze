package fsm

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/sim"
)

// Conformance-style scenario tests: full event scripts over two cooperating FSM
// instances, in the spirit of holo-vrrp's jsonl corpus. Each router is a pure
// FSM; the test plays the role of the engine, converting one router's SendAdvert
// action into the other's AdvertReceived event. Timers are fired explicitly.

func newRouter(cfg Config) *Instance {
	i := New(sim.NewFakeClock(testEpoch))
	i.Handle(Startup{Config: cfg})
	return i
}

func routerCfg(priority uint8, ip string) Config {
	return Config{
		Version:          3,
		Priority:         priority,
		Preempt:          true,
		AdvertIntervalMs: 1000,
		LocalPrimaryIP:   netip.MustParseAddr(ip),
		VIPs:             []netip.Addr{vip1},
	}
}

func findAction[T Action](actions []Action) (T, bool) {
	for _, a := range actions {
		if v, ok := a.(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

func countAction[T Action](actions []Action) int {
	n := 0
	for _, a := range actions {
		if _, ok := a.(T); ok {
			n++
		}
	}
	return n
}

// advertFrom builds the AdvertReceived event a peer would decode from a router's
// SendAdvert action and configuration.
func advertFrom(sa SendAdvert, cfg Config) AdvertReceived {
	return AdvertReceived{
		Priority:   sa.Priority,
		SrcIP:      cfg.LocalPrimaryIP,
		IntervalMs: sa.AdvertIntervalMs,
		VIPCount:   len(cfg.VIPs),
	}
}

// TestFSMScenarioElection: two routers start as Backup; the higher-priority
// router's master-down fires first, it promotes and advertises, and the
// lower-priority router adopts and stays Backup.
//
// VALIDATES: End-to-End story 1; TestFSMScenarioElection (TDD plan).
func TestFSMScenarioElection(t *testing.T) {
	high := newRouter(routerCfg(200, "192.0.2.20"))
	low := newRouter(routerCfg(100, "192.0.2.10"))
	if high.State() != StateBackup || low.State() != StateBackup {
		t.Fatalf("both routers must start Backup: high=%v low=%v", high.State(), low.State())
	}

	// The higher-priority router has the smaller skew, so its master-down fires
	// first. Promote it.
	acts := high.Handle(MasterDownExpired{Gen: high.mdArmedGen})
	if high.State() != StateMaster {
		t.Fatalf("high router must promote: state=%v", high.State())
	}
	sa, ok := findAction[SendAdvert](acts)
	if !ok {
		t.Fatal("promotion must emit SendAdvert")
	}

	// The low router hears the winner (priority 200 >= 100): adopt, stay Backup.
	lowActs := low.Handle(advertFrom(sa, high.cfg))
	if low.State() != StateBackup {
		t.Fatalf("low router must stay Backup after hearing the winner: state=%v", low.State())
	}
	if _, ok := findAction[StartMasterDownTimer](lowActs); !ok {
		t.Fatal("low router must re-arm master-down on the winner's advert")
	}
	if low.activeAdverIntervalMs != 1000 {
		t.Errorf("low router adopted interval = %d, want 1000", low.activeAdverIntervalMs)
	}
}

// TestFSMScenarioPreemptOnOff: a higher-priority router returning while a live
// lower-priority master advertises yields four distinct first-advert outcomes
// across {Preempt} x {delay}.
//
// VALIDATES: TestFSMScenarioPreemptOnOff (TDD plan); Preempt-Delay Semantics.
func TestFSMScenarioPreemptOnOff(t *testing.T) {
	// The live master advertises priority 100 (lower than our returning box's 200).
	masterAdvert := AdvertReceived{Priority: 100, SrcIP: netip.MustParseAddr("192.0.2.10"), IntervalMs: 1000}

	cases := []struct {
		name    string
		preempt bool
		delayMs int
		assert  func(t *testing.T, i *Instance, acts []Action)
	}{
		{
			name: "preempt-true-no-delay-discards", preempt: true, delayMs: 0,
			assert: func(t *testing.T, i *Instance, acts []Action) {
				if len(acts) != 0 {
					t.Errorf("preempt+no-delay must discard the losing advert, got %+v", acts)
				}
			},
		},
		{
			name: "preempt-false-adopts", preempt: false, delayMs: 0,
			assert: func(t *testing.T, i *Instance, acts []Action) {
				if _, ok := findAction[StartMasterDownTimer](acts); !ok {
					t.Errorf("preempt=false must adopt+re-arm, got %+v", acts)
				}
			},
		},
		{
			name: "preempt-true-delay-arms", preempt: true, delayMs: 5000,
			assert: func(t *testing.T, i *Instance, acts []Action) {
				if _, ok := findAction[StartPreemptDelayTimer](acts); !ok {
					t.Errorf("preempt+delay must arm the delay timer, got %+v", acts)
				}
				if i.pdArmedGen == 0 {
					t.Error("delay must be armed")
				}
			},
		},
		{
			name: "preempt-false-delay-ignored", preempt: false, delayMs: 5000,
			assert: func(t *testing.T, i *Instance, acts []Action) {
				if _, ok := findAction[StartPreemptDelayTimer](acts); ok {
					t.Errorf("preempt=false must ignore the delay, got %+v", acts)
				}
				if _, ok := findAction[StartMasterDownTimer](acts); !ok {
					t.Errorf("preempt=false must adopt+re-arm, got %+v", acts)
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := routerCfg(200, "192.0.2.20")
			cfg.Preempt = c.preempt
			cfg.PreemptDelayMs = c.delayMs
			i := newRouter(cfg)
			acts := i.Handle(masterAdvert)
			if i.State() != StateBackup {
				t.Fatalf("must stay Backup while master is alive: state=%v", i.State())
			}
			c.assert(t, i, acts)
		})
	}
}

// TestFSMScenarioGracefulShutdown: the master resigns with a priority-0 advert;
// the backup shortens master-down to Skew_Time and promotes on expiry.
//
// VALIDATES: End-to-End story 4; TestFSMScenarioGracefulShutdown (TDD plan).
func TestFSMScenarioGracefulShutdown(t *testing.T) {
	master := newRouter(routerCfg(200, "192.0.2.20"))
	master.Handle(MasterDownExpired{Gen: master.mdArmedGen}) // promote to Master
	if master.State() != StateMaster {
		t.Fatalf("setup: master must be Master, got %v", master.State())
	}
	backup := newRouter(routerCfg(100, "192.0.2.10"))

	// Master gracefully stops: emits a priority-0 advert.
	shutActs := master.Handle(Shutdown{})
	if _, ok := findAction[SendAdvertZeroPriority](shutActs); !ok {
		t.Fatal("graceful shutdown must send a priority-0 advert")
	}

	// Backup hears priority 0: master-down collapses to Skew_Time.
	prio0 := AdvertReceived{Priority: 0, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 1000}
	acts := backup.Handle(prio0)
	sm, ok := findAction[StartMasterDownTimer](acts)
	if !ok {
		t.Fatal("priority-0 advert must re-arm master-down")
	}
	wantSkew := skewTime(3, 100, 1000)
	if sm.Duration != wantSkew {
		t.Errorf("priority-0 master-down = %v, want Skew_Time %v", sm.Duration, wantSkew)
	}

	// After Skew_Time the backup promotes.
	backup.Handle(MasterDownExpired{Gen: backup.mdArmedGen})
	if backup.State() != StateMaster {
		t.Fatalf("backup must promote after skew, got %v", backup.State())
	}
}

// TestFSMScenarioMasterFlap: a backup promotes on silence (master-down expiry),
// installing VIPs once; the old master returns at higher priority and the new
// master demotes, removing VIPs exactly once.
//
// VALIDATES: End-to-End story 2; TestFSMScenarioMasterFlap (TDD plan).
func TestFSMScenarioMasterFlap(t *testing.T) {
	box := newRouter(routerCfg(100, "192.0.2.10"))

	// Master goes silent -> master-down expires -> promote, install VIPs once.
	promote := box.Handle(MasterDownExpired{Gen: box.mdArmedGen})
	if box.State() != StateMaster {
		t.Fatalf("must promote on silence, got %v", box.State())
	}
	if n := countAction[InstallVIPs](promote); n != 1 {
		t.Fatalf("promotion must install VIPs exactly once, got %d", n)
	}

	// The old master returns at higher priority -> demote, remove VIPs once.
	demote := box.Handle(AdvertReceived{Priority: 250, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 1000})
	if box.State() != StateBackup {
		t.Fatalf("higher-priority return must demote, got %v", box.State())
	}
	if n := countAction[RemoveVIPs](demote); n != 1 {
		t.Fatalf("demotion must remove VIPs exactly once, got %d", n)
	}
	if _, ok := findAction[StopTimers](demote); !ok {
		t.Error("demotion must stop timers")
	}
}

var _ = time.Second
