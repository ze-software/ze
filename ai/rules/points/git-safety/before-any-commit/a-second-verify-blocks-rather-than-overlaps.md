---
kind: directive
level:
stage:
---
**A second `ze-verify` cannot overlap the first: it blocks on the repo-wide lock**
and runs the whole thing again afterwards, so starting one while another is live
does not overlap the work, it doubles the wall clock.
