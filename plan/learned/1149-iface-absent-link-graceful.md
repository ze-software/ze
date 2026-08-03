# 1149 — iface-absent-link-graceful (CLOSURE)

Interface config-apply now **gracefully skips a configured physical (Ethernet)
interface that is absent from the deployment target** instead of aborting the
whole apply. Producer: `internal/component/iface/config_apply.go` —
pre-compute `absentPhysical` by probing each non-disabled `cfg.Ethernet` entry
with `b.GetInterface`, log `"iface config: configured interface not
present, skipping"`, then filter those names out of `allEntries`
(`:667-674`) so the Phase-2 property loop and Phase-2c admin-up loop never touch
them. Created types (dummy/veth/bridge/tunnel/wireguard/xfrm) are made in Phase 1
and are NOT skipped; a genuine (non-absent) error still aborts and rolls back via
`record()` → `rollbackPartial()` (`config_apply.go`).

Tests: `TestApplyConfigSkipsAbsentEthernet`, `TestApplyConfigRollsBackGenuineError`
(`config_apply_test.go`) — both green. AC-3 (full L2TP appliance end-to-end proof)
was tracked in **fixit-appliance-evidence-config**, now closed green (`f42c2ccb2`,
learned `1106`, real xl2tpd/pppd session 5/5 on the QEMU gokrazy appliance).

## GOTCHAS
- Only the **synchronous** phases needed the explicit skip: address reconcile
  (Phase 3+4) diffs *live* kernel state, so an absent interface is already excluded
  there; the MAC/property set (Phase 2) and admin-up (Phase 2c) iterate the config
  list and would otherwise hard-fail on `SetMACAddress`/link-up of a missing NIC.
- Why this was needed at all: the `ze init --seed` fix (learned `1103`) stopped
  baking the *build host's* absent NIC (`ens18`) into the appliance config; before
  that, the appliance booted with a config naming an interface it didn't have.
- Assumption A-2 ("skipping an absent iface leaves the rest applied") sat PENDING on
  an L2TP re-run that had already landed green in the receiving spec — a stale
  assumption is a closure smell; check the deferral destination's status before
  leaving one open.

## Files

None recorded.
