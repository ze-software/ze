# A document listener assumes its event target is an Element

`document.addEventListener` receives events whose `target` is `document` or a
text node. `Element.closest` exists on neither, so an unguarded
`e.target.closest(...)` throws and the rest of that listener never runs.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-18 | spec-fixit-cli-format-default-everywhere | web workbench under htmx | Chromium reports `e.target.closest is not a function` on `/show/system/identity/` at load/save; delegated Document/Text targets also fail `matches`. | fixed: `assets/cli.js` guards selectors with `instanceof Element`. Native identity load/save/reload, Document/Text matrix, HTML/SVG actions, namespace submission and exception-collector calibration pass. Broader web checks remain pending. |
