# Apply Re-Adds What The Data Plane Already Holds

An apply that runs on every commit adds state unconditionally instead of
reconciling against what the data plane holds. A second apply of the same
config then fails on "already exists", and state the config no longer names is
never removed.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-08 | - | VPP NAT44 SNAT pool lifecycle, `applySNAT` and `cleanupNATOrphans` (`internal/plugins/firewall/vpp/nat_linux.go`) | `reconcileWithOps` reruns `applyNATChains` on every Apply and `applySNAT` re-adds each range: VPP answers VALUE_EXIST for an address already pooled and the error is not filtered, so a second Apply with an unchanged snat term fails. `cleanupNATOrphans` sweeps static mappings only, so a removed snat term's addresses stay pooled. Read-verified, not reproduced | open: reconcile the pool against `nat44_address_dump` and the set Ze applied |
