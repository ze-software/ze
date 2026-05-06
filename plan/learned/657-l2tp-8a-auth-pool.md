# Learned: spec-l2tp-8a-auth-pool -- Local Auth + IP Pool

## What Was Built

Local authentication plugin (`l2tpauthlocal`) supporting PAP and CHAP-MD5.
IP address pool plugin (`l2tppool`) with bitmap-backed allocation.

## Key Decisions

- **SHA-256 + constant-time compare for PAP.** Not raw cleartext comparison.
  Follows RFC 1334 Section 2.2.1 while avoiding timing side-channels.

- **CHAP-MD5 per RFC 1994 Section 4.1.** Standard `MD5(id || secret || challenge)`
  construction with constant-time compare.

- **MS-CHAPv2 explicitly rejected by local auth.** Only the RADIUS backend
  supports MS-CHAPv2 (requires server-side NT hash infrastructure).

- **Bitmap allocator.** `[]uint64` bitmap, linear scan for first free bit.
  O(n/64) worst case. Adequate for BNG scale, simple to reason about.

- **Pool release via EventBus.** Pool subscribes to `SessionDown` events.
  A `sync.Map` maps `{tunnelID, sessionID}` to the allocated `netip.Addr`.
  On session-down, `LoadAndDelete` releases the address back to the bitmap.

- **Pool replacement safety.** `OnConfigApply` rejects pool replacement if
  any addresses are currently allocated. Must tear down sessions first.

- **User map behind sync.RWMutex.** Updated atomically via `setUsers()` on
  config delivery. Hot-reloadable.

## Patterns Worth Reusing

- Bitmap + sync.Mutex is the simplest correct pool allocator. No free list
  fragmentation, no allocation on the hot path beyond the bitmap scan.
- EventBus subscription for resource cleanup (pool release) keeps the pool
  decoupled from the L2TP session lifecycle code.
