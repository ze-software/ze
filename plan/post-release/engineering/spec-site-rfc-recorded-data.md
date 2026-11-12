# Spec: site-rfc-recorded-data

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P3 |
| Release reason | Website build tooling: the site build re-runs the RFC check to render a page, against the owner rule that building the website renders data. It changes no shipped binary |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The owner's words, which are the design:

> "We should generate a metadata about the rfc, the data the website needs and
> regenerate it if the data is over a week old. Use the data to generate the
> page. We should have a command to allow to regenerate it so I can update it
> when needed."

> "The data can be saved with the website itself. The week is measured from the
> timestamp of the file saved."

The problem. The RFC compliance page shows the RFC gate's verdict and its open
issues. To get them, every `./le site build` calls `rfc.Check`, the same check
as `./le rfc check`: it type-checks the test code of every package carrying an
RFC tag under a fifteen-minute bound, reads the tag corpus from git at three
revisions, and re-verifies the discrimination records. A build whose only change
is one sentence on the front page therefore re-runs a check over the code. That
contradicts the owner rule in `ai/META-DECISIONS.md`, "Website design":
"Building the website renders data; it never runs a check." The bullet under it
records this decision: what the website shows about RFC conformance comes from a
recorded data file, the pages are generated from that file, the build does not
run the RFC check to render a page, it regenerates the file only when the file is
more than a week old, and a command regenerates it on demand.

In this spec, "the published artifact" is the gh-pages checkout beside the main
checkout, which `./le site build` writes and reads back (`lastPublished`).

The goal, in the owner's terms:

