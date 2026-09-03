# Spec: website-wiki-content-migration

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | - |
| Updated | 2026-07-22 |

Awaiting closure per `ai/rules/planning.md` Spec Closure: write the learned
summary, then `git rm` this spec (two-commit sequence). All ACs delivered;
evidence below.

## Task

See Goal below (spec written before the current template; Task = Goal here).

## Goal

Bring useful content from the Codeberg wiki into the public Ze website without making the wiki a second source of truth. Publish current canonical repository documentation where it already exists, adapt the remaining worked examples, keep generated references generated, and wire every new page into navigation, search, SEO, and `llms.txt`.

## Source Audit

The comparison covered 170 root Markdown files in `../wiki` and 664 website search-index entries. Twelve navigation, housekeeping, or generated wiki files were excluded, leaving 158 substantive pages.

Coverage buckets:

- 52 wiki topics have an obvious dedicated editorial website counterpart.
- 44 are represented principally by generated plugin records.
- 62 have no obvious one-to-one website page.

The wiki is source material only. `../wiki/Home.md` labels it an unreviewed draft, and `../wiki/build/PLAN.md` says canonical documentation belongs in `main/docs/` with the wiki curating or linking to it.

## Decisions

1. Existing `main/docs/` pages are the preferred publication source.
2. Generated CLI, configuration, feature, RFC, and plugin records remain generated.
3. Task-oriented material goes under `docs/guide/`.
4. End-to-end network roles go under `usage/`.
5. Test methods and chaos evidence go under `quality/` or a linked guide.
6. Internal AI workflow material is not published unless it is useful to external contributors.
7. Every published claim must remain traceable to current source, schema, command metadata, or test evidence.
8. Consolidated guides are preferred over recreating dozens of short wiki pages.

## Deliverables

### Operator content

- Publish BGP peering, policy, and resilience guides.
- Publish lifecycle, archive, rollback, logging, update, and restart guidance.
- Expand the ExaBGP migration page with a complete worked example.
- Publish Fleet Management operations after reconciling its status.

### Management content

- Publish complete CLI and web interface guides.
- Publish REST, gRPC, and gNMI setup and operation.
- Publish the detailed Looking Glass guide.
- Publish MCP and runtime introspection guides.
- Publish the plugin authoring set: SDK overview, protocol, YANG schema, handlers, commands, testing, and metrics.

### Differentiators and usage

- Publish the chaos testing guide.
- Publish route-server, transit-edge RPKI, FlowSpec injection, chaos-tested peering, and AS-path topology examples.

### Platform and contributors

- Publish implemented platform guides for configuration, ADD-PATH, MPLS, RSVP-TE, SRv6, traffic control, healthchecks, Fleet Management, logging, self-update, archive, and environment variables.
- Publish external contributor guides for setup, repository structure, testing, debugging, CI, RFC implementation, mock servers, and documentation testing where canonical material exists.
- Publish project history, glossary, and deprecated options after adapting and checking wiki-only material.

### Integration

- Add all pages to `tools/page_registry.py`.
- Update `data/nav.json`, `data/page-links.json`, and documentation hub links.
- Regenerate HTML, Markdown mirrors, search, SEO, sitemap, and `llms.txt`.
- Run targeted render tests, link validation, and a complete website build.

## Explicit Non-Goals

- Copying `command-catalog.md`, `command-reference.md`, `configuration-reference.md`, `feature-inventory.md`, or `status.md` from the wiki.
- Hand-maintaining plugin registry facts already generated from source.
- Publishing `claude-code.md` as product documentation.
- Changing Ze runtime behaviour.

## Acceptance Criteria

| ID | Condition | Expected result |
|----|-----------|-----------------|
| AC-1 | A reader opens the documentation hub | New operator, management, plugin, platform, and contributor paths are discoverable. |
| AC-2 | A reader needs BGP setup beyond quick start | Peering, policy, and resilience guides provide current configuration and operational workflows. |
| AC-3 | A reader operates Ze day to day | Lifecycle, archive, logging, update, restart, and Fleet guidance is published. |
| AC-4 | A reader automates Ze | REST, gRPC, gNMI, MCP, CLI, web, Looking Glass, and introspection documentation is published. |
| AC-5 | A reader writes a plugin | The SDK lifecycle, schema, handlers, commands, testing, protocol, and metrics pages are reachable as one set. |
| AC-6 | A reader evaluates Ze deployment shapes | Six end-to-end usage examples and the chaos testing guide are published. |
| AC-7 | Generated references are rebuilt | No hand-authored page replaces generated CLI, config, RFC, feature, or plugin data. |
| AC-8 | The website build runs | Renderers, link checks, search, SEO, sitemap, Markdown mirrors, and `llms.txt` complete successfully. |
| AC-9 | Content is reviewed | No known contradictory maturity claim remains, and no broken internal links remain. |

