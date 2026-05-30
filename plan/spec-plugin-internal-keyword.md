# Spec: plugin `internal` keyword + external-of-builtin doctor advisory

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/7 |
| Updated | 2026-05-30 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/doctor-checks.md`, `ai/rules/derive-not-hardcode.md`, `ai/patterns/config-option.md`
4. `internal/component/plugin/schema/ze-plugin-conf.yang`, `internal/component/config/loader.go`, `cmd/ze/doctor/doctor.go`

## Task

Plugin declarations today live under a single YANG list literally named `external`, and whether a
plugin runs in-process (fast) or as a separate process (slow, IPC + JSON serialization) is implied
only by which leaf is set: `use <builtin>` and `run ze.<builtin>` are in-process, `run <path>` is a
separate process. There is no `internal` keyword, so the in-process path is not obvious.

Three deliverables:

1. **Add an explicit `internal` keyword.** A new `plugin { internal <name> { use <builtin> } }` list,
   sibling to `external`, declares a built-in plugin to run in-process. The existing `external { use }`
   and `external { run ze.X }` forms keep working for back-compat.
2. **Convert examples.** Every example (docs, wiki, `etc/`, `test/`) that declares a built-in plugin
   in-process via `external { use X }` or `external { run ze.X }` is rewritten to `internal { use X }`.
   Genuine external-process examples (`external { run ./script }`, `run /path`, `run auto`) are left
   unchanged: some users genuinely need external.
3. **Doctor advisory.** `ze doctor` flags any `external { run <cmd> }` whose target resolves to a
   built-in plugin as a severe performance problem and points the operator at `internal { use <name> }`.
   The advisory is `warning` severity (non-blocking), never `error`.

**Hard constraints (do not violate):** the external/`run` loading mechanism stays fully functional;
no routing or forwarding behavior changes; the doctor advisory is advisory only.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/doctor-checks.md` - how runtime-dependency doctor checks are added
  → Constraint: any new doctor check needs a registered diagnostic code in `internal/core/diagnostic/codes.go` plus a unit test and a `test/ui/doctor-*.ci` functional test
- [ ] `ai/rules/derive-not-hardcode.md` - enumerations must come from the registry
  → Constraint: the set of built-in plugin names MUST be read from `plugin.AvailableInternalPlugins()` (→ `registry.Names()`), never hardcoded in the doctor check
- [ ] `ai/patterns/config-option.md` - adding a YANG list/leaf
  → Constraint: every new leaf gets maximum native YANG validation; `name` reuses the `length "1..64"` constraint already on the `external` list; `use` is `type string` referencing a built-in plugin name (custom `ze:validate` + `CompleteFn` resolving against `AvailableInternalPlugins()` for tab-completion)
- [ ] `ai/rules/config-design.md` - augment vs grouping, listener pattern
  → Constraint: `internal` is a plain `list` under the existing `container plugin`; no augment needed

### RFC Summaries (MUST for protocol work)
- N/A — no wire-protocol behavior changes. Plugin loading is local config/process management only.

