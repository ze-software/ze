# Spec: api-encoder-switching

## Task

Redesign process API configuration to properly separate:
1. **WHAT** messages to receive/send (message types)
2. **HOW** to format them (encoding + format)

Fix the current bug where `encoder json` processes receive nothing or text format.

## Current State (verified)

- `make test`: PASS
- `make lint`: PASS (0 issues)
- **Phase 0 COMPLETE**: All message types dispatched
- **Phase 1 NEXT**: Config schema for `content {}` block
- Last verified: 2025-12-29

### Next Session Checklist

Before implementing Phase 1, read these files:
1. `pkg/config/bgp.go:286-290` - current schema
2. `pkg/config/bgp.go:505-510` - config.ProcessConfig
3. `pkg/config/bgp.go:538-551` - parsing logic
4. `pkg/reactor/reactor.go:57-63` - reactor.APIProcessConfig
5. `pkg/config/loader.go:111-116` - config→reactor conversion
6. `pkg/reactor/reactor.go:1538-1545` - reactor→api conversion
7. `pkg/api/types.go:336-344` - api.ProcessConfig

### Completed (Phase 0 FULL)

| Item | File | Status |
|------|------|--------|
| `RawMessage` type | `pkg/api/types.go:377-383` | ✅ |
| `MessageReceiver` interface | `pkg/reactor/reactor.go:75-82` | ✅ |
| `notifyMessageReceiver` | `pkg/reactor/reactor.go:1419-1460` | ✅ |
| `DecodeUpdate`/`DecodeUpdateRoutes` | `pkg/api/decode.go` | ✅ |
| `DecodeOpen` | `pkg/api/decode.go:334-350` | ✅ |
| `DecodeNotification` | `pkg/api/decode.go:365-384` | ✅ |
| `ContentConfig` type + `WithDefaults()` | `pkg/api/types.go:359-375` | ✅ |
| Format constants (`FormatParsed/Raw/Full`) | `pkg/api/types.go:352-357` | ✅ |
| Encoding constants (`EncodingJSON/Text`) | `pkg/api/text.go:9-13` | ✅ |
| `ReceivedRoute.ToRouteUpdate()` | `pkg/api/text.go:109-125` | ✅ |
| `FormatReceivedUpdateWithEncoding()` | `pkg/api/text.go:127-145` | ✅ |
| `FormatMessage()` format switching | `pkg/api/text.go:147-160` | ✅ |
| `FormatOpen` / `FormatNotification` / `FormatKeepalive` | `pkg/api/text.go:223-249` | ✅ |
| `JSONEncoder.Open/Notification/Keepalive` | `pkg/api/json.go:211-257` | ✅ |
| `Server.OnMessageReceived` dispatch all types | `pkg/api/server.go:407-424` | ✅ |
| `forwardOpenToProcesses` | `pkg/api/server.go:463-482` | ✅ |
| `forwardNotificationToProcesses` | `pkg/api/server.go:485-504` | ✅ |
| `forwardKeepaliveToProcesses` | `pkg/api/server.go:507-526` | ✅ |
| Session callback wiring | `pkg/reactor/reactor.go:1475` | ✅ |
| Unit tests | `pkg/api/message_receiver_test.go` | ✅ |

### Primary Bug Fixed

**Before:** `OnUpdateReceived` always used text format, ignoring `cfg.Encoder`
**After:** Uses `s.encoder.RouteAnnounce()` for JSON, `FormatReceivedUpdate()` for text

### Phase 0 Complete

All message types now flow through the API:
- UPDATE → `DecodeUpdateRoutes` → `forwardUpdateToProcesses`
- OPEN → `DecodeOpen` → `forwardOpenToProcesses`
- NOTIFICATION → `DecodeNotification` → `forwardNotificationToProcesses`
- KEEPALIVE → `forwardKeepaliveToProcesses`

### Remaining (Phase 1-3)

| Item | Status |
|------|--------|
| Config schema for `content {}` block | ⏳ |
| Per-neighbor process binding | ⏳ |
| Migration tool update | ⏳ |

## New ZeBGP Process Configuration Format

### Design Principles

