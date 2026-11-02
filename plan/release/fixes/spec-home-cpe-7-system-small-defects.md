# Spec: home-cpe-7-system-small-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Release fix: (1) a listener list with `0.0.0.0` and `::` on one port passes validation, then one bind fails and is only logged, so the service runs half-bound while the commit reports success (P1); (2) `system host` never reaches the appliance's kernel hostname, which stays the image's build-time value (P2, carried here because it is small) |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Parent: `plan/release/fixes/spec-home-cpe-0-umbrella.md` (child 7).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

### Part 1: dual wildcard listeners

A VyOS configuration lists SSH on both `0.0.0.0` and `::` port 22. Ze accepts
the same in `ssh`: validation calls the two addresses non-overlapping, and at
start the SSH server binds the first and logs the second's `EADDRINUSE`. On
Linux with `net.ipv6.bindv6only=0` (the default, and Go's default for a `[::]`
TCP listen) the `::` socket is dual-stack: it already accepts IPv4 as mapped
addresses, so it owns IPv4 port 22 too, and the two binds cannot both succeed.
Which one wins depends on list order. The defect is general: every
`ze:listener` service that binds through Go's default dual-stack listen shares
the validation rule (`ipsConflict`), so web, gNMI, API and the others accept
the same configuration.

Goal: validation refuses, for a dual-stack service, `::` together with any
IPv4 address on the same protocol and port, with an error naming both entries
and the fix ("remove `0.0.0.0`: `::` already accepts IPv4 on this service"). A
service that forces `IPV6_V6ONLY` (L2TP does, in `UDPListener.Start`,
`internal/component/l2tp/listener.go`) registers that fact, and for it the two
families stay non-overlapping. Independently, SSH binds all its listeners or
none: an additional listener that fails to bind fails the SSH server's start
the way a primary-listener failure does today, and a reload that changes the
SSH listener list either binds every new address or leaves the previous
listeners serving and refuses the reload. Today `zessh.Server.Reload` is a
no-op ("SSH server config changes require restart"), and the reload listener
migration (`listenerMigrator.buildChanges`, `cmd/ze/hub/listener_migrate.go`)
covers web, looking glass, MCP, REST and gRPC but not SSH, so a changed SSH
listener list is accepted and applied to nothing until a restart.

### Part 2: kernel hostname on the appliance

`system host` is only Ze's own identity (`ExtractSystemConfig`). On the
appliance the kernel hostname is the gokrazy instance's build-time
`Hostname` (`gokrazy/ze/config.json`: `"ze"`), so every appliance is `ze` in
its prompt, its syslog and anything that reads `uname -n`. Goal: on the
appliance build, Ze sets the kernel hostname from `system host` at start and on
every reload that changes it; on a distro install the operating system owns the
hostname and Ze leaves it alone, as today.

## Required Reading

### Architecture Docs
- [ ] `internal/component/config/yang/modules/ze-types.yang`, grouping `listener`, leaf `ip` description: "Config validation refuses two listeners that share a protocol and a port when their addresses overlap. A wildcard address overlaps every address of its own family, and an IPv4 address never overlaps an IPv6 address."
  → Constraint: the last clause is false for a dual-stack `::`; it is rewritten with the rule of this spec
- [ ] `docs/guide/configuration.md`, section "Named Listeners" - the listener rule the operator reads; no section of the page documents `system host` (only `docs/guide/bmp.md` mentions `system.host`, for `sysName`)
  → Decision: the page gains a "System Host Name" section for `system host`, its validation and the appliance-only kernel effect
- [ ] `docs/guide/appliance.md`, section "Building and installing an appliance (end to end)", "From a JSON config (recommended)" - `identity.hostname` in `appliance.json` feeds the CA common name and listings, not the image hostname
  → Constraint: the page must say which name the running appliance carries after this change
- [ ] `docs/contributing/ze-go-style.md`, `ai/doors/system.md` - before the first Go edit

### RFC Summaries (Scope: protocol)
- [ ] none for Part 1 (socket semantics, not a protocol)
- [ ] Part 2 hostname syntax: RFC 1123 Section 2.1 governs host name syntax; `rfc/full/rfc1123.txt` is absent and is fetched (`curl -o rfc/full/rfc1123.txt https://www.rfc-editor.org/rfc/rfc1123.txt`) and quoted by the implementing phase before the validator is written. No summary is written (validation scope does not grow)
  → Constraint: the kernel limit is 64 bytes (`sethostname` returns EINVAL above it); the validator refuses longer values

