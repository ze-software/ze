# GitHub Pages and Presentations

Website sources live under `website/` on the main branch.
All `../gh-pages` content MUST be generated from this repository.
`./le site build` writes the publishable artifact to `../gh-pages` and removes
old source-only files there. It reuses matching demo artifacts. Run
`./le site terminal-demo render-all` first to force new demo artifacts.
`./le site terminal-demo render name <demo-id>` re-records one demo while you work on
its tape, and publishes it beside the artifacts it did not record. The ids are in
`demos/terminal/manifest.json`.

A recording lives in the ARTIFACT tree, at `../gh-pages/assets/demos`. A render
writes it there, and a build reads it from there for the `?v=` digest it stamps
into every page that replays a demo. So one directory answers what the site
serves and what the pages name. A build that rewrites the artifact keeps the
media, because it lays the last published artifact down before any page is
rendered. `TERMINAL_DEMO_OUTPUT` moves a render to another root.
<!-- source: internal/le/site/demo.go -- (*demoCatalog).assetRoot -->
<!-- source: internal/le/site/terminaldemo/actions.go -- renderEngine -->

A recording runs in a container this repository builds and publishes to no
registry, so build it once per checkout with `./le site terminal-demo image-build`.
It reads the image tag from `demos/terminal/manifest.json`, which is the tag the
recorder runs. A render refuses to start when that image is absent and names
this action. Rebuild after any change to `demos/terminal/Dockerfile`.

Before that, a render, like the renderer's `validate` mode, checks the Docker
daemon's kernel with the demo `ze`, and
refuses a kernel that lacks any feature Ze enrolls, naming each one: a demo
recorded there would show a host fact as product behavior. The check is the
one every Docker run that runs Ze makes, described in
`docs/architecture/testing/interop.md`, "The Docker host kernel check".
<!-- source: internal/le/site/terminaldemo/render.go -- Engine.preflightContainers, Engine.checkKernel -->

`./le site terminal-demo binaries-build-ze` cross-builds the programs the
container runs into `tmp/terminal-demos/bin`: `ze`, `ze-demo`, and a linux
`le`. The `le` build takes the recipe the perf sender container uses: the
`ze_le` tag plus every daemon feature tag, `GOOS=linux`, `CGO_ENABLED=0`, and
the renderer's architecture. The file MUST be named `le`, because `cmd/ze`
selects its personality from its file name. The container records a tape with
`le site terminal-demo pty --tape <file>`. That action hands its words to the
PTY recorder, so `./le site terminal-demo pty --help` prints the recorder's own
options.
<!-- source: internal/le/linuxle/linuxle.go -- Argv -->
<!-- source: internal/le/site/terminaldemo/actions.go -- recorderBuildCommand -->
<!-- source: internal/le/site/terminaldemo/entrypoint.go -- recorderCommand -->
<!-- source: internal/le/site/terminaldemo/pty.go -- RunPTY -->

Each demo's lab and its output check are one scenario, registered under the
id that the manifest's `validate` field names. A tape starts, drives and stops
its lab with `ze-demo run <id> <action>`, and `ze-demo validate <id>` checks
the output with that scenario's validator. A new demo registers its scenario from
an `init()` in a `register_*.go` file in `internal/le/site/terminaldemo/` and
edits no central switch or map. The validator is required, so no recording
ships without a check of what it shows. A demo whose tape starts no lab
registers a validator and no runner. A tape that names an unregistered id, or a
manifest demo with no scenario, fails the package's unit test.
<!-- source: internal/le/site/terminaldemo/registry.go -- scenarioRegistry -->
<!-- source: internal/le/site/terminaldemo/register_scenarios.go -- init -->
<!-- source: internal/le/site/terminaldemo/registry_test.go -- TestEveryManifestDemoHasARegisteredScenario -->

A tape can `Source` a fragment that lives outside its demo directory, such as a
topic's `demos/terminal/topics/<topic>/configure.tape`, which both the topic
recording and the showcase play. A recording's source and definition digests
cover every tape it sources, nested, and every file beside a fragment outside
its own directory, such as the config snippets the fragment loads. Editing a
fragment therefore marks every recording that plays it stale.
<!-- source: internal/le/site/terminaldemo/manifest.go -- addSourceClosure -->

