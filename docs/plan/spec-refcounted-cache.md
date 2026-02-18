# Spec: refcounted-cache

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/plugins/bgp/reactor/recent_cache.go` — current cache implementation
3. `internal/plugins/bgp/reactor/reactor.go` — `notifyMessageReceiver` (line 4069), `ForwardUpdate` (line 3155)
4. `internal/plugins/bgp/server/events.go` — `onMessageReceived` event dispatch

## Task

### Phase 1 — Refcounted cache (DONE)

~~Prevent UPDATE loss when the RR plugin is too slow to process events. Replace boolean `retained` flag with reference-counted cache entries. Entries are evicted only when all consumers have finished. Accept increased memory usage.~~

Phase 1 is fully implemented: `consumers int32`, `Activate()`, `Decrement()`, `pending` flag, safety valve (5 min), soft limit (never rejects). `Take()` removed, replaced by `Get()` + `Decrement()`.

~~Three failure modes being fixed:~~
~~1. **Cache full**: `Add()` rejects new entries at max capacity — UPDATE silently dropped~~
~~2. **TTL expiry**: Entry expires between event dispatch and plugin's `forward` call~~
~~3. **Destructive Take()**: `ForwardUpdate` removes the entry — only one consumer ever~~

### Phase 2 — Mandatory consumer acknowledgment

Replace TTL-based eviction with mandatory consumer acknowledgment. The cache must never discard an entry based on time — entries are evicted ONLY when all consumers have explicitly acted (forward or drop).

**Design principles:**
1. **No TTL eviction** — entries remain in cache until all subscribed plugins respond with "forward" or "drop". TTL is removed entirely as an eviction mechanism.
2. **Mandatory action** — every plugin that subscribes to UPDATE events MUST respond to every update it receives. No silent ignoring. A plugin that subscribes is obligated to ack every entry.
3. **FIFO ordering** — plugin acknowledgments MUST follow receive order. A plugin cannot ack update N before it has acked updates 1..N-1. Buffering within the plugin is permitted, but acks to the engine must be ordered.
4. **Gap-based safety valve** — timeout applies only to entries that have been **passed over**: a later entry has been fully acked by all plugins, but this older entry still has consumers > 0. This detects crashed or stuck plugins without penalizing slow-but-correct processing. An entry at the processing frontier (nothing after it is fully acked) is never timed out — it's just slow.
5. **Immediate eviction on last ack** — when `Decrement()` drops consumers to 0, evict the entry immediately (return buffer to pool, delete from map). No TTL delay, no lazy cleanup scan. The normal hot path is O(1).
6. **Safety-valve-only scan** — the periodic cleanup scan is only needed for gap detection (entries passed over). Normal entries are evicted immediately by `Decrement()`. The scan runs infrequently (every 30-60s) and only matters for fault detection.
7. **Cache grows as needed** — the soft limit warns but never rejects. Memory is the tradeoff for correctness.

**Failure modes addressed by Phase 2:**
1. **Silent drop via TTL** — plugin too slow → entry expires → UPDATE lost silently. Fix: no TTL, entry stays until acked.
2. **Plugin ignores update** — plugin receives event, never acts. Fix: mandatory ack — engine detects missing acks via FIFO gap.
3. **Out-of-order processing** — plugin processes update 5 before update 3, forwarding stale state. Fix: FIFO ordering enforcement.

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/buffer-first.md` — buffer ownership patterns
  → Constraint: poolBuf must be returned to pool exactly once

### Source Files
- [ ] `internal/plugins/bgp/reactor/recent_cache.go` — current cache: `cacheEntry.retained bool`, `Add()`, `Take()`, `Retain()`, `Release()`
  → Decision: `retained bool` is insufficient — need `consumers int32` refcount
- [ ] `internal/plugins/bgp/reactor/reactor.go:4069-4182` — `notifyMessageReceiver`: dispatches event THEN inserts into cache
  → Constraint: must reorder to insert BEFORE dispatch so entry exists when plugin receives msg-id
- [ ] `internal/plugins/bgp/reactor/reactor.go:3155-3280` — `ForwardUpdate`: uses destructive `Take()` + `defer update.Release()`
  → Decision: replace with non-destructive `Get()` + `Decrement()`
