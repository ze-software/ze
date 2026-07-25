### `ze-test bgp reload` 21 test-config-apply-ordering-rotation -- load-sensitive, not reproducible

Observed once, 2026-07-25 on darwin (this host), during a full `make ze-verify`:

```
1.5s     21/38  FAIL  21  test-config-apply-ordering-rotation
    1 ✗ expect peer-exchange -> mismatch
```

The same mechanism as the `bgp-plugin-dest-peer-teardown-cluster` shard: a
`expect peer-exchange` assertion that only misses when the suite runs alongside
the rest of the verify workload.

## Why this is logged rather than fixed

It could not be made to happen again, and `ai/rules/anti-rationalization.md`
allows a shard only for exactly that case -- a non-deterministic failure actively
hunted and not caught:

- **Passes alone.** `make ze-reload-test` -> 25 PASS / 0 FAIL, test 21 green in 2.1s.
- **Passed in the immediately preceding full verify** with a byte-identical working
  tree (that run failed on an unrelated stage, `TestWorkflowMakeTargetsExist`), so
  the same code produced both outcomes.
- **Not reproduced under stress.** `scripts/dev/stress-repro.py "bgp reload"
  --test 21 --any-failure --minutes 6`: 80 invocations, 32 burner processes,
  8-way parallel on 16 CPUs -> `not reproduced in 80 invocation(s) under load`.
  Log: `tmp/stress-repro/bgp-reload-21-20260725-135958.log`.

## Attribution: NOT the session-scoped-build-artifacts work

That change (session-suffixed binaries, scratch under `tmp/s/<id>/`) was in the
tree for both verify runs, the passing one and the failing one, so it cannot be
the deterministic cause. It also contains no product code: the diff touches
`Makefile`, `mk/*.mk`, `internal/test/**` (test infrastructure), `.claude/hooks/`,
`scripts/`, and docs -- no `internal/component/`, no `internal/core/`, no
`internal/plugins/`, and no `.ci` file.

The test itself was last changed by `f41cd100a` ("switch reload tests to
conn_map=router-id, remove L2TP skip"), which is the more likely place for a new
load sensitivity to have entered.

## Next step for whoever picks this up

Re-run the reproducer with `--race` and higher `--burners`/`--parallel`; if it
still will not reproduce, instrument the peer-exchange wait rather than widening
timeouts. The levers that cleared the sibling cluster (source-peer
`option=linger`, an observer `request peer * flush` before shutdown) are the
first things to try -- see `plan/known-failures/bgp-plugin-dest-peer-teardown-cluster.md`.
