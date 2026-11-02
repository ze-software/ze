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
| Updated | 2026-10-31 |

Parent: `plan/home-cpe/spec-home-cpe-0-umbrella.md` (child 7).

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
service that forces `IPV6_V6ONLY` (L2TP does, `internal/component/l2tp/listener.go`
l.112-129) registers that fact, and for it the two families stay
non-overlapping. Independently, an additional SSH listener that fails to bind
fails the start (and the reload), never a log line.

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
- [ ] `internal/component/config/yang/` grouping `listener` in `ze-types.yang` (about l.171): "Config validation refuses two listeners that share a protocol and a port when their addresses overlap. A wildcard address overlaps every address of its own family, and an IPv4 address never overlaps an IPv6 address."
  → Constraint: the last clause is false for a dual-stack `::`; it is rewritten with the rule of this spec
- [ ] `docs/guide/configuration.md` - listener and `system` sections
- [ ] `docs/guide/appliance.md` - `identity.hostname` in `appliance.json` (l.490) feeds the CA common name and listings, not the image hostname
  → Constraint: the page must say which name the running appliance carries after this change
- [ ] `docs/contributing/ze-go-style.md`, `ai/doors/system.md` - before the first Go edit

### RFC Summaries (Scope: protocol)
- [ ] none for Part 1 (socket semantics, not a protocol)
- [ ] Part 2 hostname syntax: RFC 1123 Section 2.1 governs host name syntax; `rfc/full/rfc1123.txt` is absent and is fetched (`curl -o rfc/full/rfc1123.txt https://www.rfc-editor.org/rfc/rfc1123.txt`) and quoted by the implementing phase before the validator is written. No summary is written (validation scope does not grow)
  → Constraint: the kernel limit is 64 bytes (`sethostname` returns EINVAL above it); the validator refuses longer values

