# 023 — Collision Detection

## Objective

Implement RFC 4271 §6.8 BGP connection collision detection — when two parallel connections exist between the same BGP speakers, resolve which to keep using BGP Identifier comparison.

## Decisions

- Detection at peer/reactor level (Option A) rather than session or FSM level — reactor knows all peer states and can compare connections before handing off to sessions; matches ExaBGP's approach.
- Two-phase detection: Phase 1 rejects new connections immediately if peer is Established; Phase 2 waits for the OPEN message on the new connection to learn the remote BGP Identifier, then compares.
- Comparison as unsigned 32-bit integer (host byte order) per RFC 4271 §6.8.
- OpenSent collision (RFC says MAY examine) not implemented — only the MUST case (OpenConfirm) was implemented to keep it simple.
- Multiple incoming connections before OPEN: accept first, reject subsequent with NOTIFICATION 6/7.

## Patterns

- RFC algorithm: if `local_id < remote_id` → close existing (outgoing), accept incoming. If `local_id >= remote_id` → close incoming, keep existing.
- NOTIFICATION sent with Error Code 6 (Cease), Subcode 7 (Connection Collision Detection) — already defined in `notification.go`.

## Gotchas

- Current `session.Accept()` simply rejected if `conn != nil` — this is not RFC 4271 §6.8 compliant; it silently drops connections rather than applying the BGP ID comparison.
- Collision detection requires knowing the remote BGP ID from the OPEN message — detection cannot happen at TCP connection time, only after OPEN is received.

## Files

- `internal/reactor/reactor.go` — handleConnection() collision check
- `internal/reactor/session.go` — DetectCollision(), handleOpen() collision resolution
- `internal/reactor/peer.go` — pending incoming connection tracking
- `internal/reactor/collision_test.go` — 6 collision scenario tests
