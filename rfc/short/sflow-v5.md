# sFlow Version 5 Specification

## Meta

| Field | Value |
|-------|-------|
| Spec | sFlow Version 5 |
| Title | sFlow: A Method for Monitoring Traffic in Switched and Routed Networks |
| Author | Peter Phaal, Marc Lavine (InMon Corp., Foundry Networks) |
| Date | July 2004 |
| Status | Industry specification (FINAL, Version 1.00) |
| Relation | Supersedes sFlow v2/v4 (RFC 3176, September 2001) |
| Authoritative URL | https://sflow.org/sflow_version_5.txt |
| Encoding | XDR (RFC 1832 / RFC 4506) |
| Transport | UDP, recommended port 6343 |
| Enrolment | enrolled |
| Enrolment reason | sFlow Version 5 exporter/agent. The checklist includes transport, sample encoding, counter availability and sampling obligations from the 2026-09-21 extraction walk. Tests cover parts of these obligations; the checklist retains the remaining gaps and uncovered requirements. |
| Implementation | ze |
| Implementation reason | Ze's own Go implements this document; each requirement row cites its producer. |
| Support | drafts 80 |
| Support area | sFlow export |
| Support status | Experimental |
| Support coverage | Flow export alongside NetFlow v9 and IPFIX. Unavailable counters carry the maximum-value sentinel. Counter and flow encoders share a datagram sequence; all sources use expanded sample formats with full-width interface indexes. Counter generation changes restart source sample sequences. Other uncovered obligations remain in the checklist. Statistical sampling requirements SFLOW-V5-x-37 through x-41 remain unverified. Linux filter installation/readback tests cover configuration and lifecycle, not RNG range, packet-selection probability, one consideration per packet, independent sampling draws or long-term convergence. Current exporter and privileged kernel scenarios must be run before publishing verified coverage. |
| Support remaining | - |

**Purpose:** sFlow defines a sampling-based mechanism for monitoring traffic
in switched and routed networks. An sFlow agent embedded in a network
device exports two kinds of data: flow samples (sampled packet headers
with forwarding metadata) and counter samples (periodic interface and
system statistics). A central collector receives UDP datagrams containing
these samples and reconstructs traffic patterns, utilization, and
forwarding behavior across the monitored infrastructure.

**Scope:** Agent architecture, datagram wire format, sample types,
record types, XDR encoding. Does not define collector protocol, analysis
algorithms, or SNMP MIB. sFlow v5 is not an IETF standard.

**Relation to RFC 3176:** RFC 3176 covers sFlow v2/v4 and the SNMP MIB.
v5 adds an extensible enterprise/format namespace, expanded sample types
for large ifIndex, processor counters, and reorganized XDR. Sampling and
counter-polling concepts are unchanged.

## Agent Architecture

### Agent

Software embedded in a network device that performs packet sampling,
polls counters, marshals samples into UDP datagrams, and sends them to
collectors. The agent address is a stable IP (loopback) that uniquely
identifies the device across reboots.

### Sub-Agent

Devices with distributed forwarding may run multiple sub-agents, each
with a numeric `sub_agent_id` (u32). Agent address + sub-agent ID
uniquely identifies a sampling entity. Each maintains its own sequence
number space.

### Data Sources

A point where traffic can be observed. Types (high byte of
`sflow_data_source`): 0=ifIndex, 1=VLAN (smonVlanDataSource),
2=physical entity (entPhysicalEntry). Each source can have independent
sampling parameters.

### sFlow Instances

An sFlow instance binds a data source to sampling parameters: packet
sampling rate, counter polling interval, max header size, collector
address. Multiple instances can exist per data source.

## Transport

- **Protocol:** UDP only. Intentionally unreliable: lost datagrams
  reduce the effective sampling rate slightly but do not corrupt the
  statistical model.
- **Port:** 6343 (IANA-assigned). Configurable per instance.
- **Max datagram size:** Default 1400 bytes. Must not exceed path MTU.
- **Timing:** Samples must not be held more than 1 second before sending.
- **Sequence numbers:** Per-agent (datagram-level) and per-source
  (sample-level), unsigned 32-bit, wrapping at 2^32-1.

## Datagram Format

All structures use XDR encoding: 4-byte alignment, big-endian,
variable-length arrays prefixed by a 4-byte count.

