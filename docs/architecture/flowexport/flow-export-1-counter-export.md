# Flow Export: Counter Export

Interface counter export: sFlow v5 counter samples, and NetFlow v9 / IPFIX
interface-counter records. Cross-cutting decisions are in
[the umbrella page](flow-export-0-umbrella.md).

## IPFIX counter records use the Total IEs, not the Delta IEs

<!-- source: internal/plugins/flowexport/ipfix/ie.go -- IEOctetTotalCount, IEPacketTotalCount -->

Interface counters are raw cumulative kernel values. IPFIX counter records
therefore carry `octetTotalCount` (IE 85) and `packetTotalCount` (IE 86).

RFC 7012 defines the Delta IEs (1 and 2) as per-interval deltas. Sending a
cumulative value in a Delta IE tells the collector the value is a delta, and the
collector misreports the rate. The Delta IE constants stay defined because the
per-flow path uses them, where conntrack supplies real deltas.

`packetTotalCount` uses the full-width receive and transmit packet totals.
Those totals already include multicast and broadcast traffic. The separate
sFlow fields use 32-bit counters and unavailable-value sentinels, so summing
them would double-count traffic or introduce false packet totals.
<!-- source: internal/plugins/flowexport/register.go -- interfaceCountersFrom -->
<!-- source: internal/plugins/flowexport/ipfix/data.go -- writeCounterRecord -->

## Datagram metrics count datagrams, not Encode calls

<!-- source: internal/plugins/flowexport/exporter.go -- notifySnapshot -->
<!-- source: internal/plugins/flowexport/metrics.go -- datagram counters -->

sFlow batches counter samples and spills the overflow into more datagrams, so one
`Encode()` call can send several datagrams. `ze_flowexport_datagrams_total`
increments by the sender's datagram-count delta measured across both the template
and data sends. One increment per `Encode()` call undercounts every spill; starting
the measurement after the template undercounts every refresh.

An encoder can return successful records together with a later send error.
The exporter still records those successful datagrams, bytes and records.
<!-- source: internal/plugins/flowexport/exporter.go -- notifySnapshot, exportFlows -->

## A transport session owns one sequence

Counter and flow encoders for a collector use the same sequence on its UDP
sender. IPFIX advances it by the data records successfully sent; template
messages carry the current sequence without advancing it. NetFlow v9 counts
every successfully sent export packet, including templates. sFlow counts each
generated datagram, whether it contains counter or flow samples. Unsigned
32-bit addition wraps each sequence.
`show flow export` reads that same sender sequence; it does not maintain a
separate record count.
<!-- source: internal/plugins/flowexport/sender.go -- Sender.Sequence, Sender.AdvanceSequence -->
<!-- source: internal/plugins/flowexport/ipfix/adapter.go -- CounterEncoder.Encode, CounterEncoder.EncodeTemplate -->
<!-- source: internal/plugins/flowexport/ipfix/flow_adapter.go -- FlowEncoder.sendDataMessage, FlowEncoder.sendTemplateMessage -->
<!-- source: internal/plugins/flowexport/netflow9/adapter.go -- CounterEncoder.Encode, CounterEncoder.EncodeTemplate -->
<!-- source: internal/plugins/flowexport/netflow9/flow_adapter.go -- FlowEncoder.sendDataPacket, FlowEncoder.sendTemplatePacket -->
<!-- source: internal/plugins/flowexport/sflow/adapter.go -- CounterEncoder.Encode -->
<!-- source: internal/plugins/flowexport/sflow/flow_adapter.go -- FlowEncoder.EncodeFlowSample -->

## The template timestamp advances only after a successful send

<!-- source: internal/plugins/flowexport/exporter.go -- collectorState.lastTemplate -->

`lastTemplate` is set after `EncodeTemplate` succeeds. When a template send
fails, the timestamp stays put, data waits, and the next eligible snapshot or
flow batch retries the template. A partially sent template batch still counts
toward datagram and byte metrics. Neither a failed template nor a failed data
batch advances the counter poll timestamp.

The IPFIX counter encoder also retains the Export Time of its last successfully
sent Template message. Template refreshes and Data message headers use this
value as a minimum, so clock rollback cannot place data before its original
template. The counter records keep their snapshot timestamps.
<!-- source: internal/plugins/flowexport/ipfix/adapter.go -- CounterEncoder.EncodeTemplate, CounterEncoder.Encode -->

## sFlow if_counters truncation is the format, not a defect

<!-- source: internal/plugins/flowexport/sflow/counter.go -- if_counters encoding -->