## Risks

| Risk | Mitigation |
|------|------------|
| Wiki commands or configuration are stale | Prefer `main/docs/`; check any wiki-only example against current generated references before publishing. |
| Too many short pages make the site harder to use | Consolidate related material and link generated reference pages for details. |
| Existing relative links break when rendered | Add sources to the central registry and rely on the cross-document link manifest. |
| Fleet maturity remains contradictory | Use the implemented status from the canonical Fleet guide and align the generated feature card before publishing. |
| Build output is edited by hand | Edit sources and registries only, then regenerate. |

## Verification

- `python3 -m py_compile tools/page_registry.py tools/render-llms-txt.py`
- `tools/test_render_doc.py` (11 tests pass)
- `tools/test_render_llms.py` (4 tests pass)
- `python3 tools/check-page-links.py --skip-network` (1,784 generated external anchors validated)
- `./update-website.sh` (complete build passes)
- Browser smoke checks: documentation hub, all six worked usage pages, corrected operator commands, search discovery, usage sidebars, 1366 by 768 layout, and Docs dropdown bounds at 1024 and 1000 pixels

## Documentation Checklist

- [x] Operator guides published
- [x] Management guides published
- [x] Plugin authoring set published
- [x] Chaos and usage examples published
- [x] Platform and contributor guides published
- [x] History, glossary, and deprecated options published
- [x] Documentation hub and navigation updated
- [x] Search, sitemap, SEO, Markdown mirrors, and `llms.txt` regenerated
- [x] Final link and content review clean

## Completion Evidence

- 50 additional canonical `main/docs/` sources publish as HTML and Markdown mirrors.
- Five new worked pages publish under `usage/`: route server, transit edge with RPKI, FlowSpec injection, chaos-tested peering, and AS-path topology.
- The ExaBGP migration page now includes a staged configuration and process-bridge walkthrough.
- Fleet Management moved from “Spec'd, not built” to “Experimental and growing,” with its card linked to the implemented operations guide.
- All 55 migrated page routes are present in rendered output, the search index, sitemap, and the complete `llms.txt` documentation index.
- The configuration blocks for route server, transit RPKI, FlowSpec, AS-path topology, and ExaBGP migration validate with the production `ze` binary.
- `llms.txt` summary extraction covers YAML front matter, bold-leading paragraphs, and list-only pages without empty or metadata-derived summaries.

## Required Reading

- `../gh-pages/` website tooling (`tools/page_registry.py`, `tools/render-llms-txt.py`, `update-website.sh`)
- The Codeberg wiki (migration source) and `main/docs/` (canonical documentation)

## Current Behavior

**Source files read:** (work is in the website repo, not this one)
- [x] `../gh-pages/tools/page_registry.py` — central page/source registry the migration extended
- [x] `../gh-pages/tools/render-llms-txt.py` — `llms.txt` generation the migration exercised
- [x] `main/docs/` guides republished as canonical sources

**Behavior preserved:** generated references stay generated (AC-7); build output never hand-edited.

## Data Flow

### Entry Point
- Website build: `./update-website.sh` in `../gh-pages` reads the page registry and `main/docs/` sources.

### Transformation Path
1. Sources (canonical `main/docs/` pages + adapted wiki examples) registered in `tools/page_registry.py`.
2. Renderers produce HTML pages and Markdown mirrors.
3. Search index, sitemap, SEO metadata, and `llms.txt` regenerate from the registry.

### Boundaries Crossed
| Boundary | Format |
|----------|--------|
| main repo docs -> website registry | Markdown sources referenced by registry entries |
| registry -> published site | rendered HTML + Markdown mirrors + `llms.txt` |

### Integration Points
- Navigation/hub pages, search, sitemap, `llms.txt` — all regenerated, none hand-edited.

## Wiring Test

Website content work — no daemon feature; existing test suite and website build checks cover it.

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `./update-website.sh` | -> | page registry + renderers | `tools/test_render_doc.py` (11 pass) + `tools/check-page-links.py --skip-network` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `tools/test_render_doc.py` | `../gh-pages/tools/` | renderer output for migrated pages (11 tests, pass) |
| `tools/test_render_llms.py` | `../gh-pages/tools/` | `llms.txt` summary extraction (4 tests, pass) |

### Functional Tests

