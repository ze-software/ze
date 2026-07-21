### `rsvpte-lsp-setup` -- load-only `slice bounds` panic in `ze`, pre-existing

Observed 2026-07-04 (~1 of 4 full `ze-verify` functional runs): the `ze` engine
process panics during the `rsvpte-lsp-setup` functional test with
`panic: runtime error: slice bounds out of range [:5448] with capacity 512`
(exit 2 -> `expect exit-code 0` fails). A cap-512 buffer is resliced to hold
5448 bytes -- a "trust a length, do not grow/check the buffer" bug; 5448 bytes
is the size of a large boot-time frame (e.g. the share-registry command dump the
external `rsvpte-setup` plugin receives). The test config boots rsvp-te + a BGP
peer (`accept false`, never establishes) + the external JSON plugin.

Reproduction is environment-specific, NOT raw repetition: 0 panics across 40
serial + ~360 parallel isolated `ze-test rsvpte 3` runs, 0 under a `-race` build
in isolation (no data race detected at that load), 0 under heavy synthetic load
(which only produced 15s timeouts). It only appears in the full-verify
environment (all feature plugins compiled in, GOMAXPROCS=13, real suite load).
The verify aggregator truncates the goroutine stack to 2 lines
(`goroutine N [running]:`); the runner itself keeps up to 10 MB / 200 lines
(`runner_exec_util.go:55`, `report.go:175`), so a full-suite repro captured via
`ze-test rsvpte --all -v` (not the aggregator) will carry the crash site.

Ruled out (producers read, all safe): BGP text/JSON format scratch buffers
(`format/text_human.go:224`, `format/text_json.go:375` -- both guarded by
`if n > cap(raw)`), the RPC frame/batch pools (`pkg/plugin/rpc/framing.go`
4 KB-cap, `batch.go`, `conn.go:writeAppended`, `mux.go` -- all `append`-based),
and the RSVP message builder (`rsvpte/build.go:encodeMessage`, 1500-cap). The
BGP forwarding/update pools do not run (the peer never establishes). The cap-512
buffer is elsewhere; the captured crash stack will pin it. Owner: in-progress
this session (debugging continues).

**UPDATE 2026-07-13 — the cap-512 diagnosis is DISPROVEN; likely already-fixed or
misattributed.** Two independent exhaustive static sweeps (share-registry send
path + repo-wide pooled/fixed-cap reslice) found NO cap-512 buffer resliced to a
data-driven length on any boot-reachable path: the registry send is
`json.Marshal` + `append` (`plugin/server/startup.go:733` -> `ipc/rpc.go:171` ->
`rpc/mux.go:110`/`conn.go:286` -> `framePool` cap **4096**, append-grown); the BGP
`SessionBuffer` is 4096/65535, never 512 (`core/bgp/wire/writer.go:48`); the only
cap-512 format scratches (`format/text_human.go:219`, `text_json.go:370`) are
guarded and hold <=512 raw bytes. Dynamic: **0 reproductions in 160 runs** (40
isolated + 120 under `scripts/dev/stress-repro.py` full-core CPU+GC load,
`GOTRACEBACK=all`). The 2026-07-04 crash also predates the plugin
startup/RPC-dispatch refactors (`1eb89f509`, `3404c4396`, `8f3203ef5`, 07-07/08)
that rewrote this exact area. Conclusion: either already fixed by those refactors,
or the truncated 2-line aggregator stack misattributed another concurrent suite's
`ze` crash to rsvpte. **Do not chase the cap-512 share-registry hypothesis.** If a
`rsvpte-lsp` exit-2 recurs, reproduce with `scripts/dev/stress-repro.py rsvpte`
(keeps the untruncated stack) and grep every concurrent daemon's stderr in the
failing run before attributing. A separate `rsvpte-lsp-teardown` exit-2 (no stack
in the 200-line capture) was seen once on 2026-07-13 and did not reproduce in 160
runs; it is not the same panic and its cause is unverified.