- [ ] `internal/plugins/bgp/server/events.go:23-55` — `onMessageReceived`: iterates subscribed processes, `len(procs)` is the consumer count
  → Decision: return `len(procs)` so reactor knows consumer count for refcount
- [ ] `internal/plugin/server.go:1298-1306` — `Server.OnMessageReceived`: delegates to hooks
  → Decision: change return type to `int` to propagate consumer count
- [ ] `internal/plugin/types.go:30` — `BGPHooks.OnMessageReceived` signature
  → Decision: change to return `int`

**Key insights:**
- `messageID` is assigned at reactor.go:4104 before either dispatch or cache insertion — available for both
- Event dispatch chain: `reactor.notifyMessageReceiver()` → `plugin.Server.OnMessageReceived()` → `BGPHooks.OnMessageReceived()` → `server.onMessageReceived()` → `SendDeliverEvent()` per process
- `SendDeliverEvent` is synchronous RPC with 5s timeout per plugin, so a fast plugin CAN call `forward` before dispatch to other plugins completes

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/bgp/reactor/recent_cache.go` — `cacheEntry` has `retained bool`. `Add()` rejects when full (returns false). `Take()` removes from map. `Retain()` sets bool. `Release()` clears bool + resets TTL. Lazy eviction skips retained entries.
- [ ] `internal/plugins/bgp/reactor/reactor.go` — lines 4162-4178: `OnMessageReceived()` dispatched FIRST, then `recentUpdates.Add()` SECOND. Lines 3155-3280: `ForwardUpdate` calls `Take()` (destructive), `defer update.Release()` (returns poolBuf to pool).
- [ ] `internal/plugins/bgp/reactor/received_update.go` — `Release()` returns `poolBuf` to session pool via `ReturnReadBuffer()`, idempotent
- [ ] `internal/plugins/bgp/server/events.go` — lines 23-55: `onMessageReceived` uses `s.Subscriptions().GetMatching()` to find processes, iterates sequentially with 5s timeout each

**Behavior to preserve:**
- Cache lookup by `uint64` message-id
- ~~TTL-based expiration with lazy cleanup~~ (Phase 2: TTL removed — see below)
- `bgp cache N forward selector` command dispatches to `ForwardUpdate`
- `bgp cache N retain` / `bgp cache N release` API commands (backward compat)
- Zero-copy forwarding path in `ForwardUpdate` when encoding contexts match
- Wire splitting for oversized UPDATEs

**Behavior to change (Phase 1 — DONE):**
- ~~`Add()` currently rejects when full → always accepts (soft limit with warning)~~ ✅
- ~~`Take()` is destructive → replaced by `Get()` (non-destructive) + `Decrement()`~~ ✅
- ~~`retained bool` → `consumers int32` refcount~~ ✅
- ~~Event dispatch ordering: dispatch AFTER cache insertion (not before)~~ ✅
- ~~Buffer ownership: cache always owns buffer, `ForwardUpdate` doesn't call `Release()`~~ ✅

**Behavior to change (Phase 2 — mandatory ack):**
- TTL-based eviction → removed entirely. Entries evicted only when all consumers ack.
- `ttl` field on `RecentUpdateCache` → removed. Constructor no longer takes `ttl` parameter.
- Lazy cleanup scan → replaced by ack-driven eviction. When last consumer acks, entry is immediately evictable.
- Safety valve remains (5 min) but is ONLY for crashed plugin detection, not normal operation.
- New `bgp cache N drop` command — plugin explicitly drops an update without forwarding.
- FIFO ordering — engine tracks per-plugin sequence numbers and rejects out-of-order acks.
- Per-plugin ack tracking — engine knows which plugins have acked which updates, can detect stalled plugins.

## Data Flow (MANDATORY)

### Entry Point
- Received UPDATE wire bytes from BGP peer session

### Transformation Path (Phase 1 — DONE)
1. `reactor.notifyMessageReceiver()` — assigns messageID, creates `RawMessage`
2. `recentUpdates.Add(update)` — insert with pending flag, not evictable ✅
3. `receiver.OnMessageReceived()` → dispatches JSON event to N subscribed plugins, returns N ✅
4. `recentUpdates.Activate(id, N)` — sets consumer refcount ✅
5. Plugin receives event, parses, calls `updateRoute("*", "bgp cache N forward selector")`
6. Engine dispatches to `handleBgpCacheForward` → `ForwardUpdate(sel, id)`
7. `ForwardUpdate` calls `Get(id)` (non-destructive), sends to peers, calls `Decrement(id)` ✅
8. When all consumers decrement to 0: entry becomes TTL-evictable, lazy cleanup returns buffer to pool

### Transformation Path (Phase 2 — mandatory ack)
Steps 1-7 remain the same. Changes to step 5 and 8:

5a. Plugin receives event, parses, decides action:
    - Forward: `updateRoute("*", "bgp cache N forward selector")` — same as today
    - Drop: `updateRoute("*", "bgp cache N drop")` — NEW command, explicit discard
    - Plugin MUST do one of these for every update. No silent ignore.

5b. Engine validates FIFO ordering per plugin:
    - Engine tracks `lastAckedID` per plugin
    - Ack for update N rejected if N > lastAckedID + 1 (gap = missing ack)
    - Plugin can batch: ack N implicitly acks all updates up to N from that plugin

8. When all consumers decrement to 0: entry immediately evicted, buffer returned to pool. No TTL delay.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor → Plugin Server | `MessageReceiver.OnMessageReceived()` returns `int` | [ ] |
| Plugin Server → Hooks | `BGPHooks.OnMessageReceived` returns `int` | [ ] |
| Hooks → events.go | `onMessageReceived` returns `len(procs)` | [ ] |
| Plugin → Engine | `updateRoute` RPC → `handleBgpCacheForward` → `ForwardUpdate` | [ ] |

### Integration Points
- `reactor.MessageReceiver` interface — connects reactor to plugin server, changed to return `int`
- `plugin.BGPHooks.OnMessageReceived` — connects plugin server to BGP event dispatch, changed to return `int`
- `handleBgpCacheForward` in `handler/cache.go` — connects plugin `forward` command to `ForwardUpdate`, unchanged
- `ReceivedUpdate.Release()` — connects cache eviction to buffer pool return, now called only by cache

### Architectural Verification
- [ ] No bypassed layers — data flows reactor → server → hooks → events → plugins
- [ ] No unintended coupling — cache refcount managed entirely within reactor package
- [ ] No duplicated functionality — extends existing cache, no new cache
- [ ] Zero-copy preserved — `Get()` returns read-only reference, no buffer copy

## Acceptance Criteria

### Phase 1 (DONE)

| AC ID | Input / Condition | Expected Behavior | Status |
|-------|-------------------|-------------------|--------|
| AC-1 | Cache at max entries, all entries have consumers > 0 | New `Add()` succeeds (soft limit), warning logged | ✅ |
| AC-2 | Two plugins subscribe to UPDATE events | Cache entry gets consumers=2, each `Decrement` reduces by 1, entry evictable when 0 | ✅ |
| AC-3 | Plugin calls `forward` before `Activate()` completes | `Get()` returns entry, `Decrement()` makes consumers negative, `Activate(N)` yields correct net N-1 | ✅ |
| AC-4 | Plugin crashes without calling `forward` | Safety valve evicts entry after 5 minutes | ✅ |
| AC-5 | Zero subscribers for UPDATE events | `Activate(id, 0)` → entry goes to normal TTL eviction | ✅ |
| AC-6 | `bgp cache N retain` / `bgp cache N release` commands | Backward compatible — Retain increments consumers, Release decrements | ✅ |
| AC-7 | `ForwardUpdate` with refcounted entry | Uses `Get()` (non-destructive) + `Decrement()`, buffer NOT released by ForwardUpdate | ✅ |
| AC-8 | Concurrent `Decrement()` from multiple goroutines | Atomic, race-free | ✅ |

### Phase 2 (mandatory ack)

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-9 | Entry with all consumers at 0 (all acked), no TTL field | Entry immediately evicted, buffer returned to pool |
| AC-10 | Entry with consumers > 0, later entry fully acked, timeout elapsed | Safety valve evicts (gap detected — plugin passed over this entry) |
| AC-11 | Plugin calls `bgp cache N drop` | Consumer count decremented, entry NOT forwarded, ack recorded |
| AC-12 | Plugin sends ack for update N without acking N-1 | Engine rejects: FIFO violation. Plugin must ack in receive order |
| AC-13 | Plugin sends ack for update N, implicitly acking 1..N | All entries 1..N decremented for that plugin |
| AC-14 | RR plugin receives update, does neither forward nor drop, but later entries are fully acked | Engine detects gap (passed-over entry), force-evicts after timeout |
| AC-15 | Cache has 500K entries, all with consumers > 0 | No eviction, cache grows as needed. Soft limit warns (rate-limited), never rejects |
| AC-16 | `NewRecentUpdateCache` constructor | No `ttl` parameter. Cache does not have TTL-based eviction |
| AC-17 | Entry with consumers > 0, no later entry fully acked, timeout elapsed | NOT evicted — entry is at the processing frontier, plugin is just slow |

## 🧪 TDD Test Plan

### Unit Tests (Phase 1 — DONE)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCacheRefcountBasic` | `recent_cache_test.go` | Add with consumers=2, two Decrements, then evictable | ✅ |
| `TestCacheGetNonDestructive` | `recent_cache_test.go` | Get doesn't remove entry, multiple Gets work | ✅ |
| `TestCacheDecrementBeforeActivate` | `recent_cache_test.go` | Decrement before Activate, net count correct | ✅ |
| `TestCacheActivateZeroConsumers` | `recent_cache_test.go` | No subscribers, entry evicts by TTL | ✅ |
| `TestCacheSoftLimit` | `recent_cache_test.go` | Add succeeds when exceeding maxEntries | ✅ |
| `TestCacheSafetyValve` | `recent_cache_test.go` | Entry retained > 5 min force-evicted | ✅ |
| `TestCacheConcurrentDecrement` | `recent_cache_test.go` | Race-safe concurrent Decrements | ✅ |
| `TestCacheRetainIncrementsConsumers` | `recent_cache_test.go` | Retain adds 1, Release subtracts 1 (backward compat) | ✅ |
| `TestCacheBufferReturnedOnEviction` | `recent_cache_test.go` | Buffer returned to pool only when evicted with consumers=0 | ✅ |

