# bgp plugin role-otc-export-unknown -- rare spurious withdraw under suite parallelism

**Status: ROOT-CAUSED AND FIXED 2026-07-25.** Kept until a full suite run confirms
it; the owner can archive it into `RESOLVED.md` at that point.
**First seen:** 2026-07-25, one occurrence in a full `ze-test bgp plugin --all` run
(495 tests, 20-way parallel).

## Symptom

The destination peer receives a WITHDRAW of `10.0.0.0/24` where the test expects
the announce:

```
Expected: UPDATE (len=47)  ORIGIN: IGP  AS_PATH: [65000]  NEXT_HOP: 1.1.1.1  NLRI: 10.0.0.0/24
Received: UPDATE (len=27)  WITHDRAWN: 10.0.0.0/24
```

Raw received frame: `FFFF...FFFF:001B:02:0004180A00000000`.

## Root cause

**The source peer closes its session ~1 ms after sending the UPDATE, and `bgp-rs`
then correctly withdraws its routes to the destination peer.**

Producer chain, verified by reading the code and confirmed against the daemon's
own log:

1. `internal/test/peer/peer.go` `completed()` -- without `option=linger` a check
   peer **closes its connection the moment its own script completes**. This peer's
   script is finished as soon as it has sent its UPDATE and seen ze's EOR.
2. Daemon log, single run, all inside one millisecond at `44.239`:
   `OnMessageBatchReceived peer=127.0.0.1 event=update dir=received procs=5 msgs=1`
   immediately followed by
   `OnPeerStateChange peer=127.0.0.1 state=down reason="connection lost"`.
3. `bgp-rs` is loaded -- **auto-loaded by the `bgp` config path**, not by the
   `--plugin` flags (see the verification note in
   `plan/known-failures/bgp-plugin-rs-forward-duplicate-and-order.md`). Its
   `handleStateDown` (`rs/server_handlers.go:90-102`) hands the down peer's routes
   to `sendBatchedWithdrawals` (`:110-149`), which dispatches
   `update text nlri ipv4/unicast del prefix 10.0.0.0/24` to every peer except the
   one that went down. That produces a withdraw-only body of exactly
   `0004 180A0000 0000` -- byte-for-byte the captured frame.
4. The destination peer's rule therefore sees the WITHDRAW where the test expected
   the ANNOUNCE. Intermittent because it is a race between forwarding the announce
   and the teardown withdrawal that follows ~1 ms later.

Same mechanism and same lever as defect (9) of
`plan/spec-fixit-peer-verdict-and-forward-rail.md` (224 `forward-overflow-two-tier`,
resolved 2026-07-24): "the source check-peer closing its session the moment its
script completed, so ze correctly withdrew its routes mid-burst".

**Nothing in the daemon is wrong here.** Withdrawing a down peer's routes is
correct BGP; the test was asserting against a peer it had allowed to disconnect.

## Second defect found in the same test: the adj-rib-in assertion was vacuous

The observer polled `dispatch_until('show bgp adj-rib-in status', total-routes >= 1)`
and then gated on `if total < 0`. Two problems compounded:

- `adj_rib_in/rib.go:573-576` **deletes a peer's entire stored RIB on
  session-down** (`delete(r.ribIn, peerAddr)`). With the source peer closing 1 ms
  after its UPDATE, `total-routes` was 0 on **every** run: the poll burned its full
  5 s timeout every time.
- `total < 0` can only fire when the key is absent. A present-but-zero total fell
  straight through to the OK print and `request shutdown`, so the adj-rib-in half
  of this test asserted nothing at all and only the dest-peer wire assertion was
  load-bearing. A textbook fail-open guard (`ai/rules/fail-closed-guards.md`).

## Fix (2026-07-25)

- `option=linger:value=true` on the source peer in
  `test/plugin/role-otc-export-unknown.ci`, so the session stays up and both the
  spurious withdrawal and the RIB deletion stop happening.
- The gate tightened to `total < 1` so the assertion actually gates.
- The identical pair of defects was found and fixed in the sibling
  `test/plugin/role-otc-unicast-scope.ci` (also a member of the
  `bgp-plugin-dest-peer-teardown-cluster` shard).

Verification: 6/6 and 5/5 green respectively. **Mutation-verified**: removing
`option=linger` from either test turns it RED with
`ZE-OBSERVER-FAIL: adj-rib-in never stored the route, got total=0`, which proves
both that the tightened gate now gates and that the linger is what fixes it.

## What the earlier revisions of this file got wrong

- "398 loads `ze.bgp-adj-rib-in` and does NOT load `bgp-rs`, so `replayOwned`
  stays false and peer-up self-replay goes through `RelayStoredRoute` by
  construction." **False.** `bgp-rs` is auto-loaded by the `bgp` config path, and
  the daemon logs `peer-up replay ownership claimed by another plugin; self-replay
  disabled` in every run of this test. The relay rail is NOT the producer here.
- The `ModAccumulator.SetWithdraw` ruling-out was **correct** and stays ruled out:
  its only producer is `bgp/plugins/gr/gr_egress.go:109`, which `bgp-gr` does not
  reach in this test.
- "Unexamined candidate: the peer-down withdrawal path" was the **right** lead. It
  is the answer.
