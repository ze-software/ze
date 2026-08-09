# Session observer and continuous quality measurement

Per-session event rings record the session lifecycle. Per-login sample rings
record echo round-trip quality in 100-second buckets.

<!-- source: internal/component/l2tp/observer.go -- Observer, ObserverEvent, eventRing, eventRingPool, sampleRingPool -->
<!-- source: internal/component/l2tp/cqm.go -- CQMBucket, BucketState, sampleRing, BucketInterval -->

## Decisions

**Rings come from pre-allocated free-list pools.** Both pools are sized at
startup from the configured session and login maxima. Nothing allocates on the
hot path: a ring is handed out when a session comes up and returned when it goes
down.

**Quality history is keyed by login, events are keyed by session id.** The same
login reconnecting on a new session id continues the same sample ring, while the
event ring starts fresh. Reconnection is the common failure mode on a broadband
network gateway, so quality data that resets on reconnect answers the wrong
question.

**Least-recently-used eviction for logins.** A doubly-linked list orders the
logins. When the login maximum is reached, the least recently used login's
sample ring is reclaimed and the new login takes the slot.

**One mutex for all observer state.** Observer writes are rare next to packet
processing, so there is no contention to split.

**A 100-second bucket.** Each bucket holds a start timestamp, the state
(established, negotiating, down), an echo count, and the minimum, maximum and
summed round-trip time. The average is the sum over the count. A negative
round-trip time is clamped to zero.

## Traps this code exists to avoid

**Closing a bucket must not spin on a large time gap.** A system suspend or an
NTP jump moves the clock by more than the ring holds. The close loop caps its
iterations at the ring capacity plus one.

<!-- source: internal/component/l2tp/observer.go -- maybeCloseBucket -->

## Patterns worth reusing

- A free-list pool for ring buffers: pre-allocate N, hand one out on session up,
  return it on session down. No garbage collection pressure on the hot path.
- Keying quality data by login rather than by session survives a reconnect.
