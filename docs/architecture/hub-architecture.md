# Hub Architecture

**Status:** Design aspiration (partially implemented)

**Purpose:** Document the future architecture where `ze` acts as a central Hub orchestrating separate processes for BGP, RIB, Config Reader, and third-party plugins.

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
    peer 192.0.2.1 {
        peer-as 65002;
        group upstream;      # ← leafref validated against peer-group list
    }
    peer-group upstream {
        route-map-in filter; # ← leafref validated against route-map list
    }
}
```

- YANG validates types, ranges, patterns
- Leafrefs ensure cross-references exist
- Two-phase commit: verify all → apply all
- Hot reload: only changed blocks re-verified

---

## Implementation Progress

**Foundation already exists** (see commit `19b6564`):

| Component | Status | Location |
|-----------|--------|----------|
| Forked subsystem processes | ✅ Done | `cmd/ze-subsystem/main.go` |
| 5-stage protocol | ✅ Done | `internal/plugin/subsystem.go` |
| Bidirectional pipe communication | ✅ Done | `SubsystemHandler`, `Process` |
| Command routing to processes | ✅ Done | `SubsystemManager.FindHandler()` |
| Dynamic command registration | ✅ Done | `declare cmd <name>` in protocol |

**Still needed for full Hub Architecture:**

| Component | Status | Description |
|-----------|--------|-------------|
| YANG schema registration | ❌ Needed | Add `declare schema` message type |
| Config Reader process | ❌ Needed | Create `ze-config-reader` binary |
| Verify/Apply protocol | ❌ Needed | Add `config verify`, `config apply` |
| Handler path routing | ❌ Needed | Route by `bgp.*`, `rib.*` prefix |
| libyang integration | ❌ Needed | For YANG validation |

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
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                    │
│   │   Schema    │  │   Router    │  │     API     │                    │
│   │  Registry   │  │             │  │    Layer    │                    │
│   └─────────────┘  └─────────────┘  └─────────────┘                    │
│                                                                         │
└───────────────┬─────────────┬─────────────┬─────────────────────────────┘
                │             │             │
        pipes   │             │             │   pipes
                │             │             │
                ▼             ▼             ▼
         ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐
         │  Config   │ │    BGP    │ │    RIB    │ │  Third    │
         │  Reader   │ │  Engine   │ │  Plugin   │ │  Party    │
         └───────────┘ └───────────┘ └───────────┘ └───────────┘
```

---

## Design Principles

### 1. Single Entry Point

`ze config.conf` starts everything. The config file determines what gets started.

### 2. Plugins Define Their Own Schema

Each plugin (including BGP engine) sends its YANG schema to the Hub at startup. Third-party plugins can extend the configuration schema without modifying Ze.

### 3. Config Reader Validates Against Combined Schema

The Config Reader receives all YANG schemas from the Hub, then validates the config file against the combined schema. This enables:

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

The existing subsystem infrastructure uses this protocol (see `internal/plugin/subsystem.go`):

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

