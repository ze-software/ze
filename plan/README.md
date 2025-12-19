# ZeBGP Implementation Plans

**All active work is tracked here.**

---

## Architecture

**Read first:** `ARCHITECTURE.md` - Comprehensive system design

Key sections:
- System overview and component diagram
- Core design decisions (AS-PATH as NLRI, zero-copy, pools)
- Data flow diagrams
- Memory management and concurrency model
- Interface contracts
- Dependency graph and build order
- ExaBGP compatibility requirements
- Implementation phases

---

## Current Focus

**Pool Implementation Completion** - See `wip-pool-completion.md`

Completing remaining items from pool architecture review before proceeding to wire format.

### Remaining Issues

| # | Issue | Priority | Effort | Status |
|---|-------|----------|--------|--------|
| 4 | Debug handle validation | Medium | 30m | ❌ TODO |
| 5 | Metrics | Medium | 1h | ❌ TODO |
| 6 | Graceful shutdown | Medium | 45m | ❌ TODO |
| 7 | Slice lifetime docs | Low | 10m | ❌ TODO |
| 8 | Activity semantics docs | Low | 5m | ❌ TODO |
| 9 | Cross-pool coupling | Low | - | ⏸️ Defer |
| 10 | Lock contention | Low | - | ⏸️ Monitor |

**Total estimated effort:** 8-10 hours

**Blocks:** All wire format work (Phase 2+)

---

## Plan Files

| File | Status | Description |
|------|--------|-------------|
| `ARCHITECTURE.md` | 📋 Reference | System architecture |
| `plan-knowledge-acquisition.md` | 📋 Pre-work | Study ExaBGP for 100% compatibility |
| `wip-pool-completion.md` | 🔄 Active | Pool issues #4-10 |
| `plan-wire-format.md` | 📋 Next | Phase 2: Wire format foundation |

---

## Phase Overview

From `ZE_IMPLEMENTATION_PLAN.md`:

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Project Foundation | 🔄 In Progress |
| 2 | Wire Format Foundation | 📋 Planned |
| 3 | Message Types | 📋 Planned |
| 4 | Capabilities | 📋 Planned |
| 5 | Path Attributes | 📋 Planned |
| 6 | NLRI Types | 📋 Planned |
| 7 | RIB | 📋 Planned |
| 8 | FSM | 📋 Planned |
| 9 | Reactor | 📋 Planned |
| 10 | Configuration | 📋 Planned |
| 11 | CLI | 📋 Planned |
| 12 | API | 📋 Planned |
| 13 | Testing Infrastructure | 📋 Planned |
| 14 | Integration & Polish | 📋 Planned |

---

## Naming Convention

- `plan-<name>.md` - Planned work
- `wip-<name>.md` - Active work in progress
- `done-<name>.md` - Completed work

---

**Last Updated:** 2025-12-19
