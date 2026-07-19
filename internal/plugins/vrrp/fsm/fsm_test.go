package fsm

import (
	"bytes"
	"go/parser"
	"go/token"
	"net/netip"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/test/sim"
)

var (
	testEpoch = time.Unix(1_000_000, 0)
	localIP   = netip.MustParseAddr("192.0.2.10")
	vip1      = netip.MustParseAddr("192.0.2.1")
)

// baseCfg is a v3, non-owner, priority-100, 1000 ms, preempt-true instance with
// a single IPv4 VIP.
func baseCfg() Config {
	return Config{
		Version:          3,
		Priority:         100,
		Preempt:          true,
		AdvertIntervalMs: 1000,
		LocalPrimaryIP:   localIP,
		VIPs:             []netip.Addr{vip1},
	}
}

func ownerCfg() Config {
	c := baseCfg()
	c.IsOwner = true
	c.Priority = 255
	return c
}

func newInstance() *Instance {
	return New(sim.NewFakeClock(testEpoch))
}

// backupInstance returns an instance forced into Backup with the master-down
// timer armed at gen 100 and genSeq at 100 (so the next arm is gen 101).
func backupInstance(cfg Config) *Instance {
	i := newInstance()
	i.cfg = cfg
	i.state = StateBackup
	i.since = i.clock.Now()
	i.activeAdverIntervalMs = cfg.AdvertIntervalMs
	i.genSeq = 100
	i.mdArmedGen = 100
	return i
}

// masterInstance returns an instance forced into Master with the advert timer
// armed at gen 100 and genSeq at 100.
func masterInstance(cfg Config) *Instance {
	i := newInstance()
	i.cfg = cfg
	i.state = StateMaster
	i.since = i.clock.Now()
	i.activeAdverIntervalMs = cfg.AdvertIntervalMs
	i.genSeq = 100
	i.advArmedGen = 100
	return i
}

func mdDur(cfg Config, activeMs int) time.Duration {
	return masterDownInterval(cfg.Version, cfg.Priority, activeMs)
}
func skewDur(cfg Config, activeMs int) time.Duration {
	return skewTime(cfg.Version, cfg.Priority, activeMs)
}

