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

## Current Status

**Project is functional!** Core BGP implementation complete.

### Completed Phases

| Phase | Description | Status |
|-------|-------------|--------|
| 0 | Project Foundation | ✅ Complete |
| 1 | Wire Format Foundation | ✅ Complete |
| 2 | Message Types | ✅ Complete |
| 3 | Capabilities | ✅ Complete |
| 4 | Path Attributes | ✅ Complete |
| 5 | NLRI Types | ✅ Complete |
| 6 | RIB | ✅ Complete |
| 7 | FSM | ✅ Complete |
| 8 | Reactor | ✅ Complete |
| 9 | Configuration | ✅ Complete |
| 10 | CLI | ✅ Basic (`zebgp validate`, `zebgp run`) |
| 11 | API | ✅ Integrated |
| 12 | Testing Infrastructure | 🔄 Ongoing |
| 13 | Integration & Polish | 🔄 Ongoing |

### Test Coverage

All packages have tests and pass with `-race`:
- `internal/pool` - 50+ tests
- `pkg/wire` - Buffer operations
- `pkg/bgp/message` - Header, OPEN, UPDATE, etc.
- `pkg/bgp/attribute` - All attribute types
- `pkg/bgp/capability` - All capability types
- `pkg/bgp/nlri` - INET, IPVPN, etc.
- `pkg/bgp/fsm` - State machine
- `pkg/rib` - Route storage
- `pkg/config` - Parser, serializer
- `pkg/reactor` - Core loop

### Current Focus

**Phase 12-13:** Testing infrastructure and polish
- Integration testing with real BGP peers
- ExaBGP interop testing
- Performance benchmarks
- Documentation

---

## Plan Files

| File | Status | Description |
|------|--------|-------------|
| `ARCHITECTURE.md` | 📋 Reference | System architecture |
| `done-knowledge-acquisition.md` | ✅ Done | ExaBGP study notes |
| `done-pool-completion.md` | ✅ Done | Pool implementation complete |

---

## Naming Convention

- `plan-<name>.md` - Planned work
- `wip-<name>.md` - Active work in progress
- `done-<name>.md` - Completed work

---

**Last Updated:** 2025-12-20