The existing 5-stage protocol is **extended**, not replaced. Schema declarations are added to Stage 1, and Stage 2 is transformed from simple config push to verify/apply cycle.

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
│  Plugin → Hub: declare cmd <command>                                    │
│  Plugin → Hub: declare done                                             │
│                                                                         │
│  ─── BARRIER: All plugins declared ───                                  │
│                                                                         │
│  STAGE 2: CONFIG (transformed to verify/apply)                          │
│  ─────────────────────────────────────────────                          │
│  Hub starts Config Reader (special process)                             │
│  Hub → ConfigReader: init schemas=[...] config_path="..."               │
│                                                                         │
│  ConfigReader parses config against YANG                                │
│                                                                         │
│  For each change:                                                       │
│    ConfigReader → Hub: verify handler=bgp.peer action=create data={...} │
│    Hub → Plugin: verify action=create data={...}                        │
│    Plugin → Hub: verify.response ok                                     │
│    Hub → ConfigReader: verify.response ok                               │
│                                                                         │
│  If all verify pass:                                                    │
│    ConfigReader → Hub: apply handler=bgp.peer action=create data={...}  │
│    Hub → Plugin: apply action=create data={...}                         │
│    Plugin → Hub: apply.response ok                                      │
│    Hub → ConfigReader: apply.response ok                                │
│                                                                         │
│  ConfigReader → Hub: complete                                           │
│  Hub → All Plugins: config done                                         │
│                                                                         │
│  ─── BARRIER: Config applied ───                                        │
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
│    - ze bgp plugin rib                                                  │
│    - /path/to/third-party-plugin                                        │
└─────────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 3. STAGE 1: DECLARATION (extended)                                       │
│    Each plugin sends:                                                   │
│      declare schema module ze-bgp namespace urn:ze:bgp                  │
│      declare schema yang "module ze-bgp { ... }"                        │
│      declare schema handler bgp                                         │
│      declare schema handler bgp.peer                                    │
│      declare cmd bgp peer list                                          │
│      declare done                                                       │
│    Hub collects all schemas                                             │
│    ─── BARRIER: All plugins declared ───                                │
└─────────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 4. STAGE 2: CONFIG (via Config Reader)                                   │
│    Hub starts Config Reader process                                     │
│    Hub sends: init with all schemas + config file path                  │
│    Config Reader parses and validates config against YANG               │
│    Config Reader sends verify/apply requests                            │
│    Hub routes to plugins, collects responses                            │
│    Config Reader sends: complete                                        │
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
declare schema module ze-bgp
declare schema namespace urn:ze:bgp
declare schema handler bgp
declare schema handler bgp.peer
declare schema handler bgp.peer-group
declare schema yang <<EOF
module ze-bgp {
  namespace "urn:ze:bgp";
  prefix bgp;
  ...
}
EOF
```

**Command declaration (EXISTING):**

```
declare cmd bgp peer list
declare cmd bgp peer show
declare encoding json
declare done
```

**Key points:**
- `declare schema` prefix matches `declare cmd` pattern
- `declare schema handler <path>` - register config handler paths (longest-prefix routing)
- `declare schema yang <<EOF...EOF` - YANG content inline via heredoc
- Hub stores YANG content, distributes to Config Reader in Stage 2

**Schema debugging (CLI):**
```bash
$ ze bgp schema show
module ze-bgp {
  namespace "urn:ze:bgp";
  prefix bgp;
  ...
}
```

Same content available via CLI for human debugging.

### Stage 2: Verify/Apply Message Flow

All messages use standard protocol patterns (`#serial command` format).

```
Hub                     Config Reader               Plugin (BGP)
 │                            │                          │
 │── config schema ze-bgp ───>│                          │
 │   handlers bgp,bgp.peer    │                          │
 │   yang <<EOF...EOF         │                          │
 │── config path ...    ─────>│                          │
 │── config done        ─────>│                          │
 │                            │                          │
 │                            │ (has YANG inline)        │
 │                            │ (parses config vs YANG)  │
 │                            │                          │
 │<─ #1 config verify ────────│                          │
 │   handler "bgp.peer"       │                          │
 │   action create            │                          │
 │   path "bgp.peer[...]"     │                          │
 │   data '{...}'             │                          │
 │                            │                          │
 │── #a config verify ───────────────────────────────>│
 │   action create path "bgp.peer[...]" data '{...}'   │
 │                                                      │
 │<── @a done ─────────────────────────────────────────│
 │                            │                          │
 │── @1 done ────────────────>│                          │
 │                            │                          │
 │     (all verify pass)      │                          │
 │                            │                          │
 │<─ #2 config apply ─────────│                          │
 │   handler "bgp.peer"       │                          │
 │   path "bgp.peer[...]"     │                          │
 │                            │                          │
 │── #b config apply ────────────────────────────────>│
 │   path "bgp.peer[...]"                               │
 │                                                      │
 │<── @b done ─────────────────────────────────────────│
 │                            │                          │
 │── @2 done ────────────────>│                          │
 │                            │                          │
 │<─ #3 config complete ──────│                          │
 │                            │                          │
 │── @3 done ────────────────>│                          │
 │                            │                          │
 │── config done ────────────────────────────────────>│
```

**Key:** All messages use text protocol - `#serial command` for requests, `@serial status` for responses.

### Building on Existing Code

**SubsystemHandler extension (hub-side):**

The existing `SubsystemHandler.completeProtocol()` in `internal/plugin/subsystem.go` needs extension:

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
4. Send `declare schema yang <<EOF...EOF` with full YANG content
5. Continue with existing `declare cmd` and `declare done`

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
| Nested | `segment.segment[key=value]` | `bgp.peer-group[name=upstream]` |

**Mapping from config syntax:**
```
bgp {                           → handler: bgp
    peer 192.0.2.1 { ... }      → handler: bgp.peer
                                  path: bgp.peer[address=192.0.2.1]
    peer-group upstream { ... } → handler: bgp.peer-group
                                  path: bgp.peer-group[name=upstream]
}
```