1. **Separate concerns:** WHAT (message types) vs HOW (formatting)
2. **ExaBGP compatible:** Migration tool converts ExaBGP → ZeBGP
3. **Cleaner defaults:** Less boilerplate than ExaBGP

### ZeBGP Format

```
process <name> {
    run <command>;

    # HOW to format output
    content {
        encoding json;       # json | text (default: text)
        format parsed;       # parsed | raw | full (default: parsed)
    }
}

neighbor <address> {
    # WHAT messages this process receives/sends for this neighbor
    process <name> {
        receive {
            update;          # route announcements
            withdraw;        # route withdrawals
            open;            # OPEN messages
            notification;    # errors
            keepalive;       # heartbeats
            refresh;         # route-refresh
            state;           # up/down/connected/fsm events
        }
        send {
            update;          # can inject routes
            # ... other message types
        }
    }
}
```

### Shorthand Forms

```
# Minimal (defaults: text, parsed, no messages)
process foo { run ./script; }

# Common case: JSON updates only
process foo {
    run ./script;
    content { encoding json; }
}

neighbor 10.0.0.1 {
    process foo {
        receive { update; withdraw; }
    }
}

# All messages, full format (parsed + raw)
process foo {
    run ./script;
    content { format full; }
}

neighbor 10.0.0.1 {
    process foo {
        receive { all; }
        send { all; }
    }
}
```

### Format Options

| Option | Description |
|--------|-------------|
| `encoding json` | JSON format output |
| `encoding text` | Text format output (default) |
| `format parsed` | Decoded/interpreted fields only (default) |
| `format raw` | Wire bytes only (hex header + body) |
| `format full` | Both parsed content AND raw bytes |

### ExaBGP → ZeBGP Mapping

| ExaBGP | ZeBGP |
|--------|-------|
| `encoder json` | `content { encoding json; }` |
| `receive { parsed; }` | `content { format parsed; }` |
| `receive { packets; }` | `content { format raw; }` |
| `receive { parsed; packets; consolidate; }` | `content { format full; }` |
| `api { processes [ foo ]; }` in neighbor | `process foo { ... }` in neighbor |
| `receive { update; }` in api | `receive { update; }` in process |
| `neighbor-changes;` | `receive { state; }` |

### Migration Example

**ExaBGP input:**
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

**ZeBGP output:**
```
process foo {
    run ./script.py;
    content {
        encoding json;
        format full;
    }
}

neighbor 10.0.0.1 {
    process foo {
        receive {
            update;
            notification;
        }
    }
}
```

## Problem Analysis (Current Code)

**Primary bug:** `OnUpdateReceived` ignores `cfg.Encoder`, always uses text format.

**Secondary bug:** Config parsing at `bgp.go:548` only sets `ReceiveUpdate=true` when `encoder=text`. JSON-configured processes have `ReceiveUpdate=false`, so they receive **nothing**.

**Missing feature:** No `OnWithdrawReceived` exists. Withdrawals are not forwarded to processes at all.

**Architectural issue (config):** Current config mixes "what" and "how" at wrong levels.

**Architectural issue (interface):** `UpdateReceiver.OnUpdateReceived(peerAddr netip.Addr, routes)` only passes peer address, but `JSONEncoder.RouteAnnounce(peer PeerInfo, routes)` requires full `PeerInfo` (LocalAddress, LocalAS, PeerAS, RouterID). The interface signature must change.

### Interface Mismatch Detail

```go
// Current interface (pkg/reactor/reactor.go:77-80)
type UpdateReceiver interface {
    OnUpdateReceived(peerAddr netip.Addr, routes []api.ReceivedRoute)
}

// JSONEncoder.RouteAnnounce requires (pkg/api/json.go:123)
func (e *JSONEncoder) RouteAnnounce(peer PeerInfo, routes []RouteUpdate) string

// PeerInfo fields needed (pkg/api/types.go:57-71)
type PeerInfo struct {
    Address      netip.Addr  // ✓ We have this
    LocalAddress netip.Addr  // ✗ Missing
    LocalAS      uint32      // ✗ Missing
    PeerAS       uint32      // ✗ Missing
    RouterID     uint32      // ✗ Missing
    State        string
    Uptime       time.Duration
    // ... stats fields
}
```