// TestFSMTransitionMatrix asserts every row of the State Transition Table as one
// case: exact ordered action slice + next state.
//
// VALIDATES: State Transition Table (every row) + AC-1,2,3,4,5,6,8,9,10,11,12,13.
// PREVENTS: R-4 (order-insensitive tests hiding AnnounceFailover-before-VIPs bugs).
//
// The state-machine rows are shared by VRRPv2 and VRRPv3 except where noted; each
// tag names the exact row that pins the requirement:
// RFC requirement: RFC3768-6.4.2-4 positive -- row backup/shutdown: Backup Shutdown cancels the master-down timer (StopTimers) and transitions to Initialize (fsm.go:172)
// RFC requirement: RFC3768-6.4.2-4 negative -- row backup/advert-timer-expired/stale: a non-Shutdown event does not transition Backup to Initialize (fsm.go:194)
// RFC requirement: RFC3768-6.4.2-6 positive -- row backup/advert/priority-zero: a Priority-0 advert sets the master-down timer to Skew_Time (fsm.go:207)
// RFC requirement: RFC3768-6.4.2-6 negative -- row backup/advert/adopt/equal-priority: a non-zero-priority advert sets master-down to Master_Down_Interval, not Skew_Time (fsm.go:217)
// RFC requirement: RFC3768-6.4.2-7 positive -- row backup/advert/adopt/equal-priority: a non-zero advert with priority >= local resets the master-down timer (fsm.go:217)
// RFC requirement: RFC3768-6.4.2-7 negative -- row backup/advert/discard/preempt-lower-priority-no-delay: preempt on and advertised priority < local does not reset master-down (fsm.go:227)
// RFC requirement: RFC3768-6.4.2-8 positive -- row backup/advert/discard/preempt-lower-priority-no-delay: preempt on and advertised priority < local discards the advert (no actions) (fsm.go:227)
// RFC requirement: RFC3768-6.4.2-8 negative -- row backup/advert/adopt/equal-priority: an advert with priority >= local is accepted, not discarded (fsm.go:217)
// RFC requirement: RFC3768-6.4.3-5 positive -- row master/shutdown: Master Shutdown cancels the advert timer, sends a Priority-0 advert, and transitions to Initialize (fsm.go:264)
// RFC requirement: RFC3768-6.4.3-5 negative -- row backup/shutdown: a Backup Shutdown sends no Priority-0 advert (fsm.go:172)
// RFC requirement: RFC3768-6.4.3-6 positive -- row master/advert-timer-expired/matching-gen: the advert timer firing sends an advert and resets the timer (fsm.go:276)
// RFC requirement: RFC3768-6.4.3-7 positive -- row master/advert/priority-zero/reset-advert-timer: a Priority-0 advert makes the Master send an advert and reset the advert timer (fsm.go:303)
// RFC requirement: RFC3768-6.4.3-7 negative -- row master/advert/losing-lower-priority/v2-silent: a non-zero losing advert does not trigger the send-and-reset (fsm.go:330)
// RFC requirement: RFC3768-6.4.3-8 positive -- rows master/advert/higher-priority/demote and .../tie-break-lost: a higher-priority advert (or equal priority with greater sender IP) cancels the advert timer, arms master-down, and demotes to Backup (fsm.go:314)
// RFC requirement: RFC3768-6.4.3-8 negative -- row master/advert/losing-lower-priority/v2-silent: a lower-priority advert does not demote the Master (fsm.go:330)
// RFC requirement: RFC3768-6.4.3-9 positive -- row master/advert/losing-lower-priority/v2-silent: a v2 Master silently discards a losing advert (no actions, stays Master) (fsm.go:330)
// RFC requirement: RFC3768-6.4.3-9 negative -- row master/advert/higher-priority/demote: a winning advert is not silently discarded, it demotes the Master (fsm.go:314).
func TestFSMTransitionMatrix(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *Instance
		event   Event
		actions []Action
		next    State
	}{
		// ---- Initialize ----
		{
			name:  "init/startup/owner",
			setup: newInstance,
			event: Startup{Config: ownerCfg()},
			actions: []Action{
				SendAdvert{Priority: 255, AdvertIntervalMs: 1000},
				InstallVIPs{VIPs: ownerCfg().VIPs},
				AnnounceFailover{},
				StartAdvertTimer{Interval: time.Second, Gen: 1},
				EmitStateChange{From: StateInitialize, To: StateMaster, Reason: ReasonStartupOwner},
			},
			next: StateMaster,
		},
		{
			name:  "init/startup/non-owner",
			setup: newInstance,
			event: Startup{Config: baseCfg()},
			actions: []Action{
				StartMasterDownTimer{Duration: mdDur(baseCfg(), 1000), Gen: 1},
				EmitStateChange{From: StateInitialize, To: StateBackup, Reason: ReasonStartup},
			},
			next: StateBackup,
		},
		{
			name:    "init/startup/priority-zero-defensive",
			setup:   newInstance,
			event:   Startup{Config: func() Config { c := baseCfg(); c.Priority = 0; return c }()},
			actions: nil,
			next:    StateInitialize,
		},
		{name: "init/shutdown", setup: newInstance, event: Shutdown{}, actions: nil, next: StateInitialize},
		{name: "init/advert", setup: newInstance, event: AdvertReceived{Priority: 100, SrcIP: vip1, IntervalMs: 1000}, actions: nil, next: StateInitialize},
		{name: "init/master-down-expired", setup: newInstance, event: MasterDownExpired{Gen: 1}, actions: nil, next: StateInitialize},
		{name: "init/config-updated", setup: newInstance, event: ConfigUpdated{Config: baseCfg()}, actions: nil, next: StateInitialize},

		// ---- Backup ----
		{
			name:    "backup/shutdown",
			setup:   func() *Instance { return backupInstance(baseCfg()) },
			event:   Shutdown{},
			actions: []Action{StopTimers{}, EmitStateChange{From: StateBackup, To: StateInitialize, Reason: ReasonShutdown}},
			next:    StateInitialize,
		},
		{
			name:  "backup/master-down-expired/matching-gen",
			setup: func() *Instance { return backupInstance(baseCfg()) },
			event: MasterDownExpired{Gen: 100},
			actions: []Action{
				SendAdvert{Priority: 100, AdvertIntervalMs: 1000},
				InstallVIPs{VIPs: baseCfg().VIPs},
				AnnounceFailover{},
				StartAdvertTimer{Interval: time.Second, Gen: 101},
				EmitStateChange{From: StateBackup, To: StateMaster, Reason: ReasonMasterDownExpired},
			},
			next: StateMaster,
		},
		{
			name: "backup/preempt-delay-expired/matching-gen",
			setup: func() *Instance {
				c := baseCfg()
				c.PreemptDelayMs = 5000
				i := backupInstance(c)
				i.pdArmedGen = 100
				return i
			},
			event: PreemptDelayExpired{Gen: 100},
			actions: []Action{
				SendAdvert{Priority: 100, AdvertIntervalMs: 1000},
				InstallVIPs{VIPs: baseCfg().VIPs},
				AnnounceFailover{},
				StartAdvertTimer{Interval: time.Second, Gen: 101},
				EmitStateChange{From: StateBackup, To: StateMaster, Reason: ReasonPreemptDelayExpired},
			},
			next: StateMaster,
		},
		{
			name:    "backup/advert/priority-zero",
			setup:   func() *Instance { return backupInstance(baseCfg()) },
			event:   AdvertReceived{Priority: 0, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 1000},
			actions: []Action{StartMasterDownTimer{Duration: skewDur(baseCfg(), 1000), Gen: 101}},
			next:    StateBackup,
		},
		{
			name: "backup/advert/priority-zero/cancels-armed-delay",
			setup: func() *Instance {
				c := baseCfg()
				c.PreemptDelayMs = 5000
				i := backupInstance(c)
				i.pdArmedGen = 100
				return i
			},
			event: AdvertReceived{Priority: 0, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 1000},
			actions: []Action{
				StopPreemptDelayTimer{},
				StartMasterDownTimer{Duration: skewDur(baseCfg(), 1000), Gen: 101},
			},
			next: StateBackup,
		},
		{
			name:    "backup/advert/adopt/equal-priority",
			setup:   func() *Instance { return backupInstance(baseCfg()) },
			event:   AdvertReceived{Priority: 100, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 4000},
			actions: []Action{StartMasterDownTimer{Duration: mdDur(baseCfg(), 4000), Gen: 101}},
			next:    StateBackup,
		},
		{
			name: "backup/advert/adopt/preempt-false-lower-priority",
			setup: func() *Instance {
				c := baseCfg()
				c.Preempt = false
				return backupInstance(c)
			},
			event:   AdvertReceived{Priority: 50, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 4000},
			actions: []Action{StartMasterDownTimer{Duration: mdDur(baseCfg(), 4000), Gen: 101}},
			next:    StateBackup,
		},
		{
			name:    "backup/advert/discard/preempt-lower-priority-no-delay",
			setup:   func() *Instance { return backupInstance(baseCfg()) },
			event:   AdvertReceived{Priority: 50, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 4000},
			actions: nil,
			next:    StateBackup,
		},
		{
			name: "backup/advert/arm-delay/preempt-lower-priority-delay-set",
			setup: func() *Instance {
				c := baseCfg()
				c.PreemptDelayMs = 5000
				return backupInstance(c)
			},
			event: AdvertReceived{Priority: 50, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 4000},
			actions: []Action{
				StartPreemptDelayTimer{Duration: 5 * time.Second, Gen: 101},
				StartMasterDownTimer{Duration: mdDur(baseCfg(), 4000), Gen: 102},
			},
			next: StateBackup,
		},
		{
			name: "backup/advert/delay-already-armed/no-rearm",
			setup: func() *Instance {
				c := baseCfg()
				c.PreemptDelayMs = 5000
				i := backupInstance(c)
				i.pdArmedGen = 100
				return i
			},
			event:   AdvertReceived{Priority: 50, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 4000},
			actions: []Action{StartMasterDownTimer{Duration: mdDur(baseCfg(), 4000), Gen: 101}},
			next:    StateBackup,
		},
		{name: "backup/advert-timer-expired/stale", setup: func() *Instance { return backupInstance(baseCfg()) }, event: AdvertTimerExpired{Gen: 100}, actions: nil, next: StateBackup},
		{name: "backup/startup/idempotent", setup: func() *Instance { return backupInstance(baseCfg()) }, event: Startup{Config: baseCfg()}, actions: nil, next: StateBackup},

		// ---- Master ----
		{
			name:  "master/shutdown",
			setup: func() *Instance { return masterInstance(baseCfg()) },
			event: Shutdown{},
			actions: []Action{
				StopTimers{},
				SendAdvertZeroPriority{},
				RemoveVIPs{VIPs: baseCfg().VIPs},
				EmitStateChange{From: StateMaster, To: StateInitialize, Reason: ReasonShutdown},
			},
			next: StateInitialize,
		},
		{
			name:  "master/advert-timer-expired/matching-gen",
			setup: func() *Instance { return masterInstance(baseCfg()) },
			event: AdvertTimerExpired{Gen: 100},
			actions: []Action{
				SendAdvert{Priority: 100, AdvertIntervalMs: 1000},
				StartAdvertTimer{Interval: time.Second, Gen: 101},
			},
			next: StateMaster,
		},
		{
			name:  "master/advert/priority-zero/reset-advert-timer",
			setup: func() *Instance { return masterInstance(baseCfg()) },
			event: AdvertReceived{Priority: 0, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 1000},
			actions: []Action{
				SendAdvert{Priority: 100, AdvertIntervalMs: 1000},
				StartAdvertTimer{Interval: time.Second, Gen: 101},
			},
			next: StateMaster,
		},
		{
			name:  "master/advert/higher-priority/demote",
			setup: func() *Instance { return masterInstance(baseCfg()) },
			event: AdvertReceived{Priority: 200, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 4000},
			actions: []Action{
				StopTimers{},
				RemoveVIPs{VIPs: baseCfg().VIPs},
				StartMasterDownTimer{Duration: mdDur(baseCfg(), 4000), Gen: 101},
				EmitStateChange{From: StateMaster, To: StateBackup, Reason: ReasonHigherPriority},
			},
			next: StateBackup,
		},
		{
			name:  "master/advert/equal-priority-greater-ip/tie-break-lost",
			setup: func() *Instance { return masterInstance(baseCfg()) },
			event: AdvertReceived{Priority: 100, SrcIP: netip.MustParseAddr("192.0.2.99"), IntervalMs: 4000},
			actions: []Action{
				StopTimers{},
				RemoveVIPs{VIPs: baseCfg().VIPs},
				StartMasterDownTimer{Duration: mdDur(baseCfg(), 4000), Gen: 101},
				EmitStateChange{From: StateMaster, To: StateBackup, Reason: ReasonTieBreakLost},
			},
			next: StateBackup,
		},
		{
			name:    "master/advert/losing-lower-priority/v3-reassert",
			setup:   func() *Instance { return masterInstance(baseCfg()) },
			event:   AdvertReceived{Priority: 50, SrcIP: netip.MustParseAddr("192.0.2.99"), IntervalMs: 4000},
			actions: []Action{SendAdvert{Priority: 100, AdvertIntervalMs: 1000}},
			next:    StateMaster,
		},
		{
			name:    "master/advert/losing-equal-priority-smaller-ip/v3-reassert",
			setup:   func() *Instance { return masterInstance(baseCfg()) },
			event:   AdvertReceived{Priority: 100, SrcIP: netip.MustParseAddr("192.0.2.5"), IntervalMs: 4000},
			actions: []Action{SendAdvert{Priority: 100, AdvertIntervalMs: 1000}},
			next:    StateMaster,
		},
		{
			name:    "master/advert/losing-equal-priority-equal-ip/v3-reassert",
			setup:   func() *Instance { return masterInstance(baseCfg()) },
			event:   AdvertReceived{Priority: 100, SrcIP: localIP, IntervalMs: 4000},
			actions: []Action{SendAdvert{Priority: 100, AdvertIntervalMs: 1000}},
			next:    StateMaster,
		},
		{
			name: "master/advert/losing-lower-priority/v2-silent",
			setup: func() *Instance {
				c := baseCfg()
				c.Version = 2
				return masterInstance(c)
			},
			event:   AdvertReceived{Priority: 50, SrcIP: netip.MustParseAddr("192.0.2.99"), IntervalMs: 1000},
			actions: nil,
			next:    StateMaster,
		},
		{name: "master/master-down-expired/stale", setup: func() *Instance { return masterInstance(baseCfg()) }, event: MasterDownExpired{Gen: 100}, actions: nil, next: StateMaster},
		{name: "master/preempt-delay-expired/stale", setup: func() *Instance { return masterInstance(baseCfg()) }, event: PreemptDelayExpired{Gen: 100}, actions: nil, next: StateMaster},
		{name: "master/startup/idempotent", setup: func() *Instance { return masterInstance(baseCfg()) }, event: Startup{Config: baseCfg()}, actions: nil, next: StateMaster},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := tt.setup()
			got := i.Handle(tt.event)
			assertActions(t, got, tt.actions)
			if i.State() != tt.next {
				t.Errorf("next state = %v, want %v", i.State(), tt.next)
			}
		})
	}
}

