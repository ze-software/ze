| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-16 | weakened-followups | reload unit test | authentication fixture ran in the bare `ze_core` pass and failed because its YANG modules were compiled out | moved the unchanged test to a file guarded by its `ze_bgp` and `ze_ssh` feature tags |
