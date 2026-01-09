# Spec: api-capability-contract

## Task

Implement a plugin registration protocol where plugins proactively declare their capabilities, commands, config hooks, and address families at startup. This replaces the previous capability confirmation model.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ARCHITECTURE.md` - current API design
- [ ] `docs/architecture/api/CAPABILITY_CONTRACT.md` - existing capability contract
- [ ] `docs/architecture/config/SYNTAX.md` - config parsing requirements

### RFC Summaries (MUST for protocol work)
- [ ] `docs/rfc/rfc4271.md` - BGP-4 base protocol
- [ ] `docs/rfc/rfc4724.md` - Graceful Restart
- [ ] `docs/rfc/rfc2918.md` - Route Refresh
- [ ] `docs/rfc/rfc7313.md` - Enhanced Route Refresh

**Key insights:**
- API section must be parsed first in config
- Plugins start in parallel, stages synchronized
- Capability bytes provided by plugins for OPEN messages

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestParseRegistration` | `pkg/api/registration_test.go` | Parse registration commands |
| `TestParseConfigPattern` | `pkg/api/registration_test.go` | Config pattern with regex captures |
| `TestConflictDetection` | `pkg/api/registry_test.go` | Command and capability conflicts |
| `TestConfigDelivery` | `pkg/api/config_test.go` | Config matching and delivery format |
| `TestCapabilityInjection` | `pkg/api/capability_test.go` | Capability bytes added to OPEN |
| `TestStageSynchronization` | `pkg/api/startup_test.go` | All plugins complete stage before next |

### Functional Tests
| Test | Location | Scenario |
|------|----------|----------|
| `plugin-registration` | `qa/tests/api/` | Full 5-stage startup sequence |
| `plugin-conflict` | `qa/tests/api/` | Command conflict refuses to start |
| `plugin-timeout` | `qa/tests/api/` | Stage timeout kills plugin |
| `plugin-failed` | `qa/tests/api/` | Plugin sends failed, startup aborts |

## Files to Modify
- `pkg/api/registration.go` - registration command parsing
- `pkg/api/registry.go` - command/capability registry with conflict detection
- `pkg/api/process.go` - staged startup protocol
- `pkg/api/config.go` - config pattern matching and delivery
- `pkg/reactor/reactor.go` - API section first, capability injection
- `pkg/bgp/message/open.go` - accept plugin-provided capability bytes

## Implementation Steps
1. **Write tests** - Create unit tests for registration parsing
2. **Run tests** - Verify FAIL (paste output)
3. **Implement registration parser** - Parse all registration commands
4. **Run tests** - Verify PASS (paste output)
5. **Write conflict tests** - Test command/capability conflict detection
6. **Implement registry** - Track registrations, detect conflicts
7. **Write config delivery tests** - Test pattern matching
8. **Implement config delivery** - Match patterns, format output
9. **Write capability tests** - Test OPEN injection
10. **Implement capability injection** - Add plugin bytes to OPEN
11. **Write startup tests** - Test stage synchronization
12. **Implement staged startup** - Synchronize all plugins per stage
13. **Verify all** - `make lint && make test && make functional`

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC summaries read (all referenced RFCs)
- [ ] `docs/architecture/api/ARCHITECTURE.md` updated
- [ ] `docs/architecture/config/SYNTAX.md` updated

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-api-capability-contract.md`

---

# Protocol Specification

## Overview

Plugins register capabilities, commands, and configuration hooks with ZeBGP at startup.
This replaces the previous capability confirmation model with a proactive registration system.

## Key Concepts

| Concept | Description |
|---------|-------------|
| **Plugin** | External process communicating via stdin/stdout |
| **Registration** | Plugin declares what it handles/needs |
| **Staged startup** | Synchronized phases, all plugins complete each stage |
| **RFC grouping** | Features identified by RFC number for human readability |

## Startup Sequence

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. Parse API section of config ONLY                             │
│ 2. Start all plugins (parallel)                                 │
│ 3. Stage 1: Registration      (Plugin → ZeBGP) - all parallel   │
│ 4. Stage 2: Config Delivery   (ZeBGP → Plugin)                  │
│ 5. Stage 3: Capability Decl   (Plugin → ZeBGP)                  │
│ 6. Stage 4: Registry Sharing  (ZeBGP → Plugin)                  │
│ 7. Stage 5: Ready             (Plugin → ZeBGP)                  │
│ 8. Parse rest of config                                         │
│ 9. Start peer sessions                                          │
└─────────────────────────────────────────────────────────────────┘
```

