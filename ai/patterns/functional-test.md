# Pattern: Functional Test (.ci)

Structural template for adding functional tests to Ze.
Full format: `docs/architecture/testing/ci-format.md`. Rules: `ai/rules/testing.md`.

## Also Read

| Rule | When it applies |
|------|----------------|
| `ai/rules/testing.md` (Editor Tests) | TUI/editor testing uses `.et` format, not `.ci` |
| `ai/rules/testing.md` (Observer-Exit Antipattern) | Python observers in `.ci` MUST use `runtime_fail`, not `sys.exit(1)` |
| `ai/rules/testing.md` | Test-first: write the test before the feature code |
| `ai/rules/completion.md` | Every user-facing feature needs a `.ci` test |
| Full navigation: `ai/INDEX.md` | |

## Test Directories

| Directory | Purpose | When to use |
|-----------|---------|-------------|
| `test/plugin/` | Plugin behavior, API commands, RPC | New plugin feature, API command, event handling |
| `test/parse/` | Config parsing, CLI commands | New config option, CLI subcommand |
| `test/encode/` | Wire encoding verification | Config with routes -> verify hex output |
| `test/decode/` | Wire decoding verification | Hex input -> verify JSON output |
| `test/ui/` | CLI UI, interactive commands | CLI subcommand output, help text |
| `test/editor/` | Editor testing (.et format) | Config editor navigation, completion, commands |
| `test/web/` | Web interface | HTTP endpoints, HTMX responses |
| `test/reload/` | Config reload via SIGHUP | Config change + SIGHUP -> behavior change |
| `test/managed/` | Fleet management | Managed config, ZeFS operations |
| `test/integration/` | Multi-component | Cross-subsystem behavior |
| `test/interop/` | Interoperability | Cross-daemon testing |
| `test/chaos-web/` | Chaos dashboard | Simulator web UI |
| `test/perf/` | Performance | Benchmarks, timing |
| `test/exabgp-compat/` | ExaBGP compat | Migration, format compat |
| `test/firewall/` | Firewall/nftables | Firewall rules, NAT |
| `test/policy/` | Policy routing | Route policy, redistribution |
| `test/l2tp/` | L2TP daemon | L2TP tunnel/session lifecycle |
| `test/l2tp-wire/` | L2TP wire format | AVP encoding, control messages |
| `test/l2tp-interop/` | L2TP interop | Cross-daemon L2TP (xl2tpd) |
| `test/traffic/` | Traffic control | QoS, shaping, VPP traffic |
| `test/vpp/` | VPP backend | VPP FIB, interfaces |
| `test/static/` | Static routes | Static route installation |
| `test/ipsec/` | IPsec/IKE | IKEv2 sessions, SAs |
| `test/pppoe-interop/` | PPPoE interop | Ze PPPoE client vs accel-ppp AC |
| `test/stress/` | Stress testing | High-volume UPDATE streams |

## Runner Commands

All `ze-test` suites use the same selection contract after the suite name:
`--list`, `--all`, `--start N`, `--pattern TEXT`, or positional `N...`.
`--list` prints `N/TOTAL id name` with one-based ids; runs print one completion
line per test plus periodic progress.

| Directory | Runner command | Make target |
|-----------|---------------|-------------|
| `test/encode/` | `ze-test bgp encode [--all|--start N|N...]` | `make ze-functional-encode-test` |
| `test/plugin/` | `ze-test bgp plugin [--all|--start N|N...]` | `make ze-functional-plugin-test` |
| `test/decode/` | `ze-test bgp decode [--all|--start N|N...]` | `make ze-functional-decode-test` |
| `test/parse/` | `ze-test bgp parse [--all|--start N|N...]` | `make ze-functional-parse-test` |
| `test/reload/` | `ze-test bgp reload [--all|--start N|N...]` | `make ze-functional-reload-test` |
| `test/ui/` | `ze-test ui [--all|--start N|N...]` | `make ze-functional-ui-test` |
| `test/editor/` | `ze-test editor [--all|--start N|N...]` | `make ze-functional-editor-test` |
| `test/web/` | `ze-test web [--all|--start N|N...]` | `make ze-functional-web-test` |
| `test/managed/` | `ze-test managed [--all|--start N|N...]` | `make ze-functional-managed-test` |
| `test/l2tp/` | `ze-test l2tp [--all|--start N|N...]` | `make ze-functional-l2tp-test` |
| `test/firewall/` | `ze-test firewall [--all|--start N|N...]` | `make ze-functional-firewall-test` |
| `test/policy/` | `ze-test policy [--all|--start N|N...]` | `make ze-functional-policy-test` |
| `test/static/` | `ze-test static [--all|--start N|N...]` | `make ze-functional-static-test` |
| `test/traffic/` | `ze-test traffic [--all|--start N|N...]` | `make ze-functional-traffic-test` |
| `test/flow-export/` | `ze-test flow-export [--all|--start N|N...]` | `make ze-functional-flow-export-test` |
| `test/vpp/` | `ze-test vpp [--all|--start N|N...]` | `make ze-functional-vpp-test` |
| `test/l2tp-wire/` | `ze-test l2tp-wire [--all|--start N|N...]` | `make ze-functional-l2tp-wire-test` |
| `test/exabgp-compat/` | `ze-test exabgp [--all|--start N|N...]` | `make ze-functional-exabgp-test` |

