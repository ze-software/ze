# Spec: plugin-rr — Route Server & Persist Plugins

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/capability-contract.md` - engine API surface
4. `docs/architecture/rib-transition.md` - RIB ownership model
5. `internal/plugins/bgp-rr/server.go` - existing RR implementation
6. `internal/plugins/bgp-rr/rib.go` - RIB storage
7. `.claude/rules/plugin-design.md` - plugin patterns

## Task

Two plugins sharing common RIB infrastructure:

| Plugin | Binary Name | Use Case | RIB | Events |
|--------|-------------|----------|-----|--------|
| Route Server | `ze plugin bgp-rr` | IX route server (multi-peer) | ribIn | `update`, `state`, `open`, `refresh` |
| Persist | `ze plugin bgp-persist` | State persistence (single-peer) | ribOut | `update direction sent`, `state` |

**RR plugin** forwards all UPDATEs to all peers except source (forward-all model, no best-path).
Stores routes in Adj-RIB-In for replay when a new peer comes up.

**Persist plugin** tracks routes sent to peers and replays them on reconnect.
Needed for Test C (teardown scenario) where API process announcements must survive peer restart.

### Implementation Status

| Component | Status | Location |
|-----------|--------|----------|
| RR plugin core | ✅ Done | `internal/plugins/bgp-rr/` |
| RR RIB | ✅ Done | `internal/plugins/bgp-rr/rib.go` |
| RR peer state | ✅ Done | `internal/plugins/bgp-rr/peer.go` |
| RR unit tests | ✅ Done | `internal/plugins/bgp-rr/server_test.go`, `rib_test.go` |
| RR registration | ✅ Done | `internal/plugins/bgp-rr/register.go` |
| RR functional test | ⚠️ Partial | `test/plugin/plugin-rr-features.ci` (features only) |
| Engine cache commands | ✅ Done | `internal/plugins/bgp/handler/cache.go` |
| Peer selector `!<ip>` | ✅ Done | `internal/selector/selector.go` |
| Sent event emission | ✅ Done | `internal/plugins/bgp/server/events.go` |
| Config capability validation | ✅ Done | `internal/config/bgp.go` |
| Persist plugin | ❌ Not started | — |
| EOR on peer up | ❌ Missing | RR and persist both need this |
| Cache retain/release | ❌ Not used | RR forwards but doesn't retain cache entries |

### Remaining Work

1. **RR: Send EOR after replay** — on peer up, send End-of-RIB per family after replaying routes
2. **RR: Cache retain/release** — retain msg-ids for routes in RIB, release on withdrawal/peer down
3. **RR: End-to-end functional tests** — verify forwarding, replay, withdrawal across peers
4. **Persist plugin** — new plugin tracking outbound routes for reconnect replay
5. **Persist: functional tests** — verify teardown/reconnect scenario

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - engine architecture
  → Decision: Engine has no RIB; API plugins own all route storage
  → Constraint: Plugins communicate via SDK RPC, not direct imports
- [ ] `docs/architecture/api/capability-contract.md` - engine API surface
  → Decision: Cache commands use `bgp cache <id> <action>` syntax
  → Constraint: API process must have `send { update; }` for route-refresh/GR
- [ ] `docs/architecture/rib-transition.md` - RIB ownership model
  → Decision: Wire bytes stored in engine cache, API controls lifetime
  → Constraint: Zero-copy forward via `bgp cache <id> forward <sel>`
- [ ] `docs/architecture/api/commands.md` - command syntax reference
  → Constraint: EOR via `peer <sel> update text nlri <family> eor`
- [ ] `.claude/rules/plugin-design.md` - plugin registration patterns
  → Constraint: 5-stage startup, SDK callbacks, register.go pattern

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - BGP-4 base protocol
  → Constraint: End-of-RIB marker (empty UPDATE) required after initial table exchange
- [ ] `rfc/short/rfc7947.md` - IX BGP Route Server (if exists)
  → Constraint: RS forwards all routes, no best-path selection

**Key insights:**
- Engine provides cache + forward + selector infrastructure — plugin just orchestrates
- Forward-all model means no best-path CPU cost; peers do their own selection
- Cache retain/release controls memory: retained entries survive TTL indefinitely
- Sent events already work via `subscribe bgp event update direction sent`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/bgp-rr/server.go` - RouteServer with event dispatch, forwarding, state handling
- [ ] `internal/plugins/bgp-rr/rib.go` - RIB: Insert, Remove, ClearPeer, GetAllPeers
- [ ] `internal/plugins/bgp-rr/peer.go` - PeerState with HasCapability, SupportsFamily
- [ ] `internal/plugins/bgp-rr/register.go` - Plugin registration (bgp-rr, RFC 4456)
- [ ] `internal/plugins/bgp-rr/server_test.go` - 13 unit tests covering all handlers
- [ ] `internal/plugins/bgp-rr/rib_test.go` - RIB CRUD tests

