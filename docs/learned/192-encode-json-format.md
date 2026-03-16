# 192 — Encode JSON Format

## Objective

Add `encode json nlri <family> <json>` command so callers can encode NLRI from structured JSON instead of raw hex strings.

## Decisions

- JSON input must be minified (no spaces) because the encode protocol is space-delimited — cannot use `json.Indent` output as input.
- VPN RD (Route Distinguisher) required special handling separate from the generic NLRI encoding path.

## Patterns

- None beyond encode protocol extension.

## Gotchas

- OR-of-AND FlowSpec groups: `[[">80","<100"]]` was flattened to a single list, breaking compound conditions. Nested arrays must be preserved.
- Multiple same-type components: map assignment overwrites earlier entries — must use slice or keyed map with distinct keys.
- Always test both single and compound FlowSpec rules when touching FlowSpec JSON parsing.

## Files

- `internal/component/bgp/plugins/nlri/flowspec/encode.go` — JSON-to-NLRI encoding
- `internal/component/bgp/encode/` — encode protocol handler
- `internal/component/bgp/plugins/nlri/vpn/` — RD special handling
