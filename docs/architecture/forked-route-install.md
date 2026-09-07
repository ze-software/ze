# Route Install from a Forked Plugin

OSPF, IS-IS and the static plugin do not program the FIB. They insert
`locrib.Path` values into the process-wide Loc-RIB singleton, sysrib arbitrates,
and fib-kernel programs the result as `RTPROT_ZE`.

In a forked plugin subprocess, `locrib.Default()` returns nil, because the
singleton lives in the engine's address space. Both installers short-circuited
every insert and remove: SPF still computed and still snapshotted its routes,
and not one route reached the kernel. This is a silent black hole, so the
forked path carries the routes over RPC instead of failing fast.

<!-- source: internal/component/plugin/server/dispatch_route.go -- route-install and route-remove RPC -->
<!-- source: internal/core/rib/routeinstall/sink.go -- forked sink, buffering and retry -->
<!-- source: pkg/plugin/rpc/types.go -- RouteInstallEntry and RouteNextHop, the wire form -->

## The wire carries a name, the engine resolves it

The RPCs are `ze-plugin-engine:route-install` and `:route-remove`, both with a
batch payload. The engine applies the batch to the real `locrib.Default()` and
sysrib's `OnChange` carries it to the kernel unchanged.

A protocol ID is per-process, allocated in registration order, so it cannot
travel. The payload names the protocol and the engine looks the name up with
`redistevents.ProtocolIDOf`, then REJECTS an unknown name.

**The engine also RE-STAMPS the administrative distance.** The declaration seam
(`internal/core/rib/distance`) is process-global and sysrib is its only publisher,
so a forked producer never sees it and ships whatever bootstrap constant its own
package holds. `rib { distance { ospf 5 } }` was therefore inert for a forked OSPF
at the point arbitration happens, and nothing said so. `applyRouteInstall` reads
the declaration for the protocol the entry names and takes the wire value as the
fallback, so a protocol the declaration does not name keeps what its producer
chose.

**The entry carries the whole path.** Besides the gateway, `RouteInstallEntry`
carries the outgoing device, the next-hop's weight, the forwarding action
(`route-type`, the `routetype` number) and the equal-cost set (`ecmp`, one
`RouteNextHop` per member). A forked producer's blackhole route and its multipath
were dropped at this boundary until the fields existed, because the entry had
nowhere to put either.

Register-on-demand was the first cut and is a crash vector: a forked plugin
could exhaust the global protocol registry, which panics at about 65535 names.
Rejection is safe because the engine binary registers every in-tree protocol at
package init, regardless of config and regardless of forking. A third-party
external route installer under a new protocol would need an explicit
registration mechanism, which is not built.

## A sink per installer, not a remote Loc-RIB

Each installer holds a `RouteSink`: the local Loc-RIB in process, and the RPC
sink when forked. Making `locrib.Default()` return a remote proxy was refused,
because it would change a core contract and cross the module tiers.

The sink buffers and flushes once per SPF apply, so a delta costs one install
RPC and one remove RPC, with a bounded retry on a transient error.

<!-- source: internal/component/sysrib/sysrib_forked_withdraw_test.go -- withdrawal on plugin disconnect -->

## Disconnect withdraws, and that conflicts with graceful restart

The engine tracks the routes each plugin installed and withdraws them in
`cleanupProcess`, so a crashed forked plugin leaves no stale kernel route.

Forked graceful restart route retention is therefore NOT supported. A GR
restart is a subprocess disconnect, and the engine-side withdrawal cannot see
the plugin's GR suppress state. Resolving it needs a retain-across-restart
signal from the plugin to the engine.

Ordering makes the withdrawal safe against an in-flight install:
`handleSingleProcessCommandsRPC` registers `defer cleanupProcess` before
`defer wg.Wait()`, and LIFO runs the wait first, draining installs before the
withdrawal.

## Scope

The route-install path is forked-only. An in-process or bridge plugin holds a
non-nil local Loc-RIB and uses the local sink, so there is no direct-dispatch
twin of this RPC. The static plugin adopted the same shape when its main-table
routes moved into the Loc-RIB: `runStaticPlugin` builds with `locrib.Default()`
and installs a `routeinstall.Sink` when that answers nil. The BGP RIB shares the
nil-Loc-RIB-when-forked property and has not adopted `RouteSink`.

A kernel route needs an on-link next hop, so the kernel-level test adds a dummy
interface before it asserts that fib-kernel accepted the route.
