---
kind: directive
level: SHOULD
stage:
---
- **`Slice()` SHOULD be used by default.** Most strings are passed to a function (ParsePrefix, map lookup, Write) and discarded, and `Slice()` saves the copy.
