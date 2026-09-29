package fsm

import (
	"net/netip"
	"testing"
	"time"
)

// The expected durations in this file are literals worked from the RFC
// formulas, never values computed by skewTime or masterDownInterval, so a
// wrong formula cannot agree with itself.
//
// VRRPv2 (RFC 3768 Section 6.1), Priority 100, Advertisement_Interval 2 s:
//
//	Skew_Time            = (256 - 100) / 256 s  = 0.609375 s
//	Master_Down_Interval = 3 * 2 s + Skew_Time  = 6.609375 s
//
// The RFC 9568 interval-scaled Skew_Time would be 1.21875 s at 2 s, so the two
// documents' values differ and a v2 instance running the v3 formula is caught.
//
// VRRPv3 (RFC 5798 Section 6.1), Priority 100, adopted Master_Adver_Interval 4 s:
//
//	Skew_Time            = (256 - 100) * 4 s / 256 = 2.4375 s
//	Master_Down_Interval = 3 * 4 s + Skew_Time     = 14.4375 s
const (
	v2SkewTime       = 609375 * time.Microsecond
	v2MasterDown     = 6609375 * time.Microsecond
	v3MasterDownAt4s = 14437500 * time.Microsecond
)

var (
	senderLow  = netip.MustParseAddr("192.0.2.5")
	senderHigh = netip.MustParseAddr("192.0.2.200") // last octet >= 128: an unsigned compare is needed
)

// v2Cfg is a VRRPv2 Backup-capable router: Priority 100, Preempt on, 2 s
// Advertisement_Interval, local primary 192.0.2.10.
func v2Cfg() Config {
	c := baseCfg()
	c.Version = 2
	c.AdvertIntervalMs = 2000
	return c
}

// TestV2BackupShutdown proves a VRRPv2 Backup that receives Shutdown cancels
// its Master_Down_Timer and moves to Initialize.
//
// Method: a v2 Backup with the down-timer armed at generation 100 takes
// Shutdown; the actions must be exactly StopTimers then the Initialize state
// change. Then the expiry of the canceled timer (generation 100) is fed: it
// must produce nothing and the router must stay in Initialize, which is what a
// timer that was not canceled would violate.
//
// RFC requirement: RFC3768-6.4.2-4 positive -- a VRRPv2 Backup receiving Shutdown emits StopTimers and a Backup->Initialize state change and ends in Initialize (handleBackup fsm.go).
// RFC requirement: RFC3768-6.4.2-4 negative -- after that Shutdown, the expiry of the canceled Master_Down_Timer produces no action and does not promote the router (handleBackup fsm.go, stopAllTimers fsm.go).
func TestV2BackupShutdown(t *testing.T) {
	i := backupInstance(v2Cfg())
	got := i.Handle(Shutdown{})
	assertActions(t, got, []Action{
		StopTimers{},
		EmitStateChange{From: StateBackup, To: StateInitialize, Reason: ReasonShutdown},
	})
	if i.State() != StateInitialize {
		t.Fatalf("state = %v, want Initialize", i.State())
	}
	if got := i.Handle(MasterDownExpired{Gen: 100}); len(got) != 0 {
		t.Fatalf("canceled Master_Down_Timer fired: %+v", got)
	}
	if i.State() != StateInitialize {
		t.Fatalf("state after the canceled timer's expiry = %v, want Initialize", i.State())
	}
}

