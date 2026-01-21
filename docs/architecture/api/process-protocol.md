# External Process Protocol

**Source:** ExaBGP `reactor/api/processes.py`
**Purpose:** Document external process communication protocol

---

## Overview

ExaBGP communicates with external processes via stdin/stdout:
- **Events → stdin:** BGP events (JSON) written to process stdin
- **Commands ← stdout:** API commands read from process stdout

---

## Process Configuration

```
process announce-routes {
    run /usr/bin/python3 /path/to/script.py;
    encoder json;
}

peer 192.168.1.2 {
    api {
        processes [ announce-routes ];
        receive {
            update;
            open;
            notification;
        }
    }
}
```

---

## Process Lifecycle

### Startup

1. ExaBGP spawns process using configured `run` command
2. Process inherits environment variables
3. stdin/stdout set to non-blocking mode
4. Process added to poll/select set

### Communication

1. Events written to process stdin (newline-delimited JSON)
2. Commands read from process stdout (newline-delimited)
3. Each command processed and acknowledged (if ACK enabled)

### Plugin-Driven Config Parsing (Future)

Plugins can extend the config schema. This requires a two-phase config parsing approach:

```
CONFIG FILE                     ENGINE                          PLUGINS
───────────                     ──────                          ───────

plugin {                 →      1. Parse plugin blocks ONLY
  external gr { ... }           2. Start plugin processes  →    Started
  external rib { ... }
}
                                3. Wait for schema hooks   ←    declare conf schema capability
                                                                  graceful-restart { restart-time <\d+>; }
                                                           ←    declare done
                                ─── CONFIG PARSING BARRIER ───
peer 127.0.0.1 {         →      4. Parse rest of config
  capability {                     (using plugin-extended schema)
    graceful-restart {
      restart-time 120;         5. Deliver matching config →    config peer 127.0.0.1
    }                                                             graceful-restart restart-time 120
  }
}
                                6. Continue normal stages  →    (capability injection, ready)
```

**Key principle:** ZeBGP engine has NO hardcoded knowledge of capability-specific config
(like `graceful-restart`). Plugins define their own config schema via `declare conf schema`.

**Benefits:**
- Polyglot plugins: Any language can implement capability plugins
- No engine changes for new capabilities
- Config schema is self-documenting via plugin declarations

### 5-Stage Startup Protocol (ZeBGP)

ZeBGP uses a synchronized 5-stage startup protocol with barriers between stages.
All plugins must complete each stage before any can proceed to the next.

