# Spec: plugin-fork-environment

| Field | Value |
|-------|-------|
| Status | verification |
| Scope | plugin |
| Priority | P2 |
| Release reason | A ze env key an operator or a test sets for a forked plugin silently never reaches it, so the plugin runs on its default with no sign |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-11-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A forked plugin inherits the daemon's environment only through the shell
that runs its `run` string (`(*Process).startExternal`,
`internal/component/plugin/process/process.go`, through `newPluginShell`).
dash drops every variable whose name is not a shell identifier, so a ze key
set in its dotted spelling never reaches the plugin, and a key holding a
hyphen has no spelling that does: `env.Get` (`normalize`,
`internal/core/env/env.go`) maps dots to underscores and keeps the hyphen.
The plugin then reads the key's default and says nothing. Make every
registered ze key the daemon carries reach a forked plugin under a spelling
the plugin's `env.Get` reads, and give a hyphenated key a shell-safe
spelling, or refuse a hyphen in a registered key.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/environment.md` - "OS spellings", how `Get` reads a key
  → Decision: a dot, an underscore and a hyphen are one separator; a dotted spelling wins over the others, then the last entry
  → Constraint: two registered keys that differ only by separator would read one variable, so `MustRegister` refuses the second
- [ ] `ai/doors/system.md`, `ai/doors/core.md`, `ai/doors/plugin-infra.md`, `ai/doors/system-plugins.md`, `ai/doors/le-tooling.md` - the process boundary, env keys as an operator contract
  → Constraint: a forked plugin gets the connect-back `ZE_PLUGIN_*` variables, and the daemon's own value MUST win over an inherited one
  → Constraint: an env key name is an operator contract (core door), so no registered key is renamed by this spec
- [ ] `docs/architecture/plugin/plugin-system.md` - "The process boundary"
  → Decision: a plugin runs internal or forked, and code that works only internal is broken

**Key insights:**
- The fork and the declaration query both start the `run` string through `/bin/sh -c`, so both owe the rewrite.
- ExaBGP reads its settings only as `exabgp.x.y` or lower-case `exabgp_x_y` (`src/exabgp/environment/config.py`, `Environment.setup`, `rep_name = env_name.replace('.', '_')`), so a non-ze registered key keeps lower case.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/process/process.go` - `startExternal` set `p.cmd.Env = os.Environ()`, then appended `PATH` and the `ZE_PLUGIN_*` connect-back variables
- [ ] `internal/component/plugin/process/exitwait_linux.go` - `newPluginShell` runs the `run` string as `plugin.Shell -c`
- [ ] `internal/component/plugin/declarations.go` - `queryChildEnv` copied `os.Environ()` minus the `ze.plugin.` namespace for the declare-mode query
- [ ] `internal/core/env/env.go` - `normalize` mapped `.` to `_` and lower-cased; a hyphen was kept
- [ ] `internal/core/env/registry.go` - `MustRegister` stored a key and its aliases with no check of normalized spellings
- [ ] `internal/plugins/provision/main.go` `normalizeEnvKey`, `internal/test/fixture/fixture.go` `environmentKeyReading` - declared copies of `normalize`
- [ ] `internal/test/runner/runner_exec_util.go` - `testBudgetEnv` and `parallelFactorEnv` write the underscore spelling so a child started through a shell reads them

**Behavior to preserve:**
- `Get` precedence: a dotted spelling wins, then the last entry; aliases resolve to the canonical key
- The connect-back variables the daemon writes are what the plugin reads
- Every non-ze variable reaches the child unchanged, in its place
- The runner's underscore spellings keep working (`TestTestBudgetEnvSurvivesAShell`, `TestParallelFactorEnvSurvivesAShell`)

