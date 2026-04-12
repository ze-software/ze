# 570 -- cmd-6 Community Match Filter Plugin

## Context

Ze's vendor parity audit (cmd-0-umbrella) identified community-based route filtering as a gap. The existing `bgp-filter-community` plugin handles tag/strip (adding/removing communities from routes) but has no accept/reject filtering capability. Operators need to reject routes carrying specific communities (e.g., no-export) or accept only routes with certain customer-tagged communities.

## Decisions

- Created a new `bgp-filter-community-match` plugin over extending the existing `bgp-filter-community`, because the existing plugin uses IngressFilter/EgressFilter (structural mutation, no FilterTypes) which is architecturally incompatible with the PolicyFilterChain accept/reject dispatch pattern. The two plugins have different intents (filtering vs modification) and different registration patterns.
- Named the FilterType `community-match` (referenced as `community-match:NAME`) over `community-list`, to be explicit about the accept/reject semantics and avoid confusion with the tag/strip plugin.
- Used string comparison against the text format representation over parsing communities to wire format, because the filter text protocol already renders communities in their canonical string form (ASN:VAL, well-known names, GA:LD1:LD2, hex). String matching is simpler and guaranteed correct against what the engine emits.
- Supported all three community types (standard, large, extended) in a single plugin over separate plugins per type, because community filtering often mixes types in a single policy.

## Consequences

- Community filtering is now available via `filter import [ community-match:NAME ]`. This is the third production filter type alongside prefix-list and as-path-list.
- The tag/strip plugin (`bgp-filter-community`) and the match plugin (`bgp-filter-community-match`) can coexist in the same filter chain -- one filters, the other modifies.
- Well-known community names (no-export, no-advertise, etc.) work as match values because they appear as names in the text format. No special resolution needed.
- cmd-8 (policy-show) now has three filter types to enumerate.

## Gotchas

- The spec said "extend existing bgp-filter-community" but that was architecturally wrong. The existing plugin has no FilterTypes and uses IngressFilter/EgressFilter, not OnFilterUpdate. A new plugin was required.
- Extended communities are matched as hex strings in the text format, which is less readable than the target:ASN:NN or origin:ASN:NN forms used in config. The match value in the YANG config must match the text format output exactly.
- The `strings.Cut` approach for extracting `community ` from the text format works because `community` appears after `cluster-list` and before `extended-community` in the fixed emission order. But `large-community` contains `community` as a substring -- the `strings.Cut` on `"community "` (with trailing space) correctly avoids matching `"large-community "` because the Cut finds the first occurrence and `community` comes before `large-community` in the format output order.

## Files

- `internal/component/bgp/plugins/filter_community_match/` -- complete plugin (7 Go files + 2 test files + 3 schema files)
- `test/plugin/community-match-{accept,reject}.ci` -- 2 functional tests
- `test/parse/community-match-config.ci` -- config parse test
- `cmd/ze/main_test.go`, `internal/component/plugin/all/all_test.go` -- plugin inventory updates
- `internal/component/plugin/all/all.go` -- generated (make generate)
