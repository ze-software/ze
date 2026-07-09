# Spec: rib-arch-5 -- RFC 9069 BMP Loc-RIB Monitoring (PeerType=3)

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
4. `internal/component/bgp/plugins/bmp/` - BMP plugin (bmp.go, msg.go, header.go)
5. `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeBatch`
6. `rfc/short/rfc9069.md`

## Task

The BMP plugin (`internal/component/bgp/plugins/bmp/`: `bmp.go`, `msg.go`, `header.go`,
`cmd_show.go`) exports BGP Monitoring Protocol peer data.

GAP: RFC 9069 **Loc-RIB monitoring** (PeerType = 3) is not implemented. It requires
emitting BMP Route Monitoring messages built from the local RIB's best paths -- i.e. from
`BestChangeBatch` (`internal/component/bgp/plugins/rib/events/events.go:90`) -- with a
Loc-RIB Peer Up and PeerType=3 peer headers. A BMP Route Monitoring message embeds a full
BGP UPDATE PDU, so this needs the **UPDATE wire bytes reconstructed from the structured
best-change data** (the RIB best-change carries typed attributes/NLRI, not a ready-made
UPDATE PDU).

## Required Reading

### Architecture Docs
- [ ] `internal/component/bgp/plugins/bmp/msg.go` - existing BMP message construction
  → Constraint: reuse the existing header/message builders; add a PeerType=3 path, not a parallel encoder.
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeBatch` source data
  → Constraint: Loc-RIB RM messages are built from best-change deltas; preserve replay vs incremental.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9069.md` - BMP for Local RIB
  → Constraint: PeerType=3, Loc-RIB Instance Peer, Peer Up/Down and Route Monitoring semantics.

**Key insights:**
- The hard part is reconstructing UPDATE wire bytes from structured best-change data; the BMP framing already exists for per-peer monitoring.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; anchors verified 2026-07-08)
- [ ] `internal/component/bgp/plugins/bmp/bmp.go`, `msg.go`, `header.go` - existing BMP export (per-peer monitoring, headers, message framing)
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeBatch` (:90): typed per-(protocol,family) best changes; `IsReplay()` (:114) distinguishes replay from incremental

**Behavior to preserve:**
- Existing per-peer BMP monitoring and its wire framing; the `BestChangeBatch` contract.

**Behavior to change:**
- Add PeerType=3 Loc-RIB monitoring: Peer Up for the Loc-RIB instance and Route Monitoring messages built from best-change data.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- RIB best-change events (`BestChangeBatch`) plus BMP session lifecycle

### Transformation Path
1. RIB emits `BestChangeBatch` best paths (`events.go:90`)
2. BMP Loc-RIB consumer subscribes and, per change, reconstructs a BGP UPDATE PDU from the typed attributes/NLRI
3. The UPDATE is wrapped in a BMP Route Monitoring message with a PeerType=3 peer header (`msg.go`, `header.go`)
4. Sent to the configured BMP station over the existing BMP transport

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RIB → BMP | `BestChangeBatch` subscription | [ ] |
| structured → wire | reconstruct UPDATE PDU bytes from typed best-change data | [ ] |
| BMP → station | Route Monitoring with PeerType=3 header | [ ] |

### Integration Points
- `BestChangeBatch` (`events.go:90`) - the Loc-RIB data source
- BMP message/header builders (`msg.go`, `header.go`) - extended with the PeerType=3 path

### Architectural Verification
- [ ] No bypassed layers (Loc-RIB data reaches BMP via the event bus, not a RIB back-door)
- [ ] No unintended coupling (BMP consumes the public best-change event, not RIB internals)
- [ ] No duplicated functionality (reuse the UPDATE encoder; do not hand-roll a second one)
- [ ] Registration over hardcoding - BMP consumer registers as an event subscriber (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An UPDATE encoder can rebuild wire bytes from typed best-change attributes/NLRI | forward path encodes UPDATEs from typed data | Must build a best-change→UPDATE encoder | grep for the UPDATE encoder at design | unvalidated |
| A-2 | `BestChangeBatch` carries enough attribute detail for a faithful RM UPDATE | events.go entry fields | RM messages lose attributes; need richer event | inspect `BestChangeEntry` fields at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Reconstructed UPDATE differs from the on-wire form a peer would have sent | station-side decode mismatches | encode from the same typed attributes the forward path uses; interop-test against a BMP collector |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Best-path change with BMP Loc-RIB configured | → | PeerType=3 Route Monitoring message emitted | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | BMP Loc-RIB monitoring enabled, a prefix becomes best | station receives a PeerType=3 Route Monitoring message with a valid UPDATE PDU |
| AC-2 | Full-table replay batch | Loc-RIB Peer Up followed by Route Monitoring for each best path |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | `internal/component/bgp/plugins/bmp/msg_test.go` | PeerType=3 header + UPDATE PDU reconstruction from best-change data | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bmp-locrib` (new) | `test/plugin/bmp-locrib.ci` | operator enables Loc-RIB BMP; a route change reaches the BMP station as PeerType=3 | |

### Interop Tests (MANDATORY for protocol features)
- Design decides an interop scenario against a real BMP collector (e.g. OpenBMP/pmacct) to validate the reconstructed UPDATE decodes.

## Files to Modify

- `internal/component/bgp/plugins/bmp/msg.go` - PeerType=3 Route Monitoring construction
- `internal/component/bgp/plugins/bmp/header.go` - Loc-RIB peer header
- `internal/component/bgp/plugins/bmp/bmp.go` - subscribe to best-change; Loc-RIB Peer Up/Down lifecycle

## Implementation Steps

1. **Phase: design** - confirm the best-change→UPDATE encoder (A-1/A-2); define the Loc-RIB lifecycle.
2. **Phase: wiring** - failing test asserting a PeerType=3 RM message on a best-path change.
3. **Phase: implement (TDD)** - reconstruct UPDATE bytes; build PeerType=3 RM; wire lifecycle.
4. **Functional + interop** - `.ci` fixture; collector interop if warranted.
5. **RFC comments** - `// RFC 9069 Section X.Y` on enforcing code.
6. **Full verification** - `make ze-verify`.
7. **Complete spec** - audit, learned summary, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] PeerType=3 Route Monitoring emitted from best-change data
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## RFC Documentation

Add `// RFC 9069 Section X.Y` comments above the PeerType=3 header, Peer Up/Down, and
Route Monitoring construction code.

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `spec-rib-arch-0-umbrella.md`.