// TestPromotionSetsAdverTimerToOwnInterval proves the Master_Down_Timer
// expiry sequence for both versions: send an ADVERTISEMENT, install and
// announce the addresses, set the Adver_Timer to the router's own
// Advertisement_Interval, move to Master, in that order and with those values.
//
// Method: a v2 Backup (2 s) and a v3 Backup (1 s) that first learned a 4 s
// interval from a Master each take the live MasterDownExpired. The actions are
// compared whole, including the SendAdvert priority and interval, the VIP list
// and the Adver_Timer interval (2 s and 1 s, not the learned 4 s). Negative: a
// MasterDownExpired carrying a generation that is not armed produces nothing
// and the Backup stays Backup.
//
// RFC requirement: RFC3768-6.4.2-5 positive -- a VRRPv2 Backup whose Master_Down_Timer fires emits SendAdvert{100, 2000}, InstallVIPs, AnnounceFailover, StartAdvertTimer{2s} and the Backup->Master change, in that order (promoteToMaster fsm.go).
// RFC requirement: RFC3768-6.4.2-5 negative -- an expiry for a generation that is not armed does not promote the VRRPv2 Backup (handleBackup fsm.go).
// RFC requirement: RFC5798-6.4.2-7 positive -- a VRRPv3 Backup that learned a 4 s interval, on Master_Down_Timer expiry, emits SendAdvert{100, 1000}, InstallVIPs, AnnounceFailover, StartAdvertTimer{1s} (its own Advertisement_Interval) and the Backup->Master change, in that order (promoteToMaster fsm.go).
// RFC requirement: RFC9568-6.4.2-7 positive -- a VRRPv3 Backup that learned a 4 s interval, on Active_Down_Timer expiry, emits SendAdvert{100, 1000}, InstallVIPs, AnnounceFailover, StartAdvertTimer{1s} (its own Advertisement_Interval) and the Backup->Active change, in that order (promoteToMaster fsm.go).
func TestPromotionSetsAdverTimerToOwnInterval(t *testing.T) {
	v2 := backupInstance(v2Cfg())
	assertActions(t, v2.Handle(MasterDownExpired{Gen: 100}), []Action{
		SendAdvert{Priority: 100, AdvertIntervalMs: 2000},
		InstallVIPs{VIPs: []netip.Addr{vip1}},
		AnnounceFailover{},
		StartAdvertTimer{Interval: 2 * time.Second, Gen: 101},
		EmitStateChange{From: StateBackup, To: StateMaster, Reason: ReasonMasterDownExpired},
	})
	if v2.State() != StateMaster {
		t.Fatalf("v2 state = %v, want Master", v2.State())
	}

	v3 := backupInstance(baseCfg())
	v3.Handle(AdvertReceived{Priority: 150, SrcIP: senderLow, IntervalMs: 4000}) // arms gen 101
	assertActions(t, v3.Handle(MasterDownExpired{Gen: 101}), []Action{
		SendAdvert{Priority: 100, AdvertIntervalMs: 1000},
		InstallVIPs{VIPs: []netip.Addr{vip1}},
		AnnounceFailover{},
		StartAdvertTimer{Interval: time.Second, Gen: 102},
		EmitStateChange{From: StateBackup, To: StateMaster, Reason: ReasonMasterDownExpired},
	})

	stale := backupInstance(v2Cfg())
	if got := stale.Handle(MasterDownExpired{Gen: 99}); len(got) != 0 {
		t.Fatalf("unarmed generation promoted: %+v", got)
	}
	if stale.State() != StateBackup {
		t.Fatalf("state after unarmed expiry = %v, want Backup", stale.State())
	}
}

// TestV2BackupPriorityZeroSetsSkewTime proves a VRRPv2 Backup sets the
// Master_Down_Timer to the RFC 3768 Skew_Time on a Priority 0 advertisement.
//
// Method: positive, Priority 0 must arm exactly 0.609375 s, the
// interval-independent RFC 3768 value (the RFC 9568 form would give 1.21875 s).
// Negative, a non-zero Priority 1 advertisement under Preempt off must NOT arm
// Skew_Time: it arms the full 6.609375 s Master_Down_Interval.
//
// RFC requirement: RFC3768-6.4.2-6 positive -- a VRRPv2 Backup at Priority 100 and 2 s arms StartMasterDownTimer{0.609375s}, (256-100)/256 s, on a Priority 0 advertisement (backupAdvert fsm.go, skewTime timers.go).
// RFC requirement: RFC3768-6.4.2-6 negative -- a non-zero-priority advertisement does not set the timer to Skew_Time; it arms the 6.609375 s Master_Down_Interval (backupAdvert fsm.go).
func TestV2BackupPriorityZeroSetsSkewTime(t *testing.T) {
	i := backupInstance(v2Cfg())
	assertActions(t, i.Handle(AdvertReceived{Priority: 0, SrcIP: senderLow, IntervalMs: 2000}),
		[]Action{StartMasterDownTimer{Duration: v2SkewTime, Gen: 101}})

	cfg := v2Cfg()
	cfg.Preempt = false
	j := backupInstance(cfg)
	assertActions(t, j.Handle(AdvertReceived{Priority: 1, SrcIP: senderLow, IntervalMs: 2000}),
		[]Action{StartMasterDownTimer{Duration: v2MasterDown, Gen: 101}})
}

