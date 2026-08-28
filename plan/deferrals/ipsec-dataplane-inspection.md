# Deferrals -- spec-ipsec-dataplane-inspection

Source: `plan/spec-ipsec-dataplane-inspection.md`. Format: `ai/rules/planning.md`.

Created 2026-08-02, at the same time as the spec. The spec's metadata table names this shard,
and its Goal Gate "Deferral shard resolved: no live row without a destination" cannot be
evaluated against a file that does not exist. `plan/deferrals/ipsec-esp-dual-form-receive.md`
records that exact failure, so this file is created with the spec rather than after it.

One row exists at design time. It is the single Known Limitation in the spec that is
outstanding WORK rather than a scope boundary. The other two limitations there (no per-VRF
policy view, `ze_ipsec_tunnel_up` keeping its current definition) are decisions, not
deferrals, and are not listed here.

Reference lists in this file are written as bullets, never as tables. Every pipe-delimited
line here is read by the deferral gate as a six-cell row.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-02 | spec-ipsec-dataplane-inspection | Implement the VPP binary-API SAD and SPD dump behind `Dataplane.ListSAs` and `Dataplane.ListPolicies`, so `show vpn ipsec dataplane sa` and `... policy` report real state on a VPP dataplane instead of reporting that the backend cannot enumerate | Separable, and the spec's goal holds without it. The goal is that Ze can see its own dataplane on the appliance, and the appliance runs the XFRM backend (`ikeDataplaneName` returns `xfrm` with no override). The VPP path returns `ErrNotSupported` under AC-6, which is an honest answer rather than a silent empty list, so nothing regresses and no surface lies. Doing it needs a second binary-API dump message pair and a VPP fixture the ipsec suite does not have | `plan/spec-ipsec-dataplane-inspection.md` | deferred |

## Not deferred: the rows that stay live in the spec

Listed rather than tabled, for the reason given above.

Three items look deferrable and are not. Each is named here so a later session does not move
it into the table above without a decision.

- The kernel-level functional test (`test/ipsec/ipsec-show-dataplane-kernel.ci`) needs a
  `fsuite ipsec` line in the retired `scripts/evidence/qemu-all-tests.sh` (current producer: `internal/le/qemu/alltests.go`), which does not exist today. The
  missing line is one edit and is in scope (Phase 7). It is the reason no IPsec functional
  test has ever run against a real kernel, so deferring it would leave the spec proving its
  central claim under the noop backend, which cannot prove it at all.
- The interop scenario `dataplane-readback` is the only evidence that Ze's own read-back
  agrees with an independent reader. `ai/rules/interop-and-goal-validation.md` makes it
  mandatory for the goal, not optional.
- The `checkKernelModules` built-in false positive (`internal/component/doctor/checks_linux.go`)
  is a live defect this spec walks into, so `ai/rules/completion.md` makes it in scope. It is
  AC-10, not a deferral.
