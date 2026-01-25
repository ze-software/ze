# Spec: reactor-service-separation

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/system-architecture.md` - end-user view of target architecture
4. `docs/architecture/hub-architecture.md` - internal design details
5. `internal/plugin/subsystem.go` - existing 5-stage protocol
6. `internal/plugin/rib/rib.go` - RIB plugin (already process-based)

## Task

Refactor the architecture so that:
1. `ze` is a hub/orchestrator that reads config and forks processes
2. `ze bgp` is a separate process handling BGP protocol (includes internal/rib/ for peer-to-peer)
3. `ze rib` is a separate process for Adj-RIB tracking (from internal/plugin/rib/)
4. All processes communicate via pipes using the existing 5-stage protocol

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/system-architecture.md` - end-user view of target system
- [ ] `docs/architecture/hub-architecture.md` - internal design details
- [ ] `docs/architecture/api/process-protocol.md` - 5-stage protocol
- [ ] `docs/architecture/config/yang-config-design.md` - YANG config design

### YANG Infrastructure (already exists)
- [ ] `internal/yang/loader.go` - YANG module loader
- [ ] `internal/yang/validator.go` - Config validation against YANG
- [ ] `internal/plugin/schema.go` - SchemaRegistry, handler→plugin mapping
- [ ] `yang/ze-bgp.yang` - BGP YANG module (defines `container bgp`)
- [ ] `yang/ze-plugin.yang` - Plugin YANG module (defines `container plugin`)

### Source Files
- [ ] `internal/plugin/subsystem.go` - SubsystemHandler, 5-stage protocol
- [ ] `internal/plugin/rib/rib.go` - RIB plugin (already process-based)
- [ ] `internal/reactor/reactor.go` - current BGP+reactor coupling

**Key insights:**
- YANG infrastructure already exists (loader, validator, schema registry)
- `ze-plugin.yang` already defines plugin declaration syntax
- RIB plugin already implements process-based architecture via pipes
- 5-stage protocol already exists for process coordination
- SubsystemManager already handles forked process management
- Current reactor mixes hub concerns with BGP protocol

## Target Architecture

### Config File Structure

Config is parsed in order with three sections. Section 2 syntax matches existing `ze-plugin.yang`:

```
# ─────────────────────────────────────────────────────────
# SECTION 1: Environment (global settings)
# ─────────────────────────────────────────────────────────
env {
    log-level debug;
    working-dir /var/run/ze;
}

# ─────────────────────────────────────────────────────────
# SECTION 2: Plugin declarations (matches ze-plugin.yang)
# ─────────────────────────────────────────────────────────
plugin {
    external bgp {
        run "ze bgp";
    }

    external rib {
        run "ze rib";
    }

    external gr {
        run "ze gr";
    }

    external acme {
        run "/opt/acme/plugin";
        respawn true;           # Optional: restart if crashes
    }
}

# ─────────────────────────────────────────────────────────
# SECTION 3: Plugin configuration (config for each plugin)
# ─────────────────────────────────────────────────────────
bgp {
    local-as 65001;
    router-id 1.1.1.1;

    peer 192.0.2.1 {
        peer-as 65002;
        passive;

        capability {
            # GR plugin handles this path via YANG augment
            graceful-restart {
                enabled true;
                restart-time 120;
            }
        }
    }
}

rib {
    # RIB-specific configuration (if any)
}

acme {
    endpoint "https://monitor.example.com";
}
```

### Startup Sequence (Option B)

```
1. Hub parses env { }              → set global settings
2. Hub parses plugin { } block     → build list of processes to fork (from ze-plugin.yang)
3. Hub forks each plugin process   → (ze bgp, ze rib, ze gr, acme, ...)
4. Each process: Stage 1           → declare YANG module + handlers
5. Hub registers schemas           → SchemaRegistry.Register() for each process
6. Hub parses remaining blocks     → bgp { }, rib { }, acme { }
7. Hub routes config via schema    → SchemaRegistry.FindHandler() → correct process (Stage 2)
   - Root blocks (bgp, rib) → direct match
   - Nested paths (bgp.peer.capability.graceful-restart) → longest prefix match → ze gr
8. Processes: Stage 3              → capability declarations
9. Hub: Stage 4                    → registry sharing
10. Processes: Stage 5             → ready
```

Uses existing `internal/plugin/schema.go` SchemaRegistry for routing.

### All Plugins Are Equal Peers

There are no "built-in" vs "third-party" plugins. All use the same protocol:

| Plugin | Binary | Purpose | Config Scope |
|--------|--------|---------|--------------|
| `bgp` | `ze bgp` | BGP protocol, sessions, peer-to-peer routing | `bgp { }` (root) |
| `rib` | `ze rib` | Adj-RIB tracking, route replay | `rib { }` (root) |
| `gr` | `ze gr` | Graceful Restart capability injection | `bgp.*.capability.graceful-restart` (augments BGP) |
| `acme` | `/opt/acme/plugin` | Third-party monitoring | `acme { }` (root) |

### How Hub Routes Config to Plugins

During Stage 1, each plugin declares its YANG module and handlers:

```
# ze bgp sends:
declare schema yang ze-bgp
declare schema handler bgp
declare done

# ze rib sends:
declare schema yang ze-rib
declare schema handler rib
declare done

# ze gr sends (augments BGP, no root handler):
declare schema yang ze-gr
declare schema handler bgp.peer.capability.graceful-restart
declare schema handler bgp.peer-group.capability.graceful-restart
declare done

# Third-party plugin sends:
declare schema yang acme-monitor
declare schema handler acme
declare done
```

Hub uses existing `SchemaRegistry` (from `internal/plugin/schema.go`) to register each schema.

| Handler | Process | YANG Module |
|---------|---------|-------------|
| `bgp` | ze bgp | ze-bgp |
| `bgp.peer.capability.graceful-restart` | ze gr | ze-gr |
| `rib` | ze rib | ze-rib |
| `acme` | /opt/acme/plugin | acme-monitor |

**Routing with augments:** When parsing `bgp.peer[x].capability.graceful-restart`, hub uses longest-prefix match:
- `FindHandler("bgp.peer.capability.graceful-restart")` → ze gr (exact match)
- `FindHandler("bgp.peer.timers")` → ze bgp (prefix match on "bgp")

### Process Topology

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           ze (hub process)                               │
│                                                                         │
│  Responsibilities:                                                      │
│  ├── Parse config file (env, plugin declarations, plugin configs)      │
│  ├── Fork child processes based on plugin { } blocks                   │
│  ├── Deliver config sections to processes (Stage 2)                    │
│  │   ├── Root blocks (bgp, rib) → direct to owning process             │
│  │   └── Nested paths → longest prefix match (e.g., GR augments BGP)   │
│  ├── Route commands between processes                                   │
│  ├── Route events between processes (pub/sub)                          │
│  ├── Route capabilities to BGP (Stage 3)                               │
│  ├── Handle signals (SIGHUP → config reload)                           │
│  └── CLI command routing                                                │
│                                                                         │
│  Does NOT contain:                                                      │
│  ├── BGP protocol code                                                  │
│  ├── TCP listeners/connections                                         │
│  ├── Route storage                                                      │
│  └── Any protocol-specific logic                                        │
│                                                                         │
└────────────────────────────│────────────────────────────────────────────┘
                             │ stdin/stdout pipes
       ┌─────────────────────┼─────────────────────┬─────────────────┐
       ▼                     ▼                     ▼                 ▼
┌─────────────┐       ┌─────────────┐       ┌─────────────┐   ┌───────────┐
│   ze bgp    │       │   ze rib    │       │    ze gr    │   │  third-   │
│  (process)  │       │  (process)  │       │  (process)  │   │  party    │
│             │       │             │       │             │   │           │
│ Config:     │       │ Config:     │       │ Config:     │   │           │
│ bgp { }     │       │ rib { }     │       │ (augments   │   │           │
│ (root)      │       │ (root)      │       │  bgp.*.cap  │   │           │
│             │       │             │       │  .gr)       │   │           │
│ Contains:   │       │ Contains:   │       │             │   │           │
│ ├─plugin/   │       │ ├─plugin/   │       │ Contains:   │   │           │
│ │ bgp/*     │       │ │ rib/*     │       │ ├─plugin/   │   │           │
│ ├─plugin/   │       │ └─storage/  │       │ │ gr/*      │   │           │
│ │ bgp/rib/  │       │             │       │             │   │           │
│ └─reactor   │       │             │       │             │   │           │
│             │       │             │       │             │   │           │
│ Owns:       │       │ Owns:       │       │ Owns:       │   │           │
│ ├─listeners │       │ ├─Adj-RIB-In│       │ ├─GR state  │   │           │
│ ├─sessions  │       │ ├─Adj-RIB-  │       │ └─cap       │   │           │
│ ├─FSM       │       │ │ Out       │       │   injection │   │           │
│ └─peer-to-  │       │ └─replay    │       │             │   │           │
│   peer rib  │       │             │       │             │   │           │
└─────────────┘       └─────────────┘       └─────────────┘   └───────────┘

All processes are equal peers using 5-stage protocol via pipes.
GR has no root config block - it augments BGP schema for graceful-restart paths.
```

### Package Reorganization

Move BGP and RIB code under `internal/plugin/`:

| Current Location | New Location | Process |
|------------------|--------------|---------|
| `internal/bgp/*` | `internal/plugin/bgp/*` | `ze bgp` |
| `internal/rib/` | `internal/plugin/bgp/rib/` | `ze bgp` (peer-to-peer) |
| `internal/plugin/rib/` | `internal/plugin/rib/` | `ze rib` (adj-rib tracking) |
| `internal/reactor/` | `internal/plugin/bgp/reactor/` | `ze bgp` |

### Two RIB Packages

| Package | Process | Purpose |
|---------|---------|---------|
| `internal/plugin/bgp/rib/` | `ze bgp` | Peer-to-peer route propagation, best path selection, route reflection |
| `internal/plugin/rib/` | `ze rib` | Adj-RIB tracking, route replay on reconnect, CLI queries |

**Why both?**
- `plugin/bgp/rib/` needs low-latency access to BGP session state for route propagation
- `plugin/rib/` is observational - tracks what was sent/received for replay and CLI

### Communication Flow

```
ze bgp                        ze (hub)                      ze rib
   │                             │                             │
   │◄─── config bgp {...} ───────│                             │
   │                             │───── config rib {...} ─────►│
   │                             │                             │
   │── subscribe bgp.event.* ───►│                             │
   │                             │◄── subscribe bgp.event.* ───│
   │                             │                             │
   │ (peer establishes)          │                             │
   │── event bgp.peer.up {...} ─►│── event bgp.peer.up {...} ─►│
   │                             │                             │
   │ (UPDATE received)           │                             │
   │── event bgp.update {...} ──►│── event bgp.update {...} ──►│
   │                             │                             │
   │                             │◄── bgp peer X update ... ───│
   │◄── bgp peer X update ... ───│     (route replay)          │
   │                             │                             │
```

### GR Plugin Interaction

GR plugin coordinates with BGP and RIB via commands through hub:

```
ze gr                         ze (hub)                      ze bgp
   │                             │                             │
   │◄── config bgp.*.cap.gr ─────│                             │
   │    (JSON subtree)           │                             │
   │                             │                             │
   │ (inject capability)         │                             │
   │── bgp capability hex ... ──►│── bgp capability hex ... ──►│
   │                             │                             │
   │                             │    (BGP stores, uses in OPEN)
   │                             │                             │
   │── subscribe bgp.peer.* ────►│                             │
   │                             │                             │
   │                             │◄── event bgp.peer.restart ──│
   │◄── event bgp.peer.restart ──│                             │
   │                             │                             │
   │ (coordinate restart)        │                             │
   │── rib defer peer X ────────►│── rib defer peer X ─────────│──► ze rib
   │                             │                             │
```

**Key points:**
- BGP provides generic `bgp capability` command API
- GR uses this API to inject graceful-restart capability
- GR subscribes to peer events to handle restart scenarios
- GR coordinates with RIB for route deferral during restart

### 5-Stage Protocol (per process)

Already implemented in `internal/plugin/subsystem.go`. Each forked process follows:

| Stage | Direction | Content |
|-------|-----------|---------|
| 1 | Process → Hub | `declare schema yang <module>`, `declare schema handler <path>`, `declare cmd`, `declare done` |
| 2 | Hub → Process | `config <section>`, `config done` |
| 3 | Process → Hub | `capability hex ...`, `capability done` |
| 4 | Hub → Process | `registry cmd ...`, `registry done` |
| 5 | Process → Hub | `ready` |

- YANG module provides schema for validation
- Handler paths tell hub which config blocks to route to this process
- Hub uses existing `SchemaRegistry.FindHandler()` for routing

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHubForkProcess` | `internal/hub/hub_test.go` | Hub can fork and communicate with process | |
| `TestHubSchemaRouting` | `internal/hub/hub_test.go` | Schema registry routes config to correct process | |
| `TestHubConfigDelivery` | `internal/hub/hub_test.go` | Config sections delivered to correct process | |
| `TestHubEventRouting` | `internal/hub/hub_test.go` | Events routed to subscribers | |
| `TestHubCommandRouting` | `internal/hub/hub_test.go` | Commands routed by handler path | |
| `TestBGPProcessStartup` | `cmd/ze/bgp/process_test.go` | BGP process completes 5-stage protocol | |

**Existing tests:** YANG loading tested in `internal/yang/loader_test.go`, schema registry tested in `internal/plugin/schema_test.go`

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - architectural refactor, no new numeric inputs | | | | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `hub-fork-bgp` | `test/data/hub/fork-bgp.ci` | Hub forks ze bgp, delivers config | |
| `hub-fork-rib` | `test/data/hub/fork-rib.ci` | Hub forks ze rib, events flow | |
| `hub-event-routing` | `test/data/hub/event-routing.ci` | Events from bgp reach rib | |

### Future (if deferring any tests)
- Performance tests comparing in-process vs forked architecture
- Stress tests with many peers and high route churn

## Files to Modify

### Update cmd entry points
- `cmd/ze/main.go` - Hub mode when given config file
- `cmd/ze/bgp/main.go` - BGP process mode (already exists, enhance)

## Files to Move

### Move BGP code under plugin/
- `internal/bgp/*` → `internal/plugin/bgp/*`
- `internal/rib/` → `internal/plugin/bgp/rib/`
- `internal/reactor/` → `internal/plugin/bgp/reactor/`

## Files to Create

### Hub package
- `internal/hub/hub.go` - Core hub orchestrator
- `internal/hub/process.go` - Process management (fork, pipes)
- `internal/hub/router.go` - Command/event routing
- `internal/hub/config.go` - Minimal config parsing (env, plugin blocks)
- `internal/hub/hub_test.go` - Tests

### YANG modules (only missing ones)
- `yang/ze-rib.yang` - RIB configuration schema (defines `rib` container)
- `yang/ze-gr.yang` - GR schema (YANG `augment` of ze-bgp, adds `graceful-restart` under `capability`)

**Already exists:** `yang/ze-bgp.yang`, `yang/ze-plugin.yang`, `yang/ze-types.yang`, `internal/yang/*`, `internal/plugin/schema.go`

**YANG modification needed:** Remove `graceful-restart` leaf from `yang/ze-bgp.yang` (line 274). GR plugin owns this config via augment in `ze-gr.yang`.

## Implementation Steps

### Phase 1: Create Hub Package

1. **Create hub package** - Extract non-BGP logic from reactor
   - Process forking (already in SubsystemManager)
   - Event routing (already in plugin.Server)
   - Command routing (already in Dispatcher)
   → **Review:** Is this just reorganizing existing code?

2. **Write hub tests** - Verify hub can fork and communicate
   → **Review:** Tests cover 5-stage protocol?

3. **Run tests** - Verify PASS

### Phase 2: BGP as Forked Process

1. **Enhance ze bgp** - Make it work as forked child
   - Accept config via stdin (Stage 2)
   - Send events via stdout
   - Receive commands via stdin
   → **Review:** Does existing `ze bgp server` already do this?

2. **Move reactor BGP code** - Into ze bgp process
   - TCP listeners
   - Session management
   - FSM
   - Message handling
   → **Review:** What stays in hub?

3. **Wire hub → ze bgp** - Hub forks ze bgp, passes config
   → **Review:** Config delivery matches 5-stage protocol?

4. **Run tests** - Verify BGP works via fork

### Phase 3: RIB as Forked Process

1. **Enhance ze rib** - Already mostly done in `internal/plugin/rib/`
   - Verify it follows 5-stage protocol
   - Add any missing config handling
   → **Review:** What's missing from current implementation?

2. **Wire hub → ze rib** - Hub forks ze rib
   → **Review:** Event subscription works?

3. **Run tests** - Verify RIB receives events, can replay routes

### Phase 4: Remove Old Reactor

1. **Delete reactor package** - After all code moved/reorganized
   → **Review:** No remaining references?

2. **Update imports** - All code uses new packages

3. **Run full test suite** - `make lint && make test && make functional`
   → **Review:** All 80+ functional tests pass?

## Design Decisions

### Why fork processes instead of in-process services?

| Benefit | Description |
|---------|-------------|
| Language freedom | BGP could be rewritten in Rust, RIB in Python |
| Crash isolation | BGP crash doesn't kill RIB |
| Resource limits | Each process can have memory/CPU limits |
| Debugging | Attach debugger to single process |
| Testing | Test each process independently |

### What code stays in hub?

| In Hub | In BGP Process (`ze bgp`) | In RIB Process (`ze rib`) |
|--------|---------------------------|---------------------------|
| Process forking | TCP listeners | Adj-RIB-In storage |
| Pipe management | BGP sessions | Adj-RIB-Out storage |
| Event routing | FSM | Route replay |
| Command routing | Message encode/decode | CLI queries |
| Signal handling | Peer-to-peer routing (`plugin/bgp/rib/`) | |
| Config section routing | Protocol code (`plugin/bgp/*`) | |

### Package Structure After Refactor

```
internal/
├── hub/                    ← NEW: hub/orchestrator (protocol-agnostic)
│   ├── hub.go
│   ├── process.go
│   ├── router.go
│   └── config.go
│
├── plugin/
│   ├── bgp/                ← MOVED: from internal/bgp/ + internal/reactor/
│   │   ├── message/        ← BGP message encode/decode
│   │   ├── attribute/      ← BGP path attributes
│   │   ├── nlri/           ← BGP NLRI types
│   │   ├── capability/     ← BGP capabilities
│   │   ├── fsm/            ← BGP FSM
│   │   ├── rib/            ← MOVED: from internal/rib/ (peer-to-peer)
│   │   └── reactor/        ← MOVED: from internal/reactor/ (BGP-specific)
│   │
│   ├── rib/                ← EXISTS: Adj-RIB tracking plugin
│   │   ├── rib.go
│   │   └── storage/
│   │
│   ├── gr/                 ← EXISTS: Graceful Restart plugin
│   └── ...                 ← Other plugins
│
└── config/                 ← Stays (used by both hub and bgp)
```

### Config file parsing

Hub does **full** config parsing (like VyOS):
1. Parse entire config file
2. Validate against combined YANG schema (all plugins)
3. Convert to internal map-of-maps structure
4. Route JSON subtrees to plugins based on handler registration

**Routing by handler:**
- Root handlers: `bgp` handler gets entire `bgp { }` as JSON
- Sub-root handlers: `bgp.capability.graceful-restart` handler gets just that subtree as JSON

**Plugin config query:**
Plugins can also query hub for specific config paths on-demand:
```
query config bgp.peer[address=192.0.2.1].timers
→ {"hold-time": 90, "keepalive": 30}
```

Hub stores config as map-of-maps, provides JSON to plugins.

### Existing Plugin Infrastructure

These files already exist and will be **reused** by the hub:

| File | Purpose | Disposition |
|------|---------|-------------|
| `internal/plugin/subsystem.go` | SubsystemHandler, 5-stage protocol | Keep, use in hub |
| `internal/plugin/server.go` | Multiplexes plugin I/O | Keep, use in hub |
| `internal/plugin/handler.go` | Command/event dispatch | Keep, use in hub |
| `internal/plugin/dispatcher.go` | Routes commands to handlers | Keep, use in hub |
| `internal/plugin/command.go` | Command registration | Keep, use in hub |
| `internal/plugin/rib/` | RIB plugin process | Move to `ze rib` |
| `internal/plugin/gr/` | GR plugin process | Move to `ze gr` |
| `internal/plugin/filter/` | Filter plugin | Move to `ze bgp` (BGP-specific filtering) |

The hub will import and use the existing infrastructure from `internal/plugin/`.

### CLI Command Routing

When user runs `ze bgp peer 192.0.2.1 update ...`:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ CLI: ze bgp peer 192.0.2.1 update ...                                       │
│                                                                             │
│  1. CLI connects to hub via Unix socket (/var/run/ze/api.sock)              │
│  2. CLI sends: bgp peer 192.0.2.1 update ...                                │
│                                                                             │
│                          ▼                                                  │
│                                                                             │
│ Hub: routes by prefix                                                       │
│  1. Lookup "bgp" in handler map → ze bgp process                            │
│  2. Forward command via stdin to ze bgp                                     │
│  3. Wait for response from ze bgp stdout                                    │
│  4. Return response to CLI via socket                                       │
│                                                                             │
│                          ▼                                                  │
│                                                                             │
│ ze bgp process:                                                             │
│  1. Receives: bgp peer 192.0.2.1 update ...                                 │
│  2. Executes command                                                        │
│  3. Sends response via stdout                                               │
└─────────────────────────────────────────────────────────────────────────────┘
```

**CLI connection:** Same `ze` binary acts as both hub and CLI:
- `ze config.conf` → starts as hub, listens on Unix socket
- `ze bgp peer list` → connects to running hub as CLI client

Socket path configurable via `env { api-socket /path/to/socket; }`

CLI commands like `ze rib show` follow the same pattern, routed to `ze rib` process.

## Relationship to Hub Architecture Doc

This spec implements the vision from `docs/architecture/hub-architecture.md`:

| Hub Doc Phase | This Spec |
|---------------|-----------|
| Phase 0 (foundation) | ✅ Already done (SubsystemManager, 5-stage) |
| Phase 1 (schema) | Future - YANG schema registration |
| Phase 2 (config reader) | Future - separate config-reader process |
| This spec | **BGP and RIB as forked processes** |

## Implementation Summary

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered during implementation]

### Design Insights
- [Key learnings that should be documented elsewhere]

### Deviations from Plan
- [Any differences from original plan and why]

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read
- [ ] Architecture docs updated with learnings

### Completion (after tests pass)
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
