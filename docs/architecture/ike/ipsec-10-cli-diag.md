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
goroutines. `setActivePeers` advances the dataplane generation under that same
mutex. A kernel read that spans a republished map then answers unknown rather
than drift.

<!-- source: internal/component/ike/engine/register.go -- ActiveTable, ActivePeers, PeerInfoMap, setActivePeers -->

**`PeerSession.Stop` uses `sync.Once`.** Reconciliation and `TerminateAllSAs`
can race on the same session, and a double close of the stop channel panics.

**`PeerInfo` is a snapshot struct, not exported fields.** The peer session holds
crypto key material on the child SA. Exporting the fields would leak it into the
show layer. `Info()` copies under the mutex; `PeerInfoMap()` returns value
copies. The snapshot names each child SA half as a destination-qualified
`SAIdentity`, not as a bare SPI, so a kernel lookup matches one SA. It also
carries whether removal of that child SA has started, read under the observation
mutex.

**`PeerInfo` describes the Child SA as INSTALLED, never as configured.** Its
`ESPEncryption` and `ESPIntegrity` are read from `ChildSA.ESPGroup`, which holds
the ONE proposal the peer accepted: `selectResponderESP` narrows it on the
responder, the IKE_AUTH response path narrows it on the initiator, and each rekey
narrows the replacement's copy. Until 2026-09-16 `Info` read the session's
configured group instead, so a peer that accepted the second of two proposals was
reported as running the first. The typed `ESPEncryptionID`, `ESPKeyBits` and
`ESPIntegrityID` beside the names are the RFC 7296 Section 3.3.2 transform ids,
for a reader that derives from the algorithm. An AEAD proposal reports its
integrity as `none`, the transform's AUTH_NONE, rather than the config enum's zero.
The snapshot also carries `ChildRemoteAddr` and `ChildLocalAddr` (the installed
endpoints, as `netip.Addr`), `ChildMode` and `ChildUDPEncap`: with the transform,
those are the facts that size an ESP packet.

<!-- source: internal/component/ike/engine/reconcile.go -- PeerInfo, Info, Stop, StopGraceful -->

**`show vpn ipsec sa` reports WHICH side of a NAT each end is on.** The payload
carries `nat-detected` beside `behind-nat` and `peer-behind-nat`, because RFC
7296 Section 2.23.1's transport-mode selector substitution is written per side
and an operator diagnosing a transport tunnel needs the fact `nat-detected`
hides. All three come straight off the SA and are booleans in the JSON.

**It also reports the selector addresses BEFORE that substitution.**
`original-tsi` and `original-tsr` are the pair the peer put on the wire, and
`child-sa.ts-local` and `child-sa.ts-remote` are the pair the kernel programs.
Behind a NAT the two differ, and the section requires the originals be kept, so
the payload is the operator's only reader of them. Both are text, and both are
null on an SA that read no transport-mode selector set, which is every
tunnel-mode SA. Null says the SA never held the fact; an empty string would read
as an address nobody holds.

**`child-sa` carries the installed transform and the three facts that size an
ESP packet.** `esp-encryption` and `esp-integrity` name the proposal the peer
accepted, not the first one configured. `mode` is `tunnel` or `transport` as
installed, `udp-encapsulation` is a boolean saying the SA receives ESP inside UDP
(RFC 3948), and `remote-address` is the endpoint the SA was installed on, which
behind a NAT differs from the peer's configured address. `remote-address` is null
only when the Child SA carries no parseable endpoint, which no install path
produces. The `mtu-negotiated-transform` interop scenario proves the transform
against strongSwan: ze offers two proposals, charon accepts the second, and the
payload names the second.

<!-- source: internal/component/ike/cmd/show_ipsec.go -- saToMap, selectorAddressText, installedAddressText -->

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
sees ESP payload, so a count kept here would report zero forever.
`readSADCounters` takes one `ObserveDataplane` observation, which carries engine
belief and the kernel SAD together, and indexes that SAD by SA identity. A
counter renders null, never zero, when the observation failed and when the
kernel holds no SA under that identity. `counters-known` is false only in the
first case. See
[`ipsec-dataplane-inspection.md`](ipsec-dataplane-inspection.md).

<!-- source: internal/component/ike/cmd/show_ipsec.go -- sadCounters, readSADCounters -->

**Two gauges report the kernel, and they publish nothing when it cannot be
read.** `ze_ipsec_dataplane_sa_count{if_id}` and `ze_ipsec_dataplane_drift{peer}`
sit beside the belief gauges above. `publishDataplaneGauges` takes one
`ObserveDataplane` observation for the pass and feeds both from it, and `Update`
calls it only when `ActiveTable()` is non-nil. An unreadable SAD, and an
observation the engine changed across, each delete every series and set none.
See
[`ipsec-dataplane-inspection.md`](ipsec-dataplane-inspection.md).

<!-- source: internal/component/ike/engine/metrics.go -- publishDataplaneGauges, setGaugeSeries -->

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
