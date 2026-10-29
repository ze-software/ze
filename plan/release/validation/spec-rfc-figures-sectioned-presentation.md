# Spec: rfc-figures-sectioned-presentation

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | Accurate support claims: the published RFC coverage figure divides by rows Ze's code does not owe, so it misstates what Ze has done |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The RFC compliance pages publish one mandatory figure per RFC and per profile:
rows proven over every MUST row, with a "ceiling" beside it. Rows the operating
system performs, rows of an optional feature Ze declined, not-applicable rows
and Linux-limit rows all sit in the denominator. For the fttx-edge profile the
page reads 338 of 747 (45.2%) while Ze's own code owes 529 of those rows. The
figure therefore misreports Ze's state, and the ceiling hides the reason.

Replace the single figure and its ceiling with sections, following the owner
rules in `ai/META-DECISIONS.md` ("Goal and scope": an OS-met requirement
without a test is a testing gap; an OS-blocked requirement is reported in its
own section and never counts against Ze's mandatory coverage or holds a
release).

| Section | Counts | Status values |
|---------|--------|---------------|
| Ze MUST coverage | MUST rows Ze's code owes: total minus OS met, OS blocked, declined, not applicable | proven, correctness gap |
| OS met | MUST rows a lower layer performs, per OS and kernel version | verified (a test checks the OS behaviour on the state Ze installs), testing gap |
| OS blocked | MUST rows Linux cannot provide the primitive for, with a verified limit entry | blocked; never counted against Ze, never holds a release |
| Declined feature | MUST rows conditional on an optional feature Ze does not offer | declined, citing the RFC sentence that makes it optional |
| Not applicable | MUST rows that do not bind Ze's role | listed with evidence, not counted |
| Optional features | optional features offered over optional features defined | ratio; owned by `plan/post-release/engineering/spec-rfc-optional-feature-ratio.md`, not this spec |
| Optional requirements | SHOULD / MAY rows | proven, not yet, not ever, not decided (below) |

Decisions agreed with Thomas, 2026-10-29:

| Decision | Ruling |
|----------|--------|
| Leaving Ze's section raises the headline | Yes, but only with checked evidence: a row moves to OS met, OS blocked, declined or not applicable only when it cites the RFC sentence (section, read in `rfc/full/`) and the kernel behaviour or role condition that settles it. A check that refuses a moved row without that evidence lands before the new headline is published. This supersedes the 2026-10-25 ruling that annotating a row may not raise its figure |
| Gauges | Two bars per RFC set and per RFC row, same colours and same left-to-right order: a MUST bar and an overview bar (below) |
| MUST bar | The promise Ze must keep to be RFC compliant. Covers only the MUST rows the stack owes (Ze plus OS met); rows not owed (not applicable, declined, OS blocked) are left out of it |
| Overview bar | Every row of the RFC, MUST and SHOULD / MAY, so a reader sees how much of the optional part Ze supports as well. Counts on each segment, no single percentage |
| SHOULD / MAY states | Every row starts `not decided`. `not yet`: we intend to support it, not done; set during RFC work. `not ever`: we will not support it; only Thomas sets it (an agent may propose). `proven`: a tagged test with a verifying discrimination record |

Presentation, as agreed against the fttx-edge mockup (tables per section, the
gauges below):

| Surface | What it shows |
|---------|---------------|
| Profile summary | One line per section, plus an "All MUST rows" line equal to the sum of the five MUST sections |
| RFC list (`?profile=<name>&rfc=<stems>#rfc-list`) | One row per RFC: MUST total, then a column pair per section; a Total row at the bottom whose five MUST section columns add up to the MUST total, with a check line |
| RFC detail page | Each requirement with level, section, status and evidence, ending in a total that matches the RFC's row in the list |

Gauges: the current separate mandatory and optional bars are replaced by the
MUST bar and the overview bar above. Segments, one colour each, each also
carrying its name and count in text so the split stays readable without colour:

