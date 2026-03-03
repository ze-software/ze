# Spec: Tmpfs Format

## Task

Implement a Virtual File System (Tmpfs) format that allows embedding multiple files in a single stream. Used by:
1. Test runner (`ze-test`) - test data with embedded configs/scripts

Normal zebgp reads config from file or stdin (`-`), no Tmpfs.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config parsing patterns
- [ ] `docs/functional-tests.md` - current test format understanding

### Source Code
- [ ] `internal/test/ci/ciformat.go` - existing .ci parser
- [ ] `internal/test/runner/record.go` - current test runner
- [ ] `internal/slogutil/slogutil.go` - getEnv to extract

### RFC Summaries
N/A - internal tooling, not BGP protocol

**Key insights:**
- Test format unification reduces maintenance burden
- Tmpfs enables self-contained test files
- Same format for tests and deployment bundles

## Unified .ci Format Reference

Single parser (`internal/test/ci/`) shared by test runner and ze-peer. Each consumer interprets the line types it handles, ignores others.

### Line Types

| Prefix | Consumer | Description |
|--------|----------|-------------|
| `stdin=` | Test runner | Embed stdin content (multi-line or inline hex/text) |
| `tmpfs=` | Test runner | Embed file in temp directory (for .py plugins) |
| `option=` | ze-peer | Configure test peer behavior |
| `cmd=` | Test runner | Commands: api, background process, foreground process |
| `expect=exit:` | Test runner | Assert exit code |
| `expect=stdout:` | Test runner | Assert stdout content |
| `expect=stderr:` | Test runner | Assert stderr content |
| `expect=json:` | Test runner | Assert JSON output (field-by-field, order-independent) |
| `expect=bgp:` | ze-peer | Expect BGP wire message |
| `action=notification:` | ze-peer | Send NOTIFICATION to peer |
| `action=send:` | ze-peer | Send raw bytes to peer |

### Stdin Block

For process orchestration, stdin content is embedded using `stdin=` blocks.

**Multi-line format** (with terminator):
```
stdin=<name>:terminator=<TERM>
<content>
<TERM>
```

**Single-line format** (inline value):
```
stdin=<name>:hex=<hex-value>
stdin=<name>:text=<text-value>
```

| Field | Description |
|-------|-------------|
| `name` | Identifier referenced by `cmd=...:stdin=<name>` |
| `terminator` | End marker for multi-line (alphanumeric + underscore, unique per file) |
| `hex` | Hex-encoded content (single-line) |
| `text` | Plain text content (single-line) |

**Multi-line example:**
```
stdin=peer:terminator=EOF_PEER
option=asn:value=65000
expect=bgp:conn=1:seq=1:hex=FFFF...
EOF_PEER

stdin=ze-bgp:terminator=EOF_CONF
peer 127.0.0.1 { ... }
EOF_CONF

cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=peer
cmd=foreground:seq=2:exec=ze bgp server -:stdin=ze-bgp:timeout=10s
```

**Single-line example (decode test):**
```
stdin=payload:hex=000000EA900F00E6...
cmd=foreground:seq=1:exec=ze-test decode --family l2vpn/evpn -:stdin=payload
expect=json:json={"type":"update",...}
```

### Tmpfs Block

Tmpfs embeds files that need to exist on disk (written to temp directory).
Use for:
- `.py` plugins (written to disk, optionally inlined as zipapp)
- Any file that must be passed by path to a command

For stdin-based input (config, test data), use `stdin=` blocks instead.

```
tmpfs=<path>[:mode=<octal>][:encoding=<type>]:terminator=<TERM>
<content>
<TERM>
```

### Option Lines

```
option=asn:value=<N>                    # ASN for OPEN message
option=bind:value=ipv6                  # Bind IPv6
option=tcp_connections:value=<N>        # Multi-connection tests
option=open:value=send-unknown-capability
option=open:value=inspect-open-message
option=open:value=send-unknown-message
option=update:value=send-default-route
```

### Command Lines

All "run something" actions use `cmd=`:

```
cmd=api:conn=<N>:seq=<N>:text=<api-command>                          # API command to zebgp
cmd=background:seq=<N>:exec=<command>:stdin=<name>                   # Background process
cmd=foreground:seq=<N>:exec=<command>:stdin=<name>[:timeout=<dur>]   # Foreground process
```

| Type | Keys | Description |
|------|------|-------------|
| `api` | `conn`, `seq`, `text` | Send API command to zebgp |
| `background` | `seq`, `exec`, `stdin` | Start background process (seq=execution order) |
| `foreground` | `seq`, `exec`, `stdin`, `timeout` | Start foreground process, wait for completion |

### Decode Test Lines

For testing BGP message decoding with full JSON validation:

