# Spec: sflow-export-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | An sFlow collector's sample pool stops advancing once it saturates at `MaxUint32`, which sFlow v5 defines as the "not available" counter sentinel, and the multi-homed routed agent-address warning lacks its daemon-level `.ci` proof. |
| RFC gaps | SFLOW-V5-x-11 |
| Depends | - (split from `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` so the flow-export/system-plugins door and the BGP/BMP doors are owned in separate specs) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Scope is protocol: the wire value semantics of the sFlow v5 `flow_sample_expanded.sample_pool` field change from saturating at `2^32 - 1` to wrapping modulo `2^32` per data source, and two daemon-level `.ci` tests prove the sFlow v5 Section 4.3 multi-homed agent-address warning. No config syntax, YANG schema, or CLI command changes.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions

| Topic | Date | Decision |
|-------|------|----------|
| sFlow v5 `sample_pool` calculation (D2 / AC-1) | 2026-11-24 | Compute `sample_pool` as a per-source accumulated rate estimate wrapping modulo `2^32` (instead of saturating at `2^32 - 1`). |
| sFlow v5 collector interop carrier (Q-8 / AC-4) | 2026-10-14 | Extend the existing independent `sflowtool` collector carrier (`internal/plugins/flowexport/collectors_docker_test.go`, build tag `docker`) with a sibling test (`TestFlowExportSFlowToolSamplePoolInterop`) rather than adding a new BGP-lab interop scenario. |
| RFC-tagged unit update (Q-7 / `TestSFlowFlowSamplePoolTracksTotal`) | 2026-10-13 | Approved inverting the saturation subtest to wrap modulo `2^32` and rewriting its tag prose; per `ai/META-DECISIONS.md` ("Test changes"), the edit still runs through independent `ze-test-judge` approval via `./le rfc approve` before commit. |

## Task

This spec owns the sFlow v5 work split out of `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` (which retains the BMP R-1..R-4 residuals on the BGP engine and BGP plugins doors):

1. **sFlow v5 `sample_pool` saturation and rate-change defect (D2, `SFLOW-V5-x-11`):** `FlowEncoder.EncodeFlowSample` (`internal/plugins/flowexport/sflow/flow_adapter.go`) currently estimates `sample_pool` as `min(uint64(seq)*uint64(sample.Rate), MaxUint32)`. That has two defects against sFlow v5 Section 5:
   - Saturating at `2^32 - 1` (`4294967295`) writes the Section 5 "Unknown counter. Use the maximum counter value to indicate that the counter is not available." sentinel and freezes the counter permanently so a collector's per-interval delta reads zero. Section 5 permits a counter to wrap to zero ("An available counter may temporarily have the max value just before it rolls to zero. This is permitted.").
   - Multiplying `seq` by the current sample's `Rate` misstates `sample_pool` if a data source's sampling rate changes between samples.
   - Goal: per the 2026-11-24 owner decision, maintain a per-source (`ifIndex`) accumulated rate estimate beside each source's `sequence_number` (created and reset together per Section 5's `sequence_number` note), advancing by each sample's `Rate` in unsigned 32-bit modular arithmetic wrapping modulo `2^32`. Update `TestSFlowFlowSamplePoolTracksTotal`, add `TestSFlowSamplePoolPerSource`, add the `sflowtool` sibling test `TestFlowExportSFlowToolSamplePoolInterop`, refresh the `SFLOW-V5-x-11` discrimination record, and independently re-judge `rfc/audit/sflow-v5/SFLOW-V5-x-11.json` from `weak` to `enforced`.
2. **Multi-homed sFlow agent-address warning `.ci` coverage (journal row `claim-outlives-the-evidence-it-cites.md`, 2026-10-30):** `internal/test/fixture/register_sflow_agent_address_linux.go` already provides the Linux `CAP_NET_ADMIN` veth-pair fixture (`flow-export/sflow-agent-address-daemon` and `flow-export/sflow-agent-address-receiver`, exclusive group `sflow-agent-address-topology`), but the two `.ci` cases (`sflow-agent-address-multihomed-routed.ci` and `sflow-agent-address-multihomed-loopback.ci`) were never committed. Commit both under `test/flow-export/` and update `docs/architecture/flowexport/flow-export-1-counter-export.md` to anchor them.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform and process-boundary constraints
  → Constraint: `flowexport` is an edge plugin under `internal/plugins/`; no cross-component direct state calls.
- [ ] `ai/doors/system-plugins.md` - door covering `internal/plugins/flowexport/*`
  → Constraint: `flowexport` owns its own YANG, encoders, validators and advisors; preserve its existing registration factories and tests.
- [ ] `ai/doors/le-tooling.md` - door covering `test/flow-export/*.ci` and `internal/test/fixture/`
  → Constraint: Linux kernel tests carry `option=needs-linux` and `caps=CAP_NET_ADMIN`; compiled fixtures return errors on failure.
- [ ] `docs/architecture/flowexport/flow-export-2-flow-records.md` - `// Design:` page of `flow_adapter.go` and `flow.go`
  → Decision: all sFlow sources use expanded flow samples (format 3); `FlowEncoder` holds per-source state keyed by `ifIndex`; update this page in the same change to document per-source wrapping `sample_pool` and `TestFlowExportSFlowToolSamplePoolInterop`.
- [ ] `docs/architecture/flowexport/flow-export-1-counter-export.md` - `// Design:` page of `collectors_docker_test.go` and `register_sflow_agent_address_linux.go`
  → Decision: the `sflowtool` carrier runs under build tag `docker` (`./le test integration docker-peers`) using pinned `sflowtool` v6.11 (`c953cfe8991286457080938a29fc72019f64718a`); the multi-homed agent-address fixture owns veth pair `sfproof0`/`sfproof1` and requires exclusive group `sflow-agent-address-topology`.
- [ ] `docs/guide/flow-export.md` - operator guide for flow export
  → Constraint: document under Sampling that `sample_pool` is a per-source accumulated rate estimate wrapping modulo `2^32`.
