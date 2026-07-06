# Spec: followup-vpp-iface

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

> **BLOCKED / decision needed:** Every item needs `make vendor-pull` of a new go.fd.io/govpp binapi package. Per go-standards.md, no new third-party imports without explicit user approval. Get the vendoring decision before design.

## Task

VPP interface tunnel/mirror/LCP/wireguard features. Each is hard-blocked on vendoring a govpp binapi package that is NOT currently in `vendor/`.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **CreateTunnel vxlan (L185)** - `binapi/vxlan` absent; `VxlanAddDelTunnelV3` unwired.
- **CreateTunnel gre/ipip (L186)** - `binapi/gre` + `/ipip` absent; `GreTunnelAddDel`/`IpipAddTunnel` unwired.
- **LCP TAP pair (L188)** - `binapi/lcp` absent; Linux TAP shadow of VPP iface for BGP TCP bind unimplemented.
- **Mirror/SPAN (L189)** - `binapi/span` absent; `SpanEnableDisableL2` unwired.
- **Wireguard-via-VPP (L190)** - `binapi/wireguard` absent + requires the wireguard VPP plugin at runtime.

## Required Reading

### Source files / docs

- [ ] `internal/plugins/iface/vpp/ifacevpp.go:389,401,668` (errNotSupported stubs)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `vendor/go.fd.io/govpp/binapi/` (missing: vxlan, gre, ipip, lcp, span, wireguard)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `ai/rules/go-standards.md` (dependency/vendoring rule)
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; line numbers are pre-triage references)

- [ ] `internal/plugins/iface/vpp/ifacevpp.go`

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces named in the Task work items.

**Behavior to change:**
- Only the specific gaps enumerated in the Task work items.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `iface` config selecting a VPP tunnel/mirror/wireguard netdev under the vpp backend

### Transformation Path
1. Config requests a VPP tunnel/mirror/LCP/wireguard interface
2. `vppBackendImpl` currently returns `errNotSupported`
3. Once the binapi is vendored, the method programs VPP

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config -> ifacevpp | backend dispatch | [ ] |
| ifacevpp -> VPP | govpp vxlan/gre/ipip/lcp/span/wireguard binary API (to vendor) | [ ] |

### Integration Points
- `internal/plugins/iface/vpp/ifacevpp.go`
- `vendor/go.fd.io/govpp/binapi/` (new packages)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The verified `file:line` evidence in the Task items still holds at design time | 2026-07-06 backlog triage | Re-scope the item | grep/LSP at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope drift when the umbrella is split into per-item specs | Item needs its own design doc | Split into a dedicated spec and re-point |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config creates a vxlan tunnel under vpp | → | `ifacevpp.CreateTunnel` programs VPP | (fill during design) |
| Config enables SPAN mirror under vpp | → | `ifacevpp.SetupMirror` programs VPP | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (define per work item when this skeleton moves to `design`) | (define at design time) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | (define at design time) | per Task work item | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| vpp tunnel/mirror/wireguard (new) (`.ci`) | test/vpp | VPP netdev lifecycle once binapi is vendored | |

## Files to Modify

- `internal/plugins/iface/vpp/ifacevpp.go` - see Task work items

## Implementation Steps

1. **Phase: split** - if the umbrella covers unrelated items, split into per-item specs first.
2. **Phase: design** - for the chosen item, re-verify the `file:line` evidence and fill the Data Flow / Wiring / AC sections above.
3. **Phase: wiring** - register entry points, write the failing wiring test.
4. **Phase: implement (TDD)** - write test, fail, implement, pass, per work item.
5. **Full verification** - `make ze-verify`.
6. **Complete spec** - fill audit tables, write `plan/learned/NNN-<name>.md`, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] Every chosen work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (see `ai/rules/deferral-tracking.md`). Moves to `design` when someone picks it up.
