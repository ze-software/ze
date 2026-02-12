# Spec: RFC 9234 BGP Role Plugin

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9234.md` - RFC summary
4. `internal/plugin/gr/gr.go` - plugin pattern reference
5. `internal/plugin/registration.go` - plugin protocol

## Status: Phase 1 Complete (Engine-Side Validation)

## Task

Implement RFC 9234 BGP Role as a **plugin** with its own YANG schema.

**Key design decisions:**
- Role is a **plugin**, not engine code
- Plugin owns YANG schema (augments ze-bgp)
- Plugin sends Role capability via Stage 3
- Plugin receives OPEN events, validates role pairs, responds accept/reject
- Role passed once per peer in peer events (not per update)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - plugin protocol, peer events

### RFC Summaries
- [ ] `rfc/short/rfc9234.md` - Role capability, validation rules

### Code References
- [ ] `internal/plugin/gr/gr.go` - plugin implementation pattern (uses `declare wants config bgp` + JSON)
- [ ] `internal/plugin/hostname/hostname.go` - reference plugin for config parsing

**Key insights:**
- Role capability code 9, length 1, value 0-4
- Plugin validates role pairs per RFC 9234 Table 2
- Plugin controls accept/reject, engine just delivers events
- **Config pattern:** All plugins use `declare wants config bgp` + JSON parsing (not deprecated `declare conf`)

## Current Behavior

**Source files to read before implementation:**
- [ ] `internal/plugin/gr/gr.go` - Reference for plugin config pattern
- [ ] `internal/plugin/hostname/hostname.go` - Reference for JSON config parsing
- [ ] `internal/plugin/registration.go` - Plugin protocol (deprecated patterns removed)

**Behavior to preserve:**
- Plugin protocol stages (1-5) remain unchanged
- Capability hex format for Stage 3 declarations
- JSON event format for peer events

**Behavior to change:**
- New plugin (Role) to be created following current patterns

## Data Flow

### Entry Point
- Config file: per-peer `role` and `role-strict` settings under `capability { role { } }`
- Wire: peer OPEN message containing Role capability (code 9, 1 byte)

### Transformation Path
1. Config file parsed into Tree, resolved to JSON, delivered to role plugin via Stage 2
2. Role plugin extracts per-peer role configs, builds CapabilityDecl with Strict flag
3. Engine injects Role capability (code 9) into local OPEN per peer via CapabilityInjector
4. On peer OPEN received, session calls `validateRole()` after `negotiate()`
5. Engine extracts Role caps from both local and peer OPEN, calls `capability.ValidateRole()`
6. On mismatch: NOTIFICATION 2/11 sent, session closed

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → Engine | CapabilityDecl (Stage 3 RPC) with Strict flag | [x] |
| Engine OPEN handling | capability.ValidateRole() called synchronously | [x] |
| Engine → Wire | NOTIFICATION 2/11 on role mismatch | [x] |

### Integration Points
- `CapabilityInjector.AddPluginCapabilities()` — injects Role cap into OPEN
- `CapabilityInjector.IsCapabilityStrict()` — queries strict mode per peer/code
- `Session.handleOpen()` — calls `validateRole()` after `negotiate()`
- `capability.ValidateRole()` — RFC 9234 Section 4.2 logic

### Architectural Verification
- [x] No bypassed layers (validation in engine OPEN path, like AS/hold-time checks)
- [x] No unintended coupling (capability package has no plugin imports)
- [x] No duplicated functionality (plugin dead code removed after engine validation added)
- [x] Zero-copy preserved where applicable (capability bytes parsed in-place)

## Design

### Role Values (RFC 9234 Section 4.1)

| Value | Name | Config Syntax |
|-------|------|---------------|
| 0 | Provider | `role provider;` |
| 1 | RS | `role rs;` |
| 2 | RS-Client | `role rs-client;` |
| 3 | Customer | `role customer;` |
| 4 | Peer | `role peer;` |

### Local Role vs Peer Role

Two distinct roles per peer:

| Term | Meaning | Source |
|------|---------|--------|
| Local role | Our role in the relationship | Config: `role customer;` |
| Peer role | Their role in the relationship | Their OPEN capability |

**Valid pairs (RFC 9234 Table 2):**

| Local Role | Expected Peer Role |
|------------|-------------------|
| Provider | Customer |
| Customer | Provider |
| RS | RS-Client |
| RS-Client | RS |
| Peer | Peer |

### Plugin Architecture

```
internal/plugin/role/
├── role.go           # Plugin implementation
├── role_test.go      # Unit tests
└── ze-role.yang      # YANG schema (augments ze-bgp)
```

### Plugin Registration (Stage 1)

```
declare rfc 9234
declare receive open
declare receive negotiated
declare wants config bgp
declare done
```

### Plugin Config Delivery (Stage 2)

Engine delivers JSON config (like hostname/GR plugins):
```
config json bgp {"bgp":{"peer":{"10.0.0.1":{"capability":{"role":{"role":"customer"}}}}}}
config done
```

Plugin parses JSON and stores: peer 10.0.0.1 → local-role = customer

### Plugin Capability Declaration (Stage 3)

Plugin declares Role capability for each peer based on config:
```
capability hex 9 03 peer 10.0.0.1   # Role=Customer (0x03)
capability hex 9 00 peer 10.0.0.2   # Role=Provider (0x00)
capability done
```

### OPEN Event to Plugin

When peer's OPEN is received, engine sends to plugins that declared `receive open`:

```json
{
  "type": "open",
  "peer": {
    "address": "10.0.0.1",
    "asn": 65002
  },
  "capabilities": {
    "role": 0
  }
}
```

Note: `role` is numeric (wire value), plugin maps to name.

### Plugin Validation Logic

Plugin validates:
1. Lookup local-role for peer (from Stage 2 config)
2. Extract peer-role from OPEN event
3. Check valid pair per Table 2
4. Respond accept or reject

### Plugin Response

| Response | Effect |
|----------|--------|
| `peer 10.0.0.1 open accept` | Continue session |
| `peer 10.0.0.1 open reject role-mismatch` | NOTIFICATION 2/11 |

### Peer Event Format

After session established, peer events include both roles:

```json
{
  "type": "peer",
  "peer": {
    "address": "10.0.0.1",
    "asn": 65002,
    "local-role": "customer",
    "peer-role": "provider"
  },
  "state": "up"
}
```

### Peer Without Role Capability

If peer doesn't send Role capability:
- `peer-role` is null/absent in events
- Plugin decides: accept (compatible) or reject (strict mode)
- Configurable per-peer: `role-strict true;`

### Peer Selector by Role

```
peer [role customer]      # All peers where local-role=customer
peer [peer-role provider] # All peers where peer-role=provider
```

## YANG Schema

`internal/plugin/role/ze-role.yang`:

```yang
module ze-role {
    namespace "urn:ze:role";
    prefix role;

