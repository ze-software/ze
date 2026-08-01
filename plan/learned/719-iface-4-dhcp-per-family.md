# Learned: Move DHCP Into Per-Family Containers

## What Was Built

Moved `dhcp {}` from the unit level into `ipv4 {}`, and `dhcpv6 {}` into
`ipv6 {}`, following the Junos/Nokia model. Config syntax changed from
`unit 0 { dhcp { ... } }` to `unit 0 { ipv4 { dhcp { ... } } }`.

YANG: relocated both containers. Go: moved `DHCP` field from `unitEntry` to
`ipv4Settings`, `DHCPv6` field to `ipv6Settings`. Parsers call
`parseDHCPv4Config`/`parseDHCPv6Config` from within `parseIPv4Settings`/
`parseIPv6Settings` instead of from `parseUnits`. Reconciliation updated
to navigate `u.IPv4.DHCP` / `u.IPv6.DHCPv6`.

## Key Decisions

- **Removed `ze:os "linux"` from dhcp/dhcpv6 containers.** The parent
  `ipv4`/`ipv6` containers already carry `ze:os "linux"`. The `ze:os`
  check prunes the parent node at schema-build time, making the annotation
  on children redundant.

- **Kept `ze:backend "netlink"` on dhcp/dhcpv6.** Unlike `ze:os`, the
  backend feature gate walks the tree per-node. Without the annotation,
  the VPP commit-time rejection would stop working.

- **DHCP presence makes `ipv4Settings` non-nil.** Adding `dhcp {}` inside
  `ipv4 {}` with no other fields (no addresses, no sysctls) sets `set=true`
  and returns a non-nil `ipv4Settings`. This is correct: the family container
  has meaningful content.

- **`dhcp-auto` path unchanged.** The auto-discovery path creates synthetic
  `dhcpParams` entries directly in the `desired` map, bypassing struct
  fields entirely. No change needed.

## What Was Already Working

- `dhcpUnitKey` keying by (ifaceName, unit) was unchanged. The key comes
  from `unitEntry.Label`, not from the DHCP struct location.
- `reconcileDHCP` logic, event bus subscriptions, link failover, and
  route-priority handling were all unchanged. Only the navigation path
  to reach the DHCP config changed.
- `route-priority` correctly stays at unit level (it applies to all
  default routes, not just one family).

## Mistakes

- **Docs not updated on first pass.** `docs/guide/configuration.md` had
  two DHCP examples showing the old unit-level placement. Caught by
  `/ze-review`, not by tests. Config restructure specs should always
  grep docs for examples of the old syntax.

## Files

None recorded.
