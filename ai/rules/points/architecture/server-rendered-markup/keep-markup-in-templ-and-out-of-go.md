---
kind: directive
level: MUST
stage:
---
- **Markup MUST live in a `.templ` file. A Go string literal MUST NOT build an HTML or SVG tag in `internal/component/web` or `internal/component/lg`.**
- **A templ component MUST take a named struct. A `map[string]any` MUST NOT reach one, and a struct field wrapping one MUST NOT either.**
- **A page MUST NOT carry an inline script, an inline style attribute, or an inline event handler. Both packages answer `'self'` for script, so a browser refuses an inline script and an inline handler and tells the server nothing. The rule covers the style attribute too, so both packages hold one rule and a header CAN be tightened without a hunt.**
- **Behavior a page needs MUST reach it as a data attribute an external asset reads. That asset MUST exist in the embedded filesystem the handler serves.**
- **A new exemption MUST carry its reason and MUST raise the exact count beside it. Each guard fixes the size of its table, so widening one is an edit a reader sees.**
- **A gate that names one package MUST NOT be treated as covering its sibling. Each guard walks its own directory, and `lg` shipped two dead handlers under the web package's green.**
