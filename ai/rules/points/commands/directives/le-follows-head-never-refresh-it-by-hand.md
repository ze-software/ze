---
kind: directive
level: MUST NOT
stage:
---
**le MUST NOT be refreshed by hand: `./le` follows HEAD on its own, and `--update` is gone.** When HEAD's inputs (`internal cmd pkg api vendor go.mod go.sum feature-gates.txt feature-tags`) differ from the newest build's, the first call starts one background build of a `git archive` export of HEAD and keeps running the newest build; the build is published as `bin/builds/<UTC commit time>-<short sha>/` only after it passes its smoke test, so a peer's half-written edit never reaches it. A HEAD that fails to build is reported once on stderr, with its log at `bin/builds/.failed-<commit>.log`, and is not retried until a commit changes the inputs again.