- [ ] `docs/architecture/testing/interop.md` - independent collector test inventory
  → Constraint: line 5727 anchors `collectors_docker_test.go` (`TestFlowExportNFDumpInterop`, `TestFlowExportSFlowToolInterop`); add `TestFlowExportSFlowToolSamplePoolInterop` to that anchor in the same change.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/sflow-v5.md` - sFlow Version 5 compliance checklist
  → Constraint: `SFLOW-V5-x-11` (`[MUST]`, Section 5) keeps its verbatim field definition and MUST level per `rfc/corrections/sflow-v5.md` (2026-11-14); its `{single-polarity: ...}` annotation currently describes the old saturated product and must be updated to per-source accumulated rate wrapping modulo `2^32`.
  → Constraint: `SFLOW-V5-x-28` (`[MUST]`, Section 5) requires resetting `sequence_number` whenever `sample_pool` resets; a natural 32-bit wrap is not a reset, and `FlowEncoder` creates and resets `seq` and `pool` together per source.
  → Constraint: `SFLOW-V5-4.3-2` (`[SHOULD]`, Section 4.3) recommends a loopback agent address on a multi-homed router; proven at the daemon level by the two new `.ci` files.

### Specification text read (verbatim, `rfc/full/sflow-v5.txt`)

| Section | Quote |
|---------|-------|
| 5 (`flow_sample`) | "unsigned int sample_pool;      /* Total number of packets that could have been sampled (i.e. packets skipped by sampling process + total number of samples) */" |
| 5 (`flow_sample`, `sequence_number`) | "Note: If the agent resets the sample_pool then it must also reset the sequence_number." |
| 5 (Counter conventions) | "Unknown counter. Use the maximum counter value to indicate that the counter is not available." |
| 5 (Counter conventions) | "An available counter may temporarily have the max value just before it rolls to zero. This is permitted." |
| 4.3 (`sFlowAgentAddress`) | "In the case of a multi-homed agent, this should be the loopback address of the agent." |

**Key insights:**
- `FlowEncoder` already holds per-source state keyed by `ifIndex` (`seqNums map[uint32]uint32`); replacing the value type with a small two-field struct (`seq uint32`, `pool uint32`) keeps sequence and pool created and reset together with zero extra map lookups.
- `TestFlowExportSFlowToolInterop` (`collectors_docker_test.go`) pins exactly two datagrams, no duplicate sample types, and `sampleSequenceNo: "1"`, so adding a sibling test `TestFlowExportSFlowToolSamplePoolInterop` in the same file preserves all existing assertions untouched (owner decision Q-8).
- `TestFlowExportSFlowToolSamplePoolInterop` sets `FlowSample.Rate` directly on the public `flowexport.FlowSample` value (YANG `1..1000000` binds config only), allowing `Rate: 1 << 31` (`2147483648`) to wrap cleanly across `2^32` in three samples (`2147483648`, `0`, `2147483648`) without ever writing `4294967295`.
- The multi-homed agent-address fixture (`internal/test/fixture/register_sflow_agent_address_linux.go`) is already built, registered, and documented; only the two `.ci` files calling it are missing.

## Current Behavior (MANDATORY)

**Source files read (verified at HEAD on 2026-11-26):**
- [ ] `internal/plugins/flowexport/sflow/flow_adapter.go` - `FlowEncoder` holds `seqNums map[uint32]uint32`. `EncodeFlowSample` increments `seq := e.seqNums[sample.IfIndex] + 1` and computes `pool := uint32(min(uint64(seq)*uint64(sample.Rate), uint64(^uint32(0))))`, saturating at `MaxUint32` (`4294967295`) and multiplying `seq` by the current sample's `Rate`.
- [ ] `internal/plugins/flowexport/sflow/flow.go` - `writeFlowSample` writes `seqNum` at offset 8 and `pool` at offset 24 as big-endian `uint32` inside `flow_sample_expanded` (format `0:3`).
- [ ] `internal/plugins/flowexport/sflow/sflow_v5_flow_adapter_test.go` - `TestSFlowFlowSamplePoolTracksTotal` (tagged `SFLOW-V5-x-11 positive`) asserts `grows by rate per sample` (`rate = 1024`, 3 samples -> 1024, 2048, 3072) and `saturates rather than wraps` (`rate = MaxUint32`, 2 samples -> both `MaxUint32`). No per-source or rate-change test exists.
- [ ] `internal/plugins/flowexport/sflow/sflow_v5_test.go` - `SFLOW-V5-x-28` positive and negative tests assert a fresh encoder starts at `seq == 1`, `pool == rate`, and a continuing encoder reaches `seq == 4`, `pool == 4*rate` at constant rate; both remain true under per-source accumulation.
- [ ] `internal/plugins/flowexport/collectors_docker_test.go` - `TestFlowExportSFlowToolInterop` (tag `docker`) starts pinned `sflowtool` v6.11 via `labtest`, sends one counter sample and one flow sample at `Rate: 64`, and asserts `"samplePool": "64"`. No sibling sample-pool delta/wrap test exists.
- [ ] `rfc/audit/sflow-v5/SFLOW-V5-x-11.json` - verdict is `weak` because the test asserts saturation rather than wrapping and does not test against the definition/wrap conventions.
- [ ] `internal/test/fixture/register_sflow_agent_address_linux.go` - registers `flow-export/sflow-agent-address-daemon` (creates veth pair `sfproof0` `192.0.2.41/32` and `sfproof1` `198.51.100.41/32` and launches `ze -`) and `flow-export/sflow-agent-address-receiver` (verifies both veth interfaces and reads a real sFlow v5 counter datagram with the configured `agent-address`).
- [ ] `test/flow-export/sflow-agent-address-warning.ci` - tests only the absent-address warning; no `sflow-agent-address-multihomed-routed.ci` or `sflow-agent-address-multihomed-loopback.ci` exists in `test/flow-export/`.

**Behavior to preserve:**
- Below `2^32` at constant rate, `sample_pool` grows by `rate` per exported sample (`1024`, `2048`, `3072` for rate `1024`), matching `TestSFlowFlowSamplePoolTracksTotal` ("grows by rate per sample") and both `SFLOW-V5-x-28` tests in `sflow_v5_test.go`.
- `FlowEncoder` continues to emit `flow_sample_expanded` (format `0:3`) with full-width 32-bit `ifIndex` and `output`, FCS adjustment (`ethernetFCSOctets = 4`), and `max-datagram-size` header truncation.
- `TestFlowExportSFlowToolInterop` keeps all existing counter and flow-sample assertions unchanged.
- `sflow-agent-address-warning.ci` and the unit/engine advisor tests in `sflow_v5_agent_advice_test.go` stay untouched and green.

**Behavior to change:**
- `FlowEncoder.EncodeFlowSample` accumulates `sample.Rate` per source (`ifIndex`) in unsigned 32-bit modular arithmetic wrapping modulo `2^32`, instead of saturating `seq * rate` at `MaxUint32`.
- `TestSFlowFlowSamplePoolTracksTotal` inverts its saturation subtest to wrap modulo `2^32` and updates its tag prose; `TestSFlowSamplePoolPerSource` adds independent multi-source and rate-change coverage; `TestFlowExportSFlowToolSamplePoolInterop` proves independent `sflowtool` decoding across multiple samples and wrap past `2^32`.
- `SFLOW-V5-x-11` discrimination record and audit verdict move from `weak` to `enforced`.
- Two new Linux `.ci` tests (`test/flow-export/sflow-agent-address-multihomed-routed.ci` and `test/flow-export/sflow-agent-address-multihomed-loopback.ci`) exercise the existing `sflow-agent-address` fixture end to end through the daemon.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Per-source sFlow flow-sample state | `internal/plugins/flowexport/sflow/flow_adapter.go`, `FlowEncoder.seqNums` and `EncodeFlowSample` | Tracks `seq` per `ifIndex` in `map[uint32]uint32` and computes `sample_pool` as `min(uint64(seq)*uint64(sample.Rate), MaxUint32)` | Extends: replaces the map value with a two-field per-source struct (`seq`, `pool` both `uint32`), so sequence and accumulated pool are created and reset together per `ifIndex` and `pool` wraps modulo `2^32` |
| Expanded flow sample wire writer | `internal/plugins/flowexport/sflow/flow.go`, `writeFlowSample` | Writes big-endian `uint32` `seqNum` and `pool` into the 52-byte `flow_sample_expanded` header | Does not apply (reused unchanged): the wire width and byte offset (24) already carry a `uint32` |
| Independent `sflowtool` collector carrier | `internal/plugins/flowexport/collectors_docker_test.go`, `TestFlowExportSFlowToolInterop` (tag `docker`); image `test/interop-flowexport/Dockerfile.collectors` | Starts pinned `sflowtool` v6.11 in a `labtest` container, sends UDP datagrams through `flowexport.Sender`, parses `-j` JSON lines | Extends (owner decision Q-8): adds sibling `TestFlowExportSFlowToolSamplePoolInterop` in the same file reusing `startExportCollector`, `exportInteropSender`, and `exportSFlowDatagrams`; existing test is untouched |
| Multi-homed sFlow agent-address fixture | `internal/test/fixture/register_sflow_agent_address_linux.go`, `sflowAgentAddressDaemon`, `sflowAgentAddressReceiver` | Creates veth pair `sfproof0`/`sfproof1`, runs `ze -`, verifies real routed interfaces and received sFlow v5 counter datagram | Extends: adds the two missing `.ci` callers in `test/flow-export/` that invoke this fixture under `option=needs-linux`, `caps=CAP_NET_ADMIN`, and exclusive group `sflow-agent-address-topology` |
| RFC discrimination and audit workflow | `./le rfc approve`, `./le rfc record`, `./le rfc audit-stamp ... mode rejudge`; `rfc/audit/sflow-v5/SFLOW-V5-x-11.json` | Records breaking-mutation discrimination and independent verdicts per requirement | Extends: records discrimination and independent re-judge for `SFLOW-V5-x-11` after the producer fix |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "The RFC, then ExaBGP API compatibility, then ExaBGP's behavior, in that order (`ai/rules/rfc-compliance.md`)" | agrees | Wrapping `sample_pool` modulo `2^32` follows sFlow v5 Section 5 counter conventions so external sFlow collectors keep computing accurate deltas instead of reading `MaxUint32` as "not available" |
| `docs/architecture/flowexport/flow-export-2-flow-records.md` | Export limits and counter sources | the `// Design:` page of `flow_adapter.go` and `collectors_docker_test.go`; modified by this spec | "All sFlow sources use expanded counter and flow samples (formats 4 and 3)." | agrees | `FlowEncoder.EncodeFlowSample` keeps expanded format 3 with full-width indexes and header truncation, changes only `sample_pool` accumulation to wrap modulo `2^32`, and updates this page to document wrapping `sample_pool` and `TestFlowExportSFlowToolSamplePoolInterop` |
| `docs/architecture/flowexport/flow-export-0-umbrella.md` | Buffer-first encoding | anchored to `internal/plugins/flowexport/sflow/flow_adapter.go` | "Shared pooled buffers have a fixed 1400-byte capacity." | agrees | `EncodeFlowSample` continues writing directly into the pooled `flowexport.GetBuf()` buffer with skip-and-backfill and `max-datagram-size` header capping; unchanged |
| `docs/architecture/flowexport/flow-export-1-counter-export.md` | Daemon agent-address warning tests | anchored to `flow_adapter.go`; the `// Design:` page of `collectors_docker_test.go`; modified by this spec | "The fixture requires Linux and `CAP_NET_ADMIN`. Its callers must share the exclusive group `sflow-agent-address-topology`, because it owns a named veth pair." | agrees | Both new multi-homed `.ci` callers declare `option=needs-linux`, `caps=CAP_NET_ADMIN`, and exclusive group `sflow-agent-address-topology` to invoke the existing fixture; this spec also adds sibling `TestFlowExportSFlowToolSamplePoolInterop` to the `docker`-tagged carrier and updates both sections of this page in the same change |
| `ai/doors/system-plugins.md` | Boundaries | the door covering `internal/plugins/flowexport/*` | "A plugin owns its whole surface." | agrees | All changes stay inside `internal/plugins/flowexport/` and its tests/docs; no central registry or cross-component import is added |
| `docs/architecture/testing/interop.md` | A package test's own server container (labtest) | anchored to `internal/plugins/flowexport/collectors_docker_test.go`; modified by this spec | "A test that reads what the server wrote calls `labtest.StartContainer`, which answers the `Container` with its address and `Container.Exec`." | agrees | `TestFlowExportSFlowToolSamplePoolInterop` uses the same `startExportCollector` (`labtest.StartContainer`) and `docker`-tagged `sflowtool` carrier, and adds its symbol to the anchor in this section |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/flow-export/sflow-agent-address-multihomed-{routed,loopback}.ci` | "A `.ci` that needs a Linux kernel feature carries `option=needs-linux`" | agrees | Both new multi-homed agent-address `.ci` files carry `option=needs-linux`, `caps=CAP_NET_ADMIN`, and exclusive group `sflow-agent-address-topology`, and invoke the existing compiled fixtures |
| `docs/architecture/testing/interop.md` | Design decisions | `docs/architecture/testing/interop.md`, decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | `TestFlowExportSFlowToolSamplePoolInterop` runs in `collectors_docker_test.go` via `labtest` (`docker-peers`), requiring no kernel capability check or unrelated suite features |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- D2: a sampled Ethernet frame delivered as `flowexport.FlowSample` (`IfIndex`, `Rate`, `Output`, `OrigSize`, `Header`) to `FlowEncoder.EncodeFlowSample(sample, sender)`.
- Agent-address `.ci` tests: `ze -` started by `flow-export/sflow-agent-address-daemon` with a multi-homed veth topology (`sfproof0`, `sfproof1`) and an `sflow` collector config whose `agent-address` is either `192.0.2.41` (routed veth) or `127.0.0.1` (loopback).

