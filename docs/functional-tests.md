# ZeBGP Functional Test System

## Overview

Functional tests verify ZeBGP's BGP message encoding by comparing actual wire output against expected bytes.

```bash
# Quick start
make functional           # Run all tests
make functional-encoding  # Encoding tests only
make functional-plugin    # Plugin tests only
```

---

## Quick Start

```bash
# List available tests
zebgp-test run encoding --list
zebgp-test run plugin --list

# Run specific tests by nick
zebgp-test run encoding 4 5 6

# Run all tests
zebgp-test run encoding --all
zebgp-test run plugin --all

# Stress test (detect flaky tests)
zebgp-test run encoding --count 10 0 1
```

---

## Test Types

### 1. Encode Tests (`test/data/encode/`)

Static route tests - routes defined in config, sent at session establishment.

**Files:**
- `*.ci` - Expected messages and config reference
- `*.conf` - ZeBGP configuration

### 2. Parse Tests (`test/data/parse/`)

Config parsing tests - verify configurations parse correctly.

**Positive tests** (expect success) in `valid/` subdirectory:
- `valid/*.conf` - Valid config that should parse

**Negative tests** (expect failure) in `invalid/` subdirectory:
- `invalid/*.conf` - Invalid config that should fail
- `invalid/*.expect` - Expected error substring (or `regex:` pattern)

**Example negative test:**
```
# test/data/parse/invalid/route-refresh-no-process.conf
peer 10.0.0.1 {
    router-id 1.2.3.4;
    local-as 65001;
    peer-as 65002;
    capability { route-refresh; }
}
```

```
# test/data/parse/invalid/route-refresh-no-process.expect
route-refresh requires process with send { update; }
```

### 3. API Tests (`test/data/api/`)

Dynamic route tests - routes injected via scripts using the process API.

**Files:**
- `*.ci` - Expected messages and config reference
- `*.conf` - ZeBGP configuration (includes `process` block)
- `*.run` - Script that sends API commands

### 4. Decode Tests (`test/decode/`)

BGP message decoding tests - verify wire bytes decode to expected JSON.

**Files:**
- `*.ci` - Single file with hex payload, command, and expected JSON

**Format:**
```
stdin=payload:hex=<hex-encoded-bgp-message>
cmd=foreground:seq=1:exec=zebgp-test decode --family <family> -:stdin=payload
expect=json:json=<expected-json>
```

**Example:**
```
# IPv4 unicast decoding test
stdin=payload:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003C020000001C4001010040020040030465016501...
cmd=foreground:seq=1:exec=zebgp-test decode --family ipv4/unicast -:stdin=payload
expect=json:json={ "type": "update", "neighbor": { ... }, "announce": { ... } }
```

**JSON Validation:**
- Parsed and compared field-by-field (key order independent)
- Volatile fields ignored: `exabgp`, `zebgp`, `time`, `host`, `pid`, `ppid`, `counter`
- Neighbor normalization: `peer` ↔ `neighbor` equivalence, `direction` ignored

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

Options:
  -t, --timeout N     Timeout per test (default: 15s)
  -p, --parallel N    Max concurrent tests (0 = all, default: 0)
  -v, --verbose       Show output for each test
  -q, --quiet         Minimal output
  -s, --save DIR      Save logs to directory
  --port N            Base port to use (default: 1790)
  -c, --count N       Run each test N times (stress/benchmark mode)

Debugging:
  --server NICK       Run server only for test
  --client NICK       Run client only for test
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
zebgp-test run encoding 4

# Run tests 0, A, and B
zebgp-test run encoding 0 A B
```

---

## .ci File Format

The `.ci` file is the **source of truth** for bidirectional testing. Full format documentation: [`docs/architecture/testing/ci-format.md`](architecture/testing/ci-format.md)

```
# Tmpfs: embed config inline
tmpfs=test.conf:terminator=EOF_CONF
peer 127.0.0.1 { ... }
EOF_CONF

# Options
option=file:path=test.conf
option=asn:value=65000

