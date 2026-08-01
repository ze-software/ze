# Spec: knowledge-3-rule-digest

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | spec-knowledge-0-umbrella.md |
| Phase | 5/5 |
| Deferral shard | `plan/deferrals/knowledge-3-rule-digest.md` |
| Updated | 2026-08-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ai/rules/CONDENSED.md` is imported by `CLAUDE.md` and therefore loaded whole
into every session, every subagent, on every turn. Measured 2026-08-01:

| Measure | Value |
|---|---|
| `ai/rules/CONDENSED.md` | 398,354 bytes, about 99,600 tokens |
| Rule sections in it | 97 |
| `##` sections in it | 700 |
| Rules marked `blocking` | 69 rules, 285,022 bytes, 71% |
| Rules marked `advisory` | 28 rules, 113,332 bytes, 28% |
| Largest single contributor | `ai/rules/testing.md` at 6.4% |
| Second largest | `ai/rules/hook-mapping.md` at 6.2%, which is pure lookup reference with no directives |
| Top 20 rules' share | 55%, so the tail is flat and trimming a few files cannot fix it |

The cost is paid whether or not any of it applies to the task at hand. A session
editing a markdown file pays the same 99,600 tokens as one rewriting the BGP
wire encoder.

Every rule already carries the metadata needed to route it. 98 of 99 have a
`**When:**` trigger and all 97 digest sections have a `**Severity:**`, both
governed by `ai/rules/rule-format.md` and linted by `scripts/dev/rules_lint.py`.
The routing information exists and is unused.

### The honest ceiling

Keeping every `blocking` rule eager saves at most 28% (about 28,300 tokens).
Reaching a materially smaller eager payload therefore requires routing blocking
rules too, which is the part that can go wrong. The design below makes that safe
by never dropping *awareness* of a rule, only its body: the trigger index lists
all 97 rules in every session, so a rule can always be loaded on demand and is
never invisible.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/rule-format.md` - owns `**When:**` and `**Severity:**`
  → Constraint: the trigger is a routing key, one complete clause, one line, and
    must name what the author is DOING. That is exactly what a router needs.
  → Constraint: the digest keeps only bullets, table rows and `**bold**` lines
    verbatim; prose is truncated. Any routing change must preserve that shape.
- [ ] `ai/rules/canonical-sources.md` - the digest is generated, never hand-edited
  → Constraint: `ai/rules/CONDENSED.md` is produced by `make ze-rules-condensed`.
    Changing what is loaded means changing the generator and the import, never
    editing the digest.
- [ ] `ai/rules/rule-precedence.md` - the ladder that resolves rule conflicts
  → Constraint: rung 1 and rung 2 rules (irreversible actions, outside-facing
    correctness) must never be behind a trigger. They apply before a task type
    is even known.

**Key insights:**
- Trimming individual rules cannot work: no rule exceeds 6.4% and the top 20 are
  only 55%.
- `ai/rules/hook-mapping.md` is 6.2% of the payload and contains no directives.
  It is a lookup table and is the clearest candidate for on-demand loading.
- The safety property that makes routing acceptable is that the trigger index is
  always present, so nothing becomes undiscoverable.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/rules_condensed.py` - `condense_body` and `flush_prose` keep
  bullets, table rows and bold lines verbatim, keep only the first prose
  paragraph of a section truncated to one sentence or 220 characters, and drop
  later prose paragraphs outright
- [ ] `scripts/dev/rules_lint.py` - enforces the title, the `**When:**` trigger
  and the severity; the structural guarantee a router depends on
- [ ] `ai/INSTRUCTIONS.md` - the `@ai/rules/CONDENSED.md` import that makes the
  digest always-loaded, and the "Before You..." dispatch table above it
- [ ] `ai/rules/INDEX.md` - the existing one-line-per-rule overview, the nearest
  thing to a trigger index that already exists

**Behavior to preserve:**
- Rule files under `ai/rules/` are not edited by this spec. Only assembly changes.
- `make ze-rules-condensed` remains the generator and the digest remains generated.
- Every `blocking` rule remains reachable in every session.
- The digest's line shape, so a routed section reads identically to today.

