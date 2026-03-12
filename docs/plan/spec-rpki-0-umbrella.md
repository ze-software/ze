# Spec: rpki-0-umbrella

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | 0/5 |
| Updated | 2026-03-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc6811.md` — origin validation algorithm, VRP structure, validation states
4. `rfc/short/rfc8210.md` — RTR v1 protocol, PDU types, session lifecycle
5. `rfc/short/rfc8097.md` — OV state extended community wire format
6. `docs/learned/339-gr-receiving-speaker.md` — GR plugin pattern (model for RPKI plugin)

## Task

Add RPKI origin validation to Ze. Routes received from BGP peers are validated against a ROA cache (populated via the RTR protocol from RPKI cache servers). Invalid routes are rejected before RIB insertion. The feature is implemented as a `bgp-rpki` plugin that acts as a gatekeeper — when loaded, it coordinates with `bgp-adj-rib-in` to hold routes pending validation; when not loaded, routes flow directly into the RIB unchanged.

**Scope:** ROA-based origin validation (RFC 6811). ASPA validation is deferred to a future spec. RTR protocol v1 (RFC 8210) for cache server communication.

## Spec Set

This is an umbrella spec for a set of child specs:

| Spec | Scope | Status |
|------|-------|--------|
| `spec-rpki-0-umbrella.md` | Architecture, plugin interactions, design decisions | This file |
| `spec-rpki-1-validation-gate.md` | New RIB coordination primitive: pending/accept/reject routes | Planned |
| `spec-rpki-2-rtr-client.md` | RTR protocol client (RFC 8210), ROA cache management | Planned |
| `spec-rpki-3-origin-validation.md` | ROA validation logic (RFC 6811), per-prefix validation | Planned |
| `spec-rpki-4-config-yang.md` | YANG schema, config pipeline, CLI commands | Planned |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — reactor event loop, plugin dispatch
  → Constraint: Plugins receive events via parallel delivery; no plugin can block another's event
  → Decision: Coordination must use inter-plugin commands (DispatchCommand), not event interception
- [ ] `docs/architecture/api/process-protocol.md` — 5-stage plugin startup, TopologicalTiers
  → Constraint: Dependencies determine startup order (tier 0 first) and event delivery order (for state/EOR)
  → Decision: bgp-rpki declares Dependencies on bgp-adj-rib-in
- [ ] `docs/learned/339-gr-receiving-speaker.md` — GR plugin coordination pattern
  → Decision: retain-routes/release-routes via DispatchCommand is the established inter-plugin coordination pattern
  → Constraint: sortByReverseDependencyTier delivers state/EOR events to dependents first

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc8210.md` — RTR protocol v1 (cache-to-router communication)
  → Constraint: 11 PDU types, version negotiation, timing parameters in End-of-Data
  → Constraint: TCP transport, Session ID + Serial Number for incremental updates
- [ ] `rfc/short/rfc6811.md` — BGP Prefix Origin Validation
  → Constraint: Three validation states: Valid, Invalid, NotFound
  → Constraint: Origin AS derived from rightmost AS in final AS_SEQUENCE segment
  → Constraint: AS_SET/AS_CONFED_SET yields "NONE" origin — can never match any VRP
  → Constraint: Multiple covering VRPs — Valid if ANY matches; Invalid only if NONE match
  → Constraint: MUST re-validate when ROA cache changes
- [ ] `rfc/short/rfc8097.md` — OV State Extended Community
  → Constraint: Type 0x43, Sub-Type 0x00, non-transitive opaque
  → Constraint: Values: 0=Valid, 1=NotFound, 2=Invalid
  → Constraint: MUST NOT accept from EBGP peers by default
  → Constraint: Multiple instances: keep numerically greatest
- [ ] `rfc/short/rfc6482.md` — ROA Profile (informational — ROA structure)
  → Constraint: maxLength absent means maxLength = prefix length (exact match only)
- [ ] `rfc/short/rfc6810.md` — RTR v0 (superseded by 8210, but v0 compat may be needed)
  → Constraint: End-of-Data is 12 bytes in v0, 24 bytes in v1 (timing params added)
- [ ] `rfc/short/rfc9319.md` — maxLength guidance (operational, no protocol changes)
  → Decision: No implementation impact — purely operator guidance for ROA issuance