**Behavior to preserve:**
- RR forwards UPDATEs via `cache <id> forward` to each eligible peer individually
- Peer down clears ribIn and sends withdrawals via `update text nlri <family> del <prefix>`
- Peer up replays routes from other peers via `cache <id> forward`
- OPEN events capture capabilities and families for filtering
- Refresh events forwarded to peers with route-refresh capability
- Commands `rr status` and `rr peers` return JSON

**Behavior to change:**
- Add EOR per family after replay on peer up
- Add cache retain on route insert, release on route removal/peer down
- Persist plugin: new plugin (no existing behavior to preserve)

## Data Flow (MANDATORY)

### Entry Point
- UPDATE wire bytes arrive from TCP peer → engine assigns msg-id → caches in RecentUpdateCache
- Engine emits JSON event to subscribed plugins via deliver-event RPC (Socket B)

### Transformation Path (RR)
1. Engine receives UPDATE, assigns msg-id, caches wire bytes
2. Engine sends JSON event with msg-id to bgp-rr plugin
3. Plugin parses event, extracts family/prefix from announce/withdraw maps
4. Plugin inserts into RIB (peer → routeKey → Route with MsgID)
5. Plugin sends `bgp cache <id> retain` to engine (keep for replay)
6. Plugin sends `bgp cache <id> forward` per eligible peer (zero-copy)

### Transformation Path (Persist)
1. Engine sends UPDATE to peer (TCP write)
2. Engine emits sent event to bgp-persist plugin (direction=sent)
3. Plugin parses event, extracts family/prefix
4. Plugin inserts into ribOut (peer → routeKey → Route with MsgID)
5. On peer reconnect: plugin replays via `bgp cache <id> forward`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → Plugin | JSON event via deliver-event RPC (Socket B) | [ ] |
| Plugin → Engine | SDK UpdateRoute → ze-plugin-engine:update-route RPC (Socket A) | [ ] |
| Cache → Wire | `bgp cache <id> forward` → reactor.ForwardUpdate() | [ ] |

### Integration Points
- `sdk.Plugin.UpdateRoute(ctx, peer, command)` — sends commands to engine
- `sdk.Plugin.SetStartupSubscriptions(events, peers, format)` — registers event subscriptions
- `sdk.Registration{Commands: [...]}` — registers API commands
- `reactor.ForwardUpdate(selector, id)` — zero-copy forward from cache
- `reactor.RetainUpdate(id)` / `reactor.ReleaseUpdate(id)` — cache lifetime control

