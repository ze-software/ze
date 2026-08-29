# Spec: site renderers in Go

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 2 of 10 |
| Deferral shard | `plan/deferrals/site-renderers-in-go.md` |
| Handoff | - |
| Updated | 2026-08-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Commit `eae282592` deleted `website/tools/` (46 files, 17,245 lines of Python)
and replaced the site build with `internal/le/site`. The replacement stages
`website/` verbatim and refreshes five surfaces: the command catalog, the
command-equivalent pages, `llms.txt`, the asset bundles, and the talk decks.
The retired build had 38 render steps. Thirty-three of them have no Go producer.

The site does not look broken, and that is the defect. `seedOrCleanArtifact`
seeds every build from the previous artifact, so each of the roughly 890 pages
the Python last wrote survives with frozen content and a fresh mtime.
`./le site check` cannot see it: it reports source-only leaks and missing
Markdown mirrors, and a seeded page satisfies both. The published site is
therefore a snapshot of the repository as it stood on 2026-08-27, and no edit to
`docs/`, `website/blog/posts/`, `website/changes/posts/` or `website/data/*.json`
can reach a reader.

The goal is that every published page is generated again from its source in this
repository, by `./le site build`, with no page surviving on the strength of the
seed alone.

The acceptance target is RENDERED parity, against the page currently published in
the sibling `gh-pages` worktree. Owner decision, 2026-08-29: "the exact escaping
does not matter much if the rendering reads the same."

So a restored renderer must produce a page that reads the same in a browser and
resolves the same links. Differences a reader cannot see are permitted, and three
classes of difference are named as permitted rather than left to judgement:
character-reference spelling (`&#x27;` against `&#39;` against a literal
apostrophe), whitespace and indentation between elements, and attribute-quoting
style.

Four things are NOT reader-invisible and stay binding.

| Binding | Why it is not cosmetic |
|---------|------------------------|
| Routes, and the Markdown mirror beside each one | a moved or missing route is a 404 |
| The redirect table and its application ORDER | the 177 stubs are applied each over the last one's output, so order decides which target a legacy URL reaches |
| Visible text, structure, and link targets | this is what "reads the same" means |

Heading anchor slugs are NOT binding, and neither is any other place goldmark
answers differently from python-markdown. Owner decision, 2026-08-29: "I do not
mind to break external link to better use the new library." So the renderer takes
goldmark's answer and no compatibility shim is written. The page's own table of
contents regenerates from the same slugifier and stays self-consistent; an
inbound `#anchor` link from outside this repository lands at the top of the page
instead of at its section.

This reaches further than the slugs. Where a Markdown construct READS
differently under goldmark, the correction goes into the source Markdown, not
into the renderer: the source is ours and the fix is one-time, where a shim is
permanent.

## Required Reading

### Architecture Docs
- [ ] `website/AI.md` - the site's own design document: source tree versus Pages
  artifact, the data files, the Markdown mirror contract, the page shell
- [ ] `docs/contributing/gh-pages.md` - how the artifact is published
- [ ] `docs/contributing/ze-go-style.md` - the standard for every line written here
- [ ] `ai/rules/simplicity.md` - 33 restored steps is the widest possible invitation
  to build machinery the problem does not need

