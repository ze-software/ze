# 1245 -- fixit-bgp-concurrency-races

## Context

A second BGP-reactor audit pass (2026-07-16) surfaced four concurrency/consistency
leads, all re-confirmed REAL against producers: (1) `FSM.change` released `f.mu` around
the state callback, so two transition callbacks could overlap and complete out of order,
leaving a torn-down session marked `Established`; (2) the sent-message path double-read
`peer.session` unlocked (`reactor_notify.go`), racing the peer-run goroutine's `p.session`
write -- a nil-deref window plus mis-attribution across reconnect; (3) a dynamic peer's
establishment wrote `PeerAS`/`ImportFilters`/`ExportFilters` with no lock while other
goroutines read them (router-ID conflict check, PeerInfo builders, API/plugin snapshots);
(4) a duplicate non-MP path attribute was skipped (`continue`) but its bytes were left in
the payload, so downstream indexed consumers (RIB `MPReach`, filters, cross-context
re-encode) disagreed -- MP routes silently vanished with no log, no treat-as-withdraw, no
NOTIFICATION. Goal: fix all four at the owning layer, each driven from a failing
reproduction, with `make ze-race-reactor` clean and no new lock-order edge.

## Decisions

- **Lead 1 -- per-FSM ordered non-blocking FIFO transition queue** (`f.pending`/`f.draining`,
  enqueue under `f.mu`, first enqueuer with no active drainer drains callbacks in order
  outside `f.mu`, enqueue never blocks) **over** (a) holding `f.mu` across the callback
  (self-deadlocks on any `State()` in the callback path -- RWMutex is non-reentrant), (b)
  blocking ticket order (deadlocks: the hold-timer fires `fsm.Event` while holding `s.mu`,
  session.go:433-435, while the to-Established callback needs `s.mu.RLock` via
  `Negotiated()`), (c) making the 110-line callback idempotent (fragile, still reorders
  observers). Uncontended transitions keep today's synchronous semantics; only the contended
  case (today's corruption window) becomes ordered-async.
- **Lead 2 -- carry the source-peer string through `MessageCallback`** from inside the
  sender's `writeMu` section **over** RLock-capturing `peer.session` (the forward_pool
  pattern): the callback-argument approach deletes the racy re-lookup entirely and attributes
  correctly across reconnect, because the value originates at the write site that already
  owns it under `writeMu`. Received-path reads stay lock-free with a same-goroutine
  justification comment (AC-5, verified-benign).
- **Lead 3 -- `p.mu`-guarded write + narrow `Peer` accessors** (`PeerAS`, `ImportFilters`,
  `ExportFilters`, `IsIBGP`, `IsEBGP`) **over** `atomic.Pointer[PeerSettings]` copy-on-write
  (the `Session` captures the settings pointer at construction, so a swap would strand the
  session on the pre-resolution snapshot) and `atomic.Uint32` PeerAS (does not cover the two
  slice fields). Exactly three fields mutate post-publication (A-4, re-grepped), so the
  accessor conversion is a bounded reader inventory; immutable fields (`LocalAS`) stay direct.
- **Lead 4 -- strip keep-first at the RFC 7606 boundary**: the validator records duplicate
  non-MP occurrences as byte ranges and `enforceRFC7606` rebuilds the body keep-first via the
  existing `RebuildUpdateBody` (ATTR_DISCARD) path; `ensureIndexLocked` stays strict
  (fail-closed) **over** relaxing the index guard to skip duplicates (weakens it for every
  caller, including locally built and forwarded wire) or treat-as-withdraw for non-MP
  duplicates (contradicts RFC 7606 Section 3.g). Decode is aligned (D-4b=YES) so `ze bgp
  decode` matches on-session behavior. Duplicate MP_REACH/MP_UNREACH stays session-reset.

## Consequences

- FSM transitions are overlap-free and ordered; sent-event attribution is correct across
  reconnect; `PeerAS`/filters are race-free; duplicate non-MP attributes are handled
  keep-first consistently across RIB/filters/re-encode/decode with the fail-closed index
  guard intact.
- **`MessageCallback` now carries `sentSourcePeerStr string`.** Any future sent-event
  producer MUST pass it from inside `writeMu`; received-path callers pass `""`.
