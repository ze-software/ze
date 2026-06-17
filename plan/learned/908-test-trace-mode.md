# 908 -- test-trace-mode

Spec: `plan/spec-test-trace-mode.md` (closed retroactively). Implemented in commit
`093bf7bda` before the spec was updated from skeleton status.

## Context

All three directive-based test runners (.wb web, .et editor, .ci config) reported only file-level PASS/FAIL. When a test failed, there was no visibility into which steps passed before the failure or what exactly broke. The web runner's `-v` flag was parsed but entirely unused; the editor's `-v` showed only the first failure; the .ci runner fail-fasted on the first missed assertion with no trace.

## Decisions

- Chose a shared leaf package (`internal/test/trace`) over adding trace methods to `Colors` or each runner, because all three runner packages need the same format and a leaf avoids import cycles.
- Chose dual output (human colored glyphs + machine `VERIFY STEP: {json}`) over human-only or JSON-only, to serve both terminal reading and AI/tooling parsing in one pass, matching the existing `VERIFY FAILURE GROUP` convention.
- Chose two-tier gating (failure trace by default, all-test trace under `-v`) over always-verbose, because failure trace is the high-value path and has no parallelism cost.
- Chose incremental `recStep` closure for .ci over a full collect-all refactor, because step recording during execution is sufficient and the fail-fast verdict is unchanged.
- Chose step ordinal fallback for .et trace over adding a `Line` field to parser structs, because the .et parser structs lack source line info and adding it was out of scope.

## Consequences

- Failed tests across all three families now show exactly which steps passed and where the failure occurred, without any flag.
- `-v` is functional for web and editor: shows per-step trace for passing tests too.
- The `VERIFY STEP` token enables machine parsing of per-assertion outcomes, complementing the macro `VERIFY FAILURE GROUP` layer.
- The trace package is reusable by any future test runner that records `StepResult` slices.

## Gotchas

- The spec was never updated from skeleton despite full implementation landing. Retroactive closure required auditing the code against stale spec claims.
- Spec file paths (`cmd/ze-test/web.go`, `cmd/ze-test/editor.go`) were wrong: actual entry points are `internal/test/cli/cmd_web.go` and `internal/test/cli/cmd_editor.go`.
- The `test-runner-unify` spec in the three-spec sequence was never created as a standalone spec; its scope was absorbed into `test-web-parallel`.

## Files

- `internal/test/trace/trace.go` -- leaf trace package: StepResult, PrintTrace, dual output
- `internal/test/trace/trace_test.go` -- 5 unit tests for trace formatting
- `internal/component/web/testing/runner.go` -- .wb runner records StepResult per action/expect
- `internal/component/cli/testing/runner.go` -- .et runner records StepResult per step type
- `internal/test/runner/runner_exec.go` -- .ci runner records per-assertion steps via recStep closure
- `internal/test/runner/record.go` -- StepTrace field on Record
- `internal/test/runner/report.go` -- emits step trace in failure reports
- `internal/test/cli/cmd_web.go` -- web command: trace on fail + verbose gate
- `internal/test/cli/cmd_editor.go` -- editor command: trace on fail + verbose gate
