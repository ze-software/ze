| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-08 | ipsec-spd-selection | `policyInfoFromKernel` (xfrm_linux.go) | `show vpn ipsec dataplane policy` prints a per-socket XFRM policy's direction as `unknown`: the kernel dumps socket policies as XFRM_POLICY_MAX + dir and the read-back maps every direction as SADir(dir)+1, so Ze's ESP form socket policy and any foreign override read as unknown | open: Phase 8 of spec-ipsec-spd-selection (AC-20) maps socket directions to in, out and fwd with SPD `socket` |
