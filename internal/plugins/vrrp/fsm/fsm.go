// Design: plan/learned/1124-vrrp-first-hop-redundancy.md -- VRRP per-instance state machine
// RFC: rfc/short/rfc9568.md (VRRPv3 Section 6.4) and rfc/short/rfc3768.md (VRRPv2 Section 6.4)
//
// The per-group VRRP state machine: State type, the Instance struct, the single
// synchronous Handle method implementing the State Transition Table, and the
// Snapshot surfaced for `show vrrp`. See doc.go for the purity/threading
// invariants. Timer arithmetic lives in timers.go.
package fsm

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
)

// State is the VRRP instance state. RFC 9568 renamed Master to "Active Router";
// this package keeps Master to match operator vocabulary (see doc.go).
//
// RFC 9568 Section 6.4 / RFC 3768 Section 6.4: one state machine instance per
// Virtual Router, with states Initialize, Backup, and Active(Master).
type State uint8

const (
	// StateInitialize: RFC 9568 Section 6.4.1 -- the instance is not
	// participating; it waits for Startup.
	StateInitialize State = iota
	// StateBackup: RFC 9568 Section 6.4.2 -- monitoring the Active Router,
	// ready to promote when the down-timer fires.
	StateBackup
	// StateMaster: RFC 9568 Section 6.4.3 (RFC term "Active") -- forwarding for
	// the virtual address(es) and advertising.
	StateMaster
)

// String returns the operator-facing state name.
func (s State) String() string {
	switch s {
	case StateInitialize:
		return "Initialize"
	case StateBackup:
		return "Backup"
	case StateMaster:
		return "Master"
	default:
		return "unknown"
	}
}

// Instance is one VRRP group's state machine (per interface, per family, per
// VRID). It is pure and single-threaded: only the owning worker goroutine calls
// Handle. It holds no locks, spawns no goroutines, and performs no I/O; the only
// use of the injected clock is Now() for snapshot timestamps.
type Instance struct {
	clock clock.Clock

	cfg   Config
	state State

	// activeAdverIntervalMs is the RFC Active_Adver_Interval in milliseconds.
	// v3 learns it from accepted adverts; v2 pins it to the local configured
	// interval forever.
	activeAdverIntervalMs int
	// masterHeardSinceBackup records whether an interval was adopted from a
	// master since entering Backup, driving the ConfigUpdated interval choice.
	masterHeardSinceBackup bool

	// genSeq is the monotonic generation counter. Each timer arm draws a fresh
	// value; the armed-gen fields below hold the generation each timer role is
	// currently waiting on (0 == not armed), so a stale expiry never matches.
	genSeq      uint64
	mdArmedGen  uint64
	advArmedGen uint64
	pdArmedGen  uint64

	// Snapshot bookkeeping.
	since         time.Time
	lastAdvertSrc netip.Addr
	lastAdvertAt  time.Time
}

// New creates an Instance in Initialize. The clock is used only for Now()
// timestamps; the FSM never schedules. Configuration arrives with the Startup
// event.
func New(clk clock.Clock) *Instance {
	return &Instance{clock: clk, state: StateInitialize, since: clk.Now()}
}

// State returns the current state.
func (i *Instance) State() State { return i.state }

// Handle evaluates one event against the current state and returns the ordered
// action slice the engine must execute. It mutates internal state but performs
// no side effects itself.
func (i *Instance) Handle(ev Event) []Action {
	switch i.state {
	case StateInitialize:
		return i.handleInitialize(ev)
	case StateBackup:
		return i.handleBackup(ev)
	case StateMaster:
		return i.handleMaster(ev)
	default:
		return nil
	}
}

// ---- Initialize ----

func (i *Instance) handleInitialize(ev Event) []Action {
	switch e := ev.(type) {
	case Startup:
		i.cfg = e.Config
		// RFC 9568 Section 5.2.4: priority 0 is wire-only ("Active Router
		// relinquishing"); it is never a configured local priority. Defensive
		// reject (spec A-5): stay down, emit nothing.
		if i.cfg.Priority == 0 {
			return nil
		}
		if i.isOwner() {
			return i.enterMasterFromInitialize()
		}
		return i.enterBackupFromInitialize()
	case ConfigUpdated:
		// Store the new config silently; nothing is running yet.
		i.cfg = e.Config
		return nil
	default:
		// RFC 9568 Section 6.4.1: Initialize acts only on Startup. A buffered rx
		// or a stale timer event during shutdown is a safe no-op, never a panic
		// (holo bug 10 negative test).
		return nil
	}
}