**Routing example:**
- Request for `bgp.peer[address=192.0.2.1]`
- Registered handlers: `bgp`, `bgp.peer`, `bgp.peer-group`
- Longest match: `bgp.peer` → routes to BGP plugin

---

## Config Verification Protocol

### Message Formats

All messages follow standard IPC protocol patterns.

**Hub → Config Reader: schemas + config path (Stage 2)**
```
config schema ze-bgp handlers bgp,bgp.peer yang <<EOF
module ze-bgp {
  namespace "urn:ze:bgp";
  ...
}
EOF
config schema ze-rib handlers rib yang <<EOF
module ze-rib {
  ...
}
EOF
config path /etc/ze/config.conf
config done
```

Hub passes YANG content inline (collected from plugins in Stage 1).

**Config Reader → Hub: verify request (text protocol)**
```
#1 config verify handler "bgp.peer" action create path "bgp.peer[address=192.0.2.1]" data '{"address":"192.0.2.1","peer-as":65002}'
```

**Hub → Plugin: verify** (text protocol, routed by handler prefix)
```
#a config verify action create path "bgp.peer[address=192.0.2.1]" data '{"address":"192.0.2.1","peer-as":65002}'
```

**Plugin → Hub: verify response**
```
@a done
```
or
```
@a error peer-as cannot equal local-as
```

**Hub → Config Reader: verify response**
```
@1 done
```
or
```
@1 error peer-as cannot equal local-as
```

**Config Reader → Hub: complete**
```
#2 config complete
```

**Key consistency points:**
- Commands use `#serial command args` format
- Responses use `@serial status [data]` format
- Status values: `done`, `error` (standard IPC values)
- All components use same text protocol (no JSON wrappers)

### Action Types

| Action | old | new | Description |
|--------|-----|-----|-------------|
| `create` | null | object | New entry |
| `modify` | object | object | Changed entry |
| `delete` | object | null | Removed entry |

### Handler Interface (Plugin Side)

Plugins implement verify and apply handlers in their main command loop:

**Verify handler:**
1. Parse command: extract action, path, and data
2. Perform semantic validation (YANG already validated types)
3. Return `@serial done` or `@serial error <message>`

**Example semantic validations:**
- peer-as cannot equal local-as for eBGP
- address must not already exist (for create action)
- referenced objects must exist

**Apply handler:**
1. Parse command: extract action, path, and data
2. Apply the validated change to internal state
3. Return `@serial done` or `@serial error <message>`

### Hub Implementation

**Startup sequence:**

1. **Fork all plugins** - Start each plugin process
2. **Stage 1: Collect declarations** - Read until each sends `declare done`
   - Collect schemas from all plugins
   - BARRIER: wait for all plugins
3. **Stage 2: Config via Config Reader**
   - Spawn Config Reader process
   - Send collected schemas + config path
   - Handle verify/apply loop:
     - Receive `config verify` from Config Reader
     - Route to plugin by longest handler prefix match
     - Forward plugin response back to Config Reader
   - On `config complete`: send `config done` to all plugins
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
list peer-group {
  key "name";
  leaf name { type string; }
  leaf peer-as { type uint32; }
}

list peer {
  key "address";
  leaf address { type inet:ip-address; }

  // Reference to peer-group - MUST exist
  leaf group {
    type leafref {
      path "../../peer-group/name";
    }
  }
}
```

### Config Example

```
bgp {
    peer-group upstream {
        peer-as 65000;
    }

    peer 192.0.2.1 {
        group upstream;      # Valid - "upstream" exists
    }
    peer 192.0.2.2 {
        group nonexistent;   # INVALID - leafref target not found
    }
}
```

### leafref Use Cases

| Reference | From | To |
|-----------|------|-----|
| `group` | peer | peer-group list |
| `route-map-in` | peer | route-map list |
| `prefix-list` | route-map | prefix-list |
| `community-list` | route-map | community-list |

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

This requires all YANG modules to be loaded together in the Config Reader.

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
ze-bgp: bgp, bgp.peer, bgp.peer-group
ze-rib: rib

# Show specific schema
$ ze system schema show ze-bgp
module ze-bgp {
  namespace "urn:ze:bgp";
  ...
}

# Show example messages
$ ze system schema example verify
$ ze system schema example apply
```

**Note:** All commands use text protocol, human-readable output.

---

## Config Reader

### Responsibilities

