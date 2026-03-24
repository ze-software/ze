# Spec: rib-family-ribout

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-03-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now) -- especially AD-2 (dispatch model) and AD-5 (selector fix)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/plugins/rib/rib.go` - RIBManager, ribOut, handleSent, handleRefresh, handleState
4. `internal/component/bgp/plugins/rib/rib_commands.go` - outboundResendJSON, sendRoutes, statusJSON, handler registrations
5. `internal/component/bgp/plugins/rib/rib_pipeline.go` - outboundSource
6. `internal/component/bgp/plugins/rib/storage/peerrib.go` - per-family pattern to follow
7. `internal/component/plugin/server/command.go` - dispatchPlugin, extractPeerSelector (dispatch model context for selector fix)

## Task

Restructure `RIBManager.ribOut` from a flat map (`peerAddr -> routeKey -> *Route`) to a per-family map (`peerAddr -> family -> prefixKey -> *Route`). This enables per-family operations (LLGR readvertisement for one family, route refresh, per-family purge) without scanning all routes and string-matching on family. It also enables a `rib clear out <selector> [family]` command for targeted readvertisement.

Additionally fixes a pre-existing bug: all four selector-bearing RIB command handlers (`rib clear out`, `rib clear in`, `rib retain-routes`, `rib release-routes`) ignore the `args` parameter and use the `peer` param from the execute-command RPC, which is always `"*"` for plugin-dispatched commands. The selector (e.g., `!192.168.1.1` from the GR plugin) is silently discarded. Fix: all four handlers extract selector from `args[0]`, selector is mandatory.

The storage layer (`storage.PeerRIB`) already uses per-family maps (`map[nlri.Family]*FamilyRIB`). This spec brings `ribOut` to the same structural model.

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| ribOut type change | Flat map (peer -> routeKey -> Route) to per-family map (peer -> family -> prefixKey -> Route) |
| routeKey change | `RouteKey` in ribOut context drops family prefix, becomes `prefix` or `prefix:pathID` |
| handleSent | Store routes under `ribOut[peer][family][prefixKey]` |
| handleRefresh | Direct family lookup instead of filtering |
| handleState | Flatten all families for replay |
| outboundResendJSON | Flatten all families for resend; accept optional family argument |
| statusJSON | Nested count loop |
| updateMetrics | Nested count loop |
| outboundSource | Nested iteration for pipeline |
| rib clear out syntax | Mandatory selector: `rib clear out <selector> [family]`. No-arg form is an error |
| Selector fix (all commands) | `rib clear out`, `rib clear in`, `rib retain-routes`, `rib release-routes` all have a bug: handler ignores args and uses peer param (always `*` for plugin dispatch). Fix: extract selector from args, selector mandatory for all four |
| Empty-map cleanup | Remove empty family maps on last withdrawal, remove empty peer maps when all families empty |
| persist plugin | Matching restructuring of `persist.ribOut` (note: persist uses `\|` separator, not `:`) |

**Out of Scope:**

| Area | Reason |
|------|--------|
| ribInPool restructuring | Already per-family via `storage.PeerRIB` |
| Best-path changes | Operates on ribIn, not ribOut |
| Forward path changes | Downstream of ribOut, receives routes via updateRoute RPC |
| LLGR readvertisement | Separate spec (`spec-llgr-4-readvertisement.md`), will use per-family clear out |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RIB storage model, pool dedup
  -> Constraint: ribIn already uses per-family FamilyRIB; ribOut should follow same pattern
  -> Decision: ribOut is a plugin-internal structure, no cross-plugin API change needed

**Key insights:**
- `ribOut` is purely internal to the RIB plugin -- no other plugin reads it directly
- All external interaction is via text commands (`rib clear out`, `rib show`) and events (handleSent, handleRefresh, handleState)
- The `Route` struct already carries `Family` as a field -- per-family map makes this structural
- `RouteKey()` currently embeds family in the string (`family:prefix:pathID`); with per-family outer map, key becomes `prefix:pathID` only
- **Command dispatch model:** Plugin commands arrive via `dispatchPlugin` in `command.go`. The dispatcher does longest-match on the command name, puts remaining tokens in `args`, and passes `peerSelector` separately. For plugin-dispatched commands (via `dispatch-command` RPC), `peerSelector` is always `"*"` unless the command string contains the literal `peer <addr>` keyword. Inline selectors after the command name land in `args`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib.go:118` - `ribOut map[string]map[string]*Route` (peer -> routeKey -> Route)
- [ ] `internal/component/bgp/plugins/rib/rib.go:335-401` - `handleSent`: stores/deletes routes in ribOut per family ops
- [ ] `internal/component/bgp/plugins/rib/rib.go:510-543` - `handleRefresh`: filters ribOut by `rt.Family == family` (linear scan)
- [ ] `internal/component/bgp/plugins/rib/rib.go:549-588` - `handleState`: copies all ribOut routes for replay on peer-up
- [ ] `internal/component/bgp/plugins/rib/rib.go:166-210` - `updateMetrics`: counts `len(routes)` per peer
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go:262-293` - `outboundResendJSON`: iterates all routes per peer, no family filter
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go:309-349` - `statusJSON`: sums `len(routes)` per peer
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go:114-141` - `outboundSource`: iterates all routes per peer
- [ ] `internal/component/bgp/route.go:44-49` - `RouteKey(family, prefix, pathID)` embeds family in key string
- [ ] `internal/component/bgp/plugins/persist/server.go:78` - `ribOut map[string]map[string]*StoredRoute` (matching flat-map pattern, uses pipe separator not colon, no pathID)
- [ ] `internal/component/bgp/plugins/rib/event.go:26` - `routeKey = bgp.RouteKey` (alias)
- [ ] `internal/component/plugin/server/command.go:410-448` - `dispatchPlugin`: longest-match, remaining tokens to `args`, `peerSelector` passed separately
- [ ] `internal/component/plugin/server/command.go:579-593` - `extractPeerSelector`: only recognizes explicit `peer <addr>` keyword, defaults to `"*"`
- [ ] `internal/component/bgp/plugins/gr/gr.go:101` - GR dispatches `rib clear out !<peer>` (selector lands in args, currently ignored)

**Behavior to preserve:**
- All routes stored on handleSent are replayed on peer-up (handleState)
- Route refresh (handleRefresh) sends only the requested family's routes
- When all families are requested, all families are resent (trigger syntax changes to require mandatory selector)
- `rib show` pipeline iterates all ribOut routes
- Metrics count total routes per peer
- Routes sorted by MsgID for replay order
- persist plugin stores/replays routes with matching pattern

**Behavior to change:**
- handleRefresh: linear scan with `rt.Family == family` becomes direct map lookup
- outboundResendJSON: gains optional family parameter for targeted resend
- RouteKey usage in ribOut context: family prefix becomes redundant (keyed by outer map)
- `routeKey` alias in `event.go` may become unused after `outRouteKey` replaces it in handleSent
- Command syntax: `rib clear out <selector> [family]` -- selector is mandatory (including `*`)
- **Selector bug fix:** `rib clear out`, `rib clear in`, `rib retain-routes`, `rib release-routes` handlers currently ignore args and use the peer param from execute-command RPC. For plugin-dispatched commands (via dispatch-command RPC), peer is always `*` because `extractPeerSelector` in `command.go` only recognizes the explicit `peer <addr>` keyword syntax. The actual selector (e.g., `!192.168.1.1`) ends up in args and is discarded. Fix: all four handlers extract selector from args
- Empty family maps removed on last route withdrawal; empty peer maps removed when all families empty

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Sent UPDATE events (handleSent): engine notifies RIB of routes sent to peers
- Route refresh requests (handleRefresh): peer requests re-advertisement of a family
- Peer state changes (handleState): peer up triggers replay from ribOut
- Commands (outboundResendJSON): `rib clear out <selector> [family]` triggers resend. Selector and family arrive in `args` (not `peer` param) via `dispatchPlugin` in `command.go`

### Transformation Path
1. handleSent: event.FamilyOps (per-family) -> extract family, prefix, pathID -> store at `ribOut[peer][family][prefixKey]`
2. handleRefresh: `ribOut[peer][family]` -> direct lookup -> sendRoutes
3. handleState peer-up: `ribOut[peer]` -> flatten all families -> replayRoutes
4. outboundResendJSON: `ribOut[peer]` -> flatten all families (or single family) -> sendRoutes
5. statusJSON/updateMetrics: `ribOut[peer]` -> sum across families
6. outboundSource pipeline: `ribOut[peer]` -> flatten all families -> RouteItem list

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ribOut internal structure | Internal to RIBManager, no cross-plugin exposure | [ ] |
| RIB command interface | Text commands; adding optional family arg to `rib clear out` | [ ] |
| persist plugin | Matching restructuring, internal to persist | [ ] |

### Integration Points
- `bgp.RouteKey()` in `route.go` - shared function; ribOut callers will use a local prefixKey instead
- GR plugin dispatches `rib clear out !<peer>` - selector now correctly extracted from args (was silently ignored before)
- GR plugin dispatches `rib retain-routes <peer>`, `rib release-routes <peer>` - both now use args for selector (was silently ignored). Note: `rib mark-stale` already uses args correctly, not affected
- LLGR spec-llgr-4 will use `rib clear out !<peer> <family>` once available
- `command.go:extractPeerSelector` is NOT changed - the fix is in the RIB plugin handlers, not the dispatcher

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Architecture Decisions

### AD-1: ribOut key structure

New type is a three-level map: peer (string) -> family (string) -> prefixKey (string) -> Route.

| Level | Key type | Example | Purpose |
|-------|----------|---------|---------|
| 1 | peer address | `"10.0.0.1"` | Per-peer grouping (unchanged) |
| 2 | family | `"ipv4/unicast"` | Per-family grouping (new) |
| 3 | prefixKey | `"10.0.0.0/24"` or `"10.0.0.0/24:1"` | Route identity (family prefix dropped) |

Family key is string, not `nlri.Family`, because ribOut operates on event data that arrives as strings. Converting to `nlri.Family` and back would add overhead with no benefit -- ribOut never needs family parsing, just grouping.

PrefixKey becomes `prefix` or `prefix:pathID` (dropping the family prefix from current `RouteKey`). A local helper `outRouteKey` takes a prefix string and pathID, returns a string key. Replaces `RouteKey` usage in ribOut code.

### AD-2: `rib clear out` with mandatory selector and optional family

**Current bug:** All four selector-bearing handlers (`rib clear out`, `rib clear in`, `rib retain-routes`, `rib release-routes`) use the peer parameter from the execute-command RPC as the selector. For plugin-dispatched commands (GR plugin dispatching `rib clear out !peer`), the engine's `extractPeerSelector` only recognizes the explicit `peer <addr>` keyword syntax. Without that keyword, peer defaults to `*` and the actual selector lands in args, which all four handlers ignore.

**Dispatch path:** The command `rib clear out !192.168.1.1 ipv4/unicast` arrives at the RIB plugin with command = `rib clear out`, peer = `*`, and args containing the selector and family as separate tokens.

**Fix:** All four handlers switch from using the peer parameter to extracting the selector from the first arg. For `rib clear out`, the second arg is the optional family.

**New syntax:** `rib clear out <selector> [family]` where selector is mandatory (use `*` for all peers). The no-arg form returns an error. Disambiguation: families contain `/`, selectors do not.

| First arg | Second arg | Interpretation |
|-----------|------------|---------------|
| `*` | (none) | All peers, all families |
| `*` | `ipv4/unicast` | All peers, one family |
| `!192.168.1.1` | (none) | All except peer, all families |
| `!192.168.1.1` | `ipv4/unicast` | All except peer, one family |
| (none) | -- | Error: missing required selector |

Existing callers: GR plugin sends `rib clear out !<peerAddr>` -- selector is already the first token after the command name, so it arrives as the first arg. Works without change. LLGR will use `rib clear out !<peerAddr> <family>` once available.

### AD-3: bgp.RouteKey unchanged

`bgp.RouteKey()` is a shared function in `route.go` used by RIB code and potentially other callers. It stays as-is. RIBManager introduces a local `outRouteKey` for the ribOut-specific prefix-only key. No shared function changes. Note: persist plugin does NOT use `bgp.RouteKey` -- it has its own local key format.

### AD-4: persist plugin follows matching pattern

`persist.ribOut` has the same flat-map structure and usage pattern. Apply the same per-family restructuring. The persist plugin's `StoredRoute` stores `Family` in the struct (like `Route`), so the per-family outer map is equally natural.

**Key format difference:** persist uses `family|prefix` as its route key (`server.go:159`), while the RIB plugin uses `bgp.RouteKey` with `:` separator and pathID. After restructuring, persist's key becomes `prefix` only (dropping the `family|` prefix), matching the same simplification as the RIB plugin's `outRouteKey`.

**PathID gap (pre-existing):** `StoredRoute` has no `PathID` field (`server.go:54-58`). With ADD-PATH, two routes for the same family+prefix but different pathIDs collide in persist's ribOut. This is a pre-existing gap, not introduced by this spec. The restructuring does not make it worse.

### AD-5: Selector fix applies to all four handlers

The selector bug affects four handlers that share the same pattern (selector from peer param, args ignored):

| Command | Handler function | Line |
|---------|-----------------|------|
| `rib clear out` | `outboundResendJSON` | 107 |
| `rib clear in` | `inboundEmptyJSON` | 103 |
| `rib retain-routes` | `retainRoutesJSON` | 111 |
| `rib release-routes` | `releaseRoutesJSON` | 115 |

All four switch to extracting selector from the first arg. Selector is mandatory for all four (use `*` for all peers). The peer parameter from the execute-command RPC is no longer used as the selector source.

**Not affected:** `rib status` also ignores args, but `statusJSON` reports global state (never filters by peer), so there is no bug. `rib mark-stale`, `rib purge-stale`, `rib show`, `rib best` already use args correctly.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Sent UPDATE event with multiple families | -> | ribOut stores per-family | `TestHandleSentPerFamily` |
| Route refresh for one family | -> | Direct family lookup, only that family sent | `TestHandleRefreshPerFamily` |
| `rib clear out * <family>` command | -> | Per-family resend | `test/plugin/rib-clear-out-family.ci` |
| `rib clear out !<peer>` from plugin dispatch | -> | Selector from args filters peers | `TestOutboundResendSelectorFromArgs` |
| Peer reconnect | -> | All families replayed | `TestHandleStateReplayAllFamilies` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | handleSent with routes in ipv4/unicast and ipv6/unicast | Routes stored in separate family maps within ribOut |
| AC-2 | handleRefresh for ipv4/unicast when ribOut has both families | Only ipv4/unicast routes sent |
| AC-3 | handleState peer-up with routes in multiple families | All families' routes replayed in MsgID order |
| AC-4 | `rib clear out <selector>` (no family arg) | All families resent for matching peers |
| AC-5 | `rib clear out <selector> ipv4/unicast` | Only ipv4/unicast routes resent for matching peers |
| AC-6 | `rib status` with routes in multiple families | Total route count matches sum across all families |
| AC-7 | `rib show sent` with routes in multiple families | All routes from all families appear in pipeline output |
| AC-8 | Withdrawal via handleSent (action "del") | Route removed from correct family map |
| AC-9 | persist plugin handleSent with multiple families | StoredRoutes stored per-family (matching restructuring) |
| AC-10 | GR plugin dispatches `rib clear out !192.168.1.1` via DispatchCommand | Only peers other than 192.168.1.1 receive resent routes (selector from args[0], not peer param) |
| AC-11 | Withdrawal of last route in a family for a peer | Empty family map entry removed from ribOut; empty peer map removed when all families empty |
| AC-12 | `rib clear in 192.168.1.1` dispatched from plugin | Only that peer's inbound routes cleared (selector from args[0]) |
| AC-13 | `rib retain-routes 192.168.1.1` dispatched from plugin | Only that peer's routes retained (selector from args[0]) |
| AC-14 | `rib release-routes 192.168.1.1` dispatched from plugin | Only that peer's routes released (selector from args[0]) |
| AC-15 | `rib clear out` with no args | Error returned (missing required selector) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHandleSentPerFamily` | `rib_test.go` | AC-1: routes stored under correct family key |  |
| `TestHandleSentWithdrawalPerFamily` | `rib_test.go` | AC-8: withdrawal removes from correct family |  |
| `TestHandleRefreshPerFamily` | `rib_test.go` | AC-2: only requested family sent |  |
| `TestHandleStateReplayAllFamilies` | `rib_test.go` | AC-3: all families replayed on peer-up |  |
| `TestOutboundResendAllFamilies` | `rib_test.go` | AC-4: no-family resend replays everything |  |
| `TestOutboundResendSingleFamily` | `rib_test.go` | AC-5: family-specific resend |  |
| `TestStatusJSONMultiFamilyCount` | `rib_test.go` | AC-6: correct total across families |  |
| `TestOutboundSourceMultiFamily` | `rib_test.go` | AC-7: pipeline iterates all families |  |
| `TestPersistSentPerFamily` | `persist/server_test.go` | AC-9: persist follows matching pattern |  |
| `TestOutRouteKey` | `rib_test.go` | prefixKey with/without pathID |  |
| `TestOutboundResendSelectorFromArgs` | `rib_test.go` | AC-10: selector extracted from args[0], not peer param |  |
| `TestHandleSentWithdrawalCleansEmptyMaps` | `rib_test.go` | AC-11: empty family/peer maps removed |  |
| `TestInboundEmptySelectorFromArgs` | `rib_test.go` | AC-12: rib clear in uses args[0] |  |
| `TestRetainRoutesSelectorFromArgs` | `rib_test.go` | AC-13: rib retain-routes uses args[0] |  |
| `TestReleaseRoutesSelectorFromArgs` | `rib_test.go` | AC-14: rib release-routes uses args[0] |  |
| `TestOutboundResendNoArgError` | `rib_test.go` | AC-15: missing selector returns error |  |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A -- no new numeric inputs | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rib-clear-out-family` | `test/plugin/rib-clear-out-family.ci` | User runs `rib clear out * ipv4/unicast`, only that family resent | |

### Future (if deferring any tests)
- Per-family metrics (routes-out per family per peer) -- not needed now, metrics sum across families

## Files to Modify

- `internal/component/bgp/plugins/rib/rib.go` - ribOut type, handleSent, handleRefresh, handleState, updateMetrics, empty-map cleanup
- `internal/component/bgp/plugins/rib/rib_commands.go` - all four selector-bearing handlers switch from `peer` param to `args[0]`; outboundResendJSON adds family from `args[1]`; statusJSON nested loop
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - outboundSource nested iteration
- `internal/component/bgp/plugins/rib/rib_test.go` - update existing tests, add new tests (selector, empty-map, per-family)
- `internal/component/bgp/plugins/rib/rib_commands_test.go` - update test initialisation, update command tests to pass selector in args
- `internal/component/bgp/plugins/rib/rib_metrics_test.go` - update test initialisation
- `internal/component/bgp/plugins/rib/event.go` - remove `routeKey` alias if unused after outRouteKey replaces it in handleSent
- `internal/component/bgp/plugins/persist/server.go` - matching ribOut restructuring (note: uses `|` separator, not `:`)
- `internal/component/bgp/plugins/persist/server_test.go` - update test initialisation
- `test/plugin/api-rib-clear-out.ci` - update to use mandatory selector (`rib clear out *`)
- `test/exabgp-compat/etc/run/api-v6-comprehensive.run` - update `rib clear out` call to include mandatory selector

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A (command name unchanged, args are positional) |
| CLI commands/flags | Yes | `docs/guide/command-reference.md` -- mandatory selector, optional family |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/plugin/rib-clear-out-family.ci` |
| Existing functional test update | Yes | `test/plugin/api-rib-clear-out.ci` -- add mandatory selector `*` |
| ExaBGP compat test update | Yes | `test/exabgp-compat/etc/run/api-v6-comprehensive.run` -- add mandatory selector `*` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- `rib clear out` now requires selector, add `[family]` arg |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` -- update `rib clear out <selector> [family]` syntax |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/rib-storage-design.md` -- document per-family ribOut |

