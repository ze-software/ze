# 543 -- Redistribution Filter Phase 2

## Context

The original redistribution filter spec (spec-redistribution-filter) built the policy filter chain infrastructure: external plugins declare named filters, config references them, PolicyFilterChain runs them with accept/reject/modify semantics. However, modify responses worked only at the text level -- applyFilterDelta merged text attributes, but the modified text was silently discarded without updating wire bytes. Five items were deferred: wire-level dirty tracking, export modify .ci test, undeclared attribute validation, raw mode, and three named unit tests. On investigation, AC-13 (undeclared validation) and AC-15 (raw mode) were already implemented in the original spec but the deferrals were never closed.

## Decisions

- Bridge function `textDeltaToModOps` compares original and modified text, encodes changed attributes to wire VALUE bytes, and feeds them as AttrModSet ops into the existing ModAccumulator/buildModifiedPayload pipeline, over building a separate wire modification path. Reuses all existing infrastructure.
- AS-PATH is skipped in the text delta, over allowing text-level replacement. EBGP AS-PATH prepend happens at the wire layer in ForwardUpdate; a text-level AttrModSet would clobber the prepended local AS, violating RFC 4271 Section 9.1.2. Plugins needing AS-PATH modification should use in-process egress filters with AttrModPrepend.
- Generic AttrModHandlers registered at reactor startup via `attrModHandlersWithDefaults()`, over init() registration. The hook system blocks implicit init() registration. Generic handlers cover well-known attributes (origin, next-hop, med, local-pref, etc.) that lack specialized handlers; community/OTC plugins keep their specialized ones.
- Attribute removal (present in original, absent in modified) emits a nil-Buf AttrModSet op, over silently preserving the attribute. The handler omits optional attributes and writes zero-length for well-known.

## Consequences

- Policy filter modify responses now produce actual wire-level changes in both import (cached UPDATE) and export (forwarded UPDATE) paths. Previously modify was text-only with no wire effect.
- All five deferrals from spec-redistribution-filter are resolved. The skeleton spec can be deleted.
- The generic handler infrastructure means any new well-known attribute automatically gets AttrModSet support without writing a custom handler.
- AS-PATH is explicitly excluded from text-level modification. This is a deliberate constraint, not a missing feature.

## Gotchas

- AC-13 and AC-15 were already implemented in the original spec but listed as open deferrals. Always verify deferral claims against actual code before starting implementation.
- The export path extracts text from the original wire (before EBGP prepend) but buildModifiedPayload runs against the EBGP wire. For all attributes except AS-PATH this is fine (same bytes). AS-PATH is the exception, hence the skip.
- Extended community encoding still uses Builder + stripAttrHeader because there is no public single-value ParseExtCommunity function. Community and large-community encode directly.

## Files

- `internal/component/bgp/reactor/filter_delta.go` -- textDeltaToModOps, per-attribute text-to-wire encoders
- `internal/component/bgp/reactor/filter_delta_handlers.go` -- genericAttrSetHandler, attrModHandlersWithDefaults
- `internal/component/bgp/reactor/filter_delta_test.go` -- 8 test functions, 20 subtests
- `internal/component/bgp/reactor/reactor_api_forward.go` -- export path wiring
- `internal/component/bgp/reactor/reactor_notify.go` -- import path wiring
- `internal/component/bgp/reactor/reactor.go` -- attrModHandlersWithDefaults() at startup
- `test/plugin/redistribution-export-modify.ci` -- AC-10 functional test
