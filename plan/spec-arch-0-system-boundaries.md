# Spec: arch-0 — System Boundaries (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | 0/6 |
| Updated | 2026-03-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` — workflow rules
3. `docs/architecture/core-design.md` — current architecture
4. `docs/architecture/system-architecture.md` — current system overview

## Task

Restructure Ze's internal architecture to establish clear, interface-defined boundaries between five components: **Engine** (supervisor), **Bus** (content-agnostic pub/sub), **Config Manager** (config provider), **Plugin Manager** (plugin lifecycle), and **Subsystems** (BGP daemon and future daemons).

This is an umbrella spec. Each migration phase will have its own child spec (`spec-arch-N-<name>.md`).

### Problem Statement

The current architecture has unclear boundaries between components:

| Problem | Where |
|---------|-------|
| Three unrelated things called "Hub" | `plugin.Server`, `hub.Orchestrator`, `plugin.Hub` |
| `plugin.Server` is a god object | Event dispatch, plugin lifecycle, RPC routing, BGP hooks, startup coordination, cache tracking — all in one struct |
| No interfaces at component boundaries | Components coupled through concrete types, not contracts |
| Bus is not content-agnostic | `BGPHooks` callbacks, `EventUpdate`/`EventOpen` constants, `bgptypes.RawMessage` type assertions — all inside the generic plugin infrastructure |
| BGP daemon conflated with plugins | Reactor lives in `internal/component/bgp/` but is a first-class subsystem, not a plugin |
| No topic/channel concept | Hardcoded namespace strings (`"bgp"`, `"rib"`) with no way for new subsystems to define their own |
| Config delivery is ad-hoc | Plugins get config pushed during 5-stage startup, but subsystems load config through a completely different path |

### Key Distinction: Subsystem vs Plugin

| Concept | Role | Owns external I/O | Long-lived goroutines | Supervised by |
|---------|------|--------------------|-----------------------|---------------|
| **Subsystem** | First-class daemon that produces/consumes events | Yes (TCP listeners, timers, protocol state) | Yes (FSM, wire I/O, forwarding) | Engine directly |
| **Plugin** | Extension that adds behavior by reacting to events | No | Yes (one delivery goroutine per plugin) | Plugin Manager |
| **Plugin System** | Infrastructure for plugin lifecycle and bus connectivity | No | Yes (startup coordinator) | Engine via Plugin Manager |

The BGP daemon is a **subsystem**. It listens on TCP, runs FSMs, parses wire bytes, and publishes events to the bus. `bgp-rib`, `bgp-rs`, `bgp-gr` are **plugins** — they subscribe to bus topics and extend the subsystem's behavior. The plugin system is infrastructure that both use.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — current engine + plugin architecture
  → Constraint: engine is stateless for routes (no RIB in reactor)
  → Constraint: WireUpdate is lazy-parsed, zero-copy — bus must not force eager parsing
- [ ] `docs/architecture/system-architecture.md` — current system overview, two operating modes
  → Decision: hub mode (fork processes) vs in-process mode (goroutines + DirectBridge) both exist
  → Constraint: both modes must continue to work through the new interfaces
- [ ] `docs/architecture/rib-transition.md` — RIB ownership moved to plugins
  → Decision: RIB is plugin-owned, not engine-owned — validates subsystem/plugin split
- [ ] `docs/architecture/hub-architecture.md` — hub mode design aspirations
  → Constraint: hub mode is partially implemented — new design replaces it, not layers on top
- [ ] `docs/architecture/overview.md` — package layout
  → Constraint: `internal/component/plugin/` (infra) vs `internal/component/bgp/plugins/` (implementations) distinction preserved

### Source Files (current boundaries)
- [ ] `internal/component/plugin/types.go` — `ReactorLifecycle` (17 methods), `BGPHooks`, `PeerInfo`, `RPCRegistration`
  → Constraint: `ReactorLifecycle` is the existing boundary between generic infra and BGP — it must evolve into the Subsystem interface
- [ ] `internal/component/plugin/server.go` — `Server` struct (god object: 15+ fields across 6 concerns)
  → Constraint: this is what gets decomposed into Bus + PluginManager
- [ ] `internal/component/plugin/subscribe.go` — `SubscriptionManager`, `Subscription` (namespace/eventType/direction/peer matching)
  → Constraint: subscription matching logic is correct — it moves into the Bus, not rewritten
- [ ] `internal/component/plugin/events.go` — hardcoded event type constants (`EventUpdate`, `EventOpen`, etc.)
  → Decision: event types become opaque strings owned by subsystems, not by the bus
- [ ] `internal/component/plugin/process.go` — `Process` struct (5-stage lifecycle, `eventChan`, `DirectBridge`)
  → Constraint: 5-stage protocol preserved — it's plugin lifecycle, separate from bus
- [ ] `internal/component/plugin/hub.go` — `Hub` struct (config command routing via `SchemaRegistry`)
  → Decision: config routing becomes part of Config Manager
- [ ] `internal/hub/hub.go` — `Orchestrator` (forks external processes)
  → Decision: replaced by Engine supervisor
- [ ] `internal/component/bgp/reactor/reactor.go` — `Reactor` struct, holds `*plugin.Server` as `api` field
  → Constraint: reactor becomes a Subsystem implementation — receives Bus + ConfigProvider, no longer holds Server
- [ ] `internal/component/bgp/server/hooks.go` — `NewBGPHooks()` creating callback table
  → Decision: BGPHooks disappear — BGP subsystem publishes to bus directly
- [ ] `internal/component/bgp/server/events.go` — event delivery logic (`onMessageReceived`, etc.)
  → Constraint: delivery logic moves into the bus; formatting stays in the BGP subsystem
- [ ] `pkg/plugin/sdk/sdk.go` — `Plugin` SDK (5-stage, callbacks, `OnEvent`/`OnStructuredEvent`)
  → Constraint: SDK preserved — plugins still use it; SDK receives bus reference after Stage 5

**Key insights:**
- `plugin.Server` does six jobs: plugin lifecycle, event subscription, event dispatch, RPC routing, BGP hook injection, startup coordination
- The bus behavior (subscribe, match, deliver) is in `SubscriptionManager` + `process.deliveryLoop()` — extractable
- `BGPHooks` exists solely because `internal/component/plugin/` cannot import `internal/component/bgp/` — with a content-agnostic bus this indirection is unnecessary
- The 5-stage protocol is plugin lifecycle, not bus behavior — it belongs in Plugin Manager
- Config delivery during Stage 2 (`deliverConfigToProcess`) is a Plugin Manager concern, not a bus concern

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/types.go` — ReactorLifecycle (17 methods), BGPHooks callback struct, PeerInfo, RPCRegistration
- [ ] `internal/component/plugin/server.go` — Server struct (god object: reactor, dispatcher, rpcDispatcher, bgpHooks, procManager, subscriptions, coordinator, registry, capInjector, listener, clients)
- [ ] `internal/component/plugin/subscribe.go` — SubscriptionManager, Subscription (namespace/eventType/direction/peer matching)
- [ ] `internal/component/plugin/events.go` — hardcoded event constants (EventUpdate, EventOpen, EventState, etc.)
- [ ] `internal/component/plugin/process.go` — Process struct (5-stage lifecycle, eventChan cap 64, DirectBridge, deliveryLoop)
- [ ] `internal/component/plugin/hub.go` — Hub struct (config command routing via SchemaRegistry + SubsystemManager)
- [ ] `internal/hub/hub.go` — Orchestrator (forks external processes, composes SubsystemManager + SchemaRegistry + Hub)
- [ ] `internal/component/bgp/reactor/reactor.go` — Reactor struct (holds api *plugin.Server, messageReceiver, peers, listeners, fwdPool)
- [ ] `internal/component/bgp/server/hooks.go` — NewBGPHooks() creating BGPHooks callback table
- [ ] `internal/component/bgp/server/events.go` — onMessageReceived, onPeerStateChange delivery implementations
- [ ] `pkg/plugin/sdk/sdk.go` — Plugin SDK (5-stage, OnEvent/OnStructuredEvent/OnConfigure callbacks, MuxConn, DirectBridge)

