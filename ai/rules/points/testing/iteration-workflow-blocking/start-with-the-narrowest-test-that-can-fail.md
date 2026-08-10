---
kind: directive
level: MUST
stage:
---
**Specific before generic.** For code changes, MUST start with the narrowest test
that can fail because of the changed file: direct Go test, matching `.ci`/`.et`
case, file-level test, feature test, or suite-local command. Then move outward
only after the narrower test passes. MUST NOT spend CPU on unaffected packages or
whole suites before proving the affected code path works.
