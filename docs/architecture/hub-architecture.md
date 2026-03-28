# Hub Architecture

**Status:** Design aspiration (partially implemented)

**Purpose:** Document the future architecture where `ze` acts as a central Hub orchestrating separate processes for BGP, RIB, GR, and third-party plugins.

---

## Vision

### What We're Building

A **plugin-based configuration system** where Ze acts as a central Hub orchestrating separate processes. Each plugin defines its own configuration schema using YANG, and the Hub routes configuration to the appropriate plugin.

### Why This Matters

| Goal | Benefit |
|------|---------|
| **Extensibility** | Anyone can write a plugin that adds config sections |
| **Type Safety** | YANG validates config before plugins see it |
| **Atomicity** | No partial config state; all-or-nothing startup |
| **Language Freedom** | Plugins can be Go, Python, Rust, shell scripts |
| **Debuggability** | Schema commands expose what config is valid |

### End State

```
# Third-party plugin adds its own config section
acme-monitor {
    endpoint "https://monitor.example.com";
    interval 30;
}

bgp {
    local-as 65001;
    router-id 1.2.3.4;
    peer transit-a {
        remote {
            ip 192.0.2.1;
            as 65002;
        }
    }
}
```

- YANG validates types, ranges, patterns
- Two-phase commit: verify all → apply all
- Hot reload: only changed blocks re-verified

---

## Implementation Progress

**Foundation already exists** (see commit `19b6564`):

| Component | Status | Location |
|-----------|--------|----------|
| Forked subsystem processes | ✅ Done | `cmd/ze-subsystem/main.go` |
| 5-stage protocol | ✅ Done | `internal/component/plugin/registration.go` |
| Bidirectional pipe communication | ✅ Done | `SubsystemHandler`, `Process` |
| Command routing to processes | ✅ Done | `SubsystemManager.FindHandler()` |
| Dynamic command registration | ✅ Done | `declare cmd <name>` in protocol |
<!-- source: internal/component/plugin/registration.go -- 5-stage protocol -->
<!-- source: internal/component/plugin/process/ -- Process management -->

**Still needed for full Hub Architecture:**

| Component | Status | Description |
|-----------|--------|-------------|
| YANG schema registration | ✅ Done | `declare schema` message type in registration.go |
| Config priority | ✅ Done | `declare priority` + Priority field in Schema |
| Verify/Apply protocol | ❌ Needed | Add `config verify`, `config apply` |
| Handler path routing | ✅ Done | SchemaRegistry.FindHandler (longest prefix) |
| libyang integration | ❌ Needed | For YANG validation |
| Live/Edit config query | ❌ Needed | `query config live/edit path "..."` |

---

## Overview

Ze will evolve to a Hub-based architecture where all subsystems are separate processes communicating via pipes. This enables:

- Third-party plugins that extend configuration schema
- Clean separation of concerns
- Independent development and testing of subsystems
- Language flexibility for plugin authors

```
ze config.conf
      │
      ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                              ze (Hub)                                    │
│                                                                         │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │
│   │   Schema    │  │   Config    │  │   Router    │  │     API     │   │
│   │  Registry   │  │   State     │  │             │  │    Layer    │   │
│   └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘   │
│                                                                         │
└───────────────┬─────────────────────────┬─────────────────────────────┘
                │                         │
        pipes   │                         │   pipes
                │                         │
                ▼                         ▼
         ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐
         │    BGP    │ │    RIB    │ │    GR     │ │  Third    │
         │  Plugin   │ │  Plugin   │ │  Plugin   │ │  Party    │
         └───────────┘ └───────────┘ └───────────┘ └───────────┘
```

---

## Design Principles

### 1. Single Entry Point

`ze config.conf` starts everything. The config file determines what gets started.

### 2. Plugins Define Their Own Schema

Each plugin (including BGP engine) sends its YANG schema to the Hub at startup. Third-party plugins can extend the configuration schema without modifying Ze.

### 3. Hub Validates Against Combined Schema

The Hub collects all YANG schemas from plugins, then validates the config file against the combined schema. This enables:

- Type validation (ranges, patterns, enums)
- Cross-reference validation (leafref)
- Semantic validation via verify/apply protocol

### 4. Verify Before Apply

Configuration changes go through two phases:

1. **Verify** - Plugin validates proposed change (can reject)
2. **Apply** - Plugin applies validated change (after all verify pass)

---

## Startup Sequence

### Current 5-Stage Protocol (Already Implemented)

The existing subsystem infrastructure uses this protocol (see `internal/component/plugin/subsystem.go`):

```
Stage 1: DECLARATION    Plugin → Engine: declare cmd/encoding/...
                                         declare done
Stage 2: CONFIG         Engine → Plugin: config <key> <value>
                                         config done
Stage 3: CAPABILITY     Plugin → Engine: capability hex <code> <value>
                                         capability done
Stage 4: REGISTRY       Engine → Plugin: registry cmd <name>
                                         registry done
Stage 5: READY          Plugin → Engine: ready
```

### Extended 5-Stage Protocol (Hub Architecture)

The existing 5-stage protocol is **extended**, not replaced. Schema declarations are added to Stage 1. Stage 2 waits for initial commit before proceeding.

**Key principle:** Config is only sent to plugins on explicit commit:
- **Startup:** Initial commit after Stage 1 completes
- **SIGHUP:** Reload triggers new commit
- **CLI:** `ze config commit` command

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         EXTENDED 5-STAGE PROTOCOL                        │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  STAGE 1: DECLARATION (extended with schema)                            │
│  ──────────────────────────────────────────                             │
│  Plugin → Hub: declare schema module <name> namespace <ns>              │
│  Plugin → Hub: declare schema yang "<yang-text>"                        │
│  Plugin → Hub: declare schema handler <path>                            │
│  Plugin → Hub: declare priority <number>                                │
│  Plugin → Hub: declare cmd <command>                                    │
│  Plugin → Hub: declare done                                             │
│                                                                         │
│  ─── BARRIER: All plugins declared ───                                  │
│                                                                         │
│  STAGE 2: INITIAL COMMIT (VyOS-inspired verify/apply)                   │
│  ─────────────────────────────────────────────────────                  │
│  Hub parses config file against combined YANG                           │
│  Hub stores config as edit state (live is empty at startup)             │
│  Hub performs commit (same as SIGHUP reload or CLI commit):             │
│                                                                         │
│  For each plugin (ordered by priority):                                 │
│    Hub → Plugin: #1 config verify                                       │
│    Plugin → Hub: #2 query config live path "..."                        │
│    Hub → Plugin: @2 done data '{}'  (empty at startup)                  │
│    Plugin → Hub: #3 query config edit path "..."                        │
│    Hub → Plugin: @3 done data '{...}'                                   │
│    Plugin computes diff, validates                                      │
│    Plugin → Hub: @1 done (or @1 error <reason>)                         │
│                                                                         │
│  If all verify pass:                                                    │
│    For each plugin (ordered by priority):                               │
│      Hub → Plugin: #4 config apply                                      │
│      Plugin queries config, applies changes                             │
│      Plugin → Hub: @4 done                                              │
│    Hub: edit becomes live                                               │
│                                                                         │
│  Hub → All Plugins: config done                                         │
│                                                                         │
│  ─── BARRIER: Initial commit complete ───                               │
│                                                                         │
│  STAGE 3: CAPABILITY (unchanged)                                        │
│  STAGE 4: REGISTRY (unchanged)                                          │
│  STAGE 5: READY (unchanged)                                             │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Startup Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 1. ze config.conf                                                        │
│    Hub starts, reads config file (minimal parse - just plugin blocks)   │
└─────────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 2. Fork plugins from config (EXISTING - uses SubsystemManager)          │
│    - ze-subsystem --mode=bgp                                            │
│    - ze plugin rib                                                  │
│    - /path/to/third-party-plugin                                        │
└─────────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 3. STAGE 1: DECLARATION (extended)                                       │
│    Each plugin sends:                                                   │
│      declare schema module ze-bgp-conf namespace urn:ze:bgp:conf                  │
│      declare schema yang "module ze-bgp-conf { ... }"                        │
│      declare schema handler bgp                                         │
│      declare schema handler bgp.peer                                    │
│      declare priority 100                                               │
│      declare cmd bgp peer list                                          │
│      declare done                                                       │
│    Hub collects all schemas, builds priority order                      │
│    ─── BARRIER: All plugins declared ───                                │
└─────────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 4. STAGE 2: CONFIG (VyOS-inspired verify/apply)                          │
│    Hub parses full config against combined YANG                         │
│    Hub stores as edit state                                             │
│    For each plugin (by priority):                                       │
│      Hub sends: config verify                                           │
│      Plugin queries live/edit config                                    │
│      Plugin computes diff, validates                                    │
│      Plugin responds: done or error                                     │
│    If all pass:                                                         │
│      For each plugin (by priority):                                     │
│        Hub sends: config apply                                          │
│        Plugin applies changes                                           │
│      Hub: edit becomes live                                             │
│    Hub sends: config done (to all plugins)                              │
│    ─── BARRIER: Config applied ───                                      │
└─────────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 5. STAGES 3-5: CAPABILITY, REGISTRY, READY (unchanged)                   │
│    (EXISTING - already implemented in SubsystemHandler.completeProtocol)│
└─────────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 6. BGP peers start                                                       │
└─────────────────────────────────────────────────────────────────────────┘
```

### Stage 1: Declaration Messages

**Schema declaration (NEW):**

Schema declarations use `declare schema` prefix (consistent with `declare cmd`):

```
declare schema module ze-bgp-conf
declare schema namespace urn:ze:bgp:conf
declare schema handler bgp
declare schema handler bgp.peer
declare schema yang <<EOF
module ze-bgp-conf {
  namespace "urn:ze:bgp:conf";
  prefix bgp;
  ...
}
EOF
```

**Command declaration (EXISTING):**

```
declare cmd bgp peer list
declare cmd bgp peer detail
declare encoding json
declare done
```

**Key points:**
- `declare schema` prefix matches `declare cmd` pattern
- `declare schema handler <path>` - register config handler paths (longest-prefix routing)
- `declare schema yang <<EOF...EOF` - YANG content inline via heredoc
- Hub stores YANG content for config validation
<!-- source: internal/component/plugin/registration.go -- declare schema parsing -->

**Schema debugging (CLI):**
```bash
$ ze bgp schema show
module ze-bgp-conf {
  namespace "urn:ze:bgp:conf";
  prefix bgp;
  ...
}
```

Same content available via CLI for human debugging.

### Stage 2: Verify/Apply Message Flow (VyOS-inspired)

Plugins pull config from hub, compute diff, validate and apply. All messages use `#serial command` format.

