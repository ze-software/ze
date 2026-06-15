# Spec: env-autocomplete

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-06-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `ai/rules/planning.md`
3. `docs/architecture/config/environment.md`
4. `docs/architecture/config/yang-config-design.md`
5. `docs/architecture/api/commands.md`
6. `internal/plugins/env/env.go`
7. `internal/plugins/completion/words.go`
8. `internal/component/command/valuehints.go`
9. `internal/component/cli/client/main.go`

## Task

Autocomplete public environment-variable keys in operational CLI command mode and shell completion, matching the discoverability operators already get from YANG-backed config completion.

In scope:
- `show env get <key>`
- `show env registered [key]`
- `ze env get <key>`
- `ze env registered [key]`
- shell completion entry points that reach those commands

Out of scope:
- shell variable-assignment completion such as `export ZE_*=`
- config-tree `environment { ... }` completion, which already works through YANG
- new env vars or config leaves

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/environment.md` - env registry semantics and public/private surface
  → Decision: env-key autocomplete must use registered metadata as the source of truth and must not read runtime values.
  → Constraint: `Private` entries stay hidden from listing and autocomplete; `Secret` handling must not be triggered by completion.
- [ ] `docs/architecture/config/yang-config-design.md` - what YANG does and does not drive today
  → Decision: keep existing YANG config completion for `environment {}` unchanged; the missing feature is operational and shell completion.
  → Constraint: config completion uses YANG `CompleteFn`, enum, bool, union, and type hints; operational completion uses `command.Node`.
- [ ] `docs/architecture/api/commands.md` - operational command tree, `ArgDef`, and `ValueHints`
  → Decision: runtime env keys belong in command-tree value hints, not a new parser path.
  → Constraint: preserve current positional grammar `show env get <key>` and `show env registered [key]`; do not introduce a selector keyword or YANG arg leaf that changes syntax.
- [ ] `docs/guide/environment-variables.md` - operator-facing env forms and priority
  → Decision: completion should suggest canonical dot-form keys while preserving underscore and uppercase input compatibility.
  → Constraint: OS env > config-block value > default remains unchanged; completion is discoverability only.

### Learned Context
- [ ] `plan/learned/381-shell-completion.md` - shell script generation and helper contract
  → Constraint: shell scripts consume `word<TAB>description` rows, and each shell renders those rows differently.
- [ ] `plan/learned/518-shell-completion-v2.md` - shared `TreeCompleter` for shell and interactive command completion
  → Decision: extend the shared walker, do not add a second env-specific shell walker.
- [ ] `plan/learned/410-validate-completion.md` - `CompleteFn` limits
  → Constraint: `ze:validate` completion does not solve free-form operational arguments like `show env get <key>`.
- [ ] `plan/learned/628-env-cleanup.md` - env surface cleanup and `ApplyEnvConfig`
  → Decision: the env registry is already the canonical public catalog; do not reintroduce a parallel env surface.

**Key insights:**
- Config YANG completion already completes `environment {}` leaves from merged `*-conf.yang` modules. The missing feature is env-key completion on operational and shell surfaces.
- `command.TreeCompleter` already supports dynamic terminal values through `ValueHints`, and `ze completion words` already reuses that path for shell completion.
- `ze completion words show env get` already reaches the existing YANG `show > env > get` node. It returns no env suggestions only because that node has no env-key `ValueHints` yet.
- `ze env list` already exposes one operator-facing catalog: public concrete env entries plus concrete `ze.log.<subsystem>` rows built from `slogutil.Subsystems()`.
- `showOne` in `internal/plugins/env/env.go` still does exact-entry lookup, so concrete log-subsystem keys are listed today but not inspectable by `ze env get`.
- Shared env completion data cannot live in a plugin. Components and sibling plugins both need it, so the catalog must live in a core sibling package that can import `internal/core/env` and `internal/core/slogutil` without violating self-containment.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/env/registry.go` - `EnvEntry`, `Entries()`, `AllEntries()`, alias and prefix registration, `Private`/`Secret` semantics
- [ ] `internal/core/env/env.go` - normalized lookup, alias resolution, typed getters, `Secret` clearing on first `Get()`
- [ ] `internal/plugins/env/env.go` - `ze env` list/get/registered, `showAll`, `showOne`, log-subsystem appendix
- [ ] `internal/plugins/env/yang/ze-env-cmd.yang` - `show env list|get|registered` command tree; no typed env-key leaf
- [ ] `internal/component/cli/completer.go` - YANG config completion, including merged `environment` config nodes
- [ ] `internal/component/command/completer.go` - operational and shell completion via `Children`, `DynamicChildren`, `ValueHints`, and `ArgDefs`
- [ ] `internal/component/command/valuehints.go` - current hardwired family hints for RIB paths only
- [ ] `internal/component/cli/client/main.go` - `BuildCommandTree`, `BuildVerbCommandTree`, runtime tree wiring
- [ ] `internal/plugins/completion/words.go` - shell helper for `show` and `run` contexts only
- [ ] `internal/plugins/completion/bash.go` - generated bash script, hardcoded top-level commands, no `env` branch
- [ ] `internal/plugins/completion/zsh.go` - generated zsh script, no `env` branch
- [ ] `internal/plugins/completion/fish.go` - generated fish script, no `env` branch
- [ ] `internal/plugins/completion/nushell.go` - generated nushell script, no `env` branch
- [ ] `internal/core/slogutil/slogutil.go` - concrete log subsystem list used by `ze env list`