```
stdin=payload:hex=<hex-payload>
cmd=foreground:seq=1:exec=ze-test decode --family <family> -:stdin=payload
expect=json:json=<expected-json>
```

| Component | Description |
|-----------|-------------|
| `stdin=payload:hex=` | Hex-encoded BGP message payload (single-line) |
| `--family <family>` | Address family: `ipv4/unicast`, `l2vpn/evpn`, etc. |
| `-` | Read payload from stdin |
| `expect=json:json=` | Expected JSON output |

**JSON Comparison:**
- Parsed and compared field-by-field (key order independent)
- Volatile fields removed before comparison: `exabgp`, `ze-bgp`, `time`, `host`, `pid`, `ppid`, `counter`
- Neighbor normalization: `peer` ↔ `neighbor` treated as equivalent, `direction` field ignored
- All non-volatile fields must match exactly

### Expectation Lines

**Test runner expectations:**
```
expect=exit:code=<N>
expect=stdout:contains=<text>
expect=stdout:regex=<pattern>
expect=stdout:validate=json
expect=stdout:validate=json:contains=<text>
expect=stdout:modifier=not:contains=<text>
expect=stderr:contains=<text>
expect=stderr:modifier=not:contains=<text>
```

**ze-peer expectations:**
```
expect=bgp:conn=<N>:seq=<N>:hex=<hex>
```

### Action Lines

```
action=notification:conn=<N>:seq=<N>:text=<shutdown-message>
action=send:conn=<N>:seq=<N>:hex=<raw-bytes>
```

### Comments and Whitespace

```
# Comments start with #
# Blank lines ignored
```

### Complete Example (Encoding Test)

```
# ze-peer stdin (test expectations)
stdin=peer:terminator=EOF_PEER
option=asn:value=65000
cmd=api:conn=1:seq=1:text=update text origin set igp as-path set [65533] nhop set 10.0.1.254 nlri ipv4/unicast add 10.0.0.0/24
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FFFD4003040A0001FE180A0000
expect=json:conn=1:seq=1:json={"meta":{"version":"1.0.0","format":"ze-bgp"},...}
cmd=api:conn=1:seq=1:text=announce eor ipv4/unicast
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000
EOF_PEER

# zebgp stdin (config)
stdin=ze-bgp:terminator=EOF_CONF
peer 127.0.0.1 {
    router-id 10.0.0.2;
    local-address 127.0.0.1;
    local-as 65533;
    peer-as 65000;
    hold-time 180;

    family {
        ipv4/unicast;
    }
    announce {
        ipv4 {
            unicast 10.0.0.0/24 next-hop 10.0.1.254 local-preference 200;
        }
    }
}
EOF_CONF

# Process orchestration
cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=peer
cmd=foreground:seq=2:exec=ze bgp server -:stdin=ze-bgp:timeout=10s
```

**Flow:**
1. Test runner parses `stdin=` blocks into memory
2. Starts `ze-peer` (seq=1, background), pipes `peer` block to stdin
3. Starts `ze-bgp` (seq=2, foreground), pipes `ze-bgp` block to stdin
4. Waits for foreground to complete (or timeout)
5. Checks ze-peer output for "successful" or failure details

### Complete Example (Plugin Test with Tmpfs)

```
# Plugin script (needs to be a file on disk)
tmpfs=plugin.py:terminator=EOF_PY
#!/usr/bin/env python3
import json, sys
print(json.dumps({"ready": True}))
sys.stdout.flush()
for line in sys.stdin:
    msg = json.loads(line)
    # process message...
EOF_PY

# ze-peer stdin (test expectations)
stdin=peer:terminator=EOF_PEER
option=asn:value=65000
expect=bgp:conn=1:seq=1:hex=FFFF...
EOF_PEER

# ze bgp config (references plugin.py from Tmpfs)
stdin=ze-bgp:terminator=EOF_CONF
peer 127.0.0.1 {
    router-id 10.0.0.2;
    local-as 65533;
    peer-as 65000;
    process plugin {
        run "./plugin.py";
    }
}
EOF_CONF

# Process orchestration
cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=peer
cmd=foreground:seq=2:exec=ze bgp server -:stdin=ze-bgp:timeout=10s
```

**Flow with Tmpfs:**
1. Test runner writes `tmpfs=` files to temp directory
2. Changes to temp directory (so `./plugin.py` resolves)
3. Starts processes with stdin= blocks piped
4. Plugin reads from Tmpfs file on disk

## Format Specification

### Tmpfs Block Syntax

```
tmpfs=<relative-path>[:mode=<octal>][:encoding=<type>]:terminator=<TERM>
<content>
<TERM>
```

