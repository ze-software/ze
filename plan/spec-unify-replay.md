# Spec: unify-replay

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `DESIGN-REVIEW.md` finding 2 (Late-join replay row) and section 5 (protocol-agnostic core carrying protocol-specific shape)
4. `internal/core/events/typed.go` (Event[T] vs SignalEvent), `internal/core/redistevents/events.go`, `internal/component/bgp/plugins/rib/events/events.go`, `internal/component/sysrib/events/events.go`

## Task

DESIGN-REVIEW finding 2 flags the "late-join replay" concern as implemented with two structurally different mechanisms for the same problem: a late-joining consumer needs the existing table replayed. Investigation shows there are in fact THREE implementations:

1. `ribevents.ReplayRequest` - a payload-less `RegisterSignal` broadcast. sysrib emits it on startup; the RIB replies by walking its entire best-path table and broadcasting `BestChangeBatch{Replay:true}` to ALL best-change subscribers.
2. `sysribevents.ReplayRequest` - a second payload-less `RegisterSignal` broadcast, structurally identical to (1) one pipeline hop downstream. The FIB backends (kernel, vpp, p4) emit it on startup; sysrib replies by walking its entire system-best table and broadcasting `BestChangeBatch{Replay:true}` to ALL best-change subscribers.
3. `redistevents.ReplayRequestEvent` - a payload-carrying event whose `ReplayRequest{ReplayID uint64}` opaque correlation token lets the redistribute orchestrator target the ONE newly-established BGP peer, spanning many independent producers (static, connected, as112, l2tp, and test fakes).

The task: inventory the features of each, pick the canonical shape, port the gap features onto it, and converge the losing sites onto one replay REQUEST shape plus one response marker convention. This is a refactor: preserve all externally observable behavior. Where a full handler merge would couple isolated components, say so and recommend the narrower (vocabulary-level) unification instead.

## Required Reading

### Architecture Docs
- [ ] `DESIGN-REVIEW.md` - finding 2 (Late-join replay) and section 5 (redistevents leaf carrying BGP-only ReplayID)
  → Decision: the two shapes are flagged as one problem solved twice; the review's own note (events.go:59-63) contrasts broadcast vs token correlation, so the intended fix is one vocabulary, not two.
  → Constraint: `redistevents` is a core leaf and must stay protocol-agnostic; any shared replay type must not import BGP/peer concepts.
- [ ] `docs/architecture/api/process-protocol.md` - event-bus typed handles, JSON marshaling contract for external plugin processes
  → Constraint: the JSON `replay` / `replay-id` field on a batch is a wire contract with external plugin processes; renaming or removing a JSON tag is an external-visible change, so the marker convention must preserve the emitted JSON.
- [ ] `ai/rules/plugin-self-containment.md` - remove a plugin and all its features vanish; no plugin spelling in shared packages
  → Constraint: merging the three handler bodies into one would pull RIB best-path, sysrib system-best, and redistribute producer state into a single owner; keep handlers per-subsystem, unify only the shared request/response vocabulary.
- [ ] `ai/rules/module-tiers.md` - dependency direction dictates package placement
  → Decision: the shared replay-request payload + marker convention belongs in a core leaf (`internal/core/...`), imported by every hop, never the reverse.

