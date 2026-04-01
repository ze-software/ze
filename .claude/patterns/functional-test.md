# Pattern: Functional Test (.ci)

Structural template for adding functional tests to Ze.
Full format: `docs/architecture/testing/ci-format.md`. Rules: `rules/testing.md`.

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
| `test/hub/` | Hub daemon | Hub lifecycle |
| `test/perf/` | Performance | Benchmarks, timing |
| `test/exabgp/` | ExaBGP compat | Migration, format compat |

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
make ze-verify              # Everything except fuzz (development)
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
