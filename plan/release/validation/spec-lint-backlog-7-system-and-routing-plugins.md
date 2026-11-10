# Spec: lint-backlog-7-system-and-routing-plugins

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | 19 findings of the full lint in system plugins and the OSPF and IS-IS plugins; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | 3/3 — closure: fixes in `383a697df9` and `e20b2ee5c8` |
| Handoff | - |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in the system
plugins `debug`, `help`, `redistribute` and `skills` (door
`ai/doors/system-plugins.md`) and in the OSPF and IS-IS plugins (door
`ai/doors/routing-protocols.md`). By linter: unused 6 (all
`redistribute/metrics_test.go`), goconst 3, gocritic 3, modernize 2,
goimports 2, misspell 1, godot 1, unparam 1. Line numbers are the CI run's.

| File | Line | Linter |
|------|------|--------|
| `internal/plugins/debug/debug.go` | 84 | modernize |
| `internal/plugins/debug/debug.go` | 211 | goconst |
| `internal/plugins/help/help_ai_test.go` | 6 | goimports |
| `internal/plugins/help/help_command_plugin_modeled_test.go` | 28 | goimports |
| `internal/plugins/isis/redistribute/consumer.go` | 215 | gocritic |
| `internal/plugins/isis/redistribute/source.go` | 55 | goconst |
| `internal/plugins/isis/register.go` | 59 | misspell |
| `internal/plugins/isis/server.go` | 545 | godot |
| `internal/plugins/ospf/redistribute/consumer.go` | 225 | gocritic |
| `internal/plugins/ospf/redistribute/source.go` | 48 | goconst |
| `internal/plugins/ospf/ri_show_test.go` | 90 | unparam |
| `internal/plugins/redistribute/metrics_test.go` | 87 | unused |
| `internal/plugins/redistribute/metrics_test.go` | 89 | unused |
| `internal/plugins/redistribute/metrics_test.go` | 90 | unused |
| `internal/plugins/redistribute/metrics_test.go` | 97 | unused |
| `internal/plugins/redistribute/metrics_test.go` | 99 | unused |
| `internal/plugins/redistribute/metrics_test.go` | 100 | unused |
| `internal/plugins/redistribute/reload_test.go` | 109 | gocritic |
| `internal/plugins/skills/main.go` | 217 | modernize |

## Required Reading

### Architecture Docs
- [x] `ai/doors/system-plugins.md` - edge plugins, ownership
  → Constraint: a plugin owns its whole surface; a goconst constant stays in the plugin's package
- [x] `ai/doors/routing-protocols.md` - OSPF and IS-IS
  → Constraint: OSPFv2 and OSPFv3 share code; the two redistribute consumers (`ospf`, `isis`) carry the same gocritic and goconst findings, so fix them the same way in both
  → Constraint: an RFC quote in a comment stays byte-exact (the `isis/register.go` misspell and the `isis/server.go` godot may sit in one)
- [x] `docs/contributing/ze-go-style.md` - "The rules a tool enforces"

**Key insights:**
- The six `unused` findings are one test helper block in `redistribute/metrics_test.go`: either a test lost its caller or the helper is dead; find which before deleting.

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/plugins/redistribute/metrics_test.go` - the unused block
- [x] `internal/plugins/ospf/redistribute/consumer.go` - the gocritic site mirrored in IS-IS
- [x] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- Redistribution behavior in both protocols; every test's assertions.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope | Applies: the evidence for this child |
| Style rule table | `docs/contributing/ze-go-style.md` | Names the fix each linter expects | Applies |
| Weakening judge | `ze-test-judge` | Approves a test change that could prove less | Applies if the unused block turns out to be a lost assertion that is restored or removed |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./internal/plugins/debug/... ./internal/plugins/help/... ./internal/plugins/redistribute/... ./internal/plugins/skills/... ./internal/plugins/ospf/... ./internal/plugins/isis/..."`

### Transformation Path
1. Read each finding; fix, or annotate with the reason the linter is wrong there.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | Source edits inside the plugins | No |

