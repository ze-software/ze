# 725 -- DHCP Server Multiple Address Ranges

## Context

The DHCP server plugin used a single start/stop pair per subnet (`container range` in YANG), preventing subnets with disjoint address pools. VyOS uses `range <name> { start; stop; }` as a keyed list, and ze needed to match this model for migration parity. The change touched config parsing, address pool allocation, and the YANG schema.

## Decisions

- Chose `list range` keyed by name over keeping `container range` with a separate `additional-range` list, because VyOS uses the keyed-list model and it is cleaner for N ranges.
- Chose composite pool with per-segment bitmaps over a single flattened bitmap spanning the full address space, because disjoint ranges would waste memory on the gap between them.
- Chose format detection in `parseRanges` (check for `start` key at top level) over requiring a config migration step, because backward compatibility with the old single-range format avoids breaking existing deployments.
- Overlap detection uses `start[i] <= stop[i-1]` (inclusive ranges), not `<`, to prevent the same IP appearing in two segments.

## Consequences

- Multiple pools per subnet are now supported (up to 10), enabling split allocations for guest/management/IoT pools on a single subnet.
- Pool stats aggregate across segments; callers see total/allocated/available summed.
- Old single-range configs continue to work without migration; the parser auto-detects the format.

## Gotchas

- The spec's Critical Review said "stop1 == start2 is OK" for adjacent ranges, but with inclusive ranges that means the same IP in two segments. The correct adjacency check is `start2 > stop1` (strictly greater). Off-by-one in the spec description, not in the intent.
- `findSegment` is called under the pool lock by all mutation methods. For 10 segments this is fine; if the limit ever increases, a sorted search would be needed.

## Files

- `internal/plugins/dhcpserver/schema/ze-dhcp-server-conf.yang` -- `container range` to `list range`
- `internal/plugins/dhcpserver/config.go` -- `addressRange` type, `parseRanges`, format detection, overlap validation
- `internal/plugins/dhcpserver/pool.go` -- `poolSegment` type, composite pool with per-segment bitmaps
- `internal/plugins/dhcpserver/handler.go` -- pass `Ranges` slice to `newPool`
- `internal/plugins/dhcpserver/register.go` -- fallback serverIP from first range
- `internal/plugins/dhcpserver/config_test.go` -- 4 new tests (multi-range, single named, old format, overlap)
- `internal/plugins/dhcpserver/pool_test.go` -- 4 new tests (multi-range alloc, exhaustion, static, stats)
- `internal/plugins/dhcpserver/handler_test.go` -- updated to use `Ranges` field
- `internal/plugins/dhcpserver/lease_test.go` -- updated `newPool` calls
- `docs/guide/configuration.md` -- added DHCP server configuration section
