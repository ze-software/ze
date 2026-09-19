# Spec: harness-fail-open-guard-backlog

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-08-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Hold the nine inherited harness and tooling observations below. Their current
dispositions differ; they are not nine confirmed live defects. Each was homed at
`spec-fixit-test-harness-fail-open-guards` while that spec collected this class.
That spec closed on 2026-08-14 having shipped its own four guards, and
`ai/rules/planning.md` requires every row naming it as Destination to be
resolved inside the closing commit. This file is that destination.

**Nothing here is commissioned.** The rows keep Status `deferred` and name this
file so no record dangles. Thomas has not scheduled the work.

This file owns the retained observations. The destinations surveyed on
2026-08-14 were proposals, not assignments to sibling specs. Reconciliation
against the current producers on 2026-09-19 found retired sources and changed
contracts, recorded below. No row commissions implementation or expands a
sibling's scope.

Rows D and I were re-surveyed on 2026-08-17, when the spec they named closed with
no code change.

| # | Current assessment | Historical destination proposal | Remaining action |
|---|--------------------|---------------------------------|------------------|
| A | The inaccurate `wait_peer_eor_sent` docstring belonged to the retired `test/scripts/ze_api.py`, which has no successor at that path | Journal class `reference-checked-claim-unchecked` | Retain as historical evidence. There is no live docstring to repair; a current misleading claim needs its own source evidence |
| B | The Python `docker_rm` helpers were replaced by the shared native lab. `Docker.RemoveContainer` (`internal/le/interoplab/docker.go`) returns removal errors except the name-specific absent-container response; `Lab.preClean` (`lab.go`) returns cleanup failures | A separate Docker-removal contract spec | The recorded swallow-and-continue mechanism is absent from these current producers. No new spec is owed for the retired helpers; runtime proof is not claimed here |
| C | `ospf-sr-frr` now uses `opWaitContains` for MPLS label `16100` (`internal/le/interoplab/bgp/check_extras.go`). `waitContains` returns the wait error with the last output (`check_engine.go`) | `plan/pre-release/spec-interop-suite-red.md` | The old Python check's `log_info` success branch is retired. Do not add that stale defect to the sibling; assess any remaining interop failure against the native checker |
| D | `auditDiff` (`internal/le/testweakened/audit.go`) reads tracked diffs and explicitly skips added files. It compares old and new test units; the `test-relax:` token scanner described in August is retired | A combined untracked/Python audit spec with I | Decide whether a new-test assertion-quality check is wanted. An untracked file has no prior assertion to weaken, so adding it to this diff audit is not a settled repair |
| E | `checkHookNames` and `nameLintFiles` are now in `internal/le/doc/check/names.go`. The three-file population still excludes `ai/rules/planning.md` | Journal class `reference-checked-claim-unchecked` | Retain the population-gap investigation here. Re-establish a current dead-name witness before commissioning a widened check; the inherited `c_model_phase` example is dated evidence |
| F | `.claude/hooks/pretool-writeedit.py` and its `c_check_existing_tests` stub are retired; no matching native function was found under `internal/le/hookruntime` | A registered-empty-check spec | The original stub has no live repair target. Reintroducing a warning would require a decision about its intended contract |
| G | Point frontmatter has a closed five-field set, `kind`, `level`, `stage`, `rationale`, `excepted-by` (`pointKeys` in `internal/le/rules/points.go`); `title` remains absent | `plan/spec-rules-situation-index.md` | Owner decision: add an authored point title, or keep the existing slug/body inventory. The historical population counts must be remeasured if commissioned |
| H | A point-body digest on each `Binding` remains a proposed change to the binding contract | `plan/spec-rules-situation-index.md`, weakly | Owner decision: require digest-backed re-affirmation on body changes, or retain review-based binding maintenance. No sibling scope is changed here |
| I | `isTestPath` (`internal/le/testweakened/testweakened.go`) now calls `isPythonTest`; `detector.go` accepts `test_*.py` and `*_test.py` | A combined untracked/Python audit spec with D | The claim that Python gets zero signal is superseded. Broader Python populations need a current witness and an explicit contract before further work is scheduled |

