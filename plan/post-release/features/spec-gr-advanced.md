# Spec: gr-advanced

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| Depends | - |
| Phase | - |
| Updated | 2026-11-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc4724.md`, `rfc/short/rfc9494.md` - GR/LLGR summaries already in the repo
4. `internal/component/bgp/plugins/gr/gr.go`, `gr_state.go` - current GR implementation
5. `internal/component/bgp/message/notification.go` - NOTIFICATION codes

## Task

Destination spec for the **GR advanced** work item deferred out of
`plan/spec-followup-bgp-feature.md` (umbrella work item "GR advanced (L81,L86)").
The umbrella bundled three sub-features under one line; research (2026-07-08) split
them by RFC and by subsystem. This spec captures the two that are genuinely
Graceful-Restart features. The inherited VPN ATTR_SET requirement (RFC 6368)
is an L3VPN feature and is now owned by `plan/post-release/features/spec-bgp-vpn-attr-set.md`,
including its implementation and proof obligation.

Ze contains helper-side stale retention and EOR/timer purge machinery. That is
the baseline these extensions must preserve, rather than evidence that base GR
works end to end. The September sender-facts handoff recorded advertisement,
retention and test-discrimination findings. Current source still emits no
family tuples from `parseGRCapValue`; RFC 4724 permits that for receiving-only
helper support, so tuple absence alone does not establish a defect.
`onPeerStateChange` now waits for reverse-tier delivery results, and the GR
callback synchronously dispatches retention before returning. That ordering
supersedes the handoff's unconditional
race explanation; it does not prove successful retention.
`plan/release/validation/spec-release-audit-2-bgp-protocol.md` owns base GR capability
semantics and discriminating retention evidence. Any confirmed product defect
needs a separate fix owner; the optional extensions below do not discharge it.

- **Hard Reset (RFC 8538)** - Notification Message Support for BGP Graceful Restart.
  Two coupled pieces:
  1. **N-bit-gated retention on NOTIFICATION.** Today ze flushes on *any*
     NOTIFICATION-triggered down (pre-8538 / RFC 4724 behaviour). RFC 8538 says when
     both peers have negotiated the N-bit, a NOTIFICATION-triggered reset (other than a
     Hard Reset) drives the GR helper procedures (retain routes) instead of an
     immediate flush.
  2. **Hard Reset message (Cease, subcode 9).** Build and send a Cease/Hard-Reset that
     wraps the triggering NOTIFICATION's original [code][subcode][data]; on receipt,
     unwrap it, log the original, and **always** flush (never retain), regardless of
     N-bit.

- **Selection Deferral Timer (RFC 4724 Section 4.1)** - the Restarting-Speaker side.
  When ze itself restarts, defer running best-path selection and re-advertising routes
  to peers until either EOR has been received from every GR-negotiated peer or a
  configurable Selection Deferral Timer expires. ze has no restarting-speaker deferral
  today; GR is helper-only.

- **Helper-side synchronization bound (RFC 9494 Section 4.2)** - added 2026-11-22.
  RFC 9494 Section 4.2: "The session is termed "synchronized" for a given
  AFI/SAFI once the EoR for that AFI/SAFI has been received from the peer or
  once the Selection_Deferral_Timer discussed in [RFC4724] expires." Ze's gr
  plugin treats a family as synchronized on End-of-RIB only. A helper whose
  peer re-establishes and never sends End-of-RIB for a family therefore keeps
  that family's helper state with no bound: a family still in the Restart Time
  period keeps its stale routes, and a family whose Long-Lived Stale Time
  expired after the reconnect stays recorded as expired during synchronization,
  which keeps the peer's GR state alive. RFC 4724 defines the
  Selection_Deferral_Timer for the Restarting Speaker only, so on the helper it
  can only be a local bound. The work: a family counts as synchronized on
  End-of-RIB or when that local bound expires, and the expiry does what
  End-of-RIB does. Which bound, and from which instant it runs, is Q-1 under
  `## Open Questions`: FRR and BIRD differ, and it is Thomas's to decide.

### Scope notes (2026-11-22)

