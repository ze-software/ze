# 028 - One predicate for every refusal

**Spec:** spec-rfc-test-file-naming, closed 2026-10-04

## What the work built

A Go test file's name and its `RFC requirement:` tags now agree in both
directions. A file whose tags all cite one stem is named `<prefix><topic>_test.go`
for it, and a file named for a stem carries a tag for it or the marker
`// RFC naming: untagged -- <reason>`. `./le rfc check` runs
`checkTestFileNames` (`internal/le/rfc/names.go`) over the whole tree on every
run, and the rule point renders in `ai/rules/testing.md`.

The tree was repaired with a new writer, `./le rfc rename`
(`internal/le/rfc/rename.go`). It moves a file byte for byte, rewrites the
discrimination and audit keys that name it, rewrites the policed citations, and
lists the plain mentions. Because the record hashes are path-independent, the
evidence stays verified with no re-judging, and the baseline of `./le rfc check`
follows a byte-pure rename (`exactRenamesSince`, `coversAt`), so 537 R100
renames billed no new proof and needed no owner trailer.

## Decisions

- Rename and rewrite keys rather than re-stamp: the hashes never depended on
  the path, so a key rewrite is the whole repair.
- The rename edits only files the link sweep polices. The one declaration of
  that set moved into the leaf `internal/le/doc/citation` (`Policed`), which
  the sweep and the rename both read.
- A suffix is judged by `go/build` itself over every `go tool dist list` port,
  never by a copied list of GOOS and GOARCH names.
- Multi-stem files keep any name that claims no stem they never tag (owner).

## The lesson

The rename, its `propose` and a check finding each had to answer "would the
rename take this pair?". Each review round found one more refusal that one of
them answered by its own copy: a naming refusal (round 7), a moved build
suffix (round 8), an edited source (round 9). Each fix routed that one refusal
through a shared function and left the rest behind. The class closed only when
every refusal a single pair can draw lived in one predicate, `pairRefusals`,
returning a typed value every caller renders.

Round 10 then found the prose claiming more than the predicate sees: a
refusal judged on an evidence record (an escaped path, malformed JSON) belongs
to the batch rewrite, not to the pair. The fix was to narrow the claim, since
the batch refuses whole before any write and so fails closed.

When three callers must agree on a verdict, give them one predicate on the
first fix, and state in its comment exactly which refusals it does not see.
