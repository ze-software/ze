# 151 — Hub YANG Modules

## Objective

Define YANG modules for ZeBGP's built-in configuration: `ze-types.yang` (common types), `ze-bgp.yang` (BGP schema), and `ze-plugin.yang` (plugin declarations).

## Decisions

- Chose goyang (`github.com/openconfig/goyang`) over libyang — pure Go, no cgo dependency, sufficient for validation needs.
- Kept schema simple and pragmatic rather than OpenConfig-compatible — ze doesn't need OpenConfig alignment and the complexity is unnecessary.
- YANG files are embedded in binaries via `//go:embed` — plugins send embedded YANG content during Stage 1 declaration, no external files needed at runtime.
- `leafref` used for cross-references (peer → peer-group, peer → route-map) to enable validation that referenced objects exist.

## Patterns

- `ze-bgp.yang` uses a `peer-config` grouping to share common fields between `peer` and `peer-group` lists, avoiding duplication.
- Type `hold-time` uses `"0 | 3..65535"` range (hold-time of 1 or 2 is invalid per RFC 4271).

## Gotchas

- Hold-time valid range is non-contiguous: 0 (disabled) OR 3-65535. Values 1 and 2 are explicitly invalid. This pattern of disjoint ranges appears in YANG as pipe-separated expressions.

## Files

- `yang/ze-types.yang`, `yang/ze-bgp.yang`, `yang/ze-plugin.yang` — YANG module definitions
- `internal/yang/modules/` — embedded copies for go:embed
- `internal/yang/loader.go` — YANG loader using goyang
