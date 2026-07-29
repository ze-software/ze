# 1291 -- The pipe gate: what counts as a producer, and which shapes it can see

## Context

`ai/rules/bash-output.md` forbids piping an expensive command's output through a
lossy filter, because the truncated stream is what you then judge the run by.
Commit `afb617952` (summary 1287) rescoped the enforcing check from "every
`| tail`" to "a lossy filter after an expensive producer". Two independent
reviewers then took that check apart, and both halves of the rescope turned out
to be wrong in ways the author had tested for and missed.

## Decisions

- **Match the producer in COMMAND POSITION, never anywhere in the statement.**
  The first attempt searched the whole statement, so `git diff
  scripts/dev/hook-fixture-check.py | head -60` counted as running a gate. 69
  files became "expensive" merely by being NAMED, and reading about a check was
  blocked -- the exact false-positive tax the rescope existed to remove. The check
  now tokenizes each pipeline segment, strips env assignments and launchers
  (`python3`, `bash`, `sudo`, `timeout N`, ...), and tests the resulting command
  word.
- **Flatten continuations before splitting statements.** A newline is a statement
  boundary only when it is not a continuation: bash continues a pipeline after a
  trailing `|` or a `\`. Splitting there put the producer in one statement and the
  filter in the next, so neither tripped, and `go test ./... 2>&1 |` + newline +
  `grep -c FAIL` was allowed where the pre-rescope code blocked it.
- **`|&` is a pipeline.** It is bash shorthand for `2>&1 |`; splitting on a bare
  `|` leaves the filter segment starting with `&`, which an anchored filter
  pattern never matches. Split on `\|&?`.
- **Take the rule's open-ended clause literally.** "or any test/verify/build
  command" was implemented as five literals, so the repo's own gates
  (`hook-parity-check.py`, `verify-status.sh`, `stress-repro.py`, everything under
  `scripts/evidence/`) could be truncated freely. A reviewer hit this while
  reviewing, judging a gate by a `tail -25`.

## Consequences

- The producer set is now: `make`, `go test|build|vet|run`, `golangci-lint`,
  `bin/ze*` (with or without `./`), `ze-test`, `pytest`, all of
  `scripts/evidence/`, and role-named scripts (check/verify/test/audit/lint/
  stress/repro) under `scripts/{dev,checks,docvalid,status}`. Cheap neighbours in
  those directories (`spec-session.sh`, `session-scratch.sh`) stay usable.
- `ai/rules/hook-mapping.md` now states the shapes the check deliberately cannot
  see -- quotes, `$( )`, `bash -c "..."` -- rather than implying full coverage.

## Gotchas

- **Rescoping a guard is two changes, and the loosening half is the invisible
  one.** Tightening (new producers) shows up as blocks you notice. Loosening
  (statement splitting) shows up as silence, and silence is not something a test
  suite volunteers: the golden table had 23 Bash cases and not one contained a
  newline, so the whole bypass class was invisible to it.
- **A negative case is what pins a false positive.** Every new golden asserted
  that something IS blocked. Nothing asserted that naming a gate script stays
  allowed, which is precisely what would have caught the over-block. The golden
  now carries `git diff scripts/dev/hook-fixture-check.py | head -60` -> 0.
- **When a rule's text is broader than the code, one of them is lying.** Either
  implement the clause or narrow the rule; leaving "any test/verify/build command"
  in prose while shipping five literals means every reader over-trusts the gate.

## Files

- `.claude/hooks/pretool-bash.py` -- `EXPENSIVE_COMMAND`, `EXPENSIVE_SUBCOMMAND`, `LAUNCHERS`, `_is_expensive`, `check_pipe_tail`
- `.claude/hooks/pretool-writeedit.py` -- `c_generated_files` matches the project root's generated files by path, and resolves a relative `file_path` against the project dir rather than the CWD
- `scripts/dev/hook-parity-check.py` -- golden cases for the bypass shapes, the producer set, and the negative
- `ai/rules/hook-mapping.md` -- the pipe-tail and discovery-index rows
