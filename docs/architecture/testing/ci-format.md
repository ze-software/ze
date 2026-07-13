# .ci Test File Format

The `.ci` format is used by Ze's test runner to define functional tests. It supports embedded files (Tmpfs), test options, expectations, and commands.

> For the execution architecture (how tests are scheduled and run concurrently) and the web `.wb` format, see [`runner-architecture.md`](runner-architecture.md).

## Syntax Overview

All lines use key=value format with `:` separators:

```
action=type:key=value:key=value:...
```

| Action | Purpose |
|--------|---------|
| `stdin=` | Embed stdin content for processes |
| `tmpfs=` | Embed file content inline |
| `option=` | Test configuration |
| `cmd=` | Commands (API, shell, foreground/background) |
| `expect=` | Expectations to validate |
| `reject=` | Negative expectations (fail if matched) |
| `action=` | Actions (send notification, raw bytes) |
| `http=` | HTTP endpoint checks and readiness polls |
<!-- source: internal/test/runner/record_parse.go -- parseAndAdd, CI file parsing -->
<!-- source: internal/test/tmpfs/tmpfs.go -- Tmpfs, File, Parse -->

## Key Concepts

### Suite label, test id, and failure identity

The verify debugging protocol identifies a functional failure with:

| Field | Source | Purpose |
|-------|--------|---------|
| Suite label | `ze-test` runner label such as `plugin`, `ui`, or `managed` | First routing boundary inside `ze-functional-test` |
| Test id | One-based decimal id printed by `--list` and per-test result lines | Exact single-test rerun scope |
| Run number | `N/TOTAL` printed by `--list` and per-test result lines | Human progress marker for long suites |
| CI file path | Parsed `.ci` source path | Full test definition and embedded fixtures |
| Failure kind | Runner failure type, timeout state, or mismatch class | Conservative grouping key |
| Expected / received evidence | `TEST FAILURE` block detail | Full debugging evidence in the stage log |

In `ZE_VERIFY_MODE=1`, failed suites emit native failure-group metadata before
the full failure blocks. The compact verify index uses that metadata for group
routing and keeps the full `TEST FAILURE` blocks in the stage log.
<!-- source: internal/test/runner/failure_group.go -- suite-local failure groups -->
<!-- source: internal/test/runner/report.go -- TEST FAILURE blocks -->
<!-- source: internal/test/runner/display.go -- per-test result lines -->

### Per-step trace output

All three runner families (`.ci`, `.wb`, `.et`) record per-step outcomes during
execution and emit dual-format trace output:

- **Human:** colored checkmark/cross glyph per step with kind, assert, and detail.
- **Machine:** `VERIFY STEP: {"file":"...","step":N,"kind":"...","status":"pass|fail",...}` -- one JSON line per step, matching the `VERIFY FAILURE GROUP:` prefix convention.

Trace is emitted automatically for failed tests. Under `-v`, passing tests also
show their trace. The `.ci` runner includes the trace in its `TEST FAILURE` report
block when `StepTrace` is non-empty.
<!-- source: internal/test/trace/trace.go -- StepResult, PrintTrace, writeHuman, writeMachine -->
<!-- source: internal/test/runner/report.go -- step trace in failure reports -->

### conn and seq

Most directives use `conn=N` and `seq=N` to identify message ordering:

- **conn** (connection): 1-based TCP connection index. Each `ze-peer` instance
  manages one TCP connection. Multi-peer tests use `conn=1` for the first peer,
  `conn=2` for the second, etc. The maximum is set by `option=tcp_connections:value=N`.
- **seq** (sequence): 1-based message sequence within a connection. `seq=1` is the
  first BGP message after OPEN/KEEPALIVE, `seq=2` is the second, etc.

A test with two peers and one UPDATE each uses `conn=1:seq=1` and `conn=2:seq=1`,
not `conn=1:seq=1` and `conn=1:seq=2`.

### Port Substitution

The runner assigns ephemeral ports and exposes them as variables in commands and URLs:

| Variable | Meaning |
|----------|---------|
| `$PORT` | BGP peer port (assigned by runner, used by `ze-peer --port $PORT`) |
| `$PORT2` | Secondary port (web UI, looking glass, etc.) |

Never hardcode port numbers. Use `$PORT` in `cmd=` exec values and `$PORT2` in `http=` URLs.

## Stdin Blocks

Stdin blocks embed content that will be piped to a process's stdin.

