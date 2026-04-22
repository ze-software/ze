# ZeBGP Functional Test System

## Overview

Functional tests verify ZeBGP's BGP message encoding by comparing actual wire output against expected bytes.

```bash
# Quick start
make ze-functional-test   # Run all tests
make ze-encode-test       # Encoding tests only
make ze-plugin-test       # Plugin tests only
make ze-reload-test       # Reload tests only
```

---

## Quick Start

```bash
# List available tests
ze-test bgp encode --list
ze-test bgp plugin --list

# Run specific tests by nick
ze-test bgp encode 4 5 6

# Run all tests
ze-test bgp encode --all
ze-test bgp plugin --all

# Stress test (detect flaky tests)
ze-test bgp encode --count 10 0 1
```

---

## Test Types

### 1. Encode Tests (`test/encode/`)

Static route tests - routes defined in config, sent at session establishment.

**Files:**
- `*.ci` - Expected messages and config reference
- `*.conf` - ZeBGP configuration

### 2. Parse Tests (`test/parse/`)

Config parsing tests - verify configurations parse correctly.

**Files:** All parse tests use `.ci` format with embedded config.

**Positive tests** (expect success):
```
# test/parse/simple-v4.ci
stdin=config:terminator=EOF_CONF
bgp {
    peer test-peer {
        remote {
            ip 127.0.0.1;
            as 65533;
        }
        router-id 10.0.0.2;
        local-as 65533;
    }
}
EOF_CONF

cmd=foreground:seq=1:exec=ze bgp validate -:stdin=config
expect=exit:code=0
```

**Negative tests** (expect failure):
```
# test/parse/route-refresh-no-process.ci
stdin=config:terminator=EOF_CONF
bgp {
    peer test-peer {
        remote {
            ip 10.0.0.1;
            as 65002;
        }
        router-id 1.2.3.4;
        local-as 65001;
        capability { route-refresh; }
    }
}
EOF_CONF

cmd=foreground:seq=1:exec=ze bgp validate -:stdin=config
expect=exit:code=1
expect=stderr:contains=route-refresh requires process with send { update; }
```

### 3. API Tests (`test/api/`)

Dynamic route tests - routes injected via scripts using the process API.

**Files:**
- `*.ci` - Expected messages and config reference
- `*.conf` - ZeBGP configuration (includes `process` block)
- `*.run` - Script that sends API commands

### 3b. MCP Tests (`test/plugin/mcp-*.ci`, `test/plugin/elicitation-*.ci`)

End-to-end scenarios for the MCP transport. The runner launches a ze
daemon with `--mcp <port>` in the background and `ze-test mcp` in the
foreground; assertions come from `expect=exit:code=...` and
`expect=stdout|stderr:contains=...`.

| File | What it covers |
|------|----------------|
| `mcp-announce.ci` | `ze_execute` dispatches a BGP UPDATE via the MCP endpoint, `ze-peer` verifies the wire bytes |
| `elicitation-accept.ci` | Client declares `capabilities.elicitation={}`, queues an accept reply, `ze_execute` with empty command triggers `elicitation/create` over SSE, accepted command is dispatched |
| `elicitation-decline.ci` | Same setup, queued decline surfaces as a tool error containing "declined" |
| `elicitation-no-capability.ci` | Client does NOT declare the capability; the server fails fast with "missing required argument" instead of hanging on an elicit |

The `ze-test mcp` client understands these extra stdin directives for
elicitation scenarios: `elicit-accept <json>`, `elicit-decline`,
`elicit-cancel`. Each queues one reply; the client auto-cancels when an
elicit frame arrives with nothing queued.

### Forward-Path Claims

Claims about forwarding, per-destination egress filters, or wire-visible
re-advertisement must drive a real forward path. In practice that means the
`.ci` file must load a plugin that calls `ForwardUpdate()` or
`ForwardUpdatesDirect()` (for example `bgp-rs`) and then assert on the
destination peer with `expect=bgp:...` or another deterministic wire-visible
signal.

A two-peer setup by itself is not evidence. If no forwarding-capable plugin is
loaded, a destination `ze-peer` may establish while no egress filter ever runs.
Those files must be marked `partial` or `blocked` instead of claiming full wire
coverage.

There is also a known single-`ze-peer` multi-IP timing limitation for some
multi-destination scenarios. When a test needs one `ze-peer` process to keep
multiple local-IP sessions alive long enough for deterministic wire assertions
and that timing remains flaky, the file should name that exact blocker and stay
`partial`/`blocked` until the fixture support exists.

### Test-Only Internal Plugins (`internal/test/plugins/`)

