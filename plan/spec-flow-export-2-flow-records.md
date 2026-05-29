# Spec: Flow Export - Packet Sampling and Flow Records

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-flow-export-1-counter-export |
| Phase | - |
| Updated | 2026-05-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-flow-export-0-umbrella.md` - umbrella architecture
4. `rfc/short/sflow-v5.md` - sFlow v5 flow sample format (especially "Flow Sample" and "What Packet Sampling Would Require")
5. `rfc/short/rfc3954.md` - NetFlow v9 field types for per-flow records
6. `rfc/short/rfc7011.md` - IPFIX protocol
7. `rfc/short/rfc7012.md` - IPFIX IEs (5-tuple, counters, timestamps)
8. `internal/plugins/iface/netlink/mirror_linux.go` - existing tc clsact pattern
9. `internal/component/cmd/show/capture_interface_linux.go` - existing AF_PACKET + BPF pattern

## Task

Implement packet sampling via `tc sample` + psample netlink, and per-flow record
export via conntrack accounting. This is spec 2 of the `spec-flow-export` set.
It builds on the shared infrastructure from spec 1 (buffer pool, UDP sender,
component lifecycle) and adds:

1. **Packet sampling:** kernel-side 1-in-N sampling via tc sample action, userspace
   reception via psample generic netlink multicast group
2. **sFlow flow samples:** sampled packet headers + forwarding metadata encoded as
   sFlow v5 flow_sample records
3. **NetFlow v9 / IPFIX flow records:** per-flow 5-tuple records sourced from
   conntrack with accounting (already enabled by Ze's conntrack config)
4. **BGP enrichment:** source/destination AS and next-hop from Ze's RIB

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component boundaries, bus topics
  → Constraint: components communicate via bus or explicit function call, not shared memory
- [ ] `ai/patterns/registration.md` - registration pattern for new capabilities
  → Constraint: init() in register.go, blank import in all.go
- [ ] `plan/spec-flow-export-0-umbrella.md` - umbrella decisions
  → Decision: tc sample + psample for packet sampling
  → Decision: conntrack accounting for per-flow records
  → Decision: BGP RIB enrichment via prefix-to-AS radix tree
  → Constraint: buffer-first encoding, no fmt.Sprintf on hot paths

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/sflow-v5.md` - flow_sample structure, sampled_header record, extended_gateway record
  → Constraint: flow_sample has sequence, source_id, sampling_rate, sample_pool, drops, input, output, flow_records[]
  → Constraint: sampled_header record = header_protocol + frame_length + stripped + header bytes
  → Constraint: extended_gateway record = AS path, communities, local-pref, next-hop (from BGP RIB)
- [ ] `rfc/short/rfc3954.md` - NetFlow v9 per-flow field types
  → Constraint: 5-tuple fields: src/dst IP (8,12), src/dst port (7,11), protocol (4)
  → Constraint: counters: IN_BYTES (1), IN_PKTS (2), per-flow
  → Constraint: timestamps: FIRST_SWITCHED (22), LAST_SWITCHED (23) relative to sysUpTime
- [ ] `rfc/short/rfc7011.md` - IPFIX flow record encoding
  → Constraint: same template mechanism as counter export; different template ID for flow records
- [ ] `rfc/short/rfc7012.md` - IPFIX IEs for flow fields
  → Constraint: IE 8/12 = src/dst IPv4, IE 27/28 = src/dst IPv6, IE 7/11 = ports
  → Constraint: IE 16/17 = bgpSourceAsNumber/bgpDestinationAsNumber
  → Constraint: IE 18 = bgpNextHopIPv4Address
  → Constraint: IE 152/153 = flowStartMilliseconds/flowEndMilliseconds

