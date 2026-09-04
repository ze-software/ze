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
| TestExtractUint32Attr | Not weakened: RENAMED, and the gate cannot tell a rename from a deletion. `extractUint32Attr` became `readUint32Attr` in this commit, returning a three-state `attrReading` so a route carrying 0 is told apart from a route carrying nothing, and the test that drives it became `TestReadUint32AttrReportsAbsenceDistinctly` in the same file. Every one of the old test's five cases survives in the new table with the same input and the same expected value: `present` 100, `absent` 0, `at_end` 50, the zero value, and the uint32 maximum. The absent and zero rows also gained the reading each must return, `attrAbsent` and `attrPresent`, which is the whole point of the rename: the old signature answered 0 for both and `buildDynamicDelta` could not tell them apart. Two cases were ADDED, a value wider than uint32 answering `attrUnreadable`, and the named zero row. This test carries no `RFC requirement:` tag; the tagged units in the file are `TestParseModifyDefsMEDRemove` and `TestHandleFilterUpdateMEDRemoveIsImportOnly`, and neither one's text changed. |