**Partial state today:**
- `ze completion words show env get` and `ze completion words show env registered` can already traverse to the correct YANG command nodes, but those nodes have no env-key `ValueHints`, so the walker returns no env suggestions.
- `ze completion words env ...` cannot work today because `words.go` has no `env` context and the `env` root command has no `command.Node` tree.
- `ze env list` synthesizes concrete `ze.log.<subsystem>` rows with descriptions from `slogutil.SubsystemInfo.Description`, but `showOne()` does not consult that expansion on lookup.

**Behavior to preserve:**
- `ze config completion` and editor config completion continue to complete `environment {}` leaves exactly as today.
- `ze env list` stays sorted, hides `Private` entries, masks current values for `Secret` entries, and remains offline.
- `show env` and `ze env` command grammar stays positional: `... get <key>` and `... registered [key]`.
- Shell env-key completion stays local and offline, with no daemon or SSH requirement.
- Completion never reads current env values.
- Existing concrete keys such as `ze.bgp.openwait`, `ze.pprof`, `ze.web.listen`, and `ze.api-server.rest.enabled` keep their current lookup and display behavior.

**Behavior to change:**
- Operational command completion offers env keys after `show env get ` and `show env registered ` by attaching env-key `ValueHints` to the existing YANG nodes.
- Shell completion offers env keys after `ze env get`, `ze env registered`, `ze show env get`, and `ze show env registered`.
- Generated bash, zsh, fish, and nushell scripts expose the `env` root command through a registry-derived root inventory, not a fifth hardcoded list.
- Concrete `ze.log.<subsystem>` keys that appear in `ze env list` become inspectable and completable instead of list-only.

## Data Flow (MANDATORY)

### Entry Point
- Interactive CLI, operational mode: user types `show env get ` or `show env registered ` and presses Tab.
- Shell completion: shell script calls back into `ze completion words ...` while the user tabs through `ze env ...` or `ze show env ...`.
- Shared metadata source: env registrations arrive through `env.MustRegister`, and log-subsystem names arrive through `slogutil.Logger` / `LazyLogger` registration.

### Transformation Path
1. Packages register env entries in `internal/core/env` and log subsystems in `internal/core/slogutil`.
2. A new core sibling package, `internal/core/envcatalog`, builds the shared public catalog from `env.Entries()` plus concrete `ze.log.<subsystem>` rows from `slogutil.Subsystems()`. Concrete env entries use `EnvEntry.Description`; concrete log-subsystem rows use `slogutil.SubsystemInfo.Description`.
3. `BuildCommandTree` and `BuildVerbCommandTree` already build the YANG `show > env > get|registered` nodes.
4. `wireEnvHints(tree)` is added beside `wireRibHints(tree)` in `internal/component/command/valuehints.go`, adapting `envcatalog` rows into `command.Suggestion` values and attaching them to those existing show nodes.
5. `completionTree()` in `internal/plugins/completion/words.go` gains an `env` case that builds the tiny offline `command.Node` tree locally inside the completion plugin. The tree shape is static (`list`, `get`, `registered`), and the `get` / `registered` nodes use `envcatalog` for `ValueHints`, so no plugin-to-plugin import is needed.
6. Interactive CLI command mode and `ze completion words show ...` both call `command.TreeCompleter.Complete` on the show tree, so they reuse the new env hints without new grammar work.
7. Shell `ze env ...` completion calls `ze completion words env ...`, which walks the local env mini tree through the same `TreeCompleter`.
8. After the operator accepts a completed key, env inspection first checks concrete `env.AllEntries()` keys. If none match and the key starts with `ze.log.`, `internal/plugins/env/env.go` uses the shared `envcatalog` concrete-log lookup helper to resolve the suffix against `slogutil.Subsystems()` and reuse that subsystem description in the output.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Env registry + subsystem registry -> shared env catalog | `internal/core/envcatalog` merges `EnvEntry` rows with concrete subsystem rows | [ ] |
| Env catalog -> operational CLI completion | `command.Node.ValueHints` adapters in `wireEnvHints(tree)` | [ ] |
| Env catalog -> shell completion | `ze completion words env|show ...` tab-separated rows | [ ] |
| Completed key -> env inspection handler | existing local/root handler dispatch plus `envcatalog` concrete-log lookup helper | [ ] |

