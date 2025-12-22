# API Commit-Based Route Batching

**Status:** 🔴 Active - Required for test compatibility
**Created:** 2025-12-21
**Updated:** 2025-12-22

---

## Problem Statement

### Current ZeBGP Behavior
- API routes sent **immediately** when received
- No batching for dynamic API announcements
- `group-updates` only applies to static routes at session establishment
- Test scripts use fragile `time.sleep()` to achieve expected packet patterns

### ExaBGP Behavior
- RIB batches routes by (attributes, family) automatically
- `group start` / `group end` API commands for explicit batching
- No timer-based accumulation - routes sent when RIB flushed
- Test scripts still use `sleep()` because there's no "commit" acknowledgment

### Critical Requirement

**Converting ALL `.run` scripts to use commit-based batching is REQUIRED for `.ci` tests to pass.**

The ExaBGP test suite relies on specific packet batching patterns. Without explicit commit semantics, ZeBGP cannot reproduce the exact UPDATE message grouping that ExaBGP produces, causing `.ci` validation failures.

### Goals

1. **Primary:** Convert ALL ExaBGP `.run` tests to pass with ZeBGP
2. **Method:** Replace sleep-based timing with explicit commit semantics
3. **Result:** Deterministic, reproducible packet generation
4. **Bonus:** Efficient batching for bulk operations in production

---

## Proposed Architecture

### 1. API Commit Commands

```
commit start [label]     # Begin transaction (optional label for debugging)
announce route ...       # Routes queued, not sent
announce route ...
withdraw route ...
commit end [label]       # Flush all queued routes as batched UPDATEs
```

**Behavior:**
- `commit start` → enters transaction mode, routes queue in OutgoingRIB
- `commit end` → triggers RIB flush, groups routes by attributes, sends UPDATEs
- Returns **after** UPDATEs are sent (synchronous acknowledgment)
- Nested commits NOT supported (error if already in transaction)

**Response:**
```json
{"status": "ok", "updates_sent": 3, "routes_announced": 15, "routes_withdrawn": 2}
```

### 2. Implicit Commit (Auto-Flush)

For backwards compatibility and simple use cases:
- Routes outside transaction → sent immediately (current behavior)
- OR: configurable auto-commit delay (see RIB options below)

### 3. RIB Configuration Options

Move batching config to RIB section with new options:

```
rib {
    # Existing group-updates behavior (group by attributes)
    group-updates true;        # default: true

    # NEW: Auto-commit delay (implicit batching)
    # Routes accumulate for this duration after last route received
    # 0 = immediate send (current behavior)
    auto-commit-delay 100ms;   # default: 0

    # NEW: Maximum batch size before forced flush
    max-batch-size 1000;       # default: unlimited
}

neighbor 192.168.1.1 {
    # DEPRECATED: group-updates here
    # Still works but warns: "group-updates in neighbor is deprecated, use rib section"
}
```

**Auto-commit-delay behavior:**
1. First route arrives → start timer
2. More routes arrive → reset timer
3. Timer expires → flush (group by attributes, send UPDATEs)
4. `commit end` → immediate flush (ignores timer)

### 4. Configuration Migration

```bash
zebgp fmt config.bgp              # Format config, migrate deprecated options
zebgp fmt --check config.bgp      # Check if needs formatting (exit 1 if changes needed)
zebgp fmt --diff config.bgp       # Show what would change
```

**Migration rules:**
1. `neighbor { group-updates X }` → `rib { group-updates X }` + deprecation comment
2. Normalize indentation, ordering
3. Remove redundant defaults

---

## Phase Status

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Core Infrastructure (transactions) | ✅ Already implemented |
| 2 | Route Grouping | ✅ Already implemented |
| 3 | RIB Configuration | ⬜ Not started |
| 4 | Config Formatter | ⬜ Not started |
| 5 | Test Infrastructure | ⬜ Not started |
| 6 | Self-Check API Support | ✅ **Completed 2025-12-22** |
| 7 | Test Conversion | 🔄 In progress (0/45) |

---

## Implementation Phases

### Phase 1: Core Infrastructure

**1.1 Extend OutgoingRIB for transactions**
```go
// pkg/rib/outgoing.go
type OutgoingRIB struct {
    // ... existing fields ...

    // Transaction state
    inTransaction  bool
    transactionID  string  // optional label
}

func (r *OutgoingRIB) BeginTransaction(label string) error
func (r *OutgoingRIB) CommitTransaction() (stats CommitStats, err error)
func (r *OutgoingRIB) RollbackTransaction() error  // discard pending without sending
func (r *OutgoingRIB) InTransaction() bool
```

