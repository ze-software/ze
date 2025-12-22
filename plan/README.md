# ZeBGP Implementation Plans

**All active work is tracked here.**

---

## Architecture

**Read first:** `ARCHITECTURE.md` - Comprehensive system design

---

## Plan Status

### ✅ Complete (in `done/`)

| Plan | Description |
|------|-------------|
| `rfc7606-extension.md` | RFC 7606 full compliance (3 phases) |
| `two-level-grouping.md` | Two-level route grouping for UPDATE generation |
| `unified-commit-system.md` | Full CommitService with wire format (Phase 1-3) |
| `neighbor-to-peer-rename.md` | All 6 phases done, v2 syntax removed |
| `config-migration-system.md` | v2→v3 migration, CLI commands, docs |
| `knowledge-acquisition.md` | ExaBGP study notes |
| `pool-completion.md` | Pool implementation complete |
| `family-negotiation.md` | Family negotiation implementation |
| `align-implementation.md` | ExaBGP alignment implementation |
| `rfc-annotation.md` | RFC annotation work |
| `rib-config-design.md` | RIB configuration design |
| `edit-command.md` | Edit command implementation |
| `fsm-active-design.md` | FSM critique reviewed - no action needed |

### ⏭️ Superseded

| Plan | Superseded By |
|------|---------------|
| `api-commit-batching.md` | `done/unified-commit-system.md` |
| `config-routes-eor.md` | `done/unified-commit-system.md` |

### 📋 Planned / In Progress

| Plan | Description | Priority |
|------|-------------|----------|
| `peer-encoding-extraction.md` | Extract UPDATE builders from peer.go | High |
| `pool-integration.md` | Wire RouteStore to components | Medium |
| `exabgp-migration-tool.md` | CLI tool to convert ExaBGP configs | Medium |
| `plugin-system-mvp.md` | Plugin system MVP specification | Low |
| `plugin-system.md` | Plugin system full design | Deferred |

### 📖 Reference

| Plan | Description |
|------|-------------|
| `ARCHITECTURE.md` | System architecture |
| `exabgp-alignment.md` | Review decisions (18 ALIGN, 7 KEEP, 2 SKIP, 9 DONE) |
| `CLAUDE_CONTINUATION.md` | Session state and priorities |

---

## Structure

```
plan/
├── done/           # Completed plans
│   └── <name>.md
├── <name>.md       # Active/planned work
├── ARCHITECTURE.md # Reference
└── README.md       # This index
```

---

**Last Updated:** 2025-12-22