**Key insights:**
- `vishvananda/netlink` already has `SampleAction{Rate, Group, TruncSize}` (filter.go:401)
- `mdlayher/genetlink` already in go.sum for psample netlink reception
- Ze's mirror_linux.go shows the exact tc clsact + MatchAll filter pattern to extend
- Ze already enables `nf_conntrack_acct` (conntrack.go:163), providing per-flow byte/packet counters
- Conntrack entries give: src/dst IP, src/dst port, protocol, bytes, packets, mark, timeout
- psample netlink delivers: ifIndex, sampling rate, truncated packet header, original packet length

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/netlink/mirror_linux.go` - SetupMirror uses clsact qdisc + MatchAll filter + MirredAction; RemoveMirror deletes the qdisc
  → Constraint: clsact qdisc may already exist if mirroring is active; must not conflict
  → Constraint: filter priority determines order; mirror uses priority 1
- [ ] `internal/component/cmd/show/capture_interface_linux.go` - AF_PACKET via mdlayher/packet, BPF filter compilation
  → Constraint: existing capture is for CLI display, not daemon-mode continuous operation
- [ ] `internal/component/config/system/conntrack.go` - ConntrackConfig with Accounting bool; when true, sets nf_conntrack_acct=1
  → Constraint: accounting must be enabled for per-flow byte/packet counters; if disabled, flow records have no counters
- [ ] `internal/component/cmd/show/conntrack.go` - reads /proc/net/nf_conntrack for display
  → Constraint: show command reads conntrack on demand; flow export needs periodic or event-driven reads
- [ ] `vendor/github.com/vishvananda/netlink/filter.go:401` - SampleAction{Group uint32, Rate uint32, TruncSize uint32}
  → Constraint: Rate = 1-in-N (N packets per sample); Group = psample group ID; TruncSize = header bytes to capture
- [ ] `vendor/github.com/vishvananda/netlink/filter_linux.go:771` - SampleAction encoding: TCA_ACT_SAMPLE_RATE, TCA_ACT_SAMPLE_PSAMPLE_GROUP, TCA_ACT_SAMPLE_TRUNC_SIZE
  → Constraint: fully supported in existing vendored netlink library

**Behavior to preserve:**
- Existing tc mirred mirroring (mirror_linux.go) must not break when sampling is added
- Conntrack config and show commands unaffected
- Packet capture CLI (`show capture interface`) remains independent
- sFlow counter samples from spec 1 continue in parallel with flow samples

**Behavior to change:**
- Add tc sample action on configured interfaces (coexists with mirror via different filter priority)
- Add psample netlink reader goroutine for receiving sampled packet metadata
- Add conntrack dump/event reader for per-flow records
- Add sFlow flow_sample encoding to the sflow encoder
- Add NetFlow v9/IPFIX per-flow template and data FlowSet encoding
- Add BGP RIB subscription for AS/next-hop enrichment

## Data Flow (MANDATORY)

### Entry Point - Packet Sampling
- Config: YANG `flow-export { sampling { interface <name>; rate <N>; } }`
- Kernel: tc sample action on clsact qdisc, delivers to psample group
- Userspace: genetlink socket subscribes to PSAMPLE_NL_MCGRP_SAMPLE multicast group

### Entry Point - Conntrack Flow Records
- Config: YANG `flow-export { netflow9/ipfix { ... } }` (spec 1 already creates the exporter)
- Kernel: conntrack accounting enabled (existing Ze config)
- Userspace: periodic conntrack dump via netlink NFNL_SUBSYS_CTNETLINK, plus destroy events

### Transformation Path - Packet Sampling
1. Packet arrives at interface, kernel forwards normally
2. tc sample action fires for 1-in-N packets: copies first `trunc_size` bytes to psample
3. psample kernel module sends generic netlink message to multicast group
4. Ze's psample reader goroutine receives: {ifindex, rate, orig_size, header_bytes}
5. sFlow encoder wraps as flow_sample + sampled_header record + extended records
6. Datagram sent to sFlow collector via shared sender (spec 1)

### Transformation Path - Conntrack Flow Records
1. Ze periodically dumps conntrack table via netlink (interval = active-timeout)
2. For each entry: extract 5-tuple, byte/packet counters, timestamps
3. Compute delta since last export (track per-conntrack-ID state)
4. On conntrack destroy event: emit final flow record immediately
5. NetFlow v9/IPFIX encoder writes per-flow data records into template
6. Datagram sent to collector via shared sender (spec 1)

### Transformation Path - BGP Enrichment
1. Subscribe to BGP RIB updates via bus topic (or direct callback)
2. Build prefix-to-AS radix tree from best-path announcements
3. On flow record/sample export: longest-prefix-match src/dst IP in tree
4. Populate bgpSourceAsNumber, bgpDestinationAsNumber, bgpNextHopIPv4Address

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| kernel tc -> psample -> Ze | generic netlink multicast message (value copy of header bytes) | [ ] |
| kernel conntrack -> Ze | netlink NFNL_SUBSYS_CTNETLINK dump + multicast destroy events | [ ] |
| BGP RIB -> flowexport | atomic.Pointer swap of radix tree (lock-free reads) | [ ] |
| flowexport -> network | UDP sendto with pre-encoded buffer (spec 1 sender) | [ ] |

### Integration Points
- `iface/netlink/mirror_linux.go` - extend tc setup pattern (same clsact qdisc, different priority)
- `flowexport/sender.go` (spec 1) - shared UDP sender and buffer pool
- `flowexport/sflow/encoder.go` (spec 1) - add flow_sample alongside counter_sample
- `flowexport/netflow9/` (spec 1) - add per-flow template + data encoding
- `flowexport/ipfix/` (spec 1) - add per-flow template + data encoding
- BGP RIB component - subscribe to route updates for AS path data
- Conntrack config - verify accounting is enabled; warn if not

### Architectural Verification
- [ ] No bypassed layers (tc sample is the standard kernel sampling path)
- [ ] No unintended coupling (psample reader is independent of iface monitor)
- [ ] No duplicated functionality (reuses spec 1 sender/encoder infrastructure)
- [ ] Zero-copy preserved where applicable (header bytes copied once from kernel to buffer)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG `flow-export sampling` config | -> | `sampling.SetupSampling()` | `TestSamplingConfigWiring` |
| tc sample action installed | -> | psample reader receives packet | `TestPsampleReception` |
| psample event | -> | `sflow.WriteFlowDatagram()` | `TestSFlowFlowSampleWiring` |
| conntrack dump | -> | `netflow9.WriteFlowRecord()` | `TestConntrackFlowWiring` |
| BGP route update | -> | radix tree updated | `TestBGPEnrichmentWiring` |
| `show flow-export sampling` | -> | `sampling.Status()` | `test-flow-export-sampling.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `flow-export { sampling { interface eth0; rate 2048; } }` | tc sample action installed on eth0 with rate=2048, group=1, trunc-size=128 |
| AC-2 | Packets flowing through sampled interface | psample reader receives ~1/2048 packets with header bytes and ifIndex |
| AC-3 | sFlow collector + sampling configured | flow_sample datagrams arrive with sampled_header record containing first 128 bytes of packet header |
| AC-4 | sFlow flow_sample fields | sampling_rate=2048, input=ifIndex, output=egress ifIndex (if available), sample_pool increments |
| AC-5 | BGP sessions active + sFlow sampling | extended_gateway record populated with AS path, next-hop, local-pref from RIB |
| AC-6 | NetFlow v9 collector + conntrack accounting enabled | Per-flow records with 5-tuple, byte/packet deltas, timestamps |
| AC-7 | IPFIX collector + conntrack | Per-flow records with IANA IEs (8,12,7,11,4,1,2,152,153,16,17,18) |
| AC-8 | Conntrack entry destroyed (connection closes) | Final flow record exported within 1s (not waiting for next poll) |
| AC-9 | Config reload: add/remove sampling on interface | tc sample action added/removed without affecting existing mirror rules |
| AC-10 | Interface has both mirror and sampling configured | Both coexist: mirror at priority 1, sample at priority 100 |
| AC-11 | `show flow-export sampling` | Per-interface: rate, packets sampled, packets total (sample_pool), drops |
| AC-12 | Conntrack accounting disabled in config | Warning logged; flow records export without byte/packet counters (zero values) |
| AC-13 | Prometheus metrics | `ze_flowexport_samples_total{interface}`, `ze_flowexport_flows_total`, `ze_flowexport_flows_active` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSetupSampling` | `flowexport/sampling/tc_linux_test.go` | tc clsact qdisc + MatchAll + SampleAction installed with correct rate/group/trunc | |
| `TestRemoveSampling` | `flowexport/sampling/tc_linux_test.go` | Sample filter removed without affecting mirror filters at priority 1 | |
| `TestCoexistWithMirror` | `flowexport/sampling/tc_linux_test.go` | Both mirror (prio 1) and sample (prio 100) active simultaneously | |
| `TestPsampleParse` | `flowexport/sampling/psample_linux_test.go` | Parse psample genetlink message: extract ifindex, rate, orig_size, header bytes | |
| `TestSFlowFlowSample` | `flowexport/sflow/flow_test.go` | flow_sample record encoding: sequence, source_id, rate, pool, drops, input, output, records | |
| `TestSFlowSampledHeader` | `flowexport/sflow/flow_test.go` | sampled_header record: header_protocol=1 (ethernet), frame_length, stripped=0, header bytes XDR-padded | |
| `TestSFlowExtendedGateway` | `flowexport/sflow/flow_test.go` | extended_gateway record: AS path, communities, local-pref, next-hop encoded correctly | |
| `TestConntrackDump` | `flowexport/conntrack/reader_linux_test.go` | Parse conntrack netlink dump: extract 5-tuple, bytes, packets, mark, timeout | |
| `TestConntrackDelta` | `flowexport/conntrack/reader_linux_test.go` | Delta computation: second dump subtracts first; handles counter wrap | |
| `TestConntrackDestroy` | `flowexport/conntrack/reader_linux_test.go` | Destroy event triggers immediate final record with accumulated counters | |
| `TestNetflow9FlowTemplate` | `flowexport/netflow9/flow_template_test.go` | Per-flow template with 5-tuple + counter + timestamp field types | |
| `TestNetflow9FlowData` | `flowexport/netflow9/flow_data_test.go` | Per-flow data records packed in FlowSet; count field accurate | |
| `TestIPFIXFlowTemplate` | `flowexport/ipfix/flow_template_test.go` | Per-flow template with IANA IEs for 5-tuple + BGP enrichment | |
| `TestIPFIXFlowData` | `flowexport/ipfix/flow_data_test.go` | Per-flow data records with correct IE encoding | |
| `TestRadixTreeLookup` | `flowexport/enrich/radix_test.go` | Longest-prefix-match returns correct AS number for test prefixes | |
| `TestRadixTreeAtomicSwap` | `flowexport/enrich/radix_test.go` | Concurrent reads during tree swap see consistent state (no partial tree) | |
| `TestBGPSubscription` | `flowexport/enrich/bgp_test.go` | RIB update inserts prefix-to-AS mapping; withdrawal removes it | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| sampling rate | 1-1000000 | 1000000 | 0 | 1000001 |
| trunc-size | 64-1500 | 1500 | 63 | 1501 |
| psample group | 1-2147483647 | 2147483647 | 0 | 2147483648 |
| active-timeout | 1-3600 | 3600 | 0 | 3601 |
| inactive-timeout | 1-3600 | 3600 | 0 | 3601 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-flow-export-sampling` | `test/flow-export/sampling.ci` | Configure sampling on interface, send packets, verify sFlow flow_sample datagram arrives | |
| `test-flow-export-conntrack-flow` | `test/flow-export/conntrack-flow.ci` | Establish TCP connection through Ze, verify NetFlow v9 flow record with 5-tuple and byte counts | |
| `test-flow-export-bgp-enrich` | `test/flow-export/bgp-enrich.ci` | BGP session with known prefixes, traffic matching prefix, verify AS numbers in flow record | |
| `test-flow-export-sampling-show` | `test/flow-export/sampling-show.ci` | `show flow-export sampling` returns per-interface stats | |
| `test-flow-export-coexist` | `test/flow-export/coexist.ci` | Mirror + sampling on same interface, both work independently | |
| `test-flow-export-destroy` | `test/flow-export/destroy.ci` | TCP connection closes, final flow record exported within 1s | |

