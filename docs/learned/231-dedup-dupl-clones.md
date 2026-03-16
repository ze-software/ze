# 231 — Dedup: dupl Clone Elimination

## Objective
Remove ~620 lines of duplicate code identified by the `dupl` linter across 22 files using four targeted strategies.

## Decisions
- Group C (plugin decode loop for bgpls/evpn/vpn) skipped: the three plugins differ in validation function, decode function, result marshaling, and family declarations — a shared helper would add more complexity than it saves.
- Three large community parser copies consolidated to one exported `ParseLargeCommunity` in `attribute/text.go`, eliminating both `parseSingleLargeCommunity` and `parseLargeCommunityText`.
- `internal/test/decode/decode_test.go` not created — the shared package is exercised through delegation by the existing `test/peer` and `test/runner` suites.
- Group G partial: items below the configured `dupl` threshold are structural similarity, not true duplication; abstracting would add complexity.

## Patterns
- Four dedup strategies: (1) package extraction for cross-package identical code, (2) helper extraction for repeated in-file patterns, (3) direct deletion when an unexported copy duplicates an exported function, (4) parameterization for pairs differing by one variable.
- Type aliases (`type LargeCommunity = attribute.LargeCommunity`) enable cross-package dedup without type conversion overhead.
- `capability.ParseFromOptionalParams()` is the natural home for parsing OPEN optional parameters — named by domain, not by caller.

## Gotchas
- `dupl` at threshold 100 reports 34 groups; at 150 only 7 — always check the configured threshold before planning scope. Items below threshold are not worth abstracting.
- Structural similarity (same shape, different semantics) should keep `//nolint:dupl`, not be forced into a shared helper.

## Files
- `internal/test/decode/decode.go` — new shared BGP message decode types and functions
- `internal/component/bgp/capability/capability.go` — added `ParseFromOptionalParams()`
- `internal/component/plugin/server.go` — `ExtractConfigSubtree` exported
- `internal/component/config/serialize.go` — `serializeNonSchemaValues()` helper extracted
