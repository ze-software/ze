# bgp plugin 398 role-otc-export-unknown -- rare spurious withdraw under suite parallelism

**Status:** non-deterministic, actively hunted, NOT reproduced.
**First seen:** 2026-07-25, one occurrence in a full `ze-test bgp plugin --all` run (495 tests,
20-way parallel).

## Symptom

The destination peer receives a WITHDRAW of `10.0.0.0/24` where the test expects the announce:

```
Expected: UPDATE (len=47)  ORIGIN: IGP  AS_PATH: [65000]  NEXT_HOP: 1.1.1.1  NLRI: 10.0.0.0/24
Received: UPDATE (len=27)  WITHDRAWN: 10.0.0.0/24
```

Raw received frame: `FFFF...FFFF:001B:02:0004180A00000000`.

## Why this is recorded rather than fixed

Recording is not fixing, and a deterministic failure gets no shard
(`ai/rules/no-parking.md`). This qualifies for the one narrow exception in
`ai/rules/anti-rationalization.md` -- a non-deterministic failure actively hunted and not
reproduced:

| Attempt | Result |
|---------|--------|
| `ze-test bgp plugin --pattern role-otc-export-unknown`, 3 consecutive runs | pass, pass, pass |
| `scripts/dev/stress-repro.py bgp --test "plugin 398" --iterations 10 --minutes 6 --any-failure` (32 burners / 16 cores) | "not reproduced in 10 invocation(s) under load" (`tmp/stress-repro/bgp-plugin-398-20260725-004244.log`) |

## What is and is not ruled out

**Not ruled out: the relay rail runs in this test.** 398 loads `ze.bgp-adj-rib-in` and does
NOT load `bgp-rs` (`test/plugin/role-otc-export-unknown.ci:145`), so `replayOwned` stays
false (`adj_rib_in/rib.go:579`) and peer-up self-replay goes through `RelayStoredRoute` by
construction. An earlier revision of this file argued the relay was uninvolved because the
495-test run contained zero `relay-stored-route` log lines; a third review pass (2026-07-25)
corrected that. A SUCCESSFUL relay logs nothing at all and a suppressed one logs only at
Debug (`reactor_api_relay.go:268`), so the absence of those lines rules out relay ERRORS,
not relay involvement.

What does hold:

- The four tests that carried the egress-rail bug (372, 378, 394, 395) are green and
  non-reproducing under the same stress reproducer.
- 398 was passing before that spec's changes and passes in isolation after them.

The spurious-withdraw SHAPE resembles the original 394 capture, so the next occurrence is
worth capturing rather than dismissing.

## Next step for whoever sees it again

Capture the full `tmp/stress-repro/` log and identify the withdraw's producer.

**Ruled OUT as the producer, so do not start there:** the RFC 9494 announce-to-withdrawal
conversion in `forwardUpdateCore`. `ModAccumulator.SetWithdraw` has exactly one producer,
`bgp/plugins/gr/gr_egress.go:109`, and 398 loads neither `bgp-gr` nor any other plugin that
reaches it, so `mods.IsWithdraw()` (`reactor_api_forward.go:607`) cannot be true in this
test. An earlier revision named this path as the leading candidate.

Unexamined candidates: the peer-down withdrawal path, and whatever else can emit a
withdraw-only body to a destination with only `bgp-role` and `bgp-adj-rib-in` loaded. Read
the producing function before attributing it to anything, including this spec's change.
