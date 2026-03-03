# 058 — Update ID Forwarding

## Objective

Add `update-id` to received UPDATE events so API programs can forward UPDATEs to other peers by ID without re-parsing, using a one-shot cache that deletes entries on use.

## Decisions

- Scope: one UPDATE = one ID, regardless of NLRI count (an UPDATE with 100 NLRIs gets one ID)
- One-shot: `forward update-id <id>` deletes from cache after sending; `delete update-id <id>` acknowledges without forwarding
- TTL (60s default) is a safety fallback for orphaned entries; lazy cleanup on Add() — no background goroutine
- Cache is fixed size; drops new entries if full after eviction (signals misconfiguration, not a silent error)
- `ReceivedUpdate` and RIB Route share the same `attrs *AttributesWire` pointer — no duplication of attribute data
- Negated peer selector `!<ip>` = "all peers except this IP" for source-excluding forwarding
- Expired ID lookup fails with explicit error; no fallback scan — API programs must use fresh IDs

## Patterns

- `Take()` (not `Get()`) removes the entry and transfers ownership — prevents race between concurrent forwards of the same ID
- O(n) scan on each Add() for TTL cleanup — acceptable when maxEntries is ≤10K

## Gotchas

- Zero-copy forwarding requires source and dest peers to share the same `ContextID`; otherwise attributes must be re-encoded
- `packNLRIsFor` reuses wire bytes when contexts match, re-encodes with dest context otherwise

## Files

- `internal/reactor/received_update.go` — ReceivedUpdate type, updateID
- `internal/reactor/recent_cache.go` — RecentUpdateCache, TTL-based eviction
- `internal/reactor/reactor.go` — `assignUpdateID()`, `GetUpdateByID()`, `ForwardUpdate()`
