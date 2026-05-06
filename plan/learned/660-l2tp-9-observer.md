# Learned: spec-l2tp-9-observer -- Observer + CQM

## What Was Built

Session event observer and continuous quality measurement (CQM) system.
Per-session event rings track lifecycle events. Per-login CQM sample
rings track echo RTT quality over time in 100-second buckets.

## Key Decisions

- **Pre-allocated ring pools.** Both event rings and sample rings come from
  free-list pools sized at startup (`MaxSessions` / `MaxLogins`). No
  runtime allocation on the hot path.

- **Login-keyed CQM continuity.** Same login reconnecting on a new session
  ID continues the same sample ring. Event ring starts fresh per session ID.
  This preserves quality history across reconnects.

- **LRU eviction for logins.** Doubly-linked list (`lruHead`/`lruTail`).
  When `MaxLogins` is reached, least-recently-used login's sample ring is
  reclaimed. New login uses the pre-allocated slot.

- **Single mutex.** All observer state behind one `sync.Mutex`. No
  contention at BNG scale (observer writes are infrequent relative to
  packet processing).

- **100-second CQM buckets.** `CQMBucket` holds Start timestamp, State
  (established/negotiating/down), EchoCount, MinRTT, MaxRTT, SumRTT.
  `AvgRTT()` = SumRTT/EchoCount. Negative RTT clamped to zero.

- **Spin cap on bucket close.** `maybeCloseBucket` caps iterations at
  `ringCap+1` to prevent spinning on large time gaps (e.g. system suspend
  or NTP jump).

## Patterns Worth Reusing

- Free-list pool for ring buffers: pre-allocate N, hand out on session up,
  return on session down. Zero GC pressure on the hot path.
- Login-keyed (not session-keyed) continuity for quality data survives
  reconnects, which is the common BNG failure mode.