# Commands and expectations
cmd=api:conn=1:seq=1:text=update text nhop set 10.0.1.1 nlri ipv4/unicast add 10.0.0.0/24
expect=bgp:conn=1:seq=1:hex=FFFF...
expect=json:conn=1:seq=1:json={...}
```

### Line Types

| Action | Example | Description |
|--------|---------|-------------|
| `tmpfs=` | `tmpfs=file.conf:terminator=EOF` | Embed file content inline |
| `option=` | `option=file:path=test.conf` | Test configuration |
| `cmd=` | `cmd=api:conn=1:seq=1:text=...` | API command |
| `expect=bgp:` | `expect=bgp:conn=1:seq=1:hex=...` | Expected wire bytes |
| `expect=json:` | `expect=json:conn=1:seq=1:json=...` | Expected JSON |
| `expect=stderr:` | `expect=stderr:pattern=...` | Regex pattern in stderr |
| `expect=syslog:` | `expect=syslog:pattern=...` | Regex pattern in syslog |
| `action=notification:` | `action=notification:conn=1:seq=1:text=...` | Send NOTIFICATION |

### Tmpfs (Virtual File System)

Tmpfs allows embedding config files directly in `.ci` files:

```
tmpfs=peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533;
    peer-as 65533;
}
EOF_CONF

option=file:path=peer.conf
```

At runtime, Tmpfs files are written to a temp directory. This enables self-contained tests without separate `.conf` files.

### Logging Tests

Tests can verify logging behavior using `option:env:`, `expect:stderr:`, `reject:stderr:`, and `expect:syslog:`.

**Example: Verify server subsystem logs to stderr**
```
option:file:mytest.conf
option:env:zebgp.log.server=debug

1:raw:FFFF...
expect:stderr:subsystem=server
```

**Example: Verify DEBUG messages are filtered at INFO level**
```
option:file:mytest.conf
option:env:zebgp.log.server=info

1:raw:FFFF...
reject:stderr:level=DEBUG
```

**Example: Verify syslog backend**
```
option:file:mytest.conf
option:env:zebgp.log.server=debug

1:raw:FFFF...
expect:syslog:subsystem=server
```

When `expect:syslog:` is present, the test runner automatically:
1. Starts a test-syslog UDP server on a dynamic port
2. Sets `zebgp.log.backend=syslog` and `zebgp.log.destination=127.0.0.1:<port>`
3. Validates patterns against captured syslog messages after test

#### Syslog Testing Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                      TEST RUNNER (runner.go)                      │
│                                                                   │
│  1. Parse .ci file                                                │
│     └── Found: expect:syslog:subsystem=server                     │
│                                                                   │
│  2. Start testsyslog server (UDP, dynamic port)                   │
│     └── syslog.New(0).Start(ctx) → port 54321                 │
│                                                                   │
│  3. Auto-inject env vars for zebgp:                               │
│     └── zebgp.log.backend=syslog                                  │
│     └── zebgp.log.destination=127.0.0.1:54321                     │
│     └── zebgp.log.server=debug  (from option:env:)                │
│                                                                   │
│  4. Start zebgp with config                                       │
└──────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│                           ZEBGP                                   │
│                                                                   │
│  slogutil.Logger("server") reads env vars:                        │
│    - zebgp.log.server=debug → enabled at DEBUG                    │
│    - zebgp.log.backend=syslog → use syslog handler                │
│    - zebgp.log.destination=127.0.0.1:54321 → UDP target           │
│                                                                   │
│  logger.Debug("msg", "subsystem", "server", ...)                  │
│         │                                                         │
│         ▼                                                         │
│  slog.TextHandler → syslog.Writer → UDP packet                    │
└──────────────────────────────────────────────────────────────────┘
                                │
                      UDP: "<14>... subsystem=server ..."
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│                      TESTSYSLOG SERVER                            │
│                                                                   │
│  Receives: "<14>Jan 19 ... zebgp: level=DEBUG subsystem=server"   │
│  Stores in: srv.messages[]                                        │
└──────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│                   VALIDATION (after test)                         │
│                                                                   │
│  validateLogging() checks each expect:syslog: pattern:            │
│    if !syslogSrv.Match("subsystem=server"):                       │
│        return error("pattern not found")                          │
│                                                                   │
│  Match() does regex search across all captured messages           │
└──────────────────────────────────────────────────────────────────┘
```

