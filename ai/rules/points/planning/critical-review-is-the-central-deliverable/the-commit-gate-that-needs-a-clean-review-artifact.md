---
kind: note
level:
stage:
---
`internal/le/commit` refuses a spec-closure commit (one that adds a
`plan/journal/*.md` row naming the spec, or removes a `plan/spec-*.md`) unless `./le spec session review
check` passes: a CLEAN artifact exists, covers every reviewable file in the commit
(the ze-close closure commits all of a spec's code in commit A, so that is
full coverage), and its hashes still match (any edit after the review invalidates
it, forcing a fresh pass). A code-free closure still requires a clean artifact to
exist. Override with `--review-override <reason>` only as an explicit owner
decision (printed in the helper output alongside `--unverified`).