| Bar | Left to right |
|-----|---------------|
| MUST | Ze proven (green), OS verified (light green), OS testing gap (amber), Ze correctness gap (red) |
| Overview | Ze MUST proven (green), OS verified (light green), SHOULD proven (teal), not owed (grey), SHOULD not ever (light grey), SHOULD not yet, SHOULD not decided, OS testing gap (amber), Ze correctness gap (red) |

Mockup on fttx-edge counts from the live tree on 2026-10-29 (no OS-met row is
verified yet, and every unproven SHOULD row is still `not decided`):

```
MUST      |███████████████████████████████|▚▚▚▚▚▚▚▚▚▚▚|XXXXXXXXXXXXXXXXXX|  340/648 kept (52%)
           Ze proven 340                   OS untested 119  Ze gap 189

Overview  |███████████████████|▒|·····|░░░░░░░░░░░░░░░░░░|▚▚▚▚▚▚▚|XXXXXXXXXX|  all 1,082 rows
           MUST proven 340     S 7 not owed 99  SHOULD not decided 328  OS untested 119  Ze gap 189
```

| Where | Gauges |
|-------|--------|
| Top of the filtered RFC list (`quality/rfc-compliance/?profile=...`), before the tables | Both bars for the whole set of RFCs shown, from the same sums as the Total row |
| Each RFC row of the RFC table | Both bars per row, same segments, colours and order |

Segment widths are row counts over the bar's total, so the top bars and the
Total row show the same numbers.

Counts are summed across RFCs and divided once; percentages are never averaged.
The ceiling presentation and the old mandatory and optional bars are removed,
not kept beside the sections (`ai/rules/no-layering.md`). Published figures are corrected on the surfaces
that carried them, saying what the old figure counted.

Related: `plan/release/validation/spec-excluded-requirement-owes-an-observable-test.md`
adds the gate that enforces the OS-met test; this spec shows its absence as a
testing gap. Neither waits on the other.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/rfc-conformance-gates.md` - "Coverage figures", "The linux-limit annotation"
  → Constraint: filled at RESEARCH
- [ ] `docs/contributing/gh-pages.md` - quality pages, profile pages and the filtered RFC list
  → Constraint: filled at RESEARCH

**Key insights:** filled at RESEARCH

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/rfc/profilecoverage.go` - the per-RFC and rollup figures
  → Constraint: `figuresByStem` keeps `{not-applicable}`, `{lower-layer}`, `{feature-declined}` and `{linux-limit}` rows in `Total` and never proves them "whatever records their tags carry"; an OS-met "verified" status needs a new proof predicate for those rows, separate from Ze's `Proven`
  → Constraint: `{gap}` and `{partial}` rows are never proven even with a verifying record; this stays for the Ze section
  → Decision: the code comment cites an owner ruling of 2026-10-25 that "annotating a row may not raise its figure"; the new owner rules supersede it for the headline, so the comment and `docs/contributing/rfc-conformance-gates.md` "Coverage figures" are rewritten with the code
  → Constraint: `rollupOf` sums only walked members and reports `NotWalked`; the sectioned rollup keeps that rule
  → Constraint: SHOULD / MAY rows (`!Gated()`) go through the same annotation partition into `Optional`; no field records intent, so `not yet`, `not ever` and `not decided` need a new per-row declaration in the summary, with `not decided` the default when absent

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters] filled at RESEARCH

### Transformation Path
- filled at RESEARCH

### Boundaries Crossed
- filled at RESEARCH

### Integration Points
- filled at RESEARCH

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| filled at DESIGN | → | | |

## Acceptance Criteria

| AC ID | Input / Trigger | Expected Behavior |
|-------|-----------------|-------------------|
| filled at DESIGN | | |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| filled at DESIGN | | | |

## Files to Modify
- filled at DESIGN

### Integration Checklist
- filled at RESEARCH

### Documentation Update Checklist (BLOCKING)
- filled at RESEARCH

## Implementation Steps
- filled at DESIGN

## Owned journal rows

| Row | Journal file |
|-----|--------------|
| - | - |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le verify changed` evidence for the change set
