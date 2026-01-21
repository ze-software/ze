# Spec: ci-format-cleanup

## Task

1. Refactor .ci format for consistency: fully key=value syntax
2. Reduce functional test output verbosity to minimize token usage

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - [current .ci format]

### Source Files
- [ ] `internal/test/runner/record.go` - [main .ci parsing for test runner]
- [ ] `internal/test/runner/runner.go` - [test execution, output]
- [ ] `internal/test/runner/display.go` - [progress display]
- [ ] `internal/test/peer/peer.go:LoadExpectFile()` - [testpeer .ci parsing for options and expects]

## New Format

### Structure
```
<action>=<type>:<key>=<value>:<key>=<value>:...
```

### Sequence Semantics
- `seq` is per-connection (conn=1:seq=1, conn=1:seq=2, conn=2:seq=1)
- Same seq number = order unknown, accept any matching message
- Different seq = strict ordering within connection

### JSON/BGP Relationship
- `expect=bgp:conn=1:seq=1:hex=...` = expected wire bytes
- `expect=json:conn=1:seq=1:json=...` = expected JSON when those bytes are decoded
- Same conn/seq = same message, different representation
- JSON validates the decode path (hex → struct → JSON)

### Full Specification

```
# Options
option=file:path=test.conf
option=env:var=ze.bgp.log.server:value=debug
option=timeout:value=10s
option=asn:value=65000
option=bind:value=ipv6

# BGP expectations (ordered by seq within conn)
expect=bgp:conn=1:seq=1:hex=FFFF...
expect=bgp:conn=1:seq=2:hex=FFFF...
expect=json:conn=1:seq=1:json={...}

# Logging expectations (unordered)
expect=syslog:pattern=subsystem=server
expect=stderr:pattern=level=DEBUG
reject=stderr:pattern=level=ERROR
reject=syslog:pattern=fatal

# Actions (test peer does something)
action=notification:conn=1:seq=1:text=session ending

# API command documentation
cmd=api:conn=1:seq=1:text=update text nhop set 1.2.3.4...
```

### Migration Table

| Old | New |
|-----|-----|
| `option:file:test.conf` | `option=file:path=test.conf` |
| `option:env:ze.bgp.log.server=debug` | `option=env:var=ze.bgp.log.server:value=debug` |
| `option:timeout:10s` | `option=timeout:value=10s` |
| `option:asn:65000` | `option=asn:value=65000` |
| `option:bind:ipv6` | `option=bind:value=ipv6` |
| `option:tcp_connections:2` | `option=tcp_connections:value=2` |
| `option:open:send-unknown-capability` | `option=open:value=send-unknown-capability` |
| `option:open:inspect-open-message` | `option=open:value=inspect-open-message` |
| `option:open:send-unknown-message` | `option=open:value=send-unknown-message` |
| `option:update:send-default-route` | `option=update:value=send-default-route` |
| `1:raw:FFFF...` | `expect=bgp:conn=1:seq=1:hex=FFFF...` |
| `A1:raw:FFFF...` | `expect=bgp:conn=1:seq=1:hex=FFFF...` |
| `B2:raw:FFFF...` | `expect=bgp:conn=2:seq=2:hex=FFFF...` |
| `1:json:{...}` | `expect=json:conn=1:seq=1:json={...}` |
| `1:cmd:update...` | `cmd=api:conn=1:seq=1:text=update...` |
| `expect:syslog:pattern` | `expect=syslog:pattern=pattern` |
| `expect:stderr:pattern` | `expect=stderr:pattern=pattern` |
| `reject:stderr:pattern` | `reject=stderr:pattern=pattern` |
| (new) | `reject=syslog:pattern=pattern` |
| `A1:notification:bye` | `action=notification:conn=1:seq=1:text=bye` |

## Output Verbosity

### Current Output (10+ lines per test type)
```
ports: 1790-1812 (23 tests)
building... ready
passed 23
═══════════════════════════════════════════════════════════════════════════════
TEST SUMMARY
═══════════════════════════════════════════════════════════════════════════════
passed    23
═══════════════════════════════════════════════════════════════════════════════
Total: 23 test(s) run, 100.0% passed
```

### Proposed Quiet Mode (default)

**Success (1 line):**
```
✓ 23 passed
```

**Failure (1 line + failure detail):**
```
✗ 22 passed, 1 failed
[detailed failure report for failed test only]
```