### Top-Level Datagram

```
struct sample_datagram {
   unsigned int version;           /* 5 */
   address      agent_address;     /* IP of the agent */
   unsigned int sub_agent_id;      /* 0 for single-agent devices */
   unsigned int sequence_number;   /* per sub-agent, increments per datagram */
   unsigned int uptime;            /* sysUptime in milliseconds */
   sample_record samples<>;        /* array of sample records */
}
```

Address is `{address_type, opaque addr}` where type 1=IPv4 (4 bytes),
2=IPv6 (16 bytes).

### Data Format Encoding (Enterprise/Format)

sFlow v5 uses a 32-bit `data_format` field to identify record types:

```
typedef unsigned int data_format;

   Most significant 20 bits: SMI Private Enterprise Code
   Least significant 12 bits: structure format number
```

Enterprise 0 = standard sFlow namespace. Other values = vendor
extensions. Sample types: 0:1=flow_sample, 0:2=counters_sample,
0:3=flow_sample_expanded, 0:4=counters_sample_expanded.

Each sample_record is `{data_format, opaque sample_data<>}`. Unknown
formats are skipped using the opaque length prefix.

### Data Source Encoding

`sflow_data_source` (u32): high 8 bits = type (0=ifIndex, 1=VLAN,
2=entity), low 24 bits = index. Limits ifIndex to 2^24-1.
Expanded form: `{unsigned int source_id_type, unsigned int source_id_index}`.

### Interface Encoding (Input/Output)

`interface` (u32): high 2 bits = format (0=single ifIndex, 1=discarded
with reason, 2=multiple destinations with count), low 30 bits = value.
Special: 0x3FFFFFFF = unknown, 0 = no interface.
Expanded form: `{unsigned int format, unsigned int value}`.

## Sample Types

### Flow Sample (enterprise 0, format 1)

Produced when a packet is selected by the sampling mechanism.

```
struct flow_sample {
   unsigned int      sequence_number;  /* per-source sequence */
   sflow_data_source source_id;        /* data source that sampled */
   unsigned int      sampling_rate;    /* 1-in-N */
   unsigned int      sample_pool;      /* total packets seen by source */
   unsigned int      drops;            /* packets dropped (resource limits) */
   interface         input;            /* ingress ifIndex */
   interface         output;           /* egress ifIndex */
   flow_record       flow_records<>;   /* array of flow records */
}
```

The collector multiplies each sampled packet by `sampling_rate` to
estimate totals. `sample_pool` tracks total packets seen; `drops`
tracks samples lost to resource limits.

### Expanded Flow Sample (enterprise 0, format 3)

Same fields as flow_sample but uses `sflow_data_source_expanded` and
`interface_expanded` (separate 32-bit type + index fields) instead of
the packed encodings. Required when ifIndex > 2^24-1 or 2^30-1.

### Counter Sample (enterprise 0, format 2)

Produced at the configured polling interval for each data source.

```
struct counters_sample {
   unsigned int      sequence_number;  /* per-source sequence */
   sflow_data_source source_id;        /* data source being polled */
   counter_record    counters<>;       /* array of counter records */
}
```

### Expanded Counter Sample (enterprise 0, format 4)

Same as counters_sample but with `sflow_data_source_expanded` for
large ifIndex values.

### Record Wrappers

Both `flow_record` and `counter_record` are `{data_format, opaque data<>}`.
Unknown formats skipped via the opaque length prefix.

## Counter Record Types

### Generic Interface Counters (enterprise 0, format 1)

Maps directly to SNMP ifTable (RFC 2233) and ifXTable (RFC 2863) MIB
objects. This is the primary counter record for interface monitoring.