1. Receive YANG schemas from Hub (inline via `config schema`)
2. Parse config file using existing tokenizer
3. Validate against combined YANG schema
4. Detect changes on reload (diff current vs new)
5. Send verify/apply requests to Hub

### Spawning

Config Reader is spawned **after** Stage 1 completes for all plugins:

1. Hub collects all schema declarations from plugins (Stage 1)
2. Hub spawns Config Reader process
3. Hub sends schemas + config path to Config Reader
4. Config Reader validates and sends verify/apply requests
5. After `config complete`, Hub sends `config done` to all plugins

Config Reader does NOT participate in Stage 1 - it receives schemas, doesn't declare them.

### Command Handling

Hub handles `config reload` and `config validate` commands directly:

| Command | Handler |
|---------|---------|
| `config reload` | Hub receives, delegates to Config Reader |
| `config validate` | Hub receives, delegates to Config Reader |

These are Hub commands, not Config Reader commands. Config Reader is an internal process, not a command-declaring plugin.

### Message Protocol

See "Config Verification Protocol" section above for full message formats.

### Hot Reload

On SIGHUP or API command:

1. Hub sends `#serial config reload` to Config Reader
2. Config Reader re-parses config file
3. Config Reader diffs against current state
4. Config Reader sends verify/apply for changes only
5. Config Reader responds `@serial done`

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
│  │ plugins: map    │  │ modules: map    │  │ configPath      │         │
│  │   name → Handler│  │   name → Schema │  │ currentConfig   │         │
│  │                 │  │ handlers: map   │  │                 │         │
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
│  BGP   │    │  RIB   │    │ Config │    │ Third-Party │
│ Plugin │    │ Plugin │    │ Reader │    │   Plugin    │
└────────┘    └────────┘    └────────┘    └─────────────┘
```

### Data Structures

**SchemaRegistry** stores schema information indexed by module:

| Field | Description |
|-------|-------------|
| `modules` | Map of module name → Schema (module, namespace, yang, handlers, plugin) |
| `handlerIndex` | Map of handler path → plugin name (for routing) |

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
  "bgp"            → bgp plugin
  "bgp.peer"       → bgp plugin
  "bgp.peer-group" → bgp plugin
  "rib"            → rib plugin

FindHandler("bgp.peer[address=192.0.2.1]"):
  1. Try exact match "bgp.peer[address=192.0.2.1]" → not found
  2. Strip key: "bgp.peer" → FOUND → return "bgp" plugin

FindHandler("bgp.peer-group[name=upstream].timers"):
  1. Try "bgp.peer-group[name=upstream].timers" → not found
  2. Strip segment: "bgp.peer-group[name=upstream]" → not found
  3. Strip key: "bgp.peer-group" → FOUND → return "bgp" plugin

FindHandler("unknown.path"):
  1. No matches found → return error UNKNOWN_HANDLER
```

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
    ┌─────────┐
    │   Hub   │ ──── #r1 config reload ────▶ ┌─────────────┐
    └─────────┘                               │Config Reader│
         ▲                                    └──────┬──────┘
         │                                           │
         │                              ┌────────────┘
         │                              │
         │                    1. Re-read config file
         │                    2. Parse new config
         │                    3. Diff against current:
         │                         delete: peer 192.0.2.2
         │                         create: peer 192.0.2.3
         │                    4. Verify all changes
         │                    5. Apply all changes
         │                              │
         │ ◀──── @r1 done ──────────────┘
```

### YANG Validation (Config Reader)

```
┌─────────────────────────────────────────────────────────────┐
│                      Config Reader                          │
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
│                      │  4. Leafref validation:     │       │
│                      │     → call plugin CLI       │       │
│                      │  5. Queue verify request    │       │
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
    peer 192.0.2.1 {
        peer-as 65002;
    }
}
```

### Startup Flow

1. Hub forks `/opt/acme/monitor-plugin`
2. Plugin sends `declare schema` messages with YANG content
3. Hub collects schema alongside ze-bgp schema
4. Hub spawns Config Reader, sends all schemas
5. Config Reader validates config, sends `config verify` for `acme-monitor` → routed to ACME plugin
6. Config Reader sends `config verify` for `bgp.peer` → routed to BGP engine
7. All pass → Config Reader sends `config apply` to both
8. Config Reader sends `config complete`, Hub sends `config done` to all plugins

---

## Migration Path

### Dependency Diagram

```
Phase 0: Serial Prefix (prerequisite cleanup)
    │
    ▼