### Syntax

**Multi-line (with terminator):**
```
stdin=<name>:terminator=<TERM>
<content>
<TERM>
```

**Single-line hex:**
```
stdin=<name>:hex=<hex-value>
```

**Single-line text:**
```
stdin=<name>:text=<text-value>
```

### Parameters

| Parameter | Description |
|-----------|-------------|
| `name` | Identifier referenced by `cmd=...:stdin=<name>` |
| `terminator` | End marker for multi-line content |
| `hex` | Hex-encoded content (single-line) |
| `text` | Plain text content (single-line, newline appended) |
<!-- source: internal/test/tmpfs/tmpfs.go -- StdinBlocks map, parseStdin -->

### Examples

**Multi-line (config):**
```
stdin=ze-bgp:terminator=EOF_CONF
peer test-peer {
    remote {
        ip 127.0.0.1;
        as 65533;
    }
    local-as 65533;
}
EOF_CONF

cmd=foreground:seq=1:exec=ze bgp server -:stdin=ze
```

**Single-line hex (decode test):**
```
stdin=payload:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003C020000001C...
cmd=foreground:seq=1:exec=ze-test decode --family ipv4/unicast -:stdin=payload
expect=json:json={ "type": "update", ... }
```

**Single-line text:**
```
stdin=cmd:text=update text nhop set 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24
```

## Tmpfs (Virtual File System)

Tmpfs allows embedding multiple files within a single `.ci` file. Files are extracted to a temp directory at runtime.

### Syntax

```
tmpfs=<path>[:mode=<octal>][:encoding=<type>]:terminator=<TERM>
<content>
<TERM>
```

### Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `path` | Yes | - | Relative path (no `..`, no absolute) |
| `mode` | No | Auto | File permissions (octal: 644, 755) |
| `encoding` | No | `text` | Content encoding: `text` or `base64` |
| `terminator` | Yes | - | End marker (alone on line) |
<!-- source: internal/test/tmpfs/tmpfs.go -- File struct, Tmpfs.AddFile -->

### Mode Defaults

| Pattern | Default |
|---------|---------|
| `*.py`, `*.sh`, `*.pl`, `*.rb`, `*.bash`, `*.zsh` | 0755 |
| Everything else | 0644 |

### Terminator Rules

- Must be non-empty
- Must be unique within file (no two Tmpfs blocks can share terminator)
- Alphanumeric and underscore only: `[A-Za-z0-9_]+`
- Matched exactly (no whitespace trimming)
- Recommended: `EOF_<PURPOSE>` (e.g., `EOF_CONF`, `EOF_PY`)

### Example

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

tmpfs=plugin.py:mode=755:terminator=EOF_PY
#!/usr/bin/env python3
print('{"ready": true}')
EOF_PY

