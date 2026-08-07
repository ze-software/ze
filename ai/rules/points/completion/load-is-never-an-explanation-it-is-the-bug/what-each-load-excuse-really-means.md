---
kind: table
level:
stage:
---
| Banned | What it actually says |
|--------|-----------------------|
| "fails under load / on a loaded host" | the test waits a fixed time instead of waiting for the condition |
| "load average was ~11 vs ~2 earlier" | you measured the host instead of reading the test |
| "passes in isolation" | it depends on scheduling luck. That IS the defect, stated |
| "the failing set rotates, so it is not deterministic" | several tests share one timing assumption. Find it |
| "the contended-run detector did not trip" | that detector labels runs. It never absolves a test |
| "not reproducible, logged as non-deterministic" | you do not need a repro to fix a timing assumption. Read the test |