func (i *Instance) enterMasterFromInitialize() []Action {
	from := i.state
	i.activeAdverIntervalMs = i.cfg.AdvertIntervalMs
	i.setState(StateMaster)
	i.mdArmedGen = 0
	i.pdArmedGen = 0
	// RFC 9568 Section 6.4.1: owner sends an advertisement, announces (gratuitous
	// ARP / unsolicited NA), and starts its advert timer.
	out := []Action{
		SendAdvert{Priority: i.cfg.Priority, AdvertIntervalMs: i.cfg.AdvertIntervalMs},
		InstallVIPs{VIPs: i.cfg.VIPs},
		AnnounceFailover{},
	}
	out = i.armAdvert(out)
	out = append(out, EmitStateChange{From: from, To: StateMaster, Reason: ReasonStartupOwner})
	return out
}

func (i *Instance) enterBackupFromInitialize() []Action {
	from := i.state
	// RFC 9568 Section 6.4.1: Active_Adver_Interval = own Advertisement_Interval.
	i.activeAdverIntervalMs = i.cfg.AdvertIntervalMs
	i.masterHeardSinceBackup = false
	i.setState(StateBackup)
	i.advArmedGen = 0
	i.pdArmedGen = 0
	var out []Action
	out = i.armMasterDown(i.masterDown(), out)
	out = append(out, EmitStateChange{From: from, To: StateBackup, Reason: ReasonStartup})
	return out
}

// ---- Backup ----

func (i *Instance) handleBackup(ev Event) []Action {
	switch e := ev.(type) {
	case Shutdown:
		// RFC 9568 Section 6.4.2: cancel the down-timer, transition. No packet.
		from := i.state
		i.stopAllTimers()
		i.setState(StateInitialize)
		return []Action{StopTimers{}, EmitStateChange{From: from, To: StateInitialize, Reason: ReasonShutdown}}
	case MasterDownExpired:
		if !i.matches(i.mdArmedGen, e.Gen) {
			return nil
		}
		return i.promoteToMaster(ReasonMasterDownExpired)
	case PreemptDelayExpired:
		if !i.matches(i.pdArmedGen, e.Gen) {
			return nil
		}
		return i.promoteToMaster(ReasonPreemptDelayExpired)
	case AdvertReceived:
		return i.backupAdvert(e)
	case ConfigUpdated:
		return i.backupConfigUpdated(e)
	case Startup:
		return nil // already started; idempotent
	case AdvertTimerExpired:
		return nil // advert timer is a Master role; stale here
	default:
		return nil
	}
}

func (i *Instance) backupAdvert(e AdvertReceived) []Action {
	i.recordAdvert(e)

	// RFC 9568 Section 6.4.2 / RFC 3768 Section 6.4.2: Priority 0 means the Active
	// Router is releasing; set the down-timer to Skew_Time (from the CURRENT
	// Active_Adver_Interval and own priority) so we take over fast.
	if e.Priority == 0 {
		var out []Action
		out = i.stopDelayIfArmed(out)
		out = i.armMasterDown(i.skew(), out)
		return out
	}

	// RFC 9568 Section 6.4.2: accept a non-zero advert when Preempt is off or the
	// sender's priority is >= ours. MUST adopt the Max Advertise Interval (v3),
	// recompute, and reset the down-timer.
	if !i.cfg.Preempt || e.Priority >= i.cfg.Priority {
		var out []Action
		out = i.stopDelayIfArmed(out) // a rightful master is present
		i.adoptInterval(e.IntervalMs)
		i.masterHeardSinceBackup = true
		out = i.armMasterDown(i.masterDown(), out)
		return out
	}

	// Preempt is on and the sender's priority is lower than ours.
	if i.cfg.PreemptDelayMs == 0 {
		// RFC 9568 Section 6.4.2 MUST-discard: drop it; the down-timer keeps
		// running and its expiry promotes us (RFC-preempt takeover latency).
		return nil
	}

	// Preempt-delay (no RFC basis; Junos hold-time). While the delay holds we
	// behave as if Preempt were false: adopt + re-arm the down-timer so it never
	// fires mid-hold against a live master (spec risk R-3). The delay itself is
	// armed once, on the first sighting only.
	var out []Action
	if i.pdArmedGen == 0 {
		out = i.armPreemptDelay(out)
	}
	i.adoptInterval(e.IntervalMs)
	i.masterHeardSinceBackup = true
	out = i.armMasterDown(i.masterDown(), out)
	return out
}

