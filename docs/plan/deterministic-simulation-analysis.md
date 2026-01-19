# ZeBGP Deterministic Simulation Testing Analysis

**Date:** 2025-12-28
**Status:** Research Complete
**Author:** Claude Code Analysis

---

## Executive Summary

This report analyzes the feasibility and requirements for implementing deterministic simulation testing in ZeBGP. The goal is to enable:

1. **Seeded randomness** - All non-determinism controlled by seed
2. **Replay debugging** - Reproduce any execution exactly
3. **Fault injection** - Test failure scenarios systematically
4. **Property verification** - Prove RFC compliance

**Feasibility:** MODERATE-HIGH
**Effort:** ~15-22 days for core implementation (see Section 12)
**Risk:** LOW (backward-compatible changes)

**Known Limitations:** CGO, raw syscalls, and file I/O are NOT covered by this design.

---

## Table of Contents

1. [Industry Examples](#1-industry-examples)
2. [Sources of Non-Determinism](#2-sources-of-non-determinism)
3. [Exact Injection Points](#3-exact-injection-points)
4. [Go-Specific Approaches](#4-go-specific-approaches)
5. [Seeded Randomness Design](#5-seeded-randomness-design)
6. [Goroutine Scheduling Control](#6-goroutine-scheduling-control)
7. [Fault Injection Specification](#7-fault-injection-specification)
8. [State Capture for Replay](#8-state-capture-for-replay)
9. [Property-Based Testing](#9-property-based-testing)
10. [FSM Event Queue Design](#10-fsm-event-queue-design)
11. [Simulator Architecture](#11-simulator-architecture)
12. [Implementation Roadmap](#12-implementation-roadmap)
13. [Verification Properties](#13-verification-properties)
14. [Appendix: Complete Inventory](#14-appendix-complete-inventory)
15. [Known Limitations](#15-known-limitations)
16. [Determinism Verification](#16-determinism-verification)

---

## 1. Industry Examples

### 1.1 Turso/Limbo (Rust - SQLite rewrite)

**Repository:** [tursodatabase/limbo](https://github.com/tursodatabase/limbo)

Turso rewrote SQLite in Rust with DST built-in from the start. Their approach:

**Simulator Structure:**
```
simulator/
├── main.rs           # Entry point, seed handling
├── common/           # Shared utilities
├── generation/       # Random SQL/plan generation
├── model/            # Simplified DB model for assertions
├── profiles/         # Configuration presets
├── runner/           # Execution engine
└── shrink/           # Test case minimization (reduces failing cases)
```

**Key Concepts:**
- **Interaction Plans:** Sequence of SQL queries + assertions
- **Properties:** Invariants that must hold (e.g., "inserted row appears in SELECT")
- **Model:** Simplified database representation for determining valid next actions
- **Shrink:** Automatically minimizes failing test cases for easier debugging

**Two-Level Testing:**
| Level | Tool | Purpose |
|-------|------|---------|
| Internal DST | Limbo simulator | Fast iteration, "unit test"-like |
| External DST | Antithesis | OS-level faults, hypervisor-based |

**Quote:** *"You can plug a simulator into the I/O loop. The simulator has a seed, and given the same seed, you guarantee that every single thing will always execute in the same way, in the same order, and with the same side-effects."*

**Bug Bounty:** Turso offers $800 via [Algora](https://turso.algora.io/) for improving their simulator to catch new bugs, plus $200 for fixing bugs found.

---

### 1.2 TigerBeetle (Zig - Financial database)

**Repository:** [tigerbeetle/tigerbeetle](https://github.com/tigerbeetle/tigerbeetle)

TigerBeetle pioneered DST for financial systems with their VOPR simulator.

**Key Design Decisions:**
- **Single-threaded I/O:** *"Keeping I/O code single-threaded in userspace is best for determinism"*
- **io_uring native:** Designed for Linux's async I/O interface
- **24/7 fuzzing:** VOPR runs continuously on 1024 cores
- **Browser demo:** [sim.tigerbeetle.com](https://sim.tigerbeetle.com) shows live simulation

**VOPR (Viewstamped Operation Replayer):**
- Simulates entire cluster in single process
- Injects network, storage, and process faults at 1000x speed
- Every random decision traces back to initial seed
- Can replay any failure exactly

**Quote:** *"With this you can reproduce an entire execution by restarting the system with the same random seed."*

---

### 1.3 Polar Signals (Go - FrostDB)

**Blog:** [Mostly Deterministic Simulation Testing in Go](https://www.polarsignals.com/blog/posts/2024/05/28/mostly-dst-in-go)

Most relevant for ZeBGP since it's Go-based.

**Their Approach (3 techniques combined):**

1. **WebAssembly for single-threading:**
   - Compile Go to WASM (wasip1)
   - WASM is single-threaded by design
   - Eliminates goroutine scheduling non-determinism

2. **Modified Go runtime for seeded randomness:**
   - Go runtime uses global PRNG for scheduling decisions
   - Modified to read seed via environment variable
   - *"Less than 10 lines of code"* change

3. **Fake time via build tag:**
   - `-tags=faketime` enables playground-style time
   - Time only advances when all goroutines blocked

**Results:** Found 5 bugs in early testing (3 data loss, 2 data duplication).

**Caveat:** Title includes "(mostly)" — occasional divergence still under investigation.

---

### 1.4 FoundationDB (C++ - Distributed KV store)

The original pioneer of DST for distributed systems.

**Key Innovation:** Flow language (coroutines) that compiles to both:
- Real async code (production)
- Deterministic simulation (testing)

**Impact:** Found bugs that would take years to encounter in production.

---

### 1.5 Comparison Table

| Aspect | Turso (Rust) | TigerBeetle (Zig) | Polar Signals (Go) | ZeBGP (Go) |
|--------|--------------|-------------------|---------------------|------------|
| **I/O Abstraction** | Trait-based | Compile-time | WASM sandbox | Interface injection |
| **Scheduling** | Single-threaded | Single-threaded | WASM forces | TBD |
| **Time Control** | Injected clock | Simulated | `faketime` tag | Injected clock |
| **Seed Source** | CLI `--seed` | CLI | Env var | CLI/Env |
| **Fault Injection** | Antithesis + internal | VOPR | Manual | Internal |
| **Goroutine Control** | N/A | N/A | Modified runtime | TBD |

---

## 2. Sources of Non-Determinism

### 2.1 Time Operations

| Type | Count | Files | Impact |
|------|-------|-------|--------|
| `time.Now()` | 4 | reactor.go, session.go | Deadline calculations |
| `time.Sleep()` | 21 | tests, session.go, peer.go | Blocking waits |
| `time.After()` | 9 | peer.go, tests | Timeout channels |
| `time.AfterFunc()` | 4 | timer.go | RFC 4271 timers |

**Critical:** FSM timers (HoldTimer, KeepaliveTimer, ConnectRetryTimer) use `time.AfterFunc()` directly with no abstraction.

### 2.2 Network I/O

| Type | Count | Files | Impact |
|------|-------|-------|--------|
| `net.Dial*` | 1 | session.go:210 | Outbound connections |
| `net.Listen` | 1 | listener.go:84 | Inbound connections |
| `conn.Read` | 4 | session.go, reactor.go | Message receipt |
| `conn.Write` | 3 | session.go | Message sending |
| `Accept()` | 1 | listener.go:94 | Connection acceptance |

**Critical:** All network I/O uses real TCP sockets with no mock capability.

### 2.3 Goroutine Scheduling

| Location | Count | Purpose |
|----------|-------|---------|
| Production code | 15 | Peer loops, listeners, timers |
| Test code | 45 | Test helpers, async operations |

**Critical spawn points in production:**

| File:Line | Component | Trigger |
|-----------|-----------|---------|
| `peer.go:365` | `Peer.run()` | Peer start |
| `peer.go:441` | `Peer.Wait()` | Context wait |
| `peer.go:558` | `sendInitialRoutes()` | FSM→Established |
| `peer.go:663` | `ResolvePendingCollision()` | Collision detected |
| `listener.go:94` | `acceptLoop()` | Listener start |
| `listener.go:117` | `Listener.Wait()` | Context wait |
| `listener.go:161` | Connection handler | Accept callback |
| `reactor.go:1513` | `monitor()` | Reactor start |
| `reactor.go:1532` | `reactor.Wait()` | Context wait |
| `reactor.go:1662` | `handlePendingCollision()` | Pending connection |
| `signal.go:97` | `SignalHandler.run()` | Signal handling |
| `signal.go:114` | `SignalHandler.Wait()` | Context wait |

### 2.4 Select Statements (Non-Deterministic Choice)

Go's `select` picks arbitrarily among ready cases. Found **33 locations**:

**Production (critical):**

| File:Line | Cases | Risk |
|-----------|-------|------|
| `session.go:458-465` | ctx.Done, errChan, default | Timer vs message race |
| `session.go:90-93` | errChan send, default | Non-blocking send |
| `peer.go:462-470` | ctx.Done, default | Shutdown race |
| `peer.go:490-494` | ctx.Done, time.After | Backoff timing |
| `listener.go:137-148` | ctx.Done, accept | Accept race |
| `signal.go:133-140` | sigChan, ctx.Done | Signal delivery |
| `reactor.go:1537` | done channel | Wait race |

### 2.5 Channel Operations

| File:Line | Channel | Buffer | Senders | Receivers |
|-----------|---------|--------|---------|-----------|
| `session.go:76` | `errChan` | 2 | Timer callbacks, Teardown | Run loop |
| `signal.go:38` | `sigChan` | 1 | OS signals | Handler |
| `listener.go:116` | `done` | 0 | acceptLoop | Wait |
| `peer.go:440` | `done` | 0 | run loop | Wait |
| `reactor.go:1531` | `done` | 0 | monitor | Wait |

### 2.6 Mutex-Protected State

| File | Field | Protects | Lock Pattern |
|------|-------|----------|--------------|
| `peer.go` | `mu` RWMutex | session, opQueue, watchdog | Multiple acquire/release |
| `session.go` | `mu` RWMutex | conn, negotiated | Unlock before callback |
| `listener.go` | `mu` RWMutex | handler, running | Clean pattern |
| `reactor.go` | `mu` RWMutex | peers map, state | Nested with peer locks |
| `fsm/fsm.go` | `mu` RWMutex | state, callback | Unlock during callback |
| `fsm/timer.go` | `mu` Mutex | timer state | Unlock before callback |

**Race Windows Identified:**

1. **Peer.setState()** (peer.go:322): Callback called outside lock
2. **Session timer callbacks** (session.go:86-94): FSM event inside lock, errChan send outside
3. **Collision resolution** (peer.go:663): Async session close

---

## 3. Exact Injection Points

> **⚠️ Note:** Line numbers were captured on 2025-12-28 and may drift as code changes.
> Before implementation, verify each location with:
> ```bash
> grep -n "time.Now\|time.Sleep\|time.After\|net.Dial\|net.Listen" internal/reactor/*.go internal/bgp/fsm/*.go
> ```

### 3.1 Time Injection (10 locations)

| File | Line | Current | Replace With |
|------|------|---------|--------------|
| `reactor.go` | 1457 | `time.Now()` | `clock.Now()` |
| `reactor.go` | 1697 | `time.Now().Add(holdTime)` | `clock.Now().Add(holdTime)` |
| `session.go` | 474 | `time.Sleep(10*ms)` | `clock.Sleep(10*ms)` |
| `session.go` | 479 | `time.Now().Add(100*ms)` | `clock.Now().Add(100*ms)` |
| `session.go` | 504 | `time.Now().Add(5*s)` | `clock.Now().Add(5*s)` |
| `peer.go` | 493 | `time.After(delay)` | `clock.After(delay)` |
| `timer.go` | 151 | `time.AfterFunc(d, cb)` | `clock.AfterFunc(d, cb)` |
| `timer.go` | 188 | `time.AfterFunc(d, cb)` | `clock.AfterFunc(d, cb)` |
| `timer.go` | 269 | `time.AfterFunc(d, cb)` | `clock.AfterFunc(d, cb)` |
| `timer.go` | 319 | `time.AfterFunc(d, cb)` | `clock.AfterFunc(d, cb)` |

### 3.2 Network Injection (7 locations)

| File | Line | Current | Replace With |
|------|------|---------|--------------|
| `session.go` | 203 | `net.Dialer{}` | Injected `Dialer` interface |
| `session.go` | 210 | `d.DialContext(...)` | `dialer.DialContext(...)` |
| `listener.go` | 84 | `net.Listen(...)` | `listenerFactory.Listen(...)` |
| `session.go` | 512 | `io.ReadFull(conn, header)` | `conn.Read()` via mock |
| `session.go` | 549 | `io.ReadFull(conn, body)` | `conn.Read()` via mock |
| `session.go` | 1175 | `conn.Write(data)` | `conn.Write()` via mock |
| `session.go` | 1282 | `conn.Write(data)` | `conn.Write()` via mock |

### 3.3 Goroutine Spawn Points (12 production locations)

Each requires yield point registration:

| File | Line | Function | Yield After |
|------|------|----------|-------------|
| `peer.go` | 365 | `go p.run()` | Spawn |
| `peer.go` | 441 | `go func()` Wait | Completion |
| `peer.go` | 558 | `go sendInitialRoutes()` | Route send |
| `peer.go` | 663 | `go session.Close()` | Notification |
| `listener.go` | 94 | `go acceptLoop()` | Accept |
| `listener.go` | 117 | `go func()` Wait | Completion |
| `listener.go` | 161 | `go handler(conn)` | Handle |
| `reactor.go` | 1513 | `go r.monitor()` | Monitor |
| `reactor.go` | 1532 | `go func()` Wait | Completion |
| `reactor.go` | 1662 | `go handlePendingCollision()` | Collision |
| `signal.go` | 97 | `go run()` | Signal |
| `signal.go` | 114 | `go func()` Wait | Completion |

---

## 4. Go-Specific Approaches

Go presents unique challenges for DST due to goroutines and the runtime scheduler. Here are proven approaches:

### 4.1 Option A: WASM Sandbox (Polar Signals approach)

**How it works:**
1. Compile Go code to WebAssembly (`GOOS=wasip1 GOARCH=wasm`)
2. Run in WASM runtime (wasmtime, wazero)
3. WASM is single-threaded by design → no scheduling non-determinism

**Pros:**
- No code changes to business logic
- Automatic single-threading
- Clean separation of concerns

**Cons:**
- Some packages don't compile to WASM (syscall, unsafe)
- Slower execution
- Requires WASM-compatible dependencies

**For ZeBGP:** Network stack (`net` package) doesn't work in WASM. Would need mock network layer anyway.

### 4.2 Option B: Modified Go Runtime

**How it works:**
1. Fork Go runtime
2. Add seed parameter for scheduler PRNG
3. Read seed from environment variable

**Polar Signals implementation:** ~10 lines changed in runtime.

```go
// runtime/proc.go (conceptual)
var schedulerSeed = getenv("GO_SCHEDULER_SEED")
if schedulerSeed != "" {
    fastrandseed = parseUint64(schedulerSeed)
}
```

**Pros:**
- Works with all Go code
- Minimal changes

**Cons:**
- Requires maintaining runtime fork
- New Go versions need re-patching
- Still "(mostly)" deterministic per Polar Signals

### 4.3 Option C: Faketime Build Tag

**How it works:**
1. Use `-tags=faketime` (like Go playground)
2. Time starts at fixed point
3. Advances only when all goroutines blocked

**Pros:**
- No runtime modification
- Built into Go toolchain
- Simple to use

**Cons:**
- Only controls time, not scheduling
- Doesn't help with `select` non-determinism

### 4.4 Option D: Interface Injection (Recommended for ZeBGP)

**How it works:**
1. Define interfaces for Clock, Dialer, Listener
2. Inject implementations via constructor
3. Production: real implementations
4. Testing: mock implementations with seeded behavior

**Pros:**
- No runtime hacks
- Clean, idiomatic Go
- Backward compatible
- Works with all Go versions

**Cons:**
- Requires code changes at injection points
- Doesn't control goroutine scheduling directly
- Need to serialize with event queue for full determinism

**Recommended for ZeBGP** because:
- Minimal invasiveness
- No external dependencies
- Compatible with existing test infrastructure
- Can be incrementally adopted

### 4.5 Option E: Deterministic Hypervisors (External)

**How it works:**
1. Run ZeBGP in a deterministic hypervisor
2. Hypervisor controls all non-determinism at OS level (syscalls, scheduling, time)
3. Automatic fault injection and replay

**Commercial: Antithesis**
- Built on FreeBSD/Bhyve ("the Determinator")
- Complete container determinism without code changes
- ~$200K cost, but free tier for open source
- Used by: Ethereum, MongoDB, CockroachDB

**Open Source Alternatives:**

| Project | Status | Approach |
|---------|--------|----------|
| [Hermit](https://github.com/facebookexperimental/hermit) (Meta) | Maintenance mode | Syscall interception + PMU for thread scheduling |
| QEMU fork | In development | Utkarsh Srivastava working on deterministic QEMU |
| [databases.systems](https://databases.systems/posts/open-source-antithesis-p1) | Research | "Testbed" hypervisor attempt |

**Hermit Details:**
- Uses CPU Performance Monitoring Unit (PMU) to stop threads after fixed instruction count
- Intercepts syscalls via Reverie library
- Limitation: "long tail of unsupported system calls"
- Requires fixed filesystem (Docker) + disabled networking

**Pros:**
- No code changes required
- Catches bugs in simulator itself
- Professional support (Antithesis)

**Cons:**
- Commercial cost (Antithesis) or incomplete (open source)
- Slower than internal DST
- Less control over test scenarios
- Hermit: maintenance mode, incomplete syscall coverage

**Recommendation:**
- For ZeBGP: Use internal DST (interface injection) as primary
- Consider Antithesis free tier for open source projects
- Watch Hermit/QEMU forks for future open source options

### 4.6 ZeBGP Recommended Approach

**Hybrid strategy:**

| Layer | Approach | Handles |
|-------|----------|---------|
| Time | Interface injection (`Clock`) | Timers, deadlines |
| Network | Interface injection (`Dialer`, `Listener`) | TCP I/O |
| Select | Event queue + FSM serialization | Non-deterministic choice |
| Scheduling | Single-threaded test mode (optional) | Goroutine ordering |
| OS-level | Antithesis (optional, future) | Kernel behavior |

---

## 5. Seeded Randomness Design

### 5.1 Seed → Return Value Mapping

Every non-deterministic operation must map to a seeded return value:

| Operation | Input | Seed Determines | Return Type |
|-----------|-------|-----------------|-------------|
| `select` N ready | Case list | Case index (0..N-1) | `int` |
| `time.After(d)` | Duration | Exact fire time | `<-chan Time` |
| `net.Dial()` | Address | Success/error, latency | `(Conn, error)` |
| `conn.Read()` | Buffer | Bytes read, error | `(int, error)` |
| `conn.Write()` | Data | Bytes written, error | `(int, error)` |
| `Accept()` | - | Which conn, when | `(Conn, error)` |
| Goroutine schedule | - | Next goroutine ID | `uint64` |

### 5.2 Deterministic Scheduler Interface

```go
// internal/sim/scheduler.go

type Scheduler struct {
    seed      uint64
    rng       *rand.Rand       // Deterministic PRNG
    clock     *VirtualClock
    events    *EventHeap       // Priority queue
    seq       uint64           // Tie-breaker sequence
    goroutines map[uint64]*SimGoroutine
    current   uint64           // Currently running goroutine
}

// Replace all select statements with this
func (s *Scheduler) Select(cases []SelectCase) int {
    ready := s.findReadyCases(cases)
    if len(ready) == 0 {
        return -1 // All blocked
    }
    if len(ready) == 1 {
        return ready[0]
    }
    // Multiple ready: seed determines choice
    idx := s.rng.Intn(len(ready))
    return ready[idx]
}

// Replace time.After with this
func (s *Scheduler) After(d time.Duration) <-chan time.Time {
    ch := make(chan time.Time, 1)
    fireTime := s.clock.Now().Add(d)
    s.events.Push(Event{
        Time:    fireTime,
        Seq:     s.seq,
        Type:    EventTimerFire,
        Handler: func() { ch <- fireTime },
    })
    s.seq++
    return ch
}
```

### 5.3 Clock Interface

```go
// internal/sim/clock.go

type Clock interface {
    Now() time.Time
    After(d time.Duration) <-chan time.Time
    AfterFunc(d time.Duration, f func()) Timer
    Sleep(d time.Duration)
}

type Timer interface {
    Stop() bool
    Reset(d time.Duration) bool
    C() <-chan time.Time
}

// Real implementation (production)
type RealClock struct{}

func (c *RealClock) Now() time.Time              { return time.Now() }
func (c *RealClock) After(d time.Duration) <-chan time.Time { return time.After(d) }
func (c *RealClock) AfterFunc(d time.Duration, f func()) Timer {
    return &realTimer{t: time.AfterFunc(d, f)}
}
func (c *RealClock) Sleep(d time.Duration) { time.Sleep(d) }

// Virtual implementation (simulation)
type VirtualClock struct {
    mu      sync.Mutex
    now     time.Time
    timers  []*VirtualTimer
    seq     uint64
}

func (c *VirtualClock) Advance(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()

    target := c.now.Add(d)

    // Fire all timers in order
    for {
        next := c.nextTimer()
        if next == nil || next.deadline.After(target) {
            break
        }
        c.now = next.deadline
        next.fire()
    }

    c.now = target
}
```

---

## 6. Goroutine Scheduling Control

### 6.1 Cooperative Scheduling Model

```go
// internal/sim/goroutine.go

type SimGoroutine struct {
    id        uint64
    state     GoroutineState
    waitChan  chan struct{}
    stack     string           // For debugging
    spawnedAt time.Time        // Simulated time
    yieldedAt time.Time
}

type GoroutineState int
const (
    GoroutineReady GoroutineState = iota
    GoroutineRunning
    GoroutineBlocked
    GoroutineTerminated
)

type YieldReason int
const (
    YieldNetworkRead YieldReason = iota
    YieldNetworkWrite
    YieldTimerWait
    YieldTimerFire
    YieldChannelSend
    YieldChannelRecv
    YieldMutexAcquire
    YieldSpawn
)

func (s *Scheduler) Yield(reason YieldReason) {
    g := s.goroutines[s.current]
    g.state = GoroutineBlocked
    g.yieldedAt = s.clock.Now()

    s.log(EventGoroutineYield, g.id, reason)

    // Pick next goroutine (deterministic via seed)
    s.scheduleNext()

    // Block until rescheduled
    <-g.waitChan
}

func (s *Scheduler) scheduleNext() {
    ready := s.readyGoroutines()
    if len(ready) == 0 {
        return // Deadlock or all done
    }

    // Seed determines scheduling order
    idx := s.rng.Intn(len(ready))
    next := ready[idx]

    s.current = next.id
    next.state = GoroutineRunning
    next.waitChan <- struct{}{}
}
```

### 6.2 Yield Point Insertion

Every blocking operation needs a yield point:

| Operation | Location | Yield Call |
|-----------|----------|------------|
| `io.ReadFull` | session.go:512 | `scheduler.Yield(YieldNetworkRead)` |
| `conn.Write` | session.go:1175 | `scheduler.Yield(YieldNetworkWrite)` |
| `time.After` receive | peer.go:493 | `scheduler.Yield(YieldTimerWait)` |
| Timer callback | timer.go:151+ | `scheduler.Yield(YieldTimerFire)` |
| `Accept()` | listener.go:94 | `scheduler.Yield(YieldNetworkRead)` |
| `DialContext` | session.go:210 | `scheduler.Yield(YieldNetworkWrite)` |
| Channel send | session.go:90 | `scheduler.Yield(YieldChannelSend)` |
| Mutex acquire | various | `scheduler.Yield(YieldMutexAcquire)` |

---

## 7. Fault Injection Specification

### 7.1 Fault Types

```go
// internal/sim/fault.go

type FaultType int
const (
    FaultNone FaultType = iota

    // Connection faults
    FaultConnectionRefused    // Dial returns error
    FaultConnectionReset      // Read/Write returns RST
    FaultConnectionTimeout    // Deadline exceeded
    FaultConnectionClosed     // EOF on read

    // Message faults
    FaultPartialRead          // Read returns fewer bytes
    FaultPartialWrite         // Write returns fewer bytes
    FaultCorruptedHeader      // Invalid marker/length
    FaultCorruptedBody        // Invalid message content
    FaultReorderedMessages    // Messages arrive out of order
    FaultDuplicatedMessage    // Same message twice
    FaultDelayedDelivery      // Message delayed by N ms

    // Timer faults
    FaultTimerDrift           // Timer fires early/late
    FaultTimerMissed          // Timer never fires

    // Resource faults
    FaultOutOfMemory          // Allocation fails
    FaultBufferFull           // Channel full
)
```

### 7.2 Fault Injection Interface

```go
type FaultInjector struct {
    seed     uint64
    rng      *rand.Rand
    rules    []FaultRule
    injected []InjectedFault  // Log of injected faults
}

type FaultRule struct {
    Trigger     FaultTrigger
    Type        FaultType
    Probability float64        // 0.0-1.0, seed-based
    Params      FaultParams
}

type FaultTrigger struct {
    // When to consider injecting
    Operation   string         // "read", "write", "dial", "accept", "timer"
    MessageType uint8          // BGP message type (0=any)
    AfterCount  int            // After N operations
    AtTime      time.Duration  // At simulated time
    State       string         // FSM state
}

type FaultParams struct {
    // For partial read/write
    ByteCount int

    // For corruption
    CorruptOffset int
    CorruptBytes  []byte

    // For delay
    DelayMin time.Duration
    DelayMax time.Duration

    // For timeout
    TimeoutAfter time.Duration
}

func (f *FaultInjector) Check(op string, ctx FaultContext) *Fault {
    for _, rule := range f.rules {
        if !rule.Trigger.Matches(op, ctx) {
            continue
        }
        // Seed determines if fault fires
        if f.rng.Float64() < rule.Probability {
            fault := &Fault{
                Type:   rule.Type,
                Params: rule.Params,
            }
            f.injected = append(f.injected, InjectedFault{
                Time:  ctx.Time,
                Op:    op,
                Fault: fault,
            })
            return fault
        }
    }
    return nil
}
```

### 7.3 Fault Injection Points

| Fault Type | Injection Point | Implementation |
|------------|-----------------|----------------|
| ConnectionRefused | `MockDialer.DialContext()` | Return `syscall.ECONNREFUSED` |
| ConnectionReset | `MockConn.Read/Write()` | Return `syscall.ECONNRESET` |
| PartialRead | `MockConn.Read()` | Return `n < len(buf)` |
| PartialWrite | `MockConn.Write()` | Return `n < len(data)` |
| CorruptedHeader | `MockConn.Read()` header | Modify marker bytes |
| CorruptedBody | `MockConn.Read()` body | Modify payload |
| Timeout | `MockConn` with deadline | Return `os.ErrDeadlineExceeded` |
| DelayedDelivery | `MockConn.Write()` | Queue with delay |
| TimerDrift | `VirtualClock.AfterFunc()` | Adjust fire time |

---

## 8. State Capture for Replay

### 8.1 Event Log Format

```go
// internal/sim/event.go

type EventLog struct {
    Version   int              // Log format version
    Seed      uint64           // Initial seed
    StartTime time.Time        // Real wall clock start
    Events    []Event
}

type Event struct {
    Seq       uint64           // Global sequence number
    Time      time.Duration    // Simulated time since start
    Goroutine uint64           // Which goroutine
    Type      EventType
    Data      EventData        // Type-specific data
}

type EventType int
const (
    // Lifecycle events
    EventSimulationStart EventType = iota
    EventSimulationEnd

    // Goroutine events
    EventGoroutineSpawn
    EventGoroutineYield
    EventGoroutineResume
    EventGoroutineTerminate

    // Timer events
    EventTimerStart
    EventTimerFire
    EventTimerStop
    EventTimerReset

    // Network events
    EventNetworkDial
    EventNetworkDialResult
    EventNetworkAccept
    EventNetworkRead
    EventNetworkWrite
    EventNetworkClose

    // BGP events
    EventMessageSent
    EventMessageReceived
    EventFSMTransition

    // Concurrency events
    EventSelectEnter
    EventSelectChoice
    EventChannelSend
    EventChannelRecv
    EventMutexAcquire
    EventMutexRelease

    // Fault events
    EventFaultInjected
)

type EventData interface {
    eventData()
}

// Example event data types
type FSMTransitionData struct {
    Peer     string
    From     string
    To       string
    Event    string
}

type MessageData struct {
    Peer     string
    Type     uint8
    Length   uint16
    Payload  []byte  // For replay
}

type SelectChoiceData struct {
    CaseCount int
    ReadyCases []int
    Chosen    int
}

type FaultData struct {
    Type    FaultType
    Target  string
    Params  FaultParams
}
```

### 8.2 Replay Mechanism

```go
// internal/sim/replay.go

func Replay(log EventLog) (*Simulation, error) {
    sim := NewSimulation(log.Seed)

    for i, event := range log.Events {
        // Advance clock to event time
        sim.clock.AdvanceTo(event.Time)

        // Process event
        if err := sim.processEvent(event); err != nil {
            return nil, fmt.Errorf("event %d: %w", i, err)
        }

        // Verify determinism
        if sim.seq != event.Seq {
            return nil, fmt.Errorf("sequence mismatch at event %d: got %d, want %d",
                i, sim.seq, event.Seq)
        }
    }

    return sim, nil
}

func (s *Simulation) Checkpoint() *Snapshot {
    return &Snapshot{
        Time:       s.clock.Now(),
        Seq:        s.seq,
        FSMStates:  s.captureFSMStates(),
        Goroutines: s.captureGoroutines(),
        Timers:     s.captureTimers(),
        Channels:   s.captureChannels(),
    }
}

func (s *Simulation) Restore(snap *Snapshot) {
    s.clock.Set(snap.Time)
    s.seq = snap.Seq
    s.restoreFSMStates(snap.FSMStates)
    s.restoreGoroutines(snap.Goroutines)
    s.restoreTimers(snap.Timers)
    s.restoreChannels(snap.Channels)
}
```

### 8.3 State to Capture

| Component | State Fields | Capture Method |
|-----------|--------------|----------------|
| FSM | `state`, `passive` | `fsm.State()`, `fsm.IsPassive()` |
| Timers | Active timers, deadlines | Timer list snapshot |
| Session | `conn`, `negotiated`, `peerOpen` | Pointer snapshots |
| Peer | `state`, `session`, `opQueue` | Atomic + lock read |
| RIB | Route maps | Deep copy |
| Channels | Buffer contents | Drain and restore |

---

## 9. Property-Based Testing

Property-based testing (PBT) defines invariants that must hold for all inputs. Combined with DST, it enables exhaustive verification.

### 9.1 BGP-Specific Properties

Based on RFC 4271 and ZeBGP requirements:

| Property | Description | RFC Reference |
|----------|-------------|---------------|
| **FSM-Transitions** | All state transitions match RFC 4271 Section 8 table | RFC 4271 §8.2.2 |
| **HoldTimer-Expiry** | Peer transitions to Idle after HoldTime expires | RFC 4271 §8.2.2 |
| **Keepalive-Interval** | KEEPALIVE sent every HoldTime/3 | RFC 4271 §4.4 |
| **Collision-Resolution** | Higher BGP ID wins connection collision | RFC 4271 §6.8 |
| **Message-Order** | OPEN before UPDATE, KEEPALIVE after OPEN | RFC 4271 §8.2.2 |
| **Capability-Negotiation** | Only negotiated families in UPDATE | RFC 4760 §6 |
| **ADD-PATH-Encoding** | Path ID included iff ADD-PATH negotiated | RFC 7911 §3 |

### 9.2 Property Definition Pattern

Following Turso's model:

```go
// internal/sim/property.go

type Property interface {
    Name() string
    Description() string

    // Setup generates initial state for testing
    Setup(rng *rand.Rand) PropertyState

    // Check verifies invariant holds after action
    Check(state PropertyState, action Action, result Result) error
}

// Example: Insert-Select equivalent for BGP
type AnnounceWithdrawProperty struct{}

func (p *AnnounceWithdrawProperty) Name() string {
    return "announce-withdraw"
}

func (p *AnnounceWithdrawProperty) Description() string {
    return "Withdrawn route must not appear in RIB after UPDATE processed"
}

func (p *AnnounceWithdrawProperty) Check(
    state PropertyState,
    action Action,
    result Result,
) error {
    if action.Type != ActionWithdraw {
        return nil
    }

    // After withdraw, route must not be in RIB
    if state.RIB.Contains(action.Prefix) {
        return fmt.Errorf("prefix %s still in RIB after withdraw", action.Prefix)
    }
    return nil
}
```

### 9.3 Collision Resolution Property (Complete Example)

```go
type CollisionResolutionProperty struct{}

func (p *CollisionResolutionProperty) Check(
    state PropertyState,
    action Action,
    result Result,
) error {
    if action.Type != ActionCollision {
        return nil
    }

    // RFC 4271 §6.8: Higher BGP ID wins
    localID := state.LocalBGPID
    remoteID := action.RemoteBGPID

    expectedWinner := "local"
    if remoteID > localID {
        expectedWinner = "remote"
    }

    actualWinner := result.CollisionWinner

    if actualWinner != expectedWinner {
        return fmt.Errorf(
            "collision resolution failed: local=%d remote=%d expected=%s got=%s",
            localID, remoteID, expectedWinner, actualWinner,
        )
    }
    return nil
}
```

### 9.4 Using rapid for Property Testing

```go
import "pgregory.net/rapid"

func TestFSMTransitions(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        seed := rapid.Uint64().Draw(t, "seed")

        sim := NewSimulation(seed)
        fsm := sim.NewFSM()

        // Generate random event sequence
        numEvents := rapid.IntRange(1, 100).Draw(t, "numEvents")

        for i := 0; i < numEvents; i++ {
            event := rapid.SampledFrom(allFSMEvents).Draw(t, "event")
            stateBefore := fsm.State()

            fsm.Event(event)

            stateAfter := fsm.State()

            // Verify transition matches RFC 4271 table
            expected := rfc4271TransitionTable[stateBefore][event]
            if stateAfter != expected {
                t.Fatalf(
                    "FSM violation: state=%s event=%s expected=%s got=%s",
                    stateBefore, event, expected, stateAfter,
                )
            }
        }
    })
}
```

---

## 10. FSM Event Queue Design

### 10.1 Problem

Current FSM processes events immediately (fsm.go:123):

```go
func (f *FSM) Event(event Event) error {
    f.mu.Lock()
    defer f.mu.Unlock()
    // IMMEDIATE processing - no determinism control
}
```

Timer callbacks and message processing can race:
- Timer fires → `fsm.Event(EventHoldTimerExpires)`
- Message arrives → `fsm.Event(EventBGPOpen)`
- Which executes first is non-deterministic

### 10.2 Solution: Event Queue

```go
// internal/sim/fsmqueue.go

type FSMEventQueue struct {
    mu      sync.Mutex
    events  []QueuedFSMEvent
    seq     uint64
    clock   Clock
}

type QueuedFSMEvent struct {
    Seq       uint64
    Time      time.Time        // When queued
    Event     fsm.Event
    Source    EventSource
    Priority  int              // Lower = higher priority
}

type EventSource int
const (
    SourceTimer EventSource = iota
    SourceMessage
    SourceAPI
    SourceInternal
)

func (q *FSMEventQueue) Enqueue(e fsm.Event, source EventSource) {
    q.mu.Lock()
    defer q.mu.Unlock()

    q.events = append(q.events, QueuedFSMEvent{
        Seq:      q.seq,
        Time:     q.clock.Now(),
        Event:    e,
        Source:   source,
        Priority: q.priorityFor(e, source),
    })
    q.seq++
}

func (q *FSMEventQueue) Process(fsm *fsm.FSM) []fsm.State {
    q.mu.Lock()
    events := q.events
    q.events = nil
    q.mu.Unlock()

    // Sort by (time, priority, seq) for determinism
    sort.Slice(events, func(i, j int) bool {
        if events[i].Time != events[j].Time {
            return events[i].Time.Before(events[j].Time)
        }
        if events[i].Priority != events[j].Priority {
            return events[i].Priority < events[j].Priority
        }
        return events[i].Seq < events[j].Seq
    })

    var transitions []fsm.State
    for _, e := range events {
        before := fsm.State()
        _ = fsm.Event(e.Event)
        after := fsm.State()
        if before != after {
            transitions = append(transitions, after)
        }
    }

    return transitions
}

func (q *FSMEventQueue) priorityFor(e fsm.Event, source EventSource) int {
    // RFC 4271 doesn't specify priority, but we need determinism
    // Lower number = higher priority
    switch source {
    case SourceMessage:
        return 0  // Messages first (they arrived)
    case SourceTimer:
        return 1  // Timers second
    case SourceAPI:
        return 2  // API third
    default:
        return 3
    }
}
```

---

## 11. Simulator Architecture

Following Turso/Limbo's proven structure:

### 11.1 Directory Structure

```
internal/sim/
├── sim.go              # Main Simulation type, entry point
├── clock.go            # Clock interface + VirtualClock
├── network.go          # Dialer, Listener, Conn interfaces
├── mockconn.go         # MockConn implementation
├── scheduler.go        # Goroutine scheduler
├── goroutine.go        # SimGoroutine type
├── fault.go            # FaultInjector
├── fault_types.go      # Fault type definitions
├── event.go            # Event types
├── eventlog.go         # Event logging
├── replay.go           # Replay mechanism
├── property.go         # Property interface
├── properties/         # BGP-specific properties
│   ├── fsm.go          # FSM transition properties
│   ├── collision.go    # Collision resolution
│   ├── timer.go        # Timer behavior
│   └── message.go      # Message ordering
├── generation/         # Random generation
│   ├── config.go       # Random config generation
│   ├── message.go      # Random BGP message generation
│   └── fault.go        # Random fault scenarios
├── model/              # Simplified models for assertions
│   ├── rib.go          # Simplified RIB model
│   └── peer.go         # Simplified peer model
├── runner/             # Execution engine
│   ├── runner.go       # Main runner
│   └── plan.go         # Interaction plans
└── shrink/             # Test case minimization
    └── shrink.go       # Shrinking algorithm
```

### 11.2 Core Types

```go
// internal/sim/sim.go

type Simulation struct {
    seed       uint64
    rng        *rand.Rand
    clock      *VirtualClock
    scheduler  *Scheduler
    faults     *FaultInjector
    events     *EventLog
    properties []Property

    // ZeBGP components under test
    reactor    *reactor.Reactor
    peers      map[string]*SimPeer
}

func New(seed uint64) *Simulation {
    rng := rand.New(rand.NewSource(int64(seed)))
    return &Simulation{
        seed:      seed,
        rng:       rng,
        clock:     NewVirtualClock(),
        scheduler: NewScheduler(rng),
        faults:    NewFaultInjector(rng),
        events:    NewEventLog(seed),
        peers:     make(map[string]*SimPeer),
    }
}

func (s *Simulation) Run() error {
    for !s.scheduler.AllDone() {
        // Advance to next event
        next := s.events.NextTime()
        s.clock.AdvanceTo(next)

        // Process all events at this time
        for _, e := range s.events.At(next) {
            if err := s.process(e); err != nil {
                return err
            }
        }

        // Check properties
        for _, p := range s.properties {
            if err := p.Check(s.State()); err != nil {
                return fmt.Errorf("property %s violated: %w", p.Name(), err)
            }
        }
    }
    return nil
}
```

### 11.3 Interaction Plans (Turso pattern)

```go
// internal/sim/runner/plan.go

type InteractionPlan struct {
    Steps []Step
}

type Step struct {
    Action     Action
    Assertions []Assertion
}

type Action interface {
    Execute(sim *Simulation) error
}

type Assertion interface {
    Check(sim *Simulation) error
}

// Example actions
type ConnectPeersAction struct {
    Peer1, Peer2 string
}

type InjectFaultAction struct {
    Fault FaultSpec
}

type AdvanceTimeAction struct {
    Duration time.Duration
}

type SendUpdateAction struct {
    From   string
    Routes []Route
}

// Example assertions
type StateAssertion struct {
    Peer          string
    ExpectedState fsm.State
}

type RIBAssertion struct {
    Peer          string
    ContainsRoute netip.Prefix
}

type MessageCountAssertion struct {
    Peer          string
    MessageType   uint8
    ExpectedCount int
}
```

### 11.4 Test Case Shrinking

When a failure is found, automatically minimize the reproduction:

```go
// internal/sim/shrink/shrink.go

func Shrink(plan InteractionPlan, seed uint64) InteractionPlan {
    // Binary search to find minimal failing plan
    for len(plan.Steps) > 1 {
        half := len(plan.Steps) / 2

        // Try first half
        firstHalf := InteractionPlan{Steps: plan.Steps[:half]}
        if stillFails(firstHalf, seed) {
            plan = firstHalf
            continue
        }

        // Try second half
        secondHalf := InteractionPlan{Steps: plan.Steps[half:]}
        if stillFails(secondHalf, seed) {
            plan = secondHalf
            continue
        }

        // Can't shrink further
        break
    }

    // Try removing individual steps
    for i := 0; i < len(plan.Steps); i++ {
        without := removeStep(plan, i)
        if stillFails(without, seed) {
            plan = without
            i-- // Recheck this index
        }
    }

    return plan
}
```

### 11.5 CLI Interface

```
$ zebgp-sim --seed 12345 --steps 1000 --properties all
Seed: 12345
Steps: 1000
Properties: fsm-transitions, collision-resolution, hold-timer, ...

Running simulation...
Step 500: Injecting fault: connection-reset on peer-B
Step 501: Checking property: fsm-transitions
Step 502: VIOLATION: fsm-transitions

Property 'fsm-transitions' violated at step 502:
  State: OpenSent
  Event: TCPConnectionFails
  Expected: Active (RFC 4271 §8.2.2)
  Got: Idle

Shrinking...
Minimal reproduction: 3 steps
  Step 1: Connect peer-A to peer-B
  Step 2: Advance time 50ms
  Step 3: Inject connection-reset on peer-B

Replay command:
  zebgp-sim --seed 12345 --replay steps.json
```

---

## 12. Implementation Roadmap

### Phase 1: Clock Abstraction (2-3 days)

**Files to create:**
- `internal/sim/clock.go` - Clock interface + implementations (~150 LOC)

**Files to modify:**
- `internal/bgp/fsm/timer.go` - Add `Clock` parameter to `Timers`
- `internal/reactor/session.go` - Add `Clock` for deadline calculations
- `internal/reactor/peer.go` - Add `Clock` for backoff
- `internal/reactor/reactor.go` - Add `Clock` for startup time

**Backward compatibility:**
- Default to `RealClock{}` in constructors
- No API changes for existing callers

### Phase 2: Network Abstraction (3-4 days)

**Files to create:**
- `internal/sim/network.go` - Dialer, Listener, Conn interfaces (~300 LOC)
- `internal/sim/mockconn.go` - Mock connection implementation (~200 LOC)

**Files to modify:**
- `internal/reactor/session.go` - Inject `Dialer` interface
- `internal/reactor/listener.go` - Inject `ListenerFactory` interface

**Backward compatibility:**
- Default to real `net` package implementations
- Tests can inject mocks

### Phase 3: Scheduler & Yield Points (3-4 days)

**Files to create:**
- `internal/sim/scheduler.go` - Goroutine scheduler (~200 LOC)
- `internal/sim/goroutine.go` - SimGoroutine type (~100 LOC)

**Strategy for select replacement:**

Option A: Build tag separation
```go
//go:build !simulation

func (s *Session) Run(ctx context.Context) error {
    // Real select
    select { ... }
}
```

```go
//go:build simulation

func (s *Session) Run(ctx context.Context) error {
    // Scheduler-based select
    idx := sim.Scheduler.Select(...)
}
```

Option B: Runtime interface
```go
type SelectExecutor interface {
    Select(cases []SelectCase) int
}

var DefaultSelector SelectExecutor = &RealSelector{}

func (s *Session) Run(ctx context.Context) error {
    idx := DefaultSelector.Select(cases)
}
```

**Recommendation:** Option B for cleaner code, Option A for zero overhead in production.

### Phase 4: Fault Injection (2-3 days)

**Files to create:**
- `internal/sim/fault.go` - FaultInjector (~150 LOC)
- `internal/sim/fault_types.go` - Fault type definitions (~100 LOC)

**Integration:**
- MockConn checks FaultInjector before Read/Write
- MockDialer checks before returning connection
- VirtualClock can inject timer drift

### Phase 5: Event Logging & Replay (2-3 days)

**Files to create:**
- `internal/sim/event.go` - Event types (~200 LOC)
- `internal/sim/eventlog.go` - Logging (~150 LOC)
- `internal/sim/replay.go` - Replay mechanism (~200 LOC)

### Phase 6: Test Migration (3-5 days)

**Priority order:**
1. FSM timer tests → use VirtualClock
2. Session tests → use MockConn
3. Collision tests → use full simulation
4. Integration tests → full simulation

---

## 13. Verification Properties

With deterministic simulation, we can verify:

| Property | Test Approach |
|----------|---------------|
| **RFC 4271 FSM correctness** | All state transitions match RFC table |
| **No deadlocks** | Run with all possible interleavings |
| **No data races** | Same seed = same result (TSan still useful) |
| **Timer accuracy** | Virtual time matches expected |
| **Message ordering** | Replay produces identical sequence |
| **Fault recovery** | Inject faults, verify graceful handling |
| **Collision resolution** | Both BGP ID orderings produce correct winner |
| **Hold timer expiry** | Peer marked down after timeout |
| **Keepalive timing** | Sent at hold_time/3 intervals |

### 13.1 Example Property Test

```go
func TestCollisionResolution(t *testing.T) {
    // Property: Higher BGP ID always wins collision

    rapid.Check(t, func(t *rapid.T) {
        seed := rapid.Uint64().Draw(t, "seed")
        id1 := rapid.Uint32().Draw(t, "id1")
        id2 := rapid.Uint32().Draw(t, "id2")

        if id1 == id2 {
            t.Skip("same IDs")
        }

        sim := NewSimulation(seed)
        sim.AddPeer("A", id1)
        sim.AddPeer("B", id2)

        // Both initiate simultaneously
        sim.Connect("A", "B")
        sim.Connect("B", "A")

        sim.RunUntilStable()

        // Higher ID should win
        winner := "A"
        if id2 > id1 {
            winner = "B"
        }

        assert.Equal(t, fsm.StateEstablished, sim.State(winner))
    })
}
```

---

## 14. Appendix: Complete Inventory

### 14.1 All time.Now() Calls

```
internal/reactor/reactor.go:1457    r.startTime = time.Now()
internal/reactor/reactor.go:1697    conn.SetReadDeadline(time.Now().Add(holdTime))
internal/reactor/session.go:479     conn.SetReadDeadline(time.Now().Add(100*ms))
internal/reactor/session.go:504     conn.SetReadDeadline(time.Now().Add(5*s))
```

### 14.2 All time.Sleep() Calls

```
internal/reactor/session.go:474     time.Sleep(10 * time.Millisecond)
internal/reactor/peer_test.go:67    time.Sleep(20 * time.Millisecond)
internal/reactor/peer_test.go:121   time.Sleep(50 * time.Millisecond)
... (21 total, mostly in tests)
```

### 14.3 All time.After() Calls

```
internal/reactor/peer.go:493        case <-time.After(delay):
internal/reactor/listener_test.go:203  case <-time.After(time.Second):
... (9 total)
```

### 14.4 All time.AfterFunc() Calls

```
internal/bgp/fsm/timer.go:151       t.holdTimer = time.AfterFunc(t.holdTime, func() {...})
internal/bgp/fsm/timer.go:188       t.holdTimer = time.AfterFunc(t.holdTime, func() {...})
internal/bgp/fsm/timer.go:269       t.keepaliveTimer = time.AfterFunc(keepaliveInterval, timerFunc)
internal/bgp/fsm/timer.go:319       t.connectRetryTimer = time.AfterFunc(t.connectRetryTime, func() {...})
```

### 14.5 All Select Statements (33)

```
internal/reactor/listener.go:122,137,146
internal/reactor/peer.go:446,462,471,490
internal/reactor/session.go:90,434,458
internal/reactor/reactor.go:1537
internal/reactor/signal.go:119,133
internal/plugin/server.go:140,155,169,249,323
internal/plugin/process.go:245,272,311,439
internal/test/peer/peer.go:179,195,207,304
internal/plugin/process_test.go:422
internal/bgp/fsm/timer_test.go:44,74,82,131,180
```

### 14.6 All Goroutine Spawns (60)

See Section 2.3 for production locations (12).
Test locations: 48 in various `*_test.go` files.

### 14.7 All Channel Creations (19)

```
internal/reactor/session.go:76      errChan = make(chan error, 2)
internal/reactor/listener.go:116    done = make(chan struct{})
internal/reactor/peer.go:440        done = make(chan struct{})
internal/reactor/reactor.go:1531    done = make(chan struct{})
internal/reactor/signal.go:38       sigChan = make(chan os.Signal, 1)
internal/reactor/signal.go:113      done = make(chan struct{})
internal/plugin/process.go:182         lines = make(chan string, 100)
internal/plugin/process.go:185         writeQueue = make(chan []byte, WriteQueueHighWater)
internal/plugin/server.go:134          done = make(chan struct{})
... (19 total)
```

---

## 15. Known Limitations

### 15.1 What This Design Does NOT Cover

| Component | Why Not Covered | Mitigation |
|-----------|-----------------|------------|
| **CGO code** | C code bypasses Go abstractions | Avoid CGO or mock at boundary |
| **Raw syscalls** | `syscall` package bypasses simulation | Use higher-level Go APIs |
| **File I/O** | Not abstracted in current design | Add `FileSystem` interface if needed |
| **Signal handling** | OS signals are non-deterministic | Mock `signal.Notify` channel |
| **DNS resolution** | Uses real network | Mock at `Dialer` level |
| **TLS handshake** | Crypto randomness | Inject deterministic TLS config |
| **GC timing** | Go runtime controls GC | Cannot control, may cause variance |

### 15.2 Goroutine Scheduler Limitations

The cooperative scheduler (Section 6) has these constraints:

1. **Must yield:** Code that doesn't yield (busy loops) will hang simulation
2. **GOMAXPROCS:** Simulation assumes `GOMAXPROCS=1` or equivalent serialization
3. **Runtime interactions:** GC, stack growth, defer may not be fully deterministic
4. **Cgo callbacks:** Cannot control scheduling of Cgo callback goroutines

### 15.3 Partial Determinism Warning

Even with all abstractions in place, Go's runtime introduces sources of non-determinism:

- **Map iteration order:** Randomized per Go spec (use sorted keys)
- **Select with multiple ready:** Scheduler controls, but edge cases exist
- **Goroutine IDs:** Not exposed by Go, must track manually
- **Stack traces:** May vary slightly

**Recommendation:** Verify determinism by running same seed twice and comparing event logs.

---

## 16. Determinism Verification

### 16.1 How to Verify Simulation is Deterministic

```go
func TestDeterminism(t *testing.T) {
    seed := uint64(12345)

    // Run 1
    sim1 := NewSimulation(seed)
    sim1.Run()
    log1 := sim1.EventLog()

    // Run 2 (same seed)
    sim2 := NewSimulation(seed)
    sim2.Run()
    log2 := sim2.EventLog()

    // Must be identical
    if !log1.Equal(log2) {
        diff := log1.Diff(log2)
        t.Fatalf("Non-determinism detected at event %d:\n%s", diff.Index, diff.Description)
    }
}
```

### 16.2 CI Integration

```yaml
# .github/workflows/simulation.yml
simulation:
  runs-on: ubuntu-latest
  steps:
    - name: Run simulation (seed A)
      run: go run ./cmd/zebgp-sim --seed 12345 --output run1.log

    - name: Run simulation (seed A again)
      run: go run ./cmd/zebgp-sim --seed 12345 --output run2.log

    - name: Verify determinism
      run: diff run1.log run2.log || (echo "DETERMINISM FAILURE" && exit 1)

    - name: Run simulation (random seeds)
      run: |
        for i in $(seq 1 100); do
          seed=$RANDOM
          go run ./cmd/zebgp-sim --seed $seed --steps 10000 || exit 1
        done
```

### 16.3 Checkpoint Hashing

```go
func (s *Simulation) StateHash() uint64 {
    h := fnv.New64a()

    // Hash all deterministic state
    h.Write([]byte(fmt.Sprintf("%d", s.seq)))
    h.Write([]byte(s.clock.Now().String()))

    for _, peer := range s.peers {
        h.Write([]byte(peer.State().String()))
    }

    return h.Sum64()
}

// In simulation loop:
if step%100 == 0 {
    hash := sim.StateHash()
    log.Printf("Step %d: state hash %x", step, hash)
}
```

---

## Conclusion

ZeBGP can support deterministic simulation testing with the changes outlined in this report. The key requirements are:

1. **Clock abstraction** - Replace all `time.*` calls with injected Clock
2. **Network abstraction** - Replace all `net.*` calls with injected interfaces
3. **Scheduler control** - Replace `select` with deterministic scheduler
4. **Event queue** - Serialize FSM events for determinism
5. **Fault injection** - Enable systematic failure testing
6. **Event logging** - Enable replay debugging
7. **Property testing** - Define RFC-based invariants
8. **Test case shrinking** - Automatically minimize reproductions

The implementation is **backward-compatible** with existing production code and can be incrementally adopted.

### Key Learnings from Industry

| Project | Key Insight |
|---------|-------------|
| **Turso/Limbo** | Simulator structure (generation, model, properties, shrink) |
| **TigerBeetle** | 24/7 fuzzing, seed-based replay, VOPR architecture |
| **Polar Signals** | Go-specific: WASM + modified runtime + faketime |
| **FoundationDB** | Original DST pioneer, Flow language approach |
| **Antithesis** | External DST as complement to internal testing |

---

## References

### Industry Implementations
- [Turso: Introducing Limbo](https://turso.tech/blog/introducing-limbo-a-complete-rewrite-of-sqlite-in-rust)
- [Turso Bug Bounty (Algora)](https://turso.algora.io/)
- [TigerBeetle Safety Docs](https://docs.tigerbeetle.com/concepts/safety/)
- [TigerBeetle Simulator](https://sim.tigerbeetle.com)
- [Polar Signals: Mostly DST in Go](https://www.polarsignals.com/blog/posts/2024/05/28/mostly-dst-in-go)
- [Phil Eaton: DST Overview](https://notes.eatonphil.com/2024-08-20-deterministic-simulation-testing.html)

### Deterministic Hypervisors
- [Antithesis DST](https://antithesis.com/resources/deterministic_simulation_testing/)
- [Antithesis: So you think you want to write a deterministic hypervisor?](https://antithesis.com/blog/deterministic_hypervisor/)
- [Open Source Deterministic Hypervisors Are Coming](https://materializedview.io/p/open-source-deterministic-hypervisors)
- [Hermit (Meta)](https://github.com/facebookexperimental/hermit) - Deterministic Linux execution
- [Building an open-source version of Antithesis](https://databases.systems/posts/open-source-antithesis-p1)
- [FreeBSD Developer Summit: Antithesis](https://freebsdfoundation.org/blog/2024-freebsd-developer-summit-antithesis-deterministic-hypervisor/)

### GitHub Repositories
- [tursodatabase/limbo](https://github.com/tursodatabase/limbo)
- [tigerbeetle/tigerbeetle](https://github.com/tigerbeetle/tigerbeetle)
- [facebookexperimental/hermit](https://github.com/facebookexperimental/hermit)
