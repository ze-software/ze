# 596 -- L2TP Tunnel State Machine

## Context

Ze needed L2TPv2 tunnel lifecycle support to act as an LNS. The wire layer
(header parsing, AVP encoding, hidden AVP encryption, challenge/response
authentication) and reliable delivery engine already existed from phases 1-2.
Phase 3 (this spec) delivered the tunnel state machine, UDP listener, reactor
and timer goroutines, and the subsystem registration that makes L2TP a
config-driven component of ze.

## Decisions

- **Single reactor goroutine** over per-tunnel goroutines, because
  ReliableEngine is not safe for concurrent use and the tunnel map must be
  consistent. Timer goroutine communicates via two channels (tickReq,
  heapUpdate), never touches tunnel state directly.
- **FSM tests live in reactor_test.go** over a separate tunnel_fsm_test.go,
  because the FSM is exercised through reactor.handle() with real UDP
  loopback. Standalone FSM tests would duplicate setup and miss wiring bugs.
- **No listen-bind.ci or reject-v3.ci** created. Bind proof is subsumed by
  handshake-sccrq.ci (which fails if the socket is not bound). V3 rejection
  is covered by TestReactor_V3Dropped with real UDP, and a .ci test would
  need a Python client to craft a V3 packet for no additional coverage.
- **Parameters** as the config struct name over Config, because
  check-existing-patterns.sh blocks duplicate first-struct names and SSH
  already uses Config.
- **Split YANG config**: protocol settings (enabled, shared-secret,
  hello-interval, max-tunnels) under root `l2tp {}`, listener endpoints
  under `environment { l2tp { server } }`. L2TP is a protocol subsystem
  like BGP, not just a service, so protocol settings belong at root level.
- **Presence implies enabled**: `l2tp {}` block existing means enabled=true.
  `enabled false` to disable. `enabled true` as filler when no other
  settings needed.

## Consequences

- Session FSM (ICRQ/ICRP/ICCN/CDN) can build on this tunnel lifecycle.
  Each session will reference a parent tunnel and share its ReliableEngine.
- The timer goroutine's min-heap pattern is reusable for session-level
  timers without adding goroutines.
- The secondary index (peer addr:port, remote TID) prevents duplicate tunnel
  creation from retransmitted SCCRQs but allows multiple tunnels per peer.

## Gotchas

- Challenge AVP length validation MUST run at the reactor edge (parseSCCRQ),
  not inside the FSM. auth.ChallengeResponse panics on empty challenge/secret,
  and a peer can send a zero-length Challenge AVP.
- bytes.Buffer for slog in tests races between reactor and test goroutines.
  Use a mutex-guarded wrapper (lockedBuffer).
- YANG list entries require newline-separated leaves; compact one-line form
  (`server a { ip 1.1.1.1 port 1701 }`) produces a parse error.
- block-silent-ignore.sh rejects `default:` in switch statements. Use
  if/else chains instead.

## Files

- `internal/component/l2tp/` (39 .go files, 6 phases total)
- Key phase 3 files: `subsystem.go`, `reactor.go`, `tunnel.go`,
  `tunnel_fsm.go`, `timer.go`, `listener.go`, `config.go`, `register.go`
- Schema: `internal/component/l2tp/schema/ze-l2tp-conf.yang`
- Runner: `cmd/ze-test/l2tp.go`
- Tests: `test/l2tp/*.ci` (3), `test/parse/l2tp-*.ci` (3)
