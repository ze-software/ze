# 313 — Chaos Dashboard: Trigger Icon Buttons

## Objective

Replace the chaos action dropdown in the control panel with a flex-wrap grid of individual icon buttons (Unicode icon + short label), each loading the param form via the same `GET /control/trigger-form?action=<type>` endpoint.

## Decisions

- Backend endpoints unchanged (`handleControlTriggerForm`, `handleControlTrigger`) — purely a rendering change in `writeControlPanel`.
- Active state via inline `onclick` (querySelectorAll-based) rather than HTMX `hx-on` — simpler, no new JS dependencies.
- `chaosActionIcon()` and `chaosActionLabel()` helpers added as separate functions rather than inlined into the render loop — unit-testable for all 8 action types independently.
- Functional test skipped — pure rendering change with no protocol path to exercise end-to-end.

## Patterns

- Replacing a `<select>` with icon buttons: keep the same `hx-get` endpoint and query param contract; change only the trigger element.
- New CSS classes `.trigger-grid`, `.trigger-btn`, `.trigger-active` added to `assets/style.css`.

## Gotchas

- Implementation used `writeControlSidebar` instead of `writeControlPanel` as the actual function name — spec had the name slightly wrong.
- Functional test skipped (chaos test infra absent).

## Files

- `cmd/ze-chaos/web/control.go` — `chaosActionIcon()`, `chaosActionLabel()`, `writeTriggerButtons()`, dropdown replaced
- `cmd/ze-chaos/web/assets/style.css` — trigger grid and button styles
