# Spec: prefix-data

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/5 |
| Updated | 2026-03-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `plan/learned/413-prefix-limit.md` -- context from predecessor spec
3. `internal/component/bgp/reactor/session_prefix.go` -- enforcement logic
4. `internal/component/bgp/reactor/peersettings.go` -- PeerSettings struct

## Task

Prefix data management: query PeeringDB directly at runtime to suggest
prefix maximum values, track when each peer's maximum was last updated,
and warn operators when values are stale.

This spec receives deferred items from `spec-prefix-limit` (413-prefix-limit).
The original design proposed a build pipeline, embedded routing data, and
zefs storage. That has been replaced by direct PeeringDB queries at runtime.

## Design Change from spec-prefix-limit

~~The original spec-prefix-limit design proposed:~~
~~- Build pipeline producing routing-data.json from PeeringDB + RouteViews~~
~~- Embedded JSON in ze binary, written to zefs on first run~~
~~- source-url config for fetching from ze's repository~~
~~- source field (manual/ze) per family~~
~~- CLI commands: ze data prefix update/show/lookup/import~~
~~- Advisory suggestion field in config~~
~~- Config commit warnings for stale suggestions~~

**New design:** Query PeeringDB API directly when the operator asks for
updated values. No build pipeline. No embedded data. No zefs storage for
prefix data. Only store a per-peer timestamp of when the prefix maximum
was last updated, so ze can warn when values are stale.

### What was eliminated

| Original item | Disposition |
|---------------|-------------|
| zefs `meta/prefix/limit` storage | Eliminated -- no prefix data in zefs |
| Embedded routing-data.json in binary | Eliminated -- no build pipeline |
| routing-data.json format spec | Eliminated -- no data file |
| `source-url` global config | Eliminated -- query PeeringDB directly |
| `source` field (manual/ze) per family | Eliminated -- not needed |
| `ze data prefix update/show/lookup/import` | Eliminated -- replaced by `ze bgp peer * prefix update` |
| Suggestion field in config | Eliminated -- staleness warning is sufficient |
| Config commit warnings for suggestions | Eliminated -- no suggestion field |
| `ze_bgp_prefix_suggestion` metric | Eliminated -- no suggestion system |
| `ze_bgp_prefix_data_stale` metric | Replaced by per-peer staleness check |
| `ze_bgp_prefix_data_age_days` metric | Replaced by per-peer staleness check |
| AC-6, AC-7 (autocompletion from local data) | Eliminated -- PeeringDB queried on demand |
| AC-20 (ze data prefix update) | Eliminated -- no data commands |
| AC-22 (embedded data on new version) | Eliminated -- no embedded data |
| AC-23 (source-url override) | Eliminated -- no source-url |
| AC-24 (routing-data.json format) | Eliminated -- no data file |

### What remains (simplified)

| Feature | Description |
|---------|-------------|
| PeeringDB client | Query PeeringDB API for prefix counts by ASN |
| Update command | `ze bgp peer * prefix update` queries PeeringDB, updates config values |
| Per-peer timestamp | YANG leaf recording when maximum was last updated |
| Staleness warning | Warn when a peer's prefix maximum is stale |
| Enforcement .ci test | Fix ze-peer race condition for AC-3 wiring test |
| `prefix_ratio` metric | Computed from existing count/maximum (standalone gap) |

## Design

### PeeringDB API

PeeringDB (or a compatible service) provides per-ASN prefix counts via
its REST API.

| Field | API path | Returns |
|-------|----------|---------|
| IPv4 prefixes | `GET /api/net?asn=<N>` | `info_prefixes4` (int) |
| IPv6 prefixes | `GET /api/net?asn=<N>` | `info_prefixes6` (int) |

PeeringDB only has data for ipv4/unicast and ipv6/unicast. Other families
(VPN, flow, EVPN, etc.) must always be set manually by the operator.

