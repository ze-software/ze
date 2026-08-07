---
kind: note
level:
stage:
---
Declare one `textbuf.Buffer` before a loop and call `Reset()` between
iterations. Each iteration reuses the same 128-byte inline array:
