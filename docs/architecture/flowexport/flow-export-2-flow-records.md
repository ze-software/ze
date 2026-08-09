# Flow Export: Packet Sampling, Conntrack and Flow Records

Per-flow export: `tc` packet sampling read over psample, conntrack dumps, BGP
enrichment, and the per-protocol flow encoders. Cross-cutting decisions are in
[the umbrella page](flow-export-0-umbrella.md).

## Flow encoders are interfaces in `flowexport`, registered by factory

<!-- source: internal/plugins/flowexport/flowtypes.go -- FlowSample, ConntrackFlow, FlowSampleEncoder, FlowRecordEncoder -->
<!-- source: internal/plugins/flowexport/encoder_registry.go -- RegisterFlowSampleEncoderFactory, RegisterFlowRecordEncoderFactory -->

The import graph forces this shape. `sflow`, `netflow9` and `ipfix` import
`flowexport`, so `flowexport` cannot import them. `FlowSampleEncoder` and
`FlowRecordEncoder` are declared in `flowtypes.go`; each protocol package holds an
adapter that implements them and registers a factory.

`FlowSample` and `ConntrackFlow` are neutral value types. They carry an owned
header copy and absolute Unix-millisecond timestamps. Each adapter converts to its
own record type and time base: NetFlow v9 wants milliseconds relative to
sysUpTime, IPFIX wants absolute milliseconds. The dispatch path in `flowexport`
therefore holds no sampling, conntrack or protocol internals.

## Workers are platform-independent

<!-- source: internal/plugins/flowexport/sampling_worker.go -- sampling worker loop -->
<!-- source: internal/plugins/flowexport/conntrack_worker.go -- conntrack dump loop -->
<!-- source: internal/plugins/flowexport/sampling/psample_other.go -- non-Linux stubs -->

`samplingWorker` and `conntrackWorker` live in `flowexport` and call
`sampling.SetupSampling`, `sampling.NewPsampleReader` and `conntrack.NewReader`
without a build tag. The OS split lives in the `sampling` and `conntrack`
packages. The `_other.go` stubs return an error and the worker degrades to a
logged no-op.

The result is that the whole `flowexport` package compiles and unit-tests on
darwin, while the netlink paths run on Linux.

## BGP enrichment reads the typed best-change event

<!-- source: internal/plugins/flowexport/enrichbgp.go -- bgpEnrichBuilder, applyBatch -->
<!-- source: internal/plugins/flowexport/enrich/radix.go -- immutable radix tree -->

Enrichment subscribes to `ribevents.BestChange` through the typed handle, not
through a raw `bus.Subscribe` (`ai/rules/plugins.md`). The in-process EventBus
arrives through `Registration.ConfigureEventBus`.

`bgpEnrichBuilder` folds best-change batches into a `map[Prefix]ASEntry`. A
one-second ticker rebuilds an immutable radix tree and swaps it atomically into
the `enrich.Enricher`. Per-flow readers therefore never observe a partially built
tree. `BestChangeEntry` carries `OriginAS` and `ASPath`, so `ExportFlows` fills
IPFIX IE 16 and IE 17 (`bgpSourceAsNumber`, `bgpDestinationAsNumber`).

## Worker teardown runs outside the exporter mutex

<!-- source: internal/plugins/flowexport/exporter.go -- exporter.stop, exporter.addStopper -->

A worker `Stop()` waits for its goroutine, and that goroutine calls
`exportFlowSample` or `exportFlows`, which take `e.mu`. `exporter.stop()`
therefore sets `stopped`, closes `stopCh`, copies and clears the stopper list,
releases `e.mu`, and only then runs the stoppers. Holding the mutex across
teardown deadlocks a config reload.

## Known limits

<!-- source: internal/plugins/flowexport/conntrack/destroy.go -- destroy-event export -->
<!-- source: internal/plugins/flowexport/sflow/flow_adapter.go -- header cap against MaxDatagramSize -->

- Conntrack export is a periodic dump. `vishvananda/netlink` has no
  `NFNLGRP_CONNTRACK_DESTROY` group binding, so a closed flow is reported at the
  next dump and not at the moment it closes.
- The sFlow `flow_sample` header is capped at `MaxDatagramSize` minus the sample
  overhead. `trunc-size` accepts up to 1500, and the 1400-byte datagram still
  bounds the captured bytes. sFlow permits a captured length below
  `frame_length`, so a capped header is valid.
- sFlow extended `if_counters` reports `ifType` and `ifPromiscuousMode`. Speed,
  duplex, multicast and broadcast stay zero: the kernel exposes no per-direction
  multicast or broadcast count, and speed and duplex need a sysfs read that the
  snapshot path does not make.
