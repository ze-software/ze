---
kind: table
level:
stage:
---
| Plugin type | Transport | Auth | Config |
|-------------|-----------|------|--------|
| Internal (goroutine) | `net.Pipe()` then DirectBridge | N/A | implicit |
| External (local) | TLS over TCP (single connection) | Per-plugin token via `ZE_PLUGIN_HUB_TOKEN` env + cert pinning via `ZE_PLUGIN_CERT_FP` | `plugin { hub { server <name> { host ...; port ...; secret ...; } } }` |
| External (remote) | TLS over TCP (single connection) | Token via out-of-band config | `plugin { hub { server <name> { host ...; port ...; secret ...; } } }` |
