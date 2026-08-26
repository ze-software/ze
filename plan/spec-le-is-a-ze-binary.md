# Spec: le is a ze binary

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/le-is-a-ze-binary.md` |
| Handoff | - |
| Updated | 2026-08-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make `le` a Go binary at `cmd/le/`, built on the same engine as `ze`: the same
registry, the same command grammar, the same pipe machinery. `ze` stays the
product. `le` becomes repo management and development. **The separation is USE,
and it is carried by the plugin sets and the binaries, never by the engine.**

Three goals, each standing on its own:

1. **One ENGINE, two plugin sets, two binaries (owner directive, 2026-08-26).**
   `ze` and `le` share the registry, the command grammar and the pipe
   machinery. They share NO plugins. `ze` is never compiled with `le` code and
   `le` is never compiled with `ze` plugins. The architecture must not PRECLUDE
   a crossing, since that is the test of whether the engine is genuinely shared
   rather than merely similar, but no build ever performs one. Two registries
   kept in agreement by hand is the failure this exists to prevent; a single
   binary carrying both plugin sets is the failure the never-linked rule
   prevents.
2. **The dev tooling stops being scripts.** 50 Go files carrying
   `//go:build ignore` become real packages: compiled by `go build ./...`, seen
   by `go vet` and the linter, callable as functions by their tests.
3. **`le` inherits the CLI contract** instead of reimplementing it: the
   keyword-before-value grammar, `| json`, `| yaml` and `| table` on every
   command, completion, `help`, and the exit-code conventions.

**What "shared engine" means mechanically.** Both binaries import the same
`internal/component/command/registry` and register through the same
`MustRegisterRootHandler`. Neither imports the other's plugins, and that is
provable rather than asserted: `go list -deps` over each binary must not name a
package belonging to the other's plugin tree. Measured 2026-08-26, the mechanism
already works this way for the existing programs -- `internal/perf/cli` is
absent from `ze`'s 630-package dependency list with `ze_perf` off and present
with it on.

**Strategy: port everything, then swap (owner directive, 2026-08-26).** The Go
`le` is built to completion ALONGSIDE the Python one. Nothing is deleted as it
is ported. When the Go side covers every feature, one changeover repoints the
shims and removes the Python side. This is the duplicate-then-replace route the
Makefile-to-`le` migration used.

**Non-goal:** redesigning what the tools DO. Behaviour is preserved and proven
preserved. This spec moves and rewires; it does not rewrite checks.

Target layout:

| Path | Holds | Linked into |
|------|-------|-------------|
| `internal/component/command/`, `internal/core/` | the shared ENGINE: registry, grammar, pipes | both |
| `cmd/ze/` | the product's entry point and composition root, unchanged | `ze` |
| `internal/`, `internal/plugins/` (product trees) | `ze`'s plugins | `ze` only |
| `cmd/le/main.go` | entry point | `le` |
| `cmd/le/register.go` | composition root: blank imports say what `le` carries | `le` |
| `le/<tool>/` | `le`'s plugins, one package per tool | `le` only |
| `le/parity/` | the census measuring how much is ported | `le` only |