| Goal | What it means |
|------|---------------|
| G-1 | One recorded RFC data file holds what the website needs about RFCs, and it is saved with the website |
| G-2 | Every RFC page and every RFC figure on the site is generated from that recording |
| G-3 | The build regenerates the recording only when its saved timestamp is more than a week old, or when it is absent |
| G-4 | One command regenerates it on demand, whatever its age |
| G-5 | The page states the date of its data, because under an age-only rule it can lag the code by up to a week |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md` - the doors for `internal/le`
  → Constraint: an le tool registers its actions from `init()`; a command's package sits at the path its name predicts (`TestEveryCommandIsFoundAtThePathItsNamePredicts`), a space is a directory level and a hyphen joins one name
  → Constraint: help, completion and `le help ai` derive from the action table, so a new action needs no edit to any help string
- [ ] `docs/architecture/site-facts.md` - "The pattern", "Not every fact can be committed", "Applying this to another generated file", "One published copy of each value", "RFC completion data"
  → Decision: a published number is derived once, by a command, into a data file, and everything downstream reads that file. This spec applies that pattern to the RFC data, with the owner's age rule added
  → Constraint: every value that changes over time lives in exactly one published JSON file (`ai/META-DECISIONS.md`, "Website design"); the recording MUST NOT publish a second copy of a value the published RFC data files or the site facts already carry
  → Constraint: the category table row "Already gated", which today says the RFC requirement counts have no copy, becomes wrong and is edited in the same change
- [ ] `docs/contributing/gh-pages.md`, "Quality pages" - says the RFC compliance report reads `Collect`, `NewRenderInput` and `Check` on every build
  → Constraint: that paragraph becomes wrong and is edited in the same change
- [ ] `website/AI.md`, "The RFC requirement ledger" - says the ledger is derived once per build
  → Constraint: becomes wrong and is edited in the same change
- [ ] `docs/contributing/rfc-conformance-gates.md`, the paragraph after the requirement table (the published detail pages, "`internal/le/site/rfcledger.go` derives it")
  → Constraint: "the state of every stored proof re-verified against the tree" becomes "as of the recording", so the page is edited
- [ ] `ai/rules/cli.md` - the action verb vocabulary, `list`, the explicit-selector rule
  → Constraint: the regeneration command acts on one recording, not on a set, so the explicit-selector rule does not bind it
- [ ] `ai/rules/user-facing-errors.md` - every message says what is wrong, why, and the command that fixes it
  → Constraint: the regeneration notice and every refusal of the recording name the regeneration command

**Key insights:**
- The build already reads the last published artifact back (`lastPublished`, `snapshotArtifact`, `seedArtifact` in `internal/le/site/build.go`), so data saved with the published website reaches the next build without a new carrier.
- The site facts snapshot is the one existing site data file that records when it was produced (`generated_at`, `published_at`, `deriveSiteFacts` in `internal/le/site/facts.go`). `website/data/repo-facts.json`, `website/data/github-releases.json` and `test/health/latest.json` record no production time; `website/data/repo-history.json` records the commit of each measured day. None of them has an age rule.
- The heavy part is not `rfc.Check` alone: `rfc.Collect` plus `rfc.NewRenderInput`, which re-verifies every stored proof, runs eight times per full build (Current Behavior).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/site/actions.go` - `runBuild`: runs the feature-freshness pass (`freshness`, over `feature.StaleSupported`) before `Build`; a `producer` build (`runProducerBuild`) skips it
- [ ] `internal/le/site/build.go` - `Build`: `derived.EnsureAll`, snapshot and seed from the published artifact, stage sources, `refreshNativeSurfaces` (which calls `publishRFCLedger`, `publishRFCData`, `publishSiteFacts`), then `renderProducers`, `writeProducerRecord`; `buildPaths` shares one test-health render per build through `sync.OnceValues` (`Paths.testHealth`)
- [ ] `internal/le/site/rfccompliance.go` - `collectRFCCompliance`: `rfc.Collect`, `rfc.NewRenderInput`, then `rfc.Check(tree, nil)`; builds the `rfcCompliance` snapshot (gate verdict, violations, findings, satisfaction buckets, audit counts); `renderRFCCompliance` writes the compliance snapshot (`rfcComplianceSnapshot`), the aggregate page and one detail page per stem, reading the ledger and the figure set back from the output
- [ ] `internal/le/site/rfcledger.go` - `collectRequirementLedger`: `rfc.Collect`, `rfc.NewRenderInput`, `rfc.LoadExtractions`, `rfc.PriorityOf`; writes the requirement ledger (`rfcLedgerFile`), 22,860,346 bytes in the published artifact today
- [ ] `internal/le/site/rfcdata.go` - `collectRFCData`: `rfc.Collect`, `rfc.NewRenderInput`; `writeRFCData` replaces the per-RFC and per-profile directory (`rfcDataDir`), 279 files and about 252 KB today
- [ ] `internal/le/site/facts.go` - `factsFromRFCLedger`: `liveRFCLedger` (`rfc.Collect`) and `liveStemFigures` (`rfc.NewRenderInput`), the RFC keys of the site facts snapshot
- [ ] `internal/le/site/docs.go` - `liveDocSources`: the docs producer asks `rfc.StatusPage` for `docs/features/rfc-status.md`; `rfc.StatusPage` (`internal/le/rfc/render_ledger.go`) runs `Collect` and `NewRenderInput`
- [ ] `internal/le/site/health.go`, `internal/le/test/health/modes.go`, `internal/le/test/health/collect_rfc.go` - `testhealth.Render` → `build` → `collectRFC`: `rfc.Collect`, and `implementedSections` runs `rfc.NewRenderInput`; the health page and `factsFromTestHealth` share one call through `Paths.testHealth`
- [ ] `internal/le/feature/check.go`, `internal/le/feature/due.go` - `StaleSupported` → `checkOn`: `rfc.Collect` and `stemFigures` (`rfc.NewRenderInput`)
- [ ] `internal/le/rfc/check.go` - `Check` → `check`: its own `Collect`, the HEAD^ baselines, `loadDiscrimination`, `verifyDiscrimination`; `CheckReport` carries JSON tags
- [ ] `internal/le/rfc/check_compile.go` - `checkTagPackagesCompile` runs `go vet` over every package carrying an RFC tag under `vetTimeout` (fifteen minutes)
- [ ] `internal/le/rfc/check_baseline.go` - `headRevision`, `priorRevision`, `backlogRevision`: HEAD, HEAD^, origin/main
- [ ] `internal/le/rfc/register.go`, `internal/le/rfc/write.go` - the RFC derived artifacts; `rebuildLedger` → `IndexUpdate` → `Collect`, reached from `derived.EnsureAll` when one is absent
- [ ] `internal/le/site/checkout_test.go` - `shareCheckoutDerivations` replaces the live seams with one shared derivation per test binary
- [ ] `.github/workflows/verify.yml` - lays the published gh-pages tree down beside the checkout before the site tests

The RFC corpus collections one full `./le site build` makes. The brief's
analysis named seven; reading the source finds eight, plus one conditional:

| # | Where | What it runs | Verified |
|---|-------|--------------|----------|
| 1 | `runBuild` → `feature.StaleSupported` → `checkOn` | `Collect`, `stemFigures` (`NewRenderInput`) | Yes. Not reached by `Build` alone or by a producer build |
| 2 | `publishRFCLedger` → `collectRequirementLedger` | `Collect`, `NewRenderInput` | Yes |
| 3 | `publishRFCData` → `collectRFCData` | `Collect`, `NewRenderInput` | Yes |
| 4 | `publishSiteFacts` → `factsFromRFCLedger` | `liveRFCLedger` (`Collect`), `liveStemFigures` (`NewRenderInput`) | Yes. The brief's `factsFromRFCLedger` and `liveStemFigures` are one collection |
| 5 | `factsFromTestHealth` and the health producer, one shared `testhealth.Render` | `collectRFC` (`Collect`), `implementedSections` (`NewRenderInput`) | Yes. Shared once per build by `Paths.testHealth` |
| 6 | docs producer → `rfc.StatusPage` | `Collect`, `NewRenderInput` | Yes. Not in the brief's list |
| 7 | rfc-compliance producer → `collectRFCCompliance` | `Collect`, `NewRenderInput`, `rfc.Check` | Yes |
| 8 | inside `rfc.Check` → `check` | its own `Collect`, baselines at HEAD^ and origin/main, the vet of every tagged package, the discrimination re-verification | Yes |
| 9 | `derived.EnsureAll` → `rebuildLedger` → `IndexUpdate` | `Collect`, only when an RFC derived artifact is absent | Yes, conditional. Whether the build reads any RFC derived artifact off disk is not verified: `liveDocSources` reads the status page live, and the shards are named only as link targets (`rfcShardPath`) |

