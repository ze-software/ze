---
kind: note
level:
stage:
---
Each directive below is one physical line on purpose. `condense_body`
(`scripts/dev/rules_condensed.py`) emits a bold-led LINE raw into
`ai/rules/CORE.md`, so an instruction that wraps arrives there cut in half.

**Run `make ze-verify` in the foreground, wait for it, and never poll: the foreground return IS the completion signal.**
No background run, no sleep-and-check loop, no `tail` on a log that is still
growing.

**Do not kill it for being slow. Give the call the largest timeout your harness allows.**
A verify that is still running is not a verify that is hung, and killing one
costs the whole pass rather than the seconds it saves.

**Never take a timeout from a duration written in a rule: read `tmp/.ze-verify-duration.txt` instead.**
How long a full pass takes depends on the machine, and on what else that machine
is doing. "25 to 30 minutes" below and "4-10 minutes" in `ai/rules/testing.md`
are not a contradiction. They are different hardware. A loaded VM is not
deterministic either, so even one machine gives a spread rather than a figure.
`_record_duration` (`scripts/dev/verify-lock.sh`) appends the real elapsed
seconds for the machine you are on, and `tmp/*` is gitignored, so that file is
the only per-machine record there is. Read it as an expectation, never as a
threshold: a run past it is a slow run, not a failed one
(`plan/learned/1359-rules-corpus-paraphrase-drift.md`).

**A slow run can outlast the lock's own break threshold: raise `ZE_VERIFY_MAX_LOCK_AGE` rather than lose the pass.**
When a second invocation is waiting, `verify-lock.sh` breaks a lock whose holder
has run past `MAX_LOCK_AGE` (default 1800s) and SIGKILLs its process group. Half
an hour is often enough for a full pass and is not guaranteed on a loaded VM, so
that threshold can reach a healthy run rather than a stuck one.

**Never edit the tree while a verify runs, yours or anybody's: it reads the working tree.**
An edit mid-run invalidates the run you are waiting for.