```
struct if_counters {
   unsigned int   ifIndex;             /* ifIndex from SNMP */
   unsigned int   ifType;              /* ifType (IANA ifType) */
   unsigned hyper ifSpeed;             /* ifSpeed/ifHighSpeed in bits/sec */
   unsigned int   ifDirection;         /* 0=unknown, 1=full-duplex,
                                          2=half-duplex, 3=in, 4=out */
   unsigned int   ifStatus;            /* bit 0: ifAdminStatus (0=down, 1=up)
                                          bit 1: ifOperStatus  (0=down, 1=up) */
   unsigned hyper ifInOctets;          /* ifHCInOctets */
   unsigned int   ifInUcastPkts;       /* ifInUcastPkts */
   unsigned int   ifInMulticastPkts;   /* ifInMulticastPkts */
   unsigned int   ifInBroadcastPkts;   /* ifInBroadcastPkts */
   unsigned int   ifInDiscards;        /* ifInDiscards */
   unsigned int   ifInErrors;          /* ifInErrors */
   unsigned int   ifInUnknownProtos;   /* ifInUnknownProtos */
   unsigned hyper ifOutOctets;         /* ifHCOutOctets */
   unsigned int   ifOutUcastPkts;      /* ifOutUcastPkts */
   unsigned int   ifOutMulticastPkts;  /* ifOutMulticastPkts */
   unsigned int   ifOutBroadcastPkts;  /* ifOutBroadcastPkts */
   unsigned int   ifOutDiscards;       /* ifOutDiscards */
   unsigned int   ifOutErrors;         /* ifOutErrors */
   unsigned int   ifPromiscuousMode;   /* 0=false, 1=true */
}
```

Total fixed size: 88 bytes (16 x unsigned int = 64, 3 x unsigned hyper = 24).

Field notes: ifSpeed is 64-bit for 100G+ links (bits/sec). ifDirection
is an sFlow extension (0=unknown, 1=full-duplex, 2=half-duplex, 3=in,
4=out). ifStatus packs two bits: bit 0 = ifAdminStatus, bit 1 =
ifOperStatus (0=down, 1=up each). ifInOctets/ifOutOctets are 64-bit
(ifHCInOctets/ifHCOutOctets from ifXTable). ifPromiscuousMode is boolean
as unsigned int.

### Ethernet Counters (enterprise 0, format 2)

Maps to the EtherLike-MIB (RFC 2358, dot3StatsTable).

```
struct ethernet_counters {
   unsigned int dot3StatsAlignmentErrors;
   unsigned int dot3StatsFCSErrors;
   unsigned int dot3StatsSingleCollisionFrames;
   unsigned int dot3StatsMultipleCollisionFrames;
   unsigned int dot3StatsSQETestErrors;
   unsigned int dot3StatsDeferredTransmissions;
   unsigned int dot3StatsLateCollisions;
   unsigned int dot3StatsExcessiveCollisions;
   unsigned int dot3StatsInternalMacTransmitErrors;
   unsigned int dot3StatsCarrierSenseErrors;
   unsigned int dot3StatsFrameTooLongs;
   unsigned int dot3StatsInternalMacReceiveErrors;
   unsigned int dot3StatsSymbolErrors;
}
```

Total fixed size: 52 bytes (13 x unsigned int). Useful for detecting
physical layer problems (bad cables, failing optics, duplex mismatches).

### VLAN Counters (enterprise 0, format 5)

Per-VLAN traffic statistics.

```
struct vlan_counters {
   unsigned int   vlan_id;
   unsigned hyper octets;
   unsigned int   ucastPkts;
   unsigned int   multicastPkts;
   unsigned int   broadcastPkts;
   unsigned int   discards;
}
```

Total fixed size: 28 bytes.

### Processor Counters (enterprise 0, format 1001)

System-level resource utilization. Typically one per device (not per
interface).

```
struct processor {
   unsigned int   5s_cpu;        /* 5-second CPU utilization (percentage) */
   unsigned int   1m_cpu;        /* 1-minute CPU utilization */
   unsigned int   5m_cpu;        /* 5-minute CPU utilization */
   unsigned hyper total_memory;  /* total memory in bytes */
   unsigned hyper free_memory;   /* free memory in bytes */
}
```

Total fixed size: 28 bytes. CPU percentages 0-100, memory in bytes.

## Flow Record Types

### Raw Packet Header (enterprise 0, format 1)

The most common flow record. Contains the first N bytes of the sampled
packet, allowing the collector to parse L2-L7 headers.