option=file:path=peer.conf
option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=FFFF...
```

### Security Constraints

1. **No absolute paths** - must be relative
2. **No parent traversal** - no `..` components
3. **No hidden files** - no `.` prefix in path components
4. **Path length limit** - max 256 characters
5. **Path depth limit** - max 10 levels
<!-- source: internal/test/tmpfs/security.go -- validatePath, Validate -->

### Limits

Configurable via environment variables:

| Limit | Default | Environment Variable |
|-------|---------|---------------------|
| Max file size | 1 MB | `ze.bgp.ci.max_file_size` |
| Max total size | 1 MB | `ze.bgp.ci.max_total_size` |
| Max files | 100 | `ze.bgp.ci.max_files` |
| Max path length | 256 | `ze.bgp.ci.max_path_length` |
| Max path depth | 10 | `ze.bgp.ci.max_path_depth` |

## Options

```
option=<type>:key=value[:key=value...]
```

| Type | Keys | Description |
|------|------|-------------|
| `file` | `path=<name>` | Config file to use |
| `asn` | `value=<N>` | Override peer ASN |
| `bind` | `value=ipv6` | Bind to IPv6 |
| `timeout` | `value=<duration>` | Test timeout (e.g., `30s`). Overrides auto-timeout. |
| `tcp_connections` | `value=<N>` | Number of TCP connections |
| `open` | `value=<behavior>` | OPEN message behavior |
| `update` | `value=<behavior>` | UPDATE message behavior |
| `env` | `var=<KEY>:value=<V>` | Set environment variable |
| `skip-os` | `value=<os>[,<os>]` | Skip test on listed GOOS values (e.g., `darwin`, `linux`) |
| `needs-linux` | (none) | Linux-only test (boots a daemon that exercises real kernel features). SKIPs on non-Linux hosts and runs automatically in the QEMU Alpine VM via `make ze-qemu-all-test`. See `ai/rules/qemu-testing.md`. |
<!-- source: internal/test/runner/record_parse.go -- parseAndAdd, option parsing -->

### OPEN Behaviors

| Value | Description |
|-------|-------------|
| `send-unknown-capability` | Add unknown capability (code 66) to OPEN |
| `inspect-open-message` | Validate received OPEN against expectations |
| `send-unknown-message` | Send unknown message type (255) after OPEN |
| `drop-capability` | Remove a capability from ze-peer's OPEN response |
| `add-capability` | Add a capability to ze-peer's OPEN response |
<!-- source: internal/test/peer/checker.go -- OPEN behavior handling -->

### Capability Control (drop-capability / add-capability)

Ze-peer mirrors the peer's OPEN message back (with a modified router-id). The `drop-capability` and `add-capability` options modify this mirrored OPEN at wire level, allowing tests to control exactly which capabilities ze-peer advertises.

**Drop a capability:**

```
option=open:value=drop-capability:code=<N>
```

Removes the capability with the given code from ze-peer's OPEN response. The peer will not see this capability in the mirrored OPEN.

**Add a capability:**

```
option=open:value=add-capability:code=<N>:hex=<value-bytes>
```

Adds a capability with the given code and hex-encoded value bytes to ze-peer's OPEN response.

| Key | Description |
|-----|-------------|
| `code` | Capability code (1-255), e.g., 65 for ASN4, 2 for route-refresh |
| `hex` | Hex-encoded capability value bytes (only for add-capability) |

**Use case — testing capability mode enforcement:**

When Ze is configured with `require` mode for a capability, it sends a NOTIFICATION if the peer lacks that capability. To test this, use `drop-capability` to make ze-peer omit the capability from its response:

```
# Test: Ze requires ASN4, ze-peer drops it → Ze should send NOTIFICATION
option=open:value=drop-capability:code=65
```

When Ze is configured with `refuse` mode, it sends a NOTIFICATION if the peer has a capability. To test this, the default mirror behavior already includes the capability, but `add-capability` can add capabilities not in the original OPEN:

```
# Test: Add a custom capability for refuse testing
option=open:value=add-capability:code=73:hex=067A652D626770
```

**Multiple overrides** can be combined:

```
option=open:value=drop-capability:code=65
option=open:value=drop-capability:code=2
option=open:value=add-capability:code=73:hex=067A652D626770
```

## Commands

```
cmd=<type>:key=value[:key=value...]
```

### API Commands

```
cmd=api:conn=<N>:seq=<N>:text=<command>
```

| Key | Description |
|-----|-------------|
| `conn` | Connection number (1-4) |
| `seq` | Sequence number within connection |
| `text` | API command text |

### Example

```
cmd=api:conn=1:seq=1:text=update text origin set igp nhop set 10.0.1.1 nlri ipv4/unicast add 10.0.0.0/24
```

### Process Commands (Foreground/Background)

For orchestrating multiple processes:

```
cmd=background:seq=<N>:exec=<command>[:stdin=<name>]
cmd=foreground:seq=<N>:exec=<command>[:stdin=<name>][:timeout=<dur>]
```

| Key | Description |
|-----|-------------|
| `seq` | Execution order (lower first) |
| `exec` | Command to execute |
| `stdin` | Stdin block name to pipe |
| `timeout` | Foreground timeout (e.g., `10s`) |

**Background:** Starts and keeps running until test ends.
**Foreground:** Starts and waits for completion.

**Daemon readiness (`ze` only):** a `ze` daemon launched **either** foreground or
background is told (via `ZE_READY_FILE`) to write `daemon.ready` once startup
completes, and the runner publishes its PID to `daemon.pid` in the tmpfs directory.
Tests poll both files -- directly or through a `driver.py` helper -- before
signalling the daemon (`action=sighup`/`action=sigterm`) or asserting on it. This
handshake is armed only for `ze` daemons: `ze-peer` and helper scripts never get
`ZE_READY_FILE` and never have their PID written to `daemon.pid`.
<!-- source: internal/test/runner/runner_exec.go -- process orchestration -->

### Example (Decode Test)

```
stdin=payload:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003C...
cmd=foreground:seq=1:exec=ze-test decode --family ipv4/unicast -:stdin=payload
expect=json:json={ "type": "update", ... }
```

### Example (Multi-Process)

```
stdin=peer:terminator=EOF_PEER
option=asn:value=65000
expect=bgp:conn=1:seq=1:hex=FFFF...
EOF_PEER

