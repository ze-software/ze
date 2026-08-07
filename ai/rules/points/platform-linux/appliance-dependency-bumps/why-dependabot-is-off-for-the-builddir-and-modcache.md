---
kind: note
level:
stage:
---
The appliance builddir modules (`gokrazy/ze/builddir/`) and the checked-in module
cache (`gokrazy/modcache/`) are **excluded from Dependabot** (`.github/dependabot.yml`)
on purpose: an automated PR would fight the hand-pin (the MVS `max` is chosen
deliberately, and a bot bump reopens the stale-manifest churn described above).
Dependabot stays off; a **proactive review** replaces it: *review*, never an
automated bump.
