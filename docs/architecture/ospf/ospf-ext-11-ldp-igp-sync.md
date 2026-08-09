# LDP and IGP synchronisation

Hold an OSPF link out of the shortest path until its LDP label switched path is
up, so traffic does not use a link whose label bindings are not ready. RFC 5443
costs the link out. RFC 6138 withholds a broadcast transit link entirely unless
it is a cut edge.

## Decisions

- **OSPF subscribes to LDP session events by literal STRING topic and decodes
  the JSON payload. It never imports the LDP package.** With LDP removed, an
  ldp-sync interface stays unsynchronised and nothing breaks.
  <!-- source: internal/plugins/ospf/ldp_sync.go -- ldpSyncNamespace, ldpSyncStateName, ldpSyncTimer -->
- **A per-interface state machine with a configurable hold-down.** On hold-down
  expiry the cost is restored even when LDP never synced, which avoids a
  permanent blackhole. Epoch invalidation of stale timer callbacks prevents
  flap.
- **Point-to-point uses max-metric on the transit link only. Broadcast withholds
  the transit link unless a fresh cut-edge query says that removing it partitions
  the graph.** The query flushes any pending computation first and reuses the
  last computed per-area graph.
  <!-- source: internal/plugins/ospf/spf/cutedge.go -- IsCutEdge, flushPendingSPF -->
- **Restore recomputes the configured cost at origination time.** The stored
  cost is never overwritten.

## Constraints on callers

- The subscriber is wired for EVERY engine: the base v4 engine, each RFC 6549
  instance, and each per-address-family v6 engine. A single-engine wiring misses
  most of them.
- Running a full shortest-path computation synchronously inside Router-LSA
  origination for each unsynced broadcast interface is heavy and is what RFC
  6138 Appendix A requires. It is safe because the topology snapshot releases
  the LSDB lock before the origination callback, so the cut-edge computation
  never re-enters a held lock.

## Traps

- **A field written on a shared struct AFTER the owning table released its lock
  is a torn read**, live in any deployment. Set the field INSIDE the table
  update, under the table lock.
- **Point-to-point cost-out must NOT override the interface cost**, because that
  propagates to the connected-subnet stub link as well. A per-interface
  max-metric FLAG is applied to the transit link alone, and the stub keeps the
  configured cost.
- OSPFv3 broadcast withhold is a no-op, because the pseudonode id keys on IPv4
  fields. The v6 point-to-point cost-out does work, and v6 broadcast withhold is
  fail-safe: it always advertises.
