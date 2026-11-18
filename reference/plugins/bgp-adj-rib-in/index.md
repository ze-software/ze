# `bgp-adj-rib-in` plugin

Adj-RIB-In storage (raw hex replay)

## At a glance

| Field | Value |
|-------|-------|
| Registry area | BGP |
| Kind | Runtime plugin |
| Source path | `internal/component/bgp/plugins/adj_rib_in` |
| YANG modules | 0 |

## Configuration

None

## Dependencies

- Required: None
- Optional: None

## Used by

- Required dependency for: [`bgp-rpki`](../bgp-rpki/index.md), [`bgp-rr`](../bgp-rr/index.md)
- Optional dependency for: [`bgp-rs`](../bgp-rs/index.md)

## Startup ordering

Order only when both plugins are selected; never auto-loads a plugin.

- Start after: None
- Starts before: [`bgp-gr`](../bgp-gr/index.md)

## Repository artifacts

Package: `internal/component/bgp/plugins/adj_rib_in`

YANG files: None
Metadata source: `Registration`
