# A removed setting keeps its kernel value

The apply path writes a kernel value only while the config sets it. When the
operator removes the setting, no phase puts back the value the device had before
Ze changed it, so the device keeps the old configured value until a reboot.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-08 | - | ethernet `mac address` (`applyConfig` Phase 2, `internal/component/iface/config_apply.go`) | Deleting `mac address`, or the whole ethernet entry, leaves the configured MAC on the NIC until reboot. Phase 2 calls `SetMACAddress` only when `MACAddress` is set, and no phase restores the permanent address (`InterfaceInfo.PermanentMAC`, IFLA_PERM_ADDRESS) that the backend already reads. VyOS fixed the same class in T3871 (2b6a744cc) | open: restore the permanent MAC when the leaf or the entry goes away |