**API URL:** Configurable. Default is the public PeeringDB API. Operators
can point to a PeeringDB-compatible service (private mirror, internal
data source) by setting the URL in config.

PeeringDB is an external service, not a BGP concept. Configuration lives
under `system { peeringdb { } }` (in `ze-system-conf.yang`), not under
`bgp`. This keeps BGP config focused on BGP and allows other subsystems
to use PeeringDB data in the future.

    system {
        peeringdb {
            url "https://peeringdb.example.com";
            margin 10;
        }
    }

| YANG path | Type | Default | Description |
|-----------|------|---------|-------------|
| `system/peeringdb/url` | string | `https://www.peeringdb.com` | PeeringDB-compatible API base URL |
| `system/peeringdb/margin` | uint8 | 10 | Percentage margin above PeeringDB count (0-100) |

**Rate limiting:** 1 request per second maximum.

**TLS:** For localhost (127.0.0.1) URLs, TLS certificate validation is
skipped. This allows functional tests to run a fake PeeringDB service
on localhost without needing valid certificates. For all other hosts,
standard TLS verification applies.

**Error handling:** PeeringDB may be unreachable. The update command reports
which peers could not be updated and why. Never fails silently.

### Per-Peer Timestamp

A hidden YANG leaf in the peer-level prefix container records when the
prefix maximum was last updated. Hidden means it is not shown in normal
config output (`ze config show`) but is stored persistently and can be
queried explicitly.

| YANG path | Type | Default | Description |
|-----------|------|---------|-------------|
| `bgp/peer/prefix/updated` | string (date) | empty | ISO date when maximum was last set or refreshed. Hidden leaf. |

The timestamp is peer-level (not per-family) because the update command
refreshes all families for a peer at once. It is set:

| Event | Timestamp set to |
|-------|-----------------|
| Operator types `prefix { maximum N; }` in config | Not set (manual, no staleness tracking) |
| `ze bgp peer X prefix update` succeeds | Today's date |
| Config migration from old format | Not set (operator should run update) |

An empty timestamp means: either manually configured (no staleness concern)
or never updated from PeeringDB (operator should run update if they want
automatic tracking).

The leaf does not appear in `ze config show` output or config file
exports. It is visible via `ze bgp peer X show` (which shows runtime
and metadata) and via Prometheus labels.

### Update Command

    ze bgp peer <selector> prefix update

| Selector | Effect |
|----------|--------|
| `*` | Update all peers |
| `10.0.0.*` | Update matching peers |
| `AS65001` | Update peers with this ASN |

For each matched peer:

| Step | Action |
|------|--------|
| 1 | Read peer's remote ASN from config |
| 2 | Query PeeringDB (or URL from `system/peeringdb/url`) for prefix counts |
| 3 | Apply configured margin (`system/peeringdb/margin`, default 10%): suggested = PeeringDB count * (1 + margin/100) |
| 4 | For each negotiated family with PeeringDB data: propose new maximum |
| 5 | Show diff: old value -> new value |
| 6 | Update config with new values |
| 7 | Set `prefix { updated "YYYY-MM-DD"; }` (hidden leaf) |

The config is modified but NOT committed. The operator reviews the diff
and commits manually (`ze config commit`).

**Families without PeeringDB data** (VPN, flow, etc.) are skipped with
a note: "no PeeringDB data for ipv6/vpn -- set manually."

**Typical workflow:**

    ze bgp peer * prefix update       # query PeeringDB, update configs
    ze config diff                     # review changes
    ze config commit                   # apply

### Staleness Warning

When the `updated` timestamp is older than a configurable threshold,
ze warns the operator.

| Channel | When | What |
|---------|------|------|
| Startup log | ze starts | slog WARN: "peer X: prefix maximum last updated N months ago" |
| CLI login | Operator connects (CLI/SSH) | Banner: "2 peers have stale prefix data -- run: ze bgp peer * prefix update" |
| Prometheus | Always | `ze_bgp_prefix_stale` gauge (1 if stale, 0 if not) per peer |
| `ze bgp peer X show` | On demand | Shows last-updated date and staleness status |