Phase 1: Schema Infrastructure ◄───── YANG Modules (parallel)
    │                                       │
    ▼                                       │
Phase 2: Config Reader                      │
    │                                       │
    ▼                                       ▼
Phase 3: YANG Integration ◄─────────── (needs YANG modules)
    │
    ▼
Phase 4: Verify/Apply Protocol
    │
    ▼
Phase 5: Third-Party Support
```

### Phase 0: Foundation ✅ COMPLETE

**Already implemented** (commit `19b6564`):

- ✅ Forked subsystem processes (`cmd/ze-subsystem/`)
- ✅ 5-stage protocol via pipes (`internal/plugin/subsystem.go`)
- ✅ Bidirectional communication (`callEngine()` in subsystem)
- ✅ `SubsystemHandler` and `SubsystemManager`
- ✅ Command routing to forked processes
- ✅ Dynamic peer management (`bgp peer add/remove`)

### Phase 1: Schema Infrastructure

**Spec:** [`docs/plan/spec-hub-phase1-schema-infrastructure.md`](../plan/spec-hub-phase1-schema-infrastructure.md)

- Add `declare schema` to protocol (extend Stage 1)
- Implement SchemaRegistry in Hub (extend SubsystemManager)
- Add `system schema *` CLI commands for discovery
- Define schema message format (`declare schema module <name>`, `declare schema yang <<EOF...EOF`)

### Phase 2: Config Reader Process

**Spec:** [`docs/plan/spec-hub-phase2-config-reader.md`](../plan/spec-hub-phase2-config-reader.md)

- Create `ze-config-reader` binary (follows same pattern as `ze-subsystem`)
- Receive schemas via `config schema` messages
- Config Reader receives schemas + config file path
- Keep existing config parser, add YANG-aware layer

### Phase 3: YANG Integration

**Spec:** [`docs/plan/spec-hub-phase3-yang-integration.md`](../plan/spec-hub-phase3-yang-integration.md)

- Integrate libyang (or goyang + custom validation)
- Define `ze-bgp.yang` for BGP config
- Implement leafref validation
- Config Reader validates against combined YANG schema

### Phase 4: Verify/Apply Protocol

**Spec:** [`docs/plan/spec-hub-phase4-verify-apply.md`](../plan/spec-hub-phase4-verify-apply.md)

- Add `config verify`, `config apply` message handling
- Implement handler routing by longest prefix match
- Update subsystems to handle verify/apply (similar to existing command handling)
- Transaction semantics (all verify pass → apply)

### Phase 5: Third-Party Support

**Spec:** [`docs/plan/spec-hub-phase5-third-party.md`](../plan/spec-hub-phase5-third-party.md)

- Document plugin developer guide
- Publish schema protocol specification
- Example third-party plugin with custom YANG

### YANG Modules (Parallel)

**Spec:** [`docs/plan/spec-hub-yang-modules.md`](../plan/spec-hub-yang-modules.md)

- Define `ze-types.yang` - common types
- Define `ze-bgp.yang` - BGP configuration schema
- Define `ze-plugin.yang` - plugin configuration
- Map existing config syntax to YANG

---

## Open Questions

| # | Question | Options |
|---|----------|---------|
| 1 | Config Reader binary | `ze config reader` vs built into hub |
| 2 | YANG validation library | libyang (C) vs goyang + custom (Go) |
| 3 | Schema format in messages | Raw YANG text vs file reference |
| 4 | Transaction rollback | Auto-rollback on apply failure vs partial state |

---

## Related Documents

### Architecture

- [IPC Protocol](api/ipc_protocol.md) - Wire format, event structure
- [Process Protocol](api/process-protocol.md) - Plugin lifecycle, 5-stage startup
- [YANG Config Design](config/yang-config-design.md) - VyOS-inspired YANG design
- [VyOS Research](config/vyos-research.md) - VyOS configuration architecture

### Implementation

- [Unified Subsystem Protocol](../plan/done/149-unified-subsystem-protocol.md) - Completed spec for subsystem infrastructure
- [Pipe-Based Subsystems](../plan/spec-pipe-subsystems.md) - Spec for forked process subsystems

### Source Code

- `cmd/ze-subsystem/main.go` - Forked subsystem binary
- `internal/plugin/subsystem.go` - SubsystemHandler/Manager
- `internal/plugin/process.go` - Process pipe communication
- `internal/plugin/registration.go` - 5-stage protocol parsing

---

**Last Updated:** 2026-01-24
