# Spec: plugin-bgp-format-extraction

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/done/247-plugin-restructure.md` - predecessor spec, "Remaining Work" section
4. `internal/plugin/text.go` - BGP text/JSON formatting (848 lines, 7 BGP imports)
5. `internal/plugin/json.go` - JSONEncoder (262 lines, 2 BGP imports)
6. `internal/plugin/server_bgp.go` - BGP Server methods (566 lines, 6 BGP imports)
7. `internal/plugin/validate_open.go` - OPEN validation (181 lines, 1 BGP import)

## Task

Extract the remaining 4 BGP-specific files from `internal/plugin/` to eliminate BGP imports from the generic plugin infrastructure.

After specs 244, 246, and 247, `internal/plugin/` has 28 non-test files. 21 are fully generic (zero BGP imports). 4 are thin structural bridges (acceptable). 4 are pure BGP code that should not live in generic infrastructure:

| File | Lines | BGP Imports | Purpose |
|------|-------|-------------|---------|
| `text.go` | 848 | 7 | BGP message formatting (text + JSON UPDATE) |
| `server_bgp.go` | 566 | 6 | BGP event dispatch (Server methods) |
| `json.go` | 262 | 2 | JSONEncoder for ze-bgp JSON envelope |
| `validate_open.go` | 181 | 1 | OPEN validation broadcast (Server method) |

**Goal:** After this spec, `internal/plugin/` has zero "pure BGP" files. Only the 3 structural files (command.go, server.go, types.go) retain BGP imports via thin interface/alias bridges.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall system architecture
  → Constraint: Engine passes wire bytes to plugins; plugins implement RIB/policy
- [ ] `.claude/rules/plugin-design.md` - plugin registration and lifecycle
  → Constraint: Generic server defines callback hooks; BGP layer registers implementations

### Predecessor Spec
- [ ] `docs/plan/done/247-plugin-restructure.md` - "Remaining Work" section
  → Decision: These 4 files are the remaining BGP code in plugin/
  → Constraint: Circular import is the root blocker (text.go uses PeerInfo, server_bgp.go calls Format*)

**Key insights:**
- All 4 files share a circular import blocker: they use types from `internal/plugin/` (PeerInfo, RawMessage, ContentConfig, Server) while being called by code in `internal/plugin/`
- The callback/hook pattern from spec 247 Phase 5 design is the intended solution
- `server_bgp.go` and `validate_open.go` are Server methods — can't simply move to another package

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/text.go` - FormatMessage(), FormatOpen(), FormatNotification(), FormatKeepalive(), FormatRouteRefresh(), FormatStateChange(), FormatNegotiated(), FormatSentMessage() + all formatting helpers (JSON/text attribute formatting, NLRI formatting, filter result formatting)
- [ ] `internal/plugin/json.go` - JSONEncoder struct with methods: StateUp(), StateDown(), StateConnected(), EOR(), Notification(), Open(), Keepalive(), RouteRefresh(), Negotiated(), marshal()
- [ ] `internal/plugin/server_bgp.go` - Server methods: OnMessageReceived(), OnPeerStateChange(), OnPeerNegotiated(), OnMessageSent(), BroadcastValidateOpen(), handleDecodeNLRIRPC(), handleEncodeNLRIRPC(), handleDecodeMPReachRPC(), handleDecodeMPUnreachRPC(), handleDecodeUpdateRPC(), EncodeNLRI(), DecodeNLRI()
- [ ] `internal/plugin/validate_open.go` - OpenValidationError type, openMessageToRPC(), extractCapabilitiesFromOptParams(), broadcastValidateOpenImpl()

**Behavior to preserve:**
- All ze-bgp JSON format output (envelope, field names, nesting)
- All text format output (peer prefix, attribute formatting)
- Event subscription delivery (OnMessageReceived dispatches to matching procs)
- OPEN validation broadcast (fail-fast on first rejection)
- Codec RPCs (decode/encode NLRI, MP_REACH, MP_UNREACH, UPDATE)

**Behavior to change:**
- File locations only — no behavioral changes

## Data Flow (MANDATORY)

### Entry Point
- BGP messages arrive at reactor → reactor calls Server methods (OnMessageReceived, OnPeerStateChange, etc.)
- Plugin RPCs arrive via socket → Server dispatches to handle* methods

### Transformation Path
1. Reactor calls Server.OnMessageReceived(peer, msg)
2. Server matches subscriptions via SubscriptionManager
3. Server calls formatMessageForSubscription() → FormatMessage() or JSONEncoder methods
4. Formatted string delivered to plugin via ConnB.SendDeliverEvent()

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor → Server | Server methods (OnMessageReceived etc.) | [ ] |
| Server → Format | Direct function calls (FormatMessage, JSONEncoder) | [ ] |
| Server → Plugin | JSON string via SendDeliverEvent() | [ ] |