The CLI login warning mirrors how ze already handles errors and prefix
warnings -- a count in the status bar, actionable message, and a command
to fix it. Operators see staleness when they connect without having to
search logs.

### Prometheus Metrics (additions)

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ze_bgp_prefix_ratio` | Gauge | peer, asn, family | current_count / maximum (0.0 to 1.0+) |
| `ze_bgp_prefix_stale` | Gauge | peer, asn | 1 if updated timestamp is older than threshold, 0 otherwise |

These complement the 6 metrics already implemented in spec-prefix-limit.

### Config Inheritance

Prefix maximum can be set at three levels. More specific overrides less
specific:

| Level | Scope | Example |
|-------|-------|---------|
| `bgp/family` | All peers (root default) | "every peer gets 100K ipv4 unless overridden" |
| `bgp/group/family` | All peers in group | "IXP clients get 50K" |
| `bgp/peer/family` | This peer only | "this specific peer gets 500K" |

Currently only peer-level is implemented. Group and root defaults are
part of the general config inheritance system and may already work
via the config resolution pipeline.

## ~~Open Questions~~ Resolved

| # | Question | Answer |
|---|----------|--------|
| 1 | Staleness threshold | 6 months (fixed) |
| 2 | Config inheritance | Verify group/root defaults work via resolve pipeline |
| 3 | PeeringDB rate limits | 1 request per second max |
| 4 | Margin on PeeringDB values | Configurable (default 10%) |
| 5 | PeeringDB URL | Configurable via `system/peeringdb/url` (default public PeeringDB) |
| 6 | YANG location | `system { peeringdb { } }` not `bgp { }` -- PeeringDB is a service, not a BGP concept |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- config pipeline, resolve
- [ ] `docs/architecture/config/syntax.md` -- config format

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4486.md` -- predecessor spec context

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peersettings.go` -- PeerSettings struct with PrefixMaximum/Warning/Teardown/IdleTimeout
- [ ] `internal/component/bgp/reactor/config.go` -- parsePrefixLimitFromFamily, parsePrefixSettingsFromTree
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` -- current prefix container (lines 168-197)
- [ ] `internal/component/bgp/reactor/session_prefix.go` -- enforcement logic, metric helpers
- [ ] `internal/component/bgp/reactor/reactor_metrics.go` -- 6 existing prefix metrics

**Behavior to preserve:**
- Existing prefix enforcement (counting, warning, teardown, drop)
- Existing 6 Prometheus metrics
- Config parsing for prefix { maximum; warning; } per family
- Peer-level prefix { teardown; idle-timeout; }
- Mandatory validation (every family must have a maximum)

**Behavior to change:**
- Add `updated` leaf to peer-level prefix container (YANG + config parsing)
- Add `ze bgp peer * prefix update` command (PeeringDB query + config update)
- Add `prefix_ratio` and `prefix_stale` Prometheus metrics
- Add staleness warning at startup

## Data Flow (MANDATORY)

### Entry Point: Update Command
1. Operator runs `ze bgp peer * prefix update`
2. Command reads config: for each matching peer, get remote ASN
3. Query PeeringDB API for each peer's ASN (1 req/s)
4. For each peer: compute new maximum (PeeringDB value + 10% margin)
5. Update config values for ipv4/unicast and ipv6/unicast
6. Set peer-level `prefix { updated "YYYY-MM-DD"; }`
7. Show diff to operator
8. Operator runs `ze config commit` to apply

