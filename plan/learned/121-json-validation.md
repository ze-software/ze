# 121 — JSON Validation in Test Framework

## Objective

Make `json:` lines in `.ci` test files actually validate against decoded BGP output, instead of being stored and ignored. Covers IPv4/IPv6 unicast and FlowSpec families.

## Decisions

- Chose content-based matching (NLRI + action) over position-based matching because Ze BGP sends routes in lexicographic order, not config order — positions don't align.
- Chose to ignore `peer`, `direction` fields in comparison: these are test-environment-dependent and don't validate message content.
- Chose to skip validation for unsupported families (EVPN, VPN, BGP-LS) rather than fail — the `ze bgp decode` output works but transform logic is not implemented.
- Detected legacy ExaBGP envelope format via `"exabgp"` key presence to skip validation for older test files.

## Patterns

- The test runner invokes `ze bgp decode --update` as a subprocess to decode wire bytes to JSON. This reuses the existing decode pipeline rather than duplicating it.
- `transformEnvelopeToPlugin()` converts ze bgp decode output (ExaBGP envelope format) to plugin format. These two formats coexist and the transform is the bridge.

## Gotchas

- Ze bgp decode outputs withdrawals as `[{"nlri":"..."}]` not `["..."]` — `transformWithdraw()` must handle both formats for compatibility.
- Two JSON formats coexist in test files: older ExaBGP envelope format (in `test/data/encode/`) and newer plugin format (in `test/data/plugin/`). Detection via `"exabgp"` key.
- FlowSpec `"no-nexthop"` means no redirect target — the `next-hop` field is omitted from the `nlri` object entirely (not set to empty).

## Files

- `internal/test/runner/json.go` — transform, compare, family detection
- `internal/test/runner/json_test.go` — 15 unit tests
- `internal/test/runner/runner.go` — `validateJSON()`, `decodeToEnvelope()`