```
Hub                                        Plugin (BGP)
 │                                              │
 │  (Hub parsed config, stored as edit)         │
 │                                              │
 │── #1 config verify ─────────────────────────>│
 │                                              │
 │<── #2 query config live path "bgp" ──────────│
 │                                              │
 │── @2 done data '{...current config...}' ────>│
 │                                              │
 │<── #3 query config edit path "bgp" ──────────│
 │                                              │
 │── @3 done data '{...new config...}' ────────>│
 │                                              │
 │                        (plugin computes diff) │
 │                        (plugin validates)     │
 │                                              │
 │<── @1 done ──────────────────────────────────│
 │                                              │
 │     (all plugins verified - ordered by priority)
 │                                              │
 │── #4 config apply ──────────────────────────>│
 │                                              │
 │<── #5 query config edit path "bgp.peer" ─────│
 │                                              │
 │── @5 done data '{...}' ─────────────────────>│
 │                                              │
 │                        (plugin applies changes)
 │                                              │
 │<── @4 done ──────────────────────────────────│
 │                                              │
 │  (Hub: edit becomes live)                    │
 │                                              │
 │── config done ──────────────────────────────>│
```

**Key points:**
- Hub notifies plugin to verify/apply
- Plugin queries hub for live and edit config
- Plugin computes diff using shared library (`internal/component/config/diff/`)
- Plugin validates and applies
- Plugins processed in priority order (lower = first)

### Building on Existing Code

**SubsystemHandler extension (hub-side):**

The existing `SubsystemHandler.completeProtocol()` in `internal/component/plugin/subsystem.go` needs extension:

1. Stage 1 loop already parses `declare cmd <name>`
2. Add parsing for `declare schema module <name>`
3. Add parsing for `declare schema handler <path>`
4. Add heredoc parsing for `declare schema yang <<EOF...EOF`
5. Store collected schemas in SubsystemHandler