func (i *Instance) backupConfigUpdated(e ConfigUpdated) []Action {
	i.cfg = e.Config
	// Keep the learned Active_Adver_Interval if a master has been heard since
	// entering Backup; otherwise fall back to the new configured interval.
	if !i.masterHeardSinceBackup {
		i.activeAdverIntervalMs = i.cfg.AdvertIntervalMs
	}
	var out []Action
	out = i.stopDelayIfArmed(out) // conditions changed; the next losing advert re-arms
	out = i.armMasterDown(i.masterDown(), out)
	return out
}

// ---- Master ----

func (i *Instance) handleMaster(ev Event) []Action {
	switch e := ev.(type) {
	case Shutdown:
		// RFC 9568 Section 6.4.3 / RFC 3768 Section 6.4.3: cancel the advert
		// timer, send a Priority-0 advertisement, transition to Initialize.
		from := i.state
		i.stopAllTimers()
		i.setState(StateInitialize)
		return []Action{
			StopTimers{},
			SendAdvertZeroPriority{},
			RemoveVIPs{VIPs: i.cfg.VIPs},
			EmitStateChange{From: from, To: StateInitialize, Reason: ReasonShutdown},
		}
	case AdvertTimerExpired:
		if !i.matches(i.advArmedGen, e.Gen) {
			return nil
		}
		// RFC 9568 Section 6.4.3: send an advertisement, reset the advert timer.
		var out []Action
		out = append(out, SendAdvert{Priority: i.cfg.Priority, AdvertIntervalMs: i.cfg.AdvertIntervalMs})
		out = i.armAdvert(out)
		return out
	case AdvertReceived:
		return i.masterAdvert(e)
	case ConfigUpdated:
		return i.masterConfigUpdated(e)
	case Startup:
		return nil // idempotent
	case MasterDownExpired, PreemptDelayExpired:
		return nil // Backup roles; stale here
	default:
		return nil
	}
}

func (i *Instance) masterAdvert(e AdvertReceived) []Action {
	i.recordAdvert(e)

	// RFC 9568 Section 6.4.3 / RFC 3768 Section 6.4.3: Priority 0 -> send an
	// advertisement and reset the advert timer (assert Master; peer is resigning).
	if e.Priority == 0 {
		var out []Action
		out = append(out, SendAdvert{Priority: i.cfg.Priority, AdvertIntervalMs: i.cfg.AdvertIntervalMs})
		out = i.armAdvert(out)
		return out
	}

	// RFC 9568 Section 6.4.3: a higher-priority advert, or an equal-priority
	// advert from a greater sender primary IP (unsigned network-byte-order
	// compare; 16-byte for IPv6), wins -- demote to Backup. The tie-break runs
	// ONLY in Master (see State Transition Table Notes).
	if e.Priority > i.cfg.Priority || (e.Priority == i.cfg.Priority && i.senderWinsTieBreak(e.SrcIP)) {
		reason := ReasonHigherPriority
		if e.Priority == i.cfg.Priority {
			reason = ReasonTieBreakLost
		}
		return i.demoteToBackup(e, reason)
	}

	// Losing advert (lower priority, or equal priority with smaller/equal sender
	// IP).
	if i.cfg.Version == 3 {
		// RFC 9568 Section 6.4.3 (new in RFC 9568): discard it and send an
		// advertisement immediately to assert the Active state. The advert timer
		// is NOT reset (contrast the priority-0 row).
		return []Action{SendAdvert{Priority: i.cfg.Priority, AdvertIntervalMs: i.cfg.AdvertIntervalMs}}
	}
	// RFC 3768 Section 6.4.3: v2 silently discards the losing advertisement.
	return nil
}

