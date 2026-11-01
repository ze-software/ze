# Spec: plugin-fork-environment

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P2 |
| Release reason | A ze env key an operator or a test sets for a forked plugin silently never reaches it, so the plugin runs on its default with no sign |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-01 |

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

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| The plugin fork | `(*Process).startExternal`, `internal/component/plugin/process/process.go`; `newPluginShell`, `exitwait_*.go` | Runs the operator's `run` string through `/bin/sh -c` with the daemon's environment plus the `ZE_PLUGIN_*` connect-back variables | Extends: the environment it hands the child carries each ze key under a shell-safe spelling, or the fork stops going through a shell |
| Env key lookup | `normalize` and `Get`, `internal/core/env/env.go`; `docs/architecture/config/environment.md` | Reads a key in dot or underscore spelling, any case; a hyphen is kept | Extends: decides the shell-safe spelling of a hyphenated key (read `-` as `_`, or ban `-` at `MustRegister`) |
| The runner's shell-safe test keys | `(*Runner).testBudgetEnv`, `internal/test/runner/runner_exec_util.go`, `TestTestBudgetEnvSurvivesAShell` | Writes `ze_test_budget` so a fixture forked as a plugin reads it | Replaced by the general fix once the daemon passes keys itself; its test stays as a regression |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through` | 2026-11-01 | `startExternal` forks through `sh -c`, so a dotted or hyphenated ze key never reaches a forked plugin |