```
┌──────────────────────────────────────────────────────────────────────────┐
│                                STARTUP TIMELINE                          │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│        Plugin A          Coordinator           Plugin B                  │
│        ─────────         ───────────           ─────────                 │
│                                                                          │
│        STAGE 1: REGISTRATION                                             │
│        declare cmd ...        │                 declare cmd ...          │
│        declare conf ...       │                 declare conf ...         │
│        declare done ──────────┼────────────────► declare done            │
│             │                 │                      │                   │
│             ▼                 │                      ▼                   │
│        StageComplete(0,Reg)   │            StageComplete(1,Reg)          │
│             │                 │                      │                   │
│             ▼                 │                      │                   │
│        WaitForStage(Config)   │                      │                   │
│             │ ◄───────────────┼── BARRIER ──────────►│                   │
│             │         (all plugins complete Reg)     │                   │
│                                                                          │
│        STAGE 2: CONFIG DELIVERY                                          │
│             │                 │                      │                   │
│             ▼                 │                      ▼                   │
│        ◄── config peer ...    │             config peer ... ──►          │
│        ◄── config done        │                   config done ──►        │
│             │                 │                      │                   │
│             ▼                 │                      ▼                   │
│        StageComplete(0,Cfg)   │            StageComplete(1,Cfg)          │
│             │                 │                      │                   │
│             ▼                 │                      │                   │
│        WaitForStage(Cap)      │                      │                   │
│             │ ◄───────────────┼── BARRIER ──────────►│                   │
│             │         (all plugins complete Cfg)     │                   │
│                                                                          │
│        STAGE 3: CAPABILITY DECLARATION                                   │
│             │                 │                      │                   │
│             ▼                 │                      ▼                   │
│        capability hex 64 ...  │             capability hex 64 ...        │
│        capability done ───────┼────────────────► capability done         │
│             │                 │                      │                   │
│             ▼                 │                      ▼                   │
│        StageComplete(0,Cap)   │            StageComplete(1,Cap)          │
│             │ ◄───────────────┼── BARRIER ──────────►│                   │
│                                                                          │
│        STAGE 4: REGISTRY SHARING                                         │
│             │                 │                      │                   │
│             ▼                 │                      ▼                   │
│        ◄── registry cmd ...   │             registry cmd ... ──►         │
│        ◄── registry done      │                 registry done ──►        │
│             │                 │                      │                   │
│             ▼                 │                      ▼                   │
│        StageComplete(0,Reg)   │            StageComplete(1,Reg)          │
│             │ ◄───────────────┼── BARRIER ──────────►│                   │
│                                                                          │
│        STAGE 5: READY                                                    │
│             │                 │                      │                   │
│             ▼                 │                      ▼                   │
│        ready ─────────────────┼────────────────► ready                   │
│             │                 │                      │                   │
│             ▼                 │                      ▼                   │
│        StageComplete(0,Ready) │            StageComplete(1,Ready)        │
│             │ ◄───────────────┼── BARRIER ──────────►│                   │
│             │         (all plugins ready)            │                   │
│             ▼                 │                      ▼                   │
│        [BGP peers start]      │                [BGP peers start]         │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

**Barrier Semantics:**
- Each plugin signals stage completion via `StageComplete(pluginID, stage)`
- Coordinator waits until ALL plugins complete the current stage
- Only then does coordinator advance to next stage
- All waiting plugins unblock simultaneously

**Stage Commands:**

| Stage | Plugin → ZeBGP | ZeBGP → Plugin |
|-------|----------------|----------------|
| 1. Registration | `declare cmd/conf/receive/...`, `declare done` | - |
| 2. Config | - | `config peer <addr> <key> <value>`, `config done` |
| 3. Capability | `capability hex <code> <value> [peer <addr>]`, `capability done` | - |
| 4. Registry | - | `registry cmd <name>`, `registry done` |
| 5. Ready | `ready` | - |

**Timeout:** Each stage has a 5-second timeout (configurable via `stage-timeout` in plugin config).
If any plugin fails to complete a stage, startup aborts for all plugins.

**Why Barriers:**
- Ensures all plugins register commands before any receive config
- Ensures all capabilities declared before registry shared
- Prevents race conditions in multi-plugin configurations
- Guarantees consistent state before BGP peers start

### Shutdown

1. ExaBGP closes stdin
2. Waits for process to exit
3. Kills if not responsive

### Respawn

If `api.respawn = true` (default):
- Process respawned on unexpected exit
- Maximum 5 respawns per minute
- If exceeded, process disabled until reload

---

## Event Format (stdin)

Events are written as single-line JSON:

```json
{"exabgp":"6.0.0","time":1234567890.123,...}\n
```

### Event Types

| Type | Trigger | API Config Key |
|------|---------|----------------|
| state | up/down/connected | neighbor-changes |
| update | UPDATE received | receive.update |
| open | OPEN received | receive.open |
| keepalive | KEEPALIVE received | receive.keepalive |
| notification | NOTIFICATION received | receive.notification |
| refresh | ROUTE-REFRESH received | receive.refresh |
| operational | Operational message | receive.operational |
| negotiated | Capabilities negotiated | (always) |
| fsm | FSM state change | api.fsm |

### Filtering

Only events matching API configuration are sent:

```
api {
    receive {
        parsed;          # Parsed updates (not raw)
        update;          # UPDATE messages
        notification;    # NOTIFICATION messages
    }
}
```

---

## Command Format (stdout)

Commands are newline-delimited text:

```
update text nhop set 192.168.1.1 nlri ipv4/unicast add 10.0.0.0/8
update text nlri ipv4/unicast del 10.0.0.0/8
```

### Command Processing

1. Line read from stdout
2. Tokenized and dispatched
3. Executed against matching peers
4. Acknowledged (if enabled)

### Command Serial (ACK Control)

ACK is controlled by `#N` serial prefix on commands:

