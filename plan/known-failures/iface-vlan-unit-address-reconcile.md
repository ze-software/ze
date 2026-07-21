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
