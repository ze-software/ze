# Learned: fixit-ipsec-clear-reestablish

Spec: `plan/spec-fixit-ipsec-clear-reestablish.md` (make `clear vpn ipsec sa`
re-establish a site-to-site tunnel against a live responder in seconds, not the
~150s DPD fallback, without any unauthenticated remote teardown primitive).

NOTE (numbering): chose 1215 to dodge heavy contention around 1198-1205; run
`python3 scripts/dev/learned_numbers.py --fix` at drain to reconcile with `.counter`.

## What the change does
Two coupled halves in the IKEv2 engine (`internal/component/ike/engine/`):

**Phase A -- graceful Delete on operator clear.** `TerminateAllSAs` /
`TerminatePeerSA` (`register.go`) now call `ps.StopGraceful()` (new) instead of a
bare `Stop()`; `StopGraceful` sets `ps.graceful` then `Stop()`. The owner loop's
stopCh case (`maintainSA`, `established.go`) reads `graceful` and sends an
authenticated INFORMATIONAL Delete (`sendDeleteIKE`, RFC 7296 Section 1.4) BEFORE
`cleanupChild` -- built on the owner goroutine, the only goroutine that owns
`sa.NextMsgID` / `sa.SKKeys` (A-5). The peer tears down at once instead of waiting
for its ~150s DPD timeout.

**Phase B -- responder accepts a parallel re-init and supersedes on
authentication (RFC 7296 Section 2.4).** Three previously-coupled single-SA-per-peer
constraints were split:
- `responderBusy` now gates ONE *half-open* handshake (its documented meaning) and
  is cleared at adoption/establishment, not held across the SA's established life
  (that hold was the root-cause drop). A fresh IKE_SA_INIT arriving while a tunnel
  is up passes the CAS and is accepted into a second slot (`pendingSA`).
- Routing keys on SA identity: `PeerSession.established atomic.Bool` was replaced by
  `ownedSA atomic.Pointer[SA]` (the exact SA `maintainSA` owns). `routeInbound`
  delivers to the owner loop iff `ps.ownedSA.Load()==sa`, so the parallel half-open
  SA's packets are handled inline on the dispatch goroutine, not decrypted under the
  established SA's keys. `runEstablished` updates `ownedSA` on the IKE-rekey swap too.
- A second explicit Child SA slot (`pendingChild`): the new child is staged there
  until promotion so the old owner loop's `cleanupChild` removes only its own child
  (make-before-break, R-2).

Supersede is triggered ONLY on the authenticated IKE_AUTH: `finishResponderEstablish`
(`responder.go`), when `ownedSA!=nil`, stages `pendingChild` and signals a buffered
`ps.supersede` channel. `maintainSA` returns nil on `<-ps.supersede` after removing
only the old child; `runResponder` (`fsm.go`) then tears the old SA down and promotes
`pendingSA`/`pendingChild`. INITIAL_CONTACT is emitted unconditionally on every first
IKE_AUTH request (`buildAuthRequest`, `auth.go`; ze one-SA-per-peer model) and parsed
on receipt (`sa.InitialContact`, `handleAuthRequest`).

## Non-obvious findings / traps
- **The skeleton's one "ruled out" line was the answer.** The original spec author
  wrote off `responderBusy` (`fsm.go` line ~185) as "reset on establishment", so it
  is NOT the drop cause -- by reading the call site's log string, not the function.
  It is the FIRST statement of `runResponder`, run once per cycle before any handshake,
  and the gate is freed only after `runEstablished` RETURNS (teardown). The gate WAS
  the drop cause. Textbook `ai/rules/no-fabrication.md`: read the producer, not the caller.
- **The RFC forbids the obvious fix.** "Responder sees a new init, drops the old SA,
  accepts" is an unauthenticated remote teardown primitive (RFC 7296 Section 2.4:
  "MUST NOT conclude ... failed based on ... messages that arrive without cryptographic
  protection"). The legal shape is coexist-then-supersede-on-authentication, which is
  why supersede is wired ONLY into `finishResponderEstablish` (post-`verifyRemoteAuth`),
  never into `tryResponderSAInit`. Grep the diff: no teardown is reachable from the
  IKE_SA_INIT path.
- **Freeing the gate alone is worse than the bug (two more couplings).** `routeInbound`
  keyed on peer name (not SA), so an accepted parallel init would be shoved into the
  OLD SA's owner loop and fail to decrypt; and `ps.setSA` was a single slot that would
  clobber the established SA. The one-line gate fix looks right, passes a naive unit
  test, and misroutes on the wire. Fixed all three (gate scope + `ownedSA` routing +
  `pendingSA`/`pendingChild` slots).
- **Race-safe supersede without reading pending.State cross-goroutine.** The
  single-owner model (spec-ipsec-13) bought its correctness by NOT reading `sa.State`
  across goroutines (that is why `established atomic.Bool` existed). The supersede uses
  a channel signal from the dispatch goroutine, and `reapStalePending` reads only the
  immutable `pending.CreatedAt` plus the invariant "an authenticated pending returns
  the owner loop via the supersede case before the reap timeout (30s) can fire" -- so it
  never reads the pending SA's mutable State. `go test -race ./internal/component/ike/...`
  is clean.
- **`ownedSA` also fixes a latent rekey-routing fragility.** The old peer-name+bool
  gate happened to survive an IKE-SA rekey only because the peer name is stable; keying
  on SA identity forced an explicit `ps.ownedSA.Store(out.newSA)` on the rekey swap.
- **`clear vpn ipsec sa` is now a *graceful* close, distinct from config-change Stop.**
  Kept `Stop()` unchanged (R-6): a routine config edit must not start emitting Deletes.
  Only `TerminateAllSAs`/`TerminatePeerSA` use `StopGraceful`; engine shutdown and
  `reconcilePeers` removal keep plain `Stop()`.

## Test / verification notes (parked session limits)
- Unit tests (all pass, whole `internal/component/ike/...` race-clean): AC-3 accept
  parallel `TestResponderAcceptsReinitAfterStaleSA`; AC-6/R-1 keep-old-on-unauth
  `TestResponderKeepsOldSAOnUnauthenticatedInit`; AC-3 supersede+INITIAL_CONTACT
  `TestResponderSupersedesOnAuthenticatedInit`; Phase A `TestClearSendsIKEDelete` +
  negative `TestPlainStopSendsNoDelete`; owner-loop half `TestMaintainSARelinquishesOnSupersede`;
  routing `TestRouteInboundKeysOnSANotPeer`; re-init identity `TestTerminateAllSAsReinitiates`;
  emit/honor `TestInitiatorEmitsInitialContact`. Review-fix regression locks:
  `TestReapStalePendingSkipsEstablished`, `TestCleanupPendingSA`,
  `TestResolvePendingCleansPromotedChildOnStop` (proves no Child SA leak on the
  operator-clear + parallel-auth race), `TestResolvePendingPromotesOnSupersede`.
- **The promote-or-cleanup decision is extracted** into
  `resolvePendingAfterOwnerLoop` (`fsm.go`) so the stop-vs-supersede guard is tested at
  its entry point (fail-closed-guards). On stop it frees the pending SA+child (no leak);
  otherwise it promotes. It deliberately does NOT guard promotion on
  `State==StateEstablished` -- that would orphan a half-open pending when the old SA dies
  via DPD mid-parallel-handshake.
- The parked session could NOT run functional (`test/ipsec`) or interop (strongSwan
  Docker) suites -- they kill live servers / need Docker. AC-1/AC-2/AC-5/AC-7 end-to-end
  validation is left for CI: `test/ipsec/ipsec-clear-reestablish{,-peer}.ci` and
  `test/ipsec-interop/scenarios/10-clear-reestablish/`, `11-responder-accepts-reinit/`
  (statically validated only: `.ci` structural conformance, `check.py` py_compile).
  The `.ci` gate on the negotiated aes-cbc/sha256 (not the false-matching `established-at`
  key) and bound re-establishment well under the ~150s DPD path; the `<=10s` bound and
  changed-SPI proof are the executor/QEMU refinement, not asserted in the `.ci` format.

## Follow-ups (out of scope, recorded)
- Q2 (peer-removal Delete): `reconcilePeers` removal keeps bare `Stop()`; a config-remove
  saying goodbye is a separate spec.
- A re-negotiation Prometheus counter would be a useful R-1 DoS signal (repeated inits)
  but is a new observable surface; deferred (learned 745's rekey_total gauge caveat).
- `close-action` (none/start/restart) is parsed but unconsumed in the engine; adjacent.
