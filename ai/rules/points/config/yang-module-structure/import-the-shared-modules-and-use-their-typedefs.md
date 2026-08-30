---
kind: directive
level: MUST
stage:
---
- Shared modules MUST be imported with their reserved prefixes: `import ze-types { prefix zt; }`, `import ze-extensions { prefix ze; }`.
- If a leaf holds a value that `zt` already types, `ze-types` MUST be imported and its typedef MUST be used. Skipping the `ze-types` import MUST NOT be treated as licence to re-invent its constraints. The typedef for each concept is `docs/architecture/config/yang-config-design.md`.
- Network endpoints, both binds and remote targets, MUST use the shared groupings. A hand-rolled pair MUST NOT be used, and a combined string MUST NOT be used.
