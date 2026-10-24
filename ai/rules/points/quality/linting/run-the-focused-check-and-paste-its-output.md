---
kind: directive
level: MUST
stage:
---
**The evidence a done-claim owes is the focused check for the behavior you changed, run once, with its output pasted.** "Should work" is not evidence, and a check that stays red MUST be named in the done-claim with the one-line reason it is scaffolding rather than a product defect (`ai/rules/pre-release.md`).
**`./le verify worktree` is the full gate, a release check owed by neither a done-claim nor a push, and an agent MUST NOT run it: it gives Thomas the exact command to run himself, on an idle machine (`ai/META-DECISIONS.md`).** The focused check covers the components your change reaches; a change to reactor concurrency code MUST also run `go test -race -count=20 ./internal/component/bgp/reactor/...`. `ai/rules/precommit-verify.md` carries how to read its red.
