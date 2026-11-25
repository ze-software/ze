# Spec: fleet-2 -- Config Templates and Group Assignment

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| Depends | plan/release/features/spec-fleet-1-device-registry.md |
| Phase | 2/10 of plan/post-release/features/spec-fleet-0-umbrella.md |
| Updated | 2026-11-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/architecture/fleet-config.md` -- existing fleet config architecture
4. `internal/component/plugin/server/managed.go` -- ManagedConfigService.HandleConfigFetch
5. `spec-fleet-0-umbrella.md` -- umbrella design decisions
6. `plan/release/features/spec-fleet-1-device-registry.md` -- device registry (dependency)

## Task

Add config templates with variable substitution and group-based assignment. Instead of
maintaining N separate config blobs (one per managed device), operators define named
templates with variables (e.g., `{{.DeviceName}}`, `{{.Labels.site}}`). Groups map to
templates. When a device in a group does a config-fetch, the hub renders the template
with that device's variables and returns the result.

Currently each client's config is a separate opaque blob in the hub's ZeFS. Managing
100 devices means maintaining 100 individual files. This spec eliminates that duplication. It also owns the dispatcher-backed `DeviceConfigPart` and `TemplatePart` defined in fleet-ui-2: the UI reads the same effective configuration bytes and template usage that the hub serves.

The client protocol is unchanged. The client receives a fully rendered config, never
sees templates. Template rendering happens inside HandleConfigFetch on the hub side.

### Key Design Decisions (from umbrella)

| Decision | Detail |
|----------|--------|
| Templates rendered hub-side | Client protocol unchanged; client receives final config |
| Template syntax | Go `text/template` with restricted function set (no shell, no file access) |
| Variable sources | Device name, device labels, group name. No external data sources |
| Assignment model | Group -> template mapping in YANG config. A device's group determines its template |
| Per-device override | Device can still have a direct config blob; overrides template assignment |
| Change propagation | A hub commit changes the desired configuration of declared clients that use the template, including offline clients. The hub notifies connected clients whose effective configuration changed. An offline client fetches it on connect. |
| Protection of a local edit | A device that edited its own configuration since the last hub baseline holds that edit on reconnect. The operator can inspect and resolve the difference through the existing adopt or revert commands. A clean device can apply its newly rendered configuration on the first fetch. See the 2026-11-19 ruling in `spec-fleet-0-umbrella.md`. |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` -- config fetch protocol, version hashing
  -> Constraint: version hash must reflect rendered output, not template source
- [ ] `internal/component/plugin/server/managed.go` -- HandleConfigFetch reads config via ConfigReader
  -> Decision: use the direct configuration when found. The storage reader in `startManagedServer` returns a wrapped `fs.ErrNotExist` for an absent blob. Normalize that miss to the `ConfigReader` contract (`ErrClientConfigNotFound`) before template lookup. Other read errors reach the caller.

