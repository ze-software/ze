# Spec: Release roadmap for the repository, website, and weekly news

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling, docs |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-18 |

## Task

Use the specifications under `plan/` to show the work required before the first
release and the nice-to-haves. Generate the repository index and website from
one inventory. Give the weekly update skill the same inventory and a comparison
between reporting dates, for the website news post and its Discord message.

This document is a proposed implementation plan. It authorizes no publication,
Discord message, bulk spec reprioritization, or product implementation in this
planning session.

### Owner request, 2026-09-18

> it would be good if for the website we had a list of the spec before release and also the nice to have so we can keep track of our progress toward release.
> it should be kept organised in the repository in the plan folder and we should be able to expose it on the website when we generate it.
> make a plan for this work

> it should also be used for our ze update news skill and discord message

### Goals

| Goal | Observable result |
|------|-------------------|
| Repository organization | Existing release buckets remain authoritative. A generated index under `plan/` lists their specs and statuses. |
| Public progress | The existing `/project/roadmap/` page shows release-required work separately from nice-to-haves, with source links and snapshot provenance. |
| Shared news evidence | `ze-weekly-update` reads the same report, explains changes in plain language, and preserves those facts in the approved weekly source. |
| Honest reporting | Added, removed, reprioritized, and status-changed items remain distinguishable. Counts never imply effort estimates or release readiness. |

The tracking feature belongs in the root backlog: Ze can release without this
website feature. This placement does not change any existing spec's priority.

## Required Reading

- [ ] `plan/README.md` and `docs/contributing/spec-workflow.md`
  → Constraint: directory placement determines release importance. Status determines lifecycle. Closure deletes a spec.
- [ ] `docs/contributing/gh-pages.md` and `website/AI.md`
  → Decision: preserve `/project/roadmap/`, the shared page shell, Markdown mirror, navigation, and producer ownership checks.
- [ ] `ai/skills/ze-weekly-update.md` and `website/changes/discord/STYLE.md`
  → Constraint: verify delivered claims at their producers. Show Thomas the exact message before Discord publication.
- [ ] `ai/rules/architecture.md`, `ai/rules/simplicity.md`, and `ai/patterns/cli-command.md`
  → Decision: reuse the spec parser and registered native commands. Add no service, database, JavaScript framework, or hand-maintained backlog.
- [ ] `ai/rules/evidence.md`, `ai/rules/cli.md`, and `ai/rules/repo-maintenance.md`
  → Constraint: return structured reports, preserve diagnostic states, register discovery, and edit canonical skill sources before synchronization.

**Key insights:** Both release-required buckets must be counted, including their
skeleton, blocked, and deferred specs. `verification` remains open. A removed
file supplies no independent proof that its intended behavior was delivered.

## Current Behavior

### Source files read

| Producer | Verified behavior | Consequence |
|----------|-------------------|-------------|
| `internal/le/spec/specpath/specpath.go`: `Bucket`, `Dirs`, `All` | Defines the three direct-child spec populations. The root bucket is `after`. | Reuse this declaration for discovery and classification. |
| `internal/le/spec/status/specstatus.go`: `loadSpec`, `Collect` | Reads filesystem specs and metadata. Missing tables and statuses remain visible. `Depends` is prose. | Share metadata parsing with revision-backed input. Do not create another parser or dependency graph. |
| `internal/le/spec/status/report.go`: `Spec`, `Inventory` | Carries bucket, status, dates, dependencies, phase, and set. It lacks title and source path. | Add the missing display facts at their parser, with all callers inspected through LSP. |
| `internal/le/site/docsmanifest.go`: `oneShotPages` | The authored roadmap owns `project/roadmap/index.html`. | Transfer this route to one registered roadmap producer. |
| `internal/le/site/docs.go`: `docsSourceText`, `docsRenderer.render` | Supports generated Markdown and supplies the shared HTML shell and mirror. | Reuse the Markdown rendering path instead of implementing another HTML table renderer. |
| `internal/le/site/build.go`: `Build` | Full and partial builds both run the producers. Its source digest covers staged website files. | Collect plan inputs on every build. Carry their own digest in the roadmap report. |
| `internal/le/weekly/weekly.go`: `ParseText` | Separates arbitrary scalar front matter from the body. | Store report revision references in weekly front matter. |
| `internal/le/weekly/poster.go`: `publisher.publish` | Splits the source body, previews without `confirm`, sends on confirmation, then archives the text. | Prepare roadmap prose before approval. Never inject changing counts during sending. |