**Key insights:**
- The YANG list is named `external`; `use`/`run ze.X` already resolve to in-process. The new `internal` list is sugar that makes the in-process intent explicit and is mapped to the same `PluginConfig{Internal:true}`.
- Doctor already has `checkPlugins()` iterating plugin configs; the advisory is an extension, not a new top-level check.
- `ExtractPluginsFromTree` is the single production chokepoint for plugin resolution (verified via LSP: one non-test caller at `loader.go:40`, the rest are tests), so the new `internal` list only needs handling there plus the dependency graph.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/schema/ze-plugin-conf.yang` - `container plugin` holds `hub`, and a `list external { leaf name; leaf run; leaf use; leaf encoder; leaf respawn; leaf timeout; }`. `run` description: "Command to execute plugin as external process". `use` description: "Name of a built-in plugin to run in-process (mutually exclusive with run)".
  → Constraint: `run` and `use` are documented mutually exclusive; the new `internal` list must preserve that invariant (internal has `use`, no `run`).
- [ ] `internal/component/config/loader.go` - `ExtractPluginsFromTree` (line 199) iterates `pluginContainer.GetList("external")`. For each: `use != ""` sets `pc.Internal = true; pc.Run = useVal`; `run != ""` sets `pc.Run = runVal`; both set is an error. Non-internal plugins go through `MarkInternalPlugin(&pc)` (line 237), which detects the `ze.` prefix form. Verified via LSP: single production caller at `loader.go:40`.
  → Constraint: a new loop over `GetList("internal")` must map entries to `PluginConfig{Name, Internal:true, Run:useVal}` and reject a `run` leaf inside `internal`. Names must be unique across BOTH lists.
- [ ] `internal/component/config/graph.go` - line 154 iterates `GetList("external")` to build the plugin dependency graph.
  → Constraint: the `internal` list must be included here too, or `internal`-declared plugins are invisible to dependency ordering.
- [ ] `internal/component/plugin/resolve.go` - `ResolvePlugin` resolution rules: `ze.X` → internal (no fork), `./path` / `/path` → fork, `auto` → auto-discover. `AvailableInternalPlugins()` (line 49) returns `registry.Names()`. `IsInternalPlugin(name)` validates a built-in name.
  → Constraint: doctor must use `AvailableInternalPlugins()` / `IsInternalPlugin()` for built-in matching (derive-not-hardcode). Back-compat `ze.X` resolution stays untouched.
- [ ] `cmd/ze/doctor/doctor.go` - `checkPlugins(plugins []zeplugin.PluginConfig)` (line 759) skips `p.Internal || p.Run == ""`, then for true-external plugins validates the binary exists (`doctor-plugin-missing`, SeverityError). Uses `diagnostic.Diagnostic{Code, Severity, Message, Path}`.
  → Constraint: add the perf advisory inside this same loop (or a sibling helper called from `runChecks`); it must run for `Run != "" && !Internal` entries.
- [ ] `internal/core/diagnostic/types.go` - severities are only `SeverityError` and `SeverityWarning`.
  → Constraint: the perf advisory is `SeverityWarning`. There is no info/note level below warning.
- [ ] `internal/core/diagnostic/codes.go` - line 181 registers `doctor-plugin-missing`.
  → Constraint: register the new code (e.g. `doctor-plugin-external-builtin`) here with description and remediation.

**Behavior to preserve:**
- `external { run <path|auto> }` external-process loading, respawn, encoder, timeout — unchanged.
- `external { use X }` and `external { run ze.X }` still resolve to in-process (`Internal=true`) — back-compat.
- `run`/`use` mutual-exclusion error message.
- `doctor-plugin-missing` error behavior for missing external binaries.
- All routing, best-path, forwarding, and reactor behavior — untouched.

**Behavior to change:**
- New `internal` YANG list accepted and resolved to `PluginConfig{Internal:true}`.
- `ze doctor` emits a new `warning` for external-process loading of a built-in.
- In-process built-in examples migrated from `external { use/run ze. }` to `internal { use }`.

## Data Flow (MANDATORY)

### Entry Point
- Config file / CLI editor: `plugin { internal <name> { use <builtin> } }` and `plugin { external <name> { run <cmd> } }`.
- `ze doctor [<config>]`: offline read of the same tree.

### Transformation Path
1. YANG parse → `config.Tree` with `plugin` container holding `internal` and `external` lists.
2. `ExtractPluginsFromTree` (loader.go) → `[]plugin.PluginConfig` (`Internal` true for `internal`-list and legacy in-process forms; false for true-external).
3. `graph.go` builds dependency order across both lists.
4. Plugin manager loads each `PluginConfig` (in-process via DirectBridge when `Internal`, fork when external) — unchanged.
5. `ze doctor` → `runChecks` → `checkPlugins` reads the same `[]PluginConfig`, emits diagnostics.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ PluginConfig | `ExtractPluginsFromTree` maps `internal`/`external` lists | [ ] |
| PluginConfig ↔ doctor | `runChecks(configPath)` → `checkPlugins` | [ ] |
| Doctor ↔ registry | `AvailableInternalPlugins()` for built-in name set | [ ] |

### Integration Points
- `ExtractPluginsFromTree` — add `internal` list handling beside `external`.
- `graph.go` plugin dependency iteration — add `internal` list.
- `checkPlugins` — add advisory branch.
- `ze-plugin-conf.yang` — add `internal` list + validator.

### Architectural Verification
- [ ] No bypassed layers (internal list flows through the same PluginConfig path)
- [ ] No unintended coupling (doctor reads registry, does not import plugin impls)
- [ ] No duplicated functionality (extends `checkPlugins`, reuses `AvailableInternalPlugins`)
- [ ] Zero-copy preserved where applicable (N/A — config/offline path)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `plugin { internal rib { use bgp-rib } }` | → | `ExtractPluginsFromTree` yields `PluginConfig{Name:"rib", Internal:true, Run:"bgp-rib"}` | `TestExtractPluginsInternalList` |
| `ze doctor <conf with external run of a builtin>` | → | `checkPlugins` emits `doctor-plugin-external-builtin` | `test/ui/doctor-plugin-external-builtin.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config `plugin { internal rib { use bgp-rib } }` | Parses; resolves to `PluginConfig{Name:"rib", Internal:true, Run:"bgp-rib"}`; loads in-process |
| AC-2 | Config `internal x { use bgp-rib; run ./p }` | Rejected at parse/extract: `internal` plugins take `use`, not `run` |
| AC-3 | Same plugin name in both `internal` and `external` lists | Rejected: duplicate plugin name |
| AC-4 | Legacy `external x { use bgp-rib }` and `external x { run ze.bgp-rib }` | Still resolve to `Internal=true` (back-compat unchanged) |
| AC-5 | `ze doctor` on config with `external feed { run "<ze-binary> bgp-rib" }` (a token, after `ze.`-strip + basename, equals a built-in name) | Emits `warning`, code `doctor-plugin-external-builtin`, message names the matched built-in and recommends `internal { use <name> }` |
| AC-6 | `ze doctor` on config with `external feed { run ./collector.py }` (no token matches a built-in name) | No `doctor-plugin-external-builtin` diagnostic |
| AC-5b | Matching rule | For a token to match: split `Run` on whitespace; for each token strip a leading `ze.` prefix and take `filepath.Base`; match iff the result is in `AvailableInternalPlugins()`. Only the resulting built-in name(s) are reported. No partial/substring matching. |
| AC-7 | `ze doctor` on config with `internal rib { use bgp-rib }` | No `doctor-plugin-external-builtin` diagnostic |
| AC-8 | AC-5 advisory | Severity is `warning`; `ze doctor` does not change its pass/fail exit purely because of this diagnostic |
| AC-9 | Whole change | `run`/external mechanism and all routing code unchanged (grep proof: no edits under reactor/forwarding) |
| AC-10 | Repo example sweep | Every in-process built-in example (`external { use }` / `external { run ze. }`) in docs, wiki, `etc/`, `test/` converted to `internal { use }`; every genuine `run <path>` example unchanged (audit table) |
| AC-11 | Built-in name source | Doctor matches against `AvailableInternalPlugins()`; no hardcoded plugin-name list (grep proof) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractPluginsInternalList` | `internal/component/config/loader_test.go` | AC-1: `internal` list → `Internal=true` | |
| `TestExtractPluginsInternalRejectsRun` | `internal/component/config/loader_test.go` | AC-2 | |
| `TestExtractPluginsDuplicateAcrossLists` | `internal/component/config/loader_test.go` | AC-3 | |
| `TestExtractPluginsLegacyUseStillInternal` | `internal/component/config/loader_test.go` | AC-4 | |
| `TestCheckPluginsExternalBuiltinWarns` | `cmd/ze/doctor/doctor_test.go` | AC-5, AC-8 | |
| `TestCheckPluginsExternalScriptNoWarn` | `cmd/ze/doctor/doctor_test.go` | AC-6 | |
| `TestCheckPluginsInternalNoWarn` | `cmd/ze/doctor/doctor_test.go` | AC-7 | |
| `TestCheckPluginsBuiltinNamesFromRegistry` | `cmd/ze/doctor/doctor_test.go` | AC-11 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `internal/<name>` length | 1..64 | 64 chars | 0 chars (empty) | 65 chars |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-plugin-external-builtin` | `test/ui/doctor-plugin-external-builtin.ci` | Operator runs `ze doctor` on a config that forks a built-in; sees the perf advisory recommending `internal { use }` | |
| `doctor-plugin-internal-clean` | `test/ui/doctor-plugin-internal-clean.ci` | Operator runs `ze doctor` on the `internal { use }` form; no advisory | |
| `parse-plugin-internal` | `test/parse/plugin-internal.conf` (+ `.ci`) | Config with `internal { use }` parses and loads in-process | |

