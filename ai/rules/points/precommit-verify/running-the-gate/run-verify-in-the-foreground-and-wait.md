---
kind: note
level:
stage:
---
Each directive below is one physical line on purpose. `condense_body`
(`internal/le/rules/artifacts.go`) emits a bold-led LINE raw into
`ai/rules/CORE.md`, so an instruction that wraps arrives there cut in half.

**Run `./le verify worktree` in the foreground, wait for it, and never poll: the foreground return IS the completion signal.**
No background run, no sleep-and-check loop, no `tail` on a log that is still
growing.

**Do not kill it for being slow. Give the call the largest timeout your harness allows.**
A verify that is still running is not a verify that is hung, and killing one
costs the whole pass rather than the seconds it saves.

**When your harness caps a foreground call BELOW a full pass, you MAY start the run detached, and only where the harness raises a completion event of its own.**
The two directives above are one requirement, not two: wait for the completion
signal, and never substitute a guess for it. A harness that kills the call at a
fixed ceiling turns "run it in the foreground" into an instruction that cannot be
followed, and a truncated run rewrites no verify record, so a commit gate that
reads that record can never be cleared from inside such a harness. Where the
harness itself says when the job ended, that event IS the completion signal the
first directive names, and starting the run detached costs none of the
properties these directives protect.

**Waiting on that event is not polling. MUST NOT sleep-and-check, MUST NOT `tail` a log that is still growing, and MUST NOT start a second run to find out where the first one got to.**
The ban is on inventing a progress signal, never on the run being detached. A
second run is the worst of these: it contends for the same job slot, and
`_scan_and_claim` (`./le job run`) judges a holder by its log's mtime,
so a live run is never the one displaced.

**A harness that raises no completion event has no detached route: say the cap is in the way and hand the run to the operator.**
Reporting the limit costs one line. A verify nobody watched, whose record nobody
refreshed, is the failure this whole point exists to prevent, and claiming a pass
from a run that was killed at a ceiling is worse than not running it.

**Never edit the tree while the run is detached, and treat the wait as the same blocking wait a foreground call would have been.**
Detached says where the completion signal comes from. It does not license doing
other work in the tree meanwhile, because the run reads the working tree either
way.

**Never take a timeout from a duration written in a rule: read `tmp/.ze-verify-duration.txt` instead.**
How long a full pass takes depends on the machine, and on what else that machine
is doing. "25 to 30 minutes" below and "4-10 minutes" in `ai/rules/testing.md`
are not a contradiction. They are different hardware. A loaded VM is not
deterministic either, so even one machine gives a spread rather than a figure.
`_release` (`./le job run`) appends the real elapsed
seconds for the machine you are on, and `tmp/*` is gitignored, so that file is
the only per-machine record there is. Read it as an expectation, never as a
threshold: a run past it is a slow run, not a failed one.

**A slow run is never broken for being slow, so there is no threshold to raise.**
A waiter breaks a holder's slot only when that holder is DEAD, or when it has
made no progress for the stall window: `_scan_and_claim` (`./le job run`)
judges progress by the mtime of the job's log, never by elapsed time. A run still
writing stages is a run still working, however long it has taken. `ZE_JOB_STALL_SECONDS`
sets the window and is bounded to 60..3600; a value outside that range is refused
before the job starts, so raising it past an hour is not a route to anything.

**Never edit the tree while a verify runs, yours or anybody's: it reads the working tree.**
An edit mid-run invalidates the run you are waiting for.
