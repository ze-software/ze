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
| TestParseDelegation | RENAMED with its subject. `parseDelegation` is now `parseRegistryDelegation`, because the package gained a second parser for Ze's own table format and the two needed telling apart. The test is `TestParseRegistryDelegation` in the same file, same cases, same assertions. |
| TestParseDelegationSkipsReserved | The same rename. It is `TestParseRegistryDelegationSkipsReserved` in the same file, unchanged in substance. |
| TestParseDelegationMalformed | The same rename. It is `TestParseRegistryDelegationMalformed` in the same file, unchanged in substance. |
| TestLoadRIRTableUnreachable | RENAMED with its subject. `loadRIRTable` became the exported `FetchDelegationTable`, so that both writers of the shipped table run one recipe. The test is `TestFetchDelegationTableUnreachable` in the same file, asserting the same thing: a registry that cannot be reached stops the run. |
| TestSeedRIRTable | Its subject was DELETED and its assertions were kept. `newSeedRIRTable` read a compiled Go literal, and the seed is now an embedded data file parsed by `seedTable` (`internal/component/resolve/irr/seed.go`). All three assertions (a non-empty table, AS3333 at RIPE, AS7018 at ARIN) are in `TestTheEmbeddedSeedParses` in the same file, which adds a floor on the range count beside them. |
| TestInternRIREntry | Its subject was DELETED as duplication. `internRIREntry` mapped an already-parsed RIR name back onto the interned constant; the data file carries the registry TOKEN alone, so `parseRIRRange` reads both constants out of `rirNames` and `rirWhois` directly and there is nothing left to re-intern. What it asserted, that a known registry yields the interned name and whois host, is asserted by `TestARegistryTokenBecomesTheInternedConstants` in the same file. |
| TestInternRIREntryUnknown | The same deletion, negative half. `TestARegistryTokenBecomesTheInternedConstants` carries the unknown-token case, and `TestParseRegistryDelegationMalformed` covers the parse refusing it. Coverage went UP: an unknown token in Ze's own table is now an ERROR that names the line, where `internRIREntry` only answered false. |
| TestOnlyAllocatedAndAssignedASNRecordsAreTaken | The function it covers MOVED out of this package. `internal/le/ianaasn` held a second copy of the registry-file parser, and this spec deleted it so one format has one owner. The property is asserted against the surviving implementation by `TestParseRegistryDelegation` and `TestParseRegistryDelegationSkipsReserved` in `internal/component/resolve/irr`, in the same commit. Coverage went UP: that parser also carries the uint32 overflow guard this package's copy lacked, proven by `TestAnOversizedRecordReachesNoTable`, which is NEW in this file. |
| TestCollapseJoinsOneRegistryAndNeverTwo | The same move. `collapse` left this package for `collapseRanges` in `internal/component/resolve/irr`, where the five `TestCollapseRanges*` tests assert one registry joining, two never joining, the empty case, the single case, the overlap case, and unsorted input being an error. The last of those is a property this package's copy never had. |
| TestGeneratedTableIsByteExact | The ARTIFACT changed, and both halves of the assertion survive. The generator wrote Go source (`var seedRIRTable = []RIREntry{...}`) and now writes a data file, so a byte-exact expectation over Go source asserts a format that no longer exists. The renderer's bytes are pinned by `TestTheRenderedTableIsByteExact` in `internal/component/resolve/irr`, and that THIS generator writes exactly those bytes is pinned by `TestWriteEmitsAFileTheIRRParserReads`, which is NEW in this file and compares the written file against `irr.RenderDelegationTable` for the ranges its five fixtures declared. |
