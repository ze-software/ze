# 1335 - cache-consumer-declared-before-reactor

Route loss in interop scenario 54, fixed 2026-08-04. No spec. The defect
surfaced when a session took that scenario's workaround out.

## The defect

`(*Coordinator).RegisterCacheConsumer` (`internal/component/plugin/coordinator.go`)
forwarded a plugin's cache-consumer declaration only when a BGP reactor was
already attached, and dropped it without a word otherwise. A plugin declares
`cache-consumer` in startup Stage 1
(`engineStartupSink.onRegistration`, `plugin/server/startup.go`), and the BGP
reactor is not built until the *bgp* plugin's Stage 2 `OnConfigure`
(`bgp/plugin/register.go` `runBGPEngine`). `bgp` and `bgp-rs` both carry
`ConfigRoots: ["bgp"]` and neither depends on the other, so they share a startup
tier, and `runStartupHandshake` holds every tier member at the
StageRegistration-to-StageConfig barrier. **No config-path cache-consumer
declaration has ever reached the reactor.** `RecentUpdateCache.RegisterConsumer`
was never called in a full daemon run.

Consequence: `bgp-rs` declares `CacheConsumerUnordered: true`, so
`SetConsumerUnordered` was never called and `RecentUpdateCache.Ack` applied FIFO
cumulative-ack semantics to it. bgp-rs batches a forward and flushes it on
worker drain. An announce therefore sat in an unflushed batch while the
End-of-RIB behind it acked a HIGHER message id. That End-of-RIB carries no
family, so `processForward` releases it and never forwards it. The cumulative
sweep then evicted the announce's entry, and the flush logged `BUG:
ForwardUpdatesDirect: msgID missing from cache`. The UPDATE reached no peer, and
nothing replayed it.

`pluginLastAck` was never seeded with `highestAddedID` either, which is the same
bug's second half for a consumer that registers while traffic is flowing.

## Traps

- **The symptom lied about its trigger.** It looked like "the first UPDATE after
  a long quiet session", and both the scenario comment and the first two
  hypotheses chased time-based reclamation (safety valve, pressure valve, buffer
  reclaim, worker idle timeout). None of those fire at 50 s, and none were
  involved. The quiet period only decided WHICH UPDATE was followed by an
  End-of-RIB inside one worker drain window.
- **Reading found the wrong story three times.** What settled it was six
  temporary `Error`-level traces and one docker interop run. The traces went in
  `Add`, `Activate`, `Ack`, `evictLocked` (with `runtime.CallersFrames`),
  `Decrement` and `RegisterConsumer`. The decisive datum was the one log line
  that never appeared.
- **`Ack` works for an unregistered plugin.** `pluginLastAck[plugin]` returns 0
  for an unknown name, so a dropped registration produces no error anywhere: it
  silently downgrades an unordered consumer to FIFO.
- A guard that returns early on a missing dependency at a startup-ordering
  boundary is a defect generator (`ai/rules/evidence.md`). Record the fact and
  replay it when the dependency arrives; do not make the caller responsible for
  the order.

## What the fix activated, and had to fix too

`nonFIFOConsumers` was empty in every daemon run, so the unordered branch of
`UnregisterConsumer` was dead code. Registering bgp-rs correctly made it live,
and it carried two soundness holes of its own. Both lose routes, and both are
fixed in the same commit.

- It walked EVERY entry and decremented each, with no way to tell one this
  consumer had already acked per-entry. A second decrement takes the count to
  zero under a consumer that has NOT acked, and evicts its UPDATE. Fixed with
  one bit per unordered consumer in `cacheEntry.ackedUnordered`.
- It also walked entries added BEFORE the consumer registered. `pluginLastAck`
  marks that point at registration, but an unordered consumer's acks move it, so
  the watermark is now kept apart in `consumerFloor`.

**`seqmap.Since` broke its own promise, and the first attempt at the floor was
built on it.** `Since` binary-searches the insertion log and its type contract
requires non-decreasing sequence numbers (`internal/core/seqmap/seqmap.go`).
`RecentUpdateCache` does not supply them: `notifyMessageReceiver` takes the id
from `nextMsgID()` and reaches `Add` only after the ingress filter chain, so two
peers' read goroutines Add out of id order routinely. Two separate failures came
out of that, and they need different answers.

- `Since` iterated from the search index to the end of the log WITHOUT
  re-testing `seq >= fromSeq`, so an out-of-order entry below the bound was
  handed to `fn`. `RecentUpdateCache.Ack`'s FIFO branch acks every entry it is
  handed, so that entry was acked by a plugin that never handled it and evicted
  under a consumer that still owed it. Measured: `Since(21)` over the log
  `[20,30,5]` returned `[30,5]`. Fixed inside `Since`, one line, which is the
  altitude that makes every caller safe rather than one call site.
- `sort.Search` over an unsorted log can also land PAST an entry above the
  bound and skip it. Two answers were tried. A `sorted` flag with a full-scan
  fallback was correct but dropped every FIFO `Ack` to O(n) under an exclusive
  lock on the per-UPDATE path, which is a hot-path regression traded for a
  correctness fix. What shipped instead keeps the log sorted at insert: `Put`
  appends when the sequence is at or above the tail, and slides it into place
  when it is not. The invariant then holds by construction, the binary search
  is always valid, and the flag is gone.
- While the fallback existed, a full scan ran in LOG order rather than sequence
  order, so an early `return false` inside a `Since` callback stopped on
  whichever entry arrived first. `Ack`'s cumulative sweep did exactly that; it
  now skips and keeps walking, which is right under either implementation.

**Two answers were needed because "make it correct" and "keep it fast" pulled
apart, and the first draft took the first and lost the second.** A reviewer
catching a correctness bug is not permission to pay for it anywhere.

**Do not read "the search index is wrong" as "the start is too low".**
`sort.Search` returns an index whose predicate tested true, so the START is
never below the bound. The over-visit happens after it, in the iteration.
An earlier draft of this fix carried that wrong mechanism in a code comment for
a whole review round while the code was right for a different reason.

Separately, `ForwardUpdatesDirect` returned on the destination cap,
`errNoDestinations` and `errNoPeersMatch` before its per-id loop, acking
nothing. A FIFO consumer's next cumulative ack swept those ids; an unordered one
has no sweep, so they pinned read buffers until the 5-minute valve. All three
refusal paths now ack the batch.

**The general shape: a dead branch is not a correct branch.** Before you make an
unreachable path reachable, read it as if it were new code.

## The fix

The Coordinator keeps `cacheConsumers map[string]bool`, name to unordered.
`RegisterCacheConsumer` records then forwards, and `UnregisterCacheConsumer`
deletes then forwards. `SetReactor` replays the recorded set onto the reactor as
it attaches, outside `c.mu`, because the reactor takes its own cache lock.

## Left open, deliberately

Three residuals, all RETENTION (a pinned pooled read buffer until the safety
valve) and never route loss. They need a spec, not a paragraph.

- `consumerFloor` infers "was this delivered to me" from the message id, which
  is the very inference this work showed to be unsafe: an entry whose id is
  below a consumer's floor but which reached `Add` AFTER that consumer
  registered was delivered to it and counted by `Activate`, yet the unregister
  walk skips it. Its obligation is never released and its ack bit is never
  cleared. The fix is an arrival watermark (a per-`Add` counter on the entry)
  rather than an id watermark, on `pluginLastAck` as well as `consumerFloor`.
- `Ack`'s `if id <= lastAck { return nil }` has the same shape: an entry Added
  after that watermark with a lower id was never swept, and the comment above
  the line says it was.
- `Plugin.ForwardCached`'s new "an error still consumes the ids" holds for every
  refusal the reactor makes, but `Server.forwardCached` returns "no reactor
  available" without acking. Harmless today (no reactor means no cache) and
  still an exception to an absolute sentence.

## Files

- `internal/component/plugin/coordinator.go` -- `RegisterCacheConsumer`, `UnregisterCacheConsumer` and `SetReactor`, which record then replay the declaration
- `internal/component/plugin/coordinator_test.go` -- declare-then-attach and the registration lifecycle
- `internal/component/plugin/server/startup.go` -- `engineStartupSink.onRegistration`, the Stage 1 declaration point
- `internal/component/bgp/plugin/register.go` -- `runBGPEngine`, the Stage 2 point the reactor is built at
- `internal/component/bgp/reactor/recent_cache.go` -- `RegisterConsumer`, `Ack`, `UnregisterConsumer`, `consumerFloor` and the per-consumer ack bit
- `internal/component/bgp/reactor/recent_cache_test.go` -- the unordered unregister and FIFO sweep tests
- `internal/component/bgp/reactor/forward_update_test.go` -- the three refusal paths that must still ack the batch
- `internal/core/seqmap/seqmap.go` -- `Since`'s bound re-test and `Put`'s sorted insert
- `internal/core/seqmap/seqmap_test.go` -- the bound, the miss, and the sorted-log invariant
- `test/interop/scenarios/54-local-pref-strip-gobgp` -- the scenario whose workaround removal surfaced the defect

## Evidence

- `internal/component/plugin/coordinator_test.go`:
  `TestCacheConsumerDeclaredBeforeReactorReachesIt` drives declare-then-attach
  and asserts the unordered flag arrives.
  `TestCacheConsumerRegistrationTracksLifecycle` covers attach-then-declare and
  unregister. Mutation: deleting the replay loop turns the first one RED.
- `internal/component/bgp/reactor/recent_cache_test.go`, three tests, each
  mutation-checked against its own line:
  `TestCacheUnorderedUnregisterKeepsEntriesItAlreadyAcked` (delete the acked-bit
  skip: RED), `TestCacheUnorderedUnregisterSkipsPreRegistrationEntries` (delete
  the `id > floor` test: RED; swap `Range` for `Since`: RED, because its ids are
  laid out so the search index lands past a post-registration entry),
  `TestCacheUnorderedAckBitClearedOnUnregister` (delete the bit clear: RED).
- `internal/core/seqmap/seqmap_test.go`
  `TestSinceHonorsBoundOnUnsortedLog`, `TestSinceVisitsEveryEntryOnUnsortedLog`
  and `TestPutKeepsTheLogSorted` (make `insertLog` a plain append: the first
  goes RED with an out-of-bound entry, and the other two catch the miss and the
  broken invariant).
- `internal/component/bgp/reactor/recent_cache_test.go`
  `TestCacheFIFOCumulativeAckSweepsOutOfOrderEntries` (restore the sweep's early
  `return false`: RED, two entries left unacked).
- `internal/component/bgp/reactor/forward_update_test.go`
  `TestForwardUpdateDirectRefusalAcksTheBatch`, three refusal paths (delete both
  `ackBatch` calls: all three RED).
- `test/interop/scenarios/54-local-pref-strip-gobgp` lost the re-announce that
  hid this. Measured RED without the fix (`msgID missing from cache` id=29,
  FRR and GoBGP hold nothing), GREEN with it.
