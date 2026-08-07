---
kind: note
level:
stage:
---
On explicit commit requests, commit-helper invocation is the work. Do not run
late completeness checks, health checks, recent-commit style reviews, or
remaining-work tables unless the user explicitly asks for them. Before any
verify target, run `scripts/dev/verify-status.sh check`; a FRESH result
forbids rerunning `make ze-verify` or `make ze-verify-changed`.
