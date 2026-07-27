# 1277 -- fixit-rfc6286-bgp-identifier

## Context

`plan/learned/1270-fixit-load-dependent-functional-failures.md` recorded three RFC 6286
limitations as "known, not fixed", and the owner asked for them closed. An OPEN whose BGP
Identifier was zero, or which presented this speaker's own identifier from an internal
peer, was accepted rather than answered with OPEN Message Error / Bad BGP Identifier. The
AS-wide uniqueness check was a check-then-act over peers that had already reached
Established, so two same-AS peers presenting the same identifier concurrently were BOTH
accepted -- the load-dependent functional failure that surfaced it. And RFC 6286 support
was undisclosed: no `rfc/short/` summary, no `docs/features/rfc-status.md` row.

While tracing the race it became clear Section 2.3 (connection collision between speakers
with identical identifiers) was a small delta on the existing collision path, and is the
RFC's own answer for the shared identifier that `allow-shared-router-id` permits, so it
was pulled into scope rather than left as the only open corner of a small RFC.

## Decisions

- **Validate at OPEN and claim atomically, rather than reconcile after Established.**
  `routerIDClaims.claim` performs the entire check-and-take inside one critical section,
  so exactly one peer can win an identifier. Chosen over widening the old
  post-Established scan, which cannot be made race-free: any check that returns before
  the winner is recorded leaves a window.
- **Gate the self-identifier rejection on the peer being internal**, per RFC 6286 Section
  2.2. An external peer may legitimately carry the same identifier as this speaker -- that
  is what AS-wide rather than global uniqueness means -- and Section 2.3, not the
  validator, resolves the resulting collision.
- **Keep `allow-shared-router-id` as an operator opt-out.** RFC 6286 Section 2.1 says the
  identifier "should" be unique, lowercase, so enforcing it by default with an escape
  hatch matches the RFC's own strength rather than exceeding it.
- **Empty Data field on the Notification.** RFC 4271 Section 6.2 defines no data for the
  Bad BGP Identifier subcode, unlike Unsupported Version and Unacceptable Hold Time which
  echo the offending value.

## Consequences

- Both OPEN rails validate independently: normal receive, and the connection that wins
  collision resolution. They are separately gated, so a fix to one does not silently
  cover the other -- a mutation dropping the check from the collision rail alone turns
  exactly one test red.
- An identifier claim is held for the life of a session and released on every teardown
  path. The release defer is installed before `session.Start()`, so a failure during
  capability negotiation still frees the identifier; a leak here would permanently lock
  out a legitimate peer.
- RFC 6286 is enrolled in the requirements ledger with all five requirement ids bound, so
  `make ze-rfc-check` now ratchets on it: losing a polarity is a gate failure, not a quiet
  regression.

## Gotchas

- **The equal-identifier collision branch is reachable by an internal peer, contrary to
  what the code said for most of this work.** A comment asserted that Section 2.2 had
  already rejected such a peer, and that equal AS numbers therefore could not reach the
  branch. The order is the reverse: `DetectCollision` runs from `ResolvePendingCollision`
  on the PENDING open, and the Section 2.2 check runs later and only on the connection
  that WINS. The outcome was never wrong -- `PeerAS > LocalAS` is false for equal AS
  numbers, so the existing connection is kept -- but the safety came from the comparison,
  not from an upstream guarantee. An edit trusting "equality is impossible" would have
  introduced a real bug. Independent review caught it; nothing else could have.
- **This spec sat finished but unclosed, and the closure detector could not see it.**
  Status was `in-progress` at Phase 5/5 with every goal met, but its Review Gate section
  was empty template and no learned summary existed. `spec-closure-check.py` keys its
  high-confidence detection on a committed `plan/learned/NNN-<slug>.md` matching the spec
  stem -- exactly the artifact that was missing. **A spec that never got its summary
  written is invisible to the tool built to find unclosed specs.** Do not read that
  tool's silence as evidence a spec is genuinely open; check Phase against the goals.
- The global `router-id` zero rejection has no test binding of its own. Behaviour is
  triple-covered (`mandatory true`, `ze:validate "nonzero-ipv4"`, and `parseRouterID`),
  but the enforcing test and the ledger id both bind the per-peer leaf, so a mutation to
  the shared helper only turns the peer-level test red.
- `parsePeersFromTree` parses a peer `router-id` with bare `netip.ParseAddr` and accepts
  `0.0.0.0`, bypassing `parseRouterID`. Unreachable in production because that fallback
  needs `reloadFunc == nil || ConfigPath == ""` and both are always set, but it is a
  second non-validating parse of a leaf the public status row now makes a claim about.

## Files

- `internal/component/bgp/message/open.go` -- `ValidateBGPIdentifier` (RFC 6286 Section 2.2)
- `internal/component/bgp/reactor/routerid_unique.go` -- atomic identifier claim/release
- `internal/component/bgp/reactor/session_open_validation.go` -- shared OPEN validation
- `internal/component/bgp/reactor/session.go` -- `DetectCollision`, Section 2.3 tie-break
- `internal/component/bgp/reactor/session_handlers.go`, `session_connection.go` -- the rails
- `internal/component/bgp/reactor/config.go` -- `parseRouterID` non-zero check
- `rfc/short/rfc6286.md`, `rfc/enrolled.txt`, `docs/features/rfc-status.md`
