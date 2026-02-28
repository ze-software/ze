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

### Phase 2 — Mandatory consumer acknowledgment (DONE)

~~Replace TTL-based eviction with mandatory consumer acknowledgment. The cache must never discard an entry based on time — entries are evicted ONLY when all consumers have explicitly acted (forward or release).~~

Phase 2 is fully implemented: TTL removed entirely, `Ack()` replaces `Decrement()` for plugin consumers, FIFO cumulative ack via `seqmap.Since()`, per-plugin tracking via `pluginLastAck`, gap-based safety valve with background goroutine (`Start()`/`Stop()`), `RegisterConsumer()`/`UnregisterConsumer()` lifecycle, unordered consumer support (`SetConsumerUnordered()` for bgp-rs).

**Design changes from original plan:**
1. ~~`bgp cache N drop` command~~ — not needed. `release` serves as both release and drop (ack without forward).
2. ~~FIFO violation rejection~~ — out-of-order acks are silently accepted as no-ops (not rejected). Multi-peer delivery causes non-ID-ordered events; rejection would break real workloads.
3. ~~SDK `DropUpdate()` method~~ — not needed. Plugins use `release` to ack without forwarding.
4. **Unordered consumers** (not in original plan) — `SetConsumerUnordered()` added for bgp-rs per-source-peer workers that process entries out of global message ID order.
5. **Count-based + retain split** — `pendingConsumers int` (plugin consumers) + `retainCount int32` (API retains) replaces single `consumers int32`. `totalConsumers()` combines both.

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

**Behavior to change (Phase 2 — mandatory ack, DONE):**
- ~~TTL-based eviction → removed entirely. Entries evicted only when all consumers ack.~~ ✅
- ~~`ttl` field on `RecentUpdateCache` → removed. Constructor no longer takes `ttl` parameter.~~ ✅
- ~~Lazy cleanup scan → replaced by ack-driven eviction + gap-based background scan.~~ ✅
- ~~Safety valve → gap-based only (5 min), background goroutine (`Start()`/`Stop()`).~~ ✅
- ~~FIFO ordering — cumulative ack via `seqmap.Since()`, out-of-order acks are no-ops.~~ ✅
- ~~Per-plugin ack tracking — `pluginLastAck map[string]uint64`, `RegisterConsumer()`/`UnregisterConsumer()`.~~ ✅
- ~~Unordered consumer support — `SetConsumerUnordered()` for bgp-rs per-source-peer workers.~~ ✅

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

### Transformation Path (Phase 2 — mandatory ack, DONE)
Steps 1-7 remain the same. Changes to step 5 and 8:

~~5a. Plugin receives event, parses, decides action:~~
~~- Forward: `updateRoute("*", "bgp cache N forward selector")` → calls `ForwardUpdate` → deferred `Ack()`~~ ✅
~~- Release: `updateRoute("*", "bgp cache N release")` → calls `ReleaseUpdate` → `Ack()` without forwarding~~ ✅
~~- Plugin MUST do one of these for every update.~~ ✅

~~5b. Engine validates FIFO ordering per plugin:~~
~~- Engine tracks `pluginLastAck[plugin]` per registered consumer~~ ✅
~~- Ack for id <= lastAck is a no-op (already covered by cumulative ack)~~ ✅
~~- Cumulative ack: ack for id N implicitly acks all cached entries between lastAck+1 and N via `seqmap.Since()`~~ ✅

~~8. When all consumers ack to 0: entry immediately evicted via `evictLocked()`, buffers returned to pool. No TTL delay.~~ ✅

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

### Phase 2 (mandatory ack, DONE)

| AC ID | Input / Condition | Expected Behavior | Status |
|-------|-------------------|-------------------|--------|
| AC-9 | Entry with all consumers at 0 (all acked), no TTL field | Entry immediately evicted, buffer returned to pool | ✅ |
| AC-10 | Entry with consumers > 0, later entry fully acked, timeout elapsed | Safety valve evicts (gap detected — plugin passed over this entry) | ✅ |
| ~~AC-11~~ | ~~Plugin calls `bgp cache N drop`~~ | ~~Consumer count decremented, entry NOT forwarded~~ | Removed — `release` serves as drop |
| ~~AC-12~~ | ~~Plugin sends ack for update N without acking N-1~~ | ~~Engine rejects: FIFO violation~~ | 🔄 Changed — out-of-order ack is no-op, not rejection |
| AC-13 | Plugin sends ack for update N, implicitly acking 1..N | All cached entries between lastAck+1..N decremented for that plugin | ✅ |
| AC-14 | Plugin receives update, does neither forward nor release, but later entries are fully acked | Engine detects gap (passed-over entry), force-evicts after timeout | ✅ |
| AC-15 | Cache has 500K entries, all with consumers > 0 | No eviction, cache grows as needed. Soft limit warns (rate-limited), never rejects | ✅ |
| AC-16 | `NewRecentUpdateCache` constructor | No `ttl` parameter. Cache does not have TTL-based eviction | ✅ |
| AC-17 | Entry with consumers > 0, no later entry fully acked, timeout elapsed | NOT evicted — entry is at the processing frontier, plugin is just slow | ✅ |

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