**Behavior to preserve:**
- Every RFC page, mirror and data file the site publishes keeps its bytes for the same commit: `/quality/rfc-compliance/`, its detail pages, `/quality/rfc-compliance/profiles/`, `/reference/rfcs/`, the compliance snapshot, the requirement ledger, the per-RFC and per-profile data, the RFC and health keys of the site facts snapshot.
- `website/assets/js/rfc-figures.js` and `internal/le/site/llmsdata.go` keep reading the published paths they read today.
- `./le rfc check` is unchanged, and stays a stage of the verify engine (`rfcGateStages`).
- `./le site build producer` keeps rendering named producers over the existing artifact.

**Behavior to change:**
- The site build renders the RFC data from the recording and runs no RFC collection or check when the recording is fresh.
- The build regenerates the recording when it is absent or its saved timestamp is over a week old, and says so before it starts.
- A new command regenerates the recording on demand.
- The RFC pages state the date and commit of their data.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Site data files with an update command | `internal/le/site/facts` (`./le site facts update`, `website/data/repo-facts.json`), `internal/le/site/history` (`./le site history update`, `website/data/repo-history.json`), `internal/le/site/release` (`./le site release update`, `website/data/github-releases.json`); `docs/architecture/site-facts.md`, "Applying this to another generated file" | A number is derived once, by a command, into a data file the build reads instead of walking the tree | Extends: the same shape (one derivation, one file, one reader, one fix command, a dirty-tree warning). Adds the owner's age rule, which none of them has. Differs in one step: no gate compares the recording with HEAD, because the owner's rule is age only; the lag is disclosed on the page instead. `./le site history check`, which reports commits after the recorded one as lag and never fails, is the nearest precedent if a report is wanted later |
| Site facts timestamps | `internal/le/site/facts.go`, `siteFacts.GeneratedAt`, `PublishedAt`, `deriveSiteFacts` | The one site data file that records inside itself when it was produced | Extends: the recording carries its saved time inside the file the same way, so the week is measured from a value a checkout or a copy cannot reset |
| The published single homes of RFC values | `docs/architecture/site-facts.md`, "One published copy of each value", "RFC completion data"; `ai/META-DECISIONS.md`, "Website design" | The per-RFC and per-profile data, the site facts snapshot, the compliance snapshot and the requirement ledger each publish their values once | Extends: these files become the recording itself, read back by the next build, so no value is published twice. Only what no published file holds yet (the saved time, the commit, the RFC status Markdown, the health page's two RFC metrics) is added, in one header file beside them |
| Reading the last published artifact | `internal/le/site/build.go`, `lastPublished`, `snapshotArtifact`, `seedArtifact`; `.github/workflows/verify.yml` "Lay the published site down beside the checkout" | Every build snapshots the published artifact and seeds the output from it; CI lays it down before the site tests | Extends: "saved with the website itself" is read as this artifact, so the previous recording reaches the next build through the snapshot the build already takes, and CI already has it |
| Derived artifacts with invalidation | `internal/le/derived`, `Artifact`, `EnsureAll`, `WriteAtomic` | A pure function of the tree, untracked, deleted by any write to an input, rebuilt by its reader | Does not apply as the carrier: an artifact any RFC summary or tagged test write deletes would regenerate on nearly every build, which is the cost the age rule removes, and a derived artifact has no age by design. `WriteAtomic`'s one-rename publication is reused for the header file, and `shardsRendered`'s "present means whole" idea for the set |
| One reading shared per build | `internal/le/site/build.go`, `buildPaths`, `Paths.testHealth` (`sync.OnceValues`) | Every page that states a test count shares one test-health render | Extends: the recording is read once per build the same way and handed to every RFC producer and to `publishSiteFacts` |
| The producer record | `internal/le/site/producer.go`, `producerRecordDirectory` (`tmp/site`), `writeProducerRecord` | Records which routes each producer wrote, per build, in the checkout | Does not apply: it is a local record of one build's routes, never published and keyed per build, while the recording is data the pages render and must travel with the website |
| RFC derived artifacts | `internal/le/rfc/register.go` (ledger files, `rfc/requirements/` shards), `rfc/discrimination/` | The repository's own Markdown ledger and shards; the discrimination records are inputs of the check | Does not apply as the recording: they are repository Markdown, not the site's JSON projections, and the discrimination records are what the check reads, not what it answers. Whether the build still needs `EnsureAll` to rebuild the RFC ones is open question Q-6 |
| The RFC check and its report | `internal/le/rfc/check.go`, `Check`, `CheckReport` (JSON-tagged) | `./le rfc check` and `collectRFCCompliance` run it | Extends: the regeneration calls it once and records the parts the page shows; `CheckReport` already serialises, so the gate half needs no new type |
| The live seams of the site package | `liveRFCCompliance`, `liveRequirementLedger`, `liveRFCData`, `liveRFCLedger`, `liveStemFigures`, `liveDocSources`, `liveTestHealth`; `internal/le/site/checkout_test.go`, `shareCheckoutDerivations` | Each RFC producer derives live through its own seam; tests replace each seam | Replaces: the production RFC seams read the recording; the live derivations move behind one regeneration seam, called only by the age rule and the command. Tests replace that one seam |
| The build clock | `internal/le/site/footer.go`, `buildClock` | The publication time of one build, a variable a test fixes | Extends: the age is measured against `buildClock`, so a test fixes the day |
| The `le site` action table | `internal/le/site/actions.go`, `actions`; the `talk` verb with `Does: "update"` | Declares every `./le site` verb; help, completion and `le help ai` derive from it | Extends: the regeneration command is one more action in this table |
| The feature-freshness pass | `internal/le/site/freshness.go`, `feature.StaleSupported` | Warns about stale feature evidence before a full build | Does not apply: it renders no page, and keeping it in the build is decision 3 of `plan/handover/site-one-page-build.md`, which the owner has not taken |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le site build` → `runBuild` → `Build`.
- `./le site build producer` with an RFC producer → `runProducerBuild`.
- The new regeneration command (name: Q-3) → its handler in `internal/le/site/actions.go`.
- Format at entry: the RFC recording in the last published artifact, or none.

### Transformation Path
1. `Build` takes its snapshot of the published artifact as today.
2. The build reads the recording header from the snapshot and measures its saved time against `buildClock`.
3. Fresh (saved under seven days ago): the build carries the recorded RFC files into the output unchanged and hands the recording to every RFC producer and to `publishSiteFacts`. No `rfc.Collect`, `rfc.NewRenderInput` or `rfc.Check` runs for the site.
4. Absent, or saved seven days ago or more: the build prints the regeneration notice on stderr, runs the regeneration seam once (the live derivations of rows 2 to 8 of Current Behavior), writes the RFC data files and then the header last, and continues from step 3.
5. A recording that does not parse, names an unknown schema or is missing a member file: the build refuses, naming the file, the reason and the regeneration command.
6. The RFC producers render their pages from the recording, each stating the recorded date and commit.
7. The regeneration command runs step 4 whatever the age, writing into the published artifact.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le tooling ↔ published artifact | JSON files under the artifact's data directory, read back by the next build's snapshot | No |
| site package ↔ rfc package | `rfc.Collect`, `rfc.NewRenderInput`, `rfc.Check`, `rfc.StatusPage`, called only by the regeneration | No |
| site package ↔ test health package | the health page's two RFC metrics taken from the recording (Q-5) | No |

### Changed-Wire Census

**Search evidence:** scope `internal/le/site/`, `internal/le/test/health/`, `website/assets/js/`, `.github/workflows/`; terms `rfc-compliance.json`, `rfc-requirements.json`, `data/rfc/`, `site-facts.json`, `rfcLedgerFile`, `rfcComplianceSnapshot`, `rfcDataDir`. Positive control: `rfc-figures.js` loads the per-RFC index, and `llmsdata.go` names the requirement ledger, both found. The recording header is a new wire; the existing files keep their format and gain a reader (the next build).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| RFC recording header | writer | the regeneration seam in `internal/le/site/rfcrecord.go` (new), written last with `derived.WriteAtomic`; proposed name `rfc-record.json` in the artifact's data directory | New file; no current writer | New: holds schema, saved time, commit, dirty-tree flag, RFC status Markdown, the two health RFC metrics |
| RFC recording header | reader | the `Build` age step and every RFC producer (new) | New file; no current reader | New |
| RFC compliance snapshot | writer | `renderRFCCompliance` (`rfcComplianceSnapshot`) | Written from `collectRFCCompliance` today | Preserve format; written by the regeneration instead of every build |
| RFC compliance snapshot | reader | the page's Check results link (public readers); the next build (new) | Linked from the page today; no in-repo reader | Preserve; the next build becomes a reader |
| Requirement ledger | writer | `publishRFCLedger` | `writeNamedArtifact(paths.Output, rfcLedgerFile, ...)` | Preserve format; written by the regeneration |
| Requirement ledger | reader | `loadRequirementLedger` (rfc-compliance producer), `internal/le/site/llmsdata.go` | `readArtifactJSON(output, rfcLedgerFile, ...)` | Preserve |
| Per-RFC and per-profile data | writer | `writeRFCData` | Replaces the directory each build | Preserve format; written by the regeneration |
| Per-RFC and per-profile data | reader | `loadRFCFigureSet`, `website/assets/js/rfc-figures.js` | `loadFile` of the index | Preserve |
| Site facts RFC keys | writer | `factsFromRFCLedger` | `rfc.Collect` each build | Preserve keys; derived from the recording |
| Site facts RFC keys | reader | `internal/le/site/factmark.go`, `site.js`, home producers | `data-ze-stat` marks | Preserve |

### Integration Points
- `Build` (`internal/le/site/build.go`) - gains the age step between the snapshot and `refreshNativeSurfaces`.
- `renderRFCCompliance`, `renderRFCProfiles`, the docs producer, the health producer, `publishSiteFacts` - read the shared recording.
- `actions` (`internal/le/site/actions.go`) - one new action.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The regeneration calls the same `rfc` entry points the producers call today |
| No unintended coupling (components stay isolated) | Yes | Only `internal/le` packages change; no product code imports them (`TestNormalZeLinksNoInternalLe`) |
| No duplicated functionality (extends existing, does not recreate) | Yes | The published RFC files become the recording; no second copy of a value |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Tooling JSON, no wire encoding |
| Registration over hardcoding, outbound | Yes | The command is one row of the `le site` action table |
| Registration over hardcoding, inbound | Yes | Searched the help text, completion and `le help ai`: all derive from the action table; the producer list derives from `registerProducer` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Rendering from the recording gives the same bytes as rendering live at the same commit | Each producer already reads the ledger and figure set back from the output (`loadRequirementLedger`, `loadRFCFigureSet`) | AC-6 fails; a field is lost in the round trip | `TestTheRFCPagesRenderIdenticallyFromTheRecording` | unvalidated |
| A-2 | Every value the RFC producers need can be serialised | `rfcData` holds unexported fields (`stems`, `profiles`), so its JSON form is the written files, not the struct | The reader must rebuild `rfcData` from the written files | A-1's test | unvalidated |
| A-3 | The last published artifact is present for a normal build and in CI | `lastPublished` reads the gh-pages checkout; `verify.yml` lays it down | A build without it regenerates (absent), which the owner's rule allows | `TestABuildWithNoRecordingGeneratesItFirst` | unvalidated |
| A-4 | The saved time inside the header is the owner's "timestamp of the file saved" | Owner's words; a checkout resets file modification times | The owner overrules, and the age is read from the filesystem | Owner, D-2 constraint | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A regeneration records another session's uncommitted RFC edit | The dirty-tree flag is set | Warn before writing, as `warnUncommitted` does, and record the flag; the page says the data came from a tree with uncommitted changes when set |
| R-2 | The weekly regeneration reads as a hung build | An operator interrupts it | The notice before the check names the age, the reason, the bound of about fifteen minutes and the on-demand command |
| R-3 | The recording lags HEAD by up to a week | The recorded commit differs from HEAD | The page states the date and commit; the command refreshes it |
| R-4 | A seeded artifact from an older schema | Unknown schema in the header | Refuse with the command that regenerates it |
| R-5 | A test reaches the real regeneration and runs the fifteen-minute check | A site test past its package bound | Tests replace the regeneration seam, and the build tests assert it is not reached |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The website's RFC pages show old or missing data. Nothing in the shipped `ze` |
| How is it reverted? | Single commit revert; the next build regenerates the published files live |
| Who else touches this path? | `plan/handover/site-one-page-build.md` (the uncommitted one-page build in `internal/le/site/actions.go`, `internal/le/site/build.go`, `internal/le/site/producer.go`), any session editing `internal/le/site/` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le site build` with a fresh recording | → | `Build` age step, the RFC producers reading the recording | `TestABuildWithAFreshRecordingRunsNoRFCCheck` |
| `./le site build` with an aged recording | → | `Build` age step, regeneration seam | `TestABuildWithAnAgedRecordingRegeneratesItFirst` |
| `./le site build` with no recording | → | regeneration seam | `TestABuildWithNoRecordingGeneratesItFirst` |
| The regeneration command | → | its handler, regeneration seam | `TestTheRFCDataCommandRegeneratesWhateverTheAge` |
| `./le help site` | → | the action table | `test/ui/le-help-site-rfc-data.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A full build whose last published artifact holds a recording saved six days ago | The build completes and publishes every RFC page, mirror and data file from the recording. A test that replaces the regeneration seam, `rfc.Check` and `rfc.Collect` for the site with functions that fail the test proves none is reached | The feature-freshness pass in `runBuild` still collects the corpus: it renders no page, and moving it is the owner's decision 3 in `plan/handover/site-one-page-build.md` |
| AC-2 | A full build whose recording was saved seven days ago or more | The build prints the regeneration notice on stderr, runs the regeneration once, writes the RFC files and a header whose saved time is the build clock, then renders every RFC page from the new recording | None: an aged recording is always regenerated |
| AC-3 | A full build with no recording in the last published artifact, or no artifact | Same as AC-2, with the notice saying the recording is absent | None |
| AC-4 | A recording that does not parse, names an unknown schema, or whose header names a member file that is missing | The build stops with an error naming the file, what is wrong, and the regeneration command | A recording that parses and is fresh renders: the refusal is for an unreadable one only |
| AC-5 | The regeneration command, run with a recording saved one hour ago | It regenerates the recording, writes it into the published artifact, and prints the saved time, the commit, and the gate's first line | None: the command ignores age by the owner's design |
| AC-6 | A recording the regeneration writes at commit C, and a live render at commit C with the same build clock | Every RFC route, mirror and data file the build publishes is byte-identical between the two | The recorded-date line states the same date in both, because the clock is fixed |
| AC-7 | Any RFC page rendered from a recording | The page states "RFC data recorded on" the recorded date, at the recorded short commit, and says when the recording came from a tree with uncommitted changes | Pages that show no RFC data carry no such line |
| AC-8 | A fresh recording whose commit differs from HEAD | The build renders the recorded data unchanged and does not regenerate; the page shows the recorded commit, so the lag is visible | The lag of up to a week is the owner's design |
| AC-9 | `./le help site`, the help page of the command, completion after `./le site`, `./le help ai` | Each lists the command with its summary and parameters | None |
| AC-10 | `./le site build producer rfc-compliance` with a fresh recording | Renders the RFC compliance pages from the recording and states its date | Q-4 decides what an aged recording does here |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestABuildWithAFreshRecordingRunsNoRFCCheck` | `internal/le/site/rfcrecord_test.go` | AC-1 | |
| `TestABuildWithAnAgedRecordingRegeneratesItFirst` | `internal/le/site/rfcrecord_test.go` | AC-2, the notice text | |
| `TestABuildWithNoRecordingGeneratesItFirst` | `internal/le/site/rfcrecord_test.go` | AC-3 | |
| `TestAnUnreadableRecordingIsRefusedByName` | `internal/le/site/rfcrecord_test.go` | AC-4, each of the three causes | |
| `TestTheRFCDataCommandRegeneratesWhateverTheAge` | `internal/le/site/rfcrecord_test.go` | AC-5 | |
| `TestTheRFCPagesRenderIdenticallyFromTheRecording` | `internal/le/site/rfcrecord_test.go` | AC-6, A-1, A-2, over the shared checkout derivation | |
| `TestEveryRFCPageStatesTheDateOfItsData` | `internal/le/site/rfcrecord_test.go` | AC-7, dirty-tree wording | |
| `TestAFreshRecordingBehindHeadIsRenderedAsRecorded` | `internal/le/site/rfcrecord_test.go` | AC-8 | |
| `TestTheAgeIsReadFromTheSavedTimeNotTheFile` | `internal/le/site/rfcrecord_test.go` | The D-2 constraint: a header saved eight days ago whose file modification time is now is aged | |
| `TestAProducerBuildReadsTheRecording` | `internal/le/site/rfcrecord_test.go` | AC-10 | |
| `TestBuildRendersEveryPublishedRoute` (existing) | `internal/le/site/site_test.go` | Every route still published, with the regeneration seam shared | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Recording age | 0 to under 7 days is fresh | 6 days 23 hours 59 minutes | a saved time after the build clock is refused as a clock error | 7 days exactly is aged |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-help-site-rfc-data` | `test/ui/le-help-site-rfc-data.ci` | The owner reads `./le help site` and finds the command that refreshes the RFC data | |

The full build path is not a `.ci`: it compiles the daemon and lays down the
published artifact. The entry-point proof is the build tests above, which call
`Build`, the function `runBuild` calls.

## Files to Modify
- `internal/le/site/build.go` - the age step, the shared recording in `buildPaths`
- `internal/le/site/actions.go` - the regeneration command
- `internal/le/site/rfccompliance.go` - read the gate snapshot from the recording; the date line
- `internal/le/site/rfcledger.go` - the ledger from the recording; `collectRequirementLedger` moves behind the regeneration
- `internal/le/site/rfcdata.go` - the per-RFC and per-profile data from the recording
- `internal/le/site/rfcprofiles.go`, `internal/le/site/rfcdetail.go` - the date line
- `internal/le/site/facts.go` - `factsFromRFCLedger` from the recording
- `internal/le/site/docs.go` - the RFC status page from the recording
- `internal/le/site/health.go` - the two RFC metrics from the recording (Q-5)
- `internal/le/test/health/collect_rfc.go`, `internal/le/test/health/modes.go` - a render that takes the RFC metrics as input (Q-5)
- `internal/le/site/checkout_test.go` - `shareCheckoutDerivations` shares the regeneration seam
- `website/AI.md` - "The RFC requirement ledger"
- `docs/contributing/gh-pages.md` - "Quality pages"
- `docs/architecture/site-facts.md` - the "Already gated" row, "RFC completion data", a section on the recording and its age rule
- `docs/contributing/rfc-conformance-gates.md` - the published detail pages paragraph
- `docs/architecture/testing/test-health.md` - declared by the test-health sources this spec changes: the render that takes the two RFC metrics as input, and that the site's copy of them comes from the recording (Q-5)
- `plan/handover/site-one-page-build.md` - decision 1 points here

## Files to Create
- `internal/le/site/rfcrecord.go` - the recording: header, read, age, regeneration seam
- `internal/le/site/rfcrecord_test.go` - the unit tests above
- `test/ui/le-help-site-rfc-data.ci` - the help functional test

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling, no daemon command |
| YANG validation constraints | N-A | No YANG |
| YANG custom validators | N-A | No YANG |
| CLI commands/flags | Yes | `internal/le/site/actions.go`, one action |
| CLI grammar (keyword before value) | Yes | The command takes no value; `./le cli grammar` judges it |
| Editor autocomplete | N-A | le completion derives from the action table |
| Functional test for new RPC/API | Yes | `test/ui/le-help-site-rfc-data.ci` |
| Pipe completeness | Yes | The command answers a structured report through the shared renderer |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | Development tooling, nothing on the appliance |
| Doctor code per problem class | N-A | Development tooling |
| Prometheus counters/metrics | N-A | None |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Tooling, no feature declaration |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `website/AI.md`, `docs/contributing/gh-pages.md` (le site commands are documented there, not in the ze command reference) |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | The ledger's contents are unchanged |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/site-facts.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Run `./le spec citation anchors spec plan/post-release/engineering/spec-site-rfc-recorded-data.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/rfc-conformance-gates.md`, `docs/contributing/gh-pages.md` |

## Implementation Steps

1. **Phase: Wiring** -- the action, the age step and the regeneration seam as stubs
   - Tests: the Wiring Test rows
   - Files: `internal/le/site/actions.go`, `internal/le/site/build.go`, `internal/le/site/rfcrecord.go`
   - Verify: the tests reach the seam and fail on the stub
2. **Phase: Regeneration** -- the seam writes the RFC files and the header, header last
   - Tests: `TestTheRFCDataCommandRegeneratesWhateverTheAge`, `TestABuildWithNoRecordingGeneratesItFirst`
3. **Phase: Readers** -- each RFC producer and `publishSiteFacts` read the shared recording
   - Tests: `TestTheRFCPagesRenderIdenticallyFromTheRecording`, `TestABuildWithAFreshRecordingRunsNoRFCCheck`
4. **Phase: Age, refusal, date line**
   - Tests: the remaining unit tests and the boundary rows
5. **Phase: Docs** -- every page in Files to Modify, in the same commits as the code they describe

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No RFC producer, and no site facts key, still calls an `rfc` collector when the recording is fresh |
| Naming | JSON keys of the header are kebab-case |
| Data flow | The header is written last, so its presence means the set is whole |
| Operator errors | The notice and every refusal name the regeneration command |
| Rule: one published copy | No value appears in the header and in another published file |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The command exists | `./le help site` lists it |
| A fresh build runs no RFC check | `./le test unit pkg path ./internal/le/site/ run 'Recording'` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The header read from the artifact is untrusted input: a member path must stay inside the artifact's data directory, refused otherwise |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Owner Decisions Taken

| ID | Question | Owner's words | What the spec does |
|----|----------|---------------|--------------------|
| D-1 | Where the recording lives | "The data can be saved with the website itself." | The recording is saved in the published website, in the data directory of the gh-pages checkout: the RFC files the build already publishes there (the compliance snapshot, the requirement ledger, the per-RFC and per-profile data) plus one header file beside them, proposed as `rfc-record.json`. Not a cache under `tmp/` or the user's cache. Q-1 holds the one reading left open |
| D-2 | How the week is measured | "The week is measured from the timestamp of the file saved." | The age is the build clock minus the saved time |

**Constraint from D-2.** A `git checkout`, a clone or a copy resets a file's
modification time, so a fresh checkout of gh-pages, the export CI lays down, or
the build's own snapshot would make a filesystem time look brand new or
arbitrarily old. The spec therefore reads "the timestamp of the file saved" as
the time the regeneration saved it, recorded inside the header, as the site
facts snapshot already records `generated_at` and `published_at`. The
alternative, which the owner may choose instead: read the header file's
modification time, and accept that every fresh copy of the site resets the week.

## Open Design Questions

| ID | Question | Evidence | Recommendation |
|----|----------|----------|----------------|
| Q-1 | "Saved with the website itself": the published artifact only, or the website source tree `website/data/` on main | `website/data/repo-facts.json`, `website/data/repo-history.json` and `website/data/github-releases.json` are committed on main and staged into the artifact. The published RFC files exist only in the gh-pages checkout today, the requirement ledger is 22,860,346 bytes, and `verify.yml` lays gh-pages down beside the checkout. Artifact only: a fresh checkout with gh-pages beside it and CI both have the recording; a checkout without gh-pages regenerates once; main never carries 23 MB per regeneration; a regeneration dirties no tracked file in the shared main checkout. `website/data/`: every fresh checkout has it, but each regeneration adds a 23 MB blob to main, leaves a tracked file modified in the shared checkout, and publishes a second copy unless the file is listed in `sourceOnlyFiles` (`internal/le/site/paths.go`) | The published artifact. The header file is `rfc-record.json` in its data directory |
| Q-2 | The page lags HEAD by up to a week | The owner's rule is age only (D-2). The recorded commit is in the header | Taken as decided. Every RFC page states the recorded date and short commit (AC-7, AC-8) |
| Q-3 | The command's name and place | The sub-areas `./le site facts update`, `./le site history update`, `./le site release update` live in their own packages (`internal/le/site/facts` and the others), and the path rule would put `./le site rfc update` in a package `internal/le/site/rfc`. The build must call the regeneration, so that package could not import `site`, and the producers' projections (`requirementLedgerFrom`, `collectRFCCompliance`, `collectRFCData`) would have to move out of `site`. A verb of the `site` area itself (`talk` and `config-tree` are) keeps them where they are | `./le site rfc-data`, a verb of the `site` area with `Does: "update"` and `Writes: true`, in `internal/le/site/actions.go`. Help, completion and `le help ai` derive from the table |
| Q-4 | The fifteen-minute check stays inside `./le site build` once a week; what a producer build does with an aged recording | The vet bound is `vetTimeout` (fifteen minutes). `freshness` already prints its warning to stderr before the build | Yes, once a week, by the owner's design. Before it starts the build prints, on stderr: the recording's saved date or that it is absent, that the RFC check now runs and can take about fifteen minutes, and that `./le site rfc-data` refreshes it on demand. A producer build applies the same age rule, so a one-page render never publishes data older than a full build would |
| Q-5 | Which collections the recording replaces | Current Behavior, rows 1 to 9 | Replaces rows 2 to 8, including the health page's two RFC metrics (row 5), which are RFC figures on the site: `testhealth` gains a render that takes them as input, and `./le test health update` keeps reading live. Row 1, the freshness pass, stays (handover decision 3) |
| Q-6 | Whether the build still needs `derived.EnsureAll` to rebuild the RFC derived artifacts (row 9) | The status page is read live through `liveDocSources`; the shards are named only as link targets | The implementer verifies whether any build step reads an RFC derived artifact off disk. If none does, the build skips them, so a fresh recording runs no RFC collection at all |
| Q-7 | What CI does | CI does not publish the site. `TestBuildRendersEveryPublishedRoute` runs in CI over the real checkout through `shareCheckoutDerivations`; `./le rfc check` is a verify stage | CI never regenerates the recording and gates neither its age nor its agreement with HEAD. The site tests share the regeneration seam once per test binary, as today, and the RFC gate stays a verify stage |

## Design Insights

- The RFC check is one of eight corpus collections per build, not the only cost. Making the published RFC files the recording removes all eight for a fresh recording, which is what lets a one-sentence page edit publish in minutes.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The published RFC files are the recording, with one header | One new file holding everything | A second file holding the same values breaks the one-published-copy rule |
| The saved time is recorded inside the header | Filesystem modification time | A checkout, clone or copy resets it |
| The header is written last | Written first | Its presence then means the whole set was written |
| An unreadable recording is refused | Silently regenerated | A broken recording is a defect to see, and the command that clears it is one line |

## Known Limitations
- The pages can lag the code by up to a week, by the owner's design. The page states the date.
- The feature-freshness pass still collects the corpus in a full build: handover decision 3, not taken.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout. No agent runs any `./le verify` command
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: tooling, no protocol peer)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-site-rfc-recorded-data.md` only, in the same `./le commit create` script
