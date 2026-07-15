# Spec: vrrp-2 -- VRRP Instance State Machine and Timers

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-vrrp-1 |
| Phase | 1/9 |
| Updated | 2026-07-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `plan/spec-vrrp-0-umbrella.md` -- user decisions, sibling scopes, risks R-1/R-2/R-4
3. `rfc/short/rfc9568.md` (State Machine, Timers, Algorithms sections) and `rfc/short/rfc3768.md` (same sections) -- the ONLY safe source for FSM semantics (both reference implementations are wrong on v3)
4. `internal/core/clock/clock.go` -- injectable Clock/Timer/Ticker abstractions
5. `.claude/rules/planning.md` -- workflow rules

## Task

Design and implement the per-instance VRRP state machine and timer arithmetic as a
pure, deterministic Go package `internal/plugins/vrrp/fsm`: no sockets, no netlink,
no direct clock syscalls. One FSM instance per configured VRRP group (per interface,
per family, per VRID).

The FSM is a pure event-driven machine:

- **Inputs** = typed events (Startup, Shutdown, AdvertReceived, MasterDownExpired,
  AdvertTimerExpired, PreemptDelayExpired, ConfigUpdated) plus the current time read
  from an injected `clock.Clock` (timestamps only, never scheduling).
- **Outputs** = an ordered slice of action values (SendAdvert, SendAdvertZeroPriority,
  InstallVIPs, RemoveVIPs, AnnounceFailover, StartMasterDownTimer, StartAdvertTimer,
  StartPreemptDelayTimer, StopPreemptDelayTimer, StopTimers, EmitStateChange).

Actions-as-values is the key design decision: the FSM is table-testable without any
mocks (inspired by holo-vrrp's jsonl conformance corpus, see
`research-holo-digest`), and the plugin engine (spec-vrrp-5) is the sole executor of
actions (sockets via spec-vrrp-4, VIPs via spec-vrrp-3/iface, timers via
`internal/core/clock`).

Covers RFC 9568 (VRRPv3, default) and RFC 3768 (VRRPv2, opt-in) semantics:
Initialize/Backup/Master states, owner (priority 255) startup, skew and master-down
arithmetic, priority-0 handling, preemption with an optional vendor-style
preempt-delay (Junos hold-time semantics; no RFC support -- documented decision),
Master-state IP tie-break, v3 interval adoption, and safe handling of events that
the reference implementations mishandle (holo panics on packet-in-Initialize;
uvrrpd/holo truncate skew to zero).

