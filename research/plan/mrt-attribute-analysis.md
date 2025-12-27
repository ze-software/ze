# MRT Attribute Analysis Tool

**Goal:** Analyze MRT dumps to measure BGP attribute repetition rates for caching optimization decisions.

**Location:** `research/cmd/attribute-analyser/main.go`

---

## Motivation

ZeBGP excludes AS_PATH from attribute deduplication (believed too volatile). We hypothesize other attributes like LOCAL_PREF, MED, and COMMUNITY sets may be highly repetitive (per-session route-map additions). This tool validates these assumptions with real data.

---

## Statistics to Gather

### 1. Per-Attribute Uniqueness

For each attribute type, track:
- Total occurrences
- Unique values (by content hash)
- Bytes used
- Cache hit rate: `(total - unique) / total`

**Attributes to track:**
| Code | Attribute | Expected Repetition |
|------|-----------|---------------------|
| 1 | ORIGIN | Very high (only 3 values exist) |
| 2 | AS_PATH | Unknown (believed low) |
| 3 | NEXT_HOP | Moderate (few egress points) |
| 4 | MED | High (route-map set) |
| 5 | LOCAL_PREF | Very high (policy-based) |
| 6 | ATOMIC_AGGREGATE | High (flag only) |
| 7 | AGGREGATOR | Moderate |
| 8 | COMMUNITY | High (route-map additions) |
| 9 | ORIGINATOR_ID | High (per-RR) |
| 10 | CLUSTER_LIST | High (per-RR path) |
| 14 | MP_REACH_NLRI | Low (contains NLRI) |
| 15 | MP_UNREACH_NLRI | Low (contains NLRI) |
| 16 | EXTENDED_COMMUNITY | High (RT/SOO sets) |
| 32 | LARGE_COMMUNITY | High (route-map additions) |

### 2. Bundle Analysis

Track unique combinations of attributes **excluding AS_PATH**:
- Hash the concatenated bytes of all non-AS_PATH attributes
- Measures potential for caching entire attribute bundles

### 3. Per-Peer Analysis

For TABLE_DUMP_V2, track peer_index to answer:
- Do attributes repeat within a peer (session-specific caching)?
- Or do they repeat across peers (global caching)?

Compare:
- Per-peer unique bundles vs total bundles (session hit rate)
- Global unique bundles vs total bundles (global hit rate)

### 4. Per-Peer Community Extraction Analysis

**Goal:** Identify communities that could be stored once per-session with "negative" markers for absence.

**Hypothesis:** Route-maps often add the same communities to all routes from a peer. If community X appears in 95% of updates from peer Y, we could:
1. Store X once per session as "default community"
2. Only encode absence (negative community) for the 5% that don't have it

**Track for each peer:**
- Individual COMMUNITY values (4 bytes each)
- Individual LARGE_COMMUNITY values (12 bytes each)
- Individual EXTENDED_COMMUNITY values (8 bytes each)
- Frequency: `count / total_updates_from_peer`

**Identify "session-constant" communities:**
- Communities appearing in >90% of a peer's updates
- These are candidates for per-session storage

**Output:**
```
PER-PEER COMMUNITY ANALYSIS:
  Peer 0 (123,456 updates):
    Session-constant communities (>90%):
      65000:100    100.0% (123,456/123,456) ← always present
      65000:200     99.8% (123,210/123,456)
      65000:300     95.2% (117,510/123,456)
    Variable communities (<90%):
      1234:5678     45.2% (55,803/123,456)
      ...

    Savings estimate:
      3 communities * 4 bytes * 123,456 = 1.4 MB
      vs storing once + 5% negative markers = 24 KB
      → 98% savings on these communities

  Peer 1 (89,234 updates):
    ...
```

**JSON addition:**
```json
"community_extraction": {
  "0": {
    "updates": 123456,
    "session_constant": [
      {"community": "65000:100", "frequency": 1.0, "count": 123456},
      {"community": "65000:200", "frequency": 0.998, "count": 123210}
    ],
    "variable": [
      {"community": "1234:5678", "frequency": 0.452, "count": 55803}
    ],
    "potential_savings_bytes": 1425408
  }
}
```

---

## Output Format

### JSON Structure

```json
{
  "file": "rib.20251220.0000.gz",
  "total_updates": 847234,
  "attributes": {
    "ORIGIN": {
      "code": 1,
      "total": 847234,
      "unique": 3,
      "bytes": 847234,
      "cache_hit_rate": 0.999996
    },
    "AS_PATH": {
      "code": 2,
      "total": 847234,
      "unique": 234567,
      "bytes": 45234567,
      "cache_hit_rate": 0.723
    }
  },
  "bundles": {
    "total": 847234,
    "unique": 12345,
    "cache_hit_rate": 0.985
  },
  "per_peer": {
    "0": {
      "updates": 123456,
      "unique_bundles": 234,
      "cache_hit_rate": 0.998
    }
  },
  "bytes_by_attribute": {
    "AS_PATH": 45234567,
    "COMMUNITY": 23456789
  }
}
```

