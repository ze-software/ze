# Feature maturity

Every public statement of how mature a Ze feature is comes from one declaration
per feature, `features/<id>.md`. `./le feature check` refuses a declaration
whose level its evidence does not support, and `./le feature report` says, for
every feature, how far it is from the next level. The live vocabulary and every
feature's state are what `./le feature report` prints; this page explains the
model and does not copy them.

<!-- source: internal/le/feature/vocabulary.go -- Kind, Scope, Level, StatusLabel -->
<!-- source: internal/le/feature/declaration.go -- Parse, Load -->
<!-- source: internal/le/feature/check.go -- Check -->
<!-- source: internal/le/feature/runrecord.go -- RunRecord -->
<!-- source: internal/le/feature/scenariotree.go -- scenarioTreeID -->

## The declaration

A declaration is a Markdown file with a `## Meta` table of `| Field | Value |`
rows and a `## Description` section holding the public row prose. The file stem
is the feature id. Adding a feature is adding one file: the directory listing
is the registration, and no Go changes. A list-valued cell separates its items
with a comma and a space. A Go test item is `path/to/x_test.go::TestName`, the
spelling a discrimination record uses.

A feature has two separate facts:

| Axis | Question |
|------|----------|
| Scope | How much of the stated feature exists: complete, partial, future, rejected |
| Level | How well the implemented scope is proven: supported, experimental, stub-backed |

A partial scope lists its Scope gaps, and its public status carries
"(partial)". A future or rejected feature has no level.

Kind decides what each criterion means: a protocol needs interop against a
third-party implementation, a daemon surface needs a test that drives the
user's command, an umbrella has no evidence of its own and is bounded by the
worst of its Parts.

## The ceiling

The check computes the highest level the evidence supports and refuses a
declared level above it. A declared level below it is reported as a promotion
candidate, never refused: maturity is a release decision bounded above by
evidence.

Supported requires, among the criteria the report names:

| Criterion | Evidence the check reads |
|-----------|--------------------------|
| S1 real path | at least one real-path test, and a recorded green run of each one's present content |
| S2 interop | for a protocol feature, at least one Interop scenario not listed in Stub evidence, and a recorded green run of each such scenario's present directory |
| S3 RFC | each listed enrolled stem published as supported by the RFC ledger, with no gated requirement marked `{gap}`; an unenrolled stem is reported as a bound |
| S4 docs | a Docs page and a Doc review |
| S5 defects | no `plan/immediate/` spec naming a Components path in its Files to Modify, and a Defect review no older than the newest journal row naming one ("re-review owed") |
| S6 not stub-only | at least one real-path or interop item not listed in Stub evidence |
| Extra criteria | every extra criterion gating Supported resolves |

A real-path test file counts only when a runner of the repository runs it, and
each runner's own declaration answers, never a list kept by the check
(`runnerOf` in `internal/le/feature/runner.go`). A `.ci` is
`test/<dir>/<name>.ci` and is accepted when a functional suite is named `<dir>`
(`testfunctional.SuiteNamed`), when `le test bgp <dir>` walks it
(`cli.BgpRunnerDir`, which is how `test/chaos-web/` runs), or when a harness
package registers a `le test <dir>` command (how `test/pppoe/` runs). An `.et`
sits anywhere under the directory the editor runner walks
(`cli.EditorSuiteDir`, `cli.EditorTestSuffix`). A file none of them walks runs
nowhere, so it is refused even though it exists. A Docs or Page path that a
registered derived artifact produces (`derived.All`) resolves even when the
checkout has not rendered it yet. An Interop entry is
`<suite>/<scenario>` and resolves through the suite's catalog, which each lab
registers from the same `interoplab.Discover` call its runner makes
(`interoplab.RegisterCatalog`); an unknown suite or scenario is refused at every
level. The RSVP-TE and flow-export interop tests are Go integration tests in
their plugin packages, not Discover scenarios, so no catalog lists them yet.

An `Extra criteria` cell holds items separated by `; `, each
`<level>: <criterion> = <evidence>`. The evidence is a pointer, resolved as an
Interop entry, a real-path test item or a repository path, or a dated
attestation. A pointer that does not resolve leaves the gated level and every
level above it unmet, and so does an Interop entry pointer with no current
recorded green run.

A path that does not exist, escapes the repository, or names a Go test function
its file does not declare is refused at every level.

## Recorded runs

"The test exists" never reaches Supported, and neither does "the scenario
exists". A green run is recorded in `features/runs/<id>.json`, committed beside
the declarations: under `runs`, each real-path test with the git blob id of its
file when it passed; under `interop`, each Interop entry that counts toward a
level (every one not listed in Stub evidence, and every extra criterion pointer
naming one) with the git tree id of its scenario directory when it passed. A run
is current while that id is unchanged; editing the test or anything in the
scenario directory makes the run stale. Only passes are recorded.

