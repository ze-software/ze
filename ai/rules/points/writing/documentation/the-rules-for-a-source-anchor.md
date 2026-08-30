---
kind: directive
level: MUST
stage:
---
**Every paragraph or table in `docs/` that carries a factual claim about syntax, field names, behavior or data structures MUST carry a source anchor, written `<!-- source: <relative-path> -- <symbol-or-topic> -->` after that paragraph or row.** One anchor per factual paragraph or table, never per sentence and never per file. It MUST NOT sit inside a fenced code block, because an HTML comment renders as visible text there; put it after the closing fence.
**When you edit a page you MUST verify its existing anchors and fix a stale one, and when you change code you MUST check whether a doc anchors the changed file and update the claim that is now wrong.** Run `./le doc check verify` after editing any file under `docs/`, after adding or removing a plugin, and after touching a YANG `ze:command` declaration. It is not part of `./le verify current mode full`, so you MUST run it on demand.
