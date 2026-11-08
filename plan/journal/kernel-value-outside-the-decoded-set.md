| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-08 | ipsec-spd-selection | `policyInfoFromKernel` (xfrm_linux.go) | `show vpn ipsec dataplane policy` prints a per-socket XFRM policy's direction as `unknown`: the kernel dumps socket policies as XFRM_POLICY_MAX + dir and the read-back maps every direction as SADir(dir)+1, so Ze's ESP form socket policy and any foreign override read as unknown | fixed: `policyDirFromKernel` maps socket directions 3 to 5 to in, out and fwd; socket rows read SPD `socket` (phase 8, AC-20) |
