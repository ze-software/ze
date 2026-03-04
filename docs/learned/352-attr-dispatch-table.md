# 352 — Attribute Dispatch Table Conversion

## Objective

Replace attribute-handling switch statements with `[256]`-indexed dispatch tables for O(1) lookup and one-line attribute registration.

## Decisions

- Three switch sites converted; fourth (`separateMPAttributes` in `wireu/split.go`) skipped — only 3 cases, not worth the indirection.
- Parsers that don't need `fourByteAS` wrapped with `func(d []byte, _ bool)` adapter to fit uniform signature.
- Phase 3 uses an `attrInterner` struct binding pool + getter/setter to avoid 12 near-identical switch cases.
- Inspired by freeRouter's `bgpAttrsRx[256]` pattern.

## Patterns

- `[256]T` array indexed by attribute type code — O(1), no map overhead, nil = unhandled.
- `init()` registration: each entry is a single line, adding a new attribute never touches dispatch logic.
- Adapter wrappers normalize heterogeneous function signatures into a uniform table entry type.
- `attrInterner` struct pattern: when all cases follow the same template (get handle → release → intern → set handle), factor the varying parts (pool, getter, setter) into a descriptor.

## Gotchas

- "Known without parser" codes (PMSI, TunnelEncap, AIGP, BGPLS, PrefixSID) are left `nil` in the parser table — they fall through to `OpaqueAttribute`, same as truly unknown codes. Don't confuse "nil entry" with "bug".
- The validator table in `rfc7606.go` uses local `uint8` constants, not `attribute.AttributeCode`, to avoid cross-package coupling from the message package to the attribute package.

## Files

- `internal/component/bgp/attribute/wire.go` — `knownAttrParsers` table + `parseKnownAttribute`
- `internal/component/bgp/message/rfc7606.go` — `attrValidators` table + extracted per-attribute validator functions
- `internal/component/bgp/plugins/bgp-rib/storage/attrparse.go` — `attrInterners` table + unified `ParseAttributes` loop
