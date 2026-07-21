# Spec: fixit-bgp-concurrency-races

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/no-fabrication.md` - verdicts below were confirmed against producers on 2026-07-16; re-confirm on current HEAD before fixing (sibling specs edit the same files)
4. Source files in Current Behavior below (all cites verified 2026-07-16)

## Task

**[MEDIUM]** Four concurrency/consistency leads in the BGP reactor, surfaced by the second
audit pass (2026-07-16). A verification reading pass (2026-07-16, this spec's design phase)
read every producer and confirmed **all four leads REAL** (lead 2 is real on the sent path
only; its received-path reads are verified benign). Phase 1 still writes a failing
reproduction per lead before any fix lands.

1. **`FSM.change` releases the mutex around the state callback** (`internal/component/bgp/fsm/fsm.go:132-143`).
   REAL. `change` sets `f.state` under `f.mu`, then unlocks around the callback and relocks
   (fsm.go:136-142). `Event` (fsm.go:153-173) serializes handlers but not callbacks, so two
   transition callbacks can run concurrently and complete out of order. Concurrent `Event`
   producers (all verified): the session read loop, which is the peer run goroutine
   (`session_handlers.go:221,245`; `runOnce` calls `session.Run` directly at
   `peer_run.go:505`); the hold-timer callback, fired under `s.mu` (`session.go:433-435`) on
   a `clock.AfterFunc` goroutine (`fsm/timer.go:186,223`); the keepalive-timer callback
   (`session.go:445`, `fsm/timer.go:307-312`); the send-hold-timer path
   (`session_write.go:215,231`); Teardown/Close firing `ManualStop` from API goroutines
   (`session_connection.go:382,401,444`, reached via `Peer.Teardown` which releases `p.mu`
   first, `peer.go:818-820`); and the congestion controller firing `TCPConnectionFails`
   (`forward_pool_congestion.go:263-269`). The peer callback (`peer_run.go:332-442`) is long
   and non-idempotent: the to-Established arm stores negotiated capabilities (:340-342), sets
   `PeerStateEstablished` (:343), notifies the reactor (:382-385), and spawns
   `sendInitialRoutes` (:397); the from-Established arm tears all of that down (:399-434).
   Failure mode: the read goroutine's OpenConfirm-to-Established callback is mid-flight when
   a timer/congestion goroutine fires Established-to-Idle; the teardown callback completes,
   then the establishment callback finishes and marks a dead session Established (stale
   negotiated caps, reactor told established after closed, history out of order :436-441).
2. **Unlocked double-read of `peer.session`** on the sent-message path
   (`internal/component/bgp/reactor/reactor_notify.go:360-363`). REAL (sent path). Reads
   `peer.session != nil` then `peer.session.sentSourcePeerStr` with no lock, while the peer
   run goroutine writes `p.session` under `p.mu` (`peer_run.go:223-225` set,
   `peer_run.go:249-251` nil in the `runOnce` defer). Sent-direction callbacks run on other
   goroutines: keepalive timer (`session_write.go:98`), forward pool workers and the RS fast
   path (`session_write.go:300,336`, reached from `fwdBatchHandler` and from other sessions'
   read goroutines), `sendInitialRoutes` (which can outlive teardown, `peer_run.go:245-248`),
   and plugin RPC (`session_write.go:533,580`). Two loads permit non-nil-then-nil (panic) or,
   after a reconnect, a different session whose writeMu-guarded `sentSourcePeerStr`
   (`session.go:276-284`) is read without its `writeMu`. The correct RLock-capture pattern is
   nearby (`forward_pool.go:114-116`). The received-direction reads in the same function
   (`reactor_notify.go:410-414,437-441`) are BENIGN: they execute on the session read
   goroutine, which is the same goroutine that writes `p.session` (program order gives
   happens-before); they must be annotated, not locked.
3. **Dynamic-peer settings mutated without a lock**
   (`internal/component/bgp/reactor/reactor_dynamic.go:316,329,332`). REAL. Writes
   `p.settings.PeerAS` and the `ImportFilters`/`ExportFilters` slice headers with no lock on
   the FSM Established callback (`peer_run.go:353-355`). Cross-goroutine readers (verified):
   `checkRouterIDConflict` reads every peer's `settings.PeerAS` unlocked
   (`routerid_unique.go:58`) from other peers' OPEN-validation goroutines
   (`peer.go:617-620`, under `r.mu.RLock` only); `notifyMessageReceiver` builds `PeerInfo`
   from `s.PeerAS` (`reactor_notify.go:226-238`) on sent-direction goroutines; `Settings()`
   returns the raw pointer with no lock (`peer.go:337-339`) to API/plugin snapshot paths.
   Slice-header writes are multi-word and can tear. Grep confirms these three fields are the
   only post-publication settings mutations (`operation.go:203` and `reactor_api.go:694-749`
   run during peer construction, before publication).
4. **Duplicate-attribute policy disagreement**. REAL, protocol observable.
   `internal/component/bgp/message/rfc7606.go:245-247` skips a duplicate non-MP attribute
   (`continue`) and accepts the UPDATE, but never removes the duplicate bytes from the
   payload. `internal/core/bgp/attribute/wire.go:308-311` (`ensureIndexLocked`,
   wire.go:291-332) rejects any duplicate code as a hard error. `WireUpdate.Attrs()` itself
   succeeds (`wireu/wire_update.go:95-109`, no index build); the error surfaces lazily at
   every indexed consumer (wire.go:94,150,200,219,248). Concrete silent drop: the RIB checks
   `err == nil` on `MPReach()`/`MPUnreach()` (`rib_structured.go:210-211,240-241`) and
   silently skips MP processing, so MP routes in a duplicate-ORIGIN UPDATE vanish with no
   log, no treat-as-withdraw, no NOTIFICATION. IPv4-unicast NLRI is stored from raw bytes
   without an index (`rib_structured.go:94-96,230`), then indexed consumers
   (`filter_ordered.go:225`, cross-context re-encode `wire.go:362-365`) error later;
   `filter_ordered.go:143` ignores the error entirely. Behavior is inconsistent per family
   and per consumer. RFC 7606 Section 3.g prescribes keep-first with actual discard
   (`rfc/short/rfc7606.md:46,180`).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/no-fabrication.md` - verify each producer before acting
  → Constraint: a coherent story is a hypothesis; every verdict above cites the producer read on 2026-07-16; re-confirm on HEAD before fixing.
