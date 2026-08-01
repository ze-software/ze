# 902: PATHS-LIMIT Capability + Unified ADD-PATH Config

## Context

Ze needed PATHS-LIMIT capability support (draft-abraitis-idr-addpath-paths-limit, code 76) and its add-path config had two locations: boolean send/receive on the capability container, and a per-family list at the peer level.

## Decisions

- **Unified add-path config:** merged both locations into `session > capability > add-path { direction ...; family { ... { direction ...; limit ...; mode ...; } } }`. Default direction applies to all negotiated multiprotocol families; per-family entries override.
- **Direction-aware PathsLimit maps in EncodingCaps:** `PathsLimitSend` (remote's limits, constrains our send) and `PathsLimitRecv` (our limits, constrains peer's send). Mirrors the existing AddPathMode pattern.
- **EncodingContext derives per-direction pathsLimit:** send context reads PathsLimitSend, recv context reads PathsLimitRecv. Included in hash for dedup.
- **Enforcement in CommitService only:** static route initial sync doesn't need enforcement (config-originated, one path per prefix). RS fast-path suppresses the PATHS-LIMIT capability entirely (no per-prefix state in forwarding hot path).
- **Limit 0 = skip on parse:** matches ExaBGP behavior and draft semantics.
- **Duplicate AFI/SAFI: first entry wins** on wire parse.

## Consequences

- Old add-path config syntax (`send true; receive true;` and peer-level `add-path` list) is replaced. Tests and docs updated.
- YANG schema breaking change: old configs with boolean leaves or peer-level add-path list will be rejected.
- PATHS-LIMIT entries are only stored for families that also have ADD-PATH negotiated.

## Gotchas

- The write hook blocks `fmt.Sprintf` and string concatenation in wire/format code; use `textbuf.Buffer` chain methods instead.
- The write hook blocks `_, _ = h.Write(...)` for hash writes; extracted `hashSeparator()` helper to isolate the pattern.
- Config tests must use `map[string]any` trees matching the new YANG structure, not the old format.
- ExaBGP migration must handle both capability-level add-path (direction) and neighbor-level add-path (per-family with optional limit) as separate sources combined into the unified output.

## Files

None recorded.
