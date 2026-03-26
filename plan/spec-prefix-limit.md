# Spec: prefix-limit

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 2/2 |
| Updated | 2026-03-25 |

## Task

Implement per-peer per-family prefix maximum, mandatory for every configured
peer. This is a prerequisite for `spec-forward-congestion` which uses the
prefix maximum value to size per-peer buffer allocations.

**Prefix maximum does double duty:**

| Purpose | How |
|---------|-----|
| Safety | Tear down session if peer exceeds maximum (catches route leaks, misconfig) |
| Buffer sizing | Tells the congestion system how large a burst to expect per peer |

## Implementation History

Phase 1 (enforcement) was completed in `plan/learned/413-prefix-limit.md`.
Phase 1.5 (PeeringDB data source) was completed in `plan/learned/415-prefix-data.md`.

Key design change from 415: the original data pipeline design (embedded
routing-data.json, zefs storage, source-url, `ze data prefix` commands)
was replaced by direct PeeringDB queries at runtime. No embedded data,
no zefs, no build pipeline.

## Industry Context

| Implementation | Prefix limit behavior |
|---------------|----------------------|
| Junos | `prefix-limit` per family. Teardown on exceed. Optional `teardown <idle-timeout>` for auto-restart. Warning at configurable threshold. |
| Cisco IOS-XR | `maximum-prefix <limit> [<threshold>] [warning-only / restart <minutes>]`. Per family. |
| BIRD | `import limit <N> [action warn / block / restart / disable]`. Per protocol (peer). |
| FRR | `neighbor X maximum-prefix <N> [<threshold>] [warning-only / restart <minutes>]`. Per family. |
| OpenBGPd | `max-prefix <N> [restart <minutes>]`. Per peer. |

All major implementations support this. It is expected by operators.

## Design

### Configuration

Prefix maximum is configured per family inside the `family { }` block.
One place for everything about a family. It is mandatory -- ze refuses
to start if a negotiated family has no prefix maximum.

YANG `family` is a list keyed by name (not a leaf-list). No migration
was needed -- the spec originally thought a structural change was required
but it was already correct.

**Inheritance:** The prefix maximum can be set at three levels.
More specific overrides less specific:

| Level | Scope | Example |
|-------|-------|---------|
| `bgp/family` | All peers (root default) | "every peer gets 100K ipv4 unless overridden" |
| `bgp/group/family` | All peers in this group | "IXP clients get 50K" |
| `bgp/peer/family` | This peer only | "this specific peer gets 500K" |

**YANG schema (implemented):**

| YANG path | Type | Default | Description |
|-----------|------|---------|-------------|
| `bgp/peer/family/<family>/prefix/maximum` | uint32 | (mandatory) | Hard maximum for this family |
| `bgp/peer/family/<family>/prefix/warning` | uint32 | 90% of maximum | Warning threshold. Optional. |
| `bgp/peer/prefix/teardown` | boolean | true | Tear down on exceed (false = warn only). Peer-level. |
| `bgp/peer/prefix/idle-timeout` | uint16 | 0 | Seconds before auto-reconnect after teardown (0 = no reconnect). Peer-level. |
| `bgp/peer/prefix/updated` | string | (hidden) | ISO date of last PeeringDB update. Set by update command. |

Every negotiated family MUST have a prefix maximum. Ze refuses to start
if a family block has no corresponding prefix maximum.

Warning defaults to 90% of maximum. Override per family if needed.

### Prefix Data Source: PeeringDB

Ze queries PeeringDB directly at runtime to suggest prefix maximum
values. There is no embedded data, no zefs storage, no build pipeline.

**PeeringDB settings** live in `system { peeringdb { } }` because
PeeringDB is an external service, not a BGP concept. Other subsystems
(e.g., IRR) can reuse the same client.

