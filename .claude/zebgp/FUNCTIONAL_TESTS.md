# ZeBGP Functional Test System

## Overview

The self-check system (`test/cmd/self-check/`) runs functional tests that validate ZeBGP's BGP message output against expected byte sequences.

## Test Types

### 1. Encode Tests (`test/data/encode/`)

Static route tests - routes defined in config, sent at session establishment.

**Files:**
- `*.ci` - Expected messages and config reference
- `*.conf` - ZeBGP configuration

### 2. API Tests (`test/data/api/`)

Dynamic route tests - routes injected via Python scripts using the process API.

**Files:**
- `*.ci` - Expected messages and config reference
- `*.conf` - ZeBGP configuration (includes `process` block)
- `*.run` - Python script that sends API commands

## .ci File Format

The `.ci` file is the **source of truth** for bidirectional testing:

```
option:file:test.conf           # Config file to use
1:cmd:announce route ...        # API command
1:raw:FFFF...:0017:02:...       # Raw bytes produced by command
1:json:{...}                    # JSON representation
```

### Test Modes Using .ci Files

| Mode | Direction | Purpose |
|------|-----------|---------|
| Functional | cmd → raw | ZeBGP sends command, validate raw output |
| Encode | cmd → raw | Verify encoder produces correct bytes |
| Decode | raw → cmd | Verify decoder produces correct command |
| JSON round-trip | raw → json → raw | Verify JSON encoding/decoding |

### Line Prefixes

| Prefix | Meaning |
|--------|---------|
| `option:` | Test configuration |
| `N:cmd:` | API command (source of truth) |
| `N:raw:` | Expected raw bytes from command |
| `N:json:` | Expected JSON from raw bytes |
| `AN:notification:` | Peer A sends notification at step N |

### Circular Validation (Future)

The `.ci` format enables circular checks (to be implemented):
```
cmd → raw → cmd   (encoding/decoding consistency)
raw → json → raw  (JSON serialization consistency)
```

### Raw Message Format

```
MARKER:LENGTH:TYPE:PAYLOAD
```

- MARKER: 16 bytes (all FF)
- LENGTH: 2 bytes (total message length)
- TYPE: 1 byte (1=OPEN, 2=UPDATE, 3=NOTIFICATION, 4=KEEPALIVE)
- PAYLOAD: Variable

## Test Execution Flow

### Encode Tests

```
1. self-check starts zebgp-peer on random port
2. zebgp-peer waits for connection
3. self-check starts zebgp with config
4. zebgp connects, sends OPEN, receives OPEN
5. zebgp sends UPDATE messages (from static routes)
6. zebgp-peer validates messages against .ci expectations
7. zebgp-peer prints "successful" or "failed: message mismatch"
```

### API Tests

```
1. self-check starts zebgp-peer on random port
2. zebgp-peer waits for connection
3. self-check starts zebgp with config (includes process block)
4. zebgp spawns .run script as subprocess
5. .run script sends commands via stdout (API)
6. zebgp processes commands, sends UPDATE messages
7. zebgp-peer validates messages against .ci expectations
8. .run script calls wait_for_shutdown() and exits
9. zebgp-peer prints "successful" or "failed"
```

## Python API Scripts

The `.run` script sends commands that get batched/processed. The `:cmd:` line represents the **final raw bytes**, not the script commands.

Example: Script sends `add A`, `add B`, `withdraw A`, `add A` → batched result is `add A, add B` → `:cmd:` shows these final routes.

### Common Imports

```python
from exabgp_api import send, wait_for_ack, wait_for_shutdown
```

### Supported API Commands

| Command | Description |
|---------|-------------|
| `announce route PREFIX next-hop NH [attrs]` | Announce IPv4/IPv6 route |
| `withdraw route PREFIX` | Withdraw route |
| `announce eor [family]` | Send End-of-RIB |
| `announce flow match ... then ...` | Announce FlowSpec |
| `withdraw flow match ...` | Withdraw FlowSpec |
| `announce vpls rd RD ...` | Announce VPLS |
| `announce l2vpn TYPE ...` | Announce L2VPN/EVPN |
| `commit start [label]` | Begin transaction |
| `commit end [label]` | Flush transaction |

### Unsupported Commands (cause test failures)

| Command | Status |
|---------|--------|
| `announce ipv4 unicast PREFIX ...` | Not implemented |
| `announce ipv6 unicast PREFIX ...` | Not implemented |
| `announce ipv4 mup ...` | Not implemented |
| `neighbor X announce route ...` | Not implemented |

### wait_for_shutdown()

Scripts should call `wait_for_shutdown()` at the end to:
1. Wait for parent process to signal shutdown
2. Allow graceful cleanup
3. Prevent premature exit

## Debugging Tests

### Manual Execution

```bash
# Terminal 1: Start peer
./zebgp-peer --port 1790 --decode test/data/api/test.ci

# Terminal 2: Start zebgp
env exabgp_tcp_port=1790 zebgp_api_socketpath=/tmp/test.sock \
    ./zebgp server test/data/api/test.conf
```

### Common Issues

| Issue | Cause | Fix |
|-------|-------|-----|
| Timeout | Script doesn't exit | Add `wait_for_shutdown()` |
| Message mismatch | Wrong attributes | Check iBGP vs eBGP settings |
| "invalid prefix" | Missing /length | Use `1.2.3.4/32` not `1.2.3.4` |

## Test Status (as of 2025-12-23)

### Passing API Tests (4/12)

- `add-remove` - Basic announce/withdraw
- `announce` - Simple announcement
- `eor` - End-of-RIB
- `fast` - Multiple routes

### Failing API Tests (8/12)

| Test | Issue | Root Cause |
|------|-------|------------|
| announcement | Message mismatch | Complex route batching |
| attributes | Timeout | Unknown command format |
| check | Timeout | Needs `receive update` events |
| ipv4 | Timeout | `announce ipv4 unicast` not supported |
| ipv6 | Timeout | `announce ipv6 unicast` not supported |
| nexthop | Message mismatch | `next-hop self` not supported |
| notification | Timeout | testpeer can't send notifications |
| teardown | Timeout | Needs teardown command support |

---

**Updated:** 2025-12-23