## Files to Create

- `test/plugin/rib-clear-out-family.ci` - functional test for per-family clear out with mandatory selector

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: selector fix for all four handlers** -- fix the pre-existing bug before restructuring
   - Tests: `TestOutboundResendSelectorFromArgs`, `TestInboundEmptySelectorFromArgs`, `TestRetainRoutesSelectorFromArgs`, `TestReleaseRoutesSelectorFromArgs`, `TestOutboundResendNoArgError`
   - Files: `rib_commands.go` (all four handler registrations: switch to using args for selector). Update existing command tests to pass selector in args.
   - Verify: tests fail -> implement -> tests pass. `make ze-unit-test` passes.

2. **Phase: outRouteKey helper** -- local prefix-only key function
   - Tests: `TestOutRouteKey`
   - Files: `rib.go` or a new small helper
   - Verify: tests fail -> implement -> tests pass

3. **Phase: ribOut type change + handleSent + existing test updates** -- change type, update store/delete. All existing tests that construct ribOut directly MUST be updated in this phase (they break as soon as the type changes).
   - Tests: `TestHandleSentPerFamily`, `TestHandleSentWithdrawalPerFamily`, `TestHandleSentWithdrawalCleansEmptyMaps`
   - Files: `rib.go` (type, init, handleSent, empty-map cleanup), `event.go` (remove unused `routeKey` alias if applicable), all test files that construct ribOut directly (`rib_test.go`, `rib_commands_test.go`, `rib_metrics_test.go`). Note: `blocking_test.go` populates ribOut via handleSent events, not direct construction -- it should compile without changes but verify.
   - Verify: tests fail -> implement -> tests pass. `make ze-unit-test` passes.