- Another live session is building the Restarting-Speaker selection deferral
  (AC-5, AC-6) in the RIB plugin, under `bgp update-delay max-delay`, and it is
  uncommitted (`internal/component/bgp/plugins/rib/selection_deferral.go`,
  `config_tx.go`). This spec's text still places it in the reactor. When that
  work lands, the Selection Deferral Timer item, its data flow, its files and
  its tests here are re-read against it, and the helper-side bound above reuses
  its bound if Q-1 allows.
- The same uncommitted work edits `rfc/short/rfc4724.md` to claim "all 26
  MUST-level requirements are implemented and proven" with no spec claimed.
  That claim must be reviewed against this spec when it lands.

### Inherited ownership outside the GR feature ACs

- **VPN ATTR_SET (RFC 6368), attribute type 128** - an L3VPN PE-CE feature mis-bundled
  under "GR advanced (L86)" in the 2026-07-06 deferral triage. Not graceful restart.
- **AS-Confederation OTC (RFC 9234 Section 5, umbrella item 3, L88)** - already
  re-deferred separately in the umbrella; cross-referenced here only.

## Required Reading

### Architecture Docs
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
- [ ] `docs/features/rfc-status.md` - current GR/LLGR RFC support ledger
  → Constraint: any RFC behaviour newly implemented updates this ledger with source anchors.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4724.md` - Graceful Restart Mechanism for BGP (base GR + Section 4.1 deferral timer)
  → Constraint: Restarting Speaker MUST defer route selection until EOR-from-all-peers or the deferral timer fires (Section 4.1).
- [ ] `rfc/short/rfc9494.md` - Long-Lived Graceful Restart (already shipped; interacts with retention gating)
  → Constraint: N-bit / retention decisions must not regress the existing LLGR path.
- [ ] RFC 8538 - Notification Message Support for BGP Graceful Restart (N-bit, Cease subcode 9 Hard Reset)
  → Constraint: short summary is NOT yet in `rfc/short/`; generate it with the `ze-rfc` skill (`/ze-rfc rfc8538`) BEFORE this spec moves from `skeleton` to `ready`. Do not summarise from memory.
- [ ] RFC 6368 - Internal BGP as the PE-CE Protocol (ATTR_SET, attribute type 128) — DEFERRED
  → Constraint: `plan/post-release/features/spec-bgp-vpn-attr-set.md` owns its L3VPN design, RFC reading and proof; it is outside this spec's GR feature ACs.

**Key insights:**
- ze is a GR Helper only today; both in-scope features add Restarting-Speaker / negotiation behaviour that does not exist yet.
- Hard Reset is two coupled changes (retention gating + a new Cease subcode-9 wire message), not one.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-07-08 at the line numbers below)
- [ ] `internal/component/bgp/plugins/gr/gr.go` - `decodeGR` (:799) parses the *peer's* GR capability, extracting the R-bit (`Restarting`, :812) and N-bit (`Notification`, :813); the N-bit is stored (`grResult.Notification`, :791) but consumed **only cosmetically** in `formatGRText` (:841-842). `handleStructuredState` (:346) computes `wasNotification := reason == "notification"` (:358) and calls `onSessionDown` (:365); on activation it dispatches helper-side retention (`purge-stale`/`retain-routes`/`mark-stale`, :367-369). EOR handling (`handleEOREvent`, :567) purges stale routes for a family (RFC 4724 Section 4.2, receiving side).
- [ ] `internal/component/bgp/plugins/gr/gr_state.go` - `onSessionDown` (:114) short-circuits `if cap == nil || wasNotification { clearPeerLocked; return false }` (:118): **any** NOTIFICATION-triggered down disables retention, ignoring the N-bit. `onEORReceived` (:241) purges stale per family.
- [ ] `internal/component/bgp/message/notification.go` - `NotifyCeaseHardReset uint8 = 9 // RFC 8538` (:127) and its `String()` case "Hard Reset" (:429) exist, but there is **no** Hard Reset build (wrap `[origCode][origSubcode][origData]`) or unwrap logic anywhere in `internal/component/bgp/`.
- [ ] `internal/core/bgp/attribute/attribute.go` - attribute codes stop at `AttrPrefixSID = 40` / `AttrLargeCommunity = 32` (plus internal `AttrTombstone = 252`); **attribute code 128 (ATTR_SET) is not defined**. L3VPN NLRI scaffolding exists (`family.SAFIVPN`; `internal/component/bgp/plugins/nlri/vpn/types.go`; `internal/component/bgp/plugins/rib/rib_nlri.go`) but the ATTR_SET path-attribute does not.