`./le site build output <directory>` builds into another artifact root, and
`./le site check output <directory>` judges that same root. Both default to
`../gh-pages`, so a session verifying its own work builds into its scratch
directory and checks there rather than writing over the published tree.

Before it builds, `./le site build` warns on stderr, and lists under
`stale-supported` in its report, every feature held at Supported whose recorded
test or interop run is stale. The level stands and the page is unchanged; the
warning says the evidence wants re-recording. `./le site build refresh`
re-records those features through `./le feature record-run` first, which runs
real tests and can need Docker or a QEMU guest, so it is never the default and
never a prompt. The rule is `feature-maturity.md`, "Recorded runs".
<!-- source: internal/le/site/freshness.go -- freshness -->

See `website/AI.md` for the full reference: structure, tools, and how to add a talk.

## Rendering tests

Editorial sources own the text and publication dates. An older Pages artifact
does not override an intentional source edit. Blog, weekly-update and homepage
tests render controlled authored inputs instead of freezing the public site's
wording, newest date or article count. Their assertions cover escaped metadata,
external-link isolation, Markdown content and mirrors, shells and media,
date ordering, draft and undated feed exclusions, and page ownership and cleanup.
Do not refresh a prose snapshot to make a source edit pass.

Check the painted page as well as its DOM text. The shared reveal animation
starts on any viewport intersection: a long command reference cannot expose a
fixed fraction of its total height. DOM text alone does not prove the section
is visible.
<!-- source: website/assets/js/site.js -->

The corpus tests materialize all registered derived artifacts once per process.
This setup requires the checkout's real Git history, including a resolvable
`HEAD` for the roadmap; a source-only mount is not a substitute. A failed
materialization is reported to every consumer, not replaced by a fake revision.

The RFC site tests still read the complete checkout ledger. Read-only corpus
assertions share process-local source collection, proof input and published
ledger snapshots, rather than rescanning the tree for every rendering assertion.
The helpers refuse a different checkout root. Tests that change a ledger or its source tree
use independent fixtures; production builds do not cache the collection. The
implementer disclosure is checked within each implementation-kind section of
the complete generated HTML page and its Markdown mirror.
<!-- source: internal/le/site/rfcdetail_test.go -- publishedLedgerOfThisCheckout -->
<!-- source: internal/le/site/rfccompliance_test.go -- TestThePublishedPageSaysWhoImplementsEachDocument -->

The two full-checkout facts assertions likewise share one result from the real
facts producer, including its live test-health input. They assert both answered
counts and source provenance; controlled mutation fixtures remain independent.

## Release roadmap

`/project/roadmap/` is generated from committed `HEAD` on every full or partial
build. The `roadmap` producer calls the shared spec collector once and renders
its Markdown through the docs renderer. The page, `index.md`, and
`data/release-roadmap.json` describe that same snapshot. The JSON records the
schema version, full revision, and digest of the plan inputs.
<!-- source: internal/le/site/roadmap.go -- renderRoadmap -->

The output is an inventory preview pending owner classification. It includes
both required buckets and the root nice-to-have bucket, including unknown
metadata and parked states. Counts measure release work items and do not prove
readiness or delivery. Pending edits appear after commit and regeneration.
The feature page links this inventory rather than maintaining pending cards.
<!-- source: internal/le/spec/roadmap/roadmap.go -- Collect -->
<!-- source: internal/le/site/datapages.go -- featuresBody, featuresMirror -->

The feature page's cards come from `website/data/features.json`, but no card
states its own maturity. Each card names the `features/*.md` declarations it
describes, and the build derives its section and badge from them. A card that
no declaration backs stops the build. `docs/features.md`, which the docs
producer publishes at `/reference/feature-status/`, is rendered from the same
declarations. The rules are in `docs/contributing/feature-maturity.md`, "The
published surfaces".
<!-- source: internal/le/site/datapages.go -- loadFeatureData -->

The build does not read a local `plan/roadmap.md`. Source links name the selected
commit, and an unreadable or missing `plan/` stops the build. Verify privately
with `./le site build output <session-scratch-directory>`, then inspect the
roadmap with JavaScript disabled and a narrow viewport.
<!-- source: internal/le/site/roadmap.go -- renderRoadmap -->