### Entry Point: Staleness Check
1. ze starts, reads config for all peers
2. For each peer with `prefix { updated "..."; }`: compute age
3. If age > threshold: log warning, set `ze_bgp_prefix_stale` = 1
4. At runtime: metric stays set until config is updated and reloaded

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PeeringDB API -> prefix counts | HTTPS GET, JSON response | [ ] |
| Prefix counts -> config values | Update command writes to config tree | [ ] |
| Config -> PeerSettings | Existing parsePrefixLimitFromFamily | [ ] |
| Config timestamp -> staleness metric | Read at startup, set Prometheus gauge | [ ] |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze bgp peer X prefix update` with mock PeeringDB | -> | Config updated with PeeringDB values + timestamp | test/plugin/prefix-update.ci |
| Peer with stale `updated` timestamp at startup | -> | Warning logged + metric set | test/plugin/prefix-stale-warning.ci |
| Config with prefix maximum, peer exceeds (teardown=true) | -> | NOTIFICATION sent, session torn down | test/plugin/prefix-maximum-enforce.ci |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze bgp peer * prefix update` with reachable PeeringDB | ipv4/ipv6 maximums updated to PeeringDB value + configured margin, `updated` timestamp set |
| AC-2 | `ze bgp peer * prefix update` with unreachable PeeringDB | Error reported per peer, no config changed, no timestamp set |
| AC-3 | Peer has VPN family, PeeringDB has no VPN data | VPN family skipped with message, ipv4/ipv6 updated normally |
| ~~AC-4~~ | ~~Multiple peers with same ASN~~ | ~~PeeringDB queried once per unique ASN~~ Removed -- no ASN dedup needed |
| AC-11 | `system { peeringdb { url "https://internal.example.com"; } }` | Update command uses custom URL |
| AC-12 | `system { peeringdb { margin 20; } }` | Update command applies 20% margin instead of default 10% |
| AC-5 | Peer's `updated` timestamp is older than threshold | Warning at startup, `ze_bgp_prefix_stale` = 1 |
| AC-6 | Peer has no `updated` timestamp | No staleness warning (manually configured, no tracking) |
| AC-7 | `ze bgp peer X show` for peer with stale timestamp | Shows last-updated date and staleness status |
| AC-8 | After successful update, peer's `updated` refreshed | `ze_bgp_prefix_stale` = 0 for that peer |
| AC-9 | `ze_bgp_prefix_ratio` metric | Equals current_count / maximum for each peer/family |
| AC-10 | Config with prefix maximum, peer exceeds (teardown=true) | NOTIFICATION sent, session torn down (enforcement .ci test from spec-prefix-limit AC-3) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestPeeringDBParseResponse | peeringdb_test.go | JSON response parsing | [ ] |
| ~~TestPeeringDBDedup~~ | ~~peeringdb_test.go~~ | ~~Multiple peers same ASN = one query~~ Removed -- no dedup | |
| TestComputeMargin | peeringdb_test.go | PeeringDB value + 10% margin | [ ] |
| TestStalenessCheck | session_prefix_test.go | Timestamp age vs threshold | [ ] |
| TestPrefixRatioMetric | session_prefix_test.go | count/maximum ratio computation | [ ] |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PeeringDB prefix count | 0..max uint32 | 4294967295 | N/A (0 is valid) | N/A (API returns int) |
| Margin | 0..100 | 100 | N/A (0 means exact PeeringDB value) | 101 |
| Staleness threshold | 180 days (fixed) | N/A | N/A | N/A |
| PeeringDB response: 0 prefixes | suspicious | Skip peer, report | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| prefix-update | test/plugin/prefix-update.ci | Update command queries fake PeeringDB on localhost, updates config | [ ] |
| prefix-stale-warning | test/plugin/prefix-stale-warning.ci | Stale timestamp triggers warning | [ ] |
| prefix-maximum-enforce | test/plugin/prefix-maximum-enforce.ci | Enforcement .ci (carried from spec-prefix-limit) | [ ] |

### Fake PeeringDB Service