### Architectural Verification
- [ ] No bypassed layers (plugin uses SDK, not direct reactor access)
- [ ] No unintended coupling (bgp-rr and bgp-persist are separate packages)
- [ ] No duplicated functionality (persist reuses RIB/PeerState types — consider shared package)
- [ ] Zero-copy preserved (cache forward, not announce from pool)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer A sends UPDATE | RR forwards to all other established peers via cache forward |
| AC-2 | Peer A sends UPDATE with ipv6/unicast | RR skips peers without ipv6/unicast family |
| AC-3 | Peer A sends withdrawal | RR removes from ribIn, forwards withdrawal to others |
| AC-4 | Peer A goes down | RR clears ribIn for A, sends withdrawals to all others, releases retained cache entries |
| AC-5 | Peer B comes up | RR replays all routes from other peers to B, sends EOR per family |
| AC-6 | Peer A requests route-refresh | RR forwards refresh to peers with route-refresh capability |
| AC-7 | RR inserts route | Cache entry retained (survives TTL) |
| AC-8 | RR removes route | Cache entry released (can be evicted) |
| AC-9 | Engine sends UPDATE to peer X | Persist plugin stores in ribOut with msg-id |
| AC-10 | Peer X reconnects (persist mode) | Persist replays ribOut routes via cache forward, sends EOR |
| AC-11 | Peer X goes down (persist mode) | Persist marks peer down, keeps ribOut intact |
| AC-12 | `rr status` command | Returns `{"running":true}` |
| AC-13 | `rr peers` command | Returns JSON array of peer objects |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestServer_HandleUpdate` | `bgp-rr/server_test.go` | UPDATE stores in RIB | ✅ Done |
| `TestServer_HandleWithdraw` | `bgp-rr/server_test.go` | Withdrawal removes from RIB | ✅ Done |
| `TestServer_HandleStateDown` | `bgp-rr/server_test.go` | Peer down clears RIB | ✅ Done |
| `TestServer_HandleStateUp` | `bgp-rr/server_test.go` | Peer up sets state | ✅ Done |
| `TestServer_HandleStateUp_ExcludesSelf` | `bgp-rr/server_test.go` | Replay excludes self | ✅ Done |
| `TestServer_HandleRefresh` | `bgp-rr/server_test.go` | Refresh forwarded | ✅ Done |
| `TestServer_HandleOpen` | `bgp-rr/server_test.go` | Capabilities parsed | ✅ Done |
| `TestServer_FilterUpdateByFamily` | `bgp-rr/server_test.go` | Family filtering | ✅ Done |
| `TestServer_FilterRefreshByCapability` | `bgp-rr/server_test.go` | Capability filtering | ✅ Done |
| `TestServer_FilterReplayByFamily` | `bgp-rr/server_test.go` | Replay family filter | ✅ Done |
| `TestServer_HandleCommand_Status` | `bgp-rr/server_test.go` | rr status | ✅ Done |
| `TestServer_HandleCommand_Peers` | `bgp-rr/server_test.go` | rr peers | ✅ Done |
| `TestServer_HandleCommand_Unknown` | `bgp-rr/server_test.go` | Unknown command error | ✅ Done |
| `TestServer_IgnoreEmptyPeer` | `bgp-rr/server_test.go` | Empty peer rejected | ✅ Done |
| `TestServer_HandleUpdate_RetainsCache` | `bgp-rr/server_test.go` | UPDATE triggers retain | |
| `TestServer_HandleWithdraw_ReleasesCache` | `bgp-rr/server_test.go` | Withdrawal triggers release | |
| `TestServer_HandleStateDown_ReleasesCache` | `bgp-rr/server_test.go` | Peer down releases all | |
| `TestServer_HandleStateUp_SendsEOR` | `bgp-rr/server_test.go` | EOR sent per family after replay | |
| `TestPersist_HandleSentEvent` | `bgp-persist/server_test.go` | Sent event stores in ribOut | |
| `TestPersist_HandleStateDown` | `bgp-persist/server_test.go` | Peer down keeps ribOut | |
| `TestPersist_HandleStateUp` | `bgp-persist/server_test.go` | Peer up replays ribOut | |
| `TestPersist_HandleStateUp_SendsEOR` | `bgp-persist/server_test.go` | EOR sent per family | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MsgID | 0-uint64 max | N/A | N/A | N/A (uint64 handles all) |
| ASN | 0-uint32 max | 4294967295 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-rr-features` | `test/plugin/plugin-rr-features.ci` | `--features` output | ✅ Done |
| `plugin-rr-forward` | `test/plugin/plugin-rr-forward.ci` | A announces, B and C receive | |
| `plugin-rr-peer-down` | `test/plugin/plugin-rr-peer-down.ci` | A down, B and C get withdrawals | |
| `plugin-rr-peer-up-replay` | `test/plugin/plugin-rr-peer-up-replay.ci` | C reconnects, gets A and B routes + EOR | |
| `plugin-persist-features` | `test/plugin/plugin-persist-features.ci` | `--features` output | |
| `plugin-persist-reconnect` | `test/plugin/plugin-persist-reconnect.ci` | API sends, peer reconnects, gets replay | |