## Plugin catalog

The website plugin catalog at `../gh-pages/reference/plugins/` is
generated, not hand-authored. Its data source is each plugin's
`registry.Registration`: name, description, config roots, dependencies,
optional dependencies, startup ordering, and the YANG schema it registers.
<!-- source: internal/component/plugin/registry/registry.go -- Registration metadata -->

Two facts the catalog shows are DERIVED rather than declared, so do not look for
a field to fill in. The source directory is the package the plugin's engine
function was compiled in. The YANG file list is every `.yang` file in the
directory that holds the module the registration carries, and beside the
package when it carries none.
<!-- source: internal/le/repo/inventory/plugins.go -- pluginPackageDir, pluginYANGFiles -->

A build reads the plugin registrations, writes
`../gh-pages/data/plugin-registry.json`, and renders the catalog plus one local
detail page per plugin. It reads the daemon's own configuration schema, writes
`../gh-pages/data/yang-config-tree.json`, and renders the configuration
reference from that tree and the same registrations. Do not add a parallel
hand-written plugin list.
<!-- source: internal/le/site/build.go -- refreshNativeSurfaces -->
<!-- source: internal/le/site/plugins.go -- renderPluginCatalog -->
<!-- source: internal/le/site/config.go -- renderConfiguration -->

Startup ordering is published as `start_after` in the site registry JSON.
The catalog and detail pages label it **Start after**, and the target's detail
page labels the reverse relationship **Starts before**. These are order-only
edges when both plugins are selected, not required or optional dependencies;
they never auto-load the target. The Markdown mirrors and generated `llms.txt`
retain this distinction.

A plugin the catalog no longer carries loses its page: a build removes every
detail directory whose plugin is not in the registry it just read.

A config root that names no node of the configuration schema STOPS the build.
The owning plugin's section would otherwise publish as core, with its owner and
its YANG source silently absent.

When adding or changing a plugin, update the registration metadata in the
plugin's `register.go`. If the website needs another fact, add a structured
field to `registry.Registration` first, then render from that data. Regenerate
the site with `./le site build`.

## Command surfaces

Every published command page is generated from one file. `publishCommandCatalog`
asks `internal/le/doc/yangcontract.LiveCommandCatalog` for the answer `ze help command
--json` gives, and writes it to `../gh-pages/data/cli-commands.json`. The
producers below read that file and nothing else.
<!-- source: internal/le/site/build.go -- publishCommandCatalog, liveCommandCatalog -->
<!-- source: internal/le/site/catalog.go -- catalogFile, loadCommandCatalog -->

| Published route | Producer | What it renders |
|-----------------|----------|-----------------|
| `reference/cli/` | `internal/le/site/commands.go` -- `renderCLIReference` | One row per command, plus the pipe-operator table `renderOperatorGuide` writes into the same page |
| `reference/command-equivalents/` | `internal/le/site/equivalents.go` -- `renderCommandEquivalents` | The vendor map index, joined with `website/data/command-equivalents.json` |
| `reference/command-equivalents/<slug>/` | `internal/le/site/equivalentdetail.go` -- `renderEquivalentDetail` | One detail page per mapped command |
| `llms.txt` | `internal/le/site/derived.go` -- `renderLLMS` | One line per command, for a machine reader |

A command carries two help texts, and each surface reads the one it has room
for. `short-help` is the one-line summary, and all four producers print it
whole. `description` is the explanation, and only `renderEquivalentDetail` prints
it, as the detail page body. No producer derives one text from the other, and
none cuts either one. `docs/architecture/api/commands.md` holds the same table
for every other surface.

Registry help is literal text, not authored Markdown or HTML. The Markdown
mirrors and `llms.txt` escape Markdown punctuation rather than stripping it, so a
placeholder such as `<destination>` remains visible and a leading `-` stays text
rather than starting a list. Usage stays in a code
span, where prose escapes would change the displayed command. The drift reader
compares visible text: neutral emphasis, links and HTML wrappers are allowed,
but an unescaped tag that hides a placeholder is lost contract text.
<!-- source: internal/le/site/commands.go -- markdownProse, commandMirrorDescription -->
<!-- source: internal/le/site/derived.go -- writeLLMSCommands -->
<!-- source: internal/le/doc/yangcontract/command_surfaces.go -- markdownInlineVisibleText, validatePrimaryMarkdownContract, validateLLMSCommandContract -->

