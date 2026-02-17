# Spec: refcounted-cache

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/plugins/bgp/reactor/recent_cache.go` — current cache implementation
3. `internal/plugins/bgp/reactor/reactor.go` — `notifyMessageReceiver` (line 4069), `ForwardUpdate` (line 3155)
4. `internal/plugins/bgp/server/events.go` — `onMessageReceived` event dispatch

## Task

Prevent UPDATE loss when the RR plugin is too slow to process events. Replace boolean `retained` flag with reference-counted cache entries. Entries are evicted only when all consumers have finished. Accept increased memory usage.

Three failure modes being fixed:
1. **Cache full**: `Add()` rejects new entries at max capacity — UPDATE silently dropped
2. **TTL expiry**: Entry expires between event dispatch and plugin's `forward` call
3. **Destructive Take()**: `ForwardUpdate` removes the entry — only one consumer ever

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
- TTL-based expiration with lazy cleanup
- `bgp cache N forward selector` command dispatches to `ForwardUpdate`
- `bgp cache N retain` / `bgp cache N release` API commands (backward compat)
- Zero-copy forwarding path in `ForwardUpdate` when encoding contexts match
- Wire splitting for oversized UPDATEs

**Behavior to change:**
- `Add()` currently rejects when full → always accepts (soft limit with warning)
- `Take()` is destructive → replaced by `Get()` (non-destructive) + `Decrement()`
- `retained bool` → `consumers int32` refcount
- Event dispatch ordering: dispatch AFTER cache insertion (not before)
- Buffer ownership: cache always owns buffer, `ForwardUpdate` doesn't call `Release()`

## Data Flow (MANDATORY)

### Entry Point
- Received UPDATE wire bytes from BGP peer session

### Transformation Path
1. `reactor.notifyMessageReceiver()` — assigns messageID, creates `RawMessage`
2. **NEW**: `recentUpdates.Add(update, 0)` — insert with pending flag, not evictable
3. `receiver.OnMessageReceived()` → dispatches JSON event to N subscribed plugins, returns N
4. **NEW**: `recentUpdates.Activate(id, N)` — sets consumer refcount
5. Plugin receives event, parses, calls `updateRoute("*", "bgp cache N forward selector")`
6. Engine dispatches to `handleBgpCacheForward` → `ForwardUpdate(sel, id)`
7. **NEW**: `ForwardUpdate` calls `Get(id)` (non-destructive), sends to peers, calls `Decrement(id)`
8. When all consumers decrement to 0: entry becomes TTL-evictable, lazy cleanup returns buffer to pool

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

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Cache at max entries, all entries have consumers > 0 | New `Add()` succeeds (soft limit), warning logged |
| AC-2 | Two plugins subscribe to UPDATE events | Cache entry gets consumers=2, each `Decrement` reduces by 1, entry evictable when 0 |
| AC-3 | Plugin calls `forward` before `Activate()` completes | `Get()` returns entry, `Decrement()` makes consumers negative, `Activate(N)` yields correct net N-1 |
| AC-4 | Plugin crashes without calling `forward` | Safety valve evicts entry after 5 minutes |
| AC-5 | Zero subscribers for UPDATE events | `Activate(id, 0)` → entry goes to normal TTL eviction |
| AC-6 | `bgp cache N retain` / `bgp cache N release` commands | Backward compatible — Retain increments consumers, Release decrements |
| AC-7 | `ForwardUpdate` with refcounted entry | Uses `Get()` (non-destructive) + `Decrement()`, buffer NOT released by ForwardUpdate |
| AC-8 | Concurrent `Decrement()` from multiple goroutines | Atomic, race-free |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCacheRefcountBasic` | `recent_cache_test.go` | Add with consumers=2, two Decrements, then evictable | |
| `TestCacheGetNonDestructive` | `recent_cache_test.go` | Get doesn't remove entry, multiple Gets work | |
| `TestCacheDecrementBeforeActivate` | `recent_cache_test.go` | Decrement before Activate, net count correct | |
| `TestCacheActivateZeroConsumers` | `recent_cache_test.go` | No subscribers, entry evicts by TTL | |
| `TestCacheSoftLimit` | `recent_cache_test.go` | Add succeeds when exceeding maxEntries | |
| `TestCacheSafetyValve` | `recent_cache_test.go` | Entry retained > 5 min force-evicted | |
| `TestCacheConcurrentDecrement` | `recent_cache_test.go` | Race-safe concurrent Decrements | |
| `TestCacheRetainIncrementsConsumers` | `recent_cache_test.go` | Retain adds 1, Release subtracts 1 (backward compat) | |
| `TestCacheBufferReturnedOnEviction` | `recent_cache_test.go` | Buffer returned to pool only when evicted with consumers=0 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| consumers | 0+ | N/A (unbounded) | N/A (negative is valid during race) | N/A |
| safety valve | 5 min | 5 min (evicted) | 4m59s (not evicted) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing chaos functional test | `make chaos-functional-test` | Exercises RR forwarding under load | |

