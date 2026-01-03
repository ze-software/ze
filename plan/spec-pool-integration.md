# Spec: Pool System Integration

## MANDATORY READING (BEFORE IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  STOP. Read these files FIRST before ANY implementation:        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. .claude/INDEX.md - Find what docs to load                   │
│  3. plan/CLAUDE_CONTINUATION.md - Current state                 │
│  4. THIS SPEC FILE - Design requirements                        │
│  5. internal/store/*.go - Current implementation                │
│                                                                 │
│  DO NOT PROCEED until all are read and understood.              │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Wire the existing but unused `RouteStore` (pool system) to components for attribute deduplication and memory optimization.

---

## Design Transition Alignment

**See:** `plan/DESIGN_TRANSITION.md` for overall architecture direction.

### Status in Design Transition

This spec is **superseded** by the Pool + Wire lazy parsing design:

| This Spec Proposes | Pool+Wire Design Instead |
|-------------------|-------------------------|
| Factory methods (`NewASPath()`, etc.) | Store wire bytes via `pool.Intern()` |
| Parsed attribute interning | Wire-canonical storage (no parsing on receive) |
| RouteStore with AttributeStore | Pool with Handle-based deduplication |

### Why This Approach is Outdated

1. **Factory methods assume parsing:** `NewASPath(segments)` creates parsed `*ASPath`
2. **Pool+Wire avoids parsing:** Routes store wire bytes, parse on demand
3. **Deduplication at wrong layer:** This interns parsed objects; Pool interns bytes

### What to Do Instead

Skip this spec. Implement `spec-pool-handle-migration.md` directly:

1. `pool.Intern(attrBytes)` → Handle (deduplicates wire bytes)
2. Route stores `attrHandle` not `[]Attribute`
3. No factory methods needed - wire bytes are canonical

### What's Salvageable

The observation that RouteStore exists but is unused is valid. However:
- Don't wire parsed attribute factories
- Wire pool.Handle storage instead

---

## Problem

RouteStore exists but is **completely unused**:
- Created in `Reactor.New()` at line 883
- Only accessed for `Stop()` cleanup at line 1128
- Never passed to Peer, Session, API, or CommitManager
- **60+ direct attribute constructions** bypass deduplication

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `plan/CLAUDE_CONTINUATION.md` for current state
- **FIRST:** Read `.claude/ESSENTIAL_PROTOCOLS.md` for session rules
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- Tests passing is NOT permission to commit - wait for user

### From ESSENTIAL_PROTOCOLS.md
- TDD is BLOCKING: Tests must exist and fail before implementation
- Fix All Issues You Notice (No Broken Windows)
- Run `make test` after each phase

## Codebase Context

### Files to Modify

| File | Changes |
|------|---------|
| `pkg/reactor/reactor.go` | Pass `ribStore` to Peer creation |
| `pkg/reactor/peer.go` | Store reference, use for attribute interning |
| `pkg/api/commit.go` | Access store via CommandContext |
| `pkg/rib/commit.go` | Accept store parameter |
| `pkg/rib/update.go` | Accept store parameter |
| `pkg/rib/store.go` | Add factory methods for attributes |

### Direct Attribute Constructions to Replace

| File | Count | Examples |
|------|-------|----------|
| `peer.go` | 20 | ASPath, NextHop, Aggregator |
| `reactor.go` | 4 | ASPath, NextHop, MPUnreachNLRI |
| `commit.go` / rib files | 12 | Various attributes |

## Implementation Steps

### Phase 1: Wire RouteStore to Peer
1. Add `routeStore *rib.RouteStore` field to `Peer` struct
2. Pass `r.ribStore` in `NewPeer()` call (reactor.go)
3. Write test verifying Peer has store reference
4. Run `make test`

### Phase 2: Create Attribute Factory Methods
1. Write tests for factory methods (`TestRouteStore_NewASPath`, etc.)
2. Add factory methods to RouteStore:
   - `NewASPath(segments)`
   - `NewNextHop(addr)`
   - `NewAggregator(as, addr)`
3. Run tests - verify interning works

### Phase 3: Replace Direct Constructions (Batch 1 - peer.go)
1. Replace 20 direct `&attribute.*{}` with `p.routeStore.NewXxx()`
2. Run `make test` after each file

### Phase 4: Replace Direct Constructions (Batch 2 - other files)
1. Replace in `reactor.go` (4 violations)
2. Replace in `commit.go` / rib files (12 violations)
3. Run `make test` after each batch

### Phase 5: Add Release Calls
1. Identify where routes are withdrawn
2. Add `routeStore.ReleaseRoute(route)` at:
   - `OutgoingRIB.Withdraw()`
   - CommitManager transaction cleanup
   - Peer shutdown
3. Write tests for reference counting

### Phase 6: Update Tests
1. Create `rib.NewTestRouteStore()` for test use
2. Update test files using direct attribute construction
3. Run full `make test && make lint`

## Verification Checklist

- [ ] TDD followed for factory methods
- [ ] `make test` passes after each phase
- [ ] No direct `&attribute.*{}` in production code
- [ ] RouteStore metrics show dedup hits > 0
- [ ] Memory usage verified with duplicate routes
- [ ] `make test` passes
- [ ] `make lint` passes

## Risks

| Risk | Mitigation |
|------|------------|
| Reference count bugs | Add debug assertions, comprehensive tests |
| Performance regression | Benchmark before/after |
| Breaking tests | Run `make test` after each phase |

## Success Criteria

1. `make test && make lint` passes
2. No direct `&attribute.*{}` in production code
3. RouteStore metrics show dedup hits > 0
4. Memory usage lower with duplicate routes

---

## Relationship to spec-unified-handle-nlri.md

**This spec should be completed FIRST.**

unified-handle-nlri builds on this work:
- Uses RouteStore wiring established here
- Extends RouteStore with Ctx for NLRI pools
- Factory methods survive - they abstract storage decisions

**Design for forward compatibility:**
- Factory methods hide storage implementation
- RouteStore can be extended with Ctx later
- Attribute storage (AttributeStore) remains separate from NLRI storage (pool.Pool)