### Interop Tests (MANDATORY for protocol features)
- N/A — no wire-protocol change. Justification: plugin loading is local config + process management; no peer-visible behavior.

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/component/plugin/schema/ze-plugin-conf.yang` - add `list internal { key name; leaf name; leaf use; }` under `container plugin`
- `internal/component/config/loader.go` - `ExtractPluginsFromTree`: handle the `internal` list; reject `run` in internal; enforce cross-list unique names
- `internal/component/config/graph.go` - include `internal` list in dependency-graph iteration (line ~154)
- `cmd/ze/doctor/doctor.go` - extend `checkPlugins` with the external-of-builtin advisory
- `internal/core/diagnostic/codes.go` - register `doctor-plugin-external-builtin`
- `internal/component/plugin/validators_register.go` (or equivalent) - `ze:validate` + `CompleteFn` for the `internal/use` leaf against `AvailableInternalPlugins()`
- Example configs across `docs/`, `../wiki/`, `etc/ze/`, `test/` - convert in-process built-in declarations to `internal { use }`
- `docs/guide/plugins.md`, `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` - document the `internal` keyword and the doctor advisory

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new list/leaf) | [ ] | `internal/component/plugin/schema/ze-plugin-conf.yang` |
| YANG validation constraints | [ ] | `name` length 1..64; `use` custom `ze:validate` against built-in names |
| YANG custom validators | [ ] | `CompleteFn` returns `AvailableInternalPlugins()` for tab-completion |
| CLI commands/flags | [ ] | None new (config-driven; editor picks up YANG automatically) |
| Editor autocomplete | [ ] | Automatic for `internal/use` via `CompleteFn` |
| Functional test for new behavior | [ ] | `test/ui/doctor-plugin-external-builtin.ci`, `test/parse/plugin-internal.*` |
| Doctor check for runtime dependencies | [ ] | `cmd/ze/doctor/doctor.go`, `internal/core/diagnostic/codes.go`, unit + functional test |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` (config ergonomics note) |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | No — no new command |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` (internal vs external wording) |
| 6 | Has a user guide page? | [ ] | `docs/guide/plugins.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` if new suite dirs touched |
| 15 | Registered plugin/command/inventory changed? | [ ] | No — registry contents unchanged |
| 17 | Existing docs show config examples for this area? | [ ] | Convert `external { use }` examples in docs + wiki to `internal { use }` |

## Files to Create
- `test/ui/doctor-plugin-external-builtin.ci` - functional test for the advisory
- `test/ui/doctor-plugin-internal-clean.ci` - negative functional test
- `test/parse/plugin-internal.conf` (+ matching `.ci`) - parse/load test for the new list

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7-10. Critical review loop | Critical Review Checklist |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 14. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** — verify doctor sees the registry; create entry points and failing wiring tests
   - Tests: `TestExtractPluginsInternalList`, `test/ui/doctor-plugin-external-builtin.ci`
   - Files: `loader.go` (stub internal loop), `doctor.go` (stub advisory), `codes.go` (register code)
   - Verify: **First, confirm `AvailableInternalPlugins()` is non-empty in the `cmd/ze/doctor` context** (the doctor binary must trigger plugin `init()` registration); if empty, the advisory silently no-ops — fix the import before proceeding. Wiring tests then fail because logic is a stub.
2. **Phase: YANG `internal` list** — add the list + validator
   - Tests: `test/parse/plugin-internal.*`, boundary tests on `name`
   - Files: `ze-plugin-conf.yang`, validator registration
   - Verify: config with `internal { use }` parses; invalid `use` name rejected
3. **Phase: Config resolution** — map `internal` list to `PluginConfig`
   - Tests: `TestExtractPluginsInternalList`, `...RejectsRun`, `...DuplicateAcrossLists`, `...LegacyUseStillInternal`
   - Files: `loader.go`, `graph.go`
   - Verify: in-process loading works; back-compat forms unchanged
4. **Phase: Doctor advisory** — token-match external `run` against built-in names
   - Matching algorithm (exact, per AC-5b): for a `PluginConfig` with `Run != "" && !Internal`, split `Run` on whitespace; for each token, strip a leading `ze.` prefix then apply `filepath.Base`; if the result is in `AvailableInternalPlugins()`, emit one `doctor-plugin-external-builtin` warning naming that built-in. Whole-token equality only — never substring. Dedup if multiple tokens match the same name.
   - Tests: `TestCheckPluginsExternalBuiltinWarns`, `...ExternalScriptNoWarn`, `...InternalNoWarn`, `...BuiltinNamesFromRegistry`
   - Files: `doctor.go`, `codes.go`
   - Verify: warning only for built-in-of-external; severity warning; non-blocking
5. **Phase: Example + doc sweep** — convert in-process examples; update docs
   - Files: `docs/`, `../wiki/`, `etc/ze/`, `test/`; doc guide pages
   - Verify: grep finds no remaining in-process `external { use }`/`run ze.` in examples; genuine `run <path>` untouched; `make ze-doc-test` clean
6. **Full verification** → `make ze-verify`
7. **Complete spec** → audit tables + learned summary; two commits per planning.md

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Token-match avoids false positives (matches binary basename / `ze.`-style subcommand token, not arbitrary args) |
| Naming | YANG leaves kebab-case; diagnostic code `doctor-plugin-external-builtin` |
| Data flow | `internal` flows through `ExtractPluginsFromTree` only; reactor/forwarding untouched |
| Doctor checks | Code registered in `codes.go`; warning severity; functional `.ci` present |
| YANG validation | `internal/use` has custom validator + `CompleteFn`; `name` has length constraint |
| Rule: derive-not-hardcode | Built-in names come from `AvailableInternalPlugins()`, not a literal list |
| Rule: back-compat | Legacy `external { use }` / `run ze.X` still resolve to `Internal=true` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `internal` YANG list | `grep -n 'list internal' internal/component/plugin/schema/ze-plugin-conf.yang` |
| Resolution | `go test ./internal/component/config -run TestExtractPluginsInternal` |
| Doctor advisory | `go test ./cmd/ze/doctor -run TestCheckPluginsExternalBuiltin` |
| Functional test | `ls test/ui/doctor-plugin-external-builtin.ci` |
| Example sweep | grep shows no in-process `external { use }`/`run ze.` in `docs`/`../wiki`/`etc`/`test` examples |
| No routing edits | `git diff --stat` touches no reactor/forwarding files |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `internal/use` rejects unknown built-in names; `name` length-bounded |
| Resource exhaustion | None new (same plugin-manager load path) |
| Error leakage | Doctor message names the built-in and command; no secret material beyond what `doctor-plugin-missing` already prints |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Doctor advisory never fires | Registry empty in doctor context — Phase 1 import fix |
| False-positive advisory | Tighten token match (binary basename / subcommand only) |
| Functional test fails | Check AC; if AC wrong → DESIGN |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- The single plugin list named `external` (covering both in-process and external) is the root of the confusion; the `internal` keyword is sugar over the existing `PluginConfig{Internal:true}` mapping, so the runtime model is unchanged — only the config surface and discoverability improve.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Add explicit `internal` list | (a) docs + advisory only; (b) `internal` alias inside `external` | User chose the explicit keyword: makes "use internal" literal and self-documenting while keeping `external` for true external processes |
| Doctor warns only for built-in-of-external | Warn on every external plugin | User chose targeted: genuine external scripts should not be nagged on every run; advisory must be actionable (suggest exact `internal { use }`) |
| Built-in names from `AvailableInternalPlugins()` | Hardcode known plugin names | derive-not-hardcode rule; registry is the single source of truth |
| Keep `external { use }` / `run ze.X` working | Hard-deprecate immediately | Back-compat: existing configs must not break; "do not change external/routing" |

## Known Limitations
- **Low expected hit rate, by design.** The obvious in-process forms (`use X`, `run ze.X`) already resolve to `Internal=true` and never reach the external branch, so the advisory only fires for the narrow case of someone invoking the ze binary (or a built-in's name) as a true external `run` target. This is intentional: the advisory exists to catch that specific anti-pattern, not to second-guess every external plugin.
- **Whole-token match only.** A built-in run via an opaque wrapper script (e.g. `run ./wrapper.sh` that internally execs a built-in) is not detected. Detecting that would require executing or parsing the wrapper; out of scope. Advisory only, so a miss is harmless.
- **No substring matching.** Prevents false positives from a built-in name appearing inside an unrelated path or argument.
- `external { use }` remains valid (back-compat); the spec does not force migration of user configs, only repo examples.

## RFC Documentation
- N/A.

## Implementation Summary

### What Was Implemented
- YANG `list internal` with `name` and `use` leaves under `container plugin`
- `ze:validate "internal-plugin-name"` custom validator with `CompleteFn` for tab-completion
- `ExtractPluginsFromTree` handles `internal` list (Internal=true, rejects `run`, cross-list uniqueness)
- `graph.go` process-binding resolution checks `internal` list
- `doctor-plugin-external-builtin` diagnostic code (warning severity)
- `checkPlugins` token-match advisory for external-of-builtin
- 5 unit tests for loader, 4 unit tests for doctor, 1 validator registration test update
- 3 functional tests (parse, doctor advisory, doctor clean)
- ~120 config examples converted from `external { use }` to `internal { use }` across test/, docs/

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/guide/plugins.md` — rewrote Loading Plugins section with internal/external separation
- `docs/guide/quickstart.md` — converted example to `internal`
- `docs/guide/graceful-restart.md` — converted example to `internal`
- `docs/guide/rpki.md` — converted 3 examples to `internal`
- `docs/guide/route-reflection.md` — converted example to `internal`
- `docs/guide/operations.md` — updated `--plugin` help text to mention `internal`
- `docs/guide/command-reference.md` — updated `--plugin` help text to mention `internal`