**ze-subsystem extension (plugin-side):**

The existing `ze-subsystem` binary needs extension:

1. Send `declare schema module <name>` before other declarations
2. Send `declare schema namespace <uri>`
3. Send `declare schema handler <path>` for each config path handled
4. Send `declare priority <number>` for config ordering
5. Send `declare schema yang <<EOF...EOF` with full YANG content
6. Continue with existing `declare cmd` and `declare done`

---

## Schema Storage

### Schema Fields

Hub stores schema information collected from plugins:

| Field | Description |
|-------|-------------|
| Module | YANG module name (from `declare schema module`) |
| Namespace | YANG namespace URI (from `declare schema namespace`) |
| Yang | Full YANG module text (from `declare schema yang`) |
| Handlers | Handler paths (from `declare schema handler`) |
| Priority | Config ordering (from `declare priority`, lower = first) |
| Plugin | Name of plugin that registered this schema |

### Handler Routing

The Hub routes config events based on handler prefix using **longest prefix match**:

| Handler path | Routed to |
|--------------|-----------|
| `bgp.*` | BGP engine |
| `rib.*` | RIB plugin |
| `acme.*` | Third-party ACME plugin |

### Handler Path Syntax

Handler paths use dot-separated segments with optional key syntax for list instances:

| Type | Format | Example |
|------|--------|---------|
| Container | `segment.segment` | `bgp` |
| List item | `segment[key=value]` | `bgp.peer[address=192.0.2.1]` |
| Nested | `segment.segment[key=value]` | `bgp.peer[address=192.0.2.1].capability` |

**Mapping from config syntax:**
```
bgp {                            → handler: bgp
    peer transit-a { ... }       → handler: bgp.peer
                                   path: bgp.peer[name=transit-a]
}
```

**Routing example:**
- Request for `bgp.peer[name=transit-a]`
- Registered handlers: `bgp`, `bgp.peer`
- Longest match: `bgp.peer` → routes to BGP plugin

---

## Config Verification Protocol (VyOS-inspired)

### Message Formats

All messages follow standard IPC protocol patterns. Plugins pull config from hub.

**Hub → Plugin: verify notification**
```
#1 config verify
```

**Plugin → Hub: query live config**
```
#2 query config live path "bgp"
```

**Hub → Plugin: live config response**
```
@2 done data '{"local-as": 65001, "peer": [...]}'
```

**Plugin → Hub: query edit config**
```
#3 query config edit path "bgp"
```

**Hub → Plugin: edit config response**
```
@3 done data '{"local-as": 65001, "peer": [...new peer...]}'
```

**Plugin → Hub: verify response (after computing diff)**
```
@1 done
```
or
```
@1 error peer-as cannot equal local-as
```

**Hub → Plugin: apply notification (after all verify pass)**
```
#4 config apply
```

**Plugin → Hub: apply response (after applying changes)**
```
@4 done
```

**Hub → Plugin: config complete**
```
config done
```

**Key consistency points:**
- Commands use `#serial command args` format
- Responses use `@serial status [data]` format
- Status values: `done`, `error` (standard IPC values)
- Plugins pull config, hub doesn't push diffs
- Plugins use shared diff library to compute changes

### Priority Ordering

Plugins declare priority during Stage 1. Hub processes verify/apply in priority order:

| Priority | Plugin | Typical Use |
|----------|--------|-------------|
| 100 | BGP | Core protocol |
| 200 | RIB | Depends on BGP config |
| 300 | GR | Augments BGP |
| 1000 | Third-party | After core |

Lower priority = processed first. This ensures dependencies are configured before dependents.

### Handler Interface (Plugin Side)

Plugins implement verify and apply handlers. They query hub for config, compute diff, validate and apply.

**Verify handler:**
1. Receive `#serial config verify`
2. Query hub for live config: `#N query config live path "<handler-path>"`
3. Query hub for edit config: `#N query config edit path "<handler-path>"`
4. Compute diff using shared library (`internal/component/config/diff/`)
5. Perform semantic validation (YANG already validated types)
6. Return `@serial done` or `@serial error <message>`

**Example semantic validations:**
- peer-as cannot equal local-as for eBGP
- address must not already exist (for create action)
- referenced objects must exist