    import ze-bgp { prefix bgp; }

    description "RFC 9234 BGP Role plugin for ZeBGP";

    revision 2026-01-01 {
        description "Initial revision";
    }

    typedef role-type {
        type enumeration {
            enum provider { value 0; description "RFC 9234: Provider (0)"; }
            enum rs { value 1; description "RFC 9234: Route Server (1)"; }
            enum rs-client { value 2; description "RFC 9234: RS-Client (2)"; }
            enum customer { value 3; description "RFC 9234: Customer (3)"; }
            enum peer { value 4; description "RFC 9234: Peer (4)"; }
        }
        description "RFC 9234 BGP Role values";
    }

    augment "/bgp:bgp/bgp:peer" {
        description "Role configuration for peers";

        leaf role {
            type role-type;
            mandatory true;
            description "Our role in this peer relationship";
        }

        leaf role-strict {
            type boolean;
            default false;
            description "Require peer to send Role capability";
        }
    }
}
```

**Note:** No `peer-group` augment needed. ZeBGP uses templates + `inherit` for inheritance:

```
template {
  bgp {
    peer * {
      inherit-name customers;
      role customer;
    }
  }
}

bgp {
  peer 10.0.0.1 {
    inherit customers;   # gets role=customer
    peer-as 65001;
  }
}
```

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRoleFromConfig` | `role/role_test.go` | Config parsing stores local-role | |
| `TestRoleCapabilityBuild` | `role/role_test.go` | Stage 3 capability built correctly | |
| `TestRoleValidPairs` | `role/role_test.go` | All valid pairs accepted | |
| `TestRoleInvalidPairs` | `role/role_test.go` | Invalid pairs rejected | |
| `TestRoleMissingCapability` | `role/role_test.go` | Peer without role handled | |
| `TestRoleStrictMode` | `role/role_test.go` | Strict mode rejects missing | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Above |
|-------|-------|------------|---------------|
| Role value | 0-4 | 4 (Peer) | 5 |
| Capability length | 1 | 1 | 0, 2+ |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `role-valid-pair` | `test/data/plugin/role-valid-pair.ci` | Customer↔Provider accepted | |
| `role-invalid-pair` | `test/data/plugin/role-invalid-pair.ci` | Customer↔Customer rejected | |
| `role-peer-event` | `test/data/plugin/role-peer-event.ci` | Peer event has both roles | |
| `role-strict` | `test/data/plugin/role-strict.ci` | Strict mode rejects missing | |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/plugin/role/role.go` | Plugin implementation |
| `internal/plugin/role/role_test.go` | Unit tests |
| `internal/plugin/role/ze-role.yang` | YANG schema |
| `test/data/plugin/role-valid-pair.ci` | Functional test |
| `test/data/plugin/role-invalid-pair.ci` | Functional test |
| `test/data/plugin/role-peer-event.ci` | Functional test |
| `test/data/plugin/role-strict.ci` | Functional test |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/plugin/types.go` | Add `LocalRole`, `PeerRole` to `PeerInfo` |
| `internal/plugin/json.go` | Include roles in peer events |
| `internal/selector/selector.go` | Add `[role X]`, `[peer-role X]` filters |