### Transformation Path
1. `FlowEncoder.EncodeFlowSample` looks up (or zero-initializes) the per-`IfIndex` source state (`seq`, `pool`) in `FlowEncoder.sources` (replacing `seqNums`).
2. It advances `state.seq++` (unsigned 32-bit wrap) and accumulates `state.pool += sample.Rate` (unsigned 32-bit addition, wrapping modulo `2^32`, never clamping to `MaxUint32`), then stores `state` back under `sample.IfIndex`.
3. `writeFlowSample` encodes `state.seq` at offset 8 and `state.pool` at offset 24 of the `flow_sample_expanded` record, `writeSampledHeader` encodes the sampled Ethernet header with 4-byte FCS compensation, `backfillFlowSample` sets length and record count, `sender.AdvanceSequence(1)` advances the shared datagram sequence, and `sender.Send` transmits the UDP datagram.
4. For the agent-address `.ci` cases, `sflowAgentAddressDaemon` creates the veth pair and launches `ze -`; `flowexport`'s `adviseSFlowCollectors` inspects the router's interfaces on config apply, emits the multi-homed routed warning when `agent-address` is on `sfproof0` (`192.0.2.41`), emits no warning when `agent-address` is on `lo` (`127.0.0.1`), and exports real sFlow v5 counter datagrams verified by `sflowAgentAddressReceiver`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `flowexport` worker to `sflow.FlowEncoder` | `FlowSampleEncoder.EncodeFlowSample(FlowSample, *Sender)` interface | Yes: `flowtypes.go`, `flow_adapter.go` |
| `sflow.FlowEncoder` to external collector | sFlow v5 UDP datagram (`flow_sample_expanded`, format `0:3`), XDR big-endian uint32 `sample_pool` | Yes: `flow.go::writeFlowSample`, `collectors_docker_test.go` |
| `.ci` runner to daemon and Linux kernel | `le test fixture flow-export/sflow-agent-address-{daemon,receiver}` with `option=needs-linux` and `caps=CAP_NET_ADMIN` | Yes: `internal/test/fixture/register_sflow_agent_address_linux.go` |

