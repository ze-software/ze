# 316 — Buffered Writes

## Objective

Reduce write syscalls on the BGP forwarding hot path by adding a `bufio.Writer` (16KB) to Session and draining multiple `fwdItem`s from the forward pool channel under a single `writeMu` acquisition with one `Flush`.

## Decisions

- Batch at the Session layer, not the forward pool: `writeMu` is per-session, state checks (`Established`, `conn != nil`) happen once, forward pool passes arrays unchanged
- `SendUpdate`/`SendAnnounce`/`SendWithdraw` keep flushing per-call: used for API-injected routes and initial sync where callers expect bytes on wire immediately
- `drainBatch` is non-blocking drain after first blocking receive: forward pool worker drains what is queued without waiting, processes whole batch under one lock
- `peer_send.go` required no changes: plan to add batch wrapper was wrong; peer wrappers delegate to session unchanged

## Patterns

- Internal write/flush split already existed: `writeUpdate`, `writeRawUpdateBody`, `flushWrites` were separated from public `Send*` — optimization only required wiring at batch boundary
- Lock ordering is `s.mu` → `s.writeMu`: `closeConn` must acquire `writeMu` for final `bufWriter.Flush` inside `s.mu` critical section

## Gotchas

- Race in `closeConn`: flushed `bufWriter` under `s.mu` but not `s.writeMu`, creating race with concurrent `Send*` callers. Fix: acquire `writeMu` inside `closeConn`
- `TestForwardPoolBackpressurePropagation` broke: batch handler drained items before test finished filling channel. Fix: gate handler with signal
- 8 existing tests set `session.conn` directly without `bufWriter` — ongoing maintenance trap for tests constructing sessions manually
- `SendRawMessage` with `msgType==0` wrote to raw `conn.Write` without `writeMu` — pre-existing race fixed as side effect

## Files

- `internal/component/bgp/reactor/session.go`, `session_test.go`
- `internal/component/bgp/reactor/forward_pool.go`, `forward_pool_test.go`, `forward_update_test.go`
- `internal/component/bgp/reactor/reactor.go`
