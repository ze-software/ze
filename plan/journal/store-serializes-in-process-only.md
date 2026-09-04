# Store serializes in process only

A store guards its writers with an in-process lock and installs its file by
rename, so it is correct for every goroutine in one daemon and silently
last-writer-wins across two. The defect does not present as an error. The second
process writes a whole, valid file, and the first process's state is simply not
in it. Nothing logs, nothing fails, and the loss is discovered later as state
that was written and is no longer there.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-03 | local-ca | `BlobStore` (`pkg/zefs/store.go`), `WriteFile` -> `flush` -> `atomicWrite` | Walked into while sizing R-7 of `spec-local-ca`, which asked whether two daemons could each generate a CA root and overwrite the other. They can, and the root is not the interesting half: `BlobStore.WriteFile` takes `s.mu`, a `sync.Mutex`, and `atomicWrite` installs the WHOLE serialized blob through `os.CreateTemp` plus a rename. So a second daemon sharing one `database.zefs` replaces every key the first wrote, not merely the one it meant to change. The absence of a file lock is DELIBERATE and asserted: `TestBlobStoreNoFlock` (`pkg/zefs/store_test.go`) exists to hold it. What is missing is not the lock but the statement of the invariant it implies, one writing process per blob, anywhere a reader of the store would meet it. `docs/architecture/zefs-format.md` documents the layout and the key registry and says nothing about how many processes may open one blob | not fixed: recorded. The CA root inherits the exposure rather than creating it, so `spec-local-ca` serializes generation inside the process and states the cross-process half here (owner decision, 2026-09-03). Two repairs are separable and neither is a CA spec's: state the single-writer invariant in `docs/architecture/zefs-format.md` beside the format it constrains, and decide whether a second opener is refused rather than allowed to clobber. `TestBlobStoreNoFlock` is the current answer to the second question and it answers only the mechanism, not the consequence |
