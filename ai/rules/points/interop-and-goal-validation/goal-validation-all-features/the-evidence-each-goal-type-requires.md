---
kind: table
level:
stage:
---
| Goal type | Required evidence |
|-----------|-------------------|
| Protocol interop ("ze speaks X with Y") | Interop test passes with the named peer daemon |
| Performance ("handles N updates/sec") | `ze-perf` benchmark result pasted |
| User workflow ("user can do X via CLI") | Functional `.ci` test exercising the full workflow, or `.et` test for editor workflows |
| Data correctness ("routes installed correctly") | Functional test with explicit data assertions (hex match, JSON field match), not just exit code 0 |
| Resilience ("survives X failure") | Chaos test or fault-injection scenario |
| Security ("rejects unauthorized X") | Negative test: unauthorized attempt fails with expected error |