### Integration Points
- `server.go:175` stores `encoder *JSONEncoder` field
- `server.go:218` creates `NewJSONEncoder("6.0.0")`
- `server_bgp.go` calls FormatMessage(), FormatStateChange(), FormatNegotiated()
- Test files: `text_test.go`, `json_test.go`, `message_receiver_test.go`, `server_test.go` all call Format*

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Design

### The Circular Import Problem

```
internal/plugin/server_bgp.go  ──calls──▶  FormatMessage(PeerInfo, RawMessage, ...)
                                                          │
internal/plugin/text.go  ◀──defines──  FormatMessage()
                              │
                              └──uses──▶  PeerInfo, RawMessage, ContentConfig  (from plugin/types.go)
```

Moving text.go to `bgp/format/` creates: `plugin/ → bgp/format/` (calling Format*) AND `bgp/format/ → plugin/` (using PeerInfo) = cycle.

### Approach: Callback Registration

The generic Server defines callback interfaces. BGP code registers implementations. No direct imports needed.

**Step 1: Define callback interfaces in `internal/plugin/`**

| Callback | Signature | Currently Implemented By |
|----------|-----------|--------------------------|
| MessageFormatter | (PeerInfo, RawMessage, ContentConfig, string) → string | text.go FormatMessage() |
| StateFormatter | (PeerInfo, string, string) → string | text.go FormatStateChange() |
| SentFormatter | (PeerInfo, RawMessage, ContentConfig) → string | text.go FormatSentMessage() |
| MessageEventHandler | (Server context, PeerInfo, RawMessage) | server_bgp.go OnMessageReceived() |
| StateEventHandler | (Server context, PeerInfo, string) | server_bgp.go OnPeerStateChange() |
| NegotiatedEventHandler | (Server context, PeerInfo, any) | server_bgp.go OnPeerNegotiated() |
| SentEventHandler | (Server context, PeerInfo, RawMessage) | server_bgp.go OnMessageSent() |
| OpenValidator | (string, any, any) → error | validate_open.go BroadcastValidateOpen() |
| CodecHandler | (Server context, Process, PluginConn, Request) | server_bgp.go handle*RPC() |

**Step 2: Server stores callbacks, dispatches through them**

Server gains a `BGPHooks` field (struct of function callbacks, not interface — avoids forcing a single implementor). Generic server calls hooks when set; no-ops when nil.

**Step 3: BGP code registers hooks at server creation**

ServerConfig gains a `BGPHooks` field. The reactor/startup code that creates the Server provides the hooks, pointing to functions now living in `internal/plugins/bgp/format/` and a new `internal/plugins/bgp/server/` package.

**Step 4: Move files**

| Current | Destination | What Moves |
|---------|-------------|------------|
| `text.go` | `internal/plugins/bgp/format/text.go` | All Format* functions + helpers |
| `json.go` | `internal/plugins/bgp/format/json.go` | JSONEncoder + methods |
| `server_bgp.go` | `internal/plugins/bgp/server/events.go` | Event dispatch (OnMessage*, formatFor*) |
| `server_bgp.go` (codec RPCs) | `internal/plugins/bgp/server/codec.go` | handle*RPC methods become functions |
| `validate_open.go` | `internal/plugins/bgp/server/validate.go` | OpenValidationError + broadcast logic |

### Package Dependencies After Move

```
internal/plugin/          (generic: defines callback types, stores hooks)
    │
    └──does NOT import──▶  internal/plugins/bgp/...

internal/plugins/bgp/format/   (text.go, json.go — formatting)
    │
    ├──imports──▶  internal/plugin/  (for PeerInfo, RawMessage, ContentConfig types)
    └──imports──▶  internal/plugins/bgp/...  (attribute, filter, context, etc.)

internal/plugins/bgp/server/   (events.go, codec.go, validate.go — dispatch)
    │
    ├──imports──▶  internal/plugin/  (for Server-adjacent types, Process, PluginConn)
    ├──imports──▶  internal/plugins/bgp/format/  (for Format*, JSONEncoder)
    └──imports──▶  internal/plugins/bgp/...  (message, wireu, etc.)
```