**Key insights:**
- The genuine deletable redundancy is `ribevents.ReplayRequest` == `sysribevents.ReplayRequest`: two literal copies of the same payload-less broadcast full-table pattern at adjacent pipeline hops.
- The token-correlated `redistevents` shape is strictly more expressive: broadcast is the special case where the token addresses "everyone."
- The `Replay bool` on both BestChangeBatch types is write-only in production (set, never read by an in-process consumer); the redistevents side already derives replay-ness from the token via `IsReplay()` (single source of truth).

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/core/events/typed.go` - defines `Event[T]` (payload-carrying) and `SignalEvent` (payload-less) handles; `Register[T]` and `RegisterSignal` register the (namespace, eventType) pair with a type; the bus delivers `any` and the handle type-asserts once.
- [ ] `internal/core/redistevents/events.go` - the token-correlated winner shape: `ReplayRequest{ReplayID uint64}` payload (events.go:64-68), shared `ReplayRequestEvent` handle (events.go:74), `RouteChangeBatch.ReplayID` echo field (events.go:161-168), `IsReplay()` derived marker (events.go:195). The doc comment (events.go:59-63) explicitly contrasts the two shapes.
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - `ReplayRequest = RegisterSignal("bgp-rib","replay-request")` (events.go:111), payload-less; `BestChangeBatch.Replay bool` marker (events.go:91).
- [ ] `internal/component/bgp/plugins/rib/rib.go` - subscribes the RIB handler `ribevents.ReplayRequest.Subscribe(eb, r.replayBestPaths)` (rib.go:634).
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `replayBestPaths()` (rib_bestchange.go:1102-1170) walks the entire best-path table and emits one `BestChangeBatch{Replay:true}` per family to all subscribers.
- [ ] `internal/component/sysrib/events/events.go` - second signal: `ReplayRequest = RegisterSignal("system-rib","replay-request")` (events.go:83); `BestChangeBatch.Replay bool` (events.go:73).
- [ ] `internal/component/sysrib/sysrib.go` - emits `ribevents.ReplayRequest` upstream on the forked path (sysrib.go:892); subscribes its own `sysribevents.ReplayRequest.Subscribe(eb, s.replayBest)` (sysrib.go:900); `replayBest()` walks `s.best` and broadcasts `BestChangeBatch{Replay:true}` (sysrib.go:783-822).
- [ ] `internal/plugins/fib/kernel/fibkernel.go` - FIB consumer emits `sysribevents.ReplayRequest` on startup (fibkernel.go:463). Same at `internal/plugins/fib/vpp/fibvpp.go:388` and `internal/plugins/fib/p4/fibp4.go:172`.
- [ ] `internal/component/bgp/plugins/redistribute_egress/replay.go` - the orchestrator: `replayCoordinator.onPeerUp` allocates a monotonic `replayID`, records `replayID -> peer` (bounded, 30s TTL, 4096-cap), emits `ReplayRequest{ReplayID}` (replay.go:131); `handleReplayBatch` maps `lookupTarget(b.ReplayID)` back to the one peer and injects only to it (replay.go:201-249).
- [ ] `internal/component/bgp/plugins/redistribute_egress/register.go` - peer up/down edges call `coord.onPeerUp` / `coord.onPeerDown` (register.go:93-109), the trigger for the token path.
- [ ] `internal/plugins/static/register.go` - a producer subscriber: `redistevents.ReplayRequestEvent.Subscribe(bus, ... rm.reemitAll(r.ReplayID))` (register.go:100). Same pattern in `internal/plugins/connected/connected.go:199`, `internal/plugins/as112/redistribute.go:263`, `internal/component/l2tp/subsystem.go:166`.

**Behavior to preserve:**
- RIB late-joiner (sysrib) receives the full best-path table on startup via `bgp-rib/best-change` batches; no per-prefix loss, one batch per family.
- sysrib late-joiner (each FIB backend) receives the full system-best table on startup via `system-rib/best-change` batches.
- Redistribute late-join replay targets ONLY the newly-established peer (never re-injects to already-up peers), correlates concurrent peer-ups by distinct token, drops unknown/expired tokens with a warn, replays only Add entries, and bounds the pending map (30s TTL, 4096 cap).
- The JSON wire contract to external plugin processes: `best-change` batches still carry a `replay` marker, `route-change` batches still carry `replay-id`; a `route-change` with token 0 is still a normal incremental change (not a replay).
- Loop prevention: a bgp-sourced batch is never redistributed back into bgp.

**Behavior to change:**
- None - internal refactor, behavior preserved. The only observable difference is that the two payload-less broadcast requests become payload-carrying requests whose token addresses "broadcast"; on the JSON wire this adds a field with the broadcast sentinel and changes nothing an existing consumer branches on.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A late-joining consumer subscribes to a producer's change event and then asks for a replay of existing state. Three concrete triggers exist today:
  - Consumer startup, forked path: sysrib subscribes `bgp-rib/best-change` then emits `bgp-rib/replay-request` (sysrib.go:886-894).
  - Consumer startup: a FIB backend subscribes `system-rib/best-change` then emits `system-rib/replay-request` (fibkernel.go, fibvpp.go, fibp4.go).
  - BGP peer down->up edge: `redistribute_egress` register.go:93-109 calls `onPeerUp`, which emits `redistribute/replay-request` carrying a fresh token.

### Transformation Path
1. Trigger fires a replay-request event on the EventBus (payload-less signal today for hops 1-2; token-carrying for hop 3).
2. The producer's subscribed handler runs: walks its authoritative state (RIB best-path table, sysrib system-best table, or a producer's live route set).
3. The handler emits change batch(es) marked as a replay (`Replay:true` bool today for hops 1-2; echoed `ReplayID` for hop 3).
4. The bus delivers the batch: in-process subscribers synchronously, external plugin processes via JSON marshaling.
5. The consumer applies the batch. For hops 1-2 it is an idempotent upsert broadcast to all subscribers; for hop 3 the orchestrator's `handleReplayBatch` maps the token to one peer and injects only there.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RIB ↔ sysrib | `bgp-rib/replay-request` up, `bgp-rib/best-change` batches down | [ ] |
| sysrib ↔ FIB backend | `system-rib/replay-request` up, `system-rib/best-change` batches down | [ ] |
| redistribute orchestrator ↔ producers | `redistribute/replay-request{token}` fan-out, `route-change{ReplayID}` fan-in | [ ] |
| Engine ↔ external plugin process | JSON marshaling of the batch (`replay` / `replay-id` tags) | [ ] |

### Integration Points
- `internal/core/events` typed-handle infrastructure (`Register[T]` / `RegisterSignal`) - the shared vocabulary lands as a new core leaf payload type consumed by all three hops.
- `internal/core/redistevents` - already holds the winner shape; the shared payload type either lives here (generalized name) or in a sibling core leaf the three namespaces import.
- Per-hop typed handles (`ribevents`, `sysribevents`, `redistevents`) remain distinct because each hop has a different direction and subscriber set; they change from `RegisterSignal` to `Register[*replay.Request]` where they are the broadcast case.

### Architectural Verification
- [ ] No bypassed layers (each hop's request still flows consumer->producer; each hop's replay still flows producer->consumer on the same event pair).
- [ ] No unintended coupling (the three handler bodies stay in their own components; only the request payload type and the response marker convention are shared).
- [ ] No duplicated functionality (the two identical payload-less broadcast requests collapse onto one shared payload type; the two write-only `Replay bool` fields collapse onto the token-derived marker).
- [ ] Zero-copy preserved where applicable (batch pooling in `redistevents/pool.go` unchanged; the shared token is a value type, no new allocation).
- [ ] Registration over hardcoding — each hop keeps registering its own typed handle via `events.Register`/`RegisterSignal` under its own namespace; the shared replay-request payload type is discovered through the events type registry, and no core/shared struct gains a per-hop field, switch case, or factory (small-core/registration; `ai/rules/plugin-self-containment.md`).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `Replay bool` on `ribevents`/`sysribevents` BestChangeBatch is write-only in production (no in-process consumer branches on it). | grep of `.Replay` reads across `internal/`: only `Replay: true` writes at rib_bestchange.go:1161 and sysrib.go:813; no consumer read found. | If a consumer secretly branches on it, replacing the bool with a token marker changes behavior. | grep for `.Replay` reads + read every `best-change` subscriber handler; add a characterization test that asserts consumer behavior is identical for replay vs incremental batches. | **confirmed** — grep of `.Replay` field reads across `internal/`: the only read was `rib_bestchange_test.go:930` (a test); no production consumer branches on it. `TestBroadcastReplayCharacterization` (sysrib) asserts processEvent output is identical for replay-marked vs incremental input. |
| A-2 | The three replay-request hops must stay distinct (namespace, eventType) handles; they cannot share one handle. | Directions differ: bgp-rib request flows sysrib->rib; system-rib request flows FIB->sysrib; redistribute request flows orchestrator->many producers. Sharing one handle would cross-trigger unrelated producers. | If they could share one handle the unification would be simpler (one event, not three). | Trace subscriber sets per namespace; confirm no hop subscribes to another hop's request. | **confirmed** — three distinct `(namespace, eventType)` handles kept: `bgp-rib`/`replay-request` (emit sysrib, sub rib), `system-rib`/`replay-request` (emit fib×3, sub sysrib), `redistribute`/`replay-request` (emit orchestrator, sub producers). All bind `*replay.Request` but stay separate handles; no hop subscribes to another's request. |
| A-3 | A reserved broadcast sentinel token can coexist with the existing `ReplayID==0 means not-a-replay` convention. | redistevents.go:161-168: 0 is the incremental default; the orchestrator only treats nonzero as replay. | If 0 must stay "not a replay" AND double as broadcast, the two meanings collide and mis-route. | Introduce a distinct reserved constant (not 0) for broadcast; unit test that incremental (0), broadcast (sentinel), and targeted (N) are three disjoint cases. | **confirmed** — `replay.Broadcast = math.MaxUint64`, distinct from 0 and from the orchestrator's monotonic-from-1 targeted tokens. `TestReplayTokenDisjointCases` asserts the three cases are pairwise distinct and `IsReplay` classifies each correctly. Token 0 stays "not a replay". |
| A-4 | The JSON tags `replay` (best-change) and `replay-id` (route-change) are the external contract and must be preserved for external plugin processes. | events.go json tags; process-protocol.md documents JSON as the external contract. | Renaming a tag breaks external plugin decoders. | Keep both tags; snapshot the marshaled JSON before/after in a unit test. | **confirmed** — `TestReplayBatchJSONTagsStable` (rib/events, sysrib/events, redistevents, replay leaf) round-trips the wire literals: best-change still marshals `"replay":true` (derived from the token via `MarshalJSON`, omitted when incremental), the request payload still marshals `"replay-id"`. `RouteChangeBatch` fields (Go-name JSON) are untouched. process-protocol.md does not spell these tags, so no doc contract text changed. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Over-unification couples RIB, sysrib, and redistribute into one replay owner, violating component isolation. | `make ze-tier-check` fails, or a handler needs to import another component's state. | Unify only the payload type + marker convention (leaf), never the handler bodies; keep three per-hop handles. |
| R-2 | Broadcast sentinel collides with the incremental `ReplayID==0` semantics and mis-routes normal changes as replays. | Incremental route-change batches start getting treated as replays in a test. | Reserve a distinct nonzero sentinel; keep 0 == not-a-replay; disjoint-case unit test (A-3). |
| R-3 | Changing the two `RegisterSignal` requests to payload-carrying alters the JSON wire seen by external plugin processes. | External-plugin decode test or snapshot diff shows a new/renamed field. | Preserve existing JSON tags; the broadcast case marshals the sentinel under a stable tag; keep `SignalEvent` back-compat if the field cannot be added safely. |
| R-4 | The refactor is churn for churn's sake if the shared vocabulary is not actually reused. | Reviewer asks "what did merging buy us?" | Concrete payoff: delete one of the two identical broadcast patterns and the two write-only bools; one mental model for all replay. If payoff is judged too small, fall back to Known Limitation "keep three, document the boundary." |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP peer down->up edge | → | `redistribute_egress` `onPeerUp` -> `ReplayRequest{token}` -> producer `reemitAll` -> `handleReplayBatch` targets one peer | `TestOrchestratorTargetsNewPeerOnly` (redistribute_egress/replay_test.go) |
| BGP peer establishes with pre-injected static routes | → | shared replay vocabulary end to end, one peer targeted | `test/plugin/redistribute-late-join-configadd.ci` |
| FIB backend subscribes then requests replay | → | `system-rib/replay-request` (broadcast token) -> `sysrib.replayBest` -> `system-rib/best-change{replay}` | `test/plugin/fib-rib-event.ci` |
| sysrib subscribes then requests replay (forked path) | → | `bgp-rib/replay-request` (broadcast token) -> `rib.replayBestPaths` -> `bgp-rib/best-change{replay}` | `test/plugin/rib-reconnect-simple.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A single shared replay-request payload type exists in a core leaf, carrying an opaque correlation token. | `ribevents`, `sysribevents`, and `redistevents` all reference it; no `RegisterSignal("*","replay-request")` remains. |
| AC-2 | A FIB backend requests replay (broadcast case). | sysrib walks its full system-best table and delivers it; the request carries the reserved broadcast sentinel, not a per-target token; behavior identical to today. |
| AC-3 | sysrib (forked path) requests replay from the RIB (broadcast case). | RIB walks its full best-path table and delivers it; identical to today. |
| AC-4 | A BGP peer establishes with two other peers already up (targeted case). | Only the new peer receives the replayed redistributed routes; the two already-up peers receive nothing. |
| AC-5 | Two peers establish concurrently. | Distinct tokens; each peer receives only its own replay; no cross-delivery. |
| AC-6 | A normal incremental route-change batch is emitted (token 0). | Treated as incremental, never as a replay; disjoint from both broadcast sentinel and targeted token. |
| AC-7 | The replay marker on a change batch is queried. | Derived from the token (single source of truth); the two former `Replay bool` fields are gone or delegate to the token; emitted JSON tags unchanged. |
| AC-8 | An unknown/expired token returns in a batch. | Dropped with a warn; never mis-delivered (unchanged from today). |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOrchestratorFiresReplayOnPeerUp` | `internal/component/bgp/plugins/redistribute_egress/replay_test.go` | targeted-token path still fires on peer-up (regression) | existing |
| `TestOrchestratorTargetsNewPeerOnly` | `internal/component/bgp/plugins/redistribute_egress/replay_test.go` | only the new peer targeted (AC-4) | existing |
| `TestOrchestratorIncrementalUnchangedByReplayID` | `internal/component/bgp/plugins/redistribute_egress/replay_test.go` | token 0 stays incremental (AC-6) | existing |
| `TestReplayTokenDisjointCases` | `internal/core/redistevents/events_test.go` (or shared leaf) | incremental(0), broadcast(sentinel), targeted(N) are three disjoint cases (AC-6, A-3) | new |
| `TestBroadcastReplayCharacterization` | `internal/component/sysrib/sysrib_test.go` | consumer applies replay batch identically to incremental (A-1, AC-2) | new |
| `TestRIBBroadcastReplayCharacterization` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | RIB full-table replay unchanged after vocabulary swap (AC-3) | new |
| `TestReplayBatchJSONTagsStable` | `internal/core/redistevents/events_test.go` and per-hop events_test.go | `replay` / `replay-id` JSON tags unchanged (A-4, AC-7) | new |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `redistribute-late-join-configadd` | `test/plugin/redistribute-late-join-configadd.ci` | a peer establishing after routes are injected receives them, and only that peer does | existing - must still pass |
| `fib-rib-event` | `test/plugin/fib-rib-event.ci` | a FIB backend gets the full system-best table replayed on startup | existing - must still pass |
| `rib-reconnect-simple` | `test/plugin/rib-reconnect-simple.ci` | sysrib reconnecting gets the full best-path table replayed | existing - must still pass |
| `redistribute-as112-announce` | `test/plugin/redistribute-as112-announce.ci` | multi-producer redistribution unaffected by the marker change | existing - must still pass |

This is an internal refactor: no user-facing behavior change; the existing test suite passes with no regressions.

## Files to Modify
- `internal/core/redistevents/events.go` - generalize `ReplayRequest`/`ReplayID` into the canonical replay vocabulary; add a reserved broadcast sentinel constant distinct from 0; keep `IsReplay()` derived from the token.
- `internal/component/bgp/plugins/rib/events/events.go` - replace `RegisterSignal("bgp-rib","replay-request")` with the shared payload-carrying request handle; retire `BestChangeBatch.Replay bool` in favor of the token-derived marker while preserving the `replay` JSON tag.
- `internal/component/sysrib/events/events.go` - same migration for `system-rib/replay-request` and its `Replay bool`.
- `internal/component/sysrib/sysrib.go` - emit the broadcast-sentinel request (sysrib.go:892) and set the marker from the token in `replayBest` (sysrib.go:813).
- `internal/component/bgp/plugins/rib/rib.go` - subscribe the payload-carrying request (rib.go:634); handler ignores the token in the broadcast case.
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - set the marker from the token in `replayBestPaths` (rib_bestchange.go:1161).
- `internal/plugins/fib/kernel/fibkernel.go`, `internal/plugins/fib/vpp/fibvpp.go`, `internal/plugins/fib/p4/fibp4.go` - emit the broadcast-sentinel request instead of the payload-less signal.
- `internal/component/bgp/plugins/rib/register.go`, `internal/component/sysrib/register.go` - event-type declarations unchanged in name; verify the type registration still matches after the signal->typed swap.

## Implementation Steps

### Implementation Phases

1. **Phase: Wiring / characterization (MANDATORY FIRST)** — capture current behavior before touching the vocabulary.
   - Tests: `TestBroadcastReplayCharacterization`, `TestRIBBroadcastReplayCharacterization`, `TestReplayBatchJSONTagsStable` (assert current JSON and current consumer behavior).
   - Files: sysrib and rib `_test.go`; redistevents `events_test.go`.
   - Verify: characterization tests pass against the current code, pinning the behavior the refactor must preserve.
2. **Phase: Shared vocabulary** — define the canonical replay-request payload type and the reserved broadcast sentinel in the core leaf.
   - Tests: `TestReplayTokenDisjointCases` (incremental 0 / broadcast sentinel / targeted N).
   - Files: `internal/core/redistevents/events.go`.
   - Verify: token disjointness holds; `IsReplay()` stays derived from the token.
3. **Phase: Migrate the two broadcast hops** — swap `RegisterSignal` for the payload-carrying handle at `bgp-rib` and `system-rib`; emit the broadcast sentinel; retire the two `Replay bool` fields onto the token-derived marker while keeping the JSON tags.
   - Tests: characterization tests from Phase 1 still pass; JSON-tag test still passes.
   - Files: rib/events, sysrib/events, rib.go, rib_bestchange.go, sysrib.go, fibkernel.go, fibvpp.go, fibp4.go.
   - Verify: `test/plugin/fib-rib-event.ci` and `test/plugin/rib-reconnect-simple.ci` pass unchanged.
4. **Phase: Reconcile the token hop** — confirm the redistribute orchestrator and every producer subscriber use the shared type unchanged; targeted path untouched.
   - Tests: `TestOrchestratorTargetsNewPeerOnly`, `TestOrchestratorFiresReplayOnPeerUp`, `TestOrchestratorIncrementalUnchangedByReplayID`.
   - Files: redistribute_egress/replay.go and producer subscribers (static, connected, as112, l2tp) - ideally zero change beyond the type's new home.
   - Verify: `test/plugin/redistribute-late-join-configadd.ci` passes.
5. **Functional tests** → run the four referenced `.ci` tests; confirm no regression.
6. **RFC refs** → none (no protocol wire change).
7. **Full verification** → `make ze-verify`.
8. **Complete spec** → fill audit tables, write learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; no `RegisterSignal("*","replay-request")` remains. |
| Feature completeness | Broadcast case (hops 1-2) and targeted case (hop 3) both work end to end through the shared vocabulary. |
| Correctness | Broadcast sentinel, incremental 0, and targeted token are disjoint; unknown/expired token still dropped with a warn. |
| Naming | Shared type is protocol-agnostic (no BGP/peer spelling in the core leaf); JSON tags `replay` / `replay-id` unchanged. |
| Data flow | Three per-hop handles stay distinct; only the payload type and marker convention are shared; handler bodies not merged. |
| Registration over hardcoding | Each hop registers its own typed handle via `events.Register` under its own namespace; no core/shared struct gains a per-hop field, switch, or factory (`ai/rules/plugin-self-containment.md`). |
| Rule: module-tiers | Shared replay type lives in a core leaf; `make ze-tier-check` passes. |
| Rule: no-layering | The two `Replay bool` fields and one of the two identical `RegisterSignal` patterns are fully removed, not left alongside the new convention. |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/core`, `internal/component`, `internal/plugins`)
- [ ] Externally observable behavior preserved (JSON tags stable; broadcast and targeted cases unchanged)
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Tests written
- [ ] Tests FAIL (paste output before implementing each phase)
- [ ] Tests PASS (paste output after)
- [ ] RFC constraint comments added (N/A - no protocol change)
- [ ] Implementation Audit complete

