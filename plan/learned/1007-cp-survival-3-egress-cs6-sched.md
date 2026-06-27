# 1007 -- Egress CS6 Priority Scheduling (DSCP u32 Selector Fix)

## Context

Ze marks its own BGP packets with DSCP CS6 (`IP_TOS=0xC0`), but the traffic-control DSCP filter was broken: `translateFilter` built `netlink.U32` with `ClassId` set but no `TcU32Sel` match selector. A configured DSCP filter reached the kernel matching nothing, silently failing to classify. This meant CS6-marked BGP keepalives competed equally with attack traffic under congestion, defeating the purpose of the DSCP marking.

## Decisions

- Fixed at the source (`translateFilter`) by populating `TcU32Sel` with per-family keys, over any mark-based workaround, because the existing config surface (`match dscp N`) was correct but the backend half-implemented.
- Each DSCP/protocol filter now emits two u32 filters (ETH_P_IP + ETH_P_IPV6) over a single ETH_P_ALL filter, because u32 key offsets differ between IPv4 (TOS at byte 1) and IPv6 (traffic class across bytes 0-1 of the version/TC/flow word).
- Changed `translateFilter` return type from `(netlink.Filter, error)` to `([]netlink.Filter, error)`, updating `backend_linux.go` to iterate the slice, because the dual-family approach requires two filter objects per logical match.
- Extracted a shared `internal/core/dscp` package with `Parse()` and named DSCP map over duplicating the parser, since both firewall (`config.go`) and traffic (`config.go`) needed name-to-value resolution.
- Added input validation: DSCP 0-63, protocol 0-255 (at the `translateFilter` boundary), over relying on config-layer validation alone.

## Consequences

- `internal/core/dscp` is a new leaf package available to any component that needs DSCP name parsing (firewall config already migrated).
- The `translateFilter` API change is internal to the netlink backend; the `traffic.TrafficFilter` model is unchanged.
- The existing `vlan-qos-lab` config, which used a DSCP filter that silently did nothing, now actually classifies. No behavioral regression since the old behavior was "match nothing."

## Gotchas

- IPv4 TOS byte is at offset 0 byte 1 in the IP header (32-bit word 0). The u32 key needs `Val: dscp << 18` and `Mask: 0x00FC0000` (DSCP shifted left by 2 for TOS encoding, then left by 16 to land in byte 1 of the 32-bit word).
- IPv6 traffic class spans the version/TC/flow-label word differently: DSCP occupies bits 27-22, so `Val: dscp << 22` with `Mask: 0x0FC00000`.
- The `nl.TcU32Sel` requires `Flags: nl.TC_U32_TERMINAL` and `Nkeys: 1` to be set explicitly; omitting either produces a filter the kernel silently ignores.

## Files

- `internal/plugins/traffic/netlink/translate_linux.go` -- the fix: `dscpFilters()`, `protocolFilters()`, `u32FilterPair()` populate `TcU32Sel`
- `internal/plugins/traffic/netlink/backend_linux.go` -- updated to iterate `[]netlink.Filter` from `translateFilter`
- `internal/plugins/traffic/netlink/translate_linux_test.go` -- unit tests for DSCP/protocol selector construction
- `internal/plugins/traffic/netlink/cs6_integration_linux_test.go` -- netns integration test for CS6 classification
- `internal/core/dscp/dscp.go` -- shared DSCP name-to-value map with `Parse()`
- `internal/core/dscp/dscp_test.go` -- unit tests for Parse
- `internal/component/traffic/config.go` -- uses `dscp.Parse` for named DSCP support in match config
- `internal/component/traffic/config_test.go` -- named DSCP parsing tests
- `internal/component/firewall/config.go` -- migrated to `dscp.Parse`
- `test/traffic/020-vpp-reject-dscp-filter.ci` -- functional test: VPP backend rejects DSCP filter
- `test/traffic/021-cs6-priority-config.ci` -- functional test: CS6 priority class config
