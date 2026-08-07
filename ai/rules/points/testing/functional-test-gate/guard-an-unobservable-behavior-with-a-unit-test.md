---
kind: note
level:
stage:
---
If a test genuinely cannot be made to fail under mutation because the behavior is not
observable end-to-end (e.g. the reactor suppresses a duplicate announce, so per-peer
targeting is wire-indistinguishable), guard it with a UNIT test that inspects the
producing value directly, and say so in the test comment. Do NOT keep a `.ci` that
passes with the feature disabled.