// TestV2BackupResetsOrDiscards proves the VRRPv2 Backup advertisement rule:
// with Preempt_Mode False, or an advertised Priority greater than or equal to
// the local one, reset the Master_Down_Timer to the local
// Master_Down_Interval; else discard the advertisement.
//
// Method: the three accepting cases (equal priority, greater priority, lower
// priority with Preempt off) each arm exactly 6.609375 s and leave the router
// in Backup. The discarding case (Preempt on, lower priority) produces no
// action, and the down-timer armed before it (generation 100) still promotes,
// which shows the advertisement did not reset it.
//
// RFC requirement: RFC3768-6.4.2-7 positive -- a VRRPv2 Backup resets Master_Down_Timer to the local 6.609375 s on an equal-priority advert, a higher-priority advert, and a lower-priority advert with Preempt off (backupAdvert fsm.go).
// RFC requirement: RFC3768-6.4.2-7 negative -- with Preempt on, a lower-priority advert does not reset the timer: no action, and the previously armed timer still promotes (backupAdvert fsm.go).
// RFC requirement: RFC3768-6.4.2-8 positive -- the equal, higher and Preempt-off cases reset Master_Down_Timer to the local 6.609375 s Master_Down_Interval (backupAdvert fsm.go).
// RFC requirement: RFC3768-6.4.2-8 negative -- with Preempt on, a lower-priority advertisement is discarded: no action, state Backup, and the old timer still promotes (backupAdvert fsm.go).
func TestV2BackupResetsOrDiscards(t *testing.T) {
	preemptOff := v2Cfg()
	preemptOff.Preempt = false
	for _, tc := range []struct {
		name     string
		cfg      Config
		priority uint8
	}{
		{"equal", v2Cfg(), 100},
		{"greater", v2Cfg(), 200},
		{"lower-preempt-off", preemptOff, 50},
	} {
		i := backupInstance(tc.cfg)
		got := i.Handle(AdvertReceived{Priority: tc.priority, SrcIP: senderLow, IntervalMs: 2000})
		assertActions(t, got, []Action{StartMasterDownTimer{Duration: v2MasterDown, Gen: 101}})
		if i.State() != StateBackup {
			t.Errorf("%s: state = %v, want Backup", tc.name, i.State())
		}
	}

	i := backupInstance(v2Cfg())
	if got := i.Handle(AdvertReceived{Priority: 50, SrcIP: senderLow, IntervalMs: 2000}); len(got) != 0 {
		t.Fatalf("lower-priority advert under Preempt was not discarded: %+v", got)
	}
	if i.State() != StateBackup {
		t.Fatalf("state = %v, want Backup", i.State())
	}
	if got := i.Handle(MasterDownExpired{Gen: 100}); len(got) == 0 || i.State() != StateMaster {
		t.Fatalf("the discarded advert reset the down-timer: expiry gave %+v, state %v", got, i.State())
	}
}

