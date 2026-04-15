# 597 -- L2TP Session State Machine

## Context

L2TP tunnels (phase 3) carried no sessions. A tunnel could be established via
SCCRQ/SCCRP/SCCCN, kept alive with HELLO, and torn down with StopCCN, but no
calls could be placed through them. Phase 4 adds the session state machines
from RFC 2661 Section 10: the LNS-side incoming call (ICRQ/ICRP/ICCN) and the
LAC-side outgoing call (OCRQ/OCRP), plus CDN teardown, WEN/SLI handling,
proxy LCP/auth capture, and per-tunnel session limits.

## Decisions

- Split session code into session.go (struct + helpers) and session_fsm.go
  (handlers + parsers + builders), mirroring the tunnel.go/tunnel_fsm.go pattern,
  over putting everything in a single file. Keeps both files under 1100 lines.
- Sessions share the tunnel's ReliableEngine over having per-session engines.
  RFC 2661 sequences session messages at tunnel level.
- Session ID allocation uses crypto/rand + collision retry (maxAllocRetries=100)
  over sequential allocation. Prevents predictable IDs.
- Deferred LAC-initiated sessions (incoming LAC side, outgoing LNS side) to a
  separate spec within the L2TP set over implementing stubs. These require
  LAC-initiated tunnel creation (sending SCCRQ) which phase 3 does not have.
- MaxSessions is per-tunnel (not global) over a global limit. Matches RFC 2661's
  model where each tunnel independently manages its sessions.
- Unknown M=1 vendor AVPs tear down the session with CDN, not the tunnel with
  StopCCN, per RFC 2661 Section 24.12.

## Consequences

- Phase 5 (kernel data plane) can now trigger on session-established state to
  create kernel l2tp_ppp sessions.
- Phase 6 (PPP engine) has proxy LCP/auth fields populated from ICCN, ready
  for PPP negotiation.
- LAC-initiated session spec must be completed before the L2TP set closes.
  Two deferrals tracked in plan/deferrals.md (AC-20, AC-21).
- handleOCCN exists and is tested but only reachable via LNS-initiated outgoing
  calls, which are deferred. It will become reachable when LAC-initiated tunnels
  are added.

## Gotchas

- Header Session ID in ICRP/OCRP must be the peer's assigned SID (recipient's
  ID), not our local SID. The initial implementation had this backward. Caught
  in /ze-review before commit.
- ZLB ACKs interleave with session messages in .ci tests. Python test scripts
  must skip ZLBs (length <= 12, no body) when waiting for specific message types.
  The stopccn-cascade test failed on its first run because a ZLB arrived where
  ICRP was expected.
- Ns/Nr tracking in multi-session .ci tests is brittle. Each message must use
  the correct Ns (incremented after each send) and Nr (last seen from ze).
  Getting this wrong causes ze's reliable engine to drop messages silently.

## Files

- `internal/component/l2tp/session.go` -- session struct, state enum, map helpers
- `internal/component/l2tp/session_fsm.go` -- all handlers, parsers, builders
- `internal/component/l2tp/session_fsm_test.go` -- unit tests (28 tests)
- `internal/component/l2tp/tunnel.go` -- sessions map, maxSessions added
- `internal/component/l2tp/tunnel_fsm.go` -- session dispatch, StopCCN cascade
- `internal/component/l2tp/config.go` -- MaxSessions field + YANG + env var
- `test/l2tp/session-incoming-lns.ci` -- ICRQ/ICCN functional test
- `test/l2tp/session-cdn-teardown.ci` -- CDN functional test
- `test/l2tp/session-stopccn-cascade.ci` -- StopCCN cascade functional test
- `test/parse/l2tp-max-sessions.ci` -- config parse test
- `docs/guide/configuration.md` -- max-sessions doc
- `rfc/short/rfc2661.md` -- session state machines section
