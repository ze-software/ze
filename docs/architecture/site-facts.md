# Published Facts Come From Committed Data

<!-- source: internal/le/site/facts/sitefacts.go -- derive, write, factsFile -->
<!-- source: internal/le/site/facts/staleness.go -- check, compare, render -->
<!-- source: internal/le/site/facts.go -- factsFromRepositoryFile -->
<!-- source: internal/le/verify/engine/stages.go -- the site facts check stage -->

The website publishes numbers about this repository: how many Go packages it
holds, how many file headers explain themselves, how many peers it is tested
against. Each number is a claim, and this page states where a claim like that
must come from.

## The defect this page exists to remove

Several sessions share one checkout of ze. A tool that counts the working tree
counts every session's work in progress, and the output says nothing about it.
Two builds one minute apart then publish two different numbers for one
repository, and a reader cannot tell which is the repository and which is a
half-written file on somebody's disk.

`plan/journal/concurrent-session-corruption.md` records the class. Seventeen
generation targets in this repository derive from the working tree. This is the
one that was fixed first, and the pattern below is written so the other sixteen
have a template.

## The pattern

A published number is derived ONCE, deliberately, by a person standing in the
tree they are about to commit. The result lands in a committed file. Everything
downstream reads that file and never counts for itself.

| Step | What it is here |
|------|-----------------|
| One derivation | `derive` (`internal/le/site/facts/sitefacts.go`) is the only counter. The action that WRITES and the action that CHECKS both call it, because two counters over one tree drift by construction |
| One committed file | `website/data/repo-facts.json`, named by `factsFile` |
| One reader | `factsFromRepositoryFile` (`internal/le/site/facts.go`) fills the published snapshot from that file and refuses a fact the file does not carry |
| One gate | `./le site facts check`, a stage of `./le verify` |
| One fix | `./le site facts update`, named in every report the gate prints |

The commands are `./le site facts update` and `./le site facts check`. The pair
is the shape `./le test health update` and `./le test health check` already had,
and the pair is the point: a generated file nobody gates goes stale in silence.

The homepage's Go line figures follow that same route. `repo.go_source_lines`
counts tracked first-party Go files other than `_test.go`; `repo.go_test_lines`
counts those test files. Both include comments and blank lines and exclude the
third-party trees, named from the repository root: `vendor/`, `third_party/`
and `website/assets/vendor/`. A first-party directory merely named `vendor`,
such as `internal/le/web/vendor`, counts as source or test. Of the activity
collector's other exclusions they keep `tmp/*` and the dashboard page paths;
only the calendar also drops every directory named `vendor`. One counter,
`CountGoLines`, splits each file into code, blank and comment lines. Source includes
development tooling, so it is a repository-size figure rather than shipped
product code alone. The activity page states sizes from the committed size
history, and the deck's activity slide from a live checkout inventory; a
homepage figure is the inventory recorded by the facts update action.
<!-- source: internal/le/site/facts/sitefacts.go -- derive -->

`repo.go_vendor_lines` records the same collector's separate third-party
total, including comments and blank lines in tracked Go files under those
three trees. The homepage places this third-party code figure after test
lines. It counts lines
of dependency source; the activity inventory's module count remains a separate
measurement. All three line totals use the same display and staleness precision.
<!-- source: internal/le/site/facts/sitefacts.go -- derive -->
<!-- source: internal/le/repo/rewrite/activity.go -- collectGoStats, ThirdPartyPath, CountGoLines -->
<!-- source: internal/le/site/facts/golines.go -- displayedFact -->

## Not every fact can be committed

A published fact belongs to one of six categories, and the file records the
category of each one so a reader can tell what kind of claim a number is.

| Category | What it is a claim about | Example | Committed? |
|----------|--------------------------|---------|------------|
| Committed data | a commit | `repo.go_packages` | Yes |
| Built binary | a binary somebody built | `cli_commands`, from `ze help command --json` | No. Recorded under the `live` key, with no value and with the command that answers it |
| Network | a remote service | `github_stars`, from api.github.com | No. The build carries the previously published value when the reach fails |
| Network, committed by a command | a remote service whose answer changes only when the project acts | the release data (`release.*`), from the releases list on api.github.com | Yes, by `./le site release update` into `website/data/github-releases.json`. The build reads that file and never reaches the network for it ("Release data" below) |
| Site-owned | the website's own tree | the blog article count | Not about this repository at all |
| Already gated | a committed artifact another gate owns | the RFC requirement counts | No copy. A second record of one fact is the drift this pattern removes |

