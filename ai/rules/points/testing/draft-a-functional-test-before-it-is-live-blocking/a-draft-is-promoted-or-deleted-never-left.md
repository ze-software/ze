---
kind: directive
level: MUST
stage:
---
**A draft is not a test yet, so the draft workflow MUST end in exactly two moves:
promote it into `test/<suite>/`, or delete it.** Leaving it in the incubator is
the third move, and it is the one that is refused. A draft proves no obligation,
claims no evidence, and appears in no coverage ledger, so a session that finds
one cannot tell abandoned scaffolding from work in progress.

Because a draft is not a test, the guards that protect tests do not protect
drafts, and deleting one needs no approval. `bashTestDeletion` in
`internal/le/hookruntime/bash.go` exempts a command whose every named test path
is the incubator or sits under it. A test path is one carrying a `test/` segment
or a `_test.go` name, so a Go test counts. `writeWeakening` in
`internal/le/hookruntime/writeedit.go` returns before both its weakening
analysis and its RFC-tag branch for a file there. A command that mixes a draft
with a live test still blocks: the live one is the reason those guards exist.

An `RFC requirement:` tag inside a draft is worth nothing until the file is
live, which is why the tag guard skips it. Promoting the file is what turns the
tag into proof.
