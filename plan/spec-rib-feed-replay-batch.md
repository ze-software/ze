# Spec: RIB reconnect replay batching

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/8 |
| Updated | 2026-05-30 |

> Origin: derived from `../bird3-optimisations-report.md` item #12 (kernel/route feed
> deduplication) mapped onto ze. The BIRD finding is "avoid duplicate full-table feeds
> during startup/reload". In ze the directly-verified analogue is the per-route RPC cost
> of replaying a peer's stored Adj-RIB-Out on (re)connection. See Required Reading.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` - small-core + registration, plugin IPC model
4. `internal/component/bgp/plugins/rib/rib.go` (handleState, replayRoutes, updateRoute)
5. `internal/component/bgp/plugins/rib/ribout_entry.go` (collectAllRibOutRoutes, reconstructRoute)
6. `internal/component/bgp/plugins/cmd/update/update_text.go` (handleUpdate, ParseUpdateText)
7. `internal/component/bgp/textparse/keywords.go` (keyword constants, alias map)

## Task

Replaying a (re)connecting BGP peer's stored Adj-RIB-Out is O(routes) in independent
RPC round-trips. On peer-up, `replayRoutes` reconstructs every stored route individually,
formats a text command, and issues **one `UpdateRoute` RPC per route** through the full
dispatch pipeline (JSON marshal/unmarshal, text tokenization, NLRI parsing, batch
announce). For a peer holding a full table this is hundreds of thousands of synchronous
RPC round-trips.

**Cost breakdown (measured from code trace):**
- `reconstructRoute`: 8-12 allocs/route (~1-2us). Decodes pool wire bytes into `*Route`.
- `formatRouteCommand`: 5-20 allocs/route (~2-5us). Builds text command string.
- `updateRoute` RPC: 15-25 allocs/route (~100-200us). JSON marshal/unmarshal, `context.WithTimeout`, text tokenization, `ParseUpdateText`, `AnnounceNLRIBatch`. **This dominates.**

For 100K routes sharing 1K distinct attribute sets: ~10s of blocking replay, of which
~9.5s is RPC overhead. The decode and format costs are real but secondary.

**Goal:** extend the existing `update` command with a new `cursor` encoding mode that
maintains a stateful attribute cursor on the engine side. The plugin establishes full
attribute state once, then sends only deltas when attributes change between groups,
followed by batched NLRIs. This stays within the text command protocol (same `UpdateRoute`
RPC, same dispatcher, same attribute keywords) while drastically reducing per-call
attribute encoding and per-route call count.

### Protocol: `update cursor`

The `handleUpdate` switch (update_text.go:672) currently dispatches on encoding keyword:
`text`, `hex`, `b64`. This spec adds `cursor` as a fourth encoding mode.

```
# First command -- establish full attribute state + announce NLRIs:
peer <addr> update cursor origin igp as-path [65001 65002] med 100 \
  local-preference 200 community [65000:1] next-hop 10.0.0.1 \
  nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24 10.0.2.0/24 ...

# Next group -- only changed attributes, then NLRIs:
peer <addr> update cursor as-path [65001 65003] \
  nlri ipv4/unicast add 10.1.0.0/24 10.1.1.0/24 ...

# NLRIs only (all attributes inherited from cursor):
peer <addr> update cursor nlri ipv4/unicast add 10.2.0.0/24 ...

# Remove an attribute:
peer <addr> update cursor del med nlri ipv4/unicast add 10.3.0.0/24 ...

# Signal completion (clear cursor, free state):
peer <addr> update cursor done
```

**Grammar:**
```
cursor-command  = "update" "cursor" ( cursor-done / cursor-body )
cursor-done     = "done"
cursor-body     = *( attr-set / attr-del ) nlri-section
attr-set        = attr-keyword attr-value        ; replaces attribute in cursor
attr-del        = "del" attr-keyword             ; removes attribute from cursor
attr-keyword    = "origin" / "as-path" / "med" / "local-preference"
                / "community" / "large-community" / "extended-community"
                / "next-hop"
nlri-section    = "nlri" family "add" 1*prefix    ; announce-only, no "del"
```