Both ids are computed over the working tree, so a run recorded before a commit
stays current through it, and a shallow CI checkout needs no history. The tree
id covers the files git would track under the scenario directory, tracked or
untracked but never ignored, each hashed as it is on disk; on a clean checkout
it equals `git rev-parse HEAD:<dir>`. A record written before interop runs were
recorded has no `interop` key and reads as no scenario run.

`./le feature record-run feature <id>` is the writer. It runs each real-path
item through the repository's own runner, `go test -run ^Name$ -v` for a Go test
and the runner `runnerOf` answers for a `.ci` or `.et`, over an isolated binary
set, and writes the record only when every item was observed passing. The exit
code alone is not the observation: the item's own PASS line must be in the
output, ending with the selector that runner was given (the stem for a `.ci`,
the repository path for an `.et`), because a `-run` pattern that matched
nothing exits 0. A skip is never a pass. A `.ci` that declares
`option=needs-linux`, with or without `caps=`, and that this host would skip
(`runner.NeedsGuest`, the parser's own skip decision) runs in a throwaway QEMU
guest instead: record-run cross-builds `ze` and `ze-stripped` for the guest
under `tmp/qemu/feature-record-run/`, then runs `le test qemu run ... command
'<guest le> test qemu all-tests test <path>'`, which runs that one test through
the suite `all-tests` gives its directory, network-namespace preparation
included. The guest runner's own PASS line is still the observation. When the
guest route cannot run here (no QEMU, no KVM access, no image), the refusal
says the test skips on this host and quotes the route's own error. A
`test/pppoe/` test declares `option=netns-link` and skips outside `./le test
qemu pppoe-test`, which record-run does not drive, so it observes a skip and
records nothing. The chaos-web suite runs under the isolated set like any other:
its `le chaos run` step is the set's own `le`. A failure, a missing PASS line,
or a test file that changed during the run writes nothing.

It then runs each counted Interop entry through its suite's own runner: the
catalog's `RunScenario` (`interoplab.Catalog`), which each lab registers beside
its scenario list and which calls the same `RunAt` its `./le test integration`
or `./le test deployment` action calls, with the scenario name as the selector.
Docker is required. The observation is the suite report: it must report
exactly one scenario, under that name, passed, with no setup error and exit
code 0. A failed scenario, a report of another scenario or of none, a run that
outlasts its two-hour deadline, or a scenario directory that changed during the
run writes nothing.

## Attestations

What no check can prove, that the page and the row prose match the producing
code, rests on a dated attestation: `Doc review` and `Defect review` each read
`YYYY-MM-DD: what was judged`. A date with nothing judged is refused.

A Doc review older than the newest change to a listed Docs page or to the
declaration file is refused at every level, Experimental included: a sentence
known to be unchecked is never published, so the commit waits for the prose fix
or a fresh review. A change date is the committer date of the newest commit
touching the path, or today for a path with uncommitted changes. A shallow
checkout has no history to read it from, and the check refuses to answer there.

## The published surfaces

Two public surfaces state feature maturity, and both derive it from the
declarations. Neither one carries a status a person typed.

`docs/features.md` is a derived artifact, not a tracked file. `RenderPage`
writes it from the declarations: one table for each Kind, in the order the
vocabulary declares the Kinds, and the rows of each table ordered by name. A row
holds the linked name, the status label for its scope and level, and the
Description verbatim. A partial row adds its Scope gaps, and an umbrella row
names each part with that part's status. A part keeps its own row too. The page
renders at session start, after an edit to a file directly under `features/`
(a run record does not feed it), and in every site build through
`derived.EnsureAll`. A declaration the parser refuses stops the render, so a
feature never drops off the page in silence. The site publishes the page at
`/reference/feature-status/`. A link to it from a tracked file goes to that
URL, or names the path in code, because GitHub holds no copy of the file.

<!-- source: internal/le/feature/render.go -- RenderPage, feedsPage, rebuildPage -->
<!-- source: internal/le/feature/register.go -- derived.Register -->
<!-- source: internal/le/site/docsmanifest.go -- docsDestinationExact -->

A site feature card in `website/data/features.json` names, in `features`, the
ids of the declarations it describes, and it holds no status. The card is solid
and sits in the core section only when every feature it names is complete in
scope and supported in level. Otherwise it is dashed, sits in the experimental
section and carries the Experimental badge. The build refuses a card that names
no feature, names an id no declaration holds, names only future or rejected
features, or still carries a `status` key. The published feature count is the
number of cards the build keeps, so a level change moves a card between the
sections but does not change the count. A section with no card is not
published. The features page, the homepage, the site facts and `llms.txt` all
read the cards through `loadFeatureData`.

<!-- source: internal/le/feature/render.go -- CardShipped, CardLabel, CardClass -->
<!-- source: internal/le/site/datapages.go -- loadFeatureData, featureData.place -->
<!-- source: internal/le/site/facts.go -- factsFromSiteData -->

To add a feature, add its declaration: the page follows. To show it on the
site, name its id in the card that describes it, or add a card. To change what
a card's bullets claim, change the declarations it names first, because a card
states nothing its declarations do not.