**Behavior to change:**
- `CLAUDE.md` stops importing all 97 condensed sections.
- A trigger index covering all 97 rules is loaded instead, plus an always-on core.
- A rule's directives are loaded when its trigger matches the work.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A session starts. `CLAUDE.md`, generated from `ai/INSTRUCTIONS.md`, imports the
  rule payload.

### Transformation Path
1. `make ze-rules-condensed` reads all 97 rules and today emits one digest.
2. Under this spec it emits two artifacts: `ai/rules/TRIGGERS.md`, one line per
   rule carrying name, path, `**When:**` and `**Severity:**`; and
   `ai/rules/CORE.md`, the full condensed directives of the always-on set.
3. `ai/INSTRUCTIONS.md` imports both instead of `CONDENSED.md`.
4. `ai/rules/CONDENSED.md` continues to be generated, unimported, as the
   on-demand source a session reads when a trigger matches.
5. A router report compares, for a corpus of past tasks, which rules the trigger
   index would have surfaced against the full digest, and reports any blocking
   rule that would have been missed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Generator ↔ digest | `make ze-rules-condensed` writes the artifacts | Yes, `ai/rules/rule-format.md` names the target |
| Instructions ↔ session | `@` import in `ai/INSTRUCTIONS.md` generates `CLAUDE.md` | Yes, visible in the loaded context |
| Session ↔ on-demand rule | the agent reads `ai/rules/<name>.md` when a trigger matches | No, depends on model behavior, which is why the always-on core exists |

### Integration Points
- `ai/INSTRUCTIONS.md` is canonical for `CLAUDE.md` and `AGENTS.md`; changing the
  import requires `make ze-ai-instructions`.
