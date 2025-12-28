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
| `api-commit-batching.md` | Superseded by unified-commit-system |
| `config-routes-eor.md` | Superseded by unified-commit-system |

### 📋 Implementation Specs (Active)

| Spec | Description | Priority |
|------|-------------|----------|
| `spec-extended-nexthop.md` | RFC 8950 IPv4 NLRI with IPv6 next-hop | High |
| `spec-peer-encoding-extraction.md` | Extract UPDATE builders from peer.go | High |
| `spec-pool-integration.md` | Wire RouteStore to components | Medium |
| `spec-update-builder.md` | Fluent UPDATE builder pattern | Medium |
| `spec-route-families.md` | FlowSpec/VPLS/EVPN keyword validation | Medium |
| `spec-api-test-features.md` | Remaining API test features | Medium |
| `spec-rfc7606-validation-cache.md` | Validation result caching (optional) | Low |

### 📋 Feature Plans (Not Yet Specs)

| Plan | Description | Priority |
|------|-------------|----------|
| `exabgp-migration-tool.md` | CLI tool to convert ExaBGP configs | Medium |
| `plugin-system-mvp.md` | Plugin system MVP specification | Low |
| `plugin-system.md` | Plugin system full design | Deferred |
| `claude-folder-improvements.md` | Claude protocol improvements | Meta |

### 📖 Reference

| Plan | Description |
|------|-------------|
| `ARCHITECTURE.md` | System architecture |
| `exabgp-alignment.md` | Review decisions (18 ALIGN, 7 KEEP, 2 SKIP, 9 DONE) |
| `CLAUDE_CONTINUATION.md` | Session state and priorities |

---

## Spec Format

All implementation specs follow the format defined in `.claude/commands/prep.md`:

```markdown
# Spec: <task-name>

## Task
<description>

## Embedded Protocol Requirements
### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `plan/CLAUDE_CONTINUATION.md` for current state
- **FIRST:** Read `.claude/ESSENTIAL_PROTOCOLS.md` for session rules
- Tests MUST exist and FAIL before implementation code exists
- ...

## Codebase Context
<files to modify>

## Implementation Steps
<numbered TDD steps>

## Verification Checklist
<checkboxes>
```

---

## Structure

```
plan/
├── done/              # Completed plans
│   └── <name>.md
├── spec-<name>.md     # Active implementation specs
├── <name>.md          # Feature plans (not yet specs)
├── ARCHITECTURE.md    # Reference
├── CLAUDE_CONTINUATION.md  # Session state
└── README.md          # This index
```

---

**Last Updated:** 2025-12-28
