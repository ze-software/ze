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

## Open Owner Question

| Question | Evidence | Options |
|----------|----------|---------|
| Q1: which data source | A MaxMind or DB-IP database needs an MMDB reader, a new third-party module that Thomas must approve for this use (`ai/META-DECISIONS.md`, "Code and dependencies"), and the database carries its own licence. The five RIR delegated-extended statistics files are free, authoritative for allocation country, and are plain text needing no new module | (a) RIR delegated stats, fetched like a remote group; (b) MMDB with an approved module; (c) both, operator selects |

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
| Remote groups (design) | `plan/post-release/features/spec-firewall-remote-group.md` | URL-fetched address lists | Extends if Q1 picks (a): the RIR fetch reuses its fetch and cache |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `source-country` or `destination-country` in a term's `from` block; data-source settings under the plugin's own container.

### Transformation Path
1. Parser builds a provided-set match whose name derives from the country list and family, never from the rule.
2. The plugin loads the data source, builds interval sets per country list and family, and registers them.
3. `ApplyAll` merges them into the operator's table; a refresh replaces set elements atomically.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → firewall registry | RPC when forked, as the domain plugin does | No |
| Plugin → data source | HTTPS fetch or local file | No |

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
| AC-5 | Data source unreachable at refresh | The last good sets stay in force, a warning is logged, and `ze doctor` reports the source, its age and the fix | - |
| AC-6 | Data source unreachable at first start | The tables that name a country set are held back and the commit reports it with the fix; never an empty set that silently matches nothing | - |
| AC-7 | Unknown country code | Refused at verify | - |
| AC-8 | VPP backend | Refused at verify (named sets are already refused by the VPP verifier) | - |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGeoIPSetNamesPerSide` | `internal/component/firewall/config_test.go` | AC-1, AC-2 | |
| `TestGeoIPSourceParse` | plugin package | data source parse, AC-7 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `geoip-source-destination` | `test/firewall/geoip-source-destination.ci` | AC-1 with a fixture data source | |

## Implementation Steps

1. **Phase: Wiring** -- YANG leaves, owner registration replacing `providedV6Twin`, failing `.ci`
2. **Phase: Plugin** -- data source (after Q1), set build, refresh, doctor
3. **Phase: Failure paths** -- AC-5, AC-6

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
- [ ] Q1 answered by Thomas before design
- [ ] AC-1..AC-8 all demonstrated
- [ ] Targeted tests pass: `./le test unit plugins`, `./le test firewall`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
