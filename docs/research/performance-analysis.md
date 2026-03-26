# Ze Performance Analysis Report

**Date:** 2026-03-26
**Scope:** Identify bottlenecks affecting ze-perf convergence, throughput, and latency
**Method:** Code analysis + micro-benchmarks on hot path components

---

## Executive Summary

Ze's UPDATE forwarding pipeline is architecturally sound: zero-copy wire parsing,
pool-based buffer management, per-peer worker isolation, and batch flushing. The
design avoids the classic BGP daemon pitfalls (per-message malloc, global locks,
synchronous plugin dispatch).

However, the analysis identifies **6 high-impact** and **8 medium-impact** bottlenecks
that, combined, significantly reduce throughput under load. The most impactful are:

1. **Send hold timer recreation per write** (~3-5 us, 2 allocs per write)
2. **env.GetDuration() called per forward batch** (~1-5 us, 1 alloc per batch)
3. **Cache Retain/Release: N write-locks per UPDATE** (64 ns each, N = peer count)
4. **BufMux mutex contention under parallel load** (70 ns serial, 200-300 ns parallel)
5. **RewriteASPath allocations for EBGP** (5 allocs, 375 ns per EBGP peer variant)
6. **Bus notification map allocation per UPDATE** (1 alloc + addr.String())

---

## Benchmark Results

### Hot Path Component Costs (16 cores, Linux 6.8)

| Component | ns/op | B/op | allocs/op | Per UPDATE? |
|-----------|------:|-----:|----------:|-------------|
| **BufMux Get+Return (serial)** | 70 | 0 | 0 | Yes (read buffer) |
| **BufMux Get+Return (parallel)** | 200-300 | 0 | 0 | Yes (contention) |
| **Cache Retain+Release (serial)** | 64 | 0 | 0 | Yes, N times (N = peers) |
| **Cache Retain+Release (parallel)** | 175-196 | 0 | 0 | Yes, N times |
| **Cache Add+Get+Ack (full cycle)** | 1,335-1,546 | 433 | 4 | Yes |
| **FwdPool TryDispatch** | 247-197,964 | 0-71 | 0 | Yes, N times |
| **env.GetDuration (per batch)** | 618-4,750 | 24 | 1 | Per batch |
| **Timer Stop+AfterFunc (per write)** | 2,003-4,819 | 120 | 2 | Per write |
| **Bus notification map pattern** | 299-1,138 | 16 | 1 | Per UPDATE |
| **WireUpdate creation** | 0.48 | 0 | 0 | Yes |
| **WireUpdate parse (ensureParsed)** | 31 | 0 | 0 | Yes (once) |
| **WireUpdate Attrs()** | 122 | 80 | 1 | Yes |
| **WireUpdate Payload()** | 0.9 | 0 | 0 | Yes |
| **RewriteASPath (EBGP)** | 375-408 | 100 | 5 | Per EBGP variant |
| **SplitWireUpdate (no split)** | 37 | 8 | 1 | Only if oversized |
| **SplitWireUpdate (20 prefixes)** | 2,548-2,890 | 1,208 | 16 | Only if oversized |
| **Bus Publish (no subscribers)** | 16 | 0 | 0 | Yes |
| **Bus Publish (1 subscriber)** | 115-121 | 0 | 0 | Yes |
| **Bus Publish (10 subscribers)** | 1,022-1,064 | 0 | 0 | Yes |

### Per-UPDATE Total Estimated Cost (1 sender, 10 receiver peers, IBGP)