## Files to Modify

- `internal/plugins/bgp/reactor/recent_cache.go` — Core refcount: `consumers int32`, `pending bool`, `retainedAt time.Time`. New methods: `Activate()`, `Decrement()`. Change `Add()` to always accept. Change `Get()` to return `*ReceivedUpdate`. Remove `Take()`. Soft limit. Safety valve.
- `internal/plugins/bgp/reactor/reactor.go` — Reorder `notifyMessageReceiver`: Add → dispatch → Activate. Change `ForwardUpdate`: `Get()` + `Decrement()` instead of `Take()` + `Release()`. Change `MessageReceiver` interface to return `int`.
- `internal/plugins/bgp/reactor/received_update.go` — Update `Release()` docs
- `internal/plugins/bgp/reactor/recent_cache_test.go` — Update existing tests, add 9 new tests
- `internal/plugins/bgp/reactor/reactor_test.go` — Update `testMessageReceiver` mock to return `int`
- `internal/plugin/server.go` — `Server.OnMessageReceived` returns `int`
- `internal/plugin/types.go` — `BGPHooks.OnMessageReceived` returns `int`
- `internal/plugins/bgp/server/events.go` — `onMessageReceived` returns `len(procs)`
- `internal/plugins/bgp/server/hooks.go` — Closure propagates `int` return

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No | |
| CLI commands/flags | [ ] No | |
| Plugin SDK docs | [ ] No | |
| Functional test for new RPC/API | [ ] No — existing chaos test exercises the path | |

## Files to Create

None — all changes modify existing files.

## Implementation Steps

1. **Write unit tests** — New cache tests with new API signatures
   → **Review:** Coverage for refcount lifecycle, race, safety valve, soft limit?

2. **Run tests** — Verify FAIL
   → **Review:** Fail for the right reason (missing methods)?

3. **Implement recent_cache.go** — `consumers int32`, `Activate()`, `Decrement()`, soft limit, safety valve
   → **Review:** Buffer returned exactly once? Safety valve duration correct?

4. **Run cache tests** — Verify PASS

5. **Change interface chain** — `MessageReceiver` → `BGPHooks` → `Server` → `events.go` → `hooks.go` all return `int`
   → **Review:** All callers updated? Mock in reactor_test.go?

6. **Implement reactor.go changes** — Reorder notifyMessageReceiver, change ForwardUpdate
   → **Review:** Race between Add and Activate handled? Buffer not double-released?

7. **Run all tests** — `make ze-lint && make ze-unit-test`

8. **Final self-review** — Re-read all changes, check for races, unused code, debug statements

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error after interface change | Step 5 — find all callers |
| Race detector failure | Step 3 — check atomic ops in Decrement |
| Buffer pool corruption | Step 3 — verify Release called exactly once |

## Design Insights

- The ordering change (cache before dispatch) is the key architectural fix. Without it, there's always a window where the plugin has the msg-id but the cache doesn't have the entry.
- Allowing negative consumer counts during the Activate race simplifies the design enormously — no locks needed between dispatch and activation.
- Soft limit (vs hard rejection) is the correct choice for a route reflector — dropping updates is worse than using more memory.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Refcounted cache entries | | | |
| No update loss when cache full | | | |
| Evict only when all consumers done | | | |
| Safety valve for crashed plugins | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |
| AC-8 | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass)
- [ ] Acceptance criteria AC-1..AC-8 all demonstrated
- [ ] Tests pass (`make ze-unit-test`)
- [ ] No regressions (`make ze-lint`)
- [ ] No regressions (`make ze-functional-test`)
- [ ] Integration test: cache refcount exercised via chaos functional test

### Quality Gates
- [ ] `make ze-lint` passes
- [ ] Implementation Audit fully completed

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