### Integration Points
- `internal/core/env/registry.go` - authoritative public/private env metadata
- `internal/core/slogutil/slogutil.go` - concrete subsystem names for `ze.log.<subsystem>` expansion
- `internal/core/envcatalog` - shared public env catalog and concrete log-key lookup helper
- `internal/component/command/valuehints.go` - shared command completion wiring point
- `internal/component/cli/client/main.go` - operational command-tree build path used by CLI and shell `show`
- `internal/plugins/completion/words.go` - shared shell callback surface and local env mini tree
- `internal/plugins/env/env.go` - exact lookup, display formatting, and root command behavior

### Architectural Verification
- [ ] No bypassed layers: `show env` still flows through command trees, `ze env` still flows through its root handler, shell still flows through `ze completion` helpers.
- [ ] No unintended coupling: one env-key catalog feeds all completion surfaces.
- [ ] No duplicated functionality: shell scripts and interactive CLI do not each invent their own env-key list.
- [ ] Zero-copy preserved where applicable: this is a control-path feature, but completion stays metadata-only and never reads env values.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `command.TreeCompleter` can surface env keys at `show env get` and `show env registered` without grammar changes | `internal/component/command/completer.go` `matchChildren()` already merges `ValueHints` with static children | Need a new operational-argument completion path | Read `matchChildren()` plus existing RIB family hints | confirmed |
| A-2 | `ze completion words` can grow an `env` context by walking a tiny offline env command tree | `internal/plugins/completion/words.go` already builds local trees for `show` and `run`; adding one more context is mechanical | Each shell would need its own env helper logic | Read `completionTree()` and `writeWords()` | confirmed |
| A-3 | Completion must never call `env.Get` or any typed getter | `internal/core/env/env.go` clears `Secret` OS env on first `Get()` | Completion could mutate the environment and leak state | Read `Get()` secret-clearing path | confirmed |
| A-4 | Concrete `ze.log.<subsystem>` keys are the only user-visible dynamic expansion missing from `env.Entries()` | `internal/plugins/env/env.go` appends subsystem rows; angle-bracket search shows only private wildcard registrations for log and legacy `ze.bgp.*` | Catalog design needs a generic prefix-expansion API, not a log-only expansion | Search env registrations with angle-bracket keys and read `showAll()` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Registry-derived shell root inventory touches all four shell generators and can hide env-specific bugs under template churn | Shell-script tests fail before env-key tests reach `get` / `registered` | Make root inventory derivation its own implementation phase with dedicated tests, including a regression test that all pre-migration root commands survive |
| R-2 | Placing the shared env catalog in a plugin would invert dependencies and break plugin self-containment | `internal/component/command` or `internal/plugins/completion` needs to import `internal/plugins/env` | Put the shared catalog and concrete-log lookup helper in `internal/core/envcatalog`, and keep plugin-specific tree construction local to each caller |
| R-3 | A new value-hint registration abstraction expands scope beyond the env feature | New helper registries or generic provider plumbing appear in the diff | Follow the existing pattern: add `wireEnvHints(tree)` beside `wireRibHints(tree)` and stop there |
| R-4 | Private or secret entries leak through shell rows | Tests show `ze.plugin.*` or masked values in completion output | Filter from `env.Entries()`, never surface current values, and add explicit negative tests |

## Design Approach

### Option A, recommended: core env catalog + show-tree env hints + local env mini tree + registry-derived shell roots
Create `internal/core/envcatalog` as the shared data source for public env rows and concrete `ze.log.<subsystem>` lookup. For `show env ...`, attach env-key `ValueHints` to the existing YANG nodes by adding `wireEnvHints(tree)` beside `wireRibHints(tree)`. For `ze env ...`, extend `ze completion words` with an `env` context that builds the tiny offline env tree locally inside the completion plugin from the same core catalog. Keep shell root discovery in scope and derive the root inventory from the command registry in a separate shell-wiring phase, because Ze does not permit adding another hardcoded command list.

Why it wins:
- Reuses the existing `TreeCompleter` path for both interactive CLI and shell completion.
- Keeps one env-key source of truth, including concrete log-subsystem rows and descriptions, in a package importable by components and plugins.
- Keeps the `show` path minimal because the YANG tree already exists there.
- Keeps the `ze env` path explicit without introducing plugin-to-plugin imports.
- Fixes the current `ze env list` versus `ze env get` mismatch for concrete log-subsystem keys.
- Satisfies the auto-discovery rule for shell root commands instead of deepening hardcoded drift.

### Option B: YANG command arg leaf + `ze:validate` completion
Add a typed leaf under `show env get` / `registered` and a custom validator that returns env keys.

Why reject it:
- Current operational grammar is positional. Adding a leaf pushes toward `show env get key <name>` or a misleading string arg name completion.
- `CompleteFn` is the wrong tool for shell `ze env ...` root completion.
- It solves the YANG side while leaving shell and `ze env` as separate paths.

### Option C: shell-only helper plus ad hoc CLI glue
Add per-shell env-key callbacks and special-case `show env` or `ze env` in CLI code.

Why reject it:
- Duplicates private/secret filtering logic across five surfaces.
- Makes drift likely any time the env catalog changes.
- Produces a second completion convention beside `TreeCompleter`.

