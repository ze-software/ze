# Spec: rules-situation-index

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-08-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The rule corpus is stored by topic and consumed by situation.**

The learned-corpus drain-over-archive decision carries the measurement.
Every artifact today, the rendered rule, `TRIGGERS.md`, `CORE.md`, the router, is
a rearrangement of a DOCUMENT. An agent mid-task needs the two to five
instructions bearing on what it is about to do. Measured over 1,043 past task
descriptions: mean 2.5 rules surfaced per task out of 27, and
`internal/le/` exists precisely because sessions do not open the
rules whose triggers matched.

The split made every instruction addressable and bound 47 of 1,585 to a check.
This spec attacks the remaining 1,538 from the other end.

**The proposal.** A point gains an `applies` predicate declaring the situation
that makes it relevant (`writes: **/*.go`, `phase: implement`, `verb: git-push`,
`always`). Every existing artifact then becomes a VIEW over situation-scoped
assertions rather than a hand-maintained document: `CORE.md` is the view where
the predicate is `always`, which is already how membership is derived;
`TRIGGERS.md` is the routing view; `ai/rules/<rule>.md` stays the by-topic view
a human reads; and a check is GENERATED for any predicate that is mechanically
checkable.

That last one is the reason to do it. Today a check is written and then bound to
an instruction, which reached 47 of 1,585. Inverting it means an instruction
that states its own trigger gets a check without anyone writing one, and the
instructions that cannot are visibly the judgement calls.

**Two properties make it safe incrementally.** An absent predicate means
"inherit the rule's existing trigger", so it can never route NARROWER than
today. And it fills in the way `rationale` and `excepted-by` did, a few points
at a time, rather than as a big bang over 1,585.

`stage` is authored, optional, and deliberately empty in all 2,143 points. The
handover that started this work called it "what later lets a design-phase
subagent skip implementation directives". This spec is that later.

## Required Reading

### Architecture Docs
- [ ] The learned-corpus drain-over-archive decision (record retired with the learned corpus) - the measurement and the rejected alternative
  → Decision: citation count is not a relevance metric; the corpus is the rationale layer for hooks and code, not for rules
  → Constraint: 1,538 of 1,585 instruction points have no machine behind them
- [ ] `ai/rules/rule-format.md` - the point format this extends
  → Constraint: `POINT_KEYS` in `internal/le/rules/points.go` is a closed tuple and `parse_point` raises on any field outside it, so a fifth key is a format change and not free text
- [ ] `docs/contributing/rule-authoring.md` - the authoring surface a predicate appears on
  → Constraint: an absent field must mean "as today", never "narrower than today"

### RFC Summaries (Scope: protocol)
Not applicable. Scope is tooling.

**Key insights:** (minimal context to resume after compaction)
- `dispatchers` in `internal/le/rules/points.go` derives the roster from the PreToolUse entries in `.claude/settings.json` and cross-checks the files on disk, so a generated check has a gated place to be registered.
- `parse_bindings` takes a file list, so a later reader is added without a second convention. That is what made assumption A-5 of the closed spec recoverable rather than a redesign.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/rules/points.go` - `POINT_KEYS`, `parse_point`, `format_point`, `gate_map`, `parse_bindings`, `dispatchers`
- [ ] `internal/le/` - builds `TRIGGERS.md` and `CORE.md` from the rendered rules

**Behavior to preserve:**
- (fill during design)

**Behavior to change:**
- (fill during design)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- (fill during design)

### Transformation Path
1. (fill during design)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| (fill during design) | | No |

### Integration Points
- (fill during design)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An absent `applies` can inherit the rule's trigger without narrowing routing | `parse_point` treats absent optional keys as empty today | Points silently stop reaching sessions | Router report before and after | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A generated check fires wrongly and blocks real work | An author reports a refusal they cannot act on | Generate only for predicates that name a path or a tool, never a judgement |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An instruction stops reaching sessions, or a generated check blocks legitimate work |
| How is it reverted? | Single commit revert; the field is additive and absent means as-today |
| Who else touches this path? | Any session editing a rule |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| (fill during design) | → | | |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (fill during design) | |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | `internal/le/` | | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (fill during design) | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (fill during design) | `.claude/hooks/fixtures/` | | |

### Interop Tests (Scope: protocol)
Not applicable. No wire-visible behavior.

## Files to Modify
- `internal/le/rules/points.go` - (fill during design)

## Files to Create
- (fill during design)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Tooling only |
| CLI commands/flags | No | Make targets, not `ze` subcommands |
| Doctor check for runtime dependencies | No | No new runtime dependency |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Agent-facing tooling |
| 6 | Has a user guide page? | Yes | `docs/contributing/rule-authoring.md` |
| 15 | Registered inventory changed? | Yes | `ai/INDEX.md` if a target is added |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- (fill during design)
   - Tests: (fill during design)
   - Files: (fill during design)
   - Verify: (fill during design)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Fail closed | An unparseable predicate must refuse, never route nothing |
| No narrowing | An absent `applies` routes exactly as today, proven by the router report |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| (fill during design) | |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A predicate is authored text compiled into matching; reject a pattern that can escape its intended scope |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The predicate is knowledge a hook already holds. A check knows exactly when it fires; today that knowledge sits in the check and points AT the instruction. Inverting the direction is the whole idea.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| An absent `applies` inherits the rule's trigger | require a predicate on every point | 1,585 predicates is a big bang, and a missing one must never route narrower than today |

## Known Limitations
- This does not reduce the corpus. It reduces what must be READ, which is the number that actually hurts.

## RFC Documentation (Scope: protocol)
Not applicable. No protocol code.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only

## Scope Note (2026-08-18, owner question: one agent per stage of work)

**Where the eager payload actually is.** `./le rules payload-report`:
21,561 tokens always loaded. `ai/INSTRUCTIONS.md` 6,901, `CORE.md` 13,491,
`TRIGGERS.md` 1,169.

| `CORE.md` section | Tokens | Stage-variable |
|-------------------|--------|----------------|
| Git Safety | 4,701 | Yes, the commit PROCEDURE only |
| RFC Compliance | 4,655 | Yes, protocol work only |
| Interop and Goal Validation | 1,556 | Yes, protocol work only |
| Rule Precedence | 1,399 | No, it is the ladder |
| Never Destroy, No Layering, Stale Comments | 950 | No |

Two rules carry 70% of `CORE.md`, so SECTION-level tagging on two manifests
captures most of the win. A predicate on each of 2,245 points is not needed to
reach it.

→ Decision: tag the manifest SECTION, never the point. `git-safety` has 10
sections; the corpus has about 150. That is the unit an author can hold.

**The invariant that makes narrowing safe.** The Task section states an absent
`applies` never routes NARROWER than today. The owner's goal REQUIRES narrowing
for the tagged sections, so the invariant needs its real form.

→ Constraint: a PROHIBITION stays eager; only a PROCEDURE is stage-scoped. The
ban on the bare push verb is in the `ai/INSTRUCTIONS.md` DANGER block and refused
by `.claude/hooks/pretool-bash.py` (retired; now `internal/le/hookruntime/bash.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> besides, so dropping the 4,701 tokens of
commit-helper procedure from a research agent removes a how-to, never a ban.