## Engine Changes Required

| Change | File | Description |
|--------|------|-------------|
| OPEN event delivery | `internal/plugin/server.go` | Send OPEN to plugins declaring `receive open` |
| OPEN response handling | `internal/plugin/command.go` | Parse `peer <addr> open accept/reject` |
| Session blocking | `internal/plugin/bgp/reactor/session.go` | Wait for plugin response before proceeding |
| Role in PeerInfo | `internal/plugin/types.go` | Add LocalRole, PeerRole fields |

## Implementation Steps

1. **Create plugin skeleton**
   - `internal/plugin/role/role.go` with Stage 1-5 handling
   - Basic registration: `declare rfc 9234`, `declare receive open`

2. **Add YANG schema**
   - `internal/plugin/role/ze-role.yang`
   - Augments ze-bgp peer config

3. **Implement config handling (Stage 2)**
   - Parse `config json bgp {...}` (same pattern as GR/hostname plugins)
   - Extract role config from JSON peer tree
   - Store local-role per peer

4. **Implement capability declaration (Stage 3)**
   - Build `capability hex 9 <value> peer <addr>` for each peer

5. **Implement OPEN validation**
   - Receive OPEN event
   - Extract peer-role
   - Validate against local-role per Table 2
   - Respond accept/reject

6. **Add engine OPEN event delivery** (if not exists)
   - Check if any plugin declared `receive open`
   - Format and send OPEN event
   - Wait for response with timeout

7. **Add roles to peer events**
   - Modify PeerInfo to include LocalRole, PeerRole
   - Include in JSON peer events

8. **Add role selectors**
   - `[role X]` for local-role
   - `[peer-role X]` for peer-role

9. **Write functional tests**

## RFC Documentation

Add to plugin code:
```
// RFC 9234 Section 4.1: BGP Role Capability (Code 9, Length 1)
// RFC 9234 Section 4.2: Role pair validation per Table 2
// RFC 9234 Section 4.2: Role Mismatch Notification (2/11)
```

## Checklist

### 🏗️ Design
- [ ] Plugin architecture (not engine code)
- [ ] YANG under plugin folder
- [ ] Local-role vs peer-role distinction clear
- [ ] Backward compatible (peer without role)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] All 5 role values tested
- [ ] All valid pairs tested
- [ ] All invalid pairs tested
- [ ] Missing capability case tested

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] RFC summary read
- [ ] RFC references in code
- [ ] YANG schema documented

## Open Questions

1. **OPEN event delivery** - Does engine already send OPEN events to plugins? If not, this is a larger change.

2. **Multiple plugins receiving OPEN** - If multiple plugins declare `receive open`, how are responses aggregated? All must accept?

3. **Timeout default** - If plugin doesn't respond, accept (compatible) or reject (strict)?

## Dependencies

