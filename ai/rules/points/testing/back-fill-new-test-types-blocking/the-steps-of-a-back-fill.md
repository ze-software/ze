---
kind: directive
level:
stage:
---
1. Name the applicable set: the package glob, symbol kind, or call-site pattern the new test type is meant to cover.
2. Back-fill that set, OR record the uncovered remainder as explicit, tracked backlog (spec, handoff, or deferral table). Never leave it implicit.
3. Prefer a grep- or registry-driven audit that enumerates every applicable site over per-file judgement. `/ze-hunt` enumerates sites for grep-detectable patterns.
