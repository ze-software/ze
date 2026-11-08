# Spec: vrf-management-listeners

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | VRF is post-release (`spec-vrf-0-umbrella.md`); putting management listeners in a VRF needs VRF first |
| RFC gaps | - |
| Depends | `plan/post-release/features/spec-vrf-0-umbrella.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator keeps management traffic in its own VRF, so that the web UI,
SSH, gNMI, gRPC, MCP, the looking glass and the metrics exporter are reachable
only from the management network and never from a customer-facing table.

`spec-vrf-0-umbrella.md` designs VRF and names SSH (its AC-13, AC-14) and the
"HTTPS API" as services that bind with `SO_BINDTODEVICE` through a shared
`internal/core/vrfnet` helper. It does not cover the other management
listeners, and it does not cover how config validation judges a listener in a
VRF. This child spec owns both: every `ze:listener` service accepts a VRF, and
the listener conflict check and its error text understand VRFs.

VyOS reference: T9559 (`~/Code/github.com/vyos/vyos-1x`, `git log --grep=T9559`),
two fixes on 2026-11-05 that are the pitfalls here:

| VyOS commit | Defect | Ze criterion |
|-------------|--------|--------------|
| `1b5bdef24` | `vrf default` was taken as a device named `default`; binding the test socket failed and was reported as "port already used" | AC-3 |
| `0949562e1` | A missing or misspelt VRF made the port check report the port as free, so the error went unnoticed | AC-4 |

## Required Reading

### Architecture Docs
- [ ] `plan/post-release/features/spec-vrf-0-umbrella.md`, "Per-VRF Component Spawning" and "VRF-aware socket helper"
  → Decision: service sockets bind with `SO_BINDTODEVICE` per socket; the default VRF binds nothing (its Q4)
- [ ] `docs/architecture/config/syntax.md` - listener conflict detection at parse time
- [ ] `docs/architecture/pki/tls-listeners.md`, "rotation does not rebind the hub listeners"

**Key insights:**
- Ze judges listener conflicts on protocol, address and port from the schema's `ze:listener` lists. A VRF becomes part of that key.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/listener.go` - `ListenerEndpoint{Service, Protocol, IP, Port, Freebind, V6Only}`; `DiscoverListenerServices` walks every `ze:listener` list in the schema, so a new service needs no list edit
- [ ] `internal/component/config/cli/cmd_validate.go` - `config.CollectListeners` then `config.FindListenerConflict`; a clash becomes diagnostic `config-listener-conflict`
- [ ] `internal/component/api/rest/server.go` - `RESTServer.Reconfigure` diffs addresses and binds new ones with `net.ListenConfig` before it returns
- [ ] `internal/component/gnmi/server.go` - `Server.Serve` binds with `net.ListenConfig.Listen`

**Behavior to preserve:**
- A listener with no VRF binds as today
- Conflict detection by protocol, address and port inside one VRF

