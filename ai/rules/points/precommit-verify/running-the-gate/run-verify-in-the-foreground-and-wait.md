---
kind: note
level:
stage:
---
Each directive below is one physical line on purpose. `condense_body`
(`scripts/dev/rules_condensed.py`) emits a bold-led LINE raw into
`ai/rules/CORE.md`, so an instruction that wraps arrives there cut in half.

**Run `make ze-precommit-verify` in the foreground, wait for it, and never poll: the foreground return IS the completion signal.**
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
threshold: a run past it is a slow run, not a failed one.

**A slow run is never broken for being slow, so there is no threshold to raise.**
A waiter breaks a holder's slot only when that holder is DEAD, or when it has
made no progress for the stall window: `_scan_and_claim` (`scripts/dev/ze-run.sh`)
judges progress by the mtime of the job's log, never by elapsed time. A run still
writing stages is a run still working, however long it has taken. `ZE_JOB_STALL_SECONDS`
sets the window and is bounded to 60..3600; a value outside that range is refused
before the job starts, so raising it past an hour is not a route to anything.

**Never edit the tree while a verify runs, yours or anybody's: it reads the working tree.**
An edit mid-run invalidates the run you are waiting for.
