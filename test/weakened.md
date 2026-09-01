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
| TestCheckSupportedSignoffRefusesSupportedRowWithNoSummary | DELETED with the branch it covered, which is now unrepresentable rather than unchecked. It asserted that a `Supported` row on `docs/features/rfc-status.md` naming an RFC with no `rfc/short/` summary is refused, and the page's own comment recorded ten such rows. A row IS a summary's `Support` Meta declaration since this commit, so a row whose RFC has no summary cannot be written at all: nothing renders it. Keeping the test would pin a refusal against a state no author can reach, and it would go green forever without proving anything. The other five `checkSupportedSignoff` tests are untouched, including the one that holds the intended end state and the one that keeps `Partial`, `Experimental`, `Unsupported` and `Future` outside the population. The row-presence obligation it used to backstop is not lost either: it moved into the parser, where `readSupport` refuses a summary that declares no `Support` row at all. |
| TestEnrolledRowsTakeTheFirstWordAndSkipComments | DELETED with its producer. It exercised `parseEnrolled`, the reader of `rfc/enrolled.txt`, over comments, blank lines and a trailing-word row. That file is GENERATED from the summaries by this commit and nothing parses it any more, so the function is gone and the test had nothing left to call. What it really protected -- that a stem is read exactly and a decoration around it does not change the set -- is now a property of the Meta table rather than of a whitespace format, and `TestEnrolmentIsReadFromTheSummaryMetaTable` holds it at the new producer, with four refusal cases the old test had no equivalent for. |
| TestEnrolmentKeepsItsReason | DELETED with the same producer, `parseEnrolled`. It asserted the enrolment reason survives the parse rather than being dropped after the stem, which was a real defect when the reader kept only the first field. The reason is now its own declared field, the `Enrolment reason` Meta row, and losing it is not a parse bug that could recur but a missing row the parser REFUSES: `readEnrolment` reds any summary whose kind carries no reason. `TestEnrolmentIsReadFromTheSummaryMetaTable` asserts the reason comes back whole and that its absence is refused, so the property is held more strictly than before. |