Simple `tmpfs=` prefix, path is first value.

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `path` | Yes | - | Relative path (no `..`, no absolute) |
| `mode` | No | Auto | File permissions (octal: 644, 755) |
| `encoding` | No | `text` | `text` or `base64` |
| `terminator` | Yes | - | End marker (alone on line) |

### Terminator Constraints

- Must be non-empty
- **Must be unique within file** - no two Tmpfs blocks can use same terminator
- Alphanumeric and underscore only: `[A-Za-z0-9_]+`
- Matched exactly (no regex, no whitespace trimming)
- Recommended pattern: `EOF_<purpose>` (e.g., `EOF_RULES`, `EOF_CONF`, `EOF_PY`)

### Mode Defaults

| Pattern | Default |
|---------|---------|
| `*.py`, `*.sh`, `*.pl`, `*.rb`, `*.bash`, `*.zsh` | 0755 |
| Everything else | 0644 |

### Test Extension Syntax

```
# Tmpfs blocks (parsed first, create temp files)
tmpfs=plugin.py:terminator=EOF_PY
...
EOF_PY

tmpfs=peer.conf:terminator=EOF_CONF
...
EOF_CONF

# Options (ze-peer config)
option=asn:value=65533
option=bind:value=ipv6
option=tcp_connections:value=2

# Commands (test runner)
cmd=<shell-command>
cmd=mode=<mode>:seq=<N>[:stdin=<file>]:run=<command>

# Expectations - test runner
expect=exit:code=<N>
expect=stdout:contains=<text>
expect=stdout:regex=<pattern>
expect=stdout:validate=json
expect=stdout:modifier=not:contains=<text>
expect=stderr:contains=<text>
expect=stderr:modifier=not:contains=<text>

# Expectations - ze-peer
expect=bgp:conn=<N>:seq=<N>:hex=<hex>

# Actions - ze-peer
action=notification:conn=<N>:seq=<N>:text=<text>
action=send:conn=<N>:seq=<N>:hex=<hex>
```

### Line Type Consumers

| Line Type | Consumer | Purpose |
|-----------|----------|---------|
| `tmpfs=` | Test runner | Embed files in temp dir |
| `option=` | ze-peer | Configure test peer |
| `cmd=` | Test runner | Execute commands |
| `expect=exit:` | Test runner | Check exit code |
| `expect=stdout:` | Test runner | Check stdout |
| `expect=stderr:` | Test runner | Check stderr |
| `expect=bgp:` | ze-peer | Expect BGP message |
| `action=notification:` | ze-peer | Send NOTIFICATION |
| `action=send:` | ze-peer | Send raw bytes |

Shared parser in `internal/test/ci/` - consumers ignore lines they don't handle.

### Execution Environment

Test runner:
1. Creates temp directory
2. Writes Tmpfs files to temp dir
3. **chdir to temp dir**
4. Replaces `tmpfs//<path>` with `<path>` (now relative to cwd)
5. Executes programs in sequence order
6. Cleans up temp dir (kills background processes)

`tmpfs//` prefix makes Tmpfs references explicit:

```
run=ze bgp run tmpfs//peer.conf      # → ze bgp run peer.conf (in temp dir)
run=ze bgp validate tmpfs//peer.conf # → ze bgp validate peer.conf
```

### Multi-Program Orchestration

Programs run in `seq=` order. Background processes start and keep running:

```
# Tmpfs: test rules for ze-peer
tmpfs=rules.ci:terminator=EOF_RULES
option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=FFFF...
EOF_RULES

# Tmpfs: ze bgp config
tmpfs=ze.bgp.conf:terminator=EOF_CONF
peer 127.0.0.1 { ... }
EOF_CONF

# Program 1: ze-peer (background, validates BGP messages)
cmd=mode=background:seq=1:run=ze-peer --port 1790 tmpfs//rules.ci

# Program 2: zebgp (foreground, main test subject)
cmd=mode=foreground:seq=2:stdin=ze.bgp.conf:run=ze bgp server -

# Test runner expectations
expect=exit:code=0

# Cleanup: background processes killed after test
```

### Command Syntax

Two `cmd=` forms:

**Simple command** (run once, check exit):
```
cmd=<shell-command>
```

**Process orchestration** (background/foreground with sequencing):
```
cmd=mode=<mode>:seq=<N>[:stdin=<tmpfs-name>]:run=<command>
```

| Field | Values | Description |
|-------|--------|-------------|
| mode | `background`, `foreground` | Process lifecycle |
| seq | `1`, `2`, ... | Execution order (lower first) |
| stdin | Tmpfs filename | Pipe Tmpfs content to stdin (optional) |
| run | Program + args | What to execute |

All key=value pairs for consistent parsing.

### Stdin Piping

`stdin=<name>` pipes Tmpfs content directly to program - no temp file:

```
cmd=background:seq=1:stdin=peer-sink.conf:run=ze-peer -
cmd=foreground:seq=2:stdin=ze.bgp.conf:run=ze bgp server -
```

Benefits:
- No temp files for configs (when program reads stdin)
- Each program gets its own stdin
- Use `tmpfs//` only when temp file is actually needed

### Execution Flow

```
┌─────────────────────────────────────────────────────────────┐
│  1. Parse Tmpfs, write to temp dir                            │
│  2. chdir to temp dir                                       │
│  3. Start seq=1 (background) → ze-peer running           │
│  4. Start seq=2 (foreground) → ze bgp running                │
│  5. Send API commands to foreground process                 │
│  6. Validate expectations                                   │
│  7. Kill background processes                               │
│  8. Cleanup temp dir                                        │
└─────────────────────────────────────────────────────────────┘
```

### Limits

Configurable via environment variables (`.` or `_` for first two separators, `.` takes priority):

| Limit | Default | Environment Variable |
|-------|---------|---------------------|
| Max file size | 1 MB | `ze.bgp.ci.max_file_size` / `ze_bgp_ci_max_file_size` |
| Max total size | 1 MB | `ze.bgp.ci.max_total_size` / `ze_bgp_ci_max_total_size` |
| Max files | 100 | `ze.bgp.ci.max_files` / `ze_bgp_ci_max_files` |
| Max path length | 256 | `ze.bgp.ci.max_path_length` / `ze_bgp_ci_max_path_length` |
| Max path depth | 10 | `ze.bgp.ci.max_path_depth` / `ze_bgp_ci_max_path_depth` |

### Duplicate Paths

Duplicate paths are **rejected with error**. Each path must be unique within a Tmpfs.

### Security Constraints

1. **No absolute paths** - must be relative
2. **No parent traversal** - no `..` components after Clean()
3. **No hidden files** - no `.` prefix (configurable)
4. **Path escape check** - verify Join() result stays under base
5. **No symlinks** - write regular files only

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseTmpfsBlock` | `internal/tmpfs/tmpfs_test.go` | Basic block parsing | |
| `TestParseMultipleBlocks` | `internal/tmpfs/tmpfs_test.go` | Multiple files in stream | |
| `TestModeDefaults` | `internal/tmpfs/tmpfs_test.go` | Auto mode for scripts | |
| `TestModeExplicit` | `internal/tmpfs/tmpfs_test.go` | Explicit mode override | |
| `TestBase64Encoding` | `internal/tmpfs/tmpfs_test.go` | Binary file support | |
| `TestWriteTo` | `internal/tmpfs/tmpfs_test.go` | File creation in temp dir | |
| `TestCleanup` | `internal/tmpfs/tmpfs_test.go` | Temp dir removal | |
| `TestSignalCleanup` | `internal/tmpfs/tmpfs_test.go` | Cleanup on SIGINT/SIGTERM | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| File size | 0-1MB | 1048576 | N/A | 1048577 |
| Total size | 0-1MB | 1048576 | N/A | 1048577 |
| File count | 0-100 | 100 | N/A | 101 |
| Path length | 1-256 | 256 chars | 0 (empty) | 257 |
| Path depth | 1-10 | 10 levels | N/A | 11 |

### Security Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRejectAbsolutePath` | `internal/tmpfs/security_test.go` | `/etc/passwd` rejected | |
| `TestRejectParentTraversal` | `internal/tmpfs/security_test.go` | `../../../etc/passwd` rejected | |
| `TestRejectPathEscape` | `internal/tmpfs/security_test.go` | `foo/../../bar` rejected | |
| `TestRejectHiddenFiles` | `internal/tmpfs/security_test.go` | `.secret` rejected | |
| `TestRejectOversizeFile` | `internal/tmpfs/security_test.go` | >1MB rejected | |
| `TestRejectTooManyFiles` | `internal/tmpfs/security_test.go` | >100 files rejected | |

### Parser Robustness Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMalformedHeader` | `internal/tmpfs/tmpfs_test.go` | Invalid tmpfs= line rejected | |
| `TestMissingTerminator` | `internal/tmpfs/tmpfs_test.go` | EOF without terminator error | |
| `TestDuplicatePaths` | `internal/tmpfs/tmpfs_test.go` | Same path twice behavior | |
| `TestEmptyTerminator` | `internal/tmpfs/tmpfs_test.go` | Empty terminator rejected | |
| `TestTerminatorSpecialChars` | `internal/tmpfs/tmpfs_test.go` | Terminator with `:` or `=` | |
| `TestEmptyFile` | `internal/tmpfs/tmpfs_test.go` | 0-byte file allowed | |
| `TestEmptyPath` | `internal/tmpfs/tmpfs_test.go` | Empty path rejected | |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `tmpfs-basic` | `test/data/unit/tmpfs/basic.ci` | Simple config + script | |
| `tmpfs-subdirs` | `test/data/unit/tmpfs/subdirs.ci` | Nested directory structure | |
| `tmpfs-binary` | `test/data/unit/tmpfs/binary.ci` | Base64 encoded file | |

