# Quality Standards

**When:** before presenting any work as complete
**Severity:** blocking

## Before You Present Work As Complete

**MUST fix lint issues. MUST NOT disable linters.** Only exclusions: `fieldalignment` (govet), test-file exclusions for `dupl`, `goconst`, `prealloc` and `gosec`.

**The evidence a done-claim owes is the focused check for the behavior you changed, run once, with its output pasted.** "Should work" is not evidence, and a check that stays red MUST be named in the done-claim with the one-line reason it is scaffolding rather than a product defect (`ai/rules/pre-release.md`).
**`./le verify worktree` is the full gate, a release check owed by neither a done-claim nor a push, and it MUST NOT be started while another agent session is live or a development job is running (`ai/META-DECISIONS.md`).** The focused check covers the components your change reaches; a change to reactor concurrency code MUST also run `go test -race -count=20 ./internal/component/bgp/reactor/...`. `ai/rules/precommit-verify.md` carries how to read its red.
