# 819 -- Flow Export: Wiring the Spec-2 Integration Layer

## Context

The spec-2 packages (sampling, conntrack, enrich, and the per-protocol flow
encoders sflow/flow.go, netflow9/flow_*.go, ipfix/flow_*.go) existed as
primitives but had NO production caller: no lifecycle, no goroutine workers, no
YANG config, no flow-encoder dispatch, and no BGP RIB subscription. A survey
confirmed every exported spec-2 symbol was reachable only from tests. This work
built the integration layer that connects them to the `flowexport.Exporter`.

## Decisions

- **Flow encoding goes through interfaces defined in `flowexport`, registered by
  factory** -- the same pattern as the counter `ProtocolEncoder`. Forced by the
  import graph: the protocol packages (sflow/netflow9/ipfix) import `flowexport`
  for `Sender`/`CounterSnapshot`, so `flowexport` CANNOT import them. New
  interfaces `FlowSampleEncoder` and `FlowRecordEncoder` live in `flowtypes.go`;
  adapters in each protocol package implement them and register via
  `RegisterFlowSampleEncoderFactory` / `RegisterFlowRecordEncoderFactory`.
- **Neutral value types `FlowSample` and `ConntrackFlow`** in `flowexport` carry
  data across the boundary (owned header copy, absolute Unix-ms timestamps). Each
  adapter converts to its internal record type and time base (NetFlow v9 wants
  sysUpTime-relative ms; IPFIX wants absolute ms). This keeps `flowexport` free
  of sampling/conntrack/protocol internals on the dispatch path.
- **Workers are platform-independent; OS specifics stay in the sampling/conntrack
  packages.** `samplingWorker` and `conntrackWorker` live in `flowexport` and
  call `sampling.SetupSampling` / `sampling.NewPsampleReader` / `conntrack.NewReader`
  uniformly; the `_other.go` stubs return errors and the worker degrades to a
  logged no-op. This means the whole `flowexport` package compiles and unit-tests
  on darwin while the netlink paths are exercised on Linux/CI.
- **BGP enrichment uses the typed `ribevents.BestChange` handle** (per
  plugins.md: no raw bus.Subscribe). The in-process EventBus is captured via
  `Registration.ConfigureEventBus`. A `bgpEnrichBuilder` folds best-change batches
  into a `map[Prefix]ASEntry` and a 1s ticker rebuilds an immutable radix tree,
  swapped atomically into the `enrich.Enricher` so per-flow readers never see a
  partial tree.
- **Worker teardown runs OUTSIDE the exporter mutex.** Worker `Stop()` waits on
  its goroutine, and that goroutine calls `ExportFlow*` which take `e.mu`.
  `Exporter.Stop()` flips `stopped`, snapshots+clears the stopper list, releases
  `e.mu`, then runs stoppers -- otherwise a reload deadlocks.

## Consequences

- A config reload swaps the exporter; `Exporter.Stop()` runs all registered
  stoppers (sampling worker, conntrack worker, enrich builder), so tc sample
  actions are removed and goroutines exit cleanly.
- New config surface: `flow-export { sampling { interface <n> { rate; trunc-size;
  group; } } conntrack { enabled; active-timeout; } enrichment { bgp; } }` with
  boundary validation (rate 1..1000000, trunc-size 64..1500, group 1..2147483647,
  active-timeout 1..3600).
- Metrics added: `ze_flowexport_samples_total{interface}`,
  `ze_flowexport_flows_total{collector}`, `ze_flowexport_flows_active` (gauge).

## Status update (resolved after initial deferral)

- **AS enrichment: RESOLVED.** `ribevents.BestChangeEntry` gained `OriginAS`/`ASPath`,
  populated in `rib_bestchange.go` from the winning candidate's interned AS_PATH
  handle (`pool.ASPath.Get` + `formatASPath`). `enrichbgp.applyBatch` stores them;
  `ExportFlows` now fills SrcAS/DstAS (IPFIX IE 16/17). Full-table replay still omits
  AS data (packed record drops the handle); corrected by the next incremental change.
- **IPv6 per-flow records: RESOLVED.** Separate IPv4/IPv6 templates (IDs 257/258) in
  netflow9 and ipfix; the conntrack worker no longer skips IPv6.
- **Extended sFlow if_counters (partial):** ifType + promiscuous now populated;
  speed/duplex/multicast/broadcast left zero (kernel does not expose per-direction
  multicast or broadcast; speed/duplex need a sysfs read not yet on the snapshot path).

## Remaining known limitations
- **Conntrack export is periodic-dump only.** No immediate destroy-event export;
  vishvananda/netlink lacks the NFNLGRP_CONNTRACK_DESTROY group binding (C4).
- **sFlow flow_sample header is capped** to `MaxDatagramSize - overhead`; trunc-size
  may be configured up to 1500 but the 1400-byte datagram bounds the captured bytes
  (sFlow permits captured < frame_length).

## Files

- `internal/plugins/flowexport/flowtypes.go` -- FlowSample, ConntrackFlow, FlowSampleEncoder, FlowRecordEncoder
- `internal/plugins/flowexport/encoder_registry.go` -- flow factory registration + lookup
- `internal/plugins/flowexport/enrichbgp.go` (+ `_test.go`) -- BestChange subscription -> radix tree rebuild
- `internal/plugins/flowexport/sampling_worker.go` -- tc sample setup + psample read loop -> ExportFlowSample
- `internal/plugins/flowexport/conntrack_worker.go` -- periodic dump + delta -> ExportFlows
- `internal/plugins/flowexport/exporter.go` -- collectorState flow encoders, enricher, ExportFlowSample/ExportFlows, AddStopper, Stop ordering
- `internal/plugins/flowexport/config.go` -- Sampling/Conntrack/Enrichment parse + validate
- `internal/plugins/flowexport/register.go` -- ConfigureEventBus, startFlowSubsystems, wireEncoders flow encoders
- `internal/plugins/flowexport/{sflow,netflow9,ipfix}/flow_adapter.go` + `register.go` -- flow encoder adapters + factory registration
- `internal/plugins/flowexport/yang/ze-flowexport-conf.yang` -- sampling/conntrack/enrichment containers
