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
                                3. Wait for declarations   ←    declare wants config bgp
                                                           ←    declare done
                                ─── CONFIG PARSING BARRIER ───
peer 127.0.0.1 {         →      4. Parse config (internal plugin
  capability {                     YANG auto-loaded by YANGSchema())
    graceful-restart {
      restart-time 120;         5. Deliver JSON config     →    config json bgp {"bgp":{...}}
    }
  }
}
                                6. Continue normal stages  →    (capability injection, ready)
```

**Key principle:** Internal plugins (GR, hostname, etc.) have their YANG schemas
automatically loaded via `YANGSchema()`. Plugins request config subtrees via
`declare wants config <root>` and receive JSON format.

**Benefits:**
- Polyglot plugins: Any language can implement capability plugins
- No engine changes for new capabilities
- Config schema is self-documenting via plugin declarations

### 5-Stage Startup Protocol (Ze)

Ze uses a synchronized 5-stage startup protocol with barriers between stages.
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
│        declare wants config bgp│                 declare wants config bgp│
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
│        STAGE 2: CONFIG DELIVERY (JSON)                                   │
│             │                 │                      │                   │
│             ▼                 │                      ▼                   │
│        ◄── config json bgp ...│             config json bgp ... ──►      │
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

| Stage | Plugin → Ze | Ze → Plugin |
|-------|----------------|----------------|
| 1. Registration | `declare wants config <root>`, `declare cmd/receive/...`, `declare done` | - |
| 2. Config | - | `config json <root> <json>`, `config done` |
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

**Process controls serial numbering.** Ze echoes serial back for correlation.

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

## Ze Implementation Notes

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

## Internal Plugin Invocation Modes

Ze plugins run as **long-lived processes** (goroutines for Go, subprocesses for external).
Each plugin registers the families it handles at startup, then processes requests in a loop.

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              ENGINE                                          │
│                                                                              │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                        Family Registry                               │   │
│   │   ipv4/flowspec     → flowspec plugin                               │   │
│   │   ipv6/flowspec     → flowspec plugin                               │   │
│   │   ipv4/flowspec-vpn → flowspec plugin                               │   │
│   │   ipv6/flowspec-vpn → flowspec plugin                               │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                         │
│                              io.Pipe                                         │
│                                    │                                         │
│   ┌────────────────────────────────▼────────────────────────────────────┐   │
│   │              FLOWSPEC PLUGIN (long-lived goroutine)                 │   │
│   │                                                                      │   │
│   │  1. Startup declarations                                            │   │
│   │  2. Request loop (encode/decode)                                    │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Plugin Startup Protocol

Plugins declare which families they handle:

```
# Plugin → Engine (startup)
declare family ipv4 flowspec decode
declare family ipv4 flowspec encode
declare family ipv6 flowspec decode
declare family ipv6 flowspec encode
declare family ipv4 flowspec-vpn decode
declare family ipv4 flowspec-vpn encode
declare family ipv6 flowspec-vpn decode
declare family ipv6 flowspec-vpn encode
declare done

# Engine registers plugin in family→plugin map
# Then sends config/registry signals per normal startup protocol
```

### Automatic OPEN Capability Injection

**Key design:** When a plugin declares `decode` for a family, the engine automatically
advertises that family in OPEN messages via Multiprotocol capability (Code 1).

**Rationale:**
- If a plugin can decode a family, peers should be able to send it
- No explicit `capability hex 1` needed for Multiprotocol
- Reduces protocol overhead and prevents duplicate capability issues

**How it works:**

```
Plugin Stage 1: declare family ipv4 flowspec decode
                     ↓
Registry: families["ipv4/flow"] = "flowspec"
                     ↓
Session.sendOpen(): GetDecodeFamilies() → ["ipv4/flow", ...]
                     ↓
