---
kind: directive
level: MUST
stage:
---
**Ze MUST implement every applicable requirement of every RFC it implements, fully and correctly (`ai/META-DECISIONS.md`). Full RFC coverage MUST NOT be treated as a first-release prerequisite: shipped code MUST comply with the RFC requirements of the capabilities it implements, while a missing RFC feature MAY wait until after the first release.** A missing Linux primitive claimed as an implementation limit MUST be named with evidence, not assumed. Verification MUST exercise real entry points and include interoperability with another implementation where applicable. Helpers, isolated encoders and unfinished code alone do not prove a capability works.
