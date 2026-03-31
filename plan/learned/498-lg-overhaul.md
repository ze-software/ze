# 498 -- Looking Glass Overhaul

## Context

The Ze looking glass had a broken custom HTMX shim (40 lines) instead of the real htmx.min.js already vendored in the repo, no ASN name resolution despite the decorator framework being production-ready, hardcoded light-mode SVG colors invisible in dark mode, a confusing multi-page layout, and no family selector. Five previous LG specs were deleted as "completed" without implementing most planned features.

## Decisions

- **Replace custom HTMX shim with real htmx.min.js** (over keeping the shim): the real library (50KB) was already vendored at `third_party/web/htmx/` and synced to the web UI. Added LG as a consumer in `scripts/sync-vendor-web.sh`. This fixed SSE, navigation, loading indicators, and all HTMX features.
- **Wire existing ASN decorator** (over building a new one): `asnNameDecorator` in `internal/component/web/decorator_asn.go` with Team Cymru DNS was production-ready. Added `ASNDecorator func(string) string` to `LGConfig`, wired in hub via closure over existing decorator.
- **Single-page tab layout** (over multi-page navigation, inspired by GIXLG): three tabs (Peers, Lookup, Search) with HTMX fragment swapping into a single content div. Eliminated the "lost in navigation" problem.
- **Unified search endpoint `/lg/search`** (over separate `/lg/search/aspath` and `/lg/search/community`): single form with type selector. Fewer routes, simpler code.
- **go:embed for assets and templates** (over const strings in Go source): matches web UI and chaos web patterns. Templates as `.html` files in `templates/` directory, assets in `assets/` directory, served via `http.FileServer`.
- **CSS variables in SVG** (over hardcoded colors): SVG `<style>` uses `var(--node-fill)` etc. with fallbacks, inheriting from the page's dark/light mode CSS variables.

## Consequences

- SSE live peer updates now work (real HTMX SSE extension).
- ASN names appear in peer tables, route tables, and graph nodes.
- Graph is dark-mode aware and auto-loads on prefix lookup.
- Old URLs `/lg/search/aspath` and `/lg/search/community` return 404. Any bookmarks or scripts using them need updating to `/lg/search` with `type=aspath` or `type=community`.
- `LGConfig` has a new `DecorateASN` field (nil-safe, backward compatible).
- `sync-vendor-web.sh` now syncs to 3 consumers (chaos, web, LG).

## Gotchas

- **Search for existing solutions before writing new code.** The custom HTMX shim and unwired decorator were the two biggest problems. Both already had production-ready solutions in the same repo.
- **Learned summaries that say "future X" prove the spec is not done.** The previous learned summary (488) said "Future decorator wiring requires populating GraphNode.Name" while claiming the specs were complete.
- **SSE needs WriteTimeout override.** Adding `WriteTimeout: 60s` to the HTTP server breaks SSE connections. The fix is `http.NewResponseController(w).SetWriteDeadline(time.Time{})` in the SSE handler to clear the deadline for that connection only.
- **Go's built-in template `eq` handles all types.** A custom `eq` that type-asserts to string silently treats all non-strings as equal (nil == 42 == true). Just use the builtin.
- **`hx-target="next tr"` targets the wrong row.** It finds the next sibling `<tr>`, then `afterend` inserts after that sibling. Use `hx-target="this"` to insert after the clicked row.
- **Docs had wrong env var names** (`ze.lg.*` vs actual `ze.looking-glass.*`). A user following the docs would get an abort on unregistered key. Always verify env var names against `env.MustRegister` calls.

## Files

- `internal/component/lg/` -- rewrote 7 source files, deleted `assets.go`, created `embed.go` + `assets/` + `templates/` (8 HTML files)
- `cmd/ze/hub/main.go` -- wired ASN decorator into LG server
- `scripts/sync-vendor-web.sh` -- added LG as sync consumer
- `docs/guide/looking-glass.md` -- updated UI table, env var names, tab layout description
- `docs/architecture/web-interface.md` -- updated LG source files, URL scheme, env vars
- `docs/comparison.md` -- Ze looking glass: No changed to Yes
- `plan/learned/488-lg-looking-glass.md` -- updated stale HTMX/decorator/env statements
- `.claude/rules/memory.md` -- added two new mistake log entries
