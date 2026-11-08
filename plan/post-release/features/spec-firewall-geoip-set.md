# Spec: firewall-geoip-set

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P3 |
| Release reason | New firewall capability VyOS offers (country-code match) and Ze lacks; nothing shipped is wrong without it, so it is post-release feature work. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator wants to match traffic by the country of its source or destination
address: allow management access only from home countries, drop inbound traffic
from a list of countries, steer traffic for one country through a policy route.
Ze gains `source-country` and `destination-country` leaves (a list of ISO 3166
codes, with an optional inversion) in the firewall `from` block. A plugin keeps
the address sets for each country list current from a data source and refreshes
them on a timer.

## Owner Decision

| Decision | What Thomas decided | What it fixes in this spec |
|----------|---------------------|----------------------------|
| D1: data source | The internet registries' free statistics: the delegated-extended statistics files the five RIRs (AFRINIC, APNIC, ARIN, LACNIC, RIPE NCC) publish. They are free, plain text, authoritative for the country an allocation is registered to, and need no new module | The plugin fetches, parses and refreshes the RIR files ("Design: RIR statistics source" below) |

Rejected: a MaxMind or DB-IP MMDB database (a new reader module and a database
licence, for data the RIR files already give).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/forwarding.md` - the firewall table registry is process-local; a forked producer needs an RPC
- [ ] `docs/architecture/firewall/firewall-domain-group.md` - why a set-populating feature is its own plugin, and how its sets meet the operator's tables
  → Decision: a timer-owning firewall feature runs as its own plugin under `internal/component/firewall/plugins/`
- [ ] `docs/architecture/firewall/firewall-irr.md` - the other set-populating plugin
- [ ] `plan/post-release/features/spec-firewall-remote-group.md` - URL-sourced sets, the fetch mechanism option (a) would share

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` `MatchInSet` - `ProvidedType` marks a set another owner supplies.
- [ ] `internal/component/firewall/config.go` `parseFromBlock`, `irrSetMatch`, `domainSetMatch`, `DomainGroupSetNames`, `providedV6Twin`, `expandProvidedTermV6` - IRR and domain owners supply IPv4 and IPv6 sets; each set name comes from one exported function both sides call; `providedV6Twin` is a hand list of owner prefixes.
- [ ] `internal/component/firewall/validate.go` `validateMatch` - accepts a provided set by its `ProvidedType`.
- [ ] No country, GeoIP or MMDB code exists under `internal/` (grep for `geoip`, `maxmind`).

**Behavior to preserve:** IRR and domain groups.

**Behavior to change:** add country sets.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Provided sets | `config.go` `domainSetMatch`, `MatchInSet.ProvidedType`; `firewall.RegisterTables` | A plugin registers sets; operator terms match them by a name both sides derive from one function | Extends: a third owner, `firewall-geoip` |
| IPv6 twin expansion | `config.go` `providedV6Twin`, `expandProvidedTermV6` | Hand list of owner set prefixes | Replaces the hand list with a registration each owner makes, so the third owner edits no central list (`ai/rules/principles.md`) |
| Set-populating plugins | `internal/component/firewall/plugins/domain/`, `.../irr/` | Own process, timer, RPC to their data source, doctor check | Extends: same shape for the GeoIP plugin |
| Remote groups (design) | `plan/post-release/features/spec-firewall-remote-group.md` | URL-fetched address lists: fetch, read cap, last-good cache on disk, retry sooner than the full interval | Extends (D1): the RIR fetch reuses its fetch, cache and error posture. If that spec has not landed when this one starts, the fetch is built in the shape that spec describes so the two converge rather than duplicate |

## Design: RIR statistics source

1. **What is fetched.** The five RIRs' `delegated-<rir>-extended-latest` files, from each registry's published statistics URL. The default URL list is the plugin's own, and the operator can override each URL or point at a local file (for an air-gapped box or a mirror). Each RIR publishes a checksum file beside it; where one exists the download is checked against it.
2. **Format.** Pipe-separated lines: `registry|cc|type|start|value|date|status|...`. The version line, the summary lines and comments are skipped. Only `ipv4` and `ipv6` records with status `allocated` or `assigned` count; `available` and `reserved` carry no country. For IPv4, `value` is an address count that need not be a power of two, so a record becomes the range start to start plus count minus one, merged into the interval set as a range. For IPv6, `value` is a prefix length. The parser is checked against the file each RIR publishes today, and a record it cannot read fails the whole file, never a shorter list.
3. **Building sets.** The five files are merged into one map from country code to IPv4 and IPv6 ranges; each country list the config names becomes one interval set per family, its name derived from the list and family (AC-1, AC-2).
4. **Refresh.** The RIRs publish daily. The plugin refreshes on a timer (default once a day, operator-settable), fetching each RIR independently. A failed fetch retries sooner than the full interval, as the remote-group design does. A refresh swaps set elements atomically.
5. **Last good copy.** Each RIR's last good file is kept in the plugin's persistent state (SDK `StatePut`, never `os.WriteFile`, `ai/doors/system.md`), so a restart with the registries unreachable still has sets.
6. **Failure reporting.** One RIR failing keeps that RIR's last good data and the others' fresh data; the warning, `ze doctor` and the status command name the RIR, the URL, the age of its data and the fix. A file that fails its checksum or its parse is treated as a failed fetch.
7. **Status.** A status command reports, per RIR, the last successful fetch, the file date, the record count and the last error, from every command surface.
8. **Limit, stated in the guide.** An RIR record gives the country the block is registered to, not where its addresses are used. The operator guide says so, so nobody reads a country set as geolocation.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `source-country` or `destination-country` in a term's `from` block; RIR URL overrides and the refresh interval under the plugin's own container.