**Behavior to preserve:**
- 5-stage plugin startup protocol (declare → configure → capabilities → registry → ready)
- DirectBridge optimization for in-process plugins (bypasses socket I/O after startup)
- Subscription matching semantics (namespace, eventType, direction, peer filter)
- Per-process `eventChan` with long-lived `deliveryLoop()` goroutine
- Zero-copy event passing (WireUpdate references, not copies)
- Plugin SDK callback interface (`OnEvent`, `OnStructuredEvent`, `OnConfigure`, etc.)
- Both operating modes: in-process (goroutines + DirectBridge) and external (fork + sockets)
- YANG schema aggregation from plugins at startup
- Config reload via SIGHUP

**Behavior to change:**
- `plugin.Server` decomposed into Bus + PluginManager + startup coordinator
- `BGPHooks` callback struct eliminated — BGP publishes to bus directly
- Event type constants moved from generic infra to subsystem that owns them
- `hub.Orchestrator` replaced by Engine supervisor
- `plugin.Hub` (config router) absorbed into Config Manager
- Reactor no longer holds `*plugin.Server` — receives Bus + ConfigProvider interfaces
- Bus becomes content-agnostic — never type-asserts event payloads

## Target Architecture

### Component Hierarchy

```
Engine (supervisor)
├── Bus (content-agnostic pub/sub)
│     Moves opaque payloads between producers and consumers
│     Topics created by subsystems and plugins, not hardcoded
│
├── Config Manager (config provider)
│     File → YANG validate → tree → query by root
│     Notifies on reload
│
├── Plugin Manager (plugin lifecycle)
│     5-stage protocol, process management, DirectBridge
│     Connects plugins to the bus after Stage 5
│
└── Subsystems (registered with engine)
      └── BGP Subsystem
            TCP listeners, FSM, wire parsing, peer management
            Publishes to bus: bgp/update, bgp/state, ...
            Reads config via Config Manager
            └── Plugins (managed by Plugin Manager)
                  ├── bgp-rib    (subscribes bgp/update)
                  ├── bgp-rs     (subscribes rib/route)
                  ├── bgp-gr     (subscribes bgp/state)
                  └── bgp-nlri-* (NLRI codec via registry, no bus)
```