### Integration Points
- None beyond the packages listed.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The CI list is current for these packages | CI run of 2026-11-02 | Lines drifted | Source audit against the recorded findings, with the parent's instruction to run no CPU jobs | confirmed: the scoped lint of 2026-11-10 found only five later findings, fixed at closure |
| A-2 | The unused helpers in `redistribute/metrics_test.go` have no live caller in any build flavor | golangci-lint runs every flavor | A tagged test uses them | Package-wide source search including tagged files, and the change in 48c539ba35 | confirmed by source; independent judge approved removal before editing |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The unused block was meant to be asserted and a test silently stopped checking metrics | The helpers read a metric no test now checks | Restore the assertion it served (a judge reviews), rather than delete |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the plugins | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over the six plugin trees | Zero findings |
| AC-2 | The package tests with their tags (`ze_ospf`, `ze_isis` where gated) under `-race` | Pass |
| AC-3 | Any `//nolint` added | Names its linter and its reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | the changed plugins | AC-2 | |

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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-7-system-and-routing-plugins.md` |

## Implementation Steps

1. Scoped lint; record the current list.
2. The OSPF and IS-IS redistribute pair together; the rest per plugin.
3. Scoped lint zero; tests pass.

### Step 2 handoff — 2026-11-02

The source audit found the listed shapes still present, including both help
tests at their original plugin paths. No fresh lint, test, build or compile
was run: the parent serializes CPU jobs. The recorded CI list remains the red
baseline, not a claim about a fresh run.

| Finding | Source edit or remaining action |
|---------|---------------------------------|
| debug modernize and goconst | `optionAnswer` uses `errors.AsType`; `runDeleteModule` and `runSetTimeout` share `resultDisabled`, preserving the answer text |
| help goimports | Separate the repository imports from testify in both listed tests |
| paired consumer range copies | Both `Consumer.Rejudge` loops range keys, then read the map value once; locks, verdicts and writeback remain the same |
| paired source constants | Both source registrations reuse their package's existing `ConsumerName` for name, protocol and config root; IS-IS standard-library imports are grouped |
| IS-IS spelling and punctuation | `init` uses US spelling outside quoted text; `applyMetricStyle` ends its section reference with a period; no RFC quote was changed |
| OSPF helper parameter | `containsStubRouter` preserves the sole expected capability at all four callers, including `ri_functional_test.go`; assertions and fixtures are unchanged |
| redistribution range copy | `prefixes` indexes the input slice rather than copying each injected entry |
| skills modernize | `hasFlag` uses `slices.Contains` |
| six unused metrics declarations | Independent judge approved removal; removed only `nopCounterVec`, `nopCounter` and their four methods, preserving every `TestMetricsCadence` input and assertion |

Metrics diagnosis: `recordingRegistry.CounterVec` was changed in 48c539ba35
from `nopCounterVec` to `recordingCounterVec`. The latter records label sets,
and `TestMetricsCadence` asserts both the rejected-chain count and the zero
count for a source no rule imports. The old helper's only internal reference
is `nopCounterVec.With` returning `nopCounter`; no test calls either helper.
This is dead test scaffolding, not a lost metrics assertion. The source fix
removed the obsolete declarations after independent approval; suppressing
`unused` or adding a dummy caller would retain the cause. No test was removed
or weakened. The independent approval for `redistribute.TestMetricsCadence`
was read before applying the removal; its exact reason must accompany the
commit's `Weakening-approved` trailer.

The author read every changed hunk; the scoped `git diff --check` reported no
whitespace errors. AC-1 and AC-2 remain unverified. AC-3 has no new annotation
to judge. The parent owes scoped lint over
the six plugin trees and race tests for debug, help (including `ze_vrrp`),
redistribute, skills, OSPF and IS-IS (including their redistribution packages
and feature tags). Citation-anchor checks and the independent Review Gate
remain owed. These style-only changes alter no documented operator or wire
behavior, so no architecture or guide prose changes are required.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Metrics assertions | Establish whether unused helpers lost their callers. Restore lost assertions with judge review rather than remove coverage (A-2, R-1). |
| Paired consumers | OSPF and IS-IS receive equivalent redistribution fixes, constants remain plugin-owned, and quoted RFC text stays byte-exact. |
| Routing evidence | Six trees are lint-clean, tagged race tests pass, and annotations state reasons (AC-1–3). |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Six plugin scopes cleared | Record zero scoped lint findings and tagged race-test results (AC-1–2). |
| Preserved metrics coverage | Resolve unused-helper callers and cite judge evidence for assertion changes, with reasoned annotations (A-2, R-1, AC-3). |

### Security Review Checklist

| Concern | What to verify |
|---------|----------------|
| Redistribution decisions | Paired OSPF and IS-IS edits preserve route selection and source values. |
| Lost failure evidence | Unused-helper removal does not remove metrics assertions, and comment fixes preserve byte-exact RFC quotes. |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope "<the six trees>"` zero; `./le job run label plugins command go test -race -tags <tags> <changed packages>`, the scoped subset of the `./le test unit plugins` group. Full-run evidence belongs to the umbrella's CI run.