The rule that sorts them: **move a fact when its derivation WALKS, and leave it
where it is when it READS one artifact that is itself committed and gated.**
`liveFacts` (`internal/le/site/facts/sitefacts.go`) names the two facts this
tool cannot derive, so a number a reader cannot find in the file reads as
uncommitted rather than as forgotten.

The feature count is site-owned: it is the number of cards in
`website/data/features.json`. Each card's state is not. The build derives it
from the `features/*.md` declarations the card names, and refuses a card no
declaration backs, so the count never includes a card that describes nothing
(`docs/contributing/feature-maturity.md`, "The published surfaces").
<!-- source: internal/le/site/facts.go -- factsFromSiteData -->
<!-- source: internal/le/site/datapages.go -- loadFeatureData -->

## Two properties make the gate answerable

**It judges a COMMIT, never the working tree.** A check that read the tree would
answer differently in two sessions of one checkout, which is the defect it
exists to catch rather than a way to look for it. `check`
(`internal/le/site/facts/staleness.go`) materializes HEAD in a throwaway
worktree with `git worktree add --detach` and derives there.

**It judges the PUBLISHED figure, not the exact count.** A count reaches a page
through a formatter that floors a magnitude to one tenth of its visible unit, so
3852 and 3899 publish one string. Exactness is also not something the commit
flow can deliver: `git ls-files` answers from the index, so a regeneration run
before the files are staged cannot count what that same commit adds, and a gate
demanding exact equality would go red on the commit that fixed it. `render`
(`staleness.go`) is that formatter, and it MUST move whenever the site's own
display rule moves.

### Go line display precision

Go line totals write out "thousand" and "million" with two decimal places.
Thomas requested this convention on 2026-11-11: "M is not clear to be million".
Their staleness comparison uses that same formatter, so a visible
change is reported even when it falls inside an older count's rounding bucket.
Other repository counts keep their existing display precision. The published
snapshot carries raw line totals and their `_display` strings; every homepage
figure is marked so browser refresh uses the published value directly.
<!-- source: internal/le/site/facts/golines.go -- FormatGoLines, displayedFact -->
<!-- source: internal/le/site/facts.go -- factsFromRepositoryFile -->

## The dirty-tree warning

`warnUncommitted` (`internal/le/site/facts/actions.go`) names every Go file the
tree and the last commit disagree about, before the file is written. This is the
half that stops the tool becoming the defect it removes. Without it, a
regeneration records another session's uncommitted edit in a committed file, and
nothing in that file says where the number came from.

## Applying this to another generated file

1. Find the walk. A derivation that reads ONE committed artifact is already
   committed data and needs nothing.
2. Give the walk one home, called by both the writer and the checker.
3. Write the result to a committed file, with a category and a source sentence
   per fact, so a reader can re-derive a number by hand.
4. Register a `<area> update` action with `Writes: true` and a `<area> check`
   action without it (`internal/le/le/action`).
5. Warn on a dirty tree before writing.
6. Add the check as a stage of `./le verify`, so it runs where every other
   generated-file gate runs.
7. Make the check judge a commit, and compare what a reader sees rather than
   what the tool counted.

## One published copy of each value

Every value the site shows that changes over time is published once, in the
artifact's `data/site-facts.json`, and a page shows it only inside an element
that names its key: `data-ze-stat="<dotted.key>"`, or `data-ze-rfc="<stem>:<field>"`
for a value of `data/rfc/<stem>.json`, or `data-ze-history="<subject>:<field>"`
for a repository size of `data/repo-history.json`, the committed size history. The text inside is the last build's value
for readers without JavaScript, and a script replaces it from the file at load.
A value printed as plain text is a second copy the file cannot update.
<!-- source: internal/le/site/factmark.go -- markedFact, siteFactSnapshot -->