Gated suites (in `make ze-functional-test`): encode, plugin, parse, decode, reload,
ui, editor, managed, l2tp, firewall, policy, web, install. Non-gated suites
(run manually): static, traffic, flow-export, vpp, l2tp-wire, chaos, chaos-web, exabgp.

## .ci File Structure

```
# Comment describing the test

# 1. Embedded files (config, scripts)
tmpfs=<path>[:mode=<octal>]:terminator=<TERM>
<content>
<TERM>

# 2. Stdin blocks (for process pipes)
stdin=<name>:terminator=<TERM>
<content>
<TERM>

# 3. Test options
option=file:path=<config-file>
option=asn:value=<N>

# 4. Commands and expectations (interleaved)
cmd=api:conn=1:seq=1:text=<command>
expect=bgp:conn=1:seq=1:hex=<hex>
expect=json:conn=1:seq=1:json=<json>

# 5. Actions (notifications, signals)
action=notification:conn=1:seq=1:text=<message>
```

## Minimal Plugin Test Template

```
# Test: <describe what is being tested>

# Config with the feature under test
stdin=peer:terminator=EOF_PEER
cmd=api:conn=1:seq=1:text=<api-command>
expect=bgp:conn=1:seq=1:hex=<expected-wire-bytes>
EOF_PEER

# Ze config
tmpfs=test.conf:terminator=EOF_CONF
peer test-peer {
    remote {
        ip 127.0.0.1;
        as 65533;
    }
    router-id 10.0.0.2;
    local-address 127.0.0.1;
    local-as 65533;
    hold-time 180;

    family {
        ipv4/unicast;
    }
}
EOF_CONF

option=file:path=test.conf
option=asn:value=65533
```

### An observer plugin answers callbacks before it polls

Ze asks a filter plugin for its verdict over the callback connection, and only
`api.read_line()` answers that request. An observer parked in a dispatch RPC,
which is what every `wait_*` poll does, leaves the request unanswered until the
reactor's IPC deadline expires. The filter's `on-error` setting then decides the
route, and the filter under test never runs. Pump the callbacks first, then poll.
`test/plugin/redistribution-import-accept.ci` carries the loop.

Two more rules travel with a barrier in an observer.

| Rule | Why |
|------|-----|
| A per-peer counter is a LIFETIME total, so wait for `base + N`, or for a threshold that accounts for what establishment already counted | `updates-sent` counts the initial-sync End-of-RIB as well, because the sent branch of `onMessageReceived` (`internal/component/bgp/reactor/reactor_notify.go`) sees only `msgtype.TypeUPDATE`. A threshold of 1 is reached before any route is sent |
| `quiesce()` and `wait_for_ack()` are barriers for routes AFTER establishment, never for establishment itself | The quiescer skips a peer that has not started its initial sync, so it returns at once while the session is still opening. Use `wait_peer_eor_sent()` for that window |

<!-- source: test/scripts/ze_api.py -- read_line, wait_peer_counter, wait_peer_eor_sent -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- IncrUpdatesSent on the sent branch -->

## Minimal CLI Test Template

```
# Test: ze <domain> <subcommand> produces correct output

tmpfs=input.conf:terminator=EOF_CONF
<config content>
EOF_CONF

cmd=foreground:seq=1:exec=ze <domain> <subcommand> input.conf
expect=exit:code=0
expect=stdout:contains=<expected output>
```

## Minimal Decode Test Template

```
# Test: decode <family> produces correct JSON

stdin=payload:hex=<full-bgp-message-hex>
cmd=foreground:seq=1:exec=ze-test decode --family <afi/safi> -:stdin=payload
expect=json:json=<expected-json>
```

## Key Syntax Reference

### Commands

| Syntax | Purpose |
|--------|---------|
| `cmd=api:conn=N:seq=N:text=<cmd>` | Send API command to peer connection |
| `cmd=foreground:seq=N:exec=<cmd>` | Run process, wait for completion |
| `cmd=background:seq=N:exec=<cmd>` | Run process in background |

### Expectations

