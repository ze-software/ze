# Procedure for the A-4 Loc-RIB insert measurement

This page gives the procedure for the measurement that
`plan/post-release/features/spec-route-policy-0-umbrella.md` still owes under
assumption A-4. It has not been run. Only the owner runs it: the time figures
count only from his Mac (`ai/META-DECISIONS.md`, "Test systems and labs"), and
the procedure exports past revisions into copies of the tree, which agents may
not do. It follows the procedure of `filter-wire-0-a6-procedure.md`.

## What it measures

The cost that the route policy gate adds to a Loc-RIB insert. The same three
benchmarks run at three revisions under one Go toolchain:

| Column | Revision | Why |
|--------|----------|-----|
| `base` | `d04aabe5e0019f83369767ebf0d111a1806d5798` | The parent of the gate commit |
| `impl` | `fb94aa04977da6f0cb31f08dd46a8f315af0ffc1` | The commit that added the gate |
| `head` | `c20d1efca738da63b923f0b633f24143c0d711c5` | `origin/main` when this page was written |

The benchmarks are `BenchmarkShardedInsertSerial`,
`BenchmarkShardedInsertParallel` and
`BenchmarkShardedManyWritersDistinctShards` in
`internal/core/rib/locrib/shard_bench_test.go`. The file is the same at all
three revisions, so no file is copied between them.

The `head` column exists because one later commit changed the insert path.
`dfa96ffa2e` added the hidden-path accounting (`entry.go` `upsert` calls
`accountStored`) and one more field store in `policy.go` `admit`. With no
policy chain configured, which is the benchmark's case, that is a few compares
for each insert and no allocation. The `base` and `impl` pair measures the gate
alone. The `head` column shows whether today's insert path still holds that
result.

## Choices that the commands carry

- **A fixed iteration count, `-test.benchtime 1000000x`.** Each benchmark
  inserts into a table that grows with the iteration count. With a time budget,
  a faster revision would build a larger table and measure a different
  workload.
- **Test binaries built once, then run in interleaved rounds.** Ten rounds of
  `base`, `impl`, `head` spread thermal drift across the three columns.
- **No build tags.** No file in the package, or in the packages it imports for
  this path, carries a feature tag.
- **One GOROOT for every column,** the patched one from `./le go stdpatch
  goroot`, as in the A-6 procedure.
- **Shards left at the default,** which is the processor count. `hw.ncpu` in
  the provenance records it.

## The steps

Run these from the root of a Ze checkout on the Mac. Choose `OUT`.

1. Set the toolchain and the directories.

   ```text
   git fetch origin
   export GOROOT="$(./le go stdpatch goroot)" GOTOOLCHAIN=local CGO_ENABLED=0
   export PATH="$GOROOT/bin:$PATH"
   OUT=<output-dir>; mkdir -p "$OUT"; W=$(mktemp -d)
   ```

2. Export each revision and build its test binary.

   ```text
   for p in base:d04aabe5e0019f83369767ebf0d111a1806d5798 \
            impl:fb94aa04977da6f0cb31f08dd46a8f315af0ffc1 \
            head:c20d1efca738da63b923f0b633f24143c0d711c5; do
     n=${p%%:*}; r=${p#*:}; mkdir "$W/$n"
     git archive "$r" | tar -x -C "$W/$n"
     (cd "$W/$n" && go test -c -o "$OUT/$n.test" ./internal/core/rib/locrib/) \
       && echo "$n $r" >> "$OUT/provenance.txt"
   done
   ```

3. Record the provenance.

   ```text
   { date -u +%FT%TZ; go version; go env GOROOT; uname -a
     sysctl -n machdep.cpu.brand_string hw.ncpu hw.memsize; pmset -g batt
     echo "flags: -test.run ^\$ -test.bench ^BenchmarkSharded -test.benchmem -test.benchtime 1000000x, 10 interleaved rounds"
   } >> "$OUT/provenance.txt"
   ```

4. Run ten interleaved rounds.

   ```text
   for i in $(seq 10); do for n in base impl head; do
     "$OUT/$n.test" -test.run '^$' -test.bench '^BenchmarkSharded' \
       -test.benchmem -test.benchtime 1000000x >> "$OUT/$n.txt"
   done; done
   ```

5. Compare, then remove the exports.

   ```text
   GOROOT= go install golang.org/x/perf/cmd/benchstat@latest
   benchstat base="$OUT/base.txt" impl="$OUT/impl.txt" head="$OUT/head.txt" | tee "$OUT/benchstat.txt"
   rm -rf "$W"
   ```

The evidence is `provenance.txt`, `base.txt`, `impl.txt`, `head.txt` and
`benchstat.txt`. They go into `plan/records/` beside this page, and the spec's
A-4 row cites them.

## Not established

- `benchstat` is not vendored. Step 5 installs it, and that install has not
  been tried with this toolchain.
- The size of `candidate.go` `Path` at `impl` and at `head` was not measured.
  By the field layout the one-byte field that `dfa96ffa2e` added falls in
  existing padding, which is an inference.
- The allocation columns (`B/op`, `allocs/op`) do not depend on the host, so an
  agent may measure them on any machine. The time columns may not.