func assertActions(t *testing.T, got, want []Action) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("action count = %d, want %d\n got: %+v\nwant: %+v", len(got), len(want), got, want)
	}
	for idx := range want {
		if !reflect.DeepEqual(got[idx], want[idx]) {
			t.Errorf("action[%d] = %#v, want %#v", idx, got[idx], want[idx])
		}
	}
}

// TestFSMBackupReceivesAdvert is wiring row 1: an advert in Backup re-arms
// master-down, and v3 adopts the sender interval.
//
// VALIDATES: Wiring row 1 + AC-5 (v3 interval adoption; holo/uvrrpd discard bug).
func TestFSMBackupReceivesAdvert(t *testing.T) {
	i := backupInstance(baseCfg())
	got := i.Handle(AdvertReceived{Priority: 100, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 4000})
	want := []Action{StartMasterDownTimer{Duration: mdDur(baseCfg(), 4000), Gen: 101}}
	assertActions(t, got, want)
	if i.activeAdverIntervalMs != 4000 {
		t.Errorf("v3 must adopt sender interval: activeAdverIntervalMs = %d, want 4000", i.activeAdverIntervalMs)
	}
	if i.State() != StateBackup {
		t.Errorf("state = %v, want Backup", i.State())
	}
}

// TestFSMMasterDownPromotion is wiring row 2: promotion emits actions in the
// order SendAdvert -> InstallVIPs -> AnnounceFailover -> StartAdvertTimer ->
// EmitStateChange.
//
// VALIDATES: Wiring row 2 + AC-3 + R-4 (exact order, not membership).
func TestFSMMasterDownPromotion(t *testing.T) {
	// RFC requirement: RFC3768-6.4.2-5 positive -- when the Master_Down_Timer fires, the Backup sends an ADVERTISEMENT, installs VIPs, announces failover (gratuitous ARP), starts the advert timer, and transitions to Master, in that order (promoteToMaster fsm.go:368).
	i := backupInstance(baseCfg())
	got := i.Handle(MasterDownExpired{Gen: 100})
	if len(got) != 5 {
		t.Fatalf("expected 5 actions, got %d: %+v", len(got), got)
	}
	order := []string{}
	for _, a := range got {
		order = append(order, reflect.TypeOf(a).Name())
	}
	want := []string{"SendAdvert", "InstallVIPs", "AnnounceFailover", "StartAdvertTimer", "EmitStateChange"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("promotion action order = %v, want %v", order, want)
	}
	if i.State() != StateMaster {
		t.Errorf("state = %v, want Master", i.State())
	}
}