Some `.ci` tests need a synthetic Go-side producer to drive features that
have no real producer yet (e.g., bgp-redistribute waiting for L2TP route
events). These plugins live under `internal/test/plugins/<name>/` and
register at init() so they appear in the production daemon's plugin
registry. They do nothing until invoked via `ze.fakeredist`-style config or
via a `.ci` test's dispatch-command.

First occupant: `internal/test/plugins/fakeredist/`. Pattern:

| File | Role |
|------|------|
| `fakeredist.go` | Package state, command parser, batch builder/emitter |
| `register.go` | Plugin registration + `OnExecuteCommand` dispatcher |
| `fakeredist_test.go` | Unit tests for the command surface |

The aggregator at `internal/test/plugins/all/all.go` blank-imports every
test-only internal plugin. Production also imports the individual packages
from `internal/component/plugin/all/all.go` because `.ci` tests run
production `bin/ze`; the runtime cost is one registry entry per test plugin
and zero overhead until invoked.

<!-- source: internal/test/plugins/fakeredist/register.go -- pattern reference -->
<!-- source: internal/component/plugin/all/all.go -- production aggregator import -->

### 4. Reload Tests (`test/reload/`)

Config reload tests - verify SIGHUP-triggered reload behavior end-to-end.

**Files:** All reload tests use `.ci` format with embedded config and tmpfs alternate configs.

**How they work:**
1. Daemon starts with initial config, establishes BGP session
2. Test peer verifies initial messages
3. `action=rewrite` replaces config file with alternate version in tmpfs
4. `action=sighup` sends SIGHUP to daemon PID (read from `daemon.pid` in tmpfs)
5. Daemon reloads config — peers restart if settings changed
6. Test peer verifies reconnection and new messages

**Example:**
```
# Initial config establishes session with one route
stdin=ze-bgp:terminator=EOF_CONF
bgp { peer loopback { remote { ip 127.0.0.1; } ... nlri { ipv4/unicast add 192.168.1.0/24; } } }
EOF_CONF

# Alternate config with two routes
tmpfs=config2.conf:terminator=EOF_CONF2
bgp { peer loopback { remote { ip 127.0.0.1; } ... nlri { ipv4/unicast add 192.168.1.0/24; ipv4/unicast add 10.0.0.0/24; } } }
EOF_CONF2

option=tcp_connections:value=2
expect=bgp:conn=1:seq=1:hex=...   # Initial route
action=rewrite:conn=1:seq=2:source=config2.conf:dest=ze-bgp.conf
action=sighup:conn=1:seq=2
expect=bgp:conn=2:seq=1:hex=...   # Both routes after reload
```

### 5. VPP Tests (`test/vpp/`)

Functional tests that exercise `fib-vpp` end-to-end against a Python
GoVPP-API stub. The stub replaces the real VPP process in CI: no DPDK,
no vfio, no root. Each test runs against a fresh per-test Unix socket.
<!-- source: test/scripts/vpp_stub.py -- stdlib-only GoVPP socket-client stub -->

**Runner:** `ze-test vpp [flags] [tests...]`

| Flag | Purpose |
|------|---------|
| `-l`, `--list` | List available tests (discovered from `test/vpp/*.ci`) |
| `-a`, `--all` | Run every test under `test/vpp/` |
| `-t`, `--timeout` | Per-test timeout (default 30s) |
| `-p`, `--parallel` | Concurrent tests (default 1 -- each test binds its own Unix socket) |
| `-v`, `--verbose` | Show per-test output |
| `-s`, `--save DIR` | Save client/peer logs under `DIR/<nick>-<name>/` for offline inspection |

**Dependencies:**
- Tests use the `vpp.external` YANG leaf (default `false`) so ze connects
  via GoVPP without execing the VPP binary. See `docs/guide/vpp.md`.
- The stub runs as `python3 -m vpp_stub --socket <path> --log <path>`;
  PYTHONPATH is set by the runner to `test/scripts/`.

**Example:**
```
bin/ze-test vpp -l
bin/ze-test vpp 001-boot
bin/ze-test vpp -a
```
<!-- source: cmd/ze-test/vpp.go -- vppCmd wires EncodingTests to test/vpp/ -->

### 6. Backend Apply-Path Unit Tests (Go `_test.go`)

Backends that talk to an out-of-process IPC surface (VPP GoVPP, a netconf server,
any future RPC-backed kernel) expose a narrow unexported operation interface
(`vppOps`, `netconfOps`, ...) with only the methods the Apply path needs. The
production adapter wraps the live transport; unit tests substitute a scripted
fake that records calls and can fail on the Nth request.

`internal/plugins/traffic/vpp/` is the reference implementation:

- `ops.go` defines `vppOps` (4 methods: `dumpInterfaces`, `policerAddDel`,
  `policerDel`, `policerOutput`).
- `backend_linux.go` exposes an internal `applyWithOps(ops, desired)` entry
  point and a `govppOps{ch api.Channel}` production adapter; `Apply`
  constructs a `govppOps` around the channel it opened and calls
  `applyWithOps`.
- `apply_test.go` defines `fakeOps` (records a `[]string` of labeled calls,
  supports `failOnNthAddDel` for deterministic partial-failure tests) and
  covers the create/update/undo/reconcile/orphan branches without a running
  VPP daemon.

Use this pattern when adding a new backend whose Apply path would otherwise
require full-stack integration tests to cover every undo / reconcile branch.

<!-- source: internal/plugins/traffic/vpp/ops.go -- vppOps interface -->
<!-- source: internal/plugins/traffic/vpp/backend_linux.go -- applyWithOps, govppOps adapter -->
<!-- source: internal/plugins/traffic/vpp/apply_test.go -- fakeOps + 7 tests -->

### 7. Decode Tests (`test/decode/`)

BGP message decoding tests - verify wire bytes decode to expected JSON.

**Files:**
- `*.ci` - Single file with hex payload, command, and expected JSON

**Format:**
```
stdin=payload:hex=<hex-encoded-bgp-message>
cmd=foreground:seq=1:exec=ze-test decode --family <family> -:stdin=payload
expect=json:json=<expected-json>
```

**Example:**
```
# IPv4 unicast decoding test
stdin=payload:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003C020000001C4001010040020040030465016501...
cmd=foreground:seq=1:exec=ze-test decode --family ipv4/unicast -:stdin=payload
expect=json:json={ "type": "update", "neighbor": { ... }, "announce": { ... } }
```

**JSON Validation:**
- Parsed and compared field-by-field (key order independent)
- Volatile fields ignored: `exabgp`, `ze-bgp`, `time`, `host`, `pid`, `ppid`, `counter`
- Neighbor normalization: `peer` ↔ `neighbor` equivalence, `direction` ignored

---

## CLI Reference

```
Usage: functional <command> [options] [tests...]

Commands:
  encode      Run encoding tests (static routes)
  plugin      Run plugin tests (dynamic routes via .run scripts)

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
  --port N            Port to use (0 or omit for OS-assigned dynamic port)
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
ze-test bgp encode 4

# Run tests 0, A, and B
ze-test bgp encode 0 A B
```

---

## .ci File Format

The `.ci` file is the **source of truth** for bidirectional testing. Full format documentation: [`docs/architecture/testing/ci-format.md`](architecture/testing/ci-format.md)

```
# Tmpfs: embed config inline
tmpfs=test.conf:terminator=EOF_CONF
peer test-peer { remote { ip 127.0.0.1; } ... }
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
| `expect=stdout:` | `expect=stdout:contains=text` | Substring match in stdout |
| `expect=stderr:` | `expect=stderr:pattern=...` or `contains=...` | Regex or substring match in stderr |
| `expect=syslog:` | `expect=syslog:pattern=...` | Regex pattern in syslog |
| `reject=stderr:` | `reject=stderr:pattern=...` | Fail if stderr matches regex |
| `reject=syslog:` | `reject=syslog:pattern=...` | Fail if syslog matches regex |
| `action=notification:` | `action=notification:conn=1:seq=1:text=...` | Send NOTIFICATION |
| `action=rewrite:` | `action=rewrite:conn=1:seq=2:source=config2.conf:dest=ze-bgp.conf` | Rewrite config file |
| `action=sighup:` | `action=sighup:conn=1:seq=2` | Send SIGHUP to daemon |
| `action=sigterm:` | `action=sigterm:conn=1:seq=2` | Send SIGTERM to daemon |

### Tmpfs (Virtual File System)

Tmpfs allows embedding config files directly in `.ci` files:

```
tmpfs=peer.conf:terminator=EOF_CONF
peer test-peer {
    remote {
        ip 127.0.0.1;
        as 65533;
    }
    local-as 65533;
}
EOF_CONF

