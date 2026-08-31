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
| TestIPCPUnknownOption | RENAMED and WIDENED, not removed: it is `TestIPCPScanNCPOptions` in the same file. It could not keep its old name because its subject is deleted. It drove `ipcpHasUnknownOption`, a bool helper that answered `false` both for "no unknown option" and for "this option list is malformed and I could not tell", which is the defect this commit fixes. `scanNCPOptions` replaces it with a four-state answer, so the old two-assertion test cannot express what the new function returns. Coverage went UP: 2 cases became 8, both originals kept verbatim, and the six added are the ones the bool could never distinguish. |
| ipcp_test | Assertion count fell 16 to 15 for the same reason: the two `t.Error` calls of `TestIPCPUnknownOption` became one `t.Errorf` inside a table loop running 8 subtests. The detector counts assertion SITES, and a table-driven test concentrates many cases behind one site. Cases went 2 to 8, the file gains no assertions elsewhere, and both behaviors the old sites covered are still asserted as named subtests. |
| TestNegotiatePeerMagicZeroRejected | RENAMED to `TestNegotiatePeerMagicZeroNaked`, following the behavior Thomas ruled on. It asserted that a peer Magic-Number of zero draws a Configure-Reject. RFC 1661 Section 6.4 forbids that Reject to any implementation that transmits a Magic-Number, which ze always does. The old assertion pinned the non-conformant answer and could not be kept. The replacement asserts strictly more: no reject at all, exactly one Nak, a 4-octet Magic-Number option, and a value that is neither zero nor ze's own. |
