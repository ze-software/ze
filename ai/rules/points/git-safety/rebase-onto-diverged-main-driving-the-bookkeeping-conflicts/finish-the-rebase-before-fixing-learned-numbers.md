---
kind: note
level:
stage:
---
Finish the rebase first, then fix numbering -- never mid-rebase. Afterwards run
`make ze-learned-numbers-fix` (renumbers colliding summaries and rewrites their
references) and recompute any derived ratchet the rebase loosened (e.g.
`test/.ci-sleep-baseline` = actual `time.sleep(` count in `test/**/*.ci`).