- [ ] `ai/rules/memory-architecture.md` - the reactor's lock discipline and cross-goroutine sharing
  → Constraint: `p.settings` and `p.session` are shared across goroutines; reads/writes need `p.mu` or an atomic snapshot; `fwdFacts` (peer.go:289, peer_forward_facts.go:76-99) is the in-repo atomic-snapshot precedent.
- [ ] `ai/rules/fail-closed-guards.md` - `ensureIndexLocked` is a guard
  → Constraint: do not weaken the duplicate rejection in `wire.go`; fix the policy at the RFC 7606 boundary so the guard never sees session-path duplicates.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7606.md` - duplicate-attribute handling
  → Constraint: Section 3.g: non-MP duplicate = "Discard all but first occurrence", continue processing (rfc/short/rfc7606.md:46,180); duplicate MP_REACH/MP_UNREACH = session reset (rfc7606.go:280-285 already implements this).
  → Constraint: keep-first requires actually removing the later occurrences from the payload, not merely skipping their validation.

**Key insights:**
- Lead 4 is protocol-observable (an UPDATE with duplicate ORIGIN) and is pinned by crafted-wire tests; leads 2-3 are memory races `-race` can flag; lead 1 is an ordering bug that `-race` alone may NOT flag (each shared field is individually atomic or locked) and needs an ordering-assertion test.
- Lock orders that exist today and must not gain reverse edges: `r.mu` before `p.mu` (`reactor_dynamic.go:184` holds `r.mu` and calls `Peer.Stop` which takes `p.mu`, peer.go:780); `s.mu` before `s.writeMu` before `s.sendHoldMu` (session.go:174-190); RIB `peerMu` (outer) before `shard.mu` (inner), never an interner mutex under `peerMu` (rib.go:325-332).
- The hold-timer callback fires `Event` while HOLDING `s.mu` (session.go:433-435) and the to-Established callback takes `s.mu.RLock` via `session.Negotiated()` (peer_run.go:340). Any lead-1 design where an `Event` caller waits for another goroutine's callback deadlocks here. This resolves A-2: serialization must be non-blocking.

## Current Behavior (MANDATORY)

**Source files read:** (all on 2026-07-16; line cites current at that date)
- [ ] `internal/component/bgp/fsm/fsm.go` - `change` unlocks around the callback (:132-143); `Event` holds `f.mu` for handlers (:153-173); OpenConfirm+KeepaliveMsg resets hold timer then transitions (:367-373)
  → Constraint: `change` runs the callback only when a callback is set and from != to (:136).
- [ ] `internal/component/bgp/fsm/timer.go` - timers are `clock.AfterFunc` (:186,223,307-312,357); callbacks run on timer goroutines
- [ ] `internal/component/bgp/reactor/session.go` - hold-timer callback fires `EventHoldTimerExpires` under `s.mu` (:433-435); keepalive callback fires `EventKeepaliveTimerExpires` (:445); session lock hierarchy comment (:174-190); `sentSourcePeerStr` contract "set alongside sentMeta ... MUST NOT be read outside writeMu" (:276-284)
  → Constraint: the audit's cite ":432-434,444" drifted by one line; actual :433-435 and :445.
- [ ] `internal/component/bgp/reactor/peer_run.go` - the FSM callback closure (:332-442); `p.session` set under `p.mu` (:223-225) and nilled under `p.mu` in the runOnce defer (:249-251); `session.Run` called directly on the peer run goroutine (:505); `sendInitialRoutes` spawned per establishment (:397) and documented to outlive teardown (:245-248)
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyMessageReceiver` (:218); the unlocked sent-path double-read (:360-363); received-path unlocked reads (:410-414,437-441); `PeerInfo` built from raw settings under `r.mu.RLock` only (:226-238)
- [ ] `internal/component/bgp/reactor/session_write.go` - sent-direction callback invocations inside `writeMu` critical sections (:98,300,336,533,580)
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go` - unlocked `p.settings` mutation (:316,329,332) in `resolveDynamicPeerSettings` (:303-336), called from the Established callback (peer_run.go:353-355); `refreshForwardFacts` called after mutation (:335)
- [ ] `internal/component/bgp/reactor/routerid_unique.go` - `checkRouterIDConflict` reads `peer.settings.PeerAS` unlocked (:58) while correctly RLock-capturing `peer.session` (:61-63)
- [ ] `internal/component/bgp/reactor/peer.go` - `Settings()` returns the raw pointer, no lock (:337-339); `Peer.Stop` only cancels the context (:779-787); `Teardown` releases `p.mu` before session teardown (:801-828); `validateOpen` calls the conflict check under `r.mu.RLock` (:602-634); `fwdFacts` atomic snapshot field (:289)
- [ ] `internal/component/bgp/reactor/forward_pool.go` - the correct RLock-capture pattern (:114-116); `sentMeta`/`sentSourcePeerStr` set under `writeMu` (:146-149)
- [ ] `internal/component/bgp/reactor/forward_pool_congestion.go` - congestion teardown fires `fsm.Event` after releasing `peer.mu` (:263-269)
- [ ] `internal/component/bgp/message/rfc7606.go` - duplicate non-MP attribute skipped, bytes retained (:245-248); duplicate MP attributes session-reset (:280-285)
- [ ] `internal/core/bgp/attribute/wire.go` - `ensureIndexLocked` rejects duplicates (:291-332, rejection at :308-311); index built lazily by Get/GetRaw/All/ForEach/PackFor (:94,150,200,219,248)
- [ ] `internal/component/bgp/wireu/wire_update.go` - `Attrs()` builds no index (:95-109)
- [ ] `internal/component/bgp/reactor/session_validation.go` - `enforceRFC7606` (:26); the ATTR_DISCARD rebuild path via `message.RebuildUpdateBody` (:117-126); `isIBGP` reads `s.settings.PeerAS` on the read goroutine (:73)
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` - raw attr bytes stored without index (:94-96); MP processing silently skipped on index error (:210-211,240-241)
- [ ] `internal/component/bgp/plugins/rib/rib.go` - documented `peerMu` (outer) -> `shard.mu` (inner) hierarchy (:325-332)

