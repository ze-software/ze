# Open items from the origin/main divergence replay, 2026-08-31

The replay itself is DONE and its narrative is in `git log`, from `c0d4b3612` to
`fe51839da`. This page carries only what outlives it.

One sentence of context, because several rows below cite it: `origin/main` held 12
commits made under a different git identity on 2026-08-30, fetched here at
`refs/remotes/origin/main@{2026-08-31 00:56:48}` and never integrated, so both
machines walked the same RFCs and wrote different artifacts. The 12 were replayed
by hand, merging each file. Origin's SHAs are not ancestors of HEAD, by
construction.

## Force push readiness

Three files are tracked on `origin/main` and not in HEAD. All three were deleted
DELIBERATELY by local commits, so a force push removing them is correct:

| Path | Deleted by |
|---|---|
| `internal/component/config/cli/cmd_ls.go` | `79d199a31` |
| `test/parse/cli-data-ls-show.ci` | `03d5e261c` |
| `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` | `cc6a79d2c` |

Re-run the gate before pushing, because other sessions share this checkout:

```
comm -23 <(git ls-tree -r --name-only origin/main|sort) <(git ls-files|sort)
```

`git diff HEAD origin/main` still shows 6498 lines origin holds that HEAD does
not. 4180 of them are in files the 12 commits NEVER touched, where origin's copy
is simply the older one and local superseded it after the fork. The remaining
~2300 are in files the 12 did touch; each was read by a merge agent that kept
local as the newer authoring. That set was not re-audited line by line.

## Decisions only Thomas can take

| # | Decision |
|---|---|
| D-1 | `RFC8671-5.2-1`, the rule for marking a pre-policy Adj-RIB-Out view, is declared in `rfc/short/rfc8671.md` with no source site and no tags. Ze exports the post-policy view only, which RFC 7854 Section 5 permits. The gate prescribes retiring the id, which deletes a MUST row from the published ledger. |
| D-2 | Two specs claim one job: `plan/spec-ike-virtual-ip-assignment.md` (new, assignment only) and `plan/spec-ipsec-remote-access.md` (design, phases A-G, AC-7..AC-10 and AC-16..AC-28). One supersedes or folds into the other. Either way the survivor must say that the work reverses the recorded no-IRAC-role decision on the RFC 7296 status row. |
| D-3 | 188 `binds-another-role` sites across 18 RFCs need reclassifying under the 2026-08-31 ruling that the kind is presumed wrong. Concentrated in `rfc4364` (39), `rfc2869` (33), `rfc2865` (32), `rfc4303` (29), `rfc4761` (16), `rfc1035` (11), `rfc3032` (10). This belongs to the `rfcgate-6` spec, not to a replay session. |
| D-4 | Eleven RFC 2865 ids are now tagged twice, once on the admin path in `internal/component/radius` and once on the subscriber path in `internal/component/l2tp/plugins/authradius`. Two producers, two claims. Keep both, or cut one side. |

## Work identified and not done

| # | Item |
|---|---|
| W-1 | The named API-sync barrier from `4f58d46a1` is NOT landed. `plugin.ReactorStartupCoordinator` is embedded in `types_bgp.go`, so the signature change breaks seven packages at once. One change must carry: `reactor/peer.go` (`apiSyncExpected []string` and `apiSyncSignalled` replacing the int32 counter, `resetAPISync([]string)`, `SignalAPIReady(plugin.Sender)`), `reactor/peer_run.go`, `reactor/api_sync.go`, `reactor/reactor_api.go`, `plugins/cmd/peer/session.go`, six `mock_reactor_test.go`, `update_text_test.go`, and `plugin/types.go` plus `plugin/coordinator.go`. It also edits the RFC-tagged `TestInitialSyncClosesTheQueueGateBeforeItWaitsForRoutePushingPlugins`, so it owes an owner row. |
| W-2 | `internal/test/fixture/ui_fixture_cli_announce.go` is UNREACHABLE. It registers `ui/cli-announce-reaches-the-wire` and `ui/cli-announce-tag-round-trip`; no `.ci` names either, here or on origin, and the `peer-script` it reads exists in neither tree. The driver half was never written. |
| W-3 | The two walk tests landed in `fe51839da` owe discrimination records. A run before they landed reported 68 violations of the form "new against git HEAD and carries no discrimination proof" against their tags. |
| W-4 | `TestChildSAInboundPolicyUsesNegotiatedTS` (`internal/component/ike/engine/child_test.go`) over-claims: the tag says an inner packet from outside the negotiated TSr is dropped; the body asserts selector values and sends no packet. Correcting it owes an owner row. |
| W-5 | `RFC2865-5-1` is anchored to section 5 while its only source sentence is section 5.1. `validateID` refuses `(§5.1)` on that id, and renaming to `RFC2865-5.1-1` trips `checkRetiredRequirements`. |
| W-6 | `RFC2865-5-5` carries a positive tag and no negative anywhere. |
| W-7 | `plan/journal/unwired-feature.md` rows 76 and 79 are one defect written twice by one spec a day apart. Deduplicate on a journal pass. |
| W-8 | Eight lint findings sit in origin's own IKE code carried in by `093dfc8e5`: godot on three RFC quotations, `strings.Cut` / `CutPrefix` / `SplitSeq` modernizers, one unparam. |
| W-9 | `internal/le/site/testdata/published-configuration.md` and `published-yang-config-tree.json` are stale for `ze-radius-conf.yang`, both before and after the `profile-attribute` change. |
| W-10 | `TestDraftReadmeNamesEveryCheck` (`internal/test/runner/draft_dir_test.go`) expects the substring `docwiring/checks.go`; `test/draft/README.md` says `internal/le/doc/wiring/checks.go`. Committed drift from the package move: the test string is stale, the README is right. |
| W-11 | Fifteen rows were removed from `test/weakened.md` so an unrelated commit could pass. They are preserved verbatim in "The removed `test/weakened.md` rows" below, because the scratch copy this session recovered them from lives in a cache directory that is cleaned. Their content names AC-3, AC-8, AC-15 and the `help-shape` native verb, so the owner is `yang-short-and-long-command-help`, not `rfc-tag`. That session cannot commit until it restores them, and nobody else should: the gate refuses a row naming a test the prospective commit does not weaken. |

