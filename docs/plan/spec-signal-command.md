# Spec: Signal Command and Connection Handoff

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/hub-architecture.md` - Hub plugin architecture
4. `docs/architecture/behavior/signals.md` - current signal handling
5. `internal/plugin/bgp/reactor/signal.go` - SignalHandler implementation
6. `cmd/ze/hub/main.go` - current Hub startup and signal handling

## Task

Implement `ze signal` CLI command for sending signals to running Ze instances, with:
1. PID file management for process discovery
2. Connection handoff from Hub to plugins via SCM_RIGHTS (systemd-style)
3. VyOS-style config reload with per-peer diff and action decision

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/hub-architecture.md` - Hub/plugin separation, 5-stage protocol
- [ ] `docs/architecture/behavior/signals.md` - current signal mapping and flow
- [ ] `docs/architecture/api/process-protocol.md` - plugin IPC protocol
- [ ] `docs/architecture/config/vyos-research.md` - VyOS verify/apply model (if exists)

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - BGP FSM, session lifecycle
- [ ] `rfc/short/rfc4724.md` - Graceful Restart (separate from reload)

**Key insights:**
- Hub already handles SIGHUP with TODO for config reload
- SignalHandler in reactor has callback-based design
- VyOS uses verify → apply with diff computation
- systemd passes sockets via `LISTEN_FDS` env + inherited fds

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/main.go` - Hub startup, basic SIGHUP handling (line 111-124)
- [ ] `internal/plugin/bgp/reactor/signal.go` - SignalHandler with OnReload callback
- [ ] `internal/plugin/bgp/reactor/reactor.go` - Reactor.Reload() method (if exists)
- [ ] `internal/plugin/subsystem.go` - Plugin IPC protocol

**Behavior to preserve:**
- SIGTERM/SIGINT → graceful shutdown (send NOTIFICATIONs, close connections)
- SIGHUP → reload callback (currently logged, not implemented)
- SIGUSR1 → status dump callback
- Plugin 5-stage protocol startup sequence

**Behavior to change:**
- Add PID file write on startup, remove on shutdown
- Add `ze signal` CLI command
- Implement SIGHUP config reload with VyOS-style diff
- Add connection handoff message type for Hub → Plugin

---

## Design

### 1. PID File Management

#### Location Strategy

Priority order (first accessible wins):

| Priority | Path | Condition |
|----------|------|-----------|
| 1 | `$XDG_RUNTIME_DIR/ze/<config-hash>.pid` | XDG_RUNTIME_DIR set and writable |
| 2 | `<config-dir>/<config-name>.pid` | Config directory writable |
| 3 | Error | Neither location writable |

**Config hash:** First 8 characters of SHA256 of absolute config path.

Example:
- Config: `/etc/ze/router.conf`
- Hash: `sha256("/etc/ze/router.conf")[:8]` → `a1b2c3d4`
- PID file: `$XDG_RUNTIME_DIR/ze/a1b2c3d4.pid` or `/etc/ze/router.pid`

#### PID File Format

| Line | Content | Example |
|------|---------|---------|
| 1 | Process ID | `12345` |
| 2 | Absolute config path | `/etc/ze/router.conf` |
| 3 | Start timestamp (RFC 3339) | `2026-01-31T10:30:00Z` |

#### Lifecycle

| Event | Action |
|-------|--------|
| Startup | Create parent dir, write PID file, acquire flock(LOCK_EX) |
| Running | Hold flock - prevents duplicate instances |
| Shutdown | Release flock, remove PID file |
| Crash | Stale file detected by failed flock (LOCK_NB) |

#### Stale PID Detection

When reading PID file:
1. Read PID from file
2. Try `flock(fd, LOCK_EX|LOCK_NB)` on the PID file
3. If lock succeeds → stale file (process dead), can overwrite
4. If lock fails (EWOULDBLOCK) → process running

### 2. `ze signal` Command

#### CLI Interface

```
ze signal <command> [options] <config>

