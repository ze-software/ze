---
kind: directive
level: MUST
stage:
---
**When the design produces one of these artifacts, its row MUST be read before the design is proposed.**

| Artifact | Read | Prevents |
|----------|------|----------|
| New plugin | `ai/patterns/plugin.md` | Wrong structure, missing YANG, wrong callback |
| Cross-plugin broadcast | `pkg/ze/eventbus.go`, `internal/core/events/typed.go`, and one consumer such as fibkernel | Treating EventBus as request and response when it is async pub/sub |
| Cross-plugin request and response | `pkg/plugin/rpc/bridge.go` (DirectBridge) | Reinventing DirectBridge, which already serves synchronous typed calls from core to internal plugins |
| Shared registry | `internal/core/family/` (read the code) | A registry inside a plugin instead of core |
| Config option | `ai/patterns/config-option.md` and `ai/rules/config.md` | Missing env var, wrong YANG shape, env-only where config belongs, wrong leaf name |
| CLI command | `ai/patterns/cli-command.md` | Wrong dispatch structure |
| TUI or terminal colors | `docs/architecture/cli/color-system.md` | Wrong color roles, an inconsistent palette across surfaces |
| Platform-specific code | The existing splits (`fibkernel/backend_linux.go`, `ifacenetlink/sysctl_linux.go`) | Wrong build tag, wrong abstraction level |
| A feature with a dataplane effect | `internal/plugins/iface/netlink/` and `internal/plugins/iface/vpp/` | A netlink-only feature with no VPP support |
| Naming | `ai/rules/go-standards.md`, `ai/rules/config.md`, and a grep for analogous names | Inventing a ze-name where a kernel or standard name exists, an abbreviated YANG leaf, an env var path that does not mirror YANG |
