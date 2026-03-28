# 471 -- Admin/Operational Commands

## Context
The web interface needed a way to execute operational commands (peer teardown, rib clear, daemon shutdown) alongside config editing. These are admin-tier mutations under `/admin/` -- distinct from view-tier reads (`/show/`, `/monitor/`) and config-tier edits (`/config/`).

## Decisions
- Three-tier URL scheme (`/show/`, `/config/`, `/admin/`) over single `/api/` prefix -- matches CLI's two modes (edit + command) plus future fleet management
- Verb-first in URL (`/config/set/<path>`) over verb-last (`/<path>/set`) -- mirrors CLI command order, avoids collision with YANG node names
- Command results as titled cards that stack over replacing previous output -- multiple results visible simultaneously
- `CommandDispatcher` as function type over interface -- simplicity, one implementation needed

## Consequences
- Admin commands are POST-only (mutations) -- read-only operational queries go through `/show/`
- Card stacking is client-side HTMX behavior (`hx-swap="afterbegin"`) -- server returns one card per execution
- Command dispatch is injectable -- the actual engine command dispatch is wired post-reactor-start

## Gotchas
- Some commands are view-tier (bgp summary) and some are admin-tier (peer teardown) -- the tier depends on whether the command mutates state
- The command tree is provided as static data rather than walked from YANG at runtime -- keeps the handler decoupled from YANG internals

## Files
- `internal/component/web/handler_admin.go`, `handler_admin_test.go`
- `internal/component/web/templates/command.html`, `command_form.html`