### Changed-Wire Census

**Search evidence:** scope `internal/plugins/flowexport`, `test`, `docs`. Terms: `sample_pool`, `samplePool`, `seqNums`, `writeFlowSample`, `EncodeFlowSample`. Positive control: `flow_adapter.go::EncodeFlowSample` is found as the sole producer of `sample_pool`, `flow.go::writeFlowSample` as the sole XDR writer (offset 24), `collectors_docker_test.go::TestFlowExportSFlowToolInterop` as the independent `sflowtool` reader, and `sflow_v5_flow_adapter_test.go`, `sflow_v5_flow_test.go`, `sflow_v5_test.go` as the in-tree test decoders. No Python, shell, or external-plugin script in the repository decodes sFlow `flow_sample_expanded`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `sFlow v5 flow_sample_expanded (sample_pool uint32)` | writer | `internal/plugins/flowexport/sflow/flow_adapter.go::FlowEncoder.EncodeFlowSample` -> `flow.go::writeFlowSample` | `git grep` shows `EncodeFlowSample` is the sole caller of `writeFlowSample` in production; computes `pool` and writes big-endian `uint32` at offset 24 | Change value calculation from `min(uint64(seq)*uint64(rate), MaxUint32)` to per-source accumulated `uint32` addition wrapping modulo `2^32`; XDR layout unchanged |
| `sFlow v5 flow_sample_expanded (sample_pool uint32)` | reader | External sFlow v5 collectors, including pinned `sflowtool` v6.11 (`test/interop-flowexport/Dockerfile.collectors`) exercised by `internal/plugins/flowexport/collectors_docker_test.go::TestFlowExportSFlowToolInterop` and new sibling `TestFlowExportSFlowToolSamplePoolInterop` | `collectors_docker_test.go` runs `sflowtool -4 -p 6343 -j` and reads `"samplePool"` from each JSON datagram line | Preserve `TestFlowExportSFlowToolInterop` unchanged (`samplePool` `"64"` for 1 sample at rate 64); add sibling `TestFlowExportSFlowToolSamplePoolInterop` proving per-source accumulation and wrap past `2^32` |
| `sFlow v5 flow_sample_expanded (sample_pool uint32)` | reader | In-tree unit test decoders (`internal/plugins/flowexport/sflow/sflow_v5_flow_adapter_test.go::TestSFlowFlowSamplePoolTracksTotal`, `sflow_v5_test.go` SFLOW-V5-x-28 tests, `sflow_v5_flow_test.go`) | `sflow_v5_flow_adapter_test.go:70,94` reads `binary.BigEndian.Uint32(dg[samplePoolOffset:])`; `sflow_v5_test.go:322,342,347` checks `fs.pool` at constant rate below `2^32` | Update `TestSFlowFlowSamplePoolTracksTotal` saturation subtest to wrap modulo `2^32` (with `ze-test-judge` approval); add `TestSFlowSamplePoolPerSource`; `sflow_v5_test.go` and `sflow_v5_flow_test.go` stay green unchanged |