OPEN: Multiprotocol(AFI=1, SAFI=133)
```

**Override behavior:** Config families completely override plugin families:
- Config has `family {}` block → ONLY config families used, plugin families ignored
- Config has NO `family {}` block → plugin decode families used

This is intentional: explicit config = full control. Plugin families provide defaults
when config doesn't specify families.

**Auto-loading plugins:** When a family is configured but no plugin has claimed it,
the engine automatically loads the internal plugin for that family (if one exists).

**Two-phase plugin startup:**
1. **Phase 1:** Explicit plugins start first and register their families
2. **Phase 2:** After Phase 1 completes, engine checks which configured families are still unclaimed
3. Internal plugins are auto-loaded ONLY for unclaimed families

Auto-loading is **prevented** when:
1. An explicit plugin declares `decode` for the family (family-based check)
2. `--plugin ze.<name>` is passed on command line (prevents auto-load for that plugin)

The check is based on **family claims**, not plugin name. Plugin names are informational only.

| Config | Plugin | Result |
|--------|--------|--------|
| `family { ipv4/flow; }` | None | ✅ Auto-loads `ze.flowspec` |
| `family { ipv4/flow; }` | `--plugin ze.flowspec` | ✅ Uses explicit plugin (no auto-load) |
| `family { ipv4/flow; }` | `plugin { external my-traffic { declares ipv4/flow } }` | ✅ Uses config plugin (no auto-load, family claimed) |
| `family { ipv4/foo; }` | None | ❌ Startup fails (no plugin for family) |

**Functional tests:**
- `test/plugin/flowspec-open-capability.ci` - auto-load for known family
- `test/plugin/family-no-plugin-failure.ci` - failure for unknown family
- `test/plugin/explicit-plugin-precedence.ci` - explicit `--plugin` prevents auto-load
- `test/plugin/explicit-plugin-config.ci` - config plugin prevents auto-load (sends marker UPDATE 99.99.99.0/24 to prove external plugin is active)

**Ordering:** Plugin families are sorted alphabetically for deterministic OPEN messages.

**What plugins should NOT do:**
- ❌ Send `capability hex 1 <multiprotocol-bytes>` for their families
- ❌ Assume plugin families will be used if config has a `family {}` block

**What plugins SHOULD do:**
- ✅ Declare `decode` for all families they can parse (provides defaults)
- ✅ Use `capability hex` only for non-Multiprotocol capabilities (GR, hostname, etc.)

### Engine Routing

**Generic NLRI routing available via Server methods:**

```go
// For external plugins - routes via pipe
func (s *Server) EncodeNLRI(family Family, args []string) ([]byte, error)
func (s *Server) DecodeNLRI(family Family, hexData string) (string, error)

// For in-process plugins - call directly for efficiency
flowspec.EncodeFlowSpecComponents(family, args)
```

**How it works:**
1. `EncodeNLRI`/`DecodeNLRI` look up plugin via `registry.LookupFamily()`
2. If found, send request to plugin via pipe
3. If not found, return error (use direct call for in-process plugins)

**In-process vs External:**
- In-process plugins (flowspec): called directly for better performance
- External plugins: use `server.EncodeNLRI()`/`server.DecodeNLRI()`

### Request/Response Loop

After startup, plugin enters request loop:

| Direction | Format | Example |
|-----------|--------|---------|
| Encode request (text, default) | `encode nlri <family> <args>` | `encode nlri ipv4/flowspec destination 10.0.0.0/24` |
| Encode request (text, explicit) | `encode text nlri <family> <args>` | `encode text nlri ipv4/flowspec destination 10.0.0.0/24` |
| Encode request (JSON) | `encode json nlri <family> <json>` | `encode json nlri ipv4/flowspec {"destination":[["10.0.0.0/24/0"]]}` |
| Encode success | `encoded hex <bytes>` | `encoded hex 0701180A0000` |
| Encode error | `encoded error <msg>` | `encoded error invalid prefix` |
| Decode request | `decode nlri <family> <hex>` | `decode nlri ipv4/flowspec 0701180A0000` |
| Decode success | `decoded json <json>` | `decoded json {"destination":[["10.0.0.0/24/0"]]}` |
| Decode failure | `decoded unknown` | `decoded unknown` |

**Encode Format Specifier:**
- `encode nlri` (default): Text input format - component keywords followed by values
- `encode text nlri`: Explicit text input (same as default)
- `encode json nlri`: JSON input matching decode output format

**Round-trip workflow:**
```
# Decode to JSON
decode nlri ipv4/flowspec 0501180A0000
decoded json {"destination":[["10.0.0.0/24/0"]]}

