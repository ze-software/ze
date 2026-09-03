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
| TestEveryFixtureCategoryReachesItsProducer | The criterion was WRONG, not merely narrower. It counted any call outside a string-library allowlist as grounding, and reported five bound categories; every one of those five called a copy of the producer's rule living in hookcheck itself (sourceKind, safeSessionID, journalCells, and two more), so the true count was zero of twenty-five. Grounding now means one thing that cannot be faked: the verdict reaches hookruntime through Probe. The assertion count fell from 4 to 3 because the allowlist walk it replaced is gone. The same file gains TestEveryProbeNamesARegisteredCheck and TestEveryProbeSeparatesItsAllowFromItsRefusal, so the file's assertion count rises, and the second of those was observed RED under an overlay weakening writeCISleep, which the deleted criterion could not see. |
| categoryVerdictCalls | Not a test: a helper that parsed categoryVerdict's switch with go/ast to list the identifiers each case called. It existed only to feed the allowlist criterion above. With grounding decided by membership of categoryProbes, there is nothing for it to read, and keeping it would leave a parser nobody calls. |
| categoryConstantValue | Not a test: the other half of the same parser, resolving a category constant's identifier to its string value so the AST walk could key its map the way fixtureCategories does. It has no caller once categoryVerdictCalls is gone. |