**Key properties:**
- **Stateful cursor.** The engine maintains current attribute values per (plugin, peer)
  pair. First `update cursor` with attributes initializes the cursor. Subsequent commands
  update only the attributes present: any attribute keyword before `nlri` replaces that
  attribute's value. `del <attr>` removes an attribute. Absent attributes are inherited.
- **Cursor state location.** Package-level `sync.Map` in `cursor.go`, keyed by
  `processName:peerSelector`. `CommandContext` carries `Process.Name()` and `Peer`.
  Cursor state is a `*parsedAttrs` (update_text.go:124), reused directly: same package,
  right fields, `snapshot()` deep-copies. Set = replace field, del = nil field,
  inherit = leave field.
- **Delta encoding.** Consecutive groups sharing most attributes (common: only AS_PATH
  differs between source peers) emit only the changed field. Sorting routes by attribute
  similarity minimizes deltas.
- **Multi-NLRI.** The `nlri <family> add <prefix1> <prefix2> ...` portion reuses existing
  `parseNLRISection` multi-prefix support unchanged.
- **Announce-only.** Cursor mode supports only `nlri <family> add`. Withdrawals are not
  supported because `ribOut` stores only announced routes (withdrawals delete the entry),
  so replay never needs to express withdrawals.
- **AS_PATH handled naturally.** When AS_PATH changes, emit `as-path [65001 65003]`. The
  engine parses and stores it in the cursor. `AnnounceNLRIBatch` rebuilds wire bytes per
  peer's ASN4 capability. No special splitting logic.
- **Fits existing dispatch.** `update cursor` is a new case in `handleUpdate`'s switch.
  Same `UpdateRoute` RPC, same `Dispatcher.Dispatch` path, same YANG registration as
  `update text`. No new command tree, no new RPC methods.
- **Reuses attribute parsing.** Attribute keywords and value parsing (`origin`, `as-path`,
  `med`, `local-preference`, `community`, `large-community`, `extended-community`,
  `next-hop`) use the same `parseCommonAttributeText` logic as `update text`.
- **`del` semantics.** `update text` removed `set`/`del` for attributes (lines 569-571
  reject with migration hint). `update cursor` reintroduces `del <attr>` only:
  - `del` for attribute not in cursor: silent no-op (cursor is "current state", removing
    what is absent is harmless).
  - `del` for mandatory attribute (origin, next-hop): allowed. `DispatchNLRIGroups`
    rejects the incomplete attribute set downstream. In practice, the replay formatter
    never produces this (it formats from valid `*Route` objects).
  - Multiple `del` in one command: supported. `del med del community nlri ...`.
  - `set` is not reintroduced (implicit: specifying an attribute sets it).
- **`done` clears state.** `update cursor done` clears the cursor and frees memory.
  If the peer flaps during replay, the next `update cursor` with attributes replaces
  the stale cursor.
- **Crash cleanup.** If the plugin crashes mid-replay without sending `done`, cursor
  state leaks. `cleanupProcess` (dispatch.go:874) has no extensible cleanup hook.
  Export `ClearProcessCursors(processName string)` from the cursor package and add an
  explicit call in `cleanupProcess`.
- **Concurrency.** Replay sends cursor commands sequentially via `replayRoutes`. The only
  other `updateRoute` caller is `handleRefresh` (BoRR/EoRR markers), which are different
  commands and never touch cursor state. No per-cursor mutex needed beyond `sync.Map`.
- **Works for external plugins.** Same `UpdateRoute` JSON RPC. External Python/Go plugins
  can use the protocol unchanged.

**Wire output is unchanged:** the peer receives the same set of (prefix, attributes,
path-id) tuples. During initial sync, `AnnounceNLRIBatch` queues each NLRI individually
via `QueueAnnounce` (because `ShouldQueue()` is true), so the reactor still builds
per-NLRI wire UPDATEs in the opQueue drain loop.

**Estimated cost:** for 100K routes with 1K distinct attribute groups, well-sorted: ~1K
`UpdateRoute` calls (one full + ~999 with small deltas + done), each ~100-200us =
~100-200ms total. **50-100x improvement** over current ~10s.

