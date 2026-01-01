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
| `spec-asn4-packcontext.md` | ASN4 in PackContext (RFC 6793) |
| `spec-negotiated-packing.md` | Unified Pack(ctx) pattern (RFC 7911) |
| `spec-addpath-encoding.md` | ADD-PATH encoding support |
| `spec-extcomm-hex.md` | Extended-community hex format (RFC 4360) |
| `spec-extended-nexthop.md` | RFC 8950 design (implemented) |
| `spec-collision-detection.md` | BGP collision detection (RFC 4271 §6.8) |
| `spec-process-backpressure.md` | Process backpressure and respawn |
| `spec-self-check-rewrite.md` | ExaBGP-style functional tests |
| `unified-commit-system.md` | Full CommitService with wire format |
| `two-level-grouping.md` | Two-level route grouping for UPDATEs |
| `neighbor-to-peer-rename.md` | All 6 phases done, v2 syntax removed |
| `config-migration-system.md` | v2→v3 migration, CLI commands |
| `rfc7606-extension.md` | RFC 7606 full compliance |
| `spec-listener-per-local-address.md` | Multi-listener from peer LocalAddress |
| `spec-environment-config-block.md` | Environment block in config (ZeBGP-specific) |
| `spec-mup-api-support.md` | MUP SAFI support in API parser |
| `spec-encoding-context-design.md` | EncodingContext design |
| `spec-encoding-context-impl.md` | EncodingContext implementation |
| `spec-api-test-features.md` | API test features (14/14 pass) |
| `spec-route-families.md` | FlowSpec/VPLS/EVPN keyword validation |
| `spec-update-builder.md` | Fluent UPDATE builder pattern |
| `spec-format-based-migration.md` | Config migration with transformations |

### 📋 Implementation Specs (Active)

| Spec | Description | Priority |
|------|-------------|----------|
| `spec-peer-encoding-extraction.md` | Extract UPDATE builders from peer.go | High |
| `spec-pool-integration.md` | Wire RouteStore to components | Medium |
| `spec-rfc7606-validation-cache.md` | Validation result caching (optional) | Low |

### 📋 Feature Plans (Not Yet Specs)

| Plan | Description | Priority |
|------|-------------|----------|
| `exabgp-migration-tool.md` | CLI tool to convert ExaBGP configs | Medium |
| `plugin-system-mvp.md` | Plugin system MVP specification | Low |
| `plugin-system.md` | Plugin system full design | Deferred |

### 📖 Reference

| Plan | Description |
|------|-------------|
| `ARCHITECTURE.md` | System architecture |
| `exabgp-alignment.md` | Review decisions (18 ALIGN, 7 KEEP, 2 SKIP, 9 DONE) |
| `CLAUDE_CONTINUATION.md` | Session state and priorities |
| `deterministic-simulation-analysis.md` | Simulation testing research |

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

**Last Updated:** 2026-01-01