### Type Mismatch Detail

```go
// Text encoder uses (pkg/api/text.go:17-24)
type ReceivedRoute struct {
    Prefix          netip.Prefix
    NextHop         netip.Addr
    Origin          string
    LocalPreference uint32
    MED             uint32
    ASPath          []uint32
}

// JSON encoder uses (pkg/api/json.go:243-253)
type RouteUpdate struct {
    Prefix    string   // String, not netip.Prefix
    NextHop   string   // String, not netip.Addr
    AFI       string   // "ipv4" or "ipv6"
    SAFI      string   // "unicast", etc.
    Origin    string
    ASPath    []uint32
    LocalPref uint32
    MED       uint32
}
```

Need converter: `ReceivedRoute` → `RouteUpdate`

## Embedded Protocol Requirements

### Default Rules (ALL tasks)
- **FIRST:** Run `git status` - if modified files exist, ASK user before proceeding
- **FIRST:** Read `plan/CLAUDE_CONTINUATION.md` for current state
- **FIRST:** Read `.claude/ESSENTIAL_PROTOCOLS.md` for session rules
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

## Implementation Phases

### Phase 0: Raw Message Interface (Correct Design)

**Key principle:** Pass raw wire bytes, decode on demand based on format config.

Current (wrong):
```
Reactor → parse UPDATE → ReceivedRoute struct → Server → format text
```

Correct:
```
Reactor → RawMessage{type, bytes, peer} → Server → decode IF needed → format
```

#### 0.1 Define RawMessage type
**File:** `pkg/api/types.go`

```go
// RawMessage represents a BGP message received from a peer.
// Contains raw wire bytes for on-demand parsing.
type RawMessage struct {
    Type      message.MessageType // UPDATE, OPEN, NOTIFICATION, etc.
    RawBytes  []byte              // Original wire bytes (without marker)
    Timestamp time.Time
}
```

#### 0.2 Define MessageReceiver interface
**File:** `pkg/reactor/reactor.go`

Replace `UpdateReceiver` with generic `MessageReceiver`:

```go
// MessageReceiver receives BGP messages from peers.
// Messages are passed as raw bytes for on-demand parsing.
type MessageReceiver interface {
    // OnMessageReceived is called when a message is received from a peer.
    // msg contains raw wire bytes - parsing is done by receiver based on format config.
    OnMessageReceived(peer api.PeerInfo, msg api.RawMessage)
}
```

#### 0.3 Update notifyMessageReceiver
**File:** `pkg/reactor/reactor.go`

```go
func (r *Reactor) notifyMessageReceiver(peerAddr netip.Addr, msgType message.MessageType, rawBytes []byte) {
    r.mu.RLock()
    receiver := r.messageReceiver
    peer, hasPeer := r.peers[peerAddr.String()]
    r.mu.RUnlock()

    if receiver == nil || !hasPeer {
        return
    }

    // Build PeerInfo from peer settings
    s := peer.Settings()
    peerInfo := api.PeerInfo{
        Address:      s.Address,
        LocalAddress: peer.LocalAddress(),
        LocalAS:      s.LocalAS,
        PeerAS:       s.PeerAS,
        RouterID:     s.RouterID,
        State:        peer.State(),
    }

    msg := api.RawMessage{
        Type:      msgType,
        RawBytes:  rawBytes,
        Timestamp: time.Now(),
    }

    receiver.OnMessageReceived(peerInfo, msg)
}
```

#### 0.4 Add on-demand parsers
**File:** `pkg/api/decode.go`

```go
// DecodeUpdate parses raw UPDATE bytes into structured data.
// Called only when format=parsed or format=full.
func DecodeUpdate(rawBytes []byte, ctx *context.EncodingContext) (*ParsedUpdate, error) {
    // Parse using existing message.ParseUpdate()
}

// DecodeOpen parses raw OPEN bytes into structured data.
func DecodeOpen(rawBytes []byte) (*ParsedOpen, error) {
    // Parse using existing message.ParseOpen()
}

// DecodeNotification parses raw NOTIFICATION bytes into structured data.
func DecodeNotification(rawBytes []byte) (*ParsedNotification, error) {
    // Parse using existing message.ParseNotification()
}
```