No cycles. `internal/plugin/` imports nothing from `bgp/`. BGP packages import `plugin/` for types.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make test` after extraction | All existing tests pass, no regressions |
| AC-2 | `make functional` after extraction | All functional tests pass |
| AC-3 | `make lint` after extraction | Zero lint errors |
| AC-4 | Grep `internal/plugin/*.go` for `internal/plugins/bgp/` | Only command.go (2), server.go (1), types.go (1) have BGP imports |
| AC-5 | text.go, json.go, server_bgp.go, validate_open.go | No longer exist in `internal/plugin/` |
| AC-6 | BGP message formatting output | Identical to pre-extraction (verified by existing tests) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Existing text_test.go tests | moved to `bgp/format/text_test.go` | All Format* functions work from new location | |
| Existing json_test.go tests | moved to `bgp/format/json_test.go` | JSONEncoder works from new location | |
| Existing server_test.go BGP tests | stay or move | Server BGP event dispatch works via hooks | |
| Existing message_receiver_test.go | stays | FormatMessage integration still works via hook | |

### Boundary Tests
Not applicable — structural refactoring, no numeric fields.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing functional tests | `test/` | All BGP functionality unchanged | |

## Files to Modify
- `internal/plugin/server.go` - Add BGPHooks field, dispatch through hooks
- `internal/plugin/types.go` - Define callback function types for BGPHooks

## Files to Create
- `internal/plugins/bgp/format/text.go` - Format* functions (from plugin/text.go)
- `internal/plugins/bgp/format/json.go` - JSONEncoder (from plugin/json.go, merging with existing format/)
- `internal/plugins/bgp/format/text_test.go` - Moved tests
- `internal/plugins/bgp/format/json_test.go` - Moved tests
- `internal/plugins/bgp/server/events.go` - BGP event dispatch
- `internal/plugins/bgp/server/codec.go` - Codec RPC handlers
- `internal/plugins/bgp/server/validate.go` - OPEN validation

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| RPC count in architecture docs | No | |
| CLI commands/flags | No | |
| API commands doc | No | |
| Plugin SDK docs | No | |
| Functional test for new RPC/API | No | Existing tests cover |

## Files to Delete
- `internal/plugin/text.go` - Moved to bgp/format/
- `internal/plugin/json.go` - Moved to bgp/format/
- `internal/plugin/server_bgp.go` - Moved to bgp/server/
- `internal/plugin/validate_open.go` - Moved to bgp/server/
- `internal/plugin/text_test.go` - Moved to bgp/format/
- `internal/plugin/json_test.go` - Moved to bgp/format/

## Implementation Steps

1. **Define BGPHooks callback types in types.go**
   → Review: Are all hook signatures minimal? Do they avoid importing BGP types?

2. **Add BGPHooks to ServerConfig and Server**
   → Review: Server dispatch uses hooks when non-nil, no-ops when nil?

3. **Move text.go + json.go to bgp/format/**
   → Update package declaration, export needed symbols
   → Update imports in server_bgp.go (temporary — it moves next)
   → Review: No circular imports? All callers updated?

4. **Move server_bgp.go to bgp/server/ (events.go + codec.go)**
   → Convert Server methods to standalone functions taking Server context
   → Register as hooks in ServerConfig
   → Review: Generic server has zero BGP imports?

5. **Move validate_open.go to bgp/server/validate.go**
   → Convert broadcastValidateOpenImpl to function taking Server context
   → Register as hook
   → Review: OpenValidationError accessible to callers?

6. **Move test files**
   → text_test.go → bgp/format/text_test.go
   → json_test.go → bgp/format/json_test.go
   → Update package declarations and imports

7. **Verify all** - `make lint && make test && make functional`
   → Review: Zero BGP imports in text.go/json.go/server_bgp.go/validate_open.go (they're gone)

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Move text.go to bgp/format/ | | | |
| Move json.go to bgp/format/ | | | |
| Move server_bgp.go to bgp/server/ | | | |
| Move validate_open.go to bgp/server/ | | | |
| Define BGPHooks callback types | | | |
| Only structural BGP imports remain in plugin/ | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| text_test.go moved | | | |
| json_test.go moved | | | |
| Existing functional tests | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/types.go` | | BGPHooks types added |
| `internal/plugin/server.go` | | Hook dispatch added |
| `internal/plugins/bgp/format/text.go` | | |
| `internal/plugins/bgp/format/json.go` | | |
| `internal/plugins/bgp/server/events.go` | | |
| `internal/plugins/bgp/server/codec.go` | | |
| `internal/plugins/bgp/server/validate.go` | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass — cannot defer)
- [ ] Acceptance criteria AC-1..AC-6 all demonstrated
- [ ] Tests pass (`make test`)
- [ ] No regressions (`make functional`)
- [ ] Feature code integrated into codebase (`internal/*`)

### Quality Gates (SHOULD pass — can defer with explicit user approval)
- [ ] `make lint` passes
- [ ] Architecture docs updated with learnings
- [ ] Implementation Audit fully completed

### 🏗️ Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit behavior
- [ ] Minimal coupling
- [ ] Next-developer test

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Feature code integrated
- [ ] Functional tests verify end-user behavior

### Documentation
- [ ] Required docs read
- [ ] Architecture docs updated with new structure

### Completion
- [ ] Implementation Audit completed
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together
