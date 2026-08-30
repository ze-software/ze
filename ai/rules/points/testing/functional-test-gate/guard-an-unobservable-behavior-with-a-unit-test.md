---
kind: directive
level: MUST NOT
stage:
---
**A behavior that genuinely cannot be made to fail under mutation because it is
not observable end to end MUST be guarded by a UNIT test that inspects the
producing value directly, and the test comment MUST say so. A `.ci` that passes
with the feature disabled MUST NOT be kept.** The reactor suppressing a duplicate
announce, which makes per-peer targeting wire-indistinguishable, is the shape
this covers.
