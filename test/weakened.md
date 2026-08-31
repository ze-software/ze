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
| placeholderValueCommands | DELETED, and it is a test HELPER rather than a test. It chose which commands to exercise by matching a value placeholder in the command's DESCRIPTION. This spec takes grammar spellings out of descriptions, so the sample fell to 0 and the guard tested nothing. Replaced by `declaredValueCommands` plus `takesPositionalValue`, which sample `command.Usage` (`internal/component/command/usage.go`), the model producer of the invocation form, and keep a node carrying a `UsageValue` token. Measured 95 commands and 104 verb forms with every feature tag on, against a floor of 90. Forced RED by turning `usage.go` `appendLeafTokens` `UsageValue` into `UsageOption`: the sample fell to 4 and the floor guard fired. |
| TestDescribedValueCommandsAcceptTheirValue | RENAMED to `TestDeclaredValueCommandsAcceptTheirValue` because its sample now reads a DECLARED grammar rather than a described one. Nothing left the suite: it still runs one subtest per verb form and still asserts the command resolves with its value, over 104 forms instead of 25. Forced RED by disabling the trailing split in `extractValues` (`cmd/ze/internal/cmdutil/cmdutil.go`): 104 of 104 subtests failed with the `unknown command` defect the test exists for. |
| TestHelpLeafMultilineDescriptionIndented | DELETED, and this spec deletes its premise twice. It asserted `writeHelpLine`'s two-space indent over a MULTI-LINE `Description`. `writeHelp`, `writeHelpLine` and `writeHelpEntry` had no non-test caller and were removed with the owner's approval on 2026-08-31, and a command description is now one line by AC-3, which `./le docvalid help-shape` refuses to let drift. The indent it pinned survives on the shipped path and is asserted there: `TestPageRendersEachDeclaredHelpTextWhole` (`internal/core/helpfmt/`) and `test/ui/help-parent-node.ci`, which reads the two-space body block out of a real `ze show bgp help`. |
| TestHelpListingUnchangedWithoutUsageProse | RENAMED to `TestHelpListingIsTheDeclaredSummaryByteForByte` and ONE ASSERTION INVERTED, because the property it pinned is what this spec removes. It asserted that a listing row was byte-identical with and without an authored `Usage:` sentence in the description, BECAUSE the listing stopped at the first sentence. That cut is deleted, so the sentence now reaches the listing. The test asserts byte equality against the declared string and asserts that an authored `Usage:` sentence DOES reach the row. That is what makes `./le docvalid usage-contract` load-bearing instead of belt-and-braces: nothing downstream hides such a sentence any more. |
| TestHelpDoesNotListAChoiceGroupAsASubcommand | ASSERTIONS REDUCED 4 -> 1, because two of them drove the deleted `writeHelp` renderer and one was its setup. The property the test is named for is kept and is the surviving assertion: `HelpEntries` lists no `ze:modifier "choice"` child. The second property it carried, that the page still shows the command's OWN description, moved to the shipped path and is asserted where that page is built, in `TestHelpPageCarriesBothDeclaredHelpTexts` (`cmd/ze/command_help_page_test.go`) and in `test/ui/help-parent-node.ci`. A comment in the test names that destination. |
| TestBuildAdminCommandTree | DELETED with the thing it tested. `buildAdminCommandTree` was a static `map[string][]string` of admin nav paths, marked Deprecated since spec-web-2 Phase 6, and its only caller was this test. The admin console now walks the merged YANG command tree, which is what lets the command form show each command's declared summary and long help (AC-8). The nav shape it pinned is asserted over the real tree by `TestAdminNavFromYANGTree`, `TestAdminNavNilTree`, `TestAdminNavEmptyTree` and `TestAdminNavDeepNesting`. |
| TestBuildAdminCommandTree_FromYANG | RENAMED to `TestAdminNavFromYANGTree`. `AdminTreeFromYANG` and `walkAdminTree` are gone: the handler takes the `*command.Node` tree whole instead of a flattened children map, because the map carried no help text and the form needs both halves. The test still builds a YANG tree and still asserts the child names at each depth, now through `adminNodeAt` and `adminChildNames`. All 8 table cases survive as the four named tests. |
| TestAdminTreeFromYANG_NilTree | RENAMED to `TestAdminNavNilTree`, same reason: the producer it names no longer exists. The property is unchanged and still asserted: a nil command tree serves an empty console rather than panicking. |
| TestAdminTreeFromYANG_EmptyTree | RENAMED to `TestAdminNavEmptyTree`, same reason. The property is unchanged: a tree with no children yields no nav column. |
| TestAdminTreeFromYANG_DeepNesting | RENAMED to `TestAdminNavDeepNesting`, same reason. The property is unchanged: a path several levels deep resolves to its own node and its own children. |
| TestSummary | DELETED with `helpfmt.Summary`, the first-sentence guess this spec exists to remove (AC-15). There is no producer left to test: `grep -rn 'func Summary(' --include=*.go .` outside vendor returns nothing, in either helpfmt package. What replaces it asserts the opposite property, that nothing is shortened: `TestPageRendersEachDeclaredHelpTextWhole` and `TestPageWithoutHelpPrintsNoBodyBlock` (`internal/core/helpfmt/helpfmt_test.go`), plus `TestHelpEntriesKeepTheWholeSummary` (`internal/component/command/help_test.go`). |
| TestActionTableDeclaresFourNativeVerbs | RENAMED to `TestActionTableDeclaresEveryNativeVerb` because the count was in the name and this spec adds the fifth verb, `help-shape`. The assertion is widened, not weakened: the want set is now every registered native verb rather than a hardcoded four, so adding a sixth without registering it is still red. |
