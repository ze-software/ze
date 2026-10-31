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
for a value of `data/rfc/<stem>.json`. The text inside is the last build's value
for readers without JavaScript, and `site.js` replaces it from the file at load.
A value printed as plain text is a second copy the file cannot update.
<!-- source: internal/le/site/factmark.go -- markedFact, siteFactSnapshot -->

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

RFC completion figures are not in `data/site-facts.json`. The build publishes
them under `data/rfc/`, one file per RFC, so a change to one RFC republishes
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
<!-- source: internal/le/site/rfcdata.go -- rfcShouldCountsOf, rfcBars -->

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

## Related

- `plan/journal/concurrent-session-corruption.md` -- the class, and its count
- `docs/contributing/running-commands.md` -- how a `./le` gate is run and read
- `ai/rules/repo-maintenance.md` -- a generated file and its source move in one commit
