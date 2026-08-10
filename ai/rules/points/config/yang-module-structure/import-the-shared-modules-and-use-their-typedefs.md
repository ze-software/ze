---
kind: directive
level: MUST
stage:
---
- Shared modules MUST be imported with their reserved prefixes: `import ze-types { prefix zt; }`, `import ze-extensions { prefix ze; }`.
- If a leaf holds a value that `zt` already types, `ze-types` MUST be imported and its typedef MUST be used. Skipping the `ze-types` import MUST NOT be treated as licence to re-invent its constraints (see Value Typing).
- Network endpoints (binds and remote targets): the shared groupings MUST be used, never a hand-rolled pair or a combined string. See Network Endpoints below.