Commands:
  reload    Send SIGHUP - reload configuration
  stop      Send SIGTERM - graceful shutdown
  quit      Send SIGQUIT - immediate shutdown
  status    Check if process is running (exit 0 = running, 1 = not)

Options:
  --pid-file <path>   Use explicit PID file instead of deriving from config
  --quiet             Suppress output (useful for scripts)

Arguments:
  <config>            Config file path (used to derive PID file location)
```

#### Examples

| Use Case | Command |
|----------|---------|
| Reload config | `ze signal reload /etc/ze/router.conf` |
| Graceful stop | `ze signal stop /etc/ze/router.conf` |
| Check status | `ze signal status /etc/ze/router.conf` |
| Explicit PID file | `ze signal reload --pid-file /run/ze/custom.pid` |

#### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (signal sent, or status=running) |
| 1 | Process not running (status command) |
| 2 | PID file not found |
| 3 | Permission denied |
| 4 | Signal delivery failed |

#### Implementation

```
ze signal reload config.conf
         │
         ├─ 1. Resolve config to absolute path
         │
         ├─ 2. Compute PID file path
         │     a. Try $XDG_RUNTIME_DIR/ze/<hash>.pid
         │     b. Fallback to <config-dir>/<name>.pid
         │
         ├─ 3. Read and parse PID file
         │     → Verify config path matches
         │
         ├─ 4. Check process exists
         │     → kill(pid, 0)
         │
         ├─ 5. Send signal
         │     → kill(pid, SIGHUP)
         │
         └─ 6. Exit 0
```

### 3. Connection Handoff (Systemd-Style)

#### Problem: Current Pipes Cannot Pass File Descriptors

The current plugin architecture uses **anonymous pipes** (`cmd.StdinPipe()`, `cmd.StdoutPipe()`).
Pipes are unidirectional byte streams that **do not support SCM_RIGHTS** (file descriptor passing).

| IPC Type | Can pass fd? | Current usage |
|----------|--------------|---------------|
| Anonymous pipe | No | stdin/stdout for plugins |
| Unix domain socket (SOCK_STREAM) | Yes (SCM_RIGHTS) | Not used |
| Unix domain socket (SOCK_DGRAM) | Yes (SCM_RIGHTS) | Not used |

**Conclusion:** To enable connection handoff, we must use **Unix domain sockets** instead of pipes.

#### Solution: Socketpair for Plugin Communication

Replace `pipe()` with `socketpair(AF_UNIX, SOCK_STREAM, 0)` for external plugin communication.

**Current architecture:**

| Direction | Mechanism | FD passing |
|-----------|-----------|------------|
| Hub → Plugin | pipe (stdin) | No |
| Plugin → Hub | pipe (stdout) | No |

**New architecture:**

| Direction | Mechanism | FD passing |
|-----------|-----------|------------|
| Hub ↔ Plugin | socketpair (bidirectional) | Yes |
| Plugin stderr | pipe (unchanged) | N/A |

**Why socketpair over named socket:**
- Created atomically with `socketpair()`
- No filesystem path needed
- Inherited by child via `fork()` (same as pipes)
- Bidirectional (can replace both stdin and stdout)

#### Two Handoff Modes

**Mode A: Listen Socket Handoff (like systemd socket activation)**

Hub creates the listen socket, passes it to plugin at startup. Plugin owns the listen socket and calls `accept()` itself.

| Step | Actor | Action |
|------|-------|--------|
| 1 | Hub | Create listen socket on port 179 |
| 2 | Hub | Fork plugin, pass listen fd via SCM_RIGHTS |
| 3 | Plugin | Receive fd, convert to `*net.TCPListener` |
| 4 | Plugin | Call `Accept()` on listener |
| 5 | Plugin | Handle connections directly |

**Pros:** Simple, plugin has full control
**Cons:** Only works at startup, cannot re-route connections

**Mode B: Per-Connection Handoff**

Hub owns the listen socket, accepts connections, passes each connection fd to plugin.

| Step | Actor | Action |
|------|-------|--------|
| 1 | Hub | Listen on port 179, call `Accept()` |
| 2 | Hub | Route connection based on peer IP |
| 3 | Hub | Send fd via SCM_RIGHTS to appropriate plugin |
| 4 | Plugin | Receive fd, convert to `*net.TCPConn` |
| 5 | Plugin | Handle connection (BGP session) |

**Pros:** Hub can route to different plugins, hot-swap possible
**Cons:** More overhead per connection

**Recommendation:** Implement **Mode A** first (simpler), add Mode B later if needed.

#### Protocol: Declaring Connection Handler

Plugins that handle connections declare this in Stage 1 (Declaration):

| Declaration | Meaning |
|-------------|---------|
| `declare connection-handler listen <port>` | Plugin wants listen socket for this port |
| `declare connection-handler accept` | Plugin accepts per-connection handoff |

#### Startup Flow with Listen Socket Handoff (Mode A)

| Phase | Hub Action | Plugin Action |
|-------|------------|---------------|
| 1 | Parse config, identify listeners needed | - |
| 2 | Create listen socket(s) | - |
| 3 | Fork plugin with socketpair for IPC | Start, read from socket |
| 4 | Wait for `declare connection-handler listen 179` | Send declaration |
| 5 | Send listen fd via SCM_RIGHTS | - |
| 6 | - | Receive fd, store as listener |
| 7 | Send `connection-handler ready` | - |
| 8 | Continue 5-stage protocol | Continue protocol |
| 9 | - | Start `Accept()` loop |

#### Message Sequence (Mode A)

**Plugin → Hub (Stage 1 declaration):**

`declare connection-handler listen 179`

**Hub → Plugin (after declaration, before config stage):**

`#<serial> connection-handler fd port 179`