**Explicitly out of scope (this spec):** redesigning the live event-driven feed, the
fresh-peer (empty `ribOut`) initial loc-rib feed, and cross-peer shared snapshot.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/core-design.md` - plugin IPC model, JSON-down/text-up
  -> Constraint: `update cursor` stays within the text command protocol. Same RPC, same dispatcher.
- [ ] `ai/rules/plugin-design.md` - EventBus + DirectBridge, typed handles
  -> Constraint: DirectBridge still does JSON marshal/unmarshal per `UpdateRoute` call. ~100-200us per call. Delta encoding reduces call count, not per-call cost.
- [ ] `ai/rules/no-sprintf-alloc.md` - append-based formatting on hot paths
  -> Constraint: delta formatting should use append-based building. Attribute values already have `String()` methods.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4724.md` - Graceful Restart: EOR ordering
  -> Constraint: `update cursor done` must precede `plugin session ready` which gates EOR.
- [ ] `rfc/short/rfc7911.md` - ADD-PATH: path-id in NLRI
  -> Constraint: `path-information` is per NLRI section. Routes with different path-ids need separate `nlri` sections.

**Key insights:**
- Replay is a re-send of already-sent state, not a fresh best-path computation.
- `parseNLRISection` already supports multi-prefix `add`. Reused unchanged by cursor mode.
- `ShouldQueue` is true during replay. Reactor queues per-NLRI. Win is call count.
- `handleUpdate` (update_text.go:662) switches on encoding keyword. Adding `cursor` is a one-line dispatch case.
- `ParseUpdateText` attribute parsing (`parseCommonAttributeText`, `parseNhopFlat`) is reusable. Cursor mode wraps it with persistent state.
- `set`/`del` were removed from `update text` (rejected at lines 569-571). `del` is reintroduced only in `cursor` mode.

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `replayRoutes` (1028) sorts by MsgID, calls `updateRoute` per route, then `plugin session ready`. `updateRoute` (659) is synchronous: `context.WithTimeout(10s)` -> `sdk.Plugin.UpdateRoute` -> DirectBridge JSON marshal/unmarshal -> `Dispatcher.Dispatch` -> tokenize -> handler -> parse -> `AnnounceNLRIBatch` -> JSON result. ~100-200us per call.
  -> Constraint: `pool.RibOut.mu` never under `peerMu`. Replay runs without `peerMu` held.
- [ ] `internal/component/bgp/plugins/rib/ribout_entry.go` - `collectAllRibOutRoutes` (275) walks all families/keys, calls `reconstructRoute` (52) per entry individually (no grouping by `AttrHandle`). Decodes wire bytes into `*Route` with 8-12 allocs per entry. Many routes share the same `AttrHandle` but are decoded redundantly.
  -> Constraint: proposed change groups by `(family, AttrHandle, pathID)` and decodes each distinct `AttrHandle` once.
- [ ] `internal/component/bgp/plugins/cmd/update/update_text.go` - `handleUpdate` (662) switches on encoding: `text`/`hex`/`b64`. `ParseUpdateText` (486) parses attrs then `parseNLRISection`. Rejects `set`/`del` on attrs (line 569-571). `DispatchNLRIGroups` (749) builds `NLRIBatch`, calls `AnnounceNLRIBatch`.
  -> Decision: add `cursor` as fourth encoding case. Reuse attribute parsing; add cursor state and `del` support.
- [ ] `internal/component/bgp/textparse/keywords.go` - keyword constants, aliases, `ResolveAlias`. `kwSet`/`kwDel` defined but only for rejection detection. Attribute keywords: `origin`, `as-path`, `med`, `local-preference`, `community`, `large-community`, `extended-community`, `next-hop`. Aliases: `next`->`next-hop`, `pref`->`local-preference`, `path`->`as-path`, `s-com`->`community`, etc.
  -> Decision: `del` is reintroduced as a cursor-mode keyword. No new attribute keywords needed. Add `done` as cursor-specific keyword.
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - `ShouldQueue()` is true during replay. `AnnounceNLRIBatch` queues per-NLRI via `QueueAnnounce`. opQueue drain builds per-NLRI wire UPDATEs.
  -> Constraint: cursor mode does not change reactor behavior. Win is call count.

