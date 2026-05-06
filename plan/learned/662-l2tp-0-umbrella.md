# Learned: spec-l2tp-0-umbrella -- L2TP Full Stack

## What Was Built

Complete L2TPv2 LNS implementation across 14 child specs (1-wire through
12-ppp-interop-lab). 128 Go files in `internal/component/l2tp/` and 4
plugin directories (`l2tpauthlocal`, `l2tpauthradius`, `l2tppool`,
`l2tpshaper`). Web UI, observer/CQM, Prometheus metrics.

The stack: RFC 2661 wire format, reliable delivery with sequence windows,
tunnel/session FSMs, PPPoL2TP kernel integration, PPP engine (LCP, PAP,
CHAP-MD5, MS-CHAPv2, IPCP), subsystem wiring with component registration,
plugin infrastructure (drain channels, panic recovery, async handlers),
local auth + bitmap IP pool, RADIUS auth/acct/CoA, TC shaper (TBF/HTB),
session observer with CQM quality tracking, Prometheus metrics with
30s kernel counter polling, web management UI with uPlot CQM charts.

## Key Decisions

- **Reactor pattern throughout.** No per-tunnel goroutines. Reactor
  multiplexes all tunnel/session state machines on a single goroutine
  per reactor instance.

- **Plugin isolation via drain channels.** Plugins never block the PPP FSM.
  Typed channels + drain goroutines with panic recovery keep the session
  alive across handler failures.

- **Pre-allocation for BNG scale.** Observer rings, CQM sample buffers,
  and pool bitmaps are all pre-allocated at startup. Zero runtime
  allocation on the hot path.

- **EventBus for cross-cutting concerns.** Observer, shaper, pool cleanup,
  and accounting all subscribe to typed events. No direct coupling between
  the session lifecycle and side-effect plugins.

## Scope

All 13 umbrella ACs (tunnel handshake, session setup, IPCP, teardown,
CLI show commands, L2TPv3 rejection, auth challenge, retransmission,
config-driven startup, RADIUS auth, multi-session tunnels) are covered
by the child spec implementations.
