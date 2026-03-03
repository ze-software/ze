# 290 — Buffered TCP Session Reads

## Objective

Wrap the BGP session TCP connection with `bufio.Reader` (64KB) to reduce read syscalls from 2 per BGP message to approximately 1 per 64KB of data.

## Decisions

- Buffer size 64KB (not 8KB as originally planned) — matches the extended BGP message size limit, allowing a single kernel read to fill the buffer with one full extended message.
- `bufReader` initialized in `connectionEstablished()` rather than `Run()` — cleaner lifecycle; `connectionEstablished` is the natural point where a new connection is ready for use, and it replaces `bufReader` on reconnection.
- `bufReader` NOT nilled on `closeConn()` — `Run()` may have captured `conn`, and `bufReader` wrapping the closed conn returns a proper read error; `connectionEstablished()` replaces it on reconnection.
- Mechanical change, no design decisions beyond buffer sizing.

## Patterns

- `SetReadDeadline` must be called on the underlying `net.Conn`, not on `bufio.Reader` — `bufio.Reader` does not implement deadline methods.
- `bufio.Reader` buffer size and pool buffer size are independent: `bufio` buffers kernel reads; pools buffer parsed messages. They serve different purposes and need not match.

## Gotchas

- None.

## Files

- `internal/plugins/bgp/reactor/session.go` — `bufReader *bufio.Reader` field, initialized in `connectionEstablished()`
- `internal/plugins/bgp/reactor/session_read.go` — `io.ReadFull(s.bufReader, ...)` replaces direct `conn` reads
- `internal/plugins/bgp/reactor/session_read_test.go` — `TestSessionReadWithBufio`, `TestSessionReadDeadlineWithBufio`
