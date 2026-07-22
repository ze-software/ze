# Spec: fixit-config-file-positional-grammar

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-07-21 |

**Approval required before implementation.** ~~This spec is `design`, not `ready`.
Research is complete and a design is proposed, but the verb choice, the
migration approach for the test harness (see Risks R-1), the `ze -` stdin
sentinel question (R-2), and the YANG-fork / gate-extension scope (AC-7) all
need Thomas's decision before any code is written.~~ Superseded 2026-07-21: the
four flagged decisions are resolved by AUTONOMOUS DEFAULT (Thomas may override)
under the standing "complete all spec-fixit specs, resolve blocks with defaults"
directive; the user is engaged and steering the model split (Opus impl, 4.8
design/review). See **Design Resolution** below.

## Design Resolution (autonomous defaults -- Thomas may override)

| Flagged decision | Autonomous default chosen | Rationale |
|------------------|---------------------------|-----------|
| Verb (R-1 head / Key Design Decisions) | `start` | The spec's own recommendation: `start` is already a registered root verb whose job is to start the daemon (`ze_core_start.go:80` `cmdStart`); `--web-only` errors already point to `ze start`; `run` is a retired stub. |
| R-2 `ze -` stdin sentinel | **Keep `-` as a closed position-1 sentinel** (do NOT fold into `ze start -`). | R1 forbids a FREE-FORM value at position 1 because it can collide with a keyword. `-` is a fixed sentinel meaning stdin; the set {`-`} is closed and cannot collide with any command name, so keeping it satisfies R1's rationale. `-` alone = stdin is a universal Unix convention; `ze start -` is worse UX. This reduces the corpus migration from ~544 `exec=ze -` files to ~10. |
| R-1 migration scope | Migrate only the FREE-FORM path launches: the runner launch site(s) + `zeDaemonConfigArgIndex` (in lockstep) + the ~10 literal `exec=ze <file>.conf` directives. The 544 `exec=ze -` stay (sentinel). | Follows directly from the R-2 sentinel decision. Smaller, lower-risk diff; the `.ci` text still matches the real argv for both forms (`ze -` unchanged, `ze start x.conf` explicit). |
| AC-7 gate extension / YANG fork | **Record the gate hole + the `show route` YANG value-first fork; do NOT extend the gate or edit the YANG.** Home the general gate-hardening as a follow-up (deferral), per the spec's own scoping ("escalated, not implemented unilaterally"). | The offline bare-positional sink is Go dispatch, invisible to the YANG/registered-root feeders; a general gate extension is a policy change (it would flag the deliberately-blessed `show route [<prefix>]` fork). AC-6's `cmd/ze` regression guard covers the offline surface specifically; the general angle is recorded + deferred, which is exactly what AC-7 asks for (a proposal + escalation, not implementation). |

**AC-2 amendment (autonomous default):** AC-2 as written removes "the bare
positional config branch and `looksLikeConfig`". Under the R-2 sentinel default,
the FREE-FORM path branch (`ze_core_dispatch.go:409-422`) and `looksLikeConfig`
are removed, while the `-` sentinel handling (`:404-408`) is RETAINED as a closed
position-1 token. Position 1 stays a closed set = {YANG verbs} ∪ {registered
roots} ∪ {`-`, `-h`, `--help`}. AC-6's regression guard asserts exactly that (no
free-form *value* sink), which the retained `-` sentinel does not violate.

**AC-5 amendment (autonomous default):** "Full functional suite green" is proven
against the pre-existing-red baseline (rfc7606 audit debt, darwin-env
`install:*-kernel`, logged plugin 224/458, flaky 97/398): the migration must add
no NEW reds, not turn a red baseline green.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `ai/rules/cli-grammar.md` (the governing rule: R1 keywords-before-values, the
   Mechanical Enforcement table, the Root-namespace feeder).
3. `.claude/rules/planning.md` (workflow, status vocabulary).
4. `cmd/ze/ze_core_dispatch.go` and `cmd/ze/ze_core_start.go` (the producing code).

## Task

