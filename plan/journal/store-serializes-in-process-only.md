# Store serializes in process only

A store guards its writers with an in-process lock and installs its file by
rename, so it is correct for every goroutine in one daemon and silently
last-writer-wins across two. The defect does not present as an error. The second
process writes a whole, valid file, and the first process's state is simply not
in it. Nothing logs, nothing fails, and the loss is discovered later as state
that was written and is no longer there.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-03 | local-ca | `BlobStore` (`pkg/zefs/store.go`), `WriteFile` -> `flush` -> `atomicWrite` | Walked into while sizing R-7 of `spec-local-ca`, which asked whether two daemons could each generate a CA root and overwrite the other. The raw blob writer held an in-process mutex and installed the whole serialized blob by rename, so a second process could replace keys written by the first. `TestBlobStoreNoFlock` (`pkg/zefs/store_test.go`) deliberately required no file lock. The record found no statement of the resulting single-writer invariant in `docs/architecture/zefs-format.md` | Recorded without a repair in the CA spec, by owner decision on 2026-09-03. Reconciled 2026-09-19 with the storage migration: `docs/architecture/zefs-format.md`, Single-process ownership, now states the raw-blob invariant. `storage.OpenBlob` and `CreateBlobPopulated` (`internal/component/config/storage/blob.go`) use the stable `<artifact>.lock` sidecar through `lockStoreFile` (`open.go`), which refuses conflicting ownership with `ErrBusy`; raw `pkg/zefs` remains a separate contract. `plan/pre-release/spec-storage-2-blob-artifact.md` owns offline-editor exclusion across blob rewrites. Preserve that ownership rather than opening a second lock feature from this historical row |
