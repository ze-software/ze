# Spec: RFC 9234 BGP Role Plugin

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9234.md` - RFC summary
4. `internal/plugin/gr/gr.go` - plugin pattern reference
5. `internal/plugin/registration.go` - plugin protocol

## Status: Ready for Implementation

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
- [ ] `internal/plugin/gr/gr.go` - plugin implementation pattern
- [ ] `yang/ze-gr.yang` - YANG augment pattern

**Key insights:**
- Role capability code 9, length 1, value 0-4
- Plugin validates role pairs per RFC 9234 Table 2
- Plugin controls accept/reject, engine just delivers events

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
declare conf schema capability role { role <role:[a-z-]+>; }
```

### Plugin Config Delivery (Stage 2)

Engine delivers per-peer config:
```
config peer 10.0.0.1 capability role role customer
config done
```

Plugin stores: peer 10.0.0.1 → local-role = customer

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
   - Parse `config peer X capability role role Y`
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

---

**Created:** 2026-01-01
**Updated:** 2026-01-26