**Behavior to change:**
- `normalize` reads `-` as `_`
- `MustRegister` panics on a key or alias whose normalized spelling reads another key, and a refused entry leaves no trace
- The live fork and the declaration query pass every ze key under a shell identifier (`env.ShellSafe`)

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| The plugin fork | `(*Process).startExternal`, `internal/component/plugin/process/process.go`; `newPluginShell`, `exitwait_*.go` | Runs the operator's `run` string through `/bin/sh -c` with the daemon's environment plus the `ZE_PLUGIN_*` connect-back variables | Extends: the environment it hands the child is built by `env.ShellSafe`; the shell stays, because an operator's `run` string may use shell syntax |
| The declaration query | `queryChildEnv`, `internal/component/plugin/declarations.go` | Starts the same `run` string in declare mode with the inherited environment minus the plugin namespace | Extends: builds the inherited part through `env.ShellSafe` |
| Env key lookup | `normalize` and `Get`, `internal/core/env/env.go`; `docs/architecture/config/environment.md` | Reads a key in dot or underscore spelling, any case; a hyphen is kept | Extends: `-` is read as `_`, so every registered key has an identifier spelling; `ShellSafe` lives beside `normalize` so it reads names the same way |
| Env registry | `MustRegister`, `internal/core/env/registry.go` | Stores the key, its aliases and prefix patterns | Extends: `claimSpellings` checks every normalized spelling of the entry before recording any |
| Declared copies of `normalize` | `normalizeEnvKey` (provision), `environmentKeyReading` (fixture) | Mirror `normalize` where the package keeps its own reading | Extends: both read `-` as `_` too |
| The runner's shell-safe test keys | `(*Runner).testBudgetEnv`, `internal/test/runner/runner_exec_util.go`, `TestTestBudgetEnvSurvivesAShell` | Writes `ze_test_budget` so a fixture started through a shell reads it | Kept: the runner also starts children through a shell itself, not only through the daemon; comments updated, tests stay as regressions |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The daemon's process environment (operator `export`, systemd unit, `.ci` `option=env`), any spelling of a ze key

### Transformation Path
1. `startExternal` (or `queryChildEnv`) reads `os.Environ()`
2. `env.ShellSafe` renames each ze key to its upper-case normalized name and each other registered key to its lower-case normalized name, one entry per key, the dotted value winning
3. `startExternal` appends `PATH` and the `ZE_PLUGIN_*` variables; exec keeps the last of a duplicated name, so the daemon's own connect-back value wins
4. `/bin/sh -c` passes every identifier-named variable to the plugin
5. The plugin's `env.Get` normalizes the dotted key it asks for and finds the upper-case entry

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Daemon ↔ forked plugin | Process environment through `/bin/sh -c` | `TestForkedPluginReadsEveryZeKeySpelling`, `test/plugin/forked-plugin-reads-dotted-env-key.ci` |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `pkg/`, `cmd/`, and ExaBGP's `src/exabgp/environment/config.py`; terms `cmd.Env`, `os.Environ()`, `newPluginShell`, `env.Get(`, `os.Getenv("ZE_`, `rep_name`. Positive control: `startExternal` found as the writer and `env.Get` as the reader. Every `run` string start goes through `startExternal` or `queryChildEnv`.

| Wire | Role (writer / reader) | Carrier | Source evidence | Migration or preservation disposition |
|------|------------------------|---------|-----------------|----------------------------------------|
| Forked plugin environment | writer | `(*Process).startExternal`, `process.go` | `p.cmd.Env` set before `newPluginShell` starts the run string | Change: `env.ShellSafe(os.Environ())` |
| Forked plugin environment | writer | `queryChildEnv`, `declarations.go` | Child env of the declare-mode query, same run string | Change: `env.ShellSafe(os.Environ())` |
| Forked plugin environment | reader | `env.Get` in every Ze plugin, through `normalize` | Case-insensitive, separator-agnostic lookup | Change: `-` read as `_`; dotted precedence preserved |
| Forked plugin environment | reader | ExaBGP `Environment.setup` (`src/exabgp/environment/config.py`) | Reads `exabgp.x.y` or `env_name.replace('.', '_')`, lower case | Preserve: non-ze registered keys stay lower case |
| Forked plugin environment | reader | A third-party plugin calling `os.Getenv("ZE_...")` | Documented `ZE_*` spelling (`environment.md`) | Preserve: ze keys arrive upper case, the spelling the docs give |

