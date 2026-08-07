---
kind: table
level:
stage:
---
| Tier | Home | What it is | Examples |
|------|------|------------|----------|
| **core / infra** | `internal/core/` | A library you cannot "run as a plugin." Foundational; no config-driven lifecycle. | family, events, metrics, diagnostic, bufpool |
| **component** | `internal/component/` | A platform plugin: other plugins/components depend on it or plug into it. | bgp, iface, firewall, traffic, vpp |
| **plugin (edge)** | `internal/plugins/` | An edge plugin: a config-driven engine that nothing else depends on. | ntp, static, dhcpserver, l2tp-auth-* |
