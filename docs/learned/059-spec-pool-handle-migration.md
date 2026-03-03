# 059 — Pool Handle Migration (Abandoned)

## Objective

Design and implement migration from direct `[]byte` storage in Route to a single `pool.Handle` reference, enabling memory deduplication for large route tables. Abandoned: this optimization belongs in an API-level route reflector, not the edge speaker.

## Decisions

- Abandoned: pool/handle optimization deferred to API programs (e.g., `zebgp-rr`), not the engine
- Two modes were designed: RIB mode (reusable connection buffer + pool.Intern — 1 copy) vs API mode (allocate per read, 0 copies, GC-managed)
- Single global pool (not per-peer): cross-peer deduplication requires global scope; same bytes = same handle
- `WireUpdate.Release()` is a no-op in API mode (GC handles it); `PooledUpdate.Release()` would have decremented refcount in RIB mode

## Patterns

- In RIB mode: `conn.Read(reusableBuf)` → `pool.Intern(buf[:n])` → reuse buffer immediately (1 copy into pool)
- In API mode: `buf := make([]byte, msgLen)` → `conn.Read(buf)` → WireUpdate owns buf (0 copies)
- Use `uint32` for offset calculations in UPDATE parsing to avoid overflow with RFC 8654 extended messages (65535 bytes)

## Gotchas

- Spec was superseded before full implementation; kept for reference if building an API-level RIB with heavy deduplication needs
- `PooledUpdate` stores attrHandle (4 bytes) + NLRISet — significantly smaller than storing raw bytes per route

## Files

- Design only (abandoned) — planned files: `internal/bgp/wire/wire_update.go`, `internal/bgp/wire/pooled_update.go`
