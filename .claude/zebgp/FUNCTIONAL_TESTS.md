# ZeBGP Functional Test System

## Overview

ZeBGP has two functional test runners:

| Runner | Command | Status |
|--------|---------|--------|
| **New** | `go run ./test/cmd/functional` | Recommended |
| Legacy | `go run ./test/cmd/self-check` | Still works |

The new runner is modeled after ExaBGP's `qa/bin/functional` with state machine lifecycle, concurrent execution, and timing tracking.

---

## Quick Start

```bash
# List available tests
go run ./test/cmd/functional encoding --list
go run ./test/cmd/functional api --list

# Run specific tests by nick
go run ./test/cmd/functional encoding 4 5 6

# Run all tests
go run ./test/cmd/functional encoding --all
go run ./test/cmd/functional api --all

# Makefile targets
make functional           # Run all (encoding + api)
make functional-encoding  # Encoding tests only
make functional-api       # API tests only
```

---

## Test Types

### 1. Encode Tests (`test/data/encode/`)

Static route tests - routes defined in config, sent at session establishment.

**Files:**
- `*.ci` - Expected messages and config reference
- `*.conf` - ZeBGP configuration

### 2. API Tests (`test/data/api/`)

Dynamic route tests - routes injected via scripts using the process API.

**Files:**
- `*.ci` - Expected messages and config reference
- `*.conf` - ZeBGP configuration (includes `process` block)
- `*.run` - Script that sends API commands

---

## CLI Reference

```
Usage: functional <command> [options] [tests...]

Commands:
  encoding    Run encoding tests (static routes)
  api         Run API tests (dynamic routes via .run scripts)

Modes:
  --list, -l          List available tests
  --short-list        List test codes only (space separated)
  --all               Run all tests
  --edit              Open test files in $EDITOR
  --dry               Show commands without running

Options:
  --timeout N         Timeout per test (default: 30s)
  --parallel N        Max concurrent tests (default: 4)
  --verbose, -v       Show output for each test
  --quiet, -q         Minimal output
  --save DIR          Save logs to directory

Debugging:
  --server NICK       Run server only for test
  --client NICK       Run client only for test
  --port N            Base port to use (default: 1790)
```

---

## Nick System

Tests are assigned single-character nicks for quick selection:

```
0-9  → First 10 tests (0, 1, 2, ... 9)
A-Z  → Next 26 tests (A, B, C, ... Z)
a-z  → Next 26 tests (a, b, c, ... z)
```

Total: 62 tests max per category.

**Examples:**
```bash
# Run test with nick "4"
go run ./test/cmd/functional encoding 4

# Run tests 0, A, and B
go run ./test/cmd/functional encoding 0 A B
```

---

## .ci File Format

The `.ci` file is the **source of truth** for bidirectional testing:

```
option:file:test.conf           # Config file to use
option:asn:65000                # Override peer ASN
1:cmd:announce route ...        # API command
1:raw:FFFF...:0017:02:...       # Raw bytes produced by command
1:json:{...}                    # JSON representation
```

### Line Prefixes

| Prefix | Meaning |
|--------|---------|
| `option:file:` | Config file to use |
| `option:asn:` | Override peer ASN |
| `option:bind:` | Bind option (ipv6) |
| `N:cmd:` | API command (source of truth) |
| `N:raw:` | Expected raw bytes from command |
| `N:json:` | Expected JSON from raw bytes |
| `AN:notification:` | Peer A sends notification at step N |

### Raw Message Format

```
MARKER:LENGTH:TYPE:PAYLOAD
```

- MARKER: 16 bytes (all FF)
- LENGTH: 2 bytes (total message length)
- TYPE: 1 byte (1=OPEN, 2=UPDATE, 3=NOTIFICATION, 4=KEEPALIVE)
- PAYLOAD: Variable

---

## Test Execution Flow

### Encode Tests

```
1. Runner builds zebgp + zebgp-peer to temp dir
2. Starts zebgp-peer on unique port with .ci expectations
3. Starts zebgp with config
4. zebgp connects, sends OPEN, receives OPEN
5. zebgp sends UPDATE messages (from static routes)
6. zebgp-peer validates messages against expectations
7. zebgp-peer prints "successful" or error
```

### API Tests

```
1. Same as encode tests, plus:
5. zebgp spawns .run script as subprocess
6. .run script sends commands via API
7. zebgp processes commands, sends UPDATE messages
8. zebgp-peer validates messages
```

---

## Display Output

The runner shows live progress with colored nicks:

```
 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZa
```

| Color | Meaning |
|-------|---------|
| Gray | Skipped |
| Cyan | Running |
| Green | Passed |
| Red | Failed |
| Yellow | Timeout |

---

## Debugging Tests

### Run single test verbosely

```bash
go run ./test/cmd/functional encoding --timeout 60s --verbose 4
```

### Manual execution

```bash
# Terminal 1: Start peer
go run ./test/cmd/zebgp-peer --port 1790 test/data/encode/ebgp.ci

# Terminal 2: Run zebgp
env exabgp_tcp_port=1790 go run ./cmd/zebgp server test/data/encode/ebgp.conf
```

### Decode message bytes

```bash
# Decode UPDATE payload
go run ./cmd/zebgp-decode update 0000001540010100400200400304650165014005040000006400

# Decode full message
go run ./cmd/zebgp-decode raw FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D02...
```

---

## Adding New Tests

### 1. Create config file

```
# test/data/encode/mytest.conf
peer 127.0.0.1 {
    router-id 1.2.3.4;
    local-address 127.0.0.1;
    local-as 1;
    peer-as 1;

    static {
        route 10.0.0.0/24 next-hop 1.2.3.4;
    }
}
```

### 2. Create .ci file

```
# test/data/encode/mytest.ci
option:file:mytest.conf
1:cmd:announce route 10.0.0.0/24 next-hop 1.2.3.4
1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:002D:02:0000001540010100...
```

### 3. Generate expected bytes

Run with ExaBGP first to capture correct bytes, or use `zebgp-decode` to verify.

---

## Architecture

### New Runner (`test/pkg/`)

| File | Purpose |
|------|---------|
| `state.go` | State machine (none→starting→running→success/fail) |
| `record.go` | Test record with metadata |
| `exec.go` | Process wrapper with termination |
| `tests.go` | Test container and selection |
| `timing.go` | Performance cache for ETA |
| `encoding.go` | Encoding test discovery/execution |
| `api.go` | API test discovery/execution |
| `cli.go` | Argument parsing |

### Security

- Path traversal protection on `option:file:` and `.run` scripts
- Process isolation via `Setpgid`
- Context timeouts on all execution
- File permissions: 0600 (files), 0750 (dirs)

---

## Test Status

See `plan/CLAUDE_CONTINUATION.md` for current pass/fail status.

**Summary:** 37/51 passing (14 failing are advanced features like MUP, MVPN, FlowSpec)

---

**Updated:** 2025-12-27
