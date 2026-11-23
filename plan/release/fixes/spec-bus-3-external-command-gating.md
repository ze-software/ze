# Spec: bus-3-external-command-gating

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P0 |
| Release reason | Security fail-open boundary: external plugin processes currently receive `aaa.ReservedInternalPrefix` on `dispatch-command` / `dispatch-command-args`, unconditionally bypassing AAA for every non-BGP-send daemon command, and `route-install` / `inject-wire-route` perform no caller permission check |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-explicit-plugin-and-attach-config.md` |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-23 |

Split from `plan/release/fixes/spec-explicit-plugin-and-attach-config.md` (Child 3 of 3, per R-8) to isolate the P0 D-8 external command gating and AAA bypass closure on top of the `bus { endpoint, flow }` grammar.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Enforce owner decision D-8 ("A flow's command list is the permission for every command an external program sends, not only for a message toward a peer") and Q6 (Option A, 2026-11-23), closing the live AAA fail-open boundary found during Phase 1 research:

1. **Remove `ReservedInternalPrefix` AAA bypass from external plugins (`internal/component/plugin/server/dispatch.go`):**
   - Today `dispatchCommandPeerResponse`, `dispatchCommandArgsResponse`, and `opUpdateRoute` stamp `Username = internalPluginIdentity(proc.Name())` (`\x00ze:internal:plugin:<name>`) on every plugin process — including external TLS subprocesses (`proc.Config().RunsExternalProgram()`). `authz.Store.authorize` unconditionally allows `aaa.ReservedInternalPrefix`, so any external plugin can run arbitrary `show *`, `clear *`, `request *`, `create/delete bgp peer`, and plugin-to-plugin commands with zero permission check.
   - Under this spec, external plugins never receive `ReservedInternalPrefix` and fail closed on every engine command/RPC unless explicitly granted by a `bus { flow ... { command [ ... ]; } }` targeting that endpoint.

2. **Two-part permission vocabulary in `bus { flow { command [ ... ]; } }` (Q6 Option A):**
   - **Subsystem capability tokens** on protocol/subsystem flows (`from bgp; command [ update refresh raw ];`, `from system-rib; command [ route-install ];`, etc.) gate typed engine RPCs (`update-route`, `forward-cached`, `relay-stored-route`, `route-install`, `route-remove`, `inject-wire-route`, and BGP peer-send commands).
   - **`from cli;` flows with command path prefixes** (`flow read-only { from cli; to app; command [ "show bgp peer" "show interface" ]; }`), reusing `authz`'s command prefix matcher, gate general daemon commands dispatched over `dispatch-command` / `dispatch-command-args`.
   - Operators at the CLI, SSH, Web, REST, gRPC, and MCP remain governed by AAA (`OperatorSender`), never gated by bus flows.
   - Compiled-in internal plugins retain internal dispatch (while external forks of third-party programs are gated by flows).

3. **Observable refusal and counter (AC-12 of parent spec):**
   - Every command refused for an external program returns an operator-clear error naming the endpoint, the refused command, and the `bus { flow ... { command [ ... ]; } }` line that would grant it, increments a registered Prometheus counter, and raises a declared doctor problem code.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - external plugin RPCs (`dispatch-command`, `update-route`, `route-install`, `emit-event`)
  -> Constraint: update `internal/core/ipc/yang/ze-plugin-engine.yang` description of `dispatch-command`, which currently claims commands run under a trusted internal identity
- [ ] `docs/architecture/api/architecture.md` - "Owner ruling: endpoints and flows on the bus" (D-8)
  -> Decision: a flow's `command` list is the permission for every command an external program sends
- [ ] `docs/architecture/plugin/plugin-system.md` - external vs internal process boundary
  -> Constraint: `proc.Config().RunsExternalProgram()` distinguishes external programs from compiled-in plugins

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/dispatch.go` - `dispatchCommandPeerResponse` and `dispatchCommandArgsResponse` stamp `internalPluginIdentity(proc.Name())` on all plugins including external processes
- [ ] `internal/component/plugin/server/dispatch_registry.go` - `opUpdateRoute`, `opDispatchCommand`, `opDispatchCommandArgs`, `opRouteInstall`, `opRouteRemove`, `opInjectWireRoute`, `opEmitEvent`
- [ ] `internal/component/plugin/server/dispatch_route.go` - `opRouteInstall` and `opRouteRemove` check protocol validity only, with no caller permission check
- [ ] `internal/component/authz/authz.go` - `Store.authorize` unconditionally returns `Allow` for `aaa.ReservedInternalPrefix`
- [ ] `internal/component/bgp/reactor/send_permission.go` - `Peer.maySend` and `filterPermittedPeers` gate only the 10 BGP peer-send paths