**Source files read 2026-11-22 for the helper-side synchronization bound** (read, not run; the working tree carries another session's uncommitted edits to this file):
- [ ] `internal/component/bgp/plugins/gr/gr_state.go` - `onSessionReestablished` stops the restart timer and removes only the families the new capabilities fail (`reconnectKeepsFamily`); every other stale family stays in `staleFamilies`, and its Long-Lived Stale Time timer keeps running. `onEORReceived` is the only function that removes a family from `staleFamilies` and from `llstExpiredDuringSync` while the session is up. `handleLLSTExpired`, when the session has re-established, removes the family's unrefreshed routes and records it in `llstExpiredDuringSync`; it releases the peer's state only when the session has not re-established. `onSessionDownPlan` purges, on the next session down, every family recorded in `llstExpiredDuringSync`. No timer started at re-establishment exists.

**Behavior to preserve:**
- Existing Helper-side GR/LLGR behaviour: stale-route retention on non-notification down, EOR-driven purge (RFC 4724 Section 4.2), LLGR long-lived retention (RFC 9494), and the restart-time=0+LLGR fast path in `onSessionDown`.
- The existing GR capability decode (`decodeGR`) wire layout and JSON field names (`restarting`, `notification`, `restart-flags`).
- Per-peer metric-label cleanup on peer removal (umbrella item 6, already shipped).

**Behavior to change:**
- `onSessionDown` retention decision: gate NOTIFICATION-triggered retention on negotiated N-bit instead of always flushing (RFC 8538).
- Add Hard Reset (Cease subcode 9) build/unwrap and wire it into the send/receive paths.
- Add a Restarting-Speaker Selection Deferral Timer (RFC 4724 Section 4.1) that holds best-path selection/advertisement.
- Helper side: a re-established family counts as synchronized on End-of-RIB or when a local bound expires (RFC 9494 Section 4.2), per Q-1.

## Existing Mechanisms

Rows for the helper-side synchronization bound, added 2026-11-22. The rows for
the Hard Reset and Restarting-Speaker items are owed when the spec leaves
`skeleton`.

| Mechanism | Where it lives | What it does today | This spec |
|-----------|----------------|--------------------|-----------|
| Per-family synchronization tracking | `grStateManager` in `internal/component/bgp/plugins/gr/gr_state.go`: `staleFamilies`, `llgrFamilies`, `llstExpiredDuringSync`, `reestablishedConnectionID` | Holds, per peer, the families still awaiting End-of-RIB after a reconnect; `onEORReceived` is the one exit while the session is up | Extends: a second exit, the expiry of a local bound, runs the same completion as `onEORReceived` |
| Per-peer and per-family timers with owner guards | `startRestartTimer`, `enterLLGRLocked`, `handleTimerExpired`, `handleLLSTExpired` in the same file | Arm the Restart Time and Long-Lived Stale Time timers and ignore a callback from a superseded cycle | Extends: the bound is one more timer of the same kind, armed at the instant Q-1 fixes, with the same owner guard |
| Restarting-Speaker selection deferral | `selectionDeferral` in `internal/component/bgp/plugins/rib/selection_deferral.go`, armed from `bgp update-delay max-delay` (another session's work, uncommitted) | Defers best-path selection per family until End-of-RIB from the expected peers or the bound expires | Reuses its configured bound as the helper bound if Q-1 chooses it; the gr plugin reads the value through config, never through a call into the RIB plugin |
| Startup convergence hold | `bgp update-delay` in the reactor (`internal/component/bgp/reactor/update_delay_test.go` names its contract) | Withholds the initial routing update until peers converge or `max-delay` expires | Does not apply: it gates Ze's own advertisement, never a peer's stale routes; only its `max-delay` value is a candidate bound |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A peer sends a NOTIFICATION (plain or Cease/subcode-9 Hard Reset) that tears the session down; delivered to GR as a structured `SessionStateDown` state event with a reason string (`internal/component/bgp/plugins/gr/gr.go`).
- ze itself restarts and re-establishes sessions; peers send UPDATEs then EOR markers, delivered to GR as `eor` events (`gr.go`).
- Operator/engine decides to hard-reset a peer; the NOTIFICATION send path emits Cease/subcode-9 instead of the raw code.

### Transformation Path
1. Session-down reason and the peer's negotiated GR capability (`peerCaps`) are read in `handleStructuredState` (gr.go).
2. `onSessionDown` (gr_state.go) decides retention. New: when `wasNotification` and both local+peer N-bits were negotiated (and it was not a Hard Reset), run helper retention instead of flushing.
3. A received Cease/subcode-9 is unwrapped in the message layer to recover the original [code][subcode][data]; GR treats it as a forced flush (never retain).
4. On ze's own restart, a Selection Deferral Timer in the reactor holds best-path selection until EOR-from-all-GR-peers or timer expiry, then selection runs once and routes are advertised.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| session/FSM → GR plugin | structured state + `eor` events over the plugin event bus | [ ] |
| GR plugin → RIB | `dispatchCommand` text commands (`retain-routes`, `purge-stale`, `mark-stale`) | [ ] |
| wire ↔ message | NOTIFICATION encode/decode incl. Cease subcode 9 wrapping | [ ] |
| reactor → RIB/advertisement | selection-deferral hold on the restarting speaker | [ ] |

### Integration Points
- `internal/component/bgp/plugins/gr/` - retention decision, N-bit negotiation state, EOR tracking
- `internal/component/bgp/message/notification.go` - Hard Reset build/unwrap
- `internal/core/bgp/capability/` - advertise ze's own N-bit in the GR capability
- `internal/component/bgp/reactor/` - Selection Deferral Timer on the restarting speaker
- `internal/component/bgp/fsm/` - hard-reset send path on teardown

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing GR, does not recreate)
- [ ] Registration over hardcoding - any new CLI/config surface registers and is core-discovered, not hardcoded into a core/shared package (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | ze does not yet advertise its own N-bit in the GR capability it sends | `decodeGR` (gr.go) is decode-only; no GR encoder found in `internal/core/bgp/capability` during research | N-bit negotiation may be partly present; scope shrinks | grep/read the GR capability encoder at design time | unvalidated |
| A-2 | ze has no Restarting-Speaker restart tracking (only Helper) | grep for "deferral" in `gr/*.go` returns nothing; R-bit is parsed only from the peer's capability | Selection-deferral scope changes | LSP/grep at design time | unvalidated |
| A-3 | RFC 6368 ATTR_SET is absent and is an L3VPN (not GR) feature | attribute.go has no code 128; RFC 6368 is "iBGP as PE-CE" | If partly present, re-scope the deferral | grep at design time + RFC 6368 summary | unvalidated |
| A-4 | The Cease/subcode-9 constant is defined but unused for build/unwrap | notification.go/429 define the code + String only; grep found no wrap/unwrap | Less to build | grep at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | N-bit retention change regresses existing flush-on-NOTIFICATION expectations | existing GR/LLGR functional tests fail | gate strictly on negotiated N-bit; keep default behaviour when N-bit absent |
| R-2 | Selection Deferral Timer interacts badly with LLGR/EOR purge timing | stale routes linger or are advertised early | reuse existing EOR tracking; add explicit timer-expiry test |
| R-3 | Hard Reset unwrap on malformed/short data panics or mis-reports | fuzz/boundary tests on the NOTIFICATION parser | length-check before unwrap; boundary tests |
| R-4 | The helper bound removes routes of a slow but healthy peer that had not finished sending its table | routes of a large peer flap once after each of its restarts | Q-1 fixes the bound and its start instant; the bound is configurable and the expiry is logged with the peer and family |
| R-5 | Another session's uncommitted `rfc/short/rfc4724.md` claims "all 26 MUST-level requirements are implemented and proven" with no spec claimed, while this spec still lists RFC 4724 work as absent | the summary and this spec disagree once that work lands | review that claim against this spec when it lands, and correct whichever side the producing code shows wrong |

## Open Questions

Questions only Thomas decides. The RFC names the condition and does not say
which helper timer serves it, and FRR and BIRD differ.

### Q-1: which local bound synchronizes a family whose peer sends no End-of-RIB

RFC 9494 Section 4.2: "The session is termed "synchronized" for a given
AFI/SAFI once the EoR for that AFI/SAFI has been received from the peer or once
the Selection_Deferral_Timer discussed in [RFC4724] expires."

What each implementation does on the helper, read in source on 2026-11-22 (FRR
`f1ebf4328a`, BIRD `v3.2.1`):

| Implementation | Ends a family's helper state other than on End-of-RIB | Deciding function |
|----------------|--------------------------------------------------------|-------------------|
| FRR | A stalepath timer (`bgp graceful-restart stalepath-time`, default 360 s) armed at session DOWN, one per peer. It survives re-establishment while any family is still retained, and on expiry removes the stale routes of every retained family, skipping a family whose Long-Lived Stale Time timer is still scheduled. It is one-shot and counted from the down, so a session that returns later than stalepath-time after going down has no bound left. | `bgp_graceful_stale_timer_expire` in `bgpd/bgp_fsm.c`, armed in `bgp_stop`, kept by `bgp_peer_process_gr_cap_clear_stale`, cancelled once every family sent End-of-RIB in `bgp_update_receive_eor` (`bgpd/bgp_packet.c`) |
| FRR | Stops each family's Long-Lived Stale Time timer when the session re-establishes, so that timer never expires during synchronization. | `bgp_establish` in `bgpd/bgp_fsm.c` (cancels `t_llgr_stale`); `bgp_llgr_stale_timer_expire` runs only while the session is down |
| BIRD | No local bound. The restart timer is stopped at re-establishment, and a family in the Restart Time period then ends only on End-of-RIB or when the new capabilities fail it. | `bgp_conn_enter_established_state` in `proto/bgp/bgp.c` (stops `gr_timer`); `bgp_rx_end_mark` in `proto/bgp/packets.c` calls `bgp_graceful_restart_done` |
| BIRD | The Long-Lived Stale Time timer keeps running across re-establishment, and its expiry ends the family's helper state completely: unrefreshed routes are removed and the family is no longer in graceful restart. | `bgp_long_lived_stale_timeout` calls `bgp_graceful_restart_done` in `proto/bgp/bgp.c` |

They do not agree. FRR has a local bound, counted from the session down and not
from the re-establishment. BIRD has none, and treats the Long-Lived Stale Time
expiry itself as the end. Neither runs a timer of the
Selection_Deferral_Timer's kind on the helper.

Ze today matches BIRD on the routes (the Long-Lived Stale Time timer keeps
running and removes unrefreshed routes) and differs from both on the state: the
family stays recorded as expired during synchronization until End-of-RIB.

| Answer | What Ze does | What it changes |
|--------|--------------|-----------------|
| A: a bound counted from re-establishment, taking the value of `bgp update-delay max-delay` | A per-peer timer armed when the session re-establishes; on expiry every family still unsynchronized is treated as if its End-of-RIB arrived | The literal reading of the RFC sentence. Stricter than BIRD, and unlike FRR it also bounds a session that returns late in the long-lived period. With `max-delay` unset there is no value: Thomas also says whether a default applies (FRR's selection deferral default is 120 s) or the family then stays unbounded |
| B: a bound counted from session down, as FRR's stalepath-time, under its own config leaf | A per-peer timer armed at session down that survives re-establishment | Matches FRR. Leaves no bound when the session returns after it expired, which is the usual long-lived case, so the gap stays open for that case |
| C: no new timer; the Long-Lived Stale Time expiry after re-establishment ends the family's helper state, as BIRD | The family is dropped from the expired-during-synchronization record at expiry | Matches BIRD. A family in the Restart Time period stays unbounded, and Ze stops honouring RFC 9494 Section 4.2 "If the session subsequently resets prior to becoming synchronized, any remaining routes (for the AFI/SAFI whose LLST timer expired) MUST be removed immediately.", which it implements today |

AC-7 is written for the condition the RFC names and holds under A and B; the
answer fixes the bound's value, its start instant and its config surface.

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer with negotiated N-bit (both sides) sends a plain NOTIFICATION | → | `onSessionDown` runs helper retention instead of flushing | `gr.TestNotificationWithNegotiatedNBitRetainsRoutes` |
| Peer sends Cease/subcode-9 (Hard Reset) | → | message layer unwraps original [code][subcode][data]; GR flushes, never retains | `message.TestHardResetUnwrapYieldsOriginalNotification`, `gr.TestHardResetDownAlwaysFlushes` |
| Engine triggers a hard reset toward an N-bit peer | → | NOTIFICATION send path emits Cease/subcode-9 wrapping the original notification | `message.TestBuildHardResetWrapsNotification` |
| ze restarts; peers reconnect and EOR is still pending | → | reactor Selection Deferral Timer holds best-path selection until EOR-from-all or timer expiry | `reactor.TestSelectionDeferralHoldsUntilEOR`, `reactor.TestSelectionDeferralTimerExpiryProceeds` |
| Operator scrapes GR behaviour end to end | → | hard-reset + selection-deferral behave per RFC | `test/plugin/gr-hard-reset.ci`, `test/plugin/gr-selection-deferral.ci` |
| A GR peer re-establishes and never sends End-of-RIB for a retained family | → | the gr plugin's local bound expires and completes the family as `onEORReceived` does | `gr.TestRFC9494FamilySynchronizedWhenLocalBoundExpires`, `test/plugin/gr-helper-sync-bound.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Both peers negotiated the N-bit; peer tears the session with a plain (non-Hard-Reset) NOTIFICATION | ze runs the GR helper procedures (retains the peer's routes as stale) instead of the current immediate flush |
| AC-2 | ze receives a Cease with subcode 9 (Hard Reset) | ze unwraps and logs the original [code][subcode][data] and flushes the peer's routes immediately, regardless of N-bit negotiation |
| AC-3 | Engine/operator triggers a hard reset toward a peer that advertised the N-bit | ze sends a Cease/subcode-9 NOTIFICATION whose data wraps the triggering NOTIFICATION's [code][subcode][data]; a non-N-bit peer receives the ordinary NOTIFICATION |
| AC-4 | GR capability negotiation completes | ze advertises its own N-bit; the retention-on-NOTIFICATION behaviour (AC-1) applies only when both local and peer N-bits are set |
| AC-5 | ze restarts and re-establishes sessions with GR-negotiated peers | ze defers best-path selection and route advertisement until EOR is received from every GR-negotiated peer OR the Selection Deferral Timer expires, then selects once |
| AC-6 | Selection Deferral Timer configured (or default) and it expires before all EORs arrive | ze proceeds with best-path selection using whatever it has received; behaviour and default value are configurable via YANG |
| AC-7 | Ze is the helper; the peer re-establishes with its Forwarding State bits set and sends no End-of-RIB for a retained family; the local bound of Q-1 expires | The family counts as synchronized: its routes still marked stale are removed, its Long-Lived Stale Time timer is stopped, it leaves the expired-during-synchronization record, and when it was the last such family the peer's GR state is released, so a later session down starts a fresh GR cycle for it. End-of-RIB arriving before the bound keeps today's behaviour and cancels the bound |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildHardResetWrapsNotification` | `internal/component/bgp/message/notification_test.go` | Cease/subcode-9 build wraps original [code][subcode][data] | |
| `TestHardResetUnwrapYieldsOriginalNotification` | `internal/component/bgp/message/notification_test.go` | unwrap recovers the original notification; short/malformed data handled | |
| `TestNotificationWithNegotiatedNBitRetainsRoutes` | `internal/component/bgp/plugins/gr/gr_state_test.go` | N-bit-gated retention on plain NOTIFICATION | |
| `TestHardResetDownAlwaysFlushes` | `internal/component/bgp/plugins/gr/gr_test.go` | Hard Reset forces flush even with N-bit negotiated | |
| `TestSelectionDeferralHoldsUntilEOR` | `internal/component/bgp/reactor/selection_deferral_test.go` | selection deferred until all-EOR | |
| `TestSelectionDeferralTimerExpiryProceeds` | `internal/component/bgp/reactor/selection_deferral_test.go` | selection proceeds on timer expiry | |
| `TestRFC9494FamilySynchronizedWhenLocalBoundExpires` | `internal/component/bgp/plugins/gr/gr_state_test.go` | AC-7 on a fake clock: no End-of-RIB, the bound expires, the stale purge is dispatched for the family, the family leaves both records and the peer is no longer active; red before the bound exists because the peer stays active | |
| `TestRFC9494EndOfRIBBeforeLocalBoundCancelsIt` | `internal/component/bgp/plugins/gr/gr_state_test.go` | AC-7 second sentence: End-of-RIB first, then the bound's instant passes with no second purge | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Hard Reset wrapped data length | >= 2 bytes (code+subcode) | 2 | 1 | N/A |
| Selection Deferral Timer (seconds) | 0-65535 | 65535 | N/A | N/A (define default at design time) |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `gr-hard-reset` (`.ci`) | `test/plugin/` | operator observes a Hard Reset flushes routes while a plain NOTIFICATION under N-bit retains them | |
| `gr-selection-deferral` (`.ci`) | `test/plugin/` | after ze restart, advertisement is held until EOR/timer | |
| `gr-helper-sync-bound` (`.ci`) | `test/plugin/` | AC-7: a peer restarts, reconnects and sends no End-of-RIB; the operator sees its stale routes gone and its graceful-restart state released once the bound expires | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-gr-hard-reset-peer` | `test/interop/scenarios/` | FRR or GoBGP | Hard Reset (RFC 8538) interop: peer honours Cease/subcode-9 and ze unwraps a peer-sent one | |
| `NN-gr-selection-deferral-peer` | `test/interop/scenarios/` | FRR or GoBGP | restarting-speaker deferral does not blackhole during convergence | |

### Future (if deferring any tests)
- VPN ATTR_SET (RFC 6368) implementation and proof are owned by `plan/post-release/features/spec-bgp-vpn-attr-set.md`.

## Files to Modify
- `internal/component/bgp/message/notification.go` - Hard Reset (Cease subcode 9) build + unwrap
- `internal/component/bgp/plugins/gr/gr.go` - N-bit negotiation tracking; route the Hard-Reset-vs-plain-notification distinction into the retention decision
- `internal/component/bgp/plugins/gr/gr_state.go` - `onSessionDown` retention gate on negotiated N-bit
- `internal/core/bgp/capability/` - advertise ze's own N-bit in the GR capability (confirm A-1 first)
- `internal/component/bgp/reactor/` - Restarting-Speaker Selection Deferral Timer + advertisement hold
- `internal/component/bgp/fsm/` - hard-reset send path on teardown (confirm integration point at design time)
- `internal/component/bgp/yang/` - Selection Deferral Timer config leaf (validation constraints)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (Selection Deferral Timer leaf) | [ ] | `internal/component/bgp/yang/` — read `ai/rules/config.md` + `ai/rules/config.md` |
| YANG validation constraints (range on the timer) | [ ] | `range 0..65535` (default TBD at design) |
| CLI grammar (if a `request bgp ... hard-reset` verb is added) | [ ] | `ai/rules/cli.md` |
| Functional test for new behaviour | [ ] | `test/plugin/gr-hard-reset.ci`, `test/plugin/gr-selection-deferral.ci` |
| Prometheus counters (hard resets sent/received; deferral timer expiries) | [ ] | reuse GR metrics registry; list names at design time |
| RFC status ledger | [ ] | `docs/features/rfc-status.md` (RFC 8538 row; RFC 4724 Section 4.1 row) |

## Files to Create
- `internal/component/bgp/reactor/selection_deferral.go` - Selection Deferral Timer state (name TBD at design)
- `test/plugin/gr-hard-reset.ci` - functional test
- `test/plugin/gr-selection-deferral.ci` - functional test
- (before `ready`) RFC 8538 short summary via the `ze-rfc` skill

## Implementation Steps

1. **Phase: design** - re-verify A-1..A-4, generate the RFC 8538 summary (`/ze-rfc rfc8538`), fill AC/TDD/Data-Flow detail, move Status `skeleton` → `design` → `ready`.
2. **Phase: wiring** - register entry points; write the failing wiring tests from the Wiring Test table.
3. **Phase: Hard Reset message** - build/unwrap Cease subcode 9 in the message layer (TDD).
4. **Phase: N-bit negotiation + retention gate** - advertise local N-bit; gate `onSessionDown` retention on negotiated N-bit; Hard Reset always flushes (TDD).
5. **Phase: Selection Deferral Timer** - restarting-speaker deferral in the reactor; YANG config leaf (TDD).
6. **Functional + interop tests** - `.ci` + interop scenarios.
7. **Full verification** - `./le verify current mode full`.
8. **Complete spec** - fill audit tables, write `plan/learned/NNN-gr-advanced.md`, two-commit closure.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Hard Reset always flushes; N-bit retention only when both sides negotiated |
| Registration over hardcoding | New config/CLI surface registers and is core-discovered, not hardcoded into a core/shared package (`ai/rules/plugins.md`) |
| Data flow | Retention decision stays in the GR plugin; message layer owns wrap/unwrap; reactor owns deferral |
| Rule: no-fabrication | RFC claims cite the generated RFC 8538 summary + section, not memory |

## RFC Documentation

Add `// RFC 8538 Section X: "<quoted requirement>"` above the N-bit negotiation, the
retention gate, and the Hard Reset build/unwrap. Add `// RFC 4724 Section 4.1:
"<quoted requirement>"` above the Selection Deferral Timer. Generate the RFC 8538
short summary with the `ze-rfc` skill before quoting it.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `state-released-only-by-a-message-the-peer-may-never-send.md` | 2026-11-22 | gr plugin: a re-established family is synchronized on End-of-RIB only (RFC 9494 Section 4.2), AC-7 |

## Known Limitations / Deferred

- **VPN ATTR_SET (RFC 6368), attribute type 128 — DEFERRED (out of GR scope).**
  RFC 6368 ("Internal BGP as the PE-CE Protocol") defines the ATTR_SET path attribute
  (type 128) that carries a CE's attributes inside a PE-PE VPN update. This is an L3VPN
  feature, orthogonal to graceful restart; it was mis-bundled under "GR advanced (L86)"
  in the 2026-07-06 deferral triage. attribute code 128 is not defined
  (`internal/core/bgp/attribute/attribute.go`), though L3VPN NLRI scaffolding exists
  (`family.SAFIVPN`, `internal/component/bgp/plugins/nlri/vpn/types.go`).
  **Destination:** `plan/post-release/features/spec-bgp-vpn-attr-set.md`, created on 2026-09-19 for
  the inherited implementation and proof obligation. The July absence claim
  above remains historical and must be rechecked during that spec's research.
  ATTR_SET remains outside this spec's AC-1 through AC-6.
- **AS-Confederation OTC (RFC 9234 Section 5, umbrella item 3, L88)**: already
  re-deferred in `plan/spec-followup-bgp-feature.md` ("Item 3 re-deferral"); ze is a
  single-AS speaker so the confederation OTC rules are vacuously satisfied. Cross-
  referenced here only; not in scope.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`), not just tests
- [ ] Registration over hardcoding respected
- [ ] Interop tests pass (or justified N/A)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added (RFC 8538, RFC 4724 Section 4.1)
- [ ] RFC 8538 short summary generated before `ready`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for the timer and the Hard Reset length
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol behaviour

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/planning.md`). Moves to `design` when picked up.
- Created 2026-07-08 as the destination for umbrella item 1 ("GR advanced"). The umbrella's item-1 row points here.
