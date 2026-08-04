# 1341 -- fixit-isis-lsdb-entry-race

## Context

`TestISISDISElection` failed under `-race` on `Entry.lifetime`. The IS-IS aging tick
decremented it under the LSDB write lock. SNP generation read it through the plain
accessor `Lifetime()`, with no lock held.

`LSDB.Lookup` (`internal/plugins/isis/lsdb/lsdb.go`) hands out the LIVE `*Entry` and
releases the read lock. Every caller therefore holds a pointer it can read at any time.
The database had a lock discipline that its own exported accessors bypassed. The goal was
to fix that discipline, not the one reported field.

## Decisions

- Two atomics (`lifetime` -> `atomic.Uint32`, `purged` -> `atomic.Bool`) over locking
  every accessor, and over renaming them `...Locked` to push the lock to callers. Both
  alternatives restructure every off-lock caller (SPF, show, DIS, SNP) to hold the lock
  across the read, to fix a two-field problem. Both also cost the SNP hot path a lock per
  field read.
- `setLifetime` stays UNEXPORTED over exporting a `SetLifetime` for symmetry with
  `Lifetime()`. The atomic exists for unlocked READERS. Writes stay the lock's job, and an
  exported setter invites a write from outside it.
- `atomic.Uint32` over `atomic.Uint64`, and over manual `atomic.LoadUint32`. It is the
  narrowest typed atomic Go offers. `setLifetime`'s `types.RemainingLifetime` parameter
  keeps the value domain at uint16, so the parameter type enforces the narrowing, not the
  field.
- `recvPurgeReflooded` and `deleteAt` stay PLAIN, with the reason in the field doc, over
  making them atomic "for consistency". Consistency teaches that post-publication mutation
  alone forces an atomic. That is the misreading that produced the defect.
- `IsReceivedPurge` deleted over keeping it with a corrected doc. An exported accessor
  with no non-test caller is dead code, and it sat over a LOCK-ONLY field. That is what
  the discipline forbids.

## Consequences

- **"Does this field need an atomic?" is a CONJUNCTION of two questions.** Condition 1:
  is it mutated after `store.entries[id] = e` published the entry? Condition 2: is it
  read with no lock? Conflating them fails in both directions. Assume 1 decides it, and
  `deleteAt` and `recvPurgeReflooded` become atomic for nothing. Assume 2 decides it, and
  `sequence`, `checksum` and `raw` do. Exactly two of fourteen fields satisfy both.
- The classification rests on one choice made elsewhere. `replaceLocked` does not update
  a stored `Entry` in place. It builds a fresh one and swaps the map slot. That is what
  makes "written once before the entry is reachable" true for seven of the fourteen
  fields. **Change `replaceLocked` to mutate in place and `sequence`, `checksum`,
  `typeBlock`, `own` and `raw` all get condition 1 at once. Every one is read off-lock.**
- The reported race was one field. The CAUSE was a sentence. `Lookup`'s doc said "callers
  that only read metadata are fine". A read races a concurrent write whether or not the
  reader also writes. The doc was actively wrong, and it is why the SNP path felt safe to
  write. Fix the field without the sentence and the next accessor repeats the mistake.
- `Entry.IsOwn()` was exported with no non-test caller, and the fix was a genuine fork:
  delete the accessor, or wire `own` into `show isis database`. Deferred on purpose so
  the owner picked. **Ruled 2026-08-04: wire it.** `LSDB.Snapshot` now reads `IsOwn()`
  into `LSPSnapshot.Own`, and both database views carry an `own` field. The deferral
  shard was removed with the fix.

## Gotchas

- **A `-race` regression test can pass with the bug present.** The race detector being the
  oracle is what hides it. The committed test caught a reverted `lifetime` atomic in only
  6 of 12 single runs, and a reverted `purged` in 10 of 12.

  The mechanism: all eight LSPs reached the purge state within the first few tick
  iterations. `markPurgedLocked` then early-returns, and the tick wrote NO entry field for
  the remaining ~150 ms. The reader/writer overlap collapsed to microseconds. The fix
  keeps the WRITER writing. The test re-originates the LSPs when the tick reports them
  purged, and both mutations then reproduce 12/12.

  **Mutation-verify concurrency tests too.** "The oracle is `-race`" makes the step feel
  skippable. A coin-flip test is worse than none in the `-count=1` suite `ze-verify` runs.
- **Build mutants the runner's way.** Use `runner.TestBuildTags()`
  (`internal/test/runner/runner.go`), never a hand-written tag list. `go vet` the mutant
  before you trust a green run. A mutation whose `sed` did not apply reports as "mutation
  survived", and retires a good test.
- **An enumeration discipline applied to the CODE does not govern the PROSE about it.** It
  does not govern the code's OTHER comments either. The false claim "one of only two
  fields mutated AFTER the entry is published" was corrected in `entry.go` by a dedicated
  follow-up commit. The identical wording survived in `LSDB.Lookup`'s doc, one file away,
  for a week. The independent Review Gate found it. The struct doc's own condition-1 list
  named four of the seven qualifying fields.
- **The spec repeated the shape a third time.** Three citation errors in its first pass,
  and six more found at closure. One `file,line` named the CALLER where the rule requires
  the producer, in the section that DIAGNOSES the bug.
- **Fifteen green Implementation Audit rows are not a review.** This spec was presented as
  closure-ready on the strength of its own audit, while `review_gate.py check` was BLOCKED
  for want of any artifact. The gate, run independently, found two ISSUEs the audit had
  not. The audit checks the spec's own checklists. The gate checks what nobody planned for.

## Files

- `internal/plugins/isis/lsdb/entry.go` -- the two atomics, `Lifetime()`/`IsPurged()`
  atomic loads, unexported `setLifetime`, the FIELD DISCIPLINE struct doc,
  `IsReceivedPurge` deleted
- `internal/plugins/isis/lsdb/aging.go` -- tick decrement and `markPurgedLocked` write
  through `setLifetime`/`purged.Store`
- `internal/plugins/isis/lsdb/lsdb.go` -- `replaceLocked` and the clause 7.3.16 duplicate
  refresh write through `setLifetime`/`purged.Store`. `Lookup`'s doc rewritten twice: once
  to replace "callers that only read metadata are fine", once at closure to remove the
  surviving false enumeration
- `internal/plugins/isis/lsdb/aging_test.go` -- `TestISISLSDBEntryAccessorsAreRaceFree`.
  The tick against four reader goroutines, every exported accessor in the reader loop, and
  a writer that re-originates purged LSPs so it never stops writing
- The deferral shard that held the `IsOwn()` row was removed on 2026-08-04, when the
  owner ruled and the row was resolved by wiring the accessor into `LSDB.Snapshot`
