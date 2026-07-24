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

## What it is NOT

It is not the egress-rail divergence that `spec-fixit-bgp-egress-rail-divergence` fixed, on
the evidence available:

- The whole 495-test run contains **zero** `relay-stored-route` log lines, so the relay path
  emitted no error anywhere.
- The four tests that DID carry that bug (372, 378, 394, 395) are green and non-reproducing
  under the same stress reproducer.
- 398 was passing before that spec's changes and passes in isolation after them.

The spurious-withdraw SHAPE resembles the original 394 capture, so the next occurrence is
worth capturing rather than dismissing.

## Next step for whoever sees it again

Capture the full `tmp/stress-repro/` log and identify the withdraw's producer. Candidate
starting points, none yet confirmed: the RFC 9494 announce-to-withdrawal conversion in
`forwardUpdateCore` (`mods.IsWithdraw()`, `internal/component/bgp/reactor/reactor_api_forward.go`),
and the peer-down withdrawal path. Do not assume it is the same defect as 394 without
reading the producing function.