A lightweight HTTP server in `test/tools/` that serves PeeringDB-compatible
JSON responses on localhost. Used by .ci tests via `peeringdb-url`
pointed at `http://127.0.0.1:<port>`.

| Endpoint | Response |
|----------|----------|
| `/api/net?asn=65001` | `{"data": [{"info_prefixes4": 85000, "info_prefixes6": 12000}]}` |
| `/api/net?asn=99999` | `{"data": []}` (unknown ASN) |
| `/api/net?asn=65002` | `{"data": [{"info_prefixes4": 0, "info_prefixes6": 0}]}` (suspicious zero) |

The fake server is started by the .ci test harness before the test and
torn down after. Config points `peeringdb-url` at `http://127.0.0.1:<port>`.
No TLS needed for localhost -- the client skips certificate validation
for 127.0.0.1 hosts.

## Files to Modify

- `internal/component/config/system/schema/ze-system-conf.yang` -- add `peeringdb` container with `url` and `margin`
- `internal/component/bgp/schema/ze-bgp-conf.yang` -- add `updated` hidden leaf to peer-level prefix container
- `internal/component/bgp/reactor/config.go` -- parse `updated` from config tree
- `internal/component/config/system/` -- parse `peeringdb` settings from config tree
- `internal/component/bgp/reactor/peersettings.go` -- add PrefixUpdated field
- `internal/component/bgp/reactor/session_prefix.go` -- add ratio metric helper
- `internal/component/bgp/reactor/reactor_metrics.go` -- add prefix_ratio and prefix_stale metrics

## Files to Create

- PeeringDB client (location TBD -- likely `internal/component/bgp/peeringdb/`)
- Update command handler (likely `internal/component/bgp/plugins/cmd/peer/` extension)
- Fake PeeringDB server for testing (`test/tools/fake-peeringdb/` or similar)
- `test/plugin/prefix-update.ci`
- `test/plugin/prefix-stale-warning.ci`
- `test/plugin/prefix-maximum-enforce.ci`

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add PeeringDB prefix update |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- add `updated` leaf |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- `ze bgp peer * prefix update` |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No (covered in configuration guide) | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- PeeringDB integration |
| 12 | Internal architecture changed? | No | |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| Research | Required Reading, Current Behavior |
| Design | Design section above |
| TDD | TDD Test Plan |
| Implementation | Files to Modify/Create |
| Verification | make ze-verify |

### Implementation Phases

| Phase | What |
|-------|------|
| 1 | YANG + config parsing for `updated` leaf, staleness check, prefix_ratio metric |
| 2 | PeeringDB client (HTTP, JSON parsing, rate limiting) |
| 3 | Update command (`ze bgp peer * prefix update`) |
| 4 | Functional tests (.ci) |
| 5 | Enforcement .ci test (fix ze-peer race condition) |

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| PeeringDB client | Error handling, rate limiting, timeout |
| Config update | Values written correctly, timestamp set |
| Staleness | Threshold check works, metric set/cleared |
| No regressions | Existing prefix enforcement unchanged |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Update command works | .ci test |
| Staleness warning fires | .ci test |
| prefix_ratio metric | Unit test |
| Enforcement .ci test | .ci test passes (ze-peer fix) |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| PeeringDB TLS | TLS verification for non-localhost hosts, skip only for 127.0.0.1 |
| Input validation | PeeringDB response validated before use (reject negative, non-integer) |
| Rate limiting | 1 req/s max, no unbounded requests to PeeringDB |
| URL validation | Only http/https schemes allowed for peeringdb-url |

### Failure Routing

| Failure | Route To |
|---------|----------|
| PeeringDB unreachable | Report error, skip peer, continue others |
| PeeringDB returns 0 prefixes | Report suspicious, skip (don't set max to 0) |
| ASN not found in PeeringDB | Report, skip peer |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| zefs storage + embedded data + build pipeline | Over-engineered, unnecessary complexity | Direct PeeringDB query at runtime |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