## Review Gate

### Run 1 (initial)
Automated pre-checks:
- `make ze-validate` — initially 4 ISSUEs (pre-existing unwired exports in `rib.go`/`typed.go` surfaced by two cosmetic edits). After the user-approved debt fix: **0 issues, all checks passed**.
- `python3 scripts/dev/audit-test-relaxation.py` — 1 documented `[RELAXED]` in `route_delta_test.go` (removed `routeTypeName` assertion). Verified valid: `routeTypeName` was an exact duplicate of `RouteType.String()`; the `String()` assertion immediately above pins the identical name-per-type mapping, so coverage is retained, not weakened.

Adversarial review of the diff:

| Severity | Finding | Location | Action |
|----------|---------|----------|--------|
| NOTE | (self-review) New replay symbols (`replay.Request`/`Broadcast`/`IsReplay`, `BestChangeBatch.IsReplay`/`MarshalJSON`/`UnmarshalJSON`) all wired: registered by 3 hops, emitted by sysrib/fib, invoked by json + `IsReplay`. | `internal/core/replay`, `*/events/events.go` | none — wired |
| NOTE | `MarshalJSON`/`UnmarshalJSON` round-trip verified (`TestReplayBatchJSONTagsStable`); nil-safe (encoding/json skips nil pointers, never dispatching to the pointer-receiver marshaler); no hot-path regression (marshaling only on the already-JSON cross-process path). | rib/sysrib events | none |
| NOTE | Removed-behavior audit: two `Replay bool` fields + 2 `RegisterSignal` calls removed; the `replay` JSON marker re-established via `MarshalJSON`; full-table-replay behavior preserved (handlers unchanged); coverage increased (3 new tests). | — | none |
| NOTE | Debt fixes (OSPF de-dup, rib/typed unexports, `IsSignal` removal) are behavior-preserving renames / dead-code removal; all 29 affected packages' tests pass. | ospf/spf, rib, events | none |