**Behavior to preserve:**
- The set of (prefix, attributes, path-id) tuples the peer receives on reconnect (same RIB state).
- `plugin session ready` gating EOR.
- Lock ordering: `pool.RibOut.mu` never under `peerMu`.
- `ribOutEntry` pool refcount discipline.
- Concurrent-reconnect safety (idempotent replay).

**Behavior to change:**
- Internal delivery: fewer `UpdateRoute` calls via cursor mode with delta encoding and multi-NLRI. Observable BGP output unchanged.

## Data Flow (MANDATORY)

### Entry Point
- Peer FSM reaches Established -> state="up" -> bgp-rib `handleState`/`handleStructuredState`.

### Transformation Path
1. `ribOut[peer]` (compact `ribOutEntry` records, shared `pool.RibOut`) -- already populated by `handleSent`.
2. **Changed: grouped collection.** `collectAllRibOutRoutes` groups entries by `(family, AttrHandle, pathID)`. Decodes each distinct `AttrHandle` once. Returns `[]replayGroup{Route, Prefixes, MinMsgID}`.
3. **Changed: sorted for minimal deltas.** Sort groups by attribute similarity. Hash
   is computed over the pool wire bytes (RFC 4271 TLV) with AS_PATH TLV (type code 2)
   stripped: iterate via `attribute.NewAttrIterator`, skip type=2, hash remaining bytes.
   One pass, no decode, no allocation. Sort key: (wire-hash-sans-AS_PATH, AS_PATH wire
   bytes, family, pathID). Stable sort preserves MsgID ordering within equal groups for
   debuggability. Consecutive groups with same hash need only `as-path [...]` delta.
4. **Changed: cursor replay.**
   - First group: `updateRoute(peer, "update cursor <all attrs> nlri <fam> add <p1> <p2> ...")`.
   - Subsequent: compute delta from previous `*Route`. `updateRoute(peer, "update cursor [<changed attrs>] [del <removed>] nlri <fam> add ...")`.
   - Large groups: split when NLRIs would exceed the maximum BGP UPDATE message size (4096 bytes standard, or negotiated extended size); extra commands carry no attrs: `updateRoute(peer, "update cursor nlri <fam> add ...")`.
   - Final: `updateRoute(peer, "update cursor done")`, then `updateRoute(peer, "plugin session ready")`.
5. **Engine side:** `handleUpdateCursor` maintains per-(plugin, peer) cursor state. Applies deltas, snapshots attrs, calls `DispatchNLRIGroups` -> `AnnounceNLRIBatch`. `done` clears cursor.
6. Reactor: `AnnounceNLRIBatch` -> `QueueAnnounce` per NLRI -> opQueue drain -> per-NLRI wire UPDATEs -> EOR.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin (bgp-rib) -> Engine | text command via `UpdateRoute` (`update cursor ...`) | [ ] |
| Pool <-> replay | `pool.RibOut.Get`/`Release` by `AttrHandle` | [ ] |

### Integration Points
- `handleUpdate` (update_text.go:662) - add `case "cursor"` alongside `text`/`hex`/`b64`.
- `parseCommonAttributeText` (update_text.go) - reuse for attribute parsing in cursor commands.
- `parseNLRISection` (update_text_nlri.go:29) - reuse for NLRI parsing. No changes.
- `DispatchNLRIGroups` (update_text.go:749) - reuse for dispatching to `AnnounceNLRIBatch`. No changes.

