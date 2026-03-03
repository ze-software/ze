# 302 — UTP-1 Text Event Format

## Objective

Implement a tokenizer-friendly text event format: uniform headers, comma-separated lists, `nlri <family> add/del` replacing `announce`/`withdraw`, and drop the `set` keyword from all NLRI `String()` methods. Enables bgp-rr to parse events without positional field indexing.

## Decisions

- Event format uses `nlri <family> add/del` (matches command format) instead of a separate `family` keyword — rejected the `family <afi/safi>` prefix because it was inconsistent with the command grammar
- `TextScanner` lives in `internal/plugins/bgp/textparse/` as a shared zero-alloc whitespace tokenizer; bgp-rr imports it rather than each plugin duplicating a scanner
- INET `String()` drops path-id entirely — formatter handles it via `nlri path-id N add`; `Key()` returns bare CIDR for map keys
- Parser unit tests replaced by handler integration tests (`TestHandleUpdate_ZeBGPFormat`) — stronger coverage than standalone parse function tests
- BGP-LS `Prefix` renamed to `Reachability` (String() outputs `reachability`)

## Patterns

- NLRI token boundary detection: collect tokens until a top-level keyword (from `topLevelKeywords` map) is seen or input ends; NLRI sub-field tokens never collide with top-level keywords across all 17 NLRI types
- Dual-format NLRI parser: comma format (`prefix a,b`) and keyword boundary format (`prefix a prefix b`) both accepted — comma is primary, keyword boundary supports complex multi-token NLRIs (EVPN Type 5: 12 tokens)
- Formatter outputs comma-grouped NLRIs per family section; parser accepts both forms

## Gotchas

- FlowSpec `String()` does NOT use `set` keyword (uses match operators directly) — safe to skip; confirmed by grep before changing
- INET `String()` inclusion of `prefix` keyword: initial assumption was bare CIDR; user required `prefix <cidr>` in String() output but bare CIDR in `Key()` for map lookups — required adding a separate `Key()` method
- Multi-NLRI format: initial design used one `nlri add` per NLRI; user required comma-grouped per family — changed formatter to `writeNLRIList()`

## Files

- `internal/plugins/bgp/format/text.go` — formatter (uniform header, comma lists, nlri add/del)
- `internal/plugins/bgp/textparse/scanner.go` — shared TextScanner
- `internal/plugins/bgp-rr/server.go` — parser rewrite (all 5 parse functions)
- All 10+ NLRI plugin `types.go` files — dropped `set` keyword from `String()`
