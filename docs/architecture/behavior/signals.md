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

---

## Ze Implementation Notes

### Signal Package

```go
package signal

type Signal int

const (
    None       Signal = 0
    Shutdown   Signal = -1
    Restart    Signal = -2
    Reload     Signal = -4
    FullReload Signal = -8
)

type Handler struct {
    received Signal
    number   int
    ch       chan os.Signal
}

func NewHandler() *Handler {
    h := &Handler{
        ch: make(chan os.Signal, 1),
    }
    signal.Notify(h.ch, syscall.SIGTERM, syscall.SIGHUP,
        syscall.SIGALRM, syscall.SIGUSR1, syscall.SIGUSR2)
    go h.listen()
    return h
}

func (h *Handler) listen() {
    for sig := range h.ch {
        switch sig {
        case syscall.SIGTERM, syscall.SIGHUP:
            h.received = Shutdown
        case syscall.SIGALRM:
            h.received = Restart
        case syscall.SIGUSR1:
            h.received = Reload
        case syscall.SIGUSR2:
            h.received = FullReload
        }
    }
}
```

### Reactor Integration

```go
func (r *Reactor) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return r.shutdown()

        default:
            switch r.signal.Received() {
            case signal.Shutdown:
                return r.shutdown()
            case signal.Restart:
                r.restartPeers()
                r.signal.Rearm()
            case signal.Reload:
                r.reloadConfig()
                r.signal.Rearm()
            case signal.FullReload:
                r.fullReload()
                r.signal.Rearm()
            }

            r.runPeers()
        }
    }
}
```

---

**Last Updated:** 2025-12-19