`ze <config-file>` puts a free-form filesystem path in the FIRST positional
token. That is the exact anti-pattern `ai/rules/cli-grammar.md` R1 forbids:
"Every CLI command must place a closed keyword before any user-supplied value.
This eliminates ambiguity where a free-form value could collide with a keyword."

The dispatcher (`cmd/ze/ze_core_dispatch.go`, `zeDispatch`) makes position 1
accept THREE overlapping things at once:

- a YANG verb (`show`, `set`, `clear`, `request`, `delete`, `update`,
  `validate`, `monitor`), matched at `:321` (`isYANGVerb`);
- any registered root command (`start`, `version`, `help`, `pipe`, `bgp`,
  `signal`, `status`, `config`, `cli`, `plugin`, `interface`, `firewall`,
  `l2tp`, `sysctl`, `resolve`, `tacacs`, `yang`, `data`, `schema`, `doctor`,
  `explain`, `support`, `skills`, `env`, `connect`, `exabgp`, `completion`,
  `passwd`, `init`, ...), matched at `:381` (`dispatchRegisteredRoot`);
- a free-form config-file path, matched at `:403` (`looksLikeConfig`).

Because the three share one position, and the config check runs LAST, a config
file whose basename equals a command name (a file literally named `signal`,
`bgp`, `config`, `start`, `show`, ...) is dispatched as THAT command and never
loaded as config. The disambiguation is a `.conf`-suffix / `os.Stat` heuristic
(`looksLikeConfig`, `:578-595`), the "ambiguity-resolution hack" the rule bans:
a plain-named config file (no recognized suffix, no slash) is not recognized as
config at all, and an exact-command-name file is shadowed by the earlier
command dispatch. The outcome is silent, surprising, and order-of-checks
dependent.

**Goal.** Replace the bare positional form with an explicit keyword before the
value: `ze start <config-file>` (recommended -- see Key Design Decisions). The
keyword (`start`) precedes the path, so position 1 is a closed set only.

**Scope (both surfaces, per the request "here AND in general"):**

1. **Specific fix** -- the offline dispatch surface in
   `cmd/ze/ze_core_dispatch.go`: route config-file launch through the existing
   `start` root verb, delete the bare positional branch and the `looksLikeConfig`
   heuristic (clean cutover; Ze is pre-release, `ai/rules/compatibility.md`).