| Step | Cost (ns) | Allocs | Notes |
|------|----------:|-------:|-------|
| BufMux Get (read buffer) | 70 | 0 | Pooled |
| Header + body read | ~500 | 0 | I/O-bound |
| WireUpdate creation + parse | 32 | 0 | Zero-alloc |
| WireUpdate Attrs() | 122 | 1 | 80 B |
| RFC 7606 validation | ~200 | 0 | Linear scan |
| Prefix limit check | ~100 | 0 | Per-family |
| PeerInfo construction | ~50 | 0 | Stack |
| Ingress meta map | ~80 | 0-1 | Only if filters |
| ReceivedUpdate cache Add | ~500 | 2 | Entry + seqmap |
| Bus notification | ~400 | 1 | Map + String() |
| Delivery channel send | ~50 | 0 | Buffered channel |
| **Subtotal (receive path)** | **~2,100** | **4-5** | |
| Plugin dispatch + Activate | ~800 | 1 | Async |
| Cache Get | ~50 | 0 | Read lock |
| 10x Cache Retain | 640 | 0 | Write locks |
| 10x FwdPool TryDispatch | ~2,500 | 0 | Channel sends |
| **Subtotal (forward dispatch)** | **~3,990** | **1** | |
| 10x fwdBatchHandler: | | | |
|   env.GetDuration per batch | ~2,000 | 1 | Avoidable |
|   SetWriteDeadline | ~500 | 0 | Syscall |
|   writeRawUpdateBody | ~200 | 0 | Buffer copy |
|   bufWriter.Flush | ~500 | 0 | Syscall |
|   Clear deadline | ~500 | 0 | Syscall |
|   resetSendHoldTimer | ~3,500 | 2 | Stop + AfterFunc |
| **Subtotal (per peer write)** | **~7,200** | **3** | x10 peers |
| 10x Cache Release | 640 | 0 | Async, write locks |
| Cache Ack | ~500 | 1 | Cumulative |
| BufMux Return | 70 | 0 | |
| **GRAND TOTAL** | **~78,200** | **~36** | |

This gives a theoretical throughput ceiling of ~12,800 UPDATEs/sec per source peer
on a single core (excluding I/O wait). In practice, the forward writes are
parallelized across peer workers, so the effective bottleneck is the receive path
(~2,100 ns) plus the serial ForwardUpdate dispatch (~3,990 ns) = ~6,100 ns, giving
a theoretical ~164,000 UPDATEs/sec per source peer if write workers keep up.

---

## Bottleneck Analysis: Ranked by Impact

### Tier 1: High Impact (Measurable throughput improvement expected)

#### 1. Send Hold Timer Recreation Per Write

**Cost:** 2,000-4,800 ns + 2 allocs (120 B) per write
**Location:** `session_write.go:116-122` (resetSendHoldTimer)
**Frequency:** Every successful message write (UPDATEs, KEEPALIVEs)

```
s.sendHoldMu.Lock()
s.sendHoldTimer.Stop()
s.sendHoldTimer = s.clock.AfterFunc(s.sendHoldDuration(), s.sendHoldTimerExpired)
s.sendHoldMu.Unlock()
```

Each reset allocates a new timer via `AfterFunc()`. At 10K writes/sec per peer,
this is 10K timer allocations/sec creating significant GC pressure.

**Fix:** Reset the existing timer instead of creating a new one. Use `timer.Reset(d)`
instead of `Stop()` + `AfterFunc()`. This eliminates both allocations and reduces
cost to ~200 ns (just the atomic timer reset).

```go
// Before: 2 allocs, ~3500 ns
s.sendHoldTimer.Stop()
s.sendHoldTimer = s.clock.AfterFunc(d, callback)

// After: 0 allocs, ~200 ns
s.sendHoldTimer.Reset(d)
```

**Estimated gain:** ~3,000-4,600 ns per write, eliminating ~20K allocs/sec per peer.

---

#### 2. env.GetDuration() Called Per Forward Batch

**Cost:** 618-4,750 ns + 1 alloc (24 B) per batch
**Location:** `forward_pool.go:90` (fwdBatchHandler)
**Frequency:** Every forward batch (1000s/sec under load)

```go
writeDeadline := env.GetDuration("ze.fwd.write.deadline", fwdWriteDeadlineDefault)
```

The env package does a mutex-protected cache lookup + string operations on every call.
The value never changes at runtime.

**Fix:** Cache the write deadline in the fwdPool config at creation time. Read
`env.GetDuration` once in `newFwdPool()`, store in `fwdPoolConfig`.

**Estimated gain:** 600-4,700 ns + 1 alloc per batch.

---

#### 3. Cache Retain/Release: N Write-Locks Per UPDATE