**Behavior to preserve:**
- Operators (`OperatorSender`) remain authorized by AAA and are never blocked by bus flows.
- Compiled-in internal plugins keep functioning over DirectBridge and internal dispatch.
- External plugins that hold explicit `bus { flow { command [ ... ]; } }` grants continue to execute granted protocol sends and granted CLI commands.

**Behavior to change:**
- External plugins no longer receive `aaa.ReservedInternalPrefix` on `dispatch-command` / `dispatch-command-args`.
- Every external plugin command/RPC (`dispatch-command`, `dispatch-command-args`, `update-route`, `forward-cached`, `relay-stored-route`, `route-install`, `route-remove`, `inject-wire-route`, `emit-event`) is gated fail-closed against `bus { flow { command [ ... ]; } }`.
- Refusals return the granting `bus { flow }` line, increment a Prometheus counter, and record a doctor-declared refusal code.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| BGP send permission gate | `internal/component/bgp/reactor/send_permission.go` `Peer.maySend`, `filterPermittedPeers` | Gates 10 BGP peer-send paths for `ProcessSender`, exempts `OperatorSender` | Extends: generalizes across all external plugin engine RPCs and command dispatch using `bus { flow { command [...] } }` |
| Authz command prefix matcher | `internal/component/authz/authz.go` `Entry.matches` | Matches canonical command strings against prefix allowlists for AAA profiles | Extends: reuses the same prefix matcher for `from cli` flow `command [ ... ]` grants |
| Engine op dispatch table | `internal/component/plugin/server/dispatch_registry.go` `engineOps` | Single dispatch table for both TLS socket and DirectBridge RPCs | Extends: attaches command-capability gating once in the shared dispatch path so socket and direct paths never diverge |
| Doctor runtime problem codes | `internal/component/plugin/server/doctor_snapshot.go` `refusePluginRequest` | Records refused plugin requests under `diagnostic.CodeDoctorPluginRequestRefused` | Extends: surfaces external command permission refusals with flow-line fix |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- External plugin RPC over TLS (`dispatch-command`, `dispatch-command-args`, `update-route`, `forward-cached`, `relay-stored-route`, `route-install`, `route-remove`, `inject-wire-route`, `emit-event`).

### Transformation Path
1. [owed at design: `bus { flow }` command grants indexed per endpoint and publisher (`bgp`, `system-rib`, `cli`, ...)]
2. [owed at design: fail-closed permission check in `plugin/server` dispatch before forwarding to `Dispatcher` or subsystem handlers]
3. [owed at design: refusal error formatting, counter increment, and doctor reporting]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| External plugin <-> Engine RPC | Authenticated TLS `proc *process.Process` checked against `bus { flow }` command grants | No |

### Changed-Wire Census

**Search evidence:** [owed at design: RPC wire shapes do not change; error responses return standard `StatusError` and `ze-plugin-engine.yang` descriptions update]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [owed at design] | [role] | [carrier] | [evidence] | [disposition] |