**Key insights:**
- Go's `net.Listen("tcp", "[::]:22")` on Linux opens a dual-stack socket; `net.Listen("tcp", "0.0.0.0:22")` after it fails with `EADDRINUSE`, and the reverse order fails the same way.
- Precedent for a per-service listener property registered by the service: `config.RegisterListenerFreebind` (`internal/component/config/listener.go`), called from `internal/plugins/dnsresolver/register.go` and `internal/plugins/as112/register.go`.
- Precedent for reload rollback of listeners: `listenerMigrator.migrateListeners` and `rollbackAppliedListeners` (`cmd/ze/hub/listener_migrate.go`) reconfigure each `Reconfigurable` service and restore the previous addresses of every service already applied when one fails; `listener_migrate_lg_test.go` is the per-service test precedent.
- Precedent for a booted-appliance proof: the `./le test qemu mpls-boot` action (`internal/le/test/qemu/mplsboot.go`, registered in `actions.go`). `./le test functional appliance` runs the build-host CLI `.ci` files under `test/appliance/` and boots no image.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ssh/ssh.go` `Server.Start` - binds `config.Listen` with `net.ListenConfig.Listen` (error returned), then each of `ListenAddrs[1:]`: on error, `s.logger.Error("SSH extra listener failed to bind", ...)` and `continue`. `Server.Reload` is a no-op ("SSH server config changes require restart; no hot-reload for v1")
- [ ] `cmd/ze/hub/service_ssh.go` `sshBuildImpl` - a `Server.Start` error logs "SSH server failed to start" at warn and returns no server; the daemon runs on without SSH
- [ ] `cmd/ze/hub/listener_migrate.go` `listenerMigrator.buildChanges` - the reload change set covers web, looking glass, MCP, REST and gRPC; SSH is absent, and `zessh.Server` implements no `Reconfigurable`
- [ ] `internal/component/config/listener.go` - `ipsConflict`: "Cross-family: IPv4 and IPv6 never conflict"; `FindListenerConflict` and `ValidateListenerConflicts`
- [ ] callers: `runValidation` in `internal/component/config/cli/cmd_validate.go` (`ze config validate`), `CreateReactorFromTree` in `internal/component/bgp/config/loader_create.go` (daemon load), `validateChaosListenerConflicts` in `internal/chaos/orchestrator/conflict.go`
- [ ] `internal/component/config/listener_test.go` `TestValidateListenerConflicts_NoConflict` - its subtest "cross-family ipv6-wildcard vs ipv4-specific" (web `::` and ssh `10.0.0.1` on one port) asserts the defect as correct
- [ ] `internal/component/l2tp/listener.go` `UDPListener.Start` - its `lc.Control` forces `IPV6_V6ONLY` on IPv6 listeners
- [ ] `internal/component/config/system/system.go` `ExtractSystemConfig` - `Host` defaults to `os.Hostname()` (or "unknown" when that fails), else `system host` expanded; nothing calls `sethostname`
- [ ] `cmd/ze/hub/main_system.go` - system appliers at start (`applyConsole`, `applyHostTuning`, `applyConntrack`, called from `main.go`) and their `FromMap` twins at reload (called from `runReloadContext`, `main_reload.go`, after the transaction is accepted); each is best-effort and reports a failure as a warn line (`applyConsole`'s comment: "never blocks startup")
- [ ] `gokrazy/ze/config.json` - the `Hostname` key, `"ze"`
- [ ] grep for `Sethostname` outside vendor: only `internal/appliance/dockerhost/guest_linux.go` (the lab docker host guest)

**Behavior to preserve:**
- `::` alone keeps accepting IPv4 (dual-stack) on every service that does so today; the VyOS mapping (child 10) relies on it.
- Distinct specific addresses of different families on one port stay valid.
- L2TP's V6ONLY behaviour.
- `system host` as Ze's identity everywhere it is used today.

**Behavior to change:**
- Validation refuses `::` with an IPv4 address on one protocol and port for dual-stack services; the `TestValidateListenerConflicts_NoConflict` subtest that asserts the opposite moves to the refused set, with a `ze-test-judge` approval (its old expectation is wrong against Linux dual-stack bind semantics).
- SSH start fails when any configured listener fails to bind, with no listener left bound.
- A reload that changes the SSH listener list rebinds SSH through the listener migration, and on a bind failure keeps the previous listeners and refuses the reload.
- On the appliance, the kernel hostname follows `system host`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Listener conflict validation | `internal/component/config/listener.go` `CollectListeners`, `FindListenerConflict`, `conflicts`, `ipsConflict`; `docs/architecture/config/syntax.md`, the `ze:listener` row | collects every `ze:listener` endpoint and refuses overlapping protocol, address and port; a wildcard overlaps only its own family | extends: `ipsConflict` (through `conflicts`) learns that `::` on a dual-stack service overlaps every IPv4 address; one rule still serves every caller |
| Per-service listener property registry | `internal/component/config/listener.go` `RegisterListenerFreebind`, `listenerFreebindFor` | a service records from its own `register.go` that it binds with `IP_FREEBIND`; each `ListenerEndpoint` carries the answer | extends: the V6ONLY fact is registered the same way and carried on the endpoint, never a service name list in `ipsConflict` |
| Validation callers | `internal/component/config/cli/cmd_validate.go` (`ze config validate`), `internal/component/bgp/config/loader_create.go` (daemon load, `ValidateListenerConflicts`), `internal/chaos/orchestrator/conflict.go`; `test/plugin/config-validate-agrees-with-boot.ci` | offline validation and boot run the same conflict check | extends: they inherit the new rule unchanged, so validate and boot keep agreeing |
| L2TP V6ONLY socket | `internal/component/l2tp/listener.go` (the `lc.Control` that sets `IPV6_V6ONLY`) | forces IPv6 listeners IPv6-only so `0.0.0.0` and `::` on 1701 both bind | extends: L2TP registers the fact the socket code already enforces; the socket code is unchanged |
| SSH listener binding | `internal/component/ssh/ssh.go` `Server.Start` | binds `config.Listen` (error returned), then logs and skips each extra listener that fails | replaces: the log-and-continue path is deleted; every listener binds or none does |
| Reload listener migration | `cmd/ze/hub/listener_migrate.go` `Reconfigurable`, `listenerMigrator.buildChanges`, `migrateListeners`, `rollbackAppliedListeners` (its `// Design:` header names `docs/architecture/web-interface.md`, which carries no section on the migration) | rebinds web, looking glass, MCP, REST and gRPC on reload, and restores every already-applied service's previous addresses when one fails | extends: `zessh.Server` implements `Reconfigurable` (bind every new address or none, keep the old set on failure), and SSH joins the change set through a setter called from the SSH registration file, as `setMCP` is from `register_mcp.go`; the no-op `Server.Reload` is deleted |
| System identity | `internal/component/config/system/system.go` `ExtractSystemConfig` | `Host` from `system host` with `$ENV` expansion, else `os.Hostname()` | extends: host name validation after expansion; the identity use is kept |
| Hub system appliers | `cmd/ze/hub/main_system.go` `applyConsole`, `applyHostTuning`, `applyConntrack` and their `FromMap` reload twins | apply `system` config to the host at start and on reload | extends: the hostname applier joins them at the same call sites |
| Appliance-only runtime features | `cmd/ze/setup_features_appliance.go` (`ze_appliance` build tag) | the tag reserved for on-device runtime features (`ai/doors/appliance.md`) | extends: the hostname applier is a `ze_appliance` file with a no-op sibling, so a distro build never calls `sethostname` |
| `Sethostname` in the lab guest | `internal/appliance/dockerhost/guest_linux.go` `linuxSystem.Sethostname` | sets the hostname of the lab Docker host guest | does not apply: lab tooling, not the shipped daemon; the applier calls `unix.Sethostname` itself |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: any `ze:listener` list (SSH `server` entries and the others); `system host`.

