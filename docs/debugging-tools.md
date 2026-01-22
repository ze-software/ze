# ZeBGP Debugging Tools

This document describes the debugging tools available in ZeBGP for troubleshooting config parsing, BGP message encoding, and session issues.

## Quick Reference

| Tool | Purpose | Usage |
|------|---------|-------|
| `ze bgp config-dump` | Inspect parsed config | `ze bgp config-dump [--json] config.conf` |
| `ze-peer --decode` | Decode BGP messages | `ze-peer --decode --sink` |
| `ZEBGP_TRACE` | Pipeline tracing | `ZEBGP_TRACE=all ze bgp server config.conf` |
| Functional test diff | Test failure analysis | Automatic on message mismatch |
| `--server N` / `--client N` | Interactive test debugging | `ze-test bgp encode --server 0` |

---

## 1. Config Dump Command

**Location:** `cmd/ze/bgp/configdump.go`

Parses a config file and displays the interpreted values. Useful for verifying that config parsing works correctly.

### Usage

```bash
# Human-readable output
ze bgp config-dump config.conf

# JSON output (for scripting)
ze bgp config-dump --json config.conf
```

### Example Output

```
Warnings:
  line 5: freeform 'flow' contains nested block 'route match' - data may be lost

router-id: 1.2.3.4
local-as: 65000

peer 10.0.0.1:
  local-as: 65000
  peer-as: 65001
  static-routes:
    - prefix: 192.168.0.0/24
      next-hop: 10.0.0.254
      local-preference: 100
```

### When to Use

- Config not being parsed correctly
- Static routes not appearing
- Verifying attribute values after parsing
- Checking for parser warnings about data loss

---

## 2. BGP Message Decoder

**Location:** `internal/test/peer/decode.go`

Decodes raw BGP messages (hex) into human-readable format. Used automatically by functional tests and available via `--decode` flag.

### Usage

```bash
# In ze-peer, show decoded messages
ze-peer --decode --sink --port 1790
```

### Example Output

```
msg  recv   FFFFFFFF...:0030:02:00000014400101004002060201000000014003040A000001...
             UPDATE (len=48)
               ORIGIN: IGP
               AS_PATH: [1]
               NEXT_HOP: 10.0.0.1
               NLRI: 192.168.0.0/24
```

### Decoded Attributes

| Code | Name | Decoded Format |
|------|------|----------------|
| 1 | ORIGIN | IGP, EGP, INCOMPLETE |
| 2 | AS_PATH | [1 2 3] or [{1 2}] for AS_SET |
| 3 | NEXT_HOP | IP address |
| 4 | MED | Integer |
| 5 | LOCAL_PREF | Integer |
| 8 | COMMUNITIES | high:low format |
| 16 | EXT_COMMUNITIES | Hex (0x...) |

---

## 3. Test Failure Diff

**Location:** `internal/test/peer/peer.go`, `internal/test/peer/decode.go`

When a functional test fails with "message mismatch", the output now includes a decoded diff showing exactly what differed.

### Example Output

```
failed: message mismatch
--- Expected vs Received ---
Expected: UPDATE (len=64)
  ORIGIN: IGP
  AS_PATH: []
  NEXT_HOP: 10.0.0.1
  LOCAL_PREF: 100
  NLRI: 193.0.2.1/32

Received: KEEPALIVE (len=19)

Differences:
  - ORIGIN: IGP (missing)
  - AS_PATH: [] (missing)
  - NEXT_HOP: 10.0.0.1 (missing)
  - LOCAL_PREF: 100 (missing)
  ~ NLRI: expected=193.0.2.1/32, got=
```

### Diff Symbols

- `-` Missing attribute (expected but not received)
- `+` Unexpected attribute (received but not expected)
- `~` Different value (both present but values differ)

---

## 4. Pipeline Tracing (ZEBGP_TRACE)

**Location:** `internal/trace/trace.go`

Environment variable that enables debug logging at key points in the pipeline.

### Usage

```bash
# Enable all tracing
ZEBGP_TRACE=all ze bgp server config.conf

# Enable specific categories
ZEBGP_TRACE=config,routes ze bgp server config.conf
```

### Categories

| Category | What it traces |
|----------|---------------|
| `config` | Config parsing and loading |
| `routes` | Static route handling, UPDATE sending |
| `session` | BGP session connect/establish/close |
| `fsm` | FSM state transitions |
| `all` | All categories |

### Example Output