### Component Definitions

#### 1. Engine — Supervisor

The engine is a coordinator. It starts the bus, config manager, plugin manager, and subsystems in the correct order. It monitors health, handles signals (SIGHUP for reload, SIGTERM for shutdown), and owns the top-level context.

The engine has **no knowledge of BGP**. It does not parse wire bytes, manage peers, or understand protocol state. It starts whatever subsystems are registered.

| Responsibility | Yes | No |
|---------------|-----|-----|
| Start/stop components in order | Yes | |
| Signal handling (SIGHUP, SIGTERM) | Yes | |
| Health monitoring | Yes | |
| Know what BGP is | | No |
| Parse config content | | No — delegates to Config Manager |
| Manage plugin lifecycle | | No — delegates to Plugin Manager |
| Route events | | No — delegates to Bus |

**Current code mapping:** Partially `hub.Orchestrator`, partially `reactor.StartWithContext()` signal handling, partially `cmd/ze/hub/main.go` startup sequence. All merge into Engine.

#### 2. Bus — Content-Agnostic Pub/Sub

The bus moves opaque payloads between producers and consumers. It does not know what `bgp/update` means. It does not parse events. It does not have BGP hooks. It is pure plumbing — like RabbitMQ or Kafka.

| Responsibility | Yes | No |
|---------------|-----|-----|
| Topic creation and management | Yes | |
| Subscription matching (topic + filter) | Yes | |
| Event delivery to subscribers | Yes | |
| Know what events contain | | No |
| Parse or format event payloads | | No |
| Type-assert event data | | No |
| Manage plugin lifecycle | | No |

**Topic model:** Topics are hierarchical strings using `/` as separator: `bgp/update`, `bgp/events/peer-up`, `bgp/events/peer-down`, `rib/route`. Subsystems and plugins create topics. The hierarchy defines broadcast domains — subscribing to a prefix captures all topics under it.

**Subscription model:** Subscribers match on topic prefixes. A subscription to `bgp/` receives events from `bgp/update`, `bgp/events/peer-up`, and all other `bgp/` topics. A subscription to `bgp/events/` receives only `bgp/events/peer-up` and `bgp/events/peer-down`. A subscription to `bgp/update` receives only that exact topic. One consumer can hold multiple subscriptions to different prefixes.

**Metadata model:** Events carry opaque key-value metadata (e.g., `"peer"` → `"192.0.2.1"`, `"direction"` → `"received"`). Subscriptions can optionally filter on metadata — the bus matches strings, it doesn't know what the keys mean.

| Concept | Description |
|---------|-------------|
| **Topic** | Hierarchical string (`bgp/update`, `bgp/events/peer-up`), created by subsystems/plugins |
| **Event** | Opaque payload (`[]byte`) + metadata (string key-value pairs) + topic |
| **Subscription** | Topic prefix + optional metadata filter + consumer reference |
| **Prefix match** | Subscribe to `bgp/` → receive all `bgp/*` events; subscribe to `bgp/update` → exact match |
| **Delivery** | Bus calls consumer's delivery function; consumer owns decoding |

**Current code mapping:** `SubscriptionManager` (matching logic) + `Process.deliveryLoop()` (delivery) + `Process.eventChan` (buffering). These move into Bus.

#### 3. Config Manager — Config Provider

The config manager is the **central authority** for configuration. It loads config from files, validates against YANG schemas (aggregated from plugins), serves config subtrees to subsystems and plugins, handles live reload, and supports save/backup. It is the single interface through which all config consumers operate.

**Consumers:**

| Consumer | How it uses Config Manager |
|----------|---------------------------|
| BGP subsystem | `Get("bgp")` at startup, `Watch("bgp")` for reload |
| Plugins | Config delivered during 5-stage startup via Plugin Manager |
| `ze config edit` (CLI) | Load, validate, save, backup — interactive editing |
| Future web interface | Same load/validate/save API over HTTP |

| Responsibility | Yes | No |
|---------------|-----|-----|
| Load config file | Yes | |
| YANG validation | Yes | |
| Serve config subtrees by root name | Yes | |
| Notify on config change (reload) | Yes | |
| Aggregate YANG schemas from plugins | Yes | |
| Save config (with backup) | Yes | |
| Know what config values mean | | No — serves opaque trees |
| Apply config to running peers | | No — subsystem does that |