→ Decision: `le`'s plugins sit in a top-level `le/` tree, NOT under `internal/`
alongside the product's. The directory is the statement that these are a
different program's plugins over the same engine, and it makes the never-linked
rule readable rather than merely enforced.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/core-design.md` - the small-core-plus-registration pattern this reuses
  → Decision: the core discovers features through registries and never imports them directly, which is exactly the property that lets one import move a tool between binaries
  → Constraint: no per-feature switch, field, or factory may be added to a shared package to make a tool reachable

- [ ] `ai/rules/cli.md` - the command contract every `le` command inherits
  → Constraint: keyword before value; every command supports all pipe operators; the response payload is structured data so `| json`, `| yaml` and `| table` each render it
  → Constraint: a ported tool that prints free text and returns no structured payload has NOT met the contract

- [ ] `ai/patterns/cli-command.md` - the structural template for a command
  → Constraint: use it per tool; do not invent a per-tool shape

- [ ] `ai/rules/architecture.md` - tier rules
  → Decision: `le/` sits OUTSIDE `internal/`, so step 1 must decide whether `make ze-tier-check` extends to it. What matters is that `le/` may import the engine and must not import product plugins

- [ ] `ai/rules/no-layering.md` - "delete X first, then implement Y"
  → Decision: the owner has chosen duplicate-then-swap instead. The rule's concern, silent drift between two implementations, is answered by the parity gate (AC-9), not waived

- [ ] `ai/rules/simplicity.md` - the simplest fully correct answer
  → Decision: no per-tool `cmd/` directories and no `dev-gates.txt` build-tag manifest, because neither has a consumer

### RFC Summaries (Scope: protocol)

N/A - this spec touches no protocol surface.

**Key insights:** (minimal context to resume after compaction)

- `internal/perf/cli/register.go` is the whole precedent: `init()` calls `registry.MustRegisterRootHandler("perf", handler, Meta{...})`, and `cmd/ze/ze_perf_register.go` is six lines, a build tag plus one blank import.
- `rootHandlers` is package-level PER-PROCESS state, so two packages owning one name meet only when both are linked into one binary. Under the never-linked rule they never are, which is why `le perf` and `ze perf` can coexist. `MustRegisterRootHandler` panics on a duplicate, so if the rule were ever broken the failure is a startup crash rather than a silent shadow.
- `go list -deps` is how the never-linked rule is CHECKED. Measured: `internal/perf/cli` is absent from `ze`'s 630-package list with `ze_perf` off and present with it on.
- The 180 seconds of `go run` link time is a consequence, never the argument. The argument is the shared engine.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)

- [ ] `internal/perf/cli/register.go` - the per-program registration precedent, read in full
- [ ] `internal/component/command/registry/registry.go` - `RegisterRootHandler` at :246 refuses empty name, nil handler and duplicate name; `MustRegisterRootHandler` at :264 panics on that error; `Meta` at :148 is `{Description, Mode, Section, Subs, SubsFunc}`
- [ ] `cmd/ze/ze_perf_register.go` - six lines: `//go:build ze_perf` and one blank import
- [ ] `scripts/le/gateapp.py` - `action(opts, gates, env) -> int`; the first failing gate's exit code wins; `--json` is refused on a gate with no `json_flag`
- [ ] `scripts/le/devtools/gate.py` - `Gate(name, argv, why, json_flag, writes)` and `GateSet(area, gates)`; `why` is data, which is what lets `--list` print reasons
- [ ] `scripts/docvalid/scripts_test.go` - every `//go:build ignore` tool is tested by `exec.CommandContext(ctx, "go", "run", ...)`
- [ ] `mk/test-functional.mk` - what a completed port looks like: a header recording what moved, what STAYED and why
- [ ] `feature-gates.txt` - the manifest pattern, one line per gated package, every consumer deriving from it

`le` is Python at `scripts/le/`: 59 files, 8007 lines of code and 4011 of tests.
`./le` is a shim putting `scripts` on `sys.path`. It declares 22 areas and 156
gates. Each area exposes `add_arguments` / `options` / `action` / `main`,
dispatched by `scripts/le/registry.py`.

The 156 gates run four ways: 66 Python imported in-process, 36 `go run`, 31
shell, 23 `go test` or `go build`.

The Go half is 50 files across seven `scripts/` directories, every one
`package main` with `//go:build ignore`, reached only by `go run`. They are not
compiled by `go build ./...`, `go vet`, or the linter. Their tests drive them as
subprocesses, also by `go run`, at 40 call sites.

Measured on this machine, 2026-08-26: `go run` of a warm-cached script costs
2411 ms, a Python gate imported in-process 165 ms, and a no-op `go build` of an
unchanged package 270 ms. 36 `go run` gates in a sweep plus 40 in the test suites
is roughly 180 seconds of linker time per full run, buying nothing.

**Behavior to preserve:** (unless the user explicitly said to change it)

- Every `make` target name that exists today, and the behaviour behind it.
- The exit-code rule: the first failing gate's own code propagates. `commit_helper.py` distinguishes 3 from 1, so a flattened 1 breaks it.
- The `why` text attached to each gate, which `--list` prints.
- `ZE_REPO_ROOT` and its discovery contract.
- What `mk/test-functional.mk` records as deliberately NOT ported: a lazily-expanded shell pipeline, and prerequisite EDGES that are a make-level concern.

**Behavior to change:** (only what the user asked for)

- The language and the process model: in-process Go dispatch replaces forked `go run` and imported Python.
- The command surface gains the pipe operators, completion and structured output it does not have today.
- Dev tooling becomes compiled, so `go vet`, the linter and the tier check begin to govern it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