### Unit Tests (Phase 2 — mandatory ack, DONE)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCacheNoTTLEviction` | `recent_cache_test.go` | Entry with consumers > 0 is never evicted by time alone | ✅ |
| `TestCacheImmediateEvictOnZeroConsumers` | `recent_cache_test.go` | Entry evicted immediately when last consumer acks | ✅ |
| `TestCacheNoTTLConstructor` | `recent_cache_test.go` | Constructor takes no TTL parameter | ✅ |
| `TestCacheFIFOOrdering` | `recent_cache_test.go` | FIFO cumulative ack, out-of-order is no-op | ✅ |
| `TestCacheFIFOImplicitAck` | `recent_cache_test.go` | Ack for N implicitly acks cached entries up to N | ✅ |
| `TestCacheFIFOPerPlugin` | `recent_cache_test.go` | Per-plugin independent FIFO tracking | ✅ |
| `TestCacheOutOfOrderAck` | `recent_cache_test.go` | Multi-peer delivery, large ID gaps, re-ack | ✅ |
| `TestCacheAckBeforeActivate` | `recent_cache_test.go` | Fast plugin acks before Activate | ✅ |
| `TestCacheAckBeforeActivateTwoPlugins` | `recent_cache_test.go` | Two plugins, fast+slow ack race | ✅ |
| `TestCacheSafetyValveGapDetection` | `recent_cache_test.go` | Stalled plugin detected via gap-based safety valve | ✅ |
| `TestCacheNoTimeoutAtFrontier` | `recent_cache_test.go` | Entry at frontier never timed out | ✅ |
| `TestCacheAckExpiredEntry` | `recent_cache_test.go` | Ack for evicted entry returns ErrUpdateExpired | ✅ |
| `TestCacheConcurrentAck` | `recent_cache_test.go` | Race-safe concurrent acks from multiple goroutines | ✅ |
| `TestCacheRetainPlusPluginConsumers` | `recent_cache_test.go` | API retain + plugin consumers interact correctly | ✅ |
| `TestCacheRegisterConsumer` | `recent_cache_test.go` | RegisterConsumer initializes pluginLastAck to highestAddedID | ✅ |
| `TestCacheUnregisterConsumer` | `recent_cache_test.go` | UnregisterConsumer decrements unacked entries | ✅ |
| `TestCacheUnregisterConsumerPartialAck` | `recent_cache_test.go` | UnregisterConsumer only affects entries above lastAck | ✅ |
| `TestCacheUnregisterUnknownConsumer` | `recent_cache_test.go` | UnregisterConsumer safe for unregistered plugins | ✅ |
| `TestSafetyValveConfigurable` | `recent_cache_test.go` | SetSafetyValveDuration overrides default | ✅ |
| `TestCacheUnorderedConsumerNoSweep` | `recent_cache_test.go` | Unordered consumer per-entry ack, no cumulative sweep | ✅ |
| `TestCacheUnorderedConsumerUnregister` | `recent_cache_test.go` | UnregisterConsumer walks all entries for unordered | ✅ |
| `TestCacheUnorderedConsumerReAck` | `recent_cache_test.go` | Unordered consumer re-ack below lastAck works | ✅ |
| `TestGapScanRunsInBackground` | `recent_cache_test.go` | Background goroutine runs gap scan on ticker | ✅ |
| `TestAddDoesNotRunGapScanInline` | `recent_cache_test.go` | Add() never triggers inline scan | ✅ |
| `TestAckCumulativeSkipsNonCachedIDs` | `recent_cache_test.go` | Cumulative ack skips ID gaps efficiently | ✅ |
| `TestUnregisterConsumerUsesSince` | `recent_cache_test.go` | FIFO unregister uses Since, not Range | ✅ |
| `TestStopCleansUpGoroutine` | `recent_cache_test.go` | Stop() shuts down background scan | ✅ |
| `TestStopIdempotent` | `recent_cache_test.go` | Stop() safe to call multiple times | ✅ |
| `TestStopWithoutStart` | `recent_cache_test.go` | Stop() safe without Start() | ✅ |
| `TestConcurrentAddAckWithBackground` | `recent_cache_test.go` | Concurrent Add/Ack with background scan, race-free | ✅ |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| consumers | 0+ | N/A (unbounded) | N/A (negative is valid during race) | N/A |
| safety valve | 5 min | 5 min (evicted) | 4m59s (not evicted) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing chaos functional test | `make ze-chaos-test` | Exercises RR forwarding under load with cache ack path | ✅ |
| rs-backpressure | `test/plugin/rs-backpressure.ci` | RS flow control wiring with cache consumer | ✅ |

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

