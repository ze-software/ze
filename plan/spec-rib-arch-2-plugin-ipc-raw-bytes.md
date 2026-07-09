# Spec: rib-arch-2 -- Raw-Bytes Filter IPC (replace JSON string FilterUpdateInput.Update)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-rib-arch-0-umbrella.md` - set context
4. `pkg/plugin/rpc/types.go` - `FilterUpdateInput` / `FilterUpdateOutput`
5. `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md`

## Task

The runtime filter-update IPC (`ze-plugin-callback:filter-update`) carries route
attributes and NLRI as a **text-format JSON string** field `FilterUpdateInput.Update`
(`pkg/plugin/rpc/types.go:182`, "Text-format attributes and NLRI"); the modify response
returns a delta string `FilterUpdateOutput.Update` (`:189`). Every filter invocation
pays text (de)serialisation on the hot path.

GAP: replace the JSON text `Update` field with a **length-prefixed raw-bytes** encoding
so the engine passes wire attribute/NLRI bytes to the filter without a text round-trip.
Note a partial raw path already exists: `FilterUpdateInput.Raw` (`:183`, "Hex-encoded
raw UPDATE body") and `FilterUpdateOutput.Raw` (`:190`) are opt-in via a filter's
`raw=true` declaration, but they are hex strings inside JSON, not length-prefixed
binary. This item is about making binary the primary path, not a hex-in-JSON opt-in.

CONSTRAINT: this changes the external plugin SDK contract. Per `ai/rules/compatibility.md`
Ze carries no compat burden, but any external SDK consumer of this RPC must be updated in
lockstep, and the change touches the process-protocol framing -- design must confirm the
transport can carry length-prefixed binary alongside the line-oriented JSON envelope.

## Required Reading

### Architecture Docs
- [ ] `pkg/plugin/rpc/types.go` - the IPC struct definitions
  → Constraint: verify current field semantics (`Update` text vs `Raw` hex) before designing.
- [ ] `ai/rules/plugin-design.md` - plugin SDK / protocol contract
  → Constraint: an SDK-visible field change follows the plugin-design contract rules.
- [ ] `docs/architecture/api/process-protocol.md` - the IPC framing
  → Constraint: confirm the framing can carry length-prefixed binary, not only line-oriented JSON.

**Key insights:**
- A hex `Raw` opt-in already exists; the win is binary as the default attribute/NLRI carrier, removing text encode/decode from the filter hot path.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; anchors verified 2026-07-08)
- [ ] `pkg/plugin/rpc/types.go` - `FilterUpdateInput` (:177) with `Update string json:"update"` (:182, text) and `Raw string json:"raw,omitempty"` (:183, hex, opt-in); `FilterUpdateOutput` (:187) with `Update` (:189) and `Raw` (:190) modify fields

**Behavior to preserve:**
- The filter decision semantics (accept/reject/modify), teardown request, and per-direction (import/export) behaviour are unchanged; only the attribute/NLRI carrier encoding changes.

**Behavior to change:**
- `FilterUpdateInput.Update` / `FilterUpdateOutput.Update` become length-prefixed raw bytes instead of a JSON text string (design decides whether to remove or repurpose the hex `Raw` opt-in).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Engine invokes a runtime filter for a received/forwarded UPDATE; encodes `FilterUpdateInput`

### Transformation Path
1. Engine serialises attributes/NLRI into the IPC input (text today, `types.go:182`)
2. IPC transport delivers the input to the filter (in-process or forked plugin)
3. Filter decodes, decides accept/reject/modify, returns `FilterUpdateOutput`
4. Proposed: steps 1 and 3 use length-prefixed binary instead of a JSON text string

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| engine → filter plugin | IPC input encoding (JSON text `Update` today; length-prefixed binary proposed) | [ ] |
| filter → engine | IPC output encoding (JSON text `Update` delta today; binary proposed) | [ ] |

### Integration Points
- `FilterUpdateInput` / `FilterUpdateOutput` (`types.go:177`, `:187`) - the IPC contract
- The engine-side encode and plugin-side decode sites (design enumerates them)

### Architectural Verification
- [ ] No bypassed layers (encoding change stays at the IPC boundary)
- [ ] No unintended coupling (filter logic unaware of the carrier encoding)
- [ ] No duplicated functionality (replace the text path, do not keep both -- `ai/rules/no-layering.md`)
- [ ] Registration over hardcoding - filters register; no per-filter field added to a core struct (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The IPC framing can carry length-prefixed binary payloads | `Raw` already ships hex UPDATE bodies | Need a framing change too; larger scope | read `docs/architecture/api/process-protocol.md` at design | unvalidated |
| A-2 | No external SDK consumer depends on the text `Update` format beyond Ze's own plugins | `ai/rules/compatibility.md` (no released users) | Coordinate the SDK change | grep for `filter-update` consumers at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Binary encoding regresses forked-plugin debuggability (hex/text is human-readable) | plugin authors report opaque payloads | keep a decode helper / `ze` debug command for the binary form |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Engine invokes a runtime filter on an UPDATE | → | length-prefixed binary attributes/NLRI delivered and decoded | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Filter invoked on a received UPDATE | attributes/NLRI delivered as length-prefixed binary, decoded identically to today's text path |
| AC-2 | Filter returns modify | binary delta applied, wire output byte-identical to the text-path result |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | `pkg/plugin/rpc/types_test.go` | round-trip binary encode/decode of filter input/output | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A at skeleton stage - internal engine↔plugin IPC encoding; existing filter `.ci` suites regression-guard filter behaviour. Add a `.ci` at design only if user-visible behaviour changes | per design | filters still accept/reject/modify correctly | |

## Files to Modify

- `pkg/plugin/rpc/types.go` - `FilterUpdateInput` / `FilterUpdateOutput` carrier encoding
- engine-side filter IPC encode sites and plugin-side decode sites (enumerated at design)

## Implementation Steps

1. **Phase: design** - enumerate encode/decode sites; confirm framing (A-1); define the length-prefixed layout.
2. **Phase: wiring** - failing round-trip test for the binary carrier.
3. **Phase: implement (TDD)** - binary encode/decode; delete the text path (`ai/rules/no-layering.md`); update in-tree filter plugins in lockstep.
4. **Full verification** - `make ze-verify`.
5. **Complete spec** - audit, learned summary, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] Binary carrier implemented; text `Update` path removed
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `spec-rib-arch-0-umbrella.md`.