**1.2 Add commit commands to API**
```go
// pkg/api/commit.go
func handleCommitStart(ctx *Context, args []string) Response
func handleCommitEnd(ctx *Context, args []string) Response
func handleCommitRollback(ctx *Context, args []string) Response
```

**1.3 Wire to dispatcher**
```go
// pkg/api/dispatcher.go
dispatcher.Register("commit", handleCommit)  // routes to start/end/rollback
```

### Phase 2: Route Grouping for API Routes

**2.1 Implement attribute-based grouping**
```go
// pkg/rib/grouping.go
type RouteGroup struct {
    Attributes []attribute.Attribute
    NLRIs      []nlri.NLRI
}

func GroupByAttributes(routes []Route) []RouteGroup
```

**2.2 Generate grouped UPDATEs**
```go
// pkg/reactor/update.go
func BuildGroupedUpdates(groups []RouteGroup, negotiated Negotiated) []*message.Update
```

**2.3 Integrate with peer send path**
- Currently: `Reactor.AnnounceRoute()` → immediate send
- After: `Reactor.AnnounceRoute()` → queue in OutgoingRIB → flush on commit/timer

### Phase 3: RIB Configuration

**3.1 Add RIB section to config schema**
```go
// pkg/config/bgp.go
func ribFields() []Field {
    return []Field{
        Field("group-updates", LeafWithDefault(TypeBool, "true")),
        Field("auto-commit-delay", LeafWithDefault(TypeDuration, "0")),
        Field("max-batch-size", LeafWithDefault(TypeInt, "0")),  // 0 = unlimited
    }
}
```

**3.2 Deprecate neighbor-level group-updates**
```go
// pkg/config/loader.go - emit warning if neighbor.group-updates set
log.Warn("neighbor-level group-updates is deprecated, use rib section")
```

**3.3 Implement auto-commit timer**
```go
// pkg/rib/timer.go
type AutoCommitTimer struct {
    delay    time.Duration
    timer    *time.Timer
    onExpire func()
}
```

### Phase 4: Config Formatter

**4.1 Implement formatter**
```go
// cmd/zebgp-fmt/main.go
// OR integrated: zebgp fmt

func Format(input []byte) ([]byte, error)
func Migrate(input []byte) ([]byte, []Warning, error)
```

**4.2 CLI interface**
```bash
zebgp fmt config.bgp              # Print formatted config to stdout (default)
zebgp fmt -w config.bgp           # Write back to file (in-place)
zebgp fmt --check config.bgp      # Check if needs formatting (exit 1 if changes)
zebgp fmt --diff config.bgp       # Show diff of what would change
zebgp fmt -                       # Read from stdin, write to stdout
```

**Flags:**
| Flag | Description |
|------|-------------|
| (none) | Print to stdout (like `gofmt`) |
| `-w` | Write result to source file |
| `--check` | Exit 1 if file needs formatting (CI use) |
| `--diff` | Show unified diff |
| `-` | Read stdin |

**4.3 Migration rules**
- Move `neighbor { group-updates }` → `rib { group-updates }`
- Normalize whitespace, ordering
- Add deprecation comments for migrated options

### Phase 5: Test Infrastructure

**5.1 Update run scripts**
Replace:
```python
time.sleep(0.2)
send("announce route ...")
send("announce route ...")
time.sleep(0.2)
```

With:
```python
send("commit start batch1")
send("announce route ...")
send("announce route ...")
response = send("commit end batch1")
assert response["status"] == "ok"
```

**5.2 Update .ci expected files**
- May need regeneration if packet ordering changes
- Commit-based tests should be deterministic

---

## API Command Reference

### commit start
```
commit start [label]
```
- Begins a transaction
- Routes queued until `commit end`
- Optional label for debugging/logging
- Error if already in transaction

**Response:**
```json
{"status": "ok", "transaction": "batch1"}
```

### commit end
```
commit end [label]
```
- Flushes all queued routes
- Groups by attributes, sends minimal UPDATEs
- **Sends EOR for each family that had routes in this commit**
- Synchronous: returns after all UPDATEs and EORs sent
- Label must match start (if provided)

