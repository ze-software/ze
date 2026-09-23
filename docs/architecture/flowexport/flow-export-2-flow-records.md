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

The Linux sampler installs `MatchAll` / `ETH_P_ALL` at ingress with a per-link
sample action. `TestSampleFilterKernelLifecycle` installs the production filter
in a private network namespace and reads it back from Linux. It checks the
configured rate, group and truncation, absence of an egress filter, and that
replacing or removing one sampler leaves the other unchanged. The test requires
`integration` and a kernel with `cls_matchall` and `act_sample`; it is not evidence
of statistical convergence or independence of the kernel's random draws.
<!-- source: internal/plugins/flowexport/sampling/tc_linux.go -- buildSampleFilter -->
<!-- source: internal/plugins/flowexport/sampling/tc_sflow_v5_linux_test.go -- TestSampleFilterKernelLifecycle -->

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

## Export limits and counter sources

Conntrack export combines periodic dumps with destroy events. The destroy
listener exports a flow's residual counters when the kernel reports teardown.
If the listener cannot start, the worker logs the error and uses periodic dumps.
<!-- source: internal/plugins/flowexport/conntrack_worker.go -- conntrackWorker.Start, conntrackWorker.runDestroy -->

The collector's `max-datagram-size` bounds every datagram. NetFlow v9 and IPFIX
split flow batches by address family and then by record capacity, including
padding. A failed UDP send returns an error. IPFIX sequences advance only for
records sent, as required by RFC 7011 Section 10.3.2.
Both protocols report records sent before a later datagram fails.
<!-- source: internal/plugins/flowexport/ipfix/flow_adapter.go -- FlowEncoder.EncodeFlows, FlowEncoder.sendDataMessage -->
<!-- source: internal/plugins/flowexport/netflow9/flow_adapter.go -- FlowEncoder.sendDataPacket -->

IPFIX retains the Export Time of each successfully sent flow Template message.
Subsequent flow Template and Data messages use that value as a minimum, so a
backward wall-clock adjustment cannot place data before its template.
This implements the Export Time ordering in RFC 7011 Section 8.2.
Flow record start and end timestamps are unchanged.
<!-- source: internal/plugins/flowexport/ipfix/flow_adapter.go -- FlowEncoder.sendTemplateMessage, FlowEncoder.sendDataMessage -->

All sFlow sources use expanded counter and flow samples (formats 4 and 3).
Linux permits interface indexes above the compact encoding's limit, and the
sFlow specification forbids mixing compact and expanded encodings in one
agent. The source, input and output indexes retain all 32 bits. A counter
sample occupies 120 bytes; the expanded flow sample header occupies 52 bytes.
<!-- source: internal/plugins/flowexport/sflow/counter.go -- writeCounterSample, counterSampleSize -->
<!-- source: internal/plugins/flowexport/sflow/flow.go -- writeFlowSample, flowSampleHeaderSize -->

sFlow caps each captured header to the configured bound minus its overhead.
`trunc-size` accepts up to 1500 bytes, but the exported header can be shorter.
The Ethernet sample adds four FCS bytes to both `frame_length` and `stripped`,
because the kernel packet length excludes the FCS.
<!-- source: internal/plugins/flowexport/sflow/flow_adapter.go -- FlowEncoder.EncodeFlowSample, ethernetFCSOctets -->

sFlow counters include interface type, promiscuous mode and inbound multicast.
Speed and duplex come from sysfs and are zero or unknown when unavailable.
Broadcast, outbound multicast and unknown-protocol counts carry `0xFFFFFFFF`,
the unavailable-counter value, on every poll. IPFIX packet totals use separate
64-bit kernel totals and never add the sFlow sentinel values.
<!-- source: internal/plugins/flowexport/register.go -- notifyFromRateTracker, interfaceCountersFrom -->
<!-- source: internal/plugins/flowexport/ipfix/data.go -- writeCounterRecord -->

## Independent flow interpretation

<!-- source: internal/plugins/flowexport/interop_integration_linux_test.go -- TestFlowExportNFDumpInterop, TestFlowExportSFlowToolInterop -->

The [collector integration carrier](flow-export-1-counter-export.md#independent-collector-tests)
checks the records independent implementations decode from production encoder
output. NetFlow v9 and IPFIX each export one IPv4 and one IPv6 flow alongside
the counter record. The assertions require exact addresses, ports, protocol,
64-bit byte counts, packet counts and 32-bit source/destination ASNs, with no
duplicate or missing records.

For sFlow, the fixture contains an Ethernet frame carrying IPv4/UDP. `sflowtool`
must decode its MAC and IP addresses, EtherType and UDP ports, as well as the
expanded sample's source/input/output indexes. The expected 64-byte wire length,
four stripped FCS bytes and 60-byte captured header check the kernel-to-sFlow
length conversion. The test also requires one counter sample and one flow
sample with the shared sender's datagram sequence. It does not exercise psample
or conntrack ingestion; those producers retain their own tests.