### Integration Points
- `env.ShellSafe` is called from `startExternal` and `queryChildEnv`, both before `PATH` and the connect-back variables are appended

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Both run-string starts build their environment through `env.ShellSafe` |
| No unintended coupling (components stay isolated) | Yes | `process` and `plugin` already import `internal/core/env`; core imports nothing upward |
| No duplicated functionality (extends existing, does not recreate) | Yes | `ShellSafe` uses `normalize` and the registry's spellings; the two declared copies name their source |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Process start, control plane |
| Registration over hardcoding, outbound | Yes | No feature-specific key is named; the ze namespace and the registry decide |
| Registration over hardcoding, inbound | Yes | Searched `startExternal`, `queryChildEnv` and `runner_exec_util.go` for key lists: none; `isRegisteredSpelling` derives from the registry |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No two registered keys collide once `-` reads as `_` | Grep of every `Key:` and `Aliases:` literal (157 keys, 29 hyphenated ze keys) | A binary panics at init | `MustRegister` panics at init in every test binary that links both | confirmed |
| A-2 | ExaBGP reads only dotted or lower-case underscore names | `Environment.setup`, `config.py` | A migrated ExaBGP script loses its settings | Source read | confirmed |
| A-3 | exec keeps the last value of a duplicated name | `os/exec` `dedupEnv` | An inherited hub token wins over the daemon's | `TestForkedPluginReadsEveryZeKeySpelling` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A plugin reading a lower-case `ze_*` name with `os.Getenv` | Third-party report | The documented spelling is `ZE_*`; `env.Get` reads any case |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A forked plugin starts with a wrong key value, or every binary panics at init on a collision |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Forked filter and RR/GR `.ci` tests that set ze keys in the underscore spelling, or pass `--log-level`, as a workaround |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Daemon env `ze.test.fork.environment.dotted` and `ze.test.fork-environment.hyphen-key` with `external fork-environment { run "le test fixture plugin/fork-environment" }` | → | `startExternal` → `env.ShellSafe` → child `env.Get` (`forkEnvironment`, `internal/test/fixture/register_fork_environment.go`) | `test/plugin/forked-plugin-reads-dotted-env-key.ci` |
| `Process.StartWithContext` of an external plugin | → | `startExternal` → `env.ShellSafe` | `TestForkedPluginReadsEveryZeKeySpelling` |
| Declaration query of a `run` string | → | `queryChildEnv` → `env.ShellSafe` | `TestQueryChildEnvSpellsZeKeysForTheShell` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Daemon env holds a dotted ze key; plugin forked | The plugin's environment holds the key once under its upper-case underscore name with the dotted value, and its `env.Get` returns it | Non-ze variables pass unchanged: the plugin and its shell need them |
| AC-2 | Daemon env holds a dotted and hyphenated ze key | The plugin reads it under its upper-case name with `-` as `_` | None: every ze key is renamed |
| AC-3 | `Get` of a hyphenated key set in its underscore spelling | `Get` returns the value | The dotted spelling still wins when both are set, as before |
| AC-4 | `MustRegister` of a key or alias whose normalized spelling reads another key | Panics `BUG:` naming both; the refused entry leaves no registered key and no claimed spelling | Registering one key twice is accepted: a registration can run twice in tests |
| AC-5 | Inherited `ze.plugin.hub.token` in the daemon env | The plugin reads exactly one `ZE_PLUGIN_HUB_TOKEN`, the one the daemon issued | None |
| AC-6 | Daemon env holds a registered non-ze key (`exabgp.*`) | The plugin gets the lower-case underscore form, which ExaBGP reads | None |
| AC-7 | Declaration query of a `run` string | The queried child receives the same shell spellings, and no plugin-namespace key | Plugin-namespace keys stay removed, as before |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator sets a dotted or hyphenated ze key in the daemon env and forks a plugin that reads it | env → `startExternal` → `ShellSafe` → `sh -c` → plugin `env.Get` returns the value | `test/plugin/forked-plugin-reads-dotted-env-key.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGetReadsHyphenAsSeparator` | `internal/core/env/shell_test.go` | AC-3 | pass |
| `TestShellSafeSpellsEveryZeKeyAsAnIdentifier` | `internal/core/env/shell_test.go` | AC-1, AC-2, AC-6 through a real `/bin/sh -c env` | pass |
| `TestMustRegisterRefusesASpellingCollision` | `internal/core/env/shell_test.go` | AC-4 | pass |
| `TestForkedPluginReadsEveryZeKeySpelling` | `internal/component/plugin/process/process_env_test.go` | AC-1, AC-2, AC-5 through the real `startExternal` | pass |
| `TestQueryChildEnvSpellsZeKeysForTheShell` | `internal/component/plugin/declarations_test.go` | AC-7 | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | The feature takes no numeric input | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `forked-plugin-reads-dotted-env-key` | `test/plugin/forked-plugin-reads-dotted-env-key.ci` | A forked observer reads a dotted and a dotted-and-hyphenated ze key that only the daemon's environment holds; red with `env.ShellSafe` removed from `startExternal` | draft pass, red observed, 80-run stress clean; promoted, live run owed |

