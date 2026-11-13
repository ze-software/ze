# Route Install from a Forked Plugin

OSPF, IS-IS, static routes and BGP selection do not program the FIB. They insert
`locrib.Path` values into the daemon's Loc-RIB. The Loc-RIB admits and selects
them, the system RIB resolves forwarding, and fib-kernel programs the result
as `RTPROT_ZE`.
<!-- source: internal/core/rib/locrib/manager.go -- insert -->
<!-- source: internal/core/rib/locrib/entry.go -- selectBest -->
<!-- source: internal/component/sysrib/route_view_forwarding.go -- applySelectedBatch -->

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
batch payload. The engine applies the batch to the real `locrib.Default()`.
The selected system-RIB reader consumes admitted winners, resolves their
forwarding targets and publishes through the daemon's acknowledged output sink.
<!-- source: internal/component/plugin/server/dispatch_route.go -- applyRouteInstall -->
<!-- source: internal/component/sysrib/route_view_worker.go -- consume -->
<!-- source: internal/component/sysrib/route_view_worker_output.go -- publish -->

A protocol ID is per-process, allocated in registration order, so it cannot
travel. The payload names the protocol and the engine looks the name up with
`redistevents.ProtocolIDOf`, then REJECTS an unknown name.

**No protocol-wide distance crosses the boundary.** The declaration seam
(`internal/core/rib/distance`) is process-global and sysrib is its only publisher,
so a forked producer never sees it. It does not need to: the engine's Loc-RIB
resolves every path's distance from the declaration when it ranks
(`resolvedDistance`), and re-ranks on a reload (`(*RIB).Reselect`), so a forked
OSPF ranks at `rib { distance { ospf 5 } }` exactly as an in-process one does.
The entry's optional `distance` field carries only a route's OWN distance, the
static `distance` leaf, which overrides the declared value for that route alone.
<!-- source: internal/core/rib/locrib/distance.go -- resolvedDistance, Reselect -->
<!-- source: internal/component/plugin/server/dispatch_route.go -- applyRouteInstall -->

**The entry carries the whole path.** Besides the gateway, `RouteInstallEntry`
carries the outgoing device, the next-hop's weight, the forwarding action
(`route-type`, the `routetype` number) and the equal-cost set (`ecmp`, one
`RouteNextHop` per member). The primary and each equal-cost member also carry
`on-link`. The engine preserves this direct-adjacency marker when it rebuilds
the path, so a forked source retains the same gateway resolution behavior.
It also carries labels, the Service SID, received AIGP with its presence bit,
and the flags that distinguish BGP and recursively resolved metric paths. The
engine therefore answers metric queries from the same metadata in either
process layout.

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
A persistent install or remove failure closes the SDK connection. A live
connection with a rejected RPC is not sufficient for recovery: disconnect is
what triggers the engine's withdrawal of that producer's routes.

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

The route-install transport is used by forked installers. In-process plugins
hold a non-nil Loc-RIB and use it directly; the registered RPC also supports
Direct dispatch. The static plugin and BGP RIB install a `routeinstall.Sink`
when `locrib.Default()` is nil.

Static validates and retains its initial route set during configuration, then
applies it from `OnAllPluginsReady`, after its BFD setup and all startup
subscriptions. It never flushes route-install during configure: those runtime
operations retain their readiness guard. Later configure deliveries and config
transactions apply immediately through the same route manager.
<!-- source: internal/plugins/static/register.go -- registerStaticCallbacks -->

The BGP RIB also reads the engine's `ze-plugin-engine:route-metrics` RPC. Each
reply contains a routing revision and resolved next-hop distances, including
received AIGP along recursive BGP paths. A missing AIGP is reported separately
from unresolved reachability. A one-second revision poll clears cached distances
and reruns selection without another BGP UPDATE. The feed is mandatory in a
forked RIB: startup refuses an unavailable RPC, and a failed live poll closes
the plugin connection so engine-side disconnect cleanup removes its routes.

<!-- source: internal/component/bgp/plugins/rib/rib_remote.go -- metric feed and selected-route sink -->
<!-- source: internal/component/plugin/server/dispatch_route_metrics.go -- registered metric RPC -->

A kernel route needs an on-link next hop, so the kernel-level test adds a dummy
interface before it asserts that fib-kernel accepted the route.

## Daemon operations for the system RIB

The selected forwarding group includes admitted equal-cost paths across protocol
sources. Each member keeps its own labels, Service SID and registered owner.
That ownership remains distinct when the member names the primary's address and
device, because its labels and FIB permission can differ.
An optional member `protocol` names an owner different from the primary;
omission means the primary's protocol owns the member. FIB withholding filters
that ownership before target deduplication, so a withheld duplicate cannot erase
a permitted source's path. The daemon owns this membership and its change
notifications; both placements consume the same result.
<!-- source: internal/core/rib/locrib/forwarding_members.go -- ForwardingMembers -->
<!-- source: internal/component/sysrib/route_view_path.go -- routeViewCopyEntry -->
<!-- source: internal/component/sysrib/ecmp.go -- siblingProtocol, ecmpGroup -->

The system RIB owns `ze-sysrib:route-view-distances` and
`ze-sysrib:route-view-candidates`. Both use the selected view's authenticated
process generation and session token. Distance application is allowed during
configuration; candidate reads wait for runtime. Shared dispatch discovers the
owner's registrations without holding a system-RIB method list.
<!-- source: internal/component/sysrib/register_route_view_policy.go -- routeViewCandidatesHandler, routeViewDistancesHandler -->