### Integration Points
- `FlowEncoder` (`internal/plugins/flowexport/sflow/flow_adapter.go`) - per-`ifIndex` struct `{seq, pool uint32}` replaces `seqNums map[uint32]uint32`.
- `collectors_docker_test.go` (`internal/plugins/flowexport/collectors_docker_test.go`) - adds sibling `TestFlowExportSFlowToolSamplePoolInterop` sharing `startExportCollector`, `exportInteropSender`, `exportInteropEthernetFrame`, and `exportSFlowDatagrams`.
- `register_sflow_agent_address_linux.go` (`internal/test/fixture/register_sflow_agent_address_linux.go`) - already registered; invoked by the two new `test/flow-export/sflow-agent-address-multihomed-*.ci` files.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `FlowEncoder.EncodeFlowSample` writes through `writeFlowSample` and `flowexport.Sender`, unchanged |
| No unintended coupling (components stay isolated) | Yes | Changes stay within `internal/plugins/flowexport/` and `test/flow-export/`; no cross-component or BGP imports |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `FlowEncoder`'s per-ifIndex map, `collectors_docker_test.go`'s `sflowtool` harness, and the existing `sflow-agent-address` Linux fixture |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `EncodeFlowSample` still writes directly into the pooled `flowexport.GetBuf()` buffer with no extra heap allocation per sample |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new command, view, family, or factory; `newSFlowFlowEncoder` registration in `sflow/register.go` is unchanged |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/plugins/flowexport/` and `internal/test/fixture/`: `.ci` files in `test/flow-export/` are discovered automatically by the functional runner and the fixture names `flow-export/sflow-agent-address-{daemon,receiver}` are already registered |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|---------------------------------|----------|--------------|--------|
| A-1 | `sflowtool` v6.11 (`c953cfe8991286457080938a29fc72019f64718a`) decodes `sample_pool` as an unsigned 32-bit integer and prints `"2147483648"`, `"0"`, `"2147483648"` across the `2^32` wrap | `collectors_docker_test.go` already decodes `"samplePool": "64"` from `sflowtool -j`; sFlow v5 XDR `sample_pool` is `unsigned int` | Test expectations or JSON key formatting would fail on first run | `TestFlowExportSFlowToolSamplePoolInterop` under `-tags docker` | unvalidated |
| A-2 | The existing `flow-export/sflow-agent-address-daemon` and `flow-export/sflow-agent-address-receiver` fixtures work unmodified with both routed (`192.0.2.41`) and loopback (`127.0.0.1`) `.ci` files inside the Linux QEMU/CAP_NET_ADMIN test runner | `internal/test/fixture/register_sflow_agent_address_linux.go` lines 35-231 and `docs/architecture/flowexport/flow-export-1-counter-export.md` lines 180-210 | Fixture needs a small fix in the same work | Running both `.ci` files on Linux with `CAP_NET_ADMIN` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | An existing `SFLOW-V5-x-28` unit test breaks if per-source pool state is not reset with `seq` | `sflow_v5_test.go` fails | Store `seq` and `pool` in one map value struct per `ifIndex`, so creating/resetting an encoder resets both together |
| R-2 | Editing RFC-tagged `TestSFlowFlowSamplePoolTracksTotal` is refused by the test-weakening / RFC-tag hook | Pre-write hook or commit gate refusal | Obtain independent `ze-test-judge` approval via `./le rfc approve unit sflow.TestSFlowFlowSamplePoolTracksTotal ...` before committing |
| R-3 | The two multi-homed `.ci` tests collide on veth pair `sfproof0`/`sfproof1` when run in parallel | `create owned sFlow proof veth pair: file exists` | Both `.ci` files declare the same exclusive group `sflow-agent-address-topology` per `register_sflow_agent_address_linux.go:34` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An sFlow collector computes wrong per-interval packet deltas when `sample_pool` crosses `2^32` or when sampling rate changes; no routing, BGP, or counter-export path is affected. |
| How is it reverted? | Single commit revert of `flow_adapter.go` and its tests/docs. |
| Who else touches this path? | `internal/plugins/flowexport/sflow/` has no other open spec editing `flow_adapter.go`. `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md` owns only the BMP residuals (R-1..R-4) after this split. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Sampled Ethernet frame handed to `FlowEncoder.EncodeFlowSample` | → | Per-source `pool += sample.Rate` (uint32 wrap) -> `writeFlowSample` -> UDP datagram | `TestSFlowFlowSamplePoolTracksTotal`, `TestSFlowSamplePoolPerSource`, and `TestFlowExportSFlowToolSamplePoolInterop` |
| Daemon startup with `flow-export` sFlow collector on multi-homed veth topology | → | `adviseSFlowCollectors` -> `classifyAgentAddress` -> stderr WARN (routed) / silent (loopback) + UDP counter datagram | `test/flow-export/sflow-agent-address-multihomed-routed.ci` and `test/flow-export/sflow-agent-address-multihomed-loopback.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Single sFlow source (`ifIndex` 7), `Rate: 1024`, three consecutive samples via `EncodeFlowSample` | Decoded on-wire `sample_pool` values are `1024`, `2048`, `3072` (growth by rate per sample below `2^32`, unchanged) | None: every sample below `2^32` must match exact accumulated rate |
| AC-2 | Single sFlow source (`ifIndex` 7), `Rate: 2147483648` (`2^31`), three consecutive samples via `EncodeFlowSample` | Decoded on-wire `sample_pool` values are `2147483648`, `0`, `2147483648` (wraps modulo `2^32`; per-sample modular delta equals `Rate`; never clamps to `4294967295`) | A sample whose modular sum legitimately equals `2^32 - 1` before wrapping on the next sample is permitted by sFlow v5 Section 5 ("An available counter may temporarily have the max value just before it rolls to zero. This is permitted."); permanent saturation is forbidden |
| AC-3 | Two sFlow sources (`ifIndex` 7 and `ifIndex` 9) on one `FlowEncoder`, including a rate change on `ifIndex` 7 (e.g., `Rate: 100`, then `Rate: 250`) and a fresh `FlowEncoder` reset | Each source advances `sequence_number` and `sample_pool` independently (`ifIndex` 7 produces `100`, `350`, never `2*250 = 500`; `ifIndex` 9 advances on its own accumulator); a fresh `FlowEncoder` resets both `sequence_number` (to 1) and `sample_pool` (to `Rate`) together | None: per-source accumulation and coupled reset are deterministic |
| AC-4 | Independent `sflowtool` v6.11 collector container (`TestFlowExportSFlowToolSamplePoolInterop` in `collectors_docker_test.go`, tag `docker`): source 1 sends three samples at constant rate (e.g., `64`), source 2 sends three samples at `Rate: 2147483648` (`2^31`) | `sflowtool` decodes exact JSON `"samplePool"` strings `"64"`, `"128"`, `"192"` on source 1 (consecutive deltas equal `64`) and `"2147483648"`, `"0"`, `"2147483648"` on source 2, with `"4294967295"` absent, while `TestFlowExportSFlowToolInterop` remains green unchanged | None: `sflowtool` prints the exact 32-bit unsigned field value; restoring saturation in `EncodeFlowSample` must turn `TestFlowExportSFlowToolSamplePoolInterop` red (discrimination proof) |
| AC-5 | `SFLOW-V5-x-11` discrimination record and independent audit verdict after AC-1..AC-4 land | `./le rfc record` records a breaking-mutation discrimination for `TestSFlowFlowSamplePoolTracksTotal`, an independent agent re-judges `rfc/audit/sflow-v5/SFLOW-V5-x-11.json` to `enforced` via `./le rfc audit-stamp ... mode rejudge`, and `./le rfc check` passes with `rfc/short/sflow-v5.md`'s `SFLOW-V5-x-11` annotation updated to describe wrapping accumulation | None: `weak` verdict may not survive closure |
| AC-6 | Linux daemon run with two routed veth interfaces (`sfproof0` `192.0.2.41/32`, `sfproof1` `198.51.100.41/32`) via `test/flow-export/sflow-agent-address-multihomed-routed.ci` (`agent-address 192.0.2.41`) and `test/flow-export/sflow-agent-address-multihomed-loopback.ci` (`agent-address 127.0.0.1`) | Both runs receive a verified sFlow v5 counter datagram for `sfproof0` carrying the configured `agent-address`; the routed `.ci` asserts the `is on routed interface sfproof0, not on a loopback interface` warning on stderr, and the loopback `.ci` asserts that warning is absent | None: both `.ci` cases run under `option=needs-linux` and `caps=CAP_NET_ADMIN` and close the 2026-10-30 `claim-outlives-the-evidence-it-cites.md` journal row |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs sFlow packet sampling past `2^32` represented packets and reads per-interval deltas in an sFlow collector | `FlowEncoder.EncodeFlowSample` -> `writeFlowSample` -> `Sender.Send` -> UDP -> `sflowtool` | `TestSFlowFlowSamplePoolTracksTotal`, `TestFlowExportSFlowToolSamplePoolInterop` |
| 2 | Changes sampling rate on one interface while sampling multiple interfaces | `FlowEncoder.EncodeFlowSample` per-`ifIndex` `{seq, pool}` accumulator -> `writeFlowSample` | `TestSFlowSamplePoolPerSource` |
| 3 | Configures sFlow `agent-address` on a multi-homed Linux router using either a routed interface IP or a loopback IP | `ze -` config apply -> `adviseSFlowCollectors` -> stderr WARN or silence + sFlow v5 UDP counter export | `test/flow-export/sflow-agent-address-multihomed-routed.ci`, `test/flow-export/sflow-agent-address-multihomed-loopback.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSFlowFlowSamplePoolTracksTotal` (CHANGED, RFC-tagged `SFLOW-V5-x-11 positive`) | `internal/plugins/flowexport/sflow/sflow_v5_flow_adapter_test.go` | AC-1 (`grows by rate per sample`) and AC-2 (`wraps modulo 2^32`: rate `2^31`, three samples -> `2147483648`, `0`, `2147483648`, never `MaxUint32`); tag prose rewritten to state Ze's per-source accumulated rate estimate and sFlow v5 Section 5 wrap vs sentinel rule. Requires independent `ze-test-judge` approval (`./le rfc approve unit sflow.TestSFlowFlowSamplePoolTracksTotal ...`) | |
| `TestSFlowSamplePoolPerSource` (NEW, untagged) | `internal/plugins/flowexport/sflow/sflow_v5_flow_adapter_test.go` | AC-3: two sources (`ifIndex` 7 and 9) advance independently; rate change on `ifIndex` 7 accumulates by each sample's rate rather than `seq * currentRate`; fresh `FlowEncoder` resets both `sequence_number` and `sample_pool` together | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `sample_pool` (`uint32`) | wraps modulo `2^32` | `2^32 - 1` (`4294967295`), next sample wraps to `(2^32 - 1 + rate) mod 2^32`; tested across the boundary at `Rate = 2^31` (`2147483648`, `0`, `2147483648`) and near `MaxUint32` (`MaxUint32 - 10`, `+20` -> `9`) | N/A (unsigned 32-bit modular counter) | N/A (wraps modulo `2^32`) |
| sFlow `sampling rate` (YANG config) | `1..1000000` | `1000000` | `0` (existing YANG range refusal) | `1000001` (existing YANG range refusal) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `sflow-agent-address-multihomed-routed.ci` (NEW, `option=needs-linux`, `caps=CAP_NET_ADMIN`, exclusive group `sflow-agent-address-topology`) | `test/flow-export/sflow-agent-address-multihomed-routed.ci` | AC-6: multi-homed veth router with `agent-address 192.0.2.41` (on `sfproof0`) emits the routed-interface warning on stderr and still exports valid sFlow v5 counter datagrams | |
| `sflow-agent-address-multihomed-loopback.ci` (NEW, `option=needs-linux`, `caps=CAP_NET_ADMIN`, exclusive group `sflow-agent-address-topology`) | `test/flow-export/sflow-agent-address-multihomed-loopback.ci` | AC-6: multi-homed veth router with `agent-address 127.0.0.1` (on `lo`) emits no multi-homed agent-address warning on stderr and exports valid sFlow v5 counter datagrams | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `TestFlowExportSFlowToolSamplePoolInterop` (NEW sibling beside `TestFlowExportSFlowToolInterop`, owner decision Q-8) | `internal/plugins/flowexport/collectors_docker_test.go` (build tag `docker`; image `test/interop-flowexport/Dockerfile.collectors`, run via `./le test integration docker-peers` or `go test -tags docker`) | `sflowtool` v6.11 | AC-4: independent `sflowtool` decodes per-source `samplePool` progression (`64`, `128`, `192`) on source 1 and `2^32` wrap (`2147483648`, `0`, `2147483648`, never `4294967295`) on source 2; restoring saturation in `EncodeFlowSample` turns the test red | |