`website/roadmap/roadmap.md` contains a static release narrative.
`website/data/features.json` also carries manually maintained pending-spec cards.
Replace their backlog assertions with the canonical roadmap. Preserve useful
release-policy prose under `plan/README.md` without implying that every optional
spec must close before release.

Existing related work remains independent: `spec-site-renderers-in-go` owns the
site generator migration, and `spec-le-every-area-dispatches-through-one-table`
owns broader command grammar changes. `spec-remove-takes-the-working-tree-copy`
owns closure filesystem cleanup. This feature reads committed trees for public
reports, so an untracked leftover cannot reappear as a release blocker.

**Preserve:** all bucket assignments, status vocabulary, existing `spec status`
behavior, closure rules, public roadmap URL, and weekly approval/archive/resume
behavior. Do not implement a release gate or change the Discord transport.

## Proposed Design

### Source and presentation contract

| Input or output | Contract |
|-----------------|----------|
| `plan/immediate/spec-*.md` | Required before release: operator-visible defects and missing behavior. |
| `plan/pre-release/spec-*.md` | Required before release: other release obligations. |
| `plan/spec-*.md` | Nice-to-haves: the first release can ship without them. |
| Spec metadata and heading | Own the lifecycle state, dependencies, phase, date, and title. No parallel YAML or JSON backlog. |
| `plan/roadmap.md` | Generated repository index with relative spec links and source revision. Generated on demand, untracked, following existing derived-index conventions. |
| `/project/roadmap/` | Generated HTML and `index.md` from the same typed inventory. Show required subgroups, nice-to-haves, status breakdown, and source revision. |
| `data/release-roadmap.json` in the site output | Machine-readable copy of the exact inventory used by the page, including schema version, resolved revision, and input digest. |
| Weekly source front matter | `release-from` and `release-to` record full commit IDs behind the report used during drafting. Historical prose stays fixed. |

`plan/README.md` remains the checked-in entry point. Replace its dated count
column with the index regeneration command and the bucket definitions. Register
`plan/roadmap.md` as a derived index, so absent output is regenerable. The website
calls the producer directly and never depends on a previous local regeneration.

Register this output with `derived.Register` in the new roadmap package, using
`SessionStartDefer` and `derived.WriteAtomic`. Its completeness check compares
the recorded revision with resolved `HEAD`, so a commit cannot leave a present
index looking current. An explicit historical update remains labeled historical.

Use committed `HEAD`, resolved once to a full commit ID, by default. Read paths
and blobs from that Git tree without checkout, worktree creation, or modification
of the index. This excludes uncommitted and untracked specs from public claims.
The page and repository instructions state that pending edits appear after they
are committed and the output is regenerated.

Factor the existing metadata parser into a shared byte-input operation inside
spec status tooling. Keep filesystem `Collect` behavior unchanged. The roadmap
collector uses the same parser over revision blobs and the same bucket registry.
Batch Git reads rather than starting a process for each spec. Do not use current
filesystem dates as substitutes for historical metadata.

### Native command surface

The names below are proposed new actions, not commands available today.

| Command | Effect |
|---------|--------|
| `./le spec roadmap list [revision <ref>]` | Return one typed snapshot. Default revision is committed `HEAD`. |
| `./le spec roadmap update [revision <ref>]` | Generate `plan/roadmap.md` from that snapshot. Return path and provenance. |
| `./le spec roadmap compare from <ref> to <ref>` | Return endpoint inventories and their changes. Resolve and report both full commit IDs. |

Register the sub-area through the existing native action mechanism. All report
commands use the common pipe renderers, including JSON, YAML, and table output.
Unknown references, missing history, and read failures return named errors.

### Inventory and comparison rules