**Key insights:**
- RPKI validation is a local operation — validity state is NOT a BGP attribute, it is locally computed
- The RTR protocol is simple: TCP connection, Reset/Serial Query, receive prefix records, End-of-Data
- Origin AS extraction from AS_PATH has edge cases (AS_SET, empty, confed) that must be handled
- Cache changes trigger re-validation of all affected routes — not just new ones
- Extended community (RFC 8097) is for iBGP propagation of locally-computed state, not for eBGP

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/bgp-adj-rib-in/` — Adj-RIB-In plugin, subscribes to "update direction received", stores raw routes
- [ ] `internal/component/bgp/plugins/bgp-rib/rib.go` — RIB plugin, subscribes to "update direction sent", stores Adj-RIB-Out
- [ ] `internal/component/bgp/plugins/bgp-rib/rib_commands.go` — retain-routes/release-routes command handlers
- [ ] `internal/component/bgp/plugins/bgp-gr/gr.go` — GR plugin: model for inter-plugin coordination
- [ ] `internal/component/bgp/plugins/bgp-gr/register.go` — Dependencies declaration pattern
- [ ] `internal/component/bgp/server/events.go` — event delivery, sortByReverseDependencyTier
- [ ] `internal/component/plugin/server/subscribe.go` — SubscriptionManager, GetMatching
- [ ] `pkg/plugin/sdk/sdk_engine.go` — DispatchCommand, UpdateRoute, SubscribeEvents
- [ ] `internal/component/bgp/attribute/aspath.go` — AS_PATH parsing, origin AS extraction

**Behavior to preserve:**
- All existing event delivery (parallel for UPDATEs, sequential for state/EOR)
- bgp-adj-rib-in route storage when bgp-rpki is NOT loaded (zero overhead)
- bgp-rib Adj-RIB-Out tracking unchanged
- retain-routes/release-routes GR coordination pattern unchanged
- All existing plugin subscriptions and dependencies

**Behavior to change:**
- When bgp-rpki is loaded: routes held in "pending" state in bgp-adj-rib-in until validated
- Invalid routes rejected (not stored in Adj-RIB-In, not forwarded to peers)
- Valid/NotFound routes promoted from pending to installed
- ROA cache changes trigger re-validation of stored routes

## Architecture

### Component Overview

The RPKI feature adds one new plugin and extends one existing plugin:

| Component | Role |
|-----------|------|
| `bgp-rpki` (new plugin) | RTR client, ROA cache, validation logic, issues accept/reject commands |
| `bgp-adj-rib-in` (extended) | Adds "pending" route state, accept-routes/reject-routes command handlers |

### Plugin Interaction Pattern: Validation Gate

The validation gate is a new coordination primitive, modeled after the GR retain/release pattern but operating at the route level rather than the peer level.

**When bgp-rpki is NOT loaded:**
- bgp-adj-rib-in receives UPDATEs, stores routes immediately (current behavior, zero overhead)
- No pending state, no validation delay

**When bgp-rpki IS loaded:**
- bgp-rpki registers `rib enable-validation` command at startup (stage 5)
- bgp-adj-rib-in checks for validation-enabled flag before storing routes
- If enabled: routes stored as "pending", message ID recorded
- bgp-rpki validates each prefix against ROA cache
- bgp-rpki sends `rib accept-routes` or `rib reject-routes` per prefix
- bgp-adj-rib-in promotes or discards pending routes

### Event Flow

**Normal UPDATE flow (rpki loaded):**

| Step | Actor | Action |
|------|-------|--------|
| 1 | Engine | Delivers UPDATE event to all subscribers (parallel, unchanged) |
| 2 | bgp-adj-rib-in | Receives UPDATE, stores routes as "pending" |
| 3 | bgp-rpki | Receives same UPDATE, extracts origin AS from AS_PATH |
| 4 | bgp-rpki | For each NLRI prefix: looks up ROA cache, computes validity |
| 5a | bgp-rpki | Valid/NotFound: `DispatchCommand("rib accept-routes <peer> <msg-id> <prefix>")` |
| 5b | bgp-rpki | Invalid: `DispatchCommand("rib reject-routes <peer> <msg-id> <prefix>")` |
| 6 | bgp-adj-rib-in | Promotes accepted routes to installed, discards rejected routes |

**ROA cache update flow:**

| Step | Actor | Action |
|------|-------|--------|
| 1 | bgp-rpki | RTR session receives new/changed ROA records |
| 2 | bgp-rpki | Identifies affected prefixes (prefixes covered by changed ROAs) |
| 3 | bgp-rpki | `DispatchCommand("rib revalidate <prefix>")` for affected prefixes |
| 4 | bgp-adj-rib-in | Re-exports stored routes for the affected prefix |
| 5 | bgp-rpki | Validates again with updated cache, issues accept/reject |

**Safety valve:** If bgp-rpki does not respond within a configurable timeout (default: 30s), pending routes are accepted (fail-open). This prevents indefinite route black-holing if the RPKI plugin crashes.

### Dependency Graph

| Plugin | Dependencies | Startup Tier |
|--------|-------------|-------------|
| bgp-adj-rib-in | (none) | 0 |
| bgp-rpki | bgp-adj-rib-in | 1 |
| bgp-rib | (none) | 0 |
| bgp-rs | bgp-adj-rib-in | 1 |
| bgp-gr | bgp-rib | 1 |

bgp-rpki depends on bgp-adj-rib-in so that:
1. bgp-adj-rib-in starts first, registers its accept/reject command handlers
2. bgp-rpki can call `rib enable-validation` during its own startup
3. For state events, bgp-rpki processes before bgp-adj-rib-in (reverse dependency tier)

### Validation State Storage

Validation state per route is stored in bgp-adj-rib-in as a field on the route entry:

| Field | Type | Values |
|-------|------|--------|
| validationState | uint8 | 0=NotValidated, 1=Valid, 2=NotFound, 3=Invalid |

This field is:
- Set by bgp-rpki via accept-routes (sets Valid/NotFound) or reject-routes (sets Invalid)
- Queryable via CLI: `ze bgp rib in <peer> --rpki-state`
- Included in JSON event output when routes are forwarded to bgp-rs
- Encoded as RFC 8097 extended community when configured for iBGP propagation

### RTR Client Architecture

The RTR client runs inside the bgp-rpki plugin as a long-lived goroutine:

| Component | Responsibility |
|-----------|---------------|
| RTR session manager | TCP connections to cache servers, reconnect logic |
| PDU parser | RFC 8210 wire format parsing (all 11 PDU types) |
| ROA cache | In-memory prefix-to-ASN table, supports incremental updates |
| Validation engine | RFC 6811 algorithm, origin AS extraction, VRP matching |

Multiple cache servers supported for redundancy. Tables merged (union of all servers). If all cache servers disconnect, existing cache retained for a configurable expire time (default: 3600s per RFC 8210 Section 6).

### Config Structure (YANG)

RPKI config augments `ze-bgp-conf`:

| Path | Type | Description |
|------|------|-------------|
| `rpki/cache-server[address]` | list | RTR cache server connections |
| `rpki/cache-server/address` | string | Cache server IP/hostname |
| `rpki/cache-server/port` | uint16 | TCP port (default: 323) |
| `rpki/cache-server/preference` | uint8 | Server preference (lower = preferred) |
| `rpki/validation-timeout` | uint16 | Seconds before fail-open on pending routes (default: 30) |
| `rpki/policy/invalid-action` | enum | reject (default), log-only, accept |
| `rpki/policy/not-found-action` | enum | accept (default), reject, log-only |

### CLI Commands

| Command | Description |
|---------|-------------|
| `ze bgp rpki cache` | Show RTR cache server status |
| `ze bgp rpki roa` | Show ROA table |
| `ze bgp rpki summary` | Show validation statistics |
| `ze bgp rib in <peer> --rpki-state` | Show routes with validation state |

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- RTR: TCP connection to cache server, receives ROA prefix records (VRPs)
- BGP: Received UPDATE from peer, delivered as event to bgp-rpki plugin

### Transformation Path
1. **RTR PDU parsing** — TCP bytes parsed into ROA records (prefix, max-length, origin-AS)
2. **ROA cache storage** — Records stored in prefix-indexed table, supports incremental updates
3. **UPDATE event received** — bgp-rpki extracts origin AS from AS_PATH (rightmost in final AS_SEQUENCE)
4. **VRP lookup** — For each NLRI prefix, find covering VRPs in ROA cache
5. **Validation** — Apply RFC 6811 algorithm: NotFound (no covering VRP), Valid (matching VRP), Invalid (covering but no match)
6. **Route decision** — Issue accept-routes or reject-routes command to bgp-adj-rib-in

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RTR cache server ↔ bgp-rpki | TCP, RFC 8210 PDU format | [ ] |
| Engine ↔ bgp-rpki | JSON events (UPDATE delivery) | [ ] |
| bgp-rpki ↔ bgp-adj-rib-in | DispatchCommand (accept/reject/revalidate) | [ ] |
| bgp-adj-rib-in ↔ bgp-rs | Route state includes validation field | [ ] |

### Integration Points
- `bgp-adj-rib-in`: extended with pending state, accept/reject commands, validation field
- `bgp-rs`: reads validation state when forwarding routes (for RFC 8097 ext community)
- `events.go`: no changes needed — parallel UPDATE delivery is correct
- `register.go`: new plugin registration with Dependencies

### Architectural Verification
- [ ] No bypassed layers — bgp-rpki uses standard event delivery + DispatchCommand
- [ ] No unintended coupling — bgp-adj-rib-in checks for validation-enabled flag, not bgp-rpki directly
- [ ] No duplicated functionality — ROA cache is new; validation logic is new
- [ ] Zero-copy preserved — UPDATE events delivered as-is, bgp-rpki reads but doesn't copy wire bytes

## Design Decisions

### D1: Validation gate in bgp-adj-rib-in, not bgp-rib

bgp-adj-rib-in is the entry point for received routes. Validating here means invalid routes never enter the RIB at all. bgp-rib tracks Adj-RIB-Out (sent routes) and is the wrong layer.

### D2: Fail-open on timeout

If bgp-rpki doesn't respond within the configured timeout, pending routes are accepted. This is the safe default — a failed RPKI plugin should not black-hole all routes. Operators who want fail-closed can configure `invalid-action: reject` and accept the risk.

### D3: Per-prefix validation, not per-UPDATE

A single UPDATE can contain multiple NLRI prefixes. Different prefixes may have different ROA coverage. Validation must be per-prefix. The accept/reject commands include the prefix to identify which route to promote or discard.

### D4: RTR v1 only (initially)

RFC 8210 (v1) is the current standard. v0 (RFC 6810) support can be added later if needed. v1 includes timing parameters and version negotiation.

### D5: ASPA deferred

ASPA validation (draft-ietf-sidrops-aspa-verification) is a separate concern with its own RTR PDU type and validation algorithm. It will be a separate child spec if/when needed.

### D6: Extended community (RFC 8097) deferred to spec-rpki-3

Encoding validation state into extended communities for iBGP propagation is orthogonal to the core validation gate. It depends on the validation state being stored (spec-rpki-1) and the ROA cache existing (spec-rpki-2).

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config loads rpki cache-server | -> | bgp-rpki plugin starts, RTR session established | `TestRPKIPluginStartup` |
| Received UPDATE with rpki loaded | -> | Route held as pending, validated, accepted/rejected | `TestValidationGateAcceptReject` |
| Received UPDATE without rpki loaded | -> | Route stored immediately (no pending state) | `TestNoRPKIPluginPassthrough` |
| ROA cache update | -> | Affected routes re-validated | `TestROACacheChangeRevalidation` |
| RTR cache server reconnect | -> | Session re-established, incremental update | `TestRTRSessionReconnect` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | UPDATE received, bgp-rpki loaded, origin AS matches ROA | Route accepted into Adj-RIB-In with state=Valid |
| AC-2 | UPDATE received, bgp-rpki loaded, origin AS does not match any ROA but prefix covered | Route rejected (not stored), state=Invalid |
| AC-3 | UPDATE received, bgp-rpki loaded, no ROA covers prefix | Route accepted with state=NotFound |
| AC-4 | UPDATE received, bgp-rpki NOT loaded | Route stored immediately, no validation delay |
| AC-5 | ROA cache updated, existing route now Invalid | Route removed from Adj-RIB-In |
| AC-6 | ROA cache updated, existing route now Valid (was NotFound) | Route state updated to Valid |
| AC-7 | All RTR cache servers disconnect | Existing cache retained until expire timeout |
| AC-8 | bgp-rpki validation timeout exceeded | Pending routes accepted (fail-open) |
| AC-9 | AS_PATH contains AS_SET as final segment | Origin AS = "NONE", validation state = NotFound |

## Child Spec Execution Order

1. **spec-rpki-1-validation-gate.md** — The coordination primitive. Extends bgp-adj-rib-in with pending state, accept/reject/revalidate commands. No RTR, no ROA cache yet — uses mock validation for testing.
2. **spec-rpki-2-rtr-client.md** — RTR protocol client inside bgp-rpki plugin. Connects to cache servers, parses PDUs, maintains ROA cache. No validation logic yet — just cache management.
3. **spec-rpki-3-origin-validation.md** — RFC 6811 validation algorithm. Connects ROA cache to validation gate. Origin AS extraction from AS_PATH. Extended community encoding (RFC 8097).
4. **spec-rpki-4-config-yang.md** — YANG schema, config pipeline, CLI commands for RPKI.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]