## Files to Modify
- `internal/plugins/bgp-rr/server.go` - Add EOR on peer up, cache retain/release
- `internal/plugins/bgp-rr/server_test.go` - Tests for EOR, retain, release

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No | N/A (uses existing SDK RPCs) |
| RPC count in architecture docs | [ ] No | No new RPCs |
| CLI commands/flags | [ ] No | Uses existing `ze plugin` dispatch |
| CLI usage/help text | [ ] No | Auto-generated from registration |
| API commands doc | [ ] No | `rr status`, `rr peers` already documented |
| Plugin SDK docs | [ ] No | No new SDK methods |
| Editor autocomplete | [ ] No | YANG-driven |
| Functional test for new RPC/API | [x] Yes | `test/plugin/plugin-rr-*.ci`, `test/plugin/plugin-persist-*.ci` |

## Files to Create
- `internal/plugins/bgp-persist/server.go` - PersistServer implementation
- `internal/plugins/bgp-persist/server_test.go` - Unit tests
- `internal/plugins/bgp-persist/register.go` - Plugin registration
- `test/plugin/plugin-rr-forward.ci` - Functional: forwarding
- `test/plugin/plugin-rr-peer-down.ci` - Functional: peer down
- `test/plugin/plugin-rr-peer-up-replay.ci` - Functional: replay + EOR
- `test/plugin/plugin-persist-features.ci` - Functional: features
- `test/plugin/plugin-persist-reconnect.ci` - Functional: reconnect replay

## Implementation Steps

### Phase 1: RR — EOR + Cache Lifecycle (existing plugin enhancements)

1. **Write tests** for EOR on peer up and cache retain/release
   → **Review:** Do tests cover all families in peer's capability set?

2. **Run tests** — verify FAIL (paste output)
   → **Review:** Tests fail for the right reason?

3. **Implement EOR** — after replay loop, send `update text nlri <family> eor` per family
   → **Review:** Only sends EOR for families the peer negotiated?

4. **Implement cache retain** — on route insert, call `bgp cache <id> retain`
   → **Review:** Retain uses SDK UpdateRoute, not direct reactor call?

5. **Implement cache release** — on route removal/peer down, call `bgp cache <id> release`
   → **Review:** Release old msg-id when route is replaced by new UPDATE?

6. **Run tests** — verify PASS (paste output)

### Phase 2: Persist Plugin (new plugin)

7. **Create registration** — `bgp-persist/register.go` following plugin pattern
   → **Review:** Uses `SetStartupSubscriptions(["update direction sent", "state"])` ?

8. **Write tests** for sent event handling, peer down (keep ribOut), peer up (replay + EOR)
   → **Review:** Tests use non-default msg-ids and multiple families?

9. **Run tests** — verify FAIL

10. **Implement PersistServer** — event dispatch, ribOut storage, replay on peer up
    → **Review:** Reuses RIB/PeerState types or copies from bgp-rr?

11. **Run tests** — verify PASS

12. **Run `make generate`** — regenerate `internal/plugin/all/all.go`

13. **Update `TestAllPluginsRegistered`** — increment expected plugin count

### Phase 3: Functional Tests

14. **Create functional tests** for RR forwarding, peer down, replay
    → **Review:** Tests exercise real engine ↔ plugin communication?

15. **Create functional tests** for persist reconnect scenario

16. **Run `make ze-verify`** — lint + test + functional (paste output)

## Design: Forward-All with Last-Wins

**Model:** RS forwards all UPDATEs immediately. No best-path selection.

**Rationale:**
- Zero-copy forwarding (performance)
- Simple implementation (no best-path calculation)
- Peers maintain their own Adj-RIB-In
- Acceptable for IX route server use case

**Trade-off:** If peer A announces, then peer B announces a worse route, peers keep B's route (last received). Acceptable because routes converge quickly and peers can prefer other sources.

## Engine API Reference

Commands used by both plugins (all ✅ implemented):

