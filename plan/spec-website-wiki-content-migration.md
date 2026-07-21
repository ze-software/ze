# Spec: website-wiki-content-migration

| Field | Value |
|-------|-------|
| Status | complete |
| Depends | - |
| Phase | Complete |
| Updated | 2026-07-21 |

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
