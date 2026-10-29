# Spec: iface-resolution-across-the-plugin-boundary

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | A plugin run as an external process silently binds or queries the wrong interface, or none, on any `os-name` or `mac match` entry: incorrect shipped configuration behavior. |
| RFC gaps | - |
| Depends | - |
| Phase | Sites found and verified; per-site design owed |
| Handoff | - |
| Updated | 2026-10-29 |

## Task

Every caller of `iface.Resolve` or `iface.ResolveDevice` that can run in a
forked plugin either refuses to start as an external process, with a message
that says what is wrong, why, and the exact config that fixes it (as
`runVRRPEngine` in `internal/plugins/vrrp/register.go` does), or resolves the
logical name through a path that crosses the plugin boundary. Extend the
plugin boundary gate so that a new unguarded caller is refused.

Found while fixing the DHCP server, which gets the same guard in its own work
(`runDHCPServerPlugin`, `internal/plugins/dhcpserver/register.go`).

## Required Reading

- [ ] `docs/architecture/iface/logical-name-resolution.md`, the selector table
  and the DHCP server and VRRP binding sections.
  → Constraint: a failed resolution refuses; it never falls back to the
  logical name.
- [ ] `ai/rules/plugins.md`, process boundary.
- [ ] `ai/rules/user-facing-errors.md`, for the refusal text.

## Current Behavior

- [ ] `internal/component/iface/resolve.go`: `globalResolver` is a
  package-level selector table. `setMapping` fills it, called only from the
  iface plugin's config apply, so only in that plugin's process.
- [ ] `internal/component/iface/dispatch.go`: `ResolveDevice` returns the
  logical name unchanged when the name has no mapping and no selector. In a
  forked plugin the table is empty, so every name takes that branch.
- [ ] `internal/plugins/vrrp/register.go`: `runVRRPEngine` refuses to start
  external and logs the reason and the `internal vrrp` fix.
- [ ] `internal/plugins/iface/dhcp/register.go`: the iface/dhcp runner carries
  the same guard.
- [ ] `internal/le/plugin/boundary/pluginboundary.go`: `dangerousCalls` is the
  watch list of the plugin boundary gate; it does not list `Resolve` or
  `ResolveDevice` today.

### Unguarded callers

Found by adding `{ifacePackage, "Resolve"}` and `{ifacePackage, "ResolveDevice"}`
to `dangerousCalls`. Each call was confirmed in source on 2026-10-29.

| File | Function | Call |
|------|----------|------|
| `internal/component/bfd/session_identity.go` | `canonicalRequest` | `ResolveDevice` |
| `internal/plugins/ldp/register.go` | `resolveDiscoveryInterface` | `Resolve` |
| `internal/plugins/tftpserver/register.go` | `bindDeviceFor` | `Resolve` |
| `internal/plugins/diag/cmd/capture_interface_linux.go` | `HandleCaptureInterface` | `Resolve` |
| `internal/plugins/isis/transport/backend_linux.go` | `resolveInterface` | `Resolve` |
| `internal/plugins/ospf/transport/backend_linux.go` | package var `resolveIfaceBinding` | `Resolve` |
| `internal/plugins/ospf/v3/transport/backend_linux.go` | package var `resolveIfaceBinding` | `Resolve` |
| `internal/plugins/traffic/netlink/backend_linux.go` | `resolveOSName` | `Resolve`, falling back to the logical name on failure |

The same gate run also flags `flowexport/sampling`, `vrrp/transport` and
`iface/ra`; their plugins are already guarded elsewhere, so they need an
allowance the gate can verify, not a second guard.

## Data Flow

### Entry Point

The operator's config names a plugin `external`, or the plugin is otherwise
forked. Its runner starts in a separate process.

### Transformation Path

1. The plugin receives a logical interface name from its config or command.
2. It calls `iface.Resolve` or `iface.ResolveDevice`.
3. The call reads the forked process's own `globalResolver`, which nothing
   filled.
4. The answer is the logical name, or an error, and the plugin binds,
   queries or configures that.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | The selector table does not cross it: it is Go state in the daemon's process | No |

### Integration Points

The plugin runner's startup (`IsInternal` on the SDK plugin) and the plugin
boundary gate (`./le plugin boundary check`).

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Design Work Owed

1. For each site, find the plugin runner that reaches it and whether that
   runner can be configured external.