**Key components:**

| Component | Location | Purpose |
|-----------|----------|---------|
| `syslog.Server` | `internal/test/syslog/` | UDP server capturing syslog messages |
| `option:env:` | `.ci` file | Sets env vars (e.g., `zebgp.log.server=debug`) |
| `expect:syslog:` | `.ci` file | Regex pattern to match in captured messages |
| Auto-injection | `runner.go` | Adds `backend=syslog` + `destination=host:port` |
| `validateLogging()` | `runner.go` | Checks patterns after test completes |

**Message format:** Syslog messages use Go's `slog.TextHandler` format with syslog framing:
```
<priority>timestamp hostname zebgp: level=DEBUG subsystem=server msg="..." key=value
```

Patterns should match the key=value pairs (e.g., `subsystem=server`, `level=DEBUG`).

### Raw Message Format

```
MARKER:LENGTH:TYPE:PAYLOAD
```

- MARKER: 16 bytes (all FF)
- LENGTH: 2 bytes (total message length)
- TYPE: 1 byte (1=OPEN, 2=UPDATE, 3=NOTIFICATION, 4=KEEPALIVE)
- PAYLOAD: Variable

### JSON Validation Format

The `N:json:` lines use ZeBGP plugin format (not ExaBGP envelope format):

**Unicast:**
```json
{"meta":{"version":"1.0.0","format":"zebgp"},"message":{"type":"update"},"origin":"igp","ipv4/unicast":[{"next-hop":"10.0.1.254","action":"add","nlri":["10.0.0.0/24"]}]}
```

**FlowSpec:**
```json
{"meta":{"version":"1.0.0","format":"zebgp"},"message":{"type":"update"},"origin":"igp","ipv4/flowspec":[{"action":"add","nlri":{"next-hop":"1.2.3.4","destination":["192.168.0.1/32"],"string":"flow destination 192.168.0.1/32"}}]}
```

**Supported families:** `ipv4/unicast`, `ipv6/unicast`, `ipv4/flowspec`, `ipv6/flowspec`

**Key differences from ExaBGP envelope:**
- Flat structure (no `neighbor.message.update` nesting)
- `meta.format` = "zebgp" (not `exabgp` version)
- Family arrays at top level with `action` field
- FlowSpec: `nlri` is object with components; unicast: `nlri` is string array

**Context fields ignored:** `peer`, `direction` (test-environment dependent)

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

ExaBGP-style progress display:

```
timeout [5/30] running 4 passed 12 failed 2 [S, V]
```

| Field | Meaning |
|-------|---------|
| `timeout [N/M]` | Longest running test: N seconds elapsed, M timeout |
| `running N` | N tests currently executing |
| `passed N` | N tests passed |
| `failed N [IDs]` | N tests failed, with nicks |

### Stress Test Mode

Use `--count N` (`-c N`) to run tests multiple times for benchmarking or detecting flaky tests:

```bash
# Run test C 10 times with timing
zebgp-test run plugin -c 10 C

# Run all encoding tests 5 times
zebgp-test run encoding -c 5 -a
```

**Per-iteration timing** is shown during execution:
```
==> Iteration 1/10
==> Iteration 1: 5.2s

==> Iteration 2/10
==> Iteration 2: 4.8s
```

**Summary** shows per-test stats and overall timing:
```
STRESS TEST SUMMARY
═══════════════════════════════════════════════════════════════════════════════
Iterations: 10

Nick     Pass   Fail    T/O        Min        Avg        Max    Rate
---------------------------------------------------------------------------
0          10      0      0      108ms      332ms      764ms  100.0%
1           8      2      0      115ms      400ms      900ms   80.0%
═══════════════════════════════════════════════════════════════════════════════
Iteration timing: min=4.8s avg=5.1s max=5.7s total=51.2s
Total: 20 iterations, 18 passed, 2 failed, 0 timed out (90.0% pass rate)
```

