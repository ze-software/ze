---
kind: directive
level: MUST
stage:
---
**Dependent FILES inside another feature (`ze_l2tp` consumers).** When a feature of plugin A only exists because feature B exists, A's files MUST be tagged with B's tag and MUST get a counterpart: the cos dynamic RADIUS-CoS handler is `//go:build ze_l2tp` with a no-op stub (no BNG session events, nothing to react to), the diag l2tp capture branches live in `capture_l2tp.go` / `capture_raw_l2tp.go` with stubs answering "l2tp is not included in this build", and the web VPN/L2TP pages have not-in-this-build stub renderers so the workbench routes stay valid. A stub MUST ANSWER HONESTLY (name the missing feature) and MUST NOT silently no-op a user-visible request.
