# Subsystem Wiring: Reactor as ze.Subsystem

> **Note (2026-04-24):** This document was written during the arch-0 migration.
> The startup path has since evolved: the plugin server uses topological tier
> ordering (`startup.go`), the coordinator owns config distribution, and the
> reactor starts via the BGP plugin's `OnStarted` hook rather than direct
> `LoadReactorWithPlugins`. The diagrams below describe the pre-migration state
> and the planned target; the live code in `cmd/ze/hub/main.go` is authoritative.

This document describes the migration from direct reactor startup to Engine-supervised
startup with Bus integration, completing the arch-0 component boundary work.

## Pre-Migration Architecture

The reactor was created and started directly by `cmd/ze/hub/main.go`. It held a
`*pluginserver.Server` and `*EventDispatcher` for plugin communication. The Engine,
Bus, and Subsystem interface existed but were not wired into the startup path.

```mermaid
flowchart TB
    subgraph CLI["cmd/ze/hub/main.go"]
        LR[LoadReactorWithPlugins]
        RS[reactor.Start]
        LR --> RS
    end

    subgraph Reactor["reactor.Reactor"]
        ED[EventDispatcher]
        PS[pluginserver.Server]
        MR[MessageReceiver]
        ED --> PS
        MR --> ED
    end

    subgraph Plugins["Plugin Processes"]
        P1[bgp-rib]
        P2[bgp-rs]
        P3[bgp-gr]
    end

    RS --> Reactor
    PS --> P1
    PS --> P2
    PS --> P3

    subgraph Unused["Built but NOT wired"]
        ENG[Engine]
        BUS[Bus]
        SUB[ze.Subsystem interface]
    end

    style Unused fill:#fee,stroke:#f66
```

### Current Event Flow (UPDATE hot path)

The EventDispatcher handles per-subscriber format negotiation. Each plugin process
has a format preference (text/json, parsed/raw). The dispatcher pre-formats once
per distinct format combination and delivers the right format to each process.

```mermaid
sequenceDiagram
    participant TCP as TCP Session
    participant R as Reactor
    participant ED as EventDispatcher
    participant SM as SubscriptionManager
    participant P1 as bgp-rib (DirectBridge)
    participant P2 as bgp-rs (text/json)
    participant P3 as external plugin (text/text)

    TCP->>R: raw UPDATE bytes
    R->>R: notifyMessageReceiver()
    R->>ED: OnMessageReceived(peer, RawMessage)
    ED->>SM: GetMatching(bgp, update, received, peer)
    SM-->>ED: [P1, P2, P3]

    Note over ED: Format negotiation:<br/>P1: DirectBridge (no formatting)<br/>P2: json+parsed<br/>P3: text+parsed

    ED->>ED: formatCache: pre-format per encoding
    ED->>P1: StructuredUpdate (zero-copy, pooled)
    ED->>P2: pre-formatted JSON string
    ED->>P3: pre-formatted text string
```

### Current Event Flow (peer state change)

State events are delivered sequentially in reverse dependency order.

```mermaid
sequenceDiagram
    participant R as Reactor
    participant ED as EventDispatcher
    participant SM as SubscriptionManager
    participant GR as bgp-gr (tier 2)
    participant RIB as bgp-rib (tier 1)

    R->>ED: OnPeerStateChange(peer, "down", reason)
    ED->>SM: GetMatching(bgp, state, "", peer)
    SM-->>ED: [bgp-gr, bgp-rib]
    Note over ED: Sort by reverse dependency tier
    ED->>GR: deliver (sequential, wait for result)
    GR-->>ED: done
    ED->>RIB: deliver (sequential, wait for result)
    RIB-->>ED: done
```

## Target Architecture

The reactor is wrapped in a `BGPSubsystem` adapter that implements `ze.Subsystem`.
The Engine supervises startup. The Bus is a **notification layer** for cross-component
signaling. The EventDispatcher keeps its existing direct calling convention for plugin
data delivery (format negotiation, cache consumer counts, DirectBridge zero-copy).

The key insight: **Bus is for signaling, not data transport.** The reactor publishes
lightweight notifications to Bus topics in parallel with the existing EventDispatcher
data path. Cross-component consumers (e.g., interface plugin) subscribe to Bus signals.
Plugins that need data access the reactor directly — they already have direct access
via pluginserver.Server and DirectBridge.

