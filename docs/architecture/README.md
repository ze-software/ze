# Architecture Documentation

Architecture documents describe how the current implementation is wired. Prefer source-anchored documents here over historical research notes when checking behavior.

## Core Map

| Topic | Document | Code anchor |
|-------|----------|-------------|
| One-page overview | `../architecture.md` | `internal/component/engine/`, `cmd/ze/hub/` |
| Core design | `core-design.md` | BGP, plugin, config, and engine packages |
| System architecture | `system-architecture.md` | Hub startup and subsystem wiring |
| Plugin manager wiring | `plugin-manager-wiring.md` | `internal/component/plugin/` |
| Subsystem wiring | `subsystem-wiring.md` | Registered components and plugin server wiring |
| Config design | `config/` | `internal/component/config/` and YANG modules |
| API and IPC | `api/` | `pkg/plugin/rpc/`, `pkg/plugin/sdk/`, command schemas |
| BGP wire format | `wire/` | `internal/component/bgp/attribute/`, `message/`, `wireu/` |
| Route and RIB behavior | `route-selection.md`, `route-types.md`, `rib-transition.md` | `internal/core/rib/`, `internal/component/bgp/plugins/rib/` |
| Pools and buffers | `pool-architecture.md`, `buffer-architecture.md` | `internal/component/bgp/attrpool/`, `internal/core/bufpool/` |
| Web and UI | `web-interface.md`, `web-components.md` | `internal/component/web/` |
| Testing architecture | `testing/` | `cmd/ze/ze_test_*.go`, `internal/test/`, `test/` |
| Decisions | `decisions/` | Decision records tied to current implementation |

## Reading Order

1. Start with `../architecture.md` for the current component map.
2. Read `core-design.md` for the detailed implementation model.
3. Move into the topic directory that matches the package you are changing.
4. Use source anchors in the document to verify claims against code.
