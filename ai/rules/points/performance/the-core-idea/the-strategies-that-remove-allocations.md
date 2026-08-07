---
kind: directive
level:
stage:
---
1. **Buffer ownership** -- caller owns the buffer, callee writes into it
2. **Pool lifecycle** -- bounded pools replace unbounded `make()`
3. **Lazy parsing** -- raw byte slices with offset iterators, no parsed structs