`internal/le/doc/yangcontract` publishes no page. Its unexported `renderCommandSurfaces`
writes a contract fixture into a temporary tree, and the documentation drift
gate reads that fixture and each published page with one reader, so a fixture
the reader rejects is a reader defect and a page it rejects is a page defect.
The fixture therefore carries the shape the site writes: the usage line after
the summary, the command facts after the usage line, the detail page's pipe
labels on both detail surfaces, the command's own pipes and aliases as the
detail mirror lists them, and the `usage` segment on the `llms.txt` line. The
reader reads the mirrors' stated absences (`not declared`, `none`, the bare
`Pipes` term) as absences, and counts a command's index row once, under the
full catalog heading, though the index lists a mapped command twice. The symbol
was exported until 2026-08-29, and a build that called it overwrote 396 pages
with the fixture.
<!-- source: internal/le/doc/yangcontract/command_render.go -- renderCommandSurfaces -->
<!-- source: internal/le/doc/yangcontract/command_surfaces.go -- validateGeneratedCommandSurfaces, htmlDescriptionAndUsage, equivalentMarkdownCommandIdentities -->

## Quality pages

`../gh-pages/quality/health/` and `../gh-pages/quality/rfc-compliance/` are
rendered from the tree being built, through the two packages that own those
numbers. Neither page computes a figure of its own.

The testing-health page reads `internal/le/test/health.Render`, which answers the
metric record and the Markdown mirror in one pass. The mirror it publishes is
`docs/features/test-health.md`'s own bytes, so the site is never a second author
of that document.

`../gh-pages/reference/rfcs/` is rendered from the same tree, and it is the one
page of the docs producer that is. Every other published Markdown source is
authored and is read from the checkout; `docs/features/rfc-status.md` is
generated from each summary's `## Meta` table and is not tracked, so the site
asks `rfc.StatusPage` for it instead. The renderer, the link rewriting and the
Markdown mirror are the docs producer's own, unchanged: only where the bytes
come from differs, and `liveDocSources` is the one table that says so.
<!-- source: internal/le/site/docs.go -- liveDocSources -->
<!-- source: internal/le/rfc/render_ledger.go -- StatusPage -->

The RFC compliance report reads `internal/le/rfc`: `Collect` for the
requirements and the test tags, `NewRenderInput` for the public ledger and the
recorded audit verdicts, and `Check` for the verdict and the open issues. It
writes `../gh-pages/data/rfc-compliance.json`, which is the same answer in
machine-readable form and is linked from the page.

One page per RFC sits under that route, one for every summary in `rfc/short/`,
190 of them today. The compliance page itself is the index over them: two link tables, one for the
enrolled summaries and one for the summaries that are not enrolled, so no page
of the family is reachable only through search. 39 stems carry no row in
`docs/features/rfc-status.md`, which is why the index lives here rather than on
the mirror of that page.

Each detail page opens with the card grid the index carries, over that RFC's own
numbers. **Scale leads, then standing**, on both pages: `Gated MUSTs` and `Out
of scope`, then the shares that partition the binding population, then the proof
ratio. Every card carries its value, the arithmetic under it, and a sentence
saying what the measure means. `rfcCardsHTML` renders the grid for both pages
and `rfcCardsMirror` states it in both mirrors.

**The ratio cards partition their denominator.** `rfcStanding` groups every
binding bucket into exactly one card, so the shares add to 100% and a reader who
adds them lands on the whole. Publishing two parts of a four-part split left
3.3% of RFC 4271 unexplained. `rfcLedgerCoverage.Bucket` is the one translation
between the index's bucket keys and a stem's own counters, so the two pages
cannot publish different partitions of one idea. The proof ratio is over TAGGED
UNITS, a different population, and the sentence under the grid says so.

