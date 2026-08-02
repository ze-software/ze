# No Poll Loops

**When:** waiting for a command, a build, a QEMU boot, a lab, or another session's run to finish
**Severity:** blocking
**Related:** bash-output, no-fork-loops

## Directives

Never poll for work you launched. A Bash command started with
`run_in_background` re-invokes the session when it exits, so that notification
IS the wait. A loop that watches the same command adds a process and reports
nothing the notification does not already carry.

- **Never write a `while` or `until` loop that calls `sleep`, and never put `pgrep` in a loop condition.**
- **A poll that is genuinely the only available signal MUST die on its own. Wrap it in `timeout <seconds>`.** An unbounded watcher outlives the reason it was started for, because the session that started it has moved on.
- **Stop a watcher the moment its reason changes.** `TaskStop` the background task. "It will end eventually" is how four of them come to tick at once.
- **One watcher at a time, and never faster than one wake every 30 seconds.** Each wake competes with QEMU, Docker and `ze-verify` for the same cores. That contention is what makes the functional suites flaky, so a watcher can corrupt the run it is watching.
- **Foreground `sleep` is blocked by the harness because waiting is not work.** Reaching for a loop to win the sleep back inverts that intent. Do other work, or end the turn.
- **The harness's own examples are unbounded, and this repo overrides them.** The Bash tool text prescribes an `until` loop when a foreground `sleep` is refused, and the `Monitor` schema shows `until grep -q ...; do sleep 0.5; done`. Both are refused here, and one word fixes both: `timeout`. The 30-second floor governs a watcher that spawns a process per wake (`pgrep`, `docker`, `curl`); a local file test inside a bound can be faster.

| Waiting for | Mechanism |
|-------------|-----------|
| A command this session launched in the background | Nothing. The completion notification is the wake-up |
| A file or a log line one of your own commands will produce | ONE bounded loop in `run_in_background`: `timeout 300 bash -c 'until [ -f <path> ]; do sleep 30; done'`. It notifies once, then it is gone |
| A repeated event (every ERROR line, every CI step) | The `Monitor` tool, with `persistent` left false so its `timeout_ms` deadline applies. `persistent: true` disables that deadline and rebuilds the problem this rule exists to stop |
| Another session's `ze-verify` to release the lock | Do other work. `tmp/.ze-verify.lock.owner` names the holder, and `scripts/dev/verify-status.sh check` reports the last run's verdict. Never a watcher |
| Nothing in particular | Do not wait at all |

## Rationale

On 2026-08-02 a session left four `until ! pgrep ...; do sleep 5; done` loops
running on a machine that was also running QEMU, Docker and a 22-stage
`ze-verify`. The loops were started because a foreground `sleep` is refused, and
they were never stopped when the thing they watched changed. The wake-ups were
the contention that made the functional suites flaky for the rest of that
session.

The harm is not the fork cost that `no-fork-loops.md` measures. It is the wake
and its lifetime: a poll loop keeps taking CPU on a loaded box long after
anybody wants its answer.