Terminology: RFC 9568 renamed "Master" to "Active Router". Ze keeps
Initialize/Backup/Master because keepalived, Junos, and the umbrella's agreed CLI
surface (`show vrrp`) all speak Master/Backup. RFC citations below map Master ==
Active.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `plan/spec-vrrp-0-umbrella.md` - parent umbrella: user decisions, child scopes
  → Decision: internal unit for all intervals is MILLISECONDS; conversion to wire units (v2 seconds, v3 centiseconds) happens ONLY in the codec (spec-vrrp-1). Kills risk R-2 (s/cs/ms confusion, holo's 100x bug).
  → Constraint: risk R-1 -- FSM semantics come from `rfc/short/` only; every verified holo/uvrrpd bug becomes an explicit negative test here.
  → Constraint: risk R-4 -- macvlan is pre-created at group create (spec-vrrp-3/5); only VIP install/remove rides the Master/Backup transition, so InstallVIPs/RemoveVIPs are the only dataplane actions the FSM emits per transition.
- [ ] `docs/architecture/behavior/fsm.md` - the house FSM style (BGP)
  → Constraint: typed State and Event constants with String() methods and an RFC section comment on every constant (`internal/component/bgp/fsm/state.go:26-56` pattern); goroutine-per-lifecycle with a switch on state.
- [ ] `ai/rules/goroutine-lifecycle.md` - worker model
  → Constraint: one long-lived goroutine per instance (lifecycle-scoped is OK); events arrive on channels; timers are dedicated cancellable objects from the injected clock; never a goroutine per event.
- [ ] `ai/rules/spec-no-code.md` + `ai/rules/planning.md` - spec format
  → Constraint: state transition tables instead of code; Risks & Assumptions live tables; append-only editing.
- [ ] `ai/rules/module-tiers.md` - package placement
  → Decision: `internal/plugins/vrrp/fsm` is a leaf library under the vrrp plugin (edge tier); it imports only `internal/core/clock` and stdlib (`net/netip`, `time`). Nothing outside the vrrp plugin may import it.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9568.md` - VRRPv3: State Machine, Timers, Algorithms, MUST Requirements (State/Timers)
  → Constraint: Skew_Time = ((256 - Priority) * Active_Adver_Interval) / 256; Active_Down_Interval = 3 * Active_Adver_Interval + Skew_Time (§6.1 via Timers table).
  → Constraint: Backup MUST adopt the sender's Max Advertise Interval as Active_Adver_Interval on every accepted non-zero-priority advertisement, recomputing skew and down interval (§6.4.2).
  → Constraint: Master receiving a LOSING advertisement (lower priority, or equal priority with smaller sender address) MUST discard it AND send an ADVERTISEMENT immediately to assert the Master state; the summary's state-machine row does NOT reset Adver_Timer for this case (contrast the priority-0 row, which does) (§6.4.3, new in RFC 9568).
  → Constraint: sender-IP tie-break exists ONLY in the Master state; a Backup treats an equal-priority advertisement as authoritative regardless of sender address (State Machine "Notes").
  → Constraint: Preempt_Mode is consulted only in Backup; the owner (priority 255) always preempts independent of the flag (§6.1, §6.4.3 note).
  → Constraint: no Preempt_Delay exists in the RFC ("delayed preemption is a common vendor extension") -- ours is a documented decision, isolated and default-off.
- [ ] `rfc/short/rfc3768.md` - VRRPv2: State Machine, Timers, Algorithms, MUST Requirements (State/Timers)
  → Constraint: v2 Skew_Time = (256 - Priority) / 256 SECONDS, independent of the advertisement interval; must never integer-truncate to zero (uvrrpd bug).
  → Constraint: v2 has NO interval adoption; a mismatched Adver Int is discarded at receive validation (§7.1), so the FSM never sees it -- Backups always time out on their own configured interval.
  → Constraint: v2 Master silently discards a losing advertisement; there is no immediate re-advertisement (contrast v3).

**Key insights:**
- Both studied implementations (holo-vrrp, uvrrpd) get v3 interval adoption and skew arithmetic wrong; the `rfc/short/` state-machine and timer tables are the sole source of truth for every transition row below.
- Valid v3 skews are sub-millisecond (priority 254, interval 10 ms → 78.125 us), so computed durations must be `time.Duration` (nanoseconds) with division LAST; integer-millisecond duration fields would reintroduce the truncate-to-zero bug class this child exists to kill.
- The FSM never owns a timer: it emits Start/Stop timer actions and consumes expiry events. Determinism falls out for free and the engine (spec-vrrp-5) is the single place where `clock.Timer` lives.

## Current Behavior (MANDATORY)

**Source files read:** (producers read directly this session)
- [ ] `internal/core/clock/clock.go` - injectable time abstraction. `Clock` interface :18 (Now :20, Sleep :23, After :26, AfterFunc :32, NewTimer :36, NewTicker :41); `Ticker` :48 (Stop/C); `Timer` :61 (Stop :65, Reset :69, C :73); `RealClock` :78 wraps stdlib with zero overhead. This is the ONLY time source the vrrp engine may use.
- [ ] `internal/test/sim/sim.go` - `FakeClock` :25, deterministic test clock (fires timers on demand); referenced by clock.go's Ticker doc comment.
- [ ] `internal/chaos/virtualclock.go` - `VirtualClock` :26, chaos-side virtual time (Advance); same Clock interface.
- [ ] `internal/component/bgp/fsm/state.go` - house FSM style: `State` bit-flag constants :26-56 with per-constant RFC comments, `Event` iota constants :90-153, String() via name maps + textbuf :68,:174.
- [ ] `internal/component/bgp/fsm/timer.go` - `Timers` struct :58 holds a `clock clock.Clock` field :63 injected via SetClock :96; NewTimers :87 defaults to `clock.RealClock{}`; per-timer callback model. Precedent for clock injection, though vrrp inverts it: the FSM emits timer actions instead of owning timers, because the FSM must be pure.
- [ ] `internal/plugins/ospf/register.go` - closest engine model: OnConfigApply :442 reconciles instances against new config (instances.reconcile :450); OnStarted :455 stands up per-instance engines (instances.start :494) that subscribe to interface events and own their transports; OnExecuteCommand :505 serves show commands from engine snapshots. The vrrp engine (spec-vrrp-5) starts one instance worker goroutine per group the same way; this child defines what that worker calls.

**Behavior to preserve:** (unless user explicitly said to change)
- `internal/core/clock` is consumed as-is; no changes to the Clock/Timer/Ticker interfaces.
- `internal/component/bgp/fsm` is untouched; it is a style reference only.
- No existing vrrp behavior exists (grep: only the firewall protocol map knows "vrrp"); this package is net-new and nothing outside `internal/plugins/vrrp/` may import it.

**Behavior to change:**
- None removed. New: `internal/plugins/vrrp/fsm` package (states, events, actions, transition logic, timer math).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Typed events delivered by the owning instance-worker goroutine (spec-vrrp-5), one FSM per group:
  - `Startup` / `Shutdown` / `ConfigUpdated`: from config commit/apply/removal and parent-link readiness (engine).
  - `AdvertReceived`: produced by the ENGINE (spec-vrrp-5), which calls packet.Decode + receive validation (spec-vrrp-1) on the raw (RxMeta, payload) pairs delivered by the transport (spec-vrrp-4) and owns the group table/Lookup; carries only decoded, validated fields (priority, source IP, interval in ms, VIP count).
  - `MasterDownExpired` / `AdvertTimerExpired` / `PreemptDelayExpired`: from the engine's `clock.Timer` channels, echoing the generation of the arming action (see Timer Generations below).
- Format at entry: Go values (no wire bytes reach the FSM; the codec boundary is spec-vrrp-1).

### Transformation Path
1. Instance worker selects on its channels (rx events, timer channels, config/control) and calls the FSM's single synchronous handle method with one event.
2. FSM evaluates the transition table row for (state, event, guards), mutates internal state (state enum, Active_Adver_Interval, timer generations, timestamps via injected `clock.Now()`), and returns the ordered action slice.
3. Worker executes actions in order: packet sends → transport (spec-vrrp-4); VIP install/remove → iface address-owner registry via engine (spec-vrrp-3/5); failover announce (GARP/NA burst) → transport (spec-vrrp-4); timer arms/cancels → the worker's own `clock.Timer` set; EmitStateChange → eventbus "vrrp" namespace + metrics + log (spec-vrrp-5).
4. Timer expiry lands back on the worker's select loop and re-enters step 1 as an expiry event carrying its generation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| transport (spec-vrrp-4) → engine (spec-vrrp-5) → FSM | transport delivers raw (RxMeta, payload) only; the engine runs packet.Decode + validation (spec-vrrp-1) and its group Lookup, then builds AdvertReceived; malformed/failed-validation packets never become events | [ ] |
| engine goroutine ↔ FSM | single-threaded: only the owning worker calls the FSM; no locks inside the FSM | [ ] |
| FSM → engine | ordered action values returned; FSM performs no I/O and holds no channels | [ ] |
| engine ↔ clock | `clock.Clock` injected at instance creation; `RealClock{}` in production, `sim.FakeClock` in tests | [ ] |
| FSM ↔ clock | `Now()` only (state-entry timestamps, last-advert-seen for the show snapshot); the FSM never schedules | [ ] |

### Integration Points
- `internal/core/clock/clock.go:18` `Clock` - injected into both the FSM (timestamps) and the instance worker (timer scheduling for emitted Start*/Stop* actions)
- `internal/test/sim/sim.go:25` `FakeClock` - drives deterministic scenario tests
- `internal/plugins/ospf/register.go:455,:494` OnStarted/instances.start - the worker-ownership model the vrrp engine copies in spec-vrrp-5
- spec-vrrp-1 `packet` package - Decode/validation called by the engine (spec-vrrp-5) to produce the fields that become AdvertReceived; owns v2 interval-mismatch discard (see Version Behavior table)
- spec-vrrp-5 engine - sole caller of the FSM and sole executor of actions

### Architectural Verification
- [ ] No bypassed layers (FSM never touches sockets, netlink, or `time.` directly; all side effects are actions executed by the engine)
- [ ] No unintended coupling (fsm package imports only `internal/core/clock`, `net/netip`, `time`, stdlib; no sibling plugin, no iface, no sdk imports)
- [ ] No duplicated functionality (reuses `internal/core/clock`; does not reimplement timer plumbing that `clock.Timer` provides)
- [ ] Zero-copy preserved where applicable (events and actions are small value types; no per-event allocations beyond the returned action slice, which the worker reuses via an actions buffer passed in)
- [ ] Registration over hardcoding -- this package registers nothing and nothing central learns the string "vrrp" from it; state-change actions surface on the bus only through the engine's registered "vrrp" event namespace (spec-vrrp-5), per `ai/rules/plugin-self-containment.md`

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | v2 interval-mismatch adverts are discarded before the FSM (receive validation, spec-vrrp-1), so the v2 FSM never needs mismatch handling | `rfc/short/rfc3768.md` §7.1 (MUST discard on Adver Int mismatch is a receive check); umbrella child-1 scope "codec + receive validation" | Add a v2 guard row: mismatched interval event → discard, no re-arm | spec-vrrp-1's validation table includes the local-interval parameter; cross-checked at umbrella review | validated -- FSM has no v2 mismatch row; `TestV2NoIntervalAdoption` proves v2 never adopts a sender interval, so relying on upstream discard is correct |
| A-2 | `clock.Timer.Reset` semantics (stale fire already queued in the channel after Reset) are possible in production, so expiry events need a staleness guard | Go `time.Timer` documented Reset race; `internal/core/clock/clock.go:69` Reset delegates to `time.Timer.Reset` | Generation matching is dead weight (kept anyway: it is cheap and makes scenario tests exact) | `TestFSMStaleTimerGenerationIgnored` (unit) | validated -- generation guard implemented (genSeq + per-role armedGen); stale/unarmed expiries return no actions in every state |
| A-3 | An actions-as-values FSM covers every side effect the engine needs (no callback escape hatch required) | holo-vrrp conformance harness proves event→expected-output vectors suffice (`research-holo-digest` item: jsonl corpus); action list below audited against every RFC state-machine row | Add an action variant; the action list is an enum-like closed set owned by this package | spec-vrrp-5 wiring phase compiles the executor against the full action set | validated -- the 11-action closed set covers every transition-table row; no callback escape hatch was needed |
| A-4 | `sim.FakeClock` (`internal/test/sim/sim.go:25`) is usable from `internal/plugins/vrrp/fsm` tests (import direction legal for an edge-tier test) | isis/ospf plugin tests import core/test helpers; module-tiers allows edge → core/test | Tests define a local 10-line fake clock instead | first compile of `fsm_test.go` | validated -- fsm tests import `internal/test/sim` and pass under `go test -race` |
| A-5 | Priority-0 is never a configurable local priority (0 is wire-only, "Master releasing"); Startup always carries priority 1..254 or 255 (owner) | `rfc/short/rfc9568.md` Constants (release = 0; backup 1-254; owner 255); umbrella YANG range priority 1..254 + owner auto-255 | FSM adds a defensive reject of Startup with priority 0 (returns no actions, stays Initialize) -- added anyway as a negative test | `TestFSMStartupRejectsPriorityZero` | validated -- `handleInitialize` rejects priority 0, stays Initialize, emits nothing |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Copying reference-implementation FSM semantics smuggles in their v3 bugs (umbrella R-1) | A transition test written from `rfc/short/` disagrees with the implementation | Every transition row below cites the RFC summary section; the reference-bug list is a mandatory negative-test suite (TDD plan) |
| R-2 | Unit confusion between config ms, v2 wire seconds, v3 wire centiseconds (umbrella R-2) | Boundary tests assert literal nanosecond values; any conversion inside the FSM fails them | FSM sees ONLY milliseconds in events and emits ONLY `time.Duration`; wire conversion is locked in spec-vrrp-1 |
| R-3 | Preempt-delay (no RFC text) interacts badly with master-down expiry (promotes during the hold, defeating the delay) | Scenario test: delay armed, master keeps advertising, master-down must not fire | While the delay is armed, losing adverts re-arm master-down (behave as Preempt false); precise rules in the Preempt-Delay section |
| R-4 | Action ordering bugs (e.g., AnnounceFailover before InstallVIPs) pass unit tests that only assert set membership | Order-sensitive assertions in the transition-matrix tests | Every AC that lists actions asserts the exact ordered slice, not membership |
| R-5 | ConfigUpdated races the pre-encoded advert (holo bug: stale advert after priority/interval reconfig) | `TestFSMConfigUpdateRegeneratesAdvert` fails | SendAdvert carries priority+interval parameters on every emission; the engine builds the packet from the action's fields, never from a cached buffer |

## Preempt-Delay Semantics (decision -- no RFC basis)

`preempt-delay-seconds` (umbrella leaf, default 0 = disabled) follows Junos
`preempt hold-time` semantics: a higher-priority Backup holds for the configured
delay before taking over from a LIVE lower-priority Master. It never delays
failover to a dead Master.

| Rule | Behavior |
|------|----------|
| Arm | In Backup, with Preempt true, delay > 0, and the delay timer not armed: the FIRST advert with 0 < adv.priority < local priority arms StartPreemptDelayTimer{delay}. Subsequent losing adverts do NOT re-arm it (the hold measures from first sighting). |
| During the hold | Losing adverts are treated as if Preempt were false: (v3) adopt the sender's interval, recompute skew/master-down, re-arm master-down. This keeps master-down from firing mid-hold when the Master is alive (risk R-3). |
| Expiry | PreemptDelayExpired in Backup promotes exactly like MasterDownExpired (same ordered action sequence, reason "preempt-delay-expired"). |
| Cancel: rightful master | An advert with adv.priority >= local priority cancels the delay (StopPreemptDelayTimer) and re-arms master-down normally -- there is nothing to preempt. |
| Cancel: master resigns | A priority-0 advert cancels the delay and sets master-down := Skew_Time (normal RFC handling); promotion then happens via the fast prio-0 path -- resignation is not preemption. |
| Cancel: state exit | Any transition out of Backup emits StopTimers, which cancels all three timers including the delay. |
| Cancel: reconfig | ConfigUpdated in Backup always cancels an armed delay (conditions changed; the next losing advert re-arms under the new config). |
| Disabled (delay == 0, default) | Pure RFC behavior: losing adverts are discarded with no actions; master-down keeps running from its last re-arm and its expiry promotes (RFC-preempt takeover latency = Active_Down_Interval). |

## State Transition Table

States: Initialize, Backup, Master. One FSM instance per group. Guards are
evaluated top-down within a (State, Event) block; the first matching row wins.
Actions are ORDERED; the order is part of the contract.

Sources: `rfc/short/rfc9568.md` State Machine table + §6.4.x MUST rows;
`rfc/short/rfc3768.md` State Machine table. Version-divergent cells are marked
(v3)/(v2) and consolidated in the Version Behavior table below.

| State | Event | Guard | Actions (ordered) | Next |
|-------|-------|-------|-------------------|------|
| Initialize | Startup | IsOwner (priority 255) | SendAdvert; InstallVIPs; AnnounceFailover; StartAdvertTimer{interval}; EmitStateChange{Initialize→Master, "startup-owner"} | Master |
| Initialize | Startup | not owner (priority 1..254) | set Active_Adver_Interval := own configured interval; StartMasterDownTimer{Master_Down_Interval}; EmitStateChange{Initialize→Backup, "startup"} | Backup |
| Initialize | Startup | priority == 0 (defensive, A-5) | none (log-only via engine; stays down) | Initialize |
| Initialize | Shutdown | - | none | Initialize |
| Initialize | AdvertReceived / MasterDownExpired / AdvertTimerExpired / PreemptDelayExpired | - | none -- ignored safely, never panics (holo bug 10 negative test: a buffered rx or stale timer event during shutdown must be a no-op) | Initialize |
| Initialize | ConfigUpdated | - | store the new config; no actions | Initialize |
| Backup | Shutdown | - | StopTimers; EmitStateChange{Backup→Initialize, "shutdown"} -- NO packet (rfc9568 §6.4.2 / rfc3768 §6.4.2: cancel timer, transition) | Initialize |
| Backup | MasterDownExpired | generation matches | SendAdvert; InstallVIPs; AnnounceFailover; StartAdvertTimer{interval}; EmitStateChange{Backup→Master, "master-down-expired"} (rfc9568 §6.4.2 / rfc3768 §6.4.2) | Master |
| Backup | PreemptDelayExpired | generation matches | SendAdvert; InstallVIPs; AnnounceFailover; StartAdvertTimer{interval}; EmitStateChange{Backup→Master, "preempt-delay-expired"} | Master |
| Backup | AdvertReceived | adv.priority == 0 | StopPreemptDelayTimer (if armed); StartMasterDownTimer{Skew_Time} -- skew from CURRENT Active_Adver_Interval and own priority (rfc9568 §6.4.2 / rfc3768 §6.4.2) | Backup |
| Backup | AdvertReceived | adv.priority > 0 AND (Preempt == false OR adv.priority >= local priority) | (v3) Active_Adver_Interval := adv.interval, recompute Skew_Time + Master_Down_Interval; (v2) intervals fixed; StopPreemptDelayTimer (if armed -- a rightful master is present); StartMasterDownTimer{Master_Down_Interval} (rfc9568 §6.4.2 MUST-adopt / rfc3768 §6.4.2) | Backup |
| Backup | AdvertReceived | adv.priority > 0 AND Preempt AND adv.priority < local AND delay == 0 | none -- discard; master-down keeps running (rfc9568 §6.4.2 / rfc3768 §6.4.2 MUST-discard) | Backup |
| Backup | AdvertReceived | adv.priority > 0 AND Preempt AND adv.priority < local AND delay > 0 AND delay not armed | StartPreemptDelayTimer{delay}; (v3) adopt interval + recompute; StartMasterDownTimer{Master_Down_Interval} (hold rules above) | Backup |
| Backup | AdvertReceived | adv.priority > 0 AND Preempt AND adv.priority < local AND delay armed | (v3) adopt interval + recompute; StartMasterDownTimer{Master_Down_Interval} -- delay NOT re-armed | Backup |
| Backup | AdvertTimerExpired | - | none (stale timer; ignored) | Backup |
| Backup | ConfigUpdated | - | store config; recompute Skew_Time/Master_Down_Interval from the new priority (Active_Adver_Interval: keep the learned value if a master has been heard since entering Backup, else the new configured interval); StopPreemptDelayTimer (if armed); StartMasterDownTimer{new Master_Down_Interval} | Backup |
| Backup | Startup | - | none (already started; idempotent) | Backup |
| Master | Shutdown | - | StopTimers; SendAdvertZeroPriority; RemoveVIPs; EmitStateChange{Master→Initialize, "shutdown"} (rfc9568 §6.4.3 / rfc3768 §6.4.3: MUST send priority-0 advert before Initialize) | Initialize |
| Master | AdvertTimerExpired | generation matches | SendAdvert; StartAdvertTimer{interval} (rfc9568 §6.4.3 / rfc3768 §6.4.3) | Master |
| Master | AdvertReceived | adv.priority == 0 | SendAdvert; StartAdvertTimer{interval} -- reset (rfc9568 §6.4.3 / rfc3768 §6.4.3) | Master |
| Master | AdvertReceived | adv.priority > local, OR adv.priority == local AND adv.srcIP > local primary IP (unsigned, network byte order; 16-byte compare for IPv6) | StopTimers; (v3) Active_Adver_Interval := adv.interval, recompute; RemoveVIPs; StartMasterDownTimer{Master_Down_Interval}; EmitStateChange{Master→Backup, "higher-priority" or "tie-break-lost"} (rfc9568 §6.4.3 / rfc3768 §6.4.3) | Backup |
| Master | AdvertReceived | losing advert: adv.priority < local, OR adv.priority == local AND adv.srcIP <= local primary IP | (v3) SendAdvert immediately -- assert Master, advert timer NOT reset (rfc9568 §6.4.3, summary state-machine row); (v2) none -- silent discard (rfc3768 §6.4.3) | Master |
| Master | MasterDownExpired / PreemptDelayExpired | - | none (stale timer; ignored) | Master |
| Master | ConfigUpdated | - | store config; SendAdvert{new priority, new interval}; StartAdvertTimer{new interval}; if the VIP set changed: InstallVIPs{new full set} + AnnounceFailover (engine reconciles installs idempotently). Regeneration-from-parameters is the anti-stale-advert rule (holo bug 8 negative test) | Master |
| Master | Startup | - | none (idempotent) | Master |

Notes (RFC asymmetries, flagged deliberately):

- **Tie-break only in Master.** The sender-IP comparison never runs in Backup: two
  equal-priority Backups both promote on their (identically skewed) master-down
  expiry, and the conflict resolves in Master state when each hears the other
  (rfc9568 State Machine Notes; rfc3768 Pitfalls "Tie-break only in Master state").
  The transition matrix has an explicit test proving Backup ignores sender IP.
- **adv.srcIP == local primary IP** with equal priority is classified as a losing
  advert (not greater). Multicast loopback is disabled on the tx socket
  (spec-vrrp-4), so this only occurs on misconfiguration (duplicated primary IP);
  it must not demote the Master.
- **Owner preemption needs no special row.** An owner in Backup (possible only after
  losing a 255-vs-255 tie-break) has priority 255, so every incoming advert matches
  either the ">= local" row (another 255) or the discard row; the FSM forces
  Preempt semantics for the owner by construction of the guards (rfc9568 §6.1).

## Timer Math (timers.go)

Unit discipline (risk R-2): every interval crossing the FSM boundary is an integer
MILLISECOND count. Every computed duration is a `time.Duration` (int64
nanoseconds). Multiplication ALWAYS precedes division; division is the last
operation. Rationale: valid v3 skews are sub-millisecond (below), so
integer-millisecond duration fields cannot represent them -- the exact bug class
of uvrrpd (v2 skew truncated to 0 whole seconds) and holo (v3 skew int-cast to 0).

| Quantity | Version | Formula (input ms → output time.Duration) |
|----------|---------|-------------------------------------------|
| Skew_Time | v3 | ((256 - priority) * ActiveAdverInterval) / 256, computed on the nanosecond Duration (rfc9568 Algorithms) |
| Skew_Time | v2 | ((256 - priority) * 1 second) / 256, interval-independent (rfc3768 Algorithms) |
| Master_Down_Interval | v3 | 3 * ActiveAdverInterval + Skew_Time (rfc9568 Algorithms) |
| Master_Down_Interval | v2 | 3 * own configured interval + Skew_Time (rfc3768 Algorithms; no learned interval exists in v2) |
| Advert timer | v2 + v3 | own configured interval (the Master always advertises at its own rate) |
| Preempt delay | v2 + v3 | configured preempt-delay-seconds * 1000 ms (0 = disabled) |

Active_Adver_Interval lifecycle (v3 only; v2 pins it to the local configured
interval forever):

| Moment | Value |
|--------|-------|
| Startup → Backup | own configured interval (rfc9568 §6.4.1) |
| Accepted non-zero-priority advert in Backup (incl. during preempt hold) | sender's interval (MUST adopt, rfc9568 §6.4.2) |
| Master → Backup demotion | winning sender's interval (rfc9568 §6.4.3) |
| ConfigUpdated in Backup | keep learned value if any master heard since entering Backup; else the new configured interval |
| any → Master | own configured interval drives the advert timer; the learned value becomes irrelevant until the next demotion |

## Version Behavior Table

All v2/v3 divergence lives in this table (and only here); the FSM core is
version-agnostic everywhere else. `Version` is fixed per instance at Startup.

| Behavior | v2 (RFC 3768) | v3 (RFC 9568) |
|----------|---------------|----------------|
| Wire interval units | 8-bit whole seconds (codec converts, spec-vrrp-1) | 12-bit centiseconds (codec converts, spec-vrrp-1) |
| Config interval range (ms) | 1000..255000, whole seconds | 10..40950 |
| Interval mismatch on rx | Discarded by receive validation (spec-vrrp-1, needs local interval as input); the FSM never sees the event (A-1) | Not a discard: FSM adopts the sender's interval as Active_Adver_Interval on every accepted advert |
| Skew_Time | (256 - priority) / 256 seconds, interval-independent | ((256 - priority) * Active_Adver_Interval) / 256 |
| Master on losing advert | Silent discard, no packet | Discard + immediate SendAdvert (assert Master); advert timer not reset |
| Accept-mode | Not defined | Defined, default false; FSM stores it for the state snapshot only (see Known Limitations) |
| Address families | IPv4 only | IPv4 and IPv6 (16-byte tie-break compare) |
| Priority-0 handling, owner rules, preempt rules, shutdown rules | identical | identical |

## Timer Generations (staleness guard)

`clock.Timer.Reset` (`internal/core/clock/clock.go:69`, delegating to
`time.Timer.Reset`) can leave an already-fired tick queued in the channel; the
worker would deliver a stale expiry after a re-arm. Deterministic guard:

| Rule | Detail |
|------|--------|
| Arm | Every StartMasterDownTimer / StartAdvertTimer / StartPreemptDelayTimer action carries a Gen (uint64, monotonic per timer role per instance, assigned by the FSM). |
| Echo | The engine tags the corresponding expiry event with the Gen of the arming action it fired for. |
| Match | The FSM ignores (no actions) any expiry event whose Gen differs from the latest armed Gen for that role, and any expiry for a role not currently armed. |
| Stop | StopTimers / StopPreemptDelayTimer invalidate the role's Gen so late fires are ignored. |

This replaces holo's crash-prone assumption that expiries cannot outlive state
(bug 10) with a table-testable rule: `TestFSMStaleTimerGenerationIgnored`.

## Types (described, not coded -- `ai/rules/spec-no-code.md`)

Instance config value (embedded in Startup and ConfigUpdated):

| Field | Type | Meaning |
|-------|------|---------|
| Version | uint8 (2 or 3) | fixed per instance |
| IsOwner | bool | true forces Priority 255 |
| Priority | uint8 | 1..254; 255 iff IsOwner |
| Preempt | bool | default true (RFC) |
| PreemptDelayMs | int | 0 = disabled; from preempt-delay-seconds * 1000 |
| AdvertIntervalMs | int | validated range per version (Boundary Tests) |
| LocalPrimaryIP | netip.Addr | tie-break operand (v4 primary / v6 link-local) |
| VIPs | []netip.Addr | for InstallVIPs/RemoveVIPs payloads |
| AcceptMode | bool | v3 only; snapshot/reporting only |

Events (each a small value type; the FSM's handle method takes exactly one):

| Event | Fields | Producer |
|-------|--------|----------|
| Startup | Config | engine, on instance start |
| Shutdown | - | engine, on config removal / plugin stop / parent down |
| AdvertReceived | Priority uint8; SrcIP netip.Addr; IntervalMs int; VIPCount int | engine (spec-vrrp-5): packet.Decode + validation (spec-vrrp-1) over raw packets from the transport (spec-vrrp-4) |
| MasterDownExpired | Gen uint64 | engine timer |
| AdvertTimerExpired | Gen uint64 | engine timer |
| PreemptDelayExpired | Gen uint64 | engine timer |
| ConfigUpdated | Config | engine, on config apply |

Actions (ordered slice returned by the handle method; closed set):

| Action | Fields | Executor (spec) |
|--------|--------|-----------------|
| SendAdvert | Priority uint8; AdvertIntervalMs int | build + send advert from THESE fields (5-4); never from a cached packet |
| SendAdvertZeroPriority | - | priority-0 advert (5-4) |
| InstallVIPs | VIPs []netip.Addr (full desired set) | address-owner registration via engine (5-3) |
| RemoveVIPs | VIPs []netip.Addr | address-owner deregistration (5-3) |
| AnnounceFailover | - | GARP per IPv4 VIP / unsolicited NA per IPv6 VIP burst (5-4) |
| StartMasterDownTimer | Duration time.Duration; Gen uint64 | arm/reset the master-down clock.Timer (5) |
| StartAdvertTimer | Interval time.Duration; Gen uint64 | arm/reset the advert clock.Timer (5) |
| StartPreemptDelayTimer | Duration time.Duration; Gen uint64 | arm the preempt-delay clock.Timer (5) |
| StopPreemptDelayTimer | - | cancel only the delay timer (5) |
| StopTimers | - | cancel all three timers (5) |
| EmitStateChange | From, To State; Reason string | eventbus + metrics + log (5) |

State snapshot (for `show vrrp`, spec-vrrp-5): state, since (clock.Now at
transition), version, priority, owner/preempt/accept flags, configured + active
advert interval (ms), last advert source + timestamp, armed timers with deadlines.

## Concurrency Model

- One long-lived goroutine per instance, owned and started by the plugin engine in
  OnStarted / reconciled in OnConfigApply (ospf model,
  `internal/plugins/ospf/register.go:455,:442`). Channels in: transport rx, timer
  fires, config/control. This satisfies `ai/rules/goroutine-lifecycle.md`
  (goroutine per lifecycle; timers as dedicated cancellable objects).
- The FSM itself is synchronous and single-threaded: no locks, no channels, no
  goroutines. Only the owning worker calls it. Determinism is a package invariant
  stated in doc.go.
- Timers: the WORKER owns three `clock.Timer` values built from the injected
  `clock.Clock` (`internal/core/clock/clock.go:36` NewTimer, :69 Reset, :65 Stop)
  and selects on their channels; the FSM only emits arm/cancel actions and consumes
  expiry events. Tests drive the FSM directly (no worker, no clock scheduling) or
  through a scenario harness with `sim.FakeClock`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| transport rx (spec-vrrp-4) raw packet → engine packet.Decode + Lookup (spec-vrrp-5/1) | → | FSM handle(AdvertReceived) re-arms master-down (v3 adopts interval) | TestFSMBackupReceivesAdvert |
| clock master-down expiry (engine timer, spec-vrrp-5) | → | FSM handle(MasterDownExpired) emits the ordered Master-promotion actions | TestFSMMasterDownPromotion |
| clock preempt-delay expiry | → | FSM handle(PreemptDelayExpired) promotes with reason "preempt-delay-expired" | TestFSMPreemptDelayPromotion |
| config commit → engine → FSM Startup | → | instance worker startup dispatch into the FSM | test/vrrp/vrrp-instance-up.ci (spec-vrrp-5) |

The first three rows are Go tests inside this child (the FSM's entry points are its
event types). The .ci row is the end-to-end chain: it is owned and implemented by
spec-vrrp-5, which is the FSM's production caller; the umbrella's phase ordering
(children 1-4 are libraries, child 5 wires them) covers the
`ai/rules/wiring-completeness.md` gate at umbrella level, and this spec's exported
surface gains its non-test caller there.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Startup, owner (priority 255) | Exact ordered actions: SendAdvert, InstallVIPs, AnnounceFailover, StartAdvertTimer{interval}, EmitStateChange; state Master |
| AC-2 | Startup, non-owner | Actions: StartMasterDownTimer{3*interval + skew}, EmitStateChange; state Backup; Active_Adver_Interval = own interval |
| AC-3 | Backup, MasterDownExpired (current gen) | Exact ordered actions: SendAdvert, InstallVIPs, AnnounceFailover, StartAdvertTimer, EmitStateChange{reason "master-down-expired"}; state Master |
| AC-4 | Backup, advert priority 0 | Single re-arm: StartMasterDownTimer{Skew_Time from current Active_Adver_Interval}; delay cancelled if armed; state Backup |
| AC-5 | Backup, advert (Preempt false OR adv.prio >= local), v3 | Active_Adver_Interval := adv interval; StartMasterDownTimer recomputed from the ADOPTED interval; state Backup (reference-bug fix: holo/uvrrpd discard instead) |
| AC-6 | Backup, advert Preempt true, adv.prio < local, delay 0 | No actions; master-down untouched (promotion later via its expiry) |
| AC-7 | Preempt-delay lifecycle | First losing advert arms the delay once; losing adverts during hold re-arm master-down (v3 adopt); adv.prio >= local cancels; prio-0 cancels + skew re-arm; expiry promotes |
| AC-8 | Master, AdvertTimerExpired (current gen) | SendAdvert + StartAdvertTimer re-arm |
| AC-9 | Master, advert priority 0 | SendAdvert + StartAdvertTimer reset |
| AC-10 | Master, advert higher prio (or equal prio + greater sender IP) | Exact ordered actions: StopTimers, RemoveVIPs, StartMasterDownTimer (v3: from adopted interval), EmitStateChange; state Backup |
| AC-11 | Master, losing advert (lower prio, or equal prio + smaller/equal sender IP) | v3: SendAdvert immediately, advert timer NOT reset; v2: no actions. Tie-break comparison runs ONLY in Master (Backup test proves sender IP ignored there) |
| AC-12 | Shutdown | From Master: StopTimers, SendAdvertZeroPriority, RemoveVIPs, EmitStateChange → Initialize. From Backup: StopTimers, EmitStateChange (no packet) → Initialize |
| AC-13 | Any event in Initialize (except valid Startup) | No actions, no panic (holo bug 10 negative test); ConfigUpdated stores config silently |
| AC-14 | Timer math boundaries | Skew/MasterDown equal the literal nanosecond values in the Boundary Tests table; skew is never zero for any valid input |
| AC-15 | ConfigUpdated in Master (priority or interval change) | SendAdvert carries the NEW priority/interval and StartAdvertTimer the new interval (holo bug 8 negative test: no stale pre-encoded advert) |
| AC-16 | v2 instance | No interval adoption ever; skew is interval-independent; Master losing advert is silent |
| AC-17 | Stale timer expiry (old generation, or role not armed) | No actions in any state |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Commits a vrrp group; watches it elect | config → engine (5) → FSM Startup → Backup → master-down expiry → Master actions → VIPs live | test/vrrp/vrrp-instance-up.ci (spec-vrrp-5) + TestFSMScenarioElection here |
| 2 | Kills the master; backup takes over | prio-0 advert or silence → FSM prio-0/master-down path → promotion actions | TestFSMScenarioMasterFlap here + test/vrrp/vrrp-failover.ci (spec-vrrp-6, QEMU: promotion + GARP/NA) |
| 3 | Enables preempt with a delay; higher-prio box returns | losing adverts → delay armed → hold → PreemptDelayExpired → promotion | TestFSMScenarioPreemptDelay here + keepalived preempt scenario (spec-vrrp-6) |
| 4 | Gracefully stops ze on the master | Shutdown → SendAdvertZeroPriority → peer promotes within skew | TestFSMScenarioGracefulShutdown here + failover step of effective-vrrp-keepalived.py (spec-vrrp-6) |
| 5 | Changes priority/interval on a live master | ConfigUpdated → regenerated advert parameters on the wire | TestFSMConfigUpdateRegeneratesAdvert here + keepalived observes the new interval (spec-vrrp-6) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestFSMTransitionMatrix | `internal/plugins/vrrp/fsm/fsm_test.go` | Every row of the State Transition Table = one table-test case asserting exact ordered actions + next state (both versions where they diverge) | |
| TestFSMBackupReceivesAdvert | `internal/plugins/vrrp/fsm/fsm_test.go` | Wiring row 1: AdvertReceived in Backup re-arms master-down; v3 adopts the sender interval | |
| TestFSMMasterDownPromotion | `internal/plugins/vrrp/fsm/fsm_test.go` | Wiring row 2: promotion action order (advert before VIPs before announce) | |
| TestFSMPreemptDelayPromotion | `internal/plugins/vrrp/fsm/fsm_test.go` | Wiring row 3 + full AC-7 lifecycle | |
| TestFSMMasterTieBreak | `internal/plugins/vrrp/fsm/fsm_test.go` | AC-11 including IPv6 16-byte compare, equal-IP case, and the Backup-ignores-sender-IP asymmetry | |
| TestFSMStartupRejectsPriorityZero | `internal/plugins/vrrp/fsm/fsm_test.go` | A-5 defensive row | |
| TestFSMNoPanicInInitialize | `internal/plugins/vrrp/fsm/fsm_test.go` | AC-13: every event type fired into Initialize returns no actions (holo bug 10) | |
| TestFSMStaleTimerGenerationIgnored | `internal/plugins/vrrp/fsm/fsm_test.go` | AC-17 generation guard across re-arms and state exits | |
| TestFSMConfigUpdateRegeneratesAdvert | `internal/plugins/vrrp/fsm/fsm_test.go` | AC-15 (holo bug 8) | |
| TestSkewTime / TestMasterDownInterval | `internal/plugins/vrrp/fsm/timers_test.go` | Literal nanosecond values from the Boundary Tests table, v2 + v3 | |
| TestSkewNeverZero | `internal/plugins/vrrp/fsm/timers_test.go` | Skew > 0 for every (priority 1..255) x (valid interval) corner, both versions (uvrrpd/holo truncation bug) | |
| TestV2NoIntervalAdoption | `internal/plugins/vrrp/fsm/fsm_test.go` | AC-16: v2 Active_Adver_Interval pinned to local config across any advert sequence | |
| TestFSMScenarioElection | `internal/plugins/vrrp/fsm/scenarios_test.go` | Two-router election sequence (event scripts, holo-jsonl style): higher priority wins, loser stays Backup | |
| TestFSMScenarioPreemptOnOff | `internal/plugins/vrrp/fsm/scenarios_test.go` | Same sequence with Preempt true/false and delay 0/&gt;0 -- four outcomes | |
| TestFSMScenarioGracefulShutdown | `internal/plugins/vrrp/fsm/scenarios_test.go` | Master shutdown → prio-0 → peer FSM promotes after Skew_Time | |
| TestFSMScenarioMasterFlap | `internal/plugins/vrrp/fsm/scenarios_test.go` | Master dies (silence) → promotion; old master returns higher-prio → demotion path, VIPs removed exactly once | |

### Boundary Tests (MANDATORY for numeric inputs)

Field ranges (validated upstream by YANG in spec-vrrp-5; the FSM asserts them as
preconditions in tests, not at runtime):

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| priority (non-owner) | 1-254 | 254 | 0 (defensive row, A-5) | 255 (owner-only) |
| AdvertIntervalMs (v3) | 10-40950 | 40950 | 9 | 40960 |
| AdvertIntervalMs (v2) | 1000-255000 (whole s) | 255000 | 999 | 256000 |
| PreemptDelayMs | 0-3600000 (Junos hold-time 0..3600 s) | 3600000 | N/A (0 = disabled) | 3600001 |
| Gen | monotonic uint64 | - | N/A | N/A (wrap not reachable) |

Exact expected timer values (computed here so tests assert literal numbers;
Duration nanoseconds, multiplication before division, division last):

| Version | IntervalMs | Priority | Skew_Time (ns) | Master_Down_Interval (ns) |
|---------|-----------|----------|----------------|---------------------------|
| v3 | 10 | 1 | 9,960,937 | 39,960,937 |
| v3 | 10 | 100 | 6,093,750 | 36,093,750 |
| v3 | 10 | 254 | 78,125 | 30,078,125 |
| v3 | 10 | 255 | 39,062 | 30,039,062 |
| v3 | 1000 | 1 | 996,093,750 | 3,996,093,750 |
| v3 | 1000 | 100 | 609,375,000 | 3,609,375,000 |
| v3 | 1000 | 254 | 7,812,500 | 3,007,812,500 |
| v3 | 1000 | 255 | 3,906,250 | 3,003,906,250 |
| v3 | 40950 | 1 | 40,790,039,062 | 163,640,039,062 |
| v3 | 40950 | 100 | 24,953,906,250 | 147,803,906,250 |
| v3 | 40950 | 254 | 319,921,875 | 123,169,921,875 |
| v3 | 40950 | 255 | 159,960,937 | 123,009,960,937 |
| v2 | 1000 | 1 | 996,093,750 | 3,996,093,750 |
| v2 | 1000 | 100 | 609,375,000 | 3,609,375,000 |
| v2 | 1000 | 254 | 7,812,500 | 3,007,812,500 |
| v2 | 1000 | 255 | 3,906,250 | 3,003,906,250 |
| v2 | 255000 | 1 | 996,093,750 | 765,996,093,750 |
| v2 | 255000 | 254 | 7,812,500 | 765,007,812,500 |

(v2 skew is interval-independent: the 255000 rows reuse the same skew values.
The v3 prio-254/interval-10ms row, 78,125 ns, is the case an integer-millisecond
representation truncates to zero -- the mandatory negative test.)

### Functional Tests
<!-- Pure-logic child: the FSM has no user-facing surface of its own. End-to-end
     .ci coverage is owned by spec-vrrp-5/6 per the umbrella Wiring Test table;
     the rows below name the exact tests that exercise this FSM end to end. -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| vrrp-instance-up | `test/vrrp/vrrp-instance-up.ci` (spec-vrrp-5) | Config commit → engine → FSM Startup → election → show vrrp reports Master/Backup | |
| vrrp-backup-hold | `test/vrrp/vrrp-backup-hold.ci` (spec-vrrp-6, QEMU) | Live advert stream holds the FSM in Backup on a real kernel (hold-only, no transition) | |
| vrrp-failover | `test/vrrp/vrrp-failover.ci` (spec-vrrp-6, QEMU) | Master loss → FSM promotion + GARP/NA burst on a real kernel | |
| vrrp-show | `test/vrrp/vrrp-show.ci` (spec-vrrp-5) | Operator reads FSM state snapshot (state, timers, active interval) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| v3 election + failover + preempt on/off | `test/interop/scenarios/vrrp-*-keepalived/` (spec-vrrp-6) | keepalived | FSM transition semantics against an independent implementation (RFC 9568) | |
| v2 election (opt-in version 2) | `test/interop/scenarios/vrrp-v2-keepalived/` (spec-vrrp-6) | keepalived (default v2) | v2 FSM rows: no adoption, silent discard, fixed skew (RFC 3768) | |

### Future (if deferring any tests)
- None deferred. Every transition row, boundary value, and reference bug is tested inside this child; end-to-end and interop rows are owned by spec-vrrp-5/6 per the umbrella phase plan (not deferrals -- they cannot exist before the engine and transport do).

## Files to Modify
- `internal/plugins/vrrp/register.go` - plugin-root seam shared with siblings (umbrella Files to Create assigns it to children 2/5): if a sibling child has already created the minimal registration skeleton, this child adds the instance-worker skeleton (channels + clock.Timer select loop calling the FSM handle method, no sockets); if this child lands first, it creates that skeleton. spec-vrrp-5 completes it (SDK wiring, config, commands).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | N/A | config surface (priority, preempt, preempt-delay-seconds, advertise-interval-milliseconds, version) is owned by spec-vrrp-5; this child consumes resolved values |
| YANG validation constraints | N/A | spec-vrrp-5 (ranges mirrored in this spec's Boundary Tests) |
| YANG custom validators | N/A | spec-vrrp-5 (version-dependent interval validator) |
| CLI commands/flags | N/A | no CLI in this child; state snapshot feeds `show vrrp` in spec-vrrp-5 |
| CLI grammar (action before identifier) | N/A | no commands added |
| Editor autocomplete | N/A | no YANG in this child |
| Functional test for new RPC/API | N/A | no RPC; .ci coverage via spec-vrrp-5/6 (Functional Tests table) |
| Pipe completeness | N/A | no output-producing command |
| Env var registration | N/A | no environment leaves; all tunables are YANG (umbrella decision) |
| Doctor check for runtime dependencies | N/A | pure package: no files, sockets, ports, or binaries introduced (doctor-vrrp-* codes are children 3/4/5) |
| Prometheus counters/metrics | N/A here | EmitStateChange is the metrics feed; ze_vrrp_state / ze_vrrp_transitions_total are registered and incremented by the engine executor (spec-vrrp-5) so the FSM stays pure |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | user-facing rows land with spec-vrrp-5 (umbrella checklist); this child is an internal library |
| 2 | Config syntax changed? | No | spec-vrrp-5 owns the vrrp config surface |
| 3 | CLI command added/changed? | No | none in this child |
| 4 | API/RPC added/changed? | No | none in this child |
| 5 | Plugin added/changed? | No | plugin registration completes in spec-vrrp-5; skeleton-only here |
| 6 | Has a user guide page? | No | docs/guide/vrrp.md is spec-vrrp-5's row; preempt-delay semantics from this spec feed it |
| 7 | Wire format changed? | No | no wire code in this child (spec-vrrp-1) |
| 8 | Plugin SDK/protocol changed? | No | no SDK surface touched |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md`: RFC 9568 + RFC 3768 state-machine/timer rows gain source anchors into `internal/plugins/vrrp/fsm/` when this child closes |
| 10 | Test infrastructure changed? | No | standard go test only |
| 11 | Affects daemon comparison? | No | umbrella row covers it at close |
| 12 | Internal architecture changed? | No | new leaf package, no contract changes; grep `docs/` for `source: internal/plugins/vrrp` returns nothing today |
| 13 | Route metadata keys added/changed? | No | no route metadata |
| 14 | Prometheus counters added/changed? | No | counters registered in spec-vrrp-5 |
| 15 | Registered plugin/event/command inventory changed? | No | inventory changes land with spec-vrrp-5 |
| 16 | Changed source files referenced by doc source anchors? | No | package is new; register.go skeleton has no anchors yet |
| 17 | Existing docs show config/CLI/API examples for this area? | No | none exist for vrrp yet |

## Files to Create
- `internal/plugins/vrrp/fsm/doc.go` - package invariants (pure, single-threaded, actions-as-values), Design: anchor to the umbrella/learned summary
- `internal/plugins/vrrp/fsm/fsm.go` - State type, instance struct, the single handle method implementing the transition table, state snapshot
- `internal/plugins/vrrp/fsm/events.go` - event value types (table above)
- `internal/plugins/vrrp/fsm/actions.go` - action value types (closed set, table above)
- `internal/plugins/vrrp/fsm/timers.go` - Skew_Time / Master_Down_Interval arithmetic (ms in, time.Duration out), Active_Adver_Interval bookkeeping, generation counters
- `internal/plugins/vrrp/fsm/fsm_test.go` - transition matrix + wiring + negative tests
- `internal/plugins/vrrp/fsm/timers_test.go` - boundary tests with literal nanosecond assertions
- `internal/plugins/vrrp/fsm/scenarios_test.go` - conformance-style event-script scenarios (election, preempt on/off/delay, graceful shutdown, master flap)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + `plan/spec-vrrp-0-umbrella.md` + both rfc/short summaries |
| 2. Audit | Files to Modify/Create, TDD Test Plan; validate A-1..A-5 (grep/read, before coding) |
| 3. Wiring phase | Wiring Test table: package skeleton + failing TestFSMBackupReceivesAdvert / TestFSMMasterDownPromotion |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7-9. Fix/re-verify loop | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above (row 9 only) |
| 13. /ze-review gate | Review Gate section below |
| 14. Present summary + close | Two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- create the fsm package skeleton (typed states/events/actions with String() methods and RFC comments; handle method stub returning nil) and the instance-worker skeleton seam in `internal/plugins/vrrp/register.go` (coordinating with whichever sibling landed first)
   - Tests: TestFSMBackupReceivesAdvert, TestFSMMasterDownPromotion (fail against the stub)
   - Files: doc.go, fsm.go, events.go, actions.go, register.go seam
   - Verify: package compiles; wiring tests fail because the stub returns no actions
2. **Phase: Timer math** -- timers.go with v2/v3 skew and master-down arithmetic, Active_Adver_Interval bookkeeping, generation counters
   - Tests: TestSkewTime, TestMasterDownInterval, TestSkewNeverZero (literal values from Boundary Tests)
   - Files: timers.go, timers_test.go
   - Verify: tests fail → implement → pass; every value matches the spec table exactly
3. **Phase: Initialize + Backup rows** -- Startup (owner and non-owner), Shutdown, prio-0, adopt/discard guards, event-in-Initialize safety
   - Tests: transition-matrix rows for Initialize/Backup, TestFSMNoPanicInInitialize, TestFSMStartupRejectsPriorityZero, TestFSMBackupReceivesAdvert passes
   - Files: fsm.go
   - Verify: fail → implement → pass; wiring test 1 green
4. **Phase: Master rows + tie-break + version divergence** -- advert timer, prio-0 reflex, demotion, losing-advert v3 re-advertise vs v2 silence
   - Tests: Master matrix rows, TestFSMMasterTieBreak, TestV2NoIntervalAdoption, TestFSMMasterDownPromotion passes
   - Files: fsm.go
   - Verify: wiring test 2 green; version table fully exercised
5. **Phase: Preempt-delay + ConfigUpdated + staleness** -- delay lifecycle table, config regeneration, generation guard
   - Tests: TestFSMPreemptDelayPromotion, TestFSMConfigUpdateRegeneratesAdvert, TestFSMStaleTimerGenerationIgnored
   - Files: fsm.go, timers.go
   - Verify: AC-7, AC-15, AC-17 green
6. **Phase: Scenario suite** -- event-script conformance tests over full sequences
   - Tests: TestFSMScenarioElection, TestFSMScenarioPreemptOnOff, TestFSMScenarioGracefulShutdown, TestFSMScenarioMasterFlap
   - Files: scenarios_test.go
   - Verify: sequences reproduce the RFC narratives end to end
7. **RFC refs** -- `// RFC 9568 Section 6.4.x: "..."` / `// RFC 3768 Section 6.4.x: "..."` comments per the RFC Documentation section
8. **Full verification** -- `make ze-verify` (lint + all ze tests except fuzz)
9. **Complete spec** -- audit tables, learned summary `plan/learned/NNN-vrrp-2-fsm.md`, two commits (A: code+tests+spec+summary; B: git rm spec)

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every transition-table row has a matrix test case; every AC-N has implementation with file:line |
| Feature completeness | Every End-to-End User Story path exists up to the child-5 seam; no transition row is "TODO" |
| Correctness | v3 semantics traced to `rfc/short/rfc9568.md` sections, NEVER to holo/uvrrpd behavior; all five reference bugs (interval adoption, skew truncation, initialize panic, stale advert, tie-break scope) have failing-then-passing negative tests |
| Naming | States Initialize/Backup/Master; events/actions exactly as tabled; reason strings as tabled (they feed eventbus consumers in spec-vrrp-5) |
| Data flow | FSM performs zero I/O; grep the package for `net.`, `syscall`, `unix.`, `time.Now`, `time.After` -- only `clock.` and `netip` allowed |
| CLI grammar | N/A (no commands) |
| Registration over hardcoding | fsm package registers nothing and no central/shared package gains vrrp knowledge; all discovery happens via the plugin registry in spec-vrrp-5 (`ai/rules/plugin-self-containment.md`) |
| Doctor checks | N/A (no runtime dependencies introduced) |
| YANG validation | N/A here; ranges asserted as test preconditions match the umbrella boundary table |
| Prometheus counters | None registered here by design; EmitStateChange carries everything the engine needs (verify no dead counter stubs, holo bug 9) |
| Rule: goroutine-lifecycle | fsm package spawns zero goroutines (grep `go func` returns nothing); worker model documented for spec-vrrp-5 |
| Rule: spec-no-code / buffer-first | No allocations in the hot path beyond the reusable actions slice; no fmt.Sprintf in transition code (`ai/rules/no-sprintf-alloc.md`) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| fsm package with full transition coverage | `go test ./internal/plugins/vrrp/fsm/ -run TestFSMTransitionMatrix -v` lists one case per table row |
| Timer math exact | `go test ./internal/plugins/vrrp/fsm/ -run 'TestSkewTime|TestMasterDownInterval|TestSkewNeverZero'` green |
| Reference-bug negative suite | grep test names: NoPanicInInitialize, SkewNeverZero, V2NoIntervalAdoption (v3 adopt tested in TestFSMBackupReceivesAdvert), ConfigUpdateRegeneratesAdvert, StaleTimerGeneration |
| Scenario suite | `go test ./internal/plugins/vrrp/fsm/ -run TestFSMScenario` green |
| Purity invariant | grep package for forbidden imports (Critical Review data-flow row) returns nothing |
| register.go worker seam | file exists, compiles, references fsm.Handle path; no sockets |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | AdvertReceived fields arrive pre-validated (spec-vrrp-1), but the FSM must not misbehave on any uint8 priority (0..255) or any interval value: guards are total, no map lookups that can panic, no division by zero (divisor is the constant 256) |
| State manipulation | prio-0 and election rules exact (AC-4, AC-10, AC-11); equal-priority tie-break by IP prevents flap deadlock; owner cannot be locked out (rfc9568 erratum 8298 keeps owner adverts flowing to the FSM -- validation side, spec-vrrp-1) |
| Resource exhaustion | Advert floods cost O(1) per event: no allocation growth, no unbounded state (last-advert bookkeeping is fixed-size); actions slice reused |
| Timer abuse | A forged low-interval advert (v3 adopt) shrinks master-down to its floor of 3*10ms + skew -- bounded by codec validation (interval >= 1 cs, erratum 8301, spec-vrrp-1); noted for the spec-vrrp-6 flood scenario |
| Error leakage | Reason strings are fixed enumerated tokens, never attacker-controlled bytes |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read the cited `rfc/short/` section → RESEARCH if misunderstood; NEVER "fix" toward holo/uvrrpd behavior |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails (child 5/6) | Check the transition row it exercises; if the row is wrong → DESIGN here; if right → the executor spec owns it |
| Audit finds missing AC | Back to the relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A token-scan purity test could include comments harmlessly | The `doc.go` header listing forbidden primitives (`time.NewTimer`, `time.After`) matched its own banned-token scan | `TestFSMPackagePurity` failed on `doc.go:1` on the first full run | Reworded the doc comment to "no direct wall-clock scheduling"; kept the token scan strict on real escapes |
| The spec's `register.go` seam (Files to Modify, Phase 1) should be created by this child | The parent task scoped it out ("hands off any shared register.go; the engine child creates it later") to avoid collision with the parallel spec-vrrp-5 child editing the same file | Parent task instructions | register.go NOT created; the fsm package is self-contained and its production caller lands in spec-vrrp-5 (see Deviations) |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| `New(clk, cfg)` seeding config at construction | The spec Types table makes `Startup` and `ConfigUpdated` carry `Config`; seeding it twice is redundant and ambiguous about which wins | `New(clk clock.Clock)` only; `Startup{Config}` applies the initial config, matching the event-carries-config design |
| Per-role generation counters (one per timer role) | Redundant: expiry matching is role-scoped, so a single global monotonic counter cannot collide across roles | One instance-wide `genSeq`; each role holds its currently-armed generation (0 == not armed) |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Token-scan self-tests matching the design doc that describes the forbidden tokens | First occurrence | Purity/lint self-tests should scan code (AST), not raw bytes, or the doc must avoid the literal tokens | Noted; single occurrence, no rule change proposed yet |

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- Integer-millisecond duration fields cannot represent valid v3 skews (priority 254 at 10 ms interval → 78.125 us); the caller-sketched `durationMs` action fields were refined to `time.Duration` for exactly this reason. The truncate-to-zero bug is not an implementation slip in the references, it is a unit-representation trap.
- The RFC tie-break asymmetry (sender-IP compare only in Master) means equal-priority twins BOTH promote and then resolve; a "helpful" Backup-side tie-break would diverge from every conforming peer.
- `clock.Timer.Reset` staleness is the same trap in every Go timer consumer; carrying generations in actions/events makes it a pure-FSM concern instead of an engine race.
- (impl) A single instance-wide monotonic `genSeq` counter is a strict superset of "monotonic per timer role": each expiry event is matched only against its own role's armed generation, and a global counter never repeats a value, so cross-role collisions are impossible. Three per-role counters would be redundant. This simplified the staleness guard to one increment plus a per-role `armedGen` field (0 == not armed).
- (impl) Interface-marker events/actions (`Event`/`Action` with an unexported marker method, concrete field-carrying structs) let the transition-matrix test assert the exact ordered action slice with `reflect.DeepEqual` and zero mocks. The actions-as-values payoff the spec predicted (holo jsonl insight) materialized precisely: every transition-table row is one `(setup, event) -> (actions, next-state)` tuple.
- (impl) A token-scanning purity test must target code, not prose: the `doc.go` header that listed `time.NewTimer`/`time.After` as primitives to AVOID tripped the very `TestFSMPackagePurity` scan. Reworded the prose; the test now enforces the import allowlist (via `go/parser` ImportsOnly) plus behavioral tokens (`go func`, `time.Now`, `time.After(`, `time.NewTimer`, `time.NewTicker`) that the import check cannot catch because `time` is a legitimately allowed import.
- (impl) `netip.Addr.Compare` after `Unmap()` on both operands is exactly the RFC "unsigned integer comparison in network byte order" for both IPv4 (4-byte) and IPv6 (16-byte), so the Master tie-break needed no hand-rolled byte comparison. Equal addresses compare as 0 (not greater), which correctly classifies a duplicated-primary-IP misconfiguration as a losing advert that never demotes a healthy Master.

## Core Insight

The FSM returns ordered action VALUES instead of performing effects. Every RFC
state-machine row becomes one assertable (state, event, guards) → (actions, state)
tuple; conformance testing needs no mocks, no goroutines, no clock scheduling, and
the engine (spec-vrrp-5) stays a dumb executor that cannot corrupt protocol logic.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Actions-as-values, engine executes | FSM with injected side-effect callbacks (bgp fsm Timers model); FSM owning sockets/timers | Table-testable without mocks (holo jsonl-corpus insight); purity enforceable by import grep; single executor in spec-vrrp-5 |
| Durations as time.Duration, intervals as int ms | integer-ms duration fields (task sketch) | Valid v3 skews are sub-millisecond; ms fields reintroduce the truncate-to-zero bug class (Design Insights); division-last discipline in one file (timers.go) |
| Timer generations in Start*/expiry values | drain-on-reset in the engine; ignore the race | clock.Timer.Reset staleness is real (A-2); generations are deterministic and unit-testable; drain patterns are racy boilerplate per instance |
| Preempt-delay: arm on first losing advert, behave as non-preempt during hold, cancel on rightful master / prio-0 / reconfig / state exit | keepalived-style delay measured from startup; delaying via master-down inflation | Junos hold-time semantics (umbrella leaf name); never delays dead-master failover; interacts safely with master-down (risk R-3) |
| Keep Master/Backup naming (RFC 9568 says Active) | adopt Active Router terminology | keepalived/Junos operator vocabulary; umbrella CLI surface (`show vrrp` master column); mapping documented in Task |
| One FSM struct per group, version fixed at Startup, divergence isolated to the Version Behavior table | separate v2/v3 FSM types | The state graph is identical; only three behaviors diverge (adoption, skew, losing-advert reflex); two types would duplicate 20+ transition rows |
| equal sender IP classified as losing advert in Master | treat as greater (yield) | Loopback is disabled; equal primary IPs are misconfiguration and must not demote a healthy Master |
| ConfigUpdated in Master re-sends + re-arms immediately | wait for the next advert tick | Peers must learn the new interval/priority before old master-down deadlines expire (holo bug 8 family); one extra advert is harmless |

## Known Limitations
- Accept-mode is carried and reported but not enforced by the FSM; with the umbrella's macvlan model the kernel de-facto accepts packets to installed VIPs (holo digest observation). Enforcement, if ever wanted, is a dataplane concern (spec-vrrp-3/5); the umbrella's Known Limitations row (added 2026-07-14) records this: accept-mode false is not dataplane-enforced because VIPs live as kernel addresses on the macvlan, and nftables-based non-accept filtering is deferred.
- No v2/v3 dual-send interop mode (RFC 9568 §8.4.2): version is fixed per instance; the umbrella scopes migration coexistence out.
- No tracking (interface/route/health priority decrement): umbrella Known Limitations; the FSM's ConfigUpdated row is the future hook (a tracker would feed priority changes through it).
- The FSM trusts its caller's threading contract (single goroutine); it contains no locks by design and is not safe for concurrent use.

## RFC Documentation

Add `// RFC 9568 Section X.Y: "<quoted requirement>"` (or `// RFC 3768 Section X.Y`) above enforcing code. MUST document in this package:

| Code site | RFC citations |
|-----------|---------------|
| Each State/Event/Action constant | RFC 9568 §6.4 (state set), §5.2.4 (priority meanings) |
| Owner startup row | RFC 9568 §6.4.1 / RFC 3768 §6.4.1 (owner → Master, advert + announce) |
| Backup adopt row | RFC 9568 §6.4.2 (MUST adopt Max Advertise Interval, recompute) |
| Backup discard row | RFC 9568 §6.4.2 / RFC 3768 §6.4.2 (Preempt + lower priority MUST discard) |
| prio-0 rows | RFC 9568 §6.4.2/§6.4.3 / RFC 3768 §6.4.2/§6.4.3 |
| Master losing-advert row | RFC 9568 §6.4.3 (immediate re-advertisement, new in 9568) vs RFC 3768 §6.4.3 (silent) |
| Tie-break comparison | RFC 9568 §6.4.3 (unsigned network-byte-order compare; Master-only, see State Machine Notes) |
| Shutdown rows | RFC 9568 §6.4.2/§6.4.3 / RFC 3768 §6.4.3 (prio-0 advert from Master only) |
| Skew/master-down formulas | RFC 9568 Timers/Algorithms / RFC 3768 Algorithms |
| Preempt-delay block | explicit comment: NO RFC basis; vendor extension (Junos hold-time semantics); spec reference |

## Implementation Summary

### What Was Implemented
- Pure, deterministic, single-threaded VRRP instance FSM in `internal/plugins/vrrp/fsm/`: actions-as-values (typed events in, ordered `[]Action` out), no sockets/netlink/goroutines/wall-clock. The only clock use is `clock.Now()` for snapshot timestamps.
- `fsm.go`: `State` (Initialize/Backup/Master) with `String()` + RFC comments; `Instance` struct; `New(clk)`; `Handle(Event) []Action` dispatching by state; per-state handlers implementing every State Transition Table row; `Snapshot()` for `show vrrp`.
- `events.go`: `Config` and the 7 event value types (`Startup`, `Shutdown`, `AdvertReceived`, `MasterDownExpired`, `AdvertTimerExpired`, `PreemptDelayExpired`, `ConfigUpdated`) behind an `Event` marker interface.
- `actions.go`: the 11-member closed `Action` set and the 7 `Reason*` constants (exact strings feeding spec-vrrp-5 eventbus/metrics/logs).
- `timers.go`: `skewTime` and `masterDownInterval` as `time.Duration` (mult-before-div, `/256` last); v3 interval-scaled skew, v2 interval-independent skew; every Boundary Tests literal matched exactly.
- Generation staleness guard: single monotonic `genSeq`, per-role armed-generation fields, `matches()` check ignoring stale/unarmed expiries.
- v2/v3 divergence isolated to three points (interval adoption, skew formula, Master losing-advert reflex); preempt-delay (Junos hold-time) lifecycle; Master-only IP tie-break via `netip.Addr.Compare` after `Unmap()`.
- Tests: transition matrix (one case per table row), wiring rows 1-3, all reference-bug negatives, boundary literals, generation guard, and four conformance scenarios. All pass under `go test -race -count=1`; `golangci-lint` clean (0 issues) on the package.

### Bugs Found/Fixed
- No production bugs found (net-new package). Reference-implementation bug classes are locked out by negative tests: v3 interval adoption (holo/uvrrpd discard) -> `TestFSMBackupReceivesAdvert`; skew truncate-to-zero (uvrrpd/holo) -> `TestSkewNeverZero` + boundary literals (v3 prio-254/10 ms = 78,125 ns); panic-in-Initialize (holo bug 10) -> `TestFSMNoPanicInInitialize`; stale pre-encoded advert on reconfig (holo bug 8) -> `TestFSMConfigUpdateRegeneratesAdvert`; tie-break-in-Backup -> `TestFSMMasterTieBreak` (Backup ignores sender IP).
- Self-inflicted (fixed in-session): the purity token-scan matched the forbidden tokens named in the `doc.go` prose (Mistake Log).

### Documentation Updates
- Doc checklist row 9 (`docs/features/rfc-status.md` source anchors into `internal/plugins/vrrp/fsm/`) is deferred to spec closure and is OUTSIDE this task's allowed file scope (task restricts edits to `internal/plugins/vrrp/fsm/**` + this spec). The spec itself states the anchors land "when this child closes"; the umbrella/engine child owns the closure commit that adds them. No other doc rows apply (internal library, no user surface).

### Deviations from Plan
- **register.go seam not created.** Files to Modify and Phase 1 call for the instance-worker skeleton in `internal/plugins/vrrp/register.go`. The parent task explicitly scoped this out ("hands off any shared register.go; the engine child creates it later") to avoid collision with the parallel spec-vrrp-5 child. The fsm package compiles and tests standalone; its non-test production caller lands in spec-vrrp-5 per the umbrella phase plan (the spec's Wiring Test section already assigns the `.ci` row and the exported-surface caller to child 5). Justification: directed scope discipline + cross-child file-collision avoidance.
- **Two extra test files** (`events_test.go`, `actions_test.go`) beyond the spec's listed set. Forced by the repo TDD hook (`c_require_test_first`), which requires a matching `<name>_test.go` before each source `Write`. Additive coverage, no scope reduction. The import-restriction/purity test lives in `fsm_test.go` as `TestFSMPackagePurity`.
- **`New(clk)` takes no Config** (Startup carries it) -- matches the spec Types table where `Startup` has a `Config` field; not an AC behavior change.
- **Events/actions are field-carrying structs behind marker interfaces**, not int-enum constants with `String()` like the BGP FSM style reference. Required because VRRP events/actions carry payloads; the spec Types tables define them as value types, so this is spec-conformant. `State` remains an enum with `String()` + RFC comments.
- **Generation counter is a single global monotonic `genSeq`** (still monotonic per role); simpler superset of "per role", no behavior change.
- **`timers.go` holds only pure math**; generation bookkeeping lives with `Instance` in `fsm.go` (instance state). Keeps `timers.go` independently testable.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Pure FSM: no sockets/netlink/clock syscalls/goroutines | Done | `fsm.go`, `doc.go`; `TestFSMPackagePurity` | Import allowlist + banned-token scan enforced by test |
| Typed events in, ordered action values out | Done | `events.go`, `actions.go`, `fsm.go:Handle` | 7 events, 11 actions, `reflect.DeepEqual` matrix |
| Initialize/Backup/Master states | Done | `fsm.go:State` | `String()` + RFC comments |
| Owner startup, skew + master-down arithmetic | Done | `fsm.go`, `timers.go` | Literals exact per Boundary Tests |
| Priority-0 handling | Done | `fsm.go:backupAdvert`/`masterAdvert` | Backup->skew re-arm; Master->reset+reassert |
| Preemption + preempt-delay (Junos hold-time) | Done | `fsm.go:backupAdvert` | Arm-once, adopt-during-hold, cancel rules |
| Master-state IP tie-break (v4 + v6 16-byte) | Done | `fsm.go:senderWinsTieBreak` | `TestFSMMasterTieBreak` |
| v3 interval adoption; v2 no adoption | Done | `fsm.go:adoptInterval` | `TestFSMBackupReceivesAdvert`, `TestV2NoIntervalAdoption` |
| Safe handling of reference-mishandled events | Done | `fsm.go` | 5 negative-test bug classes green |
| Generation staleness guard | Done | `fsm.go` genSeq/armedGen | `TestFSMStaleTimerGenerationIgnored` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestFSMTransitionMatrix/init/startup/owner` | Exact ordered actions + Master |
| AC-2 | Done | `TestFSMTransitionMatrix/init/startup/non-owner` | StartMasterDownTimer{3*int+skew} + Backup |
| AC-3 | Done | matrix `backup/master-down-expired` + `TestFSMMasterDownPromotion` | Order asserted |
| AC-4 | Done | matrix `backup/advert/priority-zero` (+ cancels-armed-delay) | Skew re-arm |
| AC-5 | Done | `TestFSMBackupReceivesAdvert` + matrix adopt rows | v3 adopts sender interval |
| AC-6 | Done | matrix `backup/advert/discard/preempt-lower-priority-no-delay` | No actions, master-down untouched |
| AC-7 | Done | `TestFSMPreemptDelayPromotion`, `TestFSMPreemptDelayCancels` | Arm-once, adopt-hold, cancel, expiry promote |
| AC-8 | Done | matrix `master/advert-timer-expired/matching-gen` | SendAdvert + re-arm |
| AC-9 | Done | matrix `master/advert/priority-zero/reset-advert-timer` | Reset advert timer |
| AC-10 | Done | matrix `master/advert/higher-priority` + `equal-priority-greater-ip` | StopTimers,RemoveVIPs,StartMasterDown,Emit |
| AC-11 | Done | `TestFSMMasterTieBreak` | v6 16-byte, equal-IP loses, Backup ignores IP |
| AC-12 | Done | matrix `master/shutdown` + `backup/shutdown` | Master: prio-0 + RemoveVIPs; Backup: no packet |
| AC-13 | Done | `TestFSMNoPanicInInitialize` | Every event -> no actions, no panic |
| AC-14 | Done | `TestSkewTime`/`TestMasterDownInterval`/`TestSkewNeverZero` | Literal ns; skew never 0 |
| AC-15 | Done | `TestFSMConfigUpdateRegeneratesAdvert` | New prio/interval on wire |
| AC-16 | Done | `TestV2NoIntervalAdoption` | No adoption; silent losing advert |
| AC-17 | Done | `TestFSMStaleTimerGenerationIgnored` | Old gen / unarmed role -> no actions |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestFSMTransitionMatrix | Done | `fsm_test.go` | One case per transition-table row (both versions where divergent) |
| TestFSMBackupReceivesAdvert | Done | `fsm_test.go` | Wiring row 1 |
| TestFSMMasterDownPromotion | Done | `fsm_test.go` | Wiring row 2 (order) |
| TestFSMPreemptDelayPromotion | Done | `fsm_test.go` | Wiring row 3 + AC-7 |
| TestFSMMasterTieBreak | Done | `fsm_test.go` | AC-11 incl IPv6 + Backup asymmetry |
| TestFSMStartupRejectsPriorityZero | Done | `fsm_test.go` | A-5 |
| TestFSMNoPanicInInitialize | Done | `fsm_test.go` | AC-13 |
| TestFSMStaleTimerGenerationIgnored | Done | `fsm_test.go` | AC-17 |
| TestFSMConfigUpdateRegeneratesAdvert | Done | `fsm_test.go` | AC-15 |
| TestSkewTime / TestMasterDownInterval | Done | `timers_test.go` | Literal ns, v2 + v3 |
| TestSkewNeverZero | Done | `timers_test.go` | > 0 all corners |
| TestV2NoIntervalAdoption | Done | `fsm_test.go` | AC-16 |
| TestFSMScenarioElection | Done | `scenarios_test.go` | Higher priority wins |
| TestFSMScenarioPreemptOnOff | Done | `scenarios_test.go` | Four outcomes |
| TestFSMScenarioGracefulShutdown | Done | `scenarios_test.go` | prio-0 -> skew -> promote |
| TestFSMScenarioMasterFlap | Done | `scenarios_test.go` | Promote+install once, demote+remove once |
| (extra) TestFSMPackagePurity | Done | `fsm_test.go` | Import allowlist + banned tokens |
| (extra) TestFSMPreemptDelayCancels, TestFSMConfigUpdateInBackup, TestFSMSnapshot, TestStateString, TestSkewV2IntervalIndependent, TestEventsImplementInterface, TestAdvertReceivedCarriesDecodedFields, TestActionsImplementInterface, TestReasonConstants | Done | `*_test.go` | Additional coverage |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/vrrp/fsm/doc.go` | Done | Purity/threading invariants |
| `internal/plugins/vrrp/fsm/fsm.go` | Done | State, Instance, Handle, Snapshot |
| `internal/plugins/vrrp/fsm/events.go` | Done | Config + 7 events |
| `internal/plugins/vrrp/fsm/actions.go` | Done | 11 actions + reason constants |
| `internal/plugins/vrrp/fsm/timers.go` | Done | skew / master-down math |
| `internal/plugins/vrrp/fsm/fsm_test.go` | Done | Matrix + wiring + negatives + purity |
| `internal/plugins/vrrp/fsm/timers_test.go` | Done | Boundary literals |
| `internal/plugins/vrrp/fsm/scenarios_test.go` | Done | Conformance scenarios |
| `internal/plugins/vrrp/fsm/events_test.go` | Changed (added) | TDD-hook-required matching test |
| `internal/plugins/vrrp/fsm/actions_test.go` | Changed (added) | TDD-hook-required matching test |
| `internal/plugins/vrrp/register.go` seam | Changed (deferred) | Scoped out by parent task; engine child (spec-vrrp-5) owns it |

### Audit Summary
- **Total items:** 10 requirements + 17 ACs + 16 planned tests + 9 planned files = 52 tracked (plus 10 additive tests).
- **Done:** 10 requirements, 17 ACs, all 16 planned tests, 8 of 9 planned code/test files.
- **Partial:** none.
- **Skipped:** none.
- **Changed:** `register.go` seam deferred to spec-vrrp-5 (parent-directed scope); `events_test.go`/`actions_test.go` added (TDD hook). All documented in Deviations; no ACs dropped.

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| FSM implements RFC 9568/3768 state machines exactly (not reference-implementation behavior) | unit tests (transition matrix + reference-bug negatives) | `go test -race ./internal/plugins/vrrp/fsm/...` = ok; `TestFSMTransitionMatrix` covers every table row; reference-bug negatives green: `TestFSMBackupReceivesAdvert` (v3 adopt), `TestFSMNoPanicInInitialize` (holo bug 10), `TestFSMConfigUpdateRegeneratesAdvert` (holo bug 8), `TestFSMMasterTieBreak` (tie-break scope), `TestSkewNeverZero` (truncation) |
| Timer math exact and truncation-free in one internal unit | boundary tests with literal ns values | `TestSkewTime`/`TestMasterDownInterval` assert all 18 Boundary Tests literals (e.g. v3 prio-254/10 ms = 78,125 ns; v3 prio-1/40950 ms master-down = 163,640,039,062 ns); `TestSkewNeverZero` proves skew > 0 for priority 1..255 x valid intervals, both versions |
| Deterministic without mocks | scenario tests, no goroutines/clock scheduling in package | `TestFSMScenario{Election,PreemptOnOff,GracefulShutdown,MasterFlap}` drive full event scripts with no mocks; `TestFSMPackagePurity` (green) enforces imports = {clock, net/netip, time} and bans `go func`/`time.Now`/`time.After(`/`time.NewTimer`/`time.NewTicker` in non-test source |
| FSM reachable from transport/timer/config entry points | wiring tests + spec-vrrp-5 .ci | `TestFSMBackupReceivesAdvert` (rx->handle), `TestFSMMasterDownPromotion` (timer->handle), `TestFSMPreemptDelayPromotion` (timer->handle); the `.ci` end-to-end row and the exported-surface non-test caller are owned by spec-vrrp-5 per the umbrella phase plan (Wiring Test section) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (fill during /ze-review)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-17 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test (child-5/6 rows verified at umbrella close)
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/vrrp/fsm/`, register.go seam)
- [ ] Integration completeness proven end-to-end (wiring tests + umbrella phase plan)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented (row 9 rfc-status)
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` -- no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (RFC Documentation table complete)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior (via spec-vrrp-5/6, named above)
- [ ] Interop tests for protocol features (via spec-vrrp-6, named above)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-vrrp-2-fsm.md`
- [ ] **Commit A:** code + tests + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-vrrp-2-fsm.md` only (preserves edited spec in git history from commit A)