The one bucket in no card is `rollup`, marked `Derived` in `rfcSatisfaction`. A
`{rollup}` row carries no obligation of its own, and `rfc.CoverageRows` never
counts it. So the bucket is outside the gated population, and it enters no
card, no tape segment and no row of the bucket table. The gate derives each
such row's state from the rows it names. `rollupDeriver.fill` writes `Requirement.Derived`
for the gate and for `rfc.NewRenderInput`, so every renderer reads the state
the gate reported.

The stem page lists those rows under `Derived from other rows`, apart from the
parts and marked as outside the population. Each row's marks carry a `derived`
mark beside the `{rollup}` reason that names its targets. The mark reads `met`,
or `gap` or `unproven` with the cause naming the target that decided it.

**A color names what the measure MEANS, never how well Ze scores on it.** Green
is a good outcome at any value, red a bad one above zero, and neither a
population nor a scope count is an outcome, so both take the neutral tone. The
number under the label already carries the performance; a color that graded as
well as labeled put an amber card on the measure that is the good news.

**The grid reads in four movements**: Overall, Positive, Neutral, Negative. A
card's movement is DERIVED from its tone, `rfcCardsIn`, so a heading and a color
cannot disagree: a card under "Negative" is red because that is the only way it
lands there. **The grid holds measures only.** How the gate is enforced -- the
pre-commit stage count, the reproduce command, the inputs it reads, the
artifacts it publishes -- is one section, "How this is checked". `Pre-commit
gate / ON` was a card until 2026-09-01; it answers how, not where Ze stands.

**Check results is a table**, one row per finding: the RFC, the requirement id
linked to its own page and row, the level, what is wrong, and the requirement's
own text. Those columns come from `rfc.Finding`, which carries the parts each
check had BEFORE it formatted its line, so nothing on the page parses that line
back apart. `CheckReport.Findings` is the one list and `Violations` is rendered
from it. A finding about a file, a ratchet or a ledger row carries no
requirement and states itself in the column it fills.

**Two mechanisms take an obligation off the gated ledger, and the index
publishes both.** `{not-applicable}` annotates a requirement that EXISTS, and
the Out of scope card carries it. An excluded site never becomes a requirement:
a reviewer walked the RFC's text sentence by sentence and declined to map that
one. The Exclusion disclosure section counts those by kind, reads the vocabulary
and each kind's meaning from `rfc.ExclusionKinds` and `rfc.ExclusionKindMeaning`,
and links every summary that used a kind to its own page where the reason is.
It states its own coverage on its face, because sign-offs exist for a minority
of summaries, and it states that `ai/rules/rfc-compliance.md` treats
`binds-another-role` as PRESUMED WRONG until justified -- publishing the largest
kind without that context would be the flattery failure one layer down.

**The kinds do not all mean the same thing, and the section splits on that.**
Five say the obligation never bound Ze. `relocated-to-spec` says the opposite:
it is real, Ze owes it, it is unbuilt, and a named spec owns it, which
`./le rfc check` verifies still reserves the requirement id. Those sit under
their own heading with the reserved id, the quoted sentence and the owning spec,
and no count sums them into "declined". The split is `ExclusionKindGroup`, so a
seventh kind lands in one group or reddens a test rather than defaulting to
scope.

**The page names where a fact is AUTHORED.** `rfc/enrolled.txt`,
`rfc/not-enrolled.txt` and `docs/features/rfc-status.md` are generated by
`./le rfc index-update` from each summary's `## Meta` table, so the published
prose names that table and the ledger reads `RenderInput.Metas` rather than the
generated copies. The enrolment and disposition renderers carry no fallback
branch: `ParseMeta` refuses a summary with no `| Enrolment |` row and refuses a
kind with no reason, and `loadRequirementLedger` refuses the state by name at
the artifact boundary rather than printing a placeholder for it.