**Current code mapping:** `internal/config/` (load, parse, resolve) + `plugin.Hub` (config command routing) + `Server.deliverConfigToProcess()` (Stage 2 delivery) + `Reactor.Reload()` (apply) + `cmd/ze/config/` (edit command). Load/parse/validate/save stays. Routing and delivery move here. Apply stays in subsystem.

#### 4. Plugin Manager — Plugin Lifecycle

The plugin manager handles the 5-stage startup protocol, process management (fork/goroutine), DirectBridge setup for in-process plugins, and cleanup on shutdown. After Stage 5, it hands each plugin a Bus reference for runtime communication.

| Responsibility | Yes | No |
|---------------|-----|-----|
| 5-stage startup protocol | Yes | |
| Process fork / goroutine start | Yes | |
| DirectBridge setup (in-process) | Yes | |
| Config delivery during Stage 2 | Yes (via Config Manager) | |
| Capability collection during Stage 3 | Yes | |
| Plugin shutdown/cleanup | Yes | |
| Event subscription management | | No — plugin subscribes to Bus directly |
| Event delivery | | No — Bus handles delivery |

**Current code mapping:** `ProcessManager` + `StartupCoordinator` + `PluginRegistry` + `CapabilityInjector` + `Process` (5-stage fields). All stay, but extracted from `Server`.

#### 5. Subsystem — First-Class Daemon

A subsystem owns external I/O (TCP sockets, timers), runs long-lived goroutines (FSM, wire parsing), and publishes/subscribes events on the bus. It is supervised by the engine and receives Bus + Config Manager references at startup.

The BGP subsystem is currently the only subsystem. Future candidates: BMP monitoring, RPKI validation, telemetry export.

| Responsibility | Yes | No |
|---------------|-----|-----|
| Own TCP listeners | Yes | |
| Run protocol state machines | Yes | |
| Parse/encode wire bytes | Yes | |
| Create bus topics for its domain | Yes | |
| Publish events to bus | Yes | |
| Subscribe to bus for commands/responses | Yes | |
| Read config from Config Manager | Yes | |
| Manage plugins | | No — Plugin Manager does that |
| Know about other subsystems | | No |

**Current code mapping:** `reactor.Reactor` (TCP, FSM, peers, wire, forwarding). Stays mostly as-is but receives Bus + ConfigProvider interfaces instead of holding `*plugin.Server`.

### Component Interfaces

All interfaces defined in a single boundary package (proposed: `pkg/ze/`). Components depend on interfaces, not on each other's concrete types.

#### Engine Interface

| Method | Parameters | Returns | Purpose |
|--------|-----------|---------|---------|
| `RegisterSubsystem` | `Subsystem` | `error` | Register a subsystem before Start |
| `Start` | `context.Context` | `error` | Start all components in order |
| `Stop` | `context.Context` | `error` | Graceful shutdown |
| `Reload` | `context.Context` | `error` | Reload config, notify subsystems |
| `Bus` | — | `Bus` | Access the bus |
| `Config` | — | `ConfigProvider` | Access the config manager |
| `Plugins` | — | `PluginManager` | Access the plugin manager |

#### Bus Interface

| Method | Parameters | Returns | Purpose |
|--------|-----------|---------|---------|
| `CreateTopic` | `name string` | `Topic, error` | Register a new hierarchical topic (e.g., `bgp/update`) |
| `Publish` | `topic string, payload []byte, metadata map[string]string` | — | Publish opaque event to a topic |
| `Subscribe` | `prefix string, filter map[string]string, consumer Consumer` | `Subscription, error` | Subscribe to all topics matching prefix (e.g., `bgp/` or `bgp/update`) |
| `Unsubscribe` | `sub Subscription` | — | Remove a subscription |

The `Consumer` interface has one method:

| Method | Parameters | Returns | Purpose |
|--------|-----------|---------|---------|
| `Deliver` | `events []Event` | `error` | Receive a batch of events |

An `Event` carries:

| Field | Type | Purpose |
|-------|------|---------|
| `Payload` | `[]byte` | Opaque content — the bus never reads this |
| `Metadata` | `map[string]string` | Key-value pairs for filtering (e.g., `"peer"` → `"192.0.2.1"`, `"direction"` → `"received"`) |

#### ConfigProvider Interface

| Method | Parameters | Returns | Purpose |
|--------|-----------|---------|---------|
| `Load` | `path string` | `error` | Load config from file |
| `Get` | `root string` | `map[string]any, error` | Get config subtree for a root |
| `Validate` | — | `[]error` | Validate current config against YANG schema |
| `Save` | `path string` | `error` | Save config to file (with automatic backup) |
| `Watch` | `root string` | `<-chan ConfigChange` | Notify on config change |
| `Schema` | — | `SchemaTree` | Merged YANG schema |
| `RegisterSchema` | `name string, yang string` | `error` | Plugin registers its YANG schema |

#### PluginManager Interface

