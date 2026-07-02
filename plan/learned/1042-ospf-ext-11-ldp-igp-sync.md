# 1042 - OSPF LDP-IGP Synchronisation (RFC 5443/6138)

## Context

Hold an OSPF link out of the shortest path until its LDP LSP is up, so traffic
does not use a link whose label bindings are not ready. RFC 5443: cost-out the
link (max-metric) until LDP syncs or a hold-down timer expires. RFC 6138: on a
broadcast link, WITHHOLD the transit link entirely unless it is a cut-edge
(removing it would partition the graph). Both address families, self-contained
(OSPF does not import the ldp package). Built base-only in a worktree.

## Decisions

- OSPF subscribes to LDP session-up/down by literal STRING topic + JSON-decodes the payload (never imports `ldp`); works with LDP removed (an ldp-sync interface just stays unsynchronised).
- Per-interface state machine with a configurable hold-down; on hold-down expiry the cost is restored even if LDP never synced (avoid a permanent black hole). Epoch invalidation of stale timer callbacks prevents flap.
- P2P: max-metric (LSInfinity) the p2p/transit link only. Broadcast (RFC 6138): withhold the transit (Type-2) link unless a fresh-SPF cut-edge query says removing it partitions the graph. The cut-edge query flushes any pending SPF first and reuses the last computed per-area graph.
- Restore uses the configured cost recomputed at origination time; the stored cost is never overwritten.

## Consequences

- LDP's `SessionEvent` gained an `Interface` field; it MUST be set inside `AdjacencyTable.Update` under the table lock (see gotcha).
- Running a full SPF synchronously inside Router-LSA origination for each not-synced broadcast interface (RFC 6138 App A cut-edge) is heavy but mandated; safe because `topologySnapshot` releases the LSDB lock before the origination callback, so the cut-edge SPF never re-enters a held lock.
- OSPFv3 broadcast withhold is a no-op (the pseudonode id keys on IPv4 fields); v6 P2P cost-out does work. v6 broadcast withhold is fail-safe (always advertise) and out of scope.

## Gotchas

- DATA RACE: writing `adj.Interface = ifName` AFTER `AdjacencyTable.Update()` released its lock, while `All()` copies the struct under RLock, is a torn read live in ANY LDP deployment. Set `Interface` INSIDE `Update` under `t.mu` (pass ifName in); do not mutate the shared `*Adjacency` outside the lock.
- P2P cost-out must NOT override the interface `info.Cost` (that propagates to the connected-subnet STUB link too); use a per-interface max-metric FLAG applied only to the p2p/transit link in `routerLinks` (mirroring the RFC 6987 max-metric path), keeping the stub at the configured cost.
- Wiring the LDP-sync subscriber must cover every engine post-af-unify: the v4 base + RFC 6549 instances (multi_instance.go) AND each v6 AF engine (register_multiaf.go v6EngineSet), not a single eng6.

## Files

- `internal/plugins/ospf/ldp_sync.go` (+ test), `spf/cutedge.go` (+ test), `lsdb/ldp_sync_origination_test.go`
- `internal/plugins/ldp/{events,discovery,register}.go` (SessionEvent.Interface, Update-under-lock)
- `internal/plugins/ospf/{config,instance,register,cmd_show}.go`, `register_multiaf.go`, `multi_instance.go`, `lsdb/{flooding,origination}.go`, `origination_v6.go`, `spf/computer.go` (lastGraphs), `yang/ze-ospf-{conf,cmd}.yang`
- `test/ospf/ospf-ldp-sync-*.ci`, `test/interop/scenarios/ospf-ldp-sync-frr/`
