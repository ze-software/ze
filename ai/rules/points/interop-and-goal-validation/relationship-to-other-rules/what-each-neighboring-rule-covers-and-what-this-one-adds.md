---
kind: directive
level: MUST NOT
stage:
---
**This rule adds to its neighbors and MUST NOT be read as replacing any of them:**
- `ai/rules/testing.md` requires functional tests per feature type, and owns the test infrastructure and workflow. This rule adds interop on top for protocol features, and says when each test type is mandatory
- `ai/rules/completion.md` requires every acceptance criterion tested. This rule requires the AGGREGATE goal proven
- `ai/rules/rfc-compliance.md` requires RFC conformance in code. This rule requires that conformance proven against another implementation
