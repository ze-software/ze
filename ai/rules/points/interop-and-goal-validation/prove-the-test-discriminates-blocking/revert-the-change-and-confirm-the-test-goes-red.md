---
kind: directive
level:
stage:
---
**Before claiming an interop/functional test validates a change, revert the
change and confirm the test goes RED.** Rebuild the artifact the test drives
(the container image, the daemon binary) so the revert actually takes effect,
then restore the fix and confirm GREEN again. Record the RED result.
