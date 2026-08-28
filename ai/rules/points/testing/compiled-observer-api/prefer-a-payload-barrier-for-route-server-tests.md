---
kind: directive
level: SHOULD
stage:
---
**SHOULD wait for route-server readiness through a compiled payload predicate.**
Poll the pushed state or `eor-sent` counters before shutdown. Do not replace the
barrier with a guessed delay or a synchronous one-shot query.