**Response:**
```json
{
  "status": "ok",
  "updates_sent": 3,
  "routes_announced": 15,
  "routes_withdrawn": 2,
  "eor_sent": ["ipv4 unicast", "ipv6 unicast"],
  "transaction": "batch1"
}
```

### commit rollback
```
commit rollback [label]
```
- Discards all queued routes without sending
- Returns to non-transaction mode

**Response:**
```json
{"status": "ok", "routes_discarded": 17, "transaction": "batch1"}
```

---

## Compatibility Matrix

| Scenario | Current | With Commits |
|----------|---------|--------------|
| Single route, no transaction | Immediate send | Immediate send (unchanged) |
| Single route, in transaction | N/A | Queued until commit |
| Multiple routes, no transaction | Each sent immediately | Each sent immediately OR batched by auto-commit-delay |
| Multiple routes, in transaction | N/A | All batched on commit |
| Test scripts with sleep | Works (fragile) | Use commits (deterministic) |

---

## ExaBGP Comparison

| Feature | ExaBGP | ZeBGP (Proposed) |
|---------|--------|------------------|
| Explicit batching | `group start/end` | `commit start/end` |
| Attribute grouping | `group-updates` in neighbor | `group-updates` in rib (neighbor deprecated) |
| Timer-based batching | None | `auto-commit-delay` in rib |
| Batch size limit | None | `max-batch-size` in rib |
| Synchronous response | No | Yes (`commit end` returns stats) |
| Config formatting | None | `zebgp fmt` |

---

## Test Plan

### Unit Tests
1. `TestOutgoingRIB_Transaction` - begin/commit/rollback semantics
2. `TestGroupByAttributes` - correct grouping logic
3. `TestAutoCommitTimer` - timer reset, expiration, cancellation
4. `TestAPICommitCommands` - command parsing, responses

### Integration Tests
1. Commit multiple routes → verify single UPDATE with multiple NLRIs
2. Commit routes with different attributes → verify multiple UPDATEs
3. Auto-commit-delay → verify timer behavior
4. Nested commit → verify error

### Migration Tests
1. Old config with neighbor-level group-updates → warning emitted
2. `zebgp fmt` migrates correctly
3. Both old and new config locations work

---

## Open Questions

1. **Per-peer transactions?**
   - Current design: global transaction
   - Alternative: `commit start peer=192.168.1.1`
   - Recommendation: Start global, add per-peer later if needed

2. **Transaction timeout?**
   - What if client sends `commit start` but never `commit end`?
   - Option: Auto-rollback after N seconds
   - Option: Rollback on client disconnect

3. **Concurrent clients?**
   - Multiple API clients, each with own transaction?
   - Or single global transaction (last one wins)?
   - Recommendation: Per-client transactions (client ID from connection)

4. **ExaBGP `group` compatibility?**
   - Should we also support `group start/end` as alias?
   - Recommendation: Yes, for migration ease

---

## Files to Create/Modify

### New Files
- `pkg/rib/transaction.go` - Transaction state management
- `pkg/rib/grouping.go` - Attribute-based route grouping
- `pkg/rib/timer.go` - Auto-commit timer
- `pkg/api/commit.go` - Commit command handlers
- `cmd/zebgp-fmt/main.go` - Config formatter (or integrate in zebgp)

### Modified Files
- `pkg/rib/outgoing.go` - Add transaction support
- `pkg/api/dispatcher.go` - Register commit commands
- `pkg/api/types.go` - Add CommitStats type
- `pkg/config/bgp.go` - Add rib section schema
- `pkg/config/loader.go` - Load rib config, deprecation warnings
- `pkg/reactor/reactor.go` - Wire RIB config to peers
- `pkg/reactor/peer.go` - Use grouped UPDATE generation

---

## Timeline Estimate

| Phase | Scope | Complexity |
|-------|-------|------------|
| Phase 1 | Transaction infrastructure | Medium |
| Phase 2 | Route grouping | Medium |
| Phase 3 | RIB configuration | Low |
| Phase 4 | Config formatter | Medium |
| Phase 5 | Test updates | Low-Medium |

Total: ~2-3 focused sessions

---

## Success Criteria

1. **ALL 45 `.run` test scripts converted** to use `commit start/end`
2. **ALL `.ci` tests pass** with ZeBGP
3. Test output matches ExaBGP byte-for-byte
4. `zebgp fmt` correctly migrates old configs
5. Deprecation warnings for old `group-updates` location
6. Documentation updated

---

## Implementation Progress

### Completed (2025-12-22)

1. ✅ **Process spawning infrastructure** - `pkg/api/process.go` updated to set working directory
2. ✅ **API server process integration** - Server now starts ProcessManager and handles commands
3. ✅ **Config loader** - Passes processes and config directory to reactor
4. ✅ **Socket path configuration** - `zebgp_api_socketpath` env var for testing
5. ✅ **self-check API test support** - Loads tests from `test/data/api/`, sets socket path
6. ✅ **testpeer .ci parsing** - Ignores `option:file:`, `:cmd:`, `:json:` lines
7. ✅ **Process symlink** - `test/data/api/exabgp_api.py` → `../scripts/exabgp_api.py`

### Current State

- API tests run through self-check
- Processes spawn and send commands
- Routes are announced to peers
- Message validation works

**Remaining issue:** Message content mismatch - `.run` scripts need conversion to produce correct attributes.

### Next Steps

1. **Fix attribute handling in announce route**
   - Add `origin igp` default for iBGP
   - Add `local-preference 100` default for iBGP
   - Fix AS_PATH to be empty for iBGP (currently sends [0])

2. **Copy remaining .ci files from ExaBGP**
   - Source: `../main/qa/api/api-*.ci`
   - Target: `test/data/api/*.ci`
   - Update `option:file:` references

3. **Convert fast.run as template**
   - Update to use commit-based batching
   - Add correct attributes to match .ci expectations
   - Document conversion pattern for other tests

4. **Batch convert remaining tests**
   - 34 tests have matching .ci files in ExaBGP
   - 11 tests are ZeBGP-specific or need investigation

---

## Test Conversion Tracking

**Total:** 45 `.run` files in `test/data/api/`

### Conversion Status

| Status | Test | Notes |
|--------|------|-------|
| 🔄 | fast.run | Infrastructure works, needs attribute fixes |
| ⬜ | ack-control.run | |
| ⬜ | add-remove.run | |
| ⬜ | announce-star.run | |
| ⬜ | announce.run | |
| ⬜ | announcement.run | |
| ⬜ | api.nothing.run | |
| ⬜ | api.receive.run | |
| ⬜ | attributes-path.run | |
| ⬜ | attributes-vpn.run | |
| ⬜ | attributes.run | |
| ⬜ | blocklist.run | |
| ⬜ | broken-flow.run | |
| ⬜ | check.run | |
| ⬜ | eor.run | |
| ⬜ | fast.run | |
| ⬜ | flow-merge.run | |
| ⬜ | flow.run | |
| ⬜ | health.run | |
| ⬜ | ipv4.run | |
| ⬜ | ipv6.run | |
| ⬜ | manual-eor.run | |
| ⬜ | multi-neighbor.run | |
| ⬜ | multiple-private.run | |
| ⬜ | multiple-public.run | |
| ⬜ | multisession.run | |
| ⬜ | mvpn.run | |
| ⬜ | nexthop-self.run | |
| ⬜ | nexthop.run | |
| ⬜ | no-neighbor.run | |
| ⬜ | no-respawn-1.run | |
| ⬜ | no-respawn-2.run | |
| ⬜ | notification.run | |
| ⬜ | open.run | |
| ⬜ | peer-lifecycle.run | |
| ⬜ | reload.run | |
| ⬜ | rib.run | |
| ⬜ | rr-rib.run | |
| ⬜ | rr.run | |
| ⬜ | silence-ack.run | |
| ⬜ | simple.run | |
| ⬜ | teardown.run | |
| ⬜ | v6-comprehensive.run | |
| ⬜ | vpls.run | |
| ⬜ | vpnv4.run | |
| ⬜ | watchdog.run | |

**Legend:** ⬜ Not started | 🔄 In progress | ✅ Converted & passing | ⏭️ Skipped (N/A)

### Conversion Progress

- **Converted:** 0/45
- **Passing:** 0/45
- **Skipped:** 0/45

---

## Phase 6: Self-Check API Test Support

### Current State

The self-check system (`test/cmd/self-check/`) currently only supports **static route tests**:
- Tests in `test/data/encode/` use `.ci` + `.conf` files
- Routes are defined statically in config, sent at session establishment
- No support for dynamic API-driven tests

The `.run` scripts in `test/data/api/` are **not currently used** by ZeBGP.

### Goal

Enable self-check to run API tests that:
1. Use `.run` Python scripts to inject routes dynamically
2. Support commit-based batching for deterministic output
3. Validate against `.ci` expected messages

### Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   self-check    │     │     zebgp       │     │   zebgp-peer    │
│  (test runner)  │     │   (with API)    │     │  (BGP server)   │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         │  1. Start zebgp-peer  │                       │
         ├──────────────────────────────────────────────►│
         │                       │                       │
         │  2. Start zebgp       │                       │
         │     with API socket   │                       │
         ├──────────────────────►│                       │
         │                       │                       │
         │  3. Run .run script   │                       │
         │     (connects to API) │                       │
         ├───────┐               │                       │
         │       │ API socket    │                       │
         │       └──────────────►│                       │
         │                       │                       │
         │  4. Script sends      │  5. zebgp sends      │
         │     commit start      │     BGP UPDATE       │
         │     announce route    │─────────────────────►│
         │     commit end        │                       │
         │                       │                       │
         │                       │  6. zebgp-peer       │
         │                       │     validates        │
         │◄──────────────────────────────────────────────┤
         │  7. Success/Fail      │                       │
```

### Test File Structure

```
test/data/api/
├── fast.ci              # Expected messages
├── fast.conf            # ZeBGP config (with API enabled)
├── fast.run             # Python script to drive API
└── exabgp_api.py        # Shared helper library
```

### .conf Changes for API Tests

API tests need a config that enables the API socket:

```
# test/data/api/fast.conf
process api-driver {
    run ./test/data/api/fast.run;
    encoder text;
}

neighbor 127.0.0.1 {
    router-id 1.2.3.4;
    local-address 127.0.0.1;
    local-as 1;
    peer-as 1;

    api {
        processes [ api-driver ];
    }
}
```

### .run Script Conversion

**Before (sleep-based):**
```python
#!/usr/bin/env python3
import time
from exabgp_api import flush, wait_for_shutdown

def main():
    messages1 = [
        'announce route 1.1.0.0/24 next-hop 101.1.101.1',
        'announce route 1.1.0.0/25 next-hop 101.1.101.1',
    ]

    time.sleep(0.2)  # Wait for session
    flush('\n'.join(messages1) + '\n')
    time.sleep(0.2)  # Hope routes batch together
    wait_for_shutdown()

if __name__ == '__main__':
    main()
```

**After (commit-based):**
```python
#!/usr/bin/env python3
from exabgp_api import send, send_batch, wait_for_session, wait_for_shutdown

def main():
    messages1 = [
        'announce route 1.1.0.0/24 next-hop 101.1.101.1',
        'announce route 1.1.0.0/25 next-hop 101.1.101.1',
    ]

    wait_for_session()  # Deterministic: wait for ESTABLISHED

    # Batch 1: These will be grouped into single UPDATE
    response = send_batch('batch1', messages1)
    assert response['status'] == 'ok'
    assert response['updates_sent'] == 1  # Grouped!

    wait_for_shutdown()

if __name__ == '__main__':
    main()
```

### Updated exabgp_api.py Helper

```python
#!/usr/bin/env python3
"""API helper library for ZeBGP test scripts."""

import json
import sys
from typing import List, Optional

def flush(msg: str) -> None:
    """Write message to stdout and flush."""
    sys.stdout.write(msg)
    sys.stdout.flush()

def read_response() -> dict:
    """Read JSON response from stdin."""
    line = sys.stdin.readline().strip()
    if not line:
        return {}
    try:
        return json.loads(line)
    except json.JSONDecodeError:
        return {'raw': line}

def send(command: str) -> dict:
    """Send command and return response."""
    flush(command + '\n')
    return read_response()

def send_batch(label: str, commands: List[str]) -> dict:
    """Send commands as atomic batch using commit."""
    flush(f'commit start {label}\n')
    start_response = read_response()
    if start_response.get('status') != 'ok':
        return start_response

    for cmd in commands:
        flush(cmd + '\n')
        # Route commands don't return response in transaction

    flush(f'commit end {label}\n')
    return read_response()

def wait_for_session(timeout: float = 5.0) -> bool:
    """Wait for BGP session to reach ESTABLISHED."""
    import time
    start = time.time()
    while time.time() - start < timeout:
        response = send('status')
        if 'established' in str(response).lower():
            return True
        time.sleep(0.1)
    return False

def wait_for_shutdown() -> None:
    """Wait for shutdown signal."""
    try:
        while True:
            line = sys.stdin.readline()
            if not line:
                break
    except (IOError, EOFError):
        pass
```

### Self-Check Changes

**6.1 Detect API tests**

```go
// test/cmd/self-check/main.go
func (t *Test) IsAPITest() bool {
    // Has .run file alongside .ci
    runFile := strings.TrimSuffix(t.CIFile, ".ci") + ".run"
    _, err := os.Stat(runFile)
    return err == nil
}
```

**6.2 Run API test differently**

```go
func (r *Runner) runAPITest(ctx context.Context, test *Test) (bool, string) {
    // 1. Start zebgp-peer (same as static tests)

    // 2. Start zebgp WITH API socket
    apiSocket := fmt.Sprintf("/tmp/zebgp-test-%d.sock", test.Port)
    clientCmd := exec.CommandContext(testCtx, r.zebgpPath, "server", test.Config)
    clientCmd.Env = append(os.Environ(),
        fmt.Sprintf("exabgp_tcp_port=%d", test.Port),
        fmt.Sprintf("exabgp_api_socket=%s", apiSocket),
    )

    // 3. Wait for API socket to exist
    waitForSocket(apiSocket, 5*time.Second)

    // 4. Run .run script with API socket
    runFile := strings.TrimSuffix(test.CIFile, ".ci") + ".run"
    runCmd := exec.CommandContext(testCtx, "python3", runFile)
    runCmd.Env = append(os.Environ(),
        fmt.Sprintf("ZEBGP_API_SOCKET=%s", apiSocket),
    )
    // Connect script stdin/stdout to API socket

    // 5. Wait for completion, check zebgp-peer result
}
```

**6.3 Process communication**

The `.run` script communicates with zebgp via the process API:

```
┌──────────────┐     stdin/stdout     ┌──────────────┐
│  .run script │◄────────────────────►│    zebgp     │
│  (Python)    │     (pipes)          │   process    │
└──────────────┘                      └──────────────┘
```

ZeBGP's process manager (`pkg/api/process.go`) handles:
- Spawning the script
- Piping stdin/stdout
- Delivering API responses

---

## Phase 7: Conversion Workflow

### Step-by-Step: Converting One .run Test

1. **Identify the test**
   ```bash
   ls test/data/api/*.run
   # Pick: fast.run
   ```

2. **Check corresponding .ci file**
   ```bash
   cat test/data/api/fast.ci
   # Note expected messages
   ```

3. **Create/update config**
   ```bash
   # Ensure fast.conf exists and has:
   # - process block pointing to fast.run
   # - neighbor with api { processes [...] }
   ```

4. **Convert .run script**
   - Replace `time.sleep()` with `wait_for_session()`
   - Replace `flush()` batches with `send_batch()`
   - Add assertions on responses

5. **Test locally**
   ```bash
   # Terminal 1
   go run ./test/cmd/zebgp-peer --port 1790 test/data/api/fast.ci

   # Terminal 2
   env exabgp_tcp_port=1790 go run ./cmd/zebgp server test/data/api/fast.conf
   ```

6. **Verify with self-check**
   ```bash
   go run ./test/cmd/self-check api-fast
   ```

### Batch Conversion Script

```bash
#!/bin/bash
# convert-api-tests.sh

for run in test/data/api/*.run; do
    name=$(basename "$run" .run)
    ci="test/data/api/${name}.ci"
    conf="test/data/api/${name}.conf"

    if [[ ! -f "$ci" ]]; then
        echo "SKIP: $name (no .ci file)"
        continue
    fi

    echo "Converting: $name"

    # Backup original
    cp "$run" "${run}.bak"

    # Apply conversion (simplified - real script would parse Python)
    sed -i '' \
        -e 's/time\.sleep([0-9.]*)/# REMOVED: sleep/' \
        -e 's/from exabgp_api import.*/from exabgp_api import send, send_batch, wait_for_session, wait_for_shutdown/' \
        "$run"

    echo "  -> Manual review needed: $run"
