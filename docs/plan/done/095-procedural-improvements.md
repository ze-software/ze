# Spec: Procedural Improvements

## Task

Audit pending specs, create master TODO.md, remove stale CLAUDE_CONTINUATION.md, add Claude Code infrastructure.

## Problem

1. `docs/plan/spec-parser-unification.md` - unclear if implemented
2. No central tracking of spec status, technical debt, housekeeping
3. `docs/plan/CLAUDE_CONTINUATION.md` - 941 lines of stale historical data, never read

## Analysis Performed

### spec-parser-unification.md Audit

Compared spec design vs actual codebase:

| Spec Component | Status |
|----------------|--------|
| `internal/parse/token.go` | ❌ Missing |
| `internal/parse/api_tokenizer.go` | ❌ Missing |
| `internal/parse/config_adapter.go` | ❌ Missing |
| `internal/parse/update.go` | ❌ Missing |
| `Tokenizer` interface | ❌ Never created |
| Shared `ParseUpdate(Tokenizer)` | ❌ Never created |

**Reality:** API uses `[]string` directly (`ParseUpdateText`, `ParseUpdateWire`), config uses separate `*Tokenizer`. Only `internal/parse/community.go` exists as shared code.

**Conclusion:** Spec is design doc only, 0% implemented.

### All Pending Specs Audit

| Spec | Status |
|------|--------|
| spec-parser-unification.md | ❌ NOT IMPLEMENTED |
| spec-async-api-parser.md | ⏸️ PLACEHOLDER |
| spec-writeto-bounds-safety.md | ✅ COMPLETE (not moved) |
| spec-api-rr.md | 🟡 PARTIAL |
| spec-api-sync.md | ✅ COMPLETE (not moved) |
| spec-rfc9234-role.md | ❌ NOT IMPLEMENTED |
| spec-api-plugin-commands.md | ✅ DONE (not moved) |
| spec-api-command-serial.md | ✅ IMPLEMENTED (not moved) |
| spec-adjribout-memory-profiling.md | ❌ Not Started |
| spec-context-full-integration.md | ⏸️ Ready |
| spec-static-route-updatebuilder.md | ⚠️ Partially Obsolete |
| spec-unified-handle-nlri.md | ⏸️ Ready |
| spec-api-capability-contract.md | ❓ Unknown |
| spec-attribute-context-wire-container.md | ❓ Unknown |
| spec-rfc7606-validation-cache.md | ❓ Unknown |

### CLAUDE_CONTINUATION.md Analysis

| Section | Lines | Value |
|---------|-------|-------|
| TDD reminder | 20 | Duplicate of `.claude/rules/tdd.md` |
| Current status | 10 | Stale |
| Resume point | 10 | Git log is fresher |
| IN PROGRESS | 100 | Duplicates specs + git status |
| RECENTLY COMPLETED | 700+ | Already in `docs/plan/done/*.md` |
| TECHNICAL DEBT | 50 | Useful - migrate to TODO.md |
| PLANNED | 50 | Duplicates spec files |

**Conclusion:** Delete. Technical debt items migrated.

## Changes Made

### Created: TODO.md

Master tracking document with:
- Spec status overview table (15 specs)
- Detailed analysis of critical incomplete specs
- Complete-but-not-moved specs list
- Technical debt (3 items migrated from CLAUDE_CONTINUATION.md)
- Code quality issues
- Uncommitted changes snapshot
- Housekeeping task checklist

### Deleted: docs/plan/CLAUDE_CONTINUATION.md

- 941 lines removed
- Technical debt items preserved in TODO.md
- Historical data already exists in `docs/plan/done/*.md` and git

## RFC Summary Infrastructure

RFC summaries live in `.claude/zebgp/rfc/rfcNNNN.md` - 35 summaries exist.

These are created via `/rfc-summarisation rfcNNNN` agent when implementing protocol features.

### Existing Summaries