Fields 7 to 18 of the sFlow v5 `if_counters` structure are XDR 32-bit. The kernel
counters are `uint64`. Truncating them to `uint32` is what the sFlow v5 structure
definition requires. Do not "fix" the truncation.

The sFlow counter encoder compares `CounterGeneration` from each raw snapshot.
A generation change restarts the source sample sequence even when the new
interface's counters already exceed the old values. A normal 32-bit wire-counter
wrap does not change a known generation and therefore does not restart samples.
Sources without generation metadata fall back to decreases visible between polls.
<!-- source: internal/plugins/flowexport/snapshot.go -- InterfaceCounters.CounterGeneration -->
<!-- source: internal/plugins/flowexport/register.go -- interfaceCountersFrom -->
<!-- source: internal/plugins/flowexport/sflow/adapter.go -- resetSequencesOnDiscontinuity, countersWentBackwards -->

Linux supplies the generation alongside raw counters from an ordered link-event
and GETLINK stream. Interface replacement and full-width counter decreases
change it. Loss of notification history discards the failed snapshot and starts
fresh generations on the next successful read. There is no generic Linux
driver-reset epoch: a reset masked by enough subsequent traffic cannot be
detected from counter values alone.
<!-- source: internal/plugins/iface/netlink/counter_linux.go -- counterSource.consume, counterSource.invalidate, counterStatsDecreased -->
<!-- source: internal/plugins/iface/netlink/show_linux.go -- ListInterfaces, GetInterface -->

## Independent collector tests

<!-- source: internal/plugins/flowexport/interop_integration_linux_test.go -- TestFlowExportNFDumpInterop, TestFlowExportSFlowToolInterop -->
<!-- source: test/interop-flowexport/Dockerfile.collectors -->

The Linux integration carrier sends production encoder output through `Sender`
to independent collectors. `TestFlowExportNFDumpInterop` runs NetFlow v9 and
IPFIX against `nfcapd`, then compares `nfdump` JSON records with the fixture's
interface counters and IPv4/IPv6 flows. NetFlow's current counter template
exports separate inbound/outbound octets and unicast packet counts. IPFIX
exports combined receive/transmit totals through IEs 85 and 86.

`TestFlowExportSFlowToolInterop` checks `sflowtool`'s decoded generic counters,
including 64-bit octets and unavailable-counter sentinels. Its interface indexes
exceed both compact sFlow limits, so the assertions detect truncation of the
source index or the sampled packet's input/output ports.

The carrier requires `-tags integration` and an explicit
`-flowexport-peer-dir=<directory>` containing the peer executables. With no
directory, generic integration discovery reports a prerequisite skip. With a
directory, a missing executable fails the test. A skip supplies no
interoperability evidence. The collector Dockerfile builds nfdump v1.7.10 at
`9e47b47daa1c6a8a9a7ea92920b1b6b3ecb2ec7d` and sflowtool v6.11 at
`c953cfe8991286457080938a29fc72019f64718a`; its default command supplies the
installed peer directory and selects both tests.

Each collector gets a dynamic loopback UDP port. Readiness requires its socket
inode to appear among its own file descriptors and its UDP table. The fixtures
are sent once, and observation waits for completed rotation files or complete
JSON datagrams. Cleanup terminates and reaps only the owned child, with bounded
waits and a kill fallback. These tests cover encoder/transport interoperability;
daemon counter collection remains a separate test surface.

`TestFlowExportDaemonCounterGeneration` covers the separate producer path. It
starts the supplied Ze executable in an isolated Linux network namespace and
sends raw Ethernet frames through a dummy interface. Its UDP observations check
sFlow unavailable-counter sentinels and IPFIX's combined raw totals. Down/up
must preserve the sFlow source sequence; deletion and recreation at the same
ifIndex, with larger counters, must restart it without resetting the datagram
sequence. The daemon is stopped across each link change, so an intervening
empty poll cannot substitute for generation delivery.

This carrier requires `-tags integration`,
`-flowexport-ze-path=<absolute executable path>`, and Linux namespace, link
management and raw-socket privileges (`CAP_SYS_ADMIN`, `CAP_NET_ADMIN`,
`CAP_NET_RAW`). Build the executable with at least `ze_core ze_flowexport`.
No supplied path means a prerequisite skip; a supplied path with missing
privileges or an unusable daemon fails. The child is terminated and reaped
before the namespace is released.
<!-- source: internal/plugins/flowexport/producer_integration_linux_test.go -- TestFlowExportDaemonCounterGeneration -->