**Behavior to change:**
- Every `ze:listener` service takes a VRF; its sockets bind to that VRF's device

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Listener discovery | `internal/component/config/listener.go` `DiscoverListenerServices` | Finds every listener from the schema | Extends: the VRF leaf comes from the shared `zt:listener` grouping, so every service gains it at once |
| Listener conflict check | `internal/component/config/listener.go` `ListenerEndpoint`; `cmd_validate.go` `FindListenerConflict` | Refuses two listeners on one protocol, address and port | Extends: the key gains the VRF; default VRF equals no VRF |
| VRF socket helper | `spec-vrf-0-umbrella.md`, planned `internal/core/vrfnet` | Listen and Dial bound to a VRF device | Extends: every management listener binds through it; this spec adds no second helper |
| Listener migration | `cmd/ze/hub/listener_migrate.go` `listenerMigrator.reloadListeners` | Rebinds REST, gRPC, web, LG, MCP and SSH on reload | Extends: a VRF change is an address change and rebinds |
| Doctor | `internal/core/diagnostic/codes.go` `config-listener-conflict` | Names a listener conflict | Extends: a missing VRF gets its own code |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: a `vrf` leaf on each `ze:listener` entry (shape at design, from the umbrella's config syntax)

### Transformation Path
1. Parse: the endpoint carries its VRF; `default` or absent means no VRF
2. Validate: the VRF must exist in config; conflicts compare within one VRF
3. Bind: the service listens through the VRF helper

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to listener | the endpoint value | No |
| Listener to kernel | `SO_BINDTODEVICE` | No |

### Changed-Wire Census
**Search evidence:** no wire changes; the change is socket binding and config.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| none | | | | |

### Integration Points
- every service's bind call; the conflict check; doctor

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | to fill at design |
| No unintended coupling | No | services reach the VRF through the core helper |
| No duplicated functionality | No | one helper, one conflict check |
| Zero-copy preserved where applicable | No | N-A |
| Registration over hardcoding, outbound | No | the grouping carries the leaf; no per-service list |
| Registration over hardcoding, inbound | No | to fill at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every management listener's list uses the `zt:listener` grouping | `ListenerService.ServerList` | Some services need the leaf added by hand | `DiscoverListenerServices` over the full schema | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A listener in the management VRF stays reachable from the default table | AC-1 test reaches it | `SO_BINDTODEVICE`, proven in a netns test |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Management becomes reachable from the wrong network, or not at all |
| How is it reverted? | Single commit revert |
| Who else touches this path? | The VRF umbrella and its children |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Web listener with `vrf mgmt` | → | the VRF helper's listen | `vrf-mgmt-listener.ci` (planned, `test/plugin/`, `option=needs-linux`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Web, SSH, gNMI, gRPC, MCP, LG and the exporter each with `vrf mgmt` | Each answers on an address in `mgmt` and is unreachable from the default table | None |
| AC-2 | Two listeners on the same address and port in different VRFs | Accepted; no conflict | None |
| AC-3 | A listener with `vrf default` on a free port | Accepted exactly as a listener with no VRF; never a lookup of a device named `default`, never a false conflict | None |
| AC-4 | A listener naming VRF `mgnt` that is not configured | Refused at verify and by `ze config validate`, naming the missing VRF and the command that creates it; never reported as a port conflict and never as a free port | None |
| AC-5 | The VRF device is absent at bind time (config valid) | Bind fails with an error naming the VRF device, and doctor reports it with the fix | None |
| AC-6 | Reload moves a listener from the default VRF to `mgmt` | The listener rebinds in `mgmt` before the commit answers; a failed bind rolls back, as listener migration does today | None |
| AC-7 | Status of each listener from every surface | Shows its VRF | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestListenerConflictPerVRF` | `internal/component/config/listener_test.go` | AC-2 | |
| `TestDefaultVRFIsNoVRF` | same | AC-3 | |
| `TestMissingVRFNamedNotConflict` | same | AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N-A: no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vrf-mgmt-listener` | `test/plugin/` | AC-1, AC-6 in netns | |
| `vrf-listener-missing` | `test/parse/` | AC-3, AC-4 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | | | no protocol peer; reachability is proven in netns | |

## Files to Modify
- `internal/component/config/listener.go`, `internal/component/config/cli/cmd_validate.go`
- the bind call of each management listener, through the VRF helper
- `docs/architecture/config/syntax.md`

## Files to Create
- the tests above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `vrf` leaf in the shared listener grouping |
| YANG validation constraints | Yes | leafref to the VRF list, from the umbrella |
| YANG custom validators | Yes | VRF exists |
| CLI commands/flags | Yes | listener status shows VRF |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | Yes | VRF names |
| Functional test for new RPC/API | Yes | above |
| Pipe completeness | N-A | existing commands |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | AC-5 |
| Prometheus counters/metrics | N-A | none |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 6 | User guide | Yes | the VRF guide page the umbrella creates |

## Implementation Steps
1. **Phase: Wiring** - VRF leaf in the grouping, conflict key
2. **Phase: Validation** - AC-2 to AC-5
3. **Phase: Binding and reload** - AC-1, AC-6, AC-7

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Judge a missing VRF from config, not by probing a socket | Probe-bind in the VRF (VyOS) | A probe confuses a missing device with a busy or a free port, which is both T9559 defects |

## Known Limitations
- NTP and other outbound clients in a VRF are the umbrella's dialer work.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test functional parse`, `./le test functional plugin` (netns tests in `./le test qemu all`), `./le job run label config command go test -race ./internal/component/config/...`
