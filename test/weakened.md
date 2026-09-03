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
| TestHelpShapeGateCapsTheSummaryAtItsBound | RENAMED, not removed, and the replacement asserts more. It drove one bound, the 25-word cap, through a table whose fixtures also ran past 96 characters, so once this commit added the character cap those rows would have named two rules and proved neither. TestHelpShapeRefusesASummaryPastTheWordCap is the same test under the name the new rule set needs, with its fixtures shortened so each row breaks exactly the bound it is named for, and it keeps the boundary pair (25 words passes, 26 refuses). TestHelpShapeRefusesASummaryPastTheCharacterCap covers the bound this commit adds, with its own boundary pair at 96 and 97. Nothing left the suite: the word cap is asserted in both polarities before and after. |
| shapeLoader | Not a test and not weakened: a helper whose BODY moved. It built a loader over a command module and an API module with four inline assertions. It now delegates to shapeLoaderOver, which takes a third module because the gate walks the CONFIG tree as well, and that helper carries the same four assertions plus the config one. Every caller of shapeLoader keeps its behaviour, and the assertion count over the pair rose from 4 to 5. |