- `ai/rules/rule-format.md` documents the digest contract and must record the split.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | The trigger index is generated from the same parse as the digest, not a second list |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A, no wire path touched |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | The always-on set is derived from rule metadata, never a hand-maintained list of filenames |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `**When:**` triggers are precise enough to route without dropping a rule that should have fired | `ai/rules/rule-format.md` requires a situation clause; `rules_lint.py` enforces it | A session silently loses a blocking rule, which is worse than the token cost | The router report over a corpus of past tasks, before the import is switched | unvalidated |
| A-2 | The always-on core can be derived from metadata rather than hand-listed | `rule-precedence.md` defines rungs 1 and 2 by subject, not by name | The core becomes a hand-maintained list that drifts, reintroducing the original problem | Derive it and diff against a hand-picked list; investigate every difference | unvalidated |
| A-3 | Keeping `CONDENSED.md` generated but unimported is enough for on-demand reads | It stays a single file a session can open | Sessions cannot find the body of a triggered rule | The trigger index links the rule file directly, so the digest is a convenience, not the path | unvalidated |
| A-4 | A model that sees a matching trigger will load the rule | Unproven. This is the core behavioral bet of the spec | Rules are silently skipped and quality regresses with no signal | The miss-detector (below): a Stop-hook check reporting blocking rules whose trigger matched the session's touched file types but whose file was never read. Run for at least one week before the import switches | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A blocking rule stops reaching sessions that needed it | A hook fires on something a loaded rule would have prevented | Ship report-only first; keep every rung-1 and rung-2 rule in the always-on core permanently |
| R-2 | The saving is smaller than hoped once the core is sized | The core exceeds 40,000 tokens | Report the measured core size before switching the import; if the saving is under 50%, stop and reconsider |
| R-3 | Subagents behave differently from the main thread | Subagent reports show rule violations the main thread avoids | `.claude/hooks/subagent-context.sh` already injects context; extend it with the trigger index |
| R-4 | The split makes rule authoring harder | Authors edit the wrong artifact | Both artifacts stay generated, and `canonical-sources.md` already forbids editing generated files |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every session and subagent. This is the highest blast radius of the three children, which is why it lands last |
| How is it reverted? | Restore the single `@ai/rules/CONDENSED.md` import in `ai/INSTRUCTIONS.md` and run `make ze-ai-instructions`. One commit |
| Who else touches this path? | Every session in every checkout. A regression is felt immediately and everywhere |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-rules-condensed` | → | the trigger-index emitter in `scripts/dev/rules_condensed.py` | `test_triggers_cover_every_rule` in `scripts/dev/rules_condensed_test.py` |
| `make ze-rules-condensed` | → | the always-on core emitter | `test_core_contains_every_precedence_rung_1_and_2_rule` in `scripts/dev/rules_condensed_test.py` |
| A session start | → | the `@` imports in `ai/INSTRUCTIONS.md` | `test_instructions_import_triggers_and_core` in `scripts/dev/rules_condensed_test.py` |
| `make ze-rules-router-report` | → | the coverage report | `test_report_names_missed_blocking_rule` in `scripts/dev/rules_router_test.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-rules-condensed` runs | It emits `ai/rules/TRIGGERS.md` with exactly one line per rule, all 97 present, each under 200 characters |
| AC-2 | `ai/rules/TRIGGERS.md` is inspected | Every line carries the rule path, its `**When:**` trigger and its `**Severity:**` |
| AC-3 | `make ze-rules-condensed` runs | It emits `ai/rules/CORE.md` containing the full condensed directives of every rung-1 and rung-2 rule from `ai/rules/rule-precedence.md`, derived from metadata rather than a hand-written filename list |
| AC-4 | A rule is added under `ai/rules/` | It appears in `TRIGGERS.md` on the next generate with no other edit |
| AC-5 | The always-loaded payload is measured after the import switches | `ai/INSTRUCTIONS.md` plus `TRIGGERS.md` plus `CORE.md` is under 40,000 tokens, down from about 105,400 |
| AC-6 | `make ze-rules-router-report` runs over a corpus of past task descriptions | It reports, per task, which rules the triggers would surface and names any `blocking` rule the full digest carries that the triggers would miss |
| AC-7 | The report is run before the import switches | Zero blocking rules are missed, or every miss is added to the always-on core |
| AC-8 | The import is switched | `make ze-ai-instructions` regenerates `CLAUDE.md` and `AGENTS.md`, and `make ze-verify` passes |
| AC-9 | A session needs a rule not in the core | `ai/rules/TRIGGERS.md` names it and links its file, so it is loadable in one read |
| AC-10 | A session touches files whose types match a `blocking` rule's trigger, and never reads that rule's file | The Stop-hook miss-detector reports it by name, advisory (exit 1), never blocking the stop |
| AC-11 | A session reads every blocking rule whose trigger matched | The miss-detector reports nothing and exits 0 |
| AC-12 | The miss-detector has run for at least one week before the import is switched | Its accumulated report is pasted into the spec, and every repeatedly-missed blocking rule is added to the always-on core |

### The miss-detector

A-4 cannot be proven by reading code, so it is measured instead. A Stop-hook
check reads this session's transcript, derives which file types the session
touched, matches those against the `**When:**` triggers in `ai/rules/TRIGGERS.md`,
and compares the matched `blocking` rules against the rule files the session
actually read.

| Property | Value |
|----------|-------|
| Severity | Advisory, exit 1. It reports; it never refuses a stop |
| Placement | A new `Stop` hook, after `block-premature-stop.sh` so it can never mask a blocking gate |
| Transcript access | `scripts/dev/running_model.py` `transcript_dir` is the precedent for locating and reading the session transcript |
| Output | Appends one line per session to a report file, so the evidence accumulates rather than scrolling away |
| Purpose | It converts A-4 from a bet into a measurement. It runs BEFORE the import switches, while the full digest is still loaded, so a miss it reports is a miss that would have happened |

**CORRECTION, 2026-08-01, found by running the detector rather than reasoning
about it.** The paragraph this replaces claimed the pre-switch run validates A-4.
It does not, and the original claim was wrong.

In the current world `ai/rules/CONDENSED.md` is eagerly loaded, so a session
consults a rule by reading the digest, which leaves no trace. Opening
`ai/rules/<name>.md` is unnecessary. The detector counts only a direct read of
the rule file, so its pre-switch baseline is close to 100% miss no matter how
well the session behaved.

Measured on the session that built it: 10 blocking rules reported as missed,
including `spec-delegation.md`, `model-selection.md` and `planning.md`. That
session demonstrably FOLLOWED all three. It delegated every phase, recorded the
model acknowledgement, and used the two-commit closure. The report was wrong
about the only thing it claimed to measure.

**What the pre-switch run is actually for.** Trigger-match frequency, which is
valid and is what sizes the always-on core. A rule whose trigger matches almost
every session belongs in `CORE.md` whatever a model would do on demand. That is
a denominator, not a validation.

**When A-4 can be validated: only after the switch.** Once a rule's body is no
longer eager, opening its file is the ONLY way to consult it, so a miss becomes
a real miss. This changes the plan in one important way: the switch cannot be
gated on pre-switch proof, because no such proof is obtainable. It must instead
be REVERSIBLE and MONITORED, which it already is (a one-line import revert), and
the detector's value is highest in the days immediately after the switch, not
before it.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `**When:**` triggers are precise enough to route without dropping a rule that should have fired | `ai/rules/rule-format.md` requires a situation clause; `rules_lint.py` enforces it | A session silently loses a blocking rule, which is worse than the token cost | The router report over a corpus of past tasks, before the import is switched | unvalidated |
| A-2 | The always-on core can be derived from metadata rather than hand-listed | `rule-precedence.md` defines rungs 1 and 2 by subject, not by name | The core becomes a hand-maintained list that drifts, reintroducing the original problem | Derive it and diff against a hand-picked list; investigate every difference | unvalidated |
| A-3 | Keeping `CONDENSED.md` generated but unimported is enough for on-demand reads | It stays a single file a session can open | Sessions cannot find the body of a triggered rule | The trigger index links the rule file directly, so the digest is a convenience, not the path | unvalidated |
| A-4 | A model that sees a matching trigger will load the rule | Unproven. This is the core behavioral bet of the spec | Rules are silently skipped and quality regresses with no signal | The miss-detector (below): a Stop-hook check reporting blocking rules whose trigger matched the session's touched file types but whose file was never read. Run for at least one week before the import switches | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A blocking rule stops reaching sessions that needed it | A hook fires on something a loaded rule would have prevented | Ship report-only first; keep every rung-1 and rung-2 rule in the always-on core permanently |
| R-2 | The saving is smaller than hoped once the core is sized | The core exceeds 40,000 tokens | Report the measured core size before switching the import; if the saving is under 50%, stop and reconsider |
| R-3 | Subagents behave differently from the main thread | Subagent reports show rule violations the main thread avoids | `.claude/hooks/subagent-context.sh` already injects context; extend it with the trigger index |
| R-4 | The split makes rule authoring harder | Authors edit the wrong artifact | Both artifacts stay generated, and `canonical-sources.md` already forbids editing generated files |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every session and subagent. This is the highest blast radius of the three children, which is why it lands last |
| How is it reverted? | Restore the single `@ai/rules/CONDENSED.md` import in `ai/INSTRUCTIONS.md` and run `make ze-ai-instructions`. One commit |
| Who else touches this path? | Every session in every checkout. A regression is felt immediately and everywhere |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-rules-condensed` | → | the trigger-index emitter in `scripts/dev/rules_condensed.py` | `test_triggers_cover_every_rule` in `scripts/dev/rules_condensed_test.py` |
| `make ze-rules-condensed` | → | the always-on core emitter | `test_core_contains_every_precedence_rung_1_and_2_rule` in `scripts/dev/rules_condensed_test.py` |
| A session start | → | the `@` imports in `ai/INSTRUCTIONS.md` | `test_instructions_import_triggers_and_core` in `scripts/dev/rules_condensed_test.py` |
| `make ze-rules-router-report` | → | the coverage report | `test_report_names_missed_blocking_rule` in `scripts/dev/rules_router_test.py` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-rules-condensed` runs | It emits `ai/rules/TRIGGERS.md` with exactly one line per rule, all 97 present, each under 200 characters |
| AC-2 | `ai/rules/TRIGGERS.md` is inspected | Every line carries the rule path, its `**When:**` trigger and its `**Severity:**` |
| AC-3 | `make ze-rules-condensed` runs | It emits `ai/rules/CORE.md` containing the full condensed directives of every rung-1 and rung-2 rule from `ai/rules/rule-precedence.md`, derived from metadata rather than a hand-written filename list |
| AC-4 | A rule is added under `ai/rules/` | It appears in `TRIGGERS.md` on the next generate with no other edit |
| AC-5 | The always-loaded payload is measured after the import switches | `ai/INSTRUCTIONS.md` plus `TRIGGERS.md` plus `CORE.md` is under 40,000 tokens, down from about 105,400 |
| AC-6 | `make ze-rules-router-report` runs over a corpus of past task descriptions | It reports, per task, which rules the triggers would surface and names any `blocking` rule the full digest carries that the triggers would miss |
| AC-7 | The report is run before the import switches | Zero blocking rules are missed, or every miss is added to the always-on core |
| AC-8 | The import is switched | `make ze-ai-instructions` regenerates `CLAUDE.md` and `AGENTS.md`, and `make ze-verify` passes |
| AC-9 | A session needs a rule not in the core | `ai/rules/TRIGGERS.md` names it and links its file, so it is loadable in one read |
| AC-10 | A session touches files whose types match a `blocking` rule's trigger, and never reads that rule's file | The Stop-hook miss-detector reports it by name, advisory (exit 1), never blocking the stop |
| AC-11 | A session reads every blocking rule whose trigger matched | The miss-detector reports nothing and exits 0 |
| AC-12 | The miss-detector has run for at least one week before the import is switched | Its accumulated report is pasted into the spec, and every repeatedly-missed blocking rule is added to the always-on core |

### The miss-detector

A-4 cannot be proven by reading code, so it is measured instead. A Stop-hook
check reads this session's transcript, derives which file types the session
touched, matches those against the `**When:**` triggers in `ai/rules/TRIGGERS.md`,
and compares the matched `blocking` rules against the rule files the session
actually read.

| Property | Value |
|----------|-------|
| Severity | Advisory, exit 1. It reports; it never refuses a stop |
| Placement | A new `Stop` hook, after `block-premature-stop.sh` so it can never mask a blocking gate |
| Transcript access | `scripts/dev/running_model.py` `transcript_dir` is the precedent for locating and reading the session transcript |
| Output | Appends one line per session to a report file, so the evidence accumulates rather than scrolling away |
| Purpose | It converts A-4 from a bet into a measurement. It runs BEFORE the import switches, while the full digest is still loaded, so a miss it reports is a miss that would have happened |

The detector is deliberately run against the CURRENT eager-digest world first.
In that world every rule is already loaded, so a "miss" means the session never
consulted a rule its own file types matched. That is exactly the population at
risk once the body stops being eager.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_triggers_cover_every_rule` | `scripts/dev/rules_condensed_test.py` | AC-1, AC-4 | |
| `test_trigger_line_has_path_when_severity` | `scripts/dev/rules_condensed_test.py` | AC-2 | |
| `test_core_contains_every_precedence_rung_1_and_2_rule` | `scripts/dev/rules_condensed_test.py` | AC-3 | |
| `test_core_is_derived_not_hardcoded` | `scripts/dev/rules_condensed_test.py` | AC-3, no filename list in the generator | |
| `test_instructions_import_triggers_and_core` | `scripts/dev/rules_condensed_test.py` | AC-8 | |
| `test_report_names_missed_blocking_rule` | `scripts/dev/rules_router_test.py` | AC-6 | |
| `test_payload_under_budget` | `scripts/dev/rules_condensed_test.py` | AC-5 | |
| `test_detector_reports_unread_matched_rule` | `scripts/dev/rule_coverage_test.py` | AC-10 | |
| `test_detector_silent_when_all_read` | `scripts/dev/rule_coverage_test.py` | AC-11 | |
| `test_detector_never_blocks_stop` | `scripts/dev/rule_coverage_test.py` | Exit is 1 or 0, never 2 | |
| `test_detector_ignores_advisory_rules` | `scripts/dev/rule_coverage_test.py` | Only `blocking` rules are reported, so the signal stays actionable | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Trigger line length | 0-200 | 200 | N/A | 201 |
| Always-loaded payload, tokens | 0-40000 | 40000 | N/A | 40001 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rules_condensed_test.py` | `scripts/dev/rules_condensed_test.py` | A contributor adds a rule and it appears in the trigger index with no further edit | |
| `rules_router_test.py` | `scripts/dev/rules_router_test.py` | An agent runs `make ze-rules-router-report` and sees every blocking rule the triggers would miss | |

### Interop Tests (Scope: protocol)
N-A. Scope is tooling. No wire-visible behavior changes.

## Files to Modify

- `scripts/dev/rules_condensed.py` - emit `TRIGGERS.md` and `CORE.md` beside `CONDENSED.md`
- `ai/INSTRUCTIONS.md` - import the trigger index and the core instead of the whole digest
- `ai/rules/rule-format.md` - document the three artifacts and which is imported
- `ai/rules/canonical-sources.md` - add the two new generated targets to the sync table
- `mk/inventory.mk` - declare `ze-rules-router-report`
- `.claude/hooks/subagent-context.sh` - inject the trigger index into subagents
- `ai/INDEX.md` - Dev Tools row for the report target

## Files to Create

- `ai/rules/TRIGGERS.md` - generated, one line per rule
- `ai/rules/CORE.md` - generated, always-on directives
- `scripts/dev/rules_router.py` - the coverage report
- `scripts/dev/rules_router_test.py` - its tests
- `.claude/hooks/rule-coverage-report.sh` - the Stop-hook miss-detector, advisory
- `scripts/dev/rule_coverage.py` - the detector's logic, transcript parsing and matching
- `scripts/dev/rule_coverage_test.py` - its tests
- `scripts/dev/rules_condensed_test.py` - generator tests, if absent
- `plan/deferrals/knowledge-3-rule-digest.md` - deferral shard

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface |
| YANG validation constraints | N-A | No YANG leaf added |
| YANG custom validators | N-A | No YANG leaf added |
| CLI commands/flags | N-A | Make targets only |
| CLI grammar (keyword before value) | N-A | No CLI command added |
| Editor autocomplete | N-A | No YANG leaf added |
| Functional test for new RPC/API | N-A | No RPC added |
| Pipe completeness | N-A | No `ze` command output |
| Env var registration | N-A | No `environment/` leaf added |
| Doctor check for runtime dependencies | N-A | Build-time only |
| Prometheus counters/metrics | N-A | No daemon state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Agent tooling |
| 2 | Config syntax changed? | No | No config surface |
| 3 | CLI command added/changed? | No | No `ze` command |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | No | Contributor tooling |
| 7 | Wire format changed? | No | No wire path |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No protocol behavior |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/` gains the router report |
| 11 | Affects daemon comparison? | No | No daemon capability |
| 12 | Internal architecture changed? | No | No `internal/` change |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` and `ai/` for anchors naming `rules_condensed.py` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/rules/rule-format.md` and `ai/rules/canonical-sources.md` both describe the current single-digest flow |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- emit `TRIGGERS.md` and `CORE.md`
   alongside the existing digest, import nothing new yet
   - Tests: `test_triggers_cover_every_rule`, `test_core_contains_every_precedence_rung_1_and_2_rule`
   - Files: `scripts/dev/rules_condensed.py`
   - Verify: both artifacts generate; `CLAUDE.md` is unchanged
