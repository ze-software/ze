# 292 — Persistent Conn Reader

## Objective

Eliminate all 5 per-RPC goroutines in `pkg/plugin/rpc/conn.go` by replacing goroutine-bridged I/O with a persistent reader goroutine and deadline-based writes.

## Decisions

- `frameCh` capacity 1 (not 0): lets the reader goroutine push one frame without blocking while the consumer processes the previous one. Capacity 0 forces lock-step; larger capacity is unnecessary since reads are serial per Conn.
- Error sent via `atomic.Pointer[error]` + `readerDone` channel, not through `frameCh` as `frameResult{nil, err}` — the `block-silent-ignore.sh` hook rejects bare `default:` cases needed to drain the channel without blocking. The atomic pointer pattern is the correct Ze pattern (borrowed from MuxConn).
- `sync.Once` lazy start for `startReader()` instead of starting in `NewConn()` — MuxConn creates a Conn then immediately takes over the reader. If Conn started a reader in `NewConn`, MuxConn would race with it.
- MuxConn updated to share Conn's persistent reader via `conn.readFrame()` — discovered that MuxConn accessing `conn.reader` directly races with the persistent reader goroutine when SDK calls `ReadRequest` during handshake then wraps in MuxConn.
- 30s default write deadline when context has no deadline — startup RPCs use server-scoped contexts without explicit deadlines; prevents writes blocking indefinitely if the peer hangs without closing the socket.
- `SetWriteDeadline` timeout returns `net.Error` (i/o timeout), NOT `context.DeadlineExceeded` — translate the error to match callers' expectations.

## Patterns

- Persistent reader goroutine pattern: `frameCh chan []byte` (cap 1) + `atomic.Pointer[error]` for stored error + `readerDone chan struct{}` for close signaling. This is the same pattern MuxConn uses for concurrent multi-caller dispatch.
- Deadline-based writes eliminate goroutine bridges: `writeConn.SetWriteDeadline(deadline)` → write → `writeConn.SetWriteDeadline(time.Time{})` (clear).

## Gotchas

- DATA RACE: MuxConn accessing `conn.reader` directly + Conn's persistent reader goroutine both call `reader.Read()` — two goroutines reading from the same `bufio.Scanner`. Fix: MuxConn calls `conn.readFrame(context.Background())`.
- `SetWriteDeadline` timeout returns `net.Error`, not `context.DeadlineExceeded` — callers checking for `context.DeadlineExceeded` need explicit translation.
- `string(rune(i+1))` for i=0 produces `\x01` (control character, invalid JSON) — use `fmt.Sprintf("%d", i+1)` in test helpers.
- `block-silent-ignore.sh` hook rejects bare `default:` — cannot drain a channel silently; must use the atomic error pointer pattern.

## Files

- `pkg/plugin/rpc/conn.go` — `startReader`, `readLoop`, `readFrame`, `writeDeadline`, `writeBatchWithDeadline`, `WriteWithContext` rewritten
- `pkg/plugin/rpc/conn_test.go` — 13 new unit tests
- `pkg/plugin/rpc/mux.go` — updated to use `conn.readFrame()` (deviation from plan)