## Files to Create

- `internal/config/env/env.go` - Shared env var handling (extract from slogutil)
- `internal/config/env/env_test.go` - Tests for env handling
- `internal/tmpfs/tmpfs.go` - Core Tmpfs parser and types
- `internal/tmpfs/tmpfs_test.go` - Unit tests
- `internal/tmpfs/security_test.go` - Security boundary tests
- `internal/tmpfs/limits.go` - Constants and limit checking (uses internal/config/env)
- `internal/tmpfs/write.go` - Temp dir creation and file writing
- `internal/tmpfs/cleanup.go` - Signal handling and cleanup

## Files to Modify

- `internal/slogutil/slogutil.go` - Use internal/config/env instead of private getEnv
- `internal/test/ci/ciformat.go` - Integrate Tmpfs parsing
- `internal/test/runner/record.go` - Use Tmpfs for test execution, config rewriting

## Implementation Steps

1. **Write unit tests** - Tmpfs parsing tests BEFORE implementation
2. **Run tests** - Verify FAIL (paste output)
3. **Implement `internal/tmpfs`** - Core parser
4. **Run tests** - Verify PASS (paste output)
5. **Write security tests** - Boundary and escape tests
6. **Run tests** - Verify FAIL
7. **Implement security checks** - Validation logic
8. **Run tests** - Verify PASS
9. **Implement temp dir + cleanup** - With signal handling
10. **Integrate with test runner** - Modify ciformat.go, record.go
11. **Functional tests** - End-to-end Tmpfs tests
12. **Verify all** - `make lint && make test && make functional` (paste output)

## Script Execution Model

### Two Modes

| Mode | Config Source | Script Handling |
|------|---------------|-----------------|
| **Normal zebgp** | File or stdin | Read from filesystem |
| **Test runner (.ci)** | Tmpfs embedded | Inline via zipapp (no disk) |

### Normal zebgp Operation

```bash
# Config from file
ze bgp run peer.conf

# Config from stdin (- means stdin, standard Unix convention)
cat peer.conf | ze bgp server -

# Scripts referenced in config are read from filesystem normally
```

Config syntax unchanged:
```
process p {
    run "./plugin.py";    # Read from filesystem
}
```

### Test Runner (.ci) - Tmpfs Mode

The test runner rewrites Tmpfs-embedded Python scripts to inline execution:

1. Parse Tmpfs blocks from `.ci` file
2. For each `.py` in Tmpfs: wrap as zipapp, base64 encode
3. Rewrite config: `run "./plugin.py"` → `run "python3" "-c" "import base64..."`
4. Write only the rewritten config to temp
5. Execute zebgp with rewritten config

```go
// Test runner pseudo-code
func rewriteConfig(config string, tmpfs *Tmpfs) string {
    for path, content := range tmpfs.Files {
        if strings.HasSuffix(path, ".py") {
            zipapp := wrapAsZipapp(content)
            b64 := base64.StdEncoding.EncodeToString(zipapp)
            inlineCmd := fmt.Sprintf(`run "python3" "-c" "import base64,zipfile,io; zf=zipfile.ZipFile(io.BytesIO(base64.b64decode(b'%s'))); __builtins__['exec'](compile(zf.read('__main__.py'),'<zipapp>','exec'))"`, b64)
            config = strings.Replace(config, fmt.Sprintf(`run "./%s"`, path), inlineCmd, -1)
        }
    }
    return config
}
```

### Python Interpreter Selection

Environment variable overrides default (uses `internal/config/env`):

| `ze.bgp.path.python` / `zebgp_path_python` | Interpreter Used |
|-------------------------------------------|------------------|
| (not set) | `python3` from PATH |
| `/usr/bin/python3.11` | `/usr/bin/python3.11` |
| `python3.12` | `python3.12` from PATH |

```go
python := env.Get("path", "python")
if python == "" {
    python = "python3"
}
```

### Inline Execution Detail

```python
# .py wrapped as zipapp, passed as base64 argument
python3 -c "
import base64, sys, zipfile, io
data = base64.b64decode(b'<base64-zipapp>')
zf = zipfile.ZipFile(io.BytesIO(data))
code = compile(zf.read('__main__.py'), '<zipapp>', 'exec')
__builtins__['exec'](code)
"
```

### Benefits

