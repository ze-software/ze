# DHCP server

The DHCP server serves leases to LAN clients per RFC 2131 and RFC 2132:
the DISCOVER, OFFER, REQUEST, ACK, NAK, RELEASE, and DECLINE state machine,
address pools, lease expiry, static MAC-to-IP mappings, and the standard
options. It also carries the PXE options the provisioning path needs.

<!-- source: internal/plugins/dhcpserver/register.go -- plugin registration and engine lifecycle -->
<!-- source: internal/plugins/dhcpserver/handler.go -- the DHCP state machine and reply building -->
<!-- source: internal/plugins/dhcpserver/pool.go -- address allocation -->
<!-- source: internal/plugins/dhcpserver/lease.go -- lease tracking and expiry -->
<!-- source: internal/plugins/dhcpserver/config.go -- config parsing and validation -->

## Decisions

- **A plugin, not a component.** It owns a config subtree and a lifecycle, and
  it is coupled to neither the BGP engine nor interface management. It registers
  from `init()` and the generated composition root discovers it.
- **Its own bitmap pool, not the L2TP pool.** The L2TP pool assigns IPCP
  addresses to PPP sessions. DHCP needs static-mapping exclusion, MAC-to-address
  tracking, reservation on INIT-REBOOT, and a way to mark an address
  unavailable. Sharing would couple two unrelated domains.
- **One socket per listen interface, with multi-handler dispatch.** Each
  interface gets one UDP socket, bound with `SO_BINDTODEVICE` on Linux. The
  serving goroutine tries each subnet handler in order, routing on the relay
  address, the client address, or the requested-IP option. A bare DISCOVER with
  no hint falls through to the first handler that has pool space.
- **A static-only subnet gets an empty pool, not a missing one.** A zero-size
  pool with guards in every method is simpler than a nil pool with a branch at
  every call site.
- Non-Linux builds bind to all interfaces. The interface-specific path is a
  Linux socket option.

<!-- source: internal/plugins/dhcpserver/socket_linux.go -- SO_BINDTODEVICE binding -->
<!-- source: internal/plugins/dhcpserver/socket_other.go -- the non-Linux fallback -->

## Address ranges

A subnet carries a keyed list of ranges, so disjoint pools on one subnet are
expressible. The parser detects the old single-range shape and accepts it, so no
config migration is needed.

- **A composite pool with one bitmap per segment**, not one flat bitmap spanning
  the whole address space. A flat bitmap wastes memory on the gap between
  disjoint ranges.
- Pool statistics are summed across segments, so a caller sees one total.
- **Overlap detection compares `start[i] <= stop[i-1]`.** Ranges are inclusive,
  so adjacency requires `start2 > stop1` strictly. Reading it as `<` puts one
  address in two segments.
- The segment lookup runs under the pool lock and scans linearly. That is fine
  at the current limit of ten segments. A larger limit needs a sorted search.

## Traps

**Exhaustion on DISCOVER is answered with silence, not NAK.** RFC 2131 section
4.3.1: no address available means no response. NAK is valid for REQUEST only.
The instinct to "reject the client somehow" is the wrong one.

**DECLINE marks the address unavailable; it does not release it.** RFC 2131
section 4.3.3. Releasing it hands the same conflicting address to the next
client. The address to mark comes from the requested-IP option, not from the
client's current lease.

**Replacing a lease must release the old address.** Removing the client from the
address index alone leaks the address out of the pool forever.

**An option write must be bounds-checked.** Many DNS servers or a long domain
name overflow the response buffer. The option length field is one byte as well,
so a domain name longer than 255 bytes truncates silently. Both are rejected at
config parse time and guarded at write time.

**Every configured address must be refused unless it is IPv4.** This server
speaks RFC 2131, and the reply builder narrows each address it sends to four
bytes with `netip.Addr.As4`, which panics on an IPv6 address. The panic lands on
the DISCOVER path, so the trigger is an ordinary client packet and the config
that caused it committed clean. `parseSubnet` refuses an IPv6 subnet prefix, and
`parseIPv4Option` refuses an IPv6 `default-router`, `dns-server` or
`pxe tftp-server`. A range bound and a static mapping need no family check of
their own: each must sit inside the subnet prefix, and `netip.Prefix.Contains`
is false across families. An IPv4-mapped IPv6 spelling is refused with the rest,
which is what `config.IPv4AddressValidator` accepts for every other module.

The YANG module carries `ze:validate "ipv4-address"` on the same leaves. That
annotation declares the family. It does not enforce it, because
`ValidateCustomSections` walks a fixed list of top-level sections and `service`
is not one of them.

**The zero-value address panics.** Converting `netip.Addr{}` to four bytes
panics, which is why every pool mutation guards the empty-pool case. The first
test suite covered DISCOVER only, whose allocation path returns early, and
missed REQUEST, whose reservation path does not.

## Related

- `pxe-staging.md` for the two-stage PXE path this handler implements
- `image-server.md` and `tftp-server.md` for the other provisioning services