### Phase 2 (mandatory ack, DONE)
- ~~`internal/plugins/bgp/reactor/recent_cache.go` — TTL removed, `Ack()` method, per-plugin tracking, gap-based safety valve, `RegisterConsumer`/`UnregisterConsumer`, `SetConsumerUnordered`, background goroutine~~ ✅
- ~~`internal/plugins/bgp/reactor/reactor_api_forward.go` — `ForwardUpdate` deferred `Ack()`, `ReleaseUpdate` with consumer path, `RegisterCacheConsumer`/`UnregisterCacheConsumer`~~ ✅
- ~~`internal/plugins/bgp/reactor/recent_cache_test.go` — 47 tests covering Phase 2 (FIFO, cumulative ack, gap scan, unordered consumers, lifecycle)~~ ✅
- ~~`internal/plugins/bgp/server/events.go` — `onMessageReceived` returns cache-consumer count (not total subscriber count)~~ ✅
- ~~`pkg/plugin/rpc/types.go` — `CacheConsumer` and `CacheConsumerUnordered` registration fields~~ ✅
- ~~`pkg/plugin/rpc/text.go` — `cache-consumer` and `cache-consumer-unordered` text protocol keywords~~ ✅

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No — no new RPCs | |
| CLI commands/flags | [ ] No | |
| Plugin SDK docs | [ ] No — `cache-consumer` documented in RPC types | |
| Functional test for new RPC/API | [x] Done — chaos test + rs-backpressure | `test/plugin/` |

## Files to Create

None — all changes modify existing files.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin declares `cache-consumer: true` in Stage 1 | → | `RegisterCacheConsumer()` in `reactor_api_forward.go` | `rs-backpressure.ci` (bgp-rs declares cache-consumer) |
| Received UPDATE wire bytes | → | `Add()` → `Activate()` in `recent_cache.go` | `TestCacheNoTTLEviction`, `TestCacheActivateZeroConsumersEvictsImmediately` |
| Plugin calls `bgp cache N forward selector` | → | `ForwardUpdate()` → deferred `Ack()` | `TestCacheFIFOOrdering`, `TestCacheFIFOImplicitAck` |
| Plugin calls `bgp cache N release` | → | `ReleaseUpdate()` → `Ack()` | `TestCacheImmediateEvictOnZeroConsumers` |
| Background ticker fires | → | `runGapScan()` → `isGapEvictable()` | `TestGapScanRunsInBackground`, `TestCacheSafetyValveGapDetection` |

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

### Phase 2 (mandatory ack, DONE)