### Unit Tests (Phase 2 — mandatory ack)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCacheNoTTLEviction` | `recent_cache_test.go` | Entry with consumers > 0 is never evicted by time alone | |
| `TestCacheImmediateEvictOnZeroConsumers` | `recent_cache_test.go` | Entry evicted immediately when last consumer acks (no TTL wait) | |
| `TestCacheDropCommand` | `recent_cache_test.go` | Drop decrements consumer, does not forward | |
| `TestCacheFIFOOrdering` | `recent_cache_test.go` | Out-of-order ack rejected per plugin | |
| `TestCacheFIFOImplicitAck` | `recent_cache_test.go` | Ack for N implicitly acks 1..N for that plugin | |
| `TestCachePerPluginTracking` | `recent_cache_test.go` | Engine tracks which plugins have acked which updates | |
| `TestCacheStalledPluginDetection` | `recent_cache_test.go` | Stalled plugin detected via gap-based safety valve, entries force-evicted | |
| `TestCacheNoTTLConstructor` | `recent_cache_test.go` | Constructor takes no TTL parameter | |
| `TestCacheNoTimeoutAtFrontier` | `recent_cache_test.go` | Entry at processing frontier (no later entry fully acked) is never timed out | |
| `TestCacheGapDetection` | `recent_cache_test.go` | Entry with consumers > 0 where later entry is fully acked triggers gap-based timeout | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| consumers | 0+ | N/A (unbounded) | N/A (negative is valid during race) | N/A |
| safety valve | 5 min | 5 min (evicted) | 4m59s (not evicted) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing chaos functional test | `make chaos-functional-test` | Exercises RR forwarding under load | |
| Cache drop command | `test/plugin/` | Plugin sends `bgp cache N drop`, entry not forwarded | |
| FIFO ordering violation | `test/plugin/` | Plugin acks out of order, engine rejects | |