**Apply handler:**
1. Receive `#serial config apply`
2. Query hub for edit config (new state)
3. Apply the validated changes to internal state
4. Return `@serial done` or `@serial error <message>`

### Hub Implementation

**Startup sequence:**

1. **Fork all plugins** - Start each plugin process
2. **Stage 1: Collect declarations** - Read until each sends `declare done`
   - Collect schemas from all plugins
   - Collect priorities for ordering
   - BARRIER: wait for all plugins
3. **Stage 2: Config (VyOS-inspired verify/apply)**
   - Hub parses full config against combined YANG
   - Hub stores config as edit state
   - For each plugin (by priority, lower first):
     - Send `#serial config verify`
     - Handle `query config live/edit` requests
     - Wait for `@serial done/error` response
   - If all verify pass:
     - For each plugin (by priority):
       - Send `#serial config apply`
       - Handle query requests
       - Wait for response
     - Hub: edit becomes live
   - Send `config done` to all plugins
   - BARRIER: config applied
4. **Stages 3-5** - Continue normal protocol (unchanged)

**Handler routing:**
- Use longest prefix match for handler path
- Example: `bgp.peer.timers` matches handler `bgp.peer` (not `bgp`)
- Unknown handler → error response

**Failure handling:**
- Verify failure → abort startup, return error
- Apply failure → abort startup, system does not run

---

## YANG and leafref

### What is leafref?

leafref is a YANG type that references another leaf's value, like a foreign key in a database.

```yang
list peer {
  key "address";
  leaf address { type inet:ip-address; }
  leaf peer-as { type uint32; }
}

// Third-party plugin referencing BGP peer
leaf monitored-peer {
  type leafref {
    path "/bgp:bgp/bgp:peer/bgp:address";
  }
}
```

### Config Example

```
bgp {
    peer transit-a {
        remote {
            ip 192.0.2.1;
            as 65000;
        }
    }
}

acme-monitor {
    monitored-peer transit-a;   # Valid - peer exists
    monitored-peer no-such;     # INVALID - leafref target not found
}
```

### Cross-Plugin leafref

Plugins can reference config from other plugins:

```yang
// In third-party plugin
leaf upstream-peer {
  type leafref {
    path "/bgp:bgp/bgp:peer/bgp:address";
  }
}
```

This requires all YANG modules to be loaded together in the Hub.

---

## CLI Schema Discovery

External developers need to know the expected format. Uses `system` namespace (consistent with `system subsystem list`):

```bash
# Query protocol version and format
$ ze system schema protocol
version: 2.1
schema_format: yang-text
message_format: text
transport: stdio

# List registered schemas
$ ze system schema list
ze-bgp-conf: bgp, bgp.peer
ze-rib: rib

# Show specific schema
$ ze system schema show ze-bgp-conf
module ze-bgp-conf {
  namespace "urn:ze:bgp:conf";
  ...
}

# Show example messages
$ ze system schema example verify
$ ze system schema example apply
```

**Note:** All commands use text protocol, human-readable output.

---

## Hub Config Processing (VyOS-inspired)

Hub handles all config processing internally (no separate Config Reader process).

### Config Notification (Pull Model)

**Hub never sends config data. Hub only notifies plugins, plugins query for config.**

| Trigger | Hub Action |
|---------|------------|
| **Startup** | Notifies plugins: `config verify` then `config apply` |
| **SIGHUP** | Notifies plugins: `config verify` then `config apply` |
| **CLI commit** | Notifies plugins: `config verify` then `config apply` |

Plugins respond to notifications by querying hub for live/edit config, computing diff, and applying changes themselves.
<!-- source: internal/component/config/diff.go -- config diff computation -->

### Responsibilities

1. Parse config file using existing tokenizer
2. Validate against combined YANG schema (collected from plugins)
3. Store config as live/edit states
4. On commit: notify plugins (`config verify`, `config apply`)
5. Respond to plugin config queries (hub never pushes config)

### Config States

| State | Description |
|-------|-------------|
| **live** | Current running configuration |
| **edit** | Candidate configuration (being committed) |

On startup, live is empty, edit is loaded from file, then edit becomes live after all plugins apply.
On reload/commit, new edit is loaded, verified, then becomes live.

### Query Protocol

Plugins query config using text protocol:

