---
kind: directive
level: MUST
stage:
---
**Verification is PERIODIC: a commit waits for its focused tests, never for the gate (owner directive, 2026-08-21).** The gate costs 25 to 53 minutes on this hardware, measured on 2026-08-21 at 1486s, 1574s and 3195s (`tmp/.ze-verify-duration.txt`). A session that verifies before every commit therefore batches its work until one run is worth the wait, and that accumulation is the thing this rule exists to stop. Each finished chunk MUST land when it is finished, with its focused tests green, and the worktree gate MUST run over the resulting commits on a cadence.

**The gate therefore gates PUSHING, not committing.** A commit that stays local costs nobody anything and a commit that never happens costs the work, so a stale verify records a verification-debt row and the commit proceeds. `push "<owner authorisation>"` refuses while any row is open, which is where the debt is actually owed: a push is what reaches users. The one thing still refused at commit time is a STRUCTURAL gate red charged to the commit, because that says the tree is broken rather than merely unverified.
