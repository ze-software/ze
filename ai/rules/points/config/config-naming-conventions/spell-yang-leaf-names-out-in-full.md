---
kind: directive
level: MUST
stage:
---
**YANG leaf names MUST NOT use abbreviations.** Operators read YANG leaves in CLI completion and `show configuration`. `fwd` means nothing to someone who did not write the code. Leaf names MUST be spelled out in full: `forward`, `buffer`, `channel`, `maximum`.