- Engine must support OPEN event delivery to plugins
- Engine must support `peer <addr> open accept/reject` response

If OPEN event delivery doesn't exist, this becomes a two-part spec:
- **Spec A:** Engine OPEN event + response mechanism
- **Spec B:** Role plugin using that mechanism

## Implementation Summary (Phase 1: Engine-Side Validation)

### Key Architectural Decision

The original spec proposed plugin-side async OPEN event validation (plugin receives OPEN, responds accept/reject). Investigation revealed this would require building an entirely new async plugin gate mechanism for OPEN processing.

**Decision: Engine-side validation** — Role pair validation belongs in the engine's OPEN processing path, alongside existing checks (AS mismatch, hold-time, required families). This is simpler, synchronous, and follows the existing pattern.

The role plugin's job is:
1. Parse config (Stage 2) and declare capabilities (Stage 3) — **done**
2. Engine handles validation using `capability.ValidateRole()` — **done**

### What Was Implemented

**Phase 0 (committed 6ace01d):**
- Role plugin skeleton with SDK RPC pattern
- Config parsing from JSON (`extractPeerRoleConfigs`, `extractRoleCapabilities`)
- CLI decode (`RunCLIDecode` with JSON/text output)
- YANG schema (`ze-role.yang` augmenting `ze-bgp-conf`)
- Plugin registration via `register.go` + `registry.Register()`
- 39 subtests across 8 test functions

**Phase 1 (this changeset):**
- `capability.CodeRole = 9` constant in capability package
- `capability/role.go` — RFC 9234 validation logic (`ValidateRole`, `ValidateRolePair`, `RoleName`)
- `Strict` field propagated through full injection path: `rpc.CapabilityDecl` → `PluginCapability` → `InjectedCapability` → `CapabilityInjector.IsCapabilityStrict()`
- `Session.validateRole()` wired into `handleOpen()` after `negotiate()` — sends NOTIFICATION 2/11 on failure
- `Peer.isRoleStrict()` queries `Server.IsCapabilityStrict()` for per-peer strict mode
- Role plugin propagates `Strict: cfg.strict` to engine
- Dead code removed from role plugin (`validPairs` map, `validRolePair()` function, redundant tests)
- 11 engine-side tests in `capability/role_test.go`

### Deviations from Plan

| Original Plan | Actual | Reason |
|---------------|--------|--------|
| Plugin receives OPEN events, validates, responds accept/reject | Engine validates synchronously in `handleOpen()` | No async plugin OPEN gate exists; engine-side mirrors existing OPEN checks |
| `internal/plugin/types.go` — PeerInfo role fields | Not yet implemented | Phase 2 work |
| `internal/plugin/json.go` — roles in peer events | Not yet implemented | Phase 2 work |
| `internal/selector/selector.go` — role selectors | Not yet implemented | Phase 2 work |
| Functional `.ci` tests | Not yet created | Phase 2 work |

### Open Questions (Resolved)

