---
kind: directive
level: MUST
stage:
---
1. MUST name the applicable set: the package glob, symbol kind, or call-site pattern the new test type is meant to cover.
2. MUST back-fill that set, or record the uncovered remainder as explicit, tracked backlog (spec, handoff, or deferral table). MUST NOT leave it implicit.
3. SHOULD prefer a grep- or registry-driven audit that enumerates every applicable site over per-file judgement. `/ze-hunt` enumerates sites for grep-detectable patterns.