→ Constraint: `TRIGGERS.md` is never stage-scoped. Stage decides what is
PRE-LOADED, never what is REACHABLE, so the worst case of a mislabeled stage is
one extra Read, not an unenforced rule. That property is what makes the change
cheap to be wrong about.

**Routing is already built.** One skill per stage of work already exists
(`/ze-explore` research, `/ze-implement` implementation, `/ze-commit` and
`/ze-close` commit, `/ze-review` review), so the skill IS the stage label and no
new detection is needed.

**Open scope decision for the owner:** generated checks (the spec's stated
reason to do it, and its R-1) in the same spec, or split so the loading change
lands alone.

## Current Behavior, verified 2026-08-18 (research phase)

Read by an agent, each claim re-verified against the producing function in the
main thread.

| Producer | What it does today |
|----------|--------------------|
| `render_text` (`internal/le/rules/points.go`) | emits the manifest heading, then EVERY point body of that section, unconditionally. It never reads `Point.stage` |
| `ManifestSection` + `MANIFEST_SECTION` (same file) | a section line carries `slug`, `heading` and a `^` tight marker, and nothing else. There is no metadata slot |
| `POINT_KEYS` / `parse_point` (same file) | `stage` is already an authored per-point key, refused if misspelled, and empty in all 2,245 points. `split_rule` writes it empty for every rule-derived point |
| `load_rules` (`internal/le/`) | parses each RENDERED `ai/rules/<rule>.md` as one whole record. Section identity does not survive into it |
| `core_members` (same file) | derives the always-on set at WHOLE-RULE granularity from the precedence ladder, plus three fail-closed reasons |
| `ARTIFACTS` (same file) | `(("TRIGGERS.md", build_triggers), ("CORE.md", build_core))`, and its comment states a new artifact needs only a row here |

→ Constraint: the two generators are DECOUPLED. `rules_condensed.py` imports
`re`, `sys`, `collections` and `pathlib` and nothing local; `rules_points.py` is
imported only by its own test. So a section-level `stage` cannot reach
`build_core` through the rendered file, which carries headings and no section
metadata.

→ Decision: the stage view is a NEW artifact produced by `rules_points.py`,
which already owns manifests and sections. `CORE.md` stays whole-rule and
`core_members` is untouched. The alternative, teaching `rules_condensed.py` to
read points, couples two generators to save one walk.

→ Constraint: the authored unit is the manifest SECTION line, so
`MANIFEST_SECTION`, `parse_manifest`, `format_manifest` and `ManifestSection`
change together, and `./le rules points-roundtrip-check` is what proves the
new line shape survives split and render. A line shape it does not know raises
`RulePointsError`, so render-check, roundtrip-check and gate-map-report fail
hard rather than silently.

→ Decision: the dead per-point `stage` key is not the carrier. Authoring 2,245
of them was rejected in the Scope Note. Retiring the key, or stamping it from
the section at render, is a DESIGN-gate call.

→ Constraint: tests extend in place, no new files.
`internal/le/` covers the manifest spine (`SectionSpineTest`,
`RenderFailureTest`, `TightHeadingTest`) and `internal/le/`
covers membership and the payload budget (`CoreMembershipTest`,
`PayloadBudgetTest`, `GeneratedShapeTest`).