```
#N query config <state> path "<handler-path>"
@N done data '<json>'
```

Where:
- `<state>` = `live` or `edit`
- `<handler-path>` = dot-separated path like `bgp.peer`

### Commit Cycle (Startup, SIGHUP, or CLI)

On any commit trigger:

1. Hub parses config file (or uses already-loaded edit)
2. Hub stores as edit state
3. Hub sends `config verify` to each plugin (by priority)
4. Each plugin queries live/edit, computes diff, validates
5. If all pass: Hub sends `config apply` to each plugin
6. Plugins apply changes
7. Hub: edit becomes live
8. Hub sends `config done` to all plugins

### Command Handling

| Command | Handler |
|---------|---------|
| `config commit` | Hub handles internally (verify + apply) |
| `config reload` | Hub re-reads file, then commits |
| `config validate` | Hub handles internally (verify only, no apply) |

---

## Internal Mechanics

### Process Topology

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              ze (Hub Process)                            │
│                                                                         │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐         │
│  │ SubsystemManager│  │  SchemaRegistry │  │   Config State  │         │
│  │                 │  │                 │  │                 │         │
│  │ plugins: map    │  │ modules: map    │  │ live: map       │         │
│  │   name → Handler│  │   name → Schema │  │ edit: map       │         │
│  │ priorities: map │  │ handlers: map   │  │                 │         │
│  │                 │  │   path → Plugin │  │                 │         │
│  └────────┬────────┘  └────────┬────────┘  └─────────────────┘         │
│           │                    │                                        │
│           │    ┌───────────────┴───────────────┐                       │
│           │    │        Handler Router         │                       │
│           │    │  FindHandler(path) → Plugin   │                       │
│           │    │  (longest prefix match)       │                       │
│           │    └───────────────────────────────┘                       │
│           │                                                             │
└───────────┼─────────────────────────────────────────────────────────────┘
            │
    ┌───────┴───────┬───────────────┬───────────────┐
    │ stdin/stdout  │ stdin/stdout  │ stdin/stdout  │
    │    pipes      │    pipes      │    pipes      │
    ▼               ▼               ▼               ▼
┌────────┐    ┌────────┐    ┌────────┐    ┌─────────────┐
│  BGP   │    │  RIB   │    │   GR   │    │ Third-Party │
│ Plugin │    │ Plugin │    │ Plugin │    │   Plugin    │
└────────┘    └────────┘    └────────┘    └─────────────┘
```

### Data Structures

**SchemaRegistry** stores schema information indexed by module:

| Field | Description |
|-------|-------------|
| `modules` | Map of module name -> Schema (module, namespace, yang, handlers, plugin) |
| `handlerIndex` | Map of handler path -> plugin name (for routing) |
<!-- source: internal/component/plugin/registration.go -- schema storage -->

**SubsystemHandler** tracks each plugin process:

| Field | Description |
|-------|-------------|
| `name` | Plugin identifier |
| `process` | OS process handle |
| `stdin/stdout` | Pipe endpoints for communication |
| `serialCounter` | Atomic counter for request/response matching |
| `pending` | Map of serial → response channel (for async matching) |
| `schemas` | Schemas this plugin declared |
| `commands` | Commands this plugin declared |

### Handler Routing (Longest Prefix Match)

```
Registered handlers:
  "bgp"      → bgp plugin
  "bgp.peer" → bgp plugin
  "rib"      → rib plugin

FindHandler("bgp.peer[address=192.0.2.1]"):
  1. Try exact match "bgp.peer[address=192.0.2.1]" → not found
  2. Strip key: "bgp.peer" → FOUND → return "bgp" plugin

FindHandler("bgp.peer[address=192.0.2.1].capability"):
  1. Try "bgp.peer[address=192.0.2.1].capability" → not found
  2. Strip segment: "bgp.peer[address=192.0.2.1]" → not found
  3. Strip key: "bgp.peer" → FOUND → return "bgp" plugin

FindHandler("unknown.path"):
  1. No matches found → return error UNKNOWN_HANDLER
