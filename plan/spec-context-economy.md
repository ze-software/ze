# Spec: context-economy

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/context-economy.md` (or `-` if nothing deferred) |
| Updated | 2026-08-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A token audit of the 33 session transcripts under
`~/.claude/projects/-home-thomas-Code-github-com-ze-software-ze-main/`
(main threads plus 458 subagent transcripts, deduped by message id) measured
where the work in this repository spends its tokens.

| Measure | Value |
|---------|-------|
| API calls | 39,491 (9,370 main thread, 30,126 subagent) |
| Tokens fed to the model | 12.3B, for 229M distinct |
| Re-read factor | every distinct context token is read again 53 times |
| Spend split | cache-read 78%, cache-write 18%, output 4% |
| Subagent share | 63% |

Cost per API call is the context size at that call. The bill is therefore
`round trips x context`, and nothing else moves it much. Three properties of
current practice make both terms large:

| Property | Measurement |
|----------|-------------|
| Implementation agents run long | 169 API calls and 261k mean context per agent; the largest single agent ran 724 calls |
| The main thread never sheds context | 554k mean; 47% of main-thread spend happens above 600k, at the 1M ceiling |
| Tool calls are not batched | 85% of API calls carry exactly one tool call |

Two second-order costs follow from the same shape: 62% of all file Reads
re-read a path already read in that session (`scripts/dev/rfc_requirements.py`
300 times), and the always-loaded preamble is 23k tokens on every one of the
39,491 calls.

The goal is to lower both terms without weakening any gate. Review is
explicitly NOT a target: 139 review agents account for 11% of subagent spend,
and the fix/debug phase they prevent accounts for 27%.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/planning.md` - owns delegation, the supervisor contract, and the review loop
  → Decision: the main thread supervises and does not perform spec work; each phase runs in a subagent through its `ze-*` skill
  → Constraint: "Spawning an agent costs a round trip" is already banned reasoning, so this spec must lower cost by SIZING agents, never by spawning fewer
- [ ] `ai/rules/rule-format.md` - the contract every `ai/rules/*.md` file obeys
  → Constraint: `# Title`, then `**When:**`, `**Severity:**`, optional `**Related:**`, contiguous, in that order
  → Constraint: directives must be bullets, table rows, or bold lines; `condense_body` keeps only the first prose paragraph of a section
- [ ] `ai/rules/rule-precedence.md` - the ladder `core_members` parses
  → Decision: always-on membership is derived from rungs 1 and 2, so a new advisory-weight rule cannot be made always-on without claiming a rung it does not hold
- [ ] `ai/rules/completion.md` - no partial work, no parking
  → Constraint: a work-package boundary is a phase boundary (rung 4), never permission to abandon an unfinished package

**Key insights:**
- The measured cost driver is round trips times context, not the size of any tool output. Bash results average 438 tokens and are not worth trimming.
- `.claude/hooks/subagent-context.sh` already briefs every spawned agent, so per-agent guidance costs nothing to distribute.
- `tmp/session/session-state-<spec-stem>-<SID>.md` and `_find_latest_state_for_spec` already implement a cross-session per-spec digest. The digest row is wiring, not a new mechanism.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `scripts/dev/rules_condensed.py` - `main`, `core_members`, `precedence_rung_slugs`, `condense_body` generate `ai/rules/TRIGGERS.md` and `ai/rules/CORE.md` from one `load_rules` parse. `core_members` derives the always-on set from the ladder table plus three fail-closed conditions
- [ ] `scripts/dev/rules_index.py` - `build`, `summarise` generate `ai/rules/INDEX.md`
- [ ] `scripts/dev/rules_lint.py` - `check_rule`, `check_trigger`, `check_severity_agrees` enforce the rule-file contract
- [ ] `scripts/dev/skill_sync.sh` - `generate_into` copies `ai/skills/<name>.md` to the three gitignored mirrors; `--check` diffs them
- [ ] `scripts/dev/python_tests_test.go` - `TestPythonUnitTests` runs every `*_test.py` under `pythonTestRoots`, which includes `scripts/dev`, so a sibling test needs no make target
- [ ] `.claude/hooks/subagent-context.sh` - injects architecture constraints, the git branch, the parent's claimed spec and Status, and a fixed subagent contract into every spawn
- [ ] `.claude/hooks/lib/state-file.sh` - `_state_file` computes `tmp/session/session-state-<spec-stem>-<SID>.md`; `_find_latest_state_for_spec` recovers the newest such file for a spec across sessions
- [ ] `ai/skills/ze-implement.md` - eleven steps, one agent for the whole spec; step 5 walks the spec's Implementation Phases in one context
- [ ] `mk/inventory.mk` - every doc, rules and index tool target lives here, listed in the `.PHONY` block

