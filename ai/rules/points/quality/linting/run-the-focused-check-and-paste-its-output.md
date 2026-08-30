---
kind: directive
level: MUST
stage:
---
**The evidence a done-claim owes is the focused check for the behavior you changed, run once, with its OUTPUT PASTED.** "Should work" is not evidence, and a check that stays red MUST be named in the done-claim with the one-line reason it is scaffolding rather than a product defect (`ai/rules/pre-release.md`).
**`./le verify worktree` is the FULL gate, owed before a push rather than before a done-claim, and it MUST be run in the foreground and waited for.** It already race-instruments the component groups your changed `.go` files reach; a change to reactor concurrency code MUST also run `go test -race -count=20 ./internal/component/bgp/reactor/...`. `ai/rules/precommit-verify.md` carries how to read its red.