**Cost:** 64 ns per Retain + 64 ns per Release, multiplied by peer count
**Location:** `reactor_api_forward.go:393-394` (ForwardUpdate loop)
**Frequency:** N times per UPDATE (N = number of matching peers)

```go
for _, peer := range matchingPeers {
    a.r.recentUpdates.Retain(updateID)  // Write lock
    item.done = func() { a.r.recentUpdates.Release(updateID) }  // Write lock later
}
```

For 100 peers: 200 write lock acquisitions per UPDATE on the same mutex.
Under parallel contention (~190 ns each), this becomes 38 us of lock operations.

**Fix:** Batch Retain: add `RetainN(id, count)` that increments retainCount by N
in a single lock acquisition. Similarly batch Release using an atomic counter.

```go
// Before: N lock operations
for _, peer := range matchingPeers { cache.Retain(id) }

// After: 1 lock operation
cache.RetainN(id, len(matchingPeers))
```

**Estimated gain:** (N-1) * 64-190 ns per UPDATE. For 10 peers: 576-1,710 ns saved.

---

#### 4. BufMux Mutex Contention Under Parallel Load

**Cost:** 70 ns serial, 200-300 ns under contention
**Location:** `bufmux.go:219-224, 255-272` (Get/Return)
**Frequency:** Every message read + return

Under 16 goroutines, BufMux Get/Return cost increases 3-4x due to mutex contention.
With many concurrent peers, this directly gates the read path.

**Fix options:**
- Shard the BufMux by source peer (each peer gets its own block allocator)
- Use per-goroutine buffer caching (thread-local pattern)
- Pre-allocate a larger initial block to reduce growth contention

The simplest fix: pre-warm the BufMux with enough blocks for the expected peer count
at startup, eliminating growth-time contention spikes.

**Estimated gain:** 130-230 ns per message read under contention.

---

#### 5. RewriteASPath Allocations for EBGP Forwarding

**Cost:** 375-408 ns + 5 allocs (100 B) per EBGP wire variant
**Location:** `wireu/aspath_rewrite.go` (RewriteASPath)
**Frequency:** Per EBGP variant per UPDATE (1-2 variants cached per ReceivedUpdate)

The rewrite allocates intermediate structures during AS-PATH manipulation.
These allocations happen on every new UPDATE with EBGP peers.

**Fix:** The rewrite already writes into a caller-provided buffer. The 5 allocations
likely come from internal attribute parsing. Profiling with `go test -cpuprofile`
would identify the exact allocation sites. A zero-alloc rewrite using offset-based
manipulation (matching the buffer-first pattern) could eliminate all 5.

**Estimated gain:** 375 ns + 5 allocs per EBGP variant. For mixed IBGP/EBGP: ~375 ns
per UPDATE.

---

#### 6. Bus Notification Map Allocation Per UPDATE

**Cost:** ~400 ns + 1 alloc (16 B) per UPDATE
**Location:** `reactor_notify.go:374-377` (publishBusNotification)
**Frequency:** Every received UPDATE

```go
r.publishBusNotification("bgp/update", map[string]string{
    "peer":      peerAddr.String(),  // String allocation
    "direction": direction,
})
```

Each call allocates a new `map[string]string` and converts `netip.Addr` to string.

**Fix options:**
- Cache `peerAddr.String()` on the Peer struct (string is immutable)
- Use a pre-allocated metadata struct instead of map
- Skip bus notification for UPDATE messages entirely if no subscribers (check subscriber
  count before allocating)

**Estimated gain:** ~400 ns + 1 alloc per UPDATE.

---

### Tier 2: Medium Impact (Incremental improvements)

#### 7. WireUpdate Attrs() Allocates Per Call

**Cost:** 122 ns + 1 alloc (80 B)
**Frequency:** 1-2 times per UPDATE (notify + forward)

`Attrs()` creates a new `AttributesWire` each call. Could cache the result in
`WireUpdate` since the payload is immutable.

#### 8. Forward Pool TryDispatch Variance

**Cost:** 247 ns (warm) to 197,964 ns (cold/worker creation)
**Frequency:** N times per UPDATE