**Behavior to preserve:**
- Every existing gate, hook exit code, and rule directive. This spec removes no obligation.
- The delegation contract in `ai/rules/planning.md`: phases run in subagents, the main thread supervises.
- The review loop, its independence, and its lens floor.
- `core_members` fail-closed derivation. No rule is added to the ladder to move it into `CORE.md`.

**Behavior to change:**
- `/ze-implement` gains a work-package decomposition: one agent per implementation phase, each handing off through the per-spec state file.
- `ai/rules/planning.md` gains a supervisor-thinness directive and an explicit main-thread handoff threshold.
- `.claude/hooks/subagent-context.sh` gains the batching and reading-discipline lines, and the per-spec digest path when one exists.
- A new `ai/rules/context-economy.md` carries the full text those two lines summarise.
- `ai/rules/CORE.md` shrinks by removing text that restates another always-on rule or `ai/INSTRUCTIONS.md` verbatim. No directive is lost.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A developer or an agent runs `make ze-token-economy`.
- Session transcripts in `~/.claude/projects/<project-slug>/*.jsonl` and `<session>/subagents/*.jsonl`, JSON lines.

### Transformation Path
1. `token_economy.py` `iter_records` reads each transcript line
2. `scan_transcript` dedupes assistant records by `message.id`, because usage is repeated on every split record of one API call
3. `summarise` aggregates per session: API calls, context mean and max, cache-read, cache-write, output, subagent count
4. `report` prints the per-session table, the context-size histogram, and the capped-context counterfactual

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Tool ↔ transcript store | read-only open of `~/.claude/projects/` | No |
| Rule text ↔ agent context | `.claude/hooks/subagent-context.sh` stdout | No |
| Agent ↔ agent | `tmp/session/session-state-<spec-stem>-<SID>.md` | No |

