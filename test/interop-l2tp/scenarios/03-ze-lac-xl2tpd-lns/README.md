# 03 — ze initiator (LAC/dialer) vs xl2tpd LNS

Spec: `plan/spec-followup-l2tp-call.md` (AC-7). This is the inverse of scenarios  <!-- doc-links: ignore (interop scenario this document plans; it does not exist in the tree) -->
01/02: here **ze dials** a real `xl2tpd` daemon running as an LNS, proving the
new initiator half of the L2TPv2 tunnel FSM (SCCRQ initiation → SCCRP handling →
SCCCN → established) interoperates with an independent RFC 2661 implementation.

## What it proves

`ze` sends the SCCRQ; `xl2tpd` answers SCCRP; `ze` sends SCCCN; the L2TP control
connection is **established on both sides**. Verbatim evidence:

- ze: `l2tp: tunnel now established (initiator) ... peer-host="claude" peer-tid=<n>`
- xl2tpd: `Connection established to 127.0.0.1, <ze-port>. Local: <n>, Remote: <n> ... LNS session is 'default'`

## Run it

```
python3 test/interop-l2tp/run.py 03-ze-lac-xl2tpd-lns
# or directly:
python3 test/interop-l2tp/scenarios/03-ze-lac-xl2tpd-lns/run.py
VERBOSE=1 python3 .../run.py     # dump ze + xl2tpd logs
```

The scenario is **self-contained**: `xl2tpd` runs in Docker (`--network host`,
port 17010, reusing the lab's `Dockerfile.lac` image); `ze` runs from the repo's
`bin/ze` with isolated filesystem storage (`ZE_STORAGE_BLOB=false`, temp cwd) so
it never touches the committed `etc/ze` blob. The dial is triggered by POSTing
`request l2tp outgoing-call remote xl2tpd called 12345` to ze's token-guarded
REST API. Because it exercises only the control connection (no PPP data plane),
it needs no PPPoL2TP kernel modules and runs unprivileged — unlike the ze=LNS
scenarios 01/02, so the shared `preflight_strict()` / ze-image build are skipped
for it.

## Why the OCRQ does not complete here (A-6)

The `request l2tp outgoing-call` RPC dials the remote and then attempts an OCRQ
(LNS-side outgoing call). **xl2tpd does not implement the outgoing-call answerer
side**: it logs `message type 7 (Outgoing-Call-Request)` and closes the tunnel
(`control.c` has no `case OCRQ:` — a received OCRQ hits the "Unimplemented
message 7" default). So the RPC returns an error (`outgoing call torn down before
it established`) *by design* — the interop proof is the established control
connection, not the call.

The full OCRQ → OCRP → OCCN call flow is proven at the functional tier by
`test/l2tp/lns-outgoing-call.ci` (an inline Python LAC peer that answers the
OCRQ). Of the freely-available Linux L2TPv2 daemons, only **accel-ppp**
(`accel-pppd/ctrl/l2tp`, tunnel `mode lac`) answers an OCRQ with OCRP+OCCN, and
only through source-only CLI (`l2tp create tunnel ... mode lac`) not covered by
its manual; wiring it in is left as a follow-up. This satisfies the A-6
interop-exemption path in `ai/rules/interop-and-goal-validation.md`: the wire
behaviour is proven functionally + against a real peer for the tunnel, with the
call-answerer gap documented.

## Env-blocked: full ze-as-LAC incoming call with PPP up (A-4)

The other LAC flow — a PPPoE subscriber relayed into an L2TP **incoming** call
(ICRQ), with subscriber PPP frames bridged to the LNS — needs (a) a PPPoE
subscriber harness and (b) the kernel channel bridge (`PPPIOCBRIDGECHAN`,
`bridge_linux.go`), which requires `CAP_NET_ADMIN` and the PPPoL2TP kernel
modules. That data-plane leg is exercised by the authored, `//go:build
integration && linux` bridge test and is run under QEMU:

```
make ze-qemu-l2tp-ppp-test      # CAP_NET_ADMIN host / QEMU VM required
```

This scenario deliberately does not attempt it; the control-plane initiator
interop above is the portion runnable in an unprivileged Docker environment.