4. **Phase: handleRefresh** -- direct family lookup
   - Tests: `TestHandleRefreshPerFamily`
   - Files: `rib.go` (handleRefresh)
   - Verify: tests fail -> implement -> tests pass

5. **Phase: handleState + replay** -- flatten families for replay
   - Tests: `TestHandleStateReplayAllFamilies`
   - Files: `rib.go` (handleState)
   - Verify: tests fail -> implement -> tests pass

6. **Phase: outboundResendJSON + per-family command** -- add optional family from args[1]
   - Tests: `TestOutboundResendAllFamilies`, `TestOutboundResendSingleFamily`
   - Files: `rib_commands.go` (outboundResendJSON signature, handler passes args)
   - Verify: tests fail -> implement -> tests pass

7. **Phase: statusJSON + updateMetrics** -- nested counting
   - Tests: `TestStatusJSONMultiFamilyCount`
   - Files: `rib_commands.go` (statusJSON), `rib.go` (updateMetrics)
   - Verify: tests fail -> implement -> tests pass

8. **Phase: outboundSource pipeline** -- nested iteration
   - Tests: `TestOutboundSourceMultiFamily`
   - Files: `rib_pipeline.go` (outboundSource)
   - Verify: tests fail -> implement -> tests pass

9. **Phase: persist plugin** -- matching restructuring
   - Tests: `TestPersistSentPerFamily`
   - Files: `persist/server.go`, `persist/server_test.go`
   - Verify: tests fail -> implement -> tests pass

