# Spec: prefix-limit

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `internal/component/bgp/reactor/peersettings.go` - PeerSettings struct
3. `internal/component/bgp/reactor/peer.go` - Peer struct, session lifecycle
4. `internal/component/bgp/message/notification.go` - NotifyCeaseMaxPrefixes
5. `internal/component/bgp/schema/ze-bgp-conf.yang` - BGP config schema

## Task

Implement per-peer per-family prefix maximum, mandatory for every configured
peer. This is a prerequisite for `spec-forward-congestion` which uses the
prefix maximum value to size per-peer buffer allocations.

**Prefix maximum does double duty:**

| Purpose | How |
|---------|-----|
| Safety | Tear down session if peer exceeds maximum (catches route leaks, misconfig) |
| Buffer sizing | Tells the congestion system how large a burst to expect per peer |

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

**YANG structural change required:** Currently `family` is a leaf-list
of strings (e.g., `family { ipv4/unicast; ipv6/unicast; }`). To add
per-family config like `prefix { maximum N; }`, each family entry must
become a YANG list node keyed by family name, with child containers.

| Before | After |
|--------|-------|
| `leaf-list family { type string; }` | `list family { key name; leaf name; container prefix { ... } }` |
| `family { ipv4/unicast; }` | `family { ipv4/unicast { prefix { maximum N; } } }` |

This is a config format change. Existing configs with the string
format must be migrated. The config migration tool should handle this
automatically (add empty `prefix { }` blocks, or fail-with-message
asking operator to add prefix maximums).

**Inheritance:** The prefix maximum can be set at three levels.
More specific overrides less specific:

| Level | Scope | Example |
|-------|-------|---------|
| `bgp/family` | All peers (root default) | "every peer gets 100K ipv4 unless overridden" |
| `bgp/group/family` | All peers in this group | "IXP clients get 50K" |
| `bgp/peer/family` | This peer only | "this specific peer gets 500K" |

    bgp {
        family {
            ipv4/unicast { prefix { maximum 100000; } }
            ipv6/unicast { prefix { maximum 20000; } }
        }

        group ixp-clients {
            family {
                ipv4/unicast { prefix { maximum 50000; } }
            }
        }

        peer client-a {
            group ixp-clients;
            remote { ip 10.0.0.1; as 65001; }
            # inherits: ipv4 50K from group, ipv6 20K from root
        }

        peer big-client {
            group ixp-clients;
            remote { ip 10.0.0.2; as 65002; }
            family {
                ipv4/unicast { prefix { maximum 500000; } }
            }
            # ipv4 500K from peer, ipv6 20K from root
        }
    }

**YANG schema:**

| YANG path | Type | Default | Description |
|-----------|------|---------|-------------|
| `bgp/peer/family/<family>/prefix/maximum` | uint32 | (mandatory) | Hard maximum for this family |
| `bgp/peer/family/<family>/prefix/warning` | uint32 | 90% of maximum | Warning threshold. Optional. |
| `bgp/peer/prefix/teardown` | boolean | true | Tear down on exceed (false = warn only). Peer-level. |
| `bgp/peer/prefix/idle-timeout` | uint16 | 0 | Seconds before auto-reconnect after teardown (0 = no reconnect). Peer-level. |
| `bgp/prefix/source-url` | string | (ze default) | URL for prefix data. Defaults to ze's codeberg repository. Overridable for private mirrors. Global to BGP. |

The `source-url` is a global BGP setting. Ze is the only data source
for prefix maximums. The data ze ships may be built from PeeringDB,
routing table snapshots, or other sources -- but the operator always
gets it from ze (or a mirror at source-url). There is no direct
PeeringDB query at runtime.

Every negotiated family MUST have a prefix maximum. Ze refuses to start
if a family block has no corresponding prefix maximum.

Peers whose maximum was typed manually vs resolved from ze data
are tracked via a visible `source` field in the config. This is
NOT hidden metadata -- the operator can see and change it.

    family {
        ipv4/unicast { prefix { maximum 500000; source manual; } }
        ipv6/unicast { prefix { maximum 50000; source ze; } }
    }

| YANG path | Type | Default | Description |
|-----------|------|---------|-------------|
| `prefix/source` | enum (manual, ze) | ze | How this maximum was set. Visible in config. |

| Source | Meaning | `update` behavior |
|--------|---------|------------------|
| `ze` | Value came from ze local data (autocomplete) | Updated by `ze bgp peer * prefix maximum update` |
| `manual` | Operator typed the number | Skipped by `update` |