1. **OPEN event delivery** — Resolved: not needed. Engine validates directly.
2. **Multiple plugins receiving OPEN** — Resolved: not applicable with engine-side approach.
3. **Timeout default** — Resolved: synchronous validation, no timeout needed.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Role is a plugin | ✅ Done | `internal/plugin/role/role.go` | SDK RPC pattern |
| Plugin owns YANG schema | ✅ Done | `internal/plugin/role/schema/ze-role.yang` | Augments ze-bgp-conf |
| Plugin sends Role capability via Stage 3 | ✅ Done | `role.go:172` | Per-peer CapabilityDecl |
| Role pair validation per Table 2 | ✅ Done | `capability/role.go:49` | Engine-side, not plugin |
| Strict mode enforcement | ✅ Done | `capability/role.go:109` | Via IsCapabilityStrict path |
| NOTIFICATION 2/11 on mismatch | ✅ Done | `session.go:908` | In handleOpen() |
| Multiple different Role caps rejected | ✅ Done | `capability/role.go:96` | RFC 9234 Section 4.2 |
| PeerInfo role fields | ❌ Deferred | — | Phase 2 |
| Peer event JSON integration | ❌ Deferred | — | Phase 2 |
| Role-based selectors | ❌ Deferred | — | Phase 2 |
| Functional tests | ❌ Deferred | — | Phase 2 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Config parsing (TestRoleFromConfig equivalent) | ✅ Done | `role_test.go:16` | TestExtractRoleCapabilities_ParseBGPConfig |
| Capability build (TestRoleCapabilityBuild equivalent) | ✅ Done | `role_test.go:16` | Same test validates code/encoding/payload |
| Valid pairs | ✅ Done | `capability/role_test.go:15` | TestValidateRolePair_ValidPairs (engine-side) |
| Invalid pairs | ✅ Done | `capability/role_test.go:33` | TestValidateRolePair_InvalidPairs (engine-side) |
| Missing capability | ✅ Done | `capability/role_test.go:122` | TestValidateRole_NoPeerRole_NoStrict |
| Strict mode | ✅ Done | `capability/role_test.go:133` | TestValidateRole_NoPeerRole_Strict |
| Strict propagation | ✅ Done | `role_test.go:402` | TestExtractRoleCapabilities_StrictPropagation |
| Multiple different roles | ✅ Done | `capability/role_test.go:147` | TestValidateRole_MultipleDifferentPeerRoles |
| Multiple same roles | ✅ Done | `capability/role_test.go:164` | TestValidateRole_MultipleSamePeerRoles |
| Boundary values | ✅ Done | `capability/role_test.go:55` | TestValidateRolePair_Boundary |
| CLI decode | ✅ Done | `role_test.go:261` | TestRunCLIDecode |
| Functional tests | ❌ Deferred | — | Phase 2 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/role/role.go` | ✅ Created | Plugin implementation |
| `internal/plugin/role/role_test.go` | ✅ Created | 8 test functions, 39 subtests |
| `internal/plugin/role/schema/ze-role.yang` | ✅ Created | YANG schema |
| `internal/plugin/role/schema/embed.go` | ✅ Created | go:embed for YANG |
| `internal/plugin/role/register.go` | ✅ Created | init() registration |
| `internal/plugin/bgp/capability/role.go` | ✅ Created | Engine validation logic |
| `internal/plugin/bgp/capability/role_test.go` | ✅ Created | 11 tests |
| `internal/plugin/bgp/capability/capability.go` | ✅ Modified | CodeRole = 9 |
| `internal/plugin/bgp/reactor/session.go` | ✅ Modified | validateRole(), SetRoleStrictChecker() |
| `internal/plugin/bgp/reactor/peer.go` | ✅ Modified | isRoleStrict(), wiring |
| `internal/plugin/registration.go` | ✅ Modified | Strict field, IsCapabilityStrict() |
| `internal/plugin/server.go` | ✅ Modified | IsCapabilityStrict() delegation |
| `pkg/plugin/rpc/types.go` | ✅ Modified | Strict field on CapabilityDecl |
| `test/data/plugin/role-*.ci` | ❌ Deferred | Phase 2 |

### Audit Summary
- **Total items:** 27
- **Done:** 23
- **Partial:** 0
- **Skipped/Deferred:** 4 (PeerInfo fields, peer events, selectors, functional tests — all Phase 2)
- **Changed:** 1 (validation moved from plugin to engine — documented above)

---

## Phase 2: validate-open Callback (Engine→Plugin OPEN Validation)

### Status: Planning

### Task

Move RFC 9234 Role validation from the engine (`capability/role.go`) to the role plugin via a new `validate-open` synchronous RPC callback. The engine becomes a generic "ask plugins if this OPEN is acceptable" gateway, with no knowledge of Role-specific semantics.

**What moves OUT of engine:**
- `capability/role.go` (entire file: ValidateRole, ValidateRolePair, RoleName, role constants, Role struct, parseRole)
- `capability/role_test.go` (entire file)
- `CodeRole` constant from `capability.go`
- `case CodeRole:` from `parseCapability()` and `Code.String()`
- `Session.validateRoleWith()` from `session.go`
- `Session.roleStrictChecker` field and setter
- `Peer.isRoleStrict()` method
- `Strict` field from `PluginCapability`, `InjectedCapability`, `CapabilityDecl`
- `IsCapabilityStrict()` from `CapabilityInjector` and `Server`

**What engine GAINS (generic):**
- New RPC method: `ze-plugin-callback:validate-open`
- `Session.openValidator` callback (replaces `roleStrictChecker`)
- `Server.BroadcastValidateOpen()` (broadcasts to interested plugins, like config-verify)
- `WantsValidateOpen` flag on `DeclareRegistrationInput`

### Design Decisions

1. **validate-open is a synchronous callback** (engine→plugin via Socket B, plugin responds). Follows the exact pattern of `config-verify` / `config-apply`: engine sends, plugin responds with accept/reject.

2. **Called BEFORE negotiation** — saves work if rejected. Current flow in `handleOpen()` (session.go:862-972):
   - parse OPEN → validate version → validate hold time → store peerOpen → ~~negotiate → validateRole~~ → validate families
   - New flow: parse OPEN → validate version → validate hold time → store peerOpen → **openValidator** → negotiate → validate families

3. **All registered plugins called, fail-fast on first rejection.** `BroadcastValidateOpen` iterates processes where `WantsValidateOpen=true`, calls each via `connB.SendValidateOpen()`. First reject stops the loop.

4. **Plugin declares interest via `WantsValidateOpen` in Stage 1 registration** — auto-set by SDK when `OnValidateOpen` callback is registered.

5. **Input format:** peer address + both OPENs with capabilities as `{code, data-hex}` (just the value bytes, not TLV envelope). Engine already has the `*message.Open` structs — it converts them to a JSON-serializable format with raw capability hex. The plugin parses only the capabilities it understands (code 9 for Role).

6. **Output format:** `{accept: true}` or `{accept: false, notify-code: N, notify-subcode: N, reason: "..."}`. Using notify-code + notify-subcode keeps it generic — any OPEN validation plugin can specify the exact NOTIFICATION. Role uses code=2, subcode=11 (Role Mismatch).

7. **Strict field removed from engine types** — plugin handles strict internally. The `Strict` field on `CapabilityDecl`, `PluginCapability`, `InjectedCapability` is deleted. `IsCapabilityStrict()` is deleted from `CapabilityInjector` and `Server`.

### Data Flow

```
Session.handleOpen()
  → parse OPEN bytes (version, hold time — core RFC 4271)
  → store peerOpen
  → call s.openValidator(peerAddr, localOpen, peerOpen)
    → Peer.validateOpen(peerAddr, localOpen, peerOpen)
      → r.api.BroadcastValidateOpen(ctx, peerAddr, localOpen, peerOpen)
        → Server iterates processes where WantsValidateOpen=true
          → PluginConn.SendValidateOpen(ctx, input)
            → Plugin SDK dispatches to onValidateOpen callback
              → Role plugin: extract caps, validate pair, return accept/reject
            → Plugin SDK returns ValidateOpenOutput
          → If reject: return OpenValidationError with notify code/subcode
        → If all accept: return nil
    → Session receives error → NOTIFICATION → close
    → Session receives nil → continue to negotiate
