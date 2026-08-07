---
kind: note
level:
stage:
---
`ze-tracked-build-check` is the one entry whose red is cleared BY a commit
rather than before one. It judges what git already holds, so a broken HEAD is
fixed by committing the producer a previous commit left behind, and every other
gate on the list is fixed in the working tree first. Refusing every commit until
it goes green would therefore deadlock: the refusal would block the only commit
that can lift it. **`--broken-head-fix "<reason>"` is that commit's route
through**, and it is narrow by construction: `commit_helper.py` accepts it only
when tracked-build is the ONLY structural red, so a lint, tier or wiring failure
riding alongside still refuses. Run `make ze-tracked-build-check` after the
script and confirm it went green. If it did not, HEAD is still broken for
everybody who builds it.
