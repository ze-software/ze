---
kind: directive
level: MUST
stage:
---
1. **Transitive package drop** (no tag). A manifest line moves the package's blank imports into `all_<tag>.go`; dead-code elimination does the rest. Whole plugins qualify: `flowspec-firewall` needed one line and no source change.
2. **Core-leaf move** (no tag). A contract always-on consumers share with the feature moves to an always-on `internal/core/*` leaf; consumers change an import path only. `ze_bgp` needed three: `internal/core/bgp/routeaction` (the route-action/verb vocabulary sysrib and every FIB backend use), `internal/core/bgp/msgtype` (the message-type codes MRT classifies by), and `internal/core/bgp/ribevents` (the best-change contract sysrib and flow-export subscribe to). The LEAF MUST be moved, not the package: `bgp/message` imports `plugin/registry`, so relocating it wholesale would be a core-tier violation.
3. **Inversion-of-control seam** (no tag on the always-on side). Where always-on code reaches INTO the feature, invert it: the always-on side exposes a nil-able hook and the gated code self-registers from its own `init()`. A nil seam MUST have a CORRECT no-feature behavior, not just a nil check. `ze_bgp` inverted five: `ze config dump|diff|validate` tree resolution and peer validation plus the graceful-restart marker writer (`internal/component/config/infra`), the MRT RIB-dump provider and the web hex-packet decoder (`internal/component/plugin/registry`), and the IGP next-hop cost sysrib used to push into BGP best-path (`internal/core/rib/igpcost`).