~~1. **Write Phase 2 unit tests** — No-TTL, FIFO ordering, cumulative ack, gap detection, unordered consumers~~ ✅
~~2. **Run tests** — Verify FAIL~~ ✅
~~3. **Remove TTL from cache** — TTL field, constructor parameter, and TTL-based eviction all removed~~ ✅
~~4. **Add per-plugin ack tracking** — `pluginLastAck`, `RegisterConsumer`, `UnregisterConsumer`~~ ✅
~~5. **Add FIFO enforcement** — Cumulative ack via `seqmap.Since()`, out-of-order ack is no-op~~ ✅
~~6. **Add unordered consumer support** — `SetConsumerUnordered()` for bgp-rs~~ ✅
~~7. **Add background gap scan** — `Start()`/`Stop()` lifecycle, `gapScanLoop()`, `runGapScan()`~~ ✅
~~8. **Wire into reactor** — `ForwardUpdate` deferred `Ack()`, `ReleaseUpdate` consumer path, registration hooks~~ ✅
~~9. **Run all tests** — `make ze-lint && make ze-unit-test && make ze-functional-test`~~ ✅

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
| Remove TTL-based eviction | ✅ Done | `recent_cache.go` | No TTL anywhere |
| Mandatory consumer ack (forward or release) | ✅ Done | `reactor_api_forward.go:ForwardUpdate`, `ReleaseUpdate` | Ack() called by both paths |
| FIFO ordering enforcement | ✅ Done | `recent_cache.go:Ack()` | Cumulative ack via seqmap.Since() |
| Per-plugin ack tracking | ✅ Done | `recent_cache.go:pluginLastAck` | RegisterConsumer/UnregisterConsumer lifecycle |
| Unordered consumer support | ✅ Done | `recent_cache.go:SetConsumerUnordered` | Per-entry ack for bgp-rs |
| Background gap scan | ✅ Done | `recent_cache.go:Start()/Stop()` | Gap-based safety valve goroutine |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestRecentUpdateCacheSoftLimit` | |
| AC-2 | ✅ Done | `TestRecentCacheWithFakeClock` + `TestRecentUpdateCacheConcurrency` | |
| AC-3 | ✅ Done | `TestRecentUpdateCache*` (Decrement before Activate) | |
| AC-4 | ✅ Done | `TestRecentUpdateCache*` (safety valve) | |
| AC-5 | ✅ Done | `TestRecentUpdateCache*` (zero consumers) | |
| AC-6 | ✅ Done | `TestRecentUpdateCacheRetain`, `TestRecentUpdateCacheRelease` | |
| AC-7 | ✅ Done | `ForwardUpdate` uses `Get()` + deferred `Ack()` | |
| AC-8 | ✅ Done | `TestRecentUpdateCacheConcurrency` | |
| AC-9 | ✅ Done | `TestCacheImmediateEvictOnZeroConsumers` | Immediate eviction on last ack |
| AC-10 | ✅ Done | `TestCacheSafetyValveGapDetection` | Gap-based safety valve |
| AC-11 | Removed | N/A | `release` serves as drop — separate drop command not needed |
| AC-12 | 🔄 Changed | `TestCacheOutOfOrderAck` | Out-of-order ack is no-op, not rejection |
| AC-13 | ✅ Done | `TestCacheFIFOImplicitAck`, `TestAckCumulativeSkipsNonCachedIDs` | Cumulative ack |
| AC-14 | ✅ Done | `TestCacheSafetyValveGapDetection` | Gap detection + force-evict |
| AC-15 | ✅ Done | `TestRecentUpdateCacheSoftLimit` | Soft limit, never rejects |
| AC-16 | ✅ Done | `TestCacheNoTTLConstructor` | No TTL parameter |
| AC-17 | ✅ Done | `TestCacheNoTimeoutAtFrontier` | Frontier entries never timed out |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Phase 1 tests | ✅ Done | `recent_cache_test.go` | 17+ tests passing |
| Phase 2 tests | ✅ Done | `recent_cache_test.go` | 47 tests total (Phase 1 + Phase 2) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Phase 1 files | ✅ Done | All 9 files modified |
| Phase 2 files | ✅ Done | 6 files modified (cache, reactor_api_forward, events, rpc types, rpc text) |

### Audit Summary
- **Total items:** 17 (9 Phase 1 + 8 Phase 2)
- **Done:** 15
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (AC-12: no-op instead of rejection)
- **Removed:** 1 (AC-11: drop command superseded by release)

## Checklist

### Goal Gates — Phase 1 (DONE)
- [x] Acceptance criteria AC-1..AC-8 all demonstrated
- [x] Tests pass (`make ze-unit-test`)
- [x] No regressions (`make ze-lint`)
- [x] No regressions (`make ze-functional-test`)
- [x] Integration test: cache refcount exercised via chaos functional test

### Goal Gates — Phase 2 (mandatory ack, DONE)
- [x] Acceptance criteria AC-9..AC-16 all demonstrated (AC-11 removed, AC-12 changed)
- [x] TTL removed from cache entirely
- [x] FIFO ordering enforced per plugin (cumulative ack)
- [x] Per-plugin ack tracking implemented
- [x] Unordered consumer support added
- [x] Background gap scan implemented
- [x] Tests pass (`make ze-unit-test`)
- [x] No regressions (`make ze-functional-test`)
- [x] Integration test: chaos test + rs-backpressure

### Quality Gates
- [x] `make ze-lint` passes
- [x] Implementation Audit fully completed (Phase 1 + Phase 2)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL
- [x] Implementation complete
- [x] Tests PASS
