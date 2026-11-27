# Spec: dropped-error-0-umbrella

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P1 |
| Release reason | Shipped release code catches 276 errors (281 tree-wide, down from 328 on 2026-11-02) whose only record is a debug or info line that the default WARN level hides, so a failed operation reads as done and leaves no trace; this umbrella owns the release-wide zero across shipped paths |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

b2448d44e7 added a fifth refuse-by-default shape, `error-dropped-below-warn`
(`internal/le/arch/refusebydefault/dropped.go`, `droppedError`): a branch on
`err != nil` (or the else of `err == nil`) whose only record is a `Debug` or
`Info` log call, and which then falls through, continues, breaks or returns no
error. `./le arch refuse-by-default check` refuses it on changed lines.

Reconciled against HEAD on 2026-11-26, `./le arch refuse-by-default report`
lists **281 unannotated `error-dropped-below-warn` instances** (down from 328 on
2026-11-02) and 4 annotated instances:

- **276 instances** sit on shipped release paths across children 1–5 and 7–9
  (child 6 closed on 2026-11-15 with 0 remaining findings).
- **2 instances** sit in opt-in unreleased MOBIKE
  (`internal/component/ike/engine/mobike.go:590` and `:607`), which the owner
  ruling of 2026-11-23 (`58c46e854f`) moved out of first-release fixes into
  `plan/post-release/features/spec-ipsec-11-mobike.md` (Known Issues #3).
- **3 instances** sit in unshipped development tooling
  (`internal/chaos/web/handlers.go` 2, `internal/test/runner/peer_contract.go` 1),
  owned by child 10 (`plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md`).

Two blocking rules govern every child's resolution:

1. **Dropped-error rule** (`ai/rules/points/go-standards/directives/log-a-dropped-error-at-warn-or-above.md`):
   "An error that is caught and not returned MUST be logged at WARN or above, and
   MUST NOT be logged at debug or info." and "Dropping an error is correct only
   where going on is correct; a WARN does not make a default correct."
2. **Doctor-coverage commit gate** (`ai/rules/user-facing-errors.md`,
   `docs/architecture/doctor-and-health-checks.md` "Every problem site names a
   doctor code", `internal/le/arch/doctorcoverage`): every newly added or changed
   `Warn` or `Error` log line (including `WarnContext`, `ErrorContext`,
   `Log`/`LogAttrs` at `slog.LevelWarn`/`slog.LevelError`, and forwarding
   wrappers) and every report-bus raise under `internal/component`,
   `internal/plugins`, or `cmd/ze` MUST carry a literal or constant `code`
   attribute naming a doctor code declared by a registered doctor check (or a
   transient `diagnostic.EventCode` registered with `diagnostic.RegisterEventCode`
   and `diagnostic.Register` metadata so `recent-events` declares it), and a
   transient problem MUST also be raised on the report bus (`internal/core/report`)
   under that code. The `code` key is reserved for doctor codes; no comment
   exempts a site. `./le arch doctor-coverage check` and `./le commit create`
   refuse any added `Warn` without a covered code.

Every child resolves each instance in this order:

| Order | When | Resolution |
|-------|------|------------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid |
| 2 | Going on is right and an operator can act | Log at `Warn` with what failed, why it matters, and the exact fix (`ai/rules/user-facing-errors.md`), carrying a literal or constant `code` attribute naming a declared doctor code (from the area's `plan/release/validation/spec-doctor-coverage-*` map, or by registering the check / `diagnostic.EventCode` with `diagnostic.Register` and `diagnostic.RegisterEventCode` in the same change) under `internal/component`, `internal/plugins`, and `cmd/ze`, plus a report-bus raise for transient failures; on a per-packet or per-route hot path, count it in the subsystem's existing metrics and `Warn` once per condition |
| 3 | Going on is right and nobody can act (a teardown close, a reply to a client already gone) | Annotate the branch with `// refuse-by-default: guards <Symbol> -- <why>` on the shape's line or the line above, naming a visible dependent symbol (never the enclosing function) |

**Goal:** `./le arch refuse-by-default report` lists zero unannotated
`error-dropped-below-warn` findings across every shipped release path
(children 1–9), with every newly added `Warn` under the doctor-coverage site
roots carrying a declared doctor code and passing `./le arch doctor-coverage check`.

The work needs ten doors, so it is split by door (`ai/META-DECISIONS.md`: "a
spec whose edits need many doors is split into narrower child specs"). Each
child owns the instances under its door's base paths and the zero for them:

| Child | Door | Status | Instances (2026-11-26) | Bucket |
|-------|------|--------|------------------------|--------|
| `spec-dropped-error-1-bgp-engine` (closed 2026-11-27) | `bgp-engine.md` | closed | 0 (was 48) | release fixes |
| `spec-dropped-error-2-bgp-plugins` (closed 2026-11-27) | `bgp-plugins.md` | closed | 0 (was 63) | release fixes |
| `spec-dropped-error-3-access-security` (closed 2026-11-27) | `access-security.md` | closed | 0 (was 60; excludes 2 `mobike.go` sites moved to `spec-ipsec-11-mobike.md` on 2026-11-23) | release fixes |
| `spec-dropped-error-4-system-plugins` (closed 2026-11-27) | `system-plugins.md` | closed | 0 (was 50) | release fixes |
| `spec-dropped-error-5-forwarding` (closed 2026-11-27) | `forwarding.md` | closed | 0 (was 36) | release fixes |
| `spec-dropped-error-6-routing-protocols` (closed 2026-11-15 in `f126ead44e`) | `routing-protocols.md` | closed | 0 (was 28) | release fixes |
| `spec-dropped-error-7-plugin-infra` (closed 2026-11-27) | `plugin-infra.md` | closed | 0 (was 20) | release fixes |
| `spec-dropped-error-8-core-hub-appliance` (closed 2026-11-27) | `core.md`, `hub-engine.md`, `appliance.md` | closed | 0 (was 11: core 7, hub-engine 2, appliance 2) | release fixes |
| `spec-dropped-error-9-api-cli-config` (closed 2026-11-27) | `api-surfaces.md`, `commands-cli.md`, `config-yang.md` | closed | 0 (was 8: api-surfaces 6, commands-cli 1, config-yang 1) | release fixes |
| `plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md` | `system.md` (`internal/chaos`), `le-tooling.md` (`internal/test`) | `ready` | 3 (was 3) | post-release engineering: no shipped binary links them |

The remaining 2 tree-wide instances (`internal/component/ike/engine/mobike.go:590`
and `:607`) belong to `plan/post-release/features/spec-ipsec-11-mobike.md`
(Known Issues #3, moved out of child 3 on 2026-11-23): 276 release + 3 dev
tooling + 2 post-release MOBIKE = 281 total at HEAD on 2026-11-26.

A child takes the door whose `door-base` is the longest prefix of each
instance's path (`ai/doors/README.md`), so `internal/component/bgp/plugins/`
is child 2 and `internal/component/config/system/` is child 8.

The umbrella itself changes no code. It holds the resolution policy every child
applies, owns the release-wide acceptance, and closes after the last release
child (children 1–5 and 7–9).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform boundaries, process boundary, and operator message contract
  → Constraint: every operator-facing message names what failed, why it matters, and the exact fix across every interface; a forked plugin cannot reach daemon-local stores directly
- [ ] `docs/contributing/ze-go-style.md` - "A zero value is never an answer" and "Every error is handled"
  → Constraint: the three resolutions are return the error, count it, or log it at WARN; the annotation is the one escape and names a visible dependent guard
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: a WARN line an operator reads says what went wrong in their terms, why it matters, and the exact next step; every `Warn`/`Error` log line and report-bus raise under `internal/component`, `internal/plugins`, or `cmd/ze` MUST name a declared doctor code under its `code` attribute, and a transient problem MUST also be raised on the report bus under that code
- [ ] `docs/architecture/doctor-and-health-checks.md` - "Every problem site names a doctor code" and "Recent events: the report bus in show doctor"
  → Constraint: `./le arch doctor-coverage check` and `./le commit create` refuse any newly added `Warn`/`Error` line without a literal or constant declared doctor code under `code`; transient classes register their code with `diagnostic.Register` + `diagnostic.RegisterEventCode` and raise on `internal/core/report`
- [ ] `ai/rules/points/go-standards/directives/log-a-dropped-error-at-warn-or-above.md`
  → Constraint: a WARN does not make a default correct; an instance where going on is wrong is a behavior fix, not a log-level change
- [ ] `ai/doors/le-tooling.md` - the door for the `refuse-by-default` and `doctor-coverage` checks
  → Constraint: both checks live under `internal/le`; product code never imports `internal/le`, and scoped package tests run through `./le test unit pkg path <dir>/`

**Key insights:**
- The report is the population. Line numbers and counts drift; every child re-runs `./le arch refuse-by-default report` before its first edit rather than relying on a stored list.
- A branch that records the error (an assignment that mentions it, an increment) or hands it on is not this shape (`recordsMiss`, `mentions` in `refusebydefault.go`).
- Raising a `Debug` or `Info` call to `Warn` under `internal/component`, `internal/plugins`, or `cmd/ze` creates a newly added problem site judged by `./le arch doctor-coverage check` on commit; every such `Warn` must carry a declared doctor `code` from day one.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - `droppedError`, `caughtError`, `belowWarn` (`Debug`, `Info`, their `Context` forms, `Log` and `LogAttrs` at `LevelDebug` or `LevelInfo`), `drops`
- [ ] `internal/le/arch/refusebydefault/annotation.go` - `// refuse-by-default: guards <Symbol> -- <why>`, on the shape's line or the line above, `<Symbol>` visible in the file or sibling package files and distinct from the enclosing function
- [ ] `internal/le/arch/refusebydefault/refusebydefault.go` - `recordsMiss`, `mentions`, `ScanFile`, `RuleErrorDroppedBelowWarn`
- [ ] `internal/le/arch/doctorcoverage/doctorcoverage.go` - `Scan`, `siteRoots` (`internal/component`, `internal/plugins`, `cmd/ze`), `judge`, `StatusCovered`
- [ ] `internal/le/arch/doctorcoverage/register.go` - registers `doctor-coverage check` on `./le commit create` via `changecheck.Register`
- [ ] `internal/core/diagnostic/event_codes.go` - `EventCode`, `RegisterEventCode` (requires prior `diagnostic.Register` metadata so `ze show diagnostic` describes the code and `recent-events` declares it)

**Behavior to preserve:**
- Both checks (`refuse-by-default` and `doctor-coverage`) and their rule texts. This work resolves the population, not the detectors.

**Behavior to change:**
- Every release-scope listed instance is resolved by one of the three answers in Key Design Decisions, with every answer-2 `Warn` under `internal/component`, `internal/plugins`, or `cmd/ze` naming a declared doctor code under `code`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError`; `./le arch refuse-by-default report` | Lists 281 unannotated instances tree-wide (276 release, 2 post-release MOBIKE, 3 post-release dev tooling) and 4 annotated; `check` refuses a new one on changed lines | Applies unchanged: it is the acceptance check of every child |
| The guard annotation | `internal/le/arch/refusebydefault/annotation.go` `parseGuard`, `namesEnclosing` | Accepts `refuse-by-default: guards <Symbol> -- <why>` for a deliberate answer naming a visible dependent symbol | Extends its use: answer 3 only, where going on is correct and nobody can act on the failure |
| Doctor-coverage commit gate | `internal/le/arch/doctorcoverage/doctorcoverage.go` `Scan`, `register.go` `changecheck.Register`; `docs/architecture/doctor-and-health-checks.md`, "Every problem site names a doctor code" | Refuses any newly added or changed `Warn`/`Error` log call or report-bus raise under `internal/component`, `internal/plugins`, or `cmd/ze` that lacks a literal/constant declared doctor `code`, and any shipped `diagnostic.Diagnostic` finding without `Fix` | Applies unchanged: every answer-2 `Warn` a child adds under those site roots must satisfy it on commit |
| Event-code registration and recent-events check | `internal/core/diagnostic/event_codes.go` `RegisterEventCode`, `EventCode`; `internal/component/doctor/checks_recent_events.go` | Lets an owning package declare transient event codes (described via `diagnostic.Register`) so `recent-events` covers them in `show doctor` and `doctor-coverage` accepts them | Extends its use: children adding a transient answer-2 `Warn` register their event code here when no existing doctor check declares one |
| Operator warnings and error ring | `internal/core/report/report.go` `RaiseWarningFix`, `RaiseErrorFix` (`show warnings`, `show errors`, `docs/architecture/core-design.md` section 15) | Holds lasting warnings and recent transient error events surfaced by `show doctor`; process-local, so a forked plugin uses its log or SDK callback | Applies where a failure is a lasting condition or transient runtime fault surfaced in `show doctor` |
| Prometheus metrics registry | `docs/plugin-development/metrics.md` | Per-subsystem counters | Applies for "count it" on a per-packet or per-route path, where a `Warn` per event would flood |
| User-facing error rule | `ai/rules/user-facing-errors.md` | What, why, fix, and declared doctor `code` in every operator warning | Applies to every `Warn` a child writes |
| Doctor-coverage area specs | `plan/release/validation/spec-doctor-coverage-0-umbrella.md` and its area children (`spec-doctor-coverage-3` through `15`) | Assign doctor checks and class-to-code maps per door area | Coordinates with this umbrella: children pick codes from the matching area doctor-coverage spec where one exists, or register the code alongside the `Warn` |
| Earlier refuse-by-default backlog | `spec-refuse-instead-of-defaulting` (closed in `15c252ebea` / `e32b0c9820`) | The original four shapes and the fifth dropped-error check | Bounded overlap already closed: `Reactor.notifyOneMessage` records parse failures, and `buildWithdrawalPayload` / `fwdWithdrawal.of` propagate conversion errors; this umbrella's children retain the remaining dropped-error backlog |
| Discarded-error lint ratchet | `plan/post-release/engineering/spec-lint-ratchet-clock-and-discarded-errors.md` (ready) | `_ = f()` discards with no log at all | Does not overlap: a blank discard has no log line, which `droppedError` does not read |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "Every message a user sees names the problem, why, and the exact fix" | agrees | Every answer-2 `Warn` a child adds names what failed, why it matters, and the exact fix under `ai/rules/user-facing-errors.md`, and carries its declared doctor `code` so `show doctor` reports it across every interface |
| `ai/doors/le-tooling.md` | Where it sits | door covering the `refuse-by-default` and `doctor-coverage` gates this umbrella verifies against | "A shipped `ze` never links an `internal/le` package." | agrees | The umbrella and its children only invoke `./le arch refuse-by-default` and `./le arch doctor-coverage` as offline checks and commit gates; no product package imports `internal/le` |
| `docs/contributing/ze-go-style.md` | The rules a reader applies | `// Design:` page of `dropped.go`, `annotation.go`, and `refusebydefault.go` | "An error caught and not returned MUST be logged at WARN or above (never Debug or Info)." | agrees | Every child eliminates `Debug`/`Info`-only error drops by returning the error, counting and warning at `Warn`, or annotating a verified guard where continuing is correct |
| `docs/architecture/doctor-and-health-checks.md` | Every problem site names a doctor code | `// Design:` page of `doctorcoverage.go` and `event_codes.go` | "A site is covered when its code is one that a doctor check declares." | agrees | Every answer-2 `Warn` or report-bus raise a child adds under `internal/component`, `internal/plugins`, or `cmd/ze` names a declared doctor code or a registered `diagnostic.EventCode`, so `./le arch doctor-coverage check` and `./le commit create` accept it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failure inside a shipped function: an error value returned by a call.

### Transformation Path
1. The branch catches the error.
2. Today: a `Debug` or `Info` line, then the function goes on. With the default WARN level the line is dropped and `show doctor` never learns of it.
3. After: the error is returned to a caller that acts (answer 1), or counted and/or logged at `Warn` with what, why, fix, and a declared doctor `code` attribute plus a report-bus raise for transient failures under `internal/component`, `internal/plugins`, and `cmd/ze` (answer 2), or annotated with `// refuse-by-default: guards <Symbol> -- <why>` naming the dependent guard that makes going on correct (answer 3).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Function ↔ caller | Returned error | Yes (verified per child 1–9) |
| Daemon ↔ operator | Log at `Warn` with doctor `code`, `show warnings` / `show errors` / `show doctor`, subsystem metric | Yes (verified per child 1–9) |

### Changed-Wire Census

**Search evidence:** this umbrella modifies zero source files and changes no wire format (confirmed by `## Files to Modify` being empty and `internal/le/arch/refusebydefault/dropped.go` plus `internal/le/arch/doctorcoverage/doctorcoverage.go` remaining unchanged); any child spec whose answer-1 fix changes an operator-visible RPC response, config, or protocol message records its own changed-wire census.

### Integration Points
- `slogutil.Logger` per subsystem; the level an operator sees by default is WARN.
- `diagnostic.Register` / `diagnostic.RegisterEventCode` (`internal/core/diagnostic/event_codes.go`) and `internal/core/report` (`RaiseWarningFix` / `RaiseErrorFix`) for doctor-covered answer-2 warnings.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Errors go to the caller, the subsystem logger with its doctor `code`, the report bus, or the metric registry each subsystem already uses |
| No unintended coupling (components stay isolated) | Yes | No new cross-component import; a forked plugin cannot reach daemon-local `internal/core/report`, so it logs with its doctor code or reports through its SDK doctor callback |
| No duplicated functionality (extends existing, does not recreate) | Yes | No new logger, counter family, warning store, or doctor registry |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No hot-path buffer is touched; per-packet paths count rather than format |
| Registration over hardcoding, outbound | Yes | Any new event code or doctor check registers from its owning package's `init()` via `diagnostic.Register` and `diagnostic.RegisterEventCode` / `diagnostic.RegisterDoctorCheck` |
| Registration over hardcoding, inbound | Yes | `./le arch doctor-coverage` reads declared codes directly from source declarations; no central code list is edited |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The tree-wide population is 281 unannotated instances (276 release across children 1–5 and 7–9, 0 in closed child 6, 2 in post-release `mobike.go`, 3 in post-release child 10) | `./le arch refuse-by-default report '|' json` run on 2026-11-26 | A child's table is stale; it re-runs the report and works from the fresh output | Each child re-ran the report filtered to its paths before its first edit and closed at 0 | confirmed |
| A-2 | Every child's paths plus the `mobike.go` exclusion partition the population with no overlap and no orphan | Longest `door-base` prefix per path (`ai/doors/README.md`) plus commit `58c46e854f` moving `mobike.go` to `spec-ipsec-11-mobike.md` | An instance owned twice or never | Tree-wide report at closure lists 0 release-path findings and exactly the 5 post-release survivors (`mobike.go` x2 + `dropped-error-10` x3) | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A `Warn` on a per-packet or per-route path floods the log | One test run writes the line thousands of times | Count it in an existing metric family and `Warn` once per condition, never per event |
| R-2 | An annotation is used to make the count zero where going on is wrong | Annotation reason restates the drop instead of naming a dependent guard | Review reads each annotation's guard; the rule says a `Warn` or annotation never makes a default correct |
| R-3 | New instances land while the children work | Report total rises | `./le arch refuse-by-default check` refuses a new one on changed lines; the umbrella's AC reads the tree at closure |
| R-4 | A child raises a `Debug`/`Info` call to `Warn` under `internal/component`, `internal/plugins`, or `cmd/ze` without a declared doctor `code` | `./le arch doctor-coverage check` or `./le commit create` refuses the commit | Coordinate with the area's `plan/release/validation/spec-doctor-coverage-*` code map, or register the check / `diagnostic.EventCode` via `diagnostic.Register` + `diagnostic.RegisterEventCode` in the same change and verify with `./le arch doctor-coverage check` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Logs gain lines, `./le commit create` refuses an uncovered `Warn`, or a caller now sees an error it did not before and changes behavior |
| How is it reverted? | Per child commit |
| Who else touches this path? | Every session touching a listed line (`refuse-by-default check` and `doctor-coverage check` both run on changed lines), plus `plan/release/validation/spec-doctor-coverage-*` (which assign doctor codes to the same log lines) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over the whole tree | `TestRefuseByDefaultRefusesEachRuleFixture` (the `dropped.go` case) and `TestRefuseByDefaultReportListsTree` in `internal/le/arch/refusebydefault/refusebydefault_test.go` |
| `./le arch doctor-coverage check` / `./le commit create` | → | `judgeChange` / `Scan` refusing an added `Warn` without a declared doctor `code` | `TestCommitCreateRefusesUncoveredProblemSite` (`internal/le/arch/doctorcoverage/register_test.go`), `TestDoctorCoverageRefusesAddedUncoveredSite` and `TestDoctorCoverageAcceptsCoveredSite` (`internal/le/arch/doctorcoverage/check_test.go`), and `TestDoctorCoverageCommitAdmitsCheckAddedWithItsSite` (`internal/le/arch/doctorcoverage/judge_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le arch refuse-by-default report '|' json` after children 1–5 and 7–9 close (child 6 already closed 2026-11-15) | The command exits 0 and reports 0 unannotated `error-dropped-below-warn` findings across every shipped release path (children 1–9), with each resolved branch returning its error, counting or warning at `Warn`, or appearing in `annotated` | Two explicitly homed post-release populations remain outside this release umbrella: `internal/component/ike/engine/mobike.go` (2 findings, opt-in unreleased MOBIKE moved on 2026-11-23 to `plan/post-release/features/spec-ipsec-11-mobike.md` Known Issues #3) and `internal/chaos/` plus `internal/test/` (3 findings, unshipped dev tooling owned by `plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md`) |
| AC-2 | Release child specs 1–9 (children 1–5 and 7–9 open; child 6 closed 2026-11-15) | Every release child spec is closed with its own AC-1 green over its door's paths and its scoped main-thread tests passing | Child 10 (`plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md`) and `plan/post-release/features/spec-ipsec-11-mobike.md` are post-release specs and close on their own schedules; zero release children remain open |
| AC-3 | Every `refuse-by-default: guards <Symbol> -- <why>` annotation added by the children | Each annotation appears in the report's `annotated` list with `guard` set to a visible dependent symbol (distinct from the enclosing function) and a reason stating why continuing is correct | Only sites where continuing is genuinely correct and no operator action exists may carry a guard annotation; zero annotations are permitted where going on is wrong or an operator can act |
| AC-4 | Every answer-2 `Warn` or `Error` log line or report-bus raise added by the children under `internal/component`, `internal/plugins`, or `cmd/ze` | Each added site carries a literal or constant `code` attribute naming a declared doctor check code or registered `diagnostic.EventCode`, and `./le arch doctor-coverage check` exits 0 on each child's changes | Call sites under `internal/core`, `internal/appliance`, `internal/chaos`, `internal/test`, and `internal/le` lie outside `doctorcoverage.siteRoots` (libraries whose callers decide operator output, or unshipped tooling) and are not refused as call sites, though any `diagnostic.Diagnostic` finding in shipped Go still owes a `Fix` field |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRefuseByDefaultRefusesEachRuleFixture`, `TestRefuseByDefaultReportListsTree` | `internal/le/arch/refusebydefault/refusebydefault_test.go` | The detector still refuses `testdata/dropped.go` and accepts `testdata/annotated.go` | |
| `TestCommitCreateRefusesUncoveredProblemSite`, `TestDoctorCoverageRefusesAddedUncoveredSite`, `TestDoctorCoverageAcceptsCoveredSite`, `TestDoctorCoverageCommitAdmitsCheckAddedWithItsSite` | `internal/le/arch/doctorcoverage/register_test.go`, `check_test.go`, `judge_test.go` | The doctor-coverage commit gate refuses any added `Warn` lacking a declared doctor code and admits one whose check or event code is declared | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated release-path findings of the shape | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Per child | Each child names its suites | An operator sees the failure at the default WARN level and, where doctor-covered, in `show doctor` | |

## Files to Modify
- None in the umbrella. Each child names its files.

## Files to Create
- The ten child specs in the Task table (already created; child 6 closed on 2026-11-15).

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Per child; no child adds config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Doctor code per problem class | Yes | Applies per child for every answer-2 `Warn` or report-bus raise under `internal/component`, `internal/plugins`, and `cmd/ze`: each site carries a literal or constant `code` attribute naming a declared doctor check code or registered `diagnostic.EventCode` (`ai/rules/user-facing-errors.md`, `docs/architecture/doctor-and-health-checks.md`) |
| Prometheus counters/metrics | N-A | Per child, where one counts on a hot path |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | Per child |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Per child, if a child adds a new `diagnostic.Register` / `RegisterEventCode` entry |
| 16 | Any changed source file referenced by existing doc source anchors? | No | The umbrella changes no source |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Design presentation

### What changes

This umbrella changes no code of its own; it defines the shared resolution
policy and release-wide acceptance for its nine shipped-code child specs
(children 1–5 and 7–9, with child 6 already closed on 2026-11-15), covering 276
remaining `error-dropped-below-warn` instances across shipped release paths at
HEAD on 2026-11-26. After the release children land, no shipped release path
catches an error and hides it behind a `Debug` or `Info` log line: every site
either returns the error (answer 1), logs at `Warn` with what failed, why it
matters, the exact fix, and a registered doctor `code` attribute (plus a
report-bus raise for transient faults or a hot-path metric counter, answer 2)
satisfying `./le arch doctor-coverage check` on commit, or carries a verified
`refuse-by-default: guards <Symbol> -- <why>` annotation (answer 3). The five
remaining non-release instances — 2 in opt-in unreleased MOBIKE (`mobike.go`,
moved on 2026-11-23 to `plan/post-release/features/spec-ipsec-11-mobike.md`
Known Issues #3) and 3 in unshipped dev tooling (`internal/chaos/` and
`internal/test/`, owned by `plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md`)
— stay explicitly homed in their post-release specs.

### Before and after

1. **Answer 1 (going on is wrong):**
   - Before: a shipped function catches `err != nil`, writes one `Debug` or `Info` line that the default WARN level discards, and continues or returns success so its caller treats the failed operation as done.
   - After: the function returns the error (or fixes the underlying defect), so the caller acts on the failure instead of continuing silently.
2. **Answer 2 (going on is right and an operator can act):**
   - Before: a failure writes only a `Debug` or `Info` line with no doctor code; default logs stay empty and `show doctor` reports nothing.
   - After: the branch logs at `Warn` naming what failed, why it matters, and the exact next step, carrying a literal or constant `code` attribute declared by a doctor check or `diagnostic.RegisterEventCode` (plus a report-bus raise for transient failures so `show doctor` surfaces it, or a subsystem counter plus first-occurrence `Warn` on hot paths), and passes `./le arch doctor-coverage check`.
3. **Answer 3 (going on is right and nobody can act):**
   - Before: a benign teardown or closed-client drop appears as an unannotated `error-dropped-below-warn` finding in `./le arch refuse-by-default report`.
   - After: the branch carries `// refuse-by-default: guards <Dependent> -- <why>` naming a visible dependent symbol distinct from the enclosing function, and appears in the report's `annotated` list.
4. **Release acceptance report (`./le arch refuse-by-default report '|' json`):**
   - Before (2026-11-26): 281 unannotated `error-dropped-below-warn` findings (276 across shipped release children 1–5 and 7–9, 0 in closed child 6, 2 in post-release `mobike.go`, 3 in post-release dev tooling).
   - After children 1–5 and 7–9 close: 0 unannotated `error-dropped-below-warn` findings across every shipped release path; only the 5 explicitly homed post-release findings remain until `spec-ipsec-11-mobike.md` and `spec-dropped-error-10-dev-tooling.md` close.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Raise every dropped-error `Debug`/`Info` call mechanically to `Warn` | Violates `log-a-dropped-error-at-warn-or-above.md`: a `Warn` does not make continuing after a failure correct, and a per-packet `Warn` floods the log |
| Add bare `Warn` calls without doctor `code` attributes or add a doctor-coverage exemption comment | Violates `ai/rules/user-facing-errors.md`; `./le arch doctor-coverage check` and `./le commit create` refuse any added `Warn` without a declared doctor code and never read exemption comments |
| Keep all ten doors in one monolithic spec | Violates `ai/META-DECISIONS.md` ("a spec whose edits need many doors is split into narrower child specs") and makes review and rollback too wide |
| Keep opt-in MOBIKE (`mobike.go`) and unshipped dev tooling (`internal/chaos/`, `internal/test/`) inside the first-release acceptance gate | Contradicts the 2026-11-23 owner ruling (`58c46e854f`) moving opt-in unreleased MOBIKE to `plan/post-release/features/spec-ipsec-11-mobike.md` and keeps unshipped test tooling out of release fixes (`plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md`) |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring** -- run `./le arch refuse-by-default report '|' json`, confirm A-1 (281 tree-wide: 276 release, 2 post-release MOBIKE, 3 post-release dev tooling) and A-2 (clean partition across child doors plus the `mobike.go` exclusion).
2. **Phase: Children** -- each open release child (children 1–5 and 7–9) runs as its own spec in any order, applying the three-answer policy and doctor-coverage `code` requirement on every answer-2 `Warn` under `internal/component`, `internal/plugins`, and `cmd/ze`, verified from the main thread in the main checkout with scoped package/component/functional tests plus `./le arch refuse-by-default check` and `./le arch doctor-coverage check`.
3. **Phase: Closure** -- confirm AC-1..AC-4 over the tree after all release children (1–9) are closed, then close the umbrella.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Report clean of `error-dropped-below-warn` across all shipped release paths (children 1–9), and every release child closed |
| Rule: refuse-by-default | No annotation hides a drop where going on is wrong; every annotation names a visible dependent guard distinct from its enclosing function |
| Operator errors and doctor codes | Every answer-2 `Warn` or report-bus raise added under `internal/component`, `internal/plugins`, or `cmd/ze` names a declared doctor code under `code` and passes `./le arch doctor-coverage check` (`ai/rules/user-facing-errors.md`) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero unannotated release-path findings of the shape | `./le arch refuse-by-default report '|' json`, filtered on `rule` (`error-dropped-below-warn`) across all shipped release paths |
| Detectors unchanged and green | `./le test unit pkg path ./internal/le/arch/refusebydefault/` and `./le test unit pkg path ./internal/le/arch/doctorcoverage/` |
| Scoped child change sets verified | Per child, run in the main checkout by the main thread: the scoped package tests (`./le test unit pkg path <dir>/`), component groups and functional suites `./le repo changed scope print both` reaches, plus `./le arch refuse-by-default check` and `./le arch doctor-coverage check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | A `Warn` or a returned error that carries a secret (a key, a password, a RADIUS shared secret) in its text |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint or gate failure (`refuse-by-default` or `doctor-coverage`) | Fix inline (add declared doctor `code` or register event code). If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `dropped-error-logged-below-warn.md` | 2026-11-02 | tree-wide `./le arch refuse-by-default report` backlog (328 on 2026-11-02; 281 at HEAD on 2026-11-26) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Each instance is judged first on whether going on is correct. Where it is not, the error is returned or the behavior fixed | Raise every line to `Warn` | The rule: a `Warn` does not make a default correct |
| Where going on is correct and an operator can act: `Warn` with what, why, and fix (`ai/rules/user-facing-errors.md`) plus a literal or constant `code` attribute naming a declared doctor code (from the area's `spec-doctor-coverage-*` map or registered via `diagnostic.Register` + `diagnostic.RegisterEventCode` in the same change) under `internal/component`, `internal/plugins`, and `cmd/ze`, with a report-bus raise for transient faults; on a per-event hot path, count it in the subsystem's existing metrics and `Warn` once per condition | Bare `Warn` without doctor `code`, or `Warn` per event | `./le arch doctor-coverage check` and `./le commit create` refuse any added `Warn` lacking a declared doctor code under the site roots; a per-packet `Warn` floods the log and hides the next failure |
| Where going on is correct and nobody can act (teardown close, a reply to a client that left): `// refuse-by-default: guards <Symbol> -- <why>` | Debug line kept | The annotation is the check's one accepted escape and names why the drop is the answer |
| Split by door | One spec | Ten doors; `ai/META-DECISIONS.md`: a spec whose edits need many doors is split |
| Post-release placement for opt-in MOBIKE (`mobike.go`, 2 sites) and unshipped dev tooling (`internal/chaos/`, `internal/test/`, 3 sites) | Keep in first-release fixes | Owner ruling of 2026-11-23 (`58c46e854f`) moved opt-in unreleased MOBIKE to `plan/post-release/features/spec-ipsec-11-mobike.md` (Known Issues #3), and no shipped binary links `internal/chaos` or `internal/test/runner` (`plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md`) |

## Known Limitations
- The 2 dropped-error instances in `internal/component/ike/engine/mobike.go` (`:590` and `:607`) belong to opt-in unreleased MOBIKE and are homed in `plan/post-release/features/spec-ipsec-11-mobike.md` (Known Issues #3, moved 2026-11-23).
- The 3 dropped-error instances in `internal/chaos/web/handlers.go` (2) and `internal/test/runner/peer_contract.go` (1) belong to unshipped development tooling and are homed in `plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread per child: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, plus `./le arch refuse-by-default check` and `./le arch doctor-coverage check` (no agent runs any `./le verify` command; GitHub CI verifies pushed commits)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-0-umbrella.md` only, in the same `./le commit create` script

---

## Implementation Summary

### What Was Implemented
- All 9 shipped-release children of this umbrella closed on `main`, eliminating every `error-dropped-below-warn` finding across shipped release paths (0 remaining release findings tree-wide; 352 annotated guard sites):
  - Child 1 (`spec-dropped-error-1-bgp-engine`, 48 sites): `f49458370c4` + `7f0416c961f`
  - Child 2 (`spec-dropped-error-2-bgp-plugins`, 63 sites): `9d583df3295` + `3c7535e4c6a`
  - Child 3 (`spec-dropped-error-3-access-security`, 60 sites minus 2 post-release `mobike.go` sites): `048e653e294` + `c2dcf595751`
  - Child 4 (`spec-dropped-error-4-system-plugins`, 50 sites): `7cb035c6c43` + `3637d0be01d`
  - Child 5 (`spec-dropped-error-5-forwarding`, 36 sites): `09b67e0323a` + `c25c37f8716`
  - Child 6 (`spec-dropped-error-6-routing-protocols`, 28 sites): `f126ead44e`
  - Child 7 (`spec-dropped-error-7-plugin-infra`, 20 sites): `c8751ff9052` + `7d7fc77bab0`
  - Child 8 (`spec-dropped-error-8-core-hub-appliance`, 11 sites): `b282d3d0cb2` + `78ef5d79e16`
  - Child 9 (`spec-dropped-error-9-api-cli-config`, 8 sites): `4ff83d07f12` + `0c388957aec`
- Updated owned journal row in `plan/journal/dropped-error-logged-below-warn.md` from `open:` to `fixed:` and updated external citations in `plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md` to bare stem `spec-dropped-error-0-umbrella`.

### Bugs Found/Fixed
- Resolved real latent bugs uncovered across the 9 children (each covered by dedicated unit/functional tests in its child commit), including BMP `bounceMonitoredPeers` RFC 8671 §7.2 missing `abortAdjReplay` + BMP Stage-2 handshake `awaitingHandshake`/`OnStarted` gate (`bmp-locrib.ci`), `tftpserver` strict reload rollback, `ntp` worker teardown, `flowexport` overlapping reload, and `config dump` fail-closed `$9$` encoding error propagation.

### Documentation Updates
- Umbrella modifies zero code and owes no standalone documentation page (`git grep 'spec-dropped-error-0-umbrella'` confirms only bare-stem historical citations remain). Each child spec updated its own architecture/guide pages and `website/changes/fragments/` entry when landing.

### Deviations from Plan
- None in the umbrella policy; individual child layout refinements were documented in each closed child spec.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| All 9 release children (1–9) resolved under the 3-answer policy and closed on `main` | Done | Commits `f126ead44e` through `0c388957aec` | Every release child closed cleanly with its own Review Gate and scoped tests |
| Tree-wide `./le arch refuse-by-default report` shows 0 release-path `error-dropped-below-warn` findings | Done | `./le arch refuse-by-default report '\|' json` on 2026-11-27 | Exactly 5 post-release survivors remain (3 `dropped-error-10`, 2 `mobike.go`) |
| Every newly added `Warn` under doctor-coverage site roots carries a declared doctor code | Done | Enforced by `./le arch doctor-coverage check` on every child commit | 0 uncovered sites introduced by children 1–9 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `./le arch refuse-by-default report '\|' json` (2026-11-27): `error-dropped-below-warn: 5` (`internal/chaos/web/handlers.go:346,384`, `internal/test/runner/peer_contract.go:386`, `internal/component/ike/engine/mobike.go:590,607`) — 0 release-path findings | Exact permitted post-release survivors only |
| AC-2 | Done | `git log --oneline --grep='dropped-error'` + `ls plan/release/fixes/spec-dropped-error-*.md` | All 9 release children closed; zero release children remain open |
| AC-3 | Done | `./le arch refuse-by-default report '\|' json` (`annotated count: 352`) and per-child 0 `annotation-names-no-guard` verification | Every child verified 0 invalid annotations on its files |
| AC-4 | Done | `./le test unit pkg path ./internal/le/arch/refusebydefault/ path ./internal/le/arch/doctorcoverage/` (`exit 0, 2 packages ok`) + per-child `./le commit create` doctor-coverage gate | All 9 children passed doctor-coverage at commit time |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRefuseByDefaultRefusesEachRuleFixture`, `TestRefuseByDefaultReportListsTree` | Done | `internal/le/arch/refusebydefault/refusebydefault_test.go` | PASS via `./le test unit pkg` (`exit 0`) |
| `TestCommitCreateRefusesUncoveredProblemSite`, `TestDoctorCoverageRefusesAddedUncoveredSite`, `TestDoctorCoverageAcceptsCoveredSite`, `TestDoctorCoverageCommitAdmitsCheckAddedWithItsSite` | Done | `internal/le/arch/doctorcoverage/{register,check,judge}_test.go` | PASS via `./le test unit pkg` (`exit 0`) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Code files | Done (none) | Umbrella modifies zero source files by design |
| `plan/journal/dropped-error-logged-below-warn.md` | Done | Owned row updated from `open:` to `fixed:` |
| `plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md` | Done | Path citations updated to bare stem `spec-dropped-error-0-umbrella` |

### Audit Summary
- **Total items:** 9 release children (276 release sites), 4 ACs, 2 detector test packages, 1 owned journal row
- **Done:** All 9 release children, all 4 ACs, both detector test packages, owned journal row updated
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Zero unannotated `error-dropped-below-warn` findings across every shipped release path (children 1–9) | Architecture report | `./le arch refuse-by-default report '\|' json` on 2026-11-27 lists `error-dropped-below-warn: 5`, strictly consisting of the 3 post-release dev-tooling sites (`internal/chaos/web/handlers.go:346,384`, `internal/test/runner/peer_contract.go:386`) and 2 post-release MOBIKE sites (`internal/component/ike/engine/mobike.go:590,607`); 0 shipped release sites remain |
| Detectors (`refuse-by-default` and `doctor-coverage`) remain intact and green | Unit tests | `./le test unit pkg path ./internal/le/arch/refusebydefault/ path ./internal/le/arch/doctorcoverage/` -> `exit 0, 2 packages ok, test time 152.390s` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| 3 unshipped dev-tooling sites (`internal/chaos/web/handlers.go:346,384`, `internal/test/runner/peer_contract.go:386`) | Unshipped dev tooling outside release scope | `plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md` |
| 2 opt-in unreleased MOBIKE sites (`internal/component/ike/engine/mobike.go:590,607`) | Owner ruling `58c46e854f` (2026-11-23) moved opt-in MOBIKE out of first release | `plan/post-release/features/spec-ipsec-11-mobike.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/dropped-error-0-umbrella-b5d40004-4444-4000-8000-000000000004.md` |
| `./le spec review check` | `clean` (umbrella policy spec; 0 code files modified; all 9 child reviews CLEAN and landed) |
| Rounds | 1 (umbrella closure audit + independent `ze-design-judge` APPROVE `b084078708fc`) |
| Reviewer lenses used | spec-completeness, tree-wide acceptance gate, citation hygiene |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `plan/journal/dropped-error-logged-below-warn.md` | Yes | Tracked journal file read and updated (`row 12` marked `fixed:`) |
| `plan/post-release/engineering/spec-dropped-error-10-dev-tooling.md` | Yes | Tracked post-release child spec read and citations updated |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | 0 shipped release `error-dropped-below-warn` findings | `./le arch refuse-by-default report '\|' json`: `error-dropped-below-warn: 5` (only `chaos/web/handlers.go:346,384`, `test/runner/peer_contract.go:386`, `ike/engine/mobike.go:590,607`) |
| AC-2 | All 9 release children closed on `main` | `git log --oneline --grep='dropped-error'` shows closed commits for children 1–9 (`f126ead44e`, `c2dcf595751`, `c25c37f8716`, `78ef5d79e16`, `7f0416c961f`, `3637d0be01d`, `7d7fc77bab0`, `3c7535e4c6a`, `0c388957aec`) |
| AC-3 | Annotations valid across tree | `./le arch refuse-by-default report '\|' json` reports `annotated count: 352` and 0 invalid annotations on every closed child |
| AC-4 | Detector suites green and doctor-coverage enforced | `./le test unit pkg path ./internal/le/arch/refusebydefault/ path ./internal/le/arch/doctorcoverage/` exited `0` (2 packages ok) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le arch refuse-by-default report` + `./le arch doctor-coverage check` | `internal/le/arch/refusebydefault/refusebydefault_test.go`, `internal/le/arch/doctorcoverage/{register,check,judge}_test.go` (offline tooling; no `.ci` daemon) | Yes (read and executed via `./le test unit pkg`, `exit 0`) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | All 9 release children reconciled their path-filtered reports before editing and landed at 0 |
| A-2 | confirmed | Post-closure tree-wide report has 0 release findings and exactly the 5 post-release survivors (3 dev tooling + 2 MOBIKE) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| External spec citations | `git grep 'spec-dropped-error-0-umbrella'` after edits shows zero live path citations; only bare stems in `plan/journal/dropped-error-logged-below-warn.md` and `spec-dropped-error-10-dev-tooling.md` | Yes |
| No umbrella-owned code or doc pages | `## Files to Modify` is empty; each child spec updated its own docs and news fragment | Yes |
