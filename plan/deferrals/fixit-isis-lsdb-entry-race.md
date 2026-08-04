# Deferrals -- spec-fixit-isis-lsdb-entry-race

Opened at CLOSURE (2026-08-04), not during implementation. The spec made no
deferral while the fix was written. The one row below is a finding of the
independent Review Gate. It was raised after the code had landed. It is homed
here, not folded into the closing commit (`ai/rules/rule-precedence.md`,
"Closing comes first").

This shard OUTLIVES its source spec. It holds a live row, so commit B does not
remove it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-04 | spec-fixit-isis-lsdb-entry-race, Review Gate run 1 (ISSUE-2) | `Entry.IsOwn()` (`internal/plugins/isis/lsdb/entry.go`) is exported and has ZERO non-test callers. Its one caller repo-wide is the reader loop of `TestISISLSDBEntryAccessorsAreRaceFree` (`internal/plugins/isis/lsdb/aging_test.go`). That call was added by this spec's own test-strengthening pass. Production code reads the `own` field DIRECTLY instead, under the lock. Two sites do so: `LSDB.publishSizeMetricLocked` (`lsdb.go,555`) and `LSDB.tickLevelLocked` (`aging.go`), which builds `PurgeEvent.Own`. `IsReceivedPurge` was deleted in `71f91c170` on exactly this standard, and it had TWO test callers. The standard was applied to one accessor and not to its neighbour in the same commit | NOT goal-blocking, which is why it is homed rather than fixed on the way out. `own` is written once by `replaceLocked`, before the entry is published. So `IsOwn()` is not a race, and no acceptance criterion of this spec turns on it. It is deferred rather than done because the fix is a genuine FORK. Picking a branch is the owner's call, not a closure agent's. **Branch A, delete.** Three lines go: the accessor, its doc, and the one call in the test's accessor sweep. The accessor inventory drops from nine to eight. **Branch B, wire it.** `LSPSnapshot` (`lsdb.go,485`) carries `LSPID`, `Sequence`, `Lifetime`, `Checksum`, `Overload` and `Purged`, but NO ownership field. So `show isis database` cannot today tell an operator which LSPs this node originated. Adding it is a user-visible CLI change, with a JSON key, a YANG description, doc text and a `.ci` assertion behind it. Branch B is a small feature. Branch A admits the accessor was never needed | `-` (unhomed on purpose. Neither branch has an owning spec, and creating one would presuppose the answer. Raise with Thomas) | deferred |
