# 057 — AttributesWire: Wire-Canonical Attribute Storage

## Objective

Replace dual storage (parsed attributes + wire cache) in Route with `AttributesWire` — wire bytes as the canonical representation with lazy per-attribute parsing on demand.

## Decisions

- `packed []byte` is NOT owned by AttributesWire — caller retains ownership (zero-copy); the underlying message buffer must outlive the struct
- `attrIndex` (built lazily on first scan) stores offset+length for each attribute, avoiding re-scanning on every lookup
- Two-level cache: index built once on first access, parsed attributes cached individually on demand
- `PackFor(destCtxID)` enables zero-copy forwarding: if source and dest contexts match, return `packed` directly; otherwise re-encode
- Location note in spec: design was later moved — wire-canonical storage lives in API programs via base64-encoded wire bytes in JSON events, not in the engine's Route struct

## Patterns

- `sync.RWMutex` inside AttributesWire: read lock for cache hits, write lock for index build or parse
- `Get()` does double-check locking: RLock → check cache → RUnlock → Lock → check again → build index → parse

## Gotchas

- Use-after-free footgun: if the message buffer is returned to pool while AttributesWire still references it, all subsequent access is undefined behaviour — deliberately accepted for performance, enforced by convention not by the type system
- `Has()` checks cache then index (if built) without parsing the value — avoids unnecessary allocation when only checking presence
- `All()` takes write lock (not read lock) because it builds the index and populates the parse cache

## Files

- `internal/bgp/attribute/wire.go` — AttributesWire, attrIndex, Get/Has/GetMultiple/All/PackFor
