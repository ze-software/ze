# IPsec CLI, web and observability

The presentation layer over the IKE engine: show and clear commands, a
streaming monitor, a health check, Prometheus metrics, and a web page. It reads
engine state and touches no kernel or network code.

<!-- source: internal/component/ike/cmd/show_ipsec.go -- show vpn ipsec sa, status, peer -->
<!-- source: internal/component/ike/cmd/ipsec.go -- clear vpn ipsec sa -->
<!-- source: internal/component/ike/cmd/monitor_ipsec.go -- monitor vpn ipsec streaming handler -->
<!-- source: internal/component/ike/engine/health.go -- IPsec health check -->
<!-- source: internal/component/ike/engine/metrics.go -- IPsecMetrics, ze_ipsec_sa_count, ze_ipsec_tunnel_up, ze_ipsec_rekey_total -->
<!-- source: internal/component/web/page_vpn_ipsec.go -- the VPN IPsec table page -->

## Decisions

**The SA table is an atomic pointer; the active peer map is behind a mutex and
readers get snapshot copies.** The first attempt held the peer map behind an
atomic pointer too, which raced the engine's configure callback against the RPC
goroutines.

<!-- source: internal/component/ike/engine/register.go -- ActiveTable, ActivePeers, PeerInfoMap, setActivePeers -->

**`PeerSession.Stop` uses `sync.Once`.** Reconciliation and `TerminateAllSAs`
can race on the same session, and a double close of the stop channel panics.

**`PeerInfo` is a snapshot struct, not exported fields.** The peer session holds
crypto key material on the child SA. Exporting the fields would leak it into the
show layer. `Info()` copies under the mutex; `PeerInfoMap()` returns value
copies.

<!-- source: internal/component/ike/engine/reconcile.go -- PeerInfo, Info, Stop, StopGraceful -->

**Metrics and the health check live in the engine package.** They query engine
internal state, and the host metric registration pattern already does the same.

**Monitoring uses the event bus, not polling.** SA lifecycle events are already
emitted, so the monitor registers a streaming handler and subscribes.

**`String()` lives on the crypto ID types.** The show handler and the web page
would otherwise each carry a switch. The duplicate integrity-name helper in the
cipher file was deleted when `String()` landed.

<!-- source: internal/component/ike/crypto/transform.go -- EncryptionID.String, IntegrityID.String, DHGroupID.String -->

**`clear vpn ipsec sa` re-establishes.** The terminate functions call a stored
reconcile closure, so clearing bounces the tunnel instead of requiring a config
reload.

**Metrics update on a 5-second ticker.** SA state changes such as establishment
and detection-driven teardown happen between config reloads, so update-on-change
would miss them.

**`rekey_total` is a gauge, not a counter.** The value is the cumulative count
held in engine memory and it resets when the engine restarts. A true counter
needs persistence.

**Byte counters come from the kernel, not from this layer.** The engine never
sees ESP payload, so a count kept here would report zero forever. `show vpn
ipsec sa` reads them from the kernel SAD and renders null, never zero, when the
SAD cannot be read. See
[`ipsec-dataplane-inspection.md`](ipsec-dataplane-inspection.md).

<!-- source: internal/component/ike/cmd/show_ipsec.go -- sadCounters, readSADCounters -->

## Traps this code exists to avoid

**Importing the engine from a show handler registers the plugin everywhere.**
The show handler's import pulls the IKE plugin into every binary that imports
the show command package. The plugin inventory tests have to know that.

**Every writer of the active peer map must hold the mutex.** `reconcilePeers`
holds it around the map mutations. Any new accessor inherits that.

**The reconcile closure is a lifetime coupling.** The stored function captures
locals of the engine run loop, and the package-level terminate functions call
it. The pointer is cleared at shutdown so no stale reference survives.

**An RPC needs a YANG entry or the schema test fires.** The monitor RPC's entry
lives in the monitor command schema, not in the show command schema.

**The web page renders an HTMX table, it does not stream.** The server-sent
event broker in this project carries config-change notifications only. Live
table data uses the workbench table fragment with sub-path HTMX partials, the
same as the L2TP and BGP pages.