stdin=ze-bgp:terminator=EOF_CONF
peer test-peer { remote { ip 127.0.0.1; } ... }
EOF_CONF

cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=peer
cmd=foreground:seq=2:exec=ze bgp server -:stdin=ze-bgp:timeout=10s
```

### Example (Multi-Peer)

Tests needing two or more BGP peers use the same `$PORT` on different loopback
addresses. The `ze_bgp_tcp_port` override sets all peer ports uniformly, which
is correct because every peer listens on `$PORT`.

```
# Source peer on 127.0.0.1 (default)
stdin=source:terminator=EOF_SOURCE
option=tcp_connections:value=1
action=send:conn=1:seq=1:hex=FFFF...
EOF_SOURCE

# Dest peer on 127.0.0.2 -- sink mode absorbs any UPDATE
stdin=dest:terminator=EOF_DEST
option=tcp_connections:value=1
EOF_DEST

cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=source
cmd=background:seq=2:exec=ze-peer --bind 127.0.0.2 --mode sink --port $PORT:stdin=dest
cmd=foreground:seq=3:exec=ze -:stdin=ze-bgp:timeout=20s
```

Each ze-peer gets independent output capture and `WaitFor` synchronization.
The runner waits for each peer's "listening on" message before starting the
next command.

On Linux, 127.0.0.2 works automatically (127.0.0.0/8 routes to lo). On macOS
and FreeBSD, the test runner adds loopback aliases via the `SIOCAIFADDR` ioctl.
<!-- source: internal/test/runner/loopback_linux.go -- no-op on Linux -->
<!-- source: internal/test/runner/loopback_darwin.go -- SIOCAIFADDR on BSD -->

## Expectations

```
expect=<type>:key=value[:key=value...]
```

### BGP Wire Expectations

```
expect=bgp:conn=<N>:seq=<N>:hex=<hex-bytes>
```

Validates the exact BGP wire message received.

### JSON Expectations

```
expect=json:conn=<N>:seq=<N>:json=<json-object>
expect=json:json=<json-object>
```

Validates the decoded message matches expected JSON.

**Validation rules:**
- Parsed and compared field-by-field (key order independent)
- Volatile fields removed before comparison: `exabgp`, `ze-bgp`, `time`, `host`, `pid`, `ppid`, `counter`
- Neighbor normalization: `peer` ↔ `neighbor` treated as equivalent, `direction` field ignored
- All non-volatile fields must match exactly
<!-- source: internal/test/runner/runner_validate.go -- JSON comparison, volatile field removal -->

### Exit Code Expectations

```
expect=exit:code=<N>
```

Validates the foreground process exit code.

### Stdout Expectations

```
expect=stdout:contains=<text>
expect=stdout:!contains=<text>
expect=stdout:pattern=<regex>
```

Three modes:
- `contains=` -- substring match against stdout (multiple allowed, all must match)
- `!contains=` -- negative substring match (stdout must NOT contain text)
- `pattern=` -- regex match against stdout (uses Go `regexp` syntax)

### Stderr Expectations

```
expect=stderr:pattern=<regex>
expect=stderr:contains=<text>
```

Two modes:
- `pattern=` — regex match against stderr (uses Go `regexp` syntax)
- `contains=` — substring match against stderr

### Syslog Expectations

```
expect=syslog:pattern=<regex>
```

Validates that captured syslog output matches the regex pattern. When any `expect=syslog:` line is present, the test runner automatically starts a UDP syslog server and injects `ze.log.backend=syslog` and `ze.log.destination=127.0.0.1:<port>` into the test environment.
<!-- source: internal/test/syslog/testsyslog.go -- UDP syslog server for tests -->

### File Expectations

```
expect=file:path=<rel>:exists=true
expect=file:path=<rel>:absent=true
expect=file:path=<rel>:contains=<text>
expect=file:path=<rel>:not-contains=<text>
expect=file:glob=<rel-pattern>:count=<N>
expect=file:glob=<rel-pattern>:contains=<text>
expect=file:glob=<rel-pattern>:not-contains=<text>
```

Validates files after the test process or peer sequence has completed. Paths and
glob patterns are relative to the tmpfs directory when the test uses `tmpfs=`,
otherwise relative to the `.ci` file directory. For glob `contains`, at least one
matched file must contain the text. For glob `not-contains`, no matched file may
contain it.

Use file expectations for post-run artifacts such as generated configs, pointer
files, and logs. Do not write shell just to inspect files.
<!-- source: internal/test/runner/runner_validate.go -- validateFileChecks -->

### Negative Expectations (reject)

```
reject=stderr:pattern=<regex>
reject=stdout:contains=<text>
reject=stdout:pattern=<regex>
reject=syslog:pattern=<regex>
```

Inverse of `expect=` -- the test **fails** if the pattern matches. Used to verify that unwanted output (e.g., deprecated warnings, ERROR-level messages) does NOT appear.

| Type | Description |
|------|-------------|
| `reject=stderr:pattern=<regex>` | Fail if stderr matches regex |
| `reject=stdout:contains=<text>` | Fail if stdout contains substring |
| `reject=stdout:pattern=<regex>` | Fail if stdout matches regex |
| `reject=syslog:pattern=<regex>` | Fail if syslog output matches regex |
<!-- source: internal/test/runner/runner_exec.go -- stdout reject handling -->
<!-- source: internal/test/runner/runner_validate.go -- stderr/syslog reject handling -->

## Actions

```
action=<type>:key=value[:key=value...]
```

### Notification

```
action=notification:conn=<N>:seq=<N>:text=<message>
```

Sends NOTIFICATION with shutdown message.

### Send Raw

```
action=send:conn=<N>:seq=<N>:hex=<hex-bytes>
```

Sends raw bytes to peer.

### Rewrite Config File

```
action=rewrite:conn=<N>:seq=<N>:source=<tmpfs-file>:dest=<config-file>
```

Copies a tmpfs file over the daemon's config file. Used with `action=sighup` to test config reload.

| Key | Description |
|-----|-------------|
| `conn` | Connection number triggering the rewrite |
| `seq` | Sequence number (after matching messages) |
| `source` | Source file name in tmpfs |
| `dest` | Destination file name in tmpfs (usually `ze-bgp.conf`) |

### Send SIGHUP

```
action=sighup:conn=<N>:seq=<N>
```

Sends SIGHUP to the daemon process. Reads PID from `daemon.pid` in the tmpfs directory (written automatically by the test runner).

| Key | Description |
|-----|-------------|
| `conn` | Connection number triggering the signal |
| `seq` | Sequence number (after matching messages) |

### Send SIGTERM

```
action=sigterm:conn=<N>:seq=<N>
```

Sends SIGTERM to the daemon process. Reads PID from `daemon.pid` in the tmpfs directory (written automatically by the test runner). After sending SIGTERM, the connection is expected to close (daemon shuts down gracefully).

| Key | Description |
|-----|-------------|
| `conn` | Connection number triggering the signal |
| `seq` | Sequence number (after matching messages) |

## HTTP Checks

HTTP checks validate web endpoint responses after all `cmd=` processes have started.
Executed in `seq` order with automatic retry on connection errors (server starting up).

### Assertion Checks (get/post)

```
http=get:seq=N:url=URL:status=CODE[:contains=TEXT][:bodyfile=PATH]
http=post:seq=N:url=URL:status=CODE[:contains=TEXT][:bodyfile=PATH][:sendfile=PATH][:content-type=TYPE][:insecure-tls=true]
```

| Key | Required | Description |
|-----|----------|-------------|
| `seq` | Yes | Execution order (>= 1, lower first) |
| `url` | Yes | Request URL (supports `$PORT` and `$PORT2` substitution) |
| `status` | Yes | Expected HTTP status code |
| `contains` | No | Expected body substring |
| `bodyfile` | No | Path to file with expected body (exact match, resolved relative to `.ci` file) |
| `sendfile` | No | Path to file sent as POST request body, resolved from tmpfs first |
| `content-type` | No | Request body content type for `sendfile`, defaults to `application/json` |
| `insecure-tls` | No | Set `true` for self-signed local HTTPS endpoints |
<!-- source: internal/test/runner/runner_validate.go -- executeOneHTTPCheck -->

Retries up to 20 times at 200ms intervals on transient connection errors (ECONNREFUSED, ECONNRESET, EOF).
Non-connection errors (wrong status, missing content) fail immediately.

### Readiness Polls (wait)

```
http=wait:seq=N:url=URL:status=CODE[:contains=TEXT][:timeout=DUR]
```

| Key | Required | Default | Description |
|-----|----------|---------|-------------|
| `seq` | Yes | - | Execution order (>= 1) |
| `url` | Yes | - | Request URL |
| `status` | Yes | - | Expected HTTP status code |
| `contains` | No | - | Expected body substring |
| `timeout` | No | `15s` | Poll timeout duration |
<!-- source: internal/test/runner/runner_validate.go -- executeOneHTTPWait -->

Unlike assertion checks, `wait` retries on **all** failures: connection errors, wrong status codes,
and content mismatches. Polls at 500ms intervals until the condition is met or the timeout expires.
Wait checks run **before** assertion checks, making them suitable for waiting until a server has
populated data (e.g., routes injected by an async plugin).

### Example

```
cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=peer
cmd=background:seq=2:exec=ze -:stdin=ze-bgp

