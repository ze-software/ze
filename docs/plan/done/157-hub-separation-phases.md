# Hub Separation - Master Overview

**Status:** Complete ✅
**Purpose:** Break reactor-service-separation into manageable phases

## Goal

Refactor ZeBGP so that:
- `ze` is a hub/orchestrator that forks and coordinates plugins
- `ze bgp` is a separate process for BGP protocol
- `ze rib` is a separate process for Adj-RIB tracking
- `ze gr` is a separate process for Graceful Restart
- All plugins communicate via pipes using 5-stage protocol

## Reference Documents

- `docs/architecture/system-architecture.md` - End-user view of target
- `docs/architecture/hub-architecture.md` - Internal design details
- `docs/plan/spec-reactor-service-separation.md` - Full technical details (archive)

## Current Infrastructure

| Component | Location | Purpose |
|-----------|----------|---------|
| Hub/Orchestrator | `internal/hub/` | Config parsing, process management |
| SubsystemHandler | `internal/plugin/subsystem.go` | 5-stage protocol, forked processes |
| SchemaRegistry | `internal/plugin/schema.go` | Handler routing (longest-prefix match) |
| BGP Engine | `internal/plugin/bgp/` | BGP protocol implementation |
| GR Plugin | `internal/plugin/gr/` | Graceful restart capability injection |
| RIB Plugin | `internal/plugin/rib/` | Adj-RIB tracking |
| Child Mode | `cmd/ze/bgp/childmode.go` | BGP as hub child process |

## Phase Overview

| Phase | Spec | Description | Status |
|-------|------|-------------|--------|
| 1 | `spec-hub-phase1-foundation.md` | Create hub package, basic fork/pipe | ✅ Complete |
| 2 | `spec-hub-phase2-config.md` | Parse 3-section config, env handling | ✅ Complete |
| 3 | `spec-hub-phase3-schema-routing.md` | SchemaRegistry, JSON config delivery | ✅ Complete |
| 4 | `spec-hub-phase4-bgp-process.md` | Move BGP code, ze bgp as child | ✅ Complete |
| 5 | `spec-hub-phase5-routing.md` | Event/command routing between plugins | ✅ Complete |
| 6 | `spec-hub-phase6-gr-plugin.md` | ze-gr.yang, capability injection | ✅ Complete |
| 7 | `spec-hub-phase7-cleanup.md` | Remove old reactor, final cleanup | ✅ Complete |

## Phase Summaries

### Phase 1: Hub Foundation
- Create `internal/hub/` package
- Fork child processes
- Basic pipe communication
- Reuse existing `internal/plugin/subsystem.go` infrastructure

### Phase 2: Config Parsing
- Parse 3-section config (env, plugin, config blocks)
- Handle `env { }` for global settings
- Parse `plugin { }` to know what to fork
- Store config as map-of-maps

### Phase 3: Schema Routing
- Integrate with existing SchemaRegistry
- Plugins declare YANG + handlers + priority in Stage 1
- Hub maintains live/edit config states (VyOS-inspired)
- Pull model: hub notifies plugins, plugins query for config
- Hub never pushes config data, only sends notifications
- Shared diff library for plugins (`internal/config/diff/`)
- Priority ordering for verify/apply (lower = first)

### Phase 4: BGP Process Separation
- Move `internal/bgp/*` → `internal/plugin/bgp/*`
- Move `internal/rib/` → `internal/plugin/bgp/rib/`
- Move `internal/reactor/` → `internal/plugin/bgp/reactor/`
- Make `ze bgp` work as forked child process

### Phase 5: Event & Command Routing
- Hub routes commands by handler prefix
- Hub routes events via pub/sub
- CLI connects via Unix socket
- Verify existing RIB plugin works

### Phase 6: GR Plugin
- Create `yang/ze-gr.yang` (augments ze-bgp)
- Remove graceful-restart from ze-bgp.yang
- GR injects capabilities via `capability hex <code> <value> peer <addr>`
- GR subscribes to peer events

### Phase 7: Cleanup
- Delete old `internal/reactor/` (after code moved)
- Update all imports
- Run full test suite
- Update documentation

## Success Criteria

After all phases:
```bash
# Start hub with config
ze config.conf

# Processes running
ps aux | grep ze
# ze config.conf (hub)
# ze bgp (child)
# ze rib (child)
# ze gr (child)

# CLI works
ze bgp peer list
ze rib show

# Config reload works
kill -HUP $(pgrep -f "ze config.conf")
```

## Package Structure (Current)

```
internal/
├── hub/                    # ✅ Created in Phase 1-3
│   ├── hub.go              # Entry point, Orchestrator
│   ├── config.go           # 3-section config parsing
│   └── orchestrator.go     # Process lifecycle management
│
├── plugin/
│   ├── bgp/                # ✅ Moved in Phase 4
│   │   ├── message/
│   │   ├── attribute/
│   │   ├── nlri/
│   │   ├── capability/
│   │   ├── fsm/
│   │   ├── rib/            # BGP peer-to-peer RIB
│   │   ├── reactor/        # BGP event loop
│   │   └── filter/
│   │
│   ├── rib/                # ✅ Adj-RIB plugin
│   ├── gr/                 # ✅ GR plugin (Phase 6)
│   │
│   ├── subsystem.go        # ✅ 5-stage protocol handler
│   ├── schema.go           # ✅ Handler routing
│   └── server.go           # ✅ Plugin server
│
cmd/ze/
├── main.go                 # ✅ Routes config → hub
├── hub/main.go             # ✅ Hub command entry
└── bgp/childmode.go        # ✅ Child mode with reactor

yang/
├── ze-bgp.yang             # ✅ BGP schema
└── ze-gr.yang              # ✅ GR augment (Phase 6)

test/hub/
└── startup_test.go         # ✅ Hub functional tests
```

## Implementation Summary

### What Was Implemented

**Phase 1-3: Hub Foundation**
- `internal/hub/hub.go` - Orchestrator entry point
- `internal/hub/config.go` - 3-section config parser (env, plugin, blocks)
- `internal/hub/orchestrator.go` - Process management using existing SubsystemHandler

**Phase 4: BGP Process Separation**
- Moved `internal/bgp/*` → `internal/plugin/bgp/*`
- Moved `internal/reactor/` → `internal/plugin/bgp/reactor/`
- Moved `internal/rib/` → `internal/plugin/bgp/rib/`
- Updated 253 import paths

**Phase 5: Command Entry Points**
- `cmd/ze/main.go` routes config files to hub
- `cmd/ze/hub/main.go` - Hub command entry point
- `cmd/ze/bgp/childmode.go` - Full 5-stage protocol with reactor integration

**Phase 6: GR Plugin YANG**
- Created `yang/ze-gr.yang` (augments ze-bgp)
- Removed graceful-restart leaf from `yang/ze-bgp.yang`

**Phase 7: Cleanup & Tests**
- `test/hub/startup_test.go` - Hub functional tests
- `internal/plugin/subsystem.go` - ConfigPath passing via `--config` flag
- All imports updated, no old reactor remnants

### Key Design Decisions

1. **Config path passing**: Hub passes config file path to children via `--config` flag, not JSON config
2. **Binary detection**: SubsystemHandler detects full commands (with spaces) vs binary paths to handle `--mode` intelligently
3. **Reactor integration**: Child mode loads reactor from config file after 5-stage protocol completes

### Verification

```bash
make test       # ✅ All unit tests pass
make lint       # ✅ 0 issues
make functional # ✅ All 80+ functional tests pass
```

## Notes

- Each phase should be completable in a single session
- Each phase has its own spec with TDD test plan
- Run `make test && make lint && make functional` after each phase
- Commit after each phase passes
