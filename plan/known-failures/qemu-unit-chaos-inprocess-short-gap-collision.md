### `TestInProcessDisconnectReconnect/short_gap_collision` -- one QEMU run only, not reproducible

Observed once, 2026-07-26, in the QEMU unit phase (`make ze-qemu-needs-linux-test`,
pass 9 of this sweep):

```
--- FAIL: TestInProcessDisconnectReconnect (14.53s)
    --- FAIL: TestInProcessDisconnectReconnect/short_gap_collision (5.13s)
        runner_test.go:341: "2" is not less than or equal to "1"
            expected at most 1 established events
```

The subtest reconnects a peer after a short gap and asserts the BGP
connection-collision resolution yields at most one `established` event. Under the
VM it saw two.

## Why this is logged rather than fixed

`ai/rules/no-parking.md` allows a shard for exactly one case -- a
non-deterministic failure actively hunted and not caught -- and this is that case:

- **Not reproducible on the host.** `go test -count=5 -tags "ze_core $ZE_FEATURES"
  -run TestInProcessDisconnectReconnect ./internal/chaos/inprocess/` -> 5/5 pass,
  52.3s. (The first attempt was run WITHOUT the feature tags and failed 5/5 with
  `no such module: ze-bgp-conf`, a phantom red -- see
  `ai/rules/bash-output.md`. That attempt proves nothing and is recorded here so
  nobody repeats it.)
- **Did not occur in the two adjacent VM passes.** The same package was green in
  QEMU pass 8 and pass 7 of this sweep, on trees differing only in unrelated
  files.
- **Green in every `make ze-verify`** across this sweep (four full runs).

## What is NOT the cause

The QEMU unit phase had never executed before this sweep repaired it (its GOCACHE
pointed through a host-only symlink), so this is the FIRST time this package ran
in the VM at all. "New failure" here means newly VISIBLE, not newly introduced --
nothing in this sweep touches `internal/chaos/inprocess`, BGP session
establishment, or collision resolution.

## Next step for whoever picks this up

Reproduce it under load before theorising: the collision window is timing-bound,
and the VM is slower and differently scheduled than the host.

```
scripts/dev/stress-repro.py --help   # host-side load reproduction
make ze-qemu-debug NOBUILD=1 RUN='sh -c "cd /workspace && go test -count=20 \
  -tags \"ze_core $(awk \"\\$1 ~ /^ze_/ {print \\$1}\" feature-gates.txt | sort -u | tr \"\\n\" \" \")\" \
  -run TestInProcessDisconnectReconnect ./internal/chaos/inprocess/"'
```

If it reproduces, the question to answer is whether TWO established events is a
real protocol defect (collision resolution letting both connections reach
Established, RFC 4271 Section 6.8) or a test that counts events across a
reconnect boundary it did not intend to span. Read the producer of the event
count before assuming either.
