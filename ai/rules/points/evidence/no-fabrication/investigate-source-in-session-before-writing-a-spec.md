---
kind: note
level:
stage:
---
The `design-without-lsp` check in `.claude/hooks/pretool-writeedit.py` blocks
writing a `plan/spec-*.md` or `plan/design-*.md` file unless this session has
investigated implementation source within the last 30 minutes. It catches the
case where a spec is authored for a behavioral claim that was never traced to
the producing code.

**WHICH source counts is the spec's own subject, read off its `## Files to
Modify` and `## Files to Create` lists.** A spec about a Python tool, a shell
hook, a YANG model, or the make wiring is grounded by reading THAT file. A spec
about the daemon still needs a `.go`. Reading an unrelated file of another
language grounds nothing, so the gate refuses it.

**The kind is the file's EXTENSION, anywhere in the tree, so the way past a
block is always to read a file the spec itself names.** `.go`, `.py`, `.sh`,
`.yang`, the `Makefile` and `*.mk` each name one kind. The gate derives what a
spec demands and `.claude/hooks/mark-source-read.sh` records what a Read
supplies, and the two must accept the same set. When they did not, 11 open specs
named `.py` subjects under `test/` and `tools/` and 2 named `.sh` subjects under
`packaging/` that no Read could ever record, and the only sanctioned exit was
reading an unrelated `scripts/*.py`. A gate whose sanctioned exit is reading an
unrelated file manufactures the evidence it exists to demand.

**EVERY kind the list names must be read, and each on its own 30-minute clock.**
A spec naming a reactor `.go` beside an `mk/*.mk` makes claims about both, so
one of them cannot stand for the other. Any-of would put the choice of what
counts as evidence in the author's hands: list a cheap file beside the expensive
one, read the cheap one. A newest-across-kinds clock would do the same thing
over time, renewing a stale Go read every time the `.mk` is opened.

**The LSP tool is gopls, so it is evidence of Go and of nothing else.** An
LSP-only session does not ground a spec about Python, shell, YANG or the build.

**A window of under 20 lines does not count as reading the producer.** A whole
file counts whatever its length, because a 12-line file read entire IS the
producer. `Read(file, limit=1)` is not, and it used to clear every spec of that
kind for the next 30 minutes. The gate is strict about WHICH file was read, so
it cannot be trivial about how much of it was shown.

**A Read that showed NOTHING grounds nothing, and the whole-file rule above does
not rescue it.** An empty file reports one line of one, so it used to read as a
whole-file read and one Read of any zero-byte `.py` cleared every `py` spec in
the session. A repeat Read the harness answers with `file_unchanged` shows the
same nothing while renewing the clock, and a failed Read shows nothing at all.
Each is measured as zero now. Only a response shape the writer does not
recognise is still accepted unmeasured, so an unfamiliar payload cannot disable
the evidence path for a whole session. Renew a stale marker with
`Read(path, offset=N, limit>=20)`: the harness returns content for a window and
`file_unchanged` for a second whole Read of the same file.

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