- **The `Peer` accessors (`PeerAS`/`IsIBGP`/`IsEBGP`/`ImportFilters`/`ExportFilters`) are the
  mandatory cross-goroutine read path.** A direct `p.settings.PeerAS`/`.IsIBGP()` read is only
  safe when already holding `p.mu` (e.g. inside the op-queue drain) or genuinely
  same-goroutine; the accessors' godoc says so. Two benign same-class reads remain OUTSIDE
  this spec's four leads and are not regressions: `reactor_api.go` `OnPeerClosed`'s teardown
  read of `s.PeerAS`, and the RS fast path not setting `sentSourcePeerStr` (attribution gap,
  value preserved exactly as before). A future reactor-hardening pass owns them.

## Gotchas

- **A pre-existing `sessionLogger` DATA RACE blocked `ze-race-reactor` and was fixed at root,
  not parked** (`no-parking`): a periodic keepalive timer kept firing after two test paths
  reached Established without stopping the session (`setupEstablishedSessionEBGP` cleanup only
  closed the pipe; `Session.Run` never stopped timers on exit), and its callback read the
  package-global `sessionLogger` while a diag test swapped that var in `t.Cleanup` to measure
  zero-alloc-when-disabled. Fix: `sessionLogger` became an `atomic.Pointer`-backed provider
  with a test-only `swapSessionLogger(fn) (restore func())`; `Session.Run` now `defer`s
  `StopAll()`/`stopSendHoldTimer()`; the test setup stops timers. The zero-alloc assertion is
  byte-for-byte unchanged -- only the injection mechanism moved from a raw var swap to the
  atomic.
- **The Lead-3 fix shipped an incomplete sibling audit (review-caught).** The author
  converted the `PeerAS()` readers but left the `IsIBGP()` readers of the SAME mutable
  `PeerAS` direct -- the classic `before-writing-code` sibling-call-site miss. Converting them
  surfaced a SECOND trap: two `IsIBGP()` sites sit inside a `p.mu.Lock()` critical section, so
  the guarded accessor (which takes `p.mu.RLock()`) would DEADLOCK on the non-reentrant
  RWMutex -- those must stay direct reads (comment added). Classify each cross-goroutine
  reader by whether it already holds `p.mu` before converting.
- **The AC-3 `-race` test was flaky (review-caught, 30/30 fail under `-cpu=1`).** Its writer
  `select { case <-stop: return; default: }` always takes the ready `<-stop` after
  `close(stop)`, so under single-core scheduling the writer could return having NEVER
  resolved, leaving `PeerAS==0` and failing the final assertion. Fixed by resolving once
  synchronously before the writer; the sibling AC-2 test avoided this by having no
  writer-dependent final assertion (the correct pattern for a `-race` reproduction).
- **Lead-1 deadlock hazard confirmed AVOIDED by review, not just by testing:** the FSM is
  per-session, and the `draining` flag mutually excludes the two callbacks that take `s.mu`,
  so no `f.mu -> s.mu` cycle is closable; the queue REDUCES the pre-existing overlap hazard.
  `-race` alone does not flag Lead 1 (each shared field is individually atomic/locked) -- it
  needed the ordering-assertion test.
- **Line-cite drift:** a sibling RFC 7606 Section 5.3 commit shifted `message/rfc7606.go`
  ~+35 lines between the audit and implementation; producers were re-grepped before fixing
  (R-3). Sibling specs edit these same files -- re-confirm on HEAD.

## Files

- Lead 1: `internal/component/bgp/fsm/fsm.go`; tests `fsm/fsm_ordering_test.go`,
  `reactor/peer_established_ordering_test.go`.
- Lead 2: `reactor/reactor_notify.go`, `session.go` (`MessageCallback` +arg), `session_read.go`,
  `session_write.go`; test `reactor/reactor_notify_test.go`.
- Lead 3: `reactor/peer.go` (accessors), `reactor_dynamic.go` (guarded write),
  `routerid_unique.go`, `peer_initial_sync.go`, `reactor_api_batch.go`, `reactor_api_forward*.go`,
  `policy_dryrun.go`, `reactor_peers.go`, `forward_rs.go`, `filter_ordered.go`; test
  `reactor/reactor_dynamic_race_test.go`.
- Lead 4: `message/rfc7606.go`, `message/attr_discard.go`, `reactor/session_validation.go`,
  `cli/decode_update.go`; tests `message/rfc7606_test.go`, `reactor/session_validate_test.go`,
  `core/bgp/attribute/wire_test.go`, `cli/decode_duplicate_test.go`,
  `test/decode/bgp-update-duplicate-origin.ci`.
- Pre-existing race fix: `reactor/session.go` (atomic `sessionLogger` + `Run` timer-stop),
  `session_test.go`, `session_logger_swap_test.go`, `session_rfc7606_diag*_test.go`.