# Wait until routes are available before checking graph output
http=wait:seq=1:url=http://127.0.0.1:$PORT2/lg/graph?prefix=10.10.1.0/24&mode=aspath&format=text:status=200:contains=AS2914:timeout=15s

# Exact SVG match against reference file
http=get:seq=1:url=http://127.0.0.1:$PORT2/lg/graph?prefix=10.10.1.0/24&mode=aspath:status=200:bodyfile=expect/graph.svg

# Substring check
http=get:seq=2:url=http://127.0.0.1:$PORT2/lg/graph?prefix=10.10.1.0/24&mode=nexthop&format=text:status=200:contains=egress
```

## Complete Example

```
# Embed config using Tmpfs
tmpfs=test.conf:terminator=EOF_CONF
peer test-peer {
    remote {
        ip 127.0.0.1;
        as 65000;
    }
    router-id 10.0.0.2;
    local-address 127.0.0.1;
    local-as 65533;
    hold-time 180;

    family {
        ipv4/unicast;
    }
    announce {
        ipv4 {
            unicast 10.0.0.0/24 next-hop 10.0.1.254;
        }
    }
}
EOF_CONF

# Test configuration
option=file:path=test.conf
option=asn:value=65000

# Expected API command and wire output
cmd=api:conn=1:seq=1:text=update text origin set igp nhop set 10.0.1.254 nlri ipv4/unicast add 10.0.0.0/24
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FFFD4003040A0001FE180A0000