## Files to Modify
- `internal/plugins/flowexport/sflow/flow_adapter.go` - replace `seqNums map[uint32]uint32` with per-source `{seq, pool uint32}` map; accumulate `pool += sample.Rate` wrapping modulo `2^32` and replace the saturation comment with the sFlow v5 Section 5 wrap/reset comment
- `internal/plugins/flowexport/sflow/sflow_v5_flow_adapter_test.go` - update `TestSFlowFlowSamplePoolTracksTotal` (invert saturation subtest to wrap modulo `2^32` and update `SFLOW-V5-x-11` tag prose under `ze-test-judge` approval); add `TestSFlowSamplePoolPerSource`
- `internal/plugins/flowexport/collectors_docker_test.go` - add sibling `TestFlowExportSFlowToolSamplePoolInterop` beside unchanged `TestFlowExportSFlowToolInterop`
- `docs/architecture/flowexport/flow-export-2-flow-records.md` - document per-source wrapping `sample_pool` and `TestFlowExportSFlowToolSamplePoolInterop`
- `docs/architecture/flowexport/flow-export-1-counter-export.md` - add `TestFlowExportSFlowToolSamplePoolInterop` under "Independent collector tests" and replace the "were never committed" prose under "Daemon agent-address warning tests" with source anchors to the two new `.ci` files
- `docs/guide/flow-export.md` - document under Sampling that sFlow `sample_pool` is a per-source accumulated rate estimate wrapping modulo `2^32`
- `docs/architecture/testing/interop.md` - add `TestFlowExportSFlowToolSamplePoolInterop` to the `collectors_docker_test.go` source anchor
- `rfc/short/sflow-v5.md` - update the `{single-polarity: ...}` annotation on `SFLOW-V5-x-11` to describe per-source wrapping accumulation (row quote and MUST level unchanged)
- `rfc/audit/sflow-v5/SFLOW-V5-x-11.json` - updated by `./le rfc audit-stamp ... mode rejudge` only (never by hand)
- `rfc/discrimination/sflow-v5/SFLOW-V5-x-11.json` - updated by `./le rfc record` only (never by hand)