```

### Boundaries Crossed

| Boundary | How | Notes |
|----------|-----|-------|
| Session → Peer | `openValidator` callback closure | Same pattern as `pluginCapGetter` |
| Peer → Server | `r.api.BroadcastValidateOpen()` | Same pattern as config-verify broadcast |
| Server → Plugin | `connB.SendValidateOpen()` via Socket B | Same pattern as `SendConfigVerify` |
| Plugin → Server | JSON response `{accept, notify-code, notify-subcode, reason}` | |

### RPC Types

New types in `pkg/plugin/rpc/types.go`:

| Type | Fields | Purpose |
|------|--------|---------|
| `ValidateOpenCapability` | `Code uint8`, `Hex string` | Single capability: code + value bytes as hex |
| `ValidateOpenMessage` | `ASN uint32`, `RouterID string`, `HoldTime uint16`, `Capabilities []ValidateOpenCapability` | One side of the OPEN exchange |
| `ValidateOpenInput` | `Peer string`, `Local ValidateOpenMessage`, `Remote ValidateOpenMessage` | Input to validate-open callback |
| `ValidateOpenOutput` | `Accept bool`, `NotifyCode uint8`, `NotifySubcode uint8`, `Reason string` | Plugin response |

Changes to existing types:
- `DeclareRegistrationInput`: add `WantsValidateOpen bool` field
- `CapabilityDecl`: remove `Strict bool` field

### Error Type

New `OpenValidationError` in `internal/plugin/server.go` (or `errors.go`):

| Field | Type | Purpose |
|-------|------|---------|
| `NotifyCode` | `uint8` | NOTIFICATION error code (e.g., 2 = Open Message Error) |
| `NotifySubcode` | `uint8` | NOTIFICATION subcode (e.g., 11 = Role Mismatch) |
| `Reason` | `string` | Human-readable reason |

Implements `error` interface. Session's `handleOpen` checks for this type and uses its fields for the NOTIFICATION.

### 🧪 TDD Test Plan (Phase 2)

#### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestSendValidateOpenAccept` | `rpc_plugin_test.go` | PluginConn sends validate-open, plugin accepts |
| `TestSendValidateOpenReject` | `rpc_plugin_test.go` | PluginConn sends validate-open, plugin rejects with code/subcode |
| `TestSDKOnValidateOpen` | `sdk/sdk_test.go` | SDK dispatches validate-open to registered callback |
| `TestSDKValidateOpenAutoRegistration` | `sdk/sdk_test.go` | WantsValidateOpen auto-set in Stage 1 when callback registered |
| `TestBroadcastValidateOpenAllAccept` | `server_test.go` | Server broadcasts to 2 plugins, both accept |
| `TestBroadcastValidateOpenOneRejects` | `server_test.go` | Server broadcasts, first plugin rejects, returns error |
| `TestBroadcastValidateOpenNoPlugins` | `server_test.go` | No plugins with WantsValidateOpen, returns nil |
| `TestValidateOpenCallback_ValidPair` | `role/role_test.go` | Role plugin accepts valid Customer↔Provider pair |
| `TestValidateOpenCallback_InvalidPair` | `role/role_test.go` | Role plugin rejects invalid pair with code=2, subcode=11 |
| `TestValidateOpenCallback_NoPeerRole_NoStrict` | `role/role_test.go` | Role plugin accepts when peer has no Role cap (non-strict) |
| `TestValidateOpenCallback_NoPeerRole_Strict` | `role/role_test.go` | Role plugin rejects when peer has no Role cap (strict mode) |
| `TestValidateOpenCallback_MultipleDifferentRoles` | `role/role_test.go` | Role plugin rejects multiple different Role caps |
| `TestValidateOpenCallback_NoLocalConfig` | `role/role_test.go` | Role plugin accepts when no config for this peer |