**Key insights:** (minimal context to resume after compaction)
- The retired renderers are recoverable: `git show eae282592^:website/tools/<name>.py`.
- The published artifact is the golden fixture. Byte parity is mechanically
  checkable against the `gh-pages` worktree at its last Python-era commit.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/site/build.go` - stages sources, seeds from the previous
  artifact, refreshes command surfaces, CSS/JS and talks, removes source-only
  paths, stamps the footer
- [ ] `internal/le/site/paths.go` - the source/output contract and the
  source-only exclusion list
- [ ] `internal/le/site/pages.go` - discovers public routes from the ARTIFACT,
  not from a source-to-destination map
- [ ] `internal/le/site/actions.go` - the seven `./le site` actions

**Behavior to preserve:**
- The artifact tree and the source tree stay separate; the build never writes
  into `website/`.
- Source-only inputs never reach deployment.
- The footer publication stamp and its carry-over, restored in `ee9b23056`.

**Behavior to change:**
- A page is produced from its source on every build, rather than surviving from
  the seed.
- The command surfaces are published through the shared page shell. Today they
  are not, and this is worse than the staleness the rest of the spec addresses.
  `renderPrimaryCommandHTML`, `renderEquivalentIndexHTML` and
  `renderEquivalentHTML` (`internal/le/docvalid/command_render.go`) each open
  their output with a bare `<!doctype html><html><body>`: no head, no title, no
  stylesheet link, no header mount, no sidebar, no footer. The Python-era page
  committed at `gh-pages` HEAD carries the full shell. So the roughly 400
  `reference/` pages that the current build DOES regenerate are republished as
  unstyled fragments, and every one of them is a live regression rather than a
  stale page.

**Route inventory:** the published tree at `gh-pages` HEAD `2fa8fa2ad` holds 895
HTML files, 889 of them `index.html`.

| Population | Pages | Producer to restore |
|------------|-------|---------------------|
| Named by a `page_registry.py` map | 331 | `DOCS_MANIFEST` (115 rows), `HUB_PAGES` (5), `USE_CASE_PAGES` (9), `LAB_DETAIL_PAGES` (2), `COMPARE_PAGES` (3), `QUALITY_PAGES` (6), `NAV_PATCH_TARGETS` (12), and 179 redirect stubs |
| Produced by a data-driven renderer that consults no map | 564 | command equivalents 396, plugin catalog 97, weekly changes 38, blog 8, talks 6, and one page each from the catalog, config-reference, dependencies, homepage, activity, timeline, features, test-health, RFC-compliance and search-index renderers |
| Map destinations with no published page | 0 | - |

  → Constraint: the registry explains 37% of the site. The other 63% needs its
  renderer restored, and no map will recover it.
  → Constraint: `DOCS_MANIFEST` carries only a category per row. The title comes
  from front matter or the first H1, and the description is derived. So the
  restored manifest is a category map, not a page table.
  → Constraint: `rewrite_legacy_public_urls` applied the 177 URL redirects in
  dict insertion order, each over the output of the last. A Go map iterates
  randomly, so the redirect table MUST be an ordered slice or parity is lost.
  → Constraint: `is_frozen_talk_path` excludes any `talks/` path whose second
  segment is not exactly `index.html`. `talks/index.html` is patched;
  `talks/index.md` is frozen. The asymmetry is load-bearing.
  → Constraint: `page_root_for_dest` counts directory segments, never the file
  name, so a standalone page at the root gets an empty prefix.
  → Decision: `DOCS_MANIFEST` names `architecture/config/deprecated-options.md`
  twice. Python deduped silently; a Go map literal with a duplicate key does not
  compile, so the recovered manifest is deduped on the way in.

**The shared page shell:** `sitelib.page_head` and `sitelib.page_foot`, coupled
by the module global `_PENDING_PAGE_SIDEBAR`: the head computes the sidebar,
stashes it, and the foot consumes it.

  → Constraint: a Go port carries the shell as ONE value, not two calls, or the
  sidebar and the `has-page-sidebar` class on `<main>` disagree.
  → Constraint: `page_head` emits no canonical link and no `og:url`.
  `build.step_links` adds both afterwards through `sitelib.patch_canonical`,
  which anchors on the site.css link and inserts BEFORE it. A shell that emits
  them in place puts them in the wrong position.
  → Constraint: `<main>` takes `has-page-sidebar` when the sidebar is non-empty,
  else `site-main-wide` when the caller asked for wide, else no class. The
  sidebar wins.
  → Constraint: a sidebar group whose links all resolve to the current page is
  dropped, and a sidebar with no groups left returns empty, which removes the
  class again.

**The Markdown pipeline:** `render-doc.render` is seventeen ordered steps: front
matter, two token substitutions (plain for the mirror, `data-ze-stat` spans for
the HTML), title and category resolution, one `markdown.Markdown` instance per
page, the table of contents, link rewriting, the block-HTML mirror decision,
terminal-demo expansion, evidence-cell relayout, Yes/No cell colouring, external
link targets, the journey hero, the table of contents splice, and the shell.

  → Constraint: there is NO heading-anchor pass and NO table wrapper. Heading
  ids come only from the `toc` extension's slugifier, and `<table>` is emitted
  raw.
  → Constraint: `insert_doc_toc` splices after the FIRST literal `</div>` in the
  body. A Go port copies that rule, not its intent.
  → Constraint: the body is spliced at column zero into an indentation-sensitive
  shell, so a published page mixes indented chrome and unindented body.

**Escaping is measured, and settled as cosmetic.** The chrome uses Python
`html.escape` with `quote=True`, which writes `&#x27;` and `&quot;`. The Markdown
body uses Python-Markdown's serializer, which escapes only `&`, `<` and `>` in
text and leaves `'` literal. Go's `html.EscapeString` writes `&#39;` and `&#34;`
and so matches NEITHER. Over the published tree, the Python-era commit holds 681
`&#x27;` and zero `&#39;`; the working tree, after the Go build rewrote the
command pages, holds 226 and 143.

  → Decision: no Ze escaper is written. The owner ruled the spelling
  reader-invisible on 2026-08-29, and all three spellings render as one
  apostrophe. `internal/le/site/footer.go` keeps `html.EscapeString`.
  → Constraint: the `toc` slugifier DELETES punctuation rather than replacing
  it, does not strip leading or trailing separators, and disambiguates
  duplicates with `_1`, `_2` per page.
  → Constraint: the JSON-LD block is ordered, uses compact separators, and
  post-substitutes `</` to `<\/`. Go's encoder escapes `<` to its unicode
  escape unless `SetEscapeHTML` is turned off.

**A second live regression: `llms.txt`.**
`renderNativeCommandSurfaces` (`internal/le/docvalid/command_render.go`) writes
`llmsSurfaceName` unconditionally on every build, carrying the command surface
alone. Measured: the published file is 1035 lines at the last Python-era commit
and 399 lines in the working tree after the Go build. The retired
`render-llms-txt.py` emitted 18 sections from nav, features, dependencies, the
plugin registry, the YANG tree and every page mirror.

  → Constraint: two producers cannot own `llms.txt`. Either the restored
  renderer subsumes the docvalid one, or the two merge. This is a phase-ordering
  dependency, not a preference.

**The facts snapshot cannot be parity-checked against the published numbers.**
`internal/le/site/facts` already answers four of the roughly thirty keys
`sitefacts.build_facts` produced, and its answers DISAGREE with the published
ones: `repo.design_comments` reads 4299 against a published 3852,
`repo.go_packages` 755 against 687. The Go counters read `git ls-files` where
the Python walked the filesystem.

  → Decision: the facts page is checked against a re-derivation from the current
  tree, never against the published number. The published number is stale by
  construction.
  → Constraint: the published `data/site-facts.json` names
  `website/data/repo-facts.json` in its `_sources`, which the recovered
  `sitefacts.py` never writes. The last publish ran an uncommitted version of
  that script, so the PUBLISHED FILE is the contract and the recovered script is
  not.
  → Constraint: `sitefacts.github_stars` reaches api.github.com and, on any
  failure, keeps the number the previous artifact published. So a build's output
  depends on the network and on its own previous artifact, and a clean-room
  build cannot reproduce it. The restored producer states this or removes it.

**The plugin registry has a Go producer that is not a drop-in.**
`internal/le/inventory.Collect` reads `registry.All()` rather than parsing
`register.go`, which is the better source. It does not carry
`optional_dependencies`, `source_dir` or `yang_files`, it sees only what the
composition root imports (so the two test-only plugins in the published catalog
are missing), and it orders by plugin name where the Python ordered by
`register.go` path.

  → Decision: extend `inventory.Collect` rather than restore the regex
  extractor. Reading the registry is what `ai/rules/evidence.md` asks for, and
  the four deltas are additive.
  → Constraint: `PLUGIN.md` front matter is dead code. Zero such files exist and
  all 96 published entries carry an empty doc object. It is not restored.