## Files to Modify

### Phase 1 (DONE)
- ~~`internal/plugins/bgp/reactor/recent_cache.go` — Core refcount: `consumers int32`, `pending bool`, `retainedAt time.Time`. New methods: `Activate()`, `Decrement()`. Change `Add()` to always accept. Change `Get()` to return `*ReceivedUpdate`. Remove `Take()`. Soft limit. Safety valve.~~ ✅
- ~~`internal/plugins/bgp/reactor/reactor.go` — Reorder `notifyMessageReceiver`: Add → dispatch → Activate. Change `ForwardUpdate`: `Get()` + `Decrement()` instead of `Take()` + `Release()`. Change `MessageReceiver` interface to return `int`.~~ ✅
- ~~`internal/plugins/bgp/reactor/received_update.go` — Update `Release()` docs~~ ✅
- ~~`internal/plugins/bgp/reactor/recent_cache_test.go` — Update existing tests, add 9 new tests~~ ✅
- ~~`internal/plugins/bgp/reactor/reactor_test.go` — Update `testMessageReceiver` mock to return `int`~~ ✅
- ~~`internal/plugin/server.go` — `Server.OnMessageReceived` returns `int`~~ ✅
- ~~`internal/plugin/types.go` — `BGPHooks.OnMessageReceived` returns `int`~~ ✅
- ~~`internal/plugins/bgp/server/events.go` — `onMessageReceived` returns `len(procs)`~~ ✅
- ~~`internal/plugins/bgp/server/hooks.go` — Closure propagates `int` return~~ ✅