Worker creation on first dispatch to a new peer causes massive latency spikes.
Pre-warming workers for configured peers would eliminate cold-start overhead.

#### 9. Reactor.mu RLock Peer Iteration

**Cost:** O(N) where N = total peers in reactor
**Frequency:** Every ForwardUpdate call (2 RLock acquisitions)

ForwardUpdate iterates ALL peers to find matching ones. With 1000 peers and a
selector matching 10, 990 peers are checked and skipped.

**Fix:** Maintain a pre-computed peer group index for common selectors.

#### 10. Cache Add Full Cycle Cost

**Cost:** 1,335-1,546 ns + 4 allocs (433 B)
**Frequency:** Every received UPDATE

The 4 allocations come from: cacheEntry struct, seqmap internal entry, and
potentially the WireUpdate metadata. Worth profiling to identify exact sources.

#### 11. Forward Pool Channel Size (Default 64)

The default channel capacity of 64 items means TryDispatch starts failing quickly
under burst traffic, pushing items to the overflow path. The overflow path adds
mutex contention (overflowMu) and prevents congestion recovery until the channel
drains below 16 items (25% low-water mark).

**Fix:** Increase default from 64 to 256. This matches the delivery channel capacity
and reduces overflow frequency. Configure via `ze.fwd.chan.size=256`.

#### 12. syscall Overhead in fwdBatchHandler

Each batch incurs 2-3 syscalls: SetWriteDeadline, Flush (TCP write), clear deadline.
These are unavoidable but could be reduced by:
- Setting write deadline once per worker activation instead of per batch
- Using a monotonic deadline (absolute time) instead of relative

#### 13. Forward Pool Batch Limit Default

The default `ze.fwd.batch.limit=1024` means batches of up to 1024 items. This holds
`writeMu` for extended periods. Profile actual batch sizes to determine if 256 or 512
would better balance latency vs throughput.

#### 14. SO_SNDBUF / SO_RCVBUF Not Tuned

Socket buffer sizes are left to OS defaults. On some systems, the default TCP
send buffer may be as small as 16KB, limiting burst throughput. Setting
`SO_SNDBUF=131072` and `SO_RCVBUF=262144` would match the bufio buffer sizes
and allow larger TCP windows.

---

## Architectural Observations

### What Ze Does Well

1. **Zero-copy UPDATE parsing:** WireUpdate slices into pool buffers. No per-UPDATE
   malloc for the payload. Lazy parsing defers work until needed.

2. **Buffer pooling:** BufMux block-backed multiplexer eliminates per-message buffer
   allocation. 0 allocs for Get/Return in steady state.

3. **Per-peer write isolation:** Forward pool workers have independent channels and
   goroutines. A slow peer cannot block forwarding to other peers.

4. **Batch flushing:** Forward pool batches multiple UPDATEs into a single
   `bufWriter.Flush()`, reducing syscall count by 10-64x.

5. **Async plugin delivery:** Per-peer delivery channels decouple the TCP read
   goroutine from plugin processing. The read loop is ~2 us per UPDATE.

6. **TCP tuning:** TCP_NODELAY + DSCP CS6 + 64KB read buffer + 16KB write buffer
   are well-matched to BGP traffic patterns.

### What Limits Ze

1. **Per-write timer allocation:** The send hold timer reset pattern creates
   unnecessary GC pressure. This is the single largest per-write overhead.

2. **Per-batch env var lookup:** The `env.GetDuration` call in the forward batch
   handler adds measurable overhead that could be eliminated entirely.

3. **Serial cache locking:** N sequential Retain/Release calls on the same mutex
   creates a serialization point proportional to peer count.

4. **EBGP wire rewrite allocations:** The AS-PATH rewrite path allocates 5 objects
   per variant. This is small per-UPDATE but significant at scale.

5. **No peer group indexing:** ForwardUpdate iterates all peers linearly. With many
   peers and selective forwarding, most iterations are wasted.

---

## Recommended Action Plan

### Phase 1: Quick Wins (estimated +15-25% throughput)