option=file:path=peer.conf
```

At runtime, Tmpfs files are written to a temp directory. This enables self-contained tests without separate `.conf` files.

### Directive Placement

Test directives belong to one of two scopes:

| Scope | Consumer | Placement |
|-------|----------|-----------|
| Test runner | The `ze-test` process itself (seeds `proc.Env`, drives orchestration) | File level, outside any `stdin=...` block |
| `ze-peer` stdin | The `ze-peer` subprocess reading its stdin at runtime | Inside the `stdin=peer:terminator=X` block |

Only `expect=bgp:...`, `expect=json:...`, `expect=exit:...`, `action=...`, `option=timeout:...`, `option=open:...`, `option=update:...`, and `option=tcp_connections:...` are valid inside `stdin=peer:` blocks. The `option=timeout`, `option=open`, `option=update`, and `option=tcp_connections` forms are consumed by `ze-peer` from its stdin and must stay in-block so the subprocess receives them.

**`option=env:var=K:value=V` is consumed by the test runner (it appends to `proc.Env` when spawning `ze`/`ze-peer`/helper processes) and therefore MUST live at file level, outside any `stdin=peer:` block.** Placing it inside the block used to be silently dropped — the directive would be handed to `ze-peer`, which ignores it, and the target process would never see the variable. The parser now rejects this at `bin/ze-test <suite> -list` time with an error naming the exact directive and pointing at this section.

<!-- source: internal/test/runner/record_parse.go — parseAndAdd peer-block loop -->

**Correct placement:**

```
# option=env belongs ABOVE the stdin=peer block.
option=env:var=ze.log.bgp.server:value=debug

stdin=peer:terminator=EOF_PEER
option=timeout:value=15s
option=open:value=inspect-open-message
expect=bgp:conn=1:seq=1:hex=FFFF...
EOF_PEER
```

**Rejected at parse time:**

```
stdin=peer:terminator=EOF_PEER
option=timeout:value=15s
option=env:var=ze.log.bgp.server:value=debug   # <-- PARSE ERROR
expect=bgp:conn=1:seq=1:hex=FFFF...
EOF_PEER
```

### Logging Tests

Tests can verify logging behavior using `option=env:`, `expect=stderr:`, `reject=stderr:`, and `expect=syslog:`.

**Example: Verify server subsystem logs to stderr**
```
option=file:path=mytest.conf
option=env:var=ze.bgp.log.server:value=debug

expect=bgp:conn=1:seq=1:hex=FFFF...
expect=stderr:pattern=subsystem=server
```

**Example: Verify DEBUG messages are filtered at INFO level**
```
option=file:path=mytest.conf
option=env:var=ze.bgp.log.server:value=info

expect=bgp:conn=1:seq=1:hex=FFFF...
reject=stderr:pattern=level=DEBUG
```

**Example: Verify syslog backend**
```
option=file:path=mytest.conf
option=env:var=ze.bgp.log.server:value=debug