// TestFSMPreemptDelayPromotion is wiring row 3 + the full preempt-delay
// lifecycle (AC-7).
//
// VALIDATES: Wiring row 3, AC-7, Preempt-Delay Semantics table.
func TestFSMPreemptDelayPromotion(t *testing.T) {
	cfg := baseCfg()
	cfg.PreemptDelayMs = 5000
	i := backupInstance(cfg)

	// First losing advert arms the delay exactly once (gen 101) and adopts+rearms
	// master-down (gen 102).
	got := i.Handle(AdvertReceived{Priority: 50, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 4000})
	assertActions(t, got, []Action{
		StartPreemptDelayTimer{Duration: 5 * time.Second, Gen: 101},
		StartMasterDownTimer{Duration: mdDur(cfg, 4000), Gen: 102},
	})
	if i.pdArmedGen != 101 {
		t.Fatalf("delay armed gen = %d, want 101", i.pdArmedGen)
	}

	// A subsequent losing advert does NOT re-arm the delay; it only adopts and
	// re-arms master-down (hold measures from first sighting).
	got = i.Handle(AdvertReceived{Priority: 50, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 5000})
	assertActions(t, got, []Action{StartMasterDownTimer{Duration: mdDur(cfg, 5000), Gen: 103}})
	if i.pdArmedGen != 101 {
		t.Fatalf("delay must not re-arm: gen = %d, want 101", i.pdArmedGen)
	}

	// Expiry promotes with reason preempt-delay-expired.
	got = i.Handle(PreemptDelayExpired{Gen: 101})
	if i.State() != StateMaster {
		t.Fatalf("state after delay expiry = %v, want Master", i.State())
	}
	last := got[len(got)-1]
	esc, ok := last.(EmitStateChange)
	if !ok || esc.Reason != ReasonPreemptDelayExpired {
		t.Fatalf("last action = %#v, want EmitStateChange reason %q", last, ReasonPreemptDelayExpired)
	}
}

