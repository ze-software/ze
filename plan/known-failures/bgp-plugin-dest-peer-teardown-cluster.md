### `ze-test bgp plugin` dest-peer-teardown cluster (85, 97, 222, 398) -- load-sensitive, pre-existing

Observed 2026-07-22 on darwin (this host). Running the plugin suite as a whole
(`bin/ze-test bgp plugin --all`, 493 active tests, parallel) fails a small,
shifting subset that every member of passes when run alone. The shared symptom is
`failed: connection closed before completion`: a test peer closes after the OPEN
exchange, so the routes never reach adj-rib-in and the downstream assertion
(`expect peer-exchange`, or a `expect=stderr` log pattern) never fires.

Members seen: 85 `bgp-rs-asn4-transcode`, 97 `bmp-locrib`,
222 `forward-congestion-teardown-metrics`, 398 `role-otc-unicast-scope`. This is
the same mechanism recorded for 458 `show-l2tp-tunnel-detail`, which has its own
shard because it reproduces deterministically rather than only under suite load.
(224 `forward-overflow-two-tier`, which had its own shard for the same reason,
was root-caused and fixed on 2026-07-24 -- see `RESOLVED.md` and
`plan/spec-fixit-peer-verdict-and-forward-rail.md`. Its fixes, especially the
source-peer `option=linger` and the observer's `request peer * flush` before
shutdown, are the most likely levers for clearing this cluster too.)

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


---

## Update 2026-07-25: mechanism confirmed, two of four members fixed

**222 `forward-congestion-teardown-metrics` -- FIXED (`52ad2f71b`).** Reproduced on
an IDLE host, so this member was never really load-only. The observer's readiness
predicate (`show metrics values` non-empty) is satisfied at plugin-ready time,
which is BEFORE peers start -- the test's own comment admits the counters are
"always registered, value 0 at startup". Measured: `bgp peers started` at
18:41:33.386, observer `OK` at .388, and no `session established` line at all. So
`request shutdown` could close the session mid-OPEN, which is the shard's
`connection closed before completion`. Fixed with the established+`quiesce()`
barrier already used by `test/plugin/api-rib-clear-out.ci:36-52`, whose comment
names this exact failure. A/B on an idle host: baseline 1 failure in 8, with the
barrier 0 in 8, and with the barrier the daemon reaches `session established` and
`sent EOR` before shutdown.

**398 `role-otc-unicast-scope` -- FIXED (`52ad2f71b`).** Same family, different
lever: its source check peer closed ~1 ms after sending its UPDATE, because a check
peer without `option=linger` exits as soon as its script completes. See the
`bgp-plugin-role-otc-export-unknown` entry in `RESOLVED.md` for the full producer
chain.

**85 `bgp-rs-asn4-transcode` and 97 `bmp-locrib` -- NOT addressed.** They were not
individually reproduced this session. The full `plugin` suite has since run
495/495 twice, which is data and not proof for a load-sensitive cluster. If either
recurs, check the same two levers first -- an observer whose readiness predicate is
satisfiable before peers start, and a source peer without `option=linger` -- before
looking for anything new.

Correction to this shard's own triage note: it proposed `option=linger` as the
likely lever for the whole cluster, citing the 224 fix. That was right for 398 and
WRONG for 222, whose source peers need no linger; there the defect was the observer
shutdown gate. Do not assume one lever fits the cluster.
