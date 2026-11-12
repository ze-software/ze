# Spec: Activity codebase growth

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P3 |
| Release reason | Correct public project-size reporting; no shipped routing or lifecycle behavior changes. |
| RFC gaps | - |
| Depends | - |
| Phase | implementation: phases 1 to 4 committed (`6b71cb9764`, `f9c89e551d`, `7c2bdfeae7`, `71aa9c7188`, `13276d9afe`); phase 5 (canonical artifact and inspection) in progress; closure is `/ze-close` |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Give the project a tool that measures how large the repository was on every day
of its committed history, split by section, and make the public activity page
graph those sizes over time. The page today headlines "Total added lines", the
sum of every line every commit added. That figure is churn: a line rewritten
ten times counts ten times and a deletion never subtracts. It reads as the
project's size and it is not.

### Owner requirement, 2026-11-11

> So the report on http://10.12.100.120:8000/project/activity/ is BULLSHIT

> I want to show the growth of the codebase NOT the change over time

### Owner requirement, 2026-11-12

> Looking at the front page and this page, the number of 5 million is totally
> wrong as this reflects the changes and not the evolution of a time and total
> number of lines in the project. We need a tool to be able go back over all
> the committed repo and establish for each one and for each day, more than
> each one for each day, what were the addition to all the new total offline of
> the repo within the different section like we do in the report to graph all
> the value over time.