### Transformation Path
1. `CollectListeners` builds `ListenerEndpoint`s; each carries the service's registered V6ONLY flag (new, registered like freebind).
2. `ipsConflict` (or its caller `conflicts`): an IPv6 unspecified address on a service without V6ONLY overlaps every IPv4 address on the same protocol and port.
3. The conflict error, shared by `ze config validate` and daemon load, names both entries, the service, and the fix.
4. SSH `Start`: every listener bound or none; on a failure the bound ones are closed and the error, naming the address, returned. `sshBuildImpl` keeps its existing handling of a start error (see Open Decisions).
5. SSH reload: `listenerMigrator.buildChanges` adds the SSH change when the listener list differs; `Server.Reconfigure` binds every added address before it closes any removed one, and on a bind failure closes what it bound and returns the error with the old set still serving; `migrateListeners` then rolls back the services applied before it and the reload is refused.
6. Hostname: a system applier, compiled only into the appliance build, compares the configured `system host` leaf (expanded) with the kernel hostname and calls `sethostname` when they differ. It reads the leaf, never `SystemConfig.Host`, because `ExtractSystemConfig` fills `Host` from `os.Hostname()` or "unknown" when the leaf is unset; an unset leaf leaves the kernel hostname alone. A `sethostname` failure leaves the kernel name as it was, logs one warn line naming the value, the error and the fix, and `ze doctor` reports the mismatch; like the other system appliers it runs after the reload is accepted, so it does not refuse the reload.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to validation | listener endpoints | No |
| SSH to kernel | `listen(2)` | No |
| Reload to SSH | `listenerMigrator` change set, `Reconfigurable` | No |
| Hub to kernel | `sethostname(2)` on the appliance | No |

