# 308 — Chaos UX-5 Horizontal Control Strip

## Objective

Move the chaos dashboard control panel from the sidebar into a compact horizontal strip below the header, keeping trigger form and route dynamics panel in the sidebar.

## Decisions

- `writeControlStrip()` is the new horizontal strip renderer; `writeControlSidebar()` renders only the trigger dropdown — clean split by single concern
- `writeSpeedControl()` deleted entirely rather than just removed from sidebar — speed buttons are inline in the strip; keeping the function would be dead code
- `writeRestartSection()` similarly deleted — inline in strip
- All 5 control handlers (pause, resume, rate, stop, restart) and handleControlSpeed now return strip HTML — single swap target `#control-strip` replaces both `#control-panel` and `#speed-control`
- CSS selector `#control-panel select` updated to `.card select` — the ID no longer exists; generic selector is correct since cards are the only container with selects
- Functional test (`test/chaos/control-strip.ci`) skipped — layout-only change fully validated by 15 unit + handler tests; chaos functional tests require specific orchestrator setup

## Patterns

- HTMX `hx-target` must be updated globally when an element ID changes — grep for all references to old IDs (`#control-panel`, `#speed-control`) before shipping
- "Freeze" toggle and "Panels" toggle sit in the tab bar area — existing convention for toolbar-level controls

## Gotchas

- A test that checked for "Stop" text in the sidebar matched "Stops" in chaos action impact text — fixed to assert `/control/stop` URL instead of substring match on label text

## Files

- `cmd/ze-chaos/web/control.go` — split writeControlPanel, deleted writeSpeedControl and writeRestartSection, all handlers updated
- `cmd/ze-chaos/web/render.go` — strip between header and content, sidebar calls writeControlSidebar
- `cmd/ze-chaos/web/assets/style.css` — .control-strip flex layout, grid-template-rows: auto auto 1fr