### Transformation Path
1. Parser builds a provided-set match whose name derives from the country list and family, never from the rule.
2. The plugin fetches the five RIR statistics files (or loads its last good copies), parses them, builds interval sets per country list and family, and registers them.
3. `ApplyAll` merges them into the operator's table; a refresh replaces set elements atomically.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → firewall registry | RPC when forked, as the domain plugin does | No |
| Plugin → RIR statistics | HTTPS fetch per RIR, or a local file the operator names | No |
| Plugin → persistent state | SDK `StateGet`/`StatePut` for each RIR's last good file | No |

### Integration Points
- `parseFromBlock`, the provided-set registry, `ApplyAll`

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `source-country` in config | → | firewall-geoip set build and merge | `test/firewall/geoip-source-destination.ci` (proposed) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | One term with `source-country NL` and `destination-country US` | Two distinct sets are matched, the source set holds only NL ranges and the destination set only US ranges, and a packet must satisfy both (VyOS `92c2d6ec5`: one set name for both sides, so one constraint silently overwrote the other) | - |
| AC-2 | The same country list used in two rules | One set is shared; its name derives from the list and family | - |
| AC-3 | IPv4 and IPv6 | Each family gets its own set, and an `inet` table term gets its IPv6 twin through the owner registration | - |
| AC-4 | Inverted match | Matches addresses outside the listed countries | - |
| AC-5 | One or more RIRs unreachable at refresh | That RIR's last good data stays in force with the other RIRs' fresh data, a warning is logged, and `ze doctor` reports the RIR, its URL, the age of its data and the fix | - |
| AC-6 | No RIR data at first start (unreachable and no last good copy) | The tables that name a country set are held back and the commit reports it with the fix; never an empty set that silently matches nothing | - |
| AC-7 | Unknown country code | Refused at verify | - |
| AC-8 | VPP backend | Refused at verify (named sets are already refused by the VPP verifier) | - |
| AC-9 | An RIR file with an IPv4 record whose count is not a power of two, an IPv6 record, and `available` and `reserved` records | The IPv4 record becomes its exact range, the IPv6 record its prefix, and the `available` and `reserved` records add nothing | - |
| AC-10 | An RIR file truncated, failing its checksum, or carrying a line the parser cannot read | Treated as a failed fetch: the last good copy stays, never a shorter list | - |
| AC-11 | Restart with the registries unreachable | Sets are built from the persisted last good copies | - |
| AC-12 | Status command from every surface | Per RIR: last successful fetch, file date, record count, last error | - |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGeoIPSetNamesPerSide` | `internal/component/firewall/config_test.go` | AC-1, AC-2 | |
| `TestGeoIPSourceParse` | plugin package | RIR file parse, AC-7, AC-9, AC-10 | |
| `TestGeoIPRefreshKeepsLastGood` | plugin package | AC-5, AC-11 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `geoip-source-destination` | `test/firewall/geoip-source-destination.ci` | AC-1 with a fixture data source | |

## Implementation Steps

1. **Phase: Wiring** -- YANG leaves, owner registration replacing `providedV6Twin`, failing `.ci`
2. **Phase: Plugin** -- RIR fetch and parse, set build, refresh, persistence, status, doctor
3. **Phase: Failure paths** -- AC-5, AC-6, AC-10, AC-11

## Files to Modify
- `internal/component/firewall/config.go`, `validate.go`
- `internal/component/firewall/yang/ze-firewall-conf.yang`
- `docs/guide/firewall.md`

## Files to Create
- `internal/component/firewall/plugins/geoip/` - the plugin
- `test/firewall/geoip-source-destination.ci`

## VyOS reference

T9332 (merge `1bd023055`, PR #5487): `92c2d6ec5` "fix GeoIP source/destination
nftables set name collision", with smoketest `a9bd2b76e`. AC-1 carries it.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Targeted tests pass: `./le test unit plugins`, `./le test firewall`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
