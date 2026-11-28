# Spec: Daemon runtime one-off defects (7) - Test mock and CLI subcommands flush crashlog on --help and flag errors

| Field | Value |
|-------|-------|
| Status | design |
| Scope | le |
| Priority | P1 |
| Release reason | Fixes lost `--help` and flag-error stderr output on all seven remaining `le mock <server>` subcommands and `internal/test/cli/` commands that still call `flag.ExitOnError` and exit past `crashlog.Flush` |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-daemon-runtime-one-off-defects.md`. This child owns the one open `internal/test/` journal row routed from the umbrella.

## Task

Fix `--help` and flag-error output loss across the remaining `internal/test/mock/` and `internal/test/cli/` subcommands (`output-lost-to-an-exit-past-the-flush`, 2026-09-06):

`cmd/ze/main.go` installs `defer flushCrashlog()` around `dispatchMain`. When a subcommand builds its `flag.FlagSet` with `flag.ExitOnError`, Go's `flag` package calls `os.Exit(0)` on `--help` and `os.Exit(2)` on an unknown flag, bypassing every deferred function including `flushCrashlog()`, so help text and flag errors buffered in `crashlog`'s stderr relay can be truncated or lost. `internal/test/mock/dns/dns.go` (L78) was already fixed to use `flag.ContinueOnError` and return cleanly (`0` on `errors.Is(err, flag.ErrHelp)`, `2` on other parse errors).

Apply the same `flag.ContinueOnError` pattern to every remaining mock server and test CLI subcommand:
- `internal/test/mock/peeringdb/peeringdb.go`
- `internal/test/mock/radius/radius.go`
- `internal/test/mock/irr/irr.go`
- `internal/test/mock/rpki/rpki.go`
- `internal/test/mock/cymru/cymru.go`
- `internal/test/mock/rtr/rtr.go`
- `internal/test/mock/tacacs/tacacs.go`
- Any remaining `flag.ExitOnError` subcommands under `internal/test/cli/` (`cmd_peer.go`, `cmd_editor.go`, `cmd_l2tp_scale.go`, `cmd_bgp.go`, `cmd_vpp.go`, and any siblings found by grep).

Add a package/tree guard test ensuring no `flag.ExitOnError` remains under `internal/test/mock/` or `internal/test/cli/`, plus behavioral tests proving `--help` returns exit 0 with usage written to stderr (returned through normal Go return so `defer flushCrashlog()` runs) and unknown flags return exit 2 with error output.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts
- [ ] `ai/doors/le-tooling.md` - `le` personality, `internal/test`, crashlog flush discipline
- [ ] `docs/functional-tests.md` - mock servers and test CLI subcommands

## Current Behavior (MANDATORY)

Verified at HEAD on 2026-11-26:

- [ ] `internal/test/mock/dns/dns.go` - L78 uses `flag.ContinueOnError` and returns cleanly on `flag.ErrHelp` and parse errors so `defer flushCrashlog()` drains stderr.
- [ ] `internal/test/mock/{peeringdb,radius,irr,rpki,cymru,rtr,tacacs}/` - all seven still build `flag.NewFlagSet(..., flag.ExitOnError)`.
- [ ] `internal/test/cli/` - `cmd_peer.go`, `cmd_editor.go`, `cmd_l2tp_scale.go`, `cmd_bgp.go`, `cmd_vpp.go` (and any sibling files) still build `flag.NewFlagSet(..., flag.ExitOnError)`.

**Behavior to preserve:**
- `--help` prints usage to the flag set's output stream and results in exit status 0.
- Unknown or malformed flags print the flag parse error and usage and result in exit status 2.
- All mock server runtime behavior after flag parsing stays identical.

**Behavior to change:**
- Every `flag.NewFlagSet` under `internal/test/mock/` and `internal/test/cli/` uses `flag.ContinueOnError`, returning exit 0 on `errors.Is(err, flag.ErrHelp)` and exit 2 on parse errors instead of calling `os.Exit` inside `flag`.

## Existing Mechanisms (MANDATORY)

Verified at HEAD on 2026-11-26.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Crashlog deferred flush | `cmd/ze/main.go` `defer flushCrashlog()`; `cmd/ze/dispatch.go` `flushCrashlog`; `cmd/ze/crash_flush_test.go` | Drains `crashlog` stderr queue whenever `dispatchMain` returns normally (or panics), but cannot run if a callee invokes `os.Exit` directly | Extends: ensures mock and test CLI subcommands return cleanly instead of calling `os.Exit` via `flag.ExitOnError` |
| DNS mock `ContinueOnError` pattern | `internal/test/mock/dns/dns.go` L78 | Uses `flag.ContinueOnError` and maps `flag.ErrHelp` to exit 0 and other flag errors to exit 2 | Extends: applies the identical pattern to the remaining seven mocks and `internal/test/cli/` commands |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Before you write code in a subsystem | the door every change reads | "Grep for the pattern that already exists before you design one" | agrees | Reuses the exact `flag.ContinueOnError` pattern already established in `internal/test/mock/dns/dns.go` across every sibling mock and test CLI command. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `internal/test/mock/*` and `internal/test/cli/*` | "`le` is a personality of `cmd/ze`." | agrees | Because mocks and test CLI subcommands run through `cmd/ze/main.go`, returning instead of `flag.ExitOnError` lets `defer flushCrashlog()` drain stderr cleanly. |
| `docs/architecture/testing/ci-format.md` | Syntax Overview | `// Design:` page of `internal/test/mock/*` and `internal/test/cli/*` | "All lines use key=value format with `:` separators:" | agrees | Preserves all mock and CLI flags and exit codes (0 on `--help`, 2 on bad flag) while ensuring usage text is flushed before process exit. |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | anchored to `internal/test/mock/rtr/rtr.go` | silent (page covers Docker interop suites and peer scenarios, not `flag.ErrorHandling` in mock `--help`) | silent | Switching `rtr.go` from `flag.ExitOnError` to `flag.ContinueOnError` changes no RTR wire behavior or interop suite. |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | This spec edits only CLI flag error handling in mocks and test helpers and touches no interop capability checks. |
| `docs/architecture/testing/runner-architecture.md` | Layers | anchored to `internal/test/cli/cmd_editor.go` and `cmd_bgp.go` | "Every functional suite moves through the same five stages:" | agrees | Keeps runner invocation exit codes intact while preventing lost stderr output on `--help` or flag errors. |
| `docs/architecture/overview.md` | 1. What is Ze? | anchored to `internal/test/cli/cmd_bgp.go` | silent (high-level repository overview does not cover `flag.ContinueOnError` in test mocks) | silent | Only internal test helper flag-set error handling changes. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `le mock <name> --help` (or unknown flag) and `le test <subcmd> --help` invoked through `cmd/ze/main.go`.

### Transformation Path
1. `cmd/ze/main.go` initializes `crashlog`, defers `flushCrashlog()`, and dispatches to the mock or test CLI entrypoint.
2. Entrypoint parses flags with `flag.ContinueOnError` -> on `flag.ErrHelp` returns `0`; on other parse error returns `2`.
3. `cmd/ze/main.go` runs `flushCrashlog()` before exiting with the returned code, guaranteeing all usage/error bytes reach stderr.

### Boundaries Crossed
- Stays entirely within `internal/test/` (dev tooling). No product `ze` binary imports `internal/test/` (`TestNormalZeLinksNoInternalLe`).

### Integration Points
- `internal/test/mock/*` and `internal/test/cli/*` entrypoints called by `le`.

### Changed-Wire Census

No wire format, RPC payload, event schema, or external protocol changes in this spec. Changes are limited to Go `flag.ErrorHandling` mode in test-harness CLI entrypoints.

### Architectural Verification
- [ ] Zero `flag.ExitOnError` occurrences remain under `internal/test/mock/` and `internal/test/cli/`
- [ ] Every `le mock <name> --help` returns exit 0 and prints full usage through normal return (flushing `crashlog`)

## Risks & Assumptions

### Assumptions
- Every entrypoint under `internal/test/mock/` and `internal/test/cli/` already returns an `int` exit code (or `error`) to its caller in `internal/le/`. Any entrypoint that currently returns nothing and relies on `flag.ExitOnError` to terminate will return its exit code or error cleanly.

### Risks
- Ensure `fs.SetOutput(os.Stderr)` (the default) is preserved so `--help` output still goes to stderr as existing callers expect.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `Run([]string{"--help"})` on each mock server and test CLI command | → | `flag.ContinueOnError` return path -> `defer flushCrashlog()` | `TestMocksAndTestCLIAvoidFlagExitOnError` + per-mock `--help` tests under `internal/test/mock/` and `internal/test/cli/` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Invoking `--help` or an unknown flag on each of the seven remaining mock servers (`peeringdb`, `radius`, `irr`, `rpki`, `cymru`, `rtr`, `tacacs`) and every `internal/test/cli/` subcommand (`output-lost-to-an-exit-past-the-flush`, 2026-09-06) | Each entrypoint parses flags with `flag.ContinueOnError`, writes complete usage or flag-error text to stderr, and returns exit 0 on `flag.ErrHelp` and exit 2 on invalid flags through normal Go return so `cmd/ze/main.go`'s `defer flushCrashlog()` flushes all output | `internal/test/mock/dns/dns.go` already uses `flag.ContinueOnError` and keeps its existing behavior unchanged |
| AC-2 | Running the package AST/source guard tests under `internal/test/mock/` and `internal/test/cli/` (`output-lost-to-an-exit-past-the-flush`, 2026-09-06) | `TestMocksAvoidFlagExitOnError` and `TestCLISubcommandsAvoidFlagExitOnError` pass on the clean tree and fail naming file and line whenever `flag.ExitOnError` is introduced in any Go file under `internal/test/mock/` or `internal/test/cli/` | None; every flag set under `internal/test/mock/` and `internal/test/cli/` must return cleanly via `flag.ContinueOnError` |

## TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| `TestMocksAvoidFlagExitOnError` and `--help` return-code checks | `internal/test/mock/help_flush_test.go` | AC-1, AC-2 |
| `TestCLISubcommandsAvoidFlagExitOnError` | `internal/test/cli/help_flush_test.go` | AC-1, AC-2 |

### Functional Tests

| Test | AC |
|------|----|
| Subprocess test executing `le mock <name> --help` verifying exit 0 and non-empty usage on stderr after `crashlog.Flush` | AC-1 |

## Files to Modify
- `internal/test/mock/peeringdb/peeringdb.go`
- `internal/test/mock/radius/radius.go`
- `internal/test/mock/irr/irr.go`
- `internal/test/mock/rpki/rpki.go`
- `internal/test/mock/cymru/cymru.go`
- `internal/test/mock/rtr/rtr.go`
- `internal/test/mock/tacacs/tacacs.go`
- `internal/test/cli/cmd_peer.go`
- `internal/test/cli/cmd_editor.go`
- `internal/test/cli/cmd_l2tp_scale.go`
- `internal/test/cli/cmd_bgp.go`
- `internal/test/cli/cmd_vpp.go` (plus any sibling `internal/test/cli/*.go` file holding `flag.ExitOnError`)

## Files to Create
- `internal/test/mock/help_flush_test.go`
- `internal/test/cli/help_flush_test.go`

### Integration Checklist
- [ ] Every mock and test CLI entrypoint returns cleanly on `--help` and invalid flags
- [ ] Guard test prevents reintroducing `flag.ExitOnError` under `internal/test/mock/` and `internal/test/cli/`

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/functional-tests.md` - no user-facing flag change; verify mock CLI usage examples remain accurate

## Design presentation

### What changes
- Replaces `flag.ExitOnError` with `flag.ContinueOnError` across all seven remaining mock servers under `internal/test/mock/` and all subcommands under `internal/test/cli/`, returning exit 0 on `flag.ErrHelp` and exit 2 on parse errors, guarded by an AST/source test.

### Before and after
- **Before:** Running `le mock <name> --help` on any mock other than `dns` called `os.Exit(0)` inside Go's `flag` package, bypassing `cmd/ze/main.go`'s `defer flushCrashlog()` and losing buffered usage output.
- **After:** Every mock and test CLI subcommand returns cleanly through `dispatchMain` and `defer flushCrashlog()`, delivering complete usage and flag-error output every time.

### Alternatives set aside
- **Calling `crashlog.Flush()` from a custom `fs.Usage` hook while keeping `flag.ExitOnError`:** Rejected because `flag.ExitOnError` still bypasses every deferred cleanup in the call stack and duplicates flush calls instead of matching the clean `flag.ContinueOnError` pattern already established in `internal/test/mock/dns/dns.go`.

### Documents departed from
None.

### Choices for Thomas
none

## Implementation Steps
1. Grep `internal/test/` for every remaining `flag.ExitOnError` call site.
2. Add red guard and `--help` tests in `internal/test/mock/help_flush_test.go` and `internal/test/cli/help_flush_test.go` and confirm red at HEAD.
3. Convert every `flag.ExitOnError` site under `internal/test/mock/` and `internal/test/cli/` to `flag.ContinueOnError` following `internal/test/mock/dns/dns.go`.
4. Run `go test -race ./internal/test/mock/... ./internal/test/cli/...` and mark the owned journal row `fixed:` in `plan/journal/output-lost-to-an-exit-past-the-flush.md`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `output-lost-to-an-exit-past-the-flush` | 2026-09-06 | `ze-test <mock>` help |

## Work Not Done

Sibling runtime defects routed from `plan/release/fixes/spec-daemon-runtime-one-off-defects.md` live in Children 1 through 6.

## Review Gate

- [ ] Code review clean (`/ze-review`)
- [ ] Spec compliance verified (`/ze-review-spec`)
- [ ] Owned journal row marked `fixed:` in `plan/journal/output-lost-to-an-exit-past-the-flush.md`

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run label mock-help-flush command go test -race ./internal/test/mock/... ./internal/test/cli/...`