**Key insights:**
- ConfigReader is a `func(name string) ([]byte, error)`; template rendering can be injected here
- Version hash computed from rendered bytes ensures client sees "current" only when rendered output matches
- Template change notification requires iterating devices in the affected group

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed.go` -- HandleConfigFetch calls `s.readConfig(clientName)` then hashes
  -> Constraint: readConfig interface must accommodate template rendering transparently
- [ ] `pkg/fleet/envelope.go` -- ConfigFetchResponse with Version, Config (base64), Status
  -> Constraint: response format unchanged
- [ ] `internal/component/plugin/server/managed_registry.go` -- fleet-1's `ManagedRegistrySnapshot` lists declared clients and their labels. Fleet-2 owns their group assignments.

**Behavior to preserve:**
- Devices with direct config blobs (no template) work exactly as today
- Config-fetch response format unchanged
- Version hash semantics unchanged (hash of what client receives)
- Client protocol unchanged

**Behavior to change:**
- Fetch and change notification read the same effective configuration. A direct blob takes precedence even for a group member.
- When the direct blob is absent, render the group's assigned template with this device's variables.
- Template or group config change triggers config-changed for affected devices
- YANG schema extended with template and group-to-template mapping

## Existing Mechanisms

| Mechanism | Source | Use in this design |
|-----------|--------|--------------------|
| Desired configuration reader | `internal/component/plugin/server/managed.go`, `ConfigReader`, `HandleConfigFetch`, `BuildConfigChanged` | Both managed messages hash the bytes supplied by the same reader. Extend that reader to resolve a group template only when the direct configuration is absent. |
| Change delivery | `internal/component/plugin/server/managed_serve.go`, `NotifyConfigChanged` | Schedule an existing push for each connected client whose effective configuration changed. Reconnection already fetches the current desired bytes. |
| Declared clients and labels | `internal/component/plugin/server/managed_registry.go`, `ManagedRegistrySnapshot` | Read the client's labels from the registry and resolve group and template through fleet-2's configuration. |
| Local edit protection | `internal/component/managed/client.go`, `holdFetched` | The device keeps a local change when offered a rendered configuration just as it does for a direct blob. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A command is declared once and reaches every interface." | agrees | The backend owns the command and structured result across command surfaces. The web UI consumes its result with the same user rights. |
| `docs/architecture/fleet-config.md` | Config Storage (Hub Side) | the // Design: page of internal/component/plugin/server/managed.go | "The hub may write the configuration of a declared client at any time, and a client that is away fetches it when it connects." | agrees | A declared offline client retains its desired configuration. Delivery and acknowledgement each require device contact. |
| `docs/architecture/api/commands.md` | Transport Request Metadata | anchored to internal/component/plugin/server/managed.go | "Caller identity is injected only by trusted transport wiring." | agrees | Commands use the authenticated dispatcher identity for authorization. Group and device names identify the configuration the operator requested. |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering internal/component/plugin/server/managed.go | "The SDK stays generic." | agrees | This feature extends the hub-side managed service. Its commands and state need no feature-specific SDK callback. |
| `docs/architecture/hub-architecture.md` | What We're Building | anchored to cmd/ze/hub/managed_server.go | "Each plugin defines its own configuration schema using YANG" | agrees | Configuration settings use the owning schema and the ordinary configuration transaction. |
| `ai/doors/hub-engine.md` | Boundaries | the door covering cmd/ze/hub/managed_server.go | "SIGHUP and a managed push both reach `doReload`" | agrees | Devices apply a managed change through the existing reload route. |
| `docs/architecture/config/syntax.md` | Overview | anchored to internal/component/plugin/yang/ze-plugin-conf.yang | "The parser is YANG-driven: each config node's type (leaf, leaf-list, container, list) determines how it is parsed." | agrees | Declare named templates and groups as keyed lists. No new configuration parser is needed. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering test/managed/fleet-template-fetch.ci | "Functional `.ci` files live under `test/<suite>/`" | agrees | Drive managed devices through test/managed and the functional test harness. |
| `docs/architecture/fleet-config.md` | Design decisions | the // Design: page of internal/component/plugin/server/managed.go | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | The managed listener keeps its configured certificate and reload semantics. The feature uses its already authenticated sessions. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/plugin/server/managed.go | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | The two list commands name templates and groups. Display filtering and formatting use pipes. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/plugin/server/managed.go | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | The listing answers structured data and inherits the generic data and formatting operations. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to internal/component/plugin/server/managed.go | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | A pipe transforms the returned data. Changing an assignment requires an authorized configuration commit. |

## Data Flow (MANDATORY)

### Entry Point
- Operator configures `fleet { template edge-default { source "...config with {{.DeviceName}}..."; } }`
- Operator configures `fleet { group region-west { template edge-default; } }`
- Device in group region-west sends config-fetch

### Transformation Path
1. Device sends config-fetch with current version hash
2. Read the device's direct configuration. On success use those bytes. On a storage failure return the error.
3. When the direct configuration is absent, look up the device's group and assigned template. If neither source provides a configuration, return the existing missing-configuration error.
4. Render the template with the device name, group and labels. A missing variable fails this fetch.
5. Pass the selected bytes to the existing version comparison and managed delivery path. Its held-version and discard-version checks also apply to template clients.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to Template Engine | YANG config parsed, templates stored | [ ] |
| Template Engine to HandleConfigFetch | ConfigReader replaced with template-aware reader | [ ] |
| Template change to Notification | Iterate group members, send config-changed | [ ] |

### Integration Points
- `ManagedConfigService.HandleConfigFetch` -- intercept with template-aware ConfigReader
- `DeviceRegistry` from fleet-1 supplies label values. Fleet-2 supplies group membership.
- EventBus -- `fleet.template.changed` event triggers config-changed notifications
- YANG schema -- `fleet { template <name> { ... }; group <name> { template <ref>; } }`

### Architectural Verification
- [ ] No bypassed layers (template rendering inside HandleConfigFetch path)
- [ ] No unintended coupling (devices without templates unaffected)
- [ ] No duplicated functionality (reuses ConfigReader interface)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with template and group assignment | -> | Template stored, group linked | `test/managed/fleet-template-fetch.ci` |
| Device in group sends config-fetch | -> | Template rendered with device vars | `test/managed/fleet-template-fetch.ci` |
| Template source edited | -> | config-changed sent to group members | `test/managed/fleet-template-change.ci` |
| Device with direct blob (no group) | -> | Blob returned as before | `test/managed/fleet-template-fetch.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | YANG config with `fleet { template edge-default { source "..."; } }` | Template parsed and stored |
| AC-2 | Template with `{{.DeviceName}}` and device "edge-01" fetches config | Config contains literal "edge-01" at substitution point |
| AC-3 | Template with `{{.Labels.site}}` and device label site=london | Config contains "london" at substitution point |
| AC-4 | Group "region-west" with template "edge-default" | Every member without a direct configuration receives its rendered template through config-fetch |
| AC-5 | Device not in any group, has direct config blob | Blob returned as before (no template rendering) |
| AC-6 | Device in group but also has direct config blob | Direct blob takes precedence over template |
| AC-7 | The hub commits a template edit with connected and offline group members, including one with a direct override | Connected members whose effective bytes changed receive `config-changed`. The offline member receives the new version on reconnect, subject to its existing local-edit hold. The override still serves its own bytes and receives no template-change notification. |
| AC-8 | Template with undefined variable (label not set on device) | The hub returns an error on config-fetch identifying the missing variable and records the rendering failure. The client keeps its active configuration. |
| AC-9 | `show fleet templates` CLI | Lists all templates with usage count (how many groups/devices) |
| AC-10 | `show fleet groups` CLI | Lists all groups with member count and assigned template |
| AC-11 | Two devices produce identical rendered bytes, or a template includes a device-specific value | Identical output yields the same version. A device name substitution that changes the bytes changes the version. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTemplateRender` | `internal/component/plugin/server/template_test.go` | Basic variable substitution | |
| `TestTemplateRenderLabels` | `internal/component/plugin/server/template_test.go` | Label variable substitution | |
| `TestTemplateRenderUndefinedVar` | `internal/component/plugin/server/template_test.go` | Graceful failure on missing variable | |
| `TestTemplateAwareConfigReader` | `internal/component/plugin/server/template_test.go` | ConfigReader returns rendered config for group member | |
| `TestTemplateAwareConfigReaderDirectBlob` | `internal/component/plugin/server/template_test.go` | A direct blob wins with and without a group assignment. A read error propagates even if a template is available. | |
| `TestTemplateChangeNotification` | `internal/component/plugin/server/template_test.go` | Template edit triggers notification for group members | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-template-fetch` | `test/managed/fleet-template-fetch.ci` | Device in group gets rendered config; direct blob device gets blob | |
| `fleet-template-change` | `test/managed/fleet-template-change.ci` | Template edit triggers config-changed, device re-fetches | |

