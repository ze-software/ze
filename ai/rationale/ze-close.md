# Rationale: the close skill

`ai/skills/ze-close.md` holds the steps a closing agent runs. This page holds
the reasons and the history behind some of them. Each section is named by the
step that needs it, so the reason is one read away and the step does not carry
it. Why closure is a skill of its own is `ai/rationale/ze-implement.md`,
"Closure in its own skill".

## At most three closures

Closure is the most expensive phase in this repository, and it ends in a
commit.

## No subagent in a closure

A closure that fans out multiplies token spend without adding a lens the same
agent cannot apply by reading. Reading the diff from source is what makes the
review independent; spawning another reader adds a hop, not a lens.

## Keeping the claim until the script has run

`./le commit create` writes `Spec-work: <stem>` from the session's claim, and
the push discharge of a `Review-owed:` finds commit A only through that line
(`specWorkPopulation`, `internal/le/commit/review.go`). A claim released before
the commit lines run leaves commits A and B untagged, so the discharge judges
fewer files than the closure gate did.

## Go headers are the citer most often missed

A `// Design:` header of a tracked Go file is the citer kind the citation step
has missed most often, and `plan/journal/closure-deletes-a-cited-document.md`
counts the recurrences.

## Why a restatement writes the bare stem

`speccitation.Scan` matches the full path only, so the bare stem keeps the name
a reader needs and drops a resolution promise the tree can no longer keep.

An untracked sibling is the case that surprises: `Scan` in
`internal/le/spec/citation/speccitation.go` judges a cited spec through
`regularFile`, a stat of the working tree, which sees an untracked file, so the
working tree reads green while the tree the commit produces is red.

## Sweeping every journal row

An uncommitted row lives in one working tree and dies at the next clean or
checkout, so a closure that leaves its rows behind loses the knowledge the
closure exists to preserve.

## One script, two commits

Closure must not depend on a later reminder to remove the spec. Two commits,
because commit B's removal deletes the spec's working copy, and it deletes only
a copy whose content a commit holds. Commit A preserves the edited spec in git
history; commit B removes it from git and from disk.