**Strongest concern:** the shared env catalog must stay in a core sibling package, and both completion paths must consume that same data. If a plugin-local helper leaks into the command component or sibling completion plugin, the design violates Ze's dependency rules even if the feature works.

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Shell generator renders top-level commands | -> | registry-derived root inventory includes `env` across all shells | `TestShellScriptsExposeEnvRootFromRegistry` |
| Operational CLI user types `show env get ` | -> | existing show tree plus env-key `ValueHints` | `TestBuildCommandTreeEnvValueHints`, `test/editor/completion/operational-env-get.et` |
| Shell helper runs `ze completion words env get` | -> | env mini tree backed by shared env catalog | `TestWordsEnvGetIncludesPublicKeys`, `test/ui/completion-words-env.ci` |
| Shell helper runs `ze completion words show env get` | -> | existing show tree plus env-key `ValueHints` | `TestWordsShowEnvGetIncludesPublicKeys`, `test/ui/completion-words-show-env.ci` |
| Operator chooses `ze.log.<subsystem>` from completion | -> | shared catalog row plus log-subsystem fallback lookup | `TestShowOneFindsConcreteLogSubsystem`, `test/ui/cli-env-get-log-subsystem.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Operational CLI command mode, input `show env get ` or `show env registered ` | Completion shows sorted public env keys, excludes `Private` entries, and keeps existing command grammar unchanged |
| AC-2 | `ze completion words env get` or `ze completion words env registered` | Output uses `word<TAB>description` rows from the shared env catalog, with no value reads and no private keys |
| AC-3 | `ze completion words show env get` or `ze completion words show env registered` | Output matches the same env-key catalog used by interactive CLI command mode |
| AC-4 | Generated bash, zsh, fish, and nushell scripts | `env` appears as a completable root command through registry-derived root discovery, and `get` / `registered` call a dynamic env helper rather than embedding env keys in the script text |
| AC-5 | `ze env get <key>` and `ze env registered <key>` on an existing concrete key | Current dot and underscore forms still work exactly as before |
| AC-6 | `ze env get ze.log.<subsystem>` for a subsystem shown by `ze env list` | Command succeeds and shows metadata instead of returning `unknown env var` |
| AC-7 | Any completion scenario in this spec | Completion never reads current env values, never clears `Secret` OS env, and never exposes `Private` entries |
| AC-8 | Existing config completion under `environment {}` | Behavior remains unchanged |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | In interactive CLI command mode, types `show env get ` and presses Tab | editor operational mode -> show command tree -> env-key `ValueHints` -> dropdown | `test/editor/completion/operational-env-get.et` |
| 2 | In a shell, types `ze env get <Tab>` | shell root inventory from registry -> `ze completion words env get` -> env mini tree -> shared env catalog | `test/ui/completion-words-env.ci` |
| 3 | In a shell, types `ze show env get <Tab>` | shell root inventory from registry -> `ze completion words show env get` -> existing show tree -> env-key `ValueHints` | `test/ui/completion-words-show-env.ci` |
| 4 | Selects a concrete log subsystem key and inspects it | shared env catalog row with `SubsystemInfo.Description` -> `ze env get ze.log.<subsystem>` -> log-subsystem fallback lookup -> display | `test/ui/cli-env-get-log-subsystem.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVisibleEntriesExcludePrivate` | `internal/core/envcatalog/catalog_test.go` | Shared env catalog hides `Private` entries | |
| `TestVisibleEntriesIncludeConcreteLogSubsystems` | `internal/core/envcatalog/catalog_test.go` | Catalog expands `ze.log.<subsystem>` rows from `slogutil.Subsystems()` and uses subsystem descriptions | |
| `TestShowOneFindsConcreteLogSubsystem` | `internal/plugins/env/env_test.go` | `ze env get` resolves a concrete log-subsystem key through the shared lookup helper | |
| `TestBuildCommandTreeEnvValueHints` | `internal/component/cli/client/main_test.go` | Existing `show env get` and `show env registered` nodes get env hints | |
| `TestWordsEnvGetIncludesPublicKeys` | `internal/plugins/completion/words_test.go` | `env` context in `ze completion words` emits public env keys through the local env mini tree and shared catalog | |
| `TestWordsShowEnvGetIncludesPublicKeys` | `internal/plugins/completion/words_test.go` | Existing show-tree env path reuses shared env hints | |
| `TestShellScriptsExposeEnvRootFromRegistry` | `internal/plugins/completion/main_test.go` | Generated shell roots are derived from the registry and include `env` | |
| `TestShellRootMigrationPreservesExistingCommands` | `internal/plugins/completion/main_test.go` | Registry-derived root discovery preserves the pre-migration root command set across shell generators | |
| `TestShellScriptsUseDynamicEnvHelper` | `internal/plugins/completion/main_test.go` | Generated shell scripts call `ze completion words env` for `get` and `registered` | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A, no numeric input surface is introduced.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `completion-words-env` | `test/ui/completion-words-env.ci` | `ze completion words env get` returns env-key rows | |
| `completion-words-show-env` | `test/ui/completion-words-show-env.ci` | `ze completion words show env get` returns env-key rows | |
| `cli-env-get-log-subsystem` | `test/ui/cli-env-get-log-subsystem.ci` | Completed `ze.log.<subsystem>` key is inspectable through `ze env get` | |
| `operational-env-get` | `test/editor/completion/operational-env-get.et` | Interactive CLI dropdown offers env keys in operational mode | |

### Interop Tests (MANDATORY for protocol features)
N/A, not protocol work.

### Future (if deferring any tests)
- None planned. If concrete `ze.log.<subsystem>` support is cut, that is a scope change and needs explicit user approval.

## Files to Modify
- `internal/component/command/valuehints.go` - add `wireEnvHints(tree)` beside `wireRibHints(tree)`; adapt `envcatalog` rows to `command.Suggestion`
- `internal/component/cli/client/main_test.go` - verify existing show-tree nodes get env hints
- `internal/plugins/env/env.go` - reuse the shared `envcatalog` lookup helper for concrete log-subsystem keys and keep display behavior aligned with the catalog
- `internal/plugins/completion/words.go` - add `env` context and build the local env mini tree from `envcatalog`
- `internal/plugins/completion/bash.go` - consume the shared root inventory and dynamic env callback
- `internal/plugins/completion/zsh.go` - consume the shared root inventory and dynamic env callback
- `internal/plugins/completion/fish.go` - consume the shared root inventory and dynamic env callback
- `internal/plugins/completion/nushell.go` - consume the shared root inventory and dynamic env callback
- `internal/plugins/completion/main_test.go` - shell-generation coverage for `env` and preserved root-command inventory
- `internal/plugins/completion/words_test.go` - env words coverage
- `internal/plugins/env/env_test.go` - concrete log-key lookup coverage
- `docs/guide/command-reference.md` - document env-key completion surfaces and shell behavior
- `docs/guide/environment-variables.md` - document canonical completion form and concrete log-subsystem inspection
- `docs/architecture/config/environment.md` - update the documented autocomplete/discoverability contract if the implementation exposes concrete log-subsystem keys through inspection and completion

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | No grammar or schema change planned |
| YANG validation constraints | No | No new config leaf |
| YANG custom validators | No | Rejected for this feature; operational completion will use command value hints |
| CLI commands/flags | Yes | `internal/plugins/env/env.go`, `internal/plugins/completion/*.go` |
| CLI grammar (action before identifier) | No | Preserve existing `get <key>` grammar exactly |
| Editor autocomplete | Yes | Operational command mode via `command.TreeCompleter` and `test/editor/completion/operational-env-get.et` |
| Functional test for new RPC/API | Yes | `test/ui/completion-words-env.ci`, `test/ui/completion-words-show-env.ci`, `test/ui/cli-env-get-log-subsystem.ci` |
| Pipe completeness | No | No new output command or pipe surface |
| Env var registration | No | No new env vars |
| Doctor check for runtime dependencies | No | No new external dependency |
| Prometheus counters/metrics | No | No telemetry surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/command-reference.md`, `docs/guide/environment-variables.md` |
| 2 | Config syntax changed? | No | No |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | No | No |
| 5 | Plugin added/changed? | No | No new plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/environment-variables.md` |
| 7 | Wire format changed? | No | No |
| 8 | Plugin SDK/protocol changed? | No | No |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | Existing `.ci` and `.et` infrastructure only |
| 11 | Affects daemon comparison? | No | No |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/environment.md` if the public autocomplete contract changes |
| 13 | Route metadata keys added/changed? | No | No |
| 14 | Prometheus counters added/changed? | No | No |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | Yes | `docs/guide/command-reference.md` |
| 16 | Any changed source file is referenced by existing doc source anchors? | Yes | Re-check `docs/architecture/config/environment.md`, `docs/guide/command-reference.md`, `docs/guide/environment-variables.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/command-reference.md`, `docs/guide/environment-variables.md` |

## Files to Create

- `internal/core/envcatalog/catalog.go` - shared public env catalog and concrete log-subsystem lookup helper
- `internal/core/envcatalog/catalog_test.go` - catalog coverage for public filtering and subsystem descriptions
- `internal/plugins/completion/root_commands.go` - shared registry-derived shell root inventory built from `registry.ListRoot()`
- `test/editor/completion/operational-env-get.et` - interactive CLI command-mode env completion coverage
- `test/ui/completion-words-env.ci` - shell helper coverage for `ze env get`
- `test/ui/completion-words-show-env.ci` - shell helper coverage for `ze show env get`
- `test/ui/cli-env-get-log-subsystem.ci` - concrete log-subsystem inspection coverage

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review checklist below |
| 6. Full verification | Targeted Go tests, `.ci`, and `.et` tests for touched files |

### Implementation Phases
1. **Phase: Shell root auto-discovery**
   - Tests: `TestShellScriptsExposeEnvRootFromRegistry`, `TestShellRootMigrationPreservesExistingCommands`
   - Files: `internal/plugins/completion/root_commands.go`, `internal/plugins/completion/bash.go`, `internal/plugins/completion/zsh.go`, `internal/plugins/completion/fish.go`, `internal/plugins/completion/nushell.go`, `internal/plugins/completion/main_test.go`
   - Verify: all shell generators take their visible root commands from one `registry.ListRoot()`-backed helper, and no pre-migration root command disappears in the conversion
2. **Phase: Show-tree env hints**
   - Tests: `TestBuildCommandTreeEnvValueHints`, existing family-hint tests
   - Files: `internal/component/command/valuehints.go`, `internal/component/cli/client/main_test.go`, `internal/core/envcatalog/catalog.go`
   - Verify: `show env get` and `show env registered` already-existing YANG nodes now return env-key hints in both interactive CLI and `ze completion words show ...`
3. **Phase: Shared env catalog, local env mini tree, and lookup**
   - Tests: `TestVisibleEntriesExcludePrivate`, `TestVisibleEntriesIncludeConcreteLogSubsystems`, `TestShowOneFindsConcreteLogSubsystem`, `TestWordsEnvGetIncludesPublicKeys`, `TestWordsShowEnvGetIncludesPublicKeys`
   - Files: `internal/core/envcatalog/catalog.go`, `internal/core/envcatalog/catalog_test.go`, `internal/plugins/env/env.go`, `internal/plugins/completion/words.go`, `internal/plugins/completion/words_test.go`, `internal/plugins/env/env_test.go`
   - Verify: `ze completion words env ...` works through the local env mini tree, descriptions are stable, and completed log-subsystem keys round-trip through inspection through the shared lookup helper
4. **Phase: Interactive and end-to-end tests**
   - Tests: `test/editor/completion/operational-env-get.et`, `test/ui/completion-words-env.ci`, `test/ui/completion-words-show-env.ci`, `test/ui/cli-env-get-log-subsystem.ci`
   - Files: test files plus any final wiring adjustments
   - Verify: operator-visible completion works in command mode and shell, and concrete completed log keys round-trip through inspection
5. **Phase: Docs and verification**
   - Tests: targeted reruns for touched Go tests, `ze-test ui`, `ze-test editor -p operational-env-get` or exact new test id, and doc checks if docs changed
   - Files: command reference, env guide, architecture env doc if needed
   - Verify: docs match actual completion behavior and source anchors stay correct

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 through AC-8 each map to code and a passing test |
| Feature completeness | `show env get`, `show env registered`, `ze env get`, `ze env registered`, shell root discovery, and all four shell generators are covered |
| Correctness | Completed keys are inspectable, sorted, public-only, metadata-only, and concrete log-subsystem descriptions come from `SubsystemInfo.Description` |
| Naming | Suggestions use canonical dot-form keys, without duplicate underscore variants |
| Data flow | `show env ...` reuses the existing YANG tree; `ze env ...` uses an explicit env mini tree; both consume the same env catalog |
| CLI grammar | No grammar change, no inserted selector keyword |
| Rule: derive-not-hardcode | Shell root inventory is derived from the command registry through one helper; env-key catalog is not copied into scripts |
| Rule: no-layering | No second env completion path beside the shared walker/value-hint path |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Shared env completion catalog | Go unit test for sorted public entries, subsystem descriptions, and concrete log-subsystem expansion in `internal/core/envcatalog/catalog_test.go` |
| Inspectable completed log key | `test/ui/cli-env-get-log-subsystem.ci` |
| Operational CLI env-key completion | `test/editor/completion/operational-env-get.et` |
| Shell helper env context | `test/ui/completion-words-env.ci` |
| Show-tree env-key shell completion | `test/ui/completion-words-show-env.ci` |
| All four shell scripts expose `env` from registry-derived root discovery without losing existing roots | `internal/plugins/completion/main_test.go` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Private-key exposure | `Private` entries never appear in CLI or shell completion output |
| Secret handling | No completion path calls `env.Get`, `GetBool`, `GetInt`, or any value lookup helper |
| Log-subsystem expansion | Only actual registered subsystem names are exposed, not arbitrary `ze.log.*` suffixes |
| Output sanitization | Shell helper descriptions stay single-line and tab-safe |
| Resource bounds | Completion stays local, bounded, and does not perform SSH or daemon I/O |

## Known Limitations
- Completion suggestions should use canonical dot-form keys only. Lowercase underscore and uppercase underscore forms remain accepted input, but they should not appear as duplicate suggestions.
- Shell completion covers Ze command arguments, not shell variable-assignment syntax such as `export ZE_FOO_BAR=`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories all have a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] Targeted Go tests, `.ci`, and `.et` tests pass for all touched files
- [ ] Feature code integrated without changing existing command grammar
- [ ] Documentation Update Checklist answered with source evidence

### Quality Gates (SHOULD pass)
- [ ] Shell root discovery is registry-derived and shared across all shell generators
- [ ] Env hint wiring follows the existing `wireRibHints` pattern, with no new registration abstraction

### Design
- [ ] No premature abstraction beyond the shared shell-root helper and the shared core env-catalog helper needed to avoid duplicated logic
- [ ] No speculative features outside env-key completion and shell-root auto-discovery already required by Ze rules
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Editor test for interactive command-mode behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-env-autocomplete.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-env-autocomplete.md`