| Case | Required behavior |
|------|-------------------|
| Required total | Sum all specs in `immediate` and `pre-release`, regardless of status. Keep each subgroup visible. |
| Nice-to-have total | All root-bucket specs, independently counted. |
| Umbrella and child specs | List each real file once. Label the measure as spec/work-item count, never feature count or effort. |
| State presentation | Display the declared status and a short legend. Do not derive completion from phase text, checkboxes, or prose. |
| Missing or unknown metadata | Keep the item and its bucket count visible, with a diagnostic. Never silently omit an obligation. |
| Missing title/date | Use the explicit spec identifier as a labeled missing-title presentation. Show an unavailable date instead of inventing one. |
| Dependencies | Display declared text as escaped text. Link only exact, unambiguous references that exist in the selected tree. |
| Duplicate spec stem | Refuse the ambiguous report with both paths. Do not overwrite one entry in a map. |
| Ordering | Fixed bucket order, existing status order, then stable spec identifier as a tie-breaker. |
| Same stem, different bucket | Report one move with old and new buckets. It is no completed item. |
| Same stem, different status | Report the transition. A move can also carry a status transition. |
| Added or absent stem | Report added or removed. A renamed stem is removed plus added, with no guessed identity. |
| Removed spec links | Link the spec at the earlier commit. Current links use the selected commit. |
| Empty valid population | Render an explicit empty state and zero counts. Do not announce release readiness. |
| Missing `plan/` | Return an error. A failed read is not an empty backlog. |

The comparison is an **endpoint comparison**. Work added and removed entirely
between the two revisions does not appear. Say this in the report and weekly
instructions. It answers how the remaining queue changed, not how much work was
performed during the week. Existing commit/source research still establishes
what was delivered, including items that existed only inside that interval.

Do not add a completion percentage, estimated release date, automatic completion
ledger, or all-time closure counter. These require facts this inventory does not
hold. A drop in required count must retain its added/removed/moved explanation.

### Weekly update and Discord workflow

1. Use the prior post's `release-to` as the next comparison start when available.
2. For the first report, resolve the committed tree at the start of `covers:` and
   at the end of its final day, using UTC and the current branch's first-parent
   history. Record the resolved IDs. A missing boundary is an error, not zero.
3. For a normal report, select the end snapshot by the same end-of-period rule.
   An owner-authorized in-progress update identifies its cutoff explicitly.
4. Run the shared comparison before drafting. Read both required subgroups and
   the nice-to-have group. State remaining work and relevant scope changes.
5. Translate selected items into user-facing capability. Keep planned work under
   `Coming up`. A removed spec needs the existing source verification before a
   sentence says the behavior is delivered.
6. Add a short release-progress paragraph and the public roadmap link. Say
   `release work items` in news prose. Keep spec filenames and workflow jargon
   out of the Discord body. Preserve the standing RFC section and message budget.
7. Save fixed facts in `website/changes/posts/<covers-start>.md`, with the two
   revision IDs in front matter. Website news and Discord consume that one body.
8. Run the existing dry-run command and show its exact messages for approval.
   Later spec changes must not change the approved body or an archived update.

Edit `ai/skills/ze-weekly-update.md`, then run `./le ai skills-sync`. Update
`website/changes/discord/STYLE.md` and `website/AI.md` to explain the short
release-progress section and pinned evidence. No posting occurs as verification
of this implementation. Existing `confirm`, channel, and resume rules remain.

## Data Flow

| Stage | Input | Output |
|-------|-------|--------|
| Entry | Explicit revision or once-resolved `HEAD` | Immutable Git tree identity |
| Discovery | Canonical `specpath` buckets in that tree | Spec paths and bytes |
| Parsing | Shared metadata parser | Typed rows, titles, paths, diagnostics |
| Aggregation | Rows and optional earlier snapshot | Counts and endpoint changes with provenance |
| Repository | Snapshot | Generated `plan/roadmap.md` |
| Site | Snapshot collected once per build | JSON, HTML, and Markdown mirror |
| News | Pinned comparison plus verified product evidence | Approved Markdown source, rendered website post, unchanged Discord body |

### Architectural Verification

