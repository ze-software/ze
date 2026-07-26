### `ze-test bgp plugin` -- a rotating handful of tests fail under `-p 20` on a loaded host

Observed 2026-07-26 on darwin, in two consecutive `make ze-verify` runs on a
BYTE-IDENTICAL tree, failing a DIFFERENT set each time:

```
run 12: fail 493/495  failed 2 [222 forward-congestion-teardown-metrics, 228 gr-cli-restart]
run 13: fail 494/495  failed 1 [514 teardown-tunnel-all]
```

The four `make ze-verify` runs immediately before these were 495/495 on the same
code path. The difference is host load: those ran at load average ~2, these at
~11 (the machine picked up its normal desktop workload -- browsers, a VM). The
runner's own contended-run detector did not trip, so its threshold sits above
this level.

## Why this is logged rather than fixed

`ai/rules/no-parking.md` allows a shard for a non-deterministic failure actively
hunted and not caught, and that is this:

- **All three pass in isolation**, via the make target's own invocation (isolated
  bare-named binaries with the `zetest` tag, `ZE_BIN`/`ZE_TEST_BIN` exported --
  see `ai/rules/bash-output.md`, a directly launched runner is not equivalent):
  `pass 3/3 100.0% 2.8s`.
- **The failing set rotates.** Two runs, same tree, disjoint failures. A
  deterministic defect does not move.
- **Green four times running** on a quiet host earlier the same day.

## Attribution

NOT the QEMU-rot sweep in progress when this was observed. The only uncommitted
change at the time was to `//go:build integration && linux` files
(`cs6_integration_linux_test.go`, `integration_linux_test.go`), which the
functional suite does not compile at all.

A related row already exists in `plan/deferrals/` naming seven other
`test/plugin` tests as load-sensitive under the default `-p 20`
(`config-edit-ssh`, `ipv6`, `show-l2tp-*`, `sysctl-*`). These three are further
members of the same cluster, so the cluster is larger than that row records.

## Next step for whoever picks this up

Reproduce under controlled load rather than waiting for it to recur:

```
scripts/dev/stress-repro.py "bgp plugin" --test 222 --any-failure --minutes 6
```

The question to answer is whether these tests share ONE mechanism -- three of the
four names so far are teardown/restart shaped (`forward-congestion-teardown-metrics`,
`gr-cli-restart`, `teardown-tunnel-all`), which suggests a shutdown-ordering
assumption that only holds when the daemon is scheduled promptly, rather than
three independent flakes. Read the producer of each assertion before assuming
either.