## The removed `test/weakened.md` rows

Fifteen lines as saved, three of which repeat later lines, so twelve rows are
distinct. The owning session restores what it still weakens, and drops the rest.

| Test | Justification |
|---|---|
| TestAdminTreeFromYANG_NilTree | RENAMED to `TestAdminNavNilTree`, same reason: the producer it names no longer exists. The property is unchanged and still asserted: a nil command tree serves an empty console rather than panicking. |
| TestAdminTreeFromYANG_EmptyTree | RENAMED to `TestAdminNavEmptyTree`, same reason. The property is unchanged: a tree with no children yields no nav column. |
| TestAdminTreeFromYANG_DeepNesting | RENAMED to `TestAdminNavDeepNesting`, same reason. The property is unchanged: a path several levels deep resolves to its own node and its own children. |
| TestSummary | DELETED with `helpfmt.Summary`, the first-sentence guess this spec exists to remove (AC-15). There is no producer left to test: `grep -rn 'func Summary(' --include=*.go .` outside vendor returns nothing, in either helpfmt package. What replaces it asserts the opposite property, that nothing is shortened: `TestPageRendersEachDeclaredHelpTextWhole` and `TestPageWithoutHelpPrintsNoBodyBlock` (`internal/core/helpfmt/helpfmt_test.go`), plus `TestHelpEntriesKeepTheWholeSummary` (`internal/component/command/help_test.go`). |
| TestActionTableDeclaresFourNativeVerbs | RENAMED to `TestActionTableDeclaresEveryNativeVerb` because the count was in the name and this spec adds the fifth verb, `help-shape`. The assertion is widened, not weakened: the want set is now every registered native verb rather than a hardcoded four, so adding a sixth without registering it is still red. |
| TestBuildAdminCommandTree | DELETED with the thing it tested. `buildAdminCommandTree` was a static `map[string][]string` of admin nav paths, marked Deprecated since spec-web-2 Phase 6, and its only caller was this test. The admin console now walks the merged YANG command tree, which is what lets the command form show each command's declared summary and long help (AC-8). The nav shape it pinned is asserted over the real tree by `TestAdminNavFromYANGTree`, `TestAdminNavNilTree`, `TestAdminNavEmptyTree` and `TestAdminNavDeepNesting`. |
| TestBuildAdminCommandTree_FromYANG | RENAMED to `TestAdminNavFromYANGTree`. `AdminTreeFromYANG` and `walkAdminTree` are gone: the handler takes the `*command.Node` tree whole instead of a flattened children map, because the map carried no help text and the form needs both halves. The test still builds a YANG tree and still asserts the child names at each depth, now through `adminNodeAt` and `adminChildNames`. All 8 table cases survive as the four named tests. |
| TestAdminTreeFromYANG_NilTree | RENAMED to `TestAdminNavNilTree`, same reason: the producer it names no longer exists. The property is unchanged and still asserted: a nil command tree serves an empty console rather than panicking. |
| TestAdminTreeFromYANG_EmptyTree | RENAMED to `TestAdminNavEmptyTree`, same reason. The property is unchanged: a tree with no children yields no nav column. |
| TestAdminTreeFromYANG_DeepNesting | RENAMED to `TestAdminNavDeepNesting`, same reason. The property is unchanged: a path several levels deep resolves to its own node and its own children. |
| placeholderValueCommands | DELETED, and it is a test HELPER rather than a test. It chose which commands to exercise by matching a value placeholder in the command's DESCRIPTION. This spec takes grammar spellings out of descriptions, so the sample fell to 0 and the guard tested nothing. Replaced by `declaredValueCommands` plus `takesPositionalValue`, which sample `command.Usage` (`internal/component/command/usage.go`), the model producer of the invocation form, and keep a node carrying a `UsageValue` token. Measured 95 commands and 104 verb forms with every feature tag on, against a floor of 90. Forced RED by turning `usage.go` `appendLeafTokens` `UsageValue` into `UsageOption`: the sample fell to 4 and the floor guard fired. |
| TestDescribedValueCommandsAcceptTheirValue | RENAMED to `TestDeclaredValueCommandsAcceptTheirValue` because its sample now reads a DECLARED grammar rather than a described one. Nothing left the suite: it still runs one subtest per verb form and still asserts the command resolves with its value, over 104 forms instead of 25. Forced RED by disabling the trailing split in `extractValues` (`cmd/ze/internal/cmdutil/cmdutil.go`): 104 of 104 subtests failed with the `unknown command` defect the test exists for. |
| TestHelpLeafMultilineDescriptionIndented | DELETED, and this spec deletes its premise twice. It asserted `writeHelpLine`'s two-space indent over a MULTI-LINE `Description`. `writeHelp`, `writeHelpLine` and `writeHelpEntry` had no non-test caller and were removed with the owner's approval on 2026-08-31, and a command description is now one line by AC-3, which `./le docvalid help-shape` refuses to let drift. The indent it pinned survives on the shipped path and is asserted there: `TestPageRendersEachDeclaredHelpTextWhole` (`internal/core/helpfmt/`) and `test/ui/help-parent-node.ci`, which reads the two-space body block out of a real `ze show bgp help`. |
| TestHelpListingUnchangedWithoutUsageProse | RENAMED to `TestHelpListingIsTheDeclaredSummaryByteForByte` and ONE ASSERTION INVERTED, because the property it pinned is what this spec removes. It asserted that a listing row was byte-identical with and without an authored `Usage:` sentence in the description, BECAUSE the listing stopped at the first sentence. That cut is deleted, so the sentence now reaches the listing. The test asserts byte equality against the declared string and asserts that an authored `Usage:` sentence DOES reach the row. That is what makes `./le docvalid usage-contract` load-bearing instead of belt-and-braces: nothing downstream hides such a sentence any more. |
| TestHelpDoesNotListAChoiceGroupAsASubcommand | ASSERTIONS REDUCED 4 -> 1, because two of them drove the deleted `writeHelp` renderer and one was its setup. The property the test is named for is kept and is the surviving assertion: `HelpEntries` lists no `ze:modifier "choice"` child. The second property it carried, that the page still shows the command's OWN description, moved to the shipped path and is asserted where that page is built, in `TestHelpPageCarriesBothDeclaredHelpTexts` (`cmd/ze/command_help_page_test.go`) and in `test/ui/help-parent-node.ci`. A comment in the test names that destination. |

