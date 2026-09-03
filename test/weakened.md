# Test weakenings this commit accepts

**This file is REPLACED for each commit. It never accumulates.** Delete the rows
of the last commit, write the rows of this one. The commit gate refuses a row
naming a test the prospective commit does not weaken, so a row left behind by an
earlier commit blocks the next author rather than helping anybody. Git history
holds every past entry: `git log -p -- test/weakened.md` shows the rows of any
commit beside the change they justified.

**Several sessions share this checkout, and this is one shared path.** Write your
rows immediately before you run `./le commit create`, then read the file again
between writing them and running the script. Rows written earlier are a window
for another session to replace them, and a session that writes with `cat >`
rather than an edit replaces the file whole. The refusal is the safe outcome. The
unsafe one is silent: your commit lands carrying another session's justification,
and no gate sees it, because the file is present and the row count is plausible.
Say so on the message bus before you take the slot.

A row here is the AUTHOR's own justification. The owner's approval for changing a
test that carries an `RFC requirement:` tag is a different file,
`test/rfc-changed.md`, and a row here does not authorize one there.

`parseLedger` (`internal/le/testweakened/ledger.go`) reads the first
`| Test | Reason |` table it finds and every table row under it, so this prose is
safe above the table. Do not write a second such header anywhere in the file: the
parser refuses two tables rather than guess which one the gate should read.

**A test in a NEW file needs no row in either ledger, and the two gates disagree
about that.** The commit gate reads the file at HEAD (`committedText` in
`internal/le/commit/rfcchange.go`) and skips a path with no HEAD version, so it
computes no change for a new file and REFUSES a row naming a test in one. The
write hook reads the file on disk instead (`Proposed` in
`internal/le/testweakened/proposed.go`, where `taggedCarrier` tests the current
`oldText`), so it DEMANDS a row before it lets you edit a new file that already
carries `RFC requirement:` tags. An author who obeys the hook is then refused by
the gate. Until one of them changes, write the tags in the same edit that creates
the file, and carry no row for it.

| Test | Reason |
|------|--------|
| TestQuestionMarkRevealsAConfigCandidateDescription | REPLACED, and the replacement asserts the behaviour this commit introduces. It pinned the superseded contract: that the ? box shows a config node's DESCRIPTION. That is the defect being removed, because the description is also what the one-line row shows, so the box repeated the row and the long explanation was reachable nowhere. `TestRevealCandidateExplanationUsesLongHelp` asserts the new contract, that the box holds the ze:help, and `test/ui/config-help-both-texts.ci` proves it through the real editor binary, forced red by reverting the routing line and green on restore. |
| TestQuestionMarkOnAConfigCandidateWithNoDescriptionInventsNothing | REPLACED by a strictly stronger test. It proved a node declaring no text leaves the level where it is and says so. `TestRevealCandidateExplanationSaysNothingIsDeclared` keeps that assertion and adds the one this commit needs: the box must NOT fall back to the description, because a fallback rebuilds the defect the commit removes (ai/rules/no-layering.md). The negative half therefore covers more than it did, not less. |
