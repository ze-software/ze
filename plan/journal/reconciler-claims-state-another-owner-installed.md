# Reconciler claims state another owner installed

A reconciler makes live state equal desired state, and it reads "not desired"
as "mine to remove". Live state also holds what the kernel or a protocol client
installed on its own authority. A reconciler that cannot tell the two apart
deletes the other owner's state on every pass, and that owner only puts it back
on its own schedule, if it ever does.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-18 | - | `reconcileOnReadyWithJournal` (`internal/component/iface/config_apply.go`) | Every commit, vpp connect or owned-address change removed each unnamed address on an enabled unit: the DHCP lease, SLAAC addresses and the kernel fe80:: link-local. Reproduced red in QEMU by `TestIntegrationReconcileKeepsLearnedAddresses` | open: finite-lifetime and unnamed link-local addresses spared (0fbb8b2065); DHCPv4 leases, infinite included, spared by `RegisterLeasedAddresses`. Left: infinite DHCPv6 lease, VPP lifetimes |
