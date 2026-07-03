# Spec: cli-grammar-gate

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-command-naming (absorbed + closed by this spec) |
| Phase | 1/8 |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `ai/rules/cli-grammar.md` - the grammar rules being mechanized
3. `ai/rules/cli-patterns.md` - CLI patterns + completion tree sources
4. `docs/architecture/cli/command-namespacing.md` - filters-are-keywords, legitimate namespaces
5. `internal/component/config/yang/command.go` - BuildCommandTree, GetCommandExtension
6. `internal/component/command/node.go` - command.Node, ArgDef, ArgKind
7. `internal/component/plugin/server/command_registry.go` - commandVerbs, validateCommandName
8. `plan/learned/829-command-verb-first.md` - the agreed 8 verbs; notes this gate was never built

## Task

Build a mechanical enforcement gate that makes the CLI command-syntax rules in
`ai/rules/cli-grammar.md` regression-proof. Today those rules are prose only;
nothing checks them, so every agent that adds a command can (and does) drift from
verb-first, keyword-before-value, typed-selector, and no-`--flag`-in-YANG grammar.

The gate walks the **whole CLI command surface** (built-in YANG commands + plugin
`CommandDecl` commands) and fails if any command violates the agreed grammar. It
ships **green** by fixing every current violation in one sweep (absorbing and
closing `spec-command-naming`), leaving only principled **category exemptions**
(ExaBGP bridge, plugin/system wire protocol, editor modes).

The grammar itself is reverse-engineered from three rule docs and the running code
into an explicit eight-rule set (R1-R8, see Appendix), so future agents have a single
concrete definition instead of scattered prose. The verb vocabulary becomes one
canonical registry (today it is defined inconsistently in three places).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint: annotations. -->
- [ ] `ai/rules/cli-grammar.md` - the BLOCKING grammar rule this gate mechanizes
  → Constraint: grammar is `<verb> <noun> <action> [args]`; first token after the noun MUST be a closed compile-time keyword; free-form values never sit in an untyped positional slot.
  → Constraint: member selection needs a typed selector kind (`name`, `id`, `index`, `address`, `type`, `key`, ...). Peer commands (`show bgp peer <name|address>`) are the ONE documented untyped-selector exception.
  → Constraint: named-resource `<resource> <id> <action>` is banned; correct is `<resource> <action> <id>` (`cache retain <id>`, not `cache <id> retain`).
  → Constraint: config-tree objects mutate only via engine `set <path> <value>` / `delete <path>`; no operational `add`/`del`/`create`/`remove` verbs for anything already in the config YANG tree.
  → Constraint: identifiers are string-typed even when numeric (avoids numeric-keyword ambiguity).
  → Constraint: `--flag` syntax MUST NOT appear anywhere in a `.yang` file - not in a leaf, a `description`, or a `//` comment. Mechanical check: `grep -rnE '\-\-[a-z]' internal --include='*.yang'` minus `urn:|http|xml` must be empty.
  → Constraint: "Applies To" - rules cover online (YANG dispatch) AND offline (`cmd/ze/`) commands; no exceptions for "simple" commands.
  → Decision: some noun-first forms are deliberate namespaces and MUST be kept - `plugin encoding/format/ack`, and the `command`/`system` introspection group. These become the gate's category exemptions.
- [ ] `ai/rules/cli-patterns.md` - New Command Checklist, completion tree sources
  → Constraint: completion tree is built from TWO sources - YANG command schemas (`BuildCommandTree`) and the plugin `CommandRegistry`. The gate must cover both.
  → Constraint: `--flag` forms (`--json`, `--socket`) are a legitimate presentation artifact of the offline `cmd/ze/` `flag.NewFlagSet` tooling ONLY - never the YANG/CLI command grammar.
  → Constraint: "Known gap: 51 commands across 12 plugins lack completion because the plugin registry does not feed the completion tree" is STALE - `inject.go` now injects plugin commands into the runtime tree. Fix this doc line.
- [ ] `docs/architecture/cli/command-namespacing.md` - object-rooted, filters-are-keywords
  → Decision: Ze is object-rooted with family-as-filter (no `show ip`/`show ipv6` split). A filter (family, limit, vrf) is a YANG keyword selector, never a `--flag`.
  → Constraint: `arp` is an intentional IPv4-only shortcut for `show neighbor ipv4`; the gate must not flag it.
- [ ] `ai/rules/discovery-updates.md` - what a new verification gate must update
  → Constraint: a new gate MUST update `ai/rules/hook-mapping.md`, the rule it enforces (`cli-grammar.md`), the make-target docs, `ai/INDEX.md` (keyword + dev-tools rows), and add a `plan/learned/NNN-*.md`.
- [ ] `ai/rules/derive-not-hardcode.md` - single source of truth for enumerated data
  → Constraint: the verb vocabulary must have ONE canonical source; every other use (error strings, the plugin gate, the new gate) derives from it.
- [ ] `ai/rules/module-tiers.md` - package placement
  → Constraint: `internal/component/command` is imported by `internal/component/plugin/server` and does NOT import it back (verified) - a cycle-free home for the shared verb registry and checker.

### RFC Summaries (MUST for protocol work)
N/A - not protocol work. No wire-format change.

