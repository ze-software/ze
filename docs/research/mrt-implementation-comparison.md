# MRT Implementation Comparison

A feature comparison of MRT (Multi-Threaded Routing Toolkit, RFC 6396) support
across open-source BGP implementations. Based on source code review of local
checkouts (2026-06).

> **Disclaimer:** This comparison was generated with AI assistance from source
> code analysis. All listed projects are under active development and their
> capabilities change over time. Verify current features against each project's
> own documentation before making decisions.

Last updated: 2026-06-07

## Implementations Surveyed

| Implementation | Language | Source Location |
|----------------|----------|----------------|
| **Ze** | Go | `internal/mrt/`, `internal/component/mrt/`, `internal/analyze/` |
| FRRouting (FRR) | C | `~/Code/github.com/FRRouting/frr/` |
| BIRD 3 | C | `~/Code/gitlab.nic.cz/labs/bird/` |
| OpenBGPD | C | `~/Code/github.com/openbgpd-portable/openbgpd-portable/` |
| GoBGP | Go | `~/Code/github.com/osrg/gobgp/` |
| freeRtr | Java | `~/Code/freertr.org/rtr/` |
| RustyBGP | Rust | `~/Code/github.com/osrg/rustybgp/` |
| bgpkit-parser | Rust | `~/Code/github.com/bgpkit/bgpkit-parser/` (library, not a daemon) |
| ExaBGP | Python | `~/Code/github.com/exa-networks/exabgp/` |
| bio-rd | Go | `~/Code/github.com/bio-routing/bio-rd/` |

## MRT Type Support

### Write (daemon emits MRT)

| Type | Ze | FRR | BIRD 3 | OpenBGPD | GoBGP | freeRtr | RustyBGP |
|------|----|-----|--------|----------|-------|---------|----------|
| TABLE_DUMP v1 (12) | Encode | No | No | Yes | No | No | No |
| TABLE_DUMP_V2 (13) | Encode | Yes | Yes | Yes | Yes | Yes | No |
| BGP4MP (16) | Encode | Yes | Yes | No | Yes | Yes | Yes |
| BGP4MP_ET (17) | Encode | Yes | No | Yes | No | No | No |

Ze has encoders for all types. The daemon component produces live BGP4MP records
from the reactor via MessageObserver (raw wire bytes, async non-blocking writes).
Periodic TABLE_DUMP_V2 RIB snapshots use a cross-plugin bridge to iterate the
RIB under short per-family locks. The ze-chaos tool also uses BGP4MP encoders
to produce MRT recordings from chaos test sessions (`--mrt-file` flag).

### Read (parse MRT files)

| Type | Ze | FRR (bgp_btoa) | GoBGP | freeRtr | bgpkit-parser |
|------|----|----------------|-------|---------|---------------|
| TABLE_DUMP v1 (12) | Yes | Yes | No | No | Yes |
| TABLE_DUMP_V2 (13) | Yes | No | Yes | Yes | Yes |
| BGP4MP (16) | Yes | Yes | Yes | Yes | Yes |
| BGP4MP_ET (17) | Yes | Yes | Yes | No | Yes |
| IPv6 in BGP4MP | Yes | No | Yes | Yes | Yes |
| All add-path variants | Yes | No | Yes | Yes | Yes |
| RIB_GENERIC | Yes | No | Yes (defined) | No | Yes |
| GEO_PEER_TABLE | Yes | No | Yes (defined) | No | Yes |

Ze has the most complete read-side coverage of any Go implementation.
ExaBGP, BIRD, OpenBGPD, RustyBGP, and bio-rd have no MRT reading capability.

## TABLE_DUMP_V2 Sub-types (Write)

| Sub-type | FRR | BIRD 3 | OpenBGPD | GoBGP | freeRtr |
|----------|-----|--------|----------|-------|---------|
| PEER_INDEX_TABLE (1) | Yes | Yes | Yes | Yes | No |
| RIB_IPV4_UNICAST (2) | Yes | Yes | Yes | Yes | Yes |
| RIB_IPV4_MULTICAST (3) | No | Defined | Yes | Yes | Yes |
| RIB_IPV6_UNICAST (4) | Yes | Yes | Yes | Yes | Yes |
| RIB_IPV6_MULTICAST (5) | No | Defined | Yes | Yes | Yes |
| RIB_GENERIC (6) | No | Defined | Yes | No | No |
| GEO_PEER_TABLE (7, RFC 6397) | No | No | No | No | No |
| RIB_IPV4_UNICAST_ADDPATH (8) | Yes | Yes | Yes | Yes | No |
| RIB_IPV4_MULTICAST_ADDPATH (9) | No | Yes | Yes | Yes | No |
| RIB_IPV6_UNICAST_ADDPATH (10) | Yes | Yes | Yes | Yes | No |
| RIB_IPV6_MULTICAST_ADDPATH (11) | No | Yes | Yes | Yes | No |
| RIB_GENERIC_ADDPATH (12) | No | No | Yes | No | No |

