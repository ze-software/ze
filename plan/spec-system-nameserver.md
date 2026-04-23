# Spec: system-nameserver

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/8 |
| Updated | 2026-04-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/system/schema/ze-system-conf.yang` - system YANG
4. `internal/component/config/system/system.go` - ExtractSystemConfig
5. `internal/component/resolve/dns/resolver.go` - DNS resolver
6. `internal/plugins/iface/dhcp/resolv_linux.go` - resolv.conf writer
7. `cmd/ze/hub/main.go` - hub wiring (newResolvers at line 1381)

## Task

Add a `name-server` leaf-list to the `system` YANG container so operators can
configure static DNS servers (like VyOS `set system name-server 8.8.8.8`).
Unify with ze's internal DNS resolver: `system { name-server }` becomes the
single source of truth for DNS servers, replacing the separate
`environment { dns { server } }` leaf. DNS resolver tuning (timeout,
cache-size, cache-ttl) and resolv-conf-path also move to `system { dns {} }`.
Writes to the configurable resolv-conf-path (defaults to `/tmp/resolv.conf`
on gokrazy). Static name-servers take priority over DHCP-discovered servers.
Clean break: `environment { dns {} }` YANG is removed.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/environment.md` - environment config structure
  → Constraint: Priority is OS env > config file > YANG default. Env vars registered for DNS but never consumed.
  → Decision: DNS env vars (`ze.dns.*`) registered but never wired. Clean break removes them.
- [ ] `docs/architecture/config/environment-block.md` - environment block details
  → Constraint: `extractSections` does NOT include "dns" -- DNS has dedicated extractors that were never connected.
- [ ] `ai/patterns/config-option.md` - how to add config options
  → Constraint: YANG leaf with type, default, description. Embed + register in init(). Defaults from YANG not Go.
  → Constraint: Leaf-list uses `type ip-address` from `ze-types.yang`.

### RFC Summaries (MUST for protocol work)
N/A - not protocol work

**Key insights:**
- `environment { dns }` YANG exists but is disconnected: env vars registered, never consumed, resolver uses empty config.
- `resolv-conf-path` is currently per-interface (iface config) because DHCP is per-interface.
- Hub creates resolvers at line 457 of `cmd/ze/hub/main.go`, after config is loaded (line 190). `loadResult.Tree` is available.
- `ExtractSystemConfig()` reads from `tree.GetContainer("system")`. Pattern: add fields to SystemConfig struct, extract in same function.
- DHCP gets `ResolvConfPath` via factory callback from iface config. Moving path to system means passing it differently.
- `writeResolvConfTo()` is ~15 lines, does atomic tmp+rename. Can be extracted to a shared package.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/system/schema/ze-system-conf.yang` - defines `system { host, domain, peeringdb { url, margin }, archive [name] { ... } }`
  → Constraint: system container already exists with established children. name-server and dns {} are new siblings.
- [ ] `internal/component/resolve/dns/schema/ze-dns-conf.yang` - defines `environment { dns { server, timeout, cache-size, cache-ttl } }`
  → Decision: This entire YANG module will be removed. Clean break.
- [ ] `internal/component/resolve/dns/resolver.go` - `NewResolver(cfg ResolverConfig)` creates resolver with miekg/dns. `ResolverConfig` has Server, Timeout, CacheSize, CacheTTL. Empty Server triggers `resolveSystemDNS()` which reads `/etc/resolv.conf` and falls back to `8.8.8.8:53`.
  → Constraint: `ResolverConfig.Server` is a single string. For multiple name-servers, use the first one. Resolver makes one query per lookup, not round-robin.
  → Constraint: `resolveSystemDNS()` runs once at construction, not per-query.
- [ ] `internal/plugins/iface/dhcp/resolv_linux.go` - `writeResolvConfTo(path, servers)` does atomic write. `clearResolvConfAt(path)` removes file.
  → Constraint: Build-tagged `//go:build linux`. Needs platform stub for tests on Darwin.
- [ ] `internal/plugins/iface/dhcp/dhcp_v4_linux.go` - DHCPv4 writes DNS to resolv.conf at line 194 when `len(payload.DNSAll) > 0 && c.config.ResolvConfPath != ""`.
  → Constraint: DHCP writes on every lease acquire/renew (last-writer-wins).
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - DHCPv6 same pattern at line 209.
  → Constraint: Same last-writer-wins pattern as v4.
- [ ] `cmd/ze/hub/main.go` - `newResolvers()` at line 1381 creates `resolveDNS.NewResolver(resolveDNS.ResolverConfig{})` with empty config. Called at line 457 after config load.
  → Decision: Change `newResolvers()` to accept `SystemConfig` and use its name-server and DNS tuning fields.