### Integration Points
- [owed at design]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | External plugins in existing functional/interop tests that invoke `dispatch-command` or `route-install` can be enumerated and granted explicit flows | Phase 1 census of external test fixtures | Existing tests fail until their test configs add the needed `bus { flow }` | Scoped `plugin`, `exabgp`, and `encode` functional runs | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An in-tree plugin run forked via `./le mock plugin-external` loses an internal command it needs | `./le test functional plugin` external-mode tests fail | Separate third-party external programs from forked compiled-in plugins or grant required flows in external-mode test configs (settle explicitly at design) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | External plugins either retain a live AAA bypass (fail-open) or legitimate granted commands fail (fail-closed false refusal) |
| How is it reverted? | Single spec revert |
| Who else touches this path? | Child 1 (`spec-explicit-plugin-and-attach-config.md`) defines `bus { flow { command [...] } }` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| External plugin invoking ungranted `show bgp peer` or `route-install` | -> | `plugin/server` external command permission gate | `test/plugin/bus-external-command-gating.ci` (planned) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | External program sends an ungranted CLI command via `dispatch-command` (e.g. `show bgp peer`, `clear bgp peer`) | Command is refused (no `ReservedInternalPrefix` bypass), response names the `from cli` flow line that grants it, Prometheus refusal counter increments, and doctor records the refusal | Operators at CLI/SSH/Web/REST/gRPC/MCP are not gated by bus flows |
| AC-2 | External program holds `flow read-only { from cli; to app; command [ "show bgp peer" ]; }` | `show bgp peer` succeeds; `clear bgp peer` and `show interface` are refused | Only matching command prefixes are admitted |
| AC-3 | External program invokes `route-install` / `route-remove` or `inject-wire-route` without a granting subsystem flow vs with one | Refused without the flow (naming the granting flow line and incrementing counter); succeeds with the matching subsystem capability token | Compiled-in internal plugins retain internal dispatch |
| AC-4 | External program sends BGP `update` / `refresh` / `raw` with vs without a matching `from bgp; command [ ... ];` flow | Granted token succeeds; ungranted token is refused with the flow line and counter increment | Matches existing BGP send permission semantics under the new flow model |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs an external monitoring script granted only `from cli; command [ "show bgp peer" ];` | External TLS RPC -> `plugin/server` command gate -> allows `show bgp peer`, refuses `clear bgp` and `route-install` | `test/plugin/bus-external-command-gating.ci` (planned) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [owed at design] | | | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | No numeric inputs introduced | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bus-external-command-gating` (planned) | `test/plugin/` | AC-1 to AC-4 | |

## Files to Modify
- `internal/component/plugin/server/dispatch.go`, `dispatch_registry.go`, `dispatch_route.go` - remove external `ReservedInternalPrefix` bypass and enforce fail-closed flow command gating
- `internal/component/plugin/server/delivery_graph.go` - index per-endpoint command grants (`from <subsystem>` tokens and `from cli` prefixes)
- `internal/core/ipc/yang/ze-plugin-engine.yang` - update `dispatch-command` description
- `docs/architecture/api/process-protocol.md`, `docs/plugin-development/` - document external command permission gating

## Files to Create
- `test/plugin/bus-external-command-gating.ci` - negative + positive functional proof

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- failing functional test `test/plugin/bus-external-command-gating.ci`
2. [owed at design]

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Short subsystem tokens on protocol flows + `from cli` command prefixes reusing `authz` matcher (Q6 Option A, 2026-11-23) | Tokens only for every CLI command; command prefixes everywhere | Keeps protocol flows concise (`command [ update refresh ]`) without inventing token aliases for every CLI command |
| External plugins never receive `aaa.ReservedInternalPrefix` (2026-11-23) | Keep internal identity bypass on non-send commands | Closes a P0 fail-open security boundary where any external program could invoke arbitrary daemon commands |

## Known Limitations
- External-to-external bus flows are not supported (Q2, 2026-11-23): `from` always names a built-in subsystem/plugin, so no external program can emit arbitrary events to another external program.
