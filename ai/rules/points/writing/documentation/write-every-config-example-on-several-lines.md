---
kind: directive
level: MUST
stage:
---
**Every config example in `docs/` MUST be written one statement per line, and MUST NOT be collapsed onto one line.** The multi-line form is the house style, and an agent that reflows an example to one line, followed by an agent that reflows it back, costs two diffs and tells the reader nothing.
**The one-line form is also a syntax error unless the last statement carries its semicolon.** Automatic semicolon insertion fires at a newline, so a closing brace on the same line as the statement it closes ends the block before the statement ends. Measured with a built `ze config validate`: `attach process bgp-rr { receive [ update ] }` is refused with "expected ';' after receive, got RBRACE", while `attach process bgp-rr { receive [ update ]; }` is accepted. An operator copies what a guide shows, so a guide MUST NOT show the refused form.
**An inline mention inside a sentence or a table cell MAY stay on one line, and MUST then carry the semicolon** (`internal rib { use bgp-rib; }`). Reflowing a phrase into a block would break the sentence around it.
