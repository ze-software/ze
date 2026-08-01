# 875: Filter Delta Parse-Once (spec-filter-delta-parse-once)

## Context

When a policy filter modifies a route, the reactor converts the text delta to
wire ops through three extractors in `filter_delta.go`: `textDeltaToModOps`,
`ExtractRemovePrivateASOps`, and `ExtractASPathPrependOps`. Each independently
called `parseFilterAttrs` on the modified filter text (and textDeltaToModOps
also on the original), so every modified UPDATE on the egress
(`reactor_api_forward.go`) and ingress (`reactor_notify.go`) hot paths paid
four tokenise-and-map parses where two suffice. Each parse allocates a map, a
`strings.Fields` slice, and joined value strings. Surfaced as a deep-review
follow-up during pol-4 (`plan/learned/814-pol-4-explain.md`).

## What Changed

- The three extractors (plus `extractRemovePrivateASMode`) now accept parsed
  `map[string]string` attribute maps. Call sites parse original and modified
  exactly once and share the maps read-only.
- Dry-run (`policy_dryrun.go`) hoists the parse above `computeChangedAttrs` +
  `computeWireChanges` (both converted to maps): 6 parses per dry-run -> 2.
- `parseFilterAttrsCalls` atomic counter in `filter_chain.go` as the test seam
  proving the parse count.

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Explicit map params over a parse cache or driver func | No hidden state on a hot path; read-only sharing is visible in the signature |
| Golden corpus captured from pre-refactor code (`TestFilterDeltaParseOnceEquivalence`) | AS-path manipulation refactors must be proven non-behavioral; 12 cases cover set/remove/prepend/remove-private/AS4-suppress/community directives/combined |
| Sorted-multiset comparison for golden ops | `textDeltaToModOps` iterates Go maps; op order was never deterministic, so byte-identical means same multiset, not same sequence |
| Parse-count test via atomic counter + retry-3 | One uncontended atomic add (~1ns) vs ~700ns parse is free; retry distinguishes deterministic regression (always +2) from background-goroutine counter noise |
| Test calls wrap with `parseFilterAttrs(...)` inline | Keeps existing test tables readable; tests document that callers own the parse |

## Gotchas

- The filter-text strings at the egress/ingress call sites are
  `unsafe.String` views over a stack scratch buffer. `strings.Fields`
  substrings in the parsed maps alias that buffer, so the maps must not
  outlive the synchronous modify block. They don't: every op buffer the
  extractors emit is freshly encoded wire bytes, never a map string.
- `extractLegacyNLRIOverride` still re-tokenizes the nlri block from raw text
  (block extraction, not a `parseFilterAttrs`); deriving it from the parsed
  maps' "nlri" key is a possible future cleanup, out of scope here.

## Results

`BenchmarkFilterModifyEgress` (Apple M4 Max, count 5, representative
modify with prepend + remove-private + community):

| | ns/op | B/op | allocs/op |
|---|---|---|---|
| before | 2447 | 3000 | 34 |
| after | 1501 | 1704 | 24 |

-39% time, -43% bytes, -29% allocations per modified UPDATE. Applies only
when a policy filter actually modifies a route; unmodified forwarding is
untouched.

## Files

None recorded.
