# spec-bgp-rs-replacement-on-withdrawal: what remains after Phase 3 package d

Spec: `plan/immediate/spec-bgp-rs-replacement-on-withdrawal.md` (in-progress,
Handoff `verify`). Earlier state: `plan/handover/03-session-2026-10-18-linux-continue.md`,
sections "Item 8 continuation" and "RS replacement Phase 3 package (b)/(c)".

## Landed 2026-10-21 (package d, tests only)

`TestOwedKeyPurgeBetweenLookupAndAdmission` and `TestOwedKeyLookupChunked` in
`internal/component/bgp/reactor/rs_withdrawal_convergence_test.go`;
`TestRSOwnerWithdrawalSkipsDestinationOwnPath` and
`TestRSOwnerWithdrawalLookupFailureRetiresDestination` in
`rs_withdrawal_replacement_test.go`. Each has a recorded red (spec Design
Insights, 2026-10-21). The file holds the reusable seam `publishLookupTap`
(hook after bgp-rib's in-process lookup) and the RIB command helper
`dispatchRIBCommand`, which the remaining live tests can use.

## Phase 3 TDD rows with no test yet

Checked by `grep "func <name>("` over `internal/` on 2026-10-21. A row may be
covered under another name; read the row, then the named file, before writing.

| Test | Spec AC | Note |
|------|---------|------|
| `TestOwedKeyLivenessUnderChurn` | AC-15 | Design Insights "R-12 producer" names the liveness risk it must measure |
| `TestOwedKeyOwnerGoneAfterOweRetires` | AC-11 | needs `request bgp rib recovery` unregistered between owe and lookup |
| `TestOwedKeyStormKeepsSourceAndForwardWorkerMoving` | AC-16 | provider that blocks until released: `publishLookupTap` can block |
| `TestOwedKeyNoRecoveryOwnerWritesWithdrawal` | AC-10 last sentence | `TestOwnerWithdrawalWrittenWhenOwnerDoesNotReceiveSource` covers the unattached case only |
| `TestRSOwnerWithdrawalAddPathSourceKeepsOtherPath` | AC-9 | the live fixture negotiates no ADD-PATH yet |
| `TestRSDepartureWritesOnlyWhereSourceOwns` | round-2 B-1 | |
| `TestRSDepartureAfterReconnectKeepsNewerRoute` | AC-18 | |
| `TestRSOwnerWithdrawalLaterAnnouncementElected` | AC-6 | expectation changes with Phase 3a (Q-3) |
| `TestRSOwnerWithdrawalPolicyDeniedReplacement` | AC-5 | |
| `TestRSOwnerWithdrawalPolicyDeniedSkipsToNextSurvivor` | AC-5 | |
| `TestRSOwnerSynthesizedWithdrawalRecovers` | AC-14 | both rails, both synthesized shapes |
| `TestOwedKeyRealRIBConvergence` | AC-12 | real RIB, N=4, K=2 and K=N, every order |
| `TestOwedKeyRealRIBConvergenceProperty` | AC-12 | seeded random N in [3, 12] |
| `TestOwedKeyGRPurgeRecoversSurvivor` | AC-19 | real GR down with retention; `mark-stale`/`purge-stale` through `dispatchRIBCommand` is the lifecycle half |

Also owed by Phase 3: the AC-10 change of `TestRFC8654FatalLengthReelectsAlternateBest`
(owner-approved, re-record its discrimination), and the gates the spec names.

## Then, in the spec's order

Phase 3b (labelled withdrawal framing, its own commit), 3a (announcement
election, Q-3), 4 (bgp-rr), 5 (functional and RFC carriers), 6 (interop
`bgp-route-server-replacement-frr`), and the 2b second half once D1 of
`spec-bgp-graceful-restart-rfc-defects.md` lands. Each is about one agent's
budget.

## Facts found here

- bgp-rs sends a departure in batches of 500 (`withdrawalBatchSize`,
  `internal/component/bgp/plugins/rs/server_handlers.go`), and the reactor waits
  on each batch, so a departure never hands one recovery round more than 500
  keys. Only an owner-withdrawal burst exercises `recoveryLookupKeysMax`.
- With the chunk removed, a 1500-key admission fails `relay-stored-route: read
  buffer pool exhausted` and the worker resets sessions.
- Under `-race`, a departure of 2000 keys over the live fixture takes up to
  20 seconds.
