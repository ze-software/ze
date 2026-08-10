---
kind: directive
level: MUST
stage:
---
- You MUST register the key in `pkg/zefs/keys.go` (`meta/<subsystem>/<name>`; use a `{placeholder}` for per-entity keys and `Key(param)` to fill it).
- `statestore` is **best-effort**: `Put` is a no-op when no blob store is registered (filesystem-fallback mode). Persistence MUST stay non-fatal.
- **One shared instance, not a transient open.** The config system opens `database.zefs` once at startup and holds that single `*zefs.BlobStore` for the process; a flush re-encodes the whole file from its in-memory tree. Writing state through a SEPARATE transient store would let the config store's next flush drop every state key (and a state write could revert a concurrent config commit). So `statestore` MUST write through that same handle (registered with `SetStore` in `cmd/ze/hub`), serialized by the store's own lock: one tree, no lost updates. A write still rewrites the whole store per flush, so cadences MUST stay modest (best-effort caches, not per-packet).
