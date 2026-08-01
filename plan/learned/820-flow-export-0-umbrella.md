# 820 -- Flow Export Umbrella: Architecture Recap

## Context

The `flow-export` feature spans three specs: an umbrella (0), counter export (1),
and packet sampling + flow records (2). This summary ties together the two
implementation summaries and records the cross-cutting architecture decisions.
See [[818-flow-export-1-counter-export]] and [[819-flow-export-2-flow-records]].

## Cross-Cutting Decisions

- **Single collection, multiple consumers.** Flow export does not poll the kernel.
  It registers a callback with the existing iface rate tracker
  (`iface.RegisterCollectNotify`) and receives the same 1s snapshot Prometheus
  already consumes, using RAW (pre-baseline) counters. No second poll loop.
- **Registration over imports.** The core never imports flowexport; flowexport
  registers as a component, and the three protocol encoders register with
  flowexport via factories. The same pattern extends to the spec-2 flow encoders.
  This is what lets `flowexport` stay free of protocol-package imports despite the
  protocol packages importing `flowexport` (the import-graph constraint that
  dictated the `FlowSampleEncoder`/`FlowRecordEncoder` interface design).
- **Buffer-first encoding throughout.** All datagram assembly uses
  `WriteTo(buf, off) int` with skip-and-backfill for length/count fields, drawing
  buffers from a shared `sync.Pool` of 1400-byte slices.
- **In-process component using the SDK over an in-memory conn.** flowexport runs
  in the daemon process (so the iface callback and EventBus access are direct
  calls), while still speaking the SDK config protocol via `sdk.NewWithConn`.

## State at Closure

- Spec 1 (counter export) is complete and verified: unit tests pass, lint clean,
  darwin daemon builds, cross-vets on Linux.
- Spec 2 (sampling/conntrack/enrich + flow records) is code-complete and CI-gated:
  the netlink worker paths compile on Linux (cross-vet) but require a privileged
  Linux runner to exercise end-to-end; functional `.ci` coverage runs in CI.
- Documented deferrals: BGP AS-path enrichment (event lacks AS data), IPv6 flow
  records, conntrack destroy-event export. All recorded in spec 2 and [[819-flow-export-2-flow-records]].

## Files

See [[818-flow-export-1-counter-export]] and [[819-flow-export-2-flow-records]]
for the per-spec file lists. The component root is
`internal/plugins/flowexport-cmd/` with protocol subpackages sflow, netflow9, ipfix
and spec-2 subpackages sampling, conntrack, enrich.