done
```

---

## Detailed Data Flow

### Route Announcement Flow (with commits)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           API CLIENT (.run script)                          │
├─────────────────────────────────────────────────────────────────────────────┤
│  send("commit start batch1")                                                │
│  send("announce route 10.0.0.0/24 next-hop 1.2.3.4")                       │
│  send("announce route 10.1.0.0/24 next-hop 1.2.3.4")                       │
│  send("announce route 10.2.0.0/24 next-hop 5.6.7.8")  # Different NH       │
│  response = send("commit end batch1")                                       │
└─────────────────────────────────┬───────────────────────────────────────────┘
                                  │ stdin pipe
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           PROCESS MANAGER                                    │
│                         (pkg/api/process.go)                                │
├─────────────────────────────────────────────────────────────────────────────┤
│  Reads lines from script stdout                                             │
│  Dispatches to command handlers                                             │
└─────────────────────────────────┬───────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           API DISPATCHER                                     │
│                         (pkg/api/dispatcher.go)                             │
├─────────────────────────────────────────────────────────────────────────────┤
│  "commit start" → handleCommitStart()                                       │
│  "announce route" → handleAnnounceRoute() [queues in RIB]                   │
│  "commit end" → handleCommitEnd() [triggers flush]                          │
└─────────────────────────────────┬───────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           OUTGOING RIB                                       │
│                         (pkg/rib/outgoing.go)                               │
├─────────────────────────────────────────────────────────────────────────────┤
│  Transaction State:                                                          │
│    inTransaction: true                                                       │
│    transactionID: "batch1"                                                   │
│                                                                              │
│  Pending Routes:                                                             │
│    [10.0.0.0/24, NH=1.2.3.4, origin=igp]                                   │
│    [10.1.0.0/24, NH=1.2.3.4, origin=igp]  ─┐ Same attrs                    │
│    [10.2.0.0/24, NH=5.6.7.8, origin=igp]   │ Different NH                  │
│                                             │                                │
│  On CommitTransaction():                    │                                │
│    1. Group by attributes                   │                                │
│    2. Build UPDATE messages                 ▼                                │
└─────────────────────────────────┬───────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           ROUTE GROUPER                                      │
│                         (pkg/rib/grouping.go)                               │
├─────────────────────────────────────────────────────────────────────────────┤
│  Group 1: [NH=1.2.3.4, origin=igp]                                          │
│    NLRIs: [10.0.0.0/24, 10.1.0.0/24]                                       │
│                                                                              │
│  Group 2: [NH=5.6.7.8, origin=igp]                                          │
│    NLRIs: [10.2.0.0/24]                                                     │
└─────────────────────────────────┬───────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           UPDATE BUILDER                                     │
│                         (pkg/reactor/update.go)                             │
├─────────────────────────────────────────────────────────────────────────────┤
│  UPDATE 1:                                                                   │
│    Withdrawn Routes Length: 0                                                │
│    Total Path Attribute Length: 21                                           │
│    Path Attributes:                                                          │
│      ORIGIN: IGP                                                             │
│      NEXT_HOP: 1.2.3.4                                                       │
│    NLRI:                                                                     │
│      10.0.0.0/24                                                             │
│      10.1.0.0/24    ← Two prefixes in one UPDATE!                           │
│                                                                              │
│  UPDATE 2:                                                                   │
│    Path Attributes:                                                          │
│      ORIGIN: IGP                                                             │
│      NEXT_HOP: 5.6.7.8                                                       │
│    NLRI:                                                                     │
│      10.2.0.0/24                                                             │
└─────────────────────────────────┬───────────────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           PEER                                               │
│                         (pkg/reactor/peer.go)                               │
├─────────────────────────────────────────────────────────────────────────────┤
│  SendUpdate(update1)  →  TCP connection  →  zebgp-peer                      │
│  SendUpdate(update2)  →  TCP connection  →  zebgp-peer                      │
│                                                                              │
│  Track families: [IPv4 Unicast]                                             │
│                                                                              │
│  SendEOR(IPv4 Unicast)  →  TCP connection  →  zebgp-peer                   │
└─────────────────────────────────────────────────────────────────────────────┘

Response to client:
{
  "status": "ok",
  "updates_sent": 2,
  "routes_announced": 3,
  "routes_withdrawn": 0,
  "eor_sent": ["ipv4 unicast"],
  "transaction": "batch1"
}
```

---

## Attribute Grouping Algorithm