- Plugins start in parallel
- Each stage synchronized: ZeBGP waits for ALL plugins to complete before next stage
- Timeout: configurable per API, default 1s per stage
- Any conflict (command or capability type) → refuse to start

## Protocol Stages

### Stage 1: Registration (Plugin → ZeBGP)

Plugin declares RFCs, encodings, families, config patterns, and commands.

```
rfc add 4271
rfc add 9234
encoding add text
encoding add b64
family add ipv4 unicast
family add ipv6 unicast
family add all
conf add peer * capability hostname <hostname:.*>
cmd add rib adjacent in show
cmd add rib adjacent out show
cmd add rib adjacent out clear
registration done
```

#### Syntax Reference

| Command | Description |
|---------|-------------|
| `rfc add <number>` | Declare RFC implementation (for human info) |
| `encoding add <enc>` | Declare supported encoding (text, b64, hex) |
| `family add <afi> <safi>` | Register to receive updates for family |
| `family add all` | Receive all updates |
| `conf add <pattern>` | Register config hook with regex captures |
| `cmd add <command>` | Register command handler |
| `registration done` | Signal stage complete |

#### Config Pattern Syntax

```
conf add peer * capability hostname <hostname:.*>
         │    │             │        └─ <name:regex> capture
         │    │             └─ literal path
         │    └─ glob wildcard
         └─ config section
```

- `*` matches any single path element
- `<name:regex>` captures value with validation
- Multiple plugins can match same config (both receive it)
- Multiple captures per pattern supported

#### Multiple Captures Example

**Registration:**
```
conf add peer * capability graceful-restart <restart-time:\d+> <forwarding:(true|false)>
```

**Config Delivery:**
```
configuration peer 192.168.1.1 restart-time set 120
configuration peer 192.168.1.1 forwarding set true
configuration peer 192.168.1.2 restart-time set 90
configuration peer 192.168.1.2 forwarding set false
configuration done
```

Each `<name:regex>` becomes a separate line: `configuration <context> <name> set <value>`.

### Stage 2: Config Delivery (ZeBGP → Plugin)

ZeBGP sends matching config lines with captured values.

```
configuration peer 192.168.1.1 hostname set router1.example.com
configuration peer 192.168.1.2 hostname set router1.example.com
configuration done
```

- Only lines matching registered patterns
- Each capture: `configuration <context> <name> set <value>`
- Ends with `configuration done`

### Stage 3: Open Capability (Plugin → ZeBGP)

Plugin provides capability bytes for OPEN messages.

```
open b64 capability 73 set <base64-encoded-payload>
open b64 capability 74 set <base64-encoded-payload>
open done
```

- Multiple capabilities allowed
- Payload includes actual data (e.g., hostname from config)
- Capability type conflict between plugins → refuse to start

#### Capability Syntax

```
open <encoding> capability <code> set <payload>
     │                     │          └─ encoded capability value
     │                     └─ capability type code
     └─ b64, hex, or text
```

### Stage 4: Registry Sharing (ZeBGP → Plugin)

ZeBGP tells each plugin its name and all registered commands.

```
api name announce-routes
api announce-routes text cmd rib adjacent out show
api announce-routes text cmd rib adjacent out clear
api route-reflector text cmd rib adjacent in show
api done
```

- Plugin learns its configured name
- Plugin learns all commands from all plugins with their encoding
- Enables cross-plugin command invocation

### Stage 5: Ready (Plugin → ZeBGP)

Plugin signals startup complete or failure.

**Success:**
```
ready
```

**Failure:**
```
failed text error message here
failed b64 <base64-encoded-message>
```

**Note:** `failed` can be sent at ANY stage, not just Stage 5. Sending `failed` immediately terminates startup.

## Conflict Rules

| Conflict Type | Behavior |
|---------------|----------|
| Command conflict | Two plugins register same command → refuse to start |
| Capability type conflict | Two plugins register same type code → refuse to start |
| Config pattern overlap | Both plugins receive matching config (allowed) |
| Family overlap | Both plugins receive updates (allowed) |

## Update Delivery

Updates delivered using existing format (unchanged from current implementation).

- Plugin receives updates for registered families only
- If update contains only unregistered families → not delivered
- If update contains mixed families → delivered, plugin ignores unknown
- ZeBGP does NOT modify update wire format
- Binary-mode plugins may fail if unknown families negotiated (acceptable)