// TestFSMPreemptDelayCancels covers the cancel rows of the Preempt-Delay table:
// a rightful master cancels the delay, and a priority-0 advert cancels it.
//
// VALIDATES: AC-7 cancel paths; Preempt-Delay Semantics "Cancel: rightful
// master" and "Cancel: master resigns".
func TestFSMPreemptDelayCancels(t *testing.T) {
	cfg := baseCfg()
	cfg.PreemptDelayMs = 5000

	// Rightful master (>= local priority) cancels the delay and re-arms normally.
	i := backupInstance(cfg)
	i.pdArmedGen = 100
	got := i.Handle(AdvertReceived{Priority: 150, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 4000})
	assertActions(t, got, []Action{
		StopPreemptDelayTimer{},
		StartMasterDownTimer{Duration: mdDur(cfg, 4000), Gen: 101},
	})
	if i.pdArmedGen != 0 {
		t.Errorf("delay must be canceled: pdArmedGen = %d, want 0", i.pdArmedGen)
	}

	// Priority-0 advert cancels the delay and sets master-down to skew.
	i = backupInstance(cfg)
	i.pdArmedGen = 100
	got = i.Handle(AdvertReceived{Priority: 0, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 4000})
	assertActions(t, got, []Action{
		StopPreemptDelayTimer{},
		StartMasterDownTimer{Duration: skewDur(cfg, 1000), Gen: 101},
	})
	if i.pdArmedGen != 0 {
		t.Errorf("delay must be canceled by prio-0: pdArmedGen = %d, want 0", i.pdArmedGen)
	}
}

