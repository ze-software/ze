# 912 -- irr-prefix-store

## Context

The BGP `filter_irr` plugin owned the entire IRR pipeline: per-ASN/AS-SET prefix
resolution (IRR whois + PeeringDB AS-SET discovery) and zefs persistence (a single
blob `meta/bgp/irr-cache`). The upcoming `firewall-irr` plugin needs the same
cached data, so the resolution+persistence layer was extracted into a shared
`PrefixStore`. The goal: one place that resolves a name (ASN or AS-SET) to a prefix
list and persists it, consumed by multiple plugins, with the BGP plugin's
operator-visible behavior unchanged.

## Decisions

- Put `PrefixStore` in a new `resolve/irr/store` **subpackage**, not in package `irr`, over same-package-with-an-interface. `peeringdb` imports `resolve/irr`; a same-package store importing `peeringdb` would cycle. The subpackage imports `peeringdb` directly (`store -> peeringdb -> irr` is acyclic), which deleted the `ASSetLooker` interface the spec originally proposed (a single-implementation interface = premature abstraction). The original "same package needed for unexported `aggregateAndSort`" rationale was false: `LookupPrefixes` already aggregates internally. (see docs/architecture/resolve.md)
- Each consumer constructs its **own** `PrefixStore`; they do NOT share an in-process instance. Reason: `filter_irr` is a process-isolated SDK plugin that builds its own clients from JSON config (it cannot reach the hub's `resolve.Resolvers`). Cross-consumer sharing happens through the zefs file on disk under per-key `meta/irr/{name}`. In-process writes are serialized by the store's `fileMu`; cross-process writes are NOT coordinated (zefs `Lock` is in-process only), so only one process may write a given store file until zefs gains a file lock. The spec's original "Resolvers gets a PrefixStore field" decision was dropped as incompatible with the plugin architecture.
- `Refresh(ctx, name, asSet)` keeps the ASN as the stable identity/key (`AS<asn>`) and takes the AS-SET as a separate hint, over keying by the AS-SET name. This lets a configured or previously-discovered AS-SET drive the query while migration (keyed by `AS<asn>`) stays consistent and PeeringDB rediscovery is skippable.
- Per-entry zefs keys (`meta/irr/{name}`) over the legacy single blob, with one-time migration on `Open` (write new keys, remove the legacy key only after).

## Consequences

- `firewall-irr` (spec-firewall-irr.md) can consume `store.PrefixStore` directly; it shares cached data with BGP via the zefs file, not a struct. New cross-process concurrency surface (R-4): zefs `Lock` is in-process only, so firewall-irr must not write the same store file concurrently with BGP until zefs gains a cross-process lock; in-process writes are serialized by `fileMu`.
- The shared store may hold entries a given consumer never enrolled (other plugins' ASNs, AS-SETs); a consumer must apply only entries it owns. `filter_irr.loadFromStore` keeps the old enrollment gate (apply only configured ASNs with an empty list).
- AS-SET names with `:` (e.g. `AS3333:AS-FOO`) are valid zefs keys: zefs splits tree segments only on `/` and decode validates with `fs.ValidPath` (permits `:`). The store rejects names containing `..` because `KeyEntry.Key()` panics on them.
- No YANG: the store is configured programmatically (clients + zefs path) by its callers.
- Shipped store API is `New`/`Open`/`Refresh`/`Get` (+ `CachedEntry`). `List` and `RefreshAll` were built first (AC-6/AC-11) but removed in /ze-review: they had no in-tree consumer (the BGP plugin uses `Get`/`Refresh`/`Open` plus its own per-ASN `refreshAllNow` and `byASN` iteration), so per wiring-completeness they belong to spec-firewall-irr, which adds them wired to its consumer. Lesson: build shared-library methods with their first consumer, not ahead of it.
- `validate.py`'s unwired-export check is a bare `grep -lw <symbol>`: a unique unused name (e.g. the removed `RefreshAll`) is flagged; a common word (`List`) is masked by unrelated matches; and a type used only via inference (`CachedEntry`, consumed as `entry.PrefixList()`/`.ASSet`) is a false positive because the type name is never spelled in the consumer.

## Gotchas

- `Open` initially took a zefs **write** lock on the shared `database.zefs` on every configure (the old `loadCache` did lock-free reads). In the functional test this delayed the refresh and the "irr filter modify" decision lost its race against the incoming UPDATE. Fix: read-first `Open` (lock-free reads; write lock only when a legacy blob must be migrated).
- The IRR client caches `LookupPrefixes` results for 1h in memory, so a `Refresh` can be a silent no-op within the TTL. `TestPrefixStoreRefreshError` must use a *fresh* client pointed at an unreachable server, or a warm cache masks the failure.
- On a lookup error, `Refresh` returns a **non-nil** `CachedEntry` (resolved AS-SET, no prefixes) plus the error so the BGP plugin can still record the fallback AS-SET; it must NOT update the in-memory map or zefs (AC-7: preserve cached data).
- `zefs.KeyEntry.Prefix()` for a template returns a trailing-slash prefix (`meta/irr/`); listing must use `Dir()` (`meta/irr`) because `walk("meta/irr/")` hits an empty trailing segment and returns nil.
- A name of `"."` (or any name containing `".."`) poisons the **whole** shared store: `irr.ValidateASSetName` allows `.`, so `Refresh(ctx, ".", …)` writes key `meta/irr/.`, which `fs.ValidPath` rejects at decode -> the entire zefs file fails to load (taking every other consumer's entries with it). `validateName` must reject `"."` and `".."` explicitly (caught in /ze-review; BGP never hits it since `asnName` only yields `AS<n>`, but the firewall consumer passes user-supplied names).
- zefs's `Lock()` is an in-process mutex (lock.go), NOT a cross-process file lock, and each `persist`/`Open` opens a **fresh** `BlobStore` -- so that lock serializes nothing between calls. Concurrent persists (in-process: a manual `update bgp irr asn` racing the background refresh; or cross-process once firewall-irr writes the same file) each open -> add key -> atomic-rename the whole file, so the loser's key vanishes. The old single-blob cache hid this by rewriting a full snapshot every time. Fix: a store-level `fileMu` serializes in-process persists (regression `TestPrefixStoreConcurrentPersist`); cross-process still needs a real flock in zefs (deferred to firewall-irr). Caught in /ze-review Run 3.
- Consumer-side: the BGP plugin's `plug.prefixStore` field (like the old `irrClient`/`pdbClient`) must be assigned under `plug.mu` in handleConfigure and read under it in refreshASN -- otherwise a reconfigure concurrent with a background refresh data-races on the pointer. Fixed by assigning under the lock and capturing into a local under the RLock (regression `TestRefreshASNStoreFieldRace`, -race). Caught in /ze-review Run 4.
- The whole reconfigure field-race class is one bug: `handleConfigure` must publish ALL mutable plugin fields (byASN, prefixStore, config, refreshStop) under `plug.mu` together, and any worker that captures `st := byASN[asn]` under an RLock must RE-READ `st = byASN[asn]` under the later write lock before mutating -- a reconfigure can swap byASN in between, orphaning the captured pointer (the refresh result is silently lost). Caught in /ze-review-deep (Run 5).
- `Open` must key the in-memory map by the on-disk zefs key segment, not the entry's self-reported JSON `Name`: the store file is shared, so a corrupt/tampered blob under `meta/irr/AS13335` claiming `"name":"AS99999"` would otherwise poison the AS99999 slot. Verify `segment == e.Name` and skip mismatches.
- Lock-order false positive worth remembering: a reviewer flagged a HIGH `store.mu`/`fileMu` deadlock, but `Refresh` does `s.mu.Unlock()` *before* `persist` acquires `fileMu` -- the two are never held simultaneously, so there is no inversion. Verify lock HOLDING windows, not just acquisition order in source position.

## Files

- `internal/component/resolve/irr/store/store.go` (created) -- PrefixStore, CachedEntry, migration
- `internal/component/resolve/irr/store/store_test.go` (created) -- unit + boundary tests
- `pkg/zefs/keys.go` -- registered `KeyIRRPrefixCache` (`meta/irr/{name}`)
- `internal/component/bgp/plugins/filter_irr/filter_irr.go` -- `refreshASN` -> store; `prefixStore` field; `asnName`
- `internal/component/bgp/plugins/filter_irr/cache.go` -- reduced to `cacheStorePath` + `loadFromStore`
- `internal/component/bgp/plugins/filter_irr/cache_test.go`, `filter_irr_test.go` -- store-based tests
- `docs/architecture/resolve.md` -- IRR Prefix Store section + anchors
