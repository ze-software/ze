---
kind: directive
level: MUST NOT
stage:
---
**A second `ze-precommit-verify` cannot overlap the first: it blocks on the repo-wide lock**
and runs the whole thing again afterwards, so you MUST NOT start one while another
is live: it does not overlap the work, it doubles the wall clock.