**Every ratio is taken over the GATED population**, and no annotation takes an
obligation out of that denominator. `{not-applicable}` is scope rather than
coverage, so it is a named slice with a neutral card of its own rather than a
subtraction: removing it flattered every share above it, and the owner ruled
against that on 2026-09-02. `{lower-layer}` is a second neutral slice, added on
2026-09-03: the obligation binds Ze and a layer under Ze meets it, on state Ze
installs, so it is neither proven by Ze nor work Ze owes.
`{feature-declined}` is a third, added the same day: the obligation is
conditional on a feature the RFC makes optional and Ze does not offer, so its
condition is false and nothing is owed. That one is counted with
`{not-applicable}` in the `Out of scope` card, whose note names both kinds and
what each one says (`docs/contributing/rfc-conformance-gates.md`). The gated count keeps its place below the ratios as
the accounting total it is, and the bucket table states it as the sum of the two
populations. The page led with that count until 2026-09-01, when the owner
called the arrangement deceptive: a count of obligations judged reads as a count
of obligations met.

**The proof ratio sits immediately after the shares it corrects**, because a
test pair is not a proof. A break is observed ONCE and never re-run: what
`verifyOneDiscrimination` (`internal/le/rfc/discriminate.go`) re-checks on every
run is that nothing the red rested on has moved, so the unit still carries the
tag and the unit's behavior, the tag's claim and the producer's behavior still
hash to what the record stored. The card says that in its own words. An escape
claiming no break exists is not a proof, and neither is a LAPSED record whose
ground has moved: both are counted apart, and each appears under its own
requirement id in the proof-state section.

Under it the page carries the requirement table `rfc/requirements/<stem>.md`
carries, cell for cell, plus the requirement text, the per-RFC coverage
counters, every declared gap (naming the test that demonstrates it, where one
does) and every gated MUST with no test, the recorded
audit verdict and its freshness, what stands behind each tagged unit, the
extraction sign-off, and where a superseded obligation now lives.

Five shapes are load-bearing on that page:

- **A requirement id is never bare.** Its own row carries the anchor
  `id="<lowercased rid>"`, and every other mention -- a coverage bucket, a gap
  row, a proof-state heading, a superseded row -- links to it and carries the
  requirement's text where the mention is a row.
- **The proof state is one row per tagged unit**, with the polarity, the test
  file, the test function, the `kind/tier` carrier and the proof state each in
  its own cell. The sentence that explains `unproven` is a legend above the
  table, stated once.
- **Prose is not a table cell, and not one blob either.** The enrolment reason
  and the public ledger's cells sit under their own headings. A Coverage cell is
  a semicolon-chained list of claims and renders as one item per claim; a
  Remaining cell is a lead sentence and authored "Theme: body" groups and
  renders as those. `internal/le/site/rfcprose.go` does the split, with a depth
  counter so a semicolon inside `(...)` or inside a `{ med; }` code span never
  becomes a cut. The words are NEVER rewritten, paraphrased, summarized or
  truncated: the split is lossless, every item is balanced, and a cell carrying
  no such structure renders whole. Inside it every requirement id the RFC
  declares links to its row and every repository-rooted path links to its file;
  an id the RFC does not declare and a relative path citation are left as the
  author wrote them, because a link nobody can follow is worse than none.
- **Every table scrolls inside its own container**, `div.rfc-table-wrap`, the
  convention `.cmd-eq-table-wrap` already holds for the command family. A test
  path is one unbreakable token, so the page body would scroll sideways without
  it.
- **The requirement's sentence LEADS its row**, always visible, with no
  disclosure. Each requirement is two rows: the anchored id and its quoted
  sentence beside it spanning the rest of the row, then the level, the section
  and the tests. The
  sentence is what the row is ABOUT and everything else is metadata on it;
  hiding the subject while showing its attributes is inside out. It stays a
  TABLE rather than a block per requirement because the Proof state section
  already renders one block per requirement, and two lists of the same shape on
  one page help nobody. The colspan comes from `rfcRequirementColumns`, so a
  column added or removed cannot leave it wrong.
- **The tests read in carrier order**: kind first, tier within a kind, then
  polarity within a group, positive before negative. So every `unit/verify` row
  comes first, then `functional/verify`, then `interop/nightly`, and a reader
  sees at a glance whether an obligation is carried by unit tests or by a
  nightly interop run. The order is `rfc.CarrierRank`, declared in
  `internal/le/rfc/carriers.go` beside the kinds and tiers it orders, so a kind
  added to the carrier table cannot sort last on this page in silence;
  `TestEveryCarrierTheVocabularyDeclaresIsRanked` holds that over the real
  table. The full order is unit, functional, editor, interop, unknown by kind,
  and verify, nightly, unrun within each. A carrier the vocabulary does not rank
  sorts after every ranked one rather than at rank zero, and an absent-polarity
  row takes the rank of the requirement's best carrier so the gap shows inside
  the group the eye is already reading.