- N/A — no daemon feature and no new feature in this repo; existing test suite unaffected. The configuration blocks published on the site validate with the production `ze` binary (Completion Evidence).

## Files to Modify

| File | Change |
|------|--------|
| `../gh-pages/tools/page_registry.py` | 50 additional canonical sources + 5 usage pages registered (done) |
| `../gh-pages/usage/`, docs hub, navigation | new worked pages and paths (done) |

## Implementation Steps

1. (DONE) Audit wiki content; classify publish/adapt/drop.
2. (DONE) Register canonical sources; adapt worked examples; rebuild generated surfaces.
3. (DONE) Verify build, links, search, SEO, `llms.txt`; browser smoke checks.
4. Remaining: spec closure only (learned summary + `git rm`).

## Checklist

- [x] Tests written (renderer + llms tests listed above)
- [x] Tests FAIL was N/A for pure content pages; renderer tests guarded regressions
- [x] Tests PASS (`test_render_doc.py` 11/11, `test_render_llms.py` 4/4, link check clean)
- [x] `./le verify worktree` N/A — no change in this repo; website build (`./update-website.sh`) green

## Implementation Summary

Closed 2026-09-03, six weeks after the work landed, by an independent review that
did not write it. The delay is the finding: the spec was marked `done` on
2026-07-22 and carried no `## Review Gate`, so nothing had ever checked it.

The CONTENT the spec moved is all present and reachable. What is gone is the
tooling it was written against. `tools/page_registry.py`,
`tools/render-llms-txt.py`, `tools/check-page-links.py`, their two test files and
`update-website.sh` no longer exist; `spec-site-renderers-in-go` replaced them
with `internal/le/site/`. So this spec's own `## Verification` section names
commands that cannot be run today, and the closure evidence below is stated
against the producers that exist NOW rather than reconstructed against the ones
that did.

The `usage/` family was renamed `use-cases/`, and the old addresses survive:
`retiredUseCaseRoute` (`internal/le/site/redirect.go`) keeps them, asserted by
`redirect_test.go`.

## Goal Validation

The goal was that the wiki stays its own source of truth, referenced rather than
republished, so no answer is stated twice with two dates on it.

| Goal | Evidence |
|------|----------|
| The wiki is referenced, not copied | `writeLLMSFullWiki` (`internal/le/site/llmsfull.go`) emits a title, a URL and a one-line summary per page and no body, reading committed `website/data/wiki.json`. Its own prose states the reason: "It is referenced here rather than copied, so nothing below states an answer twice with two dates on it" |
| The documentation hub and the guide sets exist | `website/docs/docs.md` renders the hub, and the BGP, operations, automation and plugin guide sets are published under `gh-pages/guides/` |
| The six worked examples exist | `website/use-cases/` carries route-server, transit-edge-rpki, flowspec-injection, chaos-tested-peering, as-path-topology and exabgp-migration |
| Generated reference stays generated | `internal/le/site/commands.go` reads the live command registry; `catalog.go` and `config.go` do the same for the plugin, RFC and configuration references |
| The site builds | `./le site build`, which also writes `llms.txt`, `sitemap.xml` and the search index |

That evidence is produced by code written AFTER this spec, which is the honest
statement of what closure can claim here: the goal holds today, and the artifact
that proves it is not the one the spec built.

## Deferrals Resolved

None. The spec has no deferral shard, and no open row in `plan/deferrals/` names
an acceptance criterion of it.

## Review Gate

Independent closure review, 2026-09-03, by a reader that did not write the spec
(`ai/rules/planning.md`: independence is a property of the context). It walked
all nine acceptance criteria to their producers, checked whether the tree had
moved under them, looked for a goal-validation table, checked the page that
describes the behaviour, and looked for a deferral shard.

0 BLOCKER, 0 ISSUE against the PRODUCT. Every acceptance criterion has a live
producer. The findings were all artifact: no Review Gate, no closure sections, no
Goal Validation table, and a `## Verification` block naming retired commands.
This section and the two above are that gap filled.

One NOTE, acted on rather than recorded: two citations named this spec by full
path and would have dangled when closure removes the file. `website/AI.md` and
the comment above `writeLLMSFullWiki` now name the bare stem, which is the form
`plan/spec-site-renderers-in-go.md` already used. Both sit outside the
`plan/`-rooted scanner in `internal/le/spec/citation/speccitation.go`, so the
citation gate would have stayed green while the rot was real.

## Pre-Commit Verification

No product code changes in this closure, so no gate is owed over it
(`ai/rules/pre-release.md`). The two citation edits are comments and prose.
