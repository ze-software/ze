# 1324 -- Poll Loop Gate

## Context

A session left four `until ! pgrep -f qemu; do sleep 5; done` loops running on a
machine that was also running QEMU, Docker and a 22-stage `ze-verify`. The wakes
were the contention that made the functional suites flaky for the rest of that
session. A Bash command started with `run_in_background` already re-invokes the
session when it exits, so the poll carried no new information, and the loops
were never stopped when the thing they watched changed. The loop shape was
reached for because the harness refuses a foreground `sleep`.

## Decisions

- Gate on whether a loop CAN END, not on whether the wait is justified. A `timeout` in front of the loop is the escape, chosen over an allowlist of approved wait commands: the bound is the property that matters and it stays one word away, while a judgement about necessity is not mechanically checkable. Credit it PER LOOP, in the statement that loop's keyword opens. The first version searched the whole prefix, so `timeout 10 curl x; until ! pgrep -f qemu; do sleep 5; done` was accepted and the guard failed open.
- Match `while`/`until` paired with a `sleep <n>` COMMAND, or with `pgrep` in the loop CONDITION, over matching `pgrep` anywhere. `pgrep -f x | while read pid` is a one-shot loop that ends by itself.
- Require a digit or an expansion after `sleep`, so `grep -rn 'time.sleep(' test/plugin` over the `.ci` corpus stays usable.
- A new rule file over a section in `commands.md`. That rule is already 160 lines, and its trigger names running a command rather than waiting for one. The trigger is the only part of a rule that routes.
- The gate sees Bash TOOL CALLS, never scripts on disk. `scripts/evidence/*` keeps its internal QEMU waits. Only ad-hoc session watchers are refused.

## Consequences

- A wait that really is the only signal is written `timeout 300 bash -c 'until [ -f <path> ]; do sleep 30; done'` and self-terminates. Repeated events belong to `Monitor`, whose `timeout_ms` deadline applies only while `persistent` is false.
- Quoting a poll loop to test it is rejected too, the same coarseness `commands.md` documents for git verbs. Feed the payload from Python, as `hook-parity-check.py` does.

## Gotchas

- `make ze-rules-condensed` regenerates from the WORKING TREE, so it swept a concurrent session's uncommitted `ai/rules/testing.md` into `CONDENSED.md`. `rule-format.md` prescribes regenerating from HEAD plus your own rule edits in a scratch tree. Both generators derive their root from `Path(__file__).resolve().parents[2]`, so running the scratch COPY of the script IS the mechanism. `rules_condensed.py` also reads `plan/` for the router corpus.
- **The `ze-verify` stage list lives in `scripts/status/verify_run.go` (`stagesForMode`), not in the Makefile.** Grepping the Makefile to ask whether verify runs a target gives the wrong answer. `ze-hook-test` appears there only as a `.PHONY` name and its own rule, yet it is a stage in both verify modes.
- `c_model_phase` blocks a `.py` edit on Opus 5. The `tmp/session/.model-ack-<sid>` escape is the operator's call to make, never the session's.

## Files

- `ai/rules/commands.md` (new), `ai/rules/repo-maintenance.md` (poll-loop row), `ai/INDEX.md` (keyword row), `plan/learned/HOOK-FRICTION.md` (F22)
- `.claude/hooks/pretool-bash.py` (`check_poll_loop`), `scripts/dev/hook-parity-check.py` (14 corpus rows + golden)