```
enum header_protocol {
   ETHERNET_ISO88023  = 1,
   ISO88024_TOKENBUS  = 2,
   ISO88025_TOKENRING = 3,
   FDDI               = 4,
   FRAME_RELAY        = 5,
   X25                = 6,
   PPP                = 7,
   SMDS               = 8,
   AAL5               = 9,
   AAL5_IP            = 10,
   IPv4               = 11,
   IPv6               = 12,
   MPLS               = 13,
   POS                = 14
}

struct sampled_header {
   header_protocol protocol;       /* link layer type */
   unsigned int    frame_length;   /* original frame length on wire */
   unsigned int    stripped;       /* bytes stripped before sampling
                                      (e.g., preamble, FCS) */
   opaque          header<>;       /* first N bytes of the packet
                                      (N = sFlowMaxHeaderSize, default 128) */
}
```

Default max header size is 128 bytes (Ethernet + IPv4/IPv6 + TCP/UDP).
`frame_length` is the original wire size; `stripped` accounts for
preamble/FCS removed before sampling.

### Sampled IPv4 (enterprise 0, format 3)

Pre-parsed IPv4 fields. Less common than raw headers.

```
struct sampled_ipv4 {
   unsigned int length;      /* IP total length */
   unsigned int protocol;    /* IP protocol (6=TCP, 17=UDP, etc.) */
   ip_v4        src_ip;      /* source IPv4 address */
   ip_v4        dst_ip;      /* destination IPv4 address */
   unsigned int src_port;    /* TCP/UDP source port (0 if N/A) */
   unsigned int dst_port;    /* TCP/UDP destination port (0 if N/A) */
   unsigned int tcp_flags;   /* TCP flags (0 if not TCP) */
   unsigned int tos;         /* IP type-of-service / DSCP */
}
```

### Sampled IPv6 (enterprise 0, format 4)

```
struct sampled_ipv6 {
   unsigned int length;      /* IP payload length */
   unsigned int protocol;    /* next-header value */
   ip_v6        src_ip;      /* source IPv6 address */
   ip_v6        dst_ip;      /* destination IPv6 address */
   unsigned int src_port;    /* TCP/UDP source port (0 if N/A) */
   unsigned int dst_port;    /* TCP/UDP destination port (0 if N/A) */
   unsigned int tcp_flags;   /* TCP flags (0 if not TCP) */
   unsigned int priority;    /* traffic class */
}
```

### Extended Switch Data (enterprise 0, format 1001)

```
struct extended_switch {
   unsigned int src_vlan;      /* ingress 802.1Q VLAN ID */
   unsigned int src_priority;  /* ingress 802.1p priority */
   unsigned int dst_vlan;      /* egress 802.1Q VLAN ID */
   unsigned int dst_priority;  /* egress 802.1p priority */
}
```

### Extended Router Data (enterprise 0, format 1002)

```
struct extended_router {
   address      nexthop;       /* IP next-hop from the routing table */
   unsigned int src_mask_len;  /* source prefix length (e.g., /24) */
   unsigned int dst_mask_len;  /* destination prefix length */
}
```

### Extended Gateway Data (enterprise 0, format 1003)

BGP context. Relevant for Ze's BGP integration.

```
struct extended_gateway {
   address      nexthop;           /* BGP next-hop */
   unsigned int as;                /* agent's AS number */
   unsigned int src_as;            /* source AS (longest-match) */
   unsigned int src_peer_as;       /* peer AS the route was learned from */
   as_path_type dst_as_path<>;     /* AS path to destination */
   unsigned int communities<>;     /* BGP community values */
   unsigned int localpref;         /* LOCAL_PREF */
}
```

### Other Extended Types (less relevant to Ze)

- **Extended User (1004):** AAA user identity (src/dst charset + user string).
- **Extended URL (1005):** HTTP URL + host, with direction enum.
- **Extended MPLS (1006):** nexthop + in/out label stacks.
- **Extended NAT (1007):** pre-NAT src/dst addresses.

## Packet Sampling Mechanism

Per-source skip counter initialized to random value in [1, 2*N-1].
Decremented per packet; at zero, sample and reset to new random value.
Long-term rate converges to 1/N. Random reset prevents periodic traffic
from evading sampling. Agents may adjust rates to hardware-supported
values; actual rate reported in each flow sample.

## Counter Polling

The agent polls each data source at the configured
`sFlowCounterSamplingInterval` (typical: 20-120 seconds). Polls should
be staggered across sources (randomized initial offset) to avoid CPU
spikes. Counter samples may piggyback on flow sample datagrams when
space permits. All counters are cumulative since boot; the collector
computes rates by differencing consecutive samples. 64-bit counters
eliminate wraparound risk on 100G+ links.