### Integration Points
- `mk/inventory.mk` - new `ze-token-economy` target, added to the `.PHONY` block
- `scripts/dev/python_tests_test.go` `TestPythonUnitTests` - picks up `token_economy_test.py` with no target change
- `ai/INDEX.md` "## Dev Tools" - the hand-maintained discovery table
- `.claude/hooks/lib/state-file.sh` `_find_latest_state_for_spec` - the existing digest resolver the handoff reuses

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Usage fields are repeated on every split record of one API call, so dedup by `message.id` is required | measured: 3,251 assistant records against 1,667 distinct message ids in one transcript | every published figure is inflated about 2x | `token_economy_test.py` fixture with one message id across three records | unvalidated |
| A-2 | A rule trigger-routed (not always-on) still reaches agents, because `.claude/hooks/subagent-context.sh` carries the summary | the hook's own text names `ai/rules/TRIGGERS.md` as the routing index | the guidance never arrives and the change is inert | read the hook output for a spawned agent | unvalidated |
| A-3 | Splitting one long agent into N sequential agents lowers cost, because context grows with turns inside one agent | the capped-context counterfactual: agent context held at 150k is 47% less re-read | splitting adds spawn overhead without saving re-read | re-run `make ze-token-economy` after the change and compare mean agent context | unvalidated |
| A-4 | `CORE.md` can shrink without losing a directive, because some of its text restates `ai/INSTRUCTIONS.md` | `CORE.md` is generated by `condense_body`, which keeps whole table rows | the trim can only be reached by deleting a directive, and must not be attempted | diff the directive set before and after | unvalidated |
| A-5 | The transcript directory layout (`<session>.jsonl` plus `<session>/subagents/*.jsonl`) is stable | observed across all 33 sessions | the tool reports nothing and must fail loudly rather than print zeros | `token_economy.py` exits non-zero when it finds no transcript | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A work-package boundary is read as permission to stop mid-package, which is parking under a new name | an agent reports "budget reached" with an unfinished acceptance criterion | the rule states the boundary is chosen at decomposition, and an agent that finds its package too big reports that to the main thread rather than trimming scope (`ai/rules/completion.md`) |
| R-2 | Trimming `CORE.md` deletes a safety directive | a directive present before the change is absent after | the acceptance criterion is "no directive lost", with size as an outcome and never as the gate; the diff is reviewed directive by directive |
| R-3 | The batching directive pushes agents to batch dependent calls, producing wrong results from stale reads | a batched Edit lands on a file whose content the same batch just changed | the directive names independence as the precondition, and gives the counter-example |
| R-4 | Adding text to `subagent-context.sh` grows every agent's context, the cost this spec exists to lower | the injected block grows past a few lines | keep the injection to the two directives with measured value; the full rule stays in `ai/rules/context-economy.md`, read on demand |
| R-5 | `token_economy.py` reads transcripts outside the repository, so its output depends on a machine-local store | the tool reports nothing on a fresh checkout | the tool treats an absent store as a clean skip with a stated reason, never as zero cost |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible in the product. The risk is to agent workflow: a mis-stated rule makes future sessions stop early or batch dependent calls |
| How is it reverted? | Single commit revert. The rule digests regenerate from the rule files |
| Who else touches this path? | Every session in this checkout reads `ai/rules/CORE.md` and the spawn hook, so a defect here is felt by all of them at once |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-token-economy` | → | `token_economy.py` `main` | `token_economy_test.py` `test_main_reports_from_fixture` |
| `make ze-rules-condensed-check` | → | `rules_condensed.py` `main` over the new rule file | `make ze-rules-lint` clean on `ai/rules/context-economy.md` |
| `make ze-ai-check` | → | `skill_sync.sh` `generate_into` over the edited skill | `skill_sync.sh --check` diff clean |
| agent spawn | → | `.claude/hooks/subagent-context.sh` | `scripts/dev/hook-fixture-check.py` fixture `subagent-context-carries-economy` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-token-economy` on a machine with session transcripts | prints per-session API calls, mean and max context, cache-read, cache-write, output, subagent count, the context-size histogram, and the capped-context counterfactual; exits 0 |
| AC-2 | `make ze-token-economy` where the transcript store is absent | prints the store path it looked for and the reason it found nothing, exits 0 without printing zero-valued totals |
| AC-3 | a transcript whose one API call is split across three assistant records | the tool counts one API call and one instance of its usage |
| AC-4 | `ai/rules/context-economy.md` exists | `make ze-rules-lint` passes it; `make ze-rules-condensed-check` and `make ze-rules-index-check` are clean with the regenerated digests committed |
| AC-5 | an agent is spawned | its injected context carries the batching directive and the reading-discipline directive, and the per-spec digest path when the parent has a claimed spec whose state file exists |
| AC-6 | `/ze-implement` runs against a spec with N implementation phases | the skill instructs one agent per phase, each writing its handoff into the per-spec state file, and the next agent reading that file instead of re-deriving the phase before it |
| AC-7 | `ai/rules/planning.md` is read by a main thread | it states that the main thread does not run exploration commands itself, and names the context threshold at which it writes state and hands off |
| AC-8 | `ai/rules/CORE.md` regenerated after the source-rule trim | every directive present before the change is present after; the payload reported by `make ze-rules-payload` is lower |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_dedupes_split_records` | `scripts/dev/token_economy_test.py` | one message id split across records counts once (AC-3) | |
| `test_counts_subagent_transcripts` | `scripts/dev/token_economy_test.py` | `<session>/subagents/*.jsonl` are attributed to their parent session | |
| `test_absent_store_is_explicit` | `scripts/dev/token_economy_test.py` | a missing store prints the path and reason, never zeros (AC-2) | |
| `test_context_histogram_buckets` | `scripts/dev/token_economy_test.py` | spend is attributed to the context bucket of the call that spent it | |
| `test_capped_counterfactual` | `scripts/dev/token_economy_test.py` | the capped-context figure equals the sum of per-call minimums | |
| `test_main_reports_from_fixture` | `scripts/dev/token_economy_test.py` | `main` over a fixture store exits 0 and names every section (AC-1) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `--cap` (counterfactual context cap, tokens) | 1 - 1000000 | 1000000 | 0 | 1000001 |
| `--top` (sessions listed) | 1 - 1000 | 1000 | 0 | 1001 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `subagent-context-carries-economy` | `scripts/dev/hook-fixture-check.py` | a spawned agent receives the batching and reading directives (AC-5) | |
| `subagent-context-carries-spec-digest` | `scripts/dev/hook-fixture-check.py` | a spawn under a claimed spec receives the per-spec state-file path (AC-5) | |

### Interop Tests (Scope: protocol)
N-A. Scope is tooling; no wire-visible behavior changes.

## Files to Modify
- `ai/rules/planning.md` - supervisor thinness, the main-thread handoff threshold, work-package decomposition in the delegation table
- `ai/rules/rule-precedence.md` - no change to the ladder; verify the new rule is correctly NOT on rungs 1 or 2
- `ai/rules/git-safety.md` - remove text that restates `ai/INSTRUCTIONS.md` verbatim, so `CORE.md` shrinks without losing a directive (AC-8)
- `ai/skills/ze-implement.md` - one agent per implementation phase, handoff through the per-spec state file
- `ai/skills/ze-review.md` - read the digest before re-reading sources
- `.claude/hooks/subagent-context.sh` - inject the two directives and the per-spec digest path
- `mk/inventory.mk` - `ze-token-economy` target and `.PHONY` entry
- `ai/INDEX.md` - Dev Tools row for `token_economy.py`
- `ai/rules/TRIGGERS.md`, `ai/rules/CORE.md`, `ai/rules/INDEX.md` - regenerated, committed with the rule change

## Files to Create
- `ai/rules/context-economy.md` - the rule
- `scripts/dev/token_economy.py` - the measurement tool
- `scripts/dev/token_economy_test.py` - its tests
- `plan/learned/NNN-context-economy.md` - at closure

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no runtime config; the tool is a developer command |
| YANG validation constraints | No | no YANG leaf added |
| YANG custom validators | No | no YANG leaf added |
| CLI commands/flags | No | `make` target only, not a `ze` subcommand |
| CLI grammar (keyword before value) | No | no `ze` command added |
| Editor autocomplete | No | no YANG leaf added |
| Functional test for new RPC/API | No | no RPC; hook fixtures cover the agent-facing surface |
| Pipe completeness | No | no `ze` command output |
| Env var registration | No | no `environment/` leaf added |
| Doctor check for runtime dependencies | No | the tool reads a developer-machine path and reports its absence itself; it is not a daemon dependency |
| Prometheus counters/metrics | No | no runtime observable state |
| BGP family surface (new SAFI / capability / attribute) | No | not protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | developer tooling, not a product feature |
| 2 | Config syntax changed? | No | no config touched |
| 3 | CLI command added/changed? | No | make target, not a `ze` command |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | audience is agents and developers, served by `ai/INDEX.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | No | the new tests use the existing `TestPythonUnitTests` root |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | no product code touched |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registry touched |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` and `ai/` for anchors naming `ai/skills/ze-implement.md` and `.claude/hooks/subagent-context.sh`, update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/INDEX.md` Dev Tools table gains the `token_economy.py` row |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the measurement path exists and is reachable before any rule text changes
   - Tests: `test_main_reports_from_fixture`, and the two hook fixtures, all failing
   - Files: `scripts/dev/token_economy.py` skeleton, `scripts/dev/token_economy_test.py`, `mk/inventory.mk` target and `.PHONY`
   - Verify: `make ze-token-economy` runs and exits non-zero or reports a stub; the fixtures fail because the hook carries no directive yet
2. **Phase: Measurement tool** -- AC-1, AC-2, AC-3, and the boundary tests
   - Tests: all six in `token_economy_test.py`
   - Files: `scripts/dev/token_economy.py`, `ai/INDEX.md` Dev Tools row
   - Verify: `make ze-test-pkg PKG=./scripts/dev` green; `make ze-token-economy` prints the real store's report
3. **Phase: The rule** -- AC-4
   - Tests: `make ze-rules-lint`, `make ze-rules-condensed-check`, `make ze-rules-index-check`
   - Files: `ai/rules/context-economy.md`, regenerated `TRIGGERS.md`, `CORE.md`, `INDEX.md`
   - Verify: the rule is trigger-routed, absent from `CORE.md`, and present in `TRIGGERS.md` with a lint-clean trigger line
4. **Phase: Distribution** -- AC-5
   - Tests: the two hook fixtures now pass; `scripts/dev/hook-parity-check.py` unchanged
   - Files: `.claude/hooks/subagent-context.sh`
   - Verify: the injected block grew by the two directives and the digest path only
5. **Phase: Skill and supervisor** -- AC-6, AC-7
   - Tests: `make ze-ai-check` clean; `make ze-doc-test`
   - Files: `ai/skills/ze-implement.md`, `ai/skills/ze-review.md`, `ai/rules/planning.md`, regenerated digests
   - Verify: the decomposition names the per-spec state file that `_find_latest_state_for_spec` already resolves, and adds no second mechanism
6. **Phase: Preamble trim** -- AC-8
   - Tests: directive-by-directive diff of `CORE.md`; `make ze-rules-payload` before and after
   - Files: `ai/rules/git-safety.md`, regenerated `CORE.md`
   - Verify: every directive present before is present after; the payload figure is lower

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has an implementation, named by file and symbol |
| Correctness | the dedup key is `message.id`, not `requestId`; a transcript with one message id across three records counts once |
| Correctness | the capped counterfactual sums per-call minimums, and is never presented as a prediction of a future run |
| No layering | the handoff uses `tmp/session/session-state-<spec-stem>-<SID>.md` and `_find_latest_state_for_spec`. A second digest mechanism is a defect (`ai/rules/no-layering.md`) |
| No weakened gate | no hook exit code changed, no rule directive removed, no review lens dropped |
| Rule: completion | the work-package boundary text cannot be read as permission to leave an acceptance criterion unimplemented (R-1) |
| Rule: rule-format | `ai/rules/context-economy.md` directives survive `condense_body`: bullets, table rows, or bold lines, never a second prose paragraph |
| Rule: evidence | every number quoted in the rule or the skill is reproducible by `make ze-token-economy`, and is labelled with the corpus it came from |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `scripts/dev/token_economy.py` | `make ze-token-economy` |
| its tests run in CI | `make ze-test-pkg PKG=./scripts/dev` names the six tests |
| `ai/rules/context-economy.md` routed, not always-on | `grep context-economy ai/rules/TRIGGERS.md` hits, `grep context-economy ai/rules/CORE.md` does not |
| digests regenerated | `make ze-rules-condensed-check ze-rules-index-check ze-rules-lint` |
| skill mirrors in sync | `make ze-ai-check` |
| hook directives reach agents | `python3 scripts/dev/hook-fixture-check.py --only subagent-context` |
| `CORE.md` smaller, no directive lost | `make ze-rules-payload` before and after, plus the reviewed diff |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | transcript JSON is untrusted input to the tool: a malformed line is skipped, never raised as a traceback that hides the rest of the report |
| Path handling | the store path is read-only and derived from the project slug; the tool never writes into `~/.claude/` |
| Information leakage | the report prints token counts and session ids, never prompt text or file contents |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Cost per API call is the context size at that call, so every optimisation is either fewer calls or a smaller context. Trimming tool output does not appear in either term: Bash results already average 438 tokens.
- The review layer is the cheapest phase measured and prevents the second most expensive one. Cost pressure must never be applied there.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Trigger-routed rule plus a two-line hook injection | add the rule to the precedence ladder so `core_members` makes it always-on | the ladder's rungs 1 and 2 are irreversible action and correctness owed outside the repo. Context economy is neither, and claiming a rung to win preloading corrupts the ladder that resolves every other conflict |
| Reuse `tmp/session/session-state-<spec-stem>-<SID>.md` for handoff | a new `tmp/session/handoff-<spec>-<phase>.md` family | `_find_latest_state_for_spec` already resolves the newest state file for a spec across sessions. A second family would be layering |
| Work-package size decided at decomposition | a runtime turn budget enforced by a hook | subagents inherit the parent session id, so a hook cannot count one agent's calls apart from its siblings. A budget that cannot be measured would be a claim, not a gate |
| Size is an outcome of the `CORE.md` trim, never its gate | a byte or token target on `CORE.md` | a size gate rewards deleting directives, which is the one outcome the trim must not produce |

## Known Limitations
- The measurement reads a machine-local transcript store, so the figures are Thomas's machine's, not a repository property. A fresh checkout reports nothing and says so.
- No gate can prove an agent batched its independent calls or read a range instead of a file. The tool measures the aggregate after the fact; it does not enforce per-call discipline.
- The re-read figure counts tokens fed to the model, not money. Pricing is deliberately absent from the tool's output.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
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