The size history's marker is written only by `historyValueHTML`, and only the
activity page carries it. Its subject is a section identifier the file lists,
several of them joined by `+` (their sum), `total` (every section summed) or
`day`. A section or a sum carries `files`, `lines`, `gained` and `lost`, and a
Go section `code`, `blank` and `comment` as well; `day` carries `date` and
`modules`. Every value is the newest recorded day's. The build reads the file
through the history package's `ReadHistory`, refuses a checkout without it with
`./le site history update` as the fix, and writes each value inside its marker
(`historyFigures.fillHTML`). It refuses, by name, a marker naming a section the
file lacks, a field its subject does not carry (a Go field on a section without
the split included), and a marked element not in the helper's shape
(`historyStrayMarker`). The page's own script, not `site.js`, refills the
markers from the published copy. The build stages that copy like any other file
of `website/data/`, and `namedArtifacts` lists it, so an artifact without it fails
`./le site check`.
<!-- source: internal/le/site/activityhistory.go -- historyValueHTML, historyFigures, historyStrayMarker, historyDataFile -->
<!-- source: internal/le/site/activity.go -- renderActivityPage -->
<!-- source: internal/le/site/producer.go -- namedArtifacts -->

| Part | What it is |
|------|-----------|
| The one helper | `markedFact` writes the marked element; a hand-written source names a value with a `{{ze:<dotted.key>}}` token, resolved by `substitute` (Markdown) and `substituteHTML` (HTML) |
| The key set | the snapshot's own dotted JSON keys, walked from the published file (`siteFactSnapshot.keys`), so a map-typed section's keys count too and no name table exists |
| A hand-marked element | a hand-written HTML source may carry `data-ze-stat="<key>"` around fallback text. The build writes the snapshot's value into it when the key holds one, and leaves the fallback while the key's section is `null` (`markedValue`), which is what `site.js` does in the browser. A key neither the snapshot nor the `siteFacts` model names is refused by name, and so is markup or a `{{ze:}}` token inside the element, whether or not the key's section holds a value yet (`readMarkedText`) |
| The guard | `./le site check` reports `plain-figures`: each display string (a thousands separator or a "+") and each whole number of 100 or more beside its unit word that a page shows outside a marker (`checkPlainFigures`) |
| The RFC ratchet | `website/data/plain-figure-ratchet.json`, committed and never deployed, lists the RFC pages (under `quality/rfc-compliance/`) whose figures are still server-written, each with its reason. A listed page's plain figures are not refused; a listed page that shows none is reported under `stale-ratchet` and fails the check, so the list only shrinks. The check never writes it, and refuses by name a missing file, an unknown field, a page outside the RFC section, an unclean path, a page listed twice and a page with no reason (`loadPlainFigureRatchet`). An RFC page not listed is judged like any other. `spec-rfc-figures-sectioned-presentation`'s website phase emptied it: no RFC page is listed, so every RFC page is judged like any other |
<!-- source: internal/le/site/plaintext.go -- checkPlainFigures, publishedFigures, plainFigures, loadPlainFigureRatchet -->
<!-- source: internal/le/site/factfallback.go -- markedValue, modelNamesFactKey, readMarkedText -->

A producer that writes a value itself, rather than through a token, reads it
with `factLookup`: `marked` for the page, `plain` for its mirror and `llms.txt`.
Every reader of a value, a token or a lookup, resolves it through
`siteFactSnapshot.answer`, which refuses by name a key the snapshot does not
name and a key whose value is the empty string, so no page shows an empty
figure. The values a page counts are counted once, in `deriveSiteFacts`, by the same
function the page groups with, so the snapshot and the page cannot name two
populations under one key:

