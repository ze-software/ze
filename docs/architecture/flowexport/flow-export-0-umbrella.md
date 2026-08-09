# Flow Export: Cross-Cutting Architecture

The `flowexport` plugin sends interface counters and per-flow records to sFlow,
NetFlow v9 and IPFIX collectors. This page holds the decisions that apply to the
whole plugin. The two halves have their own pages:

- [Counter export](flow-export-1-counter-export.md): interface counter samples.
- [Flow records](flow-export-2-flow-records.md): packet sampling, conntrack and
  per-flow records.

## Single collection, many consumers

<!-- source: internal/plugins/flowexport/register.go -- iface.SubscribeCollectNotify, notifyFromRateTracker -->
<!-- source: internal/component/iface/rate.go -- SubscribeCollectNotify, CollectNotifyFunc -->

Flow export does not read the kernel. It subscribes to the iface rate tracker
with `iface.SubscribeCollectNotify` and receives the same one-second snapshot
that Prometheus consumes. It uses the RAW counters, before baseline subtraction.
A second poll loop was rejected: two readers of the same kernel counters give two
different answers at the same instant.

The subscription is an in-process function call. The plugin refuses to start as
an external plugin process, because the subscription would cross a process
boundary and no-op without an error.

## Registration over imports

<!-- source: internal/plugins/flowexport/encoder_registry.go -- RegisterFlowSampleEncoderFactory, RegisterFlowRecordEncoderFactory -->
<!-- source: internal/plugins/flowexport/flowtypes.go -- FlowSampleEncoder, FlowRecordEncoder -->

The core never imports `flowexport`. The protocol packages `sflow`, `netflow9`
and `ipfix` import `flowexport` for `Sender` and the snapshot types, so
`flowexport` cannot import them back. The import graph therefore dictates the
design: `flowexport` declares the encoder interfaces, and each protocol package
registers a factory for its adapter at `init()` time.

This constraint is the reason the encoder interfaces exist. A direct call from
`flowexport` into `sflow` is an import cycle, not a style choice.

## Buffer-first encoding

<!-- source: internal/plugins/flowexport/sender.go -- MaxDatagramSize, buffer pool -->

Datagram assembly uses `WriteTo(buf, off) int` with skip-and-backfill for the
length and count fields. Buffers come from a shared `sync.Pool` of
`MaxDatagramSize` slices. `MaxDatagramSize` is 1400 bytes and has one home in
`sender.go`: a duplicate constant in the `sflow` package let the UDP payload
bound drift between packages, so it was removed.

## In-process component with the SDK protocol

<!-- source: internal/plugins/flowexport/register.go -- plugin registration and config wiring -->

`flowexport` runs inside the daemon process, which is what makes the iface
callback and the EventBus access direct calls. It still speaks the SDK config
protocol over an in-memory connection, so its config surface is identical to an
out-of-process plugin.