# EOR
cmd=api:conn=1:seq=1:text=announce eor ipv4/unicast
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000
```

## Consumers

Different components consume different line types:

| Line Type | Consumer |
|-----------|----------|
| `stdin=` | Test runner (pipes to processes) |
| `tmpfs=` | Test runner (writes to temp) |
| `option=` | Test runner + ze-peer |
| `cmd=api:` | Test runner (sends to ze-peer) |
| `cmd=foreground:`, `cmd=background:` | Test runner (process orchestration) |
| `expect=exit:`, `stdout:`, `stderr:`, `json:`, `syslog:`, `file:` | Test runner |
| `reject=stderr:`, `reject=syslog:` | Test runner (negative expectations) |
| `http=get:`, `http=post:` | Test runner (HTTP assertion checks) |
| `http=wait:` | Test runner (HTTP readiness polls) |
| `expect=bgp:` | ze-peer |
| `action=notification:`, `action=send:` | ze-peer |
| `action=rewrite:`, `action=sighup:`, `action=sigterm:` | ze-peer (reload/signal tests) |
<!-- source: internal/test/peer/expect.go -- ze-peer expectation handling -->
<!-- source: internal/test/runner/record.go -- Record, State -->

Lines not recognized by a consumer are ignored.

## Migration from Old Format

Old format (deprecated):
```
option:file:test.conf
option:asn:65000
1:raw:FFFF...
1:json:{...}
```

New format:
```
option=file:path=test.conf
option=asn:value=65000
expect=bgp:conn=1:seq=1:hex=FFFF...
expect=json:conn=1:seq=1:json={...}
```

Key changes:
- `=` instead of `:` after action
- Explicit `conn=` and `seq=` for message ordering
- `hex=` prefix for wire bytes
- `json=` prefix for JSON data

## Editor Test Format (.et)

The `.et` format extends `.ci` for interactive editor testing. Tests are located in `test/editor/`.
<!-- source: internal/component/cli/testing/parser.go -- editor test parsing -->

### Overview

Editor tests simulate user input sequences against the headless configuration editor and verify state changes.

### Input Actions

| Action | Purpose | Example |
|--------|---------|---------|
| `input=type:text=<text>` | Type text | `input=type:text=edit bgp` |
| `input=key:name=<key>` | Send special key | `input=key:name=tab` |
| `input=tab` | Tab key (shorthand) | `input=tab` |
| `input=enter` | Enter key (shorthand) | `input=enter` |
| `input=ctrl:key=<c>` | Ctrl+key | `input=ctrl:key=u` |
| `input=space` | Space key | `input=space` |

### Expectations

| Expectation | Purpose | Example |
|-------------|---------|---------|
| `expect=context:path=<p>` | Context equals path | `expect=context:path=bgp.peer.1.1.1.1` |
| `expect=context:root` | Context is root | `expect=context:root` |
| `expect=completion:contains=<list>` | Completions include all items | `expect=completion:contains=set,delete,edit` |
| `expect=completion:excludes=<list>` | Completions must NOT include items | `expect=completion:excludes=vpp,kernel` |
| `expect=completion:count=<N>` | Number of completions | `expect=completion:count=5` |
| `expect=ghost:text=<suffix>` | Ghost text suggestion | `expect=ghost:text=-id` |
| `expect=dirty:true` | Has unsaved changes | `expect=dirty:true` |
| `expect=content:contains=<text>` | Config content includes text | `expect=content:contains=router-id` |
| `expect=content:not-contains=<text>` | Config content must NOT include | `expect=content:not-contains=old-value` |
| `expect=content:lines=<N>` | Config content line count | `expect=content:lines=5` |
| `expect=viewport:contains=<text>` | Displayed output includes text | `expect=viewport:contains=10.0.0.1` |
| `expect=viewport:not-contains=<text>` | Displayed output must NOT include | `expect=viewport:not-contains=error` |
| `expect=errors:count=<N>` | Validation error count | `expect=errors:count=0` |
| `expect=status:contains=<text>` | Status message | `expect=status:contains=committed` |
| `expect=error:none` | No command error | `expect=error:none` |
| `expect=timer:active` | Confirm timer running | `expect=timer:active` |
| `expect=file:path=<rel>:contains=<text>` | On-disk file content | `expect=file:path=test.conf:contains=bgp` |
| `expect=file:path=<rel>:not-contains=<text>` | File must NOT contain | `expect=file:path=test.conf:not-contains=old` |
| `expect=file:path=<rel>:absent=true` | File does not exist | `expect=file:path=test.conf:absent=true` |
<!-- source: internal/component/cli/testing/expect.go -- editor expectation types -->

### Wait Actions

| Action | Purpose | Example |
|--------|---------|---------|
| `wait=ms:<N>` | Wait N milliseconds | `wait=ms:200` |
| `wait=validation` | Wait for validation | `wait=validation` |
| `wait=timer:expire` | Wait for timer expiry | `wait=timer:expire` |

### Example

```
# Test: Edit navigation
tmpfs=test.conf:terminator=EOF_CONF
bgp {
  router-id 1.2.3.4;
  peer upstream1 {
    remote {
      ip 1.1.1.1;
      as 65001;
    }
  }
}
EOF_CONF

option=file:path=test.conf

expect=context:root
input=type:text=edit bgp
input=enter
expect=context:path=bgp
expect=error:none

input=type:text=set
input=space
expect=completion:contains=router-id,local-as,peer
```

### Test Categories

| Category | Location | Tests |
|----------|----------|-------|
| Navigation | `test/editor/navigation/` | edit, up, top, context |
| Completion | `test/editor/completion/` | commands, YANG paths, values |
| Commands | `test/editor/commands/` | set, delete, show, compare |
| Lifecycle | `test/editor/lifecycle/` | commit, rollback, load, history |
| Validation | `test/editor/validation/` | hold-time, peer-as |
| Pipe | `test/editor/pipe/` | grep, head, tail |
<!-- source: internal/component/cli/testing/parser.go -- editor test file parsing -->
<!-- source: internal/component/cli/testing/session_test.go -- editor session tests -->

Full format specification: `plan/spec-editor-testing-framework.md`
