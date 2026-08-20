# A document listener assumes its event target is an Element

`document.addEventListener` receives events whose `target` is `document` or a
text node. `Element.closest` exists on neither, so an unguarded
`e.target.closest(...)` throws and the rest of that listener never runs.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-18 | spec-fixit-cli-format-default-everywhere | web workbench page under htmx 4.0.0-beta6 | `PAGEERROR: e.target.closest is not a function` fires twice on `/show/system/identity/` in Chromium, once at load and once after a workbench save. Captured with playwright against the `demos/terminal/web-config` daemon. `internal/component/web/assets/cli.js` calls `e.target.closest` unguarded at :389, :448, :468, :619, :632 and :670; the same file guards it at :462 and :478, so the shape the guarded sites expect is the one the others meet | not fixed |
