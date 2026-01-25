# Hub Separation - Master Overview

**Status:** Planning
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

## Existing Infrastructure

**Already implemented** in `internal/plugin/`:

| Component | File | Status |
|-----------|------|--------|
| Hub struct | `hub.go` | RouteCommand, ProcessConfig, RouteCommit |
| SchemaRegistry | `schema.go` | Register, FindHandler (longest-prefix) |
| SubsystemHandler | `subsystem.go` | 5-stage protocol, forked processes |
| GR plugin | `gr/gr.go` | Capability injection working |
| RIB plugin | `rib/rib.go` | Adj-RIB tracking |

**Phases extend/integrate this, not replace it.**

## Phase Overview

| Phase | Spec | Description | Dependencies |
|-------|------|-------------|--------------|
| 1 | `spec-hub-phase1-foundation.md` | Create hub package, basic fork/pipe | None |
| 2 | `spec-hub-phase2-config.md` | Parse 3-section config, env handling | Phase 1 |
| 3 | `spec-hub-phase3-schema-routing.md` | SchemaRegistry, JSON config delivery | Phase 2 |
| 4 | `spec-hub-phase4-bgp-process.md` | Move BGP code, ze bgp as child | Phase 3 |
| 5 | `spec-hub-phase5-routing.md` | Event/command routing between plugins | Phase 4 |
| 6 | `spec-hub-phase6-gr-plugin.md` | ze-gr.yang, capability injection | Phase 5 |
| 7 | `spec-hub-phase7-cleanup.md` | Remove old reactor, final cleanup | Phase 6 |

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

## Package Structure After Completion

```
internal/
├── hub/                    # NEW: Phase 1-3
│   ├── hub.go              # Entry point, composes plugin.* components
│   ├── config.go           # Config parsing (Phase 2) + state (Phase 3)
│   └── router.go           # Command/event routing (Phase 5)
│
├── plugin/
│   ├── bgp/                # MOVED: Phase 4
│   │   ├── message/
│   │   ├── attribute/
│   │   ├── nlri/
│   │   ├── capability/
│   │   ├── fsm/
│   │   ├── rib/            # peer-to-peer
│   │   ├── reactor/
│   │   └── filter/
│   │
│   ├── rib/                # EXISTS: Phase 5 verification
│   ├── gr/                 # Phase 6
│   │
│   ├── subsystem.go        # EXISTS: reused by hub
│   ├── schema.go           # EXISTS: reused by hub
│   └── server.go           # EXISTS: reused by hub
```

## Notes

- Each phase should be completable in a single session
- Each phase has its own spec with TDD test plan
- Run `make test && make lint && make functional` after each phase
- Commit after each phase passes