// TestV2MasterAdverTimerFires proves a VRRPv2 Master whose Adver_Timer fires
// sends an ADVERTISEMENT and resets the Adver_Timer to Advertisement_Interval.
//
// Method: positive, the live expiry (generation 100) gives exactly
// SendAdvert{100, 2000} then StartAdvertTimer{2s}. Negative, an expiry for a
// generation that is not armed sends nothing and resets nothing.
//
// RFC requirement: RFC3768-6.4.3-6 positive -- a VRRPv2 Master's Adver_Timer expiry emits SendAdvert{100, 2000} then StartAdvertTimer{2s} (handleMaster fsm.go).
// RFC requirement: RFC3768-6.4.3-6 negative -- an expiry for an unarmed generation emits no advertisement and no timer reset (handleMaster fsm.go).
func TestV2MasterAdverTimerFires(t *testing.T) {
	m := masterInstance(v2Cfg())
	assertActions(t, m.Handle(AdvertTimerExpired{Gen: 100}), []Action{
		SendAdvert{Priority: 100, AdvertIntervalMs: 2000},
		StartAdvertTimer{Interval: 2 * time.Second, Gen: 101},
	})
	if got := masterInstance(v2Cfg()).Handle(AdvertTimerExpired{Gen: 99}); len(got) != 0 {
		t.Fatalf("unarmed Adver_Timer expiry acted: %+v", got)
	}
}

// TestV2MasterPriorityZero proves a VRRPv2 Master that receives a Priority 0
// advertisement sends an ADVERTISEMENT and resets the Adver_Timer.
//
// Method: positive, Priority 0 gives exactly SendAdvert{100, 2000} then
// StartAdvertTimer{2s}, and the router stays Master. Negative, a non-zero
// lower-priority advertisement to the same v2 Master sends nothing and resets
// nothing, so the response is bound to Priority 0.
//
// RFC requirement: RFC3768-6.4.3-7 positive -- a VRRPv2 Master receiving a Priority 0 advert emits SendAdvert{100, 2000} then StartAdvertTimer{2s} and stays Master (masterAdvert fsm.go).
// RFC requirement: RFC3768-6.4.3-7 negative -- a VRRPv2 Master receiving a non-zero lower-priority advert emits no advertisement and no Adver_Timer reset (masterAdvert fsm.go).
func TestV2MasterPriorityZero(t *testing.T) {
	m := masterInstance(v2Cfg())
	assertActions(t, m.Handle(AdvertReceived{Priority: 0, SrcIP: senderHigh, IntervalMs: 2000}), []Action{
		SendAdvert{Priority: 100, AdvertIntervalMs: 2000},
		StartAdvertTimer{Interval: 2 * time.Second, Gen: 101},
	})
	if m.State() != StateMaster {
		t.Fatalf("state = %v, want Master", m.State())
	}
	if got := masterInstance(v2Cfg()).Handle(AdvertReceived{Priority: 50, SrcIP: senderHigh, IntervalMs: 2000}); len(got) != 0 {
		t.Fatalf("non-zero lower-priority advert got a Priority 0 response: %+v", got)
	}
}