```
# No serial = fire-and-forget (no response)
update text nhop set 192.168.1.1 nlri ipv4/unicast add 10.0.0.0/8

# With serial = get JSON response
#1 update text nhop set 192.168.1.1 nlri ipv4/unicast add 10.0.0.0/8
```

**Response format:**

Success:
```json
{"serial":"1","status":"done"}
{"serial":"2","status":"done","data":{"routes":5}}
```

Error:
```json
{"serial":"3","status":"error","data":"invalid command"}
```

**Process controls serial numbering.** ZeBGP echoes serial back for correlation.

---

## Write Queue / Backpressure

### Problem

Slow processes can cause memory growth if events accumulate.

### Solution

```python
WRITE_QUEUE_HIGH_WATER = 1000  # Pause at 1000 queued
WRITE_QUEUE_LOW_WATER = 100    # Resume at 100 queued
```

When queue exceeds high water mark:
1. Events dropped for this process
2. Warning logged
3. Resumes when queue drains to low water

---

## Non-Blocking I/O

### Setting Non-Blocking

```python
import fcntl
import os

def set_nonblocking(fd):
    flags = fcntl.fcntl(fd, fcntl.F_GETFL)
    fcntl.fcntl(fd, fcntl.F_SETFL, flags | os.O_NONBLOCK)
```

### Handling EAGAIN

```python
try:
    data = stdin.read(4096)
except IOError as e:
    if e.errno == errno.EAGAIN:
        return  # No data available
    raise
```

---

## Async Mode

ExaBGP supports asyncio operation:

```python
class Processes:
    async def write_async(self, service: str, data: bytes) -> None:
        # Queue write
        self._write_queue[service].append(data)
        # Flush later
        await self.flush_write_queue()

    async def flush_write_queue(self) -> None:
        for service, queue in self._write_queue.items():
            while queue:
                data = queue.popleft()
                await self._write_to_process(service, data)
```

---

## Process Isolation

Processes run in separate process group:

```python
def preexec_helper():
    os.setpgrp()  # New process group
```

This prevents:
- SIGINT propagation to children
- Signal interference between ExaBGP and processes

---

## Environment Variables

Processes inherit:
- All `exabgp.*` environment variables
- Standard PATH, HOME, etc.
- Process-specific config values

---

## Error Handling

### Process Crash

1. Detected via SIGCHLD or EOF on stdout
2. If respawn enabled and limit not exceeded: respawn
3. Else: mark process as failed

### Write Error (Broken Pipe)

1. EPIPE/SIGPIPE caught
2. Process marked as dead
3. Respawn triggered if enabled

### Read Error

1. EOF indicates process exit
2. Trigger respawn if enabled

---

## ZeBGP Implementation Notes

### Process Manager

```go
type ProcessManager struct {
    processes map[string]*Process
    writeQueues map[string]chan []byte
}

type Process struct {
    name    string
    cmd     *exec.Cmd
    stdin   io.WriteCloser
    stdout  io.ReadCloser
    running bool
}

func (pm *ProcessManager) Write(name string, data []byte) error {
    select {
    case pm.writeQueues[name] <- data:
        return nil
    default:
        // Queue full, drop event
        return ErrQueueFull
    }
}
```

### Event Writer Goroutine

```go
func (p *Process) writeLoop(queue <-chan []byte) {
    for data := range queue {
        _, err := p.stdin.Write(append(data, '\n'))
        if err != nil {
            // Handle broken pipe
            return
        }
    }
}
```

### Command Reader Goroutine

