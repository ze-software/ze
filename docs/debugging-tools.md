# ZeBGP Debugging Tools

This document describes the debugging tools available in ZeBGP for troubleshooting config parsing, BGP message encoding, and session issues.

## Quick Reference

| Tool | Purpose | Usage |
|------|---------|-------|
| `ze config dump` | Inspect parsed config | `ze config dump [--json] config.conf` |
| `ze-peer --decode` | Decode BGP messages | `ze-peer --decode --sink` |
| `ze.log.*` | Per-subsystem logging | `ze.log.bgp.routes=debug ze bgp server config.conf` |
| Functional test diff | Test failure analysis | Automatic on message mismatch |
| `--server N` / `--client N` | Interactive test debugging | `ze-test bgp encode --server 0` |

---

## 1. Config Dump Command

**Location:** `cmd/ze/config/cmd_dump.go`
<!-- source: cmd/ze/config/cmd_dump.go -- config dump command -->

Parses a config file and displays the interpreted values. Useful for verifying that config parsing works correctly.

### Usage

```bash
# Human-readable output
ze config dump config.conf

# JSON output (for scripting)
ze config dump --json config.conf
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

<!-- source: internal/test/runner/decode.go -- BGP message decoding -->
<!-- source: internal/test/runner/decoding.go -- decoding helpers -->

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

<!-- source: internal/test/peer/peer.go -- test peer with message validation -->
<!-- source: internal/test/runner/diff.go -- test failure diff -->

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

## 4. Per-Subsystem Logging (ze.log.*)

<!-- source: internal/core/slogutil/slogutil.go -- Logger() function and hierarchical subsystem logging -->

Environment variables that enable structured logging at key points in the pipeline.

### Usage

```bash
# Enable all subsystems at debug level
ze.log=debug ze bgp server config.conf

# Enable specific subsystems
ze.log.bgp.config=debug ze.log.bgp.routes=debug ze bgp server config.conf
```

### Subsystems

Names follow `<domain>.<component>` convention. Run `ze env` for the full list.

| Variable | What it logs |
|----------|---------------|
| `ze.log` | Base level for ALL subsystems |
| `ze.log.bgp.config` | Config parsing and loading |
| `ze.log.bgp.routes` | Static route handling, UPDATE sending |
| `ze.log.bgp.reactor.session` | BGP session handling |
| `ze.log.bgp.reactor.peer` | Peer FSM transitions, session events |
| `ze.log.bgp.reactor` | Reactor operations (reload, etc.) |
| `ze.log.plugin.server` | Plugin RPC server |
| `ze.log.plugin.relay` | Plugin stderr relay |

Levels: `disabled`, `debug`, `info`, `warn`, `err` (case-insensitive)

### Example Output

```
time=2024-01-15T13:53:41.182Z level=DEBUG msg="config parsed" subsystem=bgp.config warnings=0
time=2024-01-15T13:53:41.183Z level=DEBUG msg="config loaded" subsystem=bgp.config peers=1
time=2024-01-15T13:53:41.183Z level=DEBUG msg="peer routes configured" subsystem=bgp.config peer=127.0.0.1 routes=2
time=2024-01-15T13:53:41.184Z level=DEBUG msg="FSM transition" subsystem=bgp.reactor.peer peer=127.0.0.1 from=OPENSENT to=OPENCONFIRM
time=2024-01-15T13:53:41.184Z level=INFO msg="session established" subsystem=bgp.reactor.peer peer=127.0.0.1 localAS=1 peerAS=1
time=2024-01-15T13:53:41.184Z level=DEBUG msg="route sent" subsystem=bgp.routes peer=127.0.0.1 prefix=193.0.2.1/32 nextHop=10.0.0.1
```

### Hierarchical Priority

Most specific wins:
1. `ze.log.bgp.reactor.peer=warn` - Peer logging at WARN
2. `ze.log.bgp.reactor=debug` - All reactor.* at DEBUG
3. `ze.log.bgp=info` - All bgp.* at INFO
4. `ze.log=debug` - All subsystems at DEBUG

Shell-compatible: `ze_log_bgp_routes=debug` also works (dot→underscore)

### Logging Locations

Currently instrumented:
- `internal/component/config/loader.go`: Config parsing, peer routes (configLogger)
- `internal/component/bgp/reactor/peer.go`: FSM, session, route operations (peerLogger, routesLogger)
- `internal/component/bgp/reactor/session.go`: RFC 7606 handling (sessionLogger)
- `internal/component/bgp/reactor/reactor.go`: Reload, route operations (reactorLogger, routesLogger)

<!-- source: internal/component/bgp/reactor/peer.go -- peerLogger, routesLogger -->
<!-- source: internal/component/bgp/reactor/session.go -- sessionLogger -->
<!-- source: internal/component/bgp/reactor/reactor.go -- reactorLogger -->

---

## 5. Parser Warnings

<!-- source: internal/component/config/parser.go -- Parser.Warnings() -->

The parser now collects warnings for potentially problematic patterns, accessible via `Parser.Warnings()`.

### Warning Types

| Pattern | Warning |
|---------|---------|
| Nested block in Freeform | `freeform 'X' contains nested block 'Y' - data may be lost` |

### Viewing Warnings

```bash
# config-dump shows warnings
ze config dump config.conf
# Output:
# Warnings:
#   line 5: freeform 'flow' contains nested block 'route match' - data may be lost

