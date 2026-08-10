---
kind: directive
level: MUST
stage:
---
**Allocation MUST happen once, at the outermost scope, and MUST pass inward.** The caller knows:
- How many times the callee will be called (loop count)
- What buffer size is needed (often a bounded maximum)
- When the buffer can be released (after all callees are done)
