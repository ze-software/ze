---
kind: note
level:
stage:
---
The native design-evidence check in `internal/le/hookruntime/writeedit.go`
blocks a spec unless the session has investigated the implementation source it
names within the freshness window. It catches a behavioral claim that was never
traced to producing code.

**The source that counts is the spec's own subject, read from its Files to
Modify and Files to Create lists.** A spec about a Go producer, a YANG model,
or a build configuration is grounded by reading that producer. An unrelated
file grounds nothing.

**Every source kind named by the spec must be read on its own freshness clock.**
`hookSourceRead` in `internal/le/hookruntime/lifecycle.go` records accepted
reads, and the Write/Edit gate consumes the same session markers. The LSP route
grounds Go symbols only.

**A window of under 20 lines does not count as reading the producer.** A whole
file counts whatever its length, because a 12-line file read entire IS the
producer. The gate is strict about WHICH file was read, so it cannot be trivial
about how much of it was shown.

**A Read that showed NOTHING grounds nothing, and the whole-file rule above does
not rescue it.** A failed Read, an empty file, or an unchanged empty payload is
measured as zero. Only a response shape the writer does not recognise is
accepted unmeasured, so an unfamiliar payload cannot disable the evidence path
for a whole session.
Renew a stale marker with `Read(path, offset=N, limit>=20)`: the harness returns
content for a window and `file_unchanged` for a second whole Read of the same file.

**A spec whose subject the gate cannot read is checked against the weaker
any-source bar, and the gate SAYS so.** That is the one permissive path left in
it, and a permissive path that says nothing is the failure this rule names.
Write each file with its path, and the gate asks for the kinds they name. Two
things are deliberately not subjects: a `### ... Checklist` row under the
section, because the section ends at the next heading of any depth, and the
description column of a table, because only the first cell of a row is a path.

It is a backstop, not a guarantee: it cannot verify that the code you read was
the code your claim depends on, and a `Bash` investigation with `grep` or `sed`
is invisible to it. See `ai/rules/repo-maintenance.md`.