**Key insights:** (minimal context to resume after compaction)
- The keyword-vs-value distinction is STRUCTURAL, not naming-based: YANG `container` (config false) → `command.Node.Children` = closed keyword tokens (verified: zero `list` nodes in any `-cmd.yang`); YANG `leaf` → `command.Node.ArgDefs` = value slots. `ArgEnum` values are themselves closed keywords; `ArgString`/`ArgUint` leaf names are the selector keyword guarding a free-form value.
- The full built-in command tree is obtainable in-process, no daemon: `yang.BuildCommandTree(yang.DefaultLoader())` after blank-importing `internal/component/plugin/all`. This is how `scripts/inventory/commands.go` already enumerates commands.
- The full RUNTIME merged surface (built-ins + started plugins) exists only via the `ze-system:command-list` RPC from a booted daemon; it returns command PATHS only (`{value, help, hidden, source}`), NOT the `ArgDef` structure. So deep structural checks need the compile-time tree; runtime-only plugin commands need the RPC.
- The verb vocabulary is defined in three disagreeing places today: `plan/learned/829` (8 verbs: show, monitor, clear, set, request, resolve, commit, update), `command_registry.go:59` `commandVerbs` (7: show, set, clear, request, commit, cache, update), and the live inventory (also uses delete, create, config, ...).

## Current Behavior (MANDATORY)

**Source files read:** (read BEFORE writing this spec)
- [ ] `ai/rules/cli-grammar.md` - grammar rules (prose only; no code enforces them)
  → Constraint: preserve every rule verbatim; the gate mechanizes, it does not reinterpret.
- [ ] `internal/component/config/yang/command.go` - `BuildCommandTree` (line 170), `mergeYANGEntry` (225), `extractArgDefs` (284), `GetCommandExtension` (404)
  → Constraint: `mergeYANGEntry` recurses ONLY into `config false` containers (line 233); leaves become `ArgDefs`. This IS the keyword/value ground truth.
  → Constraint: the command tree is built from `-cmd` suffixed modules (`cmdModuleSuffix`, line 21).
- [ ] `internal/component/command/node.go` - `Node` (line 45), `ArgDef` (32), `ArgKind` (14)
  → Constraint: `ArgDef.Name` is documented "kebab-case, used as keyword detector" (line 33); `ArgKind` ∈ {ArgString, ArgEnum, ArgUint, ArgUnion}.
- [ ] `internal/component/command/completer.go` (lines 304-328) - authoritative keyword/value reading
  → Constraint: enum values are emitted as closed keyword completions; `ArgString`/`ArgUint` leaf name is emitted as the keyword preceding the free-form value. The checker must classify value-slots the same way.
- [ ] `internal/component/plugin/server/command_registry.go` - `commandVerbs` (59), `validateCommandName` (86), `validateCommandToken` (111)
  → Constraint: `validateCommandName` runs for EVERY plugin-registered `CommandDecl` (not built-ins); it already enforces token form + verb-first, but against the stale 7-verb `commandVerbs`. This is the registration-time hook to strengthen.
  → Constraint: `validVerbList()` (line 71) already derives the error string from `commandVerbs` - keep that derive-not-hardcode discipline when swapping in the canonical registry.
- [ ] `internal/component/plugin/server/system.go:301` - `handleSystemCommandList`
  → Constraint: emits builtins (`Dispatcher().Commands()`) + plugin (`Registry().All()`) as `{value, help, hidden, source}`; `source` only when first arg == `"verbose"`. This is the ONLY merged runtime surface; it has no `ArgDef` structure.
- [ ] `scripts/checks/command_ownership.go` + `scripts/checks/checks_test.go` - prior-art gate pattern
  → Constraint: repo-wide invariant gates are `//go:build ignore` `main` programs run via `go run`, wired to a `mk/inventory.mk` target and into `_ze-verify-impl`, with a `checks_test.go` smoke test asserting an `"<name>: OK"` line. Follow this exactly for the static gate.
- [ ] `internal/test/runner/runner.go` (50, 145, 216, 227) + `internal/test/cli/register.go` + `mk/test-functional.mk:57`
  → Constraint: functional tests are `.ci` files under `test/<suite>/`, run as `bin/ze-test <suite>` against a daemon SUBPROCESS. `TestBuildTags()` compiles every plugin in, but only STARTED plugins register `CommandDecl`s. The runtime audit needs a config that starts every command-registering plugin.
- [ ] `test/plugin/plugin-command-completion.ci` - runtime `system command list` assertion precedent
  → Constraint: a `.ci` test can boot a plugin, dispatch `system command list`, and assert over the returned commands. Model the runtime audit on this.
- [ ] `plan/spec-command-naming.md` - the 21 violations this spec absorbs
  → Constraint: 18 BGP-plugin commands missing `bgp` prefix + 3 fused `fib-<backend>` tokens; renames require updating cross-plugin dispatch strings and `.ci` files (see learned/829 gotchas).

**Behavior to preserve:**
- Every command continues to execute; renames update all dispatch strings and `.ci` tests in the same change (Ze is unreleased - replace outright, no deprecation branches per `cli-grammar.md` Backward Compatibility).
- The compile-time enumeration approach of `scripts/inventory/commands.go` (blank-import `plugin/all` → `BuildCommandTree` + `AllBuiltinRPCs`) is reused, not replaced.
- `validateCommandName`'s existing token-form checks and `validVerbList()`-derived error strings.
- Category-exempt surfaces keep their noun-first forms (ExaBGP bridge, plugin/system protocol, editor modes).

**Behavior to change:** (user explicitly requested)
- Add a single canonical verb registry; `commandVerbs` derives from it (reconcile the 8-vs-7 discrepancy).
- Strengthen `validateCommandName` from verb-first-only to the full path-level ruleset (R1-R4).
- Fix every current grammar violation so the gate is green (metrics pool, config archive, create/delete iface families, mpls prose, + the 21 from spec-command-naming).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A developer/agent adds or edits a command: a YANG `ze:command` container, a plugin `CommandDecl`, or a `.yang` filter leaf.
- The gate reads the resulting command surface from three feeders (below).

### Transformation Path
1. **Canonical verb registry** (`internal/component/command`) declares the closed verb set; it is the vocabulary every check reads.
2. **Shared grammar checker** (`internal/component/command/grammar`) implements R1-R8 as pure functions over a `command.Node` tree and over path strings.
3. **Feeder 1 - compile-time YANG tree:** `scripts/checks/cli_grammar.go` builds `BuildCommandTree` in-process, walks it, runs R1-R8 (full structural depth), and runs the `--flag`-in-YANG grep (R3). Deterministic, fast.
4. **Feeder 2 - registration-time:** strengthened `validateCommandName` runs R1-R4 (path-level) on every plugin `CommandDecl` as it registers, using the canonical registry.
5. **Feeder 3 - runtime merged audit:** a `.ci` test boots a daemon with a dedicated all-plugins config, dispatches `system command list --verbose --json`, and runs R1-R4 over the complete merged surface; a drift guard asserts the config starts every command-registering plugin.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema ↔ command tree | `BuildCommandTree` merges `-cmd` modules into `command.Node` | [ ] |
| Plugin ↔ dispatcher | `CommandDecl` → `validateCommandName` → `CommandRegistry` | [ ] |
| Daemon ↔ test | `ze-system:command-list` RPC (path-only surface) | [ ] |
| Verb registry ↔ plugin gate | `commandVerbs` derives from the canonical registry | [ ] |

### Integration Points
- `command_registry.go` `commandVerbs`/`validateCommandName` (strengthen + rewire to registry).
- `scripts/checks/*.go` + `checks_test.go` + `mk/inventory.mk` + `Makefile` `_ze-verify-impl` (new static gate).
- `mk/test-functional.mk` + `internal/test/cli/register.go` + `test/<suite>/` (runtime audit + all-plugins config).

### Architectural Verification
- [ ] No bypassed layers - checker reads the same tree the CLI/completer reads.
- [ ] No unintended coupling - checker is a pure package; feeders depend on it, not each other.
- [ ] No duplicated functionality - one checker, one verb registry; extends `validateCommandName` rather than adding a parallel check.
- [ ] Zero-copy preserved where applicable - N/A (test/tooling path; no hot-path allocation added).
- [ ] Registration over hardcoding - verbs and exemptions live in registries/structural markers, not scattered literals; the plugin gate derives from the shared registry.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every keyword node in a `-cmd.yang` is a closed container; no `list` nodes exist | `command.go:233` filter + grep returned zero `list` nodes | keyword/value classifier is unsound | grep `^\s*list ` across all `ze:command` yang; unit test on the tree | confirmed |
| A-2 | `validateCommandName` runs for every plugin `CommandDecl` at registration | `command_registry.go:86`, called from `Register` (:221) | plugin commands escape Feeder 2 | unit test: register a bad `CommandDecl`, assert rejection | unvalidated |
| A-3 | A daemon config can start every command-registering plugin in one boot | harness research: startup is config-driven + auto-load (`startup_autoload.go`) | Feeder 3 can't achieve a complete snapshot | build the all-plugins config; assert dump covers every command plugin | unvalidated |
| A-4 | `system command list --verbose` returns the complete merged builtin+plugin surface | `system.go:304-338`; `test/plugin/plugin-command-completion.ci` | runtime audit is incomplete | `.ci` assertion counting commands vs known floor | unvalidated |
| A-5 | Placing the verb registry + checker in `internal/component/command` creates no import cycle | grep: server imports command, not vice-versa | must relocate to `internal/core` | `make ze-tier-check` + build | confirmed |
| A-6 | Current violations are fixable to compliant grammar without user-facing loss | inventory analysis; `cli-grammar.md` Engine-Owned Tree Mutation | some command has no compliant form | per-command classification during remediation | unvalidated |
| A-7 | Every current violation is enumerable up front (inventory + spec-command-naming + registration gate) | `make ze-command-list` (254 cmds) + spec-command-naming | gate stays red on an unknown violation | run the gate once on the unfixed tree; triage its full output | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | All-plugins config drifts as new command plugins are added → runtime audit silently misses them | new plugin's commands absent from the dump | drift guard (AC-6): enumerate command-registering plugins at compile time, assert each is started by the config; fail otherwise |
| R-2 | Category exemptions become a dumping ground for real violations | exemption count grows | exemptions keyed on STRUCTURAL identity (owning module / wire-method namespace), enumerated in one place, count reported by the gate |
| R-3 | `create`/`delete` iface commands have no obvious compliant form (runtime action vs config-tree mutation) | remediation stalls on these | classify each per `cli-grammar.md` Engine-Owned Tree Mutation: config-tree object → `set`/`delete`; genuine runtime action → operational verb added to the canonical registry with justification |
| R-4 | Renames break cross-plugin dispatch strings and `.ci` tests (learned/829 gotchas: trailing-space `replace_all`, secondary dispatch tables) | functional tests fail after rename | grep every dispatch site + `.ci`; update in the same change; runtime audit catches misses |
| R-5 | Deep structural rules (R5-R8) only apply to YANG-backed commands; runtime-only plugin commands (path-only) escape them | a plugin ships a bad typed selector with no YANG | document the coverage matrix; plugin commands with typed args SHOULD ship a `-cmd` YANG module (already the norm); registration gate still enforces R1-R4 |
| R-6 | Runtime audit adds a full daemon boot to the functional suite (cost) | suite wall-clock rises | single boot, one dump, path-level checks only; lives in the functional suite (not the fast unit/pre-commit gate) |
| R-7 | The `--flag`-in-YANG check trips on benign migration prose (e.g. mpls revision description) | false positive on descriptions | fix the one current prose hit; define R3 precisely (any `--[a-z]` in a `.yang`, including descriptions, is a violation per the rule) - so the fix is to reword, not to loosen the check |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Agent adds a noun-first plugin `CommandDecl` | → | strengthened `validateCommandName` rejects it | `TestValidateCommandNameRejectsNounFirst` |
| Agent adds a verb outside the registry | → | `validateCommandName` / static gate rejects it | `TestValidateCommandNameUnknownVerb`, `TestCLIGrammarGateStatic` |
| Agent adds a noun-first `ze:command` YANG container | → | static gate R1/R4 fails | `TestCLIGrammarGateStatic` (fixture) + `checks_test.go` smoke |
| A `--flag` appears anywhere in a `.yang` | → | static gate R3 fails | `TestCLIGrammarGateFlagInYang` |
| A free-form value with no typed selector is added | → | static gate R5 fails | `TestCLIGrammarGateKeywordBeforeValue` |
| Runtime merged surface contains a violation | → | runtime audit fails | `test/<suite>/command-surface-grammar.ci` |
| A new command-registering plugin is not in the all-plugins config | → | drift guard fails | `TestAllPluginsConfigComplete` |
| `commandVerbs` diverges from the canonical registry | → | derived-list test fails | `TestCommandVerbsDerivedFromRegistry` |

