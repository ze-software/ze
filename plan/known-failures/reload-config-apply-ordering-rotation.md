### `ze-test bgp reload` 21 test-config-apply-ordering-rotation -- load-sensitive, not reproducible

Observed once, 2026-07-25 on darwin (this host), during a full `make ze-verify`:

```
1.5s     21/38  FAIL  21  test-config-apply-ordering-rotation
    1 ✗ expect peer-exchange -> mismatch
```

The same mechanism as the dest-peer-teardown cluster (resolved 2026-07-25, see
`RESOLVED.md`): a
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
first things to try -- see `RESOLVED.md`, which archives that cluster.

## Update 2026-07-25: the mapped-batch path was hardened, the mismatch is STILL unexplained

Still not reproduced. What changed is the diagnosability of the path this test
uses -- it is one of the `conn_map` tests, so its connections are accepted as a
BATCH, OPEN-handshaked concurrently, then sorted and replayed in sorted order
(`internal/test/peer/peer_connmap.go`).

Two fail-open defects were fixed there, both found by reading, neither
reproduced:

1. **A stuck handshake wedged the whole batch with no diagnosis.**
   `doOpenHandshake` reads the OPEN via `ReadMessage`
   (`internal/test/peer/peer.go:348`), which sets no deadline, and the
   cancellation path in `runConnMap` closed only the LISTENER -- so a connection
   that was accepted but never sent an OPEN blocked the batch until the runner's
   outer timeout, which names neither the stuck connection nor the handshake as
   the thing being waited on. Accepted sockets are now registered with a watcher
   that closes them on cancel. Mutation-verified: without the watcher the batch
   wedges for the full 10s and the new test reports exactly that.
2. **A batch handshake failure did not say WHICH connection failed.** With a
   batch of N, `errOnce` recorded a bare `read OPEN: ...`. It now names the batch
   slot and the remote address.

**This does not explain the recorded failure and is not claimed to.** The symptom
here was `expect peer-exchange -> mismatch` -- a connection that WAS mapped and
delivered the wrong bytes -- not a hang. A mismatch of that shape means the sorted
batch did not line up with the `conn=N` expectations, and the untested hypothesis
worth carrying forward is that a batch picked up a connection from the previous
generation (ze retrying a dial across the SIGHUP boundary), which would shift
every subsequent slot. Nothing was measured to support that; do not repeat it as
a finding.

Capture on the next occurrence: the peer's own `conn=N remote-ip=... router-id=...`
lines (`printConnBatch`) from the saved `peer-stdout.log`, which state the mapping
the batch actually used, and compare them against the six router-ids the rotation
expects.