- [ ] `internal/component/iface/config.go` - `ResolvConfPath` at line 30 of `ifaceConfig`. Default `/tmp/resolv.conf`. Parsed from `interface { resolv-conf-path }` at line 179.
  → Decision: Remove from ifaceConfig. DHCP receives path from system config via the factory callback.
- [ ] `internal/component/config/environment.go` - DNS env vars at lines 77-81 (`ze.dns.server`, `ze.dns.timeout`, `ze.dns.cache-size`, `ze.dns.cache-ttl`).
  → Decision: Remove these env var registrations.
- [ ] `internal/component/resolve/dns/schema/embed.go` - embeds ze-dns-conf.yang
  → Decision: Delete this file (YANG module removed).
- [ ] `internal/component/resolve/dns/schema/register.go` - registers ze-dns-conf.yang
  → Decision: Delete this file.
- [ ] `internal/component/resolve/dns/schema/schema_test.go` - tests for ze-dns-conf.yang parsing
  → Decision: Delete this file.
- [ ] `internal/component/config/yang/validator.go` - line 182 maps "dns" to "ze-dns-conf"
  → Decision: Remove this case.
- [ ] `internal/component/plugin/all/all.go` - imports dns/schema at line 61
  → Decision: Remove this import.
- [ ] `internal/component/resolve/resolvers.go` - Resolvers container, DNS field
  → Constraint: Resolvers struct unchanged. DNS field still populated.
- [ ] `internal/component/iface/register.go` - `dhcpClientFactory` passes `resolvConfPath` at line 690
  → Decision: Pass resolv-conf-path from system config instead of iface config.
- [ ] `internal/component/iface/schema/ze-iface-conf.yang` - `resolv-conf-path` leaf at line 352
  → Decision: Remove this leaf.

**Behavior to preserve:**
- `Resolver` type and its API (`ResolveA`, `ResolveAAAA`, `ResolveTXT`, `ResolvePTR`, `Resolve`)
- Cache behavior (LRU, TTL respect, TTL=0 means do not cache)
- Cymru, PeeringDB, IRR resolvers wired through `Resolvers` struct
- DHCP lease acquire still discovers DNS servers and publishes them in `DHCPPayload.DNSAll`
- Fallback to `8.8.8.8:53` when no servers configured and `/etc/resolv.conf` missing
- Atomic resolv.conf writing (tmp + rename)
- gokrazy compatibility (default path `/tmp/resolv.conf`)

**Behavior to change:**
- DNS server config moves from `environment { dns { server } }` to `system { name-server [...] }`
- DNS tuning (timeout, cache-size, cache-ttl) moves from `environment { dns {} }` to `system { dns {} }`
- `resolv-conf-path` moves from `interface {}` to `system { dns {} }`
- Hub's `newResolvers()` reads from `SystemConfig` instead of empty config
- DHCP: when static name-servers exist, DHCP does not overwrite resolv.conf
- `environment { dns {} }` YANG removed (clean break)
- DNS env vars (`ze.dns.*`) removed

## Data Flow (MANDATORY)

### Entry Point
- Config file: `system { name-server [8.8.8.8 1.1.1.1]; dns { resolv-conf-path ...; timeout ...; cache-size ...; cache-ttl ...; } }`
- Parsed by config loader into Tree

### Transformation Path
1. Config file parsed by `config.LoadConfig()` into a `*config.Tree`
2. `system.ExtractSystemConfig(tree)` reads `system/name-server` leaf-list and `system/dns/*` leaves into `SystemConfig` struct
3. Hub passes `SystemConfig` to `newResolvers(sc)` which creates `dns.NewResolver()` with first name-server as Server and tuning values
4. Hub writes resolv.conf with static name-servers at startup (if resolv-conf-path non-empty)
5. DHCP on lease acquire: checks if static name-servers exist. If yes, skip resolv.conf write. If no, write DHCP-discovered servers.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree → SystemConfig | `ExtractSystemConfig()` reads leaf-list and containers | [ ] |
| SystemConfig → Resolver | `newResolvers(sc)` maps fields to `ResolverConfig` | [ ] |
| SystemConfig → resolv.conf | `writeResolvConfTo()` called at startup | [ ] |
| SystemConfig → DHCP | resolv-conf-path and static-server-count passed via factory callback | [ ] |

### Integration Points
- `system.ExtractSystemConfig()` - extended with NameServers, DNS tuning, ResolvConfPath
- `dns.NewResolver()` - receives populated `ResolverConfig` instead of zero-value
- `dhcpClientFactory` - receives resolv-conf-path from system config
- Hub startup - writes resolv.conf after config load, before DHCP starts