### Phase 2 (mandatory ack)
- `internal/plugins/bgp/reactor/recent_cache.go` — Remove TTL field, remove `ttl` from constructor, remove TTL-based expiry from all methods. Add per-plugin ack tracking. Immediate eviction when consumers reach 0.
- `internal/plugins/bgp/reactor/reactor.go` — Add `bgp cache N drop` command handler. Add FIFO ordering validation per plugin. Track per-plugin `lastAckedID`.
- `internal/plugins/bgp/reactor/recent_cache_test.go` — Add Phase 2 tests (no-TTL, drop, FIFO, stalled plugin)
- `internal/plugins/bgp/handler/cache.go` — Add `handleBgpCacheDrop` handler for the drop command
- `pkg/plugin/sdk/sdk.go` — Add `DropUpdate(ctx, id)` SDK method for plugins to call drop

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] Yes — `bgp cache N drop` | `internal/yang/modules/*.yang` |
| CLI commands/flags | [ ] No | |
| Plugin SDK docs | [x] Yes — new `DropUpdate` method | `.claude/rules/plugin-design.md` |
| Functional test for new RPC/API | [x] Yes — drop command + FIFO | `test/plugin/` |

## Files to Create

None — all changes modify existing files.

## Implementation Steps

### Phase 1 (DONE)

~~1. **Write unit tests** — New cache tests with new API signatures~~ ✅
~~2. **Run tests** — Verify FAIL~~ ✅
~~3. **Implement recent_cache.go** — `consumers int32`, `Activate()`, `Decrement()`, soft limit, safety valve~~ ✅
~~4. **Run cache tests** — Verify PASS~~ ✅
~~5. **Change interface chain** — `MessageReceiver` → `BGPHooks` → `Server` → `events.go` → `hooks.go` all return `int`~~ ✅
~~6. **Implement reactor.go changes** — Reorder notifyMessageReceiver, change ForwardUpdate~~ ✅
~~7. **Run all tests** — `make ze-lint && make ze-unit-test`~~ ✅
~~8. **Final self-review** — Re-read all changes, check for races, unused code, debug statements~~ ✅

### Phase 2 (mandatory ack)

1. **Write Phase 2 unit tests** — No-TTL eviction, drop command, FIFO ordering, stalled plugin detection
   → **Review:** Coverage for all Phase 2 ACs?

