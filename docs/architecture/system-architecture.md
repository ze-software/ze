# Ze System Architecture

**Status:** Implemented (Hub Mode)
**Last Updated:** 2026-01-30
**Purpose:** Describes Ze's hub/orchestrator mode with separate plugin processes

---

## Overview

Ze supports two operating modes:

| Mode | Trigger | Description |
|------|---------|-------------|
| **In-process** | `bgp { }` block in config | BGP daemon with in-process plugins (simpler, default) |
| **Hub mode** | `plugin { external ... }` block | Hub orchestrates separate plugin processes (this doc) |

**This document describes Hub mode.**

In hub mode, Ze runs as a **hub process** (`ze`) that orchestrates separate **plugin processes** communicating via pipes. This architecture enables:
<!-- source: internal/component/plugin/server/ -- plugin server orchestration -->

- Language freedom (plugins can be Go, Python, Rust, etc.)
- Crash isolation (BGP crash doesn't kill RIB)
- Independent development and testing
- Third-party extensibility

```
                           ┌─────────────────────┐
                           │    ze config.conf   │
                           │      (hub)          │
                           └──────────┬──────────┘
                                      │
           ┌──────────────────────────┼──────────────────────────┐
           │                          │                          │
           ▼                          ▼                          ▼
    ┌─────────────┐           ┌─────────────┐           ┌─────────────┐
    │   ze bgp    │           │   ze rib    │           │  ze gr      │
    │  (process)  │           │  (process)  │           │  (process)  │
    └─────────────┘           └─────────────┘           └─────────────┘
```

---

## Running Ze

### Basic Usage

```bash
# Start Ze with a config file
ze config.conf

# The hub process:
# 1. Parses the config file
# 2. Forks plugin processes (ze bgp, ze rib, etc.)
# 3. Routes config to each plugin
# 4. Coordinates startup via 5-stage protocol
# 5. Routes commands and events between plugins
```

### Process Hierarchy

When running, you'll see these processes:

```
$ ps aux | grep ze
user  1234  ze config.conf          # Hub process
user  1235  ze bgp                  # BGP protocol handler
user  1236  ze rib                  # Adj-RIB tracking
user  1237  ze gr                   # Graceful Restart
user  1238  /opt/acme/plugin        # Third-party plugin
```

---

## Configuration File

The hub parses the entire config file (like VyOS):
1. Parse config syntax
2. Validate against combined YANG schema (all plugins)
3. Convert to internal map-of-maps structure
4. Route JSON subtrees to plugins based on handler registration

The config file has three sections, parsed in order:

### Section 1: Environment

Global settings applied before forking processes.

```
env {
    log-level debug;
    working-dir /var/run/ze;
}
```

### Section 2: Plugin Declarations

Which processes to fork. Uses YANG schema from `ze-plugin-conf.yang`.

```
plugin {
    # Built-in plugins (shipped with ze)
    external bgp {
        run "ze bgp";
    }
    external rib {
        run "ze rib";
    }
    external gr {
        run "ze gr";
    }

    # Third-party plugins
    external acme {
        run "/opt/acme/monitor-plugin";
        respawn true;           # Restart if crashes
        timeout 60;             # Startup timeout
    }
}
```

### Section 3: Plugin Configuration

Configuration for each plugin. Routed by the hub to the appropriate process.

```
# Routed to ze bgp (handles "bgp" container per ze-bgp-conf.yang)
bgp {
    local-as 65001;
    router-id 1.1.1.1;

    peer transit-a {
        remote {
            ip 192.0.2.1;
            as 65002;
        }
        passive;

        capability {
            # GR plugin handles this path (augments BGP schema)
            graceful-restart {
                enabled true;
                restart-time 120;
            }
        }
    }

    peer-group upstream {
        peer-as 65000;
    }
}

# Routed to ze rib (handles "rib" container per ze-rib.yang)
rib {
    # RIB-specific settings
}

# Routed to /opt/acme/plugin (handles "acme" container per acme.yang)
acme {
    endpoint "https://monitor.example.com";
    interval 30;
}
```

---

## Plugin Types

### Built-in Plugins

Shipped with Ze, same binary:

| Plugin | Binary | Purpose |
|--------|--------|---------|
| `bgp` | `ze bgp` | BGP protocol, sessions, FSM, peer-to-peer routing |
| `rib` | `ze rib` | Adj-RIB-Out tracking, sent-route replay on reconnect |
| `adj-rib-in` | `ze adj-rib-in` | Adj-RIB-In storage (raw hex), received-route replay |
| `gr` | `ze gr` | Graceful Restart capability injection |

### Third-Party Plugins

Any executable that speaks the plugin protocol:

```
plugin {
    external my-plugin {
        run "/path/to/plugin";
        respawn true;
    }
}

my-plugin {
    # Config routed here based on plugin's YANG schema
}
```

### Augmenting Plugins

Some plugins (like GR) don't have their own root config block. They **augment** another plugin's schema:

```yang
// ze-gr.yang
augment "/bgp:bgp/bgp:peer/bgp:capability" {
    container graceful-restart {
        leaf enabled { type boolean; }
        leaf restart-time { type uint16; }
    }
}
```

Config for augmenting plugins appears nested within the augmented schema:

```
bgp {
    peer transit-a {
        remote {
            ip 192.0.2.1;
            as 65002;
        }
        capability {
            graceful-restart {      # Handled by ze gr, not ze bgp
                enabled true;
            }
        }
    }
}
```

**How augmenting plugins work:**

GR registers handler for `bgp.peer.capability.graceful-restart`. Hub sends just that JSON subtree to GR:

```json
{"enabled": true, "restart-time": 120}
```

GR then uses the capability API to inject the capability:

```
# GR sends command:
capability hex 64 0078 peer 192.168.1.1
               │    │        └─ target peer
               │    └─ restart-time (120) in 12-bit format
               └─ capability code 64 (graceful restart, RFC 4724)
```

BGP stores registered capabilities and includes them in OPEN messages to peers.

### GR Plugin Coordination

GR plugin coordinates with BGP and RIB via commands and events:

```
ze gr                         ze (hub)                      ze bgp
   │                             │                             │
   │◄── config (JSON subtree) ───│                             │
   │                             │                             │
   │── capability hex 64 ... ───►│─── (routes to BGP) ────────►│
   │   peer 192.168.1.1          │      (BGP stores for OPEN)  │
   │                             │                             │
   │── subscribe bgp.peer.* ────►│                             │
   │                             │                             │
   │                             │◄── event bgp.peer.restart ──│
   │◄── event bgp.peer.restart ──│                             │
   │                             │                             │
   │── rib defer peer X ────────►│─────────────────────────────│──► ze rib
   │                             │                             │
```

**Key points:**
- GR uses `capability hex <code> <value> peer <addr>` format
- Hub routes capability commands to BGP
- GR subscribes to peer events (restart, up, down)
- GR coordinates with RIB for route deferral during restart

---

## CLI Commands

### Command Routing

CLI commands are routed to plugins by prefix:

```bash
# Routed to ze bgp
ze bgp peer list
ze bgp peer upstream1 show
ze bgp peer upstream1 update ...

# Routed to ze rib
ze rib show
ze rib replay upstream1

# Routed to hub (system commands)
ze system schema list
ze system process list
ze config reload
```

### How CLI Works

```
┌─────────────────────────────────────────────────────────────────────┐
│ $ ze bgp peer list                                                  │
│                                                                     │
│  1. CLI connects to daemon via SSH (127.0.0.1:2222)                 │
│  2. CLI sends: bgp peer list                                        │
│  3. Daemon looks up "bgp" in handler map → ze bgp process           │
│  4. Daemon forwards command via stdin to ze bgp                     │
│  5. ze bgp executes, sends response via stdout                      │
│  6. Daemon returns response to CLI via SSH session                  │
│  7. CLI displays result                                             │
└─────────────────────────────────────────────────────────────────────┘
```

**Same binary, two modes:**
- `ze config.conf` → starts as daemon, listens on SSH port
- `ze bgp peer list` → connects to running daemon as SSH client

SSH target configurable via env vars `ze_ssh_host` and `ze_ssh_port`

### System Commands

Commands handled directly by the hub:

```bash
# List registered schemas
ze system schema list
ze-bgp: bgp, bgp.peer, bgp.peer-group
ze-rib: rib
ze-gr: bgp.peer.capability.graceful-restart

# Show a schema
ze system schema show ze-bgp

# List running processes
ze system process list
bgp: pid=1235 state=ready
rib: pid=1236 state=ready
gr:  pid=1237 state=ready

# Reload configuration
ze config reload
```

---

## Event Flow

Plugins communicate via events routed through the hub.

### Event Subscription

Plugins subscribe to events during startup:

```
# ze rib subscribes to BGP events
subscribe bgp.event.*
```

### Event Publishing

When something happens, plugins publish events:

```
# ze bgp publishes peer state change
event bgp.peer.up peer=192.0.2.1 asn=65002
```

### Event Routing

Hub routes events to subscribers:

```
ze bgp                        ze (hub)                      ze rib
   │                             │                             │
   │ (peer establishes)          │                             │
   │── event bgp.peer.up ───────>│                             │
   │   peer=192.0.2.1            │                             │
   │                             │── event bgp.peer.up ───────>│
   │                             │   peer=192.0.2.1            │
   │                             │                             │
   │ (UPDATE received)           │                             │
   │── event bgp.update ────────>│                             │
   │   {...}                     │── event bgp.update ────────>│
   │                             │   {...}                     │
```

---

## Startup Sequence

### 10-Step Protocol

```
1. Hub parses env { }              → Set global settings (api-socket, log-level, etc.)
2. Hub parses plugin { } block     → Build process list from ze-plugin-conf.yang
3. Hub forks each plugin           → ze bgp, ze rib, ze gr, third-party, ...
4. Each plugin: Stage 1            → Declare YANG module + handlers
5. Hub registers schemas           → Build handler routing table (SchemaRegistry)
6. Hub parses remaining config     → Full parse, validate against combined YANG
7. Hub converts to JSON            → Map-of-maps structure
8. Hub routes JSON to plugins      → Each plugin gets its subtree (Stage 2)
9. Plugins: Stage 3-4              → Capability declarations, registry sharing
10. Plugins: Stage 5               → Ready, start operating
```

After startup, GR and other plugins use commands to configure BGP:
- `capability hex <code> <value> peer <addr>` → inject capabilities
- Plugins subscribe to events for runtime coordination

### 5-Stage Protocol (Per Plugin)

Each plugin follows this protocol with the hub:

| Stage | Direction | Content |
|-------|-----------|---------|
| 1 | Plugin → Hub | `declare schema yang <module>`, `declare schema handler <path>`, `declare priority <num>`, `declare cmd <name>`, `declare done` |
| 2 | Hub → Plugin | Initial commit: `config verify` → plugin queries live/edit → `config apply` → `config done` |
| 3 | Plugin → Hub | `capability hex ...`, `capability done` |
| 4 | Hub → Plugin | `registry cmd ...`, `registry done` |
| 5 | Plugin → Hub | `ready` |
<!-- source: internal/component/plugin/registration.go -- 5-stage protocol parsing -->
<!-- source: internal/component/plugin/startup_coordinator.go -- startup coordination -->

**Priority:** Determines verify/apply order. Lower = first. Example: BGP=100, RIB=200, GR=300.

---

## Config Notification to Plugins

### Pull Model (Hub Never Pushes)

**Hub notifies plugins of config changes, plugins query for config data.**

On commit (startup, SIGHUP, or `ze config commit`), hub sends `config verify` / `config apply` notifications. Plugins query hub for config.

Based on handler registration, plugins query for their JSON config:

| Handler | Plugin receives |
|---------|-----------------|
| `bgp` (root) | Entire `bgp { }` block as JSON |
| `bgp.peer.capability.graceful-restart` (sub-root) | Just that subtree as JSON |

### On-Demand Query

Plugins query hub for specific config paths using text protocol:

```
# Query live (running) config:
#1 query config live path "bgp.peer[address=192.0.2.1]"
@1 done data '{"address": "192.0.2.1", "peer-as": 65002, "timers": {...}}'

# Query edit (candidate) config:
#2 query config edit path "bgp.peer[address=192.0.2.1]"
@2 done data '{"address": "192.0.2.1", "peer-as": 65003, "timers": {...}}'
```

Hub stores config as map-of-maps internally, provides JSON in `data` field.

---

## Live/Edit Configuration (VyOS-style)

Hub maintains two configuration states:

| State | Purpose |
|-------|---------|
| **Live** | Running configuration (what plugins are currently using) |
| **Edit** | Candidate configuration (being modified, not yet applied) |

### Commit Workflow

```
1. User modifies edit config (via CLI or file)
2. User requests commit
3. For each plugin (by priority, lower first):
   a. Hub sends: config verify
   b. Plugin queries hub for live and edit config (its section)
   c. Plugin computes diff using shared library
   d. Plugin validates changes are acceptable
   e. Plugin responds: done or error
4. If all plugins verify ok:
   a. For each plugin (by priority):
      - Hub sends: config apply
      - Plugin applies changes
      - Plugin responds: done
   b. Edit becomes new live
5. If any verify fails:
   a. Hub aborts commit
   b. Edit unchanged, live unchanged
```

**Priority examples:** BGP=100 (first), RIB=200, GR=300 (last).

### Plugin Diff Responsibility

**Hub provides:** Raw config states (live and edit) on request.

**Plugin is responsible for:**
1. Query the config sections it needs
2. Compute diff using shared library code
3. Validate and apply changes

### Shared Diff Library

Plugins use shared library code (not reimplemented per plugin) to compute differences:

```
Location: internal/component/config/diff/

Usage (Go):
  // Send query, receive JSON in data field
  live := sendQuery("query config live path \"bgp.peer\"")
  edit := sendQuery("query config edit path \"bgp.peer\"")
  changes := diff.Compare(live, edit)
  // changes = []Change{{Action: "create", Path: "bgp.peer[addr=X]", Data: {...}}, ...}
```

This library is part of the Ze codebase, available to all plugins.

---

## YANG Schema

### What YANG Provides

- Type validation (ranges, patterns, enums)
- Cross-reference validation (leafref)
- Schema-driven config routing

### Schema Registration

During Stage 1, plugins declare their YANG schema:

```
declare schema yang ze-bgp
declare schema handler bgp
declare schema handler bgp.peer
declare done
```

### Existing YANG Modules

| Module | Location | Defines |
|--------|----------|---------|
| `ze-types` | `yang/ze-types.yang` | Common types (asn, ip-address, etc.) |
| `ze-bgp-conf` | `internal/component/bgp/schema/ze-bgp-conf.yang` | `container bgp` with peers, families |
| `ze-plugin-conf` | `internal/component/plugin/schema/` | `container plugin` for process declarations |
<!-- source: internal/component/bgp/schema/ -- BGP YANG schemas -->
<!-- source: internal/component/plugin/schema/ -- plugin YANG schemas -->
| `ze-rib` | `internal/component/plugin/rib/schema/ze-rib.yang` | Augments `ze-bgp-conf` with `container rib` |
| `ze-graceful-restart` | `internal/component/plugin/gr/schema/ze-graceful-restart.yang` | Augments `ze-bgp-conf` for graceful-restart |
| `ze-hostname` | `internal/component/plugin/hostname/schema/ze-hostname.yang` | Augments `ze-bgp-conf` for FQDN capability |

**Note:** Plugin YANG schemas augment `ze-bgp-conf` to extend the configuration tree. Each plugin owns its YANG in a `schema/` subdirectory.

### YANG Augment Merging

Plugins can augment other plugins' YANG schemas. Hub merges all YANG modules into a single consistent view:

1. Each plugin declares its YANG module in Stage 1
2. Hub collects all modules
3. Hub merges augments into base modules
4. Hub validates combined schema is consistent

**Conflict handling:** If two plugins define conflicting augments (same path, different definitions), hub refuses to start. The plugins are incompatible.

---

## Package Structure

After refactoring, the code will be organized as:

```
internal/
├── hub/                    # Hub/orchestrator (protocol-agnostic)
│   ├── hub.go              # Core hub
│   ├── process.go          # Fork and manage child processes
│   ├── router.go           # Route commands/events
│   └── config.go           # Parse env and plugin blocks
│
├── plugin/
│   ├── bgp/                # BGP plugin (moved from internal/bgp/)
│   │   ├── message/        # BGP wire format
│   │   ├── attribute/      # Path attributes
│   │   ├── nlri/           # NLRI types
│   │   ├── capability/     # BGP capabilities
│   │   ├── fsm/            # State machine
│   │   ├── rib/            # Peer-to-peer routing (moved from internal/rib/)
│   │   └── reactor/        # BGP-specific reactor (moved from internal/reactor/)
│   │
│   ├── rib/                # Adj-RIB tracking plugin
│   │   ├── rib.go
│   │   └── storage/
│   │
│   ├── gr/                 # Graceful Restart plugin
│   │
│   ├── server.go           # Plugin server (reused by hub)
│   ├── handler.go          # Command/event dispatch
│   ├── schema.go           # SchemaRegistry
│   └── subsystem.go        # 5-stage protocol
│
├── yang/                   # YANG loader and validator
│   ├── loader.go
│   └── validator.go
│
└── config/                 # Config parsing (shared)
```

---

## Signals

| Signal | Handler | Action |
|--------|---------|--------|
| `SIGHUP` | Hub | Reload configuration |
| `SIGTERM` | Hub | Graceful shutdown (notify all plugins) |
| `SIGINT` | Hub | Graceful shutdown |
| `SIGUSR1` | Hub | Dump state/metrics |

### Config Reload (SIGHUP)

```
1. Hub receives SIGHUP
2. Hub re-parses config file
3. Hub diffs current vs new config
4. Hub sends verify/apply for changes to affected plugins
5. Plugins apply changes (add/remove peers, etc.)
```

---

## Benefits of This Architecture

| Benefit | Description |
|---------|-------------|
| **Crash Isolation** | BGP crash doesn't affect RIB; processes restart independently |
| **Language Freedom** | Plugins can be written in any language |
| **Independent Development** | Test and develop plugins separately |
| **Third-Party Extensibility** | Anyone can write plugins |
| **Resource Limits** | Each process can have memory/CPU limits |
| **Debugging** | Attach debugger to single process |
| **Hot Reload** | Replace plugin binary without full restart (future) |

---

## Security Model

### Privilege Dropping (ExaBGP Pattern)

Ze follows the standard Unix daemon privilege separation model:

1. Start as root (or with `CAP_NET_BIND_SERVICE`) to bind port 179
2. Bind the BGP listening socket
3. Drop privileges to the configured user/group
4. All subsequent work -- including plugin spawning -- runs as the unprivileged user

The target user/group is configured via environment variables:

| Variable | Underscore form | Purpose |
|----------|-----------------|---------|
| `ze.user` | `ze_user` | User to switch to after port binding |
| `ze.group` | `ze_group` | Group to switch to (default: primary group of user) |

When `ze.user` is not set, no privilege dropping occurs.

Implementation: `internal/core/privilege/` -- calls `setgid` then `setuid` after `reactor.Start()` binds port 179.
<!-- source: internal/core/privilege/ -- privilege dropping -->

### Plugin TLS Transport

External plugins connect back to the engine via TLS. The engine binds TLS listeners (configured via `plugin { hub { server <name> { host ...; port ...; secret ...; } } }`), forks child processes with `ZE_PLUGIN_HUB_HOST`/`ZE_PLUGIN_HUB_PORT`/`ZE_PLUGIN_HUB_TOKEN` env vars, and waits for authenticated connect-back. Each plugin uses a single bidirectional TLS connection with MuxConn for concurrent RPCs.
<!-- source: internal/component/hub/ -- hub TLS listener -->
<!-- source: pkg/plugin/rpc/ -- MuxConn for concurrent RPCs -->

### Plugin Process Isolation

Each external plugin runs in its own process group (`Setpgid`) for clean signal handling and inherits the daemon's (already-dropped) uid/gid. All plugins run as the same unprivileged user.
<!-- source: internal/component/plugin/process/ -- process isolation -->

---

## Example Session

```bash
# Start Ze
$ ze /etc/ze/config.conf
[hub] Starting with config /etc/ze/config.conf
[hub] Forking ze bgp (pid 1235)
[hub] Forking ze rib (pid 1236)
[hub] Forking ze gr (pid 1237)
[hub] All plugins ready

# Check status
$ ze system process list
bgp: pid=1235 state=ready uptime=5m
rib: pid=1236 state=ready uptime=5m
gr:  pid=1237 state=ready uptime=5m

# List BGP peers
$ ze bgp peer list
192.0.2.1  AS65002  Established  5m
192.0.2.2  AS65003  Active       -

# Show routes
$ ze rib show
Prefix          Next-Hop      AS-Path        Peer
10.0.0.0/24     192.0.2.1     65002          192.0.2.1
10.0.1.0/24     192.0.2.1     65002 65004    192.0.2.1

# Reload config
$ ze config reload
[hub] Reloading configuration
[hub] Added peer 192.0.2.3
[hub] Reload complete

# Graceful shutdown
$ kill -TERM $(pgrep -f "ze config.conf")
[hub] Received SIGTERM, shutting down
[hub] Notifying plugins...
[bgp] Sending NOTIFICATION to peers
[hub] All plugins stopped
```

---

## Related Documents

- [Core Design](core-design.md) - Canonical architecture (describes in-process mode)
- [Hub Architecture](hub-architecture.md) - Hub mode internal design details
- [Process Protocol](api/process-protocol.md) - 5-stage protocol specification
- [YANG Config Design](config/yang-config-design.md) - Schema design
- [Spec: Config Dispatch](../../plan/spec-config-dispatch.md) - Mode selection by config content

---

**Last Updated:** 2026-01-30