Result: 0 BLOCKER, 0 ISSUE (after the pre-existing gate debt was fixed per user decision). `make ze-lint` and `make ze-validate` both clean; unit + the 4 referenced functional `.ci` tests pass.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Canonical shape = the token-correlated `redistevents` request + `IsReplay()` marker. | (a) make everything a payload-less broadcast signal; (b) keep all three as-is. | The token shape is strictly more expressive: broadcast is the special case where the token addresses "everyone." The signal shape cannot target one consumer, so it cannot absorb the redistribute per-peer requirement; the reverse absorption is trivial via a reserved sentinel. |
| Unify only the request PAYLOAD TYPE and the response MARKER convention, not the handler bodies. | Merge the three handlers into one replay owner. | The three handlers read three different authoritative stores (RIB best-path, sysrib system-best, N independent producers) at three pipeline hops. Merging bodies would couple isolated components and fail `ze-tier-check` / plugin self-containment. This is the honest, bounded win: one vocabulary, three handlers that speak it. |
| Keep three distinct per-hop `(namespace, eventType)` handles. | One shared replay-request event for all hops. | Directions and subscriber sets differ (sysrib->rib, FIB->sysrib, orchestrator->producers). A single shared handle would cross-trigger unrelated producers. |
| Reserve a distinct nonzero broadcast sentinel; keep token 0 == not-a-replay. | Reuse 0 as broadcast. | 0 already means "normal incremental" in redistevents; overloading it would mis-route incremental batches as replays. |
| Retire the two write-only `Replay bool` fields onto the token-derived marker. | Keep the bool as a parallel marker. | The bool is write-only in production (A-1) and duplicates what the token already encodes; a single source of truth avoids drift, and the JSON tag is preserved for external consumers. |
| Put the shared vocabulary in a NEW neutral core leaf `internal/core/replay` (not in `redistevents`); `redistevents.ReplayRequest` becomes a type alias `= replay.Request`. | Generalize the type in place inside `redistevents` (the spec's default Files-to-Modify). | The spec explicitly offered "a sibling core leaf the three namespaces import" (line 96). DESIGN-REVIEW §5 flags `redistevents` as ALREADY over-loaded with replay semantics that serve one consumer; making rib/sysrib import a package literally documented as "redistribute route producers/consumer" would worsen that smell. A neutral `replay` leaf reads cleanly (`replay.Request`/`replay.Broadcast`/`replay.IsReplay`) and the alias keeps producer/orchestrator churn at zero. |
| Retire the `Replay bool` via a token field (`ReplayID uint64 json:"-"`) plus `MarshalJSON`/`UnmarshalJSON` that map the token to/from the legacy `replay` bool wire. | (a) hand-write no marshaler and keep a stored bool set from the token at emit; (b) custom marshal only. | Best-change round-trips through JSON in-tree (`fibvpp_test.go` `parseBatch`, the forked plugin path), so the `replay` bool must survive decode→encode. The `type alias` marshaler pattern preserves the exact wire while making the token the single source of truth (bool fully removed from the struct), satisfying the Critical Review "no-layering" check. |

## Known Limitations
- This is a PARTIAL unification by design: the RIB best-path walk, the sysrib system-best walk, and the redistribute producer re-emit remain three separate handlers. Only the request payload type and the replay marker convention are shared. If review judges the payoff (deleting one duplicated broadcast pattern plus two write-only bools, and one mental model) too small to justify the churn, the documented fallback is "keep three, document the boundary in `docs/architecture` and cross-link the two broadcast copies," recorded here rather than silently forced into a merge.
- No change to the synchronous fan-out latency concern or the pool-solves-the-cheap-problem observation from DESIGN-REVIEW section 4; those are out of scope for this vocabulary unification.

## Implementation Summary

### What Was Implemented
- New core leaf `internal/core/replay` (`replay.go`): shared `Request{ReplayID uint64 json:"replay-id"}` payload, reserved `Broadcast = math.MaxUint64` sentinel, `IsReplay(token) bool` marker predicate. Pure leaf (imports only `math`).
- `redistevents.ReplayRequest` is now a type alias `= replay.Request`; `ReplayRequestEvent` registers `*replay.Request`; `RouteChangeBatch.IsReplay()` delegates to `replay.IsReplay`. Producers/orchestrator compile unchanged via the alias.
- `ribevents` + `sysribevents`: `ReplayRequest` handles migrated from `RegisterSignal` to `events.Register[*replay.Request]`. `BestChangeBatch.Replay bool` retired to `ReplayID uint64 json:"-"` + `IsReplay()` + `MarshalJSON`/`UnmarshalJSON` that map the token to/from the legacy `replay` bool wire (JSON contract preserved, round-trip safe).
- Broadcast emit sites now carry `replay.Broadcast`: `sysrib.go` (forked-path request to bgp-rib), `fib/{kernel,vpp,p4}` (request to system-rib). Handlers `replayBestPaths`/`replayBest` take `*replay.Request` and stamp `req.ReplayID` onto the batch.
- `events/typed.go` doc examples updated (bgp-rib/replay-request is no longer the `RegisterSignal` example).
- Tests: `TestReplayBatchJSONTagsStable` (rib/events, sysrib/events, redistevents), `TestReplayTokenDisjointCases` + `TestRequestJSONTag` (replay leaf), `TestRIBBroadcastReplayCharacterization` (rib), `TestBroadcastReplayCharacterization` (sysrib); updated `TestRIBReplayOnSubscribe` (`batch.Replay` -> `batch.IsReplay()`).

### Bugs Found/Fixed
- None. Internal refactor; no behavioral defect found or introduced.

### Documentation Updates
- `docs/architecture/core-design.md` "Redistribute Late-Join Replay": rewrote the opening to describe the unified `internal/core/replay` vocabulary shared by all three hops (broadcast vs targeted), replacing the "modeled on `ribevents.ReplayRequest`" framing that implied separate mechanisms; added a source anchor to `internal/core/replay/replay.go`.
- Other docs referencing `redistevents.ReplayRequest` (features.md, guide/plugins.md, functional-tests.md) stay accurate — the alias preserves the spelling. `docs/architecture/api/process-protocol.md` does not spell the `replay`/`replay-id` tags, so no contract text changed.

### Deviations from Plan
- **Package placement:** chose the spec's stated alternative (line 96) "a sibling core leaf the three namespaces import" (`internal/core/replay`) over the default "generalize in `redistevents`", to serve DESIGN-REVIEW §5. Recorded in Key Design Decisions.
- **Marker retirement:** the two `Replay bool` fields are removed via `ReplayID` + custom `MarshalJSON`/`UnmarshalJSON` (not a hardcoded bool), because best-change round-trips through JSON in-tree. Satisfies the Critical Review "no-layering" check (bool fully removed).
- **Phase-1 test naming:** `TestBroadcastReplayCharacterization`/`TestRIBBroadcastReplayCharacterization` are authored in Phase 3 (they exercise the new token path); the refactor-stable `TestReplayBatchJSONTagsStable` plus the pre-existing behavioral tests (`TestProcessEventReplay`, `TestRIBReplayOnSubscribe`, `TestReplayRequestEventRegistered`) served as the Phase-1 behavior pins.
- **Pre-existing gate debt fixed (user-approved, out of the replay scope):** touching `redistevents` (widely imported) pulled ~200 reverse-dep packages into `ze-lint-changed`/`ze-validate` scope, surfacing pre-existing debt in files this change edits. Per user decision ("fix the pre-existing debt too"), fixed here:
  - `goconst` `intra-area` (was red on `main` independent of this change): removed `internal/plugins/ospf/spf/explain.go`'s `routeTypeName`, an exact duplicate of `RouteType.String()` (the test `TestRouteTypeNamesAndRanks` asserted they must agree). `make ze-lint` now 0 issues.
  - `ze-validate` unwired-export ISSUEs surfaced by two cosmetic edits: removed the dead `events.IsSignal` (test-only, redundant with `PayloadInfo`; no signal events remain after this change) and updated its one test to `PayloadInfo`; unexported `rib.go`'s `RunRIBPlugin`→`runRIBPlugin` (every other plugin already uses an unexported `RunEngine` target), `NewRIBManager`→`newRIBManager`, `PeerMeta`→`peerMetadata`; unexported `ospf/spf` `Computer.RunCount`→`runCount` (test-only accessor). `ze-validate` now passes.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Inventory the three replay mechanisms, pick canonical shape | Done | Key Design Decisions | Token-correlated `redistevents` shape chosen; broadcast is the token==sentinel special case |
| Converge onto one replay REQUEST shape | Done | `internal/core/replay/replay.go` (`Request`) | Three hops all register `*replay.Request`; no `RegisterSignal("*","replay-request")` remains |
| One response marker convention | Done | `*.IsReplay()` derived from token | Two `Replay bool` fields removed; marker derived from `replay.IsReplay` |
| Preserve all externally observable behavior | Done | `TestReplayBatchJSONTagsStable`, 4 functional `.ci` pass | JSON `replay`/`replay-id` tags stable; broadcast + targeted cases unchanged |
| Keep handlers per-subsystem (no full merge) | Done | 3 handlers unchanged in body | Only request payload type + marker convention shared |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `internal/core/replay/replay.go`; grep: 0 `RegisterSignal("*","replay-request")` | `Request` in core leaf; ribevents/sysribevents/redistevents all bind `*replay.Request` |
| AC-2 | Done | `fib/*` emit `replay.Broadcast`; `sysrib.replayBest`; `fib-rib-event.ci` PASS | Full system-best table walked; broadcast sentinel; behavior identical |
| AC-3 | Done | `sysrib.go:895` emits Broadcast; `rib.replayBestPaths`; `rib-reconnect-simple.ci` PASS; `TestRIBBroadcastReplayCharacterization` | Full best-path table walked; identical |
| AC-4 | Done | `redistribute_egress/replay.go` (unchanged); `TestOrchestratorTargetsNewPeerOnly`; `redistribute-late-join-configadd.ci` PASS | Only the new peer targeted |
| AC-5 | Done | `replay.go` monotonic per-peer token (unchanged) | Distinct tokens per peer-up; no cross-delivery |
| AC-6 | Done | `TestReplayTokenDisjointCases` | incremental(0)/broadcast(sentinel)/targeted(N) disjoint |
| AC-7 | Done | `ribevents`/`sysribevents` `IsReplay()`+`MarshalJSON`; `TestReplayBatchJSONTagsStable` | Marker token-derived; `Replay bool` removed; JSON tags unchanged |
| AC-8 | Done | `handleReplayBatch` (unchanged) drops unknown/expired token with warn | Unchanged from today |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestOrchestratorFiresReplayOnPeerUp` | Pass (existing) | redistribute_egress/replay_test.go | targeted path regression |
| `TestOrchestratorTargetsNewPeerOnly` | Pass (existing) | redistribute_egress/replay_test.go | AC-4 |
| `TestOrchestratorIncrementalUnchangedByReplayID` | Pass (existing) | redistribute_egress/replay_test.go | AC-6 |
| `TestReplayTokenDisjointCases` | Pass (new) | internal/core/replay/replay_test.go | AC-6, A-3 |
| `TestBroadcastReplayCharacterization` | Pass (new) | sysrib_test.go | A-1, AC-2 (consumer-identical) |
| `TestRIBBroadcastReplayCharacterization` | Pass (new) | rib_bestchange_test.go | AC-3, A-4 |
| `TestReplayBatchJSONTagsStable` | Pass (new) | rib/events, sysrib/events, redistevents events_test | A-4, AC-7 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/core/redistevents/events.go` | Done | ReplayRequest -> alias; IsReplay delegates; import replay |
| `internal/component/bgp/plugins/rib/events/events.go` | Done | Register[*replay.Request]; ReplayID+IsReplay+Marshal/Unmarshal |
| `internal/component/sysrib/events/events.go` | Done | same migration |
| `internal/component/sysrib/sysrib.go` | Done | emit Broadcast; replayBest(req); stamp token |
| `internal/component/bgp/plugins/rib/rib.go` | Done | subscribe unchanged (handler sig now matches) |
| `internal/component/bgp/plugins/rib/rib_bestchange.go` | Done | replayBestPaths(req); stamp token |
| `internal/plugins/fib/{kernel,vpp,p4}` | Done | emit Broadcast |
| `internal/core/replay/replay.go` | Added (deviation) | new shared leaf (sibling-leaf alternative) |
| `internal/component/bgp/plugins/rib/register.go`, `sysrib/register.go` | Verified | no change needed; type registration matches after signal->typed swap |

### Audit Summary
- **Total items:** 8 ACs, 7 tests, 9 file groups
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** package placement (new `replay` leaf) + marker via Marshal/Unmarshal (both recorded as deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/replay/replay.go` | yes | created; `go test ./internal/core/replay/` PASS |
| `internal/core/replay/replay_test.go` | yes | `TestReplayTokenDisjointCases`, `TestRequestJSONTag` PASS |
| `internal/component/bgp/plugins/rib/events/events_test.go` | yes | `TestReplayBatchJSONTagsStable` PASS |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | no replay-request signals remain | `grep -rn 'RegisterSignal(' internal/ pkg/` finds only the `func` def + doc examples; zero replay-request registrations |
| AC-6/AC-7 | disjoint tokens; JSON tags stable | `go test ./internal/core/replay/ ./internal/core/redistevents/ ./internal/component/sysrib/events/ ./internal/component/bgp/plugins/rib/events/` PASS |
| AC-2/AC-3/AC-4 | broadcast + targeted end-to-end | functional `.ci`: `fib-rib-event`, `rib-reconnect-simple`, `redistribute-late-join-configadd` all PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| BGP peer down->up edge (targeted) | redistribute-late-join-configadd.ci | PASS |
| FIB backend requests replay (broadcast) | fib-rib-event.ci | PASS |
| sysrib requests replay from RIB (broadcast) | rib-reconnect-simple.ci | PASS |
| multi-producer redistribution unaffected | redistribute-as112-announce.ci | PASS |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `.Replay` read only in a test; `TestBroadcastReplayCharacterization` shows consumer output identical replay vs incremental |
| A-2 | confirmed | three distinct `(namespace,eventType)` handles kept; no hop subscribes to another's request |
| A-3 | confirmed | `replay.Broadcast=MaxUint64` disjoint from 0 and monotonic targeted tokens; `TestReplayTokenDisjointCases` |
| A-4 | confirmed | `TestReplayBatchJSONTagsStable` round-trips `replay`/`replay-id`; process-protocol.md contract text unchanged |
