# Egress CS6 Priority Scheduling: the DSCP u32 Selector

Ze marks its own BGP packets with DSCP CS6 (`IP_TOS=0xC0`). The traffic-control
DSCP filter did not act on that mark: `translateFilter` built a `netlink.U32`
with `ClassId` set and no `TcU32Sel` match selector. A configured DSCP filter
reached the kernel matching nothing, so CS6-marked keepalives competed equally
with attack traffic under congestion.

## The fix is at the source

<!-- source: internal/plugins/traffic/netlink/translate_linux.go -- dscpFilters, protocolFilters, u32FilterPair -->

`translateFilter` populates `TcU32Sel` with per-family keys. The config surface
`match dscp N` was already correct, and the backend was half implemented. A
mark-based workaround would have left the config surface lying.

## Two filters per logical match

<!-- source: internal/plugins/traffic/netlink/backend_linux.go -- filter iteration -->

Each DSCP or protocol filter emits two u32 filters, one for `ETH_P_IP` and one
for `ETH_P_IPV6`, and not one `ETH_P_ALL` filter. The u32 key offsets differ
between the families: IPv4 carries TOS at byte 1, and IPv6 carries the traffic
class across bytes 0 and 1 of the version/TC/flow word.

`translateFilter` therefore returns `([]netlink.Filter, error)`, and
`backend_linux.go` iterates the slice.

## The key arithmetic

The offsets are easy to get wrong and impossible to see from a passing test:

- IPv4: the TOS byte is byte 1 of 32-bit word 0. The key needs
  `Val: dscp << 18` and `Mask: 0x00FC0000`. That is the DSCP shifted left by 2
  for the TOS encoding, then left by 16 to land in byte 1 of the word.
- IPv6: the DSCP occupies bits 27 to 22 of the first word, so `Val: dscp << 22`
  with `Mask: 0x0FC00000`.
- `nl.TcU32Sel` needs `Flags: nl.TC_U32_TERMINAL` and `Nkeys: 1` set explicitly.
  Omit either and the kernel ignores the filter without an error.

## Shared DSCP parsing

<!-- source: internal/core/dscp/dscp.go -- Parse, named DSCP map -->

`internal/core/dscp` is a leaf package holding the name-to-value map and
`Parse()`. Both the firewall config and the traffic config need name resolution,
and the firewall config was migrated onto it.

Validation happens at the `translateFilter` boundary as well as in the config
layer: DSCP 0 to 63, protocol 0 to 255.

## Consequence

The `vlan-qos-lab` config used a DSCP filter that matched nothing. It now
classifies. There is no behavioral regression, because the previous behavior was
to match nothing.
