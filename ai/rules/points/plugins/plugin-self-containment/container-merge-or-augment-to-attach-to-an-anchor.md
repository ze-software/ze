---
kind: directive
level:
stage:
---
- **Container-merge:** the owner declares its own `container <verb> { container <noun> ... }` and the YANG loader unions same-named roots (iface, resolve, ike use this for `clear`). Preferred for new carves (no base-module coupling; see "How to carve" above).
- **Augment:** the owner declares `augment "/<prefix>:<verb>" { container <noun> ... }` against the anchor module (l2tp and bgp use this for `clear`, via `augment "/cliclearcmd:clear"`). An augment names its target module, so deleting the anchor breaks every augmenting owner's build. This is the concrete reason the bare anchor must remain.