```
[TRACE 13:53:41.182] config: parsed (input): 0 neighbors
[TRACE 13:53:41.183] config: loaded config with 1 neighbors
[TRACE 13:53:41.183] routes: neighbor 127.0.0.1: 2 static routes configured
[TRACE 13:53:41.184] fsm: neighbor 127.0.0.1: OPENSENT -> OPENCONFIRM
[TRACE 13:53:41.184] fsm: neighbor 127.0.0.1: OPENCONFIRM -> ESTABLISHED
[TRACE 13:53:41.184] session: session established with 127.0.0.1 (local-as=1, peer-as=1)
[TRACE 13:53:41.184] routes: neighbor 127.0.0.1: sending 2 static routes
[TRACE 13:53:41.184] routes: neighbor 127.0.0.1: sent route 193.0.2.1/32 via 10.0.0.1
[TRACE 13:53:41.184] routes: neighbor 127.0.0.1: sent route 10.0.0.0/24 via 192.168.0.1
[TRACE 13:53:41.184] routes: neighbor 127.0.0.1: sent EOR marker
```

### Trace Points

Currently instrumented locations:
- `internal/config/loader.go`: ConfigParsed, ConfigLoaded, NeighborRoutes
- `internal/reactor/peer.go`: FSMTransition, SessionEstablished, SessionClosed, RouteSent

---

## 5. Parser Warnings

**Location:** `internal/config/parser.go`

The parser now collects warnings for potentially problematic patterns, accessible via `Parser.Warnings()`.

### Warning Types

| Pattern | Warning |
|---------|---------|
| Nested block in Freeform | `freeform 'X' contains nested block 'Y' - data may be lost` |

### Viewing Warnings

```bash
# config-dump shows warnings
ze bgp config-dump config.conf
# Output:
# Warnings:
#   line 5: freeform 'flow' contains nested block 'route match' - data may be lost

# Also visible with ZEBGP_TRACE=config
ZEBGP_TRACE=config ze bgp server config.conf
```

---

## Debugging Workflow

### 1. Config Issues

```bash
# Step 1: Check parsed config
ze bgp config-dump config.conf

# Step 2: Look for warnings
ze bgp config-dump config.conf 2>&1 | grep -i warning

# Step 3: Check JSON for exact values
ze bgp config-dump --json config.conf | jq '.Neighbors[0].StaticRoutes'
```

### 2. Route Not Being Sent

```bash
# Step 1: Enable route tracing
ZEBGP_TRACE=routes ze bgp server config.conf

# Expected output when working:
# [TRACE] routes: neighbor X: 2 static routes configured
# [TRACE] routes: neighbor X: sending 2 static routes
# [TRACE] routes: neighbor X: sent route 10.0.0.0/24 via 192.168.0.1
```

### 3. Session Issues

```bash
# Step 1: Enable session + fsm tracing
ZEBGP_TRACE=session,fsm ze bgp server config.conf

# Look for FSM transitions and session establishment
```

### 4. Test Failures

```bash
# Run single test
ze-test bgp encode 0

# Output now includes decoded diff automatically
# Look at "Differences:" section to see what's wrong
```

### 5. Interactive Functional Test Debugging

Run server and client separately to see live output:

```bash
# Terminal 1: Start test server (ze-peer)
ze-test bgp encode --server 0

# Terminal 2: Start test client (zebgp)
ze-test bgp encode --client 0
```

**Behavior:**
- Server waits for client, prints messages received, exits when test completes
- Client connects, sends configured messages, exits when server disconnects

The client uses `ze_bgp_tcp_attempts=1` automatically, so zebgp exits after the session ends instead of reconnecting.

**Use `--port` to avoid conflicts:**
```bash
ze-test bgp encode --server 0 --port 11790
ze-test bgp encode --client 0 --port 11790
```

---

## Adding New Trace Points

To add tracing to new code:

```go
import "codeberg.org/thomas-mangin/ze/internal/trace"

// Use existing helpers
trace.RouteSent(addr, prefix, nextHop)
trace.SessionEstablished(addr, localAS, peerAS)

// Or log directly
trace.Log(trace.Routes, "custom message: %s", value)
```

---

## File Locations

| File | Purpose |
|------|---------|
| `internal/test/peer/decode.go` | BGP message decoder |
| `internal/test/peer/peer.go` | Test peer with decode support |
| `internal/trace/trace.go` | Tracing infrastructure |
| `cmd/ze/bgp/configdump.go` | config-dump command |
| `cmd/ze-peer/main.go` | --decode flag |
| `internal/config/parser.go` | Parser warnings |
| `internal/config/loader.go` | Config trace points |
| `internal/reactor/peer.go` | Session/route trace points |
