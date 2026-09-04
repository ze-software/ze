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
| xfrmCounter | The gate is reading a REPLACEMENT as a weakening, and the replacement is strictly stronger. `xfrmCounter(spi, bytes)` built a fake `ip -s xfrm state` record from an SPI and a byte count and NO addresses, which is exactly the blindness this spec removes: a dump with no `src <addr> dst <addr>` header cannot say which direction a counter belongs to. It is deleted and `xfrmSA(source, target, spi, bytes)` replaces it, emitting the direction header every real record carries. No assertion left the suite. Seven tests were ADDED over the new helper, and four could not have been written against the old one: `TestParseXFRMCountersKeepsDirection`, `TestParseXFRMCountersRefusesADumpWithNoDirection`, `TestVerifyTunnelTrafficRejectsOneWayTraffic` and `TestVerifyTunnelTrafficRejectsLossyPing`. The last two are the two defects the spec names, one direction passing for two and a discarded ping verdict. |
| TestParseXFRMCountersBySPI | Deleted because the behaviour it pinned is the defect. It asserted that `parseXFRMCounters` returns a map keyed on SPI alone, which is what let ze encrypting one echo request advance both peers' maps and read as two independent observations. `TestParseXFRMCountersKeepsDirection` replaces it over the same producer, asserting the key carries the `src`/`dst` pair the real dump prints, and `TestParseXFRMCountersRefusesADumpWithNoDirection` adds the negative the old test had no way to express. Coverage of `parseXFRMCounters` is higher after this commit than before it, not lower.
