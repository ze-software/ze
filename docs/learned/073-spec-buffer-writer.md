# 073 — Buffer Writer

## Objective

Implement the BufWriter interface and SessionBuffer type for zero-allocation UPDATE message building, adding `WriteTo(buf, off) int` methods alongside existing `Pack()` on attributes and NLRI types.

## Decisions

- Chose `[]byte` + offset over `io.Writer` interface: avoids the overhead of an extra interface dispatch and aligns with pooled-buffer pattern throughout the codebase.
- Kept `Pack()` methods indefinitely for external callers: additive migration, no breaking changes.
- Buffer lifecycle: allocate 4096 at session start, re-allocate to 65535 after Extended Message capability negotiated — ties buffer size to negotiated capabilities.
- Incremental migration by phase (infrastructure → leaf types → complex types → session integration → builders) to minimise risk and verify identical output at each step.

## Patterns

- `WriteTo(buf []byte, off int) int` returns bytes written; caller guarantees sufficient capacity.
- Session holds a `writeBuf []byte` parallel to `readBuf` — resize after capability negotiation.

## Gotchas

- Extended Message capability changes the max safe buffer size; the resize must happen after OPEN negotiation, not at session creation.
- Context-dependent encoding (ASN4, ADD-PATH) requires `WriteToWithContext` variants — easy to miss when migrating attribute types.

## Files

- `internal/bgp/wire/writer.go` — BufWriter interface, SessionBuffer
- `internal/bgp/attribute/` — WriteTo on all attribute types
- `internal/bgp/nlri/` — WriteTo on all NLRI types
- `internal/reactor/session.go` — writeBuf field