```

### Freeze-After-Init (Lock-Free Dispatch)

<!-- source: internal/component/plugin/server/schema.go -- frozenSchema -->
<!-- source: internal/component/plugin/server/subsystem.go -- frozenSubsystems -->
<!-- source: internal/component/plugin/server/command_registry.go -- frozenCommands -->

All three registries (SchemaRegistry, SubsystemManager, CommandRegistry) are populated during startup and never mutated during normal operation. After startup completes, each registry's `Freeze()` method creates an immutable snapshot stored via `atomic.Pointer`. Hot-path lookups (`FindHandler`, `Get`, `Lookup`) use `atomic.Load` on the frozen snapshot instead of acquiring a read lock.

| Registry | Hot-path method | Used by |
|----------|----------------|---------|
| SchemaRegistry | `FindHandler` | Hub.RouteCommand (Orchestrator config dispatch) |
| SubsystemManager | `Get`, `FindHandler` | Hub.RouteCommand, Dispatcher.Dispatch (CLI/API) |
| CommandRegistry | `Lookup` | Dispatcher.Dispatch (CLI/API) |

Pre-freeze reads fall back to the RLock path. Post-freeze mutations (e.g., `Unregister` during plugin crash recovery) rebuild and republish the frozen snapshot atomically.

### Serial Matching (Concurrent Requests)

Hub maintains pending requests per plugin for concurrent operations:

```
Send request:
    serial := nextSerial()           // atomic increment → "a"
    pending[serial] = make(chan)     // create response channel
    write(stdin, "#a config verify ...")
    response := <-pending[serial]    // block until @a received
    delete(pending, serial)
    return response

Receive response (goroutine per plugin):
    for line := range stdout {
        serial, status, data := parse(line)  // "@a done" → "a", "done", ""
        if ch, ok := pending[serial]; ok {
            ch <- Response{status, data}
        }
    }
```

This enables concurrent verify/apply to different plugins while maintaining request/response correlation.

### Config Reload Flow

```
SIGHUP or "config reload" command
         │
         ▼
┌─────────────────────────────────────────────────────┐
│                       Hub                           │
│                                                     │
│  1. Re-read config file                             │
│  2. Parse against combined YANG                     │
│  3. Store as new edit state                         │
│  4. For each plugin (by priority):                  │
│       - Send: config verify                         │
│       - Handle query requests                       │
│       - Wait for done/error                         │
│  5. If all pass:                                    │
│       - For each plugin: Send config apply          │
│       - Edit becomes live                           │
│  6. Send: config done                               │
└─────────────────────────────────────────────────────┘
```

### YANG Validation (Hub Internal)

```
┌─────────────────────────────────────────────────────────────┐
│                       Hub Config Processing                 │
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │ YANG Loader  │    │  Validator   │    │   Tokenizer  │  │
│  │  (goyang)    │───▶│ schema tree  │    │ config file  │  │
│  └──────────────┘    └──────┬───────┘    └──────┬───────┘  │
│                             │                   │          │
│                             ▼                   ▼          │
│                      ┌─────────────────────────────┐       │
│                      │      Config Processor       │       │
│                      │                             │       │
│                      │  For each block:            │       │
│                      │  1. Parse tokens → data     │       │
│                      │  2. Map to handler path     │       │
│                      │  3. Validate vs YANG:       │       │
│                      │     - Type check            │       │
│                      │     - Range check           │       │
│                      │     - Pattern check         │       │
│                      │     - Mandatory fields      │       │
│                      │  4. Leafref validation      │       │
│                      │  5. Store in edit state     │       │
│                      └─────────────────────────────┘       │
└─────────────────────────────────────────────────────────────┘
```

---

## Error Codes

| Code | Meaning |
|------|---------|
| `INVALID_YANG` | YANG schema syntax error |
| `DUPLICATE_HANDLER` | Handler already registered by another plugin |
| `UNKNOWN_HANDLER` | No plugin handles this path |
| `VALIDATION_FAILED` | Semantic validation failed in handler |
| `LEAFREF_MISSING` | Referenced value doesn't exist |
| `TYPE_ERROR` | Value doesn't match YANG type |
| `RANGE_ERROR` | Value outside allowed range |
| `PATTERN_ERROR` | Value doesn't match pattern |
| `PARSE_ERROR` | Config file syntax error |

---

## Example: Third-Party Plugin

### Plugin YANG Schema

```yang
module acme-monitor {
  namespace "urn:acme:monitor";
  prefix acme;

