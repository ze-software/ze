---
kind: table
level:
stage:
---
| Pattern | When |
|---------|------|
| `container` + `ze:listener` + `uses zt:listener` | Single-endpoint services (web, SSH, MCP, LG, telemetry, BGP global listen) |
| `list` + `ze:listener` + `uses zt:listener` | Named multi-instance listeners (plugin hub server) |
| `container` + `ze:listener` + manual ip/port | When ip type differs from standard (BGP peer local: union with auto enum) |