### Interop Tests (Scope: protocol)
N-A: no wire-visible protocol change; the change is the process environment of a forked plugin.

## Files to Modify
- `internal/core/env/env.go` - `normalize` reads `-`; `ShellSafe`, `shellName`
- `internal/core/env/registry.go` - `spellings`, `claimSpellings`, `isRegisteredSpelling`
- `internal/core/env/env_test.go` - test key registrations
- `internal/component/plugin/process/process.go` - `startExternal` uses `env.ShellSafe`
- `internal/component/plugin/declarations.go` - `queryChildEnv` uses `env.ShellSafe`
- `internal/component/plugin/declarations_test.go` - `TestQueryChildEnvSpellsZeKeysForTheShell`
- `internal/plugins/provision/main.go`, `internal/test/fixture/fixture.go` - declared copies of `normalize`
- `internal/test/runner/runner_exec_util.go` - comments
- `docs/architecture/config/environment.md` - "OS spellings", "The shell spelling"
- `plan/journal/silent-fall-through.md` - the two owned rows
- `test/plugin/community-tag-forked.ci`, `test/plugin/community-strip-forked.ci`, `test/plugin/community-scrub-own-ga-forked.ci`, `test/plugin/community-relation-tag-forked.ci`, `test/plugin/community-blackhole-noexport-forked.ci` - comments only: the claim that the dotted form does not reach the fork

