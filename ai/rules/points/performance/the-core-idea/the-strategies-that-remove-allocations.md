---
kind: directive
level: MUST
stage:
---
1. **Buffer ownership** -- the caller MUST own the buffer, and the callee MUST write into it
2. **Pool lifecycle** -- bounded pools MUST replace unbounded `make()`
3. **Lazy parsing** -- raw byte slices with offset iterators MUST be used, not parsed structs