**Key metrics:**
- Per-test min/avg/max duration
- Per-test pass rate (color-coded: green=100%, yellow≥50%, red<50%)
- Iteration timing: min/avg/max/total wall-clock time

---

## Debugging Tests

### Run single test verbosely

```bash
zebgp-test run encoding --timeout 60s --verbose 4
```

### Manual execution

```bash
# Terminal 1: Start peer
zebgp-peer --port 1790 test/data/encode/ebgp.ci

# Terminal 2: Run zebgp
env zebgp_tcp_port=1790 zebgp server test/data/encode/ebgp.conf
```

### Decode message bytes

```bash
# Decode UPDATE payload
zebgp decode update 0000001540010100400200400304650165014005040000006400

# Decode full message
zebgp decode raw FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D02...
```

---

## Adding New Tests

### Option 1: Tmpfs (Recommended)

Single self-contained `.ci` file with embedded config:

```
# test/data/encode/mytest.ci
tmpfs=mytest.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    router-id 1.2.3.4;
    local-address 127.0.0.1;
    local-as 1;
    peer-as 1;

    family {
        ipv4/unicast;
    }
    announce {
        ipv4 {
            unicast 10.0.0.0/24 next-hop 1.2.3.4;
        }
    }
}
EOF_CONF

option=file:path=mytest.conf
cmd=api:conn=1:seq=1:text=update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D020000001540010100...
```

### Option 2: Separate Files

```
# test/data/encode/mytest.conf
peer 127.0.0.1 {
    router-id 1.2.3.4;
    local-address 127.0.0.1;
    local-as 1;
    peer-as 1;

    family {
        ipv4/unicast;
    }
    announce {
        ipv4 {
            unicast 10.0.0.0/24 next-hop 1.2.3.4;
        }
    }
}
```

```
# test/data/encode/mytest.ci
option=file:path=mytest.conf
cmd=api:conn=1:seq=1:text=update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D020000001540010100...
```

### Generate expected bytes

Run with ExaBGP first to capture correct bytes, or use `zebgp decode` to verify.

### Adding Negative Parsing Tests

To test that invalid configs are rejected with specific errors:

**1. Create invalid config:**
```
# test/data/parse/invalid/my-error.conf
peer 10.0.0.1 {
    # ... invalid configuration ...
}
```

**2. Create .expect file with expected error:**

**Substring match (default):**
```
# test/data/parse/invalid/my-error.expect
specific error message substring
```

**Regex match (for variable parts like IPs, line numbers):**
```
# test/data/parse/invalid/my-error.expect
regex:peer \d+\.\d+\.\d+\.\d+: route-refresh requires
```

The test passes if:
- `zebgp validate` exits with non-zero status
- Output contains the expected substring OR matches the regex pattern

---

## Architecture

### Package: `internal/test/runner/`

| File | Purpose |
|------|---------|
| `color.go` | TTY-aware ANSI colors |
| `decode.go` | BGP message decoding for failure reports |
| `display.go` | Live progress display |
| `json.go` | JSON validation: transform envelope → plugin format |
| `limits.go` | ulimit check and auto-raise |
| `ports.go` | Dynamic port range allocation |
| `record.go` | Test record with state machine, Tmpfs file storage |
| `report.go` | AI-friendly failure reports |
| `runner.go` | Test execution engine, Tmpfs runtime support |
| `stress.go` | Iteration stats and timing for -c/--count |
| `tmpfs_test.go` | Tmpfs parsing integration tests |

### Package: `internal/tmpfs/`

| File | Purpose |
|------|---------|
| `tmpfs.go` | Tmpfs parser and writer |
| `limits.go` | Configurable limits from environment |
| `security.go` | Path validation (traversal, escape) |
| `cleanup.go` | Signal handling for temp cleanup |

### Entry Point: `cmd/zebgp-test/`

Subcommand-based CLI with `run` for test execution and `syslog` for syslog server.

### Security

- Path traversal protection on `option:file:` and `.run` scripts
- Process isolation via `Setpgid`
- Context timeouts on all execution
- Dynamic port allocation prevents conflicts

---

**Updated:** 2026-01-20