```mermaid
flowchart TB
    subgraph CLI["cmd/ze/hub/main.go"]
        BC[Build components]
        ES[engine.Start]
        BC --> ES
    end

    subgraph Engine["Engine (supervisor)"]
        BUS[Bus — notification layer]
        PM[PluginManager]
        BGP[BGPSubsystem adapter]
    end

    ES --> Engine

    subgraph BGPSub["BGPSubsystem wraps Reactor"]
        R[Reactor]
        ED[EventDispatcher — direct data path]
        R -->|direct call| ED
        R -.->|notification| BUS
    end

    BGP --> BGPSub

    subgraph Plugins["Plugin Processes — data via EventDispatcher"]
        P1[bgp-rib]
        P2[bgp-rs]
        P3[bgp-gr]
    end

    ED --> P1
    ED --> P2
    ED --> P3

    subgraph Future["Cross-component — signals via Bus"]
        IF[Interface Plugin]
        IF -.->|publishes interface/*| BUS
        BUS -.->|notifies| BGPSub
    end

    style Future fill:#efe,stroke:#6a6
```

### Target Event Flow (UPDATE hot path)

The EventDispatcher data path is unchanged — reactor calls it directly, it returns
the cache consumer count, format negotiation and DirectBridge work exactly as before.
In parallel, the reactor publishes a lightweight Bus notification so cross-component
consumers know an UPDATE arrived.

```mermaid
sequenceDiagram
    participant TCP as TCP Session
    participant R as Reactor
    participant ED as EventDispatcher
    participant BUS as Bus
    participant P1 as bgp-rib (DirectBridge)
    participant P2 as bgp-rs (text/json)
    participant XC as Cross-component consumer

    TCP->>R: raw UPDATE bytes
    R->>R: notifyMessageReceiver()
    R->>ED: OnMessageReceived(peer, RawMessage)
    Note over ED: Data path unchanged:<br/>format negotiation, DirectBridge,<br/>cache consumer count returned
    ED->>P1: StructuredUpdate (zero-copy)
    ED->>P2: pre-formatted JSON
    ED-->>R: consumerCount (int)
    R->>R: recentUpdates.Activate(id, count)

    R--)BUS: Publish("bgp/update", notification, metadata)
    Note over BUS: Lightweight signal only.<br/>No data in payload.
    BUS--)XC: Deliver (if subscribed)
```

### Target Event Flow (peer state change)

State events use the same dual-path pattern. EventDispatcher handles plugin delivery
with dependency ordering. Bus publishes a notification for cross-component consumers.

```mermaid
sequenceDiagram
    participant R as Reactor
    participant ED as EventDispatcher
    participant BUS as Bus
    participant GR as bgp-gr (tier 2)
    participant RIB as bgp-rib (tier 1)
    participant XC as Cross-component consumer

    R->>ED: OnPeerStateChange(peer, "down", reason)
    Note over ED: Dependency-ordered delivery unchanged
    ED->>GR: deliver (sequential)
    GR-->>ED: done
    ED->>RIB: deliver (sequential)
    RIB-->>ED: done

    R--)BUS: Publish("bgp/state", JSON, metadata)
    BUS--)XC: Deliver (if subscribed)
```

### Target Event Flow (cross-component: interface plugin)

The Bus enables cross-component communication without direct imports.

```mermaid
sequenceDiagram
    participant OS as Linux Kernel
    participant IF as Interface Plugin
    participant BUS as Bus
    participant BGP as BGPSubsystem (Consumer)

    OS->>IF: netlink RTM_NEWADDR
    IF->>BUS: Publish("interface/addr/added", JSON, metadata)
    BUS->>BGP: Deliver([]Event)
    BGP->>BGP: match peer LocalAddress
    BGP->>BGP: start listener on address
```

## Component Changes

### BGPSubsystem Adapter

A thin wrapper around `reactor.Reactor` satisfying `ze.Subsystem`:

```mermaid
classDiagram
    class Subsystem {
        <<interface>>
        +Name() string
        +Start(ctx, Bus, ConfigProvider) error
        +Stop(ctx) error
        +Reload(ctx, ConfigProvider) error
    }

    class BGPSubsystem {
        -reactor *Reactor
        -bus Bus
        +Name() string
        +Start(ctx, Bus, ConfigProvider) error
        +Stop(ctx) error
        +Reload(ctx, ConfigProvider) error
    }

    class Reactor {
        +StartWithContext(ctx) error
        +Stop()
        +Wait(ctx) error
        +Reload(configPath) error
    }

    Subsystem <|.. BGPSubsystem : implements
    BGPSubsystem --> Reactor : wraps
    BGPSubsystem --> Bus : publishes/subscribes
```

