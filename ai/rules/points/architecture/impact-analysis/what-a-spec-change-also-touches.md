---
kind: directive
level: MUST
stage:
---
**A change to a spec under `plan/` MUST also update everything its row names.**

| What changed | Also update |
|---|---|
| Status change | The per-session marker, through `./le spec session` |
| An AC added or removed | The wiring test table and the audit table |
| A design decision | Annotate it with `-> Decision:` for post-compaction recovery |