1. **No temp files for Python** - scripts never touch disk
2. **Normal .py files** - no zipapp knowledge required
3. **Stdin preserved** - scripts read API messages normally
4. **Read-only filesystems** - works when /tmp unavailable

### Limitations

1. **Script size** - command line limits (~128KB Linux, ~256KB macOS)
2. **Non-Python** - shell scripts, binaries use temp file
3. **Single-file only** - multi-file packages need manual zipapp

### Multi-file Python Packages

For packages with multiple modules, user creates zipapp manually:

```bash
python3 -m zipapp mypackage/ -o plugin.py -m "mypackage:main"
```

The `.py` extension still works - zebgp detects zipapp by content (PK header).

## Test Folder Migration

### test/data/decode/*.test → test/data/decode/*.ci

**Before (3-line format):**
```
update l2vpn/evpn
000000EA900F00E600...
{ "exabgp": "5.0.0", ... }
```

**After:**
```
stdin=payload:hex=000000EA900F00E600...
cmd=foreground:seq=1:exec=ze-test decode --family l2vpn/evpn -:stdin=payload
expect=json:json={ "type": "update", "neighbor": { ... }, "announce": { ... } }
```

**JSON Validation Rules:**
- JSON comparison is **field-by-field, order-independent** (parsed, not string comparison)
- **Volatile fields ignored:** `exabgp`, `ze-bgp`, `time`, `host`, `pid`, `ppid`, `counter`
- **Neighbor normalization:** `peer` ↔ `neighbor` equivalence, `direction` field ignored
- All other fields must match exactly

### test/data/encode/*.conf + *.ci → test/data/encode/*.ci

**Before:**
```
# fast.conf (separate file)
peer 127.0.0.1 { ... }

# fast.ci (separate file)
option=file:path=fast.conf
expect=bgp:conn=1:seq=1:hex=FFFF...
```

**After (self-contained):**
```
# ze-peer rules embedded
tmpfs=rules.ci:terminator=EOF_RULES
option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=FFFF...
EOF_RULES

# ze bgp config embedded
tmpfs=fast.conf:terminator=EOF_CONF
peer 127.0.0.1 { ... }
EOF_CONF

# ze-peer validates BGP output
cmd=mode=background:seq=1:run=ze-peer --port 1790 tmpfs//rules.ci

# ze bgp runs test
cmd=mode=foreground:seq=2:stdin=fast.conf:run=ze bgp server -

expect=exit:code=0
```

### test/data/plugin/*.conf + *.ci + *.run → test/data/plugin/*.ci

**Before:**
```
# plugin.conf (separate file)
peer 127.0.0.1 {
    process p { run "./plugin.py"; }
}

# plugin.py (separate file)
#!/usr/bin/env python3
print('{"ready": true}')

# plugin.ci (separate file)
option=file:path=plugin.conf
expect=bgp:conn=1:seq=1:hex=...
```

**After (self-contained):**
```
# ze-peer rules
tmpfs=rules.ci:terminator=EOF_RULES
option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=...
EOF_RULES

# Plugin script
tmpfs=plugin.py:terminator=EOF_PY
#!/usr/bin/env python3
print('{"ready": true}')
EOF_PY

# ze bgp config
tmpfs=plugin.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    process p { run "./plugin.py"; }
}
EOF_CONF

cmd=mode=background:seq=1:run=ze-peer --port 1790 tmpfs//rules.ci
cmd=mode=foreground:seq=2:stdin=plugin.conf:run=ze bgp server -

expect=exit:code=0
```

### test/data/parse/valid/*.conf → test/data/parse/*.ci

**Before:**
```
# valid/graceful-restart.conf (file exists = test passes)
peer 127.0.0.1 {
    graceful-restart;
}
```

**After:**
```
tmpfs=graceful-restart.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    graceful-restart;
}
EOF_CONF

cmd=ze bgp validate tmpfs//graceful-restart.conf
expect=exit:code=0
```

### test/data/parse/invalid/*.conf + *.expect → test/data/parse/*.ci

**Before:**
```
# invalid/bad-config.conf
peer 127.0.0.1 {
    invalid-option;
}

# invalid/bad-config.expect
unknown option: invalid-option
```

**After:**
```
tmpfs=bad-config.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    invalid-option;
}
EOF_CONF

cmd=ze bgp validate tmpfs//bad-config.conf
expect=exit:code=1
expect=stderr:contains=unknown option: invalid-option
```

### Migration Summary

| Source | Files | Target | Status |
|--------|-------|--------|--------|
| `decode/*.test` | 18 | `test/decode/*.ci` | ✅ Done |
| `encode/*.conf + *.ci` | 42 | `test/encode/*.ci` | ✅ Done |
| `plugin/*.conf + *.ci + *.run` | 23 | `test/plugin/*.ci` | ✅ Done |
| `parse/valid/*.conf` | 10 | `test/parse/*.ci` | ✅ Done |
| `parse/invalid/*.conf + *.expect` | 2 | `test/parse/*.ci` | ✅ Done |