| Key | Counted by | Shown on |
|-----|-----------|----------|
| `cli_commands`, `cli_groups` | `factsFromCommandSurfaces`: the catalog, and `groupCommands` over it | the shared header, `/reference/cli/`, `/reference/command-equivalents/`, `llms.txt` |
| `cli_group_commands.<group>` | `factsFromCommandSurfaces`: one count for each group `groupCommands` makes, keyed by the group's slug (`cliGroupKey`) | `/reference/cli/`: the group navigation, each group's summary, and the mirror's group headings |
| `equivalents.mapping_entries`, `.reviewed`, `.vendor_cli`, `.vendor_only` | `factsFromCommandSurfaces`: `equivalentMapping.rows` and `countEquivalentRows` | `/reference/command-equivalents/` |
| `equivalents.vendors.<vendor>.commands`, `.lines`, and `equivalents.vendor_lines` | `countVendorCoverage` for each vendor the map declares, keyed by its id (`vendorCoverageKey`); the total is their lines summed | the coverage cards of `/reference/command-equivalents/` |
| `equivalents.groups.<group>.equivalent`, `.reviewed`, `.total` | `countEquivalentGroup` for each verb table `groupEquivalentRows` makes, keyed by the verb's slug (`equivalentGroupKey`) | each verb summary of the full catalog on `/reference/command-equivalents/` |
| `features.categories.<category>` | `featureCategoryCounts`, a map with one key for each category the features page has | the homepage category buttons |
| `plugin_registrations` | `factsFromSiteData`: the entries of `data/plugin-registry.json` | `llms.txt` |
| `health.<metric>.value.<n>`, `.detail.<n>`, `.action.<n>` | `healthFacts`: the n-th figure of the composed field (`healthTextParts`), keyed by the metric's own `key` | `/quality/health/`: each card and the attention table compose the sentence from marked figures around the record's own words |
| `health.<metric>.<ratio>.numerator`, `.denominator`, `.percent` | `healthFacts`: the payload object the meter is drawn from (`healthRatio`), the percentage as the page prints it | `/quality/health/`: the meter note, and the meter's bar, whose fill names `.percent` in `data-ze-stat-width` so `site.js` sets its width from the snapshot (`healthMeter`) |
| `health.<metric>.worst.<row>.<column>`, `.orphans.<row>.<column>`, `.buckets.<bucket>.<field>` | `healthFacts`: every number a supporting table prints (`healthCellFigure`); a row's name stays a word | `/quality/health/`: the supporting tables |
| `health_trends.<series>.samples`, `.latest`, `.drawn`, `.low`, `.high` | `healthFacts`: each series `TrendSeries` names (`healthSeriesValues`); `latest`, and the count, low and high of the samples the line draws (`healthDrawnSamples`, `healthSparkRange`), only once the series has `healthMinSamples` | `/quality/health/`: the trend table, and the screen-reader sentence beside each line. No figure stands in an `aria-label`: the meter's names its metric, and the line is `aria-hidden` |
A map-typed section takes its keys from the data, never from a name list: a
vendor or a group the data gains reaches the snapshot and its page with no
edit. A name that holds a dot cannot be a key segment, and `factKeySegment`
refuses it by name, as it refuses two groups whose slugs collide.

<!-- source: internal/le/site/facts.go -- factsFromCommandSurfaces, featureCategoryCounts, factsFromSiteData, factKeySegment -->
<!-- source: internal/le/site/factmark.go -- factLookup, answer -->
<!-- source: internal/le/site/equivalents.go -- countVendorCoverage, countEquivalentGroup, vendorCoverageKey, equivalentGroupKey -->
<!-- source: internal/le/site/commands.go -- cliGroupKey -->
<!-- source: internal/le/site/healthfacts.go -- healthFacts, healthTextParts, healthRatio, healthCellFigure, healthSeriesValues -->

The health sections carry each figure as the string the page prints, because
the record spells a score as `0.0` and a number would publish it as `0`. They
carry no label, prose or row name, only figures, so the snapshot grows by about
4 KB over this tree's record. A string that is not a display string is not
matched by the plain-text guard, so the health figures are policed by
`TestEveryHealthFigureIsAMarkedSnapshotValue`, which requires no number left
on the page outside a marker. The Markdown mirror stays the generator's own
`docs/features/test-health.md` (`renderHealth`), so it states the same reading
in the generator's words.

A page description (the meta, Open Graph and Twitter ones) carries no value at
all: a link preview reads it without the page's script, so a figure there is a
copy nothing refreshes.

The guard reads what a reader sees, the head and the text attributes included,
and judges the published string, as the gate above does. A value under 100 with
no separator is matched only through its marker, because bare small numbers are
everywhere in prose.

## RFC completion data

RFC completion figures are not in `data/site-facts.json`, with one exception:
the home page card's `rfc.proven`, `rfc.must_kept`, `rfc.proven_percent`,
`rfc.implemented` and `rfc.not_walked` (each with its `_display` form), which
`factsFromRFCLedger` reads from `rfc.ImplementedSectionsOf`
(`internal/le/rfc/implemented.go`): Ze proven over the MUST rows Ze and the OS
owe, summed over the walked RFCs Ze implements. The build publishes the
per-RFC figures under `data/rfc/`, one file per RFC, so a change to one RFC republishes
that file and no page. Every count is read from `rfc.StemFiguresOf`
(`internal/le/rfc/profilecoverage.go`); `rfcStemDataOf` only splits its fields
into the two bars' segments, and refuses by stem any figures whose split does
not add back to the model's own totals. The directory is removed and written
whole on every build, so no file outlives the index that names it.
<!-- source: internal/le/site/rfcdata.go -- rfcStemDataOf, rfcProfileDataOf, rfcBars, writeRFCData -->