```go
func (p *Process) readLoop(commands chan<- string) {
    scanner := bufio.NewScanner(p.stdout)
    for scanner.Scan() {
        commands <- scanner.Text()
    }
}
```

### Respawn Logic

```go
func (pm *ProcessManager) respawn(name string) error {
    p := pm.processes[name]

    // Check respawn limit
    now := time.Now()
    if now.Sub(p.lastRespawn) < time.Minute {
        p.respawnCount++
        if p.respawnCount > 5 {
            return ErrRespawnLimit
        }
    } else {
        p.respawnCount = 1
    }
    p.lastRespawn = now

    return pm.start(name)
}
```

---

## Plugin Command Registration

External processes can register custom commands that extend ZeBGP's API.

### Registration Protocol

**Process → ZeBGP (stdout):**
```
#N register command "<name>" description "<help>" [args "<usage>"] [completable] [timeout <duration>]
```

**ZeBGP → Process (stdin):**
```json
{"serial":"N","status":"done"}
{"serial":"N","status":"error","data":"conflicts with builtin: ..."}
```

### Registration Options

| Option | Description |
|--------|-------------|
| `description` | Help text (required) |
| `args` | Usage hint (e.g., `"<component>"`) |
| `completable` | Process handles argument completion |
| `timeout` | Per-command timeout (default 30s) |

### Unregistration

```
#N unregister command "<name>"
```

### Command Execution

**ZeBGP → Process (stdin):**
```json
{"serial":"a","type":"request","command":"myapp status","args":["component"],"peer":"*"}
```

**Process → ZeBGP (stdout):**
```
@a done {"status": "running"}
@a error "component not found"
```

### Streaming Responses

For large outputs, send partial responses:
```
@a+ {"chunk": 1, "data": [...]}
@a+ {"chunk": 2, "data": [...]}
@a done
```

Partials reset the timeout timer. JSON responses include `"partial": true`.

### Argument Completion

If registered with `completable`, process receives completion requests:

**ZeBGP → Process:**
```json
{"serial":"b","type":"complete","command":"myapp copy","args":["file1"],"partial":"f"}
```

**Process → ZeBGP:**
```
@b done {"completions":[{"value":"file2","help":"Second file"}]}
```

Completion timeout: 500ms (non-configurable).

### Lifecycle

- On process death: all commands auto-unregistered, pending requests cancelled
- Commands must be lowercase, no quotes in names
- Cannot shadow builtin commands

### Files

| File | Purpose |
|------|---------|
| `internal/plugin/registry.go` | CommandRegistry type |
| `internal/plugin/pending.go` | PendingRequests tracker |
| `internal/plugin/plugin.go` | Parse register/unregister/response |
| `internal/plugin/server.go` | handleRegisterCommand, handlePluginResponse |

---

## Plugin Examples: RIB and GR

This section shows concrete message flows for the built-in RIB and GR plugins.

### GR Plugin (Capability-Only)

