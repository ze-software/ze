# Learned: Named Interface Units + Unified Name Validation

## What Was Built

Changed interface unit keys from numeric IDs (`unit 0 { ... }`) to freeform
names (`unit default { ... }`, `unit firewall-3 { ... }`). YANG key changed
from `leaf id { type uint32 }` to `leaf name { type zt:node-name }`. Go struct
field changed from `unitEntry.ID int` to `unitEntry.Label string`. The DHCP
chain (factory, payload, client) changed from `unit int` to `unit string`
throughout.

Followed up with a unified `naming.ValidateNodeName` in `internal/core/naming`
and a `zt:node-name` YANG typedef. Migrated four scattered validators (iface
unit, BGP peer/group, firewall table/chain, VPP interface) to the shared
function. Pattern: `[a-zA-Z0-9_][a-zA-Z0-9_.-]*`, length caller-specified.

## Key Decisions

- **Label, not Name.** The Go field is `Label` not `Name` because `ifaceEntry`
  already has a `Name` field (the OS interface name). `Label` is the config-level
  identifier; `Name` is the OS-level identifier.

- **CLI unit/addr commands unchanged.** `ze interface unit add eth0 100` operates
  on OS-level VLAN IDs via netlink, not config unit names. These stay numeric.

- **dhcp-auto uses "default" as unit key.** When dhcp-auto discovers an
  ethernet interface, the synthetic DHCP entry uses `unit: "default"`. Collision
  with user config is impossible because dhcp-auto only activates when no
  explicit DHCP config exists.

- **Unified pattern is wider than some callers needed.** The shared pattern
  allows uppercase, underscores, and dots. Unit names and sysctl profiles
  previously only allowed lowercase. Widening is acceptable because the names
  are informational labels, not protocol identifiers.

- **Punctuation-only names now valid.** BGP peers/groups previously rejected
  names like `___` ("must contain at least one letter or digit"). The unified
  validator drops this check. These names are unusual but harmless.

## What Was Already Working

- VLAN subinterface creation used `vlan-id`, not unit key. The unit ID was
  never used in netlink operations. Changing it to a string had no effect
  on OS-level interface management.
- The netlink monitor's `Unit int` fields (in event payloads) are OS-level
  VLAN IDs from parsing `eth0.100`, completely separate from config unit names.

## Mistakes

- **Forgot `DHCPPayload.Unit` and `AddrPayload.Unit`.** The initial struct
  rename caught `unitEntry.ID` but missed the payload types in `iface.go`.
  The NTP plugin test caught this: JSON deserialization of `"unit":0` (int)
  into a string field silently produced empty string.
- **Bulk error message replacement.** Replacing all `"invalid peer name"`
  with `"invalid character"` broke tests for wildcard and IP-address
  rejections, which have different error messages. Fix: use specific
  error strings per rejection reason.

## Files

None recorded.
