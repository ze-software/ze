# 667: BNG Scale Testing Infrastructure

**Spec:** spec-bng-4-scale-testing
**Date:** 2026-05-08
**Outcome:** Delivered test infrastructure for L2TP control-plane scale validation

## What Was Built

Go LAC simulator (`ze-test l2tp-scale`) that speaks real L2TP wire protocol
(SCCRQ/SCCRP/SCCCN, ICRQ/ICRP/ICCN, CDN, StopCCN) over UDP loopback with
tunnel CHAP-MD5 authentication. Includes an embedded mock RADIUS server
(accept-all auth, accounting Start/Stop/Interim tracking, configurable latency).

Python orchestration harness (`test/l2tp-scale/`) with four scenarios:
2k-sessions, clean-teardown, pool-exhaustion, slow-radius.

## Key Design Decisions

1. **fakel2tp excluded:** fakel2tp is an EventBus route emitter, not a wire
   peer. Scale testing needs real L2TP wire protocol.

2. **Control-plane only:** No kernel PPP data plane (no pppol2tp, no pppN
   interfaces). The interop lab covers the kernel path at small scale; this
   spec covers Ze's own code at large scale.

3. **Loopback only:** No root, no namespaces, no Docker, no kernel modules.
   Runs on macOS and Linux.

4. **Single binary:** LAC simulator + mock RADIUS in one `ze-test l2tp-scale`
   process. Follows the `ze-test peer --mode inject` precedent for BGP stress.

5. **Reuse wire types:** Simulator uses `l2tp.WriteControlHeader`,
   `l2tp.WriteAVP*`, `l2tp.ParseMessageHeader`, `l2tp.NewAVPIterator` and
   `radius.Decode`/`radius.ResponseAuthenticator` from the production packages.

## Patterns Worth Reusing

- **Mock server in ze-test:** The mock RADIUS pattern (atomic counters, goroutine
  per packet, configurable latency) works for any UDP mock. Similar to the
  existing `tacacs_mock.go` pattern for TCP.

- **drainZLB helper:** After sending a control message, read with short timeout
  and discard ZLB ACKs (header-only, Length <= PayloadOff). Needed because Ze's
  reliable transport sends ZLBs that must be consumed before the next send.

## What Could Not Be Tested Here

- AC-3 (memory per session) and AC-10 (CPU profile) require pprof scraping
  during an actual Ze run. The harness infrastructure supports this but the
  scenarios do not yet exercise it.

- AC-8 (graceful shutdown) requires sending SIGTERM to Ze during the dwell
  phase and verifying accounting-stop is sent for each session.

- TestLACSimMultiSession was deferred: it needs a running Ze LNS to test
  against, making it an integration test rather than a unit test.

## Files

None recorded.