## Acceptance Criteria

<!-- Each row is a testable assertion. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The verb vocabulary | A single canonical `VerbRegistry` exists in `internal/component/command`; `command_registry.go` `commandVerbs` is derived from it (no second hardcoded list). The 8-vs-7 discrepancy is reconciled with each included verb justified. |
| AC-2 | The grammar rules | A shared checker package implements R1-R8 as pure functions over `command.Node` + path strings, with a known-good and known-bad unit fixture per rule. |
| AC-3 | Any built-in (YANG) command violates R1-R8, or any `.yang` contains `--flag` | `scripts/checks/cli_grammar.go` (wired to `make ze-cli-grammar-check` and into `_ze-verify-impl`) fails with a per-command diagnostic naming the rule; `checks_test.go` asserts `"cli-grammar: OK"` on a clean tree. |
| AC-4 | A plugin registers a `CommandDecl` violating R1-R4 | Strengthened `validateCommandName` rejects it at registration with a rule-named error derived from the canonical registry. |
| AC-5 | The daemon is booted with the all-plugins config | A `.ci` runtime audit dumps `system command list --verbose --json` and asserts every command in the merged surface satisfies R1-R4 or is category-exempt. |
| AC-6 | A command-registering plugin is not started by the all-plugins config | The drift guard fails, naming the missing plugin. |
| AC-7 | Category exemptions | Exemptions are defined by structural identity (owning module / wire-method namespace) in ONE place; the gate reports the exempt set and count. No per-command string allowlist exists. |
| AC-8 | The current command surface | Every current violation is fixed (metrics pool → `show metrics pool`; config archive → verb-first form; create/delete iface families classified per Engine-Owned Tree Mutation; mpls revision prose reworded; the 21 spec-command-naming renames applied with dispatch + `.ci` updates). All three feeders pass. |
| AC-9 | Discoverability | `ai/INDEX.md` (keyword + dev-tools rows), `ai/rules/cli-grammar.md` (Mechanical Enforcement section), `ai/rules/hook-mapping.md`, `ai/rules/testing.md`, and the stale `ai/rules/cli-patterns.md` "Known gap" line are updated; `spec-command-naming.md` is closed as absorbed; a `plan/learned/NNN-*.md` records the decision. |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Agent adds `show foo` where `foo` takes a bare `<name>` value with no selector | static gate builds tree → checker R5 flags the untyped value slot | `TestCLIGrammarGateKeywordBeforeValue` |
| 2 | Agent adds a plugin command `foo show status` (noun-first) | plugin registers → `validateCommandName` R1/R4 rejects → registration fails | `TestValidateCommandNameRejectsNounFirst` |
| 3 | Agent writes `description "filter with --family"` in a `.yang` | static gate R3 grep flags it | `TestCLIGrammarGateFlagInYang` |
| 4 | Maintainer runs `make ze-verify` on a clean tree | static gate + unit checks pass; runtime audit passes in the functional suite | `checks_test.go` smoke + `command-surface-grammar.ci` |
| 5 | Agent adds a new command-registering plugin but forgets the all-plugins config | drift guard fails in the functional suite | `TestAllPluginsConfigComplete` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerbRegistryCanonical` | `internal/component/command/verbs_test.go` | AC-1: registry contents + each verb's classification | |
| `TestCommandVerbsDerivedFromRegistry` | `internal/component/plugin/server/command_registry_test.go` | AC-1: `commandVerbs` == registry (no drift) | |
| `TestGrammarCheckerRules` (table: R1-R8) | `internal/component/command/grammar/checker_test.go` | AC-2: good/bad fixture per rule | |
| `TestValidateCommandNameRejectsNounFirst` | `internal/component/plugin/server/command_registry_test.go` | AC-4, Wiring | |
| `TestValidateCommandNameUnknownVerb` | same | AC-4 | |
| `TestCLIGrammarGateStatic` | `scripts/checks/checks_test.go` | AC-3: gate passes clean tree, fails a bad fixture | |
| `TestCLIGrammarGateFlagInYang` | `scripts/checks/checks_test.go` | AC-3/R3 | |
| `TestAllPluginsConfigComplete` | `internal/test/...` (drift guard) | AC-6 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - gate inputs are command trees/strings, not numeric ranges | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `command-surface-grammar` | `test/<suite>/command-surface-grammar.ci` | Boot all-plugins config, dump `system command list`, assert whole merged surface is compliant | |
| existing plugin/ui `.ci` suites | `test/plugin/*.ci`, `test/ui/*.ci` | Renamed commands still execute and complete | |

### Interop Tests (MANDATORY for protocol features)
N/A - no wire protocol behavior changes.

### Future (if deferring any tests)
- None. All ACs are in-scope for this spec (user chose fix-all, no deferral).

## Files to Modify
- `internal/component/plugin/server/command_registry.go` - `commandVerbs` derives from canonical registry; strengthen `validateCommandName` to R1-R4.
- `internal/component/plugin/server/command.go` - integrate category-exemption markers if the runtime/registration path needs them (e.g. editor/system skips).
- `scripts/checks/checks_test.go` - add `cli-grammar` smoke + bad-fixture tests.
- `mk/inventory.mk` - add `ze-cli-grammar-check` (+ `-json`) target.
- `Makefile` - add `ze-cli-grammar-check` to `_ze-verify-impl` (~line 274).
- `mk/test-functional.mk` / `internal/test/cli/register.go` - register the runtime audit suite/dir if a new one is used.
- Violating command definitions (fix-all):
  - `internal/component/bgp/plugins/cmd/rib/yang/ze-rib-poolstats-cmd.yang` - `metrics pool` → `show metrics pool`.
  - `internal/plugins/config-archive-cmd/yang/ze-config-archive-cmd.yang` (+ handler) - `config archive` → verb-first form.
  - `internal/component/iface/yang/ze-iface-cmd.yang` (+ handlers) - classify `create`/`delete` iface families per Engine-Owned Tree Mutation.
  - `internal/plugins/mpls-cmd/yang/ze-mpls-cmd.yang:8` - reword the `--limit` revision description.
  - spec-command-naming targets: `internal/component/bgp/plugins/{adj_rib_in,rpki,healthcheck,watchdog,rs}/*.go` (+ dispatch strings), `internal/plugins/fib/{p4,vpp,kernel}/register.go`, and their `.ci` tests.
- Discovery: `ai/INDEX.md`, `ai/rules/cli-grammar.md`, `ai/rules/hook-mapping.md`, `ai/rules/testing.md`, `ai/rules/cli-patterns.md` (stale "Known gap"), relevant `docs/`.
- `plan/spec-command-naming.md` - close (absorbed).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No - no new commands; only renames of existing | - |
| YANG validation constraints | No | - |
| YANG custom validators | No | - |
| CLI commands/flags | Yes - rename violating commands | violating `.yang`/handler files above |
| CLI grammar (action before identifier) | Yes - this spec IS the grammar gate | `ai/rules/cli-grammar.md`, new checker |
| Editor autocomplete | No - completion unchanged (paths renamed, tree still built) | - |
| Functional test for new RPC/API | Yes - runtime audit `.ci` | `test/<suite>/command-surface-grammar.ci` |
| Pipe completeness | No - no new output-producing command | - |
| Env var registration | No | - |
| Doctor check for runtime dependencies | No - no new runtime dependency (test/tooling only) | - |
| Prometheus counters/metrics | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No - gate is dev-facing | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes - renamed commands | `docs/guide/command-reference.md`, `docs/guide/command-catalogue.md` |
| 4 | API/RPC added/changed? | Yes - renamed wire paths | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | Yes - `validateCommandName` stricter | `ai/rules/plugin-design.md` (command declaration rules) |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | Yes - new gate + runtime audit | `docs/functional-tests.md`, `docs/contributing/` gate docs |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes - verb registry + checker | `docs/architecture/cli/` (namespacing or a new grammar-gate note) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered command inventory changed? | Yes - renames | `docs/guide/command-catalogue.md`, regenerate via `make ze-command-list` |
| 16 | Any changed source file referenced by doc source anchors? | Yes - grep required | grep `docs/` for anchors on renamed files |
| 17 | Existing docs show CLI examples for renamed commands? | Yes - grep required | update stale examples for metrics pool / config archive / fib / bgp-prefixed cmds |

## Files to Create
- `internal/component/command/verbs.go` (+ `verbs_test.go`) - canonical `VerbRegistry`.
- `internal/component/command/grammar/checker.go` (+ `checker_test.go`) - shared R1-R8 checker.
- `scripts/checks/cli_grammar.go` (`//go:build ignore`) - static gate over the compile-time YANG tree + `--flag` grep.
- `test/<suite>/command-surface-grammar.ci` - runtime merged audit.
- The dedicated all-plugins test config (starts every command-registering plugin) - location under `test/<suite>/` per the suite convention.
- `plan/learned/NNN-cli-grammar-gate.md` - at completion.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create; run `make ze-command-list` to enumerate the live surface |
| 3. Wiring phase | Wiring Test table - registry + checker skeletons + failing gate |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` |
| 7-14 | Critical review, deliverables, security, summary |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - canonical `VerbRegistry` + shared checker package skeletons; static gate `scripts/checks/cli_grammar.go` that builds the tree and reports (initially failing on current violations); `checks_test.go` smoke.
   - Tests: `TestVerbRegistryCanonical`, `TestGrammarCheckerRules` (stubs), `TestCLIGrammarGateStatic`
   - Verify: gate runs, enumerates violations (the triage list for Phase 5).
2. **Phase: Checker rules** - implement R1-R8 as pure functions with good/bad fixtures; classify value-slots via `ArgDef.Kind` exactly as the completer does.
   - Tests: `TestGrammarCheckerRules` (full table)
3. **Phase: Verb registry unification** - move the vocabulary into `VerbRegistry`; make `commandVerbs` derive from it; reconcile 8-vs-7; strengthen `validateCommandName` to R1-R4.
   - Tests: `TestCommandVerbsDerivedFromRegistry`, `TestValidateCommandNameRejectsNounFirst`, `TestValidateCommandNameUnknownVerb`
4. **Phase: Category exemptions** - define structural exemption markers (owning module / wire-method namespace) in one place; gate reports the exempt set.
5. **Phase: Fix-all remediation** - fix every current violation (metrics pool, config archive, create/delete iface per classification, mpls prose, + spec-command-naming's 21 with dispatch + `.ci` updates). Gate goes green.
6. **Phase: Runtime audit** - build the all-plugins config; write `command-surface-grammar.ci`; add the drift guard.
   - Tests: `command-surface-grammar.ci`, `TestAllPluginsConfigComplete`
7. **Phase: Discovery + docs** - AC-9 updates; close spec-command-naming.
8. **Full verification** - `make ze-verify`.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has code + test at file:line; all three feeders green |
| Correctness | R5 value-slot classification matches the completer's logic; peer-command exception honored; `arp` not flagged |
| Naming | Renamed commands are verb-first, kebab tokens; YANG paths mirror the corrected grammar |
| Data flow | One checker + one registry; feeders depend on the checker, not each other |
| CLI grammar | The gate itself is the enforcement; exemptions are structural, not string allowlists |
| Registration over hardcoding | `commandVerbs` derives from `VerbRegistry`; no second verb list; exemptions in one place |
| Rule: no-layering | Old command names fully removed (Ze unreleased; no deprecation branches) |
| Drift guard | All-plugins config completeness enforced by `TestAllPluginsConfigComplete` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Static gate wired | `make ze-cli-grammar-check` runs; present in `_ze-verify-impl` (`grep`) |
| Registration gate strengthened | `go test ./internal/component/plugin/server -run ValidateCommandName` |
| Runtime audit | `.ci` file exists and runs in the functional suite |
| Verb registry single source | `grep` shows `commandVerbs` references the registry; no duplicate literal |
| Green surface | `make ze-command-list` + gate exit 0 |
| spec-command-naming closed | file removed; learned summary references it |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Checker handles empty/degenerate trees without panic |
| Resource exhaustion | Runtime audit boots one daemon, bounded by suite timeout; no unbounded loop over the tree |
| Error leakage | Gate diagnostics name the rule + command path; no sensitive data |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A command has no compliant grammar form | Classify per Engine-Owned Tree Mutation; escalate `create`/`delete` decision to user if ambiguous |
| Rename breaks a `.ci`/dispatch | Fix in the remediation phase; runtime audit catches misses |
| Import cycle from registry placement | Relocate to `internal/core`; re-run `make ze-tier-check` |
| 3 fix attempts fail on one command | STOP, report, ask user |

## Design Insights
<!-- LIVE -->
- The keyword/value boundary needs no new metadata: it is already encoded as container-vs-leaf in YANG and as `Children`-vs-`ArgDefs` in `command.Node`. The checker reads the exact structure the completer and dispatcher read, so the gate can never disagree with the real CLI.
- Three feeders are not redundancy; each sees something the others structurally cannot: compile-time tree has depth but not runtime-only plugin commands; the runtime RPC has the real merged paths but no `ArgDef` depth; registration-time travels with each plugin. One checker + one registry keeps the rules identical across all three.

## Core Insight
The rules were always machine-checkable; what was missing was a single source of truth for the vocabulary (verbs) and a single reader of the structure (the checker). Unify those two and the prose rule becomes an invariant.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Three coordinated feeders, one checker, one verb registry | Runtime-harness-only; static-only (YANG walk + AST-parse plugin literals) | Runtime-only is structurally blind (RPC drops `ArgDefs`) and non-deterministic; static AST-parsing reads source not the CLI surface and misses names built dynamically. Only the combination faithfully covers the real surface with full depth. (User chose the runtime-faithful lens; this realizes it.) |
| Dedicated all-plugins config + drift guard for Feeder 3 | Best-effort broad config with a floor assertion | User chose the complete-snapshot option; the drift guard (AC-6) contains the config-drift risk it introduces. |
| Fix-all in one sweep; category exemptions only | Ship gate red as a TODO; gate + allowlist of current violations | User chose fix-all; leaves the gate meaningful (no per-command allowlist to erode) and closes spec-command-naming. |
| Verb registry in `internal/component/command` | `internal/core`; leave `commandVerbs` as-is | Cycle-free (verified); already imported by the plugin server and the YANG tree builder; the natural shared home. |
| Exemptions by structural identity (module / wire namespace) | Per-command string allowlist | A string allowlist erodes into a dumping ground (R-2); structural markers stay principled and countable. |

## Known Limitations
- Deep structural rules (R5-R8) apply only where YANG structure exists (built-ins + plugin `-cmd` modules). Runtime-only plugin commands that ship no YANG get R1-R4 (path-level) only. This matches reality: typed-argument commands already ship YANG.
- The offline `ze <domain>` host subcommands (`analyze`, `config`, `doctor`, ...) are a SEPARATE grammar that legitimately uses `--flags` (`cli-patterns.md`); they are out of scope for this operator-CLI gate.

## RFC Documentation
N/A - no RFC-governed behavior.

## Implementation Summary
<!-- Filled during /implement -->
### What Was Implemented (Feeders 1+2 complete, wired, green)
- Canonical `command.Verbs` registry (`internal/component/command/verbs.go`) incl. `create` (runtime lifecycle, ratified). Tested.
- Shared R1-R8 checker + exemptions (`internal/component/command/grammar/checker.go`). Tested. Corrections applied: `create` is a verb; `del` = completion-prefix of `delete` (not a token); R6 mandatory + selector-exempt; R5/R8 refinements.
- Static gate `scripts/checks/cli_grammar.go` (+ `cli_grammar_test.go` smoke) — walks the YANG command tree, R1-R8 + `--flag`-in-YANG. Wired: `make ze-cli-grammar-check` in `_ze-verify-impl` (Makefile 274/281), `mk/inventory.mk`. Reports `cli-grammar: OK`.
- Feeder 2: `command_registry.go` `commandVerbs` map deleted; `validateCommandName` derives from `command.Verbs` + runs `grammar.CheckName` (R7). Tests pass; added `TestValidateCommandNameUsesCanonicalVerbs`.
- Grammar violations fixed (gate-verified green): `config archive`->`request config archive`; `metrics pool`->`show metrics pool`; mpls revision `--limit` prose; `show capture` `tunnel-id` numeric->string; iface `create`/`delete` family restructured to typed `name` selectors (`ze-iface-cmd.yang`).
- Docs: `ai/rules/cli-grammar.md` "Mechanical Enforcement" section; `ai/rules/cli-patterns.md` stale "Known gap" corrected; `ai/INDEX.md` Dev-Tools + keyword rows.

### Bugs Found/Fixed
- `commandVerbs` (plugin gate) diverged from learned-829's agreed verbs and from the builtin surface; unified into one registry.
- `ze-mpls-cmd.yang:8` revision description contained a literal `--limit` (violates No-Flag-in-YANG even in prose).

### Documentation Updates
- See "What Was Implemented" docs bullet. Generated `docs/guide/command-*` catalogue regen still pending (paths changed).

### Deviations from Plan
- The iface `create`/`delete` family was a grammar redesign (typed `name` selectors), not a rename; `create` was blessed as a canonical verb (user-ratified). The 21 `spec-command-naming` renames are a namespace convention (not R1-R8) and were ALREADY implemented by prior command work -- verified (violation greps empty); `spec-command-naming` closed as done.
- Wire-break fix: `config archive`->`request config archive` broke `cmd_archive.go:57` (offline `ze config archive` tool sent the bare path over SSH); fixed to dispatch the new path.
- **AC-5 / AC-6 (Feeder 3 runtime audit + drift guard) carved to a follow-up child spec `plan/spec-cli-grammar-runtime-audit.md`** (user-approved). Reason: assumption A-3 (one config starts every command-registering plugin) is broken -- no such config exists; Feeder 3 is belt-and-suspenders over Feeders 1+2, which already enforce grammar on 100% of commands. The child spec is additive and must not reduce parent coverage.

## Implementation Audit
<!-- BLOCKING: Complete BEFORE writing learned summary. -->
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestVerbRegistryCanonical`, `TestCommandVerbsDerivedFromRegistry` | `command.Verbs`; `commandVerbs` map deleted |
| AC-2 | Done | `TestGrammarCheckerRules` (R1-R8 fixtures) | `grammar/checker.go` |
| AC-3 | Done | `make ze-cli-grammar-check` = OK; `TestCLIGrammarGateStatic` | in `_ze-verify-impl` |
| AC-4 | Done | `TestValidateCommandNameUsesCanonicalVerbs` + existing suite | derives from `command.Verbs` + `grammar.CheckName` |
| AC-5 | Carved | -> `plan/spec-cli-grammar-runtime-audit.md` | A-3 broken; belt-and-suspenders |
| AC-6 | Carved | -> `plan/spec-cli-grammar-runtime-audit.md` | drift guard with Feeder 3 |
| AC-7 | Done | `TestExemptCategory`; gate reports exempt counts | structural by wire-method namespace |
| AC-8 | Done | `make ze-cli-grammar-check` = OK (0 findings) | config archive, metrics pool, mpls, capture, iface all fixed |
| AC-9 | Done | docs updated; `spec-command-naming` closed; learned 1056 | |
### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestVerbRegistryCanonical` | Pass | `internal/component/command/verbs_test.go` | |
| `TestGrammarCheckerRules` | Pass | `internal/component/command/grammar/checker_test.go` | R1-R8 |
| `TestCLIGrammarGateStatic` | Pass | `scripts/checks/cli_grammar_test.go` | smoke |
| `TestValidateCommandNameUsesCanonicalVerbs` | Pass | `internal/component/plugin/server/command_registry_test.go` | |
### Files from Plan
| File | Status | Notes |
|------|--------|-------|
### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Grammar rules are regression-proof | gate + functional test | static gate in `_ze-verify-impl`; `command-surface-grammar.ci` in functional suite |
| Agents stop drifting from grammar | registration rejection | `validateCommandName` rejects a bad `CommandDecl` (test) |
| The rules are explicit, not scattered prose | reverse-engineered ruleset | R1-R8 table + checker with per-rule fixtures |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [pending /ze-review] | file:line | |
### Fixes applied
- [pending]
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
| AC-1 | single verb registry | `grep commandVerbs command_registry.go` = none; `command.Verbs` sole source |
| AC-3 | gate in ze-verify | `make ze-cli-grammar-check` = OK; present in Makefile `_ze-verify-impl` |
| AC-4 | registration strengthened | `go test .../plugin/server -run ValidateCommandName` = ok |
| AC-8 | surface clean | `make ze-cli-grammar-check` = "cli-grammar: OK" (255 checked, 0 findings) |
| AC-5/6 | carved to child spec | `plan/spec-cli-grammar-runtime-audit.md` exists |
### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | zero `list` nodes in any `-cmd.yang`; keyword/value classifier sound |
| A-2 | confirmed | `validateCommandName` called from `Register` (command_registry.go:221) |
| A-3 | broken | spike: no all-plugins config exists; startup config-path driven -> Feeder 3 carved to child spec |
| A-4 | deferred | with Feeder 3 (child spec) |
| A-5 | confirmed | `command` imported by plugin/server, not vice-versa; builds clean |
| A-6 | confirmed | every violation fixed to a compliant form; gate green |
| A-7 | confirmed | gate enumerated all findings; all triaged and fixed |
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (checker, registry, gate, audit)
- [ ] Integration completeness proven end-to-end (all three feeders green)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (checker used by 3 feeders)
- [ ] No speculative features (only R1-R8, only current exemptions)
- [ ] Single responsibility (checker = rules; registry = vocabulary; feeders = enumeration)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (feeders depend only on the checker)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING - before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cli-grammar-gate.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump; close spec-command-naming
- [ ] **Commit B:** `git rm plan/spec-cli-grammar-gate.md` (+ `plan/spec-command-naming.md`)

## Appendix: Reverse-Engineered Grammar Ruleset (R1-R8)

The single explicit definition the checker enforces. Derived from `ai/rules/cli-grammar.md`,
`ai/rules/cli-patterns.md`, `docs/architecture/cli/command-namespacing.md`, and
`command_registry.go`.

| ID | Rule | Level | Feeders |
|----|------|-------|---------|
| R1 | First token is a verb in the canonical `VerbRegistry` (unless category-exempt) | path | 1, 2, 3 |
| R2 | Every token is lowercase ASCII letters + digits + interior hyphens; no leading/trailing hyphen; single-space separated | path | 1, 2, 3 |
| R3 | No token is a `--flag`; and no `--[a-z]` appears anywhere in a `.yang` file (leaf, description, or comment) minus `urn:`/`http`/`xml` | path + static | 1 (+ grep), 2, 3 |
| R4 | Noun-first forms are allowed ONLY for the exempt categories (E1-E3), identified structurally | path | 1, 2, 3 |
| R5 | No free-form value slot (`ArgString`/`ArgUint` `ArgDef`) is reachable without a preceding closed keyword; member selection uses a typed selector kind (`name`/`id`/`index`/`address`/`type`/`key`/...). Enum-valued args are themselves closed keywords. Exception: peer commands (`show bgp peer <name\|address>`) | structure | 1 |
| R6 | Action keyword precedes identifier; no `<resource> <value> <action>` (`cache retain <id>`, not `cache <id> retain`) | structure | 1 |
| R7 | Config-tree objects mutate only via engine `set`/`delete` path form; no operational `create`/`add`/`del`/`remove` verbs for anything already in the config YANG tree | structure | 1 |
| R8 | Identifier value leaves are string-typed (`ArgString`), not numeric | structure | 1 |

### Category Exemptions (structural identity only)
| ID | Category | Structural marker | Examples |
|----|----------|-------------------|----------|
| E1 | ExaBGP bridge compat surface | owning modules `cmd/announce`, `cmd/update`, `cmd/raw`, `cmd/peer`; wire methods `ze-bgp:announce/withdraw/peer-raw/peer-update` | `announce`, `withdraw`, `peer raw`, `peer update` |
| E2 | Plugin/system wire protocol directives | wire-method namespaces `ze-plugin:*`, `ze-system:*`, and `ze-bgp:plugin-*` | `plugin session ready`, `system command list`, `plugin encoding` |
| E3 | Editor internal modes | wire methods `ze-editor:mode-command`, `ze-editor:mode-edit` (already in `skippedWireMethods`) | mode switches |