#### 0.5 Update Server.OnMessageReceived
**File:** `pkg/api/server.go`

```go
func (s *Server) OnMessageReceived(peer PeerInfo, msg RawMessage) {
    if s.procManager == nil {
        return
    }

    s.procManager.mu.RLock()
    defer s.procManager.mu.RUnlock()

    for name, proc := range s.procManager.processes {
        cfg := s.getProcessConfig(name)
        if cfg == nil || !cfg.wantsMessage(msg.Type) {
            continue
        }

        output := s.formatMessage(peer, msg, cfg.Content)
        _ = proc.WriteEvent(output)
    }
}

func (s *Server) formatMessage(peer PeerInfo, msg RawMessage, content ContentConfig) string {
    switch content.Format {
    case "raw":
        // Just hex-encode the wire bytes
        return s.formatRaw(peer, msg, content.Encoding)

    case "parsed":
        // Decode and format structured data
        return s.formatParsed(peer, msg, content.Encoding)

    case "full":
        // Both parsed AND raw
        return s.formatFull(peer, msg, content.Encoding)

    default:
        return s.formatParsed(peer, msg, content.Encoding)
    }
}

func (s *Server) formatParsed(peer PeerInfo, msg RawMessage, encoding string) string {
    switch msg.Type {
    case message.TypeUPDATE:
        parsed, err := DecodeUpdate(msg.RawBytes, nil)
        if err != nil {
            return s.formatError(peer, msg, err, encoding)
        }
        if encoding == "json" {
            return s.encoder.Update(peer, parsed)
        }
        return FormatTextUpdate(peer.Address, parsed)

    case message.TypeOPEN:
        parsed, err := DecodeOpen(msg.RawBytes)
        if err != nil {
            return s.formatError(peer, msg, err, encoding)
        }
        if encoding == "json" {
            return s.encoder.Open(peer, parsed)
        }
        return FormatTextOpen(peer.Address, parsed)

    // ... other message types
    }
}
```

#### 0.6 Wire up session to pass raw bytes
**File:** `pkg/reactor/session.go`

When message is received, call `notifyMessageReceiver` with raw bytes before or instead of parsing:

```go
func (s *Session) handleMessage(header *message.Header, body []byte) {
    // Notify receiver with raw bytes
    if s.peer.messageCallback != nil {
        s.peer.messageCallback(s.peerAddr, header.Type, body)
    }

    // Continue with normal processing...
}
```

#### 0.7 Run tests
```bash
make test && make lint
```

### Phase 1: Config Schema Update

Update config parsing to support `content { encoding; format; }` block.

#### Context (READ THIS FIRST)

**Three ProcessConfig structs exist (all need Format field):**

| Struct | File | Line | Purpose |
|--------|------|------|---------|
| `config.ProcessConfig` | `pkg/config/bgp.go` | 505-510 | Config parsing output |
| `reactor.APIProcessConfig` | `pkg/reactor/reactor.go` | 57-63 | Intermediate storage |
| `api.ProcessConfig` | `pkg/api/types.go` | 336-344 | API server input |

**Two conversion points (both need Format copying):**

| From | To | File | Line |
|------|----|------|------|
| config → reactor | `loader.go` | 111-116 |
| reactor → api | `reactor.go` | 1538-1545 |

**ContentConfig already exists:** `pkg/api/types.go:359-375`

**Current schema definition:** `pkg/config/bgp.go:286-290`
```go
schema.Define("process", List(TypeString,
    Field("run", MultiLeaf(TypeString)),
    Field("encoder", Leaf(TypeString)),
    Field("respawn", Leaf(TypeBool)),
))
```

**Current parsing:** `pkg/config/bgp.go:538-551`

#### 1.1 Update config schema
**File:** `pkg/config/bgp.go:286-290`

