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
| filter_aspath.TestExtractASPathField | DELETED with the function it tested. `extractASPathField` and `extractValueUntilNextAttr` were one of FOUR private copies of the same read of the policy filter text format, and this commit replaces all four with `filtertext.ASPath` (`ai/rules/no-layering.md`: X is deleted, then Y is written). Coverage did not leave the suite, it moved to the one reader: `TestASPath` and `TestASPathFieldMatchesAppendText` (`internal/component/bgp/filtertext/aspath_test.go`) drive the empty, bare-single-ASN, bracketed-multi, AS_SET and confederation shapes, and round-trip the reader against its producer `(*ASPath).AppendText`, which the deleted test never did. |
| match_test | One assertion rather than two in `filter_aspath/match_test.go`, and the one that went was the call into the deleted `extractASPathField`. What the file is FOR, that a named as-path filter matches its regex against the read path, is unchanged. |
| filter_aspath_length.TestExtractASPathField | DELETED for the same reason: the second of the four copies. `countASPathHops` now takes the string `filtertext.ASPath` returns, and `TestCountASPathHops` feeds it exactly those shapes. |
| aspath_length_test | Nine assertions rather than ten in `filter_aspath_length/aspath_length_test.go`. The lost assertion checked that the private reader stripped brackets; `filtertext.ASPath` does that now and `TestASPathFieldMatchesAppendText` asserts it once for all four callers. |
| TestValidatorRegistry_MergeGlobalCompleteFns | RENAMED to `TestValidatorRegistry_MergeGlobalCompletions`, following the method it drives. `(*ValidatorRegistry).MergeGlobalCompleteFns` became `MergeGlobalCompletions` because it now merges TWO global registries, `globalCompleteFns` and the new `globalSuggestions`, so the old name described one of the two things it does. The test body is unchanged and still asserts that a globally registered CompleteFn reaches the registry. Two tests were ADDED beside it in the same file, `TestValidatorRegistry_SuggestionDeclaresACompletionOnlyValidator` and `TestValidatorRegistry_SuggestionDoesNotDisarmAValidator`, which cover the second registry. |