#### Boundary Tests

| Field | Range | Last Valid | Invalid Above |
|-------|-------|------------|---------------|
| Role value (in validate-open input) | 0-4 | 4 (Peer) | 5 |
| Capability data hex (empty) | 1+ bytes | 1 byte | 0 bytes (empty) |

#### Functional Tests

| Test | Location | End-User Scenario |
|------|----------|-------------------|
| `role-valid-pair` | `test/plugin/role-valid-pair.ci` | Customer↔Provider session established |
| `role-invalid-pair` | `test/plugin/role-invalid-pair.ci` | Customer↔Customer NOTIFICATION 2/11 |
| `role-strict` | `test/plugin/role-strict.ci` | Strict mode rejects missing Role cap |

### Files to Modify (Phase 2)

| File | Changes |
|------|---------|
| `pkg/plugin/rpc/types.go` | Add ValidateOpen types, WantsValidateOpen field, remove Strict from CapabilityDecl |
| `internal/plugin/rpc_plugin.go` | Add `SendValidateOpen()` method |
| `pkg/plugin/sdk/sdk.go` | Add `onValidateOpen` callback, `OnValidateOpen()` setter, dispatch case, auto-set `WantsValidateOpen` |
| `internal/plugin/registration.go` | Add `WantsValidateOpen` to `PluginRegistration`, remove `Strict` from `PluginCapability` and `InjectedCapability`, remove `IsCapabilityStrict()` |
| `internal/plugin/server.go` | Add `BroadcastValidateOpen()`, update `registrationFromRPC()`, remove `IsCapabilityStrict()` |
| `internal/plugin/bgp/reactor/session.go` | Replace `roleStrictChecker` with `openValidator`, remove `validateRoleWith()`, remove `SetRoleStrictChecker()` |
| `internal/plugin/bgp/reactor/peer.go` | Replace `isRoleStrict()` with `validateOpen()`, wire `SetOpenValidator()` |
| `internal/plugin/role/role.go` | Add `OnValidateOpen` callback with RFC 9234 validation logic, store `peerRoleConfig` for validate-open use |
| `internal/plugin/role/role_test.go` | Add validate-open callback tests, adapt strict propagation tests |