## Permanent reds, both correct, both closed only by product work

`checkCoverageRatchet` reports each as no longer proven. Both were ordered by
Thomas on 2026-08-31 and neither is cleared by an annotation.

- `RFC8671-6.2-1`. `writeStatisticsReport` encodes a Statistics Report correctly
  and clears the O flag; nothing calls it, so none reaches the wire. The missing
  piece is a timer reading `statistics-timeout`, which `behaviorOf` already
  carries into `senderBehavior.statistics`.
- `RFC3948-5.1-1`. `Pool.Allocate` leases uniquely and refuses on exhaustion;
  `registerIKE` discards the pool at `_ = ipPool`, and no engine code builds a
  Configuration payload. `plan/spec-ike-virtual-ip-assignment.md` carries it.

## Owner rulings, 2026-08-31

All five are directives in
`ai/rules/points/rfc-compliance/directives/count-conformance-on-the-whole-stack.md`
(`fc868a86d`, `014a033ef`, `8658100ed`) and in
`plan/journal/green-that-could-not-have-been-red.md`. They are recorded here only
so a reader knows where to look.

1. Conformance counts what the WHOLE STACK produces; who enforces it does not
   matter. A requirement met by configuring the kernel is MET, and still owes a
   test at the boundary Ze owns.
2. A gap is an ISSUE; an exclusion is a DECISION.
3. Conformance is not owed for an OPTIONAL feature outside Ze's scope. The new
   `excluded-kind: feature-out-of-scope` states it; the absent feature is recorded
   as an implementation gap, never a conformance gap.
4. `binds-another-role` is PRESUMED WRONG and owes a justification.
5. The gomu operator set is an accepted limitation. Fix `discriminate` to refuse
   when no candidate falls inside the producing statement; do not extend gomu.

## Two process faults worth not repeating

- `./le commit create ... append` against a script that has ALREADY run re-executes
  its earlier blocks. That produced `aedfaa70f` and `85dedea88`, duplicate
  subjects, and `85dedea88` swallowed regenerated `rfc/requirements/*` shards under
  the wrong message. Use `replace` for every new commit.
- `test/rfc-changed.md` and `test/weakened.md` are rewritten PER COMMIT. Rows for
  an already-committed change must be removed before the next `./le commit create`,
  and in a shared checkout those rows may belong to another session: save them
  before deleting. The gate names an approval row by FUNCTION, not by file stem.