2. **Run tests** — Verify FAIL for the right reasons

3. **Remove TTL from cache** — Remove `ttl` field, remove TTL from constructor, change eviction to ack-only + safety valve
   → **Review:** All callers of `NewRecentUpdateCache` updated? Existing tests still pass?

4. **Add per-plugin ack tracking** — Track `lastAckedID` per plugin process. Validate FIFO ordering on ack.
   → **Review:** Data structure for tracking per-plugin state? Cleanup when plugin disconnects?

5. **Add `bgp cache N drop` handler** — New command in cache handler, decrements consumer without forwarding
   → **Review:** Same FIFO validation as forward? Plugin SDK method added?

6. **Add FIFO enforcement** — Engine validates ack ordering. Implicit ack for updates below N when acking N.
   → **Review:** What happens when plugin acks N but hasn't acked N-1? Error response? Log warning?

7. **Update SDK** — Add `DropUpdate(ctx, id)` method. Document mandatory ack obligation.
   → **Review:** SDK docs updated? Plugin examples updated?

8. **Run all tests** — `make ze-lint && make ze-unit-test && make ze-functional-test`

9. **Final self-review** — Verify all Phase 2 ACs demonstrated, FIFO ordering correct under concurrency

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error after interface change | Step 5 — find all callers |
| Race detector failure | Step 4 — check per-plugin tracking under concurrency |
| Buffer pool corruption | Step 3 — verify Release called exactly once |
| FIFO test failure | Step 6 — verify implicit ack logic |

## Design Insights

- The ordering change (cache before dispatch) is the key architectural fix. Without it, there's always a window where the plugin has the msg-id but the cache doesn't have the entry.
- Allowing negative consumer counts during the Activate race simplifies the design enormously — no locks needed between dispatch and activation.
- Soft limit (vs hard rejection) is the correct choice for a route reflector — dropping updates is worse than using more memory.

### Phase 2 design rationale

- **TTL is fundamentally wrong for a route reflector.** The RR's job is to forward every update. Time-based eviction means "if the plugin is slow, silently drop updates." That's data loss, not acceptable behavior. The only valid eviction trigger is explicit consumer acknowledgment.
- **Mandatory ack prevents silent failures.** If a plugin subscribes to updates, it is obligated to process them. An update that sits unacked is a bug (stalled plugin), detected by the safety valve — not normal operation masked by TTL.
- **FIFO ordering prevents forwarding stale state.** BGP uses implicit withdrawals — a new announcement for the same prefix replaces the old one. If the RR processes updates out of order, it can forward an obsolete path followed by the correct one, causing unnecessary route flaps downstream. FIFO ensures the final forwarded state matches the received state.
- **Implicit ack up to N simplifies plugin design.** A plugin that processes updates in order doesn't need to ack each one individually. Acking update 100 means "I've handled everything up to 100." This is the same pattern as TCP ACK numbers.
- **Gap-based safety valve is more precise than wall-clock timeout.** A 5-minute timeout penalizes slow plugins equally whether they're working through a backlog or genuinely stuck. Gap detection only fires when evidence exists that processing moved past the entry — a later entry is fully acked but this one isn't. This distinguishes "slow but making progress" from "skipped or stuck."
- **Immediate eviction in Decrement eliminates the O(n) scan for normal entries.** The cleanup scan is only needed for gap detection (rare fault condition). Normal operation is O(1) per ack — no map iteration, no amortized cost.

### Count-based consumer tracking (replaces name-based)

Per-entry tracking uses an integer counter (`pendingConsumers int`) instead of a name set (`consumerSet map[string]bool`). The cache doesn't need to know WHO the consumers are per-entry — only how many remain.

| Tracking | Scope | Purpose |
|----------|-------|---------|
| `pendingConsumers int` | Per-entry | How many acks still needed |
| `earlyAckCount int` | Per-entry | Early acks before Activate |
| `pluginLastAck map[string]uint64` | Global | FIFO enforcement + unregistration cleanup |
| `highestAddedID uint64` | Global | Initialize new consumer's lastAck to skip pre-registration entries |

