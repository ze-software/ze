---
kind: directive
level: MUST
stage:
---
**Replacing a location key with a symbol key MUST preserve multiplicity.** Two tags inside one function share a symbol, so a plain `path::Name` key collapses them and deleting one then reads as unchanged, which is a false FRESH: the one outcome a freshness check exists to prevent. `rfc/audit/*.json` keeps a within-symbol ordinal (`path::Name#2`) for exactly this. A location key gave multiplicity away for free, and a symbol key has to be asked for it.
