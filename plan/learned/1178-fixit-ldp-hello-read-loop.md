# 1178 -- LDP Basic Discovery: dedicated Hello reader goroutine

## Context

`plan/spec-fixit-ldp-hello-read-loop.md`. LDP Basic Discovery (`internal/plugins/ldp`)
sends and receives multicast Hellos per interface in `discoverOnInterface`
(`register.go`). The receive was structurally coupled to the send: the loop's
`select` had only `<-ctx.Done()` and `<-helloTicker.C -> sendHello`, and a single
1s-deadline-bounded `udpConn.ReadFromUDP` sat *after* the `select`, running exactly
once per iteration. With `DefaultHelloInterval = 5s` the loop turned ~every 5s, so
inbound Hellos were drained once per 5s, one datagram at a time. On a shared segment
with N neighbors, N Hellos arrive per interval but only 1 was consumed; the socket
receive buffer filled, Hellos dropped, hold timers (15s) expired, adjacencies flapped.
Even a single neighbor was timing-fragile: its Hello had to land inside the 1s window
that opened once per 5s.

## Decisions

- **Mirror the ISIS dedicated-reader model, do not invent a new one.**
  `internal/plugins/isis/transport/backend_linux.go` (`readLoop`) is the in-repo
  precedent: a receiver runs its own loop with its own buffer, checks a stop signal,
  and exits on socket close / ctx cancel. New `readDiscoveryLoop` follows it exactly:
  its own `recvBuf`, `ReadFromUDP -> processDiscoveryPacket` with no per-tick gate,
  exits on `ctx.Err()`, `net.ErrClosed`, or a `SetReadDeadline` error.
- **Keep the send path where it was; only the read moved.** The main loop reduces to
  `select { <-ctx.Done() -> close+join+return; <-helloTicker.C -> sendHello }`. The
  initial Hello on entry, multicast egress pinning + TTL 1, and `sendHello` cadence are
  untouched. `processDiscoveryPacket`/`AdjacencyTable.Update`/`onNewAdj` semantics
  unchanged.
- **Concurrent Read+Write on one `*net.UDPConn` is relied upon (stdlib guarantee).** The
  reader reads while `sendHello` writes the same conn; Go permits one concurrent Read and
  one Write. `AdjacencyTable` is RWMutex-guarded, so the reader's `Update` races safely
  with the existing `runAdjacencyExpiry` sweep. Confirmed `-race` clean.
- **Idempotent close via `sync.Once`, explicit on ctx cancel + deferred backstop.** The
  ctx-cancel branch calls `closeConn()` to unblock a blocked `ReadFromUDP` immediately,
  then `<-readerDone` joins the reader before returning, so no goroutine or socket leaks
  across a config reload. The `defer closeConn()` covers every other return path (and
  double-close is swallowed by the `sync.Once`, avoiding a spurious debug log).
- **1s read deadline kept as a backstop (spec A-3).** Even if a socket close is ever
  missed, the deadline wakes the reader within 1s so it re-checks `ctx.Err()` and exits.
  A dedicated test cancels ctx WITHOUT closing the socket to prove this path.
- **Repaired pre-existing integration-test staleness in scope.**
  `frr_interop_integration_linux_test.go` still called `startSessionForAdj` with the
  old 8-arg signature (a prior transport-address refactor added a `netip.Addr` arg). It
  only compiles under `integration && linux`, so normal CI never caught it. Since that
  test is the on-wire coverage of the new reader path, added the missing `c.TransportAddr`
  arg (mirroring production wiring at `register.go`).

## Consequences

- All N neighbors' Hellos per interval are now consumed; single Hellos are consumed
  promptly rather than inside a 1s-per-5s window. Adjacency flap on shared segments is
  removed at the source (the coupling), not by widening a timer.
- Tests: `internal/plugins/ldp/discovery_burst_test.go` drives `readDiscoveryLoop` over
  real loopback UDP sockets (unicast; the drain contract is identical to multicast and
  keeps the tests deterministic). `TestDiscoveryDrainsBurst` (AC-1),
  `TestDiscoverySingleHelloPrompt` (AC-2), `TestDiscoveryReaderExitsOnCancel` +
  `TestDiscoveryReaderExitsOnCancelDeadlineBackstop` (AC-3),
  `TestDiscoverySendUnaffectedByReads` (AC-4, concurrent flood + send cadence). The
  pre-fix loop would drain zero datagrams within the burst test's 3s window (it waited
  5s on the ticker before its first read), so the test captures the regression.
- Related recover-boundary work (`plan/spec-improve-5-panic-boundaries`) owns adding a
  per-message `recover` around the decode in this loop; deliberately not duplicated here.

## Files

None recorded.