func (i *Instance) demoteToBackup(e AdvertReceived, reason string) []Action {
	from := i.state
	i.stopAllTimers()
	// RFC 9568 Section 6.4.3: adopt the winner's Max Advertise Interval (v3),
	// recompute, set the down-timer.
	i.adoptInterval(e.IntervalMs)
	i.masterHeardSinceBackup = true
	i.setState(StateBackup)
	out := []Action{StopTimers{}, RemoveVIPs{VIPs: i.cfg.VIPs}}
	out = i.armMasterDown(i.masterDown(), out)
	out = append(out, EmitStateChange{From: from, To: StateBackup, Reason: reason})
	return out
}

func (i *Instance) masterConfigUpdated(e ConfigUpdated) []Action {
	oldVIPs := i.cfg.VIPs
	i.cfg = e.Config
	// Master advertises at its own configured rate.
	i.activeAdverIntervalMs = i.cfg.AdvertIntervalMs
	var out []Action
	// Regenerate the advert from the NEW parameters (holo bug 8 negative test:
	// never a stale pre-encoded advert) and re-arm the advert timer.
	out = append(out, SendAdvert{Priority: i.cfg.Priority, AdvertIntervalMs: i.cfg.AdvertIntervalMs})
	out = i.armAdvert(out)
	if !sameAddrs(oldVIPs, i.cfg.VIPs) {
		out = append(out, InstallVIPs{VIPs: i.cfg.VIPs}, AnnounceFailover{})
	}
	return out
}

// promoteToMaster is shared by the master-down and preempt-delay expiry rows.
//
// RFC 9568 Section 6.4.2 / RFC 3768 Section 6.4.2: send an advertisement,
// announce, start the advert timer, transition to Master.
func (i *Instance) promoteToMaster(reason string) []Action {
	from := i.state
	// Own configured interval drives the advert timer; the learned value becomes
	// irrelevant until the next demotion.
	i.activeAdverIntervalMs = i.cfg.AdvertIntervalMs
	i.setState(StateMaster)
	i.mdArmedGen = 0
	i.pdArmedGen = 0
	out := []Action{
		SendAdvert{Priority: i.cfg.Priority, AdvertIntervalMs: i.cfg.AdvertIntervalMs},
		InstallVIPs{VIPs: i.cfg.VIPs},
		AnnounceFailover{},
	}
	out = i.armAdvert(out)
	out = append(out, EmitStateChange{From: from, To: StateMaster, Reason: reason})
	return out
}

// ---- Snapshot ----

// Snapshot is the state view surfaced by `show vrrp` (spec-vrrp-5). Deadlines are
// owned by the engine's clock.Timer set; this reports which timer roles the FSM
// currently has armed.
type Snapshot struct {
	State                State      `json:"state"`
	Since                time.Time  `json:"since"`
	Version              uint8      `json:"version"`
	Priority             uint8      `json:"priority"`
	IsOwner              bool       `json:"is-owner"`
	Preempt              bool       `json:"preempt"`
	AcceptMode           bool       `json:"accept-mode"`
	ConfiguredIntervalMs int        `json:"configured-interval-ms"`
	ActiveIntervalMs     int        `json:"active-interval-ms"`
	LastAdvertSrc        netip.Addr `json:"last-advert-src"`
	LastAdvertAt         time.Time  `json:"last-advert-at"`
	MasterDownArmed      bool       `json:"master-down-armed"`
	AdvertArmed          bool       `json:"advert-armed"`
	PreemptDelayArmed    bool       `json:"preempt-delay-armed"`

	// SkewTime and MasterDownInterval are the DERIVED timers, exported here so
	// the operator surface reports the values this FSM actually uses. Computed
	// by the same skew()/masterDown() the state machine arms its timers with:
	// a second formula in the show path would be free to disagree with the one
	// that decides failovers, and the disagreement would be invisible.
	//
	// time.Duration, not milliseconds: a valid VRRPv3 skew is sub-millisecond
	// (78.125us at priority 254 with a 10ms interval), so a millisecond field
	// would render the most interesting values as 0.
	SkewTime           time.Duration `json:"skew-time"`
	MasterDownInterval time.Duration `json:"master-down-interval"`
}