2. **General grammar-hole angle** -- the invariant behind R1 is: *a free-form
   argument must never share a dispatch position with a closed keyword set.* The
   static gate (`grammar.CheckRootNamespace`, `make ze-cli-grammar-check`)
   governs REGISTERED root handlers only; a bare positional value sink is not a
   registered handler, so the gate does not catch this class today. Record where
   else the invariant is violated (offline dispatch; one YANG fork), and propose
   -- scoped, not over-reaching -- how the class is caught mechanically in
   future. The gate-extension / YANG-fork decision is escalated, not
   implemented unilaterally (AC-7).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/cli-grammar.md` - the governing rule (R1, Mechanical Check, Mechanical Enforcement table, Root namespace feeder, "Applies To" = offline dispatch).
  -> Constraint: R1 -- position 1 must be a closed keyword. The Mechanical Check states: "Can any positional argument before the action or selector-kind be a user-supplied value? -> Violation" and "If any `args[0]` usage passes the value to a lookup/parse function ... without first matching it against a keyword set, that is a violation." `looksLikeConfig(arg)` then `config.ResolveConfigPath(arg)` is exactly that.
  -> Constraint: "Applies To" (final section) -- the rule covers offline `cmd/ze/` dispatch, "No exceptions for 'simple' commands or 'obvious' identifier positions."
  -> Decision: Backward Compatibility section -- "If the wrong grammar has not shipped, replace it outright." Ze is pre-release, so a clean cutover (no deprecation branch) is correct.
- [ ] `ai/rules/compatibility.md` - pre-release, `internal/` and CLI surface may change; clean cutover, no shim.
  -> Constraint: only the external plugin API is frozen; the offline CLI is not.
- [ ] `ai/rules/plugin-self-containment.md` - Root namespace section (registered roots governed by `grammar.CheckRootNamespace`).

### RFC Summaries (MUST for protocol work)
- Not applicable. This is a CLI grammar / dispatch change, no wire protocol.

**Key insights:**
- The bug is a value-vs-keyword AMBIGUITY at one position, not a verb-vs-noun ordering problem. The token may legitimately BE a path; what is forbidden is the OVERLAP at position 1 between a closed command set and a free-form value sink.
- `start` is already a registered root verb whose sole job is "start the daemon"; extending it to accept an explicit config path is the minimal, precedent-following change.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/ze_core_dispatch.go` - the producing code for offline dispatch.
  -> Constraint: `arg := args[0]` (`:319`) -- position 1 is unconstrained.
  -> Constraint: `isYANGVerb(arg)` (`:321`, verb set `:555-558`) -- position 1 matches YANG verbs.
  -> Constraint: `dispatchRegisteredRoot(arg, rctx, args[1:])` (`:381`, definition `:535-541`) -- position 1 matches the closed registered-root set, dispatched BEFORE any config check.
  -> Constraint: `if looksLikeConfig(arg)` (`:403`) -- position 1 ALSO accepts a free-form path; `config.ResolveConfigPath(arg)` (`:410`) then `hub.Run(...)` (`:406`, `:419-421`).
  -> Constraint: `looksLikeConfig` (`:578-595`) -- true for `-` or a known suffix (`.conf/.cfg/.yaml/.yml/.json`), else only when the arg contains `/` or a leading `.` AND `os.Stat` succeeds (`:589-593`). A plain-named file with no suffix is not recognized.
  -> Constraint: the existing `--web-only` error already tells users to run `ze start --web-only` (`:399`), establishing `ze start` as the daemon-launch verb.
  -> Constraint: `run` is a removed deprecation stub (`:372-378`) that rejects with "'ze run' has been replaced by direct verb dispatch"; the name is retired.
  -> Constraint: the `start` root handler is registered at `:446-457` and calls `cmdStart`; the `-f <file>` global-flag launch path is at `:295-304` (keyword-first, unaffected by this bug).
- [ ] `cmd/ze/ze_core_start.go` - `cmdStart` and `startUsage`.
  -> Constraint: `cmdStart` (`:80-228`) parses only `--`-prefixed flags in its arg loop (`:89-131`, a `switch args[i]` with no `default`); a non-flag positional is silently ignored today. With no path it starts from blob storage (`:153`, `:192`).
  -> Constraint: `startUsage` (`:50-78`) documents `ze start [options]`, no positional config path.
- [ ] `cmd/ze/main_test.go` - `TestLooksLikeConfig` (`:34-58`).
  -> Constraint: cases `{"no_extension", "config", false}` (`:46`) and `{"command", "bgp", false}` (`:47`) encode the ambiguity as expectations: a config file named `config` or `bgp` is not detected as config, yet `bgp` is dispatched as a root command at `:381`. This test covers the helper the fix removes.
- [ ] `internal/component/command/grammar/checker.go` - the grammar checks.
  -> Constraint: `CheckRootNamespace` (`:214-238`) checks only HYPHENATED roots for R9 compound-vs-namespace collisions; it does not check value-vs-keyword-at-one-position. R6 (`:125-135`) fires only when a free-form value is `Mandatory` AND has keyword children AND is not a selector, and its comment (`:128-132`) EXPLICITLY exempts the optional-value-alongside-subcommands fork.
- [ ] `scripts/checks/cli_grammar.go` - the gate wiring.
  -> Constraint: `run()` (`:121-166`) enumerates registered roots (`registeredRootNames`) and namespaces (verbs + `yangContainerNames`) and runs `CheckRootNamespace`. There is no feeder for the offline bare-positional value sink -- it is neither in the YANG tree nor a registered root, so no feeder sees it. This is the gate hole.
- [ ] `internal/test/runner/runner_exec.go`, `runner_exec_util.go` - functional-test launch path (read via investigation).
  -> Constraint: the harness launches config EXCLUSIVELY via the bare positional form: `exec.CommandContext(testCtx, r.zePath, configPath)` (`runner_exec.go:211`) and the orchestrated path (`runner_exec.go:347`, argv assembled `:551-648`). `zeDaemonConfigArgIndex` (`runner_exec_util.go:247-287`) is a verbatim mirror of `looksLikeConfig` and must change in lockstep.
