# 119 — NLRI String() Command Style

## Objective

Update all NLRI type `String()` methods to use consistent command-style syntax with the `set` keyword (`field set value`), matching the API command vocabulary for display/debugging output.

## Decisions

- `String()` is for display/debugging only — NOT for direct round-trip parsing. Reconstruction requires the full command wrapper (`update text nlri <family> add <String()>`).
- Optional fields only output when present/non-zero (labels, path-id, etag, gateway, ip).
- Label stacks formatted as comma-separated values.
- BGP-LS types gained `protocol` field in their output during critical review — it was missing and is required context.

## Patterns

- All 14 NLRI type families updated to `field set value` format using `strings.Builder` for efficient concatenation.

## Gotchas

- BGP-LS String() methods were missing the `protocol` field entirely — caught during critical review. `node protocol set <proto> asn set <n>` required adding protocol extraction.
- INET format change: `<prefix> path-id=<id>` → `<prefix> [path-id set <id>]` — path-id only emitted when non-zero, not always.

## Files

- `internal/bgp/nlri/evpn.go` — all 5 EVPN types updated
- `internal/bgp/nlri/ipvpn.go` — IPVPN updated
- `internal/bgp/nlri/labeled.go` — LabeledUnicast updated
- `internal/bgp/nlri/other.go` — VPLS, MVPN, RTC, MUP updated
- `internal/bgp/nlri/bgpls.go` — all 4 BGP-LS types updated, protocol field added
- `internal/bgp/nlri/flowspec.go` — FlowSpecVPN updated
- `internal/bgp/nlri/inet.go` — INET updated