### Mode Flags
| Flag | Behavior |
|------|----------|
| (default) | Quiet: 1-line summary, failure details only |
| `-v` | Verbose: current behavior (progress, all details) |
| `-q` | Silent: exit code only (for scripts) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseCIExpectBGP` | `internal/test/runner/record_test.go` | `expect=bgp:conn=1:seq=1:hex=FF...` | |
| `TestParseCIExpectJSON` | `internal/test/runner/record_test.go` | `expect=json:conn=1:seq=1:json={...}` | |
| `TestParseCIOptionEnv` | `internal/test/runner/record_test.go` | `option=env:var=X:value=Y` | |
| `TestParseCIOptionFile` | `internal/test/runner/record_test.go` | `option=file:path=test.conf` | |
| `TestParseCIMultiConn` | `internal/test/runner/record_test.go` | conn=1, conn=2 sequencing | |
| `TestParseCISameSeq` | `internal/test/runner/record_test.go` | Same seq = unordered within conn | |
| `TestParseCIActionNotification` | `internal/test/runner/record_test.go` | `action=notification:conn=1:seq=1:text=...` | |
| `TestParseCICmdAPI` | `internal/test/runner/record_test.go` | `cmd=api:conn=1:seq=1:text=...` | |
| `TestParseCIRejectSyslog` | `internal/test/runner/record_test.go` | `reject=syslog:pattern=...` | |
| `TestParseCIInvalidFormat` | `internal/test/runner/record_test.go` | Error on old format lines | |
| `TestParseCIMissingConn` | `internal/test/runner/record_test.go` | Error when conn missing | |
| `TestParseCIMissingSeq` | `internal/test/runner/record_test.go` | Error when seq missing for bgp | |
| `TestPeerLoadExpectNewFormat` | `internal/test/peer/peer_test.go` | testpeer parses new options | |

### Error Messages
Old format lines should produce helpful errors:
```
Unknown line format "1:raw:FFFF..." - use "expect=bgp:conn=1:seq=1:hex=FFFF..."
Unknown line format "option:file:test.conf" - use "option=file:path=test.conf"
```

## Files to Modify

### Phase 1: Parser (new format only - hard switch)
- `internal/test/runner/record.go` - Parse new format, error on old
- `internal/test/peer/peer.go:LoadExpectFile()` - Parse new format, error on old

### Phase 2: Migrate .ci files (script)
Migration script in Go (consistent with project):
```bash
go run ./test/cmd/migrate-ci ./test/data/encode/*.ci
go run ./test/cmd/migrate-ci ./test/data/plugin/*.ci
go run ./test/cmd/migrate-ci ./test/data/api/*.ci
```

Files:
- `test/cmd/migrate-ci/main.go` - Migration tool

### Phase 3: Reduce output verbosity
- `internal/test/runner/display.go`
- `internal/test/runner/runner.go`
- `test/cmd/functional/main.go` - Add -v/-q flags

### Phase 4: Documentation
- `docs/functional-tests.md` - Update format documentation
- `test/cmd/functional/main.go` - Add --help with format reference

### --help Output
```
go run ./test/cmd/functional --help

...
.ci Format Reference:
  option=file:path=<file>           Config file to use
  option=env:var=<name>:value=<v>   Set environment variable
  option=timeout:value=<duration>   Test timeout (e.g., 10s)
  option=asn:value=<n>              Override peer ASN
  option=bind:value=<ipv4|ipv6>     Bind option
  option=tcp_connections:value=<n>  Number of TCP connections
  option=open:value=<flag>          OPEN message flags:
                                      send-unknown-capability
                                      inspect-open-message
                                      send-unknown-message
  option=update:value=<flag>        UPDATE message flags:
                                      send-default-route

  expect=bgp:conn=<n>:seq=<n>:hex=<hex>    Expected BGP wire bytes
  expect=json:conn=<n>:seq=<n>:json=<obj>  Expected JSON decode
  expect=stderr:pattern=<regex>            Must appear in stderr
  expect=syslog:pattern=<regex>            Must appear in syslog
  reject=stderr:pattern=<regex>            Must NOT appear in stderr
  reject=syslog:pattern=<regex>            Must NOT appear in syslog

  action=notification:conn=<n>:seq=<n>:text=<msg>  Send notification

  cmd=api:conn=<n>:seq=<n>:text=<cmd>      API command documentation
```

## Implementation Steps

1. **Write unit tests** - Test new format parsing (13 test cases)
2. **Run tests** - Verify FAIL
3. **Implement parser** - New format only, helpful errors for old format
4. **Run tests** - Verify PASS
5. **Write migration tool** - `test/cmd/migrate-ci/main.go`
6. **Migrate .ci files** - Run migration tool
7. **Run functional tests** - Verify all pass
8. **Audit output** - Identify verbose sources
9. **Reduce output** - Implement quiet default, -v/-q flags
10. **Add --help** - Format reference documentation
11. **Verify all** - `make lint && make test && make functional`

## Checklist

### 🧪 TDD
- [ ] Tests written (13 unit tests)
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Completion
- [ ] Migration tool created
- [ ] All .ci files migrated
- [ ] --help documents format
- [ ] Documentation updated
- [ ] Output verbosity reduced
- [ ] Spec moved to done/