## Files to Create
- `internal/core/env/shell_test.go` - the three env tests
- `internal/component/plugin/process/process_env_test.go` - the live-fork test
- `internal/test/fixture/register_fork_environment.go` - the `plugin/fork-environment` observer and its two test keys
- `test/plugin/forked-plugin-reads-dotted-env-key.ci` - functional test, drafted at `test/draft/plugin/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config change |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | None |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | None |
| Functional test for new RPC/API | N-A | No RPC; the `.ci` above covers the fork |
| Pipe completeness | N-A | No output |
| Env var registration | Yes | No new product key; registration now refuses spelling collisions |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | N-A | None |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | A fix of existing env reading |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | Env spellings are documented in the architecture page |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | Connect-back variables unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | No | Runner comments only |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/environment.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `environment.md` updated. Unaffected, read for this: `docs/architecture/cli/plugin-modes.md` (the query and live start still go through `/bin/sh -c`, and `ZE_PLUGIN_MODE` is already an identifier), `docs/architecture/api/process-protocol.md` (connect-back variable names unchanged), `docs/architecture/api/commands.md` (no command or `environment { }` publication change), `docs/architecture/testing/ci-format.md` (`option=env` still sets the daemon's environment as written; the daemon now carries it to forked plugins). `./le spec citation anchors spec plan/release/fixes/spec-plugin-fork-environment.md` owed at closure |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `environment.md` spelling examples updated |

## Implementation Steps

1. **Phase: Wiring** -- `env.ShellSafe` called from `startExternal` and `queryChildEnv`
   - Tests: `TestForkedPluginReadsEveryZeKeySpelling`, `TestQueryChildEnvSpellsZeKeysForTheShell`
   - Files: `process.go`, `declarations.go`
   - Verify: both tests red with the call removed
2. **Phase: Env reading** -- `normalize` reads the hyphen, `ShellSafe`, `claimSpellings`
   - Tests: the three `shell_test.go` tests
   - Files: `env.go`, `registry.go`, the declared copies
   - Verify: tests fail, implement, tests pass
3. **Phase: Functional proof** -- the `.ci`, drafted, stressed, promoted, with its red recorded
   - Tests: `forked-plugin-reads-dotted-env-key`
   - Files: `test/plugin/forked-plugin-reads-dotted-env-key.ci`
   - Verify: red with `env.ShellSafe` removed from `startExternal`, green restored

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | The user story has a working path, no broken link |
| Correctness | Dotted precedence in `ShellSafe` matches `Get`; the daemon's connect-back value wins over an inherited one |
| Naming | ze keys upper case, non-ze registered keys lower case |
| Data flow | Both run-string starts use `ShellSafe`; no third start exists |
| Rule: no-layering | The runner's underscore spellings stay because the runner also starts children through a shell, not as a fallback |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `ShellSafe` used at both starts | `grep -n "env.ShellSafe" internal/component/plugin/process/process.go internal/component/plugin/declarations.go` |
| Functional test promoted | `ls test/plugin/forked-plugin-reads-dotted-env-key.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secret propagation | `ShellSafe` renames and passes only what the daemon already passed; a Secret key `Get` cleared from the OS environment is not reintroduced |
| Token override | An inherited `ze.plugin.hub.token` cannot win over the token the daemon issued (AC-5) |

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

- Several forked `.ci` tests set ze keys in the underscore spelling, or pass `--log-level` on the run line, because the dotted form did not reach the child (`test/plugin/community-tag-forked.ci`, `test/plugin/addpath-departure-rr-gr-retained-forked.ci`). They keep working; their comments that say the dotted form does not reach the child are stale once this lands and are corrected with this work. The claim stood in five files, the five `community-*-forked.ci` named in Files to Modify. `addpath-departure-rr-gr-retained-forked.ci` says only that the child logs at its run line's `--log-level`, which stays true, and the `role-*-forked.ci` comments say the fork reads the underscore form, which is what they still set: those stay unchanged. Every underscore setting and `--log-level` stays, so no test's behaviour changes.
- A forked filter plugin's decision line does not prove the fork: the `match` condition is compiled and reported in the daemon (`policychain.MatchConditions`, `MustAddMatchReporter` in `filter_aspath_length/register.go`), which reads its own dotted keys, so a `.ci` asserting those lines stayed green with `env.ShellSafe` removed. The functional test reads the keys in a forked observer instead.
- A first version of `claimSpellings` recorded the key's spelling before checking its aliases, so an entry refused for an alias left its own spelling claimed. Every spelling is now checked before any is recorded.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through` | 2026-09-24 | `normalize` keeps a hyphen, so a hyphenated ze key has no shell-identifier spelling |
| `silent-fall-through` | 2026-11-01 | `startExternal` forks through `sh -c`, so a dotted or hyphenated ze key never reaches a forked plugin |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Read `-` as `_` in `normalize` | Ban `-` in registered keys | 29 ze keys are hyphenated and are operator contracts; renaming them changes that contract, reading them breaks nothing |
| Rewrite the child environment (`ShellSafe`) | Exec the run string without a shell | An operator's `run` string may use shell syntax (pipes, `;`, `&`) |
| Upper case for ze keys | Lower case | `ZE_*` is the documented spelling, and an inherited connect-back key renamed to the daemon's appended name is then overridden by it |
| Lower case for other registered keys | Upper case | ExaBGP reads only dotted or lower-case underscore names (`Environment.setup`) |
| Refuse a normalized collision at `MustRegister`, every spelling checked before any is recorded | Accept and let link order decide | One value would answer two keys with no sign, and a refused entry must leave no trace |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le job run label core command go test -race ./internal/core/env/`, `./le job run label plugin command go test -race ./internal/component/plugin/ ./internal/component/plugin/process/`, `./le job run label unit-pkg command go test ./internal/plugins/provision/`, and `./le test bgp plugin --pattern forked-plugin-reads-dotted-env-key`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
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
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-plugin-fork-environment.md` only, in the same `./le commit create` script

