# 126 — Full Extended Next Hop Support (RFC 8950)

## Objective

Complete RFC 8950 Extended Next Hop support by adding `nexthop` capability to the ExaBGP migration schema and verifying VPN-IPv4 encoding format.

## Decisions

- VPN-IPv4 encoding investigation showed `buildMPReachVPN` already uses 24-byte format (RD=0 + IPv6) — RFC 8950 compliant. No code change needed for encoding.
- Parser handles both RFC 5549 (16/32 bytes, legacy) and RFC 8950 (24/48 bytes) formats for backwards compatibility with old implementations.

## Patterns

- ExaBGP auto-detects nexthop capability from `nexthop { }` block presence if not explicitly set. Migration must handle both cases without duplication.

## Gotchas

- Original VPN next-hop parsing had `case 40` but the correct byte count for dual-stack VPN (RD + IPv6 + RD + IPv6) is 48. Silent parsing failure for a valid RFC 4659/8950 packet.
- RFC comments in `mpnlri.go` referenced obsolete RFC 5549 format — updated to RFC 8950.

## Files

- `internal/exabgp/schema.go` — `nexthop` added to capability block and peer level
- `internal/exabgp/migrate.go` — `convertNexthopBlock()`, `normalizeSAFI()`, capability inference from nexthop block
- `internal/bgp/attribute/mpnlri.go` — `case 48` fix, RFC comment updates