## Files to Create
- `test/flow-export/sflow-agent-address-multihomed-routed.ci` - Linux `CAP_NET_ADMIN` functional test proving the multi-homed routed `agent-address` warning through the real daemon
- `test/flow-export/sflow-agent-address-multihomed-loopback.ci` - Linux `CAP_NET_ADMIN` functional test proving a loopback `agent-address` on a multi-homed router emits no warning

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config or RPC surface changes |
| YANG validation constraints | No | Existing `rate` `1..1000000` range unchanged |
| YANG custom validators | No | Existing `validateSFlowCollectors` unchanged |
| CLI commands/flags | No | No CLI command added or changed |
| CLI grammar (keyword before value) | N-A | No CLI command |
| Editor autocomplete | N-A | No YANG leaf |
| Functional test for new RPC/API | N-A | No new RPC; `.ci` tests added for daemon agent-address warning behavior |
| Pipe completeness | N-A | No CLI output changed |
| Env var registration | No | No env var added |
| Doctor check for runtime dependencies | No | No new runtime dependency |
| Doctor code per problem class | No | Existing agent-address advice warning unchanged; `sample_pool` wrap produces no error or warning |
| Prometheus counters/metrics | No | No metric added or changed |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP feature |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Correctness fix on existing sFlow export; note `features/flow-export.md` defect review row can be refreshed at closure |
| 2 | Config syntax changed? | No | No config leaf changed |
| 3 | CLI command added/changed? | No | None |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | Existing `flowexport` plugin behavior repair |
| 6 | Has a user guide page? | Yes | `docs/guide/flow-export.md` (note wrapping per-source `sample_pool` under Sampling) |
| 7 | Wire format changed? | No | sFlow v5 XDR layout unchanged; `sample_pool` value semantics wrap instead of saturating |
| 8 | Plugin SDK/protocol changed? | No | No SDK or RPC change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/sflow-v5.md` (`SFLOW-V5-x-11` annotation), `rfc/audit/sflow-v5/SFLOW-V5-x-11.json` (via `./le rfc audit-stamp`), `rfc/discrimination/sflow-v5/SFLOW-V5-x-11.json` (via `./le rfc record`) |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/flowexport/flow-export-1-counter-export.md` (add `TestFlowExportSFlowToolSamplePoolInterop` and the two committed `.ci` paths), `docs/architecture/testing/interop.md` (anchor update) |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/flowexport/flow-export-2-flow-records.md` (per-source wrapping `sample_pool` and sibling collector test) |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Updated: `docs/architecture/flowexport/flow-export-2-flow-records.md`, `docs/architecture/flowexport/flow-export-1-counter-export.md`, `docs/architecture/testing/interop.md`. Checked via `./le spec citation anchors spec plan/release/fixes/spec-sflow-export-rfc-defects.md` and unaffected: `docs/architecture/flowexport/flow-export-0-umbrella.md` (anchors `flow_adapter.go::FlowEncoder.EncodeFlowSample` solely for `max-datagram-size` buffer capping, which is unchanged) |
| 17 | Existing docs show config/CLI/API examples for this area? | No | Config examples in `docs/guide/flow-export.md` remain valid unchanged |

## Design presentation

### What changes

`FlowEncoder.EncodeFlowSample` replaces its saturating `min(seq*rate, MaxUint32)` `sample_pool` formula with a per-source (`ifIndex`) accumulated rate estimate that wraps modulo `2^32`, created and reset alongside each source's `sequence_number`. An sFlow collector's per-interval packet delta now stays accurate past `2^32` represented packets and across sampling-rate changes instead of freezing at `4294967295` (sFlow v5's "not available" counter sentinel). The independent `sflowtool` Docker carrier gains a sibling test (`TestFlowExportSFlowToolSamplePoolInterop`) verifying multi-sample deltas and `2^32` wrap, `SFLOW-V5-x-11` is re-recorded and re-judged to `enforced`, and the two missing Linux multi-homed agent-address `.ci` tests (`sflow-agent-address-multihomed-{routed,loopback}.ci`) are committed against the existing fixture.

### Before and after

1. **sFlow flow sample pool crosses `2^32` (three samples at `Rate: 2147483648` on one interface):**
   - Before: `EncodeFlowSample` writes `sample_pool` values `2147483648`, `4294967295`, `4294967295`. Once saturated, `4294967295` tells collectors the counter is unavailable and every subsequent interval delta is `0`.
   - After: `EncodeFlowSample` writes `sample_pool` values `2147483648`, `0`, `2147483648`. Every consecutive modular delta equals `2147483648` and `sflowtool` decodes `"2147483648"`, `"0"`, `"2147483648"`.
2. **Sampling rate changes on an active source (`ifIndex` 7: sample 1 at `Rate: 100`, sample 2 at `Rate: 250`):**
   - Before: `seq * sample.Rate` writes `100`, then `500` (retroactively rewriting earlier samples at the new rate).
   - After: per-source accumulation writes `100`, then `350` (`100 + 250`), matching the actual accumulated estimate.
3. **Daemon-level proof of sFlow v5 Section 4.3 multi-homed agent-address warning:**
   - Before: `register_sflow_agent_address_linux.go` existed with no `.ci` callers, leaving the 2026-10-30 journal row in `plan/journal/claim-outlives-the-evidence-it-cites.md` open.
   - After: `test/flow-export/sflow-agent-address-multihomed-routed.ci` proves the daemon emits the routed-interface warning and exports counters, and `test/flow-export/sflow-agent-address-multihomed-loopback.ci` proves a loopback address emits no warning.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep saturating `sample_pool` at `2^32 - 1` | Violates sFlow v5 Section 5 counter conventions (`MaxUint32` means "counter not available") and freezes collector deltas at zero once reached; explicitly rejected by the 2026-11-24 owner decision |
| Compute `(seq * rate) mod 2^32` instead of accumulating per source | Produces wrong deltas if `sample.Rate` changes on a source; explicitly set aside by the 2026-11-24 owner decision ("per-source accumulated rate estimate wrapping modulo `2^32`") |
| Read actual kernel packet counts for `sample_pool` | `FlowSample` from Linux `psample` does not carry per-packet skipped-packet counts; Thomas decided on 2026-11-24 to use the per-source accumulated rate estimate |
| Create a new BGP-lab interop scenario `sflow-sample-pool-sflowtool` | `collectors_docker_test.go` already builds and runs pinned `sflowtool` v6.11 with direct `FlowSample.Rate` control (able to cross `2^32` in three samples); Thomas decided on 2026-10-14 (Q-8) to extend the existing carrier with a sibling test |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring and unit tests (MANDATORY FIRST)** -- write failing unit tests for per-source wrapping accumulation
   - Tests: `TestSFlowFlowSamplePoolTracksTotal` (update wrap subtest and `SFLOW-V5-x-11` tag prose after `ze-test-judge` `./le rfc approve`), `TestSFlowSamplePoolPerSource`
   - Files: `internal/plugins/flowexport/sflow/sflow_v5_flow_adapter_test.go`
   - Verify: run `./le test unit pkg path ./internal/plugins/flowexport/sflow/` and confirm the new wrap and rate-change assertions fail against current saturating `EncodeFlowSample`
2. **Phase: Producer fix** -- per-source wrapping `sample_pool` in `FlowEncoder`
   - Tests: `TestSFlowFlowSamplePoolTracksTotal`, `TestSFlowSamplePoolPerSource`, and existing `SFLOW-V5-x-28` tests in `sflow_v5_test.go`
   - Files: `internal/plugins/flowexport/sflow/flow_adapter.go`
   - Verify: `./le test unit pkg path ./internal/plugins/flowexport/sflow/` passes under `-race`
3. **Phase: Independent collector interop (`sflowtool`)** -- add sibling test and prove discrimination
   - Tests: `TestFlowExportSFlowToolSamplePoolInterop` (beside unchanged `TestFlowExportSFlowToolInterop`)
   - Files: `internal/plugins/flowexport/collectors_docker_test.go`
   - Verify: run `go test -tags docker -run TestFlowExportSFlowTool ./internal/plugins/flowexport/` via `./le job run`; prove red with saturation restored, then green with wrap restored
4. **Phase: Multi-homed agent-address `.ci` tests** -- close the 2026-10-30 journal row
   - Tests: `test/flow-export/sflow-agent-address-multihomed-routed.ci`, `test/flow-export/sflow-agent-address-multihomed-loopback.ci`
   - Files: both new `.ci` files
   - Verify: run both Linux `CAP_NET_ADMIN` `.ci` cases on the Linux/QEMU runner and verify green
5. **Phase: RFC records, verdict re-judge, and docs** -- record discrimination, re-judge `SFLOW-V5-x-11`, and update docs
   - Files: `rfc/short/sflow-v5.md`, `rfc/discrimination/sflow-v5/SFLOW-V5-x-11.json` (tool-written), `rfc/audit/sflow-v5/SFLOW-V5-x-11.json` (independent re-judge), `docs/architecture/flowexport/flow-export-2-flow-records.md`, `docs/architecture/flowexport/flow-export-1-counter-export.md`, `docs/guide/flow-export.md`, `docs/architecture/testing/interop.md`
   - Verify: `./le rfc check` and `./le doc check all` pass

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-6 has an implementation and test at file:line |
| Correctness | `sample_pool` wraps modulo `2^32` per source (`ifIndex`) via `uint32` addition of `sample.Rate`, never saturating at `MaxUint32`; `seq` and `pool` live in one per-source struct so they reset together |
| Tag honesty | `SFLOW-V5-x-11` tag prose states Ze's per-source accumulated rate estimate as Ze's choice (not as raw kernel packet count) and cites sFlow v5 Section 5 wrap vs max-value sentinel; does not call a natural wrap a `sequence_number` reset |
| Rule: `ai/rules/stale-comments.md` | The old saturation justification comment in `flow_adapter.go` is replaced, and the "were never committed" paragraph in `flow-export-1-counter-export.md` is replaced |
| Rule: `ai/rules/testing.md` | Both new `.ci` files carry `option=needs-linux`, `caps=CAP_NET_ADMIN`, and exclusive group `sflow-agent-address-topology`; no sleeps or weakened assertions |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Per-source wrapping `sample_pool` unit tests | `./le test unit pkg path ./internal/plugins/flowexport/sflow/` |
| Independent `sflowtool` sample-pool and wrap interop | `./le job run label sflow-docker command go test -v -tags docker -run TestFlowExportSFlowTool ./internal/plugins/flowexport/` |
| Multi-homed agent-address `.ci` tests | `./le test flow-export` (on Linux with `CAP_NET_ADMIN` / QEMU) selecting `sflow-agent-address-multihomed-routed` and `sflow-agent-address-multihomed-loopback` |
| `SFLOW-V5-x-11` discrimination record and `enforced` verdict | `./le rfc check` |
| Documentation anchors valid | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| Input validation | `sample.Rate` is a `uint32` (config YANG constrains `1..1000000`); `uint32` modular addition cannot overflow or panic for any `uint32` input |
| Resource exhaustion | Per-source map is keyed by kernel interface `ifIndex` (bounded by host interfaces), matching the existing `seqNums` map |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| `sflowtool` JSON output differs from assumption A-1 | Inspect `sflowtool` stderr/stdout diagnostics and update parser/assertion without weakening what is proven |
| Multi-homed `.ci` fails on kernel/namespace setup | Check `register_sflow_agent_address_linux.go` and QEMU runner privileges |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- Splitting sFlow D2 out of `spec-bmp-sflow-export-rfc-defects.md` keeps each spec on its own subsystem door (`system-plugins.md` + `le-tooling.md` here vs `bgp-engine.md` + `bgp-plugins.md` + `plugin-infra.md` + `le-tooling.md` on BMP), honoring `ai/META-DECISIONS.md`.
- Storing `seq` and `pool` together in one per-source struct guarantees `SFLOW-V5-x-28` ("If the agent resets the sample_pool then it must also reset the sequence_number.") by construction.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `claim-outlives-the-evidence-it-cites.md` | 2026-10-30 | flow export, `flow-export-1-counter-export.md` (two missing multi-homed sFlow agent-address `.ci` cases) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-source `{seq, pool uint32}` accumulator wrapping modulo `2^32` (owner decision 2026-11-24) | Saturate at `MaxUint32`; `(seq * rate) mod 2^32`; kernel packet counters | sFlow v5 Section 5 makes `MaxUint32` the "not available" sentinel and permits wrap to zero; accumulating per sample handles rate changes cleanly and keeps `seq` and `pool` coupled |
| Sibling `TestFlowExportSFlowToolSamplePoolInterop` in `collectors_docker_test.go` (owner decision Q-8, 2026-10-14) | New BGP-lab interop scenario `sflow-sample-pool-sflowtool`; mutating `TestFlowExportSFlowToolInterop` | Reuses pinned `sflowtool` v6.11 via `labtest`, sets `FlowSample.Rate = 2^31` directly to cross `2^32` in three samples, and leaves existing `TestFlowExportSFlowToolInterop` assertions untouched |

## Known Limitations
- `sample_pool` remains an accumulated rate estimate rather than an exact hardware/kernel skipped-packet count because Linux `psample` does not report per-sample skipped packet totals (owner decision 2026-11-24).
- Remaining BMP RFC 7854 defects (R-1..R-4) stay in `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md`.

## RFC Documentation (Scope: protocol)

Add `// sFlow v5 Section 5: ...` comments above the per-source `sample_pool` accumulation in `internal/plugins/flowexport/sflow/flow_adapter.go` quoting the `sample_pool` definition, the `sequence_number` reset note, and the Section 5 counter wrap vs `MaxUint32` unavailable-sentinel rule.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| sFlow v5 Section 5 (`flow_sample`, `sample_pool`, Counter conventions) | With `flow-export { collector <name> { protocol sflow; ... } sampling { interface <if> { rate <N>; } } }`, `EncodeFlowSample` writes a per-source accumulated `sample_pool` that advances by `Rate` and wraps modulo `2^32` rather than saturating at `MaxUint32` | `SFLOW-V5-x-28` coupled reset of `sequence_number` and `sample_pool` on fresh encoder; expanded `flow_sample_expanded` (format `0:3`) encoding for all interfaces | `TestSFlowFlowSamplePoolTracksTotal`, `TestSFlowSamplePoolPerSource`, `TestFlowExportSFlowToolSamplePoolInterop` | `sflow_v5_test.go` (`SFLOW-V5-x-28` tests), `TestFlowExportSFlowToolInterop` |
| sFlow v5 Section 4.3 (`sFlowAgentAddress`) | On a multi-homed Linux router, configuring `agent-address` on a routed interface logs a warning recommending a loopback address and still exports datagrams; configuring a loopback address logs no warning | Dummy interfaces count as loopback; config is never refused by the advisor | `test/flow-export/sflow-agent-address-multihomed-routed.ci`, `test/flow-export/sflow-agent-address-multihomed-loopback.ci` | `test/flow-export/sflow-agent-address-warning.ci`, `sflow_v5_agent_advice_test.go` |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions with positive outcomes and permitted survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist answered Yes / No / N-A with reasons

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/plugins/flowexport/sflow/`, the `docker`-tagged `TestFlowExportSFlowTool` run in `./internal/plugins/flowexport/`, and both multi-homed `.ci` tests on Linux with `CAP_NET_ADMIN`. GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/plugins/flowexport/sflow/flow_adapter.go`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Reciprocal split item (BMP R-1..R-4) named in `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + journal row status update
- [ ] **Commit B:** `remove plan/release/fixes/spec-sflow-export-rfc-defects.md` only, in the same `./le commit create` script