- A developer types `make <target>`, `./le <command>`, or `ze <command>`.
- Format at entry: an argv vector; no wire bytes, no config tree.

### Transformation Path

1. Today: a `mk/*.mk` shim forwards to `./le`; the Python shim puts `scripts` on `sys.path`; `registry.py` resolves the area; `gateapp.action` selects gates; `devtools/gate.run_gate` imports a Python script in-process or forks `go run` or a shell command.
2. During the port: unchanged. Every target and every `./le` invocation still reaches Python, so a half-finished Go side cannot break a developer.
3. After the swap: `cmd/le/main.go` dispatches through `internal/component/command/registry`; the handler registered by `le/<tool>/register.go` runs in-process; the result is a structured payload the pipe operators render.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Make shim ↔ `le` | argv, exit code | No |
| `le` ↔ registry | `MustRegisterRootHandler` at init, `LookupRoot` at dispatch | No |
| Dev tooling ↔ the compiled module | tool packages join `go build ./...`, `go vet`, the tier check | No |
| `le` ↔ `ze` | a blank import in either composition root | No |

### Integration Points

- `internal/component/command/registry` - the existing registry both binaries share; `le` adds root handlers to it exactly as `internal/perf/cli` does.
- `internal/component/command` - pipe filters, aliases, answer shapes and column order, which a ported tool registers rather than reimplements.
- `mk/*.mk` and `Makefile` - shims whose target names are preserved and whose bodies change once, at the swap.

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
| A-1 | The 50 tool files compile once `//go:build ignore` is dropped | They type-check today under `go run` | A burst of latent errors on first port | Drop the tag on ONE file and build, in step 1 | unvalidated |
| A-2 | No tool's imports pull anything the module would rather not have | Inspected `scripts/lint` and `scripts/docvalid` only; the rest is assumption | Module dependency growth | `go list -deps` per tool package before porting it | unvalidated |
| A-3 | Behaviour is preserved by moving `main()` to `Run(args) int` | The signature is the only forced change | Silent behaviour drift | Per tool: run old and new over the real tree, diff exit code and output | unvalidated |
| A-4 | Most Python test CONTENT transfers as intent, not as code | Cases and reasoning are language-independent; the harness is not | Rewrite cost higher than planned | Port one area's tests first and measure | unvalidated |
| A-5 | A `le` command name that a `ze` root also uses is HARMLESS, because the two are never linked into one binary | `rootHandlers` (`internal/component/command/registry/registry.go`) is package-level per-process state, so two packages owning one name meet only when both are linked. The owner ruled on 2026-08-26 that they never are | If a build ever did link both, `MustRegisterRootHandler` panics at init -- loud, never a silent shadow | AC-2 and AC-3 prove the never-linked premise by `go list -deps`; the collision needs no separate guard | **confirmed by design, 2026-08-26.** Measured: today's 22 `le` area names collide with none of `ze`'s 34; the verb-first split collides on exactly one, `perf`, and under the never-linked rule that costs nothing |
| A-6 | A ported tool can answer structured data without redesigning it | `ai/rules/cli.md` requires it; unmeasured for tools that print prose reports | The port becomes a rewrite for those tools | Port `scripts/lint` first and see what its output costs | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A ported test silently weakens: 40 subprocess tests become function calls | `audit-test-relaxation.py` reports `[WEAKENED]` | Each conversion states, per case, what the old assertion proved and where that proof now lives; a `test/weakened.md` row per carrier |
| R-2 | The two sides DRIFT during the port: a gate changes in Python and not in Go | The parity gate goes red | Parity runs in `ze-precommit-verify` from step 1, so drift is caught in the commit that introduces it |
| R-3 | `le` and `ze` diverge in CLI behaviour despite the shared registry | A pipe operator works on a `ze` command and not an `le` one | The wiring test drives a pipe operator through an `le` command |
| R-4 | Stale binary: `le` is built, so an old binary silently runs an old gate. Python could not fail this way | A gate passes on code that should fail it | `make` prerequisite on the binary; the 270 ms no-op build bounds the check |
| R-5 | The port stalls half-done and two `le`s live indefinitely | The parity count stops falling across a week | The count is published by `le parity`, and it is the one number that says how far along this is |
| R-6 | The swap lands with a feature missing that nobody noticed | Parity green while a Make target fails to resolve | The swap's precondition is parity AND every Make target resolving, both mechanical |
| R-7 | A tool's output is prose, and making it structured changes what operators read | A ported tool's output diff is large in step 2 | Treat output shape as behaviour: the parity test diffs it, and a deliberate change needs its own row here |
| R-8 | **The never-linked rule erodes and something quietly imports across the line.** It is the premise the shared engine rests on, and nothing about Go stops a developer adding one blank import. The day it happens, `ze` grows a dev-tool dependency, or `le` grows a product one, and the two plugin sets start sharing a binary | AC-2 or AC-3 goes red: a `le/` package appears in `ze`'s dependency list, or a product plugin in `le`'s | The invariant is CHECKED, not documented. `go list -deps` over each build flavour, in `ze-precommit-verify` from step 1. It discriminates: measured 2026-08-26, `internal/perf/cli` is absent from `ze`'s 630-package list with `ze_perf` off and present with it on, so the check can see the difference it exists to see |
| R-9 | The shared engine is shared in NAME only: `le` accretes its own grammar, its own pipe handling, its own help, and the two drift into similar-looking programs with nothing in common | A `le` package under `le/` starts declaring what `internal/component/command` already provides | AC-3b pins that both binaries link the one registry and neither declares its own. The engine is the thing this spec exists to share, so a second implementation of any part of it is the failure, not a convenience |

