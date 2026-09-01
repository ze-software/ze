# GitHub Pages and Presentations

Website sources live under `website/` on the main branch.
All `../gh-pages` content MUST be generated from this repository.
`./le site build` writes the publishable artifact to `../gh-pages` and removes
old source-only files there. It reuses matching demo artifacts. Run
`./le terminal-demo render-all` first to force new demo artifacts.
`./le terminal-demo render name <demo-id>` re-records one demo while you work on
its tape, and publishes it beside the artifacts it did not record. The ids are in
`demos/terminal/manifest.json`.

A recording runs in a container this repository builds and publishes to no
registry, so build it once per checkout with `./le terminal-demo image-build`.
It reads the image tag from `demos/terminal/manifest.json`, which is the tag the
recorder runs. A render refuses to start when that image is absent and names
this action. Rebuild after any change to `demos/terminal/Dockerfile`.

`./le site build output <directory>` builds into another artifact root, and
`./le site check output <directory>` judges that same root. Both default to
`../gh-pages`, so a session verifying its own work builds into its scratch
directory and checks there rather than writing over the published tree.

See `website/AI.md` for the full reference: structure, tools, and how to add a talk.


## Plugin catalog

The website plugin catalog at `../gh-pages/reference/plugins/` is
generated, not hand-authored. Its data source is each plugin's
`registry.Registration`: name, description, config roots, dependencies,
optional dependencies, and the YANG schema it registers.
<!-- source: internal/component/plugin/registry/registry.go -- Registration metadata -->

Two facts the catalog shows are DERIVED rather than declared, so do not look for
a field to fill in. The source directory is the package the plugin's engine
function was compiled in. The YANG file list is every `.yang` file in the
directory that holds the module the registration carries, and beside the
package when it carries none.
<!-- source: internal/le/inventory/plugins.go -- pluginPackageDir, pluginYANGFiles -->

A build reads the plugin registrations, writes
`../gh-pages/data/plugin-registry.json`, and renders the catalog plus one local
detail page per plugin. It reads the daemon's own configuration schema, writes
`../gh-pages/data/yang-config-tree.json`, and renders the configuration
reference from that tree and the same registrations. Do not add a parallel
hand-written plugin list.
<!-- source: internal/le/site/build.go -- refreshNativeSurfaces -->
<!-- source: internal/le/site/plugins.go -- renderPluginCatalog -->
<!-- source: internal/le/site/config.go -- renderConfiguration -->

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
asks `internal/le/docvalid.LiveCommandCatalog` for the answer `ze help command
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
for. `description` is the one-line summary, and all four producers print it
whole. `long-help` is the explanation, and only `renderEquivalentDetail` prints
it, as the detail page body. No producer derives one text from the other, and
none cuts either one. `docs/architecture/api/commands.md` holds the same table
for every other surface.

`internal/le/docvalid` publishes no page. Its unexported `renderCommandSurfaces`
writes a contract fixture into a temporary tree, and the documentation drift
gate compares each published page against that fixture. The symbol was exported
until 2026-08-29, and a build that called it overwrote 396 pages with the
fixture.
<!-- source: internal/le/docvalid/command_render.go -- renderCommandSurfaces -->

## Quality pages

`../gh-pages/quality/health/` and `../gh-pages/quality/rfc-compliance/` are
rendered from the tree being built, through the two packages that own those
numbers. Neither page computes a figure of its own.

The testing-health page reads `internal/le/testhealth.Render`, which answers the
metric record and the Markdown mirror in one pass. The mirror it publishes is
`docs/features/test-health.md`'s own bytes, so the site is never a second author
of that document.

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

**Every ratio is taken over the obligations that BIND Ze**, which is the gated
population less the `{not-applicable}` set. That set is scope rather than
coverage: the obligation never bound Ze, so counting it in a denominator
flatters every share above it. It is reported as its own card, in words that say
so, with the neutral tone. The gated count keeps its place below the ratios as
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
counters, every declared gap and every gated MUST with no test, the recorded
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
- **The requirement's own sentence is a row of its own**, spanning every column
  of the table, holding a `<details>` disclosure. It was inside the id cell
  until 2026-09-01 and expanded within the narrowest column on the page, which
  is where it was unreadable. A disclosure rather than a hover: hover is
  unreachable on a touch screen and invisible to a keyboard. The colspan comes
  from `rfcRequirementColumns`, the table's own declared columns, so a column
  added or removed cannot leave it wrong. The Markdown mirror keeps its Text
  column whole.
- **Both polarities share one Tests column**, a labeled line each, so neither
  list is squeezed into half the width. A polarity with no test SAYS so: "no
  negative test" is a disclosed fact under the disclosure ruling, and an empty
  half-cell is what a reader skims past.

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

On the index page the requirement buckets carry an `Accounted for` row whose
count equals the Gated MUSTs card, and a sentence saying whether every gated
MUST falls in exactly one bucket. The tape above it carries no text of its own:
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
<!-- source: internal/le/site/rfccompliance.go -- collectRFCCompliance -->
<!-- source: internal/le/site/rfcledger.go -- collectRequirementLedger -->
<!-- source: internal/le/site/rfcdetail.go -- writeRFCDetailPage -->
<!-- source: internal/le/site/rfcevidence.go -- rfcProofHTML -->
<!-- source: internal/le/site/rfcmarkup.go -- rfcTableHTML -->
<!-- source: internal/le/verify/engine/stages.go -- fullStages -->