**Determinism hazards that a Go port introduces on its own:**

  → Constraint: `sort.Slice` is NOT stable. The blog list and the search index
  both sort a pre-sorted glob by a second key and rely on the tie order, so both
  need `sort.SliceStable` or a compound key.
  → Constraint: Python compares a whole path string, so
  `firewall/plugins/irr/register.go` precedes `firewall/register.go`. A
  component-wise Go comparison gives a different order.
  → Constraint: `Counter.most_common` breaks ties on insertion order, which the
  RFC-compliance page depends on for its twelve-row gap table.

**The homepage lands last.** `render-index.py` reads `data/changes.json` and
`data/site-facts.json`, which two other steps produce, and the facts snapshot in
turn wants the command catalog and the YANG tree.

**Recovered inputs:** the 115-row manifest is written to
`tmp/session/2026-08-29-36ab4fc7-fd4f-4e52-892c-be2eaf79ddcd/scratch/docs-manifest.txt`.
Every deleted renderer is recoverable from `eae282592^`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le site build`, and the `website/` source tree plus `docs/*.md` it reads.

### Transformation Path
1. Source discovery over the tracked and untracked `website/` inputs.
2. Page rendering: Markdown or JSON to HTML through one shared page shell.
3. Mirror rendering: an `index.md` sibling for every published page.
4. Derived artifacts over the rendered tree: `llms.txt`, the search index, the
   sitemap, the redirects.
5. Staging into the artifact, source-only removal, footer stamping.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Source tree ↔ Pages artifact | `resolvePaths` refuses an output inside the source | No |
| Repository ↔ published claim | every rendered fact traces to a committed file | No |

### Integration Points
- `internal/le/docvalid` already renders the command surfaces; the restored
  renderers must not render them a second time.
- `internal/le/site/facts` already writes `website/data/repo-facts.json`.
- `internal/le/rfc` and `internal/le/testhealth` own the data the quality pages
  publish.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `gh-pages` worktree at its last commit is a faithful record of what the Python renderers produced | the working tree is dirty from a post-cutover build; only the committed state is Python-era output | the parity target is a moving one and every AC needs restating | compared `git show gh-pages HEAD` for `reference/cli/index.html` against the working tree: the committed page carries the full shell and a footer stamp reading 27 August 2026, the working-tree page is a bare fragment | confirmed |
| A-2 | Every page the maps name still has a source in this repository | the inputs the renderers read are mostly present, but `page_registry.DOCS_MANIFEST` and the redirect maps were themselves deleted | a published route has no producer and must be either re-sourced or retired, which is a scope decision for the owner | the route reconciliation: all 331 map destinations have a published page and every map is recovered from `eae282592^` | confirmed |
| A-3 | goldmark reproduces the reading of a python-markdown page for the roughly 194 Markdown-derived pages | `sane_lists`, the `toc` slugifier and the serializer each differ from CommonMark; the owner ruled character-level differences invisible | the first full build changes what a page SAYS rather than how it is spelled | render the ten most list-heavy and table-heavy published pages through both and compare visible text and link targets, before any bulk build | confirmed, with one source correction owed (see the A-3 measurement below) |
| A-4 | Changed anchor slugs are acceptable | owner decision, 2026-08-29 | an inbound `#anchor` link lands at the top of the page rather than at its section, silently | not applicable, this is a decision rather than an assumption | accepted |

**The A-3 measurement, 2026-08-29 (phase 2).** The ten pages were chosen by
counting `<li>` and `<tr>` in the published body of each of the 115 manifest
routes at `gh-pages` HEAD: the five heaviest by table row and the five heaviest
by list item. Each source was rendered through goldmark and through
python-markdown 3.10.3 with the four extensions the retired renderer used, and
the visible text, the link targets and the element counts were compared. Five
pages are identical. The five that differ fall into three classes, and no page
loses text under goldmark.

| Class | What python-markdown did | What goldmark does | Verdict |
|-------|--------------------------|--------------------|---------|
| `\|` inside a table cell | left the backslash visible, so a reader saw `accept\|reject\|modify` | unescapes it to a literal pipe, as GFM requires | goldmark fixes a visible defect; 30 occurrences on `docs/guide/command-reference.md` alone |
| a list with no blank line before it | left it as paragraph text, so four numbered lines ran together on one line | parses the list | goldmark fixes a visible defect; 3 of the 10 pages |
| an unescaped `\|` inside a code span inside a table row | did not split the cell | splits the cell as GFM requires, which breaks the code span and shows literal backticks | goldmark reads WORSE, and the fix is in the SOURCE |

The third class is the only regression, and the correction is one-time: write the
pipe as `\|`. Twenty lines carry it across `docs/` and `website/`, listed in
`tmp/session/2026-08-29-36ab4fc7-fd4f-4e52-892c-be2eaf79ddcd/scratch/a3/unescaped-pipes.txt`. They are NOT corrected here: the owner decides whether to correct the
source or accept the reading, and phase 3 is the first phase that publishes them.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | goldmark changes what a page SAYS, not how it is spelled, on a list or table python-markdown parsed differently | the phase 2 spike over the ten most list-heavy published pages shows differing visible text | settle it before any bulk rendering. If a construct genuinely diverges, fix the SOURCE Markdown rather than the renderer: the source is ours and the divergence is one-time |
| R-2 | A producer renders a route a second producer also claims, and the last one wins silently | the coverage check counts claims, so a duplicate claim is a red, not a race | AC-1 requires exactly one claimant, not at least one |
| R-3 | The first full build rewrites all 895 pages at once and the diff is unreviewable | phase 10 lands and `git status` in the artifact shows everything modified | each phase lands its own commit, so the artifact diff is reviewed one population at a time. The build is never run to completion until phase 10 |
| R-4 | The facts snapshot reaches the network during a build, so a build is not reproducible and can fail closed in CI | a build in a sandbox hangs for five seconds and publishes a stale star count | AC-11 requires the offline path to keep the previous value and say so; the timeout stays and the failure stays non-fatal |
| R-5 | Restoring `llms.txt` collides with the docvalid producer that owns it today | two writers, last one wins, and the file's content depends on producer order | phase 4 makes the command section one section of the restored file, and the coverage check refuses two claimants |
| R-6 | Running `./le site build` before phase 4 DESTROYS the published artifact | it already has: 826 paths degraded in the `gh-pages` working tree | NO phase may run the `./le site build` action from a shell until phase 4 lands. Commit `9f45348a7` made `refreshNativeSurfaces` call `docvalid.RenderCommandSurfaces` unconditionally, and that renderer emits a contract fixture for the drift checker rather than a publishable page, so a build overwrites 396 pages with fragments. A Go test may call `Build` because it builds into a temporary output. `gh-pages` HEAD is intact and the damage is working-tree only; the restore is the owner's, because `git restore` is forbidden here |
| R-7 | The retired renderers encode behaviour nobody wants back, and restoring them faithfully restores the mistakes | a restored page carries a fact nobody can trace, or a section referring to a retired tool | `ai/rules/evidence.md`: a claim on a published page traces to a committed file or it is not published. The RFC-compliance page's agent-guard block counts text in files that no longer exist and needs redefinition, not a port |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The published website. Nothing in the daemon, no operator-facing path, no protocol surface |
| How is it reverted? | Single commit revert; the artifact is republished from the previous build |
| Who else touches this path? | `spec-site-facts-from-committed-data` owns the build's data input; `spec-le-is-a-ze-binary` claims the obligation in AC-6/AC-9 |

## Design

### The shape: a producer registry, not a call list

The reason this regression was invisible for a day and would have stayed
invisible is not that somebody forgot a renderer. It is that nothing in the
repository could NAME the render steps. The completeness gate recorded the whole
38-step site build as one row, `{Target: "ze-site-generate", Area: "site", Verb:
"build"}`, so one existing action satisfied it. `./le site check` reports
source-only leaks and missing Markdown mirrors, and a page carried forward by the
seed satisfies both. A build that produced five surfaces out of thirty-eight
reported success every time it ran.

So the design's load-bearing part is an inventory, not a renderer. Every
published route is owned by a NAMED producer that registers itself, `Build`
discovers producers through the registry rather than calling them by name, and a
check refuses a published page that no producer claims. That is the repository's
own registration pattern (`ai/rules/plugins.md`, `ai/patterns/registration.md`)
applied to the thing that failed.

The renderers are then ordinary work behind one interface.

A producer does not DECLARE the routes it owns, it RETURNS the routes it wrote.
A separate declaration is a second statement of the same fact and drifts from the
first, which is the shape of the defect being fixed. Returning the written set
makes the claim self-verifying: coverage is the published routes minus the
written ones, and a route two producers write is caught by the same arithmetic
that catches a route none writes.

### Alternatives

| Approach | How it works | Trade-off |
|----------|--------------|-----------|
| **A. Producer registry in `internal/le/site`** (recommended) | Each page family registers a producer declaring the routes it owns and rendering them. `Build` iterates the registry. `./le site check` fails on a published route no producer claims | One package holds staging and rendering, which will pass 1000 lines and need splitting by concern. In exchange the coverage gap becomes mechanically visible, which is the defect being fixed |
| B. A separate `internal/le/siterender` package that `site` calls | Keeps `site` as staging only, cleaner package boundary | The call list moves rather than disappearing. Nothing counts the surfaces, so the same regression is available again the next time a producer is dropped |
| C. Restore the Python | Fastest to parity by a wide margin | Banned: `docs/contributing/ze-go-style.md` puts repository tooling in Go under `internal/le/`, and `eae282592` retired the interpreter deliberately |

A wins on the one criterion that matters here: it makes the failure that
happened impossible to repeat silently.

### What stays the same

The artifact and source trees stay separate, source-only inputs never deploy,
the seed still carries an artifact forward between builds, and the footer stamp
and its carry-over are untouched.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le site build` | → | `producers()` registry iterated by `Build` | `TestBuildRunsEveryRegisteredProducer` |
| `./le site check` | → | the published-route coverage check | `TestCheckRefusesAPublishedRouteWithNoProducer` |
| a `docs/*.md` source | → | the docs producer through the shared shell | `TestDocsProducerRendersAManifestRoute` |
| `website/data/features.json` | → | the features producer | `TestFeaturesProducerRendersEveryCard` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | any published route in the artifact | exactly one registered producer claims it; `./le site check` exits non-zero and names every route that none claims, and every route that two claim. It is armed from phase 1, so it is red for the duration of the work, and its message names this spec so another session does not diagnose it as its own breakage. It is not a verify stage (`internal/le/verify/engine/stages.go` registers `site facts check`, not `site check`), so the red blocks no commit |
| AC-2 | a full build over a tree whose sources are unchanged | every published page is rewritten by its producer, and no page survives on the strength of the seed alone |
| AC-3 | a page rendered by a restored producer | it carries the shared shell: head, theme bootstrap, canonical, header mount with its noscript fallback, `<main>` with the correct sidebar class, page sidebar, deferred script, footer with the publication stamp |
| AC-4 | a `docs/*.md` source named in the recovered manifest | it renders to its mapped route with the same visible text, structure and link targets as the published page, and an `index.md` mirror beside it |
| AC-5 | a Markdown source containing block HTML | its mirror is converted back from the rendered body rather than copied from the source |
| AC-6 | the command surfaces and `llms.txt` | they go through the shared shell, and `llms.txt` carries every section the retired renderer emitted, not the command section alone |
| AC-7 | `website/blog/posts/*.md` and `website/changes/posts/*.md` | index, detail pages and feeds render; two posts sharing a date order by filename |
| AC-8 | `website/data/features.json`, `milestones.json`, `dependencies.json`, `talks.json` | each renders its page in the data file's own order |
| AC-9 | the plugin catalog | it renders from `inventory.Collect`, extended to carry the optional dependencies, source directory and YANG files the page shows |
| AC-10 | the test-health and RFC-compliance pages | they render from `internal/le/testhealth` and `internal/le/rfc` rather than from the retired Python inputs |
| AC-11 | the facts snapshot | every published number is re-derived from the current tree, and a build with no network keeps the previously published star count and says so |
| AC-12 | the 177 legacy URLs | each resolves to the target the recovered table names, with the replacements applied in the recorded order. AC-1's coverage check CANNOT witness this: `pageRegistry` drops redirect pages through `isRedirectPage`, which is why phase 1 measured 712 unclaimed routes rather than 889. A green coverage is therefore not evidence for this row, and phase 10 owes it a test of its own |
| AC-13 | the search index, sitemap and robots file | each is regenerated from the built artifact rather than carried forward |
| AC-14 | a second build over an unchanged tree | the artifact is byte-identical to the first, network access aside |
| AC-15 | the built artifact | `llms-full.txt` is published beside `llms.txt`, carrying the full Markdown mirror of every published page, each preceded by its title and canonical URL. Frozen talk decks are excluded, as they are from every other mirror pass. The ORDER is the reading order stated below, never route order: what the software is and why it is worth evaluating comes first, how to use it comes second |
| AC-15a | the order of `llms-full.txt` | it follows `llms.txt`'s own curated section order, and the page bodies are grouped by the `website/data/nav.json` dropdowns in their declared order: Start, Evaluate, Docs, Examples, Reference, Project. One curation, two renderings: `llms.txt` links, `llms-full.txt` inlines |
| AC-15b | a page that belongs to no section, or to two | the build refuses it by name. A page is never appended to the end because nothing claimed it, and never emitted twice because two sections did |
| AC-15c | a section declared in the reading order that no page fills | the build refuses it by name, so a section that silently empties is a red rather than a gap a reader meets |
| AC-15d | `website/data/nav.json` reordered so a usage section precedes an evaluation section | the build refuses it, naming both sections. The reading order is a CONTRACT stated in the code, and nav.json supplies each section's membership and the order WITHIN a section. A menu is ordered for a menu; slaving the document's argument to it means a menu reshuffle silently rewrites what the file argues, with nothing to notice |
| AC-17 | the wiki section of `llms-full.txt` | it REFERENCES the Codeberg wiki rather than republishing it: each page's title, its public URL and a one-line summary. The wiki stays its own source of truth, which is what `spec-website-wiki-content-migration` settled on 2026-07-22 |
| AC-17a | the source of that section | `website/data/wiki.json`, committed, refreshed by its own `./le` action. The build reads only the committed file and never `../wiki`, so a machine without the sibling checkout builds the same artifact. A stale index is reported by the refresh action's check, never silently omitted |
| AC-17b | the ORDER and grouping of the wiki section | it comes from the wiki's `_Sidebar.md`, which is the curation this repository cannot generate. Measured 2026-08-29: 167 sidebar entries against 171 pages, zero sidebar links resolving to no page. Its groups are About, First Steps, Configuration, Operation, Interfaces, Plugins, Plugin Development, Chaos Testing, Blueprints, Development, Reference, which is already evaluation before usage |
| AC-17c | a wiki page the sidebar does not list | the refresh action refuses it by name, so it cannot become a silent omission in a committed artifact. Four exist today: `CLAUDE` and `command-catalog` are excluded deliberately, as agent instructions and a 302KB generated dump; `community-filters` and `telemetry` are genuine sidebar omissions and are reader content |
| AC-18 | an edit to `website/assets/css/site.css` or `website/assets/js/site.js` | it reaches the published artifact. `refreshNativeSurfaces` renders each only when the output file is ABSENT, so a seeded stylesheet is never refreshed from source |
| AC-16 | a named non-route artifact that disappears from the artifact | `./le site check` refuses it. The route check answers for pages; this answers for `llms.txt`, `llms-full.txt`, `sitemap.xml`, `robots.txt`, `data/search-index.json` and both `feed.xml` files, each of which is published, none of which is a route, and one of which lost seventeen of its eighteen sections without any check noticing |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckRefusesAPublishedRouteWithNoProducer` | `internal/le/site/producer_test.go` | AC-1 | pass |
| `TestCheckRefusesARouteTwoProducersWrote` | `internal/le/site/producer_test.go` | AC-1, the doubly-claimed half | pass |
| `TestProducerRecordsAreKeyedByArtifact` | `internal/le/site/producer_test.go` | AC-1, two artifacts keep separate records | pass |
| `TestAnArtifactWithNoRecordIsFullyUnclaimed` | `internal/le/site/producer_test.go` | AC-1, an absent record fails closed | pass |
| `TestBuildRunsEveryRegisteredProducer` | `internal/le/site/producer_test.go` | AC-2 | pass |
| `TestShellCarriesEveryChromeElement` | `internal/le/site/shell_test.go` | AC-3 | pass |
| `TestShellPutsCanonicalBeforeTheStylesheet` | `internal/le/site/shell_test.go` | AC-3 | pass |
| `TestSidebarClassFollowsAnEmptySidebar` | `internal/le/site/shell_test.go` | AC-3 | pass |
| `TestPageSidebarDropsAGroupThatResolvesToThisPage` | `internal/le/site/shell_test.go` | AC-3, the rule that empties a sidebar | pass |
| `TestPageLinksLoadFromTheRepositorysOwnData` | `internal/le/site/shell_test.go` | AC-3, the sidebar data parses | pass |
| `TestMarkdownReadsTheSameAsThePublishedPage` | `internal/le/site/markdown_test.go` | AC-4, validates A-3 | pass |
| `TestEveryDocsSourceRendersAndItsContentsResolve` | `internal/le/site/markdown_corpus_test.go` | AC-4 over the whole corpus, 447 pages | pass |
| `TestFrontMatterSplitsMetadataFromTheBody` | `internal/le/site/markdown_test.go` | AC-4, malformed metadata is refused | pass |
| `TestDocTOCGoesAfterTheFirstClosingDiv` | `internal/le/site/markdown_test.go` | AC-4, the literal splice rule | pass |
| `TestMirrorConvertsBackWhenTheSourceHoldsBlockHTML` | `internal/le/site/mirror_test.go` | AC-5 | pass |
| `TestMirrorMatchesThePublishedConversion` | `internal/le/site/mirror_test.go` | AC-5, byte parity with one published mirror | pass |
| `TestMirrorIsWrittenBesideThePage` | `internal/le/site/markdown_test.go` | AC-5, the mirror sits at the page's own URL | pass |
| `TestAnAssetEditReachesTheArtifact` | `internal/le/site/assets_build_test.go` | AC-18 | pass |
| `TestTheCommandPageRendererKeepsItsAbsentOnlyGuard` | `internal/le/site/assets_build_test.go` | AC-18, the other half of the asymmetry | pass |
| `TestBlogPostsSharingADateOrderByFilename` | `internal/le/site/blog_test.go` | AC-7 | |
| `TestPluginCatalogCarriesTheFieldsThePageShows` | `internal/le/site/plugins_test.go` | AC-9 | |
| `TestFactsSnapshotKeepsTheStarCountOffline` | `internal/le/site/facts/sitefacts_test.go` | AC-11 | |
| `TestRedirectsApplyInTheRecordedOrder` | `internal/le/site/redirect_test.go` | AC-12 | |
| `TestLLMSFullCarriesEveryPublishedMirror` | `internal/le/site/derived_test.go` | AC-15 | |
| `TestLLMSFullPutsEvaluationBeforeUsage` | `internal/le/site/derived_test.go` | AC-15a, AC-15d | |
| `TestLLMSFullRefusesAnUnsectionedPage` | `internal/le/site/derived_test.go` | AC-15b | |
| `TestLLMSFullRefusesAnEmptySection` | `internal/le/site/derived_test.go` | AC-15c | |
| `TestCheckRefusesAMissingNamedArtifact` | `internal/le/site/producer_test.go` | AC-16 | |
| `TestASecondBuildChangesNothing` | `internal/le/site/site_test.go` | AC-14 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestBuildRendersEveryPublishedRoute` | `internal/le/site/site_test.go` | a full build over the real checkout leaves no route unclaimed | |

## Files to Modify
- `internal/le/site/build.go` - iterate the producer registry
- `internal/le/site/actions.go` - the coverage check joins `./le site check`
- `internal/le/site/pages.go` - route discovery gains producer ownership
- `internal/le/docvalid/command_render.go` - the command surfaces render through the shared shell
- `internal/le/inventory/inventory.go` - carry the three fields the plugin catalog shows
- `internal/le/completeness_record_test.go` - the site row stops standing for 38 steps
- `go.mod`, `vendor/` - goldmark
- `website/AI.md` - it describes renderers that did not exist; it will describe these
- `ai/INDEX.md` - there is no row for `./le site build` at all

## Files to Create
- `internal/le/site/producer.go` - the registry and the coverage check
- `internal/le/site/shell.go` - the shared page shell
- `internal/le/site/markdown.go` - the goldmark pipeline
- `internal/le/site/mirror.go` - the `index.md` sibling
- `internal/le/site/nav.go` - `assets/header.html` and the page sidebar
- `internal/le/site/docs.go` - the manifest-driven docs producer
- `internal/le/site/blog.go`, `changes.go`, `datapages.go`, `plugins.go`, `config.go`, `quality.go`, `home.go`, `derived.go`, `redirect.go` - one producer each
- `internal/le/site/testdata/` - published pages as golden fixtures

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | developer tooling; the daemon's configuration is untouched |
| YANG validation constraints | N-A | no YANG leaf added |
| YANG custom validators | N-A | no YANG leaf added |
| CLI commands/flags | Yes | `internal/le/site/actions.go`: `check` gains the coverage report; no new verb |
| CLI grammar (keyword before value) | Yes | the existing `./le site` action table already carries keyword parameters |
| Editor autocomplete | N-A | `./le` actions are not editor config |
| Functional test for new RPC/API | N-A | no RPC or API; the end-to-end proof is `TestBuildRendersEveryPublishedRoute` over the real checkout |
| Pipe completeness | Yes | `./le site check` answers structured data, so the existing `leaction` answer path renders it through every pipe |
| Env var registration | N-A | the build takes its publication time from `buildClock`, not from an environment variable |
| Doctor check for runtime dependencies | N-A | no new file path, socket, port, module or binary at daemon runtime. The build's one external reach, the GitHub star count, already exists and already fails soft |
| Prometheus counters/metrics | N-A | build-time tooling emits no daemon metric |
| BGP family surface | N-A | no protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | the site's pages are restored, not added; `docs/features.md` describes the daemon |
| 2 | Config syntax changed? | No | no configuration surface |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` covers `ze`, not `le`; the `./le site` surface is documented in `ai/INDEX.md`, which has NO row for `./le site build` today and gains one |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | the plugin catalog is republished, the plugins are untouched |
| 6 | Has a user guide page? | Yes | `docs/contributing/gh-pages.md`, the route `ai/INDEX.md` gives for editing the website |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | the RFC-compliance PAGE is republished; no requirement's proof changes |
| 10 | Test infrastructure changed? | No | `docs/functional-tests.md` covers the `.ci` runner |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` is a source the site reads, not a thing this changes |
| 12 | Internal architecture changed? | Yes | `website/AI.md` is the site's design document and currently describes renderers that do not exist. Also `docs/architecture/core-design.md`, declared by `internal/le/docvalid/command_render.go`'s `// Design:` header: its claim that published command surfaces come from the live catalog stays true, and the change is that they now render through the shared site shell, so the line is checked and amended if it says otherwise |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `internal/le/inventory` gains three fields, so `docs/plugin-overview.md` and `docs/features/plugins.md` are checked against the republished catalog |
| 16 | Any changed source file referenced by existing doc source anchors? | DERIVED | resolve with `./le spec citation anchors spec plan/spec-site-renderers-in-go.md` at implementation time; `internal/le/site/build.go` and `paths.go` both declare `// Design: website/AI.md`, which row 12 already names |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `website/AI.md` shows a `./le site build` workflow whose steps no longer exist |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the producer registry and the coverage
   check, with no producer registered yet
   - Tests: `TestBuildRunsEveryRegisteredProducer`,
     `TestCheckRefusesAPublishedRouteWithNoProducer`,
     `TestCheckRefusesARouteTwoProducersWrote`
   - Files: `producer.go`, `build.go`, `actions.go`
     → Decision 2026-08-29 (owner): the record naming which producer wrote which
     route lives in the CHECKOUT at `tmp/site/producers-<artifact digest>.json`,
     never in the artifact. The artifact is published, so a bookkeeping file
     written there is served to the public, which is how
     `plan/verification-debt/c7beceff.md` reached the live site. Adding the path
     to `sourceOnlyFiles` is NOT the fix: `removeSourceOnly` runs after the
     producer pass, so the record would be written and then deleted, and the
     next check would call every claimed route unclaimed.
   - Verify: `./le site check` goes red and names all 712 published routes as
     unclaimed. That red IS the defect, stated for the first time
     → Correction 2026-08-29: 712, measured, not the 895 predicted above.
     `pageRegistry` counts a non-redirect `index.html` only, and the artifact
     holds 889 `index.html` of which 177 are redirect stubs. The check therefore
     cannot see the 177 stubs phase 10 restores, so phase 10 needs its own
     evidence for AC-12 and MUST NOT read a green coverage as proof of them.
     `pages.go` needed no change: `pageRegistry` already answers the route set
     the coverage arithmetic subtracts from.
2. **Phase: Shell and Markdown** -- the shared page shell, the goldmark
   pipeline, the mirror writer, the nav header and the page sidebar. Validate A-3
   FIRST against the ten most list-heavy and table-heavy published pages, before
   any bulk rendering
   - Tests: `TestShellCarriesEveryChromeElement`,
     `TestShellPutsCanonicalBeforeTheStylesheet`,
     `TestSidebarClassFollowsAnEmptySidebar`,
     `TestMarkdownReadsTheSameAsThePublishedPage`,
     `TestMirrorConvertsBackWhenTheSourceHoldsBlockHTML`
   - Files: `shell.go`, `markdown.go`, `mirror.go`, `nav.go`, `go.mod`, `vendor/`
   - Verify: a hand-picked published page round-trips with the same visible text
3. **Phase: Docs** -- the 115-row manifest, the hubs, use-cases, lab details,
   compare and quality families, and the one-shot Markdown pages
   - Files: `docs.go`
   - Verify: roughly 160 routes leave the unclaimed list
4. **Phase: Command surfaces** -- the existing docvalid producer renders through
   the shared shell, and `llms.txt` gains back its other seventeen sections
   - State at HEAD, verified by reading `refreshCommandSurfaces`
     (`internal/le/site/build.go`) after commit `219dd162a`: the CATALOG has a
     producer and the PAGES do not. `data/cli-commands.json` is written from
     `liveCommandCatalog` on every build, unconditionally; the page render sits
     behind a guard that fires only when `reference/cli/index.html` is absent.
     That split is deliberate and is the `yang` session's repair of `9f45348a7`:
     the catalog keeps a live producer while the pages wait for this phase,
     rather than the drift check being laundered green by a fixture. So phase 4
     replaces the guard with a real page producer; it does not remove it and
     leave `docvalid.RenderCommandSurfaces` publishing
   - Files: `internal/le/docvalid/command_render.go`, `derived.go`
   - Verify: both live regressions are closed
5. **Phase: Blog and changes** -- index, detail pages, both feeds, `changes.json`
   - Files: `blog.go`, `changes.go`
6. **Phase: Data pages** -- features, milestones, dependencies, the talks listing
   - Files: `datapages.go`
7. **Phase: Plugin catalog and config reference** -- extend `inventory.Collect`,
   then render both pages
   - Files: `plugins.go`, `config.go`, `internal/le/inventory/inventory.go`
8. **Phase: Quality pages** -- test-health and RFC-compliance, re-sourced
   - Files: `quality.go`
9. **Phase: Facts and homepage** -- the facts snapshot, then the homepage that
   depends on it
   - Files: `internal/le/site/facts/`, `home.go`
10. **Phase: Derived** -- the search index, the sitemap, the robots file,
    `llms-full.txt`, and the 177 redirect stubs in their recorded order. The
    coverage check extends from routes to the named non-route artifacts
    - Tests: `TestRedirectsApplyInTheRecordedOrder`,
      `TestLLMSFullCarriesEveryPublishedMirror`,
      `TestCheckRefusesAMissingNamedArtifact`
    - Files: `derived.go`, `redirect.go`, `producer.go`
    - Verify: the unclaimed list is empty and `./le site check` goes green

### Triple Challenge

| Challenge | Answer |
|-----------|--------|
| Simplicity | The registry is the only machinery added, and it exists to make the failure visible rather than to organise code. Everything else is one function per page family, which is what the retired build had. The alternative that adds less, a plain call list, cannot answer AC-1 |
| Uniformity | Registration is the repository's unifying pattern, used by families, capabilities, CLI commands, config validators and web routes. This applies it to published surfaces |
| Performance | A build is developer tooling that runs on a workstation, not a wire path. The one constraint that matters is that a build stays reproducible, which AC-14 states |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has an implementation, and the unclaimed-route count is zero |
| Coverage honesty | the check counts CLAIMS, so a route claimed twice is as red as a route claimed never |
| Evidence | every fact a restored page publishes traces to a committed file. A number carried over from the retired Python because the page had it is not evidence |
| Determinism | no output order comes from a Go map, and every sort that relies on a tie order uses a stable sort or a compound key |
| Data flow | the build writes only into the artifact, never back into `website/` |
| Rule: `ai/rules/simplicity.md` | each producer is one function per page family. An abstraction shared by fewer than three producers is not earned |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Every published route has exactly one producer | `./le site check` exits zero |
| No page survives on the seed | build twice into different outputs from a clean artifact and diff |
| The two live regressions are closed | the command pages carry the shell, and `llms.txt` carries its sections |
| The completeness census stops standing for 38 steps with one row | `internal/le/completeness_record_test.go` names the surfaces, not one target |
| A build is reproducible | `TestASecondBuildChangesNothing` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | every input is a file in this repository, so the exposure is path handling: a source path must not escape the source root, and a rendered route must not escape the artifact root |
| Untrusted content | Markdown sources are trusted, but the renderer must not pass raw HTML through into a page from a source that is not ours. Today none is |
| Resource exhaustion | the build is developer tooling with a bounded input set; no per-request path |
| Information leakage | the artifact must carry no repository-internal file. `plan/verification-debt/c7beceff.md` is published today and is exactly this failure |

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

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One new spec rather than extending `spec-le-is-a-ze-binary` | extend that spec; three separate specs | Owner decision, 2026-08-29. That spec sits at its closure gate with 15 of 15 steps landed, and its completeness test measures retired Make target names, which these render steps never were |
| Rendered parity against the published artifact | byte parity; structural equivalence with fresh markup | Owner decision, 2026-08-29, in two steps. Parity against the published page was chosen first, then relaxed from bytes to rendering: "the exact escaping does not matter much if the rendering reads the same". Byte parity would have forced a Ze reimplementation of Python's two escaping regimes and of python-markdown's non-CommonMark list handling, to buy a difference no reader sees. Anchor slugs, routes, mirrors and the redirect order stay binding because each one breaks a link rather than changing an appearance |
| `github.com/yuin/goldmark` for Markdown | write a renderer for the four extensions the Python used | Owner decision, 2026-08-29 |
| goldmark's own behaviour wins wherever it differs from python-markdown, breaking external links included | write compatibility shims: a python-markdown slugifier, its escaping regimes, its non-CommonMark list handling | Owner decision, 2026-08-29: "I do not mind to break external link to better use the new library". This governs the whole pipeline, not the slugs alone. Where a construct reads differently, the SOURCE Markdown is corrected, because the source is ours and the correction is one-time; a shim would be permanent. In-page tables of contents regenerate from the same slugifier and stay self-consistent |
| `llms-full.txt` carries every published mirror, the 396 command-equivalent pages included | restrict it to the documentation pages | Owner request, 2026-08-29. This is NEW work: no `llms-full.txt` has ever been published and the retired Python never wrote one. Measured before deciding: the mirrors total 3.8MB across 709 pages, of which the 396 command-equivalent pages are 148KB and the 97 plugin pages 87KB. Excluding them would save 6% of the file and cost a reader the command reference, so the simple rule wins |
| The CSS and JS renderers lose their absent-only guard; the page renderers keep theirs until they produce pages | remove every absent-only guard together; leave all of them | The guard is right for a producer that emits a FIXTURE and wrong for one that emits the real artifact. `renderCSS` expands the `@import` chain and minifies a real stylesheet, so refreshing it on every build is what makes a CSS edit reach a reader; `TestRenderCSSExpandsNestedImports` already pins that output. The command-page renderer emits a drift-checker fixture, and removing ITS guard in `9f45348a7` overwrote 396 published pages with fragments. Same shape, opposite answer, which is why they are decided separately rather than as a policy |
| The reading order is a contract in the code; nav.json supplies membership and within-section order | derive the whole order from nav.json | Owner directive, 2026-08-29: the code must ENSURE the file is generated logically, not merely happen to emit it that way. nav.json is ordered for a menu, so slaving the document's argument to it means a menu reshuffle silently rewrites what the file argues, with nothing to notice. Stating the order in the code and refusing a nav.json that contradicts it makes the guarantee mechanical: an unsectioned page, a duplicated page, an empty section and an inverted evaluate/use split are each a named red rather than a silent reordering |
| `llms-full.txt` reuses `llms.txt`'s curation for its order | order by route; invent a second ordering for this file alone | Owner directive, 2026-08-29: features and what makes the software worth evaluating come first, how to use it second. That ordering already exists and is already committed. The Python-era `llms.txt` opens with Product snapshot, Quality and verification model, Comparison positioning and Feature inventory, and only then reaches Configuration model, Plugin registry and the CLI surface; its page map groups by the six `nav.json` dropdowns, Start and Evaluate before Docs, Examples and Reference. A second ordering would be a second thing to keep true |
| The docs pipeline is the first population after the shell | retrofit the two live regressions first | Owner decision, 2026-08-29. It is the largest population and the one that exercises goldmark hardest, so the Markdown unknowns surface earliest. The two regressions ship for longer as a result |

## Known Limitations
- Anchor links from outside this repository into a docs page will land at the top
  of the page rather than at their section. Accepted by the owner, 2026-08-29.
- `PLUGIN.md` front matter is not restored. No such file exists and all 96
  published catalog entries carry an empty doc object.
- The RFC-compliance page's agent-guard block counted text in a hook file, a
  Makefile and a status script that no longer exist. It is redefined against the
  registered action table rather than ported.
- The star count still reaches api.github.com and still falls back to the
  previously published number, so one figure on the site is not derivable from
  the tree alone. Changing that is a separate decision.
- `plan/verification-debt/c7beceff.md` is published on the live site and no map
  explains it. Removing a published file is the owner's call and is not done
  here.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
