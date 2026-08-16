---
kind: directive
level: MUST
stage:
---
- **Prefer `make` targets. A bare `go test` omits Ze's feature build tags and produces phantom reds in unrelated packages.**
- **Never pipe a test/build command through `head`/`tail`/`grep`/`awk`/`sed`/`cat` -- run clean, read the log.**
- **Never write a shell for-loop that forks an external command per iteration when a single invocation can process all inputs.**
- **Never poll for work you launched. A Bash command started with `run_in_background` re-invokes the session when it exits, so that notification IS the wait.**
- **A loop that watches the same command adds a process and reports nothing the notification does not already carry.**
- **Never write a `while` or `until` loop that calls `sleep`, and never put `pgrep` in a loop condition.**
- **A poll that is genuinely the only available signal MUST die on its own. Wrap it in `timeout <seconds>`.** An unbounded watcher outlives the reason it was started for, because the session that started it has moved on.
- **Stop a watcher the moment its reason changes.** `TaskStop` the background task. "It will end eventually" is how four of them come to tick at once.
- **One watcher at a time, and never faster than one wake every 30 seconds.** Each wake competes with QEMU, Docker and `ze-precommit-verify` for the same cores. That contention is what makes the functional suites flaky, so a watcher can corrupt the run it is watching.
- **Foreground `sleep` is blocked by the harness because waiting is not work.** Reaching for a loop to win the sleep back inverts that intent. Do other work, or end the turn.
- **The harness's own examples are unbounded, and this repo overrides them.** The Bash tool text prescribes an `until` loop when a foreground `sleep` is refused, and the `Monitor` schema shows `until grep -q ...; do sleep 0.5; done`. Both are refused here, and one word fixes both: `timeout`. The 30-second floor governs a watcher that spawns a process per wake (`pgrep`, `docker`, `curl`); a local file test inside a bound can be faster.
- **Run `make ze-lint-changed` before claiming any Go implementation work is done.**
- **Fix every issue it reports. Do not claim done with lint failures outstanding.**