## Implementation Summary

### What Was Implemented
- Code landed as c268fa58d4 (`Spec-work: plugin-fork-environment`).
- `ShellSafe` and `shellName` (`internal/core/env/env.go`): every ze key becomes one upper-case underscore entry, every registered key of another namespace one lower-case entry, a dotted spelling winning, then the last entry; every other variable passes unchanged in its place.
- `normalize` reads `-` as `_` through the `separators` replacer (`env.go`).
- `claimSpellings` (`internal/core/env/registry.go`), called first in `MustRegister`, panics `BUG:` on a key or alias whose normalized spelling reads another key, checking every spelling before recording any; `isRegisteredSpelling` answers `shellName` from the registry.
- `startExternal` (`internal/component/plugin/process/process.go`) builds the child environment with `env.ShellSafe(os.Environ())`, before `PATH` and the `ZE_PLUGIN_*` variables are appended.
- `queryChildEnv` (`internal/component/plugin/declarations.go`) builds its inherited part the same way.
- The declared copies of `normalize` read `-` as `_`: `normalizeEnvKey` (`internal/plugins/provision/main.go`), `environmentKeyReading` (`internal/test/fixture/fixture.go`).
- The fixture observer `plugin/fork-environment` (`forkEnvironment`, `internal/test/fixture/register_fork_environment.go`) registers two test keys and reads them back with `env.Get` in the forked process, failing with the key's name when either is missing.
- `test/plugin/forked-plugin-reads-dotted-env-key.ci` sets both keys in the daemon's environment only and asserts the observer's OK line; this commit promotes it from `test/draft/plugin/`.
- Five forked `.ci` comments that said the dotted form does not reach the fork are corrected (comments only).

### Bugs Found/Fixed
- The two owned journal rows (`silent-fall-through`, 2026-09-24 and 2026-11-01): fixed, covered by the tests named in each row.
- `claimSpellings` first recorded the key's spelling before checking its aliases, so a refused entry left a trace; caught in development, covered by `TestMustRegisterRefusesASpellingCollision` (Design Insights).

### Documentation Updates
- `docs/architecture/config/environment.md`: "OS spellings" rewritten (a hyphen is a separator, `MustRegister` refuses collisions) and a new "The shell spelling" paragraph with source anchors, in c268fa58d4.
- `./le doc check all` and `./le spec citation anchors spec plan/release/fixes/spec-plugin-fork-environment.md`: not yet run, owed by the reviewing session.