| Check | Design constraint |
|-------|-------------------|
| Intended layers | Both collectors share parsing. Site and news consume the report without scanning specs themselves. |
| Component isolation | All code stays in development tooling. No daemon or protocol dependency is added. |
| No duplicate authority | Bucket registry and spec headers remain the only live declarations. Git supplies history. |
| Registration | New native sub-area and site producer register themselves. Transfer the old route rather than adding a second owner. |
| Performance | One pinned snapshot, batched blob reads, stable sorting. No hot-path allocations or runtime worker is involved. |
| Provenance | Roadmap JSON carries the digest of the inputs actually read. Do not claim the website-only `SourceDigest` covers plan data. |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | Existing directories express the desired required/optional split. | `plan/README.md` and owner request | Owner triage is needed, separately from presentation. | Design approval, no automatic moves | Proposed |
| A-2 | Remaining counts plus endpoint changes meet the progress goal. | No requested estimates or percentage | Additional completion evidence would need an explicit contract. | Design approval | Proposed |
| A-3 | Public reports must describe committed specs. | Source links and reproducible weekly history | Local edits need a separately labeled preview. | Design approval | Proposed |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Deletions or reprioritization look like delivered work. | A reduced total has no change explanation. | Named change categories and source verification for delivered claims. |
| R-2 | Blocked, deferred, or malformed specs disappear from required totals. | Counts disagree with discovered bucket files. | Count all rows before presentation filters and show diagnostics. |
| R-3 | A shallow clone cannot resolve a weekly boundary. | Git cannot read a requested revision or blob. | Refuse and name the history to fetch. Never substitute the current tree. |
| R-4 | Site copies and curated feature cards disagree. | Two pending-spec lists survive. | Replace the feature-card backlog with a link to the canonical roadmap. |
| R-5 | Concurrent commits mix snapshots. | One output contains multiple source IDs. | Resolve once, then read immutable blobs for every consumer. |
| R-6 | A historical update changes after approval. | Posting recollects live counts. | Materialize prose before approval and preserve the existing sender. |
| R-7 | Spec titles or dependencies inject markup. | Raw metadata reaches HTML or Markdown links. | Escape cells and labels, constrain links to repository paths and commit IDs. |
| R-8 | Endpoint changes are mistaken for weekly throughput. | An update calls removed counts completed counts. | State interval limitations and retain independent shipped-feature research. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Public release reporting and news accuracy. No router behavior changes. |
| Recovery | Correct the producer and regenerate in a private output directory. Do not edit generated pages. |
| Shared ownership | Spec status consumers, site producer coverage, derived-index registration, and weekly skill mirrors. |

## Wiring Test

| Entry Point | Feature Code | Proof |
|-------------|--------------|-------|
| `le spec roadmap list` and pipe forms | Shared revision collector | `TestRoadmapCommandUsesSelectedRevision` |
| `le spec roadmap update` | Same snapshot, repository renderer | `TestRoadmapIndexMatchesSnapshot` |
| Full and partial `le site build` | Registered roadmap producer | `TestRoadmapBuildTracksPlanChanges` |
| Weekly skill through `le weekly source <draft>` | Pinned report, saved body, existing publisher | `release-roadmap-weekly-preview` smoke scenario |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Specs in every canonical bucket and lifecycle state | Every file appears once. Both required buckets count toward release-required work. |
| AC-2 | Generate the repository index | `plan/roadmap.md` has grouped lists, status legend, links, and provenance. `plan/README.md` explains its regeneration. |
| AC-3 | Full or partial site build after committed spec-only changes | Existing roadmap URL, mirror, and JSON reflect the selected new tree without a manual index refresh. |
| AC-4 | A bucket move, state transition, addition, and removal | Comparison reports each fact correctly. No event is automatically called completed. |
| AC-5 | Unknown status, absent metadata, duplicate identity, missing tree, or missing history | Diagnostics preserve known obligations. Ambiguous or unreadable populations fail with named errors. |
| AC-6 | Render identical revision inputs twice | Roadmap data and content remain identical, apart from the site's existing publication stamp. |
| AC-7 | Open roadmap with JavaScript disabled and on a narrow screen | Both lists and counts are readable. Tables stay within their containers. Links, navigation, search, and machine-readable pages reach it. |
| AC-8 | Draft a weekly update for a bounded interval | Skill uses the shared comparison and records revision IDs. News distinguishes queue changes from verified delivery and links the roadmap. |
| AC-9 | Change specs after approving a weekly draft | Dry-run and archived text retain approved historical facts. No live insertion and no automatic Discord send occurs. |
| AC-10 | Feature page and discovery outputs after cutover | No separately maintained pending-spec card list or deletion-means-delivered assertion remains. Existing feature facts stay intact. |
| AC-11 | Concurrent commits or an untracked closed-spec leftover | A report stays bound to its original committed tree. Untracked files never become public blockers. |

