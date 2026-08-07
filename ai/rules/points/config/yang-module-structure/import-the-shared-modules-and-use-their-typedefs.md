---
kind: directive
level:
stage:
---
- Import shared modules with their reserved prefixes: `import ze-types { prefix zt; }`, `import ze-extensions { prefix ze; }`.
- If a leaf holds a value that `zt` already types, import `ze-types` and use the typedef. Not importing `ze-types` is not a licence to re-invent its constraints (see Value Typing).
- Network endpoints (binds and remote targets): use the shared groupings, never a hand-rolled pair or a combined string. See Network Endpoints below.
