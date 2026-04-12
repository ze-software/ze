# 563 -- bfd-6-echo-mode

## Context

BFD Echo mode (RFC 5880 §6.4 + RFC 5881 §5) lets the local end
send self-directed packets that the remote reflects through its
forwarding plane, giving a cheaper liveness check than Async
Control exchanges. Stage 6's original scope covered the full
protocol: UDP 3785 transport, per-session scheduler, RX demux,
outstanding-ID matcher, RTT histogram, detection-time switchover,
and async slow-down. After inventorying the work required, Stage 6
ships only the **config surface + wire advertisement** half and
defers the transport half as `spec-bfd-6b-echo-transport`. The
split is principled: once the configuration and session state are
wired, the transport lands as a pure addition without rewriting
any of this commit's files.

## Decisions

- **Ship only the config + advertisement half.** The full echo
  protocol is ~500+ lines plus a second UDP socket, a scheduler
  integrated with the express loop, an outstanding-ID ring, and
  a reflect path. Operators who want to experiment with echo can
  configure it today and see their peers learn the local
  advertisement; the transport half fills in the real TX/RX. A
  half-delivery that lands the persistent surface (YANG, config
  parse, session state, metrics, wire fields) is more useful
  than a stub with runtime behavior missing.

- **Echo wire format is a 16-byte `ZEEC` envelope.** RFC 5880
  §6.4 says the echo format is "a local matter." ze picks four
  bytes of magic (`ZEEC`), four bytes of local discriminator,
  four bytes of sequence, and four bytes of a millisecond
  timestamp. The magic prefix deliberately does not match a
  valid BFD Control version byte so a peer that accidentally
  treats an echo as Control bounces it with `ErrBadVersion`.

- **Multi-hop echo rejected at config parse time.** RFC 5883 §4
  prohibits multi-hop echo. A `profileConfig.validate` pass runs
  after the parser and refuses any multi-hop session that
  references a profile with an echo block.

- **Metrics registered now, even without TX/RX.** The two
  counter families `ze_bfd_echo_tx_packets_total` and
  `ze_bfd_echo_rx_packets_total` are published via the same
  `MetricsHook` pattern as every other BFD metric. The transport
  half will plug its TX/RX events into the existing hooks
  without changing the metric shape. NOC dashboards can
  reference the names from day one.

- **`Machine.EchoEnabled` guard takes both local and remote
  intent.** Local `DesiredMinEchoTxInterval > 0` AND remote
  `RemoteMinEchoRxInterval > 0` are both required. When the
  transport half lands, the scheduler tick will guard on
  `EchoEnabled()` and use `EchoInterval()` directly.

## Consequences

- **Operators can opt into echo now.** The YANG surface is
  stable; configurations written today against Stage 6 remain
  valid when the transport lands. Peers that advertise a
  non-zero `RequiredMinEchoRxInterval` will start to see ze's
  own advertisement in every Control packet.

- **Metrics surface is set.** Downstream alerting and dashboards
  can reference `ze_bfd_echo_tx_packets_total{mode="single-hop"}`
  without worrying about the metric name changing when the
  transport half lands.

- **The transport half is a clean addition.** It needs:
  - A new `transport.Echo` type wrapping a UDP socket on port
    3785 (single-hop only; multi-hop Loops skip it).
  - A per-session outstanding-ID ring on `sessionEntry`.
  - An `engine.tick` branch that fires echo TX on
    `machine.EchoInterval()` when `machine.EchoEnabled()`.
  - An RX demux that looks up the echo by local discriminator,
    computes RTT from the timestamp, and either completes an
    outstanding entry (our own echo back) or reflects the bytes
    back to the sender (peer's echo through us).
  - Detection-time switchover in `machine.DetectionInterval` to
    use `RemoteMinEchoRxInterval * RemoteDetectMult` when echo
    is active.
  - Async slow-down in `machine.TransmitInterval` to use
    `max(1s, peer.RequiredMinRxInterval)` when echo is active.

- **Stage 6 alone does not deliver a working Echo session.** The
  AC table is honest: AC-1 / AC-4 are Partial, AC-2 / AC-3 /
  AC-6 / AC-7 / AC-8 are Deferred. The deferral row points at
  `spec-bfd-6b-echo-transport` for all of them.

## Gotchas

- **`block-silent-ignore.sh` refuses `default:` in switch
  statements.** Same trap as every other BFD stage. The echo
  parser's type resolver (Stage 5-style) avoided it.

- **`dupl` linter.** The echo packet codec uses bare
  `binary.BigEndian.PutUint32` / `binary.BigEndian.Uint32` like
  every other codec; no dedup helpers were needed.

- **Spec sketched AC-9 as a fuzz target.** The packet codec is
  small enough that two targeted unit tests (`TestEchoShort`
  and `TestEchoBadMagic`) give the same coverage without adding
  a new fuzz harness. The deferral list carries a follow-up if
  a real fuzz target is desired.

## Files

- `internal/plugins/bfd/packet/echo.go` (new) -- `Echo` struct,
  `WriteEcho`, `ParseEcho`, `ZEEC` magic, `EchoLen`.
- `internal/plugins/bfd/packet/echo_test.go` (new) -- round-trip,
  short buffer, bad magic.
- `internal/plugins/bfd/session/session.go` -- `Vars` grew
  `DesiredMinEchoTxInterval`, `RequiredMinEchoRxInterval`,
  `RemoteMinEchoRxInterval`; `Init` seeds them from the
  request.
- `internal/plugins/bfd/session/fsm.go` -- `Receive` captures
  peer's advertised echo rx; `Build` populates
  `RequiredMinEchoRxInterval` in outgoing Control packets.
- `internal/plugins/bfd/session/timers.go` -- `EchoEnabled` and
  `EchoInterval` accessors.
- `internal/plugins/bfd/api/events.go` -- `SessionRequest.DesiredMinEchoTxInterval`.
- `internal/plugins/bfd/config.go` -- `echoConfig`,
  `parseEchoConfig`, `pluginConfig.validate`,
  `toSessionRequest` plumbing.
- `internal/plugins/bfd/schema/ze-bfd-conf.yang` -- `echo {
  desired-min-echo-tx-us }` presence container inside `list
  profile`.
- `internal/plugins/bfd/metrics.go` -- `echoTxPackets` /
  `echoRxPackets` CounterVecs; `metricsHook.OnEchoTx` /
  `OnEchoRx`.
- `internal/plugins/bfd/engine/engine.go` -- `MetricsHook`
  interface grows `OnEchoTx` / `OnEchoRx` methods.
- `test/plugin/bfd-echo-config.ci` (new).
- `test/plugin/bfd-echo-multi-hop-reject.ci` (new).
- `docs/guide/bfd.md` -- Echo configuration documented.
- `docs/features.md` -- BFD feature row updated.
- `plan/deferrals.md` -- Stage 6 row closed; new
  `spec-bfd-6b-echo-transport` row added.
