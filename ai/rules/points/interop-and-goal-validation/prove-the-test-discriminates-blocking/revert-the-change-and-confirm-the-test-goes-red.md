---
kind: directive
level: MUST
stage:
---
**Before claiming an interop/functional test validates a change, MUST revert the
change and confirm the test goes RED.** MUST rebuild the artifact the test drives
(the container image, the daemon binary) so the revert actually takes effect,
then restore the fix and confirm GREEN again. MUST record the RED result.