## Timeout and Errors

### Timeout Handling

- Default: 1s per stage
- Configurable per API in config
- On timeout: kill plugin, refuse to start

### Error Conditions

| Condition | Result |
|-----------|--------|
| Stage timeout | Kill plugin, startup fails |
| Command conflict | Log error, refuse to start |
| Capability conflict | Log error, refuse to start |
| `failed` response | Log message, startup fails |
| Invalid syntax | Treat as timeout |

## Cross-Plugin Commands

Plugins can invoke commands registered by other plugins:

```
api route-reflector cmd rib adjacent out show
```

ZeBGP routes command to the plugin that registered it.

## Example: Hostname Capability Plugin

### Stage 1: Registration
```
rfc add 9234
encoding add text
encoding add b64
conf add peer * capability hostname <hostname:.*>
registration done
```

### Stage 2: Config Received
```
configuration peer 192.168.1.1 hostname set router1.example.com
configuration peer 192.168.1.2 hostname set router1.example.com
configuration done
```

### Stage 3: Open Capability
```
open b64 capability 73 set cm91dGVyMS5leGFtcGxlLmNvbQ==
open done
```

### Stage 4: Registry Received
```
api name hostname-plugin
api done
```

### Stage 5: Ready
```
ready
```

## Example: RIB Plugin (RFC 4271)

### Stage 1: Registration
```
rfc add 4271
encoding add text
family add ipv4 unicast
family add ipv6 unicast
cmd add rib adjacent in show
cmd add rib adjacent out show
cmd add rib adjacent out clear
cmd add rib loc show
registration done
```

### Stage 2: Config Received
```
configuration done
```
(No config patterns registered)

### Stage 3: Open Capability
```
open done
```
(No capabilities to add)

### Stage 4: Registry Received
```
api name rib-manager
api rib-manager text cmd rib adjacent in show
api rib-manager text cmd rib adjacent out show
api rib-manager text cmd rib adjacent out clear
api rib-manager text cmd rib loc show
api done
```

### Stage 5: Ready
```
ready
```

## Example: Graceful Restart Plugin

### Stage 1: Registration
```
rfc add 4724
encoding add text
family add ipv4 unicast
family add ipv6 unicast
conf add peer * capability graceful-restart <restart-time:\d+>
cmd add peer * refresh
registration done
```

### Stage 2: Config Received
```
configuration peer 192.168.1.1 restart-time set 120
configuration peer 192.168.1.2 restart-time set 120
configuration done
```

### Stage 3: Open Capability
```
open b64 capability 64 set <gr-capability-bytes>
open done
```

### Stage 4: Registry Received
```
api name graceful-restart
api graceful-restart text cmd peer * refresh
api rib-manager text cmd rib adjacent out show
api done
```

### Stage 5: Ready
```
ready
```

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Proactive registration | Plugin declares capabilities, not router |
| RFC-based grouping | Human-readable feature organization |
| Staged sync startup | All plugins ready before peers start |
| Config-first API section | Plugins must be ready for config validation |
| No plugin dependencies | Out of scope; plugins receive all commands in registry |
| Negotiation feedback | Out of scope; ZeBGP SHOULD implement later |

## Configuration

### API Section (Must Be First)

```
api announce-routes {
    run ./plugins/announce-routes;
    timeout 2s;  # per-stage timeout, default 1s
}

api route-reflector {
    run ./plugins/route-reflector;
    timeout 5s;
}
```

### Peer Binding (After API Section)

```
peer 192.168.1.1 {
    capability {
        hostname router1.example.com;
        graceful-restart 120;
    }
    api announce-routes {
        send { update; }
    }
}
```

## ZeBGP Internal Commands

ZeBGP provides built-in commands to inspect registered features:

```
show api plugins           # List all plugins and status
show api commands          # List all registered commands
show api capabilities      # List all registered capability types
show api families          # List family registrations per plugin
show rfc                   # List known RFCs with descriptions
```

## References

- RFC 4271: A Border Gateway Protocol 4 (BGP-4)
- RFC 4724: Graceful Restart Mechanism for BGP
- RFC 2918: Route Refresh Capability for BGP-4
- RFC 7313: Enhanced Route Refresh Capability for BGP-4
- RFC 9234: Route Leak Prevention and Detection Using Roles
- draft-ietf-idr-bgp-hostname: Hostname Capability for BGP
