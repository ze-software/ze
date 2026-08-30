---
kind: directive
level: MUST NOT
stage:
---
**These three patterns MUST NOT be converted to `textbuf` during a sweep. Each one is correct as written.**

| Pattern | Why |
|---------|-----|
| `net.JoinHostPort(host, port)` where the port is already a string | Correct when the port came from `net.SplitHostPort` or from config as a string. New code SHOULD prefer `textbuf.HostPort` |
| `strings.Builder` as a long-lived `io.Writer` field | A field such as `pasteBuffer *strings.Builder` accumulates writes over time. `textbuf.Buffer` freezes on `Slice()` and its pool semantics differ |
| `strconv.Itoa(n)` passed to a sysctl or procfs map value | The kernel interface requires a `string`, and the allocation happens once per config reload, not per packet |