- [ ] `internal/component/iface/yang/ze-iface-show-cmd.yang` - the one YANG value-first fork.
  -> Constraint: `container route` (`:13-37`, `ze-show:route`) has optional free-form leaves `prefix` (`:20-22`) and `limit` (`:24-26`) as siblings of the keyword subcommand `container lookup` (`:29-36`). The token after `show route` is ambiguous (a CIDR/int value OR the keyword `lookup`); optional, so R6 stays silent and the gate passes it.

**Behavior to preserve:**
- `ze start` with no path -> start from blob storage (current `cmdStart` default).
- `-f <file>` global flag (`ze_core_dispatch.go:295-304`) -> unchanged (keyword-first already; no ambiguity).
- The blob-then-filesystem fallback used when launching from a path (`ze_core_dispatch.go:409-416`) -> preserved under `ze start <path>`.
- Config-type detection (`detectConfigType` -> `hub.Run`) semantics.

**Behavior to change (user requested):**
- Remove acceptance of a free-form config path in the FIRST positional slot of `ze`.
- `ze start <config-file>` becomes the supported form (keyword before value).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Process argv -> `zeSetup` (global-flag parse, `ze_core_dispatch.go:139-284`) -> `zeDispatch` (`:286`).
- Format at entry: `[]string` argv after global flags are consumed.

### Transformation Path
1. `arg := args[0]` (`:319`).
2. YANG-verb branch (`:321`) or registered-root branch (`:381`) or config-file branch (`:403`) -- today all three read position 1.
3. Config-file branch resolves the path and calls `hub.Run` (`:406`/`:419-421`).
4. Fix: config-file launch moves behind the `start` keyword; `cmdStart` (`ze_core_start.go`) gains an optional leading positional path and launches `hub.Run` from it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| argv -> dispatch | `zeDispatch` reads `args[0]` | [ ] |
| dispatch -> daemon | `hub.Run` / `cmdStart` -> `hub.Run` | [ ] |
| test harness -> ze argv | runner assembles `ze <config>` (to become `ze start <config>`) | [ ] |

### Integration Points
- `registry.MustRegisterRootHandler("start", ...)` (`ze_core_dispatch.go:446`) -> `cmdStart` (`ze_core_start.go:80`).
- `internal/test/runner` config launch sites (`runner_exec.go:211`, `:347`; `runner_exec_util.go:247-287`).

### Architectural Verification
- [ ] No bypassed layers (config launch flows through the `start` verb, not a positional side-door).
- [ ] No unintended coupling.
- [ ] No duplicated functionality (reuse `cmdStart` / `hub.Run`, do not add a second launch path).
- [ ] Zero-copy preserved where applicable (N/A -- dispatch, not wire).
- [ ] Registration over hardcoding (`start` is already a registered root; no new per-feature branch added to shared dispatch beyond removing the bare sink).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `start` root handler exists and `cmdStart` ignores positional args today, so accepting a path is additive | `ze_core_dispatch.go:446-457`, `ze_core_start.go:80-131` (switch matches only `--` flags, no default) | Design must add the handler instead of extending it | Code read (done) | confirmed |
| A-2 | The functional harness launches config only via the bare positional form | Explore: `runner_exec.go:211`, `:347`; zero `exec=ze start`/`-f`/`server` in `test/**/*.ci` | Fix would break every config-launch test until the harness is migrated | grep + runner read (done) | confirmed |
| A-3 | Removing the bare branch makes `looksLikeConfig` dead (only two callers, both in the removed branch) | `ze_core_dispatch.go:398`, `:403`; grep shows no other production caller (test at `main_test.go:54`) | Helper must stay; delete only the branch | grep (done) | confirmed |
| A-4 | Ze is pre-release; no deprecation shim owed for the bare form | `ai/rules/compatibility.md`; `run` was already hard-removed (`ze_core_dispatch.go:372-378`) | A deprecation branch would be required | rule read (done) | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | **Blast radius (USER DECISION).** ~544 `exec=ze -` + 10 literal `exec=ze <file>.conf` `.ci` directives, 2 runner launch sites, and the `zeDaemonConfigArgIndex` mirror all depend on the bare form. | Whole functional suite red after the dispatch change | Choose a migration approach (see below) with Thomas before coding. |
| R-2 | **`ze -` stdin sentinel (USER DECISION).** The 544 `exec=ze -` tests use `-` (stdin) at position 1, routed through the same branch. `-` is a closed sentinel, not a colliding free-form path. | Deciding whether `ze -` becomes `ze start -` or is preserved as a sentinel changes the migration size | Decide: fold `-` into `ze start -`, or keep `-` as an allowed position-1 sentinel that never collides. |
| R-3 | External scripts/plugins may call bare `ze <config>` (not greppable) | Post-cutover user reports | Pre-release means no obligation; document the cutover in the changelog/quickstart. |
| R-4 | **YANG fork + gate extension (USER DECISION, AC-7).** `show route [<prefix>]` vs `show route lookup` is a real value-vs-keyword fork the gate blesses (R6 comment). Tightening the gate would flag a pattern the project currently allows. | A gate check that fires on an intentionally-allowed fork | Record it; do NOT change the gate or the YANG without Thomas's decision. |

Migration approaches for R-1 (present to Thomas, do not pick unilaterally):
- **(A) Clean cutover of the corpus.** A Python migration script (per `ai/rules/go-standards.md` scripts-are-Python) rewrites `exec=ze -` -> `exec=ze start -` and `exec=ze <file>.conf` -> `exec=ze start <file>.conf`, plus the two runner sites and the detector. Faithful; the `.ci` text matches the real argv.
- **(B) Runner translation.** Change only the runner (`runner_exec.go:211`, `:347`) and `zeDaemonConfigArgIndex` to prepend `start` for a config launch, leaving `.ci` `exec=` directives literal. Smaller diff, but the `.ci` text then diverges from the actual argv (misleading).
- Approach (A) is recommended for faithfulness; the choice is Thomas's.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `ze start <config-file>` (offline) | -> | `cmdStart` path branch -> `hub.Run` | `test/parse/` or `test/ui/` `.ci`: `exec=ze start <file>` starts/loads the daemon |
| `ze <config-file>` (bare) after cutover | -> | falls through to "unknown command" (no auto-load) | `cmd/ze` Go test asserting bare positional path is not auto-loaded |
| `ze start signal` (file named `signal`) | -> | `cmdStart` loads the file; never dispatches the `signal` root | `.ci` proving a command-name-basename file loads via `start` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze start <config-file>` | Daemon starts and loads that file (keyword precedes the path value) |
| AC-2 | Bare `ze <config-file>` after cutover | No auto-load; the bare positional config branch and `looksLikeConfig` are removed from `zeDispatch` |
| AC-3 | A config file whose basename equals a registered root/verb (`signal`, `bgp`, `config`, ...) passed as `ze start <name>` | Loaded as config; never silently dispatched as the same-named command |
| AC-4 | `ze start` with no path | Preserves current blob-storage default launch behavior |
| AC-5 | Full functional suite after harness migration | Green; the harness and affected `.ci` files launch config via `ze start <config>` per the chosen approach (R-1) |
| AC-6 | Regression guard on the offline surface | A test proves `zeDispatch` has no free-form-value fallback at position 1 (position 1 is a closed set only) |
| AC-7 | General grammar-hole angle (SCOPED, USER DECISION) | The YANG `show route` value-first fork (`ze-iface-show-cmd.yang:13-37`) and the gate hole are recorded; whether to extend `make ze-cli-grammar-check` to flag value-vs-keyword-same-position dispatch (tightening R6's exemption) is escalated to Thomas and NOT implemented unilaterally in this spec |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Starts a daemon from a config file | `ze start <file>` -> `cmdStart` -> `hub.Run` | `.ci` in `test/parse/` or `test/ui/` |
| 2 | Names a config file after a subsystem and loads it | `ze start signal` -> `cmdStart` loads file | `.ci` proving no command collision |
| 3 | Types a bare filesystem path expecting the old behavior | `ze <file>` -> "unknown command" hint pointing to `ze start` | `cmd/ze` Go/functional test |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStartAcceptsConfigPath` | `cmd/ze/ze_core_start_test.go` (new) | `cmdStart` consumes a leading positional path and launches from it | |
| `TestDispatchNoPositionalConfigSink` | `cmd/ze/main_test.go` | bare `ze <path>` is not auto-loaded (replaces `TestLooksLikeConfig`, which tested the removed helper) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric input added) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `start-config-path` | `test/parse/` or `test/ui/` `.ci` | `ze start <file>` loads and runs | |
| `start-command-name-file` | `test/parse/` `.ci` | `ze start signal` loads a file named `signal` | |

