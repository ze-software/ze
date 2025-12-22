# ZeBGP Implementation Plans

**All active work is tracked here.**

---

## Architecture

**Read first:** `ARCHITECTURE.md` - Comprehensive system design

---

## Plan Status

### ✅ Complete

| Plan | Description |
|------|-------------|
| `two-level-grouping.md` | Two-level route grouping for UPDATE generation |
| `unified-commit-system.md` | Full CommitService with wire format |
| `neighbor-to-peer-rename.md` | All 6 phases done, v2 syntax removed |
| `config-migration-system.md` | v2→v3 migration, CLI commands, docs |
| `done-knowledge-acquisition.md` | ExaBGP study notes |
| `done-pool-completion.md` | Pool implementation complete |
| `done-family-negotiation.md` | Family negotiation implementation |
| `done-align-implementation.md` | ExaBGP alignment implementation |
| `done-rfc-annotation.md` | RFC annotation work |
| `done-rib-config-design.md` | RIB configuration design |
| `done-edit-command.md` | Edit command implementation |

### ⏭️ Superseded

| Plan | Superseded By |
|------|---------------|
| `api-commit-batching.md` | `unified-commit-system.md` |
| `config-routes-eor.md` | `unified-commit-system.md` |

### 📋 Planned / In Progress

| Plan | Description |
|------|-------------|
| `exabgp-migration-tool.md` | CLI tool to convert ExaBGP configs |
| `plugin-system.md` | Plugin system design (CoreBGP-inspired) |

### 📖 Reference

| Plan | Description |
|------|-------------|
| `ARCHITECTURE.md` | System architecture |
| `exabgp-alignment.md` | Review decisions (26 ALIGN, 8 KEEP, 2 SKIP) |
| `CLAUDE_CONTINUATION.md` | Session state and priorities |

---

## Naming Convention

- `done-<name>.md` - Completed work
- Others - Active plans or reference docs

---

**Last Updated:** 2025-12-21