## Implementation Summary

### What Was Implemented
- `383a697df9`: `debug` (`optionAnswer` uses `errors.AsType`; `resultDisabled` shared by `runDeleteModule` and `runSetTimeout`), `help` (import groups in both tests), both `Consumer.Rejudge` loops range keys and read the value once, both `Register{OSPF,ISIS}Sources` reuse `ConsumerName`, the IS-IS `init` comment spelling and `applyMetricStyle` full stop, `redistribute` `prefixes` indexes the slice, `skills` `hasFlag` uses `slices.Contains`, `ospf/gr.go` comment alignment.
- `e20b2ee5c8`: removed the uncalled `nopCounterVec`/`nopCounter` from `redistribute/metrics_test.go`, and replaced `containsStr` by `containsStubRouter` in `ospf/ri_show_test.go` (five callers, all for `"stub-router"`), each with its judge line.
- Later in-scope commits (`1701a07631`, `e396a2e360`, `c8e036be19`) were read at closure for new findings; the scoped lint below judges them.
- Closure fixes for five findings later commits added (first scoped lint, 2026-11-10, log `tmp/session/2026-11-09-222501/scratch/job-lint7-59c3994c.log`): `help/help_command_plugin_test.go` and `help/render_exit_test.go` import groups; `ospf/ipsec_install.go` `ipsecInstaller.installLocked` policy loop indexes the slice as its SA loop does (same values installed and recorded); `help/help_ai.go` `printRIBPipeline` and `printCommonErrors` (dupl, both directions) carry `//nolint:dupl` with the reason. The two are different authored texts (the `show bgp rib` grammar, and error-to-fix pairs) that share only the shape of 29 `rw.Line` calls, so there is no one fact to share; a table form would match dupl the same way.

### Bugs Found/Fixed
- None. Lint-only; behavior is unchanged.

### Documentation Updates
- None. The doc anchors that name a changed symbol (`docs/guide/redistribution.md` and `docs/guide/ospf.md` on `Rejudge`, `docs/architecture/isis/isis-11-redistribution.md` on `RegisterISISSources` and `Rejudge`, `docs/guide/isis.md` and `docs/architecture/isis/isis-6-lsdb.md` on `applyMetricStyle`, `docs/guide/command-reference.md` and `docs/guide/debugging-tools.md` on `runDeleteModule`) describe behavior these edits preserve. `./le spec citation anchors` exited 0 with no finding.

### Deviations from Plan
- None.

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 19 CI findings in the six trees | Done | `383a697df9`, `e20b2ee5c8` | Each Task row maps to a hunk in "Step 2 handoff" |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Scoped lint, Pre-Commit Verification | |
| AC-2 | Done | Race runs, Pre-Commit Verification | |
| AC-3 | Done | `git show 383a697df9 e20b2ee5c8` carries no `nolint`; the closure adds two `//nolint:dupl // <reason>` in `help/help_ai.go` | Both name the linter and the reason |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The existing package tests | Done | the six trees | Assertions unchanged; two helper removals judge-approved |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| The Task table's files | Done | `ospf/gr.go` and `ospf/ri_functional_test.go` also changed: alignment and the renamed helper |