**Behavior to preserve:**
- The documented `peerMu` -> `shard.mu` lock hierarchy in the RIB (rib.go:325-332) and the session `s.mu` -> `writeMu` -> `sendHoldMu` order (session.go:174-190); no new reverse edges.
- Uncontended FSM transitions keep today's synchronous semantics: when `Event` returns on the firing goroutine and no other transition was mid-callback, the callback has completed (the read loop depends on establishment being fully set up before the next message is processed).
- `ensureIndexLocked` stays strict (fail-closed): duplicates from any non-session source (locally built updates, injected wire) remain hard errors.
- RFC 7606 duplicate MP_REACH/MP_UNREACH session reset (rfc7606.go:280-285).
- Zero-copy forwarding on ContextID match; the duplicate-strip rebuild allocates only on the malformed-UPDATE path (same cost model as the existing ATTR_DISCARD rebuild).
- Existing single-threaded fast paths where a lock would add contention without a real race (received-direction `peer.session` reads stay lock-free with a same-goroutine justification comment).

**Behavior to change:**
- FSM transition callbacks: never overlap, always run in transition order (lead 1).
- Sent-event source-peer attribution: carried through the callback, not re-read from `peer.session` (lead 2).
- `PeerAS`/`ImportFilters`/`ExportFilters`: written and cross-goroutine-read under `p.mu` accessors (lead 3).
- Duplicate non-MP attributes: stripped keep-first at the RFC 7606 boundary so the UPDATE is processed consistently everywhere downstream (lead 4).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Lead 1: any two concurrent FSM `Event` calls where both change state (read loop, hold/keepalive/send-hold timers, teardown API, congestion controller).
- Lead 2: a sent BGP message (keepalive timer, forward worker, RS fast path, sendInitialRoutes, plugin RPC) racing `runOnce`'s teardown defer.
- Lead 3: a dynamic peer reaching Established (writes) racing another peer's OPEN validation or any API/plugin settings snapshot (reads).
- Lead 4: a received UPDATE whose path attributes contain the same non-MP code twice (e.g. two ORIGINs).

### Transformation Path
1. Lead 1: `Event` -> `f.mu` -> handler -> `change` (state write) -> unlock -> peer callback (peer_run.go:332-442) -> relock. Fix inserts an ordered, non-blocking transition queue between state write and callback execution.
2. Lead 2: `Send*` (inside `writeMu`) -> `s.onMessageReceived` -> `r.notifyMessageReceiver` -> today: re-lookup `peer.session` (racy); after fix: source-peer string travels as a callback argument captured at the write site.
3. Lead 3: OPEN received -> Established callback -> `resolveDynamicPeerSettings` (writes, after fix under `p.mu`) -> readers (`checkRouterIDConflict`, PeerInfo builders, filter-chain getters) go through `p.mu`-guarded accessors.
4. Lead 4: wire -> `readAndProcessMessage` -> `enforceRFC7606` (after fix: records duplicate occurrences and rebuilds the body keep-first, reusing the ATTR_DISCARD rebuild) -> callback dispatch -> RIB/filters/re-encode all see a single occurrence; `ensureIndexLocked` unchanged.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Timer/read/teardown/congestion goroutines <-> FSM callback | `FSM.Event` -> `change` -> peer callback; ordering via new per-FSM transition queue | [ ] |
| Sender goroutines <-> `peer.session` | today unlocked double-read vs `p.mu` write; after fix the value crosses inside the callback arguments | [ ] |
| Peer goroutines <-> `p.settings` mutable fields | `p.mu` accessors (write at establish, reads from conflict check / PeerInfo / filter getters) | [ ] |
| RFC 7606 validator <-> attribute index | one policy owner: validator strips keep-first; index stays strict | [ ] |

### Integration Points
- `FSM.change` (fsm.go:132-143) gains the ordered callback dispatch; `SetCallback` contract documents ordering and non-overlap.
- `MessageCallback` (session.go:172) gains the source-peer argument; all six invocation sites are in session_read.go:234 and session_write.go:98,300,336,533,580.
- `resolveDynamicPeerSettings` (reactor_dynamic.go:303-336) and new `Peer` accessors; `fwdFacts` refresh stays last (:335).
- `enforceRFC7606` (session_validation.go:26) reuses `message.RebuildUpdateBody` (the :117-126 pattern) for duplicate stripping.

