---
kind: table
level:
stage:
---
| Pattern | Why |
|---------|-----|
| `net.JoinHostPort(host, port)` where port is already a string and no numeric conversion is needed | Acceptable when the port comes from `net.SplitHostPort` or config as a string; prefer `textbuf.HostPort` for new code |
| `strings.Builder` as a long-lived `io.Writer` field | Struct fields like `pasteBuffer *strings.Builder` that accumulate writes over time are not string-building helpers. `textbuf.Buffer` freezes on `Slice()` and its pool semantics differ |
| `strconv.Itoa(n)` passed to sysctl/procfs map values | Writing to kernel interfaces that require `string`; the allocation is once per config reload, not per-packet |