When the operator accepts autocomplete, `source ze;` is written.
When the operator types a number directly, `source manual;` is written.
The operator can change source at any time.

The JSON data blob contains per-ASN entries with IPv4 and IPv6 counts.
When resolving, ze sets all families that have data. Families not in
the data (e.g., VPN, flow) must be set manually by the operator.

**Config syntax:**

    peer client-a {
        remote { ip 10.0.0.1; as 65001; }
        family {
            ipv4/unicast { prefix { maximum 1000000; } }
            ipv6/unicast { prefix { maximum 50000; } }
            ipv6/vpn     { prefix { maximum 500; } }
        }
    }

Source defaults to `manual` when a number is typed directly. When the
value is autocompleted from ze data, the peer is marked auto-resolved.

Warning defaults to 90% of maximum. Override per family if needed:

    peer client-a {
        family {
            ipv4/unicast { prefix { warning 800000; maximum 1000000; } }
            ipv6/unicast { prefix { maximum 50000; } }
        }
    }

**Autocompletion:** When the operator is editing the config and types
`maximum` for a family, ze offers autocompletion:

| Input | What happens | source |
|-------|-------------|--------|
| Autocomplete suggestion | Ze proposes value from local data, operator accepts | `ze` |
| `update` keyword | Ze replaces with current value from local data | `ze` |
| Explicit number | Operator types the number | `manual` |

`update` is a convenience: the operator types `maximum update;` and
ze resolves it to the current value from `meta/prefix/limit` in zefs.
Same result as accepting the autocomplete, but works in any text
editor or scripted config generation -- not just the interactive
CLI editor.

The config file always contains a concrete number. `update` is
never stored -- it is resolved at config parse/commit time.

**The `source-url` field:** Controls where ze fetches its prefix data.

| Value | Meaning |
|-------|---------|
| (default) | Ze's codeberg repository (the public data) |
| Custom URL | Private mirror or internal data source |

This allows organizations to host their own prefix data internally
instead of depending on ze's public repository.

**Example configs:**

| Peer type | Family | warning | maximum | Meaning |
|----------|--------|---------|---------|---------|
| Full table | ipv4/unicast | 900000 | 1000000 | Operator-set (manual) |
| Full table | ipv6/unicast | 180000 | 200000 | From ze data (auto-updatable) |
| IXP RS client | ipv4/unicast | 45000 | 50000 | From ze data (auto-updatable) |
| L3VPN peer | ipv6/vpn | 180 | 200 | Operator-set (manual, not in ze data) |

### Prefix Data in zefs

Ze does NOT ship a `data/` directory in the repository. All prefix data
is stored in zefs under `meta/peer/prefix/`.

**Data lifecycle:**

1. A build script analyzes PeeringDB + routing table snapshots (RouteViews,
   RIPE RIS) and produces a JSON file with per-ASN per-family prefix counts
2. This JSON is embedded in the ze binary at build time
3. On software update (first run of new version), ze writes the embedded
   data to `zefs meta/prefix/limit`
4. If zefs already has newer data (from a previous manual update), it is
   kept -- embedded data only overwrites if it is newer
5. `ze data prefix update` fetches latest from `source-url` into zefs
6. `ze data prefix import <file>` imports operator-provided JSON into zefs

**JSON format (documented):**

The `routing-data.json` file contains per-ASN per-family prefix counts:

    {
      "version": 1,
      "date": "2026-03-22",
      "source": "peeringdb+routeviews",
      "asns": {
        "65001": {
          "ipv4/unicast": 85000,
          "ipv6/unicast": 12000,
          "ipv4/vpn": 500
        },
        "65002": {
          "ipv4/unicast": 920000,
          "ipv6/unicast": 180000
        }
      }
    }

| Field | Type | Description |
|-------|------|-------------|
| `version` | int | Schema version (currently 1) |
| `date` | string | ISO date when this data was generated |
| `source` | string | What data sources were used |
| `asns` | object | Map of ASN (string) to family prefix counts |
| `asns/<asn>/<family>` | int | Expected prefix count for this ASN and family |

**zefs storage location:** `meta/prefix/limit`

**Resolution order:**

| Step | Source | Network access? |
|------|--------|----------------|
| 1 | zefs `meta/prefix/limit` | No |
| 2 | Embedded data in binary (if zefs missing or older) | No |
| 3 | Fetch from source-url via `ze data prefix update` | Yes (ze repository, not PeeringDB) |

### Prometheus Metrics for Monitoring

The point of prefix warning is to be actionable in Prometheus. Operators
must be able to build alerts that fire before sessions tear down.

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_bgp_prefix_count` | Gauge | peer, asn, family | Current prefix count per family |
| `ze_bgp_prefix_maximum` | Gauge | peer, asn, family | Configured hard maximum per family |
| `ze_bgp_prefix_warning` | Gauge | peer, asn, family | Configured warning threshold per family |
| `ze_bgp_prefix_ratio` | Gauge | peer, asn, family | current_count / maximum (0.0 to 1.0+) per family |
| `ze_bgp_prefix_warning_exceeded` | Gauge | peer, asn, family | 1 if count >= warning for this family |
| `ze_bgp_prefix_maximum_exceeded_total` | Counter | peer, asn, family | Times this family exceeded the maximum |
| `ze_bgp_prefix_teardown_total` | Counter | peer, asn | Times session was torn down (peer-level, any family) |
| `ze_bgp_prefix_data_stale` | Gauge | peer, asn, family | 1 if source data > 6 months old |
| `ze_bgp_prefix_data_age_days` | Gauge | peer, asn, family | Days since source data was resolved |
| `ze_bgp_prefix_suggestion` | Gauge | peer, asn, family | Suggested new maximum (0 if none) |

**Per-peer BGP message counters** (general, not prefix-specific, but
needed for operational visibility alongside prefix metrics):

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_bgp_updates_received_total` | Counter | peer, asn | UPDATE messages received |
| `ze_bgp_updates_sent_total` | Counter | peer, asn | UPDATE messages sent |
| `ze_bgp_notifications_received_total` | Counter | peer, asn, code, subcode | NOTIFICATION messages received |
| `ze_bgp_notifications_sent_total` | Counter | peer, asn, code, subcode | NOTIFICATION messages sent |

**Example Prometheus alert rules operators would write:**

| Alert | Expression | Meaning |
|-------|-----------|---------|
| Peer approaching maximum | `ze_bgp_prefix_ratio > 0.8` | 80% of maximum, investigate |
| Peer in warning zone | `ze_bgp_prefix_warning_exceeded == 1` | Operator-defined "too hot" |
| Peer growing fast | `rate(ze_bgp_prefix_count[5m]) > 1000` | Rapid prefix growth, possible leak |
| Repeated teardowns | `rate(ze_bgp_prefix_teardown_total[1h]) > 2` | Peer keeps getting torn down |
| Stale prefix data | `ze_bgp_prefix_data_stale == 1` | Source data needs refresh |

### Enforcement

**Counting:** Track the number of prefixes received from each peer per
family. Increment on new NLRI, decrement on withdraw. The counter
reflects the current prefix count, not the total ever received.

**Two-threshold check:** After processing each UPDATE, compare current
count against both thresholds for the affected family:

| Count vs thresholds | Action |
|--------------------|--------|
| Below warning | Normal operation |
| At or above warning | Log warning. Set `ze_bgp_prefix_warning_exceeded=1`. Log warning. |
| Exceeds maximum, teardown=true | Send NOTIFICATION (Cease, subcode 1: Maximum Number of Prefixes Reached, RFC 4486). Tear down session. Increment teardown counter. |
| Exceeds maximum, teardown=false | Log error. Set exceeded metric. Continue session. Do not accept further prefixes for that family beyond maximum. |

**NOTIFICATION format (RFC 4486 Section 4):**

| Field | Value |
|-------|-------|
| Error Code | 6 (Cease) |
| Error Subcode | 1 (Maximum Number of Prefixes Reached) |
| Data | AFI (2 bytes) + SAFI (1 byte) + prefix count (4 bytes) |

Include the prefix count in the data field -- helps the remote operator
debug which family exceeded and by how much.

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

### Advisory System

Ze never auto-changes the prefix maximum. The maximum is a safety
mechanism. Silently raising it could mask a route leak.

**Warning tracking:** When current prefix count reaches the warning
threshold, the peer is marked as having an active warning. This is
runtime state (not persisted to zefs), reset on session restart.

**Visibility:**

| Channel | How |
|---------|-----|
| CLI status bar | On CLI login, bar shows warning count (like errors): "2 warnings" |
| `ze bgp warnings` | Lists all active warnings with peer, family, count, maximum |
| `ze bgp peer <selector> show` | Per-peer detail includes active warnings |
| `ze bgp monitor` | Live view shows warnings per peer |
| Prometheus | `ze_bgp_prefix_warning_exceeded` gauge per peer/family |
| Logs | slog WARN entry on threshold breach |

The warnings system mirrors how ze already handles errors -- a count
in the status bar, a dedicated command to list them, and per-peer
detail in `show` output. Operators see warnings when they connect
to the CLI without having to search logs.

**Staleness warning on login:** When an operator logs in (CLI or SSH),
ze checks the age of the prefix data in zefs. If older than 6 months,
a warning appears in the status bar and login banner:

    ⚠ prefix data is 8 months old — run: ze data prefix update

This appears every login until the data is refreshed. Combined with
the `suggestion` mechanism, the operator is reminded to act.

| Condition | Where shown |
|-----------|------------|
| Prefix data > 6 months old | CLI/SSH login banner + status bar |
| Peer has `suggestion` field | CLI/SSH login banner + `ze bgp warnings` |

**When and where:**

| Channel | Timing | What |
|---------|--------|------|
| Prometheus | Always real-time | All prefix metrics continuously updated as UPDATEs arrive |
| CLI warning banner | Computed at login | Staleness, hot events, suggestions -- shown when operator connects |
| `ze config commit` | At commit time | Suggestion field warnings |
| Logs | At startup + on events | Startup log for stale data; runtime log on threshold breach |

Prometheus is always live. The CLI warning is the only thing
computed on-demand (at login).

**CLI commands:**

**Refresh the local prefix data in zefs:**

    ze data prefix update

| Command | Effect |
|---------|--------|
| `ze data prefix update` | Fetch latest routing-data.json from source-url, write to zefs |
| `ze data prefix show` | Show current zefs routing-data.json: date, source, ASN count |
| `ze data prefix lookup <asn>` | Show per-family prefix counts for a specific ASN |