expect=bgp:conn=1:seq=1:hex=FFFF...
expect=syslog:pattern=subsystem=server
```

When `expect=syslog:` is present, the test runner automatically:
1. Starts a test-syslog UDP server on a dynamic port
2. Sets `ze.log.backend=syslog` and `ze.log.destination=127.0.0.1:<port>`
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
│  3. Auto-inject env vars for ze-bgp:                               │
│     └── ze.bgp.log.backend=syslog                                  │
│     └── ze.bgp.log.destination=127.0.0.1:54321                     │
│     └── ze.bgp.log.server=debug  (from option:env:)                │
│                                                                   │
│  4. Start ze bgp with config                                       │
└──────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│                           ZEBGP                                   │
│                                                                   │
│  slogutil.Logger("server") reads env vars:                        │
│    - ze.bgp.log.server=debug → enabled at DEBUG                    │
│    - ze.bgp.log.backend=syslog → use syslog handler                │
│    - ze.bgp.log.destination=127.0.0.1:54321 → UDP target           │
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
│  Receives: "<14>Jan 19 ... ze-bgp: level=DEBUG subsystem=server"   │
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
| `option:env:` | `.ci` file | Sets env vars (e.g., `ze.bgp.log.server=debug`) |
| `expect:syslog:` | `.ci` file | Regex pattern to match in captured messages |
| Auto-injection | `runner.go` | Adds `backend=syslog` + `destination=host:port` |
| `validateLogging()` | `runner.go` | Checks patterns after test completes |

<!-- source: internal/test/syslog/testsyslog.go -- syslog UDP server -->
<!-- source: internal/test/runner/runner.go -- auto-injection and validateLogging -->

**Message format:** Syslog messages use Go's `slog.TextHandler` format with syslog framing:
```
<priority>timestamp hostname ze-bgp: level=DEBUG subsystem=server msg="..." key=value
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
{"meta":{"version":"1.0.0","format":"ze-bgp"},"message":{"type":"update"},"origin":"igp","ipv4/unicast":[{"next-hop":"10.0.1.254","action":"add","nlri":["10.0.0.0/24"]}]}
```

**FlowSpec:**
```json
{"meta":{"version":"1.0.0","format":"ze-bgp"},"message":{"type":"update"},"origin":"igp","ipv4/flowspec":[{"action":"add","nlri":{"next-hop":"1.2.3.4","destination":["192.168.0.1/32"],"string":"flow destination 192.168.0.1/32"}}]}
```

**Supported families:** `ipv4/unicast`, `ipv6/unicast`, `ipv4/flowspec`, `ipv6/flowspec`

**Key differences from ExaBGP envelope:**
- Flat structure (no `neighbor.message.update` nesting)
- `meta.format` = "ze-bgp" (not `exabgp` version)
- Family arrays at top level with `action` field
- FlowSpec: `nlri` is object with components; unicast: `nlri` is string array

**Context fields ignored:** `peer`, `direction` (test-environment dependent)

---

## Test Execution Flow

### Encode Tests

```
1. Runner builds ze + ze-peer to temp dir
2. Starts ze-peer on unique port with .ci expectations
3. Starts ze bgp with config
4. ze bgp connects, sends OPEN, receives OPEN
5. ze bgp sends UPDATE messages (from static routes)
6. ze-peer validates messages against expectations
7. ze-peer prints "successful" or error
```

### API Tests

```
1. Same as encode tests, plus:
5. ze bgp spawns .run script as subprocess
6. .run script sends commands via API
7. ze bgp processes commands, sends UPDATE messages
8. ze-peer validates messages
```

---

## Display Output

### Progress (during execution)

```
[5/20s] passed 12 running 4 [S:open, V:update] failed 2 [A, B]
```

| Field | Meaning |
|-------|---------|
| `[N/Ms]` | Longest running test: N seconds elapsed, M timeout |
| `passed N` | N tests passed |
| `running N [IDs]` | N tests currently executing (names shown when <= 5) |
| `failed N [IDs]` | N tests failed, with nicks |

### Section Header

Each test suite is framed by a section header:
```
═══════════════════════ encode ════════════════════════════════════════════════
```

### Summary (single line, parseable)

On success:
```
═══ PASS  42/42  100.0%  3.2s
```

On failure:
```
═══ FAIL  40/42  95.2%  3.2s  failed 2 [A, B]  timeout 1 [C]
```

| Field | Format | Meaning |
|-------|--------|---------|
| Verdict | `PASS` or `FAIL` | Green if all passed, red otherwise |
| Ratio | `N/M` | Passed / total |
| Rate | `N.N%` | Pass percentage |
| Time | `N.Ns` or `Nms` | Wall-clock elapsed |
| Failed | `failed N [nicks]` | Only shown when > 0, red |
| Timeout | `timeout N [nicks]` | Only shown when > 0, yellow |

**Regex for parsing:** `═══ (PASS|FAIL)\s+(\d+)/(\d+)\s+([0-9.]+%)\s+(\S+)`

### Stress Test Mode

Use `--count N` (`-c N`) to run tests multiple times for benchmarking or detecting flaky tests:

```bash
# Run test C 10 times with timing
ze-test bgp plugin -c 10 C

# Run all encoding tests 5 times
ze-test bgp encode -c 5 -a
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
ze-test bgp encode --timeout 60s --verbose 4
```

### Manual execution

```bash
# Terminal 1: Start peer
ze-peer --port 1790 test/encode/ebgp.ci

# Terminal 2: Run ze bgp (port is now per-peer config, not a global env var)
ze bgp server test/encode/ebgp.conf
```

### Decode message bytes

```bash
# Decode UPDATE payload
ze bgp decode update 0000001540010100400200400304650165014005040000006400