The GR plugin only participates in startup - it injects GR capabilities into OPEN messages.
**No process binding required** because it doesn't need runtime events.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        GR PLUGIN MESSAGE FLOW                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ZeBGP Engine                              GR Plugin (ze bgp plugin gr)     │
│   ────────────                              ────────────────────────────    │
│                                                                             │
│   STAGE 1: REGISTRATION                                                     │
│                                                                             │
│                              ◄───── declare conf peer * capability          │
│                                      graceful-restart:restart-time          │
│                                      <restart-time:\d+>                     │
│                              ◄───── declare done                            │
│                                                                             │
│   STAGE 2: CONFIG DELIVERY                                                  │
│                                                                             │
│   config peer 192.168.1.1    ─────►                                         │
│     restart-time 120                 (capture NAME, not full key)           │
│   config peer 10.0.0.1       ─────►                                         │
│     restart-time 90                                                         │
│   config done                ─────►                                         │
│                                        (plugin parses, stores in grConfig)  │
│                                                                             │
│   STAGE 3: CAPABILITY DECLARATION                                           │
│                                                                             │
│                              ◄───── capability hex 64 0078 peer 192.168.1.1 │
│                              ◄───── capability hex 64 005a peer 10.0.0.1    │
│                              ◄───── capability done                         │
│                                                                             │
│   (Engine stores: peer 192.168.1.1 gets GR cap [0x00,0x78] = 120s)          │
│   (Engine stores: peer 10.0.0.1 gets GR cap [0x00,0x5a] = 90s)              │
│                                                                             │
│   STAGE 4: REGISTRY SHARING                                                 │
│                                                                             │
│   registry done              ─────►                                         │
│                                                                             │
│   STAGE 5: READY                                                            │
│                                                                             │
│                              ◄───── ready                                   │
│                                                                             │
│   ═══════════════════════════════════════════════════════════════════════   │
│   BGP PEERS START - GR capability included in OPEN messages                 │
│   ═══════════════════════════════════════════════════════════════════════   │
│                                                                             │
│   RUNTIME: (minimal - just waits for shutdown)                              │
│                                                                             │
│   (stdin closed)             ─────►  (plugin exits cleanly)                 │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Wire format:** `capability hex 64 XXXX peer <addr>`
- Code 64 = Graceful Restart (RFC 4724)
- XXXX = 2-byte hex: `[R:1][Reserved:3][RestartTime:12]`
- Example: `0078` = restart-time 120 (0x78 = 120)

**Config delivery format:**
- Pattern key: `graceful-restart:restart-time` (matches against server path)
- Config delivery: `config peer <addr> restart-time <value>` (capture NAME only)
- Plugin parses the capture name, not the full pattern key

**Key insight:** GR plugin receives config for ALL peers with GR capability configured,
regardless of explicit `process gr {}` bindings. This is because `deliverConfig()` in
`internal/plugin/server.go` iterates all peers and matches against declared patterns.

### RIB Plugin (Full Lifecycle)

