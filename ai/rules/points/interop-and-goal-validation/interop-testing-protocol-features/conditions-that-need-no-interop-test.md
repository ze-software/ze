---
kind: table
level:
stage:
---
| Condition | Why |
|-----------|-----|
| Pure internal refactor, no wire-visible change | Existing interop tests cover the path |
| Config-only feature (no protocol impact) | CLI/config tests suffice |
| Tooling (ze-analyse, ze-perf) | No protocol peer involved |