// TestFSMMasterTieBreak covers AC-11: the sender-IP tie-break runs ONLY in
// Master (greater IP wins, equal/smaller IP loses), including a 16-byte IPv6
// compare, and Backup ignores sender IP entirely.
//
// VALIDATES: AC-11, State Transition Table "Tie-break only in Master" note.
func TestFSMMasterTieBreak(t *testing.T) {
	// IPv6 instance, equal priority, greater sender link-local -> demote.
	cfg6 := baseCfg()
	cfg6.LocalPrimaryIP = netip.MustParseAddr("fe80::10")
	cfg6.VIPs = []netip.Addr{netip.MustParseAddr("fe80::1")}
	i := masterInstance(cfg6)
	got := i.Handle(AdvertReceived{Priority: 100, SrcIP: netip.MustParseAddr("fe80::99"), IntervalMs: 1000})
	if i.State() != StateBackup {
		t.Errorf("IPv6 greater sender must demote: state = %v, want Backup", i.State())
	}
	esc, ok := got[len(got)-1].(EmitStateChange)
	if !ok || esc.Reason != ReasonTieBreakLost {
		t.Errorf("last action = %#v, want EmitStateChange reason %q", got[len(got)-1], ReasonTieBreakLost)
	}

	// IPv6 equal priority, smaller sender -> stay Master (losing advert reassert).
	i = masterInstance(cfg6)
	got = i.Handle(AdvertReceived{Priority: 100, SrcIP: netip.MustParseAddr("fe80::5"), IntervalMs: 1000})
	if i.State() != StateMaster {
		t.Errorf("IPv6 smaller sender must stay Master: state = %v", i.State())
	}
	assertActions(t, got, []Action{SendAdvert{Priority: 100, AdvertIntervalMs: 1000}})

	// Backup must IGNORE sender IP: an equal-priority advert with a huge sender IP
	// is authoritative (adopt + re-arm), never a tie-break.
	b := backupInstance(baseCfg())
	got = b.Handle(AdvertReceived{Priority: 100, SrcIP: netip.MustParseAddr("255.255.255.255"), IntervalMs: 4000})
	assertActions(t, got, []Action{StartMasterDownTimer{Duration: mdDur(baseCfg(), 4000), Gen: 101}})
	if b.State() != StateBackup {
		t.Errorf("Backup must stay Backup regardless of sender IP: state = %v", b.State())
	}
}

// TestFSMStartupRejectsPriorityZero covers A-5: Startup with priority 0 stays in
// Initialize and emits nothing.
//
// VALIDATES: A-5 defensive row.
func TestFSMStartupRejectsPriorityZero(t *testing.T) {
	i := newInstance()
	c := baseCfg()
	c.Priority = 0
	got := i.Handle(Startup{Config: c})
	if len(got) != 0 {
		t.Errorf("priority-0 startup emitted %d actions, want 0: %+v", len(got), got)
	}
	if i.State() != StateInitialize {
		t.Errorf("state = %v, want Initialize", i.State())
	}
}

// TestFSMNoPanicInInitialize covers AC-13 (holo bug 10): every event type fired
// into Initialize returns no actions and never panics.
//
// VALIDATES: AC-13; holo panic-on-packet-in-Initialize negative test.
func TestFSMNoPanicInInitialize(t *testing.T) {
	events := []Event{
		Shutdown{},
		AdvertReceived{Priority: 100, SrcIP: vip1, IntervalMs: 1000},
		AdvertReceived{Priority: 0, SrcIP: vip1, IntervalMs: 1000},
		MasterDownExpired{Gen: 1},
		AdvertTimerExpired{Gen: 1},
		PreemptDelayExpired{Gen: 1},
	}
	for _, ev := range events {
		i := newInstance()
		got := i.Handle(ev)
		if len(got) != 0 {
			t.Errorf("%T in Initialize emitted %d actions, want 0", ev, len(got))
		}
		if i.State() != StateInitialize {
			t.Errorf("%T changed state to %v, want Initialize", ev, i.State())
		}
	}
	// ConfigUpdated in Initialize stores config silently.
	i := newInstance()
	got := i.Handle(ConfigUpdated{Config: baseCfg()})
	if len(got) != 0 {
		t.Errorf("ConfigUpdated in Initialize emitted %d actions, want 0", len(got))
	}
	if i.cfg.Priority != 100 {
		t.Errorf("ConfigUpdated must store config: priority = %d, want 100", i.cfg.Priority)
	}
}

