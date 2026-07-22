### `ze-test bgp plugin` dest-peer-teardown cluster (85, 97, 222, 398) -- load-sensitive, pre-existing

Observed 2026-07-22 on darwin (this host). Running the plugin suite as a whole
(`bin/ze-test bgp plugin --all`, 493 active tests, parallel) fails a small,
shifting subset that every member of passes when run alone. The shared symptom is
`failed: connection closed before completion`: a test peer closes after the OPEN
exchange, so the routes never reach adj-rib-in and the downstream assertion
(`expect peer-exchange`, or a `expect=stderr` log pattern) never fires.

Members seen: 85 `bgp-rs-asn4-transcode`, 97 `bmp-locrib`,
222 `forward-congestion-teardown-metrics`, 398 `role-otc-unicast-scope`. This is
the same mechanism already recorded for 224 `forward-overflow-two-tier` and 458
`show-l2tp-tunnel-detail`, which have their own shards because they reproduce
deterministically rather than only under suite load.

PRE-EXISTING, NOT caused by `spec-feature-gate-10-bgp`. Attribution is a
like-for-like A/B against a clean HEAD baseline, not an inference:

- Baseline built from `git archive HEAD` (SHA `757663a3f`) into `tmp/headbase`,
  `ze-test` built by that tree's own Makefile, so it carries no part of the
  ze_bgp gate.
- Full plugin suite, baseline: `fail 489/493 ... failed 4 [97, 224, 398, 458]`.
- Full plugin suite, working tree: `fail 487/493 ... failed 6 [85, 97, 222, 224, 398, 458]`.
- Isolated pair `222 458`, three runs each: baseline `0/2` three times; working
  tree `0/2`, `0/2`, `1/2`. The working tree is not worse than the baseline on
  the same invocation, and the baseline reproduces the cluster on its own.
- Every member passes when run individually on the working tree
  (`ze-test bgp plugin 97 398` -> both PASS; `85` -> PASS).

The membership shifts run to run on BOTH trees, which is what makes it a
load-sensitive cluster rather than a per-test regression.

Environmental scope UNVERIFIED and NO root cause asserted. The dest-peer close
was not traced; do not read "connection closed before completion" as a cause --
it is the first observable symptom, and the reason a test peer tears down early
under suite parallelism is exactly what is unknown. Linux/CI status not checked.

Triage: use `scripts/dev/stress-repro.py plugin` per `ai/rules/flaky-under-load.md`
rather than looping `make ze-verify`, and check whether the teardown is the same
mechanism as shard `bgp-plugin-forward-overflow-two-tier.md` (which documents a
deterministic instance of the same symptom with `ZE_FWD_CHAN_SIZE=2`). If one
root cause explains all six, collapse these three shards into a single spec.