### Deviations from Plan
- The stale-comment correction reached five `.ci` files, not the two Design Insights first named: `addpath-departure-rr-gr-retained-forked.ci` carried no false claim, and four more `community-*-forked.ci` carried the same false sentence as `community-tag-forked.ci`.
- An earlier draft of the functional test asserted the as-path filter decision lines; it stayed green with the fix removed (Design Insights) and was replaced by the forked observer.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The first functional draft asserted a forked filter's decision lines | The match reporter that writes them runs in the daemon and reads the daemon's own dotted keys | The red walk: the draft stayed green with `env.ShellSafe` removed | Replaced by the `plugin/fork-environment` observer, which reads the keys inside the fork |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every registered ze key the daemon carries reaches a forked plugin under a spelling its `env.Get` reads | Done | `startExternal`, `ShellSafe` | |
| A hyphenated key has a shell-safe spelling (or a hyphen is refused) | Done | `normalize`, `separators` | The spelling route was chosen (Key Design Decisions) |
| The declaration query gets the same | Done | `queryChildEnv` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestForkedPluginReadsEveryZeKeySpelling`, `TestShellSafeSpellsEveryZeKeyAsAnIdentifier`, `forked-plugin-reads-dotted-env-key.ci` | |
| AC-2 | Done | Same three | The `.ci` key `ze.test.fork-environment.hyphen-key` |
| AC-3 | Done | `TestGetReadsHyphenAsSeparator` | |
| AC-4 | Done | `TestMustRegisterRefusesASpellingCollision` | |
| AC-5 | Done | `TestForkedPluginReadsEveryZeKeySpelling` | Asserts one `ZE_PLUGIN_HUB_TOKEN`, not the inherited value |
| AC-6 | Done | `TestShellSafeSpellsEveryZeKeyAsAnIdentifier` | `exabgp.test.shell` registered in `env_test.go` `TestMain` |
| AC-7 | Done | `TestQueryChildEnvSpellsZeKeysForTheShell` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestGetReadsHyphenAsSeparator` | Done | `internal/core/env/shell_test.go` | |
| `TestShellSafeSpellsEveryZeKeyAsAnIdentifier` | Done | `internal/core/env/shell_test.go` | |
| `TestMustRegisterRefusesASpellingCollision` | Done | `internal/core/env/shell_test.go` | |
| `TestForkedPluginReadsEveryZeKeySpelling` | Done | `internal/component/plugin/process/process_env_test.go` | |
| `TestQueryChildEnvSpellsZeKeysForTheShell` | Done | `internal/component/plugin/declarations_test.go` | |
| `forked-plugin-reads-dotted-env-key` | Done | `test/plugin/forked-plugin-reads-dotted-env-key.ci` | Live run after promotion owed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/core/env/env.go`, `registry.go`, `env_test.go`, `shell_test.go` | Done | c268fa58d4 |
| `internal/component/plugin/process/process.go`, `process_env_test.go` | Done | c268fa58d4 |
| `internal/component/plugin/declarations.go`, `declarations_test.go` | Done | c268fa58d4 |
| `internal/plugins/provision/main.go`, `internal/test/fixture/fixture.go` | Done | c268fa58d4 |
| `internal/test/fixture/register_fork_environment.go` | Done | c268fa58d4 |
| `internal/test/runner/runner_exec_util.go` | Done | c268fa58d4, comments only |
| `docs/architecture/config/environment.md` | Done | c268fa58d4 |
| `test/plugin/forked-plugin-reads-dotted-env-key.ci` | Done | Promoted in this commit |
| The five `community-*-forked.ci` | Done | Comments only, this commit |
| `plan/journal/silent-fall-through.md` | Done | Both rows fixed, citing c268fa58d4 |

### Audit Summary
- **Total items:** 26
- **Done:** 26
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the stale-comment file set, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A dotted or hyphenated ze key in the daemon's environment reaches a forked plugin, which reads it with `env.Get` | functional test, red-proven | `./le test bgp plugin --draft --pattern forked-plugin-reads-dotted-env-key` PASS on the HEAD build of le; with `p.cmd.Env = os.Environ()` in `startExternal` it FAILS with "the forked plugin read ze.test.fork.environment.dotted as \"\", want \"dotted\": the daemon's key did not reach it through the shell"; restored, green; `./le test stress-repro run suite "plugin --draft" test forked-plugin-reads-dotted-env-key any-failure iterations 80 parallel 8 burners 8`: not reproduced in 80 invocations |
| The declaration query receives the same spellings | unit test over the real function | `TestQueryChildEnvSpellsZeKeysForTheShell`, green in `go test` of `internal/component/plugin` with full tags |
| A hyphenated key has an identifier spelling, and no two registered keys share one | unit test | `TestGetReadsHyphenAsSeparator`, `TestMustRegisterRefusesASpellingCollision`, green under `go test -race ./internal/core/env/`; every test binary of the plugin and provision packages initialised without a collision panic |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | | |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | owed: round 1 by an independent reviewer in the closing session (`Handoff: verify`) |
| `./le spec review check` | not run |
| Rounds | 0 so far |
| Reviewer lenses used | owed |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | No review round has run yet | - | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/env/shell_test.go` | Yes | in c268fa58d4 (181 lines) |
| `internal/component/plugin/process/process_env_test.go` | Yes | in c268fa58d4 (115 lines) |
| `internal/test/fixture/register_fork_environment.go` | Yes | in c268fa58d4 (54 lines) |
| `test/plugin/forked-plugin-reads-dotted-env-key.ci` | Yes | `ls test/plugin/forked-plugin-reads-dotted-env-key.ci` after the move |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | The fork carries both keys | `grep -n env.ShellSafe` hits `startExternal` in `process.go`; the `.ci` passed as a draft and went red without it |
| AC-3 | `-` reads as `_` | `separators` in `env.go` maps both `.` and `-` to `_`; `TestGetReadsHyphenAsSeparator` green |
| AC-4 | Collision refused, no trace | `claimSpellings` checks every spelling, then records; `TestMustRegisterRefusesASpellingCollision` green |
| AC-5 | One daemon-issued token | `ShellSafe` runs before the `ZE_PLUGIN_HUB_TOKEN` append in `startExternal`; `TestForkedPluginReadsEveryZeKeySpelling` green |
| AC-6 | Non-ze registered key lower case | `shellName` returns the normalized name for a registered non-ze key; `TestShellSafeSpellsEveryZeKeyAsAnIdentifier` green |
| AC-7 | Query child spellings | `queryChildEnv` calls `env.ShellSafe`; `TestQueryChildEnvSpellsZeKeysForTheShell` green |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Daemon env `ze.test.fork.environment.dotted`, `ze.test.fork-environment.hyphen-key`, `external fork-environment { run "le test fixture plugin/fork-environment" }` | `test/plugin/forked-plugin-reads-dotted-env-key.ci` | Read: both keys are set by `option=env` only, the plugin is `external`, and the OK line names both values; `forkEnvironment` reads them with `env.Get` inside the fork |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `MustRegister` panics at init on a collision; `go test` of `internal/core/env`, `internal/component/plugin`, `plugin/process` (full tags) and `internal/plugins/provision` all started and passed |
| A-2 | confirmed | ExaBGP `Environment.setup`, `rep_name = env_name.replace('.', '_')` |
| A-3 | confirmed | `TestForkedPluginReadsEveryZeKeySpelling` asserts the daemon's token, not the inherited one, and passed |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `environment.md` "OS spellings": a dot, an underscore and a hyphen are one separator; `MustRegister` refuses a collision | `separators`, `claimSpellings` | Yes |
| `environment.md` "The shell spelling": ze keys upper case, other registered keys lower case, dotted wins | `ShellSafe`, `shellName` | Yes |
| Forked `.ci` comments no longer claim the dotted form misses the fork | `grep -rn "dotted form does not reach" test/` returns nothing outside drafts | Yes |
