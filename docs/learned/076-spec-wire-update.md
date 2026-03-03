# 076 — Wire Update Types

## Objective

Implement zero-copy UPDATE message parsing using concrete wire types (`WireUpdate`, `MPReachWire`, `MPUnreachWire`), replacing per-parse allocation with GC-managed ownership for the API-only receive path.

## Decisions

- Chose concrete types over interfaces: no interface overhead, no type assertions; methods are direct calls.
- Chose GC over pool for API mode: no reference counting, no compaction — simpler lifetime model when the API callback owns the data until GC.
- `MPReachWire`/`MPUnreachWire` are `[]byte` type aliases with accessor methods: no struct overhead, zero-copy access to attribute bytes.
- `sync.Once` on `AttributesWire` in WireUpdate: the attrs are derived lazily and cached for the lifetime of the update — important because multiple callers (filter, text, forward) may access them.
- Buffer pool added to session (`readBufPool sync.Pool`): zero-copy means session transfers buffer ownership to WireUpdate, then gets a fresh buffer from pool.
- `ReceivedUpdate` migrated to hold `*WireUpdate` instead of separate raw bytes + attrs + source — eliminates duplicate storage.

## Patterns

- `MPReachWire(raw)` is a type conversion, not a struct creation: `raw, _ := attrs.GetRaw(AttrMPReachNLRI); return MPReachWire(raw)`.

## Gotchas

- `MessageCallback` signature changed to include `wireUpdate *api.WireUpdate` and `ctxID bgpctx.ContextID` — all session.go callsites needed updating.
- `session.recvCtxID` must be set by Peer via `SetRecvCtxID()` AFTER capability negotiation, not at session creation.

## Files

- `internal/plugin/wire_update.go` — WireUpdate, MPReachWire, MPUnreachWire
- `internal/plugin/types.go` — WireUpdate field added to RawMessage
- `internal/reactor/reactor.go` — WireUpdate creation in notifyMessageReceiver
- `internal/reactor/session.go` — readBufPool, buffer ownership transfer
