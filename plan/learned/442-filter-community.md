# 442 -- Community Filter Plugin

## Context

Ze needed operator-configurable community filtering: tag and strip standard, large, and extended communities on ingress and egress, with cumulative config inheritance across bgp/group/peer levels. This is the first of three planned filter plugins (community, prefix, IRR). The infrastructure changes (filter priority, PeerFilterInfo identity, ze:cumulative, AttrModHandler pattern) benefit all three.

## Decisions

- Chose two-level filter priority (FilterStage + FilterPriority) over single-level, because RFC-mandated filters (loop detection), operator policy (community/prefix), and protocol annotations (OTC stamping) are fundamentally different classes. Stage constants with gaps (0/100/200) allow future insertion without renumbering.
- Chose ingress=direct payload mutation, egress=ModAccumulator+AttrModHandler over uniform approach, because the ingress path returns modified payload (IngressFilterFunc contract) while egress uses the existing progressive build pipeline (buildModifiedPayload). No new patterns introduced.
- Chose plugin-side config accumulation via ForEachPeer+mergeFilterConfigs over relying on ResolveBGPTree, because plugins receive the raw config tree (not the resolved tree). The ze:cumulative extension in deepMergeMaps still works for PeerSettings, but the plugin does its own 3-layer merge.
- Chose toAnySlice helper over modifying Tree.ToMap(), because ToMap() is used broadly and changing its output types would be a breaking change. toAnySlice normalizes []string/string/[]any at the merge boundary.

## Consequences

- New plugins with filters MUST set FilterStage explicitly; zero value = FilterStageProtocol (highest priority).
- AttrModHandlers for community codes 8, 16, 32 are now registered; any future plugin wanting to modify these attributes must coordinate or replace the handler.
- The loop filter now has stub RunEngine/CLIHandler to satisfy registration validation. A filter-only registration category would be cleaner but is deferred.
- ze:cumulative extension is available for any future YANG leaf-list that needs config-level accumulation.

## Gotchas

- buildModifiedPayload passes full attribute (header+data) as src to AttrModHandler, not just the value. The handler MUST parse the header via extractAttrValue. The OTC handler copies src verbatim (preserving header), but community handler needs to strip it. This contract mismatch was caught in deep review R1.
- Tree.ToMap() produces []string for multi-values and bare string for single values, but deepMergeAt and anySliceToStrings were initially coded for []any only (the JSON round-trip type). Both needed type-switch handling for all three variants. Caught in deep review R2.
- make generate drops the reactor/filter import from all.go because the generator only scans plugins/. The import must be manually maintained with a comment explaining why.
- Non-extended-length attributes (1-byte length field) must be promoted to extended-length when tagging increases data past 255 bytes. The ingress tag function always uses extended-length format to avoid this edge case.

## Files

- `internal/component/bgp/plugins/filter_community/` -- plugin (9 .go + 1 .yang)
- `internal/component/plugin/registry/registry.go` -- PeerFilterInfo, FilterStage, sorted filters
- `internal/component/bgp/config/resolve.go` -- deepMergeMaps cumulative, toAnySlice, cumulativePaths
- `internal/component/bgp/reactor/reactor_notify.go` -- ingress PeerFilterInfo population
- `internal/component/bgp/reactor/reactor_api_forward.go` -- egress PeerFilterInfo population
- `internal/component/config/yang/modules/ze-extensions.yang` -- cumulative extension
- `internal/component/bgp/reactor/filter/register.go` -- loop filter stubs
- `test/parse/community-filter.ci` + `test/plugin/community-*.ci` -- functional tests
