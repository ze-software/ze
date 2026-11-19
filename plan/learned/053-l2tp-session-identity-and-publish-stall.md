# 053: L2TP session identity and the publish stall

Spec: spec-l2tp-session-identity-and-publish-stall (closed 2026-11-19).

## What was fixed

- An L2TP session ID is unique inside one tunnel only (RFC 2661 Section 3.1),
  and tunnel IDs are reactor-local, so equal tunnel and session IDs are
  reachable across listeners. A logical birth (`subscriberid.L2TPBirth`) is
  minted once at admission and carried by every lifecycle event. The route
  observer, the CQM observer, accounting and CoS key by it.
- A PPP event reaches a session only through the admitted control, birth and
  subscriber ID (`pppSessionLocked`). A refused event writes one debug line.
- Every teardown (CLI clear, Disconnect-Request, session and idle timers) acts
  on a captured original (`CaptureSession`). The tuple-only
  `teardownSessionByID` is deleted. The CLI gained `tunnel-id`, `listener` and
  `expected-id`, and an ambiguous selector is refused with no effect.
- The Accounting-Stop moved onto the session's own accounting task, ordered
  after its Start and Interim-Updates, so the reactor never waits on RADIUS.
- A removed session whose cleanup is unconfirmed keeps its tunnel's admission
  slot, and a discarded tunnel still holding such roots keeps a max-tunnels
  slot, so retained cleanup roots are at most max-tunnels times max-sessions.

## Lessons

- A key that drops a fact the call already holds is found by reading the
  map's declaration, not its callers. The observers stored the tunnel ID and
  still keyed by the session ID.
- A narrowing at a function signature warns nobody: the timers held both IDs
  and passed one.
- A retention set whose entries leave only on confirmation needs its bound
  stated, and the bound has to be real. The first stated bound for
  `retiredSessions` did not hold, because a removed session freed its
  admission slot. Proving the bound showed the slot accounting was wrong.
- A test fixture that builds a bare session after admission starts to require
  an identity hangs on Linux rather than failing. Bound every wait in a
  fixture (`deliverOwnedKernelSuccess`, `e93e1b37c2`).
