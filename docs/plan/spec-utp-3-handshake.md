# Spec: Text Handshake Protocol

## Task

Design and implement a text-format alternative for the 5-stage JSON-RPC plugin handshake. The text handshake must use the same unified grammar as event delivery and text commands, enabling a single parser to handle all plugin IPC.

Parent spec: `spec-utp-0-umbrella.md`.
Depends on: `spec-utp-1-event-format.md` and `spec-utp-2-command-format.md` (shared tokenizer exists).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/text-format.md` — unified grammar reference
  → Decision:
- [ ] `docs/architecture/api/process-protocol.md` — current 5-stage handshake (source of truth)
  → Constraint:
- [ ] `docs/architecture/api/ipc_protocol.md` — IPC framing
  → Constraint:
- [ ] `docs/architecture/api/capability-contract.md` — capability contract
  → Constraint:

### Source Files
- [ ] `pkg/plugin/rpc/types.go` — JSON-RPC type definitions (all stage input/output types)
- [ ] `pkg/plugin/sdk/` — SDK startup implementation (plugin side of handshake)
- [ ] `internal/plugin/process.go` — engine side of handshake stages
- [ ] `internal/plugin/subsystem.go` — plugin lifecycle management
- [ ] `pkg/plugin/rpc/conn.go` — RPC connection (NUL-framed JSON)
- [ ] `pkg/plugin/rpc/mux.go` — MuxConn for concurrent RPCs

### RFC Summaries (not protocol work — N/A)

## Current Behavior (MANDATORY)

### 5-Stage JSON-RPC Handshake

| Stage | Direction | RPC Method | Key Data |
|-------|-----------|-----------|----------|
| 1 | Plugin→Engine (A) | `ze-plugin-engine:declare-registration` | families, commands, dependencies, config roots, YANG schema |
| 2 | Engine→Plugin (B) | `ze-plugin-callback:configure` | config sections (root + JSON blob per section) |
| 3 | Plugin→Engine (A) | `ze-plugin-engine:declare-capabilities` | BGP capabilities (code, encoding, payload, peer filter) |
| 4 | Engine→Plugin (B) | `ze-plugin-callback:share-registry` | all registered commands across plugins |
| 5 | Plugin→Engine (A) | `ze-plugin-engine:ready` | optional event subscriptions |

### Framing
- NUL-terminated JSON-RPC 2.0 frames
- Two socket pairs (A = plugin-initiated, B = engine-initiated)
- After stage 5: Socket A gets MuxConn (concurrent RPCs), Socket B enters event loop

### Complexity Assessment

| Stage | Text-Friendly? | Challenge |
|-------|---------------|-----------|
| 1 (Registration) | Medium | Nested structures (families with modes, commands with args) |
| 2 (Config) | Hard | Config is arbitrary JSON — can't flatten to key-value |
| 3 (Capabilities) | Easy | Simple list of code/encoding/payload tuples |
| 4 (Registry) | Easy | Flat command list |
| 5 (Ready) | Easy | Optional subscription params |

### Design Decisions Needed

| Question | Options | Constraint |
|----------|---------|------------|
| Config delivery in text? | (a) Embed JSON blob as quoted value, (b) key-value flattening, (c) keep JSON-RPC for config only | Config structure is arbitrary (YANG-modeled), can't predict keys |
| Framing? | (a) Newline-delimited, (b) NUL-delimited, (c) length-prefixed | Must support multi-line values if config is embedded |
| Bidirectional? | (a) Same text framing both directions, (b) Text A + JSON B | MuxConn needs request IDs for concurrent RPCs |
| Request/response IDs? | (a) Implicit (one-at-a-time per stage), (b) Explicit `id <N>` token | Stages are sequential, but runtime RPCs are concurrent |
| YANG schema delivery? | (a) Inline as quoted blob, (b) Reference by name (engine looks up), (c) Skip in text mode | YANG text can be large (multi-KB) |

## Data Flow (MANDATORY)

### Entry Points
- Plugin startup: `SDK.Startup()` → sequential stage calls on Socket A/B
- Engine startup: `Process.runStages()` → sequential stage handling

### Transformation Path
(to be completed during research — trace each stage's data flow)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → Engine (stages 1,3,5) | Socket A | [ ] |
| Engine → Plugin (stages 2,4) | Socket B | [ ] |
| Post-startup concurrent RPCs | MuxConn on Socket A | [ ] |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| (to be filled) | | | |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Text handshake completes all 5 stages | Plugin reaches ready state, enters event loop |
| AC-2 | Same tokenizer | Handshake parser shares tokenizer with event/command parsers |
| AC-3 | Family declarations | Expressible in unified text grammar |
| AC-4 | Config delivery | Config data delivered to plugin (format TBD) |
| AC-5 | Capability injection | Capabilities parsed from text, injected into OPEN |
| AC-6 | Registry sharing | Plugin receives command registry in text |
| AC-7 | Event subscription | `ready` with subscription works |
| AC-8 | Negotiation | Plugin and engine can negotiate text vs JSON mode |
| AC-9 | Fallback | If text handshake fails, can fall back to JSON-RPC |

## Proposed Text Handshake Format

### Stage 1: Registration

```
register family ipv4/unicast mode both family ipv6/unicast mode encode
register command rib-show description "Show RIB entries" args peer,type completable true
register dependency bgp-rib
register config-root bgp,bgp/peer
register schema module bgp-rs namespace urn:bgp-rs handler bgp:route-reflection
register cache-consumer true cache-consumer-unordered false
register wants-validate-open true
```

Or as a single-line key-value:

```
register families ipv4/unicast:both,ipv6/unicast:encode dependencies bgp-rib config-roots bgp,bgp/peer cache-consumer true
```

### Stage 2: Config

Config is the hard one. Options:

**Option A — JSON blob as quoted value:**
```
configure root bgp data "{\"asn\":65000,\"router-id\":\"1.1.1.1\"}"
configure root bgp/peer data "{\"address\":\"192.168.1.1\"}"
```

**Option B — One line per config root, data on separate channel:**
```
configure root bgp length 256
<256 bytes of JSON>
configure root bgp/peer length 128
<128 bytes of JSON>
```

**Option C — Keep JSON-RPC for config stage only:**
Mixed mode: stages 1,3,4,5 use text; stage 2 uses JSON-RPC. Pragmatic but breaks uniformity.

### Stage 3: Capabilities

```
capability code 9 encoding hex payload 0180
capability code 128 encoding text payload hostname=router1.example.com peers 192.168.1.1,10.0.0.1
```

### Stage 4: Registry

```
registry command rib-show plugin bgp-rib encoding json
registry command announce plugin bgp-plugin encoding text
```

### Stage 5: Ready

```
ready
ready subscribe events bgp:update,bgp:open encoding text peers 192.168.1.1
```

### Response Protocol

Text responses use a simple status line:

```
ok
ok result <data>
error <message>
```

### Runtime RPCs After Stage 5

```
update-route peer * command "update text origin set igp nlri ipv4/unicast add 10.0.0.0/24"
subscribe-events events bgp:update encoding text
dispatch-command command "rib show" args peer,192.168.1.1
```

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (to be filled) | | | |

### Functional Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (to be filled — plugin must complete handshake in text mode) | | | |

## Files to Modify
- `pkg/plugin/rpc/types.go` — add text serialization methods
- `pkg/plugin/rpc/conn.go` — support text framing mode
- `pkg/plugin/sdk/` — text mode startup path
- `internal/plugin/process.go` — text mode stage handling
- `internal/plugin/subsystem.go` — text mode negotiation

## Files to Create
- Shared text handshake parser (location TBD)
- Text handshake formatter (location TBD)

## Implementation Steps

1. Design config delivery approach (resolve Option A/B/C)
2. Design response protocol (request/response IDs for concurrent RPCs)
3. TDD: Write handshake round-trip tests
4. Implement text serialization for each stage's types
5. Add text framing mode to Conn
6. Add text startup path to SDK
7. Add text stage handling to Process
8. Add mode negotiation (text vs JSON)
9. Run full test suite including existing JSON handshake tests

### Failure Routing
| Failure | Route To |
|---------|----------|
| Config too complex for text | Use Option C (JSON for config only) |
| Concurrent RPCs need IDs | Add `id <N>` token prefix |
| MuxConn incompatible with text | Separate text Conn implementation |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Implementation Summary

### What Was Implemented
- (to be filled after implementation)

### Documentation Updates
- (to be filled after implementation)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] Architecture docs updated

### Quality Gates
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit > implicit

### Verification
- [ ] `make ze-lint` passes
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Spec moved to `docs/plan/done/NNN-utp-3-handshake.md`
- [ ] Spec included in commit
