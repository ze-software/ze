---
kind: directive
level: MUST
stage:
---
**Markup MUST live in a `.templ` file: a Go string literal MUST NOT build an HTML or SVG tag in `internal/component/web` or `internal/component/lg`, and a templ component MUST take a named struct rather than a `map[string]any`.** A page MUST NOT carry an inline script, an inline style attribute or an inline event handler, because both packages answer `'self'` for script and a browser refuses the inline form silently; behavior a page needs MUST arrive as a data attribute an external asset reads, and that asset MUST exist in the embedded filesystem the handler serves. A guard that names one package does not cover its sibling, and a new exemption MUST state its reason and raise the exact count beside it.