# Modify JSON as needed, then encode back
encode json nlri ipv4/flowspec {"destination":[["10.0.0.0/24/0"]]}
encoded hex 0501180A0000
```

**JSON format notes:**
- JSON must be minified (no spaces) since protocol is space-delimited
- JSON structure matches decode output exactly

### Mode 1: In-Process (goroutine + io.Pipe)

For Go plugins - runs in same process:

```go
// internal/plugin/process.go - startInternal()
func (p *Process) startInternal() error {
    inR, inW := io.Pipe()
    outR, outW := io.Pipe()

    runner := GetInternalPluginRunner(p.config.Name)
    go runner(inR, outW)    // Long-lived goroutine

    p.stdin = inW
    p.stdout = outR
    return nil
}
```

### Mode 2: Subprocess (fork/exec)

For external plugins (Python, Rust, etc.):

```go
// internal/plugin/process.go - startExternal()
cmd := exec.CommandContext(ctx, p.config.Run)
p.stdin, _ = cmd.StdinPipe()
p.stdout, _ = cmd.StdoutPipe()
cmd.Start()
```

### Benefits of Long-Lived Design

| Benefit | Description |
|---------|-------------|
| No per-request overhead | Plugin starts once, handles many requests |
| Language agnostic | Same protocol for Go/Python/Rust |
| Hot-swappable | Restart plugin without engine restart |
| Testable | Plugin protocol can be tested independently |
| Consistent | Same code path for goroutine and subprocess |

---

## Family Plugin NLRI System

Family plugins provide NLRI encoding/decoding for address families that require complex parsing
(FlowSpec, EVPN, BGP-LS, VPN). This section details the complete protocol.

### Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           NLRI PLUGIN ARCHITECTURE                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   API TEXT COMMAND                                                          │
│   update text nlri ipv4/flowspec add destination 10.0.0.0/24                │
│           │                                                                 │
│           ▼                                                                 │
│   ┌───────────────────────────────────────────────────────────────────┐     │
│   │                      Family Registry                               │     │
│   │   ipv4/flowspec     → flowspec plugin                             │     │
│   │   ipv6/flowspec     → flowspec plugin                             │     │
│   │   ipv4/flowspec-vpn → flowspec plugin                             │     │
│   │   l2vpn/evpn        → evpn plugin (future)                        │     │
│   └───────────────────────────────────────────────────────────────────┘     │
│           │                                                                 │
│           ▼                                                                 │
│   ┌───────────────────────────────────────────────────────────────────┐     │
│   │              encode nlri ipv4/flowspec destination 10.0.0.0/24    │     │
│   │                          REQUEST (#serial)                         │     │
│   └───────────────────────────────────────────────────────────────────┘     │
│           │                                                                 │
│           ▼                                                                 │
│   ┌───────────────────────────────────────────────────────────────────┐     │
│   │              FLOWSPEC PLUGIN (goroutine)                          │     │
│   │                                                                    │     │
│   │   Parse: "destination 10.0.0.0/24"                                │     │
│   │   Encode: FlowSpec Type 1 + prefix                                │     │
│   │   Return: hex bytes                                                │     │
│   └───────────────────────────────────────────────────────────────────┘     │
│           │                                                                 │
│           ▼                                                                 │
│   ┌───────────────────────────────────────────────────────────────────┐     │
│   │              @serial encoded hex 0701180A0000                     │     │
│   │                          RESPONSE (@serial)                        │     │
│   └───────────────────────────────────────────────────────────────────┘     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Serial Prefix Protocol

**CRITICAL:** Requests and responses use different prefixes.

| Direction | Prefix | Example |
|-----------|--------|---------|
| Engine → Plugin (request) | `#serial` | `#42 encode nlri ipv4/flowspec destination 10.0.0.0/24` |
| Plugin → Engine (response) | `@serial` | `@42 encoded hex 0701180A0000` |

**Why different prefixes:**
- Enables plugins to distinguish incoming requests from other messages
- Prevents response confusion in bidirectional communication
- Allows multiplexed requests (multiple in-flight)

### Family Registration (Stage 1)

Plugins declare which families they handle during startup:

```
# Plugin → Engine
declare family ipv4 flowspec encode         # Can encode ipv4/flowspec NLRI
declare family ipv4 flowspec decode         # Can decode ipv4/flowspec NLRI
declare family ipv6 flowspec encode
declare family ipv6 flowspec decode
declare family ipv4 flowspec-vpn encode     # VPN variant
declare family ipv4 flowspec-vpn decode
declare done
```

**Format:** `declare family <afi> <safi> <mode>`

| Field | Values | Description |
|-------|--------|-------------|
| AFI | `ipv4`, `ipv6`, `l2vpn`, `all` | Address family identifier |
| SAFI | `unicast`, `flowspec`, `flowspec-vpn`, `evpn`, etc. | Sub-address family |
| Mode | `encode`, `decode` | Direction of conversion |

**Registry conflict detection:**
- Only ONE plugin can register for a family+mode combination
- Conflict → startup error: `family conflict: ipv4/flowspec already registered by X`

**OPEN capability injection (decode mode):**
- Families declared with `decode` are automatically advertised in OPEN
- Engine adds Multiprotocol capability (Code 1) for each decode family
- No explicit `capability hex 1` needed from plugins
- See "Automatic OPEN Capability Injection" section above

### Request/Response Protocol

#### Encode Request

**Engine → Plugin:**
```
#<serial> encode nlri <family> <args...>
```

**Examples:**
```
#1 encode nlri ipv4/flowspec destination 10.0.0.0/24
#2 encode nlri ipv4/flowspec destination 10.0.0.0/24 source 192.168.0.0/16
#3 encode nlri ipv4/flowspec destination 10.0.0.0/24 protocol 6 port 80
```

**Plugin → Engine (success):**
```
@<serial> encoded hex <hex-bytes>
```

**Plugin → Engine (error):**
```
@<serial> encoded error <message>
```

#### Decode Request

**Engine → Plugin:**
```
#<serial> decode nlri <family> <hex-bytes>
```

**Examples:**
```
#1 decode nlri ipv4/flowspec 0701180A0000
#2 decode nlri ipv6/flowspec 0228200100db80000
```

**Plugin → Engine (success):**
```
@<serial> decoded json <json-object>
```

**Plugin → Engine (cannot decode):**
```
@<serial> decoded unknown
```

Note: Decode failures return `decoded unknown` without an error message (unlike encode
which returns `encoded error <message>`). This is because decode failures are often
ambiguous - the hex may be valid for a different family or version.

### Response Formats

#### Encode Response

Success returns hex-encoded wire bytes:
```
@42 encoded hex 0701180A0000
```

The hex bytes are the raw NLRI ready to embed in MP_REACH/MP_UNREACH.

#### Decode Response

Success returns JSON describing the NLRI components:
```
@42 decoded json {"destination":[["10.0.0.0/24/0"]]}
```

JSON format is family-specific. For FlowSpec, see `docs/architecture/wire/nlri-flowspec.md`.

### Public API Methods

The Server provides two methods for external callers:

```go
// EncodeNLRI encodes NLRI by routing to the appropriate family plugin.
// This is the public API for external callers (CLI tools, external plugins, tests).
// Internal code paths use direct function calls for performance.
func (s *Server) EncodeNLRI(family nlri.Family, args []string) ([]byte, error)

// DecodeNLRI decodes NLRI by routing to the appropriate family plugin.
// This is the public API for external callers (CLI tools, external plugins, tests).
// Returns the JSON representation of the decoded NLRI.
func (s *Server) DecodeNLRI(family nlri.Family, hexData string) (string, error)
```

**When to use:**
- CLI tools that don't know the family at compile time
- External plugins needing NLRI conversion
- Tests validating plugin behavior

**When NOT to use:**
- Internal code with known family → call plugin directly for performance
- Example: `update_text.go` calls `flowspec.Encode()` directly

### Implementing a Family Plugin

#### Minimal Plugin Structure

```
1. Register families at startup (declare family ...)
2. Enter request loop
3. For each line:
   a. Parse serial prefix (#N)
   b. Dispatch to encode/decode handler
   c. Send response with @N prefix
4. Exit on stdin close
```

#### Request Loop Pattern

```
Loop forever:
  line = read_stdin()
  if line starts with "#":
    serial, command = parse_serial(line)
    response = handle_request(command)
    send("@" + serial + " " + response)
```

#### Error Handling

| Error Type | Response Format | Example |
|------------|-----------------|---------|
| Invalid family (encode) | `encoded error unknown family` | `@1 encoded error unknown family: ipv4/unknown` |
| Parse error (encode) | `encoded error <details>` | `@2 encoded error invalid prefix: 10.0.0/24` |
| Cannot decode | `decoded unknown` | `@3 decoded unknown` |
| Internal error | `encoded error <msg>` | `@4 encoded error buffer overflow` |

### Files

| File | Purpose |
|------|---------|
| `internal/plugin/registration.go` | Family registry, conflict detection |
| `internal/plugin/server.go` | `EncodeNLRI()`, `DecodeNLRI()` routing |
| `internal/plugin/flowspec/plugin.go` | FlowSpec plugin implementation |
| `internal/plugin/update_text.go` | Direct plugin calls for known families |

---

## Plugin Command Registration

External processes can register custom commands that extend Ze's API.

### Registration Protocol

**Process → Ze (stdout):**
```
#N register command "<name>" description "<help>" [args "<usage>"] [completable] [timeout <duration>]
```

**Ze → Process (stdin):**
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

**Ze → Process (stdin):**
```json
{"serial":"a","type":"request","command":"myapp status","args":["component"],"peer":"*"}
```

**Process → Ze (stdout):**
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

**Ze → Process:**
```json
{"serial":"b","type":"complete","command":"myapp copy","args":["file1"],"partial":"f"}
```

**Process → Ze:**
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
│   Ze Engine                              GR Plugin (ze plugin gr)     │
│   ────────────                              ────────────────────────────    │
│                                                                             │
│   STAGE 1: REGISTRATION                                                     │
│                                                                             │
│                              ◄───── declare wants config bgp                │
│                              ◄───── declare done                            │
│                                                                             │
│   STAGE 2: CONFIG DELIVERY (JSON format)                                    │
│                                                                             │
│   config json bgp {"bgp":   ─────►                                         │
│     {"peer":{"192.168.1.1":          (full config tree as JSON)             │
│       {"capability":{                                                       │
│         "graceful-restart":                                                 │
│           {"restart-time":120}       (plugin extracts what it needs)        │
│     }}}}}                                                                   │
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
- Declaration: `declare wants config bgp` (request config subtree)
- Config delivery: `config json bgp {"bgp":{"peer":{...}}}` (full JSON tree)
- Plugin parses JSON and extracts `capability.graceful-restart.restart-time` per peer

**Key insight:** GR plugin receives the entire BGP config tree as JSON, extracting what
it needs (graceful-restart settings per peer). This pattern aligns with the hostname
plugin and replaces the deprecated pattern-based `declare conf` approach.

### RIB Plugin (Full Lifecycle)

The RIB plugin tracks routes and replays them on peer reconnect.
**Requires process binding** for runtime events.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        RIB PLUGIN MESSAGE FLOW                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   Ze Engine                              RIB Plugin (ze plugin rib)   │
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
│                              ◄───── #1 peer 192.168.1.1 plugin session ready│
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
        run "ze plugin gr";
        encoder json;
    }

    external rib {
        run "ze plugin rib";
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

## Capability Decode API

Plugins can provide capability decoding for `ze bgp decode --plugin <name>`.

This is a **standalone mode** separate from the 5-stage startup protocol.

### Usage

```bash
# Decode OPEN message with plugin-provided capability decoding
ze bgp decode --plugin ze.hostname --open FFFF...
```

Without plugin, unknown capabilities show raw hex:
```json
{"code": 73, "name": "unknown", "raw": "0C6D792D686F73742D6E616D65..."}
```

With plugin, capabilities are decoded:
```json
{"name": "fqdn", "hostname": "my-host-name", "domain": "my-domain-name.com"}
```

### Protocol

Plugin is spawned with `--decode` flag and communicates via stdin/stdout.

#### Request Formats

| Request | Description |
|---------|-------------|
| `decode capability <code> <hex>` | JSON output (default) |
| `decode json capability <code> <hex>` | JSON output (explicit) |
| `decode text capability <code> <hex>` | Human-readable text output |
| `decode nlri <family> <hex>` | JSON output (default) |
| `decode json nlri <family> <hex>` | JSON output (explicit) |
| `decode text nlri <family> <hex>` | Human-readable text output |

#### Response Formats

| Response | Description |
|----------|-------------|
| `decoded json <json>` | JSON-formatted result |
| `decoded text <text>` | Human-readable single-line text |
| `decoded unknown` | Plugin cannot decode this input |

#### Examples

**Capability decode (JSON):**

| Direction | Message |
|-----------|---------|
| ze → plugin | `decode json capability 73 0C6D792D686F7374...` |
| plugin → ze | `decoded json {"name":"fqdn","hostname":"my-host","domain":"dom.com"}` |

**Capability decode (text):**

| Direction | Message |
|-----------|---------|
| ze → plugin | `decode text capability 73 0C6D792D686F7374...` |
| plugin → ze | `decoded text fqdn                 my-host.dom.com` |

**NLRI decode (text):**

| Direction | Message |
|-----------|---------|
| ze → plugin | `decode text nlri ipv4/flow 0501180a0000` |
| plugin → ze | `decoded text destination 10.0.0.0/24` |

If plugin cannot decode:

| Direction | Message |
|-----------|---------|
| plugin → ze | `decoded unknown` |

### Plugin Implementation

Plugin entry point with `--decode` flag:

```bash
ze plugin hostname --decode
```

Plugin reads decode requests from stdin, writes responses to stdout, exits on EOF.

### Capability Registration

Currently, capability-to-plugin mapping is hardcoded in `decode.go`:

```go
var pluginCapabilityMap = map[uint8]string{
    73: "hostname", // FQDN capability
}
```

Future: Plugins will declare decodable capabilities via `declare decode capability <code>`.

### Files

| File | Purpose |
|------|---------|
| `cmd/ze/bgp/decode.go` | Invokes plugin decode API |
| `cmd/ze/bgp/plugin_hostname.go` | `--decode` flag handling (hostname) |
| `cmd/ze/bgp/plugin_flowspec.go` | `--decode` flag handling (flowspec) |
| `internal/plugin/hostname/hostname.go` | `RunDecodeMode()` - hostname capability |
| `internal/plugin/flowspec/plugin.go` | `RunFlowSpecDecode()` - FlowSpec NLRI |

---

**Last Updated:** 2026-02-02