The RIB plugin tracks routes and replays them on peer reconnect.
**Requires process binding** for runtime events.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        RIB PLUGIN MESSAGE FLOW                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ZeBGP Engine                              RIB Plugin (ze bgp plugin rib)   │
│   ────────────                              ─────────────────────────────   │
│                                                                             │
│   STAGE 1: REGISTRATION                                                     │
│                                                                             │
│                              ◄───── declare cmd rib adjacent status         │
│                              ◄───── declare cmd rib adjacent inbound show   │
│                              ◄───── declare cmd rib adjacent inbound empty  │
│                              ◄───── declare cmd rib adjacent outbound show  │
│                              ◄───── declare cmd rib adjacent outbound resend│
│                              ◄───── declare done                            │
│                                                                             │
│   STAGE 2: CONFIG DELIVERY                                                  │
│                                                                             │
│   config done                ─────►  (RIB has no config patterns)           │
│                                                                             │
│   STAGE 3: CAPABILITY DECLARATION                                           │
│                                                                             │
│                              ◄───── capability done (RIB has no caps)       │
│                                                                             │
│   STAGE 4: REGISTRY SHARING                                                 │
│                                                                             │
│   registry cmd peer ...      ─────►                                         │
│   registry cmd update ...    ─────►                                         │
│   registry done              ─────►                                         │
│                                                                             │
│   STAGE 5: READY                                                            │
│                                                                             │
│                              ◄───── ready                                   │
│                                                                             │
│   ═══════════════════════════════════════════════════════════════════════   │
│   BGP PEERS START                                                           │
│   ═══════════════════════════════════════════════════════════════════════   │
│                                                                             │
│   RUNTIME: Event Processing                                                 │
│                                                                             │
│   ─── Peer comes up ───                                                     │
│                                                                             │
│   {"type":"state",           ─────►  (plugin marks peer as up)              │
│    "peer":"192.168.1.1",                                                    │
│    "state":"up"}                                                            │
│                                                                             │
│   ─── Route sent to peer ───                                                │
│                                                                             │
│   {"type":"sent",            ─────►  (plugin stores in ribOut)              │
│    "peer":"192.168.1.1",                                                    │
│    "msg-id":123,                                                            │
│    "ipv4/unicast":[...]}                                                    │
│                                                                             │
│   ─── Peer goes down ───                                                    │
│                                                                             │
│   {"type":"state",           ─────►  (plugin clears ribIn for peer)         │
│    "peer":"192.168.1.1",                                                    │
│    "state":"down"}                                                          │
│                                                                             │
│   ─── Peer reconnects ───                                                   │
│                                                                             │
│   {"type":"state",           ─────►  (plugin replays ribOut)                │
│    "peer":"192.168.1.1",                                                    │
│    "state":"up"}                                                            │
│                                                                             │
│                              ◄───── peer 192.168.1.1 update text            │
│                                       nhop set 10.0.0.1 nlri ipv4/unicast   │
│                                       add 10.0.1.0/24                       │
│                              ◄───── #1 peer 192.168.1.1 session api ready   │
│                                                                             │
│   ─── Route refresh request ───                                             │
│                                                                             │
│   {"type":"refresh",         ─────►  (plugin sends BoRR, routes, EoRR)      │
│    "peer":"192.168.1.1",                                                    │
│    "afi":"ipv4",                                                            │
│    "safi":"unicast"}                                                        │
│                                                                             │
│                              ◄───── peer 192.168.1.1 borr ipv4/unicast      │
│                              ◄───── peer 192.168.1.1 update text ...        │
│                              ◄───── peer 192.168.1.1 eorr ipv4/unicast      │
│                                                                             │
│   ─── Command request ───                                                   │
│                                                                             │
│   {"type":"request",         ─────►  (plugin handles command)               │
│    "serial":"abc",                                                          │
│    "command":"rib adjacent                                                  │
│              status"}                                                       │
│                                                                             │
│                              ◄───── @abc done {"running":true,              │
│                                               "peers":1,                    │
│                                               "routes_in":5,                │
│                                               "routes_out":3}               │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Process Binding Requirements

| Plugin Type | Process Binding | Why |
|-------------|-----------------|-----|
| Capability-only (GR) | Not required | Only needs config delivery (Stage 2) |
| Event-driven (RIB, RR) | Required | Needs runtime events (state, sent, refresh) |
| Command-only | Required | Needs request events for registered commands |

**Config example for both plugins:**

```
plugin {
    external gr {
        run "ze bgp plugin gr";
        encoder json;
    }

    external rib {
        run "ze bgp plugin rib";
        encoder json;
    }
}

peer 192.168.1.1 {
    capability {
        graceful-restart {
            restart-time 120;
        }
    }

    # No "process gr {}" needed - GR gets config automatically

    process rib {
        receive { sent; state; }
        send { update; }
    }
}
```

### Config Delivery vs Event Delivery

The engine uses two different mechanisms:

| Mechanism | When | Filter |
|-----------|------|--------|
| **Config Delivery** (Stage 2) | Startup only | Pattern matching against ALL peers |
| **Event Delivery** (Runtime) | After startup | Process bindings per peer |

**Config delivery** (`internal/plugin/server.go:deliverConfig`):
```go
peerConfigs := s.reactor.GetPeerCapabilityConfigs()  // ALL peers
for _, peerCfg := range peerConfigs {
    for _, pattern := range reg.ConfigPatterns {
        matches := matchConfigPattern(pattern, peerCfg)
        // Send config regardless of process binding
    }
}
```

**Event delivery** (`internal/plugin/server.go:dispatchEvent`):
```go
bindings := s.reactor.GetPeerProcessBindings(peer.Address)
for _, binding := range bindings {
    if binding.ShouldSend(eventType) {
        proc := s.GetProcess(binding.PluginName)
        proc.WriteEvent(event)
    }
}
```

This separation allows capability-only plugins to work without explicit process bindings,
while still requiring bindings for plugins that need runtime event filtering.

---

**Last Updated:** 2026-01-18
