# 569 -- cmd-5 AS-Path Filter Plugin

## Context

Ze's vendor parity audit (cmd-0-umbrella) identified AS-path regex filtering as a gap across all four reference platforms (Junos, EOS, IOS-XR, VyOS). The policy framework (541-policy-framework) and prefix-filter (548-cmd-4-prefix-filter) had already landed, establishing the `ze:filter` augment pattern, `FilterTypes` resolver, and `PolicyFilterChain` dispatch. The AS-path filter needed to follow the same pattern but with regex matching instead of CIDR matching.

## Decisions

- Followed the `bgp-filter-prefix` pattern exactly over inventing a new plugin structure, because the framework was proven and the pattern cookie-cutter.
- Used Go's `regexp` package (RE2 semantics, linear time guarantee) over a custom matching engine, because RE2 inherently prevents ReDoS without needing timeout-based protection.
- Added a 512-character regex length limit as defense in depth over relying solely on RE2's linear time guarantee, because long regexes still consume memory and compilation time.
- Chose accept/reject-only (no modify action) over the prefix-filter's partition/modify path, because AS-path is an UPDATE-level attribute shared by all prefixes -- per-prefix partitioning is meaningless for AS-path matching.
- Normalized AS-path text by stripping brackets (`[65001 65002]` -> `65001 65002`) over passing the bracketed form, because regex authors expect space-separated ASNs (matching Junos/Cisco convention).
- Used `[0-9]` in .ci test configs over `\d`, because ze's config parser interprets backslash as an escape character, consuming the `\` before the regex reaches the plugin.

## Consequences

- AS-path filtering is now available via `filter import [ as-path-list:NAME ]` or `bgp-filter-aspath:NAME`. This unblocks cmd-6 (community-match) and cmd-8 (policy-show) which depend on multiple filter types existing.
- The filter framework now has two production filter types (prefix-list, as-path-list), validating that the pattern scales to additional filter types without framework changes.
- The `canonicalizeFilterRefs` and `BuildFilterRegistry` code required zero modifications -- the framework correctly discovered and resolved the new filter type purely through registration.
- Config authors must use character classes `[0-9]` instead of `\d` in regex strings. This is a gotcha worth noting if documentation is written.

## Gotchas

- Ze's config parser consumes backslashes in quoted strings: `"\d"` becomes `"d"`. Regex patterns must use explicit character classes instead of shorthand escapes. This cost a test iteration to discover.
- No changes needed to `filter_chain.go`, `filter_format.go`, or `config/peers.go` -- the framework wiring is entirely through registration. This was surprising (expected at least filter_chain.go changes).
- The plugin receives the whole UPDATE text (all attributes + NLRI), but only needs the `as-path` field. The `extractASPathField` function uses `strings.Cut` to find it -- a linear scan, not a structured parse. If the text format changes, this function breaks.

## Files

- `internal/component/bgp/plugins/filter_aspath/` -- complete plugin (7 Go files + 2 test files + 3 schema files)
- `test/plugin/aspath-filter-{accept,reject,shortform,chain}.ci` -- 4 functional tests
- `test/parse/aspath-list-config.ci` -- config parse test
- `cmd/ze/main_test.go`, `internal/component/plugin/all/all_test.go` -- plugin inventory updates
- `internal/component/plugin/all/all.go` -- generated (make generate)