Add `content` container with `encoding` and `format` fields:
```go
schema.Define("process", List(TypeString,
    Field("run", MultiLeaf(TypeString)),
    Field("encoder", Leaf(TypeString)),      // Keep for backward compat
    Field("respawn", Leaf(TypeBool)),
    Field("content", Container(              // NEW
        Field("encoding", Leaf(TypeString)), // json | text
        Field("format", Leaf(TypeString)),   // parsed | raw | full
    )),
))
```

#### 1.2 Update config.ProcessConfig
**File:** `pkg/config/bgp.go:505-510`

Add Format field:
```go
type ProcessConfig struct {
    Name          string
    Run           string
    Encoder       string
    Format        string // NEW: parsed | raw | full
    ReceiveUpdate bool
}
```

#### 1.3 Update parsing logic
**File:** `pkg/config/bgp.go:538-551`

Parse content block (with backward compat for top-level encoder):
```go
for name, proc := range tree.GetList("process") {
    pc := ProcessConfig{Name: name}
    if v, ok := proc.Get("run"); ok { pc.Run = v }

    // NEW: Check content block first
    if content := proc.GetContainer("content"); content != nil {
        if v, ok := content.Get("encoding"); ok { pc.Encoder = v }
        if v, ok := content.Get("format"); ok { pc.Format = v }
    }
    // Backward compat: top-level encoder overrides
    if v, ok := proc.Get("encoder"); ok { pc.Encoder = v }

    // Defaults
    if pc.Encoder == "" { pc.Encoder = "text" }
    if pc.Format == "" { pc.Format = "parsed" }
    if pc.Encoder == "text" { pc.ReceiveUpdate = true }

    cfg.Processes = append(cfg.Processes, pc)
}
```

#### 1.4 Update reactor.APIProcessConfig
**File:** `pkg/reactor/reactor.go:57-63`

Add Format field:
```go
type APIProcessConfig struct {
    Name          string
    Run           string
    Encoder       string
    Format        string // NEW
    Respawn       bool
    ReceiveUpdate bool
}
```

#### 1.5 Update loader conversion
**File:** `pkg/config/loader.go:111-116`

Copy Format field:
```go
reactorCfg.APIProcesses = append(reactorCfg.APIProcesses, reactor.APIProcessConfig{
    Name:          pc.Name,
    Run:           pc.Run,
    Encoder:       pc.Encoder,
    Format:        pc.Format,  // NEW
    ReceiveUpdate: pc.ReceiveUpdate,
})
```

#### 1.6 Update api.ProcessConfig
**File:** `pkg/api/types.go:336-344`

Add Format field:
```go
type ProcessConfig struct {
    Name           string
    Run            string
    Encoder        string
    Format         string // NEW: parsed | raw | full
    Respawn        bool
    RespawnEnabled bool
    WorkDir        string
    ReceiveUpdate  bool
}
```

#### 1.7 Update reactor→api conversion
**File:** `pkg/reactor/reactor.go:1538-1545`

Copy Format field:
```go
apiConfig.Processes = append(apiConfig.Processes, api.ProcessConfig{
    Name:          pc.Name,
    Run:           pc.Run,
    Encoder:       pc.Encoder,
    Format:        pc.Format,  // NEW
    Respawn:       pc.Respawn,
    WorkDir:       r.config.ConfigDir,
    ReceiveUpdate: pc.ReceiveUpdate,
})
```

#### 1.8 Wire Format into message forwarding
**File:** `pkg/api/server.go` (forwardUpdateToProcesses, etc.)

Use `cfg.Format` when calling format functions. The format switching code
already exists in `FormatMessage()` - just need to pass the config value.

#### 1.9 Tests
**File:** `pkg/config/bgp_test.go`

```go
func TestProcessContentConfig(t *testing.T) {
    tests := []struct {
        name         string
        config       string
        wantEncoding string
        wantFormat   string
    }{
        {
            name: "content block",
            config: `process foo { run ./test; content { encoding json; format full; } }`,
            wantEncoding: "json",
            wantFormat:   "full",
        },
        {
            name: "backward compat encoder",
            config: `process foo { run ./test; encoder text; }`,
            wantEncoding: "text",
            wantFormat:   "parsed", // default
        },
        {
            name: "defaults",
            config: `process foo { run ./test; }`,
            wantEncoding: "text",
            wantFormat:   "parsed",
        },
    }
    // ... test implementation
}
```