`ze data prefix update` fetches from the configured `source-url`
(default: ze's codeberg repository). This refreshes the local data
without touching any peer config.

`ze data prefix import <file>` imports a JSON file directly into zefs.
For operators who produce their own prefix data or receive it from
a third party.

| Command | Effect |
|---------|--------|
| `ze data prefix update` | Fetch latest JSON from source-url, write to zefs |
| `ze data prefix import <file>` | Import a local JSON file into zefs |
| `ze data prefix show` | Show current zefs data: date, source, ASN count |
| `ze data prefix lookup <asn>` | Show per-family prefix counts for a specific ASN |

All data commands are available both from the system CLI (`ze data ...`)
and from within an SSH session to the running ze instance. Same syntax,
same behavior.

**Update peer prefix maximums from local data:**

    ze bgp peer <selector> prefix maximum update

| Command | Effect |
|---------|--------|
| `ze bgp peer * prefix maximum update` | Update all peers using their configured source |
| `ze bgp peer 10.0.0.* prefix maximum update` | Update only matching peers |
| `ze bgp peer AS65001 prefix maximum update` | Update only peers with this ASN |

For each matched peer:
1. If peer was manually configured: skipped
2. Looks up new value from `zefs meta/prefix/limit`
5. Computes new maximum (source value + 10%)
6. Updates the config with the new concrete number
7. Updates the date metadata
8. Shows a diff: old value -> new value

The config is modified but NOT committed. The operator reviews the
diff and commits manually (`ze config commit`).

**Typical workflow:**

1. `ze data prefix update` -- refresh local data from source-url
2. `ze bgp peer * prefix maximum update` -- apply to peer configs
3. Review diff
4. `ze config commit` -- apply

### Staleness Detection and Suggestion

When the source data date is older than 6 months, ze:

1. Looks up current value from the appropriate source (ze data or PeeringDB
   depending on per-family `source`)
2. If the new value differs significantly: adds a `suggestion` field
   to the peer's prefix config for that family
3. Emits Prometheus warning: `ze_bgp_prefix_data_stale` = 1

**The suggestion field is intentionally invalid config:**

    peer client-a {
        family {
            ipv4/unicast {
                prefix {
                    maximum 88000;
                    suggestion 105000;
                }
            }
        }
    }

**Behavior with a `suggestion` present:**

| Action | What happens |
|--------|-------------|
| ze startup | Runs normally. Ignores `suggestion`. Uses `maximum`. Does NOT refuse to start. |
| ze runtime | Normal operation. Prometheus warning active. |
| `ze config commit` | **Loud warning:** "peer client-a ipv4/unicast: maximum 88000 set 6+ months ago. Current data suggests 105000. Update maximum or remove suggestion to silence." |
| Operator accepts | Changes `maximum` to 105000 (or their choice), removes `suggestion`. Clean commit. |
| Operator dismisses | Removes `suggestion`, keeps `maximum` at 88000. Clean commit. |

**Why this works:**

- ze never stops running because of a stale value (operational safety)
- The `suggestion` field makes every `ze config commit` noisy until resolved
- The operator must actively deal with it
- Prometheus `ze_bgp_prefix_data_stale` lets monitoring catch it too
- No automatic config changes -- operator decides

### Config Commit Warning Feature

`ze config commit` must check for `suggestion` fields and emit warnings.
This is a general mechanism that warns on any config field requiring
operator attention.

| Commit check | Severity | Message |
|-------------|----------|---------|
| `suggestion` field present | WARNING (loud) | "peer X family F: stale data, review suggestion" |
| `suggestion` with value very different (>50% change) | ERROR-level warning | "peer X family F: data changed dramatically (88000 -> 205000), investigate" |

The commit proceeds (not blocked), but the warning is unmissable.

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

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peersettings.go` - PeerSettings struct, no prefix maximum field exists
  -> Constraint: PeerSettings is immutable after creation (copied into Peer)
- [ ] `internal/component/bgp/reactor/peer.go` - Peer struct, session lifecycle
  -> Constraint: prefix count must be tracked per live session per family, reset on reconnect
- [ ] `internal/component/bgp/message/notification.go` - NotifyCeaseMaxPrefixes = 1 already defined
  -> Constraint: NOTIFICATION infrastructure exists, just needs to be used
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - BGP config YANG schema
  -> Constraint: new container and leaves needed under peer block
- [ ] `internal/component/config/peers.go` - config parsing for peers
  -> Constraint: must extract prefix maximum from YANG tree into PeerSettings

**Behavior to preserve:**
- PeerSettings immutability
- Config pipeline: YANG tree -> resolve -> PeerSettings
- Existing NOTIFICATION infrastructure
- Session lifecycle (teardown, reconnect)

**Behavior to change:**
- Add per-family PrefixMaximum, PrefixWarning maps to PeerSettings; per-peer manual flag (hidden metadata)
- Add per-family prefix counter to Peer (per live session)
- Add enforcement check after UPDATE processing
- Add YANG container prefix { } with per-family maximum, warning, source
- Add peer-level teardown and idle-timeout
- Add source-url for data location
- Add config parsing for prefix config
- Mandatory validation: reject config without prefix maximum per negotiated family
- Store routing data in zefs meta/prefix/limit

## Data Flow (MANDATORY)

### Entry Point
- Config file: operator sets `prefix { <family> { maximum N; } }` per peer
- Or CLI autocompletion: ze suggests value from local data, operator accepts
- Or CLI keyword: operator types `peeringdb`, ze queries API, writes number

### Enforcement Path
1. Peer session established, per-family prefix counters initialized to 0
2. UPDATE received with N new NLRIs for family F
3. Prefix counter for F incremented by N
4. Withdrawals: prefix counter for F decremented
5. After each UPDATE: compare counter to warning and maximum for F
6. If counter >= warning: log warning, set Prometheus metric
7. If counter > maximum AND teardown=true: send NOTIFICATION, close session
8. If counter > maximum AND teardown=false: log error, reject further prefixes for F

### Data Update Path
1. New ze version installed, binary contains newer routing-data.json
2. On first run: compare embedded date with zefs date
3. If embedded is newer: write to zefs meta/prefix/limit
4. `ze bgp peer * prefix maximum update` reads from zefs (source=ze) or PeeringDB API (source=peeringdb)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> PeerSettings | YANG tree extraction in peers.go | [ ] |
| UPDATE -> prefix counter | Increment/decrement per family in peer UPDATE handler | [ ] |
| Counter -> NOTIFICATION | Enforcement check after UPDATE processing | [ ] |
| Warning -> log | Structured slog entry on threshold breach | [ ] |
| Binary -> zefs | Embedded routing data written on software update | [ ] |
| PeerSettings -> congestion system | Read PrefixMaximum for buffer sizing | [ ] |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with prefix maximum, peer exceeds for one family | -> | NOTIFICATION sent, session torn down | test/plugin/prefix-maximum-enforce.ci |
| Config with prefix maximum, peer at warning | -> | Warning logged, Prometheus metric set | test/plugin/prefix-maximum-warning.ci |
| Config without prefix maximum for negotiated family | -> | Config rejected at parse time | test/parse/prefix-maximum-required.ci |
| `ze bgp peer * prefix maximum update` | -> | Values updated from configured source | test/plugin/prefix-maximum-update.ci |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Family config has `ipv4/unicast { prefix { maximum 1000000; } }` | Per-family maximum = 1000000, warning = 900000 (default 90%) |
| AC-2 | Family block has no prefix maximum | Config parse error, ze refuses to start |
| AC-3 | Peer sends prefixes exceeding family maximum (teardown=true) | NOTIFICATION Cease/MaxPrefixes sent, session torn down |
| AC-4 | Peer sends prefixes exceeding family maximum (teardown=false) | Error logged, further prefixes for that family rejected, session stays |
| AC-5 | Peer prefix count reaches family warning | Warning logged (slog), `ze_bgp_prefix_warning_exceeded{family=X}` set to 1 |
| AC-6 | Autocompletion from ze local data | Values suggested from zefs routing-data.json for ipv4/ipv6, peer source set to `ze` |
| AC-7 | Autocomplete for ASN not in local data | Error: operator must provide explicit number or run `ze data prefix update` first |
| AC-9 | Peer withdraws prefixes below family warning | Counter decremented, `ze_bgp_prefix_warning_exceeded{family=X}` set back to 0 |
| AC-10 | Session reset after teardown | All family prefix counters reset to 0, all metrics reset |
| AC-11 | `prefix { idle-timeout 30; }` after teardown | Session re-establishes after 30 seconds |
| AC-12 | Stale prefix data or pending suggestion at startup | Advisory logged at startup |
| AC-13 | Prometheus metrics exposed | All prefix metrics carry peer, asn, family labels |
| AC-14 | PrefixMaximum per family available to congestion system | spec-forward-congestion reads per-family maximum for buffer sizing |
| AC-15 | Explicit warning > maximum for a family | Config parse error: warning must be less than maximum |
| AC-16 | No warning configured, family maximum is 1000000 | Warning auto-set to 900000 (90% of maximum) |
| AC-17 | Peer has ipv4/unicast and ipv6/vpn, only ipv6/vpn exceeds | Only ipv6/vpn triggers enforcement; ipv4/unicast unaffected |
| AC-18 | `update` command | Uses zefs local data for ipv4/ipv6 |
| AC-19 | `update` on peer with manual maximum | Peer skipped |
| AC-20 | `ze data prefix update` | Fetches latest data from source-url into zefs |
| AC-21 | Peer has VPN family but data blob only has ipv4/ipv6 | VPN family maximum must be set manually, error if missing |
| AC-22 | New ze version with newer embedded data | routing-data.json in zefs updated on first run |
| AC-23 | source-url overridden in config | `ze data prefix update` fetches from custom URL |
| AC-24 | routing-data.json format | Documented JSON schema, version field for forward compat |
| AC-25 | Repeated teardowns with idle-timeout | Exponential backoff: idle-timeout x 2^(N-1), capped at 1 hour |
| AC-26 | Session stable for 5+ minutes after reconnect | Backoff counter resets |
| AC-27 | teardown=false, peer exceeds maximum | UPDATEs for that family still processed but NLRIs beyond maximum are not installed in RIB or forwarded. Withdrawals always processed. Session stays up. |

## Files to Modify

- `internal/component/bgp/schema/ze-bgp-conf.yang` - add prefix container with per-family maximum, warning, source; peer-level teardown, idle-timeout, source-url
- `internal/component/bgp/reactor/peersettings.go` - add per-family PrefixMaximum/PrefixWarning maps, peer-level PrefixSource, PrefixTeardown, PrefixIdleTimeout
- `internal/component/config/peers.go` - extract prefix config from YANG tree
- `internal/component/bgp/reactor/peer.go` - add per-family prefix counter, enforcement check
- `internal/component/bgp/server/validate.go` - mandatory prefix maximum validation per negotiated family

## Files to Create

- `test/plugin/prefix-maximum-enforce.ci` - enforcement functional test
- `test/plugin/prefix-maximum-warning.ci` - warning threshold functional test
- `test/plugin/prefix-maximum-update.ci` - bulk update functional test
- `test/parse/prefix-maximum-required.ci` - mandatory validation test

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add prefix maximum |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` - add prefix { } syntax |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - `ze bgp peer * prefix maximum update` |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No (covered in configuration guide) | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc4486.md` - note prefix maximum enforcement |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - ze now has prefix maximum |
| 12 | Internal architecture changed? | No | |

## Open Questions (for review)

1. **Default source-url:** TBD -- determined when the JSON data format
   and initial data file are committed to the repository.