## TDD Test Plan

| Test | Location | Contract defended |
|------|----------|-------------------|
| `TestRoadmapCommandUsesSelectedRevision` | New roadmap package tests | Registered entry, immutable revision selection, structured output and explicit history errors. |
| `TestRoadmapBucketAccounting` | New roadmap package tests | Parking states, umbrellas, malformed metadata, empty population, and duplicate identity cannot shrink obligations silently. |
| `TestRoadmapComparisonIsNotCompletion` | New roadmap package tests | Moves, renames, deletions, and endpoint-only changes preserve their meaning. |
| `TestRoadmapIndexMatchesSnapshot` | New roadmap package tests | Repository rows and links match the collected revision. |
| `TestRoadmapBuildTracksPlanChanges` | Site tests | Real full/partial producer execution publishes changed spec data, matching mirror and JSON, with one route owner. |
| Existing weekly preview/archive tests | `internal/le/weekly/poster_test.go` | Approval and exact-body publication remain unchanged. |

Use small temporary Git repositories for historical edge cases. Include a spec
added and removed between endpoints to prove the documented comparison limit.
Numeric boundaries are zero, one, and multiple items. No protocol or Linux-only
behavior changes, so RFC and interop tests are inapplicable.

Functional proof uses the named native commands against an isolated fixture,
then a site build into session scratch. Inspect the actual page in a browser,
including JavaScript disabled, narrow width, source links, and Markdown mirror.
For `release-roadmap-weekly-preview`, draft from pinned report data, run the
existing weekly dry run, change the fixture inventory, and repeat the dry run.
The approved body must remain unchanged and neither run can send a message.

## Files to Modify

- `internal/le/spec/status/specstatus.go`: share metadata parsing without changing filesystem status behavior.
- `internal/le/spec/status/report.go`: add title and source path facts.
- `internal/le/register.go`: import the registered roadmap sub-area once.
- `internal/le/site/docsmanifest.go`: transfer the existing roadmap route out of the authored manifest.
- `internal/le/site/docs.go`: reuse the Markdown rendering path for generated input.
- `internal/le/site/producer.go`: declare the published JSON through the named-artifact contract.
- `internal/le/site/datapages.go`: replace the curated roadmap section with a link, without counting it as a feature.
- `internal/le/site/facts.go`: remove the retired planned-card counter. Preserve shipped/experimental counts.
- `internal/le/site/derived.go`: remove the retired counter from machine-readable prose.
- `internal/le/site/facts_test.go`: update the changed planned-card contract while preserving other count assertions.
- `website/data/features.json`: retire the pending-spec card inventory.
- `website/roadmap/roadmap.md`: retire the authored backlog after preserving applicable policy in `plan/README.md`.
- `plan/README.md`: explain bucket definitions and regeneration. Remove dated manual counts.
- `.gitignore`: ignore generated `plan/roadmap.md`.
- `ai/skills/ze-weekly-update.md`: add report collection, pinned boundaries, and release-progress drafting.
- `website/changes/discord/STYLE.md`: describe the short progress section and historical facts.
- `website/AI.md`: describe the producer and weekly checklist.
- `docs/contributing/spec-workflow.md`: document the shared report and commands.
- `docs/contributing/gh-pages.md`: document the generated public roadmap.
- `ai/INDEX.md`: index release progress and shared news evidence.
- `docs/architecture/site-facts.md`: document immutable Git-tree inventory as the roadmap's provenance boundary. Keep the existing repository-facts snapshot and gate unchanged.

No route migration is needed. Existing navigation and sidebar entries remain.
Inspect their descriptions and correct only statements the new content changes.
No change to `internal/le/weekly` is required for the proposed front matter or
saved-body workflow. Keep its transport out of scope unless verification shows
that it cannot honor AC-9.