| File | Holds |
|------|-------|
| `data/rfc/index.json` | `bars`: both bars' segments left to right, each with its class name (`segment`), its words (`label`) and the dotted `fields` whose sum is its count, and the fields each bar's segments must add up to (`must-total`, `overview-total`). Then `rfcs` (stem, file, title, walked, enrolment, and `gated`, which is `rfc.Meta.Enrolled`: the enrolment row says enrolled and the Implementation kind counts against Ze's Go) and `profiles` (name, file, kind, title) |
| `data/rfc/<stem>.json` | `stem`, `title`, `walked`; `requirements`, every requirement row the summary declares at every level, walked or not (`rfcRequirementCounts`); `implementer`, the summary's Implementation kind (`ze`, `mixed`, `third-party`, `foundation`); `implemented`, `rfc.Implements` over the summary (enrolled and rendering a public row); `must` (`total`, `kept`, `ze-proven`, `os-verified`, `os-gap`, `ze-gap`, `not-owed`, `not-owed-by-kind`); `should` (`total`, `proven`, `os-verified`, `os-gap`, `not-owed`, `not-ever`, `not-yet`, `not-decided`, `not-owed-by-kind`); `proven-by-kind`; `unmet`. `must`, `should` and `proven-by-kind` are `null` for an RFC no walk bounded |
| `data/rfc/profiles/<name>.json` | `name`, `kind` (`use-case` or `technology`), `title`, `release`, and the two tiers `required` and `optional`, never merged. An aggregate's optional tier is the summaries whose `Aggregate tier` Meta row says `optional`. Both tiers come from `rfc.ProfileTiersOf` (`internal/le/rfc/priority.go`), the same membership `./le rfc tree` reads |

`must.kept` is the MUST bar: `ze-proven + os-verified + os-gap + ze-gap`.
`must.not-owed` is the rest of `must.total` (not applicable, declined, OS
blocked). A SHOULD row a lower layer performs is split the same way, into
`should.os-verified` and `should.os-gap`, and never counts as
`should.not-owed`, which holds only the rows that do not bind Ze. On the
overview bar the OS verified and OS gap segments sum MUST and SHOULD rows
(`must.os-verified + should.os-verified`, `must.os-gap + should.os-gap`); the
MUST bar counts MUST rows only.
<!-- source: internal/le/site/rfcdata.go -- rfcBars -->
<!-- source: internal/le/rfc/sectioncounts.go -- SectionsOf -->

A page marks a value with `data-ze-rfc="<subject>:<field>"` and a bar with
`data-ze-rfc-bar="<subject>:<bar>"`, where the subject is a stem, a profile
tier (`profile/<name>/required`, `profile/<name>/optional`), `enrolled` or `all`, and the
bar is `must` or `overview`. `enrolled` is every RFC the index marks `gated`, the RFCs the gate holds Ze to: a summary whose enrolment says enrolled but whose Implementation kind is `third-party` or `foundation` is not one of them. `all` is every RFC the index lists; in the
browser an ancestor carrying `data-ze-rfc-stems="<stem>,<stem>"` narrows it to
those stems, which is how a filter sums only the RFCs it shows. The build
fills `all` over the whole index. Besides the fields of a per-RFC file, a set answers
`rfcs`, `walked`, `not-walked`, `requirements` (summed over every member, walked or not; a file without the count is refused), `implemented` (how many members carry `implemented: true`, the RFCs Ze implements; a file without the flag is refused) and `register`, its mandatory register in
words: `complete` only when the set holds at least one RFC, none of them is
unwalked, and `must.ze-gap` and `must.os-gap` are both 0, `not complete`
otherwise. `assets/rfc-figures.js` reads the index, sums a set's
walked members count by count (the rest are `not-walked`), writes each value,
and draws each bar's segments in the index's order, each carrying its words and
count as text and a width of its count over the bar's total. A legend under the
bar names every non-zero segment again with its count and a colour swatch, so a
segment too narrow for its words loses none of them. A set with no member shows
`no rows`, one whose members are all unwalked `not walked`, and a
`not-owed-by-kind` key no file holds shows 0. The text inside a marked element
is the last build's value: a renderer writes the element empty, and
`rfcFigureSet.fillHTML` fills it from the `data/rfc/` files the same build
published, answering each case as the script does, a bar as its non-zero
segments' words and counts. A Markdown mirror carries the same values through
`fillMirror`. Both refuse the page when a marked element has no answer, when
a field names a not-owed kind no row can carry, or when a marker is not in the
exact shape the helpers write (another attribute, another tag, a mistyped
mirror token), which the fill could not reach. So no page prints a figure the
files do not hold, and none publishes a marker left empty. The RFC profile pages
and the per-RFC detail pages are marked this way (`docs/contributing/gh-pages.md`). A file the index names that cannot be read is named
in the element with the command that publishes it (`./le site build`), never
dropped from a sum. The segment colours are the `.rfc-seg-<segment>` rules of
`website/assets/css/30-components.css`; OS verified has its own token,
`--green-soft`, lighter than `--green` in both themes, and the words on a light
fill use `--ink-on-light`, dark in both. Every segment's words meet 4.5:1 on
their fill in both themes.
<!-- source: website/assets/js/rfc-figures.js -- membersOf, sumStems, registerOf, drawBar -->
<!-- source: internal/le/site/rfcfallback.go -- rfcFigureSet, members, rfcRegisterOf, rfcKindKnown, rfcStrayMarker, fillHTML, fillMirror -->
<!-- source: internal/le/site/browsertest/rfcfigures_test.go -- TestTheBrowserDrawsRFCFiguresFromTheirFiles, TestRFCSegmentColoursAreReadableInBothThemes -->

The browser test draws the layout the build publishes, not a copy of it:
`TestTheBarLayoutGoldenIsCurrent` writes `rfcBars` and a sample per-RFC file to
`internal/le/site/testdata/rfc-bars.json`, the browser test reads that file as
its index, and checks its own per-RFC fixtures against the sample's fields. A
stale golden fails with the command that refreshes it:
`./le job run label unit-pkg command go test -run TestTheBarLayoutGoldenIsCurrent ./internal/le/site/ -args -update`.
<!-- source: internal/le/site/rfcdata_test.go -- TestTheBarLayoutGoldenIsCurrent -->

## Release data

The newest published release is a network fact, but it changes only when the
project publishes one, so it is committed rather than fetched by every build.
`./le site release update` reads the releases list
(`https://api.github.com/repos/ze-software/ze/releases`, every page, within one
5-second bound and 8 MiB a page) and writes `website/data/github-releases.json`:
the endpoint it read, then each published release newest first with its tag,
title, publication time, prerelease flag and page URL, and each asset's name,
size, sha256 digest and download URL. It reads the list rather than
`/releases/latest`, because the list answers `[]` while no release exists and
`/latest` answers 404, so "none yet" is a state the file can commit. Drafts are
left out: no reader can download one.

The command refuses, and leaves the committed file as it was, when the forge
cannot be reached or answers something the file must not hold: a URL that is
not https on `github.com` (or, for a download, `objects.githubusercontent.com`),
a negative size, a digest that is not sha256, or more than a thousand
releases. The refusal names the endpoint, the cause and the retry command. The
site build reads the file through `siterelease.Read`, which applies the same
checks, and refuses a checkout with no file, naming the command that writes it.
<!-- source: internal/le/site/release/release.go -- Endpoint, fetch, Read, Release.check, Asset.check -->
<!-- source: internal/le/site/release/actions.go -- update -->

| Key | Derived by | Shown on |
|-----|-----------|----------|
| `release.version`, `.published`, `.url` | `factsFromReleases`: the newest release of the file, prereleases included | nothing yet |
| `release.iso.size_display`, `.bytes`, `.name`, `.sha256`, `.download_url` | `releaseISO`: the one asset whose name ends `.iso`, the size in decimal megabytes with one decimal (`megabytes`) | `/labs/appliance-install/`: the ISO size, whose fallback text "under 150 MB" stays until a release carries an ISO |

While the file states no release, the snapshot holds `"release": null` and
`_sources.release` says "none yet". A release with no `.iso` asset holds
`"iso": null`, and a release with two is refused, because the lab page states
one image.
<!-- source: internal/le/site/releasefacts.go -- factsFromReleases, releaseISO, megabytes -->

## Release inventory

The release roadmap derives directly from an immutable Git tree. On every full
or partial site build, `renderRoadmap` calls the shared spec collector once at
committed `HEAD`. That snapshot supplies the page, its Markdown mirror, and
`data/release-roadmap.json`. The JSON carries its own input digest and full
revision, because the site's website-only source digest does not cover `plan/`.
When a spec declares `RFC gaps`, the same snapshot's Profiles column reads
`rfc/short/` and `rfc/profiles/` from that commit too, and those files join
the input digest
(`docs/contributing/spec-workflow.md`, "Committed release roadmap").
<!-- source: internal/le/site/roadmap.go -- renderRoadmap -->
<!-- source: internal/le/spec/roadmap/roadmap.go -- Collect -->
<!-- source: internal/le/spec/roadmap/profiles.go -- loadRFCCorpus -->

Uncommitted edits and untracked specs cannot enter that inventory. Missing
history or an unreadable plan tree stops the build. The output remains an
inventory preview pending owner classification, and counts include each
recorded spec regardless of lifecycle state. A deletion alone supplies no
evidence of delivery. This separate provenance boundary leaves `repo-facts.json`
and its existing update/check gate unchanged.
<!-- source: internal/le/spec/roadmap/roadmap.go -- Collect -->

## Size history

The repository's size on every day of its history is the pattern's second
instance. `./le site history update` measures it from committed git objects
into `website/data/repo-history.json`, and the activity page graphs that file.
The file is written by the command and never by hand: a figure edited by hand
breaks the line arithmetic the reader checks, or states a size no tree had.
Unlike `./le site facts update` it reads no working tree and no index, so it
needs no worktree and prints no dirty-tree warning.
<!-- source: internal/le/site/history/file.go -- HistoryFile, NewHistory, ReadHistory, WriteHistory -->

**What the file records.** A schema version, the rules identifier, the
section list (identifier, label, and whether the section carries the Go
split), and one line per recorded day: the UTC date, the commit whose tree was
measured, the module count of that tree's `vendor/modules.txt` (0 when it has
none), and for each section its files, lines, and the lines it gained and lost
since the previous recorded day, with code, blank and comment lines for a Go
section. A section that holds nothing and changed nothing that day is left
out. The file carries no byte sizes and no commit counts: commits per day stay
the calendar's measurement.
<!-- source: internal/le/site/history/measure.go -- dayRecord -->