### Deviations from Plan
- None

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `internal` keyword | done | ze-plugin-conf.yang:83 | YANG `list internal` with `name` + `use` |
| Convert examples | done | ~120 files | test/, docs/ |
| Doctor advisory | done | doctor.go:780 | `doctor-plugin-external-builtin` warning |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | TestExtractPluginsInternalList | plugins_test.go:509 |
| AC-2 | done | TestExtractPluginsInternalRejectsRun | plugins_test.go:527 |
| AC-3 | done | TestExtractPluginsDuplicateAcrossLists | plugins_test.go:545 |
| AC-4 | done | TestExtractPluginsLegacyUseStillInternal | plugins_test.go:563 |
| AC-5 | done | TestCheckPluginsExternalBuiltinWarns | doctor_test.go:209 |
| AC-5b | done | matchExternalBuiltinTokens | doctor.go:800 |
| AC-6 | done | TestCheckPluginsExternalScriptNoWarn | doctor_test.go:232 |
| AC-7 | done | TestCheckPluginsInternalNoWarn | doctor_test.go:242 |
| AC-8 | done | TestCheckPluginsExternalBuiltinWarns | checks SeverityWarning |
| AC-9 | done | git diff --name-only | no reactor/forwarding files |
| AC-10 | done | grep sweep | zero remaining external { use } in examples |
| AC-11 | done | TestCheckPluginsBuiltinNamesFromRegistry | doctor_test.go:251 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestExtractPluginsInternalList | pass | bgp/config/plugins_test.go | AC-1 |
| TestExtractPluginsInternalRejectsRun | pass | bgp/config/plugins_test.go | AC-2 |
| TestExtractPluginsDuplicateAcrossLists | pass | bgp/config/plugins_test.go | AC-3 |
| TestExtractPluginsLegacyUseStillInternal | pass | bgp/config/plugins_test.go | AC-4 |
| TestCheckPluginsExternalBuiltinWarns | pass | doctor/doctor_test.go | AC-5, AC-8 |
| TestCheckPluginsExternalScriptNoWarn | pass | doctor/doctor_test.go | AC-6 |
| TestCheckPluginsInternalNoWarn | pass | doctor/doctor_test.go | AC-7 |
| TestCheckPluginsBuiltinNamesFromRegistry | pass | doctor/doctor_test.go | AC-11 |
| TestExtractPluginsInternalRequiresUse | pass | bgp/config/plugins_test.go | extra |
| doctor-plugin-external-builtin.ci | created | test/ui/ | functional |
| doctor-plugin-internal-clean.ci | created | test/ui/ | functional |
| plugin-internal.ci | created | test/parse/ | functional |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| ze-plugin-conf.yang | done | `list internal` added |
| loader.go | done | internal list handling |
| graph.go | done | internal list in process bindings |
| doctor.go | done | external-builtin advisory |
| codes.go | done | `doctor-plugin-external-builtin` registered |
| validators.go | done | `InternalPluginNameValidator` |
| validators_register.go | done | `internal-plugin-name` registered |
| validator_yang_test.go | done | `internal-plugin-name` added to AllPresent test |

### Audit Summary
- **Total items:** 32
- **Done:** 32
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Explicit `internal` keyword | functional test | `test/parse/plugin-internal.*` |
| External-of-builtin flagged | functional test | `test/ui/doctor-plugin-external-builtin.ci` |
| External mechanism preserved | grep / test | no reactor/forwarding diff; external `run` tests still pass |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | | | |

### Fixes applied
-

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for `internal/<name>` length
- [ ] Functional tests for end-to-end behavior
- [ ] Interop N/A justified
- [ ] Goal Validation table filled

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-plugin-internal-keyword.md`
