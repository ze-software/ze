---
kind: directive
level: MUST
stage:
---
**A change under `docs/` MUST also check everything its row names.**

| What changed | Also check |
|---|---|
| New factual claim | Its source anchor: `<!-- source: path -- symbol -->` |
| A feature count or list | `./le doc check verify` validates it against the live registry |
| Changed config syntax | `docs/guide/configuration.md` and `docs/architecture/config/syntax.md` |
