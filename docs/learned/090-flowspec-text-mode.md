# 090 — FlowSpec Text Mode

## Objective

Add FlowSpec NLRI support to the text mode parser, enabling API commands like `nlri ipv4/flowspec add destination 10.0.0.0/24 protocol tcp destination-port =80`.

## Decisions

- One `add`/`del` = one FlowSpec rule (unlike prefix families with multiple prefixes per add/del): FlowSpec components combine with AND logic to define a single rule; batching disconnected rules is meaningless.
- `add` comes BEFORE match components (not after): consistent with prefix family grammar where `add`/`del` precedes content.
- Extended community function syntax (`traffic-rate 65000 1000000`, `discard`, `redirect`) added alongside existing colon syntax: more readable for common FlowSpec actions.
- `push` as alias for `psh` TCP flag: ExaBGP compatibility.

## Patterns

- Value range validation per component (`flowSpecComponentMaxValue()`): protocol/icmp-type/code=0-255, ports/packet-length=0-65535, dscp=0-63.
- Bitmask operators: `flag` (INCLUDE), `=flag` (Match), `!flag` (Not), `!=flag` (Not+Match), `&flag` (AND bit).

## Gotchas

- ECE (0x40) and CWR (0x80) TCP flags were initially missing: RFC 3168 ECN flags are part of the TCP flags field. Added during critical review.
- `nolint` for `gocritic ifElseChain` was required in the component type dispatcher: the ordering of if-else branches matters (type ordering enforcement per RFC 8955 Section 4.2) and cannot be replaced by a switch.
- VPN FlowSpec families require an `rd` accumulator before the `nlri` section — consistent with other VPN families.

## Files

- `internal/plugin/update_text.go` — +550 lines: FlowSpec parsing, bitmask operators, value validation
- `internal/plugin/route.go` — extended community function constructors
- `internal/bgp/nlri/flowspec.go` — FlowOpNot, FlowOpMatch, NewFlowFragmentMatchComponent added