| YANG path | Type | Default | Description |
|-----------|------|---------|-------------|
| `system/peeringdb/url` | string | `https://www.peeringdb.com` | PeeringDB-compatible API base URL |
| `system/peeringdb/margin` | uint8 | 10 | Percentage to add on top of PeeringDB count |

**Update command:**

    ze update bgp peer <selector> prefix

| Command | Effect |
|---------|--------|
| `ze update bgp peer * prefix` | Update all peers from PeeringDB |
| `ze update bgp peer 10.0.0.* prefix` | Update only matching peers |
| `ze update bgp peer AS65001 prefix` | Update only peers with this ASN |

For each matched peer:
1. Queries PeeringDB for the peer's ASN
2. Applies margin (default 10%) to the returned counts
3. Updates ipv4/unicast and/or ipv6/unicast maximums in config
4. Sets the `updated` timestamp (hidden leaf)
5. Shows results: updated, skipped, or error per peer

The config is modified but NOT committed. The operator reviews the
diff and commits manually (`ze config commit`).

PeeringDB only provides ipv4 and ipv6 unicast counts. Other families
(VPN, flow, EVPN, etc.) must be set manually by the operator.

**Typical workflow:**

1. `ze update bgp peer * prefix` -- query PeeringDB, update configs
2. Review diff
3. `ze config commit` -- apply

