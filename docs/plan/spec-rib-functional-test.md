# Spec: RIB Plugin Functional Test

## Task

Create a functional test program that tests the RIB plugin in isolation by simulating ZeBGP and verifying plugin behavior.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ARCHITECTURE.md` - Plugin communication protocol

### Source Code
- [ ] `pkg/plugin/rib/rib.go` - RIB plugin implementation
- [ ] `pkg/plugin/rib/event.go` - Event parsing

**Key insights:**
- Plugin communicates via stdin/stdout JSON
- 5-stage startup protocol (declare/config/capability/registry/ready)
- Events: sent, update, state, request
- Responses: commands and @serial responses

## Architecture

```
┌─────────────────┐     stdin      ┌─────────────────┐
│   Test Runner   │ ─────────────► │   RIB Plugin    │
│  (simulates     │                │  (subprocess)   │
│   ZeBGP)        │ ◄───────────── │                 │
└─────────────────┘     stdout     └─────────────────┘
```

## 🧪 TDD Test Plan

### Unit Tests

N/A - This is a functional test program, not unit tests.

### Functional Tests

| Test | Scenario | Validates |
|------|----------|-----------|
| `startup_protocol` | 5-stage handshake | Plugin declares commands, completes startup |
| `status_empty` | Status with no routes | Zero counts returned |
| `populate_ribout` | Send "sent" events | Routes stored in ribOut |
| `populate_ribin` | Send "update" events | Routes stored in ribIn |
| `status_populated` | Status after population | Correct route counts |
| `inbound_show_all` | Selector `*` | All peers' routes returned |
| `inbound_show_specific` | Selector `10.0.0.1` | Only that peer's routes |
| `inbound_show_negation` | Selector `!10.0.0.1` | That peer excluded |
| `inbound_show_multi_ip` | Selector `10.0.0.1,10.0.0.2` | Both peers included |
| `outbound_show_specific` | Selector `10.0.0.1` | Only that peer's routes |
| `inbound_clear` | Clear specific peer | Routes removed, others intact |
| `outbound_resend` | Resend to up peer | Routes sent, NO "session api ready" |
| `outbound_resend_down` | Resend to down peer | No routes sent |
| `peer_reconnect` | State up event | Routes replayed, YES "session api ready" |
| `withdrawal_handling` | Withdraw event | Route removed from ribOut |
| `unknown_command` | Invalid command | Error response |

## Files to Create

| File | Purpose |
|------|---------|
| `test/cmd/rib-functional/main.go` | Test runner entry point |
| `test/cmd/rib-functional/tester.go` | RIBTester struct and methods |
| `test/cmd/rib-functional/scenarios.go` | Test scenario definitions |

## Implementation Design

### RIBTester Struct

```go
type RIBTester struct {
    cmd      *exec.Cmd
    stdin    io.WriteCloser
    stdout   *bufio.Scanner
    serial   int
    output   []string  // Collected output lines
}

// Lifecycle
func NewRIBTester() *RIBTester
func (t *RIBTester) Start() error
func (t *RIBTester) Stop()

// Protocol
func (t *RIBTester) DoStartup() error
func (t *RIBTester) SendLine(line string) error
func (t *RIBTester) SendJSON(v any) error
func (t *RIBTester) ReadLines(timeout time.Duration) []string
func (t *RIBTester) WaitFor(prefix string, timeout time.Duration) (string, error)

// Events
func (t *RIBTester) SendSent(peer, family, prefix, nexthop string, msgID uint64) error
func (t *RIBTester) SendUpdate(peer, family, prefix, nexthop string, msgID uint64) error
func (t *RIBTester) SendWithdraw(peer, family, prefix string) error
func (t *RIBTester) SendState(peer, state string) error

