# 782 -- Policy Action Macros (pol-2)

## Context

Ze's policy chain had the right structure (piped filters, text delta, wire-level modification) but lacked the action vocabulary operators need daily. The modify filter could only set absolute values. There was no way to increment/decrement integer attributes, add/remove individual communities in the policy chain, or filter by AS-path length. Operators had to work around these gaps with external plugins or manual community tag/strip (which operates outside the policy chain). The goal was to add operator-intent macros that match attribute semantics: set/inc/dec for integers, add/remove for lists, accept/reject by path length.

## Decisions

- Extended the existing modify filter over creating new filter plugins for inc/dec and community ops. The modify plugin already handles attribute mutations; adding new operation types keeps the config surface unified under `bgp/policy/modify`.
- Inc/dec computes absolute values at the plugin level (reads current from update text, does arithmetic, returns delta). The engine sees only AttrModSet, so no engine changes needed for inc/dec.
- Community add/remove uses new text directives (`community-add`, `community-remove`, etc.) that textDeltaToModOps maps to AttrModAdd/AttrModRemove. The existing communityAttrModHandler already supports Add/Remove, so only the text-to-wire bridge needed extension.
- Remove ops split into per-value chunks (one AttrModRemove per community value) over passing multi-value buffers, because removeValues in the handler requires exactly valueSize bytes.
- as-path-length as a standalone filter plugin over adding length checks to the AS-path regex filter, because intent differs (length is a simple numeric bound, not a regex match).
- Community values validated at config load time (ParseCommunity/ParseLargeCommunity/ParseExtCommunity) over deferring to runtime, to fail fast on misconfiguration.

## Consequences

- Operators can now increment/decrement local-preference, med, and aigp with saturation semantics.
- Community add/remove works for all three types (standard, large, extended) within the policy chain.
- AS-path length filtering provides a simpler alternative to regex for common "reject long paths" policy.
- The text delta format gained 6 new directive names. Any code that extends isPolicyAttrName or formatFilterAttrs must include these.
- buildDynamicDelta uses textbuf.Buffer (pool-backed) to avoid per-UPDATE allocations.

## Gotchas

- Multi-value community-remove silently did nothing in the first implementation because encodeCommunityValue concatenates all values into one buffer, but removeValues expects exactly valueSize bytes. Fixed by splitting Remove ops per value in textDeltaToModOps.
- The functional test hex payloads were initially malformed: wrong path-attribute length for LOCAL_PREF, wrong AS_PATH segment count byte. Both caught during review.
- policySingleToken map was rebuilt inside parseFilterAttrs on every call (hot path). Promoted to package-level var.

## Files

- `internal/component/bgp/plugins/filter_modify/{config,modify,filter_modify}.go` -- inc/dec, community ops, dynamic delta
- `internal/component/bgp/plugins/filter_modify/schema/ze-filter-modify.yang` -- increment/decrement/community YANG
- `internal/component/bgp/plugins/filter_aspath_length/` -- new plugin (8 files)
- `internal/component/bgp/reactor/filter_chain.go` -- 6 community directive names, policySingleToken
- `internal/component/bgp/reactor/filter_delta.go` -- communityDirectives map, per-value Remove split
- `internal/component/plugin/all/all.go` -- blank imports for as-path-length
- `docs/{features,comparison,guide/plugins}.md` -- documentation updates
- `test/plugin/{modify-increment-localpref,modify-community-add,aspath-length-reject}.ci` -- functional tests
