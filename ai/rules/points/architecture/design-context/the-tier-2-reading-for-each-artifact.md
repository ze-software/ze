---
kind: table
level:
stage:
---
| Artifact | Read | Prevents |
|----------|------|----------|
| New plugin | `ai/patterns/plugin.md` | Wrong structure, missing YANG, wrong callback |
| Cross-plugin comm (broadcast) | `pkg/ze/eventbus.go` + `internal/core/events/typed.go` + one consumer (e.g. fibkernel) | EventBus is for async pub/sub notifications, not request/response |
| Cross-plugin comm (request/response) | `pkg/plugin/rpc/bridge.go` (DirectBridge) + `plan/learned/DESIGN-HISTORY.md` "Plugin system: architecture" (294, retired) | DirectBridge for sync typed calls from core to internal plugins. Do not reinvent this. |
| Shared registry | `internal/core/family/` (read the code) | Registry inside a plugin instead of core |
| Config option | `ai/patterns/config-option.md` + `ai/rules/config.md` + `ai/rules/config.md` + `ai/rules/config.md` | Missing env var, wrong YANG shape, env-only when should be config, wrong leaf name |
| CLI command | `ai/patterns/cli-command.md` | Wrong dispatch structure |
| TUI / terminal colors | `docs/architecture/cli/color-system.md` | Wrong color roles, inconsistent palette across surfaces |
| Platform-specific | Existing splits (`fibkernel/backend_linux.go`, `ifacenetlink/sysctl_linux.go`) | Wrong build tag, wrong abstraction level |
| New feature with dataplane effect | `internal/plugins/iface/netlink/` + `internal/plugins/iface/vpp/` | Netlink-only feature without VPP support |
| Naming | `ai/rules/go-standards.md` + `ai/rules/config.md` (config/env) + grep analogous names | Inventing ze-names when kernel/standard names exist, abbreviated YANG leaves, env var path not mirroring YANG |