# Decode full message
ze bgp decode raw FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D02...
```

---

## Adding New Tests

### Option 1: Tmpfs (Recommended)

Single self-contained `.ci` file with embedded config:

```
# test/encode/mytest.ci
tmpfs=mytest.conf:terminator=EOF_CONF
peer loopback {
    remote {
        ip 127.0.0.1;
        as 1;
    }
    router-id 1.2.3.4;
    local-address 127.0.0.1;
    local-as 1;

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
# test/encode/mytest.conf
peer loopback {
    remote {
        ip 127.0.0.1;
        as 1;
    }
    router-id 1.2.3.4;
    local-address 127.0.0.1;
    local-as 1;

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
# test/encode/mytest.ci
option=file:path=mytest.conf
cmd=api:conn=1:seq=1:text=update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D020000001540010100...
```

### Generate expected bytes

Run with ExaBGP first to capture correct bytes, or use `ze bgp decode` to verify.

### Adding Negative Parsing Tests

To test that invalid configs are rejected with specific errors, create a `.ci` file:

```
# test/parse/my-error.ci
stdin=config:terminator=EOF_CONF
bgp {
    peer test-peer {
        remote {
            ip 10.0.0.1;
            as 65002;
        }
        router-id 1.2.3.4;
        local-as 65001;
        # ... invalid configuration ...
    }
}
EOF_CONF

cmd=foreground:seq=1:exec=ze bgp validate -:stdin=config
expect=exit:code=1
expect=stderr:contains=specific error message substring
```

**Regex match** (for variable parts like IPs, line numbers):
```
expect=stderr:regex=peer \d+\.\d+\.\d+\.\d+: route-refresh requires
```

The test passes if:
- `ze bgp validate` exits with code 1
- Stderr contains the expected substring OR matches the regex pattern

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
| `timing.go` | Per-test timing baseline with auto-timeout |
| `timing_test.go` | Timing baseline tests |
| `parallel.go` | Parallel test execution |
| `tmpfs_test.go` | Tmpfs parsing integration tests |

<!-- source: internal/test/runner/color.go -- ANSI colors -->
<!-- source: internal/test/runner/decode.go -- BGP message decoding -->
<!-- source: internal/test/runner/display.go -- live progress display -->
<!-- source: internal/test/runner/json.go -- JSON validation -->
<!-- source: internal/test/runner/limits.go -- ulimit check -->
<!-- source: internal/test/runner/ports.go -- port allocation -->
<!-- source: internal/test/runner/record.go -- test record state machine -->
<!-- source: internal/test/runner/report.go -- failure reports -->
<!-- source: internal/test/runner/runner.go -- test execution engine -->
<!-- source: internal/test/runner/stress.go -- stress iteration stats -->
<!-- source: internal/test/runner/timing.go -- per-test timing baseline -->
<!-- source: internal/test/runner/parallel.go -- parallel test execution -->

### Package: `internal/tmpfs/`

| File | Purpose |
|------|---------|
| `tmpfs.go` | Tmpfs parser and writer |
| `limits.go` | Configurable limits from environment |
| `security.go` | Path validation (traversal, escape) |
| `cleanup.go` | Signal handling for temp cleanup |

### Entry Point: `cmd/ze-test/`

<!-- source: cmd/ze-test/main.go -- test runner entry point -->
<!-- source: cmd/ze-test/bgp.go -- bgp test subcommand -->
<!-- source: cmd/ze-test/syslog.go -- syslog server subcommand -->
<!-- source: cmd/ze-test/rpki.go -- RPKI mock RTR subcommand -->

Subcommand-based CLI with `bgp` for BGP test execution, `syslog` for syslog server, and `rpki` for deterministic RPKI mock RTR server.

### ze-test rpki

Deterministic RTR (RFC 8210) cache server for RPKI functional tests. Auto-generates VRPs for all /8 prefixes based on the first octet modulo 3:

| First octet % 3 | VRP | Result (AS 65001) |
|-----------------|-----|-------------------|
| 0 | ASN=65001, maxLen=/32 | Valid |
| 1 | ASN=65099, maxLen=/32 | Invalid |
| 2 | No VRP | NotFound |

Usage: `ze-test rpki --port 3323 [--valid-asn 65001] [--invalid-asn 65099]`

### Security

- Path traversal protection on `option:file:` and `.run` scripts
- Process isolation via `Setpgid`
- Context timeouts on all execution
- Dynamic port allocation prevents conflicts

### ExaBGP Compatibility Test Ports

ExaBGP compatibility tests (`make ze-exabgp-test`) use OS-assigned dynamic ports. The mock BGP server (`test/exabgp-compat/bin/bgp`) binds to port 0, receives an OS-assigned port, and prints `PORT <N>` to stdout. The test runner (`test/exabgp-compat/bin/functional`) reads this line from the server's stdout temp file and passes the discovered port to the client subprocess. This eliminates port collisions when running concurrent test instances. Use `--port N` to override with an explicit port for debugging.
<!-- source: test/exabgp-compat/bin/bgp -- dynamic port binding and PORT line output -->
<!-- source: test/exabgp-compat/bin/functional -- _discover_port method -->

---

## Per-Test Timing Baseline

`ze-test` maintains a rolling timing baseline in `tmp/test-timings.json` that enables two features:
<!-- source: internal/test/runner/timing.go -- TimingEntry, LoadTimings, SaveTimings -->

**Auto-timeout:** Each test's timeout is calculated as `min(global_timeout, max(5s, 5x baseline_avg))`. A test that normally takes 500ms gets a 5s timeout instead of the default 15s. This catches hangs in seconds rather than waiting for the global timeout. Explicit `option=timeout:value=` in the `.ci` file always takes precedence.

**Slow detection:** Tests exceeding 2x their baseline average are flagged in the summary output. Investigate performance regressions before ignoring these warnings.

The baseline uses an exponential moving average (EMA, alpha=0.3) and requires 3 samples before it is used for auto-timeout. The timings file is capped at 10 MB; if it grows beyond that, timing data is reset.

---

## Route Delivery Synchronization

Plugin test scripts use `wait_for_ack()` from `test/scripts/ze_api.py` to ensure routes have been delivered to peers before proceeding. This function sends a `ze-bgp:peer-flush` RPC that blocks until all forward pool workers have drained their queued items to peer sockets (deterministic barrier).
<!-- source: test/scripts/ze_api.py -- wait_for_ack() function -->
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- peer-flush RPC handler -->
<!-- source: internal/component/bgp/reactor/forward_pool_barrier.go -- forward pool flush barrier -->

**When writing new plugin tests:**

| Pattern | Use |
|---------|-----|
| After a batch of `send()` calls | Call `wait_for_ack()` before checking results or sending dependent commands |
| Between independent `send()` calls | No synchronization needed (FIFO ordering per peer is guaranteed) |
| Before `wait_for_shutdown()` | Call `wait_for_ack()` to ensure all routes hit the wire |

**Do NOT use `time.sleep()` for forward delivery synchronization.** The flush barrier is deterministic and does not depend on timing. Use `time.sleep()` only for non-forward-pool concerns (session establishment, RPKI cache, event propagation).

---

## Editor Tests (.et format)

Editor tests (`test/editor/`) verify the interactive TUI editor and CLI using headless keystroke simulation. Run with `make ze-editor-test` or `bin/ze-test editor`.

<!-- source: internal/component/cli/testing/parser.go -- .et file parser -->

### Key Directives

| Directive | Example | Purpose |
|-----------|---------|---------|
| `tmpfs=` | `tmpfs=test.conf:terminator=EOF` | Embed test files |
| `option=file:path=` | `option=file:path=test.conf` | Config file to load |
| `option=mode:value=` | `option=mode:value=command` | Command-only mode (no editor) |
| `option=history:store` | `option=history:store` | Enable zefs-backed history persistence |
| `input=type:text=` | `input=type:text=show` | Type text |
| `input=enter/up/down/tab` | `input=enter` | Press named key |
| `expect=input:value=` | `expect=input:value=show` | Assert input buffer content |
| `expect=mode:is=` | `expect=mode:is=command` | Assert editor mode |
| `restart=` | `restart=editor` | Simulate exit + relaunch (blob store persists) |

---

## Live Tests (Docker + Internet)

Live tests run against real external infrastructure inside Docker containers.
They are **not** part of `make ze-verify` and require both Docker and internet access.

<!-- source: internal/component/bgp/plugins/rpki/rpki_live_test.go -- TestLiveRPKIValidation -->

```bash
make ze-live-test    # Run all live tests
```

### Build Tag

Live tests use `//go:build live`. They are excluded from all normal test targets
(`ze-unit-test`, `ze-functional-test`, `ze-verify`). The `ze-live-test` make target
passes `-tags live` to include them.

### RPKI Live Test

Starts a [stayrtr](https://github.com/bgp/stayrtr) container that fetches real-world
RPKI data, connects ze's RTR client, and validates known prefixes:

| Prefix | Origin AS | Expected | Owner |
|--------|-----------|----------|-------|
| `1.1.1.0/24` | 13335 | Valid | Cloudflare DNS |
| `8.8.8.0/24` | 15169 | Valid | Google DNS |
| `82.212.0.0/16` | 64496 | NotFound | No ROA coverage |
| `1.1.1.0/24` | 64496 | Invalid | Wrong origin for covered prefix |

**Requirements:** Docker, internet access (fetches ~5 MB rpki.json from Cloudflare).
**Timeout:** 180s (includes image pull, data fetch, RTR sync).
**Skip behavior:** Test skips gracefully if Docker is unavailable or image cannot be pulled.

---

## Integration Tests (Network Namespaces)

Integration tests exercise the `internal/component/iface/` package against the real Linux
kernel inside ephemeral network namespaces. They require `CAP_NET_ADMIN` (typically root)
and are excluded from all normal test targets.

<!-- source: internal/component/iface/integration_helpers_linux_test.go -- withNetNS, waitForEvent -->

```bash
make ze-integration-test        # Run all integration tests
make ze-integration-iface-test  # Run iface integration tests only
```

### Build Tag

Integration tests use `//go:build integration && linux`. They are excluded from
`ze-unit-test`, `ze-functional-test`, and `ze-verify`. The `ze-integration-iface-test`
make target passes `-tags integration` to include them.

### How They Work

Each test calls `withNetNS(t, func() { ... })` which:

1. Locks the goroutine to its OS thread (`runtime.LockOSThread`)
2. Creates a named network namespace (`netns.NewNamed`)
3. Switches into it (`netns.Set`)
4. Runs the test function (creating interfaces, addresses, etc.)
5. Restores the original namespace and deletes the test namespace in `t.Cleanup`

If namespace creation fails (missing `CAP_NET_ADMIN`), the test is skipped with `t.Skip`.

### Test Categories

| File | Covers | Tests |
|------|--------|-------|
| `manage_integration_linux_test.go` | Interface CRUD, addresses, MTU | 9 tests |
| `monitor_integration_linux_test.go` | Netlink event monitoring | 5 tests |
| `sysctl_integration_linux_test.go` | Real /proc/sys writes | 2 tests |
| `mirror_integration_linux_test.go` | tc qdisc/filter setup | 5 tests |
| `dhcp_integration_linux_test.go` | DHCPv4 with in-process server | 2 tests |
| `migrate_integration_linux_test.go` | Make-before-break migration | 2 tests |

### Shared Helpers

`integration_helpers_linux_test.go` provides:

| Helper | Purpose |
|--------|---------|
| `withNetNS(t, fn)` | Ephemeral namespace wrapper |
| `waitForEvent(t, bus, topic, timeout)` | Poll collectingBus for an event |
| `linkExists(name)` | Check if interface exists via netlink |
| `hasAddress(iface, cidr)` | Check if address is on interface |
| `requireLinkUp(t, name)` | Assert link is UP |
| `createDummyForTest(t, name)` | Create dummy with cleanup |
| `createVethForTest(t, name, peer)` | Create veth pair with cleanup |

---

## L2TP Tests

L2TP functional tests (`test/l2tp/`) verify tunnel lifecycle, session
negotiation, authentication, IP pool, and teardown over real loopback UDP.
Run with `make ze-l2tp-test` or `bin/ze-test l2tp`.

```bash
ze-test l2tp --list    # List available tests
ze-test l2tp --all     # Run all tests
```

### Tunnel lifecycle

| Test | File | What it verifies |
|------|------|-----------------|
| SCCRQ handshake | `test/l2tp/handshake-sccrq.ci` | Python client sends SCCRQ hex, receives SCCRP |
| Full handshake | `test/l2tp/handshake-full.ci` | SCCRQ/SCCRP/SCCCN/ZLB exchange with challenge/response |
| Bad challenge | `test/l2tp/bad-challenge-response.ci` | Wrong challenge response triggers StopCCN RC=4 |

### Session lifecycle

| Test | File | What it verifies |
|------|------|-----------------|
| Incoming session (LNS) | `test/l2tp/session-incoming-lns.ci` | ICRQ/ICRP/ICCN exchange establishes session |
| CDN teardown | `test/l2tp/session-cdn-teardown.ci` | CDN tears down one session cleanly |
| StopCCN cascade | `test/l2tp/session-stopccn-cascade.ci` | StopCCN tears down tunnel and all its sessions |

### Authentication and IP pool

| Test | File | What it verifies |
|------|------|-----------------|
| Auth + pool | `test/l2tp/session-auth-pool.ci` | Full session with auth-local + pool allocation |
| Auth-local config | `test/l2tp/auth-local-config.ci` | Static user config parsed and auth works |
| RADIUS basic | `test/l2tp/auth-radius-basic.ci` | RADIUS Access-Request sent, session authenticated |
| RADIUS reject | `test/l2tp/auth-radius-reject.ci` | RADIUS Access-Reject fails the session |
| Pool basic | `test/l2tp/pool-basic.ci` | Pool allocates and releases addresses |
| Pool minimal range | `test/l2tp/pool-minimal-range.ci` | Single-address pool boundary case |
| Re-auth interval | `test/l2tp/reauth-interval-clamp.ci` | Safety floor clamps the re-auth interval |

### Config parsing

| Test | File | What it verifies |
|------|------|-----------------|
| Minimal config | `test/parse/l2tp-minimal.ci` | `l2tp { server main { port 1701 } }` parses |
| Bad port | `test/parse/l2tp-bad-port.ci` | `port 0` rejected |
| Unknown field | `test/parse/l2tp-unknown-field.ci` | Unknown key rejected with suggestion |
| Max sessions | `test/parse/l2tp-max-sessions.ci` | `max-sessions` value accepted |

<!-- source: cmd/ze-test/l2tp.go -- l2tpCmd runner dispatch -->
<!-- source: internal/test/runner/record_parse.go -- .ci discovery and directive parsing -->

---

**Updated:** 2026-04-15