## Files to Modify
- `internal/component/plugin/server/managed.go` -- template-aware ConfigReader
- `cmd/ze/hub/managed_server.go` -- install the effective configuration reader in the managed service
- `internal/component/plugin/yang/ze-plugin-conf.yang` -- template and group YANG containers
- `docs/architecture/fleet-config.md` -- template selection, effective versions and the direct override

## Files to Create
- `internal/component/plugin/server/template.go` -- template storage, rendering, change notification
- `internal/component/plugin/server/template_test.go` -- unit tests
- `test/managed/fleet-template-fetch.ci` -- functional test
- `test/managed/fleet-template-change.ci` -- functional test

## Design presentation

### What changes

An operator can assign a template to a group, use each device's name and labels in its configuration, and override a single device by writing its direct configuration. Fetch and change notifications hash the effective bytes each device will receive.

### Before and after

Today the operator maintains a separate configuration for every edge router. With a group template, a change to the shared uplink setting updates every member that uses it. An offline member fetches the change when it returns. A router with its own explicit configuration continues to receive that configuration. A router with an emergency local edit shows a hold for the operator to resolve.

### Alternatives set aside

Rendering on the device would require each client to understand the hub's template language. Copying the rendered configuration into each device's direct blob would turn those copies into permanent overrides. Rendering when the effective configuration is read preserves a single group assignment and a deliberate per-device override.

### Documents departed from

None. The existing hub configuration reader still supplies the bytes for both messages.

### Choices for Thomas

The existing design specifies one group per device. The implementation must enforce that assignment and report conflicting configuration at validation, so two templates never compete to supply the same device's configuration.

## Implementation Steps

### Implementation Phases

Before the rendering phases, exercise the real config-fetch with a group template and a direct override in `test/managed/fleet-template-fetch.ci`. Also prove that the change notification and the following fetch name the same effective version.

1. **Phase: Template engine** -- template parsing, variable resolution, rendering
   - Tests: `TestTemplateRender`, `TestTemplateRenderLabels`, `TestTemplateRenderUndefinedVar`
   - Files: `template.go`, `template_test.go`
   - Verify: unit tests pass

2. **Phase: Template-aware ConfigReader** -- intercept HandleConfigFetch with template lookup
   - Tests: `TestTemplateAwareConfigReader`, `TestTemplateAwareConfigReaderDirectBlob`
   - Files: `managed.go`, `template.go`
   - Verify: a member without a direct configuration gets rendered bytes, and a direct configuration wins for grouped and ungrouped devices

3. **Phase: YANG + CLI** -- template and group configuration, `show fleet templates/groups`
   - Tests: `fleet-template-fetch.ci`
   - Files: YANG schema, CLI commands
   - Verify: templates configurable via CLI editor

4. **Phase: Change propagation** -- template edit triggers config-changed
   - Tests: `TestTemplateChangeNotification`, `fleet-template-change.ci`
   - Files: `template.go`, `managed.go`
   - Verify: template edit causes connected devices to re-fetch

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-N have implementation |
| Correctness | Version hash computed from rendered output, not template source |
| Naming | YANG template/group uses kebab-case |
| Data flow | Config-fetch -> group lookup -> template render -> response |
| Rule: exact-or-reject | Missing template variable rejects, does not silently substitute empty |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Template injection | Restricted function set in text/template (no shell, file, exec) |
| Input validation | Template source bounded (max 64KB); variable values from validated labels |
| Resource exhaustion | Template rendering timeout (prevent infinite loops in malformed templates) |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
