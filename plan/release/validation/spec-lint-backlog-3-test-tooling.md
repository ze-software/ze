# Spec: lint-backlog-3-test-tooling

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | 58 findings of the full lint in the test harness, `le` and the chaos tooling; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in the test
harness and `le` (door `ai/doors/le-tooling.md`), the chaos tooling under
`internal/chaos` and `tools/freertr-entrypoint` (both covered by
`ai/doors/system.md` alone): `internal/test/fixture` 23, `internal/le` 18,
`internal/chaos` 10, `internal/test/runner` 4, `internal/test/plugins/fakefilter`
2, `tools/freertr-entrypoint` 2, `internal/test/userns` 1. By linter: goconst
21, exhaustive 14, gosec 12, errcheck 3, misspell 3, gocritic 3, unused 1,
staticcheck 1. The eight locations 752d8f19ef cleared
(`internal/le/go/staticcheck/judge.go`, `internal/le/hookruntime/timestamp*.go`,
`internal/le/test/qemu`) are not listed; this child confirms them with its
scoped run (umbrella A-2). Line numbers are the CI run's.

| File | Line | Linter |
|------|------|--------|
| `internal/chaos/engine/action_params.go` | 241 | exhaustive |
| `internal/chaos/inprocess/chaos.go` | 151 | exhaustive |
| `internal/chaos/inprocess/runner_test.go` | 200 | exhaustive |
| `internal/chaos/inprocess/runner_test.go` | 352 | exhaustive |
| `internal/chaos/inprocess/runner_test.go` | 701 | exhaustive |
| `internal/chaos/orchestrator/types.go` | 110 | exhaustive |
| `internal/chaos/peer/receiver_test.go` | 163 | exhaustive |
| `internal/chaos/peer/simulator_test.go` | 204 | exhaustive |
| `internal/chaos/peer/simulator_test.go` | 272 | exhaustive |
| `internal/chaos/report/metrics.go` | 75 | exhaustive |
| `internal/le/go/extract/goextract.go` | 99 | gosec |
| `internal/le/go/extract/goextract.go` | 202 | gosec |
| `internal/le/go/extract/goextract.go` | 205 | gosec |
| `internal/le/go/stdpatch/gen/stdpatch.go` | 105 | gosec |
| `internal/le/go/stdpatch/gen/stdpatch.go` | 108 | gosec |
| `internal/le/go/stdpatch/gen/tree.go` | 95 | gosec |
| `internal/le/go/stdpatch/gen/tree.go` | 245 | gosec |
| `internal/le/go/stdpatch/gen/tree.go` | 271 | gosec |
| `internal/le/interoplab/bgp/prepare_inject.go` | 92 | exhaustive |
| `internal/le/interoplab/bgp/speaker_parsed_empty_mp.go` | 148 | gosec |
| `internal/le/interoplab/ipsec/ipsec.go` | 451 | unused |
| `internal/le/interoplab/ipsec/mobike_netns_linux_test.go` | 52 | staticcheck |
| `internal/le/setup/shims.go` | 409 | gosec |
| `internal/le/test/deployment/gokrazyl2tp.go` | 281 | gosec |
| `internal/le/test/functional/binaries.go` | 204 | goconst |
| `internal/le/workflowcheck/workflowcheck_test.go` | 578 | misspell |
| `internal/test/fixture/netfilter_fixture_firewall.go` | 409 | gocritic |
| `internal/test/fixture/netfilter_fixture_firewall.go` | 410 | gocritic |
| `internal/test/fixture/plugin_fixture_01_answer.go` | 811 | goconst |
| `internal/test/fixture/plugin_fixture_06_decoder.go` | 32 | goconst |
| `internal/test/fixture/plugin_fixture_08_peer_wait_test.go` | 120 | gocritic |
| `internal/test/fixture/plugin_fixture_10_process.go` | 650 | goconst |
| `internal/test/fixture/plugin_fixture_filter_thirdparty.go` | 34 | goconst |
| `internal/test/fixture/plugin_shell_extra_1.go` | 275 | goconst |
| `internal/test/fixture/plugin_shell_extra_1_auth.go` | 461 | goconst |
| `internal/test/fixture/policy_final_alternate.go` | 151 | goconst |
| `internal/test/fixture/policy_final_reload.go` | 22 | goconst |
| `internal/test/fixture/register_sflow_agent_address_linux.go` | 31 | goconst |
| `internal/test/fixture/ui_fixture_le_checks_answers.go` | 631 | goconst |
| `internal/test/fixture/ui_fixture_le_checks_answers.go` | 632 | goconst |
| `internal/test/fixture/ui_fixture_le_evidence_answers.go` | 271 | goconst |
| `internal/test/fixture/ui_fixture_le_exec_cache_env.go` | 211 | goconst |
| `internal/test/fixture/ui_fixture_le_exit_codes.go` | 63 | goconst |
| `internal/test/fixture/ui_fixture_le_inventory_answers.go` | 270 | goconst |
| `internal/test/fixture/ui_fixture_le_placement_answers.go` | 117 | goconst |
| `internal/test/fixture/ui_fixture_le_rfc_answers.go` | 252 | goconst |
| `internal/test/fixture/ui_fixture_le_rfc_answers.go` | 296 | goconst |
| `internal/test/fixture/ui_fixture_le_store_trim_answers.go` | 121 | goconst |
| `internal/test/fixture/ui_fixture_le_store_trim_answers.go` | 122 | goconst |
| `internal/test/plugins/fakefilter/fakefilter.go` | 311 | misspell |
| `internal/test/plugins/fakefilter/fakefilter.go` | 313 | misspell |
| `internal/test/runner/display.go` | 280 | exhaustive |
| `internal/test/runner/record_collection.go` | 98 | exhaustive |
| `internal/test/runner/runner_stop_test.go` | 196 | errcheck |
| `internal/test/runner/stress.go` | 21 | exhaustive |
| `internal/test/userns/userns_linux.go` | 260 | gosec |
| `tools/freertr-entrypoint/main.go` | 175 | errcheck |
| `tools/freertr-entrypoint/main.go` | 179 | errcheck |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/le-tooling.md` - le, the hooks and the test trees
  → Constraint: every concurrent session runs the hooks and `./le`; a change to `internal/le/hookruntime` or the launcher can block them all
  → Constraint: a bare `go test` omits the feature tags; run each package through `./le job run label unit-pkg command go test -tags ...`
- [ ] `docs/contributing/ze-go-style.md` - "The rules a tool enforces", the `exhaustive` C/O/P classification
  → Constraint: gosec G304/G204 on a trusted path takes `//nolint:gosec // <why the path or argv is trusted>`, as 752d8f19ef did
