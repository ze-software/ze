---
kind: table
level:
stage:
---
| Mode | Syntax | Implementation |
|------|--------|----------------|
| Fork (default) | `pluginname` | Subprocess via exec, TLS connect-back |
| Internal | `ze.pluginname` | Goroutine + net.Pipe + DirectBridge (hot path bypasses pipes) |
| Direct | `ze-pluginname` | Sync in-process call |
| Path | `/path/to/binary` | External binary, TLS connect-back |