Notes:
- FRR dumps unicast only despite defining multicast sub-types.
- BIRD defines multicast and generic but does not emit them.
- OpenBGPD is the only daemon that writes RIB_GENERIC (VPN, flowspec).
- No daemon writes GEO_PEER_TABLE; only bgpkit-parser and GoBGP read it.

## BGP4MP Sub-types (Write)

| Sub-type | FRR | BIRD 3 | GoBGP | freeRtr | RustyBGP |
|----------|-----|--------|-------|---------|----------|
| STATE_CHANGE (0) | Yes | No | No | No | No |
| MESSAGE (1) | Yes | Yes | Yes | Yes | No |
| ENTRY (2) | No | No | No | No | No |
| SNAPSHOT (3) | No | No | No | No | No |
| MESSAGE_AS4 (4) | Yes | Yes | Yes | Yes | Yes |
| STATE_CHANGE_AS4 (5) | Yes | Yes | Defined | No | No |
| MESSAGE_LOCAL (6) | No | Yes | No | Yes | No |
| MESSAGE_AS4_LOCAL (7) | No | Yes | No | Yes | No |
| MESSAGE_ADDPATH (8) | Yes | Yes | No | Yes | No |
| MESSAGE_AS4_ADDPATH (9) | Yes | Yes | Yes | Yes | Yes |
| MESSAGE_LOCAL_ADDPATH (10) | Defined | Yes | No | Yes | No |
| MESSAGE_AS4_LOCAL_ADDPATH (11) | Defined | Yes | No | Yes | No |

Notes:
- FRR and BIRD have the broadest sub-type coverage for writing.
- OpenBGPD uses BGP4MP_ET (type 17) exclusively, not plain BGP4MP (type 16).
- RustyBGP supports only MESSAGE_AS4 and MESSAGE_AS4_ADDPATH.
- BIRD is the only implementation that writes all LOCAL variants.
- freeRtr writes both local and remote variants.

## Daemon Configuration Features

### Dump Modes

| Feature | FRR | BIRD 3 | OpenBGPD | GoBGP | freeRtr | RustyBGP |
|---------|-----|--------|----------|-------|---------|----------|
| Periodic RIB snapshot | Yes | Yes | Yes | Yes | Yes | No |
| Live update stream | Yes | Yes | Yes | Yes | Yes | Yes |
| State change recording | Yes (in "all") | Yes | Yes | No | No | No |
| Concurrent streams | 3 independent | RIB + global | Multiple | 1 per type | Per-peer | 1 |
| One-shot dump (no interval) | Yes | CLI command | No | No | No | No |

FRR runs three independent streams simultaneously: `all` (every BGP message),
`updates` (UPDATE only), and `routes-mrt` (periodic RIB). Each has its own
file and interval.

BIRD separates per-table periodic RIB dumps (protocol mrt) from a global
BGP4MP message stream (mrtdump). The CLI can trigger an on-demand RIB dump.

OpenBGPD offers six dump variants: `table`/`table-mp`/`table-v2` for RIB
snapshots, plus `all in`/`all out`/`updates in`/`updates out` per peer.

### Filtering and Selection

| Feature | FRR | BIRD 3 | OpenBGPD | GoBGP | freeRtr | RustyBGP |
|---------|-----|--------|----------|-------|---------|----------|
| Table/VRF selection | No | Yes (wildcard) | Yes (`dump rib <name>`) | Per-neighbor | No | No |
| Per-peer filtering | No | No | Yes (neighbor/group) | Per-neighbor table | Yes | No |
| Direction (in/out) | No | No | Yes | No | No | No |
| Route filter on dump | No | Yes (BIRD filters) | No | No | No | No |
| Address family selection | IPv4+IPv6 | Per-table | All AFI | All AFI | IPv4+IPv6 | IPv4+IPv6 |

OpenBGPD is the only daemon with direction filtering (in vs out). BIRD is the
only daemon that applies arbitrary route filters during a dump.

### File Management

| Feature | FRR | BIRD 3 | OpenBGPD | GoBGP | freeRtr | RustyBGP |
|---------|-----|--------|----------|-------|---------|----------|
| strftime patterns | Yes | Yes | Yes | No (Go time.Format) | No | No |
| Table name in filename | No | Yes (`%N`) | No | No | No | No |
| Interval-based rotation | Yes (day-aligned) | Yes | Yes | Yes (min 60s) | Yes | Yes |
| Auto-create directories | No | No | No | Yes | No | No |
| Buffered writes | No | No | No | Yes (1 MB) | No | No |
| Compression on write | No | No | No | No | No | No |