### Human Summary

```
=== MRT Attribute Analysis ===
File: rib.20251220.0000.gz
Total UPDATEs: 847,234

ATTRIBUTE REPETITION (sorted by cache hit rate):
  Attr            Unique        Total   Hit Rate      Bytes
  ─────────────────────────────────────────────────────────
  ORIGIN               3      847,234    99.999%    847 KB
  LOCAL_PREF          47      845,102    99.994%    3.2 MB
  ATOMIC_AGG           2       12,456    99.984%     12 KB
  MED                892      623,451    99.857%    2.4 MB
  NEXT_HOP         1,247      847,234    99.853%    3.2 MB
  COMMUNITY        3,892      567,234    99.314%   23.5 MB
  LARGE_COMM       1,234      234,567    99.474%    8.9 MB
  CLUSTER_LIST       456       89,234    99.489%    712 KB
  AGGREGATOR       2,345      123,456    98.100%    987 KB
  AS_PATH        234,567      847,234    72.310%   45.2 MB  ← verify volatility

BYTES DISTRIBUTION:
  AS_PATH:        45.2 MB (52.3%)
  COMMUNITY:      23.5 MB (27.2%)
  LARGE_COMM:      8.9 MB (10.3%)
  NEXT_HOP:        3.2 MB (3.7%)
  ...

BUNDLE ANALYSIS (all attrs except AS_PATH):
  Unique bundles:  12,345 / 847,234 total
  Global cache hit rate: 98.54%

PER-PEER STATISTICS (top 10 by volume):
  Peer      Updates    Unique    Hit Rate
  ────────────────────────────────────────
     0      123,456       234     99.81%
     1       89,234       567     99.36%
     2       78,901       890     98.87%
  ...

INSIGHTS:
  - AS_PATH: 72.3% cache hit (better than expected, worth caching?)
  - Per-peer avg hit rate: 99.2% vs global 98.5%
    → Session-specific caching provides marginal benefit
```

---

## Implementation Steps

### Phase 1: Core Parsing

1. Copy MRT reading code from `research/cmd/mrt-dump/main.go`
2. Add attribute extraction from UPDATE body
3. Parse each attribute: type code, flags, length, value bytes

### Phase 2: Statistics Collection

1. Create hash maps for each attribute type: `map[hash]count`
2. Create bundle hash: concatenate non-AS_PATH attribute bytes, hash
3. Track per-peer stats via `map[peerIndex]PeerStats`

### Phase 3: Output

1. Compute derived stats (hit rates, percentages)
2. Output JSON to stdout or file
3. Output human summary to stderr

---

## Data Structures

```go
type Stats struct {
    File         string
    TotalUpdates uint64

    // Per-attribute: typeCode -> AttrStats
    Attributes map[uint8]*AttrStats

    // Bundle stats (excl AS_PATH)
    Bundles BundleStats

    // Per-peer stats
    Peers map[uint16]*PeerStats
}

type AttrStats struct {
    Code       uint8
    Name       string
    Total      uint64
    Unique     uint64           // len(Values) at end
    Bytes      uint64
    Values     map[uint64]uint64 // hash -> count
}

type BundleStats struct {
    Total  uint64
    Values map[uint64]uint64 // hash -> count
}

type PeerStats struct {
    Updates       uint64
    UniqueBundles uint64
    Bundles       map[uint64]uint64 // hash -> count

    // Community extraction analysis
    Communities     map[uint32]uint64  // community value -> count
    LargeCommunities map[[12]byte]uint64 // large community -> count
    ExtCommunities  map[uint64]uint64  // ext community -> count
}

type CommunityFreq struct {
    Value     string  // "65000:100" format
    RawValue  any     // uint32, [12]byte, or uint64
    Count     uint64
    Frequency float64 // count / total_updates
}
```

---

## Usage

```bash
# Analyze single file
./attribute-analyser rib.20251220.0000.gz

# Analyze multiple files (aggregates stats)
./attribute-analyser rib.*.gz > analysis.json 2> summary.txt

# Pretty print JSON
./attribute-analyser rib.gz | jq .
```

---

## Success Criteria

1. Tool runs on RouteViews/RIPE RIS RIB dumps
2. JSON output parseable, complete
3. Human summary readable, insightful
4. Memory usage reasonable (<4GB for full table dumps)

---

## Open Questions for Analysis

After running tool:

1. Is AS_PATH caching worth it? (threshold: >80% hit rate)
2. Per-session vs global: significant difference?
3. Which attributes provide best bytes-saved-per-cache-entry?
4. Should we cache NEXT_HOP separately (often repeated)?
5. How many communities are session-constant (>90% per-peer)?
6. Are LOCAL_PREF and MED also session-constant (route-map set)?
7. Negative community encoding: worth the complexity?