2. **Phase: The miss-detector (validates A-4, MUST run before any import change)**
   -- build the Stop hook and let it accumulate evidence in the current
   eager-digest world for at least one week
   - Tests: `test_detector_reports_unread_matched_rule`, `test_detector_silent_when_all_read`, `test_detector_never_blocks_stop`, `test_detector_ignores_advisory_rules`
   - Files: `.claude/hooks/rule-coverage-report.sh`, `scripts/dev/rule_coverage.py`
   - Verify: the detector reports on a seeded session and never returns exit 2.
     Then WAIT. Do not proceed to phase 4 until a week of report data exists
3. **Phase: The router report** -- build `rules_router.py` and run it over a
   corpus of past task descriptions drawn from closed specs
   - Tests: `test_report_names_missed_blocking_rule`
   - Verify: the report names every blocking rule the triggers would miss
4. **Phase: Size the core** -- add every rule missed by EITHER the router report
   or the accumulated detector data to the always-on core, then measure
   - Tests: `test_payload_under_budget`
   - Verify: under 40,000 tokens with zero missed blocking rules. If the saving
     is under 50%, STOP and report to the user before switching the import
5. **Phase: Switch the import** -- change `ai/INSTRUCTIONS.md`, regenerate, and
   extend `subagent-context.sh`
   - Tests: `test_instructions_import_triggers_and_core`
   - Verify: `make ze-ai-instructions`, `make ze-verify`, and a manual read of a
     fresh session's loaded context. Leave the detector running afterwards: it is
     the only ongoing signal that routing has not silently dropped a rule

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation |
| Correctness | Every rung-1 and rung-2 rule from `rule-precedence.md` is in the core, and the membership is derived, not hardcoded |
| Correctness | A rule added under `ai/rules/` reaches `TRIGGERS.md` with no second edit |
| Naming | The generated artifacts follow the existing all-caps generated convention (`CONDENSED.md`, `INDEX.md`) |
| Data flow | `TRIGGERS.md` and `CORE.md` come from the same parse as `CONDENSED.md`, never a second source |
| Rule: `ai/rules/canonical-sources.md` | Both new artifacts are listed as generated and are never hand-edited |
| Rule: `ai/rules/derive-not-hardcode.md` | The always-on set is derived from metadata |
| Rule: `ai/rules/fail-closed-guards.md` | A rule with no parseable trigger goes into the core rather than being dropped |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Trigger index generated | `wc -l ai/rules/TRIGGERS.md` shows 97 rule lines |
| Core generated | `ls ai/rules/CORE.md` |
| Payload under budget | `wc -c ai/INSTRUCTIONS.md ai/rules/TRIGGERS.md ai/rules/CORE.md` |
| Zero missed blocking rules | `make ze-rules-router-report` |
| Import switched | `grep -n 'TRIGGERS\|CORE' ai/INSTRUCTIONS.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The generator parses rule markdown. A malformed rule must fail the generate loudly, never emit a silently truncated trigger |
| Fail-closed | A rule whose trigger cannot be parsed must land in the always-on core, never be omitted from both artifacts |

### Failure Routing
| Failure | Route To |
|---------|----------|
| The report names blocking rules the triggers miss | Add them to the core; if the core then exceeds budget, back to DESIGN |
| The saving is under 50% after sizing the core | STOP and report. The spec's premise does not hold |
| A session visibly loses a rule after the switch | Revert the import immediately; this is a one-commit revert |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The metadata needed to fix this already exists and is already linted. The work
  is assembly, not authoring.
- The safety property is awareness, not inclusion. A rule whose body is not
  loaded is still named in every session, so nothing becomes undiscoverable.
- `hook-mapping.md` at 6.2% with zero directives is the proof that the digest
  currently carries reference material it never needed to carry.


### ~~Steps 1, 3 and 4 landed. Step 5 is HELD (2026-08-01)~~ SUPERSEDED

~~Step 5 is HELD.~~ The owner released the hold the same day and Step 5 landed.
The section below is kept for its measurements, which are unchanged. Read
"Step 5 landed" after it for what the hold decided.

| Measure | Value |
|---------|-------|
| Payload today (`INSTRUCTIONS` + `CONDENSED`) | 107,548 tokens |
| Payload routed (`INSTRUCTIONS` + `TRIGGERS` + `CORE`) | 21,189 tokens |
| Saving | **80%**, against a 50% stop-floor and a 40,000-token budget. AC-5 MET |
| Always-on core | 12 rules, derived from `rule-precedence.md` rungs 1-2 plus fail-closed plus router-unreachable |
| Router corpus | 987 task descriptions; 57 of 64 routed blocking rules surfaced, mean 6.0 per task |
| Blocking rules no task would surface | 7, all folded into the core |

**Step 5, the import switch, is deliberately NOT done.** Its blast radius is
every session, every subagent, every turn, and the one assumption it rests on
(A-4: a model loads a rule when its trigger matches) can only be validated AFTER
the switch, as this spec's own CORRECTION records. The detector has zero days of
data. Switching now would be trading a measured 80% saving against an unmeasured
behavioural risk, on the operator's context rather than mine. The owner decides.

**Two findings from the build.**

The core derivation takes the router corpus as a GENERATOR INPUT rather than a
report a human acts on. A blocking rule no past task would surface is added to
the core automatically. Copying those 7 names into a list would read identically
today and rot at the next rule.

`rules_index.py` and `rules_lint.py` filtered `ai/rules/` by the literal names
`INDEX.md` and `CONDENSED.md`. The moment `TRIGGERS.md` and `CORE.md` existed the
lint failed them as malformed rules and the digest ingested itself, reporting
"Rules: 99". All three now filter by all-caps stem shape. Verified: on a tree
without the new artifacts the generator's `CONDENSED.md` is byte-identical to
HEAD's, so the change is additive.

### Step 5 landed: the import is switched (2026-08-01)

| Measure | Value |
|---------|-------|
| `ai/INSTRUCTIONS.md` | 24,026 chars, 6,006 tokens |
| `ai/rules/TRIGGERS.md` | 13,435 chars, 3,358 tokens |
| `ai/rules/CORE.md` | 47,966 chars, 11,991 tokens |
| Always-loaded TOTAL | 85,427 chars, **21,356 tokens** |
| Before (`INSTRUCTIONS` + `CONDENSED`) | 430,318 chars, 107,579 tokens |
| Saving | **80.1%**, against a 50% stop-floor. Budget 40,000 tokens: MET |

`ai/INSTRUCTIONS.md` imports `TRIGGERS.md` and `CORE.md`. It imports
`CONDENSED.md` no longer. `make ze-ai-instructions` regenerated `CLAUDE.md` and
`AGENTS.md`, which carry the same two imports and no digest import.

**Why the hold ended rather than waiting for detector data.** The hold waited on
A-4, and this spec's own CORRECTION records that A-4 cannot be measured before
the switch. While the digest is eager a session consults a rule by reading the
digest, which leaves no trace, so the pre-switch miss rate is near 100% whatever
the session did. Waiting produces no evidence. The mitigation is that the change
is reversible and monitored, which it is: the detector runs from now on, and the
revert is one line.

**The revert (AC: Blast Radius).** In `ai/INSTRUCTIONS.md`, replace the two
import lines `@ai/rules/TRIGGERS.md` and `@ai/rules/CORE.md` with the single
line `@ai/rules/CONDENSED.md`, then run `make ze-ai-instructions`. Nothing else
changes. `CONDENSED.md` is still generated on every `make ze-rules-condensed`,
which is what keeps the revert to one line, and
`test_condensed_still_generated_so_the_revert_works` fails if that stops being
true.

**Subagents.** `.claude/hooks/subagent-context.sh` now names `TRIGGERS.md` and
states the read-on-demand contract. It POINTS at the index rather than inlining
it, because a subagent in this harness already receives `CLAUDE.md` with both
imports resolved. Inlining 13,435 chars per spawn would pay the index twice and
erode the saving on exactly the path this repository uses most, since every spec
phase runs in a subagent (`ai/rules/spec-delegation.md`). If a harness is ever
found that does not propagate `CLAUDE.md` to subagents, the hook must inline the
index instead, and this decision must be revisited.

**A-4 remains `unvalidated`, by construction.** The detector
(`.claude/hooks/rule-coverage-report.sh`) is now the ongoing signal, and its
data is meaningful from this commit forward rather than before it.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Trigger index plus derived always-on core | Keep all blocking rules eager | That caps the saving at 28%, measured. The trigger index makes routing blocking rules safe by preserving awareness |
| Derive the core from `rule-precedence.md` rungs | Hand-list the always-on filenames | A hand-list drifts, which is the failure this whole spec set exists to stop |
| Report-only before switching the import | Switch and watch for regressions | The blast radius is every session; a regression found afterwards is found by damage |
| Measure A-4 with a Stop-hook detector rather than accepting it | Accept the bet with the always-on core as the only mitigation; or route advisory rules only, capping the saving at 28% | The detector runs in the CURRENT world where every rule is already loaded, so a reported miss is a real one. It turns the spec's one unprovable assumption into a week of data, and it keeps running afterwards as the ongoing regression signal |
| Keep generating `CONDENSED.md` unimported | Delete it | It stays useful as a single-file read, and keeping it makes the change a one-line revert |

## Known Limitations

- A-4 cannot be PROVEN mechanically: whether a model loads a rule when its
  trigger matches is behavior, not code. It can be MEASURED, which is what the
  miss-detector does, but a week of data is evidence rather than proof. The
  always-on core remains the structural mitigation, and it is why every rung-1
  and rung-2 rule stays eager permanently.
- The detector infers "this rule was relevant" from touched file types against a
  trigger clause. A rule whose trigger is about an action rather than a file type
  (for example `ai/rules/never-destroy-work.md`) cannot be matched this way, so
  the detector under-reports. That is the safe direction: it never claims
  coverage it has not observed.
- The router report measures coverage against past task descriptions. It cannot
  anticipate a task shape the corpus does not contain.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