- **The requirement id sits beside its sentence**, in a narrow left cell, with
  the sentence in a cell spanning the rest of the row.
- **One table convention for the whole family, and no opt-out.**
  `.md-content td:first-child` is bold and sticky for every table on the site,
  which reads an identity into a first column. Eight of the eleven tables this
  family renders put a LABEL there instead, so the family drops the weight for
  all of them and keeps the stickiness that earns its place in a horizontally
  scrolling container. Every table goes through `rfcTableHTML`, which is what
  makes that true by construction rather than by discipline; 228 bold rows read
  as shouting, and two conventions on one page read as an accident.
- **Each stem page proves its own arithmetic.** Its coverage table carries a
  total row saying whether the partitioning buckets account for the gated
  population, and each bucket says whether the count the gate produced and the
  membership the page names agree. The counts come from `rfc.CoverageRows` and
  the membership from a second walk over the same requirements, so the two can
  disagree; the index carries the same shape of check against the gate's own
  gated total.
- **A page for a summary the gate does not hold never says it does.**
  `evaluate` (`internal/le/rfc/check_core.go`) skips every requirement of an
  un-enrolled RFC, so on those pages the population card names what the summary
  DECLARES and states that the gate holds none of it.
- **An un-enrolled kind is published with its meaning.** Six kinds exist:
  `enrolled` and the five dispositions `non-normative`, `backlog`, `blocked`,
  `source-restricted` and `out-of-scope`. The last says the extraction is DONE
  and the owner declined the feature, which is a scope decision rather than a
  conformance gap, and its public row may read only `Future` or `Unsupported`.
  The sentence for each comes from `rfc.DispositionKindMeaning`, so a seventh
  kind cannot print as a bare word.
- **Retiring a page is keyed on the marker this family writes**, never on a
  directory name absent from the live set. The output of a real build is the
  `../gh-pages` checkout, so a name test would delete any directory under
  `quality/rfc-compliance/` that another producer or an author put there.
- **The tests are a grid of divs**, one row per citation: the polarity, the kind
  and tier, then the test. The TIER LEADS, because a fixed-width token in front
  of a variable-width name wraps cleanly and lets the tiers align down the page.
  Divs rather than a nested table: a table inside a cell inherits the outer
  table's width pressure, which is what the restructure exists to remove. The
  block carries table roles so a screen reader meets data rather than a run of
  text, and it stacks per citation on a narrow viewport. A polarity with no test
  keeps its row and says so, because an absent polarity is a disclosed fact and
  a missing row reads as an oversight.
- **There is no Note column.** It carried four marks and each went where it
  explains something. The `{kind} reason` annotation and the nightly-only mark
  render beneath the tests they explain. The audit verdict and the superseded
  pointer are already rendered per requirement id under Proof state and
  Superseded, so they are not repeated. That was measured before the column
  went, not assumed: all 4 audit marks and all 310 superseded marks had those
  homes, while 374 of 375 `{single-polarity}` reasons and 14 of 851
  `{not-applicable}` reasons had none, which is why the annotation is rendered
  rather than dropped. `TestNothingTheNoteCarriedWasLost` holds it.
- **The Markdown mirror diverges here, and has to.** Markdown cannot nest a
  table inside a cell, so the mirror keeps labeled lines with the tier still
  leading each citation. Same population, same order, one shape it can express.

**A test is cited by NAME, never by the file it lives in.** The page printed
`internal/component/bgp/message/rfc4271_test.go` beside
`TestRFC4271MarkerAllOnesOnSend` until 2026-09-01; the path is machinery,
the link already resolves to the exact line, and the path cost width and gave
nothing. `internal/le/site/rfccitation.go` renders all three shapes: a Go test
and an interop checker by their function, a `.ci` or `.et` scenario by its own
file name, because the scenario IS the file. The whole path stays reachable as
the link's `title`, and as the link target in the mirror. Where TWO units on one
page share a test name -- three such collisions exist in this corpus, all three
with both units on one page -- the package directory is prefixed to both, and
only there, because rendering two different tests identically is the one thing
that change must not do.

