# Self-Check Test System

**Location:** `cmd/self-check/main.go`
**Test Data:** `testdata/encode/`

---

## Overview

The self-check system validates ZeBGP's BGP message encoding against expected byte sequences. It uses the same `.ci` file format as ExaBGP for test compatibility.

```
┌─────────────────┐     ┌─────────────────┐
│   self-check    │     │   zebgp-peer    │
│  (test runner)  │     │  (BGP server)   │
└────────┬────────┘     └────────┬────────┘
         │                       │
         │  1. Parse .ci file    │
         │  2. Start zebgp-peer  │
         │     with expectations │
         ├──────────────────────►│
         │                       │
         │  3. Start zebgp       │
         │     with .conf        │
         │                       │
         │  4. zebgp connects    │
         │     sends BGP msgs    │
         │         ─────────────►│
         │                       │
         │  5. zebgp-peer        │
         │     validates msgs    │
         │◄─────────────────────┤
         │  6. Success/Fail      │
         │                       │
```

---

## File Format (.ci)

The `.ci` file contains configuration reference, expected commands, and raw BGP message bytes.

```
option:file:config-name.conf
option:asn:65001
1:cmd:announce route 10.0.0.0/24 next-hop 1.2.3.4
1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:002D:02:0000001540010100...
1:json:{"exabgp":"6.0.0","type":"update",...}
```

### Line Types

| Prefix | Description | Example |
|--------|-------------|---------|
| `option:file:` | Config file to use | `option:file:attributes.conf` |
| `option:asn:` | Peer ASN override | `option:asn:65001` |
| `option:bind:` | Bind option | `option:bind:ipv6` |
| `N:cmd:` | Command that generates message | `1:cmd:announce route...` |
| `N:raw:` | Expected raw BGP bytes | `1:raw:FFFF...:0017:02:...` |
| `N:json:` | Expected JSON decode | `1:json:{...}` |

### Raw Message Format

```
N:raw:MARKER:LENGTH:TYPE:PAYLOAD
```

| Field | Bytes | Description |
|-------|-------|-------------|
| MARKER | 32 hex chars | Always `FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF` |
| LENGTH | 4 hex chars | Message length in bytes |
| TYPE | 2 hex chars | 01=OPEN, 02=UPDATE, 03=NOTIFICATION, 04=KEEPALIVE |
| PAYLOAD | variable | Message-specific data |

**Example UPDATE:**
```
1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:002D:02:0000001540010100400200400304650165014005040000006400
      └──────────── MARKER ──────────┘ └LEN┘ └T┘ └────────── PAYLOAD ──────────────────────────────┘
```

---

## Test Discovery

Tests are discovered by scanning `testdata/encode/` for `.ci` files:

```go
// cmd/self-check/main.go
func (ts *Tests) Load() error {
    pattern := filepath.Join(ts.baseDir, "testdata", "encode", "*.ci")
    files, _ := filepath.Glob(pattern)
    for _, f := range files {
        // Create test from .ci file
        nick := strings.TrimSuffix(filepath.Base(f), ".ci")
        // ...
    }
}
```

**Nick assignment:** Tests get numeric nicks (0, 1, 2...) in alphabetical order by filename.

---

## Running Tests

```bash
# List all tests
go run ./cmd/self-check --list

# Run all tests
go run ./cmd/self-check --all

# Run specific test by nick
go run ./cmd/self-check 0

# Run multiple tests
go run ./cmd/self-check 0 1 5

# Custom timeout
go run ./cmd/self-check --timeout 60s --all
```

---

## Test Execution Flow

1. **Build binaries** → `zebgp` and `zebgp-peer` to temp dir
2. **Parse .ci file** → Extract config path, options, expected messages
3. **Start zebgp-peer** → Listens on unique port, loads expectations
4. **Start zebgp** → Connects to peer, sends BGP messages
5. **Validate** → zebgp-peer compares received vs expected bytes
6. **Report** → Success if all messages match, fail otherwise

---

## Adding New Tests

### 1. Create config file

```
# testdata/encode/mytest.conf
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
# testdata/encode/mytest.ci
option:file:mytest.conf
1:cmd:announce route 10.0.0.0/24 next-hop 1.2.3.4
1:raw:FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:002D:02:0000001540010100400200400304010203044005040000006418
```

