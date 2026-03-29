# 475 -- Web Interface Umbrella Retrospective

## Context
The web interface was designed as a 6-phase spec set (web-0 umbrella + web-1 through web-6). The umbrella captured 28 design decisions and 28 acceptance criteria before implementation began. Over implementation, the UI evolved significantly beyond the original design as real browser interaction revealed better patterns. Each child spec completed individually (learned 468-474), but the umbrella was never reconciled against the final state.

## Decisions
- Chose macOS Finder-style column navigation over the spec's split-panel layout (D-15/D-17), because hierarchical tree browsing maps naturally to columns and the pattern proved itself during config view implementation before admin was built.
- Chose unified Finder for admin commands over card stacking (D-21), because the command tree is hierarchical and two different navigation paradigms for similar trees created inconsistency.
- Chose `ze start --web <port>` flag over `ze web` subcommand, because the web server runs alongside the daemon (not standalone), making a flag on `start` more accurate than a separate command.
- Chose `environment { web {} }` config block over standalone `web {}`, because web and SSH are peer server configurations and grouping them clarifies that relationship.
- Added `--insecure-web` for HTTP without auth over the spec's "HTTPS only" rule, because development and testing need a low-friction path (forces 127.0.0.1 to limit blast radius).
- Built tristate boolean toggles (default/on/off with YANG-aware coloring) over simple checkboxes, because YANG schemas distinguish "not set" from "set to false" and the UI needed to surface that distinction.
- Built table views for list entries over flat key lists, because lists with multiple leaves (peers, routes) are unreadable as single-column key names.
- Created `.wb` browser test framework with agent-browser (57 tests) over the spec's planned `.ci` tests in `test/plugin/`, because web UI testing requires actual browser interaction that `.ci` process-level tests cannot provide.
- Added peer/entry creation, theme toggle, and MCP server -- none in the original spec scope.

## Consequences
- The umbrella spec's design decisions (D-15, D-17, D-21) and visual models are now historically inaccurate. Future readers should treat the learned summaries (468-474 + this file) as the authoritative record, not the umbrella spec.
- The Finder pattern is now the standard for all tree navigation in ze's web UI. Any future hierarchical view (monitoring, diagnostics) should reuse `FinderColumn`/`ColumnItem`/`FragmentData`.
- `.wb` tests are the testing paradigm for web UI. No `.ci` web tests exist in `test/plugin/` as the spec planned. The test infrastructure lives in `internal/component/web/testing/`.
- The `environment {}` config block is the grouping pattern for server configurations. Future server-type features (metrics endpoint, gRPC) should follow this pattern.

## Gotchas
- Specs written before implementation cannot anticipate UI paradigm shifts discovered through real browser interaction. The Finder pattern was discovered during web-2 implementation and retroactively obsoleted 4 design decisions. Writing detailed visual models in specs (D-15's "left panel / right panel" layout) creates false precision that diverges from reality.
- The umbrella spec's acceptance criteria (AC-5 "left panel shows peer key names", AC-21 "titled card with header and tabular output") describe the wrong UI. ACs tied to specific visual layouts are brittle -- behavior ACs ("user can navigate to a peer and see its config") survive design evolution, layout ACs don't.
- The planned `.ci` test paths (`test/plugin/web-startup.ci` etc.) were never created. The test paradigm shifted entirely to `.wb` files. Specs that name specific test files before the testing approach is proven create phantom references.

## Files
- `plan/spec-web-0-umbrella.md` -- original umbrella spec (stale design decisions)
- `plan/learned/468-web-1-foundation.md` through `plan/learned/474-web-admin-finder.md` -- per-phase summaries
- `internal/component/web/` -- 73 files implementing the web interface
- `test/web/` -- 57 `.wb` browser tests
- `docs/guide/web-interface.md` -- user guide (current)
- `docs/architecture/web-interface.md` -- architecture doc (current)
- `docs/architecture/web-components.md` -- component design doc (current)