```go
// pkg/rib/grouping.go

// RouteGroup represents routes that share identical attributes
type RouteGroup struct {
    Family     family.Family
    Attributes []attribute.Attribute
    NLRIs      []nlri.NLRI
    Withdraws  []nlri.NLRI
}

// GroupByAttributes groups routes by their attribute set
// Routes with identical attributes can share a single UPDATE message
func GroupByAttributes(routes []PendingRoute) []RouteGroup {
    // Key: hash of (family + sorted attributes)
    groups := make(map[string]*RouteGroup)

    for _, route := range routes {
        key := groupKey(route.Family, route.Attributes)

        if g, ok := groups[key]; ok {
            // Add to existing group
            if route.IsWithdraw {
                g.Withdraws = append(g.Withdraws, route.NLRI)
            } else {
                g.NLRIs = append(g.NLRIs, route.NLRI)
            }
        } else {
            // Create new group
            g := &RouteGroup{
                Family:     route.Family,
                Attributes: route.Attributes,
            }
            if route.IsWithdraw {
                g.Withdraws = []nlri.NLRI{route.NLRI}
            } else {
                g.NLRIs = []nlri.NLRI{route.NLRI}
            }
            groups[key] = g
        }
    }

    // Convert to slice, sort for deterministic ordering
    result := make([]RouteGroup, 0, len(groups))
    for _, g := range groups {
        result = append(result, *g)
    }
    sort.Slice(result, func(i, j int) bool {
        return result[i].Key() < result[j].Key()
    })

    return result
}

// groupKey generates a unique key for attribute grouping
func groupKey(fam family.Family, attrs []attribute.Attribute) string {
    // Sort attributes by type code for consistent ordering
    sorted := make([]attribute.Attribute, len(attrs))
    copy(sorted, attrs)
    sort.Slice(sorted, func(i, j int) bool {
        return sorted[i].TypeCode() < sorted[j].TypeCode()
    })

    // Build key: family + each attribute's bytes
    var buf bytes.Buffer
    buf.WriteString(fam.String())
    for _, attr := range sorted {
        buf.Write(attr.Bytes())
    }

    return buf.String()
}
```

---

## Error Handling

### Transaction Errors

| Error | Cause | Response |
|-------|-------|----------|
| `already_in_transaction` | `commit start` when already started | `{"status": "error", "error": "already_in_transaction", "transaction": "existing_label"}` |
| `no_transaction` | `commit end` without `commit start` | `{"status": "error", "error": "no_transaction"}` |
| `label_mismatch` | `commit end X` doesn't match `commit start Y` | `{"status": "error", "error": "label_mismatch", "expected": "Y", "got": "X"}` |
| `peer_disconnected` | Peer disconnected during commit | `{"status": "error", "error": "peer_disconnected", "routes_lost": 5}` |

### Route Errors (within transaction)

Routes that fail validation are collected but don't abort the transaction:

```json
{
  "status": "partial",
  "updates_sent": 2,
  "routes_announced": 8,
  "routes_failed": 2,
  "failures": [
    {"route": "invalid/33", "error": "invalid prefix length"},
    {"route": "10.0.0.0/24", "error": "unknown attribute type 255"}
  ],
  "transaction": "batch1"
}
```

---

## Performance Considerations

### Memory

- Pending routes stored in memory until commit
- Large batches may use significant memory
- `max-batch-size` config limits memory usage

### Latency

| Mode | Latency | Use Case |
|------|---------|----------|
| Immediate (no transaction) | ~1ms per route | Interactive, low volume |
| Commit-based | Batch latency + 1ms | Bulk updates, tests |
| Auto-commit-delay | delay + batch + 1ms | Implicit batching |

### Throughput

With commit batching:
- 1000 routes → 1-10 UPDATE messages (vs 1000 without batching)
- Significant reduction in TCP overhead
- Better peer processing (fewer message boundaries)

---

## References

- ExaBGP group commands: `../src/exabgp/reactor/api/command/group.py`
- ExaBGP RIB batching: `../src/exabgp/rib/outgoing.py`
- ExaBGP test runner: `../main/qa/bin/functional`
- ZeBGP OutgoingRIB: `pkg/rib/outgoing.go`
- ZeBGP config schema: `pkg/config/bgp.go`
- ZeBGP self-check: `test/cmd/self-check/main.go`
- ZeBGP process manager: `pkg/api/process.go`
- Self-check system docs: `.claude/zebgp/SELF_CHECK_SYSTEM.md`