### Architectural Verification
- [ ] No bypassed layers (cursor commands go through same `UpdateRoute` -> `Dispatch` path)
- [ ] No unintended coupling (rib plugin unaware of reactor; cursor handler unaware of rib plugin)
- [ ] No duplicated functionality (reuses `parseCommonAttributeText`, `parseNLRISection`, `DispatchNLRIGroups`)
- [ ] State lifecycle managed (cursor per plugin+peer, cleared on `done` or replaced on new init)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Peer reconnect with N stored ribOut routes sharing M attribute sets (M < N) | -> | `replayRoutes` (cursor mode) | `TestReplayGroupsByAttrHandle` -- assert <= M+2 `updateRoute` calls (1 full + M-1 deltas + done) and exactly N prefixes delivered |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer reconnects with N ribOut routes sharing M distinct `AttrHandle`s | Replay issues <= M+1 `update cursor` commands (1 full + M-1 deltas + done), not N per-route commands. Each `AttrHandle` decoded once |
| AC-2 | Replay of mixed families / different path-ids | Routes grouped by `(family, AttrHandle, pathID)`. Different families/pathIDs produce separate `nlri` sections. Cursor state carries across |
| AC-3 | Any reconnect | Peer's resulting Adj-RIB-In identical to pre-change per-route replay |
| AC-4 | Any reconnect | `update cursor done` after all NLRIs. `plugin session ready` after `done`. No route after `done` |
| AC-5 | Empty `ribOut` | No cursor commands. Only `plugin session ready` (unchanged) |
| AC-6 | Group's NLRIs would exceed max BGP UPDATE size | Split into multiple `update cursor nlri ...` commands (no attrs, cursor inherited). All prefixes delivered |
| AC-7 | Consecutive groups with identical attributes | No attribute tokens emitted. Command is `update cursor nlri <family> add ...` |
| AC-8 | Attribute removed between groups | `del <attr>` emitted before `nlri`. Cursor reflects removal |
| AC-9 | `update cursor done` | Engine clears cursor state and frees memory |
| AC-10 | Peer flaps during replay (second cursor init before done) | Old cursor replaced by new initialization. No state leak |
| AC-11 | Plugin crashes mid-replay without sending `done` | `cleanupProcess` calls `ClearProcessCursors(processName)`. Cursor state freed. No memory leak |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReplayGroupsByAttrHandle` | `rib_replay_test.go` | AC-1 grouping + single decode + call count |  |
| `TestReplayGroupingSafety` | `rib_replay_test.go` | AC-2 family/pathID separation |  |
| `TestReplayDeltaEncoding` | `rib_replay_test.go` | AC-7 no deltas for identical attrs |  |
| `TestReplayAttrRemoval` | `rib_replay_test.go` | AC-8 `del <attr>` emitted correctly |  |
| `TestReplayOrderingAndDone` | `rib_replay_test.go` | AC-4 done + ready placement |  |
| `TestReplayEmptyRibOut` | `rib_replay_test.go` | AC-5 fresh peer |  |
| `TestReplayLargeGroupSplit` | `rib_replay_test.go` | AC-6 split at BGP UPDATE size limit |  |
| `TestCursorSetup` | `cursor_test.go` | Engine parses first `update cursor` and initializes cursor |  |
| `TestCursorDelta` | `cursor_test.go` | Engine applies attr changes to cursor |  |
| `TestCursorDel` | `cursor_test.go` | AC-8 `del` removes attr from cursor |  |
| `TestCursorInherit` | `cursor_test.go` | AC-7 NLRIs-only command inherits all cursor attrs |  |
| `TestCursorDone` | `cursor_test.go` | AC-9 cursor cleared |  |
| `TestCursorReplace` | `cursor_test.go` | AC-10 new init replaces stale cursor |  |
| `TestCursorClearProcess` | `cursor_test.go` | AC-11 `ClearProcessCursors` frees all cursors for a process |  |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NLRIs per command | 1 .. fits in max BGP UPDATE (4096 bytes standard, negotiated extended) | last NLRI that fits | N/A | split into next command |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-rib-reconnect-replay` | `test/plugin/*.ci` | Establish peer, send full table, flap, verify identical routes |  |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-reconnect-replay-frr` | `test/interop/scenarios/` | FRR | Same RIB after ze peer flap |  |

### Performance Tests
| Test | Location | Validates | Status |
|------|----------|-----------|--------|
| `BenchmarkReplayLargeTable` | `rib_replay_test.go` | call count + allocs/op drop |  |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/bgp/plugins/cmd/update/update_text.go` - add `case "cursor"` in `handleUpdate` switch, delegate to `handleUpdateCursor`.
- `internal/component/bgp/plugins/rib/rib.go` - `replayRoutes`: group, sort, emit cursor commands.
- `internal/component/bgp/plugins/rib/ribout_entry.go` - `collectAllRibOutRoutes`: group by `(family, AttrHandle, pathID)`, decode each `AttrHandle` once.
- `internal/component/bgp/textparse/keywords.go` - add `KWDone = "done"` keyword constant.
- `internal/component/plugin/server/dispatch.go` - `cleanupProcess`: add call to `ClearProcessCursors(proc.Name())` for AC-11.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] No | `update cursor` dispatches through existing `ze-bgp:peer-update` YANG path (same as `update text`) |
| CLI commands/flags | [ ] No | cursor is plugin-initiated |
| Functional test | [ ] Yes | `test/plugin/*.ci` |
| Pipe completeness | [ ] No | not user-visible output |
| Doctor check | [ ] No | - |
| Prometheus counters | [ ] Optional | replay duration / group count |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] No | - |
| 8 | Plugin SDK/protocol changed? | [ ] Yes | `docs/architecture/api/commands.md` -- document `update cursor` |
| 12 | Internal architecture changed? | [ ] Yes | replay subsystem doc |
| 16 | Changed source referenced by doc anchors? | [ ] Check | grep docs for `rib.go`/`update_text.go` |