No daemon supports compressed MRT writing. GoBGP's `time.Format` patterns
serve the same purpose as strftime but with Go syntax.

### Extended Features

| Feature | FRR | BIRD 3 | OpenBGPD | GoBGP |
|---------|-----|--------|----------|-------|
| Extended timestamp (usec) | Yes (`all-et`/`updates-et`) | No | Yes (all updates) | No |
| Add-path aware | Yes (auto) | Yes (config) | Yes (auto) | Yes (auto) |
| View/instance name in peer table | Yes (BGP instance) | No | No | No |
| Fake peer for local routes | Yes (peer 0) | Yes (peer 0) | No | No |
| Incremental/yielding dump | No | Yes (2048 routes) | No | No |

## MRT Read-Side Features

### Parsing Capabilities

| Feature | Ze | FRR (bgp_btoa) | GoBGP | freeRtr | bgpkit-parser |
|---------|----|--------------------|-------|---------|---------------|
| TABLE_DUMP v1 | Yes | Yes | No | No | Yes |
| TABLE_DUMP_V2 | Yes | No | Yes | Yes | Yes |
| BGP4MP | Yes | Yes | Yes | Yes | Yes |
| BGP4MP_ET | Yes | Yes | Yes | No | Yes |
| IPv6 in BGP4MP | Yes | No | Yes | Yes | Yes |
| All add-path variants | Yes | No | Yes | Yes | Yes |
| RIB_GENERIC | Yes | No | Yes (defined) | No | Yes |
| GEO_PEER_TABLE | Yes | No | Yes (defined) | No | Yes |

Ze and bgpkit-parser have the most complete type coverage. FRR's bgp_btoa is a
minimal debug tool: IPv4-only BGP4MP, no TABLE_DUMP_V2.

### Read-Side Filtering

| Feature | Ze | GoBGP (inject) | freeRtr | bgpkit-parser |
|---------|-----|----------------|---------|---------------|
| By peer ASN | Yes (`--peer-asn`) | Yes (`--peer-asn`) | Yes (peer address) | Yes (peer_asn, peer_asns) |
| By peer IP | Yes (`--peer-ip`) | No | Yes (mrt2flt) | Yes (peer_ip, peer_ips) |
| By prefix | Yes (`--prefix`, RIB only) | No | No | Yes (exact/super/sub) |
| By origin ASN | No | No | No | Yes |
| By address family | No | Yes (`--no-ipv4`/`--no-ipv6`) | No | Yes (ip_version) |
| By AS-path | No | No | No | Yes (regex) |
| By community | No | No | No | Yes (regex) |
| By timestamp range | Yes (`--after`/`--before`) | No | No | Yes (ts_start/ts_end) |
| By type (announce/withdraw) | No | No | No | Yes |
| By MRT type | Yes (`--type`) | No | No | No |
| Filter negation | No | No | No | Yes (`!` prefix) |
| Best-path only | No | Yes (`--only-best`) | No | No |
| Write filtered MRT | Yes | No | Yes (mrtfilter output) | Yes (MrtRibEncoder/MrtUpdatesEncoder) |

bgpkit-parser's filter system is significantly more expressive than any daemon's.
Ze's filter writes matching records verbatim (no re-encoding), preserving ET
timestamps and all sub-type variants.

### Read-Side Actions

| Action | Ze | GoBGP | freeRtr | bgpkit-parser |
|--------|-----|-------|---------|---------------|
| Inject into local RIB | Yes | Yes (gRPC AddPathStream) | Yes (mrt2self) | No |
| Replay over BGP session | Yes | No | Yes (mrt2bgp) | No |
| Convert to BMP | No | No | Yes (mrt2bmp) | No |
| Convert to pcap | Yes | No | Yes (mrt2pcap) | No |
| Convert to text | Yes (`show`) | No | Yes (mrt2full, mrt2sum) | No |
| Convert text to MRT | No | No | Yes (txt2mrt) | No |
| Statistics/summary | Yes (`ze-analyse statistics`) | No | Yes (mrt2stat) | No |
| Policy-based filtering | Yes (`ze-analyse filter`) | No | Yes (mrtfilter) | No |
| Next-hop rewrite on inject | No | Yes (`--nexthop`) | No | No |
| Backpressure control | No | Yes (`--queue-size`) | No | No |
| Write filtered MRT | Yes (verbatim records) | No | Yes (mrtfilter output) | Yes (MrtRibEncoder/MrtUpdatesEncoder) |
| Compressed input | Yes (gz, bz2) | Yes (gz, bz2) | No | Yes (gz, bz2, zstd, lz4, xz) |
| HTTP/HTTPS URL input | No | No | No | Yes |