**Which commit measures a day.** The history is `HEAD`'s first-parent history,
resolved once. A commit's day is its author day in UTC, raised to its parent's
day when earlier, so days never run backward; the committer date places no
commit, because a March 2026 rewrite stamped 1,727 commits with one. A day is
measured at its last commit in first-parent order, and a day with no commit is
not recorded. A commit authored on a later day than it was committed is
refused by name, because the raise would carry every later commit onto that
future day.
<!-- source: internal/le/site/history/days.go -- SelectDays -->

**How a file counts.** Only regular files count: a symbolic link and a
submodule are not. A blob with a NUL byte at offsets 0 to 7,999, git's own
probe, is binary and counts as one file with zero lines, a Go file included.
Any other blob counts one line per newline, plus one for an unterminated last
line. A `.go` blob in a Go section is also split into code, blank and comment
lines by `CountGoLines`, the counter the homepage figures use, so the split
sums to the line count. Each blob is read once per run, streamed one at a time,
and only its counts are kept.
<!-- source: internal/le/site/history/measure.go -- countBlob, measureTree -->
<!-- source: internal/le/repo/rewrite/activity.go -- CountGoLines, CountVendorModules -->

**Sections.** Every path falls in exactly one of twelve sections, by the first
rule of the ordered table `sections` that matches, the last section taking
every path no rule matches: third-party Go and third-party other (the trees
`ThirdPartyPath` names from the root), Go tests, Go production, functional
tests, RFC corpus, plans, agent rules, documentation, website, schemas and
other. The table is declared there once, and the file carries its identifiers
and labels, so the page reads them from the file. The rules identifier is a
digest of the table, of the third-party trees and of the two counting-rule
versions (`CountingRulesVersion`, `GoLineRulesVersion`); a test pins it, so a
rule change is made on purpose and followed by a whole-history run.
<!-- source: internal/le/site/history/sections.go -- sections, SectionOf, RulesIdentifier, rulesText -->