(fd passed out-of-band via SCM_RIGHTS ancillary data)

**Plugin → Hub (acknowledgment):**

`@<serial> done`

#### Changes to `internal/plugin/process.go`

**`startExternal()` modifications:**

| Current | New |
|---------|-----|
| Create stdin pipe | Create socketpair |
| Create stdout pipe | Use same socketpair (bidirectional) |
| `cmd.Stdin = stdinRead` | `cmd.ExtraFiles = []*os.File{childEnd}` |
| `cmd.Stdout = stdoutWrite` | Plugin reads/writes fd 3 |
| Write to `p.stdin` | Write to `p.socket` |
| Read from `p.stdout` | Read from `p.socket` |

**New method: `SendFD(fd int)`**

| Step | Operation |
|------|-----------|
| 1 | Build SCM_RIGHTS control message with fd |
| 2 | Call `WriteMsgUnix()` on socketpair |
| 3 | Close local copy of fd after send |

**New method: `ReceiveFD()` (for plugin side)**

| Step | Operation |
|------|-----------|
| 1 | Call `ReadMsgUnix()` with OOB buffer |
| 2 | Parse control message via `ParseSocketControlMessage()` |
| 3 | Extract fd via `ParseUnixRights()` |
| 4 | Return fd (caller converts to net.Conn or net.Listener) |

#### Go Socketpair Creation

Using `golang.org/x/sys/unix`:

| Step | Function | Result |
|------|----------|--------|
| 1 | `unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)` | Returns [2]int{parentFd, childFd} |
| 2 | `os.NewFile(uintptr(parentFd), "socket")` | Parent *os.File |
| 3 | `os.NewFile(uintptr(childFd), "socket")` | Child *os.File (passed to cmd.ExtraFiles) |
| 4 | `net.FileConn(parentFile)` | *net.UnixConn for read/write |

#### Plugin Side: Receiving Listen Socket

Plugin startup (in plugin code, not Hub):

| Step | Operation |
|------|-----------|
| 1 | Open fd 3 (passed via ExtraFiles) |
| 2 | Send `declare connection-handler listen 179` |
| 3 | Read message: `#N connection-handler fd port 179` |
| 4 | Call `ReadMsgUnix()` with OOB buffer to get listen fd |
| 5 | Convert: `os.NewFile(fd)` → `net.FileListener()` → `*net.TCPListener` |
| 6 | Send `@N done` |
| 7 | Start `Accept()` loop on listener |

#### Environment Variable Alternative (Systemd Compatibility)

For compatibility with systemd-style activation, also support `LISTEN_FDS` environment variable:

| Env Var | Meaning |
|---------|---------|
| `LISTEN_FDS=1` | Number of listen fds passed |
| `LISTEN_FDNAMES=bgp` | Comma-separated names for fds |
| fds start at 3 | fd 3 = first listen socket |

This allows plugins to work both with Ze and with systemd socket activation.

#### Backward Compatibility

Existing plugins continue to work:

| Plugin Type | IPC Mechanism | Connection Handling |
|-------------|---------------|---------------------|
| Old plugin (no declare) | Pipe (unchanged) | N/A |
| New plugin with `declare connection-handler` | Socketpair | Receives listen fd |
| Internal plugin | io.Pipe (unchanged) | Direct net.Conn |

**Detection:** If plugin sends `declare connection-handler`, Hub uses socketpair.
Otherwise, Hub uses traditional pipes for backward compatibility.

### 4. VyOS-Style Config Reload

#### Philosophy

Instead of "restart everything on reload", compute the minimal diff and take per-peer action:

| Change | Action |
|--------|--------|
| New peer | Add peer, start FSM |
| Removed peer | Send NOTIFICATION (Cease/Admin Shutdown), remove |
| Peer unchanged | Do nothing |
| Peer config changed | Evaluate what changed (see below) |

#### Per-Peer Change Evaluation

| What Changed | Action |
|--------------|--------|
| remote-as | Restart session (fundamental change) |
| local-as | Restart session |
| router-id | Restart session |
| capabilities | Restart session (needs re-negotiation) |
| hold-time | Restart session |
| families | Restart session |
| passive | Update in-place (affects connect behavior) |
| description | Update in-place (no protocol impact) |
| disabled | If now disabled: send NOTIFICATION, stop FSM. If now enabled: start FSM |
| timers (keepalive) | Update in-place for next interval |
| static routes | Announce/withdraw as needed |

#### Reload Flow

```
SIGHUP received
       │
       ▼
┌──────────────────────────────────────────────────────────────────────┐
│ 1. Parse new config file                                             │
│    - Full YANG validation                                            │
│    - If parse fails → log error, keep running config, done           │
└──────────────────────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────────────────────┐
│ 2. Compute peer diff                                                 │
│    - Build map: peer-address → PeerSettings (running)                │
│    - Build map: peer-address → PeerSettings (new)                    │
│    - Categorize: added, removed, unchanged, changed                  │
└──────────────────────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────────────────────┐
│ 3. For each removed peer:                                            │
│    - Log: "peer <ip> removed from config"                            │
│    - Send NOTIFICATION (Cease, Admin Shutdown)                       │
│    - Stop FSM, remove from peer map                                  │
└──────────────────────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────────────────────┐
│ 4. For each changed peer:                                            │
│    - Compute field diff                                              │
│    - If session-breaking change → restart session                    │
│    - If in-place update possible → update and continue               │
└──────────────────────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────────────────────┐
│ 5. For each added peer:                                              │
│    - Log: "peer <ip> added from config"                              │
│    - Create peer, start FSM                                          │
└──────────────────────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────────────────────┐
│ 6. Notify plugins                                                    │
│    - Send "config reload" event                                      │
│    - Plugins can query new config via IPC                            │
└──────────────────────────────────────────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────────────────────────────────┐
│ 7. Update running config reference                                   │
│    - Replace stored config with new                                  │
│    - Log: "config reloaded, N peers added, M removed, K changed"     │
└──────────────────────────────────────────────────────────────────────┘
```

#### Diff Data Structure

| Field | Type | Description |
|-------|------|-------------|
| Added | list of PeerSettings | Peers in new config, not in running |
| Removed | list of PeerSettings | Peers in running, not in new config |
| Changed | list of PeerChange | Peers in both, with differences |
| Unchanged | list of string | Peer addresses with no changes |

**PeerChange:**

| Field | Type | Description |
|-------|------|-------------|
| Address | string | Peer address |
| Old | PeerSettings | Running config |
| New | PeerSettings | New config |
| RequiresRestart | bool | True if session must restart |
| ChangedFields | list of string | Which fields differ |

---

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPIDFileLocation` | `internal/pidfile/pidfile_test.go` | XDG fallback logic | |
| `TestPIDFileCreate` | `internal/pidfile/pidfile_test.go` | Create, flock, content format | |
| `TestPIDFileStaleDetection` | `internal/pidfile/pidfile_test.go` | Detect dead process via flock | |
| `TestSignalCommandParsing` | `cmd/ze/signal/main_test.go` | CLI argument parsing | |
| `TestPeerDiffEmpty` | `internal/config/diff/diff_test.go` | No changes detected correctly | |
| `TestPeerDiffAdded` | `internal/config/diff/diff_test.go` | New peer detection | |
| `TestPeerDiffRemoved` | `internal/config/diff/diff_test.go` | Removed peer detection | |
| `TestPeerDiffChanged` | `internal/config/diff/diff_test.go` | Changed fields detection | |
| `TestPeerDiffRequiresRestart` | `internal/config/diff/diff_test.go` | Session-breaking changes | |
| `TestSocketpairCreate` | `internal/plugin/socketpair_test.go` | Create socketpair, verify bidirectional | |
| `TestSocketpairSendFD` | `internal/plugin/socketpair_test.go` | Send fd via SCM_RIGHTS | |
| `TestSocketpairReceiveFD` | `internal/plugin/socketpair_test.go` | Receive fd, verify usable | |
| `TestSocketpairSendListenFD` | `internal/plugin/socketpair_test.go` | Pass listen socket, accept works | |
| `TestProcessSocketpairMode` | `internal/plugin/process_test.go` | External plugin uses socketpair | |
| `TestDeclareConnectionHandler` | `internal/plugin/registration_test.go` | Parse connection-handler declaration | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PID | 1-4194304 | 4194304 | 0 | N/A (kernel limit) |
| Signal number | 1-31 | 31 | 0 | 32 |
| Port (handoff) | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `signal-reload` | `test/signal/reload.ci` | Send SIGHUP, verify config changes applied | |
| `signal-stop` | `test/signal/stop.ci` | Send SIGTERM, verify graceful shutdown | |
| `signal-status-running` | `test/signal/status-running.ci` | Check running process returns 0 | |
| `signal-status-stopped` | `test/signal/status-stopped.ci` | Check non-existent returns 1 | |
| `pid-file-created` | `test/signal/pid-file.ci` | Verify PID file created on startup | |
| `pid-file-removed` | `test/signal/pid-cleanup.ci` | Verify PID file removed on shutdown | |
| `reload-add-peer` | `test/reload/add-peer.ci` | Add peer via reload, verify session starts | |
| `reload-remove-peer` | `test/reload/remove-peer.ci` | Remove peer via reload, verify NOTIFICATION sent | |

### Future (if deferring any tests)
- Connection handoff tests require forked plugin mode (Phase 4 of Hub architecture)
- Full integration tests with actual BGP peer

---

## Files to Modify

### Phase 1-4 (PID, Signal, Diff, Reload)

- `cmd/ze/main.go` - Add `signal` command dispatch
- `cmd/ze/hub/main.go` - Integrate PID file, call reload on SIGHUP
- `internal/plugin/bgp/reactor/reactor.go` - Implement Reload() with diff logic

### Phase 5 (Connection Handoff)

- `internal/plugin/process.go` - Replace pipes with socketpair for external plugins
  - `startExternal()`: use `unix.Socketpair()` instead of `cmd.StdinPipe()/StdoutPipe()`
  - Add `SendFD(fd int) error` method for SCM_RIGHTS
  - Add `socket *net.UnixConn` field for bidirectional IPC
  - Modify `readLines()` to read from socket
  - Modify `writeLoop()` to write to socket
- `internal/plugin/registration.go` - Parse `declare connection-handler listen <port>`
- `internal/plugin/types.go` - Add `ConnectionHandler` to `PluginRegistration`
- `internal/hub/orchestrator.go` - Create listen sockets, pass to plugins

## Files to Create

### Phase 1-4

- `cmd/ze/signal/main.go` - `ze signal` CLI command
- `internal/pidfile/pidfile.go` - PID file management
- `internal/pidfile/pidfile_test.go` - PID file tests
- `internal/config/diff/diff.go` - Peer config diff computation
- `internal/config/diff/diff_test.go` - Diff tests

### Phase 5 (Connection Handoff)

- `internal/plugin/socketpair.go` - Socketpair creation and fd passing utilities
- `internal/plugin/socketpair_test.go` - Socketpair and SCM_RIGHTS tests
- `pkg/fdpass/fdpass.go` - Public library for fd passing (for external plugins to import)
- `test/signal/reload.ci` - Reload functional test
- `test/signal/stop.ci` - Stop functional test
- `test/reload/add-peer.ci` - Add peer via reload test
- `test/reload/remove-peer.ci` - Remove peer via reload test

---

## Implementation Steps

### Phase 1: PID File Management

1. **Write unit tests** - PID file create, read, stale detection
   → **Review:** Are edge cases covered? XDG fallback logic?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Do tests fail for the RIGHT reason?

3. **Implement `internal/pidfile/pidfile.go`**
   - `type Manager struct`
   - `func New(configPath string) (*Manager, error)`
   - `func (m *Manager) Acquire() error` - create + flock
   - `func (m *Manager) Release() error` - unlock + remove
   - `func (m *Manager) ReadPID() (int, error)` - read existing
   - `func (m *Manager) IsRunning() (bool, int, error)` - check via flock
   → **Review:** Is this the simplest solution?

4. **Run tests** - Verify PASS (paste output)

5. **Integrate into Hub startup** - `cmd/ze/hub/main.go`
   - Acquire PID file before starting reactor/orchestrator
   - Release on shutdown

### Phase 2: `ze signal` Command

1. **Write unit tests** - CLI parsing, signal mapping
2. **Run tests** - Verify FAIL
3. **Implement `cmd/ze/signal/main.go`**
   - Parse args (reload, stop, quit, status)
   - Use pidfile.Manager to find PID
   - Send signal via syscall.Kill
4. **Run tests** - Verify PASS
5. **Add to `cmd/ze/main.go`** - dispatch to signal.Run()
6. **Write functional tests** - `test/signal/*.ci`

### Phase 3: Config Diff

1. **Write unit tests** - Diff computation, requires-restart detection
2. **Run tests** - Verify FAIL
3. **Implement `internal/config/diff/diff.go`**
   - `type Diff struct` (Added, Removed, Changed, Unchanged)
   - `type PeerChange struct`
   - `func ComputePeerDiff(running, new []*PeerSettings) *Diff`
   - `func RequiresRestart(old, new *PeerSettings) bool`
4. **Run tests** - Verify PASS

### Phase 4: Reload Implementation

1. **Implement Reactor.Reload()**
   - Re-read config file
   - Compute diff
   - Apply changes per-peer
2. **Wire SIGHUP to Reload()** in Hub
3. **Write functional tests** - `test/reload/*.ci`
4. **Run full test suite**

### Phase 5: Connection Handoff (Requires Hub Mode)

**5a. Socketpair Infrastructure**

1. **Write unit tests** - `internal/plugin/socketpair_test.go`
   - TestSocketpairCreate: verify bidirectional communication
   - TestSocketpairSendFD: send fd, verify received
   - TestSocketpairSendListenFD: pass TCPListener, verify Accept() works
2. **Run tests** - Verify FAIL
3. **Implement `internal/plugin/socketpair.go`**
   - `func CreateSocketpair() (parent, child *os.File, err error)`
   - `func SendFD(conn *net.UnixConn, fd int, msg []byte) error`
   - `func ReceiveFD(conn *net.UnixConn) (fd int, msg []byte, err error)`
4. **Run tests** - Verify PASS

**5b. Process Socketpair Mode**

1. **Write unit tests** - `internal/plugin/process_test.go`
   - TestProcessSocketpairMode: external plugin uses socketpair
2. **Modify `startExternal()` in `process.go`**
   - Create socketpair instead of pipes for stdin/stdout
   - Pass child end via `cmd.ExtraFiles = []*os.File{childEnd}`
   - Plugin reads/writes fd 3 (first ExtraFile)
   - Add `socket *net.UnixConn` field
   - Modify readLines/writeLoop to use socket
3. **Run tests** - Verify PASS

**5c. Connection Handler Protocol**

1. **Write unit tests** - `internal/plugin/registration_test.go`
   - TestDeclareConnectionHandler: parse declaration
2. **Add to `registration.go`**
   - Parse `declare connection-handler listen <port>`
   - Store in PluginRegistration.ConnectionHandlers
3. **Add to `types.go`**
   - `ConnectionHandler struct { Type string, Port int }`
   - Add to PluginRegistration
4. **Run tests** - Verify PASS

**5d. Hub Listen Socket Handoff**

1. **Modify `internal/hub/orchestrator.go`**
   - Create listen sockets for declared ports
   - After Stage 1 (Declaration), before Stage 2 (Config)
   - Send listen fd via SCM_RIGHTS
   - Wait for `@serial done` acknowledgment
2. **Write functional tests**
   - Plugin receives listen socket
   - Plugin can accept connections
3. **Run full test suite**

---

## RFC Documentation

### Reference Comments
- `// RFC 4271 Section 8.1.2` - FSM Administrative Events (shutdown on config remove)
- `// RFC 4724` - Graceful Restart (separate mechanism, not part of this spec)

### Constraint Comments

When removing a peer via config reload, add a comment above the NOTIFICATION send:

| RFC | Section | Requirement |
|-----|---------|-------------|
| RFC 4271 | 6.8 | "A NOTIFICATION message with Error Code Cease SHOULD be sent if the BGP peer is going down as a result of the BGP speaker wanting to terminate the connection" |

This constraint ensures we send Cease notification when removing peers.

---

## Implementation Summary

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered during implementation]

### Design Insights
- [Key learnings that should be documented elsewhere]

### Deviations from Plan
- [Any differences from original plan and why]

---

## Checklist

### 🏗️ Design
- [x] No premature abstraction (3+ concrete use cases exist?)
- [x] No speculative features (is this needed NOW?)
- [x] Single responsibility (each component does ONE thing?)
- [x] Explicit behavior (no hidden magic or conventions?)
- [x] Minimal coupling (components isolated, dependencies minimal?)
- [x] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs
- [ ] Feature code integrated into codebase
- [ ] Functional tests verify end-user behavior

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC summaries read
- [ ] RFC references added to code
- [ ] RFC constraint comments added

### Completion
- [ ] Architecture docs updated with learnings
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