// TestV2MasterDemotesOrDiscards proves the VRRPv2 Master advertisement rule:
// a higher priority, or an equal priority from a greater sender primary
// address, cancels the Adver_Timer, sets the Master_Down_Timer to the local
// Master_Down_Interval and moves to Backup; anything else is discarded.
//
// Method: higher priority (200 from 192.0.2.5) and equal priority from
// 192.0.2.200 (greater than the local 192.0.2.10 only under an unsigned octet
// compare) each give exactly StopTimers, RemoveVIPs, StartMasterDownTimer
// {6.609375s} and the Master->Backup change. Equal priority from 192.0.2.5 and
// lower priority 50 each give no action and leave the router Master.
//
// RFC requirement: RFC3768-6.4.3-9 positive -- a VRRPv2 Master demotes on a higher-priority advert and on an equal-priority advert from a greater sender address: StopTimers, RemoveVIPs, StartMasterDownTimer{6.609375s}, Master->Backup (masterAdvert, demoteToBackup fsm.go).
// RFC requirement: RFC3768-6.4.3-9 negative -- a VRRPv2 Master discards an equal-priority advert from a smaller sender address and a lower-priority advert: no action, stays Master (masterAdvert fsm.go).
func TestV2MasterDemotesOrDiscards(t *testing.T) {
	for _, tc := range []struct {
		priority uint8
		sender   netip.Addr
		reason   string
	}{
		{200, senderLow, ReasonHigherPriority},
		{100, senderHigh, ReasonTieBreakLost},
	} {
		m := masterInstance(v2Cfg())
		assertActions(t, m.Handle(AdvertReceived{Priority: tc.priority, SrcIP: tc.sender, IntervalMs: 2000}), []Action{
			StopTimers{},
			RemoveVIPs{VIPs: []netip.Addr{vip1}},
			StartMasterDownTimer{Duration: v2MasterDown, Gen: 101},
			EmitStateChange{From: StateMaster, To: StateBackup, Reason: tc.reason},
		})
		if m.State() != StateBackup {
			t.Errorf("priority %d from %v: state = %v, want Backup", tc.priority, tc.sender, m.State())
		}
	}
	for _, tc := range []struct {
		priority uint8
		sender   netip.Addr
	}{{100, senderLow}, {50, senderHigh}} {
		m := masterInstance(v2Cfg())
		if got := m.Handle(AdvertReceived{Priority: tc.priority, SrcIP: tc.sender, IntervalMs: 2000}); len(got) != 0 {
			t.Errorf("priority %d from %v was not discarded: %+v", tc.priority, tc.sender, got)
		}
		if m.State() != StateMaster {
			t.Errorf("priority %d from %v: state = %v, want Master", tc.priority, tc.sender, m.State())
		}
	}
}

// TestV3BackupAdoptsIntervalOrDiscards proves the RFC 5798 Section 6.4.2
// Backup rule: with Preempt_Mode False, or an advertised Priority greater than
// or equal to the local one, set Master_Adver_Interval from the advertisement,
// recompute Master_Down_Interval and reset the timer; else discard.
//
// Method: equal (100), greater (200) and Preempt-off lower (50) advertisements
// carrying 4 s each arm exactly 14.4375 s, the down interval recomputed from
// the adopted 4 s. Negative: Preempt on, lower priority 50 carrying 4 s gives
// no action and the router keeps its own 1 s as Master_Adver_Interval.
//
// RFC requirement: RFC5798-6.4.2-9 positive -- a VRRPv3 Backup adopts the advertised 4 s and arms StartMasterDownTimer{14.4375s} on an equal-priority, a higher-priority and a Preempt-off lower-priority advert (backupAdvert fsm.go).
// RFC requirement: RFC5798-6.4.2-9 negative -- with Preempt on, a lower-priority advert is discarded: no action and Master_Adver_Interval is not adopted (backupAdvert fsm.go).
func TestV3BackupAdoptsIntervalOrDiscards(t *testing.T) {
	preemptOff := baseCfg()
	preemptOff.Preempt = false
	for _, tc := range []struct {
		name     string
		cfg      Config
		priority uint8
	}{
		{"equal", baseCfg(), 100},
		{"greater", baseCfg(), 200},
		{"lower-preempt-off", preemptOff, 50},
	} {
		i := backupInstance(tc.cfg)
		got := i.Handle(AdvertReceived{Priority: tc.priority, SrcIP: senderLow, IntervalMs: 4000})
		assertActions(t, got, []Action{StartMasterDownTimer{Duration: v3MasterDownAt4s, Gen: 101}})
		if i.activeAdverIntervalMs != 4000 {
			t.Errorf("%s: Master_Adver_Interval = %d ms, want 4000", tc.name, i.activeAdverIntervalMs)
		}
	}

	i := backupInstance(baseCfg())
	if got := i.Handle(AdvertReceived{Priority: 50, SrcIP: senderLow, IntervalMs: 4000}); len(got) != 0 {
		t.Fatalf("lower-priority advert under Preempt was not discarded: %+v", got)
	}
	if i.activeAdverIntervalMs != 1000 {
		t.Fatalf("discarded advert changed Master_Adver_Interval to %d ms", i.activeAdverIntervalMs)
	}
}