**Key insights:**
- Go's `net.Listen("tcp", "[::]:22")` on Linux opens a dual-stack socket; `net.Listen("tcp", "0.0.0.0:22")` after it fails with `EADDRINUSE`, and the reverse order fails the same way.
- Precedent for a per-service listener property registered by the service: `config.RegisterListenerFreebind` (`internal/component/config/listener.go` l.287), called from `internal/plugins/dnsresolver/register.go` and `internal/plugins/as112/register.go`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ssh/ssh.go` `Start` - binds `config.Listen` with `net.ListenConfig.Listen` (l.531, error returned), then each of `ListenAddrs[1:]` (l.553): on error, `s.logger.Error("SSH extra listener failed to bind", ...)` and `continue`
- [ ] `internal/component/config/listener.go` - `ipsConflict` (l.482): "Cross-family: IPv4 and IPv6 never conflict"; `FindListenerConflict` (l.438) and `ValidateListenerConflicts` (l.430)
- [ ] callers: `internal/component/config/cli/cmd_validate.go` l.413 (`ze config validate`), `internal/component/bgp/config/loader_create.go` l.98 (daemon load), `internal/chaos/orchestrator/conflict.go` l.68
- [ ] `internal/component/l2tp/listener.go` l.112-129 - forces `IPV6_V6ONLY` on IPv6 listeners
- [ ] `internal/component/config/system/system.go` `ExtractSystemConfig` - `Host` defaults to `os.Hostname()`, else `system host` expanded; nothing calls `sethostname`
- [ ] `cmd/ze/hub/main_system.go` - system appliers at start and reload (`applyConsole`, `applyHostTuning`, `applyConntrack`)
- [ ] `gokrazy/ze/config.json` l.2 - `"Hostname": "ze"`
- [ ] grep for `Sethostname` outside vendor: only `internal/appliance/dockerhost/guest_linux.go` (the lab docker host guest)

**Behavior to preserve:**
- `::` alone keeps accepting IPv4 (dual-stack) on every service that does so today; the VyOS mapping (child 10) relies on it.
- Distinct specific addresses of different families on one port stay valid.
- L2TP's V6ONLY behaviour.
- `system host` as Ze's identity everywhere it is used today.

**Behavior to change:**
- Validation refuses `::` with an IPv4 address on one protocol and port for dual-stack services.
- SSH start and reload fail when any configured listener fails to bind.
- On the appliance, the kernel hostname follows `system host`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: any `ze:listener` list (SSH `server` entries and the others); `system host`.

### Transformation Path
1. `CollectListeners` builds `ListenerEndpoint`s; each carries the service's registered V6ONLY flag (new, registered like freebind).
2. `ipsConflict` (or its caller `conflicts`): an IPv6 unspecified address on a service without V6ONLY overlaps every IPv4 address on the same protocol and port.
3. The conflict error, shared by `ze config validate` and daemon load, names both entries, the service, and the fix.
4. SSH `Start`: every listener bound or none; on a failure the bound ones are closed and the error returned, so a reload rolls back.
5. Hostname: a system applier, compiled only into the appliance build, compares `system host` (expanded) with the kernel hostname and calls `sethostname` when they differ; an unset `system host` leaves the kernel hostname alone.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to validation | listener endpoints | No |
| SSH to kernel | `listen(2)` | No |
| Hub to kernel | `sethostname(2)` on the appliance | No |

### Integration Points
- `config.RegisterListenerFreebind` pattern for the new V6ONLY registration; L2TP registers.
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
| appliance boot with `system host cpe-lab` | → | appliance hostname applier | `test/appliance/appliance-hostname-from-config.ci` (QEMU appliance suite) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | SSH `0.0.0.0` and `::`, both port 22 | `ze config validate` and commit refuse; the error names both entries, says `::` already accepts IPv4 on this service, and gives the fix |
| AC-2 | SSH `::` and `192.0.2.1`, port 22 | refused the same way |
| AC-3 | SSH `::` alone | accepted; a client reaches it over IPv4 and IPv6 |
| AC-4 | SSH `192.0.2.1` and `2001:db8::1`, port 22 | accepted, both bound |
| AC-5 | L2TP `0.0.0.0` and `::` on 1701 | accepted (V6ONLY registered), both bound |
| AC-6 | SSH with a second listener whose address is not on the host | start fails with the address and the error; on reload the previous listeners stay (rollback) |
| AC-7 | Appliance, `system host cpe-lab` | `uname -n` is `cpe-lab` after boot; changing it to `cpe-lab2` and committing changes it without reboot |
| AC-8 | Appliance, `system host` unset | kernel hostname unchanged (the image's) |
| AC-9 | `system host` longer than 64 bytes, or not a valid host name after expansion | commit refused, naming the value, the rule broken, and the fix |
| AC-10 | Distro build, `system host` set | kernel hostname unchanged; `docs/guide/configuration.md` says so |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Pastes a converted VyOS SSH block with both wildcards | validate → refusal with fix → `::` only → SSH on both families | `listener-dual-wildcard-refused.ci`, AC-3 test |
| 2 | Sets the CPE's name | commit → hostname applier → kernel | `appliance-hostname-from-config.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIPsConflictDualStackWildcard` | `internal/component/config/listener_test.go` | AC-1, AC-2, AC-4 | |
| `TestIPsConflictV6OnlyService` | same | AC-5 | |
| `TestStartFailsWhenExtraListenerFails` | `internal/component/ssh/ssh_test.go` | AC-6 | |
| `TestHostnameValidation` | `internal/component/config/system/system_test.go` | AC-9 | |
| `TestHostnameApplierOnlyWhenChanged` | `cmd/ze/hub/main_system_appliance_test.go` | AC-7, AC-8 with a fake setter | |

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
| `appliance-hostname-from-config` | `test/appliance/appliance-hostname-from-config.ci` | AC-7, AC-8 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | no protocol behaviour changes; OpenSSH reaching `::` over IPv4 is covered by the AC-3 functional test | |

## Files to Modify
- `internal/component/config/listener.go` - dual-stack rule, V6ONLY registration
- `internal/component/config/yang/` `ze-types.yang` - `listener` grouping description
- `internal/component/l2tp/register.go` (or where L2TP registers its listener service) - register V6ONLY
- `internal/component/ssh/ssh.go` - `Start` binds all or none
- `internal/component/config/system/system.go`, `yang/ze-system-conf.yang` - host name validation and description
- `cmd/ze/hub/main_system.go` - call site for the hostname applier
- `docs/guide/configuration.md`, `docs/guide/appliance.md`

## Files to Create
- `cmd/ze/hub/main_system_appliance.go` (build tag `ze_appliance`) and a no-op sibling for other builds
- `test/parse/listener-dual-wildcard-refused.ci`, `listener-dual-wildcard-v6only-accepted.ci`
- `test/appliance/appliance-hostname-from-config.ci`

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
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` (listeners, `system host`), `docs/guide/appliance.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior changed? | No | none |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | none |
| 13 | Route metadata keys? | No | none |
| 14 | Prometheus counters? | No | none |
| 15 | Registered inventory changed? | No | none |
| 16 | Doc source anchors on changed files? | Yes | `./le spec citation anchors spec plan/home-cpe/spec-home-cpe-7-system-small-defects.md` |
| 17 | Existing examples? | Yes | grep `docs/` for listener examples with both wildcards |

## Implementation Steps

1. **Phase: Wiring** - the `.ci` for AC-1 and the SSH unit test exist and fail; the appliance applier file exists as a stub.
2. **Phase: Listener rule** - V6ONLY registration, dual-stack conflict, error text (AC-1..AC-5); grep fixes for A-3 hits.
3. **Phase: SSH bind all or none** (AC-6).
4. **Phase: Hostname** - validation (AC-9), appliance applier (AC-7, AC-8), doctor, docs (AC-10).

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
| Hostname | QEMU appliance test reads `uname -n` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | host name length and characters before `sethostname` |
| Fail-open | a failed bind never leaves a partial listener set reported as success |

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
| Hostname set only on the appliance build | always call `sethostname` | on a distro, systemd-hostnamed owns the name and Ze usually runs unprivileged |
| Unset `system host` leaves the kernel name | set it to a default | the image's name is a deliberate build choice |

## Known Limitations

- `/etc/hosts` and the domain name (`system domain`) are not written to the kernel (`setdomainname`); NIS domain has no use on a CPE.