## Blast Radius

| Surface | Effect |
|---------|--------|
| `scripts/` | 11 directories; 4507 path references across the tree |
| `mk/*.mk` and `Makefile` | Every shim keeps its target NAME; only the command behind it changes, and only at the swap |
| `.claude/hooks/` | Reference `scripts/dev/ze-run.sh`, `session-scratch.sh`, `commit_helper.py`, `spec-session.sh` |
| `CLAUDE.md`, `ai/rules/`, `ai/INDEX.md` | Name script paths throughout |
| CI workflows | Call `make` targets, which are preserved |
| `go.mod` | Tool imports become module imports |

Getting out: until step 10 nothing routes to Go, so abandoning the work costs
only the unreferenced `cmd/le/` tree. After step 10 the exit is a revert of one
changeover commit.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le <name>` typed by a developer | → | the root handler registered by `le/<tool>/register.go` | `TestLeDispatchesEveryRegisteredTool` |
| a `ze` build of any flavour | → | its dependency list, which must name no `le/` package | `TestZeLinksNoLePlugin` |
| a `cmd/le` build | → | its dependency list, which must name no product plugin package | `TestLeLinksNoProductPlugin` |
| either binary's dispatch | → | the one `internal/component/command/registry`, with no second registry declared | `TestBothBinariesShareOneRegistry` |
| `./le <name> \| json` | → | the tool's structured payload through `internal/component/command` pipe filters | `TestLeCommandAnswersStructuredData` |
| tab after `./le ` | → | the command tree built from the registry | `TestLeToolsAppearInCompletion` |
| `make <target>` for every pre-existing target | → | the shim's `le` invocation | `TestEveryMakeTargetResolves` |
| `go build ./...` over a `git archive HEAD` export | → | every registered tool package | `TestCommittedTreeBuilds` |
| binary init, importing both composition roots | → | `registry.RegisterRootHandler`'s duplicate rejection | `TestNoLeNameCollidesWithZe` |
| `./le <name>` from a shell, against the BUILT binary | → | the registered handler, end to end through the artifact a developer runs | `test/ui/le-binary-dispatches.ci` |

## Acceptance Criteria

| # | Criterion | Test |
|---|-----------|------|
| AC-1 | `cmd/le/` produces a binary dispatching every ported tool through `internal/component/command/registry` | `TestLeDispatchesEveryRegisteredTool` |
| AC-2 | `ze` links NO `le` plugin: `go list -deps` over every `ze` build flavour names no `le/` package | `TestZeLinksNoLePlugin` |
| AC-3 | `le` links NO product plugin: `go list -deps` over `cmd/le` names no package from the product plugin trees | `TestLeLinksNoProductPlugin` |
| AC-3b | Both binaries dispatch through the SAME engine: each links `internal/component/command/registry`, and neither declares a registry of its own | `TestBothBinariesShareOneRegistry` |
| AC-4 | No ported tool file carries `//go:build ignore`; all are built by `go build ./...` and seen by `go vet` | `TestNoPortedToolIsBuildIgnored` |
| AC-5 | Every ported tool's test calls it as a function; no test invokes it via `go run` | `TestNoTestShellsOutToGoRun` |
| AC-6 | Every Make target that existed before the swap still resolves and reaches the same behaviour after it | `TestEveryMakeTargetResolves` |
| AC-7 | Each ported tool answers structured data, so `\| json`, `\| yaml` and `\| table` render it | `TestLeCommandAnswersStructuredData` |
| AC-8 | A gate failure propagates its own exit code, never a flattened 1 | `TestFirstFailingGateExitCodeWins` |
| AC-9 | Parity is measured, not asserted: `le parity` enumerates every Python gate and every Go command and names each unported one. Red until zero, and it runs in `ze-precommit-verify` | `TestParityNamesEveryUnportedGate` |
| AC-10 | The committed tree builds and every registered tool loads from it | `TestCommittedTreeBuilds` |
| AC-11 | Behaviour is preserved per tool: old and new agree on exit code and output over the real tree | Per-tool parity test, named in each area's commit |
| AC-12 | After the swap nothing of the Python `le` remains: no `scripts/le/`, no `//go:build ignore` tool, no reference to either | `TestNoPythonLeRemains` |
| AC-13 | A duplicate root name is rejected at init rather than shadowing | `TestNoLeNameCollidesWithZe` |