A distance request supplies the complete table resolved from `rib/distance`,
including schema defaults. The daemon validates every name and value before
publishing the immutable declaration and reselecting its Loc-RIB. Connected
distance zero is valid, and per-route overrides retain their existing meaning.
The ordered operation identity makes a retry return its original acknowledgement
without reselecting again. A failed apply restores the previous declaration and
reselects before returning the error, including when that declaration came from
an earlier system-RIB process generation.
<!-- source: internal/component/sysrib/route_view_distances.go -- applyDistances, routeViewDistanceTable, distanceBarrier -->
<!-- test: internal/component/sysrib/route_view_distances_test.go TestRouteViewDistanceApplyRollback -->
<!-- test: internal/component/sysrib/route_view_distances_test.go TestRouteViewDistanceRetryDoesNotReselect -->
<!-- test: internal/component/sysrib/route_view_distances_test.go TestRouteViewDistanceFailureRestoresDeclaration -->

A valid distance change may move more routes than the selected feed can retain.
It still applies: the daemon fences the old output and acknowledges an explicit
reset marker after reselection. The consumer must reopen and complete its
selected snapshot before publishing again. Cancellation detected during
application or a failed output withdrawal remains an apply failure and restores
the previous declaration.
<!-- test: internal/component/sysrib/route_view_distances_test.go TestRouteViewDistanceOverflowAcknowledgesReset -->
<!-- test: internal/component/sysrib/route_view_distances_test.go TestRouteViewDistanceCancellationRestoresDeclaration -->

Candidate reads capture the existing renderer's owned values from the daemon's
Loc-RIB, then encode them after releasing the RIB's shard locks. Each scan has
its own immutable snapshot and cursor, independent of the selected revision, so
a fresh scan sees hidden-only edits. Pages preserve whole prefixes and obey
both the 4096-prefix limit and the complete reply-frame budget. A prefix may
have more than 4096 paths when its complete entry fits that budget. One scan
remains pinned per session; completion, close, disconnect or the two-minute
lifetime releases its retained rows. The existing session worker performs idle
expiry, and page admission also checks the deadline.
<!-- source: internal/component/sysrib/route_view_candidates.go -- readCandidates, scanCandidates, page, expireCandidates, releaseCandidates -->
<!-- test: internal/component/sysrib/route_view_candidates_test.go TestRouteViewHiddenCandidatesStayFresh -->
<!-- test: internal/component/sysrib/route_view_candidates_test.go TestRouteViewCandidatePrefix4097Paths -->
<!-- test: internal/component/sysrib/route_view_candidates_test.go TestRouteViewCandidateCompleteFrameBudget -->
<!-- test: internal/component/sysrib/route_view_candidates_test.go TestRouteViewCandidateExpiration -->

These daemon-side handler tests establish the adapter behavior. Startup,
command and kernel parity for an external system RIB remain separate
integration requirements.

The consumer constructor opens its authenticated view and waits for the complete
distance declaration to be acknowledged by the daemon. Internal placement
requests the local-authority reader and requires a readable daemon Loc-RIB;
external placement uses the selected feed. The constructor retains owned
configuration values for display and forwarding permissions, and never writes
an external process's disconnected distance seam. A configure refusal leaves
lease cleanup to the stopped generation, because Close is a runtime operation.
The runtime worker starts without waiting for runtime admission, because the
SDK enters its callback loop only after the started callback returns. Its owner
cancels and joins the worker before closing the lease and SDK connection.
<!-- source: internal/component/sysrib/route_view_client.go -- openRouteViewClient, routeViewClientDistances -->
<!-- source: internal/component/sysrib/route_view_worker.go -- Start, run -->

Runtime configuration serializes with selected reads and commands. It has two
minutes to apply and acknowledge the complete declaration, selected frontier
and final forwarding permissions. A failed attempt receives an independent
two-minute restoration interval tied to worker shutdown. Restoration closes the
uncertain lease and reconstructs the previous configuration in a fresh session;
failed restoration reports that the running configuration was lost and closes
the SDK. Caller cancellation does not abandon the restoration verdict.
<!-- source: internal/component/sysrib/route_view_worker_operations.go -- ApplyConfiguration, perform, configure, reopen -->

Selected Reset also reconstructs a fresh session and complete current snapshot.
An in-progress configuration supplies its desired declaration to that new
session. Pre-reset forwarding caches and pending output are discarded because
the daemon already withdrew the fenced lease's ledger. Normal stopped-transport
cleanup belongs to the daemon's process cleanup; a live output-withdrawal
refusal remains an error, and the consumer retains fatal worker failures for
the plugin owner's exit status. Concurrent shutdown suppresses only cancellation
or a stopped transport; it preserves a live publication refusal.
<!-- source: internal/component/sysrib/route_view_worker_operations.go -- reopen, reopenOnce -->
<!-- source: internal/component/sysrib/route_view_client.go -- Close, Failure -->

If a valid selected reply crosses a lease fence before its output is published,
the external worker can recover without a process restart. It first requires
an explicit read Reset on that exact lease, with `rejected-publication` proving
the same fully validated operation was refused before emission. It then uses
the ordinary close and reconstruction path. Delivery errors and malformed,
foreign or replacement refusals remain fatal. A broadcast replay interrupted
by this proven reset stays queued until current authority answers it.
<!-- source: internal/component/sysrib/route_view_worker_output.go -- publicationReset -->
<!-- source: internal/component/sysrib/route_view_output.go -- provePublicationReset -->
<!-- source: internal/component/sysrib/route_view_worker.go -- consume, replay -->