### Architectural Verification
- [ ] No bypassed layers (config flows through ExtractSystemConfig, not ad-hoc tree reads)
- [ ] No unintended coupling (DHCP checks a boolean, does not import system config)
- [ ] No duplicated functionality (writeResolvConfTo extracted to shared location)
- [ ] Zero-copy preserved where applicable (N/A, not hot path)

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `system { name-server [8.8.8.8]; }` | → | `ExtractSystemConfig` reads leaf-list | `TestExtractSystemConfig_NameServers` |
| Config `system { dns { timeout 10; } }` | → | `ExtractSystemConfig` reads dns container | `TestExtractSystemConfig_DNS` |
| Hub startup with system name-servers | → | `newResolvers` uses first name-server | `test/parse/system-nameserver.ci` |
| Hub startup with system name-servers | → | resolv.conf written with all servers | `test/parse/system-nameserver.ci` |
| DHCP lease with static name-servers | → | DHCP skips resolv.conf write | `TestDHCPSkipsResolvConfWhenStaticServersExist` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config `system { name-server [8.8.8.8 1.1.1.1]; }` | `ExtractSystemConfig` returns `NameServers: ["8.8.8.8", "1.1.1.1"]` |
| AC-2 | Config with system name-servers and resolv-conf-path | resolv.conf written at startup with `nameserver 8.8.8.8` and `nameserver 1.1.1.1` lines |
| AC-3 | Config with system name-servers | Internal DNS resolver uses first name-server as upstream server |
| AC-4 | Config `system { dns { timeout 10; cache-size 5000; cache-ttl 3600; } }` | Resolver created with these values |
| AC-5 | Config `system { dns { resolv-conf-path /tmp/resolv.conf; } }` | resolv.conf written to specified path |
| AC-6 | Config with no system name-servers, DHCP active | DHCP writes DNS servers to resolv.conf (existing behavior) |
| AC-7 | Config with static name-servers, DHCP active | DHCP does NOT overwrite resolv.conf with DHCP-discovered servers |
| AC-8 | Config with no name-servers, no DHCP, no resolv.conf | Resolver falls back to `8.8.8.8:53` |
| AC-9 | Config with `environment { dns { server 8.8.8.8; } }` (old syntax) | Config validation rejects with error |
| AC-10 | Config with no `system { dns {} }` block | Default resolv-conf-path `/tmp/resolv.conf`, timeout 5, cache-size 10000, cache-ttl 86400 |
| AC-11 | `interface { resolv-conf-path }` in config (old location) | Config validation rejects with error |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractSystemConfig_NameServers` | `internal/component/config/system/system_test.go` | AC-1: leaf-list extraction | |
| `TestExtractSystemConfig_NameServers_Empty` | `internal/component/config/system/system_test.go` | AC-8: no servers returns empty slice | |
| `TestExtractSystemConfig_DNS` | `internal/component/config/system/system_test.go` | AC-4: dns tuning extraction | |
| `TestExtractSystemConfig_DNS_Defaults` | `internal/component/config/system/system_test.go` | AC-10: default values | |
| `TestExtractSystemConfig_ResolvConfPath` | `internal/component/config/system/system_test.go` | AC-5: resolv-conf-path extraction | |
| `TestWriteResolvConf` | `internal/component/config/system/resolv_linux_test.go` | AC-2: atomic resolv.conf write | |
| `TestWriteResolvConf_Empty` | `internal/component/config/system/resolv_linux_test.go` | Empty servers list writes nothing | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| timeout | 1-60 | 60 | 0 | 61 |
| cache-size | 0-1000000 | 1000000 | N/A (0 valid) | 1000001 |
| cache-ttl | 0-604800 | 604800 | N/A (0 valid) | 604801 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `system-nameserver` | `test/parse/system-nameserver.ci` | Config with name-server leaf-list parses and servers are extracted | |
| `system-dns-tuning` | `test/parse/system-dns-tuning.ci` | Config with dns tuning values parses correctly | |

### Future (if deferring any tests)
- Integration test for DHCP skip when static servers present (requires running DHCP, deferred to DHCP test suite)

## Files to Modify

- `internal/component/config/system/schema/ze-system-conf.yang` - add `name-server` leaf-list and `dns` container
- `internal/component/config/system/system.go` - extend `SystemConfig` struct and `ExtractSystemConfig()` with NameServers, DNS tuning, ResolvConfPath
- `internal/component/config/system/system_test.go` - new tests for name-server and dns extraction
- `cmd/ze/hub/main.go` - `newResolvers()` accepts SystemConfig, writes resolv.conf at startup
- `internal/component/config/environment.go` - remove `ze.dns.*` env var registrations
- `internal/component/iface/config.go` - remove `ResolvConfPath` from `ifaceConfig`
- `internal/component/iface/schema/ze-iface-conf.yang` - remove `resolv-conf-path` leaf
- `internal/component/iface/register.go` - pass resolv-conf-path from system config, pass static-servers flag to DHCP factory
- `internal/plugins/iface/dhcp/dhcp_v4_linux.go` - skip resolv.conf write when static servers flag set
- `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - skip resolv.conf write when static servers flag set
- `internal/component/config/yang/validator.go` - remove "dns" case at line 182
- `internal/component/plugin/all/all.go` - remove dns/schema import

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (name-server leaf-list + dns container) | [x] | `internal/component/config/system/schema/ze-system-conf.yang` |
| CLI commands/flags | [ ] | YANG-driven (automatic) |
| Editor autocomplete | [ ] | YANG-driven (automatic if YANG updated) |
| Functional test | [x] | `test/parse/system-nameserver.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features/dns-resolver.md` - update config location |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - system name-server syntax |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/features/dns-resolver.md` |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - ze now has static name-server like VyOS |
| 12 | Internal architecture changed? | [x] | `docs/architecture/config/environment-block.md` - remove dns section |

## Files to Create

- `internal/component/config/system/resolv_linux.go` - extracted resolv.conf writer (from DHCP plugin)
- `internal/component/config/system/resolv_linux_test.go` - tests for resolv.conf writer
- `internal/component/config/system/resolv_other.go` - no-op stub for Darwin builds
- `test/parse/system-nameserver.ci` - functional test
- `test/parse/system-dns-tuning.ci` - functional test

## Files to Delete

- `internal/component/resolve/dns/schema/ze-dns-conf.yang` - retired YANG module
- `internal/component/resolve/dns/schema/embed.go` - retired embed
- `internal/component/resolve/dns/schema/register.go` - retired registration
- `internal/component/resolve/dns/schema/schema_test.go` - retired test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: YANG schema** - Add `name-server` leaf-list and `dns` container to `ze-system-conf.yang`. Remove `ze-dns-conf.yang` module, its embed/register files, and all references (all/all.go import, validator.go case). Remove `resolv-conf-path` from `ze-iface-conf.yang`.
   - Tests: `test/parse/system-nameserver.ci`, `test/parse/system-dns-tuning.ci`
   - Files: `ze-system-conf.yang`, `ze-iface-conf.yang`, delete `dns/schema/*`, `all.go`, `validator.go`
   - Verify: functional tests parse config with new syntax

2. **Phase: SystemConfig extraction** - Extend `SystemConfig` struct with `NameServers []string`, `DNSTimeout uint16`, `DNSCacheSize uint32`, `DNSCacheTTL uint32`, `ResolvConfPath string`. Extend `ExtractSystemConfig()` to read leaf-list and dns container.
   - Tests: `TestExtractSystemConfig_NameServers`, `_DNS`, `_DNS_Defaults`, `_ResolvConfPath`
   - Files: `system.go`, `system_test.go`
   - Verify: tests fail → implement → tests pass

3. **Phase: resolv.conf writer** - Extract `writeResolvConfTo` to `internal/component/config/system/resolv_linux.go` with `resolv_other.go` stub. Write resolv.conf at hub startup when static name-servers present.
   - Tests: `TestWriteResolvConf`, `TestWriteResolvConf_Empty`
   - Files: `resolv_linux.go`, `resolv_other.go`, `resolv_linux_test.go`
   - Verify: tests fail → implement → tests pass

4. **Phase: Hub wiring** - Change `newResolvers()` to accept `SystemConfig`. Use first name-server as resolver server. Use DNS tuning values. Call resolv.conf writer at startup. Remove `ze.dns.*` env var registrations from `environment.go`.
   - Tests: functional test verifies config parses end-to-end
   - Files: `cmd/ze/hub/main.go`, `internal/component/config/environment.go`
   - Verify: build succeeds, functional tests pass

5. **Phase: DHCP coordination** - The factory callback chain that delivers `resolvConfPath` to DHCP clients must be rewired from iface config to system config. The exact chain is:

   **Current chain (to be changed):**
   1. `ifaceConfig.ResolvConfPath` parsed from `interface { resolv-conf-path }` in `iface/config.go:179`
   2. `reconcileDHCP(cfg, ...)` at `iface/register.go:690` passes `cfg.ResolvConfPath` to factory
   3. `dhcpClientFactory` function signature at `iface/register.go:572` has `resolvConfPath string` parameter
   4. `SetDHCPClientFactory` at `iface/register.go:576` matches the signature
   5. `newDHCPClientFromFactory` at `dhcp/register.go:47` receives it, puts in `DHCPConfig.ResolvConfPath`
   6. `DHCPConfig` struct at `dhcp/ifacedhcp.go:15` has `ResolvConfPath string`
   7. `dhcp_v4_linux.go:194` and `dhcp_v6_linux.go:209` use `c.config.ResolvConfPath` to write resolv.conf

   **New chain:**
   1. `SystemConfig.ResolvConfPath` extracted from `system { dns { resolv-conf-path } }` in `system.go`
   2. `SystemConfig.NameServers` extracted from `system { name-server [...] }` in `system.go`
   3. Hub passes `SystemConfig` to iface component (via a package-level setter or a new parameter to `reconcileDHCP`)
   4. `reconcileDHCP` gets `resolvConfPath` and `hasStaticNameServers bool` from system config
   5. Factory signature adds `hasStaticNameServers bool` parameter after `resolvConfPath`
   6. `DHCPConfig` gains `HasStaticNameServers bool` field
   7. `dhcp_v4_linux.go` and `dhcp_v6_linux.go` check `c.config.HasStaticNameServers` before writing resolv.conf

   **Key invariant:** When `HasStaticNameServers` is true, DHCP MUST NOT write resolv.conf. When false, existing behavior is preserved exactly (DHCP writes on lease acquire/renew, last-writer-wins).

   - Tests: `TestDHCPSkipsResolvConfWhenStaticServersExist`
   - Files: `iface/config.go` (remove ResolvConfPath), `iface/register.go` (rewire factory call + add setter for system config), `iface/register.go` (factory signature change), `dhcp/ifacedhcp.go` (add HasStaticNameServers to DHCPConfig), `dhcp/register.go` (match new factory signature), `dhcp_v4_linux.go` (guard resolv.conf write), `dhcp_v6_linux.go` (guard resolv.conf write)
   - Verify: build succeeds, existing DHCP tests still pass, new test validates skip behavior

6. **Phase: Documentation** - Update dns-resolver.md, configuration.md, environment-block.md, comparison.md
   - Files: docs files from Documentation Update Checklist
   - Verify: docs reflect new config location

7. **Full verification** - `make ze-verify` (lint + all ze tests except fuzz)

8. **Complete spec** - Fill audit tables, write learned summary to `plan/learned/NNN-system-nameserver.md`, delete spec from `plan/`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Leaf-list parsed correctly (ordered, unique IPs). DHCP coordination works (skip when static). |
| Naming | YANG uses kebab-case (`name-server`, `cache-size`). Go uses PascalCase (`NameServers`, `CacheSize`). |
| Data flow | Config → ExtractSystemConfig → newResolvers and resolv.conf writer. No shortcut reads. |
| Rule: no-layering | Old `environment { dns }` YANG fully deleted. Old `ze.dns.*` env vars removed. Old `resolv-conf-path` on interface removed. |
| Rule: exact-or-reject | `name-server` values must be valid IP addresses (YANG `ip-address` type validates) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `name-server` leaf-list in system YANG | `grep "name-server" internal/component/config/system/schema/ze-system-conf.yang` |
| `dns` container in system YANG | `grep "container dns" internal/component/config/system/schema/ze-system-conf.yang` |
| `environment { dns }` YANG removed | `ls internal/component/resolve/dns/schema/ze-dns-conf.yang` fails |
| `ze.dns.*` env vars removed | `grep "ze.dns" internal/component/config/environment.go` finds nothing |
| `resolv-conf-path` removed from iface YANG | `grep "resolv-conf-path" internal/component/iface/schema/ze-iface-conf.yang` finds nothing |
| resolv.conf writer under system | `ls internal/component/config/system/resolv_linux.go` |
| Functional test exists | `ls test/parse/system-nameserver.ci` |
| Hub wires resolver from system config | `grep "SystemConfig\|systemConfig" cmd/ze/hub/main.go` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `name-server` values validated as IP addresses by YANG `ip-address` type. No arbitrary strings. |
| Path traversal | `resolv-conf-path` must be absolute path, no `..` components (existing validation from iface config, preserve it) |
| File permissions | resolv.conf written 0o644 (world-readable by convention, same as existing DHCP writer) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

## RFC Documentation

N/A - not protocol work.

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

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-system-nameserver.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary
