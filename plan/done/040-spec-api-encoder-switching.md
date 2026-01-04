# Spec: api-encoder-switching

## MANDATORY READING (BEFORE IMPLEMENTATION)

```
┌─────────────────────────────────────────────────────────────────┐
│  STOP. Read these files FIRST before ANY implementation:        │
│                                                                 │
│  1. .claude/ESSENTIAL_PROTOCOLS.md - Session rules, TDD         │
│  2. .claude/INDEX.md - Find what docs to load                   │
│  3. plan/CLAUDE_CONTINUATION.md - Current state                 │
│  4. THIS SPEC FILE - Design requirements                        │
│  5. pkg/api/*.go, pkg/config/bgp.go - Current implementation    │
│                                                                 │
│  DO NOT PROCEED until all are read and understood.              │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Redesign process API configuration to properly separate:
1. **WHAT** messages to receive/send (message types) - per-peer
2. **HOW** to format them (encoding + format) - per-peer-binding

Fix the current bug where `encoder json` processes receive nothing or text format.

## Current State (verified)

- `make test`: PASS
- `make lint`: PASS (0 issues)
- **Phase 0 COMPLETE**: Message dispatch wired for all types
- **Phase 1 COMPLETE**: Config parsing and data flow working
- **Phase 2 COMPLETE**: Per-peer message routing with bindings
- **Phase 4 COMPLETE**: Migration tool updated
- **Phase 5 COMPLETE**: Documentation updated
- **ALL PHASES COMPLETE** ✅
- Last verified: 2025-12-30

### Known Issues in Current Code

| Location | Issue |
|----------|-------|
| `bgp.go:548` | `if pc.Encoder == "text" { pc.ReceiveUpdate = true }` - JSON gets nothing |
| `server.go:470,492,514` | TODO: All message types use `ReceiveUpdate` flag |

### Phase 1 Completed (2025-12-30)

**What was implemented:**
- Config structs: `PeerAPIBinding`, `PeerContentConfig`, `PeerReceiveConfig`, `PeerSendConfig` ✅
- Reactor struct: `reactor.APIBinding` ✅
- API struct: `api.PeerAPIBinding` ✅
- Data flow pipeline: config→loader→reactor→api ✅
- Process validation: undefined process error ✅
- Old syntax parsing: `api { processes [...]; neighbor-changes; }` ✅
- **New syntax parsing:** `api foo { content {...} receive {...} send {...} }` ✅
- **Schema:** Changed to `List(TypeString, ...)` with `_anonymous` key for old syntax ✅
- **Parsing helpers:** `parseReceiveConfig()`, `parseSendConfig()`, `parseNewAPIBinding()` ✅
- **Encoding inheritance:** `GetPeerAPIBindings()` resolves peer → process → "text" ✅
- **`all;` keyword:** Expands to all flags in receive/send blocks ✅
- **ExaBGP compat:** `processes-match`, `neighbor-changes` fields added to schema ✅
- **Tests:** 8 new tests for new syntax parsing ✅

**Phase 1 Hardening (2025-12-30):**
- Deterministic binding order: sort map keys before iteration ✅
- Case normalization: encoding/format values lowercased ✅
- Reserved name validation: reject process names starting with `_` ✅
- Reactor inheritance tests: `TestGetPeerAPIBindingsEncodingInheritance` ✅

**Remaining for future phases:**
- Validation for invalid encoding/format values (low priority)

## Design Principles

### Key Differences from ExaBGP

| Aspect | ExaBGP | ZeBGP |
|--------|--------|-------|
| Keyword | `neighbor {` | `peer {` |
| API binding | `api { processes [foo]; }` | `api foo { ... }` in peer |
| Format location | `receive { parsed; packets; }` | `content { format ...; }` per binding |
| Output syntax | `neighbor X announce route ...` | `announce nlri <family> <nlris>` |

### Architecture

```
Process = unique program (runs once, defined globally)
Peer API binding = which process, what messages, what format (per-peer)
```

One process can serve multiple peers. Each peer-binding can have different:
- Message types (update, notification, etc.)
- Format (parsed, raw, full)
- Encoding (json, text)

## Data Flow Design

### Current Flow (Process-Global)

```
config.ProcessConfig ──► loader.go ──► reactor.APIProcessConfig ──► reactor.go ──► api.ProcessConfig ──► Server
     (global)                              (global)                                    (global)
```

### New Flow (Per-Peer Bindings)

```
config.PeerConfig.APIBindings ──► loader.go ──► reactor.PeerSettings.APIBindings
                                                           │
                                                           ▼
                                              reactor.peerBindingsMap[addr]
                                                           │
                                                           ▼
                                              Server.getPeerAPIBindings(addr)
                                                     via ReactorInterface
```

### Server Access to Peer Bindings

**Option chosen: Server queries Reactor via interface**

```go
// ReactorInterface addition
type ReactorInterface interface {
    // ... existing methods ...

    // GetPeerAPIBindings returns API bindings for a peer address.
    // Returns nil if peer has no API bindings.
    GetPeerAPIBindings(addr netip.Addr) []PeerAPIBinding
}

// Server usage
func (s *Server) OnMessageReceived(peer PeerInfo, msg RawMessage) {
    bindings := s.reactor.GetPeerAPIBindings(peer.Address)
    for _, binding := range bindings {
        // ...
    }
}
```

**Why this approach:**
- No data duplication (bindings live in Reactor/PeerSettings)
- Server doesn't need to track peer lifecycle
- Consistent with existing `s.reactor.GetPeerByIP()` pattern

## ZeBGP Configuration Format

### Process Definition (Global)

```
process <name> {
    run <command>;
    encoder <type>;           # KEPT for backward compat (default: text)
    respawn <bool>;           # default: true
}
```

**Backward compatibility:** `encoder` stays in process definition but is OVERRIDDEN by peer-level `content.encoding` if specified.

### Peer API Binding

```
peer <address> {
    # ... peer settings ...

    # API binding: link peer to process with per-binding config
    api <process-name> {
        # HOW to format (per this peer-binding, overrides process encoder)
        content {
            encoding json;       # json | text (default: inherit from process)
            format parsed;       # parsed | raw | full (default: parsed)
        }

        # WHAT messages to receive from this peer
        receive {
            update;              # route announcements (includes withdrawals)
            open;                # OPEN messages
            notification;        # errors
            keepalive;           # heartbeats
            refresh;             # route-refresh
            state;               # up/down/connected/fsm events
            all;                 # shorthand for all above
        }

        # WHAT messages this process can send to this peer
        send {
            update;              # can inject routes
            refresh;             # can request route-refresh
            all;                 # shorthand for all above
        }
    }
}
```

### Shorthand Forms

```
# Minimal process definition
process foo { run ./script; }

# Minimal peer binding (defaults: inherit encoding, parsed format, no messages)
# Note: Empty block required - parser doesn't support `api foo;` shorthand
peer 10.0.0.1 {
    api foo { }
}

# Common case: JSON updates only
peer 10.0.0.1 {
    api foo {
        content { encoding json; }
        receive { update; }
    }
}

# All messages, full format
peer 10.0.0.1 {
    api foo {
        content { encoding json; format full; }
        receive { all; }
        send { all; }
    }
}
```

**Parser note:** The `List()` schema type requires a block after the key. `api foo;` without braces will fail to parse. Use `api foo { }` for minimal bindings.

### Multiple Peers, Same Process, Different Formats

```
process route-collector { run ./collector.py; }

# Peer A: JSON, parsed only
peer 10.0.0.1 {
    api route-collector {
        content { encoding json; format parsed; }
        receive { update; }
    }
}

# Peer B: Text, full format (parsed + raw)
peer 10.0.0.2 {
    api route-collector {
        content { encoding text; format full; }
        receive { update; notification; }
    }
}
```

## Output Syntax: announce nlri

### Versioning

**API Version 7** introduces the new output format. Existing v6 format preserved for compatibility.

```go
const (
    APIVersionLegacy = 6  // ExaBGP-compatible format
    APIVersionNLRI   = 7  // New announce nlri format
)
```

Process can request version via environment or config (future).

### JSON Format (v7)

```json
{
  "type": "update",
  "peer": {
    "address": "10.0.0.1",
    "asn": 65001
  },
  "announce": {
    "nlri": {
      "ipv4/unicast": {
        "192.168.1.0/24": {
          "next-hop": "10.0.0.1",
          "origin": "igp",
          "as-path": [65001]
        }
      }
    }
  }
}
```

### Text Format (v7)

```
peer 10.0.0.1 update announce nlri ipv4/unicast 192.168.1.0/24 next-hop 10.0.0.1 origin igp as-path [65001]
```

### Withdrawals

JSON:
```json
{
  "type": "update",
  "peer": { "address": "10.0.0.1" },
  "withdraw": {
    "nlri": {
      "ipv4/unicast": ["192.168.1.0/24", "192.168.2.0/24"]
    }
  }
}
```

Text:
```
peer 10.0.0.1 update withdraw nlri ipv4/unicast 192.168.1.0/24 192.168.2.0/24
```

## Format Options

| Option | Description | JSON example |
|--------|-------------|--------------|
| `format parsed` | Decoded fields only (default) | `{"announce": {"nlri": ...}}` |
| `format raw` | Wire bytes only (hex) | `{"raw": "ffffffff..."}` |
| `format full` | Both parsed AND raw | `{"announce": ..., "raw": "..."}` |

## Error Handling

### Config Validation Errors

| Condition | Error |
|-----------|-------|
| `api foo` references non-existent process | `"process 'foo' not defined"` |
| Invalid encoding value | `"invalid encoding 'xml': must be 'json' or 'text'"` |
| Invalid format value | `"invalid format 'compact': must be 'parsed', 'raw', or 'full'"` |
| Duplicate api binding for same process | Warning only (later binding wins) |

### Runtime Errors

| Condition | Behavior |
|-----------|----------|
| Process not running | Skip, log warning |
| Process write fails | Log error, continue to other processes |
| Peer disconnected | Bindings still exist, messages not sent |

## ExaBGP Migration

### ExaBGP Input
```
process foo {
    run ./script.py;
    encoder json;
}

neighbor 10.0.0.1 {
    api {
        processes [ foo ];
        receive {
            parsed;
            packets;
            consolidate;
            update;
            notification;
        }
    }
}
```

### ZeBGP Output
```
process foo {
    run ./script.py;
    encoder json;      # Kept for reference
}

peer 10.0.0.1 {
    api foo {
        content {
            encoding json;
            format full;        # parsed + packets + consolidate
        }
        receive {
            update;
            notification;
        }
    }
}
```

### Migration Mapping

| ExaBGP | ZeBGP |
|--------|-------|
| `encoder json` in process | `content { encoding json; }` in api binding |
| `encoder text` in process | `content { encoding text; }` in api binding |
| `receive { parsed; }` only | `content { format parsed; }` |
| `receive { packets; }` only | `content { format raw; }` |
| `receive { parsed; packets; consolidate; }` | `content { format full; }` |
| `api { processes [ foo ]; }` | `api foo { ... }` |
| `neighbor-changes;` | `receive { state; }` |

## Documentation Impact

Design changes require updating:
- [ ] `.claude/zebgp/api/ARCHITECTURE.md` - new API binding model
- [ ] `.claude/zebgp/config/SYNTAX.md` - `peer {` and `api <name> {}` syntax
- [ ] `plan/CLAUDE_CONTINUATION.md` - phase status

## Implementation Phases

### Phase 0: Message Dispatch (PARTIAL - needs completion)

**Done:**
- UPDATE → `DecodeUpdateRoutes` → `forwardUpdateToProcesses`
- OPEN → `DecodeOpen` → `forwardOpenToProcesses`
- NOTIFICATION → `DecodeNotification` → `forwardNotificationToProcesses`
- KEEPALIVE → `forwardKeepaliveToProcesses`

**TODO (move to Phase 1):**
- All forward functions use `ReceiveUpdate` flag - need per-message flags
- JSON encoding works, but only if `ReceiveUpdate=true`

### Phase 1: Config Schema + Data Flow

#### 1.1 Keep encoder in process schema (backward compat)
**File:** `pkg/config/bgp.go:286-290`

```go
// Process schema - encoder KEPT for backward compatibility
schema.Define("process", List(TypeString,
    Field("run", MultiLeaf(TypeString)),
    Field("encoder", Leaf(TypeString)),  // KEEP - default for processes without peer override
    Field("respawn", Leaf(TypeBool)),
))
```

#### 1.2 Add api binding to peerFields()
**File:** `pkg/config/bgp.go:166-230` (add to peerFields)

```go
// API bindings: api <process-name> { content {...}; receive {...}; send {...} }
Field("api", List(TypeString,
    Field("content", Container(
        Field("encoding", Leaf(TypeString)),  // json | text
        Field("format", Leaf(TypeString)),    // parsed | raw | full
    )),
    Field("receive", Freeform()),  // { update; notification; all; }
    Field("send", Freeform()),     // { update; refresh; all; }
)),
```

**Note:** Using `Freeform()` for receive/send blocks - parses `word;` entries as key→"true".

#### 1.3 Add config structs
**File:** `pkg/config/bgp.go` (after ProcessConfig)

```go
// PeerAPIBinding holds per-peer API binding configuration.
type PeerAPIBinding struct {
    ProcessName string
    Content     PeerContentConfig
    Receive     PeerReceiveConfig
    Send        PeerSendConfig
}

// PeerContentConfig controls HOW messages are formatted.
type PeerContentConfig struct {
    Encoding string // json | text (empty = inherit from process)
    Format   string // parsed | raw | full (default: parsed)
    // Version int - DEFERRED TO PHASE 3
}

// PeerReceiveConfig controls WHAT messages to receive.
// Note: `all;` expands to set all flags true at parse time.
type PeerReceiveConfig struct {
    Update       bool
    Open         bool
    Notification bool
    Keepalive    bool
    Refresh      bool
    State        bool
}

// PeerSendConfig controls WHAT messages process can send.
// Note: `all;` expands to set all flags true at parse time.
type PeerSendConfig struct {
    Update  bool
    Refresh bool
}
```

#### 1.4 Add to PeerConfig
**File:** `pkg/config/bgp.go` (PeerConfig struct)

```go
type PeerConfig struct {
    // ... existing fields ...
    APIBindings []PeerAPIBinding  // NEW: per-peer API bindings
}
```

#### 1.5 Update parsing in TreeToConfig
**File:** `pkg/config/bgp.go` (in peer parsing section)

```go
// Parse API bindings for this peer
for procName, apiTree := range peerTree.GetList("api") {
    binding := PeerAPIBinding{ProcessName: procName}

    // Parse content block
    if content := apiTree.GetContainer("content"); content != nil {
        if v, ok := content.Get("encoding"); ok {
            binding.Content.Encoding = v
        }
        if v, ok := content.Get("format"); ok {
            binding.Content.Format = v
        }
    }

    // Parse receive block (Freeform: entries stored as key->"true")
    if recv := apiTree.GetContainer("receive"); recv != nil {
        binding.Receive = parseReceiveConfig(recv)
    }

    // Parse send block
    if send := apiTree.GetContainer("send"); send != nil {
        binding.Send = parseSendConfig(send)
    }

    // Validate process exists
    if !hasProcess(cfg.Processes, procName) {
        return nil, fmt.Errorf("peer %s: api references undefined process %q", addr, procName)
    }

    peer.APIBindings = append(peer.APIBindings, binding)
}
```

#### 1.6 Parsing helpers
**File:** `pkg/config/bgp.go`

```go
// parseReceiveConfig parses a Freeform receive block.
// Freeform stores "update;" as key "update" -> value "true".
func parseReceiveConfig(tree *Tree) PeerReceiveConfig {
    cfg := PeerReceiveConfig{}

    // Check for "all" shorthand
    if _, ok := tree.Get("all"); ok {
        cfg.Update = true
        cfg.Open = true
        cfg.Notification = true
        cfg.Keepalive = true
        cfg.Refresh = true
        cfg.State = true
        return cfg
    }

    // Individual flags
    _, cfg.Update = tree.Get("update")
    _, cfg.Open = tree.Get("open")
    _, cfg.Notification = tree.Get("notification")
    _, cfg.Keepalive = tree.Get("keepalive")
    _, cfg.Refresh = tree.Get("refresh")
    _, cfg.State = tree.Get("state")

    return cfg
}

// parseSendConfig parses a Freeform send block.
func parseSendConfig(tree *Tree) PeerSendConfig {
    cfg := PeerSendConfig{}

    if _, ok := tree.Get("all"); ok {
        cfg.Update = true
        cfg.Refresh = true
        return cfg
    }

    _, cfg.Update = tree.Get("update")
    _, cfg.Refresh = tree.Get("refresh")

    return cfg
}

// hasProcess checks if a process name exists in the config.
func hasProcess(procs []ProcessConfig, name string) bool {
    for _, p := range procs {
        if p.Name == name {
            return true
        }
    }
    return false
}
```

#### 1.7 Add to reactor.PeerSettings
**File:** `pkg/reactor/peersettings.go`

```go
type PeerSettings struct {
    // ... existing fields (line 153-213) ...

    // APIBindings holds per-peer API process bindings.
    // Each binding specifies which process receives messages and how to format them.
    APIBindings []PeerAPIBinding
}

// PeerAPIBinding matches config.PeerAPIBinding for reactor use.
type PeerAPIBinding struct {
    ProcessName string
    Content     PeerContentConfig
    Receive     PeerReceiveConfig
    Send        PeerSendConfig
}

type PeerContentConfig struct {
    Encoding string
    Format   string
    // Version int - DEFERRED TO PHASE 3
}

type PeerReceiveConfig struct {
    Update       bool
    Open         bool
    Notification bool
    Keepalive    bool
    Refresh      bool
    State        bool
}

type PeerSendConfig struct {
    Update  bool
    Refresh bool
}
```

#### 1.8 Update loader.go conversion
**File:** `pkg/config/loader.go` (configToPeer function)

```go
// Convert API bindings
for _, ab := range nc.APIBindings {
    settings.APIBindings = append(settings.APIBindings, reactor.PeerAPIBinding{
        ProcessName: ab.ProcessName,
        Content: reactor.PeerContentConfig{
            Encoding: ab.Content.Encoding,
            Format:   ab.Content.Format,
        },
        Receive: reactor.PeerReceiveConfig{
            Update:       ab.Receive.Update,
            Open:         ab.Receive.Open,
            Notification: ab.Receive.Notification,
            Keepalive:    ab.Receive.Keepalive,
            Refresh:      ab.Receive.Refresh,
            State:        ab.Receive.State,
        },
        Send: reactor.PeerSendConfig{
            Update:  ab.Send.Update,
            Refresh: ab.Send.Refresh,
        },
    })
}
```

#### 1.9 Add ReactorInterface method
**File:** `pkg/api/types.go` (ReactorInterface)

```go
type ReactorInterface interface {
    // ... existing methods ...

    // GetPeerAPIBindings returns API bindings for a peer.
    GetPeerAPIBindings(addr netip.Addr) []PeerAPIBinding
}

// PeerAPIBinding in api package (for interface return type)
type PeerAPIBinding struct {
    ProcessName string
    Encoding    string // Resolved: peer override or process default
    Format      string
    // Version int - DEFERRED TO PHASE 3
    Receive     ReceiveConfig
    Send        SendConfig
}

type ReceiveConfig struct {
    Update       bool
    Open         bool
    Notification bool
    Keepalive    bool
    Refresh      bool
    State        bool
}

type SendConfig struct {
    Update  bool
    Refresh bool
}
```

#### 1.10 Implement in reactorAPIAdapter
**File:** `pkg/reactor/reactor.go` (reactorAPIAdapter)

```go
func (a *reactorAPIAdapter) GetPeerAPIBindings(addr netip.Addr) []api.PeerAPIBinding {
    a.r.mu.RLock()
    defer a.r.mu.RUnlock()

    peer, ok := a.r.peers[addr.String()]
    if !ok {
        return nil
    }

    settings := peer.Settings()
    result := make([]api.PeerAPIBinding, len(settings.APIBindings))

    for i, b := range settings.APIBindings {
        // Resolve encoding: peer override or process default
        encoding := b.Content.Encoding
        if encoding == "" {
            // Look up process default
            for _, pc := range a.r.config.APIProcesses {
                if pc.Name == b.ProcessName {
                    encoding = pc.Encoder
                    break
                }
            }
        }
        if encoding == "" {
            encoding = "text" // Ultimate default
        }

        format := b.Content.Format
        if format == "" {
            format = "parsed"
        }

        result[i] = api.PeerAPIBinding{
            ProcessName: b.ProcessName,
            Encoding:    encoding,
            Format:      format,
            Receive: api.ReceiveConfig{
                Update:       b.Receive.Update,
                Open:         b.Receive.Open,
                Notification: b.Receive.Notification,
                Keepalive:    b.Receive.Keepalive,
                Refresh:      b.Receive.Refresh,
                State:        b.Receive.State,
            },
            Send: api.SendConfig{
                Update:  b.Send.Update,
                Refresh: b.Send.Refresh,
            },
        }
    }

    return result
}
```

#### 1.11 Tests
**File:** `pkg/config/bgp_test.go`

```go
// parseConfig is a test helper that parses config and converts to BGPConfig.
func parseConfig(t *testing.T, input string) *BGPConfig {
    t.Helper()
    p := NewParser(BGPSchema())
    tree, err := p.Parse(input)
    require.NoError(t, err, "parse failed")
    cfg, err := TreeToConfig(tree)
    require.NoError(t, err, "TreeToConfig failed")
    return cfg
}

// parseConfigErr is a test helper that expects parsing/conversion to fail.
func parseConfigErr(t *testing.T, input string) error {
    t.Helper()
    p := NewParser(BGPSchema())
    tree, err := p.Parse(input)
    if err != nil {
        return err
    }
    _, err = TreeToConfig(tree)
    return err
}

func TestPeerAPIBinding(t *testing.T) {
    // VALIDATES: Config parsing extracts API bindings correctly
    // PREVENTS: Silent failures when api block is malformed

    input := `
        process foo { run ./test; encoder text; }
        peer 10.0.0.1 {
            router-id 1.2.3.4;
            local-as 65001;
            peer-as 65002;
            api foo {
                content { encoding json; format full; }
                receive { update; notification; }
            }
        }
    `

    cfg := parseConfig(t, input)
    require.Len(t, cfg.Peers, 1)
    require.Len(t, cfg.Peers[0].APIBindings, 1)

    binding := cfg.Peers[0].APIBindings[0]
    assert.Equal(t, "foo", binding.ProcessName)
    assert.Equal(t, "json", binding.Content.Encoding)
    assert.Equal(t, "full", binding.Content.Format)
    assert.True(t, binding.Receive.Update)
    assert.True(t, binding.Receive.Notification)
    assert.False(t, binding.Receive.Open) // Not specified
}

func TestReceiveAllExpansion(t *testing.T) {
    // VALIDATES: "all" keyword expands to all message type flags
    // PREVENTS: Missing messages when user specifies "all"

    input := `
        process foo { run ./test; }
        peer 10.0.0.1 {
            router-id 1.2.3.4;
            local-as 65001;
            peer-as 65002;
            api foo {
                receive { all; }
            }
        }
    `

    cfg := parseConfig(t, input)

    recv := cfg.Peers[0].APIBindings[0].Receive
    assert.True(t, recv.Update, "all should set Update")
    assert.True(t, recv.Open, "all should set Open")
    assert.True(t, recv.Notification, "all should set Notification")
    assert.True(t, recv.Keepalive, "all should set Keepalive")
    assert.True(t, recv.Refresh, "all should set Refresh")
    assert.True(t, recv.State, "all should set State")
}

func TestAPIBindingUndefinedProcess(t *testing.T) {
    // VALIDATES: Error when api references non-existent process
    // PREVENTS: Runtime crashes from nil process lookup

    input := `
        peer 10.0.0.1 {
            router-id 1.2.3.4;
            local-as 65001;
            peer-as 65002;
            api nonexistent {
                receive { update; }
            }
        }
    `

    err := parseConfigErr(t, input)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "undefined process")
}

func TestEncodingInheritance(t *testing.T) {
    // VALIDATES: Peer binding inherits encoder from process if not specified
    // PREVENTS: Wrong encoding when peer doesn't override

    input := `
        process foo { run ./test; encoder json; }
        peer 10.0.0.1 {
            router-id 1.2.3.4;
            local-as 65001;
            peer-as 65002;
            api foo {
                receive { update; }
            }
        }
    `

    cfg := parseConfig(t, input)

    // Content.Encoding should be empty (inherited at runtime)
    assert.Empty(t, cfg.Peers[0].APIBindings[0].Content.Encoding)

    // Verify process has the encoder
    assert.Equal(t, "json", cfg.Processes[0].Encoder)
}

func TestEmptyAPIBinding(t *testing.T) {
    // VALIDATES: Empty api block creates binding with defaults
    // PREVENTS: Crash on minimal api binding

    input := `
        process foo { run ./test; }
        peer 10.0.0.1 {
            router-id 1.2.3.4;
            local-as 65001;
            peer-as 65002;
            api foo { }
        }
    `

    cfg := parseConfig(t, input)
    require.Len(t, cfg.Peers[0].APIBindings, 1)

    binding := cfg.Peers[0].APIBindings[0]
    assert.Equal(t, "foo", binding.ProcessName)
    assert.Empty(t, binding.Content.Encoding) // Inherit from process
    assert.Empty(t, binding.Content.Format)   // Default to "parsed"
    assert.False(t, binding.Receive.Update)   // No messages subscribed
}
```

### Phase 2: Message Routing with Per-Peer Format

#### 2.1 Add GetProcess to ProcessManager
**File:** `pkg/api/process.go`

```go
// GetProcess returns a process by name, or nil if not found.
func (pm *ProcessManager) GetProcess(name string) *Process {
    pm.mu.RLock()
    defer pm.mu.RUnlock()
    return pm.processes[name]
}
```

#### 2.2 Add ProcessWriter interface for testability
**File:** `pkg/api/process.go`

```go
// ProcessWriter is the interface for writing events to a process.
// Used for testing with mock implementations.
type ProcessWriter interface {
    WriteEvent(data string) error
}

// Ensure Process implements ProcessWriter
var _ ProcessWriter = (*Process)(nil)
```

#### 2.3 Update Server.OnMessageReceived
**File:** `pkg/api/server.go`

```go
func (s *Server) OnMessageReceived(peer PeerInfo, msg RawMessage) {
    if s.procManager == nil {
        return
    }

    // Get peer-specific API bindings from reactor
    bindings := s.reactor.GetPeerAPIBindings(peer.Address)
    if len(bindings) == 0 {
        return
    }

    for _, binding := range bindings {
        if !wantsMessageType(binding.Receive, msg.Type) {
            continue
        }

        proc := s.procManager.GetProcess(binding.ProcessName)
        if proc == nil {
            continue
        }

        // Format using THIS BINDING's resolved config
        content := ContentConfig{
            Encoding: binding.Encoding,
            Format:   binding.Format,
        }
        // FormatMessage is a package-level function in text.go
        output := FormatMessage(peer, msg, content)
        _ = proc.WriteEvent(output)
    }
}

// wantsMessageType checks if receive config wants this message type.
// Note: State events are NOT BGP messages - they're handled separately
// via OnPeerStateChange (see 2.4).
func wantsMessageType(recv ReceiveConfig, msgType message.MessageType) bool {
    switch msgType {
    case message.TypeUPDATE:
        return recv.Update
    case message.TypeOPEN:
        return recv.Open
    case message.TypeNOTIFICATION:
        return recv.Notification
    case message.TypeKEEPALIVE:
        return recv.Keepalive
    default:
        return false
    }
}
```

#### 2.4 Handle state events separately
**File:** `pkg/api/server.go`

State events (up/down/connected) are NOT BGP messages - they're session lifecycle events. Add a separate handler:

```go
// OnPeerStateChange handles peer state transitions.
// Called by reactor when peer state changes (not a BGP message).
func (s *Server) OnPeerStateChange(peer PeerInfo, state string) {
    if s.procManager == nil {
        return
    }

    bindings := s.reactor.GetPeerAPIBindings(peer.Address)
    for _, binding := range bindings {
        if !binding.Receive.State {
            continue
        }

        proc := s.procManager.GetProcess(binding.ProcessName)
        if proc == nil {
            continue
        }

        content := ContentConfig{
            Encoding: binding.Encoding,
            Format:   binding.Format,
        }
        output := FormatStateChange(peer, state, content)
        _ = proc.WriteEvent(output)
    }
}
```

Add to api/types.go:
```go
// StateChangeReceiver receives peer state change notifications.
type StateChangeReceiver interface {
    OnPeerStateChange(peer PeerInfo, state string)
}
```

#### 2.5 Add FormatStateChange function
**File:** `pkg/api/text.go`

```go
// FormatStateChange formats a peer state change event.
func FormatStateChange(peer PeerInfo, state string, content ContentConfig) string {
    if content.Encoding == EncodingJSON {
        return formatStateChangeJSON(peer, state)
    }
    return formatStateChangeText(peer, state)
}

func formatStateChangeJSON(peer PeerInfo, state string) string {
    data := map[string]any{
        "type": "state",
        "peer": map[string]any{
            "address": peer.Address.String(),
            "asn":     peer.ASN,
        },
        "state": state,
    }
    b, _ := json.Marshal(data)
    return string(b) + "\n"
}

func formatStateChangeText(peer PeerInfo, state string) string {
    return fmt.Sprintf("peer %s state %s\n", peer.Address, state)
}
```

#### 2.6 Remove old forwarding functions
**File:** `pkg/api/server.go`

Remove or deprecate:
- `forwardUpdateToProcesses`
- `forwardOpenToProcesses`
- `forwardNotificationToProcesses`
- `forwardKeepaliveToProcesses`
- `getProcessConfigByName`

All replaced by unified `OnMessageReceived` with per-binding config.

#### 2.7 Tests
**File:** `pkg/api/server_test.go`

```go
// mockReactor implements ReactorInterface for testing.
type mockReactor struct {
    bindings map[string][]PeerAPIBinding // addr -> bindings
}

func (m *mockReactor) GetPeerAPIBindings(addr netip.Addr) []PeerAPIBinding {
    return m.bindings[addr.String()]
}

// Implement other ReactorInterface methods as no-ops
func (m *mockReactor) Peers() []PeerInfo                              { return nil }
func (m *mockReactor) Stats() ReactorStats                            { return ReactorStats{} }
func (m *mockReactor) Stop()                                          {}
func (m *mockReactor) Reload() error                                  { return nil }
func (m *mockReactor) AnnounceRoute(string, RouteSpec) error          { return nil }
func (m *mockReactor) WithdrawRoute(string, netip.Prefix) error       { return nil }
// ... etc for other methods ...

// mockProcessWriter implements ProcessWriter for testing.
type mockProcessWriter struct {
    events []string
    mu     sync.Mutex
}

func (w *mockProcessWriter) WriteEvent(data string) error {
    w.mu.Lock()
    defer w.mu.Unlock()
    w.events = append(w.events, data)
    return nil
}

func (w *mockProcessWriter) Events() []string {
    w.mu.Lock()
    defer w.mu.Unlock()
    return append([]string{}, w.events...)
}

// testServer creates a Server with mock dependencies for testing.
// Uses real ProcessManager but with test process configs.
func testServer(t *testing.T, reactor *mockReactor, writers map[string]*mockProcessWriter) *Server {
    t.Helper()
    // For unit tests, we test the logic directly rather than through
    // full Server construction. See testOnMessageReceived helper.
    return nil
}

// testOnMessageReceived tests message routing logic directly.
// This avoids needing to mock the full Server/ProcessManager.
func testOnMessageReceived(
    bindings []PeerAPIBinding,
    writers map[string]*mockProcessWriter,
    peer PeerInfo,
    msg RawMessage,
) {
    for _, binding := range bindings {
        if !wantsMessageType(binding.Receive, msg.Type) {
            continue
        }
        if w, ok := writers[binding.ProcessName]; ok {
            content := ContentConfig{
                Encoding: binding.Encoding,
                Format:   binding.Format,
            }
            output := FormatMessage(peer, msg, content)
            _ = w.WriteEvent(output)
        }
    }
}

func TestPerPeerFormatting(t *testing.T) {
    // VALIDATES: Same process receives different formats from different peers
    // PREVENTS: All peers getting same format regardless of config

    peerA := netip.MustParseAddr("10.0.0.1")
    peerB := netip.MustParseAddr("10.0.0.2")

    writer := &mockProcessWriter{}
    writers := map[string]*mockProcessWriter{"collector": writer}

    bindingsA := []PeerAPIBinding{{
        ProcessName: "collector",
        Encoding:    "json",
        Format:      "parsed",
        Receive:     ReceiveConfig{Update: true},
    }}
    bindingsB := []PeerAPIBinding{{
        ProcessName: "collector",
        Encoding:    "text",
        Format:      "full",
        Receive:     ReceiveConfig{Update: true},
    }}

    // Send UPDATE from peer A (JSON)
    testOnMessageReceived(
        bindingsA,
        writers,
        PeerInfo{Address: peerA, ASN: 65001},
        RawMessage{Type: message.TypeUPDATE, RawBytes: []byte{0xff}},
    )

    // Send UPDATE from peer B (text)
    testOnMessageReceived(
        bindingsB,
        writers,
        PeerInfo{Address: peerB, ASN: 65002},
        RawMessage{Type: message.TypeUPDATE, RawBytes: []byte{0xff}},
    )

    events := writer.Events()
    require.Len(t, events, 2)

    // Peer A should get JSON
    assert.Contains(t, events[0], `"type"`)
    assert.Contains(t, events[0], `"peer"`)

    // Peer B should get text with raw bytes (full format)
    assert.Contains(t, events[1], "peer 10.0.0.2")
    assert.Contains(t, events[1], "raw")
}

func TestMessageTypeFiltering(t *testing.T) {
    // VALIDATES: Process only receives subscribed message types
    // PREVENTS: Processes getting messages they didn't subscribe to

    peer := netip.MustParseAddr("10.0.0.1")
    writer := &mockProcessWriter{}
    writers := map[string]*mockProcessWriter{"updater": writer}

    bindings := []PeerAPIBinding{{
        ProcessName: "updater",
        Encoding:    "text",
        Format:      "parsed",
        Receive:     ReceiveConfig{Update: true}, // ONLY update
    }}

    peerInfo := PeerInfo{Address: peer, ASN: 65001}

    // Send UPDATE -> should be forwarded
    testOnMessageReceived(bindings, writers, peerInfo, RawMessage{Type: message.TypeUPDATE})
    assert.Len(t, writer.Events(), 1, "UPDATE should be forwarded")

    // Send NOTIFICATION -> should NOT be forwarded
    testOnMessageReceived(bindings, writers, peerInfo, RawMessage{Type: message.TypeNOTIFICATION})
    assert.Len(t, writer.Events(), 1, "NOTIFICATION should NOT be forwarded")

    // Send KEEPALIVE -> should NOT be forwarded
    testOnMessageReceived(bindings, writers, peerInfo, RawMessage{Type: message.TypeKEEPALIVE})
    assert.Len(t, writer.Events(), 1, "KEEPALIVE should NOT be forwarded")
}

// testOnPeerStateChange tests state event routing logic directly.
func testOnPeerStateChange(
    bindings []PeerAPIBinding,
    writers map[string]*mockProcessWriter,
    peer PeerInfo,
    state string,
) {
    for _, binding := range bindings {
        if !binding.Receive.State {
            continue
        }
        if w, ok := writers[binding.ProcessName]; ok {
            content := ContentConfig{
                Encoding: binding.Encoding,
                Format:   binding.Format,
            }
            output := FormatStateChange(peer, state, content)
            _ = w.WriteEvent(output)
        }
    }
}

func TestStateEventFiltering(t *testing.T) {
    // VALIDATES: State events only sent to processes with Receive.State=true
    // PREVENTS: Processes getting state events they didn't subscribe to

    peer := netip.MustParseAddr("10.0.0.1")
    writerWithState := &mockProcessWriter{}
    writerWithoutState := &mockProcessWriter{}

    writers := map[string]*mockProcessWriter{
        "with-state":    writerWithState,
        "without-state": writerWithoutState,
    }

    bindings := []PeerAPIBinding{
        {
            ProcessName: "with-state",
            Encoding:    "json",
            Format:      "parsed",
            Receive:     ReceiveConfig{Update: true, State: true},
        },
        {
            ProcessName: "without-state",
            Encoding:    "json",
            Format:      "parsed",
            Receive:     ReceiveConfig{Update: true, State: false},
        },
    }

    // Send state change
    testOnPeerStateChange(bindings, writers, PeerInfo{Address: peer, ASN: 65001}, "established")

    assert.Len(t, writerWithState.Events(), 1, "with-state should receive state event")
    assert.Len(t, writerWithoutState.Events(), 0, "without-state should NOT receive state event")
}
```

### Phase 3: Output Format Update

#### 3.1 Add Version field to structs
Add `Version int` to:
- `config.PeerContentConfig`
- `reactor.PeerContentConfig`
- `api.PeerAPIBinding`

Add to schema:
```go
Field("content", Container(
    Field("encoding", Leaf(TypeString)),
    Field("format", Leaf(TypeString)),
    Field("version", Leaf(TypeInt)),  // 6=legacy, 7=nlri
)),
```

Add parsing:
```go
if v, ok := content.Get("version"); ok {
    if n, err := strconv.Atoi(v); err == nil {
        binding.Content.Version = n
    }
}
```

Add resolution in reactorAPIAdapter:
```go
version := b.Content.Version
if version == 0 {
    version = 7 // Default to new nlri format
}
```

#### 3.2 Add JSON v7 encoder
**File:** `pkg/api/json.go`

New methods for v7 format with `announce.nlri` structure.

#### 3.3 Add Text v7 encoder
**File:** `pkg/api/text.go`

New format: `peer <addr> update announce nlri <family> <prefix> [attrs...]`

#### 3.4 Version-aware formatting
Update `FormatMessage` to check Version and use appropriate encoder:
```go
func FormatMessage(peer PeerInfo, msg RawMessage, content ContentConfig) string {
    version := content.Version
    if version == 0 {
        version = 7
    }

    if content.Encoding == EncodingJSON {
        if version == 6 {
            return formatMessageJSONv6(peer, msg, content)
        }
        return formatMessageJSONv7(peer, msg, content)
    }
    // Text format
    if version == 6 {
        return formatMessageTextv6(peer, msg, content)
    }
    return formatMessageTextv7(peer, msg, content)
}
```

### Phase 4: Migration Tool Update

Update ExaBGP config migrator to:
1. Keep `encoder` in process (for backward compat)
2. Add `api <name> { content {...}; receive {...} }` to peers
3. Map ExaBGP api blocks to new format

### Phase 5: Documentation Updates

- [ ] Update `.claude/zebgp/api/ARCHITECTURE.md`
- [ ] Update `.claude/zebgp/config/SYNTAX.md`
- [ ] Update `plan/CLAUDE_CONTINUATION.md`

## Verification Checklist

### Phase 0 (Partial)
- [x] Message dispatch wired for all types
- [x] Encoding switching works (json/text)
- [ ] Per-message-type flags (currently all use ReceiveUpdate)

### Phase 1: Config Schema + Data Flow (COMPLETE - 2025-12-30)

**All items completed:**
- [x] Structs: PeerAPIBinding, PeerContentConfig, PeerReceiveConfig, PeerSendConfig
- [x] PeerConfig has APIBindings field
- [x] PeerSettings has APIBindings field (reactor.APIBinding)
- [x] api.PeerAPIBinding type added to types.go
- [x] Validation: Error on undefined process reference
- [x] Loader: config → reactor conversion (configToPeer copies APIBindings)
- [x] ReactorInterface: GetPeerAPIBindings method added
- [x] Schema: `api` uses `List(TypeString, ...)` with `_anonymous` for old syntax
- [x] New syntax: `api <name> { content {...} }` parses correctly
- [x] Parsing: content/encoding/format extraction works
- [x] Parsing: receive flags (update, open, etc.) work
- [x] Parsing: send flags (update, refresh) work
- [x] Parsing: `all;` keyword expansion works
- [x] reactorAPIAdapter: Encoding inheritance from process (peer → process → "text")
- [x] Old syntax: `neighbor-changes;` maps to `receive.State`
- [x] ExaBGP compat: `processes-match`, `neighbor-changes` schema fields
- [x] Tests: 8 new tests for new syntax
- [x] `make test && make lint` pass

**Deferred to future phases:**
- [ ] Validation: Invalid encoding/format values (low priority)

### Phase 2: Message Routing (COMPLETE - 2025-12-30)

**All items completed:**
- [x] GetProcess method added to ProcessManager
- [x] ProcessWriter interface for testability
- [x] Server.OnMessageReceived uses GetPeerAPIBindings
- [x] Per-binding format/encoding applied via formatMessage()
- [x] Old forwarding functions removed (forwardUpdateToProcesses, etc.)
- [x] wantsMessageType helper for message type filtering
- [x] OnPeerStateChange for state events (separate from BGP messages)
- [x] FormatStateChange function (text and JSON)
- [x] Tests: TestWantsMessageType, TestFormatStateChange
- [x] `make test && make lint` pass

### Phase 3: Output Format (COMPLETE - 2025-12-30)

**All items completed:**
- [x] Version field added to structs (config, reactor, api)
- [x] Version parsing added to config schema and parser
- [x] Version inheritance in reactor (peer → default 7)
- [x] JSON v7 encoder with announce.nlri structure
- [x] Text v7 encoder with "peer X update announce nlri ..." format
- [x] Version-aware FormatMessage (routes to v6/v7 formatters)
- [x] Tests: TestFormatMessageV7Text, TestFormatMessageV7JSON, TestFormatMessageV6VsV7, TestContentConfigVersionDefault
- [x] `make test && make lint` pass

### Phase 4: Migration (COMPLETE - 2025-12-30)

**All items completed:**
- [x] MigrateAPIBlocks function added to pkg/config/migration/api.go
- [x] Integrated into MigrateV2ToV3 as Step 5
- [x] Converts: `api { processes [ foo ]; neighbor-changes; }` → `api foo { receive { state; } }`
- [x] Multiple processes create multiple named api blocks
- [x] neighbor-changes flag maps to receive { state; }
- [x] Preserves already-migrated new syntax
- [x] Works in peer, template.group, template.match blocks
- [x] processes-match patterns migrated to named blocks
- [x] Error on empty/missing processes (ErrEmptyProcesses)
- [x] Error on duplicate process names (ErrDuplicateProcess)
- [x] Error on collision with existing named blocks (ErrAPICollision)
- [x] v2 neighbor→peer integration test
- [x] 16 tests for api migration
- [x] `make test && make lint` pass

### Phase 5: Documentation (COMPLETE - 2025-12-30)

**All items completed:**
- [x] ARCHITECTURE.md: Already had API binding docs, verified current
- [x] SYNTAX.md: Updated with new api syntax, migration info, peer keyword
- [x] CONTINUATION.md: Added API encoder switching to completed section
- [x] `make test && make lint` pass

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- Tests MUST exist and FAIL before implementation code exists
- Run `make test && make lint` before claiming done
- NEVER discard uncommitted work without explicit user permission
- Verify before claiming: run commands, paste output as proof
- Tests passing is NOT permission to commit - wait for user

### From TDD_ENFORCEMENT.md
- Tests MUST exist and FAIL before implementation begins
- Every test MUST have VALIDATES and PREVENTS documentation
- Show failure output, then implementation, then pass output

### From ESSENTIAL_PROTOCOLS.md
- Post-completion self-review is MANDATORY
- Fix all 🔴/🟡 issues before claiming done
- Report 🟢 minor items to user

---

**Created:** 2025-12-29
**Updated:** 2025-12-29 (Final: GetProcess, ProcessWriter interface, mock tests fixed, Version deferred to Phase 3, FormatStateChange added)
