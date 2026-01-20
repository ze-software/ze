# Spec: VFS Format

## Task

Implement a Virtual File System (VFS) format that allows embedding multiple files in a single stream. Used by:
1. Test runner (`zebgp-test`) - test data with embedded configs/scripts

Normal zebgp reads config from file or stdin (`-`), no VFS.

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
- VFS enables self-contained test files
- Same format for tests and deployment bundles

## Unified .ci Format Reference

Single parser (`internal/test/ci/`) shared by test runner and zebgp-peer. Each consumer interprets the line types it handles, ignores others.

### Line Types

| Prefix | Consumer | Description |
|--------|----------|-------------|
| `vfs=` | Test runner | Embed file in temp directory |
| `option=` | zebgp-peer | Configure test peer behavior |
| `cmd=` | Test runner | Execute shell command |
| `expect=exit:` | Test runner | Assert exit code |
| `expect=stdout:` | Test runner | Assert stdout content |
| `expect=stderr:` | Test runner | Assert stderr content |
| `expect=bgp:` | zebgp-peer | Expect BGP wire message |
| `action=notification:` | zebgp-peer | Send NOTIFICATION to peer |
| `action=send:` | zebgp-peer | Send raw bytes to peer |

### VFS Block

```
vfs=<path>[:mode=<octal>][:encoding=<type>]:terminator=<TERM>
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

```
cmd=<shell-command>                              # Simple command
cmd=mode=background:seq=<N>:run=<command>        # Background process
cmd=mode=foreground:seq=<N>:run=<command>        # Foreground process
cmd=mode=<mode>:seq=<N>:stdin=<file>:run=<cmd>   # With stdin from VFS
```

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

**zebgp-peer expectations:**
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

### Complete Example

```
# Embed test peer rules
vfs=rules.ci:terminator=EOF_RULES
option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304
action=notification:conn=1:seq=1:text=test complete
EOF_RULES

# Embed zebgp config
vfs=peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533;
    peer-as 65533;
}
EOF_CONF

# Start zebgp-peer in background (validates BGP messages)
cmd=mode=background:seq=1:run=zebgp-peer --port 1790 vfs//rules.ci

# Start zebgp in foreground (test subject)
cmd=mode=foreground:seq=2:stdin=peer.conf:run=zebgp run -

# Test runner validates exit
expect=exit:code=0
```

## Format Specification

### VFS Block Syntax

```
vfs=<relative-path>[:mode=<octal>][:encoding=<type>]:terminator=<TERM>
<content>
<TERM>
```

Simple `vfs=` prefix, path is first value.

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `path` | Yes | - | Relative path (no `..`, no absolute) |
| `mode` | No | Auto | File permissions (octal: 644, 755) |
| `encoding` | No | `text` | `text` or `base64` |
| `terminator` | Yes | - | End marker (alone on line) |

### Terminator Constraints

- Must be non-empty
- **Must be unique within file** - no two VFS blocks can use same terminator
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
# VFS blocks (parsed first, create temp files)
vfs=plugin.py:terminator=EOF_PY
...
EOF_PY

vfs=peer.conf:terminator=EOF_CONF
...
EOF_CONF

# Options (zebgp-peer config)
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

# Expectations - zebgp-peer
expect=bgp:conn=<N>:seq=<N>:hex=<hex>

# Actions - zebgp-peer
action=notification:conn=<N>:seq=<N>:text=<text>
action=send:conn=<N>:seq=<N>:hex=<hex>
```

### Line Type Consumers

| Line Type | Consumer | Purpose |
|-----------|----------|---------|
| `vfs=` | Test runner | Embed files in temp dir |
| `option=` | zebgp-peer | Configure test peer |
| `cmd=` | Test runner | Execute commands |
| `expect=exit:` | Test runner | Check exit code |
| `expect=stdout:` | Test runner | Check stdout |
| `expect=stderr:` | Test runner | Check stderr |
| `expect=bgp:` | zebgp-peer | Expect BGP message |
| `action=notification:` | zebgp-peer | Send NOTIFICATION |
| `action=send:` | zebgp-peer | Send raw bytes |

Shared parser in `internal/test/ci/` - consumers ignore lines they don't handle.

### Execution Environment

Test runner:
1. Creates temp directory
2. Writes VFS files to temp dir
3. **chdir to temp dir**
4. Replaces `vfs//<path>` with `<path>` (now relative to cwd)
5. Executes programs in sequence order
6. Cleans up temp dir (kills background processes)

`vfs//` prefix makes VFS references explicit:

```
run=zebgp run vfs//peer.conf      # → zebgp run peer.conf (in temp dir)
run=zebgp validate vfs//peer.conf # → zebgp validate peer.conf
```

### Multi-Program Orchestration

Programs run in `seq=` order. Background processes start and keep running:

```
# VFS: test rules for zebgp-peer
vfs=rules.ci:terminator=EOF_RULES
option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=FFFF...
EOF_RULES

# VFS: zebgp config
vfs=zebgp.conf:terminator=EOF_CONF
peer 127.0.0.1 { ... }
EOF_CONF

# Program 1: zebgp-peer (background, validates BGP messages)
cmd=mode=background:seq=1:run=zebgp-peer --port 1790 vfs//rules.ci

# Program 2: zebgp (foreground, main test subject)
cmd=mode=foreground:seq=2:stdin=zebgp.conf:run=zebgp run -

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
cmd=mode=<mode>:seq=<N>[:stdin=<vfs-name>]:run=<command>
```

| Field | Values | Description |
|-------|--------|-------------|
| mode | `background`, `foreground` | Process lifecycle |
| seq | `1`, `2`, ... | Execution order (lower first) |
| stdin | VFS filename | Pipe VFS content to stdin (optional) |
| run | Program + args | What to execute |

All key=value pairs for consistent parsing.

### Stdin Piping

`stdin=<name>` pipes VFS content directly to program - no temp file:

```
cmd=background:seq=1:stdin=peer-sink.conf:run=zebgp-peer -
cmd=foreground:seq=2:stdin=zebgp.conf:run=zebgp run -
```

Benefits:
- No temp files for configs (when program reads stdin)
- Each program gets its own stdin
- Use `vfs//` only when temp file is actually needed

### Execution Flow

```
┌─────────────────────────────────────────────────────────────┐
│  1. Parse VFS, write to temp dir                            │
│  2. chdir to temp dir                                       │
│  3. Start seq=1 (background) → zebgp-peer running           │
│  4. Start seq=2 (foreground) → zebgp running                │
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
| Max file size | 1 MB | `zebgp.ci.max_file_size` / `zebgp_ci_max_file_size` |
| Max total size | 1 MB | `zebgp.ci.max_total_size` / `zebgp_ci_max_total_size` |
| Max files | 100 | `zebgp.ci.max_files` / `zebgp_ci_max_files` |
| Max path length | 256 | `zebgp.ci.max_path_length` / `zebgp_ci_max_path_length` |
| Max path depth | 10 | `zebgp.ci.max_path_depth` / `zebgp_ci_max_path_depth` |

### Duplicate Paths

Duplicate paths are **rejected with error**. Each path must be unique within a VFS.

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
| `TestParseVFSBlock` | `internal/vfs/vfs_test.go` | Basic block parsing | |
| `TestParseMultipleBlocks` | `internal/vfs/vfs_test.go` | Multiple files in stream | |
| `TestModeDefaults` | `internal/vfs/vfs_test.go` | Auto mode for scripts | |
| `TestModeExplicit` | `internal/vfs/vfs_test.go` | Explicit mode override | |
| `TestBase64Encoding` | `internal/vfs/vfs_test.go` | Binary file support | |
| `TestWriteTo` | `internal/vfs/vfs_test.go` | File creation in temp dir | |
| `TestCleanup` | `internal/vfs/vfs_test.go` | Temp dir removal | |
| `TestSignalCleanup` | `internal/vfs/vfs_test.go` | Cleanup on SIGINT/SIGTERM | |

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
| `TestRejectAbsolutePath` | `internal/vfs/security_test.go` | `/etc/passwd` rejected | |
| `TestRejectParentTraversal` | `internal/vfs/security_test.go` | `../../../etc/passwd` rejected | |
| `TestRejectPathEscape` | `internal/vfs/security_test.go` | `foo/../../bar` rejected | |
| `TestRejectHiddenFiles` | `internal/vfs/security_test.go` | `.secret` rejected | |
| `TestRejectOversizeFile` | `internal/vfs/security_test.go` | >1MB rejected | |
| `TestRejectTooManyFiles` | `internal/vfs/security_test.go` | >100 files rejected | |

### Parser Robustness Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMalformedHeader` | `internal/vfs/vfs_test.go` | Invalid vfs= line rejected | |
| `TestMissingTerminator` | `internal/vfs/vfs_test.go` | EOF without terminator error | |
| `TestDuplicatePaths` | `internal/vfs/vfs_test.go` | Same path twice behavior | |
| `TestEmptyTerminator` | `internal/vfs/vfs_test.go` | Empty terminator rejected | |
| `TestTerminatorSpecialChars` | `internal/vfs/vfs_test.go` | Terminator with `:` or `=` | |
| `TestEmptyFile` | `internal/vfs/vfs_test.go` | 0-byte file allowed | |
| `TestEmptyPath` | `internal/vfs/vfs_test.go` | Empty path rejected | |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `vfs-basic` | `test/data/unit/vfs/basic.ci` | Simple config + script | |
| `vfs-subdirs` | `test/data/unit/vfs/subdirs.ci` | Nested directory structure | |
| `vfs-binary` | `test/data/unit/vfs/binary.ci` | Base64 encoded file | |

## Files to Create

- `internal/env/env.go` - Shared env var handling (extract from slogutil)
- `internal/env/env_test.go` - Tests for env handling
- `internal/vfs/vfs.go` - Core VFS parser and types
- `internal/vfs/vfs_test.go` - Unit tests
- `internal/vfs/security_test.go` - Security boundary tests
- `internal/vfs/limits.go` - Constants and limit checking (uses internal/env)
- `internal/vfs/write.go` - Temp dir creation and file writing
- `internal/vfs/cleanup.go` - Signal handling and cleanup

## Files to Modify

- `internal/slogutil/slogutil.go` - Use internal/env instead of private getEnv
- `internal/test/ci/ciformat.go` - Integrate VFS parsing
- `internal/test/runner/record.go` - Use VFS for test execution, config rewriting

## Implementation Steps

1. **Write unit tests** - VFS parsing tests BEFORE implementation
2. **Run tests** - Verify FAIL (paste output)
3. **Implement `internal/vfs`** - Core parser
4. **Run tests** - Verify PASS (paste output)
5. **Write security tests** - Boundary and escape tests
6. **Run tests** - Verify FAIL
7. **Implement security checks** - Validation logic
8. **Run tests** - Verify PASS
9. **Implement temp dir + cleanup** - With signal handling
10. **Integrate with test runner** - Modify ciformat.go, record.go
11. **Functional tests** - End-to-end VFS tests
12. **Verify all** - `make lint && make test && make functional` (paste output)

## Script Execution Model

### Two Modes

| Mode | Config Source | Script Handling |
|------|---------------|-----------------|
| **Normal zebgp** | File or stdin | Read from filesystem |
| **Test runner (.ci)** | VFS embedded | Inline via zipapp (no disk) |

### Normal zebgp Operation

```bash
# Config from file
zebgp run peer.conf

# Config from stdin (- means stdin, standard Unix convention)
cat peer.conf | zebgp run -

# Scripts referenced in config are read from filesystem normally
```

Config syntax unchanged:
```
process p {
    run "./plugin.py";    # Read from filesystem
}
```

### Test Runner (.ci) - VFS Mode

The test runner rewrites VFS-embedded Python scripts to inline execution:

1. Parse VFS blocks from `.ci` file
2. For each `.py` in VFS: wrap as zipapp, base64 encode
3. Rewrite config: `run "./plugin.py"` → `run "python3" "-c" "import base64..."`
4. Write only the rewritten config to temp
5. Execute zebgp with rewritten config