## Provenance

Written 2026-08-14 when `spec-fixit-test-harness-fail-open-guards` closed. Nine
live rows in four shards named that spec as their Destination:
the retired deferral shard "wire-edit-4-api-origin-deferred-bird-interop" (A, B),
the retired deferral shard "fixit-ospf-sr-missing-label-passes" (C),
the retired deferral shard "fixit-firewall-concurrency-deadlock" (D) and
the retired deferral shard "rules-as-points" (E, F, G, H, I). None was an acceptance
criterion of that spec. Each was parked there because it collected this class,
and the class outlived the four guards that closed it.

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. These are historical observations; the table above
     states their current disposition and supersedes obsolete repair instructions. -->

### From `fixit-firewall-concurrency-deadlock.md`, 2026-08-07

Deferred by spec-fixit-firewall-concurrency-deadlock review round 3.

the retired `scripts/dev/audit-test-relaxation.py` (current producer: `internal/le/testweakened/audit.go`) cannot see a `test-relax:` token in an UNTRACKED test file, so the audit is blind on exactly the files where a weakened assertion is easiest to introduce: brand-new ones. The producer is the script's own diff source, `git diff` against HEAD, which lists tracked modifications only; an untracked file has no HEAD side and never appears. Measured 2026-08-07: `test/plugin/firewall-metrics-registered.ci` carries two `test-relax:` tokens and the audit reported `1 finding(s)`, that one being another session's `gr_egress_test.go`, with this file named nowhere. `/ze-review` step 0 runs this script, so a review of a new test file silently audits nothing. Fix shape: include untracked test files (`git ls-files --others --exclude-standard`) and treat their whole content as added

### From `rules-as-points.md`, 2026-08-07

Deferred by spec-rules-as-points.

Teach `check_hook_names` in the retired `scripts/dev/check_doc_links.py` (current producer: `internal/le/doc/check/links.go`) to see a dead check name in a rule file, then delete the stale `c_model_phase` prose it would then report. The function was REMOVED (`ai/rules/points/planning/work-phases/implementation-carries-no-model-requirement.md` says so, and it is in no dispatcher), yet `ai/rules/points/planning/work-phases/how-the-model-phase-gates-work-and-where-they-stop.md` still describes it as a live BLOCKING gate and the Hook-to-Rule Mapping table still carries its row. The guard misses BOTH: `NAME_LINT_FILES` covers `ai/rules/repo-maintenance.md`, where the name sits UNBACKTICKED in a table cell, and does not cover `ai/rules/planning.md`, where it is backticked

### From `rules-as-points.md`, 2026-08-07

Deferred by spec-rules-as-points.

Add an authored `title:` field to the point frontmatter, for the roughly 800 instruction-bearing points that no check binds, so `gate_map --ungated` (the retired `scripts/dev/rules_points.py` (current producer: `internal/le/rules/points.go`)) prints a scannable inventory. The 47 BOUND slugs were re-authored in phase 5 because a hook comment is the one place a human reads a slug raw. The other 1541 are read only through generated output, so re-authoring their ids would churn 800 filenames, 27 manifests and every `git log` trail for no reader. A `title:` beside `kind:` and `level:` gets the same result and touches no id

### From `rules-as-points.md`, 2026-08-07

Deferred by spec-rules-as-points.

Decide if a `Binding` must carry a digest of the point BODY it names, in the shape of `check_audit_freshness` in the retired `scripts/dev/rfc_requirements.py` (current producer: `internal/le/rfc/rfc.go`): the digest sits on the BINDING, never on the id, and a body rewritten under the same slug then leaves the binding stale until a reader re-affirms it. Today `gate_map` in the retired `scripts/dev/rules_points.py` (current producer: `internal/le/rules/points.go`) joins on `Binding.ref` alone, so a reword keeps its gate. The third of the three problems in the spec Task table is therefore answered in review rather than by a machine, and `docs/contributing/rule-authoring.md` plus `ai/rules/points/rule-format/rationale/one-instruction-one-file.md` now say so