## End-to-End User Stories

1. A developer runs `make ze-lint-changed` mid-port and it behaves exactly as before, because nothing routes to Go yet.
2. After the swap, the same command works identically, now in-process.
3. A developer runs `./le check docs | json` and gets a machine-readable report from a command nobody wrote JSON rendering for.
4. A developer needs a repo check on an appliance, adds one blank import to `cmd/ze/register.go`, rebuilds, and the command is there.
5. A developer adds a new check: one package under `le/`, a `register.go` with `init()`, one blank import in `cmd/le/register.go`. It appears in `le --help` and in completion with no further wiring, and it never reaches `ze`.
6. Anyone asks how far along the migration is, and `le parity` answers with a number and the names behind it.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunReturnsToolExitCode` | `le/<tool>/<tool>_test.go` | each ported tool's logic, called as a function | |
| `TestEveryPackageRegistersOneRootHandler` | `cmd/le/register_test.go` | one handler per package, Meta carries Description, Mode and Section | |
| `TestNoLeNameCollidesWithZe` | `cmd/le/register_test.go` | a duplicate root name panics at init rather than shadowing | |
| `TestParityNamesEveryUnportedGate` | `le/parity/parity_test.go` | the census names each unported gate, and is red while any remain | |
| `TestFirstFailingGateExitCodeWins` | `cmd/le/dispatch_test.go` | the failing gate's own code propagates, never a flattened 1 | |
| `TestNoPortedToolIsBuildIgnored` | `cmd/le/contract_test.go` | no ported file carries `//go:build ignore` | |
| `TestNoTestShellsOutToGoRun` | `cmd/le/contract_test.go` | no test invokes a tool via `go run` | |
| `TestNoPythonLeRemains` | `cmd/le/contract_test.go` | after the swap, no `scripts/le/` and no reference to it | |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| gate exit code propagated | 0-125 | 125 | N/A | N/A |
| parity unported count | 0-156 | 0 | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestLeDispatchesEveryRegisteredTool` | `cmd/le/dispatch_test.go` | a developer runs `./le <name>` for every registered tool and reaches it | |
| `TestZeLinksNoLePlugin` | `cmd/le/separation_test.go` | `go list -deps` over every `ze` flavour names no `le/` package | |
| `TestLeLinksNoProductPlugin` | `cmd/le/separation_test.go` | `go list -deps` over `cmd/le` names no product plugin package | |
| `TestBothBinariesShareOneRegistry` | `cmd/le/separation_test.go` | both link `internal/component/command/registry`, and neither declares a second one | |
| `TestLeCommandAnswersStructuredData` | `cmd/le/pipe_test.go` | `./le <name> \| json` renders without per-tool JSON code | |
| `TestLeToolsAppearInCompletion` | `cmd/le/completion_test.go` | tab completion offers the tool | |
| `TestEveryMakeTargetResolves` | `cmd/le/make_test.go` | every pre-existing `make` target still resolves | |
| `TestCommittedTreeBuilds` | `cmd/le/tracked_test.go` | the committed tree builds and every tool loads from it | |
| per-area parity test | `le/<area>/parity_test.go` | old and new agree on exit code and output over the real tree | |
| `le-binary-dispatches` | `test/ui/le-binary-dispatches.ci` | the BUILT `le` binary dispatches a registered command and renders `\| json` | |

The `.ci` is not a formality, and it is not a daemon test. Every Go test above
exercises a PACKAGE; none exercises the artifact a developer actually runs.
Between them sits the build, the composition root, and argv handling, which is
exactly where a stale or mis-composed binary fails (R-4). The `.ci` harness
already execs arbitrary commands -- `python3 run.py`, `sh run.sh`,
`./support-doctor-owner-check.py` are existing `exec=` forms -- so
`exec=./le ...` needs no new machinery. It is the one row that proves the
built thing works.

### Interop Tests (Scope: protocol)

N/A - no wire-visible behaviour changes.

**Discrimination requirement.** Every test above must be shown to FAIL when the
behaviour it pins is broken, by mutation, with the mutation recorded in the
commit (`ai/rules/interop-and-goal-validation.md`). A parity test that passes
because both sides are broken the same way proves nothing, and that is the
specific trap of a duplicate-then-swap migration.

## Files to Modify

- `Makefile`, `mk/*.mk` - AT THE SWAP ONLY: shims point at the `le` binary; target names unchanged
- `cmd/ze/register.go` (new or existing seam) - blank imports for any dev tool wanted in `ze`
- `ai/INDEX.md` - discovery rows for `le` commands
- `CLAUDE.md`, `ai/rules/commands.md` - script paths and how to run a gate, at the swap
- `.claude/hooks/*` - paths for tools leaving `scripts/dev`, at the swap
- `docs/architecture/core-design.md` - a note on the shared registry and the two composition roots

## Files to Create

- `cmd/le/main.go` - entry point
- `cmd/le/register.go` - composition root: blank imports
- `le/<tool>/<tool>.go` - one package per tool, exposing `Run(args []string) int`
- `le/<tool>/register.go` - `init()` calling `registry.MustRegisterRootHandler`
- `le/parity/` - the census measuring how much is ported
- `plan/deferrals/le-is-a-ze-binary.md` - deferral shard

## Implementation Steps

**Nothing routes to Go until step 10.** Every step before it is additive, so a
half-finished Go side cannot break a developer. Each step is its own commit,
green on its own.

**Scope is ALL of `scripts/` (owner directive, 2026-08-26): 280 code files, being
187 Python, 79 Go and 14 shell. When this spec closes, `scripts/` holds no code.**
Counted 2026-08-26.

| # | Step | PY | GO | SH | Refs |
|---|------|---:|---:|---:|-----:|
| 1 | `cmd/le/` skeleton, registration contract, wiring tests, A-5 check, A-1 build probe, `le parity` at 0 of 156. NO tool ported. The contract tests must be red for the right reason before anything moves | - | - | - | - |
| 2 | `scripts/lint` -- first end-to-end port, its test, its parity proof, and the measurement A-6 needs | - | 2 | - | 8 |
| 3 | `scripts/inventory` | - | 3 | - | 24 |
| 4 | `scripts/vendor` | - | 4 | - | 26 |
| 5 | `scripts/docvalid` | - | 3 | - | 78 |
| 6 | `scripts/status` | - | 9 | - | 271 |
| 7 | `scripts/codegen` | - | 8 | - | 491 |
| 8 | `scripts/checks` | - | 30 | - | 369 |
| 9 | `scripts/zeledon` | 2 | - | - | 26 |
| 10 | `scripts/evidence` | 19 | 4 | 2 | 300 |
| 11 | The 22 Python `le` areas (`scripts/le`) | 59 | - | - | 157 |
| 12 | `scripts/dev`, Python and Go halves | 107 | 16 | - | 2757 |
| 13 | The 12 shell scripts in `scripts/dev`, last of the ports and the highest risk. `ze-run.sh` is the job-admission wrapper every heavy command routes through; `session-scratch.sh`, `spec-session.sh`, `verify-status.sh` and `verify-lock.sh` are named by `CLAUDE.md`, by the hooks, and by every session's habits. Converting these changes an interface every agent uses, not merely an implementation | - | - | 12 | - |
| 14 | THE SWAP. Precondition: `le parity` reports zero unported AND every Make target resolves. Repoint every shim, delete `scripts/le/` and every ported script, update the hooks, `CLAUDE.md`, `ai/rules/`, `ai/INDEX.md` and every source anchor. One changeover | - | - | - | - |

→ Constraint: a shell script is not automatically a `le` command. `mk/test-fuzz.mk`
records the shape of the exception: the admission wrapper "STAYS HERE, and it
cannot move ... That is a Make-level concern about Make's own concurrency, and
`le` is not the right owner for it." Step 13 must say, per script, whether it
becomes a command, stays a thin wrapper the hooks keep calling, or does not move
-- and record the reason the way that header does.

→ Decision: steps 2 to 10 touch disjoint directories and may run in parallel
once step 1 exists. Steps 11 to 14 are sequential.

→ Decision: step 1 delivers no user-visible value and is not optional. A
migration that starts by porting a tool has no contract to port it into, and no
way to say how far along it is.

→ Constraint: the parity count must fall monotonically. A step that ports a tool
and leaves the count unchanged has not wired it.

→ Constraint: the swap is ONE changeover, not a trickle. A shim repointed early
puts a developer on a half-ported path, which is what this strategy avoids.

## Design Insights

The measured 180 seconds is NOT the argument for this change and must not be
cited as one. It comes from `go run` relinking and could be recovered by
compiling the existing scripts into one multi-tool binary with `le` left in
Python. The argument is the shared REGISTRY: two registries kept in agreement by
hand can drift, and Python cannot join Ze's. Speed is a consequence.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `cmd/le/` as its own binary | a `ze_le` build tag inside `cmd/ze/`, as `ze-perf` and `ze-chaos` do | The separation is USE. A distinct binary says so; a tag hides it behind a build flag |
| One shared registry | a second registry for dev tools, with a drift check between them | The whole point: a feature crosses by adding an import. A drift check is a worse version of not being able to drift |
| `le/` as a top-level tree, NOT `internal/le/` | `internal/le/`, mirroring `internal/perf/`; or `tools/le/` | Owner directive 2026-08-26: `le`'s plugins are not `ze`'s, and the two are never compiled together. `internal/perf/` is a per-program subtree of ONE program's tree; `le` is a different program. The directory is the statement, and it makes the never-linked rule readable rather than merely enforced. Cost: `make ze-tier-check` governs `internal/`, so step 1 must decide whether to extend it to `le/` |
| No per-tool `cmd/` directories | one small binary per tool, mirroring Python's module route | In Python the module route exists because there is no build step. In Go, `le <name>` IS that route. Per-tool binaries would be artifacts nobody consumes |
| No `dev-gates.txt` build-tag manifest | mirroring `feature-gates.txt` in full | Ze gates features because the appliance SHIPS and size matters. `le` never ships, so the tags have no consumer today. The structure leaves it a one-line addition later |
| Duplicate-then-swap | delete each Python original as its Go replacement lands, per `no-layering.md` | Owner directive, 2026-08-26, and the route the Makefile migration took. The rule exists so two implementations do not drift silently; here drift is made LOUD by the parity gate running in `ze-precommit-verify` from step 1. The cost is R-5, and the published count is what makes it visible |

## Known Limitations

- The 8007 lines of Python and 4011 lines of Python tests are sunk. Test CONTENT transfers as intent; the code does not.
- `le` gains a build step, so a stale binary can run an old gate. Python could not fail this way. R-4 mitigates it; it does not remove it.
- Build-tag gating of `le` features is deliberately not built. If a slim `le` is ever wanted, `dev-gates.txt` is the shape to add.
- Between step 1 and step 10 the repository carries two implementations of the same tooling. That is the accepted cost of the chosen strategy.
- **`internal/` conflates the engine with `ze`'s product code, and this spec does not fix it.** Measured 2026-08-26: the engine `le` needs is 12 packages (`component/command`, `component/command/registry`, `component/config/storage`, `component/plugin/registry`, and `core/env`, `core/envcatalog`, `core/helpfmt`, `core/metrics`, `core/selector`, `core/slogutil`, `core/stringsx`, `core/textbuf`). The rest is one program's product: 43 directories under `internal/component/` and 64 under `internal/plugins/`. So `internal/` is not "shared", it is "`ze`, plus a small engine nobody has named". Putting `le` at top level rather than at `internal/le/` was chosen partly for this reason: `internal/le/` would sit `le`'s plugins inside the tree they must never be linked with, while a top-level `le/` makes the never-linked rule readable at a glance. The symmetric alternative -- `internal/ze/` beside `internal/le/`, engine left in `internal/` -- was measured at **32,419 references** (24,037 for `internal/component/` and 8,382 for `internal/plugins/`), larger than this entire migration and touching every product file rather than the tooling. It is a real improvement and it needs its own spec, sequenced as: name and separate the 12-package engine first, then `internal/ze/`, then `le` could move symmetrically. The first of those three is useful on its own, because it would let `go list -deps` prove the ENGINE boundary the way AC-2 and AC-3 prove the plugin boundary (owner decision, 2026-08-26: option A, top-level `le/`).

## RFC Documentation (Scope: protocol)

N/A - this spec touches no protocol surface.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`le/*`, `cmd/le/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination
- [ ] `le parity` reports zero unported, which is the swap's precondition

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)
- [ ] Every new test shown to FAIL under a recorded mutation of the behaviour it pins

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

### Integration Checklist

| Row | Answer |
|-----|--------|
| YANG schema + validation | N/A - `le` commands are not operator config |
| CLI grammar | Yes - `ai/patterns/cli-command.md`, per tool |
| Completion | Yes - `TestLeToolsAppearInCompletion` |
| Functional test | Yes - per-area parity tests plus `cmd/le/dispatch_test.go` |
| Env var | N/A - no new env var; `ZE_REPO_ROOT` unchanged |
| Doctor check + diagnostic code | N/A - adds no runtime dependency to the daemon |
| Prometheus counters | N/A - build-host tool |
| BGP family surface | N/A - no protocol surface |

### Documentation Update Checklist (BLOCKING)

| Row | Answer |
|-----|--------|
| `ai/INDEX.md` keyword + task rows | Yes - one row per `le` command area |
| `CLAUDE.md` script paths | Yes - at the swap |
| `ai/rules/commands.md` | Yes - how to run a gate changes at the swap |
| `docs/architecture/core-design.md` | Yes - the shared registry and the two composition roots |
| `docs/contributing/` | Yes - how to add a new check |
| `docs/guide/command-reference.md` | N/A - `le` is not an operator surface |
| Source anchors (`<!-- source: -->`) | Yes - every anchor naming a moved file |
| `docs/features.md` | N/A - not a product feature |
| Plugin docs | N/A - no plugin change |
| API/RPC/event docs | N/A - no API change |
| Telemetry docs | N/A - no metric change |
| Runtime inventory docs | N/A - no registry inventory change |

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path |
| Correctness | The first failing gate's exit code propagates; `commit_helper.py` still distinguishes 3 from 1 |
| Naming | No `le` root name collides with a `ze` root name; gate names keep the `ze-` spelling docs and rules already cite |
| Data flow | Registration only: no per-tool switch, field or factory added to a shared package |
| Rule: `ai/rules/cli.md` | Each ported tool answers structured data, so the pipe operators render it |
| Rule: `ai/rules/testing.md` | Each subprocess-to-function test conversion names what the old assertion proved and where that proof now lives |
| Rule: `ai/rules/no-layering.md` | The parity gate is live from step 1, so the two implementations cannot drift silently |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `cmd/le/` builds | `go build -o bin/le ./cmd/le` |
| Every registered tool dispatches | `go test ./cmd/le/ -run TestLeDispatchesEveryRegisteredTool` |
| No ported tool is build-ignored | `grep -rL 'go:build ignore' le/` returns every file |
| No test shells out to `go run` | `grep -rn '"go", "run"' --include=*_test.go le/ cmd/le/` is empty |
| Parity count | `./le parity --json` |
| Every Make target resolves | `go test ./cmd/le/ -run TestEveryMakeTargetResolves` |
| Nothing of Python `le` remains (after the swap) | `test ! -d scripts/le` and `grep -rn 'scripts/le' . ` is empty |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | `le` takes developer argv, not untrusted input; a tool that reads repository files must still bound what it reads |
| Resource exhaustion | A tool walking the tree bounds its walk; no unbounded `make` over user-controlled counts |
| Error leakage | A gate's failure output names the file and the reason, and does not print environment or credentials |
| Authorization failing open | Not applicable to `le`, but any tool ported into `ze` becomes reachable on an appliance: check its Meta Mode and Section before adding that import |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Parity gate red after a port | The tool was not wired; do not lower the count by hand |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Review Gate

<!-- Filled by /ze-close via /ze-review. Do not fill in advance. -->

| Run | Verdict | Rounds | Lenses | Artifact |
|-----|---------|--------|--------|----------|
| | | | | |
