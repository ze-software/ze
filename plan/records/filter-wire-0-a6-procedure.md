# Procedure behind the A-6 slow-path measurement

This page records how `filter-wire-0-a6-mac.bench.txt` was produced. The
figures in that record came from `test/perf/filter-a6/measure.sh`, which
Thomas ran on his Mac on 2026-11-04. The script was then removed from the tree,
because `TestNoPythonLeRemains` (`internal/le/contract_test.go`) refuses a
tracked shell recipe and because its workflow exports a revision into a copy of
the tree, which agents may not do. The prose below keeps the procedure, so a
reader of the record can see what each figure measured and redo the measurement.

## What it measured

Assumption A-6 of `spec-filter-wire-0-umbrella`: the cost of the forked-filter
slow path for a route the filter modifies (LOCAL_PREF 100 to 200), from the
received UPDATE to the rebuilt one. It measured the same path at two revisions,
under one Go toolchain:

| Side | Benchmark | Source |
|------|-----------|--------|
| HEAD | `BenchmarkFilterSlowPathModify` | `internal/component/bgp/reactor/filter_slow_path_bench_test.go` |
| Text era | `BenchmarkFilterSlowPathModifyTextEra` | `test/perf/filter-a6/textera_bench_test.go.txt`, added to revision `ec9f715546` |

`ec9f715546` is the last revision whose live import chain sent the text
subject. Both benchmarks run a real SDK plugin over `net.Pipe` through the
engine's real `PluginConn` RPC framing. Neither includes the TLS socket.

## Where it may run

Time figures come only from the owner's Mac (`ai/META-DECISIONS.md`, "Test
systems and labs"), so the script refused any system other than Darwin. With
`A6_CHECK=1` it ran one short pass (`-benchtime 20x -count 1`) on any host, to
prove the procedure works. The times of that pass are not figures. A real run
used `-count 10`.

## The steps

1. From the checkout root, take the patched toolchain from
   `./le go stdpatch goroot`, and set `GOROOT` to it, `GOTOOLCHAIN=local`,
   `CGO_ENABLED=0`, with that root's `bin` first on `PATH`.
2. Create an output directory and a separate temporary directory for the
   text-era source.
3. Write `provenance.txt` into the output directory: the UTC date, the HEAD
   commit, the count of dirty files under `internal` and `pkg`, the full
   text-era commit, the SHA-256 of `textera_bench_test.go.txt`, `go version`,
   the GOROOT, `uname -a`, and on the Mac the CPU brand
   (`sysctl -n machdep.cpu.brand_string`) and the power source
   (`pmset -g batt`), then the benchmark flags.
4. Run the HEAD side and write its output to `head.txt`:
   `go test -tags ze_bgp -run '^$' -bench '^BenchmarkFilterSlowPathModify$' -benchmem -count 10 ./internal/component/bgp/reactor/`.
5. Export `ec9f715546` with `git archive` into the temporary directory, copy
   `textera_bench_test.go.txt` there as
   `internal/component/bgp/reactor/filter_slowpath_textera_bench_test.go`, and
   from that directory run
   `go test -run '^$' -bench '^BenchmarkFilterSlowPathModifyTextEra$' -benchmem -count 10 ./internal/component/bgp/reactor/`,
   writing its output to `textera.txt`. The text-era side takes no build tag.
6. Remove the temporary directory and print the output directory's path. The
   three files, `provenance.txt`, `head.txt` and `textera.txt`, are what
   `filter-wire-0-a6-mac.bench.txt` holds.

Step 5 exports a past revision into a copy of the tree. Only the owner runs
it, on the Mac: an agent never develops or tests in a copy of the tree.