#### 1.10 Verification
```bash
make test && make lint
```

### Phase 2: Message Routing

#### 2.1 Add message type filtering
**File:** `pkg/api/server.go`

Route messages only to processes subscribed to that type for that neighbor.

#### 2.2 Fix encoder switching
**File:** `pkg/api/server.go`

Use `ContentConfig.Encoding` to select JSON vs text formatter.

#### 2.3 Add format handling
**File:** `pkg/api/json.go`, `pkg/api/text.go`

Support `parsed`, `raw`, and `full` output formats.

### Phase 3: Migration Tool Update

#### 3.1 Add process API transform
**File:** `pkg/config/migration/process_api.go`

Transform ExaBGP process + api blocks to ZeBGP format:
- Extract `encoder` → `content.encoding`
- Map `receive { parsed; packets; consolidate; }` → `content.format`
- Move `api { processes [...] }` → `neighbor.process.<name>`
- Map message type subscriptions

### Phase 4: Verification

```bash
make test && make lint
```

Test migration:
```bash
zebgp config import exabgp.conf > zebgp.conf
zebgp config check zebgp.conf
```

## Verification Checklist

### Phase 0: Raw Message Interface ✅ COMPLETE
- [x] `RawMessage` type defined with Type, RawBytes, Timestamp
- [x] `MessageReceiver` interface replaces `UpdateReceiver` (`reactor.go:75-82`)
- [x] `notifyMessageReceiver` passes raw bytes + PeerInfo (`reactor.go:1419-1460`)
- [x] On-demand parser: `DecodeUpdate`, `DecodeUpdateRoutes` (`decode.go`)
- [x] On-demand parser: `DecodeOpen` (`decode.go:334-350`)
- [x] On-demand parser: `DecodeNotification` (`decode.go:365-384`)
- [x] `Server.OnMessageReceived` dispatches UPDATE (`server.go:413-415`)
- [x] `Server.OnMessageReceived` dispatches OPEN (`server.go:416-418`)
- [x] `Server.OnMessageReceived` dispatches NOTIFICATION (`server.go:419-421`)
- [x] `Server.OnMessageReceived` dispatches KEEPALIVE (`server.go:422-423`)
- [x] Format switching: raw, parsed, full (via `FormatMessage`)
- [x] Encoding switching: json, text (via `forwardUpdateToProcesses`)
- [x] Session wired to pass raw bytes via `peer.messageCallback` (`reactor.go:1475`)
- [x] All message type tests pass

### Phase 0 Infrastructure (Done)
- [x] `ContentConfig` type with `WithDefaults()`
- [x] Format constants: `FormatParsed`, `FormatRaw`, `FormatFull`
- [x] Encoding constants: `EncodingJSON`, `EncodingText`
- [x] `ReceivedRoute.ToRouteUpdate()` conversion
- [x] `FormatReceivedUpdateWithEncoding()` helper
- [x] `Server.OnUpdateReceived` uses encoder config
- [x] `Server.lookupPeer()` gets full PeerInfo from reactor
- [x] Tests: `TestRawMessageType`, `TestEncodingSwitchingJSON`, `TestFormatSwitchingParsedRawFull`, `TestContentConfigDefaults`, `TestReceivedRouteToRouteUpdate`

### Phase 1: Config Schema (NEXT)
- [x] ContentConfig struct added with Encoding and Format fields
- [ ] Schema: Add `content { encoding; format; }` to process definition
- [ ] Struct: Add Format to `config.ProcessConfig` (bgp.go:505)
- [ ] Struct: Add Format to `reactor.APIProcessConfig` (reactor.go:57)
- [ ] Struct: Add Format to `api.ProcessConfig` (types.go:336)
- [ ] Parsing: Extract content.encoding and content.format (bgp.go:538)
- [ ] Conversion: Copy Format in loader.go:111
- [ ] Conversion: Copy Format in reactor.go:1538
- [ ] Wiring: Use Format in server.go forwarding functions
- [ ] Tests: TestProcessContentConfig in bgp_test.go

