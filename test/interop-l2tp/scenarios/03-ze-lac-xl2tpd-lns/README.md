# 03 — ze initiator (LAC/dialer) vs xl2tpd LNS

Spec: `plan/spec-followup-l2tp-call.md` (AC-7). This is the inverse of scenarios  <!-- doc-links: ignore (interop scenario this document plans; it does not exist in the tree) -->
01/02: here **ze dials** a real `xl2tpd` daemon running as an LNS, proving the
new initiator half of the L2TPv2 tunnel FSM (SCCRQ initiation → SCCRP handling →
SCCCN → established) interoperates with an independent RFC 2661 implementation.

## What it proves

`ze` sends the SCCRQ; `xl2tpd` answers SCCRP; `ze` sends SCCCN; the L2TP control
connection is **established on both sides**. Verbatim evidence:

- ze: `l2tp: tunnel now established (initiator) ... peer-host="claude" peer-tid=<n>`
- xl2tpd: `Connection established to 172.29.0.2, <ze-port>. Local: <n>, Remote: <n> ... LNS session is 'default'`

## Run it

```
ZE_L2TP_INTEROP_SCENARIO=03-ze-lac-xl2tpd-lns le deployment docker-l2tp-ppp-test
```

The native scenario plan starts both `xl2tpd` and `ze` in privileged Docker
containers on the isolated `172.29.0.0/24` lab network. It mounts this
directory's `xl2tpd.conf`, `options.xl2tpd`, and a rendered `ze.conf`, then
waits for the xl2tpd listener and Ze's L2TP and REST listeners. The checker
POSTs `request l2tp outgoing-call remote xl2tpd called 12345` to Ze's
token-guarded REST API and requires the established control connection in both
peer logs. Because it exercises only the control connection, the native suite
skips the PPPoL2TP kernel preflight when this scenario is selected alone.

## Why the OCRQ does not complete here (A-6)

The `request l2tp outgoing-call` RPC dials the remote and then attempts an OCRQ
(LNS-side outgoing call). **xl2tpd does not implement the outgoing-call answerer
side**: it logs `message type 7 (Outgoing-Call-Request)` and closes the tunnel
(`control.c` has no `case OCRQ:` — a received OCRQ hits the "Unimplemented
message 7" default). So the RPC returns an error (`outgoing call torn down before
it established`) *by design* — the interop proof is the established control
connection, not the call.

The full OCRQ → OCRP → OCCN call flow is proven at the functional tier by
`test/l2tp/lns-outgoing-call.ci`, whose independent LAC peer answers the OCRQ.
Of the freely available Linux L2TPv2 daemons, only **accel-ppp**
(`accel-pppd/ctrl/l2tp`, tunnel `mode lac`) answers an OCRQ with OCRP+OCCN, and
only through source-only CLI (`l2tp create tunnel ... mode lac`) not covered by
its manual. This satisfies the A-6 interop-exemption path in
`ai/rules/interop-and-goal-validation.md`: the wire behaviour is proven
functionally and against a real peer for the tunnel, with the call-answerer gap
documented.

## Env-blocked: full ze-as-LAC incoming call with PPP up (A-4)

The other LAC flow — a PPPoE subscriber relayed into an L2TP **incoming** call
(ICRQ), with subscriber PPP frames bridged to the LNS — needs (a) a PPPoE
subscriber harness and (b) the kernel channel bridge (`PPPIOCBRIDGECHAN`,
`bridge_linux.go`), which requires `CAP_NET_ADMIN` and the PPPoL2TP kernel
modules. That data-plane leg is exercised by the authored, `//go:build
integration && linux` bridge test and is run under QEMU:

```
le deployment gokrazy-l2tp-ppp-test
```

This scenario deliberately does not attempt it. Its control-plane initiator
proof runs in the isolated Docker lab without requiring PPPoL2TP kernel state.
