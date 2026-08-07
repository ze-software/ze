---
kind: table
level:
stage:
---
| Layer | Location | Purpose |
|-------|----------|---------|
| Registry | `internal/component/plugin/registry/` | Central registry (leaf package, no plugin deps) |
| Family registry | `internal/core/family/` | Cross-component address-family registration (`Family`/`AFI`/`SAFI` types + `family.MustRegister`) |
| Public SDK | `pkg/plugin/sdk/` | Callback abstraction for external plugins |
| RPC Types | `pkg/plugin/rpc/` | Shared YANG RPC types + `MuxConn` for concurrent RPCs |
| Internal | `internal/component/bgp/plugins/<name>/` | Plugin implementations + `register.go` |
| All imports | `internal/component/plugin/all/` | Blank imports triggering all `init()` |
| CLI shared | `internal/component/plugin/cli/` | `PluginConfig` + `RunPlugin()` |