freeRtr has the richest MRT tooling. Ze has statistics, filtering, inject,
replay, show (human-readable dump), routes (prefix table extraction), and
convert (pcap, json) with compressed input. bgpkit-parser covers
read/filter/write as a library.

## Additional MRT-Adjacent Features

| Feature | Implementation | Notes |
|---------|---------------|-------|
| BMP-to-MRT server | freeRtr (servBmp2mrt) | Receives BMP, writes MRT files |
| MRT-to-BGP server | freeRtr (servMrt2bgp) | Serves MRT file contents over live BGP sessions |
| Unknown message collection | freeRtr | Collects unrecognized BGP messages to MRT dump |
| gRPC enable/disable | GoBGP, RustyBGP | Start/stop MRT dumps via API without config reload |

## Implementations With No MRT

| Implementation | Alternative |
|----------------|------------|
| ExaBGP | No production MRT. A 34-line experimental reader exists (dead code). Bundled pybgpdump is a third-party tool, not integrated. |
| bio-rd | No MRT at all. Uses gRPC streaming for RIB dumps and BMP for monitoring. |

## Summary: Feature Leaders by Category

| Category | Leader | Why |
|----------|--------|-----|
| Broadest write coverage | OpenBGPD | TABLE_DUMP v1 + v2, BGP4MP_ET, RIB_GENERIC, add-path, multicast, per-peer direction filtering |
| Most concurrent streams | FRR | Three independent streams (all/updates/routes) with separate files and intervals |
| Most configurable dumps | BIRD 3 | Table wildcards, route filters, on-demand CLI dump, `%N` filename, add-path toggle |
| Best API integration | GoBGP | gRPC enable/disable, MRT inject with filtering/nexthop-rewrite/backpressure |
| Richest tooling | freeRtr | Eight MRT tools (mrt2pcap, mrt2bmp, mrt2bgp, mrt2self, mrt2stat, mrt2sum, mrt2flt, mrtfilter, txt2mrt) plus BMP-to-MRT and MRT-to-BGP servers |
| Most complete parser | Ze, bgpkit-parser | Every RFC 6396/6397/8050 type/sub-type decoded, including GEO_PEER_TABLE, all add-path variants, TABLE_DUMP v1, BGP4MP_ET |
| Most complete encoder | Ze | Only implementation with tested buffer-first encoders for every MRT record type (TABLE_DUMP v1, TABLE_DUMP_V2 all subtypes, BGP4MP all subtypes, add-path, ET) |
| Direction filtering | OpenBGPD, Ze | In/out dump separation |
| Route filtering on dump | BIRD 3 | Only implementation applying arbitrary route filters during MRT export |

## Ze Status

Ze's MRT support spans three packages:

| Package | Status | Capabilities |
|---------|--------|-------------|
| `internal/mrt/` | Complete | Encode + decode all RFC 6396/6397/8050 types. BGP message deep decode (attributes, AS path, prefixes, communities). File writer with strftime rotation. File reader with gz/bz2 decompression. 16 round-trip tests + 18 BGP parse tests. |
| `internal/analyze/` | Complete | `statistics` (type/subtype/AFI/peer counts, timestamp range, BGP message types). `filter` (peer-ip, peer-asn, prefix, type, timestamp range, writes filtered MRT). `inject` (opens BGP session, sends TABLE_DUMP_V2/BGP4MP UPDATEs). `replay` (BGP4MP replay preserving timing, configurable speed). `convert pcap` (BGP4MP to pcap, IPv4). `convert json` (record headers as JSON). `show` (human-readable dump). `routes` (prefix table extraction as JSON). |
| `internal/component/mrt/` | Complete | YANG config (`ze-mrt-conf.yang`). Reactor MessageObserver for raw wire bytes. Async non-blocking writes (4096-record channel). Periodic TABLE_DUMP_V2 via cross-plugin RIB bridge (snapshot-under-lock). Per-peer and direction filtering. On-demand CLI dump (`request mrt dump-rib`). Extended timestamps. Add-path aware. State change recording. |

## RFCs Covered

| RFC | Topic | Relevant implementations |
|-----|-------|------------------------|
| RFC 6396 | MRT Routing Information Export Format | All except ExaBGP, bio-rd |
| RFC 6397 | MRT BGP RIB Geo Extensions | Ze (read), bgpkit-parser, GoBGP (read only) |
| RFC 8050 | MRT Add-Path Extensions | Ze, FRR, BIRD, OpenBGPD, GoBGP, freeRtr, RustyBGP, bgpkit-parser |
