# 1260 -- website-wiki-content-migration

## Context

The Codeberg wiki held 158 substantive draft pages (its own Home page labels it an unreviewed draft) while the public Ze website lacked most operator, management, plugin-authoring, platform, and contributor documentation. The risk was the wiki becoming a second source of truth. The goal was to bring the useful content onto the website by publishing canonical `main/docs/` pages where they exist, adapting only the remaining worked examples, keeping generated references generated, and wiring every new page into navigation, search, SEO, and `llms.txt`. All work lives in the website repo (`../gh-pages`), not this one.

## Decisions

- Canonical `main/docs/` pages are the preferred publication source over copying wiki text; the wiki is source material only. Wiki-only examples were checked against current generated references before publishing.
- Generated CLI, configuration, feature, RFC, and plugin records remain generated; no hand-authored page replaces them (AC-7), and build output is never hand-edited -- sources and registries only, then regenerate.
- Consolidated guides over recreating dozens of short wiki pages, to keep the site navigable; task material under `docs/guide/`, end-to-end network roles under `usage/`, test/chaos evidence under `quality/`.
- Internal AI workflow material (e.g. `claude-code.md`) is not published as product documentation.
- Fleet Management's contradictory maturity claim was reconciled: moved from "Spec'd, not built" to "Experimental and growing", card linked to the implemented operations guide.

## Consequences

- 50 additional canonical `main/docs/` sources publish as HTML and Markdown mirrors; five new worked pages under `usage/` (route server, transit-edge RPKI, FlowSpec injection, chaos-tested peering, AS-path topology); the ExaBGP migration page gained a staged configuration and process-bridge walkthrough. All 55 migrated routes are in the rendered output, search index, sitemap, and `llms.txt`.
- Every published configuration block for the worked pages validates with the production `ze` binary, so the examples are executable claims, not prose.
- Future doc changes in `main/docs/` now propagate to the website through the page registry (`tools/page_registry.py`); adding a page means registering a source, not authoring website HTML.
- `llms.txt` summary extraction now handles YAML front matter, bold-leading paragraphs, and list-only pages without emitting empty or metadata-derived summaries.

## Gotchas

- The spec's Mistake Log is empty; the recorded traps are structural. The standing one: coverage buckets showed 44 wiki topics represented ONLY by generated plugin records and 62 with no one-to-one website page -- consolidation was a deliberate choice, so "a wiki page has no website twin" is expected, not a migration gap.
- Verification commands live in the OTHER repo and cannot be re-run from `main`: `tools/test_render_doc.py` (11 tests), `tools/test_render_llms.py` (4 tests), `tools/check-page-links.py --skip-network` (1,784 external anchors), and the full `./update-website.sh` build, plus browser smoke checks (hub, six usage pages, 1366x768 layout, Docs dropdown at 1024/1000 px). From `main` this closure verified only that the tooling, `usage/` pages, and the 178K `llms.txt` (20 usage/ routes) exist in `../gh-pages`.
- Relative-link breakage on render is prevented by the central registry plus the cross-document link manifest; bypassing the registry for a "quick page" reintroduces it.

## Files

(all in `../gh-pages` unless noted)
- `tools/page_registry.py` (50 canonical sources + 5 usage pages registered)
- `tools/render-llms-txt.py` (summary-extraction hardening), `llms.txt` (regenerated)
- `usage/route-server/`, `usage/transit-edge-rpki/`, `usage/flowspec-injection/`, `usage/chaos-tested-peering/`, `usage/as-path-topology/`, `usage/exabgp-migration/`
- `data/nav.json`, `data/page-links.json`, documentation hub links
- `tools/test_render_doc.py`, `tools/test_render_llms.py`, `tools/check-page-links.py`
- `main/docs/` guides republished as canonical sources (this repo, unchanged content)
