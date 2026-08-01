# 745 -- IPsec CLI and Diagnostics

## Context

The IPsec subsystem (ipsec-7/8/9) built an IKE engine with SA table, child SAs, DPD, rekey, EAP, and NAT-T, but had no CLI, web, health, or metrics surface. Operators had no way to inspect tunnel state, clear SAs, or monitor lifecycle events. This spec adds the presentation and observability layer without touching kernel/network code.

## Decisions

- Exposed SA table via atomic pointer, active peers via RWMutex-protected map with snapshot copies for all readers, over atomic pointer to map (initial approach) which had data races between RPC goroutines and the engine's OnConfigure callback.
- PeerSession.Stop() uses sync.Once over bare close(stopCh), because reconcilePeers and TerminateAllSAs can race on the same session; double-close panics.
- Added PeerInfo snapshot struct with exported Info() method on PeerSession, over exporting PeerSession fields, because the fields include crypto key material (childSA.Keys) that must not leak to the show layer.
- Placed metrics and health check in the engine package rather than in telemetry/collector, because they query engine-internal state and the host.RegisterMetrics pattern does the same.
- Used RegisterStreamingHandler for monitor vpn ipsec (event bus subscription) over polling, because SA lifecycle events are already emitted to the bus.
- Added String() methods on crypto.EncryptionID, IntegrityID, DHGroupID over hardcoded switch statements in each consumer (show handler, web page). Removed duplicate integrityIDToName from cipher.go.
- TerminateAllSAs and TerminatePeerSA trigger re-establishment by calling a stored reconcile function, over requiring manual config reload. This gives "clear vpn ipsec sa" bounce semantics.
- Metrics update on a 5-second ticker goroutine over only-on-config-change, because SA state changes (establish, DPD death) happen between config reloads.
- Byte counters (ipsec_bytes_in_total, ipsec_bytes_out_total) not implemented because they require XFRM SA stat queries (linux kernel interaction), which this spec excludes.

## Consequences

- The show/ipsec.go import of engine triggers the IKE plugin registration in all binaries that import cmd/show. TestAvailablePlugins and TestAllPluginsRegistered updated.
- reconcilePeers now holds peersMu around map mutations. Any future code that accesses activePeersMap must use the mutex.
- The reEstablishFn atomic pointer creates a closure-capture coupling between runEngine locals and the package-level terminate functions. The pointer is cleared on shutdown to avoid stale references.
- rekey_total is a gauge (current cumulative count from engine memory) not a Prometheus counter, because the value resets on engine restart. A true counter would require persistent storage.

## Gotchas

- PeerSession fields are all unexported. Show handlers cannot access childSA or peerCfg directly; they use PeerInfoMap() which returns value-type snapshots under the mutex.
- The YANG test TestEveryRPCHasYANGPath catches RPCs registered without YANG entries. The ze-monitor:vpn-ipsec RPC needed an entry in ze-monitor-cmd.yang (in bgp/plugins/cmd/monitor/schema, not in cmd/show/schema).
- ESPGroup.Lifetime is uint32, not int. Type mismatch in PeerInfo caught by compiler.
- The web page uses HTMX table rendering (same as L2TP, BGP pages), not SSE streaming. The project's SSE broker is for config-change notifications only; live table data uses workbench_table fragment rendering with sub-path HTMX partials.

## Files

- `internal/component/ike/engine/register.go` -- ActiveTable, ActivePeers (mutex), PeerInfoMap, SetActiveTable, SetActivePeers, TerminateAllSAs, TerminatePeerSA, reEstablish, metrics ticker
- `internal/component/ike/engine/reconcile.go` -- PeerSession with sync.Once Stop, PeerInfo snapshot, rekey counter, peersMu around map ops
- `internal/component/ike/engine/established.go` -- rekeyCount increment
- `internal/component/ike/engine/health.go` -- health check
- `internal/component/ike/engine/health_test.go`
- `internal/component/ike/engine/metrics.go` -- sa_count, tunnel_up, rekey_total
- `internal/component/ike/engine/metrics_test.go`
- `internal/component/ike/crypto/transform.go` -- String() on EncryptionID, IntegrityID, DHGroupID
- `internal/component/ike/crypto/cipher.go` -- removed integrityIDToName (uses String())
- `internal/component/ike/cmd/show_ipsec.go` -- show vpn ipsec sa/status/peer
- `internal/component/ike/cmd/show_ipsec_test.go` -- positive and negative path tests
- `internal/component/ike/cmd/monitor_ipsec.go` -- monitor vpn ipsec streaming
- `internal/component/ike/cmd/ipsec.go` -- clear vpn ipsec sa
- `internal/component/ike/cmd/ipsec_test.go`
- `internal/component/web/page_vpn_ipsec.go` -- /show/vpn/ipsec/ table page
- `internal/component/web/workbench_pages.go` -- vpn dispatch
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` -- show vpn ipsec entries
- `internal/component/cmd/clear/yang/ze-cli-clear-cmd.yang` -- clear vpn ipsec entry
- `internal/component/bgp/plugins/cmd/monitor/yang/ze-monitor-cmd.yang` -- monitor vpn ipsec entry
- `cmd/ze/main_test.go` -- added "ike" to expected plugins
- `internal/component/plugin/all/all_test.go` -- added "ike" to expected plugins
- `docs/features.md` -- IPsec CLI and Diagnostics row
- `test/ipsec/ipsec-show-sa.ci`, `ipsec-show-status.ci`, `ipsec-show-peer.ci`, `ipsec-clear-sa.ci`, `ipsec-monitor.ci` -- functional tests
