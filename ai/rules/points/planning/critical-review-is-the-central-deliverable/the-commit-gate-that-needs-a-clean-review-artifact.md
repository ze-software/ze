---
kind: note
level:
stage:
---
`scripts/dev/commit_helper.py` refuses a spec-closure commit (one that adds a
`plan/learned/NNN-*.md` or removes a `plan/spec-*.md`) unless `review_gate.py
check` passes: a CLEAN artifact exists, covers every reviewable file in the commit
(the ze-close closure commits all of a spec's code in commit A, so that is
full coverage), and its hashes still match (any edit after the review invalidates
it, forcing a fresh pass). A code-free closure still requires a clean artifact to
exist. Override with `--review-override <reason>` only as an explicit owner
decision (printed in the helper output alongside `--unverified`).
