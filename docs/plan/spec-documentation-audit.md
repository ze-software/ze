# Spec: Documentation Accuracy Audit

## Post-Compaction Recovery / New Session Resume

**STOP. Read this section completely before doing anything.**

### Resume Instructions

1. **Read this spec file** - You are doing a documentation accuracy audit
2. **Check "Completed Files" table** - These are DONE, do not redo them
3. **Find first unchecked `[ ]` in "Remaining Files"** - Start there
4. **For each file:**
   - Read the doc file in `docs/architecture/` or `docs/`
   - Identify claims about code (types, functions, file paths)
   - Use Grep/Glob/Read to verify claims against actual `internal/` code
   - Fix any discrepancies (wrong names, outdated refs, TODOs that are done)
   - Mark `[x]` in Remaining and add row to Completed table
5. **Update this spec** after each file completion

### What NOT to do
- Do NOT write new code (this is doc-only)
- Do NOT create tests (audit task)
- Do NOT redo completed files
- Do NOT use TDD (not applicable)

### Common fixes needed
- `docs/plan/spec-*.md` → `docs/plan/done/NNN-*.md` (completed specs)
- Field names in doc examples → match actual struct fields
- "TODO" or "planned" → "implemented" if code exists

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
| 7 | `architecture/wire/attributes.md` | ✅ Fixed | Attribute interface (not embedding WireWriter); FlagExtLength; removed Transcoder; ParseAttributes pseudocode |
| 8 | `architecture/wire/nlri.md` | ✅ Fixed | WriteTo() signature (no ctx param); file refs lowercase (nlri-evpn.md etc.) |
| 9 | `architecture/wire/capabilities.md` | ✅ Fixed | Code type (not CapabilityCode); CodeX constants; Negotiated struct fields |
| 10 | `architecture/wire/buffer-writer.md` | ✅ Fixed | CheckedBufWriter (not CheckedWireWriter); Attribute doesn't embed WireWriter; context-free BufWriter signatures |
| 11 | `architecture/wire/update-packing.md` | ✅ Accurate | Design doc, no code references |
| 12 | `architecture/wire/mp-nlri-ordering.md` | ✅ Accurate | Design doc, RFC references only |
| 13 | `architecture/wire/qualifiers.md` | ✅ Fixed | RouteDistinguisher struct (Type+Value not packed); removed non-existent singletons |
| 14 | `architecture/wire/nlri-flowspec.md` | ✅ Accurate | RFC wire format reference |
| 15 | `architecture/wire/nlri-evpn.md` | ✅ Accurate | RFC wire format reference |
| 16 | `architecture/wire/nlri-bgpls.md` | ✅ Accurate | RFC wire format reference |
| 17 | `architecture/api/architecture.md` | ✅ Fixed | Code paths (internal/plugin/...) |
| 18 | `architecture/api/capability-contract.md` | ✅ Fixed | Spec ref (done/172-); code paths |
| 19 | `architecture/api/commands.md` | ✅ Accurate | Design doc |
| 20 | `architecture/api/ipc_protocol.md` | ✅ Accurate | Protocol spec |
| 21 | `architecture/api/json-format.md` | ✅ Accurate | Format spec |
| 22 | `architecture/api/process-protocol.md` | ✅ Accurate | Protocol spec |
| 23 | `architecture/api/update-syntax.md` | ✅ Fixed | Spec ref (done/089-new-syntax.md) |
| 24-29 | Config docs (6 files) | ✅ Accurate | Design/reference docs |
| 30-31 | Behavior docs (2 files) | ✅ Accurate | FSM/signal references |
| 32-34 | Edge cases (3 files) | ✅ Accurate | RFC protocol references |
| 35-43 | Other architecture (9 files) | ✅ Accurate | Design docs |
| 44-45 | Testing/Debug (2 files) | ✅ Accurate | Test/debug guides |
| 46 | Plugin design (1 file) | ✅ Accurate | Design doc |
| 47-51 | Top-level docs (5 files) | ✅ Accurate | Reference docs |
| 52-55 | ExaBGP docs (4 files) | ✅ Accurate | Comparison/migration |
| 56 | RFC guide (1 file) | ✅ Fixed | Code paths corrected |

### Remaining Files

**All files checked.**

#### Config (6 files)
- [x] `architecture/config/syntax.md` - Config reference, no code to verify
- [x] `architecture/config/environment.md` - Environment var reference
- [x] `architecture/config/environment-block.md` - Config syntax reference
- [x] `architecture/config/tokenizer.md` - Parsing reference
- [x] `architecture/config/yang-config-design.md` - Design doc (future)
- [x] `architecture/config/vyos-research.md` - Research doc

#### Behavior (2 files)
- [x] `architecture/behavior/fsm.md` - FSM protocol reference
- [x] `architecture/behavior/signals.md` - Signal handling reference

#### Edge Cases (3 files)
- [x] `architecture/edge-cases/addpath.md` - RFC 7911 reference
- [x] `architecture/edge-cases/as4.md` - RFC 6793 reference
- [x] `architecture/edge-cases/extended-message.md` - RFC 8654 reference

#### Other Architecture (9 files)
- [x] `architecture/route-types.md` - Code paths verified
- [x] `architecture/rib-transition.md` - Design doc
- [x] `architecture/overview.md` - Architecture overview
- [x] `architecture/system-architecture.md` - System overview
- [x] `architecture/hub-architecture.md` - Hub design doc
- [x] `architecture/hub-api-commands.md` - Hub API design
- [x] `architecture/message-buffer-design.md` - Design doc
- [x] `architecture/pool-architecture-review.md` - Pool review doc
- [x] `architecture/rfc-may-decisions.md` - RFC decisions doc

#### Testing/Debug (2 files)
- [x] `architecture/testing/ci-format.md` - Test format reference
- [x] `architecture/debugging/plugin-testing.md` - Debug guide

#### Plugin (1 file)
- [x] `architecture/plugin/rib-storage-design.md` - Design doc

#### Top-Level docs/ (5 files)
- [x] `functional-tests.md` - Test reference
- [x] `test-inventory.md` - Test inventory
- [x] `deprecated-options.md` - Deprecation list
- [x] `config-migration.md` - Migration guide
- [x] `debugging-tools.md` - Debug tools guide

#### ExaBGP (4 files)
- [x] `exabgp/exabgp-code-map.md` - ExaBGP mapping
- [x] `exabgp/exabgp-comparison-report.md` - Comparison doc
- [x] `exabgp/exabgp-differences.md` - Differences doc
- [x] `exabgp/exabgp-migration.md` - Migration guide

#### Contributing (1 file)
- [x] `contributing/rfc-implementation-guide.md` - Fixed code paths (internal/plugin/, internal/plugin/bgp/context/)

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
- [x] All files in "Remaining" checked off
- [ ] Spec moved to `docs/plan/done/`