- [ ] `ai/META-DECISIONS.md` - "Test changes"
  → Constraint: a fixture change that could lower what a test proves goes to `ze-test-judge`; a goconst rename does not, an assertion change does

**Key insights:**
- Most fixture findings are goconst on repeated answer strings; one package-level constant per string, in the fixture package.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/go/stdpatch/gen/tree.go` - gosec sites over the patched GOROOT tree
- [ ] `internal/test/runner/display.go` - an enum switch `exhaustive` reports
- [ ] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- Every test's assertions and every `le` command's answer.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope | Applies: the evidence for this child |
| The precedent fix | 752d8f19ef | Cleared this session's lint in staticcheck, hookruntime and qemu with reasoned gosec nolints and goconst constants | Extends: the same treatment for the remaining `internal/le` findings |
| Weakening judge | `./le test weakened approve`, `ze-test-judge` | Approves a test change that could prove less | Applies only to an assertion change, not to a mechanical lint fix |
| Lint contract journal rows | `plan/journal/lint-contract-not-applied.md`, rows owned by `plan/post-release/engineering/spec-tooling-one-off-defects.md` | Record tooling lint findings | Applies: a finding here that a row names is marked `fixed:` in the same commit |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./internal/test/... ./internal/le/... ./internal/chaos/... ./tools/freertr-entrypoint/..."`

### Transformation Path
1. Read each finding; fix, or annotate with the reason the linter is wrong there.
2. A real defect (an ignored error in the runner or the entrypoint) gets a red test first.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | Source edits inside the packages | No |

### Integration Points
- None beyond the packages listed.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The CI list is current for these packages | CI run of 2026-11-02 | Lines drifted | The scoped run before the first edit | unvalidated |
| A-2 | 752d8f19ef cleared its eight locations | Its body: a scoped lint over the three packages reports 0 issues | They reappear in this scope | This child's scoped run covers `internal/le/...` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A hook-source edit breaks the hooks for every session | `./le ai hooks unit` red; the stored digests refuse | Run `./le ai hooks unit` after any `internal/le/hookruntime` edit; re-seal the digests through their own procedure |
| R-2 | The fixture package is edited by other sessions (the peer-wait backlog) | Conflicting hunks in `internal/test/fixture` | Commit only this child's hunks |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the packages | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over `internal/test`, `internal/le`, `internal/chaos`, `tools/freertr-entrypoint` | Zero findings |
| AC-2 | The package tests of each changed package, with their tags | Pass |
| AC-3 | Any `//nolint` added | Names its linter and its reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | each changed package | AC-2 | |

## Files to Modify
- The files in the Task table.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG, CLI, env, doctor, metrics, BGP family surface | N-A | Lint fixes; no surface changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1-15, 17 | Feature, config, CLI, API, plugin, wire, SDK, RFC, test infrastructure, comparison, architecture, metadata, metrics, inventory, examples | No | Lint fixes change no documented behavior |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-3-test-tooling.md` |

## Implementation Steps

1. Scoped lint; record the current list.
2. `internal/le` and `tools/`: gosec and errcheck, reasoned.
3. `internal/test`: goconst, gocritic, exhaustive, errcheck.
4. `internal/chaos`: exhaustive, classified C/O/P.
5. Scoped lint zero; package tests pass.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Tooling safety | Each gosec annotation traces the trusted path or argv, and real ignored-error defects have red-first proof. |
| Test and hook contracts | Fixture assertions remain intact. Hook-source changes carry hook-unit evidence and the required digest procedure (R-1). |
| Tooling evidence | Scoped lint includes the eight excluded locations (A-2), changed-package tests use their tags, and annotations state reasons (AC-1–3). |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope "<the packages>"` zero; `./le job run label unit-pkg command go test -tags <tags> <changed packages>`; `./le ai hooks unit` if a hook source changed. Full-run evidence belongs to the umbrella's CI run.
