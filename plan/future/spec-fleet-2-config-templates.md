# Spec: fleet-2 -- Config Templates and Group Assignment

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-fleet-1-device-registry |
| Phase | - |
| Updated | 2026-05-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/architecture/fleet-config.md` -- existing fleet config architecture
4. `internal/component/plugin/server/managed.go` -- ManagedConfigService.HandleConfigFetch
5. `spec-fleet-0-umbrella.md` -- umbrella design decisions
6. `spec-fleet-1-device-registry.md` -- device registry (dependency)

## Task

Add config templates with variable substitution and group-based assignment. Instead of
maintaining N separate config blobs (one per managed device), operators define named
templates with variables (e.g., `{{.DeviceName}}`, `{{.Labels.site}}`). Groups map to
templates. When a device in a group does a config-fetch, the hub renders the template
with that device's variables and returns the result.

Currently each client's config is a separate opaque blob in the hub's ZeFS. Managing
100 devices means maintaining 100 individual files. This spec eliminates that duplication.

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
| Change propagation | Template edit triggers config-changed for all devices using that template |
| Single-writer interaction (fleet-6) | A template edit changes the rendered config of every device in the group, including ones currently offline -- which is a hub-side change to a disconnected device's config. To respect the single-writer rule, a template edit propagates only to currently-connected group members; offline devices render the new template on their next connect and, if their running config differs, go through the `fleet-7` reconnect compare. Resolve this precisely when fleet-2 is designed |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` -- config fetch protocol, version hashing
  -> Constraint: version hash must reflect rendered output, not template source
- [ ] `internal/component/plugin/server/managed.go` -- HandleConfigFetch reads config via ConfigReader
  -> Decision: intercept ConfigReader to check template assignment before blob lookup

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
- [ ] `internal/component/plugin/server/registry.go` -- DeviceRegistry from fleet-1 (dependency)
  -> Decision: registry provides device labels and group membership for template variable resolution

**Behavior to preserve:**
- Devices with direct config blobs (no template) work exactly as today
- Config-fetch response format unchanged
- Version hash semantics unchanged (hash of what client receives)
- Client protocol unchanged

**Behavior to change:**
- HandleConfigFetch checks group assignment before blob lookup
- If group has template: render template with device variables, return rendered config
- Template or group config change triggers config-changed for affected devices
- YANG schema extended with template and group-to-template mapping

## Data Flow (MANDATORY)

### Entry Point
- Operator configures `fleet { template edge-default { source "...config with {{.DeviceName}}..."; } }`
- Operator configures `fleet { group region-west { template edge-default; } }`
- Device in group region-west sends config-fetch

### Transformation Path
1. Device sends config-fetch with current version hash
2. HandleConfigFetch looks up device in registry (from fleet-1)
3. If device has group assignment, look up group's template
4. If template found: render with device variables (name, labels)
5. If no template (direct blob): read blob as before
6. Hash rendered/read config, compare with client version
7. Return "current" or full rendered config

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to Template Engine | YANG config parsed, templates stored | [ ] |
| Template Engine to HandleConfigFetch | ConfigReader replaced with template-aware reader | [ ] |
| Template change to Notification | Iterate group members, send config-changed | [ ] |

### Integration Points
- `ManagedConfigService.HandleConfigFetch` -- intercept with template-aware ConfigReader
- `DeviceRegistry` from fleet-1 -- provides group membership and label values
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
| AC-4 | Group "region-west" with template "edge-default" | All devices in group get rendered template on config-fetch |
| AC-5 | Device not in any group, has direct config blob | Blob returned as before (no template rendering) |
| AC-6 | Device in group but also has direct config blob | Direct blob takes precedence over template |
| AC-7 | Template source edited (config commit on hub) | config-changed sent to all connected devices in groups using that template |
| AC-8 | Template with undefined variable (label not set on device) | Render fails gracefully; device gets error in config-ack; hub logs warning |
| AC-9 | `show fleet templates` CLI | Lists all templates with usage count (how many groups/devices) |
| AC-10 | `show fleet groups` CLI | Lists all groups with member count and assigned template |
| AC-11 | Version hash of template-rendered config | Hash computed from rendered output, not template source; two devices with same labels get same hash |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTemplateRender` | `internal/component/plugin/server/template_test.go` | Basic variable substitution | |
| `TestTemplateRenderLabels` | `internal/component/plugin/server/template_test.go` | Label variable substitution | |
| `TestTemplateRenderUndefinedVar` | `internal/component/plugin/server/template_test.go` | Graceful failure on missing variable | |
| `TestTemplateAwareConfigReader` | `internal/component/plugin/server/template_test.go` | ConfigReader returns rendered config for group member | |
| `TestTemplateAwareConfigReaderDirectBlob` | `internal/component/plugin/server/template_test.go` | ConfigReader returns direct blob for non-group device | |
| `TestTemplateChangeNotification` | `internal/component/plugin/server/template_test.go` | Template edit triggers notification for group members | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-template-fetch` | `test/managed/fleet-template-fetch.ci` | Device in group gets rendered config; direct blob device gets blob | |
| `fleet-template-change` | `test/managed/fleet-template-change.ci` | Template edit triggers config-changed, device re-fetches | |

## Files to Modify
- `internal/component/plugin/server/managed.go` -- template-aware ConfigReader
- `internal/component/plugin/server/registry.go` -- group membership queries
- `internal/component/plugin/yang/ze-plugin-conf.yang` -- template and group YANG containers

## Files to Create
- `internal/component/plugin/server/template.go` -- template storage, rendering, change notification
- `internal/component/plugin/server/template_test.go` -- unit tests
- `test/managed/fleet-template-fetch.ci` -- functional test
- `test/managed/fleet-template-change.ci` -- functional test

## Implementation Steps

### Implementation Phases

1. **Phase: Template engine** -- template parsing, variable resolution, rendering
   - Tests: `TestTemplateRender`, `TestTemplateRenderLabels`, `TestTemplateRenderUndefinedVar`
   - Files: `template.go`, `template_test.go`
   - Verify: unit tests pass

2. **Phase: Template-aware ConfigReader** -- intercept HandleConfigFetch with template lookup
   - Tests: `TestTemplateAwareConfigReader`, `TestTemplateAwareConfigReaderDirectBlob`
   - Files: `managed.go`, `template.go`
   - Verify: group member gets rendered config, non-group device gets blob

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