| Method | Parameters | Returns | Purpose |
|--------|-----------|---------|---------|
| `Register` | `PluginConfig` | `error` | Register a plugin for startup |
| `StartAll` | `context.Context, Bus, ConfigProvider` | `error` | Run 5-stage protocol for all plugins |
| `StopAll` | `context.Context` | `error` | Graceful shutdown of all plugins |
| `Plugin` | `name string` | `PluginProcess, bool` | Look up running plugin |
| `Plugins` | — | `[]PluginProcess` | List all running plugins |
| `Capabilities` | — | `[]Capability` | Collected capabilities from Stage 3 |

#### Subsystem Interface

| Method | Parameters | Returns | Purpose |
|--------|-----------|---------|---------|
| `Name` | — | `string` | Subsystem identifier |
| `Start` | `context.Context, Bus, ConfigProvider` | `error` | Start the subsystem |
| `Stop` | `context.Context` | `error` | Graceful shutdown |
| `Reload` | `context.Context, ConfigProvider` | `error` | Apply config changes |

### Startup Sequence

| Step | Actor | Action |
|------|-------|--------|
| 1 | Engine | Load config via Config Manager |
| 2 | Engine | Create Bus |
| 3 | Engine | Plugin Manager runs 5-stage protocol for all registered plugins |
| 3a | Plugin Manager | Stage 1: plugins declare registration |
| 3b | Plugin Manager | Stage 2: deliver config (via Config Manager) to each plugin |
| 3c | Plugin Manager | Stage 3: plugins declare capabilities |
| 3d | Plugin Manager | Stage 4: share registry with plugins |
| 3e | Plugin Manager | Stage 5: plugins signal ready; receive Bus reference |
| 4 | Engine | Start each registered subsystem, passing Bus + ConfigProvider |
| 4a | BGP Subsystem | Create topics (`bgp/update`, `bgp/state`, `bgp/open`, `bgp/notification`, `bgp/negotiated`) |
| 4b | BGP Subsystem | Open TCP listeners, start peer FSMs |
| 5 | Engine | Enter supervision loop (signal handling, health checks) |

Note: plugins complete their 5-stage startup **before** subsystems start producing events. This ensures all subscribers are ready when events begin flowing.

### Runtime Data Flow

#### BGP UPDATE received from peer

| Step | Component | Action |
|------|-----------|--------|
| 1 | BGP Subsystem | TCP read → wire parse → `WireUpdate` (lazy, zero-copy) |
| 2 | BGP Subsystem | Format payload (JSON/text/binary depending on subscriber needs) |
| 3 | BGP Subsystem | `bus.Publish(bgpUpdateTopic, payload, {"peer": "192.0.2.1", "direction": "received"})` |
| 4 | Bus | Match subscriptions on topic + metadata filters |
| 5 | Bus | Call `consumer.Deliver(events)` for each matching subscriber |
| 6 | Plugin (e.g., bgp-rib) | Receives payload, decodes, stores in RIB |

#### Plugin sends command back to BGP subsystem

| Step | Component | Action |
|------|-----------|--------|
| 1 | Plugin (bgp-rib) | `bus.Publish(bgpCommandTopic, payload, {"command": "cache-forward"})` |
| 2 | Bus | Match subscriptions — BGP subsystem is subscribed to `bgp/command` |
| 3 | BGP Subsystem | Receives command, executes (e.g., forward cached UPDATE to peer) |

#### Config reload (SIGHUP)

| Step | Component | Action |
|------|-----------|--------|
| 1 | Engine | Catches SIGHUP, calls `Reload()` |
| 2 | Engine | Config Manager re-reads file, validates YANG |
| 3 | Config Manager | Notifies watchers via `Watch()` channels |
| 4 | BGP Subsystem | Receives config change, reconciles peers (add/remove/modify) |
| 5 | Plugin Manager | Optionally re-delivers config to plugins that requested it |

### Boundary Rules

| Rule | Rationale |
|------|-----------|
| Engine never imports subsystem packages | Engine is generic — it works with any Subsystem implementation |
| Bus never imports subsystem or plugin packages | Bus is content-agnostic — it moves bytes, not BGP messages |
| Config Manager never imports subsystem packages | Config Manager serves opaque trees — subsystems interpret them |
| Plugin Manager never imports subsystem packages | Plugin Manager handles lifecycle — subsystems handle protocol |
| Subsystems never import each other | Subsystems communicate only through the bus |
| Plugins never import subsystem internals | Plugins use the SDK + bus, not reactor internals |
| Only `pkg/ze/` defines the boundary interfaces | Single source of truth for all contracts; public so external plugins can depend on them |

### DirectBridge Optimization

DirectBridge is an implementation detail of the Bus for in-process plugins. When a plugin runs as a goroutine (internal mode, `ze.X` prefix), the Bus implementation can detect this and deliver events via function call instead of socket I/O. This is invisible to both publisher and subscriber — they use the same Bus interface.

| Transport | When | How |
|-----------|------|-----|
| Socket + JSON | External plugins (forked processes) | Serialize → write to socket → deserialize |
| Socket + text | Text-mode external plugins | Format → write line → parse |
| DirectBridge | Internal plugins (goroutines) | Function call with structured data, no serialization |

