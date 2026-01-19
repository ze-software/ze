# MRT Data Sources for BGP Testing

## MRT Format Types

### TABLE_DUMP_V2 (Type 13) - RIB Dumps

- Snapshot of routing table at a point in time
- Routes organized by prefix, NOT arrival order
- No temporal locality preserved
- Files: `bview.*.gz`, `rib.*.gz`

### BGP4MP (Type 16) - Live Updates

- BGP messages in chronological order
- Preserves temporal locality and burst patterns
- Includes UPDATE, KEEPALIVE, state changes
- Files: `updates.*.gz`

## RIPE RIS (Routing Information Service)

<https://data.ris.ripe.net/>

### RIB Dumps (TABLE_DUMP_V2)

- 20+ global collectors
- Every 8 hours
- Example: <https://data.ris.ripe.net/rrc00/latest-bview.gz>

### BGP4MP Updates

- Every 5 minutes
- Example: <https://data.ris.ripe.net/rrc00/2025.12/updates.20251222.0000.gz>

### Collectors

| Collector | Location |
|-----------|----------|
| rrc00 | Amsterdam (multi-hop) |
| rrc01 | London (LINX) |
| rrc03 | Amsterdam (AMS-IX) |
| rrc04 | Geneva (CIXP) |

Full list: <https://www.ripe.net/analyse/internet-measurements/routing-information-service-ris>

## RouteViews (University of Oregon)

<http://archive.routeviews.org/>

### RIB Dumps (TABLE_DUMP_V2)

- Multiple collectors
- Every 2 hours (varies by collector)
- Example: <http://archive.routeviews.org/bgpdata/2025.12/RIBS/rib.20251201.0000.bz2>

### BGP4MP Updates

- Every 15 minutes
- Example: <http://archive.routeviews.org/bgpdata/2025.12/UPDATES/updates.20251201.0000.bz2>

Notable peers: AS6447, AS3356 (Lumen), etc.

## Isolario

<https://www.isolario.it/Isolario_MRT_dumps/>

European collector with full feeds.

## Quick Download

Use `download.sh` for automatic recompression to gzip -9:

```bash
./download.sh                # today @ 0000
./download.sh 20251220       # specific date
./download.sh 20251220 1200  # specific date+time
```

Manual download examples:

```bash
curl -O https://data.ris.ripe.net/rrc00/latest-bview.gz
curl -O https://data.ris.ripe.net/rrc00/2025.12/updates.20251222.0000.gz
curl -O http://archive.routeviews.org/bgpdata/2025.12/RIBS/rib.20251201.0000.bz2
curl -O http://archive.routeviews.org/bgpdata/2025.12/UPDATES/updates.20251201.0000.bz2
```

## File Naming Convention

**RIB dumps:**

| File | Source |
|------|--------|
| `latest-bview.gz` | RIPE latest (symlink) |
| `rib.YYYYMMDD.HHMM.gz` | RouteViews RIB |

**BGP4MP updates:**

| File | Source |
|------|--------|
| `ripe-updates.YYYYMMDD.HHMM.gz` | RIPE updates |
| `rv-updates.YYYYMMDD.HHMM.gz` | RouteViews updates |

## Usage with ZeBGP

ZeBGP reads MRT directly via `internal/mrt`:

- TABLE_DUMP_V2: Full support
- BGP4MP: Supported for UPDATE replay

Current full table: ~1M IPv4 + ~250K IPv6 prefixes

## Temporal Locality

RIB dumps do **NOT** preserve arrival order. Routes are organized by prefix.
For realistic replay testing with burst patterns, use BGP4MP updates.

Real BGP sessions exhibit:

- Burst patterns (many routes from same peer)
- Prefix clustering (adjacent prefixes together)
- Temporal locality that aids caching

| Replay Type | Pattern |
|-------------|---------|
| MRT RIB | Worst-case random access |
| BGP4MP | Realistic temporal patterns |