**Gained and lost.** Against the previous recorded day, per path: a new path
gains all its lines, a removed path loses them, a path whose section changed
loses its old lines in the old section and gains its new lines in the new one,
and a path that stayed gains or loses the difference. The first day of the
history gains its whole tree. For every section and every pair of consecutive
days, size equals the previous size plus gained minus lost, and no file that
breaks it is written or read.
<!-- source: internal/le/site/history/measure.go -- dayRecord -->
<!-- source: internal/le/site/history/file.go -- checkArithmetic -->

**The three forms of the update.** Every refusal names its repair and leaves
the committed file as it was; the file is written last, through a temporary
file and a rename.

| Form | Measures | Refuses when |
|------|----------|--------------|
| `./le site history update` | From the newest recorded day, re-measured at that day's current last commit, to the newest day of the history; the whole history when no file exists yet | The file was measured under other rules, or a kept day's commit left the history or is not its day's last commit (both name `./le site history update all`) |
| `./le site history update from <day> to <day>` | Those days inclusive, either end open to the first or newest day; the recorded day before the range is measured at the commit the file records, as the baseline its first day is counted against | As above; no file yet (names `all`); a day not written `YYYY-MM-DD`, `from` after `to`, or a range holding no day of the history; a range that would leave a history day unrecorded between the file and the range (names `./le site history update`); a merge whose kept neighbour no longer adds up (names `all`) |
| `./le site history update all` | The whole history, replacing the file | `all` beside `from` or `to` |

