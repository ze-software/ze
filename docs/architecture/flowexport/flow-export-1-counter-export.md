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

## Datagram metrics count datagrams, not Encode calls

<!-- source: internal/plugins/flowexport/exporter.go -- notifySnapshot -->
<!-- source: internal/plugins/flowexport/metrics.go -- datagram counters -->

sFlow batches counter samples and spills the overflow into more datagrams, so one
`Encode()` call can send several datagrams. `ze_flowexport_datagrams_total`
increments by the sender's datagram-count delta measured around the call. One
increment per `Encode()` call undercounts every spill.

## The template timestamp advances only after a successful send

<!-- source: internal/plugins/flowexport/exporter.go -- collectorState.lastTemplate -->

`lastTemplate` is set after `EncodeTemplate` succeeds. When a template send
fails, the timestamp stays put and the next tick retries. Setting the timestamp
before the send suppressed the retry for a full refresh interval, and a collector
that missed the template dropped every data record until then.

## sFlow if_counters truncation is the format, not a defect

<!-- source: internal/plugins/flowexport/sflow/counter.go -- if_counters encoding -->

Fields 7 to 18 of the sFlow v5 `if_counters` structure are XDR 32-bit. The kernel
counters are `uint64`. Truncating them to `uint32` is what the sFlow v5 structure
definition requires. Do not "fix" the truncation.
