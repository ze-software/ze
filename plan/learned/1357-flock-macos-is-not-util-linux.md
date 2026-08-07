# 1357 - `flock(1)` on a macOS dev machine is not util-linux

**Date:** 2026-08-07
**Scope:** tooling, testing

## What Changed

`demos/terminal/test_render.py` started its rival lock holder with
`flock -F <path> sleep <n>`. That holder is now a Python process that takes
`fcntl.flock` directly (`LOCK_HOLDER_SOURCE`, `DemoLockTest.hold_lock`).

## The Failure

Two tests failed on every run, against code that was correct:

```
flock: invalid option -- F
AssertionError: False is not true : /var/.../demo-run.lock never appeared
```

`-F` is util-linux's `--no-fork`, and it was load-bearing. Without it `flock(1)`
forks. `holder.kill()` then reaches the wrapper while the `sleep` child keeps
the lock.

The holder exited on the usage error, so it created no lock file. `wait_for`
burned its 30-second deadline twice, and the file took 61.7s to report two
failures that said nothing about the product.

## The Mechanism

macOS ships no `flock(1)`. The one on PATH here is Homebrew's, and Homebrew's
`flock` formula is **discoteq/flock 0.4.0**, a BSD-licensed reimplementation,
not util-linux. It implements `-s -x -u -n -w -o -c -E`. It has no `-F`, and its
`--help` is close enough to util-linux's to read as the same program.

`command -v flock` succeeds on both machines. `flock -V` is what separates them:
util-linux prints `flock from util-linux <version>`, this prints `flock 0.4.0`.

## Why The Skip Guard Did Not Save It

Both tests carried `@unittest.skipIf(shutil.which("flock") is None, ...)`. A
presence guard answers "is there a binary called flock", which was never the
question. The question was "does this binary have this flag", and no guard in
the file asked it. This is the same shape as the `gopls` case
(`ai/rules/context-economy.md`): a tool on PATH is not a tool that answers.

## What To Do

| Situation | Do |
|-----------|-----|
| A test needs a process to hold a lock | Take the lock in-process (`fcntl.flock`) and signal readiness with a marker file. One process, no fork semantics to get wrong, and `kill()` means what it says |
| You reach for a `flock(1)` flag beyond `-s -x -u -n -w` | It is util-linux-only. It works in CI and in the container, and it fails on a macOS dev machine |
| A test waits on the lock FILE | That is a race, not a barrier: the file exists from the moment it is opened, which is before the lock is held. Wait on a marker the holder writes AFTER it takes the lock |
| You add a `skipIf(shutil.which(...))` guard | Say what the test needs from that binary. Presence is not capability |

## Files

- `demos/terminal/test_render.py` -- `LOCK_HOLDER_SOURCE`, `DemoLockTest.hold_lock`, and the two contention tests that used the `flock -F` holder

**The reproduction check is what classified this.** A red seen under 30 parallel
sessions reads as load flake. `ai/rules/completion.md` allows recording only a
failure you tried to reproduce and did not. Running the file alone reproduced it
in 61.7s, which made it deterministic and therefore a fix.