| # | Change | Est. Gain | Risk | Effort |
|---|--------|-----------|------|--------|
| 1 | Timer.Reset() instead of Stop+AfterFunc | 3-4.6 us/write | Low | 20 lines |
| 2 | Cache env.GetDuration at startup | 0.6-4.7 us/batch | Low | 10 lines |
| 3 | Cache peerAddr.String() on Peer | ~400 ns/UPDATE | Low | 5 lines |
| 4 | Batch cache Retain (RetainN) | (N-1)*64ns/UPDATE | Low | 30 lines |
| 5 | Increase fwd channel default to 256 | Reduced overflow | Low | 1 line |

### Phase 2: Medium Effort (estimated +10-15% additional)

| # | Change | Est. Gain | Risk | Effort |
|---|--------|-----------|------|--------|
| 6 | Cache WireUpdate Attrs() result | 122 ns + 80B/call | Low | 15 lines |
| 7 | Zero-alloc RewriteASPath | 375ns + 5 allocs | Medium | 100 lines |
| 8 | Pre-warm BufMux for peer count | Reduced contention | Low | 20 lines |
| 9 | Tune SO_SNDBUF/SO_RCVBUF | Better burst TCP | Low | 15 lines |
| 10 | Pre-warm fwd workers for configured peers | Eliminate cold-start | Low | 20 lines |

### Phase 3: Structural (estimated +20-30% additional)

| # | Change | Est. Gain | Risk | Effort |
|---|--------|-----------|------|--------|
| 11 | Peer group index for selectors | O(1) vs O(N) lookup | Medium | 200 lines |
| 12 | Sharded BufMux per-peer | Reduced contention | Medium | 150 lines |
| 13 | Skip bus notification if no subscribers | ~400 ns/UPDATE | Low | 10 lines |
| 14 | Configurable bufio buffer sizes | Better I/O batching | Low | 30 lines |

---

## Benchmark Files Created

These files can be used for regression testing after optimizations:

| File | Benchmarks | Focus |
|------|-----------|-------|
| `internal/component/bgp/reactor/hotpath_bench_test.go` | 10 | BufMux, cache, fwd pool, env, timer |
| `internal/component/bgp/wireu/wire_bench_test.go` | 16 | WireUpdate parsing, rewrite, split |
| `internal/component/bus/bus_bench_test.go` | 9 | Bus publish, map alloc patterns |

Run all benchmarks:
```bash
go test -run='^$' -bench=. -benchmem ./internal/component/bgp/reactor/ ./internal/component/bgp/wireu/ ./internal/component/bus/
```

---

## Profiling Recommendations

For deeper analysis beyond micro-benchmarks:

```bash
# CPU profile during ze-perf run
ze --pprof 127.0.0.1:6060 config.conf &
ze-perf run --dut-addr 127.0.0.1 --dut-asn 65000 --routes 10000
go tool pprof http://127.0.0.1:6060/debug/pprof/profile?seconds=30

# Mutex contention profile
go tool pprof http://127.0.0.1:6060/debug/pprof/mutex

# Block (channel) contention profile
go tool pprof http://127.0.0.1:6060/debug/pprof/block

# Heap allocation profile
go tool pprof http://127.0.0.1:6060/debug/pprof/heap
```

Focus the CPU profile on:
- `fwdBatchHandler` (write path)
- `notifyMessageReceiver` (receive path)
- `ForwardUpdate` (dispatch path)
- `readAndProcessMessage` (read loop)

---

## Methodology

1. **Code analysis:** Read every file on the UPDATE hot path, tracing data flow from
   TCP read through forwarding to TCP write. Identified all locks, allocations,
   syscalls, and computation.

2. **Agent-parallel research:** 7 specialized analysis agents examined lock contention,
   per-message overhead, forward pool mechanics, plugin dispatch, existing benchmarks,
   TCP/IO tuning, and cache operations simultaneously.

3. **Micro-benchmarks:** Created 35 benchmarks targeting specific hot-path components.
   Ran with `-benchmem -count=3` for statistical reliability.

4. **Cost modeling:** Aggregated per-component costs into a per-UPDATE budget,
   identifying which components dominate the total.