```go
// Test runner pseudo-code
func rewriteConfig(config string, vfs *VFS) string {
    for path, content := range vfs.Files {
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

Environment variable overrides default (uses `internal/env`):

| `zebgp.path.python` / `zebgp_path_python` | Interpreter Used |
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
cmd=zebgp decode --update -f l2vpn/evpn 000000EA900F00E600...
expect=exit:code=0
expect=stdout:validate=json:contains="l2vpn/evpn"
```

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
# zebgp-peer rules embedded
vfs=rules.ci:terminator=EOF_RULES
option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=FFFF...
EOF_RULES

# zebgp config embedded
vfs=fast.conf:terminator=EOF_CONF
peer 127.0.0.1 { ... }
EOF_CONF

# zebgp-peer validates BGP output
cmd=mode=background:seq=1:run=zebgp-peer --port 1790 vfs//rules.ci

# zebgp runs test
cmd=mode=foreground:seq=2:stdin=fast.conf:run=zebgp run -

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
# zebgp-peer rules
vfs=rules.ci:terminator=EOF_RULES
option=asn:value=65533
expect=bgp:conn=1:seq=1:hex=...
EOF_RULES

# Plugin script
vfs=plugin.py:terminator=EOF_PY
#!/usr/bin/env python3
print('{"ready": true}')
EOF_PY

# zebgp config
vfs=plugin.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    process p { run "./plugin.py"; }
}
EOF_CONF

cmd=mode=background:seq=1:run=zebgp-peer --port 1790 vfs//rules.ci
cmd=mode=foreground:seq=2:stdin=plugin.conf:run=zebgp run -

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
vfs=graceful-restart.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    graceful-restart;
}
EOF_CONF

cmd=zebgp validate vfs//graceful-restart.conf
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
vfs=bad-config.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    invalid-option;
}
EOF_CONF