// Commands
func (t *RIBTester) Request(command, selector string) (Response, error)
```

### Test Runner Structure

```go
func main() {
    tests := []struct {
        name string
        fn   func(*RIBTester) error
    }{
        {"startup_protocol", testStartup},
        {"status_empty", testStatusEmpty},
        // ...
    }

    tester := NewRIBTester()
    if err := tester.Start(); err != nil {
        log.Fatal(err)
    }
    defer tester.Stop()

    passed, failed := 0, 0
    for _, test := range tests {
        start := time.Now()
        err := test.fn(tester)
        duration := time.Since(start)
        if err != nil {
            fmt.Printf("✗ %s (%v): %v\n", test.name, duration, err)
            failed++
        } else {
            fmt.Printf("✓ %s (%v)\n", test.name, duration)
            passed++
        }
    }
    fmt.Printf("\n%d passed, %d failed\n", passed, failed)
}
```

## Test Scenarios Detail

### 1. Startup Protocol

```
WAIT: declare cmd rib adjacent status
WAIT: declare cmd rib adjacent inbound show
WAIT: declare cmd rib adjacent inbound clear
WAIT: declare cmd rib adjacent outbound show
WAIT: declare cmd rib adjacent outbound resend
WAIT: declare done
SEND: config done
WAIT: capability done
SEND: registry done
WAIT: ready
```

### 2. Populate RIB-Out

```json
{"type":"sent","msg-id":100,"peer":{"address":"10.0.0.1","asn":65001},
 "announce":{"ipv4/unicast":{"1.1.1.1":["10.0.0.0/24","10.0.1.0/24"]}}}

{"type":"sent","msg-id":101,"peer":{"address":"10.0.0.2","asn":65002},
 "announce":{"ipv4/unicast":{"2.2.2.2":["10.0.2.0/24"]}}}
```

### 3. Populate RIB-In

```json
{"type":"update","message":{"type":"update","id":200},
 "peer":{"address":{"local":"10.0.0.100","peer":"10.0.0.1"},
         "asn":{"local":65000,"peer":65001}},
 "announce":{"ipv4/unicast":{"1.1.1.1":["20.0.0.0/24"]}}}
```

### 4. Request Commands

```json
{"type":"request","serial":"1","command":"rib adjacent status","peer":"*"}
{"type":"request","serial":"2","command":"rib adjacent inbound show","peer":"10.0.0.1"}
{"type":"request","serial":"3","command":"rib adjacent inbound clear","peer":"10.0.0.1"}
{"type":"request","serial":"4","command":"rib adjacent outbound show","peer":"!10.0.0.2"}
{"type":"request","serial":"5","command":"rib adjacent outbound resend","peer":"10.0.0.1,10.0.0.2"}
```

### 5. State Events

```json
{"type":"state","peer":{"address":"10.0.0.1","asn":65001},"state":"up"}
{"type":"state","peer":{"address":"10.0.0.1","asn":65001},"state":"down"}
```

## Critical Verification Points

| Scenario | Must Verify |
|----------|-------------|
| Resend command | NO "session api ready" in output |
| State up (reconnect) | YES "session api ready" in output |
| Selector `*` | All peers included |
| Selector `IP` | Only that peer |
| Selector `!IP` | That peer excluded |
| Selector `IP,IP` | Both peers included |
| Clear | Routes actually removed (verify with show) |
| Withdrawal | Route removed from ribOut |
| Down peer | Resend skipped (resent=0) |

## Implementation Steps

1. **Create test harness** - RIBTester struct with process management
2. **Implement startup** - 5-stage protocol handling
3. **Implement event helpers** - SendSent, SendUpdate, SendState, etc.
4. **Implement request helper** - Send request, parse response
5. **Write test scenarios** - One function per scenario
6. **Run and verify** - All scenarios pass

## Verification Commands

```bash
# Build RIB plugin
go build -o /tmp/rib-plugin ./pkg/plugin/rib

# Run functional tests
go run ./test/cmd/rib-functional

# Or via Makefile (after adding target)
make rib-functional
```

## Expected Output

```
=== RIB Plugin Functional Tests ===
✓ startup_protocol (15ms)
✓ status_empty (2ms)
✓ populate_ribout (3ms)
✓ populate_ribin (2ms)
✓ status_populated (2ms)
✓ inbound_show_all (3ms)
✓ inbound_show_specific (2ms)
✓ inbound_show_negation (2ms)
✓ inbound_show_multi_ip (2ms)
✓ outbound_show_specific (2ms)
✓ inbound_clear (3ms)
✓ outbound_resend (5ms)
✓ outbound_resend_down (3ms)
✓ peer_reconnect (5ms)
✓ withdrawal_handling (3ms)
✓ unknown_command (2ms)

16 passed, 0 failed
```

## Checklist

### 🧪 TDD
- [ ] Test harness created
- [ ] All scenarios implemented
- [ ] All scenarios pass

### Verification
- [ ] `go build ./test/cmd/rib-functional` succeeds
- [ ] `go run ./test/cmd/rib-functional` - all pass
- [ ] `make lint` passes

### Completion
- [ ] Spec moved to `docs/plan/done/NNN-rib-functional-test.md`
