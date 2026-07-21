# 1188 -- fixit-rfc7606-treat-as-withdraw

## Context

RFC 7606 Section 2 treat-as-withdraw must remove the previously-installed route,
not merely decline to install the malformed one. `message.SynthesizeWithdraw`
(`internal/component/bgp/message/rfc7606_withdraw.go`) rewrites a malformed UPDATE's
announced routes into withdrawals; the reactor calls it on the
`RFC7606ActionTreatAsWithdraw` branch and falls through to normal dispatch. An
earlier park closed the RIB-boundary gaps (AC-1 Loc-RIB propagation, AC-5, AC-6) and
fixed a latent ADD-PATH bug, but left AC-8 (two-family) and AC-9 (non-negotiated
family) unmet. A follow-up (2026-07-21) implemented both, completing AC-1..AC-9.

## Decisions

- Proved AC-1/AC-5 at the RIB boundary by feeding the REAL `message.SynthesizeWithdraw`
  output through `RIBManager.handleReceivedStructured` (the DirectBridge path the reactor
  actually uses), then asserting Adj-RIB-In removal AND the best-change Withdraw published
  on the EventBus. The reactor test only proves a withdraw-shaped message was dispatched.
- Fixed the CODE rather than the test for AC-6: `handleReceivedStructured` never called
  `peerRIB.SetAddPath(fam, true)`, so the structured receive path created non-ADD-PATH
  FamilyRIBs and collapsed sibling paths. Added `SetAddPath` in the IPv4-NLRI and MP_REACH
  announce blocks BEFORE the first insert, mirroring the JSON path.
- **AC-8 (two-family), per D-3/D-8: one withdraw body PER MP family, not one merged body.**
  `SynthesizeWithdrawFamilies` (`rfc7606_withdraw.go:78`) emits a primary body (legacy IPv4
  field + first MP family) plus one further body per additional family. The RIB reads only
  the FIRST MP_UNREACH via `attribute.AttributesWire.GetRaw` (first-match), so a merged
  two-MP_UNREACH body silently withdraws only one family.
- **AC-8 forwarding: the extra bodies need a REAL cache entry, NOT an empty BufHandle.**
  D-8 originally dispatched `bodies[1:]` with an EMPTY `BufHandle{}`, on the premise that
  RIB-driven `publishBestChanges` would cover route-server clients. That premise is WRONG:
  RS clients are fed by TRANSPARENT forwarding through the `recentUpdates` cache entry (fast
  path `reactor_notify.go:550`, or the rs plugin's `ForwardCached`), not by the RIB. An empty
  BufHandle fails the cache gate (`reactor_notify.go:492` needs `buf.Buf != nil`), so the
  second family was never forwarded to RS clients AND `ForwardUpdatesDirect` logged a false,
  attacker-triggerable `"BUG: msgID missing from cache"`. Fix (`session_read.go:212`): dispatch
  the extra body with `BufHandle{ID: noPoolBufID, Buf: extra}`. `noPoolBufID` (`bufmux.go:45`,
  `^uint32(0)`) is the existing sentinel for a non-nil Buf NOT backed by a pool slot, so
  `ReturnReadBuffer` (`session.go:120`) no-ops on return (no double-free, no pool slot
  consumed), and `extra` is a fresh heap slice (`buildWithdrawBody` make+copy) that never
  aliases the pooled read buffer. The second body now caches and forwards to RS clients
  exactly like the primary. Chosen over acquiring a real pool buffer (pool-ownership
  complexity for a cold malformed-UPDATE path, spec D-7).
- **AC-9 (non-negotiated), per D-5: negotiation-aware synthesis + early drop.**
  `mpFamilyDispatchable` (`session_validation.go:355`) mirrors `validateUpdateFamilies`'
  accept condition exactly (skip == teardown condition, no false-accept/false-reject);
  `mpUnreachAttrList`/`mpFamilyAccepted` (`rfc7606_withdraw.go`) skip any non-negotiated MP
  family. If synthesis is empty, `processMessage` (`session_read.go:180-188`) restarts the
  HoldTimer and returns BEFORE `validateUpdateFamilies`, so a non-negotiated family stays a
  silent drop -- no NOTIFICATION, no teardown.
- **Synthesis moved OUT of `enforceRFC7606` into `processMessage`.** `enforceRFC7606`
  (`session_validation.go:185`) now only classifies/logs; synthesis is negotiation-aware and
  multi-body, neither of which fits a single-`WireUpdate` return. AC-1..AC-7 enforce-path
  tests assert only action/error/log, so the move is transparent to them.

## Consequences

- ADD-PATH received routes now key by (prefix, path-id) on the structured path. A correctness
  fix for all ADD-PATH DirectBridge sessions.
- A two-family treat-as-withdraw now withdraws BOTH families locally AND forwards both to RS
  clients; a treat-as-withdraw whose MP_REACH family was never negotiated drops for that family
  with no teardown.
- `noPoolBufID` is the idiom for "cache/forward a synthesized body the reactor built itself"
  (not from the read pool): a non-nil heap Buf whose return is a no-op. Reusable for any future
  reactor-synthesized UPDATE that must reach RS clients.

## Gotchas

- `attribute.AttributesWire.GetRaw` (`internal/core/bgp/attribute/wire.go:181`) returns the
  FIRST matching attribute only. Any code that must act on EVERY MP_UNREACH (two-family
  withdraw) must split into one body per family, not rely on a merged body.
- RS clients are TRANSPARENT-forward-fed via the `recentUpdates` cache, NOT RIB-fed. A reactor
  path that reaches the RIB (deliverChan) but skips the cache silently blackholes RS clients
  and trips a false "BUG: msgID missing from cache" ERROR. Any reactor-synthesized body meant
  for RS clients must carry a non-nil BufHandle (use `noPoolBufID` for heap bodies).
- Synthesis must be negotiation-aware BEFORE `validateUpdateFamilies` runs, else a synthesized
  withdraw for a non-negotiated family converts today's silent drop into strict-mode teardown.
- The `rib` plugin's add-path keying is set per family by `SetAddPath` BEFORE the first insert;
  setting it after the FamilyRIB exists does not retroactively convert it.
- Do NOT add an `RFC requirement:` tag to a test that only pins internal mechanics (the
  two-family split test pins Ze's split, not a new RFC obligation) -- an over-eager tag makes
  the requirement's audit verdict stale.

## Files

- internal/component/bgp/message/rfc7606_withdraw.go (`SynthesizeWithdrawFamilies` per-family split)
- internal/component/bgp/reactor/session_read.go (dispatch bodies[1:] with noPoolBufID cache-eligible handle; empty-synthesis drop before validateUpdateFamilies)
- internal/component/bgp/reactor/session_validation.go (`mpFamilyDispatchable`; synthesis moved to processMessage; enforceRFC7606 classify-only)
- internal/component/bgp/reactor/bufmux.go, session.go (noPoolBufID / ReturnReadBuffer doc comments)
- internal/component/bgp/message/rfc7606_withdraw_families_test.go, internal/component/bgp/reactor/session_rfc7606_families_test.go (NEW: split, non-negotiated skip, two-family forward-cache)
- internal/component/bgp/plugins/rib/rib_structured.go, rib_structured_test.go (AC-1/AC-5/AC-6, earlier park)