// Snapshot returns the current instance state view.
func (i *Instance) Snapshot() Snapshot {
	return Snapshot{
		State:                i.state,
		Since:                i.since,
		Version:              i.cfg.Version,
		Priority:             i.cfg.Priority,
		IsOwner:              i.cfg.IsOwner,
		Preempt:              i.cfg.Preempt,
		AcceptMode:           i.cfg.AcceptMode,
		ConfiguredIntervalMs: i.cfg.AdvertIntervalMs,
		ActiveIntervalMs:     i.activeAdverIntervalMs,
		LastAdvertSrc:        i.lastAdvertSrc,
		LastAdvertAt:         i.lastAdvertAt,
		MasterDownArmed:      i.mdArmedGen != 0,
		AdvertArmed:          i.advArmedGen != 0,
		PreemptDelayArmed:    i.pdArmedGen != 0,
		SkewTime:             i.skew(),
		MasterDownInterval:   i.masterDown(),
	}
}

// ---- internal helpers ----

func (i *Instance) setState(s State) {
	i.state = s
	i.since = i.clock.Now()
}

func (i *Instance) recordAdvert(e AdvertReceived) {
	i.lastAdvertSrc = e.SrcIP
	i.lastAdvertAt = i.clock.Now()
}

func (i *Instance) isOwner() bool { return i.cfg.IsOwner || i.cfg.Priority == 255 }

// senderWinsTieBreak reports whether the sender's primary IP is greater than the
// local primary IP, compared as unsigned integers in network byte order (16-byte
// for IPv6). Unmap normalizes any v4-in-v6 form so both operands compare in the
// same representation. Equal addresses are NOT greater, so a duplicated primary
// IP (misconfiguration) never demotes a healthy Master.
func (i *Instance) senderWinsTieBreak(src netip.Addr) bool {
	return src.Unmap().Compare(i.cfg.LocalPrimaryIP.Unmap()) > 0
}

// adoptInterval learns the sender's interval in v3; v2 pins the interval to the
// local configured value forever (no adoption; a mismatch is discarded upstream).
func (i *Instance) adoptInterval(ms int) {
	if i.cfg.Version == 3 {
		i.activeAdverIntervalMs = ms
	}
}

func (i *Instance) advInterval() time.Duration {
	return time.Duration(i.cfg.AdvertIntervalMs) * time.Millisecond
}

func (i *Instance) masterDown() time.Duration {
	return masterDownInterval(i.cfg.Version, i.cfg.Priority, i.activeAdverIntervalMs)
}

func (i *Instance) skew() time.Duration {
	return skewTime(i.cfg.Version, i.cfg.Priority, i.activeAdverIntervalMs)
}

// nextGen draws a fresh monotonic generation for a timer arm.
func (i *Instance) nextGen() uint64 {
	i.genSeq++
	return i.genSeq
}

// matches reports whether an expiry event's generation is the one currently
// armed for that role (and the role is armed at all).
func (i *Instance) matches(armedGen, eventGen uint64) bool {
	return armedGen != 0 && eventGen == armedGen
}

func (i *Instance) armMasterDown(d time.Duration, out []Action) []Action {
	g := i.nextGen()
	i.mdArmedGen = g
	return append(out, StartMasterDownTimer{Duration: d, Gen: g})
}

func (i *Instance) armAdvert(out []Action) []Action {
	g := i.nextGen()
	i.advArmedGen = g
	return append(out, StartAdvertTimer{Interval: i.advInterval(), Gen: g})
}

func (i *Instance) armPreemptDelay(out []Action) []Action {
	g := i.nextGen()
	i.pdArmedGen = g
	d := time.Duration(i.cfg.PreemptDelayMs) * time.Millisecond
	return append(out, StartPreemptDelayTimer{Duration: d, Gen: g})
}

// stopDelayIfArmed emits StopPreemptDelayTimer and invalidates the role's gen
// only when the delay is currently armed.
func (i *Instance) stopDelayIfArmed(out []Action) []Action {
	if i.pdArmedGen == 0 {
		return out
	}
	i.pdArmedGen = 0
	return append(out, StopPreemptDelayTimer{})
}

// stopAllTimers invalidates every timer role's generation so any late fire is
// ignored. It is the internal counterpart of the StopTimers action.
func (i *Instance) stopAllTimers() {
	i.mdArmedGen = 0
	i.advArmedGen = 0
	i.pdArmedGen = 0
}

// sameAddrs reports whether two virtual-address sets are element-wise equal
// (order-sensitive, matching the configured order).
func sameAddrs(a, b []netip.Addr) bool {
	if len(a) != len(b) {
		return false
	}
	for idx := range a {
		if a[idx] != b[idx] {
			return false
		}
	}
	return true
}