The Bus implementation decides which transport to use based on the plugin's process type. Neither the publisher nor the subscriber knows or cares.

## Data Flow (MANDATORY)

### Entry Point
- BGP wire bytes arrive on TCP connections (external peers)
- API commands arrive on Unix socket (CLI / external tools)
- Config arrives from file (startup) or SIGHUP (reload)
- Plugin responses arrive on bus topics (runtime)

### Transformation Path
1. Wire read — TCP raw bytes into session buffer (BGP Subsystem wire layer)
2. Message parse — raw bytes dispatched by message type (BGP Subsystem message layer)
3. Lazy parse — UPDATE becomes WireUpdate with zero-copy iterators (BGP Subsystem wireu layer)
4. Format — WireUpdate formatted to payload bytes: JSON, text, or hex (BGP Subsystem format layer)
5. Publish — payload + metadata sent to Bus (BGP Subsystem → Bus)
6. Match — Bus matches topic + filter to find subscribers (Bus)
7. Deliver — Bus calls consumer's Deliver method with payload (Bus → Plugin)
8. Decode — Plugin decodes payload into its internal representation (Plugin / SDK)
9. Process — Plugin stores in RIB, computes best-path, etc. (Plugin)
10. Respond — Plugin publishes command payload back through Bus to BGP Subsystem (Plugin → Bus → BGP Subsystem)

### Boundaries Crossed

| Boundary | Mechanism | Content |
|----------|-----------|---------|
| TCP ↔ BGP Subsystem | Session buffer, raw bytes | Wire format BGP messages |
| BGP Subsystem ↔ Bus | `bus.Publish()` / `consumer.Deliver()` | Opaque payload bytes + string metadata |
| Bus ↔ Plugin | `consumer.Deliver()` | Same opaque payload bytes |
| Plugin ↔ Bus (return) | `bus.Publish()` on command topic | Opaque command payload |
| Bus ↔ BGP Subsystem (return) | `consumer.Deliver()` | Same opaque command payload |
| Engine ↔ Config Manager | `ConfigProvider` interface | `map[string]any` config trees |
| Plugin Manager ↔ Plugin | 5-stage RPC protocol | JSON-RPC over sockets (startup only) |

### Integration Points
- `reactor.notifyMessageReceiver()` — current entry point where BGP messages enter the plugin system; becomes `bus.Publish()`
- `Server.Subscriptions().GetMatching()` — current subscription lookup; moves into Bus implementation
- `Process.deliveryLoop()` — current event delivery goroutine; becomes Bus consumer adapter
- `BGPHooks.OnMessageReceived` — current callback injection; eliminated, BGP publishes to bus directly
- `Server.deliverConfigToProcess()` — current Stage 2 config delivery; moves to Plugin Manager via Config Manager
- `ReactorLifecycle` interface — current boundary between generic infra and BGP; evolves into Subsystem interface
- `plugin.Hub.RouteCommand()` — current config command routing; absorbed into Config Manager

### Architectural Verification
- [ ] No bypassed layers — events flow through bus, not direct function calls between subsystem and plugins
- [ ] No unintended coupling — bus never imports BGP types
- [ ] No duplicated functionality — subscription matching exists once, in bus
- [ ] Zero-copy preserved — WireUpdate references passed through DirectBridge, payload bytes passed through socket path

## Migration Phases

This is an umbrella spec. Each phase is a separate child spec.

### Phase 1 — Define interfaces (`spec-arch-1-interfaces.md`)

Create the interface package with all five interfaces. No implementation changes. All existing code continues to work unchanged.

| Deliverable | Location |
|-------------|----------|
| `Engine` interface | `pkg/ze/engine.go` |
| `Bus` interface | `pkg/ze/bus.go` |
| `ConfigProvider` interface | `pkg/ze/config.go` |
| `PluginManager` interface | `pkg/ze/plugin.go` |
| `Subsystem` interface | `pkg/ze/subsystem.go` |
| `Consumer`, `Event`, `Topic`, `Subscription` types | `pkg/ze/bus.go` |

### Phase 2 — Extract Bus from `plugin.Server` (`spec-arch-2-bus.md`)

Move `SubscriptionManager`, subscription matching, and `Process.deliveryLoop()` into a `Bus` implementation. `plugin.Server` delegates to Bus instead of managing subscriptions directly.

| What moves | From | To |
|-----------|------|-----|
| `SubscriptionManager` | `internal/component/plugin/subscribe.go` | Bus implementation |
| Subscription matching | `Subscription.Matches()` | Bus implementation |
| Delivery loop | `Process.deliveryLoop()` | Bus consumer adapter |
| Event type constants | `internal/component/plugin/events.go` | BGP subsystem (or deleted — subsystems own their topic names) |

### Phase 3 — Extract Plugin Manager (`spec-arch-3-plugin-manager.md`)

