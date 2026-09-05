# Spec: show-bgp-one-name-for-the-peer-address

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Depends | spec-cli-show-bgp-answer-shapes |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The problem.** One concept, the peer address, carries two names across the
`show bgp` tree. A row that IS a peer names its address `address` in `show bgp`,
`show bgp peer statistics` and `show bgp rs peers`, and names it `peer` in
`show bgp peer capabilities` and `show bgp health`. An operator who learns one
name has to learn the other, and a pipe operator written against one answer is
refused by its sibling.

**Where each name is written.** All five are verified at the producer:

| Command | Field name | Producer |
|---------|-----------|----------|
| `show bgp` | `address` | `internal/component/bgp/plugins/cmd/peer/peer.go`, the column order at `:152` and `RegisterAddressFields` at `:237` |
| `show bgp peer statistics` | `address` | `internal/component/bgp/plugins/cmd/peer/summary.go`, the row built at `:515` and the declaration at `:68` |
| `show bgp rs peers` | `address` | `(*routeServer).peerStatus`, `internal/component/bgp/plugins/rs/server_handlers.go` |
| `show bgp peer capabilities` | `peer` | `internal/component/bgp/plugins/cmd/peer/summary.go`, the row built at `:465` and the declaration at `:62` |
| `show bgp health` | `peer` | `internal/component/bgp/plugins/cmd/peer/health.go`, the row built at `:69` and the declaration at `:52` |

Both names are constants in one file, `fieldAddress` and `fieldPeer`
(`internal/component/bgp/plugins/cmd/peer/fields.go`), whose own comment records
the split and points at `summary.go` for the reason.

**Why it is a defect rather than a taste.** Synonym rotation is habit 1 of the
Simplified Technical English guideline this repository publishes in
(`ai/INSTRUCTIONS.md`): one concept, one name, repeated. The cost is not
cosmetic. `RegisterAddressFields` names the field that `| resolve` and
`| origin` decorate, so the field name is part of the command's published
contract: an operator chain written for `show bgp health` does not carry over to
`show bgp peer statistics`, and a script that reads one answer cannot read the
other by the same key.

**Why it was deferred and not fixed in place.** `spec-cli-show-bgp-answer-shapes`
(2026-08-24) found it while declaring what each command answers. Fixing it there
would have widened that spec's diff without making a single operator chain work
that did not work already, because the declaration channel names whatever field
each handler writes and both spellings declare correctly.

**What the work is.** Pick one name for the peer address across the whole
`show bgp` tree, change the handlers that write the other, and carry the change
through every reader that the rename can reach: the column orders, the
`RegisterAddressFields` declarations, the `.ci` fixtures that assert on the key,
the web and looking-glass consumers of these payloads, and the documentation
pages that show the answers. The spec must also decide what the change owes an
operator whose script reads the old key, because the payload key is a published
answer and renaming it is a wire-visible change to every reader of that command.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - the published answer contract a field name is part of
  → Decision: [fill during research]
  → Constraint: [fill during research]
- [ ] `ai/rules/cli.md` - what a command owes its structured payload
  → Constraint: [fill during research]

**Key insights:** (minimal context to resume after compaction)
- Both names are constants in one file, so the rename starts at `fields.go` and reaches every writer of the constant it retires.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/plugins/cmd/peer/fields.go` - declares `fieldAddress = "address"` and `fieldPeer = "peer"`, and its comment records which command uses which.
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - `registerPeerRowShapes` declares `fieldPeer` for `show bgp peer capabilities` and `fieldAddress` for `show bgp peer statistics`, and states that the inconsistency costs the declaration channel nothing.
- [ ] `internal/component/bgp/plugins/cmd/peer/health.go` - writes `fieldPeer` into each health row and declares it as the address field.
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` - `(*routeServer).peerStatus` writes the address under `tokenAddress` for `show bgp rs peers`.

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every command keeps its shape, its column order and its row count.
- `| resolve` and `| origin` keep decorating the peer address of each answer.

**Behavior to change:** (only what the user asked for)
- One name for the peer address in all five answers.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator types `show bgp`, `show bgp health`, `show bgp peer capabilities`, `show bgp peer statistics` or `show bgp rs peers` at the CLI, over SSH or through the API.
- The command arrives as a command string; the answer leaves as a structured map.

### Transformation Path
1. Dispatch through the command registry to the owning handler.
2. The handler builds one row for each peer, keyed by field constants.
3. `ApplyPipes` renders or narrows the answer using the declared shape, column order and address fields.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ plugin | the row map crosses as JSON | No |
| Handler ↔ pipe operators | `RegisterAddressFields` names the key `resolve` and `origin` act on | No |

### Integration Points
- `command.RegisterColumns` and `command.RegisterAddressFields` (`internal/component/command/`) - every declaration that names the retired key.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No consumer outside the tree depends on the retired key | ze is pre-release with no shipped version | a rename breaks an operator's script with no migration | asking the owner | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A reader of the renamed key is missed and silently answers nothing | a `.ci` fixture or a web panel goes empty | grep every use of the retired constant and every literal spelling of the key |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a `show bgp` answer loses its address field for one reader |
| How is it reverted? | single commit revert |
| Who else touches this path? | `plan/immediate/spec-cli-show-bgp-answer-shapes.md`, `plan/spec-plugin-declares-answer-shape.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show bgp health \| resolve <field>` typed with the chosen name | → | the health handler's row builder | `TestShowBGPHealthNamesThePeerAddressOnce` |
| every `show bgp` answer that carries a peer address | → | the declared address fields | `TestShowBGPTreeUsesOneNameForThePeerAddress` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | any of the five commands is run | the peer address is under one name in every answer |
| AC-2 | an operator chain naming that field is typed against any of the five | it is accepted by all five |
| AC-3 | the retired name is grepped for in the tree | no producer writes it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShowBGPTreeUsesOneNameForThePeerAddress` | `internal/component/bgp/plugins/cmd/peer/fields_test.go` | AC-1, AC-3 | | <!-- doc-links: ignore (file this skeleton plans and has not created yet) -->

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-bgp-peer-address-name` | `test/plugin/show-bgp-peer-address-name.ci` | one operator chain works against all five commands | | <!-- doc-links: ignore (file this skeleton plans and has not created yet) -->

## Files to Modify
- `internal/component/bgp/plugins/cmd/peer/fields.go` - one constant for the peer address
- `internal/component/bgp/plugins/cmd/peer/summary.go` - the two declarations and the two row builders
- `internal/component/bgp/plugins/cmd/peer/health.go` - the health row and its declaration
- `internal/component/bgp/plugins/cmd/peer/peer.go` - the `show bgp` column order and declaration
- `internal/component/bgp/plugins/rs/server_handlers.go` - the route-server peer row

## Files to Create
- [named at design]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| CLI grammar (keyword before value) | | [answered at design] |
| Pipe completeness | | [answered at design] |
| Functional test for new RPC/API | | [answered at design] |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- write the failing test that asserts one name across the five answers
   - Tests: `TestShowBGPTreeUsesOneNameForThePeerAddress`
   - Files: `internal/component/bgp/plugins/cmd/peer/fields_test.go`
   - Verify: the test fails because two names are in use
2. **Phase: [named at design]**

## Known Limitations
- [filled at design]

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Current Behavior and Data Flow sections completed

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
