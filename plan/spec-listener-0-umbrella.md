# Spec: listener-0-umbrella

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-04-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/yang/modules/ze-extensions.yang` - existing extensions
4. `internal/component/config/environment.go` - env struct + registration
5. `internal/component/config/environment_extract.go` - config tree extraction

## Task

Centralize all listener configuration under a `ze:listener` YANG extension, normalize listener services to named lists (`list server { key name; }`) with `ip` + `port` leaves via a core `zt:listener` grouping and `enabled` leaf (default false), remove ExaBGP legacy environment settings (including `bgp > listen` and the `tcp`/`bgp` environment sections) that are redundant with Ze per-peer config, centralize scattered env var registrations into the config package with compound `ze.<service>.listen` vars for multi-endpoint support, and add port conflict detection at config parse time (with 0.0.0.0/:: wildcard awareness).

**Motivation:** Env var registrations are scattered across 6+ files. Listener endpoints use inconsistent naming (`host`, `address`, `ip`) and single-instance containers where multi-endpoint is needed. No mechanism detects port conflicts between services. The `environment` block carries ExaBGP legacy settings (`tcp`, `bgp`) that are redundant with per-peer config. `bgp > listen` is ExaBGP legacy (Ze derives listeners from per-peer `connection > local`). The user wants a single registry where all listeners are visible and conflicts are caught at parse time.

This is an umbrella spec. Implementation is in five child specs.

### Child Specs

| Spec | Scope | Depends | Status |
|------|-------|---------|--------|
| `spec-listener-1-yang` | zt:listener grouping, ze:listener extension, normalize all YANG to named lists with `enabled` leaf, BGP grouping restructure, remove `bgp > listen`/tcp.port/bgp.connect/bgp.accept from YANG | - | skeleton |
| `spec-listener-2-env` | Centralize env registrations, fix gaps (UpdateGroups, Backend, extraction), compound `ze.<service>.listen` vars, drop per-field listener vars, `ze.<service>.enabled` vars, update all Go consumers | spec-listener-1-yang | skeleton |
| `spec-listener-3-conflict` | CollectListeners, ValidateListenerConflicts, wire into config validation, port conflict functional tests | spec-listener-1-yang | skeleton |
| `spec-listener-5-log` | Remove 14 ExaBGP boolean log leaves, verify Ze subsystem equivalents, ensure env + CLI control for all topics | spec-listener-1-yang | skeleton |
| `spec-listener-4-migrate` | `ze config migrate` transformations (host-to-ip, tcp.port removal, bgp.connect/accept removal, bgp listen removal, log booleans), `ze exabgp migrate --env` flag with full log mapping | spec-listener-1-yang, spec-listener-5-log | skeleton |

Specs 3 and 5 depend only on spec 1. Spec 2 depends on spec 1 and must run before spec 5 (both modify environment.go/LogEnv). Spec 4 depends on specs 1 and 5 (needs log mapping for ExaBGP migration).

## ExaBGP Legacy Audit

The `environment` block is inherited from ExaBGP's `[environment]` section. This spec addresses the listener-related legacy; the rest is deferred.

### In scope (listener-related)

| Section.Leaf | Problem | Resolution |
|-------------|---------|------------|
| `bgp > listen` | ExaBGP global listen address -- Ze derives listeners from per-peer `connection > local` when `remote > accept` is true | Remove entirely. No replacement needed |
| `tcp.port` | Global listen port -- ExaBGP legacy | Remove. No replacement (per-peer config) |
| `bgp.connect` | Global connect override -- redundant with per-peer `connection > local > connect` | Remove. Per-peer setting is the Ze way |
| `bgp.accept` | Global accept override -- redundant with per-peer `connection > remote > accept` | Remove. Per-peer setting is the Ze way |

### In scope (spec-listener-5-log)

| Section | Resolution |
|---------|------------|
| `log.*` (14 legacy booleans) | Remove from YANG. Ze uses `ze.log.<subsystem>=<level>`. Verify all 12 ExaBGP topics have Ze subsystem equivalents. Ensure env + CLI control |

### Out of scope (future spec: env-cleanup)

| Section | Why deferred |
|---------|-------------|
| `tcp.attempts`, `tcp.delay`, `tcp.acl` | Connection behavior, not listener config. Move to `bgp > connection` or similar |
| `bgp.openwait` | Session timer, not listener config |
| `api.*` (7 leaves) | ExaBGP API settings. Some redundant with per-plugin config (`respawn`), some dead |
| `daemon.daemonize` | ExaBGP daemon mode. Modern daemons use systemd |
| `debug.pdb` | Python debugger. Dead in Go |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config parsing, YANG-driven schema
  -> Constraint: YANG extensions drive parser behavior; new `ze:listener` must integrate with existing extension handling
- [ ] `docs/architecture/config/environment.md` - environment configuration
  -> Constraint: environment block extraction feeds LoadEnvironmentWithConfig

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - BGP port 179, connection establishment
  -> Constraint: BGP listens on port 179 by default; per-peer local address binding

**Key insights:**
- YANG extensions in ze-extensions.yang are parsed by the config parser and drive special handling
- Environment extraction (`ExtractEnvironment`) only covers 9 sections; web/ssh/dns/mcp are missing
- BGP listeners are derived solely from per-peer `connection > local` (ip + port) when `remote > accept` is true. There is no global BGP listen -- that was ExaBGP legacy
- Env var registrations scattered across hub/main.go, ssh/client.go, reactor.go, rs/server.go, etc.
- `environment > tcp` and `environment > bgp` are entirely ExaBGP legacy; every leaf either has a proper Ze home or is dead
- BGP `connection > local > ip` and `connection > remote > ip` are currently added via augments within the same component -- should be in the grouping per YANG structure rules

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - 12 extensions defined (syntax, key-type, route-attributes, allow-unknown-fields, sensitive, validate, command, edit-shortcut, display-key, cumulative, decorate)
- [ ] `internal/component/config/environment.go` - Environment struct with 9 section sub-structs, 48 ze.bgp.* MustRegister calls, envOptions table, LoadEnvironmentWithConfig
- [ ] `internal/component/config/environment_extract.go` - ExtractEnvironment covers: daemon, log, tcp, bgp, cache, api, reactor, debug, chaos. Missing: web, ssh, dns, mcp
- [ ] `internal/component/web/schema/ze-web-conf.yang` - `environment > web { host (zt:ip-address, default 0.0.0.0), port (uint16, default 3443) }`
- [ ] `internal/component/ssh/schema/ze-ssh-conf.yang` - `environment > ssh { listen (leaf-list string "host:port"), host-key, idle-timeout, max-sessions }`
- [ ] `internal/component/mcp/schema/ze-mcp-conf.yang` - `environment > mcp { host (zt:ip-address, default 127.0.0.1), port (uint16, no default) }`
- [ ] `internal/component/lg/schema/ze-lg-conf.yang` - `environment > looking-glass { host (zt:ip-address, default 0.0.0.0), port (uint16, default 8443) }`
- [ ] `internal/component/telemetry/schema/ze-telemetry-conf.yang` - `telemetry > prometheus { address (zt:ip-address, default 0.0.0.0), port (zt:port, default 9273) }`
- [ ] `internal/component/plugin/schema/ze-plugin-conf.yang` - `plugin > hub > server (list) { host (string), port (uint16) }`
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - `bgp > listen (leaf string)`, peer `connection > local { ip, port }`, `environment > tcp { port (zt:port, default 179) }`
- [ ] `cmd/ze/hub/main.go` - 13 MustRegister calls: ze.ready.file, ze.web.{host,port,insecure}, ze.mcp.{host,port}, ze.looking-glass.{host,port,tls}, ze.dns.{server,timeout,cache-size,cache-ttl}
- [ ] `cmd/ze/internal/ssh/client/client.go` - 4 MustRegister calls: ze.config.dir, ze.ssh.{host,port,password} (client-side, not server)
- [ ] `internal/component/bgp/reactor/reactor.go` - 11 MustRegister calls: ze.fwd.{chan.size,write.deadline,pool.size,pool.maxbytes,batch.limit,teardown.grace,pool.headroom}, ze.cache.safety.valve, ze.buf.{read.size,write.size}, ze.metrics.interval
- [ ] `internal/component/bgp/plugins/rs/server.go` - 2 MustRegister calls: ze.rs.{chan.size,fwd.senders}

**Behavior to preserve:**
- All existing non-listener env var names (ze.bgp.*, ze.fwd.*, ze.rs.*, ze.dns.*, etc.)
- LoadEnvironmentWithConfig pipeline: defaults -> config block -> OS env override
- Per-peer listener creation in reactor (startMultiListeners, peerListenPort)
- SSH client env vars (ze.config.dir, ze.ssh.host, ze.ssh.port, ze.ssh.password) -- client-side, NOT listeners
- Per-peer `connection > local > connect` and `connection > remote > accept` leaves -- Ze-native, NOT legacy

**Behavior to change:**

### YANG core

| Change | Detail |
|--------|--------|
| Add `zt:listener` grouping to ze-types.yang | Two leaves: `ip` (zt:ip-address) + `port` (zt:port) |
| Add `ze:listener` extension to ze-extensions.yang | Marks a container as a network listener endpoint |
| Add `import ze-extensions` to 4 files | ze-web-conf, ze-lg-conf, ze-mcp-conf, ze-telemetry-conf (missing import) |

### Listener normalization (all use `enabled` leaf + `list server { key name; ze:listener; uses zt:listener; }` + refine defaults)

All services get an `enabled` leaf (boolean, default false) at the service container level. When enabled and the list is empty, a default listener entry is created using YANG refine defaults. When disabled, the service does not start regardless of list entries.

| Service | Current | Change |
|---------|---------|--------|
| Web | `host` (zt:ip-address) + `port` (uint16) | Add `enabled` leaf; replace host/port with `list server { key name; uses zt:listener; ze:listener; }` refine defaults 0.0.0.0:3443 |
| Looking Glass | `host` + `port` (uint16) | Same pattern; refine defaults 0.0.0.0:8443 |
| MCP | `host` + `port` (uint16, no default) | Same pattern; refine defaults 127.0.0.1 + default port |
| Telemetry | `address` + `port` (zt:port) | Same pattern; refine defaults 0.0.0.0:9273 |
| SSH | leaf-list `listen` (string "host:port") | Same pattern; refine defaults from current listen format |
| Plugin hub server | `host` (string) + `port` (uint16) | Already a list; normalize leaves to `uses zt:listener`; `string` to `zt:ip-address`; add `ze:listener` |

**Removed:** BGP global `listen` leaf (ExaBGP legacy). Ze derives BGP listeners from per-peer `connection > local` when `remote > accept` is true.

### BGP grouping restructure

| Change | Detail |
|--------|--------|
| Move `ip` from augments into `peer-fields` grouping | Both `connection > local > ip` and `connection > remote > ip` |
| Delete 4 augments | Standalone peer local/remote, grouped peer local/remote |
| Add `ze:listener` to `connection > local` in grouping | BGP peer local does NOT use `zt:listener` grouping (ip is union type with `auto` enum) |
| Validation | `connection > remote > ip` required at peer level, optional at group level |

### ExaBGP legacy removal

| Remove | Replacement |
|--------|------------|
| `bgp > listen` leaf | None -- Ze derives BGP listeners from per-peer `connection > local` |
| `environment > tcp > port` | None -- per-peer config. Port 179 is per-peer default |
| `environment > bgp > connect` | Per-peer `connection > local > connect` (already exists) |
| `environment > bgp > accept` | Per-peer `connection > remote > accept` (already exists) |
| `ze.bgp.tcp.port` env var | Removed (no global listen) |
| `ze.bgp.bgp.connect` env var | Removed (per-peer config) |
| `ze.bgp.bgp.accept` env var | Removed (per-peer config) |

**Note:** Per-peer `connection > local > connect` and `connection > remote > accept` are Ze-native and must NOT be removed. Only the global `environment > bgp` overrides are ExaBGP legacy.

### Env var changes

| Change | Detail |
|--------|--------|
| Drop per-field listener vars | Remove `ze.web.host`, `ze.web.port`, `ze.mcp.host`, `ze.mcp.port`, `ze.looking-glass.host`, `ze.looking-glass.port` (don't map to list entries) |
| Add compound listen vars | `ze.web.listen=ip:port,ip:port` format for each service (web, mcp, looking-glass, ssh, telemetry). Parsed into list entries. IPv6 uses bracket notation: `[::1]:3443` |
| Add enabled vars | `ze.web.enabled`, `ze.mcp.enabled`, `ze.looking-glass.enabled`, `ze.ssh.enabled`, `ze.telemetry.enabled` |
| Keep service-level vars | `ze.web.insecure`, `ze.looking-glass.tls` unchanged |
| Centralize registrations into config package | Move from hub/main.go, reactor.go, rs/server.go |
| Register missing SSH server leaves | `ze.ssh.host-key`, `ze.ssh.idle-timeout`, `ze.ssh.max-sessions` |
| Add ReactorEnv.UpdateGroups | Struct field + envOptions entry (dead registration fix) |
| Add LogEnv.Backend | Registration + struct field + envOptions entry (YANG gap fix) |
| Add web, ssh, dns, mcp to ExtractEnvironment | Config file values for these sections currently silently lost |
| SSH client vars unchanged | `ze.config.dir`, `ze.ssh.host`, `ze.ssh.port`, `ze.ssh.password` stay (client-side, not listeners) |
| Port conflict detection | At config parse time, collect all ze:listener list entries, check for conflicts with 0.0.0.0/:: wildcard awareness |

## Data Flow (MANDATORY)

### Entry Point
- Config file parsed into Tree via YANG-driven schema
- Environment variables read from OS env

### Transformation Path
1. Config file -> Tree (YANG parser, extensions processed)
2. Tree -> ExtractEnvironment() -> map[string]map[string]string (environment sections)
3. map -> LoadEnvironmentWithConfig() -> Environment struct (with defaults + overrides)
4. **NEW:** Tree -> CollectListeners() -> []ListenerEndpoint (scan for ze:listener containers)
5. **NEW:** []ListenerEndpoint -> ValidateListenerConflicts() -> error (port overlap check)
6. Environment struct consumed by reactor, hub, SSH server, etc.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema -> Parser | ze:listener extension recognized | [ ] |
| Config tree -> Listener collection | Walk tree for ze:listener containers | [ ] |
| Listener collection -> Validation | Check all collected endpoints for conflicts | [ ] |
| Config tree -> Environment extraction | ExtractEnvironment reads new sections | [ ] |

### Integration Points
- `config.ExtractEnvironment()` - add web, ssh, dns, mcp sections
- `config.LoadEnvironmentWithConfig()` - consume new sections
- YANG schema loading - process `ze:listener` extension
- Config validation pipeline - add conflict check after tree is parsed
- `reactor.CreateReactorFromTree()` - conflict check runs here (all config resolved)

### Architectural Verification
- [ ] No bypassed layers (conflict check uses same parsed config tree)
- [ ] No unintended coupling (listener collection reads YANG metadata, not component internals)
- [ ] No duplicated functionality (extends existing YANG extension system)
- [ ] Zero-copy preserved where applicable (N/A -- config parsing, not wire encoding)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with two services on same port | -> | ValidateListenerConflicts | `test/parse/listener-conflict-same-port.ci` |
| Config with 0.0.0.0 vs specific IP conflict | -> | ValidateListenerConflicts | `test/parse/listener-conflict-wildcard.ci` |
| Config with no conflicts | -> | ValidateListenerConflicts | `test/parse/listener-no-conflict.ci` |
| SSH config with ip + port format | -> | SSH listener parsing | `test/parse/ssh-listen-ip-port.ci` |
| BGP peer local ip + port in conflict check | -> | CollectListeners | `test/parse/listener-conflict-bgp-peer.ci` |
| `ze env registered` shows centralized vars | -> | env.Entries() | `test/parse/env-registered-centralized.ci` |
| `ze exabgp migrate --env` with env file | -> | ExaBGP env parser + merge | `test/parse/exabgp-migrate-env.ci` |

## Acceptance Criteria

Umbrella-level ACs verify cross-cutting concerns. Per-feature ACs are in child specs.

| AC ID | Input / Condition | Expected Behavior | Child spec |
|-------|-------------------|-------------------|------------|
| AC-1 | All YANG listener services use `list server { key name; }` with `uses zt:listener` | Web, LG, MCP, telemetry, SSH, plugin hub server | listener-1-yang |
| AC-2 | All services have `enabled` leaf (default false) | Enabled + empty list = default endpoint; disabled = service off | listener-1-yang |
| AC-3 | No `leaf host` or `leaf address` in listener YANG | `grep -r "leaf host\|leaf address" internal/component/*/schema/*.yang` returns nothing in listener contexts | listener-1-yang |
| AC-4 | No BGP augments for connection > local/remote > ip | 4 augments deleted, ip in peer-fields grouping | listener-1-yang |
| AC-5 | `bgp > listen` leaf removed | Config with `bgp { listen "..."; }` rejected | listener-1-yang |
| AC-6 | `environment > tcp > port` removed | Config with `environment { tcp { port 179; } }` rejected | listener-1-yang |
| AC-7 | `ze env registered` shows all vars centralized | No MustRegister outside config package (except sdk, privilege, slogutil, main.go) | listener-2-env |
| AC-8 | `ze.web.listen=0.0.0.0:3443` compound env var works | Web server binds to specified endpoint | listener-2-env |
| AC-9 | Port conflict: two services on same ip:port | Config parse error with both services named | listener-3-conflict |
| AC-10 | Port conflict: 0.0.0.0 vs specific IP on same port | Wildcard conflict detected | listener-3-conflict |
| AC-11 | 14 ExaBGP log booleans removed from YANG | `grep` for legacy log leaves returns nothing | listener-5-log |
| AC-12 | All 12 ExaBGP log topics have Ze subsystem equivalents | Verified against actual slogutil subsystem names (may differ from ExaBGP topic names) | listener-5-log |
| AC-13 | `ze config migrate` handles old config format | host to ip, tcp.port removal, bgp.connect/accept removal, bgp listen removal, log booleans | listener-4-migrate |
| AC-14 | `ze exabgp migrate --env` reads ExaBGP env file | tcp.port/bind emit comments (no global listen in Ze), log booleans mapped to Ze levels, unsupported settings emit comments | listener-4-migrate |
| AC-15 | `make ze-verify` passes after all children | Full test suite green | umbrella |

## TDD Test Plan

Unit tests, boundary tests, and functional tests are defined in each child spec.

### Unit Tests
| Test | Child spec |
|------|-----------|
| YANG schema loading after restructure | listener-1-yang |
| Conflict detection (same port, wildcard, no-conflict, BGP peer) | listener-3-conflict |
| Env registration centralization, UpdateGroups, Backend | listener-2-env |
| ExaBGP env parsing, merge, unsupported settings | listener-4-migrate |
| Ze config migrate transformations | listener-4-migrate |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| port (zt:port) | 1-65535 | 65535 | 0 | 65536 |
| SSH max-sessions (uint16) | 0-65535 | 65535 | N/A | 65536 |
| SSH idle-timeout (uint32) | 0-4294967295 | 4294967295 | N/A | N/A |

### Functional Tests
| Test | Child spec |
|------|-----------|
| Listener conflict detection (3 .ci files) | listener-3-conflict |
| SSH ip+port format, BGP listen container | listener-1-yang |
| `ze env registered` centralized output | listener-2-env |
| `ze config migrate` transformations (4 .ci files) | listener-4-migrate |
| `ze exabgp migrate --env` | listener-4-migrate |

### Future (if deferring any tests)
- Property test: random listener sets, verify conflict detection is symmetric and transitive

## Files to Modify

Detailed file lists are in each child spec. Summary by child:

| Child spec | YANG files | Go files | Test files |
|------------|-----------|----------|------------|
| listener-1-yang | 10 (ze-types, ze-extensions, 8 component schemas) | BGP config loaders, peers.go | Schema loading tests, existing .ci updates |
| listener-2-env | - | environment.go, environment_extract.go, hub/main.go, reactor.go, rs/server.go, all host-to-ip consumers | Env registration tests, extraction tests |
| listener-3-conflict | - | NEW: listener.go, loader_create.go (wiring) | NEW: listener_test.go, 3 .ci conflict tests |
| listener-5-log | ze-hub-conf.yang (remove 14 log leaves) | environment.go (remove log boolean fields/envOptions), slogutil.go (verify subsystems) | Log subsystem tests |
| listener-4-migrate | - | migration package, cmd/ze/exabgp/main.go, NEW: env.go | Migration tests, 5 .ci migration tests |

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new extension) | [x] | `ze-extensions.yang` |
| YANG schema (listener containers) | [x] | All 8 component .yang files listed above |
| CLI commands/flags | [ ] | N/A (ze env already works) |
| Editor autocomplete | [x] | Automatic (YANG-driven) |
| Functional test for conflict detection | [x] | `test/parse/listener-conflict-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- port conflict detection |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- named list pattern for all listeners, enabled leaf, removed bgp > listen/tcp.port/bgp.connect/bgp.accept |
| 2b | Config architecture? | [x] | `docs/architecture/config/syntax.md` -- ze:listener extension, zt:listener grouping, YANG structure rules |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` -- `ze exabgp migrate --env` new flag |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/environment.md` (if exists) -- compound listen vars (ze.web.listen, etc.), removed per-field vars (ze.web.host/port, etc.), enabled vars, removed legacy vars (tcp.port, bgp.connect, bgp.accept) |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [x] | `docs/architecture/config/environment.md` -- centralized registrations, ExaBGP legacy removal, listener conflict detection |
| 13 | Migration guide needed? | [x] | `docs/guide/migration.md` (or section in configuration.md) -- breaking changes: named list pattern, enabled leaf, bgp listen removal, tcp.port removal. How to update configs or use `ze config migrate` |

## Files to Create

Detailed in each child spec. New files summary:

- `internal/component/config/listener.go` - CollectListeners, ValidateListenerConflicts (listener-3-conflict)
- `internal/component/config/listener_test.go` - conflict detection unit tests (listener-3-conflict)
- `internal/exabgp/migration/env.go` - ExaBGP env file parser (listener-4-migrate)
- `internal/exabgp/migration/env_test.go` - env parser unit tests (listener-4-migrate)
- 8 new `.ci` functional test files across listener-1 through listener-4

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

Implementation is delegated to child specs. Each child follows TDD independently.

1. **spec-listener-1-yang** -- YANG core: grouping, extension, named list pattern with `enabled` leaf, BGP restructure, ExaBGP legacy removal (`bgp > listen`, tcp.port, bgp.connect/accept) from YANG
2. **spec-listener-2-env** -- Go env: centralize registrations, fix gaps, compound `ze.<service>.listen` vars, drop per-field listener vars, `ze.<service>.enabled` vars, update consumers
3. **spec-listener-3-conflict** -- Port conflict detection: CollectListeners from ze:listener list entries, ValidateListenerConflicts, wiring
4. **spec-listener-5-log** -- Log cleanup: remove 14 ExaBGP boolean leaves, verify Ze subsystem equivalents (names may differ from ExaBGP), env + CLI control
5. **spec-listener-4-migrate** -- Migration: `ze config migrate` transformations, `ze exabgp migrate --env` (tcp.bind/port emit comments, not bgp listen)
6. **Cross-spec verification** -- `make ze-verify` after all children complete
7. **Complete umbrella** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Wildcard logic handles both IPv4 (0.0.0.0) and IPv6 (::) |
| Naming | All YANG listener leaves use `ip` (not host/address); env vars use `ze.<service>.listen` (compound) and `ze.<service>.enabled` |
| Data flow | Conflict check runs after full config resolution (peers resolved, defaults applied) |
| Rule: no-layering | Old host/address leaves fully deleted, not kept alongside ip |
| Rule: compatibility | No compat shims for old env var names (Ze unreleased) |
| Env registration | No MustRegister calls remain in hub/main.go, ssh/client.go, reactor.go, rs/server.go |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `ze:listener` extension in ze-extensions.yang | `grep "extension listener" ze-extensions.yang` |
| All listener YANG use named list + `ip` not `host`/`address` | `grep -r "leaf host" internal/component/*/schema/*.yang` returns nothing in listener contexts |
| All services have `enabled` leaf | `grep -r "leaf enabled" internal/component/*/schema/*.yang` for each service |
| `bgp > listen` removed | `grep "leaf listen" ze-bgp-conf.yang` returns nothing |
| No MustRegister outside config package (except sdk, privilege, slogutil, main.go) | `grep -r "MustRegister" cmd/ze/hub/ cmd/ze/internal/ssh/ internal/component/bgp/reactor/ internal/component/bgp/plugins/rs/` returns nothing |
| Port conflict detection wired | `test/parse/listener-conflict-same-port.ci` passes |
| Compound listen vars work | `ze.web.listen=0.0.0.0:3443` accepted, web binds correctly |
| ReactorEnv.UpdateGroups populated | `grep "UpdateGroups" internal/component/config/environment.go` |
| LogEnv.Backend populated | `grep "Backend" internal/component/config/environment.go` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Port range 1-65535 enforced; IP address validated via zt:ip-address type |
| Conflict error messages | Do not leak sensitive config (passwords, secrets) in conflict error text |
| Wildcard binding | Warn or document that 0.0.0.0 binds all interfaces (security implication for MCP) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
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

- The entire `environment` block is ExaBGP legacy. This spec addresses the listener-related subset; a follow-up spec should clean up the rest (api.*, daemon.daemonize, debug.pdb).
- `augment` within the same YANG component was a misuse -- BGP's ip augments should have been in the grouping from the start. New rule added to `.claude/rules/config-design.md`.
- The `zt:listener` grouping in ze-types.yang creates a reusable pattern that any future listener can adopt.
- All listener services use named lists (`list server { key name; }`) for multi-endpoint support. The `enabled` leaf (default false) controls whether a service starts; empty list + enabled = default endpoint from YANG refine.
- `bgp > listen` is ExaBGP legacy. Ze derives BGP listeners from per-peer `connection > local` when `remote > accept` is true. No global listen needed.
- Compound env vars (`ze.web.listen=ip:port,ip:port`) replace per-field vars (`ze.web.host` + `ze.web.port`). IPv6 uses bracket notation: `[::1]:3443`. This enables multi-endpoint control via environment without config file changes.
- SSH client env vars (ze.config.dir, ze.ssh.host, ze.ssh.port, ze.ssh.password) must NOT be touched -- they are outbound connection settings, not listeners.
- `environment > bgp > connect/accept` are global overrides of per-peer settings. Ze's per-peer config is the proper mechanism; the global overrides are ExaBGP compat that should not exist. Per-peer `connection > local > connect` and `connection > remote > accept` are Ze-native and must be preserved.
- ExaBGP topic-to-Ze-subsystem mapping is provisional. Actual slogutil subsystem names differ from ExaBGP topic names (e.g., `bgp.config` not `bgp.configuration`, `bgp.server` not `bgp.network`). Spec-listener-5-log must verify actual names before mapping is finalized.

## ExaBGP Migration Tool

`ze exabgp migrate` (in `cmd/ze/exabgp/main.go`) converts ExaBGP configs to Ze format. It currently accepts a config file but has no support for the ExaBGP environment file (`exabgp.env`).

### New flag: `--env <path>`

Add `--env` flag to `ze exabgp migrate` that reads an ExaBGP env file and merges its settings into the Ze output. The ExaBGP env file is INI format parsed by Python's `configparser`, with sections like `[exabgp.tcp]` and keys like `port = 179`. Source: `exabgp/environment/config.py`.

Only non-default values are emitted. Default values are skipped (Ze has its own YANG defaults).

### Mapping: listener-related (ExaBGP legacy, no Ze equivalent)

| ExaBGP key | Ze output | Notes |
|-----------|-----------|-------|
| `tcp.bind = '10.0.0.1 10.0.0.2'` | Comment: `# exabgp.tcp.bind: Ze uses per-peer connection > local > ip. Configure each peer individually.` | No global listen in Ze |
| `tcp.port = 1179` | Comment: `# exabgp.tcp.port: Ze uses per-peer connection > local > port. Default is 179 (RFC 4271).` | No global listen in Ze |

### Mapping: environment settings (value carries to Ze config)

| ExaBGP section | Ze output path | Keys |
|---------------|---------------|------|
| `daemon` | `environment { daemon { <key> <value>; } }` | pid, user, daemonize, drop, umask |
| `cache` | `environment { cache { <key> <value>; } }` | attributes |
| `reactor` | `environment { reactor { <key> <value>; } }` | speed |
| `debug` | `environment { debug { <key> <value>; } }` | memory, configuration, selfcheck, route, defensive, rotate, timing |
| `tcp` (remaining) | `environment { tcp { <key> <value>; } }` | attempts, delay, acl |
| `bgp` (remaining) | `environment { bgp { <key> <value>; } }` | openwait |

### Mapping: logging (requires spec-listener-5-log)

ExaBGP uses per-topic booleans. Ze uses `ze.log.<subsystem>=<level>`. Migration maps `true` to `debug`, `false` to `disabled`.

| ExaBGP key | Ze output |
|-----------|-----------|
| `log.level = WARNING` | `environment { log { level warn; } }` |
| `log.destination = syslog` | `environment { log { destination syslog; } }` |
| `log.all = true` | `environment { log { level debug; } }` (overrides per-topic) |
| `log.enable = false` | `environment { log { level disabled; } }` |
| `log.short = true` | `environment { log { short true; } }` |
| `log.<topic> = true` | `environment { log { <subsystem> debug; } }` |
| `log.<topic> = false` | `environment { log { <subsystem> disabled; } }` |

ExaBGP topic to Ze subsystem mapping (verified in spec-listener-5-log):

| ExaBGP topic | Ze subsystem |
|-------------|-------------|
| configuration | bgp.configuration |
| reactor | bgp.reactor |
| daemon | bgp.daemon |
| processes | bgp.processes |
| network | bgp.network |
| statistics | bgp.statistics |
| packets | bgp.packets |
| rib | bgp.rib |
| message | bgp.message |
| timers | bgp.timers |
| routes | bgp.routes |
| parser | bgp.parser |

This mapping is provisional. Actual slogutil subsystem names differ from ExaBGP topic names (e.g., `bgp.config` not `bgp.configuration`, `bgp.server` not `bgp.network`). Only `bgp.reactor` and `bgp.routes` currently exist as exact matches. spec-listener-5-log must verify actual names, decide on mapping strategy (create new subsystems vs map to existing Ze names), and finalize this table before spec-listener-4-migrate can implement the mapping.

### Mapping: comment-only (per-peer or Python-only)

| ExaBGP key | Comment emitted |
|-----------|-----------------|
| `tcp.once` | `# exabgp.tcp.once: deprecated, use tcp.attempts = 1` |
| `bgp.passive` | `# exabgp.bgp.passive: in Ze, set per-peer: connection { remote { accept false; } }` |
| `cache.nexthops` | `# exabgp.cache.nexthops: deprecated, always enabled in Ze` |
| `debug.pdb` | `# exabgp.debug.pdb: Python debugger, not applicable to Ze` |
| `api.*` | `# exabgp.api.<key>: ExaBGP API, Ze uses YANG RPC over plugin sockets` |
| `profile.*` | `# exabgp.profile.<key>: Python profiling, not applicable to Ze` |
| `pdb.*` | `# exabgp.pdb.<key>: Python debugger, not applicable to Ze` |

The env file parsing goes in `internal/exabgp/migration/` alongside the existing config migration.

## RFC Documentation

N/A -- no new RFC behavior. BGP port 179 default is already documented.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-listener-registry.md`
- [ ] Summary included in commit