| Command | Description | Used By |
|---------|-------------|---------|
| `bgp cache <id> forward <sel>` | Zero-copy forward cached UPDATE | RR (forward to peers), Persist (replay) |
| `bgp cache <id> retain` | Prevent cache eviction | RR (keep for replay) |
| `bgp cache <id> release` | Allow cache eviction | RR (route withdrawn/peer down) |
| `bgp cache list` | List cached msg-ids | Debugging |
| `peer <sel> update text nlri <family> del <prefix>` | Send withdrawal | RR (peer down) |
| `peer <sel> update text nlri <family> eor` | Send End-of-RIB marker | RR + Persist (after replay) |
| `peer <sel> refresh <family>` | Request route refresh | RR (refresh forwarding) |

Event subscriptions:

| Subscription | Direction | Used By |
|-------------|-----------|---------|
| `update` | received (default) | RR |
| `update direction sent` | sent | Persist |
| `state` | both | RR + Persist |
| `open` | received | RR (capability capture) |
| `refresh` | received | RR |

## State

### Route

| Field | Type | Description |
|-------|------|-------------|
| MsgID | uint64 | Message ID for cache forward reference |
| Family | string | Address family (e.g., "ipv4/unicast") |
| Prefix | string | NLRI prefix (e.g., "10.0.0.0/24") |

### PeerState

| Field | Type | Description |
|-------|------|-------------|
| Address | string | Peer IP address |
| ASN | uint32 | Peer AS number |
| Up | bool | Session established |
| Capabilities | map | Negotiated capabilities (e.g., route-refresh) |
| Families | map | Supported AFI/SAFI families |

### RIB

| Field | Type | Description |
|-------|------|-------------|
| routes | map[peer] → map[routeKey] → Route | Per-peer route storage |

Route key format: `family + "|" + prefix`

### RouteServer (RR)

| Field | Type | Description |
|-------|------|-------------|
| plugin | SDK Plugin | Engine communication |
| peers | map[addr] → PeerState | Peer tracking |
| rib | RIB | Adj-RIB-In (routes FROM peers) |

### PersistServer

| Field | Type | Description |
|-------|------|-------------|
| plugin | SDK Plugin | Engine communication |
| peers | map[addr] → PeerState | Peer tracking |
| ribOut | RIB | Routes SENT TO peers (for replay) |

**Key difference:**
- `rib` (RR): Routes received FROM peers → forward to others, clear on peer down
- `ribOut` (Persist): Routes sent TO peers → replay on reconnect, keep on peer down

## Event Handling

### UPDATE Received (RR)

**Trigger:** `subscribe bgp event update` (received direction, default)

**Action:**
1. Extract peer address and msg-id from event
2. For each announced family/prefix: insert into RIB, retain cache entry
3. For each withdrawn family/prefix: remove from RIB, release old cache entry
4. Forward UPDATE to all eligible peers via `bgp cache <id> forward`

### Peer Down (RR)

**Trigger:** `subscribe bgp event state`

**Action:**
1. Get all routes from downed peer via `ClearPeer()`
2. For each cleared route: send withdrawal to other peers, release cache entry
3. Mark peer as down

### Peer Up (RR)

**Trigger:** `subscribe bgp event state`

**Action:**
1. Get all routes from all other peers
2. For each route compatible with new peer's families: forward via `bgp cache <id> forward`
3. Send EOR per family: `peer <addr> update text nlri <family> eor`
4. Mark peer as up

### Sent Event (Persist)

**Trigger:** `subscribe bgp event update direction sent`

**Action:**
1. Extract destination peer, family, prefix from sent event
2. Insert into ribOut with msg-id
3. Retain cache entry

### Peer Down (Persist)

**Action:** Mark peer down. **Do NOT clear ribOut** — routes were sent before, replay on reconnect.

### Peer Up (Persist)

**Action:**
1. Replay all ribOut routes for this peer via `bgp cache <id> forward`
2. Send EOR per family
3. Mark peer as up

## Known Limitations

| Limitation | Mitigation |
|------------|------------|
| Last-wins semantics (no best-path) | Routes converge quickly; peers do own selection |
| No per-peer filtering | All peers get same routes (except source exclusion) |
| Replay sends all cached UPDATEs | Peers handle duplicates gracefully |
| ADD-PATH not supported | Deferred — would eliminate last-wins problem |
| No shared RIB/PeerState package | bgp-persist copies types from bgp-rr (consider extracting later) |

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

## Implementation Summary

<!-- Fill after implementation -->

### What Was Implemented

### Bugs Found/Fixed

### Design Insights

### Documentation Updates

### Deviations from Plan

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| RR forwards UPDATEs to all except source | ✅ Done | `bgp-rr/server.go:171` | |
| RR stores routes in ribIn | ✅ Done | `bgp-rr/rib.go` | |
| RR peer down clears ribIn + sends withdrawals | ✅ Done | `bgp-rr/server.go:222` | |
| RR peer up replays from other peers | ✅ Done | `bgp-rr/server.go:233` | |
| RR filters by family | ✅ Done | `bgp-rr/server.go:183` | |
| RR handles route-refresh | ✅ Done | `bgp-rr/server.go:299` | |
| RR sends EOR after replay | | | |
| RR retains cache entries | | | |
| RR releases cache entries on withdrawal/down | | | |
| Persist plugin tracks sent routes | | | |
| Persist keeps ribOut on peer down | | | |
| Persist replays on peer up + EOR | | | |
| End-to-end functional tests | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestServer_HandleUpdate` | |
| AC-2 | ✅ Done | `TestServer_FilterUpdateByFamily` | |
| AC-3 | ✅ Done | `TestServer_HandleWithdraw` | |
| AC-4 | ⚠️ Partial | `TestServer_HandleStateDown` | Missing cache release |
| AC-5 | ⚠️ Partial | `TestServer_HandleStateUp` | Missing EOR |
| AC-6 | ✅ Done | `TestServer_HandleRefresh` | |
| AC-7 | | | Cache retain not implemented |
| AC-8 | | | Cache release not implemented |
| AC-9 | | | Persist not started |
| AC-10 | | | Persist not started |
| AC-11 | | | Persist not started |
| AC-12 | ✅ Done | `TestServer_HandleCommand_Status` | |
| AC-13 | ✅ Done | `TestServer_HandleCommand_Peers` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| (see unit tests table above) | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/bgp-rr/server.go` | ✅ Exists | Needs EOR + cache lifecycle |
| `internal/plugins/bgp-rr/server_test.go` | ✅ Exists | Needs new tests |
| `internal/plugins/bgp-persist/server.go` | | Not created |
| `internal/plugins/bgp-persist/register.go` | | Not created |
| `internal/plugins/bgp-persist/server_test.go` | | Not created |

### Audit Summary
- **Total items:** 25
- **Done:** 14
- **Partial:** 2 (EOR, cache lifecycle)
- **Not started:** 9 (persist plugin + functional tests)
- **Changed:** 0

## Checklist

### Goal Gates (MUST pass — cannot defer)
- [ ] Acceptance criteria AC-1..AC-13 all demonstrated
- [ ] Tests pass (`make test`)
- [ ] No regressions (`make functional`)
- [ ] Feature code integrated into codebase (`internal/plugins/bgp-rr/`, `internal/plugins/bgp-persist/`)

### Quality Gates (SHOULD pass — can defer with explicit user approval)
- [ ] `make ze-lint` passes
- [ ] Architecture docs updated with learnings
- [ ] Implementation Audit fully completed
- [ ] Mistake Log escalation candidates reviewed

### 🏗️ Design
- [ ] No premature abstraction (shared RIB package deferred until 3+ users)
- [ ] No speculative features (ADD-PATH deferred)
- [ ] Single responsibility (RR and persist are separate plugins)
- [ ] Explicit behavior (forward-all model, no hidden best-path)
- [ ] Minimal coupling (plugins use SDK, not direct engine imports)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Functional tests verify end-to-end behavior

### Documentation
- [ ] Required docs read
- [ ] RFC references added to code

### Completion
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-plugin-rr.md`
- [ ] All files committed together

## References

- RFC 4271 — BGP-4 (End-of-RIB marker)
- RFC 4456 — BGP Route Reflection
- RFC 7947 — Internet Exchange BGP Route Server
- `docs/architecture/api/capability-contract.md` — Engine API surface
- `docs/architecture/rib-transition.md` — RIB ownership model
