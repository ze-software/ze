---
kind: directive
level: MUST NOT
stage:
---
**MUST run `./le verify worktree` in the foreground, wait for it, and never poll: the foreground return IS the completion signal.**
**MUST NOT kill it for being slow, and MUST give the call the largest timeout the harness allows.** A verify that is still running is not a verify that is hung, and killing one costs the whole pass rather than the seconds it saves.
**When the harness caps a foreground call BELOW a full pass, the run MAY be started detached, and only where the harness raises a completion event of its own.** A truncated run rewrites no verify record, so a commit gate reading that record could never be cleared from inside such a harness. Where the harness itself says when the job ended, that event IS the completion signal.
**Waiting on that event is not polling: MUST NOT sleep-and-check, MUST NOT `tail` a log that is still growing, and MUST NOT start a second run to find out where the first one got to.** A second run contends for the same job slot, and a live run is never the one displaced.
**A harness that raises no completion event has no detached route: MUST say the cap is in the way and hand the run to the operator.** Claiming a pass from a run that was killed at a ceiling is worse than not running it.
**MUST NOT edit the tree while the run is detached.** Detached says where the completion signal comes from. It does not license other work in the tree meanwhile, because the run reads the working tree either way.
**MUST NOT take a timeout from a duration written in any rule: read `tmp/.ze-verify-duration.txt` instead.** Two rules quoting different figures are different hardware, not a contradiction, and a slow run is never broken for being slow. What writes that file, and the stall window that actually displaces a holder, are `docs/contributing/running-commands.md`.
