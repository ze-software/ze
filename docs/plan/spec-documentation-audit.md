# Spec: Documentation Accuracy Audit

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (progress tracked in "Completed Files" table below)
2. Continue from first unchecked item in "Remaining Files"

## Task

Review every documentation file in `docs/` (excluding `docs/plan/`) and verify accuracy against actual code. Fix discrepancies.

## Required Reading

### Architecture Docs
- [ ] Each doc file being audited - read and verify against code

### RFC Summaries
- N/A - audit task, not protocol implementation

**Key insights:**
- Common issues: outdated spec refs, wrong field names, TODO items that are done

## Current Behavior

**Source files read:**
- [ ] `docs/architecture/*.md` - documentation files being audited
- [ ] `internal/**/*.go` - source code to verify against

**Behavior to preserve:**
- Accurate documentation that matches code

**Behavior to change:**
- Fix inaccuracies found during audit

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A | N/A | Audit task - no code changes | N/A |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | N/A | Audit task - doc fixes only | N/A |

## Files to Modify

- `docs/architecture/**/*.md` - documentation files with inaccuracies

## Files to Create

None - audit task.

## Implementation Steps

1. Read doc file
2. Identify code references (types, functions, paths)
3. Verify against actual code
4. Fix discrepancies
5. Update "Completed Files" table below

## Progress Tracking

### Completed Files

| # | File | Status | Changes Made |
|---|------|--------|--------------|
| 1 | `architecture/core-design.md` | ✅ Fixed | RouteEntry→pool.Handle; added spec-message-update-removal.md link |
| 2 | `architecture/buffer-architecture.md` | ✅ Fixed | Route field names (wireBytes, nlriWireBytes); spec refs to done/; Phase 6 |
| 3 | `architecture/encoding-context.md` | ✅ Fixed | Added EnhancedRouteRefresh; 7 spec refs to done/ |
| 4 | `architecture/pool-architecture.md` | ✅ Fixed | Removed non-existent blob pools; added AtomicAggregate/Aggregator |
| 5 | `architecture/update-building.md` | ✅ Fixed | Wire-level split TODO→Implemented; 4 spec refs; lowercase doc refs |
| 6 | `architecture/wire/messages.md` | ✅ Accurate | No changes needed |

### Remaining Files

#### Wire Formats (10 files)
- [ ] `architecture/wire/attributes.md`
- [ ] `architecture/wire/nlri.md`
- [ ] `architecture/wire/capabilities.md`
- [ ] `architecture/wire/buffer-writer.md`
- [ ] `architecture/wire/update-packing.md`
- [ ] `architecture/wire/mp-nlri-ordering.md`
- [ ] `architecture/wire/qualifiers.md`
- [ ] `architecture/wire/nlri-flowspec.md`
- [ ] `architecture/wire/nlri-evpn.md`
- [ ] `architecture/wire/nlri-bgpls.md`

#### API (7 files)
- [ ] `architecture/api/architecture.md`
- [ ] `architecture/api/capability-contract.md`
- [ ] `architecture/api/commands.md`
- [ ] `architecture/api/ipc_protocol.md`
- [ ] `architecture/api/json-format.md`
- [ ] `architecture/api/process-protocol.md`
- [ ] `architecture/api/update-syntax.md`

#### Config (6 files)
- [ ] `architecture/config/syntax.md`
- [ ] `architecture/config/environment.md`
- [ ] `architecture/config/environment-block.md`
- [ ] `architecture/config/tokenizer.md`
- [ ] `architecture/config/yang-config-design.md`
- [ ] `architecture/config/vyos-research.md`

#### Behavior (2 files)
- [ ] `architecture/behavior/fsm.md`
- [ ] `architecture/behavior/signals.md`

#### Edge Cases (3 files)
- [ ] `architecture/edge-cases/addpath.md`
- [ ] `architecture/edge-cases/as4.md`
- [ ] `architecture/edge-cases/extended-message.md`

#### Other Architecture (9 files)
- [ ] `architecture/route-types.md`
- [ ] `architecture/rib-transition.md`
- [ ] `architecture/overview.md`
- [ ] `architecture/system-architecture.md`
- [ ] `architecture/hub-architecture.md`
- [ ] `architecture/hub-api-commands.md`
- [ ] `architecture/message-buffer-design.md`
- [ ] `architecture/pool-architecture-review.md`
- [ ] `architecture/rfc-may-decisions.md`

#### Testing/Debug (2 files)
- [ ] `architecture/testing/ci-format.md`
- [ ] `architecture/debugging/plugin-testing.md`

#### Plugin (1 file)
- [ ] `architecture/plugin/rib-storage-design.md`

#### Top-Level docs/ (5 files)
- [ ] `functional-tests.md`
- [ ] `test-inventory.md`
- [ ] `deprecated-options.md`
- [ ] `config-migration.md`
- [ ] `debugging-tools.md`

#### ExaBGP (4 files)
- [ ] `exabgp/exabgp-code-map.md`
- [ ] `exabgp/exabgp-comparison-report.md`
- [ ] `exabgp/exabgp-differences.md`
- [ ] `exabgp/exabgp-migration.md`

#### Contributing (1 file)
- [ ] `contributing/rfc-implementation-guide.md`

## Common Issues Found

| Issue | Fix |
|-------|-----|
| Spec refs to `docs/plan/spec-*.md` | Update to `docs/plan/done/NNN-*.md` |
| Struct field names wrong | Update to match actual code |
| TODO items that are done | Mark as implemented |
| Non-existent types/pools | Remove from docs |
| Uppercase file refs | Use lowercase |

## Checklist

### 🏗️ Design
- [x] No premature abstraction
- [x] No speculative features
- [x] Single responsibility
- [x] Explicit behavior
- [x] Minimal coupling
- [x] Next-developer test

### 🧪 TDD
- [x] Tests written (N/A - audit)
- [x] Tests FAIL (N/A - audit)
- [x] Implementation complete (in progress)
- [x] Tests PASS (N/A - audit)
- [x] Boundary tests (N/A - audit)
- [x] Feature code integrated (N/A - audit)
- [x] Functional tests (N/A - audit)

### Verification
- [ ] `make lint` passes (N/A - doc changes only)
- [ ] `make test` passes (N/A - doc changes only)
- [ ] `make functional` passes (N/A - doc changes only)

### Documentation
- [x] Required docs read (each audited doc)
- [x] RFC summaries read (N/A)
- [x] RFC references added (N/A)
- [x] RFC constraint comments (N/A)

### Completion
- [ ] All files in "Remaining" checked off
- [ ] Spec moved to `docs/plan/done/`
