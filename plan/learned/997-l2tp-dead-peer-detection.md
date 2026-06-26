# 997 -- l2tp-dead-peer-detection

## Context
When a remote LAC (xl2tpd in the interop/evidence labs) dies, it never sends a
StopCCN: the wire trace shows SCCRQ->ICCN then silence. Ze's only dead-peer
signal was HELLO retransmit exhaustion in the reliable engine (default schedule
1+2+4+8+16 = ~31s). With the lab's `hello-interval 5` total detection was ~36s,
which blew the gokrazy evidence harness's 20s teardown window
(`effective-gokrazy-l2tp-ppp.py`, "subscriber routes withdrawn") and the
interop check's 30s window. The goal was fast dead-peer detection without
weakening setup/teardown link-loss tolerance. The follow-up was flagged in
commit e231fbfdd ("a separate issue: Ze not acting on the peer StopCCN
promptly").

## Decisions
- Detect death via a dedicated `lastLiveness` timestamp (delivered control
  message OR an acknowledgement of one of our messages, including a ZLB ACK of
  a HELLO), tearing down when it ages past `hello-retries x hello-interval`.
  Chosen over: (a) a hold-timer on `lastActivity` -- which is deliberately NOT
  refreshed by ZLB ACKs, so it would false-kill an idle-but-alive peer; and
  (b) shortening the reliable-engine retransmit -- which would weaken link-loss
  tolerance for SCCRQ/StopCCN and break RFC 2661 S5.8 retention.
- Two distinct clocks: `lastActivity` (delivered-only) decides *when to probe*
  (unchanged); `lastLiveness` (delivered-or-acked) decides *when the peer is
  dead*. Conflating them is exactly the bug both naive fixes fall into.
- Gate dead-peer detection strictly on `L2TPTunnelEstablished` so setup
  (pre-established) and teardown (closed) retain the full ~31s retransmit budget.
- Surfaced `ReceiveResult.Acked` (count from `processNr`) so the tunnel can
  treat a bare ZLB ACK as proof of liveness; the engine stays unaware of DPD.
- Knob `hello-retries` (uint8, default 2, 0 disables) over an absolute
  `dead-peer-timeout` duration; expressing it as a multiple of `hello-interval`
  matches operator intent and avoids a second timer to misconfigure. Default 2
  gives ~10s detection at `hello-interval 5` (user decision 2026-06-26).

## Consequences
- Steady-state Established link-loss tolerance is now `hello-retries x
  hello-interval`; this is the intended, configurable trade-off of dead-peer
  detection. Setup and teardown are unaffected (gated on Established).
- When `hello-retries x hello-interval > ~31s` (e.g. defaults `2 x 60s`),
  retransmit exhaustion fires first and the threshold has no effect; lower
  `hello-interval` for faster detection.
- Tunnel-down reason `keepalive-timeout` (distinct from `retransmit-timeout`)
  lets operators tell "peer ignored keepalives" from "control message lost".
- `hello-retries` is parsed by BOTH config parsers (`ExtractParameters` and the
  reload-path `extractFromProvider`), hot-applied via `setHelloRetries`, and
  exposed through `show l2tp config` and the web `/l2tp` form.

## Gotchas
- There are TWO L2TP config parsers (config.go `ExtractParameters` and
  subsystem_reload.go `extractFromProvider`); a new leaf must be added to both,
  including the default in each struct literal, or reload diverges from boot.
- `ReactorParams` (reactor.go) is a separate struct from `Parameters`
  (config.go); the reactor reads `r.params` (ReactorParams). New runtime knobs
  must be added to both and mapped in subsystem.go.
- Adding a default-valued leaf broke `TestReloadNoOpOnIdentical`: the no-op
  baseline Parameters must include the new field's default so prev==next.
- The web config form test asserts an exact field count; inserting a row after
  `hello-interval` keeps indices 0..4 valid but the count assertions need
  bumping.
- `ReceiveResult.Acked` must be set in every post-ack `OnReceive` return path
  (ZLB, delivered, duplicate, reorder, discard); the data-message early return
  leaves it 0 (Nr untrusted per RFC trap 24.4).

## Files
- `internal/component/l2tp/`: reliable.go (Acked), tunnel.go (lastLiveness),
  tunnel_fsm.go (Process/handleSCCCN), reactor.go (handleTick DPD +
  ReactorParams), config.go + subsystem_reload.go (parse/default/hot-apply),
  reactor_setters.go (setHelloRetries), subsystem.go (mapping),
  snapshot.go/subsystem_snapshot.go/cmd/l2tp.go (show config),
  yang/ze-l2tp-conf.yang (leaf).
- `internal/component/web/page_l2tp.go`: config-form row.
- Tests: reliable_test.go, reactor_test.go, config_test.go,
  subsystem_reload_test.go, page_l2tp_test.go; test/parse/l2tp-hello-retries-*.ci;
  test/plugin/reload-hello-retries.ci.
- Docs: docs/guide/l2tp.md, docs/features.md, docs/functional-tests.md,
  rfc/short/rfc2661.md.
