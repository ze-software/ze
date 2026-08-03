# 1100 -- followup-l2tp-call

## Context

Ze's L2TPv2 (RFC 2661) subsystem could only ANSWER tunnels/calls (LNS). The
initiator half was FSM stubs: no SCCRQ/SCCCN/ICRQ/ICCN/OCRQ encoders, three
dead receive stubs (SCCRP/ICRP/OCRP), never-entered states (wait-ctl-reply,
wait-reply), and no way to dial a remote. This spec added the initiator half so
ze can dial a tunnel, place an LNS-side outgoing call (OCRQ) via the `request
l2tp outgoing-call` RPC, and relay a PPPoE subscriber into a LAC incoming call.
Delivered across seven code phases (b68e7e9c9 → c44fe82d5) plus a last-mile
closure session (AC-4 functional `.ci`, AC-7 real-peer interop).

## Decisions

- Functional `.ci` trigger = the `request l2tp outgoing-call` RPC dispatched over
  the **token-guarded REST API** (`/api/v1/execute`), chosen over SSH-CLI (needs
  keys, unused by `.ci`) and a config-driven auto-dial (contradicts the "calls
  are operational events" design). No-auth REST is read-only, so the config sets
  `api-server { token "secret" }` and the peer sends `Authorization: Bearer` —
  the only way to let a `.ci` invoke a mutating `request` verb.
- The inline Python peer BOTH triggers the dial (a background thread POSTs the
  blocking RPC) AND answers on the wire (main thread), on a fixed `$PORT2`. This
  is the answer to "ze is the initiator, so who makes it dial?"
- AC-7 real-peer interop = ze DIALS a real **xl2tpd LNS** and establishes the
  control connection (scenario 03), chosen over the ze-as-LAC-PPP-up path
  (needs a PPPoE subscriber harness + the CAP_NET_ADMIN kernel bridge). Scenario
  is self-contained (`run.py`, containerised peer + native `bin/ze`), so it needs
  no PPPoL2TP modules and runs unprivileged.
- LNS-outgoing real-peer answerer = documented **exemption** (A-6): source-verified
  that xl2tpd has NO OCRQ answerer; only accel-ppp (`mode lac`) answers, via
  undocumented source-only CLI. Wire proof kept at the functional tier
  (`lns-outgoing-call.ci`, Python LAC).

## Consequences

- ze can now interoperate as an L2TP LAC/initiator against a third-party LNS at
  the tunnel level (proven vs xl2tpd 1.3.18). The `remote` dial-target list +
  `request l2tp outgoing-call` RPC are the operator surface (docs/guide/l2tp.md).
- The token-guarded-REST-`/api/v1/execute` pattern is the reusable way for a
  `.ci` to drive any mutating in-process RPC against a running `ze -` daemon.
- Self-contained interop scenarios (a scenario dir shipping its own `run.py`) are
  now supported: `test/l2tp-interop/run.py` delegates to them and skips the
  ze=LNS preflight/image-build. Use this for any inverted-topology scenario.
- The LAC data plane (PPPoE↔pppol2tp kernel channel bridge, A-4) and the full
  ze-as-LAC PPP-up interop remain env-gated on CAP_NET_ADMIN: `make
  ze-qemu-l2tp-ppp-test`. Runbook in the scenario-03 README.

## Gotchas

- **xl2tpd cannot answer an OCRQ.** `control.c` has no `case OCRQ:`; a received
  message type 7 hits the default "Unimplemented message 7" and it closes the
  tunnel. So an ze-outgoing-call against xl2tpd fails BY DESIGN after the tunnel
  is up — the interop proof is the established control connection, not the call.
- **The L2TP reliable Ns/Nr must be mirrored for the initiator case.** The peer
  sends SCCRP `ns=0,nr=1`; after ze's SCCCN(ns=1)+OCRQ(ns=2) it replies OCRP
  `ns=1,nr=3` then OCCN `ns=2,nr=3`. Relying on loopback ordering (advance the
  peer's `nr` only on the next expected `ns`) keeps the Python peer simple.
- **`session-stopccn-cascade` is PRE-EXISTING, not our regression.** Built
  `bin/ze` at the parent commit `fe6aa242f` via `git archive` and ran it: fails
  3/3 there, identical to HEAD. Root cause is the answering-side reliable-receive
  window not advancing past a rapid second ICRQ, so a later StopCCN is never
  delivered. Logged in `plan/known-failures.md`.
- **`ze.l2tp.skip-kernel-probe=true` ⇒ `kernelWorker` is nil ⇒ session establishes
  at the control level with no kernel setup** (`collectKernelEventsLocked` returns
  early). This is why the outgoing-call `.ci` completes OCCN without CAP_NET_ADMIN.
- Set dotted env vars (`ze.l2tp.skip-kernel-probe`) with `env 'name=value'` — bash
  rejects dots in a bare `VAR=val` prefix (exit 127).
- The discovery index (`ai/PACKAGE-MAP.md`/`ai/DOCS-TO-CODE.md`) was stale for the
  `callsink` package since 794315507; `make ze-discovery-index` (not `make
  generate`) refreshes it, and `make ze-doc-test` gates it.

## Files

- Created (this session): `test/l2tp/tunnel-initiate-sccrq.ci`,
  `test/l2tp/lns-outgoing-call.ci`,
  `test/l2tp-interop/scenarios/03-ze-lac-xl2tpd-lns/{run.py,ze.conf,xl2tpd.conf,options.xl2tpd,README.md}`.
- Modified (this session): `test/l2tp-interop/run.py` (self-contained-scenario
  delegation), `docs/guide/l2tp.md`, `docs/features/rfc-status.md`,
  `docs/labs/l2tp-interop.md`, `plan/known-failures/RESOLVED.md`, `ai/PACKAGE-MAP.md`,
  `ai/DOCS-TO-CODE.md`, `plan/learned/1100-followup-l2tp-call.md`.
- Prior phases (committed b68e7e9c9…c44fe82d5): `tunnel_initiator.go`,
  `session_initiator.go`, `reactor_dial.go`, `outgoing_call.go`,
  `cmd/outgoing_call.go`, `relay_sink.go`, `internal/core/callsink/callsink.go`,
  `bridge_linux.go`, `yang/ze-l2tp-conf.yang`, and their tests.
