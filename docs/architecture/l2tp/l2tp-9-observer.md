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

**Quality history is keyed by login, events by the complete original Birth.**
Equal tunnel and session numbers on different listeners keep separate event
rings. A login reconnecting continues its bounded quality history while the
new admission receives its own event ring. Reads and release use the carried
Birth, never a reconstructed numeric key.

Raw events expose this reference as `subscriber-id`. Session summaries also
carry the original `tunnel-id`. An absent reference is omitted when an empty
formatting DTO is serialized; actual event admission requires a valid Birth.

<!-- source: internal/component/l2tp/observer.go -- ObserverEvent, SessionSummary, SessionEvents, releaseSessionLocked -->

**Least-recently-used eviction for logins.** A doubly-linked list orders the
logins. When the login maximum is reached, the least recently used login's
sample ring is reclaimed and the new login takes the slot.

**One mutex for all observer state.** Observer writes are rare next to packet
processing, so there is no contention to split.

Delivery checks a valid, matching native observation generation and admission
state under that mutex before changing rings, login state or metric inputs.
Retirement seals unknown lifecycle admission while preserving known originals.
A selected login contributor without an event ring can still deliver its first
Down; this matters when the event pool is full. An explicit captured disconnect
can allocate its own retained ring, but cannot write after closure. Stop closes
admission before unsubscribing, so an already copied callback is refused too.
History and quality snapshots remain readable after Stop.

<!-- source: internal/component/l2tp/observer_events.go -- observeEvent, recordDisconnect, wireObserverSubscriptions -->
<!-- source: internal/component/l2tp/observer.go -- RetainKnownSessions, Stop -->

**A login phase belongs to its selected contributor.** IP assignment selects
that Birth and establishes the phase. The first echo creating a login slot
selects a negotiating contributor; later echoes add samples without taking
over the phase. Only the selected contributor's first Down ends it. Older or
consumed Down does not promote the login in the LRU. Same-Birth assignment
reopens the phase, and a named pre-IP failure can create a Down observation.
Evicting the bounded slot forgets its provenance rather than growing history.

<!-- source: internal/component/l2tp/observer_events.go -- observeEvent -->
<!-- source: internal/component/l2tp/observer.go -- touchLogin, evictLRU -->

**A 100-second bucket.** Each bucket holds a start timestamp, the state
(established, negotiating, down), an echo count, and the minimum, maximum and
summed round-trip time. The average is the sum over the count. A negative
round-trip time is clamped to zero.

Echo counts and expected counts use 64 bits. The positive duration override
accepts intervals down to one nanosecond, which means up to 100 billion expected
echoes in a 100-second bucket. Summaries retain the complete bucket count.

The observer retains its original positive sampling interval separately from
the expected echo count. EchoState reports that exact interval even when it
does not divide 100 seconds or exceeds the bucket duration. Non-positive direct
observer intervals remain disabled; expected counts and loss still use the
existing whole-echo count for each bucket.
A live loss ratio is available only when that expected count is positive.
The echo view keeps an unavailable ratio separate from a computed zero, while
still reporting the exact positive interval when it exceeds one bucket.

The observer assigns bucket phases and event types locally, rather than reading
their numbers from a peer. Their `String` methods name every declared value and
panic with `BUG:` for an unnamed internal value. Existing zeros retain their
meaning: a zero bucket phase is `established`, and a zero event type is the empty
record, displayed as `unknown`.

## Traps this code exists to avoid

**Closing a bucket must not spin on a large time gap.** A system suspend or an
NTP jump moves the clock by more than the ring holds. The close loop caps its
iterations at the ring capacity plus one.

<!-- source: internal/component/l2tp/observer.go -- maybeCloseBucket -->

## Observation scope value

`events.ObservationGeneration` identifies one native generation within the
daemon process. Its constructor allocates a nonzero monotonically increasing
value under a cold-path mutex. Exhaustion returns an error without wrapping
or recycling a value. Copies compare directly without formatting or parsing.

The value is separate from a subscriber's identity and grants no authority to
change a session or a native resource. Its event JSON representation is a
canonical positive decimal string. Zero, malformed input and overflow are
refused; failed decoding preserves the receiver's previous value.

<!-- source: internal/component/l2tp/events/observation_generation.go -- ObservationGeneration, NewObservationGeneration, IsValid, MarshalText, UnmarshalText -->

One Start mints one scope before publishing any listener or observer. Its
reactors retain that same immutable scope and observer through delayed terminal
publication and unresolved native cleanup. Each observer uses fresh configured
pools. Partial Start unwinds its subscriptions, including a kernel-module probe
refusal after observer creation; the last original root closes its retained
observer. A pending original cleanup still refuses replacement
Start. Captures select that original observer rather than the subsystem's
current pointer, and a history read never allocates a ring.

<!-- source: internal/component/l2tp/subsystem.go -- Start, Stop, unwindLocked -->
<!-- source: internal/component/l2tp/session_retirement.go -- retireObserverLocked, closeUnretainedObserversLocked -->
<!-- source: internal/component/l2tp/session_capture.go -- CaptureSession, SessionEvents -->

## Patterns worth reusing

- A free-list pool for ring buffers: pre-allocate N, hand one out on session up,
  return it on session down. No garbage collection pressure on the hot path.
- Keying quality data by login rather than by session survives a reconnect.