### Prometheus Metrics (implemented)

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_bgp_prefix_count` | Gauge | peer, family | Current prefix count per family |
| `ze_bgp_prefix_maximum` | Gauge | peer, family | Configured hard maximum per family |
| `ze_bgp_prefix_warning` | Gauge | peer, family | Configured warning threshold per family |
| `ze_bgp_prefix_ratio` | Gauge | peer, family | current_count / maximum (0.0 to 1.0+) per family |
| `ze_bgp_prefix_warning_exceeded` | Gauge | peer, family | 1 if count >= warning for this family |
| `ze_bgp_prefix_maximum_exceeded_total` | Counter | peer, family | Times this family exceeded the maximum |
| `ze_bgp_prefix_teardown_total` | Counter | peer | Times session was torn down (peer-level, any family) |
| `ze_bgp_prefix_stale` | Gauge | peer | 1 if prefix updated timestamp is older than 6 months |

**Example Prometheus alert rules operators would write:**

| Alert | Expression | Meaning |
|-------|-----------|---------|
| Peer approaching maximum | `ze_bgp_prefix_ratio > 0.8` | 80% of maximum, investigate |
| Peer in warning zone | `ze_bgp_prefix_warning_exceeded == 1` | Operator-defined "too hot" |
| Peer growing fast | `rate(ze_bgp_prefix_count[5m]) > 1000` | Rapid prefix growth, possible leak |
| Repeated teardowns | `rate(ze_bgp_prefix_teardown_total[1h]) > 2` | Peer keeps getting torn down |
| Stale prefix data | `ze_bgp_prefix_stale == 1` | Source data needs refresh |

### Enforcement (implemented)

**Counting:** Track the number of prefixes received from each peer per
family. Increment on new NLRI, decrement on withdraw. The counter
reflects the current prefix count, not the total ever received.
Withdrawals are counted before announces in the same UPDATE to avoid
false triggers on prefix replacement.

**Two-threshold check:** After processing each UPDATE, compare current
count against both thresholds for the affected family:

| Count vs thresholds | Action |
|--------------------|--------|
| Below warning | Normal operation |
| At or above warning | Log warning (once per family). Set `ze_bgp_prefix_warning_exceeded=1`. |
| Exceeds maximum, teardown=true | Send NOTIFICATION (Cease, subcode 1: Maximum Number of Prefixes Reached, RFC 4486). Tear down session. Increment teardown counter. |
| Exceeds maximum, teardown=false | Log error. Set exceeded metric. Continue session. Do not accept further prefixes for that family beyond maximum. Withdrawals always processed. |

**NOTIFICATION format (RFC 4486 Section 4):**

| Field | Value |
|-------|-------|
| Error Code | 6 (Cease) |
| Error Subcode | 1 (Maximum Number of Prefixes Reached) |
| Data | AFI (2 bytes) + SAFI (1 byte) + prefix count (4 bytes) |

Enforcement runs in `processMessage()` before plugin delivery. Over-limit
routes never reach the RIB or get forwarded.

**Auto-reconnect:** If `idle-timeout` is non-zero, ze waits before
re-establishing the session. Uses exponential backoff on repeated
teardowns to prevent tight reconnect loops from persistent route leaks.

| Teardown count | Wait time |
|---------------|-----------|
| 1st | idle-timeout (configured value) |
| 2nd | idle-timeout x 2 |
| 3rd | idle-timeout x 4 |
| Nth | idle-timeout x 2^(N-1), capped at 1 hour |

The backoff counter resets when a session stays established for longer
than 5 minutes (stable session = transient problem resolved).

### Staleness Detection (implemented)

Per-peer `updated` timestamp tracks when PeeringDB was last consulted.
Staleness threshold is fixed at 180 days (not configurable).

| Channel | What |
|---------|------|
| Startup log | slog WARN for each peer with stale prefix data |
| Prometheus | `ze_bgp_prefix_stale` gauge per peer (1 if stale) |
| `show bgp peer X` | Shows `prefix-updated` date and `prefix-stale: true` |

### Advisory System (not yet implemented)

Ze never auto-changes the prefix maximum. The maximum is a safety
mechanism. Silently raising it could mask a route leak.

**Warning tracking:** When current prefix count reaches the warning
threshold, the peer is marked as having an active warning. This is
runtime state, reset on session restart.

**Visibility (planned):**

| Channel | How | Status |
|---------|-----|--------|
| Prometheus | `ze_bgp_prefix_warning_exceeded` gauge per peer/family | Done |
| Logs | slog WARN entry on threshold breach | Done |
| `show bgp peer <selector>` | Per-peer detail includes active warnings | Done |
| CLI status bar | On CLI login, bar shows warning count (like errors): "2 warnings" | Not yet implemented |
| `ze bgp warnings` | Lists all active warnings with peer, family, count, maximum | Not yet implemented |
| CLI staleness banner | On login, warns if prefix data > 6 months old | Not yet implemented |

The CLI warning banner and `ze bgp warnings` command are deferred to a
general CLI login warning system (not prefix-specific).

## Interaction with Forward Congestion

The prefix maximum value is consumed by `spec-forward-congestion` for
per-peer overflow buffer sizing:

| Prefix maximum | Burst estimate (10% of maximum) | Buffer share weight |
|---------------|-------------------------------|-------------------|
| 100K | 10K updates | High |
| 10K | 1K updates | Medium |
| 500 | 50 updates | Low |

The congestion system reads per-family prefix maximum from PeerSettings
and sums across families for the total per-peer buffer share.

## Current Behavior (as of phase 2)

**Implemented:**
- PeerSettings has PrefixMaximum, PrefixWarning maps, PrefixTeardown, PrefixIdleTimeout, PrefixUpdated
- Per-family prefix counter in session, reset on reconnect
- Enforcement check before plugin delivery in processMessage()
- YANG prefix container with per-family maximum/warning, peer-level teardown/idle-timeout/updated
- Config parsing in reactor/config.go (parsePrefixLimitFromFamily, parsePrefixSettingsFromTree)
- Mandatory validation: config rejected without prefix maximum per family
- PeeringDB client for runtime queries
- Prefix update command via RPC
- Auto-reconnect with exponential backoff
- 8 Prometheus metrics
- Staleness detection and startup warning

**Not implemented:**
- Editor autocompletion of prefix values from PeeringDB
- CLI login warning banner for stale prefix data
- `ze bgp warnings` command

## Data Flow

### Entry Point
- Config file: operator sets per-family `prefix { maximum N; }` per peer
- Or update command: `ze update bgp peer * prefix` queries PeeringDB, writes values

### Enforcement Path
1. Peer session established, per-family prefix counters initialized to 0
2. UPDATE received with N new NLRIs for family F
3. Withdrawals counted first (decrement)
4. Announces counted (increment)
5. After each UPDATE: compare counter to warning and maximum for F
6. If counter >= warning: log warning (once), set Prometheus metric
7. If counter > maximum AND teardown=true: send NOTIFICATION, close session
8. If counter > maximum AND teardown=false: log error, drop further UPDATEs for F

### Data Update Path
1. Operator runs `ze update bgp peer * prefix`
2. For each peer: query PeeringDB for ASN's ipv4/ipv6 counts
3. Apply margin (default 10%), write new maximum to config
4. Set `updated` timestamp
5. Operator runs `ze config commit` to apply

### Boundaries Crossed
| Boundary | How |
|----------|-----|
| Config -> PeerSettings | YANG tree extraction in reactor/config.go |
| UPDATE -> prefix counter | Increment/decrement per family in session_prefix.go |
| Counter -> NOTIFICATION | Enforcement check in session_read.go processMessage() |
| Warning -> log | Structured slog entry on threshold breach |
| PeeringDB -> config | HTTP query in peeringdb/client.go, written by prefix_update.go |
| PeerSettings -> congestion system | Read PrefixMaximum for buffer sizing |

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with prefix maximum, peer exceeds for one family | -> | NOTIFICATION sent, session torn down | test/plugin/prefix-maximum-enforce.ci |
| Config with prefix maximum, peer at warning | -> | Warning logged, Prometheus metric set | test/plugin/prefix-maximum-warning.ci |
| Config without prefix maximum for negotiated family | -> | Config rejected at parse time | test/parse/prefix-maximum-required.ci |
| `ze update bgp peer * prefix` | -> | Values updated from PeeringDB | test/plugin/api-peer-prefix-update.ci |
| Stale prefix data at startup | -> | Warning logged, Prometheus metric set | test/plugin/prefix-stale-warning.ci |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Status |
|-------|-------------------|-------------------|--------|
| AC-1 | Family config has `ipv4/unicast { prefix { maximum 1000000; } }` | Per-family maximum = 1000000, warning = 900000 (default 90%) | Done (413) |
| AC-2 | Family block has no prefix maximum | Config parse error, ze refuses to start | Done (413) |
| AC-3 | Peer sends prefixes exceeding family maximum (teardown=true) | NOTIFICATION Cease/MaxPrefixes sent, session torn down | Done (413) |
| AC-4 | Peer sends prefixes exceeding family maximum (teardown=false) | Error logged, further prefixes for that family rejected, session stays | Done (413) |
| AC-5 | Peer prefix count reaches family warning | Warning logged (slog), `ze_bgp_prefix_warning_exceeded{family=X}` set to 1 | Done (413) |
| AC-6 | `ze update bgp peer * prefix` queries PeeringDB | ipv4/ipv6 maximums updated with margin, `updated` timestamp set | Done (415) |
| AC-7 | `prefix update` for ASN not in PeeringDB | Peer skipped with error in results, other peers still updated | Done (415) |
| AC-8 | PeeringDB returns zero for both ipv4/ipv6 | Peer skipped as suspicious (zero counts likely means no data) | Done (415) |
| AC-9 | Peer withdraws prefixes below family warning | Counter decremented, `ze_bgp_prefix_warning_exceeded{family=X}` set back to 0 | Done (413) |
| AC-10 | Session reset after teardown | All family prefix counters reset to 0, all metrics reset | Done (413) |
| AC-11 | `prefix { idle-timeout 30; }` after teardown | Session re-establishes after 30 seconds | Done (413) |
| AC-12 | Stale prefix data at startup | slog WARN for each peer with stale data, `ze_bgp_prefix_stale` metric set | Done (415) |
| AC-13 | Prometheus metrics exposed | 8 prefix metrics with peer and family labels | Done (413+415) |
| AC-14 | PrefixMaximum per family available to congestion system | spec-forward-congestion reads per-family maximum for buffer sizing | Done (413) |
| AC-15 | Explicit warning > maximum for a family | Config parse error: warning must be less than maximum | Done (413) |
| AC-16 | No warning configured, family maximum is 1000000 | Warning auto-set to 900000 (90% of maximum) | Done (413) |
| AC-17 | Peer has ipv4/unicast and ipv6/vpn, only ipv6/vpn exceeds | Only ipv6/vpn triggers enforcement; ipv4/unicast unaffected | Done (413) |
| AC-18 | PeeringDB URL overridden in `system { peeringdb { url; } }` | Update command uses custom URL | Done (415) |
| AC-19 | PeeringDB margin set to 20 | Update command applies 20% margin instead of default 10% | Done (415) |
| AC-20 | `show bgp peer X` with stale data | Shows `prefix-updated` date and `prefix-stale: true` | Done (415) |
| AC-21 | Peer has VPN family but PeeringDB only has ipv4/ipv6 | VPN family maximum must be set manually, error if missing | Done (413) |
| AC-25 | Repeated teardowns with idle-timeout | Exponential backoff: idle-timeout x 2^(N-1), capped at 1 hour | Done (413) |
| AC-26 | Session stable for 5+ minutes after reconnect | Backoff counter resets | Done (413) |
| AC-27 | teardown=false, peer exceeds maximum | UPDATEs for that family still processed but NLRIs beyond maximum are not installed in RIB or forwarded. Withdrawals always processed. Session stays up. | Done (413) |

## Files Modified (phase 1: 413)

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- prefix container in family list
- `internal/component/bgp/reactor/peersettings.go` -- PrefixMaximum/Warning/Teardown/IdleTimeout/Updated fields
- `internal/component/bgp/reactor/config.go` -- parsePrefixLimitFromFamily, parsePrefixSettingsFromTree
- `internal/component/bgp/reactor/peer.go` -- auto-reconnect with exponential backoff
- `internal/component/bgp/reactor/session_prefix.go` -- counting, enforcement, NOTIFICATION, metrics
- `internal/component/bgp/reactor/session_read.go` -- prefix check before plugin delivery
- `internal/component/bgp/reactor/reactor_metrics.go` -- prefix metric registrations
- `internal/component/config/parser_list.go` -- block entries in inline list blocks

## Files Modified (phase 1.5: 415)

- `internal/component/bgp/peeringdb/client.go` -- PeeringDB HTTP client
- `internal/component/bgp/plugins/cmd/peer/prefix_update.go` -- update command handler
- `internal/component/bgp/reactor/session_prefix.go` -- IsPrefixDataStale, staleness metric
- `internal/component/bgp/reactor/session_connection.go` -- graceful TCP close
- `internal/component/bgp/reactor/reactor_peers.go` -- startup staleness warning
- `internal/component/config/system/system.go` -- PeeringDB URL/margin extraction
- `internal/component/config/system/schema/ze-system-conf.yang` -- peeringdb container
- `internal/component/plugin/types.go` -- PeerInfo.PrefixUpdated field
- `cmd/ze-test/peeringdb.go` -- fake PeeringDB server for tests

## Functional Tests

- `test/plugin/prefix-maximum-enforce.ci` -- enforcement (teardown + drop modes)
- `test/plugin/prefix-maximum-warning.ci` -- warning threshold behavior
- `test/parse/prefix-maximum-required.ci` -- mandatory validation
- `test/plugin/api-peer-prefix-update.ci` -- PeeringDB update command
- `test/plugin/prefix-stale-warning.ci` -- staleness warning

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add prefix maximum |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- add prefix { } syntax |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- `ze update bgp peer * prefix` |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No (covered in configuration guide) | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc4486.md` -- note prefix maximum enforcement |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- ze now has prefix maximum |
| 12 | Internal architecture changed? | No | |
