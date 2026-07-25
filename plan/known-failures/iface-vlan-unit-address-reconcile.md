### `internal/component/iface` `TestIntegrationApplyConfigVLANUnitAddressReconcile` -- pre-existing, VLAN unit address change never applied on reload

Confirmed pre-existing 2026-07-15 by `git archive HEAD` into a scratch tree and
running the test there in the QEMU VM: it fails **identically** on committed
`HEAD` (no VRRP work present), so `spec-vrrp-3-macvlan` did not cause it. Also
reproduced with the new owned-device reconcile pass compiled out, and
`config_apply.go`'s diff is purely additive (+143/-0), so the reconcile path is
byte-identical to `HEAD` in that configuration.

Symptom: `unit_integration_linux_test.go:133` -- after
`applyConfig(current, previous, b)` swaps a VLAN unit's address from
`10.60.200.1/24` to `10.60.200.2/24`, the new address is absent from
`parent0.200`. The reload's `applyConfig` returns **no errors**, so the address
loop believes it succeeded; the device itself survives (the `linkExists` check
above passes). The initial apply and its `requireAddress` assertion both pass,
so this is specific to the address-change-on-reload path for VLAN sub-interface
units, not to unit creation.

Fix owner: whichever session next works `internal/component/iface` VLAN unit
reconcile (`config_apply.go` unit/address loops). Not fixed by the VRRP umbrella
(`plan/spec-vrrp-0-umbrella.md`): unrelated code path, and VRRP's virtual
addresses reach the kernel through the owner registry, which is covered by
`registry_integration_linux_test.go` and the new
`device_owner_integration_linux_test.go` and is green.

**Root cause identified and fixed 2026-07-25; awaiting a QEMU run to close.**
Not a VLAN bug and not specific to `applyConfig`: it is the Linux IPv4
primary/secondary rule biting ze's make-before-break address reconcile.
`10.60.200.1/24` and `10.60.200.2/24` are the SAME subnet, so the reconcile's
add-loop (`internal/component/iface/config_apply.go:898-914`) installs
`.2` as a SECONDARY of `.1`, and the remove-loop
(`internal/component/iface/config_apply.go:916-932`) then deletes the PRIMARY
`.1` -- which makes the kernel delete every same-subnet secondary with it
(`net/ipv4/devinet.c`, `__inet_del_ifa`, unless
`net.ipv4.conf.<dev>.promote_secondaries` is 1). Both addresses vanish and
every `b.RemoveAddress` returned nil, which is why `applyConfig` reports no
errors. The same root cause produced the SIGHUP reload that left a dummy
interface with no address at all while the transaction reported success (the
operation path emits the same add-then-remove order, pinned by
`TestIfaceSameSubnetSwapOrdersAddBeforeRemove`).

Fix: `internal/plugins/iface/netlink/addr_primary.go` +
`addr_primary_linux.go` -- `RemoveAddress` now routes through
`removeAddressGuarded`, which enables `promote_secondaries` on the device
before an `AddrDel` that would otherwise cascade, so the kernel promotes a
secondary instead of flushing the subnet, and rejects the removal with a named
error if it cannot. The guard and the delete are one function so a caller
cannot reach the delete unguarded.

To close this entry, run `make ze-qemu-integration-test` and confirm
`TestIntegrationApplyConfigVLANUnitAddressReconcile` plus the new
`TestIntegrationRemoveAddressKeepsSameSubnetSibling` are green, then move this
entry to `RESOLVED.md`. `make ze-qemu-needs-linux-test` additionally runs the
new end-to-end reload fence `test/reload/test-tx-iface-address-swap.ci`.