10. **Functional tests** -- create `test/plugin/rib-clear-out-family.ci`, update `test/plugin/api-rib-clear-out.ci` (mandatory selector)

11. **Full verification** -- `make ze-verify`

12. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N (AC-1 through AC-15) has implementation with file:line |
| Correctness | Route counts match before/after restructuring (no lost routes) |
| Selector fix | All four handlers extract selector from args[0], not peer param. Test with plugin dispatch path |
| Empty-map cleanup | Withdrawal of last route removes family map; withdrawal of last family removes peer map |
| Naming | `outRouteKey` consistent, family keys are plain strings |
| Data flow | ribOut still internal to RIBManager, no leaked abstraction |
| Rule: no-layering | Old flat ribOut fully replaced, no compatibility shims |
| Rule: buffer-first | N/A (no wire encoding changes) |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| ribOut type changed | grep three-level map type in rib.go |
| handleRefresh uses direct lookup | grep `ribOut[peerAddr][family]` pattern in rib.go |
| Per-family clear out command | grep family handling in outboundResendJSON in rib_commands.go |
| Selector from args | All four handlers accept and use args parameter in rib_commands.go (no ignored args) |
| Empty-map cleanup | grep delete of empty family/peer maps in handleSent |
| persist plugin restructured | grep three-level map type in persist/server.go |
| Existing functional test updated | grep mandatory selector in `test/plugin/api-rib-clear-out.ci` |
| New functional test exists | ls `test/plugin/rib-clear-out-family.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Family string in `rib clear out <selector> <family>` -- validate format (contains "/") |
| Input validation | Selector in args[0] -- validate not empty, pass to existing matchesPeer (already handles all formats) |
| Resource exhaustion | Per-family map creation -- bounded by peer count * family count (both finite) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- Existing test `rib_test.go:460-461` asserts "ribOut should persist through state changes" with comment "we never delete it." After AC-11, empty peer maps ARE deleted on the withdrawal path (not state changes). The assertion itself remains valid for state changes, but the comment must be updated to clarify the distinction: state changes preserve ribOut, withdrawals clean up empty maps.

## RFC Documentation

N/A -- no protocol changes. This is an internal data structure refactoring.

## Implementation Summary

### What Was Implemented
- (to be filled after implementation)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-verify` passes (lint + unit + functional + exabgp + chaos)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-rib-family-ribout.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