## Counter Export

Ze exports interface counters alongside packet samples. Counter-only
operation is a supported mode, but its availability does not prove the
separate packet-sampling obligations.

### Minimum Required

1. Datagram header (version=5, agent address, sub_agent_id=0, seq, uptime).
2. Counter sample (per-interface sequence, source_id with ifIndex).
3. if_counters record (all 19 fields from Ze's interface stats).

### What to Send

Per interface and polling interval, `sflow.CounterEncoder.Encode` writes
expanded counter samples with the full interface index. The source sequence
restarts after a counter-generation change. Counter and flow encoders use
the collector's shared datagram sequence.

### Mapping Ze Interface Stats to if_counters

| if_counters field | Ze source |
|-------------------|-----------|
| ifIndex | Interface index from netlink / internal assignment |
| ifType | IANA ifType (6=ethernetCsmacd, 131=tunnel, 53=propVirtual, etc.) |
| ifSpeed | Link speed in bits/sec from netlink or config |
| ifDirection | 1=full-duplex (default for virtual interfaces) |
| ifStatus | bit 0 = admin up, bit 1 = oper up |
| ifInOctets | rx_bytes from kernel counters |
| ifInUcastPkts | rx_packets (minus multicast/broadcast if available) |
| ifInMulticastPkts | Kernel multicast count when available; otherwise unavailable sentinel |
| ifInBroadcastPkts | Kernel broadcast count when available; otherwise unavailable sentinel |
| ifInDiscards | rx_dropped |
| ifInErrors | rx_errors |
| ifInUnknownProtos | Unavailable sentinel when the source does not supply this counter |
| ifOutOctets | tx_bytes |
| ifOutUcastPkts | tx_packets (minus multicast/broadcast if available) |
| ifOutMulticastPkts | Kernel multicast count when available; otherwise unavailable sentinel |
| ifOutBroadcastPkts | Kernel broadcast count when available; otherwise unavailable sentinel |
| ifOutDiscards | tx_dropped |
| ifOutErrors | tx_errors |
| ifPromiscuousMode | IFF_PROMISC flag from interface flags |

Unavailable fields: set to max value for the type (0xFFFFFFFF for u32,
0xFFFFFFFFFFFFFFFF for u64).

### XDR Encoding

Big-endian. `unsigned int` = 4 bytes, `unsigned hyper` = 8 bytes.
Variable-length arrays/opaque: 4-byte count prefix, data padded to
4-byte boundary. `data_format`: `(enterprise << 12) | format`;
decode: `enterprise = df >> 12; format = df & 0xFFF`.

### Datagram Construction

Allocate buffer (max 1400 bytes). Write header, reserve 4 bytes for
sample count. For each pending sample: encode into temp buffer, if
encoded size + 8 (record header) fits, append it and increment count;
otherwise send current datagram and start a new one. Backpatch count,
send via UDP.

## Packet Sampling

`internal/plugins/flowexport/sampling_worker.go::Start` installs a Linux
ingress MatchAll sample action through `sampling.SetupSampling` and
`buildSampleFilter`. `PsampleReader.Read` receives sampled packet headers;
the worker passes them to `exporter.exportFlowSample`, then
`sflow.FlowEncoder.EncodeFlowSample` and `Sender.Send`.

Linux `act_sample` owns the random draw. The kernel lifecycle test checks
installed rate, group, truncation and ingress placement, and whether
replacing or removing one source preserves another. Those observations do
not prove the statistical or per-packet requirements below. No full sFlow
conformance claim follows from counter export or filter configuration.

## Security Considerations

sFlow datagrams are unencrypted, unauthenticated UDP. Mitigations:
dedicated VLAN, source address filtering at collector, VPN for WAN.
Collectors should validate sequence numbers. Counter-only export avoids
the packet-payload exposure concern of flow sampling. Agent config
should be access-controlled.

## Quick Reference: Enterprise 0 Format Numbers

| Kind | Fmt | Structure | Description |
|------|-----|-----------|-------------|
| Sample | 1 | flow_sample | Standard flow sample |
| Sample | 2 | counters_sample | Standard counter sample |
| Sample | 3 | flow_sample_expanded | Large ifIndex flow sample |
| Sample | 4 | counters_sample_expanded | Large ifIndex counter sample |
| Counter | 1 | if_counters | Generic interface (ifTable/ifXTable) |
| Counter | 2 | ethernet_counters | Ethernet (dot3StatsTable) |
| Counter | 5 | vlan_counters | Per-VLAN |
| Counter | 1001 | processor | CPU and memory |
| Flow | 1 | sampled_header | Raw packet header bytes |
| Flow | 3 | sampled_ipv4 | Pre-parsed IPv4 |
| Flow | 4 | sampled_ipv6 | Pre-parsed IPv6 |
| Flow | 1001 | extended_switch | 802.1Q VLAN/priority |
| Flow | 1002 | extended_router | Next-hop, prefix lengths |
| Flow | 1003 | extended_gateway | BGP AS path, communities, localpref |
| Flow | 1004 | extended_user | AAA user identity |
| Flow | 1005 | extended_url | HTTP URL |
| Flow | 1006 | extended_mpls | MPLS label stacks |
| Flow | 1007 | extended_nat | Pre-NAT addresses |

## References for Ze

| Document | Role | Ze touchpoint |
|----------|------|---------------|
| sFlow v5 spec | Datagram format, record types, XDR encoding | sFlow exporter component (counter samples) |
| RFC 3176 | sFlow v2/v4, SNMP MIB, sampling theory | Background context, MIB compatibility |
| RFC 1832 / RFC 4506 | XDR encoding | Wire format for all sFlow structures |
| RFC 2233 / RFC 2863 | Interfaces Group MIB (ifTable, ifXTable) | Source of generic interface counter definitions |
| RFC 2358 | EtherLike-MIB (dot3StatsTable) | Source of ethernet counter definitions |

## Compliance Checklist

Note: sFlow v5 is an industry specification, not an IETF RFC. The following requirements are derived from the sFlow v5 specification using RFC 2119-style language where the specification prescribes mandatory or recommended behavior.

- [ ] [SFLOW-V5-x-1] [MUST] Datagram version field MUST be set to 5 (Datagram Format) {single-polarity: positive; WriteDatagramHeader unconditionally writes the compile-time constant Version=5 into every datagram, so there is no other-version code path to reject (internal/plugins/flowexport/sflow/encoder.go:40, :14)}
- [ ] [SFLOW-V5-x-2] [MUST] Agent address MUST be a stable IP (e.g., loopback) that uniquely identifies the device across reboots (Agent Architecture) {single-polarity: positive; the operator-configured agent address is written verbatim into every datagram header and validated as a well-formed IP; stability/uniqueness is a config-value property with no exporter reject path (internal/plugins/flowexport/sflow/encoder.go:43-57, internal/plugins/flowexport/config.go:379-383)}
- [ ] [SFLOW-V5-x-3] [MUST] Agent address + sub_agent_id MUST uniquely identify a sampling entity (Agent Architecture) {single-polarity: positive; both agent_address and sub_agent_id are emitted in every datagram header by construction, and tuple uniqueness is an operator-config obligation (internal/plugins/flowexport/sflow/encoder.go:59, :43-57)}
- [ ] [SFLOW-V5-x-4] [MUST] Each sub-agent MUST maintain its own sequence number space (Agent Architecture) {single-polarity: positive; each collector owns its UDP Sender sequence shared by counter and flow encoders, while per-source sample sequences belong to its encoders (internal/plugins/flowexport/sender.go Sender.Sequence, internal/plugins/flowexport/sflow/adapter.go CounterEncoder, internal/plugins/flowexport/sflow/flow_adapter.go FlowEncoder)}
- [ ] [SFLOW-V5-x-5] [MUST] Datagram size MUST NOT exceed path MTU (Transport) {single-polarity: positive; encoders bound the payload, including padding, to the collector's max-datagram-size and Sender.Send refuses larger payloads; the operator must configure the bound for the path MTU (internal/plugins/flowexport/sender.go Sender.Send, internal/plugins/flowexport/sflow/encoder.go writeCounterDatagrams, internal/plugins/flowexport/sflow/flow_adapter.go EncodeFlowSample)}
- [ ] [SFLOW-V5-x-6] [MUST] Samples MUST NOT be held more than 1 second before sending (Transport) {single-polarity: positive; counter and flow samples are encoded and sent synchronously with no buffering queue, so a sample is never held beyond a sub-millisecond encode and there is no holding timer to test negatively (internal/plugins/flowexport/exporter.go:204, internal/plugins/flowexport/sflow/adapter.go:47-51)}
- [ ] [SFLOW-V5-x-7] [MUST] All structures MUST use XDR encoding: 4-byte alignment, big-endian (Datagram Format) {single-polarity: positive; every field is written via binary.BigEndian with 4-byte-aligned opaque padding, exporter-only with no decode path to reject a wrong endianness (internal/plugins/flowexport/sflow/counter.go:36, flow.go:123-128)}
- [ ] [SFLOW-V5-x-8] [MUST] Variable-length arrays and opaque data MUST be prefixed by a 4-byte count and padded to 4-byte boundary (XDR Encoding) {single-polarity: positive; the sampled_header opaque and the extended_gateway arrays are all written with a 4-byte count prefix and zero-padded to a 4-byte boundary by construction (internal/plugins/flowexport/sflow/flow.go:116-128, :187-208)}
- [ ] [SFLOW-V5-x-9] [MUST] Sequence numbers MUST be per-agent (datagram-level) and per-source (sample-level), unsigned 32-bit, wrapping (Transport)
- [ ] [SFLOW-V5-x-10] [MUST] flow_sample MUST include the actual sampling_rate used by the agent (Flow Sample) {single-polarity: positive; EncodeFlowSample writes the kernel-reported actual rate into every flow_sample, emitted unconditionally with no reject path (internal/plugins/flowexport/sflow/flow_adapter.go:78-79, flow.go:52)}
- [ ] [SFLOW-V5-x-11] [MUST] sample_pool MUST track total packets seen by the data source (Flow Sample) {single-polarity: positive; EncodeFlowSample computes sample_pool as the saturated product of cumulative samples and rate and writes it into every flow_sample, exporter-only with no negative form (internal/plugins/flowexport/sflow/flow_adapter.go:73-79, flow.go:56)}
- [ ] [SFLOW-V5-x-12] [MUST] Expanded sample types MUST be used when ifIndex exceeds 2^24-1 or 2^30-1 (Expanded Flow/Counter Sample)
- [ ] [SFLOW-V5-x-13] [MUST] Unknown record formats MUST be skipped using the opaque length prefix (Record Wrappers) {not-applicable: skipping unknown formats on receive is a collector behavior; ze is an sFlow exporter only with no sFlow decode path, though it does emit the length prefixes that let a collector skip (internal/plugins/flowexport/sflow/counter.go:60-61, flow.go:131-132)}
- [ ] [SFLOW-V5-x-14] [MUST] Counter samples MUST be produced at the configured polling interval for each data source (Counter Polling)
- [ ] [SFLOW-V5-x-15] [MUST] All counters MUST be cumulative since boot (Counter Polling) {single-polarity: positive; interfaceCountersFrom copies the raw cumulative kernel counters straight through with no differencing, so exported if_counters are cumulative by construction (internal/plugins/flowexport/register.go:343-358, snapshot.go:10-13)}
- [ ] [SFLOW-V5-x-16] [MUST] Unavailable counter fields MUST be set to max value for the type (0xFFFFFFFF for u32, 0xFFFFFFFFFFFFFFFF for u64) (Implementation Guidance)
- [ ] [SFLOW-V5-x-25] [MUST] "the sFlow Agent must not wait for a buffer to fill with samples before sending the sFlow Datagram" (Transport)
- [ ] [SFLOW-V5-x-26] [MUST] "If counters must be sent in order to satisfy the maximum sampling interval then a datagram must be sent containing the outstanding counters." (Counter Polling)
- [ ] [SFLOW-V5-x-27] [MUST NOT] "An agent must not mix compact/expanded encodings." An agent that will never use ifIndex numbers >= 2^24 "must use compact encodings for all interfaces", otherwise "the expanded formats must be used for all interfaces" (Expanded Flow/Counter Sample)
- [ ] [SFLOW-V5-x-28] [MUST] "If the agent resets the sample_pool then it must also reset the sequence_number" of the flow sample (Flow Sample)
- [ ] [SFLOW-V5-x-29] [MUST] "If the agent resets any of the counters then it must also reset the sequence_number" of the counter sample (Counter Sample)
- [ ] [SFLOW-V5-x-30] [MUST] "In the case of ifIndex-based source_id's the sequence number must be reset each time ifCounterDiscontinuityTime changes." (Counter Sample)
- [ ] [SFLOW-V5-x-31] [MUST] "Each sFlowDataSource must be associated with only one sub-agent", and "The association between sFlowDataSource and sub-agent must remain constant for the entire duration of an sFlow session." (Agent Architecture)
- [ ] [SFLOW-V5-x-32] [MUST] "Within any given sFlow session a particular counter must be always available, or always unavailable." (Counter Polling)
- [ ] [SFLOW-V5-x-33] [MUST] "A flow_sample must contain packet header information." (Flow Sample)
- [ ] [SFLOW-V5-x-34] [MUST] "Any octets added to the frame_length to compensate for encapsulations removed by the underlying hardware must also be added to the stripped count." (Raw Packet Header)
- [ ] [SFLOW-V5-x-35] [MUST] "Trailing encapsulation data for the outermost protocol layer included in the sampled header must be stripped", as must trailing data "corresponding to any leading encapsulations that were stripped" and "Outer encapsulations that are ambiguous, or not one of the standard header_protocol" (Raw Packet Header)
- [ ] [SFLOW-V5-x-36] [MUST] "extended_switch data must always be reported to describe the ingress/egress VLAN information for the packet." (Extended Switch)
- [ ] [SFLOW-V5-x-37] [MUST] "The random number generator must ensure that all numbers in the range between its maximum and minimum values of the distribution are possible" (Packet Sampling) {gap: Linux act_sample owns the random draw; the current source and kernel filter lifecycle test do not prove its output range}
- [ ] [SFLOW-V5-x-38] [MUST] Packet Flow Sampling "must ensure that any packet observed at a Data Source has an equal chance of being sampled, irrespective of the Packet Flow(s) to which it belongs" (Packet Sampling) {gap: MatchAll filter configuration is not behavioral proof of equal packet-selection probability}
- [ ] [SFLOW-V5-x-39] [MUST] "Each packet must only be considered once for sampling, irrespective of the number of ports it will be forwarded to." (Packet Sampling) {gap: ingress filter readback does not observe how often forwarded or replicated packets are considered by the sampler}
- [ ] [SFLOW-V5-x-40] [MUST] "Each sFlow sampler instance must operate independently of all other instances", and "Setting an attribute of one sampler must not alter the the behavior and settings of other sampler instances." (Agent Architecture) {gap: the kernel lifecycle test checks configuration isolation; independent sampling behavior remains unproven}
- [ ] [SFLOW-V5-x-41] [MUST] "The sampling algorithm must converge so that over time the number of packets sampled approaches 1/Nth of the total number of packets in the monitored flows." (Packet Sampling) {gap: configuring an act_sample rate does not measure long-term convergence}
- [ ] [SFLOW-V5-x-42] [MUST] After an agent adjusts a configured sampling rate, "The sampling rate must stay at its new value and never automatically return to the originally configured value." (Packet Sampling)
- [ ] [SFLOW-V5-x-17] [SHOULD] Default datagram max size SHOULD be 1400 bytes (Transport)
- [ ] [SFLOW-V5-x-18] [SHOULD] Default max header size SHOULD be 128 bytes for sampled_header (Raw Packet Header)
- [ ] [SFLOW-V5-x-19] [SHOULD] Counter polls SHOULD be staggered across sources with randomized initial offset (Counter Polling)
- [ ] [SFLOW-V5-x-20] [SHOULD] Per-source skip counter SHOULD be initialized to random value in [1, 2*N-1] for 1-in-N sampling (Packet Sampling)
- [ ] [SFLOW-V5-x-21] [SHOULD] UDP port 6343 SHOULD be used (IANA-assigned) (Transport)
- [ ] [SFLOW-V5-x-22] [MAY] Multiple sFlow instances MAY exist per data source (Agent Architecture)
- [ ] [SFLOW-V5-x-23] [MAY] Counter samples MAY piggyback on flow sample datagrams when space permits (Counter Polling)
- [ ] [SFLOW-V5-x-24] [MAY] Agents MAY adjust sampling rates to hardware-supported values; actual rate MUST be reported in each flow sample (Packet Sampling)