## Files to Create
- `internal/component/bgp/plugins/cmd/update/cursor.go` - cursor handler: state struct, delta application, `done` cleanup
- `internal/component/bgp/plugins/cmd/update/cursor_test.go` - cursor handler tests
- `internal/component/bgp/plugins/rib/rib_replay.go` - replay formatting: grouping, sorting, delta computation, cursor command building
- `internal/component/bgp/plugins/rib/rib_replay_test.go` - replay tests + benchmark
- `test/plugin/test-rib-reconnect-replay.ci` - functional reconnect test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify; confirm `handleUpdate` switch, `parseCommonAttributeText` reusability |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review gate | Review Gate section |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- `TestReplayGroupsByAttrHandle` against current `replayRoutes`; fails (per-route delivery).
   - Files: `rib_replay_test.go`

2. **Phase: Cursor handler** -- implement `handleUpdateCursor` in `cursor.go`.
   - Cursor state: `*parsedAttrs` (reused directly, same package). Stored in package-level
     `sync.Map` keyed by `processName:peerSelector` (from `CommandContext.Process.Name()`
     and `CommandContext.Peer`).
   - First `update cursor <attrs> nlri ...`: parse attrs via `parseCommonAttributeText`
     (reuse), store in cursor, parse NLRIs via `parseNLRISection` (reuse), call
     `cursor.snapshot()` to build `NLRIBatch`, call `DispatchNLRIGroups`.
   - Subsequent `update cursor [<attrs>] [del <attr>]... nlri ...`: apply deltas to cursor
     (present attrs replace field, `del` nils field), snapshot, dispatch.
   - `del` for absent attr: silent no-op. `del` for mandatory attr: allowed (downstream
     rejects). Multiple `del` supported.
   - `update cursor nlri ...` (no attrs): snapshot current cursor, dispatch. Announce-only
     (`nlri <family> add` only, no `del`).
   - `update cursor done`: delete cursor entry from `sync.Map`, return success.
   - Export `ClearProcessCursors(processName string)` for crash cleanup (AC-11).
   - Register in `handleUpdate` switch as `case "cursor"`.
   - Tests: `TestCursorSetup`, `TestCursorDelta`, `TestCursorDel`, `TestCursorInherit`,
     `TestCursorDone`, `TestCursorReplace`, `TestCursorClearProcess`

3. **Phase: Grouped collection** -- refactor `collectAllRibOutRoutes`.
   - Group by `(family, AttrHandle, pathID)`. Decode each `AttrHandle` once (call `reconstructRoute` once per group, share result). Return `[]replayGroup{Route *Route, Prefixes []string, MinMsgID uint64}`.
   - Tests: `TestReplayGroupsByAttrHandle`, `TestReplayGroupingSafety`

4. **Phase: Delta formatting and sorted replay** -- implement `replayRoutes` with cursor commands.
   - Sort groups for minimal deltas: by attrs-minus-AS_PATH similarity, then AS_PATH, then family/pathID.
   - First group: format `update cursor <all attrs from Route> nlri <fam> [path-information N] add <p1> <p2> ...`.
   - Subsequent: diff current Route against previous. Emit only changed attrs + `del` for removed attrs + `nlri ... add ...`.
   - Split groups when NLRIs would exceed the max BGP UPDATE message size.
   - After all groups: `update cursor done`, then `plugin session ready`.
   - Tests: `TestReplayDeltaEncoding`, `TestReplayAttrRemoval`, `TestReplayOrderingAndDone`, `TestReplayEmptyRibOut`, `TestReplayLargeGroupSplit`

