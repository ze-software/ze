# Spec: rules-situation-index

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The rule corpus is stored by topic and consumed by situation.**

`plan/learned/1356-learned-corpus-drain-over-archive.md` carries the measurement.
Every artifact today, the rendered rule, `TRIGGERS.md`, `CORE.md`, the router, is
a rearrangement of a DOCUMENT. An agent mid-task needs the two to five
instructions bearing on what it is about to do. Measured over 1,043 past task
descriptions: mean 2.5 rules surfaced per task out of 27, and
`scripts/dev/rule_coverage.py` exists precisely because sessions do not open the
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
- [ ] `plan/learned/1356-learned-corpus-drain-over-archive.md` - the measurement and the rejected alternative
  → Decision: citation count is not a relevance metric; the corpus is the rationale layer for hooks and code, not for rules
  → Constraint: 1,538 of 1,585 instruction points have no machine behind them
- [ ] `ai/rules/rule-format.md` - the point format this extends
  → Constraint: `POINT_KEYS` in `scripts/dev/rules_points.py` is a closed tuple and `parse_point` raises on any field outside it, so a fifth key is a format change and not free text
- [ ] `docs/contributing/rule-authoring.md` - the authoring surface a predicate appears on
  → Constraint: an absent field must mean "as today", never "narrower than today"

### RFC Summaries (Scope: protocol)
Not applicable. Scope is tooling.

**Key insights:** (minimal context to resume after compaction)
- `dispatchers` in `scripts/dev/rules_points.py` derives the roster from the PreToolUse entries in `.claude/settings.json` and cross-checks the files on disk, so a generated check has a gated place to be registered.
- `parse_bindings` takes a file list, so a later reader is added without a second convention. That is what made assumption A-5 of the closed spec recoverable rather than a redesign.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `scripts/dev/rules_points.py` - `POINT_KEYS`, `parse_point`, `format_point`, `gate_map`, `parse_bindings`, `dispatchers`
- [ ] `scripts/dev/rules_condensed.py` - builds `TRIGGERS.md` and `CORE.md` from the rendered rules

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
| (fill during design) | `scripts/dev/rules_points_test.py` | | |

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
- `scripts/dev/rules_points.py` - (fill during design)

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
- [ ] `make ze-verify` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