### Startup Sequence

```mermaid
sequenceDiagram
    participant CLI as cmd/ze/hub
    participant CFG as Config Loader
    participant ENG as Engine
    participant BUS as Bus
    participant PM as PluginManager
    participant BGP as BGPSubsystem

    CLI->>CFG: LoadReactorWithPlugins()
    CFG-->>CLI: reactor + config

    CLI->>BUS: NewBus()
    CLI->>ENG: NewEngine(bus, config, plugins)
    CLI->>ENG: RegisterSubsystem(BGPSubsystem)
    CLI->>ENG: Start(ctx)

    ENG->>PM: StartAll(ctx, bus, config)
    PM-->>ENG: plugins ready

    ENG->>BGP: Start(ctx, bus, config)
    BGP->>BGP: store bus reference
    BGP->>BGP: create Bus topics
    BGP->>BGP: subscribe EventDispatcher to bus
    BGP->>BGP: reactor.StartWithContext(ctx)
    BGP-->>ENG: started

    Note over CLI: Signal handling (SIGTERM/SIGHUP)

    CLI->>ENG: Stop(ctx)
    ENG->>BGP: Stop(ctx)
    BGP->>BGP: reactor.Stop()
    BGP->>BGP: reactor.Wait()
    ENG->>PM: StopAll(ctx)
```

## Bus as Notification Layer

The Bus is a **signaling mechanism**, not a data transport. Plugin data delivery stays
on the existing EventDispatcher direct path. The Bus publishes lightweight notifications
so cross-component consumers can react to events without importing BGP internals.

### Bus Payload

All Bus payloads are JSON-encoded notifications with minimal information:

| Field | Type | Purpose |
|-------|------|---------|
| `peer` | string | Peer address |
| `event` | string | Event type (update, state, eor, etc.) |

Additional fields per event type (e.g., `state`, `reason`, `family`).

### Why Not Data Transport?

The EventDispatcher returns cache consumer counts, handles per-subscriber format
negotiation, manages DirectBridge zero-copy delivery, and enforces dependency-ordered
delivery for state events. These are tightly coupled to synchronous calling conventions.
The Bus is fire-and-forget — it cannot return values or enforce ordering.

Plugins that need UPDATE data already have direct access via `pluginserver.Server`
and DirectBridge. Cross-component consumers (like the interface plugin) only need
signals ("a peer went down") to react — they don't need the raw UPDATE bytes.

## Bus Topics

The BGP subsystem creates these topics at startup:

| Topic | Published when | Payload |
|-------|---------------|---------|
| `bgp/update` | UPDATE received or sent | update-id reference |
| `bgp/state` | Peer state change (up/down) | JSON: peer, state, reason |
| `bgp/negotiated` | Capability negotiation complete | JSON: peer, capabilities |
| `bgp/eor` | End-of-RIB marker detected | JSON: peer, family |
| `bgp/congestion` | Forward path congestion change | JSON: peer, event-type |

> **See also:** [Config Transaction Protocol](config/transaction-protocol.md) for `config/`
> bus topics used during verify/apply/rollback of config changes.

## Migration Path

The migration preserves all existing behavior while adding Bus integration:

1. **BGPSubsystem adapter** -- wraps reactor, implements ze.Subsystem
2. **Wire startup through Engine** -- cmd/ze/hub/main.go uses Engine.Start()
3. **Reactor publishes Bus notifications** -- in parallel with existing EventDispatcher calls
4. **Existing tests continue to pass** -- behavior unchanged, only startup plumbing changes

### What Changes

| Before | After |
|--------|-------|
| `reactor.Start()` called directly | `engine.Start()` calls `BGPSubsystem.Start()` |
| No Bus reference in reactor | Reactor holds Bus, publishes notifications |
| No cross-component events | Bus enables interface/addr events for BGP |

### What Does NOT Change

| Concern | Status |
|---------|--------|
| EventDispatcher direct calling convention | Unchanged — reactor calls ED methods directly |
| Format negotiation per subscriber | Unchanged |
| DirectBridge zero-copy for in-process plugins | Unchanged |
| StructuredUpdate pooling | Unchanged |
| Cache consumer count tracking | Unchanged — returned synchronously from ED |
| Dependency-ordered state delivery | Unchanged |
| Subscription matching semantics | Unchanged |
| Plugin 5-stage startup protocol | Unchanged |
| SIGHUP config reload | Unchanged (routed through Engine.Reload) |