// TestFSMStaleTimerGenerationIgnored covers AC-17: an expiry whose Gen differs
// from the currently armed generation, or for a role not currently armed, is a
// no-op in every state.
//
// VALIDATES: AC-17, Timer Generations table; holo bug 10 replacement.
func TestFSMStaleTimerGenerationIgnored(t *testing.T) {
	// RFC requirement: RFC3768-6.4.2-5 negative -- a stale (non-armed generation) Master_Down expiry does not promote the Backup, so promotion is bound to the live timer, not any expiry event (fsm.go:178).
	// RFC requirement: RFC3768-6.4.3-6 negative -- a stale advert-timer expiry in Master does not send an advert or reset the timer; only the armed generation does (fsm.go:276).
	// Backup: arm master-down (gen 100), re-arm via advert (gen 101), then a stale
	// expiry with gen 100 must be ignored.
	i := backupInstance(baseCfg())
	i.Handle(AdvertReceived{Priority: 100, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 1000}) // re-arm -> gen 101
	got := i.Handle(MasterDownExpired{Gen: 100})
	if len(got) != 0 {
		t.Errorf("stale master-down (gen 100) emitted %d actions, want 0", len(got))
	}
	if i.State() != StateBackup {
		t.Errorf("stale expiry changed state to %v", i.State())
	}
	// The current generation still promotes.
	i.Handle(MasterDownExpired{Gen: 101})
	if i.State() != StateMaster {
		t.Fatalf("current-gen expiry must promote: state = %v", i.State())
	}

	// Master: advert timer armed at gen 101 now; a MasterDownExpired for a role
	// that is not armed in Master is ignored.
	got = i.Handle(MasterDownExpired{Gen: 101})
	if len(got) != 0 {
		t.Errorf("master-down expiry in Master emitted %d actions, want 0", len(got))
	}

	// Master: a stale advert-timer expiry (old gen) is ignored; the armed gen fires.
	stale := i.Handle(AdvertTimerExpired{Gen: 1})
	if len(stale) != 0 {
		t.Errorf("stale advert expiry emitted %d actions, want 0", len(stale))
	}
	live := i.Handle(AdvertTimerExpired{Gen: i.advArmedGen})
	if len(live) != 2 {
		t.Errorf("live advert expiry emitted %d actions, want 2", len(live))
	}
}

// TestFSMConfigUpdateRegeneratesAdvert covers AC-15 (holo bug 8): ConfigUpdated
// in Master re-sends an advert built from the NEW priority/interval and re-arms
// the advert timer with the new interval; no stale pre-encoded advert survives.
//
// VALIDATES: AC-15, R-5 (SendAdvert carries parameters, never a cached buffer).
func TestFSMConfigUpdateRegeneratesAdvert(t *testing.T) {
	i := masterInstance(baseCfg())
	newCfg := baseCfg()
	newCfg.Priority = 200
	newCfg.AdvertIntervalMs = 2000
	got := i.Handle(ConfigUpdated{Config: newCfg})
	assertActions(t, got, []Action{
		SendAdvert{Priority: 200, AdvertIntervalMs: 2000},
		StartAdvertTimer{Interval: 2 * time.Second, Gen: 101},
	})
	if i.State() != StateMaster {
		t.Errorf("state = %v, want Master", i.State())
	}

	// A VIP-set change additionally reinstalls VIPs and re-announces.
	i = masterInstance(baseCfg())
	newCfg = baseCfg()
	newCfg.VIPs = []netip.Addr{vip1, netip.MustParseAddr("192.0.2.2")}
	got = i.Handle(ConfigUpdated{Config: newCfg})
	assertActions(t, got, []Action{
		SendAdvert{Priority: 100, AdvertIntervalMs: 1000},
		StartAdvertTimer{Interval: time.Second, Gen: 101},
		InstallVIPs{VIPs: newCfg.VIPs},
		AnnounceFailover{},
	})
}

// TestV2NoIntervalAdoption covers AC-16: a v2 instance never adopts a sender
// interval; Active_Adver_Interval stays pinned to local config across any advert
// sequence, and a losing advert to a v2 Master is silent.
//
// VALIDATES: AC-16, Version Behavior table (v2 no adoption, silent discard).
func TestV2NoIntervalAdoption(t *testing.T) {
	cfg := baseCfg()
	cfg.Version = 2
	cfg.AdvertIntervalMs = 1000

	b := backupInstance(cfg)
	// Accept an advert claiming a different interval; v2 must NOT adopt it.
	got := b.Handle(AdvertReceived{Priority: 100, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 5000})
	assertActions(t, got, []Action{StartMasterDownTimer{Duration: mdDur(cfg, 1000), Gen: 101}})
	if b.activeAdverIntervalMs != 1000 {
		t.Errorf("v2 must not adopt: activeAdverIntervalMs = %d, want 1000", b.activeAdverIntervalMs)
	}

	// v2 Master, losing advert -> silent (no re-assert).
	m := masterInstance(cfg)
	got = m.Handle(AdvertReceived{Priority: 50, SrcIP: netip.MustParseAddr("192.0.2.99"), IntervalMs: 1000})
	if len(got) != 0 {
		t.Errorf("v2 Master losing advert must be silent, got %d actions: %+v", len(got), got)
	}
	if m.State() != StateMaster {
		t.Errorf("state = %v, want Master", m.State())
	}
}