### ExaBGP Migration

No changes needed - ExaBGP uses `.py`, ZeBGP uses `.py`:

| ExaBGP | ZeBGP | Notes |
|--------|-------|-------|
| `run "./plugin.py"` | `run "./plugin.py"` | Same syntax |
| `run "/usr/bin/python3 ./plugin.py"` | `run "./plugin.py"` | Strip interpreter |

### Tmpfs Resolution

```
┌─────────────────────────────────────────────────────────┐
│  Config: run "./plugin.py"                              │
└─────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────┐
│  Tmpfs lookup: plugin.py found?                           │
│  ├─ No  → check filesystem                              │
│  └─ Yes → check extension                               │
└─────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────┐
│  Extension: .py (Python)                                │
│  → wrap content as zipapp (__main__.py)                 │
│  → base64 encode                                        │
│  → Execute: python3 -c "import base64..."               │
│  → Stdin connected to zebgp API pipe                    │
│  → No file written to disk                              │
└─────────────────────────────────────────────────────────┘
```

### Example

**Tmpfs Input:**
```
tmpfs=plugin.py:terminator=EOF_PY
#!/usr/bin/env python3
import json, sys
print(json.dumps({"ready": True}))
sys.stdout.flush()
for line in sys.stdin:
    msg = json.loads(line)
    # process...
EOF_PY

tmpfs=peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533;
    process p {
        run "./plugin.py";
    }
}
EOF_CONF

cmd=ze bgp run tmpfs//peer.conf
expect=exit:code=0
```

**What happens:**
1. Tmpfs parsed: `plugin.py` and `peer.conf` in memory
2. Test runner rewrites config: `run "./plugin.py"` → inline zipapp command
3. Files written to temp dir
4. chdir to temp dir
5. `tmpfs//peer.conf` → `peer.conf`
6. Execute `ze bgp run peer.conf`
7. Python executes inline via `python3 -c "import base64..."`
8. Script runs with stdin/stdout connected to zebgp
9. Cleanup temp dir

## API Design

### internal/env

```go
package env

// Get returns zebgp env var with dot/underscore support.
// Dot notation takes priority: ze.bgp.section.key > zebgp_section_key
func Get(section, key string) string

// GetInt returns int value, or default if not set/invalid.
func GetInt(section, key string, defaultVal int) int

// GetInt64 returns int64 value, or default if not set/invalid.
func GetInt64(section, key string, defaultVal int64) int64
```

### internal/tmpfs

```go
package tmpfs

import (
    "context"
    "io"
    "io/fs"
)

// Default limits for Tmpfs parsing (overridable via ze_bgp_ci_* env vars)
const (
    DefaultMaxFileSize   = 1 << 20      // 1 MB
    DefaultMaxTotalSize  = 1 << 20      // 1 MB
    DefaultMaxFiles      = 100
    DefaultMaxPathLen    = 256
    DefaultMaxPathDepth  = 10
)

// LimitsFromEnv reads limits from environment, falling back to defaults
func LimitsFromEnv() Limits

// File represents a single file in the Tmpfs
type File struct {
    Path     string
    Mode     fs.FileMode
    Content  []byte
}

// Tmpfs holds parsed virtual filesystem
type Tmpfs struct {
    Files  []*File
    Config string // from config= line (optional)
}

// Parse reads Tmpfs blocks from reader
func Parse(r io.Reader) (*Tmpfs, error)

// ParseWithLimits reads Tmpfs with custom limits
func ParseWithLimits(r io.Reader, limits Limits) (*Tmpfs, error)

// Validate checks all files for security issues
func (v *Tmpfs) Validate() error

// WriteTo creates files in directory
// Returns cleanup function
func (v *Tmpfs) WriteTo(baseDir string) error

// WriteToTemp creates temp dir, writes files, returns path and cleanup
// Cleanup is called automatically on ctx.Done() or signals
func (v *Tmpfs) WriteToTemp(ctx context.Context) (dir string, cleanup func(), err error)

// Limits configures parsing limits
type Limits struct {
    MaxFileSize  int64
    MaxTotalSize int64
    MaxFiles     int
    MaxPathLen   int
    MaxPathDepth int
}

// DefaultLimits returns standard limits
func DefaultLimits() Limits
```

## Signal Handling