- `Activate(id, count int)` takes a count, not a name list. Sets `pendingConsumers = count - earlyAckCount`.
- `Ack(id, pluginName)` still takes a name for FIFO tracking, but per-entry it just decrements `pendingConsumers`.
- `RegisterConsumer(name)` initializes `pluginLastAck[name] = highestAddedID` so implicit acks skip pre-registration entries.
- `UnregisterConsumer(name)` walks entries with `id > pluginLastAck[name]`, decrements their `pendingConsumers`, evicts if zero.

**Why not names per-entry:** The empty-string bug (acking with `""` polluted `pluginLastAck`) revealed that per-entry name tracking couples the cache to identity details it doesn't need. A counter is simpler, avoids the map allocation per entry, and is sufficient for the eviction decision.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Refcounted cache entries | ✅ Done | `recent_cache.go` | Phase 1 |
| No update loss when cache full | ✅ Done | `recent_cache.go:Add()` | Soft limit, never rejects |
| Evict only when all consumers done | ✅ Done | `recent_cache.go:Decrement()` | Phase 1 |
| Safety valve for crashed plugins | ✅ Done | `recent_cache.go:isProtected()` | 5 min timeout |
| Remove TTL-based eviction | | | Phase 2 |
| Mandatory consumer ack (forward or drop) | | | Phase 2 |
| FIFO ordering enforcement | | | Phase 2 |
| `bgp cache N drop` command | | | Phase 2 |
| Per-plugin ack tracking | | | Phase 2 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestRecentUpdateCacheSoftLimit` | |
| AC-2 | ✅ Done | `TestRecentCacheWithFakeClock` + `TestRecentUpdateCacheConcurrency` | |
| AC-3 | ✅ Done | `TestRecentUpdateCache*` (Decrement before Activate) | |
| AC-4 | ✅ Done | `TestRecentUpdateCache*` (safety valve) | |
| AC-5 | ✅ Done | `TestRecentUpdateCache*` (zero consumers) | |
| AC-6 | ✅ Done | `TestRecentUpdateCacheRetain`, `TestRecentUpdateCacheRelease` | |
| AC-7 | ✅ Done | `ForwardUpdate` uses `Get()` + `Decrement()` | |
| AC-8 | ✅ Done | `TestRecentUpdateCacheConcurrency` | |
| AC-9 | | | Phase 2 |
| AC-10 | | | Phase 2 |
| AC-11 | | | Phase 2 |
| AC-12 | | | Phase 2 |
| AC-13 | | | Phase 2 |
| AC-14 | | | Phase 2 |
| AC-15 | | | Phase 2 |
| AC-16 | | | Phase 2 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Phase 1 tests | ✅ Done | `recent_cache_test.go` | 17+ tests passing |
| Phase 2 tests | | | Not yet written |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Phase 1 files | ✅ Done | All 9 files modified |
| Phase 2 files | | Not yet started |

### Audit Summary
- **Total items:** 17 (9 Phase 1 + 8 Phase 2)
- **Done:** 9 (Phase 1 complete)
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0
- **Pending:** 8 (Phase 2)

## Checklist

### Goal Gates — Phase 1 (DONE)
- [x] Acceptance criteria AC-1..AC-8 all demonstrated
- [x] Tests pass (`make ze-unit-test`)
- [x] No regressions (`make ze-lint`)
- [x] No regressions (`make ze-functional-test`)
- [x] Integration test: cache refcount exercised via chaos functional test

### Goal Gates — Phase 2 (mandatory ack)
- [ ] Acceptance criteria AC-9..AC-16 all demonstrated
- [ ] TTL removed from cache entirely
- [ ] `bgp cache N drop` command implemented and tested
- [ ] FIFO ordering enforced per plugin
- [ ] Per-plugin ack tracking implemented
- [ ] SDK `DropUpdate` method added
- [ ] Tests pass (`make ze-unit-test`)
- [ ] No regressions (`make ze-functional-test`)
- [ ] Integration test: drop command + FIFO ordering via functional test

### Quality Gates
- [ ] `make ze-lint` passes
- [ ] Implementation Audit fully completed (Phase 1 + Phase 2)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