### Phase 2-3: Per-Neighbor Binding (Future)
- [ ] Per-neighbor process binding works
- [ ] Message type filtering routes correctly
- [ ] Migration tool converts ExaBGP api blocks

### Final Verification
- [x] `make test` passes
- [x] `make lint` passes
- [x] **Goal verified**: Process with `encoder json` receives JSON format
- [x] **Goal verified**: Process with `encoder text` receives text format
- [x] Self-review performed
- [x] No 🔴/🟡 issues remaining

## Test Specification

### Phase 0 Tests

#### TestMessageReceiverReceivesRawBytes

```go
// TestMessageReceiverReceivesRawBytes verifies that MessageReceiver
// receives raw wire bytes, not pre-parsed structures.
//
// VALIDATES: Raw bytes are passed through for on-demand parsing.
//
// PREVENTS: Bug where messages are pre-parsed, wasting CPU for format=raw.
func TestMessageReceiverReceivesRawBytes(t *testing.T) {
    var receivedPeer api.PeerInfo
    var receivedMsg api.RawMessage

    mockReceiver := &mockMessageReceiver{
        onMessage: func(peer api.PeerInfo, msg api.RawMessage) {
            receivedPeer = peer
            receivedMsg = msg
        },
    }

    // Setup reactor with peer and mock receiver
    reactor := NewReactor(...)
    reactor.SetMessageReceiver(mockReceiver)
    reactor.AddPeer(&PeerSettings{
        Address: netip.MustParseAddr("192.168.1.2"),
        LocalAS: 65001,
        PeerAS:  65002,
    })

    // Simulate receiving UPDATE with known wire bytes
    updateBytes := []byte{0x00, 0x00, 0x00, 0x17, ...} // Valid UPDATE
    reactor.injectMessage(netip.MustParseAddr("192.168.1.2"), message.TypeUPDATE, updateBytes)

    // Assert raw bytes are passed through
    require.Equal(t, message.TypeUPDATE, receivedMsg.Type)
    require.Equal(t, updateBytes, receivedMsg.RawBytes)

    // Assert PeerInfo has all required fields
    require.Equal(t, netip.MustParseAddr("192.168.1.2"), receivedPeer.Address)
    require.Equal(t, uint32(65001), receivedPeer.LocalAS)
    require.Equal(t, uint32(65002), receivedPeer.PeerAS)
}
```

#### TestFormatSwitching

```go
// TestFormatSwitching verifies that format config controls parsing behavior.
//
// VALIDATES: format=raw doesn't parse, format=parsed parses, format=full does both.
//
// PREVENTS: Bug where parsing always happens regardless of format setting.
func TestFormatSwitching(t *testing.T) {
    updateBytes := []byte{...} // Valid UPDATE wire bytes

    tests := []struct {
        name       string
        format     string
        encoding   string
        wantRaw    bool // Output contains hex bytes
        wantParsed bool // Output contains parsed fields
    }{
        {"raw json", "raw", "json", true, false},
        {"raw text", "raw", "text", true, false},
        {"parsed json", "parsed", "json", false, true},
        {"parsed text", "parsed", "text", false, true},
        {"full json", "full", "json", true, true},
        {"full text", "full", "text", true, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg := ContentConfig{Format: tt.format, Encoding: tt.encoding}
            output := formatMessage(peer, RawMessage{Type: message.TypeUPDATE, RawBytes: updateBytes}, cfg)

            if tt.wantRaw {
                require.Contains(t, output, "raw") // Contains hex-encoded bytes
            }
            if tt.wantParsed {
                require.Contains(t, output, "announce") // Contains parsed data
            }
        })
    }
}
```

#### TestEncodingSwitching

```go
// TestEncodingSwitching verifies that encoding config controls output format.
//
// VALIDATES: encoding=json produces JSON, encoding=text produces text.
//
// PREVENTS: Bug where all processes receive text format.
func TestEncodingSwitching(t *testing.T) {
    jsonReceived := make(chan string, 1)
    textReceived := make(chan string, 1)

    jsonProc := &mockProcess{onWrite: func(s string) { jsonReceived <- s }}
    textProc := &mockProcess{onWrite: func(s string) { textReceived <- s }}

    server := &Server{
        config: ServerConfig{
            Processes: []ProcessConfig{
                {Name: "json-proc", Content: ContentConfig{Encoding: "json", Format: "parsed"}, ReceiveUpdate: true},
                {Name: "text-proc", Content: ContentConfig{Encoding: "text", Format: "parsed"}, ReceiveUpdate: true},
            },
        },
        procManager: &ProcessManager{
            processes: map[string]*Process{
                "json-proc": jsonProc,
                "text-proc": textProc,
            },
        },
        encoder: NewJSONEncoder("6.0.0"),
    }

    // Send raw UPDATE message
    peer := PeerInfo{Address: netip.MustParseAddr("192.168.1.2"), LocalAS: 65001, PeerAS: 65002}
    msg := RawMessage{Type: message.TypeUPDATE, RawBytes: validUpdateBytes}

    server.OnMessageReceived(peer, msg)

    // Verify JSON process got JSON
    jsonOut := <-jsonReceived
    require.True(t, strings.HasPrefix(jsonOut, "{"), "JSON process should receive JSON")

    // Verify text process got text
    textOut := <-textReceived
    require.True(t, strings.HasPrefix(textOut, "neighbor"), "Text process should receive text")
}
```

#### TestDecodeUpdate

```go
// TestDecodeUpdate verifies on-demand UPDATE parsing.
//
// VALIDATES: Raw bytes correctly parsed into structured data.
//
// PREVENTS: Parse errors when format=parsed or format=full.
func TestDecodeUpdate(t *testing.T) {
    // Valid UPDATE with 10.0.0.0/8, next-hop 192.168.1.1
    updateBytes := []byte{...}

    parsed, err := DecodeUpdate(updateBytes, nil)
    require.NoError(t, err)
    require.Len(t, parsed.Announced, 1)
    require.Equal(t, "10.0.0.0/8", parsed.Announced[0].Prefix.String())
}
```

### Phase 1+ Tests

### TestProcessContentConfig

```go
func TestProcessContentConfig(t *testing.T) {
    tests := []struct {
        name         string
        config       string
        wantEncoding string
        wantFormat   string
    }{
        {
            name: "json encoding with parsed format",
            config: `process foo { run ./test; content { encoding json; format parsed; } }`,
            wantEncoding: "json",
            wantFormat:   "parsed",
        },
        {
            name: "text encoding with full format",
            config: `process foo { run ./test; content { encoding text; format full; } }`,
            wantEncoding: "text",
            wantFormat:   "full",
        },
        {
            name: "defaults when content omitted",
            config: `process foo { run ./test; }`,
            wantEncoding: "text",
            wantFormat:   "parsed",
        },
    }
    // ...
}
```

### TestNeighborProcessBinding

```go
func TestNeighborProcessBinding(t *testing.T) {
    config := `
        process foo { run ./test; content { encoding json; } }
        neighbor 10.0.0.1 {
            process foo {
                receive { update; withdraw; }
            }
        }
    `
    // Assert: neighbor 10.0.0.1 has process foo bound
    // Assert: foo receives update and withdraw for this neighbor
}
```

### TestMessageTypeFiltering

```go
func TestMessageTypeFiltering(t *testing.T) {
    // Setup: Process subscribed to update only (not notification)
    // Send: UPDATE message → should be forwarded
    // Send: NOTIFICATION message → should NOT be forwarded
}
```

## Migration Transform Details

### ExaBGP `consolidate` Logic

| parsed | packets | consolidate | → ZeBGP format |
|--------|---------|-------------|----------------|
| true | false | - | `parsed` |
| false | true | - | `raw` |
| true | true | true | `full` |
| true | true | false | `full` (we don't support split events) |

### ExaBGP State Events

| ExaBGP | → ZeBGP receive |
|--------|-----------------|
| `neighbor-changes;` | `state;` |
| `negotiated;` | `state;` |
| `fsm;` | `state;` |
| `signal;` | `state;` |

---

**Created:** 2025-12-29
**Updated:** 2025-12-29 (Phase 0 partial: encoder switching fix, PeerInfo lookup, format constants)