cmd=zebgp validate vfs//bad-config.conf
expect=exit:code=1
expect=stderr:contains=unknown option: invalid-option
```

### Migration Summary

| Source | Files | Target | Format |
|--------|-------|--------|--------|
| `decode/*.test` | 18 | `decode/*.ci` | cmd + expect |
| `encode/*.conf + *.ci` | ~20 | `encode/*.ci` | vfs + cmd + expect |
| `plugin/*.conf + *.ci + *.run` | ~20 | `plugin/*.ci` | vfs + cmd + expect |
| `parse/valid/*.conf` | ~50 | `parse/*.ci` | vfs + cmd + expect=exit:0 |
| `parse/invalid/*.conf + *.expect` | ~10 | `parse/*.ci` | vfs + cmd + expect=exit:1 + stderr |

### ExaBGP Migration

No changes needed - ExaBGP uses `.py`, ZeBGP uses `.py`:

| ExaBGP | ZeBGP | Notes |
|--------|-------|-------|
| `run "./plugin.py"` | `run "./plugin.py"` | Same syntax |
| `run "/usr/bin/python3 ./plugin.py"` | `run "./plugin.py"` | Strip interpreter |

### VFS Resolution

```
┌─────────────────────────────────────────────────────────┐
│  Config: run "./plugin.py"                              │
└─────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────┐
│  VFS lookup: plugin.py found?                           │
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

**VFS Input:**
```
vfs=plugin.py:terminator=EOF_PY
#!/usr/bin/env python3
import json, sys
print(json.dumps({"ready": True}))
sys.stdout.flush()
for line in sys.stdin:
    msg = json.loads(line)
    # process...
EOF_PY

vfs=peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533;
    process p {
        run "./plugin.py";
    }
}
EOF_CONF

cmd=zebgp run vfs//peer.conf
expect=exit:code=0
```

**What happens:**
1. VFS parsed: `plugin.py` and `peer.conf` in memory
2. Test runner rewrites config: `run "./plugin.py"` → inline zipapp command
3. Files written to temp dir
4. chdir to temp dir
5. `vfs//peer.conf` → `peer.conf`
6. Execute `zebgp run peer.conf`
7. Python executes inline via `python3 -c "import base64..."`
8. Script runs with stdin/stdout connected to zebgp
9. Cleanup temp dir

## API Design

### internal/env

```go
package env

// Get returns zebgp env var with dot/underscore support.
// Dot notation takes priority: zebgp.section.key > zebgp_section_key
func Get(section, key string) string

// GetInt returns int value, or default if not set/invalid.
func GetInt(section, key string, defaultVal int) int

// GetInt64 returns int64 value, or default if not set/invalid.
func GetInt64(section, key string, defaultVal int64) int64
```

### internal/vfs

```go
package vfs

import (
    "context"
    "io"
    "io/fs"
)

// Default limits for VFS parsing (overridable via zebgp_ci_* env vars)
const (
    DefaultMaxFileSize   = 1 << 20      // 1 MB
    DefaultMaxTotalSize  = 1 << 20      // 1 MB
    DefaultMaxFiles      = 100
    DefaultMaxPathLen    = 256
    DefaultMaxPathDepth  = 10
)

// LimitsFromEnv reads limits from environment, falling back to defaults
func LimitsFromEnv() Limits

// File represents a single file in the VFS
type File struct {
    Path     string
    Mode     fs.FileMode
    Content  []byte
}

// VFS holds parsed virtual filesystem
type VFS struct {
    Files  []*File
    Config string // from config= line (optional)
}

// Parse reads VFS blocks from reader
func Parse(r io.Reader) (*VFS, error)

// ParseWithLimits reads VFS with custom limits
func ParseWithLimits(r io.Reader, limits Limits) (*VFS, error)

// Validate checks all files for security issues
func (v *VFS) Validate() error

// WriteTo creates files in directory
// Returns cleanup function
func (v *VFS) WriteTo(baseDir string) error

// WriteToTemp creates temp dir, writes files, returns path and cleanup
// Cleanup is called automatically on ctx.Done() or signals
func (v *VFS) WriteToTemp(ctx context.Context) (dir string, cleanup func(), err error)

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
func (v *VFS) WriteToTemp(ctx context.Context) (string, func(), error) {
    dir, err := os.MkdirTemp("", "zebgp-vfs-*")
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

### Simple Test (test/data/unit/vfs/basic.ci)

```
vfs=peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533;
    peer-as 65533;
}
EOF_CONF

cmd=zebgp validate vfs//peer.conf
expect=exit:code=0
```

### With Script (test/data/unit/vfs/with-script.ci)

```
vfs=peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533;
    peer-as 65533;
    process test {
        run "./plugin.py";
    }
}
EOF_CONF

vfs=plugin.py:mode=755:terminator=EOF_PY
#!/usr/bin/env python3
import json
print(json.dumps({"ready": True}))
EOF_PY

cmd=zebgp run vfs//peer.conf
expect=exit:code=0
expect=stderr:modifier=not:contains=error
```

### Subdirectories (test/data/unit/vfs/subdirs.ci)

```
vfs=conf/peer.conf:terminator=EOF_CONF
peer 127.0.0.1 {
    local-as 65533;
    process p { run "./scripts/plugin.py"; }
}
EOF_CONF

vfs=scripts/plugin.py:mode=755:terminator=EOF_PY
#!/usr/bin/env python3
print('{"ready": true}')
EOF_PY

cmd=zebgp run vfs//conf/peer.conf
expect=exit:code=0
```

## Migration Path

### Phase 1: VFS Package
- Implement `internal/vfs` standalone
- Full test coverage

### Phase 2: Test Runner Integration
- Extend `.ci` parser to recognize VFS blocks
- Update test runner to use VFS

### Phase 3: Migrate Existing Tests
| Source | Destination |
|--------|-------------|
| `test/data/decode/*.test` | `test/data/unit/decode/*.ci` |
| `test/data/parse/valid/*.conf` | `test/data/unit/parse/*.ci` |
| `test/data/parse/invalid/*.conf+.expect` | `test/data/unit/parse/*.ci` |
| `test/data/encode/*.conf+.ci` | `test/data/unit/encode/*.ci` |
| `test/data/plugin/*.conf+.ci+.run` | `test/data/unit/plugin/*.ci` |

### Phase 4: CLI Integration
- Add `zebgp run --stdin` support
- Remove old format parsers

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
- [ ] `docs/architecture/testing/ci-format.md` created (VFS + line types)
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-vfs-format.md`
- [ ] All files committed together