## Files to Create

- `internal/le/spec/roadmap/register.go`: native action grammar and derived-index registration.
- `internal/le/spec/roadmap/roadmap.go`: pinned collection, report model, counts, and repository rendering.
- `internal/le/spec/roadmap/compare.go`: endpoint comparison through the shared collector.
- `internal/le/spec/roadmap/roadmap_test.go`: historical edge cases and command reachability.
- `internal/le/site/roadmap.go`: one registered producer for page, mirror, and JSON.
- `internal/le/site/roadmap_test.go`: full and partial build proof.
- `plan/roadmap.md`: generated output, never an authored backlog or a new spec population.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | Development tooling only. |
| YANG validation constraints | N-A | No configuration change. |
| YANG custom validators | N-A | No configuration change. |
| CLI commands/flags | Yes | New native `spec roadmap` sub-area. |
| CLI grammar | Yes | Registered action keywords precede revisions. |
| Editor autocomplete | N-A | No daemon editor command. Native help and completion derive from action registration. |
| Functional test for new RPC/API | N-A | No RPC. Native command and browser smoke scenarios prove entry points. |
| Pipe completeness | Yes | Shared native renderers consume typed reports. |
| Env var registration | N-A | No new environment variable. |
| Doctor check for runtime dependencies | N-A | Uses existing repository Git/build requirements. No daemon dependency. |
| Prometheus counters/metrics | N-A | Build-time report only. |
| BGP family surface | N-A | No protocol change. |

### Documentation Update Checklist

| # | Question | Applies? | File / reason |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/contributing/gh-pages.md` covers the public roadmap. `docs/features.md` describes the router and stays unchanged. |
| 2 | Config syntax changed? | N-A | No configuration change. |
| 3 | CLI command added/changed? | Yes | `docs/contributing/spec-workflow.md` and `ai/INDEX.md` own native development commands. |
| 4 | API/RPC added/changed? | N-A | No runtime API. Document the website JSON in `website/AI.md`. |
| 5 | Plugin added/changed? | N-A | No plugin change. |
| 6 | Has a user guide page? | Yes | `plan/README.md` explains repository use. |
| 7 | Wire format changed? | N-A | No protocol change. |
| 8 | Plugin SDK/protocol changed? | N-A | No plugin change. |
| 9 | RFC behavior changed or proven? | N-A | Reporting does not alter RFC evidence. |
| 10 | Test infrastructure changed? | N-A | Reuse native package tests and browser verification. |
| 11 | Affects daemon comparison? | N-A | No router capability change. |
| 12 | Internal architecture changed? | Yes | `docs/contributing/spec-workflow.md` and `website/AI.md` describe shared report flow. |
| 13 | Route metadata changed? | N-A | No route metadata. |
| 14 | Prometheus counters changed? | N-A | No runtime metrics. |
| 15 | Command or inventory changed? | Yes | `ai/INDEX.md`, native action registration, and `plan/README.md`. |
| 16 | Source anchors affected? | Yes | `docs/contributing/gh-pages.md`, `website/AI.md`, and `docs/architecture/site-facts.md` cover changed reporting. `docs/architecture/core-design.md` and `docs/contributing/ze-go-style.md` remain unchanged because daemon architecture and composition conventions are unaffected. |
| 17 | Existing examples affected? | Yes | Weekly skill, Discord style, website checklist, and new report examples must agree on the selected revisions. |

### Discovery and Maintenance

| Concern | Required integration |
|---------|----------------------|
| First lookup | Add release roadmap and release progress to `ai/INDEX.md`, beside spec status and weekly updates. |
| Drift prevention | Native parser owns metadata. Bucket registry owns placement. Generated output declares its source and cannot become input authority. |
| Verification | Existing spec/site package runs, native entry smoke, site check, and browser inspection. No new project-wide gate. |
| Skill distribution | Edit `ai/skills/ze-weekly-update.md`, then synchronize generated copies with `./le ai skills-sync`. |

## Implementation Steps

1. **Wire the shared report.** Register the native sub-area and page producer.
   Transfer route ownership. Establish failing command and full-build cases.
   Document the proposed command contract before the next behavior change.