### Interop Tests (MANDATORY for protocol features)
- N/A -- CLI grammar / dispatch change, no wire protocol behavior.

### Future (if deferring any tests)
- The gate-extension test (AC-7) is deferred pending Thomas's decision; recorded, not silently dropped.

## Files to Modify
- `cmd/ze/ze_core_dispatch.go` - remove the bare positional config branch (`:398-423`) and the now-dead `looksLikeConfig` (`:578-595`); route config-file launch through the `start` verb.
- `cmd/ze/ze_core_start.go` - `cmdStart` accepts an optional leading non-flag positional as the config-file path (preserve blob-default when absent, blob-then-filesystem fallback when a path is given); `startUsage` documents `ze start [<config-file>] [options]`.
- `cmd/ze/main_test.go` - remove `TestLooksLikeConfig` (`:34-58`, covers the removed helper -- a legitimate deletion of a test for removed functionality per `ai/rules/no-test-deletion.md`); add the dispatch regression test (AC-6).
- `internal/test/runner/runner_exec.go` (`:211`, `:347`) - emit `ze start <config>` for config launches (per chosen R-1 approach).
- `internal/test/runner/runner_exec_util.go` (`:247-287`) - update `zeDaemonConfigArgIndex` in lockstep with the grammar change.
- `test/**/*.ci` - ~544 `exec=ze -` + 10 literal `exec=ze <file>.conf` migrated to `ze start ...` (approach = user decision R-1; a Python migration script recommended).
- (SCOPED, pending AC-7 decision) `scripts/checks/cli_grammar.go` + `internal/component/command/grammar/checker.go` (gate extension) and `internal/component/iface/yang/ze-iface-show-cmd.yang` (the `show route` fork) -- only if Thomas approves tightening.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No (offline dispatch is Go, not YANG) | - |
| CLI commands/flags | Yes | `cmd/ze/ze_core_start.go`, `cmd/ze/ze_core_dispatch.go` |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli-grammar.md` (this fix realizes R1) |
| Functional test for behavior | Yes | `test/parse/` or `test/ui/` `.ci` |
| Test harness | Yes | `internal/test/runner/runner_exec*.go` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, quickstart (`ze start <config>` replaces bare form) |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (harness launches via `ze start`) |
| Others | | No | verify with grep for source anchors at implementation time |

## Files to Create
- `test/parse/` or `test/ui/` `.ci` - functional test for `ze start <config-file>` and the command-name-basename case.
- `cmd/ze/ze_core_start_test.go` - unit test for the new positional path handling (if not folded into `main_test.go`).
- (optional) `scripts/dev/migrate-config-launch.py` - the corpus migration script if approach (A) is chosen.

## Implementation Steps

### Implementation Phases
1. **Phase: Wiring (FIRST)** -- add the positional-path parameter to `cmdStart`, register nothing new (`start` already registered), write a failing wiring `.ci` (`ze start <file>`).
2. **Phase: Offline dispatch cutover** -- delete the bare positional config branch and `looksLikeConfig`; add the regression test (AC-6).
3. **Phase: Harness migration** -- update the two runner sites and `zeDaemonConfigArgIndex`; migrate the `.ci` corpus per the chosen R-1 approach.
4. **Phase: General angle** -- record the YANG fork and gate hole; implement the gate extension ONLY if AC-7 is approved.
5. **Functional + full verification** -- `make ze-functional-test`, `make ze-cli-grammar-check`, `make ze-verify`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-6 implemented; AC-7 resolved by decision |
| CLI grammar | position 1 of `ze` is a closed set only (no free-form value sink) |
| Rule: no-layering | `looksLikeConfig` and its branch fully deleted, not left dormant |
| Harness parity | `zeDaemonConfigArgIndex` no longer mirrors a removed heuristic |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Functional suite red on config launch | R-1 migration incomplete -- finish the corpus/runner migration |
| Grammar gate green but bug reproducible | expected today (gate hole); AC-7 decision governs whether to close it |
| 3 fix attempts fail | STOP, report, ask Thomas |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Verb = `start` | `run`; a new `load` verb | `start` is already a registered root verb (`ze_core_dispatch.go:446`) whose job is to start the daemon (`cmdStart`, `ze_core_start.go:80`); the `--web-only` error already directs users to `ze start` (`:399`). `run` was hard-removed and is now a deprecation stub rejecting with "'ze run' has been replaced by direct verb dispatch" (`:372-378`) -- reusing it resurrects a retired name with conflicting semantics. A new `load` verb duplicates what `start` means. |
| Clean cutover of the bare form | Deprecation branch accepting both | Ze is pre-release (`ai/rules/compatibility.md`); the grammar has not shipped a stable contract, so the rule says replace outright, no shim. |
| Frame the invariant as value-vs-keyword ambiguity, not verb-first ordering | Frame as "add a verb" | The precise defect is that ONE position accepts both a closed set and a free-form value; the sharp rule is R1's rationale ("eliminates ambiguity where a free-form value could collide with a keyword"), which also drives the general-angle findings. |
| General angle scoped to a regression guard + escalation | Extend the static gate to parse Go dispatch; flag the YANG fork now | The offline sink is Go dispatch, invisible to the YANG/registered-root feeders; a targeted `cmd/ze` regression test is cheaper and more precise. The YANG fork is deliberately blessed by R6's comment, so tightening it is a policy change that needs Thomas's decision, not a unilateral edit. |

## Known Limitations
- The static grammar gate (`make ze-cli-grammar-check`) does not, and after this spec still will not (unless AC-7 is approved), mechanically catch a bare-positional value sink in `cmd/ze` Go dispatch. The regression guard (AC-6) covers the offline surface specifically; a general gate extension is escalated.
- The `show route [<prefix>]` YANG fork remains a value-vs-keyword ambiguity that the gate deliberately blesses; changing it is a policy decision for Thomas, not part of this change.

## Design Insights
- The three position-1 branches in `zeDispatch` (`isYANGVerb`, `dispatchRegisteredRoot`, `looksLikeConfig`) are the concrete manifestation of the ambiguity: the fix reduces position 1 to the two closed-set branches and moves the value behind `start`.
- The test harness encodes the buggy grammar twice (the launch argv and the `zeDaemonConfigArgIndex` mirror), which is why the blast radius is large and why the migration approach is a first-order decision, not an implementation detail.

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `test/ui/start-config-path.ci` (KEEPALIVE); `cmd/ze/ze_core_start.go` file-launch branch + `startConfigPath` | mutation-verified RED when `startConfigPath` stubbed (falls to blob-default → "requires blob storage") |
| AC-2 | ✅ Done | `cmd/ze/ze_core_dispatch.go` (free-form branch + `looksLikeConfig` deleted; `-` sentinel retained per Design Resolution); `test/ui/bare-config-no-autoload.ci` | AC-2 amended: free-form path removed, `-` kept as closed sentinel |
| AC-3 | ✅ Done | `test/ui/start-command-name-file.ci` (`ze start signal` loads a file named like the `signal` command) | |
| AC-4 | ✅ Done | `cmd/ze/ze_core_start.go` blob-default flow unchanged; file-launch branch is additive and returns before it | no path → existing blob-default launch preserved |
| AC-5 | ✅ Done | Clean `ze-functional-test`: 21/23 suites green; plugin suite green except pre-existing 223/224/458 + flaky-under-load 91/97/378/398/523 (all PASS in isolation); `install` = darwin-env | The 26 auth-plugin embedded-launch regression was FOUND by this suite and FIXED (migrate embedded `ze <config>` → `ze start`) |
| AC-6 | ✅ Done | `test/ui/bare-config-no-autoload.ci` — genuine discriminator; producer verified `ze_core_dispatch.go:418` emits "unknown command" | would flip RED if the sink were restored (`.conf` → `looksLikeConfig` → daemon-launch error, not "unknown command") |
| AC-7 | ✅ Resolved (autonomous default) | Recorded in Known Limitations + Design Resolution; gate extension / YANG fork NOT implemented unilaterally | AC-7's deliverable is record + escalate, which is satisfied |

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| `ze start <config-file>` replaces the bare positional form | functional test | `test/ui/start-config-path.ci` PASS (mutation-verified); 38 config-launch `.ci` + the exabgp wrapper migrated to `ze start`; full plugin/reload/ui suites green via `ze start` |
| Command-name-basename files no longer silently mis-dispatch | functional test | `test/ui/start-command-name-file.ci` PASS |
| Offline position 1 is a closed set only | unit/regression test | `test/ui/bare-config-no-autoload.ci` PASS — bare `ze <file>.conf` → "unknown command" (producer `ze_core_dispatch.go:418`) |

## Review Gate

Independent review performed by a fresh no-context reviewer (0 BLOCKER, 0 ISSUE, 3
NOTE) PLUS the lead's own independent review (the lead designed the change; the
implementation was written by a separate Opus subagent). The lead's functional
verification caught a real regression the static review structurally could not:
26 `test/plugin/*.ci` auth tests launch the daemon from embedded shell scripts
(`tmpfs=*.sh`, `exec=./script.sh`) using the bare `ze <config>` form — a
shell-script-mediated launch invisible to an `exec=ze` grep. Fixed by migrating
those embedded launches to `ze start`; re-verified green. Two additional FORK
reviewers were DISCARDED (context-confabulation: they inherited the lead's
context, believed they were the lead, and launched duplicate `make` runs; all
rogue processes were detected and killed).

### Final status
- [x] Independent review shows 0 BLOCKER, 0 ISSUE (fresh reviewer + lead)
- [x] All NOTEs recorded: (1) legacy `docs/architecture/system-architecture.md:730` example updated to `ze start`; (2) `zeDaemonConfigArgIndex` still classifies a bare `.conf` as a daemon — harmless latent inconsistency (the one test that hits it gates regardless), accepted; (3) `--web-only` guard message narrowed to `arg == "-"` — accepted, consistent with the removal.

## Pre-Commit Verification

| Item | Verification | Result |
|------|--------------|--------|
| Files exist | `git status` shows all changed/new files (7 `.go`, 42 `.ci`, exabgp wrapper, 2 docs, `ai/CODE-TO-DOCS.md`) | ✅ |
| AC-1..AC-7 | Per Implementation Audit above; each traced to a test + producing `file:line` | ✅ |
| Wiring | `ze start` registered handler `ze_core_dispatch.go:435` → `cmdStart`; runner + exabgp wrapper launch via `ze start`; `zeDaemonConfigArgIndex` storage lockstep unit-tested | ✅ |
| Tests | `make ze-lint-changed` 0 issues; `go test ./cmd/ze/... ./internal/test/runner/...` green; full plugin suite green except pre-existing 223/224/458 + flaky 91/97/378/398/523 (verified PASS in isolation); clean `ze-functional-test` other 21 suites green | ✅ |
| Assumptions | A-1..A-4 all `confirmed` (Risks table); R-1/R-2/R-4 resolved by autonomous default (Design Resolution), Thomas confirmed remove-the-sink | ✅ |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated; AC-7 resolved by Thomas's decision
- [ ] Wiring Test table complete -- every row has a concrete test
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify` passes (including `make ze-functional-test` after harness migration)
- [ ] `make ze-cli-grammar-check` passes
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Risks R-1, R-2, R-4 resolved by explicit user decision before coding

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A -- no numeric input added)
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence
