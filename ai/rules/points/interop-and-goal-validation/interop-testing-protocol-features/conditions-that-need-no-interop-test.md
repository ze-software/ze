---
kind: directive
level: MAY
stage:
---
**An interop test MAY be omitted only in these three conditions:**

| Condition | Why |
|-----------|-----|
| A pure internal refactor with no wire-visible change | The existing interop tests cover the path |
| A config-only feature with no protocol impact | CLI and config tests suffice |
| Tooling (`ze-analyse`, `ze-perf`) | No protocol peer is involved |