### Future (if deferring any tests)
- VPP flowprobe integration (requires VPP test environment)
- sFlow extended_router record (next-hop and prefix length from FIB)
- SCTP/TCP transport for IPFIX
- IPv6 conntrack flow records (same mechanism, different address family)

## Files to Modify
- `internal/component/flowexport/exporter.go` - add sampling lifecycle, conntrack reader lifecycle
- `internal/component/flowexport/config.go` - add sampling config parsing
- `internal/component/flowexport/sflow/encoder.go` - add flow_sample datagram alongside counter datagram
- `internal/component/flowexport/netflow9/template.go` - add per-flow template (separate ID from counter template)
- `internal/component/flowexport/netflow9/data.go` - add per-flow data FlowSet encoding
- `internal/component/flowexport/ipfix/template.go` - add per-flow template
- `internal/component/flowexport/ipfix/data.go` - add per-flow data Set encoding
- `internal/yang/modules/flow-export.yang` - add sampling container

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/yang/modules/flow-export.yang` (extend) |
| CLI commands/flags | [x] | `internal/component/cmd/show/flow_export.go` (extend with sampling stats) |
| CLI grammar (action before identifier) | [x] | `show flow-export sampling detail <name>` |
| Editor autocomplete | [x] | YANG-driven |
| Functional test for new RPC/API | [x] | `test/flow-export/*.ci` |
| Doctor check for runtime dependencies | [x] | Check psample kernel module loaded, conntrack accounting enabled |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - packet sampling |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - sampling stanza |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - show flow-export sampling |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/flow-export.md` - extend with sampling section |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | Already in rfc/short/ |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` - sampling subsystem |

## Files to Create
- `internal/component/flowexport/sampling/tc_linux.go` - tc sample action setup/teardown via vishvananda/netlink
- `internal/component/flowexport/sampling/tc_other.go` - stub for non-Linux
- `internal/component/flowexport/sampling/psample_linux.go` - psample genetlink reader goroutine
- `internal/component/flowexport/sampling/psample_other.go` - stub for non-Linux
- `internal/component/flowexport/sampling/sample.go` - SampledPacket type (ifIndex, rate, origSize, header)
- `internal/component/flowexport/conntrack/reader_linux.go` - conntrack netlink dump + destroy event listener
- `internal/component/flowexport/conntrack/reader_other.go` - stub for non-Linux
- `internal/component/flowexport/conntrack/flow.go` - FlowEntry type (5-tuple, counters, timestamps)
- `internal/component/flowexport/conntrack/delta.go` - per-flow delta tracking (last-exported state)
- `internal/component/flowexport/enrich/radix.go` - prefix-to-AS radix tree with atomic swap
- `internal/component/flowexport/enrich/bgp.go` - BGP RIB subscription and tree builder
- `internal/component/flowexport/sflow/flow.go` - flow_sample + sampled_header + extended_gateway encoding
- `internal/component/flowexport/netflow9/flow_template.go` - per-flow template FlowSet
- `internal/component/flowexport/netflow9/flow_data.go` - per-flow data FlowSet
- `internal/component/flowexport/ipfix/flow_template.go` - per-flow IPFIX template Set
- `internal/component/flowexport/ipfix/flow_data.go` - per-flow IPFIX data Set
- `test/flow-export/sampling.ci`
- `test/flow-export/conntrack-flow.ci`
- `test/flow-export/bgp-enrich.ci`
- `test/flow-export/sampling-show.ci`
- `test/flow-export/coexist.ci`
- `test/flow-export/destroy.ci`

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella + spec 1 |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - tc sample setup, psample reader skeleton, wiring tests
   - Tests: `TestSamplingConfigWiring`, `TestPsampleReception`
   - Files: `sampling/tc_linux.go`, `sampling/psample_linux.go`, `sampling/sample.go`
   - Verify: tc sample action installed on test interface; psample reader starts; wiring test fails on encoding stub

2. **Phase: tc sample management** - install/remove tc sample action, coexist with mirror
   - Tests: `TestSetupSampling`, `TestRemoveSampling`, `TestCoexistWithMirror`
   - Files: `sampling/tc_linux.go`
   - Verify: tc filter visible via `tc -s filter show`; mirror rules unaffected

3. **Phase: psample reader** - generic netlink subscription, message parsing
   - Tests: `TestPsampleParse`
   - Files: `sampling/psample_linux.go`
   - Verify: receives SampledPacket{ifIndex, rate, origSize, header} from psample group

4. **Phase: sFlow flow samples** - flow_sample + sampled_header + extended records
   - Tests: `TestSFlowFlowSample`, `TestSFlowSampledHeader`, `TestSFlowExtendedGateway`
   - Files: `sflow/flow.go`
   - Verify: byte-exact match against expected sFlow v5 flow_sample datagram

5. **Phase: Conntrack reader** - netlink dump + destroy events + delta tracking
   - Tests: `TestConntrackDump`, `TestConntrackDelta`, `TestConntrackDestroy`
   - Files: `conntrack/reader_linux.go`, `conntrack/flow.go`, `conntrack/delta.go`
   - Verify: flow entries extracted with 5-tuple + counters; deltas computed correctly; destroy event triggers export

6. **Phase: NetFlow v9 per-flow** - template + data for 5-tuple flow records
   - Tests: `TestNetflow9FlowTemplate`, `TestNetflow9FlowData`
   - Files: `netflow9/flow_template.go`, `netflow9/flow_data.go`
   - Verify: template has 5-tuple + counter + timestamp fields; data records encode correctly

7. **Phase: IPFIX per-flow** - template + data with IANA IEs
   - Tests: `TestIPFIXFlowTemplate`, `TestIPFIXFlowData`
   - Files: `ipfix/flow_template.go`, `ipfix/flow_data.go`
   - Verify: template uses correct IE IDs; data records match

8. **Phase: BGP enrichment** - radix tree + RIB subscription
   - Tests: `TestRadixTreeLookup`, `TestRadixTreeAtomicSwap`, `TestBGPSubscription`
   - Files: `enrich/radix.go`, `enrich/bgp.go`
   - Verify: prefix-to-AS lookup works; tree updates atomically; concurrent reads safe

9. **Phase: Integration** - wire sampling into exporter lifecycle, extend CLI/YANG
   - Tests: functional tests
   - Files: `exporter.go`, `config.go`, `flow-export.yang`
   - Verify: end-to-end: packets in -> sFlow/NetFlow datagrams out

10. **Functional tests** - end-to-end with network namespaces
11. **RFC refs** - add `// sFlow v5 Section X`, `// RFC 3954 Section X.Y`, `// RFC 7011 Section X.Y`
12. **Full verification** - `make ze-verify`
13. **Complete spec** - audit, learned summary, closure

### Key Data Structures

#### SampledPacket (from psample)

Fields: IfIndex uint32, Rate uint32, OrigSize uint32, Header []byte (first trunc_size bytes)

#### FlowEntry (from conntrack)

Fields: SrcAddr netip.Addr, DstAddr netip.Addr, SrcPort uint16, DstPort uint16,
Protocol uint8, Bytes uint64, Packets uint64, StartTime time.Time, LastSeen time.Time, Mark uint32

#### Enrichment (from BGP RIB)

Fields: SrcAS uint32, DstAS uint32, NextHop netip.Addr, LocalPref uint32, ASPath []uint32

### tc sample + psample Architecture

```
        Interface eth0
            |
            v
    [clsact qdisc] (handle ffff:)
            |
            +-- prio 1: MatchAll + MirredAction     (existing mirror, if configured)
            |
            +-- prio 100: MatchAll + SampleAction   (NEW: rate=N, group=G, trunc=128)
                    |
                    v
            [kernel psample module]
                    |
                    v
            [generic netlink: PSAMPLE_NL_MCGRP_SAMPLE]
                    |
                    v
            Ze psample reader goroutine
                    |
                    +-- sFlow: flow_sample{sampled_header, extended_gateway}
```

psample netlink message attributes (PSAMPLE_ATTR_*):
- IIFINDEX (u16): input interface index
- OIFINDEX (u16): output interface index (0 if unknown)
- ORIGSIZE (u32): original packet size
- SAMPLE_GROUP (u32): psample group (matches tc sample action)
- GROUP_SEQ (u32): per-group sequence number
- SAMPLE_RATE (u32): sampling rate (1-in-N)
- DATA (binary): truncated packet header bytes

### Conntrack Flow Export Architecture

```
    [kernel conntrack table] (nf_conntrack_acct=1)
            |
            +-- periodic dump: NFNL_SUBSYS_CTNETLINK + IPCTNL_MSG_CT_GET
            |       every active-timeout seconds
            |
            +-- destroy events: NFNLGRP_CONNTRACK_DESTROY multicast
                    immediate notification when flow ends
            |
            v
    Ze conntrack reader goroutine
            |
            +-- delta tracker (map[conntrackID]lastExported)
            |
            +-- on destroy: emit final record, remove from tracker
            |
            v
    NetFlow v9 / IPFIX encoder -> BGP enrichment -> UDP sender -> collector
```

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-13 has implementation with file:line |
| Correctness | tc sample priority (100) does not conflict with mirror (1); psample attributes parsed correctly; sFlow XDR alignment |
| Naming | Types: SampledPacket, FlowEntry, not generic "Event" or "Record" |
| Data flow | No pointers shared across goroutines; radix tree swapped atomically; psample channel bounded |
| CLI grammar | `show flow-export sampling detail <name>` |
| Doctor checks | psample module loaded, conntrack accounting enabled |
| Rule: buffer-first | sampled_header bytes copied directly into datagram buffer at offset; no intermediate allocation |
| Rule: no-sprintf | No fmt.Sprintf in per-sample or per-flow encoding paths |
| Rule: no-cross-boundary-pointers | SampledPacket and FlowEntry are value types or owned copies |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| tc sample setup works | `go test ./internal/component/flowexport/sampling/... -v` |
| psample reception works | `go test ./internal/component/flowexport/sampling/... -run TestPsample -v` |
| sFlow flow samples encode | `go test ./internal/component/flowexport/sflow/... -run TestFlow -v` |
| Conntrack reader works | `go test ./internal/component/flowexport/conntrack/... -v` |
| NetFlow v9 per-flow records | `go test ./internal/component/flowexport/netflow9/... -run TestFlow -v` |
| IPFIX per-flow records | `go test ./internal/component/flowexport/ipfix/... -run TestFlow -v` |
| BGP enrichment works | `go test ./internal/component/flowexport/enrich/... -v` |
| Mirror + sampling coexist | `test/flow-export/coexist.ci` passes |
| End-to-end sampling | `test/flow-export/sampling.ci` passes |
| Prometheus metrics registered | `grep -rn 'ze_flowexport_samples\|ze_flowexport_flows' internal/component/flowexport/` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | psample messages from kernel are trusted; but validate attribute lengths before slice access |
| Privilege | tc sample and psample require CAP_NET_ADMIN; ze doctor must check |
| Resource exhaustion | Bounded psample channel (drop oldest on overflow); bounded conntrack state map (cap at max_entries) |
| Information disclosure | Sampled packet headers may contain sensitive payload; sFlow datagrams are unencrypted UDP |
| Conntrack state size | Delta tracker map grows with active flows; GC entries on destroy event or after 2x active-timeout |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| psample not receiving | Check: is psample module loaded? is tc sample action installed? `tc -s filter show dev <iface>` |
| Conntrack dump empty | Check: is nf_conntrack_acct=1? are there active connections? |
| Mirror broken after sampling | Check: filter priorities; sampling must be higher number (lower priority) than mirror |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- tc sample action already in vishvananda/netlink: zero new dependencies for the tc side
- mdlayher/genetlink already in go.sum: zero new dependencies for the psample reader
- Ze's existing mirror_linux.go is the exact pattern to follow (same qdisc, different action)
- VyOS abandoned pmacct (userspace flow tracking) for performance; moved to kernel-side (ipt-netflow)
- Ze's approach is kernel-side for both sampling (tc sample) and flow tracking (conntrack)
- The radix tree for BGP enrichment must be built incrementally from RIB updates,
  not by dumping the full RIB periodically

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
Specific constraints:
- sFlow v5: "sampling_rate ... is the ratio of packets observed to samples generated"
- sFlow v5: "sample_pool ... total number of packets that could have been sampled"
- RFC 3954 Section 9: "FIRST_SWITCHED ... sysUpTime in milliseconds at which the first packet of this Flow was switched"
- RFC 7011 Section 3.1: "Sequence Number ... cumulative number of IPFIX Data Records"

## Implementation Summary

### What Was Implemented
- Integration layer connecting the spec-2 primitives to the Exporter (none existed before): neutral `FlowSample`/`ConntrackFlow` value types and `FlowSampleEncoder`/`FlowRecordEncoder` interfaces (flowtypes.go), factory registration in sflow/netflow9/ipfix (flow_adapter.go + register.go).
- `samplingWorker`: installs tc sample on configured interfaces and runs a long-lived psample read loop dispatching sFlow flow samples (sampling_worker.go), delegating OS specifics to the sampling package (_linux/_other).
- `conntrackWorker`: periodic conntrack dump + per-flow delta + dispatch to NetFlow v9 / IPFIX flow encoders (conntrack_worker.go).
- BGP enrichment: `bgpEnrichBuilder` subscribes to the typed `ribevents.BestChange` handle and rebuilds the `enrich` radix tree atomically (enrichbgp.go).
- Exporter dispatch (`ExportFlowSample`, `ExportFlows`) with enrichment fan-out and deadlock-safe `Stop` ordering; YANG `sampling`/`conntrack`/`enrichment` config + parse + boundary validation; metrics `samples_total`/`flows_total`/`flows_active`.

### Bugs Found/Fixed
- Reload deadlock risk: worker `Stop()` waits on goroutines that take `e.mu`; `Exporter.Stop` now runs stoppers outside the mutex.

### Documentation Updates
- docs/guide/flow-export.md sampling/conntrack/enrichment sections + Limitations; docs/features.md, comparison.md, core-design.md updated (shared with spec 1).

### Deviations from Plan
- BGP AS enrichment: RESOLVED. Added `OriginAS uint32` + `ASPath []uint32` to `ribevents.BestChangeEntry`, populated in `rib_bestchange.go checkBestPathChange` from the winning candidate's interned AS_PATH handle (via `pool.ASPath.Get` + `formatASPath`); `enrichbgp.go applyBatch` stores them in `enrich.ASEntry`, so `ExportFlows` now fills SrcAS/DstAS (AC-5 / IPFIX IE 16/17). Cold path (once per best-path change). Caveat: full-table replay events omit AS data (the packed best-path record drops the AS-path handle); replayed entries are corrected by the next incremental change. RIB best-change tests pass.
- Extended sFlow if_counters: ifType (IANA, via link-type map) and ifPromiscuousMode (via `InterfaceInfo.Promisc` from netlink link flags) now populated. ifSpeed/ifDirection/multicast/broadcast left zero -- kernel `rtnl_link_stats64` exposes neither per-direction multicast nor broadcast, and speed/duplex need a sysfs read not yet wired into the 1s snapshot path (the iface `ListInterfaces` stats path is the place; tracked as follow-up).
- Per-flow records are IPv4-only (NF9/IPFIX flow templates use IPv4 fields; `As4()` panics on IPv6). IPv6 deferred (C1).
- Conntrack export is periodic-dump only; no immediate destroy-event export (AC-8) -- vishvananda/netlink lacks the NFNLGRP_CONNTRACK_DESTROY binding (C4).
- Netlink worker paths are CI-gated: they cross-compile (`GOOS=linux go vet`) but require a privileged Linux runner to exercise end-to-end (darwin dev host cannot run them).

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `sampling.SetupSampling` via `samplingWorker.Start`; `sampling/tc_linux.go` | CI-gated (Linux+CAP_NET_ADMIN) |
| AC-2 | Done | `PsampleReader.Read` loop in `samplingWorker.run` | CI-gated |
| AC-3 | Done | `sflow.FlowEncoder.EncodeFlowSample` -> WriteSampledHeader; `test/flow-export/sampling.ci` | header capped to MTU |
| AC-4 | Done | flow_sample fields in `WriteFlowSample` (rate/input/pool) | output ifIndex 0 (egress unknown) |
| AC-5 | Partial | `WriteExtendedGateway` exists; enricher provides next-hop only | AS-path deferred (event lacks AS) |
| AC-6 | Done | `conntrackWorker` -> `netflow9.FlowEncoder.EncodeFlows`; `netflow9/flow_*_test.go` | IPv4 only |
| AC-7 | Partial | `ipfix.FlowEncoder.EncodeFlows`; IEs 8/12/7/11/4/85/86/152/153 | IE 16/17/18 require AS data (deferred) |
| AC-8 | Deferred | periodic dump only | no destroy-event listener (C4) |
| AC-9 | Done | `samplingWorker.Stop` RemoveSampling; reload via exporter swap | CI-gated |
| AC-10 | Done | sample filter priority 100 (`sampling.SampleFilterPriority`), mirror at 1 | `test/flow-export/coexist` future |
| AC-11 | Done | `ze_flowexport_samples_total`; show surfaces collector stats | per-interface sampling detail follow-up |
| AC-12 | Done | conntrack worker degrades when accounting/reader unavailable (logged) | |
| AC-13 | Done | `samplesTotal`/`flowsTotal`/`flowsActive` in metrics.go | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | spec-2 packages had no production caller (no lifecycle/dispatch) | flowexport/ | Fixed: built integration layer (workers, dispatch, factories, enrich) |
| 2 | ISSUE | reload deadlock: worker Stop waits on goroutine holding e.mu | exporter.go | Fixed: run stoppers outside e.mu |
| 3 | NOTE | BestChange event lacks AS_PATH | enrichbgp.go | Documented: next-hop-only enrichment |
| 4 | NOTE | NF9/IPFIX flow templates IPv4-only (As4 panic on IPv6) | conntrack_worker.go | Documented: IPv4 guard, IPv6 deferred |

### Fixes applied
- Built the full integration layer (finding 1) and fixed the Stop ordering (finding 2). Re-verified: `golangci-lint` 0 issues; `go test ./internal/component/flowexport/...` pass; `go vet` clean on darwin and `GOOS=linux`; `go build -o bin/ze` (darwin) ok.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | NOTE | AC-5/AC-7 AS enrichment, AC-8 destroy events, IPv6 records deferred | -- | Recorded in Deviations |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

Closure note: code-complete and CI-gated per user direction. Netlink worker paths
cross-compile but require a privileged Linux runner to exercise. The Linux daemon now
builds (the cmd/show plugin.ResponseData break was resolved). The two-commit closure is
user-triggered.

### Run 3 (/ze-review-deep, 10 agents)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | MEDIUM | NF9 flow seqNum advanced per record, not per packet (RFC 3954) | netflow9/flow_adapter.go | Fixed: e.seqNum++ per datagram; regression test TestNetflow9FlowSeqNumPerPacket |
| 2 | MEDIUM | stale docs claimed per-flow IPv4-only after IPv6 landed | docs/guide/flow-export.md, features.md | Fixed |
| 3 | MEDIUM | conntrack zero StartTime -> garbage flowStart timestamps | conntrack_worker.go | Fixed: safeUnixMillis guard |
| 4 | LOW | enrich tree full-rebuild every 1s = GC pressure at scale | enrichbgp.go | Fixed: 5s debounce; incremental noted as follow-up |
| 5 | LOW | bgpEnrichBuilder.Stop did not wait for goroutine | enrichbgp.go | Fixed: doneCh + started guard |
| 6 | LOW | samplingWorker.run could spin on sustained parse errors | sampling_worker.go | Fixed: consecutive-error backoff |
| 7 | LOW | agent-address not validated (silent 0.0.0.0 fallback) | config.go | Fixed: validate() rejects bad agent-address |
| 8 | LOW | ipfix WriteDataSet lacked per-iteration bounds guard | ipfix/data.go | Fixed: matches netflow9/flow_data guard |
| 9 | LOW | sFlow sample_pool uint32 overflow at high rate*seq | sflow/flow_adapter.go | Fixed: uint64 saturating compute |
| 10 | LOW | missing bidirectional // Related: refs | flow_template/flow.go | Fixed |
| 11 | HIGH(test) | exporter dispatch/Stop + family split untested | exporter_test.go, netflow9/flow_adapter_test.go | Fixed: added unit + regression tests |

### Run 3 accepted / documented (not changed)
- Config silently ignores unknown JSON keys: the YANG schema is the authoritative unknown-key gate before this JSON is produced; the lenient map parse is a secondary layer shared with the pre-existing collector parser. Accepted.
- `Enrichment.SrcASPath/DstASPath` are scaffolding for the deferred AS-path feature (nil today); kept rather than churned.
- Per-packet `make` in psample parse and Prometheus `With()` on the sample path: codebase-wide metrics pattern; sampling is rate-limited (1-in-N). Noted as a future optimization, not changed in isolation.
- `WriteExtendedGateway` bounds (dead code) / `WriteMessage` template+data combo: unreachable today; documented preconditions.

### Final status
- [ ] `/ze-review-deep` clean: 0 critical, 0 unaddressed high (the one HIGH was a test-coverage gap, now filled)
- Re-verified: golangci-lint 0 issues, `go test -race ./internal/component/flowexport/...` pass, go vet clean darwin + GOOS=linux, ze + ze.linux + ze-test build.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-flow-export-2-flow-records.md`
- [ ] Summary included in commit
