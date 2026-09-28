# Spec: ospf-nssa-translator-reachability

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Commit `b12697744d` fixed the RFC 3101 Section 3.2 translator yield under owner
decision D-12: a translator yields only to a functionally equivalent Type-5 from
a higher-router-ID translator. The reachability clause was left out and has a
journal row (`plan/journal/key-omits-a-fact-the-builder-uses.md`, 2026-09-27,
"partly fixed"). The translator set is read from the NSSA LSDB, not from
routers reachable over both area 0 and the NSSA, so a translator can yield to
one that cannot actually translate, and the external route goes missing.

## Defects

| ID | RFC section | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record |
|----|-------------|----------------|----------|--------------------|------------------------|
| D1 | RFC 3101 Section 3.2 (RFC3101-3.2-2) | "the calculating router has the highest router ID amongst NSSA translators that have originated a functionally equivalent Type-5 LSA (i.e. same destination, cost and non-zero forwarding address) and that are reachable over area 0 and the NSSA, then a Type-5 LSA should be generated" | `internal/plugins/ospf/lsdb/nssa.go` `HigherRIDTranslatorExternals`, `equivalentType5`/`equivalentType5V6` | The candidate translators are every B-bit router in the NSSA with an equivalent Type-5; none is checked for reachability over area 0 and the NSSA | weak; journal row partly fixed |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | a higher-RID translator unreachable over area 0 no longer suppresses Ze's Type-5 |
| Both polarities | a reachable higher-RID equivalent translator still suppresses |
| Discrimination | `./le rfc discriminate-record` for the tagged units |
| Interop | an NSSA scenario against FRR ospfd or BIRD with two translators, one partitioned from area 0 |

## Owner decisions

| Row | Question |
|-----|----------|
| - | None open: owner decision D-12 already rules the reading, and it stands |

## Required Reading

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3101.md`, `rfc/full/rfc3101.txt` Sections 3.2 and 3.3

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ospf/lsdb/nssa.go` - translator yield

**Behavior to change:** add the reachability condition.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- SPF completion in area 0 and in the NSSA on an NSSA border router

### Transformation Path
1. SPF results per area
2. translator yield decision
3. Type-5 origination or flush

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| per-area SPF ↔ LSDB origination | routing table lookup | No |

### Integration Points
- the per-area SPF routing tables

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| NSSA with two translators | → | translator yield | [to fill in design] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | higher-RID equivalent translator unreachable over area 0 | Ze originates the Type-5 |
| AC-2 | the same translator reachable over both | Ze yields |
| AC-3 | the `weak` verdict of RFC3101-3.2-2, after this spec's producer fix | the verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it with `./le rfc audit-stamp ... mode rejudge`. Moved here from "Blocked by" in `plan/pre-release/spec-rfc-verdict-fix-ospf.md` (parent P-3, 2026-09-28) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| reachability both polarities | `internal/plugins/ospf/lsdb/` | D1 | [to fill in design] |

## Files to Modify

- `internal/plugins/ospf/lsdb/nssa.go` and its tests; the journal row's Fix cell at closure

## Implementation Steps

1. Failing test; 2. fix; 3. discrimination and verdict.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)

### Verification
- [ ] `./le verify worktree`