### Integration Points
- `config.RegisterListenerFreebind` pattern for the new V6ONLY registration; L2TP registers from `internal/component/l2tp/register.go`.
- `listenerMigrator` (`cmd/ze/hub/listener_migrate.go`): SSH joins the change set through its own setter, called from `cmd/ze/hub/register_ssh.go`, as `register_mcp.go` calls `setMCP`.
- `cmd/ze/hub/main_system.go` appliers, with a build-tagged file for the appliance (the `ze_appliance` tag, `cmd/ze/setup_features_appliance.go` precedent).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes (design) | one validation rule shared by all callers |
| No unintended coupling | Yes (design) | the V6ONLY fact is registered by the owning service |
| No duplicated functionality | Yes (design) | reuses `FindListenerConflict` |
| Zero-copy preserved | N-A | config path |
| Registration over hardcoding, outbound | Yes (design) | V6ONLY registered per service |
| Registration over hardcoding, inbound | Yes (design) | no service name appears in `ipsConflict` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Every `ze:listener` service other than L2TP binds `::` dual-stack | Go default; grep for `IPV6_V6ONLY` finds only L2TP | a V6ONLY service would get a false refusal | grep `IPV6_V6ONLY` and `ListenConfig.Control` across services at implementation | unvalidated |
| A-2 | Ze runs as root, or with `CAP_SYS_ADMIN` in the initial UTS namespace, on the appliance | gokrazy runs packages as root; `dropPrivileges` is a no-op without `ze.user` | `sethostname` fails EPERM | appliance boot test | unvalidated |
| A-3 | Existing configs in `test/` and `docs/` do not list `::` with an IPv4 address on one port for a dual-stack service | none | tests go red under the new refusal | grep before Phase 2; each hit is a config that never fully worked | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A host with `net.ipv6.bindv6only=1` makes `::` IPv6-only, so the refusal is stricter than needed | none on Ze's kernel (default 0) | the error names the fix that works on both settings: list `::` only, or set the specific addresses |
| R-2 | gokrazy rewrites the hostname after Ze sets it | hostname reverts | read gokrazy's init: it sets the hostname once at boot before packages start |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a valid configuration refused at commit (Part 1); a wrong hostname (Part 2) |
| How is it reverted? | single commit revert per part |
| Who else touches this path? | every `ze:listener` service; `release/fixes/spec-network-services-one-off-defects.md` (listeners appearing later) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` on SSH with `0.0.0.0` and `::` port 22 | → | `FindListenerConflict` | `test/parse/listener-dual-wildcard-refused.ci` |
| SSH start with an extra listener that cannot bind | → | `Start` | `TestStartFailsWhenExtraListenerFails` in `internal/component/ssh/ssh_test.go` |
| reload changing the SSH listener list to one that cannot bind | → | `listenerMigrator.migrateListeners` → `Server.Reconfigure` | `TestReloadSSHListenerBindFailureKeepsPrevious` in `cmd/ze/hub/listener_migrate_ssh_test.go` |
| appliance boot with `system host cpe-lab` | → | appliance hostname applier | `./le test qemu appliance-hostname` (new action, `internal/le/test/qemu/hostname.go`, modelled on `mplsboot.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | SSH `0.0.0.0` and `::`, both port 22 | `ze config validate` and commit refuse; the error names both entries, says `::` already accepts IPv4 on this service, and gives the fix |
| AC-2 | SSH `::` and `192.0.2.1`, port 22 | refused the same way |
| AC-3 | SSH `::` alone | accepted; a client reaches it over IPv4 and IPv6 |
| AC-4 | SSH `192.0.2.1` and `2001:db8::1`, port 22 | accepted, both bound |
| AC-5 | L2TP `0.0.0.0` and `::` on 1701 | accepted (V6ONLY registered), both bound |
| AC-6 | SSH with a second listener whose address is not on the host | `Server.Start` returns an error naming the address and the bind error, and no listener of the set is left bound |
| AC-11 | Running SSH on `127.0.0.1`; a reload changes the list to `127.0.0.1` plus an address not on the host | the reload is refused naming the address; SSH still serves on `127.0.0.1` and on nothing else; a service migrated earlier in the same reload is back on its previous addresses |
| AC-12 | Running SSH on `127.0.0.1`; a reload changes the list to `::1` | SSH serves on `::1` and no longer on `127.0.0.1`, without a restart |
| AC-7 | Appliance, `system host cpe-lab` | `uname -n` is `cpe-lab` after boot; changing it to `cpe-lab2` and committing changes it without reboot |
| AC-8 | Appliance, `system host` unset | kernel hostname unchanged (the image's) |
| AC-9 | `system host` longer than 64 bytes, or not a valid host name after expansion | commit refused, naming the value, the rule broken, and the fix |
| AC-10 | Non-appliance build (distro and the others without `ze_appliance`), `system host` set | kernel hostname unchanged, `sethostname` never called; `docs/guide/configuration.md` says so |
| AC-13 | Appliance, `system host cpe-lab`, `sethostname` fails (for example EPERM) | the kernel keeps its previous name; one warn line names the value, the error and the fix; `ze doctor` reports the kernel hostname differs from `system host`; the commit or reload is not refused (the applier runs after acceptance, like the other system appliers) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Pastes a converted VyOS SSH block with both wildcards | validate → refusal with fix → `::` only → SSH on both families | `listener-dual-wildcard-refused.ci`, `TestStartServesDualStackWildcardOverBothFamilies` |
| 2 | Sets the CPE's name | commit → hostname applier → kernel | `./le test qemu appliance-hostname` |
| 3 | Moves SSH to another address with a reload | commit → listener migration → SSH rebound, or refused with the old listeners kept | `TestReloadSSHListenerBindFailureKeepsPrevious`, `TestReloadSSHListenerMoves` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIPsConflictDualStackWildcard` | `internal/component/config/listener_test.go` | AC-1, AC-2, AC-4 | |
| `TestIPsConflictV6OnlyService` | `internal/component/config/listener_test.go` | AC-5 | |
| `TestValidateListenerConflicts_NoConflict` (changed) | `internal/component/config/listener_test.go` | its subtest "cross-family ipv6-wildcard vs ipv4-specific" moves to the refused cases; the ipv4-wildcard vs ipv6-specific subtest stays accepted. Owes a `ze-test-judge` approval | |
| `TestStartServesDualStackWildcardOverBothFamilies` | `internal/component/ssh/ssh_test.go` | AC-3: `Start` on `[::]` with a free port; an SSH handshake succeeds over `127.0.0.1` and over `::1` | |
| `TestStartBindsSpecificAddressesOfBothFamilies` | `internal/component/ssh/ssh_test.go` | AC-4: `127.0.0.1` and `::1` on one port, both reachable | |
| `TestUDPListenerV6OnlyBesideIPv4Wildcard` | `internal/component/l2tp/listener_test.go` | AC-5: `newUDPListener` on `0.0.0.0` and on `::` with one port, both `Start` without error | |
| `TestStartFailsWhenExtraListenerFails` | `internal/component/ssh/ssh_test.go` | AC-6: the error names the address; the first listener's port is free again afterwards | |
| `TestReconfigureAllOrNone` | `internal/component/ssh/ssh_test.go` | AC-11 server half: `Reconfigure` to a set with an unbindable address fails, `Addresses` is unchanged and the old listener still completes a handshake | |
| `TestReloadSSHListenerBindFailureKeepsPrevious` | `cmd/ze/hub/listener_migrate_ssh_test.go` | AC-11: failed-reload preservation through `listenerMigrator.reloadListeners`, with a second fake `Reconfigurable` service applied first and asserted rolled back | |
| `TestReloadSSHListenerMoves` | `cmd/ze/hub/listener_migrate_ssh_test.go` | AC-12 | |
| `TestHostnameValidation` | `internal/component/config/system/system_test.go` | AC-9 | |
| `TestHostnameApplierOnlyWhenChanged` | `cmd/ze/hub/main_system_appliance_test.go` (`ze_appliance`) | AC-7, AC-8 with a fake setter: called once on a differing leaf, never when equal or unset | |
| `TestHostnameApplierSetFailureKeepsNameAndWarns` | `cmd/ze/hub/main_system_appliance_test.go` (`ze_appliance`) | AC-13: the fake setter returns EPERM; the fake kernel name is unchanged, one warn line carries the value, the error and the fix, and the applier returns nothing that refuses the reload | |
| `TestHostnameApplierNonApplianceNeverSets` | `cmd/ze/hub/main_system_nonappliance_test.go` (`!ze_appliance`) | AC-10: distro-hostname preservation: with `system host` set and differing from the fake kernel name, the setter is never called | |
| `TestDoctorReportsHostnameMismatch` | `internal/component/config/system/doctor_test.go` | AC-13 doctor half | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `system host` length | 1-64 bytes | 64 | 0 (empty) | 65 |
| host name label length | 1-63 | 63 | 0 | 64 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `listener-dual-wildcard-refused` | `test/parse/listener-dual-wildcard-refused.ci` | AC-1, AC-2 | |
| `listener-dual-wildcard-v6only-accepted` | `test/parse/listener-dual-wildcard-v6only-accepted.ci` | AC-5 | |
| `appliance-hostname` | `./le test qemu appliance-hostname` (new action, `internal/le/test/qemu/hostname.go`, registered in `internal/le/test/qemu/actions.go` beside `mpls-boot`): boots an image with `system host cpe-lab`, reads `uname -n`, commits `cpe-lab2`, reads it again, then boots one with the leaf unset | AC-7, AC-8 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | no protocol behaviour changes; an SSH client reaching `::` over IPv4 is covered by `TestStartServesDualStackWildcardOverBothFamilies` (AC-3) | |

## Files to Modify
- `internal/component/config/listener.go` - dual-stack rule, V6ONLY registration
- `internal/component/config/yang/` `ze-types.yang` - `listener` grouping description
- `internal/component/l2tp/register.go` - register V6ONLY for the L2TP `server` listener service
- `internal/component/config/listener_test.go` - `TestValidateListenerConflicts_NoConflict` subtest moved (judged)
- `internal/component/ssh/ssh.go` - `Start` binds all or none; `Addresses` and `Reconfigure` added; the no-op `Reload` deleted
- `cmd/ze/hub/listener_migrate.go` - SSH in `buildChanges` and its setter
- `cmd/ze/hub/register_ssh.go` - hands the built SSH server to the migrator
- `internal/component/config/system/system.go`, `internal/component/config/system/yang/ze-system-conf.yang` - host name validation and description
- `internal/component/config/system/doctor.go` - the hostname mismatch check, registered in `registerSystemDoctorChecks`
- `cmd/ze/hub/main.go`, `cmd/ze/hub/main_reload.go` - call sites of the hostname applier beside `applyConsole` and `applyConsoleFromMap`
- `internal/le/test/qemu/actions.go` - the `appliance-hostname` action
- `docs/guide/configuration.md`, `docs/guide/appliance.md`
- `docs/architecture/config/syntax.md` - declared by `listener.go` and `system.go`. The `ze:listener` row says `ValidateListenerConflicts` "rejects overlapping `ip:port` pairs"; it stays true but gains the dual-stack rule (`::` on a service without V6ONLY overlaps every IPv4 address) and the V6ONLY registration. Nothing it says about `system host` becomes wrong
- `docs/architecture/system-architecture.md` - declared by `ssh.go`; it describes the SSH subsystem, not the extra-listener bind, so nothing becomes wrong; checked at implementation against the all-or-none bind
- `docs/architecture/hub-architecture.md` - declared by `main_system.go`; it names no system applier list (only `newResolvers`, `registerPluginDNSResolver` and the boot effects), so nothing becomes wrong; the appliance hostname applier is documented in `docs/guide/appliance.md`
- `docs/features/ai-first.md` - declared by `internal/component/config/system/doctor.go`; it names the DNS resolver's doctor codes as examples of codes the owning package declares, which the new hostname code (declared in the `system` package beside `codeConfigPlatformMismatch`) also follows, so nothing becomes wrong
- `docs/architecture/core-design.md` - declared by `internal/le/test/qemu/actions.go`; it describes how an le area registers its action table, which one more action in that table keeps true
- `docs/research/l2tpv2-ze-integration.md` - declared by `l2tp/listener.go`; the socket code is unchanged and the registration lives in `register.go`, so nothing becomes wrong

## Files to Create
- `cmd/ze/hub/main_system_appliance.go` (build tag `ze_appliance`) and its no-op sibling `cmd/ze/hub/main_system_nonappliance.go` (`!ze_appliance`), with their tests `main_system_appliance_test.go` and `main_system_nonappliance_test.go`
- `cmd/ze/hub/listener_migrate_ssh_test.go`
- `test/parse/listener-dual-wildcard-refused.ci`, `listener-dual-wildcard-v6only-accepted.ci`
- `internal/le/test/qemu/hostname.go`, `hostname_test.go` - the booted appliance proof

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | descriptions; host name pattern on `system host` only if `$ENV_VAR` values stay accepted (validation after expansion otherwise) |
| YANG validation constraints | Yes | length 1..64 where expansion allows |
| YANG custom validators | Yes | host name after expansion |
| CLI commands/flags | N-A | none |
| CLI grammar | N-A | none |
| Editor autocomplete | N-A | none |
| Functional test for new RPC/API | N-A | none |
| Pipe completeness | N-A | none |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | `ze doctor` reports a kernel hostname different from `system host` on the appliance |
| Prometheus counters/metrics | No | none |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/` system entry (appliance hostname) |
| 2 | Config syntax changed? | No | semantics only |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md`, section "Named Listeners" (dual-stack rule, SSH reload), and a new "System Host Name" section; `docs/guide/appliance.md`, "From a JSON config (recommended)" (which name the running appliance carries) |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior changed? | No | none |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | none |
| 13 | Route metadata keys? | No | none |
| 14 | Prometheus counters? | No | none |
| 15 | Registered inventory changed? | No | none |
| 16 | Doc source anchors on changed files? | Yes | `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-7-system-small-defects.md` |
| 17 | Existing examples? | Yes | grep `docs/` for listener examples with both wildcards |

## Implementation Steps

1. **Phase: Wiring** - the `.ci` for AC-1, `TestStartFailsWhenExtraListenerFails` and `TestReloadSSHListenerBindFailureKeepsPrevious` exist and fail; the appliance applier file exists as a stub.
2. **Phase: Listener rule** - V6ONLY registration, dual-stack conflict, error text (AC-1..AC-5); the judged `TestValidateListenerConflicts_NoConflict` change; grep fixes for A-3 hits.
3. **Phase: SSH bind all or none** - `Start` (AC-6), `Reconfigure` and the migrator entry (AC-11, AC-12), the dual-family tests (AC-3, AC-4).
4. **Phase: Hostname** - validation (AC-9), appliance applier (AC-7, AC-8), set failure (AC-13), non-appliance no-op (AC-10), doctor, the `appliance-hostname` QEMU action, docs.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | the conflict rule keys on the service's V6ONLY registration, not a service name list |
| Correctness | no listener failure is log-only |
| Rule: user-facing-errors | both refusals say what, why, and the exact fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Refusal | `.ci` output asserts the error text |
| SSH all or none | `TestStartFailsWhenExtraListenerFails`, `TestReconfigureAllOrNone`, `TestReloadSSHListenerBindFailureKeepsPrevious` pass |
| Hostname | `./le test qemu appliance-hostname` reads `uname -n`; `TestHostnameApplierNonApplianceNeverSets` and `TestHostnameApplierSetFailureKeepsNameAndWarns` pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | host name length and characters before `sethostname` |
| Fail-open | a failed bind never leaves a partial listener set reported as success, at start or at reload |
| Exposure | a reload that moves SSH to a new address never serves it unauthenticated; the implementing phase reads the migrator's `checkReloadExposure` and `markAuthenticated` and records whether SSH, which always authenticates, needs an entry there |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A-1 refuted for a service | register V6ONLY for it, or make it dual-stack; never special-case its name |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

No open row records either defect (grep of `plan/journal/` for `EADDRINUSE`, `dual-stack`, `Sethostname`, `kernel hostname`). `plan/journal/validate-accepts-what-start-refuses.md` is this defect's class; the implementing phase adds no row because this spec fixes it.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Refuse at validation for dual-stack services | bind `::` with `IPV6_V6ONLY` everywhere so both entries succeed | V6ONLY everywhere silently drops IPv4 from every config that lists `::` alone, which is what the VyOS mapping and most operators write; refusal changes no working config and names the fix |
| V6ONLY registered per service | hardcode L2TP in `ipsConflict` | registration over enumeration (principles) |
| SSH binds all or none | keep logging | a half-bound service reported as started is a silently wrong state |
| SSH listener changes reach a running daemon through `listenerMigrator` (SSH implements `Reconfigurable`) | refuse any reload that changes the SSH listener list, naming a restart as the fix | the migrator already sequences, rolls back and refuses for five services; a sixth is one registration, while a refusal would leave SSH the only management listener that needs a restart |
| A failed `sethostname` warns and raises a `ze doctor` finding, and does not refuse the reload | refuse the commit | the system appliers run after the transaction is accepted (`runReloadContext`) and are best-effort by design (`applyConsole`); the failure cannot be known at verify without performing the change |
| The booted proof is a `./le test qemu` action | a `.ci` under `test/appliance/` | `./le test functional appliance` runs the build-host CLI and boots nothing; `mpls-boot` is the precedent for a booted check |
| Hostname set only on the appliance build | always call `sethostname` | on a distro, systemd-hostnamed owns the name and Ze usually runs unprivileged |
| Unset `system host` leaves the kernel name | set it to a default | the image's name is a deliberate build choice |

### Open decisions

| Question | Evidence | Reading this spec proceeds under |
|----------|----------|----------------------------------|
| Should an SSH bind failure at boot fail `ze start`, rather than leave the daemon running without SSH? | `sshBuildImpl` (`cmd/ze/hub/service_ssh.go`) logs "SSH server failed to start" at warn and returns no server when `Server.Start` errors, which is today's behaviour for a primary-listener failure; the Task's "never a log line" cannot hold at boot without changing that policy for every SSH start error | keep the hub's policy: an extra listener's failure now behaves exactly as a primary-listener failure (no SSH, one warn line naming the address), and the doctor's listener probe reports the address. Changing the boot policy is the owner's call |

## Known Limitations

- `/etc/hosts` and the domain name (`system domain`) are not written to the kernel (`setdomainname`); NIS domain has no use on a CPE.

## Checklist

### Pre-Spec Verification
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Existing Mechanisms table names every mechanism this spec extends or replaces
- [ ] `rfc/full/rfc1123.txt` fetched and quoted before the host name validator is written
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass: the scoped package tests of `internal/component/config`, `internal/component/config/system`, `internal/component/ssh`, `internal/component/l2tp` and `cmd/ze/hub` (once with `ze_appliance` and once without, so both applier files run), `./le test functional parse`, `./le test functional plugin` (`config-validate-agrees-with-boot.ci`), and `./le test qemu appliance-hostname`; then `./le verify changed` over the closing commits
- [ ] `ze-test-judge` approval recorded for the `TestValidateListenerConflicts_NoConflict` change, and its `Weakening-approved:` line in the commit
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for the `system host` length and label length
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests: N-A, no protocol behaviour changes

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