Move `ProcessManager`, `StartupCoordinator`, `PluginRegistry`, `CapabilityInjector`, and the 5-stage protocol out of `plugin.Server` into a `PluginManager` implementation.

### Phase 4 — Extract Config Manager (`spec-arch-4-config-manager.md`)

Wrap `internal/config/` behind `ConfigProvider`. Move config command routing (`plugin.Hub`) and Stage 2 config delivery into Config Manager. Subsystems and plugins both use the same interface.

### Phase 5 — Create Engine supervisor (`spec-arch-5-engine.md`)

Create `Engine` implementation that composes Bus + PluginManager + ConfigProvider. Replace `hub.Orchestrator` and the startup sequence in `cmd/ze/hub/main.go`. BGP reactor implements `Subsystem` and registers with Engine.

### Phase 6 — Eliminate BGPHooks (`spec-arch-6-eliminate-hooks.md`)

BGP subsystem publishes to bus directly instead of through `BGPHooks` callback injection. `internal/component/plugin/` no longer has any BGP-specific code. This is the final decoupling.

### Phase Summary

| Phase | What | Risk | Behavior Change |
|-------|------|------|-----------------|
| 1 | Define interfaces | None — new files only | None |
| 2 | Extract Bus | Medium — moves subscription logic | None — same behavior through interface |
| 3 | Extract Plugin Manager | Medium — moves lifecycle logic | None — same 5-stage protocol |
| 4 | Extract Config Manager | Low — wraps existing config code | None — same config delivery |
| 5 | Create Engine | High — new startup sequence | Startup order may change |
| 6 | Eliminate BGPHooks | Medium — changes event publishing | None — same events delivered |

Each phase ends with `make ze-verify` passing. No phase changes observable behavior. All existing tests continue to pass throughout.

## Wiring Test (MANDATORY — NOT deferrable)

Wiring tests are defined per child spec. The umbrella requires:

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Engine.Start() | → | Bus created, subsystems started | `TestEngineStartsSubsystem` (Phase 5) |
| Subsystem publishes event | → | Plugin receives via Bus | `TestBusDeliversToSubscriber` (Phase 2) |
| Config reload (SIGHUP) | → | Subsystem receives new config | `TestConfigReloadReachesSubsystem` (Phase 4) |
| Plugin 5-stage startup | → | Plugin connected to Bus | `TestPluginStartupConnectsToBus` (Phase 3) |
| BGP UPDATE received | → | Plugin receives via Bus (no BGPHooks) | `TestBGPUpdateFlowsThroughBus` (Phase 6) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Bus receives `Publish()` call with topic + payload | All matching subscribers receive the payload via `Deliver()` |
| AC-2 | Subscriber with prefix `bgp/` | Receives events published to `bgp/update`, `bgp/events/peer-up`, and all `bgp/*` topics |
| AC-3 | Subscriber with prefix `bgp/update` | Receives only events published to `bgp/update`, not `bgp/events/peer-up` |
| AC-4 | `ConfigProvider.Get("bgp")` called by subsystem | Returns the BGP config subtree as `map[string]any` |
| AC-5 | Plugin completes 5-stage startup | Plugin is connected to Bus and can subscribe/publish |
| AC-6 | Engine.Start() called with BGP subsystem registered | Bus created → plugins started → BGP subsystem started → events flow |
| AC-7 | `internal/component/plugin/` package | Zero imports from `internal/component/bgp/` or any subsystem package |
| AC-8 | Bus implementation | Zero type assertions on event payload — payload is `[]byte`, never cast |
| AC-9 | DirectBridge in-process plugin | Same Bus interface, events delivered via function call (no serialization) |
| AC-10 | SIGHUP sent to process | Config Manager re-reads file, subsystem receives updated config via `Reload()` |

## Implementation Steps

This is an umbrella spec — implementation steps are in child specs. Overall sequence:

1. Write child spec for Phase 1 (interfaces) — get approval
2. Implement Phase 1 — define interfaces, no behavior changes
3. Write child spec for Phase 2 (Bus) — get approval
4. Implement Phase 2 — extract Bus from Server
5. Continue through Phases 3-6

Each phase follows the standard TDD cycle: write tests → verify fail → implement → verify pass → audit.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Interface doesn't fit existing code | Revise interface in Phase 1 spec |
| Subscription semantics change | Back to Phase 2 design |
| 5-stage protocol breaks | Back to Phase 3 design |
| Config delivery breaks | Back to Phase 4 design |
| Startup order changes behavior | Back to Phase 5 design |
| BGP events stop flowing | Back to Phase 6 design |
| Any `make ze-verify` failure | Fix before proceeding to next phase |

## Design Insights

- The Bus's content-agnosticism is the single most important property. If the bus ever type-asserts a payload, the architecture is broken. This is the invariant that makes new subsystems possible without touching the bus.
- The 5-stage protocol is startup choreography, not runtime communication. Keeping it separate from the bus means the bus can be simple (pub/sub only) and the plugin manager can be complex (protocol state machine) without either polluting the other.
- DirectBridge is a transport optimization, not an architectural concept. It belongs inside the bus implementation, invisible to publishers and subscribers.
- "Subsystem vs plugin" is not a type hierarchy — they don't share an interface. A subsystem owns I/O and creates topics. A plugin reacts to topics. They're fundamentally different roles that both happen to use the bus.