The citations are built from the tagged units rather than from the shard's own
markdown, so the file, the line and the carrier are read as fields.
`TestEveryShardCitationIsATaggedUnit` holds that the two populations agree, over
10,768 cells.

Two links leave the page. A proof-state row links the TEST, not the path, to
`blob/main/<file>#L<line>` built from the line `rfc.Tag` records, so a reader
lands on the assertion. `repositoryBlobURL` and `repositoryLineURL`
(`internal/le/site/rfcmarkup.go`) answer every such URL, for this family and for
the documentation renderer, so the repository is spelled once.

The card colors are a vocabulary of four: `ok`, `neutral`, `warn`, `bad`. Every
card declares the RULE that chose its tone beside the tone, and
`rfcToneLegendHTML` publishes those rules under the grid. A number with no good
or bad direction -- a population, for instance -- takes `neutral` and gets no
color, because a color on a number is a verdict.

On the index page the requirement buckets carry a `Gated MUST-level requirements`
total for the implemented-RFC population and a sentence saying whether every
requirement in that population falls in exactly one bucket. The tape above it carries no text of its own:
a bucket at 4.5% of the width had a label wider than its segment, so the small
buckets were the unreadable ones. The tape is the proportion, the key beneath it
is the words, and every bucket the vocabulary declares has a color rule.

A section with nothing to show says so in one sentence. An omitted section and
an empty one read the same to a reader, and only one of them is a fact.

The gate verdict on the index page is a status block, `div.rfc-verdict`, tinted
by the same tone vocabulary as the cards. It was a `<pre>` block until
2026-09-01, which gave a one-line verdict terminal styling and the copy button
`website/assets/js/site.js` attaches to every `pre`; both told a reader to paste
a status into a shell. The invocation that reproduces the check is the only
thing on that block a reader runs.

The disclosure is FULL, by owner ruling of 2026-09-01. A gated MUST with no
test, an audit verdict of `weak`, `wrong` or `unimplemented`, a verdict that is
stale or shifted, a tagged unit with no discrimination record, and a `no-break`
record are each named on the page under the requirement id they belong to. A
count may accompany the list and never stands in for it.

For a partial requirement, the proof block states that the records cover only
scoped tag claims and give zero whole-requirement credit. The gate population
is the gated MUSTs of enrolled RFCs; published satisfaction shares use the
implemented RFCs within that population. Rollup rows belong to neither count.
These figures come from the native RFC collector, not hand-maintained totals.

The input is `../gh-pages/data/rfc-requirements.json`, derived once per build by
`publishRFCLedger` before any producer runs, in the way the plugin registry and
the command catalog already are. It is one reading of the checkout through
`rfc.Collect` and `rfc.NewRenderInput`. It never calls `rfc.Check`, which is a
second full pass that also type-checks every tagged package; the aggregate page
pays for that once and publishes the verdict it answers.

Two figures the retired renderer published are gone rather than ported. It
counted a per-check error total, which the Go gate does not answer: it returns
one list of open issues, and the page publishes that list. It also grepped a
marker string out of a hook file, a Makefile target and a status script, all
three of which were deleted with the interpreter, to claim an agent guard was
ON. The page states the live verification wiring instead, read from the declared
pre-commit stage population.
<!-- source: internal/le/site/health.go -- renderHealth -->
<!-- source: internal/le/site/rfccompliance.go -- collectRFCCompliance, rfcSatisfactionRows, rfcAccountedNote -->
<!-- source: internal/le/site/rfcledger.go -- collectRequirementLedger -->
<!-- source: internal/le/site/rfcdetail.go -- writeRFCDetailPage -->
<!-- source: internal/le/site/rfcevidence.go -- rfcProofHTML -->
<!-- source: internal/le/site/rfcmarkup.go -- rfcTableHTML -->
<!-- source: internal/le/verify/engine/stages.go -- fullStages -->
