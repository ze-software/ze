# zefs Persistence (no loose state files)

**When:** Writing or reviewing code under `internal/plugins`, `internal/component`, or `cmd/ze` that needs to persist daemon runtime **state** across a restart, reconfigure, or update (a rolling baseline, a snapshot, a sequence number, a last-known value, a cache, a hash).
**Severity:** advisory

## The rule

Persist runtime state through the managed zefs store, never as a loose file.

- **Do:** `statestore.Put(key, data)` / `statestore.Get(key)` (package `internal/core/statestore`),
  keyed by a registered `pkg/zefs` key (`meta/<subsystem>/<name>` in
  `pkg/zefs/keys.go`).
- **Don't:** `os.WriteFile` / `os.Create` / `os.OpenFile(..., O_CREATE...)` /
  `os.Rename` a state blob into a path under the config/state dir.

## Why

On the gokrazy appliance the writable `/perm` partition holds exactly one managed
artifact: `database.zefs`. It is integrity-checked (`pkg/zefs` check/repair),
seeded at install, and understood by the image build/verify tooling. A loose
`state/foo.json` next to it is invisible to all of that: it is not backed up, not
verified, and silently gone after a reimage. `resolve.Storage()` already resolves
zefs-on-appliance / filesystem-fallback-on-dev; `statestore` is the plugin-facing
equivalent: it writes through the config system's OWN blob-store handle
(registered once at daemon startup via `statestore.SetStore`), so state and config
share one in-memory tree.

## How

```go
// save (best-effort; no-op when no blob store is registered)
data, _ := json.Marshal(snapshot)
_, _ = statestore.Put(zefs.KeyDDoSDetectBaseline.Pattern, data)

// restore
if data, ok := statestore.Get(zefs.KeyDDoSDetectBaseline.Pattern); ok {
    _ = json.Unmarshal(data, &snapshot) // keep version/sanity guards
}
```

- Register the key in `pkg/zefs/keys.go` (`meta/<subsystem>/<name>`; use a
  `{placeholder}` for per-entity keys and `Key(param)` to fill it).
- `statestore` is **best-effort**: `Put` is a no-op when no blob store is
  registered (filesystem-fallback mode). Keep persistence non-fatal.
- **One shared instance, not a transient open.** The config system opens
  `database.zefs` once at startup and holds that single `*zefs.BlobStore` for the
  process; a flush re-encodes the whole file from its in-memory tree. Writing
  state through a SEPARATE transient store would let the config store's next flush
  drop every state key (and a state write could revert a concurrent config
  commit). So `statestore` writes through that same handle (registered with
  `SetStore` in `cmd/ze/hub`), serialized by the store's own lock -- one tree, no
  lost updates. A write still rewrites the whole store per flush, so keep cadences
  modest (best-effort caches, not per-packet).

## Legitimate raw filesystem writes (allowlisted)

Not every `os.WriteFile` is state. These stay raw and are allowlisted in the
guard with a reason:

- **Kernel/device control:** `/proc`, `/sys`, sysfs, `/dev`, cgroup, ethtool.
- **Ephemeral scratch:** `/tmp`, `/run`, pid files, sockets, probe/ready files.
- **External artifacts:** files produced for another consumer -- `resolv.conf`,
  systemd units, PEM exports, MRT dumps, the ze binary during self-update, the
  externally-written `config-pushed.conf` inbox.
- **The storage layer itself:** `internal/component/config/storage`, `pkg/zefs`,
  and crash-time writers (`internal/core/crashlog`) that must survive a broken
  zefs. The append-only audit log (`internal/core/audit`) also stays a tailable
  file (a blob KV store is the wrong shape for an append log).

## Gate

`make ze-fs-persistence-check` (in `make ze-verify` / `ze-verify-changed`) runs
`scripts/checks/direct_fs_persistence.go`: it flags any non-allowlisted raw
filesystem write in the scanned trees. A new legitimate non-state writer needs an
allowlist entry (with a reason); genuine state must move to `statestore`.