```go
// WriteToTemp implementation sketch
func (v *Tmpfs) WriteToTemp(ctx context.Context) (string, func(), error) {
    dir, err := os.MkdirTemp("", "zebgp-tmpfs-*")
    if err != nil {
        return "", nil, err
    }

    var once sync.Once
    cleanup := func() {
        once.Do(func() {
            os.RemoveAll(dir)
        })
    }

    // Cleanup on context cancel
    go func() {
        <-ctx.Done()
        cleanup()
    }()

    // Cleanup on signals
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigCh
        cleanup()
        os.Exit(1)
    }()

    if err := v.WriteTo(dir); err != nil {
        cleanup()
        return "", nil, err
    }

    return dir, cleanup, nil
}
```

## Example Files

### Simple Test (test/data/unit/tmpfs/basic.ci)

```
tmpfs=peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533;
    peer-as 65533;
}
EOF_CONF

cmd=ze bgp validate tmpfs//peer.conf
expect=exit:code=0
```

### With Script (test/data/unit/tmpfs/with-script.ci)

```
tmpfs=peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533;
    peer-as 65533;
    process test {
        run "./plugin.py";
    }
}
EOF_CONF

tmpfs=plugin.py:mode=755:terminator=EOF_PY
#!/usr/bin/env python3
import json
print(json.dumps({"ready": True}))
EOF_PY

cmd=ze bgp run tmpfs//peer.conf
expect=exit:code=0
expect=stderr:modifier=not:contains=error
```

### Subdirectories (test/data/unit/tmpfs/subdirs.ci)

```
tmpfs=conf/peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533;
    process p { run "./scripts/plugin.py"; }
}
EOF_CONF

tmpfs=scripts/plugin.py:mode=755:terminator=EOF_PY
#!/usr/bin/env python3
print('{"ready": true}')
EOF_PY

cmd=ze bgp run tmpfs//conf/peer.conf
expect=exit:code=0
```

## Implementation Status

### Completed ✅
- `internal/tmpfs/tmpfs.go` - Tmpfs parsing (tmpfs= blocks)
- `internal/tmpfs/tmpfs.go` - Stdin parsing (stdin= blocks, multi-line with terminator)
- `internal/tmpfs/tmpfs.go` - Single-line stdin parsing (`stdin=<name>:hex=<value>` and `stdin=<name>:text=<value>`)
- `internal/test/runner/record.go` - RunCommand struct, parseCmd for background/foreground
- `internal/test/runner/record.go` - FailType constants (fixed goconst lint)
- `internal/test/runner/runner.go` - runOrchestrated() for new format execution
- `internal/test/runner/runner.go` - Permanent debug logging via slogutil
- `test/decode/*.ci` - 18 decode tests migrated to unified format
- `test/encode/*.ci` - 42 encode tests migrated to unified format
- `test/parse/*.ci` - 12 parse tests migrated to unified format
- `test/plugin/*.ci` - 23 plugin tests migrated to unified format
- `docs/architecture/testing/ci-format.md` - Format documentation (stdin=, foreground/background commands)
- `docs/functional-tests.md` - Updated with decode tests section

### Future Enhancement (Optional)
- Python zipapp inlining for Tmpfs .py files - inline scripts as base64 zipapp to avoid temp files

## Migration Path

### Phase 1: Core Infrastructure ✅
- `internal/tmpfs` package with tmpfs= and stdin= parsing
- Test runner integration

### Phase 2: Decode Tests ✅
- 18 tests in `test/decode/*.ci`
- Format: `stdin=payload:hex=<hex>` + `cmd=foreground:exec=ze-test decode --family <family> -:stdin=payload`
- All tests passing

### Phase 3: Encode Tests ✅
- 42 tests in `test/encode/*.ci`
- Format: `stdin=peer` + `stdin=zebgp` + `cmd=background/foreground`
- All tests passing

### Phase 4: Plugin Tests ✅
- 23 tests in `test/plugin/*.ci`
- Format: `stdin=peer` + `tmpfs=<script>.run` + `stdin=zebgp` + `cmd=background/foreground`
- Python scripts use shared `ze_bgp_api.py` via PYTHONPATH
- All tests passing

### Phase 5: Parse Tests ✅
- 12 tests in `test/parse/*.ci`
- Format: `stdin=config` + `cmd=foreground:exec=ze bgp validate -`
- All tests passing

### Phase 6: Cleanup ✅
- Deleted `test/data/parse/` (superseded by `test/parse/`)
- Deleted `test/data/plugin/` (superseded by `test/plugin/`)
- Moved `test/data/scripts/` to `test/scripts/`
- Moved `test/data/migrate/` to `test/exabgp/`
- Deleted empty `test/data/` directory

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all limits

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] API documented in code comments

### Completion
- [ ] `docs/functional-tests.md` updated with unified .ci format
- [ ] `docs/architecture/testing/ci-format.md` created (Tmpfs + line types)
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-tmpfs-format.md`
- [ ] All files committed together