| Syntax | Purpose |
|--------|---------|
| `expect=bgp:conn=N:seq=N:hex=<hex>` | Exact BGP wire message match |
| `expect=json:conn=N:seq=N:json=<obj>` | JSON field-by-field match (order-independent) |
| `expect=exit:code=N` | Foreground process exit code |
| `expect=stdout:contains=<text>` | Stdout substring match |
| `expect=stderr:contains=<text>` | Stderr substring match |
| `expect=stderr:pattern=<regex>` | Stderr regex match |
| `expect=syslog:pattern=<regex>` | Syslog regex match |
| `reject=stderr:pattern=<regex>` | Fail if stderr matches |
| `reject=bgp:conn=N:pattern=<hex>` | Fail if connection N of a check-mode ze-peer receives these wire bytes. Goes in the peer block, and needs an `expect=bgp:conn=N` in the same block to deliver something the rejection is measured against. That delivery is necessary and not sufficient. Send it LAST on that connection, or add `option=linger:value=true`. Otherwise the peer stops reading before a leak can arrive |

A line in a ze-peer stdin block that neither ze-peer nor the runner acts on
fails the file at parse time (`docs/architecture/testing/ci-format.md`, "What a
ze-peer block may carry"). `option=env` is refused there and belongs outside.

### HTTP Checks

| Syntax | Purpose |
|--------|---------|
| `http=get:seq=N:url=<url>:status=<code>` | GET assertion (`$PORT`/`$PORT2` substituted) |
| `http=post:seq=N:url=<url>:status=<code>:sendfile=<file>` | POST assertion, body from file |
| `http=wait:seq=N:url=<url>:status=<code>[:timeout=<dur>]` | Readiness poll that runs before the assertions |
| `...:contains=<text>` | Response body substring |
| `...:bodyfile=<file>` | Response body exact match against a file |
| `...:content-type=<type>` | Request body type for `sendfile` (default `application/json`) |
| `...:header=<Name>: <Value>` | Request header, HTTP wire form. **Repeatable** -- the only key allowed more than once per line. Splits on the first colon (so a value can contain colons), trims whitespace, overrides the `sendfile` default `Content-Type`, and a value with no colon is a parse error. Works on `get`, `post`, and `wait` |
| `...:insecure-tls=true` | Accept a self-signed local HTTPS certificate |

Full reference: `docs/architecture/testing/ci-format.md` (HTTP Checks).

### Actions

| Syntax | Purpose |
|--------|---------|
| `action=notification:conn=N:seq=N:text=<msg>` | Send NOTIFICATION |
| `action=send:conn=N:seq=N:hex=<hex>` | Send raw bytes |
| `action=rewrite:conn=N:seq=N:source=<f>:dest=<f>` | Replace config file |
| `action=sighup:conn=N:seq=N` | Send SIGHUP to daemon |
| `action=sigterm:conn=N:seq=N` | Send SIGTERM to daemon |

### Options

| Syntax | Purpose |
|--------|---------|
| `option=file:path=<name>` | Config file to use |
| `option=asn:value=N` | Override peer ASN |
| `option=timeout:value=<dur>` | Test timeout (e.g., `30s`) |
| `option=tcp_connections:value=N` | Number of TCP connections |
| `option=env:var=KEY:value=VAL` | Set environment variable |
| `option=open:value=drop-capability:code=N` | Remove capability from peer OPEN |
| `option=open:value=add-capability:code=N:hex=<val>` | Add capability to peer OPEN |
| `option=exclusive:group=<name>` | Never run concurrently with another test in the same group (unrelated tests still run alongside). For tests contending on a kernel-global surface that unique names cannot partition -- see `docs/architecture/testing/ci-format.md` |

## Naming Convention

Tests are named descriptively with kebab-case: `<feature>-<scenario>.ci`

| Pattern | Example |
|---------|---------|
| Feature test | `api-peer-add.ci` |
| Behavior test | `graceful-restart-flush.ci` |
| Edge case | `addpath-duplicate-route.ci` |
| Error case | `config-unknown-key.ci` |

## JSON Comparison Rules

- Field order independent
- Volatile fields auto-removed: `exabgp`, `ze-bgp`, `time`, `host`, `pid`, `ppid`, `counter`
- `peer` and `neighbor` treated as equivalent
- `direction` field ignored
- All non-volatile fields must match exactly

## Running Tests

```bash
make ze-functional-test     # All functional tests
make ze-unit-test           # Unit tests only
make ze-precommit-verify              # Everything except fuzz (two-pass: cached + race-on-changed)
```

## Checklist

```
[ ] Test in correct directory (test/<category>/)
[ ] Descriptive filename with kebab-case
[ ] Config with minimal required options (remote IP, AS, local-as, family)
[ ] Expectations verify BEHAVIOR not just absence of errors
[ ] If testing wire output: exact hex match via expect=bgp
[ ] If testing JSON: expect=json with all non-volatile fields
[ ] If testing CLI: expect=exit:code + expect=stdout:contains
[ ] If testing error: expect=stderr:contains or pattern
[ ] Test runs successfully with make ze-functional-test
```