### Files to Delete (Phase 2)

| File | Reason |
|------|--------|
| `internal/plugin/bgp/capability/role.go` | Validation logic moves to role plugin |
| `internal/plugin/bgp/capability/role_test.go` | Tests move to role plugin |

### Files to Modify in capability.go (cleanup)

| Change | Reason |
|--------|--------|
| Remove `CodeRole Code = 9` constant | Engine no longer knows about Role |
| Remove `case CodeRole:` from `parseCapability()` | Role caps parsed as `*Unknown` (correct: engine doesn't decode them) |
| Remove `case CodeRole:` from `Code.String()` | Role is a plugin concern |

### Implementation Steps (Phase 2)

**Ordering follows TDD: test first, then implement.**

1. **RPC types** — Add `ValidateOpen*` types to `pkg/plugin/rpc/types.go`. Add `WantsValidateOpen` to `DeclareRegistrationInput`. Compile-only, no test needed.

2. **PluginConn method** — Add `SendValidateOpen()` to `internal/plugin/rpc_plugin.go` following `SendConfigVerify` pattern. Write test in `rpc_plugin_test.go`.

3. **SDK callback** — Add `onValidateOpen` field, `OnValidateOpen()` setter, dispatch case in `dispatchCallback()`, and handler in `pkg/plugin/sdk/sdk.go`. Auto-set `WantsValidateOpen` in `Run()` when callback is registered. Write SDK test.

4. **OpenValidationError type** — Define in `internal/plugin/server.go` with `NotifyCode`, `NotifySubcode`, `Reason` fields.

5. **Engine registration** — Add `WantsValidateOpen` to `PluginRegistration`, update `registrationFromRPC()`. Remove `Strict` from `PluginCapability` and `InjectedCapability`. Remove `IsCapabilityStrict()` from `CapabilityInjector` and `Server`.

6. **Server broadcast** — Add `BroadcastValidateOpen()` that iterates processes with `WantsValidateOpen=true`, calls `connB.SendValidateOpen()`, returns `OpenValidationError` on first rejection. Write server test.

7. **Session/Peer wiring** — Replace `roleStrictChecker` with `openValidator func(string, *message.Open, *message.Open) error` on Session. Replace `SetRoleStrictChecker` with `SetOpenValidator`. In Peer, replace `isRoleStrict()` with `validateOpen()` that calls `r.api.BroadcastValidateOpen()`. Wire in `runOnce()`.

8. **Move validation to role plugin** — Add `OnValidateOpen` callback in `RunRolePlugin`. Move `ValidateRole`, `ValidateRolePair`, `validRolePairs`, `RoleName`, role constants from `capability/role.go` to `internal/plugin/role/role.go`. The plugin extracts Role capabilities from the `ValidateOpenCapability` hex data, applies RFC 9234 Table 2 validation, and returns accept/reject.

9. **Delete engine role code** — Delete `capability/role.go` and `capability/role_test.go`. Remove `CodeRole` from `capability.go`. All tests should still pass.

10. **Remove Strict field** — Remove from `rpc.CapabilityDecl`, `PluginCapability`, `InjectedCapability`. Update role plugin to not set `Strict` (it handles strict internally via validate-open). Update tests.

11. **Functional tests** — Create `.ci` files for role validation scenarios.

12. **Verify** — `make lint && make test && make functional`

### Open Questions (Phase 2)

1. **~~OPEN message serialization for validate-open~~** — Resolved: convert `*message.Open` to `ValidateOpenMessage` by iterating `OptionalParams` capabilities. Each capability becomes `{code, hex_of_value_bytes}`. The plugin only parses what it needs.

2. **~~Timing: before or after negotiation?~~** — Resolved: BEFORE negotiation. Role validation is a precondition for session establishment. Saves work if rejected. The validate-open callback receives raw OPENs, not negotiated state.

3. **~~Multiple plugins rejecting~~** — Resolved: fail-fast. First plugin to reject wins. The error carries the NOTIFICATION code/subcode.

4. **~~Backward compatibility of Strict field removal~~** — No users, no compatibility needed (per `.claude/rules/compatibility.md`).

---

**Created:** 2026-01-01
**Updated:** 2026-02-12
