# 070 — ADD-PATH Encoding Simplification

## Objective

Fix inconsistent ADD-PATH handling across NLRI types by separating concerns: NLRI types encode payload only; a shared `WriteNLRI()` helper prepends the 4-byte path ID when negotiated.

## Decisions

- `Len()` and `WriteTo()` return payload only (no path ID) — ADD-PATH is handled by the caller via `WriteNLRI(n, buf, off, ctx)` and `LenWithContext(n, ctx)` helpers
- `hasPath bool` field removed from INET, IPVPN, LabeledUnicast — the context tells the caller whether to add path IDs, not the NLRI
- `Pack()` kept as deprecated wrapper (many callers not yet migrated)
- EVPN types not updated — they do not currently support ADD-PATH (separate work)
- `PathID()` still stored on NLRI (needed for uniqueness in RIB keying, not encoding)

## Patterns

- Four-phase approach with backward compatibility: Phase 1 adds new methods alongside old; Phase 2 updates callers; Phase 3 changes semantics (breaking); Phase 4 removes transitional methods
- New NLRI types need no ADD-PATH logic at all — they inherit it free from `WriteNLRI()`

## Gotchas

- EVPN bug existed because `packEVPN()` assumed `Bytes()` included the path ID when `hasPath=true`, but EVPN's `Bytes()` never included path IDs — wire format corruption with ADD-PATH + EVPN
- `route.Index()` and `store.Key()` must include the path ID for NLRI uniqueness, even though `WriteTo()` excludes it — two different concerns (wire format vs identity)

## Files

- `internal/bgp/nlri/nlri.go` — `WriteNLRI()`, `LenWithContext()`, NLRI interface updated
- `internal/bgp/nlri/inet.go`, `ipvpn.go`, `labeled.go` — `Len()` and `WriteTo()` return payload-only, `hasPath` removed
- `docs/architecture/wire/NLRI.md`, `ENCODING_CONTEXT.md`, `edge-cases/ADDPATH.md` — updated