### 3. Generate expected bytes

**Option A:** Run with ExaBGP first, capture output
```bash
# In exabgp directory
env exabgp_tcp_port=1790 python3 qa/sbin/bgp --port 1790 --view
# Then run exabgp and capture bytes
```

**Option B:** Use zebgp-decode to verify
```bash
# Decode existing message to verify
go run ./cmd/zebgp-decode update PAYLOAD_HEX
```

---

## Current Test Status

See `plan/CLAUDE_CONTINUATION.md` for current pass/fail status.

**Passing tests:** ~27/37
**Known failures:** See continuation file for details

---

## Debugging Failed Tests

### View expected vs actual

```bash
# Run single test with verbose output
go run ./cmd/self-check 0 2>&1 | tee /tmp/test.log
```

### Manual test execution

```bash
# Terminal 1: Start peer
go run ./cmd/zebgp-peer --port 1790 testdata/encode/attributes.ci

# Terminal 2: Run zebgp
env exabgp_tcp_port=1790 go run ./cmd/zebgp server testdata/encode/attributes.conf
```

### Decode message bytes

```bash
# Decode UPDATE message
go run ./cmd/zebgp-decode update 0000001540010100400200400304650165014005040000006400

# Decode with full marker
go run ./cmd/zebgp-decode raw FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002D020000001540010100...
```

---

## API Tests (testdata/api/)

The `testdata/api/` directory contains Python `.run` scripts from ExaBGP. These are **not currently used** by self-check but are intended for future API testing.

### .run Script Structure

```python
#!/usr/bin/env python3
import time
from exabgp_api import flush, wait_for_shutdown

def main():
    time.sleep(0.2)  # Wait for connection
    flush('announce route 10.0.0.0/24 next-hop 1.2.3.4\n')
    time.sleep(0.2)  # Allow batching
    wait_for_shutdown()

if __name__ == '__main__':
    main()
```

### Future: Commit-Based Testing

The `.run` scripts currently use `time.sleep()` for timing. Future plan (see `plan/api-commit-batching.md`) will replace with commit semantics:

```python
# Future API test pattern
send('commit start batch1')
send('announce route 10.0.0.0/24 next-hop 1.2.3.4')
send('announce route 10.1.0.0/24 next-hop 1.2.3.4')
response = send('commit end batch1')  # Synchronous, returns stats
assert response['status'] == 'ok'
```

---

## ExaBGP Test Compatibility

ZeBGP uses the same `.ci` format as ExaBGP for test portability:

| ExaBGP Location | ZeBGP Location | Purpose |
|-----------------|----------------|---------|
| `qa/encoding/*.ci` | `testdata/encode/*.ci` | Static route encoding |
| `qa/api/*.ci` | `testdata/api/*.ci` | API command encoding |
| `qa/sbin/bgp` | `cmd/zebgp-peer` | Test peer implementation |

**Copying tests from ExaBGP:**
```bash
# Copy encoding test
cp ../main/qa/encoding/conf-newtest.ci testdata/encode/newtest.ci

# May need to adjust config file path
# option:file:newtest.conf → create matching .conf
```

---

## Architecture: zebgp-peer

The test peer (`cmd/zebgp-peer`) validates received messages:

```go
// Simplified flow
func (p *Peer) handleConnection(conn net.Conn) {
    // 1. Exchange OPEN
    // 2. For each expected message:
    for _, expect := range p.expectations {
        msg := readMessage(conn)
        if !bytes.Equal(msg, expect) {
            return fmt.Errorf("mismatch at message %d", i)
        }
    }
    // 3. All matched = success
}
```

**Key flags:**
- `--sink` - Accept anything, don't validate (useful for debugging)
- `--echo` - Echo messages back
- `--view` - Print expectations and exit

---

## Concurrency

Self-check runs tests in parallel with limited concurrency:

```go
const maxConcurrent = 4
semaphore := make(chan struct{}, maxConcurrent)

for _, test := range selected {
    go func(t *Test) {
        semaphore <- struct{}{}        // Acquire
        defer func() { <-semaphore }() // Release
        // Run test...
    }(test)
}
```

Each test gets a unique port to avoid conflicts.

---

**Updated:** 2025-12-21