A kept day older than the newest recorded day must record its day's last
commit. The newest may record an earlier commit of its day: the commits made
later that day are lag, and the next default run measures them.
<!-- source: internal/le/site/history/update.go -- updateHistory, planUpdate, planRange -->
<!-- source: internal/le/site/history/actions.go -- parseUpdate -->

**The check.** `./le site history check` writes nothing and judges the file
in the checkout against `HEAD`'s first-parent history. It answers 1, naming
the day, the section where there is one, both values where there are two, and
`./le site history update all` as the repair, when:

| Refusal | What it proves |
|---------|----------------|
| The reader refuses the file: absent, malformed, an unknown field, a Go split that does not sum to its lines, or a size that is not the previous size plus gained minus lost | The contract, and the line arithmetic of every day |
| The rules identifier, or the section list under it, is not the tool's | Every day was measured under today's rules |
| A recorded commit is not on the history with that day as its day, or a day older than the newest does not record its day's last commit | The file describes this history |
| A history day between the first day of the history and the newest recorded day is absent | No gap |
| The newest recorded day, measured again, differs in its module count or in any count of any section | One full measurement |

The newest day is measured again at the commit the file records, never at its
day's current last commit, against the recorded day before it at that day's
recorded commit. The commit that lands the file is itself a later commit of
that day, and it changes the tree by adding the file, so a check that followed
the day forward would refuse the commit that satisfied it. Commits after the
recorded one, the same day or later, are reported as lag, with the default
`./le site history update` that measures them, and never fail: new commits
must not turn a gate red.

The check does not measure the older days again. Their file counts and Go
split are proven by a whole-history run: its output is deterministic, so an
altered figure shows as a diff of the committed file. A history the check
cannot read (no `HEAD`, a shallow clone, a missing object, a file the disk
refuses) answers 2, a question that was never put. It is the stage
`site history/check` of the full verification, which runs in a worktree at
`HEAD`, so there the file judged is the commit's own; CI clones the whole
history for it.
<!-- source: internal/le/site/history/check.go -- checkHistory, judgeRecorded, remeasureNewest, compareRecord, checked -->
<!-- source: internal/le/site/history/actions.go -- check, refusalCode -->
<!-- source: internal/le/verify/engine/stages.go -- fullStages -->

## Related

- `plan/journal/concurrent-session-corruption.md` -- the class, and its count
- `docs/contributing/running-commands.md` -- how a `./le` gate is run and read
- `ai/rules/repo-maintenance.md` -- a generated file and its source move in one commit