2. **Implement revision-backed inventory and comparison.** Share the parser,
   batch Git reads, preserve diagnostics, derive counts, and prove boundary cases.
   Generate the repository index and register its regeneration path.
3. **Generate the website.** Render the snapshot through the shared Markdown
   pipeline and publish JSON. Remove the authored backlog duplication. Verify
   full and partial builds, source links, discovery, and actual browser output.
4. **Integrate news drafting.** Update the canonical skill and writing checklist.
   Record revision boundaries in the weekly source. Prove the dry-run workflow
   without network publication and synchronize skill mirrors.
5. **Verify the complete workflow.** Run scoped native tests, the registered lint
   action after Go edits, documentation checks, site check, and browser proof.
   Independent review covers every AC and the false-completion cases. Report any
   unrelated red instrument without changing the roadmap's correctness contract.

### Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Population | Every bucket file appears once, including parking and malformed states. |
| History | Selected immutable trees supply every field. Renames and removals cannot imply delivery. |
| Single authority | No authored website card list or generated index supplies status facts. |
| Route ownership | Exactly one producer owns `/project/roadmap/`. Existing incoming URLs continue to work. |
| News | Pinned evidence precedes approval. Sending neither recollects facts nor changes the approved text. |
| Scope | No priority moves, release gate, completion percentage, or automatic Discord publication. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Repository list and structured report | Native list/update commands and fixture row reconciliation. |
| Public required/optional lists | Full and partial site builds, site check, browser inspection, matching JSON and mirror. |
| Shared news evidence | Pinned comparison and the weekly dry-run smoke scenario. |
| Discovery and skill distribution | Index lookup, native help, canonical skill synchronization, documentation checks. |

### Security Review Checklist

| Check | What to verify |
|-------|----------------|
| Revision arguments | Git receives argv, never interpolated shell input. Resolve commit objects and reject invalid references. |
| Publication boundary | Publish metadata fields only. Do not copy full specs, journal records, session artifacts, or `plan/` into site output. |
| Text and links | Escape metadata and constrain link targets to selected repository blobs. |
| Output safety | Build proofs use session scratch, never the published sibling or repository root as output. |
| Authorization | No live Discord send during implementation or verification. Exact text still requires owner approval. |

## Key Design Decisions

| Decision | Alternative considered | Rationale |
|----------|------------------------|-----------|
| Derive from existing buckets and headers | A separate curated release manifest | A second list would require manual reconciliation on every spec change. |
| Preserve the public roadmap route | Add a second release dashboard URL | Existing navigation and incoming links already reach the right subject. |
| Git trees provide historical snapshots | Commit another mutable status ledger | Git preserves source bytes and deleted specs without another lifecycle. |
| Report remaining work and endpoint changes | Infer percent complete or delivered count | The available facts do not establish either claim. |
| Fixed news prose before approval | Expand live roadmap tokens during publication | Approval applies to exact text and historical facts must remain fixed. |
| Untracked generated repository index | Commit a manually refreshed status table | Existing derived-index conventions avoid stale checked-in totals. |

## Known Limitations

The report measures the recorded queue. It does not prove that the queue contains
every release obligation or that an implementation meets its specification.
Spec sizes differ, so counts cannot estimate effort or a release date. Optional
means the first release does not depend on the item, never a promised later date.

## Checklist

### Pre-Spec Verification

- [ ] Metadata, source evidence, data flow, assumptions, and acceptance criteria are present.
- [ ] Bucket policy and weekly publishing restrictions are preserved.
- [ ] Integration and documentation rows have explicit applicability decisions.
- [ ] Proposed commands and files are distinguished from existing behavior.

### Goal Gates

- [ ] Every AC has observable evidence through its real entry point.
- [ ] Counts reconcile across repository, HTML, Markdown, and JSON for the same revision.
- [ ] The news dry run preserves approved historical text and sends nothing.
- [ ] Independent review is clean. Any broader gate result is reported exactly as observed.

## Review Gate

Implementation review has not run. Append `plan/TEMPLATE-CLOSURE.md` at closure.

| Run | Scope | Reviewer | Result | Evidence |
|-----|-------|----------|--------|----------|