2. Choose per plugin: refuse external start, or resolve across the boundary.
   Refusing is the existing precedent; a cross-boundary path needs its own
   design and review, and must not leave a fallback to the logical name.
3. Decide how the gate tells a guarded plugin from an unguarded one, so the
   three already-guarded callers above pass without a hand-written exemption
   list that drifts.
4. Walk the other readers of `globalResolver` (`ResolveIndex`,
   `ResolveVPPIndex`, `Addresses`, `Subscribe`) for the same shape, add them to
   `dangerousCalls`, and fold any new site into this spec.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Each affected plugin started through its registered runner as external | → | The runner's external-start guard, or its cross-boundary resolution | One test per plugin, named at design |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The plugin owning `canonicalRequest` (bfd) starts external | It refuses to start, or resolves through the boundary; a test proves it and goes red with the guard removed. |
| AC-2 | The ldp plugin starts external | Same. |
| AC-3 | The tftpserver plugin starts external | Same. |
| AC-4 | The plugin owning `HandleCaptureInterface` (diag) starts external | Same. |
| AC-5 | The isis plugin starts external | Same. |
| AC-6 | The ospf plugin's OSPFv2 transport runs in an external process | Same. |
| AC-7 | The ospf plugin's OSPFv3 transport runs in an external process | Same. |
| AC-8 | The traffic plugin starts external | Same, and the logical-name fallback in `resolveOSName` never answers for a selector entry. |
| AC-9 | Each refusal | Names the plugin, why it must run in-process, and the `internal <name>` config that fixes it (`ai/rules/user-facing-errors.md`). |
| AC-10 | `dangerousCalls` lists `Resolve` and `ResolveDevice` | `./le plugin boundary check` is green over the tree, and red when any one guard above is removed. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| One external-start test per affected plugin, named at design | the plugin's package | AC-1 to AC-9 | |
| A boundary gate test over a fixture caller of `Resolve` | `internal/le/plugin/boundary/` | AC-10 | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Named at design, where a plugin can be configured `external` in a `.ci` | `test/plugin/` | The operator configures the plugin external and gets the refusal with its fix | |

## Files to Modify

- The runner (`register.go`) of each plugin that reaches a site in the table
  above, or the site itself for a cross-boundary resolution.
- `internal/le/plugin/boundary/pluginboundary.go`: `dangerousCalls`.
- `docs/architecture/iface/logical-name-resolution.md` and each affected
  plugin's guide page.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | No config is added |
| CLI commands/flags | No | No command is added |
| Functional test for new RPC/API | No | No RPC is added |
| Doctor check for runtime dependencies | No | No new runtime dependency |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`: which plugins refuse external start |
| 12 | Internal architecture changed? | Yes | `docs/architecture/iface/logical-name-resolution.md` |
| 16 | Changed source files referenced by doc source anchors? | Derived at design | `./le spec citation anchors spec plan/release/fixes/spec-iface-resolution-across-the-plugin-boundary.md` |

## Implementation Steps

1. **Phase: Wiring** -- write each external-start test red against the
   unguarded runner.
2. **Phase: Guards** -- add the guard or the cross-boundary resolution per
   plugin; each test goes green.
3. **Phase: Gate** -- extend `dangerousCalls` in the same change as the last
   guard; prove it red with one guard removed.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Only the iface plugin's in-process apply fills `globalResolver` | `setMapping` callers in `internal/component/iface/resolve.go` | A forked plugin could have a filled table and the class is narrower | gopls call hierarchy on `setMapping` | unvalidated |

### Risks

| ID | Risk | Mitigation / fallback |
|----|------|----------------------|
| R-1 | An operator runs a routing protocol external today | The refusal message names the fix; the change is documented on the plugin's page. |
| R-2 | The gate extension lands before the guards | It lands in the same change as the last guard, so the gate never goes red for this class. |
| R-3 | A site already runs only in the daemon's process | Prove it from the runner, record it, and let the gate accept it by that proof. |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `process-global-read-across-the-plugin-boundary.md` | 2026-10-29 | `iface.Resolve` and `ResolveDevice` callers in forked plugins read an empty selector table |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] The scoped package tests of each changed plugin pass under `./le test unit`, and `./le plugin boundary check` is green

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-iface-resolution-across-the-plugin-boundary.md` only, in the same `./le commit create` script
