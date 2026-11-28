---
spec: sflow-export-rfc-defects
topic: Flow Export
---

## Before

`FlowEncoder.EncodeFlowSample` computed the sFlow v5 `sample_pool` counter as `min(seq * rate, MaxUint32)`, which permanently froze the counter at `4294967295` (the sFlow v5 Section 5 "counter not available" sentinel) once a source crossed `2^32` represented packets and retroactively misstated `sample_pool` whenever the sampling rate changed. In addition, the Linux daemon-level tests proving the multi-homed routed vs. loopback `agent-address` warning (sFlow v5 Section 4.3) had never been committed.

## After

`FlowEncoder.EncodeFlowSample` maintains a per-source (`ifIndex`) `{seq, pool uint32}` accumulator created and reset together (`SFLOW-V5-x-28`), advancing `sample_pool` by each sample's `Rate` in unsigned 32-bit modular arithmetic so collectors compute accurate per-interval deltas across `2^32` rollover and rate changes. An independent `sflowtool` v6.11 Docker collector test (`TestFlowExportSFlowToolSamplePoolInterop`) verifies multi-source deltas and `2^32` wrap on the wire, and `test/flow-export/sflow-agent-address-multihomed-{routed,loopback}.ci` prove the multi-homed routed warning and loopback silence against a live Linux daemon.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
