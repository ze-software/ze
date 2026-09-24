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
| SIGTERM | Graceful shutdown | `cmd/ze/hub/main.go` |
| SIGINT | Graceful shutdown | Same as SIGTERM (Ctrl+C) |
| SIGHUP | Config reload | `handleSIGHUPReload` (`cmd/ze/hub/main_reload.go`), then `reactor.SignalHandler.OnReload` |
| SIGUSR1 | Status dump | `reactor.SignalHandler.OnStatus` |
| SIGQUIT | Goroutine dump + exit | Go runtime default (not caught -- useful for debugging) |
<!-- source: internal/component/bgp/reactor/signal.go -- handleSignal, SIGTERM/SIGINT/SIGHUP/SIGUSR1 -->
<!-- source: cmd/ze/hub/main.go -- runYANGConfig -->

### Daemon Liveness

Daemon liveness is detected by TCP dial to the SSH port. CLI tools (`ze signal stop`, `ze signal reload`, `ze signal status`) connect via SSH to send commands. No PID files or Unix sockets are used.
<!-- source: internal/plugins/signal/main.go -- Run, RunStatus, cmdSSHExec -->

### `ze signal` CLI

**Package:** `internal/plugins/signal/`

Usage: `ze signal <command>`

| Command | Mechanism | Exit 0 | Exit 1 |
|---------|-----------|--------|--------|
| reload | SSH exec `request reload` | Reload sent | Not running / SSH error |
| stop | SSH exec `stop` | Stop sent | Not running / SSH error |
| restart | SSH exec `restart` | Restart sent | Not running / SSH error |
| reboot | SSH exec `reboot` | Reboot sent | Not running / SSH error |
| status | SSH exec `show status` | Status returned | Not running / SSH error |
| quit | SSH exec `request halt` | Quit sent | Not running / SSH error |
<!-- source: internal/plugins/signal/main.go -- Commands registry, ExitSuccess, ExitNotRunning -->

### Startup Paths

There is one startup path, `runYANGConfig`, and every config takes it:

1. Register `stopCh` for SIGINT and SIGTERM, and `hupCh` for SIGHUP
2. Load config via the YANG parser
3. Create the plugin server
4. Start SSH server (binds configured listen addresses)
5. Start the plugin server, which starts the plugins
6. Wait for SIGTERM/SIGINT, or for the plugin server to report done

Step 1 comes first because an unregistered signal has the default disposition:
a SIGHUP or SIGTERM that arrives during config parsing kills the daemon with no
log line and no shutdown. Parsing and plugin construction take seconds on a
loaded host. Until 2026-09-24 the registration sat after them, and a test
trigger that sent SIGHUP a fixed delay after the daemon started killed the
daemon silently. Registered first, the signal waits in the buffer until startup
finishes.

A stop and a reload have separate channels, each one signal deep. os/signal
drops a signal that finds its channel full. With one shared channel, a SIGHUP
queued during startup made a SIGTERM that followed it the dropped signal: the
daemon reloaded, then ran on. On separate channels a dropped signal is only
ever a second copy of a queued signal of the same kind. A process still in Go
package initialization has registered nothing yet, so a SIGHUP in its first few
hundred milliseconds still kills it.

Step 1 also comes before step 5 because a plugin takes the process signal
disposition. Every plugin opens `sdk.SignalContext`, and an in-process plugin
registers SIGINT and SIGTERM for the whole process. A SIGTERM that arrived
between the first plugin and the hub's own registration was therefore neither
fatal nor delivered to the hub: the plugins exited and the daemon waited for a
second signal. The hub now registers first, and `waitLoop` drains the queued
signal when startup finishes.

`waitLoop` is the only reader of `stopCh` and `hupCh`, so it never blocks. It gives a SIGHUP
to the reload worker through a channel that holds one signal. When a SIGHUP is
already queued behind a running reload, `waitLoop` drops the new SIGHUP: the
queued reload reads the config source when it starts, so it applies every
edit. Before 2026-09-24 the send blocked, and a SIGTERM that arrived during a
slow or failing reload was not read until that reload returned. Each reload has
a 30s timeout.
<!-- source: cmd/ze/hub/main.go -- waitLoop -->

A SIGHUP reload can find another config transaction holding the lock, for
example a commit. The reload worker then queues the SIGHUP and waits for that
transaction to end, with success or failure. The end of the holder starts the
queued reload once, and the reload reads the config source again. SIGHUPs that
arrive during the wait join the queued reload. When shutdown begins during the
wait, the worker stops and the queued reload does not run. Before 2026-09-24
the queued reload ran only after a later SIGHUP completed its own reload, so
without that SIGHUP the edit was never applied and nothing reported it.
<!-- source: cmd/ze/hub/main_reload.go -- awaitConfigTransaction -->
<!-- source: internal/component/plugin/server/reload.go -- ConfigTransactionDone -->

The reactor runs its own `SignalHandler` only when it is standalone. Under the
hub, `externalServer` is true, the reactor skips `startSignalHandler`, and the
hub owns every signal.

A second path existed until 2026-08-12, `runOrchestratorWithData`, reached by a
config whose top-level blocks were only `plugin` and `env`. It parsed the config
with its own parser and handled its own signals. It is deleted.
<!-- source: cmd/ze/hub/main.go -- runYANGConfig, signal.Notify at the top of the function -->
<!-- source: pkg/plugin/sdk/signal.go -- SignalContext -->
<!-- source: internal/component/bgp/reactor/reactor.go -- startSignalHandler, guarded by !r.externalServer -->

---

**Last Updated:** 2026-09-13