```
rfc1997.md   rfc3032.md   rfc4271.md   rfc4360.md   rfc4364.md
rfc4659.md   rfc4684.md   rfc4724.md   rfc4760.md   rfc4761.md
rfc5492.md   rfc5549.md   rfc5575.md   rfc5701.md   rfc6793.md
rfc7313.md   rfc7432.md   rfc7606.md   rfc7752.md   rfc7911.md
rfc8092.md   rfc8195.md   rfc8203.md   rfc8277.md   rfc8654.md
rfc8950.md   rfc8955.md   rfc8956.md   rfc9003.md   rfc9072.md
rfc9085.md   rfc9136.md   rfc9234.md   rfc9514.md
```

### Purpose

- Pre-digested RFC content for faster implementation
- Key sections, wire formats, constraints extracted
- Referenced in planning.md workflow

## Claude Code Infrastructure

### Commands Added

| File | Purpose |
|------|---------|
| `.claude/commands/code-review.md` | Code review slash command |
| `.claude/commands/rfc-summarisation.md` | RFC summary generation command |

### Hooks Updated

| File | Change |
|------|--------|
| `.claude/hooks/auto_linter.sh` | Updated linter hook |
| `.claude/hooks/validate-spec.sh` | New spec validation hook |

### Rules Updated

| File | Change |
|------|--------|
| `.claude/rules/planning.md` | Added RFC summary check step, constraint comments requirement |

### Documentation Updated

| File | Change |
|------|--------|
| `.claude/INDEX.md` | Added navigation index |
| `.claude/settings.json` | Updated settings |

### RFC Summaries Added

35 RFC summaries in `.claude/zebgp/rfc/`:

```
rfc1997  rfc2918  rfc3032  rfc4271  rfc4360  rfc4364  rfc4659  rfc4684
rfc4724  rfc4760  rfc4761  rfc5492  rfc5549  rfc5575  rfc5701  rfc6793
rfc7313  rfc7432  rfc7606  rfc7752  rfc7911  rfc8092  rfc8195  rfc8203
rfc8277  rfc8654  rfc8950  rfc8955  rfc8956  rfc9003  rfc9072  rfc9085
rfc9136  rfc9234  rfc9514
```

## Files Changed

### TODO Consolidation
| File | Change |
|------|--------|
| `TODO.md` | Created |
| `docs/plan/CLAUDE_CONTINUATION.md` | Deleted |
| `docs/plan/spec-todo-consolidation.md` | Created |

### Claude Infrastructure
| File | Change |
|------|--------|
| `.claude/INDEX.md` | Modified |
| `.claude/hooks/auto_linter.sh` | Modified |
| `.claude/hooks/validate-spec.sh` | Created |
| `.claude/rules/planning.md` | Modified |
| `.claude/settings.json` | Modified |
| `.claude/commands/code-review.md` | Created |
| `.claude/commands/rfc-summarisation.md` | Created |
| `.claude/zebgp/rfc/*.md` | Created (35 files) |

### Excluded (separate commits)
| File | Reason |
|------|--------|
| `docs/plan/spec-parser-unification.md` | Pre-existing modification |
| `docs/plan/spec-async-api-parser.md` | Separate spec |
| `docs/plan/spec-writeto-bounds-safety.md` | Separate spec |
| `internal/bgp/message/*_test.go` | Unrelated test changes |
| `scripts/*` | Unrelated tooling |
| `package-lock.json` | Unrelated |
| `yolo` | Unrelated |

## Checklist

- [x] Audit spec-parser-unification.md vs codebase
- [x] Audit all pending specs for status
- [x] Create comprehensive TODO.md
- [x] Migrate technical debt items
- [x] Delete CLAUDE_CONTINUATION.md
- [x] Add Claude commands
- [x] Add RFC summaries
- [x] Update planning rules
- [x] Spec moved to `docs/plan/done/095-procedural-improvements.md`