  container acme-monitor {
    leaf endpoint {
      type string;
      mandatory true;
    }
    leaf interval {
      type uint32 {
        range "10..3600";
      }
      default 60;
    }
  }
}
```

### Config File

```
plugin {
    external acme {
        run "/opt/acme/monitor-plugin";
    }
}

acme-monitor {
    endpoint "https://monitor.acme.example.com";
    interval 30;
}

bgp {
    peer transit-a {
        remote {
            ip 192.0.2.1;
            as 65002;
        }
    }
}
```

### Startup Flow

1. Hub forks `/opt/acme/monitor-plugin` (and ze bgp, etc.)
2. Each plugin sends `declare schema` messages with YANG content and priority
3. Hub collects all schemas, validates combined YANG
4. Hub parses config against combined schema, stores as edit
5. Hub sends `config verify` to each plugin (by priority)
6. Each plugin queries live/edit config, validates
7. All pass → Hub sends `config apply` to each plugin
8. Plugins apply changes, Hub promotes edit to live
9. Hub sends `config done` to all plugins

---

## Migration Path

**Hub separation phases are tracked in the umbrella spec `plan/spec-arch-0-system-boundaries.md`.**

### Dependency Diagram

```
Phase 1: Hub Foundation ──────► Phase 2: Config Parsing
                                        │
                                        ▼
                                Phase 3: Schema Routing
                                        │
                                        ▼
                                Phase 4: BGP Process Separation
                                        │
                                        ▼
                                Phase 5: Event/Command Routing
                                        │
                                        ▼
                                Phase 6: GR Plugin
                                        │
                                        ▼
                                Phase 7: Cleanup
```

### Foundation ✅ COMPLETE

**Already implemented** (see `internal/component/plugin/`):

- ✅ Forked subsystem processes (`cmd/ze-subsystem/`)
- ✅ 5-stage protocol via pipes (`internal/component/plugin/registration.go`)
- ✅ Bidirectional communication
- ✅ Command routing to forked processes
- ✅ SchemaRegistry with Register, FindHandler
<!-- source: internal/component/plugin/registration.go -- protocol implementation -->
<!-- source: internal/component/plugin/server/ -- plugin server -->

### Phase Overview

| Phase | Spec | Description |
|-------|------|-------------|
| 1 | `spec-hub-phase1-foundation.md` | Create `internal/component/hub/`, basic fork/pipe |
| 2 | `spec-hub-phase2-config.md` | Parse 3-section config, env handling |
| 3 | `spec-hub-phase3-schema-routing.md` | YANG validation, JSON delivery, query protocol |
| 4 | `spec-hub-phase4-bgp-process.md` | Move BGP code, ze bgp as child |
| 5 | `spec-hub-phase5-routing.md` | Event/command routing, SSH interface |
| 6 | `spec-hub-phase6-gr-plugin.md` | ze-gr.yang, capability injection |
| 7 | `spec-hub-phase7-cleanup.md` | Remove old code, update docs |

---

## Open Questions

| # | Question | Options |
|---|----------|---------|
| 1 | YANG validation library | libyang (C) vs goyang + custom (Go) |
| 2 | Schema format in messages | Raw YANG text vs file reference |
| 3 | Transaction rollback | Auto-rollback on apply failure vs partial state |

---

## Related Documents

### Architecture

- [IPC Protocol](api/ipc_protocol.md) - Wire format, event structure
- [Process Protocol](api/process-protocol.md) - Plugin lifecycle, 5-stage startup
- [YANG Config Design](config/yang-config-design.md) - VyOS-inspired YANG design
- [VyOS Research](config/vyos-research.md) - VyOS configuration architecture

### Implementation

- [Unified Subsystem Protocol](../../plan/learned/149-unified-subsystem-protocol.md) - Completed spec for subsystem infrastructure

### Source Code

- `cmd/ze-subsystem/main.go` - Forked subsystem binary
- `internal/component/plugin/registration.go` - 5-stage protocol parsing
- `internal/component/plugin/process/` - Process pipe communication
- `internal/component/plugin/server/` - Plugin server
<!-- source: internal/component/plugin/registration.go -- protocol parsing -->
<!-- source: internal/component/plugin/process/ -- process management -->

---

**Last Updated:** 2026-01-25