5. **Functional + interop** -- reconnect test, FRR scenario.

6. **Benchmark** -- `BenchmarkReplayLargeTable`.

7. **Full verification** -- `make ze-verify`.

8. **Complete spec** -- audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Each AC has code + test at file:line |
| Correctness | Deltas computed correctly; cursor state matches expected attrs |
| Data flow | Cursor commands go through same `UpdateRoute`/`Dispatch` path as `update text` |
| State lifecycle | Cursor per (plugin, peer). Cleared on `done`. Replaced on new init. No leak on peer flap. Freed on plugin crash via `ClearProcessCursors` |
| Rule: lock order | `pool.RibOut.mu` never under `peerMu` |
| Rule: ordering | `done` after all NLRIs; `plugin session ready` after `done` |
| Rule: no-sprintf-alloc | Delta formatter uses append-based building |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Cursor handler | `go test -run TestCursor...` |
| Grouped replay | `go test -run TestReplayGroups...` |
| Reconnect parity | run `test-rib-reconnect-replay.ci` |
| Perf delta | `go test -bench BenchmarkReplayLargeTable` before/after |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Resource use | Per-command NLRI count bounded by max BGP UPDATE size. `done` clears state |
| State leak | Peer flap during replay: next init replaces old cursor. Plugin crash: `cleanupProcess` calls `ClearProcessCursors` |
| Input validation | Attribute values validated same as `parseCommonAttributeText` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| `parseCommonAttributeText` not reusable from cursor handler | Extract into shared function with `parsedAttrs` receiver |
| Cursor state key collision (multiple plugins or concurrent replay) | Use `processName:peerSelector` composite key in `sync.Map` |
| Delta computation bug | Add round-trip test: apply deltas to cursor, verify cursor matches expected Route |
| YANG dispatch rejects `cursor` | Check if `ze-bgp:peer-update` handler receives all encoding values or just registered ones |
| `cleanupProcess` import cycle (`server` -> `cmd/update`) | Register `ClearProcessCursors` as a callback during `init()` command registration, avoiding direct import |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Checklist

### Goal Gates
- [ ] AC-1..AC-11 each implemented with code + test at file:line
- [ ] Tests written (cursor handler, replay formatting, functional, benchmark)
- [ ] Tests FAIL first (wiring test red against current per-route replay)
- [ ] Tests PASS after implementation
- [ ] Reconnect produces same RIB state (AC-3) -- interop FRR scenario green

### Quality Gates
- [ ] `make ze-test` green
- [ ] `make ze-lint-changed` clean
- [ ] `make ze-verify` green
- [ ] Lock order preserved
- [ ] `/ze-review` Review Gate: 0 BLOCKER, 0 ISSUE
- [ ] Benchmark deltas recorded in Implementation Summary

## RESEARCH Gate (BLOCKING before any scope expansion)

1. **Fresh-peer feed path:** How fresh peer receives loc-rib on bring-up. The cursor protocol could benefit this path.
2. **DirectBridge fast path:** Typed DirectBridge handler accepting cursor deltas as Go structs, skipping JSON/dispatch. Would reduce per-call cost from ~100-200us to ~5-10us. Evaluate if text protocol total (~100-200ms) is sufficient.
3. **Reuse beyond replay:** `sendRoutes` (manual resend), route server, route reflector forwarding.
4. **Zero-copy fast path:** Skip `Wire.All()` re-parse when all peers share capabilities (cached homogeneous-capability flag).

