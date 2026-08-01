# 764: BGP Attribute Flags in Ze Native JSON

## What

Added RFC 4271 attribute flags (optional, transitive, partial) to Ze native JSON output. Each attribute value is wrapped: `{"value": <v>, "optional": bool, "transitive": bool, "partial": bool}`. Plugin event format (ExaBGP) is unchanged.

## Key Decisions

- **Wrap per-attribute, not sibling map.** Each attribute carries its own flags rather than a parallel `"flags"` map at the `"attr"` level. Keeps flag data co-located with the value it describes.
- **`includeFlags bool` parameter on shared formatter.** `appendAttributeJSON` serves both the ExaBGP path (`false`) and Ze native path (`true`). The flag controls wrapping without duplicating the function.
- **Static flags for pool-based RIB.** Pool storage normalizes flags and drops Partial. `enrichRouteMapFromEntry` uses compile-time constants per attribute type (e.g., `FlagTransitive` for Origin, `FlagOptional` for MED). Wire flags are only available in the decode path.
- **`next-hop` is not wrapped.** It's a route property set by both pool attributes and direct assignment in `serializeRouteItem`. Wrapping would create inconsistent types for the same key. Flags for NEXT_HOP are always well-known transitive, adding no information.
- **LG unwraps at `extractRoutes`.** A single `unwrapRouteAttrs(rm)` call strips wrappers before data reaches templates, graph builder, and CSV export. The birdwatcher API path uses `getStr`/`getVal`/`getNum` which handle both wrapped and plain.

## Gotchas

- **Every consumer of the route map must handle the new shape.** The LG UI handler, graph code, templates, CSV export, and birdwatcher API all access attribute keys directly. Missing even one consumer produces `map[optional:false ...]` in output instead of the value. Grep for all direct access patterns (`route["origin"]`, `route["as-path"]`, etc.) across the codebase before claiming done.
- **Human-readable formatter needs type switches for pre-marshal Go types.** `parsePathAttributesZe` produces `[]uint32` for AS-path, but `.([]any)` only matches after JSON marshal/unmarshal. The human formatter runs on the raw Go map, so it needs both `case []uint32` and `case []any`.
- **Functional tests that assert attribute values break.** `test/plugin/decode-update.ci` used `attr.get('origin') != 'igp'` which fails when origin becomes a dict. Any test comparing attribute values must unwrap first.

## Pattern: Format Divergence with Shared Core

When the same data needs two JSON shapes (ExaBGP flat vs Ze wrapped), parameterize the shared formatter rather than forking it. The `includeFlags bool` approach keeps one code path for attribute serialization. The wrapper helpers (`attrKeyOpen`, `attrFlagsClose`) isolate the structural difference to two small functions.

For map-based output paths (RIB show), use a wrapper constructor (`attrWithFlags`) that returns the enriched map. For buffer-based paths (format package), use open/close helpers that conditionally emit the wrapper syntax.

## Files

None recorded.