### Audit Summary
- **Total items:** 19 findings, 3 ACs
- **Done:** all
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The six plugin trees carry no full-lint finding | lint run | Scoped `./le go lint run scope` over the six trees, 2026-11-10 after the closure fixes: exit 0, host 0 issues, GOOS=linux integration 0 issues, darwin 0 issues (log `tmp/session/2026-11-09-222501/scratch/job-lint7-eda2ec37.log`) |
| Behavior and test strength preserved | race unit tests, judge | Race run (2026-11-10, main thread, `go test -race -count=1` with `ze_core` and every `ze_*` tag in `feature-gates.txt`, `ze_vrrp` and `ze_bgp` included, over the six trees): exit 0, 34 packages ok; `Weakening-approved: ospf.containsStr` and `Test-judge:` for `metrics_test.go` in `e20b2ee5c8` |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | | |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/lint-backlog-7-system-and-routing-plugins-49da3d3d-44f5-443f-bc7a-73b9535a17f9.md` (CLEAN, 19 files) |
| `./le spec review check` | clean: `review_gate: OK (19 code files, clean, hashes match ...)` |
| Rounds | 2 (round 2 reviews the four closure-fix files after the first scoped lint) |
| Reviewer lenses used | logic and removed-behavior, style (`ze-go-style.md`; no `panic` added), lint-dodge, RFC-quote integrity, doc anchors, security |

Pre-checks: `./le repo check` reports four issues and `./le test weakened check` one, all in other subsystems' files and none in the six trees.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | No BLOCKER or ISSUE | - | - |

NOTEs: (1) both `Rejudge` loops still copy one `offer` per key; the copy is inherent, because a map value is not addressable and the loop writes the new verdict back, so this is the explicit form of the same copy, not a dodge. (2) `383a697df9` alone does not compile its OSPF tests: `ri_functional_test.go` calls `containsStubRouter`, which `e20b2ee5c8` defines. HEAD is consistent; history only. (3) The `isis/register.go` spelling and `isis/server.go` full stop sit in Ze's own prose, not in an RFC quote.

### Run 2
Scope: the closure fixes in `help/help_ai.go`, `help/help_command_plugin_test.go`, `help/render_exit_test.go` and `ospf/ipsec_install.go`. 0 BLOCKER, 0 ISSUE. The policy loop installs and records the same `SPParams` values in the same order, takes the same error path, and mirrors the SA loop above it; the name `policy` shadows no import in the file. The import edits only add the blank line before the Ze group, and no assertion changed. NOTE: the two `//nolint:dupl` lines suppress a structural match between two different authored texts. The finding is real only as a shape, and no shared function can carry two texts.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| The 15 changed files of `383a697df9` and `e20b2ee5c8` in scope | Yes | `ls` of all 15 paths printed each |
| The four closure-fix files | Yes | `git diff --stat` names `help/help_ai.go`, `help/help_command_plugin_test.go`, `help/render_exit_test.go`, `ospf/ipsec_install.go` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Zero scoped findings | First run (`job-lint7-59c3994c.log`) found 5, fixed at closure; rerun `job-lint7-eda2ec37.log`: exit 0, 0 issues on host, linux integration and darwin |
| AC-2 | Race tests pass | 2026-11-10 `./le job run label lint7-race` with `go test -race -count=1 -tags "ze_core <every ze_* tag>"` over the six trees: exit 0, 34 packages ok (help 38.1s with its `ze_vrrp` and `ze_bgp` files built, ospf 53.7s, isis 61.5s, isis/lsdb 205s). Log `tmp/session/2026-11-09-222501/scratch/job-lint7-race-8a8cb562.log`. After the closure fixes, 2026-11-10 `go test -race -count=1 -tags "<every ze_* tag>"` over `./internal/plugins/help/...` and `./internal/plugins/ospf/...`: exit 0, 17 packages ok, log `tmp/session/2026-11-09-222501/scratch/job-lint7-race2-c5f69355.log` |
| AC-3 | Every `//nolint` added names its linter and reason | `git show 383a697df9 e20b2ee5c8` carries no `nolint`; `git diff -- internal/plugins/help/help_ai.go` adds two lines of the form `//nolint:dupl // authored text, ...` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le go lint run scope` | none (lint) | Yes: `job-lint7-eda2ec37.log`, exit 0 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Source audit at closure plus the scoped lint: the CI shapes are gone, and the five findings later commits added were fixed (`job-lint7-eda2ec37.log`, 0 issues) |
| A-2 | confirmed | `grep -rn nopCounter internal/plugins/` empty; judge line in `e20b2ee5c8` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Anchors on changed symbols | `git grep` over `docs/` for `Rejudge`, `RegisterISISSources`, `applyMetricStyle`, `runDeleteModule`: claims describe preserved behavior | Yes |