# Also visible with debug logging
ze.log.config=debug ze bgp server config.conf
```

---

## Debugging Workflow

### 1. Config Issues

```bash
# Step 1: Check parsed config
ze config dump config.conf

# Step 2: Look for warnings
ze config dump config.conf 2>&1 | grep -i warning

# Step 3: Check JSON for exact values
ze config dump --json config.conf | jq '.Neighbors[0].StaticRoutes'
```

### 2. Route Not Being Sent

```bash
# Step 1: Enable route logging
ze.log.bgp.routes=debug ze.log.bgp.config=debug ze bgp server config.conf

# Expected output when working:
# level=DEBUG msg="peer routes configured" subsystem=bgp.config peer=X routes=2
# level=DEBUG msg="route sent" subsystem=bgp.routes peer=X prefix=10.0.0.0/24 nextHop=192.168.0.1
```

### 3. Session Issues

```bash
# Step 1: Enable session + peer logging
ze.log.bgp.reactor.peer=debug ze.log.bgp.reactor.session=debug ze bgp server config.conf

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

## Adding New Logging

To add logging to new code:

```go
import "codeberg.org/thomas-mangin/ze/internal/core/slogutil"

// Create a subsystem logger (once per package)
var myLogger = slogutil.Logger("my.subsystem")

// Log with structured key-value pairs
myLogger.Debug("operation completed", "peer", addr, "count", n)
myLogger.Info("session established", "peer", addr, "localAS", localAS)
myLogger.Warn("unexpected state", "state", state, "expected", expected)
```

Subsystem naming convention: `ze.log.` + simplified package path (e.g., `bgp.reactor.peer`).

---

## File Locations

| File | Purpose |
|------|---------|
| `internal/test/runner/decode.go` | BGP message decoder |
| `internal/test/peer/peer.go` | Test peer with decode support |
| `internal/core/slogutil/slogutil.go` | Logging infrastructure |
| `cmd/ze/config/cmd_dump.go` | config dump command |
| `cmd/ze-test/peer.go` | --decode flag |
| `internal/component/config/parser.go` | Parser warnings |
| `internal/component/bgp/config/loader.go` | Config logging (configLogger) |
| `internal/component/bgp/reactor/peer.go` | Peer/route logging (peerLogger, routesLogger) |
| `internal/component/bgp/reactor/session.go` | Session logging (sessionLogger) |
| `internal/component/bgp/reactor/reactor.go` | Reactor logging (reactorLogger, routesLogger) |

<!-- source: internal/core/slogutil/slogutil.go -- logging infrastructure -->
<!-- source: internal/test/peer/peer.go -- test peer -->
<!-- source: internal/test/runner/decode.go -- BGP message decoder -->
<!-- source: cmd/ze/config/cmd_dump.go -- config-dump command -->
<!-- source: cmd/ze-test/peer.go -- ze-test peer subcommand -->