## Resolved Design Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Where do interfaces live? | `pkg/ze/` | Public package — external plugins and tools can depend on these interfaces |
| Topic naming convention | Hierarchical (`bgp/update`, `bgp/events/peer-up`) | Restricts routing to zones (broadcast domains); prefix-based filtering natural |
| Event payload format | Always `[]byte` | Content-agnostic — bus never interprets payload; DirectBridge optimization hidden inside bus implementation |
| Filter matching | Prefix match on topic name | Simple, sufficient: subscribe to `bgp/` gets all BGP topics; subscribe to `bgp/events/` gets peer-up and peer-down |
| Subscription model | Subscribe to multiple topic prefixes | One consumer can subscribe to `bgp/update` and `bgp/open` separately; or `bgp/` for all BGP topics |

## 🧪 TDD Test Plan

Tests are defined per child spec. The umbrella lists the top-level tests that prove the architecture works end-to-end.

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBusPublishDeliver` | `pkg/ze/bus_test.go` | Bus delivers published event to subscriber | Phase 2 |
| `TestBusFilterMatching` | `pkg/ze/bus_test.go` | Subscriber with filter only receives matching events | Phase 2 |
| `TestBusTopicIsolation` | `pkg/ze/bus_test.go` | Events on topic A do not reach topic B subscribers | Phase 2 |
| `TestPluginManagerFiveStage` | `pkg/ze/pluginmgr_test.go` | Plugin Manager runs 5-stage protocol correctly | Phase 3 |
| `TestConfigProviderGet` | `pkg/ze/config_test.go` | ConfigProvider returns correct subtree by root | Phase 4 |
| `TestConfigProviderWatch` | `pkg/ze/config_test.go` | ConfigProvider notifies on reload | Phase 4 |
| `TestEngineStartupOrder` | `pkg/ze/engine_test.go` | Engine starts components in correct order | Phase 5 |
| `TestEngineShutdownOrder` | `pkg/ze/engine_test.go` | Engine stops components in reverse order | Phase 5 |
| `TestSubsystemReceivesBus` | `pkg/ze/engine_test.go` | Subsystem receives Bus and ConfigProvider at Start | Phase 5 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-engine-startup` | `test/engine/startup.ci` | Engine starts BGP subsystem, plugins connect, events flow | Phase 5 |
| `test-bus-event-flow` | `test/engine/bus-flow.ci` | BGP UPDATE reaches plugin through bus | Phase 6 |
| `test-config-reload` | `test/engine/config-reload.ci` | SIGHUP triggers config reload, subsystem receives update | Phase 4 |

## Files to Modify

- `internal/component/plugin/server.go` — decompose into Bus + PluginManager (Phases 2, 3)
- `internal/component/plugin/types.go` — extract interfaces to `pkg/ze/` (Phase 1)
- `internal/component/plugin/subscribe.go` — move subscription logic to Bus (Phase 2)
- `internal/component/plugin/events.go` — move event constants to BGP subsystem (Phase 2)
- `internal/component/plugin/process.go` — extract lifecycle to PluginManager, delivery to Bus (Phases 2, 3)
- `internal/component/plugin/hub.go` — absorb into Config Manager (Phase 4)
- `internal/hub/hub.go` — replace with Engine (Phase 5)
- `internal/component/bgp/reactor/reactor.go` — implement Subsystem interface (Phase 5)
- `internal/component/bgp/server/hooks.go` — eliminate BGPHooks (Phase 6)
- `internal/component/bgp/server/events.go` — BGP publishes to bus directly (Phase 6)
- `cmd/ze/hub/main.go` — use Engine instead of manual startup (Phase 5)

## Files to Create

- `pkg/ze/engine.go` — Engine interface + types (Phase 1)
- `pkg/ze/bus.go` — Bus interface + Event/Topic/Subscription/Consumer types (Phase 1)
- `pkg/ze/config.go` — ConfigProvider interface (Phase 1)
- `pkg/ze/plugin.go` — PluginManager interface (Phase 1)
- `pkg/ze/subsystem.go` — Subsystem interface (Phase 1)
- `pkg/ze/bus_impl.go` — Bus implementation (Phase 2)
- `pkg/ze/pluginmgr_impl.go` — PluginManager implementation (Phase 3)
- `pkg/ze/config_impl.go` — ConfigProvider implementation (Phase 4)
- `pkg/ze/engine_impl.go` — Engine implementation (Phase 5)
- `test/engine/startup.ci` — engine startup functional test (Phase 5)
- `test/engine/bus-flow.ci` — bus event flow functional test (Phase 6)
- `test/engine/config-reload.ci` — config reload functional test (Phase 4)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`, `pkg/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec
