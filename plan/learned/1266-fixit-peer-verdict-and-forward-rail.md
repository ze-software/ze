# 1266 -- fixit-peer-verdict-and-forward-rail

## Context

Diagnosing the four remaining `bgp plugin` functional-test failures (91, 224, 398,
458) surfaced ten defects across the test harness, the BGP forwarding rail, and a
set of fixtures. Only some were product bugs; the rest were harness/fixture defects
masking them. The spec's premise was that a fail-open multi-peer verdict made
deterministic failures read as "flaky 1-in-10". The goal was to make every peer
govern its own verdict, repair the fixtures, and fix the real forwarding defects so
the tests both pass and actually gate.

## Decisions

- **The multi-peer verdict must require EVERY check-mode peer to succeed**
  (`failedCheckPeers`), over keeping any-peer semantics with a warning. A warning
  does not fail a test; `fail-closed-guards` requires the guard to deny or speak.
- **Repair malformed fixture bytes rather than relax the assertions they feed**, and
  keep declared lengths unchanged so the intended frame is recovered, not redefined.
  A repo-wide Go gate (`TestCIFrameLengthsWellFormed`) catches malformed frames the
  moment they land, not only when the owning suite runs.
- **De-specify EOR ordering in the tests, not in the product**: RFC 4724 orders the
  EOR only against this speaker's own initial dump, so a later-learned forward
  legitimately precedes it. The racy receiver-EOR was dropped (measured 4/6→0/6);
  the deterministic source-EOR was kept behind an `eor-sent` teardown gate.
- **Forward-pool FIFO is enforced with a per-worker `overflowPending` counter**
  spanning the drain-snapshot window, over trying to fix ordering at the call sites
  (the drain-window race is unfixable from outside; the pool owns the invariant). A
  nil-peer wake-up sentinel closes the starvation wedge the gate opened.
- **Reframe per-message NLRI framing assertions to an `ordered=` FIFO needle
  subqueue**, because the forward rail legally packs same-attr NLRIs into one UPDATE
  (`fwdBucketMerge`); per-message framing is not a property ze owes, but order and
  completeness are.
- **A check peer that finishes its script must `option=linger`**, and a load
  observer must drain forwarding (`request peer * flush`, a BarrierPeer) before
  `request shutdown` -- otherwise it withdraws routes / truncates delivery mid-burst.

## Consequences

- New reusable test infrastructure: the `ordered=` peer-checker rule and the
  `option=linger` peer directive (documented in `docs/architecture/testing/ci-format.md`).
  Any forward-path `.ci` that asserts multi-route delivery should use them.
- The forward pool now guarantees per-destination FIFO across the channel/overflow
  boundary; `fwdBatchHandler` selects its peer from the first non-nil item so a
  wake-up sentinel can never suppress a real write.
- AC-4 was **re-scoped**: defect 3 (intermittent un-prepended AS_PATH) was not a
  code defect. The prepend gate is statically correct at all three rails and nil
  facts skip a peer rather than emit un-prepended. The original 2/8 flake was
  incidentally closed by the reactor concurrency-race fixes `99ff5e85f` +
  `f9146a35c` (hypothesis, not bisected). The spec's Known Limitation pre-authorized
  this re-scope; the owner approved it 2026-07-24.

## Gotchas

- **The spec outran its own bookkeeping.** By closure (2026-07-24) five of the six
  original defects had already been fixed by sibling sessions/commits since the
  2026-07-22 draft, while metadata still said Phase 1/5. Assess a stale multi-session
  spec against the CODE before assuming its defects are open -- three read-only
  investigation agents found there was almost nothing left to implement.
- **A stale claim can hide inside the spec itself.** The spec asserted the RFC 4271
  §5.1.2 prepend obligation was "unextracted"; it is present at `rfc/short/rfc4271.md`
  (`[RFC4271-5.1.2-3]`). Verify spec-stated gaps, not just spec-stated fixes.
- The `overflowPending` gate can transiently go negative during a Stop/drain race;
  harmless because TryDispatch short-circuits on `fp.stopped` before reading it.
- Cosmetic residue: the `# ORDERING:` comment in `test/plugin/bgp-rs-reactor-fastpath.ci`
  is self-contradicted by the `test-relax` block below it (the assertions are
  correct). `plan/known-failures/bgp-plugin-show-l2tp-tunnel-detail.md` (test 458)
  is a separate test's shard, not resolved by this spec.

## Files

- `internal/component/bgp/reactor/forward_pool.go`, `forward_bucket.go` (FIFO gate, wake-up sentinel, merged-item peer pick)
- `internal/test/peer/checker.go`, `peer.go`, `expect.go`, `internal/test/cli/cmd_peer.go` (`ordered=`, `option=linger`)
- `internal/test/runner/peer_contract.go`, `ci_fixture_test.go` (per-peer verdict, frame gate)
- `internal/component/bgp/reactor/reactor_notify.go`, `forward_rs.go`, `plugins/rs/server_withdrawal.go` (fail-open seams)
- `test/plugin/forward-overflow-two-tier.ci`, `forward-two-tier-under-load.ci`, `bgp-rs-reactor-fastpath{,-fallback}.ci`
- Commits: `aaefef8ce`, `16601c4c5`, `cf31a5862`, `f9146a35c`, `99ff5e85f`, `928e28b10`, `3abcbdca1`, `be870af9a`