The objective was quoted back and he approved it ("yes, go ahead, include
non-Go content too"), then added:

> Make sure the tool can be used for any time period so we can use it to update
> the time as we go.

The approved objective, as quoted back to him:

| Part | What it delivers |
|------|------------------|
| A `./le` tool that walks all committed history | Every day that has commits, measured at the last commit of that day |
| Per day, two facts | What that day added and removed, and the resulting total size of the repository |
| Split by section | The sections of the Go composition report (production, test, third-party), and non-Go content |
| One committed data file | The site build renders it and never re-measures; a later run measures only the days it is asked for |
| Graphs of size over time | On the activity page, replacing cumulative added lines as the picture of growth |
| Any time period | The same command backfills all history once and extends the record as work continues |

Approved with it: the calendar heatmap stays, only the added-lines total stops
being shown as a size, the cumulative added-lines curve gives way to size
curves per section, and nothing else leaves the page.

### What the measurement shows (prototype, 2026-11-12)

A throwaway prototype measured the whole history from git objects in 84
seconds: 327 days, 158,845 unique blobs. It is design evidence, not the tool.

| Day | Go production | Go tests | Third-party | RFC corpus | Plans | All other sections | Total lines |
|-----|---------------|----------|-------------|------------|-------|--------------------|-------------|
| 2025-12-19 | 0.01M | 0.01M | 0 | 0 | 0 | 0.03M | 0.05M |
| 2026-03-30 | 0.22M | 0.24M | 0 | 0.07M | 0.03M | 0.13M | 0.69M |
| 2026-06-18 | 0.49M | 0.46M | 0.88M | 0.12M | 0.08M | 0.27M | 2.30M |
| 2026-09-07 | 1.05M | 1.04M | 2.17M | 0.41M | 0.16M | 0.47M | 5.30M |
| 2026-11-12 | 1.59M | 1.86M | 2.31M | 0.96M | 0.48M | 0.69M | 7.89M |

The current size figures on the front page (1.58, 1.86 and 2.00 million Go
lines) are true inventories: the committed tree holds 7,274 production, 8,453
test and 6,568 third-party Go files. Only the 5.8M "Total added lines" is
churn. It sits near the true 5.4M Go total by coincidence.

### History of this spec

The first design (commits `569132f6b1`, `6090af4551`) measured first-party Go
only, inside the site build, over the page's one-year window, with no command.
It was admitted on 2026-11-11 and never started: the WIP cap refused its claim
(18 in progress against a cap of 12). The 2026-11-12 requirement replaces that
design in three ways: every section is counted, a ranged command measures, and
one committed file carries the result. The homepage unit wording correction
recorded in the first design landed in `e8780328d4` and is not owned here.

### Owner decisions

Put to him on 2026-11-12 with the committed design (`f94de21043`). His answer,
the same day: "cap exception granted, go ahead with your readings". That
grants O-1 for this spec alone and confirms the readings of O-2 to O-4, the
gained-and-lost meaning of "additions" (A-4) included.

| # | Decision | Evidence | Reading taken |
|---|----------|----------|---------------|
| O-1 | A one-off WIP cap exception to start implementation | `./le spec claim` refuses: 16 specs in progress against a cap of 12 | Granted for this spec: the claim sets the cap one above the in-progress count |
| O-2 | A growth chart on the front page | The front page shows three true size figures and no churn figure; he removed its inventory note himself on 2026-11-11 | No chart is added there |
| O-3 | The front page's three line figures read the history's newest day | They come from `website/data/repo-facts.json`, a separate recorded snapshot, so the two pages can state sizes measured at different commits | They keep their own writer |
| O-4 | The section table | See "Sections" below; 0.10M of 7.89M lines fall in the catch-all | The twelve sections as listed |
| O-5 | Regenerating `website/data/repo-facts.json` after the inventory correction (AC-14, last clause) | Open. `./le site facts update` reads the working tree. Run on 2026-11-12 after `6b71cb9764`, it warned that 212 Go files differ from the last commit (other sessions' work) and wrote counts that describe no commit, so its output was not committed. The published figures are the same before and after (1.58, 1.86 and 2.00 million) | The committed file stays as it is until the facts are regenerated on a tree that holds no other session's uncommitted Go; the owner decides when |

## Required Reading

Reading is complete; unchecked boxes remain template markers.

### Architecture Docs

- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md`
  → Constraint: a tool is one package under `internal/le/` at the path its command name predicts, registered from `init()` in its own `register.go`, plus one blank import in `internal/le/register.go`. No shipped `ze` links it.
  → Constraint: a bare `go test` omits feature tags; package tests run through `./le test unit pkg path <dir>/`.
- [ ] `docs/architecture/site-facts.md`
  → Constraint: a published number is derived once, by a deliberate command, into a committed file. The build reads that file and never counts. The writer and the checker call one derivation. The check judges a commit, never the working tree.
  → Constraint: every value that changes over time is shown inside a marked element whose text is the last build's value, filled by script from one published JSON file.
  → Decision: the guard `plain-figures` only knows values of `data/site-facts.json`, so it cannot prove a history figure is marked. A dedicated test polices the size view, as `TestEveryHealthFigureIsAMarkedSnapshotValue` does for health.
- [ ] `website/AI.md`, the homepage hero paragraphs and "Presentation tooling"
  → Constraint: the deck embed and the standalone dashboard keep their explicit added-lines and commit calendar. A new design treatment needs owner approval and a matching style-guide update.
  → Decision: the sentence "the activity page itself still measures the current checkout" and the paragraph "No historical category measurements are collected" become false and are rewritten in the phase that changes the page.
- [ ] `ai/META-DECISIONS.md`, "Website design"
  → Constraint: building the website renders data and never measures. Removing useful information needs the owner's agreement. Completion includes regenerating and checking the canonical `../gh-pages` checkout; that authorizes no push.
- [ ] `plan/journal/gate-excludes-part-of-its-population.md` (2026-08-26), `gate-verdict-depends-on-the-machine.md` (2026-08-28, 2026-10-25), `concurrent-session-corruption.md` (2026-09-05)
  → Constraint: a day bound is an explicit UTC instant, never a bare date handed to git. A fixture repository resolves its root through `filepath.EvalSymlinks`. No history walk runs inside the site build or the `internal/le/site` tests. Content comes only from git objects.

**Key insights:**

- Size is an inventory of a committed tree; additions are a flow. The page needs the inventory per day, and the flow only as the day's change.
- A March 2026 history rewrite stamped 1,727 commits with one committer date. The author date is the only clock that still places the first 98 days.
- The whole history is 327 daily trees and about 2.1 GB of unique blobs. It is read once in under two minutes; one day is two trees.
- No path-to-section classification exists anywhere in the tooling. This spec declares the first one, once.

## Current Behavior

**Source files read:**

- [ ] `internal/le/repo/rewrite/activity.go`: `collectActivity` sums per-commit additions by author date over a window, for a list of source extensions. `collectGoStats` and `addGoFile` count tracked Go files of the working tree into production, test and vendor buckets. `classifyGoLine` splits a Go line into code, blank or comment.
- [ ] `internal/le/repo/rewrite/activitymeasure.go`: `MeasureActivity` builds `ActivityWindow`, the calendar cells, the two series and the working-tree Go inventory.
- [ ] `internal/le/repo/rewrite/activityinventory.go`: `MeasureGoInventory`, the inventory entry point the homepage facts use.
- [ ] `internal/le/repo/objects/objects.go`: `Resolve` pins a revision to a commit ID. `Read` returns blobs from one buffered `cat-file --batch` call under a 30-second bound. `takeBlob` validates the length framing.
- [ ] `internal/le/site/activity.go`: `renderActivityPage` measures at build time and publishes the page and its mirror. `activitySummaryHTML` writes four cards that open on "Total added lines". `activityGoPanelHTML` writes the Go composition from the working tree.
- [ ] `internal/le/site/activitygrowth.go`: `activityGrowthScript` sums the calendar cells' additions and commits into two cumulative curves and scales each to its final point. `activityGrowthHTML` adds a "Current Go lines" block from the working tree and the sentence "Historical category totals are unavailable in this data."
- [ ] `internal/le/site/activityscript.go`: `activityPageScript` switches cards and cells between added lines and commits and fills the tooltip.
- [ ] `internal/le/site/activitygrowthstyle.go`: tab, chart and readout styles on the widget's theme roles.
- [ ] `internal/le/site/presentations.go`: `renderActivity` writes the deck's standalone calendar from the same measurement.
- [ ] `internal/le/site/plaintext.go`: `figureMarked` accepts `data-ze-stat` and `data-ze-rfc` only.
- [ ] `internal/le/site/producer.go`: `namedArtifacts` lists published files that are not routes; `checkNamedArtifacts` refuses a missing or empty one.
- [ ] `internal/le/site/facts/register.go`: the `site facts` area registration: `leroot.Register`, `leroot.RegisterActions`, `leroot.RegisterShape`.
- [ ] `internal/le/le/action/kind.go`: the keyword kinds hold no date kind.
- [ ] `internal/le/site/actions.go`: `site activity` takes `today` as a string its action checks against a day layout.
- [ ] `internal/le/verify/engine/stages.go`: `fullStages` declares the ordered stage list and holds `stage("site facts", "check")`.
- [ ] `internal/le/register.go`: one blank import per tool; the `site` areas sit together.

**Behavior to preserve:**

- The route `/project/activity/`, the shared site shell, light and dark themes, keyboard and pointer interaction, the date scrubber, the Markdown mirror.
- The calendar heatmap with its Added Lines and Commits switch, its tooltip, its four cards and the cumulative commits curve.
- The deck embed and the standalone dashboard, with their calendar, their cards and their added-lines semantics. Their Go composition gains only the first-party packages the inventory wrongly dropped.
- The homepage's three line figures and their writer (O-3); their values move by the same correction.

**Behavior to change:**

- The page opens on repository size, read from the committed history file, with the newest recorded day's figures as its headline.
- The cumulative added-lines curve is replaced by size-over-time curves per section and a daily-change chart. The size view selects a date among the file's days, on its own scrubber.
- The view tabs stay visible in both views. The four calendar cards and the Added Lines and Commits switch belong to the Calendar view and show only there; the added-lines total is labelled as lines added by commits, never as a total. Today the tabs, the cards and the switch are one block (`activitySummaryHTML`), which the deck shares without the tabs; the page alone gains the rule that hides cards and switch outside the Calendar view.
- The cumulative commits curve moves under the calendar. It is drawn from the calendar's cells, over the calendar's window, so it stays with them and keeps its own readout. Today a style rule shows it only while the switch says Commits (`body[data-metric]` in `activitygrowthstyle.go`); under the calendar it shows whatever the switch says.
- The Go composition panel, its module count and the "Current Go lines" block state the newest recorded day from the file, no longer the working tree.
- The working-tree Go inventory counts first-party packages whose directory is named `vendor`.

## Existing Mechanisms

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Immutable git object I/O | `internal/le/repo/objects/objects.go`, `Resolve`, `Read`, `takeBlob` | Pins a revision and reads blobs with checked framing, buffered, 30 seconds per call | Extends: a streaming read and a tree listing join the same package and reuse `takeBlob`'s framing checks. `Read` cannot carry 2.1 GB; a private reader in the tool would be a second protocol reader |
| Go line classification | `internal/le/repo/rewrite/activity.go`, `classifyGoLine`, `addGoFile` | Splits Go lines into code, blank and comment for the working tree | Extends: the same classifier counts committed blobs, so the report's three numbers keep one definition. The history tool does not reimplement it |
| Committed-facts pattern | `docs/architecture/site-facts.md`, "The pattern"; `internal/le/site/facts` | One derivation, one committed file, `update` and `check` actions, a verify stage | Extends: `site history` is the second instance, built to the page's seven steps. It differs where the page allows: it reads git objects, so it needs no worktree and no dirty-tree warning |
| Area and action registration | `internal/le/site/facts/register.go`, `leroot.Register`, `leaction.Action`, `leaction.Parameter` | Registers an area, its verbs and typed keywords; help and completion derive from it | Extends by registration: a new area `site history` with `update` and `check`. No list of commands is edited |
| Calendar-day keyword | `internal/le/site/actions.go`, the `today` keyword of `site activity` | A string keyword checked against the day layout by its action | Extends: `from` and `to` follow the same precedent. A new keyword kind for one area would change a shared grammar file for no gain |
| Committed tree listing | `internal/le/repo/inventory/source.go`, `sourceObjects` | Lists a commit's whole tree NUL-framed and keeps the regular files the inventory wants | Extends: the listing moves into `objects` beside `Resolve` and `Read`; `sourceObjects` calls it and keeps its own filter and refusals. A second private lister would be a copy |
| Generated-file lists | `internal/le/repo/generate.go`, `generationActions`, `generationChecks` | `./le repo update` and `./le repo check` run every listed area, `site facts` included | Does not apply: the history is refreshed on purpose before a publication. Listed there, every session's regeneration would rewrite the newest day. Its check runs as a verify stage |
| Marker fill and stray-marker refusal | `internal/le/site/rfcfallback.go`, `fillHTML`, `rfcStrayMarker` | The build fills each marked element from the published file and refuses a marker it cannot reach | Extends the pattern: the history marker gets the same build-time fill and the same refusal of an unknown or misshapen marker |
| Calendar measurement | `internal/le/repo/rewrite/activitymeasure.go`, `MeasureActivity` | Additions and commits per day for the calendar | Does not apply to size: additions are a flow and cannot state an inventory. It keeps serving the calendar, the deck and the dashboard |
| Published data files | `internal/le/site/paths.go`, `sourceOnlyFiles`; `producer.go`, `namedArtifacts` | Every `website/data/*.json` not marked source-only is staged into the artifact's `data/`; named artifacts are checked present | Extends: the history file is staged by the existing rule and joins `namedArtifacts` so its absence fails the artifact check |
| Marked figures with build-time fallback | `internal/le/site/rfcfallback.go`, the `data-ze-rfc` marker; `plaintext.go`, `figureMarked` | A script fills marked elements from a published per-topic file; the build writes the fallback text from the same file | Extends the pattern with a third marker for the history file and a build-time fallback fill |
| The page's inline scripts | `internal/le/site/activityscript.go`, `activityPageScript`; `activitygrowth.go`, `activityGrowthScript` | The page's two scripts are constants of the site package, written into the page | Extends `activityGrowthScript` in place. A separate asset file would split one view's script across two homes |
| Growth view interactions | `internal/le/site/activitygrowth.go`, `activityGrowthScript` | SVG line chart, cursor, slider, pointer and keyboard selection, readout | Replaces the lines series and its scale; keeps the chart treatment, the interactions and the commits curve |
| Verify stage list | `internal/le/verify/engine/stages.go`, `fullStages` | Declares the ordered stages; holds the facts check | Extends: one stage beside the facts check. The list is central by its own design ("declared here rather than discovered at run time") |

## Data Flow

### Entry Point

- `./le site history update` with optional `from <day>`, `to <day>` or `all`, and `./le site history check`. Days are `YYYY-MM-DD`.
- The site build's activity producer, which reads the committed file and nothing else for size.

### Transformation Path

1. **Pin.** Resolve `HEAD` once to a commit ID with `objects.Resolve`. Every later read descends from that ID.
2. **Select days.** List the first-parent ancestry oldest first with each commit's author time, committer time and tree. A commit's day is its author day in UTC, raised to its parent's day when earlier, so days never run backward along the history. A day's commit is the last commit of that day in first-parent order, whatever its timestamp. A day with no commit is not recorded. A commit whose author day is later than its own committer day is refused by name: raised days would carry every later commit onto that future day, and no commit of this history has that shape.
3. **List trees.** For each selected day in the requested range, and for the recorded day before the range, list the tree's regular files with their blob ID. Symbolic links and submodules are not regular files.
4. **Count blobs.** Stream every blob not yet counted in this run. The binary test comes first: a blob with a NUL byte among its first 8,000 bytes (offsets 0 to 7,999, git's own probe) is binary and counts as a file with zero lines, a Go file included. Any other blob counts one line per newline, plus one for an unterminated last line. A Go blob that is not binary is also split into code, blank and comment lines by the shared classifier. Counts are cached by blob ID for the run.
5. **Classify.** Each path falls in exactly one section, by the first matching rule of the declared section table.
6. **Sum.** For each day and section: files and lines, and for Go sections code, blank and comment lines. The day also records how many modules its tree's `vendor/modules.txt` lists. For each path, compare with the previous recorded day: lines a section gained and lines it lost. A path that moved between sections is lost in one and gained in the other.
7. **Verify arithmetic.** For every section and every pair of consecutive recorded days, the newer size equals the older size plus gained minus lost. A violation stops the run before anything is written.
8. **Merge and write.** Days in the requested range replace the file's days; days outside it are kept. The whole merged file must still satisfy step 7. The file is written through a temporary file and a rename, with a fixed key order and one day per line, so the same history writes the same bytes.
9. **Publish.** The site build stages the file into the artifact's `data/`, writes the newest day's figures as fallback text into marked elements, and writes the mirror. The page script fills the markers and draws the charts from the published file.

### Sections

Declared once, in the tool's package, as an ordered table; the first matching
rule wins and the last rule matches everything. The data file carries each
section's identifier and label, so the page and its legend derive from the
file and never from a second list.

| Order | Section | Matches |
|-------|---------|---------|
| 1 | Third-party Go | `.go` files under the third-party trees: `vendor/`, `third_party/` and `website/assets/vendor/`, each from the repository root |
| 2 | Third-party other | every other file under the third-party trees |
| 3 | Go tests | `_test.go` files |
| 4 | Go production | every other `.go` file |
| 5 | Functional tests | files under `test/` |
| 6 | RFC corpus | files under `rfc/` or `reference/` |
| 7 | Plans | files under `plan/` |
| 8 | Agent rules | files under `ai/` or `.claude/`, and the agent instruction files at the root |
| 9 | Documentation | files under `docs/` or `features/`, and Markdown files at the root |
| 10 | Website | files under `website/` or `demos/` |
| 11 | Schemas | `.yang` files |
| 12 | Other | everything else |

The third-party trees are named from the repository root. A directory merely
called `vendor` elsewhere is first-party: `internal/le/web/vendor` is a tool
this project wrote. The working-tree Go inventory (`collectGoStats`) drops
that package today through the exclusion `*/vendor/*`; it takes this table's
third-party rule instead, so the homepage's figures and the history count the
same Go population. The calendar's additions keep their own exclusions.

The rules are applied to every day's tree as that tree was. The file's rules
identifier is a digest of the section table and of a counting-rules version
declared beside the line rule. A test pins the digest, so an edit to the table
fails until the pin is changed on purpose, and a test pins the Go classifier's
counts on a fixed corpus, so a change to the classifier fails with the version
to raise. When the identifier changes, only a whole-history run may write.

### History File Contract

`website/data/repo-history.json`, published as `data/repo-history.json`.

| Field | Meaning |
|-------|---------|
| Schema | Version of this layout |
| Rules | Identifier of the section table and counting rules that measured every day in the file |
| Sections | Ordered identifiers and labels, and which sections carry the Go split |
| Days | Ascending recorded days, every day of the history from its first to the newest recorded, with no gap |
| Day: date | The UTC day |
| Day: commit | The commit whose tree was measured |
| Day: modules | Modules listed by that tree's `vendor/modules.txt` |
| Day: section values | Files, lines, gained, lost; code, blank and comment lines for Go sections |

The file carries no byte sizes and no commit counts: nothing designed here
reads them, and commits per day stay the calendar's measurement.

### Update Semantics

| Invocation | Days measured | Refuses when |
|------------|---------------|--------------|
| No keyword, no file | The whole history | - |
| No keyword, file present | From the newest recorded day, re-measured at that day's current last commit, to the newest day of the history | The file's rules differ, or a kept day's commit left the history: both name `./le site history update all` |
| `from` and/or `to` | Those days inclusive; an open end reaches the first or newest day | As above; no file yet (names `./le site history update all`); a day that is not `YYYY-MM-DD`; `from` after `to`; a range holding no day of the history; a range that would leave a history day unrecorded between the file's days and its own (names `./le site history update`) |
| `all` | The whole history, replacing the file | `all` together with `from` or `to` |

A kept day older than the newest must record the last commit of its day. The
newest recorded day may record an earlier commit of its day: commits made
later that day are lag, and the next default run measures them.

Cases the implementation met that this table had not named, with the repair
each refusal names:

| Case | Repair named | Why |
|------|--------------|-----|
| A range holding no day of the history | `./le site history update` | The default run is the form that always has days to measure |
| A range that overtakes the file's newest day while that day records an earlier commit of its day | `./le site history update` | Re-measuring that day is the narrower repair |
| A file whose newest recorded day is later than the history's newest day | `./le site history update all` | The file describes commits this history does not hold |
| A file that itself skips a history day | `./le site history update all` | A gap inside the file cannot be closed by extending it |
| An object that cannot be read | `git fetch --refetch`, then the same update | The repository, not the file, is incomplete |

The answer states the days measured, the range, each section's lines on the
newest day and the file written.

### Check Semantics

`./le site history check` writes nothing. It refuses, naming the day, the
section and the command that repairs it, when:

| Refusal | What it proves |
|---------|----------------|
| The file does not parse or holds an unknown field | The contract |
| Its rules identifier differs from the tool's | Every day was measured under today's rules |
| A recorded commit is not on `HEAD`'s first-parent history with that day as its day; or a day older than the newest does not record its day's last commit | The file describes this history |
| A history day between the first and the newest recorded day is absent | No gap |
| For some section and consecutive days, size differs from previous size plus gained minus lost; or a Go section's code, blank and comment lines do not sum to its lines | Line arithmetic over every day |
| The newest recorded day, re-measured at its recorded commit, differs from the file in any field | One full measurement |

The newest day is re-measured at the commit the file records, never at the
day's current last commit. The commit that lands the file is itself a later
commit of that day, and it changes the tree by adding the file; a check that
followed the day forward would go red on the commit that satisfied it.
Commits after the recorded one, the same day or later, are reported as lag
and never fail: new commits must not turn a gate red.

The check does not re-measure older days. A full audit is a whole-history
run: its output is deterministic, so any altered figure shows as a diff of
the committed file.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Git objects to tool | Pinned ancestry, NUL-framed tree listings, length-framed streamed blobs | Planned fixture-repository tests |
| Tool to committed file | One writer, one reader, strict decoding, arithmetic check on read | Planned file tests |
| Committed file to site build | The reader of the tool's package; a missing file refuses with the update command | Planned producer tests |
| Build to browser | Staged JSON, marked fallback text, the page's inline script | Planned rendered and browser tests |
| Page to Markdown mirror | The existing `htmlToMarkdown` path | Existing mirror test, expectations judged |

### Changed-Wire Census

**Search evidence:** searches covered Go under `internal/le/site`,
`internal/le/repo/rewrite`, `internal/le/repo/objects` and
`internal/le/verify`, and HTML, JS, JSON, shell and Python carriers under
`internal`, `website`, `test`, `docs` and `.claude/skills`. Terms:
`ActivityWindow`, `MeasureActivity`, `MeasureGoInventory`, `activityGrowth`,
`activity-growth`, `data-lines`, `Cumulative added`, `Historical category`,
`repo-facts`, `data-ze-stat`, `data-ze-rfc`, `namedArtifacts`, `data-metric`,
`fullStages`, `generationActions`. Positive controls: `measureCells` writes
`data-lines` and `activityGrowthScript` reads it; `publishSiteFacts` writes
`data/site-facts.json` and `site.js` reads it. No shell or Python reader of
activity data was found. `website/assets/js/site.js` reads `data-ze-stat`
only (`fillRepoStats`) and needs no change. An independent review on
2026-11-12 re-derived this census and added the summary block, the metric
style rule, the stage population test and the generation lists.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| History file | writer | New `internal/le/site/history`, the `update` action | New wire; the only writer | Created here |
| History file | reader | New `internal/le/site/history`, the `check` action and the shared reader | Same package reads what it writes | Created here; strict decoding |
| History file | reader | `internal/le/site/activity.go`, `renderActivityPage` | Today calls `MeasureActivity` for everything | Reads the file for size; keeps `MeasureActivity` for the calendar |
| History file | reader | `internal/le/site/activitygrowth.go`, `activityGrowthScript` | The page's inline script, today reading calendar cells only | Reads the published file, fills the markers, draws the charts |
| History file | reader | `internal/le/site/producer.go`, `checkNamedArtifacts` | Checks each named artifact is present and not empty | The file joins `namedArtifacts` |
| Activity measurement | writer | `internal/le/repo/rewrite/activitymeasure.go`, `measureActivity` | Builds `ActivityWindow` with cells, series and working-tree Go inventory | Preserved unchanged |
| Activity measurement | reader | `internal/le/site/activity.go`, `activityWidgetHTML`, `activityGoPanelHTML` | Renders cards, calendar and Go composition from the window | Calendar preserved; on the published page the composition reads the history file |
| Activity measurement | reader | `internal/le/site/presentations.go`, `renderActivity` | Deck embed from the same window | Preserved unchanged |
| Activity measurement | reader | `internal/le/repo/rewrite/activity.go`, `renderActivityPage` | Standalone dashboard | Preserved unchanged |
| Calendar attributes | writer | `activitymeasure.go`, `measureCells` | Writes dates, additions and commits per cell | Preserved unchanged |
| Calendar attributes | reader | `internal/le/site/activityscript.go`, `activityPageScript` | Reads both metrics, fills cards and tooltip | Preserved; on the published page the cards show with the calendar only |
| Calendar attributes | reader | `internal/le/site/activitygrowth.go`, `activityGrowthScript` | Sums `data-lines` and `data-commits` into two curves | Stops reading `data-lines`; keeps the commits curve, drawn under the calendar |
| View and metric state | writer | `internal/le/site/activity.go`, `activitySummaryHTML`; `activityscript.go`, `setMetric` | One block holds the tabs, four cards and the switch; `setMetric` writes `body.dataset.metric` | The page marks which view is open; the block's markup stays shared with the deck |
| View and metric state | reader | `internal/le/site/activitygrowthstyle.go`, the `body[data-metric]` rules; `activitygrowth.go`, `selectTab` | Hide the growth element of the other metric; show one view | The metric rules stop governing the commits curve; a page-only rule hides cards and switch outside the Calendar view |
| Marked figures | writer | `internal/le/site/activity.go`, `activitygrowth.go` | Size figures are plain text today | Written inside elements marked `data-ze-history="<section>:<field>"`, the section being one of the file's identifiers or `total`, with the newest day's value as fallback text |
| Marked figures | reader | `internal/le/site/plaintext.go`, `figureMarked` | Accepts two markers | Accepts the history marker as well; its comments naming two markers are corrected |
| Marked figures | reader | New build-time fill in `internal/le/site`, on the model of `rfcfallback.go` | The RFC fill refuses a marker it cannot reach (`rfcStrayMarker`) | Fills each history marker from the file and refuses an unknown section, an unknown field and a misshapen marker |
| Verify stage population | writer | `internal/le/verify/engine/stages.go`, `fullStages` | Declares every stage in order | Gains the history check beside the facts check |
| Verify stage population | reader | `internal/le/verify/engine/verifyengine_test.go`, `TestFullStagesMatchesNativeActionPopulation` | Lists every stage exactly | Gains the same stage |
| Generated-file lists | writer | `internal/le/repo/generate.go`, `generationActions`, `generationChecks` | List the areas `./le repo update` and `./le repo check` run | Unchanged: the history joins neither list |
| Generated-file lists | reader | `internal/le/repo/generate.go`, `runGeneration` | Runs each listed area | Unchanged |
| Public HTML and mirror | writer | `internal/le/site/activity.go`, `renderActivityPage` | `writePublishedPage`, `htmlToMarkdown` | Publishes the size wording and fallback |
| Public HTML and mirror | reader | `internal/le/site/activity_test.go` with `testdata/published-activity-main.html`, `published-activity.md` | Wording parity, numbers masked | Expectations updated through independent judging |
| Public growth view | writer | `internal/le/site/activitygrowth.go`, `activityGrowthHTML` | Writes the tabs, two chart containers, the readout and the working-tree block | Writes the size view: size and daily-change charts, readout per section, the commits curve |
| Public growth view | reader | `internal/le/site/activity_growth_test.go`, `activity_growth_integration_test.go` | Tab ids, snapshot ids, cumulative curves in Chromium | Updated through independent judging; interaction coverage kept |
| Go inventory | writer | `internal/le/repo/rewrite/activityinventory.go`, `MeasureGoInventory` | Counts the working tree's Go files through `collectGoStats`, dropping any path under a directory named `vendor` | Takes the section table's third-party rule; first-party `vendor` packages are counted |
| Go inventory | reader | `internal/le/site/facts/sitefacts.go`, `derive` | Records the homepage's three line facts | Same writer (O-3); `website/data/repo-facts.json` is regenerated once with the corrected population |
| Go inventory | reader | `internal/le/site/presentations.go`, `renderActivity`; `internal/le/repo/rewrite/activity.go`, `renderActivityPage` | Deck and dashboard Go composition | Same reader; the composition gains the corrected population |

### Integration Points

- `objects.Resolve` and the framing checks of `takeBlob`: the tool's every read.
- `classifyGoLine`: the Go split of committed blobs.
- `leroot.Register`, `leroot.RegisterActions`, `leaction.Parameter`: the area, its verbs and its day keywords.
- `registerProducer` and `renderActivityPage`: the same producer keeps the route.
- `namedArtifacts`, `figureMarked`, `activityGrowthScript`: publication, marker and script.
- `fullStages`: the check runs where the facts check runs.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes by design | The page reads the committed file through the tool's reader; the browser reads the published copy. Nothing reads the working tree for size |
| No unintended coupling (components stay isolated) | Yes | Development tooling only. The site package imports the history package's reader; the history package imports neither the site nor any product package |
| No duplicated functionality (extends existing, does not recreate) | Yes | One object reader extended, one Go classifier shared, one committed-facts pattern followed. See Existing Mechanisms |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Offline tooling. The stream holds one blob at a time and the cache holds counts, never blob bodies |
| Registration over hardcoding, outbound | Yes | A new area registers itself; the legend and labels derive from the file's section list |
| Registration over hardcoding, inbound | Yes, with four declared lists | Searched: `internal/le/register.go` (one blank import, the documented cost of a tool), `fullStages` (central by its own design), `namedArtifacts` (the whole population by its own design), `figureMarked` (one more marker). No switch, parser or help string learns a section name |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The author day in UTC, never earlier than the parent's, is the right clock | Measured: 1,727 commits share one committer date; 118 of 16,558 commits have an author day before an ancestor's, by two days at most | The curve's early months are misplaced | `TestDaySelectionFollowsEffectiveAuthorDay`; the recorded first day is 2025-12-19 | Confirmed: the test is green and the real file's first day is 2025-12-19 (`71aa9c7188`) |
| A-2 | The whole history is cheap enough to re-measure | Prototype: 84 seconds, 158,845 blobs | A whole-history run becomes a burden and rule changes get avoided | The first real `update all`, timed on this development VM as a works-or-not check only | Confirmed: the real run completed in minutes, 327 days written |
| A-3 | Twelve sections describe the repository at every point of its history | Prototype: 0.10M of 7.89M lines fall in Other today, 0.03M of 0.05M on the first day | A large unexplained Other section | The Other section's share in the real file, day by day; owner decision O-4 | Confirmed: Other holds 97,579 of 7,905,557 lines on the newest day; the owner confirmed the table |
| A-6 | No commit of this history is authored after it was committed | Measured on 16,558 first-parent commits: none has an author day after its committer day, none an author instant after its committer instant | The refusal of step 2 would stop the tool on a real commit | The first real whole-history run | Confirmed: the real whole-history run refused no commit |
| A-4 | Gained and lost per file between recorded days is what "the additions" means for growth | Owner: "the growth of the codebase NOT the change over time" | He wanted commit additions per section | Put to him as a decision on 2026-11-12 | Confirmed by the owner, 2026-11-12 |
| A-5 | The existing chart treatment carries several series without being a new treatment | `website/AI.md`: a new treatment needs owner approval | A style-guide change and his approval are owed first | His review of the preview before the canonical build | Awaiting his review |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A window or range starts at zero and loses existing content | The first day of a ranged run shows everything as gained | The recorded day before the range is measured as the baseline and never written |
| R-2 | A ranged run leaves the file inconsistent with its neighbours | The arithmetic check fails at the range boundary | The merged file is checked whole before writing; the refusal names `update all` |
| R-3 | Working-tree or another session's uncommitted work enters the file | The same commit measures differently twice | Only git objects under one pinned commit are read; a test dirties the tree |
| R-4 | A missing object or shallow history becomes a plausible zero | A day is absent or a section drops to nothing | Any unreadable object refuses with the cause; nothing is written |
| R-5 | One buffered read exhausts memory or the 30-second bound | The whole-history run fails in the object reader | Blobs are streamed one at a time with no fixed deadline per run |
| R-6 | The history is rewritten after days were recorded | A recorded commit is not on `HEAD`'s first-parent history | `update` and `check` refuse and name `update all` |
| R-7 | The section rules change and old days stay under the old rules | The file's rules identifier differs from the tool's | A ranged run refuses; only `all` writes |
| R-8 | An earlier peak or a shrinking section leaves the chart | A section's maximum is not its last value (Go production shrank in June 2026) | Scales come from every point; the browser test carries a decline |
| R-9 | The file grows until its diffs are noise | A ranged update rewrites untouched lines | One day per line and a fixed order; a ranged run changes only its days |
| R-10 | A site test or the build walks git for size | The site package's tests slow down again | Site tests read a small fixture file; only the tool's tests build fixture repositories |
| R-11 | Concurrent sessions edit the homepage and site files | A prepared diff carries foreign hunks | This spec touches no homepage file; shared site files are committed by hunk |
| R-12 | The public page loses the calendar's information | The wording fixture diff removes a card or a switch | The information-preservation map below is checked row by row at review |
| R-13 | The check goes red on the commit that lands the file, and on every later commit of that day | The verify stage fails right after an update | The check re-measures the recorded commit and reports later commits as lag; a test lands a commit after the update, the same day |
| R-14 | One commit authored far in the future pulls every later commit onto its day | The history stops gaining days | Step 2 refuses a commit authored after it was committed, by name |
| R-15 | A ranged run leaves history days unrecorded between the file and its range | The curve skips days with commits | The file must hold every history day from its first to its newest; update and check both refuse a gap |
| R-16 | Hiding the cards outside the Calendar view hides the tabs or strands the commits curve | The Size view cannot be left, or the commits curve never shows | The tabs sit outside the hidden rule; the commits curve lives under the calendar; the browser test switches views both ways |
| R-17 | The classifier changes and old days keep counts made under the old one | The Go split of an old day differs from a re-measurement | The classifier's counts are pinned on a fixed corpus; a change fails with the rules version to raise |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The public page states wrong sizes or loses its charts. No shipped binary changes |
| How is it reverted? | A forward fix through `./le commit create`; the data file is regenerated by one command |
| Who else touches this path? | Website sessions working the homepage and shared site styles; `spec-structure-checkin` plans to move the rename parser of `activity.go`, which this spec does not touch |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le site history update` through the registered action table | → | The measurement and the file writer | `TestSiteHistoryUpdateActionWritesTheFile` |
| `./le site history check` through the registered action table | → | The reader and the re-measurement | `TestSiteHistoryCheckActionJudgesTheFile` |
| The registered activity producer | → | The history reader and the size view | `TestActivityPagePublishesSizeHistory` |
| The published page in a browser | → | The page script, the markers and the charts | `TestActivitySizeChartFollowsTheHistoryFile` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A fixture history with additions, a same-size rewrite, a deletion and a move between sections, whose per-section files and lines were counted by hand | Each recorded day states exactly those hand-counted files and lines per section; a deletion lowers the size, a rewrite keeps it, a move is lost in one section and gained in the other | Comments, blank lines and tooling stay counted: the figure is repository size |
| AC-2 | A fixture tree with one file for each rule of the section table, a nested first-party `vendor` directory, a symbolic link and a submodule | Each regular file lands in the section the fixture names for it, and the sections' lines sum to the hand-counted total of the tree | The link and the submodule stay out: they are not regular files |
| AC-3 | Empty, unterminated, CRLF and binary blobs; a Go blob with code, blank and comment lines; a Go blob holding a NUL | Each blob counts the lines the fixture states; a binary blob counts as a file with zero lines; the Go blob's code, blank and comment lines are the hand-counted ones and sum to its lines; the Go blob with a NUL counts zero in all four | Binary files stay visible as files, because they are part of the repository |
| AC-4 | Commits authored out of order, in several timezones, with equal timestamps, with a merge on the first-parent line, on a history whose committer dates were rewritten | Each commit lands on its UTC author day or its parent's day when later; each day records its last commit in first-parent order; days without commits are absent; a commit authored after it was committed is refused by name | Committer dates decide no day: the rewrite made them wrong for the early history. They are read only to refuse a commit authored after it was committed |
| AC-5 | For every section and consecutive recorded days of a fixture with known gains and losses | Gained and lost equal the fixture's stated values, and the newer size equals the older size plus gained minus lost, in the tool's output and in every file it writes | No exception: a file that breaks the arithmetic is never written |
| AC-6 | `update` with no keyword, with `from` and/or `to`, and with `all` | The stated days are measured and merged; days outside the range keep their bytes; the file holds every history day from its first to its newest; the same history writes the same file twice | Days before `from` and after `to` survive untouched, because the range is the whole request |
| AC-7 | The measurement of a named commit, taken while the working tree is dirty, holds untracked files, and `HEAD` has moved on | The result equals the measurement of that commit taken on a clean tree before `HEAD` moved | Committed content only; uncommitted work survives in the tree, unread |
| AC-8 | A file under other rules, a rewritten history, a ranged run with no file, a range that would leave a gap, a malformed day, an inverted or empty range, an unreadable object | `update` refuses, names what is wrong, and names the command that repairs it: the default run for a gap, the whole-history run for the others; the committed file is unchanged | The previous file survives every refusal, because a half-written history is worse than a stale one |
| AC-9 | `check` over: a correct file; the same file after more commits the same day and on later days; a newest-day figure altered; a size altered on an older day; a stale rules identifier; a missing day | It passes the first two and reports the later commits as lag; it names the day and section of each altered figure, the stale rules and the missing day, each with its repair command | A file behind `HEAD` survives the check: new commits must not turn a gate red. An older day's files and Go split are proven by a whole-history run, not by the check |
| AC-10 | A visitor opens the activity page, with or without JavaScript | The page opens on repository size: the newest recorded day's total and per-section figures, its date, and what the numbers measure, each figure inside a history-marked element | The calendar, its switch and its four cards stay on the Calendar view, with the commits curve under the calendar; the tabs reach both views |
| AC-11 | Pointer, keyboard, scrubber, section selection, both themes, a 320px viewport, a section that shrank, switching views both ways | The selected day's per-section sizes, gained and lost agree with the file; an earlier peak and a later decline stay inside the chart; both views open from the tabs; focus stays visible | Commit activity stays inspectable on the Calendar view, labelled as activity |
| AC-12 | The site build with the committed file, with the file missing, and with a page marker naming a section the file lacks | The build publishes the page, the mirror and `data/repo-history.json` from the file without measuring; without the file it refuses and names `./le site history update`; it refuses the unknown marker by name | The calendar's own build-time measurement survives: this spec does not own it |
| AC-13 | The deck embed and the standalone dashboard | They render their calendar, cards and switch as before | Their added-lines figures survive, on surfaces that label them as additions |
| AC-14 | The working-tree Go inventory over a tree holding a first-party package in a directory named `vendor` and a root `vendor/` tree | The package's files count as source or test; the root tree counts as third-party; the homepage's recorded facts are regenerated with that population | The calendar's additions keep their own exclusions: they measure activity, not the inventory |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStreamReadsBlobsInRequestOrder` | `internal/le/repo/objects/objects_test.go` | Streamed blobs arrive in order with NUL bytes and newlines intact; a missing object refuses | Planned |
| `TestListAnswersRegularFilesOfATree` | Same | Path, mode and blob ID per entry; links and submodules told apart from regular files | Planned |
| Existing inventory tests | `internal/le/repo/inventory/` | `sourceObjects` answers the same population through the shared listing | Preserve |
| `TestSectionsClassifyEveryPathExactlyOnce` | New `internal/le/site/history/sections_test.go` | Rule order, the catch-all, a nested first-party `vendor` directory, one section per path | Planned |
| `TestRulesIdentifierPinsTheSectionTable` | Same | The identifier's pinned digest; an edited table fails | Planned |
| `TestGoLineCountsArePinned` | New `internal/le/repo/rewrite/golines_test.go` | The classifier's counts on a fixed corpus; a change fails naming the rules version | Planned |
| `TestGoInventoryCountsFirstPartyVendorNamedDirectories` | Existing `internal/le/repo/rewrite/sourcerewrite_test.go` | AC-14 | Planned |
| `TestDaySelectionFollowsEffectiveAuthorDay` | New `internal/le/site/history/days_test.go` | UTC day, out-of-order authors, equal timestamps, a first-parent merge, rewritten committer dates, absent idle days | Planned |
| `TestDaySelectionRefusesACommitAuthoredAfterItWasCommitted` | Same | The refusal names the commit | Planned |
| `TestMeasureCountsCommittedTreeBySection` | New `internal/le/site/history/measure_test.go` | Add, rewrite, delete and move across sections against hand-counted files, lines, gained and lost; the module count | Planned |
| `TestMeasureCountsLineBoundaries` | Same | Empty, unterminated, CRLF, binary, a Go blob with a NUL; the Go split against hand-counted values | Planned |
| `TestMeasureReadsOnlyCommittedObjects` | Same | A named commit measured on a dirty tree after `HEAD` moved equals its clean measurement | Planned |
| `TestMeasureRefusesUnreadableHistory` | Same | A missing object or shallow history writes nothing | Planned |
| `TestUpdateWholeHistoryWritesTheSameBytesTwice` | New `internal/le/site/history/update_test.go` | Determinism, one day per line | Planned |
| `TestUpdateRangeReplacesOnlyNamedDays` | Same | `from`, `to`, open ends, baseline day, untouched neighbours | Planned |
| `TestUpdateDefaultExtendsFromTheNewestRecordedDay` | Same | New commits append days; the newest recorded day is re-measured | Planned |
| `TestUpdateRefusesWhatItCannotMerge` | Same | Other rules, rewritten history, a range with no file, a range leaving a gap, bad day, inverted range, empty range, `all` with a bound; each names its repair | Planned |
| `TestReadRefusesMalformedFile` | New `internal/le/site/history/file_test.go` | Unknown field, unordered days, broken arithmetic | Planned |
| `TestSiteHistoryUpdateActionWritesTheFile` | New `internal/le/site/history/actions_test.go` | The registered `update` action reaches the writer | Planned |
| `TestSiteHistoryCheckActionJudgesTheFile` | Same | Each row of AC-9 | Planned |
| `TestCheckAcceptsLaterCommitsOnTheNewestDay` | New `internal/le/site/history/check_test.go` | A commit that adds the file itself, authored the same day, leaves the check green and is reported as lag | Planned |
| `TestFullStagesMatchesNativeActionPopulation` | Existing `internal/le/verify/engine/verifyengine_test.go` | The stage list holds the history check | Update: a stage is added, nothing removed |
| `TestHistoryFillRefusesUnknownMarker` | New `internal/le/site/activity_history_test.go` | An unknown section, an unknown field and a misshapen marker each refuse by name | Planned |
| `TestActivityPagePublishesSizeHistory` | New `internal/le/site/activity_history_test.go` | Producer reads a fixture file; size view is the default; marked figures equal the fixture; the data file is published | Planned |
| `TestActivityPageRefusesMissingHistory` | Same | The refusal names the update command | Planned |
| `TestEveryActivitySizeFigureIsAHistoryMarkedValue` | Same | No number in the size view outside a history marker | Planned |
| `TestActivityPageKeepsCalendarAndCommitViews` | Same | Cells, metric switch, four cards and the commits curve remain on the Calendar view; the tabs sit outside the block hidden with it | Planned |
| `TestHistoryMarkedFigureIsNotPlain` | `internal/le/site/plaintext_test.go` | The guard accepts the history marker | Planned |
| `TestActivityPageOffersGrowthRepresentation` | Existing `internal/le/site/activity_growth_test.go` | Tabs and the size view's identifiers | Update with judged expectations |
| `TestTheActivityPageReadsAsThePublishedPage`, `TestTheActivityMirrorReadsAsThePublishedMirror` | Existing `internal/le/site/activity_test.go` | Wording of page and mirror | Fixtures updated with the judge |
| `TestActivitySlideKeepsCalendarRepresentation`, `TestRenderActivityStopsAtToday`, `TestRenderActivityEmbedsHeatmap`, `TestActivityDashboardFixture` | Existing | Deck and dashboard unchanged | Preserve |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `from`, `to` | `YYYY-MM-DD` | The first and the newest day of the history | A range wholly before the first commit: refused as holding no day | A range wholly after the newest commit: refused as holding no day |
| Range | `from` not after `to` | `from` equal to `to` | `from` after `to`: refused | A range starting more than one history day after the newest recorded day: refused as a gap |
| Commit time | A commit at 23:59:59 UTC | That day | - | A commit at 00:00:00 UTC belongs to the next day |
| Author against committer time | Author day not after committer day | Author and committer on the same day | - | Author day one day after committer day: refused |
| Blob lines | Zero to the blob's content | One unterminated line counts one; a trailing newline adds none | Negative impossible | Truncated framing refuses |
| Binary probe | Offsets 0 to 7,999 | A NUL at offset 7,999 is binary | - | A NUL only at offset 8,000 is text |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestActivitySizeChartFollowsTheHistoryFile` | `internal/le/site/activity_growth_integration_test.go`, replacing `TestActivityGrowthUsesExistingDays` | Chromium draws the fixture file: several sections, one declining, date selection by pointer, keyboard and scrubber, section selection, both themes, 320px | Planned; the replacement is independently judged |
| The first whole-history run | `./le site history update all` in the main checkout, then `./le site history check` | The real history writes the committed file and the check passes it | Main-thread execution owed |
| Canonical build and check | `./le site build`, `./le site check` in the canonical `../gh-pages` checkout | The real page, mirror and data file publish; desktop and mobile, light and dark inspected | Main-thread execution owed |

No functional test file under `test/`: the tool is a `le` action with no
daemon, RPC or protocol behind it, and its registered action table is
exercised directly by the two action tests.

## Files to Modify

- `internal/le/repo/objects/objects.go`: a streaming blob read and a tree listing, sharing the existing framing checks.
- `internal/le/repo/inventory/source.go`: `sourceObjects` calls the shared tree listing and keeps its filter and refusals.
- `internal/le/repo/rewrite/activity.go`: the Go line classifier offered to the history package over blob bytes, with `addGoFile` using the same counting; `collectGoStats` takes the section table's third-party rule. No change to the calendar's additions.
- `website/data/repo-facts.json`: regenerated once by `./le site facts update` after the inventory correction.
- `internal/le/register.go`: the blank import of the new area.
- `internal/le/verify/engine/verifyengine_test.go`: the stage population gains the history check.
- `internal/le/site/actions.go`: the comment on the plain-figures report names the markers.
- `internal/le/site/activity.go`: read the history file, default size view, marked figures, calendar-scoped cards, composition from the file, description and hero sentence, mirror.
- `internal/le/site/activitygrowth.go`, `activitygrowthstyle.go`, `activityscript.go`: size view markup, script and styles; the commits curve kept; the lines curve and its working-tree block replaced.
- `internal/le/site/plaintext.go`: `figureMarked` accepts the history marker; the comments that say "two markers" are corrected.
- `internal/le/site/producer.go`: the history file joins `namedArtifacts`.
- `internal/le/verify/engine/stages.go`: the history check beside the facts check.
- `internal/le/site/activity_test.go`, `activity_growth_test.go`, `activity_growth_integration_test.go`, `internal/le/site/testdata/published-activity-main.html`, `published-activity.md`: judged expectations.
- `website/AI.md`: in the homepage paragraph, "using the activity page's Go inventory definition" and "the activity page itself still measures the current checkout"; in "Presentation tooling", the deck "draws the same measurement as `/project/activity/`" and the Growth paragraph.
- `docs/architecture/site-facts.md`: a "Size history" section; the sentence "The activity page reads a live checkout inventory"; "One published copy of each value" names the history file and its marker.
- `docs/contributing/gh-pages.md`, `ai/skills/ze-weekly-update.md`: the history update beside the facts update, before the build. The skill's source is the file under `ai/skills/`; the copy under `.claude/skills/` is generated and untracked.
- `docs/contributing/spec-workflow.md`: the paragraph on shared git object readers names the streaming read and the tree listing.
- `website/style-guide/index.html`: the multi-series chart, if the owner judges it a new treatment (A-5).
- `ai/INDEX.md`: the keyword row for repository size history.

## Files to Create

- `internal/le/site/history/register.go`, `actions.go`: the area, its two verbs and their keywords.
- `internal/le/site/history/sections.go`, `days.go`, `measure.go`, `file.go`, `update.go`, `check.go`: section table, day selection, counting, file contract, merge, check.
- `internal/le/site/history/*_test.go`: the tests named above, over small fixture repositories.
- `internal/le/site/activity_history_test.go` and a small `internal/le/site/testdata/repo-history.json` fixture.
- `website/data/repo-history.json`: written by the first whole-history run, never by hand.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | A development tool; no product RPC or config |
| YANG validation constraints | N-A | No YANG leaf |
| YANG custom validators | N-A | No config validator |
| CLI commands/flags | Yes | `internal/le/site/history/actions.go`: `update` with `from`, `to` and `all`; `check` with no keyword |
| CLI grammar (keyword before value) | Yes | Keywords precede their values through `leaction.Parameter`; `./le cli grammar` judges the area |
| Editor autocomplete | N-A | No editor surface; le completion derives from the registration |
| Functional test for new RPC/API | N-A | No endpoint; the action tests exercise the registered table |
| Pipe completeness | Yes | The answer is a struct rendered by the shared renderer, with `leroot.RegisterShape` |
| Env var registration | N-A | No setting |
| Doctor check for runtime dependencies | No | Git is already le's dependency; nothing ships |
| Doctor code per problem class | N-A | Tool refusals reach the developer at the command; no daemon state |
| Prometheus counters/metrics | N-A | Static website data |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol behavior |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Website project history, no product feature |
| 2 | Config syntax changed? | N-A | No configuration |
| 3 | CLI command added/changed? | Yes | A `le` command: `le help` derives from the registration; `docs/contributing/gh-pages.md` names it. The product command reference does not list le |
| 4 | API/RPC added/changed? | N-A | No product endpoint |
| 5 | Plugin added/changed? | N-A | No plugin |
| 6 | Has a user guide page? | Yes | `docs/contributing/gh-pages.md`: when to run the update, and the page's default view |
| 7 | Wire format changed? | Yes | `docs/architecture/site-facts.md`: the history file's contract; no product wire |
| 8 | Plugin SDK/protocol changed? | N-A | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC behavior |
| 10 | Test infrastructure changed? | No | Existing Go and Chromium harnesses |
| 11 | Affects daemon comparison? | N-A | No comparison |
| 12 | Internal architecture changed? | Yes | `docs/architecture/site-facts.md`, `website/AI.md`: the second committed-facts instance and the page's data source |
| 13 | Route metadata keys added/changed? | N-A | None |
| 14 | Prometheus counters added/changed? | N-A | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | A le area only; `ai/INDEX.md` gains its keyword row |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `website/AI.md` anchors `activitygrowth.go`; `docs/architecture/site-facts.md` anchors `activity.go` (`collectGoStats`). `docs/contributing/spec-workflow.md`, declared by `objects.go`, describes the shared git object readers and gains the streaming read and the tree listing. `docs/architecture/testing/verify-freshness-scope.md`, declared by `stages.go`, is unaffected: it describes how the stage population is fixed and names no site stage. `docs/architecture/core-design.md`, declared by the `register.go` files, is unaffected: the new area follows its one-import-per-tool composition. `docs/architecture/plugin/feature-gates.md`, declared by `internal/le/repo/inventory/source.go`, is unaffected: the committed inventory keeps its population and its refusals, and only the tree listing it calls moves into `objects`. The implementing phase runs `./le spec citation anchors` for this spec and names any further page |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/gh-pages.md` and the weekly-update skill show the site data commands; the new command joins them |

### Mechanical Discovery Checklist

| Need | Path and action |
|------|-----------------|
| First lookup | `ai/INDEX.md` gains a row for repository size history pointing at `docs/architecture/site-facts.md` |
| Regression contract | `docs/architecture/site-facts.md` states the counting rules; `./le site history check` enforces the file |
| Inventory | The area registration; the section table in one file; the data file's own section list |
| Proof | Fixture-repository tests, the action tests, the producer and browser tests, the real whole-history run and the canonical artifact check |

### Information-preservation map

| Item on the page today | Source today | After this spec |
|------------------------|--------------|-----------------|
| Calendar heatmap, Added Lines and Commits switch, tooltip, legend, range | Build-time calendar measurement | Kept, on the Calendar view |
| View tabs | Static | Kept, visible in both views |
| Four cards: total added lines, days with added lines, peak day, days shown | Same | Kept, shown with the Calendar view; the first is labelled as lines added by commits |
| Go code composition: total, production, test, dependencies with files and code, blank, comment lines; module count | Working tree | Kept; every value, the module count included, from the newest recorded day, with its date |
| Cumulative added lines curve | Sum of calendar cells | Replaced by size per section over time and a daily-change chart (approved 2026-11-12) |
| Cumulative commits curve | Sum of calendar cells | Kept, moved under the calendar it is drawn from, shown whatever the metric switch says |
| "Current Go lines": code, tests, third party, overall | Working tree | Replaced by the newest recorded day's per-section figures, which include those four |
| Sentence "Added lines count additions, not net repository size" | Static | Kept with the calendar's added-lines card |
| Sentence "Historical category totals are unavailable in this data" | Static | Removed: it becomes false |
| Journey and Trust checks links | Static | Kept |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the area, its two verbs and their keywords registered; the streaming read and tree listing in `objects`
   - Tests: `TestSiteHistoryUpdateActionWritesTheFile`, `TestSiteHistoryCheckActionJudgesTheFile`, `TestStreamReadsBlobsInRequestOrder`, `TestListAnswersRegularFilesOfATree`
   - Files: `internal/le/site/history/register.go`, `actions.go`, `internal/le/register.go`, `internal/le/repo/objects/objects.go`
   - Verify: `./le help site history` lists both verbs; the action tests fail because nothing measures yet
2. **Phase: Measurement and file** -- sections, day selection, counting, gained and lost, the file contract, update semantics and refusals
   - Tests: the `sections`, `days`, `measure`, `update` and `file` tests of the plan
   - Files: the history package; `internal/le/repo/rewrite/activity.go` for the shared classifier and the inventory's third-party rule; `internal/le/repo/inventory/source.go` for the shared listing
   - Verify: tests fail, implement, tests pass. Then the main thread runs `./le site history update all` and `./le site facts update` once each and commits `website/data/repo-history.json` and `website/data/repo-facts.json`. This phase lands as its own commit
3. **Phase: Check** -- the check action and its verify stage
   - Tests: `TestSiteHistoryCheckActionJudgesTheFile` in full, `TestCheckAcceptsLaterCommitsOnTheNewestDay`, `TestFullStagesMatchesNativeActionPopulation`
   - Files: `internal/le/site/history/check.go`, `internal/le/verify/engine/stages.go`, `internal/le/verify/engine/verifyengine_test.go`, `docs/architecture/site-facts.md`
   - Verify: the check passes the committed file on the commit that carries it and on the commits after it, and refuses a tampered copy. Own commit
4. **Phase: Page** -- the producer reads the file; size view, markers, page script, calendar-scoped cards, composition from the file, mirror
   - Tests: `TestActivityPagePublishesSizeHistory`, `TestActivityPageRefusesMissingHistory`, `TestEveryActivitySizeFigureIsAHistoryMarkedValue`, `TestActivityPageKeepsCalendarAndCommitViews`, `TestHistoryMarkedFigureIsNotPlain`, `TestActivitySizeChartFollowsTheHistoryFile`; judged updates of the existing wording, growth and browser tests
   - Files: the site files of "Files to Modify", `website/AI.md`, `docs/contributing/gh-pages.md`, the weekly-update skill, `ai/INDEX.md`
   - Verify: tests fail, implement, tests pass; the deck and dashboard tests stay green untouched
5. **Phase: Canonical artifact** -- regenerate and check `../gh-pages`, inspect the page
   - Files: none in this repository unless inspection finds a defect
   - Verify: `./le site build` and `./le site check` pass in the canonical checkout; the page is inspected at desktop and 320px, light and dark, with keyboard and without JavaScript. Nothing is pushed

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has an implementation and a named test |
| Meaning | Every size figure on the public page comes from the history file; no figure labelled as a size is a sum of additions |
| Arithmetic | Size equals previous size plus gained minus lost for every section and day, in the tool, the reader and the check |
| Days | UTC author day, never earlier than the parent's; the last commit of a day in first-parent order; the committer date read only to refuse a commit authored after it |
| Pin | Ancestry, trees and blobs all descend from one resolved commit; nothing reads the working tree, the module count included |
| Range | A ranged run changes only its days, leaves no gap, and a run that cannot merge writes nothing |
| Check | Green on the commit that lands the file and on every later commit; red only for a file that does not describe this history |
| One declaration | The section table exists once; the page's labels come from the file; the Go inventory takes its third-party rule from the table |
| Preservation | Each row of the information-preservation map holds on the rendered page; the tabs open both views |
| Legacy | The deck and the dashboard keep their markup and their calendar figures; only their Go composition moves, by the corrected population |
| Operator errors | Every refusal names what is wrong and the exact command that repairs it, last line |
| Rule: `ai/rules/no-layering.md` | The cumulative added-lines curve and the working-tree block are deleted from the public page, not kept behind the new view |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The `site history` area with `update` and `check` | `./le help site history`; `./le test unit pkg path ./internal/le/site/history/` |
| The committed history file | `./le site history check` passes on the commit that carries it |
| The size view on the public page | `./le test unit pkg path ./internal/le/site/ run 'TestActivity'`; the integration-tagged browser test |
| The canonical artifact | `./le site build` and `./le site check` in `../gh-pages` |
| Preserved deck and dashboard | Their existing tests, unchanged and green |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Git arguments | The revision is resolved once by the option-safe helper; only full object IDs reach git afterwards; paths are read NUL-framed and never passed back as arguments |
| Object framing | The streaming read validates ID, type and length as `takeBlob` does; a mismatch refuses |
| Resource bounds | One blob in memory at a time; the cache holds integers; no checkout or copy of the tree |
| File write | Temporary file and rename inside `website/data/`; a refusal leaves the committed file untouched |
| Browser injection | Section labels and figures enter the page as text content; the embedded and published JSON is encoded, never concatenated |
| Day keywords | Parsed against one layout before any use; never interpolated into a git argument as free text |

### Failure Routing

| Failure | Route To |
|---------|----------|
| The arithmetic check fails on the real history | Trace the path and day it names to the counting rule; fix the producer, never relax the check |
| The real whole-history run disagrees with the prototype table | Read both counting rules against the same blob; the tool's tests decide, the prototype is evidence only |
| A wording or browser fixture must change | The independent `ze-test-judge` agent judges the change before it lands |
| The owner judges the chart a new treatment | Stop the page phase at the preview; the style-guide update and his approval come first |
| WIP admission is refused | The spec stays `ready`; no claim is forced (O-1) |
| A foreign edit breaks the site package's build | Report it and wait; no copy of the tree is used |
| 3 fix attempts failed | Stop, report the three approaches, ask which way to fix it |

## Design Insights

### What implementation added to the design

| Where | What | Why |
|-------|------|-----|
| Check | One more refusal: the file's section list differs from the tool's while the rules identifier matches | The identifier digests the table, so this is a hand-edited file; it names `./le site history update all` |
| Check | It reads the data file from the checkout, not from `HEAD`'s tree | A developer checks an update before committing it; under the verify stage the worktree is the commit, so the judged file is the commit's own |
| Check | The default `./le site history update` is named in the answer's lag line, never as a refusal | Lag never fails |
| Check | An absent file answers 1 with the whole-history run as its repair; a file that cannot be read answers 2 | "Not recorded" is a verdict; "could not look" is not |
| Page marker | A subject may sum sections (`a+b`), and the all-Go figure sums every section the file marks with the Go split | First-party Go is production plus tests; all Go adds third-party; the file carries no first-party flag |
| Page | The Go composition's four buckets name their section identifiers in the page code | The section table exports no way to ask for one particular section; an unknown identifier refuses the build by name |
| Page | The mirror test compares headings and bullet labels only, so it cannot see a dropped paragraph | A judge noted it; the size total and the all-Go sentence are asserted by `TestActivityPagePublishesSizeHistory` instead |
| Writer | The data file is written world-readable | The temporary file's owner-only mode reached the committed file's first write; other site data files are readable by all |
| Dashboard | The standalone dashboard's two notes now name the third-party trees | The old wording said "outside vendor/", which the inventory correction made false |
| First real run | 327 days, 2025-12-19 to 2026-11-12, 7,905,557 lines on the newest day, measured at `9e38af2dd7` | Committed as `71aa9c7188` |
| Browser test | Two judges refused the replacement before a third ruling approved it | It had dropped the proof that the commits curve is drawn, the charts' `viewBox`, the pointer on the commits readout, and the tab's selected state after a keyboard switch; all four are asserted now |

- The two "five million" figures were different measurements that happen to be close: 5.8M lines added by commits in a year, and 5.4M Go lines in the tree with third-party code.
- Third-party code is the largest single section (2.31M of 7.89M lines) and arrived in two steps. A total that hides the split would tell the wrong story about the project's own growth.
- Committer dates cannot place the first quarter of this history; a design that trusted them would have opened the curve with a cliff on 2026-03-27.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `gate-excludes-part-of-its-population` | 2026-11-12 | The Go inventory drops the first-party package `internal/le/web/vendor` through the exclusion `*/vendor/*` |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A ranged command writes a committed file; the build only reads it | Measure inside the site build, as the first design did | The owner asked for a tool usable over any period; the build must not measure (`ai/META-DECISIONS.md`); the whole history is 2.1 GB of blobs |
| Count each recorded day's exact tree | Replay per-commit additions and deletions from the first commit | A tree inventory needs no diff parser, no rename handling and no trust in git's text detection; the first design reached the same decision |
| Gained and lost per file between recorded days | Per-commit additions and deletions per section from `git log --numstat` | One measurement, exact arithmetic by construction, and it answers growth. Commit additions remain on the calendar, labelled as activity |
| The author day in UTC, never earlier than the parent's | The committer day; the author's local day | Committer dates were rewritten in March 2026; commits were authored in five timezones |
| One ordered section table with a catch-all, carried in the file | Sections by file extension only; a list repeated in the page script | Paths state purpose where extensions cannot (a Markdown file is a plan, a rule or a page); one declaration serves the tool, the file and the page |
| Day keywords as checked strings | A new keyword kind for dates | `site activity` already sets the precedent; a kind is a change to the shared grammar for one area |
| A third figure marker for the history file | Copy the newest day into `data/site-facts.json` | A copy would publish one value in two files, which the one-file rule forbids |
| The size view extends the page's inline growth script | A separate script file under the website's assets | The page's two scripts are inline constants of the site package; one view keeps one script |
| The check never fails for age | Fail when the file is behind `HEAD` | Every commit would redden the gate for every session |
| The check re-measures the recorded commit of the newest day | Re-measure that day's current last commit | The commit that lands the file is a later commit of the same day and changes the tree; the gate would go red on the commit that satisfied it |
| The file must hold every history day up to its newest | Let a ranged run write any days | A gap passes the arithmetic and silently drops days from the curve |
| Refuse a commit authored after it was committed | Accept it and raise later days onto it | One such commit would freeze the history on a future day; none exists today |
| No byte sizes, no commit counts, no `day` keyword on the check | Record and offer them | The owner asked for lines per section over time; nothing designed reads the rest, and commits per day already have one definition, the calendar's |
| The commits curve lives under the calendar | Keep it in the size view on a shared scrubber | It is drawn from the calendar's cells, over the calendar's window and its author-local days; the size view's days come from the file |
| The Go inventory takes the table's third-party rule | Leave `collectGoStats` as it is and state the difference | Two counts of Go lines on one site must name one population; the old exclusion dropped first-party code |
| The history stays out of `./le repo update` | List it with the other generated files | Every session's regeneration would rewrite the newest day of a file meant to be refreshed on purpose |
| The homepage keeps its own figures | Read them from the history's newest day | Scope the owner has not decided (O-3) |

### Design review, 2026-11-12

An independent read-only review judged the first draft of this design against
source and found one blocker and seven issues. Each changed the design:

| Finding | Change |
|---------|--------|
| The check went red on the commit landing the file | Check Semantics; R-13; AC-9 |
| A future author date swallowed the history | Step 2; A-6; R-14; AC-4 |
| A ranged run could leave gaps and named the wrong repair | Update Semantics; R-15; AC-6, AC-8 |
| The arithmetic could not back "a tampered figure" | AC-9 states what the check proves; a whole-history run is the audit |
| The cards, the tabs and the commits curve were coupled | Behavior to change; census "View and metric state"; R-16; AC-10, AC-11 |
| The census missed the stage test, the generation lists, the inventory's tree lister and the marker's fill | Existing Mechanisms; census; Files to Modify |
| The history's Go sections were not the inventory's population | Sections; AC-14; owned journal row |
| Vacuous sums and an unnamed seam | AC-1, AC-2, AC-3, AC-5 pin hand-counted values; AC-7 names the commit |

### Design Admission Challenge

| Challenge | Answer |
|-----------|--------|
| Simplicity | One inventory per recorded day, one file, two verbs. No diff replay, no persistent cache, no second measurement for additions |
| Uniformity | The second instance of the committed-facts pattern, the existing registration, the existing chart treatment and the marker-and-fallback pattern of the RFC figures |
| Performance | Offline tooling. Work is bounded by distinct daily trees and unique blobs; one blob in memory at a time. The 84-second prototype is a works-or-not observation on this VM, not a reported figure |
| Strongest concern | The page phase: several series in the existing chart treatment may read as a new treatment and need the owner's approval and a style-guide entry before it can land (A-5) |

## Known Limitations

- The record follows `HEAD`'s first-parent history. Work on other branches is not measured until it lands.
- A day is a UTC day by author time. The calendar beside it still attributes commits by the author's local date, so a commit near midnight can sit on adjacent days in the two views.
- Lines include comments and blank lines; binary files count as files with zero lines.
- Commits that reach the history through a merge's second parent are measured in the merge's tree; they never select a day of their own.
- The check proves the newest recorded day in full and the line arithmetic of every day. An older day's file counts and Go split are proven by a whole-history run.
- The calendar still measures at build time. This spec neither adds to that cost nor removes it.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test unit pkg path ./internal/le/site/history/`, `./le test unit pkg path ./internal/le/repo/objects/`, `./le test unit pkg path ./internal/le/repo/inventory/`, `./le test unit pkg path ./internal/le/repo/rewrite/`, `./le test unit pkg path ./internal/le/verify/engine/`, `./le test unit pkg path ./internal/le/site/` and the integration-tagged browser test. GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional tests for end-to-end behavior (N-A under `test/`: a `le` action with no daemon behind it; the action and browser tests cover the path)
- [ ] Interop tests for protocol features (N-A: no protocol)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-activity-codebase-growth.md` only, in the same `./le commit create` script