Record findings here. Do NOT widen scope without user approval.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered |
|------------------|---------------|----------------|
| `collectAllRibOutRoutes` is the primary new-peer feed | Replays already-*sent* state; fresh feed is reactor-side | Tracing `ribOut` population |
| `BuildGroupedUnicast` is the grouped text command mechanism | Reactor wire-level for static routes, unrelated | Full dispatch path trace |
| Grouped commands produce grouped wire UPDATEs | `ShouldQueue` true; reactor queues per-NLRI | `reactor_api_batch.go` |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| v1: grouped text via `BuildGroupedUnicast` | Wrong mechanism | Research: correct dispatch path |
| v2: grouped hex commands through dispatcher | Still ~100-200us/call, no ASN4 handling | v3 |
| v3: typed DirectBridge attr-def protocol | Binary API, not debuggable, external plugin excluded, lifecycle complexity | v4 |
| v4: separate `replay` command tree | Unnecessary new YANG surface when `update cursor` fits the existing dispatch | v5: `update cursor` (this spec) |

## Design Insights
- The dominant replay cost is per-call RPC overhead (~100-200us), not attribute encoding. Reducing call count from N to M is the primary win.
- A stateful cursor with delta encoding achieves the same call count as a binary protocol while staying within the text command framework.
- Sorting groups by attribute similarity before replay minimizes deltas. Most transitions need only `as-path [...]` because routes from different source peers share all other attributes.
- `update cursor` fits the existing `handleUpdate` switch (text/hex/b64/cursor). No new YANG registration, no new RPC methods. External plugins get it for free.
- `del` was removed from `update text` but is needed for cursor mode to remove attributes between groups. Reintroducing it only in cursor mode avoids confusion with the old `text` syntax.

## Core Insight

A stateful text protocol with delta encoding gives ~50-100x replay speedup while staying debuggable, loggable, and compatible with the existing dispatcher and external plugins.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `update cursor` as encoding mode in existing `handleUpdate` | Separate `replay` command, DirectBridge binary protocol | Fits existing dispatch. No new YANG, no new RPC. External plugin compatible. One-line switch case |
| Stateful cursor with implicit set / explicit `del` | Full resend per group, attr-def templates | Delta encoding minimizes payload. Consecutive groups often differ only in AS_PATH. `del` needed for attribute removal |
| `del` for absent attr: silent no-op | Error | Cursor is "current state"; removing what is absent is harmless. Matches config diff semantics |
| `del` for mandatory attr: allow, reject downstream | Reject in cursor handler | `DispatchNLRIGroups` already validates. Replay formatter never produces this from valid `*Route` |
| Announce-only (no `nlri <family> del`) | Support withdrawals | `ribOut` stores only announced routes; withdrawals delete the entry. Replay never needs withdrawals |
| Sort by wire-hash-sans-AS_PATH | Sort by MsgID, sort by decoded values | Wire hash: one iterator pass, no decode, no alloc. Stable sort preserves MsgID within equal groups |
| Cursor state = `*parsedAttrs` | New `cursorState` struct | Same package, right fields, `snapshot()` deep-copies. No new type needed |
| Cursor map: package-level `sync.Map` keyed by `processName:peerSelector` | Attach to `CommandContext`, `Process` struct, or `Server` | Handlers are stateless (existing pattern). `sync.Map` avoids coupling to infrastructure |
| `done` to clear state | Auto-cleanup on session end | Explicit lifecycle. Frees memory immediately. Handles peer flap (new init replaces stale cursor) |
| Crash cleanup via `ClearProcessCursors` in `cleanupProcess` | Accept trivial leak | `cleanupProcess` already cleans up per-process state; cursor should follow the same pattern |
| Reuse `parseCommonAttributeText` + `parseNLRISection` | New parser | Same keywords, same value formats, same aliases. No duplication |
| Split at max BGP UPDATE size | Unlimited / arbitrary count | Natural protocol limit: split only when NLRIs would not fit in a legal BGP UPDATE (4096 bytes standard, negotiated extended) |

## Known Limitations
- Does not address fresh-peer feed or cross-peer shared replay (RESEARCH gate).
- Per-call cost is still ~100-200us (JSON + dispatch). DirectBridge typed handler could reduce to ~5-10us but is out of scope.
- Grouping benefit scales with attribute sharing.
- VPN routes with per-route RD: each NLRI has unique RD; grouping still reduces call count but NLRI sections are per-route.
- `sendRoutes` (manual resend) has the same pattern and would benefit but is out of scope.

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | | file:line | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)

### AC Verified (grep/test)

### Wiring Verified (end-to-end)

### Documentation Verified