### Architectural Verification
- [ ] No bypassed layers (locking stays within the reactor's documented hierarchy; RFC policy stays at the validation boundary)
- [ ] No unintended coupling (the transition queue is FSM-internal; no reactor types leak into `fsm`)
- [ ] No new lock ordering: transition queue is non-blocking (no goroutine waits while holding a lock); `p.mu` reads under `r.mu` follow the existing `r.mu` -> `p.mu` order
- [ ] Registration over hardcoding -- N/A (concurrency internals), stated for completeness (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each lead is a real race/inconsistency | Producer reading pass 2026-07-16: lead 1 fsm.go:132-143 + concurrent Event callers enumerated in Task; lead 2 reactor_notify.go:360-363 vs peer_run.go:249-251 (sent path REAL; received path :410-414,:437-441 BENIGN, same goroutine as writer); lead 3 reactor_dynamic.go:316,329,332 vs routerid_unique.go:58 and peer.go:337-339; lead 4 rfc7606.go:245-247 vs wire.go:308-311, silent drop at rib_structured.go:210-211 | The lead is benign; document verified-benign, no code change | Phase 1 reproductions: ordering test (lead 1), `-race` (leads 2-3), crafted duplicate-ORIGIN UPDATE (lead 4) | confirmed (reading); repro pending |
| A-2 | FSM callback serialization can be added without deadlock | Non-blocking FIFO queue: no goroutine ever waits while holding a lock. Blocking designs are PROVEN unsafe: hold-timer fires Event holding `s.mu` (session.go:433-435) while the to-Established callback needs `s.mu.RLock` (peer_run.go:340); holding `f.mu` across the callback self-deadlocks on any `State()` call. `peerMu` -> `shard.mu` is untouched (callbacks already call plugins from these goroutines today) | Redesign lead-1 fix | Deadlock analysis above (done); teardown-vs-establish stress test + `-race` at implement | confirmed by analysis; stress test pending |
| A-3 | The duplicate-attribute policy should be keep-first, implemented as actual discard | rfc/short/rfc7606.md:46 "Other attribute appears > 1 time ... Discard all but first occurrence" and :180; MP duplicates stay session-reset (rfc7606.go:280-285) | Wrong policy chosen | re-read of rfc/short/rfc7606.md done 2026-07-16 | confirmed |
| A-4 | `PeerAS`, `ImportFilters`, `ExportFilters` are the only post-publication `PeerSettings` mutations | grep of `settings.<Field> =` in reactor 2026-07-16: only reactor_dynamic.go:316,329,332 mutate after the peer is in `r.peers`; operation.go:203 and reactor_api.go:694-749 run during construction | More fields need the accessor treatment | re-run the grep during /ze-implement audit | confirmed (grep); re-verify at implement |
| A-5 | No production caller depends on `Event` returning only after its callback in the CONTENDED case (uncontended stays synchronous) | Call-site review: contended overlap is today's bug window; the read loop's own transitions stay synchronous because the firing goroutine drains its own queue entry when no drainer is active | Read loop could process an UPDATE before a contended Established callback finishes on another goroutine | review of all 46 `Event`/`logFSMEvent` sites + stress assertion that plugins never see UPDATE before established notification | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Callback serialization adds latency to hot paths | perf regression in `ze-perf` | The queue engages only on state CHANGE (rare); Events that do not transition (KeepaliveMsg/UpdateMsg in Established) never touch it; benchmark before/after |
| R-2 | A "fix" hides rather than resolves a race | race reappears under load | Drive each fix from a failing reproduction (ordering test or `-race` test), not from reading alone |
| R-3 | Sibling specs concurrently edit `fsm/fsm.go`, `reactor/session.go`, `reactor_notify.go` | merge conflicts; verdict line cites drift | Re-run the Phase 1 verification pass after any rebase; coordinate landing order with Thomas |
| R-4 | `MessageCallback` signature change ripples to fakes/tests | compile errors | The compiler enumerates every call site; mechanical update in the same commit |
| R-5 | Contended-case async callback lets the read loop run ahead of establishment (A-5) | stress test sees UPDATE delivered before established notification | Lead-3 locks remove the data race; if the ordering gap is observed, gate UPDATE delivery on establishment completion (documented fallback, needs design) |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| concurrent Established<->Idle transitions | -> | per-FSM ordered transition queue: callbacks never overlap, apply in order | `TestFSMCallbackOrderingRace` (fsm), `TestPeerEstablishedTeardownOrdering` (reactor) |
| sent-message path during peer teardown | -> | source-peer string carried via `MessageCallback` argument; no `peer.session` re-read | `TestPeerSessionSentPathRace` |
| dynamic-peer establishment + concurrent settings read | -> | `p.mu`-guarded write in `resolveDynamicPeerSettings` + reader accessors | `TestDynamicPeerSettingsRace` |
| UPDATE with duplicate ORIGIN over a live session | -> | `enforceRFC7606` strips keep-first; RIB stores the route; no silent MP drop | `TestSessionDuplicateOriginKeepFirst` (Go, net.Pipe session) |
| `ze bgp decode` on duplicate-ORIGIN hex | -> | decode path matches session keep-first policy | `test/decode/bgp-update-duplicate-origin.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Concurrent FSM transitions (establish vs hold-timer/congestion teardown), repeated under `-race` | Callbacks never overlap and complete in transition order; a peer is never left `PeerStateEstablished` after a later Established-to-Idle transition; history order matches transition order |
| AC-2 | Sent-message events racing `runOnce` teardown (session nilled mid-send) | No data race, no nil-deref window; `SourcePeerStr` attribution correct even across reconnect |
| AC-3 | Dynamic-peer establish racing `checkRouterIDConflict` and settings snapshots | No torn/stale `PeerAS` or filter-slice read under `-race`; conflict check sees either the old or the new value, never a mix |
| AC-4 | UPDATE with a duplicate well-known attribute (two ORIGINs), IPv4 and MP variants | Keep-first: first occurrence used, later occurrences removed at validation; route installed in RIB; no silent MP skip; no session reset for non-MP duplicates; duplicate MP_REACH/MP_UNREACH still session-resets |
| AC-5 | Received-direction `peer.session` reads (reactor_notify.go:410-414,437-441) | Documented verified-benign (same-goroutine justification comment); no lock added; recorded in this spec |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFSMCallbackOrderingRace` | `internal/component/bgp/fsm/fsm_test.go` | AC-1: instrumented callback (in-flight counter + recorded order) driven by concurrent Events; fails on current code | |
| `TestPeerEstablishedTeardownOrdering` | `internal/component/bgp/reactor/peer_run_test.go` | AC-1: establish vs teardown race never leaves a dead session marked Established | |
| `TestPeerSessionSentPathRace` | `internal/component/bgp/reactor/reactor_notify_test.go` | AC-2: `-race` on sent event vs `p.session = nil` | |
| `TestDynamicPeerSettingsRace` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | AC-3: `-race` on `resolveDynamicPeerSettings` vs `checkRouterIDConflict` | |
| `TestRFC7606DuplicateOriginRecorded` | `internal/component/bgp/message/rfc7606_test.go` | AC-4: validator reports duplicate occurrences for stripping | |
| `TestEnforceRFC7606DuplicateRebuild` | `internal/component/bgp/reactor/session_validation_test.go` | AC-4: body rebuilt keep-first; NLRI and non-duplicate attributes preserved byte-exact | |
| `TestDuplicateAttributeIndexStillRejects` | `internal/core/bgp/attribute/wire_test.go` | Guard unchanged: `ensureIndexLocked` still hard-errors on duplicates (fail-closed) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| non-MP duplicate occurrences of one code | 1..N | 1 kept (N=1 is normal) | N/A | occurrences 2..N stripped, first kept |
| duplicate MP_REACH or MP_UNREACH count | 1 | 1 | N/A | >1 = session reset (unchanged, rfc7606.go:280-285) |
| distinct duplicated codes in one UPDATE | 0..k | all firsts kept | N/A | each code keep-first independently |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-update-duplicate-origin` | `test/decode/bgp-update-duplicate-origin.ci` | `ze bgp decode` of an UPDATE with two ORIGIN attributes yields keep-first decode output, not an error (decode aligned with session policy; see open decision D-4b) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | FRR/BIRD/GoBGP cannot be made to emit duplicate path attributes; the malformed-wire scenario is covered by the crafted-bytes session test `TestSessionDuplicateOriginKeepFirst` and the decode .ci. Leads 1-3 are internal concurrency with no wire-visible negotiation surface | |

## Files to Modify
- `internal/component/bgp/fsm/fsm.go` - ordered non-blocking transition-callback queue in `change`/`Event`; document the `SetCallback` ordering contract (lead 1)
- `internal/component/bgp/reactor/session.go` - `MessageCallback` type gains the source-peer argument (:172); consider narrowing the `s.mu` hold around the hold-timer `logFSMEvent` (:433-435) if the deadlock analysis at implement shows it guards nothing (lead 1/2)
- `internal/component/bgp/reactor/session_read.go` - received-direction callback call site (:234) updated for the new signature (lead 2)
- `internal/component/bgp/reactor/session_write.go` - sent-direction call sites (:98,300,336,533,580) pass `s.sentSourcePeerStr` from inside `writeMu` (lead 2)
- `internal/component/bgp/reactor/reactor_notify.go` - use the callback argument; delete the `peer.session` double-read (:360-363); add same-goroutine justification comments on :410-414 and :437-441 (lead 2, AC-5)
- `internal/component/bgp/reactor/reactor_dynamic.go` - `resolveDynamicPeerSettings` writes under `p.mu` (lead 3)
- `internal/component/bgp/reactor/peer.go` - `p.mu`-guarded accessors for `PeerAS`/`ImportFilters`/`ExportFilters`; convert cross-goroutine readers found by the sibling call-site audit (lead 3)
- `internal/component/bgp/reactor/routerid_unique.go` - read `PeerAS` via the guarded accessor (existing `r.mu` -> `p.mu` order; no new edge) (lead 3)
- `internal/component/bgp/message/rfc7606.go` - record duplicate non-MP occurrences (offset list) instead of bare `continue` (lead 4)
- `internal/component/bgp/reactor/session_validation.go` - `enforceRFC7606` strips recorded duplicates keep-first via the existing rebuild path (:117-126 pattern) (lead 4)
- NOT `internal/core/bgp/attribute/wire.go` for the policy: `ensureIndexLocked` stays strict per `ai/rules/fail-closed-guards.md` (the skeleton listed it as a fix target; the design keeps the guard and fixes the boundary). Only its test file gains coverage.

## Files to Create
- `test/decode/bgp-update-duplicate-origin.ci` - functional keep-first decode test (hex UPDATE with two ORIGINs, format per test/decode/bgp-keepalive.ci)

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1: Lead 1 fixed with a per-FSM ordered FIFO transition queue; enqueue under `f.mu`, first enqueuer with no active drainer drains callbacks in order outside `f.mu`; enqueue never blocks | (a) hold `f.mu` across the callback: self-deadlock on any `State()` call in the callback path (RWMutex non-reentrant); (b) blocking ticket order (Event waits for its callback): deadlocks against the hold-timer path holding `s.mu` (session.go:433-435) while the drainer needs `s.mu.RLock` (peer_run.go:340); (c) make the 110-line callback idempotent: fragile, still exposes observers to reordering | Non-overlap + order with zero new blocking edges; uncontended case keeps today's synchronous semantics; contended case (today's corruption window) becomes ordered-async |
| D-2: Lead 2 fixed by passing the source-peer string through `MessageCallback` from inside the sender's `writeMu` section | RLock-capture of `peer.session` (forward_pool.go:114-116 pattern): fixes the nil-deref but still reads a possibly different (reconnected) session's `writeMu`-guarded field outside its `writeMu`, and mis-attributes across reconnect | The value originates at the write site that already owns it under `writeMu` (session.go:276-284 contract); the racy re-lookup disappears entirely; compiler enumerates all call sites |
| D-3: Lead 3 fixed with `p.mu`-guarded write + narrow `Peer` accessors for the three mutable fields; immutable settings fields stay lock-free | (a) `atomic.Pointer[PeerSettings]` copy-on-write: compiler-enforced coverage, but `Session` captures the settings pointer at construction (session.go:394-395), so a swap strands the session on the pre-resolution snapshot (`isIBGP` at session_validation.go:73 would read PeerAS 0 forever) or requires swapping `s.settings` and auditing every session-side unlocked read; (b) `atomic.Uint32` PeerAS: type ripples through config resolution, does not cover the slice fields | Smallest correct surface: exactly three fields mutate post-publication (A-4); accessor conversion is a bounded, greppable reader inventory; `r.mu` -> `p.mu` order already exists |
| D-4: Lead 4 fixed at the RFC 7606 boundary: validator records duplicates, `enforceRFC7606` rebuilds the body keep-first; `ensureIndexLocked` unchanged | (a) relax `ensureIndexLocked` to skip duplicates: weakens a fail-closed guard for every caller including locally built and forwarded wire; (b) treat-as-withdraw for non-MP duplicates: contradicts rfc/short/rfc7606.md:46 | One policy owner at the protocol boundary; downstream consumers (RIB, filters, re-encode, zero-copy forward) all see a clean single-occurrence payload; rebuild cost only on malformed input, same as the shipped ATTR_DISCARD path |
| D-4b (~~open, needs Thomas~~ → RESOLVED 2026-07-17 = YES, see "Resolved open decisions" below): should `ze bgp decode` run the same duplicate strip so diagnostics match session behavior? | leave decode erroring at index build (status quo for raw hex) | Recommendation: yes, align decode; the .ci pins whichever is chosen -- if Thomas prefers strict decode, the .ci asserts the explicit duplicate-attribute error instead |
| D-5: fix order 4 -> 2 -> 3 -> 1 | any order | Leads 4 and 2 are independent and lowest risk; lead 3 must land before lead 1 because the transition-queue fix can move the Established callback off the read goroutine in the contended case, widening lead 3's window if its same-goroutine assumptions were still load-bearing |

### Resolved open decisions (append-only)

→ AUTONOMOUS DEFAULT (2026-07-17) [STAKES: protocol]: **D-4b = YES** -- align `ze bgp decode` with the session keep-first strip so diagnostics match on-session behavior (adopts the D-4b recommendation). Rationale: one duplicate-attribute policy avoids a decode-vs-session divergence that would mislead an operator debugging a malformed peer; the boundary strip is reusable by the decode path (`enforceRFC7606` at session_validation.go:34 → `RebuildUpdateBody` at message/attr_discard.go:295, both re-verified present 2026-07-17), and `test/decode/bgp-update-duplicate-origin.ci` pins the chosen keep-first decode output. Thomas: override if wrong -- revert to strict decode (error at index build); the `.ci` would then assert the explicit duplicate-attribute error instead of keep-first output.

→ AUTONOMOUS DEFAULT (2026-07-17) [STAKES: arch]: **D-1 trade-off = ACCEPT the recorded design as-is** -- per-FSM ordered non-blocking FIFO transition queue; contended transitions become ordered-async (A-5/R-5), uncontended transitions keep today's synchronous semantics. Rationale: it is the least-blocking correct option and the two alternatives self-deadlock, re-verified against source 2026-07-17 -- the hold-timer callback fires `fsm.Event` while holding `s.mu` (session.go:433-435 calls `logFSMEvent`, session.go:690 calls `s.fsm.Event`), and the to-Established callback needs `s.mu.RLock` via `session.Negotiated()` (peer_run.go:340 → session.go:515-518), so alternative (b) blocking-ticket order deadlocks and alternative (a) holding `f.mu` across the callback self-deadlocks on the non-reentrant RWMutex (fsm.go:139-141). Thomas: override if wrong.

→ AUTONOMOUS DEFAULT (2026-07-17) [STAKES: scope]: **D-5 landing order = ACCEPT the recorded order (4 → 2 → 3 → 1)**. Rationale: the conservative scheduling default is the smaller self-contained path; the recorded rationale (leads 4 and 2 independent + lowest risk; lead 3 before lead 1) stands, and R-3's mitigation (re-run the Phase 1 verification pass after any rebase; the compiler enumerates every `MessageCallback` call site) already covers concurrent sibling edits to `fsm/fsm.go`/`reactor/session.go`/`reactor_notify.go` without a scheduling decision from Thomas. Thomas: override if a specific sibling-spec interleave is required.

## Implementation Steps

### Implementation Phases
1. **Phase: Verify (MANDATORY FIRST)** -- re-confirm the four verdicts on current HEAD (sibling specs touch fsm.go/session.go/reactor_notify.go; cites may drift), then write the failing reproductions: `TestFSMCallbackOrderingRace` (ordering assertion; `-race` alone may not flag lead 1), `TestPeerSessionSentPathRace` and `TestDynamicPeerSettingsRace` (`-race` flags leads 2-3), `TestSessionDuplicateOriginKeepFirst` (observes today's silent MP skip / indexed-consumer error). Any lead that fails to reproduce AND survives a fresh producer read gets documented verified-benign (AC-5 pattern) with no code change.
2. **Phase: duplicate-attribute keep-first (lead 4)** -- validator duplicate recording, `enforceRFC7606` strip via rebuild, decode alignment per D-4b decision, `.ci` test; `TestDuplicateAttributeIndexStillRejects` pins the untouched guard.
3. **Phase: sent-path session read (lead 2)** -- `MessageCallback` signature + six call sites + `notifyMessageReceiver` consumption; delete :360-363 lookup; add AC-5 comments on the received-path reads.
4. **Phase: dynamic settings guard (lead 3)** -- accessors + guarded write; sibling call-site audit: grep every `.PeerAS`, `.ImportFilters`, `.ExportFilters` read in reactor and plugins, classify same-goroutine vs cross-goroutine, convert the cross-goroutine set, justify each exemption in the commit message.
5. **Phase: FSM transition queue (lead 1)** -- queue in `fsm.go`, ordering tests green, teardown-vs-establish stress test for A-2/A-5, `ze-perf` before/after for R-1.
6. **Full verification** -- `make ze-verify` including `-race`.
7. **Complete spec** -- audit tables, `plan/learned/NNN-<name>.md`, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every confirmed lead has a failing-then-passing reproduction (ordering test or `-race` test) |
| Correctness | No new lock-order edge (transition queue non-blocking; `p.mu` reads only under existing `r.mu` -> `p.mu` order); RIB `peerMu` -> `shard.mu` untouched; keep-first strips bytes, does not merely skip validation |
| No fabrication | Benign findings (received-path `peer.session` reads) documented as verified-benign with the same-goroutine justification, not silently dropped |
| Sibling call-site audit | All `Event` callers reviewed for A-5; all mutable-settings readers converted or exempted with reasons |
| Fail-closed guards | `ensureIndexLocked` duplicate rejection unchanged and still tested from its entry points |
| Registration over hardcoding | N/A -- concurrency internals (`ai/rules/plugin-self-containment.md`) |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated (or documented benign)
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests, including `-race`)
- [ ] Registration over hardcoding respected (N/A stated)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for duplicate attributes
- [ ] Reproduction (ordering or `-race`) for each confirmed lead

## Notes
- Skeleton captured from the 2026-07-16 repository audit second BGP pass. The design-phase
  verification (2026-07-16) read every producer and confirmed all four leads REAL; per-lead
  evidence is inline in Task and Current Behavior. Only line-cite drift found: session.go
  timer callbacks are :433-435 and :445 (audit said :432-434,444). The audit's proposed fix
  target `internal/core/bgp/attribute/wire.go` was rejected in favor of fixing the RFC 7606
  boundary (D-4).
- Open items for Thomas before `ready`: D-4b (decode alignment), D-1 trade-off acceptance
  (contended transitions become ordered-async; A-5/R-5), and the D-5 landing order given
  sibling specs on `fsm/fsm.go`, `reactor/session.go`, `reactor_notify.go`.
  → RESOLVED (2026-07-17, autonomous defaults): all three answered in "Resolved open
  decisions" under Key Design Decisions -- D-4b = YES (align decode), D-1 trade-off = ACCEPT
  the recorded non-blocking-FIFO design, D-5 landing order = ACCEPT the recorded 4→2→3→1
  order. No open question remains; Thomas may override any of the three.
- → SOURCE RE-VERIFICATION (2026-07-17): all four leads re-confirmed REAL against current
  HEAD. Producers verified: lead 1 fsm.go:132-143 (`change` unlocks around the callback),
  peer_run.go:332-442 (callback closure) + :353-355 (resolveDynamicPeerSettings call) + :505
  (`session.Run`), session.go:433-435 / :690 (hold-timer fires `fsm.Event` under `s.mu`),
  session.go:515-518 (`Negotiated` takes `s.mu.RLock`); lead 2 reactor_notify.go:360-363
  (unlocked sent-path double-read) and :410-414 / :437-441 (received-path, benign, same
  goroutine), peer_run.go:223-225 / :249-251, forward_pool.go:114-116 (correct pattern),
  session.go:276-284 (`sentSourcePeerStr` writeMu contract); lead 3 reactor_dynamic.go:316,
  329,332 (unlocked writes), routerid_unique.go:58 (unlocked `PeerAS` read) + :61-63 (correct
  session RLock), peer.go:337-339 (`Settings()` raw pointer); lead 4 rib_structured.go:94-96,
  210-211,240-241 (silent MP skip on index error), wire.go:308-311 (`ensureIndexLocked`
  duplicate rejection). LINE-CITE DRIFT since 2026-07-16 (recent RFC 7606 5.3 MP-NLRI commit
  fa244032d): `message/rfc7606.go` shifted ~+35 lines -- the non-MP duplicate `continue` is
  now :281-283 (Task cites :245-247) and the MP-duplicate session-reset is now near :326+
  (Task cites :280-285); `reactor/session_validation.go` `isIBGP`'s read of
  `s.settings.PeerAS` is now :99 (cited :73) and `enforceRFC7606` is at :34 (cited :26).
  Behaviors are unchanged; per R-3 the implementer MUST re-grep these two files before fixing.

## Implementation Audit

| Requirement | Status | Evidence |
|-------------|--------|----------|
| Lead 1: FSM ordered non-blocking transition queue | Done | `fsm/fsm.go` `change`/drain (`f.pending`/`f.draining`); `fsm/fsm_ordering_test.go` (`TestFSMCallbackOrderingRace`), `reactor/peer_established_ordering_test.go` (`TestPeerEstablishedTeardownOrdering`); reviewer confirmed deadlock-free (per-session FSM + draining flag mutual-exclusion, no `f.mu->s.mu` cycle) |
| Lead 2: source-peer via `MessageCallback` arg, delete `peer.session` re-read | Done | `session.go` (`MessageCallback` +`sentSourcePeerStr`), 5 sent sites in `session_write.go` under `writeMu`, received `""` in `session_read.go`; `reactor_notify.go` re-read deleted + AC-5 comments; `reactor/reactor_notify_test.go` (`TestPeerSessionSentPathRace`) `-race` clean |
| Lead 3: `p.mu`-guarded write + accessors; cross-goroutine readers converted | Done | `reactor_dynamic.go:346-354` write under `p.mu`; `peer.go` accessors `PeerAS/ImportFilters/ExportFilters/IsIBGP/IsEBGP`; readers converted in routerid_unique/reactor_api/reactor_api_batch/reactor_api_forward*/policy_dryrun/reactor_peers/peer_initial_sync; under-lock sites (`peer_initial_sync.go:217,363`) left direct (RLock-would-deadlock); `reactor/reactor_dynamic_race_test.go` (`TestDynamicPeerSettingsRace`) 30/30 `-race -cpu=1` |
| Lead 4: RFC 7606 keep-first strip at the boundary; guard unchanged | Done | `message/rfc7606.go` records dup byte ranges; `session_validation.go` strips via `RebuildUpdateBody`; `cli/decode_update.go` aligned (D-4b); `ensureIndexLocked` unchanged; `rfc7606_test.go`/`session_validate_test.go`/`wire_test.go`/`cli/decode_duplicate_test.go`/`test/decode/bgp-update-duplicate-origin.ci` |
| AC-5: received-path reads documented verified-benign | Done | same-goroutine justification comments in `reactor_notify.go` (received-direction reads); no lock added |
| Pre-existing `sessionLogger` race fixed at root (not parked) | Done | `session.go` atomic-backed provider + `Run` `defer StopAll/stopSendHoldTimer`; `session_logger_swap_test.go`; `make ze-race-reactor` DATA RACE=0 |

## Goal Validation

| Goal | Evidence |
|------|----------|
| All four concurrency/consistency leads fixed at the owning layer, each from a reproduction | Per-lead RED-then-GREEN reproductions listed in the audit; the two `-race` reproductions (AC-2/AC-3) and the ordering test (AC-1) plus the crafted duplicate-ORIGIN decode/session tests (AC-4) |
| No new data race; reactor race gate clean | `make ze-race-reactor` (count=20): `DATA RACE count: 0`, both packages `ok`, ~101s (no hang => no deadlock). `go test -race -count=1 ./reactor/...` and `./fsm/...` `ok` |
| No new lock-order edge | Reviewer traced: transition queue non-blocking; `s.mu->p.mu` reads only on existing edge (`reactor_api.go` `OnPeerClosed` reads `PeerAS()` after `peer_run.go:423` `RUnlock`); `peerMu->shard.mu` untouched |
| Protocol-observable keep-first works end to end | `test/decode/bgp-update-duplicate-origin.ci` asserts `origin:"igp"` (first), discriminating against last-write-wins; `TestDuplicateAttributeIndexStillRejects` keeps the fail-closed guard |
| Changed packages build/lint/test clean | `go build ./...` RC=0; `go vet ./internal/component/bgp/...` clean; `make ze-lint-changed` 0 issues; `make ze-verify-changed` all changed BGP packages `ok` |

## Review Gate

- Independent adversarial review by subagent `a1366a114fa2abcb9` over the full changeset: **0 BLOCKER**. Two ISSUEs (flaky `TestDynamicPeerSettingsRace`; incomplete `IsIBGP` cross-goroutine sibling audit) were fixed by the author context and the same reviewer re-confirmed **CLEAN**. Coordinator statically cleared the fix delta and the `reactor_api.go` `OnPeerClosed` residual conversion, and re-ran the gates (`ze-race-reactor` DATA RACE=0; `TestDynamicPeerSettingsRace` 30/30 `-race -cpu=1`).
- Artifact: `review_gate.py check` CLEAN over all 39 changeset code/test files (hashes match).
- Residual (non-blocking, tracked): reviewer NOTE `forward_rs.go` RS fast path does not set `sentSourcePeerStr` (attribution gap, value preserved exactly as before; not a regression) — noted in learned 1245 for a future reactor-hardening pass.

## Pre-Commit Verification

| Table | Item | Verification |
|-------|------|--------------|
| Files Exist | `fsm/fsm_ordering_test.go`, `reactor/{peer_established_ordering_test.go,reactor_notify_test.go,reactor_dynamic_race_test.go,session_logger_swap_test.go}`, `cli/decode_duplicate_test.go`, `test/decode/bgp-update-duplicate-origin.ci` | `git status` shows all created (untracked) + 32 modified `.go` |
| AC Verified | AC-1 | `TestFSMCallbackOrderingRace` + `TestPeerEstablishedTeardownOrdering` green under `-race`; reviewer deadlock analysis CLEAN |
| AC Verified | AC-2 | `TestPeerSessionSentPathRace` green under `-race`; 5 sent sites pass `sentSourcePeerStr` under `writeMu` (grepped) |
| AC Verified | AC-3 | `TestDynamicPeerSettingsRace` 30/30 `-race -cpu=1`; complete `IsIBGP`/`IsEBGP`/`PeerAS` sibling audit (reviewer re-grep) |
| AC Verified | AC-4 | `TestRFC7606DuplicateOriginRecorded`, `TestEnforceRFC7606DuplicateRebuild`, `TestDuplicateAttributeIndexStillRejects`, `bgp-update-duplicate-origin.ci` all green |
| AC Verified | AC-5 | received-path comments present in `reactor_notify.go`; no lock added |
| Wiring Verified | duplicate-ORIGIN decode `.ci` | `test/decode/bgp-update-duplicate-origin.ci` exercises `ze bgp decode` end to end (keep-first output) |
| Verify | full-suite status | `make ze-verify-changed`: all changed BGP packages `ok`; functional+ExaBGP pass EXCEPT two ATTRIBUTED pre-existing reds — install/kernel suite (darwin environmental: no Docker/`tools/kernel-builder`, `ModuleNotFoundError 'run'`; changeset touches no install code) and `plugin` test 223 `forward-mpreach-nexthop-self-two-peer` (F2 harness `parse_error` for a vacuous peer block, tracked open deferral -> `spec-fixit-redistribute-establishment-stall` F4; changeset touches neither the `.ci` nor the harness). `make ze-race-reactor` DATA RACE=0. A single non-reproducing load-sensitive `ze-race-reactor` flake was static-cleared (site: `OnPeerClosed`, `p.mu` released before call) and did not recur across count=1 + count=20 reruns. Commit uses `--unverified` for the two attributed reds. |
| Assumptions | A-1..A-5 | A-1/A-2/A-3/A-4 confirmed (reproductions + reviewer); A-5 (contended async ordering) validated: reviewer confirmed the to-Established callback is always drained on the read goroutine, so no UPDATE-before-established gap |
