# Spec: stress-repro-refuses-a-stale-binary

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Updated | 2026-08-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`internal/le/stressrepro/run.go` decides whether a suspected flake is real. When it
runs a binary older than the tree, its answer is not merely wrong, it is
CONFIDENTLY wrong in both directions: a fix under test looks "still
reproducing" because the run never contained it, and an unrelated failure
already fixed in the tree looks like a fresh reproduction.

`_bin_from_env` already documents that exact incident: the tool once hardcoded
`bin/ze`, and the repair made it honour `ZE_BIN` / `ZE_TEST_BIN`. The FALLBACK
was left at `bin/ze`, and in this repository that path is stale by
construction, because `internal/le/session/actions.go` builds every canonical binary into a
per-session directory and the functional make targets run against an isolated
pair under `tmp/`. So the trap the repair described is still reachable by the
one route people take: invoking the tool without exporting the variables.

`ensure_binaries` checks only that the files EXIST.

Measured on 2026-08-15: an invocation without `ZE_BIN` set reported `***
REPRODUCED on invocation 1 ***` for `forward-overflow-two-tier`. The capture
showed `unknown field in peer: attach`, a config grammar the working tree
parses and the day-old `bin/ze` did not. Nothing about the suspected flake was
exercised. The same run with the session binaries exported was needed to get
any answer at all.

## Acceptance Criteria

| ID | Criterion |
|----|-----------|
| AC-1 | `ensure_binaries` refuses a binary older than the newest source file it would exercise, and names the build command instead of running |
| AC-2 | The refusal states WHICH binary is stale and by how much, so the reader is not left comparing timestamps by hand |
| AC-3 | A run whose binaries are current is unaffected: no new output, no new failure mode |
| AC-4 | The check is driven from the tool's entry point in a test, not from the helper alone (`ai/rules/evidence.md`) |
| AC-5 | The fallback path is reconsidered: either default to the session directory that `internal/le/session/actions.go` owns, or refuse to guess and require the variables |

## Notes

Related in kind, not in code: `ai/rules/commands.md` documents the same
false-red class for a bare `go test` (drops feature tags) and for launching the
runner binary directly. This is that class in the one tool whose output is a
verdict about whether a defect exists.
