# OSPF AS-external routes, redistribution and default-information

ASBR Type 5 origination (RFC 2328 Section 12.4.4), the Section 16.4 external
computation, bidirectional redistribution, and `default-information originate`.

## Decisions

- **Two route paths that are never merged.** FIB install is
  `locrib.Path` to sysrib to fibkernel at AdminDistance 110. Redistribution is
  the redistribution events path in and out, and it never touches the kernel.
  OSPF resolves intra, then inter, then E1, then E2 internally and publishes ONE
  winning `locrib.Path` per prefix. Path TYPE is the primary key: E1 always
  beats E2 whatever the cost.
  <!-- source: internal/plugins/ospf/spf/external.go -- betterExternal -->
  <!-- source: internal/plugins/ospf/redistribute/redistribute.go -- ExternalInjector, OptionalInjector -->
- **The redistribution source name and the consumer name are both the single
  string `ospf`.** The generic loop-prevention evaluator then rejects OSPF
  self-import with no OSPF-specific code.
  <!-- source: internal/plugins/ospf/redistribute/source.go -- source -->
  <!-- source: internal/plugins/ospf/redistribute/consumer.go -- consumer -->
- **`default-information originate` lives on the engine, not in the
  redistribution package**, because it reads engine and LSDB state. `always`
  originates unconditionally. The bare form originates only while a NON-OSPF
  default exists in the Loc-RIB, re-evaluated at config-apply and live through a
  Loc-RIB change watcher. OSPF runs in-process and shares the `locrib.Default()`
  singleton, so the engine reads the RIB directly.
  <!-- source: internal/plugins/ospf/default.go -- applyDefaultInformation -->

## Traps

- **A new concurrent reader exposes a latent LSDB race. Fix the LSDB, do not
  serialize the caller.** The origination path mutated the `*Entry` returned by
  `install()` after the lock was released, while the self-external count read the
  same fields under `RLock`. Single-threaded origination hid it. The fix holds
  the LSDB mutex across install, header read and purge marking.
- **`locrib.RIB.Lookup` returns a shallow `PathGroup` copy whose `Paths` slice
  shares the stored backing array.** Ranging it after the shard lock is released
  races an in-place upsert. Scan inside `RIB.Inspect` or `RIB.Iterate`, under the
  lock.
- **Two independent intents that share one LSA key need a coordinated lifecycle,
  not a self-ownership flag.** `default-information originate` and a
  redistributed `0.0.0.0/0` share one Type 5 key. Both intents are tracked, and
  the key is purged only when neither wants it. All default-route mutations are
  serialized under one mutex.
- **A method called from two goroutines that reads, decides and writes shared
  state is serialized end to end.** Per-field locking left a window in which a
  stale watcher run re-originated a default that a concurrent config disable had
  just withdrawn. Lock order: default-information mutex, then engine mutex, then
  LSDB mutex.
- **A Loc-RIB change handler runs UNDER the shard write lock and must not
  re-enter the RIB.** The handler does a non-blocking send to a coalescing
  buffered channel, and a long-lived worker does the work outside the lock.
- **Conditional default-information excludes OSPF's own default**, or it
  self-sustains.
- **A purge bypasses MinLSInterval.** RFC 2328 Section 14.1 flushing is not
  rate-limited.
- **Self Type-5 LSAs are refreshed from the AS-wide store as well as the area
  stores.** A self-refresh that walks only the area stores lets every
  redistributed external, the originated default and every NSSA-translated Type
  5 reach MaxAge and be purged about `LSRefreshTime` after the last topology
  change. That blackholes the routes domain-wide while redistribution is active.
- **One bool cannot encode "no change" AND "install failed".** An external
  origination that returns a single `changed` flag hides a store-full rejection,
  and the consumer still counts the route as injected. Failures get their own
  error channel, and store exhaustion is logged where the install is rejected.
