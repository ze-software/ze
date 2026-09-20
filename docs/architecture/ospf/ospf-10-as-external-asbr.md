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
- **Every default-route decision asks the address family first.** One function
  answers which prefix this engine advertises and which Loc-RIB table its
  condition reads: 0.0.0.0/0 in IPv4 unicast for OSPFv2, ::/0 in IPv6 unicast for
  OSPFv3. Origination then picks the LS Type the family needs, the OSPFv2 Type 5
  or the OSPFv3 0x4005 AS-External-LSA (RFC 5340 Section 4.4.3.6). RFC 5340
  Appendix A.4.2.1 reads the OSPFv2 value 0x0005 as U=0 with S2S1=00, link-local
  flooding scope, so an OSPFv3 engine originating the OSPFv2 type puts a default
  on the wire that reaches no router past the first hop.
  <!-- source: internal/plugins/ospf/default.go -- defaultRoute, originateDefaultExternal -->
- **The OSPFv3 default takes its Link State ID from the redistribution table.**
  RFC 5340 Section 4.4.3.6 strips the OSPFv3 external Link State ID of all
  addressing semantics, so nothing in ::/0 derives it. Allocating it from the one
  `redistV6` table is what makes the two intents reach the same LSA, and it also
  keeps the default inside the keep-set the redistribution withdrawal builds.
  <!-- source: internal/plugins/ospf/default.go -- v6OriginateDefaultExternal -->

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
  redistributed default share one AS-External key, in either address family:
  0.0.0.0/0 keyed Type 5 for OSPFv2, ::/0 keyed 0x4005 for OSPFv3. Both intents
  are tracked, and the key is purged only when neither wants it. All
  default-route mutations are serialized under one mutex, and the redistribution
  entry points hand the default to that coordinator BEFORE the address-family
  split, so neither family gets the coordination and the other the race.
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
