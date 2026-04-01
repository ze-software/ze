# Signal Handling

**Source:** ExaBGP `reactor/interrupt.py`, `reactor/loop.py`
**Purpose:** Document signal handling behavior

---

## Supported Signals

| Signal | Action | Description |
|--------|--------|-------------|
| SIGTERM | Shutdown | Graceful shutdown |
| SIGHUP | Shutdown | Graceful shutdown (same as SIGTERM) |
| SIGALRM | Restart | Restart all peers |
| SIGUSR1 | Reload | Reload configuration |
| SIGUSR2 | Full Reload | Full configuration reload |
| SIGINT | Shutdown | Ctrl+C, immediate shutdown |
<!-- source: internal/component/bgp/reactor/signal.go -- SignalHandler, SIGTERM/SIGINT/SIGHUP/SIGUSR1 -->

---

## Signal Flow

```
Signal Received
       │
       ▼
┌─────────────────┐
│ Signal Handler  │
│ (sets flag)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Reactor Loop    │
│ (checks flag)   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Execute Action  │
└─────────────────┘
```

---

## Signal Values

```python
class Signal:
    NONE = 0          # No signal pending
    SHUTDOWN = -1     # Graceful shutdown
    RESTART = -2      # Restart peers
    RELOAD = -4       # Reload config
    FULL_RELOAD = -8  # Full reload
```

---

## SIGTERM / SIGHUP (Shutdown)

### Behavior

1. Set `received = SHUTDOWN`
2. Reactor loop detects flag
3. Close all peer connections gracefully
4. Send NOTIFICATION (Cease/Admin Shutdown)
5. Terminate external processes
6. Exit with code 0

### Code

```python
def sigterm(self, signum, frame):
    log.critical('signal.received signal=SIGTERM', 'reactor')
    if self.received:
        log.critical('signal.ignored reason=handling_previous', 'reactor')
        return
    self.received = self.SHUTDOWN
    self.number = signum
```

---

## SIGALRM (Restart)

### Behavior

1. Set `received = RESTART`
2. Close all peer connections
3. Re-read configuration (same file)
4. Reconnect all peers

### Use Case

- Restart peers without full process restart
- Triggered by: `kill -ALRM <pid>`

---

## SIGUSR1 (Reload)

### Behavior

1. Set `received = RELOAD`
2. Parse configuration file
3. Compare with running config
4. Add new neighbors
5. Remove deleted neighbors
6. Update changed neighbors (restart those peers)

### Code

```python
def sigusr1(self, signum, frame):
    log.critical('signal.received signal=SIGUSR1', 'reactor')
    if self.received:
        log.critical('signal.ignored reason=handling_previous', 'reactor')
        return
    self.received = self.RELOAD
    self.number = signum
```

### Graceful Update

- New routes announced
- Removed routes withdrawn
- Changed peers restarted

---

## SIGUSR2 (Full Reload)

### Behavior

1. Set `received = FULL_RELOAD`
2. Close all connections
3. Re-read configuration completely
4. Restart all peers from scratch

### Use Case

- When incremental reload fails
- Complete state reset

---

## Signal Queuing

Only one signal processed at a time:

```python
def sigterm(self, signum, frame):
    if self.received:
        log.critical('signal.ignored reason=handling_previous')
        return
    self.received = self.SHUTDOWN
```

If a signal arrives while another is being processed, it's ignored.
<!-- source: internal/component/bgp/reactor/signal.go -- safeHandleSignal, handleSignal -->

---

## Signal Rearm

After processing, signals are re-armed:

```python
def rearm(self):
    self.received = Signal.NONE
    self.number = 0

    signal.signal(signal.SIGTERM, self.sigterm)
    signal.signal(signal.SIGHUP, self.sighup)
    signal.signal(signal.SIGALRM, self.sigalrm)
    signal.signal(signal.SIGUSR1, self.sigusr1)
    signal.signal(signal.SIGUSR2, self.sigusr2)
```

---

## API Notification

When SIGTERM/SIGHUP received, shutdown notification sent:

```json
{
  "exabgp": "6.0.0",
  "type": "notification",
  "notification": "shutdown"
}
```

When signal received for neighbor:

```json
{
  "exabgp": "6.0.0",
  "type": "signal",
  "neighbor": { ... },
  "code": 15,
  "name": "SIGTERM"
}
```

---

## Graceful Shutdown

On SIGTERM:

1. Stop accepting new connections
2. For each peer:
   - Send NOTIFICATION (Cease, code 6, subcode 2: Admin Shutdown)
   - Wait for ACK or timeout
   - Close TCP connection
3. Terminate external processes
4. Exit

```python
def shutdown(self):
    for peer in self.peers:
        peer.notify(Notification.CEASE, Notification.ADMIN_SHUTDOWN)
        peer.close()
    self.processes.terminate_all()
    sys.exit(0)
```
<!-- source: internal/component/bgp/reactor/signal.go -- SignalHandler.OnShutdown -->
<!-- source: internal/component/bgp/message/notification.go -- NOTIFICATION Cease codes -->

---

## Ze Implementation

Ze diverges from ExaBGP's signal mapping. The following reflects the actual implementation.

### Ze Signal Mapping

| Signal | Action | Handler |
|--------|--------|---------|
| SIGTERM | Graceful shutdown | `cmd/ze/hub/main.go` — both BGP and orchestrator paths |
| SIGINT | Graceful shutdown | Same as SIGTERM (Ctrl+C) |
| SIGHUP | Config reload | `reactor.SignalHandler.OnReload` (BGP path); `Orchestrator.Reload` (hub path — shuts down on failure) |
| SIGUSR1 | Status dump | `reactor.SignalHandler.OnStatus` (BGP path only) |
| SIGQUIT | Goroutine dump + exit | Go runtime default (not caught -- useful for debugging) |
<!-- source: internal/component/bgp/reactor/signal.go -- handleSignal, SIGTERM/SIGINT/SIGHUP/SIGUSR1 -->
<!-- source: cmd/ze/hub/main.go -- runBGPInProcess, runOrchestratorWithData -->

### Daemon Liveness

Daemon liveness is detected by TCP dial to the SSH port. CLI tools (`ze signal stop`, `ze signal reload`, `ze signal status`) connect via SSH to send commands. No PID files or Unix sockets are used.
<!-- source: cmd/ze/signal/main.go -- Run, cmdSignalReload, cmdSignalStop -->

### `ze signal` CLI

**Package:** `cmd/ze/signal/`

Usage: `ze signal <command>`

| Command | Mechanism | Exit 0 | Exit 1 |
|---------|-----------|--------|--------|
| reload | SSH exec `daemon reload` | Reload sent | Not running / SSH error |
| stop | SSH exec `stop` | Stop sent | Not running / SSH error |
| restart | SSH exec `restart` | Restart sent | Not running / SSH error |
| status | SSH exec `daemon status` | Status returned | Not running / SSH error |
| quit | SSH exec `daemon quit` | Quit sent | Not running / SSH error |
<!-- source: cmd/ze/signal/main.go -- Commands registry, ExitSuccess, ExitNotRunning -->

### Startup Paths

**BGP in-process** (`runBGPInProcess`):
1. Load config via YANG parser
2. Start SSH server (binds configured listen addresses)
3. Start reactor with `SignalHandler` (handles SIGHUP/SIGUSR1)
4. Wait for SIGTERM/SIGINT or reactor done
<!-- source: cmd/ze/hub/main.go -- runBGPInProcess -->

**Hub orchestrator** (`runOrchestratorWithData`):
1. Parse hub config
2. Start SSH server
3. Start orchestrator
4. Signal goroutine handles SIGTERM/SIGINT/SIGHUP
<!-- source: cmd/ze/hub/main.go -- runOrchestratorWithData -->
<!-- source: internal/component/bgp/reactor/signal.go -- SignalHandler.StartWithContext -->

---

**Last Updated:** 2026-02-11