// TestFSMConfigUpdateInBackup covers the Backup ConfigUpdated row: recompute from
// the new priority, keep the learned interval only if a master was heard since
// entering Backup, and cancel an armed delay.
//
// VALIDATES: State Transition Table Backup/ConfigUpdated row; Active_Adver_Interval
// lifecycle table.
func TestFSMConfigUpdateInBackup(t *testing.T) {
	// No master heard since entering Backup -> use the new configured interval.
	i := backupInstance(baseCfg())
	newCfg := baseCfg()
	newCfg.Priority = 200
	newCfg.AdvertIntervalMs = 2000
	got := i.Handle(ConfigUpdated{Config: newCfg})
	assertActions(t, got, []Action{StartMasterDownTimer{Duration: mdDur(newCfg, 2000), Gen: 101}})
	if i.activeAdverIntervalMs != 2000 {
		t.Errorf("no master heard: activeAdverIntervalMs = %d, want 2000", i.activeAdverIntervalMs)
	}

	// Master heard (adopted 4000) -> keep the learned interval across reconfig.
	i = backupInstance(baseCfg())
	i.Handle(AdvertReceived{Priority: 100, SrcIP: netip.MustParseAddr("192.0.2.20"), IntervalMs: 4000}) // adopt 4000
	newCfg = baseCfg()
	newCfg.Priority = 200
	newCfg.AdvertIntervalMs = 2000
	got = i.Handle(ConfigUpdated{Config: newCfg})
	assertActions(t, got, []Action{StartMasterDownTimer{Duration: mdDur(newCfg, 4000), Gen: 102}})
	if i.activeAdverIntervalMs != 4000 {
		t.Errorf("master heard: activeAdverIntervalMs = %d, want 4000 (keep learned)", i.activeAdverIntervalMs)
	}
}

// TestFSMSnapshot exercises the state snapshot surfaced for `show vrrp`.
//
// VALIDATES: Types section "State snapshot (for show vrrp, spec-vrrp-5)".
func TestFSMSnapshot(t *testing.T) {
	i := newInstance()
	i.Handle(Startup{Config: baseCfg()})
	snap := i.Snapshot()
	if snap.State != StateBackup {
		t.Errorf("snapshot state = %v, want Backup", snap.State)
	}
	if snap.Version != 3 || snap.Priority != 100 {
		t.Errorf("snapshot version/priority = %d/%d, want 3/100", snap.Version, snap.Priority)
	}
	if snap.ConfiguredIntervalMs != 1000 || snap.ActiveIntervalMs != 1000 {
		t.Errorf("snapshot intervals = %d/%d, want 1000/1000", snap.ConfiguredIntervalMs, snap.ActiveIntervalMs)
	}
	if !snap.MasterDownArmed {
		t.Error("snapshot must report master-down armed in Backup")
	}
	if !snap.Since.Equal(testEpoch) {
		t.Errorf("snapshot since = %v, want %v", snap.Since, testEpoch)
	}
}

// TestStateString pins the human-readable state names used by logs and snapshots.
func TestStateString(t *testing.T) {
	cases := map[State]string{
		StateInitialize: "Initialize",
		StateBackup:     "Backup",
		StateMaster:     "Master",
	}
	for st, want := range cases {
		if st.String() != want {
			t.Errorf("State(%d).String() = %q, want %q", st, st.String(), want)
		}
	}
}

// TestFSMPackagePurity enforces the FSM's purity invariant mechanically: the
// non-test source imports only clock, net/netip, and time, and contains no
// goroutines or direct wall-clock calls.
//
// VALIDATES: Concurrency Model; Critical Review data-flow row; goroutine-lifecycle
// (fsm spawns zero goroutines).
func TestFSMPackagePurity(t *testing.T) {
	allowed := map[string]bool{
		"time":      true,
		"net/netip": true,
		"codeberg.org/thomas-mangin/ze/internal/core/clock": true,
	}
	// Package restrictions are enforced by the import allowlist above; these
	// tokens catch behavioral escapes the allowlist cannot (goroutines and
	// direct wall-clock scheduling, since "time" is a legitimately allowed
	// import).
	banned := []string{"go func", "time.Now", "time.After(", "time.NewTimer", "time.NewTicker"}

	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if !allowed[p] {
				t.Errorf("%s imports disallowed package %q (FSM must stay pure: clock, net/netip, time only)", name, p)
			}
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, tok := range banned {
			if bytes.Contains(src, []byte(tok)) {
				t.Errorf("%s contains forbidden token %q (side effects must be actions, not I/O)", name, tok)
			}
		}
	}
}
