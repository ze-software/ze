---
kind: directive
level:
stage:
---
- **A feature-gated file is still an always-on pin for a DIFFERENT gate.** `cmd/ze/hub/service_ssh.go` (`//go:build ze_ssh`) imported `bgp/config`. `dep_audit.file_requires_tag` is per-tag, so it flags that file, correctly: a `ze_ssh`-on / `ze_bgp`-off build genuinely fails to compile. Clear gated files too, not just untagged ones.
- **Removing an always-on import can unlink an `init()` nobody else pulls in.** `bgp/config` registers the reactor factory, and it was linked ONLY because the hub imported it. Blank-importing it from `bgp/plugin` is the natural fix but cycles in test (`bgp/config`'s own tests import `plugin/all`, which imports `bgp/plugin`). It is linked from `cmd/ze/dispatch_bgp.go` instead: a `package main` root can never be imported back, so the edge is always safe there. After deleting an always-on import, ask what that package's `init()` was providing.
